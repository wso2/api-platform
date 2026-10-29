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

package server

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gohttpkit "github.com/wso2/api-platform/httpkit/middleware"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/mcpserver"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

const (
	idpTestIssuer = "https://idp.test"
	idpTestOrgID  = "11111111-1111-1111-1111-111111111111"
	mcpTestAud    = "mcp"
)

// countingUserRows counts the user rows the identity service writes.
type countingUserRows struct{ created int }

func (c *countingUserRows) GetOrCreateUUID(string) (string, error)             { c.created++; return "u1", nil }
func (c *countingUserRows) GetUUIDByIdentity(string) (string, error)           { return "", nil }
func (c *countingUserRows) GetSubByUUID(string) (string, bool, error)          { return "", false, nil }
func (c *countingUserRows) GetSubsByUUIDs([]string) (map[string]string, error) { return nil, nil }

// idpTestKey serves the public half of a fresh RSA key as a JWKS and signs
// tokens with the private half.
type idpTestKey struct {
	key  *rsa.PrivateKey
	jwks *httptest.Server
}

func newIDPTestKey(t *testing.T) *idpTestKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	encode := base64.RawURLEncoding.EncodeToString
	body, err := json.Marshal(map[string]any{"keys": []map[string]any{{
		"kty": "RSA", "kid": "k1", "use": "sig", "alg": "RS256",
		"n": encode(key.N.Bytes()), "e": encode(big.NewInt(int64(key.E)).Bytes()),
	}}})
	require.NoError(t, err)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return &idpTestKey{key: key, jwks: srv}
}

func (k *idpTestKey) token(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "k1"
	signed, err := token.SignedString(k.key)
	require.NoError(t, err)
	return signed
}

// idpClaims are a valid token's claims; aud is left out when nil.
func idpClaims(aud any, scope string) jwt.MapClaims {
	claims := jwt.MapClaims{"iss": idpTestIssuer, "sub": "alice", "organization": "idp-org",
		"scope": scope, "exp": time.Now().Add(time.Hour).Unix()}
	if aud != nil {
		claims["aud"] = aud
	}
	return claims
}

