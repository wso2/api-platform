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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/client"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

func budgetTestLimits() config.APIPortalPublication {
	limits := config.APIPortalPublication{
		RequestTimeout:        40 * time.Millisecond,
		ListingRequestTimeout: 40 * time.Millisecond,
		ContentRequestTimeout: 120 * time.Millisecond,
		MaxRetries:            1,
		RetryBackoff:          10 * time.Millisecond,
	}
	limits.OverallTimeout = limits.PublishWorstCase() + 150*time.Millisecond
	return limits
}

func newBudgetTestPublisher(t *testing.T, limits config.APIPortalPublication) *HTTPPortalPublisher {
	t.Helper()
	p := newTestHTTPPortalPublisherWithPolicies(t, "key",
		client.RetryConfig{MaxRetries: limits.MaxRetries, AttemptTimeout: limits.RequestTimeout, Backoff: limits.RetryBackoff},
		client.RetryConfig{MaxRetries: limits.MaxRetries, AttemptTimeout: limits.ContentRequestTimeout, Backoff: limits.RetryBackoff})
	p.plansListingTimeout = limits.PlanListingBudget()
	return p
}

// hangUntilClientGivesUp holds the request open until the client drops it. The server only
// notices a dropped connection once the request body has been read, so it is drained first;
// the timer keeps a missed cancellation from hanging the test run.
func hangUntilClientGivesUp(r *http.Request) {
	io.Copy(io.Discard, r.Body)
	select {
	case <-r.Context().Done():
	case <-time.After(5 * time.Second):
	}
}

// flakyThenOKPortal hangs the first attempt of every call and answers the retry at once.
func flakyThenOKPortal(t *testing.T) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	seen := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		mu.Lock()
		seen[key]++
		first := seen[key] == 1
		mu.Unlock()
		if first {
			hangUntilClientGivesUp(r)
			return
		}
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/subscription-plans"):
			w.Write([]byte(`{"list":[],"pagination":{"total":0}}`))
		case r.Method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusCreated)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestHTTPPortalPublisher_WorstCasePublishFitsTheStatedBudget(t *testing.T) {
	limits := budgetTestLimits()
	p := newBudgetTestPublisher(t, limits)
	portal := &model.APIPortal{URL: flakyThenOKPortal(t).URL}
	pub := &model.Publication{DisplayName: "X", Version: "1.0", AgentVisibility: "VISIBLE"}

	ctx, cancel := context.WithTimeout(context.Background(), limits.OverallTimeout)
	defer cancel()
	started := time.Now()
	if err := syncPlansWithoutMeters(ctx, p, portal, []*model.SubscriptionPlan{{Handle: "gold"}}, nil); err != nil {
		t.Fatalf("SyncPlans: %v", err)
	}
	if err := p.Publish(ctx, portal, constants.PublicationAPITypeRestAPI, "my-api", pub, nil); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if err := p.PublishContent(ctx, portal, "my-api", &PortalContent{Archive: []byte("zip")}); err != nil {
		t.Fatalf("PublishContent: %v", err)
	}
	took := time.Since(started)

	if took > limits.PublishWorstCase()+100*time.Millisecond {
		t.Fatalf("a worst-case publish took %s, more than the stated worst case %s", took, limits.PublishWorstCase())
	}
	if floor := 4*limits.RequestTimeout + limits.ContentRequestTimeout; took < floor {
		t.Fatalf("every first attempt must run to its limit: took %s, want at least %s", took, floor)
	}
}

func TestHTTPPortalPublisher_PortalDownFailsAfterOneCall(t *testing.T) {
	limits := budgetTestLimits()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hangUntilClientGivesUp(r)
	}))
	defer srv.Close()
	p := newBudgetTestPublisher(t, limits)

	ctx, cancel := context.WithTimeout(context.Background(), limits.OverallTimeout)
	defer cancel()
	started := time.Now()
	err := syncPlansWithoutMeters(ctx, p, &model.APIPortal{URL: srv.URL}, []*model.SubscriptionPlan{{Handle: "gold"}}, nil)
	took := time.Since(started)

	if !apperror.APIPublicationPortalUnavailable.Is(portalPushError(err)) {
		t.Fatalf("want the failure to surface as unavailable, got %v", err)
	}
	if worst := limits.RequestWorstCase(); took < 2*limits.RequestTimeout || took > worst+100*time.Millisecond {
		t.Fatalf("want the call to fail within its worst case %s, took %s", worst, took)
	}
	if ctx.Err() != nil {
		t.Fatal("the operation deadline should not have been reached by a single failing call")
	}
}
