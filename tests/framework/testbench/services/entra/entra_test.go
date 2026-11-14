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

package entra

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/testbench"
)

const (
	testClientID  = "5d6e7f80-91a2-4b3c-8d4e-5f6a7b8c9d0e"
	testAssertion = "testbench-federated-identity-token"
	foundryScope  = "https://ai.azure.com/.default"
	cogScope      = "https://cognitiveservices.azure.com/.default"
)

// validForm returns a well-formed client credentials request with a client assertion.
func validForm(scope string) url.Values {
	return url.Values{
		"grant_type":            {"client_credentials"},
		"client_id":             {testClientID},
		"scope":                 {scope},
		"client_assertion_type": {ClientAssertionType},
		"client_assertion":      {testAssertion},
	}
}

// serve sends req to h and returns the recorded response.
func serve(t *testing.T, h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// requestToken posts form to the tenant's token endpoint.
func requestToken(t *testing.T, svc *Service, tenant string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/"+tenant+"/oauth2/v2.0/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return serve(t, svc.Handler(), req)
}

type tokenResponse struct {
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	ExtExpiresIn int    `json:"ext_expires_in"`
	RefreshIn    *int   `json:"refresh_in"`
	AccessToken  string `json:"access_token"`
}

// issued asserts rec is a 200 token response and decodes it.
func issued(t *testing.T, rec *httptest.ResponseRecorder) tokenResponse {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var tok tokenResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tok))
	return tok
}

// claimsOf decodes the claims of a token the service issued.
func claimsOf(t *testing.T, token string) Claims {
	t.Helper()
	encoded, ok := strings.CutPrefix(token, TokenPrefix)
	require.True(t, ok, "token %q lacks the %q prefix", token, TokenPrefix)
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	require.NoError(t, err)
	var c Claims
	require.NoError(t, json.Unmarshal(raw, &c))
	return c
}

type requestLog struct {
	Count    int       `json:"count"`
	Requests []Request `json:"requests"`
}

// recorded returns the tenant's token request log.
func recorded(t *testing.T, svc *Service, tenant string) requestLog {
	t.Helper()
	rec := serve(t, svc.Handler(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/"+tenant+"/test/requests", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var l requestLog
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &l))
	return l
}

// callResource posts a chat request to a protected resource, with authorization when it is set.
func callResource(t *testing.T, svc *Service, path, authorization string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(`{"model":"chat","messages":[]}`))
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	return serve(t, svc.Handler(), req)
}

type oauthError struct {
	Error       string `json:"error"`
	Description string `json:"error_description"`
	Codes       []int  `json:"error_codes"`
	TraceID     string `json:"trace_id"`
	Correlation string `json:"correlation_id"`
}

func TestServiceContract(t *testing.T) {
	svc := New()
	require.Equal(t, "entra", svc.Name())
	require.Equal(t, Port, svc.Port())
	require.True(t, svc.Stateful())
	require.Equal(t, testbench.PartitionByBlock, svc.PartitionKey())
	require.NoError(t, (&testbench.Registry{}).Register(svc))
}

func TestTokenIssuesClaimsFromTheForm(t *testing.T) {
	tok := issued(t, requestToken(t, New(), "t1", validForm(foundryScope)))
	require.Equal(t, "Bearer", tok.TokenType)
	require.Equal(t, ExpiresIn, tok.ExpiresIn)
	require.Equal(t, ExpiresIn, tok.ExtExpiresIn)
	require.Nil(t, tok.RefreshIn, "an ordinary tenant gets no refresh_in")
	require.Equal(t, Claims{Aud: FoundryAudience, Tid: "t1", AppID: testClientID, N: 1}, claimsOf(t, tok.AccessToken))
}

