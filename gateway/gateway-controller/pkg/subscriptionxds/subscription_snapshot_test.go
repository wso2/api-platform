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

package subscriptionxds

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

// MockStorage implements the storage.Storage interface for testing.
type MockStorage struct {
	configs              []*models.StoredConfig
	configsErr           error
	subscriptionPlans    []*models.SubscriptionPlan
	subscriptionPlansErr error
	subscriptions        []*models.Subscription
	subscriptionsErr     error
}

// GetPendingBottomUpAPIs implements [storage.Storage].
func (m *MockStorage) GetPendingBottomUpAPIs() ([]*models.StoredConfig, error) {
	return nil, nil
}
func (m *MockStorage) GetPendingCPSyncArtifacts() ([]*models.StoredConfig, error) {
	return nil, nil
}

func (m *MockStorage) GetGatewayOriginArtifactsForSync() ([]*models.StoredConfig, error) {
	return nil, nil
}

// UpdateCPSyncStatus implements [storage.Storage].
func (m *MockStorage) UpdateCPSyncStatus(uuid, cpArtifactID string, status models.CPSyncStatus, reason string) error {
	return nil
}

// GetConfigByCPArtifactID implements [storage.Storage].
func (m *MockStorage) GetConfigByCPArtifactID(cpArtifactID string) (*models.StoredConfig, error) {
	return nil, nil
}

func (m *MockStorage) GetAllConfigs() ([]*models.StoredConfig, error) {
	return m.configs, m.configsErr
}

func (m *MockStorage) ListSubscriptionPlans(gatewayID string) ([]*models.SubscriptionPlan, error) {
	return m.subscriptionPlans, m.subscriptionPlansErr
}

func (m *MockStorage) ListActiveSubscriptions() ([]*models.Subscription, error) {
	return m.subscriptions, m.subscriptionsErr
}

