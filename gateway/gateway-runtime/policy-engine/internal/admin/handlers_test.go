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

package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/testutils"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

type mockXDSSyncProvider struct {
	version string
}

func (m *mockXDSSyncProvider) GetPolicyChainVersion() string {
	return m.version
}

type mockHealthProvider struct {
	healthy bool
}

func (m *mockHealthProvider) IsHealthy() bool {
	return m.healthy
}

type mockPythonHealthChecker struct {
	ready          bool
	loadedPolicies int32
	err            error
}

func (m *mockPythonHealthChecker) IsPythonHealthy() (bool, int32, error) {
	return m.ready, m.loadedPolicies, m.err
}

// TestConfigDumpHandler_MethodNotAllowed tests that non-GET methods return 405
func TestConfigDumpHandler_MethodNotAllowed(t *testing.T) {
	handler := &ConfigDumpHandler{
		kernel:   nil,
		registry: nil,
		xds:      nil,
	}

	methods := []string{
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodHead,
		http.MethodOptions,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/config_dump", nil)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			assert.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
			assert.Contains(t, recorder.Body.String(), "Method not allowed")
		})
	}
}

// TestNewConfigDumpHandler tests the NewConfigDumpHandler constructor
func TestNewConfigDumpHandler(t *testing.T) {
	handler := NewConfigDumpHandler(nil, nil, nil)

	assert.NotNil(t, handler)
	assert.Nil(t, handler.kernel)
	assert.Nil(t, handler.registry)
}

// TestConfigDumpResponse_JSONSerialization tests JSON serialization of ConfigDumpResponse
func TestConfigDumpResponse_JSONSerialization(t *testing.T) {
	response := &ConfigDumpResponse{
		XDSSync: XDSSyncInfo{
			PolicyChainVersion: "v12",
		},
		PolicyRegistry: PolicyRegistryDump{
			TotalPolicies: 2,
			Policies: []PolicyInfo{
				{Name: "jwt-auth", Version: "v1.0.0"},
				{Name: "rate-limit", Version: "v1.0.0"},
			},
		},
		Routes: RoutesDump{
			TotalRoutes: 1,
			RouteConfigs: []RouteConfig{
				{
					RouteKey:             "api-1|/users|GET",
					RequiresRequestBody:  false,
					RequiresResponseBody: false,
					TotalPolicies:        1,
					Policies: []PolicySpec{
						{
							Name:    "jwt-auth",
							Version: "v1.0.0",
							Enabled: true,
						},
					},
				},
			},
		},
		LazyResources: LazyResourcesDump{
			TotalResources:  0,
			ResourcesByType: make(map[string][]LazyResourceInfo),
		},
	}

	// Serialize to JSON
	data, err := json.Marshal(response)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Deserialize back
	var decoded ConfigDumpResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, response.PolicyRegistry.TotalPolicies, decoded.PolicyRegistry.TotalPolicies)
	assert.Equal(t, len(response.PolicyRegistry.Policies), len(decoded.PolicyRegistry.Policies))
	assert.Equal(t, response.Routes.TotalRoutes, decoded.Routes.TotalRoutes)
	assert.Equal(t, "v12", decoded.XDSSync.PolicyChainVersion)
}

// TestPolicyInfo_JSONSerialization tests JSON serialization of PolicyInfo
func TestPolicyInfo_JSONSerialization(t *testing.T) {
	info := PolicyInfo{
		Name:    "jwt-auth",
		Version: "v1.0.0",
	}

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var decoded PolicyInfo
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, info.Name, decoded.Name)
	assert.Equal(t, info.Version, decoded.Version)
}

