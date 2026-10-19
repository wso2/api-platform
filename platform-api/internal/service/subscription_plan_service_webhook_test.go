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
	"database/sql"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// webhookPlanRepo holds plans by handle and counts writes, so the tests can tell a reconcile that
// changes nothing (no write, no broadcast) from a real change.
//   - pricing is what the last create stored with the plan;
//   - usage is what GetUsage answers;
//   - now dates a created plan that has no CreatedAt, as the repository does with the current time.
type webhookPlanRepo struct {
	repository.SubscriptionPlanRepository
	plans   map[string]*model.SubscriptionPlan
	pricing *model.SubscriptionPlanPricing
	usage   model.SubscriptionPlanUsage
	now     time.Time
	creates int
	updates int
	deletes int
}

func (f *webhookPlanRepo) GetByHandleAndOrg(handle, _ string) (*model.SubscriptionPlan, error) {
	p, ok := f.plans[handle]
	if !ok {
		return nil, sql.ErrNoRows
	}
	cp := *p
	return &cp, nil
}

func (f *webhookPlanRepo) ExistsByHandleAndOrg(handle, _ string) (bool, error) {
	_, ok := f.plans[handle]
	return ok, nil
}

func (f *webhookPlanRepo) CreateWithPricing(plan *model.SubscriptionPlan, pricing *model.SubscriptionPlanPricing) error {
	f.creates++
	plan.UUID = "uuid-" + plan.Handle
	if plan.CreatedAt.IsZero() {
		plan.CreatedAt = f.now
	}
	cp := *plan
	f.plans[plan.Handle] = &cp
	f.pricing = pricing
	return nil
}

func (f *webhookPlanRepo) Update(plan *model.SubscriptionPlan) error {
	f.updates++
	cp := *plan
	f.plans[plan.Handle] = &cp
	return nil
}

func (f *webhookPlanRepo) GetUsage(string, string, string) (*model.SubscriptionPlanUsage, error) {
	usage := f.usage
	return &usage, nil
}

func (f *webhookPlanRepo) DeleteWithReferences(uuid, _, _ string) (bool, error) {
	for h, p := range f.plans {
		if p.UUID == uuid {
			f.deletes++
			delete(f.plans, h)
			return true, nil
		}
	}
	return false, nil
}

var planEventT0 = time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)

func TestUpsertPlanFromWebhook_CreatesMissingPlan(t *testing.T) {
	repo := &webhookPlanRepo{plans: map[string]*model.SubscriptionPlan{}}
	svc := NewSubscriptionPlanService(repo, nil, nil, nil, nil, nil)

	plan, err := svc.UpsertPlanFromWebhook(context.Background(), "org-1", "gold", "Gold", intPtr(5000), "MINUTE", nil, planEventT0)
	if err != nil {
		t.Fatalf("UpsertPlanFromWebhook: %v", err)
	}
	if repo.creates != 1 || plan.Status != model.SubscriptionPlanStatusActive || !plan.StopOnQuotaReach {
		t.Fatalf("creates = %d, plan = %+v", repo.creates, plan)
	}
	if *plan.ThrottleLimitCount != 5000 || plan.ThrottleLimitUnit != "MINUTE" {
		t.Fatalf("limit = %v %q", *plan.ThrottleLimitCount, plan.ThrottleLimitUnit)
	}
}

func TestUpsertPlanFromWebhook_UnchangedPlanIsNotWritten(t *testing.T) {
	repo := &webhookPlanRepo{plans: map[string]*model.SubscriptionPlan{
		"gold": {UUID: "u1", Handle: "gold", Name: "Gold", ThrottleLimitCount: intPtr(5000), ThrottleLimitUnit: "MINUTE",
			StopOnQuotaReach: false, Status: model.SubscriptionPlanStatusInactive},
	}}
	svc := NewSubscriptionPlanService(repo, nil, nil, nil, nil, nil)

	if _, err := svc.UpsertPlanFromWebhook(context.Background(), "org-1", "gold", "Gold", intPtr(5000), "MINUTE", nil, planEventT0); err != nil {
		t.Fatalf("UpsertPlanFromWebhook: %v", err)
	}
	if repo.creates != 0 || repo.updates != 0 {
		t.Fatalf("an echo of the current plan must not write; creates = %d, updates = %d", repo.creates, repo.updates)
	}
}

