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

package faultinjection

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

func serve(t *testing.T, svc *Service, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	svc.Handler().ServeHTTP(rec, req)
	return rec
}

func storeScript(t *testing.T, svc *Service, block, id, body string) {
	t.Helper()
	rec := serve(t, svc, http.MethodPut, "/"+block+"/test/scripts/"+id, body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, 0, decodeState(t, rec).Received)
}

func received(t *testing.T, svc *Service, block, id string) int {
	t.Helper()
	rec := serve(t, svc, http.MethodGet, "/"+block+"/test/scripts/"+id, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	state := decodeState(t, rec)
	require.Equal(t, id, state.ID)
	return state.Received
}

func decodeState(t *testing.T, rec *httptest.ResponseRecorder) scriptState {
	t.Helper()
	var state scriptState
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &state))
	return state
}

// unprobed returns a service that skips the forwardPort probe, for tests of the script rules alone.
func unprobed() *Service {
	svc := New()
	svc.probe = func(int) error { return nil }
	return svc
}

// target starts a stand-in testbench service that echoes what it received, and returns its port.
func target(t *testing.T) int {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"method": r.Method, "path": r.URL.Path, "query": r.URL.RawQuery, "host": r.Host,
			"key": r.Header.Get("Ocp-Apim-Subscription-Key"), "body": string(body),
			"forwardedFor": r.Header.Get("X-Forwarded-For"), "headers": r.Header, "rawPath": r.URL.RawPath,
		})
	}))
	t.Cleanup(server.Close)
	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	require.NoError(t, err)
	n, err := strconv.Atoi(port)
	require.NoError(t, err)
	return n
}

func TestServiceContract(t *testing.T) {
	svc := New()
	require.Equal(t, "fault-injection", svc.Name())
	require.Equal(t, Port, svc.Port())
	require.True(t, svc.Stateful())
	require.Equal(t, testbench.PartitionByBlock, svc.PartitionKey())
	require.NoError(t, (&testbench.Registry{}).Register(svc))
}

func TestStepsAnswerInOrderThenRequestsAreForwarded(t *testing.T) {
	svc := New()
	port := target(t)
	storeScript(t, svc, "blk", "acs_retry_1",
		fmt.Sprintf(`{"forwardPort":%d,"steps":[{"status":500},{"status":429}]}`, port))

	rec := serve(t, svc, http.MethodPost, "/blk/acs_retry_1/analyze", `{"text":"hi"}`)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, rec.Body.String(), "scripted status 500")

	rec = serve(t, svc, http.MethodPost, "/blk/acs_retry_1/analyze", `{"text":"hi"}`)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)

	for range 2 {
		rec = serve(t, svc, http.MethodPost, "/blk/acs_retry_1/analyze", `{"text":"hi"}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
	require.Equal(t, 4, received(t, svc, "blk", "acs_retry_1"), "failures and forwards are both counted")
}

func TestForwardingKeepsTheRequestAndNamesTheTarget(t *testing.T) {
	svc := New()
	port := target(t)
	storeScript(t, svc, "blk", "fwd", fmt.Sprintf(`{"forwardPort":%d,"steps":[]}`, port))

	req := httptest.NewRequest(http.MethodPost,
		"/blk/fwd/contentsafety/text:analyze?api-version=2024-09-01", strings.NewReader(`{"text":"hi"}`))
	req.Header.Set("Ocp-Apim-Subscription-Key", "test-subscription-key")
	rec := httptest.NewRecorder()
	svc.Handler().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var got map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, http.MethodPost, got["method"])
	require.Equal(t, "/contentsafety/text:analyze", got["path"])
	require.Equal(t, "api-version=2024-09-01", got["query"])
	require.Equal(t, "test-subscription-key", got["key"])
	require.Equal(t, `{"text":"hi"}`, got["body"])
	require.Equal(t, net.JoinHostPort(forwardHost, strconv.Itoa(port)), got["host"])
	require.Empty(t, got["forwardedFor"], "forwarding adds no proxy headers")
}

func TestForwardingDropsProxyAndHopByHopHeadersAndDecodesThePath(t *testing.T) {
	svc := New()
	port := target(t)
	storeScript(t, svc, "blk", "fwd", fmt.Sprintf(`{"forwardPort":%d,"steps":[]}`, port))

	req := httptest.NewRequest(http.MethodGet, "/blk/fwd/a%2Fb?ok=1&bad=%zz", nil)
	req.Header.Set("X-End-To-End", "kept")
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	req.Header.Set("X-Forwarded-Host", "client.example")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("Forwarded", "for=203.0.113.7")
	req.Header.Set("X-Forwarded-Port", "443")
	req.Header.Set("X-Forwarded-Prefix", "/api")
	req.Header.Set("Connection", "X-Hop")
	req.Header.Set("X-Hop", "dropped")
	req.Header.Set("Keep-Alive", "timeout=5")
	rec := httptest.NewRecorder()
	svc.Handler().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var got struct {
		Path    string              `json:"path"`
		RawPath string              `json:"rawPath"`
		Query   string              `json:"query"`
		Headers map[string][]string `json:"headers"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, "/a/b", got.Path, "an encoded slash reaches the target decoded")
	require.Empty(t, got.RawPath)
	require.Equal(t, "ok=1", got.Query, "an unparsable query parameter is dropped")
	require.Equal(t, []string{"kept"}, got.Headers["X-End-To-End"])
	for _, name := range []string{"X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "X-Forwarded-Port",
		"X-Forwarded-Prefix", "Forwarded",
		"Connection", "X-Hop", "Keep-Alive"} {
		require.NotContains(t, got.Headers, name)
	}
}

