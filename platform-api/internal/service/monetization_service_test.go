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
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

type fakeMeterRepo struct {
	repository.MeterRepository
	meters  map[string]*model.Meter
	upserts int
	deletes int
}

func (f *fakeMeterRepo) GetByHandle(_, handle string) (*model.Meter, error) {
	m, ok := f.meters[handle]
	if !ok {
		return nil, nil
	}
	cp := *m
	return &cp, nil
}

// ListByOrganization returns the meters in handle order, as the repository does.
func (f *fakeMeterRepo) ListByOrganization(string) ([]*model.Meter, error) {
	list := make([]*model.Meter, 0, len(f.meters))
	for _, m := range f.meters {
		cp := *m
		list = append(list, &cp)
	}
	slices.SortFunc(list, func(a, b *model.Meter) int { return strings.Compare(a.Handle, b.Handle) })
	return list, nil
}

func (f *fakeMeterRepo) Create(m *model.Meter) error {
	if _, ok := f.meters[m.Handle]; ok {
		return repository.ErrMeterExists
	}
	cp := *m
	f.meters[m.Handle] = &cp
	return nil
}

func (f *fakeMeterRepo) Upsert(m *model.Meter) error {
	f.upserts++
	cp := *m
	f.meters[m.Handle] = &cp
	return nil
}

func (f *fakeMeterRepo) Delete(_, handle string) error {
	f.deletes++
	delete(f.meters, handle)
	return nil
}

type fakePricingRepo struct {
	repository.SubscriptionPlanPricingRepository
	pricing map[string]*model.SubscriptionPlanPricing
}

func (f *fakePricingRepo) GetByPlan(planUUID, _ string) (*model.SubscriptionPlanPricing, error) {
	return f.pricing[planUUID], nil
}

func (f *fakePricingRepo) ListByOrganization(string) ([]*model.SubscriptionPlanPricing, error) {
	list := make([]*model.SubscriptionPlanPricing, 0, len(f.pricing))
	for _, p := range f.pricing {
		list = append(list, p)
	}
	return list, nil
}

type fakeHandlePlanRepo struct {
	repository.SubscriptionPlanRepository
	plans map[string]*model.SubscriptionPlan
}

func (f *fakeHandlePlanRepo) GetByHandleAndOrg(handle, _ string) (*model.SubscriptionPlan, error) {
	p, ok := f.plans[handle]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return p, nil
}

func (f *fakeHandlePlanRepo) GetHandlesByIDs(planUUIDs []string, _ string) (map[string]string, error) {
	handles := map[string]string{}
	for _, p := range f.plans {
		if slices.Contains(planUUIDs, p.UUID) {
			handles[p.UUID] = p.Handle
		}
	}
	return handles, nil
}

var monetizationT0 = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

func newMonetizationTestService() (*MonetizationService, *fakeMeterRepo, *fakePricingRepo) {
	meters := &fakeMeterRepo{meters: map[string]*model.Meter{}}
	pricing := &fakePricingRepo{pricing: map[string]*model.SubscriptionPlanPricing{}}
	return NewMonetizationService(meters, pricing, nil), meters, pricing
}

func TestUpsertMeterFromWebhook_IgnoresAStaleEventAndKeepsTheUUID(t *testing.T) {
	svc, meters, _ := newMonetizationTestService()
	meters.meters["calls"] = &model.Meter{UUID: "m-1", Handle: "calls", DisplayName: "Calls", SourceUpdatedAt: monetizationT0}

	if err := svc.UpsertMeterFromWebhook(&model.Meter{Handle: "calls", DisplayName: "Old", SourceUpdatedAt: monetizationT0.Add(-time.Second)}); err != nil {
		t.Fatalf("stale upsert: %v", err)
	}
	if meters.upserts != 0 {
		t.Fatalf("a stale event must not be written")
	}
	if err := svc.UpsertMeterFromWebhook(&model.Meter{Handle: "calls", DisplayName: "New", SourceUpdatedAt: monetizationT0}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if got := meters.meters["calls"]; got.DisplayName != "New" || got.UUID != "m-1" {
		t.Fatalf("meter = %+v, want the change applied to the same row", got)
	}
}

func TestDeleteMeterFromWebhook_IgnoresADeletionOlderThanTheCopy(t *testing.T) {
	svc, meters, _ := newMonetizationTestService()
	meters.meters["calls"] = &model.Meter{UUID: "m-1", Handle: "calls", SourceUpdatedAt: monetizationT0}

	if err := svc.DeleteMeterFromWebhook("org-1", "calls", monetizationT0.Add(-time.Minute)); err != nil || meters.deletes != 0 {
		t.Fatalf("stale delete: err = %v, deletes = %d", err, meters.deletes)
	}
	if err := svc.DeleteMeterFromWebhook("org-1", "calls", monetizationT0); err != nil || meters.deletes != 1 {
		t.Fatalf("delete: err = %v, deletes = %d", err, meters.deletes)
	}
	if err := svc.DeleteMeterFromWebhook("org-1", "calls", monetizationT0); err != nil || meters.deletes != 1 {
		t.Fatalf("a meter already gone is deleted: err = %v, deletes = %d", err, meters.deletes)
	}
}

// A meter a plan here prices was used to make that plan's prices: an API Portal event can neither
// change nor delete it. A meter no plan here prices follows the portal.
func TestMeterEvents_LeaveAMeterAPlanHerePrices(t *testing.T) {
	svc, meters, pricing := newMonetizationTestService()
	meters.meters["calls"] = &model.Meter{UUID: "m-1", Handle: "calls", DisplayName: "Calls", SourceUpdatedAt: monetizationT0}
	meters.meters["spare"] = &model.Meter{UUID: "m-2", Handle: "spare", DisplayName: "Spare", SourceUpdatedAt: monetizationT0}
	pricing.pricing["plan-1"] = pricedOnMeter("plan-1", "calls")
	later := monetizationT0.Add(time.Minute)

	if err := svc.UpsertMeterFromWebhook(&model.Meter{Handle: "calls", DisplayName: "Renamed", SourceUpdatedAt: later}); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteMeterFromWebhook("org", "calls", later); err != nil {
		t.Fatal(err)
	}
	if m := meters.meters["calls"]; m == nil || m.DisplayName != "Calls" {
		t.Fatalf("a priced meter changed: %+v", m)
	}

	if err := svc.UpsertMeterFromWebhook(&model.Meter{Handle: "spare", DisplayName: "Renamed", SourceUpdatedAt: later}); err != nil ||
		meters.meters["spare"].DisplayName != "Renamed" {
		t.Fatalf("an unpriced meter: %v, %+v", err, meters.meters["spare"])
	}
	if err := svc.DeleteMeterFromWebhook("org", "spare", later); err != nil || meters.meters["spare"] != nil {
		t.Fatalf("an unpriced meter's deletion: %v", err)
	}
}
