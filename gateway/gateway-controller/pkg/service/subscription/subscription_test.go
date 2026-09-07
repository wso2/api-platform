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

package subscription

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// Subscription and stored-config halves of the fakeStore declared in
// service_test.go. Both files back the same type; only the plan half lives
// there.

func (s *fakeStore) GetConfig(id string) (*models.StoredConfig, error) {
	*s.calls = append(*s.calls, "get_config")
	if s.configErr != nil {
		return nil, s.configErr
	}
	cfg, ok := s.configs[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", storage.ErrNotFound, id)
	}
	return cfg, nil
}

func (s *fakeStore) GetConfigByKindAndHandle(_ string, handle string) (*models.StoredConfig, error) {
	*s.calls = append(*s.calls, "get_config_by_handle")
	if s.handleErr != nil {
		return nil, s.handleErr
	}
	cfg, ok := s.byHandle[handle]
	if !ok {
		return nil, fmt.Errorf("%w: %s", storage.ErrNotFound, handle)
	}
	return cfg, nil
}

func (s *fakeStore) SaveSubscription(sub *models.Subscription) error {
	*s.calls = append(*s.calls, "save_subscription")
	if s.saveSubErr != nil {
		return s.saveSubErr
	}
	stored := *sub
	s.subs[sub.ID] = &stored
	return nil
}

func (s *fakeStore) GetSubscriptionByID(id, _ string) (*models.Subscription, error) {
	*s.calls = append(*s.calls, "get_subscription")
	if s.getSubErr != nil {
		return nil, s.getSubErr
	}
	sub, ok := s.subs[id]
	if !ok {
		return nil, nil
	}
	stored := *sub
	return &stored, nil
}

func (s *fakeStore) ListSubscriptionsByAPI(apiID, _ string, applicationID, status *string) ([]*models.Subscription, error) {
	*s.calls = append(*s.calls, "list_subscriptions")
	if s.listSubErr != nil {
		return nil, s.listSubErr
	}
	out := make([]*models.Subscription, 0, len(s.subs))
	for _, sub := range s.subs {
		if apiID != "" && sub.APIID != apiID {
			continue
		}
		if applicationID != nil && (sub.ApplicationID == nil || *sub.ApplicationID != *applicationID) {
			continue
		}
		if status != nil && string(sub.Status) != *status {
			continue
		}
		out = append(out, sub)
	}
	return out, nil
}

func (s *fakeStore) UpdateSubscription(sub *models.Subscription) error {
	*s.calls = append(*s.calls, "update_subscription")
	if s.updateSubErr != nil {
		return s.updateSubErr
	}
	stored := *sub
	s.subs[sub.ID] = &stored
	return nil
}

func (s *fakeStore) DeleteSubscription(id, _ string) error {
	*s.calls = append(*s.calls, "delete_subscription")
	if s.deleteSubErr != nil {
		return s.deleteSubErr
	}
	delete(s.subs, id)
	return nil
}

// seedRestAPI registers a stored RestApi reachable by both deployment ID and
// handle. plans, when non-nil, becomes spec.subscriptionPlans.
func seedRestAPI(store *fakeStore, uuid, handle string, plans *[]string) {
	cfg := &models.StoredConfig{
		UUID:   uuid,
		Handle: handle,
		Kind:   string(api.RestAPIKindRestApi),
		Configuration: api.RestAPI{
			Spec: api.APIConfigData{SubscriptionPlans: plans},
		},
	}
	store.configs[uuid] = cfg
	store.byHandle[handle] = cfg
}

func subCreateStatus(v api.SubscriptionCreateRequestStatus) *api.SubscriptionCreateRequestStatus {
	return &v
}

func subUpdateStatus(v api.SubscriptionUpdateRequestStatus) *api.SubscriptionUpdateRequestStatus {
	return &v
}

