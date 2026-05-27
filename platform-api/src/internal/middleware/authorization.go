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

var rbacEnabled = true

// SetRBACEnabled controls whether permission checks are enforced globally.
// When false, all authenticated requests are allowed regardless of scope.
func SetRBACEnabled(enabled bool) {
	rbacEnabled = enabled
}

// InitScopeAuthz configures scope-based authorization (Thunder mode).
// Permissions are resolved by checking perm.Scope() against the space-separated
// scope claim that Thunder embeds in the JWT — no runtime identity-service call needed.
func InitScopeAuthz() {}

// InitClaimsAuthz is retained for IDP mode where tokens carry platform role names
// instead of fine-grained scope strings.
func InitClaimsAuthz() {}

// RequirePermission returns a Gin middleware that aborts with 403 unless the
// authenticated user's token grants perm. In Thunder mode the scope claim is
// checked directly; in IDP mode platform roles are mapped to permissions as a
// fallback when the scope claim is absent or does not carry fine-grained scopes.
func RequirePermission(perm rbac.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !checkPermission(c, perm) {
			c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireAnyPermission returns a Gin middleware that aborts with 403 unless
// the authenticated user holds at least one of the given permissions.
func RequireAnyPermission(perms ...rbac.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, perm := range perms {
			if checkPermission(c, perm) {
				c.Next()
				return
			}
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		c.Abort()
	}
}

func checkPermission(c *gin.Context, perm rbac.Permission) bool {
	if !rbacEnabled {
		return true
	}
	if v, ok := c.Get(commonconstants.AuthzSkipKey); ok {
		if skip, ok2 := v.(bool); ok2 && skip {
			return true
		}
	}
	// Scope-based check: Thunder embeds fine-grained scopes directly in the JWT.
	if hasScope(c, perm.Scope()) {
		return true
	}
	// Role-based fallback: IDP tokens may carry role names (admin/developer/viewer)
	// that are mapped to permission sets rather than emitting individual scopes.
	roles, _ := GetPlatformRolesFromContext(c)
	return rbac.HasPermissionForRoles(roles, perm)
}

// hasScope reports whether the space-separated scope string stored in the Gin
// context contains target. Both ThunderAuthMiddleware and PlatformClaimsMiddleware
// write to the "scope" key, so this works regardless of which JWT path is active.
func hasScope(c *gin.Context, target string) bool {
	raw, _ := c.Get("scope")
	scopeStr, _ := raw.(string)
	for _, s := range strings.Fields(scopeStr) {
		if s == target {
			return true
		}
	}
	return false
}
