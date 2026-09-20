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

const deprecatePath = "/api/v0.9/api-portals/my-portal/apis/rest-api/my-api/deprecate"

func decodeBody(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("response is not JSON: %v: %s", err, raw)
	}
	return body
}

// Deprecate returns 409 for an API that was never published.
func TestPublicationHandler_Deprecate_NotPublished(t *testing.T) {
	r, _, cleanup := setupPublicationTestEnv(t)
	defer cleanup()

	w := doPublicationRequest(r, http.MethodPost, deprecatePath, "", nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d: %s", w.Code, w.Body.String())
	}
	body := decodeBody(t, w.Body.Bytes())
	if body["code"] != "PUBLICATION_STATE_CONFLICT" {
		t.Fatalf("want code PUBLICATION_STATE_CONFLICT, got %v", body)
	}
	if msg, _ := body["message"].(string); !strings.Contains(msg, "deprecated") {
		t.Fatalf("want a message naming the refused action, got %q", msg)
	}
}

// Deprecate returns 404 for an unknown API.
func TestPublicationHandler_Deprecate_UnknownAPI(t *testing.T) {
	r, _, cleanup := setupPublicationTestEnv(t)
	defer cleanup()

	w := doPublicationRequest(r, http.MethodPost, "/api/v0.9/api-portals/my-portal/apis/rest-api/no-such-api/deprecate", "", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d: %s", w.Code, w.Body.String())
	}
}

// Deprecate marks a published listing DEPRECATED and refuses a second call.
func TestPublicationHandler_Deprecate_MarksPublicationDeprecated(t *testing.T) {
	r, _, cleanup := setupPublicationTestEnv(t)
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

	w = doPublicationRequest(r, http.MethodPost, deprecatePath, "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("POST deprecate: want 200, got %d: %s", w.Code, w.Body.String())
	}
	body := decodeBody(t, w.Body.Bytes())
	if body["status"] != "DEPRECATED" || body["displayName"] != "My Listing" || body["version"] != "1.0" {
		t.Fatalf("POST deprecate: want the listing with status DEPRECATED, got %v", body)
	}

	w = doPublicationRequest(r, http.MethodGet, publicationPath, "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET publication: want 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := decodeBody(t, w.Body.Bytes())["status"]; got != "DEPRECATED" {
		t.Fatalf("GET publication: want status DEPRECATED, got %v", got)
	}

	w = doPublicationRequest(r, http.MethodPost, deprecatePath, "", nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("second POST deprecate: want 409, got %d: %s", w.Code, w.Body.String())
	}
}
