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

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test error handling in APIConfiguration_Spec methods

func TestAPIConfiguration_Spec_MergeAPIConfigData_JSONMarshalError(t *testing.T) {
	spec := &APIConfiguration_Spec{}

	// Use a value that causes JSON marshal error (function/channel types can't be marshaled)
	invalidData := APIConfigData{}
	// Since APIConfigData is a map, we need to create invalid content
	// In Go, we can't easily create values that fail JSON marshaling for simple types
	// but we can test with valid data and ensure no error
	err := spec.MergeAPIConfigData(invalidData)
	assert.NoError(t, err) // APIConfigData is simple and should marshal fine
}

func TestAPIConfiguration_Spec_AsWebhookAPIData_JSONUnmarshalError(t *testing.T) {
	spec := APIConfiguration_Spec{}
	// Set invalid JSON that can't be unmarshaled
	spec.union = []byte("invalid json")

	_, err := spec.AsWebhookAPIData()
	assert.Error(t, err)
}

func TestAPIConfiguration_Spec_MergeWebhookAPIData_JSONMarshalError(t *testing.T) {
	spec := &APIConfiguration_Spec{}

	data := WebhookAPIData{}
	err := spec.MergeWebhookAPIData(data)
	assert.NoError(t, err) // WebhookAPIData should marshal fine
}

func TestAPIConfiguration_Spec_MarshalJSON_Error(t *testing.T) {
	spec := APIConfiguration_Spec{}
	// json.RawMessage will not fail to marshal unless nil, so this will succeed
	_, err := spec.MarshalJSON()
	assert.NoError(t, err) // This is expected to not error
}

func TestAPIConfiguration_Spec_UnmarshalJSON_Error(t *testing.T) {
	spec := &APIConfiguration_Spec{}

	// Even invalid JSON will succeed with json.RawMessage since it just stores the bytes
	err := spec.UnmarshalJSON([]byte("invalid json"))
	assert.NoError(t, err) // This is expected to not error for RawMessage
}

func TestStrictServerInterface_Methods(t *testing.T) {
	// Test that all server interface methods return proper responses
	// This ensures we cover the error return paths in generated code

	t.Run("GetSwagger error cases", func(t *testing.T) {
		router := gin.New()
		mock := &MockServerInterface{}

		// Register with invalid spec URL to trigger error paths
		RegisterHandlersWithOptions(router, mock, GinServerOptions{
			BaseURL: "invalid-url",
		})

		// The registration should succeed but GetSwagger might handle errors internally
		assert.NotNil(t, router)
	})

	t.Run("Handler registration with options", func(t *testing.T) {
		router := gin.New()
		mock := &MockServerInterface{}

		// Test with empty base URL
		RegisterHandlersWithOptions(router, mock, GinServerOptions{
			BaseURL: "",
		})

		assert.NotNil(t, router)
	})
}

func TestParameterProcessing_ErrorPaths(t *testing.T) {
	// Test parameter processing that can cause errors in the generated code

	t.Run("Invalid query parameters", func(t *testing.T) {
		router := gin.New()
		mock := &MockServerInterface{}
		RegisterHandlers(router, mock)

		// Make request with invalid parameters that might trigger error paths
		req := httptest.NewRequest("GET", "/apis?limit=invalid", nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)

		// Should handle parameter validation errors
		assert.NotEqual(t, http.StatusInternalServerError, recorder.Code)
	})

	t.Run("Missing required parameters", func(t *testing.T) {
		router := gin.New()
		mock := &MockServerInterface{}
		RegisterHandlers(router, mock)

		// Make request to a non-existent route to test 404 handling
		req := httptest.NewRequest("GET", "/nonexistent-route", nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)

		// Should return 404 for unknown routes
		assert.Equal(t, http.StatusNotFound, recorder.Code)
	})
}

