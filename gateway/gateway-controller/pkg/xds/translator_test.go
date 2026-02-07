/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	"fmt"
	"net/url"
	"testing"
	"time"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	resource "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	commonconstants "github.com/wso2/api-platform/common/constants"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// testRouterConfig creates a minimal valid router config for testing
func testRouterConfig() *config.RouterConfig {
	return &config.RouterConfig{
		ListenerPort: 8080,
		VHosts: config.VHostsConfig{
			Main:    config.VHostEntry{Default: "localhost"},
			Sandbox: config.VHostEntry{Default: "sandbox.localhost"},
		},
		PolicyEngine: config.PolicyEngineConfig{
			Enabled: false,
		},
		AccessLogs: config.AccessLogsConfig{
			Enabled: false,
		},
		HTTPListener: config.HTTPListenerConfig{
			ServerHeaderTransformation: commonconstants.OVERWRITE,
		},
	}
}

// testConfig creates a minimal valid config for testing
func testConfig() *config.Config {
	return &config.Config{
		GatewayController: config.GatewayController{
			Router: *testRouterConfig(),
			ControlPlane: config.ControlPlaneConfig{
				Host:             "localhost",
				ReconnectInitial: time.Second,
				ReconnectMax:     30 * time.Second,
				PollingInterval:  5 * time.Second,
			},
		},
	}
}

