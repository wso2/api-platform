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
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/wso2/api-platform/common/budgetspend"
	"github.com/wso2/api-platform/common/constants"
	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/common/counterproto"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
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
	// pusher pushes this gateway's own budgets to the control plane; nil until
	// SetResourcePusher.
	pusher utils.ResourcePusher
	// counters and labels hold budget spend; nil until SetSpend (budgets off).
	counters CounterStore
	labels   UsageLabels
	now      func() time.Time
}

// CounterStore is the shared counter store budgets count spend in.
type CounterStore interface {
	Totals(ctx context.Context, namespace, idPrefix string) ([]counterproto.Total, error)
	Delete(ctx context.Context, namespace, idPrefix string) error
}

// UsageLabels knows the values of the usages this gateway has served.
type UsageLabels interface {
	// Labels returns the values of each usage of the budget, by counter ID.
	Labels(budget string) map[string]map[string]string
	// Forget drops what is kept for a deleted budget.
	Forget(budget string)
}

// SetSpend wires the counter store budget spend is kept in, and what names usages:
// the budget usage reads them, and deleting a budget clears its spend.
func (s *Service) SetSpend(counters CounterStore, labels UsageLabels) {
	s.counters, s.labels = counters, labels
}

// spendTimeout bounds reading or clearing spend in the counter store.
const spendTimeout = 5 * time.Second

// spendSweepDelay is how long after a delete its spend is cleared once more: long enough
// for a policy engine's last report (sent every second) and for the other controllers of
// the gateway to see the delete (event hub poll), so spend they were still sending for it
// is not left in the store.
var spendSweepDelay = 5 * time.Second

// clearSpend removes a deleted budget's spend, so a budget made again under its name
// starts from nothing. Policy engines drop their copies when the budget leaves their set,
// and stop charging it; spend already on its way is cleared by a second sweep, unless a
// budget of that name exists again by then. With a shared store this clears the name's
// spend for every gateway sharing it.
func (s *Service) clearSpend(name string, log *slog.Logger) {
	if s.labels != nil {
		s.labels.Forget(name)
	}
	if s.counters == nil {
		return
	}
	s.deleteSpend(name, log)
	time.AfterFunc(spendSweepDelay, func() {
		if _, err := s.db.GetConfigByKindAndHandle(models.KindBudget, name); err == nil {
			return // made again: its spend is its own now
		}
		if s.labels != nil {
			s.labels.Forget(name)
		}
		s.deleteSpend(name, log)
	})
}

// deleteSpend deletes every counter of the name in the store.
func (s *Service) deleteSpend(name string, log *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), spendTimeout)
	defer cancel()
	prefix := budgetspend.CounterIDPrefix(name)
	for _, namespace := range []string{budgetspend.NamespaceSpend, budgetspend.NamespaceUnpriced} {
		if err := s.counters.Delete(ctx, namespace, prefix); err != nil {
			log.Error("Failed to clear the deleted budget's spend", slog.String("handle", name), slog.Any("error", err))
			return
		}
	}
	log.Info("Budget spend cleared", slog.String("handle", name))
}

// UsageEntry is one usage's spend in the current period.
type UsageEntry struct {
	// Usage holds the values it is separated by; empty when shared, or when this
	// gateway has not served it.
	Usage    map[string]string
	Spent    float64
	Unpriced int64
}

// PeriodUsage is a budget's spend in its current period, highest spend first.
type PeriodUsage struct {
	PeriodStart time.Time
	Usages      []UsageEntry
}

// Spent is the total across the usages.
func (u PeriodUsage) Spent() (spent float64, unpriced int64) {
	for _, a := range u.Usages {
		spent += a.Spent
		unpriced += a.Unpriced
	}
	return spent, unpriced
}

// SpendTotals is every budget's spend, read once for a list.
type SpendTotals struct {
	spend, unpriced []counterproto.Total
}

// ReadSpend reads the spend of every budget from the counter store.
func (s *Service) ReadSpend() (SpendTotals, error) {
	return s.readSpend("")
}

func (s *Service) readSpend(prefix string) (SpendTotals, error) {
	if s.counters == nil {
		return SpendTotals{}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), spendTimeout)
	defer cancel()
	spend, err := s.counters.Totals(ctx, budgetspend.NamespaceSpend, prefix)
	if err != nil {
		return SpendTotals{}, fmt.Errorf("failed to read budget spend: %w", err)
	}
	unpriced, err := s.counters.Totals(ctx, budgetspend.NamespaceUnpriced, prefix)
	if err != nil {
		return SpendTotals{}, fmt.Errorf("failed to read budget spend: %w", err)
	}
	return SpendTotals{spend: spend, unpriced: unpriced}, nil
}

