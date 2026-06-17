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
	"platform-api/src/internal/utils"
)

type SecretRepo struct {
	db *database.DB
}

func NewSecretRepo(db *database.DB) SecretRepository {
	return &SecretRepo{db: db}
}

func (r *SecretRepo) Create(s *model.Secret) error {
	now := time.Now()
	s.CreatedAt = now
	s.UpdatedAt = now

	if s.UUID == "" {
		id, err := utils.GenerateUUID()
		if err != nil {
			return fmt.Errorf("failed to generate secret UUID: %w", err)
		}
		s.UUID = id
	}
	if s.Environment == "" {
		s.Environment = model.SecretDefaultEnvironment
	}
	if s.Type == "" {
		s.Type = model.SecretTypeGeneric
	}
	if s.Provider == "" {
		s.Provider = model.SecretProviderInHouse
	}
	if s.Status == "" {
		s.Status = model.SecretStatusActive
	}

	query := r.db.Rebind(`
		INSERT INTO secrets (
			uuid, organization_id, handle, project_id, display_name, description,
			ciphertext, hash, type, provider, status, environment,
			created_at, created_by, updated_at, updated_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)

	_, err := r.db.Exec(query,
		s.UUID, s.OrganizationID, s.Handle, s.ProjectID, s.DisplayName, s.Description,
		s.Ciphertext, s.Hash, s.Type, s.Provider, s.Status, s.Environment,
		s.CreatedAt, s.CreatedBy, s.UpdatedAt, s.UpdatedBy,
	)
	if err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}
	return nil
}

func (r *SecretRepo) GetByHandle(orgID, handle string) (*model.Secret, error) {
	query := r.db.Rebind(`
		SELECT uuid, organization_id, handle, project_id, display_name, description,
		       ciphertext, hash, type, provider, status, environment,
		       created_at, created_by, updated_at, updated_by
		FROM secrets
		WHERE organization_id = ? AND handle = ?
	`)

	s := &model.Secret{}
	err := r.db.QueryRow(query, orgID, handle).Scan(
		&s.UUID, &s.OrganizationID, &s.Handle, &s.ProjectID, &s.DisplayName, &s.Description,
		&s.Ciphertext, &s.Hash, &s.Type, &s.Provider, &s.Status, &s.Environment,
		&s.CreatedAt, &s.CreatedBy, &s.UpdatedAt, &s.UpdatedBy,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrSecretNotFound
		}
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}
	return s, nil
}

func (r *SecretRepo) List(orgID string, projectID *string, limit, offset int, updatedAfter *time.Time) ([]*model.Secret, error) {
	var (
		query string
		args  []interface{}
	)

	const cols = `SELECT uuid, organization_id, handle, project_id, display_name, description,
		       ciphertext, hash, type, provider, status, environment,
		       created_at, created_by, updated_at, updated_by FROM secrets`

	switch {
	case projectID != nil && updatedAfter != nil:
		query = r.db.Rebind(cols + ` WHERE organization_id = ? AND project_id = ? AND updated_at > ? ORDER BY updated_at DESC LIMIT ? OFFSET ?`)
		args = []interface{}{orgID, *projectID, *updatedAfter, limit, offset}
	case projectID != nil:
		query = r.db.Rebind(cols + ` WHERE organization_id = ? AND project_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`)
		args = []interface{}{orgID, *projectID, limit, offset}
	case updatedAfter != nil:
		query = r.db.Rebind(cols + ` WHERE organization_id = ? AND updated_at > ? ORDER BY updated_at DESC LIMIT ? OFFSET ?`)
		args = []interface{}{orgID, *updatedAfter, limit, offset}
	default:
		query = r.db.Rebind(cols + ` WHERE organization_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`)
		args = []interface{}{orgID, limit, offset}
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}
	defer rows.Close()

	var secrets []*model.Secret
	for rows.Next() {
		s := &model.Secret{}
		if err := rows.Scan(
			&s.UUID, &s.OrganizationID, &s.Handle, &s.ProjectID, &s.DisplayName, &s.Description,
			&s.Ciphertext, &s.Hash, &s.Type, &s.Provider, &s.Status, &s.Environment,
			&s.CreatedAt, &s.CreatedBy, &s.UpdatedAt, &s.UpdatedBy,
		); err != nil {
			return nil, fmt.Errorf("failed to scan secret row: %w", err)
		}
		secrets = append(secrets, s)
	}
	return secrets, rows.Err()
}

// ListByHandles returns secrets for the given org whose handle is in the provided list.
// If updatedAfter is set, only secrets updated after that time are returned.
// Returns an empty slice (not an error) when handles is empty.
func (r *SecretRepo) ListByHandles(orgID string, handles []string, updatedAfter *time.Time) ([]*model.Secret, error) {
	if len(handles) == 0 {
		return nil, nil
	}

	const cols = `SELECT uuid, organization_id, handle, project_id, display_name, description,
		       ciphertext, hash, type, provider, status, environment,
		       created_at, created_by, updated_at, updated_by FROM secrets`

	// Build IN clause placeholders
	placeholders := make([]string, len(handles))
	args := make([]interface{}, 0, len(handles)+3)
	args = append(args, orgID)
	for i, h := range handles {
		placeholders[i] = "?"
		args = append(args, h)
	}
	inClause := strings.Join(placeholders, ",")

	var query string
	if updatedAfter != nil {
		query = r.db.Rebind(cols + ` WHERE organization_id = ? AND handle IN (` + inClause + `) AND updated_at > ? ORDER BY updated_at DESC`)
		args = append(args, *updatedAfter)
	} else {
		query = r.db.Rebind(cols + ` WHERE organization_id = ? AND handle IN (` + inClause + `) ORDER BY created_at DESC`)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets by handles: %w", err)
	}
	defer rows.Close()

	var secrets []*model.Secret
	for rows.Next() {
		s := &model.Secret{}
		if err := rows.Scan(
			&s.UUID, &s.OrganizationID, &s.Handle, &s.ProjectID, &s.DisplayName, &s.Description,
			&s.Ciphertext, &s.Hash, &s.Type, &s.Provider, &s.Status, &s.Environment,
			&s.CreatedAt, &s.CreatedBy, &s.UpdatedAt, &s.UpdatedBy,
		); err != nil {
			return nil, fmt.Errorf("failed to scan secret row: %w", err)
		}
		secrets = append(secrets, s)
	}
	return secrets, rows.Err()
}

func (r *SecretRepo) Count(orgID string, projectID *string) (int, error) {
	var (
		query string
		args  []interface{}
		count int
	)

	if projectID != nil {
		query = r.db.Rebind(`SELECT COUNT(*) FROM secrets WHERE organization_id = ? AND project_id = ?`)
		args = []interface{}{orgID, *projectID}
	} else {
		query = r.db.Rebind(`SELECT COUNT(*) FROM secrets WHERE organization_id = ?`)
		args = []interface{}{orgID}
	}

	if err := r.db.QueryRow(query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count secrets: %w", err)
	}
	return count, nil
}

func (r *SecretRepo) Update(s *model.Secret) error {
	s.UpdatedAt = time.Now()

	query := r.db.Rebind(`
		UPDATE secrets
		SET display_name = ?, description = ?, ciphertext = ?, hash = ?,
		    environment = ?, updated_at = ?, updated_by = ?
		WHERE organization_id = ? AND handle = ?
	`)

	result, err := r.db.Exec(query,
		s.DisplayName, s.Description, s.Ciphertext, s.Hash,
		s.Environment, s.UpdatedAt, s.UpdatedBy,
		s.OrganizationID, s.Handle,
	)
	if err != nil {
		return fmt.Errorf("failed to update secret: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return constants.ErrSecretNotFound
	}
	return nil
}

func (r *SecretRepo) SoftDelete(orgID, handle, updatedBy string) error {
	query := r.db.Rebind(`
		UPDATE secrets
		SET status = 'DEPRECATED', updated_at = ?, updated_by = ?
		WHERE organization_id = ? AND handle = ?
	`)

	result, err := r.db.Exec(query, time.Now(), updatedBy, orgID, handle)
	if err != nil {
		return fmt.Errorf("failed to deprecate secret: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return constants.ErrSecretNotFound
	}
	return nil
}

func (r *SecretRepo) FindLLMProviderRefs(orgID, handle string) ([]model.SecretReference, error) {
	pattern := fmt.Sprintf(`%%{{ secret "%s" }}%%`, handle)
	query := r.db.Rebind(`
		SELECT art.handle, art.name
		FROM llm_providers lp
		JOIN artifacts art ON lp.uuid = art.uuid
		WHERE art.organization_uuid = ? AND lp.configuration LIKE ?
	`)

	rows, err := r.db.Query(query, orgID, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to scan LLM provider refs: %w", err)
	}
	defer rows.Close()

	var refs []model.SecretReference
	for rows.Next() {
		ref := model.SecretReference{Type: "llm_provider"}
		if err := rows.Scan(&ref.Handle, &ref.Name); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

func (r *SecretRepo) FindAPIRefs(orgID, handle string) ([]model.SecretReference, error) {
	pattern := fmt.Sprintf(`%%{{ secret "%s" }}%%`, handle)
	query := r.db.Rebind(`
		SELECT art.handle, art.name
		FROM rest_apis a
		JOIN artifacts art ON a.uuid = art.uuid
		WHERE art.organization_uuid = ? AND a.configuration LIKE ?
	`)

	rows, err := r.db.Query(query, orgID, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to scan API refs: %w", err)
	}
	defer rows.Close()

	var refs []model.SecretReference
	for rows.Next() {
		ref := model.SecretReference{Type: "api"}
		if err := rows.Scan(&ref.Handle, &ref.Name); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

func (r *SecretRepo) Exists(orgID string, projectID *string, handle string) (bool, error) {
	var count int
	var err error

	if projectID != nil {
		query := r.db.Rebind(`
			SELECT COUNT(*) FROM secrets
			WHERE organization_id = ? AND COALESCE(project_id, '') = ? AND handle = ? AND status = 'ACTIVE'
		`)
		err = r.db.QueryRow(query, orgID, *projectID, handle).Scan(&count)
	} else {
		// org-wide lookup: check project_id IS NULL
		query := r.db.Rebind(`
			SELECT COUNT(*) FROM secrets
			WHERE organization_id = ? AND project_id IS NULL AND handle = ? AND status = 'ACTIVE'
		`)
		err = r.db.QueryRow(query, orgID, handle).Scan(&count)
	}

	if err != nil {
		return false, fmt.Errorf("failed to check secret existence: %w", err)
	}
	return count > 0, nil
}
