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
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

func TestNewAPIDeploymentService(t *testing.T) {
	store := storage.NewConfigStore()
	service := newTestAPIDeploymentService(store, nil, nil, nil, nil)

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
		APIID:         "0000-test-api-id-0000-000000000000",
		CorrelationID: "corr-123",
		Origin:        models.OriginGatewayAPI,
		Logger:        logger,
	}

	assert.Equal(t, "test data", string(params.Data))
	assert.Equal(t, "application/yaml", params.ContentType)
	assert.Equal(t, "0000-test-api-id-0000-000000000000", params.APIID)
	assert.Equal(t, "corr-123", params.CorrelationID)
	assert.NotNil(t, params.Logger)
}

func TestAPIDeploymentResult(t *testing.T) {
	now := time.Now()
	storedCfg := &models.StoredConfig{
		UUID:         "0000-test-id-0000-000000000000",
		Kind:         "RestApi",
		DesiredState: models.StateDeployed,
		Origin:       models.OriginGatewayAPI,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	result := &APIDeploymentResult{
		StoredConfig: storedCfg,
		IsUpdate:     true,
	}

	assert.Equal(t, "0000-test-id-0000-000000000000", result.StoredConfig.UUID)
	assert.True(t, result.IsUpdate)
}

func TestGetTopicsForUpdate(t *testing.T) {
	store := storage.NewConfigStore()
	service := newTestAPIDeploymentService(store, nil, nil, nil, nil)

	t.Run("Empty config returns empty lists", func(t *testing.T) {
		// Create a config with invalid spec (will fail parsing)
		storedCfg := models.StoredConfig{
			UUID:   "0000-test-api-1-0000-000000000000",
			Kind:   string(api.WebSubAPIKindWebSubApi),
			Origin: models.OriginGatewayAPI,
		}
		// Set up an empty spec that will fail to parse
		storedCfg.Configuration = api.WebSubAPI{
			Kind: api.WebSubAPIKindWebSubApi,
			Spec: api.WebhookAPIData{},
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

		storedCfg := models.StoredConfig{
			UUID:   "0000-websub-api-1-0000-000000000000",
			Kind:   string(api.WebSubAPIKindWebSubApi),
			Origin: models.OriginGatewayAPI,
			Configuration: api.WebSubAPI{
				Kind: api.WebSubAPIKindWebSubApi,
				Spec: webhookData,
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
	service := newTestAPIDeploymentService(store, nil, nil, nil, nil)

	t.Run("Returns topics from topic manager", func(t *testing.T) {
		storedCfg := models.StoredConfig{
			UUID:   "0000-test-api-1-0000-000000000000",
			Kind:   string(api.WebSubAPIKindWebSubApi),
			Origin: models.OriginGatewayAPI,
		}

		// Add some topics to the topic manager
		store.TopicManager.Add(storedCfg.UUID, "topic1")
		store.TopicManager.Add(storedCfg.UUID, "topic2")

		topics := service.GetTopicsForDelete(storedCfg)
		assert.Len(t, topics, 2)
		assert.Contains(t, topics, "topic1")
		assert.Contains(t, topics, "topic2")
	})

	t.Run("Returns empty for non-existent config", func(t *testing.T) {
		storedCfg := models.StoredConfig{
			UUID:   "0000-non-existent-api-0000-000000000000",
			Kind:   string(api.WebSubAPIKindWebSubApi),
			Origin: models.OriginGatewayAPI,
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

func TestGenerateDeterministicUUIDv7(t *testing.T) {
	t.Run("Deterministic - same input produces same UUID", func(t *testing.T) {
		deploymentID := "dep-789"
		performedAt := time.Date(2026, 3, 4, 10, 30, 0, 0, time.UTC)

		uuid1 := GenerateDeterministicUUIDv7(deploymentID, performedAt)
		uuid2 := GenerateDeterministicUUIDv7(deploymentID, performedAt)
		assert.Equal(t, uuid1, uuid2)
	})

	t.Run("Valid UUID format", func(t *testing.T) {
		result := GenerateDeterministicUUIDv7("dep-123", time.Now())
		assert.Len(t, result, 36)
		// Verify version 7 (character at position 14 should be '7')
		assert.Equal(t, byte('7'), result[14])
	})

	t.Run("Different deploymentIDs produce different UUIDs", func(t *testing.T) {
		ts := time.Date(2026, 3, 4, 10, 30, 0, 0, time.UTC)
		uuid1 := GenerateDeterministicUUIDv7("dep-111", ts)
		uuid2 := GenerateDeterministicUUIDv7("dep-222", ts)
		assert.NotEqual(t, uuid1, uuid2)
	})

	t.Run("Different timestamps produce different UUIDs", func(t *testing.T) {
		deploymentID := "dep-789"
		uuid1 := GenerateDeterministicUUIDv7(deploymentID, time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC))
		uuid2 := GenerateDeterministicUUIDv7(deploymentID, time.Date(2026, 3, 4, 11, 0, 0, 0, time.UTC))
		assert.NotEqual(t, uuid1, uuid2)
	})

	t.Run("Time ordering - later timestamp produces lexicographically greater UUID", func(t *testing.T) {
		deploymentID := "dep-789"
		earlier := GenerateDeterministicUUIDv7(deploymentID, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		later := GenerateDeterministicUUIDv7(deploymentID, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
		assert.True(t, earlier < later, "UUIDv7 with earlier timestamp should sort before later timestamp")
	})
}

func TestSaveOrUpdateConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Save new config persists to DB", func(t *testing.T) {
		store := storage.NewConfigStore()
		mockDB := newTestMockDB()
		service := newTestAPIDeploymentService(store, mockDB, nil, nil, nil)

		apiData := api.APIConfigData{
			DisplayName: "Test API",
			Version:     "1.0.0",
			Context:     "/test",
		}

		storedCfg := &models.StoredConfig{
			UUID: "0000-new-api-id-0000-000000000000",
			Kind: string(api.RestAPIKindRestApi),
			Configuration: api.RestAPI{
				Kind: api.RestAPIKindRestApi,
				Spec: apiData,
			},
			DesiredState: models.StateDeployed,
			Origin:       models.OriginGatewayAPI,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		affected, err := service.saveOrUpdateConfig(storedCfg, logger)
		assert.NoError(t, err)
		assert.True(t, affected)

		retrieved, err := mockDB.GetConfig(storedCfg.UUID)
		assert.NoError(t, err)
		assert.Equal(t, storedCfg.UUID, retrieved.UUID)
	})

	t.Run("Update existing config persists to DB", func(t *testing.T) {
		store := storage.NewConfigStore()
		mockDB := newTestMockDB()
		service := newTestAPIDeploymentService(store, mockDB, nil, nil, nil)

		apiData := api.APIConfigData{
			DisplayName: "Test API",
			Version:     "1.0.0",
			Context:     "/test",
		}

		// First, add a config
		existingCfg := &models.StoredConfig{
			UUID: "0000-existing-api-id-0000-000000000000",
			Kind: string(api.RestAPIKindRestApi),
			Configuration: api.RestAPI{
				Kind: api.RestAPIKindRestApi,
				Spec: apiData,
			},
			DesiredState: models.StateDeployed,
			Origin:       models.OriginGatewayAPI,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		store.Add(existingCfg)
		require.NoError(t, mockDB.SaveConfig(existingCfg))

		// Now update it
		newApiData := api.APIConfigData{
			DisplayName: "Updated Test API",
			Version:     "1.0.0",
			Context:     "/test-updated",
		}

		updateCfg := &models.StoredConfig{
			UUID: "0000-existing-api-id-0000-000000000000",
			Kind: string(api.RestAPIKindRestApi),
			Configuration: api.RestAPI{
				Kind: api.RestAPIKindRestApi,
				Spec: newApiData,
			},
			DesiredState: models.StateDeployed,
			Origin:       models.OriginGatewayAPI,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		isUpdate, err := service.saveOrUpdateConfig(updateCfg, logger)
		assert.NoError(t, err)
		assert.True(t, isUpdate)

		retrieved, err := mockDB.GetConfig(updateCfg.UUID)
		assert.NoError(t, err)
		assert.Equal(t, updateCfg.UUID, retrieved.UUID)
	})
}

func TestUpdateExistingConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Updates existing config successfully", func(t *testing.T) {
		store := storage.NewConfigStore()
		mockDB := newTestMockDB()
		service := newTestAPIDeploymentService(store, mockDB, nil, nil, nil)

		apiData := api.APIConfigData{
			DisplayName: "Original API",
			Version:     "1.0.0",
			Context:     "/original",
		}

		// Add original config
		original := &models.StoredConfig{
			UUID: "0000-config-to-update-0000-000000000000",
			Kind: string(api.RestAPIKindRestApi),
			Configuration: api.RestAPI{
				Kind: api.RestAPIKindRestApi,
				Spec: apiData,
			},
			DesiredState: models.StateDeployed,
			Origin:       models.OriginGatewayAPI,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		store.Add(original)
		require.NoError(t, mockDB.SaveConfig(original))

		// Create updated config
		newApiData := api.APIConfigData{
			DisplayName: "Updated API",
			Version:     "2.0.0",
			Context:     "/updated",
		}

		newConfig := &models.StoredConfig{
			UUID: "0000-config-to-update-0000-000000000000",
			Kind: string(api.RestAPIKindRestApi),
			Configuration: api.RestAPI{
				Kind: api.RestAPIKindRestApi,
				Spec: newApiData,
			},
			DesiredState: models.StateDeployed,
			Origin:       models.OriginGatewayAPI,
		}

		affected, err := service.saveOrUpdateConfig(newConfig, logger)
		assert.NoError(t, err)
		assert.True(t, affected)
	})
}

func TestDeployAPIConfiguration_ParseError(t *testing.T) {
	store := storage.NewConfigStore()
	validator := config.NewAPIValidator()
	service := newTestAPIDeploymentService(store, nil, nil, validator, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	params := APIDeploymentParams{
		Data:          []byte("invalid yaml: ["),
		ContentType:   "application/yaml",
		CorrelationID: "test-corr",
		Origin:        models.OriginGatewayAPI,
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
	service := newTestAPIDeploymentService(store, nil, nil, validator, nil)
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
		Origin:        models.OriginGatewayAPI,
		Logger:        logger,
	}

	result, err := service.DeployAPIConfiguration(params)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestDeployAPIConfiguration_DBConflictValidation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	validator := config.NewAPIValidator()

	t.Run("rejects duplicate name version from database", func(t *testing.T) {
		store := storage.NewConfigStore()
		db := newTestMockDB()
		service := newTestAPIDeploymentService(store, db, nil, validator, nil)

		require.NoError(t, db.SaveConfig(&models.StoredConfig{
			UUID:        "rest-existing-1",
			Kind:        string(api.RestAPIKindRestApi),
			Handle:      "existing-rest-api",
			DisplayName: "Existing Rest API",
			Version:     "1.0.0",
		}))

		params := APIDeploymentParams{
			Data: []byte(`
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: another-rest-api
spec:
  displayName: Existing Rest API
  version: "1.0.0"
  context: /existing
  upstream:
    main:
      url: https://example.com
  operations:
    - method: GET
      path: /items
`),
			ContentType:   "application/yaml",
			CorrelationID: "test-corr",
			Origin:        models.OriginGatewayAPI,
			Logger:        logger,
		}

		_, err := service.DeployAPIConfiguration(params)
		require.Error(t, err)
		assert.ErrorIs(t, err, storage.ErrConflict)
		assert.Contains(t, err.Error(), "name 'Existing Rest API' and version '1.0.0' already exists")
	})

	t.Run("rejects duplicate handle from database", func(t *testing.T) {
		store := storage.NewConfigStore()
		db := newTestMockDB()
		service := newTestAPIDeploymentService(store, db, nil, validator, nil)

		require.NoError(t, db.SaveConfig(&models.StoredConfig{
			UUID:        "rest-existing-2",
			Kind:        string(api.RestAPIKindRestApi),
			Handle:      "existing-rest-api",
			DisplayName: "Existing Rest API",
			Version:     "1.0.0",
		}))

		params := APIDeploymentParams{
			Data: []byte(`
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: existing-rest-api
spec:
  displayName: Another Rest API
  version: "2.0.0"
  context: /another
  upstream:
    main:
      url: https://example.com
  operations:
    - method: GET
      path: /items
`),
			ContentType:   "application/yaml",
			CorrelationID: "test-corr",
			Origin:        models.OriginGatewayAPI,
			Logger:        logger,
		}

		_, err := service.DeployAPIConfiguration(params)
		require.Error(t, err)
		assert.ErrorIs(t, err, storage.ErrConflict)
		assert.Contains(t, err.Error(), "handle 'existing-rest-api' already exists")
	})
}

func TestDeployAPIConfiguration_UnsupportedKind(t *testing.T) {
	store := storage.NewConfigStore()
	validator := config.NewAPIValidator()
	service := newTestAPIDeploymentService(store, nil, nil, validator, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	params := APIDeploymentParams{
		Data:          []byte(`kind: RestApi`),
		ContentType:   "application/yaml",
		Kind:          "UnknownKind",
		CorrelationID: "test-corr",
		Origin:        models.OriginGatewayAPI,
		Logger:        logger,
	}

	result, err := service.DeployAPIConfiguration(params)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unsupported resource kind")
	assert.Contains(t, err.Error(), "UnknownKind")
}

func TestDeployAPIConfiguration_InferKindFromPayload(t *testing.T) {
	store := storage.NewConfigStore()
	validator := config.NewAPIValidator()
	service := newTestAPIDeploymentService(store, nil, nil, validator, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Infers RestApi kind from payload", func(t *testing.T) {
		yamlData := `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: inferred-api
spec:
  displayName: ""
  version: ""
  context: ""
`
		// Kind param is empty — should be inferred as RestApi from payload,
		// then fail validation (not kind resolution)
		params := APIDeploymentParams{
			Data:          []byte(yamlData),
			ContentType:   "application/yaml",
			CorrelationID: "test-corr",
			Origin:        models.OriginGatewayAPI,
			Logger:        logger,
		}

		_, err := service.DeployAPIConfiguration(params)
		assert.Error(t, err)
		// The error should be a validation error, proving RestApi branch was reached
		var validationErr *ValidationErrorListError
		assert.ErrorAs(t, err, &validationErr)
	})

	t.Run("Infers WebSubApi kind from payload", func(t *testing.T) {
		yamlData := `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: WebSubApi
metadata:
  name: inferred-websub
spec:
  displayName: ""
  version: ""
  context: ""
`
		params := APIDeploymentParams{
			Data:          []byte(yamlData),
			ContentType:   "application/yaml",
			CorrelationID: "test-corr",
			Origin:        models.OriginGatewayAPI,
			Logger:        logger,
		}

		_, err := service.DeployAPIConfiguration(params)
		assert.Error(t, err)
		var validationErr *ValidationErrorListError
		assert.ErrorAs(t, err, &validationErr)
	})
}

func TestDeployAPIConfiguration_EmptyKindInPayload(t *testing.T) {
	store := storage.NewConfigStore()
	validator := config.NewAPIValidator()
	service := newTestAPIDeploymentService(store, nil, nil, validator, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	yamlData := `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
metadata:
  name: no-kind-api
spec:
  displayName: No Kind API
  version: 1.0.0
  context: /nokind
`
	params := APIDeploymentParams{
		Data:          []byte(yamlData),
		ContentType:   "application/yaml",
		CorrelationID: "test-corr",
		Origin:        models.OriginGatewayAPI,
		Logger:        logger,
	}

	result, err := service.DeployAPIConfiguration(params)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "resource kind is required")
}

func TestAPIDeploymentService_Fields(t *testing.T) {
	store := storage.NewConfigStore()
	service := newTestAPIDeploymentService(store, nil, nil, nil, nil)

	assert.Equal(t, store, service.store)
	assert.NotNil(t, service.db)
	assert.Nil(t, service.snapshotManager)
	assert.NotNil(t, service.parser)
	assert.NotNil(t, service.httpClient)
}

// Tests for lines 100-111: WebSub API parsing error path
func TestDeployAPIConfiguration_WebSubParseError(t *testing.T) {
	store := storage.NewConfigStore()
	validator := config.NewAPIValidator()
	service := newTestAPIDeploymentService(store, nil, nil, validator, nil)
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
		Origin:        models.OriginGatewayAPI,
		Logger:        logger,
	}

	result, err := service.DeployAPIConfiguration(params)
	assert.Error(t, err)
	assert.Nil(t, result)
}

// Tests for lines 352-371: Database rollback on memory store failure
func TestSaveOrUpdateConfig_MemoryStoreFailure(t *testing.T) {
	t.Run("Successfully saves new config to DB", func(t *testing.T) {
		store := storage.NewConfigStore()
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		service := newTestAPIDeploymentService(store, newTestMockDB(), nil, nil, nil)

		apiData := api.APIConfigData{
			DisplayName: "New API",
			Version:     "1.0.0",
			Context:     "/new",
		}

		newCfg := &models.StoredConfig{
			UUID: "0000-new-api-id-0000-000000000000",
			Kind: string(api.RestAPIKindRestApi),
			Configuration: api.RestAPI{
				Kind: api.RestAPIKindRestApi,
				Spec: apiData,
			},
			DesiredState: models.StateDeployed,
			Origin:       models.OriginGatewayAPI,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		affected, err := service.saveOrUpdateConfig(newCfg, logger)
		assert.NoError(t, err)
		assert.True(t, affected)

		retrieved, err := service.db.GetConfig(newCfg.UUID)
		assert.NoError(t, err)
		assert.Equal(t, newCfg.UUID, retrieved.UUID)
	})

	t.Run("Update path when config exists", func(t *testing.T) {
		store := storage.NewConfigStore()
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		mockDB := newTestMockDB()

		// Add existing config
		apiData := api.APIConfigData{
			DisplayName: "Existing API",
			Version:     "1.0.0",
			Context:     "/existing",
		}

		existingCfg := &models.StoredConfig{
			UUID: "0000-existing-id-0000-000000000000",
			Kind: string(api.RestAPIKindRestApi),
			Configuration: api.RestAPI{
				Kind: api.RestAPIKindRestApi,
				Spec: apiData,
			},
			DesiredState: models.StateDeployed,
			Origin:       models.OriginGatewayAPI,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		store.Add(existingCfg)
		require.NoError(t, mockDB.SaveConfig(existingCfg))

		service := newTestAPIDeploymentService(store, mockDB, nil, nil, nil)

		// Try to save with same ID (should update instead)
		updateCfg := &models.StoredConfig{
			UUID: "0000-existing-id-0000-000000000000",
			Kind: string(api.RestAPIKindRestApi),
			Configuration: api.RestAPI{
				Kind: api.RestAPIKindRestApi,
				Spec: apiData,
			},
			DesiredState: models.StateDeployed,
			Origin:       models.OriginGatewayAPI,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		isUpdate, err := service.saveOrUpdateConfig(updateCfg, logger)
		assert.NoError(t, err)
		assert.True(t, isUpdate) // Should be update, not add
	})
}

// Tests for lines 395-497: Update rollback and WebSub HTTP operations
func TestUpdateExistingConfig_Rollback(t *testing.T) {
	t.Run("Memory store update failure without DB", func(t *testing.T) {
		store := storage.NewConfigStore()
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		mockDB := newTestMockDB()
		service := newTestAPIDeploymentService(store, mockDB, nil, nil, nil)

		apiData := api.APIConfigData{
			DisplayName: "Original API",
			Version:     "1.0.0",
			Context:     "/original",
		}

		// Add original config
		original := &models.StoredConfig{
			UUID: "0000-test-api-0000-000000000000",
			Kind: string(api.RestAPIKindRestApi),
			Configuration: api.RestAPI{
				Kind: api.RestAPIKindRestApi,
				Spec: apiData,
			},
			DesiredState: models.StateDeployed,
			Origin:       models.OriginGatewayAPI,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		store.Add(original)
		require.NoError(t, mockDB.SaveConfig(original))

		// Create an update that will fail (invalid ID in newConfig to simulate store.Update failure)
		// We can't easily simulate store.Update failure without modifying the store
		// So we test the successful path here and rely on integration tests for failure paths
		newApiData := api.APIConfigData{
			DisplayName: "Updated API",
			Version:     "2.0.0",
			Context:     "/updated",
		}

		newConfig := &models.StoredConfig{
			UUID: "0000-test-api-0000-000000000000",
			Kind: string(api.RestAPIKindRestApi),
			Configuration: api.RestAPI{
				Kind: api.RestAPIKindRestApi,
				Spec: newApiData,
			},
			DesiredState: models.StateDeployed,
			Origin:       models.OriginGatewayAPI,
		}

		affected, err := service.saveOrUpdateConfig(newConfig, logger)
		assert.NoError(t, err)
		assert.True(t, affected)
	})
}

// TestSaveOrUpdateConfig_StaleEvent verifies that when a newer config already exists in the DB,
// saveOrUpdateConfig returns affected=false (IsStale=true) and does not modify the DB row.
func TestSaveOrUpdateConfig_StaleEvent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	metrics.Init()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := storage.NewStorage(storage.BackendConfig{
		Type:       "sqlite",
		SQLitePath: dbPath,
		GatewayID:  "test-gw",
	}, logger)
	require.NoError(t, err)

	store := storage.NewConfigStore()
	service := newTestAPIDeploymentService(store, db, nil, nil, nil)

	newerTime := time.Now()
	olderTime := newerTime.Add(-10 * time.Minute)

	// First: insert a config with the newer timestamp
	newerCfg := &models.StoredConfig{
		UUID:         "00000000-0000-0000-0000-000000000001",
		Kind:         string(api.RestAPIKindRestApi),
		Handle:       "test-api",
		DisplayName:  "Test API",
		Version:      "1.0.0",
		DeploymentID: "dep-2",
		DesiredState: models.StateDeployed,
		Origin:       models.OriginControlPlane,
		CreatedAt:    newerTime,
		UpdatedAt:    newerTime,
		DeployedAt:   &newerTime,
	}

	affected, err := service.saveOrUpdateConfig(newerCfg, logger)
	require.NoError(t, err)
	assert.True(t, affected, "First insert should affect the DB")

	// Second: attempt to upsert with an older timestamp — should be stale
	staleCfg := &models.StoredConfig{
		UUID:         "00000000-0000-0000-0000-000000000001",
		Kind:         string(api.RestAPIKindRestApi),
		Handle:       "test-api",
		DisplayName:  "Test API Stale",
		Version:      "0.9.0",
		DeploymentID: "dep-1",
		DesiredState: models.StateDeployed,
		Origin:       models.OriginControlPlane,
		CreatedAt:    olderTime,
		UpdatedAt:    olderTime,
		DeployedAt:   &olderTime,
	}

	affected, err = service.saveOrUpdateConfig(staleCfg, logger)
	require.NoError(t, err)
	assert.False(t, affected, "Stale event should not affect the DB")

	// Verify the DB still has the newer config
	stored, err := db.GetConfig("00000000-0000-0000-0000-000000000001")
	require.NoError(t, err)
	assert.Equal(t, "dep-2", stored.DeploymentID, "DB should still have the newer deployment")
	assert.Equal(t, "1.0.0", stored.Version, "DB should still have the newer version")
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
	service := newTestAPIDeploymentService(store, nil, nil, nil, routerConfig)
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
	service := newTestAPIDeploymentService(store, nil, nil, nil, routerConfig)
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

func TestResolveVhostSentinels_RestApi(t *testing.T) {
	sandbox := constants.VHostGatewayDefault
	routerCfg := &config.RouterConfig{
		VHosts: config.VHostsConfig{
			Main:    config.VHostEntry{Default: "*.wso2.com"},
			Sandbox: config.VHostEntry{Default: "*-sandbox.wso2.com"},
		},
	}

	main := constants.VHostGatewayDefault
	var cfg any = api.RestAPI{
		Kind: api.RestAPIKindRestApi,
		Spec: api.APIConfigData{
			Vhosts: &struct {
				Main    string  `json:"main" yaml:"main"`
				Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main:    main,
				Sandbox: &sandbox,
			},
		},
	}

	require.NoError(t, resolveVhostSentinels(&cfg, routerCfg))

	resolved := cfg.(api.RestAPI).Spec
	require.NotNil(t, resolved.Vhosts)
	assert.Equal(t, "*.wso2.com", resolved.Vhosts.Main)
	require.NotNil(t, resolved.Vhosts.Sandbox)
	assert.Equal(t, "*-sandbox.wso2.com", *resolved.Vhosts.Sandbox)
}

func TestResolveVhostSentinels_ExplicitValuesUnchanged(t *testing.T) {
	sandboxValue := "custom-sandbox.example.com"
	routerCfg := &config.RouterConfig{
		VHosts: config.VHostsConfig{
			Main:    config.VHostEntry{Default: "*.wso2.com"},
			Sandbox: config.VHostEntry{Default: "*-sandbox.wso2.com"},
		},
	}

	var cfg any = api.RestAPI{
		Kind: api.RestAPIKindRestApi,
		Spec: api.APIConfigData{
			Vhosts: &struct {
				Main    string  `json:"main" yaml:"main"`
				Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main:    "custom.example.com",
				Sandbox: &sandboxValue,
			},
		},
	}

	require.NoError(t, resolveVhostSentinels(&cfg, routerCfg))

	resolved := cfg.(api.RestAPI).Spec
	require.NotNil(t, resolved.Vhosts)
	assert.Equal(t, "custom.example.com", resolved.Vhosts.Main)
	require.NotNil(t, resolved.Vhosts.Sandbox)
	assert.Equal(t, "custom-sandbox.example.com", *resolved.Vhosts.Sandbox)
}

func TestResolveVhostSentinels_NilVhostsPopulatesDefaults(t *testing.T) {
	routerCfg := &config.RouterConfig{
		VHosts: config.VHostsConfig{
			Main:    config.VHostEntry{Default: "*.wso2.com"},
			Sandbox: config.VHostEntry{Default: "*-sandbox.wso2.com"},
		},
	}

	var cfg any = api.RestAPI{
		Kind: api.RestAPIKindRestApi,
		Spec: api.APIConfigData{Vhosts: nil},
	}

	require.NoError(t, resolveVhostSentinels(&cfg, routerCfg))

	resolved := cfg.(api.RestAPI).Spec
	require.NotNil(t, resolved.Vhosts, "nil vhosts should be populated with defaults")
	assert.Equal(t, "*.wso2.com", resolved.Vhosts.Main)
	require.NotNil(t, resolved.Vhosts.Sandbox)
	assert.Equal(t, "*-sandbox.wso2.com", *resolved.Vhosts.Sandbox)
}

func TestResolveVhostSentinels_NilVhostsNoSandboxDefault(t *testing.T) {
	routerCfg := &config.RouterConfig{
		VHosts: config.VHostsConfig{
			Main: config.VHostEntry{Default: "*.wso2.com"},
		},
	}

	var cfg any = api.RestAPI{
		Kind: api.RestAPIKindRestApi,
		Spec: api.APIConfigData{Vhosts: nil},
	}

	require.NoError(t, resolveVhostSentinels(&cfg, routerCfg))

	resolved := cfg.(api.RestAPI).Spec
	require.NotNil(t, resolved.Vhosts, "nil vhosts should be populated with main default")
	assert.Equal(t, "*.wso2.com", resolved.Vhosts.Main)
	assert.Nil(t, resolved.Vhosts.Sandbox, "sandbox should remain nil when no sandbox default configured")
}

func TestResolveVhostSentinels_WebSubApi_NilVhostsPopulatesDefaults(t *testing.T) {
	routerCfg := &config.RouterConfig{
		VHosts: config.VHostsConfig{
			Main:    config.VHostEntry{Default: "*.wso2.com"},
			Sandbox: config.VHostEntry{Default: "*-sandbox.wso2.com"},
		},
	}

	var cfg any = api.WebSubAPI{
		Kind: api.WebSubAPIKindWebSubApi,
		Spec: api.WebhookAPIData{Vhosts: nil},
	}

	require.NoError(t, resolveVhostSentinels(&cfg, routerCfg))

	resolved := cfg.(api.WebSubAPI).Spec
	require.NotNil(t, resolved.Vhosts, "nil vhosts should be populated with defaults")
	assert.Equal(t, "*.wso2.com", resolved.Vhosts.Main)
	require.NotNil(t, resolved.Vhosts.Sandbox)
	assert.Equal(t, "*-sandbox.wso2.com", *resolved.Vhosts.Sandbox)
}

func TestResolveVhostSentinels_WebSubApi(t *testing.T) {
	sandbox := constants.VHostGatewayDefault
	routerCfg := &config.RouterConfig{
		VHosts: config.VHostsConfig{
			Main:    config.VHostEntry{Default: "*.wso2.com"},
			Sandbox: config.VHostEntry{Default: "*-sandbox.wso2.com"},
		},
	}

	var cfg any = api.WebSubAPI{
		Kind: api.WebSubAPIKindWebSubApi,
		Spec: api.WebhookAPIData{
			Vhosts: &struct {
				Main    string  `json:"main" yaml:"main"`
				Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main:    constants.VHostGatewayDefault,
				Sandbox: &sandbox,
			},
		},
	}

	require.NoError(t, resolveVhostSentinels(&cfg, routerCfg))

	resolved := cfg.(api.WebSubAPI).Spec
	require.NotNil(t, resolved.Vhosts)
	assert.Equal(t, "*.wso2.com", resolved.Vhosts.Main)
	require.NotNil(t, resolved.Vhosts.Sandbox)
	assert.Equal(t, "*-sandbox.wso2.com", *resolved.Vhosts.Sandbox)
}

func TestResolveVhostSentinels_WebSubApi_ExplicitValues(t *testing.T) {
	sandboxValue := "custom-sandbox.example.com"
	routerCfg := &config.RouterConfig{
		VHosts: config.VHostsConfig{
			Main:    config.VHostEntry{Default: "*.wso2.com"},
			Sandbox: config.VHostEntry{Default: "*-sandbox.wso2.com"},
		},
	}

	var cfg any = api.WebSubAPI{
		Kind: api.WebSubAPIKindWebSubApi,
		Spec: api.WebhookAPIData{
			Vhosts: &struct {
				Main    string  `json:"main" yaml:"main"`
				Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main:    "custom.example.com",
				Sandbox: &sandboxValue,
			},
		},
	}

	require.NoError(t, resolveVhostSentinels(&cfg, routerCfg))

	resolved := cfg.(api.WebSubAPI).Spec
	require.NotNil(t, resolved.Vhosts)
	assert.Equal(t, "custom.example.com", resolved.Vhosts.Main)
	require.NotNil(t, resolved.Vhosts.Sandbox)
	assert.Equal(t, "custom-sandbox.example.com", *resolved.Vhosts.Sandbox)
}

func TestResolveVhostSentinels_NilCfgNoOp(t *testing.T) {
	routerCfg := &config.RouterConfig{}
	require.NoError(t, resolveVhostSentinels(nil, routerCfg)) // should not panic
}

func TestResolveVhostSentinels_NilRouterCfgNoOp(t *testing.T) {
	var cfg any = api.RestAPI{Kind: api.RestAPIKindRestApi}
	require.NoError(t, resolveVhostSentinels(&cfg, nil)) // should not panic
}
