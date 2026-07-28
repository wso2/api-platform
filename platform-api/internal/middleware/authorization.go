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
	"errors"
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

// RouteMatcher resolves the route pattern a request will match, without serving
// it. *http.ServeMux satisfies this interface (Go 1.22+).
type RouteMatcher interface {
	Handler(r *http.Request) (h http.Handler, pattern string)
}

// ScopeEnforcerConfig holds options for the ScopeEnforcer middleware.
type ScopeEnforcerConfig struct {
	// ValidationMode selects how authorization is enforced: "scope" (default) or "role".
	ValidationMode string
	// Enabled controls whether scope checks are enforced.
	Enabled bool
	// Routes resolves the route pattern a request will match. It is required
	// whenever Enabled is set: ScopeEnforcer runs as an outer middleware, ahead
	// of the router, so r.Pattern is still empty when it executes and the
	// registry lookup can only be keyed off a pattern resolved from the router
	// itself. Pass the *http.ServeMux the routes are registered on.
	Routes RouteMatcher
	// SkipPaths are path prefixes exempt from scope enforcement — health/metrics
	// probes, the login endpoint, and the internal routes authenticated by a
	// gateway token rather than a user JWT. Mirrors config.Auth.SkipPaths, which
	// is the same list the authentication middleware bypasses.
	SkipPaths []string
}

// ScopeEnforcer returns a middleware that reads the required scopes for each request
// from the OpenAPI ScopeRegistry and enforces them.
//
// The matched route template (e.g. "GET /api/v0.9/rest-apis/{restApiId}") is
// resolved via cfg.Routes, then looked up in the registry. Enforcement is
// deny-by-default: a request that matches a registered route carrying no scope
// requirement is rejected rather than passed through, so a route that is added
// without a corresponding OpenAPI security block fails closed instead of
// becoming a silently unprotected endpoint. Requests under cfg.SkipPaths, and
// requests the authenticator flagged as authz-skipped, bypass the check.
//
// It returns an error when the configuration cannot enforce anything it claims
// to (GO-AUTH-011) — enforcement enabled with no registry or no route matcher —
// so the server refuses to start rather than degrading to a pass-through.
func ScopeEnforcer(registry *ScopeRegistry, cfg ScopeEnforcerConfig) (func(http.Handler) http.Handler, error) {
	mode := cfg.ValidationMode
	if mode == "" {
		mode = ValidationModeScope
	}

	if cfg.Enabled {
		if registry == nil {
			return nil, errors.New("scope enforcement is enabled but no scope registry was provided")
		}
		if cfg.Routes == nil {
			return nil, errors.New("scope enforcement is enabled but no route matcher was provided — " +
				"without it no route pattern can be resolved and every scope requirement would be skipped")
		}
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

			if hasPathPrefix(r.URL.Path, cfg.SkipPaths) {
				next.ServeHTTP(w, r)
				return
			}

			// r.Pattern is only populated once ServeMux has matched the request,
			// which happens after this middleware runs — resolve the pattern from
			// the router directly. r.Pattern is still preferred when non-empty so
			// the enforcer keeps working if it is ever moved inside the router.
			pattern := r.Pattern
			if pattern == "" {
				_, pattern = cfg.Routes.Handler(r)
			}
			if pattern == "" {
				// No registered route matched (or the router will redirect). Let it
				// answer — an unrouted request must produce its 404/405/redirect,
				// not a 403 that leaks which paths exist.
				next.ServeHTTP(w, r)
				return
			}

			// pattern is "METHOD /path/{param}" — extract the path portion.
			path := pattern
			if idx := strings.Index(pattern, " "); idx != -1 {
				path = pattern[idx+1:]
			}

			requiredScopes, found := registry.Lookup(r.Method, path)
			if !found || len(requiredScopes) == 0 {
				// Deny by default: the route exists but declares no scope
				// requirement, so there is nothing to authorize against.
				writeError(w, apperror.Forbidden.New(), "no scope requirement declared for matched route")
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
	}, nil
}

// hasPathPrefix reports whether path starts with any of the given prefixes,
// matching the prefix semantics the authentication middleware uses for its own
// skip list so both bypass exactly the same set of requests.
func hasPathPrefix(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
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

// HasEffectiveScope reports whether the request's effective scopes (the scope
// claim in "scope" mode, or IDP roles expanded via the role-scope map in
// "role" mode) satisfy the given scope, honoring ":*" wildcards the same way
// ScopeEnforcer does.
func HasEffectiveScope(r *http.Request, mode, scope string) bool {
	for _, have := range resolveEffectiveScopes(r, mode) {
		if scopeSatisfies(have, scope) {
			return true
		}
	}
	return false
}
