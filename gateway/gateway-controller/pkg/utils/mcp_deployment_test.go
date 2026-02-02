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
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

func TestNewMCPDeploymentService(t *testing.T) {
	store := storage.NewConfigStore()
	service := NewMCPDeploymentService(store, nil, nil)

	assert.NotNil(t, service)
	assert.NotNil(t, service.store)
	assert.NotNil(t, service.parser)
	assert.NotNil(t, service.validator)
	assert.NotNil(t, service.transformer)
}

func TestMCPDeploymentParams(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	params := MCPDeploymentParams{
		Data:          []byte("test data"),
		ContentType:   "application/yaml",
		ID:            "test-mcp-id",
		CorrelationID: "corr-123",
		Logger:        logger,
	}

	assert.Equal(t, "test data", string(params.Data))
	assert.Equal(t, "application/yaml", params.ContentType)
	assert.Equal(t, "test-mcp-id", params.ID)
	assert.Equal(t, "corr-123", params.CorrelationID)
	assert.NotNil(t, params.Logger)
}

func TestMCPDeploymentService_ListMCPProxies(t *testing.T) {
	store := storage.NewConfigStore()
	service := NewMCPDeploymentService(store, nil, nil)

	t.Run("Empty store returns empty list", func(t *testing.T) {
		proxies := service.ListMCPProxies()
		assert.Empty(t, proxies)
	})

	t.Run("Returns only MCP kind configs", func(t *testing.T) {
		// Add an MCP config
		apiData := api.APIConfigData{
			DisplayName: "MCP Proxy",
			Version:     "1.0.0",
			Context:     "/mcp",
		}
		var spec api.APIConfiguration_Spec
		spec.FromAPIConfigData(apiData)

		mcpConfig := &models.StoredConfig{
			ID:   "mcp-1",
			Kind: string(api.Mcp),
			Configuration: api.APIConfiguration{
				Kind: api.RestApi,
				Spec: spec,
			},
			Status:    models.StatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		store.Add(mcpConfig)

		// Add a REST API config (should not be returned)
		restConfig := &models.StoredConfig{
			ID:   "rest-1",
			Kind: string(api.RestApi),
			Configuration: api.APIConfiguration{
				Kind: api.RestApi,
				Spec: spec,
			},
			Status:    models.StatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		store.Add(restConfig)

		proxies := service.ListMCPProxies()
		assert.Len(t, proxies, 1)
		assert.Equal(t, "mcp-1", proxies[0].ID)
	})
}

func TestMCPDeploymentService_GetMCPProxyByHandle_NoDatabase(t *testing.T) {
	store := storage.NewConfigStore()
	service := NewMCPDeploymentService(store, nil, nil)

	_, err := service.GetMCPProxyByHandle("test-handle")
	assert.Error(t, err)
	assert.Equal(t, storage.ErrDatabaseUnavailable, err)
}

func TestMCPDeploymentService_CreateMCPProxy_ParseError(t *testing.T) {
	store := storage.NewConfigStore()
	service := NewMCPDeploymentService(store, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	params := MCPDeploymentParams{
		Data:          []byte("invalid yaml: ["),
		ContentType:   "application/yaml",
		CorrelationID: "test-corr",
		Logger:        logger,
	}

	_, err := service.CreateMCPProxy(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestMCPDeploymentService_CreateMCPProxy_ValidationError(t *testing.T) {
	store := storage.NewConfigStore()
	service := NewMCPDeploymentService(store, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Invalid MCP config that will fail validation
	yamlData := `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Mcp
metadata:
  name: test-mcp
spec:
  displayName: ""
  version: ""
`
	params := MCPDeploymentParams{
		Data:          []byte(yamlData),
		ContentType:   "application/yaml",
		CorrelationID: "test-corr",
		Logger:        logger,
	}

	_, err := service.CreateMCPProxy(params)
	assert.Error(t, err)
}

func TestMCPDeploymentService_CreateMCPProxy_ConflictError(t *testing.T) {
	store := storage.NewConfigStore()
	service := NewMCPDeploymentService(store, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// First, add an existing config with the same name/version
	apiData := api.APIConfigData{
		DisplayName: "Test MCP",
		Version:     "1.0.0",
		Context:     "/test",
	}
	var spec api.APIConfiguration_Spec
	spec.FromAPIConfigData(apiData)

	existingConfig := &models.StoredConfig{
		ID:   "existing-mcp",
		Kind: string(api.Mcp),
		Configuration: api.APIConfiguration{
			Kind:     api.RestApi,
			Metadata: api.Metadata{Name: "test-mcp"},
			Spec:     spec,
		},
		Status:    models.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.Add(existingConfig)

	// Try to create another with the same name/version
	upstreamURL := "http://localhost:8080"
	yamlData := `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Mcp
metadata:
  name: test-mcp
spec:
  displayName: Test MCP
  version: "1.0.0"
  context: "/test"
  upstream:
    url: "` + upstreamURL + `"
`
	params := MCPDeploymentParams{
		Data:          []byte(yamlData),
		ContentType:   "application/yaml",
		CorrelationID: "test-corr",
		Logger:        logger,
	}

	_, err := service.CreateMCPProxy(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestMCPDeploymentService_DeleteMCPProxy_NoDatabase(t *testing.T) {
	store := storage.NewConfigStore()
	service := NewMCPDeploymentService(store, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	_, err := service.DeleteMCPProxy("test-handle", "corr-id", logger)
	assert.Error(t, err)
	assert.Equal(t, storage.ErrDatabaseUnavailable, err)
}

func TestMCPDeploymentService_UpdateMCPProxy_NoDatabase(t *testing.T) {
	store := storage.NewConfigStore()
	service := NewMCPDeploymentService(store, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	params := MCPDeploymentParams{
		Data:          []byte("test data"),
		ContentType:   "application/yaml",
		CorrelationID: "corr-id",
		Logger:        logger,
	}

	_, err := service.UpdateMCPProxy("test-handle", params, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestMCPDeploymentService_SaveOrUpdateConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Save new config without DB", func(t *testing.T) {
		store := storage.NewConfigStore()
		service := NewMCPDeploymentService(store, nil, nil)

		apiData := api.APIConfigData{
			DisplayName: "Test MCP",
			Version:     "1.0.0",
			Context:     "/test",
		}
		var spec api.APIConfiguration_Spec
		spec.FromAPIConfigData(apiData)

		storedCfg := &models.StoredConfig{
			ID:   "new-mcp-id",
			Kind: string(api.Mcp),
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
}

func TestMCPDeploymentService_UpdateExistingConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Updates existing config", func(t *testing.T) {
		store := storage.NewConfigStore()
		service := NewMCPDeploymentService(store, nil, nil)

		apiData := api.APIConfigData{
			DisplayName: "Original MCP",
			Version:     "1.0.0",
			Context:     "/original",
		}
		var spec api.APIConfiguration_Spec
		spec.FromAPIConfigData(apiData)

		// Add original config
		original := &models.StoredConfig{
			ID:   "config-to-update",
			Kind: string(api.Mcp),
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
			DisplayName: "Original MCP",
			Version:     "1.0.0",
			Context:     "/updated",
		}
		var newSpec api.APIConfiguration_Spec
		newSpec.FromAPIConfigData(newApiData)

		newConfig := &models.StoredConfig{
			ID:   "config-to-update",
			Kind: string(api.Mcp),
			Configuration: api.APIConfiguration{
				Kind: api.RestApi,
				Spec: newSpec,
			},
			Status: models.StatusPending,
		}

		isUpdate, err := service.updateExistingConfig(newConfig, logger)
		assert.NoError(t, err)
		assert.True(t, isUpdate)
	})

	t.Run("Error when config not found", func(t *testing.T) {
		store := storage.NewConfigStore()
		service := NewMCPDeploymentService(store, nil, nil)

		apiData := api.APIConfigData{
			DisplayName: "Non-existent MCP",
			Version:     "1.0.0",
			Context:     "/non-existent",
		}
		var spec api.APIConfiguration_Spec
		spec.FromAPIConfigData(apiData)

		newConfig := &models.StoredConfig{
			ID:   "non-existent-config",
			Kind: string(api.Mcp),
			Configuration: api.APIConfiguration{
				Kind: api.RestApi,
				Spec: spec,
			},
			Status: models.StatusPending,
		}

		_, err := service.updateExistingConfig(newConfig, logger)
		assert.Error(t, err)
	})
}

func TestMCPDeploymentService_ParseValidateAndTransform(t *testing.T) {
	store := storage.NewConfigStore()
	service := NewMCPDeploymentService(store, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Valid MCP config", func(t *testing.T) {
		yamlData := `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Mcp
metadata:
  name: test-mcp
spec:
  displayName: Test MCP Proxy
  version: "1.0.0"
  context: "/test"
  upstream:
    url: "http://localhost:8080"
`
		params := MCPDeploymentParams{
			Data:          []byte(yamlData),
			ContentType:   "application/yaml",
			CorrelationID: "test-corr",
			Logger:        logger,
		}

		mcpConfig, apiConfig, err := service.parseValidateAndTransform(params)
		assert.NoError(t, err)
		assert.NotNil(t, mcpConfig)
		assert.NotNil(t, apiConfig)
		assert.Equal(t, "Test MCP Proxy", mcpConfig.Spec.DisplayName)
		assert.Equal(t, "1.0.0", mcpConfig.Spec.Version)
	})

	t.Run("Invalid parse returns error", func(t *testing.T) {
		params := MCPDeploymentParams{
			Data:          []byte("invalid: [yaml"),
			ContentType:   "application/yaml",
			CorrelationID: "test-corr",
			Logger:        logger,
		}

		_, _, err := service.parseValidateAndTransform(params)
		assert.Error(t, err)
	})
}

// Note: TestMCPDeploymentService_DeployMCPConfiguration is skipped because
// DeployMCPConfiguration spawns a goroutine that calls snapshotManager.UpdateSnapshot
// which would require a non-nil snapshot manager.
// The core deployment logic is tested indirectly through other tests.

func TestLatestSupportedMCPSpecVersion(t *testing.T) {
	assert.Equal(t, "2025-06-18", LATEST_SUPPORTED_MCP_SPEC_VERSION)
}

// Note: TestMCPDeploymentService_DeployMCPConfiguration_Update is skipped
// for the same reason as above - it calls DeployMCPConfiguration which
// requires a non-nil snapshot manager.
