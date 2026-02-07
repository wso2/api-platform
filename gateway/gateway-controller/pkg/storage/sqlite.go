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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

//go:embed gateway-controller-db.sql
var schemaSQL string

// SQLiteStorage implements the Storage interface using SQLite
type SQLiteStorage struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewSQLiteStorage creates a new SQLite storage instance
func NewSQLiteStorage(dbPath string, logger *slog.Logger) (*SQLiteStorage, error) {
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
		s.logger.Info("Initializing database schema (version 6)")
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

		s.logger.Info("Database schema up to date", slog.Int("version", version))
	}

	return nil
}

// SaveConfig persists a new deployment configuration
func (s *SQLiteStorage) SaveConfig(cfg *models.StoredConfig) error {
	// Extract fields for indexed columns
	displayName := cfg.GetDisplayName()
	version := cfg.GetVersion()
	context := cfg.GetContext()
	handle := cfg.GetHandle()

	if handle == "" {
		return fmt.Errorf("handle (metadata.name) is required and cannot be empty")
	}

	query := `
		INSERT INTO deployments (
			id, display_name, version, context, kind, handle,
			status, created_at, updated_at, deployed_version
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	stmt, err := s.db.Prepare(query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now()
	_, err = stmt.Exec(
		cfg.ID,
		displayName,
		version,
		context,
		cfg.Kind,
		handle,
		cfg.Status,
		now,
		now,
		cfg.DeployedVersion,
	)

	if err != nil {
		// Check for unique constraint violation
		if isUniqueConstraintError(err) {
			return fmt.Errorf("%w: configuration with displayName '%s' and version '%s' already exists", ErrConflict, displayName, version)
		}
		return fmt.Errorf("failed to insert configuration: %w", err)
	}

	_, err = s.addDeploymentConfigs(cfg)
	if err != nil {
		return fmt.Errorf("failed to add deployment configurations: %w", err)
	}

	s.logger.Info("Configuration saved",
		slog.String("id", cfg.ID),
		slog.String("displayName", displayName),
		slog.String("version", version))

	return nil
}

// UpdateConfig updates an existing deployment configuration
func (s *SQLiteStorage) UpdateConfig(cfg *models.StoredConfig) error {
	startTime := time.Now()
	table := "deployments"

	// Check if configuration exists
	_, err := s.GetConfig(cfg.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			metrics.DatabaseOperationsTotal.WithLabelValues("update", table, "error").Inc()
			metrics.StorageErrorsTotal.WithLabelValues("update", "not_found").Inc()
			return fmt.Errorf("cannot update non-existent configuration: %w", err)
		}
		metrics.DatabaseOperationsTotal.WithLabelValues("update", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("update", "database_error").Inc()
		return err
	}

	// Extract fields for indexed columns
	displayName := cfg.GetDisplayName()
	version := cfg.GetVersion()
	context := cfg.GetContext()
	handle := cfg.GetHandle()

	if handle == "" {
		metrics.DatabaseOperationsTotal.WithLabelValues("update", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("update", "validation_error").Inc()
		return fmt.Errorf("handle (metadata.name) is required and cannot be empty")
	}

	query := `
		UPDATE deployments
		SET display_name = ?, version = ?, context = ?, kind = ?, handle = ?,
			status = ?, updated_at = ?,
			deployed_version = ?
		WHERE id = ?
	`

	stmt, err := s.db.Prepare(query)
	if err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("update", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("update", "prepare_error").Inc()
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(
		displayName,
		version,
		context,
		cfg.Kind,
		handle,
		cfg.Status,
		time.Now(),
		cfg.DeployedVersion,
		cfg.ID,
	)

	if err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("update", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("update", "exec_error").Inc()
		return fmt.Errorf("failed to update configuration: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("update", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("update", "rows_affected_error").Inc()
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		metrics.DatabaseOperationsTotal.WithLabelValues("update", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("update", "not_found").Inc()
		return fmt.Errorf("%w: id=%s", ErrNotFound, cfg.ID)
	}

	_, err = s.updateDeploymentConfigs(cfg)
	if err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("update", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("update", "deployment_config_error").Inc()
		return fmt.Errorf("failed to update deployment configurations: %w", err)
	}

	// Record successful metrics
	metrics.DatabaseOperationsTotal.WithLabelValues("update", table, "success").Inc()
	metrics.DatabaseOperationDurationSeconds.WithLabelValues("update", table).Observe(time.Since(startTime).Seconds())

	s.logger.Info("Configuration updated",
		slog.String("id", cfg.ID),
		slog.String("displayName", displayName),
		slog.String("version", version))

	return nil
}

// DeleteConfig removes an deployment configuration by ID
func (s *SQLiteStorage) DeleteConfig(id string) error {
	startTime := time.Now()
	table := "deployments"
	query := `DELETE FROM deployments WHERE id = ?`

	result, err := s.db.Exec(query, id)
	if err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("delete", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("delete", "exec_error").Inc()
		return fmt.Errorf("failed to delete configuration: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("delete", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("delete", "rows_affected_error").Inc()
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		metrics.DatabaseOperationsTotal.WithLabelValues("delete", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("delete", "not_found").Inc()
		return fmt.Errorf("%w: id=%s", ErrNotFound, id)
	}

	// Record successful metrics
	metrics.DatabaseOperationsTotal.WithLabelValues("delete", table, "success").Inc()
	metrics.DatabaseOperationDurationSeconds.WithLabelValues("delete", table).Observe(time.Since(startTime).Seconds())

	s.logger.Info("Configuration deleted", slog.String("id", id))

	return nil
}

// GetConfig retrieves an deployment configuration by ID
func (s *SQLiteStorage) GetConfig(id string) (*models.StoredConfig, error) {
	startTime := time.Now()
	table := "deployments"
	query := `
		SELECT d.id, d.kind, dc.configuration, dc.source_configuration, d.status, d.created_at,
		d.updated_at, d.deployed_at, d.deployed_version
		FROM deployments d
		LEFT JOIN deployment_configs dc ON d.id = dc.id
		WHERE d.id = ?
	`

	var cfg models.StoredConfig
	var configJSON string
	var sourceConfigJSON string
	var deployedAt sql.NullTime

	err := s.db.QueryRow(query, id).Scan(
		&cfg.ID,
		&cfg.Kind,
		&configJSON,
		&sourceConfigJSON,
		&cfg.Status,
		&cfg.CreatedAt,
		&cfg.UpdatedAt,
		&deployedAt,
		&cfg.DeployedVersion,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			metrics.DatabaseOperationsTotal.WithLabelValues("read", table, "error").Inc()
			metrics.StorageErrorsTotal.WithLabelValues("read", "not_found").Inc()
			return nil, fmt.Errorf("%w: id=%s", ErrNotFound, id)
		}
		metrics.DatabaseOperationsTotal.WithLabelValues("read", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("read", "query_error").Inc()
		return nil, fmt.Errorf("failed to query configuration: %w", err)
	}

	// Parse deployed_at (nullable field)
	if deployedAt.Valid {
		cfg.DeployedAt = &deployedAt.Time
	}

	// Deserialize JSON configuration
	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), &cfg.Configuration); err != nil {
			metrics.DatabaseOperationsTotal.WithLabelValues("read", table, "error").Inc()
			metrics.StorageErrorsTotal.WithLabelValues("read", "unmarshal_error").Inc()
			return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
		}
	}
	if sourceConfigJSON != "" {
		if err := json.Unmarshal([]byte(sourceConfigJSON), &cfg.SourceConfiguration); err != nil {
			metrics.DatabaseOperationsTotal.WithLabelValues("read", table, "error").Inc()
			metrics.StorageErrorsTotal.WithLabelValues("read", "unmarshal_error").Inc()
			return nil, fmt.Errorf("failed to unmarshal source configuration: %w", err)
		}
	}

	// Record successful metrics
	metrics.DatabaseOperationsTotal.WithLabelValues("read", table, "success").Inc()
	metrics.DatabaseOperationDurationSeconds.WithLabelValues("read", table).Observe(time.Since(startTime).Seconds())

	return &cfg, nil
}

// GetConfigByNameVersion retrieves an deployment configuration by displayName and version
func (s *SQLiteStorage) GetConfigByNameVersion(name, version string) (*models.StoredConfig, error) {
	query := `
		SELECT d.id, d.kind, dc.configuration, dc.source_configuration, d.status, d.created_at, d.updated_at,
			   d.deployed_at, d.deployed_version
		FROM deployments d
		LEFT JOIN deployment_configs dc ON d.id = dc.id
		WHERE d.display_name = ? AND d.version = ?
	`

	var cfg models.StoredConfig
	var configJSON string
	var sourceConfigJSON string
	var deployedAt sql.NullTime

	err := s.db.QueryRow(query, name, version).Scan(
		&cfg.ID,
		&cfg.Kind,
		&configJSON,
		&sourceConfigJSON,
		&cfg.Status,
		&cfg.CreatedAt,
		&cfg.UpdatedAt,
		&deployedAt,
		&cfg.DeployedVersion,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: name=%s, version=%s", ErrNotFound, name, version)
		}
		return nil, fmt.Errorf("failed to query configuration: %w", err)
	}

	// Parse deployed_at (nullable field)
	if deployedAt.Valid {
		cfg.DeployedAt = &deployedAt.Time
	}

	// Deserialize JSON configuration
	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), &cfg.Configuration); err != nil {
			return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
		}
	}
	if sourceConfigJSON != "" {
		if err := json.Unmarshal([]byte(sourceConfigJSON), &cfg.SourceConfiguration); err != nil {
			return nil, fmt.Errorf("failed to unmarshal source configuration: %w", err)
		}
	}

	return &cfg, nil
}

// GetConfigByHandle retrieves a deployment configuration by handle (metadata.name)
func (s *SQLiteStorage) GetConfigByHandle(handle string) (*models.StoredConfig, error) {
	query := `
		SELECT d.id, d.kind, dc.configuration, dc.source_configuration, d.status, d.created_at, d.updated_at,
			   d.deployed_at, d.deployed_version
		FROM deployments d
		LEFT JOIN deployment_configs dc ON d.id = dc.id
		WHERE d.handle = ?
	`

	var cfg models.StoredConfig
	var configJSON string
	var sourceConfigJSON string
	var deployedAt sql.NullTime

	err := s.db.QueryRow(query, handle).Scan(
		&cfg.ID,
		&cfg.Kind,
		&configJSON,
		&sourceConfigJSON,
		&cfg.Status,
		&cfg.CreatedAt,
		&cfg.UpdatedAt,
		&deployedAt,
		&cfg.DeployedVersion,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: handle=%s", ErrNotFound, handle)
		}
		return nil, fmt.Errorf("failed to query configuration: %w", err)
	}

	// Parse deployed_at (nullable field)
	if deployedAt.Valid {
		cfg.DeployedAt = &deployedAt.Time
	}

	// Deserialize JSON configuration
	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), &cfg.Configuration); err != nil {
			return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
		}
	}
	if sourceConfigJSON != "" {
		if err := json.Unmarshal([]byte(sourceConfigJSON), &cfg.SourceConfiguration); err != nil {
			return nil, fmt.Errorf("failed to unmarshal source configuration: %w", err)
		}
	}

	return &cfg, nil
}

// GetAllConfigs retrieves all deployment configurations
func (s *SQLiteStorage) GetAllConfigs() ([]*models.StoredConfig, error) {
	query := `
			SELECT d.id, d.kind, dc.configuration, dc.source_configuration, d.status, 
			d.created_at, d.updated_at, d.deployed_at, d.deployed_version
			FROM deployments d
			LEFT JOIN deployment_configs dc ON d.id = dc.id
			ORDER BY d.created_at DESC
		`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query configurations: %w", err)
	}
	defer rows.Close()

	var configs []*models.StoredConfig

	for rows.Next() {
		var cfg models.StoredConfig
		var configJSON string
		var sourceConfigJSON string
		var deployedAt sql.NullTime

		err := rows.Scan(
			&cfg.ID,
			&cfg.Kind,
			&configJSON,
			&sourceConfigJSON,
			&cfg.Status,
			&cfg.CreatedAt,
			&cfg.UpdatedAt,
			&deployedAt,
			&cfg.DeployedVersion,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Parse deployed_at (nullable field)
		if deployedAt.Valid {
			cfg.DeployedAt = &deployedAt.Time
		}

		// Deserialize JSON configuration
		if configJSON != "" {
			if err := json.Unmarshal([]byte(configJSON), &cfg.Configuration); err != nil {
				return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
			}
		}
		if sourceConfigJSON != "" {
			if err := json.Unmarshal([]byte(sourceConfigJSON), &cfg.SourceConfiguration); err != nil {
				return nil, fmt.Errorf("failed to unmarshal source configuration: %w", err)
			}
		}

		configs = append(configs, &cfg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return configs, nil
}

// GetAllConfigsByKind retrieves all deployment configurations of a specific kind
func (s *SQLiteStorage) GetAllConfigsByKind(kind string) ([]*models.StoredConfig, error) {
	query := `
			SELECT d.id, d.kind, dc.configuration, dc.source_configuration, d.status, 
			d.created_at, d.updated_at, d.deployed_at, d.deployed_version
			FROM deployments d
			LEFT JOIN deployment_configs dc ON d.id = dc.id 
			WHERE d.kind = ?
			ORDER BY d.created_at DESC
		`

	rows, err := s.db.Query(query, kind)
	if err != nil {
		return nil, fmt.Errorf("failed to query configurations: %w", err)
	}
	defer rows.Close()

	var configs []*models.StoredConfig

	for rows.Next() {
		var cfg models.StoredConfig
		var configJSON string
		var sourceConfigJSON string
		var deployedAt sql.NullTime

		err := rows.Scan(
			&cfg.ID,
			&cfg.Kind,
			&configJSON,
			&sourceConfigJSON,
			&cfg.Status,
			&cfg.CreatedAt,
			&cfg.UpdatedAt,
			&deployedAt,
			&cfg.DeployedVersion,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Parse deployed_at (nullable field)
		if deployedAt.Valid {
			cfg.DeployedAt = &deployedAt.Time
		}

		// Deserialize JSON configuration
		if configJSON != "" {
			if err := json.Unmarshal([]byte(configJSON), &cfg.Configuration); err != nil {
				return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
			}
		}
		if sourceConfigJSON != "" {
			if err := json.Unmarshal([]byte(sourceConfigJSON), &cfg.SourceConfiguration); err != nil {
				return nil, fmt.Errorf("failed to unmarshal source configuration: %w", err)
			}
		}

		configs = append(configs, &cfg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return configs, nil
}

// LoadLLMProviderTemplatesFromDatabase loads all LLM Provider templates from database into in-memory store
func LoadLLMProviderTemplatesFromDatabase(storage Storage, cache *ConfigStore) error {
	// Get all llm provider template configurations from persistent storage
	templates, err := storage.GetAllLLMProviderTemplates()
	if err != nil {
		return fmt.Errorf("failed to load templates from database: %w", err)
	}

	for _, template := range templates {
		if err := cache.AddTemplate(template); err != nil {
			return fmt.Errorf("failed to load llm provider template %s into cache: %w", template.GetHandle(), err)
		}
	}

	return nil
}

// SaveLLMProviderTemplate persists a new LLM provider template
func (s *SQLiteStorage) SaveLLMProviderTemplate(template *models.StoredLLMProviderTemplate) error {
	// Serialize configuration to JSON
	configJSON, err := json.Marshal(template.Configuration)
	if err != nil {
		return fmt.Errorf("failed to marshal template configuration: %w", err)
	}

	handle := template.GetHandle()

	query := `
		INSERT INTO llm_provider_templates (
			id, handle, configuration, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?)
	`

	now := time.Now()
	_, err = s.db.Exec(query,
		template.ID,
		handle,
		string(configJSON),
		now,
		now,
	)

	if err != nil {
		// Check for unique constraint violation
		if err.Error() == "UNIQUE constraint failed: llm_provider_templates.handle" {
			return fmt.Errorf("%w: template with handle '%s' already exists", ErrConflict, handle)
		}
		return fmt.Errorf("failed to insert template: %w", err)
	}

	s.logger.Info("LLM provider template saved",
		slog.String("uuid", template.ID),
		slog.String("handle", handle))

	return nil
}

// UpdateLLMProviderTemplate updates an existing LLM provider template
func (s *SQLiteStorage) UpdateLLMProviderTemplate(template *models.StoredLLMProviderTemplate) error {
	// Check if template exists
	_, err := s.GetLLMProviderTemplate(template.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return fmt.Errorf("cannot update non-existent template: %w", err)
		}
		return err
	}

	// Serialize configuration to JSON
	configJSON, err := json.Marshal(template.Configuration)
	if err != nil {
		return fmt.Errorf("failed to marshal template configuration: %w", err)
	}

	handle := template.GetHandle()

	query := `
		UPDATE llm_provider_templates
		SET handle = ?, configuration = ?, updated_at = ?
		WHERE id = ?
	`

	result, err := s.db.Exec(query,
		handle,
		string(configJSON),
		time.Now(),
		template.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update template: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("%w: uuid=%s", ErrNotFound, template.ID)
	}

	s.logger.Info("LLM provider template updated",
		slog.String("uuid", template.ID),
		slog.String("handle", handle))

	return nil
}

// DeleteLLMProviderTemplate removes an LLM provider template by ID
func (s *SQLiteStorage) DeleteLLMProviderTemplate(id string) error {
	query := `DELETE FROM llm_provider_templates WHERE id = ?`

	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("%w: uuid=%s", ErrNotFound, id)
	}

	s.logger.Info("LLM provider template deleted", slog.String("uuid", id))

	return nil
}

// GetLLMProviderTemplate retrieves an LLM provider template by ID
func (s *SQLiteStorage) GetLLMProviderTemplate(id string) (*models.StoredLLMProviderTemplate, error) {
	query := `
		SELECT id, configuration, created_at, updated_at
		FROM llm_provider_templates
		WHERE id = ?
	`

	var template models.StoredLLMProviderTemplate
	var configJSON string

	err := s.db.QueryRow(query, id).Scan(
		&template.ID,
		&configJSON,
		&template.CreatedAt,
		&template.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: uuid=%s", ErrNotFound, id)
		}
		return nil, fmt.Errorf("failed to query template: %w", err)
	}

	// Deserialize JSON configuration
	if err := json.Unmarshal([]byte(configJSON), &template.Configuration); err != nil {
		return nil, fmt.Errorf("failed to unmarshal template configuration: %w", err)
	}

	return &template, nil
}

// GetAllLLMProviderTemplates retrieves all LLM provider templates
func (s *SQLiteStorage) GetAllLLMProviderTemplates() ([]*models.StoredLLMProviderTemplate, error) {
	query := `
		SELECT id, configuration, created_at, updated_at
		FROM llm_provider_templates
		ORDER BY created_at DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query templates: %w", err)
	}
	defer rows.Close()

	var templates []*models.StoredLLMProviderTemplate

	for rows.Next() {
		var template models.StoredLLMProviderTemplate
		var configJSON string

		err := rows.Scan(
			&template.ID,
			&configJSON,
			&template.CreatedAt,
			&template.UpdatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Deserialize JSON configuration
		if err := json.Unmarshal([]byte(configJSON), &template.Configuration); err != nil {
			return nil, fmt.Errorf("failed to unmarshal template configuration: %w", err)
		}

		templates = append(templates, &template)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return templates, nil
}

// SaveCertificate persists a certificate to the database
func (s *SQLiteStorage) SaveCertificate(cert *models.StoredCertificate) error {
	query := `
		INSERT INTO certificates (
			id, name, certificate, subject, issuer,
			not_before, not_after, cert_count, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		cert.ID,
		cert.Name,
		cert.Certificate,
		cert.Subject,
		cert.Issuer,
		cert.NotBefore,
		cert.NotAfter,
		cert.CertCount,
		cert.CreatedAt,
		cert.UpdatedAt,
	)

	if err != nil {
		// Check for unique constraint violation
		if isCertificateUniqueConstraintError(err) {
			return fmt.Errorf("%w: certificate with name '%s' already exists", ErrConflict, cert.Name)
		}
		return fmt.Errorf("failed to save certificate: %w", err)
	}

	return nil
}

// GetCertificate retrieves a certificate by ID
func (s *SQLiteStorage) GetCertificate(id string) (*models.StoredCertificate, error) {
	query := `
		SELECT id, name, certificate, subject, issuer,
		       not_before, not_after, cert_count, created_at, updated_at
		FROM certificates
		WHERE id = ?
	`

	var cert models.StoredCertificate
	err := s.db.QueryRow(query, id).Scan(
		&cert.ID,
		&cert.Name,
		&cert.Certificate,
		&cert.Subject,
		&cert.Issuer,
		&cert.NotBefore,
		&cert.NotAfter,
		&cert.CertCount,
		&cert.CreatedAt,
		&cert.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: id=%s", ErrNotFound, id)
		}
		return nil, fmt.Errorf("failed to get certificate: %w", err)
	}

	return &cert, nil
}

// GetCertificateByName retrieves a certificate by name
func (s *SQLiteStorage) GetCertificateByName(name string) (*models.StoredCertificate, error) {
	query := `
		SELECT id, name, certificate, subject, issuer,
		       not_before, not_after, cert_count, created_at, updated_at
		FROM certificates
		WHERE name = ?
	`

	var cert models.StoredCertificate
	err := s.db.QueryRow(query, name).Scan(
		&cert.ID,
		&cert.Name,
		&cert.Certificate,
		&cert.Subject,
		&cert.Issuer,
		&cert.NotBefore,
		&cert.NotAfter,
		&cert.CertCount,
		&cert.CreatedAt,
		&cert.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get certificate by name: %w", err)
	}

	return &cert, nil
}

// ListCertificates retrieves all certificates
func (s *SQLiteStorage) ListCertificates() ([]*models.StoredCertificate, error) {
	query := `
		SELECT id, name, certificate, subject, issuer,
		       not_before, not_after, cert_count, created_at, updated_at
		FROM certificates
		ORDER BY created_at DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list certificates: %w", err)
	}
	defer rows.Close()

	var certs []*models.StoredCertificate
	for rows.Next() {
		var cert models.StoredCertificate
		if err := rows.Scan(
			&cert.ID,
			&cert.Name,
			&cert.Certificate,
			&cert.Subject,
			&cert.Issuer,
			&cert.NotBefore,
			&cert.NotAfter,
			&cert.CertCount,
			&cert.CreatedAt,
			&cert.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan certificate: %w", err)
		}
		certs = append(certs, &cert)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating certificate rows: %w", err)
	}

	return certs, nil
}

