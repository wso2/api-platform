/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package utils

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
)

// stubRequest is what the stub records of one call: enough to assert the request was shaped for
// the revision it declared, which a method name alone cannot show.
type stubRequest struct {
	method          string
	mcpMethodHeader string
	protocolVersion string
	sessionID       string
	metaKeys        []string
}

// mcpStub is a fake MCP server that answers per JSON-RPC method, so a test can describe a server
// by which methods it implements. It records the requests it was sent, in order.
type mcpStub struct {
	mu       sync.Mutex
	requests []stubRequest
	handlers map[string]func(w http.ResponseWriter)
}

func newMCPStub() *mcpStub {
	return &mcpStub{handlers: map[string]func(w http.ResponseWriter){}}
}

func (s *mcpStub) on(method string, h func(w http.ResponseWriter)) *mcpStub {
	s.handlers[method] = h
	return s
}

// methodNotFound makes the stub answer a method the way a server that does not implement it does.
func (s *mcpStub) methodNotFound(method string) *mcpStub {
	return s.on(method, func(w http.ResponseWriter) {
		writeJSON(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"Method not found"}}`)
	})
}

func (s *mcpStub) start(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				Meta map[string]json.RawMessage `json:"_meta"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		recorded := stubRequest{
			method:          req.Method,
			mcpMethodHeader: r.Header.Get(McpMethodHeader),
			protocolVersion: r.Header.Get(McpProtocolVersionHeader),
			sessionID:       r.Header.Get(McpSessionHeader),
		}
		for key := range req.Params.Meta {
			recorded.metaKeys = append(recorded.metaKeys, key)
		}
		sort.Strings(recorded.metaKeys)

		s.mu.Lock()
		s.requests = append(s.requests, recorded)
		s.mu.Unlock()

		if h, ok := s.handlers[req.Method]; ok {
			h(w)
			return
		}
		// Anything unhandled answers as an empty successful result.
		writeJSON(w, `{"jsonrpc":"2.0","id":1,"result":{}}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func (s *mcpStub) called() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	methods := make([]string, 0, len(s.requests))
	for _, req := range s.requests {
		methods = append(methods, req.method)
	}
	return methods
}

// recorded returns the request the stub was sent for a method, and whether it was sent at all.
func (s *mcpStub) recorded(method string) (stubRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, req := range s.requests {
		if req.method == method {
			return req, true
		}
	}
	return stubRequest{}, false
}

func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}

// A 2026-07-28 server implements server/discover and no initialize at all.
func TestFetchMCPServerInfo_ModernServer(t *testing.T) {
	stub := newMCPStub().
		on(MethodServerDiscover, func(w http.ResponseWriter) {
			writeJSON(w, `{"jsonrpc":"2.0","id":1,"result":{
				"supportedVersions":["2025-06-18","2026-07-28"],
				"_meta":{"io.modelcontextprotocol/serverInfo":{"name":"modern","version":"2.0"}}}}`)
		}).
		methodNotFound(MethodInitialize).
		on(MethodToolsList, func(w http.ResponseWriter) {
			writeJSON(w, `{"jsonrpc":"2.0","id":2,"result":{"resultType":"complete","ttlMs":60000,
				"tools":[{"name":"get_weather"}]}}`)
		})
	srv := stub.start(t)

	resp, err := FetchMCPServerInfo(srv.URL, "", "")
	if err != nil {
		t.Fatalf("modern server should be reachable: %v", err)
	}

	if resp.SupportedVersions == nil {
		t.Fatal("SupportedVersions is nil, want the set server/discover reported")
	}
	got := *resp.SupportedVersions
	if len(got) != 2 || got[0] != "2025-06-18" || got[1] != "2026-07-28" {
		t.Errorf("SupportedVersions = %v, want [2025-06-18 2026-07-28]", got)
	}

	// 2026-07-28 carries the identity in _meta rather than in the result body, so reading it
	// from the wrong place yields a nil ServerInfo and an unnamed server in the portal.
	if resp.ServerInfo == nil || (*resp.ServerInfo)["name"] != "modern" {
		t.Errorf("ServerInfo = %v, want the identity carried in _meta", resp.ServerInfo)
	}

	// The modern envelope carries resultType and ttlMs beside the tools; the catalogue must
	// still parse, since the decoder ignores members it does not name.
	if resp.Tools == nil || len(*resp.Tools) != 1 {
		t.Errorf("Tools = %v, want the one tool the modern result carried", resp.Tools)
	}

	for _, m := range stub.called() {
		if m == MethodInitialize {
			t.Error("initialize was called on a server that answered server/discover")
		}
	}
}

// A pre-2026-07-28 server has no server/discover, so the probe falls back to the handshake.
func TestFetchMCPServerInfo_LegacyServer(t *testing.T) {
	stub := newMCPStub().
		methodNotFound(MethodServerDiscover).
		on(MethodInitialize, func(w http.ResponseWriter) {
			w.Header().Set("mcp-session-id", "sess-1")
			writeJSON(w, `{"jsonrpc":"2.0","id":1,"result":{
				"protocolVersion":"2025-06-18",
				"serverInfo":{"name":"legacy","version":"1.0"}}}`)
		})
	srv := stub.start(t)

	resp, err := FetchMCPServerInfo(srv.URL, "", "")
	if err != nil {
		t.Fatalf("legacy server should be reachable: %v", err)
	}
	if resp.SupportedVersions == nil {
		t.Fatal("SupportedVersions is nil, want the version initialize negotiated")
	}
	if got := *resp.SupportedVersions; len(got) != 1 || got[0] != "2025-06-18" {
		t.Errorf("SupportedVersions = %v, want [2025-06-18]", got)
	}

	called := stub.called()
	if len(called) < 2 || called[0] != MethodServerDiscover || called[1] != MethodInitialize {
		t.Errorf("call order = %v, want server/discover tried before initialize", called)
	}
}

// A server with no route for server/discover may answer 404 rather than a JSON-RPC error. That is
// still a definitive "no such method" and must fall back.
func TestFetchMCPServerInfo_DiscoverNotRouted(t *testing.T) {
	stub := newMCPStub().
		on(MethodServerDiscover, func(w http.ResponseWriter) { w.WriteHeader(http.StatusNotFound) }).
		on(MethodInitialize, func(w http.ResponseWriter) {
			writeJSON(w, `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-11-25"}}`)
		})
	srv := stub.start(t)

	resp, err := FetchMCPServerInfo(srv.URL, "", "")
	if err != nil {
		t.Fatalf("a 404 on server/discover should fall back, got: %v", err)
	}
	if got := *resp.SupportedVersions; len(got) != 1 || got[0] != "2025-11-25" {
		t.Errorf("SupportedVersions = %v, want [2025-11-25]", got)
	}
}

