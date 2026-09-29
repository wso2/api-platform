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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// graphqlInvokeMaxRequestBytes bounds the GraphiQL request body (query +
// variables + headers) the Test Console forwards through this handler.
const graphqlInvokeMaxRequestBytes = 2 << 20 // 2 MiB

// graphqlInvokeMaxResponseBytes bounds how much of the gateway's response this
// handler will buffer before relaying it back to the console.
const graphqlInvokeMaxResponseBytes = 5 << 20 // 5 MiB

// platformAPILookupMaxBytes bounds each of the small Platform API metadata
// lookups (GraphQL API, gateway, deployment list) this handler makes to
// resolve the real invoke URL.
const platformAPILookupMaxBytes = 1 << 20 // 1 MiB

// graphqlInvokeTimeout bounds the whole resolve-then-invoke round trip. Kept
// modestly below the handler's own withWriteDeadline (30s, see server.go) so
// a stuck Platform API or gateway call is caught here first, per
// go-network-service-hardening.md directive 5.
const graphqlInvokeTimeout = 15 * time.Second

var graphqlVersionPlaceholder = regexp.MustCompile(`\$version`)

// graphqlInvokeLookupError is a resolution failure with a specific status/code
// worth surfacing to the caller (not found, not deployed here) rather than a
// generic 502.
type graphqlInvokeLookupError struct {
	status  int
	code    string
	message string
}

func (e *graphqlInvokeLookupError) Error() string { return e.message }

type graphqlAPILookupResponse struct {
	Context string `json:"context"`
	Version string `json:"version"`
}

type gatewayLookupResponse struct {
	Endpoints []string `json:"endpoints"`
}

type deploymentListLookupResponse struct {
	List []struct {
		GatewayID string `json:"gatewayId"`
		Status    string `json:"status"`
	} `json:"list"`
}

