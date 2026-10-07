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
	err         error
	calls       int
	got         []*model.SubscriptionPlan
	hasDeadline bool
	remaining   time.Duration
}

func (f *planSyncPublisher) CreateMissingPlans(ctx context.Context, _ *model.APIPortal, plans []*model.SubscriptionPlan) error {
	f.calls++
	f.got = plans
	if d, ok := ctx.Deadline(); ok {
		f.hasDeadline = true
		f.remaining = time.Until(d)
	}
	return f.err
}

func newPublicationPlanService(repo *publicationPlanRepo, pub *planSyncPublisher) *PublicationService {
	return NewPublicationService(nil, nil, nil, repo, nil, pub, nil)
}

func publicationTestPlans() *publicationPlanRepo {
	return &publicationPlanRepo{plans: map[string]*model.SubscriptionPlan{
		"gold":   {UUID: "uuid-gold", Handle: "gold", Status: model.SubscriptionPlanStatusActive},
		"silver": {UUID: "uuid-silver", Handle: "silver", Status: model.SubscriptionPlanStatusActive},
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

func TestResolvePlanUUIDs(t *testing.T) {
	svc := newPublicationPlanService(publicationTestPlans(), nil)

	if uuids, err := svc.resolvePlanUUIDs(nil, "org"); err != nil || uuids != nil {
		t.Fatalf("no handles: got %v, %v", uuids, err)
	}
	uuids, err := svc.resolvePlanUUIDs([]string{"silver", "gold"}, "org")
	if err != nil || len(uuids) != 2 || uuids[0] != "uuid-silver" || uuids[1] != "uuid-gold" {
		t.Fatalf("want UUIDs in handle order, got %v, %v", uuids, err)
	}
	if _, err := svc.resolvePlanUUIDs([]string{"bronze"}, "org"); !apperror.APIPublicationValidationFailed.Is(err) {
		t.Fatalf("want a validation error for an inactive plan, got %v", err)
	}
}

func TestCreateMissingPortalPlans(t *testing.T) {
	portal := &model.APIPortal{}

	t.Run("no handles skips the portal", func(t *testing.T) {
		pub := &planSyncPublisher{}
		if err := newPublicationPlanService(publicationTestPlans(), pub).createMissingPortalPlans(context.Background(), portal, "org", nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pub.calls != 0 {
			t.Fatalf("want no portal call, got %d", pub.calls)
		}
	})

	t.Run("sends the selected plans in order under a deadline", func(t *testing.T) {
		pub := &planSyncPublisher{}
		if err := newPublicationPlanService(publicationTestPlans(), pub).createMissingPortalPlans(context.Background(), portal, "org", []string{"silver", "gold"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pub.calls != 1 || len(pub.got) != 2 || pub.got[0].Handle != "silver" || pub.got[1].Handle != "gold" {
			t.Fatalf("got %d calls, plans %v", pub.calls, pub.got)
		}
		if !pub.hasDeadline || pub.remaining > portalPlansTimeout {
			t.Fatalf("want a deadline of at most %s, got %v", portalPlansTimeout, pub.remaining)
		}
	})

	t.Run("inactive plan is rejected before any portal call", func(t *testing.T) {
		pub := &planSyncPublisher{}
		err := newPublicationPlanService(publicationTestPlans(), pub).createMissingPortalPlans(context.Background(), portal, "org", []string{"bronze"})
		if !apperror.APIPublicationValidationFailed.Is(err) || pub.calls != 0 {
			t.Fatalf("want a validation error and no portal call, got %v (%d calls)", err, pub.calls)
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
				err := newPublicationPlanService(publicationTestPlans(), pub).createMissingPortalPlans(context.Background(), portal, "org", []string{"gold"})
				if !tc.is(err) {
					t.Fatalf("got %v", err)
				}
			})
		}
	})
}