// A session-bearing legacy server rejects anything preceding initialize with a status AND a
// JSON-RPC body. The body is what names the server's own reason, so it is read before the
// status is judged.
func TestFetchMCPServerInfo_DiscoverRefusedWithStatusAndJSONRPCBody(t *testing.T) {
	stub := newMCPStub().
		on(MethodServerDiscover, func(w http.ResponseWriter) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"Server not initialized"}}`))
		}).
		on(MethodInitialize, func(w http.ResponseWriter) {
			writeJSON(w, `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-11-25"}}`)
		})
	srv := stub.start(t)

	resp, err := FetchMCPServerInfo(srv.URL, "", "")
	if err != nil {
		t.Fatalf("a 400 carrying a JSON-RPC error should fall back, got: %v", err)
	}
	if got := *resp.SupportedVersions; len(got) != 1 || got[0] != "2025-11-25" {
		t.Errorf("SupportedVersions = %v, want [2025-11-25]", got)
	}
	called := stub.called()
	if len(called) < 2 || called[0] != MethodServerDiscover || called[1] != MethodInitialize {
		t.Errorf("call order = %v, want server/discover tried before initialize", called)
	}
}

// A server can reject an unknown method before any JSON-RPC layer sees it, answering a bare 400
// with plain text or HTML. That is still the server refusing the method, so the handshake has to
// follow: treating it as a hard failure makes a perfectly reachable legacy server unusable.
func TestFetchMCPServerInfo_DiscoverRefusedWithPlainTextStatus(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusNotAcceptable, http.StatusUnsupportedMediaType} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			stub := newMCPStub().
				on(MethodServerDiscover, func(w http.ResponseWriter) {
					w.Header().Set("Content-Type", "text/plain")
					w.WriteHeader(status)
					_, _ = w.Write([]byte("Bad Request: unknown method"))
				}).
				on(MethodInitialize, func(w http.ResponseWriter) {
					w.Header().Set(McpSessionHeader, "sess-1")
					writeJSON(w, `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18"}}`)
				})
			srv := stub.start(t)

			resp, err := FetchMCPServerInfo(srv.URL, "", "")
			if err != nil {
				t.Fatalf("a plain-text %d should fall back to initialize, got: %v", status, err)
			}
			called := stub.called()
			if len(called) < 2 || called[0] != MethodServerDiscover || called[1] != MethodInitialize {
				t.Fatalf("call order = %v, want server/discover then initialize", called)
			}
			if resp.SupportedVersions == nil || (*resp.SupportedVersions)[0] != "2025-06-18" {
				t.Errorf("SupportedVersions = %v, want the handshake's revision", resp.SupportedVersions)
			}
		})
	}
}

