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

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// BackendStorage persists gateway-wide Backends. Every SQL backend implements it. It is kept
// apart from Storage so the Storage test doubles need not grow Backend methods; obtain it with
// AsBackendStorage.
type BackendStorage interface {
	// SaveBackend persists a new Backend and its artifacts row. Returns ErrConflict when an
	// Backend with the same name exists.
	SaveBackend(backend *models.StoredBackend) error
	// UpdateBackend replaces an existing Backend's configuration. Returns ErrNotFound when it
	// does not exist.
	UpdateBackend(backend *models.StoredBackend) error
	// DeleteBackend removes a Backend and its artifacts row by UUID.
	DeleteBackend(id string) error
	// GetBackend returns a Backend by UUID.
	GetBackend(id string) (*models.StoredBackend, error)
	// GetBackendByHandle returns a Backend by name.
	GetBackendByHandle(handle string) (*models.StoredBackend, error)
	// GetAllBackends returns every Backend of this gateway, ordered by name.
	GetAllBackends() ([]*models.StoredBackend, error)
}

// AsBackendStorage returns db's Backend persistence, or false when the backend has none.
func AsBackendStorage(db Storage) (BackendStorage, bool) {
	es, ok := db.(BackendStorage)
	return es, ok
}

var _ BackendStorage = (*sqlStore)(nil)

// SaveBackend persists a new Backend and its artifacts tracking row in one transaction.
func (s *sqlStore) SaveBackend(backend *models.StoredBackend) error {
	configJSON, err := json.Marshal(backend.Configuration)
	if err != nil {
		return fmt.Errorf("failed to marshal backend configuration: %w", err)
	}
	handle := backend.GetHandle()
	origin := backend.Origin
	if origin == "" {
		origin = models.OriginGatewayAPI
	}

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

	now := time.Now()
	if err := s.insertBackendArtifactRow(tx, backend.UUID, handle, origin, now); err != nil {
		if s.isUniqueViolation(err) {
			return fmt.Errorf("%w: backend with name '%s' already exists", ErrConflict, handle)
		}
		return fmt.Errorf("failed to insert backend artifact row: %w", err)
	}
	if _, err := tx.ExecQ(`
		INSERT INTO backends (uuid, gateway_id, handle, configuration, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, backend.UUID, s.gatewayId, handle, string(configJSON), now, now); err != nil {
		if s.isUniqueViolation(err) {
			return fmt.Errorf("%w: backend with name '%s' already exists", ErrConflict, handle)
		}
		return fmt.Errorf("failed to insert backend: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit backend transaction: %w", err)
	}
	committed = true
	backend.Origin, backend.CreatedAt, backend.UpdatedAt = origin, now, now

	s.logger.Info("Backend saved", slog.String("uuid", backend.UUID), slog.String("name", handle))
	return nil
}

// insertBackendArtifactRow inserts the artifacts-table row for a Backend. Backends are
// gateway-wide and unversioned: version is empty and display_name mirrors the name. The row
// carries no cp_sync_status, so the control-plane push leaves Backends alone until the control
// plane can import them; it has no row in any table GetAllConfigs joins, so a Backend never
// enters API hydration.
func (s *sqlStore) insertBackendArtifactRow(tx *sqlStoreTx, uuid, handle string, origin models.Origin, now time.Time) error {
	_, err := tx.ExecQ(`
		INSERT INTO artifacts (
			uuid, gateway_id, display_name, version, kind, handle,
			desired_state, deployment_id, origin, created_at, updated_at, deployed_at,
			cp_sync_status, cp_sync_info, cp_artifact_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		uuid, s.gatewayId,
		handle, // display_name
		"",     // version (Backends are unversioned)
		string(models.KindBackend),
		handle,
		string(models.StateDeployed),
		nil, // deployment_id
		string(origin),
		now, now,
		now, // deployed_at: a Backend is live as soon as it is stored
		nil, nil, nil,
	)
	return err
}

// UpdateBackend replaces an existing Backend's configuration. The name cannot change.
func (s *sqlStore) UpdateBackend(backend *models.StoredBackend) error {
	configJSON, err := json.Marshal(backend.Configuration)
	if err != nil {
		return fmt.Errorf("failed to marshal backend configuration: %w", err)
	}

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

	// The handle column indexes the name, so a payload naming a different Backend than the one
	// stored under this UUID is refused here too, not only by the service.
	var storedHandle string
	if err := tx.QueryRowQ(`SELECT handle FROM backends WHERE uuid = ? AND gateway_id = ?`,
		backend.UUID, s.gatewayId).Scan(&storedHandle); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: uuid=%s", ErrNotFound, backend.UUID)
		}
		return fmt.Errorf("failed to read backend: %w", err)
	}
	if storedHandle != backend.GetHandle() {
		return fmt.Errorf("backend %q cannot be renamed to %q", storedHandle, backend.GetHandle())
	}

	now := time.Now()
	result, err := tx.ExecQ(`
		UPDATE backends SET configuration = ?, updated_at = ?
		WHERE uuid = ? AND gateway_id = ?
	`, string(configJSON), now, backend.UUID, s.gatewayId)
	if err != nil {
		return fmt.Errorf("failed to update backend: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: uuid=%s", ErrNotFound, backend.UUID)
	}
	if _, err := tx.ExecQ(`
		UPDATE artifacts SET updated_at = ?, deployed_at = ?
		WHERE uuid = ? AND gateway_id = ? AND kind = ?
	`, now, now, backend.UUID, s.gatewayId, string(models.KindBackend)); err != nil {
		return fmt.Errorf("failed to update backend artifact row: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit backend transaction: %w", err)
	}
	committed = true
	backend.UpdatedAt = now

	s.logger.Info("Backend updated", slog.String("uuid", backend.UUID), slog.String("name", backend.GetHandle()))
	return nil
}

