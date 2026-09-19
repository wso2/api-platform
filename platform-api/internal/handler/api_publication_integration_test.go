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
	"context"
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
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"

	_ "github.com/mattn/go-sqlite3"
)

// alwaysSucceedsPortalPublisher is a PortalPublisher test double for tests
// that don't care about the portal push itself (draft CRUD, resolution
// logic) — every call succeeds, same as the old stand-in publisher server.go
// used before a real per-portal auth key existed to publish for real.
type alwaysSucceedsPortalPublisher struct{}

func (alwaysSucceedsPortalPublisher) Publish(_ context.Context, _ *model.APIPortal, _ string, _ *model.Publication, _ *model.PublicationContent) error {
	return nil
}

func (alwaysSucceedsPortalPublisher) Unpublish(_ context.Context, _ *model.APIPortal, _ string) error {
	return nil
}

func (alwaysSucceedsPortalPublisher) Deprecate(_ context.Context, _ *model.APIPortal, _ string, _ *model.Publication) error {
	return nil
}

// setupPublicationTestEnv creates a full PublicationHandler stack backed by an
// in-memory SQLite DB, seeded with one org, one rest_apis artifact ("my-api"),
// one active API Portal ("my-portal") and one subscription plan ("gold") — the
// minimum every draft endpoint needs to resolve its path.
func setupPublicationTestEnv(t *testing.T) (http.Handler, *database.DB, func()) {
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
		`INSERT INTO api_portals (uuid, organization_uuid, handle, display_name, status, internal_auth_key, metadata) VALUES ('portal-1', 'org-1', 'my-portal', 'My Portal', 'active', 'dummy-key', '{}')`,
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
		repository.NewAPIPortalRepo(db),
		repository.NewDocumentRepo(db),
		repository.NewSubscriptionPlanRepo(db),
		repository.NewPublicationRepo(db),
		alwaysSucceedsPortalPublisher{},
		slog.Default(),
	)
	h := NewPublicationHandler(publicationService, identityService, 0, 0, slog.Default())

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	cleanup := func() { sqlDB.Close() }
	return middleware.NewTestContextMiddleware(mux), db, cleanup
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

// livePublicationUUID reads the live (is_draft=0) row's uuid straight from
// the DB for the fixture (artifact_uuid, api_portal_uuid) pairing every test
// in this package uses — the wire response never carries it (api.Publication
// has no id field), so tests asserting the anchor's uuid stays stable across
// a republish/unpublish read it here instead.
func livePublicationUUID(t *testing.T, db *database.DB) string {
	t.Helper()
	var id string
	if err := db.QueryRow(`SELECT uuid FROM api_publications WHERE artifact_uuid = 'api-artifact-1' AND api_portal_uuid = 'portal-1' AND is_draft = 0`).Scan(&id); err != nil {
		t.Fatalf("query live publication uuid: %v", err)
	}
	return id
}

// draftPublicationUUID is livePublicationUUID's is_draft=1 counterpart.
func draftPublicationUUID(t *testing.T, db *database.DB) string {
	t.Helper()
	var id string
	if err := db.QueryRow(`SELECT uuid FROM api_publications WHERE artifact_uuid = 'api-artifact-1' AND api_portal_uuid = 'portal-1' AND is_draft = 1`).Scan(&id); err != nil {
		t.Fatalf("query draft publication uuid: %v", err)
	}
	return id
}

