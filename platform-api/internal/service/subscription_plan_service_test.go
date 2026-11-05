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
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// fakePlanRepo implements only what UpdatePlan touches; the embedded nil
// interface panics on any other call, which would flag an unexpected dependency.
type fakePlanRepo struct {
	repository.SubscriptionPlanRepository
	plan        *model.SubscriptionPlan
	updated     *model.SubscriptionPlan
	listOpts    repository.ListOptions
	listStatus  string
	countSearch string
	countStatus string
	usage       model.SubscriptionPlanUsage
	usageCalls  int
}

func (f *fakePlanRepo) GetByHandleAndOrg(string, string) (*model.SubscriptionPlan, error) {
	cp := *f.plan
	return &cp, nil
}

func (f *fakePlanRepo) Update(plan *model.SubscriptionPlan) error {
	f.updated = plan
	return nil
}

func (f *fakePlanRepo) GetUsage(string, string, string) (*model.SubscriptionPlanUsage, error) {
	f.usageCalls++
	usage := f.usage
	return &usage, nil
}

func TestUpdatePlan_ExpiryTime(t *testing.T) {
	expiry := time.Date(2030, 1, 2, 0, 0, 0, 0, time.UTC)
	newExpiry := time.Date(2031, 5, 6, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		update model.SubscriptionPlanUpdate
		want   *time.Time
	}{
		{"omitted keeps existing expiry", model.SubscriptionPlanUpdate{}, &expiry},
		{"explicit clear removes expiry", model.SubscriptionPlanUpdate{ClearExpiryTime: true}, nil},
		{"new value replaces expiry", model.SubscriptionPlanUpdate{ExpiryTime: &newExpiry}, &newExpiry},
		{"value wins over clear", model.SubscriptionPlanUpdate{ExpiryTime: &newExpiry, ClearExpiryTime: true}, &newExpiry},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakePlanRepo{plan: &model.SubscriptionPlan{Handle: "gold", Name: "Gold", ExpiryTime: &expiry}}
			svc := NewSubscriptionPlanService(repo, nil, nil, nil, nil, nil)

			if _, err := svc.UpdatePlan("gold", "org-1", "actor", &tt.update); err != nil {
				t.Fatalf("UpdatePlan: %v", err)
			}
			got := repo.updated.ExpiryTime
			if (got == nil) != (tt.want == nil) || (got != nil && !got.Equal(*tt.want)) {
				t.Fatalf("expiry = %v, want %v", got, tt.want)
			}
		})
	}
}

func (f *fakePlanRepo) ListByOrganization(_ string, opts repository.ListOptions, status string) ([]*model.SubscriptionPlan, error) {
	f.listOpts, f.listStatus = opts, status
	return []*model.SubscriptionPlan{f.plan}, nil
}

func (f *fakePlanRepo) CountByOrganization(_, search, status string) (int, error) {
	f.countSearch, f.countStatus = search, status
	return 7, nil
}

func TestListAndCountPlans_PassSearchToRepo(t *testing.T) {
	repo := &fakePlanRepo{plan: &model.SubscriptionPlan{Handle: "gold"}}
	svc := NewSubscriptionPlanService(repo, nil, nil, nil, nil, nil)

	plans, err := svc.ListPlans("org-1", repository.ListOptions{Limit: 5, Offset: 10, Search: "gold"}, "ACTIVE")
	if err != nil || len(plans) != 1 {
		t.Fatalf("ListPlans = %v, %v", plans, err)
	}
	total, err := svc.CountPlans("org-1", "gold", "ACTIVE")
	if err != nil || total != 7 {
		t.Fatalf("CountPlans = %d, %v", total, err)
	}
	if repo.listOpts != (repository.ListOptions{Limit: 5, Offset: 10, Search: "gold"}) || repo.countSearch != "gold" ||
		repo.listStatus != "ACTIVE" || repo.countStatus != "ACTIVE" {
		t.Fatalf("repo got list %+v %q, count search %q %q", repo.listOpts, repo.listStatus, repo.countSearch, repo.countStatus)
	}
}

