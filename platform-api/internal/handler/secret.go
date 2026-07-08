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

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/service"

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
		mux.HandleFunc("POST "+version+"/secrets", middleware.MapErrors(h.slogger, h.CreateSecret))
		mux.HandleFunc("GET "+version+"/secrets", middleware.MapErrors(h.slogger, h.ListSecrets))
		mux.HandleFunc("GET "+version+"/secrets/{secretId}", middleware.MapErrors(h.slogger, h.GetSecret))
		mux.HandleFunc("PUT "+version+"/secrets/{secretId}", middleware.MapErrors(h.slogger, h.UpdateSecret))
		mux.HandleFunc("DELETE "+version+"/secrets/{secretId}", middleware.MapErrors(h.slogger, h.DeleteSecret))
	}
}

func (h *SecretHandler) CreateSecret(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	userID, err := resolveActorErr(r, h.identity, "create secret")
	if err != nil {
		return err
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
		return apperror.ValidationFailed.New("id, displayName and value are required")
	}

	resp, err := h.secretService.Create(orgID, userID, &req)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return err
		}
		// Repository-origin duplicate (unique constraint) still surfaces as a sentinel.
		if errors.Is(err, constants.ErrSecretAlreadyExists) {
			return apperror.SecretExists.Wrap(err)
		}
		return apperror.Internal.Wrap(err).WithLogMessage("failed to create secret")
	}

	setLocation(w, "secrets", resp.Handle)
	httputil.WriteJSON(w, http.StatusCreated, resp)
	return nil
}

func (h *SecretHandler) ListSecrets(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
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
			return apperror.ValidationFailed.Wrap(err, "updatedAfter must be an RFC3339 timestamp")
		}
		updatedAfter = &t
	}

	resp, err := h.secretService.List(orgID, limit, offset, updatedAfter)
	if err != nil {
		return apperror.Internal.Wrap(err).WithLogMessage("failed to list secrets")
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

func (h *SecretHandler) GetSecret(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	handle := r.PathValue("secretId")
	if handle == "" {
		return apperror.ValidationFailed.New("Secret name is required")
	}

	summary, err := h.secretService.Get(orgID, handle)
	if err != nil {
		if errors.Is(err, constants.ErrSecretNotFound) {
			return apperror.SecretNotFound.Wrap(err)
		}
		return apperror.Internal.Wrap(err).WithLogMessage("failed to get secret")
	}

	httputil.WriteJSON(w, http.StatusOK, summary)
	return nil
}

func (h *SecretHandler) UpdateSecret(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	handle := r.PathValue("secretId")
	if handle == "" {
		return apperror.ValidationFailed.New("Secret name is required")
	}

	userID, err := resolveActorErr(r, h.identity, "update secret")
	if err != nil {
		return err
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
		return apperror.ValidationFailed.New("value is required")
	}

	resp, err := h.secretService.Update(orgID, handle, userID, &req)
	if err != nil {
		if errors.Is(err, constants.ErrSecretNotFound) {
			return apperror.SecretNotFound.Wrap(err)
		}
		return apperror.Internal.Wrap(err).WithLogMessage("failed to update secret")
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

func (h *SecretHandler) DeleteSecret(w http.ResponseWriter, r *http.Request) error {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return apperror.Unauthorized.New().
			WithLogMessage("organization claim not found in token")
	}

	handle := r.PathValue("secretId")
	if handle == "" {
		return apperror.ValidationFailed.New("Secret name is required")
	}

	userID, err := resolveActorErr(r, h.identity, "delete secret")
	if err != nil {
		return err
	}

	err = h.secretService.Delete(orgID, handle, userID)
	if err != nil {
		if errors.Is(err, constants.ErrSecretNotFound) {
			return apperror.SecretNotFound.Wrap(err)
		}

		var inUseErr *service.SecretInUseError
		if errors.As(err, &inUseErr) {
			refs := make([]dto.SecretReferenceDTO, 0, len(inUseErr.References))
			for _, ref := range inUseErr.References {
				refs = append(refs, dto.SecretReferenceDTO{Type: ref.Type, Handle: ref.Handle, Name: ref.Name})
			}
			return apperror.SecretInUse.New().WithDetails(dto.SecretInUseDetails{References: refs})
		}

		return apperror.Internal.Wrap(err).WithLogMessage("failed to delete secret")
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}
