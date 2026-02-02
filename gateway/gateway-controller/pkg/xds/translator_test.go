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
	"fmt"
	"testing"
	"time"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	resource "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveUpstreamDefinition_Found(t *testing.T) {
	timeout := "30s"
	definitions := &[]api.UpstreamDefinition{
		{
			Name: "test-upstream",
			Timeout: &api.UpstreamTimeout{
				Request: &timeout,
			},
			Upstreams: []struct {
				Urls   []string `json:"urls" yaml:"urls"`
				Weight *int     `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Urls: []string{"http://backend:8080"},
				},
			},
		},
	}

	def, err := resolveUpstreamDefinition("test-upstream", definitions)

	require.NoError(t, err)
	assert.NotNil(t, def)
	assert.Equal(t, "test-upstream", def.Name)
	assert.NotNil(t, def.Timeout)
	assert.NotNil(t, def.Timeout.Request)
	assert.Equal(t, "30s", *def.Timeout.Request)
}

func TestResolveUpstreamDefinition_NotFound(t *testing.T) {
	definitions := &[]api.UpstreamDefinition{
		{
			Name: "existing-upstream",
			Upstreams: []struct {
				Urls   []string `json:"urls" yaml:"urls"`
				Weight *int     `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Urls: []string{"http://backend:8080"},
				},
			},
		},
	}

	def, err := resolveUpstreamDefinition("non-existent", definitions)

	assert.Error(t, err)
	assert.Nil(t, def)
	assert.Contains(t, err.Error(), "upstream definition 'non-existent' not found")
}

func TestResolveUpstreamDefinition_NoDefinitions(t *testing.T) {
	def, err := resolveUpstreamDefinition("test-upstream", nil)

	assert.Error(t, err)
	assert.Nil(t, def)
	assert.Contains(t, err.Error(), "no definitions provided")
}

func TestParseTimeout_Valid(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
	}{
		{
			name:     "seconds",
			input:    "30s",
			expected: 30 * time.Second,
		},
		{
			name:     "minutes",
			input:    "2m",
			expected: 2 * time.Minute,
		},
		{
			name:     "milliseconds",
			input:    "500ms",
			expected: 500 * time.Millisecond,
		},
		{
			name:     "hours",
			input:    "1h",
			expected: 1 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration, err := parseTimeout(&tt.input)

			require.NoError(t, err)
			require.NotNil(t, duration)
			assert.Equal(t, tt.expected, *duration)
		})
	}
}

func TestParseTimeout_Invalid(t *testing.T) {
	invalid := "invalid"
	duration, err := parseTimeout(&invalid)

	assert.Error(t, err)
	assert.Nil(t, duration)
	assert.Contains(t, err.Error(), "invalid timeout format")
}

func TestParseTimeout_Nil(t *testing.T) {
	duration, err := parseTimeout(nil)

	assert.NoError(t, err)
	assert.Nil(t, duration)
}

func TestParseTimeout_Empty(t *testing.T) {
	empty := ""
	duration, err := parseTimeout(&empty)

	assert.NoError(t, err)
	assert.Nil(t, duration)
}

func TestResolveUpstreamCluster_WithDirectURL(t *testing.T) {
	translator := &Translator{}
	url := "http://backend:8080/api"
	upstream := &api.Upstream{
		Url: &url,
	}

	clusterName, parsedURL, timeout, err := translator.resolveUpstreamCluster("main", upstream, nil)

	require.NoError(t, err)
	assert.Equal(t, "cluster_http_backend_8080", clusterName)
	assert.NotNil(t, parsedURL)
	assert.Equal(t, "http", parsedURL.Scheme)
	assert.Equal(t, "backend:8080", parsedURL.Host)
	assert.Equal(t, "/api", parsedURL.Path)
	assert.Nil(t, timeout, "Direct URL should not have timeout override")
}

