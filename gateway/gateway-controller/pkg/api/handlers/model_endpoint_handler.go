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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/httpkit/httputil"
)

// ListModelEndpoints implements ServerInterface.ListModelEndpoints
// (GET /model-endpoints)
func (s *APIServer) ListModelEndpoints(w http.ResponseWriter, r *http.Request, params api.ListModelEndpointsParams) {
	log := middleware.GetLogger(r, s.logger)
	configs := s.llmDeploymentService.ListModelEndpoints(params)

	items := make([]any, 0, len(configs))
	for _, cfg := range configs {
		endpoint, err := rematerializeModelEndpointConfig(log, cfg.UUID, cfg.SourceConfiguration)
		if err != nil {
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
				Status: "error", Message: "Failed to get stored model endpoint configuration"})
			return
		}
		items = append(items, buildResourceResponseFromStored(endpoint, cfg))
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"status": "success", "count": len(items), "modelEndpoints": items})
}

// CreateModelEndpoint implements ServerInterface.CreateModelEndpoint
// (POST /model-endpoints)
func (s *APIServer) CreateModelEndpoint(w http.ResponseWriter, r *http.Request) {
	log := middleware.GetLogger(r, s.logger)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Failed to read request body"})
		return
	}

	result, err := s.llmDeploymentService.CreateModelEndpoint(utils.LLMDeploymentParams{
		Data:          body,
		ContentType:   r.Header.Get("Content-Type"),
		CorrelationID: middleware.GetCorrelationID(r),
		Logger:        log,
		Origin:        models.OriginGatewayAPI,
	})
	if err != nil {
		writeModelEndpointWriteError(w, log, "create", err)
		return
	}

	stored := result.StoredConfig
	log.Info("Model endpoint created", slog.String("uuid", stored.UUID), slog.String("handle", stored.Handle))

	endpoint, err := rematerializeModelEndpointConfig(log, stored.UUID, stored.SourceConfiguration)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status: "error", Message: "Failed to get stored model endpoint configuration"})
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, buildResourceResponseFromStored(endpoint, stored))
}

// GetModelEndpointById implements ServerInterface.GetModelEndpointById
// (GET /model-endpoints/{id})
func (s *APIServer) GetModelEndpointById(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)

	cfg, err := s.llmDeploymentService.GetModelEndpointByHandle(id)
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
				Status: "error", Message: fmt.Sprintf("Model endpoint with handle '%s' not found", id)})
			return
		}
		log.Error("Failed to look up model endpoint", slog.String("handle", id), slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to look up model endpoint"})
		return
	}

	endpoint, err := rematerializeModelEndpointConfig(log, cfg.UUID, cfg.SourceConfiguration)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status: "error", Message: "Failed to get stored model endpoint configuration"})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, buildResourceResponseFromStored(endpoint, cfg))
}

// UpdateModelEndpoint implements ServerInterface.UpdateModelEndpoint
// (PUT /model-endpoints/{id})
func (s *APIServer) UpdateModelEndpoint(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Failed to read request body"})
		return
	}

	result, err := s.llmDeploymentService.UpdateModelEndpoint(id, utils.LLMDeploymentParams{
		Data:          body,
		ContentType:   r.Header.Get("Content-Type"),
		CorrelationID: middleware.GetCorrelationID(r),
		Logger:        log,
		Origin:        models.OriginGatewayAPI,
	})
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
				Status: "error", Message: fmt.Sprintf("Model endpoint with handle '%s' not found", id)})
			return
		}
		writeModelEndpointWriteError(w, log, "update", err)
		return
	}

	updated := result.StoredConfig
	endpoint, err := rematerializeModelEndpointConfig(log, updated.UUID, updated.SourceConfiguration)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status: "error", Message: "Failed to get stored model endpoint configuration"})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, buildResourceResponseFromStored(endpoint, updated))
}

// DeleteModelEndpoint implements ServerInterface.DeleteModelEndpoint
// (DELETE /model-endpoints/{id})
func (s *APIServer) DeleteModelEndpoint(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)

	cfg, err := s.llmDeploymentService.DeleteModelEndpoint(id, middleware.GetCorrelationID(r), log)
	if err != nil {
		switch {
		case storage.IsNotFoundError(err):
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
				Status: "error", Message: fmt.Sprintf("Model endpoint with handle '%s' not found", id)})
		case storage.IsConflictError(err):
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
		default:
			log.Error("Failed to delete model endpoint", slog.String("handle", id), slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to delete model endpoint"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": "Model endpoint deleted successfully",
		"id":      cfg.Handle,
	})
}

// writeModelEndpointWriteError maps a model endpoint create or update failure to
// its response.
func writeModelEndpointWriteError(w http.ResponseWriter, log *slog.Logger, operation string, err error) {
	if mapRenderError(w, operation, err) {
		return
	}
	switch {
	case utils.IsPolicyDefinitionMissingError(err):
		log.Error("Model endpoint "+operation+" failed - policy definition missing", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status: "error", Message: utils.PolicyDefinitionMissingUserMessage})
	case errors.Is(err, utils.ErrModelEndpointValidation):
		log.Warn("Model endpoint configuration invalid", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
	case errors.Is(err, utils.ErrModelEndpointReserved), storage.IsConflictError(err):
		httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
	default:
		log.Error("Failed to "+operation+" model endpoint", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status: "error", Message: "Failed to " + operation + " model endpoint"})
	}
}

