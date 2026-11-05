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
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

func TestSubscriptionPlanRepo_ListAndCountSearch(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	if err := NewOrganizationRepo(db).CreateOrganization(&model.Organization{
		ID: "org-1", Handle: "acme", Name: "Acme", Region: "us",
	}); err != nil {
		t.Fatalf("CreateOrganization: %v", err)
	}
	repo := NewSubscriptionPlanRepo(db)
	for _, p := range []struct {
		handle, name string
		status       model.SubscriptionPlanStatus
	}{
		{"gold", "Gold Plan", model.SubscriptionPlanStatusActive}, {"silver", "Silver Plan", model.SubscriptionPlanStatusActive},
		{"bronze", "Bronze 100%", model.SubscriptionPlanStatusInactive},
	} {
		if err := repo.Create(&model.SubscriptionPlan{
			Handle: p.handle, Name: p.name, OrganizationUUID: "org-1", Status: p.status,
		}); err != nil {
			t.Fatalf("Create %s: %v", p.handle, err)
		}
	}

	tests := []struct {
		name      string
		opts      ListOptions
		status    string
		wantTotal int
		wantPage  int
	}{
		{"no search", ListOptions{Limit: 10}, "", 3, 3},
		{"page smaller than total", ListOptions{Limit: 2}, "", 3, 2},
		{"second page", ListOptions{Limit: 2, Offset: 2}, "", 3, 1},
		{"matches name case-insensitively", ListOptions{Limit: 10, Search: "GOLD"}, "", 1, 1},
		{"matches handle", ListOptions{Limit: 10, Search: "silv"}, "", 1, 1},
		{"percent is literal", ListOptions{Limit: 10, Search: "%"}, "", 1, 1},
		{"no match", ListOptions{Limit: 10, Search: "platinum"}, "", 0, 0},
		{"active only", ListOptions{Limit: 10}, "ACTIVE", 2, 2},
		{"inactive only", ListOptions{Limit: 10}, "INACTIVE", 1, 1},
		{"active and searched", ListOptions{Limit: 10, Search: "plan"}, "ACTIVE", 2, 2},
		{"active page", ListOptions{Limit: 1, Offset: 1}, "ACTIVE", 2, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plans, err := repo.ListByOrganization("org-1", tt.opts, tt.status)
			if err != nil {
				t.Fatalf("ListByOrganization: %v", err)
			}
			if len(plans) != tt.wantPage {
				t.Errorf("page size = %d, want %d", len(plans), tt.wantPage)
			}
			total, err := repo.CountByOrganization("org-1", tt.opts.Search, tt.status)
			if err != nil {
				t.Fatalf("CountByOrganization: %v", err)
			}
			if total != tt.wantTotal {
				t.Errorf("total = %d, want %d", total, tt.wantTotal)
			}
		})
	}
}

