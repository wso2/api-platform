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
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/wso2/go-httpkit/httputil"
)

type ApplicationHandler struct {
	applicationService *service.ApplicationService
	slogger            *slog.Logger
}

func NewApplicationHandler(applicationService *service.ApplicationService, slogger *slog.Logger) *ApplicationHandler {
	return &ApplicationHandler{applicationService: applicationService, slogger: slogger}
}

func (h *ApplicationHandler) CreateApplication(w http.ResponseWriter, r *http.Request) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	var req api.CreateApplicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.NewValidationErrorResponse(w, err)
		return
	}

	if strings.TrimSpace(req.DisplayName) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "displayName is required"))
		return
	}
	if utils.OpenAPIUUIDToString(req.ProjectId) == "00000000-0000-0000-0000-000000000000" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Project ID is required"))
		return
	}
	if strings.TrimSpace(string(req.Type)) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Application type is required"))
		return
	}

	createdBy := h.resolveRequesterUserID(r)
	app, err := h.applicationService.CreateApplication(&req, orgID, createdBy)
	if err != nil {
		h.writeApplicationError(w, r, err, "Failed to create application")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, app)
}

func (h *ApplicationHandler) GetApplication(w http.ResponseWriter, r *http.Request) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	appID := r.PathValue("id")
	if strings.TrimSpace(appID) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Application ID is required"))
		return
	}

	app, err := h.applicationService.GetApplicationByID(appID, orgID)
	if err != nil {
		h.writeApplicationError(w, r, err, "Failed to get application")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, app)
}

func (h *ApplicationHandler) ListApplications(w http.ResponseWriter, r *http.Request) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	projectID := strings.TrimSpace(r.URL.Query().Get("projectId"))
	if projectID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Project ID is required"))
		return
	}

	var limitStr string
	if v := r.URL.Query().Get("limit"); v != "" {
		limitStr = v
	} else {
		limitStr = "20"
	}
	var offsetStr string
	if v := r.URL.Query().Get("offset"); v != "" {
		offsetStr = v
	} else {
		offsetStr = "0"
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	apps, err := h.applicationService.GetApplicationsByOrganization(orgID, projectID, limit, offset)
	if err != nil {
		h.writeApplicationError(w, r, err, "Failed to list applications")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, apps)
}

func (h *ApplicationHandler) UpdateApplication(w http.ResponseWriter, r *http.Request) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	appID := r.PathValue("id")
	if strings.TrimSpace(appID) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Application ID is required"))
		return
	}
	userID := h.resolveRequesterUserID(r)

	var req api.Application
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.NewValidationErrorResponse(w, err)
		return
	}

	app, err := h.applicationService.UpdateApplication(appID, &req, orgID, userID)
	if err != nil {
		h.writeApplicationError(w, r, err, "Failed to update application")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, app)
}

func (h *ApplicationHandler) DeleteApplication(w http.ResponseWriter, r *http.Request) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	appID := r.PathValue("id")
	if strings.TrimSpace(appID) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Application ID is required"))
		return
	}
	userID := h.resolveRequesterUserID(r)

	if err := h.applicationService.DeleteApplication(appID, orgID, userID); err != nil {
		h.writeApplicationError(w, r, err, "Failed to delete application")
		return
	}

	httputil.WriteJSON(w, http.StatusNoContent, nil)
}

func (h *ApplicationHandler) ListApplicationAssociations(w http.ResponseWriter, r *http.Request) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	appID := r.PathValue("id")
	if strings.TrimSpace(appID) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Application ID is required"))
		return
	}

	limit := 20
	if limitStr := strings.TrimSpace(r.URL.Query().Get("limit")); limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err == nil && parsedLimit > 0 {
			if parsedLimit > 100 {
				parsedLimit = 100
			}
			limit = parsedLimit
		}
	}

	offset := 0
	if offsetStr := strings.TrimSpace(r.URL.Query().Get("offset")); offsetStr != "" {
		parsedOffset, err := strconv.Atoi(offsetStr)
		if err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	associations, err := h.applicationService.ListApplicationAssociations(appID, orgID, limit, offset)
	if err != nil {
		h.writeApplicationError(w, r, err, "Failed to list application associations")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, associations)
}

