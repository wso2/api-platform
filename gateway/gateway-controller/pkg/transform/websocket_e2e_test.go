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

package transform_test

import (
	"io"
	"log/slog"
	"testing"
	"time"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	upstreamhttpv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/transform"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

// The YAML an operator actually submits to POST /websocket-apis.
const websocketArtifactYAML = `
apiVersion: gateway.api-platform.wso2.com/v1
kind: WebSocketApi
metadata:
  name: chat-websocket-v1.0
spec:
  displayName: Chat
  version: v1.0
  context: /chat/$version
  upstream:
    main:
      url: ws://mock-websocket-backend:8080/ws
  channels:
    - path: /rooms
    - path: /notifications
`

// TestWebSocketGatewayFirstChain walks the whole gateway-first path for a
// WebSocketApi — the YAML an operator POSTs, through the registered parser and
// validator, the transformer, and the xDS translator — and asserts on the Envoy
// config that comes out the far end.
//
// Each stage has unit tests of its own; this exists because the stages are wired
// together by registration (RegisterKindDeployParser/RegisterKindConfigValidator,
// the transform Registry, the translator's transformer map) rather than by direct
// calls. A missing registration breaks deployment at runtime while every
// individual unit test still passes.
func TestWebSocketGatewayFirstChain(t *testing.T) {
	// Stage 1: the submitted YAML parses into the deployable shape. (That this
	// kind's parser and validator are registered with the generic deployment
	// path is asserted in pkg/utils, where those registries live.)
	var parsed api.WebSocketAPI
	require.NoError(t, config.NewParser().Parse([]byte(websocketArtifactYAML), "application/yaml", &parsed))
	require.Equal(t, "chat-websocket-v1.0", parsed.Metadata.Name)

	stored := &models.StoredConfig{
		UUID:          "ws-e2e-uuid",
		Kind:          models.KindWebSocketApi,
		Handle:        parsed.Metadata.Name,
		Version:       parsed.Spec.Version,
		DisplayName:   parsed.Spec.DisplayName,
		Configuration: parsed,
	}

	// Stage 3 + 4: the registry dispatches the kind to its transformer, and the
	// translator renders the result as Envoy config.
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	routerCfg := websocketE2ERouterConfig()
	systemCfg := &config.Config{Router: *routerCfg}

	registry := transform.NewRegistry(
		transform.NewRestAPITransformer(routerCfg, systemCfg, map[string]models.PolicyDefinition{}),
		nil,
		nil,
		nil,
		transform.NewWebSocketAPITransformer(routerCfg, systemCfg, map[string]models.PolicyDefinition{}),
	)

	translator, err := xds.NewTranslator(logger, routerCfg, nil, systemCfg)
	require.NoError(t, err)
	translator.SetTransformers(map[string]models.ConfigTransformer{models.KindWebSocketApi: registry})

	resources, err := translator.TranslateConfigs([]*models.StoredConfig{stored}, "")
	require.NoError(t, err, "a validated WebSocketApi must translate to Envoy config")

	routes := websocketRoutesFor(t, resources)
	require.Len(t, routes, 2, "one Envoy route per channel")

	for _, r := range routes {
		action := r.GetRoute()
		require.NotNil(t, action, "route %s has no route action", r.GetName())

		// The upgrade is what makes this a WebSocket route at all.
		upgrades := action.GetUpgradeConfigs()
		require.Len(t, upgrades, 1, "route %s must carry exactly the websocket upgrade", r.GetName())
		assert.Equal(t, "websocket", upgrades[0].GetUpgradeType())
		assert.True(t, upgrades[0].GetEnabled().GetValue(),
			"route %s must enable the upgrade the HCM leaves disabled", r.GetName())

		// The gateway's route idle timeout reaches the channel route; with no
		// router.websocket.max_connection_duration configured, the lifetime is
		// left uncapped.
		assert.Equal(t, int64(240), action.GetIdleTimeout().GetSeconds())
		assert.Nil(t, action.GetMaxStreamDuration(), "route %s: no lifetime cap unless the gateway sets one", r.GetName())

		// A retry policy on an upgraded stream has nothing to retry.
		assert.Nil(t, action.GetRetryPolicy(), "route %s must not carry a retry policy", r.GetName())

		// Only a GET handshake routes here.
		assertMatchesGetOnly(t, r)
	}
}

// A non-WebSocketApi route must never come out upgradable, whatever its backend
// would accept — this is the property that keeps an upgrade from being smuggled
// through a REST route.
func TestWebSocketUpgradeIsNotAppliedToOtherKinds(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	routerCfg := websocketE2ERouterConfig()
	systemCfg := &config.Config{Router: *routerCfg}

	restAPIYAML := `
apiVersion: gateway.api-platform.wso2.com/v1
kind: RestApi
metadata:
  name: books-api-v1.0
spec:
  displayName: Books
  version: v1.0
  context: /books/$version
  upstream:
    main:
      url: http://books-backend:8080
  operations:
    - method: GET
      path: /list
`
	var parsed api.RestAPI
	require.NoError(t, config.NewParser().Parse([]byte(restAPIYAML), "application/yaml", &parsed))

	stored := &models.StoredConfig{
		UUID:          "rest-uuid",
		Kind:          models.KindRestApi,
		Handle:        parsed.Metadata.Name,
		Version:       "v1.0",
		DisplayName:   "Books",
		Configuration: parsed,
	}

	registry := transform.NewRegistry(
		transform.NewRestAPITransformer(routerCfg, systemCfg, map[string]models.PolicyDefinition{}),
		nil, nil, nil,
		transform.NewWebSocketAPITransformer(routerCfg, systemCfg, map[string]models.PolicyDefinition{}),
	)
	translator, err := xds.NewTranslator(logger, routerCfg, nil, systemCfg)
	require.NoError(t, err)
	translator.SetTransformers(map[string]models.ConfigTransformer{models.KindRestApi: registry})

	resources, err := translator.TranslateConfigs([]*models.StoredConfig{stored}, "")
	require.NoError(t, err)

	routes := websocketRoutesFor(t, resources)
	require.NotEmpty(t, routes, "guard: the REST route must exist for this assertion to mean anything")
	for _, r := range routes {
		assert.Empty(t, r.GetRoute().GetUpgradeConfigs(),
			"REST route %s must not be upgradable", r.GetName())
	}
}

// resilience written in the artifact reaches the Envoy route: the channel's own
// value where it sets one, otherwise the API's.
func TestWebSocketResilienceReachesEnvoy(t *testing.T) {
	const artifact = `
apiVersion: gateway.api-platform.wso2.com/v1
kind: WebSocketApi
metadata:
  name: chat-resilience-v1.0
spec:
  displayName: Chat
  version: v1.0
  context: /chat/$version
  resilience:
    timeout: 10s
    idleTimeout: 120s
  upstream:
    main:
      url: ws://mock-websocket-backend:8080/ws
  channels:
    - path: /rooms
    - path: /notifications
      resilience:
        idleTimeout: 30m
`
	var parsed api.WebSocketAPI
	require.NoError(t, config.NewParser().Parse([]byte(artifact), "application/yaml", &parsed))
	stored := &models.StoredConfig{
		UUID: "ws-resilience-uuid", Kind: models.KindWebSocketApi, Handle: parsed.Metadata.Name,
		Version: parsed.Spec.Version, DisplayName: parsed.Spec.DisplayName, Configuration: parsed,
	}

	routerCfg := websocketE2ERouterConfig()
	systemCfg := &config.Config{Router: *routerCfg}
	registry := transform.NewRegistry(nil, nil, nil, nil,
		transform.NewWebSocketAPITransformer(routerCfg, systemCfg, map[string]models.PolicyDefinition{}))
	translator, err := xds.NewTranslator(slog.New(slog.NewTextHandler(io.Discard, nil)), routerCfg, nil, systemCfg)
	require.NoError(t, err)
	translator.SetTransformers(map[string]models.ConfigTransformer{models.KindWebSocketApi: registry})
	resources, err := translator.TranslateConfigs([]*models.StoredConfig{stored}, "")
	require.NoError(t, err)

	idle := map[string]int64{}
	for _, r := range websocketRoutesFor(t, resources) {
		action := r.GetRoute()
		assert.Equal(t, int64(10), action.GetTimeout().GetSeconds(), "route %s handshake timeout", r.GetName())
		idle[r.GetMatch().GetPath()] = action.GetIdleTimeout().GetSeconds()
	}
	assert.Equal(t, int64(120), idle["/chat/v1.0/rooms"])
	assert.Equal(t, int64(1800), idle["/chat/v1.0/notifications"])
}

// router.websocket.max_connection_duration reaches Envoy as the API cluster's
// per-stream max_stream_duration and never as a route max_stream_duration: a
// route carrying one stops Envoy enforcing the route timeout, so a backend that
// never answered the upgrade held the handshake until the idle timeout (408)
// instead of failing it at resilience.timeout (504).
func TestWebSocketMaxConnectionDurationKeepsHandshakeTimeout(t *testing.T) {
	const artifact = `
apiVersion: gateway.api-platform.wso2.com/v1
kind: WebSocketApi
metadata:
  name: chat-lifetime-v1.0
spec:
  displayName: Chat
  version: v1.0
  context: /chat/$version
  resilience:
    timeout: 3s
  upstreamDefinitions:
    - name: chat-backend
      basePath: /ws
      upstreams:
        - url: ws://mock-websocket-backend:8080
  upstream:
    main:
      ref: chat-backend
  channels:
    - path: /rooms
`
	var parsed api.WebSocketAPI
	require.NoError(t, config.NewParser().Parse([]byte(artifact), "application/yaml", &parsed))
	stored := &models.StoredConfig{
		UUID: "ws-lifetime-uuid", Kind: models.KindWebSocketApi, Handle: parsed.Metadata.Name,
		Version: parsed.Spec.Version, DisplayName: parsed.Spec.DisplayName, Configuration: parsed,
	}

	routerCfg := websocketE2ERouterConfig()
	routerCfg.WebSocket.MaxConnectionDuration = 25 * time.Second
	systemCfg := &config.Config{Router: *routerCfg}
	registry := transform.NewRegistry(nil, nil, nil, nil,
		transform.NewWebSocketAPITransformer(routerCfg, systemCfg, map[string]models.PolicyDefinition{}))
	translator, err := xds.NewTranslator(slog.New(slog.NewTextHandler(io.Discard, nil)), routerCfg, nil, systemCfg)
	require.NoError(t, err)
	translator.SetTransformers(map[string]models.ConfigTransformer{models.KindWebSocketApi: registry})
	resources, err := translator.TranslateConfigs([]*models.StoredConfig{stored}, "")
	require.NoError(t, err)

	routes := websocketRoutesFor(t, resources)
	require.Len(t, routes, 1)
	action := routes[0].GetRoute()
	assert.Equal(t, int64(3), action.GetTimeout().GetSeconds(), "the handshake timeout reaches the route")
	assert.Nil(t, action.GetMaxStreamDuration(), "the lifetime cap must not be a route max_stream_duration")

	var wsClusters []*cluster.Cluster
	for _, res := range resources[resource.ClusterType] {
		if c, ok := res.(*cluster.Cluster); ok && c.GetName() == action.GetCluster() {
			wsClusters = append(wsClusters, c)
		}
	}
	require.Len(t, wsClusters, 1, "the channel route's cluster %q is translated", action.GetCluster())

	raw, ok := wsClusters[0].GetTypedExtensionProtocolOptions()["envoy.extensions.upstreams.http.v3.HttpProtocolOptions"]
	require.True(t, ok, "the WebSocketApi cluster carries HttpProtocolOptions")
	var opts upstreamhttpv3.HttpProtocolOptions
	require.NoError(t, raw.UnmarshalTo(&opts))
	assert.Equal(t, int64(25), opts.GetCommonHttpProtocolOptions().GetMaxStreamDuration().GetSeconds())
	assert.NotNil(t, opts.GetExplicitHttpConfig().GetHttpProtocolOptions(),
		"the backend is still reached over HTTP/1.1, which a WebSocket upgrade needs")
}

func websocketE2ERouterConfig() *config.RouterConfig {
	return &config.RouterConfig{
		ListenerPort: 8080,
		// The translator builds the shared HTTP listener, which includes the Lua
		// request-transformation filter, so the script has to be resolvable from
		// this package's directory.
		LuaScriptPath: "../../lua/request_transformation.lua",
		VHosts: config.VHostsConfig{
			Main:    config.VHostEntry{Default: "localhost"},
			Sandbox: config.VHostEntry{Default: "sandbox.localhost"},
		},
		Upstream: config.RouterUpstream{
			Timeouts: config.UpstreamTimeouts{
				RouteTimeoutMs: 60000,
				// Not the 5m default, so the route assertion proves the gateway's
				// route idle timeout is what reaches a channel route.
				RouteIdleTimeoutMs: 240000,
				ConnectTimeoutMs:   5000,
			},
		},
	}
}

// websocketRoutesFor collects every route across the translated virtual hosts.
func websocketRoutesFor(t *testing.T, resources map[resource.Type][]types.Resource) []*route.Route {
	t.Helper()
	var out []*route.Route
	for _, res := range resources[resource.RouteType] {
		routeConfig, ok := res.(*route.RouteConfiguration)
		if !ok {
			continue
		}
		for _, vh := range routeConfig.VirtualHosts {
			for _, r := range vh.Routes {
				// Skip the gateway's own health/readiness routes, which are
				// direct responses rather than proxied routes, and its catch-all
				// for unknown-path handshakes, which enables the upgrade but can
				// only answer 404 (its cluster is never defined; the xds package
				// tests assert that).
				if r.GetRoute() == nil || r.GetName() == constants.GatewayNoAPIFoundUpgradeRouteName {
					continue
				}
				out = append(out, r)
			}
		}
	}
	return out
}

// assertMatchesGetOnly checks the route's header matchers pin the method to GET:
// a channel serves a handshake, and any other method on that path is not one.
func assertMatchesGetOnly(t *testing.T, r *route.Route) {
	t.Helper()
	for _, h := range r.GetMatch().GetHeaders() {
		if h.GetName() == ":method" {
			assert.Equal(t, "GET", h.GetStringMatch().GetExact(),
				"route %s must match the GET handshake only", r.GetName())
			return
		}
	}
	t.Fatalf("route %s has no :method matcher, so it would route any method", r.GetName())
}
