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

// Package agent implements the business logic behind the Agent (A2A) management
// API: parse, render, validate, persist, and announce.
//
// The control-plane (DP->CP) push is wired the same way as for the other kinds,
// but switched off — see ControlPlanePushSupported.
package agent

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	commonconstants "github.com/wso2/api-platform/common/constants"
	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine/funcs"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// CreateResult holds the result of a Create operation.
type CreateResult struct {
	StoredConfig *models.StoredConfig
	IsUpdate     bool
	// IsStale is true when a newer version of this artifact already existed in
	// the database, so nothing was written and no event was published.
	IsStale bool
}

// ListResult holds the result of a List operation.
type ListResult struct {
	Items []*models.StoredConfig
}

// GetResult holds the result of a GetByHandle operation.
type GetResult struct {
	Config *models.StoredConfig
}

// UpdateResult holds the result of an Update operation.
type UpdateResult struct {
	Config *models.StoredConfig
}

// DeleteResult holds the result of a Delete operation.
type DeleteResult struct {
	Handle string
	Config *models.StoredConfig
}

// ControlPlanePushSupported reports whether an Agent can be pushed to the
// control plane.
//
// It is false because the control plane has no Agent in its artifact model yet:
// a push would be a request it cannot serve, and every Agent would settle on
// cp_sync_status=failed. The push path below is wired regardless, so enabling it
// is this one constant plus whatever the control plane needs, rather than a new
// code path written under time pressure later.
//
// While it is false the gateway also leaves cp_sync_status unset for Agents,
// because "pending" would describe a sync that is never going to be attempted.
const ControlPlanePushSupported = false

// AgentService encapsulates business logic for Agent CRUD operations.
type AgentService struct {
	store          *storage.ConfigStore
	db             storage.Storage
	parser         *config.Parser
	validator      config.Validator
	logger         *slog.Logger
	eventHub       eventhub.EventHub
	secretResolver funcs.SecretResolver
	gatewayID      string

	// controlPlaneClient and deploymentPushEnabled drive the DP->CP push. Both
	// are set by SetControlPlanePusher; until then, and whenever
	// deploymentPushEnabled is false, nothing is pushed.
	controlPlaneClient    utils.ArtifactPusher
	deploymentPushEnabled bool
}

// SetControlPlanePusher wires the DP->CP push dependency. It is called on the
// instances that serve gateway-originated writes (the REST handlers' service and
// the immutable loader's service) and left unset elsewhere, so a
// control-plane-originated apply is never pushed back.
//
// pushEnabled must already account for ControlPlanePushSupported; callers pass
// the conjunction so the reason a push is off stays visible at the call site.
func (s *AgentService) SetControlPlanePusher(pusher utils.ArtifactPusher, pushEnabled bool) {
	s.controlPlaneClient = pusher
	s.deploymentPushEnabled = pushEnabled
}

// NewAgentService creates a new AgentService.
func NewAgentService(
	store *storage.ConfigStore,
	db storage.Storage,
	parser *config.Parser,
	validator config.Validator,
	logger *slog.Logger,
	eventHub eventhub.EventHub,
	secretResolver funcs.SecretResolver,
	gatewayID string,
) *AgentService {
	if db == nil {
		panic("AgentService requires non-nil storage")
	}
	if eventHub == nil {
		panic("AgentService requires non-nil EventHub")
	}
	if validator == nil {
		panic("AgentService requires non-nil validator")
	}
	trimmedGatewayID := strings.TrimSpace(gatewayID)
	if trimmedGatewayID == "" {
		panic("AgentService requires non-empty gateway ID")
	}
	if parser == nil {
		parser = config.NewParser()
	}

	return &AgentService{
		store:          store,
		db:             db,
		parser:         parser,
		validator:      validator,
		logger:         logger,
		eventHub:       eventHub,
		secretResolver: secretResolver,
		gatewayID:      trimmedGatewayID,
	}
}

