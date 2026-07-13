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
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/middleware"
)

func getMe(t *testing.T, mode string, decorate func(*http.Request) *http.Request) meResponse {
	t.Helper()

	r := decorate(httptest.NewRequest(http.MethodGet, "/api/portal/v0.9/me", nil))
	w := httptest.NewRecorder()

	if err := NewMeHandler(mode, slog.Default()).GetMe(w, r); err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var got meResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return got
}

// In role mode the token carries roles and no scope claim. /me must report the
// scopes those roles expand to — that expansion is the whole reason the endpoint
// exists, since a client reading the scope claim would see nothing.
func TestGetMe_RoleModeReportsExpandedScopes(t *testing.T) {
	got := getMe(t, middleware.ValidationModeRole, func(r *http.Request) *http.Request {
		r = middleware.WithUserID(r, "u-1")
		r = middleware.WithOrganization(r, "org-uuid-1")
		return middleware.WithRoles(r,
			[]string{"platform-admin"},
			[]string{"ap:project:manage", "ap:rest_api:manage"},
		)
	})

	if len(got.Scopes) != 2 || got.Scopes[0] != "ap:project:manage" {
		t.Errorf("scopes = %v, want the scopes the role expands to", got.Scopes)
	}
	if len(got.Roles) != 1 || got.Roles[0] != "platform-admin" {
		t.Errorf("roles = %v, want the raw IDP role name", got.Roles)
	}
	if got.Organization == nil || got.Organization.ID != "org-uuid-1" {
		t.Errorf("organization = %+v, want the caller's org", got.Organization)
	}
}

// In scope mode the scope claim remains the source of truth.
func TestGetMe_ScopeModeReportsScopeClaim(t *testing.T) {
	got := getMe(t, middleware.ValidationModeScope, func(r *http.Request) *http.Request {
		r = middleware.WithUserID(r, "u-1")
		return middleware.WithScope(r, "ap:project:read ap:gateway:manage")
	})

	if len(got.Scopes) != 2 || got.Scopes[1] != "ap:gateway:manage" {
		t.Errorf("scopes = %v, want the token's scope claim split into scopes", got.Scopes)
	}
}

// Roles and scopes must serialize as [] rather than null so clients can index
// into them without a nil check.
func TestGetMe_EmptyPermissionsSerializeAsArrays(t *testing.T) {
	r := middleware.WithUserID(httptest.NewRequest(http.MethodGet, "/api/portal/v0.9/me", nil), "u-1")
	w := httptest.NewRecorder()

	if err := NewMeHandler(middleware.ValidationModeRole, slog.Default()).GetMe(w, r); err != nil {
		t.Fatalf("GetMe: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, key := range []string{"roles", "scopes"} {
		if string(raw[key]) != "[]" {
			t.Errorf("%s = %s, want []", key, raw[key])
		}
	}
}
