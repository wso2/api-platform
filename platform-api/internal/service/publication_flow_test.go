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

package service_test

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

// schemaDB is a SQLite database opened through database.NewConnection, as the
// server opens one (foreign keys on, UTC timestamps), with the real schema
// applied through InitSchema.
type schemaDB struct {
	db *database.DB
}

func openSchemaDB(t *testing.T) *schemaDB {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Database{Driver: "sqlite3", Path: filepath.Join(t.TempDir(), "test.db")}
	db, err := database.NewConnection(cfg, logger)
	if err != nil {
		t.Fatalf("open SQLite database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.InitSchema("../database/schema.sql", logger); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}
	return &schemaDB{db: db}
}

// count returns the number of rows in table whose col equals val.
func (s *schemaDB) count(t *testing.T, table, col, val string) int {
	t.Helper()
	var n int
	q := s.db.Rebind(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = ?", table, col))
	if err := s.db.QueryRow(q, val).Scan(&n); err != nil {
		t.Fatalf("count(%s) failed: %v", table, err)
	}
	return n
}

// exec runs a `?`-placeholder statement.
func (s *schemaDB) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := s.db.Exec(s.db.Rebind(query), args...); err != nil {
		t.Fatalf("exec failed: %v\nquery: %s", err, query)
	}
}

// graph holds the identifiers seeded into a single organization.
type graph struct {
	org, project, apiArtifact string
	plan, planLimit           string
	apiPortal, apiDoc         string
}

// seedOrgGraph inserts the rows a publication hangs off for one organization:
// a REST API, an active subscription plan, one active API Portal and one API
// document. docIds resolution needs a real row to resolve against.
// api_publications itself stays unseeded; the tests write it through
// PublicationService.
func seedOrgGraph(t *testing.T, tdb *schemaDB) graph {
	t.Helper()
	g := graph{
		org: id(), project: id(), apiArtifact: id(),
		plan: id(), planLimit: id(),
		apiPortal: id(), apiDoc: id(),
	}

	tdb.exec(t, `INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid) VALUES (?, ?, ?, ?, ?)`,
		g.org, "h-"+g.org[:8], "it org", "us", "idp-ref")
	tdb.exec(t, `INSERT INTO projects (uuid, handle, display_name, organization_uuid) VALUES (?, ?, ?, ?)`,
		g.project, "proj", "proj", g.org)

	// REST API: an artifact + its rest_apis row (shared uuid).
	tdb.exec(t, `INSERT INTO artifacts (uuid, type, organization_uuid) VALUES (?, ?, ?)`,
		g.apiArtifact, "rest_api", g.org)
	tdb.exec(t, `INSERT INTO rest_apis (uuid, organization_uuid, handle, display_name, version, project_uuid, lifecycle_status, configuration) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		g.apiArtifact, g.org, "api-"+g.apiArtifact[:8], "api", "v1.0", g.project, "CREATED", []byte("{}"))

	tdb.exec(t, `INSERT INTO subscription_plans (uuid, handle, display_name, organization_uuid) VALUES (?, ?, ?, ?)`,
		g.plan, "plan-"+g.plan[:8], "Plan "+g.plan[:8], g.org)
	tdb.exec(t, `INSERT INTO subscription_plan_limits (uuid, subscription_plan_uuid, limit_type, limit_count, time_unit) VALUES (?, ?, ?, ?, ?)`,
		g.planLimit, g.plan, "REQUEST_COUNT", 100, "MINUTE")

	tdb.exec(t, `INSERT INTO api_portals (uuid, organization_uuid, handle, display_name, status, internal_auth_key, metadata) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		g.apiPortal, g.org, "portal-"+g.apiPortal[:8], "portal", "active", []byte("dummy-key"), []byte("{}"))
	tdb.exec(t, `INSERT INTO api_documents (uuid, artifact_uuid, organization_uuid, type, handle, display_name, file_name, content_type, content) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		g.apiDoc, g.apiArtifact, g.org, "MARKDOWN", "doc-"+g.apiDoc[:8], "Quickstart", "quickstart.md", "text/markdown", []byte("# Quickstart"))
	return g
}

func id() string { return uuid.NewString() }

// alwaysSucceedsPortalPublisher is a PortalPublisher test double for tests
// that don't care about the portal push itself (draft CRUD, resolution
// logic) — every call succeeds, same as the old stand-in publisher server.go
// used before a real per-portal auth key existed to publish for real.
type alwaysSucceedsPortalPublisher struct{}

func (alwaysSucceedsPortalPublisher) Publish(_ context.Context, _ *model.APIPortal, _, _ string, _ *model.Publication, _ *model.PublicationContent) error {
	return nil
}

func (alwaysSucceedsPortalPublisher) Unpublish(_ context.Context, _ *model.APIPortal, _ string) error {
	return nil
}

func (alwaysSucceedsPortalPublisher) CreateMissingPlans(_ context.Context, _ *model.APIPortal, _ []*model.SubscriptionPlan) error {
	return nil
}

func (alwaysSucceedsPortalPublisher) Deprecate(_ context.Context, _ *model.APIPortal, _, _ string, _ *model.Publication) error {
	return nil
}

func (alwaysSucceedsPortalPublisher) PublishContent(_ context.Context, _ *model.APIPortal, _ string, _ *service.PortalContent) error {
	return nil
}

// newPublicationTestService wires a real PublicationService against tdb.db,
// using the same repositories production code uses, so the service's
// validation/resolution logic (handle resolution, unknown-handle rejection)
// runs against real rows rather than repository fakes.
func newPublicationTestService(tdb *schemaDB) *service.PublicationService {
	return newPublicationTestServiceWith(tdb, alwaysSucceedsPortalPublisher{})
}

const publicationTestOperationTimeout = time.Minute

// newPublicationTestServiceWith builds the service with the given portal double.
func newPublicationTestServiceWith(tdb *schemaDB, portalPublisher service.PortalPublisher) *service.PublicationService {
	return newPublicationTestServiceWithTimeout(tdb, portalPublisher, publicationTestOperationTimeout)
}

func newPublicationTestServiceWithTimeout(tdb *schemaDB, portalPublisher service.PortalPublisher, operationTimeout time.Duration) *service.PublicationService {
	return service.NewPublicationService(
		repository.NewArtifactRepo(tdb.db),
		repository.NewAPIPortalRepo(tdb.db),
		repository.NewDocumentRepo(tdb.db),
		repository.NewSubscriptionPlanRepo(tdb.db),
		repository.NewPublicationRepo(tdb.db),
		portalPublisher,
		operationTimeout,
		0,
		nil,
	)
}

// apiHandleFor / portalHandleFor / planHandleFor / docHandleFor reconstruct
// the handles seedOrgGraph derives from its own UUIDs (it doesn't store them
// on graph — only the uuids it generated).
func apiHandleFor(g graph) string    { return "api-" + g.apiArtifact[:8] }
func portalHandleFor(g graph) string { return "portal-" + g.apiPortal[:8] }
func planHandleFor(g graph) string   { return "plan-" + g.plan[:8] }
func docHandleFor(g graph) string    { return "doc-" + g.apiDoc[:8] }

// TestPublicationDraft_SaveAndGetRoundTrip drives the draft-details save
// through the real service+repository stack: PUT a complete draft —
// including a non-empty docIds and subscriptionPlanIds — GET it back, and
// confirm every field round-trips, then save again to confirm the second
// save updates the same row rather than creating a second one, and
// correctly replaces (not merges) the mapping tables.
func TestPublicationDraft_SaveAndGetRoundTrip(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	svc := newPublicationTestService(tdb)

	apiType := "rest-api"
	apiHandle := apiHandleFor(g)
	portalHandle := portalHandleFor(g)
	planHandle := planHandleFor(g)
	docHandle := docHandleFor(g)

	draft := &model.Publication{
		DisplayName:         "My API Listing",
		Version:             "1.0.0",
		Description:         "desc",
		Tags:                []string{"a", "b"},
		Labels:              []string{"partner"},
		AgentVisibility:     "VISIBLE",
		ProductionURL:       "https://prod.example.com",
		SandboxURL:          "https://sandbox.example.com",
		BusinessOwner:       "Jane",
		BusinessOwnerEmail:  "jane@example.com",
		TechnicalOwner:      "John",
		TechnicalOwnerEmail: "john@example.com",
	}

	saved, err := svc.SaveDraftDetails(apiType, apiHandle, portalHandle, g.org, "actor-1", draft, []string{planHandle}, []string{docHandle})
	if err != nil {
		t.Fatalf("SaveDraftDetails failed: %v", err)
	}
	if saved.UUID == "" {
		t.Fatalf("want a generated draft UUID")
	}
	if len(saved.SubscriptionPlanIds) != 1 || saved.SubscriptionPlanIds[0] != planHandle {
		t.Fatalf("want subscriptionPlanIds [%s], got %v", planHandle, saved.SubscriptionPlanIds)
	}
	if len(saved.DocIds) != 1 || saved.DocIds[0] != docHandle {
		t.Fatalf("want docIds [%s], got %v", docHandle, saved.DocIds)
	}

	got, err := svc.GetDraft(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("GetDraft failed: %v", err)
	}
	if got.UUID != saved.UUID {
		t.Fatalf("want GetDraft to return the saved row")
	}
	if got.DisplayName != "My API Listing" || got.Version != "1.0.0" || got.Description != "desc" {
		t.Fatalf("core details did not round-trip: %+v", got)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "a" || got.Tags[1] != "b" {
		t.Fatalf("tags did not round-trip: %v", got.Tags)
	}
	if len(got.Labels) != 1 || got.Labels[0] != "partner" {
		t.Fatalf("labels did not round-trip: %v", got.Labels)
	}
	if got.ProductionURL != "https://prod.example.com" || got.SandboxURL != "https://sandbox.example.com" {
		t.Fatalf("endpoints did not round-trip: %+v", got)
	}
	if got.BusinessOwner != "Jane" || got.TechnicalOwner != "John" {
		t.Fatalf("owners did not round-trip: %+v", got)
	}
	if len(got.SubscriptionPlanIds) != 1 || got.SubscriptionPlanIds[0] != planHandle {
		t.Fatalf("want subscriptionPlanIds [%s] on GET, got %v", planHandle, got.SubscriptionPlanIds)
	}
	if len(got.DocIds) != 1 || got.DocIds[0] != docHandle {
		t.Fatalf("want docIds [%s] on GET, got %v", docHandle, got.DocIds)
	}
	if got.HasThumbnail || got.HasLandingPage {
		t.Fatalf("want no content yet, got hasThumbnail=%v hasLandingPage=%v", got.HasThumbnail, got.HasLandingPage)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("want non-zero audit timestamps")
	}

	// Second save: same (artifact, portal) pairing must update the existing
	// draft row, not create a second one, and must fully replace (not merge)
	// the plan/document selections — an empty selection this time round.
	draft2 := &model.Publication{DisplayName: "Updated Listing", Version: "1.0.1", AgentVisibility: "VISIBLE"}
	saved2, err := svc.SaveDraftDetails(apiType, apiHandle, portalHandle, g.org, "actor-2", draft2, nil, nil)
	if err != nil {
		t.Fatalf("second SaveDraftDetails failed: %v", err)
	}
	if saved2.UUID != saved.UUID {
		t.Fatalf("want the same draft row reused on a second save, got a different UUID")
	}
	if len(saved2.SubscriptionPlanIds) != 0 {
		t.Fatalf("want subscriptionPlanIds cleared on an empty resave, got %v", saved2.SubscriptionPlanIds)
	}
	if len(saved2.DocIds) != 0 {
		t.Fatalf("want docIds cleared on an empty resave, got %v", saved2.DocIds)
	}
	if tdb.count(t, "api_publications", "uuid", saved.UUID) != 1 {
		t.Fatalf("want exactly one api_publications row for this pairing")
	}
}

// TestPublicationDraft_UnknownPlanHandleRejected mirrors the doc-handle case
// for subscriptionPlanIds.
func TestPublicationDraft_UnknownPlanHandleRejected(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	svc := newPublicationTestService(tdb)

	draft := &model.Publication{DisplayName: "x", Version: "1.0", AgentVisibility: "VISIBLE"}
	_, err := svc.SaveDraftDetails("rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "actor", draft, []string{"does-not-exist"}, nil)
	if err == nil {
		t.Fatalf("want an error for an unknown subscription plan handle")
	}
	if !apperror.APIPublicationValidationFailed.Is(err) {
		t.Fatalf("want APIPublicationValidationFailed, got %v", err)
	}
}

// TestPublicationDraft_NotFoundCases verifies the three distinct 404s: an
// unknown API Portal handle, an unknown (apiType, apiId) pair, and a
// not-yet-saved draft — each on its own dedicated code.
func TestPublicationDraft_NotFoundCases(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	svc := newPublicationTestService(tdb)

	if _, err := svc.GetDraft("rest-api", apiHandleFor(g), "no-such-portal", g.org); !apperror.APIPortalNotFound.Is(err) {
		t.Fatalf("want APIPortalNotFound for an unknown portal handle, got %v", err)
	}
	if _, err := svc.GetDraft("rest-api", "no-such-api", portalHandleFor(g), g.org); !apperror.APIPublicationAPINotFound.Is(err) {
		t.Fatalf("want APIPublicationAPINotFound for an unknown API handle, got %v", err)
	}
	if _, err := svc.GetDraft("rest-api", apiHandleFor(g), portalHandleFor(g), g.org); !apperror.APIPublicationDraftNotFound.Is(err) {
		t.Fatalf("want APIPublicationDraftNotFound before any draft is saved, got %v", err)
	}
	if err := svc.SaveDraftDefinition("rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "actor", "application/json", []byte("{}")); !apperror.APIPublicationDraftNotFound.Is(err) {
		t.Fatalf("want APIPublicationDraftNotFound saving content with no draft, got %v", err)
	}
}

// TestPublicationDraft_ContentRoundTrip covers the three content endpoints:
// definition, landing page (HTML-stripping) and thumbnail (content-sniffing),
// plus hasThumbnail/hasLandingPage flipping once content is saved.
func TestPublicationDraft_ContentRoundTrip(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	svc := newPublicationTestService(tdb)

	apiType, apiHandle, portalHandle := "rest-api", apiHandleFor(g), portalHandleFor(g)
	draft := &model.Publication{DisplayName: "x", Version: "1.0", AgentVisibility: "VISIBLE"}
	if _, err := svc.SaveDraftDetails(apiType, apiHandle, portalHandle, g.org, "actor", draft, nil, nil); err != nil {
		t.Fatalf("SaveDraftDetails failed: %v", err)
	}

	// Definition.
	definitionBytes := []byte(`{"openapi":"3.0.0"}`)
	if err := svc.SaveDraftDefinition(apiType, apiHandle, portalHandle, g.org, "actor", "application/json", definitionBytes); err != nil {
		t.Fatalf("SaveDraftDefinition failed: %v", err)
	}
	definition, err := svc.GetDraftDefinition(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("GetDraftDefinition failed: %v", err)
	}
	if string(definition.Content) != string(definitionBytes) {
		t.Fatalf("definition content did not round-trip")
	}
	if definition.ContentType != "application/json" || definition.FileName != "definition.json" {
		t.Fatalf("want content-type application/json / file definition.json, got %q / %q", definition.ContentType, definition.FileName)
	}

	// Landing page — embedded raw HTML (including a <script> element, whose
	// inner text must be dropped entirely) must be stripped before storage.
	landingPageInput := "# Title\n\nSome *markdown* text.\n<script>alert(1)</script>\n<b>bold html</b> after."
	if err := svc.SaveDraftLandingPage(apiType, apiHandle, portalHandle, g.org, "actor", []byte(landingPageInput)); err != nil {
		t.Fatalf("SaveDraftLandingPage failed: %v", err)
	}
	landingPage, err := svc.GetDraftLandingPage(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("GetDraftLandingPage failed: %v", err)
	}
	stored := string(landingPage.Content)
	if strings.Contains(stored, "<script>") || strings.Contains(stored, "alert(1)") || strings.Contains(stored, "<b>") {
		t.Fatalf("want embedded HTML (and script content) stripped, got %q", stored)
	}
	if !strings.Contains(stored, "# Title") || !strings.Contains(stored, "*markdown*") || !strings.Contains(stored, "bold html") {
		t.Fatalf("want ordinary Markdown/text preserved, got %q", stored)
	}
	if landingPage.ContentType != "text/markdown" {
		t.Fatalf("want content-type text/markdown, got %q", landingPage.ContentType)
	}

	// Thumbnail — content-sniffed from bytes, never a declared/extension type.
	pngBytes := []byte("\x89PNG\r\n\x1a\n0000000000")
	if err := svc.SaveDraftThumbnail(apiType, apiHandle, portalHandle, g.org, "actor", "icon.png", pngBytes); err != nil {
		t.Fatalf("SaveDraftThumbnail failed: %v", err)
	}
	thumbnail, err := svc.GetDraftThumbnail(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("GetDraftThumbnail failed: %v", err)
	}
	if thumbnail.ContentType != "image/png" {
		t.Fatalf("want sniffed content-type image/png, got %q", thumbnail.ContentType)
	}
	if string(thumbnail.Content) != string(pngBytes) {
		t.Fatalf("thumbnail content did not round-trip")
	}

	// A non-image upload must be rejected before it ever reaches storage.
	if err := svc.SaveDraftThumbnail(apiType, apiHandle, portalHandle, g.org, "actor", "not-an-image.txt", []byte("plain text")); !apperror.APIPublicationValidationFailed.Is(err) {
		t.Fatalf("want APIPublicationValidationFailed for a non-image thumbnail, got %v", err)
	}

	// The draft's own details GET must now report both flags.
	got, err := svc.GetDraft(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("GetDraft failed: %v", err)
	}
	if !got.HasThumbnail || !got.HasLandingPage {
		t.Fatalf("want hasThumbnail=true hasLandingPage=true after saving content, got %v / %v", got.HasThumbnail, got.HasLandingPage)
	}
}

// TestPublicationPublish_CreateRepublishNoOp drives the full publish
// lifecycle through the real service+repository stack: save draft, publish
// (create), GET publication shows it, edit + publish again (republish,
// merges into the existing live row — the anchor — in place, never leaving
// two live rows and never changing the anchor's uuid), then publish again
// with no changes (no-op refresh).
func TestPublicationPublish_CreateRepublishNoOp(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	svc := newPublicationTestService(tdb)

	apiType, apiHandle, portalHandle := "rest-api", apiHandleFor(g), portalHandleFor(g)

	draft := &model.Publication{DisplayName: "Version One", Version: "1.0", AgentVisibility: "VISIBLE"}
	savedDraft, err := svc.SaveDraftDetails(apiType, apiHandle, portalHandle, g.org, "actor", draft, nil, nil)
	if err != nil {
		t.Fatalf("SaveDraftDetails failed: %v", err)
	}
	if err := svc.SaveDraftDefinition(apiType, apiHandle, portalHandle, g.org, "actor", "application/json", []byte(minimalValidDefinition)); err != nil {
		t.Fatalf("SaveDraftDefinition failed: %v", err)
	}

	published, replaced, err := svc.Publish(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor")
	if err != nil {
		t.Fatalf("Publish (create) failed: %v", err)
	}
	if replaced {
		t.Fatalf("want replaced=false on a first publish")
	}
	if published.UUID != savedDraft.UUID {
		t.Fatalf("want the draft row promoted in place (same uuid), got draft=%s published=%s", savedDraft.UUID, published.UUID)
	}
	if published.Status != "PUBLISHED" || published.DisplayName != "Version One" {
		t.Fatalf("unexpected published row: %+v", published)
	}
	if _, err := svc.GetDraft(apiType, apiHandle, portalHandle, g.org); !apperror.APIPublicationDraftNotFound.Is(err) {
		t.Fatalf("want the draft consumed by promotion (DRAFT_NOT_FOUND), got %v", err)
	}
	got, err := svc.GetPublication(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("GetPublication failed: %v", err)
	}
	if got.DisplayName != "Version One" {
		t.Fatalf("GetPublication did not reflect the publish: %+v", got)
	}
	if tdb.count(t, "api_publications", "uuid", published.UUID) != 1 {
		t.Fatalf("want exactly one row for the promoted uuid")
	}

	// Edit, then republish: the draft must be merged into the existing
	// (anchor) row, not duplicated, and the anchor's uuid must not change.
	draft2 := &model.Publication{DisplayName: "Version Two", Version: "2.0", AgentVisibility: "VISIBLE"}
	if _, err := svc.SaveDraftDetails(apiType, apiHandle, portalHandle, g.org, "actor", draft2, nil, nil); err != nil {
		t.Fatalf("second SaveDraftDetails failed: %v", err)
	}
	// mergeDraftIntoAnchor replaces the anchor's content wholesale, so the
	// draft needs its own definition again too.
	if err := svc.SaveDraftDefinition(apiType, apiHandle, portalHandle, g.org, "actor", "application/json", []byte(minimalValidDefinition)); err != nil {
		t.Fatalf("second SaveDraftDefinition failed: %v", err)
	}
	republished, replaced, err := svc.Publish(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor")
	if err != nil {
		t.Fatalf("Publish (republish) failed: %v", err)
	}
	if !replaced {
		t.Fatalf("want replaced=true on a republish")
	}
	if republished.DisplayName != "Version Two" {
		t.Fatalf("want the republished content, got %+v", republished)
	}
	if republished.UUID != published.UUID {
		t.Fatalf("want the anchor uuid stable across a republish, got first=%s republish=%s", published.UUID, republished.UUID)
	}
	liveRows, err := svc.ListPublicationSummary(apiType, apiHandle, g.org, "", "", "")
	if err != nil {
		t.Fatalf("ListPublicationSummary failed: %v", err)
	}
	publishedCount := 0
	for _, row := range liveRows {
		if row.Status == "PUBLISHED" {
			publishedCount++
		}
	}
	if publishedCount != 1 {
		t.Fatalf("want exactly one PUBLISHED portal row after republish, got %d", publishedCount)
	}

	// Publish again with no intervening edit — a normal no-op refresh, not an error.
	if _, err := svc.SaveDraftDetails(apiType, apiHandle, portalHandle, g.org, "actor", draft2, nil, nil); err != nil {
		t.Fatalf("no-op resave failed: %v", err)
	}
	if err := svc.SaveDraftDefinition(apiType, apiHandle, portalHandle, g.org, "actor", "application/json", []byte(minimalValidDefinition)); err != nil {
		t.Fatalf("no-op definition resave failed: %v", err)
	}
	noOp, _, err := svc.Publish(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor")
	if err != nil {
		t.Fatalf("Publish (no-op refresh) failed: %v", err)
	}
	if noOp.DisplayName != "Version Two" {
		t.Fatalf("no-op refresh: unexpected content: %+v", noOp)
	}
	if noOp.UUID != published.UUID {
		t.Fatalf("want the anchor uuid stable across a no-op refresh, got first=%s noop=%s", published.UUID, noOp.UUID)
	}
}

// TestPublicationUnpublish_DemotesWhenNoDraft publishes, unpublishes, and
// confirms the demoted publication is now readable as the draft (same uuid,
// no content copy), while GetPublication 404s.
func TestPublicationUnpublish_DemotesWhenNoDraft(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	svc := newPublicationTestService(tdb)

	apiType, apiHandle, portalHandle := "rest-api", apiHandleFor(g), portalHandleFor(g)
	draft := &model.Publication{DisplayName: "Listing", Version: "1.0", AgentVisibility: "VISIBLE"}
	if _, err := svc.SaveDraftDetails(apiType, apiHandle, portalHandle, g.org, "actor", draft, nil, nil); err != nil {
		t.Fatalf("SaveDraftDetails failed: %v", err)
	}
	if err := svc.SaveDraftDefinition(apiType, apiHandle, portalHandle, g.org, "actor", "application/json", []byte(minimalValidDefinition)); err != nil {
		t.Fatalf("SaveDraftDefinition failed: %v", err)
	}
	published, _, err := svc.Publish(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor")
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	// Publish's promotion consumes the draft row — confirm no draft exists
	// going into unpublish, so the demote branch (not the delete branch) is
	// the one actually exercised here.
	if _, err := svc.GetDraft(apiType, apiHandle, portalHandle, g.org); !apperror.APIPublicationDraftNotFound.Is(err) {
		t.Fatalf("want no draft before unpublish, got %v", err)
	}

	if err := svc.Unpublish(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor"); err != nil {
		t.Fatalf("Unpublish failed: %v", err)
	}

	if _, err := svc.GetPublication(apiType, apiHandle, portalHandle, g.org); !apperror.APIPublicationNotFound.Is(err) {
		t.Fatalf("want APIPublicationNotFound after unpublish, got %v", err)
	}
	demoted, err := svc.GetDraft(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("GetDraft after unpublish failed: %v", err)
	}
	if demoted.UUID != published.UUID {
		t.Fatalf("want the live row demoted in place (same uuid), got live=%s draft=%s", published.UUID, demoted.UUID)
	}
	if demoted.DisplayName != "Listing" {
		t.Fatalf("want the demoted draft to hold the unpublished content, got %+v", demoted)
	}
	if tdb.count(t, "api_publications", "uuid", published.UUID) != 1 {
		t.Fatalf("want exactly one row for the demoted uuid")
	}
}

// unpublishConflictPublisher always rejects Unpublish with a
// *service.PortalConflictError, simulating a portal that still has active
// subscriptions/API keys attached to the listing.
type unpublishConflictPublisher struct{}

func (unpublishConflictPublisher) Publish(_ context.Context, _ *model.APIPortal, _, _ string, _ *model.Publication, _ *model.PublicationContent) error {
	return nil
}

func (unpublishConflictPublisher) Unpublish(_ context.Context, _ *model.APIPortal, _ string) error {
	return &service.PortalConflictError{Message: "active consumers still attached"}
}

func (unpublishConflictPublisher) CreateMissingPlans(_ context.Context, _ *model.APIPortal, _ []*model.SubscriptionPlan) error {
	return nil
}

func (unpublishConflictPublisher) Deprecate(_ context.Context, _ *model.APIPortal, _, _ string, _ *model.Publication) error {
	return nil
}

func (unpublishConflictPublisher) PublishContent(_ context.Context, _ *model.APIPortal, _ string, _ *service.PortalContent) error {
	return nil
}

// TestPublicationUnpublish_PortalConflict verifies a portal rejection (active
// consumers) maps to APIPublicationPortalConflict and leaves the live row
// untouched — the local write only happens after the portal call succeeds.
func TestPublicationUnpublish_PortalConflict(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	svc := service.NewPublicationService(
		repository.NewArtifactRepo(tdb.db),
		repository.NewAPIPortalRepo(tdb.db),
		repository.NewDocumentRepo(tdb.db),
		repository.NewSubscriptionPlanRepo(tdb.db),
		repository.NewPublicationRepo(tdb.db),
		unpublishConflictPublisher{},
		publicationTestOperationTimeout,
		0,
		nil,
	)

	apiType, apiHandle, portalHandle := "rest-api", apiHandleFor(g), portalHandleFor(g)
	draft := &model.Publication{DisplayName: "Listing", Version: "1.0", AgentVisibility: "VISIBLE"}
	if _, err := svc.SaveDraftDetails(apiType, apiHandle, portalHandle, g.org, "actor", draft, nil, nil); err != nil {
		t.Fatalf("SaveDraftDetails failed: %v", err)
	}
	if err := svc.SaveDraftDefinition(apiType, apiHandle, portalHandle, g.org, "actor", "application/json", []byte(minimalValidDefinition)); err != nil {
		t.Fatalf("SaveDraftDefinition failed: %v", err)
	}
	published, _, err := svc.Publish(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor")
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	err = svc.Unpublish(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor")
	if !apperror.APIPublicationPortalConflict.Is(err) {
		t.Fatalf("want APIPublicationPortalConflict, got %v", err)
	}
	// This mock leaves PortalConflictError.Reason unset (a plain Message, as
	// a PortalPublisher implementation predating Reason would) — the service
	// must fall back to the generic reason rather than substituting an empty
	// string into the message's %s slot.
	if !strings.Contains(err.Error(), "the conflict is resolved") {
		t.Fatalf("want the generic fallback reason for a conflict with no Reason set, got %v", err)
	}
	if tdb.count(t, "api_publications", "uuid", published.UUID) != 1 {
		t.Fatalf("want the live row untouched after a rejected unpublish")
	}
}

// unpublishSubscriptionConflictPublisher rejects Unpublish with a
// *service.PortalConflictError carrying the curated ERR_SUB_EXIST reason,
// simulating what HTTPPortalPublisher.Unpublish itself now produces when the
// portal's response names that known error code.
type unpublishSubscriptionConflictPublisher struct{}

func (unpublishSubscriptionConflictPublisher) Publish(_ context.Context, _ *model.APIPortal, _, _ string, _ *model.Publication, _ *model.PublicationContent) error {
	return nil
}

func (unpublishSubscriptionConflictPublisher) Unpublish(_ context.Context, _ *model.APIPortal, _ string) error {
	return &service.PortalConflictError{
		Message: "the API Portal rejected removal of this listing (status 409)",
		Reason:  "active subscriptions are removed",
	}
}

func (unpublishSubscriptionConflictPublisher) CreateMissingPlans(_ context.Context, _ *model.APIPortal, _ []*model.SubscriptionPlan) error {
	return nil
}

func (unpublishSubscriptionConflictPublisher) Deprecate(_ context.Context, _ *model.APIPortal, _, _ string, _ *model.Publication) error {
	return nil
}

func (unpublishSubscriptionConflictPublisher) PublishContent(_ context.Context, _ *model.APIPortal, _ string, _ *service.PortalContent) error {
	return nil
}

// TestPublicationUnpublish_PortalConflictReasonSurfaced verifies a curated
// conflict reason (not just the generic fallback) reaches the client-facing
// apperror message end to end through the service layer.
func TestPublicationUnpublish_PortalConflictReasonSurfaced(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	svc := service.NewPublicationService(
		repository.NewArtifactRepo(tdb.db),
		repository.NewAPIPortalRepo(tdb.db),
		repository.NewDocumentRepo(tdb.db),
		repository.NewSubscriptionPlanRepo(tdb.db),
		repository.NewPublicationRepo(tdb.db),
		unpublishSubscriptionConflictPublisher{},
		publicationTestOperationTimeout,
		0,
		nil,
	)

	apiType, apiHandle, portalHandle := "rest-api", apiHandleFor(g), portalHandleFor(g)
	draft := &model.Publication{DisplayName: "Listing", Version: "1.0", AgentVisibility: "VISIBLE"}
	if _, err := svc.SaveDraftDetails(apiType, apiHandle, portalHandle, g.org, "actor", draft, nil, nil); err != nil {
		t.Fatalf("SaveDraftDetails failed: %v", err)
	}
	if err := svc.SaveDraftDefinition(apiType, apiHandle, portalHandle, g.org, "actor", "application/json", []byte(minimalValidDefinition)); err != nil {
		t.Fatalf("SaveDraftDefinition failed: %v", err)
	}
	if _, _, err := svc.Publish(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor"); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	err := svc.Unpublish(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor")
	if !apperror.APIPublicationPortalConflict.Is(err) {
		t.Fatalf("want APIPublicationPortalConflict, got %v", err)
	}
	if !strings.Contains(err.Error(), "active subscriptions are removed") {
		t.Fatalf("want the curated ERR_SUB_EXIST reason in the message, got %v", err)
	}
}

// contentRecorder keeps the content archive Publish pushes.
type contentRecorder struct {
	alwaysSucceedsPortalPublisher
	archive []byte
}

func (c *contentRecorder) PublishContent(_ context.Context, _ *model.APIPortal, _ string, content *service.PortalContent) error {
	c.archive = content.Archive
	return nil
}

// TestPublish_PushesTheDraftLandingPage checks the stored landing page reaches the portal as
// web/apiContent.md in the content archive.
func TestPublish_PushesTheDraftLandingPage(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	recorder := &contentRecorder{}
	svc := newPublicationTestServiceWith(tdb, recorder)

	publishedListing(t, tdb, svc, g)

	zr, err := zip.NewReader(bytes.NewReader(recorder.archive), int64(len(recorder.archive)))
	if err != nil {
		t.Fatalf("pushed content is not a ZIP: %v", err)
	}
	for _, f := range zr.File {
		if f.Name != "content/web/apiContent.md" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		defer rc.Close()
		if body, _ := io.ReadAll(rc); string(body) != "# live landing page" {
			t.Fatalf("want the saved landing page, got %q", body)
		}
		return
	}
	t.Fatalf("pushed content has no web/apiContent.md")
}
