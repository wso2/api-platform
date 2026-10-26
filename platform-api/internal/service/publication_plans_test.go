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
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

type publicationPlanRepo struct {
	repository.SubscriptionPlanRepository
	plans map[string]*model.SubscriptionPlan
	err   error
}

func (f *publicationPlanRepo) GetByHandles(handles []string, _ string) (map[string]*model.SubscriptionPlan, error) {
	if f.err != nil {
		return nil, f.err
	}
	found := map[string]*model.SubscriptionPlan{}
	for _, h := range handles {
		if p, ok := f.plans[h]; ok {
			found[h] = p
		}
	}
	return found, nil
}

type planSyncPublisher struct {
	PortalPublisher
	err   error
	calls int
	got   []*model.SubscriptionPlan
}

func (f *planSyncPublisher) CreateMissingPlans(_ context.Context, _ *model.APIPortal, plans []*model.SubscriptionPlan) error {
	f.calls++
	f.got = plans
	return f.err
}

func newPublicationPlanService(repo *publicationPlanRepo, pub *planSyncPublisher) *PublicationService {
	return NewPublicationService(nil, nil, nil, repo, nil, pub, time.Minute, 0, nil)
}

func publicationTestPlans() *publicationPlanRepo {
	return &publicationPlanRepo{plans: map[string]*model.SubscriptionPlan{
		"gold":   {UUID: "uuid-gold", Handle: "gold", Name: "Gold", Status: model.SubscriptionPlanStatusActive},
		"silver": {UUID: "uuid-silver", Handle: "silver", Name: "Silver", Status: model.SubscriptionPlanStatusActive},
		"bronze": {UUID: "uuid-bronze", Handle: "bronze", Status: model.SubscriptionPlanStatusInactive},
	}}
}

func TestLoadActivePlans(t *testing.T) {
	svc := newPublicationPlanService(publicationTestPlans(), nil)

	t.Run("returns active plans keyed by handle", func(t *testing.T) {
		plans, err := svc.loadActivePlans([]string{"gold", "silver"}, "org")
		if err != nil || len(plans) != 2 || plans["gold"].UUID != "uuid-gold" {
			t.Fatalf("got %v, %v", plans, err)
		}
	})

	t.Run("rejects an unknown handle", func(t *testing.T) {
		_, err := svc.loadActivePlans([]string{"gold", "missing"}, "org")
		if !apperror.APIPublicationValidationFailed.Is(err) || !strings.Contains(err.Error(), "missing") {
			t.Fatalf("want a validation error naming the handle, got %v", err)
		}
	})

	t.Run("rejects an inactive plan", func(t *testing.T) {
		_, err := svc.loadActivePlans([]string{"gold", "bronze"}, "org")
		if !apperror.APIPublicationValidationFailed.Is(err) || !strings.Contains(err.Error(), "bronze") || !strings.Contains(err.Error(), "not active") {
			t.Fatalf("want a validation error naming the inactive plan, got %v", err)
		}
	})

	t.Run("unknown handle is reported before an inactive one", func(t *testing.T) {
		_, err := svc.loadActivePlans([]string{"bronze", "missing"}, "org")
		if !apperror.APIPublicationValidationFailed.Is(err) || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("want the not found error, got %v", err)
		}
	})

	t.Run("repository failure is not a validation error", func(t *testing.T) {
		repo := publicationTestPlans()
		repo.err = errors.New("db down")
		_, err := newPublicationPlanService(repo, nil).loadActivePlans([]string{"gold"}, "org")
		if err == nil || apperror.APIPublicationValidationFailed.Is(err) || !errors.Is(err, repo.err) {
			t.Fatalf("want the wrapped repository error, got %v", err)
		}
	})
}

func TestResolvePlanSelections(t *testing.T) {
	svc := newPublicationPlanService(publicationTestPlans(), nil)

	if got, err := svc.resolvePlanSelections(nil, "org"); err != nil || got != nil {
		t.Fatalf("no handles: got %v, %v", got, err)
	}
	got, err := svc.resolvePlanSelections([]string{"silver", "gold"}, "org")
	if err != nil || len(got) != 2 || got[0].UUID != "uuid-silver" || got[0].Handle != "silver" || got[1].UUID != "uuid-gold" || got[1].Handle != "gold" {
		t.Fatalf("want selections in handle order, got %v, %v", got, err)
	}
	if got[0].DisplayName != "Silver" || got[1].DisplayName != "Gold" {
		t.Fatalf("want each selection to carry the plan's name, got %v", got)
	}
	if _, err := svc.resolvePlanSelections([]string{"bronze"}, "org"); !apperror.APIPublicationValidationFailed.Is(err) {
		t.Fatalf("want a validation error for an inactive plan, got %v", err)
	}
}

