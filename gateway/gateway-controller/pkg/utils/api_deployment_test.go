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
		uuid := generateUUID()
		assert.NotEmpty(t, uuid)
		assert.Len(t, uuid, 36) // Standard UUID length with hyphens
	})

	t.Run("Generates unique UUIDs", func(t *testing.T) {
		uuid1 := generateUUID()
		uuid2 := generateUUID()
		assert.NotEqual(t, uuid1, uuid2)
	})
}

func TestSaveOrUpdateConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Save new config without DB", func(t *testing.T) {
		store := storage.NewConfigStore()
		service := NewAPIDeploymentService(store, nil, nil, nil, nil)

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
			},
			Status:    models.StatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		isUpdate, err := service.saveOrUpdateConfig(storedCfg, logger)
		assert.NoError(t, err)
		assert.False(t, isUpdate)

		// Verify config was added
		retrieved, err := store.Get(storedCfg.ID)
		assert.NoError(t, err)
		assert.Equal(t, storedCfg.ID, retrieved.ID)
	})

	t.Run("Update existing config without DB", func(t *testing.T) {
		store := storage.NewConfigStore()
		service := NewAPIDeploymentService(store, nil, nil, nil, nil)

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
			},
			Status:    models.StatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		store.Add(existingCfg)

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
			},
			Status:    models.StatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		isUpdate, err := service.saveOrUpdateConfig(updateCfg, logger)
		assert.NoError(t, err)
		assert.True(t, isUpdate)
	})
}

func TestUpdateExistingConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Updates existing config successfully", func(t *testing.T) {
		store := storage.NewConfigStore()
		service := NewAPIDeploymentService(store, nil, nil, nil, nil)

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
			},
			Status:    models.StatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		store.Add(original)

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
			},
			Status: models.StatusPending,
		}

		isUpdate, err := service.updateExistingConfig(newConfig, original, logger)
		assert.NoError(t, err)
		assert.True(t, isUpdate)
	})
}

func TestDeployAPIConfiguration_ParseError(t *testing.T) {
	store := storage.NewConfigStore()
	validator := config.NewAPIValidator()
	service := NewAPIDeploymentService(store, nil, nil, validator, nil)
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
	service := NewAPIDeploymentService(store, nil, nil, validator, nil)
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

func TestAPIDeploymentService_Fields(t *testing.T) {
	store := storage.NewConfigStore()
	service := NewAPIDeploymentService(store, nil, nil, nil, nil)

	assert.Equal(t, store, service.store)
	assert.Nil(t, service.db)
	assert.Nil(t, service.snapshotManager)
	assert.NotNil(t, service.parser)
	assert.NotNil(t, service.httpClient)
}
