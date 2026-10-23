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
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

func websocketTransformer() *WebSocketAPITransformer {
	return NewWebSocketAPITransformer(testRouterCfg(), &config.Config{}, map[string]models.PolicyDefinition{})
}

func strPtr(s string) *string { return &s }

// testWebSocketAPI builds a minimal valid WebSocketApi artifact. Callers mutate
// the returned spec to exercise one variation at a time.
func testWebSocketAPI(mutate func(spec *api.WebSocketAPIConfigData)) *models.StoredConfig {
	spec := api.WebSocketAPIConfigData{
		DisplayName: "Chat",
		Version:     "v1.0",
		Context:     "/chat/$version",
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{Url: strPtr("ws://chat-backend:8080/ws")},
		},
		Channels: []api.WebSocketChannel{{Path: "/rooms/{roomId}"}},
	}
	if mutate != nil {
		mutate(&spec)
	}

	cfg := api.WebSocketAPI{
		ApiVersion: api.WebSocketAPIApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.WebSocketAPIKindWebSocketApi,
		Metadata:   api.Metadata{Name: "chat-websocket-v1.0"},
		Spec:       spec,
	}

	return &models.StoredConfig{
		UUID:          "ws-uuid",
		Kind:          models.KindWebSocketApi,
		Handle:        "chat-websocket-v1.0",
		Version:       "v1.0",
		Configuration: cfg,
	}
}

func TestWebSocketTransform_OneRoutePerChannel(t *testing.T) {
	stored := testWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		spec.Channels = []api.WebSocketChannel{
			{Path: "/rooms/{roomId}"},
			{Path: "/notifications"},
		}
	})

	rdc, err := websocketTransformer().Transform(stored)
	require.NoError(t, err)

	assert.Len(t, rdc.Routes, 2, "one route per channel on the main slot")
	for key, r := range rdc.Routes {
		assert.Equal(t, "GET", r.Method, "route %s: only a GET handshake is routed", key)
		assert.True(t, r.UpgradeWebSocket, "route %s: the channel route must opt into the upgrade", key)
	}
}

// A non-GET request to a channel path is not a handshake. The transformer must
// not emit a route for it, so such a request 404s rather than reaching the
// backend as an ordinary request.
func TestWebSocketTransform_OnlyGetIsRouted(t *testing.T) {
	rdc, err := websocketTransformer().Transform(testWebSocketAPI(nil))
	require.NoError(t, err)

	for key, r := range rdc.Routes {
		assert.Equal(t, "GET", r.Method, "route %s", key)
	}
}

// The idle timeout is left unset so the translator applies the gateway's route
// idle timeout, and the connection lifetime is capped only when the gateway sets
// router.websocket.max_connection_duration.
func TestWebSocketTransform_MaxConnectionDurationFromGateway(t *testing.T) {
	t.Run("unset leaves the lifetime uncapped", func(t *testing.T) {
		rdc, err := websocketTransformer().Transform(testWebSocketAPI(nil))
		require.NoError(t, err)
		for key, r := range rdc.Routes {
			assert.Nil(t, r.Timeout, "route %s: no timeout block when nothing is configured", key)
		}
	})

	t.Run("set caps every channel route", func(t *testing.T) {
		routerCfg := testRouterCfg()
		routerCfg.WebSocket.MaxConnectionDuration = 30 * time.Minute
		transformer := NewWebSocketAPITransformer(routerCfg, &config.Config{}, map[string]models.PolicyDefinition{})

		rdc, err := transformer.Transform(testWebSocketAPI(nil))
		require.NoError(t, err)
		require.NotEmpty(t, rdc.Routes)
		for key, r := range rdc.Routes {
			require.NotNil(t, r.Timeout, "route %s has no timeout block", key)
			require.NotNil(t, r.Timeout.MaxStreamDuration, "route %s", key)
			assert.Equal(t, 30*time.Minute, *r.Timeout.MaxStreamDuration)
			assert.Nil(t, r.Timeout.IdleTimeout, "route %s: idle timeout comes from the gateway route default", key)
		}
	})
}

