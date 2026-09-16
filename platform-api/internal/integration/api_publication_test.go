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
