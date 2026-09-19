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

package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/router"
	"github.com/wso2/api-platform/platform-api/internal/service"

	"github.com/wso2/api-platform/httpkit/httputil"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

// bodyReadError maps a failed request-body read to 413 when the size cap was
// exceeded, and otherwise to the given validation error.
func bodyReadError(err error, fallback error) error {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return apperror.PayloadTooLarge.New("request body exceeds the maximum allowed size")
	}
	return fallback
}

// decodeJSONBody decodes exactly one JSON value from the request body into dst, capped at
// maxBytes. Trailing non-whitespace data is rejected, and it is read through the size cap so an
// oversized body still yields 413 rather than being silently ignored.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, maxBytes int64, dst any) error {
	invalid := apperror.APIPublicationValidationFailed.New("Request body is not valid JSON")
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes))
	if err := dec.Decode(dst); err != nil {
		return bodyReadError(err, invalid)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return bodyReadError(err, invalid)
	}
	return nil
}

// restAPITypeValue is the type-agnostic path value for RestApi. The
// publish/unpublish/deprecate routes are pinned to this one literal per
// type — unlike the shared read/draft routes, apiType is not a path
// variable here.
const restAPITypeValue = "rest-api"

// defaultPublicationContentMaxBytes/defaultPublicationThumbnailMaxBytes apply
// when the corresponding config field is <= 0 — same zero-means-default
// convention as OpenAPISpecMaxFetchBytes/MCPResponseMaxBytes (config/config.go).
const (
	defaultPublicationContentMaxBytes   int64 = 10 << 20 // 10 MiB — a spec document or markdown landing page
	defaultPublicationThumbnailMaxBytes int64 = 2 << 20  // 2 MiB — a small icon, not a spec document
)

// PublicationHandler serves the API Publication draft endpoints.
type PublicationHandler struct {
	service           *service.PublicationService
	identity          *service.IdentityService
	contentMaxBytes   int64
	thumbnailMaxBytes int64
	slogger           *slog.Logger
}

// NewPublicationHandler creates a new API Publication handler.
// contentMaxBytes/thumbnailMaxBytes <= 0 fall back to the package defaults above.
func NewPublicationHandler(publicationService *service.PublicationService, identity *service.IdentityService, contentMaxBytes, thumbnailMaxBytes int64, slogger *slog.Logger) *PublicationHandler {
	if contentMaxBytes <= 0 {
		contentMaxBytes = defaultPublicationContentMaxBytes
	}
	if thumbnailMaxBytes <= 0 {
		thumbnailMaxBytes = defaultPublicationThumbnailMaxBytes
	}
	return &PublicationHandler{
		service:           publicationService,
		identity:          identity,
		contentMaxBytes:   contentMaxBytes,
		thumbnailMaxBytes: thumbnailMaxBytes,
		slogger:           slogger,
	}
}

