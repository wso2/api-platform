/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine/funcs"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

const (
	LATEST_SUPPORTED_MCP_SPEC_VERSION = "2025-06-18"
)

var ErrMCPDeploymentIDMismatch = errors.New("mcp proxy deployment id mismatch")
var ErrMCPUndeployStale = errors.New("mcp proxy undeploy skipped: newer version exists")

type MCPDeploymentParams struct {
	Data          []byte              // Raw configuration data (YAML/JSON)
	ContentType   string              // Content type for parsing
	ID            string              // ID (if provided, used for updates; if empty, generates new UUID)
	DeploymentID  string              // Platform deployment ID (empty for gateway-api origin)
	Origin        models.Origin       // Origin of the deployment: "control_plane" or "gateway_api"
	DeployedAt    *time.Time          // Deployment timestamp from platform event (nil for gateway-api origin)
	CorrelationID string              // Correlation ID for tracking
	Logger        *slog.Logger        // Logger instance
	DesiredState  models.DesiredState // Desired deployment state; empty defaults to StateDeployed
}

// MCPDeploymentService provides utilities for MCP proxy configuration deployment
type MCPDeploymentService struct {
	store           *storage.ConfigStore
	db              storage.Storage
	snapshotManager *xds.SnapshotManager
	parser          *config.Parser
	validator       *config.MCPValidator
	transformer     Transformer
	policyManager   *policyxds.PolicyManager
	eventHub        eventhub.EventHub
	gatewayID       string
	secretResolver  funcs.SecretResolver
}

// NewMCPDeploymentService creates a new MCP deployment service
func NewMCPDeploymentService(
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
	policyManager *policyxds.PolicyManager,
	policyValidator *config.PolicyValidator,
	eventHub eventhub.EventHub,
	gatewayID string,
	secretResolver funcs.SecretResolver,
) *MCPDeploymentService {
	if db == nil {
		panic("MCPDeploymentService requires non-nil storage")
	}
	trimmedGatewayID := requireReplicaSyncWiring("MCPDeploymentService", eventHub, gatewayID)

	return &MCPDeploymentService{
		store:           store,
		db:              db,
		snapshotManager: snapshotManager,
		parser:          config.NewParser(),
		validator:       config.NewMCPValidator().WithPolicyValidator(policyValidator),
		transformer:     NewMCPTransformer(),
		policyManager:   policyManager,
		eventHub:        eventHub,
		gatewayID:       trimmedGatewayID,
		secretResolver:  secretResolver,
	}
}

// HydrateStoredMCPConfig rebuilds the derived RestAPI form for a stored MCP
// configuration from its canonical source document.
func HydrateStoredMCPConfig(cfg *models.StoredConfig) error {
	if cfg == nil {
		return nil
	}

	if source, ok := cfg.SourceConfiguration.(api.MCPProxyConfiguration); ok {
		var restAPI api.RestAPI
		if _, err := NewMCPTransformer().Transform(&source, &restAPI); err != nil {
			return fmt.Errorf("failed to transform stored MCP proxy %s: %w", cfg.UUID, err)
		}
		cfg.Configuration = restAPI
		return nil
	}

	if _, ok := cfg.Configuration.(api.RestAPI); ok {
		return nil
	}

	return fmt.Errorf("unexpected MCP source configuration type %T", cfg.SourceConfiguration)
}

func (s *MCPDeploymentService) publishMCPProxyEvent(action, entityID, correlationID string, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}

	event := eventhub.Event{
		GatewayID:           s.gatewayID,
		OriginatedTimestamp: time.Now(),
		EventType:           eventhub.EventTypeMCPProxy,
		Action:              action,
		EntityID:            entityID,
		EventID:             correlationID,
		EventData:           eventhub.EmptyEventData,
	}
	if err := s.eventHub.PublishEvent(s.gatewayID, event); err != nil {
		logger.Error("Failed to publish MCP proxy event",
			slog.String("action", action),
			slog.String("entity_id", entityID),
			slog.Any("error", err))
	}
}

func isMCPNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return storage.IsNotFoundError(err)
}

func (s *MCPDeploymentService) hydrateStoredMCPConfig(cfg *models.StoredConfig) {
	if err := HydrateStoredMCPConfig(cfg); err != nil {
		configID := ""
		if cfg != nil {
			configID = cfg.UUID
		}
		slog.Default().Warn("failed to hydrate StoredConfig",
			slog.String("id", configID),
			slog.Any("error", err))
	}
}

