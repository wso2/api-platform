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

	"platform-api/src/internal/constants"
	"platform-api/src/internal/database"
	"platform-api/src/internal/model"

	"github.com/google/uuid"
)

// planSelectColumns is the shared column list for reading a subscription plan
// joined with its single throttling limit row.
//
// NOTE: SINGLE-LIMIT ASSUMPTION. Throttling limits live in subscription_plan_limits
// (one row per limit) and that table supports multiple limits per plan, but the
// platform-api currently reads only one REQUEST_COUNT limit and maps it onto the
// plan's StopOnQuotaReach / ThrottleLimitCount / ThrottleLimitUnit fields. The
// LEFT JOIN below is constrained to REQUEST_COUNT for that reason. This must be
// improved to surface all limit rows.
const planSelectColumns = `
		p.uuid, p.handle, p.name, p.billing_plan, p.expiry_time,
		p.organization_uuid, p.status, p.created_at, p.updated_at,
		spl.limit_count, spl.time_unit, spl.stop_on_quota_reach
	FROM subscription_plans p
	LEFT JOIN subscription_plan_limits spl
		ON spl.subscription_plan_uuid = p.uuid AND spl.limit_type = '` + constants.LimitTypeRequestCount + `'`

// SubscriptionPlanRepo implements SubscriptionPlanRepository
type SubscriptionPlanRepo struct {
	db *database.DB
}

// NewSubscriptionPlanRepo creates a new subscription plan repository
func NewSubscriptionPlanRepo(db *database.DB) SubscriptionPlanRepository {
	return &SubscriptionPlanRepo{db: db}
}

// Create inserts a new subscription plan together with its single throttling limit row.
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

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(r.db.Rebind(`
		INSERT INTO subscription_plans (uuid, handle, name, billing_plan, expiry_time,
			organization_uuid, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`),
		plan.UUID, plan.Handle, plan.Name, plan.BillingPlan, plan.ExpiryTime,
		plan.OrganizationUUID, string(plan.Status), plan.CreatedAt, plan.UpdatedAt,
	); err != nil {
		return fmt.Errorf("failed to insert subscription plan: %w", err)
	}

	if err := r.replaceSingleLimitTx(tx, plan); err != nil {
		return err
	}

	return tx.Commit()
}

// replaceSingleLimitTx clears the plan's single REQUEST_COUNT limit row and re-inserts
// it from the plan's ThrottleLimitCount / ThrottleLimitUnit / StopOnQuotaReach fields.
//
// A row is ALWAYS written (even when ThrottleLimitCount is nil, stored as a NULL
// limit_count) so that StopOnQuotaReach round-trips faithfully for plans with no quota.
//
// NOTE: SINGLE-LIMIT ASSUMPTION. subscription_plan_limits supports multiple limits per
// plan, but only one REQUEST_COUNT limit is persisted here. This must be improved to
// write all limits defined for the plan.
func (r *SubscriptionPlanRepo) replaceSingleLimitTx(tx *sql.Tx, plan *model.SubscriptionPlan) error {
	if _, err := tx.Exec(r.db.Rebind(`
		DELETE FROM subscription_plan_limits
		WHERE subscription_plan_uuid = ? AND limit_type = ?
	`), plan.UUID, constants.LimitTypeRequestCount); err != nil {
		return fmt.Errorf("failed to clear subscription plan limit: %w", err)
	}
	now := time.Now()
	// ThrottleLimitCount is passed as a *int so a nil count is persisted as NULL.
	if _, err := tx.Exec(r.db.Rebind(`
		INSERT INTO subscription_plan_limits (uuid, subscription_plan_uuid, organization_uuid,
			limit_type, limit_count, time_amount, time_unit, stop_on_quota_reach, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`),
		uuid.New().String(), plan.UUID, plan.OrganizationUUID, constants.LimitTypeRequestCount,
		plan.ThrottleLimitCount, 1, plan.ThrottleLimitUnit, plan.StopOnQuotaReach, now, now,
	); err != nil {
		return fmt.Errorf("failed to insert subscription plan limit: %w", err)
	}
	return nil
}

