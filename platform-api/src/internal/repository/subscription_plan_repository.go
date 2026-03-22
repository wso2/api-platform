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
	"fmt"
	"strings"
	"time"

	"platform-api/src/internal/database"
	"platform-api/src/internal/model"

	"github.com/google/uuid"
)

// SubscriptionPlanRepo implements SubscriptionPlanRepository
type SubscriptionPlanRepo struct {
	db *database.DB
}

// NewSubscriptionPlanRepo creates a new subscription plan repository
func NewSubscriptionPlanRepo(db *database.DB) SubscriptionPlanRepository {
	return &SubscriptionPlanRepo{db: db}
}

// Create inserts a new subscription plan
func (r *SubscriptionPlanRepo) Create(plan *model.SubscriptionPlan) error {
	if plan == nil {
		return fmt.Errorf("subscription plan is required")
	}
	if plan.UUID == "" {
		plan.UUID = uuid.New().String()
	}
	now := time.Now()
	plan.CreatedAt = now
	plan.UpdatedAt = now

	query := `
		INSERT INTO subscription_plans (uuid, plan_name, billing_plan, stop_on_quota_reach, throttle_limit_count,
			throttle_limit_unit, expiry_time, organization_uuid, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(r.db.Rebind(query),
		plan.UUID, plan.PlanName, plan.BillingPlan, plan.StopOnQuotaReach,
		plan.ThrottleLimitCount, plan.ThrottleLimitUnit, plan.ExpiryTime,
		plan.OrganizationUUID, string(plan.Status), plan.CreatedAt, plan.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert subscription plan: %w", err)
	}
	return nil
}

// GetByNameAndOrg retrieves a subscription plan by name and organization
func (r *SubscriptionPlanRepo) GetByNameAndOrg(planName, orgUUID string) (*model.SubscriptionPlan, error) {
	query := `
		SELECT uuid, plan_name, billing_plan, stop_on_quota_reach, throttle_limit_count,
			throttle_limit_unit, expiry_time, organization_uuid, status, created_at, updated_at
		FROM subscription_plans
		WHERE plan_name = ? AND organization_uuid = ?
	`
	plan := &model.SubscriptionPlan{}
	err := r.db.QueryRow(r.db.Rebind(query), planName, orgUUID).Scan(
		&plan.UUID, &plan.PlanName, &plan.BillingPlan, &plan.StopOnQuotaReach,
		&plan.ThrottleLimitCount, &plan.ThrottleLimitUnit, &plan.ExpiryTime,
		&plan.OrganizationUUID, &plan.Status, &plan.CreatedAt, &plan.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return plan, nil
}

// GetByID retrieves a subscription plan by ID and organization
func (r *SubscriptionPlanRepo) GetByID(planID, orgUUID string) (*model.SubscriptionPlan, error) {
	query := `
		SELECT uuid, plan_name, billing_plan, stop_on_quota_reach, throttle_limit_count,
			throttle_limit_unit, expiry_time, organization_uuid, status, created_at, updated_at
		FROM subscription_plans
		WHERE uuid = ? AND organization_uuid = ?
	`
	plan := &model.SubscriptionPlan{}
	err := r.db.QueryRow(r.db.Rebind(query), planID, orgUUID).Scan(
		&plan.UUID, &plan.PlanName, &plan.BillingPlan, &plan.StopOnQuotaReach,
		&plan.ThrottleLimitCount, &plan.ThrottleLimitUnit, &plan.ExpiryTime,
		&plan.OrganizationUUID, &plan.Status, &plan.CreatedAt, &plan.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return plan, nil
}

// GetByIDs returns a map of plan UUID to plan name for the given IDs in the organization.
// Used for bulk lookup to avoid N+1 queries. Returns empty map for empty input.
func (r *SubscriptionPlanRepo) GetByIDs(planIDs []string, orgUUID string) (map[string]string, error) {
	if len(planIDs) == 0 {
		return map[string]string{}, nil
	}
	placeholders := make([]string, len(planIDs))
	args := make([]interface{}, 0, len(planIDs)+1)
	for i, id := range planIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	args = append(args, orgUUID)
	query := fmt.Sprintf(`
		SELECT uuid, plan_name
		FROM subscription_plans
		WHERE uuid IN (%s) AND organization_uuid = ?
	`, strings.Join(placeholders, ","))
	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get plans by IDs: %w", err)
	}
	defer rows.Close()
	m := make(map[string]string)
	for rows.Next() {
		var id, planName string
		if err := rows.Scan(&id, &planName); err != nil {
			return nil, err
		}
		m[id] = planName
	}
	return m, rows.Err()
}

// ListByOrganization returns subscription plans for an organization with pagination
func (r *SubscriptionPlanRepo) ListByOrganization(orgUUID string, limit, offset int) ([]*model.SubscriptionPlan, error) {
	query := `
		SELECT uuid, plan_name, billing_plan, stop_on_quota_reach, throttle_limit_count,
			throttle_limit_unit, expiry_time, organization_uuid, status, created_at, updated_at
		FROM subscription_plans
		WHERE organization_uuid = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(r.db.Rebind(query), orgUUID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list subscription plans: %w", err)
	}
	defer rows.Close()

	var list []*model.SubscriptionPlan
	for rows.Next() {
		plan := &model.SubscriptionPlan{}
		if err := rows.Scan(
			&plan.UUID, &plan.PlanName, &plan.BillingPlan, &plan.StopOnQuotaReach,
			&plan.ThrottleLimitCount, &plan.ThrottleLimitUnit, &plan.ExpiryTime,
			&plan.OrganizationUUID, &plan.Status, &plan.CreatedAt, &plan.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, plan)
	}
	return list, rows.Err()
}

