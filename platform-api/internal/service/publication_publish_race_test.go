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

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

// publishRecorder can run a hook during Publish, while the portal push is in flight.
type publishRecorder struct {
	alwaysSucceedsPortalPublisher
	onPublish func()
}

func (p *publishRecorder) Publish(_ context.Context, _ *model.APIPortal, _, _ string, _ *model.Publication, _ *model.PublicationContent) error {
	if p.onPublish != nil {
		p.onPublish()
	}
	return nil
}

// saveDraftDetails saves just the details half of the draft for the seeded
// API and portal, with the given description.
func saveDraftDetails(t *testing.T, tdb *schemaDB, svc *service.PublicationService, g graph, description string) {
	t.Helper()
	draft := &model.Publication{DisplayName: "Live Listing", Version: "1.0", Description: description, AgentVisibility: "VISIBLE"}
	if _, err := svc.SaveDraftDetails("rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "racer", draft, []string{planHandleFor(g)}, []string{docHandleFor(g)}); err != nil {
		t.Fatalf("SaveDraftDetails failed: %v", err)
	}
}

// saveListingDraft saves both halves of the draft for the seeded API and
// portal, with the given description.
func saveListingDraft(t *testing.T, tdb *schemaDB, svc *service.PublicationService, g graph, description string) {
	t.Helper()
	saveDraftDetails(t, tdb, svc, g, description)
	if err := svc.SaveDraftDefinition("rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "racer", "application/json", []byte(minimalValidDefinition)); err != nil {
		t.Fatalf("SaveDraftDefinition failed: %v", err)
	}
}

// A draft saved while its publish is in flight — details or content — is not promoted:
// the publish returns 409 and the live listing, or its absence, stays as it was.
func TestPublicationPublish_DraftEditedDuringPush(t *testing.T) {
	edits := []struct {
		name string
		edit func(t *testing.T, tdb *schemaDB, svc *service.PublicationService, g graph)
	}{
		{"details", func(t *testing.T, tdb *schemaDB, svc *service.PublicationService, g graph) {
			saveDraftDetails(t, tdb, svc, g, "edited during push")
		}},
		{"content", func(t *testing.T, tdb *schemaDB, svc *service.PublicationService, g graph) {
			if err := svc.SaveDraftLandingPage("rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "racer", []byte("# edited during push")); err != nil {
				t.Fatalf("concurrent SaveDraftLandingPage failed: %v", err)
			}
		}},
	}

	for _, tc := range edits {
		t.Run("first publish/"+tc.name, func(t *testing.T) {
			tdb := openSchemaDB(t)
			g := seedOrgGraph(t, tdb)
			portal := &publishRecorder{}
			svc := newPublicationTestServiceWith(tdb, portal)
			apiType, apiHandle, portalHandle := "rest-api", apiHandleFor(g), portalHandleFor(g)

			saveListingDraft(t, tdb, svc, g, "original")
			portal.onPublish = func() { tc.edit(t, tdb, svc, g) }

			if _, _, err := svc.Publish(context.Background(), apiType, apiHandle, portalHandle, g.org, "publisher"); !apperror.APIPublicationDraftChanged.Is(err) {
				t.Fatalf("want APIPublicationDraftChanged, got %v", err)
			}
			if _, err := svc.GetPublication(apiType, apiHandle, portalHandle, g.org); !apperror.APIPublicationNotFound.Is(err) {
				t.Fatalf("want no live listing after the refused publish, got %v", err)
			}
			if _, err := svc.GetDraft(apiType, apiHandle, portalHandle, g.org); err != nil {
				t.Fatalf("want the edited draft kept, got %v", err)
			}

			portal.onPublish = nil
			if _, _, err := svc.Publish(context.Background(), apiType, apiHandle, portalHandle, g.org, "publisher"); err != nil {
				t.Fatalf("publishing again after the conflict failed: %v", err)
			}
		})

		t.Run("republish/"+tc.name, func(t *testing.T) {
			tdb := openSchemaDB(t)
			g := seedOrgGraph(t, tdb)
			portal := &publishRecorder{}
			svc := newPublicationTestServiceWith(tdb, portal)
			apiType, apiHandle, portalHandle := publishedListing(t, tdb, svc, g)
			saveListingDraft(t, tdb, svc, g, "next")
			portal.onPublish = func() { tc.edit(t, tdb, svc, g) }

			if _, _, err := svc.Publish(context.Background(), apiType, apiHandle, portalHandle, g.org, "publisher"); !apperror.APIPublicationDraftChanged.Is(err) {
				t.Fatalf("want APIPublicationDraftChanged, got %v", err)
			}
			live, err := svc.GetPublication(apiType, apiHandle, portalHandle, g.org)
			if err != nil {
				t.Fatalf("GetPublication failed: %v", err)
			}
			if live.Description != "live" {
				t.Fatalf("want the live listing untouched, got description %q", live.Description)
			}
		})
	}
}