// A refusal of the caller is not evidence about the method. Retrying the same credentials through
// initialize would fail the same way while replacing a precise error with a vaguer one, so these
// propagate instead of triggering the fallback.
//
// The body shape is varied because a JSON-RPC one is what a rate limiter in front of the server
// sends - this platform's own mcp-ratelimit policy answers 429 with a JSON-RPC envelope - and it
// must not be read as the server naming its own reason for not serving the method.
func TestFetchMCPServerInfo_DiscoverRefusingTheCallerDoesNotFallBack(t *testing.T) {
	bodies := map[string]struct {
		contentType string
		body        string
	}{
		"plain text": {"text/plain", "nope"},
		"JSON-RPC":   {"application/json", `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"refused"}}`},
	}

	for _, status := range []int{http.StatusForbidden, http.StatusTooManyRequests} {
		for name, shape := range bodies {
			t.Run(http.StatusText(status)+" with a "+name+" body", func(t *testing.T) {
				stub := newMCPStub().
					on(MethodServerDiscover, func(w http.ResponseWriter) {
						w.Header().Set("Content-Type", shape.contentType)
						w.WriteHeader(status)
						_, _ = w.Write([]byte(shape.body))
					}).
					on(MethodInitialize, func(w http.ResponseWriter) {
						writeJSON(w, `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18"}}`)
					})
				srv := stub.start(t)

				if _, err := FetchMCPServerInfo(srv.URL, "", ""); err == nil {
					t.Fatalf("a %d should surface, not be read as an absent method", status)
				}
				if called := stub.called(); len(called) != 1 || called[0] != MethodServerDiscover {
					t.Errorf("calls = %v, want server/discover alone", called)
				}
			})
		}
	}
}

// Any JSON-RPC error means the server is not serving 2026-07-28, not only -32601.
func TestFetchMCPServerInfo_DiscoverRefusedWithNonMethodNotFoundCode(t *testing.T) {
	stub := newMCPStub().
		on(MethodServerDiscover, func(w http.ResponseWriter) {
			writeJSON(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32602,"message":"Invalid params"}}`)
		}).
		on(MethodInitialize, func(w http.ResponseWriter) {
			writeJSON(w, `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18"}}`)
		})
	srv := stub.start(t)

	resp, err := FetchMCPServerInfo(srv.URL, "", "")
	if err != nil {
		t.Fatalf("a -32602 on server/discover should fall back, got: %v", err)
	}
	if got := *resp.SupportedVersions; len(got) != 1 || got[0] != "2025-06-18" {
		t.Errorf("SupportedVersions = %v, want [2025-06-18]", got)
	}
}

