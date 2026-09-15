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

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wso2/api-platform/common/authenticators"
	commonmodels "github.com/wso2/api-platform/common/models"
	"github.com/wso2/api-platform/httpkit/httputil"
)

// RouteKeyResolver maps an MCP tool call to the REST route key(s) that govern
// the equivalent operation. ok=false means the call could not be mapped.
type RouteKeyResolver func(tool string, args json.RawMessage) ([]string, bool)

// mcpAuthz is the authorization core shared by every MCP endpoint. It performs
// the HTTP-layer check (ScopeGate), the tool-layer check (authorize), and
// writes the RFC 6750 challenge on denial.
type mcpAuthz struct {
	// resourceRoles maps a route key ("GET /config_dump") to the roles allowed
	// to call it. A missing or empty entry denies.
	resourceRoles map[string][]string

	// roleMapping translates local role names to the IdP's scope names for
	// anything sent to a client. Internal decisions use local names only.
	roleMapping map[string][]string

	// resourceMetadataURL is the RFC 9728 document advertised in challenges.
	// Empty omits it.
	resourceMetadataURL string

	maxRequestBytes int64
	logger          *slog.Logger

	// routeKeysForCall is assigned by the owning handler after construction.
	// newMcpAuthz installs a default that maps nothing, so an unassigned
	// resolver fails closed rather than panicking.
	routeKeysForCall RouteKeyResolver
}

// mcpAuthzParams collects newMcpAuthz's dependencies.
type mcpAuthzParams struct {
	ResourceRoles       map[string][]string
	RoleMapping         map[string][]string
	ResourceMetadataURL string
	MaxRequestBytes     int64
	Logger              *slog.Logger
}

// newMcpAuthz builds an authorization core. A non-positive MaxRequestBytes
// falls back to the SDK default.
func newMcpAuthz(p mcpAuthzParams) *mcpAuthz {
	a := &mcpAuthz{
		resourceRoles:       p.ResourceRoles,
		roleMapping:         p.RoleMapping,
		resourceMetadataURL: p.ResourceMetadataURL,
		maxRequestBytes:     p.MaxRequestBytes,
		logger:              p.Logger,
		routeKeysForCall: func(string, json.RawMessage) ([]string, bool) {
			return nil, false
		},
	}
	if a.maxRequestBytes <= 0 {
		a.maxRequestBytes = mcp.DefaultMaxRequestBodyBytes
	}
	return a
}

// mcpCallerKeyType is the context key for mcpCaller. Unexported, so only this
// package can attach a caller to a request.
type mcpCallerKeyType struct{}

// mcpCaller is the authorization decision ScopeGate attaches to a request. Its
// absence is meaningful: a tool that cannot find it denies, which proves the
// gate ran.
type mcpCaller struct {
	// Auth is the verified authentication context. UserID is required by the
	// API-key tools; see callerIdentity.
	Auth commonmodels.AuthContext
	// Skipped is true when no authenticator is configured or the request
	// matched a skip path. It exempts the role check only, not identity.
	Skipped bool
}

// withMcpCaller attaches the gate's decision to the request context.
func withMcpCaller(ctx context.Context, c mcpCaller) context.Context {
	return context.WithValue(ctx, mcpCallerKeyType{}, c)
}

// mcpCallerFromContext reads the gate's decision back. ok=false means the gate
// did not run.
func mcpCallerFromContext(ctx context.Context) (mcpCaller, bool) {
	c, ok := ctx.Value(mcpCallerKeyType{}).(mcpCaller)
	return c, ok
}

// rolesFor returns the roles allowed on a route key. ok=false means the route
// is absent or has no roles; both deny.
func (a *mcpAuthz) rolesFor(key string) ([]string, bool) {
	roles, ok := a.resourceRoles[key]
	if !ok || len(roles) == 0 {
		return nil, false
	}
	return roles, true
}

// authorize is the tool-layer check. It repeats the decision ScopeGate made
// at the HTTP layer so a tool cannot execute if the handler is ever mounted
// without the gate.
func (a *mcpAuthz) authorize(ctx context.Context, key string) error {
	caller, ok := mcpCallerFromContext(ctx)
	if !ok {
		a.logger.Error("MCP tool reached without an authorization decision — denying",
			slog.String("route", key))
		return fmt.Errorf("this operation is not available")
	}
	if caller.Skipped {
		return nil
	}
	allowed, ok := a.rolesFor(key)
	if !ok {
		a.logger.Error("MCP operation has no entry in the route role map — denying",
			slog.String("route", key))
		return fmt.Errorf("this operation is not available")
	}
	if !hasAnyRole(caller.Auth.Roles, allowed) {
		// Reported in the IdP's vocabulary: that is what the client must request.
		return fmt.Errorf(
			"insufficient scope: this operation requires one of [%s]",
			strings.Join(a.scopesFor(allowed), " "))
	}
	return nil
}

// scopesFor translates local role names into the IdP's scope names. Every
// scope name sent to a client passes through here.
func (a *mcpAuthz) scopesFor(roles []string) []string {
	return authenticators.MapRolesToScopes(a.roleMapping, roles)
}

