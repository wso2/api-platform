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
	"net/url"
	"strconv"
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	policyv1alpha "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	policyenginev1 "github.com/wso2/api-platform/sdk/gateway/policyengine/v1"
)

// RestAPITransformer transforms a StoredConfig (RestAPI kind) into a RuntimeDeployConfig.
type RestAPITransformer struct {
	routerConfig      *config.RouterConfig
	systemConfig      *config.Config
	policyDefinitions map[string]api.PolicyDefinition
}

// NewRestAPITransformer creates a new RestAPITransformer.
func NewRestAPITransformer(
	routerConfig *config.RouterConfig,
	systemConfig *config.Config,
	policyDefinitions map[string]api.PolicyDefinition,
) *RestAPITransformer {
	return &RestAPITransformer{
		routerConfig:      routerConfig,
		systemConfig:      systemConfig,
		policyDefinitions: policyDefinitions,
	}
}

// Transform converts a StoredConfig with RestAPI configuration into a RuntimeDeployConfig.
func (t *RestAPITransformer) Transform(cfg *models.StoredConfig) (*models.RuntimeDeployConfig, error) {
	restCfg, ok := cfg.Configuration.(api.RestAPI)
	if !ok {
		return nil, fmt.Errorf("configuration is not a RestAPI")
	}
	apiData := restCfg.Spec

	// Extract project ID from labels
	projectID := ""
	if labels := cfg.GetLabels(); labels != nil {
		if pid, exists := (*labels)["project-id"]; exists {
			projectID = pid
		}
	}

	rdc := &models.RuntimeDeployConfig{
		Metadata: models.Metadata{
			Kind:        cfg.Kind,
			Handle:      cfg.Handle,
			Name:        apiData.DisplayName,
			Version:     apiData.Version,
			DisplayName: apiData.DisplayName,
			ProjectID:   projectID,
		},
		Context:             apiData.Context,
		PolicyChainResolver: "route-key",
		Routes:              make(map[string]*models.Route),
		PolicyChains:        make(map[string]*models.PolicyChain),
		UpstreamClusters:    make(map[string]*models.UpstreamCluster),
	}

	// Collect validated API-level policies
	apiPolicies := t.collectAPIPolicies(apiData.Policies)

	// Determine effective vhosts
	effectiveMainVHost := t.routerConfig.VHosts.Main.Default
	effectiveSandboxVHost := t.routerConfig.VHosts.Sandbox.Default
	if apiData.Vhosts != nil {
		if strings.TrimSpace(apiData.Vhosts.Main) != "" {
			effectiveMainVHost = apiData.Vhosts.Main
		}
		if apiData.Vhosts.Sandbox != nil && strings.TrimSpace(*apiData.Vhosts.Sandbox) != "" {
			effectiveSandboxVHost = *apiData.Vhosts.Sandbox
		}
	}

	// Build main upstream cluster
	mainClusterKey, _, err := t.addUpstreamCluster(rdc, "main", &apiData.Upstream.Main, apiData.UpstreamDefinitions)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve main upstream: %w", err)
	}

	// Check if dynamic cluster selection should be used
	useClusterHeader := apiData.UpstreamDefinitions != nil && len(*apiData.UpstreamDefinitions) > 0
	defaultCluster := ""
	if useClusterHeader {
		defaultCluster = mainClusterKey
	}

	// Determine auto host rewrite for main upstream
	mainAutoHostRewrite := true
	if apiData.Upstream.Main.HostRewrite != nil && *apiData.Upstream.Main.HostRewrite == api.Manual {
		mainAutoHostRewrite = false
	}

	// Determine vhosts to create routes for
	hasSandbox := apiData.Upstream.Sandbox != nil && apiData.Upstream.Sandbox.Url != nil &&
		strings.TrimSpace(*apiData.Upstream.Sandbox.Url) != ""

	// Build routes and policy chains for each operation
	for _, op := range apiData.Operations {
		vhosts := []string{effectiveMainVHost}
		if hasSandbox {
			vhosts = append(vhosts, effectiveSandboxVHost)
		}

		for _, vhost := range vhosts {
			routeKey := xds.GenerateRouteName(string(op.Method), apiData.Context, apiData.Version, op.Path, vhost)

			// Build route
			rdc.Routes[routeKey] = &models.Route{
				Method:          string(op.Method),
				Path:            xds.ConstructFullPath(apiData.Context, apiData.Version, op.Path),
				OperationPath:   op.Path,
				Vhost:           vhost,
				AutoHostRewrite: mainAutoHostRewrite,
				Upstream: models.RouteUpstream{
					ClusterKey:       mainClusterKey,
					UseClusterHeader: useClusterHeader,
					DefaultCluster:   defaultCluster,
				},
			}

			// Build policy chain: API-level + operation-level + system policies
			chain := t.buildPolicyChain(apiPolicies, apiData.Policies, op.Policies)
			injected := utils.InjectSystemPolicies(chain, t.systemConfig, nil)
			rdc.PolicyChains[routeKey] = sdkChainToModel(injected)
		}
	}

	// Add upstream definition clusters for dynamic routing
	if apiData.UpstreamDefinitions != nil {
		for _, def := range *apiData.UpstreamDefinitions {
			if len(def.Upstreams) == 0 || def.Upstreams[0].Url == "" {
				continue
			}
			defClusterKey := "upstream_" + cfg.Kind + "_" + cfg.UUID + "_" + SanitizeUpstreamDefinitionName(def.Name)
			parsedURL, err := url.Parse(def.Upstreams[0].Url)
			if err != nil {
				return nil, fmt.Errorf("invalid URL in upstream definition '%s': %w", def.Name, err)
			}
			port := ResolvePort(parsedURL)
			basePath := parsedURL.Path
			if basePath == "" {
				basePath = "/"
			}
			rdc.UpstreamClusters[defClusterKey] = &models.UpstreamCluster{
				BasePath: basePath,
				Endpoints: []models.Endpoint{{
					Host: parsedURL.Hostname(),
					Port: port,
				}},
				TLS: &models.UpstreamTLS{Enabled: parsedURL.Scheme == "https"},
			}
		}
	}

	// Add sandbox upstream and update sandbox routes if present
	if hasSandbox {
		sbClusterKey, _, err := t.addUpstreamCluster(rdc, "sandbox", apiData.Upstream.Sandbox, apiData.UpstreamDefinitions)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve sandbox upstream: %w", err)
		}

		sbAutoHostRewrite := true
		if apiData.Upstream.Sandbox.HostRewrite != nil && *apiData.Upstream.Sandbox.HostRewrite == api.Manual {
			sbAutoHostRewrite = false
		}

		// Update sandbox vhost routes to point to sandbox cluster
		for _, op := range apiData.Operations {
			routeKey := xds.GenerateRouteName(string(op.Method), apiData.Context, apiData.Version, op.Path, effectiveSandboxVHost)
			if r, exists := rdc.Routes[routeKey]; exists {
				r.Upstream.ClusterKey = sbClusterKey
				r.Upstream.UseClusterHeader = false
				r.Upstream.DefaultCluster = ""
				r.AutoHostRewrite = sbAutoHostRewrite
			}
		}
	}

	return rdc, nil
}

