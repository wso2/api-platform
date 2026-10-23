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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/model"
)

// planPortal is a fake API Portal serving the subscription plan collection.
type planPortal struct {
	handles      []string      // the portal's plans, in listing order
	total        int           // reported pagination total; len(handles) when zero
	listStatus   int           // overrides the listing's 200 when non-zero
	createStatus int           // the bulk create's status; 201 when zero
	listBody     string        // replaces the listing body when non-empty
	listDelay    time.Duration // how long each listing page takes to answer
	// listed sets a plan's entry in the portal's listing; by default each plan is listed as a
	// copy of testPlan: its name, no limits and no pricing.
	listed       map[string]map[string]any
	updateStatus int // the bulk update's status; 200 when zero
	// meters are the portal's meters by id, as its REST API gives them; meterCreateStatus is a
	// meter create's status (201 when zero), deleteStatus a delete's by path (204 when absent).
	meters            map[string]map[string]any
	meterCreateStatus int
	deleteStatus      map[string]int

	queries      []string
	postedMeters []portalMeter
	deleted      []string
	auths        []string
	posted       [][]portalPlan
	updated      [][]portalPlanUpdate
}

// syncPlansWithoutMeters syncs plans and their pricing, with no meters to send, and returns the
// error.
func syncPlansWithoutMeters(ctx context.Context, p PortalPublisher, portal *model.APIPortal, plans []*model.SubscriptionPlan,
	pricing map[string]*model.SubscriptionPlanPricing) error {
	_, err := p.SyncPlans(ctx, portal, PlanSync{Plans: plans, Pricing: pricing})
	return err
}

