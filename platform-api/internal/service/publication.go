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
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// definitionFileNamesByContentType maps an accepted definition Content-Type to
// the canonical file name it's stored under, matching the API Portal's own
// constants. No other media type is accepted.
var definitionFileNamesByContentType = map[string]string{
	"application/json":    "definition.json",
	"application/x-yaml":  "definition.yaml",
	"application/graphql": "definition.graphql",
	"application/xml":     "definition.xml",
}

// PublicationService implements the API Publication draft and the live
// publication/rollup/lifecycle operations.
type PublicationService struct {
	artifactRepo         repository.ArtifactRepository
	apiPortalRepo        repository.APIPortalRepository
	documentRepo         repository.DocumentRepository
	subscriptionPlanRepo repository.SubscriptionPlanRepository
	publicationRepo      repository.PublicationRepository
	portalPublisher      PortalPublisher
	slogger              *slog.Logger
}

// NewPublicationService creates a new API Publication service.
func NewPublicationService(
	artifactRepo repository.ArtifactRepository,
	apiPortalRepo repository.APIPortalRepository,
	documentRepo repository.DocumentRepository,
	subscriptionPlanRepo repository.SubscriptionPlanRepository,
	publicationRepo repository.PublicationRepository,
	portalPublisher PortalPublisher,
	slogger *slog.Logger,
) *PublicationService {
	if slogger == nil {
		slogger = slog.Default()
	}
	return &PublicationService{
		artifactRepo:         artifactRepo,
		apiPortalRepo:        apiPortalRepo,
		documentRepo:         documentRepo,
		subscriptionPlanRepo: subscriptionPlanRepo,
		publicationRepo:      publicationRepo,
		portalPublisher:      portalPublisher,
		slogger:              slogger,
	}
}

// resolveArtifact resolves (apiType, apiId) to the artifact's internal UUID:
// an unrecognised apiType and an unknown apiId both resolve to the same 404
// (API_NOT_FOUND) — not a distinguishable 400, which would let a caller
// enumerate installed plugins.
func (s *PublicationService) resolveArtifact(apiType, apiId, orgUUID string) (string, error) {
	if apiType == "" || apiId == "" {
		return "", apperror.APIPublicationAPINotFound.New()
	}
	metadata, err := s.artifactRepo.GetAPIMetadataByHandleAndKind(apiId, apiType, orgUUID)
	if errors.Is(err, repository.ErrUnknownArtifactKind) {
		return "", apperror.APIPublicationAPINotFound.New()
	}
	if err != nil {
		return "", fmt.Errorf("failed to resolve API by handle and kind: %w", err)
	}
	if metadata == nil {
		return "", apperror.APIPublicationAPINotFound.New()
	}
	return metadata.ID, nil
}

// resolvePortal resolves apiPortalId (handle) to the portal's internal UUID.
func (s *PublicationService) resolvePortal(apiPortalId, orgUUID string) (string, error) {
	portal, err := s.resolvePortalRow(apiPortalId, orgUUID)
	if err != nil {
		return "", err
	}
	return portal.ID, nil
}

// resolvePortalRow is resolvePortal's counterpart for callers that also need
// the portal's own handle/display name (the live publication read, for its
// apiPortalId/apiPortalName response fields).
func (s *PublicationService) resolvePortalRow(apiPortalId, orgUUID string) (*model.APIPortal, error) {
	if apiPortalId == "" {
		return nil, apperror.APIPortalNotFound.New()
	}
	portal, err := s.apiPortalRepo.GetByHandleAndOrgID(apiPortalId, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve API Portal by handle: %w", err)
	}
	if portal == nil {
		return nil, apperror.APIPortalNotFound.New()
	}
	return portal, nil
}

// getDraftRow resolves the API and API Portal, then loads the draft row (with
// resolved plan/document handles). Returns APIPublicationDraftNotFound when
// none has been saved — the shared precondition every draft content
// operation (definition/landing-page/thumbnail get and save) needs.
func (s *PublicationService) getDraftRow(apiType, apiId, apiPortalId, orgUUID string) (*model.Publication, error) {
	artifactUUID, err := s.resolveArtifact(apiType, apiId, orgUUID)
	if err != nil {
		return nil, err
	}
	portalUUID, err := s.resolvePortal(apiPortalId, orgUUID)
	if err != nil {
		return nil, err
	}

	pub, planUUIDs, docUUIDs, err := s.publicationRepo.GetDraft(artifactUUID, portalUUID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get publication draft: %w", err)
	}
	if pub == nil {
		return nil, apperror.APIPublicationDraftNotFound.New()
	}
	if err := s.resolveHandles(pub, planUUIDs, docUUIDs, orgUUID); err != nil {
		return nil, err
	}
	return pub, nil
}

