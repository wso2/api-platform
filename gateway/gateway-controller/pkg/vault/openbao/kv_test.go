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

package openbao

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/vault"
)

// fakeKV serves a minimal KV v2 read API with versioned entries and one valid token.
type fakeKV struct {
	mu        sync.Mutex
	token     string
	namespace string
	// entries[path][version] = data; version 0 is never used.
	entries  map[string]map[int]map[string]any
	deleted  map[string]bool // "path@version"
	requests []*http.Request
	logins   atomic.Int32
}

func newFakeKV(token string) *fakeKV {
	return &fakeKV{token: token, entries: map[string]map[int]map[string]any{}, deleted: map[string]bool{}}
}

func (f *fakeKV) put(path string, data map[string]any) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.entries[path] == nil {
		f.entries[path] = map[int]map[string]any{}
	}
	v := len(f.entries[path]) + 1
	f.entries[path][v] = data
	return v
}

func (f *fakeKV) setToken(tok string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.token = tok
}

func (f *fakeKV) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.Clone(context.Background()))

	if r.Method == http.MethodPost && r.URL.Path == "/v1/auth/kubernetes/login" {
		f.logins.Add(1)
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["role"] != "gw" || body["jwt"] != "sa-jwt" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"auth": map[string]any{"client_token": f.token, "lease_duration": 3600}})
		return
	}

	if r.Header.Get("X-Vault-Token") != f.token {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":["permission denied"]}`))
		return
	}
	if f.namespace != "" && r.Header.Get("X-Vault-Namespace") != f.namespace {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	path, ok := strings.CutPrefix(r.URL.Path, "/v1/secret/data/")
	if !ok || r.Method != http.MethodGet {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	versions := f.entries[path]
	if len(versions) == 0 {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":[]}`))
		return
	}
	version := len(versions)
	if q := r.URL.Query().Get("version"); q != "" {
		version, _ = strconv.Atoi(q)
	}
	data, ok := versions[version]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	meta := map[string]any{"version": version, "deletion_time": "", "destroyed": false}
	if f.deleted[path+"@"+strconv.Itoa(version)] {
		// KV v2 answers a soft-deleted version with 404 and null data.
		meta["deletion_time"] = "2026-11-28T00:00:00Z"
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"data": nil, "metadata": meta}})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"data": data, "metadata": meta}})
}

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

func newTestKV(t *testing.T, srv *httptest.Server, cfg Config) *KV {
	t.Helper()
	if cfg.Name == "" {
		cfg.Name = "amp"
	}
	cfg.Address = srv.URL
	cfg.HTTPClient = srv.Client()
	kv, err := New(cfg, nil)
	require.NoError(t, err)
	return kv
}

func ref(path, field string, version int) vault.Ref {
	return vault.Ref{Resolver: "amp", Path: path, Field: field, Version: version}
}

func TestKV_ReadsLatestAndPinnedVersion(t *testing.T) {
	f := newFakeKV("tok-1")
	f.put("gateways/gw1/openai", map[string]any{"api_key": "sk-v1"})
	f.put("gateways/gw1/openai", map[string]any{"api_key": "sk-v2"})
	srv := httptest.NewServer(f)
	defer srv.Close()
	kv := newTestKV(t, srv, Config{TokenFile: writeFile(t, "token", "tok-1\n")})

	got, err := kv.Get(context.Background(), ref("gateways/gw1/openai", "api_key", 0))
	require.NoError(t, err)
	assert.Equal(t, "sk-v2", got)

	got, err = kv.Get(context.Background(), ref("gateways/gw1/openai", "api_key", 1))
	require.NoError(t, err)
	assert.Equal(t, "sk-v1", got, "a pinned version stays readable after a newer write")

	last := f.requests[len(f.requests)-1]
	assert.Equal(t, "/v1/secret/data/gateways/gw1/openai", last.URL.Path)
	assert.Equal(t, "1", last.URL.Query().Get("version"))
	assert.Equal(t, http.MethodGet, last.Method)
}

func TestKV_NotFoundCases(t *testing.T) {
	f := newFakeKV("tok")
	f.put("a", map[string]any{"value": "x"})
	f.put("deleted", map[string]any{"value": "x"})
	f.deleted["deleted@1"] = true
	srv := httptest.NewServer(f)
	defer srv.Close()
	kv := newTestKV(t, srv, Config{TokenFile: writeFile(t, "token", "tok")})

	for name, r := range map[string]vault.Ref{
		"missing path":    ref("nope", "value", 0),
		"missing version": ref("a", "value", 9),
		"missing field":   ref("a", "other", 0),
		"deleted version": ref("deleted", "value", 1),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := kv.Get(context.Background(), r)
			assert.ErrorIs(t, err, vault.ErrNotFound)
		})
	}
}

func TestKV_NonStringField(t *testing.T) {
	f := newFakeKV("tok")
	f.put("a", map[string]any{"value": map[string]any{"nested": true}})
	srv := httptest.NewServer(f)
	defer srv.Close()
	kv := newTestKV(t, srv, Config{TokenFile: writeFile(t, "token", "tok")})

	_, err := kv.Get(context.Background(), ref("a", "value", 0))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a string")
}

