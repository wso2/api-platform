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
)

// ServiceAccountRepo persists service accounts and their revocation ledger.
type ServiceAccountRepo struct {
	db *database.DB
}

// NewServiceAccountRepo creates a new ServiceAccountRepo.
func NewServiceAccountRepo(db *database.DB) *ServiceAccountRepo {
	return &ServiceAccountRepo{db: db}
}

const serviceAccountCols = `SELECT uuid, organization_uuid, handle, name, version, owner, description,
	       client_id, client_secret_hash, masked_secret, identity_uuid, roles, status, token_version,
	       last_used_at, last_used_ip, secret_regenerated_at, secret_regenerated_by,
	       created_at, created_by, updated_at, updated_by FROM service_accounts`

func scanServiceAccount(row interface{ Scan(...any) error }) (*model.ServiceAccount, error) {
	sa := &model.ServiceAccount{}
	var lastUsedAt, regeneratedAt sql.NullTime
	var lastUsedIP, regeneratedBy, createdBy, updatedBy sql.NullString
	err := row.Scan(
		&sa.UUID, &sa.OrganizationID, &sa.Handle, &sa.DisplayName, &sa.Version, &sa.Owner, &sa.Description,
		&sa.ClientID, &sa.ClientSecretHash, &sa.MaskedSecret, &sa.IdentityUUID, &sa.Roles, &sa.Status, &sa.TokenVersion,
		&lastUsedAt, &lastUsedIP, &regeneratedAt, &regeneratedBy,
		&sa.CreatedAt, &createdBy, &sa.UpdatedAt, &updatedBy,
	)
	if err != nil {
		return nil, err
	}
	if lastUsedAt.Valid {
		t := lastUsedAt.Time
		sa.LastUsedAt = &t
	}
	if regeneratedAt.Valid {
		t := regeneratedAt.Time
		sa.SecretRegeneratedAt = &t
	}
	sa.LastUsedIP = lastUsedIP.String
	sa.SecretRegeneratedBy = regeneratedBy.String
	sa.CreatedBy = createdBy.String
	sa.UpdatedBy = updatedBy.String
	return sa, nil
}