func TestResolveAPIID(t *testing.T) {
	t.Run("resolves a deployment ID directly", func(t *testing.T) {
		svc, store, _, calls := newTestService(t)
		seedRestAPI(store, "api-uuid", "orders", nil)

		id, err := svc.ResolveAPIID("api-uuid")
		require.NoError(t, err)
		assert.Equal(t, "api-uuid", id)
		// Resolved on the first lookup; the handle fallback is never reached.
		assert.Equal(t, []string{"get_config"}, *calls)
	})

	t.Run("falls back to the handle", func(t *testing.T) {
		svc, store, _, calls := newTestService(t)
		seedRestAPI(store, "api-uuid", "orders", nil)

		id, err := svc.ResolveAPIID("orders")
		require.NoError(t, err)
		assert.Equal(t, "api-uuid", id)
		assert.Equal(t, []string{"get_config", "get_config_by_handle"}, *calls)
	})

	t.Run("rejects a non-RestApi artifact", func(t *testing.T) {
		svc, store, _, _ := newTestService(t)
		store.configs["secret-uuid"] = &models.StoredConfig{
			UUID: "secret-uuid",
			Kind: models.KindSecret,
		}

		_, err := svc.ResolveAPIID("secret-uuid")

		var notRestAPI *NotRestAPIError
		require.ErrorAs(t, err, &notRestAPI)
		assert.Equal(t, models.KindSecret, notRestAPI.Kind)
		assert.Equal(t, "secret-uuid", notRestAPI.Identifier)
	})

	t.Run("unknown identifier", func(t *testing.T) {
		svc, _, _, _ := newTestService(t)

		_, err := svc.ResolveAPIID("nope")
		assert.ErrorIs(t, err, ErrAPINotFound)
	})

	t.Run("unexpected storage failure on the ID lookup", func(t *testing.T) {
		svc, store, _, _ := newTestService(t)
		store.configErr = errors.New("database unavailable")

		_, err := svc.ResolveAPIID("anything")

		var opErr *OpError
		require.ErrorAs(t, err, &opErr)
		assert.Equal(t, OpResolve, opErr.Op)
	})

	t.Run("unexpected storage failure on the handle lookup", func(t *testing.T) {
		svc, store, _, _ := newTestService(t)
		store.handleErr = errors.New("database unavailable")

		_, err := svc.ResolveAPIID("orders")

		var opErr *OpError
		require.ErrorAs(t, err, &opErr)
		assert.Equal(t, OpResolve, opErr.Op)
	})
}

func TestCreate_AcceptsHandleAndPersistsThenPublishes(t *testing.T) {
	svc, store, hub, calls := newTestService(t)
	seedRestAPI(store, "api-uuid", "orders", nil)

	appID := "app-1"
	result, err := svc.Create(CreateParams{
		Request: api.SubscriptionCreateRequest{
			ApiId:             "orders",
			SubscriptionToken: "  tok-123  ",
			ApplicationId:     &appID,
		},
		CorrelationID: "corr-1",
		Logger:        testLogger(),
	})
	require.NoError(t, err)

	// The handle is resolved to the deployment ID, and the token is trimmed.
	assert.Equal(t, "api-uuid", result.Subscription.APIID)
	assert.Equal(t, "tok-123", result.Subscription.SubscriptionToken)
	assert.Equal(t, models.SubscriptionStatusActive, result.Subscription.Status)
	require.NotNil(t, result.Subscription.ApplicationID)
	assert.Equal(t, "app-1", *result.Subscription.ApplicationID)

	assert.Equal(t, []string{"get_config", "get_config_by_handle", "save_subscription", "publish_event"}, *calls)
	require.Len(t, hub.events, 1)
	assert.Equal(t, eventhub.EventTypeSubscription, hub.events[0].EventType)
	assert.Equal(t, "CREATE", hub.events[0].Action)
	assert.Equal(t, "corr-1", hub.events[0].EventID)
}