func TestStoreRejectsAForwardPortThatCannotBeForwardedTo(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	closed := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())

	tlsServer := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(tlsServer.Close)
	tlsPort := tlsServer.Listener.Addr().(*net.TCPAddr).Port

	for name, port := range map[string]int{"nothing listens": closed, "serves TLS": tlsPort} {
		rec := serve(t, New(), http.MethodPut, "/blk/test/scripts/bad", fmt.Sprintf(`{"forwardPort":%d}`, port))
		require.Equal(t, http.StatusBadRequest, rec.Code, name)
		require.Contains(t, rec.Body.String(), strconv.Itoa(port), name)
	}
}

func TestProbeIsNotSeenByThePlainTarget(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	t.Cleanup(server.Close)
	storeScript(t, New(), "blk", "plain", fmt.Sprintf(`{"forwardPort":%d}`, server.Listener.Addr().(*net.TCPAddr).Port))
	require.Zero(t, calls.Load(), "the probe reaches no handler")
}

func TestForwardingToAClosedPortAnswersBadGateway(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	port := server.Listener.Addr().(*net.TCPAddr).Port
	svc := New()
	storeScript(t, svc, "blk", "closed", fmt.Sprintf(`{"forwardPort":%d,"steps":[]}`, port))
	server.Close() // the target goes away after the script is stored
	rec := serve(t, svc, http.MethodGet, "/blk/closed/anything", "")
	require.Equal(t, http.StatusBadGateway, rec.Code)
}

func TestDelayStepWaitsThenForwards(t *testing.T) {
	svc := New()
	port := target(t)
	storeScript(t, svc, "blk", "slow", fmt.Sprintf(`{"forwardPort":%d,"steps":[{"delayMs":50}]}`, port))

	started := time.Now()
	rec := serve(t, svc, http.MethodGet, "/blk/slow/anything", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.GreaterOrEqual(t, time.Since(started), 50*time.Millisecond)
	var got map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, "/anything", got["path"], "the delayed request reaches the target")
}

func TestDelayStepStopsWhenTheCallerGoesAway(t *testing.T) {
	svc := New()
	port := target(t)
	storeScript(t, svc, "blk", "hang",
		fmt.Sprintf(`{"forwardPort":%d,"steps":[{"delayMs":%d}]}`, port, maxDelay.Milliseconds()))

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/blk/hang/anything", nil).WithContext(ctx)
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
		t.Fatal("the delay kept waiting after the request context ended")
	}
	require.Empty(t, rec.Body.String(), "a request whose caller left is not forwarded")
}

func TestStoringAScriptReplacesItAndResetsTheCount(t *testing.T) {
	svc := New()
	port := target(t)
	storeScript(t, svc, "blk", "again", fmt.Sprintf(`{"forwardPort":%d,"steps":[{"status":503}]}`, port))
	serve(t, svc, http.MethodGet, "/blk/again/x", "")
	serve(t, svc, http.MethodGet, "/blk/again/x", "")
	require.Equal(t, 2, received(t, svc, "blk", "again"))

	storeScript(t, svc, "blk", "again", fmt.Sprintf(`{"forwardPort":%d,"steps":[{"status":502}]}`, port))
	require.Equal(t, 0, received(t, svc, "blk", "again"))
	require.Equal(t, http.StatusBadGateway, serve(t, svc, http.MethodGet, "/blk/again/x", "").Code)
}

