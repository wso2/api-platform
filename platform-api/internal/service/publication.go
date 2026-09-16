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
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// definitionFileNamesByContentType maps an accepted definition Content-Type to
// the canonical file name it's stored under, matching the API Portal's own
// constants (DB_design_refined.md). No other media type is accepted.
var definitionFileNamesByContentType = map[string]string{
	"application/json":    "definition.json",
	"application/x-yaml":  "definition.yaml",
	"application/graphql": "definition.graphql",
	"application/xml":     "definition.xml",
}

// PublicationService implements the API Publication draft (Slice 1) and, in
// later slices, the live publication/rollup/lifecycle operations.
type PublicationService struct {
	artifactRepo         repository.ArtifactRepository
	apiPortalRepo        repository.ApiPortalRepository
	apiDocumentRepo      repository.ApiDocumentRepository
	subscriptionPlanRepo repository.SubscriptionPlanRepository
	publicationRepo      repository.PublicationRepository
	slogger              *slog.Logger
}

// NewPublicationService creates a new API Publication service.
func NewPublicationService(
	artifactRepo repository.ArtifactRepository,
	apiPortalRepo repository.ApiPortalRepository,
	apiDocumentRepo repository.ApiDocumentRepository,
	subscriptionPlanRepo repository.SubscriptionPlanRepository,
	publicationRepo repository.PublicationRepository,
	slogger *slog.Logger,
) *PublicationService {
	if slogger == nil {
		slogger = slog.Default()
	}
	return &PublicationService{
		artifactRepo:         artifactRepo,
		apiPortalRepo:        apiPortalRepo,
		apiDocumentRepo:      apiDocumentRepo,
		subscriptionPlanRepo: subscriptionPlanRepo,
		publicationRepo:      publicationRepo,
		slogger:              slogger,
	}
}

// resolveArtifact resolves (apiType, apiId) to the artifact's internal UUID,
// per REST_Design.md §4/§11: an unrecognised apiType and an unknown apiId
// both resolve to the same 404 (API_NOT_FOUND) — not a distinguishable 400,
// which would let a caller enumerate installed plugins.
func (s *PublicationService) resolveArtifact(apiType, apiId, orgUUID string) (string, error) {
	if apiType == "" || apiId == "" {
		return "", apperror.APIPublicationAPINotFound.New()
	}
	metadata, err := s.artifactRepo.GetAPIMetadataByHandleAndKind(apiId, apiType, orgUUID)
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
	if apiPortalId == "" {
		return "", apperror.APIPublicationAPIPortalNotFound.New()
	}
	portal, err := s.apiPortalRepo.GetByHandleAndOrg(apiPortalId, orgUUID)
	if err != nil {
		return "", fmt.Errorf("failed to resolve API Portal by handle: %w", err)
	}
	if portal == nil {
		return "", apperror.APIPublicationAPIPortalNotFound.New()
	}
	return portal.UUID, nil
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

	docHandles, err := s.apiDocumentRepo.GetHandlesByUUIDs(docUUIDs, orgUUID)
	if err != nil {
		return fmt.Errorf("failed to resolve document handles: %w", err)
	}
	pub.DocIds = mapValuesInOrder(docUUIDs, docHandles)
	return nil
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
		return nil, apperror.ValidationFailed.New("displayName is required")
	}
	if strings.TrimSpace(draft.Version) == "" {
		return nil, apperror.ValidationFailed.New("version is required")
	}
	switch draft.AgentVisibility {
	case "":
		draft.AgentVisibility = "VISIBLE"
	case "VISIBLE", "HIDDEN":
	default:
		return nil, apperror.ValidationFailed.New("agentVisibility must be VISIBLE or HIDDEN")
	}

	planHandles = dedupe(planHandles)
	docHandles = dedupe(docHandles)

	planUUIDs, err := s.resolvePlanUUIDs(planHandles, orgUUID)
	if err != nil {
		return nil, err
	}
	docUUIDs, err := s.resolveDocUUIDs(docHandles, orgUUID)
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
			fmt.Sprintf("subscriptionPlanIds not found in the organization's catalog: %s", strings.Join(unresolved, ", ")))
	}
	return uuids, nil
}

// resolveDocUUIDs resolves each document handle to its doc_uuid, rejecting
// any handle absent from the organization's api_documents.
func (s *PublicationService) resolveDocUUIDs(handles []string, orgUUID string) ([]string, error) {
	if len(handles) == 0 {
		return nil, nil
	}
	resolved, err := s.apiDocumentRepo.GetUUIDsByHandles(handles, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve document handles: %w", err)
	}
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
			fmt.Sprintf("docIds not found in the organization's documents: %s", strings.Join(unresolved, ", ")))
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
// is stripped before storage (REST_Design.md §9) — stricter than the portal's
// own handling, and defense-in-depth on top of it, not a substitute.
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
// the declared Content-Type or file name. GIF/WebP/SVG are rejected: per
// REST_Design.md §9, the API Portal's own image-serving code resolves a
// correct Content-Type only for PNG/JPEG, and SVG is XML that can carry
// scripts.
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
