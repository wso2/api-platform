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

package service

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

type mockSubAPIRepo struct {
	repository.APIRepository
	apis map[string]*model.API // keyed by uuid
}

func (m *mockSubAPIRepo) GetAPIByUUID(apiUUID, orgUUID string) (*model.API, error) {
	if api, ok := m.apis[apiUUID]; ok {
		return api, nil
	}
	return nil, nil
}

func (m *mockSubAPIRepo) GetAPIGatewaysWithDetails(apiUUID, orgUUID string) ([]*model.APIGatewayWithDetails, error) {
	return nil, nil
}

type mockSubArtifactRepo struct {
	repository.ArtifactRepository
	artifacts map[string]*model.Artifact    // keyed by uuid
	metadata  map[string]*model.APIMetadata // keyed by handle:kind
}

func (m *mockSubArtifactRepo) GetAPIMetadataByHandleAndKind(handle, kind, orgUUID string) (*model.APIMetadata, error) {
	key := handle + ":" + kind
	if meta, ok := m.metadata[key]; ok {
		return meta, nil
	}
	return nil, nil
}

func (m *mockSubArtifactRepo) GetByUUID(uuid, orgUUID string) (*model.Artifact, error) {
	if art, ok := m.artifacts[uuid]; ok {
		return art, nil
	}
	return nil, nil
}

type mockSubSubscriptionRepo struct {
	repository.SubscriptionRepository
	subs    map[string]*model.Subscription // keyed by uuid
	created *model.Subscription
	updated *model.Subscription
}

func (m *mockSubSubscriptionRepo) ExistsByAPIAndSubscriber(apiUUID, subscriberID, orgUUID string) (bool, error) {
	return false, nil
}

func (m *mockSubSubscriptionRepo) Create(sub *model.Subscription) error {
	sub.UUID = "generated-sub-uuid"
	m.created = sub
	if m.subs == nil {
		m.subs = make(map[string]*model.Subscription)
	}
	m.subs[sub.UUID] = sub
	return nil
}

func (m *mockSubSubscriptionRepo) GetByID(id, orgUUID string) (*model.Subscription, error) {
	if sub, ok := m.subs[id]; ok {
		return sub, nil
	}
	return nil, apperror.SubscriptionNotFound.New()
}

func (m *mockSubSubscriptionRepo) Update(sub *model.Subscription) error {
	m.updated = sub
	m.subs[sub.UUID] = sub
	return nil
}

type mockSubPlanRepo struct {
	repository.SubscriptionPlanRepository
	plans map[string]*model.SubscriptionPlan // keyed by handle
}

func (m *mockSubPlanRepo) GetByHandleAndOrg(handle, orgUUID string) (*model.SubscriptionPlan, error) {
	if p, ok := m.plans[handle]; ok {
		return p, nil
	}
	return nil, sql.ErrNoRows
}

type mockSubGraphQLRepo struct {
	repository.GraphQLAPIRepository
	apis map[string]*model.GraphQLAPI // keyed by uuid
}

func (m *mockSubGraphQLRepo) GetByUUID(uuid, orgUUID string) (*model.GraphQLAPI, error) {
	if a, ok := m.apis[uuid]; ok {
		return a, nil
	}
	return nil, nil
}

type mockSubAuditRepo struct {
	repository.AuditRepository
}

func (m *mockSubAuditRepo) Record(action, entityUUID, entityType, orgUUID, actor string) error {
	return nil
}

