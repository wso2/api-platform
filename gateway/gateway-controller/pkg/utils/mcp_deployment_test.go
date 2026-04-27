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
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

func newUnhydratedTestMCPStoredConfig(id, handle, displayName, version, contextPath string) *models.StoredConfig {
	upstreamURL := "http://localhost:8080"

	return &models.StoredConfig{
		UUID:        id,
		Kind:        string(api.MCPProxyConfigurationKindMcp),
		Handle:      handle,
		DisplayName: displayName,
		Version:     version,
		SourceConfiguration: api.MCPProxyConfiguration{
			ApiVersion: api.MCPProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.MCPProxyConfigurationKindMcp,
			Metadata:   api.Metadata{Name: handle},
			Spec: api.MCPProxyConfigData{
				DisplayName: displayName,
				Version:     version,
				Context:     stringPtr(contextPath),
				Upstream: api.MCPProxyConfigData_Upstream{
					Url: &upstreamURL,
				},
			},
		},
		DesiredState: models.StateDeployed,
		Origin:       models.OriginGatewayAPI,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

func TestNewMCPDeploymentService(t *testing.T) {
	store := storage.NewConfigStore()
	service := newTestMCPDeploymentService(store, nil, nil, nil, nil)

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
	t.Run("Empty store returns empty list", func(t *testing.T) {
		store := storage.NewConfigStore()
		db := newTestMockDB()
		service := newTestMCPDeploymentService(store, db, nil, nil, nil)

		proxies, err := service.ListMCPProxies()
		require.NoError(t, err)
		assert.Empty(t, proxies)
	})

	t.Run("Returns only MCP kind configs", func(t *testing.T) {
		store := storage.NewConfigStore()
		db := newTestMockDB()
		service := newTestMCPDeploymentService(store, db, nil, nil, nil)

		// Add an MCP config
		apiData := api.APIConfigData{
			DisplayName: "MCP Proxy",
			Version:     "1.0.0",
			Context:     "/mcp",
		}

		mcpConfig := &models.StoredConfig{
			UUID:        "0000-mcp-1-0000-000000000000",
			Kind:        string(api.MCPProxyConfigurationKindMcp),
			Handle:      "mcp-proxy",
			DisplayName: "MCP Proxy",
			Version:     "1.0.0",
			Configuration: api.RestAPI{
				Kind:     api.RestAPIKindRestApi,
				Metadata: api.Metadata{Name: "mcp-proxy"},
				Spec:     apiData,
			},
			DesiredState: models.StateDeployed,
			Origin:       models.OriginGatewayAPI,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		require.NoError(t, db.SaveConfig(mcpConfig))

		// Add a REST API config (should not be returned)
		restConfig := &models.StoredConfig{
			UUID:        "0000-rest-1-0000-000000000000",
			Kind:        string(api.RestAPIKindRestApi),
			Handle:      "rest-api",
			DisplayName: "MCP Proxy",
			Version:     "2.0.0",
			Configuration: api.RestAPI{
				Kind:     api.RestAPIKindRestApi,
				Metadata: api.Metadata{Name: "rest-api"},
				Spec:     apiData,
			},
			DesiredState: models.StateDeployed,
			Origin:       models.OriginGatewayAPI,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		require.NoError(t, db.SaveConfig(restConfig))

		proxies, err := service.ListMCPProxies()
		require.NoError(t, err)
		assert.Len(t, proxies, 1)
		assert.Equal(t, "0000-mcp-1-0000-000000000000", proxies[0].UUID)
	})

	t.Run("Hydrates database-backed MCP configs before returning", func(t *testing.T) {
		store := storage.NewConfigStore()
		db := newTestMockDB()
		service := newTestMCPDeploymentService(store, db, nil, nil, nil)

		cfg := newUnhydratedTestMCPStoredConfig("0000-db-mcp-1-0000-000000000000", "db-mcp", "DB MCP", "1.0.0", "/db-mcp")
		require.NoError(t, db.SaveConfig(cfg))

		proxies, err := service.ListMCPProxies()
		require.NoError(t, err)
		require.Len(t, proxies, 1)

		sourceCfg, ok := proxies[0].SourceConfiguration.(api.MCPProxyConfiguration)
		require.True(t, ok, "expected canonical MCP source configuration")
		assert.Equal(t, "db-mcp", sourceCfg.Metadata.Name)
	})
}

func TestIsMCPNotFoundError_UsesStorageSentinelOnly(t *testing.T) {
	assert.True(t, isMCPNotFoundError(storage.ErrNotFound))
	assert.True(t, isMCPNotFoundError(fmt.Errorf("wrapped: %w", storage.ErrNotFound)))
	assert.False(t, isMCPNotFoundError(errors.New("not found")))
}

func TestMCPDeploymentService_getMCPProxyByID_LogsHydrationFailures(t *testing.T) {
	originalLogger := slog.Default()
	var logBuf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})

	t.Run("database-backed lookup", func(t *testing.T) {
		logBuf.Reset()

		store := storage.NewConfigStore()
		db := newTestMockDB()
		service := newTestMCPDeploymentService(store, db, nil, nil, nil)
		cfg := &models.StoredConfig{
			UUID:                "0000-bad-db-mcp-0000-000000000000",
			Kind:                string(api.MCPProxyConfigurationKindMcp),
			Handle:              "bad-db-mcp",
			SourceConfiguration: "invalid",
			DesiredState:        models.StateDeployed,
			Origin:              models.OriginGatewayAPI,
			CreatedAt:           time.Now(),
			UpdatedAt:           time.Now(),
		}
		require.NoError(t, db.SaveConfig(cfg))

		found, err := service.getMCPProxyByID(cfg.UUID)
		require.NoError(t, err)
		assert.Equal(t, cfg.UUID, found.UUID)
		assert.Contains(t, logBuf.String(), "failed to hydrate StoredConfig")
		assert.Contains(t, logBuf.String(), cfg.UUID)
		assert.Contains(t, logBuf.String(), "unexpected MCP source configuration type")
	})

}

func TestMCPDeploymentService_GetMCPProxyByHandle(t *testing.T) {
	store := storage.NewConfigStore()
	db := newTestMockDB()
	service := newTestMCPDeploymentService(store, db, nil, nil, nil)
	upstreamURL := "http://localhost:8080"

	cfg := &models.StoredConfig{
		UUID:        "0000-test-handle-0000-000000000000",
		Kind:        string(api.MCPProxyConfigurationKindMcp),
		Handle:      "test-mcp",
		DisplayName: "Test MCP",
		Version:     "1.0.0",
		SourceConfiguration: api.MCPProxyConfiguration{
			ApiVersion: api.MCPProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.MCPProxyConfigurationKindMcp,
			Metadata:   api.Metadata{Name: "test-mcp"},
			Spec: api.MCPProxyConfigData{
				DisplayName: "Test MCP",
				Version:     "1.0.0",
				Context:     stringPtr("/mcp"),
				Upstream: api.MCPProxyConfigData_Upstream{
					Url: &upstreamURL,
				},
			},
		},
		DesiredState: models.StateDeployed,
		Origin:       models.OriginGatewayAPI,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	require.NoError(t, HydrateStoredMCPConfig(cfg))
	require.NoError(t, db.SaveConfig(cfg))

	found, err := service.GetMCPProxyByHandle("test-mcp")
	require.NoError(t, err)
	assert.Equal(t, cfg.UUID, found.UUID)
}
func TestMCPDeploymentService_CreateMCPProxy_ParseError(t *testing.T) {
	store := storage.NewConfigStore()
	service := newTestMCPDeploymentService(store, nil, nil, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	params := MCPDeploymentParams{
		Data:          []byte("invalid yaml: ["),
		ContentType:   "application/yaml",
		Origin:        models.OriginGatewayAPI,
		CorrelationID: "test-corr",
		Logger:        logger,
	}

	_, err := service.CreateMCPProxy(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestMCPDeploymentService_CreateMCPProxy_ValidationError(t *testing.T) {
	store := storage.NewConfigStore()
	service := newTestMCPDeploymentService(store, nil, nil, nil, nil)
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
		Origin:        models.OriginGatewayAPI,
		CorrelationID: "test-corr",
		Logger:        logger,
	}

	_, err := service.CreateMCPProxy(params)
	assert.Error(t, err)
}

func TestMCPDeploymentService_CreateMCPProxy_ConflictError(t *testing.T) {
	store := storage.NewConfigStore()
	service := newTestMCPDeploymentService(store, nil, nil, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// First, add an existing config with the same name/version
	apiData := api.APIConfigData{
		DisplayName: "Test MCP",
		Version:     "1.0.0",
		Context:     "/test",
	}

	existingConfig := &models.StoredConfig{
		UUID:        "0000-existing-mcp-0000-000000000000",
		Kind:        string(api.MCPProxyConfigurationKindMcp),
		Handle:      "test-mcp",
		DisplayName: "Test MCP",
		Version:     "1.0.0",
		Configuration: api.RestAPI{
			Kind:     api.RestAPIKindRestApi,
			Metadata: api.Metadata{Name: "test-mcp"},
			Spec:     apiData,
		},
		DesiredState: models.StateDeployed,
		Origin:       models.OriginGatewayAPI,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	require.NoError(t, service.db.SaveConfig(existingConfig))

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
		Origin:        models.OriginGatewayAPI,
		CorrelationID: "test-corr",
		Logger:        logger,
	}

	_, err := service.CreateMCPProxy(params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestMCPDeploymentService_DeleteMCPProxy_NoDatabase(t *testing.T) {
	store := storage.NewConfigStore()
	service := newTestMCPDeploymentService(store, nil, nil, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	_, err := service.DeleteMCPProxy("0000-test-handle-0000-000000000000", "corr-id", logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestMCPDeploymentService_UpdateMCPProxy_NoDatabase(t *testing.T) {
	store := storage.NewConfigStore()
	service := newTestMCPDeploymentService(store, nil, nil, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	params := MCPDeploymentParams{
		Data:          []byte("test data"),
		ContentType:   "application/yaml",
		Origin:        models.OriginGatewayAPI,
		CorrelationID: "corr-id",
		Logger:        logger,
	}

	_, err := service.UpdateMCPProxy("0000-test-handle-0000-000000000000", params, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
func TestMCPDeploymentService_SaveOrUpdateConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Save new config without DB", func(t *testing.T) {
		store := storage.NewConfigStore()
		service := newTestMCPDeploymentService(store, newTestMockDB(), nil, nil, nil)

		apiData := api.APIConfigData{
			DisplayName: "Test MCP",
			Version:     "1.0.0",
			Context:     "/test",
		}

		storedCfg := &models.StoredConfig{
			UUID:        "0000-new-mcp-id-0000-000000000000",
			Kind:        string(api.MCPProxyConfigurationKindMcp),
			Handle:      "test-mcp",
			DisplayName: "Test MCP",
			Version:     "1.0.0",
			Configuration: api.RestAPI{
				Kind:     api.RestAPIKindRestApi,
				Metadata: api.Metadata{Name: "test-mcp"},
				Spec:     apiData,
			},
			DesiredState: models.StateDeployed,
			Origin:       models.OriginGatewayAPI,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		affected, err := service.saveOrUpdateConfig(storedCfg, logger)
		assert.NoError(t, err)
		assert.True(t, affected)

		// Verify config was added to the DB
		retrieved, err := service.db.GetConfig(storedCfg.UUID)
		assert.NoError(t, err)
		assert.Equal(t, storedCfg.UUID, retrieved.UUID)
	})
}

func TestMCPDeploymentService_SaveOrUpdateConfig_UpdatesExisting(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Updates existing config via upsert", func(t *testing.T) {
		store := storage.NewConfigStore()
		db := newTestSQLiteStorage(t, logger)
		service := newTestMCPDeploymentService(store, db, nil, nil, nil)

		now := time.Now()
		apiData := api.APIConfigData{
			DisplayName: "Original MCP",
			Version:     "1.0.0",
			Context:     "/original",
		}

		// Add original config
		original := &models.StoredConfig{
			UUID:        "0000-config-to-update-0000-000000000000",
			Kind:        string(api.MCPProxyConfigurationKindMcp),
			Handle:      "original-mcp",
			DisplayName: "Original MCP",
			Version:     "1.0.0",
			Configuration: api.RestAPI{
				Kind:     api.RestAPIKindRestApi,
				Metadata: api.Metadata{Name: "original-mcp"},
				Spec:     apiData,
			},
			DesiredState: models.StateDeployed,
			Origin:       models.OriginGatewayAPI,
			CreatedAt:    now,
			UpdatedAt:    now,
			DeployedAt:   &now,
		}
		require.NoError(t, db.SaveConfig(original))
		require.NoError(t, store.Add(original))

		// Create updated config with newer timestamp
		later := now.Add(time.Second)
		newApiData := api.APIConfigData{
			DisplayName: "Original MCP",
			Version:     "1.0.0",
			Context:     "/updated",
		}

		newConfig := &models.StoredConfig{
			UUID:        "0000-config-to-update-0000-000000000000",
			Kind:        string(api.MCPProxyConfigurationKindMcp),
			Handle:      "original-mcp",
			DisplayName: "Original MCP",
			Version:     "1.0.0",
			Configuration: api.RestAPI{
				Kind:     api.RestAPIKindRestApi,
				Metadata: api.Metadata{Name: "original-mcp"},
				Spec:     newApiData,
			},
			DesiredState: models.StateDeployed,
			Origin:       models.OriginGatewayAPI,
			DeployedAt:   &later,
		}

		affected, err := service.saveOrUpdateConfig(newConfig, logger)
		assert.NoError(t, err)
		assert.True(t, affected)
	})

	t.Run("Creates new config when not found", func(t *testing.T) {
		store := storage.NewConfigStore()
		db := newTestSQLiteStorage(t, logger)
		service := newTestMCPDeploymentService(store, db, nil, nil, nil)

		now := time.Now()
		apiData := api.APIConfigData{
			DisplayName: "New MCP",
			Version:     "1.0.0",
			Context:     "/new",
		}

		newConfig := &models.StoredConfig{
			UUID:        "0000-new-config-0000-000000000000",
			Kind:        string(api.MCPProxyConfigurationKindMcp),
			Handle:      "new-mcp",
			DisplayName: "New MCP",
			Version:     "1.0.0",
			Configuration: api.RestAPI{
				Kind:     api.RestAPIKindRestApi,
				Metadata: api.Metadata{Name: "new-mcp"},
				Spec:     apiData,
			},
			DesiredState: models.StateDeployed,
			Origin:       models.OriginGatewayAPI,
			DeployedAt:   &now,
		}

		affected, err := service.saveOrUpdateConfig(newConfig, logger)
		assert.NoError(t, err)
		assert.True(t, affected)

		// Verify config was created in the DB
		retrieved, err := service.db.GetConfig(newConfig.UUID)
		assert.NoError(t, err)
		assert.Equal(t, newConfig.UUID, retrieved.UUID)
	})
}

func TestMCPDeploymentService_ParseValidateAndTransform(t *testing.T) {
	store := storage.NewConfigStore()
	service := newTestMCPDeploymentService(store, nil, nil, nil, nil)
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

func TestMCPDeploymentService_CreateMCPProxy_WithDBAndEventHubPublishesCreate(t *testing.T) {
	store := storage.NewConfigStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db := newTestSQLiteStorage(t, logger)
	mockHub := &mockLLMEventHub{}
	service := newTestMCPDeploymentServiceWithHub(store, db, nil, nil, nil, mockHub, "test-gateway")

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
	result, err := service.CreateMCPProxy(MCPDeploymentParams{
		Data:          []byte(yamlData),
		ContentType:   "application/yaml",
		Origin:        models.OriginGatewayAPI,
		CorrelationID: "corr-create-mcp",
		Logger:        logger,
	})
	require.NoError(t, err)
	created := result.StoredConfig

	storedInDB, err := db.GetConfig(created.UUID)
	require.NoError(t, err)
	assert.Equal(t, string(api.MCPProxyConfigurationKindMcp), storedInDB.Kind)

	_, err = store.Get(created.UUID)
	require.ErrorIs(t, err, storage.ErrNotFound)

	require.Len(t, mockHub.publishedEvents, 1)
	assert.Equal(t, "test-gateway", mockHub.publishedEvents[0].gatewayID)
	assert.Equal(t, "CREATE", mockHub.publishedEvents[0].event.Action)
	assert.Equal(t, eventhub.EventTypeMCPProxy, mockHub.publishedEvents[0].event.EventType)
	assert.Equal(t, created.UUID, mockHub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-create-mcp", mockHub.publishedEvents[0].event.EventID)
	assert.Equal(t, eventhub.EmptyEventData, mockHub.publishedEvents[0].event.EventData)
}

func TestMCPDeploymentService_UndeployMCPProxy_WithDBAndEventHubPublishesUpdate(t *testing.T) {
	store := storage.NewConfigStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db := newTestSQLiteStorage(t, logger)
	mockHub := &mockLLMEventHub{}
	service := newTestMCPDeploymentServiceWithHub(store, db, nil, nil, nil, mockHub, "test-gateway")
	upstreamURL := "http://localhost:8080"

	cfg := &models.StoredConfig{
		UUID:         "0000-mcp-undeploy-id-0000-000000000000",
		Kind:         string(api.MCPProxyConfigurationKindMcp),
		Handle:       "test-mcp",
		DisplayName:  "Test MCP",
		Version:      "1.0.0",
		DeploymentID: "rev-1",
		Origin:       models.OriginControlPlane,
		SourceConfiguration: api.MCPProxyConfiguration{
			ApiVersion: api.MCPProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.MCPProxyConfigurationKindMcp,
			Metadata:   api.Metadata{Name: "test-mcp"},
			Spec: api.MCPProxyConfigData{
				DisplayName: "Test MCP",
				Version:     "1.0.0",
				Context:     stringPtr("/mcp"),
				Upstream: api.MCPProxyConfigData_Upstream{
					Url: &upstreamURL,
				},
			},
		},
		DesiredState: models.StateDeployed,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	require.NoError(t, HydrateStoredMCPConfig(cfg))
	require.NoError(t, db.SaveConfig(cfg))
	require.NoError(t, store.Add(cfg))

	performedAt := time.Unix(1700000000, 0).UTC()
	updated, err := service.UndeployMCPProxy(cfg.UUID, "rev-1", &performedAt, "corr-mcp-undeploy", logger)
	require.NoError(t, err)
	assert.Equal(t, models.StateUndeployed, updated.DesiredState)
	assert.Equal(t, "rev-1", updated.DeploymentID)
	require.NotNil(t, updated.DeployedAt)
	assert.True(t, updated.DeployedAt.Equal(performedAt))

	storedInDB, err := db.GetConfig(cfg.UUID)
	require.NoError(t, err)
	assert.Equal(t, models.StateUndeployed, storedInDB.DesiredState)
	assert.Equal(t, "rev-1", storedInDB.DeploymentID)
	require.NotNil(t, storedInDB.DeployedAt)
	assert.True(t, storedInDB.DeployedAt.Equal(performedAt))

	storedInMemory, err := store.Get(cfg.UUID)
	require.NoError(t, err)
	assert.Equal(t, models.StateDeployed, storedInMemory.DesiredState)

	require.Len(t, mockHub.publishedEvents, 1)
	assert.Equal(t, "UPDATE", mockHub.publishedEvents[0].event.Action)
	assert.Equal(t, eventhub.EventTypeMCPProxy, mockHub.publishedEvents[0].event.EventType)
	assert.Equal(t, cfg.UUID, mockHub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-mcp-undeploy", mockHub.publishedEvents[0].event.EventID)
}
