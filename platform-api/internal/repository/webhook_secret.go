/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

type WebhookSecretRepo struct {
	db *database.DB
}

func NewWebhookSecretRepo(db *database.DB) WebhookSecretRepository {
	return &WebhookSecretRepo{db: db}
}

const webhookSecretMetadataCols = `SELECT uuid, organization_uuid, handle, name, type,
	       created_by, created_at, updated_by, updated_at FROM webhook_secrets_metadata`

const webhookSecretCols = `SELECT uuid, secret_uuid, secret_version, secret_enc,
	       secret_hash, masked_secret, created_by, created_at FROM webhook_secrets`

func scanWebhookSecretMetadata(row interface {
	Scan(...interface{}) error
}) (*model.WebhookSecretMetadata, error) {
	s := &model.WebhookSecretMetadata{}
	var createdBy, updatedBy sql.NullString
	var createdAt, updatedAt sql.NullTime
	if err := row.Scan(&s.UUID, &s.OrganizationID, &s.Handle, &s.Name, &s.Type,
		&createdBy, &createdAt, &updatedBy, &updatedAt); err != nil {
		return nil, err
	}
	s.CreatedBy, s.UpdatedBy = createdBy.String, updatedBy.String
	s.CreatedAt, s.UpdatedAt = createdAt.Time, updatedAt.Time
	return s, nil
}

func scanWebhookSecret(row interface {
	Scan(...interface{}) error
}) (*model.WebhookSecret, error) {
	w := &model.WebhookSecret{}
	var createdBy sql.NullString
	var createdAt sql.NullTime
	if err := row.Scan(&w.UUID, &w.SecretID, &w.Version, &w.SecretEnc,
		&w.SecretHash, &w.MaskedSecret, &createdBy, &createdAt); err != nil {
		return nil, err
	}
	w.CreatedBy = createdBy.String
	w.CreatedAt = createdAt.Time
	return w, nil
}

