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

// fakeStore is a storage.Storage that implements only the subscription,
// subscription-plan and stored-config methods these tests exercise. storage.Storage is embedded as a nil interface,
// so any other database call panics rather than silently returning a zero
// value — an unexpected read fails loudly instead of passing.
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
		ID: "plan-1",
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

	assert.Equal(t, []string{"get_plan", "update_plan", "publish_event"}, *calls)
}

func TestUpdatePlan_NotFound(t *testing.T) {
	svc, _, _, calls := newTestService(t)

	_, err := svc.UpdatePlan(UpdatePlanParams{ID: "missing", Logger: testLogger()})

	assert.ErrorIs(t, err, ErrPlanNotFound)
	assert.Equal(t, []string{"get_plan"}, *calls, "a missing plan must not be written")
}

func TestUpdatePlan_RejectsBlankPlanName(t *testing.T) {
	svc, store, _, _ := newTestService(t)
	store.plans["plan-1"] = &models.SubscriptionPlan{ID: "plan-1", PlanName: "gold"}

	blank := "   "
	_, err := svc.UpdatePlan(UpdatePlanParams{
		ID:      "plan-1",
		Request: api.SubscriptionPlanUpdateRequest{PlanName: &blank},
		Logger:  testLogger(),
	})

	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, "planName cannot be empty", validationErr.Message)
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
		unit := unit
		t.Run(unit, func(t *testing.T) {
			assert.NoError(t, validateThrottleLimits(&count, &unit))
		})
	}
}
