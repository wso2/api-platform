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

package apiportal

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/components"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// registerLegacyPlans binds the steps that store a plan's limits as a release before plan
// conditions stored them, and read back how the portal stores a plan.
func registerLegacyPlans(sc *godog.ScenarioContext, s *Steps) {
	sc.Step(`^the API Portal "([^"]*)" database holds subscription plan "([^"]*)" as a release before plan conditions stored it, with limits:$`,
		s.seedLegacyPlanLimits)
	sc.Step(`^the API Portal "([^"]*)" database should hold these earlier limit rows for subscription plan "([^"]*)":$`,
		s.assertLegacyPlanLimits)
	sc.Step(`^the API Portal "([^"]*)" database should hold (\d+) condition rows? for subscription plan "([^"]*)"$`,
		s.assertPlanConditionRows)
}

// portalLegacyLimitsDDL creates subscription_plan_limits as the releases before plan conditions
// did. This release no longer creates it, so only a database upgraded from one of those releases
// has it.
var portalLegacyLimitsDDL = map[components.DBType]string{
	components.Postgres: `CREATE TABLE IF NOT EXISTS subscription_plan_limits (
		uuid VARCHAR(40) NOT NULL,
		plan_uuid VARCHAR(40) NOT NULL,
		portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
		limit_type VARCHAR(20) NOT NULL DEFAULT 'REQUEST_COUNT',
		time_unit VARCHAR(20),
		time_amount INTEGER NOT NULL DEFAULT 1,
		limit_count BIGINT NOT NULL,
		PRIMARY KEY (portal_id, uuid),
		FOREIGN KEY (portal_id, plan_uuid) REFERENCES subscription_plans(portal_id, uuid) ON DELETE CASCADE)`,
	components.SQLServer: `IF OBJECT_ID(N'dbo.subscription_plan_limits', N'U') IS NULL
	CREATE TABLE dbo.subscription_plan_limits (
		uuid VARCHAR(40) NOT NULL,
		plan_uuid VARCHAR(40) NOT NULL,
		portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
		limit_type VARCHAR(20) NOT NULL DEFAULT 'REQUEST_COUNT',
		time_unit VARCHAR(20),
		time_amount INT NOT NULL DEFAULT 1,
		limit_count BIGINT NOT NULL,
		PRIMARY KEY (portal_id, uuid),
		FOREIGN KEY (portal_id, plan_uuid) REFERENCES subscription_plans(portal_id, uuid) ON DELETE CASCADE)`,
}

// legacyLimit is one subscription_plan_limits row, as a feature table names it.
type legacyLimit struct {
	limitType  string
	timeUnit   string
	timeAmount int
	limitCount int64
}

func (l legacyLimit) String() string {
	return fmt.Sprintf("%s %d per %d %s", l.limitType, l.limitCount, l.timeAmount, l.timeUnit)
}

// legacyLimitsFrom reads a table with the columns limit_type, time_unit, time_amount and limit_count.
func legacyLimitsFrom(table *godog.Table) ([]legacyLimit, error) {
	if table == nil || len(table.Rows) < 2 {
		return nil, fmt.Errorf("the limits table needs a header row and at least one limit")
	}
	col := map[string]int{}
	for i, c := range table.Rows[0].Cells {
		col[strings.TrimSpace(c.Value)] = i
	}
	for _, name := range []string{"limit_type", "time_unit", "time_amount", "limit_count"} {
		if _, ok := col[name]; !ok {
			return nil, fmt.Errorf("the limits table has no %q column", name)
		}
	}
	var out []legacyLimit
	for _, row := range table.Rows[1:] {
		cell := func(name string) string { return strings.TrimSpace(row.Cells[col[name]].Value) }
		amount, err := strconv.Atoi(cell("time_amount"))
		if err != nil {
			return nil, fmt.Errorf("time_amount %q: %w", cell("time_amount"), err)
		}
		count, err := strconv.ParseInt(cell("limit_count"), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("limit_count %q: %w", cell("limit_count"), err)
		}
		out = append(out, legacyLimit{limitType: cell("limit_type"), timeUnit: cell("time_unit"), timeAmount: amount, limitCount: count})
	}
	return out, nil
}

// planDatabase is a direct connection to a portal's database, for a plan the portal stores.
type planDatabase struct {
	*portalDatabase
}

// openPlanDB connects to the portal's database. The plan's own row gives its portal_id.
func (s *Steps) openPlanDB(ctx context.Context, portal string) (*planDatabase, error) {
	store, ok := s.topo.Storage.Plan.StoreFor(portal, 0)
	if !ok {
		return nil, fmt.Errorf("API Portal component %q has no database", portal)
	}
	db, closeDB, err := s.topo.OpenComponentDB(ctx, portal, "")
	if err != nil {
		return nil, err
	}
	return &planDatabase{&portalDatabase{db: db, close: closeDB, engine: store.Type}}, nil
}

// planRow finds the plan's uuid and portal_id by its handle.
func (p *planDatabase) planRow(ctx context.Context, handle string) (id, portalID string, err error) {
	if err := p.db.QueryRowContext(ctx, p.bind(`SELECT uuid, portal_id FROM subscription_plans WHERE handle = ?`), handle).Scan(&id, &portalID); err != nil {
		return "", "", fmt.Errorf("finding subscription plan %q: %w", handle, err)
	}
	return id, portalID, nil
}

