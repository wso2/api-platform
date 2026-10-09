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
	"compress/gzip"
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"os"
	"path/filepath"
	"testing"
	"time"

	"api-control-plane-bff/internal/config"
)

// newGraphQLInvokeTestPlatform builds a fake Platform API serving file-based
// login plus the three lookups handleGraphQLInvoke depends on: the GraphQL
// API's context/version, the gateway's endpoints, and its deployment list.
// gatewayEndpointURL is normally the mock gateway server's own URL.
func newGraphQLInvokeTestPlatform(t *testing.T, tok, gatewayEndpointURL string, deployed bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/auth/login"):
			json.NewEncoder(w).Encode(map[string]any{"token": tok, "expires_at": time.Now().Add(time.Hour).Unix()})
		case r.URL.Path == "/api/v0.9/graphql-apis/countries-graphql-api":
			json.NewEncoder(w).Encode(map[string]any{"context": "/countries-graphql-api", "version": "v1"})
		case r.URL.Path == "/api/v0.9/gateways/edge-gateway":
			json.NewEncoder(w).Encode(map[string]any{"endpoints": []string{gatewayEndpointURL}})
		case r.URL.Path == "/api/v0.9/graphql-apis/countries-graphql-api/deployments":
			status := "UNDEPLOYED"
			if deployed {
				status = "DEPLOYED"
			}
			json.NewEncoder(w).Encode(map[string]any{
				"list": []map[string]any{{"gatewayId": "edge-gateway", "status": status}},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{"status": "error", "code": "NOT_FOUND", "message": "not found"})
		}
	}))
}

