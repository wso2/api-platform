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
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/wso2/go-httpkit/httputil"
	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// CreateWebSubAPI implements ServerInterface.CreateWebSubAPI
// (POST /websub-apis)
func (s *APIServer) CreateWebSubAPI(w http.ResponseWriter, r *http.Request) {
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

	result, err := s.deploymentService.DeployAPIConfiguration(utils.APIDeploymentParams{
		Data:          body,
		ContentType:   r.Header.Get("Content-Type"),
		Kind:          "WebSubApi",
		APIID:         "",
		Origin:        models.OriginGatewayAPI,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to deploy WebSub API configuration", slog.Any("error", err))
		if storage.IsConflictError(err) {
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}
		if mapRenderError(w, "create", err) {
			return
		}
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
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
		go s.waitForDeploymentAndPush(cfg.UUID, correlationID, log)
	}
}

// ListWebSubAPIs implements ServerInterface.ListWebSubAPIs
// (GET /websub-apis)
func (s *APIServer) ListWebSubAPIs(w http.ResponseWriter, r *http.Request, params api.ListWebSubAPIsParams) {
	if (params.DisplayName != nil && *params.DisplayName != "") ||
		(params.Version != nil && *params.Version != "") ||
		(params.Context != nil && *params.Context != "") ||
		(params.Status != nil && *params.Status != "") {
		s.SearchDeployments(w, r, string(api.WebSubAPIKindWebSubApi))
		return
	}

	configs, err := s.db.GetAllConfigsByKind(string(api.WebSubAPIKindWebSubApi))
	if err != nil {
		s.logger.Error("Failed to list WebSub APIs", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to list WebSub API configurations",
		})
		return
	}

	contextFilter := params.Context != nil && *params.Context != ""
	items := make([]any, 0, len(configs))
	for _, cfg := range configs {
		_, err := cfg.GetContext()
		if err != nil {
			s.logger.Warn("Failed to get context for WebSub API config",
				slog.String("id", cfg.UUID),
				slog.String("displayName", cfg.DisplayName),
				slog.Any("error", err))
			if contextFilter {
				continue
			}
		}

		items = append(items, buildResourceResponseFromStored(cfg.SourceConfiguration, cfg))
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":     "success",
		"count":      len(items),
		"websubApis": items,
	})
}

// GetWebSubAPIById implements ServerInterface.GetWebSubAPIById
// (GET /websub-apis/{id})
func (s *APIServer) GetWebSubAPIById(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id

	cfg, err := s.db.GetConfigByKindAndHandle(models.KindWebSubApi, handle)
	if err != nil {
		if storage.IsDatabaseUnavailableError(err) {
			httputil.WriteJSON(w, http.StatusServiceUnavailable, api.ErrorResponse{
				Status:  "error",
				Message: "Database storage not available",
			})
			return
		}
		log.Warn("WebSub API configuration not found",
			slog.String("handle", handle))
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("WebSub API configuration with handle '%s' not found", handle),
		})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, buildResourceResponseFromStored(cfg.SourceConfiguration, cfg))
}