func (pp *planPortal) serve(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if pp.serveMetersAndDeletes(t, w, r) {
			return
		}
		if r.URL.Path != portalRESTBase+"/subscription-plans" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		pp.auths = append(pp.auths, r.Header.Get("Authorization"))
		switch r.Method {
		case http.MethodGet:
			pp.list(w, r)
		case http.MethodPost:
			var plans []portalPlan
			if err := json.NewDecoder(r.Body).Decode(&plans); err != nil {
				t.Fatalf("decode bulk create body: %v", err)
			}
			pp.posted = append(pp.posted, plans)
			status := pp.createStatus
			if status == 0 {
				status = http.StatusCreated
			}
			w.WriteHeader(status)
		case http.MethodPut:
			var plans []portalPlanUpdate
			if err := json.NewDecoder(r.Body).Decode(&plans); err != nil {
				t.Fatalf("decode bulk update body: %v", err)
			}
			pp.updated = append(pp.updated, plans)
			status := pp.updateStatus
			if status == 0 {
				status = http.StatusOK
			}
			w.WriteHeader(status)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// serveMetersAndDeletes answers the meter reads and creates and every delete; false for any other
// request.
func (pp *planPortal) serveMetersAndDeletes(t *testing.T, w http.ResponseWriter, r *http.Request) bool {
	t.Helper()
	metersPath := portalRESTBase + "/meters"
	switch {
	case r.Method == http.MethodDelete:
		pp.deleted = append(pp.deleted, strings.TrimPrefix(r.URL.Path, portalRESTBase))
		status, ok := pp.deleteStatus[strings.TrimPrefix(r.URL.Path, portalRESTBase)]
		if !ok {
			status = http.StatusNoContent
		}
		w.WriteHeader(status)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, metersPath+"/"):
		meter, ok := pp.meters[strings.TrimPrefix(r.URL.Path, metersPath+"/")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return true
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(meter)
	case r.Method == http.MethodPost && r.URL.Path == metersPath:
		var meter portalMeter
		if err := json.NewDecoder(r.Body).Decode(&meter); err != nil {
			t.Fatalf("decode meter create body: %v", err)
		}
		pp.postedMeters = append(pp.postedMeters, meter)
		status := pp.meterCreateStatus
		if status == 0 {
			status = http.StatusCreated
		}
		w.WriteHeader(status)
	default:
		return false
	}
	return true
}

func (pp *planPortal) list(w http.ResponseWriter, r *http.Request) {
	pp.queries = append(pp.queries, r.URL.RawQuery)
	time.Sleep(pp.listDelay)
	if pp.listStatus != 0 {
		w.WriteHeader(pp.listStatus)
		return
	}
	if pp.listBody != "" {
		_, _ = w.Write([]byte(pp.listBody))
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	end := min(offset+limit, len(pp.handles))
	page := []map[string]any{}
	if offset < end {
		for _, h := range pp.handles[offset:end] {
			entry, ok := pp.listed[h]
			if !ok {
				entry = map[string]any{"id": h, "displayName": "Plan " + h, "limits": []any{}}
			}
			page = append(page, entry)
		}
	}
	total := pp.total
	if total == 0 {
		total = len(pp.handles)
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"count":      len(page),
		"list":       page,
		"pagination": map[string]int{"limit": limit, "offset": offset, "total": total},
	})
}

func pageQuery(offset int) string {
	return fmt.Sprintf("limit=%d&offset=%d", portalPlansPageSize, offset)
}

func testPlan(handle string) *model.SubscriptionPlan {
	return &model.SubscriptionPlan{UUID: "uuid-" + handle, Handle: handle, Name: "Plan " + handle}
}

func testPlans(handles ...string) []*model.SubscriptionPlan {
	plans := make([]*model.SubscriptionPlan, 0, len(handles))
	for _, h := range handles {
		plans = append(plans, testPlan(h))
	}
	return plans
}

func numberedHandles(n int) []string {
	handles := make([]string, n)
	for i := range handles {
		handles[i] = fmt.Sprintf("plan-%03d", i+1)
	}
	return handles
}

func postedIDs(pp *planPortal) []string {
	var ids []string
	for _, batch := range pp.posted {
		for _, p := range batch {
			ids = append(ids, p.ID)
		}
	}
	return ids
}

func createMissing(t *testing.T, pp *planPortal, plans []*model.SubscriptionPlan) error {
	t.Helper()
	srv := pp.serve(t)
	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	return syncPlansWithoutMeters(context.Background(), p, &model.APIPortal{URL: srv.URL}, plans, nil)
}

func TestSyncPlans_NoPlansMakesNoRequest(t *testing.T) {
	pp := &planPortal{}
	if err := createMissing(t, pp, nil); err != nil {
		t.Fatalf("SyncPlans: %v", err)
	}
	if len(pp.queries)+len(pp.posted) != 0 {
		t.Fatalf("want no requests, got %d listings and %d creates", len(pp.queries), len(pp.posted))
	}
}

func TestSyncPlans_NothingMissingSkipsCreate(t *testing.T) {
	pp := &planPortal{handles: []string{"gold", "silver", "bronze"}}
	if err := createMissing(t, pp, testPlans("gold", "silver")); err != nil {
		t.Fatalf("SyncPlans: %v", err)
	}
	if len(pp.posted) != 0 {
		t.Fatalf("want no create, got %v", postedIDs(pp))
	}
	if len(pp.queries) != 1 || pp.queries[0] != pageQuery(0) {
		t.Fatalf("want one listing of the first page, got %v", pp.queries)
	}
}

func TestSyncPlans_CreatesOnlyMissingInInputOrder(t *testing.T) {
	pp := &planPortal{handles: []string{"silver"}}
	if err := createMissing(t, pp, testPlans("zeta", "silver", "alpha")); err != nil {
		t.Fatalf("SyncPlans: %v", err)
	}
	if got := strings.Join(postedIDs(pp), ","); got != "zeta,alpha" || len(pp.posted) != 1 {
		t.Fatalf("want one create of zeta,alpha, got %d creates of %q", len(pp.posted), got)
	}
	for i, auth := range pp.auths {
		if auth == "" {
			t.Fatalf("request %d carried no Authorization header", i)
		}
	}
}

func TestSyncPlans_PagesUntilTotal(t *testing.T) {
	handles := numberedHandles(portalPlansPageSize + 5)
	pp := &planPortal{handles: handles}
	if err := createMissing(t, pp, testPlans(handles[len(handles)-1], "brand-new")); err != nil {
		t.Fatalf("SyncPlans: %v", err)
	}
	want := []string{pageQuery(0), pageQuery(portalPlansPageSize)}
	if strings.Join(pp.queries, "|") != strings.Join(want, "|") {
		t.Fatalf("want listings %v, got %v", want, pp.queries)
	}
	if got := postedIDs(pp); len(got) != 1 || got[0] != "brand-new" {
		t.Fatalf("want only brand-new created, got %v", got)
	}
}

func TestSyncPlans_StopsPagingOnceAllFound(t *testing.T) {
	pp := &planPortal{handles: numberedHandles(2 * portalPlansPageSize)}
	if err := createMissing(t, pp, testPlans("plan-001")); err != nil {
		t.Fatalf("SyncPlans: %v", err)
	}
	if len(pp.queries) != 1 || len(pp.posted) != 0 {
		t.Fatalf("want one listing and no create, got %v and %v", pp.queries, postedIDs(pp))
	}
}

func TestSyncPlans_StopsOnEmptyPage(t *testing.T) {
	pp := &planPortal{handles: numberedHandles(portalPlansPageSize), total: portalPlansPageSize + 30}
	if err := createMissing(t, pp, testPlans("brand-new")); err != nil {
		t.Fatalf("SyncPlans: %v", err)
	}
	if len(pp.queries) != 2 {
		t.Fatalf("want listings of page one and the empty page two, got %v", pp.queries)
	}
	if got := postedIDs(pp); len(got) != 1 || got[0] != "brand-new" {
		t.Fatalf("want brand-new created, got %v", got)
	}
}

func TestSyncPlans_PageCapIsAnError(t *testing.T) {
	pp := &planPortal{handles: numberedHandles(portalPlansPageSize), total: 1 << 30}
	// Every page repeats the same plans, so the wanted one is never found.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pp.queries = append(pp.queries, r.URL.RawQuery)
		items := make([]map[string]string, 0, portalPlansPageSize)
		for _, h := range pp.handles {
			items = append(items, map[string]string{"id": h})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"list": items, "pagination": map[string]int{"total": pp.total}})
	}))
	defer srv.Close()

	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	err := syncPlansWithoutMeters(context.Background(), p, &model.APIPortal{URL: srv.URL}, testPlans("never-listed"), nil)
	if err == nil {
		t.Fatal("want an error when the listing never ends")
	}
	var conflict *PortalConflictError
	if errors.As(err, &conflict) {
		t.Fatalf("a page cap must not look like a conflict, got %v", err)
	}
	if len(pp.queries) != portalPlansMaxPages {
		t.Fatalf("want %d listings, got %d", portalPlansMaxPages, len(pp.queries))
	}
}

