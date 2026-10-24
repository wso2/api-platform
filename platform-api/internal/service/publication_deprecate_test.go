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
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

// minimalValidDefinition is the smallest OpenAPI 3.x document that passes
// Publish's definition validation.
const minimalValidDefinition = `{"openapi":"3.0.0","info":{"title":"t","version":"1"},"paths":{"/":{"get":{"responses":{"200":{"description":"ok"}}}}}}`

// deprecateRecorder records the listing pushed by Deprecate, can fail the call, and can
// run a hook during it.
type deprecateRecorder struct {
	alwaysSucceedsPortalPublisher
	pushed      []*model.Publication
	err         error
	onDeprecate func()
}

func (d *deprecateRecorder) Deprecate(_ context.Context, _ *model.APIPortal, _, _ string, live *model.Publication) error {
	d.pushed = append(d.pushed, live)
	if d.onDeprecate != nil {
		d.onDeprecate()
	}
	return d.err
}

// rowSnapshot is a table row as column to formatted value.
type rowSnapshot map[string]string

func snapshotRows(t *testing.T, tdb *schemaDB, query string, args ...any) []rowSnapshot {
	t.Helper()
	rows, err := tdb.db.Query(tdb.db.Rebind(query), args...)
	if err != nil {
		t.Fatalf("snapshot query failed: %v", err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatalf("snapshot columns failed: %v", err)
	}
	var out []rowSnapshot
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatalf("snapshot scan failed: %v", err)
		}
		row := rowSnapshot{}
		for i, c := range cols {
			if b, ok := vals[i].([]byte); ok {
				row[c] = string(b)
			} else {
				row[c] = fmt.Sprint(vals[i])
			}
		}
		out = append(out, row)
	}
	return out
}

func changedColumns(before, after rowSnapshot) []string {
	var changed []string
	for c, v := range before {
		if after[c] != v {
			changed = append(changed, c)
		}
	}
	sort.Strings(changed)
	return changed
}

func publishedListing(t *testing.T, tdb *schemaDB, svc *service.PublicationService, g graph) (apiType, apiHandle, portalHandle string) {
	t.Helper()
	apiType, apiHandle, portalHandle = "rest-api", apiHandleFor(g), portalHandleFor(g)
	draft := &model.Publication{DisplayName: "Live Listing", Version: "1.0", Description: "live", AgentVisibility: "VISIBLE"}
	if _, err := svc.SaveDraftDetails(apiType, apiHandle, portalHandle, g.org, "publisher", draft, []string{planHandleFor(g)}, []string{docHandleFor(g)}); err != nil {
		t.Fatalf("SaveDraftDetails failed: %v", err)
	}
	if err := svc.SaveDraftDefinition(apiType, apiHandle, portalHandle, g.org, "publisher", "application/json", []byte(minimalValidDefinition)); err != nil {
		t.Fatalf("SaveDraftDefinition failed: %v", err)
	}
	if err := svc.SaveDraftLandingPage(apiType, apiHandle, portalHandle, g.org, "publisher", []byte("# live landing page")); err != nil {
		t.Fatalf("SaveDraftLandingPage failed: %v", err)
	}
	if err := svc.SaveDraftThumbnail(apiType, apiHandle, portalHandle, g.org, "publisher", "live.png", []byte("\x89PNG\r\n\x1a\nlive-thumb")); err != nil {
		t.Fatalf("SaveDraftThumbnail failed: %v", err)
	}
	if _, _, err := svc.Publish(context.Background(), apiType, apiHandle, portalHandle, g.org, "publisher"); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	return apiType, apiHandle, portalHandle
}

func liveStatus(t *testing.T, tdb *schemaDB, svc *service.PublicationService, apiType, apiHandle, portalHandle, org string) string {
	t.Helper()
	live, err := svc.GetPublication(apiType, apiHandle, portalHandle, org)
	if err != nil {
		t.Fatalf("GetPublication failed: %v", err)
	}
	return live.Status
}

