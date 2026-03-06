/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

type sqlStore struct {
	db *sql.DB

	logger *slog.Logger

	gatewayId string

	rebindQuery func(string) string

	isConfigUniqueViolation      func(error) bool
	isCertificateUniqueViolation func(error) bool
	isTemplateUniqueViolation    func(error) bool
	isAPIKeyUniqueViolation      func(error) bool

	backendName string
}

func newSQLStore(db *sql.DB, logger *slog.Logger, backendName string, gatewayId string) *sqlStore {
	return &sqlStore{
		db:          db,
		logger:      logger,
		gatewayId:   gatewayId,
		backendName: backendName,
		// Defaults are identity/false; backends can override.
		rebindQuery:                  func(query string) string { return query },
		isConfigUniqueViolation:      func(error) bool { return false },
		isCertificateUniqueViolation: func(error) bool { return false },
		isTemplateUniqueViolation:    func(error) bool { return false },
		isAPIKeyUniqueViolation:      func(error) bool { return false },
	}
}

func (s *sqlStore) bind(query string) string {
	if s.rebindQuery == nil {
		return query
	}
	return s.rebindQuery(query)
}

func (s *sqlStore) exec(query string, args ...interface{}) (sql.Result, error) {
	return s.db.Exec(s.bind(query), args...)
}

func (s *sqlStore) queryRow(query string, args ...interface{}) *sql.Row {
	return s.db.QueryRow(s.bind(query), args...)
}

func (s *sqlStore) query(query string, args ...interface{}) (*sql.Rows, error) {
	return s.db.Query(s.bind(query), args...)
}

func (s *sqlStore) prepare(query string) (*sql.Stmt, error) {
	return s.db.Prepare(s.bind(query))
}

func (s *sqlStore) begin() (*sqlStoreTx, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	return &sqlStoreTx{tx: tx, store: s}, nil
}

type sqlStoreTx struct {
	tx    *sql.Tx
	store *sqlStore
}

func (t *sqlStoreTx) ExecQ(query string, args ...interface{}) (sql.Result, error) {
	return t.tx.Exec(t.store.bind(query), args...)
}

func (t *sqlStoreTx) QueryRowQ(query string, args ...interface{}) *sql.Row {
	return t.tx.QueryRow(t.store.bind(query), args...)
}

func (t *sqlStoreTx) QueryQ(query string, args ...interface{}) (*sql.Rows, error) {
	return t.tx.Query(t.store.bind(query), args...)
}

func (t *sqlStoreTx) Commit() error {
	return t.tx.Commit()
}

func (t *sqlStoreTx) Rollback() error {
	return t.tx.Rollback()
}
func (s *sqlStore) SaveConfig(cfg *models.StoredConfig) error {
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
			id, gateway_id, display_name, version, context, kind, handle,
			status, created_at, updated_at, deployed_version
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	tx, err := s.begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.tx.Prepare(s.bind(query))
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now()
	_, err = stmt.Exec(
		cfg.ID,
		s.gatewayId,
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
		if s.isConfigUniqueViolation(err) {
			return fmt.Errorf("%w: configuration with displayName '%s' and version '%s' already exists", ErrConflict, displayName, version)
		}
		return fmt.Errorf("failed to insert configuration: %w", err)
	}

	_, err = s.addDeploymentConfigsTx(tx, cfg)
	if err != nil {
		return fmt.Errorf("failed to add deployment configurations: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit configuration transaction: %w", err)
	}
	committed = true

	s.logger.Info("Configuration saved",
		slog.String("id", cfg.ID),
		slog.String("displayName", displayName),
		slog.String("version", version))

	return nil
}