func TestTokenRejectsABadForm(t *testing.T) {
	cases := []struct {
		name      string
		mutate    func(url.Values)
		wantError string
		wantCode  int
		wantInfo  string
	}{
		{"wrong grant type", func(f url.Values) { f.Set("grant_type", "password") }, "unsupported_grant_type", 70003, ""},
		{"missing assertion", func(f url.Values) { f.Del("client_assertion") }, "invalid_request", 50027, ""},
		{"wrong assertion type", func(f url.Values) { f.Set("client_assertion_type", "secret") }, "invalid_request", 50027, ""},
		{"missing client_id", func(f url.Values) { f.Del("client_id") }, "invalid_request", 900144, ""},
		{"scope without /.default", func(f url.Values) { f.Set("scope", "https://ai.azure.com") }, "invalid_scope", 1002012, ""},
		{"grant type repeated after a valid one", func(f url.Values) { f.Add("grant_type", "password") }, "invalid_request", 9000411, "'grant_type'"},
		{"scope repeated with the same value", func(f url.Values) { f.Add("scope", foundryScope) }, "invalid_request", 9000411, "'scope'"},
		{"empty parameter name repeated", func(f url.Values) { f.Add("", "a"); f.Add("", "b") }, "invalid_request", 9000411, "parameter '' is"},
		{"two parameters repeated", func(f url.Values) { f.Add("scope", foundryScope); f.Add("grant_type", "password") }, "invalid_request", 9000411, "'grant_type'"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := New()
			form := validForm(foundryScope)
			c.mutate(form)
			rec := requestToken(t, svc, "t1", form)
			require.Equal(t, http.StatusBadRequest, rec.Code)
			var e oauthError
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e))
			require.Equal(t, c.wantError, e.Error)
			require.Equal(t, []int{c.wantCode}, e.Codes)
			require.Contains(t, e.Description, c.wantInfo)
			log := recorded(t, svc, "t1")
			require.Equal(t, 1, log.Count, "a rejected request is still recorded")
			require.Equal(t, http.StatusBadRequest, log.Requests[0].Status)
		})
	}

	t.Run("not a form", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/t1/oauth2/v2.0/token", strings.NewReader(`{"grant_type":"client_credentials"}`))
		req.Header.Set("Content-Type", "application/json")
		require.Equal(t, http.StatusBadRequest, serve(t, New().Handler(), req).Code)
	})

	t.Run("oversized form", func(t *testing.T) {
		form := validForm(foundryScope)
		form.Set("client_assertion", strings.Repeat("a", maxFormBytes))
		require.Equal(t, http.StatusBadRequest, requestToken(t, New(), "t1", form).Code)
	})
}

func TestRequestsAreLoggedPerTenant(t *testing.T) {
	svc := New()
	issued(t, requestToken(t, svc, "t1", validForm(foundryScope)))
	issued(t, requestToken(t, svc, "t1", validForm(cogScope)))
	issued(t, requestToken(t, svc, "t2", validForm(foundryScope)))

	log := recorded(t, svc, "t1")
	require.Equal(t, 2, log.Count)
	require.Equal(t, Request{
		ClientID:            testClientID,
		Scope:               foundryScope,
		GrantType:           "client_credentials",
		ClientAssertionType: ClientAssertionType,
		ClientAssertion:     testAssertion,
		Status:              http.StatusOK,
	}, log.Requests[0])
	require.Equal(t, cogScope, log.Requests[1].Scope)
	require.Equal(t, 1, recorded(t, svc, "t2").Count)
	require.Equal(t, requestLog{Requests: []Request{}}, recorded(t, svc, "never-called"))
}

