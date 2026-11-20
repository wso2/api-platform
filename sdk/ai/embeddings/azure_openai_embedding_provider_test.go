/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied.  See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

package embeddings

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wso2/api-platform/sdk/ai/azureauthentication"
)

const azureTestClientID = "11111111-2222-3333-4444-555555555555"

type azureTokenCall struct {
	scope   string
	timeout time.Duration
}

// fakeAzureTokens stands in for the gateway's Azure identity.
type fakeAzureTokens struct {
	err error

	mu    sync.Mutex
	n     int
	calls []azureTokenCall
}

func (f *fakeAzureTokens) Token(_ context.Context, scope string, timeout time.Duration) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	f.calls = append(f.calls, azureTokenCall{scope: scope, timeout: timeout})
	if f.err != nil {
		return "", f.err
	}
	return "tok-" + string(rune('0'+f.n)), nil
}

func (f *fakeAzureTokens) snapshot() []azureTokenCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]azureTokenCall(nil), f.calls...)
}

// useFakeAzureIdentity replaces getTokenSource for one test and returns the identities asked for.
func useFakeAzureIdentity(t *testing.T, tokens *fakeAzureTokens, getErr error) *[]azureauthentication.Identity {
	t.Helper()
	var asked []azureauthentication.Identity
	prev := getTokenSource
	getTokenSource = func(id azureauthentication.Identity) (tokenSource, error) {
		asked = append(asked, id)
		if getErr != nil {
			return nil, getErr
		}
		return tokens, nil
	}
	t.Cleanup(func() { getTokenSource = prev })
	return &asked
}

// azureServer answers every request with a valid embedding and records headers and hits.
type azureServer struct {
	*httptest.Server
	hits    atomic.Int32
	mu      sync.Mutex
	headers []http.Header
}

func newAzureServer(t *testing.T) *azureServer {
	t.Helper()
	s := &azureServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.hits.Add(1)
		s.mu.Lock()
		s.headers = append(s.headers, r.Header.Clone())
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"embedding":[0.1,0.2]}]}`))
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *azureServer) lastHeader() http.Header {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.headers[len(s.headers)-1]
}

func azureConfig(endpoint string) EmbeddingProviderConfig {
	return EmbeddingProviderConfig{
		EmbeddingProvider: "AZURE_OPENAI",
		EmbeddingEndpoint: endpoint,
		AuthHeaderName:    "api-key",
		APIKey:            "k",
		TimeOut:           "5",
	}
}

func identityConfig(endpoint, typ string) EmbeddingProviderConfig {
	c := azureConfig(endpoint)
	c.AuthenticationType = typ
	return c
}

func TestAzureOpenAIInitGetsTheIdentityOnce(t *testing.T) {
	tokens := &fakeAzureTokens{}
	asked := useFakeAzureIdentity(t, tokens, nil)
	cfg := identityConfig("http://example.invalid", azureauthentication.WorkloadIdentity)
	cfg.ClientID = azureTestClientID
	p := &AzureOpenAIEmbeddingProvider{}
	if err := p.Init(cfg); err != nil {
		t.Fatal(err)
	}
	if len(*asked) != 1 || (*asked)[0] != (azureauthentication.Identity{Type: "workload-identity", ClientID: azureTestClientID}) {
		t.Fatalf("identities asked for = %+v", *asked)
	}
	if n := len(tokens.snapshot()); n != 0 {
		t.Fatalf("Init fetched %d tokens", n)
	}
}

func TestAzureOpenAIInitFailsWhenTheIdentityIsUnavailable(t *testing.T) {
	useFakeAzureIdentity(t, nil, errors.New("no IDENTITY_ENDPOINT"))
	p := &AzureOpenAIEmbeddingProvider{}
	err := p.Init(identityConfig("http://example.invalid", azureauthentication.ContainerApps))
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{`authentication type "container-apps" can't be used`, "no IDENTITY_ENDPOINT"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

