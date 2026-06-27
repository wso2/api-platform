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

	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/wso2/go-httpkit/httputil"
)

type SecretHandler struct {
	secretService *service.SecretService
	slogger       *slog.Logger
}

func NewSecretHandler(secretService *service.SecretService, slogger *slog.Logger) *SecretHandler {
	return &SecretHandler{secretService: secretService, slogger: slogger}
}

func (h *SecretHandler) RegisterRoutes(mux *http.ServeMux) {
	for _, version := range []string{"/api/v0.9", "/api/v1"} {
		mux.HandleFunc("POST "+version+"/secrets", h.CreateSecret)
		mux.HandleFunc("GET "+version+"/secrets", h.ListSecrets)
		mux.HandleFunc("GET "+version+"/secrets/{handle}", h.GetSecret)
		mux.HandleFunc("PUT "+version+"/secrets/{handle}", h.UpdateSecret)
		mux.HandleFunc("DELETE "+version+"/secrets/{handle}", h.DeleteSecret)
	}
}

func (h *SecretHandler) CreateSecret(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	username, _ := middleware.GetUsernameFromRequest(r)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		_ = r.ParseForm()
	}
	req := dto.CreateSecretRequest{
		Handle:      r.FormValue("handle"),
		DisplayName: r.FormValue("name"),
		Description: r.FormValue("description"),
		Value:       r.FormValue("value"),
		Type:        r.FormValue("type"),
	}
	if req.Handle == "" || req.Value == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "handle and value are required"))
		return
	}

	resp, err := h.secretService.Create(orgID, username, &req)
	if err != nil {
		if errors.Is(err, constants.ErrSecretAlreadyExists) {
			httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "A secret with this name already exists in this scope"))
			return
		}
		if errors.Is(err, constants.ErrInvalidSecretType) {
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
			return
		}
		h.slogger.Error("failed to create secret", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to create secret"))
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, resp)
}

func (h *SecretHandler) ListSecrets(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
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
			httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "updatedAfter must be an RFC3339 timestamp"))
			return
		}
		updatedAfter = &t
	}

	resp, err := h.secretService.List(orgID, limit, offset, updatedAfter)
	if err != nil {
		h.slogger.Error("failed to list secrets", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to list secrets"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *SecretHandler) GetSecret(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	handle := r.PathValue("handle")
	if handle == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Secret name is required"))
		return
	}

	summary, err := h.secretService.Get(orgID, handle)
	if err != nil {
		if errors.Is(err, constants.ErrSecretNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Secret not found"))
			return
		}
		h.slogger.Error("failed to get secret", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to get secret"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, summary)
}

func (h *SecretHandler) UpdateSecret(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	handle := r.PathValue("handle")
	if handle == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Secret name is required"))
		return
	}

	username, _ := middleware.GetUsernameFromRequest(r)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		_ = r.ParseForm()
	}
	req := dto.UpdateSecretRequest{
		DisplayName: r.FormValue("name"),
		Description: r.FormValue("description"),
		Value:       r.FormValue("value"),
	}
	if req.Value == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "value is required"))
		return
	}

	resp, err := h.secretService.Update(orgID, handle, username, &req)
	if err != nil {
		if errors.Is(err, constants.ErrSecretNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Secret not found"))
			return
		}
		h.slogger.Error("failed to update secret", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to update secret"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (h *SecretHandler) DeleteSecret(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	handle := r.PathValue("handle")
	if handle == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Secret name is required"))
		return
	}

	username, _ := middleware.GetUsernameFromRequest(r)

	err := h.secretService.Delete(orgID, handle, username)
	if err != nil {
		if errors.Is(err, constants.ErrSecretNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Secret not found"))
			return
		}

		var inUseErr *service.SecretInUseError
		if errors.As(err, &inUseErr) {
			refs := make([]dto.SecretReferenceDTO, 0, len(inUseErr.References))
			for _, ref := range inUseErr.References {
				refs = append(refs, dto.SecretReferenceDTO{Type: ref.Type, Handle: ref.Handle, DisplayName: ref.Name})
			}
			httputil.WriteJSON(w, http.StatusConflict, dto.SecretDeleteConflictResponse{
				Error:      "secret is referenced by active resources",
				References: refs,
			})
			return
		}

		h.slogger.Error("failed to delete secret", "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to delete secret"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