func TestSubscriptionPlanRepo_GetByHandles(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	orgRepo := NewOrganizationRepo(db)
	for _, o := range []*model.Organization{
		{ID: "org-1", Handle: "acme", Name: "Acme", Region: "us"},
		{ID: "org-2", Handle: "globex", Name: "Globex", Region: "us"},
	} {
		if err := orgRepo.CreateOrganization(o); err != nil {
			t.Fatalf("CreateOrganization %s: %v", o.ID, err)
		}
	}
	repo := NewSubscriptionPlanRepo(db)
	for _, p := range []struct {
		handle, org string
		status      model.SubscriptionPlanStatus
	}{
		{"gold", "org-1", model.SubscriptionPlanStatusActive},
		{"silver", "org-1", model.SubscriptionPlanStatusInactive},
		{"gold", "org-2", model.SubscriptionPlanStatusActive},
		{"platinum", "org-2", model.SubscriptionPlanStatusActive},
	} {
		if err := repo.Create(&model.SubscriptionPlan{
			Handle: p.handle, Name: p.handle, OrganizationUUID: p.org, Status: p.status,
		}); err != nil {
			t.Fatalf("Create %s/%s: %v", p.org, p.handle, err)
		}
	}

	t.Run("returns requested plans keyed by handle with their status", func(t *testing.T) {
		plans, err := repo.GetByHandles([]string{"gold", "silver"}, "org-1")
		if err != nil {
			t.Fatalf("GetByHandles: %v", err)
		}
		if len(plans) != 2 {
			t.Fatalf("got %d plans, want 2", len(plans))
		}
		if plans["gold"].Status != model.SubscriptionPlanStatusActive || plans["silver"].Status != model.SubscriptionPlanStatusInactive {
			t.Errorf("unexpected statuses: gold=%s silver=%s", plans["gold"].Status, plans["silver"].Status)
		}
		if plans["gold"].OrganizationUUID != "org-1" {
			t.Errorf("gold org = %s, want org-1", plans["gold"].OrganizationUUID)
		}
	})

	t.Run("omits an unknown handle", func(t *testing.T) {
		plans, err := repo.GetByHandles([]string{"gold", "missing"}, "org-1")
		if err != nil {
			t.Fatalf("GetByHandles: %v", err)
		}
		if _, ok := plans["missing"]; ok || len(plans) != 1 {
			t.Errorf("want only gold, got %v", plans)
		}
	})

	t.Run("does not return another organization's plan", func(t *testing.T) {
		plans, err := repo.GetByHandles([]string{"platinum"}, "org-1")
		if err != nil {
			t.Fatalf("GetByHandles: %v", err)
		}
		if len(plans) != 0 {
			t.Errorf("want no plans, got %v", plans)
		}
	})

	t.Run("empty input returns an empty map", func(t *testing.T) {
		plans, err := repo.GetByHandles(nil, "org-1")
		if err != nil || plans == nil || len(plans) != 0 {
			t.Errorf("want an empty non-nil map, got %v (%v)", plans, err)
		}
	})

	t.Run("an unreadable row is an error, not a partial result", func(t *testing.T) {
		var createdAt time.Time
		if err := db.QueryRow(`SELECT created_at FROM subscription_plans WHERE handle = 'silver' AND organization_uuid = 'org-1'`).Scan(&createdAt); err != nil {
			t.Fatalf("read the row: %v", err)
		}
		if _, err := db.Exec(db.Rebind(`UPDATE subscription_plans SET created_at = NULL WHERE handle = ? AND organization_uuid = ?`),
			"silver", "org-1"); err != nil {
			t.Fatalf("corrupt the row: %v", err)
		}
		defer func() {
			if _, err := db.Exec(db.Rebind("UPDATE subscription_plans SET created_at = ? WHERE handle = ?"), createdAt, "silver"); err != nil {
				t.Fatalf("restore the row: %v", err)
			}
		}()
		if _, err := repo.GetByHandles([]string{"gold", "silver"}, "org-1"); err == nil {
			t.Error("want a scan error")
		}
	})

	t.Run("database error is wrapped", func(t *testing.T) {
		broken, brokenCleanup := setupTestDB(t)
		brokenCleanup()
		if _, err := NewSubscriptionPlanRepo(broken).GetByHandles([]string{"gold"}, "org-1"); err == nil {
			t.Error("want an error from a closed database")
		}
	})
}

