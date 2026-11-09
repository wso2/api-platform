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
	"log/slog"
	"net/http"
	"strconv"

	"github.com/wso2/api-platform/httpkit/httputil"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/router"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

type WebhookSecretHandler struct {
	svc      *service.WebhookSecretService
	identity *service.IdentityService
	slogger  *slog.Logger
}

func NewWebhookSecretHandler(svc *service.WebhookSecretService, identity *service.IdentityService, slogger *slog.Logger) *WebhookSecretHandler {
	return &WebhookSecretHandler{svc: svc, identity: identity, slogger: slogger}
}

func (h *WebhookSecretHandler) RegisterRoutes(mux router.Router) {
	base := constants.APIBasePath + "/webhook-secrets"
	mux.HandleFunc("POST "+base, middleware.MapErrors(h.slogger, h.Create))
	mux.HandleFunc("GET "+base, middleware.MapErrors(h.slogger, h.List))
	mux.HandleFunc("GET "+base+"/{secretId}", middleware.MapErrors(h.slogger, h.Get))
	mux.HandleFunc("DELETE "+base+"/{secretId}", middleware.MapErrors(h.slogger, h.Delete))
	mux.HandleFunc("POST "+base+"/{secretId}/rotate", middleware.MapErrors(h.slogger, h.Rotate))
	mux.HandleFunc("DELETE "+base+"/{secretId}/versions/{version}", middleware.MapErrors(h.slogger, h.DeleteVersion))
}

func webhookSecretOrg(r *http.Request) (string, error) {
	orgID, ok := middleware.GetOrganizationFromRequest(r)
	if !ok {
		return "", apperror.Unauthorized.New().WithLogMessage("organization claim not found in token")
	}
	return orgID, nil
}

func (h *WebhookSecretHandler) Create(w http.ResponseWriter, r *http.Request) error {
	orgID, err := webhookSecretOrg(r)
	if err != nil {
		return err
	}
	actor, err := resolveActorErr(r, h.identity, "create webhook secret")
	if err != nil {
		return err
	}
	var req dto.CreateWebhookSecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationFailed.Wrap(err, "The request body is not valid JSON.")
	}
	resp, err := h.svc.Create(r.Context(), orgID, actor, &req)
	if err != nil {
		return serviceError(err, "failed to create webhook secret")
	}
	setLocation(w, "webhook-secrets", resp.Handle)
	httputil.WriteJSON(w, http.StatusCreated, resp)
	return nil
}

func (h *WebhookSecretHandler) Rotate(w http.ResponseWriter, r *http.Request) error {
	orgID, err := webhookSecretOrg(r)
	if err != nil {
		return err
	}
	actor, err := resolveActorErr(r, h.identity, "rotate webhook secret")
	if err != nil {
		return err
	}
	resp, err := h.svc.Rotate(r.Context(), orgID, r.PathValue("secretId"), actor)
	if err != nil {
		return serviceError(err, "failed to rotate webhook secret")
	}
	httputil.WriteJSON(w, http.StatusCreated, resp)
	return nil
}

func (h *WebhookSecretHandler) List(w http.ResponseWriter, r *http.Request) error {
	orgID, err := webhookSecretOrg(r)
	if err != nil {
		return err
	}
	limit, offset := parsePagination(r)
	resp, err := h.svc.List(orgID, limit, offset)
	if err != nil {
		return serviceError(err, "failed to list webhook secrets")
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

func (h *WebhookSecretHandler) Get(w http.ResponseWriter, r *http.Request) error {
	orgID, err := webhookSecretOrg(r)
	if err != nil {
		return err
	}
	resp, err := h.svc.Get(orgID, r.PathValue("secretId"))
	if err != nil {
		return serviceError(err, "failed to get webhook secret")
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
	return nil
}

func (h *WebhookSecretHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	orgID, err := webhookSecretOrg(r)
	if err != nil {
		return err
	}
	if err := h.svc.Delete(orgID, r.PathValue("secretId")); err != nil {
		return serviceError(err, "failed to delete webhook secret")
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *WebhookSecretHandler) DeleteVersion(w http.ResponseWriter, r *http.Request) error {
	orgID, err := webhookSecretOrg(r)
	if err != nil {
		return err
	}
	actor, err := resolveActorErr(r, h.identity, "delete webhook secret version")
	if err != nil {
		return err
	}
	version, convErr := strconv.Atoi(r.PathValue("version"))
	if convErr != nil || version < 1 {
		return apperror.ValidationFailed.New("The version must be a positive integer.")
	}
	if err := h.svc.DeleteVersion(orgID, r.PathValue("secretId"), actor, version); err != nil {
		return serviceError(err, "failed to delete webhook secret version")
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
