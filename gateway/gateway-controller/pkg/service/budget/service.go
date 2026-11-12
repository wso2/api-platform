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

// Package budget creates and stores budgets: spend limits on LLM traffic.
package budget

import (
	"fmt"
	"log/slog"
	"reflect"
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// budgetVersion is the artifacts.version of every budget; budgets are not versioned.
const budgetVersion = "v1"

// Service stores budgets in the gateway database.
type Service struct {
	db          storage.Storage
	logger      *slog.Logger
	pushEnabled bool
}

// NewService returns a Service. pushEnabled is the gateway's deployment-sync
// setting: when true, gateway-created budgets start as pending push-up.
func NewService(db storage.Storage, logger *slog.Logger, pushEnabled bool) *Service {
	if db == nil {
		panic("budget.Service requires non-nil storage")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{db: db, logger: logger, pushEnabled: pushEnabled}
}

// CreateParams holds the request for Create. Body is JSON or YAML.
type CreateParams struct {
	Body   []byte
	Logger *slog.Logger
}

// Create parses, validates and stores a budget created through the gateway
// API. It returns a *ValidationError for an invalid budget and an error
// wrapping storage.ErrConflict when the name is taken.
func (s *Service) Create(params CreateParams) (*models.StoredConfig, error) {
	log := params.Logger
	if log == nil {
		log = s.logger
	}

	budget, err := decode(params.Body)
	if err != nil {
		return nil, err
	}
	if err := s.checkProviderExists(budget.Spec.Resource); err != nil {
		return nil, err
	}

	id, err := utils.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate budget ID: %w", err)
	}
	now := time.Now()
	cfg := &models.StoredConfig{
		UUID:   id,
		Kind:   models.KindBudget,
		Handle: budget.Metadata.Name,
		// artifacts is unique on (kind, display_name, version), and budget
		// display names need not be unique, so the handle stands in.
		DisplayName:         budget.Metadata.Name,
		Version:             budgetVersion,
		Configuration:       *budget,
		SourceConfiguration: *budget,
		DesiredState:        models.StateDeployed,
		Origin:              models.OriginGatewayAPI,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if s.pushEnabled {
		cfg.CPSyncStatus = models.CPSyncStatusPending
	}

	if err := s.db.SaveConfig(cfg); err != nil {
		if storage.IsConflictError(err) {
			return nil, fmt.Errorf("%w: a budget named %q already exists", storage.ErrConflict, budget.Metadata.Name)
		}
		return nil, fmt.Errorf("failed to save budget: %w", err)
	}

	log.Info("Budget created", slog.String("uuid", cfg.UUID), slog.String("handle", cfg.Handle))
	return cfg, nil
}

// Get returns the budget with the given name.
func (s *Service) Get(name string) (*models.StoredConfig, error) {
	cfg, err := s.db.GetConfigByKindAndHandle(models.KindBudget, name)
	if err != nil {
		if storage.IsNotFoundError(err) {
			return nil, fmt.Errorf("%w: %q", ErrNotFound, name)
		}
		return nil, fmt.Errorf("failed to get budget: %w", err)
	}
	return cfg, nil
}

// List returns every budget on the gateway, active and inactive.
func (s *Service) List() ([]*models.StoredConfig, error) {
	cfgs, err := s.db.GetAllConfigsByKind(models.KindBudget)
	if err != nil {
		return nil, fmt.Errorf("failed to list budgets: %w", err)
	}
	return cfgs, nil
}

// UpdateParams holds the request for Update. Body is the full budget, JSON or YAML.
type UpdateParams struct {
	Name   string
	Body   []byte
	Logger *slog.Logger
}

// Update changes a gateway-created budget. Only status, the limit's amount,
// action and blockAt, and the exemptions can change; any other difference is
// a *ValidationError.
func (s *Service) Update(params UpdateParams) (*models.StoredConfig, error) {
	log := params.Logger
	if log == nil {
		log = s.logger
	}

	budget, err := decode(params.Body)
	if err != nil {
		return nil, err
	}
	if budget.Metadata.Name != params.Name {
		return nil, &ValidationError{Errors: []FieldError{{Field: "metadata.name", Message: "must match the budget being updated"}}}
	}

	existing, err := s.Get(params.Name)
	if err != nil {
		return nil, err
	}
	if existing.Origin == models.OriginControlPlane {
		return nil, ErrReadOnly
	}
	stored, ok := existing.SourceConfiguration.(api.BudgetRequest)
	if !ok {
		return nil, fmt.Errorf("stored budget %q has configuration of type %T", params.Name, existing.SourceConfiguration)
	}
	if errs := fixedFieldChanges(stored, *budget); len(errs) > 0 {
		return nil, &ValidationError{Errors: errs}
	}

	updated := *existing
	updated.Configuration = *budget
	updated.SourceConfiguration = *budget
	updated.UpdatedAt = time.Now()
	if s.pushEnabled {
		updated.CPSyncStatus = models.CPSyncStatusPending
		updated.CPSyncInfo = ""
	}
	if err := s.db.UpdateConfig(&updated); err != nil {
		return nil, fmt.Errorf("failed to update budget: %w", err)
	}

	log.Info("Budget updated", slog.String("uuid", updated.UUID), slog.String("handle", updated.Handle))
	return &updated, nil
}

// Delete removes a gateway-created budget.
func (s *Service) Delete(name string, log *slog.Logger) error {
	if log == nil {
		log = s.logger
	}
	existing, err := s.Get(name)
	if err != nil {
		return err
	}
	if existing.Origin == models.OriginControlPlane {
		return ErrReadOnly
	}
	if err := s.db.DeleteConfig(existing.UUID); err != nil {
		if storage.IsNotFoundError(err) {
			return fmt.Errorf("%w: %q", ErrNotFound, name)
		}
		return fmt.Errorf("failed to delete budget: %w", err)
	}

	log.Info("Budget deleted", slog.String("uuid", existing.UUID), slog.String("handle", name))
	return nil
}

// fixedFieldChanges lists the fields that differ but cannot change after
// creation: period, currency, resource and consumers.
func fixedFieldChanges(stored, updated api.BudgetRequest) []FieldError {
	const message = "cannot be changed after creation; create a new budget instead"
	var errs []FieldError
	if stored.Spec.Limit.Period != updated.Spec.Limit.Period {
		errs = append(errs, FieldError{Field: "spec.limit.period", Message: message})
	}
	if !reflect.DeepEqual(stored.Spec.Limit.Currency, updated.Spec.Limit.Currency) {
		errs = append(errs, FieldError{Field: "spec.limit.currency", Message: message})
	}
	if !reflect.DeepEqual(stored.Spec.Resource, updated.Spec.Resource) {
		errs = append(errs, FieldError{Field: "spec.resource", Message: message})
	}
	if !reflect.DeepEqual(stored.Spec.Consumers, updated.Spec.Consumers) {
		errs = append(errs, FieldError{Field: "spec.consumers", Message: message})
	}
	return errs
}

// checkProviderExists rejects a budget whose resource names an LLM provider
// that is not on this gateway.
func (s *Service) checkProviderExists(resource api.BudgetResource) error {
	if resource.Id == nil {
		return nil
	}
	_, err := s.db.GetConfigByKindAndHandle(models.KindLlmProvider, *resource.Id)
	switch {
	case err == nil:
		return nil
	case storage.IsNotFoundError(err):
		return &ValidationError{Errors: []FieldError{{
			Field:   "spec.resource.id",
			Message: fmt.Sprintf("no LLM provider named %q on this gateway", *resource.Id),
		}}}
	default:
		return fmt.Errorf("failed to look up LLM provider %q: %w", *resource.Id, err)
	}
}