func TestSyncPlans_MatchesHandlesExactly(t *testing.T) {
	pp := &planPortal{handles: []string{"Gold"}}
	if err := createMissing(t, pp, testPlans("gold")); err != nil {
		t.Fatalf("SyncPlans: %v", err)
	}
	if got := postedIDs(pp); len(got) != 1 || got[0] != "gold" {
		t.Fatalf("want gold created next to Gold, got %v", got)
	}
}

func TestSyncPlans_MapsPlanFields(t *testing.T) {
	limited := testPlan("limited")
	count := 1000
	limited.ThrottleLimitCount = &count
	limited.ThrottleLimitUnit = "HOUR"
	unlimited := testPlan("unlimited")

	pp := &planPortal{}
	if err := createMissing(t, pp, []*model.SubscriptionPlan{limited, unlimited}); err != nil {
		t.Fatalf("SyncPlans: %v", err)
	}
	if len(pp.posted) != 1 || len(pp.posted[0]) != 2 {
		t.Fatalf("want one create of two plans, got %v", pp.posted)
	}
	got := pp.posted[0]
	wantLimit := portalPlanLimit{LimitType: "REQUEST_COUNT", LimitCount: 1000, TimeUnit: "HOUR", TimeAmount: 1}
	if got[0].ID != "limited" || got[0].RefID != "limited" || got[0].DisplayName != "Plan limited" ||
		len(got[0].Limits) != 1 || got[0].Limits[0] != wantLimit {
		t.Fatalf("limited plan mapped wrong: %+v", got[0])
	}
	if got[1].ID != "unlimited" || len(got[1].Limits) != 0 {
		t.Fatalf("a plan with no limit must be sent without limits: %+v", got[1])
	}
}

func TestSyncPlans_OmitsLimitsKeyForUnlimitedPlan(t *testing.T) {
	body, err := json.Marshal(toPortalPlan(testPlan("unlimited"), nil))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "limits") {
		t.Fatalf("want no limits key, got %s", body)
	}
}

