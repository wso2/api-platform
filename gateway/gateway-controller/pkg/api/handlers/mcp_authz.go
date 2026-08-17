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

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"sort"
	"strings"

	"github.com/wso2/api-platform/common/authenticators"
	"github.com/wso2/go-httpkit/httputil"
)

// routeKey builds the management REST route key that governs an MCP operation
// on a kind. Keys are the RELATIVE form ("POST /llm-providers") that
// generateAuthConfig stores alongside the base-path-prefixed form, so no
// base path has to be threaded down here.
func routeKey(method string, ops *kindOps, item bool) string {
	if item {
		return method + " " + ops.Collection + "/{id}"
	}
	return method + " " + ops.Collection
}

// Caller identity, carried from the HTTP gate to the tool handlers.
type mcpCallerKeyType struct{}

// mcpCaller is the authorization decision input the gate resolved for this
// request. Its ABSENCE is meaningful: a tool handler that cannot find it denies,
// which is what proves the gate ran (GO-AUTH-015 — the invariant is enforced at
// the layer every entry point funnels through, not only in one router).
type mcpCaller struct {
	Roles []string
	// Skipped is true when the controller runs with no authenticator
	// configured, or the request matched a configured skip path. It mirrors
	// authenticators.GetAuthzSkip so MCP behaves exactly as the REST API does
	// in that mode rather than inventing its own policy.
	Skipped bool
}

func withMcpCaller(ctx context.Context, c mcpCaller) context.Context {
	return context.WithValue(ctx, mcpCallerKeyType{}, c)
}

func mcpCallerFromContext(ctx context.Context) (mcpCaller, bool) {
	c, ok := ctx.Value(mcpCallerKeyType{}).(mcpCaller)
	return c, ok
}

// rolesFor returns the local roles permitted to perform the operation behind
// routeKey. ok=false means the route is not in the map, which is a deny: the
// management API's authorization middleware is deny-by-default and MCP must not
// be more permissive than the surface it mirrors.
func (h *McpHandler) rolesFor(key string) ([]string, bool) {
	roles, ok := h.resourceRoles[key]
	if !ok || len(roles) == 0 {
		return nil, false
	}
	return roles, true
}

// authorize is the tool-layer check. The gate has already made the same
// decision at the HTTP layer; this repeats it so no tool can execute if the
// handler is ever mounted without the gate.
func (h *McpHandler) authorize(ctx context.Context, key string) error {
	caller, ok := mcpCallerFromContext(ctx)
	if !ok {
		h.logger.Error("MCP tool reached without an authorization decision — denying",
			slog.String("route", key))
		return fmt.Errorf("this operation is not available")
	}
	if caller.Skipped {
		return nil
	}
	allowed, ok := h.rolesFor(key)
	if !ok {
		h.logger.Error("MCP operation has no entry in the management route role map — denying",
			slog.String("route", key))
		return fmt.Errorf("this operation is not available")
	}
	if !hasAnyRole(caller.Roles, allowed) {
		return fmt.Errorf(
			"insufficient scope: this operation requires one of [%s]",
			strings.Join(allowed, " "))
	}
	return nil
}

// hasAnyRole reports set membership. The route role map is an explicit
// allow-list, not a hierarchy: a list like [admin, consumer] must not admit
// developer, which any ordering-based check would wrongly do.
func hasAnyRole(held, allowed []string) bool {
	for _, a := range allowed {
		if slices.Contains(held, a) {
			return true
		}
	}
	return false
}