// requestLimit is a request-count condition of one window of count requests per unit.
func requestLimit(count int, unit string) *model.SubscriptionPlanCondition {
	c := model.NewRequestCountCondition([]model.ConditionWindow{{Count: count, TimeAmount: 1, TimeUnit: unit}}, true)
	return &c
}

func TestUpdatePlan_LocksPortalValuesWhileInUse(t *testing.T) {
	newName, sameName, paddedName := "Gold Plus", "Gold", " Gold "
	description := "Now with a description"
	inactive := model.SubscriptionPlanStatusInactive
	inUse := model.SubscriptionPlanUsage{Publications: 1, Subscriptions: 2}
	toolCalls := model.SubscriptionPlanCondition{Type: model.ConditionTypeToolCallCount,
		Params: []byte(`{"limits":[{"count":10,"timeUnit":"MINUTE"}]}`)}
	// The plan's current conditions, sent again in another order of fields and without defaults.
	sameConditions := []model.SubscriptionPlanCondition{{Type: model.ConditionTypeRequestCount,
		Params: []byte(`{"onExceed":"reject","limits":[{"timeUnit":"MINUTE","count":1000}]}`)}}
	addedConditions := []model.SubscriptionPlanCondition{*requestLimit(1000, "MINUTE"), toolCalls}

	tests := []struct {
		name    string
		limit   *model.SubscriptionPlanCondition
		usage   model.SubscriptionPlanUsage
		update  model.SubscriptionPlanUpdate
		refused bool
	}{
		{"rename in use", requestLimit(1000, "MINUTE"), inUse, model.SubscriptionPlanUpdate{Name: &newName}, true},
		{"new limit count in use", requestLimit(1000, "MINUTE"), inUse, model.SubscriptionPlanUpdate{Name: &sameName, RequestLimit: requestLimit(500, "MINUTE")}, true},
		{"new limit unit in use", requestLimit(1000, "MINUTE"), inUse, model.SubscriptionPlanUpdate{Name: &sameName, RequestLimit: requestLimit(1000, "HOUR")}, true},
		{"limit cleared in use", requestLimit(1000, "MINUTE"), inUse, model.SubscriptionPlanUpdate{Name: &sameName, ClearLimit: true}, true},
		{"limit added in use", nil, inUse, model.SubscriptionPlanUpdate{Name: &sameName, RequestLimit: requestLimit(500, "MINUTE")}, true},
		{"condition added in use", requestLimit(1000, "MINUTE"), inUse, model.SubscriptionPlanUpdate{Name: &sameName, Conditions: &addedConditions}, true},
		{"same values and a status change in use", requestLimit(1000, "MINUTE"), inUse, model.SubscriptionPlanUpdate{Name: &sameName, RequestLimit: requestLimit(1000, "MINUTE"), Status: &inactive}, false},
		{"same conditions sent another way in use", requestLimit(1000, "MINUTE"), inUse, model.SubscriptionPlanUpdate{Name: &sameName, Conditions: &sameConditions}, false},
		{"description change in use", requestLimit(1000, "MINUTE"), inUse, model.SubscriptionPlanUpdate{Name: &sameName, Description: &description}, false},
		{"no limit kept in use", nil, inUse, model.SubscriptionPlanUpdate{Name: &sameName, ClearLimit: true}, false},
		{"same name with surrounding spaces in use", requestLimit(1000, "MINUTE"), inUse, model.SubscriptionPlanUpdate{Name: &paddedName, RequestLimit: requestLimit(1000, "MINUTE")}, false},
		{"rename and new limit unused", requestLimit(1000, "MINUTE"), model.SubscriptionPlanUsage{}, model.SubscriptionPlanUpdate{Name: &newName, RequestLimit: requestLimit(500, "HOUR")}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := &model.SubscriptionPlan{Handle: "gold", Name: "Gold", Status: model.SubscriptionPlanStatusActive}
			if tt.limit != nil {
				plan.Conditions = []model.SubscriptionPlanCondition{*tt.limit}
			}
			repo := &fakePlanRepo{plan: plan, usage: tt.usage}
			svc := NewSubscriptionPlanService(repo, nil, nil, nil, nil, nil)

			_, err := svc.UpdatePlan("gold", "org-1", "actor", &tt.update)
			if tt.refused {
				if !apperror.SubscriptionPlanInUse.Is(err) {
					t.Fatalf("UpdatePlan error = %v, want SubscriptionPlanInUse", err)
				}
				if repo.updated != nil {
					t.Fatal("a refused update was saved")
				}
				return
			}
			if err != nil {
				t.Fatalf("UpdatePlan: %v", err)
			}
			if repo.updated == nil {
				t.Fatal("the update was not saved")
			}
		})
	}
}

