/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

type subHandlerAPIRepo struct {
	repository.APIRepository
	apis map[string]*model.API
}

func (m *subHandlerAPIRepo) GetAPIByUUID(apiUUID, orgUUID string) (*model.API, error) {
	if api, ok := m.apis[apiUUID]; ok {
		return api, nil
	}
	return nil, nil
}

func (m *subHandlerAPIRepo) GetAPIGatewaysWithDetails(apiUUID, orgUUID string) ([]*model.APIGatewayWithDetails, error) {
	return nil, nil
}

type subHandlerArtifactRepo struct {
	repository.ArtifactRepository
	artifacts map[string]*model.Artifact
	metadata  map[string]*model.APIMetadata
}

func (m *subHandlerArtifactRepo) GetAPIMetadataByHandleAndKind(handle, kind, orgUUID string) (*model.APIMetadata, error) {
	if meta, ok := m.metadata[handle+":"+kind]; ok {
		return meta, nil
	}
	return nil, nil
}

func (m *subHandlerArtifactRepo) GetByUUID(uuid, orgUUID string) (*model.Artifact, error) {
	if art, ok := m.artifacts[uuid]; ok {
		return art, nil
	}
	return nil, nil
}

type subHandlerSubRepo struct {
	repository.SubscriptionRepository
	subs map[string]*model.Subscription
}

func (m *subHandlerSubRepo) ExistsByAPIAndSubscriber(apiUUID, subscriberID, orgUUID string) (bool, error) {
	return false, nil
}

func (m *subHandlerSubRepo) Create(sub *model.Subscription) error {
	sub.UUID = "sub-uuid-1"
	if m.subs == nil {
		m.subs = make(map[string]*model.Subscription)
	}
	m.subs[sub.UUID] = sub
	return nil
}

func (m *subHandlerSubRepo) GetByID(id, orgUUID string) (*model.Subscription, error) {
	if sub, ok := m.subs[id]; ok {
		return sub, nil
	}
	return nil, sql.ErrNoRows
}

type subHandlerPlanRepo struct {
	repository.SubscriptionPlanRepository
	plans map[string]*model.SubscriptionPlan // keyed by handle
}

func (m *subHandlerPlanRepo) GetByHandleAndOrg(handle, orgUUID string) (*model.SubscriptionPlan, error) {
	if p, ok := m.plans[handle]; ok {
		return p, nil
	}
	return nil, sql.ErrNoRows
}

