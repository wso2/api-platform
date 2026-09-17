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

package service

import (
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/client"
	"github.com/wso2/api-platform/platform-api/internal/model"

	"gopkg.in/yaml.v3"
)

// This package's TestMain (main_test.go) already initializes the shared
// outbound HTTP client that client.NewRetryableHTTPClient needs.

func newTestHTTPPortalPublisher(t *testing.T, sharedKey string) *HTTPPortalPublisher {
	t.Helper()
	retryClient, err := client.NewRetryableHTTPClient(0, 5*time.Second)
	if err != nil {
		t.Fatalf("NewRetryableHTTPClient: %v", err)
	}
	return &HTTPPortalPublisher{client: retryClient, sharedKey: sharedKey}
}

// assertMultipartParts drains r's multipart body against boundary, returning
// which named parts were present — verifies the metadata/definition part
// names REST_Design.md §8 documents, without asserting exact YAML content.
func assertMultipartParts(t *testing.T, r *http.Request) map[string]bool {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		t.Fatalf("want multipart content-type, got %q (err %v)", r.Header.Get("Content-Type"), err)
	}
	mr := multipart.NewReader(r.Body, params["boundary"])
	seen := map[string]bool{}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("NextPart: %v", err)
		}
		seen[part.FormName()] = true
	}
	return seen
}

// TestHTTPPortalPublisher_CreatesWhenNotFound verifies the existence-check
// (404) → create (POST /apis) path, the shared-key auth header, and that
// both the metadata and definition parts are sent.
func TestHTTPPortalPublisher_CreatesWhenNotFound(t *testing.T) {
	var gotMethod, gotAuth string
	var gotParts map[string]bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/apis/my-api":
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/apis":
			gotMethod = r.Method
			gotAuth = r.Header.Get("Authorization")
			gotParts = assertMultipartParts(t, r)
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	portal := &model.PublicationAPIPortal{URL: srv.URL}
	pub := &model.Publication{DisplayName: "X", Version: "1.0", AgentVisibility: "VISIBLE"}

	if err := p.Publish(context.Background(), portal, "my-api", pub, nil); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("want POST for create, got %s", gotMethod)
	}
	if gotAuth != "sharedkey test-shared-key" {
		t.Fatalf("want Authorization 'sharedkey test-shared-key', got %q", gotAuth)
	}
	if !gotParts["metadata"] || !gotParts["definition"] {
		t.Fatalf("want both metadata and definition parts, got %v", gotParts)
	}
}

// TestHTTPPortalPublisher_UpdatesWhenFound verifies the existence-check
// (200) → update (PUT /apis/{handle}) path.
func TestHTTPPortalPublisher_UpdatesWhenFound(t *testing.T) {
	var gotMethod, gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/apis/my-api":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPut && r.URL.Path == "/apis/my-api":
			gotMethod, gotPath = r.Method, r.URL.Path
			assertMultipartParts(t, r)
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	portal := &model.PublicationAPIPortal{URL: srv.URL}
	pub := &model.Publication{DisplayName: "X", Version: "1.0", AgentVisibility: "VISIBLE"}

	if err := p.Publish(context.Background(), portal, "my-api", pub, nil); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if gotMethod != http.MethodPut || gotPath != "/apis/my-api" {
		t.Fatalf("want PUT /apis/my-api for update, got %s %s", gotMethod, gotPath)
	}
}

// TestHTTPPortalPublisher_ConflictMapped verifies a 409 from the metadata
// push maps to *PortalConflictError, not a generic error.
func TestHTTPPortalPublisher_ConflictMapped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()

	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	portal := &model.PublicationAPIPortal{URL: srv.URL}
	pub := &model.Publication{DisplayName: "X", Version: "1.0", AgentVisibility: "VISIBLE"}

	err := p.Publish(context.Background(), portal, "my-api", pub, nil)
	var conflict *PortalConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("want *PortalConflictError, got %v", err)
	}
}