// scanPlan reads a subscription plan joined with its single throttling limit row
// (see planSelectColumns). When no limit row exists the throttle fields are left
// empty and StopOnQuotaReach defaults to 1.
func scanPlan(scanner rowScanner) (*model.SubscriptionPlan, error) {
	plan := &model.SubscriptionPlan{}
	var (
		limitCount  sql.NullInt64
		timeUnit    sql.NullString
		stopOnQuota sql.NullInt64
	)
	if err := scanner.Scan(
		&plan.UUID, &plan.Handle, &plan.Name, &plan.BillingPlan, &plan.ExpiryTime,
		&plan.OrganizationUUID, &plan.Status, &plan.CreatedAt, &plan.UpdatedAt,
		&limitCount, &timeUnit, &stopOnQuota,
	); err != nil {
		return nil, err
	}
	if limitCount.Valid {
		c := int(limitCount.Int64)
		plan.ThrottleLimitCount = &c
	}
	plan.ThrottleLimitUnit = timeUnit.String
	if stopOnQuota.Valid {
		plan.StopOnQuotaReach = int(stopOnQuota.Int64)
	} else {
		plan.StopOnQuotaReach = 1
	}
	return plan, nil
}

// GetByHandleAndOrg retrieves a subscription plan by handle and organization
func (r *SubscriptionPlanRepo) GetByHandleAndOrg(handle, orgUUID string) (*model.SubscriptionPlan, error) {
	query := `SELECT ` + planSelectColumns + `
		WHERE p.handle = ? AND p.organization_uuid = ?`
	return scanPlan(r.db.QueryRow(r.db.Rebind(query), handle, orgUUID))
}

// GetByID retrieves a subscription plan by ID and organization
func (r *SubscriptionPlanRepo) GetByID(planID, orgUUID string) (*model.SubscriptionPlan, error) {
	query := `SELECT ` + planSelectColumns + `
		WHERE p.uuid = ? AND p.organization_uuid = ?`
	return scanPlan(r.db.QueryRow(r.db.Rebind(query), planID, orgUUID))
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
		SELECT uuid, name
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
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		m[id] = name
	}
	return m, rows.Err()
}

// ListByOrganization returns subscription plans for an organization with pagination
func (r *SubscriptionPlanRepo) ListByOrganization(orgUUID string, limit, offset int) ([]*model.SubscriptionPlan, error) {
	pageClause, pageArgs := r.db.PaginationClause(limit, offset)
	query := `SELECT ` + planSelectColumns + `
		WHERE p.organization_uuid = ?
		ORDER BY p.created_at DESC
		` + pageClause
	rows, err := r.db.Query(r.db.Rebind(query), append([]any{orgUUID}, pageArgs...)...)
	if err != nil {
		return nil, fmt.Errorf("failed to list subscription plans: %w", err)
	}
	defer rows.Close()

	var list []*model.SubscriptionPlan
	for rows.Next() {
		plan, err := scanPlan(rows)
		if err != nil {
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

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.Exec(r.db.Rebind(`
		UPDATE subscription_plans
		SET handle = ?, name = ?, billing_plan = ?, expiry_time = ?, status = ?, updated_at = ?
		WHERE uuid = ? AND organization_uuid = ?
	`),
		plan.Handle, plan.Name, plan.BillingPlan, plan.ExpiryTime,
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

	if err := r.replaceSingleLimitTx(tx, plan); err != nil {
		return err
	}

	return tx.Commit()
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

// ExistsByHandleAndOrg returns true if a plan with the given handle exists in the organization
func (r *SubscriptionPlanRepo) ExistsByHandleAndOrg(handle, orgUUID string) (bool, error) {
	query := `
		SELECT 1 FROM subscription_plans
		WHERE handle = ? AND organization_uuid = ?
		ORDER BY (SELECT NULL)
		` + r.db.FetchFirstClause(1)
	var exists int
	err := r.db.QueryRow(r.db.Rebind(query), handle, orgUUID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return exists == 1, nil
}
