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
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/wso2/api-platform/httpkit/httputil"
	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/model"
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

// RegisterRoutes registers the five docs operations under a single
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
	docType := strings.TrimSpace(r.URL.Query().Get("type"))
	if docType != "" {
		docType = strings.ToUpper(docType)
	}
	limit, offset := parsePagination(r)

	docs, total, err := h.service.GetAllApiDocuments(artifactUUID, orgID, docType, limit, offset)
	if err != nil {
		return serviceError(err, "failed to list documents")
	}

	items := make([]api.APIDocumentMetadata, 0, len(docs))
	for _, d := range docs {
		items = append(items, documentToAPIMetadata(d))
	}
	resp := api.APIDocumentListResponse{
		Count: len(items),
		List:  items,
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
// Returns metadata + the UTF-8 content string in a single JSON response.
// Content is a string because today the endpoint only accepts markdown;
// extending to binary formats later requires either base64-encoding this
// field or splitting content into its own subroute.
func (h *APIDocumentHandler) GetDocument(w http.ResponseWriter, r *http.Request) error {
	orgID, artifactUUID, err := h.resolveArtifactUUID(r)
	if err != nil {
		return err
	}
	docID := r.PathValue("docId")
	if docID == "" {
		return apperror.ValidationFailed.New("document ID is required")
	}

	doc, err := h.service.GetDocument(artifactUUID, docID, orgID, "")
	if err != nil {
		return serviceError(err, "failed to get document")
	}

	httputil.WriteJSON(w, http.StatusOK, documentToAPIDocument(doc))
	return nil
}

// CreateDocument handles POST /apis/{apiType}/{apiId}/docs. Expects a
// multipart/form-data body with: type (required), displayName (required),
// handle (optional), and exactly one of file / inlineContent for the body.
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

	req := &dto.CreateAPIDocumentRequest{
		Type:        strings.ToUpper(parsed.docType),
		Handle:      parsed.handle,
		DisplayName: parsed.displayName,
		FileName:    parsed.fileName,
		Content:     parsed.content,
	}

	handle, err := h.service.CreateApiDocument(req, orgID, createdBy, artifactUUID)
	if err != nil {
		return serviceError(err, "failed to create document")
	}

	doc, err := h.service.GetDocument(artifactUUID, handle, orgID, "")
	if err != nil {
		return serviceError(err, "failed to load created document")
	}
	w.Header().Set("Location", r.URL.Path+"/"+handle)
	httputil.WriteJSON(w, http.StatusCreated, documentToAPIMetadata(doc))
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

	req := &dto.UpdateAPIDocumentRequest{}
	if parsed.docTypeSet {
		upper := strings.ToUpper(parsed.docType)
		req.Type = &upper
	}
	if parsed.displayNameSet {
		req.DisplayName = &parsed.displayName
	}
	if parsed.content != nil {
		req.Content = parsed.content
		if parsed.contentTypeSet {
			req.ContentType = &parsed.contentType
		}
		// A new upload may bring a new filename too; reflect it when set.
		if parsed.fileNameSet {
			req.FileName = &parsed.fileName
		} else if parsed.fileName != "" {
			req.FileName = &parsed.fileName
		}
	} else if parsed.fileNameSet {
		// Explicit metadata-only filename rename.
		req.FileName = &parsed.fileName
	}

	if err := h.service.UpdateApiDocument(req, orgID, updatedBy, artifactUUID, docID); err != nil {
		return serviceError(err, "failed to update document")
	}

	doc, err := h.service.GetDocument(artifactUUID, docID, orgID, "")
	if err != nil {
		return serviceError(err, "failed to load updated document")
	}
	httputil.WriteJSON(w, http.StatusOK, documentToAPIMetadata(doc))
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
	handle         string
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
		if vals, ok := form.Value["handle"]; ok && len(vals) > 0 {
			parsed.handle = strings.TrimSpace(vals[0])
		}
		if vals, ok := form.Value["displayName"]; ok {
			parsed.displayNameSet = true
			if len(vals) > 0 {
				parsed.displayName = strings.TrimSpace(vals[0])
			}
		}
	}

	// `file` wins over `inlineContent` when both are supplied — but we reject
	// outright rather than silently pick, matching the openapi import pattern.
	file, header, fileErr := r.FormFile("file")
	hasFile := fileErr == nil
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

	if hasFile && hasInline {
		file.Close()
		return parsedDocForm{}, apperror.ValidationFailed.New("provide either `file` or `inlineContent`, not both")
	}
	if requireContent && !hasFile && !hasInline {
		return parsedDocForm{}, apperror.ValidationFailed.New("one of `file` or `inlineContent` is required")
	}

	if hasFile {
		defer file.Close()
		data, readErr := io.ReadAll(io.LimitReader(file, h.maxBodyBytes+1))
		if readErr != nil {
			return parsedDocForm{}, apperror.ValidationFailed.New("failed to read uploaded file")
		}
		if int64(len(data)) > h.maxBodyBytes {
			return parsedDocForm{}, apperror.PayloadTooLarge.New("file exceeds maximum allowed size")
		}
		parsed.content = data
		parsed.fileName = sanitizeUploadFileName(header.Filename)
		parsed.fileNameSet = parsed.fileName != ""
		// Leave parsed.contentType unset — the service's DetectContentType
		// does the sniff from bytes + filename, in one place.
	} else if hasInline {
		parsed.content = []byte(inlineContent)
		// Inline content has no uploaded filename; the caller may supply one
		// alongside inlineContent to keep an existing filename on PUT.
		if form != nil {
			if vals, ok := form.Value["fileName"]; ok {
				parsed.fileNameSet = true
				if len(vals) > 0 {
					parsed.fileName = sanitizeUploadFileName(strings.TrimSpace(vals[0]))
				}
			}
		}
		// Inline content is markdown by convention. The explicit default
		// survives even when no filename hint is supplied, so a plain
		// inlineContent create still gets stored as markdown.
		parsed.contentType = "text/markdown; charset=utf-8"
		parsed.contentTypeSet = true
	}

	return parsed, nil
}

