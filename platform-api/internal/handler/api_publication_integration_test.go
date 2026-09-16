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
	"log/slog"
	"mime/multipart"
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

	_ "github.com/mattn/go-sqlite3"
)

// setupPublicationTestEnv creates a full PublicationHandler stack backed by an
// in-memory SQLite DB, seeded with one org, one rest_apis artifact ("my-api"),
// one active API Portal ("my-portal") and one subscription plan ("gold") — the
// minimum every draft endpoint needs to resolve its path.
func setupPublicationTestEnv(t *testing.T) (http.Handler, func()) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("failed to enable foreign keys: %v", err)
	}
	db := &database.DB{DB: sqlDB}

	schema, err := os.ReadFile(filepath.Join("..", "database", "schema.sqlite.sql"))
	if err != nil {
		t.Fatalf("failed to read schema: %v", err)
	}
	if _, err := db.Exec(string(schema)); err != nil {
		t.Fatalf("failed to apply schema: %v", err)
	}

	seed := []string{
		`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid) VALUES ('org-1', 'test-org', 'Test Org', 'default', 'idp-ref')`,
		`INSERT INTO projects (uuid, handle, display_name, organization_uuid) VALUES ('proj-1', 'proj', 'Project', 'org-1')`,
		`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES ('api-artifact-1', 'RestApi', 'org-1')`,
		`INSERT INTO rest_apis (uuid, organization_uuid, handle, display_name, version, project_uuid, lifecycle_status, configuration) VALUES ('api-artifact-1', 'org-1', 'my-api', 'My API', 'v1', 'proj-1', 'CREATED', '{}')`,
		`INSERT INTO api_portals (uuid, organization_uuid, handle, display_name, workflow_status, auth_type, auth_configuration, metadata) VALUES ('portal-1', 'org-1', 'my-portal', 'My Portal', 'active', 'local', '{}', '{}')`,
		`INSERT INTO subscription_plans (uuid, handle, display_name, organization_uuid) VALUES ('plan-1', 'gold', 'Gold', 'org-1')`,
		`INSERT INTO api_documents (uuid, artifact_uuid, organization_uuid, type, handle, display_name, content) VALUES ('doc-1', 'api-artifact-1', 'org-1', 'MARKDOWN', 'quickstart', 'Quickstart', 'content')`,
	}
	for _, q := range seed {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("seed failed (%s): %v", q, err)
		}
	}

	identityService := service.NewIdentityService(repository.NewUserIdentityMappingRepo(db))
	publicationService := service.NewPublicationService(
		repository.NewArtifactRepo(db),
		repository.NewApiPortalRepo(db),
		repository.NewApiDocumentRepo(db),
		repository.NewSubscriptionPlanRepo(db),
		repository.NewPublicationRepo(db),
		slog.Default(),
	)
	h := NewPublicationHandler(publicationService, identityService, 0, 0, slog.Default())

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	cleanup := func() { sqlDB.Close() }
	return middleware.NewTestContextMiddleware(mux), cleanup
}

const draftPath = "/api/v0.9/api-portals/my-portal/apis/rest-api/my-api/draft"

