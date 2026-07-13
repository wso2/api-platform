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
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils/clusterkey"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils/upstreamref"
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
	mainUpstreamInfo := mainUpstream.UpstreamInfo()

	// Determine vhosts to create routes for.
	// Sandbox is active when a sandbox upstream is configured via either url or ref,
	// or when any operation carries a per-op sandbox ref.
	apiSandboxHasContent := upstreamref.HasContent(apiData.Upstream.Sandbox)
	hasSandbox := upstreamref.SandboxActive(apiData.Upstream.Sandbox, apiData.Operations)

	// Check if dynamic cluster selection should be used. Enabled whenever the API has named
	// upstream definitions (so a policy can select one) OR a sandbox upstream (so a policy can
	// redirect between the API's own main/sandbox slots). Must mirror pkg/xds/translator.go's
	// useClusterHeader computation exactly — that's what determines whether Envoy's route uses
	// cluster_header routing; if this RDC (which feeds the policy engine's default-cluster
	// fallback) disagrees, Envoy expects a header the policy engine never sets.
	hasUpstreamDefinitions := apiData.UpstreamDefinitions != nil && len(*apiData.UpstreamDefinitions) > 0
	useClusterHeader := hasUpstreamDefinitions || hasSandbox
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

	// Per-op sandbox routes carry no HostRewrite; inherit the API-level sandbox
	// setting when present, else the main setting (matches the xDS path).
	sandboxAutoHostRewrite := mainAutoHostRewrite
	if apiSandboxHasContent {
		sandboxAutoHostRewrite = true
		if apiData.Upstream.Sandbox.HostRewrite != nil && *apiData.Upstream.Sandbox.HostRewrite == api.Manual {
			sandboxAutoHostRewrite = false
		}
	}

	// Build routes and policy chains for each operation
	for i, op := range apiData.Operations {
		// Operation-level resilience overrides API-level (per field); nil leaves the
		// global route timeout default in effect.
		opTimeout, opIdleTimeout, err := xds.ResolveResilience(op.Resilience)
		if err != nil {
			return nil, fmt.Errorf("invalid resilience for operation %s %s: %w", op.EffectiveMethod(), op.EffectivePath(), err)
		}
		routeTimeout := buildRouteTimeout(opTimeout, apiTimeout, opIdleTimeout, apiIdleTimeout)

		// Resolve the effective matching criteria (simple top-level form or the richer match
		// block) once per operation. Header matchers and their discriminator are vhost-
		// independent; the discriminator keeps the route key unique across operations that
		// share method/path/vhost but match on different headers (e.g. multiple Gateway-API
		// HTTPRoute rules on the same path).
		method := op.EffectiveMethod()
		opPath := op.EffectivePath()
		pathMatchType := op.EffectivePathMatchType()
		headerMatches := routeHeaderMatches(op)
		discriminator := xds.HeaderMatchDiscriminator(headerMatches)

		mainSlot := routeSlot{
			clusterKey:       mainUpstream.ClusterKey,
			useClusterHeader: useClusterHeader,
			defaultCluster:   defaultCluster,
			autoHostRewrite:  mainAutoHostRewrite,
			defaultUpstream:  mainUpstreamInfo,
		}
		// The sandbox slot starts as the main slot; only autoHostRewrite differs
		// until a per-op sandbox ref overrides it below (API-level sandbox routes
		// are re-pointed by the sandbox patch after this loop).
		sandboxSlot := mainSlot
		sandboxSlot.autoHostRewrite = sandboxAutoHostRewrite

		if op.Upstream != nil {
			if op.Upstream.Main != nil {
				if err := mainSlot.applyPerOpRef("main", cfg.Kind, cfg.UUID, method, opPath, op.Upstream.Main.Ref, apiData.UpstreamDefinitions); err != nil {
					return nil, err
				}
			}
			if op.Upstream.Sandbox != nil {
				if err := sandboxSlot.applyPerOpRef("sandbox", cfg.Kind, cfg.UUID, method, opPath, op.Upstream.Sandbox.Ref, apiData.UpstreamDefinitions); err != nil {
					return nil, err
				}
			}
		}

		vhosts := append([]string{}, mainVhosts...)
		// Add the sandbox vhost only when this op has sandbox config (API-level
		// fallback or a per-op override); otherwise it would route to the main cluster.
		sbIdx := -1
		if apiSandboxHasContent || (op.Upstream != nil && op.Upstream.Sandbox != nil) {
			vhosts = append(vhosts, effectiveSandboxVHost)
			sbIdx = len(vhosts) - 1
		}

		for vi, vhost := range vhosts {
			routeKey := xds.GenerateRouteNameWithDiscriminator(method, apiData.Context, apiData.Version, opPath, vhost, discriminator)

			// The sandbox vhost, when present, is appended last; dispatch on position
			// so equal vhost strings cannot misroute.
			slot := mainSlot
			if vi == sbIdx {
				slot = sandboxSlot
			}

			// Build route. Default is this route's own upstream (the slot's) — the single
			// field exposed to the policy engine as the route's compiled-in upstream.
			routeInfo := slot.defaultUpstream
			rdcRoute := &models.Route{
				Method:          method,
				Path:            xds.ConstructFullPath(apiData.Context, apiData.Version, opPath),
				OperationPath:   opPath,
				Vhost:           vhost,
				AutoHostRewrite: slot.autoHostRewrite,
				MatchHeaders:    headerMatches,
				PathMatchType:   pathMatchType,
				Order:           i,
				Timeout:         routeTimeout,
				Upstream: models.RouteUpstream{
					ClusterKey:       slot.clusterKey,
					UseClusterHeader: slot.useClusterHeader,
					DefaultCluster:   slot.defaultCluster,
					Default:          &routeInfo,
				},
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
			defClusterKey := clusterkey.DefinitionName(cfg.Kind, cfg.UUID, def.Name)
			// Base path comes solely from the explicit basePath field; upstreamDefinitions
			// URLs are host[:port] only (a path in the URL is rejected during validation).
			basePath := "/"
			if def.BasePath != nil && *def.BasePath != "" {
				basePath = *def.BasePath
			}
			defConnectTimeout, err := definitionConnectTimeout(&def)
			if err != nil {
				return nil, fmt.Errorf("upstream definition '%s': %w", def.Name, err)
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
				Name:           def.Name,
				BasePath:       basePath,
				Endpoints:      endpoints,
				TLS:            &models.UpstreamTLS{Enabled: tlsExists},
				ConnectTimeout: defConnectTimeout,
			}
		}
	}

	// Add sandbox upstream and update sandbox routes if present.
	// API-level sandbox is optional when per-op sandbox overrides exist.
	if apiSandboxHasContent {
		sbUpstream, err := t.addUpstreamCluster(rdc, "sandbox", apiData.Upstream.Sandbox, apiData.UpstreamDefinitions)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve sandbox upstream: %w", err)
		}
		sbUpstreamInfo := sbUpstream.UpstreamInfo()

		sbAutoHostRewrite := true
		if apiData.Upstream.Sandbox.HostRewrite != nil && *apiData.Upstream.Sandbox.HostRewrite == api.Manual {
			sbAutoHostRewrite = false
		}

		// Update sandbox vhost routes to point to sandbox cluster, except ops with
		// their own per-op sandbox override (already wired in the main loop). The route
		// key must be derived with the same header-match discriminator used when the
		// routes were built above, otherwise header-matched routes would not be found
		// and re-pointed.
		for _, op := range apiData.Operations {
			if op.Upstream != nil && op.Upstream.Sandbox != nil {
				continue
			}
			discriminator := xds.HeaderMatchDiscriminator(routeHeaderMatches(op))
			routeKey := xds.GenerateRouteNameWithDiscriminator(op.EffectiveMethod(), apiData.Context, apiData.Version, op.EffectivePath(), effectiveSandboxVHost, discriminator)
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
				// This route belongs to the sandbox slot — its own default upstream is
				// the sandbox's, not main's.
				routeSbInfo := sbUpstreamInfo
				r.Upstream.Default = &routeSbInfo
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
	headers := op.EffectiveHeaders()
	if len(headers) == 0 {
		return nil
	}
	matches := make([]models.RouteHeaderMatch, 0, len(headers))
	for _, h := range headers {
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
	// EnvoyClusterName is the name Envoy knows the cluster by, used by the policy
	// engine for the x-target-upstream header. It is always set equal to ClusterKey.
	EnvoyClusterName string
	// BasePath is the URL path component of the upstream (e.g. "/anything/foo").
	BasePath string
	// URL is the resolved upstream origin (scheme://host[:port], no path — see BasePath).
	URL string
}

// UpstreamInfo converts the resolved cluster result into the shared wire shape
// carried to the policy engine (sdk/core/policyengine.UpstreamInfo).
func (r *upstreamClusterResult) UpstreamInfo() policyenginev1.UpstreamInfo {
	return policyenginev1.UpstreamInfo{
		ClusterName: r.EnvoyClusterName,
		URL:         r.URL,
		BasePath:    r.BasePath,
	}
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

	// The connect timeout can only come from a referenced upstreamDefinition
	// (direct-URL upstreams have no timeout field). Resolve it here so the RDC->Envoy
	// translation applies it to this cluster instead of falling back to the global default.
	var connectTimeout *time.Duration
	if up != nil && up.Ref != nil && strings.TrimSpace(*up.Ref) != "" {
		ct, terr := definitionConnectTimeout(lookupUpstreamDefinition(*up.Ref, upstreamDefinitions))
		if terr != nil {
			return nil, fmt.Errorf("%s upstream: %w", upstreamName, terr)
		}
		connectTimeout = ct
	}

	// URL-stable cluster name so a URL edit updates the same cluster instead of
	// renaming it. ClusterKey and EnvoyClusterName are intentionally identical.
	clusterKey := clusterkey.HashedName(upstreamName, rdc.Metadata.UUID)

	rdc.UpstreamClusters[clusterKey] = &models.UpstreamCluster{
		BasePath: basePath,
		Endpoints: []models.Endpoint{{
			Host: parsedURL.Hostname(),
			Port: port,
		}},
		TLS:            &models.UpstreamTLS{Enabled: parsedURL.Scheme == "https"},
		ConnectTimeout: connectTimeout,
	}

	// ClusterKey and EnvoyClusterName must stay identical or the default upstream
	// path yields a 503 because Envoy cannot find the selected cluster.
	return &upstreamClusterResult{
		ClusterKey:       clusterKey,
		EnvoyClusterName: clusterKey,
		BasePath:         basePath,
		URL:              fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host),
	}, nil
}

// routeSlot carries the per-vhost route settings for one operation.
type routeSlot struct {
	clusterKey       string
	useClusterHeader bool
	defaultCluster   string
	autoHostRewrite  bool
	defaultUpstream  policyenginev1.UpstreamInfo
}

// applyPerOpRef points the slot at the referenced definition's cluster, keeping
// cluster_header on with that cluster as the default so a dynamic-endpoint policy
// can still steer the operation. autoHostRewrite keeps the API-level setting;
// per-op targets are ref-only with no HostRewrite field.
func (s *routeSlot) applyPerOpRef(env, kind, apiID, method, path, ref string, upstreamDefinitions *[]api.UpstreamDefinition) error {
	def, err := upstreamref.FindByName(ref, upstreamDefinitions)
	if err != nil {
		return fmt.Errorf("per-op %s upstream for %s %s: %w", env, method, path, err)
	}
	if len(def.Upstreams) == 0 || def.Upstreams[0].Url == "" {
		return fmt.Errorf("per-op %s upstream for %s %s: upstream definition '%s' has no URLs configured", env, method, path, strings.TrimSpace(ref))
	}
	defClusterKey := clusterkey.DefinitionName(kind, apiID, def.Name)
	basePath := "/"
	if def.BasePath != nil && *def.BasePath != "" {
		basePath = *def.BasePath
	}
	s.clusterKey = defClusterKey
	s.useClusterHeader = true
	s.defaultCluster = defClusterKey
	// This route's own compiled-in upstream is the referenced definition —
	// exposed to the policy engine as the route's default upstream.
	s.defaultUpstream = policyenginev1.UpstreamInfo{
		ClusterName: defClusterKey,
		URL:         strings.TrimSpace(def.Upstreams[0].Url),
		BasePath:    basePath,
	}
	return nil
}

// lookupUpstreamDefinition returns the upstream definition named ref (after trimming
// whitespace), or nil if defs is nil or no definition matches.
func lookupUpstreamDefinition(ref string, defs *[]api.UpstreamDefinition) *api.UpstreamDefinition {
	if defs == nil {
		return nil
	}
	name := strings.TrimSpace(ref)
	for i := range *defs {
		if strings.TrimSpace((*defs)[i].Name) == name {
			return &(*defs)[i]
		}
	}
	return nil
}

// definitionConnectTimeout parses an upstream definition's timeout.connect into a
// *time.Duration. A nil definition, absent timeout, or empty string yields (nil, nil)
// meaning "use the router's global default". A malformed or non-positive value is an
// error so a bad config fails fast instead of silently falling back to the default.
func definitionConnectTimeout(def *api.UpstreamDefinition) (*time.Duration, error) {
	if def == nil || def.Timeout == nil || def.Timeout.Connect == nil {
		return nil, nil
	}
	s := strings.TrimSpace(*def.Timeout.Connect)
	if s == "" {
		return nil, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return nil, fmt.Errorf("invalid connect timeout %q: %w", s, err)
	}
	if d <= 0 {
		return nil, fmt.Errorf("connect timeout must be positive, got %v", d)
	}
	return &d, nil
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
		// Resolve via the shared upstreamref helper and return the definition's
		// basePath so the caller rewrites the upstream path correctly.
		def, err := upstreamref.FindByName(refName, defs)
		if err != nil {
			return "", nil, err
		}
		if len(def.Upstreams) == 0 || def.Upstreams[0].Url == "" {
			return "", nil, fmt.Errorf("upstream definition '%s' has no URLs configured", refName)
		}
		basePath := ""
		if def.BasePath != nil {
			basePath = *def.BasePath
		}
		return def.Upstreams[0].Url, &basePath, nil
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
