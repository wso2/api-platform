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
	"io"
	"log/slog"
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

// Test setup

// fakeStore is a storage.Storage that implements only the stored-config,
// subscription and subscription-plan methods these tests exercise.
// storage.Storage is embedded as a nil interface, so any other database call
// panics rather than silently returning a zero value — an unexpected read
// fails loudly instead of passing.
type fakeStore struct {
	storage.Storage

	plans    map[string]*models.SubscriptionPlan
	subs     map[string]*models.Subscription
	configs  map[string]*models.StoredConfig
	byHandle map[string]*models.StoredConfig
	calls    *[]string

	saveErr   error
	getErr    error
	listErr   error
	updateErr error
	deleteErr error

	configErr    error
	handleErr    error
	saveSubErr   error
	getSubErr    error
	listSubErr   error
	updateSubErr error
	deleteSubErr error
}

func newFakeStore(calls *[]string) *fakeStore {
	return &fakeStore{
		plans:    map[string]*models.SubscriptionPlan{},
		subs:     map[string]*models.Subscription{},
		configs:  map[string]*models.StoredConfig{},
		byHandle: map[string]*models.StoredConfig{},
		calls:    calls,
	}
}

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

func (s *fakeStore) SaveSubscriptionPlan(plan *models.SubscriptionPlan) error {
	*s.calls = append(*s.calls, "save_plan")
	if s.saveErr != nil {
		return s.saveErr
	}
	stored := *plan
	s.plans[plan.ID] = &stored
	return nil
}

func (s *fakeStore) GetSubscriptionPlanByID(id, _ string) (*models.SubscriptionPlan, error) {
	*s.calls = append(*s.calls, "get_plan")
	if s.getErr != nil {
		return nil, s.getErr
	}
	plan, ok := s.plans[id]
	if !ok {
		return nil, nil
	}
	stored := *plan
	return &stored, nil
}

func (s *fakeStore) ListSubscriptionPlans(_ string) ([]*models.SubscriptionPlan, error) {
	*s.calls = append(*s.calls, "list_plans")
	if s.listErr != nil {
		return nil, s.listErr
	}
	out := make([]*models.SubscriptionPlan, 0, len(s.plans))
	for _, plan := range s.plans {
		out = append(out, plan)
	}
	return out, nil
}

func (s *fakeStore) UpdateSubscriptionPlan(plan *models.SubscriptionPlan) error {
	*s.calls = append(*s.calls, "update_plan")
	if s.updateErr != nil {
		return s.updateErr
	}
	stored := *plan
	s.plans[plan.ID] = &stored
	return nil
}

func (s *fakeStore) DeleteSubscriptionPlan(id, _ string) error {
	*s.calls = append(*s.calls, "delete_plan")
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.plans, id)
	return nil
}

// recordingHub captures published events so a test can assert that persistence
// happened before the replica-sync event, not after.
type recordingHub struct {
	calls  *[]string
	events []eventhub.Event
}

func (h *recordingHub) Initialize() error            { return nil }
func (h *recordingHub) RegisterGateway(string) error { return nil }
func (h *recordingHub) UnsubscribeAll(string) error  { return nil }
func (h *recordingHub) CleanUpEvents() error         { return nil }
func (h *recordingHub) Close() error                 { return nil }

func (h *recordingHub) PublishEvent(_ string, event eventhub.Event) error {
	*h.calls = append(*h.calls, "publish_event")
	h.events = append(h.events, event)
	return nil
}

func (h *recordingHub) Subscribe(string) (<-chan eventhub.Event, error) { return nil, nil }

