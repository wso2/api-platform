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
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	policyenginev1 "github.com/wso2/api-platform/sdk/gateway/policyengine/v1"
)

func TestHandleEvent_APIUpdate_SyncsUndeployedStatusFromDB(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)

	apiID := "undeploy-sync-api-id"
	now := time.Now()

	staleInMemory := &models.StoredConfig{
		ID:   apiID,
		Kind: string(api.RestApi),
		Configuration: api.APIConfiguration{
			ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.RestApi,
			Metadata: api.Metadata{
				Name: "undeploy-sync-handle",
			},
		},
		SourceConfiguration: api.APIConfiguration{
			ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.RestApi,
			Metadata: api.Metadata{
				Name: "undeploy-sync-handle",
			},
		},
		Status:    models.StatusDeployed,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, store.Add(staleInMemory))

	dbConfig := &models.StoredConfig{
		ID:   apiID,
		Kind: string(api.RestApi),
		Configuration: api.APIConfiguration{
			ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.RestApi,
			Metadata: api.Metadata{
				Name: "undeploy-sync-handle",
			},
		},
		SourceConfiguration: api.APIConfiguration{
			ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.RestApi,
			Metadata: api.Metadata{
				Name: "undeploy-sync-handle",
			},
		},
		Status:    models.StatusUndeployed,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, db.SaveConfig(dbConfig))

	listener := &EventListener{
		store:  store,
		db:     db,
		logger: logger,
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeAPI,
		Action:    "UPDATE",
		EntityID:  apiID,
		EventID:   "corr-api-update-undeploy",
	})

	updated, err := store.Get(apiID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusUndeployed, updated.Status)
}

func TestHandleEvent_APIDelete_RemovesAPIKeysFromMemoryAndXDS(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewConfigStore()
	xdsManager := &mockAPIKeyXDSManager{}

	apiID := "test-api-id"

	var spec api.APIConfiguration_Spec
	err := spec.FromAPIConfigData(api.APIConfigData{
		DisplayName: "Test API",
		Version:     "v1.0.0",
		Context:     "/test",
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{Url: ptr("https://example.com")},
		},
		Operations: []api.Operation{
			{
				Method: "GET",
				Path:   "/",
			},
		},
	})
	require.NoError(t, err)

	cfg := &models.StoredConfig{
		ID:   apiID,
		Kind: string(api.RestApi),
		Configuration: api.APIConfiguration{
			ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.RestApi,
			Metadata: api.Metadata{
				Name: "test-api-handle",
			},
			Spec: spec,
		},
		SourceConfiguration: api.APIConfiguration{
			ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.RestApi,
			Metadata: api.Metadata{
				Name: "test-api-handle",
			},
			Spec: spec,
		},
		Status:    models.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, store.Add(cfg))

	apiKey := &models.APIKey{
		ID:           "api-key-id-1",
		Name:         "test-key",
		DisplayName:  "Test Key",
		APIKey:       "hashed-key-value",
		MaskedAPIKey: "***test-key",
		APIId:        apiID,
		Operations:   "[\"*\"]",
		Status:       models.APIKeyStatusActive,
		CreatedAt:    time.Now(),
		CreatedBy:    "test-user",
		UpdatedAt:    time.Now(),
		Source:       "external",
	}
	require.NoError(t, store.StoreAPIKey(apiKey))

	listener := &EventListener{
		store:            store,
		apiKeyXDSManager: xdsManager,
		logger:           logger,
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeAPI,
		Action:    "DELETE",
		EntityID:  apiID,
		EventID:   "corr-api-delete",
	})

	_, err = store.Get(apiID)
	require.Error(t, err)

	_, err = store.GetAPIKeyByName(apiID, "test-key")
	require.ErrorIs(t, err, storage.ErrNotFound)

	if assert.Len(t, xdsManager.removeCalls, 1) {
		assert.Equal(t, apiID, xdsManager.removeCalls[0].apiID)
		assert.Equal(t, "Test API", xdsManager.removeCalls[0].apiName)
		assert.Equal(t, "v1.0.0", xdsManager.removeCalls[0].apiVersion)
		assert.Equal(t, "corr-api-delete", xdsManager.removeCalls[0].correlationID)
	}
}

func TestUpdatePoliciesForAPI_RemovesExistingPolicyWhenNoPoliciesAreDerived(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	policyStore := storage.NewPolicyStore()
	policyManager := policyxds.NewPolicyManager(policyStore, policyxds.NewSnapshotManager(policyStore, logger), logger)

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

	var spec api.APIConfiguration_Spec
	err := spec.FromAPIConfigData(api.APIConfigData{
		DisplayName: "Test API",
		Version:     "v1.0.0",
		Context:     "/test",
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{Url: ptr("https://example.com")},
		},
		Operations: []api.Operation{
			{
				Method: "GET",
				Path:   "/",
			},
		},
	})
	require.NoError(t, err)

	listener := &EventListener{
		logger:        logger,
		policyManager: policyManager,
		routerConfig:  &config.RouterConfig{},
		systemConfig:  &config.Config{},
	}

	listener.updatePoliciesForAPI(&models.StoredConfig{
		ID:   "test-api-id",
		Kind: string(api.RestApi),
		Configuration: api.APIConfiguration{
			ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.RestApi,
			Metadata: api.Metadata{
				Name: "test-api",
			},
			Spec: spec,
		},
	}, "corr-policy-remove")

	_, err = policyManager.GetPolicy(existingPolicyID)
	require.Error(t, err)
	assert.Equal(t, 0, policyStore.Count())
}