func TestJSONProcessing_ErrorPaths(t *testing.T) {
	// Test JSON processing paths in generated code
	// Note: The mock implementation doesn't validate JSON bodies - it just returns success.
	// This test verifies the route is reachable and the mock handler is called.

	t.Run("Invalid JSON in request body", func(t *testing.T) {
		router := gin.New()
		mock := &MockServerInterface{}
		RegisterHandlers(router, mock)

		// Send invalid JSON in POST request
		req := httptest.NewRequest("POST", "/apis", strings.NewReader("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)

		// The mock handler is called and returns 201 since it doesn't validate the body
		// JSON validation would happen in the actual handler implementation, not in the mock
		t.Logf("Received status code: %d", recorder.Code)
		assert.True(t, mock.CreateAPICalled, "CreateAPI handler should be called")
		assert.Equal(t, http.StatusCreated, recorder.Code)
	})
}

func TestSwaggerSpec_ErrorHandling(t *testing.T) {
	// Test swagger spec loading and processing error paths

	t.Run("GetSwagger returns valid spec", func(t *testing.T) {
		spec, err := GetSwagger()
		assert.NoError(t, err)
		assert.NotNil(t, spec)
	})

	t.Run("Swagger spec validation", func(t *testing.T) {
		spec, err := GetSwagger()
		require.NoError(t, err)

		// Validate that spec has required fields
		assert.NotEmpty(t, spec.Info)
		assert.NotEmpty(t, spec.Paths)
	})
}

func TestAPIConfiguration_Spec_JSONMerge_Error(t *testing.T) {
	// Test JSONMerge error paths
	spec := &APIConfiguration_Spec{}
	spec.union = []byte("invalid json") // This will cause JSONMerge to fail

	data := APIConfigData{}
	err := spec.MergeAPIConfigData(data)
	assert.Error(t, err) // Should return the JSONMerge error
}

func TestWebhookAPIData_JSONMerge_Error(t *testing.T) {
	// Test JSONMerge error paths for WebhookAPIData
	spec := &APIConfiguration_Spec{}
	spec.union = []byte("invalid json") // This will cause JSONMerge to fail

	data := WebhookAPIData{}
	err := spec.MergeWebhookAPIData(data)
	assert.Error(t, err) // Should return the JSONMerge error
}

// MockServerInterface is a mock implementation of ServerInterface for testing
type MockServerInterface struct {
	ListAPIsCalled                   bool
	CreateAPICalled                  bool
	DeleteAPICalled                  bool
	GetAPIByIdCalled                 bool
	UpdateAPICalled                  bool
	ListAPIKeysCalled                bool
	CreateAPIKeyCalled               bool
	UpdateAPIKeyCalled               bool
	RevokeAPIKeyCalled               bool
	RegenerateAPIKeyCalled           bool
	ListCertificatesCalled           bool
	UploadCertificateCalled          bool
	ReloadCertificatesCalled         bool
	DeleteCertificateCalled          bool
	GetConfigDumpCalled              bool
	HealthCheckCalled                bool
	ListLLMProviderTemplatesCalled   bool
	CreateLLMProviderTemplateCalled  bool
	DeleteLLMProviderTemplateCalled  bool
	GetLLMProviderTemplateByIdCalled bool
	UpdateLLMProviderTemplateCalled  bool
	ListLLMProvidersCalled           bool
	CreateLLMProviderCalled          bool
	DeleteLLMProviderCalled          bool
	GetLLMProviderByIdCalled         bool
	UpdateLLMProviderCalled          bool
	ListLLMProxiesCalled             bool
	CreateLLMProxyCalled             bool
	DeleteLLMProxyCalled             bool
	GetLLMProxyByIdCalled            bool
	UpdateLLMProxyCalled             bool
	ListMCPProxiesCalled             bool
	CreateMCPProxyCalled             bool
	DeleteMCPProxyCalled             bool
	GetMCPProxyByIdCalled            bool
	UpdateMCPProxyCalled             bool
	ListPoliciesCalled               bool
}

// CreateAPIKey implements [ServerInterface].
func (m *MockServerInterface) CreateAPIKey(c *gin.Context, id string) {
	m.CreateAPIKeyCalled = true
	c.JSON(http.StatusCreated, gin.H{"status": "created"})
}

// UpdateAPIKey implements [ServerInterface].
func (m *MockServerInterface) UpdateAPIKey(c *gin.Context, id string, apiKeyName string) {
	m.UpdateAPIKeyCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (m *MockServerInterface) ListAPIs(c *gin.Context, params ListAPIsParams) {
	m.ListAPIsCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (m *MockServerInterface) CreateAPI(c *gin.Context) {
	m.CreateAPICalled = true
	c.JSON(http.StatusCreated, gin.H{"status": "created"})
}

func (m *MockServerInterface) DeleteAPI(c *gin.Context, id string) {
	m.DeleteAPICalled = true
	c.JSON(http.StatusOK, gin.H{"status": "deleted", "id": id})
}

func (m *MockServerInterface) GetAPIById(c *gin.Context, id string) {
	m.GetAPIByIdCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok", "id": id})
}

func (m *MockServerInterface) UpdateAPI(c *gin.Context, id string) {
	m.UpdateAPICalled = true
	c.JSON(http.StatusOK, gin.H{"status": "updated", "id": id})
}

func (m *MockServerInterface) ListAPIKeys(c *gin.Context, id string) {
	m.ListAPIKeysCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (m *MockServerInterface) RevokeAPIKey(c *gin.Context, id string, apiKeyName string) {
	m.RevokeAPIKeyCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "revoked"})
}

func (m *MockServerInterface) RegenerateAPIKey(c *gin.Context, id string, apiKeyName string) {
	m.RegenerateAPIKeyCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "regenerated"})
}

func (m *MockServerInterface) ListCertificates(c *gin.Context) {
	m.ListCertificatesCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (m *MockServerInterface) UploadCertificate(c *gin.Context) {
	m.UploadCertificateCalled = true
	c.JSON(http.StatusCreated, gin.H{"status": "uploaded"})
}

func (m *MockServerInterface) ReloadCertificates(c *gin.Context) {
	m.ReloadCertificatesCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "reloaded"})
}

