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
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/authenticators"
	commonmodels "github.com/wso2/api-platform/common/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// newAuthzTestHandler builds an McpHandler with only the pieces the tool-layer
// authorization check needs. The authz core must be non-nil: every tool calls
// h.authz.authorize before touching a service, and a nil core is a panic rather
// than the deny these tests are asserting.
func newAuthzTestHandler(immutable bool) *McpHandler {
	lg := slog.Default()
	return &McpHandler{
		immutable: immutable,
		logger:    lg,
		authz:     newMcpAuthz(mcpAuthzParams{Logger: lg}),
	}
}

// newGateTestAuthz builds a gate over a synthetic role map. The production
// admin map grants every admin operation to the single "admin" role, so the
// step-up 403 is unreachable there; these tests use a map with more than one
// role so the shortfall path is actually exercised.
func newGateTestAuthz(resolver RouteKeyResolver) *mcpAuthz {
	a := newMcpAuthz(mcpAuthzParams{
		ResourceRoles: map[string][]string{
			"GET /widgets":  {"reader", "admin"},
			"POST /widgets": {"admin"},
		},
		RoleMapping:         map[string][]string{"admin": {"gw:admin"}},
		ResourceMetadataURL: "https://gw.example.com/.well-known/oauth-protected-resource/api/v1/mcp",
		MaxRequestBytes:     1024,
		Logger:              slog.Default(),
	})
	a.routeKeysForCall = resolver
	return a
}

// gateRequest drives one request through the gate, recording whether the
// wrapped handler was reached and what caller it saw.
func gateRequest(t *testing.T, a *mcpAuthz, method, body string, auth *commonmodels.AuthContext) (
	*httptest.ResponseRecorder, bool, mcpCaller) {
	t.Helper()

	var reached bool
	var seen mcpCaller
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		reached = true
		seen, _ = mcpCallerFromContext(r.Context())
	})

	req := httptest.NewRequest(method, "/api/management/v1/mcp", strings.NewReader(body))
	if auth != nil {
		req = req.WithContext(authenticators.WithAuthContext(req.Context(), *auth))
	}
	rec := httptest.NewRecorder()
	a.ScopeGate(next).ServeHTTP(rec, req)
	return rec, reached, seen
}

func widgetResolver(tool string, _ json.RawMessage) ([]string, bool) {
	switch tool {
	case "create_widget":
		return []string{"POST /widgets"}, true
	case "list_widgets":
		return []string{"GET /widgets"}, true
	default:
		return nil, false
	}
}

func TestScopeGateRejectsNonPost(t *testing.T) {
	a := newGateTestAuthz(widgetResolver)
	rec, reached, _ := gateRequest(t, a, http.MethodGet, "", nil)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.Equal(t, http.MethodPost, rec.Header().Get("Allow"), "RFC 9110 requires Allow on a 405")
	assert.False(t, reached, "a non-POST must never reach the SDK handler")
}

func TestScopeGateRejectsOversizedBodyWithoutNamingTheLimit(t *testing.T) {
	a := newGateTestAuthz(widgetResolver)
	huge := `{"method":"tools/call","params":{"name":"list_widgets","arguments":{"pad":"` +
		strings.Repeat("x", 4096) + `"}}}`

	rec, reached, _ := gateRequest(t, a, http.MethodPost, huge, nil)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.False(t, reached)
	// The configured ceiling is operational detail: echoing it back tells a
	// caller exactly how much to send to stay just under it.
	assert.NotContains(t, rec.Body.String(), "1024")
}

func TestScopeGateRejectsUnparseableBody(t *testing.T) {
	a := newGateTestAuthz(widgetResolver)
	rec, reached, _ := gateRequest(t, a, http.MethodPost, "not json at all", nil)

	// Unparseable input inside a protected namespace is a deny, not a
	// pass-through (GO-AUTH-017).
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.False(t, reached)
}

