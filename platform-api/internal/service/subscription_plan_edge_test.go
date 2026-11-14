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
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

func TestNormalizeConditions_LimitRefusals(t *testing.T) {
	windows := make([]string, maxConditionWindows+1)
	for i := range windows {
		windows[i] = fmt.Sprintf(`{"count":1,"timeAmount":%d,"timeUnit":"MINUTE"}`, i+1)
	}
	for name, tc := range map[string]struct{ params, message string }{
		"too many limits":      {`{"limits":[` + strings.Join(windows, ",") + `]}`, "at most 10 limits"},
		"negative time amount": {`{"limits":[{"count":1,"timeAmount":-1,"timeUnit":"DAY"}]}`, "timeAmount must be at least 1"},
	} {
		if _, err := normalizeConditions([]model.SubscriptionPlanCondition{condition("", "request-count", tc.params)}); !apperror.ValidationFailed.Is(err) ||
			!strings.Contains(err.Error(), tc.message) {
			t.Errorf("%s: err = %v, want %q", name, err, tc.message)
		}
	}
	if _, err := normalizeConditions([]model.SubscriptionPlanCondition{condition("promo", "custom", `{" ":1}`)}); err == nil ||
		!strings.Contains(err.Error(), "empty key") {
		t.Errorf("empty custom key: err = %v", err)
	}
}

// The portal's copy is updated when its name, description or limits differ, or when it lists
// conditions that differ; a portal that doesn't list conditions isn't compared on them.
func TestPortalCopyDiffers(t *testing.T) {
	plan := &model.SubscriptionPlan{Name: "Gold", Description: "For partners",
		Conditions: []model.SubscriptionPlanCondition{*requestLimit(100, "MINUTE"), condition("promo", "custom", `{"a":1}`)}}
	listed := func(conditions *[]model.SubscriptionPlanCondition) *portalListedPlan {
		return &portalListedPlan{DisplayName: " Gold ", Description: "For partners", Limits: portalLimits(plan), Conditions: conditions}
	}
	same := []model.SubscriptionPlanCondition{{Type: "request-count", Params: []byte(`{"limits":[{"count":100,"timeUnit":"MINUTE"}]}`)},
		condition("promo", "custom", `{"a":1}`)}
	other := []model.SubscriptionPlanCondition{*requestLimit(100, "MINUTE")}
	invalid := []model.SubscriptionPlanCondition{condition("", "bandwidth", `{}`)}

	for name, tc := range map[string]struct {
		copy *portalListedPlan
		want bool
	}{
		"same, conditions not listed": {listed(nil), false},
		"same conditions":             {listed(&same), false},
		"other conditions":            {listed(&other), true},
		"portal conditions invalid":   {listed(&invalid), true},
		"other name":                  {&portalListedPlan{DisplayName: "Silver", Description: "For partners", Limits: portalLimits(plan)}, true},
	} {
		if got := portalCopyDiffers(plan, tc.copy); got != tc.want {
			t.Errorf("%s: differs = %v, want %v", name, got, tc.want)
		}
	}

	stored := &model.SubscriptionPlan{Name: "Gold", Conditions: invalid}
	if !portalCopyDiffers(stored, &portalListedPlan{DisplayName: "Gold", Limits: []portalPlanLimit{}, Conditions: &same}) {
		t.Error("a stored plan whose conditions don't validate should be sent again")
	}
}

func TestPortalLimits_StoredWindowsAsTheyAre(t *testing.T) {
	plan := &model.SubscriptionPlan{Conditions: []model.SubscriptionPlanCondition{condition("requests", "request-count", `not json`)}}
	if limits := portalLimits(plan); limits == nil || len(limits) != 0 {
		t.Fatalf("limits = %+v, want an empty list", limits)
	}
	plan.Conditions[0].Params = []byte(`{"limits":[{"count":5,"timeUnit":"DAY"}],"onExceed":"reject"}`)
	if limits := portalLimits(plan); len(limits) != 1 || limits[0].TimeAmount != 1 {
		t.Fatalf("a window with no time amount: %+v, want one unit", limits)
	}
}