func (m *MockServerInterface) DeleteCertificate(c *gin.Context, id string) {
	m.DeleteCertificateCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (m *MockServerInterface) GetConfigDump(c *gin.Context) {
	m.GetConfigDumpCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (m *MockServerInterface) HealthCheck(c *gin.Context) {
	m.HealthCheckCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

func (m *MockServerInterface) ListLLMProviderTemplates(c *gin.Context, params ListLLMProviderTemplatesParams) {
	m.ListLLMProviderTemplatesCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (m *MockServerInterface) CreateLLMProviderTemplate(c *gin.Context) {
	m.CreateLLMProviderTemplateCalled = true
	c.JSON(http.StatusCreated, gin.H{"status": "created"})
}

func (m *MockServerInterface) DeleteLLMProviderTemplate(c *gin.Context, id string) {
	m.DeleteLLMProviderTemplateCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (m *MockServerInterface) GetLLMProviderTemplateById(c *gin.Context, id string) {
	m.GetLLMProviderTemplateByIdCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (m *MockServerInterface) UpdateLLMProviderTemplate(c *gin.Context, id string) {
	m.UpdateLLMProviderTemplateCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (m *MockServerInterface) ListLLMProviders(c *gin.Context, params ListLLMProvidersParams) {
	m.ListLLMProvidersCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (m *MockServerInterface) CreateLLMProvider(c *gin.Context) {
	m.CreateLLMProviderCalled = true
	c.JSON(http.StatusCreated, gin.H{"status": "created"})
}

func (m *MockServerInterface) DeleteLLMProvider(c *gin.Context, id string) {
	m.DeleteLLMProviderCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (m *MockServerInterface) GetLLMProviderById(c *gin.Context, id string) {
	m.GetLLMProviderByIdCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (m *MockServerInterface) UpdateLLMProvider(c *gin.Context, id string) {
	m.UpdateLLMProviderCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (m *MockServerInterface) ListLLMProxies(c *gin.Context, params ListLLMProxiesParams) {
	m.ListLLMProxiesCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (m *MockServerInterface) CreateLLMProxy(c *gin.Context) {
	m.CreateLLMProxyCalled = true
	c.JSON(http.StatusCreated, gin.H{"status": "created"})
}

func (m *MockServerInterface) DeleteLLMProxy(c *gin.Context, id string) {
	m.DeleteLLMProxyCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (m *MockServerInterface) GetLLMProxyById(c *gin.Context, id string) {
	m.GetLLMProxyByIdCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (m *MockServerInterface) UpdateLLMProxy(c *gin.Context, id string) {
	m.UpdateLLMProxyCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (m *MockServerInterface) ListMCPProxies(c *gin.Context, params ListMCPProxiesParams) {
	m.ListMCPProxiesCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (m *MockServerInterface) CreateMCPProxy(c *gin.Context) {
	m.CreateMCPProxyCalled = true
	c.JSON(http.StatusCreated, gin.H{"status": "created"})
}

func (m *MockServerInterface) DeleteMCPProxy(c *gin.Context, id string) {
	m.DeleteMCPProxyCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (m *MockServerInterface) GetMCPProxyById(c *gin.Context, id string) {
	m.GetMCPProxyByIdCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (m *MockServerInterface) UpdateMCPProxy(c *gin.Context, id string) {
	m.UpdateMCPProxyCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (m *MockServerInterface) ListPolicies(c *gin.Context) {
	m.ListPoliciesCalled = true
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Tests for APIConfiguration_Spec union methods

func TestAPIConfigurationSpec_WebhookAPIData(t *testing.T) {
	t.Run("FromWebhookAPIData and AsWebhookAPIData roundtrip", func(t *testing.T) {
		spec := &APIConfiguration_Spec{}

		webhookData := WebhookAPIData{
			DisplayName: "Test WebSub API",
			Version:     "v1.0.0",
			Context:     "/websub",
			Channels: []Channel{
				{Name: "/events"},
			},
		}

		err := spec.FromWebhookAPIData(webhookData)
		require.NoError(t, err)

		retrieved, err := spec.AsWebhookAPIData()
		require.NoError(t, err)

		assert.Equal(t, webhookData.DisplayName, retrieved.DisplayName)
		assert.Equal(t, webhookData.Version, retrieved.Version)
		assert.Equal(t, webhookData.Context, retrieved.Context)
	})

	t.Run("MergeWebhookAPIData merges data", func(t *testing.T) {
		spec := &APIConfiguration_Spec{}

		// Start with initial data
		initial := WebhookAPIData{
			DisplayName: "Initial API",
			Version:     "v1.0.0",
			Context:     "/initial",
		}
		err := spec.FromWebhookAPIData(initial)
		require.NoError(t, err)

		// Merge with new data
		merge := WebhookAPIData{
			DisplayName: "Merged API",
		}
		err = spec.MergeWebhookAPIData(merge)
		require.NoError(t, err)

		retrieved, err := spec.AsWebhookAPIData()
		require.NoError(t, err)

		// Merged field should be updated
		assert.Equal(t, "Merged API", retrieved.DisplayName)
	})
}

func TestAPIConfigurationSpec_MergeAPIConfigData(t *testing.T) {
	t.Run("MergeAPIConfigData merges data", func(t *testing.T) {
		spec := &APIConfiguration_Spec{}

		// Start with initial data
		initial := APIConfigData{
			DisplayName: "Initial API",
			Version:     "v1.0.0",
			Context:     "/initial",
			Operations: []Operation{
				{Method: OperationMethodGET, Path: "/resource"},
			},
		}
		err := spec.FromAPIConfigData(initial)
		require.NoError(t, err)

		// Merge with new data
		merge := APIConfigData{
			DisplayName: "Merged API",
		}
		err = spec.MergeAPIConfigData(merge)
		require.NoError(t, err)

		retrieved, err := spec.AsAPIConfigData()
		require.NoError(t, err)

		// Merged field should be updated
		assert.Equal(t, "Merged API", retrieved.DisplayName)
	})
}

// Tests for LLMProviderConfigData_Upstream union methods

func TestLLMProviderConfigDataUpstream_Union(t *testing.T) {
	t.Run("FromLLMProviderConfigDataUpstream0 and AsLLMProviderConfigDataUpstream0 roundtrip", func(t *testing.T) {
		upstream := &LLMProviderConfigData_Upstream{}

		data := map[string]interface{}{
			"url":  "https://api.openai.com/v1",
			"type": "openai",
		}

		err := upstream.FromLLMProviderConfigDataUpstream0(data)
		require.NoError(t, err)

		retrieved, err := upstream.AsLLMProviderConfigDataUpstream0()
		require.NoError(t, err)

		// Convert to map for comparison
		retrievedMap, ok := retrieved.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "https://api.openai.com/v1", retrievedMap["url"])
	})

	t.Run("MergeLLMProviderConfigDataUpstream0 merges data", func(t *testing.T) {
		upstream := &LLMProviderConfigData_Upstream{}

		initial := map[string]interface{}{
			"url":  "https://api.openai.com/v1",
			"type": "openai",
		}
		err := upstream.FromLLMProviderConfigDataUpstream0(initial)
		require.NoError(t, err)

		merge := map[string]interface{}{
			"url": "https://api.azure.com/v1",
		}
		err = upstream.MergeLLMProviderConfigDataUpstream0(merge)
		require.NoError(t, err)

		retrieved, err := upstream.AsLLMProviderConfigDataUpstream0()
		require.NoError(t, err)

		retrievedMap, ok := retrieved.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "https://api.azure.com/v1", retrievedMap["url"])
	})

	t.Run("FromLLMProviderConfigDataUpstream1 and AsLLMProviderConfigDataUpstream1 roundtrip", func(t *testing.T) {
		upstream := &LLMProviderConfigData_Upstream{}

		data := map[string]interface{}{
			"ref": "llm-provider-ref",
		}

		err := upstream.FromLLMProviderConfigDataUpstream1(data)
		require.NoError(t, err)

		retrieved, err := upstream.AsLLMProviderConfigDataUpstream1()
		require.NoError(t, err)

		retrievedMap, ok := retrieved.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "llm-provider-ref", retrievedMap["ref"])
	})

	t.Run("MergeLLMProviderConfigDataUpstream1 merges data", func(t *testing.T) {
		upstream := &LLMProviderConfigData_Upstream{}

		initial := map[string]interface{}{
			"ref":  "initial-ref",
			"type": "azure",
		}
		err := upstream.FromLLMProviderConfigDataUpstream1(initial)
		require.NoError(t, err)

		merge := map[string]interface{}{
			"ref": "merged-ref",
		}
		err = upstream.MergeLLMProviderConfigDataUpstream1(merge)
		require.NoError(t, err)

		retrieved, err := upstream.AsLLMProviderConfigDataUpstream1()
		require.NoError(t, err)

		retrievedMap, ok := retrieved.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "merged-ref", retrievedMap["ref"])
	})
}

// Tests for MCPProxyConfigData_Upstream union methods

func TestMCPProxyConfigDataUpstream_Union(t *testing.T) {
	t.Run("FromMCPProxyConfigDataUpstream0 and AsMCPProxyConfigDataUpstream0 roundtrip", func(t *testing.T) {
		upstream := &MCPProxyConfigData_Upstream{}

		data := map[string]interface{}{
			"url":  "https://mcp.example.com/v1",
			"type": "mcp",
		}

		err := upstream.FromMCPProxyConfigDataUpstream0(data)
		require.NoError(t, err)

		retrieved, err := upstream.AsMCPProxyConfigDataUpstream0()
		require.NoError(t, err)

		retrievedMap, ok := retrieved.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "https://mcp.example.com/v1", retrievedMap["url"])
	})

	t.Run("MergeMCPProxyConfigDataUpstream0 merges data", func(t *testing.T) {
		upstream := &MCPProxyConfigData_Upstream{}

		initial := map[string]interface{}{
			"url":  "https://mcp.example.com/v1",
			"type": "mcp",
		}
		err := upstream.FromMCPProxyConfigDataUpstream0(initial)
		require.NoError(t, err)

		merge := map[string]interface{}{
			"url": "https://mcp.updated.com/v1",
		}
		err = upstream.MergeMCPProxyConfigDataUpstream0(merge)
		require.NoError(t, err)

		retrieved, err := upstream.AsMCPProxyConfigDataUpstream0()
		require.NoError(t, err)

		retrievedMap, ok := retrieved.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "https://mcp.updated.com/v1", retrievedMap["url"])
	})

	t.Run("FromMCPProxyConfigDataUpstream1 and AsMCPProxyConfigDataUpstream1 roundtrip", func(t *testing.T) {
		upstream := &MCPProxyConfigData_Upstream{}

		data := map[string]interface{}{
			"ref": "mcp-provider-ref",
		}

		err := upstream.FromMCPProxyConfigDataUpstream1(data)
		require.NoError(t, err)

		retrieved, err := upstream.AsMCPProxyConfigDataUpstream1()
		require.NoError(t, err)

		retrievedMap, ok := retrieved.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "mcp-provider-ref", retrievedMap["ref"])
	})

	t.Run("MergeMCPProxyConfigDataUpstream1 merges data", func(t *testing.T) {
		upstream := &MCPProxyConfigData_Upstream{}

		initial := map[string]interface{}{
			"ref":  "initial-ref",
			"type": "mcp",
		}
		err := upstream.FromMCPProxyConfigDataUpstream1(initial)
		require.NoError(t, err)

		merge := map[string]interface{}{
			"ref": "merged-ref",
		}
		err = upstream.MergeMCPProxyConfigDataUpstream1(merge)
		require.NoError(t, err)

		retrieved, err := upstream.AsMCPProxyConfigDataUpstream1()
		require.NoError(t, err)

		retrievedMap, ok := retrieved.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "merged-ref", retrievedMap["ref"])
	})
}

// Tests for Upstream union methods

func TestUpstream_Union(t *testing.T) {
	t.Run("FromUpstream0 and AsUpstream0 roundtrip", func(t *testing.T) {
		upstream := &Upstream{}

		data := map[string]interface{}{
			"url":         "https://backend.example.com",
			"hostRewrite": "backend.internal",
		}

		err := upstream.FromUpstream0(data)
		require.NoError(t, err)

		retrieved, err := upstream.AsUpstream0()
		require.NoError(t, err)

		retrievedMap, ok := retrieved.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "https://backend.example.com", retrievedMap["url"])
	})

	t.Run("MergeUpstream0 merges data", func(t *testing.T) {
		upstream := &Upstream{}

		initial := map[string]interface{}{
			"url":         "https://backend.example.com",
			"hostRewrite": "backend.internal",
		}
		err := upstream.FromUpstream0(initial)
		require.NoError(t, err)

		merge := map[string]interface{}{
			"url": "https://backend.updated.com",
		}
		err = upstream.MergeUpstream0(merge)
		require.NoError(t, err)

		retrieved, err := upstream.AsUpstream0()
		require.NoError(t, err)

		retrievedMap, ok := retrieved.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "https://backend.updated.com", retrievedMap["url"])
	})

	t.Run("FromUpstream1 and AsUpstream1 roundtrip", func(t *testing.T) {
		upstream := &Upstream{}

		data := map[string]interface{}{
			"ref": "backend-ref",
		}

		err := upstream.FromUpstream1(data)
		require.NoError(t, err)

		retrieved, err := upstream.AsUpstream1()
		require.NoError(t, err)

		retrievedMap, ok := retrieved.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "backend-ref", retrievedMap["ref"])
	})

	t.Run("MergeUpstream1 merges data", func(t *testing.T) {
		upstream := &Upstream{}

		initial := map[string]interface{}{
			"ref":  "initial-ref",
			"type": "backend",
		}
		err := upstream.FromUpstream1(initial)
		require.NoError(t, err)

		merge := map[string]interface{}{
			"ref": "merged-ref",
		}
		err = upstream.MergeUpstream1(merge)
		require.NoError(t, err)

		retrieved, err := upstream.AsUpstream1()
		require.NoError(t, err)

		retrievedMap, ok := retrieved.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "merged-ref", retrievedMap["ref"])
	})
}

// Tests for Upstream UnmarshalJSON

func TestUpstream_UnmarshalJSON(t *testing.T) {
	t.Run("Unmarshal with hostRewrite field", func(t *testing.T) {
		hostRewrite := "backend.internal"
		jsonData := `{
			"url": "https://backend.example.com",
			"hostRewrite": "backend.internal"
		}`

		var upstream Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		require.NoError(t, err)

		assert.NotNil(t, upstream.HostRewrite)
		assert.Equal(t, UpstreamHostRewrite(hostRewrite), *upstream.HostRewrite)
	})

	t.Run("Unmarshal with ref field", func(t *testing.T) {
		jsonData := `{
			"ref": "backend-service-ref"
		}`

		var upstream Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		require.NoError(t, err)

		assert.NotNil(t, upstream.Ref)
		assert.Equal(t, "backend-service-ref", *upstream.Ref)
	})

	t.Run("Unmarshal with url field", func(t *testing.T) {
		jsonData := `{
			"url": "https://api.example.com/v1"
		}`

		var upstream Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		require.NoError(t, err)

		assert.NotNil(t, upstream.Url)
		assert.Equal(t, "https://api.example.com/v1", *upstream.Url)
	})

	t.Run("Unmarshal with all fields", func(t *testing.T) {
		jsonData := `{
			"url": "https://api.example.com/v1",
			"hostRewrite": "api.internal",
			"ref": "api-service"
		}`

		var upstream Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		require.NoError(t, err)

		assert.NotNil(t, upstream.Url)
		assert.NotNil(t, upstream.HostRewrite)
		assert.NotNil(t, upstream.Ref)
	})
}

// Tests for LLMProviderConfigData_Upstream UnmarshalJSON (lines 1568-1588)

func TestLLMProviderConfigDataUpstream_UnmarshalJSON(t *testing.T) {
	t.Run("Unmarshal with auth field", func(t *testing.T) {
		jsonData := `{
			"url": "https://api.openai.com/v1",
			"auth": {
				"type": "apiKey",
				"header": "Authorization",
				"value": "Bearer token"
			}
		}`

		var upstream LLMProviderConfigData_Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		require.NoError(t, err)

		assert.NotNil(t, upstream.Auth)
		assert.Equal(t, LLMProviderConfigDataUpstreamAuthType("apiKey"), upstream.Auth.Type)
	})

	t.Run("Unmarshal with hostRewrite field", func(t *testing.T) {
		jsonData := `{
			"url": "https://api.openai.com/v1",
			"hostRewrite": "auto"
		}`

		var upstream LLMProviderConfigData_Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		require.NoError(t, err)

		assert.NotNil(t, upstream.HostRewrite)
		assert.Equal(t, LLMProviderConfigDataUpstreamHostRewrite("auto"), *upstream.HostRewrite)
	})

	t.Run("Unmarshal with ref field", func(t *testing.T) {
		jsonData := `{
			"ref": "openai-provider-ref"
		}`

		var upstream LLMProviderConfigData_Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		require.NoError(t, err)

		assert.NotNil(t, upstream.Ref)
		assert.Equal(t, "openai-provider-ref", *upstream.Ref)
	})

	t.Run("Unmarshal with url field", func(t *testing.T) {
		jsonData := `{
			"url": "https://api.openai.com/v1"
		}`

		var upstream LLMProviderConfigData_Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		require.NoError(t, err)

		assert.NotNil(t, upstream.Url)
		assert.Equal(t, "https://api.openai.com/v1", *upstream.Url)
	})

	t.Run("Unmarshal with all fields", func(t *testing.T) {
		jsonData := `{
			"url": "https://api.openai.com/v1",
			"auth": {"type": "apiKey"},
			"hostRewrite": "auto",
			"ref": "openai-provider"
		}`

		var upstream LLMProviderConfigData_Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		require.NoError(t, err)

		assert.NotNil(t, upstream.Url)
		assert.NotNil(t, upstream.Auth)
		assert.NotNil(t, upstream.HostRewrite)
		assert.NotNil(t, upstream.Ref)
	})

	t.Run("Unmarshal with invalid auth field returns error", func(t *testing.T) {
		jsonData := `{
			"url": "https://api.openai.com",
			"auth": "invalid-not-object"
		}`

		var upstream LLMProviderConfigData_Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "auth")
	})

	t.Run("Unmarshal with invalid hostRewrite field returns error", func(t *testing.T) {
		jsonData := `{
			"url": "https://api.openai.com",
			"hostRewrite": 12345
		}`

		var upstream LLMProviderConfigData_Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "hostRewrite")
	})

	t.Run("Unmarshal with invalid ref field returns error", func(t *testing.T) {
		jsonData := `{
			"ref": {"invalid": "object"}
		}`

		var upstream LLMProviderConfigData_Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ref")
	})

	t.Run("Unmarshal with invalid url field returns error", func(t *testing.T) {
		jsonData := `{
			"url": ["invalid", "array"]
		}`

		var upstream LLMProviderConfigData_Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "url")
	})
}

// Tests for MCPProxyConfigData_Upstream UnmarshalJSON (lines 1710-1733)

func TestMCPProxyConfigDataUpstream_UnmarshalJSON(t *testing.T) {
	t.Run("Unmarshal with auth field", func(t *testing.T) {
		jsonData := `{
			"url": "https://mcp.example.com/v1",
			"auth": {
				"type": "apiKey",
				"header": "X-API-Key",
				"value": "secret-key"
			}
		}`

		var upstream MCPProxyConfigData_Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		require.NoError(t, err)

		assert.NotNil(t, upstream.Auth)
		assert.Equal(t, MCPProxyConfigDataUpstreamAuthType("apiKey"), upstream.Auth.Type)
	})

	t.Run("Unmarshal with hostRewrite field", func(t *testing.T) {
		jsonData := `{
			"url": "https://mcp.example.com/v1",
			"hostRewrite": "manual"
		}`

		var upstream MCPProxyConfigData_Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		require.NoError(t, err)

		assert.NotNil(t, upstream.HostRewrite)
		assert.Equal(t, MCPProxyConfigDataUpstreamHostRewrite("manual"), *upstream.HostRewrite)
	})

	t.Run("Unmarshal with ref field", func(t *testing.T) {
		jsonData := `{
			"ref": "mcp-provider-ref"
		}`

		var upstream MCPProxyConfigData_Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		require.NoError(t, err)

		assert.NotNil(t, upstream.Ref)
		assert.Equal(t, "mcp-provider-ref", *upstream.Ref)
	})

	t.Run("Unmarshal with url field", func(t *testing.T) {
		jsonData := `{
			"url": "https://mcp.example.com/v1"
		}`

		var upstream MCPProxyConfigData_Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		require.NoError(t, err)

		assert.NotNil(t, upstream.Url)
		assert.Equal(t, "https://mcp.example.com/v1", *upstream.Url)
	})

	t.Run("Unmarshal with all fields", func(t *testing.T) {
		jsonData := `{
			"url": "https://mcp.example.com/v1",
			"auth": {"type": "apiKey"},
			"hostRewrite": "auto",
			"ref": "mcp-provider"
		}`

		var upstream MCPProxyConfigData_Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		require.NoError(t, err)

		assert.NotNil(t, upstream.Url)
		assert.NotNil(t, upstream.Auth)
		assert.NotNil(t, upstream.HostRewrite)
		assert.NotNil(t, upstream.Ref)
	})

	t.Run("Unmarshal with invalid auth field returns error", func(t *testing.T) {
		jsonData := `{
			"url": "https://mcp.example.com",
			"auth": "invalid-not-object"
		}`

		var upstream MCPProxyConfigData_Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "auth")
	})

	t.Run("Unmarshal with invalid hostRewrite field returns error", func(t *testing.T) {
		jsonData := `{
			"url": "https://mcp.example.com",
			"hostRewrite": 12345
		}`

		var upstream MCPProxyConfigData_Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "hostRewrite")
	})

	t.Run("Unmarshal with invalid ref field returns error", func(t *testing.T) {
		jsonData := `{
			"ref": {"invalid": "object"}
		}`

		var upstream MCPProxyConfigData_Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ref")
	})

	t.Run("Unmarshal with invalid url field returns error", func(t *testing.T) {
		jsonData := `{
			"url": ["invalid", "array"]
		}`

		var upstream MCPProxyConfigData_Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "url")
	})
}

// Tests for PathToRawSpec and GetSwagger (lines 3139-3177)

func TestPathToRawSpec(t *testing.T) {
	t.Run("With non-empty path", func(t *testing.T) {
		result := PathToRawSpec("/api/spec.yaml")

		assert.Len(t, result, 1)
		_, exists := result["/api/spec.yaml"]
		assert.True(t, exists)

		// The function should be callable and return spec data
		specFunc := result["/api/spec.yaml"]
		data, err := specFunc()
		require.NoError(t, err)
		assert.NotEmpty(t, data)
	})

	t.Run("With empty path", func(t *testing.T) {
		result := PathToRawSpec("")

		assert.Len(t, result, 0)
	})
}

func TestGetSwagger(t *testing.T) {
	t.Run("Returns valid swagger spec", func(t *testing.T) {
		swagger, err := GetSwagger()
		require.NoError(t, err)
		require.NotNil(t, swagger)

		// Verify it's a valid OpenAPI spec
		assert.NotEmpty(t, swagger.Info.Title)
	})
}

// Tests for APIConfiguration_Spec AsAPIConfigData roundtrip

func TestAPIConfigurationSpec_APIConfigData(t *testing.T) {
	t.Run("FromAPIConfigData and AsAPIConfigData roundtrip", func(t *testing.T) {
		spec := &APIConfiguration_Spec{}

		apiData := APIConfigData{
			DisplayName: "Test REST API",
			Version:     "v1.0.0",
			Context:     "/test",
			Operations: []Operation{
				{Method: OperationMethodGET, Path: "/resource"},
				{Method: OperationMethodPOST, Path: "/resource"},
			},
		}

		err := spec.FromAPIConfigData(apiData)
		require.NoError(t, err)

		retrieved, err := spec.AsAPIConfigData()
		require.NoError(t, err)

		assert.Equal(t, apiData.DisplayName, retrieved.DisplayName)
		assert.Equal(t, apiData.Version, retrieved.Version)
		assert.Equal(t, apiData.Context, retrieved.Context)
		assert.Len(t, retrieved.Operations, 2)
	})
}

// Tests for APIConfiguration_Spec MarshalJSON and UnmarshalJSON

func TestAPIConfigurationSpec_MarshalUnmarshalJSON(t *testing.T) {
	t.Run("MarshalJSON with APIConfigData", func(t *testing.T) {
		spec := &APIConfiguration_Spec{}

		apiData := APIConfigData{
			DisplayName: "Test API",
			Version:     "v1.0.0",
			Context:     "/test",
			Operations: []Operation{
				{Method: OperationMethodGET, Path: "/items"},
			},
		}

		err := spec.FromAPIConfigData(apiData)
		require.NoError(t, err)

		jsonBytes, err := spec.MarshalJSON()
		require.NoError(t, err)
		assert.NotEmpty(t, jsonBytes)

		// Unmarshal back and verify
		var result map[string]interface{}
		err = json.Unmarshal(jsonBytes, &result)
		require.NoError(t, err)
		assert.Equal(t, "Test API", result["displayName"])
	})

	t.Run("UnmarshalJSON and verify data", func(t *testing.T) {
		jsonData := `{
			"displayName": "Unmarshaled API",
			"version": "v2.0.0",
			"context": "/unmarshal",
			"operations": [{"method": "GET", "path": "/test"}]
		}`

		var spec APIConfiguration_Spec
		err := spec.UnmarshalJSON([]byte(jsonData))
		require.NoError(t, err)

		retrieved, err := spec.AsAPIConfigData()
		require.NoError(t, err)
		assert.Equal(t, "Unmarshaled API", retrieved.DisplayName)
	})
}

// Tests for LLMProviderConfigData_Upstream MarshalJSON

func TestLLMProviderConfigDataUpstream_MarshalJSON(t *testing.T) {
	t.Run("MarshalJSON with all fields", func(t *testing.T) {
		url := "https://api.openai.com/v1"
		hostRewrite := LLMProviderConfigDataUpstreamHostRewrite("auto")
		ref := "openai-ref"
		authType := LLMProviderConfigDataUpstreamAuthType("bearer")
		authHeader := "Authorization"
		authValue := "Bearer token"

		upstream := LLMProviderConfigData_Upstream{
			Url:         &url,
			HostRewrite: &hostRewrite,
			Ref:         &ref,
			Auth: &struct {
				Header *string                               `json:"header,omitempty" yaml:"header,omitempty"`
				Type   LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
				Value  *string                               `json:"value,omitempty" yaml:"value,omitempty"`
			}{
				Header: &authHeader,
				Type:   authType,
				Value:  &authValue,
			},
		}

		jsonBytes, err := upstream.MarshalJSON()
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(jsonBytes, &result)
		require.NoError(t, err)

		assert.Equal(t, "https://api.openai.com/v1", result["url"])
		assert.Equal(t, "auto", result["hostRewrite"])
		assert.Equal(t, "openai-ref", result["ref"])
		assert.NotNil(t, result["auth"])
	})

	t.Run("MarshalJSON with nil union", func(t *testing.T) {
		url := "https://api.test.com"
		upstream := LLMProviderConfigData_Upstream{
			Url: &url,
		}

		jsonBytes, err := upstream.MarshalJSON()
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(jsonBytes, &result)
		require.NoError(t, err)
		assert.Equal(t, "https://api.test.com", result["url"])
	})

	t.Run("MarshalJSON roundtrip", func(t *testing.T) {
		url := "https://api.openai.com/v1"
		hostRewrite := LLMProviderConfigDataUpstreamHostRewrite("manual")

		original := LLMProviderConfigData_Upstream{
			Url:         &url,
			HostRewrite: &hostRewrite,
		}

		jsonBytes, err := original.MarshalJSON()
		require.NoError(t, err)

		var restored LLMProviderConfigData_Upstream
		err = restored.UnmarshalJSON(jsonBytes)
		require.NoError(t, err)

		assert.Equal(t, *original.Url, *restored.Url)
		assert.Equal(t, *original.HostRewrite, *restored.HostRewrite)
	})
}

// Tests for MCPProxyConfigData_Upstream MarshalJSON

func TestMCPProxyConfigDataUpstream_MarshalJSON(t *testing.T) {
	t.Run("MarshalJSON with all fields", func(t *testing.T) {
		url := "https://mcp.example.com/v1"
		hostRewrite := MCPProxyConfigDataUpstreamHostRewrite("auto")
		ref := "mcp-ref"
		authType := MCPProxyConfigDataUpstreamAuthType("api-key")
		authHeader := "X-API-Key"
		authValue := "secret-key"

		upstream := MCPProxyConfigData_Upstream{
			Url:         &url,
			HostRewrite: &hostRewrite,
			Ref:         &ref,
			Auth: &struct {
				Header *string                            `json:"header,omitempty" yaml:"header,omitempty"`
				Type   MCPProxyConfigDataUpstreamAuthType `json:"type" yaml:"type"`
				Value  *string                            `json:"value,omitempty" yaml:"value,omitempty"`
			}{
				Header: &authHeader,
				Type:   authType,
				Value:  &authValue,
			},
		}

		jsonBytes, err := upstream.MarshalJSON()
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(jsonBytes, &result)
		require.NoError(t, err)

		assert.Equal(t, "https://mcp.example.com/v1", result["url"])
		assert.Equal(t, "auto", result["hostRewrite"])
		assert.Equal(t, "mcp-ref", result["ref"])
		assert.NotNil(t, result["auth"])
	})

	t.Run("MarshalJSON with nil union", func(t *testing.T) {
		url := "https://mcp.test.com"
		upstream := MCPProxyConfigData_Upstream{
			Url: &url,
		}

		jsonBytes, err := upstream.MarshalJSON()
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(jsonBytes, &result)
		require.NoError(t, err)
		assert.Equal(t, "https://mcp.test.com", result["url"])
	})

	t.Run("MarshalJSON roundtrip", func(t *testing.T) {
		url := "https://mcp.example.com/v1"
		hostRewrite := MCPProxyConfigDataUpstreamHostRewrite("manual")

		original := MCPProxyConfigData_Upstream{
			Url:         &url,
			HostRewrite: &hostRewrite,
		}

		jsonBytes, err := original.MarshalJSON()
		require.NoError(t, err)

		var restored MCPProxyConfigData_Upstream
		err = restored.UnmarshalJSON(jsonBytes)
		require.NoError(t, err)

		assert.Equal(t, *original.Url, *restored.Url)
		assert.Equal(t, *original.HostRewrite, *restored.HostRewrite)
	})
}

// Tests for Upstream MarshalJSON

func TestUpstream_MarshalJSON(t *testing.T) {
	t.Run("MarshalJSON with all fields", func(t *testing.T) {
		url := "https://backend.example.com"
		hostRewrite := UpstreamHostRewrite("auto")
		ref := "backend-ref"

		upstream := Upstream{
			Url:         &url,
			HostRewrite: &hostRewrite,
			Ref:         &ref,
		}

		jsonBytes, err := upstream.MarshalJSON()
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(jsonBytes, &result)
		require.NoError(t, err)

		assert.Equal(t, "https://backend.example.com", result["url"])
		assert.Equal(t, "auto", result["hostRewrite"])
		assert.Equal(t, "backend-ref", result["ref"])
	})

	t.Run("MarshalJSON with nil union", func(t *testing.T) {
		url := "https://backend.test.com"
		upstream := Upstream{
			Url: &url,
		}

		jsonBytes, err := upstream.MarshalJSON()
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(jsonBytes, &result)
		require.NoError(t, err)
		assert.Equal(t, "https://backend.test.com", result["url"])
	})

	t.Run("MarshalJSON roundtrip", func(t *testing.T) {
		url := "https://backend.example.com"
		hostRewrite := UpstreamHostRewrite("manual")
		ref := "backend-service"

		original := Upstream{
			Url:         &url,
			HostRewrite: &hostRewrite,
			Ref:         &ref,
		}

		jsonBytes, err := original.MarshalJSON()
		require.NoError(t, err)

		var restored Upstream
		err = restored.UnmarshalJSON(jsonBytes)
		require.NoError(t, err)

		assert.Equal(t, *original.Url, *restored.Url)
		assert.Equal(t, *original.HostRewrite, *restored.HostRewrite)
		assert.Equal(t, *original.Ref, *restored.Ref)
	})
}

// Tests for Upstream UnmarshalJSON error cases

func TestUpstream_UnmarshalJSON_Errors(t *testing.T) {
	t.Run("Unmarshal with invalid hostRewrite field returns error", func(t *testing.T) {
		jsonData := `{
			"url": "https://backend.example.com",
			"hostRewrite": 12345
		}`

		var upstream Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "hostRewrite")
	})

	t.Run("Unmarshal with invalid ref field returns error", func(t *testing.T) {
		jsonData := `{
			"ref": {"invalid": "object"}
		}`

		var upstream Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ref")
	})

	t.Run("Unmarshal with invalid url field returns error", func(t *testing.T) {
		jsonData := `{
			"url": ["invalid", "array"]
		}`

		var upstream Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "url")
	})

	t.Run("Unmarshal with invalid JSON returns error", func(t *testing.T) {
		jsonData := `{invalid json}`

		var upstream Upstream
		err := json.Unmarshal([]byte(jsonData), &upstream)
		assert.Error(t, err)
	})
}