// validateAuthorizationCoverage asserts at startup that every route key the six
// tools can produce exists in the management role map. A tool call that maps to
// a missing key is denied at runtime; finding that at boot instead of in
// production is the point.
func (h *McpHandler) validateAuthorizationCoverage() error {
	var missing []string
	for _, kind := range canonicalKinds() {
		ops, ok := h.kinds[kind]
		if !ok {
			continue
		}
		keys := []string{
			routeKey(http.MethodGet, ops, false),
			routeKey(http.MethodGet, ops, true),
			routeKey(http.MethodPost, ops, false),
			routeKey(http.MethodPut, ops, true),
			routeKey(http.MethodDelete, ops, true),
		}
		for _, k := range keys {
			if _, ok := h.rolesFor(k); !ok {
				missing = append(missing, k)
			}
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("MCP endpoint cannot start: no role mapping for management routes %v "+
			"(add them to generateAuthConfig in cmd/controller/main.go)", missing)
	}
	return nil
}

// MCPBaselineRoles is the union of every role that can call at least one tool.
// Used for the endpoint's own entry in the route role map and as the scope set
// advertised in a 401 challenge.
func (h *McpHandler) MCPBaselineRoles() []string {
	seen := map[string]struct{}{}
	for _, kind := range canonicalKinds() {
		ops, ok := h.kinds[kind]
		if !ok {
			continue
		}
		for _, m := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete} {
			for _, item := range []bool{false, true} {
				if roles, ok := h.rolesFor(routeKey(m, ops, item)); ok {
					for _, r := range roles {
						seen[r] = struct{}{}
					}
				}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for r := range seen {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// HTTP-layer gate
// jsonRPCPeek is the minimum needed to route an authorization decision. The
// rest of the envelope is the SDK's business.
type jsonRPCPeek struct {
	Method string `json:"method"`
	Params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"params"`
}

// ScopeGate is the authoritative, HTTP-layer authorization check for the MCP
// endpoint.
//
// It exists because MCP authorization is transport-level. A denial made inside
// a tool handler becomes a JSON-RPC error inside HTTP 200; clients read that as
// "the tool failed" and never re-authorize. To make a client run the OAuth flow
// again for the missing scope, the server must answer the HTTP request itself
// with 403 and an RFC 6750 section 3.1 challenge — only an http.Handler can do
// that, before the SDK writes a status.
//
// It must be installed INSIDE the authentication middleware, so an AuthContext
// is present, and inside the baseline AuthorizationMiddleware, so a caller with
// no business at this endpoint is already gone.
func (h *McpHandler) ScopeGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caller := mcpCaller{Skipped: authenticators.GetAuthzSkip(r)}
		if !caller.Skipped {
			if ac, ok := authenticators.GetAuthContext(r); ok {
				caller.Roles = ac.Roles
			}
		}

		// MCP 2026-07-28 has a single POST endpoint. The mux only registers
		// POST, so this is defence in depth; RFC 9110 requires Allow on a 405.
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			httputil.WriteError(w, http.StatusMethodNotAllowed,
				"method_not_allowed", "Only POST is supported on this endpoint.")
			return
		}

		// Bound the read before buffering. MaxBytesReader makes the ceiling
		// enforced during the read, so it holds for chunked and HTTP/2 bodies
		// with no Content-Length.
		r.Body = http.MaxBytesReader(w, r.Body, h.maxRequestBytes)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				// Never echo the configured limit back.
				httputil.WriteError(w, http.StatusRequestEntityTooLarge,
					"payload_too_large", "Request body exceeds the maximum allowed size.")
				return
			}
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", "Malformed request.")
			return
		}
		// The SDK must still be able to read the body.
		r.Body = io.NopCloser(bytes.NewReader(body))

		// The body of an MCP POST MUST be a single JSON-RPC request or
		// notification (batching was removed from the protocol). Anything else
		// is unparseable input inside a protected namespace, which is a deny,
		// not a pass-through (GO-AUTH-017).
		var peek jsonRPCPeek
		if err := json.Unmarshal(bytes.TrimSpace(body), &peek); err != nil {
			httputil.WriteError(w, http.StatusBadRequest,
				"bad_request", "Request body must be a single JSON-RPC message.")
			return
		}

		if peek.Method == "tools/call" && peek.Params.Name != "" && !caller.Skipped {
			if keys, ok := h.routeKeysForCall(peek.Params.Name, peek.Params.Arguments); ok {
				if allowed, permitted := h.evaluate(caller.Roles, keys); !permitted {
					h.logger.Info("MCP tool call denied at the HTTP gate",
						slog.String("tool", peek.Params.Name),
						slog.Any("required_scopes", allowed))
					h.writeInsufficientScope(w, allowed)
					return
				}
			}
			// !ok means the call could not be mapped: an unknown tool, or a
			// manifest with no resolvable kind. Neither can execute — the SDK
			// answers an unknown tool with a protocol error, and every tool
			// re-resolves the kind through the same helper and returns a tool
			// error before touching a service. Passing it through is what lets
			// the model see WHY its manifest was wrong.
		}

		next.ServeHTTP(w, r.WithContext(withMcpCaller(r.Context(), caller)))
	})
}

// routeKeysForCall maps a tools/call to the management route key(s) governing
// it. More than one key is returned only for the cross-kind list_resources
// form, where the keys are alternatives: holding a role for any one of them
// admits the call, and the tool then lists only the kinds the caller may read.
func (h *McpHandler) routeKeysForCall(tool string, args json.RawMessage) ([]string, bool) {
	switch tool {
	case "deploy_api", "apply_config":
		var in deployInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, false
		}
		class := classRoutable
		if tool == "apply_config" {
			class = classConfig
		}
		env, err := readManifestEnvelope([]byte(in.Yaml))
		if err != nil {
			return nil, false
		}
		ops, err := h.resolveKind(env.Kind, class)
		if err != nil {
			return nil, false
		}
		if in.ID != "" {
			return []string{routeKey(http.MethodPut, ops, true)}, true
		}
		return []string{routeKey(http.MethodPost, ops, false)}, true

	case "undeploy_api", "delete_config":
		var in deleteInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, false
		}
		class := classRoutable
		if tool == "delete_config" {
			class = classConfig
		}
		ops, err := h.resolveKind(in.Kind, class)
		if err != nil {
			return nil, false
		}
		return []string{routeKey(http.MethodDelete, ops, true)}, true

	case "get_resource":
		var in getInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, false
		}
		ops, err := h.resolveKind(in.Kind, classAny)
		if err != nil {
			return nil, false
		}
		return []string{routeKey(http.MethodGet, ops, true)}, true

	case "list_resources":
		var in listInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, false
		}
		if in.Kind != "" {
			ops, err := h.resolveKind(in.Kind, classAny)
			if err != nil {
				return nil, false
			}
			return []string{routeKey(http.MethodGet, ops, false)}, true
		}
		keys := make([]string, 0, len(h.kinds))
		for _, kind := range canonicalKinds() {
			if ops, ok := h.kinds[kind]; ok {
				keys = append(keys, routeKey(http.MethodGet, ops, false))
			}
		}
		return keys, true

	default:
		return nil, false
	}
}

// evaluate returns the roles that would grant the call and whether the caller
// holds one. A route key absent from the map contributes nothing, so a call
// whose every key is missing is denied with an empty required set.
func (h *McpHandler) evaluate(held []string, keys []string) ([]string, bool) {
	seen := map[string]struct{}{}
	var required []string
	for _, k := range keys {
		allowed, ok := h.rolesFor(k)
		if !ok {
			h.logger.Error("MCP call maps to a management route with no role mapping",
				slog.String("route", k))
			continue
		}
		if hasAnyRole(held, allowed) {
			return allowed, true
		}
		for _, r := range allowed {
			if _, dup := seen[r]; !dup {
				seen[r] = struct{}{}
				required = append(required, r)
			}
		}
	}
	sort.Strings(required)
	return required, false
}

// writeInsufficientScope emits the RFC 6750 section 3.1 challenge the MCP
// specification requires for a runtime scope shortfall.
// Scope names are not secrets — they are published in the metadata document's
// scopes_supported — so naming them leaks nothing while being the only thing
// that makes step-up possible. The body stays sterile.
func (h *McpHandler) writeInsufficientScope(w http.ResponseWriter, required []string) {
	params := []string{
		`error="insufficient_scope"`,
		`error_description="The access token lacks a scope required for this operation."`,
	}
	if len(required) > 0 {
		params = append(params, fmt.Sprintf("scope=%q", strings.Join(required, " ")))
	}
	if h.resourceMetadataURL != "" {
		params = append(params, fmt.Sprintf("resource_metadata=%q", h.resourceMetadataURL))
	}
	w.Header().Set("WWW-Authenticate", "Bearer "+strings.Join(params, ", "))
	httputil.WriteError(w, http.StatusForbidden,
		"insufficient_scope", "The access token lacks the scope required for this operation.")
}
