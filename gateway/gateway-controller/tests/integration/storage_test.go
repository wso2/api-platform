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

package integration

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// setupTestDB creates a temporary SQLite database for testing
func setupTestDB(t *testing.T) (storage.Storage, string, func()) {
	t.Helper()

	// Initialize metrics for tests (disabled by default)
	metrics.SetEnabled(false)
	metrics.Init()

	// Create temporary directory
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create logger
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Create SQLite storage
	db, err := storage.NewSQLiteStorage(dbPath, logger)
	require.NoError(t, err, "Failed to create SQLite storage")

	// Cleanup function
	cleanup := func() {
		db.Close()
	}

	return db, dbPath, cleanup
}

// createTestConfig creates a sample API configuration for testing
func createTestConfig(name, version string) *models.StoredConfig {
	specUnion := api.APIConfiguration_Spec{}
	specUnion.FromAPIConfigData(api.APIConfigData{
		DisplayName: name,
		Version:     version,
		Context:     "/" + name,
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{
				Url: func() *string { s := "http://example.com"; return &s }(),
			},
		},
		Operations: []api.Operation{
			{
				Method: api.OperationMethod(api.GET),
				Path:   "/test",
			},
		},
	})
	return &models.StoredConfig{
		ID: uuid.New().String(),
		Configuration: api.APIConfiguration{
			ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.RestApi,
			Metadata:   api.Metadata{Name: name + "-" + version},
			Spec:       specUnion,
		},
		Status:          models.StatusPending,
		DeployedVersion: 0,
	}
}

func TestSQLiteStorage_CRUD(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	// Test SaveConfig
	t.Run("SaveConfig", func(t *testing.T) {
		cfg := createTestConfig("TestAPI", "v1.0")
		err := db.SaveConfig(cfg)
		assert.NoError(t, err, "SaveConfig should succeed")
	})

	// Test GetConfig
	t.Run("GetConfig", func(t *testing.T) {
		cfg := createTestConfig("TestAPI2", "v1.0")
		err := db.SaveConfig(cfg)
		require.NoError(t, err)

		retrieved, err := db.GetConfig(cfg.ID)
		assert.NoError(t, err, "GetConfig should succeed")
		assert.Equal(t, cfg.ID, retrieved.ID)
		assert.Equal(t, cfg.GetDisplayName(), retrieved.GetDisplayName())
		assert.Equal(t, cfg.GetVersion(), retrieved.GetVersion())
	})

	// Test GetConfigByNameVersion
	t.Run("GetConfigByNameVersion", func(t *testing.T) {
		cfg := createTestConfig("TestAPI3", "v1.0")
		err := db.SaveConfig(cfg)
		require.NoError(t, err)

		retrieved, err := db.GetConfigByNameVersion("TestAPI3", "v1.0")
		assert.NoError(t, err, "GetConfigByNameVersion should succeed")
		assert.Equal(t, cfg.ID, retrieved.ID)
		assert.Equal(t, "TestAPI3", retrieved.GetDisplayName())
		assert.Equal(t, "v1.0", retrieved.GetVersion())
	})

	// Test UpdateConfig
	t.Run("UpdateConfig", func(t *testing.T) {
		cfg := createTestConfig("TestAPI4", "v1.0")
		err := db.SaveConfig(cfg)
		require.NoError(t, err)

		// Update the configuration
		cfg.Status = "deployed"
		cfg.DeployedVersion = 1
		err = db.UpdateConfig(cfg)
		assert.NoError(t, err, "UpdateConfig should succeed")

		// Verify update
		retrieved, err := db.GetConfig(cfg.ID)
		require.NoError(t, err)
		assert.Equal(t, models.StatusDeployed, retrieved.Status)
		assert.Equal(t, int64(1), retrieved.DeployedVersion)
	})

	// Test DeleteConfig
	t.Run("DeleteConfig", func(t *testing.T) {
		cfg := createTestConfig("TestAPI5", "v1.0")
		err := db.SaveConfig(cfg)
		require.NoError(t, err)

		// Delete the configuration
		err = db.DeleteConfig(cfg.ID)
		assert.NoError(t, err, "DeleteConfig should succeed")

		// Verify deletion
		_, err = db.GetConfig(cfg.ID)
		assert.Error(t, err, "GetConfig should fail after deletion")
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})

	// Test GetAllConfigs
	t.Run("GetAllConfigs", func(t *testing.T) {
		// Clear database first by creating a fresh one
		db2, _, cleanup2 := setupTestDB(t)
		defer cleanup2()

		// Create multiple configs
		cfg1 := createTestConfig("API1", "v1.0")
		cfg2 := createTestConfig("API2", "v1.0")
		cfg3 := createTestConfig("API3", "v1.0")

		require.NoError(t, db2.SaveConfig(cfg1))
		require.NoError(t, db2.SaveConfig(cfg2))
		require.NoError(t, db2.SaveConfig(cfg3))

		// Retrieve all
		configs, err := db2.GetAllConfigs()
		assert.NoError(t, err, "GetAllConfigs should succeed")
		assert.Len(t, configs, 3, "Should have 3 configurations")
	})
}

