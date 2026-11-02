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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// --- helpers ---

func newSubscriptionTestServer(t *testing.T) (*APIServer, *models.StoredConfig) {
	t.Helper()
	server, cfg, _ := newSubscriptionTestServerWithHub(t)
	return server, cfg
}

func newSubscriptionTestServerWithHub(t *testing.T) (*APIServer, *models.StoredConfig, *mockEventHub) {
	t.Helper()
	server := createTestAPIServer()
	hub := &mockEventHub{}
	attachTestEventHub(server, hub, "test-gateway")
	cfg := seedAPIForAPIKeyHandlerTests(t, server, "subscription-api")
	return server, cfg, hub
}

// newRestrictedPlanTestServer seeds an API whose spec only enables the "allowed" plan by name.
func newRestrictedPlanTestServer(t *testing.T) (*APIServer, *models.StoredConfig) {
	t.Helper()
	server := createTestAPIServer()
	attachTestEventHub(server, &mockEventHub{}, "test-gateway")
	cfg := createTestStoredConfig("0000-restricted-api-0000-000000000000", "restricted-api", "v1.0.0", "/restricted")
	cfg.Handle = "restricted-api"
	restCfg, ok := cfg.Configuration.(api.RestAPI)
	require.True(t, ok)
	restCfg.Metadata.Name = "restricted-api"
	restCfg.Spec.SubscriptionPlans = &[]string{"Allowed"}
	cfg.Configuration = restCfg
	require.NoError(t, server.store.Add(cfg))
	require.NoError(t, server.db.SaveConfig(cfg))
	return server, cfg
}

func seedSubscriptionPlan(t *testing.T, server *APIServer, id string, status models.SubscriptionPlanStatus) {
	t.Helper()
	seedNamedSubscriptionPlan(t, server, id, "plan-"+id, status)
}

func seedNamedSubscriptionPlan(t *testing.T, server *APIServer, id, name string, status models.SubscriptionPlanStatus) {
	t.Helper()
	require.NoError(t, server.db.SaveSubscriptionPlan(&models.SubscriptionPlan{
		ID:       id,
		PlanName: name,
		Status:   status,
	}))
}

func postSubscription(t *testing.T, server *APIServer, contentType string, body []byte) (int, []byte) {
	t.Helper()
	w, r := createTestContextWithHeader("POST", "/subscriptions", body, map[string]string{"Content-Type": contentType})
	server.CreateSubscription(w, r)
	return w.Code, w.Body.Bytes()
}

func createSubscriptionViaHandler(t *testing.T, server *APIServer, req api.SubscriptionCreateRequest) (int, api.SubscriptionResponse) {
	t.Helper()
	body, err := json.Marshal(req)
	require.NoError(t, err)
	code, respBody := postSubscription(t, server, "application/json", body)
	var resp api.SubscriptionResponse
	if code == http.StatusCreated {
		require.NoError(t, json.Unmarshal(respBody, &resp))
	}
	return code, resp
}

func putSubscription(t *testing.T, server *APIServer, id, contentType string, body []byte) (int, []byte) {
	t.Helper()
	w, r := createTestContextWithHeader("PUT", "/subscriptions/"+id, body, map[string]string{"Content-Type": contentType})
	server.UpdateSubscription(w, r, id)
	return w.Code, w.Body.Bytes()
}

func updateSubscriptionViaHandler(t *testing.T, server *APIServer, id, body string) (int, api.SubscriptionResponse) {
	t.Helper()
	code, respBody := putSubscription(t, server, id, "application/json", []byte(body))
	var resp api.SubscriptionResponse
	if code == http.StatusOK {
		require.NoError(t, json.Unmarshal(respBody, &resp))
	}
	return code, resp
}

func getSubscriptionViaHandler(t *testing.T, server *APIServer, id string) (int, api.SubscriptionResponse) {
	t.Helper()
	w, r := createTestContext("GET", "/subscriptions/"+id, nil)
	server.GetSubscription(w, r, id)
	var resp api.SubscriptionResponse
	if w.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	}
	return w.Code, resp
}

func errorMessage(t *testing.T, body []byte) string {
	t.Helper()
	var resp api.ErrorResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	return resp.Message
}

// assertGenericSubscriptionConflict checks a 409 that doesn't reveal which value collided,
// so a caller can't use it to probe for tokens that are already in use.
func assertGenericSubscriptionConflict(t *testing.T, code int, body []byte) {
	t.Helper()
	require.Equal(t, http.StatusConflict, code)
	assert.Equal(t, subscriptionConflictMessage, errorMessage(t, body))
	lower := strings.ToLower(string(body))
	assert.NotContains(t, lower, "token")
	assert.NotContains(t, lower, "externalrefid")
}

// failingConfigStorage fails GetConfig for one API ID, to reach the plan-validation lookup error.
type failingConfigStorage struct {
	*MockStorage
	failConfigID string
}

func (f *failingConfigStorage) GetConfig(id string) (*models.StoredConfig, error) {
	if id == f.failConfigID {
		return nil, errors.New("database unavailable")
	}
	return f.MockStorage.GetConfig(id)
}

