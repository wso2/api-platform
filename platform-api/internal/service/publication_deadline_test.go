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
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

type deadlineRecorder struct {
	alwaysSucceedsPortalPublisher
	deadlines map[string]time.Time
}

func (d *deadlineRecorder) record(call string, ctx context.Context) {
	if d.deadlines == nil {
		d.deadlines = map[string]time.Time{}
	}
	if deadline, ok := ctx.Deadline(); ok {
		d.deadlines[call] = deadline
	}
}

func (d *deadlineRecorder) SyncPlans(ctx context.Context, _ *model.APIPortal, _ service.PlanSync) (service.PlanSyncResult, error) {
	d.record("plans", ctx)
	return service.PlanSyncResult{}, nil
}

func (d *deadlineRecorder) UndoPlanSync(_ context.Context, _ *model.APIPortal, _ service.PlanSyncResult) error {
	return nil
}

func (d *deadlineRecorder) Publish(ctx context.Context, _ *model.APIPortal, _ string, _ string, _ *model.Publication, _ *model.PublicationContent) error {
	d.record("publish", ctx)
	return nil
}

func (d *deadlineRecorder) Unpublish(ctx context.Context, _ *model.APIPortal, _, _ string) error {
	d.record("unpublish", ctx)
	return nil
}

func (d *deadlineRecorder) Deprecate(ctx context.Context, _ *model.APIPortal, _ string, _ string, _ *model.Publication) error {
	d.record("deprecate", ctx)
	return nil
}

func (d *deadlineRecorder) PublishContent(ctx context.Context, _ *model.APIPortal, _, _ string, _ *service.PortalContent) error {
	d.record("content", ctx)
	return nil
}

// stalledPortal never answers: every call waits for its context to end.
type stalledPortal struct{}

func (stalledPortal) SyncPlans(ctx context.Context, _ *model.APIPortal, _ service.PlanSync) (service.PlanSyncResult, error) {
	<-ctx.Done()
	return service.PlanSyncResult{}, ctx.Err()
}

func (stalledPortal) UndoPlanSync(_ context.Context, _ *model.APIPortal, _ service.PlanSyncResult) error {
	return nil
}

func (stalledPortal) Publish(ctx context.Context, _ *model.APIPortal, _ string, _ string, _ *model.Publication, _ *model.PublicationContent) error {
	<-ctx.Done()
	return ctx.Err()
}

func (stalledPortal) Unpublish(ctx context.Context, _ *model.APIPortal, _, _ string) error {
	<-ctx.Done()
	return ctx.Err()
}

func (stalledPortal) Deprecate(ctx context.Context, _ *model.APIPortal, _ string, _ string, _ *model.Publication) error {
	<-ctx.Done()
	return ctx.Err()
}

func (stalledPortal) PublishContent(ctx context.Context, _ *model.APIPortal, _, _ string, _ *service.PortalContent) error {
	<-ctx.Done()
	return ctx.Err()
}

const stalledOperationTimeout = 100 * time.Millisecond

func TestPublicationOperations_PortalCallsShareOneDeadline(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	portal := &deadlineRecorder{}
	svc := newPublicationTestServiceWith(tdb, portal)
	apiType, apiHandle, portalHandle := "rest-api", apiHandleFor(g), portalHandleFor(g)

	started := time.Now()
	publishedListing(t, tdb, svc, g)
	if len(portal.deadlines) != 3 || !portal.deadlines["plans"].Equal(portal.deadlines["publish"]) || !portal.deadlines["content"].Equal(portal.deadlines["publish"]) {
		t.Fatalf("want the plan step, the listing push and the content push under the same deadline, got %v", portal.deadlines)
	}
	assertDeadlineWithin(t, tdb, "publish", portal.deadlines["publish"], started, publicationTestOperationTimeout)

	started = time.Now()
	if _, err := svc.Deprecate(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor"); err != nil {
		t.Fatalf("Deprecate failed: %v", err)
	}
	assertDeadlineWithin(t, tdb, "deprecate", portal.deadlines["deprecate"], started, publicationTestOperationTimeout)

	started = time.Now()
	if err := svc.Unpublish(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor"); err != nil {
		t.Fatalf("Unpublish failed: %v", err)
	}
	assertDeadlineWithin(t, tdb, "unpublish", portal.deadlines["unpublish"], started, publicationTestOperationTimeout)
}

func assertDeadlineWithin(t *testing.T, tdb *schemaDB, call string, deadline, started time.Time, limit time.Duration) {
	t.Helper()
	if deadline.IsZero() {
		t.Fatalf("%s ran without a deadline", call)
	}
	if deadline.Before(started.Add(limit-time.Second)) || deadline.After(started.Add(limit+time.Second)) {
		t.Fatalf("want the %s deadline about %s after the request started, got %s", call, limit, deadline.Sub(started))
	}
}

func TestPublicationPublish_DeadlineExpiresDuringPortalCalls(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	svc := newPublicationTestServiceWithTimeout(tdb, stalledPortal{}, stalledOperationTimeout)
	saveDraftWithPlans(t, tdb, svc, g, []string{planHandleFor(g)})

	started := time.Now()
	_, _, err := svc.Publish(context.Background(), "rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "publisher")
	if !apperror.APIPublicationPortalUnavailable.Is(err) {
		t.Fatalf("want APIPublicationPortalUnavailable, got %v", err)
	}
	if took := time.Since(started); took > 10*stalledOperationTimeout {
		t.Fatalf("the request outlived its deadline by far: %s", took)
	}
	if _, err := svc.GetPublication("rest-api", apiHandleFor(g), portalHandleFor(g), g.org); !apperror.APIPublicationNotFound.Is(err) {
		t.Fatalf("want no live listing, got %v", err)
	}
	if _, err := svc.GetDraft("rest-api", apiHandleFor(g), portalHandleFor(g), g.org); err != nil {
		t.Fatalf("want the draft kept, got %v", err)
	}
}

func TestPublicationUnpublishAndDeprecate_DeadlineExpires(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	apiType, apiHandle, portalHandle := publishedListing(t, tdb, newPublicationTestService(tdb), g)
	stalled := newPublicationTestServiceWithTimeout(tdb, stalledPortal{}, stalledOperationTimeout)

	if _, err := stalled.Deprecate(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor"); !apperror.APIPublicationPortalUnavailable.Is(err) {
		t.Fatalf("want APIPublicationPortalUnavailable from Deprecate, got %v", err)
	}
	if got := liveStatus(t, tdb, stalled, apiType, apiHandle, portalHandle, g.org); got != model.PublicationStatusPublished {
		t.Fatalf("want the listing still PUBLISHED after a failed deprecate, got %s", got)
	}

	if err := stalled.Unpublish(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor"); !apperror.APIPublicationPortalUnavailable.Is(err) {
		t.Fatalf("want APIPublicationPortalUnavailable from Unpublish, got %v", err)
	}
	if got := liveStatus(t, tdb, stalled, apiType, apiHandle, portalHandle, g.org); got != model.PublicationStatusPublished {
		t.Fatalf("want the listing still PUBLISHED after a failed unpublish, got %s", got)
	}
}

var _ service.PortalPublisher = stalledPortal{}
