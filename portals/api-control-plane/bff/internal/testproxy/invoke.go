/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package testproxy relays one Test-console try-out request to the gateway the
// API is deployed on, and returns the gateway's whole response as data.
//
// # Why this exists
//
// The Test page's Swagger console used to have the browser call the gateway
// directly. That is cross-origin in every deployment, so the test key header
// triggers a CORS preflight the gateway only answers if the API happens to
// carry a cors policy; a plain-http gateway is additionally blocked as mixed
// content from an https portal, and a cluster-internal gateway is not routable
// from the user's machine at all. Relaying through the BFF removes all three,
// and returns *more* than a direct call could: a browser can only read simple
// response headers unless the gateway opts in with Access-Control-Expose-Headers,
// whereas the envelope below carries every one of them.
//
// # Why it is not internal/proxy
//
// internal/proxy is a reverse proxy to operator-configured upstreams, and its
// Rewrite hook sets `Authorization: Bearer <session token>` on everything it
// forwards. That is correct for the Platform API and catastrophic here: a
// test-console target is a URL a tenant admin controls, so forwarding the
// caller's Platform API token to it would hand that token to whoever runs the
// gateway. This package is deliberately a separate implementation that has no
// access to the session token at all — see Relay.Do, which only ever sets
// headers taken from the sanitized envelope.
//
// # Scope: cloud-managed gateways
//
// The relay assumes the BFF can route to the gateway. That holds for the
// cloud-managed gateways this targets, and is why there is no fallback path
// here. A self-hosted gateway registered to a remote control plane can be the
// other way round, reachable from the user's browser but not from the BFF,
// and for that the console is to grow a toggle that sends directly instead.
// That toggle is deliberately future work, not a gap this package papers over:
// until it exists, an unroutable gateway surfaces as UPSTREAM_UNREACHABLE
// rather than silently falling back to a call the browser cannot make either.
//
// # Shape
//
// The browser posts one JSON envelope and gets one JSON result back, rather
// than the BFF transparently proxying method+path. That choice is what keeps
// the trust boundary legible in both directions: no browser-supplied cookie or
// implicit header can reach the gateway, and no gateway-supplied header
// (Set-Cookie, a text/html Content-Type) can reach the portal origin as
// anything other than an inert JSON string.
package testproxy

import (
	"encoding/base64"
	"unicode/utf8"
)

// Body encodings used in both directions of the envelope. A body is represented
// as a plain UTF-8 string when it contains valid UTF-8; otherwise, it is encoded
// as base64 to preserve binary request and response data across the JSON
// round trip.
const (
	EncodingUTF8   = "utf8"
	EncodingBase64 = "base64"
)

// Header is one header or query parameter. A list preserves repeated names.
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Envelope is posted by the browser to the invoke endpoint. It contains no URL:
// the browser identifies the API and gateway, and the BFF derives the invoke
// URL through Platform API (see Resolver). Accepting an absolute caller-supplied
// URL would expose an authenticated open proxy in the control-plane network;
// restricting it safely would require denylist-based controls.
type Envelope struct {
	// OrgHandle is forwarded to Platform API as X-Org-Id for target resolution.
	// Platform API validates it against the caller's token, and Resolver verifies
	// that the resolved gateway belongs to the same organization.
	OrgHandle string `json:"orgHandle"`
	// RestAPIID is the API being tested. Entitlement to it is established by
	// Platform API answering the resolution call with the caller's own token.
	RestAPIID string `json:"restApiId"`
	// GatewayID is which of that API's deployed gateways to send to.
	GatewayID string `json:"gatewayId"`

	Method string `json:"method"`
	// Path is relative to the API's invoke URL, leading slash included.
	Path         string   `json:"path"`
	Query        []Header `json:"query"`
	Headers      []Header `json:"headers"`
	Body         string   `json:"body"`
	BodyEncoding string   `json:"bodyEncoding"`
}

// DecodeBody returns the request body, decoding base64 when specified.
// Unrecognized encodings return an error.
func (e Envelope) DecodeBody() ([]byte, error) {
	switch e.BodyEncoding {
	case "", EncodingUTF8:
		return []byte(e.Body), nil
	case EncodingBase64:
		return base64.StdEncoding.DecodeString(e.Body)
	default:
		return nil, ErrInvalidBodyEncoding
	}
}

// Relayed contains the gateway response as data.
//
// Status is the gateway's status and is not used as the BFF response status.
// A successful relay always returns HTTP 200, allowing the console to
// distinguish a gateway error from a BFF-level error.
type Relayed struct {
	Status     int      `json:"status"`
	StatusText string   `json:"statusText"`
	Headers    []Header `json:"headers"`
	Body       string   `json:"body"`
	// BodyEncoding is EncodingUTF8 or EncodingBase64 — see Body.
	BodyEncoding string `json:"bodyEncoding"`
	// Truncated reports that the body exceeded MaxResponseBytes and was clipped.
	Truncated bool `json:"truncated"`
	// DurationMs is the gateway round trip as measured by the BFF, excluding
	// target resolution and the browser's own hop to the BFF.
	DurationMs int64 `json:"durationMs"`
}

// Result is the success envelope. Outcome is a constant discriminator so the
// browser never has to infer success from the absence of an error field.
type Result struct {
	Outcome  string  `json:"outcome"` // always "response"
	Response Relayed `json:"response"`
}

// EncodeBody renders response bytes for the envelope, choosing base64 only when
// the payload is not valid UTF-8. JSON cannot carry arbitrary bytes in a string,
// so a binary response would otherwise arrive mangled by replacement characters.
func EncodeBody(b []byte) (body, encoding string) {
	if utf8.Valid(b) {
		return string(b), EncodingUTF8
	}
	return base64.StdEncoding.EncodeToString(b), EncodingBase64
}
