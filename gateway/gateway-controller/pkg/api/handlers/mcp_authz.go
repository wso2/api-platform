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
	commonmodels "github.com/wso2/api-platform/common/models"
	"github.com/wso2/api-platform/httpkit/httputil"
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

// keyRouteKey builds the route key for an api-key sub-resource. These sit two
// segments below the parent collection and carry a second placeholder, which
// routeKey cannot express.
//
// suffix is "" for the collection ("POST /rest-apis/{id}/api-keys"),
// "/{apiKeyName}" for one key, or "/{apiKeyName}/regenerate" for rotation. The
// placeholder spelling must match generateAuthConfig in cmd/controller/main.go
// exactly — a typo fails closed, which is correct but silently disables the
// tool
func keyRouteKey(method string, ops *kindOps, suffix string) string {
	return method + " " + ops.Collection + "/{id}/api-keys" + suffix
}

// apiKeyRouteSuffixes lists every api-key route suffix an MCP tool can reach,
// paired with its method. Used by MCPBaselineRoles so the advertised scope set
// includes the roles these routes need.
var apiKeyRouteSuffixes = []struct {
	Method string
	Suffix string
}{
	{http.MethodPost, ""},
	{http.MethodGet, ""},
	{http.MethodPost, "/{apiKeyName}/regenerate"},
	{http.MethodPut, "/{apiKeyName}"},
	{http.MethodDelete, "/{apiKeyName}"},
}

// Action dispatch for the certificate and subscription tools
//
// Certificates and subscriptions are not artifact kinds, so routeKey() cannot
// build their keys: certificate reload has no {id} placeholder at all, and the
// two subscription collections spell their placeholder differently
// ({subscriptionId} vs {planId}). Both tools therefore carry a literal dispatch
// table, resolved by one function each.
//
// Each resolver is called by BOTH the authorization gate (routeKeysForCall) and
// the tool handler, so the route key that was authorized is always the route key
// that executes — the same shared-predicate discipline rotateIsInjection applies
// to api-key rotation. Every key below must match generateAuthConfig in
// cmd/controller/main.go exactly; a typo fails closed, which is correct but
// silently disables the action.

// Certificate actions, canonical spellings.
const (
	certActionList   = "list"
	certActionApply  = "apply"
	certActionDelete = "delete"
	certActionReload = "reload"
)

// certAction is the management REST operation one certificate action performs.
type certAction struct {
	Action string
	// the REST route it maps to
	RouteKey string
	// whether to refuse in immutable mode
	Mutating bool
	// NeedsConfirm marks the actions requiring confirm=true.
	NeedsConfirm bool
}

var certActions = map[string]certAction{
	certActionList:   {Action: certActionList, RouteKey: "GET /certificates"},
	certActionApply:  {Action: certActionApply, RouteKey: "POST /certificates", Mutating: true},
	certActionDelete: {Action: certActionDelete, RouteKey: "DELETE /certificates/{id}", Mutating: true, NeedsConfirm: true},
	certActionReload: {Action: certActionReload, RouteKey: "POST /certificates/reload", Mutating: true, NeedsConfirm: true},
}

// certActionAliases resolves an accepted spelling to its canonical action.
// Models are inconsistent about which verb they reach for, so "upload" and
// "create" settle on "apply" before any lookup, exactly as kindAliases does for
// kind names.
var certActionAliases = map[string]string{
	"list":    certActionList,
	"apply":   certActionApply,
	"upload":  certActionApply,
	"create":  certActionApply,
	"add":     certActionApply,
	"delete":  certActionDelete,
	"remove":  certActionDelete,
	"reload":  certActionReload,
	"refresh": certActionReload,
}

// resolveCertAction resolves the certificate tool's action argument to the management REST operation it performs.
func resolveCertAction(rawAction string) (certAction, error) {
	canonical, ok := certActionAliases[normalizeToolWord(rawAction)]
	if !ok {
		return certAction{}, fmt.Errorf(
			"unknown action %q; wso2_apip_gw_manage_certificates accepts: list, apply, delete, reload",
			rawAction)
	}
	return certActions[canonical], nil
}

// Subscription types and actions, canonical spellings.
const (
	subTypeSubscription = "Subscription"
	subTypePlan         = "SubscriptionPlan"

	subActionList   = "list"
	subActionGet    = "get"
	subActionApply  = "apply"
	subActionDelete = "delete"
)

