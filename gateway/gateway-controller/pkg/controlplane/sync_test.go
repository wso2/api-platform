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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// --- computeSyncDiff Tests ---

func TestComputeSyncDiff_NewDeployments(t *testing.T) {
	now := time.Now()
	remote := []models.ControlPlaneDeployment{
		{ArtifactID: "api-1", DeploymentID: "dep-1", Kind: models.KindRestApi, State: "deployed", DeployedAt: now},
		{ArtifactID: "provider-1", DeploymentID: "dep-2", Kind: models.KindLlmProvider, State: "deployed", DeployedAt: now},
	}

	// No local configs
	var local []*models.StoredConfig

	diff := computeSyncDiff(remote, local)

	assert.Len(t, diff.toFetch, 2, "both deployments should be fetched")
	assert.Empty(t, diff.toUpdateStatus)
	assert.Empty(t, diff.toDelete)
}

func TestComputeSyncDiff_UpToDate(t *testing.T) {
	now := time.Now()
	remote := []models.ControlPlaneDeployment{
		{ArtifactID: "api-1", DeploymentID: "dep-1", Kind: models.KindRestApi, State: "deployed", DeployedAt: now},
	}

	local := []*models.StoredConfig{
		{UUID: "api-1", DeploymentID: "dep-1", Kind: models.KindRestApi, DesiredState: models.StateDeployed, Origin: models.OriginControlPlane, DeployedAt: &now},
	}

	diff := computeSyncDiff(remote, local)

	assert.Empty(t, diff.toFetch, "same deployment ID should not be fetched")
	assert.Empty(t, diff.toUpdateStatus)
	assert.Empty(t, diff.toDelete)
}

func TestComputeSyncDiff_DeploymentIDMismatch(t *testing.T) {
	now := time.Now()

	remote := []models.ControlPlaneDeployment{
		{ArtifactID: "api-1", DeploymentID: "dep-2", Kind: models.KindRestApi, State: "deployed", DeployedAt: now},
	}

	local := []*models.StoredConfig{
		{UUID: "api-1", DeploymentID: "dep-1", Kind: models.KindRestApi, DesiredState: models.StateDeployed, Origin: models.OriginControlPlane, DeployedAt: &now},
	}

	diff := computeSyncDiff(remote, local)

	assert.Len(t, diff.toFetch, 1, "different deployment ID should trigger re-fetch")
	assert.Equal(t, "api-1", diff.toFetch[0].ArtifactID)
	assert.Empty(t, diff.toUpdateStatus)
	assert.Empty(t, diff.toDelete)
}

func TestComputeSyncDiff_StatusUpdate_Undeploy(t *testing.T) {
	now := time.Now()
	remote := []models.ControlPlaneDeployment{
		{ArtifactID: "api-1", DeploymentID: "dep-1", Kind: models.KindRestApi, State: "undeployed", DeployedAt: now},
	}

	local := []*models.StoredConfig{
		{UUID: "api-1", DeploymentID: "dep-1", Kind: models.KindRestApi, DesiredState: models.StateDeployed, Origin: models.OriginControlPlane, DeployedAt: &now},
	}

	diff := computeSyncDiff(remote, local)

	assert.Empty(t, diff.toFetch)
	assert.Len(t, diff.toUpdateStatus, 1, "should detect status change to undeployed")
	assert.Equal(t, "api-1", diff.toUpdateStatus[0].ArtifactID)
	assert.Empty(t, diff.toDelete)
}

func TestComputeSyncDiff_StatusUpdate_Redeploy(t *testing.T) {
	now := time.Now()
	remote := []models.ControlPlaneDeployment{
		{ArtifactID: "api-1", DeploymentID: "dep-1", Kind: models.KindRestApi, State: "deployed", DeployedAt: now},
	}

	local := []*models.StoredConfig{
		{UUID: "api-1", DeploymentID: "dep-1", Kind: models.KindRestApi, DesiredState: models.StateUndeployed, Origin: models.OriginControlPlane, DeployedAt: &now},
	}

	diff := computeSyncDiff(remote, local)

	assert.Empty(t, diff.toFetch)
	assert.Len(t, diff.toUpdateStatus, 1, "should detect status change back to deployed")
	assert.Equal(t, "api-1", diff.toUpdateStatus[0].ArtifactID)
	assert.Empty(t, diff.toDelete)
}

