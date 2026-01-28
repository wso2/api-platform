package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
)

func TestLabelValidationForAllTypes(t *testing.T) {
	// Test LlmProvider with invalid labels
	t.Run("LlmProvider with invalid labels", func(t *testing.T) {
		validator := NewLLMValidator()
		provider := api.LLMProviderConfiguration{
			ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
			Kind:       api.LlmProvider,
			Metadata: api.Metadata{
				Name: "test-provider",
				Labels: &map[string]string{
					"Invalid Key": "value",             // Space in key should be invalid
					"valid-key":   "value with spaces", // Value with spaces should be OK
				},
			},
			Spec: api.LLMProviderConfigData{
				DisplayName: "Test Provider",
				Version:     "v1.0",
				Template:    "openai",
				Upstream: api.LLMProviderConfigData_Upstream{
					Url: stringPtr("https://api.openai.com"),
				},
				AccessControl: api.LLMAccessControl{
					Mode: api.AllowAll,
				},
			},
		}

		errors := validator.Validate(&provider)

		// Check that there is at least one error about the invalid label key
		hasLabelError := false
		for _, err := range errors {
			if err.Field == "metadata.labels" &&
				err.Message == "Label key 'Invalid Key' contains spaces. Label keys must not contain spaces." {
				hasLabelError = true
				break
			}
		}

		assert.True(t, hasLabelError, "LlmProvider should reject labels with spaces in keys")
	})

	// Test LlmProvider with valid labels
	t.Run("LlmProvider with valid labels", func(t *testing.T) {
		validator := NewLLMValidator()
		provider := api.LLMProviderConfiguration{
			ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
			Kind:       api.LlmProvider,
			Metadata: api.Metadata{
				Name: "test-provider",
				Labels: &map[string]string{
					"valid-key":   "value",
					"another_key": "value with spaces", // Value with spaces should be OK
					"environment": "test",
				},
			},
			Spec: api.LLMProviderConfigData{
				DisplayName: "Test Provider",
				Version:     "v1.0",
				Template:    "openai",
				Upstream: api.LLMProviderConfigData_Upstream{
					Url: stringPtr("https://api.openai.com"),
				},
				AccessControl: api.LLMAccessControl{
					Mode: api.AllowAll,
				},
			},
		}

		errors := validator.Validate(&provider)

		// Check that there are no label validation errors
		hasLabelError := false
		for _, err := range errors {
			if err.Field == "metadata.labels" {
				hasLabelError = true
				break
			}
		}

		assert.False(t, hasLabelError, "LlmProvider should accept valid labels")
	})

	// Test LlmProxy with invalid labels
	t.Run("LlmProxy with invalid labels", func(t *testing.T) {
		validator := NewLLMValidator()
		proxy := api.LLMProxyConfiguration{
			ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
			Kind:       api.LlmProxy,
			Metadata: api.Metadata{
				Name: "test-proxy",
				Labels: &map[string]string{
					"Invalid Key": "value", // Space in key should be invalid
				},
			},
			Spec: api.LLMProxyConfigData{
				DisplayName: "Test Proxy",
				Version:     "v1.0",
				Provider:    "test-provider",
			},
		}

		errors := validator.Validate(&proxy)

		// Check that there is at least one error about the invalid label key
		hasLabelError := false
		for _, err := range errors {
			if err.Field == "metadata.labels" &&
				err.Message == "Label key 'Invalid Key' contains spaces. Label keys must not contain spaces." {
				hasLabelError = true
				break
			}
		}

		assert.True(t, hasLabelError, "LlmProxy should reject labels with spaces in keys")
	})

	// Test MCPProxyConfiguration with invalid labels
	t.Run("MCPProxyConfiguration with invalid labels", func(t *testing.T) {
		validator := NewMCPValidator()
		mcp := api.MCPProxyConfiguration{
			ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
			Kind:       "Mcp",
			Metadata: api.Metadata{
				Name: "test-mcp",
				Labels: &map[string]string{
					"Invalid Key": "value", // Space in key should be invalid
				},
			},
			Spec: api.MCPProxyConfigData{
				DisplayName: "Test MCP",
				Version:     "v1.0",
				Upstream: api.MCPProxyConfigData_Upstream{
					Url: stringPtr("https://api.example.com"),
				},
			},
		}

		errors := validator.Validate(&mcp)

		// Check that there is at least one error about the invalid label key
		hasLabelError := false
		for _, err := range errors {
			if err.Field == "metadata.labels" &&
				err.Message == "Label key 'Invalid Key' contains spaces. Label keys must not contain spaces." {
				hasLabelError = true
				break
			}
		}

		assert.True(t, hasLabelError, "MCPProxyConfiguration should reject labels with spaces in keys")
	})

	// Test MCPProxyConfiguration with valid labels
	t.Run("MCPProxyConfiguration with valid labels", func(t *testing.T) {
		validator := NewMCPValidator()
		mcp := api.MCPProxyConfiguration{
			ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
			Kind:       "Mcp",
			Metadata: api.Metadata{
				Name: "test-mcp",
				Labels: &map[string]string{
					"valid-key":   "value",
					"environment": "test",
				},
			},
			Spec: api.MCPProxyConfigData{
				DisplayName: "Test MCP",
				Version:     "v1.0",
				Upstream: api.MCPProxyConfigData_Upstream{
					Url: stringPtr("https://api.example.com"),
				},
			},
		}

		errors := validator.Validate(&mcp)

		// Check that there are no label validation errors
		hasLabelError := false
		for _, err := range errors {
			if err.Field == "metadata.labels" {
				hasLabelError = true
				break
			}
		}

		assert.False(t, hasLabelError, "MCPProxyConfiguration should accept valid labels")
	})
}
