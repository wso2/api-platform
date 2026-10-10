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
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/wso2/api-platform/httpkit/httputil"
	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/router"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

// APIDocumentHandler serves the /apis/{apiType}/{apiId}/docs endpoints:
// create, list, read, update, delete user-authored API documents attached to any
// artifact kind. The OpenAPI DEFINITION document is managed via
// /rest-apis/{restApiId}/openapi, not here, and the service layer refuses to
// expose or mutate it through this surface.
type APIDocumentHandler struct {
	service      *service.APIDocumentService
	identity     *service.IdentityService
	cfg          *config.Server
	slogger      *slog.Logger
	maxBodyBytes int64
}

// NewAPIDocumentHandler constructs an APIDocumentHandler. maxBodyBytes
// defaults to OpenAPISpecMaxFetchBytes (same ceiling used by the existing
// spec upload path) when zero or unset.
func NewAPIDocumentHandler(apiDocumentService *service.APIDocumentService, identity *service.IdentityService, slogger *slog.Logger, cfg *config.Server) *APIDocumentHandler {
	maxBytes := constants.DefaultOpenAPISpecMaxBytes
	if cfg != nil && cfg.OpenAPISpecMaxFetchBytes > 0 {
		maxBytes = cfg.OpenAPISpecMaxFetchBytes
	}
	return &APIDocumentHandler{
		service:      apiDocumentService,
		identity:     identity,
		cfg:          cfg,
		slogger:      slogger,
		maxBodyBytes: maxBytes,
	}
}

// RegisterRoutes registers the six docs operations under a single
// {apiType} path parameter. Scope requirements are declared under the one
// matching OpenAPI path and apply to every artifact kind resolved through
// {apiType}; the artifact repository rejects an unknown kind as 404 so a
// caller cannot reach a kind the deployment did not register.
func (h *APIDocumentHandler) RegisterRoutes(mux router.Router) {
	h.slogger.Debug("Registering API document routes")
	base := constants.APIBasePath + "/apis/{apiType}/{apiId}/docs"
	mux.HandleFunc("GET "+base, middleware.MapErrors(h.slogger, h.ListDocuments))
	mux.HandleFunc("POST "+base, middleware.MapErrors(h.slogger, h.CreateDocument))
	mux.HandleFunc("GET "+base+"/{docId}", middleware.MapErrors(h.slogger, h.GetDocument))
	mux.HandleFunc("GET "+base+"/{docId}/content", middleware.MapErrors(h.slogger, h.GetDocumentContent))
	mux.HandleFunc("PUT "+base+"/{docId}", middleware.MapErrors(h.slogger, h.UpdateDocument))
	mux.HandleFunc("DELETE "+base+"/{docId}", middleware.MapErrors(h.slogger, h.DeleteDocument))
}

// fetch the org from the token, read apiId from the path, resolve the typed artifact to its UUID
func (h *APIDocumentHandler) resolveArtifactUUID(r *http.Request) (orgID, artifactUUID string, err error) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return "", "", apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}
	apiID := r.PathValue("apiId")
	if apiID == "" {
		return "", "", apperror.ValidationFailed.New("API ID is required")
	}
	apiType := r.PathValue("apiType")
	if apiType == "" {
		return "", "", apperror.ValidationFailed.New("API type is required")
	}
	artifactUUID, err = h.service.ResolveArtifactUUID(apiType, apiID, orgID)
	if err != nil {
		return "", "", serviceError(err, "failed to resolve API "+apiID+" of type "+apiType)
	}
	return orgID, artifactUUID, nil
}

