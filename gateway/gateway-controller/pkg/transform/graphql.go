/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package transform

import (
	"fmt"
	"log/slog"
	"strings"

	versionutil "github.com/wso2/api-platform/common/version"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	policyv1alpha "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
)

// GraphQLAPITransformer transforms a StoredConfig (GraphQLApi kind) into a
// RuntimeDeployConfig. Unlike RestAPITransformer, it never loops over operations: a
// GraphQL API exposes exactly one logical endpoint (the "operation" — query/mutation
// name — is identified by the POST body, not the URL), so Transform builds exactly one
// models.Route per configured upstream slot (main, and sandbox if present), not one per
// operation.
type GraphQLAPITransformer struct {
	routerConfig      *config.RouterConfig
	systemConfig      *config.Config
	policyDefinitions map[string]models.PolicyDefinition
	latestVersions    map[string]string // pre-computed policyName -> latest full semver
}

// NewGraphQLAPITransformer creates a new GraphQLAPITransformer.
func NewGraphQLAPITransformer(
	routerConfig *config.RouterConfig,
	systemConfig *config.Config,
	policyDefinitions map[string]models.PolicyDefinition,
) *GraphQLAPITransformer {
	return &GraphQLAPITransformer{
		routerConfig:      routerConfig,
		systemConfig:      systemConfig,
		policyDefinitions: policyDefinitions,
		latestVersions:    config.BuildLatestVersionIndex(policyDefinitions),
	}
}

// Transform converts a StoredConfig with GraphQLApi configuration into a
// RuntimeDeployConfig containing exactly one route per active upstream slot.
func (t *GraphQLAPITransformer) Transform(cfg *models.StoredConfig) (*models.RuntimeDeployConfig, error) {
	graphqlCfg, ok := cfg.Configuration.(api.GraphQLAPI)
	if !ok {
		return nil, fmt.Errorf("configuration is not a GraphQLAPI")
	}
	apiData := graphqlCfg.Spec

	projectID := extractProjectID(cfg)

	rdc := &models.RuntimeDeployConfig{
		Metadata: models.Metadata{
			UUID:        cfg.UUID,
			Kind:        cfg.Kind,
			Handle:      cfg.Handle,
			Version:     apiData.Version,
			DisplayName: apiData.DisplayName,
			ProjectID:   projectID,
		},
		Context:             strings.ReplaceAll(apiData.Context, "$version", apiData.Version),
		PolicyChainResolver: "route-key",
		Routes:              make(map[string]*models.Route),
		PolicyChains:        make(map[string]*models.PolicyChain),
		UpstreamClusters:    make(map[string]*models.UpstreamCluster),
		SensitiveValues:     cfg.SensitiveValues,
	}

	// Collect and resolve the API-level policy chain once — a GraphQLApi has no
	// operation-level policies (there are no operations), so the API-level chain IS
	// the route's whole chain (plus injected system policies).
	apiPolicies := t.collectAPIPolicies(apiData.Policies)
	chain := t.buildPolicyChain(apiPolicies)
	injected := utils.InjectSystemPolicies(chain, t.systemConfig, nil)

	// fullPath has no operation-path suffix: a GraphQLApi's whole route match is the
	// resolved context (ConstructFullPath(context, version, "") == context+version,
	// since appending "" is a no-op).
	fullPath := xds.ConstructFullPath(apiData.Context, apiData.Version, "")
	mainVhost := t.routerConfig.VHosts.Main.Default

	// Build main upstream cluster and its single route. The route KEY must follow the
	// "METHOD|PATH|VHOST" convention (xds.GenerateRouteName) — translator.go's
	// TranslateConfigs groups Envoy routes into virtual hosts by splitting the route's
	// Name (which is set to this map key) on "|" and reading index 2 as the vhost; an
	// ad-hoc key would silently vanish from every virtual host.
	mainUpstream, err := resolveUpstreamCluster(rdc, "main", &apiData.Upstream.Main, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve main upstream: %w", err)
	}
	mainUpstreamInfo := mainUpstream.UpstreamInfo()

	mainAutoHostRewrite := true
	if apiData.Upstream.Main.HostRewrite != nil && *apiData.Upstream.Main.HostRewrite == api.Manual {
		mainAutoHostRewrite = false
	}

	mainRouteKey := xds.GenerateRouteName("POST", apiData.Context, apiData.Version, "", mainVhost)
	rdc.Routes[mainRouteKey] = &models.Route{
		Method: "POST",
		Path:   fullPath,
		// A GraphQLApi has no operations to derive a per-route path from (see the
		// package doc above), but leaving this "" makes every request look
		// operation-less to a request-scoped policy. Some policies — api-key-auth
		// v1.2.1 among them — treat an empty OperationPath as "missing API details"
		// and fail closed, instead of "not applicable" as SharedContext.OperationPath's
		// own doc comment (sdk/core/policy/v1alpha2/context.go) says an empty value
		// must be read. "/" is not a fake sub-path: it is this API's one and only
		// operation, at its own root.
		OperationPath:   "/",
		PathMatchType:   "Exact",
		Vhost:           mainVhost,
		AutoHostRewrite: mainAutoHostRewrite,
		Upstream: models.RouteUpstream{
			ClusterKey: mainUpstream.ClusterKey,
			Default:    &mainUpstreamInfo,
		},
	}
	rdc.PolicyChains[mainRouteKey] = sdkChainToModel(injected)

	// Sandbox is active when a sandbox upstream is configured (GraphQLApi only
	// supports a direct url — see validateGraphQLUpstream — never a ref).
	hasSandbox := apiData.Upstream.Sandbox != nil &&
		apiData.Upstream.Sandbox.Url != nil && strings.TrimSpace(*apiData.Upstream.Sandbox.Url) != ""

	if hasSandbox {
		sandboxVhost := t.routerConfig.VHosts.Sandbox.Default
		if sandboxVhost == mainVhost {
			return nil, fmt.Errorf("sandbox upstream is configured but resolves to the same vhost %q as the main upstream; configure distinct vhosts to avoid route conflicts", sandboxVhost)
		}

		sbUpstream, err := resolveUpstreamCluster(rdc, "sandbox", apiData.Upstream.Sandbox, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve sandbox upstream: %w", err)
		}
		sbUpstreamInfo := sbUpstream.UpstreamInfo()

		sbAutoHostRewrite := true
		if apiData.Upstream.Sandbox.HostRewrite != nil && *apiData.Upstream.Sandbox.HostRewrite == api.Manual {
			sbAutoHostRewrite = false
		}

		sandboxRouteKey := xds.GenerateRouteName("POST", apiData.Context, apiData.Version, "", sandboxVhost)
		rdc.Routes[sandboxRouteKey] = &models.Route{
			Method: "POST",
			Path:   fullPath,
			// See the main route's OperationPath comment above.
			OperationPath:   "/",
			PathMatchType:   "Exact",
			Vhost:           sandboxVhost,
			AutoHostRewrite: sbAutoHostRewrite,
			Upstream: models.RouteUpstream{
				ClusterKey: sbUpstream.ClusterKey,
				Default:    &sbUpstreamInfo,
			},
		}
		rdc.PolicyChains[sandboxRouteKey] = sdkChainToModel(injected)
	}

	return rdc, nil
}