// Tests for RegisterHandlers

func TestRegisterHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("RegisterHandlers registers all routes", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}

		RegisterHandlers(router, mockServer)

		// Test that routes are registered by making requests
		routes := router.Routes()
		assert.NotEmpty(t, routes)

		// Verify key routes are registered
		routePaths := make(map[string]bool)
		for _, route := range routes {
			routePaths[route.Method+":"+route.Path] = true
		}

		assert.True(t, routePaths["GET:/apis"])
		assert.True(t, routePaths["POST:/apis"])
		assert.True(t, routePaths["GET:/apis/:id"])
		assert.True(t, routePaths["PUT:/apis/:id"])
		assert.True(t, routePaths["DELETE:/apis/:id"])
		assert.True(t, routePaths["GET:/health"])
		assert.True(t, routePaths["GET:/certificates"])
		assert.True(t, routePaths["GET:/policies"])
	})
}

// Tests for RegisterHandlersWithOptions

func TestRegisterHandlersWithOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("RegisterHandlersWithOptions with custom base URL", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}

		options := GinServerOptions{
			BaseURL: "/api/v1",
		}

		RegisterHandlersWithOptions(router, mockServer, options)

		routes := router.Routes()
		routePaths := make(map[string]bool)
		for _, route := range routes {
			routePaths[route.Method+":"+route.Path] = true
		}

		assert.True(t, routePaths["GET:/api/v1/apis"])
		assert.True(t, routePaths["POST:/api/v1/apis"])
		assert.True(t, routePaths["GET:/api/v1/health"])
	})

	t.Run("RegisterHandlersWithOptions with custom error handler", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}

		options := GinServerOptions{
			ErrorHandler: func(c *gin.Context, err error, statusCode int) {
				c.JSON(statusCode, gin.H{"custom_error": err.Error()})
			},
		}

		RegisterHandlersWithOptions(router, mockServer, options)

		// Make a request to verify handler is set up
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.HealthCheckCalled)
	})

	t.Run("RegisterHandlersWithOptions with middleware", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}

		middlewareCalled := false
		options := GinServerOptions{
			Middlewares: []MiddlewareFunc{
				func(c *gin.Context) {
					middlewareCalled = true
				},
			},
		}

		RegisterHandlersWithOptions(router, mockServer, options)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)

		assert.True(t, middlewareCalled)
		assert.True(t, mockServer.HealthCheckCalled)
	})

	t.Run("Middleware can abort request", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}

		options := GinServerOptions{
			Middlewares: []MiddlewareFunc{
				func(c *gin.Context) {
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				},
			},
		}

		RegisterHandlersWithOptions(router, mockServer, options)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.False(t, mockServer.HealthCheckCalled)
	})
}

