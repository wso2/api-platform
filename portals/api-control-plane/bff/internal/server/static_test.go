/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	indexBody  = "<!doctype html><title>spa</title>"
	assetBody  = "console.log('app');"
	secretBody = "TOP-SECRET-OUTSIDE-STATIC-DIR"
)

// newStaticTestDir lays out a temp tree where the secret file lives OUTSIDE the
// served static dir, so any response echoing secretBody proves a traversal escape:
//
//	<tmp>/secret.txt        <- must never be served
//	<tmp>/static/index.html <- SPA entrypoint
//	<tmp>/static/assets/app.js
func newStaticTestDir(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "secret.txt"), []byte(secretBody), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	staticDir := filepath.Join(tmp, "static")
	if err := os.MkdirAll(filepath.Join(staticDir, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir static: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte(indexBody), 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "assets", "app.js"), []byte(assetBody), 0o600); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	return staticDir
}

func TestSPAHandlerServesHashedAssetCacheable(t *testing.T) {
	h := spaHandler(newStaticTestDir(t))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != assetBody {
		t.Errorf("body = %q, want %q", got, assetBody)
	}
	// Hashed assets must not be marked no-store so they stay cacheable.
	if cc := rec.Header().Get("Cache-Control"); cc == "no-store" {
		t.Errorf("Cache-Control = %q, want asset to be cacheable", cc)
	}
}

func TestSPAHandlerServesIndexNoStore(t *testing.T) {
	h := spaHandler(newStaticTestDir(t))

	// The SPA root and any client-side route with no matching file both resolve
	// to index.html served no-store (replaces nginx try_files $uri /index.html).
	for _, path := range []string{"/", "/dashboard/settings"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if got := rec.Body.String(); got != indexBody {
				t.Errorf("body = %q, want index fallback %q", got, indexBody)
			}
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", rec.Header().Get("Cache-Control"))
			}
		})
	}
}

// TestSPAHandlerPathTraversalContained is the regression test for the reported
// (false-positive) traversal sink: adversarial paths must never resolve the
// secret file outside staticDir. filepath.Clean on the root-absolute URL path
// collapses every escaping "..", so each payload resolves inside staticDir,
// misses, and falls back to index.html.
func TestSPAHandlerPathTraversalContained(t *testing.T) {
	h := spaHandler(newStaticTestDir(t))

	payloads := []string{
		"/../secret.txt",
		"/../../secret.txt",
		"/../../../../../../secret.txt",
		"/assets/../../secret.txt",
		"/%2e%2e%2fsecret.txt",             // encoded ../
		"/..%2f..%2fsecret.txt",            // encoded ../../
		"/%2e%2e%2f%2e%2e%2fsecret.txt",    // encoded ../../
		"/assets/%2e%2e%2f%2e%2e%2fsecret.txt",
	}

	for _, p := range payloads {
		t.Run(p, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))

			// Hard invariant: the file outside staticDir must never be served.
			if body := rec.Body.String(); strings.Contains(body, secretBody) {
				t.Fatalf("path traversal escaped staticDir: response leaked secret file (status %d)", rec.Code)
			}
			// A contained request is never a 200 serving arbitrary content: the
			// stdlib file server rejects the ".." path (400), and even if a
			// refactor let it through, filepath.Clean keeps it inside staticDir
			// where it misses and falls back to index.html.
			if rec.Code == http.StatusOK && rec.Body.String() != indexBody {
				t.Errorf("status 200 served unexpected body %q", rec.Body.String())
			}
		})
	}
}
