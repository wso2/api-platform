/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied.  See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

// End-to-end coverage for /subscription-plans over the real handler, service and
// SQLite-backed repository stack. These pin what only an HTTP round trip can see:
// how a JSON null differs from an omitted expiryTime on PUT, and the list paging.

package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"

	_ "github.com/mattn/go-sqlite3"
)

const (
	planITOrgID = "org-plan-001"
	planITUser  = "sub-plan-user"
	planITPath  = constants.APIBasePath + "/subscription-plans"
)

// setupPlanHandlerTestEnv builds the plan handler over a real SQLite database
// carrying the production schema, with one organization seeded.
func setupPlanHandlerTestEnv(t *testing.T) http.Handler {
	t.Helper()

	sqlDB, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "plan-handler-test.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	db := &database.DB{DB: sqlDB}

	schema, err := os.ReadFile(filepath.Join("..", "database", "schema.sqlite.sql"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if _, err := db.Exec(string(schema)); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid)
		VALUES (?, 'plan-org', 'Plan Org', 'default', 'idp-ref')`, planITOrgID); err != nil {
		t.Fatalf("seed organization: %v", err)
	}

	identityService := service.NewIdentityService(repository.NewUserIdentityMappingRepo(db))
	planService := service.NewSubscriptionPlanService(
		repository.NewSubscriptionPlanRepo(db),
		repository.NewGatewayRepo(db),
		repository.NewOrganizationRepo(db),
		service.NewGatewayEventsService(noopEventHub{}, identityService, slog.Default()),
		noopAudit{},
		slog.Default(),
	)

	mux := http.NewServeMux()
	NewSubscriptionPlanHandler(planService, identityService, slog.Default()).RegisterRoutes(mux)
	return middleware.NewTestContextMiddleware(mux)
}

func planRequest(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Org", planITOrgID)
	req.Header.Set("X-Test-User", planITUser)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodePlan(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var plan map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return plan
}

// listPlans issues GET /subscription-plans with the given query string and returns
// the number of plans on the page along with the pagination block.
func listPlans(t *testing.T, h http.Handler, query string) (int, map[string]any) {
	t.Helper()

	rec := planRequest(t, h, http.MethodGet, planITPath+query, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list %q: status %d, body %s", query, rec.Code, rec.Body.String())
	}
	resp := decodePlan(t, rec)
	list, _ := resp["list"].([]any)
	pagination, _ := resp["pagination"].(map[string]any)
	return len(list), pagination
}

// TestPlanHandler_UpdateExpiryTime walks one plan through the three things a PUT can
// say about expiryTime: omit it (keep), send a value (replace), send null (clear).
func TestPlanHandler_UpdateExpiryTime(t *testing.T) {
	h := setupPlanHandlerTestEnv(t)

	rec := planRequest(t, h, http.MethodPost, planITPath,
		`{"id":"gold","displayName":"Gold","expiryTime":"2026-12-31T00:00:00Z"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status %d, body %s", rec.Code, rec.Body.String())
	}

	steps := []struct {
		name       string
		body       string
		wantExpiry any
	}{
		{"omitted keeps the expiry", `{"id":"gold","displayName":"Gold"}`, "2026-12-31T00:00:00Z"},
		{"a value replaces it", `{"id":"gold","displayName":"Gold","expiryTime":"2027-06-30T00:00:00Z"}`, "2027-06-30T00:00:00Z"},
		{"null clears it", `{"id":"gold","displayName":"Gold","expiryTime":null}`, nil},
		{"omitted keeps it cleared", `{"id":"gold","displayName":"Gold"}`, nil},
	}
	for _, step := range steps {
		rec := planRequest(t, h, http.MethodPut, planITPath+"/gold", step.body)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: PUT status %d, body %s", step.name, rec.Code, rec.Body.String())
		}
		if got := decodePlan(t, rec)["expiryTime"]; got != step.wantExpiry {
			t.Errorf("%s: PUT response expiryTime = %v, want %v", step.name, got, step.wantExpiry)
		}

		// The response is built from the saved row, but read it back to be sure the
		// change reached storage.
		stored := decodePlan(t, planRequest(t, h, http.MethodGet, planITPath+"/gold", ""))
		if got := stored["expiryTime"]; got != step.wantExpiry {
			t.Errorf("%s: stored expiryTime = %v, want %v", step.name, got, step.wantExpiry)
		}
	}
}

