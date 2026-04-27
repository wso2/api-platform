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
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"gopkg.in/yaml.v3"
)

type llmPublishedEvent struct {
	gatewayID string
	event     eventhub.Event
}

type mockLLMEventHub struct {
	publishedEvents []llmPublishedEvent
}

type failingUpsertDB struct {
	*testMockDB
	err error
}

func (m *mockLLMEventHub) Initialize() error {
	return nil
}

func (m *mockLLMEventHub) RegisterGateway(string) error {
	return nil
}

func (m *mockLLMEventHub) PublishEvent(gatewayID string, event eventhub.Event) error {
	m.publishedEvents = append(m.publishedEvents, llmPublishedEvent{
		gatewayID: gatewayID,
		event:     event,
	})
	return nil
}

func (m *mockLLMEventHub) Subscribe(string) (<-chan eventhub.Event, error) {
	return nil, nil
}

func (m *mockLLMEventHub) Unsubscribe(string, <-chan eventhub.Event) error {
	return nil
}

func (m *mockLLMEventHub) UnsubscribeAll(string) error {
	return nil
}

func (m *mockLLMEventHub) CleanUpEvents() error {
	return nil
}

func (m *mockLLMEventHub) Close() error {
	return nil
}

func (m *failingUpsertDB) UpsertConfig(*models.StoredConfig) (bool, error) {
	return false, m.err
}

func TestLLMDeploymentParams(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	params := LLMDeploymentParams{
		Data:          []byte("test data"),
		ContentType:   "application/yaml",
		ID:            "test-llm-id",
		CorrelationID: "corr-123",
		Logger:        logger,
		Origin:        models.OriginGatewayAPI,
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
		Spec:          []byte("test spec"),
		ContentType:   "application/yaml",
		CorrelationID: "corr-template",
		Logger:        logger,
	}

	assert.Equal(t, "test spec", string(params.Spec))
	assert.Equal(t, "application/yaml", params.ContentType)
	assert.Equal(t, "corr-template", params.CorrelationID)
	assert.NotNil(t, params.Logger)
}

func testLLMTemplateYAML(handle, displayName string) []byte {
	return []byte(fmt.Sprintf(`
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: %s
spec:
  displayName: %s
`, handle, displayName))
}

func testLLMProviderYAML(t *testing.T, handle, displayName, template string) []byte {
	t.Helper()

	cfg := api.LLMProviderConfiguration{
		ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.LLMProviderConfigurationKindLlmProvider,
		Metadata: api.Metadata{
			Name: handle,
		},
		Spec: api.LLMProviderConfigData{
			DisplayName: displayName,
			Version:     "1.0.0",
			Template:    template,
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://example.com"),
			},
			AccessControl: api.LLMAccessControl{
				Mode: api.AllowAll,
			},
		},
	}

	yamlData, err := yaml.Marshal(cfg)
	require.NoError(t, err)

	return yamlData
}

func testLLMProxyYAML(t *testing.T, handle, displayName, providerHandle string) []byte {
	t.Helper()

	cfg := api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.LLMProxyConfigurationKindLlmProxy,
		Metadata: api.Metadata{
			Name: handle,
		},
		Spec: api.LLMProxyConfigData{
			DisplayName: displayName,
			Version:     "1.0.0",
			Context:     stringPtr("/chat"),
			Provider: api.LLMProxyProvider{
				Id: providerHandle,
			},
		},
	}

	yamlData, err := yaml.Marshal(cfg)
	require.NoError(t, err)

	return yamlData
}

