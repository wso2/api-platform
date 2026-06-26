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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	gwstorage "github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// EventSQLStore implements EventStorage using a *sql.DB.
type EventSQLStore struct {
	db                *sql.DB
	gatewayID         string
	rebind            func(string) string
	isUniqueViolation func(error) bool
}

// NewEventSQLStore creates a new EventSQLStore for the given database type.
// dbType must be one of "sqlite", "postgres", or "sqlserver".
func NewEventSQLStore(db *sql.DB, gatewayID string, dbType string) EventStorage {
	return &EventSQLStore{
		db:                db,
		gatewayID:         gatewayID,
		rebind:            rebindFunc(dbType),
		isUniqueViolation: uniqueViolationFunc(dbType),
	}
}

func rebindFunc(dbType string) func(string) string {
	switch dbType {
	case "postgres":
		return func(q string) string {
			var b strings.Builder
			idx := 1
			for _, r := range q {
				if r == '?' {
					b.WriteString(fmt.Sprintf("$%d", idx))
					idx++
				} else {
					b.WriteRune(r)
				}
			}
			return b.String()
		}
	case "sqlserver":
		return func(q string) string {
			var b strings.Builder
			idx := 1
			for _, r := range q {
				if r == '?' {
					b.WriteString(fmt.Sprintf("@p%d", idx))
					idx++
				} else {
					b.WriteRune(r)
				}
			}
			return b.String()
		}
	default:
		return func(q string) string { return q }
	}
}

func uniqueViolationFunc(dbType string) func(error) bool {
	return func(err error) bool {
		if err == nil {
			return false
		}
		msg := err.Error()
		switch dbType {
		case "postgres":
			return strings.Contains(msg, "duplicate key")
		case "sqlserver":
			return strings.Contains(msg, "Cannot insert duplicate key")
		default:
			return strings.Contains(msg, "UNIQUE constraint failed")
		}
	}
}

// Initialize creates the webhook_secrets table if it does not yet exist.
func (s *EventSQLStore) Initialize() error {
	var ddl string
	// Detect SQL Server by checking if the rebind produces @p1.
	probe := s.rebind("?")
	if probe == "@p1" {
		ddl = `
IF OBJECT_ID('webhook_secrets', 'U') IS NULL
BEGIN
    CREATE TABLE webhook_secrets (
        uuid NVARCHAR(255) NOT NULL,
        gateway_id NVARCHAR(255) NOT NULL,
        artifact_uuid NVARCHAR(255) NOT NULL,
        name NVARCHAR(255) NOT NULL,
        display_name NVARCHAR(500) NOT NULL,
        ciphertext NVARCHAR(MAX) NOT NULL,
        status NVARCHAR(50) NOT NULL DEFAULT 'active',
        created_at DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
        updated_at DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
        CONSTRAINT uq_webhook_secrets_gateway_artifact_name UNIQUE (gateway_id, artifact_uuid, name)
    )
END`
	} else {
		ddl = `
CREATE TABLE IF NOT EXISTS webhook_secrets (
    uuid VARCHAR(255) NOT NULL,
    gateway_id VARCHAR(255) NOT NULL,
    artifact_uuid VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    display_name VARCHAR(500) NOT NULL,
    ciphertext TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(gateway_id, artifact_uuid, name)
)`
	}
	_, err := s.db.Exec(ddl)
	if err != nil {
		return fmt.Errorf("failed to initialize webhook_secrets table: %w", err)
	}
	return nil
}

