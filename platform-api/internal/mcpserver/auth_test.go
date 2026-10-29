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

package mcpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/authenticators"
	commonmodels "github.com/wso2/api-platform/common/models"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

// seen records the caller the auth layer passed on.
func seen(got *caller) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*got, _ = callerFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	})
}

func serve(h http.Handler, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, Path, nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// signedIn stands in for the Platform API's sign-in and org resolver: it leaves
// on the request what they would leave for this token.
func signedIn(org string, aud ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "" {
				apperror.WriteHTTP(w, apperror.Unauthorized.New(), "")
				return
			}
			r = middleware.WithScope(r, "ap:project:read ap:gateway:read")
			r = middleware.WithUserID(r, "alice")
			if org != "" {
				r = middleware.WithOrganization(r, org)
			}
			ctx := authenticators.WithAuthContext(r.Context(), commonmodels.AuthContext{
				Authenticated: true, Claims: map[string]any{"aud": aud}})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// countingMappings counts the user rows the identity service writes.
type countingMappings struct{ created int }

func (c *countingMappings) GetOrCreateUUID(string) (string, error)             { c.created++; return "u1", nil }
func (c *countingMappings) GetUUIDByIdentity(string) (string, error)           { return "", nil }
func (c *countingMappings) GetSubByUUID(string) (string, bool, error)          { return "", false, nil }
func (c *countingMappings) GetSubsByUUIDs([]string) (map[string]string, error) { return nil, nil }

func idpServer(t *testing.T) (*Server, *countingMappings) {
	s, _ := newTestServer(t)
	rows := &countingMappings{}
	s.identity = service.NewIdentityService(rows)
	s.auth.Mode = config.AuthModeIDP
	s.auth.IDP.Issuer = []string{"https://idp.example.com/oauth2/token"}
	s.cfg.PublicURL = "https://platform-api.example.com/"
	s.cfg.Auth.Audience = "mcp"
	return s, rows
}

func TestAuthenticate_TakesTheCallerFromThePlatformAPISignIn(t *testing.T) {
	s, _ := newTestServer(t)
	s.identity = service.NewIdentityService(&countingMappings{})
	var got caller
	h := signedIn(testOrg)(s.authenticate(seen(&got)))
	assert.Equal(t, http.StatusOK, serve(h, "t").Code)
	assert.Equal(t, testOrg, got.org)
	assert.NotEmpty(t, got.user)
	assert.Equal(t, toSet([]string{"ap:project:read", "ap:gateway:read"}), got.scopes)
}

func TestAuthenticate_NoOrgClaimIs401(t *testing.T) {
	s, _ := newTestServer(t)
	s.identity = service.NewIdentityService(&countingMappings{})
	var got caller
	assert.Equal(t, http.StatusUnauthorized, serve(signedIn("")(s.authenticate(seen(&got))), "t").Code)
}

func TestAuthenticate_IDPAudience(t *testing.T) {
	s, rows := idpServer(t)
	var got caller

	h := signedIn(testOrg, "rest")(s.authenticate(seen(&got)))
	assert.Equal(t, http.StatusUnauthorized, serve(h, "t").Code, "a token for another audience")
	assert.Zero(t, rows.created, "a refused token writes no user row")

	h = signedIn(testOrg, "rest", "mcp")(s.authenticate(seen(&got)))
	assert.Equal(t, http.StatusOK, serve(h, "t").Code, "several audiences, one of them the MCP one")
	assert.Equal(t, 1, rows.created)
}

// TestWrap_401CarriesLoginDiscovery goes through the wrapper, a stand-in
// sign-in and the caller check, in the order the Platform API chains them.
// TestChallengeWriter_FlushReachesTheInnerWriter pins Unwrap: go-sdk flushes
// through http.NewResponseController, which follows it.
func TestChallengeWriter_FlushReachesTheInnerWriter(t *testing.T) {
	recorder := httptest.NewRecorder()
	w := &challengeWriter{ResponseWriter: recorder}
	require.NoError(t, http.NewResponseController(w).Flush())
	assert.True(t, recorder.Flushed)
}

func TestWrap_401CarriesLoginDiscovery(t *testing.T) {
	s, _ := idpServer(t)
	var got caller
	h := s.Wrap(signedIn(testOrg, "mcp")(s.authenticate(seen(&got))))

	w := serve(h, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, `Bearer resource_metadata="https://platform-api.example.com/.well-known/oauth-protected-resource/api/v0.9/mcp"`,
		w.Header().Get("WWW-Authenticate"))
	assert.Equal(t, http.StatusOK, serve(h, "t").Code)

	h = s.Wrap(signedIn(testOrg, "rest")(s.authenticate(seen(&got))))
	w = serve(h, "t")
	assert.Equal(t, http.StatusUnauthorized, w.Code, "a wrong audience")
	assert.NotEmpty(t, w.Header().Get("WWW-Authenticate"))
}

func TestRegister_DiscoveryRoute(t *testing.T) {
	s, _ := idpServer(t)
	mux := http.NewServeMux()
	assert.Equal(t, []string{DiscoveryPath}, s.Register(mux))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, DiscoveryPath, nil))
	var meta struct {
		Resource string   `json:"resource"`
		Servers  []string `json:"authorization_servers"`
		Scopes   []string `json:"scopes_supported"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &meta))
	assert.Equal(t, "https://platform-api.example.com/api/v0.9/mcp", meta.Resource)
	assert.Equal(t, s.auth.IDP.Issuer, meta.Servers)
	assert.Contains(t, meta.Scopes, "ap:rest_api:read")
}

func TestRegister_SkipPaths(t *testing.T) {
	s, _ := newTestServer(t)
	s.auth.Mode = config.AuthModeFile
	assert.Empty(t, s.Register(http.NewServeMux()), "/mcp signs in through the normal chain")
}

func TestWrap_RefusesACrossOriginBrowserRequestBeforeSignIn(t *testing.T) {
	s, _ := idpServer(t)
	var got caller
	h := s.Wrap(signedIn(testOrg, "mcp")(s.authenticate(seen(&got))))

	r := httptest.NewRequest(http.MethodPost, Path, nil)
	r.Header.Set("Origin", "https://evil.example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusForbidden, w.Code, "no token, but the Origin check comes first")

	r = httptest.NewRequest(http.MethodPost, Path, nil)
	r.Header.Set("Authorization", "Bearer t")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code, "no Origin: a server-to-server caller")

	r = httptest.NewRequest(http.MethodPost, "/api/v0.9/mcp-proxies", nil)
	r.Header.Set("Origin", "https://evil.example.com")
	w = httptest.NewRecorder()
	s.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })).ServeHTTP(w, r)
	assert.Equal(t, http.StatusTeapot, w.Code, "other paths are left alone")
}