func doPublicationRequest(r http.Handler, method, path, contentType string, body []byte) *httptest.ResponseRecorder {
	req, _ := http.NewRequest(method, path, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("X-Test-Org", "org-1")
	req.Header.Set("X-Test-User", "alice")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestPublicationHandler_GetDraft_404BeforeAnySave verifies the draft-not-found
// path returns the DRAFT_NOT_FOUND code through the real HTTP stack.
func TestPublicationHandler_GetDraft_404BeforeAnySave(t *testing.T) {
	r, cleanup := setupPublicationTestEnv(t)
	defer cleanup()

	w := doPublicationRequest(r, http.MethodGet, draftPath, "", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["code"] != "DRAFT_NOT_FOUND" {
		t.Fatalf("want code DRAFT_NOT_FOUND, got %v", body)
	}
}

// TestPublicationHandler_SaveAndGetDraft_RoundTrip drives the details PUT/GET
// pair through the real HTTP stack, confirming the JSON request/response
// conversion (draftInputToModel/draftModelToResponse) is correct end-to-end —
// not just the underlying service call.
func TestPublicationHandler_SaveAndGetDraft_RoundTrip(t *testing.T) {
	r, cleanup := setupPublicationTestEnv(t)
	defer cleanup()

	reqBody := []byte(`{
		"displayName": "My Listing",
		"version": "1.0.0",
		"description": "desc",
		"tags": ["a", "b"],
		"agentVisibility": "HIDDEN",
		"endpoints": {"productionUrl": "https://prod.example.com"},
		"owners": {"businessOwner": "Jane"},
		"subscriptionPlanIds": ["gold"],
		"docIds": ["quickstart"]
	}`)
	w := doPublicationRequest(r, http.MethodPut, draftPath, "application/json", reqBody)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT draft: want 200, got %d: %s", w.Code, w.Body.String())
	}

	var saved map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil {
		t.Fatalf("PUT draft: invalid JSON response: %v", err)
	}
	if saved["displayName"] != "My Listing" || saved["version"] != "1.0.0" || saved["agentVisibility"] != "HIDDEN" {
		t.Fatalf("PUT draft: unexpected core fields: %v", saved)
	}
	plans, _ := saved["subscriptionPlanIds"].([]any)
	if len(plans) != 1 || plans[0] != "gold" {
		t.Fatalf("PUT draft: want subscriptionPlanIds [gold], got %v", saved["subscriptionPlanIds"])
	}
	docs, _ := saved["docIds"].([]any)
	if len(docs) != 1 || docs[0] != "quickstart" {
		t.Fatalf("PUT draft: want docIds [quickstart], got %v", saved["docIds"])
	}
	if saved["hasThumbnail"] != false || saved["hasLandingPage"] != false {
		t.Fatalf("PUT draft: want hasThumbnail/hasLandingPage false, got %v", saved)
	}
	endpoints, _ := saved["endpoints"].(map[string]any)
	if endpoints["productionUrl"] != "https://prod.example.com" {
		t.Fatalf("PUT draft: endpoints did not round-trip: %v", saved["endpoints"])
	}
	owners, _ := saved["owners"].(map[string]any)
	if owners["businessOwner"] != "Jane" {
		t.Fatalf("PUT draft: owners did not round-trip: %v", saved["owners"])
	}

	w = doPublicationRequest(r, http.MethodGet, draftPath, "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET draft: want 200, got %d: %s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["displayName"] != "My Listing" {
		t.Fatalf("GET draft: did not return the saved row: %v", got)
	}
}

// TestPublicationHandler_SaveDraft_UnknownDocId verifies the 400 path renders
// correctly through the real HTTP stack.
func TestPublicationHandler_SaveDraft_UnknownDocId(t *testing.T) {
	r, cleanup := setupPublicationTestEnv(t)
	defer cleanup()

	reqBody := []byte(`{"displayName": "x", "version": "1.0", "docIds": ["no-such-doc"]}`)
	w := doPublicationRequest(r, http.MethodPut, draftPath, "application/json", reqBody)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["code"] != "PUBLICATION_VALIDATION_FAILED" {
		t.Fatalf("want code PUBLICATION_VALIDATION_FAILED, got %v", body)
	}
}

// TestPublicationHandler_ContentEndpoints drives definition/landing-page/
// thumbnail PUT+GET through the real HTTP stack, including the one multipart
// upload in this feature.
func TestPublicationHandler_ContentEndpoints(t *testing.T) {
	r, cleanup := setupPublicationTestEnv(t)
	defer cleanup()

	// Details must exist before any content PUT.
	w := doPublicationRequest(r, http.MethodPut, draftPath, "application/json", []byte(`{"displayName":"x","version":"1.0"}`))
	if w.Code != http.StatusOK {
		t.Fatalf("PUT draft: want 200, got %d: %s", w.Code, w.Body.String())
	}

	// Definition.
	w = doPublicationRequest(r, http.MethodPut, draftPath+"/definition", "application/json", []byte(`{"openapi":"3.0.0"}`))
	if w.Code != http.StatusNoContent {
		t.Fatalf("PUT definition: want 204, got %d: %s", w.Code, w.Body.String())
	}
	w = doPublicationRequest(r, http.MethodGet, draftPath+"/definition", "", nil)
	if w.Code != http.StatusOK || w.Body.String() != `{"openapi":"3.0.0"}` {
		t.Fatalf("GET definition: want 200 with echoed body, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("GET definition: want Content-Type application/json, got %q", ct)
	}

	// Landing page — embedded HTML must be stripped.
	w = doPublicationRequest(r, http.MethodPut, draftPath+"/landing-page", "text/markdown", []byte("# Hi\n<script>alert(1)</script>"))
	if w.Code != http.StatusNoContent {
		t.Fatalf("PUT landing-page: want 204, got %d: %s", w.Code, w.Body.String())
	}
	w = doPublicationRequest(r, http.MethodGet, draftPath+"/landing-page", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET landing-page: want 200, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "<script>") || strings.Contains(w.Body.String(), "alert(1)") {
		t.Fatalf("GET landing-page: want embedded script stripped, got %q", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "# Hi") {
		t.Fatalf("GET landing-page: want markdown preserved, got %q", w.Body.String())
	}

	// Thumbnail — multipart upload, content-sniffed.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", "icon.png")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	pngBytes := []byte("\x89PNG\r\n\x1a\n0000000000")
	if _, err := part.Write(pngBytes); err != nil {
		t.Fatalf("write part: %v", err)
	}
	_ = mw.Close()
	w = doPublicationRequest(r, http.MethodPut, draftPath+"/thumbnail", mw.FormDataContentType(), buf.Bytes())
	if w.Code != http.StatusNoContent {
		t.Fatalf("PUT thumbnail: want 204, got %d: %s", w.Code, w.Body.String())
	}
	w = doPublicationRequest(r, http.MethodGet, draftPath+"/thumbnail", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET thumbnail: want 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("GET thumbnail: want Content-Type image/png, got %q", ct)
	}
	if w.Body.String() != string(pngBytes) {
		t.Fatalf("GET thumbnail: content did not round-trip")
	}

	// The details GET must now reflect both flags.
	w = doPublicationRequest(r, http.MethodGet, draftPath, "", nil)
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["hasThumbnail"] != true || got["hasLandingPage"] != true {
		t.Fatalf("GET draft: want hasThumbnail/hasLandingPage true, got %v", got)
	}
}