// A paid plan the portal lacks is sent with its pricing in the same create, a free one without. A
// paid plan the portal already has, with the same pricing written differently, is not sent.
func TestSyncPlans_SendsPricingWithPaidPlans(t *testing.T) {
	var raw []map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"list": []map[string]any{{"id": "gold", "displayName": "Plan gold", "limits": []any{}, "pricing": map[string]any{
					"pricingType": "FLAT_RATE", "currency": "usd", "billingPeriod": "MONTH", "flatAmount": "99", "prices": []any{},
				}}},
				"pagination": map[string]int{"limit": portalPlansPageSize, "offset": 0, "total": 1},
			})
		case http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
				t.Fatalf("decode bulk create body: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	unit, fee, quantity, upTo := "0.01", "99.00", int64(1000), int64(100)
	plans := testPlans("free", "starter", "silver", "gold")
	pricing := map[string]*model.SubscriptionPlanPricing{
		"uuid-starter": {PricingType: model.PricingTypeUsageBased, Currency: "USD", BillingPeriod: "MONTH", Prices: []model.SubscriptionPlanPrice{
			{MeterHandle: "llm-input", PricingModel: model.PricingModelPerUnit, UnitAmount: &unit, UnitQuantity: &quantity,
				Filters: []model.PriceFilter{{Field: "API_TYPE", Operator: "EQUALS", Values: []string{"LlmProvider"}}}},
			{MeterHandle: "api-calls", PricingModel: model.PricingModelGraduated,
				Tiers: []model.PriceTier{{UpTo: &upTo, UnitAmount: &unit}, {UnitAmount: &unit}}},
		}},
		"uuid-silver": {PricingType: model.PricingTypeFlatRate, Currency: "USD", BillingPeriod: "YEAR", FlatAmount: &fee,
			Prices: []model.SubscriptionPlanPrice{}},
		"uuid-gold": {PricingType: model.PricingTypeFlatRate, Currency: "USD", BillingPeriod: "MONTH", FlatAmount: &fee,
			Prices: []model.SubscriptionPlanPrice{}},
	}
	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	if err := syncPlansWithoutMeters(context.Background(), p, &model.APIPortal{URL: srv.URL}, plans, pricing); err != nil {
		t.Fatalf("SyncPlans: %v", err)
	}
	if len(raw) != 3 {
		t.Fatalf("want free, starter and silver sent, got %d plans", len(raw))
	}
	if _, ok := raw[0]["pricing"]; ok || string(raw[0]["id"]) != `"free"` {
		t.Fatalf("the free plan goes without pricing, got %s %s", raw[0]["id"], raw[0]["pricing"])
	}
	if string(raw[1]["id"]) != `"starter"` || string(raw[2]["id"]) != `"silver"` {
		t.Fatalf("plans sent = %s, %s", raw[1]["id"], raw[2]["id"])
	}
	// The API Portal's REST shape: camelCase, a field the pricing does not use left out, the last
	// tier's open upTo sent as null.
	want := `{"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[` +
		`{"meterId":"llm-input","filters":[{"field":"API_TYPE","operator":"EQUALS","values":["LlmProvider"]}],` +
		`"pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1000},` +
		`{"meterId":"api-calls","filters":[],"pricingModel":"GRADUATED","tiers":[` +
		`{"upTo":100,"unitAmount":"0.01","flatAmount":null},{"upTo":null,"unitAmount":"0.01","flatAmount":null}]}]}`
	if got := string(raw[1]["pricing"]); got != want {
		t.Fatalf("starter's pricing =\n%s\nwant\n%s", got, want)
	}
	want = `{"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"YEAR","flatAmount":"99.00"}`
	if got := string(raw[2]["pricing"]); got != want {
		t.Fatalf("silver's pricing =\n%s\nwant\n%s", got, want)
	}
}

func TestSyncPlans_ErrorMapping(t *testing.T) {
	tests := []struct {
		name         string
		portal       planPortal
		wantConflict bool
	}{
		{"create 409", planPortal{createStatus: http.StatusConflict}, true},
		{"create 400", planPortal{createStatus: http.StatusBadRequest}, true},
		{"create 401", planPortal{createStatus: http.StatusUnauthorized}, false},
		{"create 403", planPortal{createStatus: http.StatusForbidden}, false},
		{"create 500", planPortal{createStatus: http.StatusInternalServerError}, false},
		{"create 200 is not 201", planPortal{createStatus: http.StatusOK}, false},
		{"list 400", planPortal{listStatus: http.StatusBadRequest}, true},
		{"list 401", planPortal{listStatus: http.StatusUnauthorized}, false},
		{"list 403", planPortal{listStatus: http.StatusForbidden}, false},
		{"list 500", planPortal{listStatus: http.StatusInternalServerError}, false},
		{"list unreadable body", planPortal{listBody: "not json"}, false},
		{"list oversized body", planPortal{listBody: `{"list":[],"pad":"` + strings.Repeat("x", portalPlansMaxResponseBytes) + `"}`}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pp := tc.portal
			err := createMissing(t, &pp, testPlans("brand-new"))
			if err == nil {
				t.Fatal("want an error")
			}
			var conflict *PortalConflictError
			if got := errors.As(err, &conflict); got != tc.wantConflict {
				t.Fatalf("want conflict=%v, got %v (%v)", tc.wantConflict, got, err)
			}
			if tc.wantConflict && conflict.Reason != defaultPortalConflictReason {
				t.Fatalf("want the generic reason, got %q", conflict.Reason)
			}
		})
	}
}

func TestSyncPlans_RequestFailures(t *testing.T) {
	plans := testPlans("brand-new")

	t.Run("credential cannot be resolved", func(t *testing.T) {
		srv := (&planPortal{}).serve(t)
		p := newTestHTTPPortalPublisher(t, "test-shared-key")
		p.authRegistry = NewAPIPortalAuthRegistry(&mockAPIPortalRepository{getErr: errors.New("portal row unreadable")}, newTestVault(t))
		portal := &model.APIPortal{URL: srv.URL}

		if err := syncPlansWithoutMeters(context.Background(), p, portal, plans, nil); err == nil || !strings.Contains(err.Error(), "failed to resolve API Portal auth") {
			t.Fatalf("listing: want the credential error, got %v", err)
		}
		if err := p.createPlans(context.Background(), portal, plans, nil); err == nil || !strings.Contains(err.Error(), "failed to resolve API Portal auth") {
			t.Fatalf("create: want the credential error, got %v", err)
		}
	})

	t.Run("portal URL is not a valid URL", func(t *testing.T) {
		p := newTestHTTPPortalPublisher(t, "test-shared-key")
		portal := &model.APIPortal{URL: "http://bad host"}

		if _, err := p.listPlansPage(context.Background(), portal, 0); err == nil || !strings.Contains(err.Error(), "failed to build portal plan listing request") {
			t.Fatalf("listing: want a build error, got %v", err)
		}
		if err := p.createPlans(context.Background(), portal, plans, nil); err == nil || !strings.Contains(err.Error(), "failed to build portal plan create request") {
			t.Fatalf("create: want a build error, got %v", err)
		}
	})

	t.Run("connection drops during the create", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`{"list":[],"pagination":{"total":0}}`))
				return
			}
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("hijack: %v", err)
				return
			}
			conn.Close()
		}))
		t.Cleanup(srv.Close)
		p := newTestHTTPPortalPublisher(t, "test-shared-key")

		err := syncPlansWithoutMeters(context.Background(), p, &model.APIPortal{URL: srv.URL}, plans, nil)
		var conflict *PortalConflictError
		if err == nil || errors.As(err, &conflict) || !strings.Contains(err.Error(), "portal plan create failed") {
			t.Fatalf("want a plain create failure, got %v", err)
		}
	})
}

