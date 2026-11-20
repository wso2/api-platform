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

package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// recordedRequest is one POST the CLI made to the test server.
type recordedRequest struct {
	method          string
	protocolVersion string
	sessionID       string
}

// newProtocolServer starts a stateless MCP server (it issues no session id) that
// answers initialize with the advertised version and accepts only the accepted
// version in MCP-Protocol-Version, as the streamable HTTP transport requires of a
// stateless server. The two differ only in the override test, where the caller
// pins a version the server did not advertise.
func newProtocolServer(t *testing.T, advertised string, accepted string) (*httptest.Server, func() []recordedRequest) {
	t.Helper()
	return newProtocolServerWithStream(t, advertised, accepted, false)
}

// newStreamingProtocolServer answers with an event stream that carries a logging
// notification before the response, which is what a streamable HTTP server is
// allowed to do. The notification has no id, so a client that reads the first data
// event reads the notification instead of the answer.
func newStreamingProtocolServer(t *testing.T, advertised string, accepted string) (*httptest.Server, func() []recordedRequest) {
	t.Helper()
	return newProtocolServerWithStream(t, advertised, accepted, true)
}

func newProtocolServerWithStream(t *testing.T, advertised string, accepted string, stream bool) (*httptest.Server, func() []recordedRequest) {
	t.Helper()

	var (
		mu       sync.Mutex
		recorded []recordedRequest
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("server received an undecodable request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		version := r.Header.Get("MCP-Protocol-Version")
		mu.Lock()
		recorded = append(recorded, recordedRequest{
			method:          req.Method,
			protocolVersion: version,
			sessionID:       r.Header.Get("mcp-session-id"),
		})
		mu.Unlock()

		if req.Method != MethodInitialize && version != accepted {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			message := "Unsupported protocol version: missing MCP-Protocol-Version"
			if version != "" {
				message = "Unsupported protocol version: " + version
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": JsonRpcVersion,
				"id":      json.RawMessage(req.ID),
				"error":   map[string]interface{}{"code": -32600, "message": message},
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if req.Method == MethodInitialized {
			w.WriteHeader(http.StatusAccepted)
			return
		}

		result := map[string]interface{}{}
		switch req.Method {
		case MethodInitialize:
			result = map[string]interface{}{
				"protocolVersion": advertised,
				"capabilities":    map[string]interface{}{},
				"serverInfo":      map[string]string{"name": "fixture", "version": "1.0.0"},
			}
		case MethodToolsList:
			result = map[string]interface{}{
				"tools": []map[string]interface{}{{
					"name":        "echo",
					"description": "Echoes its input",
					"inputSchema": map[string]interface{}{"type": "object"},
				}},
			}
		case MethodPromptsList:
			result = map[string]interface{}{"prompts": []interface{}{}}
		case MethodResourcesList:
			result = map[string]interface{}{"resources": []interface{}{}}
		}

		response := map[string]interface{}{
			"jsonrpc": JsonRpcVersion,
			"id":      json.RawMessage(req.ID),
			"result":  result,
		}
		if !stream {
			_ = json.NewEncoder(w).Encode(response)
			return
		}
		encoded, err := json.Marshal(response)
		if err != nil {
			t.Errorf("fixture could not encode the response: %v", err)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "event: message\ndata: %s\n\n", notificationEvent)
		fmt.Fprintf(w, "event: message\ndata: %s\n\n", encoded)
	}))
	t.Cleanup(server.Close)

	return server, func() []recordedRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]recordedRequest(nil), recorded...)
	}
}

// TestGenerateSendsNegotiatedProtocolVersion covers the stateless server case:
// no session id is issued, so the version header is the only way the server can
// tell which protocol it is speaking.
func TestGenerateSendsNegotiatedProtocolVersion(t *testing.T) {
	server, requests := newProtocolServer(t, ProtocolVersion, ProtocolVersion)

	if err := Generate(server.URL, t.TempDir(), "", ""); err != nil {
		t.Fatalf("Generate() failed against a server that requires %s: %v", ProtocolVersion, err)
	}

	seen := requests()
	if len(seen) != 5 {
		t.Fatalf("expected 5 requests, got %d: %+v", len(seen), seen)
	}
	for _, req := range seen {
		if req.method == MethodInitialize {
			if req.protocolVersion != "" {
				t.Errorf("initialize must not carry MCP-Protocol-Version, got %q", req.protocolVersion)
			}
			continue
		}
		if req.protocolVersion != ProtocolVersion {
			t.Errorf("%s carried MCP-Protocol-Version %q, want %q", req.method, req.protocolVersion, ProtocolVersion)
		}
	}
}

