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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/subscription"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// subsAPIUUID is the deployment ID seedAPIForAPIKeyHandlerTests assigns.
const subsAPIUUID = "0000-test-api-id-0000-000000000000"

type subsServer struct {
	s  *APIServer
	db *MockStorage
}

func newSubsServer(t *testing.T) *subsServer {
	t.Helper()
	server := createTestAPIServer()
	db, ok := server.db.(*MockStorage)
	require.True(t, ok)
	return &subsServer{s: server, db: db}
}

// subsDo sends one request to a handler. An empty contentType means JSON.
func subsDo(t *testing.T, method, path, body, contentType string,
	handler func(http.ResponseWriter, *http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	if body != "" {
		raw = []byte(body)
	}
	if contentType == "" {
		contentType = "application/json"
	}
	w, r := createTestContextWithHeader(method, path, raw, map[string]string{"Content-Type": contentType})
	handler(w, r)
	return w
}

func subsDecode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	require.NoErrorf(t, json.Unmarshal(rec.Body.Bytes(), &out), "body: %s", rec.Body.String())
	return out
}

// subsVal fails the test instead of panicking when an optional field is nil.
func subsVal[T any](t *testing.T, p *T) T {
	t.Helper()
	require.NotNil(t, p, "expected the field to be set")
	return *p
}

// subsError asserts an error response's status and exact message.
func subsError(t *testing.T, rec *httptest.ResponseRecorder, status int, message string) {
	t.Helper()
	assert.Equal(t, status, rec.Code, rec.Body.String())
	body := subsDecode[api.ErrorResponse](t, rec)
	assert.Equal(t, "error", body.Status)
	assert.Equal(t, message, body.Message)
}

// subsSeedSubscription stores a subscription directly, bypassing the handler.
func subsSeedSubscription(t *testing.T, db *MockStorage, id string) *models.Subscription {
	t.Helper()
	app := "app-1"
	sub := &models.Subscription{
		ID: id, APIID: subsAPIUUID, ApplicationID: &app,
		Status: models.SubscriptionStatusActive, SubscriptionToken: "stored-token-never-returned",
	}
	require.NoError(t, db.SaveSubscription(sub))
	return sub
}

// subsSeedPlan stores a subscription plan directly.
func subsSeedPlan(t *testing.T, db *MockStorage, id, name string, status models.SubscriptionPlanStatus) {
	t.Helper()
	require.NoError(t, db.SaveSubscriptionPlan(&models.SubscriptionPlan{ID: id, PlanName: name, Status: status}))
}

// subsSeedAPIOffering stores the REST API with an explicit list of plans it offers.
func subsSeedAPIOffering(t *testing.T, srv *subsServer, plans ...string) {
	t.Helper()
	cfg := seedAPIForAPIKeyHandlerTests(t, srv.s, "petstore")
	restAPI := cfg.Configuration.(api.RestAPI)
	restAPI.Spec.SubscriptionPlans = &plans
	cfg.Configuration = restAPI
	require.NoError(t, srv.db.SaveConfig(cfg))
}

// The handle is accepted in place of the deployment ID, the token is returned
// exactly once on the create response, and the row stores the resolved ID.
func TestCreateSubscriptionResolvesTheHandleAndReturnsTheTokenOnce(t *testing.T) {
	srv := newSubsServer(t)
	seedAPIForAPIKeyHandlerTests(t, srv.s, "petstore")

	rec := subsDo(t, http.MethodPost, "/subscriptions", `{
		"apiId": "petstore", "subscriptionToken": "tok-123", "applicationId": "app-9",
		"billingCustomerId": "cust-1", "billingSubscriptionId": "bill-1", "status": "INACTIVE"}`,
		"", srv.s.CreateSubscription)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	got := subsDecode[api.SubscriptionResponse](t, rec)
	require.NotNil(t, got.Id)
	assert.Equal(t, subsAPIUUID, subsVal(t, got.ApiId), "the response carries the resolved deployment ID")
	assert.Equal(t, "tok-123", subsVal(t, got.SubscriptionToken))
	assert.Equal(t, "app-9", subsVal(t, got.ApplicationId))
	assert.Equal(t, "cust-1", subsVal(t, got.BillingCustomerId))
	assert.Equal(t, "bill-1", subsVal(t, got.BillingSubscriptionId))
	assert.Equal(t, api.SubscriptionResponseStatus("INACTIVE"), subsVal(t, got.Status))

	stored, err := srv.db.GetSubscriptionByID(subsVal(t, got.Id), "")
	require.NoError(t, err)
	assert.Equal(t, subsAPIUUID, stored.APIID)
	assert.Equal(t, models.SubscriptionStatusInactive, stored.Status)
}

