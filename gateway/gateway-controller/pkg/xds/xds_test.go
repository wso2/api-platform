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
	"io"
	"log/slog"
	"testing"

	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	commonconstants "github.com/wso2/api-platform/common/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/certstore"
)

// createTestLogger creates a no-op logger for tests
func createTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestSlogAdapter(t *testing.T) {
	logger := createTestLogger()
	adapter := &slogAdapter{logger: logger}

	// These should not panic
	adapter.Debugf("debug message: %s", "test")
	adapter.Infof("info message: %d", 42)
	adapter.Warnf("warn message")
	adapter.Errorf("error message: %v", "error")
}

func TestGenerateRouteName(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		context    string
		apiVersion string
		path       string
		vhost      string
		expected   string
	}{
		{
			name:       "basic route",
			method:     "GET",
			context:    "/api",
			apiVersion: "v1",
			path:       "/users",
			vhost:      "example.com",
			expected:   "GET|/api/users|example.com",
		},
		{
			name:       "with version placeholder",
			method:     "POST",
			context:    "/weather/$version",
			apiVersion: "v1.0",
			path:       "/forecast",
			vhost:      "api.example.com",
			expected:   "POST|/weather/v1.0/forecast|api.example.com",
		},
		{
			name:       "root context",
			method:     "DELETE",
			context:    "/",
			apiVersion: "v2",
			path:       "/items",
			vhost:      "localhost",
			expected:   "DELETE|/items|localhost",
		},
		{
			name:       "empty vhost",
			method:     "PUT",
			context:    "/service",
			apiVersion: "v1",
			path:       "/update",
			vhost:      "",
			expected:   "PUT|/service/update|",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateRouteName(tt.method, tt.context, tt.apiVersion, tt.path, tt.vhost)
			if result != tt.expected {
				t.Errorf("GenerateRouteName() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestConstructFullPath(t *testing.T) {
	tests := []struct {
		name       string
		context    string
		apiVersion string
		path       string
		expected   string
	}{
		{
			name:       "simple path",
			context:    "/api",
			apiVersion: "v1",
			path:       "/users",
			expected:   "/api/users",
		},
		{
			name:       "version placeholder replacement",
			context:    "/weather/$version",
			apiVersion: "v1.0",
			path:       "/forecast",
			expected:   "/weather/v1.0/forecast",
		},
		{
			name:       "multiple version placeholders",
			context:    "/$version/api/$version",
			apiVersion: "v2",
			path:       "/data",
			expected:   "/v2/api/v2/data",
		},
		{
			name:       "root context",
			context:    "/",
			apiVersion: "v1",
			path:       "/items",
			expected:   "/items",
		},
		{
			name:       "no version placeholder",
			context:    "/service",
			apiVersion: "v3",
			path:       "/endpoint",
			expected:   "/service/endpoint",
		},
		{
			name:       "root path",
			context:    "/api",
			apiVersion: "v1",
			path:       "/",
			expected:   "/api/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConstructFullPath(tt.context, tt.apiVersion, tt.path)
			if result != tt.expected {
				t.Errorf("ConstructFullPath() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestConvertServerHeaderTransformation(t *testing.T) {
	tests := []struct {
		name           string
		transformation string
		expected       hcm.HttpConnectionManager_ServerHeaderTransformation
	}{
		{
			name:           "APPEND_IF_ABSENT",
			transformation: commonconstants.APPEND_IF_ABSENT,
			expected:       hcm.HttpConnectionManager_APPEND_IF_ABSENT,
		},
		{
			name:           "OVERWRITE",
			transformation: commonconstants.OVERWRITE,
			expected:       hcm.HttpConnectionManager_OVERWRITE,
		},
		{
			name:           "PASS_THROUGH",
			transformation: commonconstants.PASS_THROUGH,
			expected:       hcm.HttpConnectionManager_PASS_THROUGH,
		},
		{
			name:           "unknown value",
			transformation: "UNKNOWN",
			expected:       hcm.HttpConnectionManager_OVERWRITE,
		},
		{
			name:           "empty string",
			transformation: "",
			expected:       hcm.HttpConnectionManager_OVERWRITE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertServerHeaderTransformation(tt.transformation)
			if result != tt.expected {
				t.Errorf("convertServerHeaderTransformation(%q) = %v, want %v", tt.transformation, result, tt.expected)
			}
		})
	}
}

func TestNewSDSSecretManager(t *testing.T) {
	logger := createTestLogger()
	testCache := cache.NewSnapshotCache(false, cache.IDHash{}, &slogAdapter{logger: logger})
	nodeID := "test-node"

	manager := NewSDSSecretManager(nil, testCache, nodeID, logger)

	if manager == nil {
		t.Fatal("NewSDSSecretManager() returned nil")
	}

	if manager.nodeID != nodeID {
		t.Errorf("nodeID = %q, want %q", manager.nodeID, nodeID)
	}

	if manager.GetNodeID() != nodeID {
		t.Errorf("GetNodeID() = %q, want %q", manager.GetNodeID(), nodeID)
	}

	if manager.GetCache() != testCache {
		t.Error("GetCache() did not return expected cache")
	}
}

func TestSDSSecretManager_UpdateSecrets_NoCertStore(t *testing.T) {
	logger := createTestLogger()
	testCache := cache.NewSnapshotCache(false, cache.IDHash{}, &slogAdapter{logger: logger})

	manager := NewSDSSecretManager(nil, testCache, "test-node", logger)

	// Should not error when cert store is nil
	err := manager.UpdateSecrets()
	if err != nil {
		t.Errorf("UpdateSecrets() with nil cert store returned error: %v", err)
	}
}

func TestSDSSecretManager_GetSecret_NoCertStore(t *testing.T) {
	logger := createTestLogger()
	testCache := cache.NewSnapshotCache(false, cache.IDHash{}, &slogAdapter{logger: logger})

	manager := NewSDSSecretManager(nil, testCache, "test-node", logger)

	// Should return error when cert store is nil
	_, err := manager.GetSecret()
	if err == nil {
		t.Error("GetSecret() with nil cert store should return error")
	}
}

func TestSDSSecretManager_WithCertStore(t *testing.T) {
	logger := createTestLogger()
	testCache := cache.NewSnapshotCache(false, cache.IDHash{}, &slogAdapter{logger: logger})

	// Create a cert store with mock data
	cs := certstore.NewCertStore(logger, nil, "", "")

	manager := NewSDSSecretManager(cs, testCache, "test-node", logger)

	// GetSecret should return error because cert store has no certs
	_, err := manager.GetSecret()
	if err == nil {
		t.Error("GetSecret() with empty cert store should return error")
	}
}

func TestSDSSecretManager_UpdateSecrets_EmptyCertStore(t *testing.T) {
	logger := createTestLogger()
	testCache := cache.NewSnapshotCache(false, cache.IDHash{}, &slogAdapter{logger: logger})
	cs := certstore.NewCertStore(logger, nil, "", "")

	manager := NewSDSSecretManager(cs, testCache, "test-node", logger)

	// Should not error, but log warning
	err := manager.UpdateSecrets()
	if err != nil {
		t.Errorf("UpdateSecrets() with empty cert store returned error: %v", err)
	}
}

func TestSecretNameUpstreamCA(t *testing.T) {
	if SecretNameUpstreamCA != "upstream_ca_bundle" {
		t.Errorf("SecretNameUpstreamCA = %q, want %q", SecretNameUpstreamCA, "upstream_ca_bundle")
	}
}

func TestDynamicForwardProxyClusterName(t *testing.T) {
	if DynamicForwardProxyClusterName != "dynamic-forward-proxy-cluster" {
		t.Errorf("DynamicForwardProxyClusterName = %q, want %q", DynamicForwardProxyClusterName, "dynamic-forward-proxy-cluster")
	}
}

func TestExternalProcessorGRPCServiceClusterName(t *testing.T) {
	if ExternalProcessorGRPCServiceClusterName != "ext-processor-grpc-service" {
		t.Errorf("ExternalProcessorGRPCServiceClusterName = %q, want %q", ExternalProcessorGRPCServiceClusterName, "ext-processor-grpc-service")
	}
}

func TestOTELCollectorClusterName(t *testing.T) {
	if OTELCollectorClusterName != "otel_collector" {
		t.Errorf("OTELCollectorClusterName = %q, want %q", OTELCollectorClusterName, "otel_collector")
	}
}

func TestWebSubHubInternalClusterName(t *testing.T) {
	if WebSubHubInternalClusterName != "WEBSUBHUB_INTERNAL_CLUSTER" {
		t.Errorf("WebSubHubInternalClusterName = %q, want %q", WebSubHubInternalClusterName, "WEBSUBHUB_INTERNAL_CLUSTER")
	}
}
