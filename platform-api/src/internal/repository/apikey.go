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
		INSERT INTO api_keys (id, artifact_uuid, name, masked_api_key, api_key_hashes, status, created_at, created_by, updated_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(r.db.Rebind(query),
		key.ID, key.ArtifactUUID, key.Name, key.MaskedAPIKey, key.APIKeyHashes,
		key.Status, key.CreatedAt, key.CreatedBy, key.UpdatedAt, key.ExpiresAt,
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

// GetByArtifactAndName retrieves an API key by artifact UUID and name
func (r *APIKeyRepo) GetByArtifactAndName(artifactUUID, name string) (*model.APIKey, error) {
	key := &model.APIKey{}
	query := `
		SELECT id, artifact_uuid, name, masked_api_key, api_key_hashes, status, created_at, created_by, updated_at, expires_at
		FROM api_keys
		WHERE artifact_uuid = ? AND name = ?
	`
	err := r.db.QueryRow(r.db.Rebind(query), artifactUUID, name).Scan(
		&key.ID, &key.ArtifactUUID, &key.Name, &key.MaskedAPIKey, &key.APIKeyHashes,
		&key.Status, &key.CreatedAt, &key.CreatedBy, &key.UpdatedAt, &key.ExpiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return key, nil
}
