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
	"strings"
	"testing"

	"ai-workspace-bff/internal/config"
)

// routesTestServer builds a Server wired only far enough to exercise routes(): the
// mux, the static handler and the runtime-config handler. The proxy and auth
// handlers are reached but not followed through, so their nil dependencies are
// never touched by the assertions below.
func routesTestServer(t *testing.T) *Server {
	t.Helper()
	s := &Server{cfg: &config.Config{
		Server: config.ServerConfig{
			StaticDir: newStaticTestDir(t),
		},
		RuntimeConfig: map[string]string{"APIP_AIW_AUTH_MODE": "basic"},
	}}
	s.handler = s.routes()
	return s
}

// The SPA, its assets, the runtime config and the health endpoint must all be
// reachable under the configured prefix — and the prefix must be stripped before
// files are resolved, so an asset is served rather than looked up inside a
// directory named after the base path.
func TestRoutesServeUnderBasePath(t *testing.T) {
	s := routesTestServer(t)

	cases := []struct {
		path     string
		wantBody string
	}{
		{"/ai-workspace/", indexBody},
		{"/ai-workspace/projects", indexBody}, // client-side route → index fallback
		{"/ai-workspace/assets/app.js", assetBody},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			s.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if got := rec.Body.String(); got != tc.wantBody {
				t.Errorf("body = %q, want %q", got, tc.wantBody)
			}
		})
	}

	t.Run("runtime config", func(t *testing.T) {
		rec := httptest.NewRecorder()
		s.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ai-workspace/runtime-config.js", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/javascript" {
			t.Errorf("Content-Type = %q, want application/javascript", ct)
		}
	})

	// Health answers both at the origin root (container HEALTHCHECK / kubelet probes
	// dial the pod directly, bypassing the ingress that adds the prefix) and under
	// the prefix (a probe routed through that ingress).
	for _, p := range []string{"/healthz", "/ai-workspace/healthz"} {
		t.Run("health "+p, func(t *testing.T) {
			rec := httptest.NewRecorder()
			s.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want 200", rec.Code)
			}
		})
	}
}

// The bare prefix and the origin root are both conveniences that must land on the
// app; any other unprefixed path belongs to whatever else the ingress routes on this
// host and must stay a 404 rather than being swallowed by the SPA fallback.
func TestRoutesRedirectsAndRootIsolation(t *testing.T) {
	s := routesTestServer(t)

	redirects := map[string]string{
		"/":             "/ai-workspace/",
		"/ai-workspace": "/ai-workspace/",
	}
	for from, to := range redirects {
		t.Run("redirect "+from, func(t *testing.T) {
			rec := httptest.NewRecorder()
			s.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, from, nil))
			// ServeMux's own bare-prefix → subtree redirect is a 307; the explicit
			// root redirect is a 302. Either is fine — only the target matters here.
			if rec.Code < 300 || rec.Code > 399 {
				t.Fatalf("status = %d, want a 3xx redirect", rec.Code)
			}
			if loc := rec.Header().Get("Location"); loc != to {
				t.Errorf("Location = %q, want %q", loc, to)
			}
		})
	}

	for _, p := range []string{"/projects", "/assets/app.js", "/runtime-config.js"} {
		t.Run("not found "+p, func(t *testing.T) {
			rec := httptest.NewRecorder()
			s.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want 404 (path is outside the app's base path)", rec.Code)
			}
		})
	}
}

// Path containment now sits behind http.StripPrefix, so the traversal payloads
// static_test.go throws at spaHandler directly must also be defeated when they arrive
// through the real mux with the base path attached — the layer that strips the prefix
// must not hand the file server an escaping path.
func TestRoutesPathTraversalContainedUnderBasePath(t *testing.T) {
	s := routesTestServer(t)

	payloads := []string{
		"/ai-workspace/../secret.txt",
		"/ai-workspace/../../secret.txt",
		"/ai-workspace/assets/../../secret.txt",
		"/ai-workspace/%2e%2e%2fsecret.txt",
		"/ai-workspace/..%2f..%2fsecret.txt",
		"/ai-workspace/%2e%2e%2f%2e%2e%2fsecret.txt",
		// A prefix the base path is merely a substring of must not be served either.
		"/ai-workspace-admin/../secret.txt",
	}
	for _, p := range payloads {
		t.Run(p, func(t *testing.T) {
			rec := httptest.NewRecorder()
			s.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))

			if body := rec.Body.String(); strings.Contains(body, secretBody) {
				t.Fatalf("traversal escaped staticDir: response leaked the secret file (status %d)", rec.Code)
			}
			if rec.Code == http.StatusOK && rec.Body.String() != indexBody {
				t.Errorf("status 200 served unexpected body %q", rec.Body.String())
			}
		})
	}
}
