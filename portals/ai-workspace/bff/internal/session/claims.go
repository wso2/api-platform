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
	"strings"
	"time"
)

// ClaimMapping configures which claim names carry user/org fields. Defaults
// match the Platform API file-based JWT and the SPA's OIDC defaults.
//
// Username names the single claim that carries the display name; an IDP that
// carries it under a non-standard claim can be supported via config rather than
// code. When that claim is absent the display name falls back to email, then
// the subject id.
type ClaimMapping struct {
	Username      string
	Email         string
	Roles         string
	Scope         string
	OrgID         string
	OrgName       string
	OrgHandle     string
	Organizations string

	// AuthzMode mirrors the Platform API's auth.authorization.mode: "scope"
	// (default) reads the user's effective scopes from the scope claim, "role"
	// derives them by expanding the roles claim through RoleScopeMap. It lives on
	// the claim mapping rather than in its own struct because this value is already
	// threaded to every place a User is built, and the mode decides which *claim*
	// the scopes are read from — a mapping concern.
	AuthzMode string
	// RoleScopeMap is the loaded role-to-scope grant table, used only in role mode.
	// Nil in scope mode.
	RoleScopeMap map[string][]string
}

// AuthzModeRole is the auth.authorization.mode value that derives effective scopes
// from the roles claim rather than from the scope claim.
const AuthzModeRole = "role"

// DefaultClaimMapping returns the built-in fallback mapping, used whenever a
// config.ClaimMappingConfig field is left unset — for both file-based and OIDC
// tokens. Callers may override individual keys to match a specific IDP.
func DefaultClaimMapping() ClaimMapping {
	return ClaimMapping{
		Username:      "username",
		Email:         "email",
		Roles:         "roles",
		Scope:         "scope",
		OrgID:         "organization",
		OrgName:       "org_name",
		OrgHandle:     "org_handle",
		Organizations: "organizations",
	}
}

// DecodeJWTClaims base64-decodes a JWT payload WITHOUT verifying the signature.
// The BFF never validates tokens (the Platform API does); this only extracts
// claims for display and to read exp. Returns nil on malformed input.
func DecodeJWTClaims(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		// Try standard (padded) URL encoding as a fallback.
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil
		}
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil
	}
	return claims
}

// ExpiryFromClaims reads the standard "exp" claim (seconds since epoch).
// Returns the zero time if absent.
func ExpiryFromClaims(claims map[string]any) time.Time {
	if claims == nil {
		return time.Time{}
	}
	switch v := claims["exp"].(type) {
	case float64:
		return time.Unix(int64(v), 0)
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return time.Unix(n, 0)
		}
	}
	return time.Time{}
}

// UserFromClaims builds the display User from decoded claims using the mapping.
// idClaims (OIDC id_token) is optional and consulted first for name/email.
func UserFromClaims(claims, idClaims map[string]any, m ClaimMapping) User {
	// Both lookups go through strClaim so a configured value may be a dot-separated
	// path (see resolveClaimPath). The id_token is preferred where it has the claim:
	// it carries the user's profile, while the access token often carries only ids.
	get := func(key string) string {
		if key == "" {
			return ""
		}
		if s := strClaim(idClaims, key); s != "" {
			return s
		}
		return strClaim(claims, key)
	}

	// Resolve a human-friendly display name from the configured username claim,
	// then email, and only as a last resort the opaque subject id (so the UI
	// never shows a raw UUID when a readable claim is available).
	// The roles claim is a string on some IDPs and an array on others (Entra ID emits
	// ["ap_admin"]), so read both shapes — a plain string read would leave this empty
	// for an array-valued claim. roleList also feeds the role-mode expansion below.
	roleList := strSliceClaim(claims, m.Roles)

	u := User{
		Name:   first(get(m.Username), get(m.Email), get("sub")),
		Email:  get(m.Email),
		Role:   strings.Join(roleList, " "),
		Scopes: effectiveScopes(claims, roleList, m),
	}

	orgID := strClaim(claims, m.OrgID)
	orgName := strClaim(claims, m.OrgName)
	orgHandle := strClaim(claims, m.OrgHandle)
	if orgID != "" || orgHandle != "" {
		name := orgName
		if name == "" {
			name = orgHandle
		}
		u.Org = &Org{ID: orgID, Name: name, Handle: orgHandle}
	}

	u.Organizations = strSliceClaim(claims, m.Organizations)
	return u
}

