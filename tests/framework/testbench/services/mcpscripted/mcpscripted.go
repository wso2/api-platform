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

// Package mcpscripted provides an MCP server whose tool results a scenario scripts, so a policy
// that reads tool results can be tested on every shape a server may send. The SDK-backed server
// in services/mcp answers only well-formed results; this one also answers JSON-RPC errors,
// non-2xx statuses, bodies that are not JSON-RPC, and server-sent events with progress
// notifications before the result.
//
// It is reached at http://testbench:3021/<partition>/mcp and records every request, so a
// scenario can assert whether a call reached the server: GET /<partition>/test/requests returns
// {"count": n, "requests": [{"method", "tool", "arguments"}, ...]}, oldest first.
//
// Every tool name is accepted. A tools/call's arguments script the reply:
//
//   - text: the text of the result (default "ok"). It can carry jev:<key>=<value> markers, so the
//     Jev answers follow the result a policy screens.
//   - repeat: how many times text is repeated, for a large result (default 1). The repeated text
//     may be at most MaxResultBytes.
//   - shape: what the server sends (default "text"):
//     text         a result with one text block
//     structured   a result with the text in structuredContent and no content blocks
//     resource     a result with the text in an embedded resource
//     image        a result with one image block and no text
//     rpc-error    HTTP 200 with a JSON-RPC error whose message is the text
//     http-error   HTTP 500 with the text as a plain-text body
//     not-jsonrpc  HTTP 200 with the text as a plain-text body
//     events       server-sent events: a text result, after two progress notifications when the
//     request carries params._meta.progressToken
package mcpscripted

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

// Port is the container port used by the testbench.
const Port = 3021

// Path is the MCP endpoint path, after the partition.
const Path = "/mcp"

// SessionID is the Mcp-Session-Id the server returns from initialize.
const SessionID = "mcp-scripted-session"

// Shapes a tools/call's "shape" argument selects.
const (
	ShapeText       = "text"
	ShapeStructured = "structured"
	ShapeResource   = "resource"
	ShapeImage      = "image"
	ShapeRPCError   = "rpc-error"
	ShapeHTTPError  = "http-error"
	ShapeNotJSONRPC = "not-jsonrpc"
	ShapeEvents     = "events"
)

var shapes = map[string]bool{
	ShapeText: true, ShapeStructured: true, ShapeResource: true, ShapeImage: true,
	ShapeRPCError: true, ShapeHTTPError: true, ShapeNotJSONRPC: true, ShapeEvents: true,
}

// MaxResultBytes bounds the text a call's arguments can ask for, after repeat, so one request
// cannot exhaust the shared testbench process.
const MaxResultBytes = 1 << 20

// maxPartitions bounds how many partitions the service keeps, like the jev service.
const maxPartitions = 4096

// maxRetainedRequests bounds the requests kept per partition; the count stays exact.
const maxRetainedRequests = 256

// onePixelPNG is the data of the image block the image shape returns.
const onePixelPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="

