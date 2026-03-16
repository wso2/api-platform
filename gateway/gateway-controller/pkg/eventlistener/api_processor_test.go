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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	policyenginev1 "github.com/wso2/api-platform/sdk/gateway/policyengine/v1"
)

func TestHandleEvent_APICreate_AddsConfigFromDB(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)
	cfg := testRestStoredConfig("api-create-id", "test-api", "Test API", "v1.0.0", models.StatusPending)
	require.NoError(t, db.SaveConfig(cfg))

	listener := &EventListener{
		store:  store,
		db:     db,
		logger: newTestLogger(),
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeAPI,
		Action:    "CREATE",
		EntityID:  cfg.UUID,
		EventID:   "corr-api-create",
	})

	stored, err := store.Get(cfg.UUID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusPending, stored.Status)
	assert.Equal(t, cfg.DisplayName, stored.DisplayName)
}

func TestHandleEvent_APIUpdate_RefreshesExistingConfigFromDB(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)

	stale := testRestStoredConfig("api-update-id", "test-api", "Test API", "v1.0.0", models.StatusDeployed)
	require.NoError(t, store.Add(stale))

	latest := testRestStoredConfig("api-update-id", "test-api", "Test API", "v1.0.0", models.StatusUndeployed)
	require.NoError(t, db.SaveConfig(latest))

	listener := &EventListener{
		store:  store,
		db:     db,
		logger: newTestLogger(),
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeAPI,
		Action:    "UPDATE",
		EntityID:  latest.UUID,
		EventID:   "corr-api-update",
	})

	stored, err := store.Get(latest.UUID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusUndeployed, stored.Status)
}

func TestHandleEvent_APIDelete_RemovesAPIKeysFromMemoryAndXDS(t *testing.T) {
	store := storage.NewConfigStore()
	xdsManager := &mockAPIKeyXDSManager{}
	cfg := testRestStoredConfig("api-delete-id", "delete-api", "Delete API", "v1.0.0", models.StatusPending)
	require.NoError(t, store.Add(cfg))

	apiKey := testAPIKey("api-key-id-1", "test-key", "Test Key", cfg.UUID)
	require.NoError(t, store.StoreAPIKey(apiKey))

	listener := &EventListener{
		store:            store,
		apiKeyXDSManager: xdsManager,
		logger:           newTestLogger(),
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeAPI,
		Action:    "DELETE",
		EntityID:  cfg.UUID,
		EventID:   "corr-api-delete",
	})

	_, err := store.Get(cfg.UUID)
	require.ErrorIs(t, err, storage.ErrNotFound)

	_, err = store.GetAPIKeyByName(cfg.UUID, apiKey.Name)
	require.ErrorIs(t, err, storage.ErrNotFound)

	if assert.Len(t, xdsManager.removeCalls, 1) {
		assert.Equal(t, cfg.UUID, xdsManager.removeCalls[0].apiID)
		assert.Equal(t, cfg.DisplayName, xdsManager.removeCalls[0].apiName)
		assert.Equal(t, cfg.Version, xdsManager.removeCalls[0].apiVersion)
		assert.Equal(t, "corr-api-delete", xdsManager.removeCalls[0].correlationID)
	}
}

func TestUpdatePoliciesForAPI_RemovesExistingPolicyWhenNoPoliciesAreDerived(t *testing.T) {
	policyStore := storage.NewPolicyStore()
	policyManager := policyxds.NewPolicyManager(policyStore, policyxds.NewSnapshotManager(policyStore, newTestLogger()), newTestLogger())
	existingPolicyID := "test-api-id-policies"

	require.NoError(t, policyStore.Set(&models.StoredPolicyConfig{
		ID: existingPolicyID,
		Configuration: policyenginev1.Configuration{
			Metadata: policyenginev1.Metadata{
				APIName: "Test API",
				Version: "v1.0.0",
				Context: "/test",
			},
		},
	}))

	listener := &EventListener{
		logger:        newTestLogger(),
		policyManager: policyManager,
		routerConfig: &config.RouterConfig{
			VHosts: config.VHostsConfig{
				Main: config.VHostEntry{Default: "api.example.com"},
			},
		},
		systemConfig: &config.Config{},
	}

	listener.updatePoliciesForAPI(
		testRestStoredConfig("test-api-id", "test-api", "Test API", "v1.0.0", models.StatusPending),
		"corr-policy-remove",
	)

	_, err := policyManager.GetPolicy(existingPolicyID)
	require.Error(t, err)
	assert.Equal(t, 0, policyStore.Count())
}

func TestUpdatePoliciesForAPI_AddsDerivedPolicyWhenPoliciesExist(t *testing.T) {
	policyStore := storage.NewPolicyStore()
	policyManager := policyxds.NewPolicyManager(policyStore, policyxds.NewSnapshotManager(policyStore, newTestLogger()), newTestLogger())
	cfg := testRestStoredConfigWithPolicies(
		"policy-api-id",
		"policy-api",
		"Policy API",
		"v1.0.0",
		models.StatusPending,
		[]api.Policy{
			{Name: "rate-limit", Version: "v1"},
		},
	)

	listener := &EventListener{
		logger:        newTestLogger(),
		policyManager: policyManager,
		routerConfig: &config.RouterConfig{
			VHosts: config.VHostsConfig{
				Main: config.VHostEntry{Default: "api.example.com"},
			},
		},
		systemConfig: &config.Config{},
		policyDefinitions: map[string]api.PolicyDefinition{
			"rate-limit-v1.0.0": {Name: "rate-limit", Version: "v1.0.0"},
		},
	}

	listener.updatePoliciesForAPI(cfg, "corr-policy-add")

	policy, err := policyManager.GetPolicy(cfg.UUID + "-policies")
	require.NoError(t, err)
	assert.Equal(t, "Policy API", policy.APIName())
	assert.Equal(t, "v1.0.0", policy.APIVersion())
	require.Len(t, policy.Configuration.Routes, 1)
	require.Len(t, policy.Configuration.Routes[0].Policies, 1)
	assert.Equal(t, "rate-limit", policy.Configuration.Routes[0].Policies[0].Name)
	assert.Equal(t, "v1", policy.Configuration.Routes[0].Policies[0].Version)
}