func TestKV_RereadsRotatedTokenFileAfter403(t *testing.T) {
	f := newFakeKV("tok-old")
	f.put("a", map[string]any{"value": "sk"})
	srv := httptest.NewServer(f)
	defer srv.Close()
	tokenPath := writeFile(t, "token", "tok-old")
	kv := newTestKV(t, srv, Config{TokenFile: tokenPath})

	_, err := kv.Get(context.Background(), ref("a", "value", 0))
	require.NoError(t, err)

	// Rotate the token on the server and on disk; the cached token now fails once.
	f.setToken("tok-new")
	require.NoError(t, os.WriteFile(tokenPath, []byte("tok-new"), 0o600))

	got, err := kv.Get(context.Background(), ref("a", "value", 0))
	require.NoError(t, err)
	assert.Equal(t, "sk", got)
}

func TestKV_AccessDenied(t *testing.T) {
	f := newFakeKV("right")
	f.put("a", map[string]any{"value": "sk"})
	srv := httptest.NewServer(f)
	defer srv.Close()
	kv := newTestKV(t, srv, Config{TokenFile: writeFile(t, "token", "wrong")})

	_, err := kv.Get(context.Background(), ref("a", "value", 0))
	require.ErrorIs(t, err, vault.ErrAccessDenied)
	assert.NotContains(t, err.Error(), "permission denied", "response bodies are never put into errors")
	assert.Len(t, f.requests, 2, "one retry after re-reading the token")
}

func TestKV_KubernetesAuthCachesToken(t *testing.T) {
	f := newFakeKV("k8s-token")
	f.put("a", map[string]any{"value": "sk"})
	srv := httptest.NewServer(f)
	defer srv.Close()
	kv := newTestKV(t, srv, Config{Kubernetes: &KubernetesAuthConfig{Role: "gw", JWTFile: writeFile(t, "jwt", "sa-jwt")}})

	for i := 0; i < 3; i++ {
		got, err := kv.Get(context.Background(), ref("a", "value", 0))
		require.NoError(t, err)
		assert.Equal(t, "sk", got)
	}
	assert.Equal(t, int32(1), f.logins.Load())
}

func TestKV_SendsNamespaceAndCustomMount(t *testing.T) {
	f := newFakeKV("tok")
	f.namespace = "team-a"
	f.put("a", map[string]any{"value": "sk"})
	srv := httptest.NewServer(f)
	defer srv.Close()
	kv := newTestKV(t, srv, Config{TokenFile: writeFile(t, "token", "tok"), Namespace: "team-a", Mount: "/secret/"})

	got, err := kv.Get(context.Background(), ref("a", "value", 0))
	require.NoError(t, err)
	assert.Equal(t, "sk", got)
}

func TestKV_RejectsWrongResolverAndBadPath(t *testing.T) {
	f := newFakeKV("tok")
	srv := httptest.NewServer(f)
	defer srv.Close()
	kv := newTestKV(t, srv, Config{TokenFile: writeFile(t, "token", "tok")})

	_, err := kv.Get(context.Background(), vault.Ref{Resolver: "other", Path: "a", Field: "value"})
	assert.Error(t, err)
	_, err = kv.Get(context.Background(), vault.Ref{Resolver: "amp", Path: "../sys/seal", Field: "value"})
	assert.Error(t, err)
	assert.Empty(t, f.requests, "invalid references never reach the server")
}

func TestKV_DoesNotFollowRedirects(t *testing.T) {
	var redirected atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected.Store(true)
	}))
	defer target.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()

	kv, err := New(Config{Name: "amp", Address: srv.URL, TokenFile: writeFile(t, "token", "tok")}, nil)
	require.NoError(t, err)
	_, err = kv.Get(context.Background(), ref("a", "value", 0))
	assert.Error(t, err)
	assert.False(t, redirected.Load(), "the token must never be sent to a redirect target")
}

func TestKV_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer srv.Close()
	kv := newTestKV(t, srv, Config{TokenFile: writeFile(t, "token", "tok"), RequestTimeout: 50 * time.Millisecond})

	start := time.Now()
	_, err := kv.Get(context.Background(), ref("a", "value", 0))
	assert.Error(t, err)
	assert.Less(t, time.Since(start), time.Second)
}

func TestNew_ValidatesConfig(t *testing.T) {
	tok := writeFile(t, "token", "tok")
	jwt := writeFile(t, "jwt", "j")
	tests := map[string]Config{
		"bad name":           {Name: "Bad", Address: "http://x:8200", TokenFile: tok},
		"no address":         {Name: "amp", TokenFile: tok},
		"bad scheme":         {Name: "amp", Address: "ftp://x", TokenFile: tok},
		"credentials in url": {Name: "amp", Address: "http://u:p@x:8200", TokenFile: tok},
		"bad mount":          {Name: "amp", Address: "http://x:8200", Mount: "a/../b", TokenFile: tok},
		"no auth":            {Name: "amp", Address: "http://x:8200"},
		"both auths": {Name: "amp", Address: "http://x:8200", TokenFile: tok,
			Kubernetes: &KubernetesAuthConfig{Role: "r", JWTFile: jwt}},
		"missing ca file": {Name: "amp", Address: "https://x:8200", TokenFile: tok, CAFile: "/nonexistent.pem"},
	}
	for name, cfg := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := New(cfg, nil)
			assert.Error(t, err)
		})
	}

	kv, err := New(Config{Name: "amp", Address: "https://openbao:8200", TokenFile: tok}, nil)
	require.NoError(t, err)
	assert.Equal(t, Type, kv.Type())
	assert.Equal(t, DefaultMount, kv.mount)
	assert.Equal(t, DefaultRequestTimeout, kv.timeout)
}
