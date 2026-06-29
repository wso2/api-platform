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
// Username may be a comma-separated list of candidate claims; the first one
// present (in order) wins, so an IDP that carries the display name under a
// non-standard claim can be supported via config rather than code.
type ClaimMapping struct {
	Username  string
	Email     string
	Role      string
	Scope     string
	OrgID     string
	OrgName   string
	OrgHandle string
}

// DefaultClaimMapping returns the mapping used for file-based tokens (and as the
// fallback for OIDC). For OIDC, callers may override the org/user keys to match
// the IDP.
func DefaultClaimMapping() ClaimMapping {
	return ClaimMapping{
		Username:  "username,name,preferred_username",
		Email:     "email",
		Role:      "platform_role",
		Scope:     "scope",
		OrgID:     "organization",
		OrgName:   "org_name",
		OrgHandle: "org_handle",
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
	get := func(key string) string {
		if key == "" {
			return ""
		}
		if idClaims != nil {
			if s, ok := idClaims[key].(string); ok && s != "" {
				return s
			}
		}
		if s, ok := claims[key].(string); ok {
			return s
		}
		return ""
	}

	// Resolve a human-friendly display name. Try each configured candidate claim
	// in order, then email, and only as a last resort the opaque subject id (so
	// the UI never shows a raw UUID when a readable claim is available).
	candidates := splitClaims(m.Username)
	vals := make([]string, 0, len(candidates)+2)
	for _, key := range candidates {
		vals = append(vals, get(key))
	}
	vals = append(vals, get(m.Email), strClaim(claims, "sub"))

	u := User{
		Name:   first(vals...),
		Email:  get(m.Email),
		Role:   strClaim(claims, m.Role),
		Scopes: scopes(claims, m.Scope),
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
	return u
}

func strClaim(claims map[string]any, key string) string {
	if key == "" || claims == nil {
		return ""
	}
	if s, ok := claims[key].(string); ok {
		return s
	}
	return ""
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

// splitClaims parses a comma-separated list of claim names into a trimmed,
// non-empty slice. A single name (no commas) yields a one-element slice.
func splitClaims(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func first(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
