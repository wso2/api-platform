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
	"time"

	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
)

// APIKeyRepo implements APIKeyRepository
type APIKeyRepo struct {
	db *database.DB
}

// NewAPIKeyRepo creates a new API key repository
func NewAPIKeyRepo(db *database.DB) APIKeyRepository {
	return &APIKeyRepo{db: db}
}

// Create inserts a new API key record
func (r *APIKeyRepo) Create(key *model.APIKey) error {
	key.CreatedAt = time.Now()
	key.UpdatedAt = time.Now()

	query := `
		INSERT INTO api_keys (uuid, artifact_uuid, name, masked_api_key, api_key_hashes, status, created_at, created_by, updated_at, expires_at, provisioned_by, allowed_targets)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(r.db.Rebind(query),
		key.UUID, key.ArtifactUUID, key.Name, key.MaskedAPIKey, key.APIKeyHashes,
		key.Status, key.CreatedAt, key.CreatedBy, key.UpdatedAt, key.ExpiresAt,
		key.ProvisionedBy, key.AllowedTargets,
	)
	return err
}

// Update modifies an existing API key record identified by (artifact_uuid, name)
func (r *APIKeyRepo) Update(key *model.APIKey) error {
	key.UpdatedAt = time.Now()

	query := `
		UPDATE api_keys
		SET masked_api_key = ?, api_key_hashes = ?, status = ?, updated_at = ?, expires_at = ?
		WHERE artifact_uuid = ? AND name = ?
	`
	result, err := r.db.Exec(r.db.Rebind(query),
		key.MaskedAPIKey, key.APIKeyHashes, key.Status, key.UpdatedAt, key.ExpiresAt,
		key.ArtifactUUID, key.Name,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("api key not found")
	}
	return nil
}

// Revoke marks an API key as revoked
func (r *APIKeyRepo) Revoke(artifactUUID, name string) error {
	query := `
		UPDATE api_keys
		SET status = 'revoked', updated_at = ?
		WHERE artifact_uuid = ? AND name = ?
	`
	result, err := r.db.Exec(r.db.Rebind(query), time.Now(), artifactUUID, name)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("api key not found")
	}
	return nil
}

// ListByArtifact retrieves all API keys for a given artifact UUID
func (r *APIKeyRepo) ListByArtifact(artifactUUID string) ([]*model.APIKey, error) {
	query := `
		SELECT uuid, artifact_uuid, name, masked_api_key, api_key_hashes, status, created_at, created_by, updated_at, expires_at, provisioned_by, allowed_targets
		FROM api_keys
		WHERE artifact_uuid = ?
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(r.db.Rebind(query), artifactUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*model.APIKey
	for rows.Next() {
		key := &model.APIKey{}
		var provisionedBy sql.NullString
		if err := rows.Scan(
			&key.UUID, &key.ArtifactUUID, &key.Name, &key.MaskedAPIKey, &key.APIKeyHashes,
			&key.Status, &key.CreatedAt, &key.CreatedBy, &key.UpdatedAt, &key.ExpiresAt,
			&provisionedBy, &key.AllowedTargets,
		); err != nil {
			return nil, err
		}
		if provisionedBy.Valid {
			key.ProvisionedBy = &provisionedBy.String
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

// Delete removes an API key record permanently
func (r *APIKeyRepo) Delete(artifactUUID, name string) error {
	query := `DELETE FROM api_keys WHERE artifact_uuid = ? AND name = ?`
	result, err := r.db.Exec(r.db.Rebind(query), artifactUUID, name)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("api key not found")
	}
	return nil
}

// ListLLMAPIKeysByUser retrieves all API keys for LLM providers and proxies created by a given user within an org.
func (r *APIKeyRepo) ListLLMAPIKeysByUser(orgUUID, username string) ([]*model.UserAPIKey, error) {
	query := `
		SELECT ak.uuid, ak.artifact_uuid, ak.name, ak.masked_api_key, ak.api_key_hashes,
		       ak.status, ak.created_at, ak.created_by, ak.updated_at, ak.expires_at,
		       ak.provisioned_by, ak.allowed_targets,
		       a.handle, a.kind
		FROM api_keys ak
		JOIN artifacts a ON a.uuid = ak.artifact_uuid
		WHERE ak.created_by = ?
		  AND a.organization_uuid = ?
		  AND a.kind IN ('LlmProvider', 'LlmProxy')
		ORDER BY ak.created_at DESC
	`
	rows, err := r.db.Query(r.db.Rebind(query), username, orgUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*model.UserAPIKey
	for rows.Next() {
		key := &model.UserAPIKey{}
		var provisionedBy sql.NullString
		if err := rows.Scan(
			&key.UUID, &key.ArtifactUUID, &key.Name, &key.MaskedAPIKey, &key.APIKeyHashes,
			&key.Status, &key.CreatedAt, &key.CreatedBy, &key.UpdatedAt, &key.ExpiresAt,
			&provisionedBy, &key.AllowedTargets,
			&key.ArtifactHandle, &key.ArtifactKind,
		); err != nil {
			return nil, err
		}
		if provisionedBy.Valid {
			key.ProvisionedBy = &provisionedBy.String
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

// GetByArtifactAndName retrieves an API key by artifact UUID and name
func (r *APIKeyRepo) GetByArtifactAndName(artifactUUID, name string) (*model.APIKey, error) {
	key := &model.APIKey{}
	var provisionedBy sql.NullString
	query := `
		SELECT uuid, artifact_uuid, name, masked_api_key, api_key_hashes, status, created_at, created_by, updated_at, expires_at, provisioned_by, allowed_targets
		FROM api_keys
		WHERE artifact_uuid = ? AND name = ?
	`
	err := r.db.QueryRow(r.db.Rebind(query), artifactUUID, name).Scan(
		&key.UUID, &key.ArtifactUUID, &key.Name, &key.MaskedAPIKey, &key.APIKeyHashes,
		&key.Status, &key.CreatedAt, &key.CreatedBy, &key.UpdatedAt, &key.ExpiresAt,
		&provisionedBy, &key.AllowedTargets,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if provisionedBy.Valid {
		key.ProvisionedBy = &provisionedBy.String
	}
	return key, nil
}