func (h *recordingHub) Unsubscribe(string, <-chan eventhub.Event) error { return nil }

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestService(t *testing.T) (*SubscriptionService, *fakeStore, *recordingHub, *[]string) {
	t.Helper()

	calls := &[]string{}
	store := newFakeStore(calls)
	hub := &recordingHub{calls: calls}
	resources := utils.NewSubscriptionResourceService(store, nil, hub, "test-gateway")

	return NewSubscriptionService(store, resources), store, hub, calls
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

// seedPlan stores a subscription plan with the given status.
func seedPlan(store *fakeStore, id, name string, status models.SubscriptionPlanStatus) {
	store.plans[id] = &models.SubscriptionPlan{ID: id, PlanName: name, Status: status}
}

func subCreateStatus(v api.SubscriptionCreateRequestStatus) *api.SubscriptionCreateRequestStatus {
	return &v
}

func subUpdateStatus(v api.SubscriptionUpdateRequestStatus) *api.SubscriptionUpdateRequestStatus {
	return &v
}

func planCreateStatus(v api.SubscriptionPlanCreateRequestStatus) *api.SubscriptionPlanCreateRequestStatus {
	return &v
}

func planUpdateStatus(v api.SubscriptionPlanUpdateRequestStatus) *api.SubscriptionPlanUpdateRequestStatus {
	return &v
}

func planCreateUnit(v api.SubscriptionPlanCreateRequestThrottleLimitUnit) *api.SubscriptionPlanCreateRequestThrottleLimitUnit {
	return &v
}

func TestNewSubscriptionService_PanicsOnMissingDependencies(t *testing.T) {
	store := newFakeStore(&[]string{})
	resources := utils.NewSubscriptionResourceService(store, nil, &recordingHub{calls: &[]string{}}, "test-gateway")

	assert.PanicsWithValue(t, "SubscriptionService requires non-nil storage", func() {
		NewSubscriptionService(nil, resources)
	})
	assert.PanicsWithValue(t, "SubscriptionService requires SubscriptionResourceService", func() {
		NewSubscriptionService(store, nil)
	})
}

// Subscriptions

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

func TestCreate_ExternalRefID(t *testing.T) {
	t.Run("becomes the subscription ID", func(t *testing.T) {
		svc, store, _, _ := newTestService(t)
		seedRestAPI(store, "api-uuid", "orders", nil)
		ref := "apim-sub-7f1c"

		result, err := svc.Create(CreateParams{Request: api.SubscriptionCreateRequest{
			ApiId: "orders", SubscriptionToken: "t", ExternalRefId: &ref}, Logger: testLogger()})

		require.NoError(t, err)
		assert.Equal(t, ref, result.Subscription.ID)
	})

	t.Run("an empty value is treated as absent", func(t *testing.T) {
		svc, store, _, _ := newTestService(t)
		seedRestAPI(store, "api-uuid", "orders", nil)
		empty := ""

		result, err := svc.Create(CreateParams{Request: api.SubscriptionCreateRequest{
			ApiId: "orders", SubscriptionToken: "t", ExternalRefId: &empty}, Logger: testLogger()})

		require.NoError(t, err)
		assert.NotEmpty(t, result.Subscription.ID)
	})

	t.Run("an invalid value is refused before the API is resolved", func(t *testing.T) {
		svc, _, _, calls := newTestService(t)
		ref := "-starts-with-a-dash"

		_, err := svc.Create(CreateParams{Request: api.SubscriptionCreateRequest{
			ApiId: "orders", SubscriptionToken: "t", ExternalRefId: &ref}, Logger: testLogger()})

		var validationErr *ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, "externalRefId must be 1-64 characters of letters, digits, '.', '_' or '-', "+
			"starting with a letter or digit", validationErr.Message)
		assert.Empty(t, *calls)
	})
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

func TestUpdate_Status(t *testing.T) {
	svc, store, _, calls := newTestService(t)
	existing := &models.Subscription{
		ID:     "sub-1",
		APIID:  "api-uuid",
		Status: models.SubscriptionStatusActive,
	}
	store.subs["sub-1"] = existing

	result, err := svc.Update(UpdateParams{
		Existing:      existing,
		Request:       api.SubscriptionUpdateRequest{Status: subUpdateStatus(api.SubscriptionUpdateRequestStatusINACTIVE)},
		CorrelationID: "corr-2",
		Logger:        testLogger(),
	})
	require.NoError(t, err)

	assert.Equal(t, models.SubscriptionStatusInactive, result.Subscription.Status)
	// The API binding is untouched — an update cannot move a subscription to another API.
	assert.Equal(t, "api-uuid", result.Subscription.APIID)
	assert.Equal(t, []string{"update_subscription", "publish_event"}, *calls)
}

func TestUpdate_InvalidStatusDoesNotWrite(t *testing.T) {
	svc, store, _, calls := newTestService(t)
	store.subs["sub-1"] = &models.Subscription{ID: "sub-1", Status: models.SubscriptionStatusActive}

	_, err := svc.Update(UpdateParams{
		Existing: store.subs["sub-1"],
		Request:  api.SubscriptionUpdateRequest{Status: subUpdateStatus("PENDING")},
		Logger:   testLogger(),
	})

	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, "invalid status: PENDING", validationErr.Message)
	assert.Empty(t, *calls)
}