func TestAzureOpenAIAPIKeyModeIsUnchanged(t *testing.T) {
	prev := getTokenSource
	getTokenSource = func(azureauthentication.Identity) (tokenSource, error) {
		t.Error("getTokenSource called in api-key mode")
		return nil, errors.New("unexpected")
	}
	t.Cleanup(func() { getTokenSource = prev })
	srv := newAzureServer(t)
	p := &AzureOpenAIEmbeddingProvider{}
	if err := p.Init(azureConfig(srv.URL)); err != nil {
		t.Fatal(err)
	}
	if _, err := p.GetEmbedding("x"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.GetEmbeddings([]string{"x"}); err != nil {
		t.Fatal(err)
	}
	h := srv.lastHeader()
	if h.Get("api-key") != "k" || h.Get("Authorization") != "" {
		t.Fatalf("headers = %v", h)
	}
}

func TestAzureOpenAIIdentityModeSendsBearerOnly(t *testing.T) {
	tokens := &fakeAzureTokens{}
	useFakeAzureIdentity(t, tokens, nil)
	srv := newAzureServer(t)
	p := &AzureOpenAIEmbeddingProvider{}
	if err := p.Init(identityConfig(srv.URL, azureauthentication.WorkloadIdentity)); err != nil {
		t.Fatal(err)
	}
	if _, err := p.GetEmbedding("x"); err != nil {
		t.Fatal(err)
	}
	h := srv.lastHeader()
	if h.Get("Authorization") != "Bearer tok-1" || len(h.Values("api-key")) != 0 {
		t.Fatalf("GetEmbedding headers = %v", h)
	}
	if _, err := p.GetEmbeddings([]string{"x"}); err != nil {
		t.Fatal(err)
	}
	h = srv.lastHeader()
	if h.Get("Authorization") != "Bearer tok-2" || len(h.Values("api-key")) != 0 {
		t.Fatalf("GetEmbeddings headers = %v", h)
	}
}

func runWithScope(t *testing.T, scope string) azureTokenCall {
	t.Helper()
	tokens := &fakeAzureTokens{}
	useFakeAzureIdentity(t, tokens, nil)
	srv := newAzureServer(t)
	cfg := identityConfig(srv.URL, azureauthentication.WorkloadIdentity)
	cfg.TokenScope = scope
	p := &AzureOpenAIEmbeddingProvider{}
	if err := p.Init(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := p.GetEmbedding("x"); err != nil {
		t.Fatal(err)
	}
	calls := tokens.snapshot()
	if len(calls) != 1 {
		t.Fatalf("token calls = %d", len(calls))
	}
	return calls[0]
}

func TestAzureOpenAIIdentityModeUsesTheDefaultScope(t *testing.T) {
	if got := runWithScope(t, "").scope; got != DefaultTokenScope {
		t.Fatalf("scope = %q", got)
	}
}

func TestAzureOpenAIIdentityModeUsesTheConfiguredScope(t *testing.T) {
	const scope = "https://ai.azure.com/.default"
	if got := runWithScope(t, scope).scope; got != scope {
		t.Fatalf("scope = %q", got)
	}
}

func TestAzureOpenAIIdentityModeTokenRequestIsBounded(t *testing.T) {
	if got := runWithScope(t, "").timeout; got != azureauthentication.DefaultTokenTimeout {
		t.Fatalf("timeout = %v", got)
	}
}

func TestAzureOpenAIIdentityModeGetsATokenPerCall(t *testing.T) {
	tokens := &fakeAzureTokens{}
	useFakeAzureIdentity(t, tokens, nil)
	srv := newAzureServer(t)
	p := &AzureOpenAIEmbeddingProvider{}
	if err := p.Init(identityConfig(srv.URL, azureauthentication.WorkloadIdentity)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := p.GetEmbedding("x"); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(tokens.snapshot()); n != 2 {
		t.Fatalf("token calls = %d", n)
	}
}

func TestAzureOpenAITokenFailureSendsNoRequest(t *testing.T) {
	tokens := &fakeAzureTokens{err: &azureauthentication.TokenError{Reason: "Microsoft Entra ID answered 400 invalid_client"}}
	useFakeAzureIdentity(t, tokens, nil)
	srv := newAzureServer(t)
	p := &AzureOpenAIEmbeddingProvider{}
	if err := p.Init(identityConfig(srv.URL, azureauthentication.WorkloadIdentity)); err != nil {
		t.Fatal(err)
	}
	_, err1 := p.GetEmbedding("x")
	_, err2 := p.GetEmbeddings([]string{"x"})
	for _, err := range []error{err1, err2} {
		var terr *azureauthentication.TokenError
		if err == nil || !errors.As(err, &terr) {
			t.Fatalf("err = %v, want a wrapped TokenError", err)
		}
		if !strings.HasPrefix(err.Error(), "could not obtain an access token for the embedding provider") {
			t.Errorf("err = %q", err)
		}
	}
	if n := srv.hits.Load(); n != 0 {
		t.Fatalf("server hits = %d", n)
	}
}

// redirectingAzureServer answers every request with a 307 to elsewhere.
func redirectingAzureServer(t *testing.T, elsewhere string) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere+r.URL.RequestURI(), http.StatusTemporaryRedirect)
	}))
	t.Cleanup(s.Close)
	return s
}

func TestAzureOpenAIRefusesRedirects(t *testing.T) {
	for _, mode := range []string{AuthenticationTypeAPIKey, azureauthentication.WorkloadIdentity} {
		t.Run(mode, func(t *testing.T) {
			useFakeAzureIdentity(t, &fakeAzureTokens{}, nil)
			elsewhere := newAzureServer(t)
			listed := redirectingAzureServer(t, elsewhere.URL)
			p := &AzureOpenAIEmbeddingProvider{}
			if err := p.Init(identityConfig(listed.URL, mode)); err != nil {
				t.Fatal(err)
			}
			_, err1 := p.GetEmbedding("x")
			_, err2 := p.GetEmbeddings([]string{"x"})
			for name, err := range map[string]error{"GetEmbedding": err1, "GetEmbeddings": err2} {
				var serr *StatusError
				if !errors.As(err, &serr) || serr.StatusCode != http.StatusTemporaryRedirect {
					t.Errorf("%s err = %v, want a StatusError with status 307", name, err)
				}
			}
			if n := elsewhere.hits.Load(); n != 0 {
				t.Fatalf("the redirect target was hit %d times", n)
			}
		})
	}
}

func TestAzureOpenAIReportsTheStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":"PermissionDenied"}}`))
	}))
	t.Cleanup(srv.Close)
	p := &AzureOpenAIEmbeddingProvider{}
	if err := p.Init(azureConfig(srv.URL)); err != nil {
		t.Fatal(err)
	}
	_, err1 := p.GetEmbedding("x")
	_, err2 := p.GetEmbeddings([]string{"x"})
	for name, err := range map[string]error{"GetEmbedding": err1, "GetEmbeddings": err2} {
		var serr *StatusError
		if !errors.As(err, &serr) {
			t.Fatalf("%s err = %v, want a StatusError", name, err)
		}
		if serr.StatusCode != http.StatusForbidden || serr.Body != `{"error":{"code":"PermissionDenied"}}` {
			t.Errorf("%s StatusError = %+v", name, serr)
		}
		if want := `Azure OpenAI API error: status 403, body: {"error":{"code":"PermissionDenied"}}`; err.Error() != want {
			t.Errorf("%s err = %q, want %q", name, err, want)
		}
	}
}
