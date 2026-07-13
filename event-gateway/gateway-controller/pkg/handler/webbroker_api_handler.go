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

package handler

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/go-httpkit/httputil"

	eventgateway "github.com/wso2/api-platform/event-gateway/gateway-controller/pkg/api/eventgateway"
)

// CreateWebBrokerApi handles POST /webbroker-apis
func (s *WebSubServer) CreateWebBrokerApi(w http.ResponseWriter, r *http.Request) {
	log := middleware.GetLogger(r, s.logger)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, eventgateway.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	correlationID := middleware.GetCorrelationID(r)

	result, err := s.deploymentService.DeployAPIConfiguration(utils.APIDeploymentParams{
		Data:          body,
		ContentType:   r.Header.Get("Content-Type"),
		Kind:          "WebBrokerApi",
		APIID:         "",
		Origin:        models.OriginGatewayAPI,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to deploy WebBrokerApi configuration", slog.Any("error", err))
		if storage.IsConflictError(err) {
			httputil.WriteJSON(w, http.StatusConflict, eventgateway.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}
		if mapRenderError(w, "create", err) {
			return
		}
		httputil.WriteJSON(w, http.StatusBadRequest, eventgateway.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	cfg := result.StoredConfig

	httputil.WriteJSON(w, http.StatusCreated, buildResourceResponseFromStored(cfg.SourceConfiguration, cfg))

	if result.IsStale {
		return
	}

	if s.controlPlaneClient != nil && s.controlPlaneClient.IsConnected() && s.systemConfig.Controller.ControlPlane.DeploymentSyncEnabled {
		go s.waitForDeploymentAndPush(cfg.UUID, correlationID, cfg.DeployedAt, log)
	}
}

// ListWebBrokerApis handles GET /webbroker-apis
func (s *WebSubServer) ListWebBrokerApis(w http.ResponseWriter, r *http.Request, params eventgateway.ListWebBrokerApisParams) {
	configs, err := s.db.GetAllConfigsByKind(string(models.KindWebBrokerApi))
	if err != nil {
		s.logger.Error("Failed to list WebBrokerApis", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, eventgateway.ErrorResponse{
			Status:  "error",
			Message: "Failed to list WebBrokerApi configurations",
		})
		return
	}

	// TODO: Implement query parameter filtering (displayName, version, status)
	// For now, returning all WebBrokerApis without filtering
	items := make([]any, 0, len(configs))
	for _, cfg := range configs {
		items = append(items, buildResourceResponseFromStored(cfg.SourceConfiguration, cfg))
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":        "success",
		"count":         len(items),
		"webBrokerApis": items,
	})
}

// GetWebBrokerApiById handles GET /webbroker-apis/{id}
func (s *WebSubServer) GetWebBrokerApiById(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id

	cfg, err := s.db.GetConfigByKindAndHandle(models.KindWebBrokerApi, handle)
	if err != nil {
		if storage.IsDatabaseUnavailableError(err) {
			log.Error("Database unavailable", slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusServiceUnavailable, eventgateway.ErrorResponse{
				Status:  "error",
				Message: "Database is temporarily unavailable",
			})
			return
		}
		log.Warn("WebBrokerApi not found", slog.String("handle", handle))
		httputil.WriteJSON(w, http.StatusNotFound, eventgateway.ErrorResponse{
			Status:  "error",
			Message: "WebBrokerApi not found",
		})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, buildResourceResponseFromStored(cfg.SourceConfiguration, cfg))
}

// DeleteWebBrokerApiById handles DELETE /webbroker-apis/{id}
func (s *WebSubServer) DeleteWebBrokerApiById(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id

	cfg, err := s.db.GetConfigByKindAndHandle(models.KindWebBrokerApi, handle)
	if err != nil {
		if storage.IsDatabaseUnavailableError(err) {
			log.Error("Database unavailable", slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusServiceUnavailable, eventgateway.ErrorResponse{
				Status:  "error",
				Message: "Database is temporarily unavailable",
			})
			return
		}
		log.Warn("WebBrokerApi not found", slog.String("handle", handle))
		httputil.WriteJSON(w, http.StatusNotFound, eventgateway.ErrorResponse{
			Status:  "error",
			Message: "WebBrokerApi not found",
		})
		return
	}

	correlationID := middleware.GetCorrelationID(r)

	if err := s.db.DeleteConfig(cfg.UUID); err != nil {
		log.Error("Failed to delete WebBrokerApi from database", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, eventgateway.ErrorResponse{
			Status:  "error",
			Message: "Failed to delete configuration",
		})
		return
	}

	// Publish delete event for xDS propagation
	s.publishWebSubEvent(eventhub.EventTypeAPI, "DELETE", cfg.UUID, correlationID, log)

	log.Info("WebBrokerApi deleted successfully",
		slog.String("id", id),
		slog.String("correlation_id", correlationID))

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": "WebBrokerApi deleted successfully",
	})
}

// CreateWebBrokerAPIKey implements ServerInterface.CreateWebBrokerAPIKey
// (POST /webbroker-apis/{id}/api-keys)
func (s *WebSubServer) CreateWebBrokerAPIKey(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "CreateWebBrokerAPIKey", correlationID)
	if !ok {
		return
	}

	var request eventgateway.APIKeyCreationRequest
	if err := s.bindRequestBody(r, &request); err != nil {
		log.Error("Failed to parse request body for WebBroker API key creation",
			slog.Any("error", err),
			slog.String("handle", handle),
			slog.String("correlation_id", correlationID))
		httputil.WriteJSON(w, http.StatusBadRequest, eventgateway.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	params := utils.APIKeyCreationParams{
		Kind:          models.KindWebBrokerApi,
		Handle:        handle,
		Request:       toManagementAPIKeyCreationRequest(request),
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.CreateAPIKey(params)
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, eventgateway.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebBroker API '%s' not found", handle)})
		} else if storage.IsConflictError(err) {
			httputil.WriteJSON(w, http.StatusConflict, eventgateway.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to create WebBroker API key", slog.String("handle", handle), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, eventgateway.ErrorResponse{Status: "error", Message: "Failed to create API key"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, result.Response)
}

// ListWebBrokerAPIKeys implements ServerInterface.ListWebBrokerAPIKeys
// (GET /webbroker-apis/{id}/api-keys)
func (s *WebSubServer) ListWebBrokerAPIKeys(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "ListWebBrokerAPIKeys", correlationID)
	if !ok {
		return
	}

	params := utils.ListAPIKeyParams{
		Kind:          models.KindWebBrokerApi,
		Handle:        handle,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.ListAPIKeys(params)
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, eventgateway.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebBroker API '%s' not found", handle)})
		} else {
			log.Error("Failed to list WebBroker API keys", slog.String("handle", handle), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, eventgateway.ErrorResponse{Status: "error", Message: "Failed to list API keys"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

// RevokeWebBrokerAPIKey implements ServerInterface.RevokeWebBrokerAPIKey
// (DELETE /webbroker-apis/{id}/api-keys/{apiKeyName})
func (s *WebSubServer) RevokeWebBrokerAPIKey(w http.ResponseWriter, r *http.Request, id string, apiKeyName string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "RevokeWebBrokerAPIKey", correlationID)
	if !ok {
		return
	}

	params := utils.APIKeyRevocationParams{
		Kind:          models.KindWebBrokerApi,
		Handle:        handle,
		APIKeyName:    apiKeyName,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.RevokeAPIKey(params)
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, eventgateway.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebBroker API '%s' not found", handle)})
		} else {
			log.Error("Failed to revoke WebBroker API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, eventgateway.ErrorResponse{Status: "error", Message: "Failed to revoke API key"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

// UpdateWebBrokerAPIKey implements ServerInterface.UpdateWebBrokerAPIKey
// (PUT /webbroker-apis/{id}/api-keys/{apiKeyName})
func (s *WebSubServer) UpdateWebBrokerAPIKey(w http.ResponseWriter, r *http.Request, id string, apiKeyName string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "UpdateWebBrokerAPIKey", correlationID)
	if !ok {
		return
	}

	var request eventgateway.APIKeyCreationRequest
	if err := s.bindRequestBody(r, &request); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, eventgateway.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	if request.ApiKey == nil || strings.TrimSpace(*request.ApiKey) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, eventgateway.ErrorResponse{Status: "error", Message: "apiKey is required"})
		return
	}

	params := utils.APIKeyUpdateParams{
		Kind:          models.KindWebBrokerApi,
		Handle:        handle,
		APIKeyName:    apiKeyName,
		Request:       toManagementAPIKeyCreationRequest(request),
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.UpdateAPIKey(params)
	if err != nil {
		if storage.IsOperationNotAllowedError(err) {
			httputil.WriteJSON(w, http.StatusBadRequest, eventgateway.ErrorResponse{Status: "error", Message: err.Error()})
		} else if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, eventgateway.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebBroker API or API key '%s' not found", apiKeyName)})
		} else if storage.IsConflictError(err) {
			httputil.WriteJSON(w, http.StatusConflict, eventgateway.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to update WebBroker API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, eventgateway.ErrorResponse{Status: "error", Message: "Failed to update API key"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

// RegenerateWebBrokerAPIKey implements ServerInterface.RegenerateWebBrokerAPIKey
// (POST /webbroker-apis/{id}/api-keys/{apiKeyName}/regenerate)
func (s *WebSubServer) RegenerateWebBrokerAPIKey(w http.ResponseWriter, r *http.Request, id string, apiKeyName string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "RegenerateWebBrokerAPIKey", correlationID)
	if !ok {
		return
	}

	var request eventgateway.APIKeyRegenerationRequest
	if err := s.bindRequestBody(r, &request); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, eventgateway.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	params := utils.APIKeyRegenerationParams{
		Kind:          models.KindWebBrokerApi,
		Handle:        handle,
		APIKeyName:    apiKeyName,
		Request:       toManagementAPIKeyRegenerationRequest(request),
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.RegenerateAPIKey(params)
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, eventgateway.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebBroker API or API key '%s' not found", apiKeyName)})
		} else {
			log.Error("Failed to regenerate WebBroker API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, eventgateway.ErrorResponse{Status: "error", Message: "Failed to regenerate API key"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}
