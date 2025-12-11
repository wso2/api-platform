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
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"go.uber.org/zap"
)

// setupTestDB creates a temporary SQLite database for testing
func setupTestDB(t *testing.T) (storage.Storage, string, func()) {
	t.Helper()

	// Create temporary directory
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create logger
	logger, err := zap.NewDevelopment()
	require.NoError(t, err, "Failed to create logger")

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
		Name:    name,
		Version: version,
		Context: "/" + name,
		Upstreams: []api.Upstream{
			{Url: "http://example.com"},
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
			Version: api.ApiPlatformWso2Comv1,
			Kind:    api.Httprest,
			Spec:    specUnion,
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
		assert.Equal(t, cfg.GetName(), retrieved.GetName())
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
		assert.Equal(t, "TestAPI3", retrieved.GetName())
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

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

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