func TestCreateSubscriptionAcceptsYAML(t *testing.T) {
	srv := newSubsServer(t)
	seedAPIForAPIKeyHandlerTests(t, srv.s, "petstore")

	rec := subsDo(t, http.MethodPost, "/subscriptions",
		"apiId: petstore\nsubscriptionToken: tok-yaml\n", "application/yaml", srv.s.CreateSubscription)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Equal(t, "tok-yaml", subsVal(t, subsDecode[api.SubscriptionResponse](t, rec).SubscriptionToken))
}

// A plan the API explicitly offers is accepted, matched by name without regard
// to case.
func TestCreateSubscriptionAcceptsAPlanTheAPIOffers(t *testing.T) {
	srv := newSubsServer(t)
	subsSeedAPIOffering(t, srv, "gold")
	subsSeedPlan(t, srv.db, "plan-gold", "Gold", models.SubscriptionPlanStatusActive)

	rec := subsDo(t, http.MethodPost, "/subscriptions",
		`{"apiId": "petstore", "subscriptionToken": "t", "subscriptionPlanId": "plan-gold"}`,
		"", srv.s.CreateSubscription)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Equal(t, "plan-gold", subsVal(t, subsDecode[api.SubscriptionResponse](t, rec).SubscriptionPlanId))
}

// Every rejection is reported with the status and message a client can act on,
// and nothing is stored.
func TestCreateSubscriptionRejections(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, srv *subsServer)
		body    string
		status  int
		message string
	}{
		{"malformed body", nil, `{"apiId": `, http.StatusBadRequest, "Invalid request body"},
		{"missing apiId", nil, `{"subscriptionToken": "t"}`, http.StatusBadRequest, "apiId is required"},
		{"blank token", nil, `{"apiId": "petstore", "subscriptionToken": "  "}`,
			http.StatusBadRequest, "subscriptionToken is required"},
		{"unknown API", nil, `{"apiId": "nope", "subscriptionToken": "t"}`,
			http.StatusNotFound, "RestAPI with identifier 'nope' not found"},
		{"not a REST API", func(t *testing.T, srv *subsServer) {
			require.NoError(t, srv.db.SaveConfig(&models.StoredConfig{UUID: "mcp-1", Kind: models.KindMcp, Handle: "tools"}))
		}, `{"apiId": "mcp-1", "subscriptionToken": "t"}`,
			http.StatusBadRequest, "Configuration with identifier 'mcp-1' is not a REST API"},
		{"invalid status", func(t *testing.T, srv *subsServer) {
			seedAPIForAPIKeyHandlerTests(t, srv.s, "petstore")
		}, `{"apiId": "petstore", "subscriptionToken": "t", "status": "PAUSED"}`,
			http.StatusBadRequest, "invalid status: PAUSED"},
		{"unknown plan", func(t *testing.T, srv *subsServer) {
			seedAPIForAPIKeyHandlerTests(t, srv.s, "petstore")
		}, `{"apiId": "petstore", "subscriptionToken": "t", "subscriptionPlanId": "no-plan"}`,
			http.StatusBadRequest, "Subscription plan not found or not enabled"},
		{"inactive plan", func(t *testing.T, srv *subsServer) {
			seedAPIForAPIKeyHandlerTests(t, srv.s, "petstore")
			subsSeedPlan(t, srv.db, "plan-x", "Legacy", models.SubscriptionPlanStatusInactive)
		}, `{"apiId": "petstore", "subscriptionToken": "t", "subscriptionPlanId": "plan-x"}`,
			http.StatusBadRequest, "Subscription plan is not active"},
		{"plan the API does not offer", func(t *testing.T, srv *subsServer) {
			subsSeedAPIOffering(t, srv, "Gold")
			subsSeedPlan(t, srv.db, "plan-silver", "Silver", models.SubscriptionPlanStatusActive)
		}, `{"apiId": "petstore", "subscriptionToken": "t", "subscriptionPlanId": "plan-silver"}`,
			http.StatusBadRequest, `Subscription plan "Silver" is not enabled for this API`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newSubsServer(t)
			if tt.setup != nil {
				tt.setup(t, srv)
			}

			rec := subsDo(t, http.MethodPost, "/subscriptions", tt.body, "", srv.s.CreateSubscription)

			subsError(t, rec, tt.status, tt.message)
			assert.Empty(t, srv.db.subscriptions, "a rejected request stores nothing")
		})
	}
}

