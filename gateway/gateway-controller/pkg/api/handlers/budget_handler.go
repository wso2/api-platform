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

	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/wso2/api-platform/common/budgetspend"
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
	if !s.budgetsAvailable(w) {
		return
	}
	log := middleware.GetLogger(r, s.logger)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Failed to read request body"})
		return
	}

	stored, err := s.budgetService.Create(budget.CreateParams{Body: body, ContentType: r.Header.Get("Content-Type"), CorrelationID: middleware.GetCorrelationID(r), Logger: log})
	if err != nil {
		var invalid *budget.ValidationError
		switch {
		case errors.As(err, &invalid):
			httputil.WriteJSON(w, http.StatusBadRequest, budgetValidationResponse(invalid))
		case isBudgetRequestError(err):
			httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
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

	httputil.WriteJSON(w, http.StatusCreated, s.budgetWithUsage(stored, log))
}

// ListBudgets implements ServerInterface.ListBudgets
// (GET /budgets)
func (s *APIServer) ListBudgets(w http.ResponseWriter, r *http.Request) {
	if !s.budgetsAvailable(w) {
		return
	}
	log := middleware.GetLogger(r, s.logger)

	cfgs, err := s.budgetService.List()
	if err != nil {
		log.Error("Failed to list budgets", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{Status: "error", Message: "Failed to list budgets"})
		return
	}

	spend, err := s.budgetService.ReadSpend()
	if err != nil {
		log.Warn("Budget spend not shown: it cannot be read", slog.Any("error", err))
	}
	items := make([]api.Budget, 0, len(cfgs))
	for _, cfg := range cfgs {
		if err != nil {
			items = append(items, budgetResponse(cfg, nil))
			continue
		}
		usage := s.budgetService.UsageOf(cfg, spend)
		items = append(items, budgetResponse(cfg, &usage))
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"status": "success", "count": len(items), "budgets": items})
}

// GetBudget implements ServerInterface.GetBudget
// (GET /budgets/{id})
func (s *APIServer) GetBudget(w http.ResponseWriter, r *http.Request, id string) {
	if !s.budgetsAvailable(w) {
		return
	}
	log := middleware.GetLogger(r, s.logger)

	cfg, err := s.budgetService.Get(id)
	if err != nil {
		writeBudgetError(w, log, "get", err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, s.budgetWithUsage(cfg, log))
}

// UpdateBudget implements ServerInterface.UpdateBudget
// (PUT /budgets/{id})
func (s *APIServer) UpdateBudget(w http.ResponseWriter, r *http.Request, id string) {
	if !s.budgetsAvailable(w) {
		return
	}
	log := middleware.GetLogger(r, s.logger)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("Failed to read request body", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: "Failed to read request body"})
		return
	}

	cfg, err := s.budgetService.Update(budget.UpdateParams{Name: id, Body: body, ContentType: r.Header.Get("Content-Type"), CorrelationID: middleware.GetCorrelationID(r), Logger: log})
	if err != nil {
		writeBudgetError(w, log, "update", err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, s.budgetWithUsage(cfg, log))
}