// UpdateWebSubAPI implements ServerInterface.UpdateWebSubAPI
// (PUT /websub-apis/{id})
func (s *APIServer) UpdateWebSubAPI(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	existing, err := s.db.GetConfigByKindAndHandle(models.KindWebSubApi, handle)
	if err != nil {
		log.Warn("WebSub API configuration not found",
			slog.String("handle", handle))
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("WebSub API configuration with handle '%s' not found", handle),
		})
		return
	}

	correlationID := middleware.GetCorrelationID(r)

	result, err := s.deploymentService.DeployAPIConfiguration(utils.APIDeploymentParams{
		Data:          body,
		ContentType:   r.Header.Get("Content-Type"),
		Kind:          "WebSubApi",
		APIID:         existing.UUID,
		Origin:        models.OriginGatewayAPI,
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to update WebSub API configuration", slog.Any("error", err))
		if storage.IsConflictError(err) {
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}
		if mapRenderError(w, "update", err) {
			return
		}
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	updated := result.StoredConfig

	log.Info("WebSub API configuration updated",
		slog.String("id", updated.UUID),
		slog.String("handle", handle))

	httputil.WriteJSON(w, http.StatusOK, buildResourceResponseFromStored(updated.SourceConfiguration, updated))
}

// DeleteWebSubAPI implements ServerInterface.DeleteWebSubAPI
// (DELETE /websub-apis/{id})
func (s *APIServer) DeleteWebSubAPI(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	cfg, err := s.db.GetConfigByKindAndHandle(models.KindWebSubApi, handle)
	if err != nil {
		log.Warn("WebSub API configuration not found",
			slog.String("handle", handle))
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("WebSub API configuration with handle '%s' not found", handle),
		})
		return
	}

	if err := s.db.DeleteConfig(cfg.UUID); err != nil {
		log.Error("Failed to delete WebSub API config from database", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to delete configuration",
		})
		return
	}

	if err := s.db.RemoveAPIKeysAPI(cfg.UUID); err != nil {
		log.Warn("Failed to remove API keys from database",
			slog.String("handle", handle),
			slog.Any("error", err))
	}

	// Deregister WebSub topics
	topicsToUnregister := s.deploymentService.GetTopicsForDelete(*cfg)
	for _, topic := range topicsToUnregister {
		ctx, cancel := context.WithTimeout(context.Background(),
			time.Duration(s.routerConfig.EventGateway.TimeoutSeconds)*time.Second)
		if err := s.deploymentService.UnregisterTopicWithHub(ctx, s.httpClient, topic,
			s.routerConfig.EventGateway.RouterHost, s.routerConfig.EventGateway.WebSubHubListenerPort, log); err != nil {
			log.Error("Failed to deregister topic from WebSubHub",
				slog.Any("error", err),
				slog.String("topic", topic))
		} else {
			log.Info("Successfully deregistered topic from WebSubHub",
				slog.String("topic", topic))
		}
		cancel()
	}

	s.publishWebSubEvent(eventhub.EventTypeAPI, "DELETE", cfg.UUID, correlationID, log)

	log.Info("WebSub API configuration deleted",
		slog.String("id", cfg.UUID),
		slog.String("handle", handle))

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": "WebSub API configuration deleted successfully",
		"id":      handle,
	})
}

// CreateWebSubAPIKey implements ServerInterface.CreateWebSubAPIKey
// (POST /websub-apis/{id}/api-keys)
func (s *APIServer) CreateWebSubAPIKey(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "CreateWebSubAPIKey", correlationID)
	if !ok {
		return
	}

	var request api.APIKeyCreationRequest
	if err := s.bindRequestBody(r, &request); err != nil {
		log.Error("Failed to parse request body for WebSub API key creation",
			slog.Any("error", err),
			slog.String("handle", handle),
			slog.String("correlation_id", correlationID))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	params := utils.APIKeyCreationParams{
		Kind:          models.KindWebSubApi,
		Handle:        handle,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.CreateAPIKey(params)
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API '%s' not found", handle)})
		} else if storage.IsConflictError(err) {
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to create WebSub API key", slog.String("handle", handle), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to create API key"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, result.Response)
}

