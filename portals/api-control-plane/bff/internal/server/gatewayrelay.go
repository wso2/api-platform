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

	"api-control-plane-bff/internal/egress"
	"api-control-plane-bff/internal/gatewayrelay"
)

// handleGatewayInvoke (POST /api/gateway-relay/invoke) relays one Gateway-relay
// try-out request to the gateway the API is deployed on.
//
// This is a BFF-owned endpoint, mounted beside /api/login and /api/session
// rather than under the reverse proxy's prefix, because it is not a proxied
// call: the destination is derived here, per request, and the request that
// leaves this process shares no headers with the one that arrived.
//
// It is always POST, so the global requireCSRF middleware covers it even when
// the API operation being tested is a GET.
func (s *Server) handleGatewayInvoke(w http.ResponseWriter, r *http.Request) {
	// Nothing about a relayed response is cacheable, and some of it is
	// per-session by construction.
	w.Header().Set("Cache-Control", "no-store")

	if s.gatewayRelay == nil || s.gatewayResolver == nil {
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
	var env gatewayrelay.Envelope
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

	// Bound separately because resolution performs two sequential Platform API
	// reads and must finish before the route's write deadline.
	resolveCtx, cancelResolve := context.WithTimeout(r.Context(), gatewayResolveTimeout)
	target, err := s.gatewayResolver.Resolve(resolveCtx, token, env.OrgHandle, env.RestAPIID, env.GatewayID)
	// Captured before cancelling, so the deadline is distinguishable from the
	// cancellation this defer-free call performs itself.
	resolveErr := resolveCtx.Err()
	cancelResolve()
	if err != nil {
		// Running out of time says nothing about whether this gateway may be
		// tested, so it must not answer as though it did: a 403 would send the
		// user off to check a deployment that is perfectly fine.
		if errors.Is(resolveErr, context.DeadlineExceeded) {
			slog.Warn("gateway relay target resolution timed out",
				"rest_api_id", env.RestAPIID,
				"gateway_id", env.GatewayID,
				"req_id", requestID)
			writeServerErrorJSON(w, http.StatusGatewayTimeout, "UPSTREAM_TIMEOUT",
				"the control plane did not respond in time", requestID)
			return
		}
		slog.Warn("gateway relay target rejected",
			"err", err,
			"rest_api_id", env.RestAPIID,
			"gateway_id", env.GatewayID,
			"req_id", requestID)
		writeResolveError(w, err)
		return
	}

	relayed, err := s.gatewayRelay.Do(r.Context(), target, env)
	if err != nil {
		s.writeRelayError(w, err, env, target, requestID)
		return
	}

	slog.Info("gateway relay invoke",
		"rest_api_id", env.RestAPIID,
		"gateway_id", target.GatewayID,
		"method", relayedMethod(env),
		"upstream_status", relayed.Status,
		"duration_ms", relayed.DurationMs,
		"truncated", relayed.Truncated,
		"req_id", requestID)

	writeJSON(w, http.StatusOK, gatewayrelay.Result{Outcome: "response", Response: relayed})
}

// envelopeStructuralBytes reserves space for envelope metadata, headers, and JSON syntax.
const envelopeStructuralBytes = 32 << 10

// maxEnvelopeBytes allows for base64 encoding, JSON escaping, and envelope
// metadata while keeping the decoded body within MaxRequestBytes.
func (s *Server) maxEnvelopeBytes() int64 {
	return s.cfg.GatewayRelay.MaxRequestBytes/3*4 + envelopeStructuralBytes
}

// writeResolveError maps resolution failures to client-facing codes. API and
// deployment errors are distinct; cross-organization gateways remain
// indistinguishable from undeployed ones to avoid confirming their existence.
func writeResolveError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, gatewayrelay.ErrAPINotFound):
		writeErrorJSON(w, http.StatusNotFound, "API_NOT_FOUND",
			"this API could not be found, or this session cannot access it")
	case errors.Is(err, gatewayrelay.ErrGatewayNotFound),
		errors.Is(err, gatewayrelay.ErrGatewayOtherOrg):
		writeErrorJSON(w, http.StatusNotFound, "GATEWAY_NOT_DEPLOYED",
			"this API is not deployed to the selected gateway")
	case errors.Is(err, gatewayrelay.ErrGatewayNoEndpoint):
		writeErrorJSON(w, http.StatusUnprocessableEntity, "GATEWAY_NO_ENDPOINT",
			"the selected gateway has no endpoint configured")
	case errors.Is(err, gatewayrelay.ErrUnsupportedScheme):
		writeErrorJSON(w, http.StatusUnprocessableEntity, "GATEWAY_UNSUPPORTED_SCHEME",
			"the selected gateway's endpoint is not an http or https address")
	default:
		// An unrecognised resolution failure is a bug or an upstream shape
		// change, not a statement about this caller's target.
		writeErrorJSON(w, http.StatusBadGateway, "UPSTREAM_UNREACHABLE",
			"the control plane could not be reached")
	}
}

