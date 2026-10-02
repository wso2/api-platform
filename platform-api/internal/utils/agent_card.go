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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// AgentCardWellKnownPath is the A2A discovery path an Agent Card is served on
	// when the supplied endpoint names the agent rather than the card itself.
	AgentCardWellKnownPath = "/.well-known/agent-card.json"

	// defaultAgentCardMaxFetchBytes bounds the fetched card when the configured
	// ceiling is absent or non-positive. It matches the contract's per-card limit
	// (1 MiB), which is also what the gateway will accept — a document larger than
	// this could never be served, so reading more of it has no purpose.
	defaultAgentCardMaxFetchBytes int64 = 1 << 20

	// agentCardFetchTimeout bounds the whole fetch — DNS, connect, TLS and body
	// read. The card fetch sits on a page-display path, so a slow upstream must
	// fail fast rather than hold a request worker for the server's write timeout.
	agentCardFetchTimeout = 10 * time.Second
)

// Fetch outcome classes. A caller maps each onto its own client-facing wording;
// the wrapped cause carries the detail and is for the internal log line only.
//
// The distinction that matters is not how the fetch failed but whether the
// control plane ever saw a usable Agent Card, so these deliberately stop at
// three: could not get an answer, was refused, or got something that is not a
// card.
var (
	// ErrAgentCardUnreachable covers every transport-level failure: connection
	// refused, DNS failure, TLS failure, timeout, and a non-2xx status that is
	// not an authentication refusal.
	ErrAgentCardUnreachable = errors.New("agent card upstream unreachable")

	// ErrAgentCardUnauthorized is the upstream refusing the credentials the
	// control plane presented (401/403).
	ErrAgentCardUnauthorized = errors.New("agent card upstream rejected the supplied credentials")

	// ErrAgentCardUnusable is a successful response whose body is not a document
	// at all: not JSON, not a JSON object, or oversized.
	ErrAgentCardUnusable = errors.New("agent card upstream returned an unusable document")
)

// FetchAgentCard retrieves an A2A Agent Card from an upstream agent and returns
// the response body **verbatim**, having first confirmed it is a JSON document.
//
// The bytes are deliberately not re-encoded from the parsed form: the card
// document is free-form and a round trip through a Go map reorders its keys and
// rewrites its whitespace, which changes exactly the bytes a future Agent Card
// signing implementation signs. The parse below is a check, not a conversion.
//
// rawURL may name either the agent (in which case the well-known discovery path
// is appended) or the card document itself. headerName/headerValue carry the
// already-resolved upstream credential verbatim, the same way the gateway sends
// it; pass empty strings for an unauthenticated fetch.
//
// The request goes through the shared SSRF-guarded client, so the host is
// resolved and the resolved IP is what gets dialed, closing the DNS-rebinding
// window, and every redirect hop is re-validated and bounded to the original
// host — which matters here because the caller's credential rides on the
// request and net/http forwards a custom header across a cross-host redirect.
//
// Returned errors wrap one of the three sentinels above and are sterile at the
// top level: the upstream URL, the credential and the raw body appear only in
// the wrapped cause, for the caller's internal log line.
func FetchAgentCard(ctx context.Context, rawURL, headerName, headerValue string, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = defaultAgentCardMaxFetchBytes
	}

	target, err := AgentCardURL(rawURL)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, agentCardFetchTimeout)
	defer cancel()

	client, err := NewUpstreamFetchClient(agentCardFetchTimeout)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrAgentCardUnreachable, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: build request: %w", ErrAgentCardUnreachable, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "wso2-api-platform")
	// Content-negotiation headers are set by this function, so a credential that
	// asked to travel in one of them would silently overwrite the request's own.
	if headerName != "" && !isReservedAgentCardHeader(headerName) {
		req.Header.Set(headerName, headerValue)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrAgentCardUnreachable, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		// Never relayed as this API's own 401: a client treats a 401 from the
		// control plane as its session expiring, so an upstream demanding
		// credentials would otherwise sign the viewer out.
		return nil, fmt.Errorf("%w: upstream status %d", ErrAgentCardUnauthorized, resp.StatusCode)
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return nil, fmt.Errorf("%w: upstream status %d", ErrAgentCardUnreachable, resp.StatusCode)
	}

	// The shared client's own MaxResponseBytes is disabled by design, so the
	// ceiling is applied here — one byte past the limit, so an oversized document
	// is detected rather than silently truncated into unparsable JSON.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read response body: %w", ErrAgentCardUnreachable, err)
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("%w: document exceeds %d bytes", ErrAgentCardUnusable, maxBytes)
	}

	if err := AssertAgentCard(body); err != nil {
		return nil, err
	}
	return body, nil
}

// AssertAgentCard checks that a fetched body is a JSON document the caller can
// render as an Agent Card, without converting it. Exported so the fetch path and
// its tests agree on what counts.
//
// The check stops at "is this a JSON object". Nothing inside is inspected: the
// card is the upstream agent's document, its shape is the A2A specification's to
// define and the gateway's to enforce at deploy time, and a preview that refused
// to show a card because a field the control plane expected was missing would
// hide the very thing the author needs to see to fix it. What this does catch is
// a response that is not a card at all — an HTML error page, a JSON array, a
// bare scalar — which would otherwise surface as a 500 rather than as the
// upstream problem it is.
func AssertAgentCard(body []byte) error {
	var card map[string]json.RawMessage
	if err := json.Unmarshal(body, &card); err != nil {
		return fmt.Errorf("%w: response is not a JSON object", ErrAgentCardUnusable)
	}
	if card == nil {
		return fmt.Errorf("%w: response is a null document", ErrAgentCardUnusable)
	}
	return nil
}

// AgentCardURL resolves the document to fetch from a configured upstream
// endpoint: the endpoint itself when it already names a card, otherwise the
// endpoint with the A2A well-known discovery path appended.
//
// The endpoint's own query and fragment are dropped when the discovery path is
// appended — they belong to the agent's own address, not to its card — and a
// trailing slash does not produce a doubled separator.
func AgentCardURL(rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", fmt.Errorf("%w: no upstream URL", ErrAgentCardUnreachable)
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("%w: unparsable upstream URL", ErrAgentCardUnreachable)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("%w: upstream URL scheme %q is not supported", ErrAgentCardUnreachable, parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("%w: upstream URL has no host", ErrAgentCardUnreachable)
	}
	if strings.HasSuffix(parsed.Path, ".json") {
		return parsed.String(), nil
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/") + AgentCardWellKnownPath
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

// isReservedAgentCardHeader reports whether a configured credential header would
// collide with one this fetch sets itself.
func isReservedAgentCardHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "accept", "user-agent", "host", "content-type":
		return true
	default:
		return false
	}
}