func TestComputeSyncDiff_DeployedAtMismatch(t *testing.T) {
	localTime := time.Now().Add(-1 * time.Hour)
	remoteTime := time.Now()

	remote := []models.ControlPlaneDeployment{
		{ArtifactID: "api-1", DeploymentID: "dep-1", Kind: models.KindRestApi, State: "deployed", DeployedAt: remoteTime},
	}

	local := []*models.StoredConfig{
		{UUID: "api-1", DeploymentID: "dep-1", Kind: models.KindRestApi, DesiredState: models.StateDeployed, Origin: models.OriginControlPlane, DeployedAt: &localTime},
	}

	diff := computeSyncDiff(remote, local)

	assert.Len(t, diff.toFetch, 1, "same deployment ID and state but different deployed_at should re-fetch")
	assert.Empty(t, diff.toUpdateStatus)
	assert.Empty(t, diff.toDelete)
}

func TestComputeSyncDiff_OrphanDeletion(t *testing.T) {
	now := time.Now()
	// Remote has no deployments
	var remote []models.ControlPlaneDeployment

	local := []*models.StoredConfig{
		{UUID: "api-1", Kind: models.KindRestApi, DesiredState: models.StateDeployed, Origin: models.OriginControlPlane, DeployedAt: &now},
		{UUID: "provider-1", Kind: models.KindLlmProvider, DesiredState: models.StateDeployed, Origin: models.OriginControlPlane, DeployedAt: &now},
	}

	diff := computeSyncDiff(remote, local)

	assert.Empty(t, diff.toFetch)
	assert.Empty(t, diff.toUpdateStatus)
	assert.Len(t, diff.toDelete, 2, "both local-only configs should be marked for deletion")
}

func TestComputeSyncDiff_OnlyControlPlaneConfigs(t *testing.T) {
	now := time.Now()
	// Remote has nothing
	var remote []models.ControlPlaneDeployment

	// Caller (syncDeployments) uses GetAllConfigsByOrigin(OriginControlPlane),
	// so only CP configs are passed in. Gateway-API configs are never seen.
	local := []*models.StoredConfig{
		{UUID: "cp-api", Kind: models.KindRestApi, DesiredState: models.StateDeployed, Origin: models.OriginControlPlane, DeployedAt: &now},
	}

	diff := computeSyncDiff(remote, local)

	// CP config not in remote → should be marked for deletion
	assert.Len(t, diff.toDelete, 1, "control-plane config missing from remote should be deleted")
	assert.Equal(t, "cp-api", diff.toDelete[0])
}

func TestComputeSyncDiff_NilDeployedAt(t *testing.T) {
	now := time.Now()
	remote := []models.ControlPlaneDeployment{
		{ArtifactID: "api-1", DeploymentID: "dep-1", Kind: models.KindRestApi, State: "deployed", DeployedAt: now},
	}

	local := []*models.StoredConfig{
		{UUID: "api-1", Kind: models.KindRestApi, DesiredState: models.StateDeployed, Origin: models.OriginControlPlane, DeployedAt: nil},
	}

	diff := computeSyncDiff(remote, local)

	assert.Len(t, diff.toFetch, 1, "config with nil DeployedAt should be re-fetched")
}

func TestComputeSyncDiff_MixedScenario(t *testing.T) {
	midTime := time.Now().Add(-1 * time.Hour)
	newTime := time.Now()

	remote := []models.ControlPlaneDeployment{
		// New deployment (not in local)
		{ArtifactID: "new-api", DeploymentID: "dep-new", Kind: models.KindRestApi, State: "deployed", DeployedAt: newTime},
		// Updated deployment (different deployment ID)
		{ArtifactID: "updated-api", DeploymentID: "dep-updated-v2", Kind: models.KindRestApi, State: "deployed", DeployedAt: newTime},
		// Undeployed (status change, same deployment ID)
		{ArtifactID: "undeployed-api", DeploymentID: "dep-undeploy", Kind: models.KindRestApi, State: "undeployed", DeployedAt: midTime},
		// Up-to-date (same deployment ID, same state)
		{ArtifactID: "current-api", DeploymentID: "dep-current", Kind: models.KindRestApi, State: "deployed", DeployedAt: midTime},
	}

	local := []*models.StoredConfig{
		{UUID: "updated-api", DeploymentID: "dep-updated-v1", Kind: models.KindRestApi, DesiredState: models.StateDeployed, Origin: models.OriginControlPlane, DeployedAt: &midTime},
		{UUID: "undeployed-api", DeploymentID: "dep-undeploy", Kind: models.KindRestApi, DesiredState: models.StateDeployed, Origin: models.OriginControlPlane, DeployedAt: &midTime},
		{UUID: "current-api", DeploymentID: "dep-current", Kind: models.KindRestApi, DesiredState: models.StateDeployed, Origin: models.OriginControlPlane, DeployedAt: &midTime},
		{UUID: "orphan-api", DeploymentID: "dep-orphan", Kind: models.KindRestApi, DesiredState: models.StateDeployed, Origin: models.OriginControlPlane, DeployedAt: &midTime},
	}

	diff := computeSyncDiff(remote, local)

	assert.Len(t, diff.toFetch, 2, "new-api and updated-api should be fetched")
	assert.Len(t, diff.toUpdateStatus, 1, "undeployed-api should be status-updated")
	assert.Len(t, diff.toDelete, 1, "orphan-api should be deleted")

	// Verify specific entries
	fetchIDs := make(map[string]bool)
	for _, d := range diff.toFetch {
		fetchIDs[d.ArtifactID] = true
	}
	assert.True(t, fetchIDs["new-api"])
	assert.True(t, fetchIDs["updated-api"])

	assert.Equal(t, "undeployed-api", diff.toUpdateStatus[0].ArtifactID)
	assert.Equal(t, "orphan-api", diff.toDelete[0])
}