// RegisterRoutes registers the API Publication draft routes.
func (h *PublicationHandler) RegisterRoutes(mux router.Router) {
	base := constants.APIBasePath + "/api-portals/{apiPortalId}/apis/{apiType}/{apiId}"
	mux.HandleFunc("GET "+base+"/draft", middleware.MapErrors(h.slogger, h.GetDraft))
	mux.HandleFunc("PUT "+base+"/draft", middleware.MapErrors(h.slogger, h.SaveDraft))
	mux.HandleFunc("GET "+base+"/draft/definition", middleware.MapErrors(h.slogger, h.GetDraftDefinition))
	mux.HandleFunc("PUT "+base+"/draft/definition", middleware.MapErrors(h.slogger, h.SaveDraftDefinition))
	mux.HandleFunc("GET "+base+"/draft/landing-page", middleware.MapErrors(h.slogger, h.GetDraftLandingPage))
	mux.HandleFunc("PUT "+base+"/draft/landing-page", middleware.MapErrors(h.slogger, h.SaveDraftLandingPage))
	mux.HandleFunc("GET "+base+"/draft/thumbnail", middleware.MapErrors(h.slogger, h.GetDraftThumbnail))
	mux.HandleFunc("PUT "+base+"/draft/thumbnail", middleware.MapErrors(h.slogger, h.SaveDraftThumbnail))
	mux.HandleFunc("GET "+base+"/publication", middleware.MapErrors(h.slogger, h.GetPublication))
	mux.HandleFunc("GET "+base+"/publication/definition", middleware.MapErrors(h.slogger, h.GetPublicationDefinition))
	mux.HandleFunc("GET "+base+"/publication/landing-page", middleware.MapErrors(h.slogger, h.GetPublicationLandingPage))
	mux.HandleFunc("GET "+base+"/publication/thumbnail", middleware.MapErrors(h.slogger, h.GetPublicationThumbnail))
	mux.HandleFunc("GET "+constants.APIBasePath+"/api-publications", middleware.MapErrors(h.slogger, h.ListPublications))
	mux.HandleFunc("POST "+constants.APIBasePath+"/api-portals/{apiPortalId}/apis/rest-api/{apiId}/publish", middleware.MapErrors(h.slogger, h.Publish))
	mux.HandleFunc("POST "+constants.APIBasePath+"/api-portals/{apiPortalId}/apis/rest-api/{apiId}/unpublish", middleware.MapErrors(h.slogger, h.Unpublish))
	mux.HandleFunc("POST "+constants.APIBasePath+"/api-portals/{apiPortalId}/apis/rest-api/{apiId}/deprecate", middleware.MapErrors(h.slogger, h.Deprecate))
}

// draftPathParams extracts the three identity segments every route under
// base carries, in the order the service methods take them.
func draftPathParams(r *http.Request) (apiType, apiId, apiPortalId string) {
	return r.PathValue("apiType"), r.PathValue("apiId"), r.PathValue("apiPortalId")
}

// GetDraft handles GET .../draft
func (h *PublicationHandler) GetDraft(w http.ResponseWriter, r *http.Request) error {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}
	apiType, apiId, apiPortalId := draftPathParams(r)

	pub, err := h.service.GetDraft(apiType, apiId, apiPortalId, orgId)
	if err != nil {
		return serviceError(err, "failed to get publication draft")
	}

	httputil.WriteJSON(w, http.StatusOK, draftModelToResponse(pub))
	return nil
}

// SaveDraft handles PUT .../draft
func (h *PublicationHandler) SaveDraft(w http.ResponseWriter, r *http.Request) error {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}
	apiType, apiId, apiPortalId := draftPathParams(r)

	actor, err := resolveActorErr(r, h.identity, "save publication draft")
	if err != nil {
		return err
	}

	var in api.PublicationDraftDetailsInput
	if err := decodeJSONBody(w, r, h.contentMaxBytes, &in); err != nil {
		return err
	}

	draft, planHandles, docHandles := draftInputToModel(&in)
	saved, err := h.service.SaveDraftDetails(apiType, apiId, apiPortalId, orgId, actor, draft, planHandles, docHandles)
	if err != nil {
		return serviceError(err, "failed to save publication draft")
	}

	httputil.WriteJSON(w, http.StatusOK, draftModelToResponse(saved))
	return nil
}

// GetDraftDefinition handles GET .../draft/definition
func (h *PublicationHandler) GetDraftDefinition(w http.ResponseWriter, r *http.Request) error {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}
	apiType, apiId, apiPortalId := draftPathParams(r)

	content, err := h.service.GetDraftDefinition(apiType, apiId, apiPortalId, orgId)
	if err != nil {
		return serviceError(err, "failed to get publication draft definition")
	}
	writeContent(w, content)
	return nil
}

