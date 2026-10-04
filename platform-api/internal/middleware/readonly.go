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

// TEMP-READ-ONLY-MODE: this whole file is part of the temporary organization-scoped
// read-only mode used while Bijira migrates from Platform API v1 to v2. Delete it
// when the mode is removed — see README.md "Read-only (maintenance) mode — temporary"
// for the removal checklist.

package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
)

// readOnlyExemptRoutes lists routes that use a mutating HTTP method but perform
// no database write, keyed by the full "METHOD /path" route pattern exactly as
// it is registered on the mux. They stay available in read-only mode.
// validate-openapi only parses the uploaded spec; fetch-server-info and
// fetch-agent-card only probe an upstream (the Agent Card fetch goes through an
// in-process cache) — none of them writes to the database.
//
// The two /api/internal/v1 entries are the gateway sync reads. They carry no
// organization in the request context (gateway-token routes bypass the user-JWT
// chain), so ReadOnlyGuard lets them through anyway; listing them here keeps
// that true even if the auth chain ever starts populating the organization for
// gateway routes — blocking either would break every gateway's sync.
//
// The set is intentionally fail-closed: a read-style POST that is not listed
// here is blocked for a read-only organization, the safe direction during
// maintenance. ValidateReadOnlyExemptRoutes checks at startup that every entry
// is still a registered route, so a renamed or removed route cannot leave a
// stale exemption behind unnoticed.
var readOnlyExemptRoutes = map[string]struct{}{
	"POST " + constants.APIBasePath + "/rest-apis/validate-openapi":     {},
	"POST " + constants.APIBasePath + "/mcp-proxies/fetch-server-info":  {},
	"POST " + constants.APIBasePath + "/agent-proxies/fetch-agent-card": {},
	"POST /api/internal/v1/deployments/fetch-batch":                     {},
	"POST /api/internal/v1/artifacts/exists":                            {},
}

// ReadOnlyGuardConfig holds the dependencies of ReadOnlyGuard.
type ReadOnlyGuardConfig struct {
	// ReadOnly is the read-only mode configuration. A nil value, or one with
	// Enabled false, makes the guard a pass-through.
	ReadOnly *config.ReadOnly
	// Routes resolves the route pattern a request will match. Required whenever
	// ReadOnly is enabled: the guard runs as an outer middleware, ahead of the
	// router, so r.Pattern is still empty when it executes and the exempt-route
	// lookup can only be keyed off a pattern resolved from the router itself.
	// Pass the *http.ServeMux the routes are registered on.
	Routes RouteMatcher
	// SkipPaths are the auth skip-path prefixes: the routes authenticated by
	// something other than a user JWT (gateway token, webhook signature, login),
	// which therefore carry no organization in the request context. Only these
	// may pass the guard without an organization — their writing handlers apply
	// the check themselves once the organization is known. Required whenever
	// ReadOnly is enabled. Pass config.Auth.SkipPaths, the same list the
	// authentication middleware and ScopeEnforcer use, matched through the same
	// hasPathPrefix, so the three cannot drift into exempting different requests.
	SkipPaths []string
	// Logger receives one Warn line per rejected request. Defaults to slog.Default().
	Logger *slog.Logger
}

