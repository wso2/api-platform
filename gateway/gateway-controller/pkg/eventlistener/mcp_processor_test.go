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
	policyenginev1 "github.com/wso2/api-platform/sdk/gateway/policyengine/v1"
)

func testMCPStoredConfig(uuid, handle, displayName, version string, desiredState models.DesiredState, policies []api.Policy) *models.StoredConfig {
	now := time.Now()
	upstreamURL := "https://example.com"
	mcp := api.MCPProxyConfiguration{
		ApiVersion: api.MCPProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.Mcp,
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
		Kind:                string(api.Mcp),
		Handle:              handle,
		DisplayName:         displayName,
		Version:             version,
		SourceConfiguration: mcp,
		DesiredState:        desiredState,
		Origin:              models.OriginGatewayAPI,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	_ = utils.HydrateStoredMCPConfig(cfg)
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

	policyStore := storage.NewPolicyStore()
	policyManager := policyxds.NewPolicyManager(policyStore, policyxds.NewSnapshotManager(policyStore, newTestLogger()), newTestLogger())

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

	policy, err := policyManager.GetPolicy(cfg.UUID + "-policies")
	require.NoError(t, err)
	require.NotEmpty(t, policy.Configuration.Routes)
}

func TestHandleEvent_MCPDelete_RemovesLocalStateAndPolicy(t *testing.T) {
	store := storage.NewConfigStore()
	cfg := testMCPStoredConfig("mcp-delete-id", "test-mcp", "Test MCP", "v1.0.0", models.StateDeployed, nil)
	require.NoError(t, store.Add(cfg))

	policyStore := storage.NewPolicyStore()
	policyManager := policyxds.NewPolicyManager(policyStore, policyxds.NewSnapshotManager(policyStore, newTestLogger()), newTestLogger())
	require.NoError(t, policyStore.Set(&models.StoredPolicyConfig{
		ID: cfg.UUID + "-policies",
		Configuration: policyenginev1.Configuration{
			Metadata: policyenginev1.Metadata{
				APIName: "Test MCP",
				Version: "v1.0.0",
				Context: "/mcp",
			},
		},
	}))

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

	_, err = policyManager.GetPolicy(cfg.UUID + "-policies")
	require.Error(t, err)
}
