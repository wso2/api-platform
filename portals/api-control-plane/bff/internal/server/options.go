/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package server

import "net/http"

// Options extends the BFF's default route set for a host binary that embeds
// this module (see the exported bff/app package) — e.g. cloud-control-plane's
// own BFF, which registers cloud-only routes and hides/overrides a handful of
// standalone ones without forking this module. The zero value reproduces
// today's exact standalone behavior: every field is additive/optional, and a
// caller that never sets one gets the unmodified default for it. New(...)
// accepts Options variadically specifically so every existing call site
// (this module's own main.go, its tests) keeps compiling unchanged.
type Options struct {
	// ExtraRoutes are registered on the same mux as every default route, so
	// they run through the same middleware chain (CSRF, security headers,
	// session resolution) automatically. Keyed like http.ServeMux patterns,
	// e.g. "POST /api/environments". A pattern that collides with a default
	// route's pattern is a caller bug (net/http.ServeMux.Handle panics on a
	// duplicate registration) — construct these with care, since it is not
	// validated here.
	ExtraRoutes map[string]http.Handler

	// DisabledRoutes lists default route patterns (matching the exact string
	// passed to register() in routes()) to skip registering entirely. A real
	// 404 for that pattern, not merely a hidden UI element.
	DisabledRoutes []string

	// RouteOverrides replaces a default route's handler outright, keyed by
	// the same pattern the default registration uses.
	RouteOverrides map[string]http.Handler

	// WrapRoute wraps a default route's handler instead of replacing it — for
	// augmenting behavior (e.g. injecting host-resolved data into a proxied
	// request) while still delegating to the original handler.
	WrapRoute map[string]func(http.Handler) http.Handler
}