// Usage returns the named budget's spend in its current period.
func (s *Service) Usage(name string) (*PeriodUsage, error) {
	cfg, err := s.Get(name)
	if err != nil {
		return nil, err
	}
	totals, err := s.readSpend(budgetspend.CounterIDPrefix(name))
	if err != nil {
		return nil, err
	}
	usage := s.UsageOf(cfg, totals)
	return &usage, nil
}

// UsageOf returns a stored budget's spend in its current period from totals.
func (s *Service) UsageOf(cfg *models.StoredConfig, totals SpendTotals) PeriodUsage {
	req, _ := cfg.SourceConfiguration.(api.BudgetRequest)
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	start := budgetspend.PeriodStart(string(req.Spec.Limit.Period), now())
	usage := PeriodUsage{PeriodStart: start}
	prefix := budgetspend.CounterIDPrefix(cfg.Handle)
	window := start.Unix()
	byID := map[string]*UsageEntry{}
	row := func(id string) *UsageEntry {
		a := byID[id]
		if a == nil {
			a = &UsageEntry{Usage: map[string]string{}}
			byID[id] = a
		}
		return a
	}
	for _, t := range totals.spend {
		if t.Key.Window == window && strings.HasPrefix(t.Key.ID, prefix) {
			row(t.Key.ID).Spent += t.Total
		}
	}
	for _, t := range totals.unpriced {
		if t.Key.Window == window && strings.HasPrefix(t.Key.ID, prefix) {
			row(t.Key.ID).Unpriced += int64(t.Total)
		}
	}
	var labels map[string]map[string]string
	if s.labels != nil && len(byID) > 0 {
		labels = s.labels.Labels(cfg.Handle)
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		if values, ok := labels[id]; ok {
			byID[id].Usage = values
		}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := byID[ids[i]], byID[ids[j]]
		if a.Spent != b.Spent {
			return a.Spent > b.Spent
		}
		return ids[i] < ids[j]
	})
	for _, id := range ids {
		usage.Usages = append(usage.Usages, *byID[id])
	}
	return usage
}

// SetResourcePusher wires the push of gateway-created budgets to the control plane, the
// way gateway-created artifacts are pushed: each budget after it is created or updated,
// and a deletion when one is deleted. pushEnabled is deployment_sync_enabled.
func (s *Service) SetResourcePusher(pusher utils.ResourcePusher, pushEnabled bool) {
	s.pusher = pusher
	s.pushEnabled = pushEnabled
}