// ReadOnlyGuard returns a middleware that rejects write requests — any method
// other than GET, HEAD or OPTIONS — with HTTP 503 (apperror.OrganizationReadOnly)
// when the request's organization is read-only under config.ReadOnly.
//
// It must sit after authentication, organization resolution and scope
// enforcement in the chain, so those behave exactly as before — an unauthorized
// caller still gets its 401/403 — and only a write that would otherwise have
// succeeded is turned into a 503. The organization is read from the request
// context (GO-AUTH-005), never from request input.
//
// A request that carries no organization in the context passes through only
// when it is on an auth skip path — the gateway-token routes under
// /api/internal/v1, the webhook receiver, the login endpoint — where the
// organization is only known inside the handler once the gateway token or
// webhook signature has been verified; the handlers that write on those routes
// apply the same check themselves. On any other route a missing organization
// is treated as read-only (fail closed): the auth chain normally guarantees an
// organization there, but an IDP token without the organization claim is
// accepted by the claims middleware, and such a caller must not be able to
// write during a freeze just because it could not be attributed. Requests the
// router matches nothing for also pass through, so an unknown path keeps
// producing the router's 404/405 rather than a 503.
//
// 503 is deliberate: the condition is temporary and operator-driven, and the
// gateway-controller treats 401/403/404/409/422 from the control plane as
// permanent failures and exits, but retries 503.
//
// It returns an error when the configuration cannot do what it claims
// (GO-AUTH-011): read-only mode enabled with no route matcher, without which no
// exempt route could be recognised, or with no skip paths, without which every
// gateway-token and webhook write would be rejected here, before its handler
// could identify the organization, for writable organizations too.
func ReadOnlyGuard(cfg ReadOnlyGuardConfig) (func(http.Handler) http.Handler, error) {
	ro := cfg.ReadOnly
	if ro != nil && ro.Enabled {
		if cfg.Routes == nil {
			return nil, errors.New("read-only mode is enabled but no route matcher was provided — " +
				"without it the exempt read-only routes cannot be recognised")
		}
		if len(cfg.SkipPaths) == 0 {
			return nil, errors.New("read-only mode is enabled but no auth skip paths were provided — " +
				"without them the gateway-token and webhook routes would be rejected before their " +
				"handlers could identify the organization")
		}
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ro == nil || !ro.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}

			// r.Pattern is only populated once ServeMux has matched the request,
			// which happens after this middleware runs — resolve the pattern from
			// the router directly. r.Pattern is still preferred when non-empty so
			// the guard keeps working if it is ever moved inside the router.
			pattern := r.Pattern
			if pattern == "" {
				_, pattern = cfg.Routes.Handler(r)
			}
			if pattern == "" {
				// No registered route matched: let the router answer with its
				// own 404/405 rather than turning an unknown path into a 503.
				next.ServeHTTP(w, r)
				return
			}

			if _, exempt := readOnlyExemptRoutes[pattern]; exempt {
				next.ServeHTTP(w, r)
				return
			}

			org, ok := GetOrganizationFromRequest(r)
			if (!ok || org == "") && hasPathPrefix(r.URL.Path, cfg.SkipPaths) {
				// A skip-path route (gateway token, webhook signature, login):
				// the organization is resolved, and the check applied, inside
				// the handler. Anywhere else a missing organization falls
				// through to IsReadOnlyOrg, which treats it as read-only.
				next.ServeHTTP(w, r)
				return
			}

			if !ro.IsReadOnlyOrg(org) {
				next.ServeHTTP(w, r)
				return
			}

			logger.Warn("Rejected write request in read-only mode",
				"organization", org, "method", r.Method, "route", pattern)
			apperror.WriteHTTP(w, apperror.OrganizationReadOnly.New(), "")
		})
	}, nil
}

// ValidateReadOnlyExemptRoutes checks that every route readOnlyExemptRoutes
// exempts is registered on the router under exactly that pattern. An exemption
// for a route that no longer exists, or that was renamed, is a stale entry
// that would silently stop exempting the route it was written for — refuse to
// start instead (GO-AUTH-011), whether or not read-only mode is currently
// enabled, so the drift is caught before the mode is ever switched on.
func ValidateReadOnlyExemptRoutes(routes RouteMatcher) error {
	if routes == nil {
		return nil
	}

	var problems []string
	for key := range readOnlyExemptRoutes {
		method, path, _ := strings.Cut(key, " ")
		req, err := http.NewRequest(method, path, nil)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: not a probeable route: %v", key, err))
			continue
		}
		if _, pattern := routes.Handler(req); pattern != key {
			problems = append(problems, fmt.Sprintf(
				"%s: exempt from read-only mode, but the router matched %q — "+
					"update readOnlyExemptRoutes to the route's current pattern, or drop the entry",
				key, pattern))
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("read-only exempt routes do not match the registered routes:\n  %s",
			strings.Join(problems, "\n  "))
	}
	return nil
}
