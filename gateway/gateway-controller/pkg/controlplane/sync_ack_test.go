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

package controlplane

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// Deployments the gateway applies through its startup sync are acknowledged
// like live events, so the control plane can settle a deployment it sent while
// the gateway was offline.

// serveSyncBatch makes the control plane answer POST /deployments/fetch-batch
// with a tar.gz holding yamlContent as {deploymentID}/artifact.yaml.
func (h *agentEventsHarness) serveSyncBatch(t *testing.T, deploymentID, yamlContent string) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: deploymentID + "/artifact.yaml",
		Mode: 0o600,
		Size: int64(len(yamlContent)),
	}))
	_, err := tw.Write([]byte(yamlContent))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	body := buf.Bytes()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/internal/v1/deployments/fetch-batch" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)

	h.client.apiUtilsService = utils.NewAPIUtilsService(utils.PlatformAPIConfig{
		BaseURL: server.URL + "/api/internal/v1",
		Token:   agentEvtToken,
	}, testHTTPClient(false), h.client.logger)
}

func syncAgentDeployment(deploymentID, state string, deployedAt time.Time) models.ControlPlaneDeployment {
	return models.ControlPlaneDeployment{
		ArtifactID:   agentEvtID,
		DeploymentID: deploymentID,
		Kind:         models.KindAgent,
		State:        state,
		DeployedAt:   deployedAt,
	}
}

// A deployment made while the gateway was offline is applied by the sync and
// acked with the control plane's performed_at.
func TestProcessSyncFetchBatch_AcksAppliedDeployment(t *testing.T) {
	h := newAgentEventsHarness(t)
	deployedAt := time.Now().UTC().Truncate(time.Millisecond)
	h.serveSyncBatch(t, "dep-1", agentEvtYAML("Weather Agent"))

	h.client.processSyncFetchBatch([]models.ControlPlaneDeployment{
		syncAgentDeployment("dep-1", string(models.StateDeployed), deployedAt),
	}, agentEvtGatewayID)

	ack := h.nextAck(t)
	assert.Equal(t, DeploymentAckPayload{
		DeploymentID: "dep-1",
		ArtifactID:   agentEvtID,
		ResourceType: models.KindAgent,
		Action:       "deploy",
		Status:       "success",
		PerformedAt:  ack.PerformedAt,
	}, ack)
	assert.True(t, deployedAt.Equal(ack.PerformedAt), "the ack echoes the concurrency token")

	stored, err := h.db.GetConfig(agentEvtID)
	require.NoError(t, err)
	assert.Equal(t, "dep-1", stored.DeploymentID)
}

// An artifact the gateway cannot apply is acked as failed with the same code
// the live handler would report.
func TestProcessSyncFetchBatch_AcksFailedApply(t *testing.T) {
	h := newAgentEventsHarness(t)
	deployedAt := time.Now().UTC().Truncate(time.Millisecond)
	h.serveSyncBatch(t, "dep-1", "not: [valid")

	h.client.processSyncFetchBatch([]models.ControlPlaneDeployment{
		syncAgentDeployment("dep-1", string(models.StateDeployed), deployedAt),
	}, agentEvtGatewayID)

	ack := h.nextAck(t)
	assert.Equal(t, "failed", ack.Status)
	assert.Equal(t, "deploy", ack.Action)
	assert.NotEmpty(t, ack.ErrorCode)
	assert.True(t, deployedAt.Equal(ack.PerformedAt))
}

// A sync write that loses to a newer stored version changes nothing and is
// not acked, matching the live handlers.
func TestProcessSyncFetchBatch_StaleIsNotAcked(t *testing.T) {
	h := newAgentEventsHarness(t)
	newer := time.Now().UTC().Truncate(time.Millisecond)
	h.deploy(t, agentEvtID, "dep-2", newer)
	h.serveSyncBatch(t, "dep-1", agentEvtYAML("Weather Agent Stale"))

	h.client.processSyncFetchBatch([]models.ControlPlaneDeployment{
		syncAgentDeployment("dep-1", string(models.StateDeployed), newer.Add(-time.Minute)),
	}, agentEvtGatewayID)

	h.assertNoAck(t)
	stored, err := h.db.GetConfig(agentEvtID)
	require.NoError(t, err)
	assert.Equal(t, "dep-2", stored.DeploymentID)
}

