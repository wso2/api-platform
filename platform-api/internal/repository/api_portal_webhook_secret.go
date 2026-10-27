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

	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// APIPortalWebhookSecretRepo implements APIPortalWebhookSecretRepository.
type APIPortalWebhookSecretRepo struct {
	db *database.DB
}

// NewAPIPortalWebhookSecretRepo creates a new per-portal webhook secret repository.
func NewAPIPortalWebhookSecretRepo(db *database.DB) APIPortalWebhookSecretRepository {
	return &APIPortalWebhookSecretRepo{db: db}
}

const apiPortalWebhookSecretSelectColumns = `
	portal_uuid, secret_enc, previous_secret_enc, previous_valid_until, created_at, rotated_at
`

func scanAPIPortalWebhookSecretRow(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.APIPortalWebhookSecret, error) {
	s := &model.APIPortalWebhookSecret{}
	if err := scanner.Scan(
		&s.PortalUUID, &s.SecretEnc, &s.PreviousSecretEnc, &s.PreviousValidUntil, &s.CreatedAt, &s.RotatedAt,
	); err != nil {
		return nil, err
	}
	return s, nil
}

// GetByPortalUUID returns the secret row for portalUUID, or (nil, nil) when absent.
func (r *APIPortalWebhookSecretRepo) GetByPortalUUID(portalUUID string) (*model.APIPortalWebhookSecret, error) {
	query := fmt.Sprintf(`
		SELECT %s FROM api_portal_webhook_secrets
		WHERE portal_uuid = ?
	`, apiPortalWebhookSecretSelectColumns)
	row := r.db.QueryRow(r.db.Rebind(query), portalUUID)
	s, err := scanAPIPortalWebhookSecretRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return s, nil
}

// Insert creates a first-time secret row. created_at is set to now (UTC).
func (r *APIPortalWebhookSecretRepo) Insert(portalUUID string, secretEnc []byte) error {
	query := `
		INSERT INTO api_portal_webhook_secrets (portal_uuid, secret_enc, created_at)
		VALUES (?, ?, ?)
	`
	_, err := r.db.Exec(r.db.Rebind(query), portalUUID, secretEnc, time.Now().UTC())
	return err
}

// Rotate writes newSecretEnc as the current secret, moves the existing secret_enc
// to previous_secret_enc, sets previous_valid_until, and stamps rotated_at. All in
// one statement so a concurrent read sees either the pre- or post-rotation state,
// never a half-rotation.
func (r *APIPortalWebhookSecretRepo) Rotate(portalUUID string, newSecretEnc []byte, previousValidUntil time.Time) error {
	now := time.Now().UTC()
	query := `
		UPDATE api_portal_webhook_secrets
		SET previous_secret_enc  = secret_enc,
		    previous_valid_until = ?,
		    secret_enc           = ?,
		    rotated_at           = ?
		WHERE portal_uuid = ?
	`
	res, err := r.db.Exec(r.db.Rebind(query), previousValidUntil, newSecretEnc, now, portalUUID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("api_portal_webhook_secrets: no row to rotate for portal %q", portalUUID)
	}
	return nil
}

// ClearExpiredPrevious nulls previous_secret_enc + previous_valid_until when
// previous_valid_until has passed. Lazy cleanup; safe to call unconditionally —
// the WHERE guards both "no previous slot" and "still inside the grace window".
func (r *APIPortalWebhookSecretRepo) ClearExpiredPrevious(portalUUID string, now time.Time) error {
	query := `
		UPDATE api_portal_webhook_secrets
		SET previous_secret_enc  = NULL,
		    previous_valid_until = NULL
		WHERE portal_uuid = ? AND previous_valid_until IS NOT NULL AND previous_valid_until <= ?
	`
	_, err := r.db.Exec(r.db.Rebind(query), portalUUID, now)
	return err
}