// handleGraphQLInvoke (POST /api/graphql-console/{graphqlApiId}/gateways/{gatewayId}/invoke)
// lets the GraphQL Test Console call a deployed GraphQL API's real gateway
// endpoint through this same-origin BFF route instead of the browser talking
// to the gateway directly — sidestepping both the gateway's CORS
// configuration and its (often self-signed, dev-only) TLS certificate.
//
// The target URL is resolved entirely from the caller's own organization-scoped
// Platform API records (the GraphQL API's context/version, the gateway's
// registered endpoint, and confirmation the two are actually linked by a
// DEPLOYED deployment) — never from a client-supplied URL. Gateways are
// legitimately often on private/internal addresses, so the usual SSRF
// network-range filtering doesn't apply here; the control is authorization
// against data the caller's own session token already scopes to their org
// (see ssrf-prevention.md).
//
// Unlike proxyHandler/proxy.ReverseProxy, this handler deliberately does NOT
// overwrite the caller's Authorization header with the BFF session token: the
// whole point of the console is that the caller supplies their own test
// credential for the target API's own auth policy, which is unrelated to
// their Platform API session.
func (s *Server) handleGraphQLInvoke(w http.ResponseWriter, r *http.Request) {
	token, ok := s.tokenFromCookie(r)
	if !ok {
		writeErrorJSON(w, http.StatusUnauthorized, "NOT_AUTHENTICATED", "not authenticated")
		return
	}
	if s.oidc == nil && tokenExpired(token) {
		s.clearSessionCookie(w)
		writeErrorJSON(w, http.StatusUnauthorized, "SESSION_EXPIRED", "session expired")
		return
	}

	graphqlAPIID := r.PathValue("graphqlApiId")
	gatewayID := r.PathValue("gatewayId")
	if graphqlAPIID == "" || gatewayID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "graphqlApiId and gatewayId are required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, graphqlInvokeMaxRequestBytes)
	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		if isBodyTooLarge(err) {
			writeErrorJSON(w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "request body too large")
			return
		}
		writeErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST_BODY", "invalid request body")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), graphqlInvokeTimeout)
	defer cancel()

	invokeURL, err := s.resolveGraphQLInvokeURL(ctx, token, graphqlAPIID, gatewayID)
	if err != nil {
		var lookupErr *graphqlInvokeLookupError
		if errors.As(err, &lookupErr) {
			writeErrorJSON(w, lookupErr.status, lookupErr.code, lookupErr.message)
			return
		}
		slog.Error("graphql invoke: failed to resolve target", "err", err)
		writeServerErrorJSON(w, http.StatusBadGateway, "INVOKE_RESOLUTION_FAILED",
			"failed to resolve the API's gateway endpoint", w.Header().Get("X-Request-Id"))
		return
	}

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, invokeURL, bytes.NewReader(reqBody))
	if err != nil {
		slog.Error("graphql invoke: failed to build upstream request", "err", err)
		writeServerErrorJSON(w, http.StatusInternalServerError, "INVOKE_REQUEST_FAILED",
			"failed to build the upstream request", w.Header().Get("X-Request-Id"))
		return
	}
	// Forward the caller's headers as-is, except hop-by-hop / BFF-owned ones.
	// Deliberately NOT stripping/overwriting Authorization — see doc comment.
	//
	// accept-encoding is dropped rather than forwarded: http.Transport only
	// transparently decompresses a response when IT added Accept-Encoding
	// itself (default behavior when the request sets none). Forwarding the
	// browser's own Accept-Encoding (e.g. "gzip, deflate, br") makes the
	// Transport treat compression as explicitly requested by the caller, so a
	// gzipped upstream response comes back through resp.Body still compressed
	// — and since only Content-Type is relayed below (never Content-Encoding),
	// the browser has no way to know to decompress it either, so it renders
	// raw compressed bytes as if they were the JSON body. Omitting the header
	// here lets the Transport manage request/response compression itself and
	// hand back plain decompressed bytes.
	for name, values := range r.Header {
		switch strings.ToLower(name) {
		case "cookie", "host", "content-length", "connection", "x-requested-by", "accept-encoding":
			continue
		}
		for _, v := range values {
			upstreamReq.Header.Add(name, v)
		}
	}
	if upstreamReq.Header.Get("Content-Type") == "" {
		upstreamReq.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.upstream.Do(upstreamReq)
	if err != nil {
		writeErrorJSON(w, http.StatusBadGateway, "INVOKE_FAILED", "the gateway could not be reached")
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, graphqlInvokeMaxResponseBytes+1))
	if err != nil {
		slog.Error("graphql invoke: failed to read upstream response", "err", err)
		writeServerErrorJSON(w, http.StatusBadGateway, "INVOKE_READ_FAILED",
			"failed to read the gateway's response", w.Header().Get("X-Request-Id"))
		return
	}
	if len(body) > graphqlInvokeMaxResponseBytes {
		writeErrorJSON(w, http.StatusBadGateway, "INVOKE_RESPONSE_TOO_LARGE", "the gateway's response was too large")
		return
	}

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)
}

// resolveGraphQLInvokeURL looks up the GraphQL API and gateway from Platform
// API (using the caller's own session token, so results are already scoped to
// their organization), confirms the API is actually DEPLOYED to that gateway,
// and builds the real invoke URL the same way InvokeUrlPanel.tsx's
// buildInvokeUrl does client-side.
func (s *Server) resolveGraphQLInvokeURL(ctx context.Context, token, graphqlAPIID, gatewayID string) (string, error) {
	api, err := s.fetchGraphQLAPIForInvoke(ctx, token, graphqlAPIID)
	if err != nil {
		return "", err
	}
	gw, err := s.fetchGatewayForInvoke(ctx, token, gatewayID)
	if err != nil {
		return "", err
	}
	if len(gw.Endpoints) == 0 {
		return "", &graphqlInvokeLookupError{
			status: http.StatusUnprocessableEntity, code: "GATEWAY_HAS_NO_ENDPOINT",
			message: "the selected gateway has no registered endpoint",
		}
	}
	if err := s.confirmGraphQLDeployment(ctx, token, graphqlAPIID, gatewayID); err != nil {
		return "", err
	}
	return buildGraphQLInvokeURL(gw.Endpoints[0], api.Context, api.Version), nil
}

