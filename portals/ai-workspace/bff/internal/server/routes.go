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

import (
	"net/http"

	"ai-workspace-bff/internal/paths"
)

// routes builds the mux and wraps it with the global middleware chain. Every route
// below is registered under paths.Base via s.path, so an ingress can route one
// prefix here without rewriting paths and an all-in-one deployment can host several
// portals on a single host.
func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// Health, at the origin root: the container HEALTHCHECK and Kubernetes probes
	// dial the pod directly, bypassing whatever ingress adds the prefix, so this one
	// route must stay reachable outside the base path. It is also registered under the
	// prefix so a probe that does come through the ingress works too.
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("GET "+s.path("/healthz"), handleHealth)

	// Convenience for a direct visit to the origin root (localhost, all-in-one): send
	// it to the app instead of a bare 404. Only the exact root — every other
	// unprefixed path stays a 404, since it belongs to whatever else the ingress
	// routes on this host. ServeMux itself redirects the bare prefix
	// ("/ai-workspace") to the subtree registered below, so that needs no route.
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, s.path("/"), http.StatusFound)
	})

	// Runtime config consumed by the SPA before app init.
	mux.HandleFunc("GET "+s.path("/runtime-config.js"), s.handleRuntimeConfig)

	// Auth endpoints.
	mux.HandleFunc("POST "+s.path("/api/login"), s.handleLogin)
	mux.HandleFunc("POST "+s.path("/api/logout"), s.handleLogout)
	mux.HandleFunc("GET "+s.path("/api/session"), s.handleSession)
	mux.HandleFunc("GET "+s.path("/api/auth/login"), s.handleOIDCLogin)
	mux.HandleFunc("GET "+s.path("/api/auth/callback"), s.handleOIDCCallback)

	// Creates the BFF owns rather than forwards: each spans two Platform API calls and
	// compensates server-side if the second fails, which a pass-through cannot do (see
	// composite_handlers.go). They are named for their resource, like every other route
	// in this /api namespace — that the BFF orchestrates instead of forwarding is an
	// implementation detail the browser should not read off a URL.
	//
	// Registered here, outside the proxy prefix, rather than intercepting the
	// pass-through path for the same resource: that would put the upstream API version
	// in a browser-facing route, where bumping it silently stops matching and disables
	// the compensation with no error anywhere. The version stays in the handler alone.
	mux.HandleFunc("POST "+s.path("/api/llm-providers"), s.handleCreateLLMProvider)
	mux.HandleFunc("POST "+s.path("/api/mcp-proxies"), s.handleCreateMCPServer)

	// Same-origin reverse proxy to the Platform API. The proxy's Rewrite hook
	// strips the base path and the proxy prefix before forwarding (see
	// server.New), so we register the subtree directly.
	mux.HandleFunc(s.path(paths.Proxy)+"/", s.handleProxy)

	// SPA static files + client-side routing fallback (must be last). The prefix is
	// stripped so file lookups resolve against the static dir, not a directory named
	// after the base path.
	mux.Handle(s.path("/"), http.StripPrefix(paths.Base, spaHandler(s.cfg.Server.StaticDir)))

	return chain(mux,
		recoverPanic,
		requestID,
		logRequests,
		securityHeaders,
		s.requireCSRF,
	)
}