func TestNormalizeConditions_KeepsTheTokenType(t *testing.T) {
	got, err := normalizeConditions([]model.SubscriptionPlanCondition{condition("", "token-count", `{"limits":[{"count":9,"timeUnit":"DAY"}],"tokenType":"prompt"}`)})
	if err != nil || !strings.Contains(string(got[0].Params), `"tokenType":"prompt"`) {
		t.Fatalf("got %v, %v", got, err)
	}
}

// failingPlanRepo fails the writes or the usage read the test asks for.
type failingPlanRepo struct {
	*webhookPlanRepo
	saveErr  error
	usageErr error
}

func (f *failingPlanRepo) CreateWithPricing(plan *model.SubscriptionPlan, pricing *model.SubscriptionPlanPricing) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	return f.webhookPlanRepo.CreateWithPricing(plan, pricing)
}

func (f *failingPlanRepo) Update(plan *model.SubscriptionPlan) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	return f.webhookPlanRepo.Update(plan)
}

func (f *failingPlanRepo) GetUsage(a, b, c string) (*model.SubscriptionPlanUsage, error) {
	if f.usageErr != nil {
		return nil, f.usageErr
	}
	return f.webhookPlanRepo.GetUsage(a, b, c)
}

func newFailingPlanService() (*SubscriptionPlanService, *failingPlanRepo) {
	_, inner := newPlanTestService()
	repo := &failingPlanRepo{webhookPlanRepo: inner}
	return NewSubscriptionPlanService(repo, nil, nil, nil, nil, nil), repo
}

func TestPlanService_RefusesInvalidValues(t *testing.T) {
	svc, _ := newFailingPlanService()
	long := strings.Repeat("é", model.MaxSubscriptionPlanDescriptionChars+1)

	if _, err := svc.CreatePlan("org-1", "", &model.SubscriptionPlan{Handle: "silver", Name: "Silver", Description: long}, nil); !apperror.ValidationFailed.Is(err) {
		t.Errorf("create with a long description: err = %v", err)
	}
	if _, err := svc.CreatePlan("org-1", "", &model.SubscriptionPlan{Handle: "silver", Name: "Silver",
		Conditions: []model.SubscriptionPlanCondition{condition("", "bandwidth", `{}`)}}, nil); !apperror.ValidationFailed.Is(err) {
		t.Errorf("create with an invalid condition: err = %v", err)
	}
	if _, err := svc.UpdatePlan("gold", "org-1", "", &model.SubscriptionPlanUpdate{Description: &long}); !apperror.ValidationFailed.Is(err) {
		t.Errorf("update with a long description: err = %v", err)
	}
	invalid := []model.SubscriptionPlanCondition{condition("", "bandwidth", `{}`)}
	if _, err := svc.UpdatePlan("gold", "org-1", "", &model.SubscriptionPlanUpdate{Conditions: &invalid}); !apperror.ValidationFailed.Is(err) {
		t.Errorf("update with an invalid condition: err = %v", err)
	}
	status := model.SubscriptionPlanStatus("RETIRED")
	if _, err := svc.UpdatePlan("gold", "org-1", "", &model.SubscriptionPlanUpdate{Status: &status}); !apperror.ValidationFailed.Is(err) ||
		!strings.Contains(err.Error(), "invalid status") {
		t.Errorf("update with an unknown status: err = %v", err)
	}
}

