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

// Package mcp provides a stateless streamable-HTTP MCP server for integration tests.
// It exposes the required provider surface through the official MCP SDK.
package mcp

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Port is the container port used by the testbench.
const Port = 3009

// Path is the MCP endpoint path.
const Path = "/mcp"

// addInput contains the operands accepted by the add tool.
type addInput struct {
	// float64 matches JSON's number type and permits fractional operands.
	A float64 `json:"a" jsonschema:"First number"`
	B float64 `json:"b" jsonschema:"Second number"`
}

// echoInput contains the message accepted by the echo tool.
type echoInput struct {
	Message string `json:"message" jsonschema:"Message to echo"`
}

// Service implements the testbench MCP service.
type Service struct {
	handler http.Handler
}

// New builds a stateless MCP service and registers its tools.
func New() *Service {
	server := newServer()

	// Stateless keeps the shared testbench safe for concurrent blocks.
	handler := mcpsdk.NewStreamableHTTPHandler(
		func(*http.Request) *mcpsdk.Server { return server },
		&mcpsdk.StreamableHTTPOptions{Stateless: true},
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", health)
	mux.Handle(Path, handler)

	return &Service{handler: mux}
}

func newServer() *mcpsdk.Server {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "everything",
		Version: "1.0.0",
	}, nil)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "add",
		Description: "Adds two numbers",
	}, add)
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "echo",
		Description: "Echoes back the input",
	}, echo)
	return server
}

// Name returns the service registration name.
func (s *Service) Name() string { return "mcp" }

// Port returns the service's listening port.
func (s *Service) Port() int { return Port }

// Stateful reports whether the service keeps request-specific state.
func (s *Service) Stateful() bool { return false }

// Handler returns the MCP and health handlers.
func (s *Service) Handler() http.Handler { return s.handler }

func health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "mcp",
	}); err != nil {
		log.Printf("mcp: failed to write health response: %v", err)
	}
}

// add returns the sum of two numbers.
func add(_ context.Context, _ *mcpsdk.CallToolRequest, in addInput) (*mcpsdk.CallToolResult, any, error) {
	text := "The sum of " + formatNumber(in.A) + " and " + formatNumber(in.B) +
		" is " + formatNumber(in.A+in.B) + "."
	return textResult(text), nil, nil
}

// echo returns the supplied message.
func echo(_ context.Context, _ *mcpsdk.CallToolRequest, in echoInput) (*mcpsdk.CallToolResult, any, error) {
	return textResult("Echo: " + in.Message), nil, nil
}

// textResult creates a result containing one text content block.
func textResult(text string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: text}},
	}
}

// formatNumber renders a number without unnecessary trailing zeroes.
func formatNumber(f float64) string {
	if f == 0 {
		return "0"
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}
