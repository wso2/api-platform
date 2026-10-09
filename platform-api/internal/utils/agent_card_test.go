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
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// validAgentCardJSON carries every key an A2A card is defined by, plus a vendor
// extension and deliberately unsorted keys — so a test that compares bytes can
// tell a verbatim pass-through from a re-encode.
const validAgentCardJSON = `{"version":"1.0.0","name":"Weather Agent","description":"Forecasts",` +
	`"supportedInterfaces":[{"protocolBinding":"JSONRPC","url":"https://agents.example.com/rpc","protocolVersion":"1.0"}],` +
	`"capabilities":{"streaming":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],` +
	`"skills":[{"id":"forecast","name":"Forecast","description":"Multi-day","tags":["weather"]}],` +
	`"x-vendor-custom":{"kept":true}}`

func TestAgentCardURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "bare host appends the well-known path", in: "http://agent:9000", want: "http://agent:9000/.well-known/agent-card.json"},
		{name: "trailing slash does not double the separator", in: "http://agent:9000/", want: "http://agent:9000/.well-known/agent-card.json"},
		{name: "sub-path is preserved", in: "https://gw.example.com/weather", want: "https://gw.example.com/weather/.well-known/agent-card.json"},
		{name: "a url already naming a card is used verbatim", in: "https://gw.example.com/cards/weather.json", want: "https://gw.example.com/cards/weather.json"},
		{name: "query on a card url survives", in: "https://gw.example.com/card.json?v=2", want: "https://gw.example.com/card.json?v=2"},
		{name: "query on an agent url is dropped", in: "https://gw.example.com/weather?v=2", want: "https://gw.example.com/weather/.well-known/agent-card.json"},
		{name: "empty", in: "   ", wantErr: true},
		{name: "no scheme", in: "agent:9000", wantErr: true},
		{name: "unsupported scheme", in: "file:///etc/passwd", wantErr: true},
		{name: "no host", in: "http:///weather", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := AgentCardURL(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("AgentCardURL(%q) error = nil, want an error (got %q)", tc.in, got)
				}
				if !errors.Is(err, ErrAgentCardUnreachable) {
					t.Errorf("AgentCardURL(%q) error = %v, want it to wrap ErrAgentCardUnreachable", tc.in, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("AgentCardURL(%q) error = %v, want nil", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("AgentCardURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestAssertAgentCard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "a complete card", body: validAgentCardJSON},
		// The card's interior is the A2A specification's to define and the
		// gateway's to enforce at deploy time. A preview that refused to show a
		// sparse or unfamiliar card would hide the very document the author needs
		// to see in order to fix it.
		{name: "a card missing fields the spec requires", body: strings.Replace(validAgentCardJSON, `"skills":`, `"otherSkills":`, 1)},
		{name: "an object that carries no card fields at all", body: `{"error":"not found"}`},
		{name: "an empty object", body: `{}`},
		{name: "an object carrying only extensions", body: `{"x-vendor-custom":{"kept":true}}`},
		{name: "html error page", body: `<html><body>502 Bad Gateway</body></html>`, wantErr: true},
		{name: "json array", body: `[{"name":"x"}]`, wantErr: true},
		{name: "json scalar", body: `"weather-agent"`, wantErr: true},
		{name: "json null", body: `null`, wantErr: true},
		{name: "empty body", body: ``, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := AssertAgentCard([]byte(tc.body))
			if tc.wantErr {
				if err == nil {
					t.Fatal("AssertAgentCard() error = nil, want an error")
				}
				if !errors.Is(err, ErrAgentCardUnusable) {
					t.Errorf("AssertAgentCard() error = %v, want it to wrap ErrAgentCardUnusable", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("AssertAgentCard() error = %v, want nil", err)
			}
		})
	}
}

// The card is handed back exactly as the upstream sent it — same key order, same
// whitespace. Re-encoding it would change the bytes a future Agent Card signing
// implementation signs.
func TestFetchAgentCard_ReturnsBodyVerbatim(t *testing.T) {
	t.Parallel()

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(validAgentCardJSON))
	}))
	t.Cleanup(server.Close)

	card, err := FetchAgentCard(context.Background(), server.URL, "", "", 0)
	if err != nil {
		t.Fatalf("FetchAgentCard() error = %v, want nil", err)
	}
	if string(card) != validAgentCardJSON {
		t.Errorf("card was not returned verbatim:\n got %s\nwant %s", card, validAgentCardJSON)
	}
	if gotPath != AgentCardWellKnownPath {
		t.Errorf("requested path = %q, want %q", gotPath, AgentCardWellKnownPath)
	}
}

func TestFetchAgentCard_SendsTheSuppliedCredentialHeader(t *testing.T) {
	t.Parallel()

	var gotKey, gotAccept string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		gotAccept = r.Header.Get("Accept")
		_, _ = w.Write([]byte(validAgentCardJSON))
	}))
	t.Cleanup(server.Close)

	if _, err := FetchAgentCard(context.Background(), server.URL, "X-API-Key", "preview-key", 0); err != nil {
		t.Fatalf("FetchAgentCard() error = %v, want nil", err)
	}
	if gotKey != "preview-key" {
		t.Errorf("X-API-Key = %q, want %q", gotKey, "preview-key")
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q, want %q", gotAccept, "application/json")
	}
}

