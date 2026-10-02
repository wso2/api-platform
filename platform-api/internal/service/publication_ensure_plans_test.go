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
	"reflect"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

type fakePlanRepo struct {
	repository.SubscriptionPlanRepository
	plans map[string]*model.SubscriptionPlan
}

func (f *fakePlanRepo) GetByHandles(handles []string, _ string) (map[string]*model.SubscriptionPlan, error) {
	out := map[string]*model.SubscriptionPlan{}
	for _, h := range handles {
		if p, ok := f.plans[h]; ok {
			out[h] = p
		}
	}
	return out, nil
}

type fakePlanPortal struct {
	PortalPublisher
	created []string
	err     error

	calls [][]string
}

func (f *fakePlanPortal) CreateSubscriptionPlansIfAbsent(_ context.Context, _ *model.APIPortal, plans []PortalPlan) ([]string, error) {
	var handles []string
	for _, p := range plans {
		handles = append(handles, p.Handle)
	}
	f.calls = append(f.calls, handles)
	return f.created, f.err
}

func plansFixture() map[string]*model.SubscriptionPlan {
	return map[string]*model.SubscriptionPlan{
		"gold":   {UUID: "u-gold", Handle: "gold", Name: "Gold", Status: model.SubscriptionPlanStatusActive, ThrottleLimitCount: intPtr(100), ThrottleLimitUnit: "MINUTE"},
		"free":   {UUID: "u-free", Handle: "free", Name: "Free", Status: model.SubscriptionPlanStatusActive},
		"silver": {UUID: "u-silver", Handle: "silver", Name: "Silver", Status: model.SubscriptionPlanStatusActive, ThrottleLimitCount: intPtr(10), ThrottleLimitUnit: "HOUR"},
		"old":    {UUID: "u-old", Handle: "old", Name: "Old", Status: model.SubscriptionPlanStatusInactive},
	}
}

func newEnsureSvc(portal *fakePlanPortal) *PublicationService {
	return &PublicationService{
		subscriptionPlanRepo: &fakePlanRepo{plans: plansFixture()},
		portalPublisher:      portal,
		slogger:              newTestLogger(),
	}
}

func ensure(s *PublicationService, handles ...string) error {
	return s.ensurePortalPlans(context.Background(), &model.APIPortal{Handle: "p"}, handles, "org", "actor")
}

func TestEnsurePortalPlans_NoPlansMakesNoPortalCalls(t *testing.T) {
	portal := &fakePlanPortal{}
	if err := ensure(newEnsureSvc(portal)); err != nil {
		t.Fatal(err)
	}
	if len(portal.calls) != 0 {
		t.Fatalf("want no portal calls, got %v", portal.calls)
	}
}

func TestEnsurePortalPlans_RejectsInactiveAndUnknownWithoutPortalCall(t *testing.T) {
	for _, h := range []string{"old", "nope"} {
		portal := &fakePlanPortal{}
		err := ensure(newEnsureSvc(portal), "gold", h)
		if !apperror.APIPublicationValidationFailed.Is(err) {
			t.Fatalf("%s: want validation error, got %v", h, err)
		}
		if len(portal.calls) != 0 {
			t.Fatalf("%s: want no portal call", h)
		}
	}
}

func TestEnsurePortalPlans_SendsAllSelectedPlansInOneCallInSelectionOrder(t *testing.T) {
	portal := &fakePlanPortal{created: []string{"silver"}}
	if err := ensure(newEnsureSvc(portal), "silver", "free", "gold"); err != nil {
		t.Fatal(err)
	}
	if want := [][]string{{"silver", "free", "gold"}}; !reflect.DeepEqual(portal.calls, want) {
		t.Fatalf("portal calls = %v, want %v", portal.calls, want)
	}
}

func TestEnsurePortalPlans_NothingCreatedIsSuccess(t *testing.T) {
	portal := &fakePlanPortal{}
	if err := ensure(newEnsureSvc(portal), "gold", "free"); err != nil {
		t.Fatal(err)
	}
	if len(portal.calls) != 1 {
		t.Fatalf("want one portal call, got %v", portal.calls)
	}
}

func TestEnsurePortalPlans_Errors(t *testing.T) {
	portal := &fakePlanPortal{err: &PortalConflictError{Message: "rejected"}}
	if err := ensure(newEnsureSvc(portal), "gold"); !apperror.APIPublicationPortalConflict.Is(err) {
		t.Fatalf("rejection: want conflict, got %v", err)
	}

	portal = &fakePlanPortal{err: errors.New("down")}
	if err := ensure(newEnsureSvc(portal), "gold"); !apperror.APIPublicationPortalUnavailable.Is(err) {
		t.Fatalf("failure: want unavailable, got %v", err)
	}
}

func TestToPortalPlan(t *testing.T) {
	plans := plansFixture()
	gold := toPortalPlan(plans["gold"])
	if gold.Handle != "gold" || gold.DisplayName != "Gold" || gold.RefID != "u-gold" ||
		!reflect.DeepEqual(gold.Limits, []PortalPlanLimit{{LimitType: "REQUEST_COUNT", TimeUnit: "MINUTE", TimeAmount: 1, LimitCount: 100}}) {
		t.Fatalf("unexpected gold: %+v", gold)
	}
	free := toPortalPlan(plans["free"])
	if !reflect.DeepEqual(free.Limits, []PortalPlanLimit{{LimitType: "REQUEST_COUNT", TimeAmount: 1, LimitCount: -1}}) {
		t.Fatalf("unexpected unlimited plan: %+v", free)
	}
}