func TestWebSocketTransform_UpstreamSchemeMapping(t *testing.T) {
	for _, tc := range []struct {
		name     string
		url      string
		wantTLS  bool
		wantPort int
	}{
		{"plaintext", "ws://chat-backend:8080/ws", false, 8080},
		{"tls", "wss://chat-backend:8443/ws", true, 8443},
		{"tls default port", "wss://chat-backend/ws", true, 443},
		{"plaintext default port", "ws://chat-backend/ws", false, 80},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stored := testWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
				spec.Upstream.Main = api.Upstream{Url: strPtr(tc.url)}
			})

			rdc, err := websocketTransformer().Transform(stored)
			require.NoError(t, err)
			require.Len(t, rdc.UpstreamClusters, 1)

			for _, uc := range rdc.UpstreamClusters {
				require.Len(t, uc.Endpoints, 1)
				assert.Equal(t, tc.wantPort, uc.Endpoints[0].Port)
				assert.Equal(t, "/ws", uc.BasePath, "an inline url's path is the backend base path")
				require.NotNil(t, uc.TLS)
				assert.Equal(t, tc.wantTLS, uc.TLS.Enabled, "ws maps to a plain cluster, wss to a TLS one")
			}
		})
	}
}

// wsDefinition builds an upstream definition for one or more targets.
func wsDefinition(name string, urls ...string) api.WebSocketUpstreamDefinition {
	def := api.WebSocketUpstreamDefinition{Name: name}
	for _, u := range urls {
		def.Upstreams = append(def.Upstreams, struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{Url: u})
	}
	return def
}

// withDefinitions puts defs on the spec and points main at the first.
func withDefinitions(spec *api.WebSocketAPIConfigData, defs ...api.WebSocketUpstreamDefinition) {
	spec.UpstreamDefinitions = &defs
	spec.Upstream.Main = api.Upstream{Ref: strPtr(defs[0].Name)}
}

// FR-015/FR-016: a referenced definition supplies the cluster's targets, base
// path, connect timeout, connection-count bounds and tls.
func TestWebSocketTransform_UpstreamDefinitionSettings(t *testing.T) {
	maxConns, w1, w2 := 500, 80, 20
	def := wsDefinition("chat-backend", "wss://chat-1:8443", "wss://chat-2:8443")
	def.Upstreams[0].Weight, def.Upstreams[1].Weight = &w1, &w2
	def.BasePath = strPtr("/ws")
	def.Timeout = &api.UpstreamTimeout{Connect: strPtr("5s")}
	maxPending := 10
	def.Connection = &api.WebSocketUpstreamConnection{
		MaxConnections:       &maxConns,
		MaxPendingHandshakes: &maxPending,
	}
	def.Tls = &map[string]interface{}{"identity": "chat-identity", "trustedCAs": []interface{}{"chat-ca"}}
	stored := testWebSocketAPI(func(spec *api.WebSocketAPIConfigData) { withDefinitions(spec, def) })

	rdc, err := websocketTransformer().Transform(stored)
	require.NoError(t, err)
	require.Len(t, rdc.UpstreamClusters, 1)

	uc, ok := rdc.UpstreamClusters["upstream_WebSocketApi_ws-uuid_chat-backend"]
	require.True(t, ok, "a definition's cluster uses the RestApi upstreamDefinition key, got %v", rdc.UpstreamClusters)
	assert.Equal(t, "/ws", uc.BasePath)
	require.Len(t, uc.Endpoints, 2)
	assert.Equal(t, 80, *uc.Endpoints[0].Weight)
	assert.Equal(t, 20, *uc.Endpoints[1].Weight)
	assert.Equal(t, 5*time.Second, *uc.ConnectTimeout)
	assert.Equal(t, uint32(500), *uc.MaxConnections)
	assert.Equal(t, uint32(10), *uc.MaxPendingRequests)
	require.NotNil(t, uc.TLS)
	assert.True(t, uc.TLS.Enabled)
	assert.Equal(t, "chat-identity", uc.TLS.IdentityName)
	assert.Equal(t, []string{"chat-ca"}, uc.TLS.TrustedCANames)
}