// subAction is the management REST operation one (type, action, id-presence)
type subAction struct {
	Type         string
	Action       string
	RouteKey     string
	Mutating     bool
	NeedsID      bool
	NeedsConfirm bool
}

// subCollections holds each type's collection path and the placeholder its item
// routes use. The placeholders genuinely differ, which is the whole reason this
// is a table and not a shared "/{id}" suffix.
var subCollections = map[string]struct {
	Collection  string
	Placeholder string
}{
	subTypeSubscription: {Collection: "/subscriptions", Placeholder: "{subscriptionId}"},
	subTypePlan:         {Collection: "/subscription-plans", Placeholder: "{planId}"},
}

var subTypeAliases = map[string]string{
	"subscription":     subTypeSubscription,
	"subscriptions":    subTypeSubscription,
	"subscriptionplan": subTypePlan,
	"plan":             subTypePlan,
	"plans":            subTypePlan,
}

var subActionAliases = map[string]string{
	"list":   subActionList,
	"get":    subActionGet,
	"read":   subActionGet,
	"apply":  subActionApply,
	"create": subActionApply,
	"update": subActionApply,
	"delete": subActionDelete,
	"remove": subActionDelete,
}

// resolveSubscriptionAction resolves the subscription tool's arguments to the
// management REST operation they perform. Only apply depends on the id, and
// only on whether one was supplied: present means update, absent means create,
// which is the same dispatch-on-argument shape write() uses for manifests.
func resolveSubscriptionAction(rawType, rawAction, id string) (subAction, error) {
	// Get the resources Type
	resourceType, ok := subTypeAliases[normalizeToolWord(rawType)]
	if !ok {
		return subAction{}, fmt.Errorf(
			"unknown type %q; wso2_apip_gw_manage_subscriptions accepts: %s, %s",
			rawType, subTypeSubscription, subTypePlan)
	}
	// Get the resources Action
	action, ok := subActionAliases[normalizeToolWord(rawAction)]
	if !ok {
		return subAction{}, fmt.Errorf(
			"unknown action %q; wso2_apip_gw_manage_subscriptions accepts: list, get, apply, delete",
			rawAction)
	}

	// Get the path(contains collection and placeholder) from the Collections map based on the resource type
	paths := subCollections[resourceType]
	// Create complete path for the resource
	item := paths.Collection + "/" + paths.Placeholder
	op := subAction{Type: resourceType, Action: action}

	switch action {
	case subActionList:
		op.RouteKey = http.MethodGet + " " + paths.Collection
	case subActionGet:
		op.RouteKey = http.MethodGet + " " + item
		op.NeedsID = true
	case subActionDelete:
		op.RouteKey = http.MethodDelete + " " + item
		op.Mutating, op.NeedsID, op.NeedsConfirm = true, true, true
	default: // apply
		op.Mutating = true
		if strings.TrimSpace(id) != "" {
			op.RouteKey = http.MethodPut + " " + item
		} else {
			op.RouteKey = http.MethodPost + " " + paths.Collection
		}
	}

	return op, nil
}

// normalizeToolWord folds an action or type argument to the form the alias maps
// are keyed on, so "SubscriptionPlan", "subscription_plan" and "plan" all reach
// one value before any comparison. Shares kindSeparatorStripper with
// normalizeKind so the two tolerate exactly the same spellings.
func normalizeToolWord(raw string) string {
	return strings.ToLower(kindSeparatorStripper.Replace(strings.TrimSpace(raw)))
}

// MCPCertificateRouteKeys is every route key the certificate tool can reach,
// read out of the dispatch table itself rather than restated, so it cannot
// describe a surface the resolver does not actually produce.
//
// Exported because cmd/controller asserts each key exists in
// generateAuthConfig's role map. That assertion is what catches a mistyped
// placeholder, which otherwise fails closed and silently disables an action.
func MCPCertificateRouteKeys() []string {
	keys := make([]string, 0, len(certActions))
	for _, op := range certActions {
		keys = append(keys, op.RouteKey)
	}
	sort.Strings(keys)
	return keys
}

