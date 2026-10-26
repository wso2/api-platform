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
	"unicode/utf8"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// Maximum lengths of the draft fields the editor sends, matching the
// api_publications column widths. A longer value is rejected up front: the
// PostgreSQL and SQL Server columns would otherwise fail the insert with an
// unhandled error, while SQLite would store it silently.
const (
	publicationDisplayNameMaxLength = 255
	publicationVersionMaxLength     = 30
	publicationDescriptionMaxLength = 1023
	publicationURLMaxLength         = 255
	publicationOwnerMaxLength       = 255
)

// definitionFileNamesByContentType maps an accepted definition Content-Type to
// the canonical file name it's stored under, matching the API Portal's own
// constants. No other media type is accepted.
var definitionFileNamesByContentType = map[string]string{
	"application/json":    "definition.json",
	"application/yaml":    "definition.yaml",
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

	portalOperationTimeout time.Duration
	// archiveMaxBytes bounds the content ZIP pushed to the portal.
	archiveMaxBytes int64
}

// NewPublicationService creates a new API Publication service.
func NewPublicationService(
	artifactRepo repository.ArtifactRepository,
	apiPortalRepo repository.APIPortalRepository,
	documentRepo repository.DocumentRepository,
	subscriptionPlanRepo repository.SubscriptionPlanRepository,
	publicationRepo repository.PublicationRepository,
	portalPublisher PortalPublisher,
	portalOperationTimeout time.Duration,
	archiveMaxBytes int64,
	slogger *slog.Logger,
) *PublicationService {
	if slogger == nil {
		slogger = slog.Default()
	}
	if archiveMaxBytes <= 0 {
		archiveMaxBytes = defaultPublicationArchiveMaxBytes
	}
	return &PublicationService{
		artifactRepo:         artifactRepo,
		apiPortalRepo:        apiPortalRepo,
		documentRepo:         documentRepo,
		subscriptionPlanRepo: subscriptionPlanRepo,
		publicationRepo:      publicationRepo,
		portalPublisher:      portalPublisher,
		slogger:              slogger,

		portalOperationTimeout: portalOperationTimeout,
		archiveMaxBytes:        archiveMaxBytes,
	}
}

// portalOperationContext applies the one deadline shared by all portal calls of a request.
func (s *PublicationService) portalOperationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, s.portalOperationTimeout)
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

	pub, plans, docs, err := s.publicationRepo.GetDraft(artifactUUID, portalUUID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get publication draft: %w", err)
	}
	if pub == nil {
		return nil, apperror.APIPublicationDraftNotFound.New()
	}
	applySelections(pub, plans, docs)
	return pub, nil
}

// applySelections fills a publication's plan and document selections from its
// mapping rows. The id lists include selections whose source was deleted, so
// the live listing keeps sending them to the portal; Subscription Plans and
// Documents say which ones are deleted.
func applySelections(pub *model.Publication, plans, docs []model.PublicationSelection) {
	pub.SubscriptionPlans = nonNilSelections(plans)
	pub.Documents = nonNilSelections(docs)
	pub.SubscriptionPlanIds = selectionHandles(plans)
	pub.DocIds = selectionHandles(docs)
}

// selectionHandles returns the handles of selections in order. It is never nil,
// so it serializes as "[]" rather than "null" when empty.
func selectionHandles(selections []model.PublicationSelection) []string {
	handles := make([]string, 0, len(selections))
	for _, sel := range selections {
		handles = append(handles, sel.Handle)
	}
	return handles
}

func nonNilSelections(selections []model.PublicationSelection) []model.PublicationSelection {
	if selections == nil {
		return []model.PublicationSelection{}
	}
	return selections
}

// withoutDeleted returns the selections whose source still exists.
func withoutDeleted(selections []model.PublicationSelection) []model.PublicationSelection {
	kept := make([]model.PublicationSelection, 0, len(selections))
	for _, sel := range selections {
		if !sel.Deleted {
			kept = append(kept, sel)
		}
	}
	return kept
}

// deletedHandles returns the handles of selections whose source was deleted.
func deletedHandles(selections []model.PublicationSelection) []string {
	var handles []string
	for _, sel := range selections {
		if sel.Deleted {
			handles = append(handles, sel.Handle)
		}
	}
	return handles
}