func TestUpdate_Plan(t *testing.T) {
	newSub := func(planID *string) *models.Subscription {
		return &models.Subscription{ID: "sub-1", APIID: "api-uuid", Status: models.SubscriptionStatusActive, SubscriptionPlanID: planID}
	}
	gold := "plan-gold"

	t.Run("moves the subscription to another plan", func(t *testing.T) {
		svc, store, _, _ := newTestService(t)
		seedRestAPI(store, "api-uuid", "orders", nil)
		seedPlan(store, gold, "gold", models.SubscriptionPlanStatusActive)

		result, err := svc.Update(UpdateParams{Existing: newSub(nil),
			Request: api.SubscriptionUpdateRequest{SubscriptionPlanId: &gold},
			Fields:  map[string]interface{}{"subscriptionPlanId": gold}, Logger: testLogger()})

		require.NoError(t, err)
		require.NotNil(t, result.Subscription.SubscriptionPlanID)
		assert.Equal(t, gold, *result.Subscription.SubscriptionPlanID)
	})

	t.Run("an explicit null removes the plan", func(t *testing.T) {
		svc, _, _, _ := newTestService(t)

		result, err := svc.Update(UpdateParams{Existing: newSub(&gold),
			Fields: map[string]interface{}{"subscriptionPlanId": nil}, Logger: testLogger()})

		require.NoError(t, err)
		assert.Nil(t, result.Subscription.SubscriptionPlanID)
	})

	t.Run("an omitted plan is kept", func(t *testing.T) {
		svc, _, _, _ := newTestService(t)

		result, err := svc.Update(UpdateParams{Existing: newSub(&gold), Fields: map[string]interface{}{}, Logger: testLogger()})

		require.NoError(t, err)
		require.NotNil(t, result.Subscription.SubscriptionPlanID)
		assert.Equal(t, gold, *result.Subscription.SubscriptionPlanID)
	})

	t.Run("an unchanged plan is not re-validated", func(t *testing.T) {
		svc, store, _, calls := newTestService(t)
		// Inactive, so a re-validation would refuse the update.
		seedPlan(store, gold, "gold", models.SubscriptionPlanStatusInactive)

		_, err := svc.Update(UpdateParams{Existing: newSub(&gold),
			Request: api.SubscriptionUpdateRequest{SubscriptionPlanId: &gold},
			Fields:  map[string]interface{}{"subscriptionPlanId": gold}, Logger: testLogger()})

		require.NoError(t, err)
		assert.NotContains(t, *calls, "get_plan")
	})

	t.Run("a new plan that is not active is refused before anything is written", func(t *testing.T) {
		svc, store, _, calls := newTestService(t)
		seedRestAPI(store, "api-uuid", "orders", nil)
		seedPlan(store, gold, "gold", models.SubscriptionPlanStatusInactive)

		_, err := svc.Update(UpdateParams{Existing: newSub(nil),
			Request: api.SubscriptionUpdateRequest{SubscriptionPlanId: &gold},
			Fields:  map[string]interface{}{"subscriptionPlanId": gold}, Logger: testLogger()})

		var validationErr *ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, "Subscription plan is not active", validationErr.Message)
		assert.NotContains(t, *calls, "update_subscription")
	})
}

