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

// Composite BFF handlers that orchestrate two Platform API calls atomically
// from the browser's perspective. The browser sends one request; the BFF
// forwards it to the Platform API and, on failure, compensates by deleting
// any secret that was already created before the main resource call.
//
// This covers the unrecoverable edge case that client-side compensation cannot
// handle: createSecret succeeds → createResource fails → deleteSecret also
// fails (e.g. the tab closed or the network died mid-compensation).

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
)

// secretHandleRE matches {{ secret "handle" }} placeholders embedded in JSON
// bodies. The quotes may be JSON-escaped (\") when the placeholder appears as
// the value of a JSON string field, so both forms are matched. The handle is
// the first capture group.
var secretHandleRE = regexp.MustCompile(`\{\{\s*secret\s+\\?"([^"\\]+)\\?"\s*\}\}`)

// extractSecretHandle returns the first secret handle found in body, or "".
func extractSecretHandle(body []byte) string {
	m := secretHandleRE.FindSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	return string(m[1])
}

// platformDo performs a single authenticated request against the Platform API,
// returning the raw response. The caller is responsible for closing the body.
func (s *Server) platformDo(ctx context.Context, jwt, method, path string, header http.Header, body []byte) (*http.Response, error) {
	url := strings.TrimRight(s.cfg.PlatformAPIURL, "/") + path
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, err
	}
	// Forward relevant headers from the original request (Content-Type, etc.)
	for k, vs := range header {
		k = http.CanonicalHeaderKey(k)
		// Never forward hop-by-hop or auth headers — we set our own.
		switch k {
		case "Authorization", "Cookie", "Connection", "Te", "Trailers", "Transfer-Encoding", "Upgrade":
			continue
		}
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	return s.platformClient().Do(req)
}

// platformClient returns an *http.Client backed by the same transport used by
// the reverse proxy (shared connection pool, same TLS skip-verify setting).
func (s *Server) platformClient() *http.Client {
	return &http.Client{Transport: s.proxy.Transport}
}

// deleteSecretAsync fires a best-effort DELETE /secrets/{handle} in a new
// goroutine. Used for compensation after a resource creation failure. Errors
// are logged but do not affect the caller.
func (s *Server) deleteSecretAsync(jwt, handle, apiBase string) {
	go func() {
		path := apiBase + "/secrets/" + handle
		resp, err := s.platformDo(context.Background(), jwt, http.MethodDelete, path, nil, nil)
		if err != nil {
			slog.Warn("bff: secret compensation DELETE failed", "handle", handle, "err", err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			slog.Warn("bff: secret compensation DELETE returned error", "handle", handle, "status", resp.StatusCode)
		}
	}()
}

// handleCreateWithSecretCompensation is the shared implementation for composite
// create endpoints. It:
//  1. Reads and buffers the request body.
//  2. Forwards POST <resourcePath> to the Platform API.
//  3. On a non-2xx response, extracts the secret handle from the request body
//     and fires DELETE /secrets/{handle} as best-effort compensation.
//  4. Relays the Platform API response (status + body) verbatim to the caller.
//
// apiBasePath is the versioned API prefix, e.g. "/api/v0.9".
func (s *Server) handleCreateWithSecretCompensation(w http.ResponseWriter, r *http.Request, resourcePath, apiBasePath string) {
	jwt, ok := s.tokenFromCookie(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read request body"})
		return
	}

	resp, err := s.platformDo(r.Context(), jwt, http.MethodPost, apiBasePath+resourcePath, r.Header, body)
	if err != nil {
		slog.Error("bff: platform API call failed", "path", resourcePath, "err", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "upstream request failed"})
		return
	}
	defer resp.Body.Close()

	// On failure, compensate by deleting the secret that was already created.
	if resp.StatusCode >= 400 {
		if handle := extractSecretHandle(body); handle != "" {
			s.deleteSecretAsync(jwt, handle, apiBasePath)
		}
	}

	// Relay the Platform API response verbatim.
	respBody, _ := io.ReadAll(resp.Body)
	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)
}

// handleCreateLLMProvider (POST /api/bff/llm-providers) — composite endpoint
// that creates an LLM provider and compensates on failure by deleting the
// pre-created secret.
func (s *Server) handleCreateLLMProvider(w http.ResponseWriter, r *http.Request) {
	s.handleCreateWithSecretCompensation(w, r, "/llm-providers", "/api/v0.9")
}

// handleCreateMCPServer (POST /api/bff/mcp-servers) — composite endpoint
// that creates an MCP server and compensates on failure by deleting the
// pre-created secret.
func (s *Server) handleCreateMCPServer(w http.ResponseWriter, r *http.Request) {
	s.handleCreateWithSecretCompensation(w, r, "/mcp-proxies", "/api/v0.9")
}