// collectAPIPolicies returns the resolved API-level policies as a slice in spec
// order, mirroring RestAPITransformer.collectAPIPolicies exactly (duplicated rather
// than extracted to a shared function because it is only a few lines and — unlike
// resolveUpstreamCluster, which is a large self-contained block with no transformer
// state — depends on t.policyDefinitions/t.latestVersions, so sharing it would mean
// plumbing those through a standalone helper for a single call site on each side).
func (t *GraphQLAPITransformer) collectAPIPolicies(policies *[]api.Policy) []policyenginev1.PolicyInstance {
	var result []policyenginev1.PolicyInstance
	if policies == nil {
		return result
	}
	for _, p := range *policies {
		resolved, err := config.ResolvePolicyVersion(t.policyDefinitions, t.latestVersions, p.Name, p.Version)
		if err != nil {
			slog.Error("Failed to resolve policy version for GraphQL API-level policy", "policy_name", p.Name, "error", err)
			continue
		}
		result = append(result, convertAPIPolicyToSDK(p, policyv1alpha.LevelAPI, versionutil.MajorVersion(resolved)))
	}
	return result
}

// buildPolicyChain returns the API-level policy chain. A GraphQLApi has no
// operation-level policies to merge in (there are no operations), unlike
// RestAPITransformer.buildPolicyChain.
func (t *GraphQLAPITransformer) buildPolicyChain(apiPolicies []policyenginev1.PolicyInstance) []policyenginev1.PolicyInstance {
	result := make([]policyenginev1.PolicyInstance, 0, len(apiPolicies))
	result = append(result, apiPolicies...)
	return result
}
