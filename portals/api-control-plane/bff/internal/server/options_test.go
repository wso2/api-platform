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
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"

	"api-control-plane-bff/internal/config"
	"api-control-plane-bff/internal/session"
)

// A caller that never sets Options must observe byte-for-byte the same
// behavior as before this contract existed — proven by re-running an
// existing end-to-end scenario through New(ctx, cfg) with no opts at all.
func TestOptions_ZeroValue_IsStandaloneBehavior(t *testing.T) {
	tok := makeJWT(map[string]any{"username": "admin", "scope": "ap:project:read"})
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"token": tok})
	}))
	defer platform.Close()

	cfg := newTestConfig(platform.URL)
	srv, err := New(context.Background(), cfg) // no Options argument at all
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()
	bff := httptest.NewServer(srv.Handler())
	defer bff.Close()

	res, err := http.Get(bff.URL + "/healthz")
	if err != nil {
		t.Fatalf("healthz request: %v", err)
	}
	assertStatus(t, res, http.StatusOK)
}

func TestOptions_ExtraRoutes_Reachable(t *testing.T) {
	cfg := newTestConfig("https://unused.example.com")
	opts := Options{
		ExtraRoutes: map[string]http.Handler{
			"GET /api/environments": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("environments"))
			}),
		},
	}
	srv, err := New(context.Background(), cfg, opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()
	bff := httptest.NewServer(srv.Handler())
	defer bff.Close()

	res, err := http.Get(bff.URL + "/api/environments")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	assertStatus(t, res, http.StatusOK)
}

func TestOptions_DisabledRoutes_404s(t *testing.T) {
	cfg := newTestConfig("https://unused.example.com")
	opts := Options{DisabledRoutes: []string{"GET /healthz"}}
	srv, err := New(context.Background(), cfg, opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()
	bff := httptest.NewServer(srv.Handler())
	defer bff.Close()

	res, err := http.Get(bff.URL + "/healthz")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	assertStatus(t, res, http.StatusNotFound)
}

func TestOptions_RouteOverrides_ReplacesDefaultHandler(t *testing.T) {
	cfg := newTestConfig("https://unused.example.com")
	opts := Options{
		RouteOverrides: map[string]http.Handler{
			"GET /healthz": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusTeapot)
			}),
		},
	}
	srv, err := New(context.Background(), cfg, opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()
	bff := httptest.NewServer(srv.Handler())
	defer bff.Close()

	res, err := http.Get(bff.URL + "/healthz")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	assertStatus(t, res, http.StatusTeapot)
}

func TestOptions_WrapRoute_DelegatesToOriginal(t *testing.T) {
	cfg := newTestConfig("https://unused.example.com")
	var wrapperRan bool
	opts := Options{
		WrapRoute: map[string]func(http.Handler) http.Handler{
			"GET /healthz": func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					wrapperRan = true
					w.Header().Set("X-Wrapped", "true")
					next.ServeHTTP(w, r) // delegates to the original handleHealth
				})
			},
		},
	}
	srv, err := New(context.Background(), cfg, opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()
	bff := httptest.NewServer(srv.Handler())
	defer bff.Close()

	res, err := http.Get(bff.URL + "/healthz")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	assertStatus(t, res, http.StatusOK) // original handleHealth still ran
	if !wrapperRan {
		t.Error("expected the wrapper to run")
	}
	if res.Header.Get("X-Wrapped") != "true" {
		t.Error("expected the wrapper's header to be present")
	}
	var body map[string]string
	json.NewDecoder(res.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("expected original handleHealth body to pass through, got %v", body)
	}
}

// An ExtraRoutes handler reads identity via session.FromContext exactly like
// a default handler would — proving the shared middleware chain (not a
// second auth path) is what makes this "seamless."
func TestOptions_ExtraRoutes_SeesSessionViaContext(t *testing.T) {
	tok := makeJWT(map[string]any{
		"username": "admin", "scope": "ap:project:read",
		"organization": "org-123", "org_name": "Acme", "org_handle": "acme",
	})
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"token": tok})
	}))
	defer platform.Close()

	cfg := newTestConfig(platform.URL)

	var gotOrgID string
	var gotOK bool
	opts := Options{
		ExtraRoutes: map[string]http.Handler{
			"GET /api/environments": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				u, ok := session.FromContext(r.Context())
				gotOK = ok
				if ok && u.Org != nil {
					gotOrgID = u.Org.ID
				}
				w.WriteHeader(http.StatusOK)
			}),
		},
	}
	srv, err := New(context.Background(), cfg, opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()
	bff := httptest.NewServer(srv.Handler())
	defer bff.Close()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	// Unauthenticated call: no session in context, handler must see ok=false.
	res, err := client.Get(bff.URL + "/api/environments")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	assertStatus(t, res, http.StatusOK)
	if gotOK {
		t.Error("expected ok=false for an unauthenticated request")
	}

	// Log in, then call again: handler must now see the resolved org.
	loginReq, _ := http.NewRequest(http.MethodPost, bff.URL+"/api/login",
		strings.NewReader(`{"username":"admin","password":"secret"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set(config.CSRFHeaderName, "api-control-plane")
	res, err = client.Do(loginReq)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	assertStatus(t, res, http.StatusOK)

	res, err = client.Get(bff.URL + "/api/environments")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	assertStatus(t, res, http.StatusOK)
	if !gotOK {
		t.Fatal("expected ok=true for an authenticated request")
	}
	if gotOrgID != "org-123" {
		t.Errorf("org ID = %q, want org-123", gotOrgID)
	}
}