// --- Issue point 1: caller-supplied reference ID on create ---

func TestCreateSubscription_ExternalRefIDUsedAsID(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)

	code, resp := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId:             cfg.UUID,
		SubscriptionToken: "token-ext-1",
		ExternalRefId:     ptr("apim-sub-123"),
	})
	require.Equal(t, http.StatusCreated, code)
	require.NotNil(t, resp.Id)
	require.NotNil(t, resp.ExternalRefId)
	assert.Equal(t, "apim-sub-123", *resp.Id)
	assert.Equal(t, "apim-sub-123", *resp.ExternalRefId)
	require.NotNil(t, resp.SubscriptionToken, "the create response is the one place the token is returned")
	assert.Equal(t, "token-ext-1", *resp.SubscriptionToken)

	stored, err := server.db.GetSubscriptionByID("apim-sub-123", "")
	require.NoError(t, err)
	assert.Equal(t, cfg.UUID, stored.APIID)

	// GET by the caller's ID returns both id and externalRefId, and never the token.
	code, got := getSubscriptionViaHandler(t, server, "apim-sub-123")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "apim-sub-123", *got.Id)
	assert.Equal(t, "apim-sub-123", *got.ExternalRefId)
	assert.Nil(t, got.SubscriptionToken)
}

func TestCreateSubscription_WithoutExternalRefIDGeneratesID(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)

	code, resp := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId:             cfg.UUID,
		SubscriptionToken: "token-gen-1",
	})
	require.Equal(t, http.StatusCreated, code)
	require.NotNil(t, resp.Id)
	assert.NotEmpty(t, *resp.Id)
	require.NotNil(t, resp.ExternalRefId)
	assert.Equal(t, *resp.Id, *resp.ExternalRefId)
}

func TestCreateSubscription_EmptyExternalRefIDTreatedAsAbsent(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)

	code, resp := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId:             cfg.UUID,
		SubscriptionToken: "token-empty-ref",
		ExternalRefId:     ptr(""),
	})
	require.Equal(t, http.StatusCreated, code)
	assert.NotEmpty(t, *resp.Id, "an empty externalRefId falls back to a generated ID")
}

func TestCreateSubscription_ValidExternalRefIDFormats(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)

	for i, ref := range []string{
		"a",
		"A1",
		"apim-sub_1.v2",
		"5f1c2a9e-1234-5678-9abc-def012345678",
		strings.Repeat("a", maxExternalRefIDLength),
	} {
		t.Run(ref, func(t *testing.T) {
			code, resp := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
				ApiId:             cfg.UUID,
				SubscriptionToken: fmt.Sprintf("token-valid-%d", i),
				ExternalRefId:     ptr(ref),
			})
			require.Equal(t, http.StatusCreated, code)
			assert.Equal(t, ref, *resp.Id)
		})
	}
}

func TestCreateSubscription_InvalidExternalRefID(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)

	for i, ref := range []string{
		" ref",                // leading whitespace is rejected, not trimmed
		"ref ",                // trailing whitespace
		"\tref",               // tab
		"ref\n",               // trailing newline
		"   ",                 // whitespace only
		"has space",           // inner whitespace
		"-leading-dash",       // must start with a letter or digit
		".leading-dot",        //
		"_leading-underscore", //
		"a/b",                 // path separator
		"../etc",              // traversal
		"ref%20",              // encoded characters
		"ref?x=1",             // query characters
		strings.Repeat("a", maxExternalRefIDLength+1),
	} {
		t.Run(fmt.Sprintf("%q", ref), func(t *testing.T) {
			code, body := postSubscription(t, server, "application/json", mustJSON(t, api.SubscriptionCreateRequest{
				ApiId:             cfg.UUID,
				SubscriptionToken: fmt.Sprintf("token-invalid-%d", i),
				ExternalRefId:     ptr(ref),
			}))
			require.Equal(t, http.StatusBadRequest, code)
			assert.Contains(t, errorMessage(t, body), "externalRefId")
		})
	}
	list, err := server.db.ListSubscriptionsByAPI(cfg.UUID, "", nil, nil)
	require.NoError(t, err)
	assert.Empty(t, list, "a rejected externalRefId must not create a subscription")
}

func TestCreateSubscription_DuplicateExternalRefIDConflicts(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)

	code, _ := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId: cfg.UUID, SubscriptionToken: "token-dup-1", ExternalRefId: ptr("dup-ref"),
	})
	require.Equal(t, http.StatusCreated, code)

	code, body := postSubscription(t, server, "application/json", mustJSON(t, api.SubscriptionCreateRequest{
		ApiId: cfg.UUID, SubscriptionToken: "token-dup-2", ExternalRefId: ptr("dup-ref"),
	}))
	assertGenericSubscriptionConflict(t, code, body)

	// The original subscription is untouched.
	stored, err := server.db.GetSubscriptionByID("dup-ref", "")
	require.NoError(t, err)
	assert.Equal(t, "token-dup-1", stored.SubscriptionToken)
}

