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
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/model"
)

const plansPath = portalRESTBase + "/subscription-plans"

func TestHTTPPortalPublisher_CreateSubscriptionPlansIfAbsent(t *testing.T) {
	var gotQuery, gotAuth, gotType string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != plansPath {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		gotQuery, gotAuth, gotType = r.URL.RawQuery, r.Header.Get("Authorization"), r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `[{"id":"gold","status":"exists"},{"id":"free","status":"created"}]`)
	}))
	defer srv.Close()

	p := newTestHTTPPortalPublisher(t, "k")
	created, err := p.CreateSubscriptionPlansIfAbsent(context.Background(), &model.APIPortal{URL: srv.URL}, []PortalPlan{
		{Handle: "gold", DisplayName: "Gold", RefID: "u1", Limits: []PortalPlanLimit{{LimitType: "REQUEST_COUNT", TimeUnit: "MINUTE", TimeAmount: 1, LimitCount: 100}}},
		{Handle: "free", DisplayName: "Free", RefID: "u2", Limits: []PortalPlanLimit{{LimitType: "REQUEST_COUNT", TimeAmount: 1, LimitCount: -1}}},
	})
	if err != nil {
		t.Fatalf("CreateSubscriptionPlansIfAbsent: %v", err)
	}
	if !reflect.DeepEqual(created, []string{"free"}) {
		t.Fatalf("created = %v, want [free]", created)
	}
	if gotQuery != "existing=keep" || gotAuth != "SharedKey k" || gotType != "application/json" {
		t.Fatalf("unexpected request: query=%q auth=%q type=%q", gotQuery, gotAuth, gotType)
	}

	var body []map[string]any
	if err := json.Unmarshal(gotBody, &body); err != nil || len(body) != 2 {
		t.Fatalf("want a JSON array of 2 plans, got %s (%v)", gotBody, err)
	}
	if _, has := body[0]["description"]; has {
		t.Fatalf("description must be omitted, got %s", gotBody)
	}
	if body[0]["id"] != "gold" || body[0]["displayName"] != "Gold" || body[0]["refId"] != "u1" {
		t.Fatalf("unexpected first plan: %v", body[0])
	}
	free := body[1]["limits"].([]any)[0].(map[string]any)
	if free["limitCount"].(float64) != -1 || free["timeUnit"] != nil || free["limitType"] != "REQUEST_COUNT" {
		t.Fatalf("unlimited plan must send limitCount -1 and null timeUnit, got %v", free)
	}
}

func TestHTTPPortalPublisher_CreateSubscriptionPlansIfAbsent_NothingCreated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `[{"id":"gold","status":"exists"}]`)
	}))
	defer srv.Close()
	p := newTestHTTPPortalPublisher(t, "k")
	created, err := p.CreateSubscriptionPlansIfAbsent(context.Background(), &model.APIPortal{URL: srv.URL}, []PortalPlan{{Handle: "gold"}})
	if err != nil || len(created) != 0 {
		t.Fatalf("want no created plans and no error, got %v / %v", created, err)
	}
}

func TestHTTPPortalPublisher_CreateSubscriptionPlansIfAbsent_Errors(t *testing.T) {
	cases := []struct {
		status       int
		wantConflict bool
	}{{400, true}, {404, true}, {409, true}, {401, false}, {403, false}, {500, false}}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
		}))
		p := newTestHTTPPortalPublisher(t, "k")
		_, err := p.CreateSubscriptionPlansIfAbsent(context.Background(), &model.APIPortal{URL: srv.URL}, []PortalPlan{{Handle: "gold"}})
		srv.Close()
		if err == nil {
			t.Fatalf("status %d: want error", tc.status)
		}
		if _, isConflict := err.(*PortalConflictError); isConflict != tc.wantConflict {
			t.Fatalf("status %d: conflict=%v, want %v (%v)", tc.status, isConflict, tc.wantConflict, err)
		}
	}
}

func TestHTTPPortalPublisher_CreateSubscriptionPlansIfAbsent_BadBody(t *testing.T) {
	bodies := map[string]string{
		"not an array": `{"message":"Bulk creation is not allowed"}`,
		"oversized":    `[{"id":"gold","status":"exists","pad":"` + strings.Repeat("a", portalPlanResultMaxBytes) + `"}]`,
	}
	for name, body := range bodies {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, body)
		}))
		p := newTestHTTPPortalPublisher(t, "k")
		_, err := p.CreateSubscriptionPlansIfAbsent(context.Background(), &model.APIPortal{URL: srv.URL}, []PortalPlan{{Handle: "gold"}})
		srv.Close()
		if err == nil {
			t.Fatalf("%s: want error", name)
		}
	}
}