// resolveHandles fills pub.SubscriptionPlanIds/DocIds from the raw UUIDs the
// mapping tables store. Entries that fail to resolve (should not normally
// happen — subscription_plans is ON DELETE RESTRICT and api_documents rows
// cascade their mapping row away on delete) are silently dropped rather than
// failing the read, so a GET stays robust against any inconsistency.
func (s *PublicationService) resolveHandles(pub *model.Publication, planUUIDs, docUUIDs []string, orgUUID string) error {
	planHandles, err := s.subscriptionPlanRepo.GetHandlesByIDs(planUUIDs, orgUUID)
	if err != nil {
		return fmt.Errorf("failed to resolve subscription plan handles: %w", err)
	}
	pub.SubscriptionPlanIds = mapValuesInOrder(planUUIDs, planHandles)

	docHandles, err := s.documentRepo.GetDocumentHandlesByUUIDs(docUUIDs, orgUUID)
	if err != nil {
		return fmt.Errorf("failed to resolve document handles: %w", err)
	}
	pub.DocIds = mapValuesInOrder(docUUIDs, docHandles)
	return nil
}

// conflictReasonOrDefault returns conflict.Reason, or defaultPortalConflictReason
// when a PortalPublisher left it unset, so the conflict message is never blank.
func conflictReasonOrDefault(conflict *PortalConflictError) string {
	if conflict.Reason == "" {
		return defaultPortalConflictReason
	}
	return conflict.Reason
}

// mapValuesInOrder looks up each id in m, preserving ids's order and skipping
// any id absent from m. Always returns a non-nil slice so it serializes as
// "[]" rather than "null" when empty.
func mapValuesInOrder(ids []string, m map[string]string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if v, ok := m[id]; ok {
			out = append(out, v)
		}
	}
	return out
}

// dedupe removes duplicate handles, preserving first-occurrence order. A
// handle repeated in the request body would otherwise violate the mapping
// table's composite primary key.
func dedupe(handles []string) []string {
	seen := make(map[string]struct{}, len(handles))
	out := make([]string, 0, len(handles))
	for _, h := range handles {
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		out = append(out, h)
	}
	return out
}

// GetDraft returns the draft's details, subscription plans and documents.
func (s *PublicationService) GetDraft(apiType, apiId, apiPortalId, orgUUID string) (*model.Publication, error) {
	return s.getDraftRow(apiType, apiId, apiPortalId, orgUUID)
}

// SaveDraftDetails creates the draft on first save, or replaces it in full,
// resolving subscriptionPlanIds/docIds handles to their internal UUIDs and
// rejecting any handle absent from the organization's catalog.
func (s *PublicationService) SaveDraftDetails(apiType, apiId, apiPortalId, orgUUID, actor string, draft *model.Publication, planHandles, docHandles []string) (*model.Publication, error) {
	artifactUUID, err := s.resolveArtifact(apiType, apiId, orgUUID)
	if err != nil {
		return nil, err
	}
	portalUUID, err := s.resolvePortal(apiPortalId, orgUUID)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(draft.DisplayName) == "" {
		return nil, apperror.APIPublicationValidationFailed.New("displayName is required")
	}
	if strings.TrimSpace(draft.Version) == "" {
		return nil, apperror.APIPublicationValidationFailed.New("version is required")
	}
	switch draft.AgentVisibility {
	case "":
		draft.AgentVisibility = "VISIBLE"
	case "VISIBLE", "HIDDEN":
	default:
		return nil, apperror.APIPublicationValidationFailed.New("agentVisibility must be VISIBLE or HIDDEN")
	}

	planHandles = dedupe(planHandles)
	docHandles = dedupe(docHandles)

	planUUIDs, err := s.resolvePlanUUIDs(planHandles, orgUUID)
	if err != nil {
		return nil, err
	}
	docUUIDs, err := s.resolveDocUUIDs(artifactUUID, docHandles, orgUUID)
	if err != nil {
		return nil, err
	}

	draft.OrganizationUUID = orgUUID
	draft.ArtifactUUID = artifactUUID
	draft.APIPortalUUID = portalUUID
	draft.IsDraft = true

	saved, err := s.publicationRepo.SaveDraftDetails(draft, planUUIDs, docUUIDs, actor)
	if err != nil {
		return nil, fmt.Errorf("failed to save publication draft: %w", err)
	}
	// Echo back exactly what was validated and stored, rather than a second
	// round trip through the mapping tables to re-resolve UUIDs to handles.
	saved.SubscriptionPlanIds = nonNil(planHandles)
	saved.DocIds = nonNil(docHandles)
	return saved, nil
}