func TestUpsertPlanFromWebhook_UpdatesChangedFieldsOnly(t *testing.T) {
	repo := &webhookPlanRepo{plans: map[string]*model.SubscriptionPlan{
		"gold": {UUID: "u1", Handle: "gold", Name: "Gold", ThrottleLimitCount: intPtr(5000), ThrottleLimitUnit: "MINUTE",
			StopOnQuotaReach: false, Status: model.SubscriptionPlanStatusInactive},
	}}
	svc := NewSubscriptionPlanService(repo, nil, nil, nil, nil, nil)

	if _, err := svc.UpsertPlanFromWebhook(context.Background(), "org-1", "gold", "Gold Plus", intPtr(100), "HOUR", nil, planEventT0); err != nil {
		t.Fatalf("UpsertPlanFromWebhook: %v", err)
	}
	got := repo.plans["gold"]
	if repo.updates != 1 || got.Name != "Gold Plus" || *got.ThrottleLimitCount != 100 || got.ThrottleLimitUnit != "HOUR" {
		t.Fatalf("updates = %d, plan = %+v", repo.updates, got)
	}
	// The portal has no stop-on-quota or status setting, so the stored values are kept.
	if got.StopOnQuotaReach || got.Status != model.SubscriptionPlanStatusInactive {
		t.Fatalf("stopOnQuotaReach and status must be left as they were: %+v", got)
	}
}

func TestUpsertPlanFromWebhook_ClearsRemovedLimit(t *testing.T) {
	repo := &webhookPlanRepo{plans: map[string]*model.SubscriptionPlan{
		"gold": {UUID: "u1", Handle: "gold", Name: "Gold", ThrottleLimitCount: intPtr(5000), ThrottleLimitUnit: "MINUTE"},
	}}
	svc := NewSubscriptionPlanService(repo, nil, nil, nil, nil, nil)

	if _, err := svc.UpsertPlanFromWebhook(context.Background(), "org-1", "gold", "Gold", nil, "", nil, planEventT0); err != nil {
		t.Fatalf("UpsertPlanFromWebhook: %v", err)
	}
	if got := repo.plans["gold"]; got.ThrottleLimitCount != nil || got.ThrottleLimitUnit != "" {
		t.Fatalf("limit not cleared: %+v", got)
	}
}

func TestDeletePlanFromWebhook_MissingPlanIsDeleted(t *testing.T) {
	repo := &webhookPlanRepo{plans: map[string]*model.SubscriptionPlan{
		"gold": {UUID: "u1", Handle: "gold", Name: "Gold"},
	}}
	svc := NewSubscriptionPlanService(repo, nil, nil, nil, nil, nil)

	if err := svc.DeletePlanFromWebhook(context.Background(), "org-1", "gold", planEventT0); err != nil || repo.deletes != 1 {
		t.Fatalf("delete: err = %v, deletes = %d", err, repo.deletes)
	}
	if err := svc.DeletePlanFromWebhook(context.Background(), "org-1", "gold", planEventT0); err != nil {
		t.Fatalf("a repeated delete must succeed, got %v", err)
	}
}

// The API Portal's deletion of a plan removes the plan here whatever offers or uses it, since
// deletePlanWithReferences takes the plan out of those first.
func TestDeletePlanFromWebhook_FollowsThePortalEvenWhenThePlanIsUsed(t *testing.T) {
	repo := &webhookPlanRepo{plans: map[string]*model.SubscriptionPlan{"gold": {UUID: "u1", Handle: "gold", Name: "Gold"}},
		usage: model.SubscriptionPlanUsage{RESTAPIs: 1, Subscriptions: 3}}
	svc := NewSubscriptionPlanService(repo, nil, nil, nil, nil, nil)

	if err := svc.DeletePlanFromWebhook(context.Background(), "org-1", "gold", planEventT0); err != nil || repo.deletes != 1 {
		t.Fatalf("delete: err = %v, deletes = %d", err, repo.deletes)
	}
}