// TestRouteConfig_JSONSerialization tests JSON serialization of RouteConfig
func TestRouteConfig_JSONSerialization(t *testing.T) {
	condition := "request.headers['x-test'] == 'true'"
	config := RouteConfig{
		RouteKey:             "api-1|/users|POST",
		RequiresRequestBody:  true,
		RequiresResponseBody: false,
		TotalPolicies:        2,
		Policies: []PolicySpec{
			{
				Name:               "jwt-auth",
				Version:            "v1.0.0",
				Enabled:            true,
				ExecutionCondition: nil,
				Parameters:         map[string]interface{}{"issuer": "test"},
			},
			{
				Name:               "rate-limit",
				Version:            "v1.0.0",
				Enabled:            true,
				ExecutionCondition: &condition,
				Parameters:         map[string]interface{}{"limit": 100},
			},
		},
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)

	var decoded RouteConfig
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, config.RouteKey, decoded.RouteKey)
	assert.Equal(t, config.RequiresRequestBody, decoded.RequiresRequestBody)
	assert.Equal(t, config.RequiresResponseBody, decoded.RequiresResponseBody)
	assert.Equal(t, config.TotalPolicies, decoded.TotalPolicies)
	assert.Len(t, decoded.Policies, 2)
}

// TestLazyResourcesDump_JSONSerialization tests JSON serialization of LazyResourcesDump
func TestLazyResourcesDump_JSONSerialization(t *testing.T) {
	dump := LazyResourcesDump{
		TotalResources: 2,
		ResourcesByType: map[string][]LazyResourceInfo{
			"jwks": {
				{
					ID:           "jwks-1",
					ResourceType: "jwks",
					Resource:     map[string]interface{}{"uri": "https://example.com/.well-known/jwks.json"},
				},
			},
			"certificate": {
				{
					ID:           "cert-1",
					ResourceType: "certificate",
					Resource:     map[string]interface{}{"path": "/certs/ca.pem"},
				},
			},
		},
	}

	data, err := json.Marshal(dump)
	require.NoError(t, err)

	var decoded LazyResourcesDump
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, dump.TotalResources, decoded.TotalResources)
	assert.Len(t, decoded.ResourcesByType, 2)
	assert.Len(t, decoded.ResourcesByType["jwks"], 1)
	assert.Len(t, decoded.ResourcesByType["certificate"], 1)
}

// TestPolicySpec_JSONSerialization tests JSON serialization of PolicySpec with nil condition
func TestPolicySpec_JSONSerialization(t *testing.T) {
	tests := []struct {
		name string
		spec PolicySpec
	}{
		{
			name: "with nil execution condition",
			spec: PolicySpec{
				Name:               "jwt-auth",
				Version:            "v1.0.0",
				Enabled:            true,
				ExecutionCondition: nil,
				Parameters:         map[string]interface{}{"issuer": "test"},
			},
		},
		{
			name: "with execution condition",
			spec: PolicySpec{
				Name:               "rate-limit",
				Version:            "v1.0.0",
				Enabled:            false,
				ExecutionCondition: testutils.PtrString("request.method == 'POST'"),
				Parameters:         nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.spec)
			require.NoError(t, err)

			var decoded PolicySpec
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.spec.Name, decoded.Name)
			assert.Equal(t, tt.spec.Version, decoded.Version)
			assert.Equal(t, tt.spec.Enabled, decoded.Enabled)
			if tt.spec.ExecutionCondition == nil {
				assert.Nil(t, decoded.ExecutionCondition)
			} else {
				assert.Equal(t, *tt.spec.ExecutionCondition, *decoded.ExecutionCondition)
			}
		})
	}
}

// TestPolicyRegistryDump_EmptyPolicies tests PolicyRegistryDump with empty policies
func TestPolicyRegistryDump_EmptyPolicies(t *testing.T) {
	dump := PolicyRegistryDump{
		TotalPolicies: 0,
		Policies:      []PolicyInfo{},
	}

	data, err := json.Marshal(dump)
	require.NoError(t, err)

	var decoded PolicyRegistryDump
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, 0, decoded.TotalPolicies)
	assert.Empty(t, decoded.Policies)
}

// TestRoutesDump_EmptyRoutes tests RoutesDump with empty routes
func TestRoutesDump_EmptyRoutes(t *testing.T) {
	dump := RoutesDump{
		TotalRoutes:  0,
		RouteConfigs: []RouteConfig{},
	}

	data, err := json.Marshal(dump)
	require.NoError(t, err)

	var decoded RoutesDump
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, 0, decoded.TotalRoutes)
	assert.Empty(t, decoded.RouteConfigs)
}

