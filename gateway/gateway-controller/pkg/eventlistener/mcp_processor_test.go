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

package eventlistener

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

func testMCPStoredConfig(uuid, handle, displayName, version string, desiredState models.DesiredState, policies []api.Policy) *models.StoredConfig {
	now := time.Now()
	upstreamURL := "https://example.com"
	mcp := api.MCPProxyConfiguration{
		ApiVersion: api.MCPProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.MCPProxyConfigurationKindMcp,
		Metadata: api.Metadata{
			Name: handle,
		},
		Spec: api.MCPProxyConfigData{
			DisplayName: displayName,
			Version:     version,
			Context:     stringPtr("/mcp"),
			Upstream: api.MCPProxyConfigData_Upstream{
				Url: &upstreamURL,
			},
		},
	}
	if len(policies) > 0 {
		mcp.Spec.Policies = &policies
	}

	cfg := &models.StoredConfig{
		UUID:                uuid,
		Kind:                string(api.MCPProxyConfigurationKindMcp),
		Handle:              handle,
		DisplayName:         displayName,
		Version:             version,
		SourceConfiguration: mcp,
		DesiredState:        desiredState,
		Origin:              models.OriginGatewayAPI,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	_ = utils.HydrateStoredMCPConfig(cfg, nil)
	return cfg
}

func TestHandleEvent_MCPUpdate_RehydratesConfigAndPolicyFromDB(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)
	cfg := testMCPStoredConfig(
		"mcp-update-id",
		"test-mcp",
		"Test MCP",
		"v1.0.0",
		models.StateUndeployed,
		[]api.Policy{{Name: "rate-limit", Version: "v1"}},
	)
	require.NoError(t, db.SaveConfig(cfg))

	policyManager := policyxds.NewPolicyManager(policyxds.NewSnapshotManager(newTestLogger()), newTestLogger())

	listener := &EventListener{
		store:         store,
		db:            db,
		policyManager: policyManager,
		routerConfig: &config.RouterConfig{
			VHosts: config.VHostsConfig{
				Main: config.VHostEntry{Default: "api.example.com"},
			},
		},
		systemConfig: &config.Config{},
		policyDefinitions: map[string]models.PolicyDefinition{
			"rate-limit-v1.0.0": {Name: "rate-limit", Version: "v1.0.0"},
		},
		logger: newTestLogger(),
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeMCPProxy,
		Action:    "UPDATE",
		EntityID:  cfg.UUID,
		EventID:   "corr-mcp-update",
	})

	stored, err := store.Get(cfg.UUID)
	require.NoError(t, err)
	assert.Equal(t, models.StateUndeployed, stored.DesiredState)
	_, ok := stored.Configuration.(api.RestAPI)
	assert.True(t, ok)
}

func TestHandleEvent_MCPDelete_RemovesLocalStateAndPolicy(t *testing.T) {
	store := storage.NewConfigStore()
	cfg := testMCPStoredConfig("mcp-delete-id", "test-mcp", "Test MCP", "v1.0.0", models.StateDeployed, nil)
	require.NoError(t, store.Add(cfg))

	runtimeStore := storage.NewRuntimeConfigStore()
	policyManager := policyxds.NewPolicyManager(policyxds.NewSnapshotManager(newTestLogger()), newTestLogger())
	policyManager.SetRuntimeStore(runtimeStore)
	runtimeKey := storage.Key(cfg.Kind, cfg.Handle)
	runtimeStore.Set(runtimeKey, &models.RuntimeDeployConfig{})

	listener := &EventListener{
		store:         store,
		policyManager: policyManager,
		logger:        newTestLogger(),
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeMCPProxy,
		Action:    "DELETE",
		EntityID:  cfg.UUID,
		EventID:   "corr-mcp-delete",
	})

	_, err := store.Get(cfg.UUID)
	require.ErrorIs(t, err, storage.ErrNotFound)

	_, exists := runtimeStore.Get(runtimeKey)
	assert.False(t, exists)
}