// A database that can't hold the conditions yet refuses the plan with its own error, on create and
// update; a failed usage read stops an update that needs it.
func TestPlanService_StorageErrors(t *testing.T) {
	svc, repo := newFailingPlanService()
	repo.saveErr = fmt.Errorf("save: %w", repository.ErrPlanConditionsTableMissing)
	if _, err := svc.CreatePlan("org-1", "", &model.SubscriptionPlan{Handle: "silver", Name: "Silver"}, nil); !apperror.SubscriptionPlanConditionsUnsupported.Is(err) {
		t.Errorf("create: err = %v", err)
	}
	description := "Updated"
	if _, err := svc.UpdatePlan("gold", "org-1", "", &model.SubscriptionPlanUpdate{Description: &description}); !apperror.SubscriptionPlanConditionsUnsupported.Is(err) {
		t.Errorf("update: err = %v", err)
	}

	repo.saveErr = fmt.Errorf("save: %w", repository.ErrPlanDescriptionColumnMissing)
	if _, err := svc.UpdatePlan("gold", "org-1", "", &model.SubscriptionPlanUpdate{Description: &description}); !apperror.SubscriptionPlanConditionsUnsupported.Is(err) ||
		!strings.Contains(err.Error(), "description can be saved only after") {
		t.Errorf("update with a description: err = %v", err)
	}

	repo.saveErr = nil
	repo.usageErr = errors.New("usage unavailable")
	name := "Gold Plus"
	if _, err := svc.UpdatePlan("gold", "org-1", "", &model.SubscriptionPlanUpdate{Name: &name}); !errors.Is(err, repo.usageErr) {
		t.Errorf("rename with no usage: err = %v", err)
	}
}

// A stored condition that no longer validates is kept as it is when other fields change.
func TestUpdatePlan_KeepsAStoredConditionThatNoLongerValidates(t *testing.T) {
	svc, repo := newFailingPlanService()
	stored := condition("Legacy_Promo", "custom", `{"a":1}`)
	repo.plans["gold"].Conditions = []model.SubscriptionPlanCondition{stored}

	description := "Updated"
	got, err := svc.UpdatePlan("gold", "org-1", "", &model.SubscriptionPlanUpdate{Description: &description})
	if err != nil {
		t.Fatalf("UpdatePlan: %v", err)
	}
	if len(got.Conditions) != 1 || got.Conditions[0].ID != "Legacy_Promo" || got.Description != "Updated" {
		t.Fatalf("plan = %+v", got)
	}
}

// Plans from an API Portal may be saved without their description on a database not yet upgraded;
// plans made through the REST API may not.
func TestPlanService_PortalPlansMayDropTheDescription(t *testing.T) {
	svc, repo := newPlanTestService()
	if _, err := svc.UpsertPlanFromWebhook("org-1", "silver", "Silver", "From the portal", nil, nil, nil, planEventT0); err != nil {
		t.Fatal(err)
	}
	if !repo.plans["silver"].DescriptionOptional {
		t.Error("a created portal plan should allow dropping its description")
	}
	if _, err := svc.UpsertPlanFromWebhook("org-1", "gold", "Gold", "Now described", nil, win(100, "MINUTE"), nil, planEventT0); err != nil {
		t.Fatal(err)
	}
	if !repo.plans["gold"].DescriptionOptional {
		t.Error("an updated portal plan should allow dropping its description")
	}

	created, err := svc.CreatePlan("org-1", "", &model.SubscriptionPlan{Handle: "bronze", Name: "Bronze", Description: "Made here"}, nil)
	if err != nil || created.DescriptionOptional {
		t.Fatalf("a REST plan: %+v, %v; want its description required", created, err)
	}
}

// An updated name is saved trimmed, so a padded name on a plan in use can't reach the gateways, which
// match an API's plans by name. A blank name on a plan not in use is refused.
func TestUpdatePlan_TrimsTheName(t *testing.T) {
	svc, repo := newPlanTestService()
	repo.usage = model.SubscriptionPlanUsage{RESTAPIs: 1, Subscriptions: 1}

	padded := "  Gold  "
	got, err := svc.UpdatePlan("gold", "org-1", "", &model.SubscriptionPlanUpdate{Name: &padded})
	if err != nil {
		t.Fatalf("UpdatePlan: %v", err)
	}
	if got.Name != "Gold" || repo.plans["gold"].Name != "Gold" {
		t.Fatalf("name = %q, stored %q, want Gold", got.Name, repo.plans["gold"].Name)
	}

	repo.usage = model.SubscriptionPlanUsage{}
	blank := "   "
	if _, err := svc.UpdatePlan("gold", "org-1", "", &model.SubscriptionPlanUpdate{Name: &blank}); !apperror.ValidationFailed.Is(err) {
		t.Errorf("blank name: err = %v", err)
	}
}