// ListWebSubAPIKeys implements ServerInterface.ListWebSubAPIKeys
// (GET /websub-apis/{id}/api-keys)
func (s *APIServer) ListWebSubAPIKeys(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "ListWebSubAPIKeys", correlationID)
	if !ok {
		return
	}

	params := utils.ListAPIKeyParams{
		Kind:          models.KindWebSubApi,
		Handle:        handle,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.ListAPIKeys(params)
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API '%s' not found", handle)})
		} else {
			log.Error("Failed to list WebSub API keys", slog.String("handle", handle), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to list API keys"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

// RevokeWebSubAPIKey implements ServerInterface.RevokeWebSubAPIKey
// (DELETE /websub-apis/{id}/api-keys/{apiKeyName})
func (s *APIServer) RevokeWebSubAPIKey(w http.ResponseWriter, r *http.Request, id string, apiKeyName string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "RevokeWebSubAPIKey", correlationID)
	if !ok {
		return
	}

	params := utils.APIKeyRevocationParams{
		Kind:          models.KindWebSubApi,
		Handle:        handle,
		APIKeyName:    apiKeyName,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.RevokeAPIKey(params)
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API '%s' not found", handle)})
		} else {
			log.Error("Failed to revoke WebSub API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to revoke API key"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

// UpdateWebSubAPIKey implements ServerInterface.UpdateWebSubAPIKey
// (PUT /websub-apis/{id}/api-keys/{apiKeyName})
func (s *APIServer) UpdateWebSubAPIKey(w http.ResponseWriter, r *http.Request, id string, apiKeyName string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "UpdateWebSubAPIKey", correlationID)
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
		Kind:          models.KindWebSubApi,
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
		} else if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API or API key '%s' not found", apiKeyName)})
		} else if storage.IsConflictError(err) {
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
		} else {
			log.Error("Failed to update WebSub API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to update API key"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

// RegenerateWebSubAPIKey implements ServerInterface.RegenerateWebSubAPIKey
// (POST /websub-apis/{id}/api-keys/{apiKeyName}/regenerate)
func (s *APIServer) RegenerateWebSubAPIKey(w http.ResponseWriter, r *http.Request, id string, apiKeyName string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "RegenerateWebSubAPIKey", correlationID)
	if !ok {
		return
	}

	var request api.APIKeyRegenerationRequest
	if err := s.bindRequestBody(r, &request); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	params := utils.APIKeyRegenerationParams{
		Kind:          models.KindWebSubApi,
		Handle:        handle,
		APIKeyName:    apiKeyName,
		Request:       request,
		User:          user,
		CorrelationID: correlationID,
		Logger:        log,
	}

	result, err := s.apiKeyService.RegenerateAPIKey(params)
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API or API key '%s' not found", apiKeyName)})
		} else {
			log.Error("Failed to regenerate WebSub API key", slog.String("handle", handle), slog.String("key", apiKeyName), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to regenerate API key"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

// ============================================================
// Webhook Secret handlers
// ============================================================

// CreateWebSubAPISecret implements ServerInterface.CreateWebSubAPISecret
// (POST /websub-apis/{id}/secrets)
func (s *APIServer) CreateWebSubAPISecret(w http.ResponseWriter, r *http.Request, handle string) {
	log := middleware.GetLogger(r, s.logger)
	correlationID := middleware.GetCorrelationID(r)

	cfg, err := s.db.GetConfigByKindAndHandle(models.KindWebSubApi, handle)
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API '%s' not found", handle)})
			return
		}
		if storage.IsDatabaseUnavailableError(err) {
			httputil.WriteJSON(w, http.StatusServiceUnavailable, api.ErrorResponse{Status: "error", Message: "Storage unavailable"})
			return
		}
		log.Error("Failed to look up WebSub API", slog.String("handle", handle), slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Internal error looking up WebSub API"})
		return
	}
	if cfg == nil {
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API '%s' not found", handle)})
		return
	}

	var request api.WebhookSecretCreationRequest
	if err := s.bindRequestBody(r, &request); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %s", err.Error())})
		return
	}

	ws, plaintext, err := s.webhookSecretService.Generate(cfg.UUID, request.DisplayName, correlationID)
	if err != nil {
		if storage.IsConflictError(err) {
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
			return
		}
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: err.Error()})
			return
		}
		log.Error("Failed to generate webhook secret", slog.String("handle", handle), slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to generate webhook secret"})
		return
	}

	secretStatus := api.WebhookSecretInfoStatus(ws.Status)
	httputil.WriteJSON(w, http.StatusCreated, api.WebhookSecretCreationResponse{
		Status:  "success",
		Message: "Webhook secret generated successfully",
		Secret:  plaintext,
		WebhookSecret: &api.WebhookSecretInfo{
			Name:        &ws.Name,
			DisplayName: &ws.DisplayName,
			Status:      &secretStatus,
			CreatedAt:   &ws.CreatedAt,
			UpdatedAt:   &ws.UpdatedAt,
		},
	})
}

// ListWebSubAPISecrets implements ServerInterface.ListWebSubAPISecrets
// (GET /websub-apis/{id}/secrets)
func (s *APIServer) ListWebSubAPISecrets(w http.ResponseWriter, r *http.Request, handle string) {
	log := middleware.GetLogger(r, s.logger)

	cfg, err := s.db.GetConfigByKindAndHandle(models.KindWebSubApi, handle)
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API '%s' not found", handle)})
			return
		}
		if storage.IsDatabaseUnavailableError(err) {
			httputil.WriteJSON(w, http.StatusServiceUnavailable, api.ErrorResponse{Status: "error", Message: "Storage unavailable"})
			return
		}
		log.Error("Failed to look up WebSub API", slog.String("handle", handle), slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Internal error looking up WebSub API"})
		return
	}
	if cfg == nil {
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API '%s' not found", handle)})
		return
	}

	wsList, err := s.webhookSecretService.List(cfg.UUID)
	if err != nil {
		log.Error("Failed to list webhook secrets", slog.String("handle", handle), slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to list webhook secrets"})
		return
	}

	items := make([]api.WebhookSecretInfo, 0, len(wsList))
	for _, ws := range wsList {
		secretStatus := api.WebhookSecretInfoStatus(ws.Status)
		items = append(items, api.WebhookSecretInfo{
			Name:        &ws.Name,
			DisplayName: &ws.DisplayName,
			Status:      &secretStatus,
			CreatedAt:   &ws.CreatedAt,
			UpdatedAt:   &ws.UpdatedAt,
		})
	}

	total := len(items)
	status := "success"
	httputil.WriteJSON(w, http.StatusOK, api.WebhookSecretListResponse{
		Status:     &status,
		TotalCount: &total,
		Secrets:    &items,
	})
}

// RegenerateWebSubAPISecret implements ServerInterface.RegenerateWebSubAPISecret
// (POST /websub-apis/{id}/secrets/{secretName}/regenerate)
func (s *APIServer) RegenerateWebSubAPISecret(w http.ResponseWriter, r *http.Request, handle string, secretName string) {
	log := middleware.GetLogger(r, s.logger)
	correlationID := middleware.GetCorrelationID(r)

	cfg, err := s.db.GetConfigByKindAndHandle(models.KindWebSubApi, handle)
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API '%s' not found", handle)})
			return
		}
		if storage.IsDatabaseUnavailableError(err) {
			httputil.WriteJSON(w, http.StatusServiceUnavailable, api.ErrorResponse{Status: "error", Message: "Storage unavailable"})
			return
		}
		log.Error("Failed to look up WebSub API", slog.String("handle", handle), slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Internal error looking up WebSub API"})
		return
	}
	if cfg == nil {
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API '%s' not found", handle)})
		return
	}

	ws, plaintext, err := s.webhookSecretService.Regenerate(cfg.UUID, secretName, correlationID)
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Webhook secret '%s' not found", secretName)})
			return
		}
		log.Error("Failed to regenerate webhook secret", slog.String("handle", handle), slog.String("secret", secretName), slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to regenerate webhook secret"})
		return
	}

	secretStatus := api.WebhookSecretInfoStatus(ws.Status)
	httputil.WriteJSON(w, http.StatusOK, api.WebhookSecretCreationResponse{
		Status:  "success",
		Message: "Webhook secret regenerated successfully",
		Secret:  plaintext,
		WebhookSecret: &api.WebhookSecretInfo{
			Name:        &ws.Name,
			DisplayName: &ws.DisplayName,
			Status:      &secretStatus,
			CreatedAt:   &ws.CreatedAt,
			UpdatedAt:   &ws.UpdatedAt,
		},
	})
}

// DeleteWebSubAPISecret implements ServerInterface.DeleteWebSubAPISecret
// (DELETE /websub-apis/{id}/secrets/{secretName})
func (s *APIServer) DeleteWebSubAPISecret(w http.ResponseWriter, r *http.Request, handle string, secretName string) {
	log := middleware.GetLogger(r, s.logger)
	correlationID := middleware.GetCorrelationID(r)

	cfg, err := s.db.GetConfigByKindAndHandle(models.KindWebSubApi, handle)
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API '%s' not found", handle)})
			return
		}
		if storage.IsDatabaseUnavailableError(err) {
			httputil.WriteJSON(w, http.StatusServiceUnavailable, api.ErrorResponse{Status: "error", Message: "Storage unavailable"})
			return
		}
		log.Error("Failed to look up WebSub API", slog.String("handle", handle), slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Internal error looking up WebSub API"})
		return
	}
	if cfg == nil {
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("WebSub API '%s' not found", handle)})
		return
	}

	if err := s.webhookSecretService.Delete(cfg.UUID, secretName, correlationID); err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Webhook secret '%s' not found", secretName)})
			return
		}
		log.Error("Failed to delete webhook secret", slog.String("handle", handle), slog.String("secret", secretName), slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to delete webhook secret"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