func TestModesAreChosenByTenantPrefix(t *testing.T) {
	cases := []struct {
		tenant string
		want   []int
	}{
		{PrefixInvalidClient + "x", []int{http.StatusUnauthorized, http.StatusUnauthorized}},
		{PrefixRejectOnce + "x", []int{http.StatusUnauthorized, http.StatusOK}},
		{PrefixThrottleOnce + "x", []int{http.StatusTooManyRequests, http.StatusOK}},
		{PrefixErrorOnce + "x", []int{http.StatusInternalServerError, http.StatusOK}},
		{PrefixUnavailable + "x", []int{http.StatusServiceUnavailable, http.StatusServiceUnavailable}},
		{PrefixRefreshSoon + "x", []int{http.StatusOK, http.StatusOK}},
		{"plain-x", []int{http.StatusOK, http.StatusOK}},
	}
	for _, c := range cases {
		t.Run(c.tenant, func(t *testing.T) {
			svc := newWithSlowDelay(time.Millisecond)
			var recs []*httptest.ResponseRecorder
			for i, want := range c.want {
				rec := requestToken(t, svc, c.tenant, validForm(foundryScope))
				require.Equal(t, want, rec.Code, "request %d: %s", i+1, rec.Body.String())
				recs = append(recs, rec)
			}
			log := recorded(t, svc, c.tenant)
			require.Equal(t, len(c.want), log.Count)
			for i, want := range c.want {
				require.Equal(t, want, log.Requests[i].Status)
			}

			switch {
			case strings.HasPrefix(c.tenant, PrefixInvalidClient), strings.HasPrefix(c.tenant, PrefixRejectOnce):
				var e oauthError
				require.NoError(t, json.Unmarshal(recs[0].Body.Bytes(), &e))
				require.Equal(t, "invalid_client", e.Error)
				require.Equal(t, []int{700213}, e.Codes)
				require.True(t, strings.HasPrefix(e.Description, "AADSTS700213:"), e.Description)
				require.NotEmpty(t, e.TraceID)
				require.NotEmpty(t, e.Correlation)
			case strings.HasPrefix(c.tenant, PrefixThrottleOnce):
				require.Equal(t, "1", recs[0].Header().Get("Retry-After"))
			case strings.HasPrefix(c.tenant, PrefixErrorOnce):
				require.Empty(t, recs[0].Body.String())
			case strings.HasPrefix(c.tenant, PrefixRefreshSoon):
				first, second := issued(t, recs[0]), issued(t, recs[1])
				require.NotNil(t, first.RefreshIn)
				require.Equal(t, 2, *first.RefreshIn)
				require.Nil(t, second.RefreshIn, "only the first token is marked for an early refresh")
				require.NotEqual(t, first.AccessToken, second.AccessToken)
			}
		})
	}
}

// A malformed request is answered 400 and recorded, but does not use up a tenant's one-time
// failure: the first well-formed request still gets it.
func TestAMalformedRequestDoesNotUseUpAOneTimeFailure(t *testing.T) {
	svc := New()
	tenant := PrefixRejectOnce + "x"
	malformed := validForm(foundryScope)
	malformed.Del("client_id")

	require.Equal(t, http.StatusBadRequest, requestToken(t, svc, tenant, malformed).Code)
	require.Equal(t, http.StatusUnauthorized, requestToken(t, svc, tenant, validForm(foundryScope)).Code)
	tok := issued(t, requestToken(t, svc, tenant, validForm(foundryScope)))
	require.Equal(t, 3, claimsOf(t, tok.AccessToken).N, "N counts every request for the tenant")

	log := recorded(t, svc, tenant)
	require.Equal(t, 3, log.Count, "the malformed request is still counted")
	require.Equal(t, []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusOK}, statuses(log))
}

// A malformed request does not use up a refresh-soon- tenant's early refresh either: the first
// token still carries refresh_in, though N counts the malformed request.
func TestAMalformedRequestDoesNotUseUpTheEarlyRefresh(t *testing.T) {
	svc := New()
	tenant := PrefixRefreshSoon + "x"
	malformed := validForm(foundryScope)
	malformed.Del("client_id")

	require.Equal(t, http.StatusBadRequest, requestToken(t, svc, tenant, malformed).Code)
	first := issued(t, requestToken(t, svc, tenant, validForm(foundryScope)))
	require.NotNil(t, first.RefreshIn, "the first token carries refresh_in")
	require.Equal(t, refreshSoonSeconds, *first.RefreshIn)
	require.Equal(t, 2, claimsOf(t, first.AccessToken).N, "N counts every request for the tenant")
	second := issued(t, requestToken(t, svc, tenant, validForm(foundryScope)))
	require.Nil(t, second.RefreshIn, "only the first token is marked for an early refresh")
}

// statuses returns the status of every request in log, oldest first.
func statuses(log requestLog) []int {
	out := make([]int, 0, len(log.Requests))
	for _, r := range log.Requests {
		out = append(out, r.Status)
	}
	return out
}