// An inline url carries no connection settings: its cluster takes the gateway's
// upstream defaults.
func TestWebSocketTransform_InlineURLUsesGatewayDefaults(t *testing.T) {
	routerCfg := testRouterCfg()
	routerCfg.WebSocket.Upstream.MaxConnectionsDefault = 50
	transformer := NewWebSocketAPITransformer(routerCfg, &config.Config{}, map[string]models.PolicyDefinition{})

	rdc, err := transformer.Transform(testWebSocketAPI(nil))
	require.NoError(t, err)
	for _, uc := range rdc.UpstreamClusters {
		assert.Equal(t, uint32(50), *uc.MaxConnections)
		assert.Nil(t, uc.ConnectTimeout)
	}
}

// A value above the gateway's ceiling is rejected rather than clamped: silently
// lowering it would let an API author believe a bound applies that does not.
func TestWebSocketTransform_RejectsValueAboveGatewayBound(t *testing.T) {
	routerCfg := testRouterCfg()
	routerCfg.WebSocket.Upstream.MaxConnectionsMax = 1000
	transformer := NewWebSocketAPITransformer(routerCfg, &config.Config{}, map[string]models.PolicyDefinition{})

	tooMany := 10000 // ten times the ceiling
	def := wsDefinition("chat-backend", "ws://chat-backend:8080")
	def.Connection = &api.WebSocketUpstreamConnection{MaxConnections: &tooMany}
	_, err := transformer.Transform(testWebSocketAPI(func(spec *api.WebSocketAPIConfigData) { withDefinitions(spec, def) }))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds the gateway's configured maximum")
}

// An unset ceiling (0) means the operator configured none, so nothing is
// enforced — distinct from a ceiling of zero connections.
func TestWebSocketTransform_NoCeilingConfiguredMeansNoCeiling(t *testing.T) {
	many := 1000000
	def := wsDefinition("chat-backend", "ws://chat-backend:8080")
	def.Connection = &api.WebSocketUpstreamConnection{MaxConnections: &many}
	rdc, err := websocketTransformer().Transform(testWebSocketAPI(func(spec *api.WebSocketAPIConfigData) { withDefinitions(spec, def) }))
	require.NoError(t, err)
	for _, uc := range rdc.UpstreamClusters {
		assert.Equal(t, uint32(1000000), *uc.MaxConnections)
	}
}

// A maxConnections beyond uint32 must be rejected rather than wrapped when it is
// narrowed to Envoy's threshold type — even with no gateway maximum configured.
func TestWebSocketTransform_RejectsMaxConnectionsAboveUint32(t *testing.T) {
	tooMany := int(math.MaxUint32) + 2 // would wrap to 1
	def := wsDefinition("chat-backend", "ws://chat-backend:8080")
	def.Connection = &api.WebSocketUpstreamConnection{MaxConnections: &tooMany}
	stored := testWebSocketAPI(func(spec *api.WebSocketAPIConfigData) { withDefinitions(spec, def) })

	_, err := websocketTransformer().Transform(stored)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds the largest supported value")
}

// Two WebSocketApis on one backend must not share a cluster: each cluster carries
// its own API's pool bounds, and a shared one would silently take another API's.
func TestWebSocketTransform_ClusterIsPerAPI(t *testing.T) {
	withMax := func(uuid string, maxConns int) *models.StoredConfig {
		def := wsDefinition("chat-backend", "ws://chat-backend:8080")
		def.Connection = &api.WebSocketUpstreamConnection{MaxConnections: &maxConns}
		stored := testWebSocketAPI(func(spec *api.WebSocketAPIConfigData) { withDefinitions(spec, def) })
		stored.UUID = uuid
		return stored
	}

	first, err := websocketTransformer().Transform(withMax("ws-uuid-a", 10))
	require.NoError(t, err)
	second, err := websocketTransformer().Transform(withMax("ws-uuid-b", 20))
	require.NoError(t, err)

	require.Len(t, first.UpstreamClusters, 1)
	require.Len(t, second.UpstreamClusters, 1)
	for keyA, a := range first.UpstreamClusters {
		for keyB, b := range second.UpstreamClusters {
			assert.NotEqual(t, keyA, keyB, "same backend, different APIs must yield different clusters")
			assert.Equal(t, uint32(10), *a.MaxConnections)
			assert.Equal(t, uint32(20), *b.MaxConnections)
		}
	}
	for _, route := range first.Routes {
		_, ok := first.UpstreamClusters[route.Upstream.ClusterKey]
		assert.True(t, ok, "route must point at this API's own cluster")
	}
}

