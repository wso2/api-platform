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

// This file adds a second MCP server that speaks only the handshake era, because the service in
// mcp.go cannot: the SDK it wraps implements every revision up to 2026-07-28, so a test pointed
// at it can never exercise what the gateway and the control plane do when the server behind them
// is older. Two behaviours are unreachable without this one:
//
//   - a session. mcp.go runs stateless and mints no Mcp-Session-Id at all, so nothing proves the
//     gateway carries one from an initialize response into the requests that follow.
//   - the discovery fallback. mcp.go answers server/discover, so the control plane's initialize
//     path - the larger half of that code - never runs against a real server.
//
// It shares newServer(), so both eras expose the same two tools and a test can attribute a
// difference in outcome to the era alone.
//
// It is stateful, which the shared testbench allows only for a service that isolates its state
// by block: sessions live in a per-block handler and are addressed as /<block>/mcp. Without that
// isolation one block's handshake could answer another block's request.

package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

// LegacyPort is the container port used by the testbench for the handshake-era server.
const LegacyPort = 3013

// methodServerDiscover is the RPC 2026-07-28 introduced, which this server does not implement.
const methodServerDiscover = "server/discover"

// LegacyService is a handshake-era MCP server: stateful, and without server/discover.
type LegacyService struct {
	// mu guards the partition map.
	mu sync.Mutex

	partitions map[string]http.Handler
}

// NewLegacy builds an MCP service that behaves as a server predating 2026-07-28.
func NewLegacy() *LegacyService {
	return &LegacyService{partitions: map[string]http.Handler{}}
}

// Name returns the service registration name.
func (s *LegacyService) Name() string { return "mcp-legacy" }

// Port returns the service's listening port.
func (s *LegacyService) Port() int { return LegacyPort }

// Stateful reports whether the service keeps request-specific state. It does, and that is the
// point of it: a session opened by one request is read by the next.
func (s *LegacyService) Stateful() bool { return true }

// PartitionKey returns the partitioning strategy used by this stateful service.
func (s *LegacyService) PartitionKey() string { return testbench.PartitionByBlock }

// Handler serves the partitioned routes.
func (s *LegacyService) Handler() http.Handler {
	routes := http.NewServeMux()
	routes.HandleFunc("/health", legacyHealth)
	routes.Handle(Path, s.withoutServerDiscover())

	return testbench.PartitionRouter(routes)
}

// handlerFor returns the MCP handler owning this block's sessions, creating it on first use.
//
// Stateless is deliberately false. It is what makes the server mint a session id, and it also
// makes the SDK refuse a request carrying the modern _meta envelope - which is how a real legacy
// server answers one, so that refusal is the fixture working rather than a limitation of it.
func (s *LegacyService) handlerFor(key string) http.Handler {
	s.mu.Lock()
	defer s.mu.Unlock()
	if handler, ok := s.partitions[key]; ok {
		return handler
	}
	handler := mcpsdk.NewStreamableHTTPHandler(
		func(*http.Request) *mcpsdk.Server { return newServer() },
		&mcpsdk.StreamableHTTPOptions{Stateless: false},
	)
	s.partitions[key] = handler
	return handler
}

// withoutServerDiscover answers server/discover the way a server that never implemented it does,
// and passes everything else to this block's handler untouched.
//
// The SDK exempts server/discover from its own pre-handshake rejection, so without this the
// method would still be answered here and the control plane would never take its fallback.
func (s *LegacyService) withoutServerDiscover() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "cannot read request body", http.StatusBadRequest)
			return
		}
		// The body is consumed to read the method, so the downstream handler is given a fresh
		// reader over the same bytes.
		r.Body = io.NopCloser(bytes.NewReader(body))

		var envelope struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if err := json.Unmarshal(body, &envelope); err == nil && envelope.Method == methodServerDiscover {
			writeMethodNotFound(w, envelope.ID)
			return
		}

		key, ok := testbench.PartitionKeyFromContext(r.Context())
		if !ok {
			http.Error(w, "missing partition key", http.StatusBadRequest)
			return
		}
		s.handlerFor(key).ServeHTTP(w, r)
	})
}

func writeMethodNotFound(w http.ResponseWriter, id json.RawMessage) {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	body := json.RawMessage(`{"jsonrpc":"2.0","id":` + string(id) +
		`,"error":{"code":-32601,"message":"Method not found"}}`)
	if _, err := w.Write(body); err != nil {
		log.Printf("mcp-legacy: failed to write method-not-found response: %v", err)
	}
}

func legacyHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "mcp-legacy",
	}); err != nil {
		log.Printf("mcp-legacy: failed to write health response: %v", err)
	}
}