// collectAPIPolicies validates and collects API-level policies into SDK format.
func (t *RestAPITransformer) collectAPIPolicies(policies *[]api.Policy) map[string]policyenginev1.PolicyInstance {
	result := make(map[string]policyenginev1.PolicyInstance)
	if policies == nil {
		return result
	}
	for _, p := range *policies {
		_, err := config.ResolvePolicyVersion(t.policyDefinitions, p.Name, p.Version)
		if err != nil {
			slog.Error("Failed to resolve policy version for API-level policy", "policy_name", p.Name, "error", err)
			continue
		}
		result[p.Name] = convertAPIPolicyToSDK(p, policyv1alpha.LevelAPI, p.Version)
	}
	return result
}

// buildPolicyChain builds a merged list: API-level + operation-level policies (SDK format).
func (t *RestAPITransformer) buildPolicyChain(
	apiPolicies map[string]policyenginev1.PolicyInstance,
	specPolicies *[]api.Policy,
	opPolicies *[]api.Policy,
) []policyenginev1.PolicyInstance {
	var result []policyenginev1.PolicyInstance

	// API-level policies (in spec order, validated via apiPolicies map)
	if specPolicies != nil {
		for _, p := range *specPolicies {
			if v, ok := apiPolicies[p.Name]; ok {
				result = append(result, v)
			}
		}
	}

	// Operation-level policies
	if opPolicies != nil {
		for _, opPol := range *opPolicies {
			_, err := config.ResolvePolicyVersion(t.policyDefinitions, opPol.Name, opPol.Version)
			if err != nil {
				slog.Error("Failed to resolve operation-level policy version", "policy_name", opPol.Name, "error", err)
				continue
			}
			result = append(result, convertAPIPolicyToSDK(opPol, policyv1alpha.LevelRoute, opPol.Version))
		}
	}

	return result
}