// SaveDraftDefinition handles PUT .../draft/definition
func (h *PublicationHandler) SaveDraftDefinition(w http.ResponseWriter, r *http.Request) error {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}
	apiType, apiId, apiPortalId := draftPathParams(r)

	actor, err := resolveActorErr(r, h.identity, "save publication draft definition")
	if err != nil {
		return err
	}

	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType == "" {
		return apperror.APIPublicationValidationFailed.New(
			"Content-Type must be one of application/json, application/x-yaml, application/graphql, application/xml")
	}

	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, h.contentMaxBytes))
	if err != nil {
		return bodyReadError(err, apperror.APIPublicationValidationFailed.New("definition upload could not be read"))
	}

	if err := h.service.SaveDraftDefinition(apiType, apiId, apiPortalId, orgId, actor, contentType, data); err != nil {
		return serviceError(err, "failed to save publication draft definition")
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// GetDraftLandingPage handles GET .../draft/landing-page
func (h *PublicationHandler) GetDraftLandingPage(w http.ResponseWriter, r *http.Request) error {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}
	apiType, apiId, apiPortalId := draftPathParams(r)

	content, err := h.service.GetDraftLandingPage(apiType, apiId, apiPortalId, orgId)
	if err != nil {
		return serviceError(err, "failed to get publication draft landing page")
	}
	writeContent(w, content)
	return nil
}

// SaveDraftLandingPage handles PUT .../draft/landing-page
func (h *PublicationHandler) SaveDraftLandingPage(w http.ResponseWriter, r *http.Request) error {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}
	apiType, apiId, apiPortalId := draftPathParams(r)

	actor, err := resolveActorErr(r, h.identity, "save publication draft landing page")
	if err != nil {
		return err
	}

	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, h.contentMaxBytes))
	if err != nil {
		return bodyReadError(err, apperror.APIPublicationValidationFailed.New("landing page upload could not be read"))
	}

	if err := h.service.SaveDraftLandingPage(apiType, apiId, apiPortalId, orgId, actor, data); err != nil {
		return serviceError(err, "failed to save publication draft landing page")
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// GetDraftThumbnail handles GET .../draft/thumbnail
func (h *PublicationHandler) GetDraftThumbnail(w http.ResponseWriter, r *http.Request) error {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}
	apiType, apiId, apiPortalId := draftPathParams(r)

	content, err := h.service.GetDraftThumbnail(apiType, apiId, apiPortalId, orgId)
	if err != nil {
		return serviceError(err, "failed to get publication draft thumbnail")
	}
	writeContent(w, content)
	return nil
}

// SaveDraftThumbnail handles PUT .../draft/thumbnail. The one multipart
// endpoint in this feature — per file-access.md, processed fully in memory,
// content-sniffed rather than trusted from the declared type or file name.
func (h *PublicationHandler) SaveDraftThumbnail(w http.ResponseWriter, r *http.Request) error {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}
	apiType, apiId, apiPortalId := draftPathParams(r)

	actor, err := resolveActorErr(r, h.identity, "save publication draft thumbnail")
	if err != nil {
		return err
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.thumbnailMaxBytes)
	if err := r.ParseMultipartForm(h.thumbnailMaxBytes); err != nil {
		return bodyReadError(err, apperror.APIPublicationValidationFailed.New("thumbnail upload is not a valid multipart form"))
	}
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		return apperror.APIPublicationValidationFailed.New("file is required")
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, h.thumbnailMaxBytes+1))
	if err != nil {
		return apperror.APIPublicationValidationFailed.New("thumbnail upload could not be read")
	}
	if int64(len(data)) > h.thumbnailMaxBytes {
		return apperror.PayloadTooLarge.New("request body exceeds the maximum allowed size")
	}

	// Filename only in storage (file-access.md): strip any directory component
	// from the uploader's declared name before it ever reaches the service/DB.
	fileName := ""
	if fileHeader != nil {
		fileName = sanitizeUploadFileName(fileHeader.Filename)
	}

	if err := h.service.SaveDraftThumbnail(apiType, apiId, apiPortalId, orgId, actor, fileName, data); err != nil {
		return serviceError(err, "failed to save publication draft thumbnail")
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// sanitizeUploadFileName reduces an uploader-declared name to a bare file name,
// returning "" for names that are not usable (dot-only, separator-only, or
// containing a NUL byte).
func sanitizeUploadFileName(name string) string {
	if strings.ContainsRune(name, 0) {
		return ""
	}
	base := filepath.Base(name)
	if base == "." || base == ".." || base == string(filepath.Separator) {
		return ""
	}
	return base
}

// GetPublication handles GET .../publication
func (h *PublicationHandler) GetPublication(w http.ResponseWriter, r *http.Request) error {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}
	apiType, apiId, apiPortalId := draftPathParams(r)

	pub, err := h.service.GetPublication(apiType, apiId, apiPortalId, orgId)
	if err != nil {
		return serviceError(err, "failed to get publication")
	}

	httputil.WriteJSON(w, http.StatusOK, publicationModelToResponse(pub))
	return nil
}