// Update updates an existing subscription plan
func (r *SubscriptionPlanRepo) Update(plan *model.SubscriptionPlan) error {
	if plan == nil {
		return fmt.Errorf("subscription plan is required")
	}
	plan.UpdatedAt = time.Now()
	query := `
		UPDATE subscription_plans
		SET plan_name = ?, billing_plan = ?, stop_on_quota_reach = ?, throttle_limit_count = ?,
			throttle_limit_unit = ?, expiry_time = ?, status = ?, updated_at = ?
		WHERE uuid = ? AND organization_uuid = ?
	`
	result, err := r.db.Exec(r.db.Rebind(query),
		plan.PlanName, plan.BillingPlan, plan.StopOnQuotaReach,
		plan.ThrottleLimitCount, plan.ThrottleLimitUnit, plan.ExpiryTime,
		string(plan.Status), plan.UpdatedAt,
		plan.UUID, plan.OrganizationUUID,
	)
	if err != nil {
		return fmt.Errorf("failed to update subscription plan: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("subscription plan not found: %s", plan.UUID)
	}
	return nil
}

// Delete removes a subscription plan by ID and organization
func (r *SubscriptionPlanRepo) Delete(planID, orgUUID string) error {
	query := `DELETE FROM subscription_plans WHERE uuid = ? AND organization_uuid = ?`
	result, err := r.db.Exec(r.db.Rebind(query), planID, orgUUID)
	if err != nil {
		return fmt.Errorf("failed to delete subscription plan: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("subscription plan not found: %s", planID)
	}
	return nil
}

// ExistsByNameAndOrg returns true if a plan with the given name exists in the organization
func (r *SubscriptionPlanRepo) ExistsByNameAndOrg(planName, orgUUID string) (bool, error) {
	query := `
		SELECT 1 FROM subscription_plans
		WHERE plan_name = ? AND organization_uuid = ?
		LIMIT 1
	`
	var exists int
	err := r.db.QueryRow(r.db.Rebind(query), planName, orgUUID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return exists == 1, nil
}