// conflictReasonOrDefault returns conflict.Reason, or defaultPortalConflictReason
// when a PortalPublisher left it unset, so the conflict message is never blank.
func conflictReasonOrDefault(conflict *PortalConflictError) string {
	if conflict.Reason == "" {
		return defaultPortalConflictReason
	}
	return conflict.Reason
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
	if err := validateDraftFieldLengths(draft); err != nil {
		return nil, err
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

	plans, err := s.resolvePlanSelections(planHandles, orgUUID)
	if err != nil {
		return nil, err
	}
	docs, err := s.resolveDocSelections(artifactUUID, docHandles, orgUUID)
	if err != nil {
		return nil, err
	}

	draft.OrganizationUUID = orgUUID
	draft.ArtifactUUID = artifactUUID
	draft.APIPortalUUID = portalUUID
	draft.IsDraft = true

	saved, err := s.publicationRepo.SaveDraftDetails(draft, plans, docs, actor)
	if err != nil {
		return nil, fmt.Errorf("failed to save publication draft: %w", err)
	}
	// Echo back exactly what was validated and stored, rather than a second
	// round trip through the mapping tables.
	applySelections(saved, plans, docs)
	return saved, nil
}

// validateDraftFieldLengths counts characters rather than bytes, since the
// columns are VARCHAR(n).
func validateDraftFieldLengths(draft *model.Publication) error {
	limits := []struct {
		field string
		value string
		max   int
	}{
		{"displayName", draft.DisplayName, publicationDisplayNameMaxLength},
		{"version", draft.Version, publicationVersionMaxLength},
		{"description", draft.Description, publicationDescriptionMaxLength},
		{"productionUrl", draft.ProductionURL, publicationURLMaxLength},
		{"sandboxUrl", draft.SandboxURL, publicationURLMaxLength},
		{"businessOwner", draft.BusinessOwner, publicationOwnerMaxLength},
		{"businessOwnerEmail", draft.BusinessOwnerEmail, publicationOwnerMaxLength},
		{"technicalOwner", draft.TechnicalOwner, publicationOwnerMaxLength},
		{"technicalOwnerEmail", draft.TechnicalOwnerEmail, publicationOwnerMaxLength},
	}
	for _, l := range limits {
		if utf8.RuneCountInString(l.value) > l.max {
			return apperror.APIPublicationValidationFailed.New(fmt.Sprintf("%s must not exceed %d characters", l.field, l.max))
		}
	}
	return nil
}

// nonNil returns s, or a non-nil empty slice when s is nil, so the field
// serializes as "[]" rather than "null".
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// loadActivePlans loads the plans for handles, keyed by handle, rejecting any handle
// absent from the organization's catalog and any plan that is not ACTIVE.
func (s *PublicationService) loadActivePlans(handles []string, orgUUID string) (map[string]*model.SubscriptionPlan, error) {
	plans, err := s.subscriptionPlanRepo.GetByHandles(handles, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve subscription plan handles: %w", err)
	}
	var unknown, inactive []string
	for _, h := range handles {
		plan, ok := plans[h]
		switch {
		case !ok:
			unknown = append(unknown, h)
		case plan.Status != model.SubscriptionPlanStatusActive:
			inactive = append(inactive, h)
		}
	}
	if len(unknown) > 0 {
		return nil, apperror.APIPublicationValidationFailed.New(
			"subscriptionPlanIds not found in the organization's catalog: " + strings.Join(unknown, ", "))
	}
	if len(inactive) > 0 {
		return nil, apperror.APIPublicationValidationFailed.New(
			"subscriptionPlanIds that are not active: " + strings.Join(inactive, ", "))
	}
	return plans, nil
}

// resolvePlanSelections resolves each subscription plan handle to a selection
// carrying the plan's UUID, handle and name, rejecting any handle that is
// unknown or not ACTIVE.
func (s *PublicationService) resolvePlanSelections(handles []string, orgUUID string) ([]model.PublicationSelection, error) {
	if len(handles) == 0 {
		return nil, nil
	}
	plans, err := s.loadActivePlans(handles, orgUUID)
	if err != nil {
		return nil, err
	}
	return planSelections(handles, plans), nil
}

func planSelections(handles []string, plans map[string]*model.SubscriptionPlan) []model.PublicationSelection {
	selections := make([]model.PublicationSelection, 0, len(handles))
	for _, h := range handles {
		selections = append(selections, model.PublicationSelection{UUID: plans[h].UUID, Handle: h, DisplayName: plans[h].Name})
	}
	return selections
}

// GetDraftDefinition returns the draft's stored definition content.
func (s *PublicationService) GetDraftDefinition(apiType, apiId, apiPortalId, orgUUID string) (*model.PublicationContent, error) {
	return s.getDraftContent(apiType, apiId, apiPortalId, orgUUID, model.PublicationContentTypeDefinition)
}

// SaveDraftDefinition replaces the draft's definition. contentTypeHeader must
// be one of the four accepted serializations; it selects both the stored
// Content-Type and the canonical file name recorded alongside it.
//
// A draft is a work in progress: its definition is not required to be a
// complete, valid document, or even one that matches apiType's own rules —
// only Publish enforces that, at the one point it actually matters (a
// listing going live). Saving progress must never be blocked on that.
func (s *PublicationService) SaveDraftDefinition(apiType, apiId, apiPortalId, orgUUID, actor, contentTypeHeader string, data []byte) error {
	fileName, ok := definitionFileNamesByContentType[contentTypeHeader]
	if !ok {
		return apperror.APIPublicationValidationFailed.New(
			"Content-Type must be one of application/json, application/yaml, application/graphql, application/xml")
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

	pub, plans, docs, err := s.publicationRepo.GetPublication(artifactUUID, portal.ID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get publication: %w", err)
	}
	if pub == nil {
		return nil, apperror.APIPublicationNotFound.New()
	}
	applySelections(pub, plans, docs)
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

// Publish publishes the current draft to the API Portal. The client saves the draft just
// before calling, so APIPublicationDraftNotFound is a fail-closed check, not a normal gate.
// This is the one place a definition must exist and be valid.
//
// The portal is pushed first (listing, then content), after the content is loaded and packed
// outside the portal deadline; nothing local changes unless the push succeeds. Then
// PublicationRepository.PromoteDraftToPublication flips the draft row in place on a first
// publish, or merges it into the live row on a republish, so the live row's uuid never changes.
//
// A draft saved during the push is not promoted (APIPublicationDraftChanged). A push is never
// undone, so a failed content push after the listing was accepted leaves the portal ahead of
// local state until the next publish.
func (s *PublicationService) Publish(ctx context.Context, apiType, apiId, apiPortalId, orgUUID, actor string) (*model.Publication, bool, error) {
	artifactUUID, err := s.resolveArtifact(apiType, apiId, orgUUID)
	if err != nil {
		return nil, false, err
	}
	portal, err := s.resolvePortalRow(apiPortalId, orgUUID)
	if err != nil {
		return nil, false, err
	}

	draft, draftPlans, draftDocs, err := s.publicationRepo.GetDraft(artifactUUID, portal.ID, orgUUID)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get publication draft: %w", err)
	}
	if draft == nil {
		// Defensive only — see doc comment above.
		return nil, false, apperror.APIPublicationDraftNotFound.New()
	}
	// A deleted document is dropped from the listing; a deleted plan is refused below.
	draftDocs = withoutDeleted(draftDocs)
	applySelections(draft, draftPlans, draftDocs)

	definition, err := s.publicationRepo.GetContent(draft.UUID, model.PublicationContentTypeDefinition, orgUUID)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get publication draft definition: %w", err)
	}
	if err := validateDefinitionContent(apiType, definition); err != nil {
		return nil, false, err
	}

	// Taken before the plans and documents are read, so an edit that lands while
	// publishing still reads as edited afterwards.
	snapshotAt := time.Now().UTC()
	if deleted := deletedHandles(draftPlans); len(deleted) > 0 {
		return nil, false, apperror.APIPublicationValidationFailed.New(
			"subscriptionPlanIds that were deleted: " + strings.Join(deleted, ", "))
	}
	plans, err := s.activePlansInOrder(draft.SubscriptionPlanIds, orgUUID)
	if err != nil {
		return nil, false, err
	}
	pushedDocs, err := s.resolveDocSelections(artifactUUID, draft.DocIds, orgUUID)
	if err != nil {
		return nil, false, err
	}
	thumbnail, err := s.publicationRepo.GetContent(draft.UUID, model.PublicationContentTypeImage, orgUUID)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get publication draft thumbnail: %w", err)
	}
	landingPage, err := s.publicationRepo.GetContent(draft.UUID, model.PublicationContentTypeMarketing, orgUUID)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get publication draft landing page: %w", err)
	}
	content, err := s.loadPortalContent(portalContentSource{
		ArtifactUUID: artifactUUID,
		OrgUUID:      orgUUID,
		DocHandles:   draft.DocIds,
		Thumbnail:    thumbnail,
		LandingPage:  landingPage,
	})
	if err != nil {
		return nil, false, err
	}

	ctx, cancel := s.portalOperationContext(ctx)
	defer cancel()

	if err := s.createMissingPortalPlans(ctx, portal, plans); err != nil {
		return nil, false, err
	}

	if err := s.portalPublisher.Publish(ctx, portal, apiType, apiId, draft, definition); err != nil {
		return nil, false, portalPushError(err)
	}
	if content != nil {
		if err := s.portalPublisher.PublishContent(ctx, portal, apiId, content); err != nil {
			return nil, false, portalPushError(err)
		}
	}

	pushedPlans := make([]model.PublicationSelection, 0, len(plans))
	for _, plan := range plans {
		pushedPlans = append(pushedPlans, model.PublicationSelection{UUID: plan.UUID, Handle: plan.Handle, DisplayName: plan.Name})
	}
	published, wasReplace, err := s.publicationRepo.PromoteDraftToPublication(artifactUUID, portal.ID, orgUUID, actor, draft.UpdatedAt, snapshotAt, pushedPlans, pushedDocs)
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
	applySelections(published, pushedPlans, pushedDocs)
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
	ctx, cancel := s.portalOperationContext(ctx)
	defer cancel()

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
		return apperror.APIPublicationStateConflict.New("This API is already unpublished from this API Portal. No changes were made.")
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
		return apperror.APIPublicationStateConflict.New("This API was already unpublished by another request. No changes were made.")
	}
	return nil
}

