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
	if s.Type == "" {
		s.Type = model.SecretTypeGeneric
	}
	if s.Provider == "" {
		s.Provider = model.SecretProviderInHouse
	}
	if s.Status == "" {
		s.Status = model.SecretStatusActive
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	query := r.db.Rebind(`
		INSERT INTO secrets (
			uuid, organization_uuid, handle, name, description,
			ciphertext, hash, type, provider, status,
			created_at, created_by, updated_at, updated_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	_, err = tx.Exec(query,
		s.UUID, s.OrganizationID, s.Handle, s.DisplayName, s.Description,
		s.Ciphertext, s.Hash, s.Type, s.Provider, s.Status,
		s.CreatedAt, s.CreatedBy, s.UpdatedAt, s.UpdatedBy,
	)
	if err != nil {
		if isSecretUniqueViolation(err) {
			return constants.ErrSecretAlreadyExists
		}
		return fmt.Errorf("failed to create secret: %w", err)
	}

	scopeQuery := r.db.Rebind(`
		INSERT INTO secret_scopes (secret_uuid, scope, scope_value)
		VALUES (?, ?, ?)
	`)
	for _, sc := range s.Scopes {
		if _, err := tx.Exec(scopeQuery, s.UUID, sc.Scope, sc.ScopeValue); err != nil {
			return fmt.Errorf("failed to create secret scope: %w", err)
		}
	}

	return tx.Commit()
}

const secretCols = `SELECT uuid, organization_uuid, handle, name, description,
	       ciphertext, hash, type, provider, status,
	       created_at, created_by, updated_at, updated_by FROM secrets`

func scanSecret(row interface {
	Scan(...interface{}) error
}) (*model.Secret, error) {
	s := &model.Secret{}
	err := row.Scan(
		&s.UUID, &s.OrganizationID, &s.Handle, &s.DisplayName, &s.Description,
		&s.Ciphertext, &s.Hash, &s.Type, &s.Provider, &s.Status,
		&s.CreatedAt, &s.CreatedBy, &s.UpdatedAt, &s.UpdatedBy,
	)
	return s, err
}

func (r *SecretRepo) GetByHandle(orgID, handle string) (*model.Secret, error) {
	query := r.db.Rebind(secretCols + `
		WHERE organization_uuid = ? AND handle = ?
	`)
	s, err := scanSecret(r.db.QueryRow(query, orgID, handle))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrSecretNotFound
		}
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}
	return s, nil
}

func (r *SecretRepo) List(orgID string, limit, offset int, updatedAfter *time.Time) ([]*model.Secret, error) {
	// SQL Server has no LIMIT keyword; PaginationClause yields the dialect's
	// row-limiting clause (and its args in the order it expects them).
	pageClause, pageArgs := r.db.PaginationClause(limit, offset)
	var (
		query string
		args  []interface{}
	)
	if updatedAfter != nil {
		query = r.db.Rebind(secretCols + ` WHERE organization_uuid = ? AND updated_at > ? ORDER BY updated_at DESC ` + pageClause)
		args = append([]interface{}{orgID, *updatedAfter}, pageArgs...)
	} else {
		query = r.db.Rebind(secretCols + ` WHERE organization_uuid = ? ORDER BY created_at DESC ` + pageClause)
		args = append([]interface{}{orgID}, pageArgs...)
	}
	return r.querySecrets(query, args...)
}

// ListByHandles returns secrets for the given org whose handle is in the provided list.
// If updatedAfter is set, only secrets updated after that time are returned.
// Returns nil (not an error) when handles is empty.
func (r *SecretRepo) ListByHandles(orgID string, handles []string, updatedAfter *time.Time) ([]*model.Secret, error) {
	if len(handles) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(handles))
	args := make([]interface{}, 0, len(handles)+2)
	args = append(args, orgID)
	for i, h := range handles {
		placeholders[i] = "?"
		args = append(args, h)
	}
	inClause := `handle IN (` + strings.Join(placeholders, ",") + `)`

	var query string
	if updatedAfter != nil {
		query = r.db.Rebind(secretCols + ` WHERE organization_uuid = ? AND ` + inClause + ` AND updated_at > ? ORDER BY updated_at DESC`)
		args = append(args, *updatedAfter)
	} else {
		query = r.db.Rebind(secretCols + ` WHERE organization_uuid = ? AND ` + inClause + ` ORDER BY created_at DESC`)
	}
	return r.querySecrets(query, args...)
}

func (r *SecretRepo) querySecrets(query string, args ...interface{}) ([]*model.Secret, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query secrets: %w", err)
	}
	defer rows.Close()

	var secrets []*model.Secret
	for rows.Next() {
		s, err := scanSecret(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan secret row: %w", err)
		}
		secrets = append(secrets, s)
	}
	return secrets, rows.Err()
}

func (r *SecretRepo) Count(orgID string) (int, error) {
	var count int
	query := r.db.Rebind(`SELECT COUNT(*) FROM secrets WHERE organization_uuid = ?`)
	if err := r.db.QueryRow(query, orgID).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count secrets: %w", err)
	}
	return count, nil
}

func (r *SecretRepo) Update(s *model.Secret) error {
	s.UpdatedAt = time.Now()

	query := r.db.Rebind(`
		UPDATE secrets
		SET name = ?, description = ?, ciphertext = ?, hash = ?,
		    status = ?, updated_at = ?, updated_by = ?
		WHERE organization_uuid = ? AND handle = ?
	`)

	result, err := r.db.Exec(query,
		s.DisplayName, s.Description, s.Ciphertext, s.Hash,
		s.Status, s.UpdatedAt, s.UpdatedBy,
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
// secret in a single transaction, eliminating the TOCTOU window.
// Returns the references without deprecating if any are found.
func (r *SecretRepo) FindRefsAndSoftDelete(orgID, handle, updatedBy string) ([]model.SecretReference, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var lockQuery string
	if r.db.Driver() == "postgres" || r.db.Driver() == "postgresql" {
		lockQuery = `SELECT uuid FROM secrets WHERE organization_uuid = $1 AND handle = $2 LIMIT 1 FOR UPDATE`
	} else {
		// SQL Server rejects LIMIT; FetchFirstClause yields the dialect's
		// fixed-row clause (it needs an ORDER BY, hence ORDER BY (SELECT NULL)).
		lockQuery = r.db.Rebind(`SELECT uuid FROM secrets WHERE organization_uuid = ? AND handle = ? ORDER BY (SELECT NULL) ` + r.db.FetchFirstClause(1))
	}
	var lockedID string
	if err := tx.QueryRow(lockQuery, orgID, handle).Scan(&lockedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrSecretNotFound
		}
		return nil, fmt.Errorf("failed to lock secret row: %w", err)
	}

	refsQuery := r.db.Rebind(`
		SELECT DISTINCT
			COALESCE(ra.handle, lp.handle, lpr.handle, mcp.handle, asr.artifact_uuid) AS handle,
			COALESCE(ra.name,   lp.name,   lpr.name,   mcp.name,   '')               AS name,
			art.type
		FROM artifact_secret_refs asr
		JOIN artifacts art ON art.uuid = asr.artifact_uuid
		LEFT JOIN rest_apis     ra  ON ra.uuid  = asr.artifact_uuid
		LEFT JOIN llm_providers lp  ON lp.uuid  = asr.artifact_uuid
		LEFT JOIN llm_proxies   lpr ON lpr.uuid = asr.artifact_uuid
		LEFT JOIN mcp_proxies   mcp ON mcp.uuid = asr.artifact_uuid
		WHERE asr.organization_uuid = ? AND asr.secret_handle = ?
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
		WHERE organization_uuid = ? AND handle = ?
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
		SELECT DISTINCT
			COALESCE(ra.handle, lp.handle, lpr.handle, mcp.handle, asr.artifact_uuid) AS handle,
			COALESCE(ra.name,   lp.name,   lpr.name,   mcp.name,   '')               AS name,
			art.type
		FROM artifact_secret_refs asr
		JOIN artifacts art ON art.uuid = asr.artifact_uuid
		LEFT JOIN rest_apis     ra  ON ra.uuid  = asr.artifact_uuid
		LEFT JOIN llm_providers lp  ON lp.uuid  = asr.artifact_uuid
		LEFT JOIN llm_proxies   lpr ON lpr.uuid = asr.artifact_uuid
		LEFT JOIN mcp_proxies   mcp ON mcp.uuid = asr.artifact_uuid
		WHERE asr.organization_uuid = ? AND asr.secret_handle = ?
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
		WHERE organization_uuid = ? AND handle = ? AND status = 'ACTIVE'
	`)
	if err := r.db.QueryRow(query, orgID, handle).Scan(&count); err != nil {
		return false, fmt.Errorf("failed to check secret existence: %w", err)
	}
	return count > 0, nil
}