func (r *ServiceAccountRepo) Create(sa *model.ServiceAccount, subject string) error {
	now := time.Now().UTC()
	sa.CreatedAt, sa.UpdatedAt = now, now
	if sa.Version == "" {
		sa.Version = "v1.0"
	}
	if sa.Status == "" {
		sa.Status = model.ServiceAccountStatusActive
	}
	if sa.TokenVersion == 0 {
		sa.TokenVersion = 1
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.Exec(r.db.Rebind(`
		INSERT INTO service_accounts (
			uuid, organization_uuid, handle, name, version, owner, description,
			client_id, client_secret_hash, masked_secret, identity_uuid, roles, status, token_version,
			created_at, created_by, updated_at, updated_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		sa.UUID, sa.OrganizationID, sa.Handle, sa.DisplayName, sa.Version, sa.Owner, sa.Description,
		sa.ClientID, sa.ClientSecretHash, sa.MaskedSecret, sa.IdentityUUID, sa.Roles, sa.Status, sa.TokenVersion,
		sa.CreatedAt, sa.CreatedBy, sa.UpdatedAt, sa.UpdatedBy,
	)
	if err != nil {
		if r.db.IsDuplicateKeyError(err) {
			return apperror.ServiceAccountExists.Wrap(err)
		}
		return fmt.Errorf("failed to create service account: %w", err)
	}

	// The identity row's uuid is the account's own, which is how the startup
	// check tells a minted sa: identity from a foreign one.
	_, err = tx.Exec(r.db.Rebind(`INSERT INTO user_idp_references (uuid, idp_id, created_at) VALUES (?, ?, ?)`),
		sa.IdentityUUID, subject, now)
	if err != nil {
		return fmt.Errorf("failed to create service account identity: %w", err)
	}
	return tx.Commit()
}

func (r *ServiceAccountRepo) GetByHandle(orgID, handle string) (*model.ServiceAccount, error) {
	sa, err := scanServiceAccount(r.db.QueryRow(r.db.Rebind(serviceAccountCols+
		` WHERE organization_uuid = ? AND handle = ?`), orgID, handle))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.ServiceAccountNotFound.Wrap(err)
		}
		return nil, fmt.Errorf("failed to get service account: %w", err)
	}
	return sa, nil
}

func (r *ServiceAccountRepo) GetByClientID(clientID string) (*model.ServiceAccount, error) {
	sa, err := scanServiceAccount(r.db.QueryRow(r.db.Rebind(serviceAccountCols+
		` WHERE client_id = ?`), clientID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.ServiceAccountNotFound.Wrap(err)
		}
		return nil, fmt.Errorf("failed to get service account: %w", err)
	}
	return sa, nil
}

func (r *ServiceAccountRepo) List(orgID string, limit, offset int) ([]*model.ServiceAccount, error) {
	pageClause, pageArgs := r.db.PaginationClause(limit, offset)
	rows, err := r.db.Query(r.db.Rebind(serviceAccountCols+
		` WHERE organization_uuid = ? ORDER BY created_at DESC `+pageClause),
		append([]any{orgID}, pageArgs...)...)
	if err != nil {
		return nil, fmt.Errorf("failed to list service accounts: %w", err)
	}
	defer rows.Close()

	var out []*model.ServiceAccount
	for rows.Next() {
		sa, err := scanServiceAccount(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan service account: %w", err)
		}
		out = append(out, sa)
	}
	return out, rows.Err()
}

func (r *ServiceAccountRepo) Count(orgID string) (int, error) {
	var n int
	if err := r.db.QueryRow(r.db.Rebind(`SELECT COUNT(*) FROM service_accounts WHERE organization_uuid = ?`), orgID).Scan(&n); err != nil {
		return 0, fmt.Errorf("failed to count service accounts: %w", err)
	}
	return n, nil
}

// Update writes sa only if the row still has the token version and status the
// caller read, so a concurrent disable or revoke is never overwritten.
func (r *ServiceAccountRepo) Update(sa *model.ServiceAccount, prevVersion int64, prevStatus string, rev *model.ServiceAccountRevocation) error {
	sa.UpdatedAt = time.Now().UTC()
	return r.inTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(r.db.Rebind(`
			UPDATE service_accounts
			SET name = ?, owner = ?, description = ?, roles = ?, status = ?, token_version = ?, updated_at = ?, updated_by = ?
			WHERE organization_uuid = ? AND uuid = ? AND token_version = ? AND status = ?`),
			sa.DisplayName, sa.Owner, sa.Description, sa.Roles, sa.Status, sa.TokenVersion, sa.UpdatedAt, sa.UpdatedBy,
			sa.OrganizationID, sa.UUID, prevVersion, prevStatus)
		if err := r.requireOneRow(tx, res, err, sa.OrganizationID, sa.UUID); err != nil {
			return err
		}
		return r.revokeTx(tx, rev)
	})
}

// UpdateSecret replaces the secret only if the row still has prevVersion.
func (r *ServiceAccountRepo) UpdateSecret(sa *model.ServiceAccount, prevVersion int64, rev *model.ServiceAccountRevocation) error {
	now := time.Now().UTC()
	sa.UpdatedAt = now
	sa.SecretRegeneratedAt = &now
	return r.inTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(r.db.Rebind(`
			UPDATE service_accounts
			SET client_secret_hash = ?, masked_secret = ?, secret_regenerated_at = ?, secret_regenerated_by = ?,
			    token_version = ?, updated_at = ?, updated_by = ?
			WHERE organization_uuid = ? AND uuid = ? AND token_version = ?`),
			sa.ClientSecretHash, sa.MaskedSecret, now, sa.SecretRegeneratedBy, sa.TokenVersion, now, sa.UpdatedBy,
			sa.OrganizationID, sa.UUID, prevVersion)
		if err := r.requireOneRow(tx, res, err, sa.OrganizationID, sa.UUID); err != nil {
			return err
		}
		return r.revokeTx(tx, rev)
	})
}

// Delete removes the account only if it still has prevVersion: rev was built
// from that version, and a token minted after a concurrent bump would outlive it.
func (r *ServiceAccountRepo) Delete(orgID, uuid string, prevVersion int64, rev *model.ServiceAccountRevocation) error {
	return r.inTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(r.db.Rebind(`DELETE FROM service_accounts WHERE organization_uuid = ? AND uuid = ? AND token_version = ?`),
			orgID, uuid, prevVersion)
		if err := r.requireOneRow(tx, res, err, orgID, uuid); err != nil {
			return err
		}
		return r.revokeTx(tx, rev)
	})
}

func (r *ServiceAccountRepo) TouchLastUsed(uuid string, at time.Time, ip string) error {
	_, err := r.db.Exec(r.db.Rebind(`UPDATE service_accounts SET last_used_at = ?, last_used_ip = ? WHERE uuid = ?`),
		at.UTC(), ip, uuid)
	if err != nil {
		return fmt.Errorf("failed to record service account use: %w", err)
	}
	return nil
}

func (r *ServiceAccountRepo) ForeignReservedIdentities() ([]string, error) {
	rows, err := r.db.Query(r.db.Rebind(`SELECT uuid, idp_id FROM user_idp_references WHERE idp_id LIKE ?`),
		model.ServiceAccountSubPrefix+"%")
	if err != nil {
		return nil, fmt.Errorf("failed to scan reserved identities: %w", err)
	}
	defer rows.Close()

	var foreign []string
	for rows.Next() {
		var id, idpID string
		if err := rows.Scan(&id, &idpID); err != nil {
			return nil, err
		}
		if accountUUID, ok := model.AccountUUIDFromSubject(idpID); !ok || accountUUID != id {
			foreign = append(foreign, idpID)
		}
	}
	return foreign, rows.Err()
}

// Revoke upserts one watermark outside any other write.
func (r *ServiceAccountRepo) Revoke(rev *model.ServiceAccountRevocation) error {
	return r.inTx(func(tx *sql.Tx) error { return r.revokeTx(tx, rev) })
}

// revokeTx is a portable, monotonic upsert: move the watermark only forwards,
// insert if there is none, and retry the update if a concurrent insert won.
func (r *ServiceAccountRepo) revokeTx(tx *sql.Tx, rev *model.ServiceAccountRevocation) error {
	if rev == nil {
		return nil
	}
	expiresAt, revokedAt := rev.ExpiresAt.UTC(), time.Now().UTC()
	update := func() (bool, error) {
		res, err := tx.Exec(r.db.Rebind(`
			UPDATE service_account_revocations
			SET min_token_version = ?, expires_at = ?, revoked_by = ?, revoked_at = ?
			WHERE account_uuid = ? AND min_token_version < ?`),
			rev.MinTokenVersion, expiresAt, rev.RevokedBy, revokedAt, rev.AccountUUID, rev.MinTokenVersion)
		if err != nil {
			return false, fmt.Errorf("failed to update revocation: %w", err)
		}
		n, err := res.RowsAffected()
		return n > 0, err
	}
	if done, err := update(); err != nil || done {
		return err
	}

	var exists int
	err := tx.QueryRow(r.db.Rebind(`SELECT COUNT(*) FROM service_account_revocations WHERE account_uuid = ?`),
		rev.AccountUUID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to read revocation: %w", err)
	}
	if exists > 0 {
		return nil // an equal or later watermark is already there
	}
	_, err = tx.Exec(r.db.Rebind(`
		INSERT INTO service_account_revocations
			(account_uuid, organization_uuid, min_token_version, expires_at, revoked_by, revoked_at)
		VALUES (?, ?, ?, ?, ?, ?)`),
		rev.AccountUUID, rev.OrganizationID, rev.MinTokenVersion, expiresAt, rev.RevokedBy, revokedAt)
	if err != nil && r.db.IsDuplicateKeyError(err) {
		_, err = update()
	}
	if err != nil {
		return fmt.Errorf("failed to insert revocation: %w", err)
	}
	return nil
}

func (r *ServiceAccountRepo) ListActive(now time.Time) ([]*model.ServiceAccountRevocation, error) {
	rows, err := r.db.Query(r.db.Rebind(`
		SELECT account_uuid, organization_uuid, min_token_version, expires_at
		FROM service_account_revocations WHERE expires_at > ?`), now.UTC())
	if err != nil {
		return nil, fmt.Errorf("failed to list revocations: %w", err)
	}
	defer rows.Close()

	var out []*model.ServiceAccountRevocation
	for rows.Next() {
		rev := &model.ServiceAccountRevocation{}
		if err := rows.Scan(&rev.AccountUUID, &rev.OrganizationID, &rev.MinTokenVersion, &rev.ExpiresAt); err != nil {
			return nil, fmt.Errorf("failed to scan revocation: %w", err)
		}
		out = append(out, rev)
	}
	return out, rows.Err()
}

func (r *ServiceAccountRepo) PruneExpired(now time.Time) (int64, error) {
	res, err := r.db.Exec(r.db.Rebind(`DELETE FROM service_account_revocations WHERE expires_at <= ?`), now.UTC())
	if err != nil {
		return 0, fmt.Errorf("failed to prune revocations: %w", err)
	}
	return res.RowsAffected()
}

func (r *ServiceAccountRepo) inTx(fn func(tx *sql.Tx) error) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// requireOneRow turns a conditional write that matched nothing into not-found
// when the row is gone, and a conflict when it changed since the caller read it.
func (r *ServiceAccountRepo) requireOneRow(tx *sql.Tx, res sql.Result, err error, orgID, uuid string) error {
	if err != nil {
		return fmt.Errorf("failed to write service account: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	var exists int
	if err := tx.QueryRow(r.db.Rebind(`SELECT COUNT(*) FROM service_accounts WHERE organization_uuid = ? AND uuid = ?`),
		orgID, uuid).Scan(&exists); err != nil {
		return fmt.Errorf("failed to read service account: %w", err)
	}
	if exists == 0 {
		return apperror.ServiceAccountNotFound.New()
	}
	return apperror.Conflict.New().WithLogMessage("service account changed concurrently")
}