func (s *MCPDeploymentService) getMCPProxyByID(id string) (*models.StoredConfig, error) {
	cfg, err := s.db.GetConfig(id)
	if err != nil {
		return nil, err
	}
	s.hydrateStoredMCPConfig(cfg)
	return cfg, nil
}

// DeployMCPConfiguration handles the complete MCP configuration deployment process
func (s *MCPDeploymentService) DeployMCPConfiguration(params MCPDeploymentParams) (*APIDeploymentResult, error) {
	if !models.IsValidOrigin(params.Origin) {
		return nil, fmt.Errorf("invalid or missing origin: %q", params.Origin)
	}

	mcpConfig, apiConfig, err := s.parseValidateAndTransform(params)
	if err != nil {
		return nil, err
	}
	// Generate API ID if not provided
	apiID := params.ID
	if apiID == "" {
		var err error
		apiID, err = GenerateUUID()
		if err != nil {
			return nil, fmt.Errorf("failed to generate API ID: %w", err)
		}
	}

	handle := mcpConfig.Metadata.Name

	name, version, err := ExtractNameVersion(*apiConfig)
	if err != nil {
		return nil, err
	}

	existingByNameVersion, err := s.db.GetConfigByKindNameAndVersion(string(api.MCPProxyConfigurationKindMcp), name, version)
	if err == nil {
		if existingByNameVersion != nil && existingByNameVersion.UUID != apiID {
			return nil, fmt.Errorf("%w: configuration with name '%s' and version '%s' already exists", storage.ErrConflict, name, version)
		}
	} else if !storage.IsNotFoundError(err) {
		return nil, fmt.Errorf("failed to check existing MCP proxy name/version conflict: %w", err)
	}
	if handle != "" {
		existingByHandle, err := s.db.GetConfigByKindAndHandle(string(api.MCPProxyConfigurationKindMcp), handle)
		if err == nil {
			if existingByHandle != nil && existingByHandle.UUID != apiID {
				return nil, fmt.Errorf("%w: configuration with handle '%s' already exists", storage.ErrConflict, handle)
			}
		} else if !storage.IsNotFoundError(err) {
			return nil, fmt.Errorf("failed to check existing MCP proxy handle conflict: %w", err)
		}
	}

	isUpdate := false
	if params.ID != "" {
		if existing, err := s.db.GetConfig(params.ID); err == nil && existing != nil {
			isUpdate = true
		}
	}

	// Create stored configuration
	now := time.Now()
	deployedAt := params.DeployedAt
	if deployedAt == nil {
		truncated := now.Truncate(time.Millisecond)
		deployedAt = &truncated
	}
	mcpDesiredState := params.DesiredState
	if mcpDesiredState == "" {
		mcpDesiredState = models.StateDeployed
	}

	storedCfg := &models.StoredConfig{
		UUID:                apiID,
		Kind:                string(api.MCPProxyConfigurationKindMcp),
		Handle:              mcpConfig.Metadata.Name,
		DisplayName:         mcpConfig.Spec.DisplayName,
		Version:             mcpConfig.Spec.Version,
		Configuration:       *apiConfig,
		SourceConfiguration: *mcpConfig,
		DesiredState:        mcpDesiredState,
		DeploymentID:        params.DeploymentID,
		Origin:              params.Origin,
		CreatedAt:           now,
		UpdatedAt:           now,
		DeployedAt:          deployedAt,
	}

	// Save or update using timestamp-guarded upsert.
	// affected=true means the DB row was actually inserted or updated.
	// affected=false means a newer version already exists (stale event — no-op).
	affected, err := s.saveOrUpdateConfig(storedCfg, params.Logger)
	if err != nil {
		return nil, err
	}

	if !affected {
		// Stale event — DB was not modified. Return success but skip event publishing and xDS update.
		return &APIDeploymentResult{
			StoredConfig: storedCfg,
			IsUpdate:     isUpdate,
			IsStale:      true,
		}, nil
	}

	// Log success
	if isUpdate {
		params.Logger.Info("MCP configuration updated",
			slog.String("api_id", apiID),
			slog.String("displayName", mcpConfig.Spec.DisplayName),
			slog.String("version", mcpConfig.Spec.Version),
			slog.String("correlation_id", params.CorrelationID))
	} else {
		params.Logger.Info("MCP configuration created",
			slog.String("api_id", apiID),
			slog.String("displayName", mcpConfig.Spec.DisplayName),
			slog.String("version", mcpConfig.Spec.Version),
			slog.String("correlation_id", params.CorrelationID))
	}

	action := "CREATE"
	if isUpdate {
		action = "UPDATE"
	}
	s.publishMCPProxyEvent(action, apiID, params.CorrelationID, params.Logger)

	return &APIDeploymentResult{
		StoredConfig: storedCfg,
		IsUpdate:     isUpdate,
	}, nil
}