// Deprecate marks the live listing as deprecated on the portal, then locally. It
// requires the listing to be PUBLISHED (409 PUBLICATION_STATE_CONFLICT otherwise).
// The draft is neither read nor modified.
func (s *PublicationService) Deprecate(ctx context.Context, apiType, apiId, apiPortalId, orgUUID, actor string) (*model.Publication, error) {
	ctx, cancel := s.portalOperationContext(ctx)
	defer cancel()

	artifactUUID, err := s.resolveArtifact(apiType, apiId, orgUUID)
	if err != nil {
		return nil, err
	}
	portal, err := s.resolvePortalRow(apiPortalId, orgUUID)
	if err != nil {
		return nil, err
	}

	live, livePlans, liveDocs, err := s.publicationRepo.GetPublication(artifactUUID, portal.ID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get publication: %w", err)
	}
	if live == nil {
		return nil, apperror.APIPublicationStateConflict.New("This API is not published on this API Portal, so it cannot be deprecated.")
	}
	if live.Status != model.PublicationStatusPublished {
		return nil, apperror.APIPublicationStateConflict.New("This API is already deprecated on this API Portal. No changes were made.")
	}
	applySelections(live, livePlans, liveDocs)

	if err := s.portalPublisher.Deprecate(ctx, portal, apiType, apiId, live); err != nil {
		return nil, portalPushError(err)
	}

	found, err := s.publicationRepo.DeprecatePublication(artifactUUID, portal.ID, orgUUID, actor)
	if err != nil {
		return nil, fmt.Errorf("failed to deprecate publication: %w", err)
	}
	if !found {
		// The row changed since the check above (for example, a concurrent unpublish).
		return nil, apperror.APIPublicationStateConflict.New("The API status changed during the request. No changes were made.")
	}
	return s.getPublicationRow(apiType, apiId, apiPortalId, orgUUID)
}

// activePlansInOrder loads the plans for handles, in handle order, rejecting any that
// is unknown or not active.
func (s *PublicationService) activePlansInOrder(handles []string, orgUUID string) ([]*model.SubscriptionPlan, error) {
	if len(handles) == 0 {
		return nil, nil
	}
	byHandle, err := s.loadActivePlans(handles, orgUUID)
	if err != nil {
		return nil, err
	}
	plans := make([]*model.SubscriptionPlan, 0, len(handles))
	for _, h := range handles {
		plans = append(plans, byHandle[h])
	}
	return plans, nil
}

// createMissingPortalPlans makes sure the portal has every plan the draft selects,
// creating those it lacks. Failures come back already mapped for the caller.
func (s *PublicationService) createMissingPortalPlans(ctx context.Context, portal *model.APIPortal, plans []*model.SubscriptionPlan) error {
	if len(plans) == 0 {
		return nil
	}
	if err := s.portalPublisher.CreateMissingPlans(ctx, portal, plans); err != nil {
		return portalPushError(err)
	}
	return nil
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
