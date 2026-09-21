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
	"strings"
	"testing"
)

const unpublishPath = "/api/v0.9/api-portals/my-portal/apis/rest-api/my-api/unpublish"

// TestPublicationHandler_Unpublish_NotLive verifies the 409 precondition — an
// API that was never published cannot be unpublished.
func TestPublicationHandler_Unpublish_NotLive(t *testing.T) {
	r, _, cleanup := setupPublicationTestEnv(t)
	defer cleanup()

	w := doPublicationRequest(r, http.MethodPost, unpublishPath, "", nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["code"] != "PUBLICATION_STATE_CONFLICT" {
		t.Fatalf("want code PUBLICATION_STATE_CONFLICT, got %v", body)
	}
	if msg, _ := body["message"].(string); !strings.Contains(msg, "unpublished") {
		t.Fatalf("want a message naming the refused action, got %q", msg)
	}
}

// TestPublicationHandler_Unpublish_DemotesWhenNoDraft publishes, then
// unpublishes, and confirms the demoted publication is now readable as the
// draft (same content, no copy), while GET publication 404s.
func TestPublicationHandler_Unpublish_DemotesWhenNoDraft(t *testing.T) {
	r, db, cleanup := setupPublicationTestEnv(t)
	defer cleanup()

	w := doPublicationRequest(r, http.MethodPut, draftPath, "application/json",
		[]byte(`{"displayName":"My Listing","version":"1.0"}`))
	if w.Code != http.StatusOK {
		t.Fatalf("PUT draft: want 200, got %d: %s", w.Code, w.Body.String())
	}
	w = doPublicationRequest(r, http.MethodPost, publishPath, "", nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST publish: want 201, got %d: %s", w.Code, w.Body.String())
	}
	// Promotion consumed the draft — nothing to demote-collide with.
	w = doPublicationRequest(r, http.MethodGet, draftPath, "", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GET draft before unpublish: want 404 (consumed by promotion), got %d", w.Code)
	}

	w = doPublicationRequest(r, http.MethodPost, unpublishPath, "", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("POST unpublish: want 204, got %d: %s", w.Code, w.Body.String())
	}

	w = doPublicationRequest(r, http.MethodGet, publicationPath, "", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GET publication after unpublish: want 404, got %d: %s", w.Code, w.Body.String())
	}
	w = doPublicationRequest(r, http.MethodGet, draftPath, "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET draft after unpublish: want 200 (demoted), got %d: %s", w.Code, w.Body.String())
	}
	var demoted map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &demoted)
	if demoted["displayName"] != "My Listing" {
		t.Fatalf("GET draft after unpublish: want the demoted content, got %v", demoted)
	}

	var liveCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM api_publications WHERE artifact_uuid = 'api-artifact-1' AND api_portal_uuid = 'portal-1'`).Scan(&liveCount); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if liveCount != 1 {
		t.Fatalf("want exactly 1 row after demote (same row flipped), got %d", liveCount)
	}
}

// TestPublicationHandler_Unpublish_DeletesWhenDraftExists covers the other
// branch: when an in-progress draft already exists as its own row, unpublish
// merges its content into the anchor (the live row being unpublished)
// instead of leaving the draft's own row as the survivor — the anchor's id
// is the durable identity for this pairing and must not change.
func TestPublicationHandler_Unpublish_DeletesWhenDraftExists(t *testing.T) {
	r, db, cleanup := setupPublicationTestEnv(t)
	defer cleanup()

	w := doPublicationRequest(r, http.MethodPut, draftPath, "application/json",
		[]byte(`{"displayName":"Live Listing","version":"1.0"}`))
	if w.Code != http.StatusOK {
		t.Fatalf("PUT draft: want 200, got %d: %s", w.Code, w.Body.String())
	}
	w = doPublicationRequest(r, http.MethodPost, publishPath, "", nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST publish: want 201, got %d: %s", w.Code, w.Body.String())
	}
	// The publication uuid is internal bookkeeping, never returned over the
	// wire (api.Publication has no id field) — read it straight from the DB.
	anchorID := livePublicationUUID(t, db)

	// Start a fresh in-progress edit — its own, separate row/uuid until merged.
	w = doPublicationRequest(r, http.MethodPut, draftPath, "application/json",
		[]byte(`{"displayName":"In-Progress Edit","version":"2.0"}`))
	if w.Code != http.StatusOK {
		t.Fatalf("PUT draft (in-progress): want 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := draftPublicationUUID(t, db); got == anchorID {
		t.Fatalf("PUT draft (in-progress): want a different uuid from the anchor before unpublish, got %s", got)
	}

	w = doPublicationRequest(r, http.MethodPost, unpublishPath, "", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("POST unpublish: want 204, got %d: %s", w.Code, w.Body.String())
	}

	w = doPublicationRequest(r, http.MethodGet, publicationPath, "", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GET publication after unpublish: want 404, got %d: %s", w.Code, w.Body.String())
	}
	w = doPublicationRequest(r, http.MethodGet, draftPath, "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET draft after unpublish: want 200, got %d: %s", w.Code, w.Body.String())
	}
	var stillDraft map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &stillDraft)
	if stillDraft["displayName"] != "In-Progress Edit" {
		t.Fatalf("GET draft after unpublish: want the in-progress content merged in, got %v", stillDraft)
	}
	if got := draftPublicationUUID(t, db); got != anchorID {
		t.Fatalf("GET draft after unpublish: want the anchor uuid to survive, anchor=%s got=%s", anchorID, got)
	}

	var rowCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM api_publications WHERE artifact_uuid = 'api-artifact-1' AND api_portal_uuid = 'portal-1'`).Scan(&rowCount); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("want exactly 1 row (the anchor survives, merged in place; the in-progress draft's own row discarded), got %d", rowCount)
	}
}