// Remaining interface methods (no-op implementations)
func (m *MockStorage) SaveConfig(cfg *models.StoredConfig) error           { return nil }
func (m *MockStorage) UpdateConfig(cfg *models.StoredConfig) error         { return nil }
func (m *MockStorage) UpsertConfig(cfg *models.StoredConfig) (bool, error) { return true, nil }
func (m *MockStorage) DeleteConfig(id string) error                        { return nil }
func (m *MockStorage) GetConfig(id string) (*models.StoredConfig, error)   { return nil, nil }
func (m *MockStorage) GetConfigByKindAndHandle(kind, handle string) (*models.StoredConfig, error) {
	return nil, nil
}
func (m *MockStorage) GetConfigByKindNameAndVersion(kind, displayName, version string) (*models.StoredConfig, error) {
	return nil, nil
}
func (m *MockStorage) GetAllConfigsByKind(kind string) ([]*models.StoredConfig, error) {
	return nil, nil
}
func (m *MockStorage) GetAllConfigsByOrigin(origin models.Origin) ([]*models.StoredConfig, error) {
	return nil, nil
}
func (m *MockStorage) SaveLLMProviderTemplate(template *models.StoredLLMProviderTemplate) error {
	return nil
}
func (m *MockStorage) UpdateLLMProviderTemplate(template *models.StoredLLMProviderTemplate) error {
	return nil
}
func (m *MockStorage) DeleteLLMProviderTemplate(id string) error { return nil }
func (m *MockStorage) GetLLMProviderTemplate(id string) (*models.StoredLLMProviderTemplate, error) {
	return nil, nil
}
func (m *MockStorage) GetAllLLMProviderTemplates() ([]*models.StoredLLMProviderTemplate, error) {
	return nil, nil
}
func (m *MockStorage) GetLLMProviderTemplateByHandle(handle string) (*models.StoredLLMProviderTemplate, error) {
	return nil, nil
}
func (m *MockStorage) SaveAPIKey(apiKey *models.APIKey) error                 { return nil }
func (m *MockStorage) UpsertAPIKey(apiKey *models.APIKey) error               { return nil }
func (m *MockStorage) GetAPIKeyByID(id string) (*models.APIKey, error)        { return nil, nil }
func (m *MockStorage) GetAPIKeyByUUID(uuid string) (*models.APIKey, error)    { return nil, nil }
func (m *MockStorage) GetAPIKeyByKey(key string) (*models.APIKey, error)      { return nil, nil }
func (m *MockStorage) GetAPIKeysByAPI(apiId string) ([]*models.APIKey, error) { return nil, nil }
func (m *MockStorage) GetAllAPIKeys() ([]*models.APIKey, error)               { return nil, nil }
func (m *MockStorage) GetAPIKeysByApplicationUUID(applicationUUID string) ([]*models.APIKey, error) {
	return nil, nil
}
func (m *MockStorage) GetAPIKeysByAPIAndName(apiId, name string) (*models.APIKey, error) {
	return nil, nil
}
func (m *MockStorage) UpdateAPIKey(apiKey *models.APIKey) error        { return nil }
func (m *MockStorage) DeleteAPIKey(key string) error                   { return nil }
func (m *MockStorage) RemoveAPIKeysAPI(apiId string) error             { return nil }
func (m *MockStorage) RemoveAPIKeyAPIAndName(apiId, name string) error { return nil }
func (m *MockStorage) CountActiveAPIKeysByUserAndAPI(apiId, userID string) (int, error) {
	return 0, nil
}
func (m *MockStorage) ListAPIKeysForArtifactsNotIn(artifactUUIDs, keyUUIDs []string) ([]*models.APIKey, error) {
	return nil, nil
}
func (m *MockStorage) DeleteAPIKeysByUUIDs(uuids []string) error                { return nil }
func (m *MockStorage) SaveSubscriptionPlan(plan *models.SubscriptionPlan) error { return nil }
func (m *MockStorage) GetSubscriptionPlanByID(id, gatewayID string) (*models.SubscriptionPlan, error) {
	return nil, nil
}
func (m *MockStorage) UpdateSubscriptionPlan(plan *models.SubscriptionPlan) error { return nil }
func (m *MockStorage) DeleteSubscriptionPlan(id, gatewayID string) error          { return nil }
func (m *MockStorage) DeleteSubscriptionPlansNotIn(ids []string) error            { return nil }
func (m *MockStorage) SaveSubscription(sub *models.Subscription) error            { return nil }
func (m *MockStorage) GetSubscriptionByID(id, gatewayID string) (*models.Subscription, error) {
	return nil, nil
}
func (m *MockStorage) ListSubscriptionsByAPI(apiID, gatewayID string, applicationID, status *string) ([]*models.Subscription, error) {
	return nil, nil
}
func (m *MockStorage) UpdateSubscription(sub *models.Subscription) error { return nil }
func (m *MockStorage) DeleteSubscription(id, gatewayID string) error     { return nil }
func (m *MockStorage) DeleteSubscriptionsForAPINotIn(apiID string, ids []string) error {
	return nil
}
func (m *MockStorage) ReplaceApplicationAPIKeyMappings(application *models.StoredApplication, mappings []*models.ApplicationAPIKeyMapping) ([]string, error) {
	return nil, nil
}
func (m *MockStorage) SaveCertificate(cert *models.StoredCertificate) error        { return nil }
func (m *MockStorage) GetCertificate(id string) (*models.StoredCertificate, error) { return nil, nil }
func (m *MockStorage) GetCertificateByName(name string) (*models.StoredCertificate, error) {
	return nil, nil
}
func (m *MockStorage) ListCertificates() ([]*models.StoredCertificate, error)     { return nil, nil }
func (m *MockStorage) DeleteCertificate(id string) error                          { return nil }
func (m *MockStorage) SaveSecret(secret *models.Secret) error                     { return nil }
func (m *MockStorage) GetSecrets() ([]models.SecretMeta, error)                   { return nil, nil }
func (m *MockStorage) GetSecret(handle string) (*models.Secret, error)            { return nil, nil }
func (m *MockStorage) UpdateSecret(secret *models.Secret) (*models.Secret, error) { return nil, nil }
func (m *MockStorage) DeleteSecret(handle string) error                           { return nil }
func (m *MockStorage) SecretExists(handle string) (bool, error)                   { return false, nil }
func (m *MockStorage) GetDB() *sql.DB                                             { return nil }
func (m *MockStorage) Close() error                                               { return nil }
func (m *MockStorage) SaveWebhookSecret(secret *models.WebhookSecret) error       { return nil }
func (m *MockStorage) GetWebhookSecretsByArtifact(artifactUUID string) ([]*models.WebhookSecret, error) {
	return nil, nil
}
func (m *MockStorage) GetWebhookSecretByArtifactAndName(artifactUUID, name string) (*models.WebhookSecret, error) {
	return nil, nil
}
func (m *MockStorage) GetWebhookSecretByUUID(uuid string) (*models.WebhookSecret, error) {
	return nil, nil
}
func (m *MockStorage) UpdateWebhookSecret(secret *models.WebhookSecret) error { return nil }
func (m *MockStorage) DeleteWebhookSecret(artifactUUID, name string) error    { return nil }
func (m *MockStorage) GetAllWebhookSecrets() ([]*models.WebhookSecret, error) { return nil, nil }

