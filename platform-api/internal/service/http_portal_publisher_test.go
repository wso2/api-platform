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

// newTestHTTPPortalPublisher wires a publisher whose auth registry always
// resolves to sharedKey, regardless of which portal is looked up —
// mockAPIPortalRepository.GetByHandleAndOrgID ignores its arguments and
// always returns the one fixed row below.
func newTestHTTPPortalPublisher(t *testing.T, sharedKey string) *HTTPPortalPublisher {
	t.Helper()
	retryClient, err := client.NewRetryableHTTPClient(0, 5*time.Second)
	if err != nil {
		t.Fatalf("NewRetryableHTTPClient: %v", err)
	}
	testVault := newTestVault(t)
	encryptedKey, err := testVault.Encrypt(context.Background(), sharedKey)
	if err != nil {
		t.Fatalf("encrypt test shared key: %v", err)
	}
	portalRepo := &mockAPIPortalRepository{getResult: &model.APIPortal{
		Handle:          "test-portal",
		OrganizationID:  "test-org",
		InternalAuthKey: encryptedKey,
	}}
	authRegistry := NewAPIPortalAuthRegistry(portalRepo, testVault)
	return &HTTPPortalPublisher{client: retryClient, authRegistry: authRegistry}
}

// assertMultipartParts drains r's multipart body against boundary, returning
// which named parts were present — verifies the metadata/definition part
// names the portal push sends, without asserting exact YAML content.
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
// (404) → create (POST {base}/apis) path, the shared-key auth header, and that
// both the metadata and definition parts are sent.
func TestHTTPPortalPublisher_CreatesWhenNotFound(t *testing.T) {
	var gotMethod, gotAuth string
	var gotParts map[string]bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == portalRESTBase+"/apis/my-api":
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == portalRESTBase+"/apis":
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
	portal := &model.APIPortal{URL: srv.URL}
	pub := &model.Publication{DisplayName: "X", Version: "1.0", AgentVisibility: "VISIBLE"}

	if err := p.Publish(context.Background(), portal, "my-api", pub, nil); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("want POST for create, got %s", gotMethod)
	}
	if gotAuth != "SharedKey test-shared-key" {
		t.Fatalf("want Authorization 'SharedKey test-shared-key', got %q", gotAuth)
	}
	if !gotParts["metadata"] || !gotParts["definition"] {
		t.Fatalf("want both metadata and definition parts, got %v", gotParts)
	}
}

// TestHTTPPortalPublisher_UpdatesWhenFound verifies the existence-check
// (200) → update (PUT {base}/apis/{handle}) path.
func TestHTTPPortalPublisher_UpdatesWhenFound(t *testing.T) {
	var gotMethod, gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == portalRESTBase+"/apis/my-api":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPut && r.URL.Path == portalRESTBase+"/apis/my-api":
			gotMethod, gotPath = r.Method, r.URL.Path
			assertMultipartParts(t, r)
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	portal := &model.APIPortal{URL: srv.URL}
	pub := &model.Publication{DisplayName: "X", Version: "1.0", AgentVisibility: "VISIBLE"}

	if err := p.Publish(context.Background(), portal, "my-api", pub, nil); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if gotMethod != http.MethodPut || gotPath != portalRESTBase+"/apis/my-api" {
		t.Fatalf("want PUT %s/apis/my-api for update, got %s %s", portalRESTBase, gotMethod, gotPath)
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
	portal := &model.APIPortal{URL: srv.URL}
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
	portal := &model.APIPortal{URL: srv.URL}
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
	portal := &model.APIPortal{URL: srv.URL}

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
	portal := &model.APIPortal{URL: srv.URL}
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
	portal := &model.APIPortal{URL: srv.URL}
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
	portal := &model.APIPortal{URL: srv.URL}
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
	portal := &model.APIPortal{URL: srv.URL}
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
	portal := &model.APIPortal{URL: srv.URL}

	if err := p.Unpublish(context.Background(), portal, "my-api"); err != nil {
		t.Fatalf("Unpublish: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != portalRESTBase+"/apis/my-api" {
		t.Fatalf("want DELETE %s/apis/my-api, got %s %s", portalRESTBase, gotMethod, gotPath)
	}
	if gotAuth != "SharedKey test-shared-key" {
		t.Fatalf("want Authorization 'SharedKey test-shared-key', got %q", gotAuth)
	}
}

// TestHTTPPortalPublisher_Unpublish_NotFoundIsSuccess verifies a 404 (already
// removed) is treated as success, so a retry after an already-successful
// removal converges rather than erroring.
func TestHTTPPortalPublisher_Unpublish_NotFoundIsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	portal := &model.APIPortal{URL: srv.URL}

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
	portal := &model.APIPortal{URL: srv.URL}

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
	portal := &model.APIPortal{URL: srv.URL}

	err := p.Unpublish(context.Background(), portal, "my-api")
	if err == nil {
		t.Fatal("want an error for a persistent 500")
	}
	var conflict *PortalConflictError
	if errors.As(err, &conflict) {
		t.Fatalf("want a plain error, not *PortalConflictError, for a 500")
	}
}

// TestHTTPPortalPublisher_AuthFailureIsNotConflict verifies a 401/403 from the
// portal (bad or unauthorized SharedKey) surfaces as a plain error — mapped
// by the service layer to 503 — never a *PortalConflictError (409), on every
// call that talks to the portal.
func TestHTTPPortalPublisher_AuthFailureIsNotConflict(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer srv.Close()

			p := newTestHTTPPortalPublisher(t, "test-shared-key")
			portal := &model.APIPortal{URL: srv.URL}
			pub := &model.Publication{DisplayName: "X", Version: "1.0", AgentVisibility: "VISIBLE"}

			calls := map[string]error{
				"publish (existence check)": p.Publish(context.Background(), portal, "my-api", pub, nil),
				"unpublish":                 p.Unpublish(context.Background(), portal, "my-api"),
				"deprecate":                 p.Deprecate(context.Background(), portal, "my-api", pub),
			}
			for name, err := range calls {
				if err == nil {
					t.Fatalf("%s: want an error for %d", name, status)
				}
				var conflict *PortalConflictError
				if errors.As(err, &conflict) {
					t.Fatalf("%s: want a plain error, not *PortalConflictError, for %d", name, status)
				}
			}
		})
	}
}

