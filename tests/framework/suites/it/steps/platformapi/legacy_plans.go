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

package platformapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/components"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// RegisterLegacyPlans binds the steps that store a subscription plan in the control plane's
// database as a release before plan conditions stored it, and count the rows a plan is stored in.
func RegisterLegacyPlans(sc *godog.ScenarioContext, s *Steps) {
	sc.Step(`^the control plane database holds a subscription plan "([^"]*)" of project "([^"]*)"'s organization as a release before plan conditions stored it, with a request limit of (\d+) per "(MINUTE|HOUR|DAY|MONTH)" that (rejects|allows) requests over it$`,
		s.seedLegacyPlan)
	sc.Step(`^the control plane database should hold (\d+) (earlier limit|condition) rows? for subscription plan "([^"]*)"$`,
		s.assertPlanRows)
}

// legacyLimitsDDL creates subscription_plan_limits as the releases before plan conditions did. This
// release no longer creates it, so a database upgraded from one of those releases is the only kind
// that has it.
var legacyLimitsDDL = map[components.DBType]string{
	components.Postgres: `CREATE TABLE IF NOT EXISTS subscription_plan_limits (
		uuid VARCHAR(40) PRIMARY KEY,
		subscription_plan_uuid VARCHAR(40) NOT NULL,
		limit_type VARCHAR(20) NOT NULL DEFAULT 'REQUEST_COUNT',
		time_unit VARCHAR(20) NOT NULL,
		time_amount INTEGER NOT NULL DEFAULT 1,
		limit_count BIGINT NOT NULL,
		limit_count_unit VARCHAR(10),
		stop_on_quota_reach SMALLINT NOT NULL DEFAULT 1,
		FOREIGN KEY (subscription_plan_uuid) REFERENCES subscription_plans(uuid) ON DELETE CASCADE,
		UNIQUE(subscription_plan_uuid, limit_type, time_amount, time_unit))`,
	components.SQLServer: `IF OBJECT_ID(N'dbo.subscription_plan_limits', N'U') IS NULL
	CREATE TABLE dbo.subscription_plan_limits (
		uuid VARCHAR(40) PRIMARY KEY,
		subscription_plan_uuid VARCHAR(40) NOT NULL,
		limit_type VARCHAR(20) NOT NULL DEFAULT 'REQUEST_COUNT',
		time_unit VARCHAR(20) NOT NULL,
		time_amount INT NOT NULL DEFAULT 1,
		limit_count BIGINT NOT NULL,
		limit_count_unit VARCHAR(10),
		stop_on_quota_reach SMALLINT NOT NULL DEFAULT 1,
		FOREIGN KEY (subscription_plan_uuid) REFERENCES subscription_plans(uuid) ON DELETE CASCADE,
		UNIQUE(subscription_plan_uuid, limit_type, time_amount, time_unit))`,
}

// controlPlaneComponent is the component that owns the control plane database.
const controlPlaneComponent = "platform-api"

// controlPlaneDB is a direct connection to the control plane's database server.
type controlPlaneDB struct {
	db     *sql.DB
	close  func() error
	engine components.DBType
}

// openControlPlaneDB connects to platform-api's database. Only a database server can be written
// to: an embedded SQLite store is read from a copy of its file.
func (s *Steps) openControlPlaneDB(ctx context.Context) (*controlPlaneDB, error) {
	if s.topo == nil || s.topo.Storage == nil || s.topo.Storage.Plan == nil {
		return nil, errors.New("this block has no storage plan")
	}
	store, ok := s.topo.Storage.Plan.StoreFor(controlPlaneComponent, 0)
	if !ok {
		return nil, errors.New("platform-api has no database in this block")
	}
	if store.Embedded {
		return nil, fmt.Errorf("platform-api uses an embedded %s store, which a step cannot write to; "+
			"run this scenario in a block whose platform-api database is postgres or sqlserver", store.Type)
	}
	db, closeDB, err := s.topo.OpenComponentDB(ctx, controlPlaneComponent, "")
	if err != nil {
		return nil, err
	}
	return &controlPlaneDB{db: db, close: closeDB, engine: store.Type}, nil
}

