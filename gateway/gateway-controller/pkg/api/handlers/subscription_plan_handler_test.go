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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// --- helpers ---

func newPlanTestServer(t *testing.T) (*APIServer, *mockEventHub) {
	t.Helper()
	server := createTestAPIServer()
	hub := &mockEventHub{}
	attachTestEventHub(server, hub, "test-gateway")
	return server, hub
}

var planTestExpiry = time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

// seedFullSubscriptionPlan stores a plan with every optional field set.
func seedFullSubscriptionPlan(t *testing.T, server *APIServer, id, name string) {
	t.Helper()
	expiry := planTestExpiry
	require.NoError(t, server.db.SaveSubscriptionPlan(&models.SubscriptionPlan{
		ID:                 id,
		PlanName:           name,
		BillingPlan:        ptr("COMMERCIAL"),
		StopOnQuotaReach:   true,
		ThrottleLimitCount: ptr(100),
		ThrottleLimitUnit:  ptr("Min"),
		ExpiryTime:         &expiry,
		Status:             models.SubscriptionPlanStatusActive,
	}))
}

func putSubscriptionPlan(t *testing.T, server *APIServer, id, contentType, body string) (int, []byte) {
	t.Helper()
	w, r := createTestContextWithHeader("PUT", "/subscription-plans/"+id, []byte(body), map[string]string{"Content-Type": contentType})
	server.UpdateSubscriptionPlan(w, r, id)
	return w.Code, w.Body.Bytes()
}

func deleteSubscriptionPlanViaHandler(t *testing.T, server *APIServer, id string) (int, []byte) {
	t.Helper()
	w, r := createTestContext("DELETE", "/subscription-plans/"+id, nil)
	server.DeleteSubscriptionPlan(w, r, id)
	return w.Code, w.Body.Bytes()
}

func storedPlan(t *testing.T, server *APIServer, id string) *models.SubscriptionPlan {
	t.Helper()
	plan, err := server.db.GetSubscriptionPlanByID(id, "")
	require.NoError(t, err)
	return plan
}

// assertFullPlanUnchanged checks that a plan seeded by seedFullSubscriptionPlan was not modified.
func assertFullPlanUnchanged(t *testing.T, server *APIServer, id, name string) {
	t.Helper()
	plan := storedPlan(t, server, id)
	assert.Equal(t, name, plan.PlanName)
	require.NotNil(t, plan.BillingPlan)
	assert.Equal(t, "COMMERCIAL", *plan.BillingPlan)
	assert.True(t, plan.StopOnQuotaReach)
	require.NotNil(t, plan.ThrottleLimitCount)
	assert.Equal(t, 100, *plan.ThrottleLimitCount)
	require.NotNil(t, plan.ThrottleLimitUnit)
	assert.Equal(t, "Min", *plan.ThrottleLimitUnit)
	require.NotNil(t, plan.ExpiryTime)
	assert.True(t, planTestExpiry.Equal(*plan.ExpiryTime))
	assert.Equal(t, models.SubscriptionPlanStatusActive, plan.Status)
}

// --- create ---

func TestCreateSubscriptionPlan_DuplicateNameConflicts(t *testing.T) {
	server, _ := newPlanTestServer(t)
	seedFullSubscriptionPlan(t, server, "plan-gold", "Gold")

	w, r := createTestContextWithHeader("POST", "/subscription-plans", []byte(`{"planName":"Gold"}`),
		map[string]string{"Content-Type": "application/json"})
	server.CreateSubscriptionPlan(w, r)
	require.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, "Subscription plan already exists", errorMessage(t, w.Body.Bytes()))
}

func TestCreateSubscriptionPlan_BodyTooLarge(t *testing.T) {
	server, _ := newPlanTestServer(t)

	body := fmt.Sprintf(`{"planName":%q}`, strings.Repeat("x", maxSubscriptionRequestBodyBytes))
	w, r := createTestContextWithHeader("POST", "/subscription-plans", []byte(body),
		map[string]string{"Content-Type": "application/json"})
	server.CreateSubscriptionPlan(w, r)
	require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Equal(t, "Request body too large", errorMessage(t, w.Body.Bytes()))
}

