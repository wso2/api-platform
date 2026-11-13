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
	"sort"

	adminapi "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/admin"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/budget"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/httpkit/httputil"
)

// CreateBudget implements ServerInterface.CreateBudget
// (POST /budgets)
func (s *APIServer) CreateBudget(w http.ResponseWriter, r *http.Request) {
	log := middleware.GetLogger(r, s.logger)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Failed to read request body"})
		return
	}

	stored, err := s.budgetService.Create(budget.CreateParams{Body: body, CorrelationID: middleware.GetCorrelationID(r), Logger: log})
	if err != nil {
		var invalid *budget.ValidationError
		switch {
		case errors.As(err, &invalid):
			httputil.WriteJSON(w, http.StatusBadRequest, budgetValidationResponse(invalid))
		case storage.IsConflictError(err):
			httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
		case errors.Is(err, budget.ErrPublish):
			log.Error("Failed to publish budgets", slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: budget.ErrPublish.Error()})
		default:
			log.Error("Failed to create budget", slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to create budget"})
		}
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, budgetResponse(stored))
}

// ListBudgets implements ServerInterface.ListBudgets
// (GET /budgets)
func (s *APIServer) ListBudgets(w http.ResponseWriter, r *http.Request) {
	log := middleware.GetLogger(r, s.logger)

	cfgs, err := s.budgetService.List()
	if err != nil {
		log.Error("Failed to list budgets", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to list budgets"})
		return
	}

	items := make([]api.Budget, 0, len(cfgs))
	for _, cfg := range cfgs {
		items = append(items, budgetResponse(cfg))
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"status": "success", "count": len(items), "budgets": items})
}

// GetBudget implements ServerInterface.GetBudget
// (GET /budgets/{id})
func (s *APIServer) GetBudget(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)

	cfg, err := s.budgetService.Get(id)
	if err != nil {
		writeBudgetError(w, log, "get", err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, budgetResponse(cfg))
}

// UpdateBudget implements ServerInterface.UpdateBudget
// (PUT /budgets/{id})
func (s *APIServer) UpdateBudget(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Failed to read request body"})
		return
	}

	cfg, err := s.budgetService.Update(budget.UpdateParams{Name: id, Body: body, CorrelationID: middleware.GetCorrelationID(r), Logger: log})
	if err != nil {
		writeBudgetError(w, log, "update", err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, budgetResponse(cfg))
}

// DeleteBudget implements ServerInterface.DeleteBudget
// (DELETE /budgets/{id})
func (s *APIServer) DeleteBudget(w http.ResponseWriter, r *http.Request, id string) {
	log := middleware.GetLogger(r, s.logger)

	if err := s.budgetService.Delete(id, middleware.GetCorrelationID(r), log); err != nil {
		writeBudgetError(w, log, "delete", err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"status": "success", "message": "Budget deleted successfully", "id": id})
}

// writeBudgetError maps a budget service error to its HTTP response.
func writeBudgetError(w http.ResponseWriter, log *slog.Logger, action string, err error) {
	var invalid *budget.ValidationError
	switch {
	case errors.As(err, &invalid):
		httputil.WriteJSON(w, http.StatusBadRequest, budgetValidationResponse(invalid))
	case errors.Is(err, budget.ErrNotFound):
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{Status: "error", Message: err.Error()})
	case errors.Is(err, budget.ErrReadOnly):
		httputil.WriteJSON(w, http.StatusConflict, api.ErrorResponse{Status: "error", Message: err.Error()})
	case errors.Is(err, budget.ErrPublish):
		log.Error("Failed to publish budgets", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: budget.ErrPublish.Error()})
	default:
		log.Error("Failed to "+action+" budget", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to " + action + " budget"})
	}
}

// budgetResponse is the stored budget with the server-managed status.
func budgetResponse(cfg *models.StoredConfig) api.Budget {
	req, _ := cfg.SourceConfiguration.(api.BudgetRequest)
	status := buildResourceStatus(cfg)
	return api.Budget{
		ApiVersion: api.BudgetApiVersion(req.ApiVersion),
		Kind:       api.BudgetKind(req.Kind),
		Metadata:   req.Metadata,
		Spec:       req.Spec,
		Status:     &status,
	}
}

func budgetValidationResponse(invalid *budget.ValidationError) api.ErrorResponse {
	details := make([]api.ValidationError, 0, len(invalid.Errors))
	for _, fe := range invalid.Errors {
		field, message := fe.Field, fe.Message
		details = append(details, api.ValidationError{Field: &field, Message: &message})
	}
	return api.ErrorResponse{Status: "error", Message: "Budget validation failed", Errors: &details}
}

// configDumpBudgets lists the stored budgets for the config dump, in name order, each
// marked enforced when it is published to the policy engine.
func (s *APIServer) configDumpBudgets() ([]adminapi.ConfigDumpBudgetItem, error) {
	if s.budgetService == nil {
		return []adminapi.ConfigDumpBudgetItem{}, nil
	}
	cfgs, err := s.budgetService.List()
	if err != nil {
		return nil, err
	}
	published := map[string]bool{}
	if s.budgetPublisher != nil {
		published = s.budgetPublisher.PublishedNames()
	}
	sort.Slice(cfgs, func(i, j int) bool { return cfgs[i].Handle < cfgs[j].Handle })
	items := make([]adminapi.ConfigDumpBudgetItem, 0, len(cfgs))
	for _, cfg := range cfgs {
		configuration, err := toGenericMap(cfg.SourceConfiguration)
		if err != nil {
			return nil, fmt.Errorf("budget %q: %w", cfg.Handle, err)
		}
		id, err := uuidToOpenAPIUUID(cfg.UUID)
		if err != nil {
			return nil, fmt.Errorf("budget %q: %w", cfg.Handle, err)
		}
		createdAt, updatedAt := cfg.CreatedAt, cfg.UpdatedAt
		items = append(items, adminapi.ConfigDumpBudgetItem{
			Id:            id,
			Configuration: &configuration,
			Metadata: &adminapi.ConfigDumpBudgetMetadata{
				Origin:    ptr(string(cfg.Origin)),
				Enforced:  ptr(published[cfg.Handle]),
				CreatedAt: &createdAt,
				UpdatedAt: &updatedAt,
			},
		})
	}
	return items, nil
}