// DeleteCertificate deletes a certificate by ID
func (s *SQLiteStorage) DeleteCertificate(id string) error {
	query := `DELETE FROM certificates WHERE id = ?`

	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete certificate: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		s.logger.Debug("Certificate not found for deletion", slog.String("id", id))
		return ErrNotFound
	}

	s.logger.Info("Certificate deleted", slog.String("id", id))

	return nil
}

// API Key Storage Methods

// SaveAPIKey persists a new API key to the database or updates existing one
// if an API key with the same apiId and name already exists
func (s *SQLiteStorage) SaveAPIKey(apiKey *models.APIKey) error {

	// Begin transaction to ensure atomicity
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Ensure transaction is properly handled
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p) // Re-throw panic after rollback
		}
	}()

	// Before inserting, check for duplicates if this is an external key
	if apiKey.Source == "external" && apiKey.IndexKey != nil {
		var count int
		checkQuery := `SELECT COUNT(*) FROM api_keys                                                  
						WHERE apiId = ? AND index_key = ? AND source = 'external'`
		err := tx.QueryRow(checkQuery, apiKey.APIId, apiKey.IndexKey).Scan(&count)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to check for duplicate API key: %w", err)
		}
		if count > 0 {
			tx.Rollback()
			return fmt.Errorf("%w: API key value already exists for this API", ErrConflict)
		}
	}

	// First, check if an API key with the same apiId and name exists
	checkQuery := `SELECT id FROM api_keys WHERE apiId = ? AND name = ?`
	var existingID string
	err = tx.QueryRow(checkQuery, apiKey.APIId, apiKey.Name).Scan(&existingID)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		tx.Rollback()
		return fmt.Errorf("failed to check existing API key: %w", err)
	}

	if errors.Is(err, sql.ErrNoRows) {
		// No existing record, insert new API key
		insertQuery := `
			INSERT INTO api_keys (
				id, name, display_name, api_key, masked_api_key, apiId, operations, status,
				created_at, created_by, updated_at, expires_at, expires_in_unit, expires_in_duration,
				source, external_ref_id, index_key
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`

		_, err := tx.Exec(insertQuery,
			apiKey.ID,
			apiKey.Name,
			apiKey.DisplayName,
			apiKey.APIKey,
			apiKey.MaskedAPIKey,
			apiKey.APIId,
			apiKey.Operations,
			apiKey.Status,
			apiKey.CreatedAt,
			apiKey.CreatedBy,
			apiKey.UpdatedAt,
			apiKey.ExpiresAt,
			apiKey.Unit,
			apiKey.Duration,
			apiKey.Source,
			apiKey.ExternalRefId,
			apiKey.IndexKey,
		)

		if err != nil {
			tx.Rollback()
			// Check for unique constraint violation on api_key field
			if isAPIKeyUniqueConstraintError(err) {
				return fmt.Errorf("%w: API key value already exists", ErrConflict)
			}
			return fmt.Errorf("failed to insert API key: %w", err)
		}

		s.logger.Info("API key inserted successfully",
			slog.String("id", apiKey.ID),
			slog.String("name", apiKey.Name),
			slog.String("apiId", apiKey.APIId),
			slog.String("created_by", apiKey.CreatedBy))
	} else {
		// Existing record found, return conflict error that API Key name already exists
		tx.Rollback()
		s.logger.Error("API key name already exists for the API",
			slog.String("name", apiKey.Name),
			slog.String("apiId", apiKey.APIId),
			slog.Any("error", ErrConflict))
		return fmt.Errorf("%w: API key name already exists for the API: %s", ErrConflict, apiKey.Name)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetAPIKeyByID retrieves an API key by its ID
func (s *SQLiteStorage) GetAPIKeyByID(id string) (*models.APIKey, error) {
	query := `
		SELECT id, name, display_name, api_key, masked_api_key, apiId, operations, status,
		       created_at, created_by, updated_at, expires_at, source, external_ref_id, index_key
		FROM api_keys
		WHERE id = ?
	`

	var apiKey models.APIKey
	var expiresAt sql.NullTime
	var externalRefId sql.NullString
	var indexKey sql.NullString

	err := s.db.QueryRow(query, id).Scan(
		&apiKey.ID,
		&apiKey.Name,
		&apiKey.DisplayName,
		&apiKey.APIKey,
		&apiKey.MaskedAPIKey,
		&apiKey.APIId,
		&apiKey.Operations,
		&apiKey.Status,
		&apiKey.CreatedAt,
		&apiKey.CreatedBy,
		&apiKey.UpdatedAt,
		&expiresAt,
		&apiKey.Source,
		&externalRefId,
		&indexKey,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: key not found", ErrNotFound)
		}
		return nil, fmt.Errorf("failed to query API key: %w", err)
	}

	// Handle nullable fields
	if expiresAt.Valid {
		apiKey.ExpiresAt = &expiresAt.Time
	}
	if externalRefId.Valid {
		apiKey.ExternalRefId = &externalRefId.String
	}
	if indexKey.Valid {
		apiKey.IndexKey = &indexKey.String
	}

	return &apiKey, nil
}