// ListDocuments handles GET /apis/{apiType}/{apiId}/docs.
// Optional query param `type` filters to a single document type.
func (h *APIDocumentHandler) ListDocuments(w http.ResponseWriter, r *http.Request) error {
	orgID, artifactUUID, err := h.resolveArtifactUUID(r)
	if err != nil {
		return err
	}
	rawDocType := strings.TrimSpace(r.URL.Query().Get("type"))
	var docType string
	if rawDocType != "" {
		if normalized, ok := service.NormalizeAPIDocumentType(rawDocType); ok {
			docType = normalized
		} else if constants.ForbiddenOtherTypeNames[strings.ToLower(rawDocType)] {
			return apperror.ValidationFailed.New("invalid document type filter")
		} else {
			// Custom type name — pass through as-is; the repository will query
			// type = 'DOC_<rawDocType>' which matches how custom types are stored.
			docType = rawDocType
		}
	}
	limit, offset := parsePagination(r)

	docs, total, err := h.service.GetAllApiDocuments(artifactUUID, orgID, docType, limit, offset)
	if err != nil {
		return serviceError(err, "failed to list documents")
	}

	resp := api.APIDocumentListResponse{
		Count: len(docs),
		List:  docs,
		Pagination: api.Pagination{
			Total:  total,
			Offset: offset,
			Limit:  limit,
		},
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

// GetDocument handles GET /apis/{apiType}/{apiId}/docs/{docId}.
// Returns document metadata only
func (h *APIDocumentHandler) GetDocument(w http.ResponseWriter, r *http.Request) error {
	orgID, artifactUUID, err := h.resolveArtifactUUID(r)
	if err != nil {
		return err
	}
	docID := r.PathValue("docId")
	if docID == "" {
		return apperror.ValidationFailed.New("document ID is required")
	}
	if constants.ReservedAPIDocumentHandles[docID] {
		return apperror.ValidationFailed.New("cannot access a system-managed document via this endpoint")
	}

	doc, err := h.service.GetDocument(artifactUUID, docID, orgID)
	if err != nil {
		return serviceError(err, "failed to get document")
	}

	httputil.WriteJSON(w, http.StatusOK, doc)
	return nil
}

// GetDocumentContent handles GET /apis/{apiType}/{apiId}/docs/{docId}/content.
func (h *APIDocumentHandler) GetDocumentContent(w http.ResponseWriter, r *http.Request) error {
	orgID, artifactUUID, err := h.resolveArtifactUUID(r)
	if err != nil {
		return err
	}
	docID := r.PathValue("docId")
	if docID == "" {
		return apperror.ValidationFailed.New("document ID is required")
	}
	if constants.ReservedAPIDocumentHandles[docID] {
		return apperror.ValidationFailed.New("cannot access a system-managed document via this endpoint")
	}

	doc, content, err := h.service.GetDocumentWithContent(artifactUUID, docID, orgID, "")
	if err != nil {
		return serviceError(err, "failed to get document content")
	}

	if len(content) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}

	ct := "application/octet-stream"
	if doc.ContentType != nil && *doc.ContentType != "" {
		ct = *doc.ContentType
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if doc.FileName != nil && *doc.FileName != "" {
		fn := strings.NewReplacer(`"`, `\"`, `\`, `\\`).Replace(*doc.FileName)
		w.Header().Set("Content-Disposition", `inline; filename="`+fn+`"`)
	}
	_, _ = w.Write(content)
	return nil
}

// CreateDocument handles POST /apis/{apiType}/{apiId}/docs. Expects a
// multipart/form-data body with: type (required), displayName (required),
// id (optional handle), and exactly one of file / inlineContent for the body.
func (h *APIDocumentHandler) CreateDocument(w http.ResponseWriter, r *http.Request) error {
	orgID, artifactUUID, err := h.resolveArtifactUUID(r)
	if err != nil {
		return err
	}
	createdBy, err := resolveActorErr(r, h.identity, "create document")
	if err != nil {
		return err
	}

	parsed, err := h.parseDocMultipart(w, r, true)
	if err != nil {
		return err
	}

	// converts the user-supplied type to the canonical camelCase form, if valid type
	normalizedType, typeOK := service.NormalizeAPIDocumentType(parsed.docType)
	if !typeOK {
		return apperror.ValidationFailed.New("invalid document type")
	}
	req := &dto.CreateAPIDocumentRequest{
		Type:          normalizedType,
		Handle:        parsed.handle,
		DisplayName:   parsed.displayName,
		FileName:      parsed.fileName,
		Content:       parsed.content,
		OtherTypeName: parsed.otherTypeName,
	}

	handle, err := h.service.CreateApiDocument(req, orgID, createdBy, artifactUUID)
	if err != nil {
		return serviceError(err, "failed to create document")
	}

	doc, err := h.service.GetDocument(artifactUUID, handle, orgID)
	if err != nil {
		return serviceError(err, "failed to load created document")
	}
	w.Header().Set("Location", r.URL.Path+"/"+url.PathEscape(handle))
	httputil.WriteJSON(w, http.StatusCreated, doc)
	return nil
}

// UpdateDocument handles PUT /apis/{apiType}/{apiId}/docs/{docId}. Every
// multipart field is optional — the service merges the supplied subset onto
// the stored row, leaving unmentioned fields alone. Omitting both `file` and
// `inlineContent` means a metadata-only update; the stored bytes are
// untouched.
func (h *APIDocumentHandler) UpdateDocument(w http.ResponseWriter, r *http.Request) error {
	orgID, artifactUUID, err := h.resolveArtifactUUID(r)
	if err != nil {
		return err
	}
	docID := r.PathValue("docId")
	if docID == "" {
		return apperror.ValidationFailed.New("document ID is required")
	}
	updatedBy, err := resolveActorErr(r, h.identity, "update document")
	if err != nil {
		return err
	}

	parsed, err := h.parseDocMultipart(w, r, false)
	if err != nil {
		return err
	}

	if parsed.handleSet && parsed.handle != docID {
		return apperror.ValidationFailed.New("id in request body does not match the document ID in the path")
	}

	req := &dto.UpdateAPIDocumentRequest{}
	if parsed.docTypeSet {
		normalizedType, typeOK := service.NormalizeAPIDocumentType(parsed.docType)
		if !typeOK {
			return apperror.ValidationFailed.New("invalid document type")
		}
		req.Type = &normalizedType
		req.OtherTypeName = parsed.otherTypeName
	}
	if parsed.displayNameSet {
		req.DisplayName = &parsed.displayName
	}
	if parsed.content != nil {
		req.Content = parsed.content
		if parsed.contentTypeSet {
			req.ContentType = &parsed.contentType
		}
		if parsed.fileNameSet {
			req.FileName = &parsed.fileName
		}
	} else if parsed.fileNameSet {
		// Explicit metadata-only filename rename.
		req.FileName = &parsed.fileName
	}

	if err := h.service.UpdateApiDocument(req, orgID, updatedBy, artifactUUID, docID); err != nil {
		return serviceError(err, "failed to update document")
	}

	doc, err := h.service.GetDocument(artifactUUID, docID, orgID)
	if err != nil {
		return serviceError(err, "failed to load updated document")
	}
	httputil.WriteJSON(w, http.StatusOK, doc)
	return nil
}

// DeleteDocument handles DELETE /apis/{apiType}/{apiId}/docs/{docId}.
func (h *APIDocumentHandler) DeleteDocument(w http.ResponseWriter, r *http.Request) error {
	orgID, artifactUUID, err := h.resolveArtifactUUID(r)
	if err != nil {
		return err
	}
	docID := r.PathValue("docId")
	if docID == "" {
		return apperror.ValidationFailed.New("document ID is required")
	}
	deletedBy, err := resolveActorErr(r, h.identity, "delete document")
	if err != nil {
		return err
	}

	if err := h.service.DeleteApiDocument(artifactUUID, docID, orgID, deletedBy); err != nil {
		return serviceError(err, "failed to delete document")
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// parsedDocForm captures the shape of a parsed multipart doc request. The
// *Set flags separate "field was present (even if empty)" from "field was
// absent" so a PUT can leave a field alone vs explicitly blank it.
type parsedDocForm struct {
	docType        string
	docTypeSet     bool
	otherTypeName  string // only meaningful when docType == "Other"
	handle         string
	handleSet      bool
	displayName    string
	displayNameSet bool
	fileName       string
	fileNameSet    bool
	contentType    string
	contentTypeSet bool
	// content is nil when neither `file` nor `inlineContent` was supplied,
	// so a PUT can tell apart "no body change" from "replace with empty".
	content []byte
}

// parseDocMultipart parses the multipart form on r. requireContent enforces
// that exactly one of file / inlineContent was supplied (true on POST, false
// on PUT where either may be omitted for a metadata-only update).
func (h *APIDocumentHandler) parseDocMultipart(w http.ResponseWriter, r *http.Request, requireContent bool) (parsedDocForm, error) {
	const multipartOverhead = 1 << 20
	r.Body = http.MaxBytesReader(w, r.Body, h.maxBodyBytes+multipartOverhead)
	if err := r.ParseMultipartForm(h.maxBodyBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return parsedDocForm{}, apperror.PayloadTooLarge.New("request body exceeds the maximum allowed size")
		}
		return parsedDocForm{}, apperror.ValidationFailed.New("invalid multipart form")
	}

	var parsed parsedDocForm
	form := r.MultipartForm
	if form != nil {
		if vals, ok := form.Value["type"]; ok {
			parsed.docTypeSet = true
			if len(vals) > 0 {
				parsed.docType = strings.TrimSpace(vals[0])
			}
		}
		if vals, ok := form.Value["id"]; ok {
			parsed.handleSet = true
			if len(vals) > 0 {
				parsed.handle = strings.TrimSpace(vals[0])
			}
		}
		if vals, ok := form.Value["otherTypeName"]; ok && len(vals) > 0 {
			parsed.otherTypeName = strings.TrimSpace(vals[0])
		}
		if vals, ok := form.Value["displayName"]; ok {
			parsed.displayNameSet = true
			if len(vals) > 0 {
				parsed.displayName = strings.TrimSpace(vals[0])
			}
		}
	}

	var inlineContent string
	hasInline := false
	if form != nil {
		if vals, ok := form.Value["inlineContent"]; ok {
			hasInline = true
			if len(vals) > 0 {
				inlineContent = vals[0]
			}
		}
	}

	if requireContent && !hasInline {
		return parsedDocForm{}, apperror.ValidationFailed.New("`inlineContent` is required")
	}

	if hasInline {
		parsed.content = []byte(inlineContent)
		// The caller may supply an explicit fileName field alongside inlineContent
		// (e.g. to keep or rename the stored filename on PUT).
		if form != nil {
			if vals, ok := form.Value["fileName"]; ok {
				parsed.fileNameSet = true
				if len(vals) > 0 {
					parsed.fileName = sanitizeUploadFileName(strings.TrimSpace(vals[0]))
				}
			}
		}
		parsed.contentType = "text/markdown; charset=utf-8"
		parsed.contentTypeSet = true
	}

	return parsed, nil
}