func TestXDSSyncStatusHandler(t *testing.T) {
	handler := NewXDSSyncStatusHandler(&mockXDSSyncProvider{version: "pc-v9"})

	req := httptest.NewRequest(http.MethodGet, "/xds_sync_status", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)

	var response XDSSyncStatusResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "policy-engine", response.Component)
	assert.Equal(t, "pc-v9", response.PolicyChainVersion)
	assert.False(t, response.Timestamp.IsZero())
}

func TestXDSSyncStatusHandler_MethodNotAllowed(t *testing.T) {
	handler := NewXDSSyncStatusHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/xds_sync_status", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

// TestDumpPolicySpecs tests the dumpPolicySpecs function
func TestDumpPolicySpecs(t *testing.T) {
	tests := []struct {
		name     string
		specs    []policy.PolicySpec
		expected []PolicySpec
	}{
		{
			name:     "empty specs",
			specs:    []policy.PolicySpec{},
			expected: []PolicySpec{},
		},
		{
			name: "single spec without condition",
			specs: []policy.PolicySpec{
				{
					Name:               "jwt-auth",
					Version:            "v1.0.0",
					Enabled:            true,
					ExecutionCondition: nil,
					Parameters: policy.PolicyParameters{
						Raw: map[string]interface{}{"issuer": "test-issuer"},
					},
				},
			},
			expected: []PolicySpec{
				{
					Name:               "jwt-auth",
					Version:            "v1.0.0",
					Enabled:            true,
					ExecutionCondition: nil,
					Parameters:         map[string]interface{}{"issuer": "test-issuer"},
				},
			},
		},
		{
			name: "single spec with condition",
			specs: []policy.PolicySpec{
				{
					Name:               "rate-limit",
					Version:            "v2.0.0",
					Enabled:            false,
					ExecutionCondition: testutils.PtrString("request.method == 'POST'"),
					Parameters: policy.PolicyParameters{
						Raw: map[string]interface{}{"limit": 100},
					},
				},
			},
			expected: []PolicySpec{
				{
					Name:               "rate-limit",
					Version:            "v2.0.0",
					Enabled:            false,
					ExecutionCondition: testutils.PtrString("request.method == 'POST'"),
					Parameters:         map[string]interface{}{"limit": 100},
				},
			},
		},
		{
			name: "multiple specs",
			specs: []policy.PolicySpec{
				{
					Name:    "jwt-auth",
					Version: "v1.0.0",
					Enabled: true,
					Parameters: policy.PolicyParameters{
						Raw: map[string]interface{}{"issuer": "test"},
					},
				},
				{
					Name:    "rate-limit",
					Version: "v1.0.0",
					Enabled: true,
					Parameters: policy.PolicyParameters{
						Raw: map[string]interface{}{"limit": 50},
					},
				},
				{
					Name:    "cors",
					Version: "v1.0.0",
					Enabled: false,
					Parameters: policy.PolicyParameters{
						Raw: nil,
					},
				},
			},
			expected: []PolicySpec{
				{
					Name:       "jwt-auth",
					Version:    "v1.0.0",
					Enabled:    true,
					Parameters: map[string]interface{}{"issuer": "test"},
				},
				{
					Name:       "rate-limit",
					Version:    "v1.0.0",
					Enabled:    true,
					Parameters: map[string]interface{}{"limit": 50},
				},
				{
					Name:       "cors",
					Version:    "v1.0.0",
					Enabled:    false,
					Parameters: nil,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dumpPolicySpecs(tt.specs)

			assert.Len(t, result, len(tt.expected))
			for i, expected := range tt.expected {
				assert.Equal(t, expected.Name, result[i].Name)
				assert.Equal(t, expected.Version, result[i].Version)
				assert.Equal(t, expected.Enabled, result[i].Enabled)
				if expected.ExecutionCondition == nil {
					assert.Nil(t, result[i].ExecutionCondition)
				} else {
					require.NotNil(t, result[i].ExecutionCondition)
					assert.Equal(t, *expected.ExecutionCondition, *result[i].ExecutionCondition)
				}
			}
		})
	}
}

// TestHealthHandler_Healthy tests that healthy provider returns 200
func TestHealthHandler_Healthy(t *testing.T) {
	handler := NewHealthHandler(&mockHealthProvider{healthy: true}, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)

	var response HealthResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "healthy", response.Status)
	assert.NotEmpty(t, response.Timestamp)
	assert.Empty(t, response.Reason)
}

// TestHealthHandler_Unhealthy tests that unhealthy provider returns 503
func TestHealthHandler_Unhealthy(t *testing.T) {
	handler := NewHealthHandler(&mockHealthProvider{healthy: false}, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)

	var response HealthResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "unhealthy", response.Status)
	assert.NotEmpty(t, response.Timestamp)
	assert.Equal(t, "policy engine is unhealthy", response.Reason)
}

