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

// ListModels implements ServerInterface.ListModels
// (GET /models)
func (s *APIServer) ListModels(w http.ResponseWriter, r *http.Request, params api.ListModelsParams) {
	log := middleware.GetLogger(r, s.logger)
	configs := s.llmDeploymentService.ListModels(params)

	items := make([]any, 0, len(configs))
	for _, cfg := range configs {
		model, err := rematerializeModelConfig(log, cfg.UUID, cfg.SourceConfiguration)
		if err != nil {
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
				Status: "error", Message: "Failed to get stored Model configuration"})
			return
		}
		items = append(items, buildResourceResponseFromStored(model, cfg))
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"status": "success", "count": len(items), "models": items})
}

// CreateModel implements ServerInterface.CreateModel
// (POST /models)
func (s *APIServer) CreateModel(w http.ResponseWriter, r *http.Request) {
	log := middleware.GetLogger(r, s.logger)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Failed to read request body"})
		return
	}

	result, err := s.llmDeploymentService.CreateModel(utils.LLMDeploymentParams{
		Data:          body,
		ContentType:   r.Header.Get("Content-Type"),
		CorrelationID: middleware.GetCorrelationID(r),
		Logger:        log,
		Origin:        models.OriginGatewayAPI,
	})
	if err != nil {
		writeModelWriteError(w, log, "create", err)
		return
	}

	stored := result.StoredConfig
	log.Info("Model created", slog.String("uuid", stored.UUID), slog.String("handle", stored.Handle))

	model, err := rematerializeModelConfig(log, stored.UUID, stored.SourceConfiguration)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status: "error", Message: "Failed to get stored Model configuration"})
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, buildResourceResponseFromStored(model, stored))
}

// GetModelById implements ServerInterface.GetModelById
// (GET /models/{id})
func (s *APIServer) GetModelById(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)

	cfg, err := s.llmDeploymentService.GetModelByHandle(id)
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
				Status: "error", Message: fmt.Sprintf("Model with handle '%s' not found", id)})
			return
		}
		log.Error("Failed to look up Model", slog.String("handle", id), slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to look up Model"})
		return
	}

	model, err := rematerializeModelConfig(log, cfg.UUID, cfg.SourceConfiguration)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status: "error", Message: "Failed to get stored Model configuration"})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, buildResourceResponseFromStored(model, cfg))
}

// UpdateModel implements ServerInterface.UpdateModel
// (PUT /models/{id})
func (s *APIServer) UpdateModel(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Failed to read request body"})
		return
	}

	result, err := s.llmDeploymentService.UpdateModel(id, utils.LLMDeploymentParams{
		Data:          body,
		ContentType:   r.Header.Get("Content-Type"),
		CorrelationID: middleware.GetCorrelationID(r),
		Logger:        log,
		Origin:        models.OriginGatewayAPI,
	})
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
				Status: "error", Message: fmt.Sprintf("Model with handle '%s' not found", id)})
			return
		}
		writeModelWriteError(w, log, "update", err)
		return
	}

	updated := result.StoredConfig
	model, err := rematerializeModelConfig(log, updated.UUID, updated.SourceConfiguration)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status: "error", Message: "Failed to get stored Model configuration"})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, buildResourceResponseFromStored(model, updated))
}

// DeleteModel implements ServerInterface.DeleteModel
// (DELETE /models/{id})
func (s *APIServer) DeleteModel(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)

	cfg, err := s.llmDeploymentService.DeleteModel(id, middleware.GetCorrelationID(r), log)
	if err != nil {
		if storage.IsNotFoundError(err) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
				Status: "error", Message: fmt.Sprintf("Model with handle '%s' not found", id)})
			return
		}
		log.Error("Failed to delete Model", slog.String("handle", id), slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to delete Model"})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": "Model deleted successfully",
		"id":      cfg.Handle,
	})
}

// writeModelWriteError maps a Model create or update failure to its response.
func writeModelWriteError(w http.ResponseWriter, log *slog.Logger, operation string, err error) {
	if mapRenderError(w, operation, err) {
		return
	}
	switch {
	case utils.IsPolicyDefinitionMissingError(err):
		log.Error("Model "+operation+" failed - policy definition missing", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status: "error", Message: utils.PolicyDefinitionMissingUserMessage})
	case errors.Is(err, utils.ErrModelValidation):
		log.Warn("Model configuration invalid", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
	case storage.IsConflictError(err):
		httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
	default:
		log.Error("Failed to "+operation+" Model", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status: "error", Message: "Failed to " + operation + " Model"})
	}
}

// rematerializeModelConfig re-encodes a stored Model into a response-bound copy
// with every target credential redacted. The stored SourceConfiguration is never
// modified.
func rematerializeModelConfig(log *slog.Logger, id string, source any) (api.ModelConfiguration, error) {
	j, err := json.Marshal(source)
	if err != nil {
		log.Error("Failed to marshal stored Model", slog.String("id", id), slog.Any("error", err))
		return api.ModelConfiguration{}, fmt.Errorf("marshal Model: %w", err)
	}
	var model api.ModelConfiguration
	if err := json.Unmarshal(j, &model); err != nil {
		log.Error("Failed to unmarshal stored Model", slog.String("id", id), slog.Any("error", err))
		return api.ModelConfiguration{}, fmt.Errorf("unmarshal Model: %w", err)
	}
	redactModelCredentials(&model)
	return model, nil
}
