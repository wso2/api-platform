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
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/service"

	"github.com/wso2/go-httpkit/httputil"
)

type ApplicationHandler struct {
	applicationService *service.ApplicationService
	identity           *service.IdentityService
	slogger            *slog.Logger
}

func NewApplicationHandler(applicationService *service.ApplicationService, identity *service.IdentityService, slogger *slog.Logger) *ApplicationHandler {
	return &ApplicationHandler{applicationService: applicationService, identity: identity, slogger: slogger}
}

func (h *ApplicationHandler) CreateApplication(w http.ResponseWriter, r *http.Request) error {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	var req api.CreateApplicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.NewValidation(err)
	}

	if strings.TrimSpace(req.DisplayName) == "" {
		return apperror.ValidationFailed.New("displayName is required")
	}
	if strings.TrimSpace(req.ProjectId) == "" {
		return apperror.ValidationFailed.New("Project ID is required")
	}
	if strings.TrimSpace(string(req.Type)) == "" {
		return apperror.ValidationFailed.New("Application type is required")
	}

	createdBy, err := resolveActorErr(r, h.identity, "create application")
	if err != nil {
		return err
	}
	app, err := h.applicationService.CreateApplication(&req, orgID, createdBy)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to create application in project %s for org %s by user %s", req.ProjectId, orgID, createdBy))
	}

	setLocation(w, "applications", app.Id)
	httputil.WriteJSON(w, http.StatusCreated, app)
	return nil
}

func (h *ApplicationHandler) GetApplication(w http.ResponseWriter, r *http.Request) error {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	appID := r.PathValue("applicationId")
	if strings.TrimSpace(appID) == "" {
		return apperror.ValidationFailed.New("Application ID is required")
	}

	app, err := h.applicationService.GetApplicationByID(appID, orgID)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to get application %s in org %s", appID, orgID))
	}

	httputil.WriteJSON(w, http.StatusOK, app)
	return nil
}

func (h *ApplicationHandler) ListApplications(w http.ResponseWriter, r *http.Request) error {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	projectID := strings.TrimSpace(r.URL.Query().Get("projectId"))
	if projectID == "" {
		return apperror.ValidationFailed.New("Project ID is required")
	}

	opts := parseListOptions(r)

	apps, err := h.applicationService.GetApplicationsByOrganization(orgID, projectID, opts)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to list applications for project %s in org %s", projectID, orgID))
	}

	httputil.WriteJSON(w, http.StatusOK, apps)
	return nil
}

func (h *ApplicationHandler) UpdateApplication(w http.ResponseWriter, r *http.Request) error {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	appID := r.PathValue("applicationId")
	if strings.TrimSpace(appID) == "" {
		return apperror.ValidationFailed.New("Application ID is required")
	}
	userID, err := resolveActorErr(r, h.identity, "update application")
	if err != nil {
		return err
	}

	var req api.Application
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.NewValidation(err)
	}

	app, err := h.applicationService.UpdateApplication(appID, &req, orgID, userID)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to update application %s in org %s by user %s", appID, orgID, userID))
	}

	httputil.WriteJSON(w, http.StatusOK, app)
	return nil
}

func (h *ApplicationHandler) DeleteApplication(w http.ResponseWriter, r *http.Request) error {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	appID := r.PathValue("applicationId")
	if strings.TrimSpace(appID) == "" {
		return apperror.ValidationFailed.New("Application ID is required")
	}
	userID, err := resolveActorErr(r, h.identity, "delete application")
	if err != nil {
		return err
	}

	if err := h.applicationService.DeleteApplication(appID, orgID, userID); err != nil {
		return serviceError(err, fmt.Sprintf("failed to delete application %s in org %s by user %s", appID, orgID, userID))
	}

	httputil.WriteJSON(w, http.StatusNoContent, nil)
	return nil
}

func (h *ApplicationHandler) ListApplicationAssociations(w http.ResponseWriter, r *http.Request) error {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	appID := r.PathValue("applicationId")
	if strings.TrimSpace(appID) == "" {
		return apperror.ValidationFailed.New("Application ID is required")
	}

	limit, offset := parsePagination(r)

	associations, err := h.applicationService.ListApplicationAssociations(appID, orgID, limit, offset)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to list associations for application %s in org %s", appID, orgID))
	}

	httputil.WriteJSON(w, http.StatusOK, associations)
	return nil
}

func (h *ApplicationHandler) AddApplicationAssociations(w http.ResponseWriter, r *http.Request) error {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	appID := r.PathValue("applicationId")
	if strings.TrimSpace(appID) == "" {
		return apperror.ValidationFailed.New("Application ID is required")
	}

	var req service.AddApplicationAssociationsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.NewValidation(err)
	}
	if len(req.Associations) == 0 {
		return apperror.ValidationFailed.New("At least one association is required")
	}

	associations, err := h.applicationService.AddApplicationAssociations(appID, &req, orgID)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to add associations for application %s in org %s", appID, orgID))
	}

	httputil.WriteJSON(w, http.StatusOK, associations)
	return nil
}

func (h *ApplicationHandler) RemoveApplicationAssociation(w http.ResponseWriter, r *http.Request) error {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	appID := r.PathValue("applicationId")
	associationID := r.PathValue("associationId")
	if strings.TrimSpace(appID) == "" {
		return apperror.ValidationFailed.New("Application ID is required")
	}
	if strings.TrimSpace(associationID) == "" {
		return apperror.ValidationFailed.New("Association ID is required")
	}

	if err := h.applicationService.RemoveApplicationAssociation(appID, associationID, orgID); err != nil {
		return serviceError(err, fmt.Sprintf("failed to remove association %s for application %s in org %s", associationID, appID, orgID))
	}

	httputil.WriteJSON(w, http.StatusNoContent, nil)
	return nil
}

