/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package controlplane

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

func seedBudget(t *testing.T, client *Client, id, name string, origin models.Origin) {
	t.Helper()
	budget := api.BudgetRequest{
		ApiVersion: api.BudgetRequestApiVersionV1, Kind: api.BudgetRequestKindBudget,
		Metadata: api.BudgetMetadata{Name: name},
	}
	now := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	require.NoError(t, client.db.SaveConfig(&models.StoredConfig{
		UUID: id, Kind: models.KindBudget, Handle: name, DisplayName: name, Version: "v1",
		Configuration: budget, SourceConfiguration: budget, DesiredState: models.StateDeployed,
		Origin: origin, CPSyncStatus: models.CPSyncStatusPending, CreatedAt: now, UpdatedAt: now,
	}))
}

// captureResources serves POST /gateways/{id}/resources, recording each request and
// answering with respond.
func captureResources(t *testing.T, respond func(req gatewayResourcesRequest) gatewayResourcesResponse) (*Client, *[]gatewayResourcesRequest) {
	t.Helper()
	var mu sync.Mutex
	var got []gatewayResourcesRequest
	client := newManifestTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/internal/v1/gateways/"+testGatewayID+"/resources", r.URL.Path)
		assert.Equal(t, "test-token", r.Header.Get("api-key"))
		var req gatewayResourcesRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		mu.Lock()
		got = append(got, req)
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(respond(req))
	})
	setGatewayID(client, testGatewayID)
	return client, &got
}

// setGatewayID records the gateway ID the control plane assigned on connect.
func setGatewayID(client *Client, id string) {
	client.state.mu.Lock()
	client.state.GatewayID = id
	client.state.mu.Unlock()
}

func dpids(items []gatewayResource) []string {
	out := []string{}
	for _, item := range items {
		out = append(out, item.DPID)
	}
	return out
}

func TestPushGatewayResources_SendsACompleteListAndRecordsResults(t *testing.T) {
	client, got := captureResources(t, func(req gatewayResourcesRequest) gatewayResourcesResponse {
		return gatewayResourcesResponse{Total: 2, Success: 1, Failed: 1, Resources: map[string]gatewayResourceResult{
			"budget-a": {ID: "cp-a", Status: "synced"},
			"budget-b": {Status: "failed", Error: "BUDGET_ALREADY_EXISTS"},
		}}
	})
	seedBudget(t, client, "budget-a", "team-a", models.OriginGatewayAPI)
	seedBudget(t, client, "budget-b", "team-b", models.OriginGatewayAPI)
	seedBudget(t, client, "budget-cp", "org-wide", models.OriginControlPlane)

	client.pushGatewayResources()

	require.Len(t, *got, 1)
	req := (*got)[0]
	assert.True(t, req.Complete)
	assert.Empty(t, req.Deleted)
	assert.ElementsMatch(t, []string{"budget-a", "budget-b"}, dpids(req.Resources), "only the gateway's own budgets go up")

	a, err := client.db.GetConfig("budget-a")
	require.NoError(t, err)
	assert.Equal(t, models.CPSyncStatusSuccess, a.CPSyncStatus)
	assert.Equal(t, "cp-a", a.CPArtifactID, "the control plane's ID is stored")
	b, err := client.db.GetConfig("budget-b")
	require.NoError(t, err)
	assert.Equal(t, models.CPSyncStatusFailed, b.CPSyncStatus)
	assert.Equal(t, "BUDGET_ALREADY_EXISTS", b.CPSyncInfo)
}

func TestPushGatewayResources_SendsAnEmptyCompleteListSoOfflineDeletesReachTheControlPlane(t *testing.T) {
	client, got := captureResources(t, func(gatewayResourcesRequest) gatewayResourcesResponse {
		return gatewayResourcesResponse{}
	})

	client.pushGatewayResources()

	require.Len(t, *got, 1)
	assert.True(t, (*got)[0].Complete)
	assert.NotNil(t, (*got)[0].Resources, "sent as an empty list")
	assert.Empty(t, (*got)[0].Resources)
}