// seedPlanUsers creates the things that offer or use the plans gold and silver:
//   - the REST API shop-api, listing both, with a publication draft for the portal east offering gold;
//   - the REST API other-api, listing silver;
//   - an MCP proxy whose live publication offers gold;
//   - two subscriptions on gold;
//   - gold in artifact_subscription_plans.
func seedPlanUsers(t *testing.T, db *database.DB, gold, silver *model.SubscriptionPlan) {
	t.Helper()
	for _, stmt := range []string{
		`INSERT INTO projects (uuid, handle, display_name, organization_uuid) VALUES ('proj-1', 'default', 'Default', 'org-1')`,
		`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES ('api-1', 'RestApi', 'org-1'), ('api-2', 'RestApi', 'org-1'),
			('mcp-1', 'Mcp', 'org-1')`,
		`INSERT INTO rest_apis (uuid, organization_uuid, handle, display_name, version, project_uuid, configuration) VALUES
			('api-1', 'org-1', 'shop-api', 'Shop API', 'v1', 'proj-1', '{"name":"Shop","subscriptionPlans":["gold","silver"]}'),
			('api-2', 'org-1', 'other-api', 'Other API', 'v1', 'proj-1', '{"name":"Other","subscriptionPlans":["silver"]}')`,
		`INSERT INTO mcp_proxies (uuid, organization_uuid, handle, display_name, project_uuid, configuration) VALUES
			('mcp-1', 'org-1', 'weather-mcp', 'Weather MCP', 'proj-1', '{"name":"Weather","subscriptionPlans":["gold"]}')`,
		`INSERT INTO api_portals (uuid, organization_uuid, handle, display_name, internal_auth_key) VALUES
			('portal-1', 'org-1', 'east', 'East', X'00')`,
		`INSERT INTO api_publications (uuid, organization_uuid, artifact_uuid, api_portal_uuid, is_draft, status, display_name,
			version, created_by, updated_by) VALUES
			('pub-1', 'org-1', 'api-1', 'portal-1', 1, NULL, 'Shop API', 'v1', 'u', 'u'),
			('pub-2', 'org-1', 'mcp-1', 'portal-1', 0, 'PUBLISHED', 'Weather MCP', 'v1', 'u', 'u'),
			('pub-3', 'org-1', 'api-2', 'portal-1', 1, NULL, 'Other API', 'v1', 'u', 'u')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}
	for _, row := range [][]any{
		{"pub-1", gold.UUID, gold.Handle, gold.Name}, {"pub-1", silver.UUID, silver.Handle, silver.Name},
		{"pub-2", gold.UUID, gold.Handle, gold.Name}, {"pub-3", silver.UUID, silver.Handle, silver.Name},
	} {
		if _, err := db.Exec(`INSERT INTO api_publication_plan_mappings (organization_uuid, publication_uuid, subscription_plan_uuid,
			plan_handle, display_name, created_by) VALUES ('org-1', ?, ?, ?, ?, 'u')`, row...); err != nil {
			t.Fatalf("seed mapping: %v", err)
		}
	}
	for i, plan := range []string{gold.UUID, gold.UUID, silver.UUID} {
		if _, err := db.Exec(`INSERT INTO subscriptions (uuid, artifact_uuid, subscriber_id, subscription_token,
			subscription_token_hash, subscription_plan_uuid, organization_uuid) VALUES (?, 'api-1', 'user', 'token', ?, ?, 'org-1')`,
			fmt.Sprintf("sub-%d", i), fmt.Sprintf("hash-%d", i), plan); err != nil {
			t.Fatalf("seed subscription: %v", err)
		}
	}
	if _, err := db.Exec(`INSERT INTO artifact_subscription_plans (artifact_uuid, subscription_plan_uuid) VALUES ('api-1', ?)`, gold.UUID); err != nil {
		t.Fatalf("seed artifact plan: %v", err)
	}
}

func TestSubscriptionPlanRepo_UsageAndDeleteWithReferences(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	if err := NewOrganizationRepo(db).CreateOrganization(&model.Organization{ID: "org-1", Handle: "acme", Name: "Acme", Region: "us"}); err != nil {
		t.Fatalf("CreateOrganization: %v", err)
	}
	repo := NewSubscriptionPlanRepo(db)
	fee := "9.00"
	gold := &model.SubscriptionPlan{Handle: "gold", Name: "Gold", OrganizationUUID: "org-1", Status: model.SubscriptionPlanStatusActive,
		Conditions: []model.SubscriptionPlanCondition{requestCount(100, "MINUTE")}}
	pricing := &model.SubscriptionPlanPricing{PricingType: model.PricingTypeFlatRate, Currency: "USD", BillingPeriod: "MONTH",
		FlatAmount: &fee, CreatedAt: time.Now().UTC()}
	if err := repo.CreateWithPricing(gold, pricing); err != nil {
		t.Fatalf("CreateWithPricing: %v", err)
	}
	silver := &model.SubscriptionPlan{Handle: "silver", Name: "Silver", OrganizationUUID: "org-1", Status: model.SubscriptionPlanStatusActive}
	if err := repo.Create(silver); err != nil {
		t.Fatalf("Create silver: %v", err)
	}
	if stored, err := NewSubscriptionPlanPricingRepo(db).GetByPlan(gold.UUID, "org-1"); err != nil || stored == nil || *stored.FlatAmount != "9.00" {
		t.Fatalf("the pricing is stored with the plan: %+v, %v", stored, err)
	}
	seedPlanUsers(t, db, gold, silver)

	usage, err := repo.GetUsage(gold.UUID, gold.Handle, "org-1")
	if err != nil {
		t.Fatalf("GetUsage: %v", err)
	}
	want := &model.SubscriptionPlanUsage{RESTAPIs: 1, MCPProxies: 1, Drafts: 1, Publications: 1, Subscriptions: 2}
	if !reflect.DeepEqual(usage, want) {
		t.Fatalf("usage = %+v, want %+v", usage, want)
	}

	deleted, err := repo.DeleteWithReferences(gold.UUID, gold.Handle, "org-1")
	if err != nil || !deleted {
		t.Fatalf("DeleteWithReferences = %v, %v", deleted, err)
	}
	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		if err := db.QueryRow(query, args...).Scan(&n); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return n
	}
	for query, want := range map[string]int{
		`SELECT COUNT(*) FROM subscription_plans WHERE handle = 'gold'`:                                           0,
		`SELECT COUNT(*) FROM subscription_plan_conditions WHERE subscription_plan_uuid = '` + gold.UUID + `'`:    0,
		`SELECT COUNT(*) FROM subscription_plan_pricing WHERE subscription_plan_uuid = '` + gold.UUID + `'`:       0,
		`SELECT COUNT(*) FROM api_publication_plan_mappings WHERE subscription_plan_uuid = '` + gold.UUID + `'`:   2,
		`SELECT COUNT(*) FROM api_publication_plan_mappings WHERE subscription_plan_uuid = '` + silver.UUID + `'`: 2,
		`SELECT COUNT(*) FROM artifact_subscription_plans`:                                                        0,
		`SELECT COUNT(*) FROM subscriptions WHERE subscription_plan_uuid IS NULL`:                                 2,
		`SELECT COUNT(*) FROM subscriptions WHERE subscription_plan_uuid = '` + silver.UUID + `'`:                 1,
	} {
		if got := count(query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}
	// The plan leaves the REST APIs' configurations; everything else in them is kept.
	for handle, wantConfig := range map[string]string{
		"shop-api":  `{"name":"Shop","subscriptionPlans":["silver"]}`,
		"other-api": `{"name":"Other","subscriptionPlans":["silver"]}`,
	} {
		var configuration []byte
		if err := db.QueryRow(`SELECT configuration FROM rest_apis WHERE handle = ?`, handle).Scan(&configuration); err != nil {
			t.Fatalf("read %s: %v", handle, err)
		}
		if string(configuration) != wantConfig {
			t.Errorf("%s configuration = %s, want %s", handle, configuration, wantConfig)
		}
	}

	// It leaves the MCP proxy's configuration too.
	var mcpConfiguration []byte
	if err := db.QueryRow(`SELECT configuration FROM mcp_proxies WHERE handle = 'weather-mcp'`).Scan(&mcpConfiguration); err != nil ||
		string(mcpConfiguration) != `{"name":"Weather"}` {
		t.Errorf("weather-mcp configuration = %s, %v; want the plan removed", mcpConfiguration, err)
	}

	if deleted, err := repo.DeleteWithReferences(gold.UUID, gold.Handle, "org-1"); err != nil || deleted {
		t.Fatalf("a plan already gone: deleted = %v, err = %v", deleted, err)
	}
	usage, err = repo.GetUsage(silver.UUID, silver.Handle, "org-1")
	if err != nil {
		t.Fatalf("GetUsage silver: %v", err)
	}
	want = &model.SubscriptionPlanUsage{RESTAPIs: 2, Drafts: 2, Publications: 0, Subscriptions: 1}
	if !reflect.DeepEqual(usage, want) {
		t.Fatalf("silver usage = %+v, want %+v", usage, want)
	}
}

// A REST API with an empty configuration lists no plans, so it neither counts toward a plan's usage
// nor stops the plan from being removed from the other REST APIs.
func TestRestAPIPlanHandles_EmptyConfigurationListsNone(t *testing.T) {
	for _, configuration := range [][]byte{nil, []byte(""), []byte(" \n\t")} {
		if plans, err := restAPIPlanHandles(configuration); err != nil || plans != nil {
			t.Errorf("restAPIPlanHandles(%q) = %v, %v; want none", configuration, plans, err)
		}
		if got, err := withoutPlanHandle(configuration, "gold"); err != nil || got != nil {
			t.Errorf("withoutPlanHandle(%q) = %s, %v; want no rewrite", configuration, got, err)
		}
	}
	if _, err := restAPIPlanHandles([]byte("{")); err == nil {
		t.Error("want a configuration that is not JSON to be refused as before")
	}
}

func TestWithoutPlanHandle_KeepsTheOtherFields(t *testing.T) {
	got, err := withoutPlanHandle([]byte(`{"name":"Shop","subscriptionPlans":["gold"],"upstream":{"main":{"url":"https://a"}}}`), "gold")
	if err != nil || string(got) != `{"name":"Shop","upstream":{"main":{"url":"https://a"}}}` {
		t.Fatalf("got %s, %v", got, err)
	}
	if got, err := withoutPlanHandle([]byte(`{"name":"Shop","subscriptionPlans":["silver"]}`), "gold"); err != nil || got != nil {
		t.Fatalf("a configuration without the plan is not rewritten, got %s, %v", got, err)
	}
}

// A CreatedAt the caller set (a copy of an API Portal plan) is kept; without one the plan is dated
// now. UpdatedAt is always now.
func TestSubscriptionPlanRepo_CreateKeepsTheCallersCreatedAt(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	if err := NewOrganizationRepo(db).CreateOrganization(&model.Organization{ID: "org-1", Handle: "acme", Name: "Acme", Region: "us"}); err != nil {
		t.Fatalf("CreateOrganization: %v", err)
	}
	repo := NewSubscriptionPlanRepo(db)
	madeInPortal := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	before := time.Now().UTC().Add(-time.Second)
	copied := &model.SubscriptionPlan{Handle: "gold", Name: "Gold", OrganizationUUID: "org-1", Status: model.SubscriptionPlanStatusActive,
		CreatedAt: madeInPortal}
	if err := repo.Create(copied); err != nil {
		t.Fatalf("Create copy: %v", err)
	}
	if err := repo.Create(&model.SubscriptionPlan{Handle: "silver", Name: "Silver", OrganizationUUID: "org-1",
		Status: model.SubscriptionPlanStatusActive}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	gold, err := repo.GetByHandleAndOrg("gold", "org-1")
	if err != nil || !gold.CreatedAt.Equal(madeInPortal) || gold.UpdatedAt.Before(before) {
		t.Fatalf("copy = %+v, %v; want createdAt %v and updatedAt now", gold, err, madeInPortal)
	}
	silver, err := repo.GetByHandleAndOrg("silver", "org-1")
	if err != nil || silver.CreatedAt.Before(before) {
		t.Fatalf("plan = %+v, %v; want createdAt now", silver, err)
	}
}

// requestCount is a request-count condition of one window of count requests per unit.
func requestCount(count int, unit string) model.SubscriptionPlanCondition {
	return model.NewRequestCountCondition([]model.ConditionWindow{{Count: count, TimeAmount: 1, TimeUnit: unit}}, true)
}

// newPlanTestDB is a database with the organization org-1.
func newPlanTestDB(t *testing.T) *database.DB {
	t.Helper()
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	if err := NewOrganizationRepo(db).CreateOrganization(&model.Organization{ID: "org-1", Handle: "acme", Name: "Acme", Region: "us"}); err != nil {
		t.Fatalf("CreateOrganization: %v", err)
	}
	return db
}

// asLegacyDatabase turns the test database into one created by a release before conditions: it has
// subscription_plan_limits, as those releases created it. withConditions false also takes away what
// this release adds (subscription_plan_conditions and subscription_plans.description), as on a server
// database whose upgrade script has not run.
func asLegacyDatabase(t *testing.T, db *database.DB, withConditions bool) {
	t.Helper()
	stmts := []string{`CREATE TABLE subscription_plan_limits (
		uuid VARCHAR(40) PRIMARY KEY,
		subscription_plan_uuid VARCHAR(40) NOT NULL,
		limit_type VARCHAR(20) NOT NULL DEFAULT 'REQUEST_COUNT',
		time_unit VARCHAR(20) NOT NULL,
		time_amount INTEGER NOT NULL DEFAULT 1,
		limit_count BIGINT NOT NULL,
		limit_count_unit VARCHAR(10),
		stop_on_quota_reach SMALLINT NOT NULL DEFAULT 1,
		FOREIGN KEY (subscription_plan_uuid) REFERENCES subscription_plans(uuid) ON DELETE CASCADE,
		UNIQUE(subscription_plan_uuid, limit_type, time_amount, time_unit))`}
	if !withConditions {
		stmts = append(stmts, `DROP TABLE subscription_plan_conditions`, `ALTER TABLE subscription_plans DROP COLUMN description`)
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
}

// insertLegacyPlan stores a plan as a release before conditions did: the plan row and, when count is
// not 0, its request limit row in subscription_plan_limits.
func insertLegacyPlan(t *testing.T, db *database.DB, uuid, handle string, count int, unit string, stopOnQuotaReach int) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO subscription_plans (uuid, handle, display_name, organization_uuid, status) VALUES (?, ?, ?, 'org-1', 'ACTIVE')`,
		uuid, handle, handle); err != nil {
		t.Fatalf("insert plan: %v", err)
	}
	if count == 0 {
		return
	}
	if _, err := db.Exec(`INSERT INTO subscription_plan_limits (uuid, subscription_plan_uuid, limit_type, time_unit, time_amount, limit_count,
		stop_on_quota_reach) VALUES (?, ?, 'REQUEST_COUNT', ?, 1, ?, ?)`, "limit-"+uuid, uuid, unit, count, stopOnQuotaReach); err != nil {
		t.Fatalf("insert limit: %v", err)
	}
}