func TestRequestsWithoutAScriptAreNotFoundAndNotCounted(t *testing.T) {
	svc := New()
	rec := serve(t, svc, http.MethodPost, "/blk/unscripted/analyze", `{}`)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), `no script for "unscripted"`)

	rec = serve(t, svc, http.MethodGet, "/blk/test/scripts/unscripted", "")
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestScriptsAreIsolatedPerBlockAndID(t *testing.T) {
	svc := New()
	port := target(t)
	storeScript(t, svc, "blk-a", "shared", fmt.Sprintf(`{"forwardPort":%d,"steps":[{"status":500}]}`, port))
	storeScript(t, svc, "blk-b", "shared", fmt.Sprintf(`{"forwardPort":%d,"steps":[]}`, port))

	require.Equal(t, http.StatusInternalServerError, serve(t, svc, http.MethodGet, "/blk-a/shared/x", "").Code)
	require.Equal(t, http.StatusOK, serve(t, svc, http.MethodGet, "/blk-b/shared/x", "").Code,
		"another block's script never answers")
	require.Equal(t, 1, received(t, svc, "blk-a", "shared"))
	require.Equal(t, 1, received(t, svc, "blk-b", "shared"))

	require.Equal(t, http.StatusNotFound, serve(t, svc, http.MethodGet, "/blk-a/other/x", "").Code,
		"another id in the same block has no script")
	require.Equal(t, http.StatusNotFound, serve(t, svc, http.MethodGet, "/blk-c/test/scripts/shared", "").Code)
}

func TestConcurrentRequestsTakeEachStepExactlyOnce(t *testing.T) {
	svc := New()
	port := target(t)
	steps := make([]string, maxStepsPerScript)
	for i := range steps {
		steps[i] = `{"status":500}`
	}
	storeScript(t, svc, "blk", "busy",
		fmt.Sprintf(`{"forwardPort":%d,"steps":[%s]}`, port, strings.Join(steps, ",")))

	const workers, each = 8, 6
	var mu sync.Mutex
	codes := map[int]int{}
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for range each {
				code := serve(t, svc, http.MethodGet, "/blk/busy/x", "").Code
				mu.Lock()
				codes[code]++
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	require.Equal(t, maxStepsPerScript, codes[http.StatusInternalServerError])
	require.Equal(t, workers*each-maxStepsPerScript, codes[http.StatusOK])
	require.Equal(t, workers*each, received(t, svc, "blk", "busy"))
}

func TestStoreRejectsAnInvalidScript(t *testing.T) {
	tooMany := make([]string, maxStepsPerScript+1)
	for i := range tooMany {
		tooMany[i] = `{"status":500}`
	}
	tests := []struct {
		name string
		body string
		want int
	}{
		{name: "empty body", body: "", want: http.StatusBadRequest},
		{name: "malformed JSON", body: `{"forwardPort":`, want: http.StatusBadRequest},
		{name: "two documents", body: `{"forwardPort":3005}{}`, want: http.StatusBadRequest},
		{name: "unknown field", body: `{"forwardPort":3005,"steps":[{"delay":5}]}`, want: http.StatusBadRequest},
		{name: "missing forwardPort", body: `{"steps":[]}`, want: http.StatusBadRequest},
		{name: "forwardPort zero", body: `{"forwardPort":0}`, want: http.StatusBadRequest},
		{name: "forwardPort above range", body: `{"forwardPort":65536}`, want: http.StatusBadRequest},
		{name: "forwardPort is this service", body: fmt.Sprintf(`{"forwardPort":%d}`, Port), want: http.StatusBadRequest},
		{name: "empty step", body: `{"forwardPort":3005,"steps":[{}]}`, want: http.StatusBadRequest},
		{name: "status and delay", body: `{"forwardPort":3005,"steps":[{"status":500,"delayMs":5}]}`, want: http.StatusBadRequest},
		{name: "status below range", body: `{"forwardPort":3005,"steps":[{"status":399}]}`, want: http.StatusBadRequest},
		{name: "status above range", body: `{"forwardPort":3005,"steps":[{"status":600}]}`, want: http.StatusBadRequest},
		{name: "negative delay", body: `{"forwardPort":3005,"steps":[{"delayMs":-1}]}`, want: http.StatusBadRequest},
		{name: "delay above cap", body: fmt.Sprintf(`{"forwardPort":3005,"steps":[{"delayMs":%d}]}`,
			maxDelay.Milliseconds()+1), want: http.StatusBadRequest},
		{name: "too many steps", body: fmt.Sprintf(`{"forwardPort":3005,"steps":[%s]}`, strings.Join(tooMany, ",")),
			want: http.StatusBadRequest},
		{name: "oversized body", body: `{"forwardPort":3005,"steps":[` + strings.Repeat(" ", maxScriptBodyBytes) + `]}`,
			want: http.StatusRequestEntityTooLarge},
		{name: "delay that overflows a duration", body: `{"forwardPort":3005,"steps":[{"delayMs":9223372036855}]}`,
			want: http.StatusBadRequest},
		{name: "oversized trailing whitespace", body: `{"forwardPort":3005}` + strings.Repeat(" ", maxScriptBodyBytes),
			want: http.StatusRequestEntityTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// unprobed, so a 400 comes from the script rules and not from the port probe.
			svc := unprobed()
			rec := serve(t, svc, http.MethodPut, "/blk/test/scripts/s1", tt.body)
			require.Equal(t, tt.want, rec.Code, rec.Body.String())
			require.Equal(t, http.StatusNotFound, serve(t, svc, http.MethodGet, "/blk/test/scripts/s1", "").Code,
				"a rejected script is not stored")
		})
	}
}

