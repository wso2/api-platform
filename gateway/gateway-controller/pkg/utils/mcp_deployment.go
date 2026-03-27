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
	"context"
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
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

const (
	LATEST_SUPPORTED_MCP_SPEC_VERSION = "2025-06-18"
)

var ErrMCPDeploymentIDMismatch = errors.New("mcp proxy deployment id mismatch")
var ErrMCPUndeployStale = errors.New("mcp proxy undeploy skipped: newer version exists")

type MCPDeploymentParams struct {
	Data          []byte        // Raw configuration data (YAML/JSON)
	ContentType   string        // Content type for parsing
	ID            string        // ID (if provided, used for updates; if empty, generates new UUID)
	DeploymentID  string        // Platform deployment ID (empty for gateway-api origin)
	Origin        models.Origin // Origin of the deployment: "control_plane" or "gateway_api"
	DeployedAt    *time.Time    // Deployment timestamp from platform event (nil for gateway-api origin)
	CorrelationID string        // Correlation ID for tracking
	Logger        *slog.Logger  // Logger instance
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
}

// NewMCPDeploymentService creates a new MCP deployment service
func NewMCPDeploymentService(
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
	policyManager *policyxds.PolicyManager,
	policyValidator *config.PolicyValidator,
) *MCPDeploymentService {
	return &MCPDeploymentService{
		store:           store,
		db:              db,
		snapshotManager: snapshotManager,
		parser:          config.NewParser(),
		validator:       config.NewMCPValidator().WithPolicyValidator(policyValidator),
		transformer:     NewMCPTransformer(),
		policyManager:   policyManager,
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

// SetEventHub configures EventHub publishing for replica-synced MCP proxy flows.
func (s *MCPDeploymentService) SetEventHub(eventHub eventhub.EventHub, gatewayID string) {
	s.eventHub = eventHub
	s.gatewayID = gatewayID
}

func (s *MCPDeploymentService) isEventDriven() bool {
	return s.eventHub != nil
}

func (s *MCPDeploymentService) publishMCPProxyEvent(action, entityID, correlationID string, logger *slog.Logger) {
	if s.eventHub == nil {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	if strings.TrimSpace(s.gatewayID) == "" {
		logger.Warn("Skipping MCP proxy event publish because gateway ID is not configured",
			slog.String("action", action),
			slog.String("entity_id", entityID))
		return
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
	if s.db != nil {
		cfg, err := s.db.GetConfig(id)
		if err == nil {
			s.hydrateStoredMCPConfig(cfg)
			return cfg, nil
		}
		if !isMCPNotFoundError(err) {
			return nil, err
		}
	}

	cfg, err := s.store.Get(id)
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

	existingConfigs := s.store.GetAllByKind(string(api.Mcp))
	if s.db != nil {
		if storedConfigs, err := s.db.GetAllConfigsByKind(string(api.Mcp)); err == nil {
			existingConfigs = storedConfigs
		}
	}
	for _, cfg := range existingConfigs {
		if cfg.UUID == apiID {
			continue
		}
		if cfg.DisplayName == name && cfg.Version == version {
			return nil, fmt.Errorf("%w: configuration with name '%s' and version '%s' already exists", storage.ErrConflict, name, version)
		}
		if handle != "" && cfg.Handle == handle {
			return nil, fmt.Errorf("%w: configuration with handle '%s' already exists", storage.ErrConflict, handle)
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
		deployedAt = &now
	}
	storedCfg := &models.StoredConfig{
		UUID:                apiID,
		Kind:                string(api.Mcp),
		Handle:              mcpConfig.Metadata.Name,
		DisplayName:         mcpConfig.Spec.DisplayName,
		Version:             mcpConfig.Spec.Version,
		Configuration:       *apiConfig,
		SourceConfiguration: *mcpConfig,
		DesiredState:        models.StateDeployed,
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

	if s.isEventDriven() {
		action := "CREATE"
		if isUpdate {
			action = "UPDATE"
		}
		s.publishMCPProxyEvent(action, apiID, params.CorrelationID, params.Logger)
	} else if s.snapshotManager != nil {
		// Update xDS snapshot asynchronously
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := s.snapshotManager.UpdateSnapshot(ctx, params.CorrelationID); err != nil {
				params.Logger.Error("Failed to update xDS snapshot",
					slog.Any("error", err),
					slog.String("api_id", apiID),
					slog.String("correlation_id", params.CorrelationID))
			}
		}()
	}

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
	if s.db != nil {
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

		// In event-driven mode, the EventListener will update the in-memory store
		// via event processing. Only update the store inline for non-event-driven mode.
		if !s.isEventDriven() {
			if existing, _ := s.store.Get(storedCfg.UUID); existing != nil {
				storedCfg.CreatedAt = existing.CreatedAt
				if err := s.store.Update(storedCfg); err != nil {
					logger.Warn("Failed to update config in memory store after DB upsert",
						slog.String("api_id", storedCfg.UUID),
						slog.Any("error", err))
				}
			} else {
				if err := s.store.Add(storedCfg); err != nil {
					logger.Warn("Failed to add config to memory store after DB upsert",
						slog.String("api_id", storedCfg.UUID),
						slog.Any("error", err))
				}
			}
		}

		return true, nil
	}

	// Memory-only mode (no database): use in-memory store directly
	if existing, _ := s.store.Get(storedCfg.UUID); existing != nil {
		storedCfg.CreatedAt = existing.CreatedAt
		if err := s.store.Update(storedCfg); err != nil {
			return false, fmt.Errorf("failed to update config in memory store: %w", err)
		}
	} else {
		if err := s.store.Add(storedCfg); err != nil {
			return false, fmt.Errorf("failed to add config to memory store: %w", err)
		}
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

	// Validate configuration
	validationErrors := s.validator.Validate(&mcpConfig)
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

// ListMCPProxies returns all stored MCP proxy configurations with their
// derived RestAPI Configuration hydrated from StoredConfig.SourceConfiguration.
func (s *MCPDeploymentService) ListMCPProxies() []*models.StoredConfig {
	configs := s.store.GetAllByKind(string(api.Mcp))
	if s.db != nil {
		if storedConfigs, err := s.db.GetAllConfigsByKind(string(api.Mcp)); err == nil {
			configs = storedConfigs
		}
	}
	for _, cfg := range configs {
		s.hydrateStoredMCPConfig(cfg)
	}
	return configs
}

// GetMCPProxyByHandle returns an MCP proxy configuration by its handle (metadata.name)
func (s *MCPDeploymentService) GetMCPProxyByHandle(handle string) (*models.StoredConfig, error) {
	if s.db != nil {
		cfg, err := s.db.GetConfigByKindAndHandle(models.KindMcp, handle)
		if err == nil {
			s.hydrateStoredMCPConfig(cfg)
			return cfg, nil
		}
		if isMCPNotFoundError(err) {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}

	cfg, err := s.store.GetByKindAndHandle(models.KindMcp, handle)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, storage.ErrNotFound
	}
	s.hydrateStoredMCPConfig(cfg)
	return cfg, nil
}

// CreateMCPProxy is a convenience wrapper around DeployMCPConfiguration for creating MCP proxies
func (s *MCPDeploymentService) CreateMCPProxy(params MCPDeploymentParams) (*APIDeploymentResult, error) {
	return s.DeployMCPConfiguration(params)
}

// UpdateMCPProxy updates an existing MCP proxy identified by its handle
func (s *MCPDeploymentService) UpdateMCPProxy(handle string, params MCPDeploymentParams, logger *slog.Logger) (*models.StoredConfig, error) {
	existing, err := s.GetMCPProxyByHandle(handle)
	if err != nil || existing == nil {
		return nil, fmt.Errorf("MCP proxy configuration with handle '%s' not found", handle)
	}

	// Check if this is an undeployment request
	if isUndeploy, err := s.isMCPUndeployRequest(params); err != nil {
		return nil, err
	} else if isUndeploy {
		return s.undeployMCPViaREST(existing, params)
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

// undeployMCPViaREST handles undeployment initiated via the REST API update endpoint.
// It updates DesiredState to StateUndeployed, preserves DeployedAt, and publishes an event.
func (s *MCPDeploymentService) undeployMCPViaREST(existing *models.StoredConfig, params MCPDeploymentParams) (*models.StoredConfig, error) {
	existing.DesiredState = models.StateUndeployed
	existing.UpdatedAt = time.Now()
	// Preserve DeployedAt to track when it was last deployed

	// Update the deployment state in the stored management payload so it stays
	// consistent with the top-level DesiredState when read back from the database.
	if src, ok := existing.SourceConfiguration.(api.MCPProxyConfiguration); ok {
		state := api.MCPProxyConfigDataDeploymentStateUndeployed
		src.Spec.DeploymentState = &state
		existing.SourceConfiguration = src
	}

	if s.db != nil {
		if err := s.db.UpdateConfig(existing); err != nil {
			return nil, fmt.Errorf("failed to persist undeployment: %w", err)
		}
	}

	if s.isEventDriven() {
		s.publishMCPProxyEvent("UPDATE", existing.UUID, params.CorrelationID, params.Logger)
	} else {
		if err := s.store.Update(existing); err != nil {
			return nil, fmt.Errorf("failed to update config in memory store: %w", err)
		}

		if s.snapshotManager != nil {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := s.snapshotManager.UpdateSnapshot(ctx, params.CorrelationID); err != nil {
					params.Logger.Error("Failed to update xDS snapshot", slog.Any("error", err))
				}
			}()
		}
	}

	params.Logger.Info("MCP proxy configuration undeployed",
		slog.String("id", existing.UUID),
		slog.String("handle", existing.Handle))

	return existing, nil
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

	// Delete from database (only if persistent mode)
	if s.db != nil {
		if err := s.db.DeleteConfig(cfg.UUID); err != nil {
			logger.Error("Failed to delete config from database", slog.Any("error", err))
			return nil, fmt.Errorf("failed to delete configuration from database: %w", err)
		}
	}

	if s.isEventDriven() {
		s.publishMCPProxyEvent("DELETE", cfg.UUID, correlationID, logger)
		return cfg, nil
	}

	// Delete from in-memory store
	if err := s.store.Delete(cfg.UUID); err != nil {
		logger.Error("Failed to delete config from memory store", slog.Any("error", err))
		return nil, fmt.Errorf("failed to delete configuration from memory store: %w", err)
	}

	if s.snapshotManager != nil {
		// Update xDS snapshot asynchronously
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := s.snapshotManager.UpdateSnapshot(ctx, correlationID); err != nil {
				logger.Error("Failed to update xDS snapshot", slog.Any("error", err))
			}
		}()
	}

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

	undeployedAt := time.Now()
	if performedAt != nil && !performedAt.IsZero() {
		undeployedAt = *performedAt
	}

	updated := *cfg
	updated.DesiredState = models.StateUndeployed
	updated.DeploymentID = deploymentID
	updated.DeployedAt = &undeployedAt
	updated.UpdatedAt = time.Now()

	// Timestamp-guarded upsert: only writes if deployed_at is newer than what's in DB.
	if s.db != nil {
		affected, err := s.db.UpsertConfig(&updated)
		if err != nil {
			return nil, fmt.Errorf("failed to upsert configuration in database: %w", err)
		}
		if !affected {
			return nil, ErrMCPUndeployStale
		}
	}

	if s.isEventDriven() {
		s.publishMCPProxyEvent("UPDATE", updated.UUID, correlationID, logger)
		return &updated, nil
	}

	if err := s.store.Update(&updated); err != nil {
		return nil, fmt.Errorf("failed to update configuration in memory store: %w", err)
	}

	if s.snapshotManager != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := s.snapshotManager.UpdateSnapshot(ctx, correlationID); err != nil {
				logger.Error("Failed to update xDS snapshot", slog.Any("error", err))
			}
		}()
	}

	return &updated, nil
}