// guidPattern matches a lowercase GUID, the shape of Entra ID's trace_id and correlation_id.
var guidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Error bodies carry GUIDs as trace and correlation IDs, as Entra ID's do, so the tenant appears
// in a gateway log only when the gateway logs it.
func TestErrorIDsAreGUIDsWithoutTheTenant(t *testing.T) {
	malformed := validForm(foundryScope)
	malformed.Del("client_id")
	cases := []struct {
		tenant string
		form   url.Values
		status int
	}{
		{"guid-t1", malformed, http.StatusBadRequest},
		{PrefixInvalidClient + "guid", validForm(foundryScope), http.StatusUnauthorized},
		{PrefixThrottleOnce + "guid", validForm(foundryScope), http.StatusTooManyRequests},
		{PrefixUnavailable + "guid", validForm(foundryScope), http.StatusServiceUnavailable},
	}
	seen := map[string]bool{}
	for _, c := range cases {
		rec := requestToken(t, New(), c.tenant, c.form)
		require.Equal(t, c.status, rec.Code, rec.Body.String())
		var e oauthError
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e))
		for _, id := range []string{e.TraceID, e.Correlation} {
			require.Regexp(t, guidPattern, id, c.tenant)
			require.NotContains(t, id, c.tenant)
			require.False(t, seen[id], "every ID is fresh: %s", id)
			seen[id] = true
		}
	}
}

func TestSlowWaitsAndStopsWhenCancelled(t *testing.T) {
	svc := newWithSlowDelay(50 * time.Millisecond)
	start := time.Now()
	issued(t, requestToken(t, svc, PrefixSlow+"x", validForm(foundryScope)))
	require.GreaterOrEqual(t, time.Since(start), 50*time.Millisecond)

	svc = newWithSlowDelay(time.Hour)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/"+PrefixSlow+"y/oauth2/v2.0/token",
		strings.NewReader(validForm(foundryScope).Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		svc.Handler().ServeHTTP(rec, req)
		close(done)
	}()
	// Cancel only once the request is admitted, so the handler is past admission and waiting.
	require.NoError(t, retry.Await(t.Context(), retry.Options{Fast: true},
		func(context.Context) (int, error) { return recorded(t, svc, PrefixSlow+"y").Count, nil },
		func(count int) bool { return count == 1 },
		"waiting for the slow token request to be admitted"))
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("slow mode kept waiting after the caller went away")
	}
	require.Zero(t, rec.Body.Len(), "a caller that left is sent no token")
	log := recorded(t, svc, PrefixSlow+"y")
	require.Equal(t, 1, log.Count)
	require.Equal(t, 0, log.Requests[0].Status, "a request whose caller left before the answer is recorded with status 0")
}

func TestPartitionBudget(t *testing.T) {
	svc := New()
	for i := 0; i < maxPartitions; i++ {
		svc.partitions[fmt.Sprintf("tenant-%d", i)] = &partition{}
	}
	require.Equal(t, http.StatusInsufficientStorage, requestToken(t, svc, "one-too-many", validForm(foundryScope)).Code)
	issued(t, requestToken(t, svc, "tenant-1", validForm(foundryScope)))
}

func TestRetentionIsBoundedButTheCountIsExact(t *testing.T) {
	svc := New()
	for i := 0; i < maxRetainedRequests+5; i++ {
		issued(t, requestToken(t, svc, "t1", validForm(foundryScope)))
	}
	log := recorded(t, svc, "t1")
	require.Equal(t, maxRetainedRequests+5, log.Count)
	require.Len(t, log.Requests, maxRetainedRequests)
}