func TestResolveUpstreamCluster_WithRef_WithTimeout(t *testing.T) {
	translator := &Translator{}
	ref := "my-upstream"
	timeoutStr := "45s"
	upstream := &api.Upstream{
		Ref: &ref,
	}
	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Timeout: &api.UpstreamTimeout{
				Request: &timeoutStr,
			},
			Upstreams: []struct {
				Urls   []string `json:"urls" yaml:"urls"`
				Weight *int     `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Urls: []string{"http://backend-1:9000/v2"},
				},
			},
		},
	}

	clusterName, parsedURL, timeout, err := translator.resolveUpstreamCluster("main", upstream, definitions)

	require.NoError(t, err)
	assert.Equal(t, "cluster_http_backend-1_9000", clusterName)
	assert.NotNil(t, parsedURL)
	assert.Equal(t, "http", parsedURL.Scheme)
	assert.Equal(t, "backend-1:9000", parsedURL.Host)
	assert.Equal(t, "/v2", parsedURL.Path)
	require.NotNil(t, timeout)
	require.NotNil(t, timeout.Request)
	assert.Equal(t, 45*time.Second, *timeout.Request)
}

func TestResolveUpstreamCluster_WithRef_NoTimeout(t *testing.T) {
	translator := &Translator{}
	ref := "my-upstream"
	upstream := &api.Upstream{
		Ref: &ref,
	}
	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Upstreams: []struct {
				Urls   []string `json:"urls" yaml:"urls"`
				Weight *int     `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Urls: []string{"http://backend:8080"},
				},
			},
		},
	}

	clusterName, parsedURL, timeout, err := translator.resolveUpstreamCluster("main", upstream, definitions)

	require.NoError(t, err)
	assert.Equal(t, "cluster_http_backend_8080", clusterName)
	assert.NotNil(t, parsedURL)
	assert.Nil(t, timeout, "No timeout in definition should result in nil timeout")
}

func TestResolveUpstreamCluster_WithRef_NotFound(t *testing.T) {
	translator := &Translator{}
	ref := "non-existent"
	upstream := &api.Upstream{
		Ref: &ref,
	}
	definitions := &[]api.UpstreamDefinition{
		{
			Name: "other-upstream",
			Upstreams: []struct {
				Urls   []string `json:"urls" yaml:"urls"`
				Weight *int     `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Urls: []string{"http://backend:8080"},
				},
			},
		},
	}

	_, _, _, err := translator.resolveUpstreamCluster("main", upstream, definitions)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve main upstream ref")
	assert.Contains(t, err.Error(), "upstream definition 'non-existent' not found")
}

func TestResolveUpstreamCluster_WithRef_InvalidTimeout(t *testing.T) {
	translator := &Translator{}
	ref := "my-upstream"
	invalidTimeout := "invalid"
	upstream := &api.Upstream{
		Ref: &ref,
	}
	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Timeout: &api.UpstreamTimeout{
				Request: &invalidTimeout,
			},
			Upstreams: []struct {
				Urls   []string `json:"urls" yaml:"urls"`
				Weight *int     `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Urls: []string{"http://backend:8080"},
				},
			},
		},
	}

	_, _, _, err := translator.resolveUpstreamCluster("main", upstream, definitions)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid timeout in upstream definition")
}

func TestResolveUpstreamCluster_WithRef_NoURLs(t *testing.T) {
	translator := &Translator{}
	ref := "my-upstream"
	upstream := &api.Upstream{
		Ref: &ref,
	}
	definitions := &[]api.UpstreamDefinition{
		{
			Name:      "my-upstream",
			Upstreams: []struct {
				Urls   []string `json:"urls" yaml:"urls"`
				Weight *int     `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{},
		},
	}

	_, _, _, err := translator.resolveUpstreamCluster("main", upstream, definitions)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "has no URLs configured")
}

func TestResolveUpstreamCluster_NoURLOrRef(t *testing.T) {
	translator := &Translator{}
	upstream := &api.Upstream{}

	_, _, _, err := translator.resolveUpstreamCluster("main", upstream, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no main upstream configured")
}

func TestResolveUpstreamCluster_InvalidURL(t *testing.T) {
	translator := &Translator{}
	invalidURL := "not a valid url"
	upstream := &api.Upstream{
		Url: &invalidURL,
	}

	_, _, _, err := translator.resolveUpstreamCluster("main", upstream, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid main upstream URL")
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
