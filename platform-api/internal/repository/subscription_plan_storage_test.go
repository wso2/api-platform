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
	"errors"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

func newPlan(handle string, conditions ...model.SubscriptionPlanCondition) *model.SubscriptionPlan {
	return &model.SubscriptionPlan{Handle: handle, Name: handle, OrganizationUUID: "org-1",
		Status: model.SubscriptionPlanStatusActive, Conditions: conditions}
}

func dropTable(t *testing.T, db *database.DB, table string) {
	t.Helper()
	if _, err := db.Exec(`DROP TABLE ` + table); err != nil {
		t.Fatalf("drop %s: %v", table, err)
	}
}

func wantErrContaining(t *testing.T, what string, err error, text string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), text) {
		t.Errorf("%s: err = %v, want one containing %q", what, err, text)
	}
}

// On a database not yet upgraded, a soft request limit is stored with stop_on_quota_reach 0, and an
// old limit row with no time amount reads as one unit.
func TestSubscriptionPlanRepo_LegacyStorageDetails(t *testing.T) {
	db := newPlanTestDB(t)
	asLegacyDatabase(t, db, false)
	repo := NewSubscriptionPlanRepo(db)

	soft := model.NewRequestCountCondition([]model.ConditionWindow{{Count: 7, TimeAmount: 1, TimeUnit: "DAY"}}, false)
	plan := newPlan("soft", soft)
	if err := repo.Create(plan); err != nil {
		t.Fatalf("Create: %v", err)
	}
	var stop int
	if err := db.QueryRow(`SELECT stop_on_quota_reach FROM subscription_plan_limits WHERE subscription_plan_uuid = ?`, plan.UUID).Scan(&stop); err != nil || stop != 0 {
		t.Fatalf("stop_on_quota_reach = %d, %v; want 0", stop, err)
	}

	insertLegacyPlan(t, db, "p-old", "old", 30, "minute", 1)
	if _, err := db.Exec(`UPDATE subscription_plan_limits SET time_amount = 0 WHERE subscription_plan_uuid = 'p-old'`); err != nil {
		t.Fatal(err)
	}
	old, err := repo.GetByID("p-old", "org-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if want := `{"limits":[{"count":30,"timeAmount":1,"timeUnit":"MINUTE"}],"onExceed":"reject"}`; string(old.Conditions[0].Params) != want {
		t.Fatalf("params = %s, want %s", old.Conditions[0].Params, want)
	}

	broken := newPlan("broken", model.SubscriptionPlanCondition{ID: "requests", Type: model.ConditionTypeRequestCount, Params: []byte(`not json`)})
	wantErrContaining(t, "unreadable request condition", repo.Create(broken), "failed to read the requests condition")

	twice := newPlan("twice", model.NewRequestCountCondition([]model.ConditionWindow{
		{Count: 1, TimeAmount: 1, TimeUnit: "DAY"}, {Count: 2, TimeAmount: 1, TimeUnit: "DAY"}}, true))
	wantErrContaining(t, "same window twice", repo.Create(twice), "failed to insert subscription plan limit")
}

// A database with neither plan conditions nor old limits stores plans with no conditions, and refuses
// a request limit it has nowhere to keep.
func TestSubscriptionPlanRepo_NoConditionStorage(t *testing.T) {
	db := newPlanTestDB(t)
	dropTable(t, db, "subscription_plan_conditions")
	repo := NewSubscriptionPlanRepo(db)

	if err := repo.Create(newPlan("free")); err != nil {
		t.Fatalf("Create with no conditions: %v", err)
	}
	if err := repo.Create(newPlan("gold", requestCount(5, "MINUTE"))); !errors.Is(err, ErrPlanConditionsTableMissing) {
		t.Fatalf("Create with a request limit = %v, want ErrPlanConditionsTableMissing", err)
	}
}

// Storage failures are returned, not ignored. The tables are dropped after the repository has read
// the schema, as if they failed under it.
func TestSubscriptionPlanRepo_StorageFailures(t *testing.T) {
	db := newPlanTestDB(t)
	asLegacyDatabase(t, db, true)
	repo := NewSubscriptionPlanRepo(db)

	gold := newPlan("gold", requestCount(5, "MINUTE"))
	if err := repo.Create(gold); err != nil {
		t.Fatalf("Create: %v", err)
	}
	dup := newPlan("dup", requestCount(1, "MINUTE"), requestCount(2, "HOUR"))
	wantErrContaining(t, "two conditions with one id", repo.Create(dup), "failed to insert subscription plan condition requests")

	insertLegacyPlan(t, db, "p-old", "old", 10, "HOUR", 1)
	dropTable(t, db, "subscription_plan_limits")
	wantErrContaining(t, "update without the limits table", repo.Update(gold), "failed to clear the subscription plan's earlier limit")
	_, err := repo.GetByID("p-old", "org-1")
	wantErrContaining(t, "read without the limits table", err, "failed to read subscription plan limits")

	dropTable(t, db, "subscription_plan_conditions")
	wantErrContaining(t, "create without the conditions table", repo.Create(newPlan("silver")), "failed to clear subscription plan conditions")
	_, err = repo.GetByHandleAndOrg("gold", "org-1")
	wantErrContaining(t, "get", err, "failed to read subscription plan conditions")
	_, err = repo.GetByHandles([]string{"gold"}, "org-1")
	wantErrContaining(t, "get by handles", err, "failed to read subscription plan conditions")
	_, err = repo.ListByOrganization("org-1", ListOptions{Limit: 10}, "")
	wantErrContaining(t, "list", err, "failed to read subscription plan conditions")
}