// GetPublicationDefinition handles GET .../publication/definition
func (h *PublicationHandler) GetPublicationDefinition(w http.ResponseWriter, r *http.Request) error {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}
	apiType, apiId, apiPortalId := draftPathParams(r)

	content, err := h.service.GetPublicationDefinition(apiType, apiId, apiPortalId, orgId)
	if err != nil {
		return serviceError(err, "failed to get publication definition")
	}
	writeContent(w, content)
	return nil
}

// GetPublicationLandingPage handles GET .../publication/landing-page
func (h *PublicationHandler) GetPublicationLandingPage(w http.ResponseWriter, r *http.Request) error {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}
	apiType, apiId, apiPortalId := draftPathParams(r)

	content, err := h.service.GetPublicationLandingPage(apiType, apiId, apiPortalId, orgId)
	if err != nil {
		return serviceError(err, "failed to get publication landing page")
	}
	writeContent(w, content)
	return nil
}

// GetPublicationThumbnail handles GET .../publication/thumbnail
func (h *PublicationHandler) GetPublicationThumbnail(w http.ResponseWriter, r *http.Request) error {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}
	apiType, apiId, apiPortalId := draftPathParams(r)

	content, err := h.service.GetPublicationThumbnail(apiType, apiId, apiPortalId, orgId)
	if err != nil {
		return serviceError(err, "failed to get publication thumbnail")
	}
	writeContent(w, content)
	return nil
}

// ListPublications handles GET /api-publications?apiType=&apiId=&... — the
// cross-portal rollup: every active API Portal for the org, annotated with
// this API's publication status against it.
func (h *PublicationHandler) ListPublications(w http.ResponseWriter, r *http.Request) error {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}

	apiType := r.URL.Query().Get("apiType")
	apiId := r.URL.Query().Get("apiId")
	if apiType == "" || apiId == "" {
		return apperror.ValidationFailed.New("apiType and apiId query parameters are required")
	}
	opts := parseListOptions(r)

	summaries, err := h.service.ListPublicationSummary(apiType, apiId, orgId, opts.SortBy, opts.SortOrder, opts.Search)
	if err != nil {
		return serviceError(err, "failed to list API publications")
	}

	resp := api.PublicationSummaryResponse{
		List: publicationSummariesToResponse(pageWindow(summaries, opts.Limit, opts.Offset)),
		Pagination: api.Pagination{
			Total:  len(summaries),
			Offset: opts.Offset,
			Limit:  opts.Limit,
		},
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// Publish handles POST .../rest-api/{apiId}/publish. Bodyless: the client
// always saves the draft (PUT) immediately before calling this action, so
// DRAFT_NOT_FOUND here is a defensive check, not a normal user-facing gate.
func (h *PublicationHandler) Publish(w http.ResponseWriter, r *http.Request) error {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}
	apiPortalId, apiId := r.PathValue("apiPortalId"), r.PathValue("apiId")

	actor, err := resolveActorErr(r, h.identity, "publish API")
	if err != nil {
		return err
	}

	pub, replaced, err := h.service.Publish(r.Context(), restAPITypeValue, apiId, apiPortalId, orgId, actor)
	if err != nil {
		return serviceError(err, "failed to publish API")
	}

	if replaced {
		httputil.WriteJSON(w, http.StatusOK, publicationModelToResponse(pub))
		return nil
	}
	w.Header().Set("Location", constants.APIBasePath+"/api-portals/"+apiPortalId+"/apis/"+restAPITypeValue+"/"+apiId+"/publication")
	httputil.WriteJSON(w, http.StatusCreated, publicationModelToResponse(pub))
	return nil
}

