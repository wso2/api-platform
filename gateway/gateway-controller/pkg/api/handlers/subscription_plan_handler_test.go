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

package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/subscription"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

func TestCreateSubscriptionPlanStoresEveryField(t *testing.T) {
	srv := newSubsServer(t)

	rec := subsDo(t, http.MethodPost, "/subscription-plans", `{
		"planName": "  Gold  ", "billingPlan": "bp-gold", "stopOnQuotaReach": false,
		"throttleLimitCount": 100, "throttleLimitUnit": "Min",
		"expiryTime": "2030-01-31T23:59:59Z", "status": "INACTIVE"}`, "", srv.s.CreateSubscriptionPlan)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	got := subsDecode[api.SubscriptionPlanResponse](t, rec)
	require.NotNil(t, got.Id)
	assert.Equal(t, "Gold", subsVal(t, got.PlanName), "the name is trimmed")
	assert.Equal(t, "bp-gold", subsVal(t, got.BillingPlan))
	assert.False(t, subsVal(t, got.StopOnQuotaReach))
	assert.Equal(t, 100, subsVal(t, got.ThrottleLimitCount))
	assert.Equal(t, "Min", subsVal(t, got.ThrottleLimitUnit))
	assert.True(t, time.Date(2030, 1, 31, 23, 59, 59, 0, time.UTC).Equal(subsVal(t, got.ExpiryTime)))
	assert.Equal(t, api.SubscriptionPlanResponseStatus("INACTIVE"), subsVal(t, got.Status))

	stored := srv.db.subscriptionPlans[subsVal(t, got.Id)]
	require.NotNil(t, stored)
	assert.Equal(t, "Gold", stored.PlanName)
	assert.Equal(t, models.SubscriptionPlanStatusInactive, stored.Status)
}

// Omitted fields take the service defaults: the quota stops serving, and the
// plan is active.
func TestCreateSubscriptionPlanDefaults(t *testing.T) {
	srv := newSubsServer(t)

	rec := subsDo(t, http.MethodPost, "/subscription-plans", "planName: Basic\n", "application/yaml",
		srv.s.CreateSubscriptionPlan)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	got := subsDecode[api.SubscriptionPlanResponse](t, rec)
	assert.True(t, subsVal(t, got.StopOnQuotaReach))
	assert.Equal(t, api.SubscriptionPlanResponseStatus("ACTIVE"), subsVal(t, got.Status))
	for _, field := range []string{"billingPlan", "throttleLimitCount", "throttleLimitUnit", "expiryTime"} {
		assert.NotContainsf(t, rec.Body.String(), `"`+field+`"`, "unset %s is omitted", field)
	}
}

func TestCreateSubscriptionPlanRejections(t *testing.T) {
	for _, tt := range []struct {
		name    string
		body    string
		message string
	}{
		{"malformed body", `{"planName": `, "Invalid request body"},
		{"missing name", `{"planName": "   "}`, "planName is required"},
		{"count without unit", `{"planName": "p", "throttleLimitCount": 10}`,
			"throttleLimitCount and throttleLimitUnit must be provided together"},
		{"unit without count", `{"planName": "p", "throttleLimitUnit": "Day"}`,
			"throttleLimitCount and throttleLimitUnit must be provided together"},
		{"non-positive count", `{"planName": "p", "throttleLimitCount": 0, "throttleLimitUnit": "Day"}`,
			"throttleLimitCount must be positive"},
		{"unknown unit", `{"planName": "p", "throttleLimitCount": 5, "throttleLimitUnit": "Week"}`,
			"throttleLimitUnit must be one of: Day, Hour, Min, Month"},
		{"invalid status", `{"planName": "p", "status": "RETIRED"}`, "invalid status: RETIRED"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv := newSubsServer(t)

			rec := subsDo(t, http.MethodPost, "/subscription-plans", tt.body, "", srv.s.CreateSubscriptionPlan)

			subsError(t, rec, http.StatusBadRequest, tt.message)
			assert.Empty(t, srv.db.subscriptionPlans)
		})
	}
}