// writeRelayError maps a relay failure onto a distinct client-facing code.
//
// Only the relay's own failures reach here; a gateway that answered with any
// status at all is a success (see gatewayrelay.Relayed). Keeping the two apart is
// what lets the console say "your backend returned 502" and "the portal could
// not reach your gateway" as different sentences, instead of showing one 502
// that could mean either.
func (s *Server) writeRelayError(w http.ResponseWriter, err error, env gatewayrelay.Envelope, target gatewayrelay.Target, requestID string) {
	switch {
	case errors.Is(err, egress.ErrTargetNotAllowed),
		errors.Is(err, egress.ErrIPLiteralTarget),
		errors.Is(err, egress.ErrNoTargetHost),
		// Policy refusal, even when discovered during dialing.
		errors.Is(err, egress.ErrAddressNotAllowed):
		// The target resolved fine; the egress policy refused it. Same
		// generic answer as any other refused target, with the specific
		// reason kept server-side.
		slog.Warn("gateway relay target refused by egress policy",
			"err", err, "gateway_id", target.GatewayID,
			"rest_api_id", env.RestAPIID, "req_id", requestID)
		writeErrorJSON(w, http.StatusForbidden, "TARGET_NOT_ALLOWED",
			"this gateway cannot be tested from the portal")
	case errors.Is(err, gatewayrelay.ErrBusy):
		writeErrorJSON(w, http.StatusServiceUnavailable, "RELAY_BUSY",
			"too many test requests are in flight, try again shortly")
	case isRequestRejected(err):
		slog.Warn("gateway relay request rejected", "err", err, "req_id", requestID)
		writeErrorJSON(w, http.StatusBadRequest, "INVALID_TEST_REQUEST",
			"this request cannot be sent as written")
	case errors.Is(err, context.DeadlineExceeded):
		slog.Warn("gateway relay upstream timed out",
			"gateway_id", target.GatewayID, "rest_api_id", env.RestAPIID, "req_id", requestID)
		writeServerErrorJSON(w, http.StatusGatewayTimeout, "UPSTREAM_TIMEOUT",
			"the gateway did not respond in time", requestID)
	default:
		// The underlying error frequently names the resolved host or IP, so it
		// is logged and never returned.
		slog.Warn("gateway relay upstream unreachable",
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
		gatewayrelay.ErrInvalidMethod,
		gatewayrelay.ErrInvalidPath,
		gatewayrelay.ErrPathEscapesAPI,
		gatewayrelay.ErrInvalidHeaderName,
		gatewayrelay.ErrInvalidHeaderValue,
		gatewayrelay.ErrForbiddenHeader,
		gatewayrelay.ErrTooManyHeaders,
		gatewayrelay.ErrHeadersTooLarge,
		gatewayrelay.ErrTooManyQueryParams,
		gatewayrelay.ErrInvalidQueryName,
		gatewayrelay.ErrInvalidBodyEncoding,
		gatewayrelay.ErrBodyTooLarge,
	} {
		if errors.Is(err, sentinel) {
			return true
		}
	}
	return false
}

// relayedMethod returns the normalized method for the audit line, falling back
// to the raw value when it was the thing that failed validation.
func relayedMethod(env gatewayrelay.Envelope) string {
	if m, err := gatewayrelay.NormalizeMethod(env.Method); err == nil {
		return m
	}
	return "INVALID"
}

// gatewayResolveTimeout bounds target resolution: two sequential Platform API
// reads, against a control plane the BFF is co-deployed with. Deliberately far
// below the shared upstream client's own 60s ceiling, which is sized for user
// -facing proxying rather than for a phase that has to finish before the relay
// has even started.
// A var rather than a const so a test can shorten it; nothing else writes it.
var gatewayResolveTimeout = 10 * time.Second

// gatewayInvokeWriteDeadline bounds how long this handler may take to write its
// response. It is the sum of the two phases it actually performs — resolution
// and the relayed call — plus a margin to write the result, rather than the
// generic nonStreamingWriteDeadline. Sized this way the inner bounds always
// fire first, so a stuck gateway or a slow control plane surfaces as a clean
// error rather than a connection cut off mid-response.
func (s *Server) gatewayInvokeWriteDeadline() time.Duration {
	return gatewayResolveTimeout + s.cfg.GatewayRelay.RequestTimeout + 5*time.Second
}