func TestSQLiteStorage_ErrorHandling(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	// Test duplicate save (UNIQUE constraint)
	t.Run("DuplicateConfig", func(t *testing.T) {
		cfg := createTestConfig("DupeAPI", "v1.0")
		err := db.SaveConfig(cfg)
		require.NoError(t, err)

		// Try to save again with same name/version
		cfg2 := createTestConfig("DupeAPI", "v1.0")
		err = db.SaveConfig(cfg2)
		assert.Error(t, err, "SaveConfig should fail for duplicate name/version")
		assert.ErrorIs(t, err, storage.ErrConflict)
	})

	// Test update non-existent config
	t.Run("UpdateNonExistent", func(t *testing.T) {
		cfg := createTestConfig("NonExistent", "v1.0")
		err := db.UpdateConfig(cfg)
		assert.Error(t, err, "UpdateConfig should fail for non-existent config")
	})

	// Test delete non-existent config
	t.Run("DeleteNonExistent", func(t *testing.T) {
		err := db.DeleteConfig("non-existent-id")
		assert.Error(t, err, "DeleteConfig should fail for non-existent config")
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})

	// Test get non-existent config
	t.Run("GetNonExistent", func(t *testing.T) {
		_, err := db.GetConfig("non-existent-id")
		assert.Error(t, err, "GetConfig should fail for non-existent config")
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})

	// Test get by name/version non-existent
	t.Run("GetByNameVersionNonExistent", func(t *testing.T) {
		_, err := db.GetConfigByNameVersion("NonExistent", "v1.0")
		assert.Error(t, err, "GetConfigByNameVersion should fail for non-existent config")
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})
}

func TestSQLiteStorage_Close(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	err := db.Close()
	assert.NoError(t, err, "Close should succeed")
}

func TestSQLiteStorage_LoadFromDatabase(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	// Create and save test configurations
	cfg1 := createTestConfig("LoadAPI1", "v1.0")
	cfg2 := createTestConfig("LoadAPI2", "v1.0")

	require.NoError(t, db.SaveConfig(cfg1))
	require.NoError(t, db.SaveConfig(cfg2))

	// Create a new ConfigStore
	configStore := storage.NewConfigStore()

	// Load from database
	err := storage.LoadFromDatabase(db, configStore)
	assert.NoError(t, err, "LoadFromDatabase should succeed")

	// Verify loaded configs
	allConfigs := configStore.GetAll()
	assert.Len(t, allConfigs, 2, "Should have loaded 2 configurations")
}

func TestSQLiteStorage_DatabaseFileCreation(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Create storage (should create database files)
	db, err := storage.NewSQLiteStorage(dbPath, logger)
	require.NoError(t, err)
	defer db.Close()

	// Verify database file exists
	_, err = os.Stat(dbPath)
	assert.NoError(t, err, "Database file should exist")

	// WAL mode files may or may not exist immediately, but check they can be created
	// after a write operation
	cfg := createTestConfig("TestAPI", "v1.0")
	err = db.SaveConfig(cfg)
	require.NoError(t, err)

	// After a write, WAL files should exist
	walPath := dbPath + "-wal"
	shmPath := dbPath + "-shm"

	// Note: WAL files may be checkpointed automatically, so we don't require them
	// but if they exist, that's expected
	_, walErr := os.Stat(walPath)
	_, shmErr := os.Stat(shmPath)

	if walErr == nil {
		t.Logf("WAL file exists: %s", walPath)
	}
	if shmErr == nil {
		t.Logf("SHM file exists: %s", shmPath)
	}
}