// Unpublish handles POST .../rest-api/{apiId}/unpublish. Valid only when
// currently published or deprecated.
func (h *PublicationHandler) Unpublish(w http.ResponseWriter, r *http.Request) error {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}
	apiPortalId, apiId := r.PathValue("apiPortalId"), r.PathValue("apiId")

	actor, err := resolveActorErr(r, h.identity, "unpublish API")
	if err != nil {
		return err
	}

	if err := h.service.Unpublish(r.Context(), restAPITypeValue, apiId, apiPortalId, orgId, actor); err != nil {
		return serviceError(err, "failed to unpublish API")
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// Deprecate handles POST .../rest-api/{apiId}/deprecate.
func (h *PublicationHandler) Deprecate(w http.ResponseWriter, r *http.Request) error {
	orgId, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}
	apiPortalId, apiId := r.PathValue("apiPortalId"), r.PathValue("apiId")

	actor, err := resolveActorErr(r, h.identity, "deprecate API")
	if err != nil {
		return err
	}

	pub, err := h.service.Deprecate(r.Context(), restAPITypeValue, apiId, apiPortalId, orgId, actor)
	if err != nil {
		return serviceError(err, "failed to deprecate API")
	}

	httputil.WriteJSON(w, http.StatusOK, publicationModelToResponse(pub))
	return nil
}

// writeContent writes a stored definition/landing-page/thumbnail as its raw
// bytes with its stored Content-Type — matching how the portal itself serves
// this same content.
func writeContent(w http.ResponseWriter, content *model.PublicationContent) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if content.ContentType != "" {
		w.Header().Set("Content-Type", content.ContentType)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content.Content)
}

// draftInputToModel converts the generated request body into the internal
// model, plus the raw subscriptionPlanIds/docIds handle lists (resolved to
// UUIDs by the service, not here).
func draftInputToModel(in *api.PublicationDraftDetailsInput) (*model.Publication, []string, []string) {
	pub := &model.Publication{
		DisplayName: in.DisplayName,
		Version:     in.Version,
	}
	if in.Description != nil {
		pub.Description = *in.Description
	}
	if in.AgentVisibility != nil {
		pub.AgentVisibility = string(*in.AgentVisibility)
	}
	if in.Tags != nil {
		pub.Tags = *in.Tags
	}
	if in.Labels != nil {
		pub.Labels = *in.Labels
	}
	if in.Endpoints != nil {
		if in.Endpoints.ProductionUrl != nil {
			pub.ProductionURL = *in.Endpoints.ProductionUrl
		}
		if in.Endpoints.SandboxUrl != nil {
			pub.SandboxURL = *in.Endpoints.SandboxUrl
		}
	}
	if in.Owners != nil {
		if in.Owners.BusinessOwner != nil {
			pub.BusinessOwner = *in.Owners.BusinessOwner
		}
		if in.Owners.BusinessOwnerEmail != nil {
			pub.BusinessOwnerEmail = string(*in.Owners.BusinessOwnerEmail)
		}
		if in.Owners.TechnicalOwner != nil {
			pub.TechnicalOwner = *in.Owners.TechnicalOwner
		}
		if in.Owners.TechnicalOwnerEmail != nil {
			pub.TechnicalOwnerEmail = string(*in.Owners.TechnicalOwnerEmail)
		}
	}

	var planHandles, docHandles []string
	if in.SubscriptionPlanIds != nil {
		planHandles = *in.SubscriptionPlanIds
	}
	if in.DocIds != nil {
		docHandles = *in.DocIds
	}
	return pub, planHandles, docHandles
}

