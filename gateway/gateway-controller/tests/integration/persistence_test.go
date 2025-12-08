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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"go.uber.org/zap"
)

// TestDatabasePersistenceAcrossRestarts verifies that configurations
// survive database close and reopen (simulating application restart)
func TestDatabasePersistenceAcrossRestarts(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "persistence.db")

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	// Phase 1: Create database and save configurations
	t.Log("Phase 1: Creating database and saving configurations")
	{
		db, err := storage.NewSQLiteStorage(dbPath, logger)
		require.NoError(t, err, "Failed to create database")

		// Save multiple configurations
		cfg1 := createTestConfig("PersistAPI1", "v1.0")
		cfg2 := createTestConfig("PersistAPI2", "v1.0")
		cfg3 := createTestConfig("PersistAPI3", "v2.0")

		require.NoError(t, db.SaveConfig(cfg1))
		require.NoError(t, db.SaveConfig(cfg2))
		require.NoError(t, db.SaveConfig(cfg3))

		// Verify they exist
		configs, err := db.GetAllConfigs()
		require.NoError(t, err)
		require.Len(t, configs, 3)

		// Close database (simulate shutdown)
		require.NoError(t, db.Close())
		t.Log("Database closed successfully")
	}

	// Phase 2: Reopen database and verify data persisted
	t.Log("Phase 2: Reopening database and verifying persistence")
	{
		db, err := storage.NewSQLiteStorage(dbPath, logger)
		require.NoError(t, err, "Failed to reopen database")
		defer db.Close()

		// Verify all configurations are still there
		configs, err := db.GetAllConfigs()
		assert.NoError(t, err, "GetAllConfigs should succeed after restart")
		assert.Len(t, configs, 3, "All 3 configurations should persist")

		// Verify each configuration by name/version
		cfg1, err := db.GetConfigByNameVersion("PersistAPI1", "v1.0")
		assert.NoError(t, err, "Should find PersistAPI1/v1.0")
		assert.NotNil(t, cfg1)

		cfg2, err := db.GetConfigByNameVersion("PersistAPI2", "v1.0")
		assert.NoError(t, err, "Should find PersistAPI2/v1.0")
		assert.NotNil(t, cfg2)

		cfg3, err := db.GetConfigByNameVersion("PersistAPI3", "v2.0")
		assert.NoError(t, err, "Should find PersistAPI3/v2.0")
		assert.NotNil(t, cfg3)
	}

	// Phase 3: Update a configuration and verify it persists
	t.Log("Phase 3: Updating configuration and verifying update persistence")
	{
		db, err := storage.NewSQLiteStorage(dbPath, logger)
		require.NoError(t, err, "Failed to reopen database")

		// Get and update a configuration
		cfg, err := db.GetConfigByNameVersion("PersistAPI1", "v1.0")
		require.NoError(t, err)
		cfg.Status = "deployed"
		cfg.DeployedVersion = 5

		require.NoError(t, db.UpdateConfig(cfg))
		require.NoError(t, db.Close())
		t.Log("Configuration updated and database closed")
	}

	// Phase 4: Reopen and verify update persisted
	t.Log("Phase 4: Verifying update persisted")
	{
		db, err := storage.NewSQLiteStorage(dbPath, logger)
		require.NoError(t, err, "Failed to reopen database")
		defer db.Close()

		cfg, err := db.GetConfigByNameVersion("PersistAPI1", "v1.0")
		assert.NoError(t, err)
		assert.Equal(t, models.StatusDeployed, cfg.Status)
		assert.Equal(t, int64(5), cfg.DeployedVersion)
	}

	// Phase 5: Delete a configuration and verify deletion persists
	t.Log("Phase 5: Deleting configuration and verifying deletion persistence")
	{
		db, err := storage.NewSQLiteStorage(dbPath, logger)
		require.NoError(t, err, "Failed to reopen database")

		// Get the ID of PersistAPI2
		cfg, err := db.GetConfigByNameVersion("PersistAPI2", "v1.0")
		require.NoError(t, err)

		// Delete it
		require.NoError(t, db.DeleteConfig(cfg.ID))
		require.NoError(t, db.Close())
		t.Log("Configuration deleted and database closed")
	}

	// Phase 6: Reopen and verify deletion persisted
	t.Log("Phase 6: Verifying deletion persisted")
	{
		db, err := storage.NewSQLiteStorage(dbPath, logger)
		require.NoError(t, err, "Failed to reopen database")
		defer db.Close()

		// Should only have 2 configurations now
		configs, err := db.GetAllConfigs()
		assert.NoError(t, err)
		assert.Len(t, configs, 2, "Should have 2 configurations after deletion")

		// Verify PersistAPI2 is gone
		_, err = db.GetConfigByNameVersion("PersistAPI2", "v1.0")
		assert.Error(t, err, "PersistAPI2 should not exist")
		assert.ErrorIs(t, err, storage.ErrNotFound)
	}
}