func TestCreate_RequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		request api.SubscriptionCreateRequest
		message string
	}{
		{
			name:    "missing apiId",
			request: api.SubscriptionCreateRequest{SubscriptionToken: "tok"},
			message: "apiId is required",
		},
		{
			name:    "blank apiId",
			request: api.SubscriptionCreateRequest{ApiId: "   ", SubscriptionToken: "tok"},
			message: "apiId is required",
		},
		{
			name:    "missing subscriptionToken",
			request: api.SubscriptionCreateRequest{ApiId: "orders"},
			message: "subscriptionToken is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _, calls := newTestService(t)

			_, err := svc.Create(CreateParams{Request: tc.request, Logger: testLogger()})

			var validationErr *ValidationError
			require.ErrorAs(t, err, &validationErr)
			assert.Equal(t, tc.message, validationErr.Message)
			assert.Empty(t, *calls, "a rejected request must not reach storage")
		})
	}
}

func TestCreate_UnknownAPI(t *testing.T) {
	svc, _, _, _ := newTestService(t)

	_, err := svc.Create(CreateParams{
		Request: api.SubscriptionCreateRequest{ApiId: "missing", SubscriptionToken: "tok"},
		Logger:  testLogger(),
	})

	assert.ErrorIs(t, err, ErrAPINotFound)
}

func TestCreate_InvalidStatus(t *testing.T) {
	svc, store, _, _ := newTestService(t)
	seedRestAPI(store, "api-uuid", "orders", nil)

	_, err := svc.Create(CreateParams{
		Request: api.SubscriptionCreateRequest{
			ApiId:             "api-uuid",
			SubscriptionToken: "tok",
			Status:            subCreateStatus("PENDING"),
		},
		Logger: testLogger(),
	})

	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, "invalid status: PENDING", validationErr.Message)
}

func TestCreate_PlanValidation(t *testing.T) {
	activePlan := &models.SubscriptionPlan{
		ID:       "plan-1",
		PlanName: "Gold",
		Status:   models.SubscriptionPlanStatusActive,
	}

	t.Run("plan does not exist", func(t *testing.T) {
		svc, store, _, _ := newTestService(t)
		seedRestAPI(store, "api-uuid", "orders", nil)

		planID := "missing-plan"
		_, err := svc.Create(CreateParams{
			Request: api.SubscriptionCreateRequest{
				ApiId: "api-uuid", SubscriptionToken: "tok", SubscriptionPlanId: &planID,
			},
			Logger: testLogger(),
		})

		var validationErr *ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, "Subscription plan not found or not enabled", validationErr.Message)
	})

	t.Run("plan is not active", func(t *testing.T) {
		svc, store, _, _ := newTestService(t)
		seedRestAPI(store, "api-uuid", "orders", nil)
		store.plans["plan-1"] = &models.SubscriptionPlan{
			ID: "plan-1", PlanName: "Gold", Status: models.SubscriptionPlanStatusInactive,
		}

		planID := "plan-1"
		_, err := svc.Create(CreateParams{
			Request: api.SubscriptionCreateRequest{
				ApiId: "api-uuid", SubscriptionToken: "tok", SubscriptionPlanId: &planID,
			},
			Logger: testLogger(),
		})

		var validationErr *ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, "Subscription plan is not active", validationErr.Message)
	})

	t.Run("plan is not offered by this API", func(t *testing.T) {
		svc, store, _, _ := newTestService(t)
		offered := []string{"Silver", "Bronze"}
		seedRestAPI(store, "api-uuid", "orders", &offered)
		store.plans["plan-1"] = activePlan

		planID := "plan-1"
		_, err := svc.Create(CreateParams{
			Request: api.SubscriptionCreateRequest{
				ApiId: "api-uuid", SubscriptionToken: "tok", SubscriptionPlanId: &planID,
			},
			Logger: testLogger(),
		})

		var validationErr *ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, `Subscription plan "Gold" is not enabled for this API`, validationErr.Message)
	})

	t.Run("plan match is case-insensitive", func(t *testing.T) {
		svc, store, _, _ := newTestService(t)
		offered := []string{"gold"}
		seedRestAPI(store, "api-uuid", "orders", &offered)
		store.plans["plan-1"] = activePlan

		planID := "plan-1"
		_, err := svc.Create(CreateParams{
			Request: api.SubscriptionCreateRequest{
				ApiId: "api-uuid", SubscriptionToken: "tok", SubscriptionPlanId: &planID,
			},
			Logger: testLogger(),
		})
		assert.NoError(t, err)
	})

	t.Run("an API with no declared plans accepts any active plan", func(t *testing.T) {
		svc, store, _, _ := newTestService(t)
		empty := []string{}
		seedRestAPI(store, "api-uuid", "orders", &empty)
		store.plans["plan-1"] = activePlan

		planID := "plan-1"
		_, err := svc.Create(CreateParams{
			Request: api.SubscriptionCreateRequest{
				ApiId: "api-uuid", SubscriptionToken: "tok", SubscriptionPlanId: &planID,
			},
			Logger: testLogger(),
		})
		assert.NoError(t, err)
	})
}