// draftModelToResponse converts the internal model into the generated
// response shape.
func draftModelToResponse(pub *model.Publication) api.PublicationDraftDetails {
	resp := api.PublicationDraftDetails{
		DisplayName:    pub.DisplayName,
		Version:        pub.Version,
		HasThumbnail:   &pub.HasThumbnail,
		HasLandingPage: &pub.HasLandingPage,
	}
	if pub.Description != "" {
		resp.Description = &pub.Description
	}
	if pub.AgentVisibility != "" {
		av := api.PublicationDraftDetailsAgentVisibility(pub.AgentVisibility)
		resp.AgentVisibility = &av
	}
	if len(pub.Tags) > 0 {
		resp.Tags = &pub.Tags
	}
	if len(pub.Labels) > 0 {
		resp.Labels = &pub.Labels
	}
	if pub.ProductionURL != "" || pub.SandboxURL != "" {
		resp.Endpoints = &struct {
			ProductionUrl *string `json:"productionUrl,omitempty" yaml:"productionUrl,omitempty"`
			SandboxUrl    *string `json:"sandboxUrl,omitempty" yaml:"sandboxUrl,omitempty"`
		}{}
		if pub.ProductionURL != "" {
			resp.Endpoints.ProductionUrl = &pub.ProductionURL
		}
		if pub.SandboxURL != "" {
			resp.Endpoints.SandboxUrl = &pub.SandboxURL
		}
	}
	if pub.BusinessOwner != "" || pub.BusinessOwnerEmail != "" || pub.TechnicalOwner != "" || pub.TechnicalOwnerEmail != "" {
		resp.Owners = &struct {
			BusinessOwner       *string              `json:"businessOwner,omitempty" yaml:"businessOwner,omitempty"`
			BusinessOwnerEmail  *openapi_types.Email `json:"businessOwnerEmail,omitempty" yaml:"businessOwnerEmail,omitempty"`
			TechnicalOwner      *string              `json:"technicalOwner,omitempty" yaml:"technicalOwner,omitempty"`
			TechnicalOwnerEmail *openapi_types.Email `json:"technicalOwnerEmail,omitempty" yaml:"technicalOwnerEmail,omitempty"`
		}{}
		if pub.BusinessOwner != "" {
			resp.Owners.BusinessOwner = &pub.BusinessOwner
		}
		if pub.BusinessOwnerEmail != "" {
			email := openapi_types.Email(pub.BusinessOwnerEmail)
			resp.Owners.BusinessOwnerEmail = &email
		}
		if pub.TechnicalOwner != "" {
			resp.Owners.TechnicalOwner = &pub.TechnicalOwner
		}
		if pub.TechnicalOwnerEmail != "" {
			email := openapi_types.Email(pub.TechnicalOwnerEmail)
			resp.Owners.TechnicalOwnerEmail = &email
		}
	}

	planIds := api.SubscriptionPlanIdList(pub.SubscriptionPlanIds)
	resp.SubscriptionPlanIds = &planIds
	docIds := api.DocIdList(pub.DocIds)
	resp.DocIds = &docIds

	if !pub.CreatedAt.IsZero() {
		resp.CreatedAt = &pub.CreatedAt
	}
	if pub.CreatedBy != "" {
		resp.CreatedBy = &pub.CreatedBy
	}
	if !pub.UpdatedAt.IsZero() {
		resp.UpdatedAt = &pub.UpdatedAt
	}
	if pub.UpdatedBy != "" {
		resp.UpdatedBy = &pub.UpdatedBy
	}
	return resp
}

// publicationSummariesToResponse converts the internal rollup rows into the
// generated PublicationSummaryItem list.
func publicationSummariesToResponse(items []*model.PublicationSummary) []api.PublicationSummaryItem {
	out := make([]api.PublicationSummaryItem, 0, len(items))
	for _, item := range items {
		status := api.PublicationSummaryItemStatus(item.Status)
		resp := api.PublicationSummaryItem{
			ApiPortalId:          &item.APIPortalHandle,
			ApiPortalName:        &item.APIPortalName,
			DraftUpdatedAt:       item.DraftUpdatedAt,
			PublicationUpdatedAt: item.PublicationUpdatedAt,
			Status:               &status,
		}
		if item.APIPortalDescription != "" {
			resp.ApiPortalDescription = &item.APIPortalDescription
		}
		if item.APIPortalURL != "" {
			resp.ApiPortalUrl = &item.APIPortalURL
		}
		out = append(out, resp)
	}
	return out
}

