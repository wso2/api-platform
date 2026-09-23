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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ai-workspace-bff/internal/config"
	"ai-workspace-bff/internal/paths"
	"ai-workspace-bff/internal/proxy"
)

// ---------------------------------------------------------------------------
// extractSecretHandle unit tests
// ---------------------------------------------------------------------------

func TestExtractSecretHandle(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "standard placeholder",
			body: `{"upstream":{"main":{"auth":{"value":"{{ secret \"my-provider-api-key\" }}"}}}}`,
			want: "my-provider-api-key",
		},
		{
			name: "placeholder with extra spaces",
			body: `{"value":"{{  secret  \"handle-with-spaces\"  }}"}`,
			want: "handle-with-spaces",
		},
		{
			name: "no placeholder",
			body: `{"upstream":{"main":{"auth":{"value":"already-a-token"}}}}`,
			want: "",
		},
		{
			name: "empty body",
			body: `{}`,
			want: "",
		},
		{
			name: "multiple placeholders returns first",
			body: `{"a":"{{ secret \"first\" }}","b":"{{ secret \"second\" }}"}`,
			want: "first",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handles := extractSecretHandles([]byte(tc.body))
			got := ""
			if len(handles) > 0 {
				got = handles[0]
			}
			if got != tc.want {
				t.Errorf("extractSecretHandles() first = %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// handleCreateWithSecretCompensation integration tests
//
// These tests spin up a fake Platform API httptest.Server to record calls
// made by the BFF handler and verify compensation behaviour.
// ---------------------------------------------------------------------------

// buildTestServer returns a minimal *Server wired against the given platform
// API URL, with a session cookie whose value equals the supplied jwt.
func buildTestServer(t *testing.T, platformURL, jwt string) (*Server, *httptest.Server) {
	t.Helper()

	// MaxResponseBytes: -1 matches the shipped default (see config.defaultConfig) —
	// the zero value would instead apply httpclient's own 10MiB cap, which these
	// tests don't need.
	transport, err := proxy.NewTransport(
		config.HTTPClientConfig{Timeouts: config.HTTPClientTimeoutsConfig{MaxResponseBytes: -1}},
		proxy.TLSClientOptions{SkipVerify: true},
	)
	if err != nil {
		t.Fatalf("NewTransport: %v", err)
	}

	cfg := &config.Config{
		ControlPlane: config.ControlPlaneConfig{URL: platformURL},
		Cookie:       config.CookieConfig{Name: "_ai_workspace_session"},
	}

	s := &Server{
		cfg:          cfg,
		proxy:        proxy.ReverseProxy(mustParseURL(platformURL), paths.Proxy, transport),
		refreshLocks: make(map[string]*refreshLock),
	}

	// Wrap the handler so we can inject the session cookie on every request.
	bffSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.AddCookie(&http.Cookie{Name: cfg.Cookie.Name, Value: jwt})
		s.handleCreateWithSecretCompensation(w, r, "/llm-providers", paths.PlatformAPI)
	}))
	t.Cleanup(bffSrv.Close)

	return s, bffSrv
}

func mustParseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}

// fakeplatformAPI builds an httptest.Server that records received requests and
// responds with the supplied statusCode + body.
type recordedRequest struct {
	method string
	path   string
	body   string
	auth   string
}

func fakeControlPlane(t *testing.T, responses map[string]struct {
	status int
	body   string
}) (*httptest.Server, *[]recordedRequest) {
	t.Helper()
	var recorded []recordedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		recorded = append(recorded, recordedRequest{
			method: r.Method,
			path:   r.URL.Path + "?" + r.URL.RawQuery,
			body:   string(b),
			auth:   r.Header.Get("Authorization"),
		})
		key := r.Method + " " + r.URL.Path
		if resp, ok := responses[key]; ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(resp.status)
			_, _ = fmt.Fprint(w, resp.body)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return srv, &recorded
}

func TestHandleCreateWithSecretCompensation_Success(t *testing.T) {
	platform, calls := fakeControlPlane(t, map[string]struct {
		status int
		body   string
	}{
		"POST /api/v0.9/llm-providers": {http.StatusCreated, `{"id":"prov-1"}`},
	})

	_, bff := buildTestServer(t, platform.URL, "test-jwt")

	body := `{"id":"prov-1","upstream":{"main":{"auth":{"value":"{{ secret \"prov-1-api-key\" }}"}}}}`
	resp, err := http.Post(bff.URL, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want 201", resp.StatusCode)
	}

	// Only one call to the platform API — no compensation DELETE.
	if len(*calls) != 1 {
		t.Errorf("platform calls = %d, want 1", len(*calls))
	}
	if (*calls)[0].method != "POST" {
		t.Errorf("first call method = %q, want POST", (*calls)[0].method)
	}
}