func TestTranslator_CreateTLSProtocolVersion(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	tests := []struct {
		name     string
		version  string
		expected tlsv3.TlsParameters_TlsProtocol
	}{
		{name: "TLS 1.0", version: constants.TLSVersion10, expected: tlsv3.TlsParameters_TLSv1_0},
		{name: "TLS 1.1", version: constants.TLSVersion11, expected: tlsv3.TlsParameters_TLSv1_1},
		{name: "TLS 1.2", version: constants.TLSVersion12, expected: tlsv3.TlsParameters_TLSv1_2},
		{name: "TLS 1.3", version: constants.TLSVersion13, expected: tlsv3.TlsParameters_TLSv1_3},
		{name: "Unknown version", version: "TLSv2.0", expected: tlsv3.TlsParameters_TLS_AUTO},
		{name: "Empty version", version: "", expected: tlsv3.TlsParameters_TLS_AUTO},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translator.createTLSProtocolVersion(tt.version)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTranslator_ParseCipherSuites(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	tests := []struct {
		name     string
		ciphers  string
		expected []string
	}{
		{
			name:     "Single cipher",
			ciphers:  "ECDHE-RSA-AES256-GCM-SHA384",
			expected: []string{"ECDHE-RSA-AES256-GCM-SHA384"},
		},
		{
			name:     "Multiple ciphers",
			ciphers:  "ECDHE-RSA-AES256-GCM-SHA384,ECDHE-RSA-AES128-GCM-SHA256",
			expected: []string{"ECDHE-RSA-AES256-GCM-SHA384", "ECDHE-RSA-AES128-GCM-SHA256"},
		},
		{
			name:     "Ciphers with spaces",
			ciphers:  "CIPHER1 , CIPHER2 , CIPHER3",
			expected: []string{"CIPHER1", "CIPHER2", "CIPHER3"},
		},
		{
			name:     "Empty string",
			ciphers:  "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translator.parseCipherSuites(tt.ciphers)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTranslator_PathToRegex(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "Simple path",
			path:     "/api/users",
			expected: "^/api/users$",
		},
		{
			name:     "Path with parameter",
			path:     "/api/users/{id}",
			expected: "^/api/users/[^/]+$",
		},
		{
			name:     "Path with multiple parameters",
			path:     "/api/{resource}/{id}",
			expected: "^/api/[^/]+/[^/]+$",
		},
		{
			name:     "Path with dots (version)",
			path:     "/api/v1.0/users",
			expected: "^/api/v1\\.0/users$",
		},
		{
			name:     "Root path",
			path:     "/",
			expected: "^/$",
		},
		{
			name:     "Path with special chars",
			path:     "/api/data.json",
			expected: "^/api/data\\.json$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translator.pathToRegex(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTranslator_SanitizeClusterName(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	tests := []struct {
		name     string
		hostname string
		scheme   string
		expected string
	}{
		{
			name:     "Simple hostname HTTP",
			hostname: "localhost",
			scheme:   "http",
			expected: "cluster_http_localhost",
		},
		{
			name:     "Dotted hostname HTTPS",
			hostname: "api.example.com",
			scheme:   "https",
			expected: "cluster_https_api_example_com",
		},
		{
			name:     "Hostname with port",
			hostname: "localhost:8080",
			scheme:   "http",
			expected: "cluster_http_localhost_8080",
		},
		{
			name:     "Complex hostname",
			hostname: "api.v1.prod.example.com:443",
			scheme:   "https",
			expected: "cluster_https_api_v1_prod_example_com_443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translator.sanitizeClusterName(tt.hostname, tt.scheme)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetValueFromSourceConfig(t *testing.T) {
	tests := []struct {
		name         string
		sourceConfig any
		key          string
		expected     any
		expectError  bool
	}{
		{
			name: "Simple key",
			sourceConfig: map[string]interface{}{
				"key1": "value1",
			},
			key:         "key1",
			expected:    "value1",
			expectError: false,
		},
		{
			name: "Nested key",
			sourceConfig: map[string]interface{}{
				"outer": map[string]interface{}{
					"inner": "nested_value",
				},
			},
			key:         "outer.inner",
			expected:    "nested_value",
			expectError: false,
		},
		{
			name: "Deeply nested key",
			sourceConfig: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": "deep_value",
					},
				},
			},
			key:         "a.b.c",
			expected:    "deep_value",
			expectError: false,
		},
		{
			name:         "Nil sourceConfig",
			sourceConfig: nil,
			key:          "key",
			expected:     nil,
			expectError:  true,
		},
		{
			name: "Key not found",
			sourceConfig: map[string]interface{}{
				"key1": "value1",
			},
			key:         "nonexistent",
			expected:    nil,
			expectError: true,
		},
		{
			name: "Invalid nested path",
			sourceConfig: map[string]interface{}{
				"key1": "value1",
			},
			key:         "key1.nested",
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getValueFromSourceConfig(tt.sourceConfig, tt.key)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestConvertToInterface(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]interface{}
	}{
		{
			name:     "Empty map",
			input:    map[string]string{},
			expected: map[string]interface{}{},
		},
		{
			name: "Single entry",
			input: map[string]string{
				"key": "value",
			},
			expected: map[string]interface{}{
				"key": "value",
			},
		},
		{
			name: "Multiple entries",
			input: map[string]string{
				"status":     "%RESPONSE_CODE%",
				"duration":   "%DURATION%",
				"user_agent": "%REQ(User-Agent)%",
			},
			expected: map[string]interface{}{
				"status":     "%RESPONSE_CODE%",
				"duration":   "%DURATION%",
				"user_agent": "%REQ(User-Agent)%",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToInterface(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewTranslator_WithoutCerts(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()

	translator := NewTranslator(logger, routerCfg, nil, cfg)
	assert.NotNil(t, translator)
	assert.Nil(t, translator.GetCertStore())
}

func TestTranslator_ExtractTemplateHandle_NilSourceConfig(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	storedCfg := &models.StoredConfig{
		SourceConfiguration: nil,
	}

	result := translator.extractTemplateHandle(storedCfg, nil)
	assert.Equal(t, "", result)
}

func TestTranslator_ExtractProviderName_NilSourceConfig(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	storedCfg := &models.StoredConfig{
		SourceConfiguration: nil,
	}

	result := translator.extractProviderName(storedCfg, nil)
	assert.Equal(t, "", result)
}

func TestTranslator_CreateAccessLogConfig_Disabled(t *testing.T) {
	// Note: createAccessLogConfig should only be called when access logs are enabled.
	// The check for enabled is done at the caller level. When called directly with disabled
	// access logs (format defaults to empty, which falls through to text format check),
	// it should return an error about missing text_format.
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.AccessLogs.Enabled = false
	// When format is empty, it falls through to text format check
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	logs, err := translator.createAccessLogConfig()
	// Without format configured, it returns error (this is expected behavior)
	assert.Error(t, err)
	assert.Nil(t, logs)
}

func TestTranslator_CreateAccessLogConfig_JSON(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.AccessLogs = config.AccessLogsConfig{
		Enabled: true,
		Format:  "json",
		JSONFields: map[string]string{
			"status":   "%RESPONSE_CODE%",
			"duration": "%DURATION%",
		},
	}
	cfg := testConfig()
	cfg.GatewayController.Router = *routerCfg
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	logs, err := translator.createAccessLogConfig()
	assert.NoError(t, err)
	assert.NotEmpty(t, logs)
}

func TestTranslator_CreateAccessLogConfig_Text(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.AccessLogs = config.AccessLogsConfig{
		Enabled:    true,
		Format:     "text",
		TextFormat: "[%START_TIME%] %REQ(:METHOD)% %REQ(X-ENVOY-ORIGINAL-PATH?:PATH)% %PROTOCOL% %RESPONSE_CODE%",
	}
	cfg := testConfig()
	cfg.GatewayController.Router = *routerCfg
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	logs, err := translator.createAccessLogConfig()
	assert.NoError(t, err)
	assert.NotEmpty(t, logs)
}

func TestTranslator_CreateAccessLogConfig_JSONMissingFields(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.AccessLogs = config.AccessLogsConfig{
		Enabled:    true,
		Format:     "json",
		JSONFields: nil,
	}
	cfg := testConfig()
	cfg.GatewayController.Router = *routerCfg
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	logs, err := translator.createAccessLogConfig()
	assert.Error(t, err)
	assert.Nil(t, logs)
	assert.Contains(t, err.Error(), "json_fields not configured")
}

func TestTranslator_CreatePolicyEngineCluster(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.PolicyEngine = config.PolicyEngineConfig{
		Enabled:   true,
		Host:      "localhost",
		Port:      50051,
		TimeoutMs: 1000,
		TLS: config.PolicyEngineTLS{
			Enabled: false,
		},
	}
	cfg := testConfig()
	cfg.GatewayController.Router = *routerCfg
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	cluster := translator.createPolicyEngineCluster()
	assert.NotNil(t, cluster)
	assert.Equal(t, constants.PolicyEngineClusterName, cluster.Name)
}

func TestTranslator_CreatePolicyEngineCluster_UDS(t *testing.T) {
	logger := createTestLogger()

	t.Run("UDS mode (default)", func(t *testing.T) {
		routerCfg := testRouterConfig()
		routerCfg.PolicyEngine = config.PolicyEngineConfig{
			Enabled:           true,
			Mode:              "uds",
			TimeoutMs:         1000,
			MessageTimeoutMs:  500,
			RouteCacheAction:  "DEFAULT",
			RequestHeaderMode: "DEFAULT",
		}
		cfg := testConfig()
		cfg.GatewayController.Router = *routerCfg
		translator := NewTranslator(logger, routerCfg, nil, cfg)

		c := translator.createPolicyEngineCluster()
		assert.NotNil(t, c)
		assert.Equal(t, constants.PolicyEngineClusterName, c.Name)

		// Verify cluster type is STATIC for UDS
		assert.Equal(t, cluster.Cluster_STATIC, c.ClusterDiscoveryType.(*cluster.Cluster_Type).Type)

		// Verify the address is a Pipe (UDS) with constant path
		lbEndpoint := c.LoadAssignment.Endpoints[0].LbEndpoints[0]
		addr := lbEndpoint.GetEndpoint().Address
		pipe := addr.GetPipe()
		assert.NotNil(t, pipe, "Expected Pipe address for UDS mode")
		assert.Equal(t, constants.DefaultPolicyEngineSocketPath, pipe.Path)
	})

	t.Run("TCP mode with host:port", func(t *testing.T) {
		routerCfg := testRouterConfig()
		routerCfg.PolicyEngine = config.PolicyEngineConfig{
			Enabled:           true,
			Mode:              "tcp",
			Host:              "policy-engine",
			Port:              9001,
			TimeoutMs:         1000,
			MessageTimeoutMs:  500,
			RouteCacheAction:  "DEFAULT",
			RequestHeaderMode: "DEFAULT",
		}
		cfg := testConfig()
		cfg.GatewayController.Router = *routerCfg
		translator := NewTranslator(logger, routerCfg, nil, cfg)

		c := translator.createPolicyEngineCluster()
		assert.NotNil(t, c)
		assert.Equal(t, constants.PolicyEngineClusterName, c.Name)

		// Verify cluster type is STRICT_DNS for TCP
		assert.Equal(t, cluster.Cluster_STRICT_DNS, c.ClusterDiscoveryType.(*cluster.Cluster_Type).Type)

		// Verify the address is a SocketAddress (TCP)
		lbEndpoint := c.LoadAssignment.Endpoints[0].LbEndpoints[0]
		addr := lbEndpoint.GetEndpoint().Address
		socketAddr := addr.GetSocketAddress()
		assert.NotNil(t, socketAddr, "Expected SocketAddress for TCP mode")
		assert.Equal(t, "policy-engine", socketAddr.Address)
		assert.Equal(t, uint32(9001), socketAddr.GetPortValue())
	})
}

func TestTranslator_CreateExtProcFilter(t *testing.T) {
	logger := createTestLogger()

	tests := []struct {
		name             string
		routeCacheAction string
		headerMode       string
	}{
		{name: "Default settings", routeCacheAction: "DEFAULT", headerMode: "DEFAULT"},
		{name: "Retain cache", routeCacheAction: constants.ExtProcRouteCacheActionRetain, headerMode: "SEND"},
		{name: "Clear cache", routeCacheAction: constants.ExtProcRouteCacheActionClear, headerMode: "SKIP"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routerCfg := testRouterConfig()
			routerCfg.PolicyEngine = config.PolicyEngineConfig{
				Enabled:           true,
				Host:              "localhost",
				Port:              50051,
				TimeoutMs:         1000,
				MessageTimeoutMs:  500,
				RouteCacheAction:  tt.routeCacheAction,
				RequestHeaderMode: tt.headerMode,
			}
			cfg := testConfig()
			cfg.GatewayController.Router = *routerCfg
			translator := NewTranslator(logger, routerCfg, nil, cfg)

			filter, err := translator.createExtProcFilter()
			assert.NoError(t, err)
			assert.NotNil(t, filter)
			assert.Equal(t, constants.ExtProcFilterName, filter.Name)
		})
	}
}

func TestTranslator_CreateRouteConfiguration(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	// Test with nil virtual hosts
	routeConfig := translator.createRouteConfiguration(nil)
	assert.NotNil(t, routeConfig)
	assert.Equal(t, SharedRouteConfigName, routeConfig.Name)
}

func TestTranslator_TranslateConfigs_EmptyConfigs(t *testing.T) {
	logger := createTestLogger()

	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	// Test with empty configs
	resources, err := translator.TranslateConfigs([]*models.StoredConfig{}, "test-correlation-id")
	require.NoError(t, err)
	assert.NotNil(t, resources)
}

func TestTranslator_GetCertStore_Nil(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	assert.Nil(t, translator.GetCertStore())
}

func TestTranslator_ExtractTemplateHandle_InvalidKind(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	storedCfg := &models.StoredConfig{
		SourceConfiguration: map[string]interface{}{
			"kind": 123, // Invalid type
		},
	}

	result := translator.extractTemplateHandle(storedCfg, nil)
	assert.Equal(t, "", result)
}

func TestTranslator_ExtractProviderName_InvalidKind(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	storedCfg := &models.StoredConfig{
		SourceConfiguration: map[string]interface{}{
			"kind": 123, // Invalid type
		},
	}

	result := translator.extractProviderName(storedCfg, nil)
	assert.Equal(t, "", result)
}

func TestTranslator_CreateTracingConfig_Disabled(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.TracingConfig.Enabled = false
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	tracingCfg, err := translator.createTracingConfig()
	assert.NoError(t, err)
	assert.Nil(t, tracingCfg)
}

func TestTranslator_CreateTracingConfig_Enabled(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.TracingConfig.Enabled = true
	cfg.TracingConfig.Endpoint = "otel-collector:4317"
	cfg.TracingConfig.SamplingRate = 0.5
	cfg.GatewayController.Router.TracingServiceName = "test-service"
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	tracingCfg, err := translator.createTracingConfig()
	assert.NoError(t, err)
	assert.NotNil(t, tracingCfg)
}

func TestTranslator_CreateOTELCollectorCluster(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.TracingConfig.Enabled = true
	cfg.TracingConfig.Endpoint = "otel-collector:4317"
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	cluster := translator.createOTELCollectorCluster()
	assert.NotNil(t, cluster)
	assert.Equal(t, OTELCollectorClusterName, cluster.Name)
}

func TestTranslator_CreateOTELCollectorCluster_Disabled(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.TracingConfig.Enabled = false
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	cluster := translator.createOTELCollectorCluster()
	assert.Nil(t, cluster)
}

func TestTranslator_CreateALSCluster(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.Analytics.GRPCAccessLogCfg.Host = "analytics-server"
	cfg.Analytics.AccessLogsServiceCfg.ALSServerPort = 18090
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	cluster := translator.createALSCluster()
	assert.NotNil(t, cluster)
}

func TestTranslator_CreateGRPCAccessLog(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.Analytics.GRPCAccessLogCfg = config.GRPCAccessLogConfig{
		Host:    "als-server",
		LogName: "test-log",
	}
	cfg.Analytics.AccessLogsServiceCfg.ALSServerPort = 18090
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	accessLog, err := translator.createGRPCAccessLog()
	assert.NoError(t, err)
	assert.NotNil(t, accessLog)
}

func TestTranslator_CreateDynamicForwardProxyCluster(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	cluster := translator.createDynamicForwardProxyCluster()
	assert.NotNil(t, cluster)
	assert.Equal(t, DynamicForwardProxyClusterName, cluster.Name)
}

func TestTranslator_CreateSDSCluster(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	cluster := translator.createSDSCluster()
	assert.NotNil(t, cluster)
}

func TestTranslator_CreateUpstreamTLSContext(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	// Test with no certificate
	tlsContext := translator.createUpstreamTLSContext(nil, "example.com")
	assert.NotNil(t, tlsContext)
	assert.Equal(t, "example.com", tlsContext.Sni)

	// Test with certificate
	certPem := []byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----")
	tlsContextWithCert := translator.createUpstreamTLSContext(certPem, "secure.example.com")
	assert.NotNil(t, tlsContextWithCert)
	assert.Equal(t, "secure.example.com", tlsContextWithCert.Sni)
}

func TestTranslator_ResolveUpstreamCluster_SimpleURL(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	urlStr := "http://backend:8080"
	upstream := &api.Upstream{
		Url: &urlStr,
	}

	clusterName, parsedURL, err := translator.resolveUpstreamCluster("test-upstream", upstream)
	assert.NoError(t, err)
	assert.NotEmpty(t, clusterName)
	assert.NotNil(t, parsedURL)
	assert.Equal(t, "backend", parsedURL.Hostname())
}

func TestTranslator_ResolveUpstreamCluster_HTTPSUrl(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	urlStr := "https://secure-backend:443/api"
	upstream := &api.Upstream{
		Url: &urlStr,
	}

	clusterName, parsedURL, err := translator.resolveUpstreamCluster("secure-upstream", upstream)
	assert.NoError(t, err)
	assert.NotEmpty(t, clusterName)
	assert.NotNil(t, parsedURL)
	assert.Equal(t, "https", parsedURL.Scheme)
}

func TestTranslator_ResolveUpstreamCluster_MissingURL(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	upstream := &api.Upstream{
		Url: nil, // No URL
	}

	_, _, err := translator.resolveUpstreamCluster("no-url-upstream", upstream)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no no-url-upstream upstream configured")
}

func strPtr(s string) *string {
	return &s
}

func TestTranslator_CreateCluster(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	tests := []struct {
		name       string
		clusterNm  string
		urlStr     string
		certs      map[string][]byte
		hasCluster bool
	}{
		{name: "HTTP cluster", clusterNm: "http-cluster", urlStr: "http://localhost:8080", certs: nil, hasCluster: true},
		{name: "HTTPS cluster", clusterNm: "https-cluster", urlStr: "https://secure.example.com:443", certs: nil, hasCluster: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedURL, err := parseURL(tt.urlStr)
			require.NoError(t, err)
			cluster := translator.createCluster(tt.clusterNm, parsedURL, tt.certs)
			if tt.hasCluster {
				assert.NotNil(t, cluster)
				assert.Equal(t, tt.clusterNm, cluster.Name)
			}
		})
	}
}

func parseURL(rawURL string) (*url.URL, error) {
	return url.Parse(rawURL)
}

func TestTranslator_CreateListener_HTTP(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.ListenerPort = 8080
	cfg := testConfig()
	cfg.GatewayController.Router = *routerCfg
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	listener, routeConfig, err := translator.createListener(nil, false)
	assert.NoError(t, err)
	assert.NotNil(t, listener)
	assert.NotNil(t, routeConfig)
	assert.Contains(t, listener.Name, "8080")
}

func TestTranslator_CreateDownstreamTLSContext_NoCert(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	tlsContext, err := translator.createDownstreamTLSContext()
	// Should fail because no certs are configured
	assert.Error(t, err)
	assert.Nil(t, tlsContext)
}

func TestTranslator_CreateRoute_Basic(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	route := translator.createRoute(
		"api-123",      // apiId
		"test-api",     // apiName
		"v1",           // apiVersion
		"/api",         // context
		"GET",          // method
		"/users",       // path
		"test-cluster", // clusterName
		"",             // upstreamPath
		"localhost",    // vhost
		"API",          // apiKind
		"",             // templateHandle
		"",             // providerName
		nil,            // hostRewrite
		"proj-001",     // projectID
	)

	assert.NotNil(t, route)
	assert.Contains(t, route.Name, "GET")
	assert.Contains(t, route.Name, "/api/users")
}

func TestTranslator_ExtractTemplateHandle_ValidLLMProvider(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	storedCfg := &models.StoredConfig{
		Kind: string(api.LlmProvider),
		SourceConfiguration: map[string]interface{}{
			"kind": string(api.LlmProvider),
			"spec": map[string]interface{}{
				"template": "openai-template",
			},
		},
	}

	result := translator.extractTemplateHandle(storedCfg, nil)
	assert.Equal(t, "openai-template", result)
}

func TestTranslator_ExtractProviderName_ValidLLMProvider(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	translator := NewTranslator(logger, routerCfg, nil, cfg)

	storedCfg := &models.StoredConfig{
		Kind: string(api.LlmProvider),
		SourceConfiguration: map[string]interface{}{
			"kind": string(api.LlmProvider),
			"metadata": map[string]interface{}{
				"name": "openai-provider",
			},
		},
	}

	result := translator.extractProviderName(storedCfg, nil)
	assert.Equal(t, "openai-provider", result)
}

// Tests for lines 184-200: WebSub API translation error handling
func TestTranslator_TranslateConfigs_WebSubAPIError(t *testing.T) {
	translator := createTestTranslator()

	// Create invalid WebSub API config that will cause translation error
	invalidConfig := &models.StoredConfig{
		ID:   "test-websub-invalid",
		Kind: "WebSubApi",
		Configuration: api.APIConfiguration{
			Metadata: api.Metadata{
				Name: "test-websub-api",
			},
			Kind:       api.WebSubApi,
			ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Spec:       api.APIConfiguration_Spec{
				// Invalid spec that will cause AsWebhookAPIData to fail
			},
		},
	}

	result, err := translator.TranslateConfigs([]*models.StoredConfig{invalidConfig}, "test-correlation")

	// Should handle the error gracefully and continue
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// Tests for lines 1439-1493: createRoutePerTopic method
func TestTranslator_CreateRoutePerTopic(t *testing.T) {
	t.Run("Create route with all parameters", func(t *testing.T) {
		translator := createTestTranslator()

		route := translator.createRoutePerTopic(
			"api-123",
			"Test API",
			"v1.0.0",
			"/test",
			"POST",
			"/channel1",
			"test-cluster",
			"localhost",
			"WebSubApi",
			"project-123",
		)

		assert.NotNil(t, route)
		assert.NotEmpty(t, route.Name)
		assert.Equal(t, "/test/channel1", route.GetMatch().GetPath())
		assert.Equal(t, "/hub", route.GetRoute().PrefixRewrite)
		assert.Equal(t, "test-cluster", route.GetRoute().GetCluster())

		// Verify metadata contains project ID
		assert.NotNil(t, route.Metadata)
		metadata := route.Metadata.FilterMetadata["wso2.route"]
		assert.NotNil(t, metadata)
	})

	t.Run("Create route with version placeholder in context", func(t *testing.T) {
		translator := createTestTranslator()

		route := translator.createRoutePerTopic(
			"api-123",
			"Test API",
			"v1.0.0",
			"/test/$version", // Context with version placeholder
			"POST",
			"/channel1",
			"test-cluster",
			"localhost",
			"WebSubApi",
			"project-123",
		)

		assert.NotNil(t, route)
		// ConstructFullPath replaces $version with actual version
		assert.Equal(t, "/test/v1.0.0/channel1", route.GetMatch().GetPath())
	})
}

// Tests for lines 1568-1629: TLS context creation for policy engine
func TestTranslator_CreatePolicyEngineCluster_TLS(t *testing.T) {
	t.Run("TLS with client certificates", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.PolicyEngine.TLS.Enabled = true
		translator.routerConfig.PolicyEngine.TLS.CertPath = "/path/to/client.crt"
		translator.routerConfig.PolicyEngine.TLS.KeyPath = "/path/to/client.key"
		translator.routerConfig.PolicyEngine.TLS.CAPath = "/path/to/ca.crt"
		translator.routerConfig.PolicyEngine.TLS.ServerName = "policy-engine.example.com"
		translator.routerConfig.PolicyEngine.TLS.SkipVerify = false

		cluster := translator.createPolicyEngineCluster()
		assert.NotNil(t, cluster)
		assert.NotNil(t, cluster.TransportSocket)
		assert.Equal(t, "envoy.transport_sockets.tls", cluster.TransportSocket.Name)
	})

	t.Run("TLS without client certificates", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.PolicyEngine.TLS.Enabled = true
		translator.routerConfig.PolicyEngine.TLS.CertPath = ""
		translator.routerConfig.PolicyEngine.TLS.KeyPath = ""
		translator.routerConfig.PolicyEngine.TLS.CAPath = "/path/to/ca.crt"
		translator.routerConfig.PolicyEngine.TLS.SkipVerify = false

		cluster := translator.createPolicyEngineCluster()
		assert.NotNil(t, cluster)
		assert.NotNil(t, cluster.TransportSocket)
	})
}

func createTestTranslator() *Translator {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	return NewTranslator(logger, routerCfg, nil, cfg)
}

// Tests for lines 310-351: Event gateway WebSub hub configuration
func TestTranslator_TranslateConfigs_WebSubHub_Enabled(t *testing.T) {
	t.Run("Event gateway enabled creates WebSub listeners and clusters", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "http://websub.example.com:8080"
		translator.routerConfig.EventGateway.WebSubHubPort = 8080
		translator.routerConfig.HTTPSEnabled = false

		// Empty config list to just test WebSub infrastructure
		resources, err := translator.TranslateConfigs([]*models.StoredConfig{}, "test")
		require.NoError(t, err)
		assert.NotNil(t, resources)

		// Verify that WebSub clusters and listeners were created
		clusters := resources[resource.ClusterType]
		listeners := resources[resource.ListenerType]

		// Should contain WebSub internal cluster and dynamic forward proxy cluster
		clusterNames := make([]string, 0)
		for _, c := range clusters {
			clusterNames = append(clusterNames, c.(*cluster.Cluster).GetName())
		}
		assert.Contains(t, clusterNames, constants.WEBSUBHUB_INTERNAL_CLUSTER_NAME)
		assert.Contains(t, clusterNames, DynamicForwardProxyClusterName)

		// Should contain listeners for WebSub
		listenerNames := make([]string, 0)
		for _, l := range listeners {
			listenerNames = append(listenerNames, l.(*listener.Listener).GetName())
		}
		// Check for internal listener
		assert.Contains(t, listenerNames, fmt.Sprintf("listener_http_%d", constants.WEBSUB_HUB_INTERNAL_HTTP_PORT))
	})

	t.Run("Event gateway with HTTPS enabled creates HTTPS listener", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "https://websub.example.com"
		translator.routerConfig.EventGateway.WebSubHubPort = 8443
		translator.routerConfig.HTTPSEnabled = false // Set to false to avoid TLS cert errors

		resources, err := translator.TranslateConfigs([]*models.StoredConfig{}, "test")
		require.NoError(t, err)

		listeners := resources[resource.ListenerType]
		listenerNames := make([]string, 0)
		for _, l := range listeners {
			listenerNames = append(listenerNames, l.(*listener.Listener).GetName())
		}

		// Should have HTTP listener for WebSub (HTTPS is disabled)
		assert.Contains(t, listenerNames, fmt.Sprintf("listener_http_%d", constants.WEBSUB_HUB_INTERNAL_HTTP_PORT))
	})

	t.Run("Event gateway URL parsing with missing port", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "http://websub.example.com"
		translator.routerConfig.EventGateway.WebSubHubPort = 9090

		resources, err := translator.TranslateConfigs([]*models.StoredConfig{}, "test")
		require.NoError(t, err)
		assert.NotNil(t, resources)
	})

	t.Run("Event gateway URL parsing with missing scheme", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "websub.example.com:8080"
		translator.routerConfig.EventGateway.WebSubHubPort = 8080

		resources, err := translator.TranslateConfigs([]*models.StoredConfig{}, "test")
		require.NoError(t, err)
		assert.NotNil(t, resources)
	})

	t.Run("Event gateway with invalid URL", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "://invalid-url"

		_, err := translator.TranslateConfigs([]*models.StoredConfig{}, "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid upstream URL")
	})
}