// saveOrUpdateConfig handles the atomic save/update operation using timestamp-guarded
// upsert. Returns (affected, error) where affected=true means the DB row was actually
// inserted or updated. Callers should only publish EventHub events and update xDS
// snapshots when affected=true.
func (s *MCPDeploymentService) saveOrUpdateConfig(storedCfg *models.StoredConfig, logger *slog.Logger) (bool, error) {
	affected, err := s.db.UpsertConfig(storedCfg)
	if err != nil {
		return false, fmt.Errorf("failed to upsert config to database: %w", err)
	}
	if !affected {
		logger.Debug("Skipped stale MCP configuration (newer version exists in DB)",
			slog.String("api_id", storedCfg.UUID),
			slog.String("displayName", storedCfg.DisplayName),
			slog.String("version", storedCfg.Version))
		return false, nil
	}

	return true, nil
}

func (s *MCPDeploymentService) parseValidateAndTransform(params MCPDeploymentParams) (*api.MCPProxyConfiguration, *api.RestAPI, error) {
	var mcpConfig api.MCPProxyConfiguration
	var apiConfig api.RestAPI
	// Parse configuration
	err := s.parser.Parse(params.Data, params.ContentType, &mcpConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	// Render template expressions ({{ secret "..." }}, {{ env "..." }}, {{ default ... }}, etc.)
	// BEFORE validation so the validator sees resolved values, not raw template syntax.
	// We render in a temp StoredConfig then cast back. The original mcpConfig (unrendered)
	// is what callers persist as SourceConfiguration; each replica re-renders on consumption.
	renderHolder := &models.StoredConfig{Configuration: mcpConfig}
	if err := templateengine.RenderSpec(renderHolder, s.secretResolver, params.Logger); err != nil {
		return nil, nil, err
	}
	renderedMCP, ok := renderHolder.Configuration.(api.MCPProxyConfiguration)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected configuration type %T after rendering MCP proxy", renderHolder.Configuration)
	}

	// Validate configuration against rendered values
	validationErrors := s.validator.Validate(&renderedMCP)
	if len(validationErrors) > 0 {
		errors := make([]string, 0, len(validationErrors))
		params.Logger.Warn("Configuration validation failed",
			slog.String("api_id", params.ID),
			slog.String("name", mcpConfig.Spec.DisplayName),
			slog.Int("num_errors", len(validationErrors)))

		for i, e := range validationErrors {
			params.Logger.Warn("Validation error",
				slog.String("field", e.Field),
				slog.String("message", e.Message))
			errors = append(errors, fmt.Sprintf("%d. %s: %s", i+1, e.Field, e.Message))
		}

		combinedMsg := strings.Join(errors, "; ")

		return nil, nil, fmt.Errorf("configuration validation failed with %d error(s): %s", len(validationErrors), combinedMsg)
	}

	// Transform to API configuration
	apiConfigPtr, err := s.transformer.Transform(&mcpConfig, &apiConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to transform MCP configuration to API configuration: %w", err)
	}
	apiConfig = *apiConfigPtr
	return &mcpConfig, &apiConfig, nil
}

// ListMCPProxies returns all stored MCP proxy configurations from the database.
func (s *MCPDeploymentService) ListMCPProxies() ([]*models.StoredConfig, error) {
	configs, err := s.db.GetAllConfigsByKind(string(api.MCPProxyConfigurationKindMcp))
	if err != nil {
		return nil, err
	}
	return configs, nil
}

