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
	"time"

	commonconstants "github.com/wso2/api-platform/common/constants"
	versionutil "github.com/wso2/api-platform/common/version"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	policyv1alpha "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
)

// RestAPITransformer transforms a StoredConfig (RestAPI kind) into a RuntimeDeployConfig.
type RestAPITransformer struct {
	routerConfig      *config.RouterConfig
	systemConfig      *config.Config
	policyDefinitions map[string]models.PolicyDefinition
	latestVersions    map[string]string // pre-computed policyName -> latest full semver
}

// NewRestAPITransformer creates a new RestAPITransformer.
func NewRestAPITransformer(
	routerConfig *config.RouterConfig,
	systemConfig *config.Config,
	policyDefinitions map[string]models.PolicyDefinition,
) *RestAPITransformer {
	return &RestAPITransformer{
		routerConfig:      routerConfig,
		systemConfig:      systemConfig,
		policyDefinitions: policyDefinitions,
		latestVersions:    config.BuildLatestVersionIndex(policyDefinitions),
	}
}

// Transform converts a StoredConfig with RestAPI configuration into a RuntimeDeployConfig.
// extractProjectID reads project ID from the annotation (preferred) then falls back to the
// deprecated bare label, logging a warning if only the label is present.
func extractProjectID(cfg *models.StoredConfig) string {
	if annotations := cfg.GetAnnotations(); annotations != nil {
		if pid, exists := (*annotations)[commonconstants.AnnotationProjectID]; exists {
			return pid
		}
	}
	if labels := cfg.GetLabels(); labels != nil {
		if pid, exists := (*labels)[commonconstants.DeprecatedLabelProjectID]; exists {
			slog.Warn("deprecated project-id label detected; migrate to annotation",
				"annotation", commonconstants.AnnotationProjectID,
				"api", cfg.Handle)
			return pid
		}
	}
	return ""
}