func setupSubscriptionTestEnv() (*SubscriptionService, *mockSubAPIRepo, *mockSubArtifactRepo, *mockSubSubscriptionRepo, *mockSubPlanRepo, *mockSubGraphQLRepo) {
	apiRepo := &mockSubAPIRepo{apis: make(map[string]*model.API)}
	artifactRepo := &mockSubArtifactRepo{
		artifacts: make(map[string]*model.Artifact),
		metadata:  make(map[string]*model.APIMetadata),
	}
	subRepo := &mockSubSubscriptionRepo{subs: make(map[string]*model.Subscription)}
	planRepo := &mockSubPlanRepo{
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
			"unlimited-internal": {
				UUID:   "uuid-plan-unlimited",
				Handle: "unlimited-internal",
				Name:   "Unlimited Internal",
				Status: model.SubscriptionPlanStatusActive,
			},
			"bronze-inactive": {
				UUID:   "uuid-plan-bronze",
				Handle: "bronze-inactive",
				Name:   "Bronze Inactive",
				Status: model.SubscriptionPlanStatusInactive,
			},
		},
	}
	gqlRepo := &mockSubGraphQLRepo{apis: make(map[string]*model.GraphQLAPI)}
	auditRepo := &mockSubAuditRepo{}

	svc := NewSubscriptionService(apiRepo, artifactRepo, subRepo, planRepo, nil, nil, auditRepo, nil)
	svc.SetGraphQLAPIRepository(gqlRepo)

	return svc, apiRepo, artifactRepo, subRepo, planRepo, gqlRepo
}