// A listing that spans many slow pages ends at the caller's deadline, as an ordinary failure
// and not as a conflict, and the create is never sent.
func TestSyncPlans_SlowMultiPageListingStopsAtTheDeadline(t *testing.T) {
	const pageDelay = 40 * time.Millisecond
	pp := &planPortal{handles: numberedHandles(20 * portalPlansPageSize), listDelay: pageDelay}
	srv := pp.serve(t)
	p := newTestHTTPPortalPublisher(t, "test-shared-key")

	ctx, cancel := context.WithTimeout(context.Background(), 3*pageDelay)
	defer cancel()
	err := syncPlansWithoutMeters(ctx, p, &model.APIPortal{URL: srv.URL}, testPlans("never-listed"), nil)
	srv.Close() // waits for the handler still answering the cancelled page, so the recorded queries are stable

	var conflict *PortalConflictError
	if !errors.Is(err, context.DeadlineExceeded) || errors.As(err, &conflict) {
		t.Fatalf("want a plain deadline failure, got %v", err)
	}
	if len(pp.queries) < 2 || len(pp.queries) >= 20 {
		t.Fatalf("want the listing cut off part-way through, got %d pages", len(pp.queries))
	}
	if len(pp.posted) != 0 {
		t.Fatalf("want no create after a failed listing, got %v", postedIDs(pp))
	}
}

// The plan listing has a time budget of its own, however many pages it spans and whatever
// deadline the caller's context carries.
func TestSyncPlans_ListingStopsAtItsOwnBudget(t *testing.T) {
	const pageDelay = 40 * time.Millisecond
	pp := &planPortal{handles: numberedHandles(20 * portalPlansPageSize), listDelay: pageDelay}
	srv := pp.serve(t)
	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	p.plansListingTimeout = 3 * pageDelay

	started := time.Now()
	err := syncPlansWithoutMeters(context.Background(), p, &model.APIPortal{URL: srv.URL}, testPlans("never-listed"), nil)
	took := time.Since(started)
	srv.Close() // waits for the handler still answering the cancelled page, so the recorded queries are stable

	var conflict *PortalConflictError
	if !errors.Is(err, context.DeadlineExceeded) || errors.As(err, &conflict) {
		t.Fatalf("want a plain deadline failure, got %v", err)
	}
	if took > 10*pageDelay {
		t.Fatalf("the listing outlived its budget by far: %s", took)
	}
	if len(pp.queries) < 2 || len(pp.queries) >= 20 {
		t.Fatalf("want the listing cut off part-way through, got %d pages", len(pp.queries))
	}
	if len(pp.posted) != 0 {
		t.Fatalf("want no create after a failed listing, got %v", postedIDs(pp))
	}
}

func TestSyncPlans_ListingWithinItsBudgetCompletes(t *testing.T) {
	const pageDelay = 10 * time.Millisecond
	handles := numberedHandles(3 * portalPlansPageSize)
	pp := &planPortal{handles: handles, listDelay: pageDelay}
	srv := pp.serve(t)
	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	p.plansListingTimeout = time.Minute

	if err := syncPlansWithoutMeters(context.Background(), p, &model.APIPortal{URL: srv.URL}, testPlans(handles[len(handles)-1], "brand-new"), nil); err != nil {
		t.Fatalf("SyncPlans: %v", err)
	}
	if len(pp.queries) != 3 {
		t.Fatalf("want all three pages read, got %v", pp.queries)
	}
	if got := postedIDs(pp); len(got) != 1 || got[0] != "brand-new" {
		t.Fatalf("want only brand-new created, got %v", got)
	}
}

// syncPlans runs SyncPlans against pp with the given pricing.
func syncPlans(t *testing.T, pp *planPortal, plans []*model.SubscriptionPlan, pricing map[string]*model.SubscriptionPlanPricing) error {
	t.Helper()
	srv := pp.serve(t)
	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	return syncPlansWithoutMeters(context.Background(), p, &model.APIPortal{URL: srv.URL}, plans, pricing)
}