func TestScriptBoundariesAreAccepted(t *testing.T) {
	steps := make([]string, maxStepsPerScript)
	for i := range steps {
		steps[i] = `{"status":599}`
	}
	steps[0] = `{"status":400}`
	steps[1] = fmt.Sprintf(`{"delayMs":%d}`, maxDelay.Milliseconds())
	steps[2] = `{"delayMs":1}`
	for _, body := range []string{
		`{"forwardPort":1}`,
		`{"forwardPort":65535,"steps":[]}`,
		fmt.Sprintf(`{"forwardPort":3005,"steps":[%s]}`, strings.Join(steps, ",")),
	} {
		storeScript(t, unprobed(), "blk", "edge", body)
	}
}

func TestScriptIDsAreValidated(t *testing.T) {
	longest := strings.Repeat("a", maxScriptIDLen)
	storeScript(t, unprobed(), "blk", longest, `{"forwardPort":3005}`)
	storeScript(t, unprobed(), "blk", "Mixed_Case-1", `{"forwardPort":3005}`)

	for _, id := range []string{"test", strings.Repeat("a", maxScriptIDLen+1), "bad.id", "bad%20id"} {
		svc := New()
		require.Equal(t, http.StatusBadRequest,
			serve(t, svc, http.MethodPut, "/blk/test/scripts/"+id, `{"forwardPort":3005}`).Code, id)
		require.Equal(t, http.StatusBadRequest, serve(t, svc, http.MethodGet, "/blk/test/scripts/"+id, "").Code, id)
		require.Equal(t, http.StatusBadRequest, serve(t, svc, http.MethodGet, "/blk/"+id+"/x", "").Code, id)
	}
	require.Equal(t, http.StatusBadRequest, serve(t, New(), http.MethodGet, "/blk/", "").Code, "an empty id")
	require.Equal(t, http.StatusBadRequest, serve(t, New(), http.MethodGet, "/", "").Code, "a missing partition")
}

func TestScriptBudgetIsBounded(t *testing.T) {
	svc := unprobed()
	doc := scriptDocument{ForwardPort: 3005}
	for i := range maxScriptsPerPartition {
		require.True(t, svc.store("blk", fmt.Sprintf("s%d", i), doc))
	}
	require.False(t, svc.store("blk", "one-too-many", doc))
	require.True(t, svc.store("blk", "s0", doc), "an existing script can be replaced at the cap")
	require.True(t, svc.store("other", "s0", doc), "the cap is per block")

	rec := serve(t, svc, http.MethodPut, "/blk/test/scripts/one-too-many", `{"forwardPort":3005}`)
	require.Equal(t, http.StatusInsufficientStorage, rec.Code)
}

func TestStoredStepsDoNotAliasTheDocument(t *testing.T) {
	svc := New()
	doc := scriptDocument{ForwardPort: 3005, Steps: []step{{Status: 500}}}
	require.True(t, svc.store("blk", "copy", doc))
	doc.Steps[0].Status = 503
	next, _, ok := svc.take("blk", "copy")
	require.True(t, ok)
	require.Equal(t, 500, next.Status)
}