// A transient failure establishes nothing about the server's era, so the probe fails rather than
// downgrading to the handshake.
func TestFetchMCPServerInfo_DiscoverFailsTransientlyDoesNotFallBack(t *testing.T) {
	stub := newMCPStub().
		on(MethodServerDiscover, func(w http.ResponseWriter) { w.WriteHeader(http.StatusBadGateway) }).
		on(MethodInitialize, func(w http.ResponseWriter) {
			writeJSON(w, `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18"}}`)
		})
	srv := stub.start(t)

	if _, err := FetchMCPServerInfo(srv.URL, "", ""); err == nil {
		t.Fatal("a 502 on server/discover must fail the probe, not downgrade to the handshake")
	}

	for _, m := range stub.called() {
		if m == MethodInitialize {
			t.Error("fell back to initialize after a transient server/discover failure")
		}
	}
}

// A 5xx can come from an intermediary that never reached the server, so a JSON-RPC body in one
// is not the server refusing and must not downgrade the probe.
func TestFetchMCPServerInfo_TransientFailureWithJSONRPCBodyDoesNotFallBack(t *testing.T) {
	stub := newMCPStub().
		on(MethodServerDiscover, func(w http.ResponseWriter) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"upstream unavailable"}}`))
		}).
		on(MethodInitialize, func(w http.ResponseWriter) {
			writeJSON(w, `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18"}}`)
		})
	srv := stub.start(t)

	if _, err := FetchMCPServerInfo(srv.URL, "", ""); err == nil {
		t.Fatal("a 502 must fail the probe whatever its body contains")
	}
	for _, m := range stub.called() {
		if m == MethodInitialize {
			t.Error("fell back to initialize after a 502")
		}
	}
}

// A server that reports no versions is read successfully and simply yields none.
func TestFetchMCPServerInfo_NoVersionsReported(t *testing.T) {
	stub := newMCPStub().
		on(MethodServerDiscover, func(w http.ResponseWriter) {
			writeJSON(w, `{"jsonrpc":"2.0","id":1,"result":{
				"_meta":{"io.modelcontextprotocol/serverInfo":{"name":"quiet"}}}}`)
		})
	srv := stub.start(t)

	resp, err := FetchMCPServerInfo(srv.URL, "", "")
	if err != nil {
		t.Fatalf("a server reporting no versions is still reachable: %v", err)
	}
	if resp.SupportedVersions != nil {
		t.Errorf("SupportedVersions = %v, want nil when the server reported none", *resp.SupportedVersions)
	}
	if resp.ServerInfo == nil {
		t.Error("ServerInfo is nil, want what the server did report")
	}
}

