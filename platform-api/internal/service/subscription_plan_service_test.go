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

			if _, err := svc.UpdatePlan(context.Background(), "gold", "org-1", "actor", &tt.update); err != nil {
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

func TestUpdatePlan_LocksPortalValuesWhileInUse(t *testing.T) {
	limit := 1000
	newName, sameName, paddedName := "Gold Plus", "Gold", " Gold "
	newCount, sameCount := 500, 1000
	newUnit, sameUnit := "HOUR", "MINUTE"
	inactive := model.SubscriptionPlanStatusInactive
	inUse := model.SubscriptionPlanUsage{Publications: 1, Subscriptions: 2}

	tests := []struct {
		name    string
		limit   *int
		usage   model.SubscriptionPlanUsage
		update  model.SubscriptionPlanUpdate
		refused bool
	}{
		{"rename in use", &limit, inUse, model.SubscriptionPlanUpdate{Name: &newName}, true},
		{"new limit count in use", &limit, inUse, model.SubscriptionPlanUpdate{Name: &sameName, ThrottleLimitCount: &newCount, ThrottleLimitUnit: &sameUnit}, true},
		{"new limit unit in use", &limit, inUse, model.SubscriptionPlanUpdate{Name: &sameName, ThrottleLimitCount: &sameCount, ThrottleLimitUnit: &newUnit}, true},
		{"limit cleared in use", &limit, inUse, model.SubscriptionPlanUpdate{Name: &sameName, ClearLimit: true}, true},
		{"limit added in use", nil, inUse, model.SubscriptionPlanUpdate{Name: &sameName, ThrottleLimitCount: &newCount, ThrottleLimitUnit: &sameUnit}, true},
		{"same values and a status change in use", &limit, inUse, model.SubscriptionPlanUpdate{Name: &sameName, ThrottleLimitCount: &sameCount, ThrottleLimitUnit: &sameUnit, Status: &inactive}, false},
		{"no limit kept in use", nil, inUse, model.SubscriptionPlanUpdate{Name: &sameName, ClearLimit: true}, false},
		{"same name with surrounding spaces in use", &limit, inUse, model.SubscriptionPlanUpdate{Name: &paddedName, ThrottleLimitCount: &sameCount, ThrottleLimitUnit: &sameUnit}, false},
		{"rename and new limit unused", &limit, model.SubscriptionPlanUsage{}, model.SubscriptionPlanUpdate{Name: &newName, ThrottleLimitCount: &newCount, ThrottleLimitUnit: &newUnit}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := &model.SubscriptionPlan{Handle: "gold", Name: "Gold", Status: model.SubscriptionPlanStatusActive}
			if tt.limit != nil {
				count := *tt.limit
				plan.ThrottleLimitCount, plan.ThrottleLimitUnit = &count, "MINUTE"
			}
			repo := &fakePlanRepo{plan: plan, usage: tt.usage}
			svc := NewSubscriptionPlanService(repo, nil, nil, nil, nil, nil)

			_, err := svc.UpdatePlan(context.Background(), "gold", "org-1", "actor", &tt.update)
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

func TestUpdatePlan_InUseMessage(t *testing.T) {
	name := "Gold Plus"
	repo := &fakePlanRepo{
		plan:  &model.SubscriptionPlan{Handle: "gold", Name: "Gold"},
		usage: model.SubscriptionPlanUsage{RESTAPIs: 1, Subscriptions: 1},
	}
	svc := NewSubscriptionPlanService(repo, nil, nil, nil, nil, nil)

	_, err := svc.UpdatePlan(context.Background(), "gold", "org-1", "actor", &model.SubscriptionPlanUpdate{Name: &name})
	want := "The plan Gold is in use. To change its name or limit, create a new plan."
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("UpdatePlan error = %v, want message %q", err, want)
	}
}

func TestUpsertPlanFromWebhook_IgnoresTheLock(t *testing.T) {
	limit := 1000
	repo := &fakePlanRepo{
		plan:  &model.SubscriptionPlan{Handle: "partner", Name: "Partner", ThrottleLimitCount: &limit, ThrottleLimitUnit: "MINUTE"},
		usage: model.SubscriptionPlanUsage{Subscriptions: 3},
	}
	svc := NewSubscriptionPlanService(repo, nil, nil, nil, nil, nil)

	newLimit := 2000
	if _, err := svc.UpsertPlanFromWebhook(context.Background(), "org-1", "partner", "Partner Plus", &newLimit, "MINUTE", nil, time.Now()); err != nil {
		t.Fatalf("UpsertPlanFromWebhook: %v", err)
	}
	if repo.updated == nil || repo.updated.Name != "Partner Plus" || *repo.updated.ThrottleLimitCount != 2000 {
		t.Fatalf("portal change not applied: %+v", repo.updated)
	}
	if repo.usageCalls != 0 {
		t.Fatalf("GetUsage called %d times for a portal change", repo.usageCalls)
	}
}
