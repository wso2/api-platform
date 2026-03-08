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
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// TestDatabaseFileCreation verifies that SQLite database files are created correctly
func TestDatabaseFileCreation(t *testing.T) {
	// Initialize metrics for tests (disabled by default)
	metrics.SetEnabled(false)
	metrics.Init()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Create storage
	db, err := storage.NewStorage(storage.BackendConfig{Type: "sqlite", SQLitePath: dbPath}, logger)
	require.NoError(t, err, "Failed to create SQLite storage")
	defer db.Close()

	// Verify main database file exists
	_, err = os.Stat(dbPath)
	assert.NoError(t, err, "Database file should exist at %s", dbPath)

	// WAL and SHM files are created after the first write
	cfg := createTestConfig("TestAPI", "v1.0")
	require.NoError(t, db.SaveConfig(cfg))

	// Check for WAL and SHM files (they may exist)
	walPath := dbPath + "-wal"
	shmPath := dbPath + "-shm"

	walStat, walErr := os.Stat(walPath)
	shmStat, shmErr := os.Stat(shmPath)

	if walErr == nil {
		t.Logf("WAL file exists: %s (size: %d bytes)", walPath, walStat.Size())
	} else {
		t.Logf("WAL file does not exist (may be checkpointed): %s", walPath)
	}

	if shmErr == nil {
		t.Logf("SHM file exists: %s (size: %d bytes)", shmPath, shmStat.Size())
	} else {
		t.Logf("SHM file does not exist: %s", shmPath)
	}
}

// TestSchemaInitialization verifies that the database schema is correctly initialized
func TestSchemaInitialization(t *testing.T) {
	// Initialize metrics for tests (disabled by default)
	metrics.SetEnabled(false)
	metrics.Init()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "schema.db")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Create storage (should initialize schema)
	db, err := storage.NewStorage(storage.BackendConfig{Type: "sqlite", SQLitePath: dbPath}, logger)
	require.NoError(t, err)
	defer db.Close()

	// Open raw SQLite connection to inspect schema
	rawDB, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer rawDB.Close()

	// Verify schema version
	t.Run("SchemaVersion", func(t *testing.T) {
		var version int
		err := rawDB.QueryRow("PRAGMA user_version").Scan(&version)
		assert.NoError(t, err)
		assert.Equal(t, 1, version, "Schema version should be 1")
	})

	// Verify artifacts table exists
	t.Run("TableExists", func(t *testing.T) {
		var tableName string
		err := rawDB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='artifacts'").Scan(&tableName)
		assert.NoError(t, err)
		assert.Equal(t, "artifacts", tableName)
	})

	// Verify table schema
	t.Run("TableSchema", func(t *testing.T) {
		rows, err := rawDB.Query("PRAGMA table_info(artifacts)")
		require.NoError(t, err)
		defer rows.Close()

		columns := make(map[string]string)
		for rows.Next() {
			var cid int
			var name, colType string
			var notNull, pk int
			var dfltValue sql.NullString

			err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk)
			require.NoError(t, err)

			columns[name] = colType
		}

		// Verify expected columns exist
		expectedColumns := map[string]string{
			"uuid":         "TEXT",
			"display_name": "TEXT",
			"version":      "TEXT",
			"kind":         "TEXT",
			"handle":       "TEXT",
			"status":       "TEXT",
			"created_at":   "TIMESTAMP",
			"updated_at":   "TIMESTAMP",
			"deployed_at":  "TIMESTAMP",
			"gateway_id":   "TEXT",
		}

		for colName, colType := range expectedColumns {
			actualType, exists := columns[colName]
			assert.True(t, exists, "Column %s should exist", colName)
			assert.Equal(t, colType, actualType, "Column %s should have type %s", colName, colType)
		}
	})

	// Verify per-resource-type tables exist
	t.Run("ResourceTypeTablesExist", func(t *testing.T) {
		tables := []string{"rest_apis", "websub_apis", "llm_providers", "llm_proxies", "mcp_proxies"}
		for _, table := range tables {
			var tableName string
			err := rawDB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&tableName)
			assert.NoError(t, err, "Table %s should exist", table)
			assert.Equal(t, table, tableName)
		}
	})

	// Verify rest_apis table schema
	t.Run("RestApisTableSchema", func(t *testing.T) {
		rows, err := rawDB.Query("PRAGMA table_info(rest_apis)")
		require.NoError(t, err)
		defer rows.Close()

		columns := make(map[string]string)
		for rows.Next() {
			var cid int
			var name, colType string
			var notNull, pk int
			var dfltValue sql.NullString
			err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk)
			require.NoError(t, err)
			columns[name] = colType
		}

		expectedColumns := map[string]string{
			"uuid":          "TEXT",
			"configuration": "TEXT",
		}
		for colName, colType := range expectedColumns {
			actualType, exists := columns[colName]
			assert.True(t, exists, "Column %s should exist in rest_apis", colName)
			assert.Equal(t, colType, actualType, "Column %s should have type %s", colName, colType)
		}
	})

	// Verify indexes exist
	t.Run("IndexesExist", func(t *testing.T) {
		rows, err := rawDB.Query("SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='artifacts'")
		require.NoError(t, err)
		defer rows.Close()

		indexes := make(map[string]bool)
		for rows.Next() {
			var name string
			err := rows.Scan(&name)
			require.NoError(t, err)
			indexes[name] = true
		}

		expectedIndexes := []string{
			"idx_artifacts_status",
			"idx_artifacts_kind",
		}

		for _, idxName := range expectedIndexes {
			assert.True(t, indexes[idxName], "Index %s should exist", idxName)
		}
	})

	// Verify UNIQUE constraint on artifacts
	t.Run("UniqueConstraint", func(t *testing.T) {
		var sqlStr string
		err := rawDB.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='artifacts'").Scan(&sqlStr)
		require.NoError(t, err)

		assert.Contains(t, sqlStr, "UNIQUE(gateway_id, kind, display_name, version)", "Should have UNIQUE constraint on (gateway_id, kind, display_name, version)")
	})

	// Verify CHECK constraint on status
	t.Run("CheckConstraint", func(t *testing.T) {
		var sqlStr string
		err := rawDB.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='artifacts'").Scan(&sqlStr)
		require.NoError(t, err)

		assert.Contains(t, sqlStr, "CHECK(status IN ('pending', 'deployed', 'failed', 'undeployed'))", "Should have CHECK constraint on status")
	})

	// Verify WAL mode is enabled
	t.Run("WALMode", func(t *testing.T) {
		var journalMode string
		err := rawDB.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
		assert.NoError(t, err)
		assert.Equal(t, "wal", journalMode, "Journal mode should be WAL")
	})

	// Verify foreign keys setting (we don't have foreign keys in our schema,
	// but the pragma should be readable)
	t.Run("ForeignKeysPragma", func(t *testing.T) {
		var foreignKeys int
		err := rawDB.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys)
		assert.NoError(t, err)
		// Foreign keys pragma should return either 0 or 1 (we don't have FKs in our schema)
		assert.Contains(t, []int{0, 1}, foreignKeys, "Foreign keys pragma should be readable")
	})
}