// Request is one recorded request to the MCP endpoint.
type Request struct {
	Method    string          `json:"method"`
	Tool      string          `json:"tool,omitempty"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

type partition struct {
	count    int
	requests []Request
}

// Service implements testbench.Service and testbench.Partitioned.
type Service struct {
	mu         sync.Mutex
	partitions map[string]*partition
}

// New returns a new scripted MCP service.
func New() *Service { return &Service{partitions: map[string]*partition{}} }

// Name returns the service registration name.
func (s *Service) Name() string { return "mcp-scripted" }

// Port returns the service's listening port.
func (s *Service) Port() int { return Port }

// Stateful reports that the service retains the requests it receives.
func (s *Service) Stateful() bool { return true }

// PartitionKey returns the partitioning strategy used by this stateful service.
func (s *Service) PartitionKey() string { return testbench.PartitionByBlock }

// Handler serves the partitioned routes.
func (s *Service) Handler() http.Handler {
	routes := http.NewServeMux()
	routes.HandleFunc("GET /test/requests", s.scoped(s.recorded))
	routes.HandleFunc("GET /test/health", s.scoped(s.health))
	routes.HandleFunc("POST "+Path, s.scoped(s.mcp))
	return testbench.NormalizeMethod(testbench.PartitionRouter(routes))
}

func (s *Service) scoped(fn func(string, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key, ok := testbench.PartitionKeyFromContext(r.Context())
		if !ok || key == "" {
			http.Error(w, "mcp-scripted: request has no partition", http.StatusInternalServerError)
			return
		}
		fn(key, w, r)
	}
}

// message is the part of a JSON-RPC request the server reads.
type message struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params struct {
		ProtocolVersion string          `json:"protocolVersion"`
		Name            string          `json:"name"`
		Arguments       json.RawMessage `json:"arguments"`
		Meta            struct {
			ProgressToken json.RawMessage `json:"progressToken"`
		} `json:"_meta"`
	} `json:"params"`
}

// script is the part of a tools/call's arguments that picks the reply.
type script struct {
	Text   *string `json:"text"`
	Repeat int     `json:"repeat"`
	Shape  string  `json:"shape"`
}

func (s *Service) mcp(key string, w http.ResponseWriter, r *http.Request) {
	var msg message
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		writeJSON(w, http.StatusBadRequest, rpcError(nil, -32700, "parse error"))
		return
	}
	recorded := Request{Method: msg.Method}
	if msg.Method == "tools/call" {
		recorded.Tool, recorded.Arguments = msg.Params.Name, msg.Params.Arguments
	}
	if !s.record(key, recorded) {
		http.Error(w, "mcp-scripted: too many partitions", http.StatusServiceUnavailable)
		return
	}

	switch {
	case strings.HasPrefix(msg.Method, "notifications/"):
		w.WriteHeader(http.StatusAccepted)
	case msg.Method == "initialize":
		version := msg.Params.ProtocolVersion
		if version == "" {
			version = "2025-06-18"
		}
		w.Header().Set("Mcp-Session-Id", SessionID)
		writeJSON(w, http.StatusOK, rpcResult(msg.ID, map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "mcp-scripted", "version": "1.0.0"},
		}))
	case msg.Method == "ping":
		writeJSON(w, http.StatusOK, rpcResult(msg.ID, map[string]any{}))
	case msg.Method == "tools/list":
		writeJSON(w, http.StatusOK, rpcResult(msg.ID, map[string]any{"tools": []map[string]any{{
			"name":        "respond",
			"description": "Replies with the result its arguments script. Every other tool name behaves the same.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
				"text":   map[string]any{"type": "string"},
				"repeat": map[string]any{"type": "integer"},
				"shape":  map[string]any{"type": "string"},
			}},
		}}}))
	case msg.Method == "tools/call":
		s.toolCall(w, msg)
	default:
		writeJSON(w, http.StatusOK, rpcError(msg.ID, -32601, "method not found: "+msg.Method))
	}
}

func (s *Service) toolCall(w http.ResponseWriter, msg message) {
	sc := script{}
	if len(msg.Params.Arguments) > 0 && string(msg.Params.Arguments) != "null" {
		if err := json.Unmarshal(msg.Params.Arguments, &sc); err != nil {
			writeJSON(w, http.StatusOK, rpcError(msg.ID, -32602, "arguments must be an object of text, repeat and shape"))
			return
		}
	}
	if sc.Shape == "" {
		sc.Shape = ShapeText
	}
	if !shapes[sc.Shape] {
		writeJSON(w, http.StatusOK, rpcError(msg.ID, -32602, "unknown shape: "+sc.Shape))
		return
	}
	text := "ok"
	if sc.Text != nil {
		text = *sc.Text
	}
	// Checked before repeating, by division, so the product can neither be allocated nor overflow.
	if sc.Repeat < 0 || len(text) > MaxResultBytes || (sc.Repeat > 1 && sc.Repeat > MaxResultBytes/max(len(text), 1)) {
		writeJSON(w, http.StatusOK, rpcError(msg.ID, -32602, fmt.Sprintf("repeat must not be negative, and text repeated must be at most %d bytes", MaxResultBytes)))
		return
	}
	if sc.Repeat > 1 {
		text = strings.Repeat(text, sc.Repeat)
	}

	textResult := map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
	switch sc.Shape {
	case ShapeText:
		writeJSON(w, http.StatusOK, rpcResult(msg.ID, textResult))
	case ShapeStructured:
		writeJSON(w, http.StatusOK, rpcResult(msg.ID, map[string]any{
			"content": []any{}, "structuredContent": map[string]any{"text": text},
		}))
	case ShapeResource:
		writeJSON(w, http.StatusOK, rpcResult(msg.ID, map[string]any{"content": []map[string]any{{
			"type": "resource", "resource": map[string]any{"uri": "test://result", "mimeType": "text/plain", "text": text},
		}}}))
	case ShapeImage:
		writeJSON(w, http.StatusOK, rpcResult(msg.ID, map[string]any{"content": []map[string]any{{
			"type": "image", "data": onePixelPNG, "mimeType": "image/png",
		}}}))
	case ShapeRPCError:
		writeJSON(w, http.StatusOK, rpcError(msg.ID, -32000, text))
	case ShapeHTTPError:
		writeText(w, http.StatusInternalServerError, text)
	case ShapeNotJSONRPC:
		writeText(w, http.StatusOK, text)
	case ShapeEvents:
		writeEvents(w, msg.ID, msg.Params.Meta.ProgressToken, textResult)
	}
}

// writeEvents sends the result as server-sent events. MCP allows progress notifications only for
// a token the request supplied, so the two that come first are sent only when it has one.
func writeEvents(w http.ResponseWriter, id, progressToken json.RawMessage, result map[string]any) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	var events []any
	if len(progressToken) > 0 && string(progressToken) != "null" {
		for progress := 1; progress <= 2; progress++ {
			events = append(events, map[string]any{"jsonrpc": "2.0", "method": "notifications/progress",
				"params": map[string]any{"progressToken": progressToken, "progress": progress, "total": 2}})
		}
	}
	events = append(events, rpcResult(id, result))
	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			log.Printf("mcp-scripted: encoding an event: %v", err)
			return
		}
		if _, err := fmt.Fprintf(w, "event: message\ndata: %s\n\n", data); err != nil {
			log.Printf("mcp-scripted: writing an event: %v", err)
			return
		}
	}
}

func rpcResult(id json.RawMessage, result any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": rawID(id), "result": result}
}

func rpcError(id json.RawMessage, code int, text string) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": rawID(id), "error": map[string]any{"code": code, "message": text}}
}

// rawID keeps the request's id as it was sent, and null when there was none.
func rawID(id json.RawMessage) json.RawMessage {
	if len(id) == 0 {
		return json.RawMessage("null")
	}
	return id
}

func (s *Service) record(key string, req Request) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.partitions[key]
	if p == nil {
		if len(s.partitions) >= maxPartitions {
			return false
		}
		p = &partition{}
		s.partitions[key] = p
	}
	p.count++
	if len(p.requests) < maxRetainedRequests {
		p.requests = append(p.requests, req)
	}
	return true
}

// recorded reports GET /test/requests as {"count": n, "requests": [...]}, oldest first.
func (s *Service) recorded(key string, w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	count, requests := 0, []Request{}
	if p := s.partitions[key]; p != nil {
		count = p.count
		requests = append(requests, p.requests...)
	}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"count": count, "requests": requests})
}

func (s *Service) health(_ string, w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "mcp-scripted"})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("mcp-scripted: encoding a response: %v", err)
	}
}

func writeText(w http.ResponseWriter, status int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write([]byte(text)); err != nil {
		log.Printf("mcp-scripted: writing a response: %v", err)
	}
}