// documentToAPIMetadata strips the content BLOB and reshapes a model.Document
// into the generated api.APIDocumentMetadata response type. Empty optional
// fields on the model become nil pointers so JSON output omits them, matching
// the OpenAPI contract's `omitempty` optionality.
//
// Note: api.APIDocumentMetadata.Id in the spec is the document handle, not
// the DB UUID — the handle is what callers use in /docs/{docId}.
func documentToAPIMetadata(d *model.Document) api.APIDocumentMetadata {
	return api.APIDocumentMetadata{
		Id:          d.Handle,
		Type:        api.APIDocumentType(d.Type),
		DisplayName: d.DisplayName,
		FileName:    optionalString(d.FileName),
		ContentType: optionalString(d.ContentType),
		CreatedBy:   optionalString(d.CreatedBy),
		CreatedAt:   optionalTime(d.CreatedAt),
		UpdatedBy:   optionalString(d.UpdatedBy),
		UpdatedAt:   optionalTime(d.UpdatedAt),
	}
}

// documentToAPIDocument is documentToAPIMetadata plus the UTF-8 content
// payload — the response shape for the single-doc GET.
func documentToAPIDocument(d *model.Document) api.APIDocument {
	return api.APIDocument{
		Id:          d.Handle,
		Type:        api.APIDocumentType(d.Type),
		DisplayName: d.DisplayName,
		FileName:    optionalString(d.FileName),
		ContentType: optionalString(d.ContentType),
		CreatedBy:   optionalString(d.CreatedBy),
		CreatedAt:   optionalTime(d.CreatedAt),
		UpdatedBy:   optionalString(d.UpdatedBy),
		UpdatedAt:   optionalTime(d.UpdatedAt),
		Content:     string(d.Content),
	}
}

// optionalString returns nil for an empty string so JSON serialisation
// honours `omitempty` on *string fields in the generated api types.
func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// optionalTime returns nil for a zero time.Time so JSON serialisation honours
// `omitempty` on *time.Time fields in the generated api types.
func optionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