// nonNil returns s, or a non-nil empty slice when s is nil, so the field
// serializes as "[]" rather than "null".
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// resolvePlanUUIDs resolves each subscription plan handle to its UUID,
// rejecting any handle absent from the organization's catalog.
func (s *PublicationService) resolvePlanUUIDs(handles []string, orgUUID string) ([]string, error) {
	if len(handles) == 0 {
		return nil, nil
	}
	resolved, err := s.subscriptionPlanRepo.GetUUIDsByHandles(handles, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve subscription plan handles: %w", err)
	}
	return uuidsForHandles(handles, resolved, "subscriptionPlanIds not found in the organization's catalog")
}

// resolveDocUUIDs resolves each document handle to its doc_uuid, scoped to
// artifactUUID (a handle can legitimately repeat across two APIs in the same
// org), rejecting any handle absent for this artifact.
func (s *PublicationService) resolveDocUUIDs(artifactUUID string, handles []string, orgUUID string) ([]string, error) {
	if len(handles) == 0 {
		return nil, nil
	}
	resolved, err := s.documentRepo.GetDocumentUUIDsByHandles(artifactUUID, handles, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve document handles: %w", err)
	}
	return uuidsForHandles(handles, resolved, "docIds not found in the organization's documents")
}

// uuidsForHandles returns the UUID of every handle in order, or a validation
// error listing the handles that have none.
func uuidsForHandles(handles []string, resolved map[string]string, notFoundMessage string) ([]string, error) {
	uuids := make([]string, 0, len(handles))
	var unresolved []string
	for _, h := range handles {
		if id, ok := resolved[h]; ok {
			uuids = append(uuids, id)
		} else {
			unresolved = append(unresolved, h)
		}
	}
	if len(unresolved) > 0 {
		return nil, apperror.APIPublicationValidationFailed.New(
			fmt.Sprintf("%s: %s", notFoundMessage, strings.Join(unresolved, ", ")))
	}
	return uuids, nil
}

// GetDraftDefinition returns the draft's stored definition content.
func (s *PublicationService) GetDraftDefinition(apiType, apiId, apiPortalId, orgUUID string) (*model.PublicationContent, error) {
	return s.getDraftContent(apiType, apiId, apiPortalId, orgUUID, model.PublicationContentTypeDefinition)
}

// SaveDraftDefinition replaces the draft's definition. contentTypeHeader must
// be one of the four accepted serializations; it selects both the stored
// Content-Type and the canonical file name recorded alongside it.
func (s *PublicationService) SaveDraftDefinition(apiType, apiId, apiPortalId, orgUUID, actor, contentTypeHeader string, data []byte) error {
	fileName, ok := definitionFileNamesByContentType[contentTypeHeader]
	if !ok {
		return apperror.APIPublicationValidationFailed.New(
			"Content-Type must be one of application/json, application/x-yaml, application/graphql, application/xml")
	}
	pub, err := s.getDraftRow(apiType, apiId, apiPortalId, orgUUID)
	if err != nil {
		return err
	}
	content := &model.PublicationContent{
		OrganizationUUID: orgUUID,
		PublicationUUID:  pub.UUID,
		Type:             model.PublicationContentTypeDefinition,
		FileName:         fileName,
		ContentType:      contentTypeHeader,
		Content:          data,
	}
	if err := s.publicationRepo.SaveContent(content, actor); err != nil {
		return fmt.Errorf("failed to save publication draft definition: %w", err)
	}
	return nil
}

// GetDraftLandingPage returns the draft's stored landing page content.
func (s *PublicationService) GetDraftLandingPage(apiType, apiId, apiPortalId, orgUUID string) (*model.PublicationContent, error) {
	return s.getDraftContent(apiType, apiId, apiPortalId, orgUUID, model.PublicationContentTypeMarketing)
}