// Deprecate pushes the live row (not the draft) and changes only its status and audit
// columns; the draft, contents and mappings stay identical.
func TestPublicationDeprecate_ChangesOnlyStatus(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	portal := &deprecateRecorder{}
	svc := newPublicationTestServiceWith(tdb, portal)

	apiType, apiHandle, portalHandle := publishedListing(t, tdb, svc, g)
	live, err := svc.GetPublication(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("GetPublication failed: %v", err)
	}

	// A draft that differs from the live listing.
	edit := &model.Publication{DisplayName: "Draft Edit", Version: "2.0", Description: "edit", AgentVisibility: "HIDDEN"}
	if _, err := svc.SaveDraftDetails(apiType, apiHandle, portalHandle, g.org, "editor", edit, []string{planHandleFor(g)}, nil); err != nil {
		t.Fatalf("SaveDraftDetails (edit) failed: %v", err)
	}
	if err := svc.SaveDraftDefinition(apiType, apiHandle, portalHandle, g.org, "editor", "application/json", []byte(`{"openapi":"3.1.0"}`)); err != nil {
		t.Fatalf("SaveDraftDefinition (edit) failed: %v", err)
	}
	draft, err := svc.GetDraft(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("GetDraft failed: %v", err)
	}

	pubRow := "SELECT * FROM api_publications WHERE uuid = ?"
	both := []any{live.UUID, draft.UUID}
	beforeLive := snapshotRows(t, tdb, pubRow, live.UUID)
	beforeDraft := snapshotRows(t, tdb, pubRow, draft.UUID)
	beforeContents := snapshotRows(t, tdb, "SELECT * FROM api_publication_contents WHERE publication_uuid IN (?, ?) ORDER BY uuid", both...)
	beforeDocs := snapshotRows(t, tdb, "SELECT * FROM api_publication_doc_mappings WHERE publication_uuid IN (?, ?) ORDER BY publication_uuid, doc_uuid", both...)
	beforePlans := snapshotRows(t, tdb, "SELECT * FROM api_publication_plan_mappings WHERE publication_uuid IN (?, ?) ORDER BY publication_uuid, subscription_plan_uuid", both...)
	if len(beforeContents) == 0 || len(beforeDocs) == 0 || len(beforePlans) == 0 {
		t.Fatalf("fixture must have content and mapping rows to compare")
	}

	deprecated, err := svc.Deprecate(context.Background(), apiType, apiHandle, portalHandle, g.org, "deprecator")
	if err != nil {
		t.Fatalf("Deprecate failed: %v", err)
	}

	if deprecated.Status != model.PublicationStatusDeprecated || deprecated.UUID != live.UUID {
		t.Fatalf("want the same live row now DEPRECATED, got uuid=%s status=%s", deprecated.UUID, deprecated.Status)
	}
	if deprecated.Version != "1.0" || deprecated.DisplayName != "Live Listing" {
		t.Fatalf("want the response to describe the live listing, got %+v", deprecated)
	}

	if len(portal.pushed) != 1 || portal.pushed[0].Version != "1.0" || portal.pushed[0].DisplayName != "Live Listing" {
		t.Fatalf("want the live listing (v1.0) pushed to the portal, not the draft, got %+v", portal.pushed)
	}
	if !reflect.DeepEqual(portal.pushed[0].SubscriptionPlanIds, []string{planHandleFor(g)}) {
		t.Fatalf("want the live row's plan handles pushed, got %v", portal.pushed[0].SubscriptionPlanIds)
	}

	afterLive := snapshotRows(t, tdb, pubRow, live.UUID)
	if got, want := changedColumns(beforeLive[0], afterLive[0]), []string{"status", "updated_at", "updated_by"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("want only %v to change on the live row, got %v", want, got)
	}
	if afterLive[0]["status"] != model.PublicationStatusDeprecated || afterLive[0]["updated_by"] != "deprecator" {
		t.Fatalf("want status DEPRECATED updated_by deprecator, got %v", afterLive[0])
	}

	if got := snapshotRows(t, tdb, pubRow, draft.UUID); !reflect.DeepEqual(beforeDraft, got) {
		t.Fatalf("draft row changed:\nbefore %v\nafter  %v", beforeDraft, got)
	}
	if got := snapshotRows(t, tdb, "SELECT * FROM api_publication_contents WHERE publication_uuid IN (?, ?) ORDER BY uuid", both...); !reflect.DeepEqual(beforeContents, got) {
		t.Fatalf("content rows changed")
	}
	if got := snapshotRows(t, tdb, "SELECT * FROM api_publication_doc_mappings WHERE publication_uuid IN (?, ?) ORDER BY publication_uuid, doc_uuid", both...); !reflect.DeepEqual(beforeDocs, got) {
		t.Fatalf("document mappings changed")
	}
	if got := snapshotRows(t, tdb, "SELECT * FROM api_publication_plan_mappings WHERE publication_uuid IN (?, ?) ORDER BY publication_uuid, subscription_plan_uuid", both...); !reflect.DeepEqual(beforePlans, got) {
		t.Fatalf("plan mappings changed")
	}
}

