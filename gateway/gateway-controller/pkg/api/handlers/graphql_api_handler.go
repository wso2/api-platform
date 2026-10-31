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
	"io"
	"log/slog"
	"net/http"
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/graphqlapi"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/httpkit/httputil"
)

// GraphQLAPI CRUD handlers. The logic lives in pkg/service/graphqlapi, so the REST
// API and the MCP tools share it; these handlers map its results to HTTP responses.

// CreateGraphQLAPI implements ServerInterface.CreateGraphQLAPI
// (POST /graphql-apis)
func (s *APIServer) CreateGraphQLAPI(w http.ResponseWriter, r *http.Request) {
	log := middleware.GetLogger(r, s.logger)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	correlationID := middleware.GetCorrelationID(r)

	stored, err := s.getGraphQLAPIService().Create(graphqlapi.CreateParams{
		Body:          body,
		ContentType:   r.Header.Get("Content-Type"),
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to deploy GraphQL API configuration", slog.Any("error", err))
		if mapRenderError(w, "create", err) {
			return
		}
		if mapValidationError(w, err) {
			return
		}
		if storage.IsConflictError(err) {
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}
		if isGraphQLAPICreateBadRequest(err) {
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to create configuration",
		})
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, buildResourceResponseFromStored(stored.SourceConfiguration, stored))
}

// ListGraphQLAPIs implements ServerInterface.ListGraphQLAPIs
// (GET /graphql-apis)
func (s *APIServer) ListGraphQLAPIs(w http.ResponseWriter, r *http.Request, params api.ListGraphQLAPIsParams) {
	if (params.DisplayName != nil && *params.DisplayName != "") || (params.Version != nil && *params.Version != "") || (params.Context != nil && *params.Context != "") || (params.Status != nil && *params.Status != "") {
		s.SearchDeployments(w, r, string(api.GraphQLAPIKindGraphQLApi))
		return
	}
	configs, err := s.getGraphQLAPIService().List()
	if err != nil {
		s.logger.Error("Failed to get GraphQL APIs", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to retrieve GraphQL API configurations",
		})
		return
	}

	items := make([]any, 0, len(configs))
	for _, cfg := range configs {
		items = append(items, buildResourceResponseFromStored(cfg.SourceConfiguration, cfg))
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":      "success",
		"count":       len(items),
		"graphqlApis": items,
	})
}

// GetGraphQLAPIById implements ServerInterface.GetGraphQLAPIById
// (GET /graphql-apis/{id})
func (s *APIServer) GetGraphQLAPIById(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)

	cfg, err := s.getGraphQLAPIService().GetByHandle(id)
	if err != nil {
		writeGraphQLAPILoadError(w, log, id, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, buildResourceResponseFromStored(cfg.SourceConfiguration, cfg))
}

// UpdateGraphQLAPI implements ServerInterface.UpdateGraphQLAPI
// (PUT /graphql-apis/{id})
func (s *APIServer) UpdateGraphQLAPI(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	correlationID := middleware.GetCorrelationID(r)

	stored, err := s.getGraphQLAPIService().Update(graphqlapi.UpdateParams{
		Handle:        id,
		Body:          body,
		ContentType:   r.Header.Get("Content-Type"),
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		var parseErr *graphqlapi.ParseError
		var mismatchErr *graphqlapi.HandleMismatchError
		switch {
		case isGraphQLAPILookupError(err):
			writeGraphQLAPILoadError(w, log, id, err)
		case errors.As(err, &parseErr):
			log.Error("Failed to parse GraphQL API configuration", slog.Any("error", parseErr.Cause))
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("failed to parse configuration: %v", parseErr.Cause),
			})
		case errors.As(err, &mismatchErr):
			log.Warn("GraphQL API update handle mismatch", slog.String("pathHandle", id), slog.String("bodyHandle", mismatchErr.YAMLHandle))
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("metadata.name '%s' does not match path id '%s'", mismatchErr.YAMLHandle, id),
			})
		default:
			writeGraphQLAPIUpdateError(w, log, err)
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, buildResourceResponseFromStored(stored.SourceConfiguration, stored))
}