// TestPublicationHandler_GetDraft_404BeforeAnySave verifies the draft-not-found
// path returns the DRAFT_NOT_FOUND code through the real HTTP stack.
func TestPublicationHandler_GetDraft_404BeforeAnySave(t *testing.T) {
	r, _, cleanup := setupPublicationTestEnv(t)
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
	r, _, cleanup := setupPublicationTestEnv(t)
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
	r, _, cleanup := setupPublicationTestEnv(t)
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
	r, _, cleanup := setupPublicationTestEnv(t)
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
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("GET definition: want X-Content-Type-Options nosniff, got %q", got)
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

const publicationPath = "/api/v0.9/api-portals/my-portal/apis/rest-api/my-api/publication"

// TestPublicationHandler_GetPublication_404WhenNotPublished verifies the
// not-published path returns PUBLICATION_NOT_FOUND.
func TestPublicationHandler_GetPublication_404WhenNotPublished(t *testing.T) {
	r, _, cleanup := setupPublicationTestEnv(t)
	defer cleanup()

	w := doPublicationRequest(r, http.MethodGet, publicationPath, "", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["code"] != "PUBLICATION_NOT_FOUND" {
		t.Fatalf("want code PUBLICATION_NOT_FOUND, got %v", body)
	}
}

// TestPublicationHandler_GetPublication_RoundTrip seeds a live (is_draft = 0)
// row directly, isolating the read path from Publish, and drives all four
// publication reads through the real HTTP stack.
func TestPublicationHandler_GetPublication_RoundTrip(t *testing.T) {
	r, db, cleanup := setupPublicationTestEnv(t)
	defer cleanup()

	seedPublication := []string{
		`INSERT INTO api_publications (
			uuid, organization_uuid, artifact_uuid, api_portal_uuid, is_draft, status,
			display_name, version, description, agent_visibility, created_by, updated_by
		) VALUES (
			'pub-1', 'org-1', 'api-artifact-1', 'portal-1', 0, 'PUBLISHED',
			'My Listing', '1.0.0', 'live desc', 'VISIBLE', 'alice', 'alice'
		)`,
		`INSERT INTO api_publication_plan_mappings (organization_uuid, publication_uuid, subscription_plan_uuid, created_by)
			VALUES ('org-1', 'pub-1', 'plan-1', 'alice')`,
		`INSERT INTO api_publication_doc_mappings (organization_uuid, publication_uuid, doc_uuid, created_by)
			VALUES ('org-1', 'pub-1', 'doc-1', 'alice')`,
		`INSERT INTO api_publication_contents (uuid, organization_uuid, publication_uuid, type, content_type, content, created_by, updated_by)
			VALUES ('content-1', 'org-1', 'pub-1', 'API_DEFINITION', 'application/json', '{"openapi":"3.0.0"}', 'alice', 'alice')`,
		`INSERT INTO api_publication_contents (uuid, organization_uuid, publication_uuid, type, content_type, content, created_by, updated_by)
			VALUES ('content-2', 'org-1', 'pub-1', 'MARKETING', 'text/markdown', '# Live', 'alice', 'alice')`,
		`INSERT INTO api_publication_contents (uuid, organization_uuid, publication_uuid, type, content_type, content, created_by, updated_by)
			VALUES ('content-3', 'org-1', 'pub-1', 'IMAGE', 'image/png', X'89504e470d0a1a0a', 'alice', 'alice')`,
	}
	for _, q := range seedPublication {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("seed failed (%s): %v", q, err)
		}
	}

	w := doPublicationRequest(r, http.MethodGet, publicationPath, "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET publication: want 200, got %d: %s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["displayName"] != "My Listing" || got["status"] != "PUBLISHED" {
		t.Fatalf("GET publication: unexpected core fields: %v", got)
	}
	if got["apiPortalId"] != "my-portal" || got["apiPortalName"] != "My Portal" {
		t.Fatalf("GET publication: want apiPortalId/apiPortalName from the resolved portal, got %v", got)
	}
	plans, _ := got["subscriptionPlanIds"].([]any)
	if len(plans) != 1 || plans[0] != "gold" {
		t.Fatalf("GET publication: want subscriptionPlanIds [gold], got %v", got["subscriptionPlanIds"])
	}
	docs, _ := got["docIds"].([]any)
	if len(docs) != 1 || docs[0] != "quickstart" {
		t.Fatalf("GET publication: want docIds [quickstart], got %v", got["docIds"])
	}
	if got["hasThumbnail"] != true || got["hasLandingPage"] != true {
		t.Fatalf("GET publication: want hasThumbnail/hasLandingPage true, got %v", got)
	}

	w = doPublicationRequest(r, http.MethodGet, publicationPath+"/definition", "", nil)
	if w.Code != http.StatusOK || w.Body.String() != `{"openapi":"3.0.0"}` {
		t.Fatalf("GET publication definition: want 200 with stored body, got %d: %s", w.Code, w.Body.String())
	}

	w = doPublicationRequest(r, http.MethodGet, publicationPath+"/landing-page", "", nil)
	if w.Code != http.StatusOK || w.Body.String() != "# Live" {
		t.Fatalf("GET publication landing-page: want 200 with stored body, got %d: %s", w.Code, w.Body.String())
	}

	w = doPublicationRequest(r, http.MethodGet, publicationPath+"/thumbnail", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET publication thumbnail: want 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("GET publication thumbnail: want Content-Type image/png, got %q", ct)
	}
}

const listPublicationsPath = "/api/v0.9/api-publications?apiType=rest-api&apiId=my-api"

// TestPublicationHandler_ListPublications_MixedStatuses seeds a second
// (published, active) portal and a third (pending, not-yet-active) one
// alongside the base env's "my-portal", then verifies the rollup: an
// unpublished portal reports NOT_PUBLISHED, a published one PUBLISHED, a
// deprecated one DEPRECATED, and the pending portal is excluded entirely.
func TestPublicationHandler_ListPublications_MixedStatuses(t *testing.T) {
	r, db, cleanup := setupPublicationTestEnv(t)
	defer cleanup()

	seed := []string{
		`INSERT INTO api_portals (uuid, organization_uuid, handle, display_name, status, internal_auth_key, metadata)
			VALUES ('portal-2', 'org-1', 'partner-portal', 'Partner Portal', 'active', 'dummy-key', '{}')`,
		`INSERT INTO api_portals (uuid, organization_uuid, handle, display_name, status, internal_auth_key, metadata)
			VALUES ('portal-3', 'org-1', 'staging-portal', 'Staging Portal', 'pending', 'dummy-key', '{}')`,
		// my-portal (portal-1): a draft only -> still NOT_PUBLISHED, but draftUpdatedAt is set.
		`INSERT INTO api_publications (uuid, organization_uuid, artifact_uuid, api_portal_uuid, is_draft, display_name, version, created_by, updated_by)
			VALUES ('draft-1', 'org-1', 'api-artifact-1', 'portal-1', 1, 'My Listing', '1.0.0', 'alice', 'alice')`,
		// partner-portal (portal-2): a live, deprecated row.
		`INSERT INTO api_publications (uuid, organization_uuid, artifact_uuid, api_portal_uuid, is_draft, status, display_name, version, created_by, updated_by)
			VALUES ('pub-2', 'org-1', 'api-artifact-1', 'portal-2', 0, 'DEPRECATED', 'My Listing', '1.0.0', 'alice', 'alice')`,
	}
	for _, q := range seed {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("seed failed (%s): %v", q, err)
		}
	}

	w := doPublicationRequest(r, http.MethodGet, listPublicationsPath, "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET api-publications: want 200, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	list, _ := body["list"].([]any)
	if len(list) != 2 {
		t.Fatalf("want 2 active portals (staging-portal excluded), got %d: %v", len(list), list)
	}

	byPortal := make(map[string]map[string]any, len(list))
	for _, raw := range list {
		item, _ := raw.(map[string]any)
		byPortal[item["apiPortalId"].(string)] = item
	}

	myPortal, ok := byPortal["my-portal"]
	if !ok {
		t.Fatalf("want my-portal in the rollup, got %v", byPortal)
	}
	if myPortal["status"] != "NOT_PUBLISHED" {
		t.Fatalf("my-portal: want status NOT_PUBLISHED (draft only), got %v", myPortal["status"])
	}
	if myPortal["draftUpdatedAt"] == nil {
		t.Fatalf("my-portal: want non-null draftUpdatedAt, got %v", myPortal)
	}
	if myPortal["publicationUpdatedAt"] != nil {
		t.Fatalf("my-portal: want null publicationUpdatedAt, got %v", myPortal)
	}

	partnerPortal, ok := byPortal["partner-portal"]
	if !ok {
		t.Fatalf("want partner-portal in the rollup, got %v", byPortal)
	}
	if partnerPortal["status"] != "DEPRECATED" {
		t.Fatalf("partner-portal: want status DEPRECATED, got %v", partnerPortal["status"])
	}
	if partnerPortal["publicationUpdatedAt"] == nil {
		t.Fatalf("partner-portal: want non-null publicationUpdatedAt, got %v", partnerPortal)
	}

	if _, excluded := byPortal["staging-portal"]; excluded {
		t.Fatalf("want staging-portal (pending) excluded from the rollup, got %v", byPortal)
	}

	pagination, _ := body["pagination"].(map[string]any)
	if pagination["total"] != float64(2) {
		t.Fatalf("want pagination.total 2, got %v", pagination)
	}
}

// TestPublicationHandler_ListPublications_QueryFilter verifies the `query`
// param filters by portal handle/display name.
func TestPublicationHandler_ListPublications_QueryFilter(t *testing.T) {
	r, db, cleanup := setupPublicationTestEnv(t)
	defer cleanup()

	if _, err := db.Exec(`INSERT INTO api_portals (uuid, organization_uuid, handle, display_name, status, internal_auth_key, metadata)
		VALUES ('portal-2', 'org-1', 'partner-portal', 'Partner Portal', 'active', 'dummy-key', '{}')`); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	w := doPublicationRequest(r, http.MethodGet, listPublicationsPath+"&query=partner", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET api-publications: want 200, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	list, _ := body["list"].([]any)
	if len(list) != 1 {
		t.Fatalf("want 1 portal matching query=partner, got %d: %v", len(list), list)
	}
	item, _ := list[0].(map[string]any)
	if item["apiPortalId"] != "partner-portal" {
		t.Fatalf("want partner-portal, got %v", item)
	}
}

// TestPublicationHandler_OversizedBody_Returns413 verifies every body-reading
// draft endpoint answers 413 (not 400) once its size cap is exceeded, while a
// malformed body under the cap stays a 400.
func TestPublicationHandler_OversizedBody_Returns413(t *testing.T) {
	r, _, cleanup := setupPublicationTestEnv(t)
	defer cleanup()

	over := int(defaultPublicationContentMaxBytes) + 1
	bigJSON := append([]byte(`{"displayName":"`), bytes.Repeat([]byte("a"), over)...)
	bigJSON = append(bigJSON, []byte(`"}`)...)
	bigRaw := bytes.Repeat([]byte("a"), over)

	var thumb bytes.Buffer
	mw := multipart.NewWriter(&thumb)
	part, _ := mw.CreateFormFile("file", "t.png")
	_, _ = part.Write(bytes.Repeat([]byte("a"), int(defaultPublicationThumbnailMaxBytes)+1))
	_ = mw.Close()

	cases := []struct {
		name, path, contentType string
		body                    []byte
	}{
		{"draft json", draftPath, "application/json", bigJSON},
		{"definition", draftPath + "/definition", "application/json", bigRaw},
		{"landing page", draftPath + "/landing-page", "text/markdown", bigRaw},
		{"thumbnail", draftPath + "/thumbnail", mw.FormDataContentType(), thumb.Bytes()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doPublicationRequest(r, http.MethodPut, tc.path, tc.contentType, tc.body)
			if w.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("want 413, got %d: %.200s", w.Code, w.Body.String())
			}
		})
	}

	w := doPublicationRequest(r, http.MethodPut, draftPath, "application/json", []byte(`{not json`))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("malformed JSON: want 400, got %d", w.Code)
	}
}