// GetAPIKeyByKey retrieves an API key by its key value
func (s *SQLiteStorage) GetAPIKeyByKey(key string) (*models.APIKey, error) {
	query := `
		SELECT id, name, display_name, api_key, masked_api_key, apiId, operations, status,
		       created_at, created_by, updated_at, expires_at, source, external_ref_id, index_key
		FROM api_keys
		WHERE api_key = ?
	`

	var apiKey models.APIKey
	var expiresAt sql.NullTime
	var externalRefId sql.NullString
	var indexKey sql.NullString

	err := s.db.QueryRow(query, key).Scan(
		&apiKey.ID,
		&apiKey.Name,
		&apiKey.DisplayName,
		&apiKey.APIKey,
		&apiKey.MaskedAPIKey,
		&apiKey.APIId,
		&apiKey.Operations,
		&apiKey.Status,
		&apiKey.CreatedAt,
		&apiKey.CreatedBy,
		&apiKey.UpdatedAt,
		&expiresAt,
		&apiKey.Source,
		&externalRefId,
		&indexKey,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: key not found", ErrNotFound)
		}
		return nil, fmt.Errorf("failed to query API key: %w", err)
	}

	// Handle nullable fields
	if expiresAt.Valid {
		apiKey.ExpiresAt = &expiresAt.Time
	}
	if externalRefId.Valid {
		apiKey.ExternalRefId = &externalRefId.String
	}
	if indexKey.Valid {
		apiKey.IndexKey = &indexKey.String
	}

	return &apiKey, nil
}