func TestCreateSubscription_TokenConflictIsGeneric(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)

	server.db.(*MockStorage).saveErr = fmt.Errorf("%w: subscription token already exists for this API", storage.ErrConflict)
	code, body := postSubscription(t, server, "application/json", mustJSON(t, api.SubscriptionCreateRequest{
		ApiId: cfg.UUID, SubscriptionToken: "token-taken",
	}))
	assertGenericSubscriptionConflict(t, code, body)
}

func TestCreateSubscription_PublishesEventWithExternalRefID(t *testing.T) {
	server, cfg, hub := newSubscriptionTestServerWithHub(t)

	code, _ := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId: cfg.UUID, SubscriptionToken: "token-event", ExternalRefId: ptr("event-ref"),
	})
	require.Equal(t, http.StatusCreated, code)
	require.Len(t, hub.publishedEvents, 1)
	assert.Equal(t, eventhub.EventTypeSubscription, hub.publishedEvents[0].event.EventType)
	assert.Equal(t, "CREATE", hub.publishedEvents[0].event.Action)
	assert.Equal(t, "event-ref", hub.publishedEvents[0].event.EntityID)
}

// The example from the issue: several subscriptions on one API with no applicationId used to be
// indistinguishable in GET /subscriptions?apiId=... With externalRefId each one can be told apart
// and addressed directly.
func TestSubscriptions_WithoutApplicationIDAreDistinguishableByExternalRefID(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)

	refs := []string{"apim-sub-a", "apim-sub-b", "apim-sub-c"}
	for i, ref := range refs {
		code, _ := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
			ApiId: cfg.UUID, SubscriptionToken: fmt.Sprintf("token-multi-%d", i), ExternalRefId: ptr(ref),
		})
		require.Equal(t, http.StatusCreated, code)
	}

	apiID := cfg.UUID
	w, r := createTestContext("GET", "/subscriptions?apiId="+apiID, nil)
	server.ListSubscriptions(w, r, api.ListSubscriptionsParams{ApiId: &apiID})
	require.Equal(t, http.StatusOK, w.Code)
	var list api.SubscriptionListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.Equal(t, 3, *list.Count)
	listed := map[string]bool{}
	for _, sub := range *list.Subscriptions {
		assert.Nil(t, sub.ApplicationId)
		require.NotNil(t, sub.ExternalRefId)
		assert.Equal(t, *sub.Id, *sub.ExternalRefId)
		listed[*sub.ExternalRefId] = true
	}
	for _, ref := range refs {
		assert.True(t, listed[ref], "list must include %s", ref)
	}

	// Updating one by its reference leaves the others alone.
	code, _ := updateSubscriptionViaHandler(t, server, "apim-sub-b", `{"status":"REVOKED"}`)
	require.Equal(t, http.StatusOK, code)
	for _, ref := range refs {
		code, got := getSubscriptionViaHandler(t, server, ref)
		require.Equal(t, http.StatusOK, code)
		want := api.SubscriptionResponseStatus("ACTIVE")
		if ref == "apim-sub-b" {
			want = "REVOKED"
		}
		assert.Equal(t, want, *got.Status, ref)
	}
}

// --- Create request validation and error paths ---

