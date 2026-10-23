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

// Package transform — WebSocketApi.
//
// A WebSocketApi differs from every other kind here in what a "request" is. The
// gateway sees exactly one HTTP exchange per connection: the GET handshake that
// asks to upgrade. After Envoy answers 101 the stream is an opaque byte pipe, so
// everything the gateway can decide — routing, auth, rate limiting, which policy
// chain runs — is decided once, at the handshake, and nothing after it.
//
// Three consequences shape this transformer:
//
//   - One route per channel per upstream slot, matching GET only. A channel is a
//     path clients connect to; a non-GET request to the same path is not a
//     handshake and must not route.
//   - Routes opt into the upgrade individually (Route.UpgradeWebSocket). The HCM
//     disables upgrades globally, so a REST or GraphQL route cannot be turned
//     into a tunnel even if its backend would accept one.
//   - The ordinary route timeout is left alone, because it does not end an
//     upgraded connection. The gateway's route idle timeout bounds it, and the
//     optional router.websocket.max_connection_duration caps its lifetime.
package transform

import (
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	policyv1alpha "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
)

// handshakeMethod is the only method routed for a channel. RFC 6455 opens a
// WebSocket with a GET carrying Upgrade: websocket; any other method on the same
// path is an ordinary request that this API does not serve.
const handshakeMethod = "GET"

// Last-resort upstream bounds, used when the gateway config carries no upstream
// default for a setting (a zero value), so an upstream cluster is never left
// with "no limit" by an omitted config key.
const (
	fallbackWebSocketIdleTimeout           = 5 * time.Minute
	fallbackWebSocketMaxConnectionDuration = 1 * time.Hour
)

// WebSocketAPITransformer converts WebSocketApi artifacts into RuntimeDeployConfigs.
type WebSocketAPITransformer struct {
	routerConfig      *config.RouterConfig
	systemConfig      *config.Config
	policyDefinitions map[string]models.PolicyDefinition
	latestVersions    map[string]string
}

// NewWebSocketAPITransformer creates a new WebSocketAPITransformer.
func NewWebSocketAPITransformer(routerConfig *config.RouterConfig, systemConfig *config.Config, policyDefinitions map[string]models.PolicyDefinition) *WebSocketAPITransformer {
	return &WebSocketAPITransformer{
		routerConfig:      routerConfig,
		systemConfig:      systemConfig,
		policyDefinitions: policyDefinitions,
		latestVersions:    config.BuildLatestVersionIndex(policyDefinitions),
	}
}

// Transform converts a StoredConfig holding a WebSocketApi into a
// RuntimeDeployConfig with one route per channel per active upstream slot.
func (t *WebSocketAPITransformer) Transform(cfg *models.StoredConfig) (*models.RuntimeDeployConfig, error) {
	websocketCfg, ok := cfg.Configuration.(api.WebSocketAPI)
	if !ok {
		return nil, fmt.Errorf("configuration is not a WebSocketAPI")
	}
	apiData := websocketCfg.Spec

	if len(apiData.Channels) == 0 {
		return nil, fmt.Errorf("WebSocketApi has no channels")
	}

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

	// Every channel of this API shares the same client-facing bounds: the
	// gateway's route idle timeout, which the translator applies because the route
	// leaves IdleTimeout unset, and router.websocket.max_connection_duration when
	// the operator sets one. Unset, a connection's lifetime is not capped.
	var routeTimeout *models.RouteTimeout
	if maxDuration := t.routerConfig.WebSocket.MaxConnectionDuration; maxDuration > 0 {
		routeTimeout = &models.RouteTimeout{MaxStreamDuration: &maxDuration}
	}

	apiPolicies := t.collectAPIPolicies(apiData.Policies)

	mainVhost := t.routerConfig.VHosts.Main.Default
	mainUpstream, err := t.addWebSocketUpstreamCluster(rdc, "main", &apiData.Upstream.Main, apiData.UpstreamDefinitions)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve main upstream: %w", err)
	}

	if err := t.addChannelRoutes(rdc, apiData, mainUpstream, &apiData.Upstream.Main, mainVhost, apiPolicies, routeTimeout); err != nil {
		return nil, err
	}

	// Sandbox is active when a sandbox upstream is configured, by url or by ref.
	sb := apiData.Upstream.Sandbox
	hasSandbox := sb != nil &&
		((sb.Url != nil && strings.TrimSpace(*sb.Url) != "") || (sb.Ref != nil && strings.TrimSpace(*sb.Ref) != ""))

	if hasSandbox {
		sandboxVhost := t.routerConfig.VHosts.Sandbox.Default
		if sandboxVhost == mainVhost {
			return nil, fmt.Errorf("sandbox upstream is configured but resolves to the same vhost %q as the main upstream; configure distinct vhosts to avoid route conflicts", sandboxVhost)
		}

		sbUpstream, err := t.addWebSocketUpstreamCluster(rdc, "sandbox", apiData.Upstream.Sandbox, apiData.UpstreamDefinitions)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve sandbox upstream: %w", err)
		}

		if err := t.addChannelRoutes(rdc, apiData, sbUpstream, apiData.Upstream.Sandbox, sandboxVhost, apiPolicies, routeTimeout); err != nil {
			return nil, err
		}
	}

	return rdc, nil
}