// TestGenerateUsesTheVersionTheServerNegotiates pins that the header follows the
// initialize response rather than the version the CLI proposes.
func TestGenerateUsesTheVersionTheServerNegotiates(t *testing.T) {
	const negotiated = "2025-11-25"
	server, requests := newProtocolServer(t, negotiated, negotiated)

	if err := Generate(server.URL, t.TempDir(), "", ""); err != nil {
		t.Fatalf("Generate() failed against a server that negotiated %s: %v", negotiated, err)
	}

	for _, req := range requests() {
		if req.method == MethodInitialize {
			continue
		}
		if req.protocolVersion != negotiated {
			t.Errorf("%s carried MCP-Protocol-Version %q, want the negotiated %q",
				req.method, req.protocolVersion, negotiated)
		}
	}
}

// TestGenerateKeepsCallerSuppliedProtocolVersion checks that an explicit --header
// still wins, which is how a caller pins a version the server did not advertise.
// The fixture advertises ProtocolVersion and accepts only pinned, so ignoring the
// caller's header fails the requests as well as the assertion.
func TestGenerateKeepsCallerSuppliedProtocolVersion(t *testing.T) {
	const pinned = "2025-03-26"
	server, requests := newProtocolServer(t, ProtocolVersion, pinned)

	if err := Generate(server.URL, t.TempDir(), "MCP-Protocol-Version", pinned); err != nil {
		t.Fatalf("Generate() failed with a caller-supplied protocol version: %v", err)
	}

	for _, req := range requests() {
		if req.method == MethodInitialize {
			continue
		}
		if req.protocolVersion != pinned {
			t.Errorf("%s carried MCP-Protocol-Version %q, want the caller's %q",
				req.method, req.protocolVersion, pinned)
		}
	}
}

// notificationEvent is a server-initiated message with no id: a logging
// notification or a progress update that may share the stream with a response.
const notificationEvent = `{"jsonrpc":"2.0","method":"notifications/message","params":{"level":"info","data":"starting"}}`

// TestGenerateSkipsNotificationsBeforeTheInitializeResponse pins that the
// negotiated version is read from the response and not from whatever event the
// stream happened to write first. A notification carries no result, so reading it
// loses the version and the later requests go out with the proposal, which this
// server rejects.
func TestGenerateSkipsNotificationsBeforeTheInitializeResponse(t *testing.T) {
	const negotiated = "2025-11-25"
	server, requests := newStreamingProtocolServer(t, negotiated, negotiated)

	if err := Generate(server.URL, t.TempDir(), "", ""); err != nil {
		t.Fatalf("Generate() failed against a stream that notifies first: %v", err)
	}

	for _, req := range requests() {
		if req.method == MethodInitialize {
			continue
		}
		if req.protocolVersion != negotiated {
			t.Errorf("%s carried MCP-Protocol-Version %q, want the negotiated %q",
				req.method, req.protocolVersion, negotiated)
		}
	}
}

// TestGenerateReadsTheToolsListResponseOnANotificationPrefixedStream covers the
// same trap one request later: the tool list has to come from the response, not
// from the notification that precedes it.
func TestGenerateReadsTheToolsListResponseOnANotificationPrefixedStream(t *testing.T) {
	server, _ := newStreamingProtocolServer(t, ProtocolVersion, ProtocolVersion)
	outputDir := t.TempDir()

	if err := Generate(server.URL, outputDir, "", ""); err != nil {
		t.Fatalf("Generate() failed against a stream that notifies first: %v", err)
	}

	generated, err := os.ReadFile(filepath.Join(outputDir, "generated-mcp.yaml"))
	if err != nil {
		t.Fatalf("could not read the generated configuration: %v", err)
	}
	if !strings.Contains(string(generated), "echo") {
		t.Errorf("the generated configuration lost the tool from the response:\n%s", generated)
	}
}