func TestUpdate_Token(t *testing.T) {
	t.Run("rotates the token", func(t *testing.T) {
		svc, store, _, _ := newTestService(t)
		token := "  new-token  "

		_, err := svc.Update(UpdateParams{Existing: &models.Subscription{ID: "sub-1"},
			Request: api.SubscriptionUpdateRequest{SubscriptionToken: &token}, Logger: testLogger()})

		require.NoError(t, err)
		assert.Equal(t, "new-token", store.subs["sub-1"].SubscriptionToken)
	})

	t.Run("an empty token is refused", func(t *testing.T) {
		svc, _, _, _ := newTestService(t)
		blank := "   "

		_, err := svc.Update(UpdateParams{Existing: &models.Subscription{ID: "sub-1"},
			Request: api.SubscriptionUpdateRequest{SubscriptionToken: &blank}, Logger: testLogger()})

		var validationErr *ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, "subscriptionToken must not be empty", validationErr.Message)
	})

	t.Run("a token conflict remains detectable", func(t *testing.T) {
		svc, store, _, _ := newTestService(t)
		store.updateSubErr = fmt.Errorf("%w: subscription token already exists for this API", storage.ErrConflict)
		token := "taken"

		_, err := svc.Update(UpdateParams{Existing: &models.Subscription{ID: "sub-1"},
			Request: api.SubscriptionUpdateRequest{SubscriptionToken: &token}, Logger: testLogger()})

		assert.True(t, storage.IsConflictError(err))
	})
}

// An update that sends nothing is still a write, and leaves the subscription as it was.
func TestUpdate_EmptyRequestChangesNothing(t *testing.T) {
	svc, store, _, _ := newTestService(t)
	store.subs["sub-1"] = &models.Subscription{ID: "sub-1", Status: models.SubscriptionStatusActive}

	result, err := svc.Update(UpdateParams{Existing: store.subs["sub-1"], Logger: testLogger()})
	require.NoError(t, err)

	assert.Equal(t, models.SubscriptionStatusActive, result.Subscription.Status)
}

