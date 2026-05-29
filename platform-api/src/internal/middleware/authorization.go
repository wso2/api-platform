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

	"platform-api/src/internal/rbac"

	commonconstants "github.com/wso2/api-platform/common/constants"

	"github.com/gin-gonic/gin"
)

const (
	// ValidationModeScope validates using the JWT scope claim directly.
	ValidationModeScope = "scope"
	// ValidationModeRole expands platform roles to their full permission set and validates against that.
	ValidationModeRole = "role"
)

var rbacEnabled = true

// SetRBACEnabled controls whether scope checks are enforced globally.
// When false, all authenticated requests are allowed regardless of scope.
func SetRBACEnabled(enabled bool) {
	rbacEnabled = enabled
}

// InitScopeAuthz is retained for compatibility.
func InitScopeAuthz() {}

// InitClaimsAuthz is retained for compatibility.
func InitClaimsAuthz() {}

// ScopeEnforcerConfig holds options for the ScopeEnforcer middleware.
type ScopeEnforcerConfig struct {
	// ValidationMode controls how the caller's permissions are resolved.
	// ValidationModeScope ("scope"): read the JWT scope claim directly.
	// ValidationModeRole  ("role"):  expand platform_roles to their full permission set.
	// Defaults to ValidationModeScope when empty.
	ValidationMode string
}

// ScopeEnforcer returns a Gin middleware that reads the required scopes for each
// request from the OpenAPI ScopeRegistry and enforces them.
//
// The validation path is determined entirely by cfg.ValidationMode — there is no
// fallback between the two modes. Routes not present in the registry are passed
// through without a scope check, relying on authentication alone.
func ScopeEnforcer(registry *ScopeRegistry, cfg ScopeEnforcerConfig) gin.HandlerFunc {
	mode := cfg.ValidationMode
	if mode == "" {
		mode = ValidationModeScope
	}

	return func(c *gin.Context) {
		if !rbacEnabled {
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
				if have == required {
					c.Next()
					return
				}
			}
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		c.Abort()
	}
}

// resolveEffectiveScopes returns the caller's effective scopes based on the chosen mode.
// In scope mode the JWT scope claim is used as-is.
// In role mode platform roles are expanded to their full permission set.
func resolveEffectiveScopes(c *gin.Context, mode string) []string {
	if mode == ValidationModeRole {
		roles, _ := GetPlatformRolesFromContext(c)
		return expandRolesToScopes(roles)
	}
	// scope mode
	raw, _ := c.Get("scope")
	scopeStr, _ := raw.(string)
	return strings.Fields(scopeStr)
}

// expandRolesToScopes converts platform role names to the full list of scope strings
// those roles grant.
func expandRolesToScopes(roles []string) []string {
	if len(roles) == 0 {
		return nil
	}
	perms := rbac.PermissionsForRoles(roles)
	scopes := make([]string, 0, len(perms))
	for perm := range perms {
		scopes = append(scopes, perm.Scope())
	}
	return scopes
}