// seedLegacyPlanLimits turns a plan into one saved before plan conditions: its condition rows are
// removed and its limits are stored in subscription_plan_limits, as those releases stored them.
// The portal reads the table only after a restart, as after an upgrade.
func (s *Steps) seedLegacyPlanLimits(ctx context.Context, portal, handle string, table *godog.Table) error {
	handle, err := stepscommon.Expand(ctx, handle)
	if err != nil {
		return err
	}
	limits, err := legacyLimitsFrom(table)
	if err != nil {
		return err
	}
	database, err := s.openPlanDB(ctx, portal)
	if err != nil {
		return err
	}
	defer func() { _ = database.close() }()
	if database.engine != components.Postgres && database.engine != components.SQLServer {
		return fmt.Errorf("the API Portal uses an embedded %s store, which a step cannot write to; "+
			"run this scenario in a block whose api-portal database is postgres or sqlserver", database.engine)
	}
	if _, err := database.db.ExecContext(ctx, portalLegacyLimitsDDL[database.engine]); err != nil {
		return fmt.Errorf("creating the earlier subscription_plan_limits table: %w", err)
	}
	planID, portalID, err := database.planRow(ctx, handle)
	if err != nil {
		return err
	}
	tx, err := database.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, database.bind(`DELETE FROM subscription_plan_conditions WHERE plan_uuid = ? AND portal_id = ?`),
		planID, portalID); err != nil {
		return fmt.Errorf("removing the conditions of plan %q: %w", handle, err)
	}
	for _, l := range limits {
		id, err := unique.Unique(ctx, "legacy-limit")
		if err != nil {
			return err
		}
		var unit any = l.timeUnit
		if l.timeUnit == "" {
			unit = nil
		}
		if _, err := tx.ExecContext(ctx, database.bind(`INSERT INTO subscription_plan_limits (uuid, portal_id, plan_uuid, limit_type,
			time_unit, time_amount, limit_count) VALUES (?, ?, ?, ?, ?, ?, ?)`),
			id, portalID, planID, l.limitType, unit, l.timeAmount, l.limitCount); err != nil {
			return fmt.Errorf("storing limit %s of plan %q: %w", l, handle, err)
		}
	}
	return tx.Commit()
}

// assertLegacyPlanLimits compares the plan's subscription_plan_limits rows, in any order.
func (s *Steps) assertLegacyPlanLimits(ctx context.Context, portal, handle string, table *godog.Table) error {
	handle, err := stepscommon.Expand(ctx, handle)
	if err != nil {
		return err
	}
	want, err := legacyLimitsFrom(table)
	if err != nil {
		return err
	}
	database, err := s.openPlanDB(ctx, portal)
	if err != nil {
		return err
	}
	defer func() { _ = database.close() }()
	planID, portalID, err := database.planRow(ctx, handle)
	if err != nil {
		return err
	}
	rows, err := database.db.QueryContext(ctx, database.bind(`SELECT limit_type, time_unit, time_amount, limit_count
		FROM subscription_plan_limits WHERE plan_uuid = ? AND portal_id = ?`), planID, portalID)
	if err != nil {
		return fmt.Errorf("reading the earlier limits of plan %q: %w", handle, err)
	}
	defer func() { _ = rows.Close() }()
	var got []legacyLimit
	for rows.Next() {
		var l legacyLimit
		var unit sql.NullString
		if err := rows.Scan(&l.limitType, &unit, &l.timeAmount, &l.limitCount); err != nil {
			return err
		}
		l.timeUnit = unit.String
		got = append(got, l)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if a, b := sortedLimits(got), sortedLimits(want); a != b {
		return fmt.Errorf("plan %q has earlier limits [%s], want [%s]", handle, a, b)
	}
	return nil
}

func sortedLimits(limits []legacyLimit) string {
	out := make([]string, len(limits))
	for i, l := range limits {
		out[i] = l.String()
	}
	sort.Strings(out)
	return strings.Join(out, "; ")
}

// assertPlanConditionRows counts the plan's rows in subscription_plan_conditions.
func (s *Steps) assertPlanConditionRows(ctx context.Context, portal string, want int, handle string) error {
	handle, err := stepscommon.Expand(ctx, handle)
	if err != nil {
		return err
	}
	database, err := s.openPlanDB(ctx, portal)
	if err != nil {
		return err
	}
	defer func() { _ = database.close() }()
	planID, portalID, err := database.planRow(ctx, handle)
	if err != nil {
		return err
	}
	var got int
	if err := database.db.QueryRowContext(ctx, database.bind(`SELECT COUNT(*) FROM subscription_plan_conditions WHERE plan_uuid = ? AND portal_id = ?`),
		planID, portalID).Scan(&got); err != nil {
		return fmt.Errorf("counting the conditions of plan %q: %w", handle, err)
	}
	if got != want {
		return fmt.Errorf("plan %q has %d condition rows, want %d", handle, got, want)
	}
	return nil
}