// TestParseEventStreamResponseMatchesTheRequestID covers the shapes an event
// stream is allowed to use: the space after data: is optional, and a JSON-RPC id
// may be a string, in which case it still has to match the request's own id.
func TestParseEventStreamResponseMatchesTheRequestID(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		id      string
		wantID  string
		wantRaw string
		wantErr bool
	}{
		{
			name:   "first event is the answer",
			body:   "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":2,\"result\":{}}\n\n",
			id:     "2",
			wantID: "2",
		},
		{
			name:   "notification comes first",
			body:   "data: " + notificationEvent + "\n\ndata: {\"jsonrpc\":\"2.0\",\"id\":2,\"result\":{}}\n\n",
			id:     "2",
			wantID: "2",
		},
		{
			name:   "no space after the colon",
			body:   "data:{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}\n\n",
			id:     "1",
			wantID: "1",
		},
		{
			name:   "string id",
			body:   "data: {\"jsonrpc\":\"2.0\",\"id\":\"abc\",\"result\":{}}\n\n",
			id:     "\"abc\"",
			wantID: "\"abc\"",
		},
		{
			name:   "data split over several lines",
			body:   "event: message\ndata: {\"jsonrpc\":\"2.0\",\ndata: \"id\":2,\"result\":{}}\n\n",
			id:     "2",
			wantID: "2",
		},
		{
			name:   "carriage returns",
			body:   "data: {\"jsonrpc\":\"2.0\",\"id\":3,\"result\":{}}\r\n\r\n",
			id:     "3",
			wantID: "3",
		},
		{
			name:    "no event answers the request",
			body:    "data: {\"jsonrpc\":\"2.0\",\"id\":2,\"result\":{}}\n\n",
			id:      "1",
			wantErr: true,
		},
		{
			name:    "no request id returns the first event",
			body:    "data: " + "{\"jsonrpc\":\"2.0\",\"result\":{\"a\":1}}" + "\n\n" + "data: " + "{\"jsonrpc\":\"2.0\",\"result\":{\"b\":2}}",
			wantRaw: "{\"jsonrpc\":\"2.0\",\"result\":{\"a\":1}}",
		},
		{
			name:    "a notification-only stream answers nothing",
			body:    "data: " + notificationEvent + "\n\n",
			id:      "1",
			wantErr: true,
		},
		{
			name:    "a server-to-client request is not the answer",
			body:    "data: " + "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"sampling/createMessage\"}" + "\n\n" + "data: " + "{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"a\":1}}",
			id:      "1",
			wantID:  "1",
			wantRaw: "{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"a\":1}}",
		},
		{
			name:    "no data event",
			body:    "event: ping\n\n",
			id:      "1",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseEventStreamResponse([]byte(tc.body), json.RawMessage(tc.id))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %s", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseEventStreamResponse() failed: %v", err)
			}
			var envelope struct {
				ID json.RawMessage `json:"id"`
			}
			if err := json.Unmarshal(got, &envelope); err != nil {
				t.Fatalf("the returned event is not JSON: %v", err)
			}
			if string(envelope.ID) != tc.wantID {
				t.Errorf("returned the event with id %s, want %s", envelope.ID, tc.wantID)
			}
			if tc.wantRaw != "" && string(got) != tc.wantRaw {
				t.Errorf("returned %s, want %s", got, tc.wantRaw)
			}
		})
	}
}

// TestGenerateFailsWhenInitializeFails pins that a failed initialize is an error
// and not a server that merely omitted the version: there is no session to carry
// a version for.
func TestGenerateFailsWhenInitializeFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "the gateway is down", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	err := Generate(server.URL, t.TempDir(), "", "")
	if err == nil {
		t.Fatal("Generate() succeeded against a server that answers 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("the error does not name the status: %v", err)
	}
}

