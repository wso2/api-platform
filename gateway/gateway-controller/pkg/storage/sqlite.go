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
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"log/slog"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed gateway-controller-db.sql
var schemaSQL string

const (
	sqliteUniqueArtifactsNameVersion = "UNIQUE constraint failed: artifacts.gateway_id, artifacts.kind, artifacts.display_name, artifacts.version"
	sqliteUniqueArtifactsUUID        = "UNIQUE constraint failed: artifacts.uuid"
	sqliteUniqueArtifactsHandle      = "UNIQUE constraint failed: artifacts.gateway_id, artifacts.kind, artifacts.handle"
	sqliteUniqueCertificatesName     = "UNIQUE constraint failed: certificates.name, certificates.gateway_id"
	sqliteUniqueCertificatesUUID     = "UNIQUE constraint failed: certificates.uuid"
	sqliteUniqueTemplatesHandle      = "UNIQUE constraint failed: llm_provider_templates.handle, llm_provider_templates.gateway_id"
	sqliteUniqueAPIKeysKey           = "UNIQUE constraint failed: api_keys.api_key"
	sqliteUniqueAPIKeysUUID          = "UNIQUE constraint failed: api_keys.uuid"
)

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
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	logger.Info("SQLite storage initialized",
		slog.String("database_path", dbPath),
		slog.String("journal_mode", "WAL"))

	return storage, nil
}