// CreateParams holds parameters for the Create operation.
type CreateParams struct {
	Body          []byte
	ContentType   string
	CorrelationID string
	Origin        models.Origin
	// ID pins the artifact UUID. Empty means "resolve from the artifact-id
	// annotation, else generate one".
	ID           string
	DeploymentID string
	// DeployedAt stamps this deployment. The upsert is timestamp-guarded, so
	// this is what decides whether a concurrent write is a newer deployment or a
	// stale one. Defaults to now.
	DeployedAt *time.Time
	Logger     *slog.Logger
}

// Create deploys a new Agent configuration.
func (s *AgentService) Create(params CreateParams) (*CreateResult, error) {
	log := s.loggerFor(params.Logger)

	origin := params.Origin
	if origin == "" {
		origin = models.OriginGatewayAPI
	}
	if !models.IsValidOrigin(origin) {
		return nil, fmt.Errorf("invalid or missing origin: %q", origin)
	}

	agentConfig, err := s.parse(params.Body, params.ContentType)
	if err != nil {
		return nil, err
	}

	artifactID, err := s.resolveArtifactID(params.ID, agentConfig.Metadata.Annotations)
	if err != nil {
		return nil, err
	}

	isUpdate := false
	if existing, err := s.db.GetConfig(artifactID); err == nil && existing != nil {
		isUpdate = true
	} else if err != nil && !storage.IsNotFoundError(err) && !storage.IsDatabaseUnavailableError(err) {
		return nil, fmt.Errorf("failed to look up existing configuration: %w", err)
	}

	now := time.Now()
	deployedAt := params.DeployedAt
	if deployedAt == nil {
		truncated := now.Truncate(time.Millisecond)
		deployedAt = &truncated
	}

	storedCfg := &models.StoredConfig{
		UUID:                artifactID,
		Kind:                models.KindAgent,
		Handle:              agentConfig.Metadata.Name,
		Configuration:       *agentConfig,
		SourceConfiguration: *agentConfig,
		DesiredState:        desiredStateOf(agentConfig),
		DeploymentID:        params.DeploymentID,
		Origin:              origin,
		CPSyncStatus:        s.initialCPSyncStatus(origin),
		CreatedAt:           now,
		UpdatedAt:           now,
		DeployedAt:          deployedAt,
	}

	// Render before validation so the validator sees resolved values, then
	// validate the rendered form. Configuration ends up rendered;
	// SourceConfiguration stays as the user wrote it, and that is what the
	// database stores — each replica re-renders on consumption, so a rotated
	// secret is picked up without rewriting the artifact.
	rendered, err := s.renderAndValidate(storedCfg, log)
	if err != nil {
		return nil, err
	}

	if err := s.validateArtifactConflicts(artifactID, rendered.Spec.DisplayName, rendered.Spec.Version, storedCfg.Handle); err != nil {
		return nil, err
	}

	storedCfg.DisplayName = rendered.Spec.DisplayName
	storedCfg.Version = rendered.Spec.Version

	// Timestamp-guarded upsert: affected=false means a newer version of this
	// artifact is already stored, so this request lost a race and must not
	// publish an event that would make replicas converge backwards.
	affected, err := s.db.UpsertConfig(storedCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to persist agent configuration: %w", err)
	}
	if !affected {
		log.Debug("Skipped stale agent configuration (newer version exists in DB)",
			slog.String("agent_id", artifactID),
			slog.String("handle", storedCfg.Handle))
		return &CreateResult{StoredConfig: storedCfg, IsUpdate: isUpdate, IsStale: true}, nil
	}

	action := "CREATE"
	if isUpdate {
		action = "UPDATE"
	}
	s.publishEvent(action, artifactID, params.CorrelationID, log)

	s.pushDeployedArtifact(storedCfg, params.CorrelationID, log)

	log.Info("Agent configuration deployed",
		slog.String("agent_id", artifactID),
		slog.String("handle", storedCfg.Handle),
		slog.String("displayName", storedCfg.DisplayName),
		slog.String("version", storedCfg.Version),
		slog.String("action", action))

	return &CreateResult{StoredConfig: storedCfg, IsUpdate: isUpdate}, nil
}

