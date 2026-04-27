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

package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/constants"
	commonmodels "github.com/wso2/api-platform/common/models"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// Helper to create server with LLM deployment service
func createTestServerWithLLM() *APIServer {
	server := createTestAPIServer()
	// Create LLM deployment service with test infrastructure
	policyVersionResolver := utils.NewLoadedPolicyVersionResolver(server.policyDefinitions)
	policyValidator := config.NewPolicyValidator(server.policyDefinitions)
	llmService := utils.NewLLMDeploymentService(
		server.store,
		server.db,
		nil, // snapshotManager
		nil, // lazyResourceManager
		make(map[string]*api.LLMProviderTemplate),
		server.deploymentService,
		server.routerConfig,
		policyVersionResolver,
		policyValidator,
	)
	server.llmDeploymentService = llmService
	return server
}

// TestListAPIKeysSuccess tests successful API key listing
func TestListAPIKeysSuccess(t *testing.T) {
	server := createTestAPIServer()

	// Create test API configuration
	apiConfig := createTestStoredConfig("0000-test-handle-0000-000000000000", "Test API", "1.0.0", "/test")
	server.db.(*MockStorage).configs["0000-test-handle-0000-000000000000"] = apiConfig
	server.store.Add(apiConfig)

	// Create test API keys
	key1 := &models.APIKey{
		UUID:         "0000-key1-0000-000000000000",
		Name:         "0000-key1-0000-000000000000",
		APIKey:       "hashed-key-1",
		MaskedAPIKey: "***key-1",
		ArtifactUUID: "0000-test-handle-0000-000000000000",
		Status:       models.APIKeyStatusActive,
		CreatedAt:    time.Now(),
		CreatedBy:    "test-user",
		UpdatedAt:    time.Now(),
	}
	key2 := &models.APIKey{
		UUID:         "0000-key2-0000-000000000000",
		Name:         "0000-key2-0000-000000000000",
		APIKey:       "hashed-key-2",
		MaskedAPIKey: "***key-2",
		ArtifactUUID: "0000-test-handle-0000-000000000000",
		Status:       models.APIKeyStatusActive,
		CreatedAt:    time.Now(),
		CreatedBy:    "test-user",
		UpdatedAt:    time.Now(),
	}

	server.db.(*MockStorage).apiKeys["0000-key1-0000-000000000000"] = key1
	server.db.(*MockStorage).apiKeys["0000-key2-0000-000000000000"] = key2

	c, w := createTestContext("GET", "/rest-apis/test-handle/api-keys", nil)
	c.Set(constants.AuthContextKey, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})

	server.ListAPIKeys(c, "0000-test-handle-0000-000000000000")

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response["status"])
}

// TestListAPIKeysAPINotFound tests listing keys for non-existent API
func TestListAPIKeysAPINotFound(t *testing.T) {
	server := createTestAPIServer()

	c, w := createTestContext("GET", "/rest-apis/nonexistent/api-keys", nil)
	c.Set(constants.AuthContextKey, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})

	server.ListAPIKeys(c, "nonexistent")

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response api.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "error", response.Status)
}

// TestListLLMProviderTemplatesEmpty tests listing with no templates
func TestListLLMProviderTemplatesEmpty(t *testing.T) {
	server := createTestServerWithLLM()

	c, w := createTestContext("GET", "/llm-provider-templates", nil)
	server.ListLLMProviderTemplates(c, api.ListLLMProviderTemplatesParams{})

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response["status"])
	assert.Equal(t, float64(0), response["count"])
}