// A 2026-07-28 server rejects a request that omits the per-request envelope with -32602, and one
// that omits the mirrored method header with -32020, so every modern call must carry both.
func TestFetchMCPServerInfo_ModernRequestsCarryEnvelopeAndMirroredMethod(t *testing.T) {
	stub := newMCPStub().
		on(MethodServerDiscover, func(w http.ResponseWriter) {
			writeJSON(w, `{"jsonrpc":"2.0","id":1,"result":{
				"supportedVersions":["2026-07-28"],
				"_meta":{"io.modelcontextprotocol/serverInfo":{"name":"modern"}}}}`)
		}).
		on(MethodToolsList, func(w http.ResponseWriter) {
			writeJSON(w, `{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"t"}]}}`)
		})
	srv := stub.start(t)

	if _, err := FetchMCPServerInfo(srv.URL, "", ""); err != nil {
		t.Fatalf("modern server should be reachable: %v", err)
	}

	wantMeta := []string{
		"io.modelcontextprotocol/clientCapabilities",
		"io.modelcontextprotocol/clientInfo",
		"io.modelcontextprotocol/protocolVersion",
	}
	for _, method := range []string{MethodServerDiscover, MethodToolsList, MethodResourcesList} {
		req, ok := stub.recorded(method)
		if !ok {
			t.Errorf("%s was never sent", method)
			continue
		}
		if req.mcpMethodHeader != method {
			t.Errorf("%s: Mcp-Method = %q, want %q", method, req.mcpMethodHeader, method)
		}
		if req.protocolVersion != SpecVersion20260728 {
			t.Errorf("%s: MCP-Protocol-Version = %q, want %s", method, req.protocolVersion, SpecVersion20260728)
		}
		if strings.Join(req.metaKeys, ",") != strings.Join(wantMeta, ",") {
			t.Errorf("%s: params._meta keys = %v, want %v", method, req.metaKeys, wantMeta)
		}
	}
}

// A server that negotiated a pre-2026-07-28 revision mirrors nothing and defines no envelope, so
// sending either would be a request it has no rules for.
func TestFetchMCPServerInfo_LegacyRequestsCarryNeither(t *testing.T) {
	stub := newMCPStub().
		methodNotFound(MethodServerDiscover).
		on(MethodInitialize, func(w http.ResponseWriter) {
			writeJSON(w, `{"jsonrpc":"2.0","id":1,"result":{
				"protocolVersion":"2025-06-18","serverInfo":{"name":"legacy"}}}`)
		})
	srv := stub.start(t)

	if _, err := FetchMCPServerInfo(srv.URL, "", ""); err != nil {
		t.Fatalf("legacy server should be reachable: %v", err)
	}

	req, ok := stub.recorded(MethodToolsList)
	if !ok {
		t.Fatal("tools/list was never sent")
	}
	if req.mcpMethodHeader != "" {
		t.Errorf("Mcp-Method = %q on a legacy call, want it absent", req.mcpMethodHeader)
	}
	if len(req.metaKeys) != 0 {
		t.Errorf("params._meta = %v on a legacy call, want it absent", req.metaKeys)
	}
	// It does carry the negotiated revision: the spec has an HTTP client send the version the
	// handshake settled on with every later request in that session.
	if req.protocolVersion != "2025-06-18" {
		t.Errorf("MCP-Protocol-Version = %q, want the negotiated 2025-06-18", req.protocolVersion)
	}
}

// After server/discover the client addresses the server under a revision it reported, not under
// the one that happened to answer the discovery call.
func TestFetchMCPServerInfo_NegotiatesTheVersionTheServerReported(t *testing.T) {
	stub := newMCPStub().
		on(MethodServerDiscover, func(w http.ResponseWriter) {
			writeJSON(w, `{"jsonrpc":"2.0","id":1,"result":{"supportedVersions":["2025-11-25"]}}`)
		})
	srv := stub.start(t)

	if _, err := FetchMCPServerInfo(srv.URL, "", ""); err != nil {
		t.Fatalf("server should be reachable: %v", err)
	}

	req, ok := stub.recorded(MethodToolsList)
	if !ok {
		t.Fatal("tools/list was never sent")
	}
	if req.protocolVersion != "2025-11-25" {
		t.Errorf("MCP-Protocol-Version = %q, want the reported 2025-11-25", req.protocolVersion)
	}
	if len(req.metaKeys) != 0 {
		t.Errorf("params._meta = %v, want it absent below 2026-07-28", req.metaKeys)
	}
}