// SaveDraftLandingPage replaces the draft's landing page. Embedded raw HTML
// is stripped before storage — stricter than the portal's own handling, and
// defense-in-depth on top of it, not a substitute.
func (s *PublicationService) SaveDraftLandingPage(apiType, apiId, apiPortalId, orgUUID, actor string, markdown []byte) error {
	pub, err := s.getDraftRow(apiType, apiId, apiPortalId, orgUUID)
	if err != nil {
		return err
	}
	sanitized := utils.StripEmbeddedHTML(string(markdown))
	content := &model.PublicationContent{
		OrganizationUUID: orgUUID,
		PublicationUUID:  pub.UUID,
		Type:             model.PublicationContentTypeMarketing,
		ContentType:      "text/markdown",
		Content:          []byte(sanitized),
	}
	if err := s.publicationRepo.SaveContent(content, actor); err != nil {
		return fmt.Errorf("failed to save publication draft landing page: %w", err)
	}
	return nil
}

// GetDraftThumbnail returns the draft's stored thumbnail content.
func (s *PublicationService) GetDraftThumbnail(apiType, apiId, apiPortalId, orgUUID string) (*model.PublicationContent, error) {
	return s.getDraftContent(apiType, apiId, apiPortalId, orgUUID, model.PublicationContentTypeImage)
}

// SaveDraftThumbnail replaces the draft's thumbnail. The content type is
// sniffed from the uploaded bytes (file-access.md) — PNG and JPEG only, never
// the declared Content-Type or file name. GIF/WebP/SVG are rejected: the API
// Portal's own image-serving code resolves a correct Content-Type only for
// PNG/JPEG, and SVG is XML that can carry scripts.
func (s *PublicationService) SaveDraftThumbnail(apiType, apiId, apiPortalId, orgUUID, actor, fileName string, data []byte) error {
	sniffed := http.DetectContentType(data)
	if sniffed != "image/png" && sniffed != "image/jpeg" {
		return apperror.APIPublicationValidationFailed.New("thumbnail must be a PNG or JPEG image")
	}
	pub, err := s.getDraftRow(apiType, apiId, apiPortalId, orgUUID)
	if err != nil {
		return err
	}
	content := &model.PublicationContent{
		OrganizationUUID: orgUUID,
		PublicationUUID:  pub.UUID,
		Type:             model.PublicationContentTypeImage,
		FileName:         fileName,
		ContentType:      sniffed,
		Content:          data,
	}
	if err := s.publicationRepo.SaveContent(content, actor); err != nil {
		return fmt.Errorf("failed to save publication draft thumbnail: %w", err)
	}
	return nil
}

// getDraftContent loads the draft's content row of the given type, returning
// a generic NotFound (not APIPublicationDraftNotFound) when the draft exists
// but has not stored this particular piece — DRAFT_NOT_FOUND specifically
// means no draft has been saved at all, which would be misleading here.
func (s *PublicationService) getDraftContent(apiType, apiId, apiPortalId, orgUUID string, contentType model.PublicationContentType) (*model.PublicationContent, error) {
	pub, err := s.getDraftRow(apiType, apiId, apiPortalId, orgUUID)
	if err != nil {
		return nil, err
	}
	content, err := s.publicationRepo.GetContent(pub.UUID, contentType, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get publication draft content: %w", err)
	}
	if content == nil {
		return nil, apperror.NotFound.New()
	}
	return content, nil
}

// getPublicationRow resolves the API and API Portal, then loads the live
// (is_draft = 0) row — GetDraft's read-only counterpart. Returns
// APIPublicationNotFound when this API has no live listing on this portal.
func (s *PublicationService) getPublicationRow(apiType, apiId, apiPortalId, orgUUID string) (*model.Publication, error) {
	artifactUUID, err := s.resolveArtifact(apiType, apiId, orgUUID)
	if err != nil {
		return nil, err
	}
	portal, err := s.resolvePortalRow(apiPortalId, orgUUID)
	if err != nil {
		return nil, err
	}

	pub, planUUIDs, docUUIDs, err := s.publicationRepo.GetPublication(artifactUUID, portal.ID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get publication: %w", err)
	}
	if pub == nil {
		return nil, apperror.APIPublicationNotFound.New()
	}
	if err := s.resolveHandles(pub, planUUIDs, docUUIDs, orgUUID); err != nil {
		return nil, err
	}
	pub.APIPortalHandle = portal.Handle
	pub.APIPortalName = portal.Name
	return pub, nil
}