// A credential configured to travel in a header this fetch sets itself would
// otherwise silently overwrite the request's own content negotiation.
func TestFetchAgentCard_RefusesToOverwriteItsOwnHeaders(t *testing.T) {
	t.Parallel()

	var gotAccept string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		_, _ = w.Write([]byte(validAgentCardJSON))
	}))
	t.Cleanup(server.Close)

	if _, err := FetchAgentCard(context.Background(), server.URL, "Accept", "text/html", 0); err != nil {
		t.Fatalf("FetchAgentCard() error = %v, want nil", err)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q, want the fetch's own %q", gotAccept, "application/json")
	}
}

// Every upstream failure class is classified, and none of the three sentinels is
// an internal error: the control plane is healthy in all of them.
func TestFetchAgentCard_ClassifiesUpstreamFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    error
	}{
		{
			name:    "401 unauthorized",
			handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) },
			want:    ErrAgentCardUnauthorized,
		},
		{
			name:    "403 forbidden",
			handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) },
			want:    ErrAgentCardUnauthorized,
		},
		{
			name:    "404 not found",
			handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) },
			want:    ErrAgentCardUnreachable,
		},
		{
			name:    "500 from the upstream",
			handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
			want:    ErrAgentCardUnreachable,
		},
		{
			name: "200 that is not a card",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`<html><body>hello</body></html>`))
			},
			want: ErrAgentCardUnusable,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(tc.handler)
			t.Cleanup(server.Close)

			card, err := FetchAgentCard(context.Background(), server.URL, "", "", 0)
			if err == nil {
				t.Fatalf("FetchAgentCard() error = nil, want an error (got card %s)", card)
			}
			if !errors.Is(err, tc.want) {
				t.Errorf("FetchAgentCard() error = %v, want it to wrap %v", err, tc.want)
			}
			if card != nil {
				t.Errorf("FetchAgentCard() returned a card alongside an error: %s", card)
			}
		})
	}
}

// A document the control plane would not itself have authored is still the
// upstream's card, and is passed through untouched.
func TestFetchAgentCard_ReturnsASparseDocumentUnchanged(t *testing.T) {
	t.Parallel()

	const sparse = `{"name":"Weather Agent"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(sparse))
	}))
	t.Cleanup(server.Close)

	card, err := FetchAgentCard(context.Background(), server.URL, "", "", 0)
	if err != nil {
		t.Fatalf("FetchAgentCard() error = %v, want nil", err)
	}
	if string(card) != sparse {
		t.Errorf("card = %s, want %s", card, sparse)
	}
}

func TestFetchAgentCard_UnreachableUpstream(t *testing.T) {
	t.Parallel()

	// Started and immediately closed, so the port is closed and the dial fails —
	// the connection-refused case, without depending on a fixed port number.
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := server.URL
	server.Close()

	_, err := FetchAgentCard(context.Background(), url, "", "", 0)
	if err == nil {
		t.Fatal("FetchAgentCard() error = nil, want an error")
	}
	if !errors.Is(err, ErrAgentCardUnreachable) {
		t.Errorf("FetchAgentCard() error = %v, want it to wrap ErrAgentCardUnreachable", err)
	}
}

// An oversized document is refused rather than truncated: a truncated card would
// fail as unparsable JSON, which reports the wrong problem.
func TestFetchAgentCard_RejectsAnOversizedDocument(t *testing.T) {
	t.Parallel()

	padded := strings.Replace(validAgentCardJSON, `"x-vendor-custom":{"kept":true}`,
		`"x-vendor-custom":{"kept":"`+strings.Repeat("p", 4096)+`"}`, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(padded))
	}))
	t.Cleanup(server.Close)

	_, err := FetchAgentCard(context.Background(), server.URL, "", "", 512)
	if err == nil {
		t.Fatal("FetchAgentCard() error = nil, want an error")
	}
	if !errors.Is(err, ErrAgentCardUnusable) {
		t.Errorf("FetchAgentCard() error = %v, want it to wrap ErrAgentCardUnusable", err)
	}

	// The same document fetches cleanly once the ceiling admits it, so the
	// rejection above is the ceiling talking and not the document itself.
	if _, err := FetchAgentCard(context.Background(), server.URL, "", "", int64(len(padded))); err != nil {
		t.Fatalf("FetchAgentCard() with a sufficient ceiling: error = %v, want nil", err)
	}
}

// A cancelled context stops the fetch rather than holding the caller for the
// full timeout.
func TestFetchAgentCard_HonoursCallerCancellation(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		_, _ = w.Write([]byte(validAgentCardJSON))
	}))
	t.Cleanup(func() {
		close(release)
		server.Close()
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := FetchAgentCard(ctx, server.URL, "", "", 0)
	if err == nil {
		t.Fatal("FetchAgentCard() error = nil, want an error")
	}
	if !errors.Is(err, ErrAgentCardUnreachable) {
		t.Errorf("FetchAgentCard() error = %v, want it to wrap ErrAgentCardUnreachable", err)
	}
}
