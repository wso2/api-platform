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
	"net/http"
	"net/http/httptest"
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

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": JsonRpcVersion,
			"id":      json.RawMessage(req.ID),
			"result":  result,
		})
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
