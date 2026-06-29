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

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/go-httpkit/httputil"
)

// CreateLLMProviderTemplate implements ServerInterface.CreateLLMProviderTemplate
// (POST /llm-provider-templates)
func (s *APIServer) CreateLLMProviderTemplate(w http.ResponseWriter, r *http.Request) {
	log := middleware.GetLogger(r, s.logger)
	correlationID := middleware.GetCorrelationID(r)

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	storedTemplate, err := s.llmDeploymentService.CreateLLMProviderTemplate(utils.LLMTemplateParams{
		Spec:          body,
		ContentType:   r.Header.Get("Content-Type"),
		CorrelationID: correlationID,
		Logger:        log,
	})

	if err != nil {
		if errors.Is(err, utils.ErrLLMTemplateValidation) {
			log.Warn("Template configuration invalid", slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}
		if storage.IsConflictError(err) {
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}
		log.Error("Failed to create LLM provider template", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to create LLM provider template",
		})
		return
	}

	log.Info("LLM provider template created successfully",
		slog.String("uuid", storedTemplate.UUID),
		slog.String("handle", storedTemplate.GetHandle()))

	httputil.WriteJSON(w, http.StatusCreated, buildTemplateResourceResponse(storedTemplate))
}

// ListLLMProviderTemplates implements ServerInterface.ListLLMProviderTemplates
// (GET /llm-providers/templates)
func (s *APIServer) ListLLMProviderTemplates(w http.ResponseWriter, r *http.Request, params api.ListLLMProviderTemplatesParams) {
	templates := s.llmDeploymentService.ListLLMProviderTemplates(params.DisplayName)

	items := make([]any, 0, len(templates))
	for _, tmpl := range templates {
		items = append(items, buildTemplateResourceResponse(tmpl))
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":    "success",
		"count":     len(items),
		"templates": items,
	})
}

// GetLLMProviderTemplateById implements ServerInterface.GetLLMProviderTemplateById
// (GET /llm-provider-templates/{id})
func (s *APIServer) GetLLMProviderTemplateById(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)

	template, err := s.llmDeploymentService.GetLLMProviderTemplateByHandle(id)
	if err != nil {
		if storage.IsNotFoundError(err) {
			log.Warn("LLM provider template not found", slog.String("handle", id))
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("Template with id '%s' not found", id),
			})
			return
		}
		log.Error("Failed to get LLM provider template", slog.String("handle", id), slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to get LLM provider template",
		})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, buildTemplateResourceResponse(template))
}

// UpdateLLMProviderTemplate implements ServerInterface.UpdateLLMProviderTemplate
// (PUT /llm-provider-templates/{id})
func (s *APIServer) UpdateLLMProviderTemplate(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)
	correlationID := middleware.GetCorrelationID(r)

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	updated, err := s.llmDeploymentService.UpdateLLMProviderTemplate(id, utils.LLMTemplateParams{
		Spec:          body,
		ContentType:   r.Header.Get("Content-Type"),
		CorrelationID: correlationID,
		Logger:        log,
	})
	if err != nil {
		if errors.Is(err, utils.ErrLLMTemplateNotFound) {
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("Template with id '%s' not found", id),
			})
			return
		}
		if errors.Is(err, utils.ErrLLMTemplateValidation) {
			log.Warn("Template configuration invalid", slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}
		log.Error("Failed to update LLM provider template", slog.String("id", id), slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to update LLM provider template",
		})
		return
	}

	log.Info("LLM provider template updated successfully",
		slog.String("uuid", updated.UUID),
		slog.String("handle", updated.GetHandle()))

	httputil.WriteJSON(w, http.StatusOK, buildTemplateResourceResponse(updated))
}

// DeleteLLMProviderTemplate implements ServerInterface.DeleteLLMProviderTemplate
// (DELETE /llm-provider-templates/{id})
func (s *APIServer) DeleteLLMProviderTemplate(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)
	correlationID := middleware.GetCorrelationID(r)

	deleted, err := s.llmDeploymentService.DeleteLLMProviderTemplate(id, correlationID, log)
	if err != nil {
		if errors.Is(err, utils.ErrLLMTemplateNotFound) {
			log.Warn("LLM provider template not found for deletion", slog.String("handle", id))
			httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
				Status:  "error",
				Message: fmt.Sprintf("Template with id '%s' not found", id),
			})
			return
		}
		log.Error("Failed to delete LLM provider template", slog.String("id", id), slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to delete LLM provider template",
		})
		return
	}

	log.Info("LLM provider template deleted successfully",
		slog.String("uuid", deleted.UUID),
		slog.String("handle", deleted.GetHandle()))

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": "LLM provider template deleted successfully",
		"id":      deleted.GetHandle(),
	})
}