// --- update: rename ---

func TestUpdateSubscriptionPlan_RenameToTakenNameConflicts(t *testing.T) {
	server, _ := newPlanTestServer(t)
	seedFullSubscriptionPlan(t, server, "plan-gold", "Gold")
	seedFullSubscriptionPlan(t, server, "plan-silver", "Silver")

	code, body := putSubscriptionPlan(t, server, "plan-silver", "application/json", `{"planName":"Gold","billingPlan":null}`)
	require.Equal(t, http.StatusConflict, code)
	assert.Equal(t, "A subscription plan with this name already exists", errorMessage(t, body))
	assertFullPlanUnchanged(t, server, "plan-silver", "Silver")
}

func TestUpdateSubscriptionPlan_RenameSucceeds(t *testing.T) {
	tests := []struct {
		name     string
		planName string
		want     string
	}{
		{"new name", "Platinum", "Platinum"},
		{"same name", "Gold", "Gold"},
		{"name is trimmed", "  Diamond  ", "Diamond"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, _ := newPlanTestServer(t)
			seedFullSubscriptionPlan(t, server, "plan-gold", "Gold")

			code, body := putSubscriptionPlan(t, server, "plan-gold", "application/json", fmt.Sprintf(`{"planName":%q}`, tt.planName))
			require.Equal(t, http.StatusOK, code, string(body))
			var resp api.SubscriptionPlanResponse
			require.NoError(t, json.Unmarshal(body, &resp))
			assert.Equal(t, tt.want, *resp.PlanName)
			assert.Equal(t, tt.want, storedPlan(t, server, "plan-gold").PlanName)
		})
	}
}

// --- update: clearing optional fields ---

func TestUpdateSubscriptionPlan_NullClearsOptionalFields(t *testing.T) {
	server, _ := newPlanTestServer(t)
	seedFullSubscriptionPlan(t, server, "plan-gold", "Gold")

	code, body := putSubscriptionPlan(t, server, "plan-gold", "application/json",
		`{"billingPlan":null,"throttleLimitCount":null,"throttleLimitUnit":null,"expiryTime":null}`)
	require.Equal(t, http.StatusOK, code, string(body))

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &raw))
	for _, field := range []string{"billingPlan", "throttleLimitCount", "throttleLimitUnit", "expiryTime"} {
		assert.NotContains(t, raw, field)
	}

	plan := storedPlan(t, server, "plan-gold")
	assert.Nil(t, plan.BillingPlan)
	assert.Nil(t, plan.ThrottleLimitCount)
	assert.Nil(t, plan.ThrottleLimitUnit)
	assert.Nil(t, plan.ExpiryTime)
	// Fields that weren't sent stay as they were.
	assert.Equal(t, "Gold", plan.PlanName)
	assert.True(t, plan.StopOnQuotaReach)
	assert.Equal(t, models.SubscriptionPlanStatusActive, plan.Status)
}

func TestUpdateSubscriptionPlan_EmptyStringsClear(t *testing.T) {
	server, _ := newPlanTestServer(t)
	seedFullSubscriptionPlan(t, server, "plan-gold", "Gold")

	code, body := putSubscriptionPlan(t, server, "plan-gold", "application/json",
		`{"billingPlan":"  ","throttleLimitCount":null,"throttleLimitUnit":""}`)
	require.Equal(t, http.StatusOK, code, string(body))

	plan := storedPlan(t, server, "plan-gold")
	assert.Nil(t, plan.BillingPlan)
	assert.Nil(t, plan.ThrottleLimitCount)
	assert.Nil(t, plan.ThrottleLimitUnit)
	require.NotNil(t, plan.ExpiryTime)
}