// TestPlanHandler_ListPaging checks limit/offset slice the list while pagination.total
// stays the full count, which is what the console's pager is driven by.
func TestPlanHandler_ListPaging(t *testing.T) {
	h := setupPlanHandlerTestEnv(t)
	for i := 1; i <= 25; i++ {
		body := fmt.Sprintf(`{"id":"plan-%d","displayName":"Plan %d"}`, i, i)
		if rec := planRequest(t, h, http.MethodPost, planITPath, body); rec.Code != http.StatusCreated {
			t.Fatalf("create plan %d: status %d, body %s", i, rec.Code, rec.Body.String())
		}
	}

	pages := []struct {
		query     string
		wantCount int
		wantLimit float64
		wantStart float64
	}{
		{"", 20, 20, 0},
		{"?limit=20&offset=20", 5, 20, 20},
		{"?limit=10&offset=10", 10, 10, 10},
		{"?limit=20&offset=40", 0, 20, 40},
	}
	for _, page := range pages {
		count, pagination := listPlans(t, h, page.query)
		if count != page.wantCount {
			t.Errorf("list %q: %d items, want %d", page.query, count, page.wantCount)
		}
		if pagination["total"] != float64(25) || pagination["limit"] != page.wantLimit || pagination["offset"] != page.wantStart {
			t.Errorf("list %q: pagination = %v, want total 25, limit %v, offset %v",
				page.query, pagination, page.wantLimit, page.wantStart)
		}
	}
}

// TestPlanHandler_ListSearch checks that query filters across the whole collection, not
// just one page, and that pagination.total counts the matches so the pager follows them.
func TestPlanHandler_ListSearch(t *testing.T) {
	h := setupPlanHandlerTestEnv(t)
	for _, plan := range []string{
		`{"id":"gold-basic","displayName":"Gold Basic"}`,
		`{"id":"gold-pro","displayName":"Gold Pro"}`,
		`{"id":"silver","displayName":"Silver Tier"}`,
		`{"id":"bronze","displayName":"Bronze 100%"}`,
	} {
		if rec := planRequest(t, h, http.MethodPost, planITPath, plan); rec.Code != http.StatusCreated {
			t.Fatalf("create %s: status %d, body %s", plan, rec.Code, rec.Body.String())
		}
	}

	searches := []struct {
		name      string
		query     string
		wantCount int
		wantTotal float64
	}{
		{"matches display name, case-insensitive", "?query=SILVER", 1, 1},
		{"matches handle", "?query=gold-pro", 1, 1},
		{"matches several", "?query=gold", 2, 2},
		{"total counts matches beyond the page", "?query=gold&limit=1", 1, 2},
		{"second page of matches", "?query=gold&limit=1&offset=1", 1, 2},
		{"no match", "?query=platinum", 0, 0},
		{"percent is literal, not a wildcard", "?query=%25", 1, 1},
		{"blank search returns everything", "?query=", 4, 4},
	}
	for _, s := range searches {
		count, pagination := listPlans(t, h, s.query)
		if count != s.wantCount {
			t.Errorf("%s: %d items, want %d", s.name, count, s.wantCount)
		}
		if pagination["total"] != s.wantTotal {
			t.Errorf("%s: pagination.total = %v, want %v", s.name, pagination["total"], s.wantTotal)
		}
	}
}

func TestPlanHandler_UpdateRejectsBadBody(t *testing.T) {
	h := setupPlanHandlerTestEnv(t)
	if rec := planRequest(t, h, http.MethodPost, planITPath, `{"id":"gold","displayName":"Gold"}`); rec.Code != http.StatusCreated {
		t.Fatalf("create: status %d, body %s", rec.Code, rec.Body.String())
	}

	tests := map[string]string{
		"malformed json": `{"displayName":`,
		"oversized body": `{"displayName":"` + strings.Repeat("x", maxPlanUpdateBodyBytes) + `"}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			if rec := planRequest(t, h, http.MethodPut, planITPath+"/gold", body); rec.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want 400, body %.200s", rec.Code, rec.Body.String())
			}
		})
	}
}