func (r *WebhookSecretRepo) Create(info *model.WebhookSecretMetadata, first *model.WebhookSecret) error {
	now := time.Now().UTC()
	info.CreatedAt, info.UpdatedAt = now, now
	first.CreatedAt = now

	var err error
	if info.UUID == "" {
		if info.UUID, err = utils.GenerateUUID(); err != nil {
			return fmt.Errorf("failed to generate secret info UUID: %w", err)
		}
	}
	if first.UUID == "" {
		if first.UUID, err = utils.GenerateUUID(); err != nil {
			return fmt.Errorf("failed to generate webhook secret UUID: %w", err)
		}
	}
	first.SecretID = info.UUID
	first.Version = 1

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.Exec(r.db.Rebind(`
		INSERT INTO webhook_secrets_metadata (uuid, organization_uuid, handle, name, type, last_version,
			created_by, created_at, updated_by, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		info.UUID, info.OrganizationID, info.Handle, info.Name, info.Type, first.Version,
		info.CreatedBy, info.CreatedAt, info.UpdatedBy, info.UpdatedAt)
	if err != nil {
		if isSecretUniqueViolation(err) {
			return apperror.WebhookSecretExists.Wrap(err)
		}
		return fmt.Errorf("failed to insert secret info: %w", err)
	}
	if err := insertWebhookSecret(r.db, tx, first); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit webhook secret: %w", err)
	}
	return nil
}

func insertWebhookSecret(db *database.DB, tx *sql.Tx, w *model.WebhookSecret) error {
	_, err := tx.Exec(db.Rebind(`
		INSERT INTO webhook_secrets (uuid, secret_uuid, secret_version, secret_enc,
			secret_hash, masked_secret, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`),
		w.UUID, w.SecretID, w.Version, w.SecretEnc,
		w.SecretHash, w.MaskedSecret, w.CreatedBy, w.CreatedAt)
	if err != nil {
		if isSecretUniqueViolation(err) {
			// Backstop for the (secret_uuid, secret_version) uniqueness; the
			// row lock taken by the callers makes this unreachable in practice.
			return apperror.WebhookSecretVersionLimit.Wrap(err)
		}
		return fmt.Errorf("failed to insert webhook secret version: %w", err)
	}
	return nil
}

func (r *WebhookSecretRepo) GetMetadata(orgID, handle string) (*model.WebhookSecretMetadata, error) {
	query := r.db.Rebind(webhookSecretMetadataCols + ` WHERE organization_uuid = ? AND handle = ?`)
	s, err := scanWebhookSecretMetadata(r.db.QueryRow(query, orgID, handle))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.WebhookSecretNotFound.Wrap(err)
		}
		return nil, fmt.Errorf("failed to get secret info: %w", err)
	}
	return s, nil
}

func (r *WebhookSecretRepo) ListMetadata(orgID string, limit, offset int) ([]*model.WebhookSecretMetadata, int, error) {
	var total int
	if err := r.db.QueryRow(r.db.Rebind(`SELECT COUNT(*) FROM webhook_secrets_metadata WHERE organization_uuid = ?`), orgID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count secret info: %w", err)
	}
	pageClause, pageArgs := r.db.PaginationClause(limit, offset)
	query := r.db.Rebind(webhookSecretMetadataCols + ` WHERE organization_uuid = ? ORDER BY created_at DESC, uuid ` + pageClause)
	rows, err := r.db.Query(query, append([]interface{}{orgID}, pageArgs...)...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list secret info: %w", err)
	}
	defer rows.Close()
	var out []*model.WebhookSecretMetadata
	for rows.Next() {
		s, err := scanWebhookSecretMetadata(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan secret info: %w", err)
		}
		out = append(out, s)
	}
	return out, total, rows.Err()
}

func (r *WebhookSecretRepo) ListVersions(secretUUID string) ([]*model.WebhookSecret, error) {
	query := r.db.Rebind(webhookSecretCols + ` WHERE secret_uuid = ? ORDER BY secret_version DESC`)
	rows, err := r.db.Query(query, secretUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to list webhook secret versions: %w", err)
	}
	defer rows.Close()
	var out []*model.WebhookSecret
	for rows.Next() {
		w, err := scanWebhookSecret(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan webhook secret version: %w", err)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (r *WebhookSecretRepo) ListVersionsForOrg(orgID, secretType string) ([]*model.WebhookSecret, error) {
	query := r.db.Rebind(`SELECT w.uuid, w.secret_uuid, w.secret_version, w.secret_enc,
	       w.secret_hash, w.masked_secret, w.created_by, w.created_at
	FROM webhook_secrets w
	JOIN webhook_secrets_metadata i ON i.uuid = w.secret_uuid
	WHERE i.organization_uuid = ? AND i.type = ?
	ORDER BY w.created_at DESC, w.secret_version DESC`)
	rows, err := r.db.Query(query, orgID, secretType)
	if err != nil {
		return nil, fmt.Errorf("failed to list webhook secret versions for org: %w", err)
	}
	defer rows.Close()
	var out []*model.WebhookSecret
	for rows.Next() {
		w, err := scanWebhookSecret(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan webhook secret version: %w", err)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (r *WebhookSecretRepo) GetVersionByHash(orgID, secretType, hash string) (*model.WebhookSecret, error) {
	query := r.db.Rebind(`SELECT w.uuid, w.secret_uuid, w.secret_version, w.secret_enc,
	       w.secret_hash, w.masked_secret, w.created_by, w.created_at
	FROM webhook_secrets w
	JOIN webhook_secrets_metadata i ON i.uuid = w.secret_uuid
	WHERE i.organization_uuid = ? AND i.type = ? AND w.secret_hash = ?`)
	w, err := scanWebhookSecret(r.db.QueryRow(query, orgID, secretType, hash))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to look up webhook secret by hash: %w", err)
	}
	return w, nil
}

// lockMetadata takes a write lock on the webhook_secrets_metadata row so concurrent rotations
// and deletions of the same secret serialize on the version count.
func (r *WebhookSecretRepo) lockMetadata(tx *sql.Tx, secretUUID, actor string) error {
	res, err := tx.Exec(r.db.Rebind(`UPDATE webhook_secrets_metadata SET updated_at = ?, updated_by = ? WHERE uuid = ?`),
		time.Now().UTC(), actor, secretUUID)
	if err != nil {
		return fmt.Errorf("failed to lock secret info: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperror.WebhookSecretNotFound.New()
	}
	return nil
}

func (r *WebhookSecretRepo) AddVersion(secretUUID string, maxVersions int, w *model.WebhookSecret) error {
	var err error
	if w.UUID == "" {
		if w.UUID, err = utils.GenerateUUID(); err != nil {
			return fmt.Errorf("failed to generate webhook secret UUID: %w", err)
		}
	}
	w.SecretID = secretUUID
	w.CreatedAt = time.Now().UTC()

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if err := r.lockMetadata(tx, secretUUID, w.CreatedBy); err != nil {
		return err
	}
	var count, maxVersion int
	if err := tx.QueryRow(r.db.Rebind(`SELECT COUNT(*), COALESCE(MAX(secret_version), 0)
		FROM webhook_secrets WHERE secret_uuid = ?`), secretUUID).Scan(&count, &maxVersion); err != nil {
		return fmt.Errorf("failed to count webhook secret versions: %w", err)
	}
	if count >= maxVersions {
		return apperror.WebhookSecretVersionLimit.New()
	}
	// A version number is never handed out twice: last_version only grows, so deleting the
	// newest version cannot make its number available again, and a delete aimed at a version
	// that has since been replaced cannot remove the credential issued after it.
	var lastVersion int
	if err := tx.QueryRow(r.db.Rebind(`SELECT last_version FROM webhook_secrets_metadata WHERE uuid = ?`),
		secretUUID).Scan(&lastVersion); err != nil {
		return fmt.Errorf("failed to read the last webhook secret version: %w", err)
	}
	if maxVersion > lastVersion {
		lastVersion = maxVersion
	}
	w.Version = lastVersion + 1
	if _, err := tx.Exec(r.db.Rebind(`UPDATE webhook_secrets_metadata SET last_version = ? WHERE uuid = ?`),
		w.Version, secretUUID); err != nil {
		return fmt.Errorf("failed to record the last webhook secret version: %w", err)
	}
	if err := insertWebhookSecret(r.db, tx, w); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit webhook secret rotation: %w", err)
	}
	return nil
}

func (r *WebhookSecretRepo) DeleteVersion(secretUUID string, version int, actor string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if err := r.lockMetadata(tx, secretUUID, actor); err != nil {
		return err
	}
	var count int
	if err := tx.QueryRow(r.db.Rebind(`SELECT COUNT(*) FROM webhook_secrets WHERE secret_uuid = ?`),
		secretUUID).Scan(&count); err != nil {
		return fmt.Errorf("failed to count webhook secret versions: %w", err)
	}
	var exists int
	if err := tx.QueryRow(r.db.Rebind(`SELECT COUNT(*) FROM webhook_secrets WHERE secret_uuid = ? AND secret_version = ?`),
		secretUUID, version).Scan(&exists); err != nil {
		return fmt.Errorf("failed to look up webhook secret version: %w", err)
	}
	if exists == 0 {
		return apperror.WebhookSecretNotFound.New()
	}
	if count <= 1 {
		return apperror.WebhookSecretLastVersion.New()
	}
	if _, err := tx.Exec(r.db.Rebind(`DELETE FROM webhook_secrets WHERE secret_uuid = ? AND secret_version = ?`),
		secretUUID, version); err != nil {
		return fmt.Errorf("failed to delete webhook secret version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit webhook secret version delete: %w", err)
	}
	return nil
}

func (r *WebhookSecretRepo) Delete(orgID, handle string) error {
	res, err := r.db.Exec(r.db.Rebind(`DELETE FROM webhook_secrets_metadata WHERE organization_uuid = ? AND handle = ?`), orgID, handle)
	if err != nil {
		return fmt.Errorf("failed to delete secret info: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperror.WebhookSecretNotFound.New()
	}
	return nil
}
