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

package xds

import (
	"testing"
	"time"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	upstreamhttpv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// The HCM must declare the websocket upgrade and leave it disabled. Envoy
// requires the type to be named at the HCM before any route may enable it, so
// omitting the block would make every route-level opt-in silently inert; naming
// it with enabled=false is what makes the per-route switch the only one that
// matters, and what stops a REST/GraphQL/MCP/LLM route being upgraded into a
// tunnel whenever its backend would agree.
func TestTranslator_HCMDeclaresWebSocketUpgradeDisabled(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.Router = *routerCfg
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	lis, _, err := translator.createListener(nil, false, false)
	require.NoError(t, err)

	manager := extractHCM(t, lis)
	upgrades := manager.GetUpgradeConfigs()
	require.Len(t, upgrades, 1, "the HCM must declare exactly the websocket upgrade")
	assert.Equal(t, "websocket", upgrades[0].GetUpgradeType())
	assert.False(t, upgrades[0].GetEnabled().GetValue(),
		"the HCM-level upgrade must stay disabled so only a WebSocketApi route can enable it")
}

// RFC 8441 extended CONNECT stays off unless an operator turns it on: browsers
// fall back to a separate HTTP/1.1 connection on their own, so enabling it by
// default would expose a far less exercised path for no new capability.
func TestTranslator_HTTP2ConnectIsOptIn(t *testing.T) {
	logger := createTestLogger()

	t.Run("off by default", func(t *testing.T) {
		routerCfg := testRouterConfig()
		cfg := testConfig()
		cfg.Router = *routerCfg
		translator, err := NewTranslator(logger, routerCfg, nil, cfg)
		require.NoError(t, err)

		lis, _, err := translator.createListener(nil, false, false)
		require.NoError(t, err)

		assert.False(t, extractHCM(t, lis).GetHttp2ProtocolOptions().GetAllowConnect())
	})

	t.Run("enabled when configured", func(t *testing.T) {
		routerCfg := testRouterConfig()
		routerCfg.WebSocket.AllowHTTP2Connect = true
		cfg := testConfig()
		cfg.Router = *routerCfg
		translator, err := NewTranslator(logger, routerCfg, nil, cfg)
		require.NoError(t, err)

		lis, _, err := translator.createListener(nil, false, false)
		require.NoError(t, err)

		assert.True(t, extractHCM(t, lis).GetHttp2ProtocolOptions().GetAllowConnect())
	})
}

// Path canonicalization is listener-level and must keep applying once the
// websocket upgrade block is added to the same HCM — a handshake whose path is
// normalized differently from what policy was written against is the same
// route-desync bug as for any other request.
func TestTranslator_PathNormalizationSurvivesUpgradeConfig(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.Router = *routerCfg
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	lis, _, err := translator.createListener(nil, false, false)
	require.NoError(t, err)

	manager := extractHCM(t, lis)
	assert.True(t, manager.GetNormalizePath().GetValue(), "normalize_path")
	assert.True(t, manager.GetMergeSlashes(), "merge_slashes")
	require.NotEmpty(t, manager.GetUpgradeConfigs(), "guard: the upgrade block is present for this assertion to mean anything")
}

func TestApplyUpstreamConnectionBounds_NoBoundsLeavesClusterUntouched(t *testing.T) {
	c := &cluster.Cluster{Name: "c"}
	require.NoError(t, applyUpstreamConnectionBounds(c, &models.UpstreamCluster{}))

	assert.Nil(t, c.CircuitBreakers, "a kind that sets no bounds must not gain a circuit breaker")
	assert.Empty(t, c.TypedExtensionProtocolOptions,
		"typed_extension_protocol_options changes protocol inference, so it must not appear unasked")
}

func TestApplyUpstreamConnectionBounds_MaxConnectionsSetsCircuitBreaker(t *testing.T) {
	maxConns := uint32(500)
	c := &cluster.Cluster{Name: "c"}
	require.NoError(t, applyUpstreamConnectionBounds(c, &models.UpstreamCluster{MaxConnections: &maxConns}))

	require.NotNil(t, c.CircuitBreakers)
	require.Len(t, c.CircuitBreakers.Thresholds, 1)
	th := c.CircuitBreakers.Thresholds[0]
	assert.Equal(t, corev3.RoutingPriority_DEFAULT, th.GetPriority())
	assert.Equal(t, uint32(500), th.GetMaxConnections().GetValue())
	// Pending is bounded so a saturated backend rejects instead of queueing behind
	// Envoy's default of 1024, but never to 0: every handshake is briefly pending
	// while its new upstream connection opens.
	require.NotNil(t, th.GetMaxPendingRequests())
	assert.Equal(t, uint32(500), th.GetMaxPendingRequests().GetValue())
}

func TestApplyUpstreamConnectionBounds_PoolTimeouts(t *testing.T) {
	idle := 120 * time.Second
	maxDuration := 30 * time.Minute
	c := &cluster.Cluster{Name: "c"}

	require.NoError(t, applyUpstreamConnectionBounds(c, &models.UpstreamCluster{
		IdleTimeout:           &idle,
		MaxConnectionDuration: &maxDuration,
	}))

	raw, ok := c.TypedExtensionProtocolOptions["envoy.extensions.upstreams.http.v3.HttpProtocolOptions"]
	require.True(t, ok, "pool timeouts travel as HttpProtocolOptions")

	var opts upstreamhttpv3.HttpProtocolOptions
	require.NoError(t, raw.UnmarshalTo(&opts))

	assert.Equal(t, idle, opts.GetCommonHttpProtocolOptions().GetIdleTimeout().AsDuration())
	assert.Equal(t, maxDuration, opts.GetCommonHttpProtocolOptions().GetMaxConnectionDuration().AsDuration())

	// Setting the options struct at all disables Envoy's protocol inference, so
	// the HTTP version has to be stated. It must be HTTP/1.1: an upgrade is an
	// HTTP/1.1 mechanism, and an HTTP/2 upstream would need RFC 8441 support on
	// the backend, which this phase does not support.
	require.NotNil(t, opts.GetExplicitHttpConfig(), "protocol must be stated explicitly once these options are set")
	assert.NotNil(t, opts.GetExplicitHttpConfig().GetHttpProtocolOptions(), "upstream must be pinned to HTTP/1.1")
}