// Request limits sent the old way change only the request-count condition; the plan's other
// conditions stay, and the legacy request limit follows.
func TestUpdatePlan_RequestLimitKeepsOtherConditions(t *testing.T) {
	custom := model.SubscriptionPlanCondition{ID: "ip-ranges", Type: model.ConditionTypeCustom, Params: []byte(`{"cidr":"10.0.0.0/8"}`)}
	repo := &fakePlanRepo{plan: &model.SubscriptionPlan{Handle: "gold", Name: "Gold",
		Conditions: []model.SubscriptionPlanCondition{*requestLimit(1000, "MINUTE"), custom}}}
	svc := NewSubscriptionPlanService(repo, nil, nil, nil, nil, nil)

	name := "Gold"
	got, err := svc.UpdatePlan("gold", "org-1", "actor", &model.SubscriptionPlanUpdate{Name: &name, RequestLimit: requestLimit(20, "HOUR")})
	if err != nil {
		t.Fatalf("UpdatePlan: %v", err)
	}
	if len(got.Conditions) != 2 || got.Conditions[0].Type != model.ConditionTypeRequestCount || got.Conditions[1].ID != "ip-ranges" {
		t.Fatalf("conditions = %+v, want the new request limit then ip-ranges", got.Conditions)
	}
	if got.ThrottleLimitCount == nil || *got.ThrottleLimitCount != 20 || got.ThrottleLimitUnit != "HOUR" {
		t.Fatalf("legacy limit = %v %q, want 20 HOUR", got.ThrottleLimitCount, got.ThrottleLimitUnit)
	}
}

func TestUpdatePlan_InUseMessage(t *testing.T) {
	name := "Gold Plus"
	repo := &fakePlanRepo{
		plan:  &model.SubscriptionPlan{Handle: "gold", Name: "Gold"},
		usage: model.SubscriptionPlanUsage{RESTAPIs: 1, Subscriptions: 1},
	}
	svc := NewSubscriptionPlanService(repo, nil, nil, nil, nil, nil)

	_, err := svc.UpdatePlan("gold", "org-1", "actor", &model.SubscriptionPlanUpdate{Name: &name})
	want := "The plan Gold is in use. To change its name or conditions, create a new plan."
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("UpdatePlan error = %v, want message %q", err, want)
	}
}

func TestUpsertPlanFromWebhook_IgnoresTheLock(t *testing.T) {
	repo := &fakePlanRepo{
		plan:  &model.SubscriptionPlan{Handle: "partner", Name: "Partner", Conditions: []model.SubscriptionPlanCondition{*requestLimit(1000, "MINUTE")}},
		usage: model.SubscriptionPlanUsage{Subscriptions: 3},
	}
	svc := NewSubscriptionPlanService(repo, nil, nil, nil, nil, nil)

	windows := []model.ConditionWindow{{Count: 2000, TimeAmount: 1, TimeUnit: "MINUTE"}}
	if _, err := svc.UpsertPlanFromWebhook("org-1", "partner", "Partner Plus", "", nil, windows, nil, time.Now()); err != nil {
		t.Fatalf("UpsertPlanFromWebhook: %v", err)
	}
	if repo.updated == nil || repo.updated.Name != "Partner Plus" || *repo.updated.ThrottleLimitCount != 2000 {
		t.Fatalf("portal change not applied: %+v", repo.updated)
	}
	if repo.usageCalls != 0 {
		t.Fatalf("GetUsage called %d times for a portal change", repo.usageCalls)
	}
}