// GetMCPProxyByHandle returns an MCP proxy configuration by its handle (metadata.name)
func (s *MCPDeploymentService) GetMCPProxyByHandle(handle string) (*models.StoredConfig, error) {
	cfg, err := s.db.GetConfigByKindAndHandle(models.KindMcp, handle)
	if err != nil {
		if isMCPNotFoundError(err) {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}
	s.hydrateStoredMCPConfig(cfg)
	return cfg, nil
}

// CreateMCPProxy is a convenience wrapper around DeployMCPConfiguration for creating MCP proxies
func (s *MCPDeploymentService) CreateMCPProxy(params MCPDeploymentParams) (*APIDeploymentResult, error) {
	return s.DeployMCPConfiguration(params)
}

// UpdateMCPProxy updates an existing MCP proxy identified by its handle.
// The full config is always applied. If deploymentState is "undeployed", the proxy is also
// removed from router traffic while preserving the updated configuration.
func (s *MCPDeploymentService) UpdateMCPProxy(handle string, params MCPDeploymentParams, logger *slog.Logger) (*models.StoredConfig, error) {
	existing, err := s.GetMCPProxyByHandle(handle)
	if err != nil || existing == nil {
		return nil, fmt.Errorf("MCP proxy configuration with handle '%s' not found", handle)
	}

	// Extract deploymentState from the incoming config
	if isUndeploy, err := s.isMCPUndeployRequest(params); err != nil {
		return nil, err
	} else if isUndeploy {
		params.DesiredState = models.StateUndeployed
		// DeployedAt left nil — deploy method will set time.Now() to mark undeployment time
	} else {
		// Advance DeployedAt so the timestamp-guarded upsert accepts the update.
		now := time.Now().Truncate(time.Millisecond)
		params.DeployedAt = &now
	}

	// Ensure Deploy uses existing ID so it performs an update
	params.ID = existing.UUID
	res, err := s.DeployMCPConfiguration(params)
	if err != nil {
		return nil, err
	}
	return res.StoredConfig, nil
}

// isMCPUndeployRequest parses just enough of the MCP config to check if deploymentState is "undeployed".
func (s *MCPDeploymentService) isMCPUndeployRequest(params MCPDeploymentParams) (bool, error) {
	var mcpConfig api.MCPProxyConfiguration
	if err := s.parser.Parse(params.Data, params.ContentType, &mcpConfig); err != nil {
		return false, fmt.Errorf("failed to parse MCP proxy configuration: %w", err)
	}
	return mcpConfig.Spec.DeploymentState != nil &&
		*mcpConfig.Spec.DeploymentState == api.MCPProxyConfigDataDeploymentStateUndeployed, nil
}

// DeleteMCPProxy deletes an MCP proxy by handle using store/db and updates snapshot
func (s *MCPDeploymentService) DeleteMCPProxy(handle, correlationID string, logger *slog.Logger) (*models.StoredConfig, error) {
	cfg, err := s.GetMCPProxyByHandle(handle)
	if err != nil {
		if isMCPNotFoundError(err) {
			logger.Error("MCP proxy configuration not found", slog.String("handle", handle))
			return nil, fmt.Errorf("MCP proxy configuration with handle '%s' not found", handle)
		}
		logger.Error("Failed to fetch MCP proxy configuration",
			slog.String("handle", handle),
			slog.Any("error", err))
		return nil, fmt.Errorf("failed to fetch MCP proxy configuration")
	}

	// Remove runtime config first, before touching the source of truth.
	// If this fails the DB/store still hold the record so the caller can retry.
	if s.policyManager != nil {
		if err := s.policyManager.DeleteAPIConfig(cfg.Kind, cfg.Handle); err != nil {
			logger.Error("Failed to remove runtime config for MCP proxy", slog.Any("error", err))
			return nil, fmt.Errorf("failed to remove runtime config for MCP proxy: %w", err)
		}
	}

	if err := s.db.DeleteConfig(cfg.UUID); err != nil {
		logger.Error("Failed to delete config from database", slog.Any("error", err))
		return nil, fmt.Errorf("failed to delete configuration from database: %w", err)
	}
	s.publishMCPProxyEvent("DELETE", cfg.UUID, correlationID, logger)
	return cfg, nil
}

// UndeployMCPProxy marks an MCP proxy as undeployed while preserving its
// canonical configuration for future redeploys.
func (s *MCPDeploymentService) UndeployMCPProxy(
	id string,
	deploymentID string,
	performedAt *time.Time,
	correlationID string,
	logger *slog.Logger,
) (*models.StoredConfig, error) {
	cfg, err := s.getMCPProxyByID(id)
	if err != nil {
		return nil, err
	}

	if cfg.DeploymentID != "" && deploymentID != "" && cfg.DeploymentID != deploymentID {
		return nil, ErrMCPDeploymentIDMismatch
	}

	undeployedAt := time.Now().Truncate(time.Millisecond)
	if performedAt != nil && !performedAt.IsZero() {
		undeployedAt = (*performedAt).Truncate(time.Millisecond)
	}

	updated := *cfg
	updated.DesiredState = models.StateUndeployed
	updated.DeploymentID = deploymentID
	updated.DeployedAt = &undeployedAt
	updated.UpdatedAt = time.Now()

	// Timestamp-guarded upsert: only writes if deployed_at is newer than what's in DB.
	affected, err := s.db.UpsertConfig(&updated)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert configuration in database: %w", err)
	}
	if !affected {
		return nil, ErrMCPUndeployStale
	}
	s.publishMCPProxyEvent("UPDATE", updated.UUID, correlationID, logger)
	return &updated, nil
}
