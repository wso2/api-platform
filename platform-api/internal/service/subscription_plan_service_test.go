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
	"testing"
	"time"

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
	countSearch string
}

func (f *fakePlanRepo) GetByHandleAndOrg(string, string) (*model.SubscriptionPlan, error) {
	cp := *f.plan
	return &cp, nil
}

func (f *fakePlanRepo) Update(plan *model.SubscriptionPlan) error {
	f.updated = plan
	return nil
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

func (f *fakePlanRepo) ListByOrganization(_ string, opts repository.ListOptions) ([]*model.SubscriptionPlan, error) {
	f.listOpts = opts
	return []*model.SubscriptionPlan{f.plan}, nil
}

func (f *fakePlanRepo) CountByOrganization(_, search string) (int, error) {
	f.countSearch = search
	return 7, nil
}

func TestListAndCountPlans_PassSearchToRepo(t *testing.T) {
	repo := &fakePlanRepo{plan: &model.SubscriptionPlan{Handle: "gold"}}
	svc := NewSubscriptionPlanService(repo, nil, nil, nil, nil, nil)

	plans, err := svc.ListPlans("org-1", repository.ListOptions{Limit: 5, Offset: 10, Search: "gold"})
	if err != nil || len(plans) != 1 {
		t.Fatalf("ListPlans = %v, %v", plans, err)
	}
	total, err := svc.CountPlans("org-1", "gold")
	if err != nil || total != 7 {
		t.Fatalf("CountPlans = %d, %v", total, err)
	}
	if repo.listOpts != (repository.ListOptions{Limit: 5, Offset: 10, Search: "gold"}) || repo.countSearch != "gold" {
		t.Fatalf("repo got list %+v, count search %q", repo.listOpts, repo.countSearch)
	}
}