// callerIdentity returns the verified caller for user-scoped operations (the
// API-key tools). It fails closed on a missing context or an empty UserID,
// even when Skipped is set: an empty UserID would act as no filter at all in
// APIKeyService.
func (a *mcpAuthz) callerIdentity(ctx context.Context) (*commonmodels.AuthContext, error) {
	caller, ok := mcpCallerFromContext(ctx)
	if !ok {
		a.logger.Error("MCP key tool reached without an authorization decision — denying")
		return nil, fmt.Errorf("this operation is not available")
	}
	if strings.TrimSpace(caller.Auth.UserID) == "" {
		a.logger.Warn("MCP key tool denied: no authenticated user identity on the request",
			slog.Bool("authz_skipped", caller.Skipped))
		return nil, fmt.Errorf(
			"this operation requires an authenticated user identity; API keys are scoped to the user who created them")
	}
	auth := caller.Auth
	return &auth, nil
}

// hasAnyRole reports whether held contains any of allowed. The role list is
// an allow-list, not a hierarchy.
func hasAnyRole(held, allowed []string) bool {
	for _, a := range allowed {
		if slices.Contains(held, a) {
			return true
		}
	}
	return false
}

// jsonRPCPeek is the subset of a JSON-RPC message needed for an authorization
// decision.
type jsonRPCPeek struct {
	Method string `json:"method"`
	Params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"params"`
}

// ScopeGate is the HTTP-layer authorization check for an MCP endpoint.
//
// A denial inside a tool becomes a JSON-RPC error in an HTTP 200, which MCP
// clients treat as a tool failure rather than a reason to re-authorize. Only a
// real 403 carrying an RFC 6750 challenge makes a client run the OAuth flow
// again, so the check must happen here, before the SDK writes a status.
//
// Must be installed inside the authentication and baseline authorization
// middleware, so an AuthContext is present.
func (a *mcpAuthz) ScopeGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read the auth context regardless of Skipped: the API-key tools need
		// the UserID even when role enforcement is off.
		caller := mcpCaller{Skipped: authenticators.GetAuthzSkip(r)}
		if ac, ok := authenticators.GetAuthContext(r); ok {
			caller.Auth = ac
		}

		// Defence in depth; the mux registers POST only. RFC 9110 requires
		// Allow on a 405.
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			httputil.WriteError(w, http.StatusMethodNotAllowed,
				"method_not_allowed", "Only POST is supported on this endpoint.")
			return
		}

		// Enforce the size limit during the read, so it holds for bodies
		// without a Content-Length.
		r.Body = http.MaxBytesReader(w, r.Body, a.maxRequestBytes)
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

		// An MCP POST carries exactly one JSON-RPC message. Anything else is
		// a deny, not a pass-through.
		var peek jsonRPCPeek
		if err := json.Unmarshal(bytes.TrimSpace(body), &peek); err != nil {
			httputil.WriteError(w, http.StatusBadRequest,
				"bad_request", "Request body must be a single JSON-RPC message.")
			return
		}

		if peek.Method == "tools/call" && peek.Params.Name != "" && !caller.Skipped {
			if keys, ok := a.routeKeysForCall(peek.Params.Name, peek.Params.Arguments); ok {
				if allowed, permitted := a.evaluate(caller.Auth.Roles, keys); !permitted {
					// Log the scopes sent in the challenge, not the local roles.
					a.logger.Info("MCP tool call denied at the HTTP gate",
						slog.String("tool", peek.Params.Name),
						slog.Any("required_scopes", a.scopesFor(allowed)))
					a.writeInsufficientScope(w, allowed)
					return
				}
			}
			// An unmappable call (unknown tool, unresolvable kind) is passed
			// through: the SDK or the tool reports the specific error, and every
			// tool re-checks authorization before acting.
		}

		next.ServeHTTP(w, r.WithContext(withMcpCaller(r.Context(), caller)))
	})
}

// evaluate reports whether held satisfies any of keys. On denial it returns
// the union of roles that would have been accepted, for the challenge's scope
// hint. A key absent from the map contributes nothing.
func (a *mcpAuthz) evaluate(held []string, keys []string) ([]string, bool) {
	seen := map[string]struct{}{}
	var required []string
	for _, k := range keys {
		allowed, ok := a.rolesFor(k)
		if !ok {
			a.logger.Error("MCP call maps to a route with no role mapping",
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

// writeInsufficientScope writes the 403 and RFC 6750 challenge for a scope
// shortfall. Role names are translated to IdP scope names here, the single
// point every challenge passes through.
func (a *mcpAuthz) writeInsufficientScope(w http.ResponseWriter, requiredRoles []string) {
	params := []string{
		`error="insufficient_scope"`,
		`error_description="The access token lacks a scope required for this operation."`,
	}
	if required := a.scopesFor(requiredRoles); len(required) > 0 {
		params = append(params, fmt.Sprintf("scope=%q", strings.Join(required, " ")))
	}
	if a.resourceMetadataURL != "" {
		params = append(params, fmt.Sprintf("resource_metadata=%q", a.resourceMetadataURL))
	}
	w.Header().Set("WWW-Authenticate", "Bearer "+strings.Join(params, ", "))
	httputil.WriteError(w, http.StatusForbidden,
		"insufficient_scope", "The access token lacks the scope required for this operation.")
}