// publicationModelToResponse converts the internal model into the generated
// live-publication response shape — the same fields as draftModelToResponse
// plus apiPortalId/apiPortalName/status, which only ever apply to a live
// listing (the Publication schema, not PublicationDraftDetails).
func publicationModelToResponse(pub *model.Publication) api.Publication {
	resp := api.Publication{
		DisplayName:    &pub.DisplayName,
		Version:        &pub.Version,
		HasThumbnail:   &pub.HasThumbnail,
		HasLandingPage: &pub.HasLandingPage,
	}
	if pub.APIPortalHandle != "" {
		resp.ApiPortalId = &pub.APIPortalHandle
	}
	if pub.APIPortalName != "" {
		resp.ApiPortalName = &pub.APIPortalName
	}
	if pub.Status != "" {
		status := api.PublicationStatus(pub.Status)
		resp.Status = &status
	}
	if pub.Description != "" {
		resp.Description = &pub.Description
	}
	if pub.AgentVisibility != "" {
		av := api.PublicationAgentVisibility(pub.AgentVisibility)
		resp.AgentVisibility = &av
	}
	if len(pub.Tags) > 0 {
		resp.Tags = &pub.Tags
	}
	if len(pub.Labels) > 0 {
		resp.Labels = &pub.Labels
	}
	if pub.ProductionURL != "" || pub.SandboxURL != "" {
		resp.Endpoints = &struct {
			ProductionUrl *string `json:"productionUrl,omitempty" yaml:"productionUrl,omitempty"`
			SandboxUrl    *string `json:"sandboxUrl,omitempty" yaml:"sandboxUrl,omitempty"`
		}{}
		if pub.ProductionURL != "" {
			resp.Endpoints.ProductionUrl = &pub.ProductionURL
		}
		if pub.SandboxURL != "" {
			resp.Endpoints.SandboxUrl = &pub.SandboxURL
		}
	}
	if pub.BusinessOwner != "" || pub.BusinessOwnerEmail != "" || pub.TechnicalOwner != "" || pub.TechnicalOwnerEmail != "" {
		resp.Owners = &struct {
			BusinessOwner       *string              `json:"businessOwner,omitempty" yaml:"businessOwner,omitempty"`
			BusinessOwnerEmail  *openapi_types.Email `json:"businessOwnerEmail,omitempty" yaml:"businessOwnerEmail,omitempty"`
			TechnicalOwner      *string              `json:"technicalOwner,omitempty" yaml:"technicalOwner,omitempty"`
			TechnicalOwnerEmail *openapi_types.Email `json:"technicalOwnerEmail,omitempty" yaml:"technicalOwnerEmail,omitempty"`
		}{}
		if pub.BusinessOwner != "" {
			resp.Owners.BusinessOwner = &pub.BusinessOwner
		}
		if pub.BusinessOwnerEmail != "" {
			email := openapi_types.Email(pub.BusinessOwnerEmail)
			resp.Owners.BusinessOwnerEmail = &email
		}
		if pub.TechnicalOwner != "" {
			resp.Owners.TechnicalOwner = &pub.TechnicalOwner
		}
		if pub.TechnicalOwnerEmail != "" {
			email := openapi_types.Email(pub.TechnicalOwnerEmail)
			resp.Owners.TechnicalOwnerEmail = &email
		}
	}

	planIds := api.SubscriptionPlanIdList(pub.SubscriptionPlanIds)
	resp.SubscriptionPlanIds = &planIds
	docIds := api.DocIdList(pub.DocIds)
	resp.DocIds = &docIds

	if !pub.CreatedAt.IsZero() {
		resp.CreatedAt = &pub.CreatedAt
	}
	if pub.CreatedBy != "" {
		resp.CreatedBy = &pub.CreatedBy
	}
	if !pub.UpdatedAt.IsZero() {
		resp.UpdatedAt = &pub.UpdatedAt
	}
	if pub.UpdatedBy != "" {
		resp.UpdatedBy = &pub.UpdatedBy
	}
	return resp
}