func (t *RestAPITransformer) Transform(cfg *models.StoredConfig) (*models.RuntimeDeployConfig, error) {
	restCfg, ok := cfg.Configuration.(api.RestAPI)
	if !ok {
		return nil, fmt.Errorf("configuration is not a RestAPI")
	}
	apiData := restCfg.Spec

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

	// Collect validated API-level policies
	apiPolicies := t.collectAPIPolicies(apiData.Policies)

	// Determine effective vhosts. vhosts.main may carry several production hostnames separated
	// by ";" (e.g. when a Gateway-API HTTPRoute attaches to multiple listener hostnames); every
	// entry serves the main upstream and the first is the primary vhost. When unset, the gateway
	// default applies. Sandbox is always a single hostname.
	effectiveSandboxVHost := t.routerConfig.VHosts.Sandbox.Default
	mainVhosts := []string{t.routerConfig.VHosts.Main.Default}
	if apiData.Vhosts != nil {
		if parsed := splitVhosts(apiData.Vhosts.Main); len(parsed) > 0 {
			mainVhosts = parsed
		}
		if apiData.Vhosts.Sandbox != nil && strings.TrimSpace(*apiData.Vhosts.Sandbox) != "" {
			effectiveSandboxVHost = *apiData.Vhosts.Sandbox
		}
	}

	// Build main upstream cluster
	mainUpstream, err := t.addUpstreamCluster(rdc, "main", &apiData.Upstream.Main, apiData.UpstreamDefinitions)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve main upstream: %w", err)
	}

	// Check if dynamic cluster selection should be used
	useClusterHeader := apiData.UpstreamDefinitions != nil && len(*apiData.UpstreamDefinitions) > 0
	defaultCluster := ""
	if useClusterHeader {
		// The default cluster must be the name Envoy actually knows the cluster by.
		// translateRuntimeConfig names clusters by their rdc.UpstreamClusters map key
		// (ClusterKey, e.g. "upstream_main_<host>_<port>"), NOT the sanitized
		// "cluster_<scheme>_<host>" form (EnvoyClusterName), so the cluster-header
		// fallback must reference ClusterKey or it points at a non-existent cluster.
		defaultCluster = mainUpstream.ClusterKey
	}

	// Determine auto host rewrite for main upstream
	mainAutoHostRewrite := true
	if apiData.Upstream.Main.HostRewrite != nil && *apiData.Upstream.Main.HostRewrite == api.Manual {
		mainAutoHostRewrite = false
	}

	// Determine vhosts to create routes for.
	// Sandbox is active when a sandbox upstream is configured via either url or ref.
	hasSandbox := apiData.Upstream.Sandbox != nil &&
		((apiData.Upstream.Sandbox.Url != nil && strings.TrimSpace(*apiData.Upstream.Sandbox.Url) != "") ||
			(apiData.Upstream.Sandbox.Ref != nil && strings.TrimSpace(*apiData.Upstream.Sandbox.Ref) != ""))

	// Guard: sandbox and main vhosts must differ, otherwise sandbox routes would
	// overwrite main routes (same route key) and the sandbox patch would leave only
	// a sandbox-cluster route with no main-cluster route at all.
	if hasSandbox {
		for _, mv := range mainVhosts {
			if mv == effectiveSandboxVHost {
				return nil, fmt.Errorf("sandbox upstream is configured but resolves to the same vhost %q as a main upstream; configure distinct vhosts to avoid route conflicts", effectiveSandboxVHost)
			}
		}
	}

	// Resolve API-level resilience timeouts once; operation-level values override these.
	apiTimeout, apiIdleTimeout, err := xds.ResolveResilience(apiData.Resilience)
	if err != nil {
		return nil, fmt.Errorf("invalid API-level resilience: %w", err)
	}

	// Build routes and policy chains for each operation
	for i, op := range apiData.Operations {
		// Operation-level resilience overrides API-level (per field); nil leaves the
		// global route timeout default in effect.
		opTimeout, opIdleTimeout, err := xds.ResolveResilience(op.Resilience)
		if err != nil {
			return nil, fmt.Errorf("invalid resilience for operation %s %s: %w", op.Method, op.Path, err)
		}
		routeTimeout := buildRouteTimeout(opTimeout, apiTimeout, opIdleTimeout, apiIdleTimeout)

		vhosts := append([]string{}, mainVhosts...)
		if hasSandbox {
			vhosts = append(vhosts, effectiveSandboxVHost)
		}

		// Header matchers and their discriminator are vhost-independent, so derive them
		// once per operation. The discriminator keeps the route key unique across
		// operations that share method/path/vhost but match on different headers
		// (e.g. multiple Gateway-API HTTPRoute rules on the same path).
		headerMatches := routeHeaderMatches(op)
		discriminator := xds.HeaderMatchDiscriminator(headerMatches)

		for _, vhost := range vhosts {
			routeKey := xds.GenerateRouteNameWithDiscriminator(string(op.Method), apiData.Context, apiData.Version, op.Path, vhost, discriminator)

			rdcRoute := &models.Route{
				Method:          string(op.Method),
				Path:            xds.ConstructFullPath(apiData.Context, apiData.Version, op.Path),
				OperationPath:   op.Path,
				Vhost:           vhost,
				AutoHostRewrite: mainAutoHostRewrite,
				MatchHeaders:    headerMatches,
				Order:           i,
				Timeout:         routeTimeout,
				Upstream: models.RouteUpstream{
					ClusterKey:       mainUpstream.ClusterKey,
					UseClusterHeader: useClusterHeader,
					DefaultCluster:   defaultCluster,
				},
			}
			if op.PathMatchType != nil {
				rdcRoute.PathMatchType = string(*op.PathMatchType)
			}
			rdc.Routes[routeKey] = rdcRoute

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
			basePath := "/"
			if def.BasePath != nil && *def.BasePath != "" {
				basePath = *def.BasePath
			}
			endpoints := make([]models.Endpoint, 0, len(def.Upstreams))
			tlsExists := false
			plaintextExists := false
			for _, up := range def.Upstreams {
				parsedURL, err := url.Parse(up.Url)
				if err != nil {
					return nil, fmt.Errorf("invalid URL in upstream definition '%s': %w", def.Name, err)
				}
				port := ResolvePort(parsedURL)
				ep := models.Endpoint{Host: parsedURL.Hostname(), Port: port}
				if up.Weight != nil {
					ep.Weight = up.Weight
				}
				endpoints = append(endpoints, ep)
				if parsedURL.Scheme == "https" {
					tlsExists = true
				} else {
					plaintextExists = true
				}
			}
			// A single Envoy cluster has one transport socket, and the model carries one TLS bit
			// for the whole cluster (createWeightedCluster applies it to every endpoint). A weighted
			// definition that mixes https and non-https endpoints therefore cannot be represented —
			// the plaintext endpoints would be silently dialed over TLS. Reject it with a clear error
			// instead of collapsing to an ambiguous single flag. Uniform definitions (all https or all
			// plaintext) are unaffected: Enabled = tlsExists matches the previous "any https" result.
			if tlsExists && plaintextExists {
				return nil, fmt.Errorf("upstream definition '%s' mixes https and non-https endpoints; "+
					"all endpoints in a definition must use the same scheme", def.Name)
			}
			rdc.UpstreamClusters[defClusterKey] = &models.UpstreamCluster{
				Name:      def.Name,
				BasePath:  basePath,
				Endpoints: endpoints,
				TLS:       &models.UpstreamTLS{Enabled: tlsExists},
			}
		}
	}

	// Add sandbox upstream and update sandbox routes if present
	if hasSandbox {
		sbUpstream, err := t.addUpstreamCluster(rdc, "sandbox", apiData.Upstream.Sandbox, apiData.UpstreamDefinitions)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve sandbox upstream: %w", err)
		}

		sbAutoHostRewrite := true
		if apiData.Upstream.Sandbox.HostRewrite != nil && *apiData.Upstream.Sandbox.HostRewrite == api.Manual {
			sbAutoHostRewrite = false
		}

		// Update sandbox vhost routes to point to sandbox cluster. The route key must be
		// derived with the same header-match discriminator used when the routes were built
		// above, otherwise header-matched routes would not be found and re-pointed.
		for _, op := range apiData.Operations {
			discriminator := xds.HeaderMatchDiscriminator(routeHeaderMatches(op))
			routeKey := xds.GenerateRouteNameWithDiscriminator(string(op.Method), apiData.Context, apiData.Version, op.Path, effectiveSandboxVHost, discriminator)
			if r, exists := rdc.Routes[routeKey]; exists {
				r.Upstream.ClusterKey = sbUpstream.ClusterKey
				// Mirror main on sandbox routes: cluster_header lets a dynamic-endpoint policy
				// divert sandbox traffic, defaulting to the sandbox cluster when none does.
				r.Upstream.UseClusterHeader = useClusterHeader
				if useClusterHeader {
					// Use ClusterKey (the name Envoy knows the cluster by), not
					// EnvoyClusterName — see the main-cluster default above.
					r.Upstream.DefaultCluster = sbUpstream.ClusterKey
				} else {
					r.Upstream.DefaultCluster = ""
				}
				r.AutoHostRewrite = sbAutoHostRewrite
			}
		}
	}

	return rdc, nil
}