func limitedPlan(handle, name string, count int, unit string) *model.SubscriptionPlan {
	plan := testPlan(handle)
	plan.Name = name
	plan.ThrottleLimitCount, plan.ThrottleLimitUnit = &count, unit
	return plan
}

func listedLimit(count int, unit string) []any {
	return []any{map[string]any{"limitType": "REQUEST_COUNT", "limitCount": count, "timeUnit": unit, "timeAmount": 1}}
}

// A copy whose name or limit differs is updated with the plan's own, and only those copies.
func TestSyncPlans_UpdatesCopiesWhoseNameOrLimitDiffer(t *testing.T) {
	pp := &planPortal{
		handles: []string{"gold", "silver", "bronze", "free"},
		listed: map[string]map[string]any{
			"gold":   {"id": "gold", "displayName": "Gold", "limits": listedLimit(1000, "MINUTE")},
			"silver": {"id": "silver", "displayName": "Silver", "limits": listedLimit(100, "minute")},
			"bronze": {"id": "bronze", "displayName": "Bronze", "limits": listedLimit(10, "HOUR")},
			"free":   {"id": "free", "displayName": "Free", "limits": listedLimit(5, "MINUTE")},
		},
	}
	plans := []*model.SubscriptionPlan{
		limitedPlan("gold", "Gold Plus", 1000, "MINUTE"),  // renamed
		limitedPlan("silver", "Silver", 100, "MINUTE"),    // same, its unit written in lower case
		limitedPlan("bronze", "Bronze", 20, "HOUR"),       // new limit
		{UUID: "uuid-free", Handle: "free", Name: "Free"}, // limit removed
	}
	if err := syncPlans(t, pp, plans, nil); err != nil {
		t.Fatalf("SyncPlans: %v", err)
	}
	if len(pp.posted) != 0 {
		t.Fatalf("nothing is missing, yet %d creates were sent", len(pp.posted))
	}
	if len(pp.updated) != 1 || len(pp.updated[0]) != 3 {
		t.Fatalf("want one update of gold, bronze and free, got %+v", pp.updated)
	}
	got := pp.updated[0]
	if got[0].ID != "gold" || got[0].DisplayName != "Gold Plus" || len(got[0].Limits) != 1 || got[0].Limits[0].LimitCount != 1000 {
		t.Fatalf("gold update = %+v", got[0])
	}
	if got[1].ID != "bronze" || len(got[1].Limits) != 1 || got[1].Limits[0].LimitCount != 20 || got[1].Limits[0].TimeUnit != "HOUR" {
		t.Fatalf("bronze update = %+v", got[1])
	}
	if got[2].ID != "free" || got[2].Limits == nil || len(got[2].Limits) != 0 {
		t.Fatalf("free update must send an empty limits list, got %+v", got[2])
	}
}

// An update never carries pricing, which the portal refuses for a plan that exists, and always
// carries limits, without which the portal keeps the old ones.
func TestSyncPlans_UpdateBodyShape(t *testing.T) {
	var raw []map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"list": []map[string]any{{"id": "gold", "displayName": "Gold", "limits": listedLimit(5, "MINUTE"), "pricing": map[string]any{
					"pricingType": "FLAT_RATE", "currency": "USD", "billingPeriod": "MONTH", "flatAmount": "10.00", "prices": []any{},
				}}},
				"pagination": map[string]int{"total": 1},
			})
		case http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
				t.Fatalf("decode bulk update body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	fee := "10"
	plan := &model.SubscriptionPlan{UUID: "uuid-gold", Handle: "gold", Name: "Gold"}
	pricing := map[string]*model.SubscriptionPlanPricing{"uuid-gold": {
		PricingType: model.PricingTypeFlatRate, Currency: "USD", BillingPeriod: "MONTH", FlatAmount: &fee, Prices: []model.SubscriptionPlanPrice{},
	}}
	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	if err := syncPlansWithoutMeters(context.Background(), p, &model.APIPortal{URL: srv.URL}, []*model.SubscriptionPlan{plan}, pricing); err != nil {
		t.Fatalf("SyncPlans: %v", err)
	}
	if len(raw) != 1 {
		t.Fatalf("want one plan updated, got %d", len(raw))
	}
	if _, ok := raw[0]["pricing"]; ok {
		t.Fatalf("an update must not carry pricing: %v", raw[0])
	}
	if string(raw[0]["limits"]) != "[]" || string(raw[0]["id"]) != `"gold"` || string(raw[0]["displayName"]) != `"Gold"` {
		t.Fatalf("update body = %v", raw[0])
	}
}