func TestUpdate_DeletedBeforeTheWrite(t *testing.T) {
	svc, store, _, _ := newTestService(t)
	store.updateSubErr = fmt.Errorf("%w: subscription not found", storage.ErrNotFound)

	_, err := svc.Update(UpdateParams{Existing: &models.Subscription{ID: "sub-1"}, Logger: testLogger()})
	assert.ErrorIs(t, err, ErrSubscriptionNotFound)
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
		assert.Equal(t, OpResolveByID, opErr.Op)
	})

	t.Run("unexpected storage failure on the handle lookup", func(t *testing.T) {
		svc, store, _, _ := newTestService(t)
		store.handleErr = errors.New("database unavailable")

		_, err := svc.ResolveAPIID("orders")

		var opErr *OpError
		require.ErrorAs(t, err, &opErr)
		assert.Equal(t, OpResolveByHandle, opErr.Op)
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

// Subscription plans

func TestCreatePlan_PersistsThenPublishes(t *testing.T) {
	svc, store, hub, calls := newTestService(t)

	count := 500
	expiry := time.Date(2027, 1, 31, 23, 59, 59, 0, time.UTC)
	stopOnQuota := false

	result, err := svc.CreatePlan(CreatePlanParams{
		Request: api.SubscriptionPlanCreateRequest{
			PlanName:           "  gold  ",
			ThrottleLimitCount: &count,
			ThrottleLimitUnit:  planCreateUnit(api.SubscriptionPlanCreateRequestThrottleLimitUnitHour),
			ExpiryTime:         &expiry,
			StopOnQuotaReach:   &stopOnQuota,
			Status:             planCreateStatus(api.SubscriptionPlanCreateRequestStatusINACTIVE),
		},
		CorrelationID: "corr-1",
		Logger:        testLogger(),
	})
	require.NoError(t, err)

	// planName is trimmed, every optional field is carried through, and the
	// caller-supplied status wins over the ACTIVE default.
	assert.Equal(t, "gold", result.Plan.PlanName)
	assert.Equal(t, models.SubscriptionPlanStatusInactive, result.Plan.Status)
	assert.False(t, result.Plan.StopOnQuotaReach)
	assert.Equal(t, &count, result.Plan.ThrottleLimitCount)
	require.NotNil(t, result.Plan.ThrottleLimitUnit)
	assert.Equal(t, "Hour", *result.Plan.ThrottleLimitUnit)
	assert.Equal(t, &expiry, result.Plan.ExpiryTime)
	assert.NotEmpty(t, result.Plan.ID)

	assert.Contains(t, store.plans, result.Plan.ID)

	// The DB write must land before the replica-sync event; a peer that reacts
	// to the event has to be able to read the row.
	assert.Equal(t, []string{"save_plan", "publish_event"}, *calls)
	require.Len(t, hub.events, 1)
	assert.Equal(t, eventhub.EventTypeSubscriptionPlan, hub.events[0].EventType)
	assert.Equal(t, "CREATE", hub.events[0].Action)
	assert.Equal(t, result.Plan.ID, hub.events[0].EntityID)
	assert.Equal(t, "corr-1", hub.events[0].EventID)
}

func TestCreatePlan_DefaultsStatusAndStopOnQuotaReach(t *testing.T) {
	svc, _, _, _ := newTestService(t)

	result, err := svc.CreatePlan(CreatePlanParams{
		Request: api.SubscriptionPlanCreateRequest{PlanName: "silver"},
		Logger:  testLogger(),
	})
	require.NoError(t, err)

	assert.Equal(t, models.SubscriptionPlanStatusActive, result.Plan.Status)
	assert.True(t, result.Plan.StopOnQuotaReach)
	assert.Nil(t, result.Plan.ThrottleLimitCount)
	assert.Nil(t, result.Plan.ThrottleLimitUnit)
}

func TestCreatePlan_ValidationFailuresDoNotTouchStorage(t *testing.T) {
	count := 10
	badCount := 0

	tests := []struct {
		name    string
		request api.SubscriptionPlanCreateRequest
		message string
	}{
		{
			name:    "blank plan name",
			request: api.SubscriptionPlanCreateRequest{PlanName: "   "},
			message: "planName is required",
		},
		{
			name:    "count without unit",
			request: api.SubscriptionPlanCreateRequest{PlanName: "p", ThrottleLimitCount: &count},
			message: "throttleLimitCount and throttleLimitUnit must be provided together",
		},
		{
			name: "unit without count",
			request: api.SubscriptionPlanCreateRequest{
				PlanName:          "p",
				ThrottleLimitUnit: planCreateUnit(api.SubscriptionPlanCreateRequestThrottleLimitUnitDay),
			},
			message: "throttleLimitCount and throttleLimitUnit must be provided together",
		},
		{
			name: "non-positive count",
			request: api.SubscriptionPlanCreateRequest{
				PlanName:           "p",
				ThrottleLimitCount: &badCount,
				ThrottleLimitUnit:  planCreateUnit(api.SubscriptionPlanCreateRequestThrottleLimitUnitDay),
			},
			message: "throttleLimitCount must be positive",
		},
		{
			name: "unknown status",
			request: api.SubscriptionPlanCreateRequest{
				PlanName: "p",
				Status:   planCreateStatus("PENDING"),
			},
			message: "invalid status: PENDING",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _, calls := newTestService(t)

			_, err := svc.CreatePlan(CreatePlanParams{Request: tc.request, Logger: testLogger()})

			var validationErr *ValidationError
			require.ErrorAs(t, err, &validationErr)
			assert.Equal(t, tc.message, validationErr.Message)
			assert.Empty(t, *calls, "a rejected request must not reach storage")
		})
	}
}

func TestCreatePlan_ConflictRemainsDetectable(t *testing.T) {
	svc, store, _, _ := newTestService(t)
	store.saveErr = fmt.Errorf("%w: subscription plan already exists", storage.ErrConflict)

	_, err := svc.CreatePlan(CreatePlanParams{
		Request: api.SubscriptionPlanCreateRequest{PlanName: "gold"},
		Logger:  testLogger(),
	})

	// OpError must unwrap, or the handler could not answer 409.
	require.Error(t, err)
	assert.True(t, storage.IsConflictError(err))

	var opErr *OpError
	require.ErrorAs(t, err, &opErr)
	assert.Equal(t, OpCreate, opErr.Op)
}

func TestUpdatePlan_AppliesOnlySuppliedFields(t *testing.T) {
	svc, store, _, calls := newTestService(t)

	original := &models.SubscriptionPlan{
		ID:               "plan-1",
		PlanName:         "gold",
		StopOnQuotaReach: true,
		Status:           models.SubscriptionPlanStatusActive,
	}
	store.plans["plan-1"] = original

	newName := "  platinum  "
	result, err := svc.UpdatePlan(UpdatePlanParams{
		Existing: original,
		Request: api.SubscriptionPlanUpdateRequest{
			PlanName: &newName,
			Status:   planUpdateStatus(api.SubscriptionPlanUpdateRequestStatusINACTIVE),
		},
		CorrelationID: "corr-2",
		Logger:        testLogger(),
	})
	require.NoError(t, err)

	assert.Equal(t, "platinum", result.Plan.PlanName)
	assert.Equal(t, models.SubscriptionPlanStatusInactive, result.Plan.Status)
	// Untouched fields survive the patch.
	assert.True(t, result.Plan.StopOnQuotaReach)

	assert.Equal(t, "gold", original.PlanName, "the stored plan is changed through a copy")

	assert.Equal(t, []string{"update_plan", "publish_event"}, *calls)
}

func TestUpdatePlan_ClearsAndKeepsFields(t *testing.T) {
	newPlan := func() *models.SubscriptionPlan {
		limit, unit, billing := 100, "Min", "stripe-gold"
		expiry := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
		return &models.SubscriptionPlan{ID: "plan-1", PlanName: "gold", BillingPlan: &billing,
			ThrottleLimitCount: &limit, ThrottleLimitUnit: &unit, ExpiryTime: &expiry,
			Status: models.SubscriptionPlanStatusActive}
	}

	t.Run("explicit nulls clear the fields, through a copy", func(t *testing.T) {
		svc, _, _, _ := newTestService(t)
		existing := newPlan()

		result, err := svc.UpdatePlan(UpdatePlanParams{Existing: existing, Logger: testLogger(),
			Fields: map[string]interface{}{"billingPlan": nil, "expiryTime": nil, "throttleLimitCount": nil, "throttleLimitUnit": nil}})

		require.NoError(t, err)
		assert.Nil(t, result.Plan.BillingPlan)
		assert.Nil(t, result.Plan.ExpiryTime)
		assert.Nil(t, result.Plan.ThrottleLimitCount)
		assert.Nil(t, result.Plan.ThrottleLimitUnit)
		assert.NotNil(t, existing.BillingPlan, "the stored plan is changed through a copy")
	})

	t.Run("omitted fields are kept", func(t *testing.T) {
		svc, _, _, _ := newTestService(t)

		result, err := svc.UpdatePlan(UpdatePlanParams{Existing: newPlan(), Fields: map[string]interface{}{}, Logger: testLogger()})

		require.NoError(t, err)
		assert.Equal(t, "stripe-gold", *result.Plan.BillingPlan)
		assert.Equal(t, 100, *result.Plan.ThrottleLimitCount)
		assert.NotNil(t, result.Plan.ExpiryTime)
	})

	t.Run("clearing half the throttle limit is refused", func(t *testing.T) {
		svc, _, _, _ := newTestService(t)

		_, err := svc.UpdatePlan(UpdatePlanParams{Existing: newPlan(),
			Fields: map[string]interface{}{"throttleLimitCount": nil}, Logger: testLogger()})

		var validationErr *ValidationError
		require.ErrorAs(t, err, &validationErr)
		assert.Equal(t, "throttleLimitCount and throttleLimitUnit must be set or cleared together", validationErr.Message)
	})

	t.Run("a rename conflict remains detectable", func(t *testing.T) {
		svc, store, _, _ := newTestService(t)
		store.updateErr = fmt.Errorf("%w: subscription plan name already exists", storage.ErrConflict)
		name := "silver"

		_, err := svc.UpdatePlan(UpdatePlanParams{Existing: newPlan(),
			Request: api.SubscriptionPlanUpdateRequest{PlanName: &name}, Logger: testLogger()})

		assert.True(t, storage.IsConflictError(err))
	})
}

func TestUpdatePlan_RejectsBlankPlanName(t *testing.T) {
	svc, store, _, _ := newTestService(t)
	store.plans["plan-1"] = &models.SubscriptionPlan{ID: "plan-1", PlanName: "gold"}

	blank := "   "
	_, err := svc.UpdatePlan(UpdatePlanParams{
		Existing: store.plans["plan-1"],
		Request:  api.SubscriptionPlanUpdateRequest{PlanName: &blank},
		Logger:   testLogger(),
	})

	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, "planName cannot be empty", validationErr.Message)
}

