//go:build integration

/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
 *  KIND, either express or implied. See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

// newPublicationTestService wires a real PublicationService against it.db,
// using the same repositories production code uses. Unlike the rest of this
// package (which drives repositories directly), these tests go through the
// service layer: Slice 1's actual validation/resolution logic (handle
// resolution, unknown-handle rejection) lives there, not in the repository.
func newPublicationTestService(it *itDB) *service.PublicationService {
	return service.NewPublicationService(
		repository.NewArtifactRepo(it.db),
		repository.NewApiPortalRepo(it.db),
		repository.NewApiDocumentRepo(it.db),
		repository.NewSubscriptionPlanRepo(it.db),
		repository.NewPublicationRepo(it.db),
		service.NewStandInPortalPublisher(),
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

// TestPublicationDraft_SaveAndGetRoundTrip drives Slice 1's draft-details save
// through the real service+repository stack: PUT a complete draft — including
// a non-empty docIds and subscriptionPlanIds — GET it back, and confirm every
// field round-trips (Implementation_Plan.md's Slice 1 "Done when"), then save
// again to confirm the second save updates the same row rather than creating
// a second one, and correctly replaces (not merges) the mapping tables.
func TestPublicationDraft_SaveAndGetRoundTrip(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	g := seedOrgGraph(t, it)
	svc := newPublicationTestService(it)

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
		t.Fatalf("[%s] SaveDraftDetails failed: %v", it.driver, err)
	}
	if saved.UUID == "" {
		t.Fatalf("[%s] want a generated draft UUID", it.driver)
	}
	if len(saved.SubscriptionPlanIds) != 1 || saved.SubscriptionPlanIds[0] != planHandle {
		t.Fatalf("[%s] want subscriptionPlanIds [%s], got %v", it.driver, planHandle, saved.SubscriptionPlanIds)
	}
	if len(saved.DocIds) != 1 || saved.DocIds[0] != docHandle {
		t.Fatalf("[%s] want docIds [%s], got %v", it.driver, docHandle, saved.DocIds)
	}

	got, err := svc.GetDraft(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("[%s] GetDraft failed: %v", it.driver, err)
	}
	if got.UUID != saved.UUID {
		t.Fatalf("[%s] want GetDraft to return the saved row", it.driver)
	}
	if got.DisplayName != "My API Listing" || got.Version != "1.0.0" || got.Description != "desc" {
		t.Fatalf("[%s] core details did not round-trip: %+v", it.driver, got)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "a" || got.Tags[1] != "b" {
		t.Fatalf("[%s] tags did not round-trip: %v", it.driver, got.Tags)
	}
	if len(got.Labels) != 1 || got.Labels[0] != "partner" {
		t.Fatalf("[%s] labels did not round-trip: %v", it.driver, got.Labels)
	}
	if got.ProductionURL != "https://prod.example.com" || got.SandboxURL != "https://sandbox.example.com" {
		t.Fatalf("[%s] endpoints did not round-trip: %+v", it.driver, got)
	}
	if got.BusinessOwner != "Jane" || got.TechnicalOwner != "John" {
		t.Fatalf("[%s] owners did not round-trip: %+v", it.driver, got)
	}
	if len(got.SubscriptionPlanIds) != 1 || got.SubscriptionPlanIds[0] != planHandle {
		t.Fatalf("[%s] want subscriptionPlanIds [%s] on GET, got %v", it.driver, planHandle, got.SubscriptionPlanIds)
	}
	if len(got.DocIds) != 1 || got.DocIds[0] != docHandle {
		t.Fatalf("[%s] want docIds [%s] on GET, got %v", it.driver, docHandle, got.DocIds)
	}
	if got.HasThumbnail || got.HasLandingPage {
		t.Fatalf("[%s] want no content yet, got hasThumbnail=%v hasLandingPage=%v", it.driver, got.HasThumbnail, got.HasLandingPage)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("[%s] want non-zero audit timestamps", it.driver)
	}

	// Second save: same (artifact, portal) pairing must update the existing
	// draft row, not create a second one, and must fully replace (not merge)
	// the plan/document selections — an empty selection this time round.
	draft2 := &model.Publication{DisplayName: "Updated Listing", Version: "1.0.1", AgentVisibility: "VISIBLE"}
	saved2, err := svc.SaveDraftDetails(apiType, apiHandle, portalHandle, g.org, "actor-2", draft2, nil, nil)
	if err != nil {
		t.Fatalf("[%s] second SaveDraftDetails failed: %v", it.driver, err)
	}
	if saved2.UUID != saved.UUID {
		t.Fatalf("[%s] want the same draft row reused on a second save, got a different UUID", it.driver)
	}
	if len(saved2.SubscriptionPlanIds) != 0 {
		t.Fatalf("[%s] want subscriptionPlanIds cleared on an empty resave, got %v", it.driver, saved2.SubscriptionPlanIds)
	}
	if len(saved2.DocIds) != 0 {
		t.Fatalf("[%s] want docIds cleared on an empty resave, got %v", it.driver, saved2.DocIds)
	}
	if it.count(t, "api_publications", "uuid", saved.UUID) != 1 {
		t.Fatalf("[%s] want exactly one api_publications row for this pairing", it.driver)
	}
}

// TestPublicationDraft_UnknownDocHandleRejected verifies an unresolvable
// docIds handle is rejected — the FK from api_publication_doc_mappings.doc_uuid
// to api_documents(uuid) means this must be caught before any write.
func TestPublicationDraft_UnknownDocHandleRejected(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	g := seedOrgGraph(t, it)
	svc := newPublicationTestService(it)

	draft := &model.Publication{DisplayName: "x", Version: "1.0", AgentVisibility: "VISIBLE"}
	_, err := svc.SaveDraftDetails("rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "actor", draft, nil, []string{"does-not-exist"})
	if err == nil {
		t.Fatalf("[%s] want an error for an unknown document handle", it.driver)
	}
	if !apperror.APIPublicationValidationFailed.Is(err) {
		t.Fatalf("[%s] want APIPublicationValidationFailed, got %v", it.driver, err)
	}
}

// TestPublicationDraft_UnknownPlanHandleRejected mirrors the doc-handle case
// for subscriptionPlanIds.
func TestPublicationDraft_UnknownPlanHandleRejected(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	g := seedOrgGraph(t, it)
	svc := newPublicationTestService(it)

	draft := &model.Publication{DisplayName: "x", Version: "1.0", AgentVisibility: "VISIBLE"}
	_, err := svc.SaveDraftDetails("rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "actor", draft, []string{"does-not-exist"}, nil)
	if err == nil {
		t.Fatalf("[%s] want an error for an unknown subscription plan handle", it.driver)
	}
	if !apperror.APIPublicationValidationFailed.Is(err) {
		t.Fatalf("[%s] want APIPublicationValidationFailed, got %v", it.driver, err)
	}
}

// TestPublicationDraft_NotFoundCases verifies the three distinct 404s: an
// unknown API Portal handle, an unknown (apiType, apiId) pair, and a
// not-yet-saved draft — each on its own dedicated code, per REST_Design.md §11.
func TestPublicationDraft_NotFoundCases(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	g := seedOrgGraph(t, it)
	svc := newPublicationTestService(it)

	if _, err := svc.GetDraft("rest-api", apiHandleFor(g), "no-such-portal", g.org); !apperror.APIPublicationAPIPortalNotFound.Is(err) {
		t.Fatalf("[%s] want APIPublicationAPIPortalNotFound for an unknown portal handle, got %v", it.driver, err)
	}
	if _, err := svc.GetDraft("rest-api", "no-such-api", portalHandleFor(g), g.org); !apperror.APIPublicationAPINotFound.Is(err) {
		t.Fatalf("[%s] want APIPublicationAPINotFound for an unknown API handle, got %v", it.driver, err)
	}
	if _, err := svc.GetDraft("rest-api", apiHandleFor(g), portalHandleFor(g), g.org); !apperror.APIPublicationDraftNotFound.Is(err) {
		t.Fatalf("[%s] want APIPublicationDraftNotFound before any draft is saved, got %v", it.driver, err)
	}
	if err := svc.SaveDraftDefinition("rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "actor", "application/json", []byte("{}")); !apperror.APIPublicationDraftNotFound.Is(err) {
		t.Fatalf("[%s] want APIPublicationDraftNotFound saving content with no draft, got %v", it.driver, err)
	}
}

// TestPublicationDraft_ContentRoundTrip covers the three content endpoints:
// definition, landing page (HTML-stripping) and thumbnail (content-sniffing),
// plus hasThumbnail/hasLandingPage flipping once content is saved.
func TestPublicationDraft_ContentRoundTrip(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	g := seedOrgGraph(t, it)
	svc := newPublicationTestService(it)

	apiType, apiHandle, portalHandle := "rest-api", apiHandleFor(g), portalHandleFor(g)
	draft := &model.Publication{DisplayName: "x", Version: "1.0", AgentVisibility: "VISIBLE"}
	if _, err := svc.SaveDraftDetails(apiType, apiHandle, portalHandle, g.org, "actor", draft, nil, nil); err != nil {
		t.Fatalf("[%s] SaveDraftDetails failed: %v", it.driver, err)
	}

	// Definition.
	definitionBytes := []byte(`{"openapi":"3.0.0"}`)
	if err := svc.SaveDraftDefinition(apiType, apiHandle, portalHandle, g.org, "actor", "application/json", definitionBytes); err != nil {
		t.Fatalf("[%s] SaveDraftDefinition failed: %v", it.driver, err)
	}
	definition, err := svc.GetDraftDefinition(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("[%s] GetDraftDefinition failed: %v", it.driver, err)
	}
	if string(definition.Content) != string(definitionBytes) {
		t.Fatalf("[%s] definition content did not round-trip", it.driver)
	}
	if definition.ContentType != "application/json" || definition.FileName != "definition.json" {
		t.Fatalf("[%s] want content-type application/json / file definition.json, got %q / %q", it.driver, definition.ContentType, definition.FileName)
	}

	// Landing page — embedded raw HTML (including a <script> element, whose
	// inner text must be dropped entirely) must be stripped before storage.
	landingPageInput := "# Title\n\nSome *markdown* text.\n<script>alert(1)</script>\n<b>bold html</b> after."
	if err := svc.SaveDraftLandingPage(apiType, apiHandle, portalHandle, g.org, "actor", []byte(landingPageInput)); err != nil {
		t.Fatalf("[%s] SaveDraftLandingPage failed: %v", it.driver, err)
	}
	landingPage, err := svc.GetDraftLandingPage(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("[%s] GetDraftLandingPage failed: %v", it.driver, err)
	}
	stored := string(landingPage.Content)
	if strings.Contains(stored, "<script>") || strings.Contains(stored, "alert(1)") || strings.Contains(stored, "<b>") {
		t.Fatalf("[%s] want embedded HTML (and script content) stripped, got %q", it.driver, stored)
	}
	if !strings.Contains(stored, "# Title") || !strings.Contains(stored, "*markdown*") || !strings.Contains(stored, "bold html") {
		t.Fatalf("[%s] want ordinary Markdown/text preserved, got %q", it.driver, stored)
	}
	if landingPage.ContentType != "text/markdown" {
		t.Fatalf("[%s] want content-type text/markdown, got %q", it.driver, landingPage.ContentType)
	}

	// Thumbnail — content-sniffed from bytes, never a declared/extension type.
	pngBytes := []byte("\x89PNG\r\n\x1a\n0000000000")
	if err := svc.SaveDraftThumbnail(apiType, apiHandle, portalHandle, g.org, "actor", "icon.png", pngBytes); err != nil {
		t.Fatalf("[%s] SaveDraftThumbnail failed: %v", it.driver, err)
	}
	thumbnail, err := svc.GetDraftThumbnail(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("[%s] GetDraftThumbnail failed: %v", it.driver, err)
	}
	if thumbnail.ContentType != "image/png" {
		t.Fatalf("[%s] want sniffed content-type image/png, got %q", it.driver, thumbnail.ContentType)
	}
	if string(thumbnail.Content) != string(pngBytes) {
		t.Fatalf("[%s] thumbnail content did not round-trip", it.driver)
	}

	// A non-image upload must be rejected before it ever reaches storage.
	if err := svc.SaveDraftThumbnail(apiType, apiHandle, portalHandle, g.org, "actor", "not-an-image.txt", []byte("plain text")); !apperror.APIPublicationValidationFailed.Is(err) {
		t.Fatalf("[%s] want APIPublicationValidationFailed for a non-image thumbnail, got %v", it.driver, err)
	}

	// The draft's own details GET must now report both flags.
	got, err := svc.GetDraft(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("[%s] GetDraft failed: %v", it.driver, err)
	}
	if !got.HasThumbnail || !got.HasLandingPage {
		t.Fatalf("[%s] want hasThumbnail=true hasLandingPage=true after saving content, got %v / %v", it.driver, got.HasThumbnail, got.HasLandingPage)
	}
}

// TestPublicationPublish_DraftNotFound verifies Publish's defensive
// precondition — with no draft ever saved, DRAFT_NOT_FOUND is returned
// rather than publishing nothing.
func TestPublicationPublish_DraftNotFound(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	g := seedOrgGraph(t, it)
	svc := newPublicationTestService(it)

	_, _, err := svc.Publish(context.Background(), "rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "actor")
	if !apperror.APIPublicationDraftNotFound.Is(err) {
		t.Fatalf("[%s] want APIPublicationDraftNotFound, got %v", it.driver, err)
	}
}

// TestPublicationPublish_CreateRepublishNoOp drives Slice 5's full lifecycle
// through the real service+repository stack — the "Done when" from
// Implementation_Plan.md: save draft, publish (create), GET publication
// shows it, edit + publish again (republish, replaces the old row in place,
// never leaving two live rows), then publish again with no changes (no-op
// refresh).
func TestPublicationPublish_CreateRepublishNoOp(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	g := seedOrgGraph(t, it)
	svc := newPublicationTestService(it)

	apiType, apiHandle, portalHandle := "rest-api", apiHandleFor(g), portalHandleFor(g)

	draft := &model.Publication{DisplayName: "Version One", Version: "1.0", AgentVisibility: "VISIBLE"}
	savedDraft, err := svc.SaveDraftDetails(apiType, apiHandle, portalHandle, g.org, "actor", draft, nil, nil)
	if err != nil {
		t.Fatalf("[%s] SaveDraftDetails failed: %v", it.driver, err)
	}

	published, replaced, err := svc.Publish(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor")
	if err != nil {
		t.Fatalf("[%s] Publish (create) failed: %v", it.driver, err)
	}
	if replaced {
		t.Fatalf("[%s] want replaced=false on a first publish", it.driver)
	}
	if published.UUID != savedDraft.UUID {
		t.Fatalf("[%s] want the draft row promoted in place (same uuid), got draft=%s published=%s", it.driver, savedDraft.UUID, published.UUID)
	}
	if published.Status != "PUBLISHED" || published.DisplayName != "Version One" {
		t.Fatalf("[%s] unexpected published row: %+v", it.driver, published)
	}
	if _, err := svc.GetDraft(apiType, apiHandle, portalHandle, g.org); !apperror.APIPublicationDraftNotFound.Is(err) {
		t.Fatalf("[%s] want the draft consumed by promotion (DRAFT_NOT_FOUND), got %v", it.driver, err)
	}
	got, err := svc.GetPublication(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("[%s] GetPublication failed: %v", it.driver, err)
	}
	if got.DisplayName != "Version One" {
		t.Fatalf("[%s] GetPublication did not reflect the publish: %+v", it.driver, got)
	}
	if it.count(t, "api_publications", "uuid", published.UUID) != 1 {
		t.Fatalf("[%s] want exactly one row for the promoted uuid", it.driver)
	}

	// Edit, then republish: the old live row must be replaced, not duplicated.
	draft2 := &model.Publication{DisplayName: "Version Two", Version: "2.0", AgentVisibility: "VISIBLE"}
	if _, err := svc.SaveDraftDetails(apiType, apiHandle, portalHandle, g.org, "actor", draft2, nil, nil); err != nil {
		t.Fatalf("[%s] second SaveDraftDetails failed: %v", it.driver, err)
	}
	republished, replaced, err := svc.Publish(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor")
	if err != nil {
		t.Fatalf("[%s] Publish (republish) failed: %v", it.driver, err)
	}
	if !replaced {
		t.Fatalf("[%s] want replaced=true on a republish", it.driver)
	}
	if republished.DisplayName != "Version Two" {
		t.Fatalf("[%s] want the republished content, got %+v", it.driver, republished)
	}
	liveRows, err := svc.ListPublicationSummary(apiType, apiHandle, g.org, "", "", "")
	if err != nil {
		t.Fatalf("[%s] ListPublicationSummary failed: %v", it.driver, err)
	}
	publishedCount := 0
	for _, row := range liveRows {
		if row.Status == "PUBLISHED" {
			publishedCount++
		}
	}
	if publishedCount != 1 {
		t.Fatalf("[%s] want exactly one PUBLISHED portal row after republish, got %d", it.driver, publishedCount)
	}

	// Publish again with no intervening edit — a normal no-op refresh, not an error.
	if _, err := svc.SaveDraftDetails(apiType, apiHandle, portalHandle, g.org, "actor", draft2, nil, nil); err != nil {
		t.Fatalf("[%s] no-op resave failed: %v", it.driver, err)
	}
	noOp, _, err := svc.Publish(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor")
	if err != nil {
		t.Fatalf("[%s] Publish (no-op refresh) failed: %v", it.driver, err)
	}
	if noOp.DisplayName != "Version Two" {
		t.Fatalf("[%s] no-op refresh: unexpected content: %+v", it.driver, noOp)
	}
}

// TestPublicationUnpublish_NotLive verifies the 409 precondition — an API
// that was never published (no live row at all) cannot be unpublished.
func TestPublicationUnpublish_NotLive(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	g := seedOrgGraph(t, it)
	svc := newPublicationTestService(it)

	err := svc.Unpublish(context.Background(), "rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "actor")
	if !apperror.APIPublicationNotLive.Is(err) {
		t.Fatalf("[%s] want APIPublicationNotLive, got %v", it.driver, err)
	}
}

// TestPublicationUnpublish_DemotesWhenNoDraft drives Slice 6's "Done when":
// publish, unpublish, and confirm the demoted publication is now readable as
// the draft (same uuid, no content copy), while GetPublication 404s.
func TestPublicationUnpublish_DemotesWhenNoDraft(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	g := seedOrgGraph(t, it)
	svc := newPublicationTestService(it)

	apiType, apiHandle, portalHandle := "rest-api", apiHandleFor(g), portalHandleFor(g)
	draft := &model.Publication{DisplayName: "Listing", Version: "1.0", AgentVisibility: "VISIBLE"}
	if _, err := svc.SaveDraftDetails(apiType, apiHandle, portalHandle, g.org, "actor", draft, nil, nil); err != nil {
		t.Fatalf("[%s] SaveDraftDetails failed: %v", it.driver, err)
	}
	published, _, err := svc.Publish(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor")
	if err != nil {
		t.Fatalf("[%s] Publish failed: %v", it.driver, err)
	}
	// Publish's promotion consumes the draft row — confirm no draft exists
	// going into unpublish, so the demote branch (not the delete branch) is
	// the one actually exercised here.
	if _, err := svc.GetDraft(apiType, apiHandle, portalHandle, g.org); !apperror.APIPublicationDraftNotFound.Is(err) {
		t.Fatalf("[%s] want no draft before unpublish, got %v", it.driver, err)
	}

	if err := svc.Unpublish(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor"); err != nil {
		t.Fatalf("[%s] Unpublish failed: %v", it.driver, err)
	}

	if _, err := svc.GetPublication(apiType, apiHandle, portalHandle, g.org); !apperror.APIPublicationNotFound.Is(err) {
		t.Fatalf("[%s] want APIPublicationNotFound after unpublish, got %v", it.driver, err)
	}
	demoted, err := svc.GetDraft(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("[%s] GetDraft after unpublish failed: %v", it.driver, err)
	}
	if demoted.UUID != published.UUID {
		t.Fatalf("[%s] want the live row demoted in place (same uuid), got live=%s draft=%s", it.driver, published.UUID, demoted.UUID)
	}
	if demoted.DisplayName != "Listing" {
		t.Fatalf("[%s] want the demoted draft to hold the unpublished content, got %+v", it.driver, demoted)
	}
	if it.count(t, "api_publications", "uuid", published.UUID) != 1 {
		t.Fatalf("[%s] want exactly one row for the demoted uuid", it.driver)
	}
}

// TestPublicationUnpublish_DeletesWhenDraftExists covers the other branch: an
// in-progress draft must survive untouched, and the (now-superseded) live row
// must be deleted rather than clobbering it.
func TestPublicationUnpublish_DeletesWhenDraftExists(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	g := seedOrgGraph(t, it)
	svc := newPublicationTestService(it)

	apiType, apiHandle, portalHandle := "rest-api", apiHandleFor(g), portalHandleFor(g)
	draft := &model.Publication{DisplayName: "Live Listing", Version: "1.0", AgentVisibility: "VISIBLE"}
	if _, err := svc.SaveDraftDetails(apiType, apiHandle, portalHandle, g.org, "actor", draft, nil, nil); err != nil {
		t.Fatalf("[%s] SaveDraftDetails failed: %v", it.driver, err)
	}
	published, _, err := svc.Publish(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor")
	if err != nil {
		t.Fatalf("[%s] Publish failed: %v", it.driver, err)
	}

	// Start a fresh in-progress edit — this draft must survive unpublish
	// untouched, not be overwritten by the demoted publication content.
	inProgress := &model.Publication{DisplayName: "In-Progress Edit", Version: "2.0", AgentVisibility: "VISIBLE"}
	savedDraft, err := svc.SaveDraftDetails(apiType, apiHandle, portalHandle, g.org, "actor", inProgress, nil, nil)
	if err != nil {
		t.Fatalf("[%s] SaveDraftDetails (in-progress) failed: %v", it.driver, err)
	}

	if err := svc.Unpublish(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor"); err != nil {
		t.Fatalf("[%s] Unpublish failed: %v", it.driver, err)
	}

	if _, err := svc.GetPublication(apiType, apiHandle, portalHandle, g.org); !apperror.APIPublicationNotFound.Is(err) {
		t.Fatalf("[%s] want APIPublicationNotFound after unpublish, got %v", it.driver, err)
	}
	stillDraft, err := svc.GetDraft(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("[%s] GetDraft after unpublish failed: %v", it.driver, err)
	}
	if stillDraft.UUID != savedDraft.UUID || stillDraft.DisplayName != "In-Progress Edit" {
		t.Fatalf("[%s] want the pre-existing draft left untouched, got %+v", it.driver, stillDraft)
	}
	// The superseded live row must be gone, not left behind as a second row.
	if it.count(t, "api_publications", "uuid", published.UUID) != 0 {
		t.Fatalf("[%s] want the unpublished live row deleted", it.driver)
	}
	if it.count(t, "api_publications", "uuid", savedDraft.UUID) != 1 {
		t.Fatalf("[%s] want exactly one row for the surviving draft", it.driver)
	}
}

// unpublishConflictPublisher always rejects Unpublish with a
// *service.PortalConflictError, simulating a portal that still has active
// subscriptions/API keys attached to the listing.
type unpublishConflictPublisher struct{}

func (unpublishConflictPublisher) Publish(_ context.Context, _ *model.PublicationAPIPortal, _ string, _ *model.Publication, _ *model.PublicationContent) error {
	return nil
}

func (unpublishConflictPublisher) Unpublish(_ context.Context, _ *model.PublicationAPIPortal, _ string) error {
	return &service.PortalConflictError{Message: "active consumers still attached"}
}

// TestPublicationUnpublish_PortalConflict verifies a portal rejection (active
// consumers) maps to APIPublicationPortalConflict and leaves the live row
// untouched — the local write only happens after the portal call succeeds.
func TestPublicationUnpublish_PortalConflict(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	g := seedOrgGraph(t, it)
	svc := service.NewPublicationService(
		repository.NewArtifactRepo(it.db),
		repository.NewApiPortalRepo(it.db),
		repository.NewApiDocumentRepo(it.db),
		repository.NewSubscriptionPlanRepo(it.db),
		repository.NewPublicationRepo(it.db),
		unpublishConflictPublisher{},
		nil,
	)

	apiType, apiHandle, portalHandle := "rest-api", apiHandleFor(g), portalHandleFor(g)
	draft := &model.Publication{DisplayName: "Listing", Version: "1.0", AgentVisibility: "VISIBLE"}
	if _, err := svc.SaveDraftDetails(apiType, apiHandle, portalHandle, g.org, "actor", draft, nil, nil); err != nil {
		t.Fatalf("[%s] SaveDraftDetails failed: %v", it.driver, err)
	}
	published, _, err := svc.Publish(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor")
	if err != nil {
		t.Fatalf("[%s] Publish failed: %v", it.driver, err)
	}

	err = svc.Unpublish(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor")
	if !apperror.APIPublicationPortalConflict.Is(err) {
		t.Fatalf("[%s] want APIPublicationPortalConflict, got %v", it.driver, err)
	}
	if it.count(t, "api_publications", "uuid", published.UUID) != 1 {
		t.Fatalf("[%s] want the live row untouched after a rejected unpublish", it.driver)
	}
}