func loginTestClient(t *testing.T, bffURL string) *http.Client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	req, _ := http.NewRequest(http.MethodPost, bffURL+"/api/login", strings.NewReader(`{"username":"admin","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(config.CSRFHeaderName, "api-control-plane")
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	assertStatus(t, res, http.StatusOK)
	return client
}

func TestGraphQLInvoke_UnauthenticatedRejected(t *testing.T) {
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer platform.Close()

	cfg := newTestConfig(platform.URL)
	srv, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()
	bff := httptest.NewServer(srv.Handler())
	defer bff.Close()

	invokeReq, _ := http.NewRequest(http.MethodPost,
		bff.URL+"/api/graphql-console/countries-graphql-api/gateways/edge-gateway/invoke",
		strings.NewReader(`{"query":"{ __typename }"}`))
	invokeReq.Header.Set("Content-Type", "application/json")
	invokeReq.Header.Set(config.CSRFHeaderName, "api-control-plane")
	res, err := http.DefaultClient.Do(invokeReq)
	if err != nil {
		t.Fatalf("invoke request: %v", err)
	}
	assertStatus(t, res, http.StatusUnauthorized)
}

// The whole point of this handler: the caller's own test credential for the
// TARGET API reaches the gateway unmodified (never the BFF session's own
// platform-api token), while the browser-facing session cookie never leaks
// upstream — the same session-hygiene bar proxyHandler is held to.
func TestGraphQLInvoke_ForwardsCallerAuthAndResolvesRealGatewayURL(t *testing.T) {
	tok := makeJWT(map[string]any{"username": "admin"})

	var gotAuth, gotCookie, gotPath, gotBody, gotCSRF string
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotCookie = r.Header.Get("Cookie")
		gotPath = r.URL.Path
		gotCSRF = r.Header.Get(config.CSRFHeaderName)
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"__typename": "Query"}})
	}))
	defer gateway.Close()

	platform := newGraphQLInvokeTestPlatform(t, tok, gateway.URL, true)
	defer platform.Close()

	cfg := newTestConfig(platform.URL)
	srv, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()
	bff := httptest.NewServer(srv.Handler())
	defer bff.Close()

	client := loginTestClient(t, bff.URL)

	invokeReq, _ := http.NewRequest(http.MethodPost,
		bff.URL+"/api/graphql-console/countries-graphql-api/gateways/edge-gateway/invoke",
		strings.NewReader(`{"query":"{ __typename }"}`))
	invokeReq.Header.Set("Content-Type", "application/json")
	invokeReq.Header.Set(config.CSRFHeaderName, "api-control-plane")
	invokeReq.Header.Set("Authorization", "Bearer caller-own-test-jwt")
	res, err := client.Do(invokeReq)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	assertStatus(t, res, http.StatusOK)

	if gotAuth != "Bearer caller-own-test-jwt" {
		t.Errorf("gateway Authorization = %q, want the caller's own test credential unmodified", gotAuth)
	}
	if gotCookie != "" {
		t.Errorf("gateway Cookie = %q, want empty (BFF session cookie must never leak upstream)", gotCookie)
	}
	if gotCSRF != "" {
		t.Errorf("gateway %s = %q, want empty (BFF-internal header, not for the target API)", config.CSRFHeaderName, gotCSRF)
	}
	if gotPath != "/countries-graphql-api" {
		t.Errorf("gateway path = %q, want /countries-graphql-api (endpoint + resolved context)", gotPath)
	}
	if gotBody != `{"query":"{ __typename }"}` {
		t.Errorf("gateway body = %q, want the request body forwarded verbatim", gotBody)
	}

	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), `"__typename":"Query"`) {
		t.Errorf("response body = %q, want the gateway's response relayed back", body)
	}
}

// Regression test: the browser's own Accept-Encoding header (fetch() always
// sends one; JS cannot omit or override it) used to be forwarded verbatim to
// the gateway. That makes http.Transport treat compression as explicitly
// requested by the caller, so it stops auto-decompressing — a gzip-compressing
// upstream's response then reached the browser as raw compressed bytes with
// no Content-Encoding header telling it to decompress, rendering as garbage
// ("Unexpected token ... is not valid JSON"). Not forwarding Accept-Encoding
// lets http.Transport manage compression itself and hand back plain bytes.
func TestGraphQLInvoke_DecompressesGzippedUpstreamResponse(t *testing.T) {
	tok := makeJWT(map[string]any{"username": "admin"})

	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			t.Error("gateway saw no Accept-Encoding: gzip — http.Transport should still offer its own")
		}
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", "application/json")
		gz := gzip.NewWriter(w)
		gz.Write([]byte(`{"data":{"__typename":"Query"}}`))
		gz.Close()
	}))
	defer gateway.Close()

	platform := newGraphQLInvokeTestPlatform(t, tok, gateway.URL, true)
	defer platform.Close()

	cfg := newTestConfig(platform.URL)
	srv, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()
	bff := httptest.NewServer(srv.Handler())
	defer bff.Close()

	client := loginTestClient(t, bff.URL)
	invokeReq, _ := http.NewRequest(http.MethodPost,
		bff.URL+"/api/graphql-console/countries-graphql-api/gateways/edge-gateway/invoke",
		strings.NewReader(`{"query":"{ __typename }"}`))
	invokeReq.Header.Set("Content-Type", "application/json")
	invokeReq.Header.Set(config.CSRFHeaderName, "api-control-plane")
	// Mirrors what a real browser fetch() always sends and cannot be told not
	// to — this is the header this handler must NOT forward upstream.
	invokeReq.Header.Set("Accept-Encoding", "gzip, deflate, br")
	res, err := client.Do(invokeReq)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	assertStatus(t, res, http.StatusOK)

	if ce := res.Header.Get("Content-Encoding"); ce != "" {
		t.Errorf("response Content-Encoding = %q, want empty (body must already be plain bytes)", ce)
	}
	body, _ := io.ReadAll(res.Body)
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("response body is not valid JSON (%v): %q", err, body)
	}
}

func TestGraphQLInvoke_NotDeployedToGatewayRejected(t *testing.T) {
	tok := makeJWT(map[string]any{"username": "admin"})
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("gateway must not be called when the API isn't deployed there")
	}))
	defer gateway.Close()

	platform := newGraphQLInvokeTestPlatform(t, tok, gateway.URL, false /* not deployed */)
	defer platform.Close()

	cfg := newTestConfig(platform.URL)
	srv, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()
	bff := httptest.NewServer(srv.Handler())
	defer bff.Close()

	client := loginTestClient(t, bff.URL)
	invokeReq, _ := http.NewRequest(http.MethodPost,
		bff.URL+"/api/graphql-console/countries-graphql-api/gateways/edge-gateway/invoke",
		strings.NewReader(`{"query":"{ __typename }"}`))
	invokeReq.Header.Set("Content-Type", "application/json")
	invokeReq.Header.Set(config.CSRFHeaderName, "api-control-plane")
	res, err := client.Do(invokeReq)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	assertStatus(t, res, http.StatusUnprocessableEntity)

	var body map[string]any
	json.NewDecoder(res.Body).Decode(&body)
	if body["code"] != "GRAPHQL_API_NOT_DEPLOYED" {
		t.Errorf("error code = %v, want GRAPHQL_API_NOT_DEPLOYED", body["code"])
	}
}

// A graphqlApiId the caller's org can't see (or that doesn't exist) must come
// back as a plain 404 — never a client-suppliable URL, and never a distinction
// that would let a caller probe for another org's handles.
func TestGraphQLInvoke_UnknownAPIRejected(t *testing.T) {
	tok := makeJWT(map[string]any{"username": "admin"})
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/auth/login") {
			json.NewEncoder(w).Encode(map[string]any{"token": tok, "expires_at": time.Now().Add(time.Hour).Unix()})
			return
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{"status": "error", "code": "NOT_FOUND", "message": "not found"})
	}))
	defer platform.Close()

	cfg := newTestConfig(platform.URL)
	srv, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()
	bff := httptest.NewServer(srv.Handler())
	defer bff.Close()

	client := loginTestClient(t, bff.URL)
	invokeReq, _ := http.NewRequest(http.MethodPost,
		bff.URL+"/api/graphql-console/does-not-exist/gateways/edge-gateway/invoke",
		strings.NewReader(`{"query":"{ __typename }"}`))
	invokeReq.Header.Set("Content-Type", "application/json")
	invokeReq.Header.Set(config.CSRFHeaderName, "api-control-plane")
	res, err := client.Do(invokeReq)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	assertStatus(t, res, http.StatusNotFound)

	var body map[string]any
	json.NewDecoder(res.Body).Decode(&body)
	if body["code"] != "GRAPHQL_API_NOT_FOUND" {
		t.Errorf("error code = %v, want GRAPHQL_API_NOT_FOUND", body["code"])
	}
}

func TestBuildGraphQLInvokeURL(t *testing.T) {
	tests := []struct {
		endpoint, context, version, want string
	}{
		{"https://gw.example.com", "/countries-graphql-api", "v1", "https://gw.example.com/countries-graphql-api"},
		{"gw.example.com", "/graphql", "v1", "https://gw.example.com/graphql"},
		{"https://gw.example.com/", "graphql", "v1", "https://gw.example.com/graphql"},
		{"https://gw.example.com", "/api/$version/graphql", "v2", "https://gw.example.com/api/v2/graphql"},
		{"https://gw.example.com", "", "v1", "https://gw.example.com/"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s+%s", tt.endpoint, tt.context), func(t *testing.T) {
			got := buildGraphQLInvokeURL(tt.endpoint, tt.context, tt.version)
			if got != tt.want {
				t.Errorf("buildGraphQLInvokeURL(%q, %q, %q) = %q, want %q", tt.endpoint, tt.context, tt.version, got, tt.want)
			}
		})
	}
}

// invokeThroughBFF logs in and POSTs a trivial query through the Test Console
// route against a fake Platform API that registers gatewayEndpointURL for the
// gateway, returning the BFF's response.
func invokeThroughBFF(t *testing.T, cfgMutate func(*config.Config), gatewayEndpointURL string) *http.Response {
	t.Helper()
	tok := makeJWT(map[string]any{"username": "admin"})
	platform := newGraphQLInvokeTestPlatform(t, tok, gatewayEndpointURL, true)
	t.Cleanup(platform.Close)

	cfg := newTestConfig(platform.URL)
	if cfgMutate != nil {
		cfgMutate(cfg)
	}
	srv, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	bff := httptest.NewServer(srv.Handler())
	t.Cleanup(bff.Close)

	client := loginTestClient(t, bff.URL)
	req, _ := http.NewRequest(http.MethodPost,
		bff.URL+"/api/graphql-console/countries-graphql-api/gateways/edge-gateway/invoke",
		strings.NewReader(`{"query":"{ __typename }"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(config.CSRFHeaderName, "api-control-plane")
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	t.Cleanup(func() { res.Body.Close() })
	return res
}

func errorCode(t *testing.T, res *http.Response) string {
	t.Helper()
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	return body.Code
}

func newTLSGateway(t *testing.T) *httptest.Server {
	t.Helper()
	gw := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"__typename":"Query"}}`)
	}))
	t.Cleanup(gw.Close)
	return gw
}

// A self-signed gateway certificate is rejected by default — the invoke client
// must not inherit the control plane's TLS settings — and the failure is
// reported as a certificate-trust problem rather than a bare "unreachable".
func TestGraphQLInvoke_UntrustedGatewayCertificateReported(t *testing.T) {
	gw := newTLSGateway(t)
	res := invokeThroughBFF(t, func(cfg *config.Config) {
		cfg.ControlPlane.TLSSkipVerify = true // must NOT leak onto the gateway hop
	}, gw.URL)
	assertStatus(t, res, http.StatusBadGateway)
	if code := errorCode(t, res); code != "GATEWAY_TLS_UNTRUSTED" {
		t.Errorf("code = %q, want GATEWAY_TLS_UNTRUSTED", code)
	}
}

func TestGraphQLInvoke_GatewayCAFileTrusted(t *testing.T) {
	gw := newTLSGateway(t)
	caPath := filepath.Join(t.TempDir(), "gateway-ca.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: gw.Certificate().Raw})
	if err := os.WriteFile(caPath, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	res := invokeThroughBFF(t, func(cfg *config.Config) { cfg.GatewayInvoke.CAFile = caPath }, gw.URL)
	assertStatus(t, res, http.StatusOK)
}

func TestGraphQLInvoke_GatewaySkipVerify(t *testing.T) {
	gw := newTLSGateway(t)
	res := invokeThroughBFF(t, func(cfg *config.Config) { cfg.GatewayInvoke.TLSSkipVerify = true }, gw.URL)
	assertStatus(t, res, http.StatusOK)
}

// The gateway's redirect is relayed, never followed server-side.
func TestGraphQLInvoke_GatewayRedirectNotFollowed(t *testing.T) {
	var followed bool
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { followed = true }))
	t.Cleanup(target.Close)
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(gw.Close)

	res := invokeThroughBFF(t, nil, gw.URL)
	assertStatus(t, res, http.StatusTemporaryRedirect)
	if followed {
		t.Error("redirect target was dialed; the invoke client must not follow gateway redirects")
	}
}

// https://localhost:... is the classic misregistration: fine from the
// operator's browser, but from the console backend it is the backend itself.
func TestGraphQLInvoke_LoopbackEndpointExplained(t *testing.T) {
	// Grab a free loopback port, then close it so the dial is refused.
	l := httptest.NewServer(http.NotFoundHandler())
	port := l.Listener.Addr().(*net.TCPAddr).Port
	l.Close()

	res := invokeThroughBFF(t, nil, fmt.Sprintf("http://localhost:%d", port))
	assertStatus(t, res, http.StatusBadGateway)
	if code := errorCode(t, res); code != "GATEWAY_ENDPOINT_LOOPBACK" {
		t.Errorf("code = %q, want GATEWAY_ENDPOINT_LOOPBACK", code)
	}
}

func TestIsLoopbackHost(t *testing.T) {
	for host, want := range map[string]bool{
		"localhost": true, "LOCALHOST": true, "gw.localhost": true, "127.0.0.1": true, "127.1.2.3": true, "::1": true,
		"gateway-runtime": false, "10.0.0.5": false, "example.com": false, "": false,
	} {
		if got := isLoopbackHost(host); got != want {
			t.Errorf("isLoopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}