// GetPublication returns the live listing's details, subscription plans and
// documents.
func (s *PublicationService) GetPublication(apiType, apiId, apiPortalId, orgUUID string) (*model.Publication, error) {
	return s.getPublicationRow(apiType, apiId, apiPortalId, orgUUID)
}

// GetPublicationDefinition returns the live listing's stored definition
// content.
func (s *PublicationService) GetPublicationDefinition(apiType, apiId, apiPortalId, orgUUID string) (*model.PublicationContent, error) {
	return s.getPublicationContent(apiType, apiId, apiPortalId, orgUUID, model.PublicationContentTypeDefinition)
}

// GetPublicationLandingPage returns the live listing's stored landing page
// content.
func (s *PublicationService) GetPublicationLandingPage(apiType, apiId, apiPortalId, orgUUID string) (*model.PublicationContent, error) {
	return s.getPublicationContent(apiType, apiId, apiPortalId, orgUUID, model.PublicationContentTypeMarketing)
}

// GetPublicationThumbnail returns the live listing's stored thumbnail
// content.
func (s *PublicationService) GetPublicationThumbnail(apiType, apiId, apiPortalId, orgUUID string) (*model.PublicationContent, error) {
	return s.getPublicationContent(apiType, apiId, apiPortalId, orgUUID, model.PublicationContentTypeImage)
}

// getPublicationContent loads the live listing's content row of the given
// type, returning a generic NotFound (not APIPublicationNotFound) when the
// listing exists but has not stored this particular piece — mirrors
// getDraftContent's same distinction for the draft tier.
func (s *PublicationService) getPublicationContent(apiType, apiId, apiPortalId, orgUUID string, contentType model.PublicationContentType) (*model.PublicationContent, error) {
	pub, err := s.getPublicationRow(apiType, apiId, apiPortalId, orgUUID)
	if err != nil {
		return nil, err
	}
	content, err := s.publicationRepo.GetContent(pub.UUID, contentType, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get publication content: %w", err)
	}
	if content == nil {
		return nil, apperror.NotFound.New()
	}
	return content, nil
}

// Publish publishes the current draft to the API Portal. In the UI's own
// flow, the client always saves the draft (draft PUT) immediately before
// calling this bodyless action, so a draft is guaranteed to exist and
// already validated by the time this runs — APIPublicationDraftNotFound
// here is a defensive, fail-closed check for a client bug or a failed prior
// save proceeding anyway, not a normal user-facing gate. Publish itself
// validates nothing further.
//
// The portal is pushed first: nothing local changes unless that succeeds. On
// success, one transaction (PublicationRepository.PromoteDraftToPublication):
// on a first publish, flip the draft row in place — same row, same uuid; on
// a republish, merge the draft's content into the existing live row (the
// anchor) instead, so the anchor's uuid — the durable identity for this
// (API, portal) pairing — never changes across a republish. A repeat publish
// with no intervening edit goes through the same path and is a normal
// no-op refresh, not an error.
//
// A draft saved during the portal push is not promoted (APIPublicationDraftChanged).
// The push is not undone, so the portal may differ until the caller publishes again.
func (s *PublicationService) Publish(ctx context.Context, apiType, apiId, apiPortalId, orgUUID, actor string) (*model.Publication, bool, error) {
	artifactUUID, err := s.resolveArtifact(apiType, apiId, orgUUID)
	if err != nil {
		return nil, false, err
	}
	portal, err := s.resolvePortalRow(apiPortalId, orgUUID)
	if err != nil {
		return nil, false, err
	}

	draft, planUUIDs, docUUIDs, err := s.publicationRepo.GetDraft(artifactUUID, portal.ID, orgUUID)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get publication draft: %w", err)
	}
	if draft == nil {
		// Defensive only — see doc comment above.
		return nil, false, apperror.APIPublicationDraftNotFound.New()
	}
	if err := s.resolveHandles(draft, planUUIDs, docUUIDs, orgUUID); err != nil {
		return nil, false, err
	}

	definition, err := s.publicationRepo.GetContent(draft.UUID, model.PublicationContentTypeDefinition, orgUUID)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get publication draft definition: %w", err)
	}

	if err := s.portalPublisher.Publish(ctx, portal, apiId, draft, definition); err != nil {
		return nil, false, portalPushError(err)
	}

	published, wasReplace, err := s.publicationRepo.PromoteDraftToPublication(artifactUUID, portal.ID, orgUUID, actor, draft.UpdatedAt)
	if err != nil {
		return nil, false, fmt.Errorf("failed to promote publication draft: %w", err)
	}
	if published == nil {
		// The draft existed moments ago (checked above) but is gone now —
		// same defensive DRAFT_NOT_FOUND, not a normal outcome.
		return nil, false, apperror.APIPublicationDraftNotFound.New()
	}

	published.APIPortalHandle = portal.Handle
	published.APIPortalName = portal.Name
	// The plan/doc selections themselves didn't change during promotion —
	// only which row they're attached to may have (a republish reparents
	// them onto the anchor) — so the already-resolved handles still apply.
	published.SubscriptionPlanIds = draft.SubscriptionPlanIds
	published.DocIds = draft.DocIds
	return published, wasReplace, nil
}

