/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package handlers

import (
	"net/http"

	"github.com/wso2/go-httpkit/httputil"
)

// ProtectedResourceMetadata is the OAuth 2.0 Protected Resource Metadata
// document (RFC 9728). An MCP client reads it, unauthenticated, to learn which
// authorization server issues tokens for this endpoint. The MCP specification
// makes implementing it a MUST for a protected MCP server.
type ProtectedResourceMetadata struct {
	// Resource is the canonical URI of the MCP endpoint. Clients bind their
	// token request to it via the RFC 8707 `resource` parameter, and the
	// gateway must accept only tokens whose audience matches it.
	Resource string `json:"resource"`
	// AuthorizationServers lists issuer identifiers the client may use. The
	// MCP specification requires at least one.
	AuthorizationServers []string `json:"authorization_servers"`
	// ScopesSupported advertises the scopes this resource understands. Kept to
	// the set that grants baseline access; anything beyond it is requested
	// incrementally through step-up challenges.
	ScopesSupported []string `json:"scopes_supported,omitempty"`
	// BearerMethodsSupported: this resource accepts the token in the header only.
	BearerMethodsSupported []string `json:"bearer_methods_supported,omitempty"`
}

// NewProtectedResourceMetadataHandler serves the static RFC 9728 document.
//
// It MUST NOT sit behind authentication: a client with no token has to read it
// to begin the OAuth flow. It is registered directly on the mux rather than
// through the authenticated router, so it is exempt by routing structure rather
// than by a path-prefix exception (GO-AUTH-004).
func NewProtectedResourceMetadataHandler(meta ProtectedResourceMetadata) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Public, cacheable, non-sensitive metadata — readable cross-origin so
		// browser-based MCP clients can fetch it.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		httputil.WriteJSON(w, http.StatusOK, meta)
	}
}
