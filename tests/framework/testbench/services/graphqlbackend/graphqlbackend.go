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

// Package graphqlbackend provides a GraphQL-shaped upstream that echoes the request body back
// verbatim, unwrapped, as the response body.
package graphqlbackend

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

// Port is the container port used by the testbench.
const Port = 3013

// Service implements the stateless GraphQL backend testbench service.
type Service struct{}

// New creates a graphqlbackend service.
func New() *Service { return &Service{} }

// Name returns the service registration name.
func (s *Service) Name() string { return "graphql-backend" }

// Port returns the service's listening port.
func (s *Service) Port() int { return Port }

// Stateful reports whether the service keeps request-specific state.
func (s *Service) Stateful() bool { return false }

// Handler returns the graphqlbackend service's HTTP handler.
//
// Every non-/health request gets the raw request body echoed back verbatim as the response
// body, with no wrapping envelope. This is what a test needs to control the gateway's
// response-phase GraphQL analytics enrichment (isError/errorCount/isPartialSuccess): the
// generic reflect-style backends (services/backend, services/echo) always wrap the body in
// their own {method,path,...} envelope, so a request-supplied top-level "errors" array can
// never appear at the top level of their response the way a real GraphQL server's would.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("/", s.echo)
	return testbench.NormalizeMethod(mux)
}

func (s *Service) health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "graphql-backend",
	}); err != nil {
		log.Printf("graphqlbackend: failed to write health response: %v", err)
	}
}

func (s *Service) echo(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	defer func() { _ = r.Body.Close() }()
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "Request Entity Too Large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if codeStr := r.URL.Query().Get("statusCode"); codeStr != "" {
		if code, err := strconv.Atoi(codeStr); err == nil && code >= 200 && code <= 599 {
			w.WriteHeader(code)
		}
	}
	if _, err := w.Write(body); err != nil {
		log.Printf("graphqlbackend: failed to write echoed body: %v", err)
	}
}
