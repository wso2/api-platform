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

package middleware

import (
	"net/http"
	"strings"

	"github.com/wso2/api-platform/common/authenticators"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
)

const (
	// ValidationModeScope validates using the JWT scope claim directly.
	ValidationModeScope = "scope"
	// ValidationModeRole validates by expanding IDP roles into platform roles.
	ValidationModeRole = "role"
)

// InitScopeAuthz is retained for compatibility.
func InitScopeAuthz() {}

// InitClaimsAuthz is retained for compatibility.
func InitClaimsAuthz() {}

// ScopeEnforcerConfig holds options for the ScopeEnforcer middleware.
type ScopeEnforcerConfig struct {
	// ValidationMode selects how authorization is enforced: "scope" (default) or "role".
	ValidationMode string
	// Enabled controls whether scope checks are enforced.
	Enabled bool
}

// ScopeEnforcer returns a middleware that reads the required scopes for each request
// from the OpenAPI ScopeRegistry and enforces them.
//
// It uses r.Pattern (set by net/http ServeMux in Go 1.22+) to identify the matched
// route template (e.g. "GET /api/v0.9/rest-apis/{id}"). Routes not present in the
// registry are passed through without a scope check.
func ScopeEnforcer(registry *ScopeRegistry, cfg ScopeEnforcerConfig) func(http.Handler) http.Handler {
	mode := cfg.ValidationMode
	if mode == "" {
		mode = ValidationModeScope
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			if authenticators.GetAuthzSkip(r) {
				next.ServeHTTP(w, r)
				return
			}

			// r.Pattern is "METHOD /path/{param}" — extract the path portion.
			pattern := r.Pattern
			path := pattern
			if idx := strings.Index(pattern, " "); idx != -1 {
				path = pattern[idx+1:]
			}

			requiredScopes, found := registry.Lookup(r.Method, path)
			if !found || len(requiredScopes) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			effectiveScopes := resolveEffectiveScopes(r, mode)

			for _, required := range requiredScopes {
				for _, have := range effectiveScopes {
					if scopeSatisfies(have, required) {
						next.ServeHTTP(w, r)
						return
					}
				}
			}

			writeError(w, apperror.Forbidden.New(), "insufficient scopes for route")
		})
	}
}

// scopeSatisfies reports whether a held scope grants a required scope.
func scopeSatisfies(have, required string) bool {
	if have == required {
		return true
	}
	if !strings.HasSuffix(have, ":*") {
		return false
	}
	base := strings.TrimSuffix(have, "*")
	if !strings.HasPrefix(required, base) {
		return false
	}
	remainder := required[len(base):]
	if remainder == "" {
		return false
	}
	segments := strings.Count(remainder, ":") + 1
	if strings.Count(base, ":") == 1 {
		return segments == 2
	}
	return segments == 1
}

// resolveEffectiveScopes returns the effective scopes for the request.
func resolveEffectiveScopes(r *http.Request, mode string) []string {
	if mode == ValidationModeRole {
		roles, _ := GetPlatformRolesFromRequest(r)
		return roles
	}
	scope, _ := GetScopeFromRequest(r)
	return strings.Fields(scope)
}