func (h *ApplicationHandler) ListApplicationAPIKeys(w http.ResponseWriter, r *http.Request) error {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	appID := r.PathValue("applicationId")
	if strings.TrimSpace(appID) == "" {
		return apperror.ValidationFailed.New("Application ID is required")
	}

	limit, offset := parsePagination(r)

	keys, err := h.applicationService.ListMappedAPIKeys(appID, orgID, limit, offset)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to list mapped API keys for application %s in org %s", appID, orgID))
	}

	httputil.WriteJSON(w, http.StatusOK, keys)
	return nil
}

func (h *ApplicationHandler) ListApplicationAssociationAPIKeys(w http.ResponseWriter, r *http.Request) error {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	appID := r.PathValue("applicationId")
	associationID := r.PathValue("associationId")
	if strings.TrimSpace(appID) == "" {
		return apperror.ValidationFailed.New("Application ID is required")
	}
	if strings.TrimSpace(associationID) == "" {
		return apperror.ValidationFailed.New("Association ID is required")
	}

	limit, offset := parsePagination(r)

	keys, err := h.applicationService.ListMappedAPIKeysForAssociation(appID, associationID, orgID, limit, offset)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to list mapped API keys for association %s of application %s in org %s", associationID, appID, orgID))
	}

	httputil.WriteJSON(w, http.StatusOK, keys)
	return nil
}

func (h *ApplicationHandler) AddApplicationAPIKeys(w http.ResponseWriter, r *http.Request) error {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	appID := r.PathValue("applicationId")
	userID, err := resolveActorErr(r, h.identity, "add application API keys")
	if err != nil {
		return err
	}
	if strings.TrimSpace(appID) == "" {
		return apperror.ValidationFailed.New("Application ID is required")
	}

	var req api.AddApplicationAPIKeysRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.NewValidation(err)
	}
	if len(req.ApiKeys) == 0 {
		return apperror.ValidationFailed.New("At least one API key mapping is required")
	}

	keys, err := h.applicationService.AddMappedAPIKeys(appID, &req, orgID, userID)
	if err != nil {
		return serviceError(err, fmt.Sprintf("failed to add mapped API keys for application %s in org %s by user %s", appID, orgID, userID))
	}

	httputil.WriteJSON(w, http.StatusOK, keys)
	return nil
}

func (h *ApplicationHandler) RemoveApplicationAPIKey(w http.ResponseWriter, r *http.Request) error {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	appID := r.PathValue("applicationId")
	keyID := r.PathValue("apiKeyId")
	entityID := strings.TrimSpace(r.URL.Query().Get("entityID"))
	userID, err := resolveActorErr(r, h.identity, "remove mapped application API key")
	if err != nil {
		return err
	}
	if strings.TrimSpace(appID) == "" {
		return apperror.ValidationFailed.New("Application ID is required")
	}
	if strings.TrimSpace(keyID) == "" {
		return apperror.ValidationFailed.New("API key id is required")
	}
	if entityID == "" {
		return apperror.ValidationFailed.New("Entity ID is required")
	}

	if err := h.applicationService.RemoveMappedAPIKey(appID, keyID, entityID, orgID, userID); err != nil {
		return serviceError(err, fmt.Sprintf("failed to remove mapped API key %s for application %s in org %s by user %s", keyID, appID, orgID, userID))
	}

	httputil.WriteJSON(w, http.StatusNoContent, nil)
	return nil
}

func (h *ApplicationHandler) RegisterRoutes(mux *http.ServeMux) {
	base := constants.APIBasePath + "/applications"
	mux.HandleFunc("GET "+base, middleware.MapErrors(h.slogger, h.ListApplications))
	mux.HandleFunc("POST "+base, middleware.MapErrors(h.slogger, h.CreateApplication))
	mux.HandleFunc("GET "+base+"/{applicationId}", middleware.MapErrors(h.slogger, h.GetApplication))
	mux.HandleFunc("PUT "+base+"/{applicationId}", middleware.MapErrors(h.slogger, h.UpdateApplication))
	mux.HandleFunc("DELETE "+base+"/{applicationId}", middleware.MapErrors(h.slogger, h.DeleteApplication))

	mux.HandleFunc("GET "+base+"/{applicationId}/api-keys", middleware.MapErrors(h.slogger, h.ListApplicationAPIKeys))
	mux.HandleFunc("POST "+base+"/{applicationId}/api-keys", middleware.MapErrors(h.slogger, h.AddApplicationAPIKeys))
	mux.HandleFunc("DELETE "+base+"/{applicationId}/api-keys/{apiKeyId}", middleware.MapErrors(h.slogger, h.RemoveApplicationAPIKey))
	mux.HandleFunc("GET "+base+"/{applicationId}/associations", middleware.MapErrors(h.slogger, h.ListApplicationAssociations))
	mux.HandleFunc("POST "+base+"/{applicationId}/associations", middleware.MapErrors(h.slogger, h.AddApplicationAssociations))
	mux.HandleFunc("GET "+base+"/{applicationId}/associations/{associationId}/api-keys", middleware.MapErrors(h.slogger, h.ListApplicationAssociationAPIKeys))
	mux.HandleFunc("DELETE "+base+"/{applicationId}/associations/{associationId}", middleware.MapErrors(h.slogger, h.RemoveApplicationAssociation))
}
