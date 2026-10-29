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
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"

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
		p.uuid, p.handle, p.display_name, p.expiry_time,
		p.organization_uuid, p.status, p.created_by, p.updated_by, p.created_at, p.updated_at,
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
	return r.CreateWithPricing(plan, nil)
}

// CreateWithPricing inserts a new subscription plan, its throttling limit row and, for a paid plan
// (pricing not nil), its pricing, all in one transaction. The plan is dated with the CreatedAt the
// caller sets, or now when none is set. A plan copied from an API Portal is given the time it was
// made there.
func (r *SubscriptionPlanRepo) CreateWithPricing(plan *model.SubscriptionPlan, pricing *model.SubscriptionPlanPricing) error {
	if plan == nil {
		return fmt.Errorf("subscription plan is required")
	}
	if plan.UUID == "" {
		plan.UUID = uuid.New().String()
	}
	now := time.Now().UTC()
	if plan.CreatedAt.IsZero() {
		plan.CreatedAt = now
	}
	plan.UpdatedAt = now

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(r.db.Rebind(`
		INSERT INTO subscription_plans (uuid, handle, display_name, expiry_time,
			organization_uuid, status, created_by, updated_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`),
		plan.UUID, plan.Handle, plan.Name, plan.ExpiryTime,
		plan.OrganizationUUID, string(plan.Status), plan.CreatedBy, plan.UpdatedBy, plan.CreatedAt, plan.UpdatedAt,
	); err != nil {
		return fmt.Errorf("failed to insert subscription plan: %w", err)
	}

	if err := r.replaceSingleLimitTx(tx, plan); err != nil {
		return err
	}

	if pricing != nil {
		pricing.SubscriptionPlanUUID = plan.UUID
		pricing.OrganizationUUID = plan.OrganizationUUID
		if err := insertSubscriptionPlanPricing(tx, r.db, pricing, now); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// replaceSingleLimitTx clears the plan's single REQUEST_COUNT limit row and re-inserts
// it from the plan's ThrottleLimitCount / ThrottleLimitUnit / StopOnQuotaReach fields.
//
// limit_count is NOT NULL, so a row is written ONLY when ThrottleLimitCount is set.
// When it is nil the plan has no quota: the row is left deleted and reads default
// StopOnQuotaReach to true (see scanPlan).
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
	// No quota configured: leave the row deleted (limit_count is NOT NULL).
	if plan.ThrottleLimitCount == nil {
		return nil
	}
	// stop_on_quota_reach is a SMALLINT (0/1) column; map the boolean domain field onto it.
	stopOnQuotaReach := 0
	if plan.StopOnQuotaReach {
		stopOnQuotaReach = 1
	}
	if _, err := tx.Exec(r.db.Rebind(`
		INSERT INTO subscription_plan_limits (uuid, subscription_plan_uuid,
			limit_type, time_unit, time_amount, limit_count, stop_on_quota_reach)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`),
		uuid.New().String(), plan.UUID, constants.LimitTypeRequestCount,
		plan.ThrottleLimitUnit, 1, *plan.ThrottleLimitCount, stopOnQuotaReach,
	); err != nil {
		return fmt.Errorf("failed to insert subscription plan limit: %w", err)
	}
	return nil
}

// scanPlan reads a subscription plan joined with its single throttling limit row
// (see planSelectColumns). When no limit row exists the throttle fields are left
// empty and StopOnQuotaReach defaults to true.
func scanPlan(scanner rowScanner) (*model.SubscriptionPlan, error) {
	plan := &model.SubscriptionPlan{}
	var (
		createdBy, updatedBy sql.NullString
		limitCount           sql.NullInt64
		timeUnit             sql.NullString
		stopOnQuota          sql.NullInt64
	)
	if err := scanner.Scan(
		&plan.UUID, &plan.Handle, &plan.Name, &plan.ExpiryTime,
		&plan.OrganizationUUID, &plan.Status, &createdBy, &updatedBy, &plan.CreatedAt, &plan.UpdatedAt,
		&limitCount, &timeUnit, &stopOnQuota,
	); err != nil {
		return nil, err
	}
	plan.CreatedBy = createdBy.String
	plan.UpdatedBy = updatedBy.String
	if limitCount.Valid {
		c := int(limitCount.Int64)
		plan.ThrottleLimitCount = &c
	}
	plan.ThrottleLimitUnit = timeUnit.String
	if stopOnQuota.Valid {
		plan.StopOnQuotaReach = stopOnQuota.Int64 != 0
	} else {
		// No limit row: default to blocking on quota reach (SMALLINT default 1).
		plan.StopOnQuotaReach = true
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
		SELECT uuid, display_name
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

// GetByHandles loads each plan whose handle is in handles, keyed by handle, scoped
// to the organization. A handle absent from the returned map does not exist in the
// org's catalog. Returns an empty map for empty input.
func (r *SubscriptionPlanRepo) GetByHandles(handles []string, orgUUID string) (map[string]*model.SubscriptionPlan, error) {
	if len(handles) == 0 {
		return map[string]*model.SubscriptionPlan{}, nil
	}
	placeholders := make([]string, len(handles))
	args := make([]interface{}, 0, len(handles)+1)
	for i, h := range handles {
		placeholders[i] = "?"
		args = append(args, h)
	}
	args = append(args, orgUUID)
	query := `SELECT ` + planSelectColumns + `
		WHERE p.handle IN (` + strings.Join(placeholders, ",") + `) AND p.organization_uuid = ?`
	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to load subscription plans by handle: %w", err)
	}
	defer rows.Close()

	plans := make(map[string]*model.SubscriptionPlan, len(handles))
	for rows.Next() {
		plan, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		plans[plan.Handle] = plan
	}
	return plans, rows.Err()
}

// GetHandlesByIDs maps subscription_plan_uuid to handle, for reconstructing a
// subscriptionPlanIds response from stored mapping rows. Returns an empty map for
// empty input.
func (r *SubscriptionPlanRepo) GetHandlesByIDs(planUUIDs []string, orgUUID string) (map[string]string, error) {
	if len(planUUIDs) == 0 {
		return map[string]string{}, nil
	}
	placeholders := make([]string, len(planUUIDs))
	args := make([]interface{}, 0, len(planUUIDs)+1)
	for i, id := range planUUIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	args = append(args, orgUUID)
	query := fmt.Sprintf(`
		SELECT uuid, handle
		FROM subscription_plans
		WHERE uuid IN (%s) AND organization_uuid = ?
	`, strings.Join(placeholders, ","))
	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve subscription plan uuids: %w", err)
	}
	defer rows.Close()
	m := make(map[string]string)
	for rows.Next() {
		var uuid, handle string
		if err := rows.Scan(&uuid, &handle); err != nil {
			return nil, err
		}
		m[uuid] = handle
	}
	return m, rows.Err()
}

// planListClause is the filter ListByOrganization and CountByOrganization share: the optional
// case-insensitive match on the plan name or handle, and the optional status ("" for any).
func planListClause(search, status string) (string, []any) {
	clause, args := handleSearchClause(search)
	if status != "" {
		clause += " AND status = ?"
		args = append(args, status)
	}
	return clause, args
}

// ListByOrganization returns subscription plans for an organization with pagination,
// optionally filtered by a case-insensitive match on the plan name or handle and by status
// ("" for any).
func (r *SubscriptionPlanRepo) ListByOrganization(orgUUID string, opts ListOptions, status string) ([]*model.SubscriptionPlan, error) {
	query := `SELECT ` + planSelectColumns + `
		WHERE p.organization_uuid = ?`
	args := []any{orgUUID}
	if filterClause, filterArgs := planListClause(opts.Search, status); filterClause != "" {
		query += filterClause
		args = append(args, filterArgs...)
	}
	pageClause, pageArgs := r.db.PaginationClause(opts.Limit, opts.Offset)
	query += " ORDER BY p.created_at DESC " + pageClause
	args = append(args, pageArgs...)
	rows, err := r.db.Query(r.db.Rebind(query), args...)
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

// CountByOrganization returns the number of subscription plans in an organization
// that match the same search and status as ListByOrganization, independent of pagination.
func (r *SubscriptionPlanRepo) CountByOrganization(orgUUID, search, status string) (int, error) {
	var total int
	query := `SELECT COUNT(*) FROM subscription_plans WHERE organization_uuid = ?`
	args := []any{orgUUID}
	if filterClause, filterArgs := planListClause(search, status); filterClause != "" {
		query += filterClause
		args = append(args, filterArgs...)
	}
	if err := r.db.QueryRow(r.db.Rebind(query), args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("failed to count subscription plans: %w", err)
	}
	return total, nil
}

// Update updates an existing subscription plan
func (r *SubscriptionPlanRepo) Update(plan *model.SubscriptionPlan) error {
	if plan == nil {
		return fmt.Errorf("subscription plan is required")
	}
	plan.UpdatedAt = time.Now().UTC()

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.Exec(r.db.Rebind(`
		UPDATE subscription_plans
		SET handle = ?, display_name = ?, expiry_time = ?, status = ?, updated_by = ?, updated_at = ?
		WHERE uuid = ? AND organization_uuid = ?
	`),
		plan.Handle, plan.Name, plan.ExpiryTime,
		string(plan.Status), plan.UpdatedBy, plan.UpdatedAt,
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

// restAPIPlanHandles returns the subscription plan handles a REST API's stored configuration lists;
// an empty configuration lists none.
func restAPIPlanHandles(configuration []byte) ([]string, error) {
	if len(bytes.TrimSpace(configuration)) == 0 {
		return nil, nil
	}
	var config struct {
		SubscriptionPlans []string `json:"subscriptionPlans"`
	}
	if err := json.Unmarshal(configuration, &config); err != nil {
		return nil, err
	}
	return config.SubscriptionPlans, nil
}

// GetUsage counts what offers or uses the plan (see model.SubscriptionPlanUsage): the REST APIs
// whose configuration lists its handle, the APIs whose publication drafts or live publications
// offer it, and its subscriptions.
func (r *SubscriptionPlanRepo) GetUsage(planUUID, handle, orgUUID string) (*model.SubscriptionPlanUsage, error) {
	usage := &model.SubscriptionPlanUsage{}
	drafts, publications := map[string]bool{}, map[string]bool{}

	rows, err := r.db.Query(r.db.Rebind(`SELECT handle, configuration FROM rest_apis WHERE organization_uuid = ?`), orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to list REST APIs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var apiHandle string
		var configuration []byte
		if err := rows.Scan(&apiHandle, &configuration); err != nil {
			return nil, fmt.Errorf("failed to scan REST API: %w", err)
		}
		plans, err := restAPIPlanHandles(configuration)
		if err != nil {
			return nil, fmt.Errorf("failed to decode the configuration of REST API %s: %w", apiHandle, err)
		}
		if slices.Contains(plans, handle) {
			usage.RESTAPIs++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list REST APIs: %w", err)
	}

	// Each API is counted once, by name, however many portals it is on: a REST API by its handle,
	// any other API by its publication's display name.
	pubRows, err := r.db.Query(r.db.Rebind(`
		SELECT p.is_draft, COALESCE(ra.handle, p.display_name)
		FROM api_publication_plan_mappings m
		JOIN api_publications p ON p.uuid = m.publication_uuid AND p.organization_uuid = m.organization_uuid
		LEFT JOIN rest_apis ra ON ra.uuid = p.artifact_uuid
		WHERE m.organization_uuid = ? AND m.subscription_plan_uuid = ?
	`), orgUUID, planUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to list publications offering the plan: %w", err)
	}
	defer pubRows.Close()
	for pubRows.Next() {
		var isDraft int
		var name string
		if err := pubRows.Scan(&isDraft, &name); err != nil {
			return nil, fmt.Errorf("failed to scan publication: %w", err)
		}
		if isDraft == 1 {
			drafts[name] = true
		} else {
			publications[name] = true
		}
	}
	if err := pubRows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list publications offering the plan: %w", err)
	}

	if err := r.db.QueryRow(r.db.Rebind(`SELECT COUNT(*) FROM subscriptions WHERE organization_uuid = ? AND subscription_plan_uuid = ?`),
		orgUUID, planUUID).Scan(&usage.Subscriptions); err != nil {
		return nil, fmt.Errorf("failed to count subscriptions on the plan: %w", err)
	}

	usage.Drafts, usage.Publications = len(drafts), len(publications)
	return usage, nil
}

// DeleteWithReferences deletes the plan and every reference to it, in one transaction:
//   - the plan is removed from the REST APIs whose configuration lists its handle and from
//     artifact_subscription_plans;
//   - the publication rows (drafts and live) that offer it are kept: they hold their own copy of the
//     plan and show it as deleted;
//   - its subscriptions are kept, without a plan;
//   - the plan is deleted; its limits and pricing go with it (ON DELETE CASCADE).
//
// It returns false when the plan does not exist.
func (r *SubscriptionPlanRepo) DeleteWithReferences(planUUID, handle, orgUUID string) (bool, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	now := time.Now().UTC()

	if _, err := tx.Exec(r.db.Rebind(`DELETE FROM artifact_subscription_plans WHERE subscription_plan_uuid = ?`), planUUID); err != nil {
		return false, fmt.Errorf("failed to remove the plan from artifacts: %w", err)
	}
	if err := r.removePlanFromRESTAPIsTx(tx, handle, orgUUID, now); err != nil {
		return false, err
	}
	if _, err := tx.Exec(r.db.Rebind(`UPDATE subscriptions SET subscription_plan_uuid = NULL, updated_at = ?
		WHERE organization_uuid = ? AND subscription_plan_uuid = ?`), now, orgUUID, planUUID); err != nil {
		return false, fmt.Errorf("failed to clear the plan of its subscriptions: %w", err)
	}
	result, err := tx.Exec(r.db.Rebind(`DELETE FROM subscription_plans WHERE uuid = ? AND organization_uuid = ?`), planUUID, orgUUID)
	if err != nil {
		return false, fmt.Errorf("failed to delete subscription plan: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return false, nil
	}
	return true, tx.Commit()
}

// removePlanFromRESTAPIsTx removes the plan handle from the subscriptionPlans of every REST API of
// the organization that lists it, leaving the rest of each configuration as it is.
func (r *SubscriptionPlanRepo) removePlanFromRESTAPIsTx(tx *sql.Tx, handle, orgUUID string, now time.Time) error {
	rows, err := tx.Query(r.db.Rebind(`SELECT uuid, configuration FROM rest_apis WHERE organization_uuid = ?`), orgUUID)
	if err != nil {
		return fmt.Errorf("failed to list REST APIs: %w", err)
	}
	updated := map[string][]byte{}
	for rows.Next() {
		var apiUUID string
		var configuration []byte
		if err := rows.Scan(&apiUUID, &configuration); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan REST API: %w", err)
		}
		changed, err := withoutPlanHandle(configuration, handle)
		if err != nil {
			rows.Close()
			return fmt.Errorf("failed to remove the plan from REST API %s: %w", apiUUID, err)
		}
		if changed != nil {
			updated[apiUUID] = changed
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to list REST APIs: %w", err)
	}
	for apiUUID, configuration := range updated {
		if _, err := tx.Exec(r.db.Rebind(`UPDATE rest_apis SET configuration = ?, updated_at = ? WHERE uuid = ? AND organization_uuid = ?`),
			configuration, now, apiUUID, orgUUID); err != nil {
			return fmt.Errorf("failed to remove the plan from REST API %s: %w", apiUUID, err)
		}
	}
	return nil
}

// withoutPlanHandle returns the REST API configuration with handle removed from its
// subscriptionPlans, or nil when it does not list it. The other fields are kept as stored.
func withoutPlanHandle(configuration []byte, handle string) ([]byte, error) {
	plans, err := restAPIPlanHandles(configuration)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(plans, handle) {
		return nil, nil
	}
	var config map[string]json.RawMessage
	if err := json.Unmarshal(configuration, &config); err != nil {
		return nil, err
	}
	kept := slices.DeleteFunc(plans, func(h string) bool { return h == handle })
	if len(kept) == 0 {
		delete(config, "subscriptionPlans")
	} else {
		encoded, err := json.Marshal(kept)
		if err != nil {
			return nil, err
		}
		config["subscriptionPlans"] = encoded
	}
	return json.Marshal(config)
}