// addChannelRoutes emits one route per channel for a single upstream slot, each
// with that channel's own policy chain (API-level policies plus the channel's).
func (t *WebSocketAPITransformer) addChannelRoutes(
	rdc *models.RuntimeDeployConfig,
	apiData api.WebSocketAPIConfigData,
	upstream *upstreamClusterResult,
	slot *api.Upstream,
	vhost string,
	apiPolicies []policyenginev1.PolicyInstance,
	routeTimeout *models.RouteTimeout,
) error {
	upstreamInfo := upstream.UpstreamInfo()

	autoHostRewrite := true
	if slot.HostRewrite != nil && *slot.HostRewrite == api.UpstreamHostRewriteManual {
		autoHostRewrite = false
	}

	for i, channel := range apiData.Channels {
		channelPath := strings.TrimSpace(channel.Path)

		// The route key must follow the "METHOD|PATH|VHOST" convention
		// (xds.GenerateRouteName): translator.go groups Envoy routes into virtual
		// hosts by splitting the route name on "|" and reading the vhost from it,
		// so an ad-hoc key would silently vanish from every virtual host.
		routeKey := xds.GenerateRouteName(handshakeMethod, apiData.Context, apiData.Version, channelPath, vhost)
		if _, exists := rdc.Routes[routeKey]; exists {
			// Duplicate channel paths are rejected at validation time; reaching
			// here means something upstream of this let one through, and silently
			// overwriting would drop a channel's policies while leaving it listed
			// in the artifact.
			return fmt.Errorf("duplicate route for channel path %q on vhost %q", channelPath, vhost)
		}

		chain := t.buildPolicyChain(apiPolicies, channel.Policies)
		injected := utils.InjectSystemPolicies(chain, t.systemConfig, nil)

		// A per-route copy: the translator may set slot-specific fields on it, and
		// sharing one pointer across routes would make that leak between them.
		var timeoutForRoute *models.RouteTimeout
		if routeTimeout != nil {
			copied := *routeTimeout
			timeoutForRoute = &copied
		}

		rdc.Routes[routeKey] = &models.Route{
			Method:          handshakeMethod,
			Path:            xds.ConstructFullPath(apiData.Context, apiData.Version, channelPath),
			OperationPath:   channelPath,
			PathMatchType:   pathMatchTypeForChannel(channelPath),
			Vhost:           vhost,
			AutoHostRewrite: autoHostRewrite,
			Order:           i,
			Timeout:         timeoutForRoute,
			// This route, and only this route, accepts the upgrade.
			UpgradeWebSocket: true,
			Upstream: models.RouteUpstream{
				ClusterKey: upstream.ClusterKey,
				Default:    &upstreamInfo,
			},
		}
		rdc.PolicyChains[routeKey] = sdkChainToModel(injected)
	}

	return nil
}

// pathMatchTypeForChannel picks the match type for a channel path. A path with a
// {parameter} segment is a template the router has to match structurally; a plain
// path matches exactly, so that /rooms does not also catch /rooms/42.
func pathMatchTypeForChannel(channelPath string) string {
	if strings.Contains(channelPath, "{") {
		return "PathPrefix"
	}
	return "Exact"
}