// List returns Agent configurations, optionally filtered.
func (s *AgentService) List(params api.ListAgentsParams) (*ListResult, error) {
	configs, err := s.db.GetAllConfigsByKind(models.KindAgent)
	if err != nil {
		s.logger.Error("Failed to get agents", slog.Any("error", err))
		return nil, fmt.Errorf("failed to retrieve agent configurations")
	}

	items := make([]*models.StoredConfig, 0, len(configs))
	for _, cfg := range configs {
		if params.DisplayName != nil && *params.DisplayName != "" && cfg.DisplayName != *params.DisplayName {
			continue
		}
		if params.Version != nil && *params.Version != "" && cfg.Version != *params.Version {
			continue
		}
		if params.Context != nil && *params.Context != "" {
			cfgContext, err := cfg.GetContext()
			if err != nil {
				s.logger.Error("Failed to get context for agent config",
					slog.String("uuid", cfg.UUID), slog.Any("error", err))
				continue
			}
			if cfgContext != *params.Context {
				continue
			}
		}
		if params.Status != nil && *params.Status != "" && string(cfg.DesiredState) != string(*params.Status) {
			continue
		}

		items = append(items, cfg)
	}

	return &ListResult{Items: items}, nil
}

// GetByHandle retrieves an Agent by its handle from the database.
//
// The lookup is kind-scoped, so a handle owned by another artifact kind reads as
// absent from here rather than as a kind mismatch — the same behaviour the other
// per-kind endpoints have.
func (s *AgentService) GetByHandle(handle string) (*GetResult, error) {
	cfg, err := s.db.GetConfigByKindAndHandle(models.KindAgent, handle)
	if err != nil {
		if storage.IsDatabaseUnavailableError(err) {
			return nil, err
		}
		return nil, ErrNotFound
	}
	return &GetResult{Config: cfg}, nil
}

// UpdateParams holds parameters for the Update operation.
type UpdateParams struct {
	Handle        string
	Body          []byte
	ContentType   string
	CorrelationID string
	Logger        *slog.Logger
}

// Update modifies an existing Agent configuration.
func (s *AgentService) Update(params UpdateParams) (*UpdateResult, error) {
	log := s.loggerFor(params.Logger)

	agentConfig, err := s.parse(params.Body, params.ContentType)
	if err != nil {
		return nil, err
	}

	if agentConfig.Metadata.Name != "" && agentConfig.Metadata.Name != params.Handle {
		return nil, &HandleMismatchError{
			PathHandle: params.Handle,
			YAMLHandle: agentConfig.Metadata.Name,
		}
	}

	existing, err := s.db.GetConfigByKindAndHandle(models.KindAgent, params.Handle)
	if err != nil {
		if storage.IsDatabaseUnavailableError(err) {
			return nil, err
		}
		return nil, ErrNotFound
	}

	// A request that omits the write-only upstream credential inherits the
	// stored one, so an update authored from a GET response (which never
	// contains the credential) does not silently drop it.
	inheritUpstreamCredential(agentConfig, existing.SourceConfiguration)

	existing.Configuration = *agentConfig
	existing.SourceConfiguration = *agentConfig

	rendered, err := s.renderAndValidate(existing, log)
	if err != nil {
		return nil, err
	}

	if err := s.validateArtifactConflicts(existing.UUID, rendered.Spec.DisplayName, rendered.Spec.Version, existing.Handle); err != nil {
		return nil, err
	}

	desiredState := desiredStateOf(agentConfig)
	now := time.Now()
	truncatedNow := now.Truncate(time.Millisecond)
	existing.DisplayName = rendered.Spec.DisplayName
	existing.Version = rendered.Spec.Version
	existing.DesiredState = desiredState
	existing.UpdatedAt = now
	existing.DeployedAt = &truncatedNow

	if err := s.db.UpdateConfig(existing); err != nil {
		log.Error("Failed to update agent config in database", slog.Any("error", err))
		return nil, fmt.Errorf("failed to persist configuration update: %w", err)
	}

	s.publishEvent("UPDATE", existing.UUID, params.CorrelationID, log)
	s.pushDeployedArtifact(existing, params.CorrelationID, log)

	log.Info("Agent configuration updated",
		slog.String("agent_id", existing.UUID),
		slog.String("handle", params.Handle),
		slog.String("desired_state", string(desiredState)))

	return &UpdateResult{Config: existing}, nil
}

