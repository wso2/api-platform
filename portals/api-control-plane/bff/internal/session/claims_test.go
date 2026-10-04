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

package session

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

// makeJWT builds an unsigned JWT (header.payload.signature) for decode tests.
// The BFF never verifies signatures, so a fake signature is fine.
func makeJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	pb, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(pb)
	return header + "." + payload + ".fakesignature"
}

func TestDecodeJWTClaims(t *testing.T) {
	tok := makeJWT(t, map[string]any{"sub": "alice", "scope": "ap:project:read ap:gateway:manage"})
	claims := DecodeJWTClaims(tok)
	if claims == nil {
		t.Fatal("expected claims, got nil")
	}
	if claims["sub"] != "alice" {
		t.Errorf("sub = %v, want alice", claims["sub"])
	}
}

func TestDecodeJWTClaims_Malformed(t *testing.T) {
	for _, bad := range []string{"", "notajwt", "only.two"} {
		if got := DecodeJWTClaims(bad); got != nil {
			t.Errorf("DecodeJWTClaims(%q) = %v, want nil", bad, got)
		}
	}
}

func TestUserFromClaims_FileBased(t *testing.T) {
	claims := map[string]any{
		"username":     "admin",
		"scope":        "ap:project:read ap:project:manage",
		"organization": "org-123",
		"org_name":     "Acme",
		"org_handle":   "acme",
	}
	u := UserFromClaims(claims, nil, DefaultClaimMapping())

	if u.Name != "admin" {
		t.Errorf("Name = %q, want admin", u.Name)
	}
	if len(u.Scopes) != 2 || u.Scopes[0] != "ap:project:read" {
		t.Errorf("Scopes = %v, want 2 scopes", u.Scopes)
	}
	if u.Org == nil || u.Org.ID != "org-123" || u.Org.Handle != "acme" {
		t.Errorf("Org = %+v, want org-123/acme", u.Org)
	}
}

func TestUserFromClaims_ScopesArray(t *testing.T) {
	// IDPs like Asgardeo may carry scopes as an array under "scp".
	claims := map[string]any{
		"sub": "u1",
		"scp": []any{"ap:rest_api:read", "ap:rest_api:create"},
	}
	u := UserFromClaims(claims, nil, DefaultClaimMapping())
	if len(u.Scopes) != 2 {
		t.Errorf("Scopes = %v, want 2", u.Scopes)
	}
	// Falls back to sub for the name when username claim is absent.
	if u.Name != "u1" {
		t.Errorf("Name = %q, want u1 (from sub)", u.Name)
	}
}

func TestUserFromClaims_IDClaimsPreferred(t *testing.T) {
	at := map[string]any{"given_name": "", "email": ""}
	id := map[string]any{"given_name": "Alice", "email": "alice@example.com"}
	m := DefaultClaimMapping()
	m.Username = "given_name"
	u := UserFromClaims(at, id, m)
	if u.Name != "Alice" || u.Email != "alice@example.com" {
		t.Errorf("got name=%q email=%q, want Alice/alice@example.com", u.Name, u.Email)
	}
}

func TestExpiryFromClaims(t *testing.T) {
	exp := time.Now().Add(time.Hour).Unix()
	claims := map[string]any{"exp": float64(exp)}
	got := ExpiryFromClaims(claims)
	if got.Unix() != exp {
		t.Errorf("ExpiryFromClaims = %d, want %d", got.Unix(), exp)
	}
	if !ExpiryFromClaims(map[string]any{}).IsZero() {
		t.Error("expected zero time when exp absent")
	}
}

func TestResolveClaimPath_Nested(t *testing.T) {
	// Keycloak-style nested roles claim.
	claims := map[string]any{
		"realm_access": map[string]any{
			"roles": []any{"admin", "editor"},
		},
	}
	m := DefaultClaimMapping()
	m.Roles = "realm_access.roles" // not a string; Role stays empty, Scopes exercises the array path below
	m.Scope = "realm_access.roles"
	u := UserFromClaims(claims, nil, m)
	if len(u.Scopes) != 2 || u.Scopes[0] != "admin" || u.Scopes[1] != "editor" {
		t.Errorf("Scopes = %v, want [admin editor] via dotted path", u.Scopes)
	}
}

func TestResolveClaimPath_MissingSegmentReturnsNil(t *testing.T) {
	claims := map[string]any{"realm_access": map[string]any{}}
	if got := resolveClaimPath(claims, "realm_access.roles"); got != nil {
		t.Errorf("resolveClaimPath = %v, want nil for missing leaf segment", got)
	}
	if got := resolveClaimPath(claims, "missing.nested.path"); got != nil {
		t.Errorf("resolveClaimPath = %v, want nil for missing root segment", got)
	}
}

func TestResolveClaimPath_NonMapIntermediateReturnsNil(t *testing.T) {
	// "sub" is a string, not a map — walking a dotted path through it must not panic.
	claims := map[string]any{"sub": "alice"}
	if got := resolveClaimPath(claims, "sub.nested"); got != nil {
		t.Errorf("resolveClaimPath = %v, want nil when an intermediate segment isn't a map", got)
	}
}

