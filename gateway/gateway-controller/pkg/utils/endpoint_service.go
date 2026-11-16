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

package utils

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

var (
	// ErrEndpointNotFound is returned when a gateway-wide Endpoint does not exist.
	ErrEndpointNotFound = errors.New("endpoint not found")

	// ErrEndpointValidation is returned when an Endpoint fails parsing or validation.
	ErrEndpointValidation = errors.New("endpoint configuration invalid")

	// ErrEndpointInUse is returned when an Endpoint cannot be deleted because a
	// deployed API names it as the fixed target of a policy.
	ErrEndpointInUse = errors.New("endpoint in use")
)

// endpointSyncTimeout bounds one reconcile of the router snapshot and the
// policy engine's lazy resources after an Endpoint change.
const endpointSyncTimeout = 10 * time.Second

// EndpointParams carries one Endpoint create or update request.
type EndpointParams struct {
	Spec          []byte       // Raw configuration (YAML or JSON)
	ContentType   string       // Content type for parsing
	CorrelationID string       // Correlation ID for tracking
	Logger        *slog.Logger // Logger
}

// EndpointSyncer applies the Endpoints in the database to this replica.
type EndpointSyncer interface {
	Sync(ctx context.Context, correlationID string) error
}

// EndpointEventPublisher announces an Endpoint change to the other replicas.
type EndpointEventPublisher interface {
	PublishEvent(gatewayID string, event eventhub.Event) error
}

// EndpointService creates, updates and deletes gateway-wide Endpoints.
//
// An Endpoint is never bound to an API here. Adding, changing or removing one
// changes no API's stored configuration and redeploys nothing: a change to its
// backend replaces only its own cluster, and a change to its base path or
// labels only the policy engine's copy of it.
type EndpointService struct {
	db        storage.Storage
	store     *storage.ConfigStore
	syncer    EndpointSyncer
	events    EndpointEventPublisher
	gatewayID string
	parser    *config.Parser
	validator *config.EndpointValidator
	routing   *config.EndpointRouting
	logger    *slog.Logger
}

// NewEndpointService creates the service. validator checks an Endpoint before
// it is stored; routing reads which deployed APIs name an Endpoint as a fixed
// target, for the delete guard. events may be nil on a controller without
// replica sync.
func NewEndpointService(
	db storage.Storage,
	store *storage.ConfigStore,
	syncer EndpointSyncer,
	events EndpointEventPublisher,
	gatewayID string,
	validator *config.EndpointValidator,
	routing *config.EndpointRouting,
	logger *slog.Logger,
) *EndpointService {
	if db == nil {
		panic("EndpointService requires non-nil storage")
	}
	if syncer == nil {
		panic("EndpointService requires a non-nil syncer")
	}
	if validator == nil {
		validator = config.NewEndpointValidator(nil)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &EndpointService{
		db:        db,
		store:     store,
		syncer:    syncer,
		events:    events,
		gatewayID: gatewayID,
		parser:    config.NewParser(),
		validator: validator,
		routing:   routing,
		logger:    logger,
	}
}

func (s *EndpointService) log(params EndpointParams) *slog.Logger {
	if params.Logger != nil {
		return params.Logger
	}
	return s.logger
}

// parseAndValidate parses a request body into an Endpoint and validates it.
func (s *EndpointService) parseAndValidate(params EndpointParams) (*api.EndpointConfiguration, error) {
	var endpoint api.EndpointConfiguration
	if err := s.parser.Parse(params.Spec, params.ContentType, &endpoint); err != nil {
		return nil, fmt.Errorf("%w: failed to parse endpoint configuration: %v", ErrEndpointValidation, err)
	}
	// Status is server-managed; whatever a client sent is dropped.
	endpoint.Status = nil

	validationErrors := s.validator.Validate(&endpoint)
	if len(validationErrors) > 0 {
		messages := make([]string, 0, len(validationErrors))
		for i, e := range validationErrors {
			messages = append(messages, fmt.Sprintf("%d. %s: %s", i+1, e.Field, e.Message))
		}
		s.log(params).Warn("Endpoint validation failed",
			slog.String("endpoint", endpoint.Metadata.Name), slog.Int("error_count", len(validationErrors)))
		return nil, fmt.Errorf("%w: %d error(s): %s", ErrEndpointValidation, len(validationErrors), strings.Join(messages, "; "))
	}
	return &endpoint, nil
}

// Create validates and stores a new Endpoint, makes it routable on this
// replica, and announces it to the others.
func (s *EndpointService) Create(params EndpointParams) (*models.StoredEndpoint, error) {
	endpoint, err := s.parseAndValidate(params)
	if err != nil {
		return nil, err
	}
	name := endpoint.Metadata.Name

	if existing, err := s.db.GetEndpointByHandle(name); err == nil && existing != nil {
		return nil, fmt.Errorf("%w: endpoint with name '%s' already exists", storage.ErrConflict, name)
	} else if err != nil && !storage.IsNotFoundError(err) {
		return nil, fmt.Errorf("failed to check for an existing endpoint: %w", err)
	}

	id, err := GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate endpoint ID: %w", err)
	}
	now := time.Now()
	stored := &models.StoredEndpoint{UUID: id, Configuration: *endpoint, CreatedAt: now, UpdatedAt: now}

	if err := s.db.SaveEndpoint(stored); err != nil {
		if storage.IsConflictError(err) {
			return nil, fmt.Errorf("%w: endpoint with name '%s' already exists", storage.ErrConflict, name)
		}
		return nil, fmt.Errorf("failed to save endpoint to database: %w", err)
	}

	s.apply("CREATE", stored.UUID, params.CorrelationID, s.log(params))
	return stored, nil
}

