//go:build integration

/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied. See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

package integration

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

func (p *publishRecorder) Publish(_ context.Context, _ *model.APIPortal, _ string, _ *model.Publication, _ *model.PublicationContent) error {
	if p.onPublish != nil {
		p.onPublish()
	}
	return nil
}

// saveListingDraft saves the draft for the seeded API and portal with the given description.
func saveListingDraft(t *testing.T, it *itDB, svc *service.PublicationService, g graph, description string) {
	t.Helper()
	draft := &model.Publication{DisplayName: "Live Listing", Version: "1.0", Description: description, AgentVisibility: "VISIBLE"}
	if _, err := svc.SaveDraftDetails("rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "racer", draft, []string{planHandleFor(g)}, []string{docHandleFor(g)}); err != nil {
		t.Fatalf("[%s] SaveDraftDetails failed: %v", it.driver, err)
	}
}

// A draft saved while its publish is in flight — details or content — is not promoted:
// the publish returns 409 and the live listing, or its absence, stays as it was.
func TestPublicationPublish_DraftEditedDuringPush(t *testing.T) {
	edits := []struct {
		name string
		edit func(t *testing.T, it *itDB, svc *service.PublicationService, g graph)
	}{
		{"details", func(t *testing.T, it *itDB, svc *service.PublicationService, g graph) {
			saveListingDraft(t, it, svc, g, "edited during push")
		}},
		{"content", func(t *testing.T, it *itDB, svc *service.PublicationService, g graph) {
			if err := svc.SaveDraftLandingPage("rest-api", apiHandleFor(g), portalHandleFor(g), g.org, "racer", []byte("# edited during push")); err != nil {
				t.Fatalf("[%s] concurrent SaveDraftLandingPage failed: %v", it.driver, err)
			}
		}},
	}

	for _, tc := range edits {
		t.Run("first publish/"+tc.name, func(t *testing.T) {
			it := openITDB(t)
			defer it.db.Close()
			g := seedOrgGraph(t, it)
			portal := &publishRecorder{}
			svc := newPublicationTestServiceWith(it, portal)
			apiType, apiHandle, portalHandle := "rest-api", apiHandleFor(g), portalHandleFor(g)

			saveListingDraft(t, it, svc, g, "original")
			portal.onPublish = func() { tc.edit(t, it, svc, g) }

			if _, _, err := svc.Publish(context.Background(), apiType, apiHandle, portalHandle, g.org, "publisher"); !apperror.APIPublicationDraftChanged.Is(err) {
				t.Fatalf("[%s] want APIPublicationDraftChanged, got %v", it.driver, err)
			}
			if _, err := svc.GetPublication(apiType, apiHandle, portalHandle, g.org); !apperror.APIPublicationNotFound.Is(err) {
				t.Fatalf("[%s] want no live listing after the refused publish, got %v", it.driver, err)
			}
			if _, err := svc.GetDraft(apiType, apiHandle, portalHandle, g.org); err != nil {
				t.Fatalf("[%s] want the edited draft kept, got %v", it.driver, err)
			}

			portal.onPublish = nil
			if _, _, err := svc.Publish(context.Background(), apiType, apiHandle, portalHandle, g.org, "publisher"); err != nil {
				t.Fatalf("[%s] publishing again after the conflict failed: %v", it.driver, err)
			}
		})

		t.Run("republish/"+tc.name, func(t *testing.T) {
			it := openITDB(t)
			defer it.db.Close()
			g := seedOrgGraph(t, it)
			portal := &publishRecorder{}
			svc := newPublicationTestServiceWith(it, portal)
			apiType, apiHandle, portalHandle := publishedListing(t, it, svc, g)
			saveListingDraft(t, it, svc, g, "next")
			portal.onPublish = func() { tc.edit(t, it, svc, g) }

			if _, _, err := svc.Publish(context.Background(), apiType, apiHandle, portalHandle, g.org, "publisher"); !apperror.APIPublicationDraftChanged.Is(err) {
				t.Fatalf("[%s] want APIPublicationDraftChanged, got %v", it.driver, err)
			}
			live, err := svc.GetPublication(apiType, apiHandle, portalHandle, g.org)
			if err != nil {
				t.Fatalf("[%s] GetPublication failed: %v", it.driver, err)
			}
			if live.Description != "live" {
				t.Fatalf("[%s] want the live listing untouched, got description %q", it.driver, live.Description)
			}
		})
	}
}