func TestPushResources_OneBudgetAndADeletion(t *testing.T) {
	client, got := captureResources(t, func(req gatewayResourcesRequest) gatewayResourcesResponse {
		resp := gatewayResourcesResponse{Resources: map[string]gatewayResourceResult{}}
		for _, item := range append(req.Resources, req.Deleted...) {
			resp.Resources[item.DPID] = gatewayResourceResult{ID: "cp-" + item.DPID, Status: "synced"}
		}
		return resp
	})
	seedBudget(t, client, "budget-a", "team-a", models.OriginGatewayAPI)
	a, err := client.db.GetConfig("budget-a")
	require.NoError(t, err)

	client.pushResources(gatewayResourcesRequest{Resources: []gatewayResource{toGatewayResource(a)}},
		map[string]*models.StoredConfig{a.UUID: a})
	client.pushResources(gatewayResourcesRequest{Deleted: []gatewayResource{toGatewayResource(a)}}, nil)

	require.Len(t, *got, 2)
	assert.False(t, (*got)[0].Complete, "a single push is not complete")
	assert.Equal(t, []string{"budget-a"}, dpids((*got)[0].Resources))
	assert.Equal(t, []string{"budget-a"}, dpids((*got)[1].Deleted))
	assert.Empty(t, (*got)[1].Resources)

	stored, err := client.db.GetConfig("budget-a")
	require.NoError(t, err)
	assert.Equal(t, models.CPSyncStatusSuccess, stored.CPSyncStatus)
	assert.Equal(t, "cp-budget-a", stored.CPArtifactID)
}

func TestPushResources_ServerErrorLeavesStatus(t *testing.T) {
	client := newManifestTLSServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	setGatewayID(client, testGatewayID)
	seedBudget(t, client, "budget-a", "team-a", models.OriginGatewayAPI)

	client.pushGatewayResources()

	a, err := client.db.GetConfig("budget-a")
	require.NoError(t, err)
	assert.Equal(t, models.CPSyncStatusPending, a.CPSyncStatus, "pushed again on the next connect")
}

func TestPushResources_RetriesPerPushRetryConfig(t *testing.T) {
	var hits int32
	client := newManifestTLSServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&hits, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(gatewayResourcesResponse{Resources: map[string]gatewayResourceResult{
			"budget-a": {ID: "cp-a", Status: "synced"},
		}})
	})
	client.config.PushRetry = config.PushRetryConfig{MaxAttempts: 2, InitialDuration: time.Millisecond, ProgressionFactor: 1}
	setGatewayID(client, testGatewayID)
	seedBudget(t, client, "budget-a", "team-a", models.OriginGatewayAPI)

	client.pushGatewayResources()

	assert.Equal(t, int32(2), atomic.LoadInt32(&hits), "retried once, then succeeded")
	a, err := client.db.GetConfig("budget-a")
	require.NoError(t, err)
	assert.Equal(t, models.CPSyncStatusSuccess, a.CPSyncStatus)
}

func TestPushResource_IgnoresOtherKindsAndControlPlaneBudgets(t *testing.T) {
	assert.False(t, isPushedResource(&models.StoredConfig{Kind: models.KindRestApi, Origin: models.OriginGatewayAPI}))
	assert.False(t, isPushedResource(&models.StoredConfig{Kind: models.KindBudget, Origin: models.OriginControlPlane}))
	assert.True(t, isPushedResource(&models.StoredConfig{Kind: models.KindBudget, Origin: models.OriginGatewayAPI}))
	assert.False(t, isPushedResource(nil))
}

func TestPushGatewayArtifacts_LeavesSyncedResourceKindsOut(t *testing.T) {
	client := createTestClient(t)
	seedBudget(t, client, "budget-a", "team-a", models.OriginGatewayAPI)
	client.apiUtilsService.SetBaseURL("http://127.0.0.1:1") // any request would fail

	client.pushGatewayArtifacts()

	a, err := client.db.GetConfig("budget-a")
	require.NoError(t, err)
	assert.Equal(t, models.CPSyncStatusPending, a.CPSyncStatus, "budgets are not artifacts")
}

// A control plane that does not take gateway resources (AI Workspace) answers 404: the
// push stops at once, without retries.
func TestPushResources_NotFoundMeansTheControlPlaneDoesNotTakeThem(t *testing.T) {
	var hits int32
	client := newManifestTLSServer(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusNotFound)
	})
	client.config.PushRetry = config.PushRetryConfig{MaxAttempts: 5, InitialDuration: time.Millisecond, ProgressionFactor: 1}
	setGatewayID(client, testGatewayID)
	seedBudget(t, client, "budget-a", "team-a", models.OriginGatewayAPI)

	client.pushGatewayResources()

	assert.Equal(t, int32(1), atomic.LoadInt32(&hits), "not retried")
}

// Budgets off on this gateway (no budget applier): nothing is pushed on connect.
func TestCanPushResources_OnlyWithBudgetsEnabled(t *testing.T) {
	client := createTestClient(t)
	client.config.DeploymentSyncEnabled = true
	client.SetBudgetApplier(nil)
	assert.False(t, client.canPushResources())
}