// A deletion that happened before the plan here was created is of an older plan with the same
// handle (deleted, then created again through the REST API): it leaves the plan.
func TestDeletePlanFromWebhook_LeavesAPlanCreatedAfterTheDeletion(t *testing.T) {
	repo := &webhookPlanRepo{plans: map[string]*model.SubscriptionPlan{
		"gold": {UUID: "u2", Handle: "gold", Name: "Gold", CreatedAt: planEventT0.Add(time.Minute)},
	}}
	svc := NewSubscriptionPlanService(repo, nil, nil, nil, nil, nil)

	if err := svc.DeletePlanFromWebhook(context.Background(), "org-1", "gold", planEventT0); err != nil || repo.deletes != 0 {
		t.Fatalf("late deletion: err = %v, deletes = %d; want the newer plan kept", err, repo.deletes)
	}
	if got := repo.plans["gold"]; got == nil || got.Name != "Gold" {
		t.Fatalf("plan = %+v, want the newer plan unchanged", got)
	}

	// A plan created before the deletion is the one deleted.
	repo.plans["gold"].CreatedAt = planEventT0.Add(-time.Minute)
	if err := svc.DeletePlanFromWebhook(context.Background(), "org-1", "gold", planEventT0); err != nil || repo.deletes != 1 {
		t.Fatalf("deletion: err = %v, deletes = %d", err, repo.deletes)
	}
}

// A copy of a portal plan is dated when the plan was made in the portal, not when its event was
// processed, so the portal's deletion of it soon after is followed, however late the created event
// was processed.
func TestDeletePlanFromWebhook_DeletesACopyCreatedFromAnEarlierEvent(t *testing.T) {
	repo := &webhookPlanRepo{plans: map[string]*model.SubscriptionPlan{}, now: planEventT0.Add(5 * time.Second)}
	svc := NewSubscriptionPlanService(repo, nil, nil, nil, nil, nil)

	if _, err := svc.UpsertPlanFromWebhook(context.Background(), "org-1", "gold", "Gold", nil, "", nil, planEventT0); err != nil {
		t.Fatalf("created event: %v", err)
	}
	if got := repo.plans["gold"]; got == nil || !got.CreatedAt.Equal(planEventT0) {
		t.Fatalf("plan = %+v, want it dated at the created event's time", got)
	}
	if err := svc.DeletePlanFromWebhook(context.Background(), "org-1", "gold", planEventT0.Add(time.Second)); err != nil || repo.deletes != 1 {
		t.Fatalf("deleted event: err = %v, deletes = %d; want the copy deleted", err, repo.deletes)
	}
}

// newPricedPlanService is a plan service whose organization has the meter "calls", for plans
// created with pricing.
func newPricedPlanService() (*SubscriptionPlanService, *webhookPlanRepo, *fakePricingRepo) {
	repo := &webhookPlanRepo{plans: map[string]*model.SubscriptionPlan{}}
	meters := &fakeMeterRepo{meters: map[string]*model.Meter{"calls": {UUID: "m-1", Handle: "calls"}}}
	pricing := &fakePricingRepo{pricing: map[string]*model.SubscriptionPlanPricing{}}
	svc := NewSubscriptionPlanService(repo, nil, nil, nil, nil, nil)
	svc.SetPricingRepositories(meters, pricing)
	return svc, repo, pricing
}

func flatPricing(amount string) *model.SubscriptionPlanPricing {
	return &model.SubscriptionPlanPricing{OrganizationUUID: "org-1", PricingType: model.PricingTypeFlatRate, Currency: "USD",
		BillingPeriod: "MONTH", FlatAmount: &amount, Prices: []model.SubscriptionPlanPrice{}}
}

// A plan created in the API Portal is stored with its pricing in one write, both dated when the
// portal made them.
func TestUpsertPlanFromWebhook_StoresANewPlanWithItsPricing(t *testing.T) {
	svc, repo, _ := newPricedPlanService()
	unit := "0.01"
	pricing := &model.SubscriptionPlanPricing{OrganizationUUID: "org-1", PricingType: model.PricingTypeUsageBased, Currency: "USD",
		BillingPeriod: "MONTH", Prices: []model.SubscriptionPlanPrice{
			{MeterHandle: "calls", PricingModel: model.PricingModelPerUnit, UnitAmount: &unit, Filters: []model.PriceFilter{}}}}

	plan, err := svc.UpsertPlanFromWebhook(context.Background(), "org-1", "gold", "Gold", intPtr(5000), "MINUTE", pricing, planEventT0)
	if err != nil {
		t.Fatalf("UpsertPlanFromWebhook: %v", err)
	}
	if repo.creates != 1 || repo.pricing != pricing || !plan.CreatedAt.Equal(planEventT0) {
		t.Fatalf("creates = %d, stored pricing = %+v, plan = %+v; want one create with the pricing", repo.creates, repo.pricing, plan)
	}
	if !pricing.CreatedAt.Equal(planEventT0) {
		t.Errorf("pricing dated %v, want %v", pricing.CreatedAt, planEventT0)
	}
}

