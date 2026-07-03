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

// routes builds the mux and wraps it with the global middleware chain.
func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// Health (no auth, no CSRF).
	mux.HandleFunc("GET /healthz", handleHealth)

	// Runtime config consumed by the SPA before app init.
	mux.HandleFunc("GET /runtime-config.js", s.handleRuntimeConfig)

	// Auth endpoints.
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	mux.HandleFunc("GET /api/session", s.handleSession)
	mux.HandleFunc("GET /api/auth/login", s.handleOIDCLogin)
	mux.HandleFunc("GET /api/auth/callback", s.handleOIDCCallback)

	// Composite BFF endpoints — orchestrate two Platform API calls with
	// server-side compensation on failure. Must be registered before the
	// catch-all proxy so these paths are not forwarded upstream as-is.
	mux.HandleFunc("POST /api/bff/llm-providers", s.handleCreateLLMProvider)
	mux.HandleFunc("POST /api/bff/mcp-proxies", s.handleCreateMCPServer)

	// Same-origin reverse proxy to the Platform API. The proxy's Rewrite hook
	// strips the prefix before forwarding, so we register the subtree directly.
	mux.HandleFunc(s.cfg.ProxyPrefix+"/", s.handleProxy)

	// SPA static files + client-side routing fallback (must be last).
	mux.Handle("/", spaHandler(s.cfg.StaticDir))

	return chain(mux,
		recoverPanic,
		requestID,
		logRequests,
		securityHeaders,
		s.requireCSRF,
	)
}
