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

	commonconstants "github.com/wso2/api-platform/common/constants"

	"github.com/gin-gonic/gin"
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
	// Enabled controls whether scope checks are enforced. When false, all authenticated
	// requests are allowed regardless of scope.
	Enabled bool
}

// ScopeEnforcer returns a Gin middleware that reads the required scopes for each
// request from the OpenAPI ScopeRegistry and enforces them.
//
// The validation path is determined by cfg.ValidationMode. Routes not present in
// the registry are passed through without a scope check, relying on authentication alone.
func ScopeEnforcer(registry *ScopeRegistry, cfg ScopeEnforcerConfig) gin.HandlerFunc {
	mode := cfg.ValidationMode
	if mode == "" {
		mode = ValidationModeScope
	}

	return func(c *gin.Context) {
		if !cfg.Enabled {
			c.Next()
			return
		}

		if v, ok := c.Get(commonconstants.AuthzSkipKey); ok {
			if skip, ok2 := v.(bool); ok2 && skip {
				c.Next()
				return
			}
		}

		requiredScopes, found := registry.Lookup(c.Request.Method, c.FullPath())
		if !found || len(requiredScopes) == 0 {
			c.Next()
			return
		}

		effectiveScopes := resolveEffectiveScopes(c, mode)

		for _, required := range requiredScopes {
			for _, have := range effectiveScopes {
				if scopeSatisfies(have, required) {
					c.Next()
					return
				}
			}
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		c.Abort()
	}
}

// scopeSatisfies reports whether a held scope grants a required scope.
//
// A required scope (sourced from the OpenAPI spec) is always concrete:
// <prefix>:<resource>[:<sub-resource>...]:<action>. A held scope may be concrete
// (exact match) or a wildcard ending in ":*".
//
// Wildcards are own-level only — they cover every action directly at their level
// and never descend into sub-resources, never match a prefix, never transitively:
//
//	<prefix>:*                     covers <prefix>:<resource>:<action>
//	                               (every action on root-level resources;
//	                                NOT <prefix>:<resource>:<sub>:<action>)
//	<prefix>:<resource>:*          covers <prefix>:<resource>:<action>
//	                               (all actions directly on the resource;
//	                                NOT its sub-resources)
//	<prefix>:<resource>:<sub>:*    covers <prefix>:<resource>:<sub>:<action>
//	                               (all actions directly on that sub-resource)
//
// The top-level wildcard (one segment before ":*", e.g. "ap:*") expands across
// every root resource, so it covers a resource + action pair (two trailing
// segments). Every deeper wildcard covers exactly one trailing segment — the
// action — at its own level.
func scopeSatisfies(have, required string) bool {
	if have == required {
		return true
	}
	if !strings.HasSuffix(have, ":*") {
		return false
	}
	// base keeps the trailing colon, e.g. "ap:" or "ap:gateway:".
	base := strings.TrimSuffix(have, "*")
	if !strings.HasPrefix(required, base) {
		return false
	}
	remainder := required[len(base):]
	if remainder == "" {
		return false
	}
	segments := strings.Count(remainder, ":") + 1
	// A top-level namespace wildcard ("<prefix>:*") leaves only the prefix and its
	// trailing colon in base (a single colon), so it covers a resource + action
	// pair. Every deeper wildcard covers a single trailing action segment at its
	// own level.
	if strings.Count(base, ":") == 1 {
		return segments == 2
	}
	return segments == 1
}

// resolveEffectiveScopes returns the effective scopes for the request.
// In scope mode it reads the JWT scope claim directly.
// In role mode it returns the platform_roles resolved by PlatformClaimsMiddleware,
// allowing role names to be matched against the scope registry entries.
func resolveEffectiveScopes(c *gin.Context, mode string) []string {
	if mode == ValidationModeRole {
		raw, _ := c.Get("platform_roles")
		roles, _ := raw.([]string)
		return roles
	}
	raw, _ := c.Get("scope")
	scopeStr, _ := raw.(string)
	return strings.Fields(scopeStr)
}
