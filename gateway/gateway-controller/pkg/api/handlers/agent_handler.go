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
	"strings"
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/agent"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/httpkit/httputil"
)

// metricsKindAgent is the kind label Agent operations report under.
const metricsKindAgent = "agent"

// AgentHandler handles HTTP requests for Agent (A2A) CRUD operations.
type AgentHandler struct {
	service *agent.AgentService
	logger  *slog.Logger
}

// NewAgentHandler creates a new AgentHandler.
func NewAgentHandler(service *agent.AgentService, logger *slog.Logger) *AgentHandler {
	return &AgentHandler{
		service: service,
		logger:  logger,
	}
}

// CreateAgent implements ServerInterface.CreateAgent
// (POST /agents)
func (h *AgentHandler) CreateAgent(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	operation := "create"

	log := middleware.GetLogger(r, h.logger)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		metrics.APIOperationsTotal.WithLabelValues(operation, "error", metricsKindAgent).Inc()
		metrics.ValidationErrorsTotal.WithLabelValues(operation, "read_body_failed").Inc()
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	result, err := h.service.Create(agent.CreateParams{
		Body:          body,
		ContentType:   r.Header.Get("Content-Type"),
		CorrelationID: middleware.GetCorrelationID(r),
		Origin:        models.OriginGatewayAPI,
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to deploy agent configuration", slog.Any("error", err))
		metrics.APIOperationsTotal.WithLabelValues(operation, "error", metricsKindAgent).Inc()
		h.mapWriteError(w, operation, "", err)
		return
	}

	metrics.APIOperationsTotal.WithLabelValues(operation, "success", metricsKindAgent).Inc()
	metrics.APIOperationDurationSeconds.WithLabelValues(operation, metricsKindAgent).Observe(time.Since(startTime).Seconds())
	metrics.APIsTotal.WithLabelValues(metricsKindAgent, "active").Inc()

	response, err := h.buildAgentResponse(log, result.StoredConfig)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to build agent response",
		})
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, response)
}

// ListAgents implements ServerInterface.ListAgents
// (GET /agents)
func (h *AgentHandler) ListAgents(w http.ResponseWriter, r *http.Request, params api.ListAgentsParams) {
	log := middleware.GetLogger(r, h.logger)

	result, err := h.service.List(params)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to retrieve agent configurations",
		})
		return
	}

	items := make([]any, 0, len(result.Items))
	for _, cfg := range result.Items {
		item, err := h.buildAgentResponse(log, cfg)
		if err != nil {
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Failed to build agent response",
			})
			return
		}
		items = append(items, item)
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status": "success",
		"count":  len(items),
		"agents": items,
	})
}

// GetAgentById implements ServerInterface.GetAgentById
// (GET /agents/{id})
func (h *AgentHandler) GetAgentById(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, h.logger)

	result, err := h.service.GetByHandle(id)
	if err != nil {
		h.mapReadError(w, log, id, err)
		return
	}

	response, err := h.buildAgentResponse(log, result.Config)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to build agent response",
		})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, response)
}

// UpdateAgent implements ServerInterface.UpdateAgent
// (PUT /agents/{id})
func (h *AgentHandler) UpdateAgent(w http.ResponseWriter, r *http.Request, id string) {
	startTime := time.Now()
	operation := "update"

	log := middleware.GetLogger(r, h.logger)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		metrics.APIOperationsTotal.WithLabelValues(operation, "error", metricsKindAgent).Inc()
		metrics.ValidationErrorsTotal.WithLabelValues(operation, "read_body_failed").Inc()
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	result, err := h.service.Update(agent.UpdateParams{
		Handle:        id,
		Body:          body,
		ContentType:   r.Header.Get("Content-Type"),
		CorrelationID: middleware.GetCorrelationID(r),
		Logger:        log,
	})
	if err != nil {
		log.Error("Failed to update agent configuration", slog.Any("error", err))
		metrics.APIOperationsTotal.WithLabelValues(operation, "error", metricsKindAgent).Inc()
		h.mapWriteError(w, operation, id, err)
		return
	}

	metrics.APIOperationsTotal.WithLabelValues(operation, "success", metricsKindAgent).Inc()
	metrics.APIOperationDurationSeconds.WithLabelValues(operation, metricsKindAgent).Observe(time.Since(startTime).Seconds())

	response, err := h.buildAgentResponse(log, result.Config)
	if err != nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to build agent response",
		})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, response)
}

