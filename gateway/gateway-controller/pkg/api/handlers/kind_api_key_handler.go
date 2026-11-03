/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/httpkit/httputil"
)

// The API key routes of the model kinds. They have the same verbs, bodies and
// answers as an LLM proxy's key routes, for any artifact kind the API key service
// handles; noun names the kind in the answers.

func (s *APIServer) createKindAPIKey(w http.ResponseWriter, r *http.Request, kind, noun, handle string) {
	log := middleware.GetLogger(r, s.logger)
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "Create"+kind+"APIKey", correlationID)
	if !ok {
		return
	}

	var request api.APIKeyCreationRequest
	if err := s.bindRequestBody(r, &request); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	result, err := s.apiKeyService.CreateAPIKey(utils.APIKeyCreationParams{
		Kind:          kind,
		Handle:        handle,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		switch {
		case storage.IsNotFoundError(err):
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("%s '%s' not found", noun, handle)})
		case storage.IsConflictError(err):
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
		case errors.Is(err, utils.ErrAPIKeyExpirationInPast), errors.Is(err, utils.ErrUnsupportedAPIKeyExpirationUnit):
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		default:
			log.Error("Failed to create API key", slog.String("kind", kind), slog.String("handle", handle), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to create API key"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, result.Response)
}

func (s *APIServer) listKindAPIKeys(w http.ResponseWriter, r *http.Request, kind, noun, handle string) {
	log := middleware.GetLogger(r, s.logger)
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "List"+kind+"APIKeys", correlationID)
	if !ok {
		return
	}

	result, err := s.apiKeyService.ListAPIKeys(utils.ListAPIKeyParams{
		Kind:          kind,
		Handle:        handle,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("%s '%s' not found", noun, handle)})
			return
		}
		log.Error("Failed to list API keys", slog.String("kind", kind), slog.String("handle", handle), slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to list API keys"})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

func (s *APIServer) regenerateKindAPIKey(w http.ResponseWriter, r *http.Request, kind, noun, handle, apiKeyName string) {
	log := middleware.GetLogger(r, s.logger)
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "Regenerate"+kind+"APIKey", correlationID)
	if !ok {
		return
	}

	var request api.APIKeyRegenerationRequest
	if err := s.bindRequestBody(r, &request); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	result, err := s.apiKeyService.RegenerateAPIKey(utils.APIKeyRegenerationParams{
		Kind:          kind,
		Handle:        handle,
		APIKeyName:    apiKeyName,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		switch {
		case storage.IsNotFoundError(err):
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("%s or API key '%s' not found", noun, apiKeyName)})
		case errors.Is(err, utils.ErrAPIKeyExpirationInPast), errors.Is(err, utils.ErrUnsupportedAPIKeyExpirationUnit):
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		default:
			log.Error("Failed to regenerate API key", slog.String("kind", kind), slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to regenerate API key"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

func (s *APIServer) updateKindAPIKey(w http.ResponseWriter, r *http.Request, kind, noun, handle, apiKeyName string) {
	log := middleware.GetLogger(r, s.logger)
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "Update"+kind+"APIKey", correlationID)
	if !ok {
		return
	}

	var request api.APIKeyCreationRequest
	if err := s.bindRequestBody(r, &request); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}
	if request.ApiKey == nil || strings.TrimSpace(*request.ApiKey) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "apiKey is required"})
		return
	}

	result, err := s.apiKeyService.UpdateAPIKey(utils.APIKeyUpdateParams{
		Kind:          kind,
		Handle:        handle,
		APIKeyName:    apiKeyName,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		switch {
		case storage.IsOperationNotAllowedError(err):
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		case storage.IsNotFoundError(err):
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("%s or API key '%s' not found", noun, apiKeyName)})
		case storage.IsConflictError(err):
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
		case errors.Is(err, utils.ErrAPIKeyExpirationInPast), errors.Is(err, utils.ErrUnsupportedAPIKeyExpirationUnit):
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		default:
			log.Error("Failed to update API key", slog.String("kind", kind), slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to update API key"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

func (s *APIServer) revokeKindAPIKey(w http.ResponseWriter, r *http.Request, kind, noun, handle, apiKeyName string) {
	log := middleware.GetLogger(r, s.logger)
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "Revoke"+kind+"APIKey", correlationID)
	if !ok {
		return
	}

	result, err := s.apiKeyService.RevokeAPIKey(utils.APIKeyRevocationParams{
		Kind:          kind,
		Handle:        handle,
		APIKeyName:    apiKeyName,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("%s '%s' not found", noun, handle)})
			return
		}
		log.Error("Failed to revoke API key", slog.String("kind", kind), slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to revoke API key"})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}