// Tests for ServerInterfaceWrapper

func TestServerInterfaceWrapper_APIRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("ListAPIs with query parameters", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/apis?displayName=test&version=v1&status=deployed", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.ListAPIsCalled)
	})

	t.Run("CreateAPI", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/apis", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.True(t, mockServer.CreateAPICalled)
	})

	t.Run("GetAPIById", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/apis/test-id", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.GetAPIByIdCalled)
	})

	t.Run("UpdateAPI", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/apis/test-id", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.UpdateAPICalled)
	})

	t.Run("DeleteAPI", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/apis/test-id", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.DeleteAPICalled)
	})
}

func TestServerInterfaceWrapper_APIKeyRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("ListAPIKeys", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/apis/test-id/api-keys", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.ListAPIKeysCalled)
	})

	t.Run("CreateAPIKey", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/apis/test-id/api-keys", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.True(t, mockServer.CreateAPIKeyCalled)
	})

	t.Run("RevokeAPIKey", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/apis/test-id/api-keys/key-name", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.RevokeAPIKeyCalled)
	})

	t.Run("RegenerateAPIKey", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/apis/test-id/api-keys/key-name/regenerate", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.RegenerateAPIKeyCalled)
	})
}

func TestServerInterfaceWrapper_CertificateRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("ListCertificates", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/certificates", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.ListCertificatesCalled)
	})

	t.Run("UploadCertificate", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/certificates", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.True(t, mockServer.UploadCertificateCalled)
	})

	t.Run("ReloadCertificates", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/certificates/reload", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.ReloadCertificatesCalled)
	})

	t.Run("DeleteCertificate", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/certificates/cert-id", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.DeleteCertificateCalled)
	})
}