// resolveBoundedDuration resolves one duration setting: the artifact's value when
// it sets one, otherwise the gateway default — and rejects a value above the
// gateway's ceiling instead of clamping it, so an operator's bound cannot be
// exceeded without the deploy failing.
func resolveBoundedDuration(raw *string, defaultValue, maxValue time.Duration, field string) (*time.Duration, error) {
	value := defaultValue

	if raw != nil && strings.TrimSpace(*raw) != "" {
		parsed, err := time.ParseDuration(strings.TrimSpace(*raw))
		if err != nil {
			return nil, fmt.Errorf("invalid %s %q: %w", field, *raw, err)
		}
		if parsed <= 0 {
			return nil, fmt.Errorf("%s must be greater than zero", field)
		}
		value = parsed
	}

	if value <= 0 {
		return nil, nil
	}
	if maxValue > 0 && value > maxValue {
		return nil, fmt.Errorf("%s %s exceeds the gateway's configured maximum of %s", field, value, maxValue)
	}

	return &value, nil
}

// orFallback returns configured when it is set, otherwise fallback. Used for the
// connection bounds whose invariant must hold whether or not config supplies a
// default.
func orFallback(configured, fallback time.Duration) time.Duration {
	if configured > 0 {
		return configured
	}
	return fallback
}

// resolveBoundedConnections resolves a concurrent-connection ceiling the same way
// resolveBoundedDuration resolves a duration. A nil result means "leave Envoy's
// own default alone".
func resolveBoundedConnections(raw *int, defaultValue, maxValue uint32, field string) (*uint32, error) {
	value := defaultValue

	if raw != nil {
		if *raw < 1 {
			return nil, fmt.Errorf("%s must be at least 1", field)
		}
		// Checked before narrowing: uint32() of a larger int wraps silently.
		if uint64(*raw) > math.MaxUint32 {
			return nil, fmt.Errorf("%s %d exceeds the largest supported value of %d", field, *raw, uint32(math.MaxUint32))
		}
		value = uint32(*raw)
	}

	if value == 0 {
		return nil, nil
	}
	if maxValue > 0 && value > maxValue {
		return nil, fmt.Errorf("%s %d exceeds the gateway's configured maximum of %d", field, value, maxValue)
	}

	return &value, nil
}

// addWebSocketUpstreamCluster registers the cluster for one upstream slot: a
// direct ws/wss url, which uses the gateway-wide upstream connection defaults, or
// a ref to one of the API's upstream definitions, which carries its own targets,
// basePath, connect and idle timeouts, connection-pool bounds and tls.
//
// It exists instead of reusing addUpstreamCluster because that helper rejects any
// scheme but http/https, and a WebSocketApi's upstream is ws/wss. The mapping is
// direct — ws is a plain TCP cluster, wss a TLS one — because the backend leg is
// an ordinary HTTP/1.1 connection that carries an Upgrade header; the WebSocket
// part is the route's business, not the cluster's.
//
// The cluster carries this API's connection-pool bounds, so it is keyed by the API
// rather than by the backend: the translator merges clusters by name, and two
// WebSocketApis pointing at one backend would otherwise share a cluster whose
// bounds came from whichever was translated last. A definition's cluster uses the
// RestApi upstreamDefinition key, so main and sandbox referencing one definition
// share one cluster.
func (t *WebSocketAPITransformer) addWebSocketUpstreamCluster(
	rdc *models.RuntimeDeployConfig,
	slotName string,
	up *api.Upstream,
	defs *[]api.WebSocketUpstreamDefinition,
) (*upstreamClusterResult, error) {
	if up == nil {
		return nil, fmt.Errorf("%s upstream is required", slotName)
	}
	if up.Ref != nil && strings.TrimSpace(*up.Ref) != "" {
		def := lookupWebSocketUpstreamDefinition(*up.Ref, defs)
		if def == nil {
			return nil, fmt.Errorf("%s upstream: ref %q does not match any upstreamDefinitions entry", slotName, strings.TrimSpace(*up.Ref))
		}
		clusterKey := "upstream_" + rdc.Metadata.Kind + "_" + rdc.Metadata.UUID + "_" + SanitizeUpstreamDefinitionName(def.Name)
		return t.addWebSocketCluster(rdc, clusterKey, def.Name, def.BasePath, def.Upstreams, def.Timeout, def.Connection, def.Tls,
			fmt.Sprintf("upstreamDefinitions[%s]", def.Name))
	}
	if up.Url == nil || strings.TrimSpace(*up.Url) == "" {
		return nil, fmt.Errorf("%s upstream url or ref is required", slotName)
	}

	parsedURL, err := url.Parse(strings.TrimSpace(*up.Url))
	if err != nil {
		return nil, fmt.Errorf("invalid %s upstream URL: %w", slotName, err)
	}
	var basePath *string
	if parsedURL.Path != "" {
		basePath = &parsedURL.Path
	}
	origin := parsedURL.Scheme + "://" + parsedURL.Host
	targets := []struct {
		Url    string `json:"url" yaml:"url"`
		Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
	}{{Url: origin}}
	clusterKey := "upstream_" + rdc.Metadata.Kind + "_" + rdc.Metadata.UUID + "_" + slotName
	return t.addWebSocketCluster(rdc, clusterKey, "", basePath, targets, nil, nil, nil, "upstream."+slotName)
}