// DeleteBackend removes a Backend and its artifacts row by UUID in one transaction.
func (s *sqlStore) DeleteBackend(id string) error {
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

	result, err := tx.ExecQ(`DELETE FROM backends WHERE uuid = ? AND gateway_id = ?`, id, s.gatewayId)
	if err != nil {
		return fmt.Errorf("failed to delete backend: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: uuid=%s", ErrNotFound, id)
	}
	if _, err := tx.ExecQ(`DELETE FROM artifacts WHERE uuid = ? AND gateway_id = ? AND kind = ?`,
		id, s.gatewayId, string(models.KindBackend)); err != nil {
		return fmt.Errorf("failed to delete backend artifact row: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit backend transaction: %w", err)
	}
	committed = true

	s.logger.Info("Backend deleted", slog.String("uuid", id))
	return nil
}

const backendSelectColumns = `
	SELECT e.uuid, e.configuration, e.created_at, e.updated_at, a.origin
	FROM backends e
	LEFT JOIN artifacts a ON a.uuid = e.uuid AND a.gateway_id = e.gateway_id AND a.kind = ?
`

// GetBackend returns a Backend by UUID.
func (s *sqlStore) GetBackend(id string) (*models.StoredBackend, error) {
	backend, err := scaBackend(s.queryRow(backendSelectColumns+` WHERE e.uuid = ? AND e.gateway_id = ?`,
		string(models.KindBackend), id, s.gatewayId))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: uuid=%s", ErrNotFound, id)
	}
	return backend, err
}

// GetBackendByHandle returns a Backend by name.
func (s *sqlStore) GetBackendByHandle(handle string) (*models.StoredBackend, error) {
	backend, err := scaBackend(s.queryRow(backendSelectColumns+` WHERE e.gateway_id = ? AND e.handle = ?`,
		string(models.KindBackend), s.gatewayId, handle))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: name=%s", ErrNotFound, handle)
	}
	return backend, err
}

// GetAllBackends returns every Backend of this gateway, ordered by name.
func (s *sqlStore) GetAllBackends() ([]*models.StoredBackend, error) {
	rows, err := s.query(backendSelectColumns+` WHERE e.gateway_id = ? ORDER BY e.handle`,
		string(models.KindBackend), s.gatewayId)
	if err != nil {
		return nil, fmt.Errorf("failed to query backends: %w", err)
	}
	defer rows.Close()

	backends := []*models.StoredBackend{}
	for rows.Next() {
		backend, err := scaBackend(rows)
		if err != nil {
			return nil, err
		}
		backends = append(backends, backend)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating backend rows: %w", err)
	}
	return backends, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scaBackend(row rowScanner) (*models.StoredBackend, error) {
	var backend models.StoredBackend
	var configJSON string
	var origin sql.NullString
	if err := row.Scan(&backend.UUID, &configJSON, &backend.CreatedAt, &backend.UpdatedAt, &origin); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to sca backend: %w", err)
	}
	if err := json.Unmarshal([]byte(configJSON), &backend.Configuration); err != nil {
		return nil, fmt.Errorf("failed to unmarshal backend configuration: %w", err)
	}
	backend.Origin = models.OriginGatewayAPI
	if origin.Valid && origin.String != "" {
		backend.Origin = models.Origin(origin.String)
	}
	return &backend, nil
}
