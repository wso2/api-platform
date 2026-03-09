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
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

func setupSQLiteDBForAPIDeploymentTests(t *testing.T) storage.Storage {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	metrics.Init()
	dbPath := filepath.Join(t.TempDir(), "api-deployment-test.db")
	db, err := storage.NewStorage(storage.BackendConfig{
		Type:       "sqlite",
		SQLitePath: dbPath,
		GatewayID:  "platform-gateway-id",
	}, logger)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}

type mockEventHub struct {
	mu     sync.Mutex
	events []eventhub.Event
}

func (m *mockEventHub) Initialize() error { return nil }

func (m *mockEventHub) RegisterGateway(gatewayID string) error { return nil }

func (m *mockEventHub) PublishEvent(orgID string, event eventhub.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return nil
}

func (m *mockEventHub) Subscribe(orgID string) (<-chan eventhub.Event, error) {
	ch := make(chan eventhub.Event)
	close(ch)
	return ch, nil
}

func (m *mockEventHub) Unsubscribe(orgID string, subscriber <-chan eventhub.Event) error { return nil }

func (m *mockEventHub) UnsubscribeAll(orgID string) error { return nil }

func (m *mockEventHub) CleanUpEvents() error { return nil }

func (m *mockEventHub) Close() error { return nil }

func TestNewAPIDeploymentService(t *testing.T) {
	store := storage.NewConfigStore()
	service := NewAPIDeploymentService(store, nil, nil, nil, nil)

	assert.NotNil(t, service)
	assert.NotNil(t, service.store)
	assert.NotNil(t, service.parser)
	assert.NotNil(t, service.httpClient)
}

func TestAPIDeploymentParams(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	params := APIDeploymentParams{
		Data:          []byte("test data"),
		ContentType:   "application/yaml",
		APIID:         "test-api-id",
		CorrelationID: "corr-123",
		Logger:        logger,
	}

	assert.Equal(t, "test data", string(params.Data))
	assert.Equal(t, "application/yaml", params.ContentType)
	assert.Equal(t, "test-api-id", params.APIID)
	assert.Equal(t, "corr-123", params.CorrelationID)
	assert.NotNil(t, params.Logger)
}