func (h *ApplicationHandler) AddApplicationAssociations(w http.ResponseWriter, r *http.Request) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	appID := r.PathValue("id")
	if strings.TrimSpace(appID) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Application ID is required"))
		return
	}

	var req service.AddApplicationAssociationsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.NewValidationErrorResponse(w, err)
		return
	}
	if len(req.Associations) == 0 {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "At least one association is required"))
		return
	}

	associations, err := h.applicationService.AddApplicationAssociations(appID, &req, orgID)
	if err != nil {
		h.writeApplicationError(w, r, err, "Failed to add application associations")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, associations)
}

func (h *ApplicationHandler) RemoveApplicationAssociation(w http.ResponseWriter, r *http.Request) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	appID := r.PathValue("id")
	associationID := r.PathValue("associationId")
	if strings.TrimSpace(appID) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Application ID is required"))
		return
	}
	if strings.TrimSpace(associationID) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Association ID is required"))
		return
	}

	if err := h.applicationService.RemoveApplicationAssociation(appID, associationID, orgID); err != nil {
		h.writeApplicationError(w, r, err, "Failed to remove application association")
		return
	}

	httputil.WriteJSON(w, http.StatusNoContent, nil)
}

func (h *ApplicationHandler) ListApplicationAPIKeys(w http.ResponseWriter, r *http.Request) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	appID := r.PathValue("id")
	if strings.TrimSpace(appID) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Application ID is required"))
		return
	}

	limit := 20
	if limitStr := strings.TrimSpace(r.URL.Query().Get("limit")); limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err == nil && parsedLimit > 0 {
			if parsedLimit > 100 {
				parsedLimit = 100
			}
			limit = parsedLimit
		}
	}

	offset := 0
	if offsetStr := strings.TrimSpace(r.URL.Query().Get("offset")); offsetStr != "" {
		parsedOffset, err := strconv.Atoi(offsetStr)
		if err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	keys, err := h.applicationService.ListMappedAPIKeys(appID, orgID, limit, offset)
	if err != nil {
		h.writeApplicationError(w, r, err, "Failed to list mapped API keys")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, keys)
}

func (h *ApplicationHandler) ListApplicationAssociationAPIKeys(w http.ResponseWriter, r *http.Request) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	appID := r.PathValue("id")
	associationID := r.PathValue("associationId")
	if strings.TrimSpace(appID) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Application ID is required"))
		return
	}
	if strings.TrimSpace(associationID) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Association ID is required"))
		return
	}

	limit := 20
	if limitStr := strings.TrimSpace(r.URL.Query().Get("limit")); limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err == nil && parsedLimit > 0 {
			if parsedLimit > 100 {
				parsedLimit = 100
			}
			limit = parsedLimit
		}
	}

	offset := 0
	if offsetStr := strings.TrimSpace(r.URL.Query().Get("offset")); offsetStr != "" {
		parsedOffset, err := strconv.Atoi(offsetStr)
		if err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	keys, err := h.applicationService.ListMappedAPIKeysForAssociation(appID, associationID, orgID, limit, offset)
	if err != nil {
		h.writeApplicationError(w, r, err, "Failed to list mapped API keys for association")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, keys)
}

func (h *ApplicationHandler) AddApplicationAPIKeys(w http.ResponseWriter, r *http.Request) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	appID := r.PathValue("id")
	userID := h.resolveRequesterUserID(r)
	if strings.TrimSpace(appID) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Application ID is required"))
		return
	}

	var req api.AddApplicationAPIKeysRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.NewValidationErrorResponse(w, err)
		return
	}
	if len(req.ApiKeys) == 0 {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "At least one API key mapping is required"))
		return
	}

	keys, err := h.applicationService.AddMappedAPIKeys(appID, &req, orgID, userID)
	if err != nil {
		h.writeApplicationError(w, r, err, "Failed to add mapped API keys")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, keys)
}

func (h *ApplicationHandler) RemoveApplicationAPIKey(w http.ResponseWriter, r *http.Request) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	appID := r.PathValue("id")
	keyID := r.PathValue("apiKeyId")
	entityID := strings.TrimSpace(r.URL.Query().Get("entityID"))
	userID := h.resolveRequesterUserID(r)
	if strings.TrimSpace(appID) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Application ID is required"))
		return
	}
	if strings.TrimSpace(keyID) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "API key id is required"))
		return
	}
	if entityID == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Entity ID is required"))
		return
	}

	if err := h.applicationService.RemoveMappedAPIKey(appID, keyID, entityID, orgID, userID); err != nil {
		h.writeApplicationError(w, r, err, "Failed to remove mapped API key")
		return
	}

	httputil.WriteJSON(w, http.StatusNoContent, nil)
}