// A copy priced differently refuses the whole sync before anything is written, even a plan the
// portal lacks.
func TestSyncPlans_RefusesACopyPricedDifferently(t *testing.T) {
	ten, twenty := "10.00", "20.00"
	flat := func(amount *string) *model.SubscriptionPlanPricing {
		return &model.SubscriptionPlanPricing{PricingType: model.PricingTypeFlatRate, Currency: "USD", BillingPeriod: "MONTH",
			FlatAmount: amount, Prices: []model.SubscriptionPlanPrice{}}
	}
	listedFlat := func(amount string) map[string]any {
		return map[string]any{"pricingType": "FLAT_RATE", "currency": "USD", "billingPeriod": "MONTH", "flatAmount": amount, "prices": []any{}}
	}
	tests := []struct {
		name    string
		listed  map[string]any // the copy's pricing; nil for a free copy
		pricing *model.SubscriptionPlanPricing
	}{
		{"free copy of a paid plan", nil, flat(&ten)},
		{"paid copy of a free plan", listedFlat("10.00"), nil},
		{"another amount", listedFlat("10.00"), flat(&twenty)},
		{"another billing period", map[string]any{"pricingType": "FLAT_RATE", "currency": "USD", "billingPeriod": "YEAR", "flatAmount": "10.00"}, flat(&ten)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry := map[string]any{"id": "gold", "displayName": "Plan gold", "limits": []any{}}
			if tc.listed != nil {
				entry["pricing"] = tc.listed
			}
			pp := &planPortal{handles: []string{"gold"}, listed: map[string]map[string]any{"gold": entry}}
			pricing := map[string]*model.SubscriptionPlanPricing{}
			if tc.pricing != nil {
				pricing["uuid-gold"] = tc.pricing
			}
			err := syncPlans(t, pp, testPlans("brand-new", "gold"), pricing)
			var conflict *PortalPlanPricingConflictError
			if !errors.As(err, &conflict) || conflict.Handle != "gold" {
				t.Fatalf("want a pricing conflict on gold, got %v", err)
			}
			if len(pp.posted) != 0 || len(pp.updated) != 0 {
				t.Fatalf("nothing may be written: %d creates, %d updates", len(pp.posted), len(pp.updated))
			}
		})
	}
}

// Usage pricing listed in another order and with amounts written another way is the same pricing.
func TestSyncPlans_SameUsagePricingWrittenDifferently(t *testing.T) {
	unit, quantity, upTo := "0.010", int64(1000), int64(100)
	pricing := map[string]*model.SubscriptionPlanPricing{"uuid-gold": {
		PricingType: model.PricingTypeUsageBased, Currency: "USD", BillingPeriod: "MONTH", Prices: []model.SubscriptionPlanPrice{
			{MeterHandle: "llm-input", PricingModel: model.PricingModelPerUnit, UnitAmount: &unit, UnitQuantity: &quantity,
				Filters: []model.PriceFilter{{Field: "API_TYPE", Operator: "IN", Values: []string{"LlmProvider", " WebSubApi", "LlmProvider"}},
					{Field: "HTTP_METHOD", Operator: "IN", Values: []string{"get", "POST"}}}},
			{MeterHandle: "api-calls", PricingModel: model.PricingModelGraduated,
				Tiers: []model.PriceTier{{UpTo: &upTo, UnitAmount: &unit}, {UnitAmount: &unit}}},
		},
	}}
	listed := map[string]any{"pricingType": "USAGE_BASED", "currency": "USD", "billingPeriod": "MONTH", "flatAmount": nil, "prices": []any{
		map[string]any{"meterId": "api-calls", "filters": []any{}, "pricingModel": "GRADUATED", "unitAmount": nil, "unitQuantity": nil,
			"tiers": []any{map[string]any{"upTo": 100, "unitAmount": "0.01", "flatAmount": nil}, map[string]any{"upTo": nil, "unitAmount": "0.01", "flatAmount": nil}}},
		map[string]any{"meterId": "llm-input", "filters": []any{
			map[string]any{"field": "HTTP_METHOD", "operator": "IN", "values": []any{"POST", "GET"}},
			map[string]any{"field": "API_TYPE", "operator": "IN", "values": []any{"WebSubApi", "LlmProvider"}}},
			"pricingModel": "PER_UNIT", "unitAmount": "0.01", "unitQuantity": 1000, "tiers": nil},
	}}
	pp := &planPortal{handles: []string{"gold"}, listed: map[string]map[string]any{
		"gold": {"id": "gold", "displayName": "Plan gold", "limits": []any{}, "pricing": listed},
	}}
	if err := syncPlans(t, pp, testPlans("gold"), pricing); err != nil {
		t.Fatalf("SyncPlans: %v", err)
	}
	if len(pp.posted) != 0 || len(pp.updated) != 0 {
		t.Fatalf("an unchanged copy is left alone: %d creates, %d updates", len(pp.posted), len(pp.updated))
	}
}

