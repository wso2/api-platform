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

package backend

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/clientca"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/upstreamguard"
)

var (
	// ErrValidation wraps every invalid-configuration error (see ValidationError).
	ErrValidation = errors.New("backend configuration invalid")
	// ErrNotFound is returned when the named Backend does not exist.
	ErrNotFound = errors.New("backend not found")
	// ErrInUse wraps a refused delete (see InUseError).
	ErrInUse = errors.New("backend is in use")
)

// ValidationError lists every problem found in a Backend configuration.
type ValidationError struct {
	Errors []config.ValidationError
}

func (e *ValidationError) Error() string {
	msgs := make([]string, 0, len(e.Errors))
	for _, v := range e.Errors {
		if v.Field == "" {
			msgs = append(msgs, v.Message)
			continue
		}
		msgs = append(msgs, v.Field+": "+v.Message)
	}
	return fmt.Sprintf("%s: %s", ErrValidation, strings.Join(msgs, "; "))
}

func (e *ValidationError) Unwrap() error { return ErrValidation }

// Reference is one deployed API that names a Backend as a fixed target.
type Reference struct {
	APIHandle string
	FieldPath string
}

// InUseError is a delete refused because deployed APIs name the Backend as a fixed target.
type InUseError struct {
	Name       string
	References []Reference
}

func (e *InUseError) Error() string {
	handles := make([]string, 0, len(e.References))
	seen := map[string]bool{}
	for _, ref := range e.References {
		if !seen[ref.APIHandle] {
			seen[ref.APIHandle] = true
			handles = append(handles, ref.APIHandle)
		}
	}
	return fmt.Sprintf("Backend '%s' is the fixed target of deployed APIs: %s", e.Name, strings.Join(handles, ", "))
}

func (e *InUseError) Unwrap() error { return ErrInUse }

// EventPublisher publishes replica-sync events.
type EventPublisher interface {
	PublishEvent(gatewayID string, event eventhub.Event) error
}

// DeployedRestAPIs returns every deployed RestApi configuration.
type DeployedRestAPIs func() ([]*models.StoredConfig, error)

// Service implements the Backend resource: validation, persistence, the in-use guard on delete,
// and propagation to this replica (immediately) and to the others (through the event hub).
type Service struct {
	db             storage.BackendStorage
	parser         *config.Parser
	validator      *config.BackendValidator
	routing        config.BackendRoutingIndex
	latestVersions map[string]string
	deployedAPIs   DeployedRestAPIs
	syncer         *Syncer
	events         EventPublisher
	gatewayID      string
	logger         *slog.Logger
	now            func() time.Time
}

// ServiceConfig carries the Service's collaborators.
type ServiceConfig struct {
	DB                storage.BackendStorage
	Validator         *config.BackendValidator
	PolicyDefinitions map[string]models.PolicyDefinition
	DeployedRestAPIs  DeployedRestAPIs
	Syncer            *Syncer
	Events            EventPublisher
	GatewayID         string
	Logger            *slog.Logger
}

// NewService creates a Backend service.
func NewService(cfg ServiceConfig) *Service {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	validator := cfg.Validator
	if validator == nil {
		validator = config.NewBackendValidator(nil)
	}
	return &Service{
		db:             cfg.DB,
		parser:         config.NewParser(),
		validator:      validator,
		routing:        config.BuildBackendRoutingIndex(cfg.PolicyDefinitions),
		latestVersions: config.BuildLatestVersionIndex(cfg.PolicyDefinitions),
		deployedAPIs:   cfg.DeployedRestAPIs,
		syncer:         cfg.Syncer,
		events:         cfg.Events,
		gatewayID:      cfg.GatewayID,
		logger:         logger,
		now:            time.Now,
	}
}

// Request is the payload of a create or update.
type Request struct {
	Body          []byte
	ContentType   string
	CorrelationID string
	Origin        models.Origin
}

