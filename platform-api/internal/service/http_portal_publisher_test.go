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