func TestUpdateSubscriptionPlan_YAMLNullClears(t *testing.T) {
	server, _ := newPlanTestServer(t)
	seedFullSubscriptionPlan(t, server, "plan-gold", "Gold")

	code, body := putSubscriptionPlan(t, server, "plan-gold", "application/yaml",
		"throttleLimitCount: null\nthrottleLimitUnit: null\nexpiryTime: null\n")
	require.Equal(t, http.StatusOK, code, string(body))

	plan := storedPlan(t, server, "plan-gold")
	assert.Nil(t, plan.ThrottleLimitCount)
	assert.Nil(t, plan.ThrottleLimitUnit)
	assert.Nil(t, plan.ExpiryTime)
	require.NotNil(t, plan.BillingPlan)
	assert.Equal(t, "COMMERCIAL", *plan.BillingPlan)
}

func TestUpdateSubscriptionPlan_OmittedFieldsUnchanged(t *testing.T) {
	for _, body := range []string{`{}`, `{"status":"ACTIVE"}`} {
		t.Run(body, func(t *testing.T) {
			server, _ := newPlanTestServer(t)
			seedFullSubscriptionPlan(t, server, "plan-gold", "Gold")

			code, respBody := putSubscriptionPlan(t, server, "plan-gold", "application/json", body)
			require.Equal(t, http.StatusOK, code, string(respBody))
			assertFullPlanUnchanged(t, server, "plan-gold", "Gold")
		})
	}
}

// --- update: setting fields ---

func TestUpdateSubscriptionPlan_SetsAllFields(t *testing.T) {
	server, _ := newPlanTestServer(t)
	seedNamedSubscriptionPlan(t, server, "plan-basic", "Basic", models.SubscriptionPlanStatusActive)

	code, body := putSubscriptionPlan(t, server, "plan-basic", "application/json", `{
		"planName":"Premium","billingPlan":"FREE","stopOnQuotaReach":false,
		"throttleLimitCount":5,"throttleLimitUnit":"Hour",
		"expiryTime":"2028-06-30T12:00:00Z","status":"INACTIVE"}`)
	require.Equal(t, http.StatusOK, code, string(body))

	var resp api.SubscriptionPlanResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "Premium", *resp.PlanName)
	assert.Equal(t, "FREE", *resp.BillingPlan)
	assert.False(t, *resp.StopOnQuotaReach)
	assert.Equal(t, 5, *resp.ThrottleLimitCount)
	assert.Equal(t, "Hour", *resp.ThrottleLimitUnit)
	assert.True(t, time.Date(2028, 6, 30, 12, 0, 0, 0, time.UTC).Equal(*resp.ExpiryTime))
	assert.Equal(t, api.SubscriptionPlanResponseStatus("INACTIVE"), *resp.Status)

	plan := storedPlan(t, server, "plan-basic")
	assert.Equal(t, "Premium", plan.PlanName)
	assert.Equal(t, 5, *plan.ThrottleLimitCount)
	assert.Equal(t, "Hour", *plan.ThrottleLimitUnit)
	assert.Equal(t, models.SubscriptionPlanStatusInactive, plan.Status)
}

func TestUpdateSubscriptionPlan_AddsLimitToUnlimitedPlan(t *testing.T) {
	server, _ := newPlanTestServer(t)
	seedNamedSubscriptionPlan(t, server, "plan-free", "Free", models.SubscriptionPlanStatusActive)

	code, body := putSubscriptionPlan(t, server, "plan-free", "application/json", `{"throttleLimitCount":10,"throttleLimitUnit":"Day"}`)
	require.Equal(t, http.StatusOK, code, string(body))
	plan := storedPlan(t, server, "plan-free")
	require.NotNil(t, plan.ThrottleLimitCount)
	assert.Equal(t, 10, *plan.ThrottleLimitCount)
	assert.Equal(t, "Day", *plan.ThrottleLimitUnit)
}

// --- update: validation ---