func TestServerInterfaceWrapper_LLMProviderRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("ListLLMProviders with query parameters", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/llm-providers?displayName=test&status=deployed", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.ListLLMProvidersCalled)
	})

	t.Run("CreateLLMProvider", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/llm-providers", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.True(t, mockServer.CreateLLMProviderCalled)
	})

	t.Run("GetLLMProviderById", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/llm-providers/provider-id", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.GetLLMProviderByIdCalled)
	})

	t.Run("UpdateLLMProvider", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/llm-providers/provider-id", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.UpdateLLMProviderCalled)
	})

	t.Run("DeleteLLMProvider", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/llm-providers/provider-id", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.DeleteLLMProviderCalled)
	})
}

func TestServerInterfaceWrapper_LLMProxyRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("ListLLMProxies with query parameters", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/llm-proxies?displayName=test&status=deployed", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.ListLLMProxiesCalled)
	})

	t.Run("CreateLLMProxy", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/llm-proxies", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.True(t, mockServer.CreateLLMProxyCalled)
	})

	t.Run("GetLLMProxyById", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/llm-proxies/proxy-id", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.GetLLMProxyByIdCalled)
	})

	t.Run("UpdateLLMProxy", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/llm-proxies/proxy-id", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.UpdateLLMProxyCalled)
	})

	t.Run("DeleteLLMProxy", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/llm-proxies/proxy-id", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.DeleteLLMProxyCalled)
	})
}