// DeleteBudget implements ServerInterface.DeleteBudget
// (DELETE /budgets/{id})
func (s *APIServer) DeleteBudget(w http.ResponseWriter, r *http.Request, id string) {
	if !s.budgetsAvailable(w) {
		return
	}
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
	case isBudgetRequestError(err):
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error", Message: err.Error()})
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

// budgetResponse is the stored budget with the server-managed status: its lifecycle and,
// when usage is known, the current period's spend.
func budgetResponse(cfg *models.StoredConfig, usage *budget.PeriodUsage) api.Budget {
	req, _ := cfg.SourceConfiguration.(api.BudgetRequest)
	rs := buildResourceStatus(cfg)
	status := api.BudgetStatus{Id: rs.Id, CreatedAt: rs.CreatedAt, UpdatedAt: rs.UpdatedAt}
	if usage != nil {
		spent, unpriced := usage.Spent()
		status.CurrentPeriod = &api.BudgetCurrentPeriod{
			PeriodStart:      openapi_types.Date{Time: usage.PeriodStart},
			Spent:            budgetspend.FormatUSD(spent),
			UnpricedRequests: unpriced,
		}
	}
	return api.Budget{
		ApiVersion: api.BudgetApiVersion(req.ApiVersion),
		Kind:       api.BudgetKind(req.Kind),
		Metadata:   req.Metadata,
		Spec:       req.Spec,
		Status:     &status,
	}
}

// budgetWithUsage is budgetResponse with the budget's spend read from the counter store;
// without it when the store cannot be read.
func (s *APIServer) budgetWithUsage(cfg *models.StoredConfig, log *slog.Logger) api.Budget {
	usage, err := s.budgetService.Usage(cfg.Handle)
	if err != nil {
		log.Warn("Budget spend not shown: it cannot be read", slog.String("handle", cfg.Handle), slog.Any("error", err))
		return budgetResponse(cfg, nil)
	}
	return budgetResponse(cfg, usage)
}

// SetBudgetSpend wires the counter store budget spend is kept in and what names its
// allowances, for the budget usage and for clearing a deleted budget's spend.
func (s *APIServer) SetBudgetSpend(counters budget.CounterStore, labels budget.AllowanceLabels) {
	s.budgetService.SetSpend(counters, labels)
}

// Budget usage page bounds, as the control plane's.
const (
	budgetUsageDefaultLimit = 20
	budgetUsageMaxLimit     = 100
)

// GetBudgetUsage implements ServerInterface.GetBudgetUsage
// (GET /budgets/{id}/usage)
func (s *APIServer) GetBudgetUsage(w http.ResponseWriter, r *http.Request, id string, params api.GetBudgetUsageParams) {
	if !s.budgetsAvailable(w) {
		return
	}
	log := middleware.GetLogger(r, s.logger)
	limit, offset := budgetUsageDefaultLimit, 0
	if params.Limit != nil {
		limit = *params.Limit
	}
	if params.Offset != nil {
		offset = *params.Offset
	}
	if limit < 1 || limit > budgetUsageMaxLimit || offset < 0 {
		httputil.WriteJSON(w, http.StatusBadRequest, api.ErrorResponse{Status: "error",
			Message: fmt.Sprintf("limit must be 1 to %d and offset 0 or more", budgetUsageMaxLimit)})
		return
	}
	usage, err := s.budgetService.Usage(id)
	if err != nil {
		writeBudgetError(w, log, "read the usage of", err)
		return
	}
	page := usage.Allowances[min(offset, len(usage.Allowances)):min(offset+limit, len(usage.Allowances))]
	rows := make([]api.BudgetAllowanceUsage, 0, len(page))
	for _, a := range page {
		rows = append(rows, api.BudgetAllowanceUsage{Allowance: a.Allowance, Spent: budgetspend.FormatUSD(a.Spent), UnpricedRequests: a.Unpriced})
	}
	httputil.WriteJSON(w, http.StatusOK, api.BudgetUsageListResponse{
		PeriodStart: openapi_types.Date{Time: usage.PeriodStart},
		Allowances:  rows, Total: len(usage.Allowances), Limit: limit, Offset: offset,
	})
}

// isBudgetRequestError reports a body that could not be read, or that names another budget
// than the path: both are the caller's to fix, like the agent API's parse and handle errors.
func isBudgetRequestError(err error) bool {
	var parseErr *budget.ParseError
	var nameErr *budget.NameMismatchError
	return errors.As(err, &parseErr) || errors.As(err, &nameErr)
}

func budgetValidationResponse(invalid *budget.ValidationError) api.ErrorResponse {
	details := make([]api.ValidationError, 0, len(invalid.Errors))
	for _, fe := range invalid.Errors {
		field, message := fe.Field, fe.Message
		details = append(details, api.ValidationError{Field: &field, Message: &message})
	}
	return api.ErrorResponse{Status: "error", Message: "Budget validation failed", Errors: &details}
}

// budgetsAvailable answers 404 when budgets are off on this gateway
// (ai_cost_management.budgets.enabled) and reports whether the request may go on.
func (s *APIServer) budgetsAvailable(w http.ResponseWriter) bool {
	if s.budgetsEnabled {
		return true
	}
	httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
		Status: "error", Message: "Budgets are not enabled on this gateway. Set ai_cost_management.budgets.enabled to use them.",
	})
	return false
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

// BudgetService is the service that stores budgets, for the immutable loader; nil when
// budgets are disabled.
func (s *APIServer) BudgetService() *budget.Service {
	if !s.budgetsEnabled {
		return nil
	}
	return s.budgetService
}
