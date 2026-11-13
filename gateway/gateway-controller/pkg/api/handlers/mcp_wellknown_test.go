/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const wellKnownPath = "/.well-known/oauth-protected-resource/api/management/v1/mcp"

var wellKnownMeta = ProtectedResourceMetadata{
	Resource:               "https://gw.example.com/api/management/v1/mcp",
	AuthorizationServers:   []string{"https://idp.example.com/oauth2/token"},
	ScopesSupported:        []string{"gw:read", "gw:write"},
	BearerMethodsSupported: []string{"header"},
}

func serveWellKnown(t *testing.T, h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// wellKnownDoc decodes the body into a generic object, so assertions are made
// on the JSON names a client sees rather than on the Go struct.
func wellKnownDoc(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var doc map[string]any
	require.NoErrorf(t, json.Unmarshal(rec.Body.Bytes(), &doc), "body is not a JSON object: %q", rec.Body.String())
	return doc
}

// RFC 9728 section 3.2: a successful response MUST be 200 OK with a JSON
// object in application/json, using the parameter names of section 2.
func TestWellKnownServesTheMetadataDocument(t *testing.T) {
	rec := serveWellKnown(t, NewProtectedResourceMetadataHandler(wellKnownMeta),
		httptest.NewRequest(http.MethodGet, wellKnownPath, nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Equal(t, map[string]any{
		"resource":                 "https://gw.example.com/api/management/v1/mcp",
		"authorization_servers":    []any{"https://idp.example.com/oauth2/token"},
		"scopes_supported":         []any{"gw:read", "gw:write"},
		"bearer_methods_supported": []any{"header"},
	}, wellKnownDoc(t, rec), "exactly these four RFC 9728 parameters, and nothing else")
}

// The document is public and static. Browser-based MCP clients fetch it
// cross-origin, and it may be cached for an hour.
func TestWellKnownIsPublicAndCacheable(t *testing.T) {
	rec := serveWellKnown(t, NewProtectedResourceMetadataHandler(wellKnownMeta),
		httptest.NewRequest(http.MethodGet, wellKnownPath, nil))

	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "public, max-age=3600", rec.Header().Get("Cache-Control"))
}

// Empty optional lists are omitted, as RFC 9728 section 3.2 requires.
func TestWellKnownOmitsEmptyOptionalParameters(t *testing.T) {
	for name, meta := range map[string]ProtectedResourceMetadata{
		"nil lists": {
			Resource:             wellKnownMeta.Resource,
			AuthorizationServers: wellKnownMeta.AuthorizationServers,
		},
		"zero-length lists": {
			Resource:               wellKnownMeta.Resource,
			AuthorizationServers:   wellKnownMeta.AuthorizationServers,
			ScopesSupported:        []string{},
			BearerMethodsSupported: []string{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			doc := wellKnownDoc(t, serveWellKnown(t, NewProtectedResourceMetadataHandler(meta),
				httptest.NewRequest(http.MethodGet, wellKnownPath, nil)))

			assert.NotContains(t, doc, "scopes_supported")
			assert.NotContains(t, doc, "bearer_methods_supported")
			assert.Equal(t, wellKnownMeta.Resource, doc["resource"])
			assert.Equal(t, []any{"https://idp.example.com/oauth2/token"}, doc["authorization_servers"])
		})
	}
}

// The document does not depend on the request or its credentials.
func TestWellKnownIgnoresTheRequest(t *testing.T) {
	h := NewProtectedResourceMetadataHandler(wellKnownMeta)
	anonymous := serveWellKnown(t, h, httptest.NewRequest(http.MethodGet, wellKnownPath, nil))

	withToken := httptest.NewRequest(http.MethodGet, wellKnownPath+"?x=1", nil)
	withToken.Header.Set("Authorization", "Bearer some-token")
	authenticated := serveWellKnown(t, h, withToken)

	assert.Equal(t, http.StatusOK, anonymous.Code)
	assert.Equal(t, anonymous.Code, authenticated.Code)
	assert.JSONEq(t, anonymous.Body.String(), authenticated.Body.String())
}

// The document is fixed when the handler is built, so every request is served
// the same bytes.
func TestWellKnownServesTheSameDocumentOnEveryRequest(t *testing.T) {
	h := NewProtectedResourceMetadataHandler(wellKnownMeta)

	first := serveWellKnown(t, h, httptest.NewRequest(http.MethodGet, wellKnownPath, nil))
	second := serveWellKnown(t, h, httptest.NewRequest(http.MethodGet, wellKnownPath, nil))

	assert.Equal(t, first.Body.String(), second.Body.String())
}

// Mounted as main.go mounts it: GET only, exact paths, outside auth.
func TestWellKnownMountedAsInProduction(t *testing.T) {
	h := NewProtectedResourceMetadataHandler(wellKnownMeta)
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+wellKnownPath, h)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", h)

	for _, path := range []string{wellKnownPath, "/.well-known/oauth-protected-resource"} {
		rec := serveWellKnown(t, mux, httptest.NewRequest(http.MethodGet, path, nil))
		assert.Equalf(t, http.StatusOK, rec.Code, "GET %s", path)
		assert.Equal(t, wellKnownMeta.Resource, wellKnownDoc(t, rec)["resource"])
	}

	rec := serveWellKnown(t, mux, httptest.NewRequest(http.MethodPost, wellKnownPath, nil))
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)

	rec = serveWellKnown(t, mux, httptest.NewRequest(http.MethodGet, wellKnownPath+"/extra", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code, "exact patterns: no metadata for some other path")
}
