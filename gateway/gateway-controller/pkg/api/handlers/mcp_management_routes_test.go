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
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	commonmodels "github.com/wso2/api-platform/common/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/secrets"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/certificate"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/subscription"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// routesAllServices wires every optional service so the registry is complete:
// all seven kinds, API keys on the key-bearing ones, and both action tools.
func routesAllServices() McpHandlerParams {
	return McpHandlerParams{
		Logger:              toolsDiscard,
		SecretService:       &secrets.SecretService{},
		APIKeyService:       &utils.APIKeyService{},
		CertificateService:  &certificate.CertificateService{},
		SubscriptionService: &subscription.SubscriptionService{},
	}
}

func routesArgs(t *testing.T, in any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(in)
	require.NoError(t, err)
	return raw
}

// Placeholder spellings must match generateAuthConfig's role map exactly; a
// mismatch is a key with no roles, which denies every caller.
func TestRouteKeyBuildsCollectionAndItemKeys(t *testing.T) {
	ops := &kindOps{Collection: "/llm-providers"}

	assert.Equal(t, "POST /llm-providers", routeKey(http.MethodPost, ops, false))
	assert.Equal(t, "GET /llm-providers", routeKey(http.MethodGet, ops, false))
	assert.Equal(t, "PUT /llm-providers/{id}", routeKey(http.MethodPut, ops, true))
	assert.Equal(t, "DELETE /llm-providers/{id}", routeKey(http.MethodDelete, ops, true))
}

func TestKeyRouteKeyBuildsTheAPIKeySubResourceKeys(t *testing.T) {
	ops := &kindOps{Collection: "/agents"}

	assert.Equal(t, "POST /agents/{id}/api-keys", keyRouteKey(http.MethodPost, ops, ""))
	assert.Equal(t, "PUT /agents/{id}/api-keys/{apiKeyName}", keyRouteKey(http.MethodPut, ops, "/{apiKeyName}"))
	assert.Equal(t, "POST /agents/{id}/api-keys/{apiKeyName}/regenerate",
		keyRouteKey(http.MethodPost, ops, "/{apiKeyName}/regenerate"))
}

// apiKeyRouteSuffixes must list exactly the routes the key tools resolve to.
func TestAPIKeyRouteSuffixesAreExactlyWhatTheKeyToolsReach(t *testing.T) {
	h := newMcpHandler(routesAllServices())
	calls := []struct {
		tool string
		in   any
	}{
		{"wso2_apip_gw_issue_api_key", issueKeyInput{Kind: "RestApi", ID: "orders"}},
		{"wso2_apip_gw_list_api_keys", listKeysInput{Kind: "RestApi", ID: "orders"}},
		{"wso2_apip_gw_rotate_api_key", rotateKeyInput{Kind: "RestApi", ID: "orders", KeyName: "k"}},
		{"wso2_apip_gw_rotate_api_key", rotateKeyInput{Kind: "RestApi", ID: "orders", KeyName: "k", ApiKey: "supplied"}},
		{"wso2_apip_gw_revoke_api_key", revokeKeyInput{Kind: "RestApi", ID: "orders", KeyName: "k"}},
	}
	var reached []string
	for _, c := range calls {
		keys, ok := h.routeKeysForCall(c.tool, routesArgs(t, c.in))
		require.Truef(t, ok, "%s must map", c.tool)
		reached = append(reached, keys...)
	}

	var declared []string
	for _, r := range apiKeyRouteSuffixes {
		declared = append(declared, keyRouteKey(r.Method, h.kinds[models.KindRestApi], r.Suffix))
	}
	assert.ElementsMatch(t, reached, declared)
}

func TestMCPCertificateRouteKeys(t *testing.T) {
	assert.Equal(t, []string{
		"DELETE /certificates/{id}",
		"GET /certificates",
		"POST /certificates",
		"POST /certificates/reload",
	}, MCPCertificateRouteKeys(), "sorted, and one key per action")
}

// Every (type, action, id-presence) combination the subscription tool can
// resolve, produced by resolveSubscriptionAction exactly as a real call is.
func TestMCPSubscriptionRouteKeys(t *testing.T) {
	assert.Equal(t, []string{
		"DELETE /subscription-plans/{planId}",
		"DELETE /subscriptions/{subscriptionId}",
		"GET /subscription-plans",
		"GET /subscription-plans/{planId}",
		"GET /subscriptions",
		"GET /subscriptions/{subscriptionId}",
		"POST /subscription-plans",
		"POST /subscriptions",
		"PUT /subscription-plans/{planId}",
		"PUT /subscriptions/{subscriptionId}",
	}, MCPSubscriptionRouteKeys())
}

