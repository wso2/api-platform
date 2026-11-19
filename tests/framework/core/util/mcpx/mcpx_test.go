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

package mcpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

type addInput struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
}

// newToolServer serves an add tool, a tool that reports its own failure, and a tool that waits
// until its caller gives up.
func newToolServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "mcpx-test", Version: "1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "add", Description: "Adds two numbers"},
		func(_ context.Context, _ *mcp.CallToolRequest, in addInput) (*mcp.CallToolResult, any, error) {
			text := fmt.Sprintf("The sum of %v and %v is %v.", in.A, in.B, in.A+in.B)
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: "fail", Description: "Always fails"},
		func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "it failed"}}}, nil, nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: "wait", Description: "Waits until the call is cancelled"},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
			select {
			case <-ctx.Done():
			case <-time.After(2 * time.Second):
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "done"}}}, nil, nil
		})
	return server
}

// newDiscoverServer answers server/discover and keeps no session.
func newDiscoverServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := newToolServer()
	return startServer(t, mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true}))
}

// newLegacyServer refuses server/discover and issues session ids, as a server that predates MCP
// 2026-07-28 does, so a client has to fall back to initialize.
func newLegacyServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := newToolServer()
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	return startServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		var envelope struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if json.Unmarshal(body, &envelope) == nil && envelope.Method == "server/discover" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":` + string(envelope.ID) +
				`,"error":{"code":-32601,"message":"Method not found"}}`))
			return
		}
		handler.ServeHTTP(w, r)
	}))
}

func startServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func connect(t *testing.T, endpoint string, opts Options) *Session {
	t.Helper()
	session, err := Connect(context.Background(), endpoint, opts)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func addArguments() map[string]any { return map[string]any{"a": 1, "b": 2} }

func TestConnectValidatesItsInputs(t *testing.T) {
	for _, tc := range []struct {
		name, endpoint string
		opts           Options
		want           string
	}{
		{name: "empty endpoint", endpoint: "", want: "endpoint URL is required"},
		{name: "blank endpoint", endpoint: "   ", want: "endpoint URL is required"},
		{name: "relative endpoint", endpoint: "/mcp", want: "absolute http or https URL"},
		{name: "unsupported scheme", endpoint: "ftp://example.test/mcp", want: "absolute http or https URL"},
		{name: "missing host", endpoint: "http:///mcp", want: "absolute http or https URL"},
		{name: "malformed endpoint", endpoint: "http://exa mple.test/mcp", want: "endpoint URL"},
		{name: "negative timeout", endpoint: "http://example.test/mcp", opts: Options{CallTimeout: -time.Second},
			want: "must not be negative"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			session, err := Connect(context.Background(), tc.endpoint, tc.opts)
			require.Nil(t, session)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestConnectFallsBackToInitializeWhenServerDiscoverIsRefused(t *testing.T) {
	session := connect(t, newLegacyServer(t).URL+"/mcp", Options{})

	require.Equal(t, []string{"server/discover", "initialize", "notifications/initialized"}, session.Handshake())
	require.Equal(t, "2025-11-25", session.ProtocolVersion())
	require.NotEmpty(t, session.ID())

	result, err := session.CallTool(context.Background(), "add", addArguments())
	require.NoError(t, err)
	require.Equal(t, ToolResult{Text: "The sum of 1 and 2 is 3."}, result)
}

func TestConnectUsesServerDiscoverWhenTheServerAnswersIt(t *testing.T) {
	session := connect(t, newDiscoverServer(t).URL+"/mcp", Options{})

	require.Equal(t, []string{"server/discover"}, session.Handshake())
	require.Equal(t, "2026-07-28", session.ProtocolVersion())
	require.Empty(t, session.ID())

	result, err := session.CallTool(context.Background(), "add", addArguments())
	require.NoError(t, err)
	require.Equal(t, "The sum of 1 and 2 is 3.", result.Text)
}

func TestConnectReportsARefusedEndpoint(t *testing.T) {
	server := startServer(t, http.NotFoundHandler())
	session, err := Connect(context.Background(), server.URL+"/mcp", Options{})
	require.Nil(t, session)
	require.ErrorContains(t, err, "mcpx: connecting to "+server.URL+"/mcp")
}

func TestHandshakeIsACopy(t *testing.T) {
	session := connect(t, newDiscoverServer(t).URL+"/mcp", Options{})
	handshake := session.Handshake()
	handshake[0] = "changed"
	require.Equal(t, []string{"server/discover"}, session.Handshake())
}

func TestRequestsCarryTheCallerHeadersButKeepTheProtocolHeaders(t *testing.T) {
	type seenRequest struct {
		header       http.Header
		host, method string
	}
	upstream := newDiscoverServer(t)
	var mu sync.Mutex
	var seen []seenRequest
	recorder := startServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		var envelope struct {
			Method string `json:"method"`
		}
		_ = json.Unmarshal(body, &envelope)
		mu.Lock()
		seen = append(seen, seenRequest{header: r.Header.Clone(), host: r.Host, method: envelope.Method})
		mu.Unlock()
		forward(w, r, upstream.URL)
	}))

	headers := map[string]string{"Authorization": "Basic YWRtaW46YWRtaW4=", "Content-Type": "text/plain"}
	session := connect(t, recorder.URL+"/mcp", Options{Headers: headers, Host: "mcp.example.test"})
	headers["Authorization"] = "changed after Connect"
	_, err := session.CallTool(context.Background(), "add", addArguments())
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, seen, 2)
	require.Equal(t, "server/discover", seen[0].method)
	require.Equal(t, "tools/call", seen[1].method)
	for _, req := range seen {
		require.Equal(t, "Basic YWRtaW46YWRtaW4=", req.header.Get("Authorization"))
		require.Equal(t, "application/json", req.header.Get("Content-Type"))
		require.Equal(t, "mcp.example.test", req.host)
	}
}

