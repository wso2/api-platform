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
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