// Deprecate is refused, without contacting the portal, unless the listing is PUBLISHED.
func TestPublicationDeprecate_OnlyWhenPublished(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	portal := &deprecateRecorder{}
	svc := newPublicationTestServiceWith(tdb, portal)
	apiType, apiHandle, portalHandle := "rest-api", apiHandleFor(g), portalHandleFor(g)

	if _, err := svc.Deprecate(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor"); !apperror.APIPublicationStateConflict.Is(err) || !strings.Contains(err.Error(), "deprecated") {
		t.Fatalf("want APIPublicationStateConflict naming the deprecate action for a never-published API, got %v", err)
	}

	publishedListing(t, tdb, svc, g)
	if _, err := svc.Deprecate(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor"); err != nil {
		t.Fatalf("first Deprecate failed: %v", err)
	}
	if _, err := svc.Deprecate(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor"); !apperror.APIPublicationStateConflict.Is(err) {
		t.Fatalf("want APIPublicationStateConflict for an already-deprecated API, got %v", err)
	}
	if len(portal.pushed) != 1 {
		t.Fatalf("want the portal contacted only for the one valid deprecate, got %d calls", len(portal.pushed))
	}
}

// A portal rejection or outage leaves the listing PUBLISHED.
func TestPublicationDeprecate_PortalFailureLeavesStatus(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want *apperror.Def
	}{
		{"portal conflict", &service.PortalConflictError{Message: "rejected"}, &apperror.APIPublicationPortalConflict},
		{"portal unavailable", errors.New("connection refused"), &apperror.APIPublicationPortalUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tdb := openSchemaDB(t)
			g := seedOrgGraph(t, tdb)
			portal := &deprecateRecorder{}
			svc := newPublicationTestServiceWith(tdb, portal)
			apiType, apiHandle, portalHandle := publishedListing(t, tdb, svc, g)

			portal.err = tc.err
			if _, err := svc.Deprecate(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor"); !tc.want.Is(err) {
				t.Fatalf("want %v, got %v", tc.want.Code, err)
			}
			if got := liveStatus(t, tdb, svc, apiType, apiHandle, portalHandle, g.org); got != model.PublicationStatusPublished {
				t.Fatalf("want status still PUBLISHED after a failed portal push, got %s", got)
			}
		})
	}
}

// A deprecate that races with an unpublish returns 409 and leaves the unpublish in effect.
func TestPublicationDeprecate_ConcurrentUnpublish(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	portal := &deprecateRecorder{}
	svc := newPublicationTestServiceWith(tdb, portal)
	apiType, apiHandle, portalHandle := publishedListing(t, tdb, svc, g)

	portal.onDeprecate = func() {
		if err := svc.Unpublish(context.Background(), apiType, apiHandle, portalHandle, g.org, "racer"); err != nil {
			t.Fatalf("concurrent Unpublish failed: %v", err)
		}
	}
	if _, err := svc.Deprecate(context.Background(), apiType, apiHandle, portalHandle, g.org, "actor"); !apperror.APIPublicationStateConflict.Is(err) {
		t.Fatalf("want APIPublicationStateConflict after a concurrent unpublish, got %v", err)
	}
	if _, err := svc.GetPublication(apiType, apiHandle, portalHandle, g.org); !apperror.APIPublicationNotFound.Is(err) {
		t.Fatalf("want the unpublish to stand (no live listing), got %v", err)
	}
}