// Deploying an MCP proxy refreshes the subscription snapshot, so a subscription that arrived before
// the proxy was known here takes effect without waiting for another refresh.
func TestHandleEvent_MCPUpdate_RefreshesSubscriptions(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)
	cfg := testMCPStoredConfig("mcp-refresh-id", "refresh-mcp", "Refresh MCP", "v1.0.0", models.StateDeployed, nil)
	require.NoError(t, db.SaveConfig(cfg))
	snapshots := &mockSubscriptionSnapshotUpdater{}
	listener := &EventListener{
		store: store, db: db, subscriptionManager: snapshots,
		policyManager: policyxds.NewPolicyManager(policyxds.NewSnapshotManager(newTestLogger()), newTestLogger()),
		routerConfig:  &config.RouterConfig{VHosts: config.VHostsConfig{Main: config.VHostEntry{Default: "api.example.com"}}},
		systemConfig:  &config.Config{},
		logger:        newTestLogger(),
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeMCPProxy, Action: "UPDATE", EntityID: cfg.UUID, EventID: "corr-mcp-refresh",
	})

	assert.Equal(t, 1, snapshots.callCount)
}

// Deleting an MCP proxy removes only its subscriptions, including one keyed by its control-plane ID,
// and refreshes the subscription snapshot.
func TestHandleEvent_MCPDelete_RemovesSubscriptions(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)
	cfg := testMCPStoredConfig("mcp-subs-id", "subs-mcp", "Subs MCP", "v1.0.0", models.StateDeployed, nil)
	cfg.CPArtifactID = "mcp-cp-id"
	require.NoError(t, store.Add(cfg))
	for _, sub := range []*models.Subscription{
		{ID: "sub-mcp", APIID: cfg.UUID, SubscriptionToken: "mcp-token", Status: models.SubscriptionStatusActive},
		{ID: "sub-mcp-cp", APIID: "mcp-cp-id", SubscriptionToken: "mcp-cp-token", Status: models.SubscriptionStatusActive},
		{ID: "sub-other", APIID: "other-api", SubscriptionToken: "other-token", Status: models.SubscriptionStatusActive},
	} {
		require.NoError(t, db.SaveSubscription(sub))
	}
	snapshots := &mockSubscriptionSnapshotUpdater{}
	listener := &EventListener{store: store, db: db, subscriptionManager: snapshots, logger: newTestLogger()}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeMCPProxy, Action: "DELETE", EntityID: cfg.UUID, EventID: "corr-mcp-subs",
	})

	left, err := db.ListSubscriptionsByAPI(cfg.UUID, "", nil, nil)
	require.NoError(t, err)
	assert.Empty(t, left)
	leftByCP, err := db.ListSubscriptionsByAPI("mcp-cp-id", "", nil, nil)
	require.NoError(t, err)
	assert.Empty(t, leftByCP)
	other, err := db.ListSubscriptionsByAPI("other-api", "", nil, nil)
	require.NoError(t, err)
	assert.Len(t, other, 1)
	assert.Equal(t, 1, snapshots.callCount)
}

type failingSnapshotUpdater struct{ calls int }

func (f *failingSnapshotUpdater) UpdateSnapshot(context.Context) error {
	f.calls++
	return errors.New("snapshot unavailable")
}

// A failed delete skips the snapshot refresh; a failed refresh is only logged.
func TestDeleteSubscriptionsOf_Failures(t *testing.T) {
	(&EventListener{logger: newTestLogger()}).deleteSubscriptionsOf(nil, "api-1", "evt")

	closed := setupSQLiteDBForEventListenerTests(t)
	snapshots := &mockSubscriptionSnapshotUpdater{}
	require.NoError(t, closed.Close())
	(&EventListener{db: closed, subscriptionManager: snapshots, logger: newTestLogger()}).deleteSubscriptionsOf(nil, "api-1", "evt")
	assert.Zero(t, snapshots.callCount)

	failing := &failingSnapshotUpdater{}
	(&EventListener{db: setupSQLiteDBForEventListenerTests(t), subscriptionManager: failing, logger: newTestLogger()}).deleteSubscriptionsOf(nil, "api-1", "evt")
	assert.Equal(t, 1, failing.calls)
}