// A price on a meter that is not here refuses the plan: neither the plan nor its pricing is stored.
func TestUpsertPlanFromWebhook_NeedsEveryMeter(t *testing.T) {
	svc, repo, _ := newPricedPlanService()
	unit := "0.01"
	pricing := &model.SubscriptionPlanPricing{OrganizationUUID: "org-1", PricingType: model.PricingTypeUsageBased, Currency: "USD",
		BillingPeriod: "MONTH", Prices: []model.SubscriptionPlanPrice{
			{MeterHandle: "tokens", PricingModel: model.PricingModelPerUnit, UnitAmount: &unit, Filters: []model.PriceFilter{}}}}

	_, err := svc.UpsertPlanFromWebhook(context.Background(), "org-1", "gold", "Gold", nil, "", pricing, planEventT0)
	if !apperror.ValidationFailed.Is(err) || repo.creates != 0 {
		t.Fatalf("err = %v, creates = %d; want a refusal and nothing stored", err, repo.creates)
	}
}

// A paid plan here keeps its pricing: the same pricing again (a repeated delivery, or an update) is
// accepted, and other pricing or none is refused, storing nothing.
func TestUpsertPlanFromWebhook_APaidPlanHereKeepsItsPricing(t *testing.T) {
	svc, repo, pricing := newPricedPlanService()
	if _, err := svc.UpsertPlanFromWebhook(context.Background(), "org-1", "gold", "Gold", nil, "", flatPricing("10.00"), planEventT0); err != nil {
		t.Fatalf("first delivery: %v", err)
	}
	pricing.pricing["uuid-gold"] = repo.pricing

	if _, err := svc.UpsertPlanFromWebhook(context.Background(), "org-1", "gold", "Gold Plus", nil, "", flatPricing("10"), planEventT0); err != nil {
		t.Fatalf("the same pricing again must be accepted: %v", err)
	}
	if repo.plans["gold"].Name != "Gold Plus" {
		t.Errorf("name = %q, want the update applied", repo.plans["gold"].Name)
	}
	for name, other := range map[string]*model.SubscriptionPlanPricing{"other pricing": flatPricing("20.00"), "no pricing": nil} {
		if _, err := svc.UpsertPlanFromWebhook(context.Background(), "org-1", "gold", "Gold", nil, "", other, planEventT0); !apperror.SubscriptionPlanPricingLocked.Is(err) {
			t.Errorf("%s: got %v, want the pricing-locked conflict", name, err)
		}
	}
	if repo.creates != 1 || repo.updates != 1 || *pricing.pricing["uuid-gold"].FlatAmount != "10.00" {
		t.Errorf("only the name may change after the first delivery: creates %d, updates %d", repo.creates, repo.updates)
	}
}

// A missed created event is recovered by the next update, which carries the pricing too: the plan
// is created paid. A free plan here that an event gives pricing is another plan with the same
// handle: it is refused and stays free.
func TestUpsertPlanFromWebhook_AnUpdateRecoversAMissedCreatedEvent(t *testing.T) {
	svc, repo, pricing := newPricedPlanService()
	if _, err := svc.UpsertPlanFromWebhook(context.Background(), "org-1", "gold", "Gold", nil, "", flatPricing("10.00"), planEventT0); err != nil {
		t.Fatalf("an update as the first event: %v", err)
	}
	if repo.creates != 1 || repo.pricing == nil || *repo.pricing.FlatAmount != "10.00" {
		t.Fatalf("creates = %d, pricing = %+v; want gold created with its pricing", repo.creates, repo.pricing)
	}

	repo.plans["silver"] = &model.SubscriptionPlan{UUID: "uuid-silver", Handle: "silver", Name: "Silver"}
	if _, err := svc.UpsertPlanFromWebhook(context.Background(), "org-1", "silver", "Silver", nil, "", flatPricing("5.00"), planEventT0); !apperror.SubscriptionPlanPricingLocked.Is(err) {
		t.Errorf("pricing for a free plan here: got %v, want the pricing-locked conflict", err)
	}
	if pricing.pricing["uuid-silver"] != nil || repo.plans["silver"].Name != "Silver" {
		t.Errorf("the free plan must stay as it was")
	}
}