// DeleteParams holds parameters for the Delete operation.
type DeleteParams struct {
	Handle        string
	CorrelationID string
	Logger        *slog.Logger
}

// Delete removes an Agent configuration.
func (s *AgentService) Delete(params DeleteParams) (*DeleteResult, error) {
	log := s.loggerFor(params.Logger)

	cfg, err := s.db.GetConfigByKindAndHandle(models.KindAgent, params.Handle)
	if err != nil {
		if storage.IsDatabaseUnavailableError(err) {
			return nil, err
		}
		return nil, ErrNotFound
	}

	if err := s.db.DeleteConfig(cfg.UUID); err != nil {
		log.Error("Failed to delete agent config from database", slog.Any("error", err))
		return nil, fmt.Errorf("failed to delete configuration: %w", err)
	}

	// Delete, not undeploy: an undeployed Agent keeps its configuration so it
	// can be redeployed, and revoking its keys there would cut off every client
	// for what is meant to be a reversible operation.
	if err := s.db.RemoveAPIKeysAPI(cfg.UUID); err != nil {
		log.Warn("Failed to remove API keys from database",
			slog.String("handle", params.Handle),
			slog.Any("error", err))
	}

	s.publishEvent("DELETE", cfg.UUID, params.CorrelationID, log)
	s.pushArtifactUndeploy(cfg, log)

	log.Info("Agent configuration deleted",
		slog.String("agent_id", cfg.UUID),
		slog.String("handle", params.Handle))

	return &DeleteResult{Handle: params.Handle, Config: cfg}, nil
}

// parse reads the request payload into an AgentConfiguration and rejects a
// payload that declares a different kind. Without that check a POST to /agents
// carrying `kind: Mcp` would be stored under the wrong kind and then be
// invisible to every Agent read path.
func (s *AgentService) parse(body []byte, contentType string) (*api.AgentConfiguration, error) {
	var agentConfig api.AgentConfiguration
	if err := s.parser.Parse(body, contentType, &agentConfig); err != nil {
		return nil, &ParseError{Cause: err}
	}
	if agentConfig.Kind != "" && string(agentConfig.Kind) != models.KindAgent {
		return nil, &KindMismatchError{Kind: string(agentConfig.Kind)}
	}
	agentConfig.Kind = api.AgentConfigurationKindAgent
	return &agentConfig, nil
}

// renderAndValidate renders cfg.Configuration in place and validates the result,
// returning the rendered configuration. cfg.SourceConfiguration is left as
// supplied.
func (s *AgentService) renderAndValidate(cfg *models.StoredConfig, log *slog.Logger) (*api.AgentConfiguration, error) {
	if err := templateengine.RenderSpec(cfg, s.secretResolver, log); err != nil {
		return nil, err
	}

	rendered, ok := cfg.Configuration.(api.AgentConfiguration)
	if !ok {
		return nil, fmt.Errorf("unexpected configuration type %T after rendering agent", cfg.Configuration)
	}

	if validationErrors := s.validator.Validate(&rendered); len(validationErrors) > 0 {
		for i, e := range validationErrors {
			log.Warn("Agent validation error",
				slog.Int("index", i+1),
				slog.String("field", e.Field),
				slog.String("message", e.Message))
		}
		return nil, &ValidationError{Errors: validationErrors}
	}

	// Write the validated value back: a validator may coerce rendered-template
	// strings into their schema-declared types, and those coercions have to
	// reach the stored configuration rather than staying in a local copy.
	cfg.Configuration = rendered
	return &rendered, nil
}

