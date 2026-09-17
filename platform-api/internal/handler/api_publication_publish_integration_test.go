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
	"encoding/json"
	"net/http"
	"testing"
)

const publishPath = "/api/v0.9/api-portals/my-portal/apis/rest-api/my-api/publish"

// TestPublicationHandler_Publish_DraftNotFound verifies the defensive
// DRAFT_NOT_FOUND path: calling /publish directly, with no draft ever saved,
// must be rejected rather than publish nothing. In the real UI flow this is
// unreachable (the client always saves the draft immediately before calling
// publish) — this test exercises the fail-closed guard, not the normal path.
func TestPublicationHandler_Publish_DraftNotFound(t *testing.T) {
	r, _, cleanup := setupPublicationTestEnv(t)
	defer cleanup()

	w := doPublicationRequest(r, http.MethodPost, publishPath, "", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["code"] != "DRAFT_NOT_FOUND" {
		t.Fatalf("want code DRAFT_NOT_FOUND, got %v", body)
	}
}

// TestPublicationHandler_Publish_CreatesFromDraft drives the client's real
// sequence — draft PUT, then bodyless publish — for a first-ever publish:
// 201 Created with a Location header, the draft is consumed (promoted, not
// copied), and GET publication reflects exactly what was in the draft.
func TestPublicationHandler_Publish_CreatesFromDraft(t *testing.T) {
	r, db, cleanup := setupPublicationTestEnv(t)
	defer cleanup()

	// Step 1 of the client's publish flow: save the draft (same as "Save Draft").
	w := doPublicationRequest(r, http.MethodPut, draftPath, "application/json",
		[]byte(`{"displayName":"My Listing","version":"1.0","subscriptionPlanIds":["gold"],"docIds":["quickstart"]}`))
	if w.Code != http.StatusOK {
		t.Fatalf("PUT draft: want 200, got %d: %s", w.Code, w.Body.String())
	}

	// Step 2: the bodyless publish action.
	w = doPublicationRequest(r, http.MethodPost, publishPath, "", nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST publish: want 201, got %d: %s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != publicationPath {
		t.Fatalf("POST publish: want Location %q, got %q", publicationPath, loc)
	}
	var published map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &published)
	if published["displayName"] != "My Listing" || published["status"] != "PUBLISHED" {
		t.Fatalf("POST publish: unexpected body: %v", published)
	}
	plans, _ := published["subscriptionPlanIds"].([]any)
	if len(plans) != 1 || plans[0] != "gold" {
		t.Fatalf("POST publish: want subscriptionPlanIds [gold], got %v", published["subscriptionPlanIds"])
	}

	// GET publication shows it.
	w = doPublicationRequest(r, http.MethodGet, publicationPath, "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET publication: want 200, got %d: %s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["displayName"] != "My Listing" {
		t.Fatalf("GET publication: did not return the published row: %v", got)
	}

	// The draft was promoted in place, not copied — no draft row remains.
	w = doPublicationRequest(r, http.MethodGet, draftPath, "", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GET draft after publish: want 404 (draft consumed by promotion), got %d: %s", w.Code, w.Body.String())
	}

	// Exactly one live row backs this — the promotion flips the existing
	// row rather than inserting a second one.
	var liveCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM api_publications WHERE artifact_uuid = 'api-artifact-1' AND api_portal_uuid = 'portal-1' AND is_draft = 0`).Scan(&liveCount); err != nil {
		t.Fatalf("count live rows: %v", err)
	}
	if liveCount != 1 {
		t.Fatalf("want exactly 1 live row after first publish, got %d", liveCount)
	}
}

// TestPublicationHandler_Publish_RepublishReplacesOldRow verifies the
// merged-table promotion for a republish: edit + publish again must merge
// the new draft's content into the existing live row (the anchor) in place —
// never leaving two live rows, never changing the anchor's id, and returning
// 200 (not 201) since an existing listing was updated rather than created.
func TestPublicationHandler_Publish_RepublishReplacesOldRow(t *testing.T) {
	r, db, cleanup := setupPublicationTestEnv(t)
	defer cleanup()

	// First publish.
	w := doPublicationRequest(r, http.MethodPut, draftPath, "application/json",
		[]byte(`{"displayName":"Version One","version":"1.0"}`))
	if w.Code != http.StatusOK {
		t.Fatalf("PUT draft #1: want 200, got %d: %s", w.Code, w.Body.String())
	}
	w = doPublicationRequest(r, http.MethodPost, publishPath, "", nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST publish #1: want 201, got %d: %s", w.Code, w.Body.String())
	}
	// The publication uuid is internal bookkeeping, never returned over the
	// wire (api.Publication carries no id field) — read the anchor's uuid
	// straight from the DB, the same way liveCount below does.
	anchorID := livePublicationUUID(t, db)

	// Edit, then republish.
	w = doPublicationRequest(r, http.MethodPut, draftPath, "application/json",
		[]byte(`{"displayName":"Version Two","version":"2.0"}`))
	if w.Code != http.StatusOK {
		t.Fatalf("PUT draft #2: want 200, got %d: %s", w.Code, w.Body.String())
	}
	w = doPublicationRequest(r, http.MethodPost, publishPath, "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("POST publish #2 (republish): want 200, got %d: %s", w.Code, w.Body.String())
	}
	var published map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &published)
	if published["displayName"] != "Version Two" {
		t.Fatalf("POST publish #2: want the updated content, got %v", published)
	}
	if got := livePublicationUUID(t, db); got != anchorID {
		t.Fatalf("POST publish #2: want the anchor uuid to stay stable across a republish, first=%s republish=%s", anchorID, got)
	}

	w = doPublicationRequest(r, http.MethodGet, publicationPath, "", nil)
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["displayName"] != "Version Two" {
		t.Fatalf("GET publication: want the republished content, got %v", got)
	}

	// Still exactly one live row — the old one was deleted, not left behind.
	var liveCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM api_publications WHERE artifact_uuid = 'api-artifact-1' AND api_portal_uuid = 'portal-1' AND is_draft = 0`).Scan(&liveCount); err != nil {
		t.Fatalf("count live rows: %v", err)
	}
	if liveCount != 1 {
		t.Fatalf("want exactly 1 live row after republish (old one replaced), got %d", liveCount)
	}
}

// TestPublicationHandler_Publish_NoOpRefresh verifies a repeat publish with
// no intervening edit is a normal no-op refresh, not an error — the client
// still calls draft PUT (with the same content) before publish every time.
func TestPublicationHandler_Publish_NoOpRefresh(t *testing.T) {
	r, _, cleanup := setupPublicationTestEnv(t)
	defer cleanup()

	body := []byte(`{"displayName":"Same Listing","version":"1.0"}`)
	w := doPublicationRequest(r, http.MethodPut, draftPath, "application/json", body)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT draft #1: want 200, got %d: %s", w.Code, w.Body.String())
	}
	w = doPublicationRequest(r, http.MethodPost, publishPath, "", nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST publish #1: want 201, got %d: %s", w.Code, w.Body.String())
	}

	// Re-save the identical draft content, then publish again with no changes.
	w = doPublicationRequest(r, http.MethodPut, draftPath, "application/json", body)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT draft #2 (no-op): want 200, got %d: %s", w.Code, w.Body.String())
	}
	w = doPublicationRequest(r, http.MethodPost, publishPath, "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("POST publish #2 (no-op refresh): want 200, got %d: %s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["displayName"] != "Same Listing" {
		t.Fatalf("POST publish #2: unexpected body: %v", got)
	}
}