func TestCreateSubscription_OptionalFieldsArePersistedAndKeptOnUpdate(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)
	seedSubscriptionPlan(t, server, "plan-a", models.SubscriptionPlanStatusActive)
	inactive := api.SubscriptionCreateRequestStatus("INACTIVE")

	code, resp := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId:                 cfg.UUID,
		SubscriptionToken:     "token-optional",
		ExternalRefId:         ptr("sub-optional"),
		ApplicationId:         ptr("app-1"),
		SubscriptionPlanId:    ptr("plan-a"),
		BillingCustomerId:     ptr("cus-1"),
		BillingSubscriptionId: ptr("bsub-1"),
		Status:                &inactive,
	})
	require.Equal(t, http.StatusCreated, code)
	assert.Equal(t, "app-1", *resp.ApplicationId)
	assert.Equal(t, "cus-1", *resp.BillingCustomerId)
	assert.Equal(t, "bsub-1", *resp.BillingSubscriptionId)
	assert.Equal(t, api.SubscriptionResponseStatus("INACTIVE"), *resp.Status)

	// PUT can't change these fields, so an update must carry them over unchanged.
	code, updated := updateSubscriptionViaHandler(t, server, "sub-optional", `{"status":"ACTIVE","subscriptionToken":"token-optional-2"}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "app-1", *updated.ApplicationId)
	assert.Equal(t, "cus-1", *updated.BillingCustomerId)
	assert.Equal(t, "bsub-1", *updated.BillingSubscriptionId)
	assert.Equal(t, cfg.UUID, *updated.ApiId)
	assert.Equal(t, "plan-a", *updated.SubscriptionPlanId)
}

func TestCreateSubscription_EmptyApplicationIDTreatedAsAbsent(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)

	code, resp := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId: cfg.UUID, SubscriptionToken: "token-no-app", ApplicationId: ptr(""),
	})
	require.Equal(t, http.StatusCreated, code)
	assert.Nil(t, resp.ApplicationId)
}

func TestCreateSubscription_RequestValidation(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)

	tests := []struct {
		name     string
		body     string
		wantCode int
		wantMsg  string
	}{
		{"malformed JSON", `{"apiId":`, http.StatusBadRequest, "Invalid request body"},
		{"missing apiId", `{"subscriptionToken":"t"}`, http.StatusBadRequest, "apiId is required"},
		{"blank apiId", `{"apiId":"  ","subscriptionToken":"t"}`, http.StatusBadRequest, "apiId is required"},
		{"missing token", fmt.Sprintf(`{"apiId":%q}`, cfg.UUID), http.StatusBadRequest, "subscriptionToken is required"},
		{"blank token", fmt.Sprintf(`{"apiId":%q,"subscriptionToken":"   "}`, cfg.UUID), http.StatusBadRequest, "subscriptionToken is required"},
		{"invalid status", fmt.Sprintf(`{"apiId":%q,"subscriptionToken":"t","status":"PAUSED"}`, cfg.UUID), http.StatusBadRequest, "invalid status"},
		{"unknown API", `{"apiId":"no-such-api","subscriptionToken":"t"}`, http.StatusNotFound, "not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, body := postSubscription(t, server, "application/json", []byte(tt.body))
			require.Equal(t, tt.wantCode, code)
			assert.Contains(t, errorMessage(t, body), tt.wantMsg)
		})
	}
}

func TestCreateSubscription_ResolvesAPIByHandleAndTrimsToken(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)

	code, resp := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId: "subscription-api", SubscriptionToken: "  padded-token  ", ExternalRefId: ptr("by-handle"),
	})
	require.Equal(t, http.StatusCreated, code)
	assert.Equal(t, cfg.UUID, *resp.ApiId, "the handle resolves to the deployment ID")
	stored, err := server.db.GetSubscriptionByID("by-handle", "")
	require.NoError(t, err)
	assert.Equal(t, "padded-token", stored.SubscriptionToken)
}

func TestCreateSubscription_BodyTooLarge(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)

	body := fmt.Sprintf(`{"apiId":%q,"subscriptionToken":%q}`, cfg.UUID, strings.Repeat("x", maxSubscriptionRequestBodyBytes))
	code, respBody := postSubscription(t, server, "application/json", []byte(body))
	require.Equal(t, http.StatusRequestEntityTooLarge, code)
	assert.Equal(t, "Request body too large", errorMessage(t, respBody))
}

func TestCreateSubscription_SaveFailure(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)

	server.db.(*MockStorage).saveErr = errors.New("database unavailable")
	code, body := postSubscription(t, server, "application/json", mustJSON(t, api.SubscriptionCreateRequest{
		ApiId: cfg.UUID, SubscriptionToken: "token-fail",
	}))
	require.Equal(t, http.StatusInternalServerError, code)
	assert.Equal(t, "Failed to create subscription", errorMessage(t, body))
}

func TestCreateSubscription_PlanValidation(t *testing.T) {
	server, cfg := newRestrictedPlanTestServer(t)
	seedNamedSubscriptionPlan(t, server, "plan-allowed", "Allowed", models.SubscriptionPlanStatusActive)
	seedNamedSubscriptionPlan(t, server, "plan-other", "Other", models.SubscriptionPlanStatusActive)
	seedNamedSubscriptionPlan(t, server, "plan-inactive", "Allowed-Inactive", models.SubscriptionPlanStatusInactive)

	tests := []struct {
		name     string
		planID   string
		wantCode int
		wantMsg  string
	}{
		{"plan not found", "no-such-plan", http.StatusBadRequest, "not found"},
		{"plan inactive", "plan-inactive", http.StatusBadRequest, "not active"},
		{"plan not enabled for API", "plan-other", http.StatusBadRequest, "not enabled for this API"},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, body := postSubscription(t, server, "application/json", mustJSON(t, api.SubscriptionCreateRequest{
				ApiId: cfg.UUID, SubscriptionToken: fmt.Sprintf("token-plan-%d", i), SubscriptionPlanId: ptr(tt.planID),
			}))
			require.Equal(t, tt.wantCode, code)
			assert.Contains(t, errorMessage(t, body), tt.wantMsg)
		})
	}

	// The enabled check matches plan names case-insensitively.
	seedNamedSubscriptionPlan(t, server, "plan-allowed-lower", "allowed", models.SubscriptionPlanStatusActive)
	code, _ := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId: cfg.UUID, SubscriptionToken: "token-plan-ok", SubscriptionPlanId: ptr("plan-allowed-lower"),
	})
	assert.Equal(t, http.StatusCreated, code)
}

func TestCreateSubscription_PlanValidationConfigLookupFailure(t *testing.T) {
	mock := NewMockStorage()
	db := &failingConfigStorage{MockStorage: mock}
	server := createTestAPIServerWithDB(db)
	attachTestEventHub(server, &mockEventHub{}, "test-gateway")
	cfg := seedAPIForAPIKeyHandlerTests(t, server, "subscription-api")
	seedSubscriptionPlan(t, server, "plan-a", models.SubscriptionPlanStatusActive)
	db.failConfigID = cfg.UUID

	// Resolve by handle, so only the plan-validation config lookup hits the failure.
	code, body := postSubscription(t, server, "application/json", mustJSON(t, api.SubscriptionCreateRequest{
		ApiId: "subscription-api", SubscriptionToken: "token-cfg", SubscriptionPlanId: ptr("plan-a"),
	}))
	require.Equal(t, http.StatusInternalServerError, code)
	assert.Equal(t, "Failed to validate subscription plan", errorMessage(t, body))
}

// --- Issue point 2: PUT updates plan and token in place ---

func TestUpdateSubscription_ChangesPlanInPlace(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)
	seedSubscriptionPlan(t, server, "plan-a", models.SubscriptionPlanStatusActive)
	seedSubscriptionPlan(t, server, "plan-b", models.SubscriptionPlanStatusActive)

	code, _ := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId: cfg.UUID, SubscriptionToken: "token-plan", ExternalRefId: ptr("sub-plan"), SubscriptionPlanId: ptr("plan-a"),
	})
	require.Equal(t, http.StatusCreated, code)

	// No token in the request: a plan change no longer needs the original token.
	code, resp := updateSubscriptionViaHandler(t, server, "sub-plan", `{"subscriptionPlanId":"plan-b"}`)
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, resp.SubscriptionPlanId)
	assert.Equal(t, "plan-b", *resp.SubscriptionPlanId)
	assert.Equal(t, "sub-plan", *resp.Id, "the subscription keeps its ID")
	assert.Nil(t, resp.SubscriptionToken)

	stored, err := server.db.GetSubscriptionByID("sub-plan", "")
	require.NoError(t, err)
	require.NotNil(t, stored.SubscriptionPlanID)
	assert.Equal(t, "plan-b", *stored.SubscriptionPlanID)
	assert.Equal(t, models.SubscriptionStatusActive, stored.Status)
	assert.Equal(t, "token-plan", stored.SubscriptionToken, "the token is untouched by a plan change")
}

func TestUpdateSubscription_PlanOmittedKeptNullOrEmptyCleared(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)
	seedSubscriptionPlan(t, server, "plan-a", models.SubscriptionPlanStatusActive)

	create := func(ref string) {
		code, _ := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
			ApiId: cfg.UUID, SubscriptionToken: "token-" + ref, ExternalRefId: ptr(ref), SubscriptionPlanId: ptr("plan-a"),
		})
		require.Equal(t, http.StatusCreated, code)
	}

	// Status-only update leaves the plan as it was.
	create("sub-omit")
	code, resp := updateSubscriptionViaHandler(t, server, "sub-omit", `{"status":"INACTIVE"}`)
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, resp.SubscriptionPlanId)
	assert.Equal(t, "plan-a", *resp.SubscriptionPlanId)
	assert.Equal(t, api.SubscriptionResponseStatus("INACTIVE"), *resp.Status)

	for name, body := range map[string]string{
		"explicit null": `{"subscriptionPlanId":null}`,
		"empty string":  `{"subscriptionPlanId":""}`,
		"blank string":  `{"subscriptionPlanId":"   "}`,
	} {
		t.Run(name, func(t *testing.T) {
			ref := "sub-clear-" + strings.ReplaceAll(name, " ", "-")
			create(ref)
			code, resp := updateSubscriptionViaHandler(t, server, ref, body)
			require.Equal(t, http.StatusOK, code)
			assert.Nil(t, resp.SubscriptionPlanId)
			stored, err := server.db.GetSubscriptionByID(ref, "")
			require.NoError(t, err)
			assert.Nil(t, stored.SubscriptionPlanID)
		})
	}
}

func TestUpdateSubscription_YAMLBody(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)
	seedSubscriptionPlan(t, server, "plan-a", models.SubscriptionPlanStatusActive)

	code, _ := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId: cfg.UUID, SubscriptionToken: "token-yaml", ExternalRefId: ptr("sub-yaml"), SubscriptionPlanId: ptr("plan-a"),
	})
	require.Equal(t, http.StatusCreated, code)

	// A YAML body without the plan key keeps the plan.
	code, _ = putSubscription(t, server, "sub-yaml", "application/yaml", []byte("status: INACTIVE\n"))
	require.Equal(t, http.StatusOK, code)
	stored, err := server.db.GetSubscriptionByID("sub-yaml", "")
	require.NoError(t, err)
	require.NotNil(t, stored.SubscriptionPlanID)
	assert.Equal(t, models.SubscriptionStatusInactive, stored.Status)

	// An explicit YAML null removes it, the same as JSON.
	code, _ = putSubscription(t, server, "sub-yaml", "application/yaml", []byte("subscriptionPlanId: null\n"))
	require.Equal(t, http.StatusOK, code)
	stored, err = server.db.GetSubscriptionByID("sub-yaml", "")
	require.NoError(t, err)
	assert.Nil(t, stored.SubscriptionPlanID)
}

func TestUpdateSubscription_RejectsInvalidPlan(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)
	seedSubscriptionPlan(t, server, "plan-a", models.SubscriptionPlanStatusActive)
	seedSubscriptionPlan(t, server, "plan-inactive", models.SubscriptionPlanStatusInactive)

	code, _ := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId: cfg.UUID, SubscriptionToken: "token-bad-plan", ExternalRefId: ptr("sub-bad-plan"), SubscriptionPlanId: ptr("plan-a"),
	})
	require.Equal(t, http.StatusCreated, code)

	for body, wantMsg := range map[string]string{
		`{"subscriptionPlanId":"does-not-exist"}`:                                 "not found",
		`{"subscriptionPlanId":"plan-inactive"}`:                                  "not active",
		`{"subscriptionPlanId":"does-not-exist","status":"REVOKED"}`:              "not found",
		`{"subscriptionPlanId":"does-not-exist","subscriptionToken":"token-new"}`: "not found",
	} {
		code, respBody := putSubscription(t, server, "sub-bad-plan", "application/json", []byte(body))
		require.Equal(t, http.StatusBadRequest, code, body)
		assert.Contains(t, errorMessage(t, respBody), wantMsg, body)
	}

	// A rejected update changes nothing, including fields that were valid in the same request.
	stored, err := server.db.GetSubscriptionByID("sub-bad-plan", "")
	require.NoError(t, err)
	assert.Equal(t, "plan-a", *stored.SubscriptionPlanID)
	assert.Equal(t, models.SubscriptionStatusActive, stored.Status)
	assert.Equal(t, "token-bad-plan", stored.SubscriptionToken)
}

func TestUpdateSubscription_RejectsPlanNotEnabledForAPI(t *testing.T) {
	server, cfg := newRestrictedPlanTestServer(t)
	seedNamedSubscriptionPlan(t, server, "plan-allowed", "Allowed", models.SubscriptionPlanStatusActive)
	seedNamedSubscriptionPlan(t, server, "plan-other", "Other", models.SubscriptionPlanStatusActive)

	code, _ := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId: cfg.UUID, SubscriptionToken: "token-restricted", ExternalRefId: ptr("sub-restricted"), SubscriptionPlanId: ptr("plan-allowed"),
	})
	require.Equal(t, http.StatusCreated, code)

	code, body := putSubscription(t, server, "sub-restricted", "application/json", []byte(`{"subscriptionPlanId":"plan-other"}`))
	require.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, errorMessage(t, body), "not enabled for this API")
}

func TestUpdateSubscription_UnchangedPlanIsNotRevalidated(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)
	seedSubscriptionPlan(t, server, "plan-a", models.SubscriptionPlanStatusActive)

	code, _ := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId: cfg.UUID, SubscriptionToken: "token-same", ExternalRefId: ptr("sub-same"), SubscriptionPlanId: ptr("plan-a"),
	})
	require.Equal(t, http.StatusCreated, code)

	// The plan is deactivated after the subscription was created on it.
	plan, err := server.db.GetSubscriptionPlanByID("plan-a", "")
	require.NoError(t, err)
	plan.Status = models.SubscriptionPlanStatusInactive

	// Resending the same plan with an unrelated change must not be blocked by the deactivation.
	code, resp := updateSubscriptionViaHandler(t, server, "sub-same", `{"subscriptionPlanId":"plan-a","status":"INACTIVE"}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "plan-a", *resp.SubscriptionPlanId)
	assert.Equal(t, api.SubscriptionResponseStatus("INACTIVE"), *resp.Status)
}