func TestCreate_ConflictRemainsDetectable(t *testing.T) {
	svc, store, _, _ := newTestService(t)
	seedRestAPI(store, "api-uuid", "orders", nil)
	store.saveSubErr = fmt.Errorf("%w: subscription token already exists for this API", storage.ErrConflict)

	_, err := svc.Create(CreateParams{
		Request: api.SubscriptionCreateRequest{ApiId: "api-uuid", SubscriptionToken: "tok"},
		Logger:  testLogger(),
	})

	// OpError must unwrap, or the handler could not answer 409.
	require.Error(t, err)
	assert.True(t, storage.IsConflictError(err))
}

func TestGet_NotFound(t *testing.T) {
	t.Run("nil row", func(t *testing.T) {
		svc, _, _, _ := newTestService(t)

		_, err := svc.Get("missing")
		assert.ErrorIs(t, err, ErrSubscriptionNotFound)
	})

	t.Run("storage not-found error", func(t *testing.T) {
		svc, store, _, _ := newTestService(t)
		store.getSubErr = fmt.Errorf("%w: subscription", storage.ErrNotFound)

		_, err := svc.Get("missing")
		assert.ErrorIs(t, err, ErrSubscriptionNotFound)
	})

	t.Run("unexpected storage error is a load failure", func(t *testing.T) {
		svc, store, _, _ := newTestService(t)
		store.getSubErr = errors.New("database unavailable")

		_, err := svc.Get("any")

		var opErr *OpError
		require.ErrorAs(t, err, &opErr)
		assert.Equal(t, OpLoad, opErr.Op)
	})
}

func TestUpdate_ChangesStatusOnly(t *testing.T) {
	svc, store, _, calls := newTestService(t)
	store.subs["sub-1"] = &models.Subscription{
		ID:     "sub-1",
		APIID:  "api-uuid",
		Status: models.SubscriptionStatusActive,
	}

	result, err := svc.Update(UpdateParams{
		ID:            "sub-1",
		Request:       api.SubscriptionUpdateRequest{Status: subUpdateStatus(api.SubscriptionUpdateRequestStatusINACTIVE)},
		CorrelationID: "corr-2",
		Logger:        testLogger(),
	})
	require.NoError(t, err)

	assert.Equal(t, models.SubscriptionStatusInactive, result.Subscription.Status)
	// The API binding is untouched — an update cannot re-point a subscription.
	assert.Equal(t, "api-uuid", result.Subscription.APIID)
	assert.Equal(t, []string{"get_subscription", "update_subscription", "publish_event"}, *calls)
}

func TestUpdate_NoStatusIsANoOpWrite(t *testing.T) {
	svc, store, _, _ := newTestService(t)
	store.subs["sub-1"] = &models.Subscription{ID: "sub-1", Status: models.SubscriptionStatusActive}

	result, err := svc.Update(UpdateParams{ID: "sub-1", Logger: testLogger()})
	require.NoError(t, err)

	assert.Equal(t, models.SubscriptionStatusActive, result.Subscription.Status)
}