// Storage failures are reported generically: the underlying error is logged,
// never returned.
func TestCreateSubscriptionStorageFailures(t *testing.T) {
	for _, tt := range []struct {
		name    string
		inject  func(db *MockStorage)
		status  int
		message string
	}{
		{"already subscribed", func(db *MockStorage) { db.saveErr = fmt.Errorf("%w: unique pair", storage.ErrConflict) },
			http.StatusConflict, "Application already subscribed to this API"},
		{"save fails", func(db *MockStorage) { db.saveErr = errors.New("disk full at /var/lib/gw.db") },
			http.StatusInternalServerError, "Failed to create subscription"},
		{"API lookup fails", func(db *MockStorage) { db.getErr = errors.New("database is locked") },
			http.StatusInternalServerError, "Failed to resolve API identifier"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv := newSubsServer(t)
			seedAPIForAPIKeyHandlerTests(t, srv.s, "petstore")
			tt.inject(srv.db)

			rec := subsDo(t, http.MethodPost, "/subscriptions",
				`{"apiId": "petstore", "subscriptionToken": "t"}`, "", srv.s.CreateSubscription)

			subsError(t, rec, tt.status, tt.message)
			assert.NotContains(t, rec.Body.String(), "/var/lib", "no internal detail in the response")
			assert.NotContains(t, rec.Body.String(), "locked")
		})
	}
}

func TestListSubscriptionsFiltersAndNeverReturnsTokens(t *testing.T) {
	srv := newSubsServer(t)
	seedAPIForAPIKeyHandlerTests(t, srv.s, "petstore")
	subsSeedSubscription(t, srv.db, "sub-1")
	other := "app-2"
	require.NoError(t, srv.db.SaveSubscription(&models.Subscription{
		ID: "sub-2", APIID: "some-other-api", ApplicationID: &other, Status: models.SubscriptionStatusRevoked,
	}))
	list := func(params api.ListSubscriptionsParams) *httptest.ResponseRecorder {
		return subsDo(t, http.MethodGet, "/subscriptions", "", "",
			func(w http.ResponseWriter, r *http.Request) { srv.s.ListSubscriptions(w, r, params) })
	}

	rec := list(api.ListSubscriptionsParams{})
	require.Equal(t, http.StatusOK, rec.Code)
	all := subsDecode[api.SubscriptionListResponse](t, rec)
	assert.Equal(t, 2, subsVal(t, all.Count))
	assert.NotContains(t, rec.Body.String(), "stored-token-never-returned")
	assert.NotContains(t, rec.Body.String(), "subscriptionToken")

	byHandle := "petstore"
	got := subsDecode[api.SubscriptionListResponse](t, list(api.ListSubscriptionsParams{ApiId: &byHandle}))
	require.Equal(t, 1, subsVal(t, got.Count), "the handle resolves to the API's deployment ID")
	assert.Equal(t, "sub-1", subsVal(t, subsVal(t, got.Subscriptions)[0].Id))

	app := "app-2"
	got = subsDecode[api.SubscriptionListResponse](t, list(api.ListSubscriptionsParams{ApplicationId: &app}))
	require.Equal(t, 1, subsVal(t, got.Count))
	assert.Equal(t, "sub-2", subsVal(t, subsVal(t, got.Subscriptions)[0].Id))

	status := api.ListSubscriptionsParamsStatus("ACTIVE")
	got = subsDecode[api.SubscriptionListResponse](t, list(api.ListSubscriptionsParams{Status: &status}))
	require.Equal(t, 1, subsVal(t, got.Count))
	assert.Equal(t, "sub-1", subsVal(t, subsVal(t, got.Subscriptions)[0].Id))
}

