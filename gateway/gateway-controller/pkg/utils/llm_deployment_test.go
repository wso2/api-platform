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

package utils

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

func TestLLMDeploymentParams(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	params := LLMDeploymentParams{
		Data:          []byte("test data"),
		ContentType:   "application/yaml",
		ID:            "test-llm-id",
		CorrelationID: "corr-123",
		Logger:        logger,
	}

	assert.Equal(t, "test data", string(params.Data))
	assert.Equal(t, "application/yaml", params.ContentType)
	assert.Equal(t, "test-llm-id", params.ID)
	assert.Equal(t, "corr-123", params.CorrelationID)
	assert.NotNil(t, params.Logger)
}

func TestLLMTemplateParams(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	params := LLMTemplateParams{
		Spec:        []byte("test spec"),
		ContentType: "application/yaml",
		Logger:      logger,
	}

	assert.Equal(t, "test spec", string(params.Spec))
	assert.Equal(t, "application/yaml", params.ContentType)
	assert.NotNil(t, params.Logger)
}

func TestLLMDeploymentService_ListLLMProviders(t *testing.T) {
	t.Run("Empty store returns empty list", func(t *testing.T) {
		store := storage.NewConfigStore()
		routerConfig := &config.RouterConfig{ListenerPort: 8080}
		apiDeploymentService := NewAPIDeploymentService(store, nil, nil, nil, nil)
		service := NewLLMDeploymentService(store, nil, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

		providers := service.ListLLMProviders(api.ListLLMProvidersParams{})
		assert.Empty(t, providers)
	})

	t.Run("Returns only LLM provider kind configs", func(t *testing.T) {
		store := storage.NewConfigStore()
		routerConfig := &config.RouterConfig{ListenerPort: 8080}
		apiDeploymentService := NewAPIDeploymentService(store, nil, nil, nil, nil)
		service := NewLLMDeploymentService(store, nil, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

		// Add an LLM provider config
		apiData := api.APIConfigData{
			DisplayName: "LLM Provider",
			Version:     "1.0.0",
			Context:     "/llm",
		}
		var spec api.APIConfiguration_Spec
		require.NoError(t, spec.FromAPIConfigData(apiData))

		llmConfig := &models.StoredConfig{
			ID:   "llm-provider-1",
			Kind: string(api.LlmProvider),
			Configuration: api.APIConfiguration{
				Kind:     api.RestApi,
				Metadata: api.Metadata{Name: "llm-provider-1"},
				Spec:     spec,
			},
			Status:    models.StatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		store.Add(llmConfig)

		providers := service.ListLLMProviders(api.ListLLMProvidersParams{})
		assert.Len(t, providers, 1)
		assert.Equal(t, "llm-provider-1", providers[0].ID)
	})

	t.Run("Filter by display name", func(t *testing.T) {
		store := storage.NewConfigStore()
		routerConfig := &config.RouterConfig{ListenerPort: 8080}
		apiDeploymentService := NewAPIDeploymentService(store, nil, nil, nil, nil)
		service := NewLLMDeploymentService(store, nil, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

		// Add first provider
		apiData1 := api.APIConfigData{
			DisplayName: "First Provider",
			Version:     "1.0.0",
			Context:     "/first",
		}
		var spec1 api.APIConfiguration_Spec
		require.NoError(t, spec1.FromAPIConfigData(apiData1))

		config1 := &models.StoredConfig{
			ID:   "llm-provider-1",
			Kind: string(api.LlmProvider),
			Configuration: api.APIConfiguration{
				Kind:     api.RestApi,
				Metadata: api.Metadata{Name: "first-provider"},
				Spec:     spec1,
			},
			Status:    models.StatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		store.Add(config1)

		// Add second provider with different display name
		apiData2 := api.APIConfigData{
			DisplayName: "Filtered Provider",
			Version:     "1.0.0",
			Context:     "/filtered",
		}
		var spec2 api.APIConfiguration_Spec
		require.NoError(t, spec2.FromAPIConfigData(apiData2))

		config2 := &models.StoredConfig{
			ID:   "llm-provider-2",
			Kind: string(api.LlmProvider),
			Configuration: api.APIConfiguration{
				Kind:     api.RestApi,
				Metadata: api.Metadata{Name: "filtered-provider"},
				Spec:     spec2,
			},
			Status:    models.StatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		store.Add(config2)

		displayName := "Filtered Provider"
		providers := service.ListLLMProviders(api.ListLLMProvidersParams{
			DisplayName: &displayName,
		})
		assert.Len(t, providers, 1)
		assert.Equal(t, "llm-provider-2", providers[0].ID)
	})
}

func TestLLMDeploymentService_ListLLMProxies(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := NewAPIDeploymentService(store, nil, nil, nil, nil)
	service := NewLLMDeploymentService(store, nil, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

	t.Run("Empty store returns empty list", func(t *testing.T) {
		proxies := service.ListLLMProxies(api.ListLLMProxiesParams{})
		assert.Empty(t, proxies)
	})

	t.Run("Returns only LLM proxy kind configs", func(t *testing.T) {
		// Add an LLM proxy config
		apiData := api.APIConfigData{
			DisplayName: "LLM Proxy",
			Version:     "1.0.0",
			Context:     "/llm-proxy",
		}
		var spec api.APIConfiguration_Spec
		spec.FromAPIConfigData(apiData)

		llmProxyConfig := &models.StoredConfig{
			ID:   "llm-proxy-1",
			Kind: string(api.LlmProxy),
			Configuration: api.APIConfiguration{
				Kind: api.RestApi,
				Spec: spec,
			},
			Status:    models.StatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		store.Add(llmProxyConfig)

		proxies := service.ListLLMProxies(api.ListLLMProxiesParams{})
		assert.Len(t, proxies, 1)
		assert.Equal(t, "llm-proxy-1", proxies[0].ID)
	})
}

func TestLLMDeploymentService_ListLLMProviderTemplates(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := NewAPIDeploymentService(store, nil, nil, nil, nil)
	service := NewLLMDeploymentService(store, nil, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

	t.Run("Empty store returns empty list", func(t *testing.T) {
		templates := service.ListLLMProviderTemplates(nil)
		assert.Empty(t, templates)
	})

	t.Run("Returns all templates with nil filter", func(t *testing.T) {
		template := &models.StoredLLMProviderTemplate{
			ID: "template-1",
			Configuration: api.LLMProviderTemplate{
				Metadata: api.Metadata{Name: "openai"},
				Spec: api.LLMProviderTemplateData{
					DisplayName: "OpenAI Template",
				},
			},
		}
		store.AddTemplate(template)

		templates := service.ListLLMProviderTemplates(nil)
		assert.Len(t, templates, 1)
	})

	t.Run("Returns all templates with empty filter", func(t *testing.T) {
		emptyFilter := ""
		templates := service.ListLLMProviderTemplates(&emptyFilter)
		assert.NotEmpty(t, templates)
	})

	t.Run("Filters by display name", func(t *testing.T) {
		template2 := &models.StoredLLMProviderTemplate{
			ID: "template-2",
			Configuration: api.LLMProviderTemplate{
				Metadata: api.Metadata{Name: "anthropic"},
				Spec: api.LLMProviderTemplateData{
					DisplayName: "Anthropic Template",
				},
			},
		}
		store.AddTemplate(template2)

		filter := "Anthropic Template"
		templates := service.ListLLMProviderTemplates(&filter)
		assert.Len(t, templates, 1)
		assert.Equal(t, "anthropic", templates[0].Configuration.Metadata.Name)
	})
}

func TestLLMDeploymentService_GetLLMProviderTemplateByHandle(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := NewAPIDeploymentService(store, nil, nil, nil, nil)
	service := NewLLMDeploymentService(store, nil, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

	t.Run("Returns error for non-existent template", func(t *testing.T) {
		_, err := service.GetLLMProviderTemplateByHandle("non-existent")
		assert.Error(t, err)
	})

	t.Run("Returns template by handle", func(t *testing.T) {
		template := &models.StoredLLMProviderTemplate{
			ID: "template-1",
			Configuration: api.LLMProviderTemplate{
				Metadata: api.Metadata{Name: "test-template"},
				Spec: api.LLMProviderTemplateData{
					DisplayName: "Test Template",
				},
			},
		}
		store.AddTemplate(template)

		found, err := service.GetLLMProviderTemplateByHandle("test-template")
		assert.NoError(t, err)
		assert.NotNil(t, found)
		assert.Equal(t, "Test Template", found.Configuration.Spec.DisplayName)
	})
}

func TestLLMDeploymentService_CreateLLMProviderTemplate_ParseError(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := NewAPIDeploymentService(store, nil, nil, nil, nil)
	service := NewLLMDeploymentService(store, nil, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	params := LLMTemplateParams{
		Spec:        []byte("invalid yaml: ["),
		ContentType: "application/yaml",
		Logger:      logger,
	}

	_, err := service.CreateLLMProviderTemplate(params)
	assert.Error(t, err)
}

func TestLLMDeploymentService_CreateLLMProviderTemplate_ValidationError(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := NewAPIDeploymentService(store, nil, nil, nil, nil)
	service := NewLLMDeploymentService(store, nil, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Template with empty metadata name
	yamlData := `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: ""
spec:
  displayName: ""
`
	params := LLMTemplateParams{
		Spec:        []byte(yamlData),
		ContentType: "application/yaml",
		Logger:      logger,
	}

	_, err := service.CreateLLMProviderTemplate(params)
	assert.Error(t, err)
}

func TestLLMDeploymentService_UpdateLLMProviderTemplate_NotFound(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := NewAPIDeploymentService(store, nil, nil, nil, nil)
	service := NewLLMDeploymentService(store, nil, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	params := LLMTemplateParams{
		Spec:        []byte("test"),
		ContentType: "application/yaml",
		Logger:      logger,
	}

	_, err := service.UpdateLLMProviderTemplate("non-existent", params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLLMDeploymentService_UpdateLLMProviderTemplate_HandleChange(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := NewAPIDeploymentService(store, nil, nil, nil, nil)
	service := NewLLMDeploymentService(store, nil, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Add existing template
	template := &models.StoredLLMProviderTemplate{
		ID: "template-1",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: "original-handle"},
			Spec: api.LLMProviderTemplateData{
				DisplayName: "Original Template",
			},
		},
	}
	store.AddTemplate(template)

	// Try to update with different handle
	yamlData := `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: different-handle
spec:
  displayName: Updated Template
`
	params := LLMTemplateParams{
		Spec:        []byte(yamlData),
		ContentType: "application/yaml",
		Logger:      logger,
	}

	_, err := service.UpdateLLMProviderTemplate("original-handle", params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot change template handle")
}

func TestLLMDeploymentService_DeleteLLMProviderTemplate_NotFound(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := NewAPIDeploymentService(store, nil, nil, nil, nil)
	service := NewLLMDeploymentService(store, nil, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

	_, err := service.DeleteLLMProviderTemplate("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLLMDeploymentService_DeleteLLMProviderTemplate_Success(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := NewAPIDeploymentService(store, nil, nil, nil, nil)
	service := NewLLMDeploymentService(store, nil, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

	// Add template
	template := &models.StoredLLMProviderTemplate{
		ID: "template-to-delete",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: "delete-me"},
			Spec: api.LLMProviderTemplateData{
				DisplayName: "Delete Me Template",
			},
		},
	}
	store.AddTemplate(template)

	// Delete it
	deleted, err := service.DeleteLLMProviderTemplate("delete-me")
	assert.NoError(t, err)
	assert.NotNil(t, deleted)
	assert.Equal(t, "delete-me", deleted.Configuration.Metadata.Name)

	// Verify it's gone
	_, err = store.GetTemplateByHandle("delete-me")
	assert.Error(t, err)
}

func TestLLMDeploymentService_DeployLLMProviderConfiguration_ParseError(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := NewAPIDeploymentService(store, nil, nil, nil, nil)
	service := NewLLMDeploymentService(store, nil, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	params := LLMDeploymentParams{
		Data:          []byte("invalid yaml: ["),
		ContentType:   "application/yaml",
		CorrelationID: "test-corr",
		Logger:        logger,
	}

	_, err := service.DeployLLMProviderConfiguration(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestLLMDeploymentService_DeployLLMProxyConfiguration_ParseError(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := NewAPIDeploymentService(store, nil, nil, nil, nil)
	service := NewLLMDeploymentService(store, nil, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	params := LLMDeploymentParams{
		Data:          []byte("invalid yaml: ["),
		ContentType:   "application/yaml",
		CorrelationID: "test-corr",
		Logger:        logger,
	}

	_, err := service.DeployLLMProxyConfiguration(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestLLMDeploymentService_UpdateLLMProvider_NotFound(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := NewAPIDeploymentService(store, nil, nil, nil, nil)
	service := NewLLMDeploymentService(store, nil, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	params := LLMDeploymentParams{
		Data:          []byte("test"),
		ContentType:   "application/yaml",
		CorrelationID: "test-corr",
		Logger:        logger,
	}

	_, err := service.UpdateLLMProvider("non-existent", params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLLMDeploymentService_UpdateLLMProxy_NotFound(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := NewAPIDeploymentService(store, nil, nil, nil, nil)
	service := NewLLMDeploymentService(store, nil, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	params := LLMDeploymentParams{
		Data:          []byte("test"),
		ContentType:   "application/yaml",
		CorrelationID: "test-corr",
		Logger:        logger,
	}

	_, err := service.UpdateLLMProxy("non-existent", params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLLMDeploymentService_DeleteLLMProvider_NotFound(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := NewAPIDeploymentService(store, nil, nil, nil, nil)
	service := NewLLMDeploymentService(store, nil, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	_, err := service.DeleteLLMProvider("non-existent", "corr-id", logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLLMDeploymentService_DeleteLLMProxy_NotFound(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := NewAPIDeploymentService(store, nil, nil, nil, nil)
	service := NewLLMDeploymentService(store, nil, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	_, err := service.DeleteLLMProxy("non-existent", "corr-id", logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestMatchesFilters(t *testing.T) {
	t.Run("Invalid config returns false", func(t *testing.T) {
		config := &models.StoredConfig{
			ID:   "test-config",
			Kind: string(api.LlmProvider),
			// No valid spec
		}
		result := matchesFilters(config, api.ListLLMProvidersParams{})
		assert.False(t, result)
	})

	t.Run("Unsupported params type returns false", func(t *testing.T) {
		apiData := api.APIConfigData{
			DisplayName: "Test",
			Version:     "1.0.0",
			Context:     "/test",
		}
		var spec api.APIConfiguration_Spec
		spec.FromAPIConfigData(apiData)

		config := &models.StoredConfig{
			ID:   "test-config",
			Kind: string(api.LlmProvider),
			Configuration: api.APIConfiguration{
				Kind: api.RestApi,
				Spec: spec,
			},
		}
		result := matchesFilters(config, "unsupported type")
		assert.False(t, result)
	})

	t.Run("Matches all filters", func(t *testing.T) {
		vhost := "api.example.com"
		apiData := api.APIConfigData{
			DisplayName: "Test Provider",
			Version:     "1.0.0",
			Context:     "/test",
			Vhosts: &struct {
				Main    string  `json:"main" yaml:"main"`
				Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main: vhost,
			},
		}
		var spec api.APIConfiguration_Spec
		spec.FromAPIConfigData(apiData)

		config := &models.StoredConfig{
			ID:   "test-config",
			Kind: string(api.LlmProvider),
			Configuration: api.APIConfiguration{
				Kind: api.RestApi,
				Spec: spec,
			},
			Status: models.StatusPending,
		}

		displayName := "Test Provider"
		version := "1.0.0"
		context := "/test"
		status := api.ListLLMProvidersParamsStatusPending

		result := matchesFilters(config, api.ListLLMProvidersParams{
			DisplayName: &displayName,
			Version:     &version,
			Context:     &context,
			Status:      &status,
			Vhost:       &vhost,
		})
		assert.True(t, result)
	})

	t.Run("Fails on display name mismatch", func(t *testing.T) {
		apiData := api.APIConfigData{
			DisplayName: "Test Provider",
			Version:     "1.0.0",
			Context:     "/test",
		}
		var spec api.APIConfiguration_Spec
		spec.FromAPIConfigData(apiData)

		config := &models.StoredConfig{
			ID:   "test-config",
			Kind: string(api.LlmProvider),
			Configuration: api.APIConfiguration{
				Kind: api.RestApi,
				Spec: spec,
			},
		}

		wrongName := "Wrong Name"
		result := matchesFilters(config, api.ListLLMProvidersParams{
			DisplayName: &wrongName,
		})
		assert.False(t, result)
	})
}

func TestLLMDeploymentService_InitializeOOBTemplates_Empty(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := NewAPIDeploymentService(store, nil, nil, nil, nil)
	service := NewLLMDeploymentService(store, nil, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

	err := service.InitializeOOBTemplates(nil)
	assert.NoError(t, err)

	err = service.InitializeOOBTemplates(map[string]*api.LLMProviderTemplate{})
	assert.NoError(t, err)
}

func TestLLMDeploymentService_InitializeOOBTemplates_ValidTemplates(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := NewAPIDeploymentService(store, nil, nil, nil, nil)
	service := NewLLMDeploymentService(store, nil, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

	templates := map[string]*api.LLMProviderTemplate{
		"openai": {
			ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LlmProviderTemplate,
			Metadata:   api.Metadata{Name: "openai"},
			Spec: api.LLMProviderTemplateData{
				DisplayName: "OpenAI Template",
			},
		},
	}

	err := service.InitializeOOBTemplates(templates)
	assert.NoError(t, err)

	// Verify template was added
	found, err := store.GetTemplateByHandle("openai")
	assert.NoError(t, err)
	assert.NotNil(t, found)
}

func TestLLMDeploymentService_InitializeOOBTemplates_UpdateExisting(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := NewAPIDeploymentService(store, nil, nil, nil, nil)
	service := NewLLMDeploymentService(store, nil, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

	// Add existing template
	existingTemplate := &models.StoredLLMProviderTemplate{
		ID: "existing-id",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: "existing"},
			Spec: api.LLMProviderTemplateData{
				DisplayName: "Existing Template",
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, store.AddTemplate(existingTemplate))

	// Initialize with updated template
	templates := map[string]*api.LLMProviderTemplate{
		"existing": {
			ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LlmProviderTemplate,
			Metadata:   api.Metadata{Name: "existing"},
			Spec: api.LLMProviderTemplateData{
				DisplayName: "Updated Template",
			},
		},
	}

	err := service.InitializeOOBTemplates(templates)
	assert.NoError(t, err)

	// Verify template was updated
	found, err := store.GetTemplateByHandle("existing")
	assert.NoError(t, err)
	assert.Equal(t, "Updated Template", found.Configuration.Spec.DisplayName)
}