// rematerializeModelEndpointConfig re-encodes a stored model endpoint into a
// response-bound copy. The stored SourceConfiguration is never modified. Its
// policy parameters are returned as authored: secrets are referenced through
// template expressions, which are stored unrendered.
func rematerializeModelEndpointConfig(log *slog.Logger, id string, source any) (api.ModelEndpointConfiguration, error) {
	j, err := json.Marshal(source)
	if err != nil {
		log.Error("Failed to marshal stored model endpoint", slog.String("id", id), slog.Any("error", err))
		return api.ModelEndpointConfiguration{}, fmt.Errorf("marshal model endpoint: %w", err)
	}
	var endpoint api.ModelEndpointConfiguration
	if err := json.Unmarshal(j, &endpoint); err != nil {
		log.Error("Failed to unmarshal stored model endpoint", slog.String("id", id), slog.Any("error", err))
		return api.ModelEndpointConfiguration{}, fmt.Errorf("unmarshal model endpoint: %w", err)
	}
	return endpoint, nil
}

// ─── API keys ────────────────────────────────────────────────────────────────

// CreateModelEndpointAPIKey implements ServerInterface.CreateModelEndpointAPIKey
// (POST /model-endpoints/{id}/api-keys). An endpoint key reaches every Model the
// endpoint serves, and its catalog.
func (s *APIServer) CreateModelEndpointAPIKey(w http.ResponseWriter, r *http.Request, id string) {
	s.createKindAPIKey(w, r, models.KindModelEndpoint, "Model endpoint", id)
}

// ListModelEndpointAPIKeys implements ServerInterface.ListModelEndpointAPIKeys
// (GET /model-endpoints/{id}/api-keys)
func (s *APIServer) ListModelEndpointAPIKeys(w http.ResponseWriter, r *http.Request, id string) {
	s.listKindAPIKeys(w, r, models.KindModelEndpoint, "Model endpoint", id)
}

// RegenerateModelEndpointAPIKey implements ServerInterface.RegenerateModelEndpointAPIKey
// (POST /model-endpoints/{id}/api-keys/{apiKeyName}/regenerate)
func (s *APIServer) RegenerateModelEndpointAPIKey(w http.ResponseWriter, r *http.Request, id string, apiKeyName string) {
	s.regenerateKindAPIKey(w, r, models.KindModelEndpoint, "Model endpoint", id, apiKeyName)
}

// UpdateModelEndpointAPIKey implements ServerInterface.UpdateModelEndpointAPIKey
// (PUT /model-endpoints/{id}/api-keys/{apiKeyName})
func (s *APIServer) UpdateModelEndpointAPIKey(w http.ResponseWriter, r *http.Request, id string, apiKeyName string) {
	s.updateKindAPIKey(w, r, models.KindModelEndpoint, "Model endpoint", id, apiKeyName)
}

// RevokeModelEndpointAPIKey implements ServerInterface.RevokeModelEndpointAPIKey
// (DELETE /model-endpoints/{id}/api-keys/{apiKeyName})
func (s *APIServer) RevokeModelEndpointAPIKey(w http.ResponseWriter, r *http.Request, id string, apiKeyName string) {
	s.revokeKindAPIKey(w, r, models.KindModelEndpoint, "Model endpoint", id, apiKeyName)
}

// CreateModelAPIKey implements ServerInterface.CreateModelAPIKey
// (POST /models/{id}/api-keys). A Model key reaches that Model on every endpoint
// serving it, and a catalog listing only it.
func (s *APIServer) CreateModelAPIKey(w http.ResponseWriter, r *http.Request, id string) {
	s.createKindAPIKey(w, r, models.KindModel, "Model", id)
}

// ListModelAPIKeys implements ServerInterface.ListModelAPIKeys
// (GET /models/{id}/api-keys)
func (s *APIServer) ListModelAPIKeys(w http.ResponseWriter, r *http.Request, id string) {
	s.listKindAPIKeys(w, r, models.KindModel, "Model", id)
}

// RegenerateModelAPIKey implements ServerInterface.RegenerateModelAPIKey
// (POST /models/{id}/api-keys/{apiKeyName}/regenerate)
func (s *APIServer) RegenerateModelAPIKey(w http.ResponseWriter, r *http.Request, id string, apiKeyName string) {
	s.regenerateKindAPIKey(w, r, models.KindModel, "Model", id, apiKeyName)
}

// UpdateModelAPIKey implements ServerInterface.UpdateModelAPIKey
// (PUT /models/{id}/api-keys/{apiKeyName})
func (s *APIServer) UpdateModelAPIKey(w http.ResponseWriter, r *http.Request, id string, apiKeyName string) {
	s.updateKindAPIKey(w, r, models.KindModel, "Model", id, apiKeyName)
}

// RevokeModelAPIKey implements ServerInterface.RevokeModelAPIKey
// (DELETE /models/{id}/api-keys/{apiKeyName})
func (s *APIServer) RevokeModelAPIKey(w http.ResponseWriter, r *http.Request, id string, apiKeyName string) {
	s.revokeKindAPIKey(w, r, models.KindModel, "Model", id, apiKeyName)
}