// --- syncCorrelationID Tests ---

func TestSyncCorrelationID_UsesEtag(t *testing.T) {
	dep := models.ControlPlaneDeployment{
		ArtifactID: "api-1",
		DeployedAt: time.Now(),
		Etag:       "etag-12345",
	}

	id := syncCorrelationID(dep)
	assert.Equal(t, "etag-12345", id)
}

func TestSyncCorrelationID_GeneratesDeterministic(t *testing.T) {
	deployedAt := time.Date(2026, 3, 24, 10, 0, 0, 0, time.UTC)
	dep := models.ControlPlaneDeployment{
		ArtifactID: "api-1",
		DeployedAt: deployedAt,
		Etag:       "",
	}

	id1 := syncCorrelationID(dep)
	id2 := syncCorrelationID(dep)

	assert.Equal(t, id1, id2, "same inputs should produce same correlation ID")
	assert.NotEmpty(t, id1)

	// Verify it matches the deterministic UUID generation
	expected := utils.GenerateDeterministicUUIDv7("api-1", deployedAt)
	assert.Equal(t, expected, id1)
}

// --- syncEventType Tests ---

func TestSyncEventType(t *testing.T) {
	tests := []struct {
		kind     string
		expected string
	}{
		{models.KindLlmProvider, string(eventhub.EventTypeLLMProvider)},
		{models.KindLlmProxy, string(eventhub.EventTypeLLMProxy)},
		{models.KindRestApi, string(eventhub.EventTypeAPI)},
		{"UnknownKind", string(eventhub.EventTypeAPI)},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			result := syncEventType(tt.kind)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

// --- processSyncDeletions Tests ---

// A SoapApi deleted on the control plane while the gateway was disconnected is an
// orphan on reconnect. It is removed through the same generic deletion path as a
// RestApi, and its removal is announced.
func TestProcessSyncDeletions_RemovesOrphanedSoapAPI(t *testing.T) {
	client, store, db, hub := createDeletionTestClient()

	const soapID = "0199a1b2-0000-7000-8000-00000000c001"
	spec := api.SoapAPIData{DisplayName: "Calculator", Context: "/calc", Version: "v1.0"}
	spec.Upstream.Main = api.Upstream{Url: api.Ptr("http://calculator:3002/calculator/v1")}
	soapAPI := &models.StoredConfig{
		UUID:         soapID,
		Kind:         models.KindSoapApi,
		Handle:       "calculator-v1.0",
		DisplayName:  "Calculator",
		Version:      "v1.0",
		DesiredState: models.StateDeployed,
		Origin:       models.OriginControlPlane,
		Configuration: api.SoapAPI{
			Kind:     api.SoapAPIKindSoapApi,
			Metadata: api.Metadata{Name: "calculator-v1.0"},
			Spec:     spec,
		},
	}
	require.NoError(t, db.SaveConfig(soapAPI))
	require.NoError(t, store.Add(soapAPI))

	client.processSyncDeletions([]string{soapID}, "test-gateway")

	_, err := db.GetConfig(soapID)
	assert.True(t, storage.IsNotFoundError(err), "the orphaned SoapApi must be removed, got %v", err)
	assert.Equal(t, 1, db.removeKeyCallCount, "its API keys must be removed with it")
	require.Len(t, hub.publishedEvents, 1)
	assert.Equal(t, eventhub.EventTypeAPI, hub.publishedEvents[0].event.EventType)
	assert.Equal(t, "DELETE", hub.publishedEvents[0].event.Action)
	assert.Equal(t, soapID, hub.publishedEvents[0].event.EntityID)
}
