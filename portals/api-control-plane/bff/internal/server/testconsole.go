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

package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"api-control-plane-bff/internal/testproxy"
)

// handleTestInvoke (POST /api/test-console/invoke) relays one Test-console
// try-out request to the gateway the API is deployed on.
//
// This is a BFF-owned endpoint, mounted beside /api/login and /api/session
// rather than under the reverse proxy's prefix, because it is not a proxied
// call: the destination is derived here, per request, and the request that
// leaves this process shares no headers with the one that arrived.
//
// It is always POST, so the global requireCSRF middleware covers it even when
// the API operation being tested is a GET.
func (s *Server) handleTestInvoke(w http.ResponseWriter, r *http.Request) {
	// Nothing about a relayed response is cacheable, and some of it is
	// per-session by construction.
	w.Header().Set("Cache-Control", "no-store")

	if s.testRelay == nil || s.testResolver == nil {
		writeErrorJSON(w, http.StatusNotFound, "NOT_FOUND", "not found")
		return
	}

	// A missing cookie and an expired token are one outcome with one response:
	// the caller has no usable session. Branching them would let a caller tell
	// "never signed in" from "signed in, now expired".
	//
	// Unlike proxyHandler, this endpoint never refreshes a near-expiry OIDC
	// token: the token is used only to ask Platform API which gateway the
	// caller may reach, and a refresh here would rotate the session cookie as
	// a side effect of a try-out click. An expired token simply fails
	// resolution, and the SPA's next Platform API call refreshes as usual.
	token, ok := s.tokenFromCookie(r)
	if !ok || tokenExpired(token) {
		writeErrorJSON(w, http.StatusUnauthorized, "SESSION_EXPIRED", "session expired")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.maxEnvelopeBytes())
	var env testproxy.Envelope
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&env); err != nil {
		if isBodyTooLarge(err) {
			writeErrorJSON(w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "request body too large")
			return
		}
		writeErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST_BODY", "invalid request body")
		return
	}
	if env.RestAPIID == "" || env.GatewayID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST_BODY", "invalid request body")
		return
	}

	requestID := w.Header().Get("X-Request-Id")

	target, err := s.testResolver.Resolve(r.Context(), token, env.OrgHandle, env.RestAPIID, env.GatewayID)
	if err != nil {
		// The specific reason stays server-side. Telling a caller whether the
		// API id exists, whether the gateway is merely undeployed, or what the
		// resolved address was would map the tenant's topology for them.
		slog.Warn("test console target rejected",
			"err", err,
			"rest_api_id", env.RestAPIID,
			"gateway_id", env.GatewayID,
			"req_id", requestID)
		writeErrorJSON(w, http.StatusForbidden, "TARGET_NOT_ALLOWED", "this gateway cannot be tested from the portal")
		return
	}

	relayed, err := s.testRelay.Do(r.Context(), target, env)
	if err != nil {
		s.writeRelayError(w, err, env, target, requestID)
		return
	}

	slog.Info("test console invoke",
		"rest_api_id", env.RestAPIID,
		"gateway_id", target.GatewayID,
		"method", relayedMethod(env),
		"upstream_status", relayed.Status,
		"duration_ms", relayed.DurationMs,
		"truncated", relayed.Truncated,
		"req_id", requestID)

	writeJSON(w, http.StatusOK, testproxy.Result{Outcome: "response", Response: relayed})
}

// envelopeStructuralBytes reserves space for envelope metadata, headers, and JSON syntax.
const envelopeStructuralBytes = 32 << 10

// maxEnvelopeBytes allows for base64 encoding, JSON escaping, and envelope
// metadata while keeping the decoded body within MaxRequestBytes.
func (s *Server) maxEnvelopeBytes() int64 {
	return s.cfg.TestConsole.MaxRequestBytes/3*4 + envelopeStructuralBytes
}

// writeRelayError maps a relay failure onto a distinct client-facing code.
//
// Only the relay's own failures reach here; a gateway that answered with any
// status at all is a success (see testproxy.Relayed). Keeping the two apart is
// what lets the console say "your backend returned 502" and "the portal could
// not reach your gateway" as different sentences, instead of showing one 502
// that could mean either.
func (s *Server) writeRelayError(w http.ResponseWriter, err error, env testproxy.Envelope, target testproxy.Target, requestID string) {
	switch {
	case errors.Is(err, testproxy.ErrBusy):
		writeErrorJSON(w, http.StatusServiceUnavailable, "RELAY_BUSY",
			"too many test requests are in flight, try again shortly")
	case isRequestRejected(err):
		slog.Warn("test console request rejected", "err", err, "req_id", requestID)
		writeErrorJSON(w, http.StatusBadRequest, "INVALID_TEST_REQUEST",
			"this request cannot be sent as written")
	case errors.Is(err, context.DeadlineExceeded):
		slog.Warn("test console upstream timed out",
			"gateway_id", target.GatewayID, "rest_api_id", env.RestAPIID, "req_id", requestID)
		writeServerErrorJSON(w, http.StatusGatewayTimeout, "UPSTREAM_TIMEOUT",
			"the gateway did not respond in time", requestID)
	default:
		// The underlying error frequently names the resolved host or IP, so it
		// is logged and never returned.
		slog.Warn("test console upstream unreachable",
			"err", err, "gateway_id", target.GatewayID, "rest_api_id", env.RestAPIID, "req_id", requestID)
		writeServerErrorJSON(w, http.StatusBadGateway, "UPSTREAM_UNREACHABLE",
			"the gateway could not be reached from the portal", requestID)
	}
}

// isRequestRejected reports whether the relay refused the caller's own request
// shape, as opposed to failing to reach the gateway. These are the caller's to
// fix, so they are a 4xx rather than a 5xx.
func isRequestRejected(err error) bool {
	for _, sentinel := range []error{
		testproxy.ErrInvalidMethod,
		testproxy.ErrInvalidPath,
		testproxy.ErrPathEscapesAPI,
		testproxy.ErrInvalidHeaderName,
		testproxy.ErrInvalidHeaderValue,
		testproxy.ErrForbiddenHeader,
		testproxy.ErrTooManyHeaders,
		testproxy.ErrHeadersTooLarge,
		testproxy.ErrTooManyQueryParams,
		testproxy.ErrInvalidQueryName,
		testproxy.ErrInvalidBodyEncoding,
		testproxy.ErrBodyTooLarge,
	} {
		if errors.Is(err, sentinel) {
			return true
		}
	}
	return false
}

// relayedMethod returns the normalized method for the audit line, falling back
// to the raw value when it was the thing that failed validation.
func relayedMethod(env testproxy.Envelope) string {
	if m, err := testproxy.NormalizeMethod(env.Method); err == nil {
		return m
	}
	return "INVALID"
}

// testInvokeWriteDeadline bounds how long this handler may take to write its
// response. It sits modestly above the relay's own request timeout rather than
// at the generic nonStreamingWriteDeadline, so a stuck gateway is caught by the
// inner bound it was sized for — not by an outer one that would hold the
// connection far longer than the call could legitimately take.
func (s *Server) testInvokeWriteDeadline() time.Duration {
	return s.cfg.TestConsole.RequestTimeout + 5*time.Second
}