// GetAPIKeysByAPI retrieves all API keys for a specific API
func (s *SQLiteStorage) GetAPIKeysByAPI(apiId string) ([]*models.APIKey, error) {
	query := `
		SELECT id, name, display_name, api_key, masked_api_key, apiId, operations, status,
		       created_at, created_by, updated_at, expires_at, source, external_ref_id, index_key
		FROM api_keys
		WHERE apiId = ?
		ORDER BY created_at DESC
	`

	rows, err := s.db.Query(query, apiId)
	if err != nil {
		return nil, fmt.Errorf("failed to query API keys: %w", err)
	}
	defer rows.Close()

	var apiKeys []*models.APIKey

	for rows.Next() {
		var apiKey models.APIKey
		var expiresAt sql.NullTime
		var externalRefId sql.NullString
		var indexKey sql.NullString

		err := rows.Scan(
			&apiKey.ID,
			&apiKey.Name,
			&apiKey.DisplayName,
			&apiKey.APIKey,
			&apiKey.MaskedAPIKey,
			&apiKey.APIId,
			&apiKey.Operations,
			&apiKey.Status,
			&apiKey.CreatedAt,
			&apiKey.CreatedBy,
			&apiKey.UpdatedAt,
			&expiresAt,
			&apiKey.Source,
			&externalRefId,
			&indexKey,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan API key row: %w", err)
		}

		// Handle nullable fields
		if expiresAt.Valid {
			apiKey.ExpiresAt = &expiresAt.Time
		}
		if externalRefId.Valid {
			apiKey.ExternalRefId = &externalRefId.String
		}
		if indexKey.Valid {
			apiKey.IndexKey = &indexKey.String
		}

		apiKeys = append(apiKeys, &apiKey)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating API key rows: %w", err)
	}

	return apiKeys, nil
}