func TestActivePlansInOrder(t *testing.T) {
	svc := newPublicationPlanService(publicationTestPlans(), nil)

	t.Run("no handles loads nothing", func(t *testing.T) {
		plans, err := svc.activePlansInOrder(nil, "org")
		if err != nil || plans != nil {
			t.Fatalf("got %v, %v", plans, err)
		}
	})

	t.Run("returns the selected plans in order", func(t *testing.T) {
		plans, err := svc.activePlansInOrder([]string{"silver", "gold"}, "org")
		if err != nil || len(plans) != 2 || plans[0].Handle != "silver" || plans[1].Handle != "gold" {
			t.Fatalf("got %v, %v", plans, err)
		}
	})

	t.Run("rejects an inactive plan", func(t *testing.T) {
		if _, err := svc.activePlansInOrder([]string{"bronze"}, "org"); !apperror.APIPublicationValidationFailed.Is(err) {
			t.Fatalf("want a validation error, got %v", err)
		}
	})
}

func TestCreateMissingPortalPlans(t *testing.T) {
	portal := &model.APIPortal{}
	plans := []*model.SubscriptionPlan{{Handle: "silver"}, {Handle: "gold"}}

	t.Run("no plans skips the portal", func(t *testing.T) {
		pub := &planSyncPublisher{}
		if err := newPublicationPlanService(publicationTestPlans(), pub).createMissingPortalPlans(context.Background(), portal, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pub.calls != 0 {
			t.Fatalf("want no portal call, got %d", pub.calls)
		}
	})

	t.Run("sends the selected plans in order", func(t *testing.T) {
		pub := &planSyncPublisher{}
		if err := newPublicationPlanService(publicationTestPlans(), pub).createMissingPortalPlans(context.Background(), portal, plans); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pub.calls != 1 || len(pub.got) != 2 || pub.got[0].Handle != "silver" || pub.got[1].Handle != "gold" {
			t.Fatalf("got %d calls, plans %v", pub.calls, pub.got)
		}
	})

	t.Run("portal failures map to conflict or unavailable", func(t *testing.T) {
		tests := []struct {
			name string
			err  error
			is   func(error) bool
		}{
			{"rejection", &PortalConflictError{Message: "rejected"}, apperror.APIPublicationPortalConflict.Is},
			{"unreachable", errors.New("connection refused"), apperror.APIPublicationPortalUnavailable.Is},
			{"deadline", context.DeadlineExceeded, apperror.APIPublicationPortalUnavailable.Is},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				pub := &planSyncPublisher{err: tc.err}
				err := newPublicationPlanService(publicationTestPlans(), pub).createMissingPortalPlans(context.Background(), portal, plans)
				if !tc.is(err) {
					t.Fatalf("got %v", err)
				}
			})
		}
	})
}

type publishFlowArtifactRepo struct{ repository.ArtifactRepository }

func (publishFlowArtifactRepo) GetAPIMetadataByHandleAndKind(_, _, _ string) (*model.APIMetadata, error) {
	return &model.APIMetadata{ID: "artifact-uuid"}, nil
}

type publishFlowPortalRepo struct{ repository.APIPortalRepository }

func (publishFlowPortalRepo) GetByHandleAndOrgID(handle, _ string) (*model.APIPortal, error) {
	return &model.APIPortal{ID: "portal-uuid", Handle: handle}, nil
}

type publishFlowPublicationRepo struct {
	repository.PublicationRepository
}

func (publishFlowPublicationRepo) GetDraft(_, _, _ string) (*model.Publication, []model.PublicationSelection, []model.PublicationSelection, error) {
	return &model.Publication{UUID: "draft-uuid"}, []model.PublicationSelection{{UUID: "uuid-gold", Handle: "gold", DisplayName: "Gold"}}, nil, nil
}

func (publishFlowPublicationRepo) GetContent(_ string, contentType model.PublicationContentType, _ string) (*model.PublicationContent, error) {
	if contentType != model.PublicationContentTypeDefinition {
		return nil, nil
	}
	return &model.PublicationContent{ContentType: "application/json", Content: []byte(validRESTDefinitionSpec)}, nil
}

// A failed plan step ends Publish before the listing is pushed or the draft promoted: the
// publisher's Publish and the repository's promote are left unimplemented, so reaching
// either panics.
func TestPublish_StopsWhenThePlanStepFails(t *testing.T) {
	tests := []struct {
		name string
		err  error
		is   func(error) bool
	}{
		{"portal unavailable", errors.New("connection refused"), apperror.APIPublicationPortalUnavailable.Is},
		{"portal rejection", &PortalConflictError{Message: "rejected"}, apperror.APIPublicationPortalConflict.Is},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pub := &planSyncPublisher{err: tc.err}
			svc := NewPublicationService(
				publishFlowArtifactRepo{}, publishFlowPortalRepo{}, nil,
				publicationTestPlans(), publishFlowPublicationRepo{}, pub, time.Minute, 0, nil)

			published, _, err := svc.Publish(context.Background(), restAPITypeValue, "my-api", "my-portal", "org", "actor")
			if !tc.is(err) || published != nil {
				t.Fatalf("want the mapped plan error and no publication, got %v, %v", published, err)
			}
			if pub.calls != 1 || len(pub.got) != 1 || pub.got[0].Handle != "gold" {
				t.Fatalf("want one plan call carrying the draft's plan, got %d calls with %v", pub.calls, pub.got)
			}
		})
	}
}