// A server can answer server/discover and still report a set whose highest shared revision
// predates 2026-07-28. Those revisions make the initialize lifecycle mandatory, so the handshake
// has to run for its session: without one a stateful server rejects every catalogue call, and the
// fetch would return success with nothing in it.
func TestFetchMCPServerInfo_LegacyNegotiationAfterDiscoverStillInitializes(t *testing.T) {
	stub := newMCPStub().
		on(MethodServerDiscover, func(w http.ResponseWriter) {
			writeJSON(w, `{"jsonrpc":"2.0","id":1,"result":{"supportedVersions":["2025-06-18","2025-11-25"]}}`)
		}).
		on(MethodInitialize, func(w http.ResponseWriter) {
			w.Header().Set(McpSessionHeader, "sess-legacy")
			// initialize negotiates one revision, and a narrower one than discovery reported.
			writeJSON(w, `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18"}}`)
		})
	srv := stub.start(t)

	resp, err := FetchMCPServerInfo(srv.URL, "", "")
	if err != nil {
		t.Fatalf("server should be reachable: %v", err)
	}

	called := stub.called()
	if len(called) < 2 || called[0] != MethodServerDiscover || called[1] != MethodInitialize {
		t.Fatalf("call order = %v, want server/discover then initialize", called)
	}

	req, ok := stub.recorded(MethodToolsList)
	if !ok {
		t.Fatal("tools/list was never sent")
	}
	if req.sessionID != "sess-legacy" {
		t.Errorf("tools/list mcp-session-id = %q, want the session initialize established", req.sessionID)
	}
	// The handshake settles the revision for the session, so its answer outranks the one
	// discovery advertised: the stub reported 2025-11-25 but initialize negotiated 2025-06-18,
	// and a strict server rejects a header naming a revision it never agreed to.
	if req.protocolVersion != "2025-06-18" {
		t.Errorf("MCP-Protocol-Version = %q, want the handshake's 2025-06-18", req.protocolVersion)
	}

	// The notification closing the handshake belongs to that same session and carries it too.
	notified, ok := stub.recorded(MethodInitialized)
	if !ok {
		t.Fatal("notifications/initialized was never sent")
	}
	if notified.protocolVersion != "2025-06-18" {
		t.Errorf("notifications/initialized MCP-Protocol-Version = %q, want 2025-06-18",
			notified.protocolVersion)
	}
	if notified.sessionID != "sess-legacy" {
		t.Errorf("notifications/initialized mcp-session-id = %q, want the handshake's session",
			notified.sessionID)
	}

	// What discovery reported outranks what initialize negotiated: the handshake is here for the
	// session, not to narrow the server's own answer about which revisions it serves.
	if resp.SupportedVersions == nil {
		t.Fatal("supportedVersions is absent, want what discovery reported")
	}
	if got := *resp.SupportedVersions; len(got) != 2 || got[0] != "2025-06-18" || got[1] != "2025-11-25" {
		t.Errorf("supportedVersions = %v, want both revisions discovery reported", got)
	}
}

// A revision this client does not implement must never be negotiated: it would ride in the
// MCP-Protocol-Version header while _meta still declared 2026-07-28, so the one request would
// state two revisions and a conformant server would reject it.
func TestFetchMCPServerInfo_NegotiatesOnlyARevisionThisClientImplements(t *testing.T) {
	t.Run("a newer-only server is refused rather than addressed wrongly", func(t *testing.T) {
		stub := newMCPStub().
			on(MethodServerDiscover, func(w http.ResponseWriter) {
				writeJSON(w, `{"jsonrpc":"2.0","id":1,"result":{"supportedVersions":["2027-01-01"]}}`)
			})
		srv := stub.start(t)

		_, err := FetchMCPServerInfo(srv.URL, "", "")
		if !errors.Is(err, errNoSharedSpecVersion) {
			t.Fatalf("err = %v, want errNoSharedSpecVersion", err)
		}
		if _, sent := stub.recorded(MethodToolsList); sent {
			t.Error("tools/list must not be sent under a revision this client cannot speak")
		}
	})

	t.Run("the highest shared revision is taken, not the highest reported", func(t *testing.T) {
		stub := newMCPStub().
			on(MethodServerDiscover, func(w http.ResponseWriter) {
				writeJSON(w, `{"jsonrpc":"2.0","id":1,"result":{"supportedVersions":["2027-01-01","2025-11-25"]}}`)
			})
		srv := stub.start(t)

		if _, err := FetchMCPServerInfo(srv.URL, "", ""); err != nil {
			t.Fatalf("server should be reachable: %v", err)
		}
		req, ok := stub.recorded(MethodToolsList)
		if !ok {
			t.Fatal("tools/list was never sent")
		}
		if req.protocolVersion != "2025-11-25" {
			t.Errorf("MCP-Protocol-Version = %q, want the highest shared revision 2025-11-25", req.protocolVersion)
		}
	})
}