func (h *ApplicationHandler) RegisterRoutes(mux *http.ServeMux) {
	base := constants.APIBasePath + "/applications"
	mux.HandleFunc("GET "+base, h.ListApplications)
	mux.HandleFunc("POST "+base, h.CreateApplication)
	mux.HandleFunc("GET "+base+"/{id}", h.GetApplication)
	mux.HandleFunc("PUT "+base+"/{id}", h.UpdateApplication)
	mux.HandleFunc("DELETE "+base+"/{id}", h.DeleteApplication)

	mux.HandleFunc("GET "+base+"/{id}/api-keys", h.ListApplicationAPIKeys)
	mux.HandleFunc("POST "+base+"/{id}/api-keys", h.AddApplicationAPIKeys)
	mux.HandleFunc("DELETE "+base+"/{id}/api-keys/{apiKeyId}", h.RemoveApplicationAPIKey)
	mux.HandleFunc("GET "+base+"/{id}/associations", h.ListApplicationAssociations)
	mux.HandleFunc("POST "+base+"/{id}/associations", h.AddApplicationAssociations)
	mux.HandleFunc("GET "+base+"/{id}/associations/{associationId}/api-keys", h.ListApplicationAssociationAPIKeys)
	mux.HandleFunc("DELETE "+base+"/{id}/associations/{associationId}", h.RemoveApplicationAssociation)
}

func (h *ApplicationHandler) resolveRequesterUserID(r *http.Request) string {
	userID := strings.TrimSpace(r.Header.Get("x-user-id"))
	if userID != "" {
		return userID
	}

	if ctxUserID, ok := middleware.GetUserIDFromRequest(r); ok {
		return strings.TrimSpace(ctxUserID)
	}

	return ""
}

func (h *ApplicationHandler) writeApplicationError(w http.ResponseWriter, r *http.Request, err error, fallback string) {
	if h.slogger != nil {
		h.slogger.Error(fallback,
			"error", err,
			"path", r.URL.Path,
			"method", r.Method,
			"id", r.PathValue("id"),
		)
	}

	switch {
	case errors.Is(err, constants.ErrApplicationNotFound):
		httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Application not found"))
	case errors.Is(err, constants.ErrProjectNotFound):
		httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Project not found"))
	case errors.Is(err, constants.ErrOrganizationNotFound):
		httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Organization not found"))
	case errors.Is(err, constants.ErrApplicationExists):
		httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "Application already exists in project"))
	case errors.Is(err, constants.ErrHandleExists):
		httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "Application handle already exists in organization"))
	case errors.Is(err, constants.ErrAPIKeyNotFound):
		httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "API key not found"))
	case errors.Is(err, constants.ErrAPIKeyForbidden):
		httputil.WriteJSON(w, http.StatusForbidden, utils.NewErrorResponse(403, "Forbidden", "Only the key creator can perform this action"))
	case errors.Is(err, constants.ErrInvalidApplicationName):
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "displayName is required"))
	case errors.Is(err, constants.ErrInvalidApplicationType):
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Application type is required"))
	case errors.Is(err, constants.ErrUnsupportedApplicationType):
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid application type. Only 'genai' is supported"))
	case errors.Is(err, constants.ErrInvalidHandle):
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid application handle format"))
	case errors.Is(err, constants.ErrInvalidApplicationID):
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid application id"))
	case errors.Is(err, constants.ErrInvalidAPIKey):
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid API key id"))
	case errors.Is(err, constants.ErrArtifactNotFound):
		httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Association target not found"))
	case errors.Is(err, constants.ErrArtifactInvalidKind):
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid association kind. Only LlmProvider and LlmProxy are supported"))
	case errors.Is(err, constants.ErrInvalidInput):
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Invalid application association input"))
	case errors.Is(err, constants.ErrHandleImmutable):
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"The id is immutable and must match the application being updated"))
	default:
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", fallback))
	}
}