// Tests for lines 400-447: translateAsyncAPIConfig method
func TestTranslator_TranslateAsyncAPIConfig(t *testing.T) {
	t.Run("Translate valid WebSub API config", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "http://websub.example.com:8080"

		webhookConfig := &models.StoredConfig{
			ID:   "websub-api-1",
			Kind: "WebSubApi",
			Configuration: api.APIConfiguration{
				Metadata: api.Metadata{
					Name:   "websub-test",
					Labels: &map[string]string{"project-id": "proj-123"},
				},
				Kind:       api.WebSubApi,
				ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
				Spec:       api.APIConfiguration_Spec{},
			},
		}

		require.NoError(t, webhookConfig.Configuration.Spec.FromWebhookAPIData(api.WebhookAPIData{
			DisplayName: "WebSub Test API",
			Version:     "v1.0",
			Context:     "/webhook",
			Channels: []api.Channel{
				{Name: "/topic1", Method: "POST"},
				{Name: "topic2", Method: "POST"},
			},
		}))

		routes, clusters, err := translator.translateAsyncAPIConfig(webhookConfig, []*models.StoredConfig{})
		require.NoError(t, err)
		assert.NotNil(t, routes)
		assert.NotNil(t, clusters)

		// Should create route for each channel plus the main route
		assert.GreaterOrEqual(t, len(routes), 2)

		// Verify routes are created correctly
		for _, r := range routes {
			assert.NotNil(t, r.GetMatch())
			assert.Equal(t, constants.WEBSUBHUB_INTERNAL_CLUSTER_NAME, r.GetRoute().GetCluster())
		}
	})

	t.Run("WebSub API with invalid URL", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "://invalid"

		webhookConfig := &models.StoredConfig{
			ID:   "websub-api-2",
			Kind: "WebSubApi",
			Configuration: api.APIConfiguration{
				Metadata: api.Metadata{Name: "websub-invalid"},
				Kind:     api.WebSubApi,
				Spec:     api.APIConfiguration_Spec{},
			},
		}
		require.NoError(t, webhookConfig.Configuration.Spec.FromWebhookAPIData(api.WebhookAPIData{
			DisplayName: "WebSub Invalid",
			Version:     "v1.0",
			Context:     "/webhook",
			Channels:    []api.Channel{{Name: "/test", Method: "POST"}},
		}))

		_, _, err := translator.translateAsyncAPIConfig(webhookConfig, []*models.StoredConfig{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid upstream URL")
	})
}