// An empty result is an empty array, not a missing field or null.
func TestListSubscriptionsEmpty(t *testing.T) {
	srv := newSubsServer(t)

	rec := subsDo(t, http.MethodGet, "/subscriptions", "", "",
		func(w http.ResponseWriter, r *http.Request) {
			srv.s.ListSubscriptions(w, r, api.ListSubscriptionsParams{})
		})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"count": 0, "subscriptions": []}`, rec.Body.String())
}

func TestListSubscriptionsFailures(t *testing.T) {
	for _, tt := range []struct {
		name    string
		setup   func(srv *subsServer)
		apiID   string
		status  int
		message string
	}{
		{"unknown apiId filter", nil, "nope", http.StatusNotFound, "RestAPI with identifier 'nope' not found"},
		{"apiId filter is not a REST API", func(srv *subsServer) {
			_ = srv.db.SaveConfig(&models.StoredConfig{UUID: "mcp-1", Kind: models.KindMcp, Handle: "tools"})
		}, "mcp-1", http.StatusBadRequest, "Configuration with identifier 'mcp-1' is not a REST API"},
		{"apiId lookup fails", func(srv *subsServer) { srv.db.getErr = errors.New("database is locked") },
			"petstore", http.StatusInternalServerError, "Failed to resolve API identifier"},
		{"listing fails", func(srv *subsServer) { srv.db.getErr = errors.New("database is locked") },
			"", http.StatusInternalServerError, "Failed to list subscriptions"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv := newSubsServer(t)
			if tt.setup != nil {
				tt.setup(srv)
			}
			params := api.ListSubscriptionsParams{}
			if tt.apiID != "" {
				params.ApiId = &tt.apiID
			}

			rec := subsDo(t, http.MethodGet, "/subscriptions", "", "",
				func(w http.ResponseWriter, r *http.Request) { srv.s.ListSubscriptions(w, r, params) })

			subsError(t, rec, tt.status, tt.message)
		})
	}
}

func TestGetSubscription(t *testing.T) {
	srv := newSubsServer(t)
	subsSeedSubscription(t, srv.db, "sub-1")
	get := func(id string) *httptest.ResponseRecorder {
		return subsDo(t, http.MethodGet, "/subscriptions/"+id, "", "",
			func(w http.ResponseWriter, r *http.Request) { srv.s.GetSubscription(w, r, id) })
	}

	rec := get("sub-1")
	require.Equal(t, http.StatusOK, rec.Code)
	got := subsDecode[api.SubscriptionResponse](t, rec)
	assert.Equal(t, "sub-1", subsVal(t, got.Id))
	assert.Nil(t, got.SubscriptionToken, "a read never returns the token")

	subsError(t, get("no-such-sub"), http.StatusNotFound, "Subscription not found")

	srv.db.getErr = errors.New("database is locked")
	subsError(t, get("sub-1"), http.StatusInternalServerError, "Failed to get subscription")
}

func TestUpdateSubscriptionChangesTheStatus(t *testing.T) {
	srv := newSubsServer(t)
	subsSeedSubscription(t, srv.db, "sub-1")

	rec := subsDo(t, http.MethodPut, "/subscriptions/sub-1", `{"status": "REVOKED"}`, "",
		func(w http.ResponseWriter, r *http.Request) { srv.s.UpdateSubscription(w, r, "sub-1") })

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := subsDecode[api.SubscriptionResponse](t, rec)
	assert.Equal(t, api.SubscriptionResponseStatus("REVOKED"), subsVal(t, got.Status))
	assert.Nil(t, got.SubscriptionToken)
	assert.Equal(t, models.SubscriptionStatusRevoked, srv.db.subscriptions["sub-1"].Status)
}

// The REST update accepts a body with no status and leaves the row as it was.
func TestUpdateSubscriptionWithoutAStatusChangesNothing(t *testing.T) {
	srv := newSubsServer(t)
	subsSeedSubscription(t, srv.db, "sub-1")

	rec := subsDo(t, http.MethodPut, "/subscriptions/sub-1", `{}`, "",
		func(w http.ResponseWriter, r *http.Request) { srv.s.UpdateSubscription(w, r, "sub-1") })

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, api.SubscriptionResponseStatus("ACTIVE"), subsVal(t, subsDecode[api.SubscriptionResponse](t, rec).Status))
}

func TestUpdateSubscriptionFailures(t *testing.T) {
	for _, tt := range []struct {
		name    string
		id      string
		body    string
		inject  func(db *MockStorage)
		status  int
		message string
	}{
		// Existence is checked before the body is read, so an unknown id is a
		// 404 even when the body is also invalid.
		{"unknown id with a malformed body", "no-such-sub", `{"status": `, nil,
			http.StatusNotFound, "Subscription not found"},
		{"malformed body", "sub-1", `{"status": `, nil, http.StatusBadRequest, "Invalid request body"},
		{"invalid status", "sub-1", `{"status": "PAUSED"}`, nil, http.StatusBadRequest, "invalid status: PAUSED"},
		{"existence check fails", "sub-1", `{"status": "REVOKED"}`,
			func(db *MockStorage) { db.getErr = errors.New("database is locked") },
			http.StatusInternalServerError, "Failed to get subscription"},
		{"write fails", "sub-1", `{"status": "REVOKED"}`,
			func(db *MockStorage) { db.updateErr = errors.New("database is locked") },
			http.StatusInternalServerError, "Failed to update subscription"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv := newSubsServer(t)
			subsSeedSubscription(t, srv.db, "sub-1")
			if tt.inject != nil {
				tt.inject(srv.db)
			}

			rec := subsDo(t, http.MethodPut, "/subscriptions/"+tt.id, tt.body, "",
				func(w http.ResponseWriter, r *http.Request) { srv.s.UpdateSubscription(w, r, tt.id) })

			subsError(t, rec, tt.status, tt.message)
			// MockStorage returns its stored pointer, so a failed write still shows the change
			// in memory; only check the cases rejected before the row is modified.
			if srv.db.updateErr == nil {
				assert.Equal(t, models.SubscriptionStatusActive, srv.db.subscriptions["sub-1"].Status,
					"a refused update leaves the row unchanged")
			}
		})
	}
}

func TestDeleteSubscription(t *testing.T) {
	srv := newSubsServer(t)
	subsSeedSubscription(t, srv.db, "sub-1")
	del := func(id string) *httptest.ResponseRecorder {
		return subsDo(t, http.MethodDelete, "/subscriptions/"+id, "", "",
			func(w http.ResponseWriter, r *http.Request) { srv.s.DeleteSubscription(w, r, id) })
	}

	rec := del("sub-1")
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
	assert.NotContains(t, srv.db.subscriptions, "sub-1")

	subsError(t, del("sub-1"), http.StatusNotFound, "Subscription not found")
}

func TestDeleteSubscriptionFailures(t *testing.T) {
	for _, tt := range []struct {
		name    string
		inject  func(db *MockStorage)
		message string
	}{
		{"existence check fails", func(db *MockStorage) { db.getErr = errors.New("database is locked") },
			"Failed to get subscription"},
		{"delete fails", func(db *MockStorage) { db.deleteErr = errors.New("database is locked") },
			"Failed to delete subscription"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv := newSubsServer(t)
			subsSeedSubscription(t, srv.db, "sub-1")
			tt.inject(srv.db)

			rec := subsDo(t, http.MethodDelete, "/subscriptions/sub-1", "", "",
				func(w http.ResponseWriter, r *http.Request) { srv.s.DeleteSubscription(w, r, "sub-1") })

			subsError(t, rec, http.StatusInternalServerError, tt.message)
			assert.Contains(t, srv.db.subscriptions, "sub-1")
		})
	}
}

func TestSubscriptionErrorMappersRemainingBranches(t *testing.T) {
	rec := httptest.NewRecorder()
	mapCreateSubscriptionError(rec, toolsDiscard, "petstore",
		&subscription.OpError{Op: subscription.OpValidate, Cause: errors.New("config gone")})
	subsError(t, rec, http.StatusInternalServerError, "Failed to validate subscription plan")

	// The row disappearing between the existence check and the write.
	rec = httptest.NewRecorder()
	mapSubscriptionUpdateError(rec, toolsDiscard, subscription.ErrSubscriptionNotFound)
	subsError(t, rec, http.StatusNotFound, "Subscription not found")

	// Update re-reads the row; a failure there is a read failure.
	rec = httptest.NewRecorder()
	mapSubscriptionUpdateError(rec, toolsDiscard, &subscription.OpError{Op: subscription.OpLoad, Cause: errors.New("x")})
	subsError(t, rec, http.StatusInternalServerError, "Failed to get subscription")
}

// Optional fields that are not set are omitted, and a stored token is never
// part of a read response.
func TestSubscriptionToResponseOmitsUnsetFields(t *testing.T) {
	resp := subscriptionToResponse(&models.Subscription{
		ID: "sub-1", APIID: "api-1", GatewayID: "gw-1", SubscriptionToken: "secret",
	})
	raw, err := json.Marshal(resp)
	require.NoError(t, err)

	for _, field := range []string{"applicationId", "subscriptionPlanId", "billingCustomerId",
		"billingSubscriptionId", "status", "subscriptionToken"} {
		assert.NotContainsf(t, string(raw), `"`+field+`"`, "%s must be omitted", field)
	}
	assert.Equal(t, "sub-1", subsVal(t, resp.Id))
	assert.Equal(t, "gw-1", subsVal(t, resp.GatewayId))
}

func TestSubscriptionToResponseWithTokenAddsOnlyARealToken(t *testing.T) {
	assert.Equal(t, "tok", subsVal(t, subscriptionToResponseWithToken(&models.Subscription{SubscriptionToken: "tok"}).SubscriptionToken))
	assert.Nil(t, subscriptionToResponseWithToken(&models.Subscription{}).SubscriptionToken,
		"no token, no field")
}