func TestUpdateSubscription_PlanValidationConfigLookupFailure(t *testing.T) {
	mock := NewMockStorage()
	db := &failingConfigStorage{MockStorage: mock}
	server := createTestAPIServerWithDB(db)
	attachTestEventHub(server, &mockEventHub{}, "test-gateway")
	cfg := seedAPIForAPIKeyHandlerTests(t, server, "subscription-api")
	seedSubscriptionPlan(t, server, "plan-a", models.SubscriptionPlanStatusActive)
	seedSubscriptionPlan(t, server, "plan-b", models.SubscriptionPlanStatusActive)

	code, _ := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId: cfg.UUID, SubscriptionToken: "token-cfg", ExternalRefId: ptr("sub-cfg"), SubscriptionPlanId: ptr("plan-a"),
	})
	require.Equal(t, http.StatusCreated, code)

	db.failConfigID = cfg.UUID
	code, body := putSubscription(t, server, "sub-cfg", "application/json", []byte(`{"subscriptionPlanId":"plan-b"}`))
	require.Equal(t, http.StatusInternalServerError, code)
	assert.Equal(t, "Failed to validate subscription plan", errorMessage(t, body))
}

func TestUpdateSubscription_RotatesToken(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)
	seedSubscriptionPlan(t, server, "plan-a", models.SubscriptionPlanStatusActive)

	code, _ := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId: cfg.UUID, SubscriptionToken: "token-old", ExternalRefId: ptr("sub-rotate"), SubscriptionPlanId: ptr("plan-a"),
	})
	require.Equal(t, http.StatusCreated, code)

	code, respBody := putSubscription(t, server, "sub-rotate", "application/json", []byte(`{"subscriptionToken":"  token-new  "}`))
	require.Equal(t, http.StatusOK, code)
	assert.NotContains(t, string(respBody), "token-new", "the token must never be returned on update")
	var resp api.SubscriptionResponse
	require.NoError(t, json.Unmarshal(respBody, &resp))
	assert.Equal(t, "sub-rotate", *resp.Id, "rotation keeps the subscription ID")
	assert.Equal(t, "plan-a", *resp.SubscriptionPlanId, "rotation keeps the plan")

	stored, err := server.db.GetSubscriptionByID("sub-rotate", "")
	require.NoError(t, err)
	assert.Equal(t, "token-new", stored.SubscriptionToken, "the new token is trimmed and stored")
}