func TestUpdatePlan_DeletedBeforeTheWrite(t *testing.T) {
	svc, store, _, _ := newTestService(t)
	store.updateErr = fmt.Errorf("%w: subscription plan not found", storage.ErrNotFound)

	_, err := svc.UpdatePlan(UpdatePlanParams{Existing: &models.SubscriptionPlan{ID: "plan-1", PlanName: "gold"}, Logger: testLogger()})
	assert.ErrorIs(t, err, ErrPlanNotFound)
}

func TestGetPlan_NotFound(t *testing.T) {
	t.Run("nil row", func(t *testing.T) {
		svc, _, _, _ := newTestService(t)

		_, err := svc.GetPlan("missing")
		assert.ErrorIs(t, err, ErrPlanNotFound)
	})

	t.Run("storage not-found error", func(t *testing.T) {
		svc, store, _, _ := newTestService(t)
		store.getErr = fmt.Errorf("%w: subscription plan", storage.ErrNotFound)

		_, err := svc.GetPlan("missing")
		assert.ErrorIs(t, err, ErrPlanNotFound)
	})

	t.Run("unexpected storage error is a load failure", func(t *testing.T) {
		svc, store, _, _ := newTestService(t)
		store.getErr = errors.New("database unavailable")

		_, err := svc.GetPlan("any")

		var opErr *OpError
		require.ErrorAs(t, err, &opErr)
		assert.Equal(t, OpLoad, opErr.Op)
	})
}

