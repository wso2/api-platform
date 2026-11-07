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
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"

	"github.com/google/uuid"
)

// planSchema records which plan tables and columns exist. Server databases are set up by the operator,
// so the new table or column may be missing until the upgrade script runs, and new installs have no
// subscription_plan_limits. Checked again on each use until the table and column are both found.
type planSchema struct {
	conditions   bool // subscription_plan_conditions
	legacyLimits bool // subscription_plan_limits
	description  bool // subscription_plans.description
}

// ErrPlanConditionsTableMissing: the database has no subscription_plan_conditions table yet, and the
// plan has conditions that subscription_plan_limits can't hold.
var ErrPlanConditionsTableMissing = errors.New("the database has no subscription_plan_conditions table; " +
	"run this release's database upgrade to store this plan's conditions")

// ErrPlanDescriptionColumnMissing: the database has no subscription_plans.description column yet.
var ErrPlanDescriptionColumnMissing = errors.New("the database has no subscription_plans.description column; " +
	"run this release's database upgrade to store plan descriptions")

// SubscriptionPlanRepo implements SubscriptionPlanRepository
type SubscriptionPlanRepo struct {
	db *database.DB

	schemaMu      sync.Mutex
	planSchema    planSchema
	schemaUpdated bool
}

// NewSubscriptionPlanRepo creates a new subscription plan repository
func NewSubscriptionPlanRepo(db *database.DB) SubscriptionPlanRepository {
	return &SubscriptionPlanRepo{db: db}
}

// schema returns which plan tables and columns exist. Once the conditions table and the description
// column are found, the result is kept; until then it is checked again, so a failed check or an
// upgrade made while running is picked up without a restart.
func (r *SubscriptionPlanRepo) schema() planSchema {
	r.schemaMu.Lock()
	defer r.schemaMu.Unlock()
	if r.schemaUpdated {
		return r.planSchema
	}
	r.planSchema = planSchema{
		conditions:   r.db.HasTable("subscription_plan_conditions"),
		legacyLimits: r.db.HasTable("subscription_plan_limits"),
		description:  r.db.HasColumn("subscription_plans", "description"),
	}
	r.schemaUpdated = r.planSchema.conditions && r.planSchema.description
	return r.planSchema
}

// checkDescription refuses a description the database can't store yet, unless the plan allows
// dropping it (plans synced from an API Portal).
func checkDescription(schema planSchema, plan *model.SubscriptionPlan) error {
	if !schema.description && plan.Description != "" && !plan.DescriptionOptional {
		return ErrPlanDescriptionColumnMissing
	}
	return nil
}

// planSelectColumns reads a plan's own columns; loadConditions reads its conditions. The description
// reads as NULL when the column doesn't exist yet.
func (r *SubscriptionPlanRepo) planSelectColumns() string {
	description := "NULL"
	if r.schema().description {
		description = "p.description"
	}
	return `
		p.uuid, p.handle, p.display_name, ` + description + `, p.expiry_time,
		p.organization_uuid, p.status, p.created_by, p.updated_by, p.created_at, p.updated_at
	FROM subscription_plans p`
}

// Create inserts a new subscription plan together with its conditions.
func (r *SubscriptionPlanRepo) Create(plan *model.SubscriptionPlan) error {
	return r.CreateWithPricing(plan, nil)
}

