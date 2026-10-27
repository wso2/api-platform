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

package service_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

// planSyncRecorder records the order of the portal calls Publish makes, and the
// plans and context handed to SyncPlans.
type planSyncRecorder struct {
	alwaysSucceedsPortalPublisher
	events      []string
	plans       []*model.SubscriptionPlan
	deadline    time.Time
	hasDeadline bool
	plansErr    error
	// created is what the sync reports it created; publishErr fails the listing push.
	created    service.PlanSyncResult
	publishErr error
	undone     *service.PlanSyncResult
	undoAlive  bool // the undo's context was still live
}

func (p *planSyncRecorder) SyncPlans(ctx context.Context, _ *model.APIPortal, sync service.PlanSync) (service.PlanSyncResult, error) {
	p.events = append(p.events, "plans")
	p.plans = sync.Plans
	p.deadline, p.hasDeadline = ctx.Deadline()
	return p.created, p.plansErr
}

func (p *planSyncRecorder) Publish(_ context.Context, _ *model.APIPortal, _ string, _ string, _ *model.Publication, _ *model.PublicationContent) error {
	p.events = append(p.events, "publish")
	return p.publishErr
}

func (p *planSyncRecorder) UndoPlanSync(ctx context.Context, _ *model.APIPortal, created service.PlanSyncResult) error {
	p.events = append(p.events, "undo")
	p.undone = &created
	p.undoAlive = ctx.Err() == nil
	return nil
}

func saveDraftWithPlans(t *testing.T, tdb *schemaDB, svc *service.PublicationService, g graph, planHandles []string) {
	t.Helper()
	draft := &model.Publication{DisplayName: "Live Listing", Version: "1.0", AgentVisibility: "VISIBLE"}
	if _, err := svc.SaveDraftDetails("rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "publisher", draft, planHandles, nil); err != nil {
		t.Fatalf("SaveDraftDetails failed: %v", err)
	}
	if err := svc.SaveDraftDefinition("rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "publisher", "application/json", []byte(minimalValidDefinition)); err != nil {
		t.Fatalf("SaveDraftDefinition failed: %v", err)
	}
}

func TestPublicationPublish_CreatesMissingPlansBeforePush(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	portal := &planSyncRecorder{}
	svc := newPublicationTestServiceWith(tdb, portal)
	saveDraftWithPlans(t, tdb, svc, g, []string{planHandleFor(g)})

	before := time.Now()
	if _, _, err := svc.Publish(context.Background(), "rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "publisher"); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if len(portal.events) != 2 || portal.events[0] != "plans" || portal.events[1] != "publish" {
		t.Fatalf("want plans before publish, got %v", portal.events)
	}
	if len(portal.plans) != 1 || portal.plans[0].Handle != planHandleFor(g) || portal.plans[0].UUID != g.plan {
		t.Fatalf("want the selected plan handed over, got %+v", portal.plans)
	}
	if !portal.hasDeadline || portal.deadline.After(before.Add(publicationTestOperationTimeout+time.Second)) {
		t.Fatalf("want the plan step under the operation deadline, got deadline %v (set=%v)", portal.deadline, portal.hasDeadline)
	}
}

