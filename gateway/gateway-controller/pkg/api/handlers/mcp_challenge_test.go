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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/authenticators"
	commonmodels "github.com/wso2/api-platform/common/models"
	"github.com/wso2/api-platform/httpkit/httputil"
)

const challengeURL = "https://gw.example.com/.well-known/oauth-protected-resource/api/management/v1/mcp"

var challengeScopes = []string{"gw:read", "gw:write"}

// challengeServe runs one request through the middleware around next.
func challengeServe(t *testing.T, mw func(http.Handler) http.Handler, next http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/management/v1/mcp", nil))
	return rec
}

func writeStatus(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		httputil.WriteError(w, status, "code", "message")
	}
}

func TestChallengeOnUnauthorized(t *testing.T) {
	rec := challengeServe(t, MCPChallengeMiddleware(challengeURL, challengeScopes), writeStatus(http.StatusUnauthorized))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t,
		`Bearer resource_metadata="`+challengeURL+`", scope="gw:read gw:write"`,
		rec.Header().Get("WWW-Authenticate"))
	assert.Contains(t, rec.Body.String(), `"code":"code"`, "the body the inner handler wrote is untouched")
}

// A 403 carries the RFC 6750 insufficient_scope error so a client knows to
// step up rather than re-authenticate from scratch.
func TestChallengeOnForbidden(t *testing.T) {
	rec := challengeServe(t, MCPChallengeMiddleware(challengeURL, challengeScopes), writeStatus(http.StatusForbidden))

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t,
		`Bearer error="insufficient_scope", resource_metadata="`+challengeURL+`", scope="gw:read gw:write"`,
		rec.Header().Get("WWW-Authenticate"))
}

// Each parameter is included only when there is a value for it. With nothing
// to say, a 401 gets no challenge at all rather than a bare "Bearer ".
func TestChallengeOmitsParametersWithoutAValue(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		scopes  []string
		want401 string
		want403 string
	}{
		{"nothing configured", "", nil,
			"", `Bearer error="insufficient_scope"`},
		{"metadata only", challengeURL, nil,
			`Bearer resource_metadata="` + challengeURL + `"`,
			`Bearer error="insufficient_scope", resource_metadata="` + challengeURL + `"`},
		{"scopes only", "", []string{"gw:admin"},
			`Bearer scope="gw:admin"`,
			`Bearer error="insufficient_scope", scope="gw:admin"`},
		{"empty scope list is omitted", challengeURL, []string{},
			`Bearer resource_metadata="` + challengeURL + `"`,
			`Bearer error="insufficient_scope", resource_metadata="` + challengeURL + `"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := MCPChallengeMiddleware(tt.url, tt.scopes)

			rec := challengeServe(t, mw, writeStatus(http.StatusUnauthorized))
			assert.Equal(t, tt.want401, rec.Header().Get("WWW-Authenticate"))
			_, present := rec.Header()["Www-Authenticate"]
			assert.Equal(t, tt.want401 != "", present, "no header at all when the challenge would be empty")

			rec = challengeServe(t, mw, writeStatus(http.StatusForbidden))
			assert.Equal(t, tt.want403, rec.Header().Get("WWW-Authenticate"))
		})
	}
}

// Only 401 and 403 are challenges. A success, a client error or a server error
// is passed through without one.
func TestChallengeIsOnlyAddedToUnauthorizedAndForbidden(t *testing.T) {
	mw := MCPChallengeMiddleware(challengeURL, challengeScopes)
	for _, status := range []int{
		http.StatusOK, http.StatusAccepted, http.StatusNoContent,
		http.StatusBadRequest, http.StatusNotFound, http.StatusMethodNotAllowed,
		http.StatusRequestEntityTooLarge, http.StatusInternalServerError,
	} {
		rec := challengeServe(t, mw, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) })

		assert.Equalf(t, status, rec.Code, "status %d", status)
		assert.Emptyf(t, rec.Header().Get("WWW-Authenticate"), "status %d must carry no challenge", status)
	}
}

// A challenge set by the inner handler is never overwritten.
func TestChallengeNeverOverwritesAnExistingHeader(t *testing.T) {
	mw := MCPChallengeMiddleware(challengeURL, challengeScopes)
	for _, tc := range []struct {
		status int
		header string
	}{
		{http.StatusForbidden, `Bearer error="insufficient_scope", scope="gw:deploy"`},
		{http.StatusUnauthorized, `Basic realm="gateway"`},
	} {
		rec := challengeServe(t, mw, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("WWW-Authenticate", tc.header)
			w.WriteHeader(tc.status)
		})

		assert.Equal(t, tc.header, rec.Header().Get("WWW-Authenticate"))
		assert.Len(t, rec.Header().Values("WWW-Authenticate"), 1, "not appended as a second challenge either")
	}
}

// ScopeGate's own 403 challenge reaches the client unchanged.
func TestChallengeLeavesScopeGatesStepUpChallengeAlone(t *testing.T) {
	gate := newGateTestAuthz(widgetResolver)
	reader := &commonmodels.AuthContext{UserID: "u1", Roles: []string{"reader"}}
	inner := gate.ScopeGate(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("a denied call must not reach the handler")
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"create_widget"}}`))
	req = req.WithContext(authenticators.WithAuthContext(req.Context(), *reader))
	rec := httptest.NewRecorder()
	MCPChallengeMiddleware(challengeURL, challengeScopes)(inner).ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	got := rec.Header().Get("WWW-Authenticate")
	assert.Contains(t, got, `error_description=`, "the gate's own challenge, not the middleware's")
	assert.Contains(t, got, `scope="gw:admin"`, "the scope this call needs, translated to the IdP's name")
	assert.NotContains(t, got, "gw:read gw:write", "the generic advertised scopes must not replace it")
}