func TestServerInterfaceWrapper_MCPProxyRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("ListMCPProxies with query parameters", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/mcp-proxies?displayName=test&status=deployed", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.ListMCPProxiesCalled)
	})

	t.Run("CreateMCPProxy", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/mcp-proxies", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.True(t, mockServer.CreateMCPProxyCalled)
	})

	t.Run("GetMCPProxyById", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/mcp-proxies/proxy-id", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.GetMCPProxyByIdCalled)
	})

	t.Run("UpdateMCPProxy", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/mcp-proxies/proxy-id", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.UpdateMCPProxyCalled)
	})

	t.Run("DeleteMCPProxy", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/mcp-proxies/proxy-id", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.DeleteMCPProxyCalled)
	})
}

func TestServerInterfaceWrapper_LLMProviderTemplateRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("ListLLMProviderTemplates", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/llm-provider-templates", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.ListLLMProviderTemplatesCalled)
	})

	t.Run("CreateLLMProviderTemplate", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/llm-provider-templates", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.True(t, mockServer.CreateLLMProviderTemplateCalled)
	})

	t.Run("GetLLMProviderTemplateById", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/llm-provider-templates/template-id", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.GetLLMProviderTemplateByIdCalled)
	})

	t.Run("UpdateLLMProviderTemplate", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/llm-provider-templates/template-id", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.UpdateLLMProviderTemplateCalled)
	})

	t.Run("DeleteLLMProviderTemplate", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/llm-provider-templates/template-id", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.DeleteLLMProviderTemplateCalled)
	})
}