// MCPSubscriptionRouteKeys is the same for the subscription tool. It walks every
// (type, action, id-presence) combination through resolveSubscriptionAction, so
// the two placeholder spellings are exercised exactly as a real call would
// produce them.
func MCPSubscriptionRouteKeys() []string {
	var keys []string
	for resourceType := range subCollections {
		for _, action := range []string{subActionList, subActionGet, subActionDelete} {
			if op, err := resolveSubscriptionAction(resourceType, action, ""); err == nil {
				keys = append(keys, op.RouteKey)
			}
		}
		// apply reaches two routes, chosen by whether an id was supplied.
		for _, id := range []string{"", "an-id"} {
			if op, err := resolveSubscriptionAction(resourceType, subActionApply, id); err == nil {
				keys = append(keys, op.RouteKey)
			}
		}
	}
	sort.Strings(keys)
	return keys
}

// certAndSubscriptionRouteKeys narrows the two surfaces above to the tools this
// gateway actually registered, so a handler built without one of the services
// does not advertise routes no tool can reach.
func (h *McpHandler) certAndSubscriptionRouteKeys() []string {
	var keys []string
	if h.certificateService != nil {
		keys = append(keys, MCPCertificateRouteKeys()...)
	}
	if h.subscriptionService != nil {
		keys = append(keys, MCPSubscriptionRouteKeys()...)
	}
	return keys
}

// Caller identity, carried from the HTTP gate to the tool handlers.
type mcpCallerKeyType struct{}

// mcpCaller is the authorization decision input the gate resolved for this
// request. Its ABSENCE is meaningful: a tool handler that cannot find it denies,
// which is what proves the gate ran (GO-AUTH-015 — the invariant is enforced at
// the layer every entry point funnels through, not only in one router).
type mcpCaller struct {
	// Auth is the verified authentication context, carried whole rather than
	// reduced to Roles. UserID is what APIKeyService scopes every key operation
	// on (canRegenerateAPIKey, canRevokeAPIKey, filterAPIKeysByUser), and none
	// of those guard against an empty value — an empty UserID matches every key
	// whose CreatedBy is also empty and acts as NO filter on a list. Dropping
	// the field here would be the implicit-empty-caller widening GO-AUTH-020
	// prohibits, so it is kept and callerIdentity fails closed on it.
	Auth commonmodels.AuthContext
	// Skipped is true when the controller runs with no authenticator
	// configured, or the request matched a configured skip path. It mirrors
	// authenticators.GetAuthzSkip so MCP behaves exactly as the REST API does
	// in that mode rather than inventing its own policy.
	//
	// It exempts the ROLE check only. Identity is a separate question: see
	// callerIdentity, which refuses regardless of Skipped.
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
	if !hasAnyRole(caller.Auth.Roles, allowed) {
		// Named in the IdP's vocabulary, not the gateway's: this string reaches
		// the model, and telling it to obtain a local role name it cannot
		// request from the IdP is worse than saying nothing.
		return fmt.Errorf(
			"insufficient scope: this operation requires one of [%s]",
			strings.Join(h.scopesFor(allowed), " "))
	}
	return nil
}

// scopesFor projects local roles into the IdP scope vocabulary configured in
// auth.idp.role_mapping.
//
// Every value that leaves this process naming what a caller must obtain — each
// WWW-Authenticate `scope`, each client-visible scope error — goes through
// here. Internal logs keep the local role names, which is what an operator
// reads the route role map in.
func (h *McpHandler) scopesFor(roles []string) []string {
	return authenticators.MapRolesToScopes(h.roleMapping, roles)
}

