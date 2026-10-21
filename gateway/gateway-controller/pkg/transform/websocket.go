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
//     upgraded connection. Idle timeout and max stream duration are what bound
//     it, and both always resolve to a finite value.
package transform

import (
	"fmt"
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

// Last-resort bounds, used when the gateway config carries no default for a
// setting (a zero value).
//
// FR-031 requires every WebSocketApi to end up with a finite idle timeout and a
// finite maximum connection duration, because a connection is authorized once at
// its handshake and never re-checked — an unbounded connection is an unbounded
// window for access that has since been revoked. Deriving that guarantee purely
// from configuration would make it only as good as the config: a deployment that
// omitted these keys would either refuse every WebSocketApi or, worse, hand Envoy
// "no limit". Falling back to a value here makes the invariant structural, and
// still honours any limit the operator does set.
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

	// Resolve the downstream connection bounds once: every channel of this API
	// shares them, and both always resolve to a finite value even when the
	// artifact sets neither.
	routeTimeout, err := t.resolveDownstreamTimeout(apiData.Connection)
	if err != nil {
		return nil, err
	}

	apiPolicies := t.collectAPIPolicies(apiData.Policies)

	mainVhost := t.routerConfig.VHosts.Main.Default
	mainUpstream, err := t.addWebSocketUpstreamCluster(rdc, "main", &apiData.Upstream.Main)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve main upstream: %w", err)
	}

	if err := t.addChannelRoutes(rdc, apiData, mainUpstream, &apiData.Upstream.Main, mainVhost, apiPolicies, routeTimeout); err != nil {
		return nil, err
	}

	// Sandbox is active when a sandbox upstream is configured. A WebSocketApi
	// only supports a direct url (see validateWebSocketUpstream), never a ref.
	hasSandbox := apiData.Upstream.Sandbox != nil &&
		apiData.Upstream.Sandbox.Url != nil && strings.TrimSpace(*apiData.Upstream.Sandbox.Url) != ""

	if hasSandbox {
		sandboxVhost := t.routerConfig.VHosts.Sandbox.Default
		if sandboxVhost == mainVhost {
			return nil, fmt.Errorf("sandbox upstream is configured but resolves to the same vhost %q as the main upstream; configure distinct vhosts to avoid route conflicts", sandboxVhost)
		}

		sbUpstream, err := t.addWebSocketUpstreamCluster(rdc, "sandbox", apiData.Upstream.Sandbox)
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
		timeoutForRoute := *routeTimeout

		rdc.Routes[routeKey] = &models.Route{
			Method:          handshakeMethod,
			Path:            xds.ConstructFullPath(apiData.Context, apiData.Version, channelPath),
			OperationPath:   channelPath,
			PathMatchType:   pathMatchTypeForChannel(channelPath),
			Vhost:           vhost,
			AutoHostRewrite: autoHostRewrite,
			Order:           i,
			Timeout:         &timeoutForRoute,
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

// resolveDownstreamTimeout turns the artifact's connection block into the route
// timeouts, filling in the gateway-wide defaults for anything it omits and
// rejecting anything above the gateway-wide bounds.
//
// The ordinary route Timeout is deliberately left nil: it does not apply once the
// stream is upgraded, so setting it would only constrain the handshake itself,
// which the gateway's own defaults already cover.
func (t *WebSocketAPITransformer) resolveDownstreamTimeout(conn *api.WebSocketConnectionSettings) (*models.RouteTimeout, error) {
	limits := t.routerConfig.WebSocket.Downstream

	idle, err := resolveBoundedDuration(
		connIdleTimeout(conn),
		orFallback(limits.IdleTimeoutDefault, fallbackWebSocketIdleTimeout),
		limits.IdleTimeoutMax,
		"connection.idleTimeout",
	)
	if err != nil {
		return nil, err
	}

	maxDuration, err := resolveBoundedDuration(
		connMaxConnectionDuration(conn),
		orFallback(limits.MaxConnectionDurationDefault, fallbackWebSocketMaxConnectionDuration),
		limits.MaxConnectionDurationMax,
		"connection.maxConnectionDuration",
	)
	if err != nil {
		return nil, err
	}

	// Guard the invariant the whole design leans on rather than trusting config:
	// a zero here would hand Envoy "no limit", turning every connection on this
	// API into an unbounded window for access granted at the handshake.
	if maxDuration == nil || *maxDuration <= 0 {
		return nil, fmt.Errorf("connection.maxConnectionDuration resolved to no limit; a WebSocketApi must always have a finite maximum connection duration")
	}

	return &models.RouteTimeout{
		IdleTimeout:       idle,
		MaxStreamDuration: maxDuration,
	}, nil
}

func connIdleTimeout(conn *api.WebSocketConnectionSettings) *string {
	if conn == nil {
		return nil
	}
	return conn.IdleTimeout
}

func connMaxConnectionDuration(conn *api.WebSocketConnectionSettings) *string {
	if conn == nil {
		return nil
	}
	return conn.MaxConnectionDuration
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

// addWebSocketUpstreamCluster registers the cluster for one upstream slot.
//
// It exists instead of reusing addUpstreamCluster because that helper rejects any
// scheme but http/https, and a WebSocketApi's upstream is ws/wss. The mapping is
// direct — ws is a plain TCP cluster, wss a TLS one — because the backend leg is
// an ordinary HTTP/1.1 connection that carries an Upgrade header; the WebSocket
// part is the route's business, not the cluster's.
func (t *WebSocketAPITransformer) addWebSocketUpstreamCluster(
	rdc *models.RuntimeDeployConfig,
	slotName string,
	up *api.Upstream,
) (*upstreamClusterResult, error) {
	// Check ref before url: someone who wrote a ref did configure an upstream, so
	// "ref is not supported" explains the problem, where "url is required" would
	// just look wrong to them.
	if up != nil && up.Ref != nil && strings.TrimSpace(*up.Ref) != "" {
		return nil, fmt.Errorf("%s upstream: ref is not supported for WebSocketApi; use a direct url", slotName)
	}
	if up == nil || up.Url == nil || strings.TrimSpace(*up.Url) == "" {
		return nil, fmt.Errorf("%s upstream URL is required", slotName)
	}

	parsedURL, err := url.Parse(strings.TrimSpace(*up.Url))
	if err != nil {
		return nil, fmt.Errorf("invalid %s upstream URL: %w", slotName, err)
	}
	if parsedURL.Host == "" || (parsedURL.Scheme != "ws" && parsedURL.Scheme != "wss") {
		return nil, fmt.Errorf("invalid %s upstream URL: must include host and ws/wss scheme", slotName)
	}

	secure := parsedURL.Scheme == "wss"
	// Envoy knows this cluster by its transport, not by the application protocol
	// riding on it, so the cluster is named and built as http/https.
	transportScheme := "http"
	if secure {
		transportScheme = "https"
	}

	port := resolveWebSocketPort(parsedURL, secure)

	basePath := parsedURL.Path
	if basePath == "" {
		basePath = "/"
	}

	connectTimeout, err := parseOptionalDuration(
		upstreamSlotDuration(up, func(ut *api.UpstreamTimeout) *string { return ut.Connect }),
		fmt.Sprintf("upstream.%s.timeout.connect", slotName),
	)
	if err != nil {
		return nil, err
	}

	limits := t.routerConfig.WebSocket.Upstream

	idleTimeout, err := resolveBoundedDuration(
		upstreamSlotDuration(up, func(ut *api.UpstreamTimeout) *string { return ut.IdleTimeout }),
		orFallback(limits.IdleTimeoutDefault, fallbackWebSocketIdleTimeout), limits.IdleTimeoutMax,
		fmt.Sprintf("upstream.%s.timeout.idleTimeout", slotName))
	if err != nil {
		return nil, err
	}

	maxConnectionDuration, err := resolveBoundedDuration(
		upstreamSlotDuration(up, func(ut *api.UpstreamTimeout) *string { return ut.MaxConnectionDuration }),
		orFallback(limits.MaxConnectionDurationDefault, fallbackWebSocketMaxConnectionDuration), limits.MaxConnectionDurationMax,
		fmt.Sprintf("upstream.%s.timeout.maxConnectionDuration", slotName))
	if err != nil {
		return nil, err
	}

	var maxConnectionsRaw *int
	if up.Timeout != nil {
		maxConnectionsRaw = up.Timeout.MaxConnections
	}
	maxConnections, err := resolveBoundedConnections(maxConnectionsRaw, limits.MaxConnectionsDefault, limits.MaxConnectionsMax,
		fmt.Sprintf("upstream.%s.timeout.maxConnections", slotName))
	if err != nil {
		return nil, err
	}

	clusterKey := fmt.Sprintf("upstream_%s_%s_%d", slotName, parsedURL.Hostname(), port)

	rdc.UpstreamClusters[clusterKey] = &models.UpstreamCluster{
		BasePath: basePath,
		Endpoints: []models.Endpoint{{
			Host: parsedURL.Hostname(),
			Port: port,
		}},
		TLS:                   &models.UpstreamTLS{Enabled: secure},
		ConnectTimeout:        connectTimeout,
		IdleTimeout:           idleTimeout,
		MaxConnectionDuration: maxConnectionDuration,
		MaxConnections:        maxConnections,
	}

	return &upstreamClusterResult{
		ClusterKey:       clusterKey,
		EnvoyClusterName: sanitizeEnvoyClusterName(parsedURL.Host, transportScheme),
		BasePath:         basePath,
		URL:              fmt.Sprintf("%s://%s", transportScheme, parsedURL.Host),
	}, nil
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

// upstreamSlotDuration reads one duration field off a slot's timeout block,
// returning nil when the slot or the field is absent.
func upstreamSlotDuration(up *api.Upstream, pick func(*api.UpstreamTimeout) *string) *string {
	if up == nil || up.Timeout == nil {
		return nil
	}
	return pick(up.Timeout)
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