func TestUpdateSubscription_RejectsEmptyToken(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)

	code, _ := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId: cfg.UUID, SubscriptionToken: "token-keep", ExternalRefId: ptr("sub-empty-token"),
	})
	require.Equal(t, http.StatusCreated, code)

	for _, body := range []string{`{"subscriptionToken":""}`, `{"subscriptionToken":"   "}`} {
		code, respBody := putSubscription(t, server, "sub-empty-token", "application/json", []byte(body))
		require.Equal(t, http.StatusBadRequest, code, body)
		assert.Contains(t, errorMessage(t, respBody), "must not be empty")
	}
	stored, err := server.db.GetSubscriptionByID("sub-empty-token", "")
	require.NoError(t, err)
	assert.Equal(t, "token-keep", stored.SubscriptionToken)
}

func TestUpdateSubscription_AllFieldsAtOnce(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)
	seedSubscriptionPlan(t, server, "plan-a", models.SubscriptionPlanStatusActive)
	seedSubscriptionPlan(t, server, "plan-b", models.SubscriptionPlanStatusActive)

	code, _ := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId: cfg.UUID, SubscriptionToken: "token-all-old", ExternalRefId: ptr("sub-all"), SubscriptionPlanId: ptr("plan-a"),
	})
	require.Equal(t, http.StatusCreated, code)

	code, resp := updateSubscriptionViaHandler(t, server, "sub-all",
		`{"status":"REVOKED","subscriptionPlanId":"plan-b","subscriptionToken":"token-all-new"}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, api.SubscriptionResponseStatus("REVOKED"), *resp.Status)
	assert.Equal(t, "plan-b", *resp.SubscriptionPlanId)

	stored, err := server.db.GetSubscriptionByID("sub-all", "")
	require.NoError(t, err)
	assert.Equal(t, "token-all-new", stored.SubscriptionToken)
}

func TestUpdateSubscription_StatusOnlyKeepsPlanAndToken(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)
	seedSubscriptionPlan(t, server, "plan-a", models.SubscriptionPlanStatusActive)

	code, _ := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId: cfg.UUID, SubscriptionToken: "token-status", ExternalRefId: ptr("sub-status"), SubscriptionPlanId: ptr("plan-a"),
	})
	require.Equal(t, http.StatusCreated, code)

	for _, status := range []string{"INACTIVE", "REVOKED", "ACTIVE"} {
		code, resp := updateSubscriptionViaHandler(t, server, "sub-status", fmt.Sprintf(`{"status":%q}`, status))
		require.Equal(t, http.StatusOK, code)
		assert.Equal(t, api.SubscriptionResponseStatus(status), *resp.Status)
	}
	stored, err := server.db.GetSubscriptionByID("sub-status", "")
	require.NoError(t, err)
	assert.Equal(t, "plan-a", *stored.SubscriptionPlanID)
	assert.Equal(t, "token-status", stored.SubscriptionToken)
}

func TestUpdateSubscription_EmptyBodyObjectChangesNothing(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)

	code, _ := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId: cfg.UUID, SubscriptionToken: "token-noop", ExternalRefId: ptr("sub-noop"),
	})
	require.Equal(t, http.StatusCreated, code)

	code, resp := updateSubscriptionViaHandler(t, server, "sub-noop", `{}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, api.SubscriptionResponseStatus("ACTIVE"), *resp.Status)
	stored, err := server.db.GetSubscriptionByID("sub-noop", "")
	require.NoError(t, err)
	assert.Equal(t, "token-noop", stored.SubscriptionToken)
}

