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

// FR-012/FR-031: both bounds resolve to a finite value even when the artifact
// sets neither, so a connection can never outlive revocation indefinitely.
func TestWebSocketTransform_ConnectionBoundsAlwaysFinite(t *testing.T) {
	rdc, err := websocketTransformer().Transform(testWebSocketAPI(nil))
	require.NoError(t, err)

	for key, r := range rdc.Routes {
		require.NotNil(t, r.Timeout, "route %s has no timeout block", key)
		require.NotNil(t, r.Timeout.MaxStreamDuration, "route %s: maxConnectionDuration must never resolve to unlimited", key)
		assert.Greater(t, *r.Timeout.MaxStreamDuration, time.Duration(0))
		require.NotNil(t, r.Timeout.IdleTimeout, "route %s: idleTimeout must always resolve", key)
		assert.Greater(t, *r.Timeout.IdleTimeout, time.Duration(0))
	}
}

func TestWebSocketTransform_ArtifactConnectionSettingsWin(t *testing.T) {
	stored := testWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		spec.Connection = &api.WebSocketConnectionSettings{
			IdleTimeout:           strPtr("10s"),
			MaxConnectionDuration: strPtr("30s"),
		}
	})

	rdc, err := websocketTransformer().Transform(stored)
	require.NoError(t, err)

	for _, r := range rdc.Routes {
		assert.Equal(t, 10*time.Second, *r.Timeout.IdleTimeout)
		assert.Equal(t, 30*time.Second, *r.Timeout.MaxStreamDuration)
	}
}

// A value above the gateway's ceiling is rejected rather than clamped: silently
// lowering it would let an API author believe a bound applies that does not.
func TestWebSocketTransform_RejectsValueAboveGatewayBound(t *testing.T) {
	routerCfg := testRouterCfg()
	routerCfg.WebSocket.Downstream.MaxConnectionDurationMax = 24 * time.Hour
	transformer := NewWebSocketAPITransformer(routerCfg, &config.Config{}, map[string]models.PolicyDefinition{})

	stored := testWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		spec.Connection = &api.WebSocketConnectionSettings{
			MaxConnectionDuration: strPtr("240h"), // ten times the configured ceiling
		}
	})

	_, err := transformer.Transform(stored)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds the gateway's configured maximum")
}

// An unset ceiling (0) means the operator configured none, so nothing is
// enforced — distinct from a ceiling of zero duration.
func TestWebSocketTransform_NoCeilingConfiguredMeansNoCeiling(t *testing.T) {
	stored := testWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		spec.Connection = &api.WebSocketConnectionSettings{
			MaxConnectionDuration: strPtr("240h"),
		}
	})

	rdc, err := websocketTransformer().Transform(stored)
	require.NoError(t, err)
	for _, r := range rdc.Routes {
		assert.Equal(t, 240*time.Hour, *r.Timeout.MaxStreamDuration)
	}
}

func TestWebSocketTransform_UpstreamSchemeMapping(t *testing.T) {
	for _, tc := range []struct {
		name       string
		url        string
		wantTLS    bool
		wantPort   int
		wantPrefix string
	}{
		{"plaintext", "ws://chat-backend:8080/ws", false, 8080, "http://"},
		{"tls", "wss://chat-backend:8443/ws", true, 8443, "https://"},
		{"tls default port", "wss://chat-backend/ws", true, 443, "https://"},
		{"plaintext default port", "ws://chat-backend/ws", false, 80, "http://"},
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
				require.NotNil(t, uc.TLS)
				assert.Equal(t, tc.wantTLS, uc.TLS.Enabled, "ws maps to a plain cluster, wss to a TLS one")
			}
		})
	}
}

// FR-015/FR-016: each slot's pool bounds are read from its own timeout block.
func TestWebSocketTransform_UpstreamPoolBounds(t *testing.T) {
	maxConns := 500
	stored := testWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		spec.Upstream.Main = api.Upstream{
			Url: strPtr("wss://chat-backend:8443/ws"),
			Timeout: &api.UpstreamTimeout{
				Connect:               strPtr("5s"),
				IdleTimeout:           strPtr("120s"),
				MaxConnectionDuration: strPtr("30m"),
				MaxConnections:        &maxConns,
			},
		}
	})

	rdc, err := websocketTransformer().Transform(stored)
	require.NoError(t, err)

	for _, uc := range rdc.UpstreamClusters {
		require.NotNil(t, uc.ConnectTimeout)
		assert.Equal(t, 5*time.Second, *uc.ConnectTimeout)
		require.NotNil(t, uc.IdleTimeout)
		assert.Equal(t, 120*time.Second, *uc.IdleTimeout)
		require.NotNil(t, uc.MaxConnectionDuration)
		assert.Equal(t, 30*time.Minute, *uc.MaxConnectionDuration)
		require.NotNil(t, uc.MaxConnections)
		assert.Equal(t, uint32(500), *uc.MaxConnections)
	}
}

// Two WebSocketApis on one backend must not share a cluster: each cluster carries
// its own API's pool bounds, and a shared one would silently take another API's.
func TestWebSocketTransform_ClusterIsPerAPI(t *testing.T) {
	withMax := func(uuid string, maxConns int) *models.StoredConfig {
		stored := testWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
			spec.Upstream.Main.Timeout = &api.UpstreamTimeout{MaxConnections: &maxConns}
		})
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
	stored := testWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		spec.Upstream.Main = api.Upstream{
			Url:     strPtr("wss://chat-backend:8443/ws"),
			Timeout: &api.UpstreamTimeout{MaxConnections: &mainMax},
		}
		spec.Upstream.Sandbox = &api.Upstream{
			Url:     strPtr("ws://chat-sandbox:8080/ws"),
			Timeout: &api.UpstreamTimeout{MaxConnections: &sandboxMax},
		}
	})

	rdc, err := websocketTransformer().Transform(stored)
	require.NoError(t, err)

	assert.Len(t, rdc.UpstreamClusters, 2, "one cluster per slot")
	assert.Len(t, rdc.Routes, 2, "one channel on each of two slots")

	seen := map[uint32]bool{}
	for _, uc := range rdc.UpstreamClusters {
		require.NotNil(t, uc.MaxConnections)
		seen[*uc.MaxConnections] = true
	}
	assert.True(t, seen[500] && seen[5], "each slot keeps its own ceiling, got %v", seen)
}

func TestWebSocketTransform_RejectsNonWebSocketScheme(t *testing.T) {
	stored := testWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		spec.Upstream.Main = api.Upstream{Url: strPtr("https://chat-backend:8443/ws")}
	})

	_, err := websocketTransformer().Transform(stored)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ws/wss scheme")
}

func TestWebSocketTransform_RejectsUpstreamRef(t *testing.T) {
	stored := testWebSocketAPI(func(spec *api.WebSocketAPIConfigData) {
		spec.Upstream.Main = api.Upstream{Ref: strPtr("some-definition")}
	})

	_, err := websocketTransformer().Transform(stored)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ref is not supported")
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