func TestUpdateSubscriptionPlan_ThrottleLimitValidation(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantMsg string
	}{
		{"count without unit", `{"throttleLimitCount":10}`, "throttleLimitCount and throttleLimitUnit must be provided together"},
		{"unit without count", `{"throttleLimitUnit":"Min"}`, "throttleLimitCount and throttleLimitUnit must be provided together"},
		{"count cleared alone", `{"throttleLimitCount":null}`, "throttleLimitCount and throttleLimitUnit must be set or cleared together"},
		{"unit cleared alone", `{"throttleLimitUnit":null}`, "throttleLimitCount and throttleLimitUnit must be set or cleared together"},
		{"count cleared, unit set", `{"throttleLimitCount":null,"throttleLimitUnit":"Min"}`, "throttleLimitCount and throttleLimitUnit must be set or cleared together"},
		{"count set, unit cleared", `{"throttleLimitCount":10,"throttleLimitUnit":null}`, "throttleLimitCount and throttleLimitUnit must be set or cleared together"},
		{"zero count", `{"throttleLimitCount":0,"throttleLimitUnit":"Min"}`, "throttleLimitCount must be positive"},
		{"negative count", `{"throttleLimitCount":-1,"throttleLimitUnit":"Min"}`, "throttleLimitCount must be positive"},
		{"unknown unit", `{"throttleLimitCount":10,"throttleLimitUnit":"Week"}`, "throttleLimitUnit must be one of: Day, Hour, Min, Month"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, _ := newPlanTestServer(t)
			seedFullSubscriptionPlan(t, server, "plan-gold", "Gold")

			code, body := putSubscriptionPlan(t, server, "plan-gold", "application/json", tt.body)
			require.Equal(t, http.StatusBadRequest, code)
			assert.Equal(t, tt.wantMsg, errorMessage(t, body))
			assertFullPlanUnchanged(t, server, "plan-gold", "Gold")
		})
	}
}

func TestUpdateSubscriptionPlan_RequestValidation(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCode int
		wantMsg  string
	}{
		{"malformed JSON", `{"planName":`, http.StatusBadRequest, "Invalid request body"},
		{"wrong type", `{"throttleLimitCount":"ten"}`, http.StatusBadRequest, "Invalid request body"},
		{"empty planName", `{"planName":""}`, http.StatusBadRequest, "planName cannot be empty"},
		{"blank planName", `{"planName":"   "}`, http.StatusBadRequest, "planName cannot be empty"},
		{"invalid status", `{"status":"PAUSED"}`, http.StatusBadRequest, "invalid status: PAUSED"},
		// A later invalid field rejects the whole request, including the valid changes before it.
		{"valid fields then invalid status", `{"planName":"Renamed","billingPlan":null,"throttleLimitCount":null,"throttleLimitUnit":null,"status":"PAUSED"}`,
			http.StatusBadRequest, "invalid status: PAUSED"},
		{"body too large", fmt.Sprintf(`{"billingPlan":%q}`, strings.Repeat("x", maxSubscriptionRequestBodyBytes)),
			http.StatusRequestEntityTooLarge, "Request body too large"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, _ := newPlanTestServer(t)
			seedFullSubscriptionPlan(t, server, "plan-gold", "Gold")

			code, body := putSubscriptionPlan(t, server, "plan-gold", "application/json", tt.body)
			require.Equal(t, tt.wantCode, code)
			assert.Equal(t, tt.wantMsg, errorMessage(t, body))
			assertFullPlanUnchanged(t, server, "plan-gold", "Gold")
		})
	}
}

func TestUpdateSubscriptionPlan_StorageErrors(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(m *MockStorage)
		wantCode int
		wantMsg  string
	}{
		{"lookup fails", func(m *MockStorage) { m.getErr = errors.New("database unavailable") },
			http.StatusInternalServerError, "Failed to get subscription plan"},
		{"deleted before the write", func(m *MockStorage) { m.updateErr = storage.ErrNotFound },
			http.StatusNotFound, "Subscription plan not found"},
		{"write fails", func(m *MockStorage) { m.updateErr = errors.New("database unavailable") },
			http.StatusInternalServerError, "Failed to update subscription plan"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, _ := newPlanTestServer(t)
			seedFullSubscriptionPlan(t, server, "plan-gold", "Gold")

			tt.setup(server.db.(*MockStorage))
			code, body := putSubscriptionPlan(t, server, "plan-gold", "application/json", `{"status":"INACTIVE"}`)
			require.Equal(t, tt.wantCode, code)
			assert.Equal(t, tt.wantMsg, errorMessage(t, body))
		})
	}
}