// SaveWebhookSecret persists a new webhook secret.
func (s *EventSQLStore) SaveWebhookSecret(secret *models.WebhookSecret) error {
	now := time.Now().UTC()
	q := s.rebind(`INSERT INTO webhook_secrets
        (uuid, gateway_id, artifact_uuid, name, display_name, ciphertext, status, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	_, err := s.db.Exec(q,
		secret.UUID,
		secret.GatewayID,
		secret.ArtifactUUID,
		secret.Name,
		secret.DisplayName,
		string(secret.Ciphertext),
		secret.Status,
		now,
		now,
	)
	if err != nil {
		if s.isUniqueViolation(err) {
			return fmt.Errorf("%w: secret with name %q already exists for artifact %q",
				gwstorage.ErrConflict, secret.Name, secret.ArtifactUUID)
		}
		return fmt.Errorf("failed to save webhook secret: %w", err)
	}
	secret.CreatedAt = now
	secret.UpdatedAt = now
	return nil
}

// GetWebhookSecretsByArtifact returns all webhook secrets for the given artifact.
func (s *EventSQLStore) GetWebhookSecretsByArtifact(artifactUUID string) ([]*models.WebhookSecret, error) {
	q := s.rebind(`SELECT uuid, gateway_id, artifact_uuid, name, display_name, ciphertext, status, created_at, updated_at
        FROM webhook_secrets WHERE artifact_uuid = ?`)
	rows, err := s.db.Query(q, artifactUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to query webhook secrets by artifact: %w", err)
	}
	defer rows.Close()
	return scanWebhookSecretRows(rows)
}

// GetWebhookSecretByArtifactAndName returns a single secret by artifact UUID and name.
func (s *EventSQLStore) GetWebhookSecretByArtifactAndName(artifactUUID, name string) (*models.WebhookSecret, error) {
	q := s.rebind(`SELECT uuid, gateway_id, artifact_uuid, name, display_name, ciphertext, status, created_at, updated_at
        FROM webhook_secrets WHERE artifact_uuid = ? AND name = ?`)
	row := s.db.QueryRow(q, artifactUUID, name)
	ws, err := scanWebhookSecretRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, gwstorage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get webhook secret by artifact and name: %w", err)
	}
	return ws, nil
}

// GetWebhookSecretByUUID returns a single secret by its UUID.
func (s *EventSQLStore) GetWebhookSecretByUUID(uuid string) (*models.WebhookSecret, error) {
	q := s.rebind(`SELECT uuid, gateway_id, artifact_uuid, name, display_name, ciphertext, status, created_at, updated_at
        FROM webhook_secrets WHERE uuid = ?`)
	row := s.db.QueryRow(q, uuid)
	ws, err := scanWebhookSecretRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, gwstorage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get webhook secret by UUID: %w", err)
	}
	return ws, nil
}

// UpdateWebhookSecret updates the ciphertext and updated_at of an existing secret.
func (s *EventSQLStore) UpdateWebhookSecret(secret *models.WebhookSecret) error {
	now := time.Now().UTC()
	q := s.rebind(`UPDATE webhook_secrets SET ciphertext = ?, updated_at = ? WHERE uuid = ?`)
	result, err := s.db.Exec(q, string(secret.Ciphertext), now, secret.UUID)
	if err != nil {
		return fmt.Errorf("failed to update webhook secret: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected after update: %w", err)
	}
	if n == 0 {
		return gwstorage.ErrNotFound
	}
	secret.UpdatedAt = now
	return nil
}

// DeleteWebhookSecret removes a webhook secret by artifact UUID and name.
func (s *EventSQLStore) DeleteWebhookSecret(artifactUUID, name string) error {
	q := s.rebind(`DELETE FROM webhook_secrets WHERE artifact_uuid = ? AND name = ?`)
	result, err := s.db.Exec(q, artifactUUID, name)
	if err != nil {
		return fmt.Errorf("failed to delete webhook secret: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected after delete: %w", err)
	}
	if n == 0 {
		return gwstorage.ErrNotFound
	}
	return nil
}

// GetAllWebhookSecrets returns every webhook secret in the store.
func (s *EventSQLStore) GetAllWebhookSecrets() ([]*models.WebhookSecret, error) {
	q := `SELECT uuid, gateway_id, artifact_uuid, name, display_name, ciphertext, status, created_at, updated_at
        FROM webhook_secrets`
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("failed to query all webhook secrets: %w", err)
	}
	defer rows.Close()
	return scanWebhookSecretRows(rows)
}

func scanWebhookSecretRows(rows *sql.Rows) ([]*models.WebhookSecret, error) {
	var out []*models.WebhookSecret
	for rows.Next() {
		ws, err := scanWebhookSecretRowFields(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ws)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return out, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanWebhookSecretRow(row *sql.Row) (*models.WebhookSecret, error) {
	return scanWebhookSecretRowFields(row)
}

func scanWebhookSecretRowFields(s rowScanner) (*models.WebhookSecret, error) {
	var ws models.WebhookSecret
	var ciphertext string
	err := s.Scan(
		&ws.UUID,
		&ws.GatewayID,
		&ws.ArtifactUUID,
		&ws.Name,
		&ws.DisplayName,
		&ciphertext,
		&ws.Status,
		&ws.CreatedAt,
		&ws.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	ws.Ciphertext = []byte(ciphertext)
	return &ws, nil
}