// TestLoadFromDatabaseWithMultipleRestarts tests the LoadFromDatabase function
// across multiple restart cycles
func TestLoadFromDatabaseWithMultipleRestarts(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "load-test.db")

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	// Create and populate database
	db, err := storage.NewSQLiteStorage(dbPath, logger)
	require.NoError(t, err)

	cfg1 := createTestConfig("LoadTest1", "v1.0")
	cfg2 := createTestConfig("LoadTest2", "v1.0")
	cfg3 := createTestConfig("LoadTest3", "v1.0")

	require.NoError(t, db.SaveConfig(cfg1))
	require.NoError(t, db.SaveConfig(cfg2))
	require.NoError(t, db.SaveConfig(cfg3))
	require.NoError(t, db.Close())

	// Simulate 3 restart cycles
	for i := 1; i <= 3; i++ {
		t.Logf("Restart cycle %d", i)

		// Reopen database
		db, err := storage.NewSQLiteStorage(dbPath, logger)
		require.NoError(t, err)

		// Create fresh ConfigStore
		configStore := storage.NewConfigStore()

		// Load from database
		err = storage.LoadFromDatabase(db, configStore)
		assert.NoError(t, err, "LoadFromDatabase should succeed")

		// Verify all configs loaded
		allConfigs := configStore.GetAll()
		assert.Len(t, allConfigs, 3, "Should load all 3 configurations")

		// Verify configs are correct
		for _, cfg := range allConfigs {
			assert.NotEmpty(t, cfg.ID)
			assert.NotEmpty(t, cfg.GetName())
			assert.NotEmpty(t, cfg.GetVersion())
		}

		require.NoError(t, db.Close())
	}
}

// TestZeroDataLoss verifies that configurations survive restarts with zero data loss
// This is the success criterion SC-003 from the specification
func TestZeroDataLoss(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "zero-loss.db")

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	// Create 10 configurations
	t.Log("Creating 10 configurations")
	db, err := storage.NewSQLiteStorage(dbPath, logger)
	require.NoError(t, err)

	for i := 1; i <= 10; i++ {
		cfg := createTestConfig("ZeroLossAPI"+string(rune(i+'0')), "v1.0")
		require.NoError(t, db.SaveConfig(cfg))
	}

	// Get all configs before restart
	configsBefore, err := db.GetAllConfigs()
	require.NoError(t, err)
	require.Len(t, configsBefore, 10)

	require.NoError(t, db.Close())

	// Restart and verify zero data loss
	t.Log("Restarting and verifying zero data loss")
	db, err = storage.NewSQLiteStorage(dbPath, logger)
	require.NoError(t, err)
	defer db.Close()

	configsAfter, err := db.GetAllConfigs()
	assert.NoError(t, err)
	assert.Len(t, configsAfter, 10, "Zero data loss: all 10 configurations should persist")

	// Verify each configuration matches (by ID)
	beforeMap := make(map[string]*models.StoredConfig)
	for _, cfg := range configsBefore {
		beforeMap[cfg.ID] = cfg
	}

	for _, cfgAfter := range configsAfter {
		cfgBefore, exists := beforeMap[cfgAfter.ID]
		assert.True(t, exists, "Configuration should exist in both before and after")
		assert.Equal(t, cfgBefore.GetName(), cfgAfter.GetName())
		assert.Equal(t, cfgBefore.GetVersion(), cfgAfter.GetVersion())
		assert.Equal(t, cfgBefore.Status, cfgAfter.Status)
	}

	t.Log("Zero data loss verified successfully")
}
