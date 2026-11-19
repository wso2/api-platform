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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DefaultCallTimeout bounds the handshake and each call when Options.CallTimeout is zero.
const DefaultCallTimeout = 30 * time.Second

// clientName identifies the framework's client to the server.
const clientName = "apip-test-framework"

// ErrSessionClosed is returned by a call on a session that has been closed.
var ErrSessionClosed = errors.New("mcpx: session is closed")

// Options say what each request carries besides the protocol's own headers, and how long a
// call may take.
type Options struct {
	// Headers are added to every request, except a header the SDK sets itself.
	Headers map[string]string
	// Host, when not empty, replaces the Host of every request.
	Host string
	// CallTimeout bounds the handshake and each call. Zero selects DefaultCallTimeout.
	CallTimeout time.Duration
}

// ToolResult is what a tool call returned.
type ToolResult struct {
	// Text joins the text content blocks of the answer, one per line.
	Text string
	// IsError reports a tool that ran but failed.
	IsError bool
}

// Session is one open SDK client session. Its methods are safe for concurrent use.
type Session struct {
	sdk         *mcp.ClientSession
	handshake   []string
	callTimeout time.Duration

	mu     sync.Mutex
	closed bool
}

// Connect opens a session to the MCP endpoint at rawURL with one attempt; a caller waiting for an
// endpoint to become routable retries it with core/util/retry.
func Connect(ctx context.Context, rawURL string, opts Options) (*Session, error) {
	endpoint, err := parseEndpoint(rawURL)
	if err != nil {
		return nil, err
	}
	timeout := opts.CallTimeout
	if timeout < 0 {
		return nil, fmt.Errorf("mcpx: call timeout must not be negative, got %s", timeout)
	}
	if timeout == 0 {
		timeout = DefaultCallTimeout
	}

	sent := &sentMethods{}
	transport := &mcp.StreamableClientTransport{
		Endpoint: endpoint,
		HTTPClient: &http.Client{
			Timeout: timeout,
			Transport: &requestDecorator{
				base: http.DefaultTransport, headers: cloneHeaders(opts.Headers), host: opts.Host, sent: sent,
			},
		},
		MaxRetries:           -1,
		DisableStandaloneSSE: true,
	}
	connectCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: clientName, Version: "1.0.0"}, nil)
	sdk, err := client.Connect(connectCtx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("mcpx: connecting to %s: %w", endpoint, err)
	}
	return &Session{sdk: sdk, handshake: sent.list(), callTimeout: timeout}, nil
}

// parseEndpoint accepts only an absolute http or https URL.
func parseEndpoint(rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", errors.New("mcpx: endpoint URL is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("mcpx: endpoint URL %q: %w", rawURL, err)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("mcpx: endpoint URL %q must be an absolute http or https URL", rawURL)
	}
	return trimmed, nil
}

// Handshake returns the JSON-RPC methods the client sent while it connected, in order.
func (s *Session) Handshake() []string {
	if s == nil {
		return nil
	}
	return slices.Clone(s.handshake)
}

// ProtocolVersion returns the protocol version the handshake agreed on.
func (s *Session) ProtocolVersion() string {
	if s == nil {
		return ""
	}
	if result := s.sdk.InitializeResult(); result != nil {
		return result.ProtocolVersion
	}
	return ""
}

// ID returns the Mcp-Session-Id the server issued, or "" from a server that keeps no session.
func (s *Session) ID() string {
	if s == nil {
		return ""
	}
	return s.sdk.ID()
}

// CallTool calls the named tool. A transport or JSON-RPC failure is returned as the error; a
// tool that ran but failed is reported in ToolResult.IsError. The SDK ends a session whose
// request was refused with a non-success HTTP status, so a caller expecting a refusal opens a
// new session for the next call.
func (s *Session) CallTool(ctx context.Context, name string, arguments map[string]any) (ToolResult, error) {
	if s == nil {
		return ToolResult{}, errors.New("mcpx: session is nil")
	}
	if strings.TrimSpace(name) == "" {
		return ToolResult{}, errors.New("mcpx: tool name is required")
	}
	if s.isClosed() {
		return ToolResult{}, ErrSessionClosed
	}
	callCtx, cancel := context.WithTimeout(ctx, s.callTimeout)
	defer cancel()
	result, err := s.sdk.CallTool(callCtx, &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		return ToolResult{}, fmt.Errorf("mcpx: calling tool %q: %w", name, err)
	}
	texts := make([]string, 0, len(result.Content))
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			texts = append(texts, text.Text)
		}
	}
	return ToolResult{Text: strings.Join(texts, "\n"), IsError: result.IsError}, nil
}

// Close ends the session; a server that keeps sessions is told with a DELETE. Closing a nil or
// closed session does nothing.
func (s *Session) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()
	return s.sdk.Close()
}

func (s *Session) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// sentMethods records the JSON-RPC method of every message the client posts, in order.
type sentMethods struct {
	mu      sync.Mutex
	methods []string
}

func (m *sentMethods) add(method string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.methods = append(m.methods, method)
}

func (m *sentMethods) list() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.methods)
}

// requestDecorator puts the caller's headers and Host on every request the SDK sends, keeping any
// header the SDK set, and records the request's JSON-RPC method.
type requestDecorator struct {
	base    http.RoundTripper
	headers map[string]string
	host    string
	sent    *sentMethods
}

// RoundTrip sends a decorated copy of the request; a RoundTripper must not change the request it
// is given.
func (d *requestDecorator) RoundTrip(req *http.Request) (*http.Response, error) {
	out := req.Clone(req.Context())
	for name, value := range d.headers {
		if out.Header.Get(name) == "" {
			out.Header.Set(name, value)
		}
	}
	if d.host != "" {
		out.Host = d.host
	}
	if method := jsonRPCMethod(req); method != "" {
		d.sent.add(method)
	}
	return d.base.RoundTrip(out)
}

// jsonRPCMethod reads the method of a request's JSON-RPC body from a fresh copy of the body, so
// the body that is sent stays unread. It returns "" for a request without a JSON-RPC body, such
// as the DELETE that ends a session.
func jsonRPCMethod(req *http.Request) string {
	if req == nil || req.GetBody == nil {
		return ""
	}
	body, err := req.GetBody()
	if err != nil {
		return ""
	}
	defer body.Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		return ""
	}
	var envelope struct {
		Method string `json:"method"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return ""
	}
	return envelope.Method
}

func cloneHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(headers))
	for name, value := range headers {
		cloned[name] = value
	}
	return cloned
}
