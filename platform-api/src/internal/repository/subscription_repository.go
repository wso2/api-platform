/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"platform-api/src/config"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
	"platform-api/src/internal/utils"

	"github.com/google/uuid"
)

// HashSubscriptionToken computes a SHA-256 hash of the token for secure storage.
// The same token always produces the same hash for deterministic lookups.
// Exported for use when propagating to gateways (events, internal API).
func HashSubscriptionToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func hashSubscriptionToken(token string) string {
	return HashSubscriptionToken(token)
}

// getSubscriptionTokenEncryptionKey returns the 32-byte key for token encryption, or nil if not configured.
func getSubscriptionTokenEncryptionKey() ([]byte, error) {
	cfg := config.GetConfig()
	keyStr := cfg.Database.SubscriptionTokenEncryptionKey
	if keyStr == "" {
		keyStr = cfg.JWT.SecretKey
	}
	if keyStr == "" {
		return nil, fmt.Errorf("subscription token encryption requires DATABASE_SUBSCRIPTION_TOKEN_ENCRYPTION_KEY or JWT_SECRET_KEY")
	}
	return utils.DeriveEncryptionKey(keyStr)
}

// SubscriptionRepo implements SubscriptionRepository
type SubscriptionRepo struct {
	db *database.DB
}

// NewSubscriptionRepo creates a new subscription repository
func NewSubscriptionRepo(db *database.DB) SubscriptionRepository {
	return &SubscriptionRepo{db: db}
}