// TestHTTPPortalPublisher_NonConflictClientErrorMapped verifies a 4xx other
// than 409 from the metadata push — e.g. api-portal's 404 "subscription plan
// not found" when a draft references a plan the portal doesn't recognize —
// is also treated as non-retryable (*PortalConflictError), not bucketed into
// the generic 503-unavailable branch alongside a genuinely unreachable
// portal. Regression test: this case used to fall through to the default
// branch and surface as 503 PUBLICATION_PORTAL_UNAVAILABLE, even though
// retrying without fixing the draft's data could never succeed.
func TestHTTPPortalPublisher_NonConflictClientErrorMapped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound) // existence check: doesn't exist yet
			return
		}
		w.WriteHeader(http.StatusNotFound) // the create push itself rejected, e.g. unresolvable plan reference
	}))
	defer srv.Close()

	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	portal := &model.PublicationAPIPortal{URL: srv.URL}
	pub := &model.Publication{DisplayName: "X", Version: "1.0", AgentVisibility: "VISIBLE"}

	err := p.Publish(context.Background(), portal, "my-api", pub, nil)
	var conflict *PortalConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("want *PortalConflictError for a non-409 4xx, got %v", err)
	}
}

// TestHTTPPortalPublisher_Unpublish_ConflictReasonFromKnownCode verifies a
// portal response naming a known error code (ERR_SUB_EXIST, the real-world
// case: an API with active subscriptions can't be removed) resolves to its
// pre-approved reason phrase — never the portal's own raw error text.
func TestHTTPPortalPublisher_Unpublish_ConflictReasonFromKnownCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"status":"error","code":"CONFLICT","message":"ERR_SUB_EXIST","errors":[{"message":"API has subscriptions."}]}`))
	}))
	defer srv.Close()

	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	portal := &model.PublicationAPIPortal{URL: srv.URL}

	err := p.Unpublish(context.Background(), portal, "my-api")
	var conflict *PortalConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("want *PortalConflictError, got %v", err)
	}
	if conflict.Reason != "active subscriptions are removed" {
		t.Fatalf("want the curated ERR_SUB_EXIST reason, got %q", conflict.Reason)
	}
}

// TestHTTPPortalPublisher_ConflictReasonFallsBackForUnknownCode verifies a
// portal response naming an error code outside the known allowlist — or one
// that doesn't parse as the expected envelope at all — falls back to the
// generic reason rather than surfacing anything portal-specific.
func TestHTTPPortalPublisher_ConflictReasonFallsBackForUnknownCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`not even json`))
	}))
	defer srv.Close()

	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	portal := &model.PublicationAPIPortal{URL: srv.URL}
	pub := &model.Publication{DisplayName: "X", Version: "1.0", AgentVisibility: "VISIBLE"}

	err := p.Publish(context.Background(), portal, "my-api", pub, nil)
	var conflict *PortalConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("want *PortalConflictError, got %v", err)
	}
	if conflict.Reason != defaultPortalConflictReason {
		t.Fatalf("want the default fallback reason for an unparseable body, got %q", conflict.Reason)
	}
}

// TestHTTPPortalPublisher_UnavailableOnServerError verifies a persistent 5xx
// from the metadata push surfaces as a plain error (mapped by the service
// layer to 503, not 409) — never a *PortalConflictError.
func TestHTTPPortalPublisher_UnavailableOnServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	portal := &model.PublicationAPIPortal{URL: srv.URL}
	pub := &model.Publication{DisplayName: "X", Version: "1.0", AgentVisibility: "VISIBLE"}

	err := p.Publish(context.Background(), portal, "my-api", pub, nil)
	if err == nil {
		t.Fatal("want an error for a persistent 500")
	}
	var conflict *PortalConflictError
	if errors.As(err, &conflict) {
		t.Fatalf("want a plain error, not *PortalConflictError, for a 500")
	}
}

// TestHTTPPortalPublisher_SendsDefinitionContent verifies the definition
// part carries the draft's actual stored definition bytes/file name, not a
// placeholder, when one is provided.
func TestHTTPPortalPublisher_SendsDefinitionContent(t *testing.T) {
	var gotDefinitionBytes []byte
	var gotFileName string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("ParseMediaType: %v", err)
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("NextPart: %v", err)
			}
			if part.FormName() == "definition" {
				gotFileName = part.FileName()
				gotDefinitionBytes, _ = io.ReadAll(part)
			}
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	portal := &model.PublicationAPIPortal{URL: srv.URL}
	pub := &model.Publication{DisplayName: "X", Version: "1.0", AgentVisibility: "VISIBLE"}
	definition := &model.PublicationContent{
		FileName:    "definition.yaml",
		ContentType: "application/x-yaml",
		Content:     []byte("openapi: 3.0.0"),
	}

	if err := p.Publish(context.Background(), portal, "my-api", pub, definition); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if gotFileName != "definition.yaml" {
		t.Fatalf("want definition file name definition.yaml, got %q", gotFileName)
	}
	if string(gotDefinitionBytes) != "openapi: 3.0.0" {
		t.Fatalf("want definition content to round-trip, got %q", gotDefinitionBytes)
	}
}

// TestHTTPPortalPublisher_SubscriptionPlansAsPlainStringArray verifies the
// metadata YAML sends subscriptionPlans as a plain string array of plan
// handles (["Gold"]), never the {id: <handle>} object-array shape — the
// portal's own OpenAPI spec documents that object-array shape as valid only
// for a plain JSON metadata field, a different upload path from this YAML
// file part. Regression test for a real bug: sending {id: ...} objects here
// used to make the portal's YAML parser JS-stringify each entry
// (`String({id:"Gold"})` -> "[object Object]") before looking it up, so
// every plan reference 404'd as "Subscription plan not found" regardless of
// whether the handle itself was correct.
func TestHTTPPortalPublisher_SubscriptionPlansAsPlainStringArray(t *testing.T) {
	var gotYAML []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("ParseMediaType: %v", err)
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("NextPart: %v", err)
			}
			if part.FormName() == "metadata" {
				gotYAML, _ = io.ReadAll(part)
			}
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	portal := &model.PublicationAPIPortal{URL: srv.URL}
	pub := &model.Publication{
		DisplayName:         "X",
		Version:             "1.0",
		AgentVisibility:     "VISIBLE",
		SubscriptionPlanIds: []string{"Gold", "Silver"},
	}

	if err := p.Publish(context.Background(), portal, "my-api", pub, nil); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	var envelope struct {
		Spec struct {
			SubscriptionPlans []string `yaml:"subscriptionPlans"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(gotYAML, &envelope); err != nil {
		t.Fatalf("unmarshal sent YAML: %v\n%s", err, gotYAML)
	}
	want := []string{"Gold", "Silver"}
	got := envelope.Spec.SubscriptionPlans
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("want subscriptionPlans %v as a plain string array, got %v\nraw YAML:\n%s", want, got, gotYAML)
	}
}