// callerIdentity returns the verified caller for operations that are scoped to
// an individual user rather than only to a role — every API-key tool.
//
// It fails closed on a missing context AND on an empty UserID, including when
// Skipped is set. Skipped exempts the role check, not identity: passing an empty
// UserID into APIKeyService would put every unidentified caller into one shared
// CreatedBy bucket and one shared per-user quota, and would turn a key listing
// into an unfiltered one. This mirrors the REST path, where
// handlerkit.ExtractAuthenticatedUser answers 401 when no authenticated user is
// present, so the api-key endpoints are already unreachable without an
// authenticator.
//
// Returning a non-nil context also keeps the process alive: every APIKeyService
// method logs user.UserID before any nil check, so a nil User is a panic.
func (h *McpHandler) callerIdentity(ctx context.Context) (*commonmodels.AuthContext, error) {
	caller, ok := mcpCallerFromContext(ctx)
	if !ok {
		h.logger.Error("MCP key tool reached without an authorization decision — denying")
		return nil, fmt.Errorf("this operation is not available")
	}
	if strings.TrimSpace(caller.Auth.UserID) == "" {
		h.logger.Warn("MCP key tool denied: no authenticated user identity on the request",
			slog.Bool("authz_skipped", caller.Skipped))
		return nil, fmt.Errorf(
			"this operation requires an authenticated user identity; API keys are scoped to the user who created them")
	}
	auth := caller.Auth
	return &auth, nil
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

// MCPRouteKeys is every management route key some MCP tool can reach, sorted
// and deduplicated.
//
// It is the single enumeration of the MCP endpoint's authorization surface:
// MCPBaselineRoles derives the advertised scope set from it, and a test asserts
// every key it returns exists in generateAuthConfig's map. That test is the
// only thing that catches a mistyped placeholder, which otherwise fails closed
// and silently disables a tool.
//
// Kinds absent from the registry and tools whose service was never wired
// contribute nothing, so the surface always describes this gateway rather than
// every gateway.
func (h *McpHandler) MCPRouteKeys() []string {
	seen := map[string]struct{}{}
	add := func(keys ...string) {
		for _, k := range keys {
			seen[k] = struct{}{}
		}
	}

	for _, kind := range canonicalKinds() {
		ops, ok := h.kinds[kind]
		if !ok {
			continue
		}
		for _, m := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete} {
			for _, item := range []bool{false, true} {
				add(routeKey(m, ops, item))
			}
		}
		// Without these the api-key routes' roles never surface, and consumer
		// would be missing from the advertised scopes_supported even though
		// "POST /mcp" admits it.
		if ops.Keys != nil {
			for _, r := range apiKeyRouteSuffixes {
				add(keyRouteKey(r.Method, ops, r.Suffix))
			}
		}
	}
	add(h.certAndSubscriptionRouteKeys()...)

	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// MCPBaselineRoles is the union of every role that can call at least one tool.
// Used for the endpoint's own entry in the route role map, and as the default
// entry set main.go projects through MapRolesToScopes to build the advertised
// scopes.
//
// It returns LOCAL ROLES and must keep doing so: it reads h.resourceRoles, and
// callers either compare against that map or project explicitly. Projecting
// here would double-translate main.go's operator-configured entry set.
func (h *McpHandler) MCPBaselineRoles() []string {
	seen := map[string]struct{}{}
	for _, key := range h.MCPRouteKeys() {
		if roles, ok := h.rolesFor(key); ok {
			for _, r := range roles {
				seen[r] = struct{}{}
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
		// The auth context is read regardless of Skipped. Skipped exempts the
		// role check; it does not mean there is no identity, and the API-key
		// tools need the UserID even when role enforcement is off.
		caller := mcpCaller{Skipped: authenticators.GetAuthzSkip(r)}
		if ac, ok := authenticators.GetAuthContext(r); ok {
			caller.Auth = ac
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
				if allowed, permitted := h.evaluate(caller.Auth.Roles, keys); !permitted {
					// Logs the scopes actually sent in the challenge, not the
					// local roles behind them — that is what a client stuck in a
					// step-up loop can be compared against.
					h.logger.Info("MCP tool call denied at the HTTP gate",
						slog.String("tool", peek.Params.Name),
						slog.Any("required_scopes", h.scopesFor(allowed)))
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

// routeKeysForCall takes the tool name and arguments the caller sent and works
// out which normal REST API operation that tool is really doing — e.g.
// "POST /rest-apis". That route key is what we check the caller's roles
// against, since the role rules are written for REST routes, not tool names.
//
// Normally one key. It is several only for a bare "list everything" call
// (wso2_apip_gw_list_resources with no kind): those are alternatives — a role
// for any one kind lets the call in, and the tool then lists only the kinds
// the caller may read. Returns false when the call can't be matched (unknown
// tool, bad arguments, unknown kind).
func (h *McpHandler) routeKeysForCall(tool string, args json.RawMessage) ([]string, bool) {
	switch tool {
	case "wso2_apip_gw_deploy_api", "wso2_apip_gw_apply_config":
		var in deployInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, false
		}
		class := classRoutable
		if tool == "wso2_apip_gw_apply_config" {
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

	case "wso2_apip_gw_undeploy_api", "wso2_apip_gw_delete_config":
		var in deleteInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, false
		}
		class := classRoutable
		if tool == "wso2_apip_gw_delete_config" {
			class = classConfig
		}
		ops, err := h.resolveKind(in.Kind, class)
		if err != nil {
			return nil, false
		}
		return []string{routeKey(http.MethodDelete, ops, true)}, true

	case "wso2_apip_gw_get_resource":
		var in getInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, false
		}
		ops, err := h.resolveKind(in.Kind, classAny)
		if err != nil {
			return nil, false
		}
		return []string{routeKey(http.MethodGet, ops, true)}, true

	case "wso2_apip_gw_list_resources":
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

	case "wso2_apip_gw_issue_api_key":
		var in issueKeyInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, false
		}
		return h.keyRouteKeysFor(in.Kind, http.MethodPost, "")

	case "wso2_apip_gw_list_api_keys":
		var in listKeysInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, false
		}
		return h.keyRouteKeysFor(in.Kind, http.MethodGet, "")

	case "wso2_apip_gw_rotate_api_key":
		var in rotateKeyInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, false
		}
		// Dispatched exactly as the tool dispatches it, through the same
		// helper: a caller-supplied key value is an injection (PUT), otherwise
		// the Gateway generates one (POST .../regenerate). One route key per
		// call, so the disjunctive evaluate() below stays correct.
		if rotateIsInjection(in) {
			return h.keyRouteKeysFor(in.Kind, http.MethodPut, "/{apiKeyName}")
		}
		return h.keyRouteKeysFor(in.Kind, http.MethodPost, "/{apiKeyName}/regenerate")

	case "wso2_apip_gw_revoke_api_key":
		var in revokeKeyInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, false
		}
		return h.keyRouteKeysFor(in.Kind, http.MethodDelete, "/{apiKeyName}")

	// The two action-dispatched tools resolve through the same functions their
	// handlers call, so the key authorized here is the key that executes. Both
	// are exact for every resolvable call: the route depends only on the
	// action, the type, and whether an id was supplied — all readable from the
	// arguments without touching a service.
	case "wso2_apip_gw_manage_certificates":
		var in manageCertificatesInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, false
		}
		op, err := resolveCertAction(in.Action)
		if err != nil {
			return nil, false
		}
		return []string{op.RouteKey}, true

	case "wso2_apip_gw_manage_subscriptions":
		var in manageSubscriptionsInput
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, false
		}
		op, err := resolveSubscriptionAction(in.Type, in.Action, in.ID)
		if err != nil {
			return nil, false
		}
		return []string{op.RouteKey}, true

	default:
		return nil, false
	}
}