// The fetch path always deploys, so it does not ack a deployment the control
// plane expects undeployed.
func TestProcessSyncFetchBatch_UndeployedRemoteIsNotAcked(t *testing.T) {
	h := newAgentEventsHarness(t)
	deployedAt := time.Now().UTC().Truncate(time.Millisecond)
	h.serveSyncBatch(t, "dep-1", agentEvtYAML("Weather Agent"))

	h.client.processSyncFetchBatch([]models.ControlPlaneDeployment{
		syncAgentDeployment("dep-1", string(models.StateUndeployed), deployedAt),
	}, agentEvtGatewayID)

	h.assertNoAck(t)
}

// An undeploy made while the gateway was offline is applied by the sync and
// acked as an undeploy.
func TestProcessSyncStatusUpdates_AcksUndeploy(t *testing.T) {
	h := newAgentEventsHarness(t)
	deployedAt := time.Now().UTC().Truncate(time.Millisecond)
	h.deploy(t, agentEvtID, "dep-1", deployedAt)
	undeployedAt := deployedAt.Add(time.Second)

	h.client.processSyncStatusUpdates([]models.ControlPlaneDeployment{
		syncAgentDeployment("dep-1", string(models.StateUndeployed), undeployedAt),
	}, agentEvtGatewayID)

	ack := h.nextAck(t)
	assert.Equal(t, DeploymentAckPayload{
		DeploymentID: "dep-1",
		ArtifactID:   agentEvtID,
		ResourceType: models.KindAgent,
		Action:       "undeploy",
		Status:       "success",
		PerformedAt:  ack.PerformedAt,
	}, ack)
	assert.True(t, undeployedAt.Equal(ack.PerformedAt), "the ack echoes the concurrency token")

	stored, err := h.db.GetConfig(agentEvtID)
	require.NoError(t, err)
	assert.Equal(t, models.StateUndeployed, stored.DesiredState)
}

// A restore (undeployed → deployed) made while the gateway was offline is acked
// as a deploy.
func TestProcessSyncStatusUpdates_AcksRestore(t *testing.T) {
	h := newAgentEventsHarness(t)
	deployedAt := time.Now().UTC().Truncate(time.Millisecond)
	h.deploy(t, agentEvtID, "dep-1", deployedAt)
	h.client.processSyncStatusUpdates([]models.ControlPlaneDeployment{
		syncAgentDeployment("dep-1", string(models.StateUndeployed), deployedAt.Add(time.Second)),
	}, agentEvtGatewayID)
	require.Equal(t, "undeploy", h.nextAck(t).Action)
	restoredAt := deployedAt.Add(2 * time.Second)

	h.client.processSyncStatusUpdates([]models.ControlPlaneDeployment{
		syncAgentDeployment("dep-1", string(models.StateDeployed), restoredAt),
	}, agentEvtGatewayID)

	ack := h.nextAck(t)
	assert.Equal(t, "deploy", ack.Action)
	assert.Equal(t, "success", ack.Status)
	assert.True(t, restoredAt.Equal(ack.PerformedAt))
}

// A status update older than the stored version is skipped and not acked.
func TestProcessSyncStatusUpdates_StaleIsNotAcked(t *testing.T) {
	h := newAgentEventsHarness(t)
	deployedAt := time.Now().UTC().Truncate(time.Millisecond)
	h.deploy(t, agentEvtID, "dep-1", deployedAt)

	h.client.processSyncStatusUpdates([]models.ControlPlaneDeployment{
		syncAgentDeployment("dep-1", string(models.StateUndeployed), deployedAt.Add(-time.Minute)),
	}, agentEvtGatewayID)

	h.assertNoAck(t)
	stored, err := h.db.GetConfig(agentEvtID)
	require.NoError(t, err)
	assert.Equal(t, models.StateDeployed, stored.DesiredState)
}

func TestSyncAckResourceType(t *testing.T) {
	tests := []struct {
		kind     string
		expected string
	}{
		{models.KindRestApi, "api"},
		{models.KindGraphQLApi, "graphqlapi"},
		{models.KindLlmProvider, "llmprovider"},
		{models.KindLlmProxy, "llmproxy"},
		{models.KindMcp, "mcpproxy"},
		{models.KindAgent, models.KindAgent},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			assert.Equal(t, tt.expected, syncAckResourceType(tt.kind))
		})
	}
}
