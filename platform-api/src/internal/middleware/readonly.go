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
	"log/slog"
	"net/http"

	"platform-api/src/config"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
)

// ReadOnlyErrorDescription is the description returned with every read-only mode 503.
// It is shared with the handler-level checks on gateway-token routes (gateway_internal.go),
// which have no organization in the request context and so bypass this middleware's lookup.
const ReadOnlyErrorDescription = "Organization is in read-only mode: write operations are temporarily disabled for maintenance"

// readOnlyExemptRoutes lists routes that use a mutating HTTP method but perform no
// database writes, keyed by "METHOD <gin route template>". They stay available in
// read-only mode. The two /api/internal/v1 entries are gateway sync reads — blocking
// them would break every gateway in all-organizations mode. The set is intentionally
// fail-closed: a read-style POST that is not listed here is blocked during a freeze.
var readOnlyExemptRoutes = map[string]struct{}{
	"POST /api/portal/v1/auth/login":                {},
	"POST /api/v1/rest-apis/validate-openapi":       {},
	"POST /api/v1/api-projects/validate":            {},
	"POST /api/v1/mcp-proxies/fetch-server-info":    {},
	"POST /api/v1/git/repo/fetch-branches":          {},
	"POST /api/v1/git/repo/branch/fetch-content":    {},
	"POST /api/internal/v1/deployments/fetch-batch": {},
	"POST /api/internal/v1/artifacts/exists":        {},
}

// ReadOnlyGuard returns a Gin middleware that rejects write requests (any method other
// than GET, HEAD or OPTIONS) with HTTP 503 when the request's organization is in
// read-only mode. It must be registered after the authentication middleware so the
// organization is already in the context.
//
// Requests without an organization in the context (gateway-token routes under
// Auth.SkipPaths) pass through in list mode and are guarded inside their handlers;
// in all-organizations mode they are rejected here. 503 is used deliberately: the
// gateway-controller treats 401/403/404/409/422 as permanent failures and exits, but
// retries 503.
func ReadOnlyGuard(ro *config.ReadOnly, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !ro.Enabled() {
			c.Next()
			return
		}

		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}

		// Gin runs global middleware for unmatched routes too; let those return 404.
		fullPath := c.FullPath()
		if fullPath == "" {
			c.Next()
			return
		}

		if _, exempt := readOnlyExemptRoutes[c.Request.Method+" "+fullPath]; exempt {
			c.Next()
			return
		}

		orgID, _ := GetOrganizationFromContext(c)
		if !ro.IsReadOnlyOrg(orgID) {
			c.Next()
			return
		}

		if logger != nil {
			logger.Info("Rejected write request in read-only mode",
				"organization", orgID, "method", c.Request.Method, "path", fullPath)
		}
		c.JSON(http.StatusServiceUnavailable, utils.NewErrorResponse(http.StatusServiceUnavailable,
			"Service Unavailable", ReadOnlyErrorDescription))
		c.Abort()
	}
}
