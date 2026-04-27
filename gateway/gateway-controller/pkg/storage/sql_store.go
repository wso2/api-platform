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
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// hashSubscriptionToken computes a SHA-256 hash of the token for secure storage.
// The same token always produces the same hash for deterministic lookups.
func hashSubscriptionToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

type sqlStore struct {
	db *sql.DB

	logger *slog.Logger

	gatewayId string

	rebindQuery func(string) string

	isUniqueViolation func(error) bool

	backendName string
}

func newSQLStore(db *sql.DB, logger *slog.Logger, backendName string, gatewayId string) *sqlStore {
	return &sqlStore{
		db:          db,
		logger:      logger,
		gatewayId:   gatewayId,
		backendName: backendName,
		// Defaults are identity/false; backends can override.
		rebindQuery:       func(query string) string { return query },
		isUniqueViolation: func(error) bool { return false },
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

func (s *sqlStore) rollbackTx(tx *sqlStoreTx, reason string) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		s.logger.Warn("Failed to rollback transaction",
			slog.String("reason", reason),
			slog.Any("error", err))
	}
}

// kindToResourceTable maps a kind string to its per-type table name.
func kindToResourceTable(kind string) (string, error) {
	switch kind {
	case "RestApi":
		return "rest_apis", nil
	case "WebSubApi":
		return "websub_apis", nil
	case "LlmProvider":
		return "llm_providers", nil
	case "LlmProxy":
		return "llm_proxies", nil
	case "Mcp":
		return "mcp_proxies", nil
	default:
		return "", fmt.Errorf("unknown kind: %s", kind)
	}
}

// unmarshalSourceConfig unmarshals JSON into the correct typed struct for the given kind.
// RestApi/WebSubApi rows can populate Configuration directly because the stored
// payload is already the deployable shape. LLM provider/proxy rows only restore
// SourceConfiguration; their derived RestAPI form is rebuilt later by the
// deployment/event-listener layer once templates and policies are available.
func unmarshalSourceConfig(cfg *models.StoredConfig, jsonData string) error {
	switch cfg.Kind {
	case "RestApi":
		var config api.RestAPI
		if err := json.Unmarshal([]byte(jsonData), &config); err != nil {
			return fmt.Errorf("failed to unmarshal configuration: %w", err)
		}
		cfg.SourceConfiguration = config
		cfg.Configuration = config
	case "WebSubApi":
		var config api.WebSubAPI
		if err := json.Unmarshal([]byte(jsonData), &config); err != nil {
			return fmt.Errorf("failed to unmarshal configuration: %w", err)
		}
		cfg.SourceConfiguration = config
		cfg.Configuration = config
	case "LlmProvider":
		var config api.LLMProviderConfiguration
		if err := json.Unmarshal([]byte(jsonData), &config); err != nil {
			return fmt.Errorf("failed to unmarshal source configuration: %w", err)
		}
		cfg.SourceConfiguration = config
		cfg.Configuration = config
	case "LlmProxy":
		var config api.LLMProxyConfiguration
		if err := json.Unmarshal([]byte(jsonData), &config); err != nil {
			return fmt.Errorf("failed to unmarshal source configuration: %w", err)
		}
		cfg.SourceConfiguration = config
		cfg.Configuration = config
	case "Mcp":
		var config api.MCPProxyConfiguration
		if err := json.Unmarshal([]byte(jsonData), &config); err != nil {
			return fmt.Errorf("failed to unmarshal source configuration: %w", err)
		}
		cfg.SourceConfiguration = config
	default:
		return fmt.Errorf("unknown kind: %s", cfg.Kind)
	}
	return nil
}

func (s *sqlStore) SaveConfig(cfg *models.StoredConfig) error {
	if cfg.Handle == "" {
		return fmt.Errorf("handle (metadata.name) is required and cannot be empty")
	}

	query := `
		INSERT INTO artifacts (
			uuid, gateway_id, display_name, version, kind, handle,
			desired_state, deployment_id, origin, created_at, updated_at, deployed_at,
			cp_sync_status, cp_sync_info, cp_artifact_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
	var deploymentID interface{}
	if cfg.DeploymentID != "" {
		deploymentID = cfg.DeploymentID
	}
	var deployedAt interface{}
	if cfg.DeployedAt != nil && !cfg.DeployedAt.IsZero() {
		deployedAt = *cfg.DeployedAt
	}
	var cpSyncStatus interface{}
	if cfg.CPSyncStatus != "" {
		cpSyncStatus = cfg.CPSyncStatus
	}
	var cpSyncInfo interface{}
	if cfg.CPSyncInfo != "" {
		cpSyncInfo = cfg.CPSyncInfo
	}
	var cpArtifactID interface{}
	if cfg.CPArtifactID != "" {
		cpArtifactID = cfg.CPArtifactID
	}
	_, err = stmt.Exec(
		cfg.UUID,
		s.gatewayId,
		cfg.DisplayName,
		cfg.Version,
		cfg.Kind,
		cfg.Handle,
		cfg.DesiredState,
		deploymentID,
		cfg.Origin,
		now,
		now,
		deployedAt,
		cpSyncStatus,
		cpSyncInfo,
		cpArtifactID,
	)

	if err != nil {
		// Check for unique constraint violation
		if s.isUniqueViolation(err) {
			return fmt.Errorf("%w: configuration with displayName '%s' and version '%s' already exists", ErrConflict, cfg.DisplayName, cfg.Version)
		}
		return fmt.Errorf("failed to insert configuration: %w", err)
	}

	_, err = s.addResourceConfigTx(tx, cfg)
	if err != nil {
		return fmt.Errorf("failed to add resource configuration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit configuration transaction: %w", err)
	}
	committed = true

	s.logger.Info("Configuration saved",
		slog.String("uuid", cfg.UUID),
		slog.String("kind", cfg.Kind),
		slog.String("handle", cfg.Handle))

	return nil
}

// UpdateConfig updates an existing deployment configuration
func (s *sqlStore) UpdateConfig(cfg *models.StoredConfig) error {
	startTime := time.Now()
	table := "artifacts"

	// Check if configuration exists
	_, err := s.GetConfig(cfg.UUID)
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

	if cfg.Handle == "" {
		metrics.DatabaseOperationsTotal.WithLabelValues("update", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("update", "validation_error").Inc()
		return fmt.Errorf("handle (metadata.name) is required and cannot be empty")
	}

	query := `
		UPDATE artifacts
		SET display_name = ?, version = ?, kind = ?, handle = ?,
			desired_state = ?, deployment_id = ?, origin = ?, updated_at = ?, deployed_at = ?,
			cp_sync_status = ?, cp_sync_info = ?, cp_artifact_id = ?
		WHERE uuid = ? AND gateway_id = ?
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

	var updateDeploymentID interface{}
	if cfg.DeploymentID != "" {
		updateDeploymentID = cfg.DeploymentID
	}
	var updateDeployedAt interface{}
	if cfg.DeployedAt != nil && !cfg.DeployedAt.IsZero() {
		updateDeployedAt = *cfg.DeployedAt
	}
	var updateCPSyncStatus interface{}
	if cfg.CPSyncStatus != "" {
		updateCPSyncStatus = cfg.CPSyncStatus
	}
	var updateCPSyncInfo interface{}
	if cfg.CPSyncInfo != "" {
		updateCPSyncInfo = cfg.CPSyncInfo
	}
	var updateCPArtifactID interface{}
	if cfg.CPArtifactID != "" {
		updateCPArtifactID = cfg.CPArtifactID
	}
	result, err := stmt.Exec(
		cfg.DisplayName,
		cfg.Version,
		cfg.Kind,
		cfg.Handle,
		cfg.DesiredState,
		updateDeploymentID,
		cfg.Origin,
		time.Now(),
		updateDeployedAt,
		updateCPSyncStatus,
		updateCPSyncInfo,
		updateCPArtifactID,
		cfg.UUID,
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
		return fmt.Errorf("%w: uuid=%s", ErrNotFound, cfg.UUID)
	}

	_, err = s.updateResourceConfigTx(tx, cfg)
	if err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("update", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("update", "resource_config_error").Inc()
		return fmt.Errorf("failed to update resource configuration: %w", err)
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
		slog.String("uuid", cfg.UUID),
		slog.String("displayName", cfg.DisplayName),
		slog.String("version", cfg.Version))

	return nil
}

