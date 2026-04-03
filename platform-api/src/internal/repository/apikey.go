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
	"strings"
	"time"

	"platform-api/src/internal/constants"
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
		INSERT INTO api_keys (uuid, artifact_uuid, name, masked_api_key, api_key_hashes, status, created_at, created_by, updated_at, expires_at, issuer, allowed_targets)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(r.db.Rebind(query),
		key.UUID, key.ArtifactUUID, key.Name, key.MaskedAPIKey, key.APIKeyHashes,
		key.Status, key.CreatedAt, key.CreatedBy, key.UpdatedAt, key.ExpiresAt,
		key.Issuer, key.AllowedTargets,
	)
	return err
}

// Update modifies an existing API key record identified by (artifact_uuid, name)
func (r *APIKeyRepo) Update(key *model.APIKey) error {
	key.UpdatedAt = time.Now()

	query := `
		UPDATE api_keys
		SET masked_api_key = ?, api_key_hashes = ?, status = ?, updated_at = ?, expires_at = ?, issuer = ?
		WHERE artifact_uuid = ? AND name = ?
	`
	result, err := r.db.Exec(r.db.Rebind(query),
		key.MaskedAPIKey, key.APIKeyHashes, key.Status, key.UpdatedAt, key.ExpiresAt, key.Issuer,
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
		SELECT uuid, artifact_uuid, name, masked_api_key, api_key_hashes, status, created_at, created_by, updated_at, expires_at, issuer, allowed_targets
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
		var issuer sql.NullString
		if err := rows.Scan(
			&key.UUID, &key.ArtifactUUID, &key.Name, &key.MaskedAPIKey, &key.APIKeyHashes,
			&key.Status, &key.CreatedAt, &key.CreatedBy, &key.UpdatedAt, &key.ExpiresAt,
			&issuer, &key.AllowedTargets,
		); err != nil {
			return nil, err
		}
		if issuer.Valid {
			key.Issuer = &issuer.String
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

// ListByGatewayAndKind retrieves all API keys for artifacts of the given kind that have
// an active deployment association on the specified gateway within the organisation
// (status_desired in DEPLOYED or UNDEPLOYED).
// When issuer is non-empty only keys whose issuer column matches are returned;
// an empty issuer returns keys regardless of their issuer value.
func (r *APIKeyRepo) ListByGatewayAndKind(gatewayID, orgID, kind, issuer string) ([]*model.APIKey, error) {
	base := `
		SELECT k.uuid, k.artifact_uuid, k.name, k.masked_api_key, k.api_key_hashes,
		       k.status, k.created_at, k.created_by, k.updated_at, k.expires_at,
		       k.issuer, k.allowed_targets
		FROM api_keys k
		INNER JOIN artifacts a ON k.artifact_uuid = a.uuid
		INNER JOIN deployment_status s ON s.artifact_uuid = a.uuid
		WHERE s.gateway_uuid = ?
		  AND s.organization_uuid = ?
		  AND a.kind = ?
		  AND s.status_desired IN ('DEPLOYED', 'UNDEPLOYED')`

	args := []any{gatewayID, orgID, kind}
	if issuer != "" {
		base += "\n\t\t  AND k.issuer = ?"
		args = append(args, issuer)
	}
	base += "\n\t\tORDER BY k.created_at DESC"

	rows, err := r.db.Query(r.db.Rebind(base), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*model.APIKey
	for rows.Next() {
		key := &model.APIKey{}
		var issuerVal sql.NullString
		if err := rows.Scan(
			&key.UUID, &key.ArtifactUUID, &key.Name, &key.MaskedAPIKey, &key.APIKeyHashes,
			&key.Status, &key.CreatedAt, &key.CreatedBy, &key.UpdatedAt, &key.ExpiresAt,
			&issuerVal, &key.AllowedTargets,
		); err != nil {
			return nil, err
		}
		if issuerVal.Valid {
			key.Issuer = &issuerVal.String
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

// ListAPIKeysByUser retrieves API keys created by a given user within an org, optionally filtered by artifact kinds.
// If kinds is empty, all supported kinds (RestApi, LlmProvider, LlmProxy) are returned.
func (r *APIKeyRepo) ListAPIKeysByUser(orgUUID, username string, kinds []string) ([]*model.UserAPIKey, error) {
	if len(kinds) == 0 {
		kinds = []string{constants.RestApi, constants.LLMProvider, constants.LLMProxy}
	}

	placeholders := make([]string, len(kinds))
	args := []any{username, orgUUID}
	for i, k := range kinds {
		placeholders[i] = "?"
		args = append(args, k)
	}

	query := fmt.Sprintf(`
		SELECT ak.uuid, ak.artifact_uuid, ak.name, ak.masked_api_key, ak.api_key_hashes,
		       ak.status, ak.created_at, ak.created_by, ak.updated_at, ak.expires_at,
		       ak.issuer, ak.allowed_targets,
		       a.handle, a.kind
		FROM api_keys ak
		JOIN artifacts a ON a.uuid = ak.artifact_uuid
		WHERE ak.created_by = ?
		  AND a.organization_uuid = ?
		  AND a.kind IN (%s)
		ORDER BY ak.created_at DESC
	`, strings.Join(placeholders, ", "))

	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*model.UserAPIKey
	for rows.Next() {
		key := &model.UserAPIKey{}
		var issuer sql.NullString
		if err := rows.Scan(
			&key.UUID, &key.ArtifactUUID, &key.Name, &key.MaskedAPIKey, &key.APIKeyHashes,
			&key.Status, &key.CreatedAt, &key.CreatedBy, &key.UpdatedAt, &key.ExpiresAt,
			&issuer, &key.AllowedTargets,
			&key.ArtifactHandle, &key.ArtifactKind,
		); err != nil {
			return nil, err
		}
		if issuer.Valid {
			key.Issuer = &issuer.String
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

// GetByArtifactAndName retrieves an API key by artifact UUID and name
func (r *APIKeyRepo) GetByArtifactAndName(artifactUUID, name string) (*model.APIKey, error) {
	key := &model.APIKey{}
	var issuer sql.NullString
	query := `
		SELECT uuid, artifact_uuid, name, masked_api_key, api_key_hashes, status, created_at, created_by, updated_at, expires_at, issuer, allowed_targets
		FROM api_keys
		WHERE artifact_uuid = ? AND name = ?
	`
	err := r.db.QueryRow(r.db.Rebind(query), artifactUUID, name).Scan(
		&key.UUID, &key.ArtifactUUID, &key.Name, &key.MaskedAPIKey, &key.APIKeyHashes,
		&key.Status, &key.CreatedAt, &key.CreatedBy, &key.UpdatedAt, &key.ExpiresAt,
		&issuer, &key.AllowedTargets,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if issuer.Valid {
		key.Issuer = &issuer.String
	}
	return key, nil
}