// TestListLLMProviderTemplatesWithData tests listing with templates
func TestListLLMProviderTemplatesWithData(t *testing.T) {
	server := createTestServerWithLLM()

	now := time.Now()
	template1 := &models.StoredLLMProviderTemplate{
		UUID: "0000-template1-0000-000000000000",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{
				Name: "0000-template1-0000-000000000000",
			},
			Spec: api.LLMProviderTemplateData{
				DisplayName: "OpenAI Template",
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	template2 := &models.StoredLLMProviderTemplate{
		UUID: "0000-template2-0000-000000000000",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{
				Name: "0000-template2-0000-000000000000",
			},
			Spec: api.LLMProviderTemplateData{
				DisplayName: "Claude Template",
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Add templates to store
	require.NoError(t, server.db.SaveLLMProviderTemplate(template1))
	require.NoError(t, server.db.SaveLLMProviderTemplate(template2))

	c, w := createTestContext("GET", "/llm-provider-templates", nil)
	server.ListLLMProviderTemplates(c, api.ListLLMProviderTemplatesParams{})

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response["status"])
	assert.Equal(t, float64(2), response["count"])

	templates := response["templates"].([]interface{})
	assert.Len(t, templates, 2)
}

// TestGetLLMProviderTemplateByIdSuccess tests getting a template by ID
func TestGetLLMProviderTemplateByIdSuccess(t *testing.T) {
	server := createTestServerWithLLM()

	now := time.Now()
	template := &models.StoredLLMProviderTemplate{
		UUID: "0000-template1-0000-000000000000",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{
				Name: "0000-template1-0000-000000000000",
			},
			Spec: api.LLMProviderTemplateData{
				DisplayName: "OpenAI Template",
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	require.NoError(t, server.db.SaveLLMProviderTemplate(template))
	require.NoError(t, server.store.AddTemplate(template))

	c, w := createTestContext("GET", "/llm-provider-templates/template1", nil)
	server.GetLLMProviderTemplateById(c, "0000-template1-0000-000000000000")

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// GET returns the k8s-shaped resource body directly. The server-managed
	// status block carries the handle (name) as `id`.
	metadata, ok := response["metadata"].(map[string]interface{})
	require.True(t, ok, "metadata should be a map, got %T", response["metadata"])
	assert.Equal(t, "0000-template1-0000-000000000000", metadata["name"])
	status, ok := response["status"].(map[string]interface{})
	require.True(t, ok, "status should be a map, got %T", response["status"])
	assert.Equal(t, "0000-template1-0000-000000000000", status["id"])
}

// TestListLLMProvidersEmpty tests listing with no providers
func TestListLLMProvidersEmpty(t *testing.T) {
	server := createTestServerWithLLM()

	c, w := createTestContext("GET", "/llm-providers", nil)
	server.ListLLMProviders(c, api.ListLLMProvidersParams{})

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response["status"])
	assert.Equal(t, float64(0), response["count"])
}

// TestListLLMProvidersWithData tests listing with providers
func TestListLLMProvidersWithData(t *testing.T) {
	server := createTestServerWithLLM()

	now := time.Now()
	provider := &models.StoredConfig{
		UUID:         "0000-provider1-0000-000000000000",
		Kind:         "LlmProvider",
		Handle:       "openai-provider",
		DisplayName:  "OpenAI Provider",
		Version:      "1.0.0",
		DesiredState: "deployed",
		Origin:       models.OriginGatewayAPI,
		SourceConfiguration: api.LLMProviderConfiguration{
			ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LLMProviderConfigurationKindLlmProvider,
			Metadata: api.Metadata{
				Name: "openai-provider",
			},
			Spec: api.LLMProviderConfigData{
				DisplayName: "OpenAI Provider",
				Version:     "1.0.0",
				Template:    "openai-template",
				Upstream: api.LLMProviderConfigData_Upstream{
					Url: stringPtr("https://example.com"),
				},
				AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	require.NoError(t, server.store.Add(provider))
	require.NoError(t, server.db.SaveConfig(provider))

	c, w := createTestContext("GET", "/llm-providers", nil)
	server.ListLLMProviders(c, api.ListLLMProvidersParams{})

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response["status"])
	assert.Equal(t, float64(1), response["count"])

	providers := response["providers"].([]interface{})
	assert.Len(t, providers, 1)
}

// TestListLLMProxiesEmpty tests listing with no proxies
func TestListLLMProxiesEmpty(t *testing.T) {
	server := createTestServerWithLLM()

	c, w := createTestContext("GET", "/llm-proxies", nil)
	server.ListLLMProxies(c, api.ListLLMProxiesParams{})

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response["status"])
	assert.Equal(t, float64(0), response["count"])
}

// TestListLLMProxiesWithData tests listing with proxies
func TestListLLMProxiesWithData(t *testing.T) {
	server := createTestServerWithLLM()

	now := time.Now()
	proxy := &models.StoredConfig{
		UUID:         "0000-proxy1-0000-000000000000",
		Kind:         "LlmProxy",
		Handle:       "0000-llm-proxy-1-0000-000000000000",
		DisplayName:  "LLM Proxy 1",
		Version:      "1.0.0",
		DesiredState: "deployed",
		Origin:       models.OriginGatewayAPI,
		SourceConfiguration: api.LLMProxyConfiguration{
			ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LLMProxyConfigurationKindLlmProxy,
			Metadata: api.Metadata{
				Name: "0000-llm-proxy-1-0000-000000000000",
			},
			Spec: api.LLMProxyConfigData{
				DisplayName: "LLM Proxy 1",
				Version:     "1.0.0",
				Provider: api.LLMProxyProvider{
					Id: "openai-provider",
				},
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	require.NoError(t, server.store.Add(proxy))
	require.NoError(t, server.db.SaveConfig(proxy))

	c, w := createTestContext("GET", "/llm-proxies", nil)
	server.ListLLMProxies(c, api.ListLLMProxiesParams{})

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response["status"])
	assert.Equal(t, float64(1), response["count"])

	proxies := response["proxies"].([]interface{})
	assert.Len(t, proxies, 1)
}
