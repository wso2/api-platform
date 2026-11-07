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

package webhook

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

const planTestOrgUUID = "33333333-3333-3333-3333-333333333333"

type planTestOrgs struct{}

func (planTestOrgs) GetOrganizationByHandle(handle string) (*model.Organization, error) {
	if handle != "acme" {
		return nil, nil
	}
	return &model.Organization{ID: planTestOrgUUID, Handle: "acme"}, nil
}

// upsertCall is one plan the receiver passed on, with its pricing (nil for a free plan).
// conditions is nil when the event carried none; windows are its request limits.
type upsertCall struct {
	orgUUID, handle, name, description string
	conditions                         *[]model.SubscriptionPlanCondition
	windows                            []model.ConditionWindow
	occurredAt                         time.Time
	pricing                            *model.SubscriptionPlanPricing
}

type fakePlanService struct {
	upserts          []upsertCall
	deletes          []string
	deleteOccurredAt time.Time
	deleteErr        error
}

func (f *fakePlanService) UpsertPlanFromWebhook(orgUUID, handle, name, description string, conditions *[]model.SubscriptionPlanCondition, windows []model.ConditionWindow, pricing *model.SubscriptionPlanPricing, occurredAt time.Time) (*model.SubscriptionPlan, error) {
	f.upserts = append(f.upserts, upsertCall{orgUUID, handle, name, description, conditions, windows, occurredAt, pricing})
	return &model.SubscriptionPlan{Handle: handle, Name: name}, nil
}

func (f *fakePlanService) DeletePlanFromWebhook(orgUUID, handle string, occurredAt time.Time) error {
	f.deletes = append(f.deletes, orgUUID+"/"+handle)
	f.deleteOccurredAt = occurredAt
	return f.deleteErr
}

