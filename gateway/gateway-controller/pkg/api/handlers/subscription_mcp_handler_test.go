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

package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

const mcpProxyUUID = "0000-mcp-proxy-0000-000000000000"

// seedMCPProxy stores an MCP proxy as the database returns it, with only its source configuration.
func seedMCPProxy(t *testing.T, server *APIServer, handle string, plans *[]string) *models.StoredConfig {
	t.Helper()
	proxy := api.MCPProxyConfiguration{
		ApiVersion: api.MCPProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.MCPProxyConfigurationKindMcp,
		Metadata:   api.Metadata{Name: handle},
		Spec:       api.MCPProxyConfigData{DisplayName: "Docs MCP", Version: "v1.0", SubscriptionPlans: plans},
	}
	cfg := &models.StoredConfig{
		UUID: mcpProxyUUID, Kind: models.KindMcp, Handle: handle, DisplayName: "Docs MCP", Version: "v1.0",
		SourceConfiguration: proxy, DesiredState: models.StateDeployed,
	}
	require.NoError(t, server.db.SaveConfig(cfg))
	return cfg
}

func newMCPSubscriptionTestServer(t *testing.T, plans *[]string) *APIServer {
	t.Helper()
	server := createTestAPIServer()
	attachTestEventHub(server, &mockEventHub{}, "test-gateway")
	seedMCPProxy(t, server, "docs-mcp", plans)
	seedNamedSubscriptionPlan(t, server, "plan-gold", "Gold", models.SubscriptionPlanStatusActive)
	seedNamedSubscriptionPlan(t, server, "plan-silver", "Silver", models.SubscriptionPlanStatusActive)
	return server
}

func listSubscriptionsFor(t *testing.T, server *APIServer, apiID string) (int, []api.SubscriptionResponse) {
	t.Helper()
	w, r := createTestContext("GET", "/subscriptions?apiId="+apiID, nil)
	server.ListSubscriptions(w, r, api.ListSubscriptionsParams{ApiId: &apiID})
	if w.Code != http.StatusOK {
		return w.Code, nil
	}
	var list api.SubscriptionListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	if list.Subscriptions == nil {
		return w.Code, nil
	}
	return w.Code, *list.Subscriptions
}

// MCP proxy subscriptions go through the same lifecycle as REST API ones.
func TestMCPSubscription_Lifecycle(t *testing.T) {
	server := newMCPSubscriptionTestServer(t, &[]string{"Gold", "Silver"})

	code, byHandle := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId: "docs-mcp", SubscriptionToken: "mcp-token-1", SubscriptionPlanId: ptr("plan-gold"), ExternalRefId: ptr("mcp-sub-1"),
	})
	require.Equal(t, http.StatusCreated, code)
	assert.Equal(t, mcpProxyUUID, *byHandle.ApiId)

	code, byID := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId: mcpProxyUUID, SubscriptionToken: "mcp-token-2", ExternalRefId: ptr("mcp-sub-2"),
	})
	require.Equal(t, http.StatusCreated, code)
	assert.Equal(t, mcpProxyUUID, *byID.ApiId)

	code, listed := listSubscriptionsFor(t, server, "docs-mcp")
	require.Equal(t, http.StatusOK, code)
	assert.Len(t, listed, 2)

	code, revoked := updateSubscriptionViaHandler(t, server, "mcp-sub-1", `{"status":"INACTIVE"}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, api.SubscriptionResponseStatus("INACTIVE"), *revoked.Status)

	code, resumed := updateSubscriptionViaHandler(t, server, "mcp-sub-1", `{"status":"ACTIVE"}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, api.SubscriptionResponseStatus("ACTIVE"), *resumed.Status)

	code, moved := updateSubscriptionViaHandler(t, server, "mcp-sub-1", `{"subscriptionPlanId":"plan-silver","subscriptionToken":"mcp-token-1b"}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "plan-silver", *moved.SubscriptionPlanId)

	w, r := createTestContext("DELETE", "/subscriptions/mcp-sub-1", nil)
	server.DeleteSubscription(w, r, "mcp-sub-1")
	require.Equal(t, http.StatusNoContent, w.Code)
	code, _ = getSubscriptionViaHandler(t, server, "mcp-sub-1")
	assert.Equal(t, http.StatusNotFound, code)
}

// An MCP proxy that lists plans accepts only those; one that lists none accepts any active plan.
func TestMCPSubscription_PlanMustBeOffered(t *testing.T) {
	server := newMCPSubscriptionTestServer(t, &[]string{"Gold"})

	code, body := postSubscription(t, server, "application/json",
		[]byte(`{"apiId":"docs-mcp","subscriptionToken":"mcp-token-s","subscriptionPlanId":"plan-silver"}`))
	require.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, errorMessage(t, body), `"Silver" is not enabled for this API`)

	code, _ = createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId: "docs-mcp", SubscriptionToken: "mcp-token-g", SubscriptionPlanId: ptr("plan-gold"), ExternalRefId: ptr("mcp-sub-g"),
	})
	require.Equal(t, http.StatusCreated, code)
	code, raw := putSubscription(t, server, "mcp-sub-g", "application/json", []byte(`{"subscriptionPlanId":"plan-silver"}`))
	require.Equal(t, http.StatusBadRequest, code, string(raw))

	open := newMCPSubscriptionTestServer(t, nil)
	code, _ = createSubscriptionViaHandler(t, open, api.SubscriptionCreateRequest{
		ApiId: "docs-mcp", SubscriptionToken: "mcp-token-open", SubscriptionPlanId: ptr("plan-silver"),
	})
	assert.Equal(t, http.StatusCreated, code)
}

func TestSubscription_RefusesOtherKinds(t *testing.T) {
	server := newMCPSubscriptionTestServer(t, nil)
	require.NoError(t, server.db.SaveConfig(&models.StoredConfig{
		UUID: "0000-llm-provider-0000-000000000000", Kind: models.KindLlmProvider, Handle: "openai", DesiredState: models.StateDeployed,
	}))

	code, body := postSubscription(t, server, "application/json",
		[]byte(`{"apiId":"0000-llm-provider-0000-000000000000","subscriptionToken":"llm-token"}`))
	require.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, errorMessage(t, body), "is not a REST API or MCP proxy")

	code, body = postSubscription(t, server, "application/json", []byte(`{"apiId":"no-such-proxy","subscriptionToken":"x-token"}`))
	require.Equal(t, http.StatusNotFound, code)
	assert.Contains(t, errorMessage(t, body), "REST API or MCP proxy with identifier 'no-such-proxy' not found")
}