func (s *Service) canPush() bool {
	return s.pushEnabled && s.pusher != nil
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

// CreateParams holds the request for Create. Body is JSON or YAML, as ContentType says.
type CreateParams struct {
	Body          []byte
	ContentType   string
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

	budget, err := decode(params.Body, params.ContentType)
	if err != nil {
		return nil, err
	}
	if err := s.checkProviderExists(budget.Spec); err != nil {
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
	if artifactID(budget.Metadata) != "" {
		if err := s.checkIDFree(id); err != nil {
			return nil, err
		}
	}

	if err := s.db.SaveConfig(cfg); err != nil {
		if storage.IsConflictError(err) {
			return nil, fmt.Errorf("%w: a budget named %q already exists", storage.ErrConflict, budget.Metadata.Name)
		}
		return nil, fmt.Errorf("failed to save budget: %w", err)
	}

	log.Info("Budget created", slog.String("uuid", cfg.UUID), slog.String("handle", cfg.Handle))
	if s.canPush() {
		s.pusher.PushResource(cfg)
	}
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

// UpdateParams holds the request for Update. Body is the full budget, JSON or YAML, as
// ContentType says.
type UpdateParams struct {
	Name          string
	Body          []byte
	ContentType   string
	CorrelationID string
	Logger        *slog.Logger
}

// Update changes a gateway-created budget. Only status, the limit's amount,
// action and blockAt, and the exemptions can change; any other difference is
// a *ValidationError. A budget from the control plane is managed there and is
// read-only here (ErrReadOnly).
func (s *Service) Update(params UpdateParams) (*models.StoredConfig, error) {
	log := params.Logger
	if log == nil {
		log = s.logger
	}

	budget, err := decode(params.Body, params.ContentType)
	if err != nil {
		return nil, err
	}
	if budget.Metadata.Name != params.Name {
		return nil, &NameMismatchError{PathName: params.Name, BodyName: budget.Metadata.Name}
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
	if s.canPush() {
		s.pusher.PushResource(&updated)
	}
	return &updated, s.publish("UPDATE", updated.Handle, params.CorrelationID, log)
}

// Delete removes a gateway-created budget. A budget from the control plane is
// managed there and is read-only here (ErrReadOnly).
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
	if s.canPush() {
		s.pusher.PushResourceDeletion(existing)
	}
	// Policy engines first, so they stop charging it, then its spend.
	err = s.publish("DELETE", name, correlationID, log)
	s.clearSpend(name, log)
	return err
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

// checkIDFree rejects an artifact-id annotation that names an artifact already stored,
// of any kind: the database would refuse the row, and the conflict is the ID, not the name.
func (s *Service) checkIDFree(id string) error {
	existing, err := s.db.GetConfig(id)
	if err == nil && existing != nil {
		return fmt.Errorf("%w: artifact ID %q is already used by %s %q",
			storage.ErrConflict, id, existing.Kind, existing.Handle)
	}
	if err != nil && !storage.IsNotFoundError(err) {
		return fmt.Errorf("failed to check budget ID %q: %w", id, err)
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
			return "", &ValidationError{Errors: []config.ValidationError{{
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
	return utils.AnnotationValue(metadata.Annotations, constants.AnnotationArtifactID)
}

// fixedFieldChanges lists the fields that differ but cannot change after
// creation: the artifact-id annotation, period, splitBy and filters.
func fixedFieldChanges(stored, updated api.BudgetRequest) []config.ValidationError {
	const message = "cannot be changed after creation; create a new budget instead"
	var errs []config.ValidationError
	if artifactID(stored.Metadata) != artifactID(updated.Metadata) {
		errs = append(errs, config.ValidationError{Field: "metadata.annotations." + constants.AnnotationArtifactID, Message: message})
	}
	if stored.Spec.Limit.Period != updated.Spec.Limit.Period {
		errs = append(errs, config.ValidationError{Field: "spec.limit.period", Message: message})
	}
	if !reflect.DeepEqual(stored.Spec.Limit.SplitBy, updated.Spec.Limit.SplitBy) {
		errs = append(errs, config.ValidationError{Field: "spec.limit.splitBy", Message: message})
	}
	if !reflect.DeepEqual(stored.Spec.Filters, updated.Spec.Filters) {
		errs = append(errs, config.ValidationError{Field: "spec.filters", Message: message})
	}
	return errs
}

// checkProviderExists rejects a budget whose llmProvider filter names an LLM provider
// that is not on this gateway.
func (s *Service) checkProviderExists(spec api.BudgetSpec) error {
	if spec.Filters == nil {
		return nil
	}
	for i, f := range *spec.Filters {
		if f.Type != api.BudgetFilterTypeLlmProvider || f.Id == nil {
			continue
		}
		_, err := s.db.GetConfigByKindAndHandle(models.KindLlmProvider, *f.Id)
		switch {
		case err == nil:
		case storage.IsNotFoundError(err):
			return &ValidationError{Errors: []config.ValidationError{{
				Field:   fmt.Sprintf("spec.filters[%d].id", i),
				Message: fmt.Sprintf("no LLM provider named %q on this gateway", *f.Id),
			}}}
		default:
			return fmt.Errorf("failed to look up LLM provider %q: %w", *f.Id, err)
		}
	}
	return nil
}

// ApplyControlPlaneBudget stores a budget the control plane sent (budget.created,
// budget.updated, or the sync on connect) as a control-plane budget: read-only here and
// never pushed back. One whose updatedAt is not newer than the stored copy's (kept in
// DeployedAt) is ignored, so a late or replayed event never undoes a newer change.
func (s *Service) ApplyControlPlaneBudget(in utils.ControlPlaneBudget, correlationID string, log *slog.Logger) error {
	if log == nil {
		log = s.logger
	}
	if in.BudgetID == "" {
		return fmt.Errorf("control-plane budget has no budgetId")
	}
	budget, err := decode(in.Budget, "application/json")
	if err != nil {
		return fmt.Errorf("control-plane budget %s is not valid here: %w", in.BudgetID, err)
	}
	log = log.With(slog.String("uuid", in.BudgetID), slog.String("handle", budget.Metadata.Name))
	changedAt := in.UpdatedAt.UTC()

	existing, err := s.db.GetConfig(in.BudgetID)
	if err != nil && !storage.IsNotFoundError(err) {
		return fmt.Errorf("failed to read budget %s: %w", in.BudgetID, err)
	}
	if existing != nil && err == nil {
		if existing.Kind != models.KindBudget || existing.Origin != models.OriginControlPlane {
			return fmt.Errorf("%w: %s is this gateway's own %s", storage.ErrConflict, in.BudgetID, existing.Kind)
		}
		// The control plane's change time is kept in DeployedAt, as for the artifacts it
		// deploys: the database stamps updated_at itself.
		if existing.DeployedAt != nil && !in.UpdatedAt.After(*existing.DeployedAt) {
			log.Debug("Ignoring a control-plane budget no newer than the stored copy")
			return nil
		}
		updated := *existing
		updated.Configuration = *budget
		updated.SourceConfiguration = *budget
		updated.UpdatedAt = time.Now()
		updated.DeployedAt = &changedAt
		if err := s.db.UpdateConfig(&updated); err != nil {
			return fmt.Errorf("failed to update control-plane budget: %w", err)
		}
		log.Info("Control-plane budget updated")
		return s.publish("UPDATE", updated.Handle, correlationID, log)
	}

	cfg := &models.StoredConfig{
		UUID:                in.BudgetID,
		Kind:                models.KindBudget,
		Handle:              budget.Metadata.Name,
		DisplayName:         budget.Metadata.Name,
		Version:             budgetVersion,
		Configuration:       *budget,
		SourceConfiguration: *budget,
		DesiredState:        models.StateDeployed,
		Origin:              models.OriginControlPlane,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
		DeployedAt:          &changedAt,
	}
	if err := s.db.SaveConfig(cfg); err != nil {
		if storage.IsConflictError(err) {
			// The gateway has its own budget of that name: the control plane refused it
			// (BUDGET_ALREADY_EXISTS), so the two cannot both be here.
			return fmt.Errorf("%w: this gateway already has a budget named %q", storage.ErrConflict, cfg.Handle)
		}
		return fmt.Errorf("failed to save control-plane budget: %w", err)
	}
	log.Info("Control-plane budget created")
	return s.publish("CREATE", cfg.Handle, correlationID, log)
}

// DeleteControlPlaneBudget deletes a control-plane budget (budget.deleted, or one the
// sync on connect no longer lists). An unknown ID, or one of the gateway's own budgets,
// is left alone.
func (s *Service) DeleteControlPlaneBudget(budgetID, correlationID string, log *slog.Logger) error {
	if log == nil {
		log = s.logger
	}
	existing, err := s.db.GetConfig(budgetID)
	if err != nil {
		if storage.IsNotFoundError(err) {
			return nil
		}
		return fmt.Errorf("failed to read budget %s: %w", budgetID, err)
	}
	if existing.Kind != models.KindBudget || existing.Origin != models.OriginControlPlane {
		log.Warn("Not deleting: not a control-plane budget", slog.String("uuid", budgetID))
		return nil
	}
	if err := s.db.DeleteConfig(budgetID); err != nil && !storage.IsNotFoundError(err) {
		return fmt.Errorf("failed to delete control-plane budget: %w", err)
	}
	log.Info("Control-plane budget deleted", slog.String("uuid", budgetID), slog.String("handle", existing.Handle))
	// Policy engines first, so they stop charging it, then its spend.
	err = s.publish("DELETE", existing.Handle, correlationID, log)
	s.clearSpend(existing.Handle, log)
	return err
}

// ControlPlaneBudgetIDs lists the stored control-plane budgets.
func (s *Service) ControlPlaneBudgetIDs() ([]string, error) {
	configs, err := s.db.GetAllConfigsByKind(models.KindBudget)
	if err != nil {
		return nil, fmt.Errorf("failed to list budgets: %w", err)
	}
	ids := []string{}
	for _, cfg := range configs {
		if cfg.Origin == models.OriginControlPlane {
			ids = append(ids, cfg.UUID)
		}
	}
	return ids, nil
}