func (s *Service) parseAndValidate(req Request) (*api.BackendRequest, error) {
	var cfg api.BackendRequest
	if err := s.parser.Parse(req.Body, req.ContentType, &cfg); err != nil {
		return nil, &ValidationError{Errors: []config.ValidationError{{Message: fmt.Sprintf("failed to parse Backend configuration: %v", err)}}}
	}
	if errs := s.validator.Validate(&cfg); len(errs) > 0 {
		return nil, &ValidationError{Errors: errs}
	}
	// The same network access control that applies to API upstream URLs applies to the
	// Backend's endpoints. Like the API path, the response names the refused field only.
	if err := templateengine.RunPostRenderCheck(string(models.KindBackend), &cfg); err != nil {
		if upstreamguard.IsRefused(err) {
			return nil, &ValidationError{Errors: []config.ValidationError{{
				Field:   upstreamguard.RefusedFieldPath(err),
				Message: "This URL is not permitted by the gateway's network access control.",
			}}}
		}
		return nil, fmt.Errorf("checking backend endpoints against network access control: %w", err)
	}
	return &cfg, nil
}

// Create validates and stores a new Backend, applies it on this replica, and tells the others.
func (s *Service) Create(req Request) (*models.StoredBackend, error) {
	cfg, err := s.parseAndValidate(req)
	if err != nil {
		return nil, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("failed to generate backend ID: %w", err)
	}
	stored := &models.StoredBackend{UUID: id.String(), Configuration: *cfg, Origin: req.Origin}
	if err := s.db.SaveBackend(stored); err != nil {
		return nil, err
	}
	s.propagate("CREATE", stored.UUID, req.CorrelationID)
	return stored, nil
}

// Update replaces an existing Backend's spec and labels. Its name cannot change.
func (s *Service) Update(name string, req Request) (*models.StoredBackend, error) {
	existing, err := s.Get(name)
	if err != nil {
		return nil, err
	}
	cfg, err := s.parseAndValidate(req)
	if err != nil {
		return nil, err
	}
	if cfg.Metadata.Name != name {
		return nil, &ValidationError{Errors: []config.ValidationError{{Field: "metadata.name",
			Message: fmt.Sprintf("metadata.name '%s' does not match the Backend '%s'; a Backend cannot be renamed", cfg.Metadata.Name, name)}}}
	}
	updated := &models.StoredBackend{
		UUID:          existing.UUID,
		Configuration: *cfg,
		Origin:        existing.Origin,
		CreatedAt:     existing.CreatedAt,
	}
	if err := s.db.UpdateBackend(updated); err != nil {
		if storage.IsNotFoundError(err) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
		}
		return nil, err
	}
	s.propagate("UPDATE", updated.UUID, req.CorrelationID)
	return updated, nil
}

// Delete removes a Backend unless a deployed API names it as a fixed target (and has no upstream
// definition of that name, which would take precedence). Requests that select it through
// backendExpression afterwards get 400 from the policy and are never misrouted.
func (s *Service) Delete(name, correlationID string) (*models.StoredBackend, error) {
	existing, err := s.Get(name)
	if err != nil {
		return nil, err
	}
	refs, err := s.References(name)
	if err != nil {
		return nil, fmt.Errorf("failed to verify backend references: %w", err)
	}
	if len(refs) > 0 {
		return nil, &InUseError{Name: name, References: refs}
	}
	if err := s.db.DeleteBackend(existing.UUID); err != nil {
		if storage.IsNotFoundError(err) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
		}
		return nil, err
	}
	s.propagate("DELETE", existing.UUID, correlationID)
	return existing, nil
}

// Get returns a Backend by name.
func (s *Service) Get(name string) (*models.StoredBackend, error) {
	backend, err := s.db.GetBackendByHandle(name)
	if err != nil {
		if storage.IsNotFoundError(err) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
		}
		return nil, err
	}
	return backend, nil
}

// List returns every Backend, ordered by name.
func (s *Service) List() ([]*models.StoredBackend, error) {
	return s.db.GetAllBackends()
}

// Warnings returns the non-fatal findings for a stored Backend, reported in the create and update
// responses: tls settings that weaken verification, an expired identity, or no trust source.
func (s *Service) Warnings(stored *models.StoredBackend) []clientca.Warning {
	if stored == nil {
		return nil
	}
	return s.validator.Warnings(&stored.Configuration)
}