// GetAPIKeysByAPIAndName retrieves an API key by its apiId and name
func (s *SQLiteStorage) GetAPIKeysByAPIAndName(apiId, name string) (*models.APIKey, error) {
	query := `
		SELECT id, name, display_name, api_key, masked_api_key, apiId, operations, status,
		       created_at, created_by, updated_at, expires_at, source, external_ref_id, index_key
		FROM api_keys
		WHERE apiId = ? AND name = ?
		LIMIT 1
	`

	var apiKey models.APIKey
	var expiresAt sql.NullTime
	var externalRefId sql.NullString
	var indexKey sql.NullString

	err := s.db.QueryRow(query, apiId, name).Scan(
		&apiKey.ID,
		&apiKey.Name,
		&apiKey.DisplayName,
		&apiKey.APIKey,
		&apiKey.MaskedAPIKey,
		&apiKey.APIId,
		&apiKey.Operations,
		&apiKey.Status,
		&apiKey.CreatedAt,
		&apiKey.CreatedBy,
		&apiKey.UpdatedAt,
		&expiresAt,
		&apiKey.Source,
		&externalRefId,
		&indexKey,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to query API key by name: %w", err)
	}

	// Handle nullable fields
	if expiresAt.Valid {
		apiKey.ExpiresAt = &expiresAt.Time
	}
	if externalRefId.Valid {
		apiKey.ExternalRefId = &externalRefId.String
	}
	if indexKey.Valid {
		apiKey.IndexKey = &indexKey.String
	}

	return &apiKey, nil
}

