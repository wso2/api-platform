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

	"github.com/wso2/api-platform/platform-api/internal/model"
)

// planPortal is a fake API Portal serving the subscription plan collection.
type planPortal struct {
	handles      []string // the portal's plans, in listing order
	total        int      // reported pagination total; len(handles) when zero
	listStatus   int      // overrides the listing's 200 when non-zero
	createStatus int      // the bulk create's status; 201 when zero
	listBody     string   // replaces the listing body when non-empty

	queries []string
	auths   []string
	posted  [][]portalPlan
}

func (pp *planPortal) serve(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func (pp *planPortal) list(w http.ResponseWriter, r *http.Request) {
	pp.queries = append(pp.queries, r.URL.RawQuery)
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
	page := []map[string]string{}
	if offset < end {
		for _, h := range pp.handles[offset:end] {
			page = append(page, map[string]string{"id": h})
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
	return p.CreateMissingPlans(context.Background(), &model.APIPortal{URL: srv.URL}, plans)
}

func TestCreateMissingPlans_NoPlansMakesNoRequest(t *testing.T) {
	pp := &planPortal{}
	if err := createMissing(t, pp, nil); err != nil {
		t.Fatalf("CreateMissingPlans: %v", err)
	}
	if len(pp.queries)+len(pp.posted) != 0 {
		t.Fatalf("want no requests, got %d listings and %d creates", len(pp.queries), len(pp.posted))
	}
}

func TestCreateMissingPlans_NothingMissingSkipsCreate(t *testing.T) {
	pp := &planPortal{handles: []string{"gold", "silver", "bronze"}}
	if err := createMissing(t, pp, testPlans("gold", "silver")); err != nil {
		t.Fatalf("CreateMissingPlans: %v", err)
	}
	if len(pp.posted) != 0 {
		t.Fatalf("want no create, got %v", postedIDs(pp))
	}
	if len(pp.queries) != 1 || pp.queries[0] != "limit=20&offset=0" {
		t.Fatalf("want one listing of the first page, got %v", pp.queries)
	}
}

func TestCreateMissingPlans_CreatesOnlyMissingInInputOrder(t *testing.T) {
	pp := &planPortal{handles: []string{"silver"}}
	if err := createMissing(t, pp, testPlans("zeta", "silver", "alpha")); err != nil {
		t.Fatalf("CreateMissingPlans: %v", err)
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

func TestCreateMissingPlans_PagesUntilTotal(t *testing.T) {
	pp := &planPortal{handles: numberedHandles(25)}
	if err := createMissing(t, pp, testPlans("plan-025", "brand-new")); err != nil {
		t.Fatalf("CreateMissingPlans: %v", err)
	}
	want := []string{"limit=20&offset=0", "limit=20&offset=20"}
	if strings.Join(pp.queries, "|") != strings.Join(want, "|") {
		t.Fatalf("want listings %v, got %v", want, pp.queries)
	}
	if got := postedIDs(pp); len(got) != 1 || got[0] != "brand-new" {
		t.Fatalf("want only brand-new created, got %v", got)
	}
}

func TestCreateMissingPlans_StopsPagingOnceAllFound(t *testing.T) {
	pp := &planPortal{handles: numberedHandles(100)}
	if err := createMissing(t, pp, testPlans("plan-001")); err != nil {
		t.Fatalf("CreateMissingPlans: %v", err)
	}
	if len(pp.queries) != 1 || len(pp.posted) != 0 {
		t.Fatalf("want one listing and no create, got %v and %v", pp.queries, postedIDs(pp))
	}
}

func TestCreateMissingPlans_StopsOnEmptyPage(t *testing.T) {
	pp := &planPortal{handles: numberedHandles(20), total: 50}
	if err := createMissing(t, pp, testPlans("brand-new")); err != nil {
		t.Fatalf("CreateMissingPlans: %v", err)
	}
	if len(pp.queries) != 2 {
		t.Fatalf("want listings of page one and the empty page two, got %v", pp.queries)
	}
	if got := postedIDs(pp); len(got) != 1 || got[0] != "brand-new" {
		t.Fatalf("want brand-new created, got %v", got)
	}
}

func TestCreateMissingPlans_PageCapIsAnError(t *testing.T) {
	pp := &planPortal{handles: numberedHandles(20), total: 1 << 30}
	// Every page repeats the same 20 plans, so the wanted one is never found.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pp.queries = append(pp.queries, r.URL.RawQuery)
		items := make([]map[string]string, 0, 20)
		for _, h := range pp.handles {
			items = append(items, map[string]string{"id": h})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"list": items, "pagination": map[string]int{"total": pp.total}})
	}))
	defer srv.Close()

	p := newTestHTTPPortalPublisher(t, "test-shared-key")
	err := p.CreateMissingPlans(context.Background(), &model.APIPortal{URL: srv.URL}, testPlans("never-listed"))
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

func TestCreateMissingPlans_MatchesHandlesExactly(t *testing.T) {
	pp := &planPortal{handles: []string{"Gold"}}
	if err := createMissing(t, pp, testPlans("gold")); err != nil {
		t.Fatalf("CreateMissingPlans: %v", err)
	}
	if got := postedIDs(pp); len(got) != 1 || got[0] != "gold" {
		t.Fatalf("want gold created next to Gold, got %v", got)
	}
}

func TestCreateMissingPlans_MapsPlanFields(t *testing.T) {
	limited := testPlan("limited")
	count := 1000
	limited.ThrottleLimitCount = &count
	limited.ThrottleLimitUnit = "HOUR"
	unlimited := testPlan("unlimited")

	pp := &planPortal{}
	if err := createMissing(t, pp, []*model.SubscriptionPlan{limited, unlimited}); err != nil {
		t.Fatalf("CreateMissingPlans: %v", err)
	}
	if len(pp.posted) != 1 || len(pp.posted[0]) != 2 {
		t.Fatalf("want one create of two plans, got %v", pp.posted)
	}
	got := pp.posted[0]
	wantLimit := portalPlanLimit{LimitType: "REQUEST_COUNT", LimitCount: 1000, TimeUnit: "HOUR", TimeAmount: 1}
	if got[0].ID != "limited" || got[0].RefID != "uuid-limited" || got[0].DisplayName != "Plan limited" ||
		len(got[0].Limits) != 1 || got[0].Limits[0] != wantLimit {
		t.Fatalf("limited plan mapped wrong: %+v", got[0])
	}
	if got[1].ID != "unlimited" || len(got[1].Limits) != 0 {
		t.Fatalf("a plan with no limit must be sent without limits: %+v", got[1])
	}
}

func TestCreateMissingPlans_OmitsLimitsKeyForUnlimitedPlan(t *testing.T) {
	body, err := json.Marshal(toPortalPlan(testPlan("unlimited")))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "limits") {
		t.Fatalf("want no limits key, got %s", body)
	}
}

func TestCreateMissingPlans_ErrorMapping(t *testing.T) {
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