// requestTokensConcurrently sends n well-formed token requests for the tenant at once and returns
// every response after all are answered. The goroutines only serve; the caller asserts.
func requestTokensConcurrently(t *testing.T, svc *Service, tenant string, n int) []*httptest.ResponseRecorder {
	t.Helper()
	handler := svc.Handler()
	body := validForm(foundryScope).Encode()
	recs := make([]*httptest.ResponseRecorder, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range recs {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/"+tenant+"/oauth2/v2.0/token", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		recs[i] = rec
		wg.Go(func() {
			<-start
			handler.ServeHTTP(rec, req)
		})
	}
	close(start)
	wg.Wait()
	return recs
}

func TestConcurrentTokenRequestsAreCountedExactly(t *testing.T) {
	svc := New()
	seen := map[int]bool{}
	for _, rec := range requestTokensConcurrently(t, svc, "t1", 50) {
		seen[claimsOf(t, issued(t, rec).AccessToken).N] = true
	}
	require.Len(t, seen, 50, "every request gets its own sequence number")
	require.Equal(t, 50, recorded(t, svc, "t1").Count)
}

// A one-time failure fails exactly one of many concurrent requests: the first one admitted.
func TestConcurrentRequestsShareOneTimeFailure(t *testing.T) {
	svc := New()
	tenant := PrefixRejectOnce + "concurrent"
	codes := map[int]int{}
	for _, rec := range requestTokensConcurrently(t, svc, tenant, 50) {
		codes[rec.Code]++
	}
	require.Equal(t, map[int]int{http.StatusUnauthorized: 1, http.StatusOK: 49}, codes)
	log := recorded(t, svc, tenant)
	require.Equal(t, 50, log.Count)
	require.Equal(t, http.StatusUnauthorized, log.Requests[0].Status, "the request admitted first is the one that fails")
}

func TestReservedAndInvalidTenantsAreRejected(t *testing.T) {
	for _, tenant := range []string{"test", "Upper-Case", "contoso.onmicrosoft.com"} {
		require.Equal(t, http.StatusBadRequest, requestToken(t, New(), tenant, validForm(foundryScope)).Code, tenant)
	}
}

func TestResourceChecksBearerAndAudience(t *testing.T) {
	svc := New()
	foundryToken := issued(t, requestToken(t, svc, "t1", validForm(foundryScope))).AccessToken
	cogToken := issued(t, requestToken(t, svc, "t1", validForm(cogScope))).AccessToken

	rec := callResource(t, svc, "/t1/foundry/chat/completions", "Bearer "+foundryToken)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, foundryToken, rec.Header().Get(HeaderToken))
	require.Equal(t, "t1", rec.Header().Get(HeaderTenant))
	require.Equal(t, FoundryAudience, rec.Header().Get(HeaderAudience))
	var completion struct {
		Object  string            `json:"object"`
		Headers map[string]string `json:"headers"`
		Token   Claims            `json:"token"`
		Usage   map[string]int    `json:"usage"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &completion))
	require.Equal(t, "chat.completion", completion.Object)
	require.Equal(t, "Bearer "+foundryToken, completion.Headers["Authorization"])
	require.Equal(t, "t1", completion.Token.Tid)
	require.Equal(t, 10, completion.Usage["total_tokens"])

	rec = callResource(t, svc, "/t1/foundry/chat/completions", "Bearer "+cogToken)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "audience")
	require.Equal(t, CognitiveAudience, rec.Header().Get(HeaderAudience))

	rec = callResource(t, svc, "/t1/cognitive/anything", "Bearer "+cogToken)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, CognitiveAudience, rec.Header().Get(HeaderAudience))

	for _, authorization := range []string{"", "Bearer not-a-testbench-token", "Basic " + foundryToken, "Bearer " + TokenPrefix + "%%%"} {
		rec = callResource(t, svc, "/t1/foundry", authorization)
		require.Equal(t, http.StatusUnauthorized, rec.Code, authorization)
		require.Empty(t, rec.Header().Get(HeaderTenant), authorization)
	}
	rec = callResource(t, svc, "/t1/foundry", "")
	require.Empty(t, rec.Header().Values(HeaderToken), "no bearer token, no token header")

	rec = callResource(t, svc, "/t1/elsewhere", "Bearer "+foundryToken)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestOverTLSSharesTheServiceState(t *testing.T) {
	certPEM, keyPEM := selfSignedPEM(t)
	svc := New()
	secure, err := svc.OverTLS(certPEM, keyPEM)
	require.NoError(t, err)
	require.Equal(t, "entra-tls", secure.Name())
	require.Equal(t, TLSPort, secure.Port())
	tlsService, ok := secure.(testbench.TLSService)
	require.True(t, ok, "the HTTPS service must declare its TLS configuration")

	registry := &testbench.Registry{}
	require.NoError(t, registry.Register(svc))
	require.NoError(t, registry.Register(secure))

	server := httptest.NewUnstartedServer(secure.Handler())
	server.TLS = tlsService.TLSConfig().Clone()
	server.StartTLS()
	defer server.Close()

	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(certPEM))
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: roots, ServerName: "testbench", MinVersion: tls.VersionTLS12},
	}}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/t1/oauth2/v2.0/token",
		strings.NewReader(validForm(foundryScope).Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	require.NoError(t, err, "a client that trusts only the testbench certificate completes the handshake")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, 1, recorded(t, svc, "t1").Count, "a token requested over HTTPS is read back from the plain service")
}

func TestOverTLSRejectsAnInvalidKeyPair(t *testing.T) {
	_, err := New().OverTLS([]byte("not a certificate"), []byte("not a key"))
	require.Error(t, err)
}

// selfSignedPEM returns a self-signed certificate for host testbench and its key, both PEM-encoded.
func selfSignedPEM(t *testing.T) ([]byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "testbench"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		DNSNames:              []string{"testbench"},
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

const (
	testManagedClientID = "11111111-2222-3333-4444-555555555555"
	// testBlock is the block partition a test's IMDS requests are sent under.
	testBlock = "block-a"
)

// imdsGet sends a managed identity token request to the IMDS route under a block's partition.
func imdsGet(t *testing.T, svc *Service, block string, query url.Values, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/"+block+"/metadata/identity/oauth2/token?"+query.Encode(), nil)
	for name, values := range header {
		req.Header[name] = values
	}
	return serve(t, svc.Handler(), req)
}

// imdsGetRaw sends a managed identity token request whose query string is sent exactly as given.
func imdsGetRaw(t *testing.T, svc *Service, block, rawQuery string, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/"+block+"/metadata/identity/oauth2/token?"+rawQuery, nil)
	for name, values := range header {
		req.Header[name] = values
	}
	return serve(t, svc.Handler(), req)
}

// imdsRecorded returns the block's IMDS request log for an identity: a client ID, or
// SystemAssigned.
func imdsRecorded(t *testing.T, svc *Service, block, identity string) requestLog {
	t.Helper()
	path := "/" + block + "/test/managed-identity/" + identity
	rec := serve(t, svc.Handler(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var l requestLog
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &l))
	return l
}

// imdsQuery is a well-formed IMDS query; an empty clientID asks for the system-assigned identity.
func imdsQuery(clientID, resource string) url.Values {
	q := url.Values{"api-version": {IMDSAPIVersion}, "resource": {resource}}
	if clientID != "" {
		q.Set("client_id", clientID)
	}
	return q
}

var metadataTrue = http.Header{"Metadata": {"true"}}

func TestIMDSIssuesATokenForTheResource(t *testing.T) {
	svc := New()
	rec := imdsGet(t, svc, testBlock, imdsQuery(testManagedClientID, FoundryAudience), metadataTrue)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "3599", body["expires_in"], "IMDS sends numbers as strings")
	require.Equal(t, "Bearer", body["token_type"])
	require.Equal(t, FoundryAudience, body["resource"])
	require.Equal(t, testManagedClientID, body["client_id"])
	token, ok := body["access_token"].(string)
	require.True(t, ok)
	require.Equal(t, Claims{Aud: FoundryAudience, Tid: "managed-identity", AppID: testManagedClientID, N: 1}, claimsOf(t, token))

	log := imdsRecorded(t, svc, testBlock, testManagedClientID)
	require.Equal(t, 1, log.Count)
	got := log.Requests[0]
	require.Equal(t, testManagedClientID, got.ClientID)
	require.Equal(t, FoundryAudience, got.Resource)
	require.Equal(t, "2018-02-01", got.APIVersion)
	require.Equal(t, "true", got.Metadata)
	require.Equal(t, http.StatusOK, got.Status)
}

func TestIMDSSystemAssignedIdentity(t *testing.T) {
	svc := New()
	rec := imdsGet(t, svc, testBlock, imdsQuery("", FoundryAudience), metadataTrue)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	token, _ := body["access_token"].(string)
	require.Equal(t, "system-assigned", claimsOf(t, token).AppID)

	log := imdsRecorded(t, svc, testBlock, SystemAssigned)
	require.Equal(t, 1, log.Count)
	require.Empty(t, log.Requests[0].ClientID)
}

func TestIMDSRejectsAMalformedRequest(t *testing.T) {
	// repeated is a well-formed query with one parameter given a second, acceptable value.
	repeated := func(name, value string) url.Values {
		q := imdsQuery(testManagedClientID, FoundryAudience)
		q.Add(name, value)
		return q
	}
	cases := []struct {
		name      string
		query     url.Values
		header    http.Header
		wantError string
		wantInfo  string
	}{
		{"no Metadata header", imdsQuery(testManagedClientID, FoundryAudience), nil, "bad_request_102", ""},
		{"Metadata false", imdsQuery(testManagedClientID, FoundryAudience), http.Header{"Metadata": {"false"}}, "bad_request_102", ""},
		{"Metadata true and false", imdsQuery(testManagedClientID, FoundryAudience), http.Header{"Metadata": {"true", "false"}}, "bad_request_102", ""},
		{"Metadata true twice", imdsQuery(testManagedClientID, FoundryAudience), http.Header{"Metadata": {"true", "true"}}, "bad_request_102", ""},
		{"no api-version", url.Values{"resource": {FoundryAudience}, "client_id": {testManagedClientID}}, metadataTrue, "invalid_request", ""},
		{"another api-version", url.Values{"api-version": {"2019-01-01"}, "resource": {FoundryAudience}, "client_id": {testManagedClientID}}, metadataTrue, "invalid_request", ""},
		{"no resource", url.Values{"api-version": {IMDSAPIVersion}, "client_id": {testManagedClientID}}, metadataTrue, "invalid_request", ""},
		{"X-Forwarded-For", imdsQuery(testManagedClientID, FoundryAudience), http.Header{"Metadata": {"true"}, "X-Forwarded-For": {"10.0.0.1"}}, "invalid_request", ""},
		{"an empty X-Forwarded-For", imdsQuery(testManagedClientID, FoundryAudience), http.Header{"Metadata": {"true"}, "X-Forwarded-For": {""}}, "invalid_request", "X-Forwarded-For"},
		{"resource given twice", repeated("resource", CognitiveAudience), metadataTrue, "invalid_request", "'resource'"},
		{"client_id given twice", repeated("client_id", "99999999-8888-7777-6666-555555555555"), metadataTrue, "invalid_request", "'client_id'"},
		{"api-version given twice", repeated("api-version", IMDSAPIVersion), metadataTrue, "invalid_request", "'api-version'"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := New()
			rec := imdsGet(t, svc, testBlock, c.query, c.header)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			var e oauthError
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e))
			require.Equal(t, c.wantError, e.Error)
			require.Contains(t, e.Description, c.wantInfo)
			log := imdsRecorded(t, svc, testBlock, testManagedClientID)
			require.Equal(t, 1, log.Count, "a rejected request is still recorded")
			require.Equal(t, http.StatusBadRequest, log.Requests[0].Status)
		})
	}
}

// A query string that does not parse cleanly is rejected, not read with the bad pairs left out.
func TestIMDSRejectsAMalformedQueryString(t *testing.T) {
	base := "api-version=" + IMDSAPIVersion + "&client_id=" + testManagedClientID
	cases := []struct{ name, rawQuery string }{
		{"a bad escape in a repeated parameter", base + "&resource=" + url.QueryEscape(FoundryAudience) + "&resource=%ZZ"},
		{"a semicolon", base + "&resource=" + url.QueryEscape(FoundryAudience) + ";x=y"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := New()
			rec := imdsGetRaw(t, svc, testBlock, c.rawQuery, metadataTrue)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			require.JSONEq(t, `{"error":"invalid_request","error_description":"The query string is malformed"}`, rec.Body.String())
			log := imdsRecorded(t, svc, testBlock, testManagedClientID)
			require.Equal(t, 1, log.Count, "a malformed query is still recorded")
			require.Equal(t, http.StatusBadRequest, log.Requests[0].Status)
		})
	}

	// A semicolon that hides client_id would otherwise get a token for the system-assigned identity.
	svc := New()
	hidden := "api-version=" + IMDSAPIVersion + "&resource=" + url.QueryEscape(FoundryAudience) + "&client_id=" + testManagedClientID + ";x=y"
	require.Equal(t, http.StatusBadRequest, imdsGetRaw(t, svc, testBlock, hidden, metadataTrue).Code)
	require.Equal(t, 1, imdsRecorded(t, svc, testBlock, SystemAssigned).Count,
		"the rejection is logged under the identity the query's well-formed part names")
}

func TestIMDSModesAreChosenByResource(t *testing.T) {
	cases := []struct {
		resource string
		want     [2]int
	}{
		{ResourceNotFoundOnce, [2]int{http.StatusNotFound, http.StatusOK}},
		{ResourceGoneOnce, [2]int{http.StatusGone, http.StatusOK}},
		{ResourceThrottleOnce, [2]int{http.StatusTooManyRequests, http.StatusOK}},
		{ResourceErrorOnce, [2]int{http.StatusInternalServerError, http.StatusOK}},
		{ResourceIdentityNotFound, [2]int{http.StatusBadRequest, http.StatusBadRequest}},
	}
	for _, c := range cases {
		t.Run(c.resource, func(t *testing.T) {
			svc := New()
			for i, want := range c.want {
				rec := imdsGet(t, svc, testBlock, imdsQuery(testManagedClientID, c.resource), metadataTrue)
				require.Equal(t, want, rec.Code, "request %d: %s", i+1, rec.Body.String())
				if c.resource == ResourceIdentityNotFound {
					require.JSONEq(t, `{"error":"invalid_request","error_description":"Identity not found"}`, rec.Body.String())
				}
			}
		})
	}
}

// A malformed IMDS request is answered 400 and recorded, but does not use up the identity's
// one-time failure.
func TestIMDSMalformedRequestDoesNotUseUpAOneTimeFailure(t *testing.T) {
	svc := New()
	query := imdsQuery(testManagedClientID, ResourceNotFoundOnce)

	require.Equal(t, http.StatusBadRequest, imdsGet(t, svc, testBlock, query, nil).Code, "no Metadata header")
	require.Equal(t, http.StatusNotFound, imdsGet(t, svc, testBlock, query, metadataTrue).Code)
	require.Equal(t, http.StatusOK, imdsGet(t, svc, testBlock, query, metadataTrue).Code)

	log := imdsRecorded(t, svc, testBlock, testManagedClientID)
	require.Equal(t, 3, log.Count)
	require.Equal(t, []int{http.StatusBadRequest, http.StatusNotFound, http.StatusOK}, statuses(log))
}

func TestResourceReportsTheTokenAppID(t *testing.T) {
	svc := New()
	rec := imdsGet(t, svc, testBlock, imdsQuery(testManagedClientID, FoundryAudience), metadataTrue)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	token, _ := body["access_token"].(string)

	ok := callResource(t, svc, "/p/foundry", "Bearer "+token)
	require.Equal(t, http.StatusOK, ok.Code, ok.Body.String())
	require.Equal(t, testManagedClientID, ok.Header().Get(HeaderAppID))
	require.Equal(t, "managed-identity", ok.Header().Get(HeaderTenant))

	none := callResource(t, svc, "/p/foundry", "")
	require.Equal(t, http.StatusUnauthorized, none.Code)
	require.Empty(t, none.Header().Values(HeaderAppID))
}

// Two blocks share one testbench, and the gateway in each sends its IMDS requests under its own
// block's partition: the same identity in two blocks keeps two counters and two logs, so each
// block's first request gets the fail-once answer.
func TestIMDSStateIsScopedToTheBlock(t *testing.T) {
	svc := New()
	for _, block := range []string{"block-a", "block-b"} {
		first := imdsGet(t, svc, block, imdsQuery(testManagedClientID, ResourceNotFoundOnce), metadataTrue)
		require.Equal(t, http.StatusNotFound, first.Code, "block %s: the identity's first request", block)
		second := imdsGet(t, svc, block, imdsQuery(testManagedClientID, ResourceNotFoundOnce), metadataTrue)
		require.Equal(t, http.StatusOK, second.Code, "block %s: the identity's second request", block)
		require.Equal(t, http.StatusOK, imdsGet(t, svc, block, imdsQuery("", FoundryAudience), metadataTrue).Code)
	}
	for _, block := range []string{"block-a", "block-b"} {
		require.Equal(t, 2, imdsRecorded(t, svc, block, testManagedClientID).Count, "block %s", block)
		require.Equal(t, 1, imdsRecorded(t, svc, block, SystemAssigned).Count, "block %s", block)
	}
	require.Equal(t, 0, imdsRecorded(t, svc, "block-c", testManagedClientID).Count, "a block that sent nothing has an empty log")
}