// A failed listing push takes back the plans and meters its publish created in the portal, with a
// time limit of its own, since the push may have failed because the publish's time ran out.
func TestPublicationPublish_FailedPushRemovesWhatThePublishCreated(t *testing.T) {
	for name, pushErr := range map[string]error{"rejected": &service.PortalConflictError{Message: "rejected"}, "timed out": context.DeadlineExceeded} {
		tdb := openSchemaDB(t)
		g := seedOrgGraph(t, tdb)
		created := service.PlanSyncResult{Plans: []string{planHandleFor(g)}, Meters: []string{"calls"}}
		portal := &planSyncRecorder{created: created, publishErr: pushErr}
		svc := newPublicationTestServiceWith(tdb, portal)
		saveDraftWithPlans(t, tdb, svc, g, []string{planHandleFor(g)})

		ctx, cancel := context.WithCancel(context.Background())
		if name == "timed out" {
			cancel() // the publish's own time is up when the undo runs
		}
		if _, _, err := svc.Publish(ctx, "rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "publisher"); err == nil {
			t.Fatalf("%s: want the push's failure", name)
		}
		cancel()
		if !reflect.DeepEqual(portal.events, []string{"plans", "publish", "undo"}) || portal.undone == nil ||
			!reflect.DeepEqual(*portal.undone, created) || !portal.undoAlive {
			t.Fatalf("%s: events %v, undone %+v (live %v), want what the sync created removed", name,
				portal.events, portal.undone, portal.undoAlive)
		}
	}

	// A push that succeeds keeps them; a sync that created nothing has nothing to undo.
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	portal := &planSyncRecorder{created: service.PlanSyncResult{Plans: []string{planHandleFor(g)}}}
	svc := newPublicationTestServiceWith(tdb, portal)
	saveDraftWithPlans(t, tdb, svc, g, []string{planHandleFor(g)})
	if _, _, err := svc.Publish(context.Background(), "rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "publisher"); err != nil ||
		portal.undone != nil {
		t.Fatalf("a successful push: %v, undone %+v", err, portal.undone)
	}
}

func TestPublicationPublish_NoPlansSkipsPlanSync(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	portal := &planSyncRecorder{}
	svc := newPublicationTestServiceWith(tdb, portal)
	saveDraftWithPlans(t, tdb, svc, g, nil)

	if _, _, err := svc.Publish(context.Background(), "rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "publisher"); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	if len(portal.events) != 1 || portal.events[0] != "publish" {
		t.Fatalf("want only the publish call, got %v", portal.events)
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
			tdb := openSchemaDB(t)
			g := seedOrgGraph(t, tdb)
			portal := &planSyncRecorder{plansErr: tc.err}
			svc := newPublicationTestServiceWith(tdb, portal)
			saveDraftWithPlans(t, tdb, svc, g, []string{planHandleFor(g)})

			_, _, err := svc.Publish(context.Background(), "rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "publisher")
			if !tc.is(err) {
				t.Fatalf("want the mapped portal error, got %v", err)
			}
			if len(portal.events) != 1 || portal.events[0] != "plans" {
				t.Fatalf("want the push skipped after a plan failure, got %v", portal.events)
			}
			if _, err := svc.GetPublication("rest-api", apiHandleFor(g), portalHandleFor(g), g.org); !apperror.APIPublicationNotFound.Is(err) {
				t.Fatalf("want no live listing, got %v", err)
			}
			if _, err := svc.GetDraft("rest-api", apiHandleFor(g), portalHandleFor(g), g.org); err != nil {
				t.Fatalf("want the draft kept, got %v", err)
			}
		})
	}
}

func deactivatePlan(t *testing.T, tdb *schemaDB, g graph) {
	t.Helper()
	tdb.exec(t, "UPDATE subscription_plans SET status = ? WHERE uuid = ?", string(model.SubscriptionPlanStatusInactive), g.plan)
}

func TestPublicationDraft_InactivePlanRejected(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	svc := newPublicationTestService(tdb)
	deactivatePlan(t, tdb, g)

	draft := &model.Publication{DisplayName: "x", Version: "1.0", AgentVisibility: "VISIBLE"}
	_, err := svc.SaveDraftDetails("rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "actor", draft, []string{planHandleFor(g)}, nil)
	if !apperror.APIPublicationValidationFailed.Is(err) {
		t.Fatalf("want APIPublicationValidationFailed for an inactive plan, got %v", err)
	}
	if !strings.Contains(err.Error(), planHandleFor(g)) || !strings.Contains(err.Error(), "not active") {
		t.Fatalf("want the error to name the inactive plan, got %v", err)
	}
}

func TestPublicationPublish_PlanDeactivatedAfterSaveIsRejected(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	portal := &planSyncRecorder{}
	svc := newPublicationTestServiceWith(tdb, portal)
	saveDraftWithPlans(t, tdb, svc, g, []string{planHandleFor(g)})
	deactivatePlan(t, tdb, g)

	_, _, err := svc.Publish(context.Background(), "rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "publisher")
	if !apperror.APIPublicationValidationFailed.Is(err) {
		t.Fatalf("want APIPublicationValidationFailed, got %v", err)
	}
	if len(portal.events) != 0 {
		t.Fatalf("want no portal call for an inactive plan, got %v", portal.events)
	}
	if _, err := svc.GetPublication("rest-api", apiHandleFor(g), portalHandleFor(g), g.org); !apperror.APIPublicationNotFound.Is(err) {
		t.Fatalf("want no live listing, got %v", err)
	}
	draft, err := svc.GetDraft("rest-api", apiHandleFor(g), portalHandleFor(g), g.org)
	if err != nil || len(draft.SubscriptionPlanIds) != 1 || draft.SubscriptionPlanIds[0] != planHandleFor(g) {
		t.Fatalf("want the draft kept with its plan so the user can clear it, got %+v (%v)", draft, err)
	}
}