// splitVhosts parses a vhosts.main value into its individual production hostnames. Multiple
// hostnames may be provided separated by ";" (each serves the main upstream); surrounding
// whitespace is trimmed, empty entries are dropped, and duplicates are removed while preserving
// order. A single hostname (the common case) returns a one-element slice.
func splitVhosts(raw string) []string {
	parts := strings.Split(raw, ";")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

// routeHeaderMatches converts an operation's Gateway-API-style header matchers into the model form
// used for both the Envoy route match and the route-key discriminator. Returning a single canonical
// slice keeps the main route build and the sandbox patch loop in agreement on the route key.
func routeHeaderMatches(op api.Operation) []models.RouteHeaderMatch {
	if op.MatchHeaders == nil {
		return nil
	}
	matches := make([]models.RouteHeaderMatch, 0, len(*op.MatchHeaders))
	for _, h := range *op.MatchHeaders {
		headerType := "Exact"
		if h.Type != nil {
			headerType = string(*h.Type)
		}
		matches = append(matches, models.RouteHeaderMatch{
			Name:  h.Name,
			Value: h.Value,
			Type:  headerType,
		})
	}
	return matches
}

// collectAPIPolicies validates and collects API-level policies into SDK format.
// buildRouteTimeout applies operation-over-API precedence (per field) and returns a
// *models.RouteTimeout, or nil when neither level configured any timeout (so the global
// route timeout default applies).
func buildRouteTimeout(opTimeout, apiTimeout, opIdle, apiIdle *time.Duration) *models.RouteTimeout {
	timeout := opTimeout
	if timeout == nil {
		timeout = apiTimeout
	}
	idle := opIdle
	if idle == nil {
		idle = apiIdle
	}
	if timeout == nil && idle == nil {
		return nil
	}
	return &models.RouteTimeout{Timeout: timeout, IdleTimeout: idle}
}

func (t *RestAPITransformer) collectAPIPolicies(policies *[]api.Policy) map[string]policyenginev1.PolicyInstance {
	result := make(map[string]policyenginev1.PolicyInstance)
	if policies == nil {
		return result
	}
	for _, p := range *policies {
		resolved, err := config.ResolvePolicyVersion(t.policyDefinitions, t.latestVersions, p.Name, p.Version)
		if err != nil {
			slog.Error("Failed to resolve policy version for API-level policy", "policy_name", p.Name, "error", err)
			continue
		}
		result[p.Name] = convertAPIPolicyToSDK(p, policyv1alpha.LevelAPI, versionutil.MajorVersion(resolved))
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
			resolved, err := config.ResolvePolicyVersion(t.policyDefinitions, t.latestVersions, opPol.Name, opPol.Version)
			if err != nil {
				slog.Error("Failed to resolve operation-level policy version", "policy_name", opPol.Name, "error", err)
				continue
			}
			result = append(result, convertAPIPolicyToSDK(opPol, policyv1alpha.LevelRoute, versionutil.MajorVersion(resolved)))
		}
	}

	return result
}