// A database upgraded while the server runs is picked up: the schema is checked again until the
// conditions table and description column are found.
func TestSubscriptionPlanRepo_SchemaCheckedUntilUpgraded(t *testing.T) {
	db := newPlanTestDB(t)
	asLegacyDatabase(t, db, false)
	repo := NewSubscriptionPlanRepo(db)

	plan := newPlan("gold")
	plan.Description = "For partners"
	if err := repo.Create(plan); !errors.Is(err, ErrPlanDescriptionColumnMissing) {
		t.Fatalf("before the upgrade: err = %v, want ErrPlanDescriptionColumnMissing", err)
	}

	for _, stmt := range []string{
		`ALTER TABLE subscription_plans ADD COLUMN description VARCHAR(1023)`,
		`CREATE TABLE subscription_plan_conditions (uuid VARCHAR(40) PRIMARY KEY, subscription_plan_uuid VARCHAR(40) NOT NULL,
			organization_uuid VARCHAR(40) NOT NULL, condition_id VARCHAR(64) NOT NULL, condition_type VARCHAR(32) NOT NULL,
			params TEXT NOT NULL, sort_order INTEGER NOT NULL DEFAULT 0, UNIQUE(subscription_plan_uuid, condition_id))`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	plan.Conditions = []model.SubscriptionPlanCondition{{ID: "tool-calls", Type: model.ConditionTypeToolCallCount,
		Params: []byte(`{"limits":[{"count":10,"timeAmount":1,"timeUnit":"MINUTE"}],"onExceed":"reject"}`)}}
	if err := repo.Create(plan); err != nil {
		t.Fatalf("after the upgrade: %v", err)
	}
	got, err := repo.GetByHandleAndOrg("gold", "org-1")
	if err != nil || got.Description != "For partners" || len(got.Conditions) != 1 {
		t.Fatalf("gold = %+v, %v", got, err)
	}
}

// An update refuses a description the database can't store yet, and a configuration that can't be
// read or a missing table fails the usage read and the delete.
func TestSubscriptionPlanRepo_UsageAndUpdateFailures(t *testing.T) {
	db := newPlanTestDB(t)
	repo := NewSubscriptionPlanRepo(db)
	plan := newPlan("gold")
	if err := repo.Create(plan); err != nil {
		t.Fatalf("Create: %v", err)
	}
	for _, stmt := range []string{
		`INSERT INTO projects (uuid, handle, display_name, organization_uuid) VALUES ('proj-1', 'default', 'Default', 'org-1')`,
		`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES ('mcp-1', 'Mcp', 'org-1')`,
		`INSERT INTO mcp_proxies (uuid, organization_uuid, handle, display_name, project_uuid, configuration) VALUES
			('mcp-1', 'org-1', 'weather-mcp', 'Weather MCP', 'proj-1', '{')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	_, err := repo.GetUsage(plan.UUID, plan.Handle, "org-1")
	wantErrContaining(t, "unreadable MCP proxy configuration", err, "failed to decode the configuration of MCP proxy weather-mcp")
	_, err = repo.DeleteWithReferences(plan.UUID, plan.Handle, "org-1")
	wantErrContaining(t, "delete with an unreadable MCP proxy configuration", err, "failed to remove the plan from MCP proxy mcp-1")

	dropTable(t, db, "mcp_proxies")
	_, err = repo.GetUsage(plan.UUID, plan.Handle, "org-1")
	wantErrContaining(t, "usage without the MCP proxy table", err, "failed to list MCP proxies")
	_, err = repo.DeleteWithReferences(plan.UUID, plan.Handle, "org-1")
	wantErrContaining(t, "delete without the MCP proxy table", err, "failed to list MCP proxies")

	if _, err := db.Exec(`ALTER TABLE subscription_plans DROP COLUMN description`); err != nil {
		t.Fatal(err)
	}
	stale := NewSubscriptionPlanRepo(db)
	plan.Description = "For partners"
	if err := stale.Update(plan); !errors.Is(err, ErrPlanDescriptionColumnMissing) {
		t.Errorf("update with a description = %v, want ErrPlanDescriptionColumnMissing", err)
	}
}