// DeleteAgent implements ServerInterface.DeleteAgent
// (DELETE /agents/{id})
func (h *AgentHandler) DeleteAgent(w http.ResponseWriter, r *http.Request, id string) {
	startTime := time.Now()
	operation := "delete"

	log := middleware.GetLogger(r, h.logger)

	_, err := h.service.Delete(agent.DeleteParams{
		Handle:        id,
		CorrelationID: middleware.GetCorrelationID(r),
		Logger:        log,
	})
	if err != nil {
		metrics.APIOperationsTotal.WithLabelValues(operation, "error", metricsKindAgent).Inc()
		h.mapReadError(w, log, id, err)
		return
	}

	metrics.APIOperationsTotal.WithLabelValues(operation, "success", metricsKindAgent).Inc()
	metrics.APIOperationDurationSeconds.WithLabelValues(operation, metricsKindAgent).Observe(time.Since(startTime).Seconds())
	metrics.APIsTotal.WithLabelValues(metricsKindAgent, "active").Dec()

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": "Agent deleted successfully",
		"id":      id,
	})
}

// The five API key handlers below hang off APIServer rather than AgentHandler:
// the shared APIKeyService lives there, and every other kind's key handlers are
// APIServer methods too. Agent keys reuse the existing key schemas, storage,
// xDS and `api-key-auth` enforcement unchanged — the only Agent-specific part
// is `Kind: models.KindAgent`, which selects the artifact the handle resolves
// against. Omitting it is not an error: the service defaults an empty Kind to
// KindRestApi and would silently operate on a RestAPI with the same handle.

// CreateAgentAPIKey implements ServerInterface.CreateAgentAPIKey
// (POST /agents/{id}/api-keys)
func (s *APIServer) CreateAgentAPIKey(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "CreateAgentAPIKey", correlationID)
	if !ok {
		return
	}

	var request api.APIKeyCreationRequest
	if err := s.bindRequestBody(r, &request); err != nil {
		log.Error("Failed to parse request body for Agent API key creation",
			slog.Any("error", err),
			slog.String("handle", handle),
			slog.String("correlation_id", correlationID))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	params := utils.APIKeyCreationParams{
		Kind:          models.KindAgent,
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
		} else {
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: err.Error()})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, result.Response)
}

// ListAgentAPIKeys implements ServerInterface.ListAgentAPIKeys
// (GET /agents/{id}/api-keys)
func (s *APIServer) ListAgentAPIKeys(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "ListAgentAPIKeys", correlationID)
	if !ok {
		return
	}

	params := utils.ListAPIKeyParams{
		Kind:          models.KindAgent,
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
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: err.Error()})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

// RegenerateAgentAPIKey implements ServerInterface.RegenerateAgentAPIKey
// (POST /agents/{id}/api-keys/{apiKeyName}/regenerate)
func (s *APIServer) RegenerateAgentAPIKey(w http.ResponseWriter, r *http.Request, id string, apiKeyName string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "RegenerateAgentAPIKey", correlationID)
	if !ok {
		return
	}

	var request api.APIKeyRegenerationRequest
	if err := s.bindRequestBody(r, &request); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	params := utils.APIKeyRegenerationParams{
		Kind:          models.KindAgent,
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
		} else {
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: err.Error()})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

// UpdateAgentAPIKey implements ServerInterface.UpdateAgentAPIKey
// (PUT /agents/{id}/api-keys/{apiKeyName})
func (s *APIServer) UpdateAgentAPIKey(w http.ResponseWriter, r *http.Request, id string, apiKeyName string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "UpdateAgentAPIKey", correlationID)
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
		Kind:          models.KindAgent,
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
		} else {
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: err.Error()})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

// RevokeAgentAPIKey implements ServerInterface.RevokeAgentAPIKey
// (DELETE /agents/{id}/api-keys/{apiKeyName})
func (s *APIServer) RevokeAgentAPIKey(w http.ResponseWriter, r *http.Request, id string, apiKeyName string) {
	log := middleware.GetLogger(r, s.logger)
	handle := id
	correlationID := middleware.GetCorrelationID(r)

	user, ok := s.extractAuthenticatedUser(w, r, "RevokeAgentAPIKey", correlationID)
	if !ok {
		return
	}

	params := utils.APIKeyRevocationParams{
		Kind:          models.KindAgent,
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
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: err.Error()})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result.Response)
}