func (m *subHandlerPlanRepo) GetByID(planID, orgUUID string) (*model.SubscriptionPlan, error) {
	for _, p := range m.plans {
		if p.UUID == planID {
			return p, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (m *subHandlerPlanRepo) GetHandlesByIDs(planUUIDs []string, orgUUID string) (map[string]string, error) {
	result := make(map[string]string)
	for _, id := range planUUIDs {
		for _, p := range m.plans {
			if p.UUID == id {
				result[id] = p.Handle
			}
		}
	}
	return result, nil
}

func (m *subHandlerPlanRepo) GetByIDs(planIDs []string, orgUUID string) (map[string]string, error) {
	result := make(map[string]string)
	for _, id := range planIDs {
		for _, p := range m.plans {
			if p.UUID == id {
				result[id] = p.Name
			}
		}
	}
	return result, nil
}

type subHandlerOrgRepo struct {
	repository.OrganizationRepository
}

func (m *subHandlerOrgRepo) GetOrganizationByUUID(uuid string) (*model.Organization, error) {
	return &model.Organization{
		ID:     uuid,
		Handle: "test-org",
	}, nil
}

type subHandlerAuditRepo struct {
	repository.AuditRepository
}

func (m *subHandlerAuditRepo) Record(action, entityUUID, entityType, orgUUID, actor string) error {
	return nil
}

type subHIdentityRepo struct {
	repository.UserIdentityMappingRepository
}

func (r subHIdentityRepo) GetOrCreateUUID(identity string) (string, error) {
	return "internal-" + identity, nil
}

func (r subHIdentityRepo) GetSubByUUID(uuid string) (string, bool, error) {
	return strings.TrimPrefix(uuid, "internal-"), true, nil
}

func (r subHIdentityRepo) GetSubsByUUIDs(uuids []string) (map[string]string, error) {
	out := map[string]string{}
	for _, u := range uuids {
		out[u] = strings.TrimPrefix(u, "internal-")
	}
	return out, nil
}

func setupSubscriptionHandlerTest(t *testing.T) http.Handler {
	apiRepo := &subHandlerAPIRepo{apis: make(map[string]*model.API)}
	artifactRepo := &subHandlerArtifactRepo{
		artifacts: make(map[string]*model.Artifact),
		metadata:  make(map[string]*model.APIMetadata),
	}
	subRepo := &subHandlerSubRepo{subs: make(map[string]*model.Subscription)}
	planRepo := &subHandlerPlanRepo{
		plans: map[string]*model.SubscriptionPlan{
			"gold": {
				UUID:   "uuid-plan-gold",
				Handle: "gold",
				Name:   "Gold Plan",
				Status: model.SubscriptionPlanStatusActive,
			},
			"silver": {
				UUID:   "uuid-plan-silver",
				Handle: "silver",
				Name:   "Silver Plan",
				Status: model.SubscriptionPlanStatusActive,
			},
		},
	}
	orgRepo := &subHandlerOrgRepo{}
	auditRepo := &subHandlerAuditRepo{}

	apiUUID := "uuid-plan-check-api"
	apiHandle := "plan-check-api"
	artifactRepo.metadata[apiHandle+":RestApi"] = &model.APIMetadata{
		ID:     apiUUID,
		Handle: apiHandle,
		Kind:   constants.RestApi,
	}
	artifactRepo.artifacts[apiUUID] = &model.Artifact{
		UUID:   apiUUID,
		Handle: apiHandle,
		Type:   constants.RestApi,
	}
	apiRepo.apis[apiUUID] = &model.API{
		ID:     apiUUID,
		Handle: apiHandle,
		Kind:   constants.RestApi,
		Configuration: model.RestAPIConfig{
			SubscriptionPlans: []string{"gold"},
		},
	}

	subSvc := service.NewSubscriptionService(apiRepo, artifactRepo, subRepo, planRepo, orgRepo, nil, auditRepo, nil)
	planSvc := service.NewSubscriptionPlanService(planRepo, nil, orgRepo, nil, auditRepo, nil)
	identitySvc := service.NewIdentityService(subHIdentityRepo{})

	handler := NewSubscriptionHandler(subSvc, planSvc, identitySvc, slog.Default())

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return mux
}

func doSubRequest(h http.Handler, method, target string, body any) *httptest.ResponseRecorder {
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, target, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	req = middleware.WithOrganization(req, "org-uuid-1")
	req = middleware.WithUserID(req, "user-actor-1")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestSubscriptionHandler_CreateSubscriptionValidation(t *testing.T) {
	app := setupSubscriptionHandlerTest(t)

	t.Run("listed plan returns 201", func(t *testing.T) {
		gold := "gold"
		rec := doSubRequest(app, "POST", "/api/v0.9/subscriptions", CreateSubscriptionRequest{
			ArtifactID:         "plan-check-api",
			Kind:               "RestApi",
			SubscriberID:       "user-1",
			SubscriptionPlanID: &gold,
		})
		require.Equal(t, http.StatusCreated, rec.Code)

		var resp map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "gold", resp["subscriptionPlanId"])
		assert.Equal(t, "Gold Plan", resp["subscriptionPlanName"])
	})

	t.Run("unlisted plan returns 400", func(t *testing.T) {
		silver := "silver"
		rec := doSubRequest(app, "POST", "/api/v0.9/subscriptions", CreateSubscriptionRequest{
			ArtifactID:         "plan-check-api",
			Kind:               "RestApi",
			SubscriberID:       "user-2",
			SubscriptionPlanID: &silver,
		})
		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), `Subscription plan \"silver\" is not enabled for this API`)
	})

	t.Run("no plan returns 400", func(t *testing.T) {
		rec := doSubRequest(app, "POST", "/api/v0.9/subscriptions", CreateSubscriptionRequest{
			ArtifactID:   "plan-check-api",
			Kind:         "RestApi",
			SubscriberID: "user-3",
		})
		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "Subscription plan is required for this API")
	})
}