// The two slots are bounded independently — saturating main must not constrain
// sandbox, and neither inherits the other's settings.
func TestWebSocketTransform_SandboxIsIndependent(t *testing.T) {
	mainMax, sandboxMax := 500, 5
	mainDef := wsDefinition("chat-main", "wss://chat-backend:8443")
	mainDef.Connection = &api.WebSocketUpstreamConnection{MaxConnections: &mainMax}
	sbDef := wsDefinition("chat-sandbox", "ws://chat-sandbox:8080")
	sbDef.Connection = &api.WebSocketUpstreamConnection{MaxConnections: &sandboxMax}
	stored := testWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		withDefinitions(spec, mainDef, sbDef)
		spec.Upstream.Sandbox = &api.Upstream{Ref: strPtr("chat-sandbox")}
	})

	rdc, err := websocketTransformer().Transform(stored)
	require.NoError(t, err)

	assert.Len(t, rdc.UpstreamClusters, 2, "one cluster per definition")
	assert.Len(t, rdc.Routes, 2, "one channel on each of two slots")

	seen := map[uint32]bool{}
	for _, uc := range rdc.UpstreamClusters {
		require.NotNil(t, uc.MaxConnections)
		seen[*uc.MaxConnections] = true
	}
	assert.True(t, seen[500] && seen[5], "each slot keeps its own ceiling, got %v", seen)
}

// Main and sandbox referencing one definition share its single cluster.
func TestWebSocketTransform_SlotsSharingADefinitionShareItsCluster(t *testing.T) {
	stored := testWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		withDefinitions(spec, wsDefinition("chat-backend", "ws://chat-backend:8080"))
		spec.Upstream.Sandbox = &api.Upstream{Ref: strPtr("chat-backend")}
	})

	rdc, err := websocketTransformer().Transform(stored)
	require.NoError(t, err)
	assert.Len(t, rdc.UpstreamClusters, 1)
	assert.Len(t, rdc.Routes, 2)
}

func TestWebSocketTransform_RejectsNonWebSocketScheme(t *testing.T) {
	stored := testWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		spec.Upstream.Main = api.Upstream{Url: strPtr("https://chat-backend:8443/ws")}
	})

	_, err := websocketTransformer().Transform(stored)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ws/wss scheme")
}

func TestWebSocketTransform_RejectsUnknownRef(t *testing.T) {
	stored := testWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		spec.Upstream.Main = api.Upstream{Ref: strPtr("no-such-definition")}
	})

	_, err := websocketTransformer().Transform(stored)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match any upstreamDefinitions entry")
}

// One cluster has one transport socket, so a definition cannot mix ws and wss.
func TestWebSocketTransform_RejectsMixedSchemeDefinition(t *testing.T) {
	stored := testWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		withDefinitions(spec, wsDefinition("chat-backend", "ws://chat-1:8080", "wss://chat-2:8443"))
	})

	_, err := websocketTransformer().Transform(stored)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mixes ws and wss")
}

func TestWebSocketTransform_RejectsWrongConfigurationType(t *testing.T) {
	_, err := websocketTransformer().Transform(&models.StoredConfig{
		UUID:          "u",
		Kind:          models.KindWebSocketApi,
		Configuration: api.GraphQLAPI{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a WebSocketAPI")
}
