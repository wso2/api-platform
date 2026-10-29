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
	"context"
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
	// is the same list the authentication middleware bypasses — through the same
	// hasPathPrefix matcher, so both exempt exactly the same requests.
	SkipPaths []string
	// LegacyScopes translates the pre-permission-tree scope names a token, an
	// IDP role or a role-to-scope mapping may still carry into the permissions
	// the OpenAPI spec declares (resources/legacy-scope-mapping.yaml). Every
	// request's held scopes are expanded through it before matching, in both
	// validation modes. Nil accepts only the scopes the spec declares.
	LegacyScopes LegacyScopeMap
}

// keyEffectiveScopes carries the request's effective scopes — the scope claim
// or expanded roles, with legacy names translated — from ScopeEnforcer to the
// handlers' own checks (HasEffectiveScope), so both see the same set.
const keyEffectiveScopes contextKey = "effective_scopes"

// effectiveScopes is the context value under keyEffectiveScopes. A pointer
// type, so that an unauthenticated request's empty list is still recorded as
// "resolved" and not mistaken for "ScopeEnforcer never ran".
type effectiveScopes struct {
	scopes []string
}

// withEffectiveScopes records the request's effective scopes for the handlers.
func withEffectiveScopes(r *http.Request, scopes []string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), keyEffectiveScopes, &effectiveScopes{scopes: scopes}))
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
// Matching is an OR over the operation's accepted-scope list, by exact string
// equality: the caller is admitted when any scope it effectively holds names
// one of them. Two things make that OR reach further than the strings a token
// literally carries, and both are resolved before the comparison, never by
// pattern-matching the scope strings at request time:
//
//   - Parents in the permission tree. The accepted list is generated from
//     resources/permission-tree.yaml (scripts/permission_tree.py) and already
//     names, for the node an operation is bound to, that node's permission and
//     the same action's permission on every ancestor that defines it — so a
//     request bound to ap:api:rest:deployment:manage is admitted by
//     ap:api:rest:manage or ap:api:manage as well. A permission the tree does
//     not define is therefore never accepted, however its string reads.
//   - Legacy names. cfg.LegacyScopes expands the pre-tree names a token may
//     still carry (ap:rest_api:manage) into the tree permission each grants
//     (ap:api:rest:manage), which then matches the generated list exactly as
//     a native tree token would — including through the parent entries above.
//
// The expanded set is recorded on the request for the handlers' own checks
// (HasEffectiveScope) whether or not enforcement is enabled, so a handler's
// ownership-override decision sees the same scopes the enforcer did.
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
			// Resolve the effective scopes once — claim or expanded roles, then
			// legacy names translated — and hand the same set to the handlers.
			effective := cfg.LegacyScopes.Expand(rawEffectiveScopes(r, mode))
			r = withEffectiveScopes(r, effective)

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

			for _, required := range requiredScopes {
				for _, have := range effective {
					if have == required {
						next.ServeHTTP(w, r)
						return
					}
				}
			}

			writeError(w, apperror.Forbidden.New(), "insufficient scopes for route")
		})
	}, nil
}

// hasPathPrefix reports whether reqPath is, or is nested under, any of the given
// path prefixes: "/health" covers "/health" and "/health/live" but not
// "/health-probe-fake", and "/api/internal/v1/secrets" does not cover
// "/api/internal/v1/secrets-admin".
//
// It delegates to authenticators.HasPathPrefix — the same matcher the IDP-mode
// auth middleware applies to this same skip list — so authentication and scope
// enforcement cannot drift into exempting different sets of requests, whichever
// auth mode is configured.
func hasPathPrefix(reqPath string, prefixes []string) bool {
	return authenticators.HasPathPrefix(reqPath, prefixes)
}

// rawEffectiveScopes returns the scopes the request holds before any legacy
// translation: the scope claim in "scope" mode, or the IDP roles expanded via
// the role-scope map in "role" mode.
func rawEffectiveScopes(r *http.Request, mode string) []string {
	if mode == ValidationModeRole {
		roles, _ := GetPlatformRolesFromRequest(r)
		return roles
	}
	scope, _ := GetScopeFromRequest(r)
	return strings.Fields(scope)
}

// resolveEffectiveScopes returns the effective scopes for the request: the set
// ScopeEnforcer recorded (legacy names translated) when it ran, otherwise the
// raw claim or roles.
func resolveEffectiveScopes(r *http.Request, mode string) []string {
	if recorded, ok := r.Context().Value(keyEffectiveScopes).(*effectiveScopes); ok {
		return recorded.scopes
	}
	return rawEffectiveScopes(r, mode)
}

// HasEffectiveScope reports whether the request's effective scopes (the scope
// claim in "scope" mode, or IDP roles expanded via the role-scope map in
// "role" mode, with legacy names translated by ScopeEnforcer) include the
// given scope. Matching is exact, the same way ScopeEnforcer matches: a scope
// grants only what it names, so every scope a caller needs must be declared in
// the OpenAPI spec and held outright — or held by a legacy name that
// resources/legacy-scope-mapping.yaml maps onto it.
func HasEffectiveScope(r *http.Request, mode, scope string) bool {
	for _, have := range resolveEffectiveScopes(r, mode) {
		if have == scope {
			return true
		}
	}
	return false
}