func TestListPlans(t *testing.T) {
	svc, store, _, _ := newTestService(t)
	store.plans["a"] = &models.SubscriptionPlan{ID: "a", PlanName: "gold"}
	store.plans["b"] = &models.SubscriptionPlan{ID: "b", PlanName: "silver"}

	result, err := svc.ListPlans()
	require.NoError(t, err)
	assert.Len(t, result.Plans, 2)
}

func TestListPlans_StorageFailure(t *testing.T) {
	svc, store, _, _ := newTestService(t)
	store.listErr = errors.New("database unavailable")

	_, err := svc.ListPlans()

	var opErr *OpError
	require.ErrorAs(t, err, &opErr)
	assert.Equal(t, OpList, opErr.Op)
}

func TestDeletePlan(t *testing.T) {
	t.Run("removes the plan and publishes", func(t *testing.T) {
		svc, store, hub, calls := newTestService(t)
		store.plans["plan-1"] = &models.SubscriptionPlan{ID: "plan-1"}

		require.NoError(t, svc.DeletePlan("plan-1", "corr-3", testLogger()))

		assert.NotContains(t, store.plans, "plan-1")
		assert.Equal(t, []string{"delete_plan", "publish_event"}, *calls)
		require.Len(t, hub.events, 1)
		assert.Equal(t, "DELETE", hub.events[0].Action)
	})

	t.Run("missing plan maps to ErrPlanNotFound", func(t *testing.T) {
		svc, store, _, _ := newTestService(t)
		store.deleteErr = fmt.Errorf("%w: subscription plan not found", storage.ErrNotFound)

		err := svc.DeletePlan("missing", "", testLogger())
		assert.ErrorIs(t, err, ErrPlanNotFound)
	})
}

