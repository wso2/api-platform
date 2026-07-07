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
	"strconv"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	"github.com/wso2/go-httpkit/httputil"
)

type SecretHandler struct {
	secretService *service.SecretService
	identity      *service.IdentityService
	slogger       *slog.Logger
}

func NewSecretHandler(secretService *service.SecretService, identity *service.IdentityService, slogger *slog.Logger) *SecretHandler {
	return &SecretHandler{secretService: secretService, identity: identity, slogger: slogger}
}

func (h *SecretHandler) RegisterRoutes(mux *http.ServeMux) {
	for _, version := range []string{"/api/v0.9", "/api/v1"} {
		mux.HandleFunc("POST "+version+"/secrets", h.CreateSecret)
		mux.HandleFunc("GET "+version+"/secrets", h.ListSecrets)
		mux.HandleFunc("GET "+version+"/secrets/{secretId}", h.GetSecret)
		mux.HandleFunc("PUT "+version+"/secrets/{secretId}", h.UpdateSecret)
		mux.HandleFunc("DELETE "+version+"/secrets/{secretId}", h.DeleteSecret)
	}
}

func (h *SecretHandler) CreateSecret(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}

	userID, ok := resolveActor(w, r, h.identity, h.slogger, "create secret")
	if !ok {
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		_ = r.ParseForm()
	}
	req := dto.CreateSecretRequest{
		Handle:      r.FormValue("id"),
		DisplayName: r.FormValue("displayName"),
		Description: r.FormValue("description"),
		Value:       r.FormValue("value"),
		Type:        r.FormValue("type"),
	}
	if req.Handle == "" || req.DisplayName == "" || req.Value == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "id, displayName and value are required"))
		return
	}

	resp, err := h.secretService.Create(orgID, userID, &req)
	if err != nil {
		if errors.Is(err, constants.ErrSecretAlreadyExists) {
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponseWithCode(
				utils.CodeCommonConflict, "A secret with this name already exists in this scope"))
			return
		}
		if errors.Is(err, constants.ErrInvalidSecretType) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, err.Error()))
			return
		}
		h.slogger.Error("failed to create secret", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
			utils.CodeCommonInternalError, "Failed to create secret"))
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, resp)
}

func (h *SecretHandler) ListSecrets(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}

	limit := 25
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			if v > 100 {
				v = 100
			}
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	var updatedAfter *time.Time
	if ua := r.URL.Query().Get("updatedAfter"); ua != "" {
		t, err := time.Parse(time.RFC3339, ua)
		if err != nil {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
				utils.CodeCommonValidationFailed, "updatedAfter must be an RFC3339 timestamp"))
			return
		}
		updatedAfter = &t
	}

	resp, err := h.secretService.List(orgID, limit, offset, updatedAfter)
	if err != nil {
		h.slogger.Error("failed to list secrets", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
			utils.CodeCommonInternalError, "Failed to list secrets"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *SecretHandler) GetSecret(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}

	handle := r.PathValue("secretId")
	if handle == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "Secret name is required"))
		return
	}

	summary, err := h.secretService.Get(orgID, handle)
	if err != nil {
		if errors.Is(err, constants.ErrSecretNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeCommonNotFound, "Secret not found"))
			return
		}
		h.slogger.Error("failed to get secret", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
			utils.CodeCommonInternalError, "Failed to get secret"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, summary)
}

func (h *SecretHandler) UpdateSecret(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}

	handle := r.PathValue("secretId")
	if handle == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "Secret name is required"))
		return
	}

	userID, ok := resolveActor(w, r, h.identity, h.slogger, "update secret")
	if !ok {
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		_ = r.ParseForm()
	}
	req := dto.UpdateSecretRequest{
		DisplayName: r.FormValue("displayName"),
		Description: r.FormValue("description"),
		Value:       r.FormValue("value"),
	}
	if req.Value == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "value is required"))
		return
	}

	resp, err := h.secretService.Update(orgID, handle, userID, &req)
	if err != nil {
		if errors.Is(err, constants.ErrSecretNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeCommonNotFound, "Secret not found"))
			return
		}
		h.slogger.Error("failed to update secret", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
			utils.CodeCommonInternalError, "Failed to update secret"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *SecretHandler) DeleteSecret(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}

	handle := r.PathValue("secretId")
	if handle == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponseWithCode(
			utils.CodeCommonValidationFailed, "Secret name is required"))
		return
	}

	userID, ok := resolveActor(w, r, h.identity, h.slogger, "delete secret")
	if !ok {
		return
	}

	err := h.secretService.Delete(orgID, handle, userID)
	if err != nil {
		if errors.Is(err, constants.ErrSecretNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponseWithCode(
				utils.CodeCommonNotFound, "Secret not found"))
			return
		}

		var inUseErr *service.SecretInUseError
		if errors.As(err, &inUseErr) {
			refs := make([]dto.SecretReferenceDTO, 0, len(inUseErr.References))
			for _, ref := range inUseErr.References {
				refs = append(refs, dto.SecretReferenceDTO{Type: ref.Type, Handle: ref.Handle, Name: ref.Name})
			}
			resp := utils.NewErrorResponseWithCode(utils.CodeSecretInUse, "The secret is referenced by one or more active resources.")
			resp.Details = dto.SecretInUseDetails{References: refs}
			httputil.WriteJSON(w, http.StatusConflict, resp)
			return
		}

		h.slogger.Error("failed to delete secret", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
			utils.CodeCommonInternalError, "Failed to delete secret"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