// keyRouteKeysFor resolves a raw kind to the single api-key route key governing
// the call. ok=false means the kind could not be resolved or bears no keys; the
// gate passes such a call through so the tool itself can explain why, and the
// tool re-resolves through the same helper before touching a service.
func (h *McpHandler) keyRouteKeysFor(rawKind, method, suffix string) ([]string, bool) {
	ops, err := h.resolveKeyBearing(rawKind)
	if err != nil {
		return nil, false
	}
	return []string{keyRouteKey(method, ops, suffix)}, true
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
//
// requiredRoles arrives in local-role vocabulary and is projected here rather
// than at the call sites: this is the single choke point for the step-up
// challenge, so no future caller can forget the translation and emit a role
// name the IdP has never heard of.
func (h *McpHandler) writeInsufficientScope(w http.ResponseWriter, requiredRoles []string) {
	params := []string{
		`error="insufficient_scope"`,
		`error_description="The access token lacks a scope required for this operation."`,
	}
	if required := h.scopesFor(requiredRoles); len(required) > 0 {
		params = append(params, fmt.Sprintf("scope=%q", strings.Join(required, " ")))
	}
	if h.resourceMetadataURL != "" {
		params = append(params, fmt.Sprintf("resource_metadata=%q", h.resourceMetadataURL))
	}
	w.Header().Set("WWW-Authenticate", "Bearer "+strings.Join(params, ", "))
	httputil.WriteError(w, http.StatusForbidden,
		"insufficient_scope", "The access token lacks the scope required for this operation.")
}