// UpdateConfig updates an existing deployment configuration
func (s *sqlStore) UpdateConfig(cfg *models.StoredConfig) error {
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
		WHERE id = ? AND gateway_id = ?
	`

	tx, err := s.begin()
	if err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("update", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("update", "tx_begin_error").Inc()
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.tx.Prepare(s.bind(query))
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
		s.gatewayId,
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

	_, err = s.updateDeploymentConfigsTx(tx, cfg)
	if err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("update", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("update", "deployment_config_error").Inc()
		return fmt.Errorf("failed to update deployment configurations: %w", err)
	}

	if err := tx.Commit(); err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("update", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("update", "commit_error").Inc()
		return fmt.Errorf("failed to commit configuration transaction: %w", err)
	}
	committed = true

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
func (s *sqlStore) DeleteConfig(id string) error {
	startTime := time.Now()
	table := "deployments"
	query := `DELETE FROM deployments WHERE id = ? AND gateway_id = ?`

	result, err := s.exec(query, id, s.gatewayId)
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
func (s *sqlStore) GetConfig(id string) (*models.StoredConfig, error) {
	startTime := time.Now()
	table := "deployments"
	query := `
		SELECT d.id, d.kind, dc.configuration, dc.source_configuration, d.status, d.created_at,
		d.updated_at, d.deployed_at, d.deployed_version
		FROM deployments d
		LEFT JOIN deployment_configs dc ON d.id = dc.id
		WHERE d.id = ? AND d.gateway_id = ?
	`

	var cfg models.StoredConfig
	var configJSON sql.NullString
	var sourceConfigJSON sql.NullString
	var deployedAt sql.NullTime

	err := s.queryRow(query, id, s.gatewayId).Scan(
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
	if configJSON.Valid && configJSON.String != "" {
		if err := json.Unmarshal([]byte(configJSON.String), &cfg.Configuration); err != nil {
			metrics.DatabaseOperationsTotal.WithLabelValues("read", table, "error").Inc()
			metrics.StorageErrorsTotal.WithLabelValues("read", "unmarshal_error").Inc()
			return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
		}
	}
	if sourceConfigJSON.Valid && sourceConfigJSON.String != "" {
		if err := json.Unmarshal([]byte(sourceConfigJSON.String), &cfg.SourceConfiguration); err != nil {
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
func (s *sqlStore) GetConfigByNameVersion(name, version string) (*models.StoredConfig, error) {
	query := `
		SELECT d.id, d.kind, dc.configuration, dc.source_configuration, d.status, d.created_at, d.updated_at,
			   d.deployed_at, d.deployed_version
		FROM deployments d
		LEFT JOIN deployment_configs dc ON d.id = dc.id
		WHERE d.display_name = ? AND d.version = ? AND d.gateway_id = ?
	`

	var cfg models.StoredConfig
	var configJSON sql.NullString
	var sourceConfigJSON sql.NullString
	var deployedAt sql.NullTime

	err := s.queryRow(query, name, version, s.gatewayId).Scan(
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
	if configJSON.Valid && configJSON.String != "" {
		if err := json.Unmarshal([]byte(configJSON.String), &cfg.Configuration); err != nil {
			return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
		}
	}
	if sourceConfigJSON.Valid && sourceConfigJSON.String != "" {
		if err := json.Unmarshal([]byte(sourceConfigJSON.String), &cfg.SourceConfiguration); err != nil {
			return nil, fmt.Errorf("failed to unmarshal source configuration: %w", err)
		}
	}

	return &cfg, nil
}

// GetConfigByHandle retrieves a deployment configuration by handle (metadata.name)
func (s *sqlStore) GetConfigByHandle(handle string) (*models.StoredConfig, error) {
	query := `
		SELECT d.id, d.kind, dc.configuration, dc.source_configuration, d.status, d.created_at, d.updated_at,
			   d.deployed_at, d.deployed_version
		FROM deployments d
		LEFT JOIN deployment_configs dc ON d.id = dc.id
		WHERE d.handle = ? AND d.gateway_id = ?
	`

	var cfg models.StoredConfig
	var configJSON sql.NullString
	var sourceConfigJSON sql.NullString
	var deployedAt sql.NullTime

	err := s.queryRow(query, handle, s.gatewayId).Scan(
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
	if configJSON.Valid && configJSON.String != "" {
		if err := json.Unmarshal([]byte(configJSON.String), &cfg.Configuration); err != nil {
			return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
		}
	}
	if sourceConfigJSON.Valid && sourceConfigJSON.String != "" {
		if err := json.Unmarshal([]byte(sourceConfigJSON.String), &cfg.SourceConfiguration); err != nil {
			return nil, fmt.Errorf("failed to unmarshal source configuration: %w", err)
		}
	}

	return &cfg, nil
}

// GetAllConfigs retrieves all deployment configurations
func (s *sqlStore) GetAllConfigs() ([]*models.StoredConfig, error) {
	query := `
			SELECT d.id, d.kind, dc.configuration, dc.source_configuration, d.status, 
			d.created_at, d.updated_at, d.deployed_at, d.deployed_version
			FROM deployments d
			LEFT JOIN deployment_configs dc ON d.id = dc.id
			WHERE d.gateway_id = ?
			ORDER BY d.created_at DESC
		`

	rows, err := s.query(query, s.gatewayId)
	if err != nil {
		return nil, fmt.Errorf("failed to query configurations: %w", err)
	}
	defer rows.Close()

	var configs []*models.StoredConfig

	for rows.Next() {
		var cfg models.StoredConfig
		var configJSON sql.NullString
		var sourceConfigJSON sql.NullString
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
		if configJSON.Valid && configJSON.String != "" {
			if err := json.Unmarshal([]byte(configJSON.String), &cfg.Configuration); err != nil {
				return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
			}
		}
		if sourceConfigJSON.Valid && sourceConfigJSON.String != "" {
			if err := json.Unmarshal([]byte(sourceConfigJSON.String), &cfg.SourceConfiguration); err != nil {
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
func (s *sqlStore) GetAllConfigsByKind(kind string) ([]*models.StoredConfig, error) {
	query := `
			SELECT d.id, d.kind, dc.configuration, dc.source_configuration, d.status, 
			d.created_at, d.updated_at, d.deployed_at, d.deployed_version
			FROM deployments d
			LEFT JOIN deployment_configs dc ON d.id = dc.id 
			WHERE d.kind = ? AND d.gateway_id = ?
			ORDER BY d.created_at DESC
		`

	rows, err := s.query(query, kind, s.gatewayId)
	if err != nil {
		return nil, fmt.Errorf("failed to query configurations: %w", err)
	}
	defer rows.Close()

	var configs []*models.StoredConfig

	for rows.Next() {
		var cfg models.StoredConfig
		var configJSON sql.NullString
		var sourceConfigJSON sql.NullString
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
		if configJSON.Valid && configJSON.String != "" {
			if err := json.Unmarshal([]byte(configJSON.String), &cfg.Configuration); err != nil {
				return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
			}
		}
		if sourceConfigJSON.Valid && sourceConfigJSON.String != "" {
			if err := json.Unmarshal([]byte(sourceConfigJSON.String), &cfg.SourceConfiguration); err != nil {
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

// SaveLLMProviderTemplate persists a new LLM provider template
func (s *sqlStore) SaveLLMProviderTemplate(template *models.StoredLLMProviderTemplate) error {
	// Serialize configuration to JSON
	configJSON, err := json.Marshal(template.Configuration)
	if err != nil {
		return fmt.Errorf("failed to marshal template configuration: %w", err)
	}

	handle := template.GetHandle()

	query := `
		INSERT INTO llm_provider_templates (
			id, gateway_id, handle, configuration, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	_, err = s.exec(query,
		template.ID,
		s.gatewayId,
		handle,
		string(configJSON),
		now,
		now,
	)

	if err != nil {
		// Check for unique constraint violation
		if s.isTemplateUniqueViolation(err) {
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
func (s *sqlStore) UpdateLLMProviderTemplate(template *models.StoredLLMProviderTemplate) error {
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
		WHERE id = ? AND gateway_id = ?
	`

	result, err := s.exec(query,
		handle,
		string(configJSON),
		time.Now(),
		template.ID,
		s.gatewayId,
	)

	if err != nil {
		if s.isTemplateUniqueViolation(err) {
			return fmt.Errorf("%w: template with handle '%s' already exists", ErrConflict, handle)
		}
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
func (s *sqlStore) DeleteLLMProviderTemplate(id string) error {
	query := `DELETE FROM llm_provider_templates WHERE id = ? AND gateway_id = ?`

	result, err := s.exec(query, id, s.gatewayId)
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
func (s *sqlStore) GetLLMProviderTemplate(id string) (*models.StoredLLMProviderTemplate, error) {
	query := `
		SELECT id, configuration, created_at, updated_at
		FROM llm_provider_templates
		WHERE id = ? AND gateway_id = ?
	`

	var template models.StoredLLMProviderTemplate
	var configJSON string

	err := s.queryRow(query, id, s.gatewayId).Scan(
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
func (s *sqlStore) GetAllLLMProviderTemplates() ([]*models.StoredLLMProviderTemplate, error) {
	query := `
		SELECT id, configuration, created_at, updated_at
		FROM llm_provider_templates
		WHERE gateway_id = ?
		ORDER BY created_at DESC
	`

	rows, err := s.query(query, s.gatewayId)
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
func (s *sqlStore) SaveCertificate(cert *models.StoredCertificate) error {
	query := `
		INSERT INTO certificates (
			id, gateway_id, name, certificate, subject, issuer,
			not_before, not_after, cert_count, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.exec(query,
		cert.ID,
		s.gatewayId,
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
		if s.isCertificateUniqueViolation(err) {
			return fmt.Errorf("%w: certificate with name '%s' already exists", ErrConflict, cert.Name)
		}
		return fmt.Errorf("failed to save certificate: %w", err)
	}

	return nil
}

// GetCertificate retrieves a certificate by ID
func (s *sqlStore) GetCertificate(id string) (*models.StoredCertificate, error) {
	query := `
		SELECT id, name, certificate, subject, issuer,
		       not_before, not_after, cert_count, created_at, updated_at
		FROM certificates
		WHERE id = ? AND gateway_id = ?
	`

	var cert models.StoredCertificate
	err := s.queryRow(query, id, s.gatewayId).Scan(
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
func (s *sqlStore) GetCertificateByName(name string) (*models.StoredCertificate, error) {
	query := `
		SELECT id, name, certificate, subject, issuer,
		       not_before, not_after, cert_count, created_at, updated_at
		FROM certificates
		WHERE name = ? AND gateway_id = ?
	`

	var cert models.StoredCertificate
	err := s.queryRow(query, name, s.gatewayId).Scan(
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
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get certificate by name: %w", err)
	}

	return &cert, nil
}

// ListCertificates retrieves all certificates
func (s *sqlStore) ListCertificates() ([]*models.StoredCertificate, error) {
	query := `
		SELECT id, name, certificate, subject, issuer,
		       not_before, not_after, cert_count, created_at, updated_at
		FROM certificates
		WHERE gateway_id = ?
		ORDER BY created_at DESC
	`

	rows, err := s.query(query, s.gatewayId)
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
func (s *sqlStore) DeleteCertificate(id string) error {
	query := `DELETE FROM certificates WHERE id = ? AND gateway_id = ?`

	result, err := s.exec(query, id, s.gatewayId)
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
func (s *sqlStore) SaveAPIKey(apiKey *models.APIKey) error {

	// Begin transaction to ensure atomicity
	tx, err := s.begin()
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

	// First, check if an API key with the same apiId and name exists
	checkQuery := `SELECT id FROM api_keys WHERE apiId = ? AND name = ? AND gateway_id = ?`
	var existingID string
	err = tx.QueryRowQ(checkQuery, apiKey.APIId, apiKey.Name, s.gatewayId).Scan(&existingID)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		tx.Rollback()
		return fmt.Errorf("failed to check existing API key: %w", err)
	}

	if errors.Is(err, sql.ErrNoRows) {
		// No existing record, insert new API key
		insertQuery := `
			INSERT INTO api_keys (
				id, gateway_id, name, display_name, api_key, masked_api_key, apiId, status,
				created_at, created_by, updated_at, expires_at, expires_in_unit, expires_in_duration,
				source, external_ref_id
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`

		_, err := tx.ExecQ(insertQuery,
			apiKey.ID,
			s.gatewayId,
			apiKey.Name,
			apiKey.DisplayName,
			apiKey.APIKey,
			apiKey.MaskedAPIKey,
			apiKey.APIId,
			apiKey.Status,
			apiKey.CreatedAt,
			apiKey.CreatedBy,
			apiKey.UpdatedAt,
			apiKey.ExpiresAt,
			apiKey.Unit,
			apiKey.Duration,
			apiKey.Source,
			apiKey.ExternalRefId,
		)

		if err != nil {
			tx.Rollback()
			// Check for unique constraint violation on api_key field
			if s.isAPIKeyUniqueViolation(err) {
				return fmt.Errorf("%w: API key value already exists", ErrConflict)
			}
			return fmt.Errorf("failed to insert API key: %w", err)
		}

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

	s.logger.Info("API key inserted successfully",
		slog.String("id", apiKey.ID),
		slog.String("name", apiKey.Name),
		slog.String("apiId", apiKey.APIId),
		slog.String("created_by", apiKey.CreatedBy))

	return nil
}

// GetAPIKeyByID retrieves an API key by its ID
func (s *sqlStore) GetAPIKeyByID(id string) (*models.APIKey, error) {
	query := `
		SELECT id, name, display_name, api_key, masked_api_key, apiId, status,
		       created_at, created_by, updated_at, expires_at, source, external_ref_id
		FROM api_keys
		WHERE id = ? AND gateway_id = ?
	`

	var apiKey models.APIKey
	var expiresAt sql.NullTime
	var externalRefId sql.NullString

	err := s.queryRow(query, id, s.gatewayId).Scan(
		&apiKey.ID,
		&apiKey.Name,
		&apiKey.DisplayName,
		&apiKey.APIKey,
		&apiKey.MaskedAPIKey,
		&apiKey.APIId,
		&apiKey.Status,
		&apiKey.CreatedAt,
		&apiKey.CreatedBy,
		&apiKey.UpdatedAt,
		&expiresAt,
		&apiKey.Source,
		&externalRefId,
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

	return &apiKey, nil
}

// GetAPIKeyByKey retrieves an API key by its key value
func (s *sqlStore) GetAPIKeyByKey(key string) (*models.APIKey, error) {
	query := `
		SELECT id, name, display_name, api_key, masked_api_key, apiId, status,
		       created_at, created_by, updated_at, expires_at, source, external_ref_id
		FROM api_keys
		WHERE api_key = ? AND gateway_id = ?
	`

	var apiKey models.APIKey
	var expiresAt sql.NullTime
	var externalRefId sql.NullString

	err := s.queryRow(query, key, s.gatewayId).Scan(
		&apiKey.ID,
		&apiKey.Name,
		&apiKey.DisplayName,
		&apiKey.APIKey,
		&apiKey.MaskedAPIKey,
		&apiKey.APIId,
		&apiKey.Status,
		&apiKey.CreatedAt,
		&apiKey.CreatedBy,
		&apiKey.UpdatedAt,
		&expiresAt,
		&apiKey.Source,
		&externalRefId,
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

	return &apiKey, nil
}

// GetAPIKeysByAPI retrieves all API keys for a specific API
func (s *sqlStore) GetAPIKeysByAPI(apiId string) ([]*models.APIKey, error) {
	query := `
		SELECT id, name, display_name, api_key, masked_api_key, apiId, status,
		       created_at, created_by, updated_at, expires_at, source, external_ref_id
		FROM api_keys
		WHERE apiId = ? AND gateway_id = ?
		ORDER BY created_at DESC
	`

	rows, err := s.query(query, apiId, s.gatewayId)
	if err != nil {
		return nil, fmt.Errorf("failed to query API keys: %w", err)
	}
	defer rows.Close()

	var apiKeys []*models.APIKey

	for rows.Next() {
		var apiKey models.APIKey
		var expiresAt sql.NullTime
		var externalRefId sql.NullString

		err := rows.Scan(
			&apiKey.ID,
			&apiKey.Name,
			&apiKey.DisplayName,
			&apiKey.APIKey,
			&apiKey.MaskedAPIKey,
			&apiKey.APIId,
			&apiKey.Status,
			&apiKey.CreatedAt,
			&apiKey.CreatedBy,
			&apiKey.UpdatedAt,
			&expiresAt,
			&apiKey.Source,
			&externalRefId,
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

		apiKeys = append(apiKeys, &apiKey)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating API key rows: %w", err)
	}

	return apiKeys, nil
}

// GetAPIKeysByAPIAndName retrieves an API key by its apiId and name
func (s *sqlStore) GetAPIKeysByAPIAndName(apiId, name string) (*models.APIKey, error) {
	query := `
		SELECT id, name, display_name, api_key, masked_api_key, apiId, status,
		       created_at, created_by, updated_at, expires_at, source, external_ref_id
		FROM api_keys
		WHERE apiId = ? AND name = ? AND gateway_id = ?
		LIMIT 1
	`

	var apiKey models.APIKey
	var expiresAt sql.NullTime
	var externalRefId sql.NullString

	err := s.queryRow(query, apiId, name, s.gatewayId).Scan(
		&apiKey.ID,
		&apiKey.Name,
		&apiKey.DisplayName,
		&apiKey.APIKey,
		&apiKey.MaskedAPIKey,
		&apiKey.APIId,
		&apiKey.Status,
		&apiKey.CreatedAt,
		&apiKey.CreatedBy,
		&apiKey.UpdatedAt,
		&expiresAt,
		&apiKey.Source,
		&externalRefId,
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

	return &apiKey, nil
}

// UpdateAPIKey updates an existing API key
func (s *sqlStore) UpdateAPIKey(apiKey *models.APIKey) error {

	// Begin transaction to ensure atomicity
	tx, err := s.begin()
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

	updateQuery := `
			UPDATE api_keys
			SET api_key = ?, masked_api_key = ?, display_name = ?, status = ?, created_by = ?, updated_at = ?, expires_at = ?, expires_in_unit = ?, expires_in_duration = ?,
			    source = ?, external_ref_id = ?
			WHERE apiId = ? AND name = ? AND gateway_id = ?
		`

	_, err = tx.ExecQ(updateQuery,
		apiKey.APIKey,
		apiKey.MaskedAPIKey,
		apiKey.DisplayName,
		apiKey.Status,
		apiKey.CreatedBy,
		apiKey.UpdatedAt,
		apiKey.ExpiresAt,
		apiKey.Unit,
		apiKey.Duration,
		apiKey.Source,
		apiKey.ExternalRefId,
		apiKey.APIId,
		apiKey.Name,
		s.gatewayId,
	)

	if err != nil {
		tx.Rollback()
		// Check for unique constraint violation on api_key field
		if s.isAPIKeyUniqueViolation(err) {
			return fmt.Errorf("%w: API key value already exists", ErrConflict)
		}
		return fmt.Errorf("failed to update API key: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info("API key updated successfully",
		slog.String("name", apiKey.Name),
		slog.String("apiId", apiKey.APIId),
		slog.String("created_by", apiKey.CreatedBy))

	return nil
}

// DeleteAPIKey removes an API key by its key value
func (s *sqlStore) DeleteAPIKey(key string) error {
	query := `DELETE FROM api_keys WHERE api_key = ? AND gateway_id = ?`

	result, err := s.exec(query, key, s.gatewayId)
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
func (s *sqlStore) RemoveAPIKeysAPI(apiId string) error {
	query := `DELETE FROM api_keys WHERE apiId = ? AND gateway_id = ?`

	_, err := s.exec(query, apiId, s.gatewayId)
	if err != nil {
		return fmt.Errorf("failed to remove API keys for API: %w", err)
	}

	s.logger.Info("API keys removed successfully",
		slog.String("apiId", apiId))

	return nil
}

// RemoveAPIKeyAPIAndName removes an API key by its apiId and name
func (s *sqlStore) RemoveAPIKeyAPIAndName(apiId, name string) error {
	query := `DELETE FROM api_keys WHERE apiId = ? AND name = ? AND gateway_id = ?`

	result, err := s.exec(query, apiId, name, s.gatewayId)
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
func (s *sqlStore) Close() error {
	backend := s.backendName
	if backend == "" {
		backend = "SQL"
	}
	s.logger.Info("Closing storage", slog.String("backend", backend))
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}
	return nil
}

func (s *sqlStore) addDeploymentConfigsTx(tx *sqlStoreTx, cfg *models.StoredConfig) (bool, error) {
	query := `INSERT INTO deployment_configs (id, configuration, source_configuration) VALUES (?, ?, ?)`

	stmt, err := tx.tx.Prepare(s.bind(query))
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

func (s *sqlStore) updateDeploymentConfigsTx(tx *sqlStoreTx, cfg *models.StoredConfig) (bool, error) {
	query := `UPDATE deployment_configs SET configuration = ?, source_configuration = ? WHERE id = ?`

	stmt, err := tx.tx.Prepare(s.bind(query))
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

// GetAllAPIKeys retrieves all active API keys from the database.
func (s *sqlStore) GetAllAPIKeys() ([]*models.APIKey, error) {
	query := `
		SELECT id, name, display_name, api_key, masked_api_key, apiId, status,
		       created_at, created_by, updated_at, expires_at, source, external_ref_id
		FROM api_keys
		WHERE status = 'active' AND gateway_id = ?
		ORDER BY created_at DESC
	`

	rows, err := s.query(query, s.gatewayId)
	if err != nil {
		return nil, fmt.Errorf("failed to query all API keys: %w", err)
	}
	defer rows.Close()

	var apiKeys []*models.APIKey

	for rows.Next() {
		var apiKey models.APIKey
		var expiresAt sql.NullTime
		var externalRefId sql.NullString

		err := rows.Scan(
			&apiKey.ID,
			&apiKey.Name,
			&apiKey.DisplayName,
			&apiKey.APIKey,
			&apiKey.MaskedAPIKey,
			&apiKey.APIId,
			&apiKey.Status,
			&apiKey.CreatedAt,
			&apiKey.CreatedBy,
			&apiKey.UpdatedAt,
			&expiresAt,
			&apiKey.Source,
			&externalRefId,
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

		apiKeys = append(apiKeys, &apiKey)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating API key rows: %w", err)
	}

	return apiKeys, nil
}

// CountActiveAPIKeysByUserAndAPI counts active API keys for a specific user and API
func (s *sqlStore) CountActiveAPIKeysByUserAndAPI(apiId, userID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM api_keys
		WHERE apiId = ? AND created_by = ? AND status = ? AND gateway_id = ?
	`

	var count int
	err := s.queryRow(query, apiId, userID, models.APIKeyStatusActive, s.gatewayId).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count active API keys for user %s and API %s: %w", userID, apiId, err)
	}

	return count, nil
}
