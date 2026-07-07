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
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	"github.com/wso2/go-httpkit/httputil"
)

// APIKeyUserHandler handles listing API keys for a user across artifact types.
type APIKeyUserHandler struct {
	apiKeyUserService *service.APIKeyUserService
	identity          *service.IdentityService
	slogger           *slog.Logger
}

// NewAPIKeyUserHandler creates a new APIKeyUserHandler.
func NewAPIKeyUserHandler(apiKeyUserService *service.APIKeyUserService, identity *service.IdentityService, slogger *slog.Logger) *APIKeyUserHandler {
	return &APIKeyUserHandler{
		apiKeyUserService: apiKeyUserService,
		identity:          identity,
		slogger:           slogger,
	}
}

// ListUserAPIKeys handles GET /api/v0.9/me/api-keys
func (h *APIKeyUserHandler) ListUserAPIKeys(w http.ResponseWriter, r *http.Request) {
	orgID, exists := middleware.GetOrganizationFromRequest(r)
	if !exists {
		httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponseWithCode(
			utils.CodeCommonUnauthorized, "Organization claim not found in token"))
		return
	}

	callerUserID, ok := resolveActor(w, r, h.identity, h.slogger, "list user API keys")
	if !ok {
		return
	}

	var types []string
	if typeParam := r.URL.Query().Get("type"); typeParam != "" {
		types = strings.Split(typeParam, ",")
	}

	response, err := h.apiKeyUserService.ListAPIKeysByUser(r.Context(), orgID, callerUserID, types)
	if err != nil {
		h.slogger.Error("Failed to list API keys for user", "orgId", orgID, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponseWithCode(
			utils.CodeCommonInternalError, "Failed to list API keys"))
		return
	}

	httputil.WriteJSON(w, http.StatusOK, response)
}

// RegisterRoutes registers the user API key routes.
func (h *APIKeyUserHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+constants.APIBasePath+"/me/api-keys", h.ListUserAPIKeys)
}