// buildAgentResponse renders one stored Agent as a management API response body,
// with the upstream credential stripped.
func (h *AgentHandler) buildAgentResponse(log *slog.Logger, cfg *models.StoredConfig) (any, error) {
	agentConfig, err := rematerializeAgentConfig(log, cfg.UUID, cfg.DisplayName, cfg.SourceConfiguration)
	if err != nil {
		return nil, err
	}

	// Echo the resolved context so a `$version`-style placeholder is not
	// reflected back verbatim, matching the other kinds' list/get bodies. An
	// Agent may have no context at all, in which case there is nothing to
	// resolve and the field stays absent rather than becoming an empty string.
	if resolved, err := cfg.GetContext(); err == nil && resolved != "" {
		agentConfig.Spec.Context = &resolved
	}

	return buildResourceResponseFromStored(agentConfig, cfg), nil
}

// mapWriteError maps service errors to HTTP responses for Create and Update.
func (h *AgentHandler) mapWriteError(w http.ResponseWriter, operation, handle string, err error) {
	if mapRenderError(w, operation, err) {
		return
	}

	var parseErr *agent.ParseError
	if errors.As(err, &parseErr) {
		metrics.ValidationErrorsTotal.WithLabelValues(operation, "parse_failed").Inc()
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Failed to parse configuration: %v", parseErr.Cause),
		})
		return
	}

	var kindErr *agent.KindMismatchError
	if errors.As(err, &kindErr) {
		metrics.ValidationErrorsTotal.WithLabelValues(operation, "kind_mismatch").Inc()
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: kindErr.Error(),
		})
		return
	}

	var handleErr *agent.HandleMismatchError
	if errors.As(err, &handleErr) {
		metrics.ValidationErrorsTotal.WithLabelValues(operation, "handle_mismatch").Inc()
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: handleErr.Error(),
		})
		return
	}

	var validationErr *agent.ValidationError
	if errors.As(err, &validationErr) {
		metrics.ValidationErrorsTotal.WithLabelValues(operation, "validation_failed").Add(float64(len(validationErr.Errors)))
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
		return
	}

	if errors.Is(err, agent.ErrNotFound) {
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Agent with handle '%s' not found", handle),
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

	if storage.IsDatabaseUnavailableError(err) {
		httputil.WriteJSON(w, http.StatusServiceUnavailable, api.ErrorResponse{
			Status:  "error",
			Message: "Database storage not available",
		})
		return
	}

	httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
		Status:  "error",
		Message: "Failed to persist agent configuration",
	})
}

// mapReadError maps service errors to HTTP responses for Get and Delete.
func (h *AgentHandler) mapReadError(w http.ResponseWriter, log *slog.Logger, handle string, err error) {
	if storage.IsDatabaseUnavailableError(err) {
		httputil.WriteJSON(w, http.StatusServiceUnavailable, api.ErrorResponse{
			Status:  "error",
			Message: "Database storage not available",
		})
		return
	}

	if errors.Is(err, agent.ErrNotFound) {
		log.Warn("Agent configuration not found", slog.String("handle", handle))
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("Agent with handle '%s' not found", handle),
		})
		return
	}

	httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
		Status:  "error",
		Message: "Failed to retrieve agent configuration",
	})
}

// rematerializeAgentConfig round-trips a stored Agent source configuration into
// a response-bound copy and redacts the upstream credential from it.
//
// The round-trip is what makes the redaction safe: it produces a value distinct
// from the StoredConfig's own SourceConfiguration, which each replica re-renders
// on consumption and must therefore keep intact.
func rematerializeAgentConfig(log *slog.Logger, id, displayName string, source any) (api.AgentConfiguration, error) {
	j, err := json.Marshal(source)
	if err != nil {
		log.Error("Failed to marshal stored agent source configuration",
			slog.String("id", id),
			slog.String("displayName", displayName),
			slog.Any("error", err))
		return api.AgentConfiguration{}, fmt.Errorf("marshal agent config: %w", err)
	}
	var agentConfig api.AgentConfiguration
	if err := json.Unmarshal(j, &agentConfig); err != nil {
		log.Error("Failed to unmarshal stored agent configuration",
			slog.String("id", id),
			slog.String("displayName", displayName),
			slog.Any("error", err))
		return api.AgentConfiguration{}, fmt.Errorf("unmarshal agent config: %w", err)
	}
	redactAgentCredentials(&agentConfig)
	return agentConfig, nil
}
