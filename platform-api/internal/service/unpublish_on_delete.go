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

package service

import (
	"context"
	"fmt"

	"github.com/wso2/api-platform/platform-api/internal/model"
)

// ArtifactUnpublisher removes an API's listings from every API Portal it is
// published on. An API's delete calls it first: deleting the artifact cascades
// away its api_publications rows, and a portal is never told, so it would keep
// listing and serving the deleted API with nothing left here to unpublish it.
type ArtifactUnpublisher interface {
	UnpublishEverywhere(ctx context.Context, apiType, apiHandle, artifactUUID, orgUUID, actor string) error
}

// unpublishBeforeDelete runs u for the artifact when there is one. A nil u
// (a build without API Portals) has nothing to unpublish.
func unpublishBeforeDelete(u ArtifactUnpublisher, apiType, apiHandle, artifactUUID, orgUUID, actor string) error {
	if u == nil {
		return nil
	}
	return u.UnpublishEverywhere(context.Background(), apiType, apiHandle, artifactUUID, orgUUID, actor)
}

// unpublishPasses bounds how often UnpublishEverywhere re-reads the artifact's listings.
// A publish to another portal can land while a pass is running: Publish pushes to the
// portal before it writes its row, so a pass's snapshot can miss it. Each further pass
// picks up such a listing; one with nothing new to unpublish ends the loop.
const unpublishPasses = 3

// UnpublishEverywhere implements ArtifactUnpublisher. For each live listing
// (PUBLISHED or DEPRECATED) it unpublishes the API on that portal, then marks
// the listing unpublished here, so a delete that stops part way leaves every
// row matching its portal. The portal is asked to force the removal, so the
// listing's subscriptions and active API keys go with it.
//
// A portal that cannot be reached, or that still refuses (a paid subscription),
// fails the call, and the caller does not delete: deleting anyway would leave
// that portal serving the API with no way to withdraw it. A portal that is not
// active (pending or failed) has no running portal to hold the listing and is
// skipped. Draft rows have nothing on any portal and are left to the delete's
// cascade.
func (s *PublicationService) UnpublishEverywhere(ctx context.Context, apiType, apiHandle, artifactUUID, orgUUID, actor string) error {
	for pass := 0; pass < unpublishPasses; pass++ {
		unpublished, err := s.unpublishLiveListings(ctx, apiType, apiHandle, artifactUUID, orgUUID, actor)
		if err != nil {
			return err
		}
		if unpublished == 0 {
			return nil
		}
	}
	return nil
}

// unpublishLiveListings runs one pass over the artifact's listings and returns how
// many it unpublished.
func (s *PublicationService) unpublishLiveListings(ctx context.Context, apiType, apiHandle, artifactUUID, orgUUID, actor string) (int, error) {
	rows, err := s.publicationRepo.ListStatusByArtifact(artifactUUID, orgUUID)
	if err != nil {
		return 0, fmt.Errorf("failed to list the API's publications: %w", err)
	}
	unpublished := 0
	for _, row := range rows {
		if row.IsDraft || (row.Status != model.PublicationStatusPublished && row.Status != model.PublicationStatusDeprecated) {
			continue
		}
		portal, err := s.apiPortalRepo.GetByUUID(row.APIPortalUUID, orgUUID)
		if err != nil {
			return 0, fmt.Errorf("failed to resolve API Portal %s: %w", row.APIPortalUUID, err)
		}
		if portal == nil {
			continue
		}
		if !portal.IsActive() {
			s.slogger.Warn("Skipping unpublish on a portal that is not active",
				"apiType", apiType, "apiHandle", apiHandle, "apiPortal", portal.Handle, "status", portal.Status)
			continue
		}
		if err := s.removeFromPortal(ctx, portal, apiType, apiHandle); err != nil {
			s.slogger.Error("API not deleted: could not unpublish it from an API Portal",
				"apiType", apiType, "apiHandle", apiHandle, "apiPortal", portal.Handle, "error", err)
			return 0, portalPushError(err)
		}
		if _, err := s.publicationRepo.UnpublishPublication(artifactUUID, portal.ID, orgUUID, actor); err != nil {
			return 0, fmt.Errorf("failed to mark the publication on API Portal %s unpublished: %w", portal.Handle, err)
		}
		unpublished++
	}
	return unpublished, nil
}

// removeFromPortal takes the listing off portal for good: forced when the publisher
// supports it, so its subscriptions and active API keys go with it, and a plain
// unpublish otherwise.
func (s *PublicationService) removeFromPortal(ctx context.Context, portal *model.APIPortal, apiType, apiHandle string) error {
	ctx, cancel := s.portalOperationContext(ctx)
	defer cancel()
	if forcer, ok := s.portalPublisher.(ForceUnpublisher); ok {
		return forcer.ForceUnpublish(ctx, portal, apiType, apiHandle)
	}
	return s.portalPublisher.Unpublish(ctx, portal, apiType, apiHandle)
}