// TestPublicationHandler_Thumbnail_UnusableFileName_Returns400 verifies an upload whose
// declared name sanitizes to nothing is rejected instead of stored with an empty name.
func TestPublicationHandler_Thumbnail_UnusableFileName_Returns400(t *testing.T) {
	r, _, cleanup := setupPublicationTestEnv(t)
	defer cleanup()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("file", "..")
	_, _ = part.Write([]byte("\x89PNG\r\n\x1a\n0000000000"))
	_ = mw.Close()

	w := doPublicationRequest(r, http.MethodPut, draftPath+"/thumbnail", mw.FormDataContentType(), buf.Bytes())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %.200s", w.Code, w.Body.String())
	}
}

// TestPublicationHandler_ResolveAPIErrors verifies the status codes for a
// missing or unresolvable API reference: absent list params are a 400, while
// an unknown apiId or an unrecognised apiType is the same 404 on every route.
func TestPublicationHandler_ResolveAPIErrors(t *testing.T) {
	r, _, cleanup := setupPublicationTestEnv(t)
	defer cleanup()

	const list = "/api/v0.9/api-publications"
	const portalAPIs = "/api/v0.9/api-portals/my-portal/apis"
	cases := []struct {
		name, path string
		want       int
	}{
		{"list without params", list, http.StatusBadRequest},
		{"list without apiId", list + "?apiType=rest-api", http.StatusBadRequest},
		{"list unknown apiId", list + "?apiType=rest-api&apiId=nope", http.StatusNotFound},
		{"list unknown apiType", list + "?apiType=bogus&apiId=my-api", http.StatusNotFound},
		{"draft unknown apiId", portalAPIs + "/rest-api/nope/draft", http.StatusNotFound},
		{"draft unknown apiType", portalAPIs + "/bogus/my-api/draft", http.StatusNotFound},
		{"publication unknown apiType", portalAPIs + "/bogus/my-api/publication", http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doPublicationRequest(r, http.MethodGet, tc.path, "", nil)
			if w.Code != tc.want {
				t.Fatalf("want %d, got %d: %.200s", tc.want, w.Code, w.Body.String())
			}
		})
	}
}