// addWebSocketCluster builds one upstream cluster from its targets and optional
// definition settings. Idle timeout and connection-pool values the definition omits resolve to the
// gateway-wide upstream defaults; values above the gateway maximums are rejected.
func (t *WebSocketAPITransformer) addWebSocketCluster(
	rdc *models.RuntimeDeployConfig,
	clusterKey, defName string,
	basePathOverride *string,
	targets []struct {
		Url    string `json:"url" yaml:"url"`
		Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
	},
	timeout *api.WebSocketUpstreamTimeout,
	conn *api.WebSocketUpstreamConnection,
	tls *map[string]interface{},
	field string,
) (*upstreamClusterResult, error) {
	if len(targets) == 0 {
		return nil, fmt.Errorf("%s: at least one upstream target is required", field)
	}

	endpoints := make([]models.Endpoint, 0, len(targets))
	var first *url.URL
	secure, plain := false, false
	for _, target := range targets {
		parsedURL, err := url.Parse(strings.TrimSpace(target.Url))
		if err != nil {
			return nil, fmt.Errorf("invalid %s upstream URL: %w", field, err)
		}
		if parsedURL.Host == "" || (parsedURL.Scheme != "ws" && parsedURL.Scheme != "wss") {
			return nil, fmt.Errorf("invalid %s upstream URL: must include host and ws/wss scheme", field)
		}
		isSecure := parsedURL.Scheme == "wss"
		if isSecure {
			secure = true
		} else {
			plain = true
		}
		endpoints = append(endpoints, models.Endpoint{
			Host:   parsedURL.Hostname(),
			Port:   resolveWebSocketPort(parsedURL, isSecure),
			Weight: target.Weight,
		})
		if first == nil {
			first = parsedURL
		}
	}
	// One cluster has one transport socket, so a definition cannot mix ws and wss.
	if secure && plain {
		return nil, fmt.Errorf("%s mixes ws and wss targets; all targets must use the same scheme", field)
	}

	// Envoy knows this cluster by its transport, not by the application protocol
	// riding on it, so the cluster is named and built as http/https.
	transportScheme := "http"
	if secure {
		transportScheme = "https"
	}

	basePath := "/"
	if basePathOverride != nil && *basePathOverride != "" {
		basePath = *basePathOverride
	}

	var rawConnect, rawIdle *string
	if timeout != nil {
		rawConnect, rawIdle = timeout.Connect, timeout.IdleTimeout
	}
	connectTimeout, err := parseOptionalDuration(rawConnect, field+".timeout.connect")
	if err != nil {
		return nil, err
	}

	limits := t.routerConfig.WebSocket.Upstream
	var rawMaxDuration *string
	var rawMaxConnections *int
	if conn != nil {
		rawMaxDuration, rawMaxConnections = conn.MaxConnectionDuration, conn.MaxConnections
	}
	idleTimeout, err := resolveBoundedDuration(rawIdle,
		orFallback(limits.IdleTimeoutDefault, fallbackWebSocketIdleTimeout), limits.IdleTimeoutMax,
		field+".timeout.idleTimeout")
	if err != nil {
		return nil, err
	}
	maxConnectionDuration, err := resolveBoundedDuration(rawMaxDuration,
		orFallback(limits.MaxConnectionDurationDefault, fallbackWebSocketMaxConnectionDuration), limits.MaxConnectionDurationMax,
		field+".connection.maxConnectionDuration")
	if err != nil {
		return nil, err
	}
	maxConnections, err := resolveBoundedConnections(rawMaxConnections, limits.MaxConnectionsDefault, limits.MaxConnectionsMax,
		field+".connection.maxConnections")
	if err != nil {
		return nil, err
	}

	rdc.UpstreamClusters[clusterKey] = &models.UpstreamCluster{
		Name:                  defName,
		BasePath:              basePath,
		Endpoints:             endpoints,
		TLS:                   upstreamTLSFromParams(tls, secure),
		ConnectTimeout:        connectTimeout,
		IdleTimeout:           idleTimeout,
		MaxConnectionDuration: maxConnectionDuration,
		MaxConnections:        maxConnections,
	}

	return &upstreamClusterResult{
		ClusterKey:       clusterKey,
		EnvoyClusterName: sanitizeEnvoyClusterName(first.Host, transportScheme),
		BasePath:         basePath,
		URL:              fmt.Sprintf("%s://%s", transportScheme, first.Host),
	}, nil
}