// TestHTTPPortalPublisher_PushAuthFailureIsNotConflict covers the metadata
// push itself (existence check passes, the push is refused with 401/403).
func TestHTTPPortalPublisher_PushAuthFailureIsNotConflict(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.WriteHeader(status)
			}))
			defer srv.Close()

			p := newTestHTTPPortalPublisher(t, "test-shared-key")
			portal := &model.APIPortal{URL: srv.URL}
			pub := &model.Publication{DisplayName: "X", Version: "1.0", AgentVisibility: "VISIBLE"}

			err := p.Publish(context.Background(), portal, "my-api", pub, nil)
			if err == nil {
				t.Fatalf("want an error for %d", status)
			}
			var conflict *PortalConflictError
			if errors.As(err, &conflict) {
				t.Fatalf("want a plain error, not *PortalConflictError, for %d", status)
			}
		})
	}
}

// liveListingForDeprecate returns a live row with every portal-overwritten field populated.
func liveListingForDeprecate() *model.Publication {
	return &model.Publication{
		Status:              model.PublicationStatusPublished,
		DisplayName:         "Orders",
		Version:             "1.0",
		Description:         "Order management",
		Tags:                []string{"payments"},
		Labels:              []string{"finance"},
		AgentVisibility:     "HIDDEN",
		ProductionURL:       "https://prod.example.com",
		SandboxURL:          "https://sandbox.example.com",
		BusinessOwner:       "Bo",
		BusinessOwnerEmail:  "bo@example.com",
		TechnicalOwner:      "To",
		TechnicalOwnerEmail: "to@example.com",
		SubscriptionPlanIds: []string{"Gold"},
	}
}

