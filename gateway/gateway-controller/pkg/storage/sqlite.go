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
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"go.uber.org/zap"
)

//go:embed gateway-controller-db.sql
var schemaSQL string

// SQLiteStorage implements the Storage interface using SQLite
type SQLiteStorage struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewSQLiteStorage creates a new SQLite storage instance
func NewSQLiteStorage(dbPath string, logger *zap.Logger) (*SQLiteStorage, error) {
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
		zap.String("database_path", dbPath),
		zap.String("journal_mode", "WAL"))

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
		s.logger.Info("Initializing database schema (version 1)")
		s.logger.Debug("Creating schema with SQL", zap.String("schema_sql", schemaSQL))

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
				name TEXT NOT NULL UNIQUE,
				configuration TEXT NOT NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			);`); err != nil {
				return fmt.Errorf("failed to migrate schema to version 4: %w", err)
			}
			if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_template_name ON llm_provider_templates(name);`); err != nil {
				return fmt.Errorf("failed to create llm_provider_templates index: %w", err)
			}
			if _, err := s.db.Exec("PRAGMA user_version = 4"); err != nil {
				return fmt.Errorf("failed to set schema version to 4: %w", err)
			}

			s.logger.Info("Schema migrated to version 4 (llm_provider_templates)")

			version = 4
		}

		s.logger.Info("Database schema up to date", zap.Int("version", version))
	}

	return nil
}

// SaveConfig persists a new deployment configuration
func (s *SQLiteStorage) SaveConfig(cfg *models.StoredConfig) error {
	// Extract fields for indexed columns
	name := cfg.GetName()
	version := cfg.GetVersion()
	context := cfg.GetContext()
	handle := cfg.GetHandle()

	if handle == "" {
		return fmt.Errorf("handle (metadata.name) is required and cannot be empty")
	}

	query := `
		INSERT INTO deployments (
			id, name, version, context, kind, handle,
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
		name,
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
			return fmt.Errorf("%w: configuration with name '%s' and version '%s' already exists", ErrConflict, name, version)
		}
		return fmt.Errorf("failed to insert configuration: %w", err)
	}

	_, err = s.addDeploymentConfigs(cfg)
	if err != nil {
		return fmt.Errorf("failed to add deployment configurations: %w", err)
	}

	s.logger.Info("Configuration saved",
		zap.String("id", cfg.ID),
		zap.String("name", name),
		zap.String("version", version))

	return nil
}

// UpdateConfig updates an existing deployment configuration
func (s *SQLiteStorage) UpdateConfig(cfg *models.StoredConfig) error {
	// Check if configuration exists
	_, err := s.GetConfig(cfg.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return fmt.Errorf("cannot update non-existent configuration: %w", err)
		}
		return err
	}

	// Extract fields for indexed columns
	name := cfg.GetName()
	version := cfg.GetVersion()
	context := cfg.GetContext()
	handle := cfg.GetHandle()

	if handle == "" {
		return fmt.Errorf("handle (metadata.name) is required and cannot be empty")
	}

	query := `
		UPDATE deployments
		SET name = ?, version = ?, context = ?, kind = ?, handle = ?,
			status = ?, updated_at = ?,
			deployed_version = ?
		WHERE id = ?
	`

	stmt, err := s.db.Prepare(query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(
		name,
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
		return fmt.Errorf("failed to update configuration: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("%w: id=%s", ErrNotFound, cfg.ID)
	}

	_, err = s.updateDeploymentConfigs(cfg)
	if err != nil {
		return fmt.Errorf("failed to update deployment configurations: %w", err)
	}

	s.logger.Info("Configuration updated",
		zap.String("id", cfg.ID),
		zap.String("name", name),
		zap.String("version", version))

	return nil
}

// DeleteConfig removes an deployment configuration by ID
func (s *SQLiteStorage) DeleteConfig(id string) error {
	query := `DELETE FROM deployments WHERE id = ?`

	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete configuration: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("%w: id=%s", ErrNotFound, id)
	}

	s.logger.Info("Configuration deleted", zap.String("id", id))

	return nil
}

// GetConfig retrieves an deployment configuration by ID
func (s *SQLiteStorage) GetConfig(id string) (*models.StoredConfig, error) {
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
			return nil, fmt.Errorf("%w: id=%s", ErrNotFound, id)
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

// GetConfigByNameVersion retrieves an deployment configuration by name and version
func (s *SQLiteStorage) GetConfigByNameVersion(name, version string) (*models.StoredConfig, error) {
	query := `
		SELECT d.id, d.kind, dc.configuration, dc.source_configuration, d.status, d.created_at, d.updated_at,
			   d.deployed_at, d.deployed_version
		FROM deployments d
		LEFT JOIN deployment_configs dc ON d.id = dc.id
		WHERE d.name = ? AND d.version = ?
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
			return fmt.Errorf("failed to load llm provider template %s into cache: %w", template.GetName(), err)
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

	name := template.GetName()

	query := `
		INSERT INTO llm_provider_templates (
			id, name, configuration, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?)
	`

	now := time.Now()
	_, err = s.db.Exec(query,
		template.ID,
		name,
		string(configJSON),
		now,
		now,
	)

	if err != nil {
		// Check for unique constraint violation
		if isUniqueConstraintError(err) || (err != nil && err.Error() == "UNIQUE constraint failed: llm_provider_templates.name") {
			return fmt.Errorf("%w: template with name '%s' already exists", ErrConflict, name)
		}
		return fmt.Errorf("failed to insert template: %w", err)
	}

	s.logger.Info("LLM provider template saved",
		zap.String("id", template.ID),
		zap.String("name", name))

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

	name := template.GetName()

	query := `
		UPDATE llm_provider_templates
		SET name = ?, configuration = ?, updated_at = ?
		WHERE id = ?
	`

	result, err := s.db.Exec(query,
		name,
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
		return fmt.Errorf("%w: id=%s", ErrNotFound, template.ID)
	}

	s.logger.Info("LLM provider template updated",
		zap.String("id", template.ID),
		zap.String("name", name))

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
		return fmt.Errorf("%w: id=%s", ErrNotFound, id)
	}

	s.logger.Info("LLM provider template deleted", zap.String("id", id))

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
			return nil, fmt.Errorf("%w: id=%s", ErrNotFound, id)
		}
		return nil, fmt.Errorf("failed to query template: %w", err)
	}

	// Deserialize JSON configuration
	if err := json.Unmarshal([]byte(configJSON), &template.Configuration); err != nil {
		return nil, fmt.Errorf("failed to unmarshal template configuration: %w", err)
	}

	return &template, nil
}

// GetLLMProviderTemplateByName retrieves an LLM provider template by name
func (s *SQLiteStorage) GetLLMProviderTemplateByName(name string) (*models.StoredLLMProviderTemplate, error) {
	query := `
		SELECT id, configuration, created_at, updated_at
		FROM llm_provider_templates
		WHERE name = ?
	`

	var template models.StoredLLMProviderTemplate
	var configJSON string

	err := s.db.QueryRow(query, name).Scan(
		&template.ID,
		&configJSON,
		&template.CreatedAt,
		&template.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: name=%s", ErrNotFound, name)
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
		s.logger.Debug("Certificate not found for deletion", zap.String("id", id))
		return ErrNotFound
	}

	s.logger.Info("Certificate deleted", zap.String("id", id))

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
	return err != nil && (err.Error() == "UNIQUE constraint failed: deployments.name, deployments.version" ||
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