// TestPostJSONRPCRejectsASuccessShapedBodyOnANon2xxStatus pins that a non-2xx
// answer is an error even when its body looks like a result: a 500 that returns
// an empty tools list must not become a generated configuration without tools.
func TestPostJSONRPCRejectsASuccessShapedBodyOnANon2xxStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"tools":[]}}`))
	}))
	t.Cleanup(server.Close)

	request := JsonRPCRequest{JSONRPC: JsonRpcVersion, ID: 2, Method: MethodToolsList}
	_, err := postJSONRPCWithSession(server.URL, request, mcpSession{}, "", "")
	if err == nil {
		t.Fatal("postJSONRPCWithSession() accepted a 500 whose body looked like a result")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("the error does not name the status: %v", err)
	}
}

// TestGenerateFailsWhenAFailedInitializeLooksSuccessful pins the order: a non-2xx
// answer is rejected before its body is read for a protocol version, so a 500 that
// happens to carry one cannot create a session.
func TestGenerateFailsWhenAFailedInitializeLooksSuccessful(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-11-25","capabilities":{}}}`))
	}))
	t.Cleanup(server.Close)

	if err := Generate(server.URL, t.TempDir(), "", ""); err == nil {
		t.Fatal("Generate() accepted a session from a 500 that carried a protocol version")
	}
}

// TestGenerateFailsWhenInitializeStreamsOnlyNotifications pins that a 2xx stream
// with no response does not become a session: without an initialize result there
// is no negotiated version to carry, and no tool list to discover.
func TestGenerateFailsWhenInitializeStreamsOnlyNotifications(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: %s\n\n", notificationEvent)
	}))
	t.Cleanup(server.Close)

	if err := Generate(server.URL, t.TempDir(), "", ""); err == nil {
		t.Fatal("Generate() created a session from a stream that carried no response")
	}
}

// TestGenerateFailsWhenInitializeIsNotAResponse pins that a decodable 2xx body
// without a result is not a handshake: a notification or an empty object carries
// no server capabilities, so there is no session to build on.
func TestGenerateFailsWhenInitializeIsNotAResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","method":"notifications/message"}`))
	}))
	t.Cleanup(server.Close)

	if err := Generate(server.URL, t.TempDir(), "", ""); err == nil {
		t.Fatal("Generate() created a session from a body that carried no result")
	}
}

// TestGenerateReportsAnErrorFromAFailedEventStream checks that a non-2xx answer
// written as an event stream still reports the server's own reason, which is the
// only diagnosis it gives.
func TestGenerateReportsAnErrorFromAFailedEventStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "data: {\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-32600,\"message\":\"unsupported version 2025-06-18\"}}\n\n")
	}))
	t.Cleanup(server.Close)

	err := Generate(server.URL, t.TempDir(), "", "")
	if err == nil {
		t.Fatal("Generate() accepted a failed event stream")
	}
	if !strings.Contains(err.Error(), "unsupported version") {
		t.Errorf("the error does not carry the server's reason: %v", err)
	}
}

// TestGenerateFailsWhenInitializeIsUnreadable pins that a 2xx body that is not a
// JSON-RPC response fails the handshake instead of creating a session on a
// version nobody agreed to.
func TestGenerateFailsWhenInitializeIsUnreadable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("this is not the JSON you are looking for"))
	}))
	t.Cleanup(server.Close)

	if err := Generate(server.URL, t.TempDir(), "", ""); err == nil {
		t.Fatal("Generate() created a session from a body that was not JSON")
	}
}

// TestGenerateFailsWhenAResponseAnswersAnotherRequest pins the id in both plain
// JSON paths: a body cannot be taken for the answer to a request it does not name.
func TestGenerateFailsWhenAResponseAnswersAnotherRequest(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		status int
	}{
		{
			name:   "initialize answers id 2",
			body:   `{"jsonrpc":"2.0","id":2,"result":{"protocolVersion":"2025-06-18","capabilities":{}}}`,
			status: http.StatusOK,
		},
		{
			name:   "initialize reports a version that is not a string",
			body:   `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":123,"capabilities":{}}}`,
			status: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(server.Close)

			if err := Generate(server.URL, t.TempDir(), "", ""); err == nil {
				t.Fatal("Generate() accepted a response that does not answer the request")
			}
		})
	}
}

// TestPostJSONRPCRejectsAnAnswerToAnotherRequest covers the same rule after
// initialize, where a tool list from another request would otherwise be written
// into the configuration.
func TestPostJSONRPCRejectsAnAnswerToAnotherRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":9,"result":{"tools":[]}}`))
	}))
	t.Cleanup(server.Close)

	request := JsonRPCRequest{JSONRPC: JsonRpcVersion, ID: 2, Method: MethodToolsList}
	if _, err := postJSONRPCWithSession(server.URL, request, mcpSession{}, "", ""); err == nil {
		t.Fatal("postJSONRPCWithSession() accepted a response to another request")
	}
}