func TestCallToolValidatesItsInputs(t *testing.T) {
	var nilSession *Session
	_, err := nilSession.CallTool(context.Background(), "add", addArguments())
	require.ErrorContains(t, err, "session is nil")

	session := connect(t, newDiscoverServer(t).URL+"/mcp", Options{})
	_, err = session.CallTool(context.Background(), " ", addArguments())
	require.ErrorContains(t, err, "tool name is required")

	require.NoError(t, session.Close())
	_, err = session.CallTool(context.Background(), "add", addArguments())
	require.ErrorIs(t, err, ErrSessionClosed)
}

func TestCallToolReportsFailures(t *testing.T) {
	session := connect(t, newDiscoverServer(t).URL+"/mcp", Options{})

	result, err := session.CallTool(context.Background(), "fail", nil)
	require.NoError(t, err)
	require.Equal(t, ToolResult{Text: "it failed", IsError: true}, result)

	_, err = session.CallTool(context.Background(), "missing", nil)
	require.ErrorContains(t, err, `mcpx: calling tool "missing"`)
}

func TestCallToolHonoursTheCallTimeout(t *testing.T) {
	session := connect(t, newDiscoverServer(t).URL+"/mcp", Options{CallTimeout: 200 * time.Millisecond})
	start := time.Now()
	_, err := session.CallTool(context.Background(), "wait", nil)
	require.Error(t, err)
	require.Less(t, time.Since(start), 1500*time.Millisecond)
}

func TestOneSessionServesConcurrentCalls(t *testing.T) {
	session := connect(t, newDiscoverServer(t).URL+"/mcp", Options{})
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := session.CallTool(context.Background(), "add", map[string]any{"a": i, "b": 1})
			if err == nil && result.Text != fmt.Sprintf("The sum of %d and 1 is %d.", i, i+1) {
				err = fmt.Errorf("call %d returned %q", i, result.Text)
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}

func TestNilAndClosedSessionsAreSafe(t *testing.T) {
	var nilSession *Session
	require.Nil(t, nilSession.Handshake())
	require.Empty(t, nilSession.ProtocolVersion())
	require.Empty(t, nilSession.ID())
	require.NoError(t, nilSession.Close())

	session := connect(t, newLegacyServer(t).URL+"/mcp", Options{})
	require.NoError(t, session.Close())
	require.NoError(t, session.Close())
}

func TestJSONRPCMethod(t *testing.T) {
	newRequest := func(method string, body io.Reader) *http.Request {
		req, err := http.NewRequest(method, "http://example.test/mcp", body)
		require.NoError(t, err)
		return req
	}

	req := newRequest(http.MethodPost, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call"}`))
	require.Equal(t, "tools/call", jsonRPCMethod(req))
	sent, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Contains(t, string(sent), "tools/call", "the body that is sent must stay unread")

	require.Empty(t, jsonRPCMethod(newRequest(http.MethodDelete, nil)))
	require.Empty(t, jsonRPCMethod(newRequest(http.MethodPost, strings.NewReader(`{"jsonrpc":`))))
	require.Empty(t, jsonRPCMethod(newRequest(http.MethodPost, strings.NewReader(`{"jsonrpc":"2.0","id":1}`))))
	require.Empty(t, jsonRPCMethod(nil))

	failing := newRequest(http.MethodPost, strings.NewReader(`{}`))
	failing.GetBody = func() (io.ReadCloser, error) { return nil, errors.New("no body") }
	require.Empty(t, jsonRPCMethod(failing))
}

func TestSentMethodsIsSafeForConcurrentUse(t *testing.T) {
	recorded := &sentMethods{}
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			recorded.add("ping")
			_ = recorded.list()
		}()
	}
	wg.Wait()
	require.Len(t, recorded.list(), 50)
}

func TestCloneHeaders(t *testing.T) {
	require.Nil(t, cloneHeaders(nil))
	require.Nil(t, cloneHeaders(map[string]string{}))
	original := map[string]string{"A": "1"}
	cloned := cloneHeaders(original)
	original["A"] = "2"
	require.Equal(t, map[string]string{"A": "1"}, cloned)
}

// forward passes a request on to the upstream and copies its answer back.
func forward(w http.ResponseWriter, r *http.Request, upstream string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	proxy, err := http.NewRequestWithContext(r.Context(), r.Method, upstream+r.URL.Path, bytes.NewReader(body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	proxy.Header = r.Header.Clone()
	resp, err := http.DefaultClient.Do(proxy)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	for name, values := range resp.Header {
		w.Header()[name] = values
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
