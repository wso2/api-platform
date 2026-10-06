//go:build integration

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
 */

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

// planSyncRecorder records the order of the portal calls Publish makes, and the
// plans and context handed to CreateMissingPlans.
type planSyncRecorder struct {
	alwaysSucceedsPortalPublisher
	events      []string
	plans       []*model.SubscriptionPlan
	deadline    time.Time
	hasDeadline bool
	plansErr    error
}

func (p *planSyncRecorder) CreateMissingPlans(ctx context.Context, _ *model.APIPortal, plans []*model.SubscriptionPlan) error {
	p.events = append(p.events, "plans")
	p.plans = plans
	p.deadline, p.hasDeadline = ctx.Deadline()
	return p.plansErr
}

func (p *planSyncRecorder) Publish(_ context.Context, _ *model.APIPortal, _ string, _ *model.Publication, _ *model.PublicationContent) error {
	p.events = append(p.events, "publish")
	return nil
}

func saveDraftWithPlans(t *testing.T, it *itDB, svc *service.PublicationService, g graph, planHandles []string) {
	t.Helper()
	draft := &model.Publication{DisplayName: "Live Listing", Version: "1.0", AgentVisibility: "VISIBLE"}
	if _, err := svc.SaveDraftDetails("rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "publisher", draft, planHandles, nil); err != nil {
		t.Fatalf("[%s] SaveDraftDetails failed: %v", it.driver, err)
	}
	if err := svc.SaveDraftDefinition("rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "publisher", "application/json", []byte(minimalValidDefinition)); err != nil {
		t.Fatalf("[%s] SaveDraftDefinition failed: %v", it.driver, err)
	}
}

func TestPublicationPublish_CreatesMissingPlansBeforePush(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	g := seedOrgGraph(t, it)
	portal := &planSyncRecorder{}
	svc := newPublicationTestServiceWith(it, portal)
	saveDraftWithPlans(t, it, svc, g, []string{planHandleFor(g)})

	before := time.Now()
	if _, _, err := svc.Publish(context.Background(), "rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "publisher"); err != nil {
		t.Fatalf("[%s] Publish failed: %v", it.driver, err)
	}

	if len(portal.events) != 2 || portal.events[0] != "plans" || portal.events[1] != "publish" {
		t.Fatalf("[%s] want plans before publish, got %v", it.driver, portal.events)
	}
	if len(portal.plans) != 1 || portal.plans[0].Handle != planHandleFor(g) || portal.plans[0].UUID != g.plan {
		t.Fatalf("[%s] want the selected plan handed over, got %+v", it.driver, portal.plans)
	}
	if !portal.hasDeadline || portal.deadline.After(before.Add(26*time.Second)) {
		t.Fatalf("[%s] want the plan step bounded to 25s, got deadline %v (set=%v)", it.driver, portal.deadline, portal.hasDeadline)
	}
}

func TestPublicationPublish_NoPlansSkipsPlanSync(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	g := seedOrgGraph(t, it)
	portal := &planSyncRecorder{}
	svc := newPublicationTestServiceWith(it, portal)
	saveDraftWithPlans(t, it, svc, g, nil)

	if _, _, err := svc.Publish(context.Background(), "rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "publisher"); err != nil {
		t.Fatalf("[%s] Publish failed: %v", it.driver, err)
	}
	if len(portal.events) != 1 || portal.events[0] != "publish" {
		t.Fatalf("[%s] want only the publish call, got %v", it.driver, portal.events)
	}
}

func TestPublicationPublish_PlanSyncFailureStopsBeforePush(t *testing.T) {
	tests := []struct {
		name string
		err  error
		is   func(error) bool
	}{
		{"portal rejection", &service.PortalConflictError{Message: "rejected", Reason: "the conflict is resolved"}, apperror.APIPublicationPortalConflict.Is},
		{"portal unavailable", errors.New("portal plan create failed"), apperror.APIPublicationPortalUnavailable.Is},
		{"deadline", context.DeadlineExceeded, apperror.APIPublicationPortalUnavailable.Is},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			it := openITDB(t)
			defer it.db.Close()
			g := seedOrgGraph(t, it)
			portal := &planSyncRecorder{plansErr: tc.err}
			svc := newPublicationTestServiceWith(it, portal)
			saveDraftWithPlans(t, it, svc, g, []string{planHandleFor(g)})

			_, _, err := svc.Publish(context.Background(), "rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "publisher")
			if !tc.is(err) {
				t.Fatalf("[%s] want the mapped portal error, got %v", it.driver, err)
			}
			if len(portal.events) != 1 || portal.events[0] != "plans" {
				t.Fatalf("[%s] want the push skipped after a plan failure, got %v", it.driver, portal.events)
			}
			if _, err := svc.GetPublication("rest-api", apiHandleFor(g), portalHandleFor(g), g.org); !apperror.APIPublicationNotFound.Is(err) {
				t.Fatalf("[%s] want no live listing, got %v", it.driver, err)
			}
			if _, err := svc.GetDraft("rest-api", apiHandleFor(g), portalHandleFor(g), g.org); err != nil {
				t.Fatalf("[%s] want the draft kept, got %v", it.driver, err)
			}
		})
	}
}