func TestScopeGateStepsUpOnInsufficientScope(t *testing.T) {
	a := newGateTestAuthz(widgetResolver)
	caller := &commonmodels.AuthContext{UserID: "u1", Roles: []string{"reader"}}

	rec, reached, _ := gateRequest(t, a, http.MethodPost,
		`{"method":"tools/call","params":{"name":"create_widget","arguments":{}}}`, caller)

	require.Equal(t, http.StatusForbidden, rec.Code)
	assert.False(t, reached, "a denied call must not reach the tool")

	// The challenge is the whole point: without a real 403 carrying it, an MCP
	// client reads a tool-layer denial as "the tool failed" and never
	// re-authorizes.
	challenge := rec.Header().Get("WWW-Authenticate")
	assert.Contains(t, challenge, `error="insufficient_scope"`)
	assert.Contains(t, challenge, `resource_metadata="https://gw.example.com/`)
	// Projected into the IdP's vocabulary via role_mapping — naming the local
	// role "admin" would tell the client to request something the IdP has never
	// heard of.
	assert.Contains(t, challenge, `scope="gw:admin"`)
	assert.NotContains(t, challenge, `scope="admin"`)
}

func TestScopeGateAllowsCallerHoldingTheRole(t *testing.T) {
	a := newGateTestAuthz(widgetResolver)
	caller := &commonmodels.AuthContext{UserID: "u1", Roles: []string{"admin"}}

	rec, reached, seen := gateRequest(t, a, http.MethodPost,
		`{"method":"tools/call","params":{"name":"create_widget","arguments":{}}}`, caller)

	assert.Equal(t, http.StatusOK, rec.Code)
	require.True(t, reached)
	assert.Equal(t, "u1", seen.Auth.UserID, "the tool layer needs the identity, not just the roles")
}

func TestScopeGatePassesThroughUnmappableCall(t *testing.T) {
	a := newGateTestAuthz(widgetResolver)
	caller := &commonmodels.AuthContext{UserID: "u1", Roles: []string{"reader"}}

	// An unknown tool cannot execute — the SDK answers with a protocol error,
	// and every tool re-resolves before touching a service. Passing it through
	// is what lets the model see why its call was wrong.
	_, reached, _ := gateRequest(t, a, http.MethodPost,
		`{"method":"tools/call","params":{"name":"no_such_tool","arguments":{}}}`, caller)

	assert.True(t, reached)
}

func TestScopeGatePassesThroughNonToolCalls(t *testing.T) {
	a := newGateTestAuthz(widgetResolver)
	caller := &commonmodels.AuthContext{UserID: "u1", Roles: []string{"reader"}}

	for _, method := range []string{"initialize", "tools/list"} {
		t.Run(method, func(t *testing.T) {
			_, reached, _ := gateRequest(t, a, http.MethodPost,
				`{"method":"`+method+`"}`, caller)
			assert.True(t, reached, "%s carries no authorization decision", method)
		})
	}
}

func TestScopeGateDefaultResolverDeniesNothingAndAuthorizesNothing(t *testing.T) {
	// A handler that forgets to assign a resolver must fail closed rather than
	// panic on a nil func. The default maps nothing, so every call is
	// "unmappable" and the tool layer's own authorize is what refuses.
	a := newMcpAuthz(mcpAuthzParams{Logger: slog.Default()})
	caller := &commonmodels.AuthContext{UserID: "u1", Roles: []string{"admin"}}

	_, reached, _ := gateRequest(t, a, http.MethodPost,
		`{"method":"tools/call","params":{"name":"create_widget","arguments":{}}}`, caller)

	assert.True(t, reached, "the default resolver must not panic")
}

func TestAuthorizeDeniesWhenGateNeverRan(t *testing.T) {
	a := newGateTestAuthz(widgetResolver)

	// The absence of a caller in the context is the invariant: it proves the
	// gate ran (GO-AUTH-015).
	err := a.authorize(t.Context(), "GET /widgets")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not available")
}

func TestCallerIdentityFailsClosedOnEmptyUserID(t *testing.T) {
	a := newGateTestAuthz(widgetResolver)

	// Skipped exempts the ROLE check, never identity: an empty UserID acts as
	// no filter at all in APIKeyService (GO-AUTH-020).
	ctx := withMcpCaller(t.Context(), mcpCaller{Skipped: true})
	_, err := a.callerIdentity(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "authenticated user identity")
}

// newWriteGateTestHandler builds a handler with a hand-written kind registry.
// routeKeysForCall needs only Kind, Routable and Collection on each entry, so
// no service is wired; the collection paths are spelled as in
// mcp_management_kinds.go so the expected route keys below match production.
func newWriteGateTestHandler() *McpHandler {
	return &McpHandler{kinds: map[string]*kindOps{
		models.KindRestApi: {Kind: models.KindRestApi, Routable: true, Collection: "/rest-apis"},
		models.KindMcp:     {Kind: models.KindMcp, Routable: true, Collection: "/mcp-proxies"},
		models.KindSecret:  {Kind: models.KindSecret, Routable: false, Collection: "/secrets"},
	}}
}