func TestUpdateSubscription_RequestValidation(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)

	code, _ := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId: cfg.UUID, SubscriptionToken: "token-validate", ExternalRefId: ptr("sub-validate"),
	})
	require.Equal(t, http.StatusCreated, code)

	tests := []struct {
		name     string
		body     string
		wantCode int
		wantMsg  string
	}{
		{"malformed JSON", `{"status":`, http.StatusBadRequest, "Invalid request body"},
		{"empty body", ``, http.StatusBadRequest, "Invalid request body"},
		{"invalid status", `{"status":"PAUSED"}`, http.StatusBadRequest, "invalid status"},
		{"too large", fmt.Sprintf(`{"subscriptionToken":%q}`, strings.Repeat("x", maxSubscriptionRequestBodyBytes)),
			http.StatusRequestEntityTooLarge, "Request body too large"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, body := putSubscription(t, server, "sub-validate", "application/json", []byte(tt.body))
			require.Equal(t, tt.wantCode, code)
			assert.Contains(t, errorMessage(t, body), tt.wantMsg)
		})
	}
	stored, err := server.db.GetSubscriptionByID("sub-validate", "")
	require.NoError(t, err)
	assert.Equal(t, models.SubscriptionStatusActive, stored.Status)
	assert.Equal(t, "token-validate", stored.SubscriptionToken)
}