// References returns every deployed RestApi policy parameter that names the Backend as a fixed
// target, at API or operation level. An API with an upstream definition of the same name is
// skipped: its fixed targets resolve to that definition, never to the Backend.
func (s *Service) References(name string) ([]Reference, error) {
	if s.deployedAPIs == nil {
		return nil, nil
	}
	configs, err := s.deployedAPIs()
	if err != nil {
		return nil, err
	}
	var refs []Reference
	for _, cfg := range configs {
		restCfg, ok := cfg.Configuration.(api.RestAPI)
		if !ok || definesUpstream(restCfg.Spec.UpstreamDefinitions, name) {
			continue
		}
		refs = append(refs, s.policyRefs(cfg.Handle, "spec.policies", restCfg.Spec.Policies, name)...)
		for i, op := range restCfg.Spec.Operations {
			refs = append(refs, s.policyRefs(cfg.Handle, fmt.Sprintf("spec.operations[%d].policies", i), op.Policies, name)...)
		}
	}
	sort.SliceStable(refs, func(i, j int) bool { return refs[i].APIHandle < refs[j].APIHandle })
	return refs, nil
}

// definesUpstream reports whether an API declares an upstream definition with this name.
func definesUpstream(definitions *[]api.UpstreamDefinition, name string) bool {
	if definitions == nil {
		return false
	}
	for _, def := range *definitions {
		if def.Name == name {
			return true
		}
	}
	return false
}

func (s *Service) policyRefs(apiHandle, prefix string, policies *[]api.Policy, name string) []Reference {
	if policies == nil {
		return nil
	}
	var refs []Reference
	for i, p := range *policies {
		if p.Params == nil {
			continue
		}
		version := strings.TrimSpace(p.Version)
		if version == "" {
			version = s.latestVersions[p.Name]
		}
		for _, ref := range s.routing.StaticBackendRefs(p.Name, version, *p.Params) {
			if ref.Backend == name {
				refs = append(refs, Reference{APIHandle: apiHandle,
					FieldPath: fmt.Sprintf("%s[%d].params.%s", prefix, i, ref.Param)})
			}
		}
	}
	return refs
}

// propagate applies a stored change on this replica right away, so the response reflects a live
// Backend, and publishes it so every other replica applies it too.
func (s *Service) propagate(action, uuid, correlationID string) {
	if correlationID == "" {
		correlationID = uuid
	}
	if s.syncer != nil {
		s.syncer.HandleBackendEvent(eventhub.Event{Action: action, EntityID: uuid, EventID: correlationID})
	}
	if s.events == nil {
		return
	}
	event := eventhub.Event{
		GatewayID:           s.gatewayID,
		OriginatedTimestamp: s.now(),
		EventType:           eventhub.EventTypeBackend,
		Action:              action,
		EntityID:            uuid,
		EventID:             correlationID,
		EventData:           eventhub.EmptyEventData,
	}
	if err := s.events.PublishEvent(s.gatewayID, event); err != nil {
		s.logger.Error("Failed to publish backend event",
			slog.String("action", action), slog.String("entity_id", uuid), slog.Any("error", err))
	}
}

// DeployedRestAPIsFrom lists the deployed RestApi configurations the delete guard checks. It
// reads the database, the source of truth, whenever there is one: an API is written there before
// any replica applies it, so nothing deployed is missed. The in-memory store converges from the
// database asynchronously and would still hold an API for a moment after it is deleted, refusing
// a delete that is safe. The store is read only when the controller runs without a database.
func DeployedRestAPIsFrom(db storage.Storage, store *storage.ConfigStore) DeployedRestAPIs {
	return func() ([]*models.StoredConfig, error) {
		var configs []*models.StoredConfig
		if db != nil {
			stored, err := db.GetAllConfigsByKind(models.KindRestApi)
			if err != nil {
				return nil, err
			}
			configs = stored
		} else if store != nil {
			configs = store.GetAllByKind(models.KindRestApi)
		}
		deployed := make([]*models.StoredConfig, 0, len(configs))
		for _, cfg := range configs {
			if cfg.DesiredState == models.StateDeployed {
				deployed = append(deployed, cfg)
			}
		}
		return deployed, nil
	}
}