// Deprecate sends a single PUT with the full live metadata, status DEPRECATED, and no
// definition part.
func TestHTTPPortalPublisher_Deprecate_SendsLiveListingWithDeprecatedStatus(t *testing.T) {
	var requests []string
	var gotAuth string
	var gotParts map[string]bool
	var gotYAML []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		gotAuth = r.Header.Get("Authorization")

		_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("ParseMediaType: %v", err)
		}
		gotParts = map[string]bool{}
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("NextPart: %v", err)
			}
			gotParts[part.FormName()] = true
			if part.FormName() == "metadata" {
				gotYAML, _ = io.ReadAll(part)
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	portal := &model.APIPortal{URL: srv.URL}

	if err := p.Deprecate(context.Background(), portal, "my-api", liveListingForDeprecate()); err != nil {
		t.Fatalf("Deprecate: %v", err)
	}
	if len(requests) != 1 || requests[0] != "PUT "+portalRESTBase+"/apis/my-api" {
		t.Fatalf("want exactly one PUT %s/apis/my-api, got %v", portalRESTBase, requests)
	}
	if gotAuth != "SharedKey test-shared-key" {
		t.Fatalf("want Authorization 'SharedKey test-shared-key', got %q", gotAuth)
	}
	if len(gotParts) != 1 || !gotParts["metadata"] {
		t.Fatalf("want only the metadata part (no definition), got %v", gotParts)
	}

	var envelope struct {
		Metadata struct {
			Name string `yaml:"name"`
		} `yaml:"metadata"`
		Spec struct {
			Type              string   `yaml:"type"`
			DisplayName       string   `yaml:"displayName"`
			Version           string   `yaml:"version"`
			Description       string   `yaml:"description"`
			Status            string   `yaml:"status"`
			AgentVisibility   string   `yaml:"agentVisibility"`
			Tags              []string `yaml:"tags"`
			Labels            []string `yaml:"labels"`
			ReferenceID       string   `yaml:"referenceId"`
			SubscriptionPlans []string `yaml:"subscriptionPlans"`
			Endpoints         struct {
				ProductionURL string `yaml:"productionUrl"`
				SandboxURL    string `yaml:"sandboxUrl"`
			} `yaml:"endpoints"`
			BusinessInformation struct {
				BusinessOwner       string `yaml:"businessOwner"`
				BusinessOwnerEmail  string `yaml:"businessOwnerEmail"`
				TechnicalOwner      string `yaml:"technicalOwner"`
				TechnicalOwnerEmail string `yaml:"technicalOwnerEmail"`
			} `yaml:"businessInformation"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(gotYAML, &envelope); err != nil {
		t.Fatalf("unmarshal sent YAML: %v\n%s", err, gotYAML)
	}
	spec := envelope.Spec
	if spec.Status != "DEPRECATED" {
		t.Fatalf("want status DEPRECATED, got %q", spec.Status)
	}
	if envelope.Metadata.Name != "my-api" || spec.ReferenceID != "my-api" || spec.Type != "REST" {
		t.Fatalf("want identity fields resent, got name=%q referenceId=%q type=%q", envelope.Metadata.Name, spec.ReferenceID, spec.Type)
	}
	if spec.DisplayName != "Orders" || spec.Version != "1.0" || spec.Description != "Order management" || spec.AgentVisibility != "HIDDEN" {
		t.Fatalf("want live core fields resent unchanged, got %+v", spec)
	}
	if len(spec.Tags) != 1 || spec.Tags[0] != "payments" || len(spec.Labels) != 1 || spec.Labels[0] != "finance" ||
		len(spec.SubscriptionPlans) != 1 || spec.SubscriptionPlans[0] != "Gold" {
		t.Fatalf("want tags, labels and plans resent (the portal replaces them), got %+v", spec)
	}
	if spec.Endpoints.ProductionURL != "https://prod.example.com" || spec.Endpoints.SandboxURL != "https://sandbox.example.com" {
		t.Fatalf("want endpoints resent, got %+v", spec.Endpoints)
	}
	if spec.BusinessInformation.BusinessOwner != "Bo" || spec.BusinessInformation.BusinessOwnerEmail != "bo@example.com" ||
		spec.BusinessInformation.TechnicalOwner != "To" || spec.BusinessInformation.TechnicalOwnerEmail != "to@example.com" {
		t.Fatalf("want owner contacts resent (the portal nulls omitted ones), got %+v", spec.BusinessInformation)
	}
}

// Deprecate maps a non-auth 4xx to *PortalConflictError and a server error to a plain
// error. 401/403 are covered by TestHTTPPortalPublisher_AuthFailureIsNotConflict.
func TestHTTPPortalPublisher_Deprecate_ErrorMapping(t *testing.T) {
	cases := []struct {
		status       int
		wantConflict bool
	}{
		{http.StatusBadRequest, true},
		{http.StatusNotFound, true},
		{http.StatusConflict, true},
		{http.StatusInternalServerError, false},
		{http.StatusBadGateway, false},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()

			p := newTestHTTPPortalPublisher(t, "test-shared-key")
			err := p.Deprecate(context.Background(), &model.APIPortal{URL: srv.URL}, "my-api", liveListingForDeprecate())
			if err == nil {
				t.Fatalf("want an error for %d", tc.status)
			}
			var conflict *PortalConflictError
			if got := errors.As(err, &conflict); got != tc.wantConflict {
				t.Fatalf("status %d: want conflict=%v, got %v (%v)", tc.status, tc.wantConflict, got, err)
			}
		})
	}
}

// Deprecate path-escapes the API handle.
func TestHTTPPortalPublisher_Deprecate_EscapesHandleInPath(t *testing.T) {
	var gotEscapedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEscapedPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	if err := p.Deprecate(context.Background(), &model.APIPortal{URL: srv.URL}, "a/../b c", liveListingForDeprecate()); err != nil {
		t.Fatalf("Deprecate: %v", err)
	}
	if want := portalRESTBase + "/apis/a%2F..%2Fb%20c"; gotEscapedPath != want {
		t.Fatalf("want escaped path %q, got %q", want, gotEscapedPath)
	}
}

// The portal serves its REST API under /api-portal/api/v0.9, not at its root.
func TestPortalURLsIncludeTheRESTBase(t *testing.T) {
	for _, root := range []string{"https://portal.example.com", "https://portal.example.com/"} {
		portal := &model.APIPortal{URL: root}
		if got, want := apisURL(portal), "https://portal.example.com/api-portal/api/v0.9/apis"; got != want {
			t.Errorf("apisURL(%q) = %q, want %q", root, got, want)
		}
		if got, want := apiURL(portal, "my-api"), "https://portal.example.com/api-portal/api/v0.9/apis/my-api"; got != want {
			t.Errorf("apiURL(%q) = %q, want %q", root, got, want)
		}
	}
}