// createTestConfigWithLabels creates a sample API configuration with labels for testing
func createTestConfigWithLabels(name, version string, labels map[string]string) *models.StoredConfig {
	specUnion := api.APIConfiguration_Spec{}
	specUnion.FromAPIConfigData(api.APIConfigData{
		DisplayName: name,
		Version:     version,
		Context:     "/" + name,
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{
				Url: func() *string { s := "http://example.com"; return &s }(),
			},
		},
		Operations: []api.Operation{
			{
				Method: api.OperationMethod(api.GET),
				Path:   "/test",
			},
		},
	})

	// Handle labels properly: nil map should result in nil pointer
	var labelsPtr *map[string]string
	if labels != nil {
		labelsPtr = &labels
	}

	return &models.StoredConfig{
		ID: uuid.New().String(),
		Configuration: api.APIConfiguration{
			ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.RestApi,
			Metadata: api.Metadata{
				Name:   name + "-" + version,
				Labels: labelsPtr,
			},
			Spec: specUnion,
		},
		Status:          models.StatusPending,
		DeployedVersion: 0,
	}
}

func TestConfigStore_LabelsStorage(t *testing.T) {
	configStore := storage.NewConfigStore()

	t.Run("StoreLabels", func(t *testing.T) {
		labels := map[string]string{
			"environment": "production",
			"team":        "backend",
			"version":     "v1",
		}

		// Add a config which includes labels (labels are stored via Add)
		cfg := createTestConfigWithLabels("test-api", "v1.0", labels)
		err := configStore.Add(cfg)
		assert.NoError(t, err, "Add should succeed")

		// Verify labels were stored
		handle := cfg.GetHandle()
		retrieved, err := configStore.GetLabelsMap(handle)
		assert.NoError(t, err, "GetLabelsMap should succeed")
		assert.Equal(t, labels, retrieved, "Retrieved labels should match stored labels")
	})

	t.Run("GetLabelsMap_NotFound", func(t *testing.T) {
		_, err := configStore.GetLabelsMap("non-existent-handle")
		assert.Error(t, err, "GetLabelsMap should fail for non-existent handle")
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})

	t.Run("GetLabelsMap_ReturnsCopy", func(t *testing.T) {
		labels := map[string]string{"key": "value"}
		cfg := createTestConfigWithLabels("test-api-v2.0", "v1.0", labels)
		require.NoError(t, configStore.Add(cfg))

		retrieved, err := configStore.GetLabelsMap(cfg.GetHandle())
		require.NoError(t, err)

		// Modify the retrieved map
		retrieved["new-key"] = "new-value"

		// Verify original wasn't modified
		retrieved2, err := configStore.GetLabelsMap(cfg.GetHandle())
		require.NoError(t, err)
		assert.NotEqual(t, retrieved, retrieved2, "Original labels should not be modified")
		assert.Equal(t, labels, retrieved2, "Original labels should remain unchanged")
	})

	t.Run("DeleteLabels", func(t *testing.T) {
		labels := map[string]string{"key": "value"}
		cfg := createTestConfigWithLabels("test-api-v3.0", "v1.0", labels)
		require.NoError(t, configStore.Add(cfg))

		// Remove labels by updating the config with nil labels
		cfg.Configuration.Metadata.Labels = nil
		err := configStore.Update(cfg)
		assert.NoError(t, err, "Update (remove labels) should succeed")

		// Verify labels were deleted
		_, err = configStore.GetLabelsMap(cfg.GetHandle())
		assert.Error(t, err, "GetLabelsMap should fail after deletion")
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})

	t.Run("DeleteLabels_NotFound", func(t *testing.T) {
		// Attempting to update a non-existent config should fail
		cfg := createTestConfigWithLabels("non-existent-handle", "v1.0", nil)
		err := configStore.Update(cfg)
		assert.Error(t, err, "Update should fail for non-existent config")
	})

	t.Run("StoreLabels_NilLabels", func(t *testing.T) {
		labels := map[string]string{"key": "value"}
		cfg := createTestConfigWithLabels("test-api-v4.0", "v1.0", labels)
		require.NoError(t, configStore.Add(cfg))

		// Store nil labels via Update (should remove them)
		cfg.Configuration.Metadata.Labels = nil
		err := configStore.Update(cfg)
		assert.NoError(t, err, "Update (nil labels) should succeed")

		// Verify labels were removed
		_, err = configStore.GetLabelsMap(cfg.GetHandle())
		assert.Error(t, err, "GetLabelsMap should fail after storing nil labels")
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})

	t.Run("StoreLabels_EmptyMap", func(t *testing.T) {
		emptyLabels := map[string]string{}
		cfg := createTestConfigWithLabels("test-api-v5.0", "v1.0", emptyLabels)
		require.NoError(t, configStore.Add(cfg))

		retrieved, err := configStore.GetLabelsMap(cfg.GetHandle())
		assert.NoError(t, err, "GetLabelsMap should succeed")
		assert.Equal(t, emptyLabels, retrieved, "Retrieved labels should be empty map")
	})
}

