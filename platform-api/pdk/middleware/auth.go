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

// Package middleware is the public tier of the platform's request-context
// helpers. It mirrors the internal/middleware package structure, re-exporting
// only the accessors that are part of the public contract so external plugins
// read auth-chain values through a stable surface, never the internal package.
package middleware

import (
	"net/http"

	internalmiddleware "github.com/wso2/api-platform/platform-api/internal/middleware"
)

// GetOrganizationFromRequest returns the authenticated organization UUID resolved
// by the platform auth chain from the request context, and whether it was present.
//
// Plugin handlers MUST scope tenant data by this value, never by an organization
// id taken from request path, query, or body (GO-AUTH-005). It is populated for
// every route that is not an auth skip-path.
//
// It wraps the internal context accessor so the context key stays a single source
// of truth; external plugins reach it only through this public helper.
func GetOrganizationFromRequest(r *http.Request) (string, bool) {
	return internalmiddleware.GetOrganizationFromRequest(r)
}
