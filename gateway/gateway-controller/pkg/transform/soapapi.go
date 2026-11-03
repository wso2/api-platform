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
	"strings"

	versionutil "github.com/wso2/api-platform/common/version"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	policyv1alpha "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
)

// SoapWildcardPath is the operation path of every SOAP route. "/*" makes a SOAP API a
// single wildcard resource under its context: all SOAP traffic for the service is
// proxied to the backend without per-operation (SOAPAction) awareness.
const SoapWildcardPath = "/*"

// soapRouteMethods are the methods a SOAP API serves: POST carries SOAP envelopes and
// GET serves ?wsdl / ?xsd retrieval from the backend.
var soapRouteMethods = []string{"POST", "GET"}

// soapDescriptionMethod is the method of the route that serves the service description,
// and so the one the WSDL rewrite is attached to.
const soapDescriptionMethod = "GET"

// SoapAPITransformer transforms a StoredConfig (SoapApi kind) into a RuntimeDeployConfig.
//
// A SOAP API is passthrough only: one wildcard POST route and one wildcard GET route at
// the API context, both proxied unchanged to the backend over a single upstream cluster
// (plus the same pair on the sandbox vhost when a sandbox upstream is configured). The
// wildcard path is rewritten as ^<context>(.*)$ -> <upstreamPath>\1, so a request to
// <context>/soap11 reaches <upstreamPath>/soap11 and the ?wsdl query string is kept.
//
// The GET route also carries the SOAP system policy, which rewrites the backend's URLs in
// the service description it returns to the gateway's.
type SoapAPITransformer struct {
	routerConfig      *config.RouterConfig
	systemConfig      *config.Config
	policyDefinitions map[string]models.PolicyDefinition
	latestVersions    map[string]string // pre-computed policyName -> latest full semver
}

// NewSoapAPITransformer creates a new SoapAPITransformer.
func NewSoapAPITransformer(
	routerConfig *config.RouterConfig,
	systemConfig *config.Config,
	policyDefinitions map[string]models.PolicyDefinition,
) *SoapAPITransformer {
	return &SoapAPITransformer{
		routerConfig:      routerConfig,
		systemConfig:      systemConfig,
		policyDefinitions: policyDefinitions,
		latestVersions:    config.BuildLatestVersionIndex(policyDefinitions),
	}
}

// Transform converts a StoredConfig with SoapAPI configuration into a RuntimeDeployConfig.
func (t *SoapAPITransformer) Transform(cfg *models.StoredConfig) (*models.RuntimeDeployConfig, error) {
	soapCfg, ok := cfg.Configuration.(api.SoapAPI)
	if !ok {
		return nil, fmt.Errorf("configuration is not a SoapAPI")
	}
	apiData := soapCfg.Spec

	rdc := &models.RuntimeDeployConfig{
		Metadata: models.Metadata{
			UUID:        cfg.UUID,
			Kind:        cfg.Kind,
			Handle:      cfg.Handle,
			Version:     apiData.Version,
			DisplayName: apiData.DisplayName,
			ProjectID:   extractProjectID(cfg),
		},
		Context:             strings.ReplaceAll(apiData.Context, "$version", apiData.Version),
		PolicyChainResolver: "route-key",
		Routes:              make(map[string]*models.Route),
		PolicyChains:        make(map[string]*models.PolicyChain),
		UpstreamClusters:    make(map[string]*models.UpstreamCluster),
		SensitiveValues:     cfg.SensitiveValues,
	}

	// Effective vhosts (fall back to the gateway defaults when not specified).
	mainVhost := t.routerConfig.VHosts.Main.Default
	sandboxVhost := t.routerConfig.VHosts.Sandbox.Default
	if apiData.Vhosts != nil {
		if strings.TrimSpace(apiData.Vhosts.Main) != "" {
			mainVhost = apiData.Vhosts.Main
		}
		if apiData.Vhosts.Sandbox != nil && strings.TrimSpace(*apiData.Vhosts.Sandbox) != "" {
			sandboxVhost = *apiData.Vhosts.Sandbox
		}
	}

	// The API-level chain is the whole chain of every SOAP route: there are no
	// operations to carry operation-level policies.
	apiPolicies, err := t.collectAPIPolicies(apiData.Policies)
	if err != nil {
		return nil, err
	}
	injected := utils.InjectSystemPolicies(apiPolicies, t.systemConfig, nil)
	// The fault chain is the API's fault policies followed by the fault-path system
	// policies, built the same way as a RestApi operation's (see RestAPITransformer.Transform).
	faultChain := appendFaultSystemPolicies(
		collectFaultPolicies(t.policyDefinitions, t.latestVersions, apiData.FaultPolicies, policyv1alpha.LevelAPI),
		sdkChainToModel(utils.FaultSystemPolicies(t.systemConfig, nil)).Policies,
	)

	mainUpstream, err := addUpstreamCluster(rdc, "main", &apiData.Upstream.Main, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve main upstream: %w", err)
	}
	if err := t.addRoutes(rdc, &apiData, mainVhost, mainUpstream, &apiData.Upstream.Main, injected, faultChain); err != nil {
		return nil, err
	}

	// Sandbox is active when a sandbox upstream is configured (SoapApi only supports a
	// direct url — see validateSoapAPIConfig — never a ref).
	hasSandbox := apiData.Upstream.Sandbox != nil &&
		apiData.Upstream.Sandbox.Url != nil && strings.TrimSpace(*apiData.Upstream.Sandbox.Url) != ""
	if hasSandbox {
		// The sandbox route keys would collide with the main ones (same METHOD|PATH|VHOST).
		if sandboxVhost == mainVhost {
			return nil, fmt.Errorf("sandbox upstream is configured but resolves to the same vhost %q as the main upstream; configure distinct vhosts to avoid route conflicts", sandboxVhost)
		}
		sbUpstream, err := addUpstreamCluster(rdc, "sandbox", apiData.Upstream.Sandbox, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve sandbox upstream: %w", err)
		}
		if err := t.addRoutes(rdc, &apiData, sandboxVhost, sbUpstream, apiData.Upstream.Sandbox, injected, faultChain); err != nil {
			return nil, err
		}
	}

	return rdc, nil
}

