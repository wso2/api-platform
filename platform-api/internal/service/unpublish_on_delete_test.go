/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

// unpublishRecorder records every portal Unpublish call and can fail them.
type unpublishRecorder struct {
	alwaysSucceedsPortalPublisher
	portals []string
	err     error
}

func (u *unpublishRecorder) Unpublish(_ context.Context, portal *model.APIPortal, _, _ string) error {
	u.portals = append(u.portals, portal.Handle)
	return u.err
}

// addPortal inserts a second API Portal into g's organization with the given status.
func addPortal(t *testing.T, tdb *schemaDB, g graph, status string) string {
	t.Helper()
	uuid := id()
	handle := "portal-" + uuid[:8]
	tdb.exec(t, `INSERT INTO api_portals (uuid, organization_uuid, handle, display_name, status, internal_auth_key, metadata) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		uuid, g.org, handle, "portal", status, []byte("dummy-key"), []byte("{}"))
	return handle
}

// saveDraftOn saves a minimal draft for the API on the portal.
func saveDraftOn(t *testing.T, svc *service.PublicationService, apiHandle, portalHandle, org string) {
	t.Helper()
	draft := &model.Publication{DisplayName: "Listing", Version: "1.0", AgentVisibility: "VISIBLE"}
	if _, err := svc.SaveDraftDetails("rest-api", apiHandle, portalHandle, org, "actor", draft, nil, nil); err != nil {
		t.Fatalf("SaveDraftDetails failed: %v", err)
	}
	if err := svc.SaveDraftDefinition("rest-api", apiHandle, portalHandle, org, "actor", "application/json", []byte(minimalValidDefinition)); err != nil {
		t.Fatalf("SaveDraftDefinition failed: %v", err)
	}
}

// Every live listing (published or deprecated) is unpublished on its portal and
// marked unpublished here; a portal holding only a draft is never called.
func TestUnpublishEverywhere_UnpublishesEveryLiveListing(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	rec := &unpublishRecorder{}
	svc := newPublicationTestServiceWith(tdb, rec)
	apiHandle := apiHandleFor(g)

	published := portalHandleFor(g)
	saveDraftOn(t, svc, apiHandle, published, g.org)
	if _, _, err := svc.Publish(context.Background(), "rest-api", apiHandle, published, g.org, "actor"); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	deprecated := addPortal(t, tdb, g, "active")
	saveDraftOn(t, svc, apiHandle, deprecated, g.org)
	if _, _, err := svc.Publish(context.Background(), "rest-api", apiHandle, deprecated, g.org, "actor"); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	if _, err := svc.Deprecate(context.Background(), "rest-api", apiHandle, deprecated, g.org, "actor"); err != nil {
		t.Fatalf("Deprecate failed: %v", err)
	}

	draftOnly := addPortal(t, tdb, g, "active")
	saveDraftOn(t, svc, apiHandle, draftOnly, g.org)

	if err := svc.UnpublishEverywhere(context.Background(), "rest-api", apiHandle, g.apiArtifact, g.org, "deleter"); err != nil {
		t.Fatalf("UnpublishEverywhere failed: %v", err)
	}

	got := map[string]bool{}
	for _, p := range rec.portals {
		got[p] = true
	}
	if len(rec.portals) != 2 || !got[published] || !got[deprecated] {
		t.Fatalf("want Unpublish on %s and %s only, got %v", published, deprecated, rec.portals)
	}
	for _, p := range []string{published, deprecated} {
		if _, err := svc.GetPublication("rest-api", apiHandle, p, g.org); !apperror.APIPublicationNotFound.Is(err) {
			t.Fatalf("want no live listing on %s after UnpublishEverywhere, got %v", p, err)
		}
	}
}

// A portal that is not active has no running portal to hold the listing, so it is
// skipped rather than blocking the delete.
func TestUnpublishEverywhere_SkipsAPortalThatIsNotActive(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	rec := &unpublishRecorder{}
	svc := newPublicationTestServiceWith(tdb, rec)
	apiHandle, portalHandle := apiHandleFor(g), portalHandleFor(g)

	saveDraftOn(t, svc, apiHandle, portalHandle, g.org)
	if _, _, err := svc.Publish(context.Background(), "rest-api", apiHandle, portalHandle, g.org, "actor"); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	tdb.exec(t, `UPDATE api_portals SET status = ? WHERE uuid = ?`, "failed", g.apiPortal)

	if err := svc.UnpublishEverywhere(context.Background(), "rest-api", apiHandle, g.apiArtifact, g.org, "deleter"); err != nil {
		t.Fatalf("UnpublishEverywhere failed: %v", err)
	}
	if len(rec.portals) != 0 {
		t.Fatalf("want no portal call for a failed portal, got %v", rec.portals)
	}
}

// A portal that cannot be reached fails the call with the publication service's
// unavailable error, and the live listing stays as it was.
func TestUnpublishEverywhere_FailsWhenAPortalCannotBeReached(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	rec := &unpublishRecorder{err: errors.New("connection refused")}
	svc := newPublicationTestServiceWith(tdb, rec)
	apiHandle, portalHandle := apiHandleFor(g), portalHandleFor(g)

	saveDraftOn(t, svc, apiHandle, portalHandle, g.org)
	live, _, err := svc.Publish(context.Background(), "rest-api", apiHandle, portalHandle, g.org, "actor")
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	err = svc.UnpublishEverywhere(context.Background(), "rest-api", apiHandle, g.apiArtifact, g.org, "deleter")
	if !apperror.APIPublicationPortalUnavailable.Is(err) {
		t.Fatalf("want APIPublicationPortalUnavailable, got %v", err)
	}
	still, err := svc.GetPublication("rest-api", apiHandle, portalHandle, g.org)
	if err != nil || still.UUID != live.UUID {
		t.Fatalf("want the live listing untouched, got %+v, %v", still, err)
	}
}

// An API that was never published has nothing to unpublish.
func TestUnpublishEverywhere_NoPublications(t *testing.T) {
	tdb := openSchemaDB(t)
	g := seedOrgGraph(t, tdb)
	rec := &unpublishRecorder{err: errors.New("must not be called")}
	svc := newPublicationTestServiceWith(tdb, rec)

	if err := svc.UnpublishEverywhere(context.Background(), "rest-api", apiHandleFor(g), g.apiArtifact, g.org, "deleter"); err != nil {
		t.Fatalf("UnpublishEverywhere failed: %v", err)
	}
	if len(rec.portals) != 0 {
		t.Fatalf("want no portal call, got %v", rec.portals)
	}
}
