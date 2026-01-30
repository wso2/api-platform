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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