// TestHealthHandler_NilProvider tests that nil provider returns healthy (safe default)
func TestHealthHandler_NilProvider(t *testing.T) {
	handler := NewHealthHandler(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)

	var response HealthResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "healthy", response.Status)
	assert.NotEmpty(t, response.Timestamp)
	assert.Empty(t, response.Reason)
}

// TestHealthHandler_MethodNotAllowed tests that non-GET methods return 405
func TestHealthHandler_MethodNotAllowed(t *testing.T) {
	handler := NewHealthHandler(&mockHealthProvider{healthy: true}, nil)

	methods := []string{
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/health", nil)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			assert.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
		})
	}
}

// TestHealthHandler_BothHealthy_WithPython tests that both healthy returns 200
func TestHealthHandler_BothHealthy_WithPython(t *testing.T) {
	pythonHealth := &mockPythonHealthChecker{ready: true, loadedPolicies: 3}
	handler := NewHealthHandler(&mockHealthProvider{healthy: true}, pythonHealth)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)

	var response HealthResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "healthy", response.Status)
	assert.NotEmpty(t, response.Timestamp)
	assert.Empty(t, response.Reason)
}

// TestHealthHandler_PEUnhealthy_PythonHealthy tests policy engine unhealthy returns 503 with reason
func TestHealthHandler_PEUnhealthy_PythonHealthy(t *testing.T) {
	pythonHealth := &mockPythonHealthChecker{ready: true, loadedPolicies: 2}
	handler := NewHealthHandler(&mockHealthProvider{healthy: false}, pythonHealth)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)

	var response HealthResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "unhealthy", response.Status)
	assert.NotEmpty(t, response.Timestamp)
	assert.Equal(t, "policy engine is unhealthy", response.Reason)
}

// TestHealthHandler_PEHealthy_PythonUnhealthy tests Python unhealthy returns 503 with reason
func TestHealthHandler_PEHealthy_PythonUnhealthy(t *testing.T) {
	pythonHealth := &mockPythonHealthChecker{ready: false, loadedPolicies: 0}
	handler := NewHealthHandler(&mockHealthProvider{healthy: true}, pythonHealth)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)

	var response HealthResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "unhealthy", response.Status)
	assert.NotEmpty(t, response.Timestamp)
	assert.Equal(t, "python executor is unhealthy", response.Reason)
}

// TestHealthHandler_BothUnhealthy tests both unhealthy returns 503 with reason
func TestHealthHandler_BothUnhealthy(t *testing.T) {
	pythonHealth := &mockPythonHealthChecker{ready: false, loadedPolicies: 0}
	handler := NewHealthHandler(&mockHealthProvider{healthy: false}, pythonHealth)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)

	var response HealthResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "unhealthy", response.Status)
	assert.NotEmpty(t, response.Timestamp)
	assert.Equal(t, "policy engine and python executor are unhealthy", response.Reason)
}

// TestHealthHandler_PythonError tests Python health check error returns 503 with reason
func TestHealthHandler_PythonError(t *testing.T) {
	pythonHealth := &mockPythonHealthChecker{ready: false, loadedPolicies: 0, err: assert.AnError}
	handler := NewHealthHandler(&mockHealthProvider{healthy: true}, pythonHealth)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)

	var response HealthResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "unhealthy", response.Status)
	assert.NotEmpty(t, response.Timestamp)
	assert.Equal(t, "python executor is unhealthy", response.Reason)
}