// DeleteGraphQLAPI implements ServerInterface.DeleteGraphQLAPI
// (DELETE /graphql-apis/{id})
func (s *APIServer) DeleteGraphQLAPI(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)
	correlationID := middleware.GetCorrelationID(r)

	if err := s.getGraphQLAPIService().Delete(graphqlapi.DeleteParams{
		Handle:        id,
		CorrelationID: correlationID,
		Logger:        log,
	}); err != nil {
		var deleteErr *graphqlapi.DeleteError
		if errors.As(err, &deleteErr) {
			log.Error("Failed to delete GraphQL API config from database", slog.Any("error", deleteErr.Cause))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Failed to delete configuration",
			})
			return
		}
		writeGraphQLAPILoadError(w, log, id, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": "GraphQLApi deleted successfully",
		"id":      id,
	})
}

// isGraphQLAPILookupError reports whether err came from looking up a GraphQL API by handle.
func isGraphQLAPILookupError(err error) bool {
	var loadErr *graphqlapi.LoadError
	return errors.Is(err, graphqlapi.ErrNotFound) || errors.As(err, &loadErr)
}

// writeGraphQLAPILoadError writes the response for a failed GraphQL API lookup: 404 when it does not
// exist; otherwise it logs the storage error and returns 500.
func writeGraphQLAPILoadError(w http.ResponseWriter, log *slog.Logger, handle string, err error) {
	if errors.Is(err, graphqlapi.ErrNotFound) {
		log.Warn("GraphQL API configuration not found", slog.String("handle", handle))
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("GraphQLApi with handle '%s' not found", handle),
		})
		return
	}
	cause := err
	var loadErr *graphqlapi.LoadError
	if errors.As(err, &loadErr) {
		cause = loadErr.Cause
	}
	log.Error("Failed to get GraphQL API configuration", slog.Any("error", cause), slog.String("handle", handle))
	httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
		Status:  "error",
		Message: "Failed to retrieve configuration",
	})
}

// writeGraphQLAPIUpdateError writes the response for a failed GraphQL API redeploy.
func writeGraphQLAPIUpdateError(w http.ResponseWriter, log *slog.Logger, err error) {
	log.Error("Failed to update GraphQL API configuration", slog.Any("error", err))
	if mapRenderError(w, "update", err) {
		return
	}
	if mapValidationError(w, err) {
		return
	}
	if storage.IsConflictError(err) {
		httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}
	httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
		Status:  "error",
		Message: "Failed to update configuration",
	})
}

