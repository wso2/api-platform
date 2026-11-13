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

package service

import (
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

func TestStampAttachedPlans_StoresNameAndTime(t *testing.T) {
	repo := &publicationPlanRepo{plans: map[string]*model.SubscriptionPlan{"gold": {Handle: "gold", Name: "Gold"}}}
	now := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	got, err := stampAttachedPlans(repo, model.AttachedPlansFromHandles([]string{"gold", "gone"}), "org", now)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].DisplayName != "Gold" || got[0].AttachedAt == nil || !got[0].AttachedAt.Equal(now) {
		t.Errorf("gold = %+v", got[0])
	}
	if got[1].Handle != "gone" || got[1].DisplayName != "" || got[1].AttachedAt != nil {
		t.Errorf("a plan not in the catalog keeps only its handle, got %+v", got[1])
	}
}

func TestAnnotatePlanRefs_StatusByThePublicationRule(t *testing.T) {
	attached := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	repo := &publicationPlanRepo{plans: map[string]*model.SubscriptionPlan{
		"same":    {Handle: "same", Name: "Same", UpdatedAt: attached.Add(-time.Hour)},
		"renamed": {Handle: "renamed", Name: "Renamed now", UpdatedAt: attached.Add(time.Hour)},
		"legacy":  {Handle: "legacy", Name: "Legacy", UpdatedAt: attached.Add(time.Hour)},
	}}
	stored := model.AttachedPlans{
		{Handle: "same", DisplayName: "Same", AttachedAt: &attached},
		{Handle: "renamed", DisplayName: "Renamed", AttachedAt: &attached},
		{Handle: "deleted", DisplayName: "Deleted plan", AttachedAt: &attached},
		{Handle: "legacy"}, // stored as a handle string before the details were kept
	}
	refs := *toRefs(stored)
	if err := annotatePlanRefs(repo, &refs, stored, "org"); err != nil {
		t.Fatal(err)
	}
	want := map[string][2]string{
		"same":    {"Same", "UNCHANGED"},
		"renamed": {"Renamed now", "UPDATED"},
		"deleted": {"Deleted plan", "DELETED"},
		"legacy":  {"Legacy", "UNCHANGED"},
	}
	for _, r := range refs {
		w := want[r.Id]
		if r.DisplayName == nil || *r.DisplayName != w[0] || r.SourceStatus == nil || string(*r.SourceStatus) != w[1] {
			t.Errorf("%s = %v / %v, want %v", r.Id, deref(r.DisplayName), r.SourceStatus, w)
		}
	}
}

func TestPlanRefsWithHandles_DropsReadOnlyFields(t *testing.T) {
	refs := planRefsWithHandles([]string{"gold"})
	if len(refs) != 1 || refs[0].Id != "gold" || refs[0].DisplayName != nil || refs[0].SourceStatus != nil {
		t.Fatalf("got %+v", refs)
	}
}

func toRefs(plans model.AttachedPlans) *[]api.SubscriptionPlanRef {
	refs := make([]api.SubscriptionPlanRef, 0, len(plans))
	for _, p := range plans {
		name := p.DisplayName
		refs = append(refs, api.SubscriptionPlanRef{Id: p.Handle, DisplayName: &name})
	}
	return &refs
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}