// platformAPIGet issues an authenticated GET against the Platform API,
// reusing the same transport/TLS-trust settings as the primary reverse proxy
// (cfg.ControlPlane.*) — this handler adds no new upstream config surface.
func (s *Server) platformAPIGet(ctx context.Context, token, pathAndQuery string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.ControlPlane.URL+pathAndQuery, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	return s.upstream.Do(req)
}

func (s *Server) fetchGraphQLAPIForInvoke(ctx context.Context, token, graphqlAPIID string) (*graphqlAPILookupResponse, error) {
	resp, err := s.platformAPIGet(ctx, token, "/api/v0.9/graphql-apis/"+url.PathEscape(graphqlAPIID))
	if err != nil {
		return nil, fmt.Errorf("fetch graphql api: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		return nil, &graphqlInvokeLookupError{status: http.StatusNotFound, code: "GRAPHQL_API_NOT_FOUND", message: "the GraphQL API was not found"}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch graphql api: unexpected status %d", resp.StatusCode)
	}
	var out graphqlAPILookupResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, platformAPILookupMaxBytes+1)).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode graphql api response: %w", err)
	}
	return &out, nil
}

func (s *Server) fetchGatewayForInvoke(ctx context.Context, token, gatewayID string) (*gatewayLookupResponse, error) {
	resp, err := s.platformAPIGet(ctx, token, "/api/v0.9/gateways/"+url.PathEscape(gatewayID))
	if err != nil {
		return nil, fmt.Errorf("fetch gateway: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		return nil, &graphqlInvokeLookupError{status: http.StatusNotFound, code: "GATEWAY_NOT_FOUND", message: "the gateway was not found"}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch gateway: unexpected status %d", resp.StatusCode)
	}
	var out gatewayLookupResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, platformAPILookupMaxBytes+1)).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode gateway response: %w", err)
	}
	return &out, nil
}

// confirmGraphQLDeployment rejects a graphqlAPIID/gatewayID pair that isn't
// actually linked by a live deployment — the console's own gateway picker
// only ever offers deployed gateways, but this handler doesn't trust that
// client-side invariant for an authorization decision.
func (s *Server) confirmGraphQLDeployment(ctx context.Context, token, graphqlAPIID, gatewayID string) error {
	q := url.Values{}
	q.Set("gatewayId", gatewayID)
	q.Set("status", "DEPLOYED")
	path := "/api/v0.9/graphql-apis/" + url.PathEscape(graphqlAPIID) + "/deployments?" + q.Encode()

	resp, err := s.platformAPIGet(ctx, token, path)
	if err != nil {
		return fmt.Errorf("fetch graphql api deployments: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		return &graphqlInvokeLookupError{status: http.StatusNotFound, code: "GRAPHQL_API_NOT_FOUND", message: "the GraphQL API was not found"}
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch graphql api deployments: unexpected status %d", resp.StatusCode)
	}
	var out deploymentListLookupResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, platformAPILookupMaxBytes+1)).Decode(&out); err != nil {
		return fmt.Errorf("decode deployments response: %w", err)
	}
	for _, d := range out.List {
		if d.GatewayID == gatewayID && d.Status == "DEPLOYED" {
			return nil
		}
	}
	return &graphqlInvokeLookupError{
		status: http.StatusUnprocessableEntity, code: "GRAPHQL_API_NOT_DEPLOYED",
		message: "the GraphQL API is not deployed to the selected gateway",
	}
}

// buildGraphQLInvokeURL mirrors InvokeUrlPanel.tsx's buildInvokeUrl exactly:
// `{endpoint}{context}` with a scheme ensured, slashes normalized, and any
// `$version` placeholder in context resolved.
func buildGraphQLInvokeURL(endpoint, apiContext, version string) string {
	trimmed := strings.TrimSpace(endpoint)
	base := trimmed
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		base = "https://" + trimmed
	}
	base = strings.TrimRight(base, "/")

	resolvedContext := apiContext
	if resolvedContext == "" {
		resolvedContext = "/"
	}
	resolvedContext = graphqlVersionPlaceholder.ReplaceAllString(resolvedContext, version)
	resolvedContext = strings.TrimSpace(resolvedContext)
	if !strings.HasPrefix(resolvedContext, "/") {
		resolvedContext = "/" + resolvedContext
	}
	return base + resolvedContext
}