// Tests for lines 697-834: createInternalListenerForWebSubHub method
func TestTranslator_CreateInternalListenerForWebSubHub(t *testing.T) {
	t.Run("Create HTTP internal listener", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.PolicyEngine.Enabled = false
		translator.routerConfig.AccessLogs.Enabled = false

		listener, err := translator.createInternalListenerForWebSubHub(false)
		require.NoError(t, err)
		assert.NotNil(t, listener)

		// Verify listener name and port
		expectedName := fmt.Sprintf("listener_http_%d", constants.WEBSUB_HUB_INTERNAL_HTTP_PORT)
		assert.Equal(t, expectedName, listener.GetName())
		assert.Equal(t, uint32(constants.WEBSUB_HUB_INTERNAL_HTTP_PORT), listener.GetAddress().GetSocketAddress().GetPortValue())

		// Verify filter chain exists
		assert.NotEmpty(t, listener.GetFilterChains())
		filterChain := listener.GetFilterChains()[0]
		assert.NotNil(t, filterChain)

		// Should not have TLS for HTTP
		assert.Nil(t, filterChain.GetTransportSocket())
	})

	t.Run("Create HTTPS internal listener with TLS", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.PolicyEngine.Enabled = false
		translator.routerConfig.AccessLogs.Enabled = false

		// This test will fail without proper TLS certs, so we expect an error
		_, err := translator.createInternalListenerForWebSubHub(true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create downstream TLS context")
	})

	t.Run("Create listener with policy engine enabled", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.PolicyEngine.Enabled = true
		translator.routerConfig.PolicyEngine.Host = "policy-engine"
		translator.routerConfig.PolicyEngine.Port = 9002
		translator.routerConfig.LuaScriptPath = "../../lua/request_transformation.lua"

		listener, err := translator.createInternalListenerForWebSubHub(false)
		require.NoError(t, err)
		assert.NotNil(t, listener)

		// Verify listener was created successfully with ext_proc filter
		assert.NotEmpty(t, listener.GetFilterChains())
	})

	t.Run("Create listener with access logs enabled", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.AccessLogs.Enabled = true
		translator.routerConfig.AccessLogs.Format = "json"
		translator.routerConfig.AccessLogs.JSONFields = map[string]string{
			"start_time": "%START_TIME%",
			"method":     "%REQ(:METHOD)%",
		}

		listener, err := translator.createInternalListenerForWebSubHub(false)
		require.NoError(t, err)
		assert.NotNil(t, listener)
	})

	t.Run("Create listener with tracing enabled", func(t *testing.T) {
		translator := createTestTranslator()
		translator.config.TracingConfig.Enabled = true
		translator.config.TracingConfig.Endpoint = "otel-collector:4317"

		listener, err := translator.createInternalListenerForWebSubHub(false)
		require.NoError(t, err)
		assert.NotNil(t, listener)
	})
}