func TestUserFromClaims_DottedUsernamePath(t *testing.T) {
	claims := map[string]any{
		"identity": map[string]any{"display_name": "Bob"},
	}
	m := DefaultClaimMapping()
	m.Username = "identity.display_name"
	u := UserFromClaims(claims, nil, m)
	if u.Name != "Bob" {
		t.Errorf("Name = %q, want Bob via dotted username path", u.Name)
	}
}

func TestUserFromClaims_RoleMode(t *testing.T) {
	claims := map[string]any{
		"preferred_username": "user1@example.onmicrosoft.com",
		"roles":              []any{"ap_admin"},
		"scp":                "access",
	}
	m := DefaultClaimMapping()
	m.Username = "preferred_username"
	m.AuthzMode = AuthzModeRole
	m.RoleScopeMap = map[string][]string{
		"ap_admin": {"ap:organization:manage", "ap:project:manage"},
	}

	u := UserFromClaims(claims, nil, m)

	want := []string{"ap:organization:manage", "ap:project:manage"}
	if len(u.Scopes) != len(want) || u.Scopes[0] != want[0] || u.Scopes[1] != want[1] {
		t.Errorf("Scopes = %v, want %v", u.Scopes, want)
	}
	// The token's own "access" scope must not leak through as an effective scope.
	for _, s := range u.Scopes {
		if s == "access" {
			t.Errorf("Scopes = %v, want the scope claim ignored in role mode", u.Scopes)
		}
	}
	if u.Role != "ap_admin" {
		t.Errorf("Role = %q, want ap_admin", u.Role)
	}
}

// Several roles union; the display Role carries all of them.
func TestUserFromClaims_RoleModeSeveralRoles(t *testing.T) {
	claims := map[string]any{"sub": "u1", "roles": []any{"ap_viewer", "ap_publisher"}}
	m := DefaultClaimMapping()
	m.AuthzMode = AuthzModeRole
	m.RoleScopeMap = map[string][]string{
		"ap_viewer":    {"ap:project:read"},
		"ap_publisher": {"ap:rest_api:manage"},
	}
	u := UserFromClaims(claims, nil, m)
	if len(u.Scopes) != 2 {
		t.Errorf("Scopes = %v, want 2", u.Scopes)
	}
	if u.Role != "ap_viewer ap_publisher" {
		t.Errorf("Role = %q, want both roles", u.Role)
	}
}

// A role the operator never mapped grants nothing, and role mode must NOT fall back to
// the scope claim — that would show actions as available which then fail with 403.
func TestUserFromClaims_RoleModeUnmappedRoleGrantsNothing(t *testing.T) {
	claims := map[string]any{
		"sub":   "u1",
		"roles": []any{"SomeAzureGroup"},
		"scp":   "access",
	}
	m := DefaultClaimMapping()
	m.AuthzMode = AuthzModeRole
	m.RoleScopeMap = map[string][]string{"ap_admin": {"ap:organization:manage"}}

	if u := UserFromClaims(claims, nil, m); len(u.Scopes) != 0 {
		t.Errorf("Scopes = %v, want empty for an unmapped role", u.Scopes)
	}
}

// Scope mode is unchanged by the role-mode addition: the scope claim still wins and the
// roles claim is not expanded even when a map happens to be present.
func TestUserFromClaims_ScopeModeIgnoresRoles(t *testing.T) {
	claims := map[string]any{
		"sub":   "u1",
		"roles": []any{"ap_admin"},
		"scope": "ap:project:read",
	}
	m := DefaultClaimMapping()
	m.RoleScopeMap = map[string][]string{"ap_admin": {"ap:organization:manage"}}

	u := UserFromClaims(claims, nil, m)
	if len(u.Scopes) != 1 || u.Scopes[0] != "ap:project:read" {
		t.Errorf("Scopes = %v, want [ap:project:read]", u.Scopes)
	}
}

// A string-valued roles claim (Asgardeo) reads the same as an array one.
func TestUserFromClaims_RolesAsString(t *testing.T) {
	claims := map[string]any{"sub": "u1", "roles": "ap_admin ap_viewer"}
	m := DefaultClaimMapping()
	m.AuthzMode = AuthzModeRole
	m.RoleScopeMap = map[string][]string{
		"ap_admin":  {"ap:organization:manage"},
		"ap_viewer": {"ap:project:read"},
	}
	if u := UserFromClaims(claims, nil, m); len(u.Scopes) != 2 {
		t.Errorf("Scopes = %v, want 2", u.Scopes)
	}
}

// Keycloak nests roles under realm_access; the dotted-path mapping this BFF supports
// for every claim must reach them for the role-mode expansion too.
func TestUserFromClaims_RoleModeDottedRolesPath(t *testing.T) {
	claims := map[string]any{
		"sub":          "u1",
		"realm_access": map[string]any{"roles": []any{"ap_viewer"}},
	}
	m := DefaultClaimMapping()
	m.Roles = "realm_access.roles"
	m.AuthzMode = AuthzModeRole
	m.RoleScopeMap = map[string][]string{"ap_viewer": {"ap:project:read"}}

	u := UserFromClaims(claims, nil, m)
	if len(u.Scopes) != 1 || u.Scopes[0] != "ap:project:read" {
		t.Errorf("Scopes = %v, want [ap:project:read]", u.Scopes)
	}
	if u.Role != "ap_viewer" {
		t.Errorf("Role = %q, want ap_viewer", u.Role)
	}
}