// generateSubscriptionToken creates a cryptographically random opaque token
func generateSubscriptionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate subscription token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// Create inserts a new subscription.
// The token is encrypted for storage and hashed for uniqueness; sub.SubscriptionToken retains the raw value for the create response.
func (r *SubscriptionRepo) Create(sub *model.Subscription) error {
	if sub.UUID == "" {
		sub.UUID = uuid.New().String()
	}
	if sub.SubscriptionToken == "" {
		token, err := generateSubscriptionToken()
		if err != nil {
			return err
		}
		sub.SubscriptionToken = token
	}
	hashedToken := hashSubscriptionToken(sub.SubscriptionToken)

	key, err := getSubscriptionTokenEncryptionKey()
	if err != nil {
		return fmt.Errorf("subscription token encryption: %w", err)
	}
	encryptedToken, err := utils.EncryptSubscriptionToken(key, sub.SubscriptionToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt subscription token: %w", err)
	}

	now := time.Now()
	sub.CreatedAt = now
	sub.UpdatedAt = now

	query := `
		INSERT INTO subscriptions (uuid, api_uuid, application_id, subscription_token, subscription_token_hash, subscription_plan_uuid,
			organization_uuid, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = r.db.Exec(r.db.Rebind(query),
		sub.UUID, sub.APIUUID, sub.ApplicationID, encryptedToken, hashedToken,
		sub.SubscriptionPlanID, sub.OrganizationUUID, string(sub.Status),
		sub.CreatedAt, sub.UpdatedAt,
	)
	if err != nil {
		if isSubscriptionUniqueViolation(err) {
			return constants.ErrSubscriptionAlreadyExists
		}
		return fmt.Errorf("failed to insert subscription: %w", err)
	}
	return nil
}

// isSubscriptionUniqueViolation detects DB unique-constraint violations on (api_uuid, application_id, organization_uuid).
func isSubscriptionUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	// SQLite: UNIQUE constraint failed: subscriptions.api_uuid, subscriptions.application_id, subscriptions.organization_uuid
	if strings.Contains(s, "UNIQUE constraint failed") && strings.Contains(s, "subscriptions") {
		return true
	}
	// PostgreSQL: duplicate key value violates unique constraint (code 23505)
	if strings.Contains(s, "duplicate key value violates unique constraint") ||
		strings.Contains(s, "23505") {
		return true
	}
	// MySQL: Error 1062 (23000): Duplicate entry
	if strings.Contains(s, "Duplicate entry") || strings.Contains(s, "1062") {
		return true
	}
	return false
}

// GetByID retrieves a subscription by ID and organization.
// Decrypts subscription_token for API response.
func (r *SubscriptionRepo) GetByID(subscriptionID, orgUUID string) (*model.Subscription, error) {
	query := `
		SELECT uuid, api_uuid, application_id, subscription_token, subscription_plan_uuid,
			organization_uuid, status, created_at, updated_at
		FROM subscriptions
		WHERE uuid = ? AND organization_uuid = ?
	`
	sub := &model.Subscription{}
	var storedToken string
	err := r.db.QueryRow(r.db.Rebind(query), subscriptionID, orgUUID).Scan(
		&sub.UUID, &sub.APIUUID, &sub.ApplicationID, &storedToken,
		&sub.SubscriptionPlanID, &sub.OrganizationUUID, &sub.Status,
		&sub.CreatedAt, &sub.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrSubscriptionNotFound
		}
		return nil, err
	}
	sub.SubscriptionToken = r.decryptSubscriptionToken(storedToken)
	return sub, nil
}

// ListByFilters returns subscriptions filtered by API and/or application for an organization.
// Decrypts subscription_token for each row.
func (r *SubscriptionRepo) ListByFilters(orgUUID string, apiUUID *string, applicationID *string, status *string, limit, offset int) ([]*model.Subscription, error) {
	query := `
		SELECT uuid, api_uuid, application_id, subscription_token, subscription_plan_uuid,
			organization_uuid, status, created_at, updated_at
		FROM subscriptions
		WHERE organization_uuid = ?
	`
	args := []interface{}{orgUUID}
	if apiUUID != nil && *apiUUID != "" {
		query += ` AND api_uuid = ?`
		args = append(args, *apiUUID)
	}
	if applicationID != nil && *applicationID != "" {
		query += ` AND application_id = ?`
		args = append(args, *applicationID)
	}
	if status != nil && *status != "" {
		query += ` AND status = ?`
		args = append(args, *status)
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list subscriptions: %w", err)
	}
	defer rows.Close()

	var list []*model.Subscription
	for rows.Next() {
		sub := &model.Subscription{}
		var storedToken string
		if err := rows.Scan(
			&sub.UUID, &sub.APIUUID, &sub.ApplicationID, &storedToken,
			&sub.SubscriptionPlanID, &sub.OrganizationUUID, &sub.Status,
			&sub.CreatedAt, &sub.UpdatedAt,
		); err != nil {
			return nil, err
		}
		sub.SubscriptionToken = r.decryptSubscriptionToken(storedToken)
		list = append(list, sub)
	}
	return list, rows.Err()
}

// decryptSubscriptionToken decrypts stored token for API response.
func (r *SubscriptionRepo) decryptSubscriptionToken(stored string) string {
	if stored == "" {
		return ""
	}
	key, err := getSubscriptionTokenEncryptionKey()
	if err != nil {
		return ""
	}
	plain, err := utils.DecryptSubscriptionToken(key, stored)
	if err != nil {
		return ""
	}
	return plain
}

// CountByFilters returns the total count of subscriptions matching the same filters as ListByFilters.
func (r *SubscriptionRepo) CountByFilters(orgUUID string, apiUUID *string, applicationID *string, status *string) (int, error) {
	query := `SELECT COUNT(*) FROM subscriptions WHERE organization_uuid = ?`
	args := []interface{}{orgUUID}
	if apiUUID != nil && *apiUUID != "" {
		query += ` AND api_uuid = ?`
		args = append(args, *apiUUID)
	}
	if applicationID != nil && *applicationID != "" {
		query += ` AND application_id = ?`
		args = append(args, *applicationID)
	}
	if status != nil && *status != "" {
		query += ` AND status = ?`
		args = append(args, *status)
	}
	var count int
	err := r.db.QueryRow(r.db.Rebind(query), args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count subscriptions: %w", err)
	}
	return count, nil
}

// Update updates an existing subscription with all mutable fields.
func (r *SubscriptionRepo) Update(sub *model.Subscription) error {
	sub.UpdatedAt = time.Now()
	query := `
		UPDATE subscriptions
		SET subscription_plan_uuid = ?, application_id = ?, status = ?, updated_at = ?
		WHERE uuid = ? AND organization_uuid = ?
	`
	result, err := r.db.Exec(r.db.Rebind(query),
		sub.SubscriptionPlanID, sub.ApplicationID, string(sub.Status), sub.UpdatedAt,
		sub.UUID, sub.OrganizationUUID,
	)
	if err != nil {
		return fmt.Errorf("failed to update subscription: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return constants.ErrSubscriptionNotFound
	}
	return nil
}

// Delete removes a subscription by ID and organization
func (r *SubscriptionRepo) Delete(subscriptionID, orgUUID string) error {
	query := `DELETE FROM subscriptions WHERE uuid = ? AND organization_uuid = ?`
	result, err := r.db.Exec(r.db.Rebind(query), subscriptionID, orgUUID)
	if err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return constants.ErrSubscriptionNotFound
	}
	return nil
}

// ExistsByAPIAndApplication returns true if a subscription exists for the given API and application.
// applicationID empty string matches application_id IS NULL (token-based subscriptions).
func (r *SubscriptionRepo) ExistsByAPIAndApplication(apiUUID, applicationID, orgUUID string) (bool, error) {
	query := `
		SELECT 1 FROM subscriptions
		WHERE api_uuid = ? AND organization_uuid = ?
		  AND COALESCE(application_id, '') = COALESCE(?, '')
		LIMIT 1
	`
	var exists int
	err := r.db.QueryRow(r.db.Rebind(query), apiUUID, orgUUID, applicationID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return exists == 1, nil
}