func TestHandleCreateWithSecretCompensation_ProviderFailTriggersDelete(t *testing.T) {
	var deleteCalled atomic.Bool

	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "POST /api/v0.9/llm-providers":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprint(w, `{"error":"upstream failure"}`)
		case "DELETE /api/v0.9/secrets/prov-1-api-key":
			deleteCalled.Store(true)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer platform.Close()

	_, bff := buildTestServer(t, platform.URL, "test-jwt")

	body := `{"id":"prov-1","upstream":{"main":{"auth":{"value":"{{ secret \"prov-1-api-key\" }}"}}}}`
	resp, err := http.Post(bff.URL, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// BFF should relay the platform error status.
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}

	// Compensation DELETE must be fired asynchronously — give it a moment.
	deadline := time.Now().Add(2 * time.Second)
	for !deleteCalled.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !deleteCalled.Load() {
		t.Error("compensation DELETE was not called after provider creation failure")
	}
}

func TestHandleCreateWithSecretCompensation_NoSecretNoDelete(t *testing.T) {
	platform, calls := fakeControlPlane(t, map[string]struct {
		status int
		body   string
	}{
		"POST /api/v0.9/llm-providers": {http.StatusBadRequest, `{"error":"bad request"}`},
	})

	_, bff := buildTestServer(t, platform.URL, "test-jwt")

	// Body has no {{ secret "..." }} placeholder — no compensation should fire.
	body := `{"id":"prov-1","upstream":{"main":{"auth":{"type":"none"}}}}`
	resp, err := http.Post(bff.URL, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}

	// Allow any async goroutine to fire (it shouldn't).
	time.Sleep(100 * time.Millisecond)

	if len(*calls) != 1 {
		t.Errorf("platform calls = %d, want 1 (no compensation DELETE)", len(*calls))
	}
}