// Update replaces the configuration of the Endpoint called name. The name is
// the Endpoint's identity — its cluster name and what requests select it by —
// so it cannot change.
func (s *EndpointService) Update(name string, params EndpointParams) (*models.StoredEndpoint, error) {
	existing, err := s.Get(name)
	if err != nil {
		return nil, err
	}

	endpoint, err := s.parseAndValidate(params)
	if err != nil {
		return nil, err
	}
	if endpoint.Metadata.Name != existing.GetHandle() {
		return nil, fmt.Errorf("%w: cannot change endpoint name from '%s' to '%s'; create a new endpoint instead",
			ErrEndpointValidation, existing.GetHandle(), endpoint.Metadata.Name)
	}

	updated := &models.StoredEndpoint{
		UUID:          existing.UUID,
		Configuration: *endpoint,
		CreatedAt:     existing.CreatedAt,
		UpdatedAt:     time.Now(),
	}
	if err := s.db.UpdateEndpoint(updated); err != nil {
		if storage.IsNotFoundError(err) {
			return nil, fmt.Errorf("%w: name=%s", ErrEndpointNotFound, name)
		}
		return nil, fmt.Errorf("failed to update endpoint in database: %w", err)
	}

	s.apply("UPDATE", updated.UUID, params.CorrelationID, s.log(params))
	return updated, nil
}

// Delete removes the Endpoint called name. It is refused with ErrEndpointInUse
// while a deployed API names the Endpoint as the fixed target of a policy.
//
// Selection by request header cannot be guarded the same way, because it
// happens at request time. It does not need to be: once the Endpoint is gone a
// request that names it is rejected and is never sent to another backend.
func (s *EndpointService) Delete(name, correlationID string, logger *slog.Logger) (*models.StoredEndpoint, error) {
	if logger == nil {
		logger = s.logger
	}
	existing, err := s.Get(name)
	if err != nil {
		return nil, err
	}

	users, err := s.StaticallyTargetingAPIs(existing.GetHandle())
	if err != nil {
		return nil, fmt.Errorf("failed to check endpoint references: %w", err)
	}
	if len(users) > 0 {
		return nil, fmt.Errorf("%w: Endpoint '%s' is the fixed target of deployed APIs: %s",
			ErrEndpointInUse, existing.GetHandle(), strings.Join(users, ", "))
	}

	if err := s.db.DeleteEndpoint(existing.UUID); err != nil {
		if storage.IsNotFoundError(err) {
			return nil, fmt.Errorf("%w: name=%s", ErrEndpointNotFound, name)
		}
		return nil, fmt.Errorf("failed to delete endpoint from database: %w", err)
	}

	s.apply("DELETE", existing.UUID, correlationID, logger)
	return existing, nil
}

// apply makes a committed change take effect on this replica and announces it
// to the others. The change is already durable, so a failure here is logged
// rather than returned: every replica, this one included, reconciles from the
// database again when the event arrives.
func (s *EndpointService) apply(action, endpointID, correlationID string, logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), endpointSyncTimeout)
	defer cancel()
	if err := s.syncer.Sync(ctx, correlationID); err != nil {
		logger.Error("Failed to apply endpoint change on this replica",
			slog.String("action", action), slog.String("endpoint_id", endpointID), slog.Any("error", err))
	}

	if s.events == nil {
		return
	}
	event := eventhub.Event{
		GatewayID:           s.gatewayID,
		OriginatedTimestamp: time.Now(),
		EventType:           eventhub.EventTypeEndpoint,
		Action:              action,
		EntityID:            endpointID,
		EventID:             correlationID,
		EventData:           eventhub.EmptyEventData,
	}
	if err := s.events.PublishEvent(s.gatewayID, event); err != nil {
		logger.Error("Failed to publish endpoint event",
			slog.String("action", action), slog.String("endpoint_id", endpointID), slog.Any("error", err))
	}
}