// upstreamClusterResult holds the result of resolving and registering an upstream cluster.
type upstreamClusterResult struct {
	// ClusterKey is the internal key used in rdc.UpstreamClusters.
	ClusterKey string
	// EnvoyClusterName is the Envoy cluster name matching pkg/xds/translator.go's
	// sanitizeClusterName format ("cluster_<scheme>_<sanitized_host>").
	// This is the value Envoy knows the cluster by, so PE must use it for x-target-upstream.
	EnvoyClusterName string
	// BasePath is the URL path component of the upstream (e.g. "/anything/foo").
	BasePath string
}

// addUpstreamCluster resolves an upstream and adds it to the RuntimeDeployConfig.
func (t *RestAPITransformer) addUpstreamCluster(
	rdc *models.RuntimeDeployConfig,
	upstreamName string,
	up *api.Upstream,
	upstreamDefinitions *[]api.UpstreamDefinition,
) (*upstreamClusterResult, error) {
	rawURL, refBasePath, err := resolveUpstreamURL(upstreamName, up, upstreamDefinitions)
	if err != nil {
		return nil, err
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid %s upstream URL: %w", upstreamName, err)
	}
	if parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return nil, fmt.Errorf("invalid %s upstream URL: must include host and http/https scheme", upstreamName)
	}

	port := ResolvePort(parsedURL)
	// Direct URLs carry their base path in the path; a ref takes its base path solely from
	// the definition's basePath field (upstreamDefinitions URLs are host[:port] only).
	basePath := parsedURL.Path
	if refBasePath != nil {
		basePath = *refBasePath
	}
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

	return &upstreamClusterResult{
		ClusterKey:       clusterKey,
		EnvoyClusterName: sanitizeEnvoyClusterName(parsedURL.Host, parsedURL.Scheme),
		BasePath:         basePath,
	}, nil
}

// sanitizeEnvoyClusterName computes the Envoy cluster name from a URL host and scheme,
// matching the sanitizeClusterName logic in pkg/xds/translator.go.
func sanitizeEnvoyClusterName(host, scheme string) string {
	name := strings.ReplaceAll(host, ".", "_")
	name = strings.ReplaceAll(name, ":", "_")
	return "cluster_" + scheme + "_" + name
}

// resolveUpstreamURL resolves the URL from an upstream (direct URL or ref). For a ref it
// also returns the referenced definition's base path (from basePath, never the URL); for a
// direct URL the returned base-path pointer is nil, signalling the caller to use the URL path.
func resolveUpstreamURL(name string, up *api.Upstream, defs *[]api.UpstreamDefinition) (string, *string, error) {
	if up.Url != nil && strings.TrimSpace(*up.Url) != "" {
		return strings.TrimSpace(*up.Url), nil, nil
	}
	if up.Ref != nil && strings.TrimSpace(*up.Ref) != "" {
		refName := strings.TrimSpace(*up.Ref)
		if defs == nil {
			return "", nil, fmt.Errorf("upstream definition '%s' referenced but no definitions provided", refName)
		}
		for _, def := range *defs {
			if def.Name == refName {
				if len(def.Upstreams) == 0 || def.Upstreams[0].Url == "" {
					return "", nil, fmt.Errorf("upstream definition '%s' has no URLs", refName)
				}
				basePath := ""
				if def.BasePath != nil {
					basePath = *def.BasePath
				}
				return def.Upstreams[0].Url, &basePath, nil
			}
		}
		return "", nil, fmt.Errorf("upstream definition '%s' not found", refName)
	}
	return "", nil, fmt.Errorf("%s upstream has no URL or ref", name)
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