// The other production pairing: the real authentication middleware writes a
// bare 401 inside this one, and the client receives the discovery challenge.
func TestChallengeDecoratesTheRealAuthenticationFailure(t *testing.T) {
	authMW, err := authenticators.AuthMiddleware(commonmodels.AuthConfig{
		BasicAuth: &commonmodels.BasicAuth{
			Enabled: true,
			Users:   []commonmodels.User{{Username: "admin", Password: "s3cret", Roles: []string{"admin"}}},
		},
	}, toolsDiscard)
	require.NoError(t, err)
	reached := false
	handler := MCPChallengeMiddleware(challengeURL, challengeScopes)(authMW(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) { reached = true })))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/management/v1/mcp", nil))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, reached)
	assert.Equal(t, `Bearer resource_metadata="`+challengeURL+`", scope="gw:read gw:write"`,
		rec.Header().Get("WWW-Authenticate"))

	req := httptest.NewRequest(http.MethodPost, "/api/management/v1/mcp", nil)
	req.SetBasicAuth("admin", "s3cret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.True(t, reached, "valid credentials pass through")
	assert.Empty(t, rec.Header().Get("WWW-Authenticate"), "an authenticated request gets no challenge")
}

// A write without a status is an implicit 200, and a later 401 adds no challenge.
func TestChallengeWriteWithoutAStatusIsAnImplicitOK(t *testing.T) {
	rec := challengeServe(t, MCPChallengeMiddleware(challengeURL, challengeScopes),
		func(w http.ResponseWriter, _ *http.Request) {
			_, err := w.Write([]byte("hello"))
			require.NoError(t, err)
			_, _ = w.Write([]byte(" world"))
			w.WriteHeader(http.StatusUnauthorized)
		})

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "hello world", rec.Body.String())
	// rec.Header() is the live map, so this also catches a challenge added
	// after the headers were committed.
	assert.Empty(t, rec.Header().Get("WWW-Authenticate"))
}

// Only the first status decides. Once a 200 has been committed, a later
// WriteHeader(401) must not slip a challenge into headers already sent.
func TestChallengeOnlyTheFirstStatusDecides(t *testing.T) {
	rec := challengeServe(t, MCPChallengeMiddleware(challengeURL, challengeScopes),
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.WriteHeader(http.StatusUnauthorized)
		})

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("WWW-Authenticate"))
}

// Each request gets a fresh writer, so one request's committed status cannot
// suppress the next request's challenge.
func TestChallengeStateIsPerRequest(t *testing.T) {
	status := http.StatusOK
	handler := MCPChallengeMiddleware(challengeURL, challengeScopes)(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) }))

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/", nil))
	status = http.StatusUnauthorized
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/", nil))

	assert.Empty(t, first.Header().Get("WWW-Authenticate"))
	assert.NotEmpty(t, second.Header().Get("WWW-Authenticate"))
}

// The MCP SDK flushes its SSE stream through http.ResponseController, exactly
// as here. The flush must reach the real writer, and commit a 200 first.
func TestChallengeWriterFlushesThroughTheResponseController(t *testing.T) {
	rec := challengeServe(t, MCPChallengeMiddleware(challengeURL, challengeScopes),
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			require.NoError(t, http.NewResponseController(w).Flush())
			_, _ = w.Write([]byte("data: {}\n\n"))
		})

	assert.True(t, rec.Flushed, "the flush reached the underlying writer")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "data: {}\n\n", rec.Body.String())
	assert.Empty(t, rec.Header().Get("WWW-Authenticate"))
}

// noFlushWriter is a ResponseWriter that cannot flush.
type noFlushWriter struct {
	header http.Header
	status int
}

func (w *noFlushWriter) Header() http.Header         { return w.header }
func (w *noFlushWriter) Write(b []byte) (int, error) { return len(b), nil }
func (w *noFlushWriter) WriteHeader(status int)      { w.status = status }

// Flushing over a writer that cannot flush is a no-op: it neither panics nor
// commits a status, so a later 401 is still decorated.
func TestChallengeWriterFlushOverANonFlusherIsANoOp(t *testing.T) {
	under := &noFlushWriter{header: http.Header{}}
	MCPChallengeMiddleware(challengeURL, challengeScopes)(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.(http.Flusher).Flush()
			assert.Zero(t, under.status, "a no-op flush commits no status")
			w.WriteHeader(http.StatusUnauthorized)
		})).ServeHTTP(under, httptest.NewRequest(http.MethodPost, "/", nil))

	assert.Equal(t, http.StatusUnauthorized, under.status)
	assert.NotEmpty(t, under.header.Get("WWW-Authenticate"))
}

// Unwrap exposes the underlying writer, so ResponseController operations the
// wrapper does not implement itself still reach it.
func TestChallengeWriterUnwrapsToTheUnderlyingWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	var seen http.ResponseWriter
	MCPChallengeMiddleware(challengeURL, challengeScopes)(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			u, ok := w.(interface{ Unwrap() http.ResponseWriter })
			require.True(t, ok)
			seen = u.Unwrap()
		})).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))

	assert.Same(t, rec, seen)
}