// resolveArtifactID resolves the artifact UUID: explicit id, then the
// artifact-id annotation, then a fresh UUID.
//
// Nothing supplies either input for an Agent today — the gateway REST handler
// and the file loader both leave them empty, so every create mints a new UUID.
// They exist for the caller that owns an artifact's identity elsewhere and needs
// this gateway to store it under that id: the control-plane apply path does
// exactly that for the other kinds (see CreateMCPProxyFromYAML), and an Agent
// gains the same path when the control plane models the kind.
//
// Note what a pinned id means on a create: the row already exists, so the write
// updates it rather than colliding on handle. That is the intended behaviour for
// a caller replaying an artifact it owns, and the reason the pin is not taken
// from anywhere a request body reaches by default.
func (s *AgentService) resolveArtifactID(id string, annotations *map[string]string) (string, error) {
	if id != "" {
		return id, nil
	}
	if annotations != nil {
		if annotated := (*annotations)[commonconstants.AnnotationArtifactID]; annotated != "" {
			if err := utils.ValidateUUIDFormat(annotated); err != nil {
				return "", fmt.Errorf("invalid %s annotation: %w", commonconstants.AnnotationArtifactID, err)
			}
			return annotated, nil
		}
	}
	generated, err := utils.GenerateUUID()
	if err != nil {
		return "", fmt.Errorf("failed to generate agent ID: %w", err)
	}
	return generated, nil
}

// validateArtifactConflicts rejects a display-name/version or handle collision
// with a different Agent. The database enforces both as unique constraints; this
// turns the constraint violation into a 409 with a message naming the clash.
func (s *AgentService) validateArtifactConflicts(currentID, displayName, version, handle string) error {
	existingByNameVersion, err := s.db.GetConfigByKindNameAndVersion(models.KindAgent, displayName, version)
	if err == nil {
		if existingByNameVersion != nil && existingByNameVersion.UUID != currentID {
			return fmt.Errorf("%w: configuration with name '%s' and version '%s' already exists",
				storage.ErrConflict, displayName, version)
		}
	} else if !storage.IsNotFoundError(err) {
		return fmt.Errorf("failed to check existing Agent name/version conflict: %w", err)
	}

	if handle == "" {
		return nil
	}

	existingByHandle, err := s.db.GetConfigByKindAndHandle(models.KindAgent, handle)
	if err == nil {
		if existingByHandle != nil && existingByHandle.UUID != currentID {
			return fmt.Errorf("%w: configuration with handle '%s' already exists",
				storage.ErrConflict, handle)
		}
	} else if !storage.IsNotFoundError(err) {
		return fmt.Errorf("failed to check existing Agent handle conflict: %w", err)
	}

	return nil
}

// publishEvent announces the change so every replica converges through the
// event listener, including the one that made it.
func (s *AgentService) publishEvent(action, entityID, correlationID string, logger *slog.Logger) {
	event := eventhub.Event{
		GatewayID:           s.gatewayID,
		OriginatedTimestamp: time.Now(),
		EventType:           eventhub.EventTypeAPI,
		Action:              action,
		EntityID:            entityID,
		EventID:             correlationID,
		EventData:           eventhub.EmptyEventData,
	}

	if err := s.eventHub.PublishEvent(s.gatewayID, event); err != nil {
		logger.Warn("Failed to publish agent event to event hub",
			slog.String("gateway_id", s.gatewayID),
			slog.String("action", action),
			slog.String("entity_id", entityID),
			slog.Any("error", err))
		return
	}
	logger.Debug("Published agent event to event hub",
		slog.String("gateway_id", s.gatewayID),
		slog.String("action", action),
		slog.String("entity_id", entityID))
}

// initialCPSyncStatus returns the cp_sync_status a newly written Agent row
// starts with: "pending" for a gateway-originated artifact that this gateway
// will try to push, and unset otherwise.
//
// It keys off deploymentPushEnabled rather than live connectivity, so a
// temporarily disconnected gateway still records that a push is owed — but a
// gateway that will never push (see ControlPlanePushSupported) does not leave
// rows describing a sync that is not coming.
func (s *AgentService) initialCPSyncStatus(origin models.Origin) models.CPSyncStatus {
	if origin == models.OriginGatewayAPI && s.deploymentPushEnabled {
		return models.CPSyncStatusPending
	}
	return ""
}