// lookupWebSocketUpstreamDefinition returns the definition named ref (after
// trimming whitespace), or nil when none matches.
func lookupWebSocketUpstreamDefinition(ref string, defs *[]api.WebSocketUpstreamDefinition) *api.WebSocketUpstreamDefinition {
	if defs == nil {
		return nil
	}
	ref = strings.TrimSpace(ref)
	for i := range *defs {
		if strings.TrimSpace((*defs)[i].Name) == ref {
			return &(*defs)[i]
		}
	}
	return nil
}

// resolveWebSocketPort returns the explicit port, or the default for the scheme.
func resolveWebSocketPort(u *url.URL, secure bool) int {
	if p := u.Port(); p != "" {
		// url.Parse has already validated the port is numeric.
		var port int
		_, _ = fmt.Sscanf(p, "%d", &port)
		if port > 0 {
			return port
		}
	}
	if secure {
		return 443
	}
	return 80
}

// parseOptionalDuration parses a duration that has no gateway-wide default behind
// it — absent means "leave Envoy's own default alone" rather than "substitute
// ours", which is why it does not go through resolveBoundedDuration.
func parseOptionalDuration(raw *string, field string) (*time.Duration, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(*raw))
	if err != nil {
		return nil, fmt.Errorf("invalid %s %q: %w", field, *raw, err)
	}
	if parsed <= 0 {
		return nil, fmt.Errorf("%s must be greater than zero", field)
	}
	return &parsed, nil
}

// collectAPIPolicies resolves the API-level policy list into SDK policy instances.
func (t *WebSocketAPITransformer) collectAPIPolicies(policies *[]api.Policy) []policyenginev1.PolicyInstance {
	return resolvePolicyInstances(t.policyDefinitions, t.latestVersions, policies, policyv1alpha.LevelAPI)
}

// buildPolicyChain composes a channel's chain: the API-level policies, then the
// channel's own. Channel policies come second so a channel can add to, and
// override within, what the API declares.
//
// A channel resolves at LevelRoute: a channel is this kind's per-route scope, the
// same position an operation occupies for a RestApi.
func (t *WebSocketAPITransformer) buildPolicyChain(apiPolicies []policyenginev1.PolicyInstance, channelPolicies *[]api.Policy) []policyenginev1.PolicyInstance {
	channelInstances := resolvePolicyInstances(t.policyDefinitions, t.latestVersions, channelPolicies, policyv1alpha.LevelRoute)

	chain := make([]policyenginev1.PolicyInstance, 0, len(apiPolicies)+len(channelInstances))
	chain = append(chain, apiPolicies...)
	chain = append(chain, channelInstances...)
	return chain
}
