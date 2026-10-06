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

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/router"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

// APIThumbnailHandler serves the /apis/{apiType}/{apiId}/thumbnail endpoints.
// Thumbnail is a singleton image document per artifact — one PUT replaces
// whatever was there (no POST), GET streams the stored bytes, and DELETE
// removes it so the UI falls back to the artifact's name-initials avatar.
type APIThumbnailHandler struct {
	service      *service.APIDocumentService
	identity     *service.IdentityService
	slogger      *slog.Logger
	maxBodyBytes int64
}

func NewAPIThumbnailHandler(apiDocumentService *service.APIDocumentService, identity *service.IdentityService, slogger *slog.Logger, cfg *config.Server) *APIThumbnailHandler {
	maxBytes := constants.DefaultThumbnailMaxBytes
	if cfg != nil && cfg.ThumbnailMaxFetchBytes > 0 {
		maxBytes = cfg.ThumbnailMaxFetchBytes
	}
	return &APIThumbnailHandler{
		service:      apiDocumentService,
		identity:     identity,
		slogger:      slogger,
		maxBodyBytes: maxBytes,
	}
}

func (h *APIThumbnailHandler) RegisterRoutes(mux router.Router) {
	h.slogger.Debug("Registering API thumbnail routes")
	base := constants.APIBasePath + "/apis/{apiType}/{apiId}/thumbnail"
	mux.HandleFunc("GET "+base, middleware.MapErrors(h.slogger, h.GetThumbnail))
	mux.HandleFunc("PUT "+base, middleware.MapErrors(h.slogger, h.UpsertThumbnail))
	mux.HandleFunc("DELETE "+base, middleware.MapErrors(h.slogger, h.DeleteThumbnail))
}

func (h *APIThumbnailHandler) resolveArtifactUUID(r *http.Request) (orgID, artifactUUID string, err error) {
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

// GetThumbnail handles GET /apis/{apiType}/{apiId}/thumbnail. Streams the stored bytes
// with the sniffed Content-Type. 204 No Content when thumbnail is not set
func (h *APIThumbnailHandler) GetThumbnail(w http.ResponseWriter, r *http.Request) error {
	orgID, artifactUUID, err := h.resolveArtifactUUID(r)
	if err != nil {
		return err
	}

	doc, content, err := h.service.GetDocumentWithContent(artifactUUID, constants.DocumentHandleThumbnail,
		orgID, constants.DocumentTypeThumbnail)
	if err != nil {
		if apperror.NotFound.Is(err) {
			w.WriteHeader(http.StatusNoContent)
			return nil
		}
		return serviceError(err, "failed to fetch thumbnail")
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
	// Thumbnail bytes change in place at a fixed URL — a browser HTTP cache
	// would return stale bytes on the first refetch after a PUT.
	// no-store keeps the browser from writing a cache entry at all.
	w.Header().Set("Cache-Control", "no-store")
	if doc.FileName != nil && *doc.FileName != "" {
		fn := strings.NewReplacer(`"`, `\"`, `\`, `\\`).Replace(*doc.FileName)
		w.Header().Set("Content-Disposition", `inline; filename="`+fn+`"`)
	}
	_, _ = w.Write(content)
	return nil
}

// UpsertThumbnail handles PUT /apis/{apiType}/{apiId}/thumbnail.
// The server sniffs the uploaded bytes and rejects anything that isn't JPEG or PNG;
// the uploader's declared Content-Type and filename extension are not trusted.
func (h *APIThumbnailHandler) UpsertThumbnail(w http.ResponseWriter, r *http.Request) error {
	orgID, artifactUUID, err := h.resolveArtifactUUID(r)
	if err != nil {
		return err
	}
	if artifactUUID == "" {
		return apperror.ValidationFailed.New("artifact UUID is required")
	}
	updatedBy, err := resolveActorErr(r, h.identity, "upsert thumbnail")
	if err != nil {
		return err
	}

	const multipartOverhead = 1 << 20
	r.Body = http.MaxBytesReader(w, r.Body, h.maxBodyBytes+multipartOverhead)
	if err := r.ParseMultipartForm(h.maxBodyBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return apperror.PayloadTooLarge.New("thumbnail body exceeds the maximum allowed size")
		}
		return apperror.ValidationFailed.New("invalid multipart form")
	}

	file, header, fileErr := r.FormFile("file")
	if fileErr != nil {
		return apperror.ValidationFailed.New("`file` field is required")
	}
	defer file.Close()

	data, readErr := io.ReadAll(io.LimitReader(file, h.maxBodyBytes+1))
	if readErr != nil {
		return apperror.ValidationFailed.New("failed to read uploaded file")
	}
	if int64(len(data)) > h.maxBodyBytes {
		return apperror.PayloadTooLarge.New("thumbnail exceeds maximum allowed size")
	}
	if len(data) == 0 {
		return apperror.ValidationFailed.New("thumbnail content is required")
	}
	sniffedContentType := h.service.GetImageContentType(data)
	if !allowedThumbnailContentTypes[sniffedContentType] {
		return apperror.ValidationFailed.New("thumbnail must be a JPEG or PNG image")
	}
	fileName := sanitizeUploadFileName(header.Filename)

	req := &dto.CreateAPIDocumentRequest{
		Type:        constants.DocumentTypeThumbnail,
		Handle:      constants.DocumentHandleThumbnail,
		DisplayName: constants.DocumentDisplayNameThumbnail,
		FileName:    fileName,
		Content:     data,
	}

	if err := h.service.UpsertDocument(req, orgID, updatedBy, artifactUUID); err != nil {
		return serviceError(err, "failed to upsert thumbnail")
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// DeleteThumbnail handles DELETE /apis/{apiType}/{apiId}/thumbnail.
func (h *APIThumbnailHandler) DeleteThumbnail(w http.ResponseWriter, r *http.Request) error {
	orgID, artifactUUID, err := h.resolveArtifactUUID(r)
	if err != nil {
		return err
	}
	deletedBy, err := resolveActorErr(r, h.identity, "delete thumbnail")
	if err != nil {
		return err
	}
	if err := h.service.DeleteAPIThumbnail(artifactUUID, orgID, deletedBy); err != nil {
		return serviceError(err, "failed to delete thumbnail")
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// allowedThumbnailContentTypes is the sniff-based allowlist for thumbnail
// uploads. The uploader's Content-Type header and filename are untrusted —
// http.DetectContentType reads magic bytes from the payload itself.
var allowedThumbnailContentTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
}