func TestHandleCreateWithSecretCompensation_Unauthenticated(t *testing.T) {
	platform, _ := fakeControlPlane(t, nil)
	cfg := &config.Config{
		ControlPlane: config.ControlPlaneConfig{URL: platform.URL},
		Cookie:       config.CookieConfig{Name: "_ai_workspace_session"},
	}
	// MaxResponseBytes: -1 matches the shipped default (see config.defaultConfig) —
	// the zero value would instead apply httpclient's own 10MiB cap, which these
	// tests don't need.
	transport, err := proxy.NewTransport(
		config.HTTPClientConfig{Timeouts: config.HTTPClientTimeoutsConfig{MaxResponseBytes: -1}},
		proxy.TLSClientOptions{SkipVerify: true},
	)
	if err != nil {
		t.Fatalf("NewTransport: %v", err)
	}
	s := &Server{
		cfg:          cfg,
		proxy:        proxy.ReverseProxy(mustParseURL(platform.URL), paths.Proxy, transport),
		refreshLocks: make(map[string]*refreshLock),
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/ai-workspace/api/llm-providers", strings.NewReader(`{}`))
	// No cookie set → should get 401.
	s.handleCreateWithSecretCompensation(w, r, "/llm-providers", paths.PlatformAPI)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
	var body map[string]string
	_ = json.NewDecoder(w.Body).Decode(&body)
	if body["code"] != "UNAUTHORIZED" {
		t.Errorf("code = %q, want %q", body["code"], "UNAUTHORIZED")
	}
	if body["message"] != "Invalid or expired credentials." {
		t.Errorf("message = %q, want %q", body["message"], "Invalid or expired credentials.")
	}
}

// ---------------------------------------------------------------------------
// handlePublishMCPProxy integration tests
// ---------------------------------------------------------------------------

// buildPublishTestServer returns a BFF test server whose only route is the
// composite publish endpoint, with a session cookie injected on every request
// when jwt is non-empty.
func buildPublishTestServer(t *testing.T, platformURL, jwt string) *httptest.Server {
	t.Helper()

	transport, err := proxy.NewTransport(
		config.HTTPClientConfig{Timeouts: config.HTTPClientTimeoutsConfig{MaxResponseBytes: -1}},
		proxy.TLSClientOptions{SkipVerify: true},
	)
	if err != nil {
		t.Fatalf("NewTransport: %v", err)
	}

	cfg := &config.Config{
		ControlPlane: config.ControlPlaneConfig{URL: platformURL},
		Cookie:       config.CookieConfig{Name: "_ai_workspace_session"},
	}
	s := &Server{
		cfg:          cfg,
		proxy:        proxy.ReverseProxy(mustParseURL(platformURL), paths.Proxy, transport),
		refreshLocks: make(map[string]*refreshLock),
	}

	// Registered on a real mux so r.PathValue resolves the same wildcards the
	// production route declares.
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/api-portals/{apiPortalId}/mcp-proxies/{mcpProxyId}/publish", s.handlePublishMCPProxy)

	bffSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if jwt != "" {
			r.AddCookie(&http.Cookie{Name: cfg.Cookie.Name, Value: jwt})
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(bffSrv.Close)
	return bffSrv
}

const publishTestPath = "/api/api-portals/acme-portal/mcp-proxies/my-proxy/publish"

func TestHandlePublishMCPProxy_SavesDraftThenPublishes(t *testing.T) {
	platform, calls := fakeControlPlane(t, map[string]struct {
		status int
		body   string
	}{
		"PUT /api/v0.9/api-portals/acme-portal/apis/mcp-proxy/my-proxy/draft":    {http.StatusOK, `{"displayName":"My Proxy"}`},
		"POST /api/v0.9/api-portals/acme-portal/apis/mcp-proxy/my-proxy/publish": {http.StatusCreated, `{"status":"PUBLISHED"}`},
	})

	bff := buildPublishTestServer(t, platform.URL, "test-jwt")

	draft := `{"displayName":"My Proxy","version":"1.0.0"}`
	resp, err := http.Post(bff.URL+publishTestPath, "application/json", strings.NewReader(draft))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// The publish response, not the draft's, is what the browser gets back.
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want 201", resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(got)) != `{"status":"PUBLISHED"}` {
		t.Errorf("body = %q, want the publish response", got)
	}

	if len(*calls) != 2 {
		t.Fatalf("platform calls = %d, want 2", len(*calls))
	}
	if (*calls)[0].method != http.MethodPut || (*calls)[0].body != draft {
		t.Errorf("first call = %s %q, want PUT with the draft body verbatim", (*calls)[0].method, (*calls)[0].body)
	}
	if (*calls)[1].method != http.MethodPost || (*calls)[1].body != "" {
		t.Errorf("second call = %s %q, want POST with no body", (*calls)[1].method, (*calls)[1].body)
	}
	if (*calls)[1].auth != "Bearer test-jwt" {
		t.Errorf("publish auth = %q, want the session's bearer token", (*calls)[1].auth)
	}
}

func TestHandlePublishMCPProxy_DraftFailureSkipsPublish(t *testing.T) {
	platform, calls := fakeControlPlane(t, map[string]struct {
		status int
		body   string
	}{
		"PUT /api/v0.9/api-portals/acme-portal/apis/mcp-proxy/my-proxy/draft": {http.StatusBadRequest, `{"error":"bad draft"}`},
	})

	bff := buildPublishTestServer(t, platform.URL, "test-jwt")

	resp, err := http.Post(bff.URL+publishTestPath, "application/json", strings.NewReader(`{"displayName":"My Proxy"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want the relayed 400", resp.StatusCode)
	}
	if len(*calls) != 1 {
		t.Errorf("platform calls = %d, want 1 (publish must not run after a failed draft)", len(*calls))
	}
}

func TestHandlePublishMCPProxy_EmptyBodyRejected(t *testing.T) {
	platform, calls := fakeControlPlane(t, nil)
	bff := buildPublishTestServer(t, platform.URL, "test-jwt")

	resp, err := http.Post(bff.URL+publishTestPath, "application/json", strings.NewReader("  "))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
	if len(*calls) != 0 {
		t.Errorf("platform calls = %d, want 0", len(*calls))
	}
}

func TestHandlePublishMCPProxy_Unauthenticated(t *testing.T) {
	platform, calls := fakeControlPlane(t, nil)
	bff := buildPublishTestServer(t, platform.URL, "") // no session cookie

	resp, err := http.Post(bff.URL+publishTestPath, "application/json", strings.NewReader(`{"displayName":"My Proxy"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
	if len(*calls) != 0 {
		t.Errorf("platform calls = %d, want 0", len(*calls))
	}
}