// UpdateAPIKey updates an existing API key
func (s *SQLiteStorage) UpdateAPIKey(apiKey *models.APIKey) error {

	// Begin transaction to ensure atomicity
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Ensure transaction is properly handled
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p) // Re-throw panic after rollback
		}
	}()

	if apiKey.Source == "external" && apiKey.IndexKey != nil {
		// Check for duplicate API key value within the same API (same value, different name)
		duplicateCheckQuery := `
			SELECT id, name FROM api_keys
			WHERE apiId = ? AND index_key = ? AND name != ?
			LIMIT 1
		`
		var duplicateID, duplicateName string
		err := tx.QueryRow(duplicateCheckQuery, apiKey.APIId, apiKey.IndexKey, apiKey.Name).Scan(&duplicateID, &duplicateName)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			tx.Rollback()
			return fmt.Errorf("failed to check for duplicate API key: %w", err)
		}
		if err == nil {
			// Row found: same key value already exists for this API under a different name
			tx.Rollback()
			return fmt.Errorf("%w: API key value already exists for this API", ErrConflict)
		}
	}

	updateQuery := `
			UPDATE api_keys
			SET api_key = ?, masked_api_key = ?, display_name = ?, operations = ?, status = ?, created_by = ?, updated_at = ?, expires_at = ?, expires_in_unit = ?, expires_in_duration = ?,
			    source = ?, external_ref_id = ?, index_key = ?
			WHERE apiId = ? AND name = ?
		`

	_, err = tx.Exec(updateQuery,
		apiKey.APIKey,
		apiKey.MaskedAPIKey,
		apiKey.DisplayName,
		apiKey.Operations,
		apiKey.Status,
		apiKey.CreatedBy,
		apiKey.UpdatedAt,
		apiKey.ExpiresAt,
		apiKey.Unit,
		apiKey.Duration,
		apiKey.Source,
		apiKey.ExternalRefId,
		apiKey.IndexKey,
		apiKey.APIId,
		apiKey.Name,
	)

	if err != nil {
		tx.Rollback()
		// Check for unique constraint violation on api_key field
		if isAPIKeyUniqueConstraintError(err) {
			return fmt.Errorf("%w: API key value already exists", ErrConflict)
		}
		return fmt.Errorf("failed to update API key: %w", err)
	}

	s.logger.Info("API key updated successfully",
		slog.String("name", apiKey.Name),
		slog.String("apiId", apiKey.APIId),
		slog.String("created_by", apiKey.CreatedBy))

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// DeleteAPIKey removes an API key by its key value
func (s *SQLiteStorage) DeleteAPIKey(key string) error {
	query := `DELETE FROM api_keys WHERE api_key = ?`

	result, err := s.db.Exec(query, key)
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("%w: API key not found", ErrNotFound)
	}

	s.logger.Info("API key deleted successfully", slog.String("key_prefix", key[:min(8, len(key))]+"***"))

	return nil
}