// newIDPChain builds the Platform API's own chain in idp mode with MCP on, in
// the order server.go uses, over a mux holding the MCP route and two others.
func newIDPChain(t *testing.T, key *idpTestKey, idpAudience []string, readOnly config.ReadOnly) (http.Handler, *countingUserRows) {
	t.Helper()
	const specPath = "../../resources/openapi.yaml"
	scopes, err := middleware.LoadScopeRegistry(specPath)
	require.NoError(t, err)

	cfg := &config.Server{OpenAPISpecPath: specPath}
	cfg.Auth.Mode = config.AuthModeIDP
	cfg.Auth.IDP.Issuer = []string{idpTestIssuer}
	cfg.Auth.IDP.JWKSUrl = key.jwks.URL
	cfg.Auth.IDP.Audience = idpAudience
	cfg.Auth.ClaimMappings.Organization = "organization"
	cfg.Auth.ClaimMappings.Scope = "scope"
	cfg.Auth.Authorization.Enabled = true
	cfg.Auth.Authorization.Mode = middleware.ValidationModeScope
	cfg.Auth.SkipPaths = []string{"/health"}
	cfg.ReadOnly = readOnly
	cfg.MCPServer = config.MCPServer{Enabled: true, MaxRequestBytes: 1 << 20,
		PublicURL: "https://platform-api.example.com",
		Auth:      config.MCPServerAuth{Audience: mcpTestAud}}

	rows := &countingUserRows{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	ok := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
	mux.HandleFunc("GET /api/v0.9/projects", ok)
	mux.HandleFunc("GET /api/v0.9/mcp-proxies", ok)
	skip, wrap, err := startMCPServer(logger, mux, mcpserver.Deps{Config: cfg, Scopes: scopes,
		Identity: service.NewIdentityService(rows), Logger: logger})
	require.NoError(t, err)
	cfg.Auth.SkipPaths = append(cfg.Auth.SkipPaths, skip...)

	utils.InitSharedHTTPClient(&http.Client{Timeout: 5 * time.Second}, 0)
	authenticator, err := buildAuthenticator(cfg, logger, nil)
	require.NoError(t, err)

	chain := []func(http.Handler) http.Handler{
		gohttpkit.CORSMiddleware(gohttpkit.CORSOptions{AllowedMethods: []string{"GET", "POST", "OPTIONS"},
			AllowedHeaders: []string{"Origin", "Content-Length", "Content-Type", "Authorization"}}),
		wrap,
	}
	chain = append(chain, authenticator.Middleware()...)
	chain = append(chain, middleware.OrganizationResolverMiddleware(func(claim string) (string, bool) {
		if claim == "idp-org" {
			return idpTestOrgID, true
		}
		return "", false
	}))
	enforcer, err := middleware.ScopeEnforcer(scopes, middleware.ScopeEnforcerConfig{
		ValidationMode: cfg.Auth.Authorization.Mode,
		Enabled:        true,
		Routes:         mux,
		SkipPaths:      cfg.Auth.SkipPaths,
		ExemptRoutes:   []string{"POST " + mcpserver.Path},
	})
	require.NoError(t, err)
	chain = append(chain, enforcer)
	// The guard sits in the chain so a read-only organization is refused before any /mcp call.
	readOnlyGuard, err := middleware.ReadOnlyGuard(middleware.ReadOnlyGuardConfig{
		ReadOnly: &cfg.ReadOnly, Routes: mux, SkipPaths: cfg.Auth.SkipPaths, Logger: logger,
	})
	require.NoError(t, err)
	chain = append(chain, readOnlyGuard)
	return gohttpkit.Chain(chain...)(mux), rows
}

func callToolsList(h http.Handler, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, mcpserver.Path,
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func toolNames(t *testing.T, w *httptest.ResponseRecorder) []string {
	t.Helper()
	var body struct {
		Result struct {
			Tools []struct{ Name string } `json:"tools"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
	var names []string
	for _, tool := range body.Result.Tools {
		names = append(names, tool.Name)
	}
	return names
}

// The MCP audience is checked on top of the sign-in's own audience check, so
// each case runs with auth.idp.audience empty and set.
func TestMCPServer_IDPModeChecksTheMCPAudience(t *testing.T) {
	key := newIDPTestKey(t)
	const scope = "ap:project:read ap:gateway:read"

	for _, idpAudience := range [][]string{nil, {"rest", mcpTestAud}} {
		t.Run(strings.Join(idpAudience, ","), func(t *testing.T) {
			h, rows := newIDPChain(t, key, idpAudience, config.ReadOnly{})

			for name, aud := range map[string]any{
				"list":    []string{mcpTestAud},
				"string":  mcpTestAud,
				"several": []string{"rest", mcpTestAud},
			} {
				w := callToolsList(h, key.token(t, idpClaims(aud, scope)))
				assert.Equal(t, http.StatusOK, w.Code, name+": "+w.Body.String())
				assert.Contains(t, toolNames(t, w), "list_projects", name)
			}
			rows.created = 0
			noOrg := idpClaims([]string{mcpTestAud}, scope)
			delete(noOrg, "organization")
			for name, token := range map[string]string{
				"wrong audience": key.token(t, idpClaims([]string{"rest"}, scope)),
				"no audience":    key.token(t, idpClaims(nil, scope)),
				"no org claim":   key.token(t, noOrg),
			} {
				assert.Equal(t, http.StatusUnauthorized, callToolsList(h, token).Code, name)
			}
			w := callToolsList(h, "")
			assert.Equal(t, http.StatusUnauthorized, w.Code, "no token")
			assert.Contains(t, w.Header().Get("WWW-Authenticate"), "resource_metadata")
			assert.Zero(t, rows.created, "a refused token writes no user row")
		})
	}
}

// With MCP on, the exemption covers the MCP route only.
func TestMCPServer_OtherRoutesKeepTheirScopeCheck(t *testing.T) {
	key := newIDPTestKey(t)
	h, _ := newIDPChain(t, key, nil, config.ReadOnly{})

	status := func(path, scope string) int {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Header.Set("Authorization", "Bearer "+key.token(t, idpClaims(mcpTestAud, scope)))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	assert.Equal(t, http.StatusForbidden, status("/api/v0.9/mcp-proxies", "ap:project:read"))
	assert.Equal(t, http.StatusOK, status("/api/v0.9/mcp-proxies", "ap:mcp_proxy:read"))
	assert.Equal(t, http.StatusForbidden, status("/api/v0.9/projects", "ap:gateway:read"))
	assert.Equal(t, http.StatusOK, status("/api/v0.9/projects", "ap:project:read"))
}

// In read-only mode a frozen organization gets 503 on every /mcp call; a writable one is untouched.
func TestMCPServer_ReadOnlyOrgIsRefused(t *testing.T) {
	key := newIDPTestKey(t)
	token := key.token(t, idpClaims(mcpTestAud, "ap:project:read"))

	frozen, _ := newIDPChain(t, key, nil, config.ReadOnly{Enabled: true})
	w := callToolsList(frozen, token)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "ORGANIZATION_READ_ONLY")

	writable, _ := newIDPChain(t, key, nil, config.ReadOnly{Enabled: true, WritableOrganizations: []string{idpTestOrgID}})
	w = callToolsList(writable, token)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.NotEmpty(t, toolNames(t, w))
}