// A tool that was not registered contributes no route keys, so its roles do
// not reach the advertised set of a gateway that cannot serve it.
func TestCertAndSubscriptionRouteKeysFollowTheRegisteredTools(t *testing.T) {
	assert.Empty(t, (&McpHandler{}).certAndSubscriptionRouteKeys())

	assert.Equal(t, MCPCertificateRouteKeys(),
		(&McpHandler{certificateService: &certificate.CertificateService{}}).certAndSubscriptionRouteKeys())

	assert.Equal(t, MCPSubscriptionRouteKeys(),
		(&McpHandler{subscriptionService: &subscription.SubscriptionService{}}).certAndSubscriptionRouteKeys())

	both := (&McpHandler{
		certificateService:  &certificate.CertificateService{},
		subscriptionService: &subscription.SubscriptionService{},
	}).certAndSubscriptionRouteKeys()
	assert.ElementsMatch(t, append(MCPCertificateRouteKeys(), MCPSubscriptionRouteKeys()...), both)
}

func TestRouteKeysForCallMapsEveryTool(t *testing.T) {
	h := newMcpHandler(routesAllServices())

	tests := []struct {
		name string
		tool string
		in   any
		want []string
	}{
		{"deploy create", "wso2_apip_gw_deploy_api", deployInput{Kind: "Agent", Yaml: "x"}, []string{"POST /agents"}},
		{"deploy update", "wso2_apip_gw_deploy_api", deployInput{Kind: "llm-provider", Yaml: "x", ID: "p"},
			[]string{"PUT /llm-providers/{id}"}},
		{"apply_config create", "wso2_apip_gw_apply_config", deployInput{Kind: "LlmProviderTemplate", Yaml: "x"},
			[]string{"POST /llm-provider-templates"}},
		{"undeploy", "wso2_apip_gw_undeploy_api", deleteInput{Kind: "LlmProxy", ID: "p"},
			[]string{"DELETE /llm-proxies/{id}"}},
		{"delete_config", "wso2_apip_gw_delete_config", deleteInput{Kind: "Secret", ID: "s"},
			[]string{"DELETE /secrets/{id}"}},
		{"get a routable kind", "wso2_apip_gw_get_resource", getInput{Kind: "Mcp", ID: "m"},
			[]string{"GET /mcp-proxies/{id}"}},
		{"get a config kind", "wso2_apip_gw_get_resource", getInput{Kind: "secret", ID: "s"},
			[]string{"GET /secrets/{id}"}},
		{"list one kind", "wso2_apip_gw_list_resources", listInput{Kind: "RestApi"},
			[]string{"GET /rest-apis"}},
		{"issue key", "wso2_apip_gw_issue_api_key", issueKeyInput{Kind: "LlmProvider", ID: "p"},
			[]string{"POST /llm-providers/{id}/api-keys"}},
		{"list keys", "wso2_apip_gw_list_api_keys", listKeysInput{Kind: "Agent", ID: "a"},
			[]string{"GET /agents/{id}/api-keys"}},
		{"rotate generates", "wso2_apip_gw_rotate_api_key", rotateKeyInput{Kind: "LlmProxy", ID: "p", KeyName: "k"},
			[]string{"POST /llm-proxies/{id}/api-keys/{apiKeyName}/regenerate"}},
		// Whitespace is not a key value, so this is a regeneration too: the
		// same rotateIsInjection decision the tool makes.
		{"rotate with a blank key generates", "wso2_apip_gw_rotate_api_key",
			rotateKeyInput{Kind: "RestApi", ID: "o", KeyName: "k", ApiKey: "   "},
			[]string{"POST /rest-apis/{id}/api-keys/{apiKeyName}/regenerate"}},
		{"rotate installs a supplied key", "wso2_apip_gw_rotate_api_key",
			rotateKeyInput{Kind: "RestApi", ID: "o", KeyName: "k", ApiKey: "supplied-value"},
			[]string{"PUT /rest-apis/{id}/api-keys/{apiKeyName}"}},
		{"revoke key", "wso2_apip_gw_revoke_api_key", revokeKeyInput{Kind: "RestApi", ID: "o", KeyName: "k"},
			[]string{"DELETE /rest-apis/{id}/api-keys/{apiKeyName}"}},
		{"certificate action", "wso2_apip_gw_manage_certificates", manageCertificatesInput{Action: "Refresh"},
			[]string{"POST /certificates/reload"}},
		{"subscription action", "wso2_apip_gw_manage_subscriptions",
			manageSubscriptionsInput{Type: "plan", Action: "apply", ID: "p1"},
			[]string{"PUT /subscription-plans/{planId}"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := h.routeKeysForCall(tt.tool, routesArgs(t, tt.in))
			require.True(t, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

// The cross-kind list is admitted by any readable kind, so it maps to the GET
// collection key of every registered kind, in canonical order.
func TestRouteKeysForCallListWithoutKindCoversEveryRegisteredKind(t *testing.T) {
	h := newMcpHandler(routesAllServices())

	got, ok := h.routeKeysForCall("wso2_apip_gw_list_resources", routesArgs(t, listInput{}))

	require.True(t, ok)
	assert.Equal(t, []string{
		"GET /agents",
		"GET /llm-providers",
		"GET /llm-provider-templates",
		"GET /llm-proxies",
		"GET /mcp-proxies",
		"GET /rest-apis",
		"GET /secrets",
	}, got)

	// A kind this gateway did not register is not part of the set.
	bare := newMcpHandler(McpHandlerParams{Logger: toolsDiscard})
	got, ok = bare.routeKeysForCall("wso2_apip_gw_list_resources", routesArgs(t, listInput{}))
	require.True(t, ok)
	assert.NotContains(t, got, "GET /secrets")
	assert.Len(t, got, 6)
}

// Calls that cannot be mapped return ok=false and no keys.
func TestRouteKeysForCallUnmappableCalls(t *testing.T) {
	h := newMcpHandler(routesAllServices())
	bare := newMcpHandler(McpHandlerParams{Logger: toolsDiscard})

	tests := []struct {
		name string
		h    *McpHandler
		tool string
		args json.RawMessage
	}{
		{"unknown tool", h, "wso2_apip_gw_drop_everything", routesArgs(t, map[string]any{})},
		{"unknown kind", h, "wso2_apip_gw_get_resource", routesArgs(t, getInput{Kind: "Widget", ID: "w"})},
		{"undeploy a config kind", h, "wso2_apip_gw_undeploy_api", routesArgs(t, deleteInput{Kind: "Secret", ID: "s"})},
		{"delete_config a routable kind", h, "wso2_apip_gw_delete_config", routesArgs(t, deleteInput{Kind: "Mcp", ID: "m"})},
		{"list an unknown kind", h, "wso2_apip_gw_list_resources", routesArgs(t, listInput{Kind: "Widget"})},
		{"issue on a kind without keys", h, "wso2_apip_gw_issue_api_key", routesArgs(t, issueKeyInput{Kind: "Mcp", ID: "m"})},
		{"list keys on a config kind", h, "wso2_apip_gw_list_api_keys", routesArgs(t, listKeysInput{Kind: "Secret", ID: "s"})},
		{"rotate on an unknown kind", h, "wso2_apip_gw_rotate_api_key", routesArgs(t, rotateKeyInput{Kind: "Widget"})},
		{"revoke with no key service", bare, "wso2_apip_gw_revoke_api_key",
			routesArgs(t, revokeKeyInput{Kind: "RestApi", ID: "o", KeyName: "k"})},
		{"unknown certificate action", h, "wso2_apip_gw_manage_certificates",
			routesArgs(t, manageCertificatesInput{Action: "rotate"})},
		{"unknown subscription type", h, "wso2_apip_gw_manage_subscriptions",
			routesArgs(t, manageSubscriptionsInput{Type: "Application", Action: "list"})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.h.routeKeysForCall(tt.tool, tt.args)
			assert.False(t, ok)
			assert.Nil(t, got)
		})
	}
}

// Arguments that do not decode into the tool's input shape are unmappable, for
// every tool, rather than decoded as zero values and authorized as something.
func TestRouteKeysForCallRejectsMalformedArguments(t *testing.T) {
	h := newMcpHandler(routesAllServices())
	for _, tool := range []string{
		"wso2_apip_gw_deploy_api", "wso2_apip_gw_apply_config",
		"wso2_apip_gw_undeploy_api", "wso2_apip_gw_delete_config",
		"wso2_apip_gw_get_resource", "wso2_apip_gw_list_resources",
		"wso2_apip_gw_issue_api_key", "wso2_apip_gw_list_api_keys",
		"wso2_apip_gw_rotate_api_key", "wso2_apip_gw_revoke_api_key",
		"wso2_apip_gw_manage_certificates", "wso2_apip_gw_manage_subscriptions",
	} {
		t.Run(tool, func(t *testing.T) {
			got, ok := h.routeKeysForCall(tool, json.RawMessage(`{"kind": 42, "action": [1]`))
			assert.False(t, ok)
			assert.Nil(t, got)
		})
	}
}

func TestKeyRouteKeysFor(t *testing.T) {
	h := newMcpHandler(routesAllServices())

	got, ok := h.keyRouteKeysFor("llm_proxy", http.MethodDelete, "/{apiKeyName}")
	require.True(t, ok)
	assert.Equal(t, []string{"DELETE /llm-proxies/{id}/api-keys/{apiKeyName}"}, got)

	for _, kind := range []string{"Mcp", "LlmProviderTemplate", "Secret", "Widget"} {
		got, ok := h.keyRouteKeysFor(kind, http.MethodGet, "")
		assert.Falsef(t, ok, "%s bears no keys", kind)
		assert.Nil(t, got)
	}
}

// A caller granted only the keys the gate resolved must pass each tool's own check.
func TestGateKeysAreTheKeysEachToolAuthorizes(t *testing.T) {
	calls := []struct {
		name string
		tool string
		in   any
		run  func(h *McpHandler, ctx context.Context) error
	}{
		{"deploy create", "wso2_apip_gw_deploy_api", deployInput{Kind: "RestApi", Yaml: "x"},
			func(h *McpHandler, ctx context.Context) error {
				_, _, err := h.deployAPI(ctx, nil, deployInput{Kind: "RestApi", Yaml: "x"})
				return err
			}},
		{"deploy update", "wso2_apip_gw_deploy_api", deployInput{Kind: "Mcp", Yaml: "x", ID: "m"},
			func(h *McpHandler, ctx context.Context) error {
				_, _, err := h.deployAPI(ctx, nil, deployInput{Kind: "Mcp", Yaml: "x", ID: "m"})
				return err
			}},
		{"apply_config", "wso2_apip_gw_apply_config", deployInput{Kind: "Secret", Yaml: "x", ID: "s"},
			func(h *McpHandler, ctx context.Context) error {
				_, _, err := h.applyConfig(ctx, nil, deployInput{Kind: "Secret", Yaml: "x", ID: "s"})
				return err
			}},
		{"undeploy", "wso2_apip_gw_undeploy_api", deleteInput{Kind: "RestApi", ID: "o", Confirm: true},
			func(h *McpHandler, ctx context.Context) error {
				_, _, err := h.undeployAPI(ctx, nil, deleteInput{Kind: "RestApi", ID: "o", Confirm: true})
				return err
			}},
		{"delete_config", "wso2_apip_gw_delete_config", deleteInput{Kind: "Secret", ID: "s", Confirm: true},
			func(h *McpHandler, ctx context.Context) error {
				_, _, err := h.deleteConfig(ctx, nil, deleteInput{Kind: "Secret", ID: "s", Confirm: true})
				return err
			}},
		{"get", "wso2_apip_gw_get_resource", getInput{Kind: "Mcp", ID: "m"},
			func(h *McpHandler, ctx context.Context) error {
				_, _, err := h.getResource(ctx, nil, getInput{Kind: "Mcp", ID: "m"})
				return err
			}},
		{"list one kind", "wso2_apip_gw_list_resources", listInput{Kind: "RestApi"},
			func(h *McpHandler, ctx context.Context) error {
				_, _, err := h.listResources(ctx, nil, listInput{Kind: "RestApi"})
				return err
			}},
		{"list every kind", "wso2_apip_gw_list_resources", listInput{},
			func(h *McpHandler, ctx context.Context) error {
				_, _, err := h.listResources(ctx, nil, listInput{})
				return err
			}},
		{"issue key", "wso2_apip_gw_issue_api_key", issueKeyInput{Kind: "RestApi", ID: "o"},
			func(h *McpHandler, ctx context.Context) error {
				_, _, err := h.issueAPIKey(ctx, nil, issueKeyInput{Kind: "RestApi", ID: "o"})
				return err
			}},
		{"list keys", "wso2_apip_gw_list_api_keys", listKeysInput{Kind: "RestApi", ID: "o"},
			func(h *McpHandler, ctx context.Context) error {
				_, _, err := h.listAPIKeys(ctx, nil, listKeysInput{Kind: "RestApi", ID: "o"})
				return err
			}},
		{"rotate generates", "wso2_apip_gw_rotate_api_key", rotateKeyInput{Kind: "RestApi", ID: "o", KeyName: "k"},
			func(h *McpHandler, ctx context.Context) error {
				_, _, err := h.rotateAPIKey(ctx, nil, rotateKeyInput{Kind: "RestApi", ID: "o", KeyName: "k"})
				return err
			}},
		{"rotate installs", "wso2_apip_gw_rotate_api_key",
			rotateKeyInput{Kind: "RestApi", ID: "o", KeyName: "k", ApiKey: strings.Repeat("v", 40)},
			func(h *McpHandler, ctx context.Context) error {
				_, _, err := h.rotateAPIKey(ctx, nil,
					rotateKeyInput{Kind: "RestApi", ID: "o", KeyName: "k", ApiKey: strings.Repeat("v", 40)})
				return err
			}},
		{"revoke key", "wso2_apip_gw_revoke_api_key", revokeKeyInput{Kind: "RestApi", ID: "o", KeyName: "k", Confirm: true},
			func(h *McpHandler, ctx context.Context) error {
				_, _, err := h.revokeAPIKey(ctx, nil, revokeKeyInput{Kind: "RestApi", ID: "o", KeyName: "k", Confirm: true})
				return err
			}},
	}

	for _, c := range calls {
		t.Run(c.name, func(t *testing.T) {
			f := newToolsHandler(false)
			keys, ok := f.h.routeKeysForCall(c.tool, routesArgs(t, c.in))
			require.True(t, ok)

			granted := map[string][]string{}
			for _, k := range keys {
				granted[k] = []string{"exactly-these"}
			}
			f.h.authz = newMcpAuthz(mcpAuthzParams{ResourceRoles: granted, Logger: toolsDiscard})
			ctx := withMcpCaller(context.Background(), mcpCaller{
				Auth: commonmodels.AuthContext{UserID: "alice", Roles: []string{"exactly-these"}},
			})

			// Every stub succeeds, so any error means the tool checked a different key.
			require.NoError(t, c.run(f.h, ctx),
				"the tool authorized a different key than the gate resolved: %v", keys)
		})
	}
}

// Every key a tool can resolve to must be in the advertised set.
func TestMCPRouteKeysCoversEveryReachableKey(t *testing.T) {
	h := newMcpHandler(routesAllServices())
	advertised := h.MCPRouteKeys()

	assert.True(t, sort.StringsAreSorted(advertised), "sorted")
	seen := map[string]bool{}
	for _, k := range advertised {
		assert.Falsef(t, seen[k], "duplicate key %q", k)
		seen[k] = true
	}

	var reachable []string
	for kind, ops := range h.kinds {
		reachable = append(reachable,
			routeKey(http.MethodPost, ops, false), routeKey(http.MethodGet, ops, false),
			routeKey(http.MethodGet, ops, true), routeKey(http.MethodPut, ops, true),
			routeKey(http.MethodDelete, ops, true))
		if ops.Keys != nil {
			for _, r := range apiKeyRouteSuffixes {
				reachable = append(reachable, keyRouteKey(r.Method, ops, r.Suffix))
			}
		} else {
			assert.NotContainsf(t, advertised, keyRouteKey(http.MethodPost, ops, ""),
				"%s bears no keys, so no key route is advertised for it", kind)
		}
	}
	reachable = append(reachable, MCPCertificateRouteKeys()...)
	reachable = append(reachable, MCPSubscriptionRouteKeys()...)
	for _, k := range reachable {
		assert.Containsf(t, advertised, k, "reachable key %q is not advertised", k)
	}
}

// A gateway without the optional services advertises nothing for them.
func TestMCPRouteKeysDescribesTheGatewayAsConfigured(t *testing.T) {
	keys := newMcpHandler(McpHandlerParams{Logger: toolsDiscard}).MCPRouteKeys()

	for _, k := range keys {
		assert.NotContains(t, k, "/secrets", "no secret service, no Secret routes")
		assert.NotContains(t, k, "/api-keys", "no key service, no key routes")
		assert.NotContains(t, k, "/certificates", "no certificate tool, no certificate routes")
		assert.NotContains(t, k, "/subscription", "no subscription tool, no subscription routes")
	}
	assert.Contains(t, keys, "POST /rest-apis")
}

// Baseline roles come only from advertised keys that exist in the role map.
func TestMCPBaselineRoles(t *testing.T) {
	params := routesAllServices()
	params.ResourceRoles = map[string][]string{
		"GET /rest-apis":                {"developer", "admin"},
		"POST /llm-providers":           {"admin"},
		"GET /agents/{id}/api-keys":     {"consumer"},
		"POST /certificates/reload":     {"operator"},
		"GET /subscriptions":            {"admin"},
		"GET /unrelated-admin-endpoint": {"auditor"},
	}
	h := newMcpHandler(params)

	assert.Equal(t, []string{"admin", "consumer", "developer", "operator"}, h.MCPBaselineRoles(),
		"sorted, deduplicated, and without the role granted only off the MCP surface")

	assert.Empty(t, newMcpHandler(routesAllServices()).MCPBaselineRoles(), "an empty role map grants nothing")
}
