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

package storage

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed gateway-controller-db.sql
var schemaSQL string

// SQLiteStorage implements the Storage interface using SQLite
type SQLiteStorage struct {
	db     *sql.DB
	logger *slog.Logger
}

// newSQLiteStorage creates a new SQLite storage instance.
func newSQLiteStorage(dbPath string, logger *slog.Logger) (*SQLiteStorage, error) {
	// Build connection string with SQLite pragmas for optimal performance
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_cache_size=2000&_foreign_keys=ON", dbPath)

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// CRITICAL: Prevents "database is locked" errors with concurrent access
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	storage := &SQLiteStorage{
		db:     db,
		logger: logger,
	}

	// Initialize schema if needed
	if err := storage.initSchema(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("failed to initialize schema: %w", errors.Join(err, closeErr))
		}
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	logger.Info("SQLite storage initialized",
		slog.String("database_path", dbPath),
		slog.String("journal_mode", "WAL"))

	return storage, nil
}

const currentSchemaVersion = 6

// previousSchemaVersion is the only older schema version this binary migrates
// in place. Any other stored version is refused rather than skipped.
const previousSchemaVersion = 5

// initSchema creates the database schema if it doesn't exist
func (s *SQLiteStorage) initSchema() error {
	var version int
	err := s.db.QueryRow("PRAGMA user_version").Scan(&version)
	if err != nil {
		return fmt.Errorf("failed to query schema version: %w", err)
	}

	switch {
	case version == 0:
		s.logger.Info("Initializing database schema", slog.Int("version", currentSchemaVersion))
		if _, err := s.db.Exec(schemaSQL); err != nil {
			return fmt.Errorf("failed to create schema: %w", err)
		}
		s.logger.Info("Database schema initialized successfully")
	case version == previousSchemaVersion:
		s.logger.Info("Migrating database schema",
			slog.Int("from_version", version), slog.Int("to_version", currentSchemaVersion))
		if err := s.migrateSchemaV5ToV6(); err != nil {
			return fmt.Errorf("failed to migrate schema from version %d to %d: %w", version, currentSchemaVersion, err)
		}
		s.logger.Info("Database schema migrated successfully")
	case version != currentSchemaVersion:
		return fmt.Errorf("unsupported schema version %d, expected %d; delete the database to recreate", version, currentSchemaVersion)
	}

	s.logger.Info("Database schema up to date", slog.Int("version", currentSchemaVersion))
	return nil
}

// migrateSchemaV5ToV6 adds the certificates usage, role, match_json,
// private_key_ciphertext and key_algorithm columns to a version 5 database
// and sets user_version to 6. usage and role are defaulted; the rest are
// nullable. No existing column changes.
func (s *SQLiteStorage) migrateSchemaV5ToV6() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin migration transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.Exec(`ALTER TABLE certificates ADD COLUMN usage TEXT NOT NULL DEFAULT 'upstream'`); err != nil {
		return fmt.Errorf("failed to add certificates.usage column: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE certificates ADD COLUMN role TEXT NOT NULL DEFAULT 'client'`); err != nil {
		return fmt.Errorf("failed to add certificates.role column: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE certificates ADD COLUMN match_json TEXT`); err != nil {
		return fmt.Errorf("failed to add certificates.match_json column: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE certificates ADD COLUMN private_key_ciphertext TEXT`); err != nil {
		return fmt.Errorf("failed to add certificates.private_key_ciphertext column: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE certificates ADD COLUMN key_algorithm TEXT`); err != nil {
		return fmt.Errorf("failed to add certificates.key_algorithm column: %w", err)
	}
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", currentSchemaVersion)); err != nil {
		return fmt.Errorf("failed to set schema version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration: %w", err)
	}
	return nil
}

func isSQLiteUniqueConstraintError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed:")
}