func TestUpdateSubscriptionPlan_NotFound(t *testing.T) {
	server, _ := newPlanTestServer(t)

	code, body := putSubscriptionPlan(t, server, "missing", "application/json", `{"status":"INACTIVE"}`)
	require.Equal(t, http.StatusNotFound, code)
	assert.Equal(t, "Subscription plan not found", errorMessage(t, body))
}

func TestUpdateSubscriptionPlan_PublishesUpdateEvent(t *testing.T) {
	server, hub := newPlanTestServer(t)
	seedFullSubscriptionPlan(t, server, "plan-gold", "Gold")

	w, r := createTestContextWithHeader("PUT", "/subscription-plans/plan-gold",
		[]byte(`{"throttleLimitCount":null,"throttleLimitUnit":null}`), map[string]string{"Content-Type": "application/json"})
	r = withCorrelationID(r, "corr-id-update-plan")
	server.UpdateSubscriptionPlan(w, r, "plan-gold")
	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, hub.publishedEvents, 1)
	assert.Equal(t, eventhub.EventTypeSubscriptionPlan, hub.publishedEvents[0].event.EventType)
	assert.Equal(t, "UPDATE", hub.publishedEvents[0].event.Action)
	assert.Equal(t, "plan-gold", hub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-id-update-plan", hub.publishedEvents[0].event.EventID)
}

func TestUpdateSubscriptionPlan_RejectedUpdatePublishesNoEvent(t *testing.T) {
	server, hub := newPlanTestServer(t)
	seedFullSubscriptionPlan(t, server, "plan-gold", "Gold")
	seedFullSubscriptionPlan(t, server, "plan-silver", "Silver")

	code, _ := putSubscriptionPlan(t, server, "plan-silver", "application/json", `{"planName":"Gold"}`)
	require.Equal(t, http.StatusConflict, code)
	assert.Empty(t, hub.publishedEvents)
}

// --- delete ---

func TestDeleteSubscriptionPlan_InUseConflictsUntilSubscriptionsAreRemoved(t *testing.T) {
	server, hub := newPlanTestServer(t)
	seedFullSubscriptionPlan(t, server, "plan-gold", "Gold")
	require.NoError(t, server.db.SaveSubscription(&models.Subscription{
		ID: "sub-1", APIID: "api-1", SubscriptionTokenHash: "hash-1",
		SubscriptionPlanID: ptr("plan-gold"), Status: models.SubscriptionStatusActive,
	}))

	code, body := deleteSubscriptionPlanViaHandler(t, server, "plan-gold")
	require.Equal(t, http.StatusConflict, code)
	assert.Equal(t, "Subscription plan is in use by one or more subscriptions", errorMessage(t, body))
	assertFullPlanUnchanged(t, server, "plan-gold", "Gold")
	assert.Empty(t, hub.publishedEvents)

	require.NoError(t, server.db.DeleteSubscription("sub-1", ""))
	code, _ = deleteSubscriptionPlanViaHandler(t, server, "plan-gold")
	require.Equal(t, http.StatusNoContent, code)
	_, err := server.db.GetSubscriptionPlanByID("plan-gold", "")
	assert.True(t, storage.IsNotFoundError(err))
}

func TestDeleteSubscriptionPlan_Errors(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		server, _ := newPlanTestServer(t)
		code, body := deleteSubscriptionPlanViaHandler(t, server, "missing")
		require.Equal(t, http.StatusNotFound, code)
		assert.Equal(t, "Subscription plan not found", errorMessage(t, body))
	})
	t.Run("storage failure", func(t *testing.T) {
		server, _ := newPlanTestServer(t)
		seedFullSubscriptionPlan(t, server, "plan-gold", "Gold")
		server.db.(*MockStorage).deleteErr = errors.New("database unavailable")
		code, body := deleteSubscriptionPlanViaHandler(t, server, "plan-gold")
		require.Equal(t, http.StatusInternalServerError, code)
		assert.Equal(t, "Failed to delete subscription plan", errorMessage(t, body))
	})
}