func TestParseSubscriptionPlanStatus(t *testing.T) {
	active, err := parseSubscriptionPlanStatus("ACTIVE")
	require.NoError(t, err)
	assert.Equal(t, models.SubscriptionPlanStatusActive, active)

	inactive, err := parseSubscriptionPlanStatus("INACTIVE")
	require.NoError(t, err)
	assert.Equal(t, models.SubscriptionPlanStatusInactive, inactive)

	// Case matters: the enum is upper-case and the storage column is not
	// normalised, so "active" must be rejected rather than silently accepted.
	_, err = parseSubscriptionPlanStatus("active")
	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, "invalid status: active", validationErr.Message)
}

func TestValidateThrottleLimits(t *testing.T) {
	positive := 100
	zero := 0
	negative := -1
	day := "Day"
	empty := ""
	bogus := "Fortnight"

	tests := []struct {
		name    string
		count   *int
		unit    *string
		wantErr string
	}{
		{name: "neither supplied", count: nil, unit: nil},
		{name: "both supplied", count: &positive, unit: &day},
		{name: "empty unit counts as absent", count: nil, unit: &empty},
		{
			name:    "count without unit",
			count:   &positive,
			wantErr: "throttleLimitCount and throttleLimitUnit must be provided together",
		},
		{
			name:    "unit without count",
			unit:    &day,
			wantErr: "throttleLimitCount and throttleLimitUnit must be provided together",
		},
		{
			name:    "empty unit with a count",
			count:   &positive,
			unit:    &empty,
			wantErr: "throttleLimitCount and throttleLimitUnit must be provided together",
		},
		{name: "zero count", count: &zero, unit: &day, wantErr: "throttleLimitCount must be positive"},
		{name: "negative count", count: &negative, unit: &day, wantErr: "throttleLimitCount must be positive"},
		{
			name:    "unknown unit",
			count:   &positive,
			unit:    &bogus,
			wantErr: "throttleLimitUnit must be one of: Day, Hour, Min, Month",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateThrottleLimits(tc.count, tc.unit)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Equal(t, tc.wantErr, err.Error())
		})
	}
}

func TestValidateThrottleLimits_AcceptsEveryDocumentedUnit(t *testing.T) {
	count := 1
	for _, unit := range []string{"Day", "Hour", "Min", "Month"} {
		t.Run(unit, func(t *testing.T) {
			assert.NoError(t, validateThrottleLimits(&count, &unit))
		})
	}
}
