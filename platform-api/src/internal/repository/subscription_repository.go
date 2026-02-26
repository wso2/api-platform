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
	"database/sql"
	"fmt"
	"time"

	"platform-api/src/internal/database"
	"platform-api/src/internal/model"

	"github.com/google/uuid"
)

// SubscriptionRepo implements SubscriptionRepository
type SubscriptionRepo struct {
	db *database.DB
}

// NewSubscriptionRepo creates a new subscription repository
func NewSubscriptionRepo(db *database.DB) SubscriptionRepository {
	return &SubscriptionRepo{db: db}
}

// Create inserts a new subscription
func (r *SubscriptionRepo) Create(sub *model.Subscription) error {
	if sub.UUID == "" {
		sub.UUID = uuid.New().String()
	}
	now := time.Now()
	sub.CreatedAt = now
	sub.UpdatedAt = now

	query := `
		INSERT INTO subscriptions (uuid, api_uuid, application_id, organization_uuid, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(r.db.Rebind(query), sub.UUID, sub.APIUUID, sub.ApplicationID, sub.OrganizationUUID, string(sub.Status), sub.CreatedAt, sub.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert subscription: %w", err)
	}
	return nil
}

// GetByID retrieves a subscription by ID and organization
func (r *SubscriptionRepo) GetByID(subscriptionID, orgUUID string) (*model.Subscription, error) {
	query := `
		SELECT uuid, api_uuid, application_id, organization_uuid, status, created_at, updated_at
		FROM subscriptions
		WHERE uuid = ? AND organization_uuid = ?
	`
	sub := &model.Subscription{}
	err := r.db.QueryRow(r.db.Rebind(query), subscriptionID, orgUUID).Scan(
		&sub.UUID,
		&sub.APIUUID,
		&sub.ApplicationID,
		&sub.OrganizationUUID,
		&sub.Status,
		&sub.CreatedAt,
		&sub.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return sub, nil
}

// ListByFilters returns subscriptions filtered by API and/or application for an organization.
// If apiUUID is nil, all APIs are considered. If applicationID is nil, all applications are considered.
func (r *SubscriptionRepo) ListByFilters(orgUUID string, apiUUID *string, applicationID *string, status *string) ([]*model.Subscription, error) {
	query := `
		SELECT uuid, api_uuid, application_id, organization_uuid, status, created_at, updated_at
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
	query += ` ORDER BY created_at DESC`

	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list subscriptions: %w", err)
	}
	defer rows.Close()

	var list []*model.Subscription
	for rows.Next() {
		sub := &model.Subscription{}
		if err := rows.Scan(&sub.UUID, &sub.APIUUID, &sub.ApplicationID, &sub.OrganizationUUID, &sub.Status, &sub.CreatedAt, &sub.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, sub)
	}
	return list, rows.Err()
}

// Update updates an existing subscription
func (r *SubscriptionRepo) Update(sub *model.Subscription) error {
	sub.UpdatedAt = time.Now()
	query := `
		UPDATE subscriptions
		SET status = ?, updated_at = ?
		WHERE uuid = ? AND organization_uuid = ?
	`
	result, err := r.db.Exec(r.db.Rebind(query), string(sub.Status), sub.UpdatedAt, sub.UUID, sub.OrganizationUUID)
	if err != nil {
		return fmt.Errorf("failed to update subscription: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("subscription not found: %s", sub.UUID)
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
		return fmt.Errorf("subscription not found: %s", subscriptionID)
	}
	return nil
}

// ExistsByAPIAndApplication returns true if a subscription exists for the given API and application
func (r *SubscriptionRepo) ExistsByAPIAndApplication(apiUUID, applicationID, orgUUID string) (bool, error) {
	query := `
		SELECT 1 FROM subscriptions
		WHERE api_uuid = ? AND application_id = ? AND organization_uuid = ?
		LIMIT 1
	`
	var exists int
	err := r.db.QueryRow(r.db.Rebind(query), apiUUID, applicationID, orgUUID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return exists == 1, nil
}