func TestAPIDeploymentResult(t *testing.T) {
	now := time.Now()
	storedCfg := &models.StoredConfig{
		ID:        "test-id",
		Kind:      "RestApi",
		Status:    models.StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	result := &APIDeploymentResult{
		StoredConfig: storedCfg,
		IsUpdate:     true,
	}

	assert.Equal(t, "test-id", result.StoredConfig.ID)
	assert.True(t, result.IsUpdate)
}

func TestGetTopicsForUpdate(t *testing.T) {
	store := storage.NewConfigStore()
	service := NewAPIDeploymentService(store, nil, nil, nil, nil)

	t.Run("Empty config returns empty lists", func(t *testing.T) {
		// Create a config with invalid spec (will fail parsing)
		storedCfg := models.StoredConfig{
			ID:   "test-api-1",
			Kind: string(api.WebSubApi),
		}
		// Set up an empty spec that will fail to parse
		var spec api.APIConfiguration_Spec
		spec.FromAPIConfigData(api.APIConfigData{})
		storedCfg.Configuration = api.APIConfiguration{
			Kind: api.WebSubApi,
			Spec: spec,
		}

		toRegister, toUnregister := service.GetTopicsForUpdate(storedCfg)
		assert.Empty(t, toRegister)
		assert.Empty(t, toUnregister)
	})

	t.Run("Valid WebSub config returns topics", func(t *testing.T) {
		webhookData := api.WebhookAPIData{
			DisplayName: "Test WebSub API",
			Version:     "1.0.0",
			Context:     "/test/$version",
			Channels: []api.Channel{
				{Name: "/events"},
				{Name: "/notifications"},
			},
		}

		var spec api.APIConfiguration_Spec
		err := spec.FromWebhookAPIData(webhookData)
		require.NoError(t, err)

		storedCfg := models.StoredConfig{
			ID:   "websub-api-1",
			Kind: string(api.WebSubApi),
			Configuration: api.APIConfiguration{
				Kind: api.WebSubApi,
				Spec: spec,
			},
		}

		toRegister, toUnregister := service.GetTopicsForUpdate(storedCfg)
		// New topics should be registered
		assert.NotEmpty(t, toRegister)
		assert.Empty(t, toUnregister)
	})
}

func TestGetTopicsForDelete(t *testing.T) {
	store := storage.NewConfigStore()
	service := NewAPIDeploymentService(store, nil, nil, nil, nil)

	t.Run("Returns topics from topic manager", func(t *testing.T) {
		storedCfg := models.StoredConfig{
			ID:   "test-api-1",
			Kind: string(api.WebSubApi),
		}

		// Add some topics to the topic manager
		store.TopicManager.Add(storedCfg.ID, "topic1")
		store.TopicManager.Add(storedCfg.ID, "topic2")

		topics := service.GetTopicsForDelete(storedCfg)
		assert.Len(t, topics, 2)
		assert.Contains(t, topics, "topic1")
		assert.Contains(t, topics, "topic2")
	})

	t.Run("Returns empty for non-existent config", func(t *testing.T) {
		storedCfg := models.StoredConfig{
			ID:   "non-existent-api",
			Kind: string(api.WebSubApi),
		}

		topics := service.GetTopicsForDelete(storedCfg)
		assert.Empty(t, topics)
	})
}

func TestGenerateUUID(t *testing.T) {
	t.Run("Generates valid UUID", func(t *testing.T) {
		uuid, err := GenerateUUID()
		assert.NoError(t, err)
		assert.NotEmpty(t, uuid)
		assert.Len(t, uuid, 36) // Standard UUID length with hyphens
	})

	t.Run("Generates unique UUIDs", func(t *testing.T) {
		uuid1, err := GenerateUUID()
		assert.NoError(t, err)
		uuid2, err := GenerateUUID()
		assert.NoError(t, err)
		assert.NotEqual(t, uuid1, uuid2)
	})
}

func TestSaveOrUpdateConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Save new config to DB", func(t *testing.T) {
		store := storage.NewConfigStore()
		db := setupSQLiteDBForAPIDeploymentTests(t)
		service := NewAPIDeploymentService(store, db, nil, nil, nil)

		apiData := api.APIConfigData{
			DisplayName: "Test API",
			Version:     "1.0.0",
			Context:     "/test",
		}
		var spec api.APIConfiguration_Spec
		spec.FromAPIConfigData(apiData)

		storedCfg := &models.StoredConfig{
			ID:   "new-api-id",
			Kind: string(api.RestApi),
			Configuration: api.APIConfiguration{
				Kind: api.RestApi,
				Spec: spec,
				Metadata: api.Metadata{
					Name: "new-api-id",
				},
			},
			Status:    models.StatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		isUpdate, err := service.saveOrUpdateConfig(storedCfg, logger)
		require.NoError(t, err)
		assert.False(t, isUpdate)

		// Verify config was persisted to DB
		retrieved, err := db.GetConfig(storedCfg.ID)
		require.NoError(t, err)
		assert.Equal(t, storedCfg.ID, retrieved.ID)
	})

	t.Run("Update existing config in DB", func(t *testing.T) {
		store := storage.NewConfigStore()
		db := setupSQLiteDBForAPIDeploymentTests(t)
		service := NewAPIDeploymentService(store, db, nil, nil, nil)

		apiData := api.APIConfigData{
			DisplayName: "Test API",
			Version:     "1.0.0",
			Context:     "/test",
		}
		var spec api.APIConfiguration_Spec
		spec.FromAPIConfigData(apiData)

		// First, add a config
		existingCfg := &models.StoredConfig{
			ID:   "existing-api-id",
			Kind: string(api.RestApi),
			Configuration: api.APIConfiguration{
				Kind: api.RestApi,
				Spec: spec,
				Metadata: api.Metadata{
					Name: "existing-api",
				},
			},
			Status:    models.StatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, db.SaveConfig(existingCfg))

		// Now update it
		newApiData := api.APIConfigData{
			DisplayName: "Updated Test API",
			Version:     "1.0.0",
			Context:     "/test-updated",
		}
		var newSpec api.APIConfiguration_Spec
		newSpec.FromAPIConfigData(newApiData)

		updateCfg := &models.StoredConfig{
			ID:   "existing-api-id",
			Kind: string(api.RestApi),
			Configuration: api.APIConfiguration{
				Kind: api.RestApi,
				Spec: newSpec,
				Metadata: api.Metadata{
					Name: "existing-api",
				},
			},
			Status:    models.StatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		isUpdate, err := service.saveOrUpdateConfig(updateCfg, logger)
		assert.NoError(t, err)
		assert.True(t, isUpdate)

		retrieved, err := db.GetConfig(updateCfg.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Test API", retrieved.GetDisplayName())
	})
}

func TestUpdateExistingConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Updates existing config successfully", func(t *testing.T) {
		store := storage.NewConfigStore()
		db := setupSQLiteDBForAPIDeploymentTests(t)
		service := NewAPIDeploymentService(store, db, nil, nil, nil)

		apiData := api.APIConfigData{
			DisplayName: "Original API",
			Version:     "1.0.0",
			Context:     "/original",
		}
		var spec api.APIConfiguration_Spec
		spec.FromAPIConfigData(apiData)

		// Add original config
		original := &models.StoredConfig{
			ID:   "config-to-update",
			Kind: string(api.RestApi),
			Configuration: api.APIConfiguration{
				Kind: api.RestApi,
				Spec: spec,
				Metadata: api.Metadata{
					Name: "config-to-update",
				},
			},
			Status:    models.StatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, db.SaveConfig(original))
		existingFromDB, err := db.GetConfig(original.ID)
		require.NoError(t, err)

		// Create updated config
		newApiData := api.APIConfigData{
			DisplayName: "Updated API",
			Version:     "2.0.0",
			Context:     "/updated",
		}
		var newSpec api.APIConfiguration_Spec
		newSpec.FromAPIConfigData(newApiData)

		newConfig := &models.StoredConfig{
			ID:   "config-to-update",
			Kind: string(api.RestApi),
			Configuration: api.APIConfiguration{
				Kind: api.RestApi,
				Spec: newSpec,
				Metadata: api.Metadata{
					Name: "config-to-update",
				},
			},
			Status: models.StatusPending,
		}

		isUpdate, err := service.updateExistingConfig(newConfig, existingFromDB, logger)
		assert.NoError(t, err)
		assert.True(t, isUpdate)

		retrieved, err := db.GetConfig(newConfig.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated API", retrieved.GetDisplayName())
	})
}