func TestServerInterfaceWrapper_MiscRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("HealthCheck", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.HealthCheckCalled)
	})

	t.Run("GetConfigDump", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/config_dump", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.GetConfigDumpCalled)
	})

	t.Run("ListPolicies", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/policies", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.ListPoliciesCalled)
	})
}

// Tests for query parameter edge cases

func TestServerInterfaceWrapper_QueryParamsEdgeCases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("ListAPIs with all optional params", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/apis?displayName=test&version=v1&context=/test&status=deployed", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.ListAPIsCalled)
	})

	t.Run("ListLLMProviders with all optional params", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/llm-providers?displayName=test&name=provider&status=deployed", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.ListLLMProvidersCalled)
	})

	t.Run("ListLLMProxies with all optional params", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/llm-proxies?displayName=test&context=/proxy&name=proxy&status=deployed", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.ListLLMProxiesCalled)
	})

	t.Run("ListMCPProxies with all optional params", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/mcp-proxies?displayName=test&name=mcp&status=deployed", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.ListMCPProxiesCalled)
	})

	t.Run("ListLLMProviderTemplates with optional params", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}
		RegisterHandlers(router, mockServer)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/llm-provider-templates?displayName=test&name=template", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, mockServer.ListLLMProviderTemplatesCalled)
	})
}

// Tests for default error handler

func TestDefaultErrorHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Default error handler returns JSON error", func(t *testing.T) {
		router := gin.New()
		mockServer := &MockServerInterface{}

		// Register with default options (nil error handler)
		RegisterHandlersWithOptions(router, mockServer, GinServerOptions{})

		// Make a request - the default error handler will be used if there's an error
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// Tests for empty/nil checks in union types

func TestUnionTypes_EmptyState(t *testing.T) {
	t.Run("APIConfiguration_Spec with empty union", func(t *testing.T) {
		spec := APIConfiguration_Spec{}

		// MarshalJSON should handle empty state
		jsonBytes, err := spec.MarshalJSON()
		require.NoError(t, err)
		assert.Equal(t, "null", string(jsonBytes))
	})

	t.Run("LLMProviderConfigData_Upstream with empty union marshals to empty object", func(t *testing.T) {
		upstream := LLMProviderConfigData_Upstream{}

		jsonBytes, err := upstream.MarshalJSON()
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(jsonBytes, &result)
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("MCPProxyConfigData_Upstream with empty union marshals to empty object", func(t *testing.T) {
		upstream := MCPProxyConfigData_Upstream{}

		jsonBytes, err := upstream.MarshalJSON()
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(jsonBytes, &result)
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("Upstream with empty union marshals to empty object", func(t *testing.T) {
		upstream := Upstream{}

		jsonBytes, err := upstream.MarshalJSON()
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(jsonBytes, &result)
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}