// Missing plans are created first, then the changed copies are updated.
func TestSyncPlans_CreatesThenUpdates(t *testing.T) {
	pp := &planPortal{handles: []string{"gold"}, listed: map[string]map[string]any{
		"gold": {"id": "gold", "displayName": "Old name", "limits": []any{}},
	}}
	if err := syncPlans(t, pp, testPlans("silver", "gold"), nil); err != nil {
		t.Fatalf("SyncPlans: %v", err)
	}
	if len(pp.posted) != 1 || len(pp.posted[0]) != 1 || pp.posted[0][0].ID != "silver" {
		t.Fatalf("creates = %+v", pp.posted)
	}
	if len(pp.updated) != 1 || len(pp.updated[0]) != 1 || pp.updated[0][0].ID != "gold" || pp.updated[0][0].DisplayName != "Plan gold" {
		t.Fatalf("updates = %+v", pp.updated)
	}
}

func TestSyncPlans_UpdateErrorMapping(t *testing.T) {
	tests := []struct {
		status       int
		wantConflict bool
	}{
		{http.StatusConflict, true},
		{http.StatusBadRequest, true},
		{http.StatusUnauthorized, false},
		{http.StatusInternalServerError, false},
	}
	for _, tc := range tests {
		t.Run(strconv.Itoa(tc.status), func(t *testing.T) {
			pp := &planPortal{handles: []string{"gold"}, updateStatus: tc.status, listed: map[string]map[string]any{
				"gold": {"id": "gold", "displayName": "Old name", "limits": []any{}},
			}}
			err := syncPlans(t, pp, testPlans("gold"), nil)
			if err == nil {
				t.Fatal("want an error")
			}
			var conflict *PortalConflictError
			if got := errors.As(err, &conflict); got != tc.wantConflict {
				t.Fatalf("want conflict=%v, got %v (%v)", tc.wantConflict, got, err)
			}
		})
	}
}

// A copy deleted in the portal between the listing and the update comes back from the update
// (201) without pricing: fine for a free plan, a pricing conflict for a paid one.
func TestSyncPlans_UpdateThatRecreatesACopy(t *testing.T) {
	fee := "10"
	paid := &model.SubscriptionPlanPricing{PricingType: model.PricingTypeFlatRate, Currency: "USD", BillingPeriod: "MONTH",
		FlatAmount: &fee, Prices: []model.SubscriptionPlanPrice{}}
	listedPaid := map[string]any{"pricingType": "FLAT_RATE", "currency": "USD", "billingPeriod": "MONTH", "flatAmount": "10"}

	free := &planPortal{handles: []string{"gold"}, updateStatus: http.StatusCreated, listed: map[string]map[string]any{
		"gold": {"id": "gold", "displayName": "Old name", "limits": []any{}},
	}}
	if err := syncPlans(t, free, testPlans("gold"), nil); err != nil {
		t.Fatalf("free plan: %v", err)
	}

	paidPortal := &planPortal{handles: []string{"gold"}, updateStatus: http.StatusCreated, listed: map[string]map[string]any{
		"gold": {"id": "gold", "displayName": "Old name", "limits": []any{}, "pricing": listedPaid},
	}}
	err := syncPlans(t, paidPortal, testPlans("gold"), map[string]*model.SubscriptionPlanPricing{"uuid-gold": paid})
	var conflict *PortalPlanPricingConflictError
	if !errors.As(err, &conflict) || conflict.Handle != "gold" {
		t.Fatalf("paid plan: want a pricing conflict, got %v", err)
	}
}

// A per-unit price saved here without a quantity is the portal's per-1-unit price.
func TestSyncPlans_PerUnitPriceWithoutQuantity(t *testing.T) {
	unit := "0.01"
	pricing := map[string]*model.SubscriptionPlanPricing{"uuid-gold": {
		PricingType: model.PricingTypeUsageBased, Currency: "USD", BillingPeriod: "MONTH", Prices: []model.SubscriptionPlanPrice{
			{MeterHandle: "api-calls", PricingModel: model.PricingModelPerUnit, UnitAmount: &unit, Filters: []model.PriceFilter{}},
		},
	}}
	listed := map[string]any{"pricingType": "USAGE_BASED", "currency": "USD", "billingPeriod": "MONTH", "prices": []any{
		map[string]any{"meterId": "api-calls", "filters": []any{}, "pricingModel": "PER_UNIT", "unitAmount": "0.01", "unitQuantity": 1, "tiers": nil},
	}}
	pp := &planPortal{handles: []string{"gold"}, listed: map[string]map[string]any{
		"gold": {"id": "gold", "displayName": "Plan gold", "limits": []any{}, "pricing": listed},
	}}
	if err := syncPlans(t, pp, testPlans("gold"), pricing); err != nil {
		t.Fatalf("SyncPlans: %v", err)
	}
	if len(pp.posted) != 0 || len(pp.updated) != 0 {
		t.Fatalf("an unchanged copy is left alone: %d creates, %d updates", len(pp.posted), len(pp.updated))
	}
}
