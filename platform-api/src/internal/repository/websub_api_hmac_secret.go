/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
 *
 */

package repository

import (
	"database/sql"
	"errors"
	"time"

	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
)

// WebSubAPIHmacSecretRepo handles database operations for WebSub API HMAC secrets
type WebSubAPIHmacSecretRepo struct {
	db *database.DB
}

// NewWebSubAPIHmacSecretRepo creates a new WebSubAPIHmacSecretRepo
func NewWebSubAPIHmacSecretRepo(db *database.DB) *WebSubAPIHmacSecretRepo {
	return &WebSubAPIHmacSecretRepo{db: db}
}

// Create persists a new HMAC secret
func (r *WebSubAPIHmacSecretRepo) Create(secret *model.WebSubAPIHmacSecret) error {
	now := time.Now().UTC()
	secret.CreatedAt = now
	secret.UpdatedAt = now
	query := `
		INSERT INTO websub_api_hmac_secrets (uuid, artifact_uuid, name, display_name, encrypted_secret, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.Exec(r.db.Rebind(query),
		secret.UUID, secret.ArtifactUUID, secret.Name, secret.DisplayName,
		secret.EncryptedSecret, secret.Status, secret.CreatedAt, secret.UpdatedAt,
	)
	return err
}

// GetByArtifactAndName fetches a specific HMAC secret by artifact UUID and name
func (r *WebSubAPIHmacSecretRepo) GetByArtifactAndName(artifactUUID, name string) (*model.WebSubAPIHmacSecret, error) {
	query := `
		SELECT uuid, artifact_uuid, name, display_name, encrypted_secret, status, created_at, updated_at
		FROM websub_api_hmac_secrets
		WHERE artifact_uuid = ? AND name = ?`
	row := r.db.QueryRow(r.db.Rebind(query), artifactUUID, name)
	s := &model.WebSubAPIHmacSecret{}
	var displayName sql.NullString
	if err := row.Scan(&s.UUID, &s.ArtifactUUID, &s.Name, &displayName, &s.EncryptedSecret, &s.Status, &s.CreatedAt, &s.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	s.DisplayName = displayName.String
	return s, nil
}

// ListByArtifact returns all HMAC secrets for an artifact
func (r *WebSubAPIHmacSecretRepo) ListByArtifact(artifactUUID string) ([]*model.WebSubAPIHmacSecret, error) {
	query := `
		SELECT uuid, artifact_uuid, name, display_name, encrypted_secret, status, created_at, updated_at
		FROM websub_api_hmac_secrets
		WHERE artifact_uuid = ?
		ORDER BY created_at ASC`
	rows, err := r.db.Query(r.db.Rebind(query), artifactUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var secrets []*model.WebSubAPIHmacSecret
	for rows.Next() {
		s := &model.WebSubAPIHmacSecret{}
		var displayName sql.NullString
		if err := rows.Scan(&s.UUID, &s.ArtifactUUID, &s.Name, &displayName, &s.EncryptedSecret, &s.Status, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		s.DisplayName = displayName.String
		secrets = append(secrets, s)
	}
	return secrets, rows.Err()
}

// Update replaces the encrypted secret value (used on regenerate)
func (r *WebSubAPIHmacSecretRepo) Update(secret *model.WebSubAPIHmacSecret) error {
	secret.UpdatedAt = time.Now().UTC()
	query := `
		UPDATE websub_api_hmac_secrets
		SET encrypted_secret = ?, updated_at = ?
		WHERE artifact_uuid = ? AND name = ?`
	result, err := r.db.Exec(r.db.Rebind(query),
		secret.EncryptedSecret, secret.UpdatedAt, secret.ArtifactUUID, secret.Name,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// Delete permanently removes a secret
func (r *WebSubAPIHmacSecretRepo) Delete(artifactUUID, name string) error {
	query := `DELETE FROM websub_api_hmac_secrets WHERE artifact_uuid = ? AND name = ?`
	result, err := r.db.Exec(r.db.Rebind(query), artifactUUID, name)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