func TestDeployAPIConfiguration_ParseError(t *testing.T) {
	store := storage.NewConfigStore()
	validator := config.NewAPIValidator()
	service := NewAPIDeploymentService(store, nil, validator, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	params := APIDeploymentParams{
		Data:          []byte("invalid yaml: ["),
		ContentType:   "application/yaml",
		CorrelationID: "test-corr",
		Logger:        logger,
	}

	result, err := service.DeployAPIConfiguration(params)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestDeployAPIConfiguration_ValidationError(t *testing.T) {
	store := storage.NewConfigStore()
	validator := config.NewAPIValidator()
	service := NewAPIDeploymentService(store, nil, validator, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Invalid YAML that will pass parsing but fail validation
	yamlData := `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: test-api
spec:
  displayName: ""
  version: ""
  context: ""
`
	params := APIDeploymentParams{
		Data:          []byte(yamlData),
		ContentType:   "application/yaml",
		CorrelationID: "test-corr",
		Logger:        logger,
	}

	result, err := service.DeployAPIConfiguration(params)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestDeployAPIConfiguration_UsesDBForConflictValidation(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForAPIDeploymentTests(t)
	validator := config.NewAPIValidator()
	service := NewAPIDeploymentService(store, db, validator, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var existingSpec api.APIConfiguration_Spec
	existingSpec.FromAPIConfigData(api.APIConfigData{
		DisplayName: "Existing API",
		Version:     "1.0.0",
		Context:     "/existing",
	})
	existingCfg := &models.StoredConfig{
		ID:   "db-existing-id",
		Kind: string(api.RestApi),
		Configuration: api.APIConfiguration{
			Kind: api.RestApi,
			Metadata: api.Metadata{
				Name: "db-existing-handle",
			},
			Spec: existingSpec,
		},
		Status:    models.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, db.SaveConfig(existingCfg))

	t.Run("name/version conflict from DB", func(t *testing.T) {
		yamlData := `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: another-handle
spec:
  displayName: Existing API
  version: 1.0.0
  context: /another
  upstream:
    main:
      url: https://example.com
  operations:
    - method: GET
      path: /pets
`
		params := APIDeploymentParams{
			Data:          []byte(yamlData),
			ContentType:   "application/yaml",
			CorrelationID: "test-corr",
			Logger:        logger,
		}

		result, err := service.DeployAPIConfiguration(params)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.True(t, storage.IsConflictError(err))
	})

	t.Run("handle conflict from DB", func(t *testing.T) {
		yamlData := `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: db-existing-handle
spec:
  displayName: Brand New API
  version: 2.0.0
  context: /brand-new
  upstream:
    main:
      url: https://example.com
  operations:
    - method: GET
      path: /orders
`
		params := APIDeploymentParams{
			Data:          []byte(yamlData),
			ContentType:   "application/yaml",
			CorrelationID: "test-corr",
			Logger:        logger,
		}

		result, err := service.DeployAPIConfiguration(params)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.True(t, storage.IsConflictError(err))
	})
}

func TestAPIDeploymentService_Fields(t *testing.T) {
	store := storage.NewConfigStore()
	service := NewAPIDeploymentService(store, nil, nil, nil, nil)

	assert.Equal(t, store, service.store)
	assert.Nil(t, service.db)
	assert.NotNil(t, service.parser)
	assert.NotNil(t, service.httpClient)
}

func TestDeleteAPIConfiguration_NoDatabase(t *testing.T) {
	store := storage.NewConfigStore()
	service := NewAPIDeploymentService(store, nil, nil, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	result, err := service.DeleteAPIConfiguration(APIDeletionParams{
		Handle:        "test-handle",
		CorrelationID: "corr-1",
		Logger:        logger,
	})

	assert.Nil(t, result)
	require.Error(t, err)
	assert.True(t, storage.IsDatabaseUnavailableError(err))
}

func TestDeleteAPIConfiguration_PublishesEventWithoutMutatingMemoryStore(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForAPIDeploymentTests(t)
	hub := &mockEventHub{}
	service := NewAPIDeploymentService(store, db, nil, nil, hub, constants.PlatformGatewayId)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var spec api.APIConfiguration_Spec
	spec.FromAPIConfigData(api.APIConfigData{
		DisplayName: "Delete Test API",
		Version:     "v1",
		Context:     "/delete-test",
	})

	cfg := &models.StoredConfig{
		ID: "delete-test-id",
		Configuration: api.APIConfiguration{
			Kind: api.RestApi,
			Metadata: api.Metadata{
				Name: "delete-test-handle",
			},
			Spec: spec,
		},
		SourceConfiguration: api.APIConfiguration{
			Kind: api.RestApi,
			Metadata: api.Metadata{
				Name: "delete-test-handle",
			},
			Spec: spec,
		},
		Status:          models.StatusPending,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		DeployedAt:      nil,
		DeployedVersion: 0,
	}

	require.NoError(t, db.SaveConfig(cfg))
	require.NoError(t, store.Add(cfg))

	result, err := service.DeleteAPIConfiguration(APIDeletionParams{
		Handle:        "delete-test-handle",
		CorrelationID: "corr-delete",
		Logger:        logger,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, cfg.ID, result.StoredConfig.ID)

	// DB must be deleted in the write path.
	_, err = db.GetConfig(cfg.ID)
	require.Error(t, err)
	assert.True(t, storage.IsNotFoundError(err))

	// In-memory ConfigStore must remain unchanged until event listener sync.
	_, err = store.Get(cfg.ID)
	require.NoError(t, err)

	require.Len(t, hub.events, 1)
	assert.Equal(t, eventhub.EventTypeAPI, hub.events[0].EventType)
	assert.Equal(t, "DELETE", hub.events[0].Action)
	assert.Equal(t, cfg.ID, hub.events[0].EntityID)
	assert.Equal(t, "corr-delete", hub.events[0].EventID)
	assert.Equal(t, eventhub.EmptyEventData, hub.events[0].EventData)
}

func TestDeleteAPIConfiguration_ByAPIIDPublishesEventWithoutMutatingMemoryStore(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForAPIDeploymentTests(t)
	hub := &mockEventHub{}
	service := NewAPIDeploymentService(store, db, nil, nil, hub, constants.PlatformGatewayId)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var spec api.APIConfiguration_Spec
	spec.FromAPIConfigData(api.APIConfigData{
		DisplayName: "Delete By ID Test API",
		Version:     "v1",
		Context:     "/delete-by-id-test",
	})

	cfg := &models.StoredConfig{
		ID: "delete-by-id-test-id",
		Configuration: api.APIConfiguration{
			Kind: api.RestApi,
			Metadata: api.Metadata{
				Name: "delete-by-id-test-handle",
			},
			Spec: spec,
		},
		SourceConfiguration: api.APIConfiguration{
			Kind: api.RestApi,
			Metadata: api.Metadata{
				Name: "delete-by-id-test-handle",
			},
			Spec: spec,
		},
		Status:          models.StatusPending,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		DeployedAt:      nil,
		DeployedVersion: 0,
	}

	require.NoError(t, db.SaveConfig(cfg))
	require.NoError(t, store.Add(cfg))

	result, err := service.DeleteAPIConfiguration(APIDeletionParams{
		APIID:         cfg.ID,
		CorrelationID: "corr-delete-by-id",
		Logger:        logger,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, cfg.ID, result.StoredConfig.ID)

	_, err = db.GetConfig(cfg.ID)
	require.Error(t, err)
	assert.True(t, storage.IsNotFoundError(err))

	_, err = store.Get(cfg.ID)
	require.NoError(t, err)

	require.Len(t, hub.events, 1)
	assert.Equal(t, eventhub.EventTypeAPI, hub.events[0].EventType)
	assert.Equal(t, "DELETE", hub.events[0].Action)
	assert.Equal(t, cfg.ID, hub.events[0].EntityID)
	assert.Equal(t, "corr-delete-by-id", hub.events[0].EventID)
	assert.Equal(t, eventhub.EmptyEventData, hub.events[0].EventData)
}

func TestUndeployAPIConfiguration_NoDatabase(t *testing.T) {
	store := storage.NewConfigStore()
	service := NewAPIDeploymentService(store, nil, nil, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	result, err := service.UndeployAPIConfiguration(APIUndeploymentParams{
		APIID:         "test-id",
		CorrelationID: "corr-undeploy",
		Logger:        logger,
	})

	assert.Nil(t, result)
	require.Error(t, err)
	assert.True(t, storage.IsDatabaseUnavailableError(err))
}

func TestUndeployAPIConfiguration_NotFound(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForAPIDeploymentTests(t)
	hub := &mockEventHub{}
	service := NewAPIDeploymentService(store, db, nil, nil, hub, constants.PlatformGatewayId)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	result, err := service.UndeployAPIConfiguration(APIUndeploymentParams{
		APIID:         "missing-id",
		CorrelationID: "corr-undeploy-missing",
		Logger:        logger,
	})

	assert.Nil(t, result)
	require.Error(t, err)
	assert.True(t, storage.IsNotFoundError(err))
	assert.Empty(t, hub.events)
}

func TestUndeployAPIConfiguration_PublishesEventWithoutMutatingMemoryStore(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForAPIDeploymentTests(t)
	hub := &mockEventHub{}
	service := NewAPIDeploymentService(store, db, nil, nil, hub, constants.PlatformGatewayId)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var spec api.APIConfiguration_Spec
	spec.FromAPIConfigData(api.APIConfigData{
		DisplayName: "Undeploy Test API",
		Version:     "v1",
		Context:     "/undeploy-test",
	})

	cfg := &models.StoredConfig{
		ID: "undeploy-test-id",
		Configuration: api.APIConfiguration{
			Kind: api.RestApi,
			Metadata: api.Metadata{
				Name: "undeploy-test-handle",
			},
			Spec: spec,
		},
		SourceConfiguration: api.APIConfiguration{
			Kind: api.RestApi,
			Metadata: api.Metadata{
				Name: "undeploy-test-handle",
			},
			Spec: spec,
		},
		Status:          models.StatusDeployed,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		DeployedAt:      nil,
		DeployedVersion: 7,
	}

	require.NoError(t, db.SaveConfig(cfg))
	require.NoError(t, store.Add(cfg))

	result, err := service.UndeployAPIConfiguration(APIUndeploymentParams{
		APIID:         cfg.ID,
		CorrelationID: "corr-undeploy",
		Logger:        logger,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, cfg.ID, result.StoredConfig.ID)
	assert.Equal(t, models.StatusUndeployed, result.StoredConfig.Status)
	assert.EqualValues(t, 7, result.StoredConfig.DeployedVersion)

	dbConfig, err := db.GetConfig(cfg.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusUndeployed, dbConfig.Status)
	assert.EqualValues(t, 7, dbConfig.DeployedVersion)

	// In-memory ConfigStore must remain unchanged until event listener sync.
	inMemoryConfig, err := store.Get(cfg.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusDeployed, inMemoryConfig.Status)

	require.Len(t, hub.events, 1)
	assert.Equal(t, eventhub.EventTypeAPI, hub.events[0].EventType)
	assert.Equal(t, "UPDATE", hub.events[0].Action)
	assert.Equal(t, cfg.ID, hub.events[0].EntityID)
	assert.Equal(t, "corr-undeploy", hub.events[0].EventID)
	assert.Equal(t, eventhub.EmptyEventData, hub.events[0].EventData)
}

// Tests for lines 100-111: WebSub API parsing error path
func TestDeployAPIConfiguration_WebSubParseError(t *testing.T) {
	store := storage.NewConfigStore()
	validator := config.NewAPIValidator()
	service := NewAPIDeploymentService(store, nil, validator, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Create a WebSub API with invalid spec structure
	yamlData := `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: WebSubApi
metadata:
  name: test-websub
spec:
  displayName: Test WebSub
  version: 1.0.0
  context: /test
  channels: invalid
`
	params := APIDeploymentParams{
		Data:          []byte(yamlData),
		ContentType:   "application/yaml",
		CorrelationID: "test-corr",
		Logger:        logger,
	}

	result, err := service.DeployAPIConfiguration(params)
	assert.Error(t, err)
	assert.Nil(t, result)
}

// Tests for lines 166-268: WebSub topic registration/unregistration paths
func TestDeployAPIConfiguration_WebSubTopicOperations(t *testing.T) {
	// Helper to create a failing WebSub hub server
	failingHub := func(t *testing.T) (string, int, func()) {
		t.Helper()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "hub error", http.StatusInternalServerError)
		}))
		u, _ := url.Parse(srv.URL)
		p, _ := strconv.Atoi(u.Port())
		return u.Hostname(), p, srv.Close
	}

	t.Run("Topic registration error path", func(t *testing.T) {
		host, port, closeFn := failingHub(t)
		t.Cleanup(closeFn)

		store := storage.NewConfigStore()
		validator := config.NewAPIValidator()
		routerConfig := &config.RouterConfig{
			EventGateway: config.EventGatewayConfig{
				RouterHost:            host,
				WebSubHubListenerPort: port,
				TimeoutSeconds:        1,
			},
		}
		service := NewAPIDeploymentService(store, nil, validator, routerConfig, nil)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))

		// Create a valid WebSub API
		yamlData := `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: WebSubApi
metadata:
  name: test-websub
spec:
  displayName: Test WebSub API
  version: 1.0.0
  context: /test/$version
  channels:
    - name: /events
    - name: /notifications
`
		params := APIDeploymentParams{
			Data:          []byte(yamlData),
			ContentType:   "application/yaml",
			CorrelationID: "test-corr",
			Logger:        logger,
		}

		// This will fail because the hub returns an error
		result, err := service.DeployAPIConfiguration(params)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to complete topic operations")
	})

	t.Run("Topic deregistration during update", func(t *testing.T) {
		host, port, closeFn := failingHub(t)
		t.Cleanup(closeFn)

		store := storage.NewConfigStore()
		validator := config.NewAPIValidator()
		routerConfig := &config.RouterConfig{
			EventGateway: config.EventGatewayConfig{
				RouterHost:            host,
				WebSubHubListenerPort: port,
				TimeoutSeconds:        1,
			},
		}
		service := NewAPIDeploymentService(store, nil, validator, routerConfig, nil)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))

		// Add existing WebSub API with topics
		webhookData := api.WebhookAPIData{
			DisplayName: "Existing WebSub",
			Version:     "1.0.0",
			Context:     "/existing/$version",
			Channels: []api.Channel{
				{Name: "/old-topic"},
			},
		}
		var spec api.APIConfiguration_Spec
		require.NoError(t, spec.FromWebhookAPIData(webhookData))

		existingCfg := &models.StoredConfig{
			ID:   "existing-websub",
			Kind: string(api.WebSubApi),
			Configuration: api.APIConfiguration{
				Kind: api.WebSubApi,
				Spec: spec,
			},
			Status:    models.StatusDeployed,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		store.Add(existingCfg)
		store.TopicManager.Add(existingCfg.ID, "/existing/1.0.0/old-topic")

		// Update with new topics (will try to deregister old one)
		yamlData := `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: WebSubApi
metadata:
  name: existing-websub
spec:
  displayName: Updated WebSub
  version: 1.0.0
  context: /existing/$version
  channels:
    - name: /new-topic
`
		params := APIDeploymentParams{
			Data:          []byte(yamlData),
			APIID:         "existing-websub",
			ContentType:   "application/yaml",
			CorrelationID: "test-corr",
			Logger:        logger,
		}

		// Will fail because the hub returns an error
		result, err := service.DeployAPIConfiguration(params)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestSaveOrUpdateConfig_DoesNotMutateMemoryStoreInWritePath(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForAPIDeploymentTests(t)
	service := NewAPIDeploymentService(store, db, nil, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	apiData := api.APIConfigData{
		DisplayName: "Original API",
		Version:     "1.0.0",
		Context:     "/original",
	}
	var spec api.APIConfiguration_Spec
	spec.FromAPIConfigData(apiData)

	original := &models.StoredConfig{
		ID:   "memory-db-sync-test",
		Kind: string(api.RestApi),
		Configuration: api.APIConfiguration{
			Kind: api.RestApi,
			Metadata: api.Metadata{
				Name: "memory-db-sync-test",
			},
			Spec: spec,
		},
		Status:    models.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	require.NoError(t, db.SaveConfig(original))
	require.NoError(t, store.Add(original))

	updatedData := api.APIConfigData{
		DisplayName: "Updated API",
		Version:     "1.0.0",
		Context:     "/updated",
	}
	var updatedSpec api.APIConfiguration_Spec
	updatedSpec.FromAPIConfigData(updatedData)

	updateCfg := &models.StoredConfig{
		ID:   "memory-db-sync-test",
		Kind: string(api.RestApi),
		Configuration: api.APIConfiguration{
			Kind: api.RestApi,
			Metadata: api.Metadata{
				Name: "memory-db-sync-test",
			},
			Spec: updatedSpec,
		},
		Status: models.StatusPending,
	}

	isUpdate, err := service.saveOrUpdateConfig(updateCfg, logger)
	require.NoError(t, err)
	assert.True(t, isUpdate)

	dbConfig, err := db.GetConfig(updateCfg.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated API", dbConfig.GetDisplayName())

	inMemoryConfig, err := store.Get(updateCfg.ID)
	require.NoError(t, err)
	assert.Equal(t, "Original API", inMemoryConfig.GetDisplayName())
}

func TestUpdateExistingConfig_RequiresDatabase(t *testing.T) {
	store := storage.NewConfigStore()
	service := NewAPIDeploymentService(store, nil, nil, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	apiData := api.APIConfigData{
		DisplayName: "Original API",
		Version:     "1.0.0",
		Context:     "/original",
	}
	var spec api.APIConfiguration_Spec
	spec.FromAPIConfigData(apiData)

	original := &models.StoredConfig{
		ID:   "test-api",
		Kind: string(api.RestApi),
		Configuration: api.APIConfiguration{
			Kind: api.RestApi,
			Metadata: api.Metadata{
				Name: "test-api",
			},
			Spec: spec,
		},
		Status:    models.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	newConfig := &models.StoredConfig{
		ID:            "test-api",
		Kind:          string(api.RestApi),
		Configuration: original.Configuration,
		Status:        models.StatusPending,
	}

	isUpdate, err := service.updateExistingConfig(newConfig, original, logger)
	assert.Error(t, err)
	assert.False(t, isUpdate)
	assert.True(t, storage.IsDatabaseUnavailableError(err))
}

func TestSendTopicRequestToHub_RetryLogic(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{
		EventGateway: config.EventGatewayConfig{
			RouterHost:            "localhost",
			WebSubHubListenerPort: 8084,
			TimeoutSeconds:        1,
		},
	}
	service := NewAPIDeploymentService(store, nil, nil, routerConfig, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Context timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		time.Sleep(10 * time.Millisecond) // Ensure context is expired

		err := service.RegisterTopicWithHub(ctx, service.httpClient, "/test-topic", "localhost", 8084, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "canceled")
	})

	t.Run("Connection refused", func(t *testing.T) {
		ctx := context.Background()

		// Try to connect to a port that's not listening
		err := service.RegisterTopicWithHub(ctx, service.httpClient, "/test-topic", "localhost", 19999, logger)
		assert.Error(t, err)
	})

	t.Run("Unregister topic", func(t *testing.T) {
		ctx := context.Background()

		err := service.UnregisterTopicWithHub(ctx, service.httpClient, "/test-topic", "localhost", 19999, logger)
		assert.Error(t, err)
	})
}

func TestRegisterAndUnregisterTopicWithHub(t *testing.T) {
	store := storage.NewConfigStore()
	routerConfig := &config.RouterConfig{
		EventGateway: config.EventGatewayConfig{
			RouterHost:            "localhost",
			WebSubHubListenerPort: 8084,
			TimeoutSeconds:        1,
		},
	}
	service := NewAPIDeploymentService(store, nil, nil, routerConfig, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("RegisterTopicWithHub calls sendTopicRequestToHub", func(t *testing.T) {
		ctx := context.Background()
		err := service.RegisterTopicWithHub(ctx, service.httpClient, "/test", "localhost", 9999, logger)
		assert.Error(t, err) // Will fail because no server is running
	})

	t.Run("UnregisterTopicWithHub calls sendTopicRequestToHub", func(t *testing.T) {
		ctx := context.Background()
		err := service.UnregisterTopicWithHub(ctx, service.httpClient, "/test", "localhost", 9999, logger)
		assert.Error(t, err) // Will fail because no server is running
	})
}