// resolveClaimPath walks a dot-separated path into nested claim objects. A path with
// no "." is an ordinary flat lookup, so every claim_mappings value may name either a
// top-level claim ("org_id") or a nested one ("organization.uuid").
//
// It mirrors the Platform API's resolveClaimPath (middleware/auth.go) deliberately:
// the two services read the SAME token with the SAME configured mappings, so a path
// one understands and the other does not means the UI and the API disagree about
// which org the caller is in — the UI showing no org while the API happily scopes
// every query to one. Nested organization claims are the common shape (Choreo STS
// issues {"organization": {"handle": ..., "uuid": ...}}), not an edge case.
func resolveClaimPath(obj map[string]any, path string) (any, bool) {
	if path == "" || obj == nil {
		return nil, false
	}
	parts := strings.SplitN(path, ".", 2)
	val, ok := obj[parts[0]]
	if !ok {
		return nil, false
	}
	if len(parts) == 1 {
		return val, true
	}
	nested, ok := val.(map[string]any)
	if !ok {
		return nil, false
	}
	return resolveClaimPath(nested, parts[1])
}

// ClaimString resolves a configured claim name — flat ("org_handle") or a
// dot-separated path into a nested object ("organization.handle") — to its string
// value, or "" when it is absent or not a string.
//
// Exported because the token-exchange layer reads the same claims from the same
// configured mapping: two resolvers would mean a path that works for the session and
// silently reads empty in a log line, which is exactly the confusion this is meant to
// remove.
func ClaimString(claims map[string]any, path string) string {
	return strClaim(claims, path)
}

func strClaim(claims map[string]any, key string) string {
	if key == "" || claims == nil {
		return ""
	}
	val, ok := resolveClaimPath(claims, key)
	if !ok {
		return ""
	}
	s, _ := val.(string)
	return s
}

// strSliceClaim reads a claim that may be a single string, a space-delimited string,
// or an array of strings. Roles arrive in all three shapes depending on the IDP:
// Asgardeo sends a string, Entra ID sends an array.
func strSliceClaim(claims map[string]any, key string) []string {
	if key == "" || claims == nil {
		return nil
	}
	raw, ok := resolveClaimPath(claims, key)
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case string:
		return strings.Fields(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// effectiveScopes resolves the scopes the SPA gates on, mirroring the Platform API's
// resolveEffectiveScopes: in role mode the roles claim expanded through the grant
// table, otherwise the scope claim as-is.
//
// Role mode deliberately does not fall back to the scope claim when the expansion is
// empty. A role the operator never mapped granting nothing is a real deny-by-default
// outcome, and the Platform API reaches the same one for the same token — falling back
// here would show actions as available that then fail with 403.
func effectiveScopes(claims map[string]any, roleList []string, m ClaimMapping) []string {
	if m.AuthzMode == AuthzModeRole {
		return ExpandRoles(roleList, m.RoleScopeMap)
	}
	return scopes(claims, m.Scope)
}

// scopes reads the scope claim, which may be a space-delimited string ("scope")
// or an array ("scp" on some IDPs). It checks the configured key and "scp".
func scopes(claims map[string]any, key string) []string {
	raw, ok := claims[key]
	if !ok {
		raw = claims["scp"]
	}
	switch v := raw.(type) {
	case string:
		return strings.Fields(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return []string{}
}

func first(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