// CreateWithPricing inserts a plan, its conditions and, for a paid plan, its pricing in one transaction.
// The plan keeps the CreatedAt the caller sets (a portal plan is dated when it was made there).
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
	// Read before the transaction: on SQLite, with one connection, the check would wait for it.
	schema := r.schema()
	if err := checkDescription(schema, plan); err != nil {
		return err
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	cols := []string{"uuid", "handle", "display_name", "expiry_time",
		"organization_uuid", "status", "created_by", "updated_by", "created_at", "updated_at"}
	args := []any{plan.UUID, plan.Handle, plan.Name, plan.ExpiryTime,
		plan.OrganizationUUID, string(plan.Status), plan.CreatedBy, plan.UpdatedBy, plan.CreatedAt, plan.UpdatedAt}
	if schema.description {
		cols = append(cols, "description")
		args = append(args, nullIfEmpty(plan.Description))
	}
	if _, err := tx.Exec(r.db.Rebind(`INSERT INTO subscription_plans (`+strings.Join(cols, ", ")+`)
		VALUES (`+inPlaceholders(len(cols))+`)`), args...); err != nil {
		return fmt.Errorf("failed to insert subscription plan: %w", err)
	}

	if err := r.saveConditionsTx(tx, plan, schema); err != nil {
		return err
	}

	if pricing != nil {
		pricing.SubscriptionPlanUUID = plan.UUID
		pricing.OrganizationUUID = plan.OrganizationUUID
		if err := insertSubscriptionPlanPricing(tx, r.db, pricing, now); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	if !schema.description {
		plan.Description = ""
	}
	plan.SetLegacyLimit()
	return nil
}

// saveConditionsTx replaces the plan's condition rows. It also deletes the plan's old
// subscription_plan_limits row, so a plan saved without conditions isn't read as the old limit again.
// Without the conditions table, it falls back to saveLegacyLimitTx.
func (r *SubscriptionPlanRepo) saveConditionsTx(tx *sql.Tx, plan *model.SubscriptionPlan, schema planSchema) error {
	if !schema.conditions {
		return r.saveLegacyLimitTx(tx, plan, schema)
	}
	if _, err := tx.Exec(r.db.Rebind(`
		DELETE FROM subscription_plan_conditions WHERE subscription_plan_uuid = ?
	`), plan.UUID); err != nil {
		return fmt.Errorf("failed to clear subscription plan conditions: %w", err)
	}
	for i, c := range plan.Conditions {
		if _, err := tx.Exec(r.db.Rebind(`
			INSERT INTO subscription_plan_conditions (uuid, subscription_plan_uuid, organization_uuid,
				condition_id, condition_type, params, sort_order)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`),
			uuid.New().String(), plan.UUID, plan.OrganizationUUID, c.ID, c.Type, []byte(c.Params), i,
		); err != nil {
			return fmt.Errorf("failed to insert subscription plan condition %s: %w", c.ID, err)
		}
	}
	if schema.legacyLimits {
		if _, err := tx.Exec(r.db.Rebind(`
			DELETE FROM subscription_plan_limits WHERE subscription_plan_uuid = ?
		`), plan.UUID); err != nil {
			return fmt.Errorf("failed to clear the subscription plan's earlier limit: %w", err)
		}
	}
	plan.LegacyLimits = false
	return nil
}

// saveLegacyLimitTx stores the request-count condition in subscription_plan_limits, one row per window.
// Other conditions return ErrPlanConditionsTableMissing.
func (r *SubscriptionPlanRepo) saveLegacyLimitTx(tx *sql.Tx, plan *model.SubscriptionPlan, schema planSchema) error {
	var limits model.ConditionLimits
	for _, c := range plan.Conditions {
		if c.Type != model.ConditionTypeRequestCount || !schema.legacyLimits {
			return ErrPlanConditionsTableMissing
		}
		if err := json.Unmarshal(c.Params, &limits); err != nil {
			return fmt.Errorf("failed to read the %s condition: %w", c.ID, err)
		}
	}
	if !schema.legacyLimits {
		return nil
	}
	if _, err := tx.Exec(r.db.Rebind(`
		DELETE FROM subscription_plan_limits WHERE subscription_plan_uuid = ?
	`), plan.UUID); err != nil {
		return fmt.Errorf("failed to clear subscription plan limit: %w", err)
	}
	// stop_on_quota_reach is a SMALLINT (0/1) column.
	stopOnQuotaReach := 1
	if limits.OnExceed == model.ConditionOnExceedAllow {
		stopOnQuotaReach = 0
	}
	for _, w := range limits.Limits {
		if _, err := tx.Exec(r.db.Rebind(`
			INSERT INTO subscription_plan_limits (uuid, subscription_plan_uuid,
				limit_type, time_unit, time_amount, limit_count, stop_on_quota_reach)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`),
			uuid.New().String(), plan.UUID, constants.LimitTypeRequestCount,
			w.TimeUnit, w.TimeAmount, w.Count, stopOnQuotaReach,
		); err != nil {
			return fmt.Errorf("failed to insert subscription plan limit: %w", err)
		}
	}
	plan.LegacyLimits = len(limits.Limits) > 0
	return nil
}

// scanPlan reads a plan's own columns (see planSelectColumns).
func scanPlan(scanner rowScanner) (*model.SubscriptionPlan, error) {
	plan := &model.SubscriptionPlan{}
	var createdBy, updatedBy, description sql.NullString
	if err := scanner.Scan(
		&plan.UUID, &plan.Handle, &plan.Name, &description, &plan.ExpiryTime,
		&plan.OrganizationUUID, &plan.Status, &createdBy, &updatedBy, &plan.CreatedAt, &plan.UpdatedAt,
	); err != nil {
		return nil, err
	}
	plan.Description = description.String
	plan.CreatedBy = createdBy.String
	plan.UpdatedBy = updatedBy.String
	return plan, nil
}

// getPlan reads one plan with its conditions.
func (r *SubscriptionPlanRepo) getPlan(where string, args ...any) (*model.SubscriptionPlan, error) {
	plan, err := scanPlan(r.db.QueryRow(r.db.Rebind(`SELECT `+r.planSelectColumns()+` WHERE `+where), args...))
	if err != nil {
		return nil, err
	}
	if err := r.loadConditions([]*model.SubscriptionPlan{plan}); err != nil {
		return nil, err
	}
	return plan, nil
}

// loadConditions sets each plan's conditions from subscription_plan_conditions, or, when it has none
// there, from its old subscription_plan_limits rows. Then it fills the legacy request limit.
func (r *SubscriptionPlanRepo) loadConditions(plans []*model.SubscriptionPlan) error {
	if len(plans) == 0 {
		return nil
	}
	byUUID := make(map[string]*model.SubscriptionPlan, len(plans))
	args := make([]any, 0, len(plans))
	for _, p := range plans {
		p.Conditions = []model.SubscriptionPlanCondition{}
		p.LegacyLimits = false
		if _, seen := byUUID[p.UUID]; !seen {
			args = append(args, p.UUID)
		}
		byUUID[p.UUID] = p
	}

	schema := r.schema()
	if schema.conditions {
		rows, err := r.db.Query(r.db.Rebind(`
			SELECT subscription_plan_uuid, condition_id, condition_type, params
			FROM subscription_plan_conditions
			WHERE subscription_plan_uuid IN (`+inPlaceholders(len(args))+`)
			ORDER BY subscription_plan_uuid, sort_order
		`), args...)
		if err != nil {
			return fmt.Errorf("failed to read subscription plan conditions: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var planUUID string
			var params []byte
			var c model.SubscriptionPlanCondition
			if err := rows.Scan(&planUUID, &c.ID, &c.Type, &params); err != nil {
				return err
			}
			c.Params = json.RawMessage(params)
			if p := byUUID[planUUID]; p != nil {
				p.Conditions = append(p.Conditions, c)
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
	}

	if schema.legacyLimits {
		var legacy []any
		for _, uuid := range args {
			if len(byUUID[uuid.(string)].Conditions) == 0 {
				legacy = append(legacy, uuid)
			}
		}
		if len(legacy) > 0 {
			if err := r.loadLegacyLimits(byUUID, legacy); err != nil {
				return err
			}
		}
	}

	for _, p := range plans {
		p.SetLegacyLimit()
	}
	return nil
}

// loadLegacyLimits reads old REQUEST_COUNT rows as each plan's request-count condition.
func (r *SubscriptionPlanRepo) loadLegacyLimits(byUUID map[string]*model.SubscriptionPlan, planUUIDs []any) error {
	args := append([]any{constants.LimitTypeRequestCount}, planUUIDs...)
	rows, err := r.db.Query(r.db.Rebind(`
		SELECT subscription_plan_uuid, time_unit, time_amount, limit_count, stop_on_quota_reach
		FROM subscription_plan_limits
		WHERE limit_type = ? AND subscription_plan_uuid IN (`+inPlaceholders(len(planUUIDs))+`)
		ORDER BY subscription_plan_uuid, time_amount, time_unit
	`), args...)
	if err != nil {
		return fmt.Errorf("failed to read subscription plan limits: %w", err)
	}
	defer rows.Close()

	windows := map[string][]model.ConditionWindow{}
	stop := map[string]bool{}
	var order []string
	for rows.Next() {
		var planUUID, timeUnit string
		var timeAmount int
		var count int64
		var stopOnQuota int64
		if err := rows.Scan(&planUUID, &timeUnit, &timeAmount, &count, &stopOnQuota); err != nil {
			return err
		}
		if _, seen := windows[planUUID]; !seen {
			order = append(order, planUUID)
			stop[planUUID] = stopOnQuota != 0
		}
		if timeAmount < 1 {
			timeAmount = 1
		}
		windows[planUUID] = append(windows[planUUID], model.ConditionWindow{
			Count: int(count), TimeAmount: timeAmount, TimeUnit: strings.ToUpper(timeUnit),
		})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, planUUID := range order {
		sortWindows(windows[planUUID])
		p := byUUID[planUUID]
		p.Conditions = []model.SubscriptionPlanCondition{model.NewRequestCountCondition(windows[planUUID], stop[planUUID])}
		p.LegacyLimits = true
	}
	return nil
}

// sortWindows orders old limit rows shortest window first, since they carry no order of their own.
func sortWindows(windows []model.ConditionWindow) {
	minutes := map[string]int{"MINUTE": 1, "HOUR": 60, "DAY": 24 * 60, "MONTH": 31 * 24 * 60}
	slices.SortStableFunc(windows, func(a, b model.ConditionWindow) int {
		return minutes[a.TimeUnit]*a.TimeAmount - minutes[b.TimeUnit]*b.TimeAmount
	})
}

// inPlaceholders returns n comma-separated "?" placeholders.
func inPlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// GetByHandleAndOrg retrieves a subscription plan by handle and organization
func (r *SubscriptionPlanRepo) GetByHandleAndOrg(handle, orgUUID string) (*model.SubscriptionPlan, error) {
	return r.getPlan(`p.handle = ? AND p.organization_uuid = ?`, handle, orgUUID)
}

// GetByID retrieves a subscription plan by ID and organization
func (r *SubscriptionPlanRepo) GetByID(planID, orgUUID string) (*model.SubscriptionPlan, error) {
	return r.getPlan(`p.uuid = ? AND p.organization_uuid = ?`, planID, orgUUID)
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
	query := `SELECT ` + r.planSelectColumns() + `
		WHERE p.handle IN (` + strings.Join(placeholders, ",") + `) AND p.organization_uuid = ?`
	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to load subscription plans by handle: %w", err)
	}
	defer rows.Close()

	plans := make(map[string]*model.SubscriptionPlan, len(handles))
	var list []*model.SubscriptionPlan
	for rows.Next() {
		plan, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		plans[plan.Handle] = plan
		list = append(list, plan)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if err := r.loadConditions(list); err != nil {
		return nil, err
	}
	return plans, nil
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
	query := `SELECT ` + r.planSelectColumns() + `
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if err := r.loadConditions(list); err != nil {
		return nil, err
	}
	return list, nil
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
	// Read before the transaction: on SQLite, with one connection, the check would wait for it.
	schema := r.schema()
	if err := checkDescription(schema, plan); err != nil {
		return err
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	set := "handle = ?, display_name = ?, expiry_time = ?, status = ?, updated_by = ?, updated_at = ?"
	args := []any{plan.Handle, plan.Name, plan.ExpiryTime, string(plan.Status), plan.UpdatedBy, plan.UpdatedAt}
	if schema.description {
		set += ", description = ?"
		args = append(args, nullIfEmpty(plan.Description))
	}
	args = append(args, plan.UUID, plan.OrganizationUUID)
	result, err := tx.Exec(r.db.Rebind(`
		UPDATE subscription_plans SET `+set+`
		WHERE uuid = ? AND organization_uuid = ?
	`), args...)
	if err != nil {
		return fmt.Errorf("failed to update subscription plan: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("subscription plan not found: %s", plan.UUID)
	}

	if err := r.saveConditionsTx(tx, plan, schema); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	if !schema.description {
		plan.Description = ""
	}
	plan.SetLegacyLimit()
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

// restAPIPlanHandles returns the subscription plan handles a REST API's or MCP proxy's stored configuration lists;
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

	var err error
	if usage.RESTAPIs, err = r.countPlanListers(planListerRESTAPIs, handle, orgUUID); err != nil {
		return nil, err
	}
	if usage.MCPProxies, err = r.countPlanListers(planListerMCPProxies, handle, orgUUID); err != nil {
		return nil, err
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
//   - the plan is deleted; its conditions and pricing go with it (ON DELETE CASCADE).
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
	for _, lister := range []planLister{planListerRESTAPIs, planListerMCPProxies} {
		if err := r.removePlanFromListersTx(tx, lister, handle, orgUUID, now); err != nil {
			return false, err
		}
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

// planLister is a table of artifacts whose configuration lists plan handles in subscriptionPlans.
type planLister struct{ table, kind, kinds string }

var (
	planListerRESTAPIs   = planLister{"rest_apis", "REST API", "REST APIs"}
	planListerMCPProxies = planLister{"mcp_proxies", "MCP proxy", "MCP proxies"}
)

// countPlanListers counts the artifacts of the table whose configuration lists the plan's handle.
func (r *SubscriptionPlanRepo) countPlanListers(l planLister, handle, orgUUID string) (int, error) {
	rows, err := r.db.Query(r.db.Rebind(`SELECT handle, configuration FROM `+l.table+` WHERE organization_uuid = ?`), orgUUID)
	if err != nil {
		return 0, fmt.Errorf("failed to list %s: %w", l.kinds, err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var artifactHandle string
		var configuration []byte
		if err := rows.Scan(&artifactHandle, &configuration); err != nil {
			return 0, fmt.Errorf("failed to scan %s: %w", l.kind, err)
		}
		plans, err := restAPIPlanHandles(configuration)
		if err != nil {
			return 0, fmt.Errorf("failed to decode the configuration of %s %s: %w", l.kind, artifactHandle, err)
		}
		if slices.Contains(plans, handle) {
			n++
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("failed to list %s: %w", l.kinds, err)
	}
	return n, nil
}

// removePlanFromListersTx removes the plan's handle from the subscriptionPlans of every artifact of
// the table that lists it.
func (r *SubscriptionPlanRepo) removePlanFromListersTx(tx *sql.Tx, l planLister, handle, orgUUID string, now time.Time) error {
	rows, err := tx.Query(r.db.Rebind(`SELECT uuid, configuration FROM `+l.table+` WHERE organization_uuid = ?`), orgUUID)
	if err != nil {
		return fmt.Errorf("failed to list %s: %w", l.kinds, err)
	}
	updated := map[string][]byte{}
	for rows.Next() {
		var artifactUUID string
		var configuration []byte
		if err := rows.Scan(&artifactUUID, &configuration); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan %s: %w", l.kind, err)
		}
		changed, err := withoutPlanHandle(configuration, handle)
		if err != nil {
			rows.Close()
			return fmt.Errorf("failed to remove the plan from %s %s: %w", l.kind, artifactUUID, err)
		}
		if changed != nil {
			updated[artifactUUID] = changed
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to list %s: %w", l.kinds, err)
	}
	for artifactUUID, configuration := range updated {
		if _, err := tx.Exec(r.db.Rebind(`UPDATE `+l.table+` SET configuration = ?, updated_at = ? WHERE uuid = ? AND organization_uuid = ?`),
			configuration, now, artifactUUID, orgUUID); err != nil {
			return fmt.Errorf("failed to remove the plan from %s %s: %w", l.kind, artifactUUID, err)
		}
	}
	return nil
}

// withoutPlanHandle returns the stored configuration with handle removed from its
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