// RemoveAPIKeysAPI removes an API keys by apiId
func (s *SQLiteStorage) RemoveAPIKeysAPI(apiId string) error {
	query := `DELETE FROM api_keys WHERE apiId = ?`

	_, err := s.db.Exec(query, apiId)
	if err != nil {
		return fmt.Errorf("failed to remove API keys for API: %w", err)
	}

	s.logger.Info("API keys removed successfully",
		slog.String("apiId", apiId))

	return nil
}

// RemoveAPIKeyAPIAndName removes an API key by its apiId and name
func (s *SQLiteStorage) RemoveAPIKeyAPIAndName(apiId, name string) error {
	query := `DELETE FROM api_keys WHERE apiId = ? AND name = ?`

	result, err := s.db.Exec(query, apiId, name)
	if err != nil {
		return fmt.Errorf("failed to remove API key: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("%w: API key not found", ErrNotFound)
	}

	s.logger.Info("API key removed successfully",
		slog.String("apiId", apiId),
		slog.String("name", name))

	return nil
}

// Close closes the database connection
func (s *SQLiteStorage) Close() error {
	s.logger.Info("Closing SQLite storage")
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}
	return nil
}

// LoadFromDatabase loads all configurations from database into the in-memory cache
func LoadFromDatabase(storage Storage, cache *ConfigStore) error {
	// Get all configurations from persistent storage
	configs, err := storage.GetAllConfigs()
	if err != nil {
		return fmt.Errorf("failed to load configurations from database: %w", err)
	}

	// Load into in-memory cache
	for _, cfg := range configs {
		if err := cache.Add(cfg); err != nil {
			return fmt.Errorf("failed to load config %s into cache: %w", cfg.ID, err)
		}
	}

	return nil
}