// CreateGraphQLAPIKey implements ServerInterface.CreateGraphQLAPIKey
// (POST /graphql-apis/{id}/api-keys)
func (s *APIServer) CreateGraphQLAPIKey(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "CreateGraphQLAPIKey", correlationID)
	if !ok {
		return
	}

	var request api.APIKeyCreationRequest
	if err := s.bindRequestBody(r, &request); err != nil {
		log.Error("Failed to parse request body for GraphQL API key creation",
			slog.Any("error", err),
			slog.String("handle", handle),
			slog.String("correlation_id", correlationID))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	params := utils.APIKeyCreationParams{
		Kind:          models.KindGraphQLApi,
		Handle:        handle,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.CreateAPIKey(params)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else if storage.IsConflictError(err) || strings.Contains(err.Error(), "already exists") {
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else if errors.Is(err, utils.ErrAPIKeyExpirationInPast) || errors.Is(err, utils.ErrUnsupportedAPIKeyExpirationUnit) {
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to create GraphQL API key", slog.String("handle", handle), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to create API key"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, result.Response)
}

// RevokeGraphQLAPIKey implements ServerInterface.RevokeGraphQLAPIKey
// (DELETE /graphql-apis/{id}/api-keys/{apiKeyName})
func (s *APIServer) RevokeGraphQLAPIKey(w http.ResponseWriter, r *http.Request, id string, apiKeyName string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "RevokeGraphQLAPIKey", correlationID)
	if !ok {
		return
	}

	params := utils.APIKeyRevocationParams{
		Kind:          models.KindGraphQLApi,
		Handle:        handle,
		APIKeyName:    apiKeyName,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.RevokeAPIKey(params)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to revoke GraphQL API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to revoke API key"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

// UpdateGraphQLAPIKey implements ServerInterface.UpdateGraphQLAPIKey
// (PUT /graphql-apis/{id}/api-keys/{apiKeyName})
func (s *APIServer) UpdateGraphQLAPIKey(w http.ResponseWriter, r *http.Request, id string, apiKeyName string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "UpdateGraphQLAPIKey", correlationID)
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

	params := utils.APIKeyUpdateParams{
		Kind:          models.KindGraphQLApi,
		Handle:        handle,
		APIKeyName:    apiKeyName,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.UpdateAPIKey(params)
	if err != nil {
		if storage.IsOperationNotAllowedError(err) {
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else if strings.Contains(err.Error(), "not found") {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else if storage.IsConflictError(err) || strings.Contains(err.Error(), "already exists") {
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else if errors.Is(err, utils.ErrAPIKeyExpirationInPast) || errors.Is(err, utils.ErrUnsupportedAPIKeyExpirationUnit) {
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to update GraphQL API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to update API key"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

// RegenerateGraphQLAPIKey implements ServerInterface.RegenerateGraphQLAPIKey
// (POST /graphql-apis/{id}/api-keys/{apiKeyName}/regenerate)
func (s *APIServer) RegenerateGraphQLAPIKey(w http.ResponseWriter, r *http.Request, id string, apiKeyName string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "RegenerateGraphQLAPIKey", correlationID)
	if !ok {
		return
	}

	var request api.APIKeyRegenerationRequest
	if err := s.bindRequestBody(r, &request); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	params := utils.APIKeyRegenerationParams{
		Kind:          models.KindGraphQLApi,
		Handle:        handle,
		APIKeyName:    apiKeyName,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.RegenerateAPIKey(params)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else if errors.Is(err, utils.ErrAPIKeyExpirationInPast) || errors.Is(err, utils.ErrUnsupportedAPIKeyExpirationUnit) {
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to regenerate GraphQL API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to regenerate API key"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

// ListGraphQLAPIKeys implements ServerInterface.ListGraphQLAPIKeys
// (GET /graphql-apis/{id}/api-keys)
func (s *APIServer) ListGraphQLAPIKeys(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "ListGraphQLAPIKeys", correlationID)
	if !ok {
		return
	}

	params := utils.ListAPIKeyParams{
		Kind:          models.KindGraphQLApi,
		Handle:        handle,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.ListAPIKeys(params)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to list GraphQL API keys", slog.String("handle", handle), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to list API keys"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

// mapValidationError maps a *utils.ValidationErrorListError to a 400 response with
// structured field errors, mirroring RestAPIHandler.mapCreateError's handling of the
// same error type.
// isGraphQLAPICreateBadRequest reports whether err is a client-input failure from
// the shared DeployAPIConfiguration path (config-parse failure, kind mismatch, or
// missing origin) that CreateGraphQLAPI's other error mappers (mapRenderError,
// mapValidationError, storage.IsConflictError) don't recognize — without this,
// these fell through to a generic 500 instead of the 400 the caller actually made.
// Mirrors isRestAPICreateBadRequest (rest_api_handler.go); DeployAPIConfiguration
// is the same shared function for every kind, so most error message classes are
// identical. One is GraphQL-specific: this endpoint hardcodes params.Kind =
// "GraphQLApi" so the body is always parsed as a GraphQLAPI regardless of what its
// own kind field says, but DeployAPIConfiguration's validator lookup keys off that
// parsed kind field, not the resolved one — a body with a wrong kind value parses
// fine, then fails validator lookup with "unexpected configuration type" instead
// of a recognized validation error.
func isGraphQLAPICreateBadRequest(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "failed to parse configuration") ||
		strings.Contains(message, "resource kind is required") ||
		strings.Contains(message, "unsupported resource kind") ||
		strings.Contains(message, "invalid or missing origin") ||
		strings.Contains(message, "unexpected configuration type")
}

func mapValidationError(w http.ResponseWriter, err error) bool {
	var validationErr *utils.ValidationErrorListError
	if !errors.As(err, &validationErr) {
		return false
	}
	apiErrors := make([]api.ValidationError, len(validationErr.Errors))
	for i, e := range validationErr.Errors {
		apiErrors[i] = api.ValidationError{
			Field:   stringPtr(e.Field),
			Message: stringPtr(e.Message),
		}
	}
	httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
		Status:  "error",
		Message: "Configuration validation failed",
		Errors:  &apiErrors,
	})
	return true
}