// TestSchemaInitializationIdempotent verifies that schema initialization is idempotent
// (reopening an existing database doesn't recreate the schema)
func TestSchemaInitializationIdempotent(t *testing.T) {
	// Initialize metrics for tests (disabled by default)
	metrics.SetEnabled(false)
	metrics.Init()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "idempotent.db")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// First initialization
	db1, err := storage.NewStorage(storage.BackendConfig{Type: "sqlite", SQLitePath: dbPath}, logger)
	require.NoError(t, err)

	// Add a configuration
	cfg := createTestConfig("IdempotentAPI", "v1.0")
	require.NoError(t, db1.SaveConfig(cfg))

	db1.Close()

	// Second initialization (should not recreate schema or lose data)
	db2, err := storage.NewStorage(storage.BackendConfig{Type: "sqlite", SQLitePath: dbPath}, logger)
	require.NoError(t, err)
	defer db2.Close()

	// Verify configuration still exists
	retrieved, err := db2.GetConfigByHandle("IdempotentAPI-v1.0")
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, cfg.UUID, retrieved.UUID)
}

// TestEmptyDatabaseInitialization tests that a fresh database initializes correctly
// (Success Criterion SC-001)
func TestEmptyDatabaseInitialization(t *testing.T) {
	// Initialize metrics for tests (disabled by default)
	metrics.SetEnabled(false)
	metrics.Init()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "empty.db")

	// Ensure database file doesn't exist
	_, err := os.Stat(dbPath)
	assert.True(t, os.IsNotExist(err), "Database file should not exist initially")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Create storage (should auto-create database and schema)
	db, err := storage.NewStorage(storage.BackendConfig{Type: "sqlite", SQLitePath: dbPath}, logger)
	assert.NoError(t, err, "Should successfully create database from scratch")
	defer db.Close()

	// Verify database file now exists
	_, err = os.Stat(dbPath)
	assert.NoError(t, err, "Database file should now exist")

	// Verify we can perform operations
	cfg := createTestConfig("EmptyTestAPI", "v1.0")
	err = db.SaveConfig(cfg)
	assert.NoError(t, err, "Should be able to save configuration to fresh database")

	// Verify we can retrieve it
	retrieved, err := db.GetConfig(cfg.UUID)
	assert.NoError(t, err)
	assert.Equal(t, cfg.UUID, retrieved.UUID)
}

// TestDatabaseIntegrityCheck verifies that the database maintains integrity
func TestDatabaseIntegrityCheck(t *testing.T) {
	// Initialize metrics for tests (disabled by default)
	metrics.SetEnabled(false)
	metrics.Init()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "integrity.db")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	db, err := storage.NewStorage(storage.BackendConfig{Type: "sqlite", SQLitePath: dbPath}, logger)
	require.NoError(t, err)

	// Add multiple configurations
	for i := 0; i < 10; i++ {
		cfg := createTestConfig("IntegrityAPI"+string(rune(i+'0')), "v1.0")
		require.NoError(t, db.SaveConfig(cfg))
	}

	db.Close()

	// Run integrity check on the database
	rawDB, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer rawDB.Close()

	var result string
	err = rawDB.QueryRow("PRAGMA integrity_check").Scan(&result)
	assert.NoError(t, err)
	assert.Equal(t, "ok", result, "Database integrity check should pass")
}