func TestCreateSubscriptionPlanStorageFailures(t *testing.T) {
	for _, tt := range []struct {
		name    string
		inject  func(db *MockStorage)
		status  int
		message string
	}{
		{"name taken", func(db *MockStorage) { db.saveErr = fmt.Errorf("%w: plan name", storage.ErrConflict) },
			http.StatusConflict, "Subscription plan already exists"},
		{"save fails", func(db *MockStorage) { db.saveErr = errors.New("disk full") },
			http.StatusInternalServerError, "Failed to create subscription plan"},
		{"database unavailable", func(db *MockStorage) { db.unavailable = true },
			http.StatusInternalServerError, "Failed to create subscription plan"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv := newSubsServer(t)
			tt.inject(srv.db)

			rec := subsDo(t, http.MethodPost, "/subscription-plans", `{"planName": "Gold"}`, "",
				srv.s.CreateSubscriptionPlan)

			subsError(t, rec, tt.status, tt.message)
		})
	}
}

func TestListSubscriptionPlans(t *testing.T) {
	srv := newSubsServer(t)
	list := func() *httptest.ResponseRecorder {
		return subsDo(t, http.MethodGet, "/subscription-plans", "", "", srv.s.ListSubscriptionPlans)
	}

	rec := list()
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"count": 0, "subscriptionPlans": []}`, rec.Body.String(),
		"an empty result is an empty array, not null")

	subsSeedPlan(t, srv.db, "plan-1", "Gold", models.SubscriptionPlanStatusActive)
	subsSeedPlan(t, srv.db, "plan-2", "Silver", models.SubscriptionPlanStatusInactive)
	got := subsDecode[api.SubscriptionPlanListResponse](t, list())
	assert.Equal(t, 2, subsVal(t, got.Count))
	var names []string
	for _, p := range subsVal(t, got.SubscriptionPlans) {
		names = append(names, subsVal(t, p.PlanName))
	}
	assert.ElementsMatch(t, []string{"Gold", "Silver"}, names)

	srv.db.getErr = errors.New("database is locked")
	subsError(t, list(), http.StatusInternalServerError, "Failed to list subscription plans")
}

func TestGetSubscriptionPlan(t *testing.T) {
	srv := newSubsServer(t)
	subsSeedPlan(t, srv.db, "plan-1", "Gold", models.SubscriptionPlanStatusActive)
	get := func(id string) *httptest.ResponseRecorder {
		return subsDo(t, http.MethodGet, "/subscription-plans/"+id, "", "",
			func(w http.ResponseWriter, r *http.Request) { srv.s.GetSubscriptionPlan(w, r, id) })
	}

	rec := get("plan-1")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Gold", subsVal(t, subsDecode[api.SubscriptionPlanResponse](t, rec).PlanName))

	subsError(t, get("no-such-plan"), http.StatusNotFound, "Subscription plan not found")

	srv.db.getErr = errors.New("database is locked")
	subsError(t, get("plan-1"), http.StatusInternalServerError, "Failed to get subscription plan")
}

// A partial update changes only the fields it carries.
func TestUpdateSubscriptionPlanAppliesOnlyTheGivenFields(t *testing.T) {
	srv := newSubsServer(t)
	billing := "bp-1"
	require.NoError(t, srv.db.SaveSubscriptionPlan(&models.SubscriptionPlan{
		ID: "plan-1", PlanName: "Gold", BillingPlan: &billing, StopOnQuotaReach: true,
		Status: models.SubscriptionPlanStatusActive,
	}))

	rec := subsDo(t, http.MethodPut, "/subscription-plans/plan-1",
		`{"planName": "Platinum", "throttleLimitCount": 50, "throttleLimitUnit": "Hour", "status": "INACTIVE"}`, "",
		func(w http.ResponseWriter, r *http.Request) { srv.s.UpdateSubscriptionPlan(w, r, "plan-1") })

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := subsDecode[api.SubscriptionPlanResponse](t, rec)
	assert.Equal(t, "Platinum", subsVal(t, got.PlanName))
	assert.Equal(t, 50, subsVal(t, got.ThrottleLimitCount))
	assert.Equal(t, api.SubscriptionPlanResponseStatus("INACTIVE"), subsVal(t, got.Status))
	assert.Equal(t, "bp-1", subsVal(t, got.BillingPlan), "a field the update did not carry is unchanged")
	assert.True(t, subsVal(t, got.StopOnQuotaReach))
	assert.Equal(t, "Platinum", srv.db.subscriptionPlans["plan-1"].PlanName)
}

func TestUpdateSubscriptionPlanFailures(t *testing.T) {
	for _, tt := range []struct {
		name    string
		id      string
		body    string
		inject  func(db *MockStorage)
		status  int
		message string
	}{
		// Existence is checked before the body is read.
		{"unknown id with a malformed body", "no-such-plan", `{"planName": `, nil,
			http.StatusNotFound, "Subscription plan not found"},
		{"malformed body", "plan-1", `{"planName": `, nil, http.StatusBadRequest, "Invalid request body"},
		{"blank name", "plan-1", `{"planName": "  "}`, nil, http.StatusBadRequest, "planName cannot be empty"},
		{"count without unit", "plan-1", `{"throttleLimitCount": 5}`, nil, http.StatusBadRequest,
			"throttleLimitCount and throttleLimitUnit must be provided together"},
		{"invalid status", "plan-1", `{"status": "RETIRED"}`, nil, http.StatusBadRequest, "invalid status: RETIRED"},
		{"existence check fails", "plan-1", `{"planName": "X"}`,
			func(db *MockStorage) { db.getErr = errors.New("database is locked") },
			http.StatusInternalServerError, "Failed to get subscription plan"},
		{"write fails", "plan-1", `{"planName": "X"}`,
			func(db *MockStorage) { db.updateErr = errors.New("database is locked") },
			http.StatusInternalServerError, "Failed to update subscription plan"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv := newSubsServer(t)
			subsSeedPlan(t, srv.db, "plan-1", "Gold", models.SubscriptionPlanStatusActive)
			if tt.inject != nil {
				tt.inject(srv.db)
			}

			rec := subsDo(t, http.MethodPut, "/subscription-plans/"+tt.id, tt.body, "",
				func(w http.ResponseWriter, r *http.Request) { srv.s.UpdateSubscriptionPlan(w, r, tt.id) })

			subsError(t, rec, tt.status, tt.message)
			// See TestUpdateSubscriptionFailures for why a failed write is not checked here.
			if srv.db.updateErr == nil {
				assert.Equal(t, "Gold", srv.db.subscriptionPlans["plan-1"].PlanName, "a refused update changes nothing")
			}
		})
	}
}

func TestDeleteSubscriptionPlan(t *testing.T) {
	srv := newSubsServer(t)
	subsSeedPlan(t, srv.db, "plan-1", "Gold", models.SubscriptionPlanStatusActive)
	del := func(id string) *httptest.ResponseRecorder {
		return subsDo(t, http.MethodDelete, "/subscription-plans/"+id, "", "",
			func(w http.ResponseWriter, r *http.Request) { srv.s.DeleteSubscriptionPlan(w, r, id) })
	}

	rec := del("plan-1")
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
	assert.NotContains(t, srv.db.subscriptionPlans, "plan-1")

	subsError(t, del("plan-1"), http.StatusNotFound, "Subscription plan not found")

	subsSeedPlan(t, srv.db, "plan-2", "Silver", models.SubscriptionPlanStatusActive)
	srv.db.deleteErr = errors.New("database is locked")
	subsError(t, del("plan-2"), http.StatusInternalServerError, "Failed to delete subscription plan")
	assert.Contains(t, srv.db.subscriptionPlans, "plan-2")
}

func TestPlanUpdateErrorMapperRemainingBranches(t *testing.T) {
	rec := httptest.NewRecorder()
	mapPlanUpdateError(rec, toolsDiscard, subscription.ErrPlanNotFound)
	subsError(t, rec, http.StatusNotFound, "Subscription plan not found")

	rec = httptest.NewRecorder()
	mapPlanUpdateError(rec, toolsDiscard, &subscription.OpError{Op: subscription.OpLoad, Cause: errors.New("x")})
	subsError(t, rec, http.StatusInternalServerError, "Failed to get subscription plan")
}

// Empty optional values are omitted rather than rendered as "" or null.
func TestSubscriptionPlanToResponseOmitsEmptyOptionalFields(t *testing.T) {
	empty := ""
	resp := subscriptionPlanToResponse(&models.SubscriptionPlan{
		ID: "plan-1", PlanName: "Gold", BillingPlan: &empty, ThrottleLimitUnit: &empty,
	})
	raw, err := json.Marshal(resp)
	require.NoError(t, err)

	for _, field := range []string{"billingPlan", "throttleLimitCount", "throttleLimitUnit", "expiryTime", "status"} {
		assert.NotContainsf(t, string(raw), `"`+field+`"`, "%s must be omitted", field)
	}
	assert.Equal(t, "plan-1", subsVal(t, resp.Id))
	assert.False(t, subsVal(t, resp.StopOnQuotaReach), "the flag is always present, even when false")
}