// The gate must derive the route key for a write from the "kind" argument
// alone — the same value the tool resolves — and never from the manifest.
func TestWriteGateResolvesKindArgument(t *testing.T) {
	h := newWriteGateTestHandler()

	tests := []struct {
		name string
		tool string
		in   deployInput
		want []string
		ok   bool
	}{
		{"deploy create", "wso2_apip_gw_deploy_api",
			deployInput{Kind: "RestApi", Yaml: "kind: RestApi\n"}, []string{"POST /rest-apis"}, true},
		{"deploy update", "wso2_apip_gw_deploy_api",
			deployInput{Kind: "RestApi", Yaml: "kind: RestApi\n", ID: "orders"}, []string{"PUT /rest-apis/{id}"}, true},
		{"deploy alias spelling", "wso2_apip_gw_deploy_api",
			deployInput{Kind: "rest-api", Yaml: "kind: RestApi\n"}, []string{"POST /rest-apis"}, true},
		{"apply create", "wso2_apip_gw_apply_config",
			deployInput{Kind: "Secret", Yaml: "kind: Secret\n"}, []string{"POST /secrets"}, true},
		{"apply update", "wso2_apip_gw_apply_config",
			deployInput{Kind: "Secret", Yaml: "kind: Secret\n", ID: "db-pass"}, []string{"PUT /secrets/{id}"}, true},

		// Class mismatches and unknown kinds are unmappable: the gate passes
		// them through and the tool reports the specific error.
		{"deploy rejects config kind", "wso2_apip_gw_deploy_api",
			deployInput{Kind: "Secret", Yaml: "kind: Secret\n"}, nil, false},
		{"apply rejects routable kind", "wso2_apip_gw_apply_config",
			deployInput{Kind: "Mcp", Yaml: "kind: Mcp\n"}, nil, false},
		{"unknown kind", "wso2_apip_gw_deploy_api",
			deployInput{Kind: "Widget", Yaml: "kind: Widget\n"}, nil, false},
		{"missing kind", "wso2_apip_gw_deploy_api",
			deployInput{Yaml: "kind: RestApi\n"}, nil, false},

		// The manifest is never opened by the gate: a yaml that names one kind
		// while the argument names another resolves on the argument. The
		// disagreement is caught later, by the service's own kind validation.
		{"manifest kind ignored", "wso2_apip_gw_deploy_api",
			deployInput{Kind: "RestApi", Yaml: "kind: Mcp\n"}, []string{"POST /rest-apis"}, true},
		{"unparseable manifest still maps", "wso2_apip_gw_deploy_api",
			deployInput{Kind: "RestApi", Yaml: ": not yaml ["}, []string{"POST /rest-apis"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, err := json.Marshal(tt.in)
			require.NoError(t, err)

			got, ok := h.routeKeysForCall(tt.tool, args)

			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

// The gate and the tool must resolve the same kind from the same argument;
// if they could disagree, the gate would authorize one collection while the
// handler wrote to another.
func TestWriteGateAgreesWithHandler(t *testing.T) {
	h := newWriteGateTestHandler()

	for _, tc := range []struct {
		tool  string
		class kindClass
		kind  string
	}{
		{"wso2_apip_gw_deploy_api", classRoutable, "RestApi"},
		{"wso2_apip_gw_deploy_api", classRoutable, "MCP"},
		{"wso2_apip_gw_apply_config", classConfig, "secret"},
	} {
		t.Run(tc.tool+"/"+tc.kind, func(t *testing.T) {
			args, err := json.Marshal(deployInput{Kind: tc.kind, Yaml: "x", ID: "r1"})
			require.NoError(t, err)

			gateKeys, ok := h.routeKeysForCall(tc.tool, args)
			require.True(t, ok)
			require.Len(t, gateKeys, 1)

			ops, err := h.resolveKind(tc.kind, tc.class)
			require.NoError(t, err)
			assert.Equal(t, routeKey(http.MethodPut, ops, true), gateKeys[0])
		})
	}
}