// Get returns the Endpoint called name, or ErrEndpointNotFound.
func (s *EndpointService) Get(name string) (*models.StoredEndpoint, error) {
	endpoint, err := s.db.GetEndpointByHandle(name)
	if err == nil {
		return endpoint, nil
	}
	if storage.IsNotFoundError(err) {
		return nil, fmt.Errorf("%w: name=%s", ErrEndpointNotFound, name)
	}
	if storage.IsDatabaseUnavailableError(err) && s.store != nil {
		if cached, ok := s.store.GetEndpoint(name); ok {
			return cached, nil
		}
		return nil, fmt.Errorf("%w: name=%s", ErrEndpointNotFound, name)
	}
	return nil, fmt.Errorf("failed to get endpoint '%s': %w", name, err)
}

// List returns every Endpoint, ordered by name.
func (s *EndpointService) List() ([]*models.StoredEndpoint, error) {
	endpoints, err := s.db.GetAllEndpoints()
	if err != nil {
		if storage.IsDatabaseUnavailableError(err) && s.store != nil {
			return s.store.GetAllEndpoints(), nil
		}
		return nil, fmt.Errorf("failed to list endpoints: %w", err)
	}
	sort.Slice(endpoints, func(i, j int) bool { return endpoints[i].GetHandle() < endpoints[j].GetHandle() })
	return endpoints, nil
}

// StaticallyTargetingAPIs returns the handles of the deployed APIs that name
// the Endpoint as a fixed target: in a parameter its policy definition
// annotates x-wso2-endpoint-routing: endpointName, at the API level or on an
// operation.
//
// The in-memory store converges from the database asynchronously, so a
// reference found in either counts as live.
func (s *EndpointService) StaticallyTargetingAPIs(endpointName string) ([]string, error) {
	if s.routing == nil {
		return nil, nil
	}

	deployed := make(map[string]*models.StoredConfig)
	configs, err := s.db.GetAllConfigsByKind(models.KindRestApi)
	if err != nil && !storage.IsDatabaseUnavailableError(err) {
		return nil, err
	}
	for _, cfg := range configs {
		if cfg.DesiredState == models.StateDeployed {
			deployed[cfg.UUID] = cfg
		}
	}
	if s.store != nil {
		for _, cfg := range s.store.GetAllByKind(models.KindRestApi) {
			if cfg.DesiredState != models.StateDeployed {
				continue
			}
			if _, ok := deployed[cfg.UUID]; !ok {
				deployed[cfg.UUID] = cfg
			}
		}
	}

	var users []string
	for _, cfg := range deployed {
		restAPI, ok := restAPIOf(cfg)
		if !ok {
			continue
		}
		if restAPITargetsEndpoint(s.routing, restAPI, endpointName) {
			users = append(users, cfg.Handle)
		}
	}
	sort.Strings(users)
	return users, nil
}

// restAPIOf returns the RestAPI a stored configuration holds. The source
// configuration is what the user wrote, so it is preferred.
func restAPIOf(cfg *models.StoredConfig) (api.RestAPI, bool) {
	for _, candidate := range []any{cfg.SourceConfiguration, cfg.Configuration} {
		switch typed := candidate.(type) {
		case api.RestAPI:
			return typed, true
		case *api.RestAPI:
			if typed != nil {
				return *typed, true
			}
		}
	}
	return api.RestAPI{}, false
}

// restAPITargetsEndpoint reports whether any API-level or operation-level
// policy of restAPI names endpointName as a fixed target.
func restAPITargetsEndpoint(routing *config.EndpointRouting, restAPI api.RestAPI, endpointName string) bool {
	names := func(policies *[]api.Policy) bool {
		for _, target := range routing.StaticTargets(policies) {
			if target == endpointName {
				return true
			}
		}
		return false
	}
	if names(restAPI.Spec.Policies) {
		return true
	}
	for _, op := range restAPI.Spec.Operations {
		if names(op.Policies) {
			return true
		}
	}
	return false
}

// MissingStaticTargets returns, sorted, the Endpoints restAPI names as fixed
// targets that the gateway does not have. Deploying such an API is accepted —
// the routes answer 503 until the Endpoint exists and recover with no redeploy
// — so no deploy order is ever required; the caller logs a warning.
func MissingStaticTargets(routing *config.EndpointRouting, store *storage.ConfigStore, restAPI api.RestAPI) []string {
	if routing == nil || store == nil {
		return nil
	}
	missing := make(map[string]struct{})
	note := func(policies *[]api.Policy) {
		for _, target := range routing.StaticTargets(policies) {
			if _, ok := store.GetEndpoint(target); !ok {
				missing[target] = struct{}{}
			}
		}
	}
	note(restAPI.Spec.Policies)
	for _, op := range restAPI.Spec.Operations {
		note(op.Policies)
	}
	names := make([]string, 0, len(missing))
	for name := range missing {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