func testStoredLLMTemplate(uuid, handle, displayName string) *models.StoredLLMProviderTemplate {
	now := time.Now()
	return &models.StoredLLMProviderTemplate{
		UUID: uuid,
		Configuration: api.LLMProviderTemplate{
			ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LLMProviderTemplateKindLlmProviderTemplate,
			Metadata: api.Metadata{
				Name: handle,
			},
			Spec: api.LLMProviderTemplateData{
				DisplayName: displayName,
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestLLMDeploymentService_ListLLMProviders(t *testing.T) {
	t.Run("Empty store returns empty list", func(t *testing.T) {
		store := storage.NewConfigStore()
		routerConfig := &config.RouterConfig{ListenerPort: 8080}
		apiDeploymentService := newTestAPIDeploymentService(store, newTestMockDB(), nil, nil, nil)
		service := NewLLMDeploymentService(store, newTestMockDB(), nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

		providers := service.ListLLMProviders(api.ListLLMProvidersParams{})
		assert.Empty(t, providers)
	})

	t.Run("Returns only LLM provider kind configs", func(t *testing.T) {
		store := storage.NewConfigStore()
		routerConfig := &config.RouterConfig{ListenerPort: 8080}
		db := newTestMockDB()
		apiDeploymentService := newTestAPIDeploymentService(store, db, nil, nil, nil)
		service := NewLLMDeploymentService(store, db, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

		// Add an LLM provider config
		apiData := api.APIConfigData{
			DisplayName: "LLM Provider",
			Version:     "1.0.0",
			Context:     "/llm",
		}

		llmConfig := &models.StoredConfig{
			UUID:        "0000-llm-provider-1-0000-000000000000",
			Kind:        string(api.LLMProviderConfigurationKindLlmProvider),
			Handle:      "0000-llm-provider-1-0000-000000000000",
			DisplayName: "LLM Provider",
			Version:     "1.0.0",
			Configuration: api.RestAPI{
				Kind:     api.RestAPIKindRestApi,
				Metadata: api.Metadata{Name: "0000-llm-provider-1-0000-000000000000"},
				Spec:     apiData,
			},
			DesiredState: models.StateDeployed,
			Origin:       models.OriginGatewayAPI,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		db.SaveConfig(llmConfig)
		store.Add(llmConfig)

		providers := service.ListLLMProviders(api.ListLLMProvidersParams{})
		assert.Len(t, providers, 1)
		assert.Equal(t, "0000-llm-provider-1-0000-000000000000", providers[0].UUID)
	})

	t.Run("Filter by display name", func(t *testing.T) {
		store := storage.NewConfigStore()
		routerConfig := &config.RouterConfig{ListenerPort: 8080}
		db := newTestMockDB()
		apiDeploymentService := newTestAPIDeploymentService(store, db, nil, nil, nil)
		service := NewLLMDeploymentService(store, db, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

		// Add first provider
		apiData1 := api.APIConfigData{
			DisplayName: "First Provider",
			Version:     "1.0.0",
			Context:     "/first",
		}

		config1 := &models.StoredConfig{
			UUID:        "0000-llm-provider-1-0000-000000000000",
			Kind:        string(api.LLMProviderConfigurationKindLlmProvider),
			Handle:      "first-provider",
			DisplayName: "First Provider",
			Version:     "1.0.0",
			Configuration: api.RestAPI{
				Kind:     api.RestAPIKindRestApi,
				Metadata: api.Metadata{Name: "first-provider"},
				Spec:     apiData1,
			},
			DesiredState: models.StateDeployed,
			Origin:       models.OriginGatewayAPI,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		db.SaveConfig(config1)
		store.Add(config1)

		// Add second provider with different display name
		apiData2 := api.APIConfigData{
			DisplayName: "Filtered Provider",
			Version:     "1.0.0",
			Context:     "/filtered",
		}

		config2 := &models.StoredConfig{
			UUID:        "0000-llm-provider-2-0000-000000000000",
			Kind:        string(api.LLMProviderConfigurationKindLlmProvider),
			Handle:      "filtered-provider",
			DisplayName: "Filtered Provider",
			Version:     "1.0.0",
			Configuration: api.RestAPI{
				Kind:     api.RestAPIKindRestApi,
				Metadata: api.Metadata{Name: "filtered-provider"},
				Spec:     apiData2,
			},
			DesiredState: models.StateDeployed,
			Origin:       models.OriginGatewayAPI,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		db.SaveConfig(config2)
		store.Add(config2)

		displayName := "Filtered Provider"
		providers := service.ListLLMProviders(api.ListLLMProvidersParams{
			DisplayName: &displayName,
		})
		assert.Len(t, providers, 1)
		assert.Equal(t, "0000-llm-provider-2-0000-000000000000", providers[0].UUID)
	})
}

func TestLLMDeploymentService_ListLLMProxies(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	db := newTestMockDB()
	apiDeploymentService := newTestAPIDeploymentService(store, db, nil, nil, nil)
	service := NewLLMDeploymentService(store, db, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

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
		llmProxyConfig := &models.StoredConfig{
			UUID:        "0000-llm-proxy-1-0000-000000000000",
			Kind:        string(api.LLMProxyConfigurationKindLlmProxy),
			Handle:      "0000-llm-proxy-1-0000-000000000000",
			DisplayName: "LLM Proxy",
			Version:     "1.0.0",
			Configuration: api.RestAPI{
				Kind: api.RestAPIKindRestApi,
				Spec: apiData,
			},
			DesiredState: models.StateDeployed,
			Origin:       models.OriginGatewayAPI,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		db.SaveConfig(llmProxyConfig)
		store.Add(llmProxyConfig)

		proxies := service.ListLLMProxies(api.ListLLMProxiesParams{})
		assert.Len(t, proxies, 1)
		assert.Equal(t, "0000-llm-proxy-1-0000-000000000000", proxies[0].UUID)
	})
}

func TestLLMDeploymentService_ListLLMProviderTemplates(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	db := newTestMockDB()
	apiDeploymentService := newTestAPIDeploymentService(store, db, nil, nil, nil)
	service := NewLLMDeploymentService(store, db, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

	t.Run("Empty store returns empty list", func(t *testing.T) {
		templates := service.ListLLMProviderTemplates(nil)
		assert.Empty(t, templates)
	})

	t.Run("Returns all templates with nil filter", func(t *testing.T) {
		template := &models.StoredLLMProviderTemplate{
			UUID: "0000-template-1-0000-000000000000",
			Configuration: api.LLMProviderTemplate{
				Metadata: api.Metadata{Name: "openai"},
				Spec: api.LLMProviderTemplateData{
					DisplayName: "OpenAI Template",
				},
			},
		}
		db.SaveLLMProviderTemplate(template)
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
			UUID: "0000-template-2-0000-000000000000",
			Configuration: api.LLMProviderTemplate{
				Metadata: api.Metadata{Name: "anthropic"},
				Spec: api.LLMProviderTemplateData{
					DisplayName: "Anthropic Template",
				},
			},
		}
		db.SaveLLMProviderTemplate(template2)
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
	db := newTestMockDB()
	apiDeploymentService := newTestAPIDeploymentService(store, db, nil, nil, nil)
	service := NewLLMDeploymentService(store, db, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

	t.Run("Returns error for non-existent template", func(t *testing.T) {
		_, err := service.GetLLMProviderTemplateByHandle("0000-non-existent-0000-000000000000")
		assert.Error(t, err)
	})

	t.Run("Returns template by handle", func(t *testing.T) {
		template := &models.StoredLLMProviderTemplate{
			UUID: "0000-template-1-0000-000000000000",
			Configuration: api.LLMProviderTemplate{
				Metadata: api.Metadata{Name: "test-template"},
				Spec: api.LLMProviderTemplateData{
					DisplayName: "Test Template",
				},
			},
		}
		db.SaveLLMProviderTemplate(template)
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
	apiDeploymentService := newTestAPIDeploymentService(store, newTestMockDB(), nil, nil, nil)
	service := NewLLMDeploymentService(store, newTestMockDB(), nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)
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
	apiDeploymentService := newTestAPIDeploymentService(store, newTestMockDB(), nil, nil, nil)
	service := NewLLMDeploymentService(store, newTestMockDB(), nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)
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
	apiDeploymentService := newTestAPIDeploymentService(store, newTestMockDB(), nil, nil, nil)
	service := NewLLMDeploymentService(store, newTestMockDB(), nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	params := LLMTemplateParams{
		Spec:        []byte("test"),
		ContentType: "application/yaml",
		Logger:      logger,
	}

	_, err := service.UpdateLLMProviderTemplate("0000-non-existent-0000-000000000000", params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLLMDeploymentService_UpdateLLMProviderTemplate_HandleChange(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	db := newTestMockDB()
	apiDeploymentService := newTestAPIDeploymentService(store, db, nil, nil, nil)
	service := NewLLMDeploymentService(store, db, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Add existing template
	template := &models.StoredLLMProviderTemplate{
		UUID: "0000-template-1-0000-000000000000",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: "original-handle"},
			Spec: api.LLMProviderTemplateData{
				DisplayName: "Original Template",
			},
		},
	}
	db.SaveLLMProviderTemplate(template)
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
	apiDeploymentService := newTestAPIDeploymentService(store, newTestMockDB(), nil, nil, nil)
	service := NewLLMDeploymentService(store, newTestMockDB(), nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	_, err := service.DeleteLLMProviderTemplate("0000-non-existent-0000-000000000000", "corr-delete-template", logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLLMDeploymentService_DeleteLLMProviderTemplate_Success(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	db := newTestMockDB()
	apiDeploymentService := newTestAPIDeploymentService(store, db, nil, nil, nil)
	service := NewLLMDeploymentService(store, db, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

	// Add template
	template := &models.StoredLLMProviderTemplate{
		UUID: "0000-template-to-delete-0000-000000000000",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: "delete-me"},
			Spec: api.LLMProviderTemplateData{
				DisplayName: "Delete Me Template",
			},
		},
	}
	db.SaveLLMProviderTemplate(template)

	// Delete it
	deleted, err := service.DeleteLLMProviderTemplate("delete-me", "corr-delete-template", slog.New(slog.NewTextHandler(io.Discard, nil)))
	assert.NoError(t, err)
	assert.NotNil(t, deleted)
	assert.Equal(t, "delete-me", deleted.Configuration.Metadata.Name)

	// Verify the canonical row is gone from the DB.
	_, err = db.GetLLMProviderTemplate(template.UUID)
	assert.Error(t, err)
}

func TestLLMDeploymentService_CreateLLMProviderTemplate_WithDBAndEventHubPublishesCreate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewConfigStore()
	db := newTestSQLiteStorage(t, logger)
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	mockHub := &mockLLMEventHub{}
	apiDeploymentService := newTestAPIDeploymentServiceWithHub(store, db, nil, nil, routerConfig, mockHub, "test-gateway")
	service := NewLLMDeploymentService(store, db, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

	created, err := service.CreateLLMProviderTemplate(LLMTemplateParams{
		Spec:          testLLMTemplateYAML("openai", "OpenAI Template"),
		ContentType:   "application/yaml",
		CorrelationID: "corr-llm-template-create",
		Logger:        logger,
	})
	require.NoError(t, err)
	require.NotNil(t, created)

	require.Len(t, mockHub.publishedEvents, 1)
	assert.Equal(t, "test-gateway", mockHub.publishedEvents[0].gatewayID)
	assert.Equal(t, eventhub.EventTypeLLMTemplate, mockHub.publishedEvents[0].event.EventType)
	assert.Equal(t, "CREATE", mockHub.publishedEvents[0].event.Action)
	assert.Equal(t, created.UUID, mockHub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-llm-template-create", mockHub.publishedEvents[0].event.EventID)

	_, err = db.GetLLMProviderTemplate(created.UUID)
	require.NoError(t, err)

	_, err = store.GetTemplate(created.UUID)
	require.Error(t, err)
}

func TestLLMDeploymentService_CreateLLMProviderTemplate_HandleConflict(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewConfigStore()
	db := newTestMockDB()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := newTestAPIDeploymentService(store, db, nil, nil, nil)
	service := NewLLMDeploymentService(store, db, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

	require.NoError(t, db.SaveLLMProviderTemplate(testStoredLLMTemplate("template-existing-id", "openai", "OpenAI Template")))

	_, err := service.CreateLLMProviderTemplate(LLMTemplateParams{
		Spec:          testLLMTemplateYAML("openai", "OpenAI Template"),
		ContentType:   "application/yaml",
		CorrelationID: "corr-llm-template-create-conflict",
		Logger:        logger,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, storage.ErrConflict)
	assert.Contains(t, err.Error(), "template with handle 'openai' already exists")
}

func TestLLMDeploymentService_UpdateLLMProviderTemplate_WithDBAndEventHubPublishesUpdate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewConfigStore()
	db := newTestSQLiteStorage(t, logger)
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	mockHub := &mockLLMEventHub{}
	apiDeploymentService := newTestAPIDeploymentServiceWithHub(store, db, nil, nil, routerConfig, mockHub, "test-gateway")
	service := NewLLMDeploymentService(store, db, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

	existing := testStoredLLMTemplate("template-update-id", "openai", "OpenAI Template")
	require.NoError(t, db.SaveLLMProviderTemplate(existing))
	require.NoError(t, store.AddTemplate(existing))

	updated, err := service.UpdateLLMProviderTemplate("openai", LLMTemplateParams{
		Spec:          testLLMTemplateYAML("openai", "Updated OpenAI Template"),
		ContentType:   "application/yaml",
		CorrelationID: "corr-llm-template-update",
		Logger:        logger,
	})
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, existing.UUID, updated.UUID)

	require.Len(t, mockHub.publishedEvents, 1)
	assert.Equal(t, "test-gateway", mockHub.publishedEvents[0].gatewayID)
	assert.Equal(t, eventhub.EventTypeLLMTemplate, mockHub.publishedEvents[0].event.EventType)
	assert.Equal(t, "UPDATE", mockHub.publishedEvents[0].event.Action)
	assert.Equal(t, existing.UUID, mockHub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-llm-template-update", mockHub.publishedEvents[0].event.EventID)

	storedInDB, err := db.GetLLMProviderTemplate(existing.UUID)
	require.NoError(t, err)
	assert.Equal(t, "Updated OpenAI Template", storedInDB.Configuration.Spec.DisplayName)

	storedInMemory, err := store.GetTemplate(existing.UUID)
	require.NoError(t, err)
	assert.Equal(t, "OpenAI Template", storedInMemory.Configuration.Spec.DisplayName)
}

func TestLLMDeploymentService_DeleteLLMProviderTemplate_WithDBAndEventHubPublishesDelete(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewConfigStore()
	db := newTestSQLiteStorage(t, logger)
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	mockHub := &mockLLMEventHub{}
	apiDeploymentService := newTestAPIDeploymentServiceWithHub(store, db, nil, nil, routerConfig, mockHub, "test-gateway")
	service := NewLLMDeploymentService(store, db, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

	template := testStoredLLMTemplate("template-delete-id", "openai", "OpenAI Template")
	require.NoError(t, db.SaveLLMProviderTemplate(template))
	require.NoError(t, store.AddTemplate(template))

	deleted, err := service.DeleteLLMProviderTemplate("openai", "corr-llm-template-delete", logger)
	require.NoError(t, err)
	require.NotNil(t, deleted)
	assert.Equal(t, template.UUID, deleted.UUID)

	require.Len(t, mockHub.publishedEvents, 1)
	assert.Equal(t, "test-gateway", mockHub.publishedEvents[0].gatewayID)
	assert.Equal(t, eventhub.EventTypeLLMTemplate, mockHub.publishedEvents[0].event.EventType)
	assert.Equal(t, "DELETE", mockHub.publishedEvents[0].event.Action)
	assert.Equal(t, template.UUID, mockHub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-llm-template-delete", mockHub.publishedEvents[0].event.EventID)

	_, err = db.GetLLMProviderTemplate(template.UUID)
	require.Error(t, err)

	_, err = store.GetTemplate(template.UUID)
	require.NoError(t, err)
}

func TestLLMDeploymentService_DeployLLMProviderConfiguration_ParseError(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := newTestAPIDeploymentService(store, newTestMockDB(), nil, nil, nil)
	service := NewLLMDeploymentService(store, newTestMockDB(), nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	params := LLMDeploymentParams{
		Data:          []byte("invalid yaml: ["),
		ContentType:   "application/yaml",
		CorrelationID: "test-corr",
		Logger:        logger,
		Origin:        models.OriginGatewayAPI,
	}

	_, err := service.DeployLLMProviderConfiguration(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestLLMDeploymentService_DeployLLMProviderConfiguration_SaveErrorReturnsSanitizedError(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	db := &failingUpsertDB{
		testMockDB: newTestMockDB(),
		err: errors.New(
			"failed to upsert artifact: UNIQUE constraint failed: artifacts.gateway_id, artifacts.kind, artifacts.handle",
		),
	}
	template := testStoredLLMTemplate("0000-template-1-0000-000000000000", "openai", "OpenAI Template")
	require.NoError(t, db.SaveLLMProviderTemplate(template))

	mockHub := &mockLLMEventHub{}
	apiDeploymentService := newTestAPIDeploymentServiceWithHub(store, db, nil, nil, routerConfig, mockHub, "test-gateway")
	service := NewLLMDeploymentService(store, db, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

	var logOutput bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logOutput, nil))

	_, err := service.DeployLLMProviderConfiguration(LLMDeploymentParams{
		Data:          testLLMProviderYAML(t, "hotel-booking-provider", "Hotel Booking Provider", "openai"),
		ContentType:   "application/yaml",
		CorrelationID: "test-corr",
		Logger:        logger,
		Origin:        models.OriginGatewayAPI,
	})

	require.Error(t, err)
	assert.Equal(t, "failed to save or update LLM provider configuration", err.Error())
	assert.NotContains(t, err.Error(), "UNIQUE constraint failed")
	assert.Contains(t, logOutput.String(), "Failed to save or update LLM provider configuration")
	assert.Contains(t, logOutput.String(), "UNIQUE constraint failed")
	assert.Empty(t, mockHub.publishedEvents)
}

func TestLLMDeploymentService_DeployLLMProviderConfiguration_ConflictValidation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("rejects duplicate name version from database", func(t *testing.T) {
		store := storage.NewConfigStore()
		db := newTestMockDB()
		routerConfig := &config.RouterConfig{ListenerPort: 8080}
		require.NoError(t, db.SaveLLMProviderTemplate(testStoredLLMTemplate("template-openai-id", "openai", "OpenAI Template")))
		require.NoError(t, db.SaveConfig(&models.StoredConfig{
			UUID:        "provider-existing-id",
			Kind:        string(api.LLMProviderConfigurationKindLlmProvider),
			Handle:      "existing-provider",
			DisplayName: "Hotel Booking Provider",
			Version:     "1.0.0",
		}))

		apiDeploymentService := newTestAPIDeploymentService(store, db, nil, nil, nil)
		service := NewLLMDeploymentService(store, db, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

		_, err := service.DeployLLMProviderConfiguration(LLMDeploymentParams{
			Data:          testLLMProviderYAML(t, "new-provider", "Hotel Booking Provider", "openai"),
			ContentType:   "application/yaml",
			CorrelationID: "test-corr",
			Logger:        logger,
			Origin:        models.OriginGatewayAPI,
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, storage.ErrConflict)
		assert.Contains(t, err.Error(), "name 'Hotel Booking Provider' and version '1.0.0' already exists")
	})

	t.Run("rejects duplicate handle from database", func(t *testing.T) {
		store := storage.NewConfigStore()
		db := newTestMockDB()
		routerConfig := &config.RouterConfig{ListenerPort: 8080}
		require.NoError(t, db.SaveLLMProviderTemplate(testStoredLLMTemplate("template-openai-id", "openai", "OpenAI Template")))
		require.NoError(t, db.SaveConfig(&models.StoredConfig{
			UUID:        "provider-existing-id",
			Kind:        string(api.LLMProviderConfigurationKindLlmProvider),
			Handle:      "hotel-booking-provider",
			DisplayName: "Hotel Booking Provider",
			Version:     "1.0.0",
		}))

		apiDeploymentService := newTestAPIDeploymentService(store, db, nil, nil, nil)
		service := NewLLMDeploymentService(store, db, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

		_, err := service.DeployLLMProviderConfiguration(LLMDeploymentParams{
			Data:          testLLMProviderYAML(t, "hotel-booking-provider", "Another Provider", "openai"),
			ContentType:   "application/yaml",
			CorrelationID: "test-corr",
			Logger:        logger,
			Origin:        models.OriginGatewayAPI,
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, storage.ErrConflict)
		assert.Contains(t, err.Error(), "handle 'hotel-booking-provider' already exists")
	})
}

func TestLLMDeploymentService_DeployLLMProxyConfiguration_ParseError(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := newTestAPIDeploymentService(store, newTestMockDB(), nil, nil, nil)
	service := NewLLMDeploymentService(store, newTestMockDB(), nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	params := LLMDeploymentParams{
		Data:          []byte("invalid yaml: ["),
		ContentType:   "application/yaml",
		CorrelationID: "test-corr",
		Logger:        logger,
		Origin:        models.OriginGatewayAPI,
	}

	_, err := service.DeployLLMProxyConfiguration(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestLLMDeploymentService_DeployLLMProxyConfiguration_ConflictValidation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	baseProvider := &models.StoredConfig{
		UUID:        "provider-a-id",
		Kind:        string(api.LLMProviderConfigurationKindLlmProvider),
		Handle:      "provider-a",
		DisplayName: "Provider A",
		Version:     "1.0.0",
		SourceConfiguration: api.LLMProviderConfiguration{
			ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LLMProviderConfigurationKindLlmProvider,
			Metadata: api.Metadata{
				Name: "provider-a",
			},
			Spec: api.LLMProviderConfigData{
				DisplayName: "Provider A",
				Version:     "1.0.0",
				Template:    "openai",
				Upstream: api.LLMProviderConfigData_Upstream{
					Url: stringPtr("https://example.com"),
				},
				AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
			},
		},
	}

	t.Run("rejects duplicate name version from database", func(t *testing.T) {
		store := storage.NewConfigStore()
		db := newTestMockDB()
		routerConfig := &config.RouterConfig{ListenerPort: 8080}
		require.NoError(t, db.SaveLLMProviderTemplate(testStoredLLMTemplate("template-openai-id", "openai", "OpenAI Template")))
		require.NoError(t, db.SaveConfig(baseProvider))
		require.NoError(t, db.SaveConfig(&models.StoredConfig{
			UUID:        "proxy-existing-id",
			Kind:        string(api.LLMProxyConfigurationKindLlmProxy),
			Handle:      "existing-proxy",
			DisplayName: "Assistant Proxy",
			Version:     "1.0.0",
		}))

		apiDeploymentService := newTestAPIDeploymentService(store, db, nil, nil, nil)
		service := NewLLMDeploymentService(store, db, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

		_, err := service.DeployLLMProxyConfiguration(LLMDeploymentParams{
			Data:          testLLMProxyYAML(t, "new-proxy", "Assistant Proxy", "provider-a"),
			ContentType:   "application/yaml",
			CorrelationID: "test-corr",
			Logger:        logger,
			Origin:        models.OriginGatewayAPI,
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, storage.ErrConflict)
		assert.Contains(t, err.Error(), "name 'Assistant Proxy' and version '1.0.0' already exists")
	})

	t.Run("rejects duplicate handle from database", func(t *testing.T) {
		store := storage.NewConfigStore()
		db := newTestMockDB()
		routerConfig := &config.RouterConfig{ListenerPort: 8080}
		require.NoError(t, db.SaveLLMProviderTemplate(testStoredLLMTemplate("template-openai-id", "openai", "OpenAI Template")))
		require.NoError(t, db.SaveConfig(baseProvider))
		require.NoError(t, db.SaveConfig(&models.StoredConfig{
			UUID:        "proxy-existing-id",
			Kind:        string(api.LLMProxyConfigurationKindLlmProxy),
			Handle:      "assistant-proxy",
			DisplayName: "Assistant Proxy",
			Version:     "1.0.0",
		}))

		apiDeploymentService := newTestAPIDeploymentService(store, db, nil, nil, nil)
		service := NewLLMDeploymentService(store, db, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

		_, err := service.DeployLLMProxyConfiguration(LLMDeploymentParams{
			Data:          testLLMProxyYAML(t, "assistant-proxy", "Another Proxy", "provider-a"),
			ContentType:   "application/yaml",
			CorrelationID: "test-corr",
			Logger:        logger,
			Origin:        models.OriginGatewayAPI,
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, storage.ErrConflict)
		assert.Contains(t, err.Error(), "handle 'assistant-proxy' already exists")
	})
}

func TestLLMDeploymentService_UpdateLLMProvider_NotFound(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := newTestAPIDeploymentService(store, newTestMockDB(), nil, nil, nil)
	service := NewLLMDeploymentService(store, newTestMockDB(), nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	params := LLMDeploymentParams{
		Data:          []byte("test"),
		ContentType:   "application/yaml",
		CorrelationID: "test-corr",
		Logger:        logger,
		Origin:        models.OriginGatewayAPI,
	}

	_, err := service.UpdateLLMProvider("0000-non-existent-0000-000000000000", params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLLMDeploymentService_UpdateLLMProxy_NotFound(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := newTestAPIDeploymentService(store, newTestMockDB(), nil, nil, nil)
	service := NewLLMDeploymentService(store, newTestMockDB(), nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	params := LLMDeploymentParams{
		Data:          []byte("test"),
		ContentType:   "application/yaml",
		CorrelationID: "test-corr",
		Logger:        logger,
		Origin:        models.OriginGatewayAPI,
	}

	_, err := service.UpdateLLMProxy("0000-non-existent-0000-000000000000", params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLLMDeploymentService_DeleteLLMProvider_NotFound(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := newTestAPIDeploymentService(store, newTestMockDB(), nil, nil, nil)
	service := NewLLMDeploymentService(store, newTestMockDB(), nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	_, err := service.DeleteLLMProvider("0000-non-existent-0000-000000000000", "corr-id", logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLLMDeploymentService_DeleteLLMProvider_WithDBAndEventHubPublishesDeleteAndRemovesDBKeys(t *testing.T) {
	metrics.Init()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewConfigStore()
	db := newTestSQLiteStorage(t, logger)
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	mockHub := &mockLLMEventHub{}
	apiDeploymentService := newTestAPIDeploymentServiceWithHub(store, db, nil, nil, routerConfig, mockHub, "test-gateway")
	service := NewLLMDeploymentService(store, db, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

	cfg := &models.StoredConfig{
		UUID:        "llm-provider-delete-id",
		Kind:        string(api.LLMProviderConfigurationKindLlmProvider),
		Handle:      "llm-provider-delete",
		DisplayName: "LLM Provider Delete",
		Version:     "v1.0.0",
		SourceConfiguration: api.LLMProviderConfiguration{
			ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LLMProviderConfigurationKindLlmProvider,
			Metadata: api.Metadata{
				Name: "llm-provider-delete",
			},
			Spec: api.LLMProviderConfigData{
				DisplayName: "LLM Provider Delete",
				Version:     "v1.0.0",
				Template:    "openai",
				Upstream: api.LLMProviderConfigData_Upstream{
					Url: stringPtr("https://example.com"),
				},
				AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
			},
		},
		DesiredState: models.StateDeployed,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	key := newTestStoredAPIKey(cfg.UUID, "provider-key", "user-a", "external")

	require.NoError(t, db.SaveConfig(cfg))
	require.NoError(t, db.SaveAPIKey(key))
	require.NoError(t, store.Add(cfg))

	deleted, err := service.DeleteLLMProvider(cfg.Handle, "corr-llm-provider-delete", logger)
	require.NoError(t, err)
	require.NotNil(t, deleted)
	assert.Equal(t, cfg.UUID, deleted.UUID)

	require.Len(t, mockHub.publishedEvents, 1)
	assert.Equal(t, "test-gateway", mockHub.publishedEvents[0].gatewayID)
	assert.Equal(t, eventhub.EventTypeLLMProvider, mockHub.publishedEvents[0].event.EventType)
	assert.Equal(t, "DELETE", mockHub.publishedEvents[0].event.Action)
	assert.Equal(t, cfg.UUID, mockHub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-llm-provider-delete", mockHub.publishedEvents[0].event.EventID)

	_, err = db.GetConfig(cfg.UUID)
	require.Error(t, err)

	_, err = db.GetAPIKeysByAPIAndName(cfg.UUID, key.Name)
	require.Error(t, err)

	_, err = store.Get(cfg.UUID)
	require.NoError(t, err)
}

func TestLLMDeploymentService_DeleteLLMProxy_NotFound(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := newTestAPIDeploymentService(store, newTestMockDB(), nil, nil, nil)
	service := NewLLMDeploymentService(store, newTestMockDB(), nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	_, err := service.DeleteLLMProxy("0000-non-existent-0000-000000000000", "corr-id", logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLLMDeploymentService_DeleteLLMProxy_WithDBAndEventHubPublishesDeleteAndRemovesDBKeys(t *testing.T) {
	metrics.Init()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewConfigStore()
	db := newTestSQLiteStorage(t, logger)
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	mockHub := &mockLLMEventHub{}
	apiDeploymentService := newTestAPIDeploymentServiceWithHub(store, db, nil, nil, routerConfig, mockHub, "test-gateway")
	service := NewLLMDeploymentService(store, db, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

	providerCfg := &models.StoredConfig{
		UUID:        "provider-1",
		Kind:        string(api.LLMProviderConfigurationKindLlmProvider),
		Handle:      "provider-a",
		DisplayName: "Provider A",
		Version:     "v1.0.0",
		SourceConfiguration: api.LLMProviderConfiguration{
			ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LLMProviderConfigurationKindLlmProvider,
			Metadata: api.Metadata{
				Name: "provider-a",
			},
			Spec: api.LLMProviderConfigData{
				DisplayName: "Provider A",
				Version:     "v1.0.0",
				Template:    "openai",
				Upstream: api.LLMProviderConfigData_Upstream{
					Url: stringPtr("https://example.com"),
				},
				AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
			},
		},
		DesiredState: models.StateDeployed,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	cfg := &models.StoredConfig{
		UUID:        "llm-proxy-delete-id",
		Kind:        string(api.LLMProxyConfigurationKindLlmProxy),
		Handle:      "llm-proxy-delete",
		DisplayName: "LLM Proxy Delete",
		Version:     "v1.0.0",
		SourceConfiguration: api.LLMProxyConfiguration{
			ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LLMProxyConfigurationKindLlmProxy,
			Metadata: api.Metadata{
				Name: "llm-proxy-delete",
			},
			Spec: api.LLMProxyConfigData{
				DisplayName: "LLM Proxy Delete",
				Version:     "v1.0.0",
				Provider: api.LLMProxyProvider{
					Id: "provider-a",
				},
			},
		},
		DesiredState: models.StateDeployed,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	key := newTestStoredAPIKey(cfg.UUID, "proxy-key", "user-a", "external")

	require.NoError(t, db.SaveConfig(providerCfg))
	require.NoError(t, db.SaveConfig(cfg))
	require.NoError(t, db.SaveAPIKey(key))
	require.NoError(t, store.Add(cfg))

	deleted, err := service.DeleteLLMProxy(cfg.Handle, "corr-llm-proxy-delete", logger)
	require.NoError(t, err)
	require.NotNil(t, deleted)
	assert.Equal(t, cfg.UUID, deleted.UUID)

	require.Len(t, mockHub.publishedEvents, 1)
	assert.Equal(t, "test-gateway", mockHub.publishedEvents[0].gatewayID)
	assert.Equal(t, eventhub.EventTypeLLMProxy, mockHub.publishedEvents[0].event.EventType)
	assert.Equal(t, "DELETE", mockHub.publishedEvents[0].event.Action)
	assert.Equal(t, cfg.UUID, mockHub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-llm-proxy-delete", mockHub.publishedEvents[0].event.EventID)

	_, err = db.GetConfig(cfg.UUID)
	require.Error(t, err)

	_, err = db.GetAPIKeysByAPIAndName(cfg.UUID, key.Name)
	require.Error(t, err)

	_, err = store.Get(cfg.UUID)
	require.NoError(t, err)
}

func TestMatchesFilters(t *testing.T) {
	t.Run("Invalid config returns false", func(t *testing.T) {
		config := &models.StoredConfig{
			UUID:        "0000-test-config-0000-000000000000",
			Kind:        string(api.LLMProviderConfigurationKindLlmProvider),
			Handle:      "0000-test-config-0000-000000000000",
			DisplayName: "Test Config",
			Version:     "1.0.0",
			Origin:      models.OriginGatewayAPI,
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

		config := &models.StoredConfig{
			UUID:        "0000-test-config-0000-000000000000",
			Kind:        string(api.LLMProviderConfigurationKindLlmProvider),
			Handle:      "0000-test-config-0000-000000000000",
			DisplayName: "Test",
			Version:     "1.0.0",
			Configuration: api.RestAPI{
				Kind: api.RestAPIKindRestApi,
				Spec: apiData,
			},
			Origin: models.OriginGatewayAPI,
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

		config := &models.StoredConfig{
			UUID:        "0000-test-config-0000-000000000000",
			Kind:        string(api.LLMProviderConfigurationKindLlmProvider),
			Handle:      "0000-test-config-0000-000000000000",
			DisplayName: "Test Provider",
			Version:     "1.0.0",
			Configuration: api.RestAPI{
				Kind: api.RestAPIKindRestApi,
				Spec: apiData,
			},
			DesiredState: models.StateDeployed,
			Origin:       models.OriginGatewayAPI,
		}

		displayName := "Test Provider"
		version := "1.0.0"
		context := "/test"
		status := api.ListLLMProvidersParamsStatusDeployed

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

		config := &models.StoredConfig{
			UUID:        "0000-test-config-0000-000000000000",
			Kind:        string(api.LLMProviderConfigurationKindLlmProvider),
			Handle:      "0000-test-config-0000-000000000000",
			DisplayName: "Test Provider",
			Version:     "1.0.0",
			Configuration: api.RestAPI{
				Kind: api.RestAPIKindRestApi,
				Spec: apiData,
			},
			Origin: models.OriginGatewayAPI,
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
	apiDeploymentService := newTestAPIDeploymentService(store, newTestMockDB(), nil, nil, nil)
	service := NewLLMDeploymentService(store, newTestMockDB(), nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

	err := service.InitializeOOBTemplates(nil)
	assert.NoError(t, err)

	err = service.InitializeOOBTemplates(map[string]*api.LLMProviderTemplate{})
	assert.NoError(t, err)
}

func TestLLMDeploymentService_InitializeOOBTemplates_ValidTemplates(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := newTestAPIDeploymentService(store, newTestMockDB(), nil, nil, nil)
	service := NewLLMDeploymentService(store, newTestMockDB(), nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

	templates := map[string]*api.LLMProviderTemplate{
		"openai": {
			ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LLMProviderTemplateKindLlmProviderTemplate,
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
	apiDeploymentService := newTestAPIDeploymentService(store, newTestMockDB(), nil, nil, nil)
	service := NewLLMDeploymentService(store, newTestMockDB(), nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)

	// Add existing template
	existingTemplate := &models.StoredLLMProviderTemplate{
		UUID: "0000-existing-id-0000-000000000000",
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
			Kind:       api.LLMProviderTemplateKindLlmProviderTemplate,
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