// addUpstreamCluster resolves an upstream and adds it to the RuntimeDeployConfig.
// Returns the cluster key and the base path.
func (t *RestAPITransformer) addUpstreamCluster(
	rdc *models.RuntimeDeployConfig,
	upstreamName string,
	up *api.Upstream,
	upstreamDefinitions *[]api.UpstreamDefinition,
) (string, string, error) {
	rawURL, err := resolveUpstreamURL(upstreamName, up, upstreamDefinitions)
	if err != nil {
		return "", "", err
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid %s upstream URL: %w", upstreamName, err)
	}
	if parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return "", "", fmt.Errorf("invalid %s upstream URL: must include host and http/https scheme", upstreamName)
	}

	port := ResolvePort(parsedURL)
	basePath := parsedURL.Path
	if basePath == "" {
		basePath = "/"
	}

	clusterKey := fmt.Sprintf("upstream_%s_%s_%d", upstreamName, parsedURL.Hostname(), port)

	rdc.UpstreamClusters[clusterKey] = &models.UpstreamCluster{
		BasePath: basePath,
		Endpoints: []models.Endpoint{{
			Host: parsedURL.Hostname(),
			Port: port,
		}},
		TLS: &models.UpstreamTLS{Enabled: parsedURL.Scheme == "https"},
	}

	return clusterKey, basePath, nil
}

// resolveUpstreamURL resolves the URL from an upstream (direct URL or ref).
func resolveUpstreamURL(name string, up *api.Upstream, defs *[]api.UpstreamDefinition) (string, error) {
	if up.Url != nil && strings.TrimSpace(*up.Url) != "" {
		return strings.TrimSpace(*up.Url), nil
	}
	if up.Ref != nil && strings.TrimSpace(*up.Ref) != "" {
		refName := strings.TrimSpace(*up.Ref)
		if defs == nil {
			return "", fmt.Errorf("upstream definition '%s' referenced but no definitions provided", refName)
		}
		for _, def := range *defs {
			if def.Name == refName {
				if len(def.Upstreams) == 0 || def.Upstreams[0].Url == "" {
					return "", fmt.Errorf("upstream definition '%s' has no URLs", refName)
				}
				return def.Upstreams[0].Url, nil
			}
		}
		return "", fmt.Errorf("upstream definition '%s' not found", refName)
	}
	return "", fmt.Errorf("%s upstream has no URL or ref", name)
}

// ResolvePort returns the port from a URL, defaulting to 80/443.
func ResolvePort(u *url.URL) int {
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err == nil {
			return port
		}
	}
	if u.Scheme == "https" {
		return 443
	}
	return 80
}

// SanitizeUpstreamDefinitionName replaces dots and colons for Envoy cluster name compatibility.
func SanitizeUpstreamDefinitionName(name string) string {
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, ":", "_")
	return name
}

// convertAPIPolicyToSDK converts an api.Policy to policyenginev1.PolicyInstance.
func convertAPIPolicyToSDK(p api.Policy, attachedTo policyv1alpha.Level, resolvedVersion string) policyenginev1.PolicyInstance {
	paramsMap := make(map[string]interface{})
	if p.Params != nil {
		for k, v := range *p.Params {
			paramsMap[k] = v
		}
	}
	if attachedTo != "" {
		paramsMap["attachedTo"] = string(attachedTo)
	}

	return policyenginev1.PolicyInstance{
		Name:               p.Name,
		Version:            resolvedVersion,
		Enabled:            true,
		ExecutionCondition: p.ExecutionCondition,
		Parameters:         paramsMap,
	}
}

// sdkChainToModel converts a slice of SDK PolicyInstance to a models.PolicyChain.
func sdkChainToModel(instances []policyenginev1.PolicyInstance) *models.PolicyChain {
	chain := &models.PolicyChain{
		Policies: make([]models.Policy, 0, len(instances)),
	}
	for _, inst := range instances {
		chain.Policies = append(chain.Policies, models.Policy{
			Name:               inst.Name,
			Version:            inst.Version,
			Params:             inst.Parameters,
			ExecutionCondition: inst.ExecutionCondition,
		})
	}
	return chain
}