func TestUpdate_InvalidStatusDoesNotWrite(t *testing.T) {
	svc, store, _, calls := newTestService(t)
	store.subs["sub-1"] = &models.Subscription{ID: "sub-1", Status: models.SubscriptionStatusActive}

	_, err := svc.Update(UpdateParams{
		ID:      "sub-1",
		Request: api.SubscriptionUpdateRequest{Status: subUpdateStatus("PENDING")},
		Logger:  testLogger(),
	})

	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, "invalid status: PENDING", validationErr.Message)
	assert.Equal(t, []string{"get_subscription"}, *calls)
}

func TestUpdate_NotFound(t *testing.T) {
	svc, _, _, _ := newTestService(t)

	_, err := svc.Update(UpdateParams{ID: "missing", Logger: testLogger()})
	assert.ErrorIs(t, err, ErrSubscriptionNotFound)
}

func TestList_Filters(t *testing.T) {
	svc, store, _, _ := newTestService(t)
	appA := "app-a"
	store.subs["s1"] = &models.Subscription{ID: "s1", APIID: "api-1", ApplicationID: &appA, Status: models.SubscriptionStatusActive}
	store.subs["s2"] = &models.Subscription{ID: "s2", APIID: "api-2", Status: models.SubscriptionStatusRevoked}

	all, err := svc.List(ListFilter{})
	require.NoError(t, err)
	assert.Len(t, all.Subscriptions, 2, "an empty APIID means every API on the gateway")

	byAPI, err := svc.List(ListFilter{APIID: "api-1"})
	require.NoError(t, err)
	require.Len(t, byAPI.Subscriptions, 1)
	assert.Equal(t, "s1", byAPI.Subscriptions[0].ID)

	revoked := string(models.SubscriptionStatusRevoked)
	byStatus, err := svc.List(ListFilter{Status: &revoked})
	require.NoError(t, err)
	require.Len(t, byStatus.Subscriptions, 1)
	assert.Equal(t, "s2", byStatus.Subscriptions[0].ID)
}

func TestList_StorageFailure(t *testing.T) {
	svc, store, _, _ := newTestService(t)
	store.listSubErr = errors.New("database unavailable")

	_, err := svc.List(ListFilter{})

	var opErr *OpError
	require.ErrorAs(t, err, &opErr)
	assert.Equal(t, OpList, opErr.Op)
}

func TestDelete(t *testing.T) {
	t.Run("removes the subscription and publishes", func(t *testing.T) {
		svc, store, hub, calls := newTestService(t)
		store.subs["sub-1"] = &models.Subscription{ID: "sub-1"}

		require.NoError(t, svc.Delete("sub-1", "corr-3", testLogger()))

		assert.NotContains(t, store.subs, "sub-1")
		assert.Equal(t, []string{"get_subscription", "delete_subscription", "publish_event"}, *calls)
		require.Len(t, hub.events, 1)
		assert.Equal(t, "DELETE", hub.events[0].Action)
	})

	t.Run("missing subscription is rejected before the delete", func(t *testing.T) {
		svc, _, _, calls := newTestService(t)

		err := svc.Delete("missing", "", testLogger())

		assert.ErrorIs(t, err, ErrSubscriptionNotFound)
		assert.Equal(t, []string{"get_subscription"}, *calls)
	})
}

func TestParseSubscriptionStatus(t *testing.T) {
	for _, raw := range []string{"ACTIVE", "INACTIVE", "REVOKED"} {
		parsed, err := parseSubscriptionStatus(raw)
		require.NoError(t, err)
		assert.Equal(t, models.SubscriptionStatus(raw), parsed)
	}

	// Case matters: the enum is upper-case and the storage column is not
	// normalised, so "active" must be rejected rather than silently accepted.
	_, err := parseSubscriptionStatus("active")
	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, "invalid status: active", validationErr.Message)
}
