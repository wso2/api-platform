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
	"strings"
	"time"

	"github.com/wso2/api-platform/common/constants"
	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// budgetVersion is the artifacts.version of every budget; budgets are not versioned.
const budgetVersion = "v1"

// Publisher sends the stored budgets to the policy engine.
type Publisher interface {
	Publish(correlationID string) error
}

// Service stores budgets in the gateway database, publishes them to this
// replica's policy engine and tells the other replicas through the event hub.
type Service struct {
	db          storage.Storage
	publisher   Publisher
	eventHub    eventhub.EventHub
	gatewayID   string
	logger      *slog.Logger
	pushEnabled bool
}

// NewService returns a Service. pushEnabled is the gateway's deployment-sync
// setting: when true, gateway-created budgets start as pending push-up.
func NewService(db storage.Storage, publisher Publisher, eventHub eventhub.EventHub, gatewayID string, logger *slog.Logger, pushEnabled bool) *Service {
	if db == nil {
		panic("budget.Service requires non-nil storage")
	}
	if publisher == nil {
		panic("budget.Service requires a non-nil publisher")
	}
	if eventHub == nil {
		panic("budget.Service requires a non-nil event hub")
	}
	if strings.TrimSpace(gatewayID) == "" {
		panic("budget.Service requires a gateway ID")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{db: db, publisher: publisher, eventHub: eventHub, gatewayID: strings.TrimSpace(gatewayID), logger: logger, pushEnabled: pushEnabled}
}

// CreateParams holds the request for Create. Body is JSON or YAML.
type CreateParams struct {
	Body          []byte
	CorrelationID string
	Logger        *slog.Logger
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

	id, err := budgetID(budget.Metadata)
	if err != nil {
		return nil, err
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
	return cfg, s.publish("CREATE", cfg.Handle, params.CorrelationID, log)
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
	Name          string
	Body          []byte
	CorrelationID string
	Logger        *slog.Logger
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
	return &updated, s.publish("UPDATE", updated.Handle, params.CorrelationID, log)
}

// Delete removes a gateway-created budget.
func (s *Service) Delete(name, correlationID string, log *slog.Logger) error {
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
	return s.publish("DELETE", name, correlationID, log)
}

// publish follows a committed change: it tells the other replicas, then sends
// the stored budgets to this replica's policy engine. The event goes out even
// when the local publish fails, since the change is saved either way; a local
// failure wraps ErrPublish.
func (s *Service) publish(action, name, correlationID string, log *slog.Logger) error {
	event := eventhub.Event{
		GatewayID:           s.gatewayID,
		OriginatedTimestamp: time.Now(),
		EventType:           eventhub.EventTypeBudget,
		Action:              action,
		EntityID:            name,
		EventID:             correlationID,
		EventData:           eventhub.EmptyEventData,
	}
	if err := s.eventHub.PublishEvent(s.gatewayID, event); err != nil {
		log.Error("Failed to publish budget event to event hub",
			slog.String("action", action), slog.String("handle", name), slog.Any("error", err))
	}
	if err := s.publisher.Publish(correlationID); err != nil {
		return fmt.Errorf("%w: %v", ErrPublish, err)
	}
	return nil
}

// budgetID is the UUID of a budget created through the gateway: the artifact-id
// annotation when given, as for the gateway's other kinds, and otherwise a new one. The
// shared spend is keyed by the budget's name, not this UUID, so the same budget on every
// gateway counts against one total whatever its UUID.
func budgetID(metadata api.BudgetMetadata) (string, error) {
	if given := artifactID(metadata); given != "" {
		if err := utils.ValidateUUIDFormat(given); err != nil {
			return "", &ValidationError{Errors: []FieldError{{
				Field:   "metadata.annotations." + constants.AnnotationArtifactID,
				Message: err.Error(),
			}}}
		}
		return given, nil
	}
	id, err := utils.GenerateUUID()
	if err != nil {
		return "", fmt.Errorf("failed to generate budget ID: %w", err)
	}
	return id, nil
}

func artifactID(metadata api.BudgetMetadata) string {
	if metadata.Annotations == nil {
		return ""
	}
	return (*metadata.Annotations)[constants.AnnotationArtifactID]
}

// fixedFieldChanges lists the fields that differ but cannot change after
// creation: the artifact-id annotation, period, currency, resource and consumers.
func fixedFieldChanges(stored, updated api.BudgetRequest) []FieldError {
	const message = "cannot be changed after creation; create a new budget instead"
	var errs []FieldError
	if artifactID(stored.Metadata) != artifactID(updated.Metadata) {
		errs = append(errs, FieldError{Field: "metadata.annotations." + constants.AnnotationArtifactID, Message: message})
	}
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
