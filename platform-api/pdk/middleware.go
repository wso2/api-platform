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

package pdk

import "net/http"

// Middleware is a standard Go middleware — it wraps one handler with another.
type Middleware func(http.Handler) http.Handler

// ChainPosition selects where a plugin's middleware is spliced into the server's
// request chain. Only two positions are exposed on purpose: a plugin can wrap the
// whole request or run with the authenticated identity, without the platform's
// internal middleware order becoming part of the public contract. A plugin cannot
// insert middleware *between* platform steps (e.g. between auth and the scope
// enforcer).
type ChainPosition int

const (
	// BeforePlatformChain is outermost: it runs before CORS and auth, on every
	// request, with NO authenticated identity in the context yet. Good for request
	// IDs, tracing, panic recovery, IP allow-lists. It cannot mark a request as
	// authenticated — the platform's auth always runs after it and is the only
	// thing that sets identity, so GO-AUTH-005 still holds.
	BeforePlatformChain ChainPosition = iota
	// AfterPlatformChain is innermost: it runs after auth, organization
	// resolution, and scope enforcement, just before routing. The authenticated
	// org and identity are in the context and can be read with
	// middleware.GetOrganizationFromRequest.
	// Good for per-tenant rate limiting, audit, or request enrichment — still
	// scoped by the org from context (GO-AUTH-005).
	AfterPlatformChain
)

// PositionedMiddleware pairs a middleware with the position it should occupy in
// the chain.
type PositionedMiddleware struct {
	Position ChainPosition
	Wrap     Middleware
}

// MiddlewareProvider is an OPTIONAL interface a Plugin may implement (like
// AuthSkipPathProvider) to contribute middleware to the request chain. Return an
// empty slice to add none. Within a position, middleware runs in plugin
// registration order.
type MiddlewareProvider interface {
	Middleware() []PositionedMiddleware
}