func TestConfigStore_LabelsWithAddUpdateDelete(t *testing.T) {
	configStore := storage.NewConfigStore()

	t.Run("Add stores labels automatically", func(t *testing.T) {
		labels := map[string]string{
			"environment": "production",
			"team":        "backend",
		}
		cfg := createTestConfigWithLabels("LabelTestAPI1", "v1.0", labels)

		err := configStore.Add(cfg)
		require.NoError(t, err, "Add should succeed")

		// Verify labels were stored
		handle := cfg.GetHandle()
		retrieved, err := configStore.GetLabelsMap(handle)
		assert.NoError(t, err, "GetLabelsMap should succeed")
		assert.Equal(t, labels, retrieved, "Labels should be stored automatically on Add")
	})

	t.Run("Add with nil labels", func(t *testing.T) {
		cfg := createTestConfigWithLabels("LabelTestAPI2", "v1.0", nil)

		err := configStore.Add(cfg)
		require.NoError(t, err, "Add should succeed with nil labels")

		// Verify no labels were stored
		handle := cfg.GetHandle()
		_, err = configStore.GetLabelsMap(handle)
		assert.Error(t, err, "GetLabelsMap should fail when labels are nil")
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})

	t.Run("Update updates labels", func(t *testing.T) {
		initialLabels := map[string]string{"key1": "value1"}
		cfg := createTestConfigWithLabels("LabelTestAPI3", "v1.0", initialLabels)

		err := configStore.Add(cfg)
		require.NoError(t, err)

		// Update with new labels
		updatedLabels := map[string]string{
			"key1": "updated-value1",
			"key2": "value2",
		}
		cfg.Configuration.Metadata.Labels = &updatedLabels

		err = configStore.Update(cfg)
		require.NoError(t, err, "Update should succeed")

		// Verify labels were updated
		handle := cfg.GetHandle()
		retrieved, err := configStore.GetLabelsMap(handle)
		assert.NoError(t, err, "GetLabelsMap should succeed")
		assert.Equal(t, updatedLabels, retrieved, "Labels should be updated")
	})

	t.Run("Update removes labels when set to nil", func(t *testing.T) {
		initialLabels := map[string]string{"key1": "value1"}
		cfg := createTestConfigWithLabels("LabelTestAPI4", "v1.0", initialLabels)

		err := configStore.Add(cfg)
		require.NoError(t, err)

		// Update with nil labels
		cfg.Configuration.Metadata.Labels = nil

		err = configStore.Update(cfg)
		require.NoError(t, err, "Update should succeed")

		// Verify labels were removed
		handle := cfg.GetHandle()
		_, err = configStore.GetLabelsMap(handle)
		assert.Error(t, err, "GetLabelsMap should fail after setting labels to nil")
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})

	t.Run("Update handles handle change", func(t *testing.T) {
		labels := map[string]string{"key": "value"}
		cfg := createTestConfigWithLabels("LabelTestAPI5", "v1.0", labels)

		err := configStore.Add(cfg)
		require.NoError(t, err)

		oldHandle := cfg.GetHandle()

		// Create a new config object with updated handle (don't modify the original)
		updatedCfg := &models.StoredConfig{
			ID: cfg.ID, // Same ID
			Configuration: api.APIConfiguration{
				ApiVersion: cfg.Configuration.ApiVersion,
				Kind:       cfg.Configuration.Kind,
				Metadata: api.Metadata{
					Name:   "new-handle-v1.0", // New name
					Labels: cfg.Configuration.Metadata.Labels,
				},
				Spec: cfg.Configuration.Spec,
			},
			Status:          cfg.Status,
			DeployedVersion: cfg.DeployedVersion,
		}
		newHandle := updatedCfg.GetHandle()

		err = configStore.Update(updatedCfg)
		require.NoError(t, err, "Update should succeed")

		// Verify labels moved to new handle
		_, err = configStore.GetLabelsMap(oldHandle)
		assert.Error(t, err, "Old handle should not have labels")

		retrieved, err := configStore.GetLabelsMap(newHandle)
		assert.NoError(t, err, "New handle should have labels")
		assert.Equal(t, labels, retrieved, "Labels should be at new handle")
	})

	t.Run("Delete removes labels", func(t *testing.T) {
		labels := map[string]string{"key": "value"}
		cfg := createTestConfigWithLabels("LabelTestAPI6", "v1.0", labels)

		err := configStore.Add(cfg)
		require.NoError(t, err)

		handle := cfg.GetHandle()

		// Verify labels exist
		_, err = configStore.GetLabelsMap(handle)
		assert.NoError(t, err, "Labels should exist before deletion")

		// Delete the config
		err = configStore.Delete(cfg.ID)
		require.NoError(t, err, "Delete should succeed")

		// Verify labels were removed
		_, err = configStore.GetLabelsMap(handle)
		assert.Error(t, err, "GetLabelsMap should fail after Delete")
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})
}

