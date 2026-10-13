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

func (d *deadlineRecorder) CreateMissingPlans(ctx context.Context, _ *model.APIPortal, _ []*model.SubscriptionPlan) error {
	d.record("plans", ctx)
	return nil
}

func (d *deadlineRecorder) Publish(ctx context.Context, _ *model.APIPortal, _ string, _ *model.Publication, _ *model.PublicationContent) error {
	d.record("publish", ctx)
	return nil
}

func (d *deadlineRecorder) Unpublish(ctx context.Context, _ *model.APIPortal, _ string) error {
	d.record("unpublish", ctx)
	return nil
}

func (d *deadlineRecorder) Deprecate(ctx context.Context, _ *model.APIPortal, _ string, _ *model.Publication) error {
	d.record("deprecate", ctx)
	return nil
}

// stalledPortal never answers: every call waits for its context to end.
type stalledPortal struct{}

func (stalledPortal) CreateMissingPlans(ctx context.Context, _ *model.APIPortal, _ []*model.SubscriptionPlan) error {
	<-ctx.Done()
	return ctx.Err()
}

func (stalledPortal) Publish(ctx context.Context, _ *model.APIPortal, _ string, _ *model.Publication, _ *model.PublicationContent) error {
	<-ctx.Done()
	return ctx.Err()
}

func (stalledPortal) Unpublish(ctx context.Context, _ *model.APIPortal, _ string) error {
	<-ctx.Done()
	return ctx.Err()
}

func (stalledPortal) Deprecate(ctx context.Context, _ *model.APIPortal, _ string, _ *model.Publication) error {
	<-ctx.Done()
	return ctx.Err()
}

const stalledOperationTimeout = 100 * time.Millisecond

func TestPublicationOperations_PortalCallsShareOneDeadline(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	g := seedOrgGraph(t, it)
	portal := &deadlineRecorder{}
	svc := newPublicationTestServiceWith(it, portal)
	apiType, apiHandle, portalHandle := "rest-api", apiHandleFor(g), portalHandleFor(g)

	started := time.Now()
	publishedListing(t, it, svc, g)
	if len(portal.deadlines) != 2 || !portal.deadlines["plans"].Equal(portal.deadlines["publish"]) {
		t.Fatalf("[%s] want the plan step and the push under the same deadline, got %v", it.driver, portal.deadlines)
	}
	assertDeadlineWithin(t, it, "publish", portal.deadlines["publish"], started, publicationTestOperationTimeout)

	started = time.Now()
	if _, err := svc.Deprecate(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor"); err != nil {
		t.Fatalf("[%s] Deprecate failed: %v", it.driver, err)
	}
	assertDeadlineWithin(t, it, "deprecate", portal.deadlines["deprecate"], started, publicationTestOperationTimeout)

	started = time.Now()
	if err := svc.Unpublish(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor"); err != nil {
		t.Fatalf("[%s] Unpublish failed: %v", it.driver, err)
	}
	assertDeadlineWithin(t, it, "unpublish", portal.deadlines["unpublish"], started, publicationTestOperationTimeout)
}

func assertDeadlineWithin(t *testing.T, it *itDB, call string, deadline, started time.Time, limit time.Duration) {
	t.Helper()
	if deadline.IsZero() {
		t.Fatalf("[%s] %s ran without a deadline", it.driver, call)
	}
	if deadline.Before(started.Add(limit-time.Second)) || deadline.After(started.Add(limit+time.Second)) {
		t.Fatalf("[%s] want the %s deadline about %s after the request started, got %s", it.driver, call, limit, deadline.Sub(started))
	}
}

func TestPublicationPublish_DeadlineExpiresDuringPortalCalls(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	g := seedOrgGraph(t, it)
	svc := newPublicationTestServiceWithTimeout(it, stalledPortal{}, stalledOperationTimeout)
	saveDraftWithPlans(t, it, svc, g, []string{planHandleFor(g)})

	started := time.Now()
	_, _, err := svc.Publish(context.Background(), "rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "publisher")
	if !apperror.APIPublicationPortalUnavailable.Is(err) {
		t.Fatalf("[%s] want APIPublicationPortalUnavailable, got %v", it.driver, err)
	}
	if took := time.Since(started); took > 10*stalledOperationTimeout {
		t.Fatalf("[%s] the request outlived its deadline by far: %s", it.driver, took)
	}
	if _, err := svc.GetPublication("rest-api", apiHandleFor(g), portalHandleFor(g), g.org); !apperror.APIPublicationNotFound.Is(err) {
		t.Fatalf("[%s] want no live listing, got %v", it.driver, err)
	}
	if _, err := svc.GetDraft("rest-api", apiHandleFor(g), portalHandleFor(g), g.org); err != nil {
		t.Fatalf("[%s] want the draft kept, got %v", it.driver, err)
	}
}

func TestPublicationUnpublishAndDeprecate_DeadlineExpires(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	g := seedOrgGraph(t, it)
	apiType, apiHandle, portalHandle := publishedListing(t, it, newPublicationTestService(it), g)
	stalled := newPublicationTestServiceWithTimeout(it, stalledPortal{}, stalledOperationTimeout)

	if _, err := stalled.Deprecate(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor"); !apperror.APIPublicationPortalUnavailable.Is(err) {
		t.Fatalf("[%s] want APIPublicationPortalUnavailable from Deprecate, got %v", it.driver, err)
	}
	if got := liveStatus(t, it, stalled, apiType, apiHandle, portalHandle, g.org); got != model.PublicationStatusPublished {
		t.Fatalf("[%s] want the listing still PUBLISHED after a failed deprecate, got %s", it.driver, got)
	}

	if err := stalled.Unpublish(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor"); !apperror.APIPublicationPortalUnavailable.Is(err) {
		t.Fatalf("[%s] want APIPublicationPortalUnavailable from Unpublish, got %v", it.driver, err)
	}
	if got := liveStatus(t, it, stalled, apiType, apiHandle, portalHandle, g.org); got != model.PublicationStatusPublished {
		t.Fatalf("[%s] want the listing still PUBLISHED after a failed unpublish, got %s", it.driver, got)
	}
}

var _ service.PortalPublisher = stalledPortal{}