// initSchema creates the database schema if it doesn't exist
func (s *SQLiteStorage) initSchema() error {
	// Check schema version
	var version int
	err := s.db.QueryRow("PRAGMA user_version").Scan(&version)
	if err != nil {
		return fmt.Errorf("failed to query schema version: %w", err)
	}

	if version == 0 {
		s.logger.Info("Initializing database schema (version 10)")
		s.logger.Debug("Creating schema with SQL", slog.String("schema_sql", schemaSQL))

		// Execute schema creation SQL
		if _, err := s.db.Exec(schemaSQL); err != nil {
			return fmt.Errorf("failed to create schema: %w", err)
		}

		s.logger.Info("Database schema initialized successfully")
	} else {
		// Migrations
		if version == 1 {
			// Add policy_definitions table (idempotent due to IF NOT EXISTS in embedded schema)
			if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS policy_definitions (
				name TEXT NOT NULL,
				version TEXT NOT NULL,
				provider TEXT NOT NULL,
				description TEXT,
				flows_request_require_header INTEGER,
				flows_request_require_body INTEGER,
				flows_response_require_header INTEGER,
				flows_response_require_body INTEGER,
				parameters_schema TEXT,
				PRIMARY KEY (name, version)
			);`); err != nil {
				return fmt.Errorf("failed to migrate schema to version 2: %w", err)
			}
			if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_policy_provider ON policy_definitions(provider);`); err != nil {
				return fmt.Errorf("failed to create policy_definitions index: %w", err)
			}
			if _, err := s.db.Exec("PRAGMA user_version = 2"); err != nil {
				return fmt.Errorf("failed to set schema version to 2: %w", err)
			}
			s.logger.Info("Schema migrated to version 2 (policy_definitions)")
			version = 2
		}

		if version == 2 {
			// Add certificates table
			if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS certificates (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL UNIQUE,
				certificate BLOB NOT NULL,
				subject TEXT NOT NULL,
				issuer TEXT NOT NULL,
				not_before TIMESTAMP NOT NULL,
				not_after TIMESTAMP NOT NULL,
				cert_count INTEGER NOT NULL DEFAULT 1,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			);`); err != nil {
				return fmt.Errorf("failed to migrate schema to version 3 (certificates): %w", err)
			}
			if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_cert_name ON certificates(name);`); err != nil {
				return fmt.Errorf("failed to create certificates name index: %w", err)
			}
			if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_cert_expiry ON certificates(not_after);`); err != nil {
				return fmt.Errorf("failed to create certificates expiry index: %w", err)
			}
			if _, err := s.db.Exec("PRAGMA user_version = 3"); err != nil {
				return fmt.Errorf("failed to set schema version to 3: %w", err)
			}
			s.logger.Info("Schema migrated to version 3 (certificates table)")
			version = 3
		}

		if version == 3 {
			// Add llm_provider_templates table
			if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS llm_provider_templates (
				id TEXT PRIMARY KEY,
				handle TEXT NOT NULL UNIQUE,
				configuration TEXT NOT NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			);`); err != nil {
				return fmt.Errorf("failed to migrate schema to version 4: %w", err)
			}
			if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_template_handle ON llm_provider_templates(handle);`); err != nil {
				return fmt.Errorf("failed to create llm_provider_templates index: %w", err)
			}
			if _, err := s.db.Exec("PRAGMA user_version = 4"); err != nil {
				return fmt.Errorf("failed to set schema version to 4: %w", err)
			}

			s.logger.Info("Schema migrated to version 4 (llm_provider_templates)")

			version = 4
		}

		if version == 4 {
			// Add API keys table with masked_api_key column
			if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS api_keys (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				api_key TEXT NOT NULL UNIQUE,
				masked_api_key TEXT NOT NULL,
				apiId TEXT NOT NULL,
				operations TEXT NOT NULL DEFAULT '*',
				status TEXT NOT NULL CHECK(status IN ('active', 'revoked', 'expired')) DEFAULT 'active',
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				created_by TEXT NOT NULL DEFAULT 'system',
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				expires_at TIMESTAMP NULL,
				expires_in_unit TEXT NULL,
				expires_in_duration INTEGER NULL,
				FOREIGN KEY (apiId) REFERENCES deployments(id) ON DELETE CASCADE,
				UNIQUE (apiId, name)
			);`); err != nil {
				return fmt.Errorf("failed to migrate schema to version 5 (api_keys): %w", err)
			}

			if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_api_key ON api_keys(api_key);`); err != nil {
				return fmt.Errorf("failed to create api_keys key index: %w", err)
			}
			if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_api_key_api ON api_keys(apiId);`); err != nil {
				return fmt.Errorf("failed to create api_keys handle index: %w", err)
			}
			if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_api_key_status ON api_keys(status);`); err != nil {
				return fmt.Errorf("failed to create api_keys status index: %w", err)
			}
			if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_api_key_expiry ON api_keys(expires_at);`); err != nil {
				return fmt.Errorf("failed to create api_keys expiry index: %w", err)
			}
			if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_created_by ON api_keys(created_by);`); err != nil {
				return fmt.Errorf("failed to create api_keys created_by index: %w", err)
			}
			if _, err := s.db.Exec("PRAGMA user_version = 5"); err != nil {
				return fmt.Errorf("failed to set schema version to 5: %w", err)
			}
			s.logger.Info("Schema migrated to version 5 (api_keys table with masked_api_key)")
			version = 5
		}

		if version == 5 {
			// Check if masked_api_key column exists, if not add it (for existing tables)
			var columnExists int
			err := s.db.QueryRow(`
				SELECT COUNT(*) FROM pragma_table_info('api_keys') 
				WHERE name = 'masked_api_key'
			`).Scan(&columnExists)
			if err == nil && columnExists == 0 {
				// Column doesn't exist, add it (as nullable first, then update)
				if _, err := s.db.Exec(`ALTER TABLE api_keys ADD COLUMN masked_api_key TEXT`); err != nil {
					return fmt.Errorf("failed to add masked_api_key column: %w", err)
				}
				// Update existing rows to have a masked version of their api_key
				if _, err := s.db.Exec(`
					UPDATE api_keys 
					SET masked_api_key = CASE 
						WHEN length(api_key) > 12 THEN substr(api_key, 1, 8) || '...' || substr(api_key, -4)
						ELSE api_key
					END
					WHERE masked_api_key IS NULL
				`); err != nil {
					s.logger.Warn("Failed to update existing masked_api_key values", slog.Any("error", err))
				}
			}

			// Add external API key support columns (only if missing; fresh DBs may already have them)
			err = s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('api_keys') WHERE name = 'source'`).Scan(&columnExists)
			if err == nil && columnExists == 0 {
				if _, err := s.db.Exec(`ALTER TABLE api_keys ADD COLUMN source TEXT NOT NULL DEFAULT 'local'`); err != nil {
					return fmt.Errorf("failed to add source column to api_keys: %w", err)
				}
			}
			err = s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('api_keys') WHERE name = 'external_ref_id'`).Scan(&columnExists)
			if err == nil && columnExists == 0 {
				if _, err := s.db.Exec(`ALTER TABLE api_keys ADD COLUMN external_ref_id TEXT NULL`); err != nil {
					return fmt.Errorf("failed to add external_ref_id column to api_keys: %w", err)
				}
			}
			// Backfill legacy keys: treat NULL, empty, or 'null' source as 'local' (DB + local cache consistency)
			if _, err := s.db.Exec(`
				UPDATE api_keys
				SET source = 'local'
				WHERE
					source IS NULL
					OR trim(source) = ''
					OR lower(trim(source)) = 'null'
			`); err != nil {
				s.logger.Warn("Failed to backfill api_keys.source for legacy keys", slog.Any("error", err))
			}
			if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_api_key_source ON api_keys(source);`); err != nil {
				return fmt.Errorf("failed to create api_keys source index: %w", err)
			}
			if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_api_key_external_ref ON api_keys(external_ref_id);`); err != nil {
				return fmt.Errorf("failed to create api_keys external_ref_id index: %w", err)
			}
			// Add index_key column for O(1) external API key lookup optimization
			err = s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('api_keys') WHERE name = 'index_key'`).Scan(&columnExists)
			if err == nil && columnExists == 0 {
				if _, err := s.db.Exec(`ALTER TABLE api_keys ADD COLUMN index_key TEXT NULL`); err != nil {
					return fmt.Errorf("failed to add index_key column to api_keys: %w", err)
				}
			}
			if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_api_key_index_key ON api_keys(index_key);`); err != nil {
				return fmt.Errorf("failed to create api_keys index_key index: %w", err)
			}
			// Add display_name column for human-readable API key names
			err = s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('api_keys') WHERE name = 'display_name'`).Scan(&columnExists)
			if err == nil && columnExists == 0 {
				if _, err := s.db.Exec(`ALTER TABLE api_keys ADD COLUMN display_name TEXT NOT NULL DEFAULT ''`); err != nil {
					return fmt.Errorf("failed to add display_name column to api_keys: %w", err)
				}
				// Backfill existing rows: set display_name = name for existing API keys
				if _, err := s.db.Exec(`UPDATE api_keys SET display_name = name WHERE display_name = ''`); err != nil {
					s.logger.Warn("Failed to backfill api_keys.display_name", slog.Any("error", err))
				}
			}
			if _, err := s.db.Exec("PRAGMA user_version = 6"); err != nil {
				return fmt.Errorf("failed to set schema version to 6: %w", err)
			}
			s.logger.Info("Schema migrated to version 6 (api_keys: external ref, index_key, display_name)")
			version = 6
		}

		if version == 6 {
			// Rebuild deployments table to update CHECK constraint to include 'undeployed' status
			s.logger.Info("Migrating schema to version 7 (adding 'undeployed' status to deployments)")

			// Disable foreign keys before migration (PRAGMA cannot be used inside transaction)
			if _, err := s.db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
				return fmt.Errorf("failed to disable foreign keys for migration: %w", err)
			}

			// Begin transaction for atomic migration
			tx, err := s.db.BeginTx(context.Background(), nil)
			if err != nil {
				// Re-enable foreign keys before returning
				s.db.Exec("PRAGMA foreign_keys = ON")
				return fmt.Errorf("failed to begin transaction for migration to version 7: %w", err)
			}
			defer func() {
				if err != nil {
					if rbErr := tx.Rollback(); rbErr != nil {
						s.logger.Error("Failed to rollback migration transaction", slog.Any("error", rbErr))
					}
					// Re-enable foreign keys after rollback
					s.db.Exec("PRAGMA foreign_keys = ON")
				}
			}()

			// SQLite doesn't support ALTER COLUMN, so we need to rebuild the table
			// 1. Create new table with updated constraint
			if _, err = tx.Exec(`CREATE TABLE deployments_new (
				id TEXT PRIMARY KEY,
				display_name TEXT NOT NULL,
				version TEXT NOT NULL,
				context TEXT NOT NULL,
				kind TEXT NOT NULL,
				handle TEXT NOT NULL UNIQUE,
				status TEXT NOT NULL CHECK(status IN ('pending', 'deployed', 'failed', 'undeployed')),
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				deployed_at TIMESTAMP,
				deployed_version INTEGER NOT NULL DEFAULT 0,
				UNIQUE(display_name, version)
			);`); err != nil {
				return fmt.Errorf("failed to create deployments_new table: %w", err)
			}

			// 2. Copy data from old table to new table
			if _, err = tx.Exec(`
				INSERT INTO deployments_new 
				SELECT id, display_name, version, context, kind, handle, status, 
				       created_at, updated_at, deployed_at, deployed_version
				FROM deployments;
			`); err != nil {
				return fmt.Errorf("failed to copy data to deployments_new: %w", err)
			}

			// 3. Drop old table
			if _, err = tx.Exec(`DROP TABLE deployments;`); err != nil {
				return fmt.Errorf("failed to drop old deployments table: %w", err)
			}

			// 4. Rename new table to original name
			if _, err = tx.Exec(`ALTER TABLE deployments_new RENAME TO deployments;`); err != nil {
				return fmt.Errorf("failed to rename deployments_new to deployments: %w", err)
			}

			// 5. Recreate indexes
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_name_version ON deployments(display_name, version);`); err != nil {
				return fmt.Errorf("failed to create idx_name_version: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_status ON deployments(status);`); err != nil {
				return fmt.Errorf("failed to create idx_status: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_context ON deployments(context);`); err != nil {
				return fmt.Errorf("failed to create idx_context: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_kind ON deployments(kind);`); err != nil {
				return fmt.Errorf("failed to create idx_kind: %w", err)
			}

			// 6. Update schema version
			if _, err = tx.Exec("PRAGMA user_version = 7"); err != nil {
				return fmt.Errorf("failed to set schema version to 7: %w", err)
			}

			// Commit the transaction
			if err = tx.Commit(); err != nil {
				return fmt.Errorf("failed to commit migration to version 7: %w", err)
			}

			// Re-enable foreign keys after successful migration
			if _, err := s.db.Exec("PRAGMA foreign_keys = ON"); err != nil {
				return fmt.Errorf("failed to re-enable foreign keys after migration: %w", err)
			}

			s.logger.Info("Schema migrated to version 7 (deployments status includes 'undeployed')")
			version = 7
		}

		if version == 7 {
			s.logger.Info("Migrating schema to version 8 (adding the new column gateway_id)")

			if _, err := s.db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
				return fmt.Errorf("failed to disable foreign keys for migration to version 8: %w", err)
			}

			tx, err := s.db.BeginTx(context.Background(), nil)
			if err != nil {
				s.db.Exec("PRAGMA foreign_keys = ON")
				return fmt.Errorf("failed to begin transaction for migration to version 8: %w", err)
			}
			defer func() {
				if err != nil {
					if rbErr := tx.Rollback(); rbErr != nil {
						s.logger.Error("Failed to rollback migration transaction", slog.Any("error", rbErr))
					}
					s.db.Exec("PRAGMA foreign_keys = ON")
				}
			}()

			if _, err = tx.Exec(`CREATE TABLE deployments_new_v8 (
				id TEXT PRIMARY KEY,
				gateway_id TEXT NOT NULL DEFAULT 'platform-gateway-id',
				display_name TEXT NOT NULL,
				version TEXT NOT NULL,
				context TEXT NOT NULL,
				kind TEXT NOT NULL,
				handle TEXT NOT NULL,
				status TEXT NOT NULL CHECK(status IN ('pending', 'deployed', 'failed', 'undeployed')),
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				deployed_at TIMESTAMP,
				deployed_version INTEGER NOT NULL DEFAULT 0,
				UNIQUE(display_name, version, gateway_id),
				UNIQUE(handle, gateway_id)
			);`); err != nil {
				return fmt.Errorf("failed to create deployments_new_v8 table: %w", err)
			}

			if _, err = tx.Exec(`
				INSERT INTO deployments_new_v8 (
					id, display_name, version, context, kind, handle, status,
					created_at, updated_at, deployed_at, deployed_version
				)
				SELECT id, display_name, version, context, kind, handle, status,
				       created_at, updated_at, deployed_at, deployed_version
				FROM deployments;
			`); err != nil {
				return fmt.Errorf("failed to copy data to deployments_new_v8: %w", err)
			}

			if _, err = tx.Exec(`DROP TABLE deployments;`); err != nil {
				return fmt.Errorf("failed to drop deployments table during version 8 migration: %w", err)
			}
			if _, err = tx.Exec(`ALTER TABLE deployments_new_v8 RENAME TO deployments;`); err != nil {
				return fmt.Errorf("failed to rename deployments_new_v8 table: %w", err)
			}

			if _, err = tx.Exec(`CREATE TABLE certificates_new_v8 (
				id TEXT PRIMARY KEY,
				gateway_id TEXT NOT NULL DEFAULT 'platform-gateway-id',
				name TEXT NOT NULL,
				certificate BLOB NOT NULL,
				subject TEXT NOT NULL,
				issuer TEXT NOT NULL,
				not_before TIMESTAMP NOT NULL,
				not_after TIMESTAMP NOT NULL,
				cert_count INTEGER NOT NULL DEFAULT 1,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(name, gateway_id)
			);`); err != nil {
				return fmt.Errorf("failed to create certificates_new_v8 table: %w", err)
			}

			if _, err = tx.Exec(`
				INSERT INTO certificates_new_v8 (
					id, name, certificate, subject, issuer, not_before, not_after,
					cert_count, created_at, updated_at
				)
				SELECT id, name, certificate, subject, issuer, not_before, not_after,
				       cert_count, created_at, updated_at
				FROM certificates;
			`); err != nil {
				return fmt.Errorf("failed to copy data to certificates_new_v8: %w", err)
			}

			if _, err = tx.Exec(`DROP TABLE certificates;`); err != nil {
				return fmt.Errorf("failed to drop certificates table during version 8 migration: %w", err)
			}
			if _, err = tx.Exec(`ALTER TABLE certificates_new_v8 RENAME TO certificates;`); err != nil {
				return fmt.Errorf("failed to rename certificates_new_v8 table: %w", err)
			}

			if _, err = tx.Exec(`CREATE TABLE llm_provider_templates_new_v8 (
				id TEXT PRIMARY KEY,
				gateway_id TEXT NOT NULL DEFAULT 'platform-gateway-id',
				handle TEXT NOT NULL,
				configuration TEXT NOT NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(handle, gateway_id)
			);`); err != nil {
				return fmt.Errorf("failed to create llm_provider_templates_new_v8 table: %w", err)
			}

			if _, err = tx.Exec(`
				INSERT INTO llm_provider_templates_new_v8 (
					id, handle, configuration, created_at, updated_at
				)
				SELECT id, handle, configuration, created_at, updated_at
				FROM llm_provider_templates;
			`); err != nil {
				return fmt.Errorf("failed to copy data to llm_provider_templates_new_v8: %w", err)
			}

			if _, err = tx.Exec(`DROP TABLE llm_provider_templates;`); err != nil {
				return fmt.Errorf("failed to drop llm_provider_templates table during version 8 migration: %w", err)
			}
			if _, err = tx.Exec(`ALTER TABLE llm_provider_templates_new_v8 RENAME TO llm_provider_templates;`); err != nil {
				return fmt.Errorf("failed to rename llm_provider_templates_new_v8 table: %w", err)
			}

			if _, err = tx.Exec(`CREATE TABLE api_keys_new_v8 (
				id TEXT PRIMARY KEY,
				gateway_id TEXT NOT NULL DEFAULT 'platform-gateway-id',
				name TEXT NOT NULL,
				api_key TEXT NOT NULL UNIQUE,
				masked_api_key TEXT NOT NULL,
				apiId TEXT NOT NULL,
				operations TEXT NOT NULL DEFAULT '*',
				status TEXT NOT NULL CHECK(status IN ('active', 'revoked', 'expired')) DEFAULT 'active',
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				created_by TEXT NOT NULL DEFAULT 'system',
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				expires_at TIMESTAMP NULL,
				expires_in_unit TEXT NULL,
				expires_in_duration INTEGER NULL,
				source TEXT NOT NULL DEFAULT 'local',
				external_ref_id TEXT NULL,
				index_key TEXT NULL,
				display_name TEXT NOT NULL DEFAULT '',
				FOREIGN KEY (apiId) REFERENCES deployments(id) ON DELETE CASCADE,
				UNIQUE (apiId, name, gateway_id)
			);`); err != nil {
				return fmt.Errorf("failed to create api_keys_new_v8 table: %w", err)
			}

			if _, err = tx.Exec(`
				INSERT INTO api_keys_new_v8 (
					id, name, api_key, masked_api_key, apiId, operations, status,
					created_at, created_by, updated_at, expires_at, expires_in_unit, expires_in_duration,
					source, external_ref_id, index_key, display_name
				)
				SELECT id, name, api_key, masked_api_key, apiId, operations, status,
				       created_at, created_by, updated_at, expires_at, expires_in_unit, expires_in_duration,
				       source, external_ref_id, index_key, display_name
				FROM api_keys;
			`); err != nil {
				return fmt.Errorf("failed to copy data to api_keys_new_v8: %w", err)
			}

			if _, err = tx.Exec(`DROP TABLE api_keys;`); err != nil {
				return fmt.Errorf("failed to drop api_keys table during version 8 migration: %w", err)
			}
			if _, err = tx.Exec(`ALTER TABLE api_keys_new_v8 RENAME TO api_keys;`); err != nil {
				return fmt.Errorf("failed to rename api_keys_new_v8 table: %w", err)
			}

			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_status ON deployments(status);`); err != nil {
				return fmt.Errorf("failed to recreate idx_status in version 8 migration: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_context ON deployments(context);`); err != nil {
				return fmt.Errorf("failed to recreate idx_context in version 8 migration: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_kind ON deployments(kind);`); err != nil {
				return fmt.Errorf("failed to recreate idx_kind in version 8 migration: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_deployments_gateway_id ON deployments(gateway_id);`); err != nil {
				return fmt.Errorf("failed to recreate idx_deployments_gateway_id in version 8 migration: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_certificates_gateway_id ON certificates(gateway_id);`); err != nil {
				return fmt.Errorf("failed to recreate idx_certificates_gateway_id in version 8 migration: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_cert_name ON certificates(name);`); err != nil {
				return fmt.Errorf("failed to recreate idx_cert_name in version 8 migration: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_cert_expiry ON certificates(not_after);`); err != nil {
				return fmt.Errorf("failed to recreate idx_cert_expiry in version 8 migration: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_llm_provider_templates_gateway_id ON llm_provider_templates(gateway_id);`); err != nil {
				return fmt.Errorf("failed to recreate idx_llm_provider_templates_gateway_id in version 8 migration: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_template_handle ON llm_provider_templates(handle);`); err != nil {
				return fmt.Errorf("failed to recreate idx_template_handle in version 8 migration: %w", err)
			}

			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_api_key ON api_keys(api_key);`); err != nil {
				return fmt.Errorf("failed to recreate idx_api_key in version 8 migration: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_api_key_api ON api_keys(apiId);`); err != nil {
				return fmt.Errorf("failed to recreate idx_api_key_api in version 8 migration: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_api_key_status ON api_keys(status);`); err != nil {
				return fmt.Errorf("failed to recreate idx_api_key_status in version 8 migration: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_api_key_expiry ON api_keys(expires_at);`); err != nil {
				return fmt.Errorf("failed to recreate idx_api_key_expiry in version 8 migration: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_created_by ON api_keys(created_by);`); err != nil {
				return fmt.Errorf("failed to recreate idx_created_by in version 8 migration: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_api_key_source ON api_keys(source);`); err != nil {
				return fmt.Errorf("failed to recreate idx_api_key_source in version 8 migration: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_api_key_external_ref ON api_keys(external_ref_id);`); err != nil {
				return fmt.Errorf("failed to recreate idx_api_key_external_ref in version 8 migration: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_api_key_index_key ON api_keys(index_key);`); err != nil {
				return fmt.Errorf("failed to recreate idx_api_key_index_key in version 8 migration: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_api_keys_gateway_id ON api_keys(gateway_id);`); err != nil {
				return fmt.Errorf("failed to recreate idx_api_keys_gateway_id in version 8 migration: %w", err)
			}
			if _, err = tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_external_api_key
					ON api_keys(apiId, index_key)
					WHERE source = 'external' AND index_key IS NOT NULL;`); err != nil {
				return fmt.Errorf("failed to recreate idx_unique_external_api_key in version 8 migration: %w", err)
			}

			if _, err = tx.Exec("PRAGMA user_version = 8"); err != nil {
				return fmt.Errorf("failed to set schema version to 8: %w", err)
			}

			if err = tx.Commit(); err != nil {
				return fmt.Errorf("failed to commit migration to version 8: %w", err)
			}

			if _, err := s.db.Exec("PRAGMA foreign_keys = ON"); err != nil {
				return fmt.Errorf("failed to re-enable foreign keys after migration to version 8: %w", err)
			}

			s.logger.Info("Schema migrated to version 8 (added gateway_id column to tables.)")
			version = 8
		}

		// Migration to version 8: Drop index_key column and indexes if they exist
		if version == 8 {
			s.logger.Info("Migrating schema to version 9 (removing index_key if exists)")

			// Disable foreign keys for migration
			if _, err := s.db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
				return fmt.Errorf("failed to disable foreign keys for migration: %w", err)
			}

			// Begin transaction for atomic migration
			tx, err := s.db.BeginTx(context.Background(), nil)
			if err != nil {
				// Re-enable foreign keys before returning
				s.db.Exec("PRAGMA foreign_keys = ON")
				return fmt.Errorf("failed to begin transaction for migration to version 8: %w", err)
			}
			defer func() {
				if err != nil {
					if rbErr := tx.Rollback(); rbErr != nil {
						s.logger.Error("Failed to rollback migration transaction", slog.Any("error", rbErr))
					}
					// Re-enable foreign keys after rollback
					s.db.Exec("PRAGMA foreign_keys = ON")
				}
			}()

			// Drop indexes if they exist
			if _, err = tx.Exec(`DROP INDEX IF EXISTS idx_unique_external_api_key;`); err != nil {
				return fmt.Errorf("failed to drop idx_unique_external_api_key: %w", err)
			}
			if _, err = tx.Exec(`DROP INDEX IF EXISTS idx_api_key_index_key;`); err != nil {
				return fmt.Errorf("failed to drop idx_api_key_index_key: %w", err)
			}

			// Check if index_key column exists and drop it
			var indexKeyExists int
			err = tx.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('api_keys') WHERE name = 'index_key'`).Scan(&indexKeyExists)
			if err == nil && indexKeyExists > 0 {
				s.logger.Info("Dropping index_key column from api_keys table")
				if _, err = tx.Exec(`ALTER TABLE api_keys DROP COLUMN index_key;`); err != nil {
					return fmt.Errorf("failed to drop index_key column: %w", err)
				}
			}

			// Update schema version
			if _, err = tx.Exec("PRAGMA user_version = 9"); err != nil {
				return fmt.Errorf("failed to set schema version to 9: %w", err)
			}

			// Commit the transaction
			if err = tx.Commit(); err != nil {
				return fmt.Errorf("failed to commit migration to version 8: %w", err)
			}

			// Re-enable foreign keys after successful migration
			if _, err := s.db.Exec("PRAGMA foreign_keys = ON"); err != nil {
				return fmt.Errorf("failed to re-enable foreign keys after migration: %w", err)
			}

			s.logger.Info("Schema migrated to version 9 (removed index_key)")
			version = 9
		}

		// Migration to version 10: deployments→artifacts, per-resource-type tables, id→uuid
		if version == 9 {
			s.logger.Info("Migrating schema to version 10 (artifacts + per-resource-type tables)")

			// Check if this is a real migration (old deployments table exists) or just a version bump
			var deploymentsExists bool
			if err := s.db.QueryRow("SELECT COUNT(*) > 0 FROM sqlite_master WHERE type='table' AND name='deployments'").Scan(&deploymentsExists); err != nil {
				return fmt.Errorf("v10: failed to check deployments table: %w", err)
			}

			if !deploymentsExists {
				// Fresh DB already has the new schema, just bump version
				if _, err := s.db.Exec("PRAGMA user_version = 10"); err != nil {
					return fmt.Errorf("v10: failed to set schema version: %w", err)
				}
				s.logger.Info("Schema version bumped to 10 (already has new schema)")
				version = 10
			} else {

			if _, err := s.db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
				return fmt.Errorf("failed to disable foreign keys for migration to version 10: %w", err)
			}

			tx, err := s.db.BeginTx(context.Background(), nil)
			if err != nil {
				s.db.Exec("PRAGMA foreign_keys = ON")
				return fmt.Errorf("failed to begin transaction for migration to version 10: %w", err)
			}
			defer func() {
				if err != nil {
					if rbErr := tx.Rollback(); rbErr != nil {
						s.logger.Error("Failed to rollback migration transaction", slog.Any("error", rbErr))
					}
					s.db.Exec("PRAGMA foreign_keys = ON")
				}
			}()

			// 1. Rename deployments → artifacts (with column changes)
			if _, err = tx.Exec(`CREATE TABLE artifacts (
				uuid TEXT PRIMARY KEY,
				gateway_id TEXT NOT NULL DEFAULT 'default',
				display_name TEXT NOT NULL,
				version TEXT NOT NULL,
				kind TEXT NOT NULL,
				handle TEXT NOT NULL,
				status TEXT NOT NULL CHECK(status IN ('pending', 'deployed', 'failed', 'undeployed')),
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				deployed_at TIMESTAMP,
				UNIQUE(gateway_id, kind, display_name, version),
				UNIQUE(gateway_id, kind, handle)
			);`); err != nil {
				return fmt.Errorf("v10: failed to create artifacts table: %w", err)
			}

			if _, err = tx.Exec(`
				INSERT INTO artifacts (uuid, gateway_id, display_name, version, kind, handle, status, created_at, updated_at, deployed_at)
				SELECT id, gateway_id, display_name, version, kind, handle, status, created_at, updated_at, deployed_at
				FROM deployments;
			`); err != nil {
				return fmt.Errorf("v10: failed to copy deployments to artifacts: %w", err)
			}

			// 2. Create per-resource-type tables
			if _, err = tx.Exec(`CREATE TABLE rest_apis (
				uuid TEXT PRIMARY KEY,
				configuration TEXT NOT NULL,
				FOREIGN KEY(uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE
			);`); err != nil {
				return fmt.Errorf("v10: failed to create rest_apis table: %w", err)
			}

			if _, err = tx.Exec(`CREATE TABLE websub_apis (
				uuid TEXT PRIMARY KEY,
				configuration TEXT NOT NULL,
				FOREIGN KEY(uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE
			);`); err != nil {
				return fmt.Errorf("v10: failed to create websub_apis table: %w", err)
			}

			if _, err = tx.Exec(`CREATE TABLE llm_providers (
				uuid TEXT PRIMARY KEY,
				configuration TEXT NOT NULL,
				FOREIGN KEY(uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE
			);`); err != nil {
				return fmt.Errorf("v10: failed to create llm_providers table: %w", err)
			}

			if _, err = tx.Exec(`CREATE TABLE llm_proxies (
				uuid TEXT PRIMARY KEY,
				configuration TEXT NOT NULL,
				provider_uuid TEXT NOT NULL,
				FOREIGN KEY(uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
				FOREIGN KEY(provider_uuid) REFERENCES llm_providers(uuid) ON DELETE RESTRICT
			);`); err != nil {
				return fmt.Errorf("v10: failed to create llm_proxies table: %w", err)
			}

			if _, err = tx.Exec(`CREATE TABLE mcp_proxies (
				uuid TEXT PRIMARY KEY,
				configuration TEXT NOT NULL,
				FOREIGN KEY(uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE
			);`); err != nil {
				return fmt.Errorf("v10: failed to create mcp_proxies table: %w", err)
			}

			// 3. Migrate data from deployment_configs to per-type tables
			// RestApi/WebSubApi: use configuration column (source IS the derived config)
			if _, err = tx.Exec(`
				INSERT INTO rest_apis (uuid, configuration)
				SELECT dc.id, dc.configuration
				FROM deployment_configs dc
				JOIN artifacts a ON dc.id = a.uuid
				WHERE a.kind = 'RestApi';
			`); err != nil {
				return fmt.Errorf("v10: failed to migrate rest_apis data: %w", err)
			}

			if _, err = tx.Exec(`
				INSERT INTO websub_apis (uuid, configuration)
				SELECT dc.id, dc.configuration
				FROM deployment_configs dc
				JOIN artifacts a ON dc.id = a.uuid
				WHERE a.kind = 'WebSubApi';
			`); err != nil {
				return fmt.Errorf("v10: failed to migrate websub_apis data: %w", err)
			}

			// LlmProvider/LlmProxy/Mcp: use source_configuration column (original typed config)
			if _, err = tx.Exec(`
				INSERT INTO llm_providers (uuid, configuration)
				SELECT dc.id, dc.source_configuration
				FROM deployment_configs dc
				JOIN artifacts a ON dc.id = a.uuid
				WHERE a.kind = 'LlmProvider';
			`); err != nil {
				return fmt.Errorf("v10: failed to migrate llm_providers data: %w", err)
			}

			if _, err = tx.Exec(`
				INSERT INTO llm_proxies (uuid, configuration, provider_uuid)
				SELECT dc.id, dc.source_configuration, ''
				FROM deployment_configs dc
				JOIN artifacts a ON dc.id = a.uuid
				WHERE a.kind = 'LlmProxy';
			`); err != nil {
				return fmt.Errorf("v10: failed to migrate llm_proxies data: %w", err)
			}

			if _, err = tx.Exec(`
				INSERT INTO mcp_proxies (uuid, configuration)
				SELECT dc.id, dc.source_configuration
				FROM deployment_configs dc
				JOIN artifacts a ON dc.id = a.uuid
				WHERE a.kind = 'Mcp';
			`); err != nil {
				return fmt.Errorf("v10: failed to migrate mcp_proxies data: %w", err)
			}

			// 4. Drop old tables
			if _, err = tx.Exec(`DROP TABLE deployment_configs;`); err != nil {
				return fmt.Errorf("v10: failed to drop deployment_configs: %w", err)
			}
			if _, err = tx.Exec(`DROP TABLE deployments;`); err != nil {
				return fmt.Errorf("v10: failed to drop deployments: %w", err)
			}

			// 5. Rebuild certificates with uuid column
			if _, err = tx.Exec(`CREATE TABLE certificates_v10 (
				uuid TEXT PRIMARY KEY,
				gateway_id TEXT NOT NULL DEFAULT 'default',
				name TEXT NOT NULL,
				certificate BLOB NOT NULL,
				subject TEXT NOT NULL,
				issuer TEXT NOT NULL,
				not_before TIMESTAMP NOT NULL,
				not_after TIMESTAMP NOT NULL,
				cert_count INTEGER NOT NULL DEFAULT 1,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(name, gateway_id)
			);`); err != nil {
				return fmt.Errorf("v10: failed to create certificates_v10: %w", err)
			}
			if _, err = tx.Exec(`
				INSERT INTO certificates_v10 (uuid, gateway_id, name, certificate, subject, issuer, not_before, not_after, cert_count, created_at, updated_at)
				SELECT id, gateway_id, name, certificate, subject, issuer, not_before, not_after, cert_count, created_at, updated_at
				FROM certificates;
			`); err != nil {
				return fmt.Errorf("v10: failed to copy certificates: %w", err)
			}
			if _, err = tx.Exec(`DROP TABLE certificates;`); err != nil {
				return fmt.Errorf("v10: failed to drop old certificates: %w", err)
			}
			if _, err = tx.Exec(`ALTER TABLE certificates_v10 RENAME TO certificates;`); err != nil {
				return fmt.Errorf("v10: failed to rename certificates_v10: %w", err)
			}

			// 6. Rebuild llm_provider_templates with uuid column
			if _, err = tx.Exec(`CREATE TABLE llm_provider_templates_v10 (
				uuid TEXT PRIMARY KEY,
				gateway_id TEXT NOT NULL DEFAULT 'default',
				handle TEXT NOT NULL,
				configuration TEXT NOT NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(handle, gateway_id)
			);`); err != nil {
				return fmt.Errorf("v10: failed to create llm_provider_templates_v10: %w", err)
			}
			if _, err = tx.Exec(`
				INSERT INTO llm_provider_templates_v10 (uuid, gateway_id, handle, configuration, created_at, updated_at)
				SELECT id, gateway_id, handle, configuration, created_at, updated_at
				FROM llm_provider_templates;
			`); err != nil {
				return fmt.Errorf("v10: failed to copy llm_provider_templates: %w", err)
			}
			if _, err = tx.Exec(`DROP TABLE llm_provider_templates;`); err != nil {
				return fmt.Errorf("v10: failed to drop old llm_provider_templates: %w", err)
			}
			if _, err = tx.Exec(`ALTER TABLE llm_provider_templates_v10 RENAME TO llm_provider_templates;`); err != nil {
				return fmt.Errorf("v10: failed to rename llm_provider_templates_v10: %w", err)
			}

			// 7. Rebuild api_keys with uuid and artifact_uuid columns
			if _, err = tx.Exec(`CREATE TABLE api_keys_v10 (
				uuid TEXT PRIMARY KEY,
				gateway_id TEXT NOT NULL DEFAULT 'default',
				name TEXT NOT NULL,
				api_key TEXT NOT NULL UNIQUE,
				masked_api_key TEXT NOT NULL,
				artifact_uuid TEXT NOT NULL,
				operations TEXT NOT NULL DEFAULT '*',
				status TEXT NOT NULL CHECK(status IN ('active', 'revoked', 'expired')) DEFAULT 'active',
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				created_by TEXT NOT NULL DEFAULT 'system',
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				expires_at TIMESTAMP NULL,
				expires_in_unit TEXT NULL,
				expires_in_duration INTEGER NULL,
				source TEXT NOT NULL DEFAULT 'local',
				external_ref_id TEXT NULL,
				display_name TEXT NOT NULL DEFAULT '',
				FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
				UNIQUE (artifact_uuid, name, gateway_id)
			);`); err != nil {
				return fmt.Errorf("v10: failed to create api_keys_v10: %w", err)
			}
			if _, err = tx.Exec(`
				INSERT INTO api_keys_v10 (uuid, gateway_id, name, api_key, masked_api_key, artifact_uuid, operations, status,
					created_at, created_by, updated_at, expires_at, expires_in_unit, expires_in_duration,
					source, external_ref_id, display_name)
				SELECT id, gateway_id, name, api_key, masked_api_key, apiId, operations, status,
					created_at, created_by, updated_at, expires_at, expires_in_unit, expires_in_duration,
					source, external_ref_id, display_name
				FROM api_keys;
			`); err != nil {
				return fmt.Errorf("v10: failed to copy api_keys: %w", err)
			}
			if _, err = tx.Exec(`DROP TABLE api_keys;`); err != nil {
				return fmt.Errorf("v10: failed to drop old api_keys: %w", err)
			}
			if _, err = tx.Exec(`ALTER TABLE api_keys_v10 RENAME TO api_keys;`); err != nil {
				return fmt.Errorf("v10: failed to rename api_keys_v10: %w", err)
			}

			// 8. Recreate indexes
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_status ON artifacts(status);`); err != nil {
				return fmt.Errorf("v10: failed to create idx_status: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_kind ON artifacts(kind);`); err != nil {
				return fmt.Errorf("v10: failed to create idx_kind: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_artifacts_gateway_id ON artifacts(gateway_id);`); err != nil {
				return fmt.Errorf("v10: failed to create idx_artifacts_gateway_id: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_cert_name ON certificates(name);`); err != nil {
				return fmt.Errorf("v10: failed to recreate idx_cert_name: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_cert_expiry ON certificates(not_after);`); err != nil {
				return fmt.Errorf("v10: failed to recreate idx_cert_expiry: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_certificates_gateway_id ON certificates(gateway_id);`); err != nil {
				return fmt.Errorf("v10: failed to recreate idx_certificates_gateway_id: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_template_handle ON llm_provider_templates(handle);`); err != nil {
				return fmt.Errorf("v10: failed to recreate idx_template_handle: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_llm_provider_templates_gateway_id ON llm_provider_templates(gateway_id);`); err != nil {
				return fmt.Errorf("v10: failed to recreate idx_llm_provider_templates_gateway_id: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_api_key ON api_keys(api_key);`); err != nil {
				return fmt.Errorf("v10: failed to recreate idx_api_key: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_api_key_api ON api_keys(artifact_uuid);`); err != nil {
				return fmt.Errorf("v10: failed to recreate idx_api_key_api: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_api_key_status ON api_keys(status);`); err != nil {
				return fmt.Errorf("v10: failed to recreate idx_api_key_status: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_api_key_expiry ON api_keys(expires_at);`); err != nil {
				return fmt.Errorf("v10: failed to recreate idx_api_key_expiry: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_created_by ON api_keys(created_by);`); err != nil {
				return fmt.Errorf("v10: failed to recreate idx_created_by: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_api_key_source ON api_keys(source);`); err != nil {
				return fmt.Errorf("v10: failed to recreate idx_api_key_source: %w", err)
			}
			if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_api_key_external_ref ON api_keys(external_ref_id);`); err != nil {
				return fmt.Errorf("v10: failed to recreate idx_api_key_external_ref: %w", err)
			}

			// 9. Set version
			if _, err = tx.Exec("PRAGMA user_version = 10"); err != nil {
				return fmt.Errorf("v10: failed to set schema version: %w", err)
			}

			if err = tx.Commit(); err != nil {
				return fmt.Errorf("v10: failed to commit migration: %w", err)
			}

			if _, err := s.db.Exec("PRAGMA foreign_keys = ON"); err != nil {
				return fmt.Errorf("v10: failed to re-enable foreign keys: %w", err)
			}

			s.logger.Info("Schema migrated to version 10 (artifacts + per-resource-type tables)")
			version = 10

			} // end deploymentsExists else block
		}
	}

	s.logger.Info("Database schema up to date", slog.Int("version", version))

	return nil
}

func isUniqueConstraintError(err error) bool {
	return err != nil && (err.Error() == sqliteUniqueArtifactsNameVersion ||
		err.Error() == sqliteUniqueArtifactsUUID ||
		err.Error() == sqliteUniqueArtifactsHandle)
}

func isCertificateUniqueConstraintError(err error) bool {
	return err != nil && (err.Error() == sqliteUniqueCertificatesName ||
		err.Error() == sqliteUniqueCertificatesUUID)
}

func isTemplateUniqueConstraintError(err error) bool {
	return err != nil && err.Error() == sqliteUniqueTemplatesHandle
}

func isAPIKeyUniqueConstraintError(err error) bool {
	return err != nil &&
		(err.Error() == sqliteUniqueAPIKeysKey ||
			err.Error() == sqliteUniqueAPIKeysUUID)
}