// bind rewrites ?-placeholders into the engine's own form.
func (c *controlPlaneDB) bind(query string) string {
	var marker func(int) string
	switch c.engine {
	case components.Postgres:
		marker = func(n int) string { return "$" + strconv.Itoa(n) }
	case components.SQLServer:
		marker = func(n int) string { return "@p" + strconv.Itoa(n) }
	default:
		return query
	}
	var out strings.Builder
	n := 0
	for _, r := range query {
		if r == '?' {
			n++
			out.WriteString(marker(n))
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

// seedLegacyPlan writes a plan as the releases before plan conditions did: a subscription_plans
// row and one REQUEST_COUNT row in subscription_plan_limits, with the window's amount of 1. The
// control plane reads the table only after a restart, as after an upgrade.
func (s *Steps) seedLegacyPlan(ctx context.Context, handle, project string, count int, unit, overLimit string) error {
	handle, err := stepscommon.Expand(ctx, handle)
	if err != nil {
		return err
	}
	project, err = stepscommon.Expand(ctx, project)
	if err != nil {
		return err
	}
	conn, err := s.openControlPlaneDB(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.close() }()

	ddl, ok := legacyLimitsDDL[conn.engine]
	if !ok {
		return fmt.Errorf("no earlier subscription_plan_limits definition for %s", conn.engine)
	}
	if _, err := conn.db.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("creating the earlier subscription_plan_limits table: %w", err)
	}
	var org string
	if err := conn.db.QueryRowContext(ctx, conn.bind(`SELECT organization_uuid FROM projects WHERE handle = ?`), project).Scan(&org); err != nil {
		return fmt.Errorf("finding the organization of project %q: %w", project, err)
	}
	planUUID, err := unique.Unique(ctx, "legacy-plan")
	if err != nil {
		return err
	}
	limitUUID, err := unique.Unique(ctx, "legacy-limit")
	if err != nil {
		return err
	}
	stop := 1
	if overLimit == "allows" {
		stop = 0
	}
	tx, err := conn.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, conn.bind(`INSERT INTO subscription_plans (uuid, handle, display_name, organization_uuid, status)
		VALUES (?, ?, ?, ?, 'ACTIVE')`), planUUID, handle, handle, org); err != nil {
		return fmt.Errorf("storing plan %q: %w", handle, err)
	}
	if _, err := tx.ExecContext(ctx, conn.bind(`INSERT INTO subscription_plan_limits (uuid, subscription_plan_uuid, limit_type,
		time_unit, time_amount, limit_count, stop_on_quota_reach) VALUES (?, ?, 'REQUEST_COUNT', ?, 1, ?, ?)`),
		limitUUID, planUUID, unit, count, stop); err != nil {
		return fmt.Errorf("storing the request limit of plan %q: %w", handle, err)
	}
	return tx.Commit()
}

// assertPlanRows counts the plan's rows in subscription_plan_limits (earlier limit) or
// subscription_plan_conditions (condition).
func (s *Steps) assertPlanRows(ctx context.Context, want int, kind, handle string) error {
	handle, err := stepscommon.Expand(ctx, handle)
	if err != nil {
		return err
	}
	table := "subscription_plan_conditions"
	if kind == "earlier limit" {
		table = "subscription_plan_limits"
	}
	conn, err := s.openControlPlaneDB(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.close() }()
	var got int
	if err := conn.db.QueryRowContext(ctx, conn.bind(`SELECT COUNT(*) FROM `+table+` r
		JOIN subscription_plans p ON p.uuid = r.subscription_plan_uuid WHERE p.handle = ?`), handle).Scan(&got); err != nil {
		return fmt.Errorf("counting %s rows of plan %q: %w", table, handle, err)
	}
	if got != want {
		return fmt.Errorf("plan %q has %d %s rows, want %d", handle, got, table, want)
	}
	return nil
}
