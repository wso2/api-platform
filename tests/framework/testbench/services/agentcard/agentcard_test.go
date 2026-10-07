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

package agentcard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

func serve(t *testing.T, svc *Service, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()
	svc.Handler().ServeHTTP(rec, req)
	return rec
}

func requestCount(t *testing.T, svc *Service, block, scope string) int {
	t.Helper()
	rec := serve(t, svc, http.MethodGet, "/"+block+"/test/requests?scope="+scope, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Scope string `json:"scope"`
		Count int    `json:"count"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, scope, body.Scope)
	return body.Count
}

func TestServiceContract(t *testing.T) {
	svc := New()
	require.Equal(t, "agentcard", svc.Name())
	require.Equal(t, Port, svc.Port())
	require.True(t, svc.Stateful())
	require.Equal(t, testbench.PartitionByBlock, svc.PartitionKey())
	require.NoError(t, (&testbench.Registry{}).Register(svc))
}

func TestModes(t *testing.T) {
	svc := newWithSlowDelay(20 * time.Millisecond)
	tests := []struct {
		name        string
		path        string
		headers     map[string]string
		wantStatus  int
		wantName    string
		wantNonJSON bool
	}{
		{name: "ok", path: "/blk/s1/ok/.well-known/agent-card.json", wantStatus: http.StatusOK, wantName: "Card s1"},
		{name: "custom card path", path: "/blk/s1/ok/cards/public.json", wantStatus: http.StatusOK, wantName: "Card s1"},
		{name: "alternate", path: "/blk/s1/alternate/.well-known/agent-card.json", wantStatus: http.StatusOK, wantName: "Alternate card s1"},
		{name: "auth accepted", path: "/blk/s1/auth/.well-known/agent-card.json",
			headers: map[string]string{CredentialHeader: Credential("s1")}, wantStatus: http.StatusOK, wantName: "Protected source card s1"},
		{name: "auth missing", path: "/blk/s1/auth/.well-known/agent-card.json", wantStatus: http.StatusUnauthorized},
		{name: "auth wrong scope credential", path: "/blk/s1/auth/.well-known/agent-card.json",
			headers: map[string]string{CredentialHeader: Credential("s2")}, wantStatus: http.StatusUnauthorized},
		{name: "malformed", path: "/blk/s1/malformed/.well-known/agent-card.json", wantStatus: http.StatusOK, wantNonJSON: true},
		{name: "slow", path: "/blk/s1/slow/.well-known/agent-card.json", wantStatus: http.StatusOK, wantName: "Slow card s1"},
		{name: "server error", path: "/blk/s1/status500/.well-known/agent-card.json", wantStatus: http.StatusInternalServerError},
		{name: "unknown mode", path: "/blk/s1/teleport/.well-known/agent-card.json", wantStatus: http.StatusNotFound},
		{name: "not a card path", path: "/blk/s1/ok/health", wantStatus: http.StatusNotFound},
		{name: "missing mode", path: "/blk/s1/agent-card.json", wantStatus: http.StatusNotFound},
		{name: "invalid scope", path: "/blk/S_1/ok/.well-known/agent-card.json", wantStatus: http.StatusBadRequest},
		{name: "missing partition", path: "/", wantStatus: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(t, svc, http.MethodGet, tt.path, tt.headers)
			require.Equal(t, tt.wantStatus, rec.Code, rec.Body.String())
			if tt.wantNonJSON {
				var doc map[string]any
				require.Error(t, json.Unmarshal(rec.Body.Bytes(), &doc))
				return
			}
			if tt.wantName == "" {
				return
			}
			var card map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &card))
			require.Equal(t, tt.wantName, card["name"])
			for _, key := range []string{"name", "description", "version", "supportedInterfaces",
				"capabilities", "defaultInputModes", "defaultOutputModes", "skills"} {
				require.Contains(t, card, key)
			}
		})
	}
}

func TestEveryCardRequestIsCountedPerScopeAndBlock(t *testing.T) {
	svc := New()
	require.Equal(t, 0, requestCount(t, svc, "blk", "fresh"))

	serve(t, svc, http.MethodGet, "/blk/counted/ok/.well-known/agent-card.json", nil)
	serve(t, svc, http.MethodGet, "/blk/counted/auth/.well-known/agent-card.json", nil)
	serve(t, svc, http.MethodGet, "/blk/counted/status500/.well-known/agent-card.json", nil)
	require.Equal(t, 3, requestCount(t, svc, "blk", "counted"), "failures count as upstream contacts too")

	require.Equal(t, 0, requestCount(t, svc, "other", "counted"), "blocks never share counters")
	require.Equal(t, 0, requestCount(t, svc, "blk", "unrelated"), "scopes never share counters")

	// A request rejected for its path never reached a card, so it is not counted.
	serve(t, svc, http.MethodGet, "/blk/counted/ok/health", nil)
	require.Equal(t, 3, requestCount(t, svc, "blk", "counted"))
}

func TestRequestsRejectsAnInvalidScope(t *testing.T) {
	svc := New()
	for _, query := range []string{"", "?scope=", "?scope=Upper", fmt.Sprintf("?scope=%065d", 0)} {
		rec := serve(t, svc, http.MethodGet, "/blk/test/requests"+query, nil)
		require.Equal(t, http.StatusBadRequest, rec.Code, query)
	}
}

func TestScopeBudgetIsBounded(t *testing.T) {
	svc := New()
	for i := range maxScopesPerPartition {
		require.True(t, svc.count("blk", fmt.Sprintf("s%d", i)))
	}
	require.False(t, svc.count("blk", "one-too-many"))
	require.True(t, svc.count("blk", "s0"), "an existing scope keeps counting at the cap")
	require.True(t, svc.count("other", "s0"), "the cap is per block")
}

func TestSlowModeStopsWhenTheCallerGoesAway(t *testing.T) {
	svc := newWithSlowDelay(time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/blk/s1/slow/.well-known/agent-card.json", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		svc.Handler().ServeHTTP(rec, req)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("slow mode kept waiting after the request context ended")
	}
	require.Empty(t, rec.Body.String())
}

func TestConcurrentRequestsAreCountedExactly(t *testing.T) {
	svc := New()
	const workers, each = 8, 25
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for range each {
				serve(t, svc, http.MethodGet, "/blk/busy/ok/.well-known/agent-card.json", nil)
			}
		})
	}
	wg.Wait()
	require.Equal(t, workers*each, requestCount(t, svc, "blk", "busy"))
}