func TestCreateSubscription_APIWithSubscriptionPlans(t *testing.T) {
	svc, apiRepo, artifactRepo, _, _, _ := setupSubscriptionTestEnv()
	orgUUID := "test-org"

	apiUUID := "uuid-rest-api-1"
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

	t.Run("listed plan succeeds", func(t *testing.T) {
		planID := "gold"
		sub, err := svc.CreateSubscription(apiHandle, "RestApi", orgUUID, "user-1", nil, &planID, "", "ACTIVE", "actor-1")
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if sub.SubscriptionPlanID == nil || *sub.SubscriptionPlanID != "uuid-plan-gold" {
			t.Fatalf("expected resolved plan UUID 'uuid-plan-gold', got %v", sub.SubscriptionPlanID)
		}
	})

	t.Run("listed plan case-insensitive match succeeds", func(t *testing.T) {
		planID := "Gold"
		// If handle lookup in plan repo is "gold", but user passed "gold"
		// let's test lowercase vs uppercase
		planID = "gold"
		sub, err := svc.CreateSubscription(apiHandle, "RestApi", orgUUID, "user-case", nil, &planID, "", "ACTIVE", "actor-1")
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if sub.SubscriptionPlanID == nil || *sub.SubscriptionPlanID != "uuid-plan-gold" {
			t.Fatalf("expected resolved plan UUID 'uuid-plan-gold', got %v", sub.SubscriptionPlanID)
		}
	})

	t.Run("unlisted active plan rejected with 400", func(t *testing.T) {
		planID := "silver"
		_, err := svc.CreateSubscription(apiHandle, "RestApi", orgUUID, "user-2", nil, &planID, "", "ACTIVE", "actor-1")
		if err == nil {
			t.Fatal("expected error for unlisted plan, got nil")
		}
		if !apperror.ValidationFailed.Is(err) {
			t.Fatalf("expected ValidationFailed error, got %v", err)
		}
		if !strings.Contains(err.Error(), `Subscription plan "silver" is not enabled for this API`) {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("unlisted internal plan rejected with 400", func(t *testing.T) {
		planID := "unlimited-internal"
		_, err := svc.CreateSubscription(apiHandle, "RestApi", orgUUID, "user-3", nil, &planID, "", "ACTIVE", "actor-1")
		if err == nil {
			t.Fatal("expected error for unlisted internal plan, got nil")
		}
		if !apperror.ValidationFailed.Is(err) {
			t.Fatalf("expected ValidationFailed error, got %v", err)
		}
		if !strings.Contains(err.Error(), `Subscription plan "unlimited-internal" is not enabled for this API`) {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("no plan provided rejected with 400", func(t *testing.T) {
		_, err := svc.CreateSubscription(apiHandle, "RestApi", orgUUID, "user-4", nil, nil, "", "ACTIVE", "actor-1")
		if err == nil {
			t.Fatal("expected error for missing plan on API with subscriptionPlans, got nil")
		}
		if !apperror.ValidationFailed.Is(err) {
			t.Fatalf("expected ValidationFailed error, got %v", err)
		}
		if !strings.Contains(err.Error(), "Subscription plan is required for this API") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("empty string plan rejected with 400", func(t *testing.T) {
		emptyPlan := "   "
		_, err := svc.CreateSubscription(apiHandle, "RestApi", orgUUID, "user-5", nil, &emptyPlan, "", "ACTIVE", "actor-1")
		if err == nil {
			t.Fatal("expected error for empty plan on API with subscriptionPlans, got nil")
		}
		if !apperror.ValidationFailed.Is(err) {
			t.Fatalf("expected ValidationFailed error, got %v", err)
		}
		if !strings.Contains(err.Error(), "Subscription plan is required for this API") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("inactive plan rejected with 400", func(t *testing.T) {
		planID := "bronze-inactive"
		_, err := svc.CreateSubscription(apiHandle, "RestApi", orgUUID, "user-6", nil, &planID, "", "ACTIVE", "actor-1")
		if err == nil {
			t.Fatal("expected error for inactive plan, got nil")
		}
		if !apperror.ValidationFailed.Is(err) {
			t.Fatalf("expected ValidationFailed error, got %v", err)
		}
		if !strings.Contains(err.Error(), "Subscription plan is not active") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("unknown non-existent plan rejected with 404", func(t *testing.T) {
		planID := "does-not-exist"
		_, err := svc.CreateSubscription(apiHandle, "RestApi", orgUUID, "user-7", nil, &planID, "", "ACTIVE", "actor-1")
		if err == nil {
			t.Fatal("expected error for non-existent plan, got nil")
		}
		if !apperror.SubscriptionPlanNotFound.Is(err) {
			t.Fatalf("expected SubscriptionPlanNotFound error, got %v", err)
		}
	})
}

func TestCreateSubscription_APIWithoutSubscriptionPlans(t *testing.T) {
	svc, apiRepo, artifactRepo, _, _, _ := setupSubscriptionTestEnv()
	orgUUID := "test-org"

	apiUUID := "uuid-rest-api-open"
	apiHandle := "open-api"
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
		ID:            apiUUID,
		Handle:        apiHandle,
		Kind:          constants.RestApi,
		Configuration: model.RestAPIConfig{
			// No SubscriptionPlans configured
		},
	}

	t.Run("no plan provided succeeds", func(t *testing.T) {
		sub, err := svc.CreateSubscription(apiHandle, "RestApi", orgUUID, "user-1", nil, nil, "", "ACTIVE", "actor-1")
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if sub.SubscriptionPlanID != nil {
			t.Fatalf("expected nil plan UUID, got %v", sub.SubscriptionPlanID)
		}
	})

	t.Run("any active plan in org succeeds", func(t *testing.T) {
		planID := "silver"
		sub, err := svc.CreateSubscription(apiHandle, "RestApi", orgUUID, "user-2", nil, &planID, "", "ACTIVE", "actor-1")
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if sub.SubscriptionPlanID == nil || *sub.SubscriptionPlanID != "uuid-plan-silver" {
			t.Fatalf("expected 'uuid-plan-silver', got %v", sub.SubscriptionPlanID)
		}
	})

	t.Run("inactive plan rejected", func(t *testing.T) {
		planID := "bronze-inactive"
		_, err := svc.CreateSubscription(apiHandle, "RestApi", orgUUID, "user-3", nil, &planID, "", "ACTIVE", "actor-1")
		if err == nil {
			t.Fatal("expected error for inactive plan, got nil")
		}
		if !apperror.ValidationFailed.Is(err) {
			t.Fatalf("expected ValidationFailed, got %v", err)
		}
	})
}

func TestCreateSubscription_GraphQLAPIWithSubscriptionPlans(t *testing.T) {
	svc, _, artifactRepo, _, _, gqlRepo := setupSubscriptionTestEnv()
	orgUUID := "test-org"

	gqlUUID := "uuid-gql-api-1"
	gqlHandle := "countries-graphql-api"
	artifactRepo.metadata[gqlHandle+":GraphQLApi"] = &model.APIMetadata{
		ID:     gqlUUID,
		Handle: gqlHandle,
		Kind:   constants.GraphQLApi,
	}
	artifactRepo.artifacts[gqlUUID] = &model.Artifact{
		UUID:   gqlUUID,
		Handle: gqlHandle,
		Type:   constants.GraphQLApi,
	}
	gqlRepo.apis[gqlUUID] = &model.GraphQLAPI{
		ID:     gqlUUID,
		Handle: gqlHandle,
		Configuration: model.GraphQLAPIConfig{
			SubscriptionPlans: []string{"gold"},
		},
	}

	t.Run("listed plan succeeds", func(t *testing.T) {
		planID := "gold"
		sub, err := svc.CreateSubscription(gqlHandle, "GraphQLApi", orgUUID, "user-gql-1", nil, &planID, "", "ACTIVE", "actor-1")
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if sub.SubscriptionPlanID == nil || *sub.SubscriptionPlanID != "uuid-plan-gold" {
			t.Fatalf("expected 'uuid-plan-gold', got %v", sub.SubscriptionPlanID)
		}
	})

	t.Run("unlisted plan fails", func(t *testing.T) {
		planID := "silver"
		_, err := svc.CreateSubscription(gqlHandle, "GraphQLApi", orgUUID, "user-gql-2", nil, &planID, "", "ACTIVE", "actor-1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !apperror.ValidationFailed.Is(err) {
			t.Fatalf("expected ValidationFailed, got %v", err)
		}
	})

	t.Run("no plan fails", func(t *testing.T) {
		_, err := svc.CreateSubscription(gqlHandle, "GraphQLApi", orgUUID, "user-gql-3", nil, nil, "", "ACTIVE", "actor-1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !apperror.ValidationFailed.Is(err) {
			t.Fatalf("expected ValidationFailed, got %v", err)
		}
	})
}

func TestChangePlan(t *testing.T) {
	svc, apiRepo, artifactRepo, subRepo, _, _ := setupSubscriptionTestEnv()
	orgUUID := "test-org"

	apiUUID := "uuid-rest-api-change"
	apiHandle := "change-plan-api"
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

	subID := "sub-existing-1"
	existingPlan := "uuid-plan-gold"
	subRepo.subs[subID] = &model.Subscription{
		UUID:               subID,
		ArtifactUUID:       apiUUID,
		SubscriberID:       "subscriber-alice",
		SubscriptionPlanID: &existingPlan,
		OrganizationUUID:   orgUUID,
		Status:             model.SubscriptionStatusActive,
	}

	t.Run("switch to listed plan succeeds", func(t *testing.T) {
		sub, err := svc.ChangePlan(subID, orgUUID, "subscriber-alice", "gold")
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if sub.SubscriptionPlanID == nil || *sub.SubscriptionPlanID != "uuid-plan-gold" {
			t.Fatalf("expected 'uuid-plan-gold', got %v", sub.SubscriptionPlanID)
		}
	})

	t.Run("switch to unlisted plan fails with 400", func(t *testing.T) {
		_, err := svc.ChangePlan(subID, orgUUID, "subscriber-alice", "silver")
		if err == nil {
			t.Fatal("expected error for unlisted plan, got nil")
		}
		if !apperror.ValidationFailed.Is(err) {
			t.Fatalf("expected ValidationFailed, got %v", err)
		}
		if !strings.Contains(err.Error(), `Subscription plan "silver" is not enabled for this API`) {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("switch to empty plan fails with 400", func(t *testing.T) {
		_, err := svc.ChangePlan(subID, orgUUID, "subscriber-alice", "")
		if err == nil {
			t.Fatal("expected error for empty plan, got nil")
		}
		if !apperror.ValidationFailed.Is(err) {
			t.Fatalf("expected ValidationFailed, got %v", err)
		}
	})

	t.Run("switch to inactive plan fails with 400", func(t *testing.T) {
		_, err := svc.ChangePlan(subID, orgUUID, "subscriber-alice", "bronze-inactive")
		if err == nil {
			t.Fatal("expected error for inactive plan, got nil")
		}
		if !apperror.ValidationFailed.Is(err) {
			t.Fatalf("expected ValidationFailed, got %v", err)
		}
	})

	t.Run("wrong subscriber fails with 403", func(t *testing.T) {
		_, err := svc.ChangePlan(subID, orgUUID, "subscriber-other", "gold")
		if err == nil {
			t.Fatal("expected forbidden error, got nil")
		}
		if !apperror.SubscriptionForbidden.Is(err) {
			t.Fatalf("expected SubscriptionForbidden, got %v", err)
		}
	})
}