// addRoutes adds the wildcard POST and GET routes for one vhost, each with its own copy
// of the policy chain and the shared fault chain. The GET route's chain ends with the WSDL
// rewrite for this vhost's upstream.
func (t *SoapAPITransformer) addRoutes(
	rdc *models.RuntimeDeployConfig,
	apiData *api.SoapAPIData,
	vhost string,
	upstream *upstreamClusterResult,
	up *api.Upstream,
	chain []policyenginev1.PolicyInstance,
	faultChain []models.Policy,
) error {
	wsdlRewrite, err := t.wsdlRewritePolicy(rdc.Context, vhost, up)
	if err != nil {
		return err
	}
	autoHostRewrite := up.HostRewrite == nil || *up.HostRewrite != api.UpstreamHostRewriteManual
	upstreamInfo := upstream.UpstreamInfo()
	fullPath := xds.ConstructFullPath(apiData.Context, apiData.Version, SoapWildcardPath)

	for _, method := range soapRouteMethods {
		routeKey := xds.GenerateRouteName(method, apiData.Context, apiData.Version, SoapWildcardPath, vhost)
		rdc.Routes[routeKey] = &models.Route{
			Method:          method,
			Path:            fullPath,
			OperationPath:   SoapWildcardPath,
			Vhost:           vhost,
			AutoHostRewrite: autoHostRewrite,
			Upstream: models.RouteUpstream{
				ClusterKey: upstream.ClusterKey,
				Default:    &upstreamInfo,
			},
		}
		routeChain := sdkChainToModel(chain)
		if method == soapDescriptionMethod {
			// Last in the chain, so it runs first in the response phase and every
			// response policy the author attached sees the gateway's URLs.
			routeChain.Policies = append(routeChain.Policies, wsdlRewrite)
		}
		routeChain.FaultPolicies = append([]models.Policy(nil), faultChain...)
		rdc.PolicyChains[routeKey] = routeChain
	}
	return nil
}

// wsdlRewritePolicy builds the SOAP system policy instance for one GET route.
//
// Attached by name, so its version is resolved from the loaded definitions like an
// author-attached policy's — and, as for the A2A system policy, an unresolvable one fails
// the deployment rather than being dropped: without it the description would publish the
// backend's own address, routing every generated client around the gateway.
func (t *SoapAPITransformer) wsdlRewritePolicy(gatewayPath, vhost string, up *api.Upstream) (models.Policy, error) {
	resolved, err := config.ResolvePolicyVersion(t.policyDefinitions, t.latestVersions, constants.SOAP_SYSTEM_POLICY_NAME, "")
	if err != nil {
		return models.Policy{}, fmt.Errorf("cannot attach the SOAP system policy: %w", err)
	}
	upstreamURL := ""
	if up.Url != nil {
		upstreamURL = strings.TrimSpace(*up.Url)
	}
	wsdlRewrite := map[string]interface{}{
		constants.SOAP_POLICY_PARAM_GATEWAY_PATH: gatewayPath,
		constants.SOAP_POLICY_PARAM_UPSTREAM_URL: upstreamURL,
		constants.SOAP_POLICY_PARAM_VHOST:        vhost,
	}
	// The router buffers at most this much for the policy to read, so the policy's own
	// ceiling follows it rather than a second, disagreeing limit. Unset, the policy's
	// default (the router's default) applies.
	if limit := t.routerConfig.HTTPListener.PerConnectionBufferLimitBytes; limit > 0 {
		wsdlRewrite[constants.SOAP_POLICY_PARAM_MAX_BYTES] = int(limit)
	}
	return models.Policy{
		Name:    constants.SOAP_SYSTEM_POLICY_NAME,
		Version: versionutil.MajorVersion(resolved),
		Params: map[string]interface{}{
			constants.SOAP_POLICY_PARAM_WSDL_REWRITE: wsdlRewrite,
		},
	}, nil
}

// collectAPIPolicies returns the resolved API-level policies in spec order. As in
// GraphQLAPITransformer, a policy that fails to resolve aborts the deployment rather
// than being dropped: the API-level chain is a SOAP route's entire chain, so silently
// omitting a policy (an auth policy among them) would deploy weaker enforcement than
// configured.
func (t *SoapAPITransformer) collectAPIPolicies(policies *[]api.Policy) ([]policyenginev1.PolicyInstance, error) {
	var result []policyenginev1.PolicyInstance
	if policies == nil {
		return result, nil
	}
	for _, p := range *policies {
		resolved, err := config.ResolvePolicyVersion(t.policyDefinitions, t.latestVersions, p.Name, p.Version)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve policy version for SOAP API-level policy %q: %w", p.Name, err)
		}
		result = append(result, convertAPIPolicyToSDK(p, policyv1alpha.LevelAPI, versionutil.MajorVersion(resolved)))
	}
	return result, nil
}
