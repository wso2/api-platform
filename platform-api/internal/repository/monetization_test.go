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

package repository

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

func setupMonetizationTestDB(t *testing.T) (*database.DB, MeterRepository, SubscriptionPlanPricingRepository, *model.SubscriptionPlan, func()) {
	t.Helper()
	db, cleanup := setupTestDB(t)
	if err := NewOrganizationRepo(db).CreateOrganization(&model.Organization{
		ID: "org-1", Handle: "acme", Name: "Acme", Region: "us",
	}); err != nil {
		t.Fatalf("CreateOrganization: %v", err)
	}
	plan := &model.SubscriptionPlan{Handle: "gold", Name: "Gold", OrganizationUUID: "org-1", Status: model.SubscriptionPlanStatusActive}
	if err := NewSubscriptionPlanRepo(db).Create(plan); err != nil {
		t.Fatalf("Create plan: %v", err)
	}
	return db, NewMeterRepo(db), NewSubscriptionPlanPricingRepo(db), plan, cleanup
}

func TestMeterRepo_UpsertReplacesByHandleAndRoundTripsTheEventKeys(t *testing.T) {
	db, meters, _, _, cleanup := setupMonetizationTestDB(t)
	defer cleanup()

	at := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	// A custom token key and filter key, as for a gateway that names its analytics fields differently.
	typeKey := "usage.completion_tokens"
	filterFields := []model.MeterFilterField{
		{Field: model.FilterFieldAPIType, EventKey: "custom.api.kind"},
		{Field: model.FilterFieldHTTPMethod, EventKey: "request.verb"},
	}
	meter := &model.Meter{
		OrganizationUUID: "org-1", Handle: "calls", DisplayName: "Calls", Type: model.MeterTypeTokens,
		TypeKey: &typeKey, FilterFields: filterFields, SourceUpdatedAt: at,
	}
	if err := meters.Upsert(meter); err != nil {
		t.Fatalf("Upsert (create): %v", err)
	}
	if got, err := meters.GetByHandle("org-1", "calls"); err != nil || got == nil || got.Type != model.MeterTypeTokens ||
		got.TypeKey == nil || *got.TypeKey != typeKey || !reflect.DeepEqual(got.FilterFields, filterFields) {
		t.Fatalf("meter = %+v, %v; want the event key and the filter fields with their keys kept", got, err)
	}
	// The filter fields are kept in the form the API Portal's events carry them, in order.
	var stored []byte
	if err := db.QueryRow(`SELECT filter_fields FROM meters WHERE handle = 'calls'`).Scan(&stored); err != nil {
		t.Fatalf("read filter fields: %v", err)
	}
	if want := `[{"field":"API_TYPE","event_key":"custom.api.kind"},{"field":"HTTP_METHOD","event_key":"request.verb"}]`; string(stored) != want {
		t.Errorf("stored filter fields %s, want %s", stored, want)
	}

	if err := meters.Upsert(&model.Meter{
		OrganizationUUID: "org-1", Handle: "calls", DisplayName: "Successful calls", Description: "Billed calls",
		Type: "REQUEST_COUNT", SourceUpdatedAt: at.Add(time.Minute),
	}); err != nil {
		t.Fatalf("Upsert (replace): %v", err)
	}

	got, err := meters.GetByHandle("org-1", "calls")
	if err != nil || got == nil {
		t.Fatalf("GetByHandle: %v, %v", got, err)
	}
	if got.UUID != meter.UUID || got.DisplayName != "Successful calls" || got.Description != "Billed calls" ||
		got.Type != "REQUEST_COUNT" || got.TypeKey != nil ||
		got.FilterFields == nil || len(got.FilterFields) != 0 {
		t.Errorf("meter = %+v, want the original row replaced", got)
	}
	if err := db.QueryRow(`SELECT filter_fields FROM meters WHERE handle = 'calls'`).Scan(&stored); err != nil || string(stored) != "[]" {
		t.Errorf("stored filter fields %s, %v; want [] when there are none", stored, err)
	}
	if !got.SourceUpdatedAt.Equal(at.Add(time.Minute)) {
		t.Errorf("sourceUpdatedAt = %v, want %v", got.SourceUpdatedAt, at.Add(time.Minute))
	}

	list, err := meters.ListByOrganization("org-1")
	if err != nil || len(list) != 1 {
		t.Fatalf("ListByOrganization = %d meters, err %v", len(list), err)
	}
	if err := meters.Delete("org-1", "calls"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := meters.Delete("org-1", "calls"); err != nil {
		t.Fatalf("a repeated Delete must succeed: %v", err)
	}
	if got, err := meters.GetByHandle("org-1", "calls"); err != nil || got != nil {
		t.Errorf("after Delete: %v, %v", got, err)
	}
}

// Pricing is stored with its plan, in the same transaction, and read back by plan and by
// organization.
func TestSubscriptionPlanPricingRepo_ReadsThePricingStoredWithItsPlan(t *testing.T) {
	db, _, pricing, plan, cleanup := setupMonetizationTestDB(t)
	defer cleanup()
	plans := NewSubscriptionPlanRepo(db)

	if got, err := pricing.GetByPlan(plan.UUID, "org-1"); err != nil || got != nil {
		t.Fatalf("a free plan has no pricing: %v, %v", got, err)
	}
	fee := "99.00"
	flat := &model.SubscriptionPlan{Handle: "flat", Name: "Flat", OrganizationUUID: "org-1", Status: model.SubscriptionPlanStatusActive}
	if err := plans.CreateWithPricing(flat, &model.SubscriptionPlanPricing{PricingType: model.PricingTypeFlatRate,
		Currency: "USD", BillingPeriod: "MONTH", FlatAmount: &fee, SourceUpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("CreateWithPricing (flat rate): %v", err)
	}
	if got, err := pricing.GetByPlan(flat.UUID, "org-1"); err != nil || got == nil || got.PricingType != model.PricingTypeFlatRate ||
		got.FlatAmount == nil || *got.FlatAmount != "99.00" || got.Prices == nil || len(got.Prices) != 0 {
		t.Fatalf("flat rate = %+v, %v", got, err)
	}

	unit, upTo, quantity := "2.50", int64(1000), int64(1_000_000)
	prices := []model.SubscriptionPlanPrice{
		{MeterHandle: "llm-tokens", PricingModel: model.PricingModelPerUnit, UnitAmount: &unit, UnitQuantity: &quantity,
			Filters: []model.PriceFilter{{Field: "API_TYPE", Operator: "EQUALS", Values: []string{"LlmProvider"}}}},
		{MeterHandle: "calls", PricingModel: model.PricingModelGraduated, Filters: []model.PriceFilter{},
			Tiers: []model.PriceTier{{UpTo: &upTo, UnitAmount: &unit}, {UnitAmount: &unit}}},
	}
	usage := &model.SubscriptionPlan{Handle: "usage", Name: "Usage", OrganizationUUID: "org-1", Status: model.SubscriptionPlanStatusActive}
	if err := plans.CreateWithPricing(usage, &model.SubscriptionPlanPricing{PricingType: model.PricingTypeUsageBased,
		Currency: "EUR", BillingPeriod: "YEAR", Prices: prices, SourceUpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("CreateWithPricing (usage based): %v", err)
	}

	got, err := pricing.GetByPlan(usage.UUID, "org-1")
	if err != nil || got == nil {
		t.Fatalf("GetByPlan: %v, %v", got, err)
	}
	if got.PricingType != model.PricingTypeUsageBased || got.Currency != "EUR" || got.BillingPeriod != "YEAR" || got.FlatAmount != nil {
		t.Errorf("pricing = %+v, want the usage based pricing", got)
	}
	if !reflect.DeepEqual(got.Prices, prices) {
		t.Errorf("prices = %+v, want %+v in order", got.Prices, prices)
	}
	list, err := pricing.ListByOrganization("org-1")
	if err != nil || len(list) != 2 {
		t.Fatalf("ListByOrganization = %+v, %v; want the two paid plans", list, err)
	}
	for _, p := range list {
		if p.SubscriptionPlanUUID == usage.UUID && !reflect.DeepEqual(p.Prices, prices) {
			t.Errorf("listed prices = %+v, want %+v", p.Prices, prices)
		}
	}
	if list, err := pricing.ListByOrganization("org-2"); err != nil || len(list) != 0 {
		t.Errorf("another organization's pricing = %+v, %v; want none", list, err)
	}
	// The prices are kept in the form the API Portal's events carry them.
	var stored []byte
	if err := db.QueryRow(`SELECT prices FROM subscription_plan_pricing WHERE subscription_plan_uuid = ?`, usage.UUID).Scan(&stored); err != nil {
		t.Fatalf("read prices: %v", err)
	}
	for _, key := range []string{`"meter_handle":"llm-tokens"`, `"filters":[{"field":"API_TYPE","operator":"EQUALS","values":["LlmProvider"]}]`, `"up_to":1000`} {
		if !strings.Contains(string(stored), key) {
			t.Errorf("stored prices %s lack %s", stored, key)
		}
	}
	if strings.Contains(string(stored), "display_name") {
		t.Errorf("a price has no name, stored %s", stored)
	}
}

func TestSubscriptionBillingRepo_UpsertGetMany(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	if err := NewOrganizationRepo(db).CreateOrganization(&model.Organization{ID: "org-1", Handle: "acme", Name: "Acme", Region: "us"}); err != nil {
		t.Fatalf("CreateOrganization: %v", err)
	}
	// subscription_billing references subscriptions(uuid); insert the minimum rows it needs.
	if _, err := db.Exec(`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES ('art-1', 'RestApi', 'org-1')`); err != nil {
		t.Fatalf("insert artifact: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO subscriptions (uuid, artifact_uuid, subscriber_id, subscription_token, subscription_token_hash, organization_uuid)
		VALUES ('sub-1', 'art-1', 'user-1', 'tok', 'hash', 'org-1')`); err != nil {
		t.Fatalf("insert subscription: %v", err)
	}
	repo := NewSubscriptionBillingRepo(db)

	if got, err := repo.Get("sub-1", "org-1"); err != nil || got != nil {
		t.Fatalf("a free subscription has no billing: %v, %v", got, err)
	}
	at := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	for _, status := range []string{model.BillingStatusActive, model.BillingStatusUnpaid} {
		if err := repo.Upsert(&model.SubscriptionBilling{SubscriptionUUID: "sub-1", OrganizationUUID: "org-1", Status: status,
			CustomerID: "cus_1", SubscriptionID: "sub_stripe_1", UpdatedAt: at}); err != nil {
			t.Fatalf("Upsert %s: %v", status, err)
		}
	}
	got, err := repo.GetMany([]string{"sub-1", "sub-missing"}, "org-1")
	if err != nil || len(got) != 1 {
		t.Fatalf("GetMany = %v, %v", got, err)
	}
	if b := got["sub-1"]; b.Status != model.BillingStatusUnpaid || b.CustomerID != "cus_1" || !b.UpdatedAt.Equal(at) {
		t.Errorf("billing = %+v", b)
	}
}
