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

	"github.com/jackc/pgx/v5/pgconn"
	sqlite3 "github.com/mattn/go-sqlite3"

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
	if s.ValueScope == "" {
		s.ValueScope = model.SecretDefaultValueScope
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
			ciphertext, hash, type, provider, status, value_scope,
			created_at, created_by, updated_at, updated_by
		) VALUES (?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)

	_, err := r.db.Exec(query,
		s.UUID, s.OrganizationID, s.Handle, s.DisplayName, s.Description,
		s.Ciphertext, s.Hash, s.Type, s.Provider, s.Status, s.ValueScope,
		s.CreatedAt, s.CreatedBy, s.UpdatedAt, s.UpdatedBy,
	)
	if err != nil {
		if isSecretUniqueViolation(err) {
			return constants.ErrSecretAlreadyExists
		}
		return fmt.Errorf("failed to create secret: %w", err)
	}
	return nil
}

func (r *SecretRepo) GetByHandle(orgID, handle string) (*model.Secret, error) {
	query := r.db.Rebind(`
		SELECT uuid, organization_id, handle, project_id, display_name, description,
		       ciphertext, hash, type, provider, status, value_scope,
		       created_at, created_by, updated_at, updated_by
		FROM secrets
		WHERE organization_id = ? AND handle = ? AND project_id IS NULL
	`)

	s := &model.Secret{}
	err := r.db.QueryRow(query, orgID, handle).Scan(
		&s.UUID, &s.OrganizationID, &s.Handle, &s.ProjectID, &s.DisplayName, &s.Description,
		&s.Ciphertext, &s.Hash, &s.Type, &s.Provider, &s.Status, &s.ValueScope,
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

func (r *SecretRepo) List(orgID string, limit, offset int, updatedAfter *time.Time) ([]*model.Secret, error) {
	var (
		query string
		args  []interface{}
	)

	const cols = `SELECT uuid, organization_id, handle, project_id, display_name, description,
		       ciphertext, hash, type, provider, status, value_scope,
		       created_at, created_by, updated_at, updated_by FROM secrets`

	if updatedAfter != nil {
		query = r.db.Rebind(cols + ` WHERE organization_id = ? AND project_id IS NULL AND updated_at > ? ORDER BY updated_at DESC LIMIT ? OFFSET ?`)
		args = []interface{}{orgID, *updatedAfter, limit, offset}
	} else {
		query = r.db.Rebind(cols + ` WHERE organization_id = ? AND project_id IS NULL ORDER BY created_at DESC LIMIT ? OFFSET ?`)
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
			&s.Ciphertext, &s.Hash, &s.Type, &s.Provider, &s.Status, &s.ValueScope,
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
func (r *SecretRepo) ListByHandles(orgID string, handles []string, updatedAfter *time.Time, valueScopes []string) ([]*model.Secret, error) {
	if len(handles) == 0 {
		return nil, nil
	}

	const cols = `SELECT uuid, organization_id, handle, project_id, display_name, description,
		       ciphertext, hash, type, provider, status, value_scope,
		       created_at, created_by, updated_at, updated_by FROM secrets`

	args := make([]interface{}, 0, len(handles)+len(valueScopes)+3)
	args = append(args, orgID)

	handlePlaceholders := make([]string, len(handles))
	for i, h := range handles {
		handlePlaceholders[i] = "?"
		args = append(args, h)
	}
	inClause := `handle IN (` + strings.Join(handlePlaceholders, ",") + `)`

	scopeClause := ""
	if len(valueScopes) > 0 {
		scopePlaceholders := make([]string, len(valueScopes))
		for i, s := range valueScopes {
			scopePlaceholders[i] = "?"
			args = append(args, s)
		}
		scopeClause = ` AND value_scope IN (` + strings.Join(scopePlaceholders, ",") + `)`
	}

	var query string
	if updatedAfter != nil {
		query = r.db.Rebind(cols + ` WHERE organization_id = ? AND project_id IS NULL AND ` + inClause + scopeClause + ` AND updated_at > ? ORDER BY updated_at DESC`)
		args = append(args, *updatedAfter)
	} else {
		query = r.db.Rebind(cols + ` WHERE organization_id = ? AND project_id IS NULL AND ` + inClause + scopeClause + ` ORDER BY created_at DESC`)
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
			&s.Ciphertext, &s.Hash, &s.Type, &s.Provider, &s.Status, &s.ValueScope,
			&s.CreatedAt, &s.CreatedBy, &s.UpdatedAt, &s.UpdatedBy,
		); err != nil {
			return nil, fmt.Errorf("failed to scan secret row: %w", err)
		}
		secrets = append(secrets, s)
	}
	return secrets, rows.Err()
}

func (r *SecretRepo) Count(orgID string) (int, error) {
	var count int
	query := r.db.Rebind(`SELECT COUNT(*) FROM secrets WHERE organization_id = ? AND project_id IS NULL`)
	if err := r.db.QueryRow(query, orgID).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count secrets: %w", err)
	}
	return count, nil
}

func (r *SecretRepo) Update(s *model.Secret) error {
	s.UpdatedAt = time.Now()

	query := r.db.Rebind(`
		UPDATE secrets
		SET display_name = ?, description = ?, ciphertext = ?, hash = ?,
		    updated_at = ?, updated_by = ?
		WHERE organization_id = ? AND handle = ?
	`)

	result, err := r.db.Exec(query,
		s.DisplayName, s.Description, s.Ciphertext, s.Hash,
		s.UpdatedAt, s.UpdatedBy,
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

// FindRefsAndSoftDelete checks for active artifact references and deprecates the
// secret in a single transaction, eliminating the TOCTOU window that exists
// when replicas run FindRefs and SoftDelete as separate operations.
// Returns the references without deprecating if any are found.
func (r *SecretRepo) FindRefsAndSoftDelete(orgID, handle, updatedBy string) ([]model.SecretReference, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	refsQuery := r.db.Rebind(`
		SELECT DISTINCT art.handle, art.name, art.kind
		FROM artifact_secret_refs asr
		JOIN artifacts art ON art.uuid = asr.artifact_uuid
		WHERE asr.organization_id = ? AND asr.secret_handle = ?
	`)
	rows, err := tx.Query(refsQuery, orgID, handle)
	if err != nil {
		return nil, fmt.Errorf("failed to find secret refs: %w", err)
	}
	var refs []model.SecretReference
	for rows.Next() {
		var ref model.SecretReference
		if err := rows.Scan(&ref.Handle, &ref.Name, &ref.Type); err != nil {
			rows.Close()
			return nil, err
		}
		refs = append(refs, ref)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(refs) > 0 {
		return refs, nil
	}

	deleteQuery := r.db.Rebind(`
		UPDATE secrets
		SET status = 'DEPRECATED', updated_at = ?, updated_by = ?
		WHERE organization_id = ? AND handle = ?
	`)
	result, err := tx.Exec(deleteQuery, time.Now(), updatedBy, orgID, handle)
	if err != nil {
		return nil, fmt.Errorf("failed to deprecate secret: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, constants.ErrSecretNotFound
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit secret deletion: %w", err)
	}
	return nil, nil
}

func (r *SecretRepo) FindRefs(orgID, handle string) ([]model.SecretReference, error) {
	query := r.db.Rebind(`
		SELECT DISTINCT art.handle, art.name, art.kind
		FROM artifact_secret_refs asr
		JOIN artifacts art ON art.uuid = asr.artifact_uuid
		WHERE asr.organization_id = ? AND asr.secret_handle = ?
	`)

	rows, err := r.db.Query(query, orgID, handle)
	if err != nil {
		return nil, fmt.Errorf("failed to find secret refs: %w", err)
	}
	defer rows.Close()

	var refs []model.SecretReference
	for rows.Next() {
		var ref model.SecretReference
		if err := rows.Scan(&ref.Handle, &ref.Name, &ref.Type); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

func isSecretUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey ||
			sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique
	}
	lowerMsg := strings.ToLower(err.Error())
	return strings.Contains(lowerMsg, "duplicate key") ||
		strings.Contains(lowerMsg, "unique constraint failed")
}

func (r *SecretRepo) Exists(orgID, handle string) (bool, error) {
	var count int
	query := r.db.Rebind(`
		SELECT COUNT(*) FROM secrets
		WHERE organization_id = ? AND project_id IS NULL AND handle = ? AND status = 'ACTIVE'
	`)
	if err := r.db.QueryRow(query, orgID, handle).Scan(&count); err != nil {
		return false, fmt.Errorf("failed to check secret existence: %w", err)
	}
	return count > 0, nil
}