func countRows(t *testing.T, db *database.DB, table, planUUID string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE subscription_plan_uuid = ?`, planUUID).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

// A plan's description and conditions are stored and read back in order; an update replaces them,
// and the legacy request limit follows the request-count condition.
func TestSubscriptionPlanRepo_ConditionsRoundTrip(t *testing.T) {
	db := newPlanTestDB(t)
	repo := NewSubscriptionPlanRepo(db)
	toolCalls := model.SubscriptionPlanCondition{ID: "tool-calls", Type: model.ConditionTypeToolCallCount,
		Params: []byte(`{"limits":[{"count":10000,"timeAmount":1,"timeUnit":"DAY"}],"onExceed":"reject"}`)}
	custom := model.SubscriptionPlanCondition{ID: "ip-ranges", Type: model.ConditionTypeCustom, Params: []byte(`{"cidr":"10.0.0.0/8","limit":50}`)}
	plan := &model.SubscriptionPlan{Handle: "gold", Name: "Gold", Description: "For partners", OrganizationUUID: "org-1",
		Status:     model.SubscriptionPlanStatusActive,
		Conditions: []model.SubscriptionPlanCondition{toolCalls, requestCount(100, "MINUTE"), custom}}
	if err := repo.Create(plan); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByHandleAndOrg("gold", "org-1")
	if err != nil {
		t.Fatalf("GetByHandleAndOrg: %v", err)
	}
	if got.Description != "For partners" || got.LegacyLimits {
		t.Errorf("description = %q, legacy = %v", got.Description, got.LegacyLimits)
	}
	if len(got.Conditions) != 3 || got.Conditions[0].ID != "tool-calls" || got.Conditions[1].ID != "requests" || got.Conditions[2].ID != "ip-ranges" ||
		string(got.Conditions[2].Params) != `{"cidr":"10.0.0.0/8","limit":50}` {
		t.Fatalf("conditions = %+v, want tool-calls, requests, ip-ranges in that order", got.Conditions)
	}
	if got.ThrottleLimitCount == nil || *got.ThrottleLimitCount != 100 || got.ThrottleLimitUnit != "MINUTE" || !got.StopOnQuotaReach {
		t.Errorf("legacy limit = %v %q %v, want 100 MINUTE blocking", got.ThrottleLimitCount, got.ThrottleLimitUnit, got.StopOnQuotaReach)
	}

	got.Conditions = []model.SubscriptionPlanCondition{custom}
	got.Description = ""
	if err := repo.Update(got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	listed, err := repo.ListByOrganization("org-1", ListOptions{Limit: 10}, "")
	if err != nil || len(listed) != 1 {
		t.Fatalf("ListByOrganization = %v, %v", listed, err)
	}
	if len(listed[0].Conditions) != 1 || listed[0].Conditions[0].ID != "ip-ranges" || listed[0].Description != "" {
		t.Fatalf("after update: conditions = %+v, description = %q", listed[0].Conditions, listed[0].Description)
	}
	if listed[0].ThrottleLimitCount != nil {
		t.Errorf("a plan with no request-count condition has no legacy limit, got %d", *listed[0].ThrottleLimitCount)
	}
}

// On a database upgraded from a release before conditions, a plan with no condition rows is read
// from its subscription_plan_limits row; saving it stores its conditions and removes that row, so a
// plan later saved with no conditions is not read as the old limit again.
func TestSubscriptionPlanRepo_ReadsLegacyLimits(t *testing.T) {
	db := newPlanTestDB(t)
	asLegacyDatabase(t, db, true)
	insertLegacyPlan(t, db, "p-gold", "gold", 1000, "HOUR", 0)
	insertLegacyPlan(t, db, "p-free", "free", 0, "", 1)
	repo := NewSubscriptionPlanRepo(db)

	plans, err := repo.GetByHandles([]string{"gold", "free"}, "org-1")
	if err != nil {
		t.Fatalf("GetByHandles: %v", err)
	}
	gold := plans["gold"]
	if !gold.LegacyLimits || len(gold.Conditions) != 1 || gold.Conditions[0].ID != "requests" ||
		string(gold.Conditions[0].Params) != `{"limits":[{"count":1000,"timeAmount":1,"timeUnit":"HOUR"}],"onExceed":"allow"}` {
		t.Fatalf("gold = %+v, want its legacy limit as a soft request-count condition", gold)
	}
	if gold.ThrottleLimitCount == nil || *gold.ThrottleLimitCount != 1000 || gold.StopOnQuotaReach {
		t.Errorf("legacy limit = %v, stop = %v", gold.ThrottleLimitCount, gold.StopOnQuotaReach)
	}
	if free := plans["free"]; free.LegacyLimits || len(free.Conditions) != 0 || free.ThrottleLimitCount != nil {
		t.Errorf("free = %+v, want no conditions", free)
	}

	gold.Conditions = nil
	if err := repo.Update(gold); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if n := countRows(t, db, "subscription_plan_limits", "p-gold"); n != 0 {
		t.Fatalf("the old limit row is kept after saving: %d rows", n)
	}
	got, err := repo.GetByID("p-gold", "org-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.LegacyLimits || len(got.Conditions) != 0 {
		t.Fatalf("a plan saved with no conditions reads as %+v", got.Conditions)
	}

	got.Conditions = []model.SubscriptionPlanCondition{requestCount(5, "MINUTE")}
	if err := repo.Update(got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if n := countRows(t, db, "subscription_plan_conditions", "p-gold"); n != 1 {
		t.Fatalf("conditions stored = %d, want 1", n)
	}
}

// On a server database whose upgrade has not run (no subscription_plan_conditions table, no
// description column), plans are read and stored as before: a request limit goes to
// subscription_plan_limits, and other conditions are refused.
func TestSubscriptionPlanRepo_DatabaseNotUpgraded(t *testing.T) {
	db := newPlanTestDB(t)
	asLegacyDatabase(t, db, false)
	insertLegacyPlan(t, db, "p-gold", "gold", 1000, "HOUR", 1)
	repo := NewSubscriptionPlanRepo(db)

	gold, err := repo.GetByHandleAndOrg("gold", "org-1")
	if err != nil || !gold.LegacyLimits || gold.ThrottleLimitCount == nil || *gold.ThrottleLimitCount != 1000 {
		t.Fatalf("gold = %+v, %v", gold, err)
	}

	silver := &model.SubscriptionPlan{Handle: "silver", Name: "Silver", Description: "not stored", OrganizationUUID: "org-1",
		Status: model.SubscriptionPlanStatusActive, Conditions: []model.SubscriptionPlanCondition{requestCount(20, "MINUTE")}}
	if err := repo.Create(silver); !errors.Is(err, ErrPlanDescriptionColumnMissing) {
		t.Fatalf("Create with a description = %v, want ErrPlanDescriptionColumnMissing", err)
	}
	// A plan from an API Portal is saved without its description instead.
	silver.DescriptionOptional = true
	if err := repo.Create(silver); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if silver.Description != "" {
		t.Errorf("description = %q, want it empty: the database can't store it yet", silver.Description)
	}
	if n := countRows(t, db, "subscription_plan_limits", silver.UUID); n != 1 {
		t.Fatalf("limit rows = %d, want the request limit stored the old way", n)
	}
	got, err := repo.GetByHandleAndOrg("silver", "org-1")
	if err != nil || got.ThrottleLimitCount == nil || *got.ThrottleLimitCount != 20 || got.Description != "" {
		t.Fatalf("silver = %+v, %v", got, err)
	}

	toolCalls := &model.SubscriptionPlan{Handle: "agents", Name: "Agents", OrganizationUUID: "org-1", Status: model.SubscriptionPlanStatusActive,
		Conditions: []model.SubscriptionPlanCondition{{ID: "tool-calls", Type: model.ConditionTypeToolCallCount,
			Params: []byte(`{"limits":[{"count":10,"timeAmount":1,"timeUnit":"MINUTE"}],"onExceed":"reject"}`)}}}
	if err := repo.Create(toolCalls); !errors.Is(err, ErrPlanConditionsTableMissing) {
		t.Fatalf("Create with a tool-call condition = %v, want ErrPlanConditionsTableMissing", err)
	}
	if exists, _ := repo.ExistsByHandleAndOrg("agents", "org-1"); exists {
		t.Error("a refused plan was stored")
	}
}

func TestInPlaceholders(t *testing.T) {
	for n, want := range map[int]string{0: "", 1: "?", 3: "?,?,?"} {
		if got := inPlaceholders(n); got != want {
			t.Errorf("inPlaceholders(%d) = %q, want %q", n, got, want)
		}
	}
}

// Old limit rows carry no order, so their windows are read shortest first, whatever order they were
// written in; the gateways' single limit is then the shortest one-unit window.
func TestSubscriptionPlanRepo_LegacyWindowsShortestFirst(t *testing.T) {
	db := newPlanTestDB(t)
	asLegacyDatabase(t, db, true)
	insertLegacyPlan(t, db, "p-gold", "gold", 500, "DAY", 1)
	if _, err := db.Exec(`INSERT INTO subscription_plan_limits (uuid, subscription_plan_uuid, limit_type, time_unit, time_amount, limit_count,
		stop_on_quota_reach) VALUES ('l2', 'p-gold', 'REQUEST_COUNT', 'HOUR', 2, 90, 1), ('l3', 'p-gold', 'REQUEST_COUNT', 'MINUTE', 1, 20, 1)`); err != nil {
		t.Fatalf("insert limits: %v", err)
	}
	gold, err := NewSubscriptionPlanRepo(db).GetByHandleAndOrg("gold", "org-1")
	if err != nil {
		t.Fatalf("GetByHandleAndOrg: %v", err)
	}
	want := `{"limits":[{"count":20,"timeAmount":1,"timeUnit":"MINUTE"},{"count":90,"timeAmount":2,"timeUnit":"HOUR"},{"count":500,"timeAmount":1,"timeUnit":"DAY"}],"onExceed":"reject"}`
	if string(gold.Conditions[0].Params) != want {
		t.Fatalf("params = %s, want %s", gold.Conditions[0].Params, want)
	}
	if *gold.ThrottleLimitCount != 20 || gold.ThrottleLimitUnit != "MINUTE" {
		t.Fatalf("legacy limit = %d %s, want 20 MINUTE", *gold.ThrottleLimitCount, gold.ThrottleLimitUnit)
	}
}