func TestUpdateSubscription_TokenConflict(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)

	code, _ := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId: cfg.UUID, SubscriptionToken: "token-conflict", ExternalRefId: ptr("sub-conflict"),
	})
	require.Equal(t, http.StatusCreated, code)

	server.db.(*MockStorage).updateErr = fmt.Errorf("%w: subscription token already exists for this API", storage.ErrConflict)
	code, body := putSubscription(t, server, "sub-conflict", "application/json", []byte(`{"subscriptionToken":"token-taken"}`))
	assertGenericSubscriptionConflict(t, code, body)
}

func TestUpdateSubscription_StorageErrors(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(m *MockStorage)
		wantCode int
		wantMsg  string
	}{
		{"lookup fails", func(m *MockStorage) { m.getErr = errors.New("database unavailable") },
			http.StatusInternalServerError, "Failed to get subscription"},
		{"deleted before the write", func(m *MockStorage) { m.updateErr = storage.ErrNotFound },
			http.StatusNotFound, "Subscription not found"},
		{"write fails", func(m *MockStorage) { m.updateErr = errors.New("database unavailable") },
			http.StatusInternalServerError, "Failed to update subscription"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, cfg := newSubscriptionTestServer(t)
			code, _ := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
				ApiId: cfg.UUID, SubscriptionToken: "token-store", ExternalRefId: ptr("sub-store"),
			})
			require.Equal(t, http.StatusCreated, code)

			tt.setup(server.db.(*MockStorage))
			code, body := putSubscription(t, server, "sub-store", "application/json", []byte(`{"status":"INACTIVE"}`))
			require.Equal(t, tt.wantCode, code)
			assert.Equal(t, tt.wantMsg, errorMessage(t, body))
		})
	}
}

func TestUpdateSubscription_NotFound(t *testing.T) {
	server, _ := newSubscriptionTestServer(t)

	code, _ := updateSubscriptionViaHandler(t, server, "missing", `{"status":"INACTIVE"}`)
	assert.Equal(t, http.StatusNotFound, code)
}

func TestUpdateSubscription_PublishesUpdateEvent(t *testing.T) {
	server, cfg, hub := newSubscriptionTestServerWithHub(t)

	code, _ := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId: cfg.UUID, SubscriptionToken: "token-upd-event", ExternalRefId: ptr("sub-upd-event"),
	})
	require.Equal(t, http.StatusCreated, code)

	w, r := createTestContextWithHeader("PUT", "/subscriptions/sub-upd-event", []byte(`{"subscriptionToken":"token-upd-event-2"}`),
		map[string]string{"Content-Type": "application/json"})
	r = withCorrelationID(r, "corr-id-update-subscription")
	server.UpdateSubscription(w, r, "sub-upd-event")
	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, hub.publishedEvents, 2)
	assert.Equal(t, eventhub.EventTypeSubscription, hub.publishedEvents[1].event.EventType)
	assert.Equal(t, "UPDATE", hub.publishedEvents[1].event.Action)
	assert.Equal(t, "sub-upd-event", hub.publishedEvents[1].event.EntityID)
	assert.Equal(t, "corr-id-update-subscription", hub.publishedEvents[1].event.EventID)
}

// --- GET and DELETE by the caller's reference ID ---

func TestDeleteSubscription_ByExternalRefID(t *testing.T) {
	server, cfg := newSubscriptionTestServer(t)

	code, _ := createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId: cfg.UUID, SubscriptionToken: "token-delete", ExternalRefId: ptr("sub-delete"),
	})
	require.Equal(t, http.StatusCreated, code)

	w, r := createTestContext("DELETE", "/subscriptions/sub-delete", nil)
	server.DeleteSubscription(w, r, "sub-delete")
	require.Equal(t, http.StatusNoContent, w.Code)

	code, _ = getSubscriptionViaHandler(t, server, "sub-delete")
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = updateSubscriptionViaHandler(t, server, "sub-delete", `{"status":"ACTIVE"}`)
	assert.Equal(t, http.StatusNotFound, code)

	// The reference is free again once the subscription is gone.
	code, _ = createSubscriptionViaHandler(t, server, api.SubscriptionCreateRequest{
		ApiId: cfg.UUID, SubscriptionToken: "token-delete-2", ExternalRefId: ptr("sub-delete"),
	})
	assert.Equal(t, http.StatusCreated, code)
}

func mustJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}