func newPlanTestReceiver(t *testing.T) (*Receiver, *fakePlanService, config.Webhook) {
	t.Helper()
	cfg := config.Webhook{
		Enabled:            true,
		Secret:             "plan-test-secret",
		SignatureTolerance: 5 * time.Minute,
		MaxBodySize:        1 << 20,
		SignatureHeader:    "X-Api-Portal-Signature",
	}
	plans := &fakePlanService{}
	r, err := NewReceiver(cfg, nil, nil, nil, plans, nil, planTestOrgs{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewReceiver: %v", err)
	}
	return r, plans, cfg
}

func postPlanEvent(t *testing.T, r *Receiver, cfg config.Webhook, eventType, data string) (*httptest.ResponseRecorder, error) {
	t.Helper()
	body := []byte(`{"event_id":"evt-plan","event_type":"` + eventType + `","occurred_at":"2026-10-11T09:30:00Z","org":{"ref_id":"acme"},"data":` + data + `}`)
	req := httptest.NewRequest(http.MethodPost, RoutePath, bytes.NewReader(body))
	req.Header.Set(cfg.SignatureHeader, sign(cfg.Secret, time.Now().Unix(), body))
	rec := httptest.NewRecorder()
	return rec, r.ReceiveEvent(rec, req)
}

func TestSubscriptionPlanCreated_CreatesAFreePlanWithRequestLimit(t *testing.T) {
	r, plans, cfg := newPlanTestReceiver(t)

	rec, err := postPlanEvent(t, r, cfg, EventSubscriptionPlanCreated,
		`{"handle":"gold","display_name":"Gold","limits":[{"limit_type":"REQUEST_COUNT","limit_count":5000,"time_unit":"minute","time_amount":1}],"pricing":null}`)
	if err != nil || rec.Code != http.StatusOK {
		t.Fatalf("status = %d, err = %v", rec.Code, err)
	}
	if len(plans.upserts) != 1 {
		t.Fatalf("upserts = %d, want 1", len(plans.upserts))
	}
	got := plans.upserts[0]
	if got.orgUUID != planTestOrgUUID || got.handle != "gold" || got.name != "Gold" || got.pricing != nil {
		t.Errorf("upsert = %+v, want the free plan gold", got)
	}
	if got.conditions != nil || len(got.windows) != 1 || got.windows[0] != (model.ConditionWindow{Count: 5000, TimeAmount: 1, TimeUnit: "MINUTE"}) {
		t.Errorf("conditions = %v, windows = %+v, want only the request window 5000 per MINUTE", got.conditions, got.windows)
	}
	if want := time.Date(2026, 10, 11, 9, 30, 0, 0, time.UTC); !got.occurredAt.Equal(want) {
		t.Errorf("occurredAt = %v, want %v", got.occurredAt, want)
	}
}

func TestSubscriptionPlanUpdated_UnlimitedAndOtherLimitTypesMapToNoLimit(t *testing.T) {
	r, plans, cfg := newPlanTestReceiver(t)

	_, err := postPlanEvent(t, r, cfg, EventSubscriptionPlanUpdated,
		`{"handle":"unlimited","display_name":"Unlimited","limits":[{"limit_type":"REQUEST_COUNT","limit_count":-1,"time_unit":null},{"limit_type":"EVENT_COUNT","limit_count":10,"time_unit":"MINUTE"}]}`)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got := plans.upserts[0]; len(got.windows) != 0 {
		t.Errorf("windows = %+v, want none", got.windows)
	}
}

func TestSubscriptionPlanUpserted_RejectsMissingOrOverlongHandle(t *testing.T) {
	r, plans, cfg := newPlanTestReceiver(t)

	for _, data := range []string{
		`{"display_name":"No handle"}`,
		`{"handle":"this-handle-is-much-longer-than-forty-characters","display_name":"Long"}`,
		`{"handle":"gold","display_name":"  "}`,
	} {
		_, err := postPlanEvent(t, r, cfg, EventSubscriptionPlanCreated, data)
		var appErr *apperror.Error
		if !errors.As(err, &appErr) || appErr.Code != apperror.CodeCommonValidationFailed {
			t.Errorf("data %s: want a validation error, got %v", data, err)
		}
	}
	if len(plans.upserts) != 0 {
		t.Errorf("invalid events must not reach the service, got %d upserts", len(plans.upserts))
	}
}

// paidPlan is the payload of subscription_plan.created for the plan gold with the given pricing.
func paidPlan(pricing string) string {
	return `{"handle":"gold","display_name":"Gold","limits":[],"pricing":` + pricing + `}`
}

func TestSubscriptionPlanCreated_CopiesUsagePrices(t *testing.T) {
	r, plans, cfg := newPlanTestReceiver(t)

	_, err := postPlanEvent(t, r, cfg, EventSubscriptionPlanCreated, paidPlan(
		`{"pricing_type":"usage_based","currency":"usd","billing_period":"MONTH","flat_amount":null,"prices":[
		  {"meter_handle":"llm-input","filters":[{"field":" API type ","operator":"equals","values":["LlmProvider"]}],
		   "pricing_model":"per_unit","unit_amount":"2.50","unit_quantity":1000000,"tiers":null},
		  {"meter_handle":"llm-input","filters":[{"field":"API type","operator":"IN","values":["Mcp","LlmProxy"]}],
		   "pricing_model":"PER_UNIT","unit_amount":"0.15","unit_quantity":1000000,"tiers":null},
		  {"meter_handle":"api-calls","filters":[],"pricing_model":"GRADUATED",
		   "unit_amount":null,"unit_quantity":null,
		   "tiers":[{"up_to":1000,"unit_amount":"0","flat_amount":null},{"up_to":null,"unit_amount":"0.001","flat_amount":null}]}]}`))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	got := plans.upserts[0]
	if got.handle != "gold" || got.pricing == nil {
		t.Fatalf("upsert = %+v, want gold with its pricing", got)
	}
	p := got.pricing
	if p.OrganizationUUID != planTestOrgUUID || p.PricingType != "USAGE_BASED" || p.Currency != "USD" || p.FlatAmount != nil ||
		len(p.Prices) != 3 {
		t.Fatalf("pricing = %+v", p)
	}
	if want := time.Date(2026, 10, 11, 9, 30, 0, 0, time.UTC); !p.CreatedAt.Equal(want) {
		t.Errorf("createdAt = %v, want %v", p.CreatedAt, want)
	}
	tokens, calls := p.Prices[0], p.Prices[2]
	if tokens.MeterHandle != "llm-input" || tokens.PricingModel != "PER_UNIT" || *tokens.UnitQuantity != 1000000 || tokens.Tiers != nil ||
		!reflect.DeepEqual(tokens.Filters, []model.PriceFilter{{Field: "API type", Operator: "EQUALS", Values: []string{"LlmProvider"}}}) {
		t.Errorf("first price = %+v", tokens)
	}
	if mini := p.Prices[1]; mini.MeterHandle != "llm-input" || len(mini.Filters) != 1 || len(mini.Filters[0].Values) != 2 {
		t.Errorf("second price = %+v", mini)
	}
	if calls.MeterHandle != "api-calls" || len(calls.Tiers) != 2 || *calls.Tiers[0].UpTo != 1000 || calls.Tiers[1].UpTo != nil ||
		*calls.Tiers[1].UnitAmount != "0.001" {
		t.Errorf("third price = %+v", calls)
	}
}

// A filter's property name and values are kept as written; names are matched ignoring case, so the
// two prices filter on one property and count different requests.
func TestSubscriptionPlanCreated_CopiesFiltersAsWritten(t *testing.T) {
	r, plans, cfg := newPlanTestReceiver(t)

	if _, err := postPlanEvent(t, r, cfg, EventSubscriptionPlanCreated, paidPlan(
		`{"pricing_type":"USAGE_BASED","currency":"USD","billing_period":"MONTH","prices":[
		  {"meter_handle":"api-calls","filters":[{"field":"api type","operator":"equals","values":["RestApi"]}],
		   "pricing_model":"PER_UNIT","unit_amount":"0.01","unit_quantity":1},
		  {"meter_handle":"api-calls","filters":[{"field":"API type","operator":"NOT_EQUALS","values":["RestApi"]}],
		   "pricing_model":"PER_UNIT","unit_amount":"0.02","unit_quantity":1}]}`)); err != nil {
		t.Fatalf("err = %v", err)
	}
	p := plans.upserts[0].pricing
	if !reflect.DeepEqual(p.Prices[0].Filters, []model.PriceFilter{{Field: "api type", Operator: "EQUALS", Values: []string{"RestApi"}}}) ||
		!reflect.DeepEqual(p.Prices[1].Filters, []model.PriceFilter{{Field: "API type", Operator: "NOT_EQUALS", Values: []string{"RestApi"}}}) {
		t.Errorf("prices = %+v", p.Prices)
	}
}

// Comparisons are copied with their numbers as plain whole numbers.
func TestSubscriptionPlanCreated_CopiesComparisons(t *testing.T) {
	r, plans, cfg := newPlanTestReceiver(t)

	if _, err := postPlanEvent(t, r, cfg, EventSubscriptionPlanCreated, paidPlan(
		`{"pricing_type":"USAGE_BASED","currency":"USD","billing_period":"MONTH","prices":[
		  {"meter_handle":"llm-input","filters":[
		    {"field":"Prompt tokens","operator":"greater_than","values":["100"]},
		    {"field":"Prompt tokens","operator":"LESS_THAN_OR_EQUAL","values":["0500"]}],
		   "pricing_model":"PER_UNIT","unit_amount":"2.50","unit_quantity":1000000}]}`)); err != nil {
		t.Fatalf("err = %v", err)
	}
	want := []model.PriceFilter{
		{Field: "Prompt tokens", Operator: "GREATER_THAN", Values: []string{"100"}},
		{Field: "Prompt tokens", Operator: "LESS_THAN_OR_EQUAL", Values: []string{"500"}},
	}
	if got := plans.upserts[0].pricing.Prices[0].Filters; !reflect.DeepEqual(got, want) {
		t.Errorf("filters = %+v, want %+v", got, want)
	}
}

func TestSubscriptionPlanCreated_CopiesAFlatRate(t *testing.T) {
	r, plans, cfg := newPlanTestReceiver(t)

	if _, err := postPlanEvent(t, r, cfg, EventSubscriptionPlanCreated, paidPlan(
		`{"pricing_type":"FLAT_RATE","currency":"USD","billing_period":"YEAR","flat_amount":"990.00","prices":[]}`)); err != nil {
		t.Fatalf("err = %v", err)
	}
	p := plans.upserts[0].pricing
	if p.PricingType != "FLAT_RATE" || p.BillingPeriod != "YEAR" || p.FlatAmount == nil || *p.FlatAmount != "990.00" ||
		p.Prices == nil || len(p.Prices) != 0 {
		t.Errorf("pricing = %+v", p)
	}
}

// An update carries the plan's pricing too, which never changes, so a missed created event is
// recovered by the next update.
func TestSubscriptionPlanUpdated_PassesItsPricing(t *testing.T) {
	r, plans, cfg := newPlanTestReceiver(t)

	if _, err := postPlanEvent(t, r, cfg, EventSubscriptionPlanUpdated, paidPlan(
		`{"pricing_type":"FLAT_RATE","currency":"USD","billing_period":"MONTH","flat_amount":"9.00","prices":[]}`)); err != nil {
		t.Fatalf("err = %v", err)
	}
	if got := plans.upserts[0].pricing; got == nil || got.FlatAmount == nil || *got.FlatAmount != "9.00" {
		t.Errorf("pricing = %+v, want the flat rate 9.00", got)
	}
}

func TestSubscriptionPlanCreated_RejectsMalformedPricing(t *testing.T) {
	r, plans, cfg := newPlanTestReceiver(t)

	usage := func(prices string) string {
		return paidPlan(`{"pricing_type":"USAGE_BASED","currency":"USD","billing_period":"MONTH","prices":[` + prices + `]}`)
	}
	for _, tc := range []struct{ data, reason string }{
		{usage(`{"meter_handle":"calls","pricing_model":"PER_UNIT","unit_amount":"0.01","filters":[{"field":"Total","operator":"GREATER_THAN","values":["1.5"]}]}`),
			"price 1: A comparison on Total takes a whole number from 0 to 1000000000000"},
		{usage(`{"meter_handle":"calls","pricing_model":"PER_UNIT","unit_amount":"0.01","filters":[{"field":"Total","operator":"GREATER_THAN","values":["9"]},{"field":"total","operator":"LESS_THAN","values":["5"]}]}`),
			"price 1: the comparisons on Total can't all be true"},
		{paidPlan(`{"currency":"USD","billing_period":"MONTH","flat_amount":"1"}`),
			"pricingType must be one of FLAT_RATE, USAGE_BASED"},
		{paidPlan(`{"pricing_type":"FLAT_RATE","currency":"US","billing_period":"MONTH","flat_amount":"1"}`),
			"currency must be the code of a currency with two decimal places, such as USD or EUR. Currencies with no or three decimal places cannot be used."},
		{paidPlan(`{"pricing_type":"FLAT_RATE","currency":"USD","billing_period":"MONTH","flat_amount":"0"}`),
			"flatAmount must be more than 0"},
		{paidPlan(`{"pricing_type":"FLAT_RATE","currency":"USD","billing_period":"MONTH","flat_amount":"9","prices":[{"meter_handle":"calls","pricing_model":"PER_UNIT","unit_amount":"0.01"}]}`),
			"A flat rate plan has no usage prices"},
		{paidPlan(`{"pricing_type":"USAGE_BASED","currency":"USD","billing_period":"MONTH","flat_amount":"9","prices":[{"meter_handle":"calls","pricing_model":"PER_UNIT","unit_amount":"0.01"}]}`),
			"A usage based plan has no flat amount"},
		{usage(``), "A usage based plan needs 1 to 20 prices"},
		{usage(`{"meter_handle":"calls","pricing_model":"PER_UNIT","unit_amount":"0.01"},{"meter_handle":"calls","pricing_model":"PER_UNIT","unit_amount":"0.02","filters":[{"field":"API type","operator":"EQUALS","values":["LlmProvider"]}]}`),
			"Prices 1 and 2 both count the same requests of the meter calls; give them different filters."},
		{usage(`{"meter_handle":"calls","pricing_model":"PER_UNIT","unit_amount":"0.01","filters":[{"field":"Resource","operator":"MATCHES","values":["/a"]}]}`),
			"The operator of filter 1 of price 1 must be one of EQUALS, NOT_EQUALS, IN, NOT_IN, STARTS_WITH, GREATER_THAN, GREATER_THAN_OR_EQUAL, LESS_THAN, LESS_THAN_OR_EQUAL"},
		{usage(`{"meter_handle":"calls","pricing_model":"PER_UNIT","unit_amount":"0.01","filters":[{"field":"Status code","operator":"IN","values":["200"]},{"field":"status code","operator":"LESS_THAN","values":["300"]}]}`),
			"price 1 filters on Status code more than once"},
		{usage(`{"meter_handle":"calls","pricing_model":"FLAT"}`), "pricingModel of price 1 must be one of PER_UNIT, VOLUME, GRADUATED"},
		{usage(`{"pricing_model":"PER_UNIT","unit_amount":"0.01"}`), "meterId of price 1 is required"},
		{usage(`{"meter_handle":"calls","pricing_model":"PER_UNIT","unit_amount":"0.01","tiers":[]}`),
			"A PER_UNIT price (price 1) does not take tiers"},
		{usage(`{"meter_handle":"calls","pricing_model":"VOLUME","tiers":[{"up_to":100,"unit_amount":"1"}]}`),
			"The last tier of price 1 must have no upper bound (upTo null)"},
		{usage(`{"meter_handle":"calls","pricing_model":"VOLUME","tiers":[{"up_to":100,"unit_amount":"1"},{"up_to":50,"unit_amount":"1"},{"up_to":null,"unit_amount":"1"}]}`),
			"Each tier of price 1 needs an upTo greater than the tier before it"},
		{usage(`{"meter_handle":"calls","pricing_model":"VOLUME","tiers":[{"up_to":null}]}`),
			"Tier 1 of price 1 needs a unitAmount, a flatAmount or both"},
	} {
		_, err := postPlanEvent(t, r, cfg, EventSubscriptionPlanCreated, tc.data)
		var appErr *apperror.Error
		if !errors.As(err, &appErr) || appErr.Code != apperror.CodeCommonValidationFailed || !strings.Contains(err.Error(), tc.reason) {
			t.Errorf("%s: want a validation error for %q, got %v", tc.data, tc.reason, err)
		}
	}
	if len(plans.upserts) != 0 {
		t.Errorf("a plan with invalid pricing must not reach the service, got %d", len(plans.upserts))
	}
}

func TestSubscriptionPlanDeleted_DeletesByHandle(t *testing.T) {
	r, plans, cfg := newPlanTestReceiver(t)

	rec, err := postPlanEvent(t, r, cfg, EventSubscriptionPlanDeleted, `{"handle":"silver"}`)
	if err != nil || rec.Code != http.StatusOK {
		t.Fatalf("status = %d, err = %v", rec.Code, err)
	}
	if len(plans.deletes) != 1 || plans.deletes[0] != planTestOrgUUID+"/silver" {
		t.Errorf("deletes = %v", plans.deletes)
	}
	// The deletion carries the time the API Portal made it.
	if want := time.Date(2026, 10, 11, 9, 30, 0, 0, time.UTC); !plans.deleteOccurredAt.Equal(want) {
		t.Errorf("occurredAt = %v, want %v", plans.deleteOccurredAt, want)
	}
}

// Every finite request limit with a supported unit is a window, with its time amount; a second
// limit over the same length of time is left out.
func TestRequestWindows_KeepsEverySupportedRequestLimit(t *testing.T) {
	got := requestWindows([]subscriptionPlanLimit{
		{LimitType: "REQUEST_COUNT", LimitCount: 100, TimeUnit: "SECOND"},
		{LimitType: "REQUEST_COUNT", LimitCount: 2000, TimeUnit: "HOUR", TimeAmount: 2},
		{LimitType: "request_count", LimitCount: 50, TimeUnit: "minute"},
		{LimitType: "REQUEST_COUNT", LimitCount: 3000, TimeUnit: "MINUTE", TimeAmount: 120},
		{LimitType: "BANDWIDTH", LimitCount: 10, TimeUnit: "DAY"},
		{LimitType: "REQUEST_COUNT", LimitCount: 9000, TimeUnit: "MONTH"},
		{LimitType: "REQUEST_COUNT", LimitCount: 300, TimeUnit: "DAY", TimeAmount: 1},
	})
	want := []model.ConditionWindow{{Count: 2000, TimeAmount: 2, TimeUnit: "HOUR"}, {Count: 50, TimeAmount: 1, TimeUnit: "MINUTE"},
		{Count: 9000, TimeAmount: 1, TimeUnit: "MONTH"}, {Count: 300, TimeAmount: 1, TimeUnit: "DAY"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("windows = %+v, want %+v", got, want)
	}
}

// A plan holds at most ten windows: an older portal's later limits are left out.
func TestRequestWindows_AtMostTen(t *testing.T) {
	var limits []subscriptionPlanLimit
	for i := 1; i <= 12; i++ {
		limits = append(limits, subscriptionPlanLimit{LimitType: "REQUEST_COUNT", LimitCount: i, TimeUnit: "MINUTE", TimeAmount: i})
	}
	if got := requestWindows(limits); len(got) != 10 || got[9].Count != 10 {
		t.Fatalf("windows = %+v, want the first ten", got)
	}
}

// A portal that sends conditions has them passed on as they are, with the plan's description.
func TestSubscriptionPlanCreated_PassesConditionsAndDescription(t *testing.T) {
	r, plans, cfg := newPlanTestReceiver(t)

	_, err := postPlanEvent(t, r, cfg, EventSubscriptionPlanCreated, `{"handle":"agents","display_name":"Agents",
		"description":"For AI agents","limits":[],
		"conditions":[{"type":"tool-call-count","params":{"limits":[{"count":10,"timeUnit":"MINUTE"}]}},
			{"id":"ip-ranges","type":"custom","params":{"cidr":"10.0.0.0/8"}}],"pricing":null}`)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	got := plans.upserts[0]
	if got.description != "For AI agents" || got.conditions == nil || len(*got.conditions) != 2 ||
		(*got.conditions)[0].Type != model.ConditionTypeToolCallCount || (*got.conditions)[1].ID != "ip-ranges" {
		t.Fatalf("upsert = %+v, want the description and both conditions", got)
	}
}

// "conditions": null is the same as no conditions: the limits give the request windows.
func TestSubscriptionPlanCreated_NullConditionsUseTheLimits(t *testing.T) {
	r, plans, cfg := newPlanTestReceiver(t)

	_, err := postPlanEvent(t, r, cfg, EventSubscriptionPlanCreated, `{"handle":"gold","display_name":"Gold","conditions":null,
		"limits":[{"limit_type":"REQUEST_COUNT","limit_count":10,"time_unit":"DAY","time_amount":1}],"pricing":null}`)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got := plans.upserts[0]; got.conditions != nil || len(got.windows) != 1 || got.windows[0].TimeUnit != "DAY" {
		t.Fatalf("upsert = %+v, want only the request window", got)
	}
}