func TestNewSnapshotManager(t *testing.T) {
	t.Run("creates snapshot manager with nil logger", func(t *testing.T) {
		store := &MockStorage{}
		sm := NewSnapshotManager(store, nil)

		require.NotNil(t, sm)
		assert.NotNil(t, sm.cache)
		assert.NotNil(t, sm.logger)
		assert.Equal(t, int64(0), sm.version)
	})

	t.Run("creates snapshot manager with custom logger", func(t *testing.T) {
		store := &MockStorage{}
		logger := slog.Default()
		sm := NewSnapshotManager(store, logger)

		require.NotNil(t, sm)
		assert.Equal(t, logger, sm.logger)
	})
}

func TestSnapshotManager_GetCache(t *testing.T) {
	store := &MockStorage{}
	sm := NewSnapshotManager(store, nil)

	cache := sm.GetCache()
	assert.NotNil(t, cache)
}

func TestSnapshotManager_UpdateSnapshot(t *testing.T) {
	ctx := context.Background()

	t.Run("successful update with no subscriptions", func(t *testing.T) {
		store := &MockStorage{
			configs:           []*models.StoredConfig{},
			subscriptionPlans: []*models.SubscriptionPlan{},
			subscriptions:     []*models.Subscription{},
		}
		sm := NewSnapshotManager(store, nil)

		err := sm.UpdateSnapshot(ctx)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), sm.version)
	})

	t.Run("successful update with subscriptions", func(t *testing.T) {
		appID := "app-1"
		planID := "plan-1"
		throttleCount := 100
		throttleUnit := "minute"

		store := &MockStorage{
			configs: []*models.StoredConfig{
				{UUID: "api-1", Kind: "RestApi"},
			},
			subscriptionPlans: []*models.SubscriptionPlan{
				{
					ID:                 planID,
					ThrottleLimitCount: &throttleCount,
					ThrottleLimitUnit:  &throttleUnit,
					StopOnQuotaReach:   true,
				},
			},
			subscriptions: []*models.Subscription{
				{
					APIID:                 "api-1",
					ApplicationID:         &appID,
					SubscriptionPlanID:    &planID,
					SubscriptionTokenHash: "token-hash-1",
					Status:                models.SubscriptionStatusActive,
				},
			},
		}
		sm := NewSnapshotManager(store, nil)

		err := sm.UpdateSnapshot(ctx)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), sm.version)
	})

	t.Run("GetAllConfigs error", func(t *testing.T) {
		store := &MockStorage{
			configsErr: errors.New("database connection failed"),
		}
		sm := NewSnapshotManager(store, nil)

		err := sm.UpdateSnapshot(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load configs")
	})

	t.Run("ListSubscriptionPlans error", func(t *testing.T) {
		store := &MockStorage{
			configs:              []*models.StoredConfig{},
			subscriptionPlansErr: errors.New("plans table error"),
		}
		sm := NewSnapshotManager(store, nil)

		err := sm.UpdateSnapshot(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to list subscription plans")
	})

	t.Run("ListActiveSubscriptions error", func(t *testing.T) {
		store := &MockStorage{
			configs:           []*models.StoredConfig{},
			subscriptionPlans: []*models.SubscriptionPlan{},
			subscriptionsErr:  errors.New("subscriptions query failed"),
		}
		sm := NewSnapshotManager(store, nil)

		err := sm.UpdateSnapshot(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to list active subscriptions")
	})

	t.Run("missing subscription plan", func(t *testing.T) {
		planID := "missing-plan-id"
		store := &MockStorage{
			configs: []*models.StoredConfig{
				{UUID: "api-1", Kind: "RestApi"},
			},
			subscriptionPlans: []*models.SubscriptionPlan{}, // no plans
			subscriptions: []*models.Subscription{
				{
					ID:                 "sub-1",
					APIID:              "api-1",
					SubscriptionPlanID: &planID,
					Status:             models.SubscriptionStatusActive,
				},
			},
		}
		sm := NewSnapshotManager(store, nil)

		err := sm.UpdateSnapshot(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "references plan")
		assert.Contains(t, err.Error(), "missing")
	})

	t.Run("skips subscriptions for non-existent APIs", func(t *testing.T) {
		store := &MockStorage{
			configs: []*models.StoredConfig{
				{UUID: "api-1", Kind: "RestApi"},
			},
			subscriptionPlans: []*models.SubscriptionPlan{},
			subscriptions: []*models.Subscription{
				{
					APIID:                 "api-not-exists",
					SubscriptionTokenHash: "token-hash",
					Status:                models.SubscriptionStatusActive,
				},
			},
		}
		sm := NewSnapshotManager(store, nil)

		err := sm.UpdateSnapshot(ctx)
		assert.NoError(t, err)
	})

	t.Run("skips non-RestApi kind configs", func(t *testing.T) {
		store := &MockStorage{
			configs: []*models.StoredConfig{
				{UUID: "api-1", Kind: "WebSocketApi"},
			},
			subscriptionPlans: []*models.SubscriptionPlan{},
			subscriptions: []*models.Subscription{
				{
					APIID:                 "api-1",
					SubscriptionTokenHash: "token-hash",
					Status:                models.SubscriptionStatusActive,
				},
			},
		}
		sm := NewSnapshotManager(store, nil)

		err := sm.UpdateSnapshot(ctx)
		assert.NoError(t, err)
		// Subscription was skipped because API kind was not RestApi
	})

	t.Run("handles nil subscription in list", func(t *testing.T) {
		store := &MockStorage{
			configs: []*models.StoredConfig{
				{UUID: "api-1", Kind: "RestApi"},
			},
			subscriptionPlans: []*models.SubscriptionPlan{},
			subscriptions: []*models.Subscription{
				nil,
				{
					APIID:                 "api-1",
					SubscriptionTokenHash: "token-hash",
					Status:                models.SubscriptionStatusActive,
				},
			},
		}
		sm := NewSnapshotManager(store, nil)

		err := sm.UpdateSnapshot(ctx)
		assert.NoError(t, err)
	})

	t.Run("handles nil config in list", func(t *testing.T) {
		store := &MockStorage{
			configs: []*models.StoredConfig{
				nil,
				{UUID: "api-1", Kind: "RestApi"},
			},
			subscriptionPlans: []*models.SubscriptionPlan{},
			subscriptions:     []*models.Subscription{},
		}
		sm := NewSnapshotManager(store, nil)

		err := sm.UpdateSnapshot(ctx)
		assert.NoError(t, err)
	})

	t.Run("handles nil plan in list", func(t *testing.T) {
		store := &MockStorage{
			configs:           []*models.StoredConfig{},
			subscriptionPlans: []*models.SubscriptionPlan{nil},
			subscriptions:     []*models.Subscription{},
		}
		sm := NewSnapshotManager(store, nil)

		err := sm.UpdateSnapshot(ctx)
		assert.NoError(t, err)
	})

	t.Run("subscription without plan ID", func(t *testing.T) {
		store := &MockStorage{
			configs: []*models.StoredConfig{
				{UUID: "api-1", Kind: "RestApi"},
			},
			subscriptionPlans: []*models.SubscriptionPlan{},
			subscriptions: []*models.Subscription{
				{
					APIID:                 "api-1",
					SubscriptionPlanID:    nil, // no plan
					SubscriptionTokenHash: "token-hash",
					Status:                models.SubscriptionStatusActive,
				},
			},
		}
		sm := NewSnapshotManager(store, nil)

		err := sm.UpdateSnapshot(ctx)
		assert.NoError(t, err)
	})

	t.Run("subscription without application ID", func(t *testing.T) {
		store := &MockStorage{
			configs: []*models.StoredConfig{
				{UUID: "api-1", Kind: "RestApi"},
			},
			subscriptionPlans: []*models.SubscriptionPlan{},
			subscriptions: []*models.Subscription{
				{
					APIID:                 "api-1",
					ApplicationID:         nil, // no application
					SubscriptionTokenHash: "token-hash",
					Status:                models.SubscriptionStatusActive,
				},
			},
		}
		sm := NewSnapshotManager(store, nil)

		err := sm.UpdateSnapshot(ctx)
		assert.NoError(t, err)
	})

	t.Run("version increments on each successful update", func(t *testing.T) {
		store := &MockStorage{
			configs:           []*models.StoredConfig{},
			subscriptionPlans: []*models.SubscriptionPlan{},
			subscriptions:     []*models.Subscription{},
		}
		sm := NewSnapshotManager(store, nil)

		err := sm.UpdateSnapshot(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(1), sm.version)

		err = sm.UpdateSnapshot(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(2), sm.version)

		err = sm.UpdateSnapshot(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(3), sm.version)
	})

	t.Run("plan with nil throttle limit count", func(t *testing.T) {
		planID := "plan-no-limit"
		throttleUnit := "hour"
		store := &MockStorage{
			configs: []*models.StoredConfig{
				{UUID: "api-1", Kind: "RestApi"},
			},
			subscriptionPlans: []*models.SubscriptionPlan{
				{
					ID:                 planID,
					ThrottleLimitCount: nil, // unlimited
					ThrottleLimitUnit:  &throttleUnit,
					StopOnQuotaReach:   false,
				},
			},
			subscriptions: []*models.Subscription{
				{
					APIID:                 "api-1",
					SubscriptionPlanID:    &planID,
					SubscriptionTokenHash: "token-hash",
					Status:                models.SubscriptionStatusActive,
				},
			},
		}
		sm := NewSnapshotManager(store, nil)

		err := sm.UpdateSnapshot(ctx)
		assert.NoError(t, err)
	})

	t.Run("plan with nil throttle limit unit", func(t *testing.T) {
		planID := "plan-no-unit"
		throttleCount := 1000
		store := &MockStorage{
			configs: []*models.StoredConfig{
				{UUID: "api-1", Kind: "RestApi"},
			},
			subscriptionPlans: []*models.SubscriptionPlan{
				{
					ID:                 planID,
					ThrottleLimitCount: &throttleCount,
					ThrottleLimitUnit:  nil, // unset
					StopOnQuotaReach:   true,
				},
			},
			subscriptions: []*models.Subscription{
				{
					APIID:                 "api-1",
					SubscriptionPlanID:    &planID,
					SubscriptionTokenHash: "token-hash",
					Status:                models.SubscriptionStatusActive,
				},
			},
		}
		sm := NewSnapshotManager(store, nil)

		err := sm.UpdateSnapshot(ctx)
		assert.NoError(t, err)
	})
}

// snapshotSubscriptions decodes the subscriptions currently published in the
// manager's xDS cache, so tests can assert on what the policy engine actually
// receives rather than only that UpdateSnapshot returned no error.
func snapshotSubscriptions(t *testing.T, sm *SnapshotManager) []policyenginev1.SubscriptionData {
	t.Helper()

	resource, ok := sm.cache.GetResources()["subscription-state"]
	require.True(t, ok, "subscription-state resource missing from cache")

	anyMsg, ok := resource.(*anypb.Any)
	require.True(t, ok, "cached resource is not an *anypb.Any")

	// anypb.New stamped a struct payload before the TypeUrl was overwritten with
	// the custom SubscriptionStateTypeURL, so unmarshal the struct directly.
	st := &structpb.Struct{}
	require.NoError(t, proto.Unmarshal(anyMsg.Value, st))

	jsonBytes, err := st.MarshalJSON()
	require.NoError(t, err)

	var state policyenginev1.SubscriptionStateResource
	require.NoError(t, json.Unmarshal(jsonBytes, &state))
	return state.Subscriptions
}

// A bottom-up (DP->CP) synced API is stored on the gateway under its locally
// generated UUID with the control-plane UUID recorded as cp_artifact_id, so
// subscriptions broadcast by the control plane arrive keyed by the CP UUID.
// They must still reach the policy engine, keyed by the local UUID that the
// Envoy route metadata uses.
func TestSnapshotManager_UpdateSnapshot_DPToCPSyncedAPI(t *testing.T) {
	ctx := context.Background()

	t.Run("accepts a subscription keyed by the control-plane UUID", func(t *testing.T) {
		store := &MockStorage{
			configs: []*models.StoredConfig{
				{UUID: "gw-uuid-1", CPArtifactID: "cp-uuid-1", Kind: models.KindRestApi},
			},
			subscriptionPlans: []*models.SubscriptionPlan{},
			subscriptions: []*models.Subscription{
				{
					ID:                    "sub-1",
					APIID:                 "cp-uuid-1",
					SubscriptionTokenHash: "token-hash-1",
					Status:                models.SubscriptionStatusActive,
				},
			},
		}
		sm := NewSnapshotManager(store, nil)

		require.NoError(t, sm.UpdateSnapshot(ctx))

		subs := snapshotSubscriptions(t, sm)
		require.Len(t, subs, 1)
		// Emitted under the local UUID — the policy engine matches on that, not the CP one.
		assert.Equal(t, "gw-uuid-1", subs[0].APIId)
		assert.Equal(t, "token-hash-1", subs[0].SubscriptionToken)
	})

	t.Run("still accepts a subscription keyed by the local UUID", func(t *testing.T) {
		store := &MockStorage{
			configs: []*models.StoredConfig{
				{UUID: "gw-uuid-1", CPArtifactID: "cp-uuid-1", Kind: models.KindRestApi},
			},
			subscriptionPlans: []*models.SubscriptionPlan{},
			subscriptions: []*models.Subscription{
				{
					ID:                    "sub-1",
					APIID:                 "gw-uuid-1",
					SubscriptionTokenHash: "token-hash-1",
					Status:                models.SubscriptionStatusActive,
				},
			},
		}
		sm := NewSnapshotManager(store, nil)

		require.NoError(t, sm.UpdateSnapshot(ctx))

		subs := snapshotSubscriptions(t, sm)
		require.Len(t, subs, 1)
		assert.Equal(t, "gw-uuid-1", subs[0].APIId)
	})

	t.Run("a CP UUID belonging to another API is still rejected", func(t *testing.T) {
		store := &MockStorage{
			configs: []*models.StoredConfig{
				{UUID: "gw-uuid-1", CPArtifactID: "cp-uuid-1", Kind: models.KindRestApi},
			},
			subscriptionPlans: []*models.SubscriptionPlan{},
			subscriptions: []*models.Subscription{
				{
					ID:                    "sub-1",
					APIID:                 "cp-uuid-other",
					SubscriptionTokenHash: "token-hash-1",
					Status:                models.SubscriptionStatusActive,
				},
			},
		}
		sm := NewSnapshotManager(store, nil)

		require.NoError(t, sm.UpdateSnapshot(ctx))
		assert.Empty(t, snapshotSubscriptions(t, sm))
	})

	t.Run("a cp_artifact_id on a non-RestApi config is not accepted", func(t *testing.T) {
		store := &MockStorage{
			configs: []*models.StoredConfig{
				{UUID: "gw-uuid-1", CPArtifactID: "cp-uuid-1", Kind: models.KindMcp},
			},
			subscriptionPlans: []*models.SubscriptionPlan{},
			subscriptions: []*models.Subscription{
				{
					ID:                    "sub-1",
					APIID:                 "cp-uuid-1",
					SubscriptionTokenHash: "token-hash-1",
					Status:                models.SubscriptionStatusActive,
				},
			},
		}
		sm := NewSnapshotManager(store, nil)

		require.NoError(t, sm.UpdateSnapshot(ctx))
		assert.Empty(t, snapshotSubscriptions(t, sm))
	})
}

func TestSubscriptionStateTypeURL(t *testing.T) {
	assert.Equal(t, "api-platform.wso2.org/v1.SubscriptionState", SubscriptionStateTypeURL)
}
