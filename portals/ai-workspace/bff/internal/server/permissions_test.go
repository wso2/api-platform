/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package server

import (
	"context"
	"net/http"
	"testing"

	"ai-workspace-bff/internal/session"
)

const meOK = `{"userId":"u-1","roles":["platform-admin"],"scopes":["ap:project:manage","ap:rest_api:manage"]}`

// A role-only token carries no scope claim, so the BFF must resolve the caller's
// scopes from the Platform API — otherwise the SPA hides every control.
func TestEnrichPermissions_RoleOnlyTokenGetsScopes(t *testing.T) {
	platform, calls := fakePlatformAPI(t, map[string]struct {
		status int
		body   string
	}{
		"GET " + portalMePath: {http.StatusOK, meOK},
	})
	s, _ := buildTestServer(t, platform.URL, "jwt-abc")

	u := session.User{Name: "alice"} // no scopes, no role
	s.enrichPermissions(context.Background(), "jwt-abc", &u)

	if len(u.Scopes) != 2 || u.Scopes[0] != "ap:project:manage" {
		t.Errorf("Scopes = %v, want the two scopes the role expands to", u.Scopes)
	}
	if u.Role != "platform-admin" {
		t.Errorf("Role = %q, want the IDP role as a display label", u.Role)
	}
	if len(*calls) != 1 || (*calls)[0].auth != "Bearer jwt-abc" {
		t.Errorf("calls = %+v, want one /me call bearing the caller's token", *calls)
	}
}

// A token that already carries scopes is authoritative; no upstream hop is made.
func TestEnrichPermissions_ScopedTokenSkipsLookup(t *testing.T) {
	platform, calls := fakePlatformAPI(t, map[string]struct {
		status int
		body   string
	}{
		"GET " + portalMePath: {http.StatusOK, meOK},
	})
	s, _ := buildTestServer(t, platform.URL, "jwt-abc")

	u := session.User{Scopes: []string{"ap:project:read"}}
	s.enrichPermissions(context.Background(), "jwt-abc", &u)

	if len(u.Scopes) != 1 || u.Scopes[0] != "ap:project:read" {
		t.Errorf("Scopes = %v, want the token's own scopes left untouched", u.Scopes)
	}
	if len(*calls) != 0 {
		t.Errorf("calls = %+v, want no /me call when the token already carries scopes", *calls)
	}
}

// A failed lookup must leave the user with no scopes: the app hides privileged
// controls rather than offering actions the Platform API would then reject.
func TestEnrichPermissions_LookupFailureLeavesNoScopes(t *testing.T) {
	platform, _ := fakePlatformAPI(t, map[string]struct {
		status int
		body   string
	}{
		"GET " + portalMePath: {http.StatusInternalServerError, `{"status":"error"}`},
	})
	s, _ := buildTestServer(t, platform.URL, "jwt-abc")

	u := session.User{Name: "alice"}
	s.enrichPermissions(context.Background(), "jwt-abc", &u)

	if len(u.Scopes) != 0 {
		t.Errorf("Scopes = %v, want none when the lookup fails", u.Scopes)
	}
}