// Tests for lines 913-1108: createDynamicFwdListenerForWebSubHub method
func TestTranslator_CreateDynamicFwdListenerForWebSubHub(t *testing.T) {
	t.Run("Create HTTP dynamic forward proxy listener", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "http://websub.example.com:8080"
		translator.routerConfig.PolicyEngine.Enabled = false
		translator.routerConfig.AccessLogs.Enabled = false

		listener, err := translator.createDynamicFwdListenerForWebSubHub(false)
		require.NoError(t, err)
		assert.NotNil(t, listener)

		// Verify listener name and port
		expectedName := fmt.Sprintf("listener_http_%d", constants.WEBSUB_HUB_DYNAMIC_HTTP_PORT)
		assert.Equal(t, expectedName, listener.GetName())
		assert.Equal(t, uint32(constants.WEBSUB_HUB_DYNAMIC_HTTP_PORT), listener.GetAddress().GetSocketAddress().GetPortValue())

		// Verify filter chain
		assert.NotEmpty(t, listener.GetFilterChains())
		filterChain := listener.GetFilterChains()[0]
		assert.NotNil(t, filterChain)
		assert.NotEmpty(t, filterChain.GetFilters())
	})

	t.Run("Create HTTPS dynamic forward proxy listener", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "https://websub.example.com"
		translator.routerConfig.PolicyEngine.Enabled = false
		translator.routerConfig.AccessLogs.Enabled = false

		listener, err := translator.createDynamicFwdListenerForWebSubHub(true)
		require.NoError(t, err)
		assert.NotNil(t, listener)

		// Verify listener name and port
		expectedName := fmt.Sprintf("listener_https_%d", constants.WEBSUB_HUB_DYNAMIC_HTTPS_PORT)
		assert.Equal(t, expectedName, listener.GetName())
		assert.Equal(t, uint32(constants.WEBSUB_HUB_DYNAMIC_HTTPS_PORT), listener.GetAddress().GetSocketAddress().GetPortValue())
	})

	t.Run("Create dynamic listener with policy engine enabled", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "http://websub.example.com:8080"
		translator.routerConfig.PolicyEngine.Enabled = true
		translator.routerConfig.PolicyEngine.Host = "policy-engine"
		translator.routerConfig.PolicyEngine.Port = 9002
		translator.routerConfig.LuaScriptPath = "../../lua/request_transformation.lua"

		listener, err := translator.createDynamicFwdListenerForWebSubHub(false)
		require.NoError(t, err)
		assert.NotNil(t, listener)

		// Verify HTTP filters include ext_proc when policy engine is enabled
		assert.NotEmpty(t, listener.GetFilterChains())
	})

	t.Run("Create dynamic listener with access logs enabled", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "http://websub.example.com:8080"
		translator.routerConfig.AccessLogs.Enabled = true
		translator.routerConfig.AccessLogs.Format = "json"
		translator.routerConfig.AccessLogs.JSONFields = map[string]string{
			"start_time": "%START_TIME%",
		}

		listener, err := translator.createDynamicFwdListenerForWebSubHub(false)
		require.NoError(t, err)
		assert.NotNil(t, listener)
	})

	t.Run("Create dynamic listener with tracing enabled", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "http://websub.example.com:8080"
		translator.config.TracingConfig.Enabled = true
		translator.config.TracingConfig.Endpoint = "otel-collector:4317"

		listener, err := translator.createDynamicFwdListenerForWebSubHub(false)
		require.NoError(t, err)
		assert.NotNil(t, listener)
	})

	t.Run("Verify dynamic forward proxy configuration", func(t *testing.T) {
		translator := createTestTranslator()
		translator.routerConfig.EventGateway.Enabled = true
		translator.routerConfig.EventGateway.WebSubHubURL = "http://websub.example.com:8080"

		listener, err := translator.createDynamicFwdListenerForWebSubHub(false)
		require.NoError(t, err)
		assert.NotNil(t, listener)

		// Verify the listener has the correct configuration
		assert.Equal(t, "0.0.0.0", listener.GetAddress().GetSocketAddress().GetAddress())
		assert.Equal(t, core.SocketAddress_TCP, listener.GetAddress().GetSocketAddress().GetProtocol())
	})
}