// Reporting nothing is not evidence of nothing shared: some SDKs fill supportedVersions from
// their modern set only, so a dual-era server under-reports.
func TestNegotiateVersion_NoReportedVersionsMeansModern(t *testing.T) {
	got, err := negotiateVersion(nil)
	if err != nil || got != SpecVersion20260728 {
		t.Fatalf("negotiateVersion(nil) = %q, %v, want %q and no error", got, err, SpecVersion20260728)
	}
}

// The response records what the server named, whatever this platform or a gateway serves: an
// older revision is a fact about that server, and dropping it would make the stored set a claim
// about us instead. Only a value that is not a revision date at all is discarded. Negotiation is
// a separate question - addressing the server uses a revision this client can actually speak.
func TestFetchMCPServerInfo_ReportsEveryRevisionTheServerNamed(t *testing.T) {
	t.Run("every revision is kept, including older and unknown ones", func(t *testing.T) {
		stub := newMCPStub().
			on(MethodServerDiscover, func(w http.ResponseWriter) {
				writeJSON(w, `{"jsonrpc":"2.0","id":1,"result":{"supportedVersions":
					["2024-11-05","2025-03-26","2025-06-18","2025-11-25","2026-07-28","2027-03-01","banana"]}}`)
			})
		srv := stub.start(t)

		resp, err := FetchMCPServerInfo(srv.URL, "", "")
		if err != nil {
			t.Fatalf("server should be reachable: %v", err)
		}
		if resp.SupportedVersions == nil {
			t.Fatal("SupportedVersions is nil, want every revision the server named")
		}
		// "banana" is not a revision date, so it is the one value dropped.
		want := []string{"2024-11-05", "2025-03-26", "2025-06-18", "2025-11-25", "2026-07-28", "2027-03-01"}
		if got := *resp.SupportedVersions; !slices.Equal(got, want) {
			t.Errorf("SupportedVersions = %v, want %v", got, want)
		}
		req, ok := stub.recorded(MethodToolsList)
		if !ok {
			t.Fatal("tools/list was never sent")
		}
		if req.protocolVersion != SpecVersion20260728 {
			t.Errorf("MCP-Protocol-Version = %q, want the highest revision this client implements",
				req.protocolVersion)
		}
	})

	t.Run("a legacy server's own revision is reported, old as it is", func(t *testing.T) {
		stub := newMCPStub().
			methodNotFound(MethodServerDiscover).
			on(MethodInitialize, func(w http.ResponseWriter) {
				w.Header().Set("mcp-session-id", "sess-1")
				writeJSON(w, `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26"}}`)
			})
		srv := stub.start(t)

		resp, err := FetchMCPServerInfo(srv.URL, "", "")
		if err != nil {
			t.Fatalf("legacy server should be reachable: %v", err)
		}
		if resp.SupportedVersions == nil {
			t.Fatal("SupportedVersions is nil, want the revision the handshake negotiated")
		}
		if got := *resp.SupportedVersions; !slices.Equal(got, []string{"2025-03-26"}) {
			t.Errorf("SupportedVersions = %v, want [2025-03-26]", got)
		}
	})
}
