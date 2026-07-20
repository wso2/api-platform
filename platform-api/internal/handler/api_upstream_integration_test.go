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

	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	_ "github.com/mattn/go-sqlite3"
)

const restAPIBase = "/api/v0.9/rest-apis"
const restAPIOrg = "org-api-it-001"
const restAPIProject = "api-it-project"

// setupAPIHandlerEnv builds the real route -> handler -> service -> repository stack over a
// temporary file-backed SQLite DB, seeded with a test organization and project, so tests can
// assert the HTTP status codes the REST API handler maps service errors to.
func setupAPIHandlerEnv(t *testing.T) (http.Handler, func()) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	db := &database.DB{DB: sqlDB}

	schema, err := os.ReadFile(filepath.Join("..", "database", "schema.sqlite.sql"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if _, err = db.Exec(string(schema)); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	if _, err = db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
		VALUES ('` + restAPIOrg + `', 'test-api-org', 'Test API Org', 'default', 'idp-ref', datetime('now'), datetime('now'))`); err != nil {
		t.Fatalf("insert org: %v", err)
	}
	if _, err = db.Exec(`INSERT INTO projects (uuid, handle, display_name, organization_uuid, description, created_at, updated_at)
		VALUES ('proj-api-it-001', '` + restAPIProject + `', 'API IT Project', '` + restAPIOrg + `', '', datetime('now'), datetime('now'))`); err != nil {
		t.Fatalf("insert project: %v", err)
	}

	identityService := service.NewIdentityService(repository.NewUserIdentityMappingRepo(db))
	apiService := service.NewAPIService(
		repository.NewAPIRepo(db), repository.NewProjectRepo(db), repository.NewOrganizationRepo(db),
		repository.NewGatewayRepo(db),
		nil, // deploymentRepo: unused by the create/update validation paths under test
		repository.NewSubscriptionPlanRepo(db), repository.NewCustomPolicyRepo(db),
		nil, // gatewayEventsService: unused by create/update in these tests
		&utils.APIUtil{}, slog.Default(), repository.NewAuditRepo(db), identityService,
	)

	h := NewAPIHandler(apiService, identityService, slog.Default())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	return middleware.NewTestContextMiddleware(mux), func() { _ = sqlDB.Close() }
}

// doRESTAPIJSON issues an authenticated JSON request against r, scoped to restAPIOrg.
func doRESTAPIJSON(t *testing.T, r http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Test-Org", restAPIOrg)
	req.Header.Set("X-Test-User", "alice")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// restAPIBody returns a create/update request body whose single operation references the
// given upstream definition name; the declared pool contains only "alt-backend".
func restAPIBody(opRef string) string {
	return fmt.Sprintf(`{
		"displayName": "Upstream IT API",
		"version": "v1.0",
		"context": "/upstream-it",
		"projectId": %q,
		"upstream": {"main": {"url": "http://main:8080"}},
		"upstreamDefinitions": [{"name": "alt-backend", "upstreams": [{"url": "http://alt:9000"}]}],
		"operations": [{"request": {"method": "GET", "path": "/x", "upstream": {"main": {"ref": %q}}}}]
	}`, restAPIProject, opRef)
}

// TestAPIHandler_CreateInvalidUpstreamRefReturns400 pins the invalid-upstream mapping to
// HTTP 400 on create: an unresolved per-operation ref must surface as a validation
// failure, not an internal error.
func TestAPIHandler_CreateInvalidUpstreamRefReturns400(t *testing.T) {
	r, cleanup := setupAPIHandlerEnv(t)
	defer cleanup()

	w := doRESTAPIJSON(t, r, http.MethodPost, restAPIBase, restAPIBody("missing"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unresolved per-op ref, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "not declared in upstreamDefinitions") {
		t.Fatalf("expected unresolved-ref detail in response, got: %s", w.Body.String())
	}
}

// TestAPIHandler_CreateInvalidBasePathReturns400 pins the same mapping for the
// upstreamDefinitions basePath contract (must start with '/' and must not end with '/').
func TestAPIHandler_CreateInvalidBasePathReturns400(t *testing.T) {
	r, cleanup := setupAPIHandlerEnv(t)
	defer cleanup()

	body := fmt.Sprintf(`{
		"displayName": "Upstream IT API",
		"version": "v1.0",
		"context": "/upstream-it",
		"projectId": %q,
		"upstream": {"main": {"url": "http://main:8080"}},
		"upstreamDefinitions": [{"name": "alt-backend", "basePath": "/api/v2/", "upstreams": [{"url": "http://alt:9000"}]}],
		"operations": [{"request": {"method": "GET", "path": "/x", "upstream": {"main": {"ref": "alt-backend"}}}}]
	}`, restAPIProject)
	w := doRESTAPIJSON(t, r, http.MethodPost, restAPIBase, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for basePath with trailing '/', got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "basePath") {
		t.Fatalf("expected basePath detail in response, got: %s", w.Body.String())
	}
}

// TestAPIHandler_UpdateInvalidUpstreamRefReturns400 pins the update-path mapping: a valid
// API is created first, then an update that dangles its per-operation ref must return 400.
func TestAPIHandler_UpdateInvalidUpstreamRefReturns400(t *testing.T) {
	r, cleanup := setupAPIHandlerEnv(t)
	defer cleanup()

	created := doRESTAPIJSON(t, r, http.MethodPost, restAPIBase, restAPIBody("alt-backend"))
	if created.Code != http.StatusCreated {
		t.Fatalf("expected 201 for valid create, got %d: %s", created.Code, created.Body.String())
	}
	var createdAPI struct {
		Id string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &createdAPI); err != nil || createdAPI.Id == "" {
		t.Fatalf("could not read created API id: %v, body: %s", err, created.Body.String())
	}

	w := doRESTAPIJSON(t, r, http.MethodPut, restAPIBase+"/"+createdAPI.Id, restAPIBody("missing"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unresolved per-op ref on update, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "not declared in upstreamDefinitions") {
		t.Fatalf("expected unresolved-ref detail in response, got: %s", w.Body.String())
	}
}

// TestAPIHandler_CreateThenGetRoundTripsUpstreamConfig pins the JSON encoding of the
// feature over HTTP: a valid create returns 201 and a subsequent GET returns the pool
// (name, basePath, timeout, weights) and the per-operation ref exactly as configured.
func TestAPIHandler_CreateThenGetRoundTripsUpstreamConfig(t *testing.T) {
	r, cleanup := setupAPIHandlerEnv(t)
	defer cleanup()

	body := fmt.Sprintf(`{
		"displayName": "Upstream RT API",
		"version": "v1.0",
		"context": "/upstream-rt",
		"projectId": %q,
		"upstream": {"main": {"url": "http://main:8080"}},
		"upstreamDefinitions": [{
			"name": "alt-backend",
			"basePath": "/api/v2",
			"timeout": {"connect": "5s"},
			"upstreams": [
				{"url": "http://backend-a:8080", "weight": 80},
				{"url": "http://backend-b:8080", "weight": 20}
			]
		}],
		"operations": [{"request": {"method": "GET", "path": "/x", "upstream": {"main": {"ref": "alt-backend"}}}}]
	}`, restAPIProject)
	created := doRESTAPIJSON(t, r, http.MethodPost, restAPIBase, body)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", created.Code, created.Body.String())
	}
	var createdAPI struct {
		Id string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &createdAPI); err != nil || createdAPI.Id == "" {
		t.Fatalf("could not read created API id: %v, body: %s", err, created.Body.String())
	}

	got := doRESTAPIJSON(t, r, http.MethodGet, restAPIBase+"/"+createdAPI.Id, "")
	if got.Code != http.StatusOK {
		t.Fatalf("expected 200 on GET, got %d: %s", got.Code, got.Body.String())
	}
	var resp struct {
		UpstreamDefinitions []struct {
			Name     string `json:"name"`
			BasePath string `json:"basePath"`
			Timeout  *struct {
				Connect string `json:"connect"`
			} `json:"timeout"`
			Upstreams []struct {
				Url    string `json:"url"`
				Weight *int   `json:"weight"`
			} `json:"upstreams"`
		} `json:"upstreamDefinitions"`
		Operations []struct {
			Request struct {
				Path     string `json:"path"`
				Upstream *struct {
					Main *struct {
						Ref string `json:"ref"`
					} `json:"main"`
				} `json:"upstream"`
			} `json:"request"`
		} `json:"operations"`
	}
	if err := json.Unmarshal(got.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode GET response: %v, body: %s", err, got.Body.String())
	}
	if len(resp.UpstreamDefinitions) != 1 {
		t.Fatalf("want 1 upstreamDefinition, got: %s", got.Body.String())
	}
	def := resp.UpstreamDefinitions[0]
	if def.Name != "alt-backend" || def.BasePath != "/api/v2" {
		t.Errorf("definition mismatch: %+v", def)
	}
	if def.Timeout == nil || def.Timeout.Connect != "5s" {
		t.Errorf("timeout mismatch: %+v", def.Timeout)
	}
	if len(def.Upstreams) != 2 ||
		def.Upstreams[0].Weight == nil || *def.Upstreams[0].Weight != 80 ||
		def.Upstreams[1].Weight == nil || *def.Upstreams[1].Weight != 20 {
		t.Errorf("weighted targets mismatch: %+v", def.Upstreams)
	}
	var sawRef bool
	for _, op := range resp.Operations {
		if op.Request.Path == "/x" {
			sawRef = op.Request.Upstream != nil && op.Request.Upstream.Main != nil &&
				op.Request.Upstream.Main.Ref == "alt-backend"
		}
	}
	if !sawRef {
		t.Errorf("per-op ref missing in GET response: %s", got.Body.String())
	}
}
