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
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"go.uber.org/zap"
)

// TestConcurrentWrites tests that SQLite can handle concurrent write operations
// without errors or data corruption (Success Criterion SC-009)
func TestConcurrentWrites(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "concurrent.db")

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	db, err := storage.NewSQLiteStorage(dbPath, logger)
	require.NoError(t, err)
	defer db.Close()

	const numGoroutines = 10
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	t.Logf("Starting %d concurrent write operations", numGoroutines)

	// Launch 10 concurrent goroutines to save configurations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			cfg := createTestConfig(fmt.Sprintf("ConcurrentAPI%d", id), "v1.0")
			if err := db.SaveConfig(cfg); err != nil {
				errors <- fmt.Errorf("goroutine %d failed to save config: %w", id, err)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errors)

	// Check for errors
	var errorList []error
	for err := range errors {
		errorList = append(errorList, err)
	}

	assert.Empty(t, errorList, "No errors should occur during concurrent writes")

	// Verify all configurations were saved
	configs, err := db.GetAllConfigs()
	assert.NoError(t, err)
	assert.Len(t, configs, numGoroutines, fmt.Sprintf("All %d configurations should be saved", numGoroutines))
}

// TestConcurrentReads tests concurrent read operations
func TestConcurrentReads(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "concurrent-reads.db")

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	db, err := storage.NewSQLiteStorage(dbPath, logger)
	require.NoError(t, err)
	defer db.Close()

	// Create a test configuration
	cfg := createTestConfig("ReadTestAPI", "v1.0")
	require.NoError(t, db.SaveConfig(cfg))

	const numGoroutines = 20
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	t.Logf("Starting %d concurrent read operations", numGoroutines)

	// Launch concurrent goroutines to read the same configuration
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Read by ID
			_, err := db.GetConfig(cfg.ID)
			if err != nil {
				errors <- fmt.Errorf("goroutine %d failed to get config by ID: %w", id, err)
				return
			}

			// Read by name/version
			_, err = db.GetConfigByNameVersion("ReadTestAPI", "v1.0")
			if err != nil {
				errors <- fmt.Errorf("goroutine %d failed to get config by name/version: %w", id, err)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errors)

	// Check for errors
	var errorList []error
	for err := range errors {
		errorList = append(errorList, err)
	}

	assert.Empty(t, errorList, "No errors should occur during concurrent reads")
}

// TestConcurrentMixedOperations tests concurrent mix of reads, writes, updates, and deletes
func TestConcurrentMixedOperations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "mixed-ops.db")

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	db, err := storage.NewSQLiteStorage(dbPath, logger)
	require.NoError(t, err)
	defer db.Close()

	// Pre-populate with some configurations
	for i := 0; i < 5; i++ {
		cfg := createTestConfig(fmt.Sprintf("MixedAPI%d", i), "v1.0")
		require.NoError(t, db.SaveConfig(cfg))
	}

	const numGoroutines = 15
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	t.Logf("Starting %d concurrent mixed operations", numGoroutines)

	// Writers (5 goroutines)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			cfg := createTestConfig(fmt.Sprintf("NewAPI%d", id), "v1.0")
			if err := db.SaveConfig(cfg); err != nil {
				errors <- fmt.Errorf("writer %d failed: %w", id, err)
			}
		}(i)
	}

	// Readers (5 goroutines)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			_, err := db.GetConfigByNameVersion(fmt.Sprintf("MixedAPI%d", id), "v1.0")
			if err != nil {
				errors <- fmt.Errorf("reader %d failed: %w", id, err)
			}
		}(i)
	}

	// Updaters (5 goroutines)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			cfg, err := db.GetConfigByNameVersion(fmt.Sprintf("MixedAPI%d", id), "v1.0")
			if err != nil {
				errors <- fmt.Errorf("updater %d failed to get config: %w", id, err)
				return
			}

			cfg.Status = models.StatusDeployed
			cfg.DeployedVersion = int64(id)
			if err := db.UpdateConfig(cfg); err != nil {
				errors <- fmt.Errorf("updater %d failed to update: %w", id, err)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errors)

	// Check for errors
	var errorList []error
	for err := range errors {
		errorList = append(errorList, err)
	}

	assert.Empty(t, errorList, "No errors should occur during concurrent mixed operations")

	// Verify database integrity
	configs, err := db.GetAllConfigs()
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(configs), 5, "At least 5 configurations should exist")
}

// TestConcurrentUpdatesOnSameConfig tests multiple goroutines updating the same configuration
func TestConcurrentUpdatesOnSameConfig(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "same-config.db")

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	db, err := storage.NewSQLiteStorage(dbPath, logger)
	require.NoError(t, err)
	defer db.Close()

	// Create a single configuration
	cfg := createTestConfig("SharedAPI", "v1.0")
	require.NoError(t, db.SaveConfig(cfg))

	const numGoroutines = 10
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	t.Logf("Starting %d concurrent updates on the same configuration", numGoroutines)

	// Launch concurrent goroutines to update the same configuration
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Get the configuration
			cfg, err := db.GetConfig(cfg.ID)
			if err != nil {
				errors <- fmt.Errorf("goroutine %d failed to get config: %w", id, err)
				return
			}

			// Update it
			cfg.DeployedVersion = int64(id)
			if err := db.UpdateConfig(cfg); err != nil {
				errors <- fmt.Errorf("goroutine %d failed to update config: %w", id, err)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errors)

	// Check for errors
	var errorList []error
	for err := range errors {
		errorList = append(errorList, err)
	}

	assert.Empty(t, errorList, "No errors should occur during concurrent updates")

	// Verify the configuration still exists and is valid
	finalCfg, err := db.GetConfig(cfg.ID)
	assert.NoError(t, err)
	assert.NotNil(t, finalCfg)
	assert.Equal(t, "SharedAPI", finalCfg.GetDisplayName())

	// The final DeployedVersion will be whichever goroutine completed last
	t.Logf("Final deployed version: %d", finalCfg.DeployedVersion)
}

// TestConcurrentGetAllConfigs tests concurrent GetAllConfigs calls
func TestConcurrentGetAllConfigs(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "get-all.db")

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	db, err := storage.NewSQLiteStorage(dbPath, logger)
	require.NoError(t, err)
	defer db.Close()

	// Pre-populate with configurations
	for i := 0; i < 20; i++ {
		cfg := createTestConfig(fmt.Sprintf("GetAllAPI%d", i), "v1.0")
		require.NoError(t, db.SaveConfig(cfg))
	}

	const numGoroutines = 10
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	t.Logf("Starting %d concurrent GetAllConfigs calls", numGoroutines)

	// Launch concurrent GetAllConfigs calls
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			configs, err := db.GetAllConfigs()
			if err != nil {
				errors <- fmt.Errorf("goroutine %d failed: %w", id, err)
				return
			}

			if len(configs) != 20 {
				errors <- fmt.Errorf("goroutine %d got %d configs, expected 20", id, len(configs))
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errors)

	// Check for errors
	var errorList []error
	for err := range errors {
		errorList = append(errorList, err)
	}

	assert.Empty(t, errorList, "No errors should occur during concurrent GetAllConfigs")
}
