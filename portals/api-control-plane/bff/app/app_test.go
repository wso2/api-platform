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

// package app_test (external, black-box) deliberately imports ONLY
// api-control-plane-bff/app — the same constraint a real host module (e.g.
// cloud-control-plane's own BFF) is under. If this file ever needed to
// import an internal/ package to build a working server, that would mean
// the app package's public surface is insufficient on its own.
package app_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api-control-plane-bff/app"
)

func TestApp_New_WithExtraRoute(t *testing.T) {
	cfg := &app.Config{}
	cfg.Server.HTTP.Enabled = true
	cfg.ControlPlane.URL = "https://unused.example.com"
	cfg.ControlPlane.ProxyPrefix = "/proxy"
	cfg.Session.Store = "memory"
	cfg.Session.AbsoluteTTL = 8 * time.Hour
	cfg.Session.Cookie.Name = "_test_session"
	cfg.Auth.Mode = "basic"

	srv, err := app.New(context.Background(), cfg, app.Options{
		ExtraRoutes: map[string]http.Handler{
			"GET /api/environments": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if _, ok := app.SessionFromContext(r.Context()); ok {
					t.Error("expected no session for an unauthenticated request")
				}
				w.WriteHeader(http.StatusOK)
			}),
		},
	})
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	defer srv.Close()

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/environments")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", res.StatusCode)
	}

	res, err = http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Errorf("healthz status = %d, want 200 (default routes must still be present)", res.StatusCode)
	}
}