// TestHTTPPortalPublisher_Unpublish_DeletesListing verifies the DELETE
// /apis/{handle} call and the shared-key auth header.
func TestHTTPPortalPublisher_Unpublish_DeletesListing(t *testing.T) {
	var gotMethod, gotPath, gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	portal := &model.PublicationAPIPortal{URL: srv.URL}

	if err := p.Unpublish(context.Background(), portal, "my-api"); err != nil {
		t.Fatalf("Unpublish: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/apis/my-api" {
		t.Fatalf("want DELETE /apis/my-api, got %s %s", gotMethod, gotPath)
	}
	if gotAuth != "sharedkey test-shared-key" {
		t.Fatalf("want Authorization 'sharedkey test-shared-key', got %q", gotAuth)
	}
}

// TestHTTPPortalPublisher_Unpublish_NotFoundIsSuccess verifies a 404 (already
// removed) is treated as success — the idempotency REST_Design.md §7
// "Retrying" promises for a retry after an already-successful removal.
func TestHTTPPortalPublisher_Unpublish_NotFoundIsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	portal := &model.PublicationAPIPortal{URL: srv.URL}

	if err := p.Unpublish(context.Background(), portal, "my-api"); err != nil {
		t.Fatalf("want a 404 treated as success, got %v", err)
	}
}

// TestHTTPPortalPublisher_Unpublish_ConflictMapped verifies a 409 (active
// consumers still attached) maps to *PortalConflictError, not a generic error.
func TestHTTPPortalPublisher_Unpublish_ConflictMapped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()

	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	portal := &model.PublicationAPIPortal{URL: srv.URL}

	err := p.Unpublish(context.Background(), portal, "my-api")
	var conflict *PortalConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("want *PortalConflictError, got %v", err)
	}
}

// TestHTTPPortalPublisher_Unpublish_UnavailableOnServerError verifies a
// persistent 5xx surfaces as a plain error (mapped by the service layer to
// 503, not 409) — never a *PortalConflictError.
func TestHTTPPortalPublisher_Unpublish_UnavailableOnServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	portal := &model.PublicationAPIPortal{URL: srv.URL}

	err := p.Unpublish(context.Background(), portal, "my-api")
	if err == nil {
		t.Fatal("want an error for a persistent 500")
	}
	var conflict *PortalConflictError
	if errors.As(err, &conflict) {
		t.Fatalf("want a plain error, not *PortalConflictError, for a 500")
	}
}