func TestConfigStore_LabelsWithAllAPITypes(t *testing.T) {
	configStore := storage.NewConfigStore()

	labels := map[string]string{
		"environment": "production",
		"team":        "backend",
	}

	t.Run("RestApi with labels", func(t *testing.T) {
		cfg := createTestConfigWithLabels("RestAPILabel", "v1.0", labels)
		cfg.Configuration.Kind = api.RestApi

		err := configStore.Add(cfg)
		require.NoError(t, err)

		handle := cfg.GetHandle()
		retrieved, err := configStore.GetLabelsMap(handle)
		assert.NoError(t, err)
		assert.Equal(t, labels, retrieved)
	})

	t.Run("WebSubApi with labels", func(t *testing.T) {
		specUnion := api.APIConfiguration_Spec{}
		specUnion.FromWebhookAPIData(api.WebhookAPIData{
			DisplayName: "AsyncAPILabel",
			Version:     "v1.0",
			Context:     "/async",
			Channels: []api.Channel{
				{
					Name:   "/events",
					Method: api.SUB,
				},
			},
		})

		cfg := &models.StoredConfig{
			ID: uuid.New().String(),
			Configuration: api.APIConfiguration{
				ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
				Kind:       api.WebSubApi,
				Metadata: api.Metadata{
					Name:   "async-api-v1.0",
					Labels: &labels,
				},
				Spec: specUnion,
			},
			Status:          models.StatusPending,
			DeployedVersion: 0,
		}

		err := configStore.Add(cfg)
		require.NoError(t, err)

		handle := cfg.GetHandle()
		retrieved, err := configStore.GetLabelsMap(handle)
		assert.NoError(t, err)
		assert.Equal(t, labels, retrieved)
	})
}

func TestSQLiteStorage_LabelsPersistence(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	t.Run("SaveConfig with labels persists labels", func(t *testing.T) {
		labels := map[string]string{
			"environment": "production",
			"test":        "db-persistence",
		}
		cfg := createTestConfigWithLabels("PersistAPI1", "v1.0", labels)

		err := db.SaveConfig(cfg)
		require.NoError(t, err)

		// Retrieve and verify labels are persisted
		retrieved, err := db.GetConfig(cfg.ID)
		require.NoError(t, err)
		assert.Equal(t, labels, *retrieved.Configuration.Metadata.Labels, "Labels should be persisted")
	})

	t.Run("UpdateConfig updates labels", func(t *testing.T) {
		initialLabels := map[string]string{"key1": "value1"}
		cfg := createTestConfigWithLabels("PersistAPI2", "v1.0", initialLabels)

		err := db.SaveConfig(cfg)
		require.NoError(t, err)

		// Update labels
		updatedLabels := map[string]string{
			"key1": "updated-value1",
			"key2": "value2",
		}
		cfg.Configuration.Metadata.Labels = &updatedLabels

		err = db.UpdateConfig(cfg)
		require.NoError(t, err)

		// Verify labels were updated
		retrieved, err := db.GetConfig(cfg.ID)
		require.NoError(t, err)
		assert.Equal(t, updatedLabels, *retrieved.Configuration.Metadata.Labels, "Labels should be updated")
	})

	t.Run("LoadFromDatabase loads labels into memory", func(t *testing.T) {
		labels := map[string]string{
			"environment": "production",
			"team":        "backend",
		}
		cfg := createTestConfigWithLabels("PersistAPI3", "v1.0", labels)

		err := db.SaveConfig(cfg)
		require.NoError(t, err)

		// Create a new ConfigStore and load from database
		configStore := storage.NewConfigStore()
		err = storage.LoadFromDatabase(db, configStore)
		require.NoError(t, err)

		// Verify labels are loaded into memory
		handle := cfg.GetHandle()
		retrieved, err := configStore.GetLabelsMap(handle)
		assert.NoError(t, err, "Labels should be loaded into memory")
		assert.Equal(t, labels, retrieved, "Loaded labels should match persisted labels")
	})
}