// addDeploymentConfigs adds deployment configuration details to the database
func (s *SQLiteStorage) addDeploymentConfigs(cfg *models.StoredConfig) (bool, error) {
	query := `INSERT INTO deployment_configs (id, configuration, source_configuration) VALUES (?, ?, ?)`

	stmt, err := s.db.Prepare(query)
	if err != nil {
		return false, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	configJSON, err := json.Marshal(cfg.Configuration)
	if err != nil {
		return false, fmt.Errorf("failed to marshal configuration: %w", err)
	}
	sourceConfigJSON, err := json.Marshal(cfg.SourceConfiguration)
	if err != nil {
		return false, fmt.Errorf("failed to marshal source configuration: %w", err)
	}

	_, err = stmt.Exec(
		cfg.ID,
		string(configJSON),
		string(sourceConfigJSON),
	)

	if err != nil {
		return false, fmt.Errorf("failed to insert deployment configuration: %w", err)
	}

	return true, nil
}

// updateDeploymentConfigs updates deployment configuration details in the database
func (s *SQLiteStorage) updateDeploymentConfigs(cfg *models.StoredConfig) (bool, error) {
	query := `UPDATE deployment_configs SET configuration = ?, source_configuration = ? WHERE id = ?`

	stmt, err := s.db.Prepare(query)
	if err != nil {
		return false, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	configJSON, err := json.Marshal(cfg.Configuration)
	if err != nil {
		return false, fmt.Errorf("failed to marshal configuration: %w", err)
	}
	sourceConfigJSON, err := json.Marshal(cfg.SourceConfiguration)
	if err != nil {
		return false, fmt.Errorf("failed to marshal source configuration: %w", err)
	}

	result, err := stmt.Exec(
		string(configJSON),
		string(sourceConfigJSON),
		cfg.ID,
	)
	if err != nil {
		return false, fmt.Errorf("failed to update deployment configuration: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return false, fmt.Errorf("no deployment config found for id=%s", cfg.ID)
	}

	return true, nil
}

// isUniqueConstraintError checks if the error is a UNIQUE constraint violation
func isUniqueConstraintError(err error) bool {
	// SQLite error code 19 is CONSTRAINT error
	// Error message contains "UNIQUE constraint failed"
	return err != nil && (err.Error() == "UNIQUE constraint failed: deployments.display_name, deployments.version" ||
		err.Error() == "UNIQUE constraint failed: deployments.id" ||
		err.Error() == "UNIQUE constraint failed: deployments.handle")
}

// isCertificateUniqueConstraintError checks if the error is a UNIQUE constraint violation for certificates
func isCertificateUniqueConstraintError(err error) bool {
	// SQLite error code 19 is CONSTRAINT error
	// Error message contains "UNIQUE constraint failed"
	return err != nil && (err.Error() == "UNIQUE constraint failed: certificates.name" ||
		err.Error() == "UNIQUE constraint failed: certificates.id")
}

// Helper function to check for API key unique constraint errors
func isAPIKeyUniqueConstraintError(err error) bool {
	return err != nil &&
		(err.Error() == "UNIQUE constraint failed: api_keys.api_key" ||
			err.Error() == "UNIQUE constraint failed: api_keys.id" ||
			err.Error() == "UNIQUE constraint failed: index 'idx_unique_external_api_key'")
}

// GetAllAPIKeys retrieves all API keys from the database
func (s *SQLiteStorage) GetAllAPIKeys() ([]*models.APIKey, error) {
	query := `
		SELECT id, name, display_name, api_key, masked_api_key, apiId, operations, status,
		       created_at, created_by, updated_at, expires_at, source, external_ref_id, index_key
		FROM api_keys
		WHERE status = 'active'
		ORDER BY created_at DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query all API keys: %w", err)
	}
	defer rows.Close()

	var apiKeys []*models.APIKey

	for rows.Next() {
		var apiKey models.APIKey
		var expiresAt sql.NullTime
		var externalRefId sql.NullString
		var indexKey sql.NullString

		err := rows.Scan(
			&apiKey.ID,
			&apiKey.Name,
			&apiKey.DisplayName,
			&apiKey.APIKey,
			&apiKey.MaskedAPIKey,
			&apiKey.APIId,
			&apiKey.Operations,
			&apiKey.Status,
			&apiKey.CreatedAt,
			&apiKey.CreatedBy,
			&apiKey.UpdatedAt,
			&expiresAt,
			&apiKey.Source,
			&externalRefId,
			&indexKey,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan API key row: %w", err)
		}

		// Handle nullable fields
		if expiresAt.Valid {
			apiKey.ExpiresAt = &expiresAt.Time
		}
		if externalRefId.Valid {
			apiKey.ExternalRefId = &externalRefId.String
		}
		if indexKey.Valid {
			apiKey.IndexKey = &indexKey.String
		}

		apiKeys = append(apiKeys, &apiKey)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating API key rows: %w", err)
	}

	return apiKeys, nil
}

// LoadAPIKeysFromDatabase loads all API keys from database into both the ConfigStore and APIKeyStore
func LoadAPIKeysFromDatabase(storage Storage, configStore *ConfigStore, apiKeyStore *APIKeyStore) error {
	// Get all API keys from persistent storage
	apiKeys, err := storage.GetAllAPIKeys()
	if err != nil {
		return fmt.Errorf("failed to load API keys from database: %w", err)
	}

	// Load into both stores
	for _, apiKey := range apiKeys {
		// Load into ConfigStore for backward compatibility
		if err := configStore.StoreAPIKey(apiKey); err != nil {
			return fmt.Errorf("failed to load API key %s into ConfigStore: %w", apiKey.ID, err)
		}

		// Load into APIKeyStore for state-of-the-world updates
		if err := apiKeyStore.Store(apiKey); err != nil {
			return fmt.Errorf("failed to load API key %s into APIKeyStore: %w", apiKey.ID, err)
		}
	}

	return nil
}

// CountActiveAPIKeysByUserAndAPI counts active API keys for a specific user and API
func (s *SQLiteStorage) CountActiveAPIKeysByUserAndAPI(apiId, userID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM api_keys
		WHERE apiId = ? AND created_by = ? AND status = ?
	`

	var count int
	err := s.db.QueryRow(query, apiId, userID, models.APIKeyStatusActive).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count active API keys for user %s and API %s: %w", userID, apiId, err)
	}

	return count, nil
}