// UpsertConfig performs a timestamp-guarded insert-or-update.
// The artifact row is only written if no existing row has a newer deployed_at.
// Returns (true, nil) when the DB was actually modified, (false, nil) when a
// newer version already exists (stale event), or (false, error) on failure.
func (s *sqlStore) UpsertConfig(cfg *models.StoredConfig) (bool, error) {
	startTime := time.Now()
	table := "artifacts"

	if cfg.Handle == "" {
		return false, fmt.Errorf("handle (metadata.name) is required and cannot be empty")
	}

	// INSERT ... ON CONFLICT upsert with deployed_at guard.
	// The WHERE clause ensures we only overwrite when the incoming deployed_at
	// is strictly newer than the stored value (or the stored value is NULL).
	query := `
		INSERT INTO artifacts (
			uuid, gateway_id, display_name, version, kind, handle,
			desired_state, deployment_id, origin, created_at, updated_at, deployed_at,
			cp_sync_status, cp_sync_info, cp_artifact_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(gateway_id, uuid) DO UPDATE SET
			display_name  = excluded.display_name,
			version       = excluded.version,
			kind          = excluded.kind,
			handle        = excluded.handle,
			desired_state = excluded.desired_state,
			deployment_id = excluded.deployment_id,
			origin        = excluded.origin,
			updated_at    = excluded.updated_at,
			deployed_at   = excluded.deployed_at
		WHERE artifacts.deployed_at IS NULL
		   OR artifacts.deployed_at < excluded.deployed_at
	`

	tx, err := s.begin()
	if err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("upsert", table, "error").Inc()
		return false, fmt.Errorf("failed to begin transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.tx.Prepare(s.bind(query))
	if err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("upsert", table, "error").Inc()
		return false, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now()
	var deploymentID interface{}
	if cfg.DeploymentID != "" {
		deploymentID = cfg.DeploymentID
	}

	var upsertCPSyncStatus interface{}
	if cfg.CPSyncStatus != "" {
		upsertCPSyncStatus = cfg.CPSyncStatus
	}
	var upsertCPSyncInfo interface{}
	if cfg.CPSyncInfo != "" {
		upsertCPSyncInfo = cfg.CPSyncInfo
	}
	var upsertCPArtifactID interface{}
	if cfg.CPArtifactID != "" {
		upsertCPArtifactID = cfg.CPArtifactID
	}
	result, err := stmt.Exec(
		cfg.UUID,
		s.gatewayId,
		cfg.DisplayName,
		cfg.Version,
		cfg.Kind,
		cfg.Handle,
		cfg.DesiredState,
		deploymentID,
		cfg.Origin,
		now,
		now,
		cfg.DeployedAt,
		upsertCPSyncStatus,
		upsertCPSyncInfo,
		upsertCPArtifactID,
	)
	if err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("upsert", table, "error").Inc()
		return false, fmt.Errorf("failed to upsert artifact: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("upsert", table, "error").Inc()
		return false, fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		// Stale event — existing row has a newer deployed_at. No-op.
		_ = tx.Rollback()
		committed = true // prevent double-rollback in defer
		metrics.DatabaseOperationsTotal.WithLabelValues("upsert", table, "skipped").Inc()
		s.logger.Info("Upsert skipped (stale event)",
			slog.String("uuid", cfg.UUID),
			slog.String("deployment_id", cfg.DeploymentID))
		return false, nil
	}

	// Upsert the per-resource-type table (configuration JSON).
	if err := s.upsertResourceConfigTx(tx, cfg); err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("upsert", table, "error").Inc()
		return false, fmt.Errorf("failed to upsert resource configuration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("upsert", table, "error").Inc()
		return false, fmt.Errorf("failed to commit upsert transaction: %w", err)
	}
	committed = true

	metrics.DatabaseOperationsTotal.WithLabelValues("upsert", table, "success").Inc()
	metrics.DatabaseOperationDurationSeconds.WithLabelValues("upsert", table).Observe(time.Since(startTime).Seconds())

	s.logger.Info("Configuration upserted",
		slog.String("uuid", cfg.UUID),
		slog.String("kind", cfg.Kind),
		slog.String("handle", cfg.Handle),
		slog.String("deployment_id", cfg.DeploymentID))

	return true, nil
}

// DeleteConfig removes an artifact configuration by UUID
func (s *sqlStore) DeleteConfig(id string) error {
	startTime := time.Now()
	table := "artifacts"
	tx, err := s.begin()
	if err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("delete", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("delete", "begin_error").Inc()
		return fmt.Errorf("failed to begin delete transaction: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			s.rollbackTx(tx, "delete configuration transaction not committed")
		}
	}()

	if _, err := tx.ExecQ(`DELETE FROM subscriptions WHERE gateway_id = ? AND api_id = ?`, s.gatewayId, id); err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("delete", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("delete", "cleanup_subscriptions_error").Inc()
		return fmt.Errorf("failed to delete subscriptions for configuration: %w", err)
	}

	if _, err := tx.ExecQ(`DELETE FROM api_keys WHERE gateway_id = ? AND artifact_uuid = ?`, s.gatewayId, id); err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("delete", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("delete", "cleanup_api_keys_error").Inc()
		return fmt.Errorf("failed to delete API keys for configuration: %w", err)
	}

	result, err := tx.ExecQ(`DELETE FROM artifacts WHERE uuid = ? AND gateway_id = ?`, id, s.gatewayId)
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

	if err := tx.Commit(); err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("delete", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("delete", "commit_error").Inc()
		return fmt.Errorf("failed to commit delete transaction: %w", err)
	}
	committed = true

	// Record successful metrics
	metrics.DatabaseOperationsTotal.WithLabelValues("delete", table, "success").Inc()
	metrics.DatabaseOperationDurationSeconds.WithLabelValues("delete", table).Observe(time.Since(startTime).Seconds())

	s.logger.Info("Configuration deleted", slog.String("uuid", id))

	return nil
}

// GetConfig retrieves an artifact configuration by UUID
func (s *sqlStore) GetConfig(id string) (*models.StoredConfig, error) {
	startTime := time.Now()
	table := "artifacts"

	// Step 1: Get artifact base record
	artifactQuery := `
		SELECT uuid, kind, handle, display_name, version, desired_state, deployment_id, origin, created_at, updated_at, deployed_at,
			cp_sync_status, cp_sync_info, cp_artifact_id
		FROM artifacts
		WHERE uuid = ? AND gateway_id = ?
	`

	var cfg models.StoredConfig
	var deployedAt sql.NullTime
	var deploymentID sql.NullString
	var cpSyncStatus sql.NullString
	var cpSyncInfo sql.NullString
	var cpArtifactID sql.NullString

	err := s.queryRow(artifactQuery, id, s.gatewayId).Scan(
		&cfg.UUID,
		&cfg.Kind,
		&cfg.Handle,
		&cfg.DisplayName,
		&cfg.Version,
		&cfg.DesiredState,
		&deploymentID,
		&cfg.Origin,
		&cfg.CreatedAt,
		&cfg.UpdatedAt,
		&deployedAt,
		&cpSyncStatus,
		&cpSyncInfo,
		&cpArtifactID,
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

	if deployedAt.Valid {
		cfg.DeployedAt = &deployedAt.Time
	}
	if deploymentID.Valid {
		cfg.DeploymentID = deploymentID.String
	}
	if cpSyncStatus.Valid {
		cfg.CPSyncStatus = models.CPSyncStatus(cpSyncStatus.String)
	}
	if cpSyncInfo.Valid {
		cfg.CPSyncInfo = cpSyncInfo.String
	}
	if cpArtifactID.Valid {
		cfg.CPArtifactID = cpArtifactID.String
	}

	// Step 2: Get configuration from the correct type table
	if err := s.loadResourceConfig(&cfg); err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("read", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("read", "unmarshal_error").Inc()
		return nil, err
	}

	// Record successful metrics
	metrics.DatabaseOperationsTotal.WithLabelValues("read", table, "success").Inc()
	metrics.DatabaseOperationDurationSeconds.WithLabelValues("read", table).Observe(time.Since(startTime).Seconds())

	return &cfg, nil
}

// GetConfigByKindAndHandle retrieves a deployment configuration by kind and handle (metadata.name)
func (s *sqlStore) GetConfigByKindAndHandle(kind string, handle string) (*models.StoredConfig, error) {
	artifactQuery := `
		SELECT uuid, kind, handle, display_name, version, desired_state, deployment_id, origin, created_at, updated_at, deployed_at,
			cp_sync_status, cp_sync_info, cp_artifact_id
		FROM artifacts
		WHERE kind = ? AND handle = ? AND gateway_id = ?
	`

	var cfg models.StoredConfig
	var deployedAt sql.NullTime
	var deploymentID sql.NullString
	var cpSyncStatus sql.NullString
	var cpSyncInfo sql.NullString
	var cpArtifactID sql.NullString

	err := s.queryRow(artifactQuery, kind, handle, s.gatewayId).Scan(
		&cfg.UUID,
		&cfg.Kind,
		&cfg.Handle,
		&cfg.DisplayName,
		&cfg.Version,
		&cfg.DesiredState,
		&deploymentID,
		&cfg.Origin,
		&cfg.CreatedAt,
		&cfg.UpdatedAt,
		&deployedAt,
		&cpSyncStatus,
		&cpSyncInfo,
		&cpArtifactID,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: handle=%s", ErrNotFound, handle)
		}
		return nil, fmt.Errorf("failed to query configuration: %w", err)
	}

	if deployedAt.Valid {
		cfg.DeployedAt = &deployedAt.Time
	}
	if deploymentID.Valid {
		cfg.DeploymentID = deploymentID.String
	}
	if cpSyncStatus.Valid {
		cfg.CPSyncStatus = models.CPSyncStatus(cpSyncStatus.String)
	}
	if cpSyncInfo.Valid {
		cfg.CPSyncInfo = cpSyncInfo.String
	}
	if cpArtifactID.Valid {
		cfg.CPArtifactID = cpArtifactID.String
	}

	if err := s.loadResourceConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// GetConfigByKindNameAndVersion retrieves a deployment configuration by kind, display name, and version.
func (s *sqlStore) GetConfigByKindNameAndVersion(kind, displayName, version string) (*models.StoredConfig, error) {
	artifactQuery := `
		SELECT uuid, kind, handle, display_name, version, desired_state, deployment_id, origin, created_at, updated_at, deployed_at,
			cp_sync_status, cp_sync_info, cp_artifact_id
		FROM artifacts
		WHERE kind = ? AND display_name = ? AND version = ? AND gateway_id = ?
	`

	var cfg models.StoredConfig
	var deployedAt sql.NullTime
	var deploymentID sql.NullString
	var cpSyncStatus sql.NullString
	var cpSyncInfo sql.NullString
	var cpArtifactID sql.NullString

	err := s.queryRow(artifactQuery, kind, displayName, version, s.gatewayId).Scan(
		&cfg.UUID,
		&cfg.Kind,
		&cfg.Handle,
		&cfg.DisplayName,
		&cfg.Version,
		&cfg.DesiredState,
		&deploymentID,
		&cfg.Origin,
		&cfg.CreatedAt,
		&cfg.UpdatedAt,
		&deployedAt,
		&cpSyncStatus,
		&cpSyncInfo,
		&cpArtifactID,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: display_name=%s version=%s", ErrNotFound, displayName, version)
		}
		return nil, fmt.Errorf("failed to query configuration: %w", err)
	}

	if deployedAt.Valid {
		cfg.DeployedAt = &deployedAt.Time
	}
	if deploymentID.Valid {
		cfg.DeploymentID = deploymentID.String
	}
	if cpSyncStatus.Valid {
		cfg.CPSyncStatus = models.CPSyncStatus(cpSyncStatus.String)
	}
	if cpSyncInfo.Valid {
		cfg.CPSyncInfo = cpSyncInfo.String
	}
	if cpArtifactID.Valid {
		cfg.CPArtifactID = cpArtifactID.String
	}

	if err := s.loadResourceConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// GetAllConfigs retrieves all artifact configurations
// TODO: (renuka) Remove this method once the in memory cache is removed.
func (s *sqlStore) GetAllConfigs() ([]*models.StoredConfig, error) {
	// Use UNION ALL across all type tables joined with artifacts
	query := `
			SELECT a.uuid, a.kind, a.handle, a.display_name, a.version, r.configuration, a.desired_state,
				a.deployment_id, a.origin, a.created_at, a.updated_at, a.deployed_at,
				a.cp_sync_status, a.cp_sync_info, a.cp_artifact_id
			FROM artifacts a
			JOIN rest_apis r ON a.uuid = r.uuid AND a.gateway_id = r.gateway_id
			WHERE a.gateway_id = ?

		UNION ALL

			SELECT a.uuid, a.kind, a.handle, a.display_name, a.version, w.configuration, a.desired_state,
				a.deployment_id, a.origin, a.created_at, a.updated_at, a.deployed_at,
				a.cp_sync_status, a.cp_sync_info, a.cp_artifact_id
			FROM artifacts a
			JOIN websub_apis w ON a.uuid = w.uuid AND a.gateway_id = w.gateway_id
			WHERE a.gateway_id = ?

		UNION ALL

			SELECT a.uuid, a.kind, a.handle, a.display_name, a.version, lp.configuration, a.desired_state,
				a.deployment_id, a.origin, a.created_at, a.updated_at, a.deployed_at,
				a.cp_sync_status, a.cp_sync_info, a.cp_artifact_id
			FROM artifacts a
			JOIN llm_providers lp ON a.uuid = lp.uuid AND a.gateway_id = lp.gateway_id
			WHERE a.gateway_id = ?

		UNION ALL

			SELECT a.uuid, a.kind, a.handle, a.display_name, a.version, lx.configuration, a.desired_state,
				a.deployment_id, a.origin, a.created_at, a.updated_at, a.deployed_at,
				a.cp_sync_status, a.cp_sync_info, a.cp_artifact_id
			FROM artifacts a
			JOIN llm_proxies lx ON a.uuid = lx.uuid AND a.gateway_id = lx.gateway_id
			WHERE a.gateway_id = ?

		UNION ALL

		SELECT a.uuid, a.kind, a.handle, a.display_name, a.version, m.configuration, a.desired_state,
			a.deployment_id, a.origin, a.created_at, a.updated_at, a.deployed_at,
			a.cp_sync_status, a.cp_sync_info, a.cp_artifact_id
		FROM artifacts a
		JOIN mcp_proxies m ON a.uuid = m.uuid AND a.gateway_id = m.gateway_id
		WHERE a.gateway_id = ?
		ORDER BY created_at DESC
	`

	rows, err := s.query(query, s.gatewayId, s.gatewayId, s.gatewayId, s.gatewayId, s.gatewayId)
	if err != nil {
		return nil, fmt.Errorf("failed to query configurations: %w", err)
	}
	defer rows.Close()

	return s.scanConfigRows(rows)
}

// GetAllConfigsByKind retrieves all artifact configurations of a specific kind
func (s *sqlStore) GetAllConfigsByKind(kind string) ([]*models.StoredConfig, error) {
	resourceTable, err := kindToResourceTable(kind)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
			SELECT a.uuid, a.kind, a.handle, a.display_name, a.version, r.configuration, a.desired_state,
				a.deployment_id, a.origin, a.created_at, a.updated_at, a.deployed_at,
				a.cp_sync_status, a.cp_sync_info, a.cp_artifact_id
			FROM artifacts a
			JOIN %s r ON a.uuid = r.uuid AND a.gateway_id = r.gateway_id
			WHERE a.kind = ? AND a.gateway_id = ?
			ORDER BY a.created_at DESC
		`, resourceTable)

	rows, err := s.query(query, kind, s.gatewayId)
	if err != nil {
		return nil, fmt.Errorf("failed to query configurations: %w", err)
	}
	defer rows.Close()

	return s.scanConfigRows(rows)
}

// scanConfigRows scans rows from a query that returns (uuid, kind, handle, display_name, version, configuration, desired_state, deployment_id, origin, created_at, updated_at, deployed_at, enable_cp_sync, cp_sync_status, cp_sync_info)
func (s *sqlStore) scanConfigRows(rows *sql.Rows) ([]*models.StoredConfig, error) {
	var configs []*models.StoredConfig

	for rows.Next() {
		var cfg models.StoredConfig
		var configJSON sql.NullString
		var deployedAt sql.NullTime
		var deploymentID sql.NullString
		var cpSyncStatus sql.NullString
		var cpSyncInfo sql.NullString
		var cpArtifactID sql.NullString

		err := rows.Scan(
			&cfg.UUID,
			&cfg.Kind,
			&cfg.Handle,
			&cfg.DisplayName,
			&cfg.Version,
			&configJSON,
			&cfg.DesiredState,
			&deploymentID,
			&cfg.Origin,
			&cfg.CreatedAt,
			&cfg.UpdatedAt,
			&deployedAt,
			&cpSyncStatus,
			&cpSyncInfo,
			&cpArtifactID,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if deployedAt.Valid {
			cfg.DeployedAt = &deployedAt.Time
		}
		if deploymentID.Valid {
			cfg.DeploymentID = deploymentID.String
		}
		if cpSyncStatus.Valid {
			cfg.CPSyncStatus = models.CPSyncStatus(cpSyncStatus.String)
		}
		if cpSyncInfo.Valid {
			cfg.CPSyncInfo = cpSyncInfo.String
		}
		if cpArtifactID.Valid {
			cfg.CPArtifactID = cpArtifactID.String
		}

		if configJSON.Valid && configJSON.String != "" {
			if err := unmarshalSourceConfig(&cfg, configJSON.String); err != nil {
				return nil, err
			}
		}

		configs = append(configs, &cfg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return configs, nil
}

// GetAllConfigsByOrigin retrieves artifact metadata (without full configuration) for all
// configs with the given origin. Queries only the artifacts table — no resource-table JOINs —
// so Configuration/SourceConfiguration will be nil.
func (s *sqlStore) GetAllConfigsByOrigin(origin models.Origin) ([]*models.StoredConfig, error) {
	query := `
		SELECT uuid, kind, handle, display_name, version, desired_state,
			deployment_id, origin, created_at, updated_at, deployed_at,
			cp_sync_status, cp_sync_info, cp_artifact_id
		FROM artifacts
		WHERE origin = ? AND gateway_id = ?
		ORDER BY created_at DESC
	`

	rows, err := s.query(query, string(origin), s.gatewayId)
	if err != nil {
		return nil, fmt.Errorf("failed to query configurations by origin: %w", err)
	}
	defer rows.Close()

	var configs []*models.StoredConfig
	for rows.Next() {
		var cfg models.StoredConfig
		var deployedAt sql.NullTime
		var deploymentID sql.NullString
		var cpSyncStatus sql.NullString
		var cpSyncInfo sql.NullString
		var cpArtifactID sql.NullString

		err := rows.Scan(
			&cfg.UUID,
			&cfg.Kind,
			&cfg.Handle,
			&cfg.DisplayName,
			&cfg.Version,
			&cfg.DesiredState,
			&deploymentID,
			&cfg.Origin,
			&cfg.CreatedAt,
			&cfg.UpdatedAt,
			&deployedAt,
			&cpSyncStatus,
			&cpSyncInfo,
			&cpArtifactID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if deployedAt.Valid {
			cfg.DeployedAt = &deployedAt.Time
		}
		if deploymentID.Valid {
			cfg.DeploymentID = deploymentID.String
		}
		if cpSyncStatus.Valid {
			cfg.CPSyncStatus = models.CPSyncStatus(cpSyncStatus.String)
		}
		if cpSyncInfo.Valid {
			cfg.CPSyncInfo = cpSyncInfo.String
		}
		if cpArtifactID.Valid {
			cfg.CPArtifactID = cpArtifactID.String
		}

		configs = append(configs, &cfg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return configs, nil
}

// UpdateCPSyncStatus updates the cp_sync_status, cp_sync_info, cp_artifact_id, and updated_at
// fields for an artifact. Used by the bottom-up sync engine to record sync outcomes without
// reloading the full config. Pass an empty cpArtifactID when no CP UUID is known (e.g. on
// failure paths or pre-sync).
func (s *sqlStore) UpdateCPSyncStatus(uuid, cpArtifactID string, status models.CPSyncStatus, reason string) error {
	query := `
		UPDATE artifacts
		SET cp_sync_status = ?, cp_sync_info = ?, cp_artifact_id = ?, updated_at = ?
		WHERE uuid = ? AND gateway_id = ?
	`
	var reasonVal interface{}
	if reason != "" {
		reasonVal = reason
	}
	var cpArtifactIDVal interface{}
	if cpArtifactID != "" {
		cpArtifactIDVal = cpArtifactID
	}
	result, err := s.exec(query, string(status), reasonVal, cpArtifactIDVal, time.Now(), uuid, s.gatewayId)
	if err != nil {
		return fmt.Errorf("failed to update cp_sync_status: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: uuid=%s", ErrNotFound, uuid)
	}
	return nil
}

// GetConfigByCPArtifactID looks up a config by the APIM/Control Plane UUID assigned during
// bottom-up sync. Returns ErrNotFound if no match.
func (s *sqlStore) GetConfigByCPArtifactID(cpArtifactID string) (*models.StoredConfig, error) {
	artifactQuery := `
		SELECT uuid, kind, handle, display_name, version, desired_state, deployment_id, origin, created_at, updated_at, deployed_at,
			cp_sync_status, cp_sync_info, cp_artifact_id
		FROM artifacts
		WHERE gateway_id = ? AND cp_artifact_id = ?
	`

	var cfg models.StoredConfig
	var deployedAt sql.NullTime
	var deploymentID sql.NullString
	var cpSyncStatus sql.NullString
	var cpSyncInfo sql.NullString
	var cpArtifactIDCol sql.NullString

	err := s.queryRow(artifactQuery, s.gatewayId, cpArtifactID).Scan(
		&cfg.UUID,
		&cfg.Kind,
		&cfg.Handle,
		&cfg.DisplayName,
		&cfg.Version,
		&cfg.DesiredState,
		&deploymentID,
		&cfg.Origin,
		&cfg.CreatedAt,
		&cfg.UpdatedAt,
		&deployedAt,
		&cpSyncStatus,
		&cpSyncInfo,
		&cpArtifactIDCol,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: cp_artifact_id=%s", ErrNotFound, cpArtifactID)
		}
		return nil, fmt.Errorf("failed to query configuration by cp_artifact_id: %w", err)
	}

	if deployedAt.Valid {
		cfg.DeployedAt = &deployedAt.Time
	}
	if deploymentID.Valid {
		cfg.DeploymentID = deploymentID.String
	}
	if cpSyncStatus.Valid {
		cfg.CPSyncStatus = models.CPSyncStatus(cpSyncStatus.String)
	}
	if cpSyncInfo.Valid {
		cfg.CPSyncInfo = cpSyncInfo.String
	}
	if cpArtifactIDCol.Valid {
		cfg.CPArtifactID = cpArtifactIDCol.String
	}

	if err := s.loadResourceConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// GetPendingBottomUpAPIs returns all RestApi artifacts that originated from the gateway REST API
// and have a cp_sync_status of 'pending' or 'failed'.
// Used by the bottom-up sync to determine which APIs need to be pushed to the control plane.
func (s *sqlStore) GetPendingBottomUpAPIs() ([]*models.StoredConfig, error) {
	query := `
		SELECT a.uuid, a.kind, a.handle, a.display_name, a.version, r.configuration, a.desired_state,
			a.deployment_id, a.origin, a.created_at, a.updated_at, a.deployed_at,
			a.cp_sync_status, a.cp_sync_info, a.cp_artifact_id
		FROM artifacts a
		JOIN rest_apis r ON a.uuid = r.uuid AND a.gateway_id = r.gateway_id
		WHERE a.gateway_id = ?
		  AND a.origin = 'gateway_api'
		  AND a.cp_sync_status IN ('pending', 'failed')
		ORDER BY a.created_at ASC
	`

	rows, err := s.query(query, s.gatewayId)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending bottom-up APIs: %w", err)
	}
	defer rows.Close()

	return s.scanConfigRows(rows)
}

// loadResourceConfig loads the configuration from the correct type table into the StoredConfig.
// cfg.UUID and cfg.Kind must already be populated.
func (s *sqlStore) loadResourceConfig(cfg *models.StoredConfig) error {
	resourceTable, err := kindToResourceTable(cfg.Kind)
	if err != nil {
		return err
	}

	query := fmt.Sprintf(`SELECT configuration FROM %s WHERE uuid = ? AND gateway_id = ?`, resourceTable)

	var configJSON sql.NullString
	err = s.queryRow(query, cfg.UUID, s.gatewayId).Scan(&configJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("resource config not found for uuid=%s in table %s", cfg.UUID, resourceTable)
		}
		return fmt.Errorf("failed to query resource config: %w", err)
	}

	if configJSON.Valid && configJSON.String != "" {
		if err := unmarshalSourceConfig(cfg, configJSON.String); err != nil {
			return err
		}
	}

	return nil
}

// addResourceConfigTx inserts the resource config into the correct type table.
func (s *sqlStore) addResourceConfigTx(tx *sqlStoreTx, cfg *models.StoredConfig) (bool, error) {
	resourceTable, err := kindToResourceTable(cfg.Kind)
	if err != nil {
		return false, err
	}

	configJSON, err := json.Marshal(cfg.SourceConfiguration)
	if err != nil {
		return false, fmt.Errorf("failed to marshal configuration: %w", err)
	}

	var query string
	var args []interface{}

	if cfg.Kind == "LlmProxy" {
		// Proxies persist both the raw JSON payload and the resolved provider UUID
		// so relational lookups do not depend on parsing the configuration blob.
		proxyConfig, ok := cfg.SourceConfiguration.(api.LLMProxyConfiguration)
		if !ok {
			return false, fmt.Errorf("expected LLMProxyConfiguration but got %T", cfg.SourceConfiguration)
		}
		providerUUID, err := s.resolveProviderUUID(tx, proxyConfig.Spec.Provider.Id)
		if err != nil {
			return false, fmt.Errorf("failed to resolve provider: %w", err)
		}
		query = fmt.Sprintf(`INSERT INTO %s (uuid, gateway_id, configuration, provider_uuid) VALUES (?, ?, ?, ?)`, resourceTable)
		args = []interface{}{cfg.UUID, s.gatewayId, string(configJSON), providerUUID}
	} else {
		query = fmt.Sprintf(`INSERT INTO %s (uuid, gateway_id, configuration) VALUES (?, ?, ?)`, resourceTable)
		args = []interface{}{cfg.UUID, s.gatewayId, string(configJSON)}
	}

	stmt, err := tx.tx.Prepare(s.bind(query))
	if err != nil {
		return false, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(args...)
	if err != nil {
		return false, fmt.Errorf("failed to insert resource configuration: %w", err)
	}

	return true, nil
}

// updateResourceConfigTx updates the resource config in the correct type table.
func (s *sqlStore) updateResourceConfigTx(tx *sqlStoreTx, cfg *models.StoredConfig) (bool, error) {
	resourceTable, err := kindToResourceTable(cfg.Kind)
	if err != nil {
		return false, err
	}

	configJSON, err := json.Marshal(cfg.SourceConfiguration)
	if err != nil {
		return false, fmt.Errorf("failed to marshal configuration: %w", err)
	}

	var query string
	var args []interface{}

	if cfg.Kind == "LlmProxy" {
		// Keep the provider UUID in sync with the latest handle->UUID resolution
		// so proxy reads can follow a stable foreign key instead of JSON content.
		proxyConfig, ok := cfg.SourceConfiguration.(api.LLMProxyConfiguration)
		if !ok {
			return false, fmt.Errorf("expected LLMProxyConfiguration but got %T", cfg.SourceConfiguration)
		}
		providerUUID, err := s.resolveProviderUUID(tx, proxyConfig.Spec.Provider.Id)
		if err != nil {
			return false, fmt.Errorf("failed to resolve provider: %w", err)
		}
		query = fmt.Sprintf(`UPDATE %s SET configuration = ?, provider_uuid = ? WHERE uuid = ? AND gateway_id = ?`, resourceTable)
		args = []interface{}{string(configJSON), providerUUID, cfg.UUID, s.gatewayId}
	} else {
		query = fmt.Sprintf(`UPDATE %s SET configuration = ? WHERE uuid = ? AND gateway_id = ?`, resourceTable)
		args = []interface{}{string(configJSON), cfg.UUID, s.gatewayId}
	}

	stmt, err := tx.tx.Prepare(s.bind(query))
	if err != nil {
		return false, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(args...)
	if err != nil {
		return false, fmt.Errorf("failed to update resource configuration: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return false, fmt.Errorf("no resource config found for uuid=%s", cfg.UUID)
	}

	return true, nil
}

// upsertResourceConfigTx performs INSERT ... ON CONFLICT(gateway_id, uuid)
// DO UPDATE for the per-resource-type table. Works identically on SQLite
// (3.24+) and PostgreSQL.
func (s *sqlStore) upsertResourceConfigTx(tx *sqlStoreTx, cfg *models.StoredConfig) error {
	resourceTable, err := kindToResourceTable(cfg.Kind)
	if err != nil {
		return err
	}

	configJSON, err := json.Marshal(cfg.SourceConfiguration)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	var query string
	var args []interface{}

	if cfg.Kind == "LlmProxy" {
		proxyConfig, ok := cfg.SourceConfiguration.(api.LLMProxyConfiguration)
		if !ok {
			return fmt.Errorf("expected LLMProxyConfiguration but got %T", cfg.SourceConfiguration)
		}
		providerUUID, err := s.resolveProviderUUID(tx, proxyConfig.Spec.Provider.Id)
		if err != nil {
			return fmt.Errorf("failed to resolve provider: %w", err)
		}
		query = fmt.Sprintf(`
			INSERT INTO %s (uuid, gateway_id, configuration, provider_uuid) VALUES (?, ?, ?, ?)
			ON CONFLICT(gateway_id, uuid) DO UPDATE SET
				configuration  = excluded.configuration,
				provider_uuid  = excluded.provider_uuid
		`, resourceTable)
		args = []interface{}{cfg.UUID, s.gatewayId, string(configJSON), providerUUID}
	} else {
		query = fmt.Sprintf(`
			INSERT INTO %s (uuid, gateway_id, configuration) VALUES (?, ?, ?)
			ON CONFLICT(gateway_id, uuid) DO UPDATE SET
				configuration = excluded.configuration
		`, resourceTable)
		args = []interface{}{cfg.UUID, s.gatewayId, string(configJSON)}
	}

	stmt, err := tx.tx.Prepare(s.bind(query))
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	if _, err = stmt.Exec(args...); err != nil {
		return fmt.Errorf("failed to upsert resource configuration: %w", err)
	}

	return nil
}

// resolveProviderUUID looks up the provider UUID from the database by provider handle and gateway ID.
// Must use the transaction to avoid deadlock (SQLite has MaxOpenConns=1).
func (s *sqlStore) resolveProviderUUID(tx *sqlStoreTx, providerHandle string) (string, error) {
	var uuid string
	query := s.bind(`SELECT a.uuid FROM artifacts a WHERE a.handle = ? AND a.gateway_id = ? AND a.kind = 'LlmProvider'`)
	err := tx.tx.QueryRow(query, providerHandle, s.gatewayId).Scan(&uuid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("provider '%s' not found for gateway '%s'", providerHandle, s.gatewayId)
		}
		return "", fmt.Errorf("failed to look up provider UUID: %w", err)
	}
	return uuid, nil
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
			uuid, gateway_id, handle, configuration, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	_, err = s.exec(query,
		template.UUID,
		s.gatewayId,
		handle,
		string(configJSON),
		now,
		now,
	)

	if err != nil {
		// Check for unique constraint violation
		if s.isUniqueViolation(err) {
			return fmt.Errorf("%w: template with handle '%s' already exists", ErrConflict, handle)
		}
		return fmt.Errorf("failed to insert template: %w", err)
	}

	s.logger.Info("LLM provider template saved",
		slog.String("uuid", template.UUID),
		slog.String("handle", handle))

	return nil
}

// UpdateLLMProviderTemplate updates an existing LLM provider template
func (s *sqlStore) UpdateLLMProviderTemplate(template *models.StoredLLMProviderTemplate) error {
	// Check if template exists
	_, err := s.GetLLMProviderTemplate(template.UUID)
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
		WHERE uuid = ? AND gateway_id = ?
	`

	result, err := s.exec(query,
		handle,
		string(configJSON),
		time.Now(),
		template.UUID,
		s.gatewayId,
	)

	if err != nil {
		if s.isUniqueViolation(err) {
			return fmt.Errorf("%w: template with handle '%s' already exists", ErrConflict, handle)
		}
		return fmt.Errorf("failed to update template: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("%w: uuid=%s", ErrNotFound, template.UUID)
	}

	s.logger.Info("LLM provider template updated",
		slog.String("uuid", template.UUID),
		slog.String("handle", handle))

	return nil
}

// DeleteLLMProviderTemplate removes an LLM provider template by UUID
func (s *sqlStore) DeleteLLMProviderTemplate(id string) error {
	query := `DELETE FROM llm_provider_templates WHERE uuid = ? AND gateway_id = ?`

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

// GetLLMProviderTemplate retrieves an LLM provider template by UUID
func (s *sqlStore) GetLLMProviderTemplate(id string) (*models.StoredLLMProviderTemplate, error) {
	query := `
		SELECT uuid, configuration, created_at, updated_at
		FROM llm_provider_templates
		WHERE uuid = ? AND gateway_id = ?
	`

	var template models.StoredLLMProviderTemplate
	var configJSON string

	err := s.queryRow(query, id, s.gatewayId).Scan(
		&template.UUID,
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
		SELECT uuid, configuration, created_at, updated_at
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
			&template.UUID,
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

// GetLLMProviderTemplateByHandle retrieves an LLM provider template by handle.
func (s *sqlStore) GetLLMProviderTemplateByHandle(handle string) (*models.StoredLLMProviderTemplate, error) {
	query := `
		SELECT uuid, configuration, created_at, updated_at
		FROM llm_provider_templates
		WHERE gateway_id = ? AND handle = ?
	`

	var template models.StoredLLMProviderTemplate
	var configJSON string

	err := s.queryRow(query, s.gatewayId, handle).Scan(
		&template.UUID,
		&configJSON,
		&template.CreatedAt,
		&template.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: handle=%s", ErrNotFound, handle)
		}
		return nil, fmt.Errorf("failed to query template: %w", err)
	}

	if err := json.Unmarshal([]byte(configJSON), &template.Configuration); err != nil {
		return nil, fmt.Errorf("failed to unmarshal template configuration: %w", err)
	}

	return &template, nil
}

// SaveCertificate persists a certificate to the database
func (s *sqlStore) SaveCertificate(cert *models.StoredCertificate) error {
	query := `
		INSERT INTO certificates (
			uuid, gateway_id, name, certificate, subject, issuer,
			not_before, not_after, cert_count, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.exec(query,
		cert.UUID,
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
		if s.isUniqueViolation(err) {
			return fmt.Errorf("%w: certificate with name '%s' already exists", ErrConflict, cert.Name)
		}
		return fmt.Errorf("failed to save certificate: %w", err)
	}

	return nil
}

// GetCertificate retrieves a certificate by UUID
func (s *sqlStore) GetCertificate(id string) (*models.StoredCertificate, error) {
	query := `
		SELECT uuid, name, certificate, subject, issuer,
		       not_before, not_after, cert_count, created_at, updated_at
		FROM certificates
		WHERE uuid = ? AND gateway_id = ?
	`

	var cert models.StoredCertificate
	err := s.queryRow(query, id, s.gatewayId).Scan(
		&cert.UUID,
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
		SELECT uuid, name, certificate, subject, issuer,
		       not_before, not_after, cert_count, created_at, updated_at
		FROM certificates
		WHERE name = ? AND gateway_id = ?
	`

	var cert models.StoredCertificate
	err := s.queryRow(query, name, s.gatewayId).Scan(
		&cert.UUID,
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
		SELECT uuid, name, certificate, subject, issuer,
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
			&cert.UUID,
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

// DeleteCertificate deletes a certificate by UUID
func (s *sqlStore) DeleteCertificate(id string) error {
	query := `DELETE FROM certificates WHERE uuid = ? AND gateway_id = ?`

	result, err := s.exec(query, id, s.gatewayId)
	if err != nil {
		return fmt.Errorf("failed to delete certificate: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		s.logger.Debug("Certificate not found for deletion", slog.String("uuid", id))
		return ErrNotFound
	}

	s.logger.Info("Certificate deleted", slog.String("uuid", id))

	return nil
}

// API Key Storage Methods

// SaveAPIKey persists a new API key to the database or updates existing one
// if an API key with the same artifact_uuid and name already exists
func (s *sqlStore) SaveAPIKey(apiKey *models.APIKey) error {

	// Begin transaction to ensure atomicity
	tx, err := s.begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Ensure transaction is properly handled
	defer func() {
		if p := recover(); p != nil {
			s.rollbackTx(tx, "panic while saving API key")
			panic(p) // Re-throw panic after rollback
		}
	}()

	// First, check if an API key with the same artifact_uuid and name exists
	checkQuery := `SELECT uuid FROM api_keys WHERE artifact_uuid = ? AND name = ? AND gateway_id = ?`
	var existingUUID string
	err = tx.QueryRowQ(checkQuery, apiKey.ArtifactUUID, apiKey.Name, s.gatewayId).Scan(&existingUUID)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		s.rollbackTx(tx, "failed to check existing API key")
		return fmt.Errorf("failed to check existing API key: %w", err)
	}

	if errors.Is(err, sql.ErrNoRows) {
		// No existing record, insert new API key
		insertQuery := `
			INSERT INTO api_keys (
				uuid, gateway_id, name, api_key, masked_api_key, artifact_uuid, status,
				created_at, created_by, updated_at, expires_at,
				source, external_ref_id, issuer
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`

		_, err := tx.ExecQ(insertQuery,
			apiKey.UUID,
			s.gatewayId,
			apiKey.Name,
			apiKey.APIKey,
			apiKey.MaskedAPIKey,
			apiKey.ArtifactUUID,
			apiKey.Status,
			apiKey.CreatedAt,
			apiKey.CreatedBy,
			apiKey.UpdatedAt,
			apiKey.ExpiresAt,
			apiKey.Source,
			apiKey.ExternalRefId,
			apiKey.Issuer,
		)

		if err != nil {
			s.rollbackTx(tx, "failed to insert API key")
			// Check for unique constraint violation on api_key field
			if s.isUniqueViolation(err) {
				return fmt.Errorf("%w: API key value already exists", ErrConflict)
			}
			return fmt.Errorf("failed to insert API key: %w", err)
		}

	} else {
		// Existing record found, return conflict error that API Key name already exists
		s.rollbackTx(tx, "api key name already exists for API")
		s.logger.Error("API key name already exists for the API",
			slog.String("name", apiKey.Name),
			slog.String("artifact_uuid", apiKey.ArtifactUUID),
			slog.Any("error", ErrConflict))
		return fmt.Errorf("%w: API key name already exists for the API: %s", ErrConflict, apiKey.Name)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info("API key inserted successfully",
		slog.String("name", apiKey.Name),
		slog.String("created_by", apiKey.CreatedBy))

	return nil
}

// UpsertAPIKey inserts or conditionally updates an API key.
//
// The conflict target is the unique composite key (gateway_id, artifact_uuid, name).
// The DO UPDATE only fires when the stored updated_at is strictly older than the incoming one,
// so a racing WebSocket event that already wrote a newer record is never overwritten.
// source is preserved from the existing row when it is already set.
// external_ref_id falls back to the existing value when the incoming one is NULL.
func (s *sqlStore) UpsertAPIKey(apiKey *models.APIKey) error {
	query := `
		INSERT INTO api_keys (
			uuid, gateway_id, name, api_key, masked_api_key, artifact_uuid, status,
			created_at, created_by, updated_at, expires_at,
			source, external_ref_id, issuer
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(gateway_id, artifact_uuid, name) DO UPDATE SET
			uuid            = excluded.uuid,
			api_key         = excluded.api_key,
			masked_api_key  = excluded.masked_api_key,
			status          = excluded.status,
			updated_at      = excluded.updated_at,
			expires_at      = excluded.expires_at,
			source          = CASE WHEN api_keys.source != '' THEN api_keys.source ELSE excluded.source END,
			external_ref_id = COALESCE(excluded.external_ref_id, api_keys.external_ref_id),
			issuer          = CASE WHEN api_keys.issuer != '' THEN api_keys.issuer ELSE excluded.issuer END
		WHERE api_keys.updated_at < excluded.updated_at
	`

	_, err := s.exec(query,
		apiKey.UUID,
		s.gatewayId,
		apiKey.Name,
		apiKey.APIKey,
		apiKey.MaskedAPIKey,
		apiKey.ArtifactUUID,
		apiKey.Status,
		apiKey.CreatedAt,
		apiKey.CreatedBy,
		apiKey.UpdatedAt,
		apiKey.ExpiresAt,
		apiKey.Source,
		apiKey.ExternalRefId,
		apiKey.Issuer,
	)
	if err != nil {
		if s.isUniqueViolation(err) {
			return fmt.Errorf("%w: API key value already exists", ErrConflict)
		}
		return fmt.Errorf("failed to upsert API key: %w", err)
	}

	s.logger.Debug("API key upserted",
		slog.String("name", apiKey.Name),
		slog.String("artifact_uuid", apiKey.ArtifactUUID))

	return nil
}

// GetAPIKeyByID retrieves an API key by its UUID
func (s *sqlStore) GetAPIKeyByID(id string) (*models.APIKey, error) {
	query := `
		SELECT uuid, name, api_key, masked_api_key, artifact_uuid, status,
		       created_at, created_by, updated_at, expires_at, source, external_ref_id,
		       issuer
		FROM api_keys
		WHERE uuid = ? AND gateway_id = ?
	`

	var apiKey models.APIKey
	var expiresAt sql.NullTime
	var externalRefId sql.NullString
	var issuer sql.NullString

	err := s.queryRow(query, id, s.gatewayId).Scan(
		&apiKey.UUID,
		&apiKey.Name,
		&apiKey.APIKey,
		&apiKey.MaskedAPIKey,
		&apiKey.ArtifactUUID,
		&apiKey.Status,
		&apiKey.CreatedAt,
		&apiKey.CreatedBy,
		&apiKey.UpdatedAt,
		&expiresAt,
		&apiKey.Source,
		&externalRefId,
		&issuer,
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
	if issuer.Valid {
		apiKey.Issuer = &issuer.String
	}

	return &apiKey, nil
}

// GetAPIKeyByUUID retrieves an API key by its platform UUID
func (s *sqlStore) GetAPIKeyByUUID(uuid string) (*models.APIKey, error) {
	query := `
		SELECT uuid, name, api_key, masked_api_key, artifact_uuid, status,
		       created_at, created_by, updated_at, expires_at, source, external_ref_id,
		       issuer
		FROM api_keys
		WHERE uuid = ? AND gateway_id = ?
		LIMIT 1
	`

	var apiKey models.APIKey
	var expiresAt sql.NullTime
	var externalRefId sql.NullString
	var issuer sql.NullString

	err := s.queryRow(query, uuid, s.gatewayId).Scan(
		&apiKey.UUID,
		&apiKey.Name,
		&apiKey.APIKey,
		&apiKey.MaskedAPIKey,
		&apiKey.ArtifactUUID,
		&apiKey.Status,
		&apiKey.CreatedAt,
		&apiKey.CreatedBy,
		&apiKey.UpdatedAt,
		&expiresAt,
		&apiKey.Source,
		&externalRefId,
		&issuer,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: key not found", ErrNotFound)
		}
		return nil, fmt.Errorf("failed to query API key by UUID: %w", err)
	}

	if expiresAt.Valid {
		apiKey.ExpiresAt = &expiresAt.Time
	}
	if externalRefId.Valid {
		apiKey.ExternalRefId = &externalRefId.String
	}
	if issuer.Valid {
		apiKey.Issuer = &issuer.String
	}

	return &apiKey, nil
}

// GetAPIKeyByKey retrieves an API key by its key value
func (s *sqlStore) GetAPIKeyByKey(key string) (*models.APIKey, error) {
	query := `
		SELECT uuid, name, api_key, masked_api_key, artifact_uuid, status,
		       created_at, created_by, updated_at, expires_at, source, external_ref_id,
		       issuer
		FROM api_keys
		WHERE api_key = ? AND gateway_id = ?
	`

	var apiKey models.APIKey
	var expiresAt sql.NullTime
	var externalRefId sql.NullString
	var issuer sql.NullString

	err := s.queryRow(query, key, s.gatewayId).Scan(
		&apiKey.UUID,
		&apiKey.Name,
		&apiKey.APIKey,
		&apiKey.MaskedAPIKey,
		&apiKey.ArtifactUUID,
		&apiKey.Status,
		&apiKey.CreatedAt,
		&apiKey.CreatedBy,
		&apiKey.UpdatedAt,
		&expiresAt,
		&apiKey.Source,
		&externalRefId,
		&issuer,
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
	if issuer.Valid {
		apiKey.Issuer = &issuer.String
	}

	return &apiKey, nil
}

// GetAPIKeysByAPI retrieves all API keys for a specific API
func (s *sqlStore) GetAPIKeysByAPI(apiId string) ([]*models.APIKey, error) {
	query := `
		SELECT ak.uuid, ak.name, ak.api_key, ak.masked_api_key, ak.artifact_uuid, ak.status,
		       ak.created_at, ak.created_by, ak.updated_at, ak.expires_at, ak.source, ak.external_ref_id,
		       ak.issuer, app.application_uuid, app.application_name
		FROM api_keys ak
		LEFT JOIN application_api_keys aak
		  ON aak.api_key_id = ak.uuid AND aak.gateway_id = ak.gateway_id
		LEFT JOIN applications app
		  ON app.application_uuid = aak.application_uuid
		WHERE ak.artifact_uuid = ? AND ak.gateway_id = ?
		ORDER BY ak.created_at DESC
	`

	rows, err := s.query(query, apiId, s.gatewayId)
	if err != nil {
		return nil, fmt.Errorf("failed to query API keys: %w", err)
	}
	defer rows.Close()

	return s.scanAPIKeyRows(rows)
}

// GetAPIKeysByAPIAndName retrieves an API key by its artifact_uuid and name
func (s *sqlStore) GetAPIKeysByAPIAndName(apiId, name string) (*models.APIKey, error) {
	query := `
		SELECT uuid, name, api_key, masked_api_key, artifact_uuid, status,
		       created_at, created_by, updated_at, expires_at, source, external_ref_id,
		       issuer
		FROM api_keys
		WHERE artifact_uuid = ? AND name = ? AND gateway_id = ?
		LIMIT 1
	`

	var apiKey models.APIKey
	var expiresAt sql.NullTime
	var externalRefId sql.NullString
	var issuer sql.NullString

	err := s.queryRow(query, apiId, name, s.gatewayId).Scan(
		&apiKey.UUID,
		&apiKey.Name,
		&apiKey.APIKey,
		&apiKey.MaskedAPIKey,
		&apiKey.ArtifactUUID,
		&apiKey.Status,
		&apiKey.CreatedAt,
		&apiKey.CreatedBy,
		&apiKey.UpdatedAt,
		&expiresAt,
		&apiKey.Source,
		&externalRefId,
		&issuer,
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
	if issuer.Valid {
		apiKey.Issuer = &issuer.String
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
			s.rollbackTx(tx, "panic while updating API key")
			panic(p) // Re-throw panic after rollback
		}
	}()

	updateQuery := `
			UPDATE api_keys
			SET api_key = ?, masked_api_key = ?, status = ?, created_by = ?, updated_at = ?, expires_at = ?,
			    source = ?, external_ref_id = ?
			WHERE artifact_uuid = ? AND name = ? AND gateway_id = ?
		`

	_, err = tx.ExecQ(updateQuery,
		apiKey.APIKey,
		apiKey.MaskedAPIKey,
		apiKey.Status,
		apiKey.CreatedBy,
		apiKey.UpdatedAt,
		apiKey.ExpiresAt,
		apiKey.Source,
		apiKey.ExternalRefId,
		apiKey.ArtifactUUID,
		apiKey.Name,
		s.gatewayId,
	)

	if err != nil {
		s.rollbackTx(tx, "failed to update API key")
		// Check for unique constraint violation on api_key field
		if s.isUniqueViolation(err) {
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
		slog.String("artifact_uuid", apiKey.ArtifactUUID),
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

// RemoveAPIKeysAPI removes an API keys by artifact_uuid
func (s *sqlStore) RemoveAPIKeysAPI(apiId string) error {
	query := `DELETE FROM api_keys WHERE artifact_uuid = ? AND gateway_id = ?`

	_, err := s.exec(query, apiId, s.gatewayId)
	if err != nil {
		return fmt.Errorf("failed to remove API keys for API: %w", err)
	}

	s.logger.Info("API keys removed successfully",
		slog.String("artifact_uuid", apiId))

	return nil
}

// RemoveAPIKeyAPIAndName removes an API key by its artifact_uuid and name
func (s *sqlStore) RemoveAPIKeyAPIAndName(apiId, name string) error {
	query := `DELETE FROM api_keys WHERE artifact_uuid = ? AND name = ? AND gateway_id = ?`

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
		slog.String("artifact_uuid", apiId),
		slog.String("name", name))

	return nil
}

// ReplaceApplicationAPIKeyMappings atomically replaces all API key mappings for an application.
func (s *sqlStore) ReplaceApplicationAPIKeyMappings(application *models.StoredApplication, mappings []*models.ApplicationAPIKeyMapping) error {
	if application == nil || application.ApplicationUUID == "" || application.ApplicationID == "" || application.ApplicationName == "" || application.ApplicationType == "" {
		return fmt.Errorf("invalid application payload")
	}

	tx, err := s.begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	seen := make(map[string]struct{})
	now := time.Now()

	if _, err = tx.ExecQ(`
		INSERT INTO applications (
			application_uuid, gateway_id, application_id, application_name, application_type, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(gateway_id, application_uuid) DO UPDATE SET
			application_id = excluded.application_id,
			application_name = excluded.application_name,
			application_type = excluded.application_type,
			updated_at = excluded.updated_at
	`, application.ApplicationUUID, s.gatewayId, application.ApplicationID, application.ApplicationName, application.ApplicationType, now, now); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to upsert application metadata: %w", err)
	}

	if _, err = tx.ExecQ(`
		DELETE FROM application_api_keys
		WHERE application_uuid = ? AND gateway_id = ?
	`, application.ApplicationUUID, s.gatewayId); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to clear application mappings: %w", err)
	}

	for _, mapping := range mappings {
		if mapping == nil {
			continue
		}
		if mapping.ApplicationUUID == "" || mapping.APIKeyID == "" {
			_ = tx.Rollback()
			return fmt.Errorf("invalid application mapping payload")
		}
		if mapping.ApplicationUUID != application.ApplicationUUID {
			_ = tx.Rollback()
			return fmt.Errorf("application mapping UUID mismatch")
		}

		composite := mapping.ApplicationUUID + ":" + mapping.APIKeyID
		if _, exists := seen[composite]; exists {
			continue
		}
		seen[composite] = struct{}{}

		if _, err = tx.ExecQ(`
			INSERT INTO application_api_keys (
				application_uuid, api_key_id, gateway_id, created_at, updated_at
			)
			VALUES (?, ?, ?, ?, ?)
		`, mapping.ApplicationUUID, mapping.APIKeyID, s.gatewayId, now, now); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to insert application mapping: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit application mapping transaction: %w", err)
	}

	return nil
}

// Close closes the database connection
func (s *sqlStore) GetDB() *sql.DB {
	return s.db
}

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

// GetAllAPIKeys retrieves all active API keys from the database.
func (s *sqlStore) GetAllAPIKeys() ([]*models.APIKey, error) {
	query := `
		SELECT ak.uuid, ak.name, ak.api_key, ak.masked_api_key, ak.artifact_uuid, ak.status,
		       ak.created_at, ak.created_by, ak.updated_at, ak.expires_at, ak.source, ak.external_ref_id,
		       ak.issuer, app.application_uuid, app.application_name
		FROM api_keys ak
		LEFT JOIN application_api_keys aak
		  ON aak.api_key_id = ak.uuid AND aak.gateway_id = ak.gateway_id
		LEFT JOIN applications app
		  ON app.application_uuid = aak.application_uuid
		WHERE ak.status = 'active' AND ak.gateway_id = ?
		ORDER BY ak.created_at DESC
	`

	rows, err := s.query(query, s.gatewayId)
	if err != nil {
		return nil, fmt.Errorf("failed to query all API keys: %w", err)
	}
	defer rows.Close()

	return s.scanAPIKeyRows(rows)
}

// scanAPIKeyRows scans rows from a query that returns API key columns
func (s *sqlStore) scanAPIKeyRows(rows *sql.Rows) ([]*models.APIKey, error) {
	var apiKeys []*models.APIKey

	for rows.Next() {
		var apiKey models.APIKey
		var expiresAt sql.NullTime
		var externalRefId sql.NullString
		var issuer sql.NullString
		var applicationID sql.NullString
		var applicationName sql.NullString

		err := rows.Scan(
			&apiKey.UUID,
			&apiKey.Name,
			&apiKey.APIKey,
			&apiKey.MaskedAPIKey,
			&apiKey.ArtifactUUID,
			&apiKey.Status,
			&apiKey.CreatedAt,
			&apiKey.CreatedBy,
			&apiKey.UpdatedAt,
			&expiresAt,
			&apiKey.Source,
			&externalRefId,
			&issuer,
			&applicationID,
			&applicationName,
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
		if issuer.Valid {
			apiKey.Issuer = &issuer.String
		}
		if applicationID.Valid {
			apiKey.ApplicationID = applicationID.String
		}
		if applicationName.Valid {
			apiKey.ApplicationName = applicationName.String
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
		WHERE artifact_uuid = ? AND created_by = ? AND status = ? AND gateway_id = ?
	`

	var count int
	err := s.queryRow(query, apiId, userID, models.APIKeyStatusActive, s.gatewayId).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count active API keys for user %s and API %s: %w", userID, apiId, err)
	}

	return count, nil
}

// ========================================
// Subscription Plan Methods
// ========================================

// SaveSubscriptionPlan persists a new subscription plan.
func (s *sqlStore) SaveSubscriptionPlan(plan *models.SubscriptionPlan) error {
	if plan == nil {
		return fmt.Errorf("failed to insert subscription plan: nil plan")
	}
	plan.GatewayID = s.gatewayId
	now := time.Now()
	plan.CreatedAt = now
	plan.UpdatedAt = now
	query := `
		INSERT INTO subscription_plans (uuid, gateway_id, plan_name, billing_plan, stop_on_quota_reach,
			throttle_limit_count, throttle_limit_unit, expiry_time, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.exec(query, plan.ID, s.gatewayId, plan.PlanName, plan.BillingPlan,
		plan.StopOnQuotaReach, plan.ThrottleLimitCount, plan.ThrottleLimitUnit,
		plan.ExpiryTime, string(plan.Status), plan.CreatedAt, plan.UpdatedAt)
	if err != nil {
		if s.isUniqueViolation(err) {
			return fmt.Errorf("%w: subscription plan already exists", ErrConflict)
		}
		return fmt.Errorf("failed to insert subscription plan: %w", err)
	}
	return nil
}

// GetSubscriptionPlanByID retrieves a subscription plan by ID and gateway.
func (s *sqlStore) GetSubscriptionPlanByID(id, gatewayID string) (*models.SubscriptionPlan, error) {
	query := `
		SELECT uuid, gateway_id, plan_name, billing_plan, stop_on_quota_reach,
			throttle_limit_count, throttle_limit_unit, expiry_time, status, created_at, updated_at
		FROM subscription_plans
		WHERE uuid = ? AND gateway_id = ?
	`
	plan := &models.SubscriptionPlan{}
	err := s.queryRow(query, id, s.gatewayId).Scan(
		&plan.ID, &plan.GatewayID, &plan.PlanName, &plan.BillingPlan,
		&plan.StopOnQuotaReach, &plan.ThrottleLimitCount, &plan.ThrottleLimitUnit,
		&plan.ExpiryTime, &plan.Status, &plan.CreatedAt, &plan.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return plan, nil
}

// ListSubscriptionPlans returns all subscription plans for a gateway.
func (s *sqlStore) ListSubscriptionPlans(gatewayID string) ([]*models.SubscriptionPlan, error) {
	query := `
		SELECT uuid, gateway_id, plan_name, billing_plan, stop_on_quota_reach,
			throttle_limit_count, throttle_limit_unit, expiry_time, status, created_at, updated_at
		FROM subscription_plans
		WHERE gateway_id = ?
		ORDER BY created_at DESC
	`
	rows, err := s.query(query, s.gatewayId)
	if err != nil {
		return nil, fmt.Errorf("failed to list subscription plans: %w", err)
	}
	defer rows.Close()
	var list []*models.SubscriptionPlan
	for rows.Next() {
		plan := &models.SubscriptionPlan{}
		if err := rows.Scan(
			&plan.ID, &plan.GatewayID, &plan.PlanName, &plan.BillingPlan,
			&plan.StopOnQuotaReach, &plan.ThrottleLimitCount, &plan.ThrottleLimitUnit,
			&plan.ExpiryTime, &plan.Status, &plan.CreatedAt, &plan.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, plan)
	}
	return list, rows.Err()
}

// UpdateSubscriptionPlan updates an existing subscription plan.
func (s *sqlStore) UpdateSubscriptionPlan(plan *models.SubscriptionPlan) error {
	if plan == nil {
		return fmt.Errorf("failed to update subscription plan: nil plan")
	}
	plan.GatewayID = s.gatewayId
	plan.UpdatedAt = time.Now()
	query := `
		UPDATE subscription_plans
		SET plan_name = ?, billing_plan = ?, stop_on_quota_reach = ?, throttle_limit_count = ?,
			throttle_limit_unit = ?, expiry_time = ?, status = ?, updated_at = ?
		WHERE uuid = ? AND gateway_id = ?
	`
	result, err := s.exec(query,
		plan.PlanName, plan.BillingPlan, plan.StopOnQuotaReach,
		plan.ThrottleLimitCount, plan.ThrottleLimitUnit, plan.ExpiryTime,
		string(plan.Status), plan.UpdatedAt,
		plan.ID, s.gatewayId,
	)
	if err != nil {
		return fmt.Errorf("failed to update subscription plan: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected when updating subscription plan: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: subscription plan not found: %s", ErrNotFound, plan.ID)
	}
	return nil
}

// DeleteSubscriptionPlan removes a subscription plan by ID and gateway.
func (s *sqlStore) DeleteSubscriptionPlan(id, gatewayID string) error {
	query := `DELETE FROM subscription_plans WHERE uuid = ? AND gateway_id = ?`
	result, err := s.exec(query, id, s.gatewayId)
	if err != nil {
		return fmt.Errorf("failed to delete subscription plan: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected when deleting subscription plan: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: subscription plan not found: %s", ErrNotFound, id)
	}
	return nil
}

// DeleteSubscriptionPlansNotIn removes plans for this gateway whose IDs are not in the given set.
// Used for bulk-sync reconciliation when plans were deleted on the control plane during downtime.
func (s *sqlStore) DeleteSubscriptionPlansNotIn(ids []string) error {
	gatewayID := s.gatewayId
	if len(ids) == 0 {
		query := `DELETE FROM subscription_plans WHERE gateway_id = ?`
		_, err := s.exec(query, gatewayID)
		if err != nil {
			return fmt.Errorf("failed to delete subscription plans not in set: %w", err)
		}
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, 0, len(ids)+1)
	args = append(args, gatewayID)
	for i := range ids {
		placeholders[i] = "?"
		args = append(args, ids[i])
	}
	query := fmt.Sprintf(`DELETE FROM subscription_plans WHERE gateway_id = ? AND uuid NOT IN (%s)`,
		strings.Join(placeholders, ","))
	_, err := s.exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete subscription plans not in set: %w", err)
	}
	return nil
}

// SaveSubscription persists a new subscription.
// Receives plain token from platform-api; encrypts and hashes before storage.
func (s *sqlStore) SaveSubscription(sub *models.Subscription) error {
	if sub == nil {
		return fmt.Errorf("failed to insert subscription: nil subscription")
	}
	sub.GatewayID = s.gatewayId
	plainToken := sub.SubscriptionToken
	if plainToken == "" {
		return fmt.Errorf("subscription token cannot be empty")
	}
	tokenHash := hashSubscriptionToken(plainToken)
	sub.SubscriptionTokenHash = tokenHash

	now := time.Now()
	sub.CreatedAt = now
	sub.UpdatedAt = now
	query := `
		INSERT INTO subscriptions (uuid, gateway_id, api_id, application_id, subscription_token_hash,
			subscription_plan_id, billing_customer_id, billing_subscription_id, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.exec(query, sub.ID, s.gatewayId, sub.APIID, sub.ApplicationID,
		tokenHash, sub.SubscriptionPlanID, sub.BillingCustomerID, sub.BillingSubscriptionID, string(sub.Status), sub.CreatedAt, sub.UpdatedAt)
	if err != nil {
		if s.isUniqueViolation(err) {
			return fmt.Errorf("%w: subscription token already exists for this API", ErrConflict)
		}
		return fmt.Errorf("failed to insert subscription: %w", err)
	}
	return nil
}

// GetSubscriptionByID retrieves a subscription by ID and gateway.
// SubscriptionToken is not stored; use Platform-API to retrieve the original token.
func (s *sqlStore) GetSubscriptionByID(id, gatewayID string) (*models.Subscription, error) {
	query := `
		SELECT uuid, api_id, application_id, subscription_token_hash, subscription_plan_id,
			billing_customer_id, billing_subscription_id, gateway_id, status, created_at, updated_at
		FROM subscriptions
		WHERE uuid = ? AND gateway_id = ?
	`
	sub := &models.Subscription{}
	err := s.queryRow(query, id, s.gatewayId).Scan(
		&sub.ID, &sub.APIID, &sub.ApplicationID, &sub.SubscriptionTokenHash,
		&sub.SubscriptionPlanID, &sub.BillingCustomerID, &sub.BillingSubscriptionID,
		&sub.GatewayID, &sub.Status,
		&sub.CreatedAt, &sub.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return sub, nil
}

// ListSubscriptionsByAPI returns subscriptions for an API with optional filters.
func (s *sqlStore) ListSubscriptionsByAPI(apiID, gatewayID string, applicationID *string, status *string) ([]*models.Subscription, error) {
	query := `
		SELECT uuid, api_id, application_id, subscription_token_hash, subscription_plan_id,
			billing_customer_id, billing_subscription_id, gateway_id, status, created_at, updated_at
		FROM subscriptions
		WHERE gateway_id = ?
	`
	args := []interface{}{s.gatewayId}
	if apiID != "" {
		query += ` AND api_id = ?`
		args = append(args, apiID)
	}
	if applicationID != nil && *applicationID != "" {
		query += ` AND application_id = ?`
		args = append(args, *applicationID)
	}
	if status != nil && *status != "" {
		query += ` AND status = ?`
		args = append(args, *status)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := s.query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list subscriptions: %w", err)
	}
	defer rows.Close()
	var list []*models.Subscription
	for rows.Next() {
		sub := &models.Subscription{}
		if err := rows.Scan(&sub.ID, &sub.APIID, &sub.ApplicationID, &sub.SubscriptionTokenHash,
			&sub.SubscriptionPlanID, &sub.BillingCustomerID, &sub.BillingSubscriptionID,
			&sub.GatewayID, &sub.Status, &sub.CreatedAt, &sub.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, sub)
	}
	return list, rows.Err()
}

// ListActiveSubscriptions returns all ACTIVE subscriptions for this gateway in one query.
func (s *sqlStore) ListActiveSubscriptions() ([]*models.Subscription, error) {
	query := `
		SELECT uuid, api_id, application_id, subscription_token_hash, subscription_plan_id,
			billing_customer_id, billing_subscription_id, gateway_id, status, created_at, updated_at
		FROM subscriptions
		WHERE gateway_id = ? AND status = ?
		ORDER BY created_at DESC
	`
	rows, err := s.query(query, s.gatewayId, string(models.SubscriptionStatusActive))
	if err != nil {
		return nil, fmt.Errorf("failed to list active subscriptions: %w", err)
	}
	defer rows.Close()
	var list []*models.Subscription
	for rows.Next() {
		sub := &models.Subscription{}
		if err := rows.Scan(&sub.ID, &sub.APIID, &sub.ApplicationID, &sub.SubscriptionTokenHash,
			&sub.SubscriptionPlanID, &sub.BillingCustomerID, &sub.BillingSubscriptionID,
			&sub.GatewayID, &sub.Status, &sub.CreatedAt, &sub.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, sub)
	}
	return list, rows.Err()
}

// UpdateSubscription updates an existing subscription.
// Persists all mutable fields: application_id, subscription_token_hash, subscription_plan_id, status.
// If plainToken is set, hashes it; otherwise reuses existing SubscriptionTokenHash (for status-only updates from DB-loaded subs).
func (s *sqlStore) UpdateSubscription(sub *models.Subscription) error {
	if sub == nil {
		return fmt.Errorf("failed to update subscription: nil subscription")
	}
	sub.GatewayID = s.gatewayId
	sub.UpdatedAt = time.Now()
	plainToken := sub.SubscriptionToken
	tokenHash := sub.SubscriptionTokenHash
	if plainToken != "" {
		tokenHash = hashSubscriptionToken(plainToken)
	}
	if tokenHash == "" {
		return fmt.Errorf("subscription token hash cannot be empty")
	}
	sub.SubscriptionTokenHash = tokenHash

	query := `
		UPDATE subscriptions
		SET api_id = ?, application_id = ?, subscription_token_hash = ?,
			subscription_plan_id = ?, billing_customer_id = ?, billing_subscription_id = ?,
			status = ?, updated_at = ?
		WHERE uuid = ? AND gateway_id = ?
	`
	result, err := s.exec(query, sub.APIID, sub.ApplicationID, sub.SubscriptionTokenHash,
		sub.SubscriptionPlanID, sub.BillingCustomerID, sub.BillingSubscriptionID,
		string(sub.Status), sub.UpdatedAt, sub.ID, s.gatewayId)
	if err != nil {
		return fmt.Errorf("failed to update subscription: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected when updating subscription: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: subscription not found: %s", ErrNotFound, sub.ID)
	}
	return nil
}

// DeleteSubscription removes a subscription by ID and gateway.
func (s *sqlStore) DeleteSubscription(id, gatewayID string) error {
	query := `DELETE FROM subscriptions WHERE uuid = ? AND gateway_id = ?`
	result, err := s.exec(query, id, s.gatewayId)
	if err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected when deleting subscription: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: subscription not found: %s", ErrNotFound, id)
	}
	return nil
}

// DeleteSubscriptionsForAPINotIn removes subscriptions for the given API whose IDs are not in the set.
// Used for bulk-sync reconciliation when subscriptions were deleted on the control plane during downtime.
func (s *sqlStore) DeleteSubscriptionsForAPINotIn(apiID string, ids []string) error {
	gatewayID := s.gatewayId
	if apiID == "" {
		return fmt.Errorf("apiID is required for DeleteSubscriptionsForAPINotIn")
	}
	if len(ids) == 0 {
		query := `DELETE FROM subscriptions WHERE gateway_id = ? AND api_id = ?`
		_, err := s.exec(query, gatewayID, apiID)
		if err != nil {
			return fmt.Errorf("failed to delete subscriptions for API not in set: %w", err)
		}
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, 0, len(ids)+2)
	args = append(args, gatewayID, apiID)
	for i := range ids {
		placeholders[i] = "?"
		args = append(args, ids[i])
	}
	query := fmt.Sprintf(`DELETE FROM subscriptions WHERE gateway_id = ? AND api_id = ? AND uuid NOT IN (%s)`,
		strings.Join(placeholders, ","))
	_, err := s.exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete subscriptions for API not in set: %w", err)
	}
	return nil
}

// SaveSecret persists a new encrypted secret
func (s *sqlStore) SaveSecret(secret *models.Secret) error {
	startTime := time.Now()
	table := "secrets"

	query := `
	INSERT INTO secrets (gateway_id, handle, display_name, description, ciphertext, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now().UTC()
	_, err := s.exec(query,
		s.gatewayId,
		secret.Handle,
		secret.DisplayName,
		secret.Description,
		secret.Ciphertext,
		now,
		now,
	)

	if err != nil {
		if s.isUniqueViolation(err) {
			metrics.DatabaseOperationsTotal.WithLabelValues("insert", table, "error").Inc()
			metrics.StorageErrorsTotal.WithLabelValues("insert", "conflict").Inc()
			return fmt.Errorf("%w: secret with id '%s' already exists", ErrConflict, secret.Handle)
		}
		metrics.DatabaseOperationsTotal.WithLabelValues("insert", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("insert", "exec_error").Inc()
		s.logger.Error("Failed to save secret",
			slog.String("secret_handle", secret.Handle),
			slog.Any("error", err),
		)
		return fmt.Errorf("failed to save secret: %w", err)
	}

	// Reflect the persisted timestamps back onto the caller's struct so the
	// handler layer can surface them in the response without a round-trip read.
	secret.CreatedAt = now
	secret.UpdatedAt = now

	metrics.DatabaseOperationsTotal.WithLabelValues("insert", table, "success").Inc()
	metrics.DatabaseOperationDurationSeconds.WithLabelValues("insert", table).Observe(time.Since(startTime).Seconds())

	s.logger.Debug("Secret saved successfully",
		slog.String("secret_handle", secret.Handle),
	)

	return nil
}

// GetSecrets retrieves metadata for all secrets (no ciphertext)
func (s *sqlStore) GetSecrets() ([]models.SecretMeta, error) {
	startTime := time.Now()
	table := "secrets"

	query := `SELECT handle, display_name, created_at, updated_at FROM secrets WHERE gateway_id = ? ORDER BY created_at DESC`

	rows, err := s.query(query, s.gatewayId)
	if err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("read", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("read", "query_error").Inc()
		return nil, fmt.Errorf("failed to query secrets: %w", err)
	}
	defer rows.Close()

	var secrets []models.SecretMeta
	for rows.Next() {
		var secret models.SecretMeta
		if err := rows.Scan(&secret.Handle, &secret.DisplayName, &secret.CreatedAt, &secret.UpdatedAt); err != nil {
			metrics.DatabaseOperationsTotal.WithLabelValues("read", table, "error").Inc()
			metrics.StorageErrorsTotal.WithLabelValues("read", "scan_error").Inc()
			return nil, fmt.Errorf("failed to scan secret meta: %w", err)
		}
		secrets = append(secrets, secret)
	}

	if err := rows.Err(); err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("read", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("read", "rows_error").Inc()
		return nil, fmt.Errorf("error iterating secrets: %w", err)
	}

	metrics.DatabaseOperationsTotal.WithLabelValues("read", table, "success").Inc()
	metrics.DatabaseOperationDurationSeconds.WithLabelValues("read", table).Observe(time.Since(startTime).Seconds())

	s.logger.Debug("Secrets retrieved successfully",
		slog.Int("count", len(secrets)),
	)

	return secrets, nil
}

// GetSecret retrieves a secret by Handle
func (s *sqlStore) GetSecret(handle string) (*models.Secret, error) {
	startTime := time.Now()
	table := "secrets"

	query := `
	SELECT handle, display_name, description, ciphertext, created_at, updated_at
	FROM secrets
	WHERE gateway_id = ? AND handle = ?
	`

	var secret models.Secret
	err := s.queryRow(query, s.gatewayId, handle).Scan(
		&secret.Handle,
		&secret.DisplayName,
		&secret.Description,
		&secret.Ciphertext,
		&secret.CreatedAt,
		&secret.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		metrics.DatabaseOperationsTotal.WithLabelValues("read", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("read", "not_found").Inc()
		return nil, fmt.Errorf("%w: id=%s", ErrNotFound, handle)
	}

	if err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("read", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("read", "query_error").Inc()
		s.logger.Error("Failed to get secret",
			slog.String("secret_handle", handle),
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}

	metrics.DatabaseOperationsTotal.WithLabelValues("read", table, "success").Inc()
	metrics.DatabaseOperationDurationSeconds.WithLabelValues("read", table).Observe(time.Since(startTime).Seconds())

	s.logger.Debug("Secret retrieved successfully",
		slog.String("secret_handle", secret.Handle),
	)

	return &secret, nil
}

// UpdateSecret updates an existing secret and returns the updated model with timestamps
func (s *sqlStore) UpdateSecret(secret *models.Secret) (*models.Secret, error) {
	startTime := time.Now()
	table := "secrets"

	query := `
	UPDATE secrets
	SET display_name = ?, description = ?, ciphertext = ?, updated_at = ?
	WHERE gateway_id = ? AND handle = ?
	RETURNING handle, display_name, description, ciphertext, created_at, updated_at
	`

	now := time.Now().UTC()
	row := s.queryRow(query,
		secret.DisplayName,
		secret.Description,
		secret.Ciphertext,
		now,
		s.gatewayId,
		secret.Handle,
	)

	var updated models.Secret
	err := row.Scan(&updated.Handle, &updated.DisplayName, &updated.Description, &updated.Ciphertext, &updated.CreatedAt, &updated.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			metrics.DatabaseOperationsTotal.WithLabelValues("update", table, "error").Inc()
			metrics.StorageErrorsTotal.WithLabelValues("update", "not_found").Inc()
			return nil, fmt.Errorf("%w: id=%s", ErrNotFound, secret.Handle)
		}
		metrics.DatabaseOperationsTotal.WithLabelValues("update", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("update", "exec_error").Inc()
		s.logger.Error("Failed to update secret",
			slog.String("secret_handle", secret.Handle),
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("failed to update secret: %w", err)
	}

	metrics.DatabaseOperationsTotal.WithLabelValues("update", table, "success").Inc()
	metrics.DatabaseOperationDurationSeconds.WithLabelValues("update", table).Observe(time.Since(startTime).Seconds())

	s.logger.Debug("Secret updated successfully",
		slog.String("secret_handle", secret.Handle),
	)

	return &updated, nil
}

// DeleteSecret permanently removes a secret
func (s *sqlStore) DeleteSecret(handle string) error {
	startTime := time.Now()
	table := "secrets"

	query := `DELETE FROM secrets WHERE gateway_id = ? AND handle = ?`

	result, err := s.exec(query, s.gatewayId, handle)
	if err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("delete", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("delete", "exec_error").Inc()
		s.logger.Error("Failed to delete secret",
			slog.String("secret_handle", handle),
			slog.Any("error", err),
		)
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		metrics.DatabaseOperationsTotal.WithLabelValues("delete", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("delete", "rows_affected_error").Inc()
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		metrics.DatabaseOperationsTotal.WithLabelValues("delete", table, "error").Inc()
		metrics.StorageErrorsTotal.WithLabelValues("delete", "not_found").Inc()
		return fmt.Errorf("%w: id=%s", ErrNotFound, handle)
	}

	metrics.DatabaseOperationsTotal.WithLabelValues("delete", table, "success").Inc()
	metrics.DatabaseOperationDurationSeconds.WithLabelValues("delete", table).Observe(time.Since(startTime).Seconds())

	s.logger.Debug("Secret deleted successfully",
		slog.String("secret_handle", handle),
	)

	return nil
}

// SecretExists checks if a secret with the given handle exists
func (s *sqlStore) SecretExists(handle string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM secrets WHERE gateway_id = ? AND handle = ?)`

	var exists bool
	err := s.queryRow(query, s.gatewayId, handle).Scan(&exists)
	if err != nil {
		s.logger.Error("Failed to check secret existence",
			slog.String("secret_handle", handle),
			slog.Any("error", err),
		)
		return false, fmt.Errorf("failed to check secret existence: %w", err)
	}

	return exists, nil
}

// ListAPIKeysForArtifactsNotIn returns uuid + artifact_uuid for keys that would be removed
// by DeleteAPIKeysForArtifactsNotIn. Call this before the delete to collect identifiers
// needed for publishing EventHub events.
func (s *sqlStore) ListAPIKeysForArtifactsNotIn(artifactUUIDs []string, keyUUIDs []string) ([]*models.APIKey, error) {
	if len(artifactUUIDs) == 0 {
		return nil, nil
	}
	artifactPlaceholders := make([]string, len(artifactUUIDs))
	args := make([]interface{}, 0, len(artifactUUIDs)+len(keyUUIDs)+1)
	args = append(args, s.gatewayId)
	for i, id := range artifactUUIDs {
		artifactPlaceholders[i] = "?"
		args = append(args, id)
	}
	var query string
	if len(keyUUIDs) == 0 {
		query = fmt.Sprintf(
			`SELECT uuid, artifact_uuid, name FROM api_keys WHERE gateway_id = ? AND artifact_uuid IN (%s)`,
			strings.Join(artifactPlaceholders, ","),
		)
	} else {
		keyPlaceholders := make([]string, len(keyUUIDs))
		for i, id := range keyUUIDs {
			keyPlaceholders[i] = "?"
			args = append(args, id)
		}
		query = fmt.Sprintf(
			`SELECT uuid, artifact_uuid, name FROM api_keys WHERE gateway_id = ? AND artifact_uuid IN (%s) AND uuid NOT IN (%s)`,
			strings.Join(artifactPlaceholders, ","),
			strings.Join(keyPlaceholders, ","),
		)
	}
	rows, err := s.query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys for artifacts not in set: %w", err)
	}
	defer rows.Close()
	var keys []*models.APIKey
	for rows.Next() {
		k := &models.APIKey{}
		if err := rows.Scan(&k.UUID, &k.ArtifactUUID, &k.Name); err != nil {
			return nil, fmt.Errorf("failed to scan API key row: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// DeleteAPIKeysByUUIDs removes API keys by their UUIDs.
func (s *sqlStore) DeleteAPIKeysByUUIDs(uuids []string) error {
	if len(uuids) == 0 {
		return nil
	}
	placeholders := make([]string, len(uuids))
	args := make([]interface{}, 0, len(uuids)+1)
	args = append(args, s.gatewayId)
	for i, id := range uuids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	query := fmt.Sprintf(`DELETE FROM api_keys WHERE gateway_id = ? AND uuid IN (%s)`,
		strings.Join(placeholders, ","))
	_, err := s.exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete API keys by UUIDs: %w", err)
	}
	return nil
}