// Unpublish removes the live listing from portal, then writes locally.
// Valid only when currently published or deprecated (409
// PUBLICATION_STATE_CONFLICT otherwise). On success: if no draft
// exists, the live row (the anchor) is demoted into the draft in place;
// otherwise an existing draft's content is merged into the anchor instead of
// leaving the draft's own row as the survivor — the anchor's uuid is the
// durable identity for this pairing and doesn't change either way. Either way
// a draft survives.
func (s *PublicationService) Unpublish(ctx context.Context, apiType, apiId, apiPortalId, orgUUID, actor string) error {
	artifactUUID, err := s.resolveArtifact(apiType, apiId, orgUUID)
	if err != nil {
		return err
	}
	portal, err := s.resolvePortalRow(apiPortalId, orgUUID)
	if err != nil {
		return err
	}

	live, _, _, err := s.publicationRepo.GetPublication(artifactUUID, portal.ID, orgUUID)
	if err != nil {
		return fmt.Errorf("failed to get publication: %w", err)
	}
	if live == nil || (live.Status != model.PublicationStatusPublished && live.Status != model.PublicationStatusDeprecated) {
		return apperror.APIPublicationStateConflict.New("unpublished")
	}

	if err := s.portalPublisher.Unpublish(ctx, portal, apiId); err != nil {
		return portalPushError(err)
	}

	found, err := s.publicationRepo.UnpublishPublication(artifactUUID, portal.ID, orgUUID, actor)
	if err != nil {
		return fmt.Errorf("failed to unpublish publication: %w", err)
	}
	if !found {
		// The live row existed moments ago (checked above) but is gone now —
		// same defensive precondition failure, not a normal outcome.
		return apperror.APIPublicationStateConflict.New("unpublished")
	}
	return nil
}

// Deprecate marks the live listing as deprecated on the portal, then locally. It
// requires the listing to be PUBLISHED (409 PUBLICATION_STATE_CONFLICT otherwise).
// The draft is neither read nor modified.
func (s *PublicationService) Deprecate(ctx context.Context, apiType, apiId, apiPortalId, orgUUID, actor string) (*model.Publication, error) {
	artifactUUID, err := s.resolveArtifact(apiType, apiId, orgUUID)
	if err != nil {
		return nil, err
	}
	portal, err := s.resolvePortalRow(apiPortalId, orgUUID)
	if err != nil {
		return nil, err
	}

	live, planUUIDs, docUUIDs, err := s.publicationRepo.GetPublication(artifactUUID, portal.ID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get publication: %w", err)
	}
	if live == nil || live.Status != model.PublicationStatusPublished {
		return nil, apperror.APIPublicationStateConflict.New("deprecated")
	}
	if err := s.resolveHandles(live, planUUIDs, docUUIDs, orgUUID); err != nil {
		return nil, err
	}

	if err := s.portalPublisher.Deprecate(ctx, portal, apiId, live); err != nil {
		return nil, portalPushError(err)
	}

	found, err := s.publicationRepo.DeprecatePublication(artifactUUID, portal.ID, orgUUID, actor)
	if err != nil {
		return nil, fmt.Errorf("failed to deprecate publication: %w", err)
	}
	if !found {
		// The row changed since the check above (for example, a concurrent unpublish).
		return nil, apperror.APIPublicationStateConflict.New("deprecated")
	}
	return s.getPublicationRow(apiType, apiId, apiPortalId, orgUUID)
}