// canPushToControlPlane reports whether a DP->CP push should be attempted now.
func (s *AgentService) canPushToControlPlane() bool {
	return s.deploymentPushEnabled && s.controlPlaneClient != nil &&
		s.controlPlaneClient.IsConnected() && !s.controlPlaneClient.IsOnPrem()
}

// pushDeployedArtifact announces a gateway-originated Agent to the control plane
// once the deployment it belongs to has landed.
//
// Only gateway-originated artifacts are pushed: a control-plane-originated apply
// arriving here would otherwise be echoed straight back to its author.
func (s *AgentService) pushDeployedArtifact(cfg *models.StoredConfig, correlationID string, log *slog.Logger) {
	if cfg == nil || cfg.Origin != models.OriginGatewayAPI || !s.canPushToControlPlane() {
		return
	}
	if s.store == nil {
		log.Warn("Skipping control plane push: no config store to observe deployment completion",
			slog.String("agent_id", cfg.UUID))
		return
	}

	pusher := s.controlPlaneClient
	store := s.store
	cfgID := cfg.UUID
	deployedAt := cfg.DeployedAt
	pusher.SubmitArtifactPush(func() {
		utils.WaitForDeploymentAndPush(store, pusher, cfgID, correlationID, deployedAt, log)
	})
}

// pushArtifactUndeploy tells the control plane a gateway-originated Agent is
// gone. The artifact is pushed as undeployed rather than deleted, matching how
// every other kind reports a local delete upstream.
func (s *AgentService) pushArtifactUndeploy(cfg *models.StoredConfig, log *slog.Logger) {
	if cfg == nil || cfg.Origin != models.OriginGatewayAPI || !s.canPushToControlPlane() {
		return
	}

	undeploy := *cfg
	undeploy.DesiredState = models.StateUndeployed
	pusher := s.controlPlaneClient
	pusher.SubmitArtifactPush(func() {
		uc := undeploy
		if err := pusher.PushArtifact(uc.UUID, &uc, uc.DeploymentID); err != nil {
			log.Error("Failed to push agent undeploy to control plane",
				slog.String("agent_id", uc.UUID), slog.Any("error", err))
		}
	})
}

func (s *AgentService) loggerFor(log *slog.Logger) *slog.Logger {
	if log != nil {
		return log
	}
	return s.logger
}

// desiredStateOf reads the requested deployment state, defaulting to deployed.
func desiredStateOf(cfg *api.AgentConfiguration) models.DesiredState {
	if cfg.Spec.DeploymentState != nil && *cfg.Spec.DeploymentState == api.AgentConfigDataDeploymentStateUndeployed {
		return models.StateUndeployed
	}
	return models.StateDeployed
}

// inheritUpstreamCredential copies the stored upstream credential onto an
// incoming configuration that does not carry one.
//
// `auth.value` is write-only: it never appears in a management API response, so
// a client that reads an Agent, edits it, and PUTs it back has no credential to
// send. Treating that as "remove the credential" would break the upstream on an
// unrelated edit. Setting `type: none` is how a credential is actually removed.
func inheritUpstreamCredential(incoming *api.AgentConfiguration, storedSource any) {
	if incoming == nil || incoming.Spec.Upstream.Auth == nil {
		return
	}
	if incoming.Spec.Upstream.Auth.Value != nil && *incoming.Spec.Upstream.Auth.Value != "" {
		return
	}
	if incoming.Spec.Upstream.Auth.Type == api.AgentConfigDataUpstreamAuthTypeNone {
		return
	}

	stored, ok := storedSource.(api.AgentConfiguration)
	if !ok {
		return
	}
	if stored.Spec.Upstream.Auth == nil || stored.Spec.Upstream.Auth.Value == nil {
		return
	}
	inherited := *stored.Spec.Upstream.Auth.Value
	if inherited == "" {
		return
	}
	incoming.Spec.Upstream.Auth.Value = &inherited
}