// A deprecate that loses a race with another deprecate returns 409 and keeps the winner's
// audit fields.
func TestPublicationDeprecate_ConcurrentDeprecate(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	portal := &deprecateRecorder{}
	svc := newPublicationTestServiceWith(tdb, portal)
	apiType, apiHandle, portalHandle := publishedListing(t, tdb, svc, g)

	portal.onDeprecate = func() {
		portal.onDeprecate = nil
		if _, err := svc.Deprecate(context.Background(), apiType, apiHandle, portalHandle, g.org, "first"); err != nil {
			t.Fatalf("concurrent Deprecate failed: %v", err)
		}
	}
	if _, err := svc.Deprecate(context.Background(), apiType, apiHandle, portalHandle, g.org, "second"); !apperror.APIPublicationStateConflict.Is(err) {
		t.Fatalf("want APIPublicationStateConflict for the deprecate that lost the race, got %v", err)
	}
	live, err := svc.GetPublication(apiType, apiHandle, portalHandle, g.org)
	if err != nil {
		t.Fatalf("GetPublication failed: %v", err)
	}
	if live.Status != model.PublicationStatusDeprecated || live.UpdatedBy != "first" {
		t.Fatalf("want DEPRECATED by the first deprecate, got status=%s updatedBy=%s", live.Status, live.UpdatedBy)
	}
}

// Walks publish, unpublish, publish, deprecate, unpublish, then a refused deprecate.
func TestPublicationLifecycle_Cycle(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	svc := newPublicationTestServiceWith(tdb, &deprecateRecorder{})
	ctx := context.Background()
	apiType, apiHandle, portalHandle := "rest-api", apiHandleFor(g), portalHandleFor(g)

	saveAndPublish := func(version string) {
		t.Helper()
		draft := &model.Publication{DisplayName: "Cycle", Version: version, AgentVisibility: "VISIBLE"}
		if _, err := svc.SaveDraftDetails(apiType, apiHandle, portalHandle, g.org, "actor", draft, nil, nil); err != nil {
			t.Fatalf("SaveDraftDetails %s failed: %v", version, err)
		}
		if err := svc.SaveDraftDefinition(apiType, apiHandle, portalHandle, g.org, "actor", "application/json", []byte(minimalValidDefinition)); err != nil {
			t.Fatalf("SaveDraftDefinition %s failed: %v", version, err)
		}
		if _, _, err := svc.Publish(ctx, apiType, apiHandle, portalHandle, g.org, "actor"); err != nil {
			t.Fatalf("Publish %s failed: %v", version, err)
		}
	}
	unpublish := func() {
		t.Helper()
		if err := svc.Unpublish(ctx, apiType, apiHandle, portalHandle, g.org, "actor"); err != nil {
			t.Fatalf("Unpublish failed: %v", err)
		}
	}

	saveAndPublish("1.0")
	if got := liveStatus(t, tdb, svc, apiType, apiHandle, portalHandle, g.org); got != model.PublicationStatusPublished {
		t.Fatalf("after publish want PUBLISHED, got %s", got)
	}
	unpublish()
	if _, err := svc.GetPublication(apiType, apiHandle, portalHandle, g.org); !apperror.APIPublicationNotFound.Is(err) {
		t.Fatalf("after unpublish want no live listing, got %v", err)
	}
	// The unpublished listing is now the draft; publish it again.
	if _, _, err := svc.Publish(ctx, apiType, apiHandle, portalHandle, g.org, "actor"); err != nil {
		t.Fatalf("second Publish failed: %v", err)
	}
	if _, err := svc.Deprecate(ctx, apiType, apiHandle, portalHandle, g.org, "actor"); err != nil {
		t.Fatalf("Deprecate failed: %v", err)
	}
	if got := liveStatus(t, tdb, svc, apiType, apiHandle, portalHandle, g.org); got != model.PublicationStatusDeprecated {
		t.Fatalf("after deprecate want DEPRECATED, got %s", got)
	}
	unpublish()
	if _, err := svc.Deprecate(ctx, apiType, apiHandle, portalHandle, g.org, "actor"); !apperror.APIPublicationStateConflict.Is(err) {
		t.Fatalf("deprecate after unpublish want APIPublicationStateConflict, got %v", err)
	}

	// Publishing a deprecated listing returns it to PUBLISHED.
	saveAndPublish("2.0")
	if _, err := svc.Deprecate(ctx, apiType, apiHandle, portalHandle, g.org, "actor"); err != nil {
		t.Fatalf("second Deprecate failed: %v", err)
	}
	saveAndPublish("3.0")
	if got := liveStatus(t, tdb, svc, apiType, apiHandle, portalHandle, g.org); got != model.PublicationStatusPublished {
		t.Fatalf("publish after deprecate want PUBLISHED, got %s", got)
	}
}