// portalPushError maps a failed portal call to 409 when the portal rejected the
// request, and to 503 otherwise.
func portalPushError(err error) error {
	var conflict *PortalConflictError
	if errors.As(err, &conflict) {
		return apperror.APIPublicationPortalConflict.New(conflictReasonOrDefault(conflict))
	}
	return apperror.APIPublicationPortalUnavailable.Wrap(err)
}

// publicationStatusNotPublished is the rollup's own label for "no live row
// exists" — never a value api_publications itself stores.
const publicationStatusNotPublished = "NOT_PUBLISHED"

// ListPublicationSummary returns the GET /api-publications rollup: every
// active API Portal for the org, annotated with this API's publication
// status against it. search is matched case-insensitively against the
// portal's handle and display name; sortBy is "name" (portal display name)
// or anything else, including "createdAt" and unrecognized values, which
// falls back to the portal's own registration time — the same
// unrecognized-falls-back-to-default convention parseListOptions documents
// for every other collection GET. The returned slice is already
// filtered/sorted in full; the caller windows it for pagination.
func (s *PublicationService) ListPublicationSummary(apiType, apiId, orgUUID, sortBy, sortOrder, search string) ([]*model.PublicationSummary, error) {
	artifactUUID, err := s.resolveArtifact(apiType, apiId, orgUUID)
	if err != nil {
		return nil, err
	}

	portals, err := s.apiPortalRepo.ListActiveByOrg(orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to list active API Portals: %w", err)
	}
	statusRows, err := s.publicationRepo.ListStatusByArtifact(artifactUUID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to list publication status: %w", err)
	}

	draftUpdatedAt := make(map[string]time.Time, len(statusRows))
	liveStatus := make(map[string]string, len(statusRows))
	liveUpdatedAt := make(map[string]time.Time, len(statusRows))
	for _, row := range statusRows {
		if row.IsDraft {
			draftUpdatedAt[row.APIPortalUUID] = row.UpdatedAt
		} else {
			liveStatus[row.APIPortalUUID] = row.Status
			liveUpdatedAt[row.APIPortalUUID] = row.UpdatedAt
		}
	}

	search = strings.ToLower(strings.TrimSpace(search))
	summaries := make([]*model.PublicationSummary, 0, len(portals))
	for _, portal := range portals {
		if search != "" &&
			!strings.Contains(strings.ToLower(portal.Name), search) &&
			!strings.Contains(strings.ToLower(portal.Handle), search) {
			continue
		}

		status := publicationStatusNotPublished
		if st, ok := liveStatus[portal.ID]; ok {
			status = st
		}
		summary := &model.PublicationSummary{
			APIPortalHandle:      portal.Handle,
			APIPortalName:        portal.Name,
			APIPortalDescription: portal.Description,
			APIPortalURL:         portal.URL,
			Status:               status,
			APIPortalCreatedAt:   portal.CreatedAt,
		}
		if t, ok := draftUpdatedAt[portal.ID]; ok {
			summary.DraftUpdatedAt = &t
		}
		if t, ok := liveUpdatedAt[portal.ID]; ok {
			summary.PublicationUpdatedAt = &t
		}
		summaries = append(summaries, summary)
	}

	sortPublicationSummaries(summaries, sortBy, sortOrder)
	return summaries, nil
}

// sortPublicationSummariesLess reports whether a sorts strictly before b,
// ascending, for the given sortBy field.
func sortPublicationSummariesLess(a, b *model.PublicationSummary, sortBy string) bool {
	if sortBy == "name" {
		return strings.ToLower(a.APIPortalName) < strings.ToLower(b.APIPortalName)
	}
	return a.APIPortalCreatedAt.Before(b.APIPortalCreatedAt)
}

// sortPublicationSummaries sorts items in place by sortBy/sortOrder.
func sortPublicationSummaries(items []*model.PublicationSummary, sortBy, sortOrder string) {
	sort.SliceStable(items, func(i, j int) bool {
		if sortOrder == "asc" {
			return sortPublicationSummariesLess(items[i], items[j], sortBy)
		}
		return sortPublicationSummariesLess(items[j], items[i], sortBy)
	})
}
