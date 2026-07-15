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
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
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
		return serviceError(err, "failed to create secret")
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

	limit, offset := parsePagination(r)

	var updatedAfter *time.Time
	if ua := r.URL.Query().Get("updatedAfter"); ua != "" {
		t, err := time.Parse(time.RFC3339, ua)
		if err != nil {
			return apperror.ValidationFailed.Wrap(err, "updatedAfter must be an RFC3339 timestamp")
		}
		// Normalize to UTC so the comparison against UTC-stored updated_at
		// values is a correct chronological comparison regardless of the
		// offset the client sent (some drivers compare timestamp bind
		// parameters as text, where mixed offsets sort incorrectly).
		utc := t.UTC()
		updatedAfter = &utc
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
		return serviceError(err, "failed to get secret")
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
		return serviceError(err, "failed to update secret")
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
		// SecretInUseError is a typed service error rather than a catalog error: it
		// carries the blocking references, which become the response's details field.
		var inUseErr *service.SecretInUseError
		if errors.As(err, &inUseErr) {
			refs := make([]dto.SecretReferenceDTO, 0, len(inUseErr.References))
			for _, ref := range inUseErr.References {
				refs = append(refs, dto.SecretReferenceDTO{Type: ref.Type, Handle: ref.Handle, Name: ref.Name})
			}
			return apperror.SecretInUse.New().WithDetails(dto.SecretInUseDetails{References: refs})
		}

		return serviceError(err, "failed to delete secret")
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}
