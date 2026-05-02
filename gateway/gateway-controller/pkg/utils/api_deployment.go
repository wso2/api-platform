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
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	commonconstants "github.com/wso2/api-platform/common/constants"
	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine/funcs"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

// APIDeploymentParams contains parameters for API deployment operations
type APIDeploymentParams struct {
	Data          []byte        // Raw configuration data (YAML/JSON)
	ContentType   string        // Content type for parsing
	Kind          string        // API kind: "RestApi" or "WebSubApi"
	APIID         string        // API ID (if provided, used for updates; if empty, generates new UUID)
	DeploymentID  string        // Platform deployment ID (empty for gateway-api origin)
	Origin        models.Origin // Origin of the deployment: "control_plane" or "gateway_api"
	DeployedAt    *time.Time    // Deployment timestamp from platform event (nil for gateway-api origin)
	CorrelationID string        // Correlation ID for tracking
	Logger        *slog.Logger  // Logger instance
}

// APIDeploymentResult contains the result of API deployment
type APIDeploymentResult struct {
	StoredConfig *models.StoredConfig
	IsUpdate     bool
	IsStale      bool // true if the DB row was NOT modified (a newer version already exists)
}

// ValidationErrorListError wraps validation errors for API configuration.
// This allows callers to return structured validation errors in API responses.
type ValidationErrorListError struct {
	Errors []config.ValidationError
}

func (e *ValidationErrorListError) Error() string {
	return fmt.Sprintf("configuration validation failed with %d errors", len(e.Errors))
}

// APIDeploymentService provides utilities for API configuration deployment
type APIDeploymentService struct {
	store           *storage.ConfigStore
	db              storage.Storage
	snapshotManager *xds.SnapshotManager
	parser          *config.Parser
	validator       config.Validator
	routerConfig    *config.RouterConfig
	httpClient      *http.Client
	eventHub        eventhub.EventHub
	gatewayID       string
	secretResolver  funcs.SecretResolver
}

func (s *APIDeploymentService) validateArtifactConflicts(kind, currentID, displayName, version, handle string) error {
	existingByNameVersion, err := s.db.GetConfigByKindNameAndVersion(kind, displayName, version)
	if err == nil {
		if existingByNameVersion != nil && existingByNameVersion.UUID != currentID {
			return fmt.Errorf("%w: configuration with name '%s' and version '%s' already exists",
				storage.ErrConflict, displayName, version)
		}
	} else if !storage.IsNotFoundError(err) {
		return fmt.Errorf("failed to check existing %s name/version conflict: %w", kind, err)
	}

	existingByHandle, err := s.db.GetConfigByKindAndHandle(kind, handle)
	if err == nil {
		if existingByHandle != nil && existingByHandle.UUID != currentID {
			return fmt.Errorf("%w: configuration with handle '%s' already exists",
				storage.ErrConflict, handle)
		}
	} else if !storage.IsNotFoundError(err) {
		return fmt.Errorf("failed to check existing %s handle conflict: %w", kind, err)
	}

	return nil
}

// NewAPIDeploymentService creates a new API deployment service
func NewAPIDeploymentService(
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
	validator config.Validator,
	routerConfig *config.RouterConfig,
	eventHub eventhub.EventHub,
	gatewayID string,
	secretResolver funcs.SecretResolver,
) *APIDeploymentService {
	if db == nil {
		panic("APIDeploymentService requires non-nil storage")
	}
	trimmedGatewayID := requireReplicaSyncWiring("APIDeploymentService", eventHub, gatewayID)

	return &APIDeploymentService{
		store:           store,
		db:              db,
		snapshotManager: snapshotManager,
		parser:          config.NewParser(),
		validator:       validator,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
		routerConfig:    routerConfig,
		eventHub:        eventHub,
		gatewayID:       trimmedGatewayID,
		secretResolver:  secretResolver,
	}
}

// TODO: (VirajSalaka) We do not need gatewayID in the event as it is part of the publishEvent.
// publishEvent publishes an event to the EventHub for async processing.
func (s *APIDeploymentService) publishEvent(eventType eventhub.EventType, action, entityID, correlationID string, logger *slog.Logger) {
	event := eventhub.Event{
		GatewayID:           s.gatewayID,
		OriginatedTimestamp: time.Now(),
		EventType:           eventType,
		Action:              action,
		EntityID:            entityID,
		EventID:             correlationID,
		EventData:           eventhub.EmptyEventData,
	}
	if err := s.eventHub.PublishEvent(s.gatewayID, event); err != nil {
		logger.Error("Failed to publish event",
			slog.String("event_type", string(eventType)),
			slog.String("action", action),
			slog.String("entity_id", entityID),
			slog.Any("error", err))
	}
}

// DeployAPIConfiguration handles the complete API configuration deployment process
// Important: The APIDeploymentResult contains resolved secrets. Do not expose them in responses.
func (s *APIDeploymentService) DeployAPIConfiguration(params APIDeploymentParams) (*APIDeploymentResult, error) {
	if !models.IsValidOrigin(params.Origin) {
		return nil, fmt.Errorf("invalid or missing origin: %q", params.Origin)
	}

	var (
		parsedConfig         any
		kind                 string
		handle               string
		annotationArtifactID string
	)

	// If Kind is not provided, infer it from the payload
	resolvedKind := params.Kind
	if resolvedKind == "" {
		var envelope struct {
			Kind string `json:"kind" yaml:"kind"`
		}
		if err := s.parser.Parse(params.Data, params.ContentType, &envelope); err != nil {
			return nil, fmt.Errorf("failed to parse configuration to infer kind: %w", err)
		}
		if envelope.Kind == "" {
			return nil, fmt.Errorf("resource kind is required: set Kind in deployment params or include a 'kind' field in the payload")
		}
		resolvedKind = envelope.Kind
	}

	// Parse into the typed config and extract identifiers that live outside the spec block
	// (kind, metadata.name, artifact-id annotation). Spec-level identifiers (DisplayName,
	// Version) are extracted later from the rendered Configuration so the validator and
	// downstream logic see resolved template values, not raw template syntax.
	switch resolvedKind {
	case "WebSubApi":
		var webSubConfig api.WebSubAPI
		if err := s.parser.Parse(params.Data, params.ContentType, &webSubConfig); err != nil {
			return nil, fmt.Errorf("failed to parse configuration: %w", err)
		}
		handle = webSubConfig.Metadata.Name
		kind = string(webSubConfig.Kind)
		parsedConfig = webSubConfig
		annotationArtifactID = annotationValue(webSubConfig.Metadata.Annotations, commonconstants.AnnotationArtifactID)
	case "RestApi":
		var restConfig api.RestAPI
		if err := s.parser.Parse(params.Data, params.ContentType, &restConfig); err != nil {
			return nil, fmt.Errorf("failed to parse configuration: %w", err)
		}
		handle = restConfig.Metadata.Name
		kind = string(restConfig.Kind)
		parsedConfig = restConfig
		annotationArtifactID = annotationValue(restConfig.Metadata.Annotations, commonconstants.AnnotationArtifactID)
	default:
		return nil, fmt.Errorf("unsupported resource kind %q: must be \"RestApi\" or \"WebSubApi\"", resolvedKind)
	}

	// Resolve API ID: explicit param > artifact-id annotation > auto-generate
	apiID := params.APIID
	if apiID == "" {
		if annotationArtifactID != "" {
			if err := ValidateUUIDFormat(annotationArtifactID); err != nil {
				return nil, fmt.Errorf("invalid %s annotation: %w", commonconstants.AnnotationArtifactID, err)
			}
			apiID = annotationArtifactID
		} else {
			var err error
			apiID, err = GenerateUUID()
			if err != nil {
				return nil, fmt.Errorf("failed to generate API ID: %w", err)
			}
		}
	}

	var isUpdate bool
	var existingConfig *models.StoredConfig

	if s.db != nil {
		var err error
		existingConfig, err = s.db.GetConfig(apiID)
		if err == nil && existingConfig != nil {
			isUpdate = true
		} else if err != nil && !storage.IsNotFoundError(err) && !storage.IsDatabaseUnavailableError(err) {
			return nil, fmt.Errorf("failed to look up existing configuration: %w", err)
		}
	}

	// Create stored configuration. DisplayName/Version are populated below from the
	// rendered Configuration so identifiers reflect resolved template values.
	now := time.Now()
	deployedAt := params.DeployedAt
	if deployedAt == nil {
		truncated := now.Truncate(time.Millisecond)
		deployedAt = &truncated
	}
	var cpSyncStatus models.CPSyncStatus
	if params.Origin == models.OriginGatewayAPI {
		cpSyncStatus = models.CPSyncStatusPending
	}
	storedCfg := &models.StoredConfig{
		UUID:                apiID,
		Kind:                kind,
		Handle:              handle,
		Configuration:       parsedConfig,
		SourceConfiguration: parsedConfig,
		DesiredState:        models.StateDeployed,
		DeploymentID:        params.DeploymentID,
		Origin:              params.Origin,
		CPSyncStatus:        cpSyncStatus,
		CreatedAt:           now,
		UpdatedAt:           now,
		DeployedAt:          deployedAt,
	}

	// Resolve gateway-default vhost sentinels before render so the stored vhosts are
	// immune to future gateway config changes. Sync SourceConfiguration so the resolved
	// (but still unrendered) vhosts are what gets persisted — the DB layer marshals
	// SourceConfiguration, and each replica re-renders Configuration on consumption.
	if err := resolveVhostSentinels(&storedCfg.Configuration, s.routerConfig); err != nil {
		return nil, fmt.Errorf("failed to resolve vhost sentinels: %w", err)
	}
	storedCfg.SourceConfiguration = storedCfg.Configuration

	// Render template expressions ({{ secret "..." }}, {{ env "..." }}, {{ default ... }}, etc.)
	// BEFORE validation so the validator sees resolved values, not raw template syntax.
	// Configuration becomes the rendered version; SourceConfiguration stays unrendered.
	if err := templateengine.RenderSpec(storedCfg, s.secretResolver, params.Logger); err != nil {
		return nil, err
	}

	// Validate against the rendered Configuration and extract spec-level identifiers.
	var apiName, apiVersion string
	switch c := storedCfg.Configuration.(type) {
	case api.WebSubAPI:
		apiName = c.Spec.DisplayName
		apiVersion = c.Spec.Version
		validationErrors := s.validator.Validate(&c)
		if len(validationErrors) > 0 {
			s.logValidationErrors(params.Logger, apiID, apiName, validationErrors)
			return nil, &ValidationErrorListError{Errors: validationErrors}
		}
	case api.RestAPI:
		apiName = c.Spec.DisplayName
		apiVersion = c.Spec.Version
		validationErrors := s.validator.Validate(&c)
		if len(validationErrors) > 0 {
			s.logValidationErrors(params.Logger, apiID, apiName, validationErrors)
			return nil, &ValidationErrorListError{Errors: validationErrors}
		}
	default:
		return nil, fmt.Errorf("unexpected configuration type %T after rendering", storedCfg.Configuration)
	}
	storedCfg.DisplayName = apiName
	storedCfg.Version = apiVersion

	if err := s.validateArtifactConflicts(kind, apiID, apiName, apiVersion, handle); err != nil {
		return nil, err
	}

	// Compute WebSub topic diff BEFORE persisting — ConfigStore.Add populates TopicManager,
	// so GetTopicsForUpdate must run while the store still has the old state. Runs against
	// the rendered Configuration so topic names reflect resolved template values.
	var topicsToRegister, topicsToUnregister []string
	if kind == "WebSubApi" {
		topicsToRegister, topicsToUnregister = s.GetTopicsForUpdate(*storedCfg)
	}

	var saveErr error

	// Try to save/update the configuration using timestamp-guarded upsert.
	// affected=true means the row was actually inserted or updated in the DB.
	// affected=false means a newer version already exists (stale event — no-op).
	affected, saveErr := s.saveOrUpdateConfig(storedCfg, params.Logger)
	if saveErr != nil {
		return nil, saveErr
	}

	if !affected {
		// Stale event — DB was not modified. Return success but skip event publishing and xDS update.
		return &APIDeploymentResult{
			StoredConfig: storedCfg,
			IsUpdate:     isUpdate,
			IsStale:      true,
		}, nil
	}

	// WebSub topic registration/deregistration — only after successful, non-stale persistence.
	if kind == "WebSubApi" {
		if err := s.completeWebSubTopicOperations(apiID, topicsToRegister, topicsToUnregister, params.Logger); err != nil {
			if rollbackErr := s.rollbackPersistedAPIConfiguration(storedCfg, existingConfig, isUpdate, params.Logger); rollbackErr != nil {
				return nil, fmt.Errorf("%w; rollback failed: %v", err, rollbackErr)
			}
			return nil, err
		}
	}

	// Log success
	if isUpdate {
		params.Logger.Info("API configuration updated",
			slog.String("api_id", apiID),
			slog.String("name", apiName),
			slog.String("version", apiVersion),
			slog.String("correlation_id", params.CorrelationID))
	} else {
		params.Logger.Info("API configuration created",
			slog.String("api_id", apiID),
			slog.String("name", apiName),
			slog.String("version", apiVersion),
			slog.String("correlation_id", params.CorrelationID))
	}

	action := "CREATE"
	if isUpdate {
		action = "UPDATE"
	}
	s.publishEvent(eventhub.EventTypeAPI, action, apiID, params.CorrelationID, params.Logger)

	return &APIDeploymentResult{
		StoredConfig: storedCfg,
		IsUpdate:     isUpdate,
		IsStale:      false,
	}, nil
}

func (s *APIDeploymentService) completeWebSubTopicOperations(
	apiID string,
	topicsToRegister []string,
	topicsToUnregister []string,
	logger *slog.Logger,
) error {
	if len(topicsToRegister) == 0 && len(topicsToUnregister) == 0 {
		return nil
	}
	if s.routerConfig == nil {
		return fmt.Errorf("failed to complete topic operations: router configuration is required for WebSub topic operations")
	}

	// TODO: Pre configure the dynamic forward proxy rules for event gw.
	// This communication bridge will be created on the gw startup and can perform
	// internal communication with the WebSub hub without relying on dynamic rules.
	var wg sync.WaitGroup

	runTopicOps := func(
		list []string,
		action string,
		successMessage string,
	) {
		if len(list) == 0 {
			return
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Info("Starting topic operation",
				slog.String("action", action),
				slog.Int("total_topics", len(list)),
				slog.String("api_id", apiID))

			var childWg sync.WaitGroup
			for _, topic := range list {
				childWg.Add(1)
				go func(topic string) {
					defer childWg.Done()
					logger.Info(successMessage,
						slog.String("topic", topic),
						slog.String("api_id", apiID))
				}(topic)
			}
			childWg.Wait()
		}()
	}

	runTopicOps(
		topicsToRegister,
		"register",
		"Successfully registered topic with WebSubHub",
	)

	runTopicOps(
		topicsToUnregister,
		"deregister",
		"Successfully deregistered topic from WebSubHub",
	)

	wg.Wait()
	logger.Info("Topic lifecycle operations completed",
		slog.String("api_id", apiID),
		slog.Int("registered", len(topicsToRegister)),
		slog.Int("deregistered", len(topicsToUnregister)))

	return nil
}

func (s *APIDeploymentService) rollbackPersistedAPIConfiguration(
	storedCfg *models.StoredConfig,
	previousCfg *models.StoredConfig,
	isUpdate bool,
	logger *slog.Logger,
) error {
	if isUpdate {
		if previousCfg == nil {
			return fmt.Errorf("previous configuration was not found for rollback")
		}
		if err := s.db.UpdateConfig(previousCfg); err != nil {
			return fmt.Errorf("failed to restore previous configuration after topic operation failure: %w", err)
		}
		logger.Warn("Rolled back API configuration update after topic operation failure",
			slog.String("api_id", storedCfg.UUID))
		return nil
	}

	if err := s.db.DeleteConfig(storedCfg.UUID); err != nil {
		return fmt.Errorf("failed to delete newly persisted configuration after topic operation failure: %w", err)
	}
	logger.Warn("Rolled back API configuration create after topic operation failure",
		slog.String("api_id", storedCfg.UUID))
	return nil
}

func (s *APIDeploymentService) GetTopicsForUpdate(apiConfig models.StoredConfig) ([]string, []string) {
	topics := s.store.TopicManager.GetAllByConfig(apiConfig.UUID)
	topicsToRegister := []string{}
	topicsToUnregister := []string{}
	apiTopicsPerRevision := make(map[string]bool)

	webSubCfg, ok := apiConfig.Configuration.(api.WebSubAPI)
	if !ok {
		return topicsToRegister, topicsToUnregister
	}
	asyncData := webSubCfg.Spec

	for _, topic := range asyncData.Hub.Channels {
		// Remove leading '/' from name, context, version and topic path if present
		contextWithVersion := strings.ReplaceAll(asyncData.Context, "$version", asyncData.Version)
		contextWithVersion = strings.TrimPrefix(contextWithVersion, "/")
		contextWithVersion = strings.ReplaceAll(contextWithVersion, "/", "_")
		name := strings.TrimPrefix(topic.Name, "/")
		modifiedTopic := fmt.Sprintf("%s_%s", contextWithVersion, name)
		apiTopicsPerRevision[modifiedTopic] = true
	}

	for _, topic := range topics {
		if _, exists := apiTopicsPerRevision[topic]; !exists {
			topicsToUnregister = append(topicsToUnregister, topic)
		}
	}

	for topic := range apiTopicsPerRevision {
		if s.store.TopicManager.IsTopicExist(apiConfig.UUID, topic) {
			continue
		}
		topicsToRegister = append(topicsToRegister, topic)
	}

	return topicsToRegister, topicsToUnregister
}

func (s *APIDeploymentService) logValidationErrors(logger *slog.Logger, apiID string, apiName string, validationErrors []config.ValidationError) {
	logger.Warn("Configuration validation failed",
		slog.String("api_id", apiID),
		slog.String("name", apiName),
		slog.Int("num_errors", len(validationErrors)))
	for _, e := range validationErrors {
		logger.Warn("Validation error",
			slog.String("field", e.Field),
			slog.String("message", e.Message))
	}
}

func (s *APIDeploymentService) GetTopicsForDelete(apiConfig models.StoredConfig) []string {
	return s.store.TopicManager.GetAllByConfig(apiConfig.UUID)
}

// saveOrUpdateConfig performs a timestamp-guarded upsert of the API configuration.
func (s *APIDeploymentService) saveOrUpdateConfig(storedCfg *models.StoredConfig, logger *slog.Logger) (bool, error) {
	affected, err := s.db.UpsertConfig(storedCfg)
	if err != nil {
		return false, fmt.Errorf("failed to upsert config to database: %w", err)
	}
	if !affected {
		logger.Debug("Skipped stale API configuration (newer version exists in DB)",
			slog.String("api_id", storedCfg.UUID),
			slog.String("displayName", storedCfg.DisplayName),
			slog.String("version", storedCfg.Version))
		return false, nil
	}

	return true, nil
}

func (s *APIDeploymentService) RegisterTopicWithHub(ctx context.Context, httpClient *http.Client, topic, webSubHubHost string, webSubPort int, logger *slog.Logger) error {
	return s.sendTopicRequestToHub(ctx, httpClient, topic, "register", webSubHubHost, webSubPort, logger)
}

// UnregisterTopicWithHub unregisters a topic from the WebSubHub
func (s *APIDeploymentService) UnregisterTopicWithHub(ctx context.Context, httpClient *http.Client, topic, webSubHubHost string, webSubPort int, logger *slog.Logger) error {
	return s.sendTopicRequestToHub(ctx, httpClient, topic, "deregister", webSubHubHost, webSubPort, logger)
}

// sendTopicRequestToHub sends a topic registration/unregistration request to the WebSubHub
func (s *APIDeploymentService) sendTopicRequestToHub(ctx context.Context, httpClient *http.Client, topic string, mode string, webSubHubHost string, webSubPort int, logger *slog.Logger) error {
	// Prepare form data
	formData := url.Values{}
	formData.Set("hub.mode", mode)
	formData.Set("hub.topic", topic)

	// Build target URL to gwHost reverse proxy endpoint (no proxy)
	targetURL := fmt.Sprintf("http://%s:%d/websubhub/operations", webSubHubHost, webSubPort)

	// Retry on 404 Not Found (hub might not be ready immediately)
	const maxRetries = 5
	backoff := 500 * time.Millisecond
	var lastStatus int

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Encode form values so special characters in hub.topic are properly percent-encoded
		req, err := http.NewRequestWithContext(ctx, "POST", targetURL, strings.NewReader(formData.Encode()))
		if err != nil {
			return fmt.Errorf("failed to create HTTP request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := httpClient.Do(req)
		if err != nil {
			// If the context was cancelled or deadline exceeded, surface that
			select {
			case <-ctx.Done():
				return fmt.Errorf("request canceled: %w", ctx.Err())
			default:
			}
			return fmt.Errorf("failed to send HTTP request: %w", err)
		}

		// Ensure body is closed before next loop/return
		func() {
			defer resp.Body.Close()

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				logger.Debug("Topic request sent to WebSubHub",
					slog.String("topic", topic),
					slog.String("mode", mode),
					slog.Int("status", resp.StatusCode))
			}

			lastStatus = resp.StatusCode
		}()

		// Success path returned above
		if lastStatus == 0 {
			return nil
		}

		// Retry only on 404 or 503, up to maxRetries
		if (lastStatus == http.StatusNotFound || lastStatus == http.StatusServiceUnavailable) && attempt < maxRetries {
			select {
			case <-ctx.Done():
				return fmt.Errorf("request canceled: %w", ctx.Err())
			case <-time.After(backoff):
			}
			backoff *= 2
			lastStatus = 0
			continue
		}
		return fmt.Errorf("WebSubHub returned non-success status: %d", lastStatus)
	}

	return fmt.Errorf("WebSubHub request failed after %d retries; last status: %d", maxRetries, lastStatus)
}

// resolveVhostSentinels replaces the gateway-default sentinel in a RestAPI or WebSubAPI's vhosts
// with the actual default values from the router config. This ensures that the stored value is
// always a concrete hostname, making deployments immune to future gateway config changes.
// cfg must be a pointer to an any holding either api.RestAPI or api.WebSubAPI.
func resolveVhostSentinels(cfg *any, routerCfg *config.RouterConfig) error {
	if cfg == nil || routerCfg == nil {
		return nil
	}
	switch c := (*cfg).(type) {
	case api.RestAPI:
		if c.Spec.Vhosts == nil {
			// Populate defaults when vhosts is omitted entirely (e.g. direct gateway deployment
			// without platform-api injecting sentinels). This freezes the current gateway defaults
			// so that routing is immune to future config changes.
			main := routerCfg.VHosts.Main.Default
			c.Spec.Vhosts = &struct {
				Main    string  `json:"main" yaml:"main"`
				Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main: main,
			}
			if sandboxDefault := routerCfg.VHosts.Sandbox.Default; sandboxDefault != "" {
				c.Spec.Vhosts.Sandbox = &sandboxDefault
			}
			*cfg = c
			return nil
		}
		if c.Spec.Vhosts.Main == constants.VHostGatewayDefault {
			c.Spec.Vhosts.Main = routerCfg.VHosts.Main.Default
		}
		if c.Spec.Vhosts.Sandbox != nil && *c.Spec.Vhosts.Sandbox == constants.VHostGatewayDefault {
			resolved := routerCfg.VHosts.Sandbox.Default
			if resolved != "" {
				c.Spec.Vhosts.Sandbox = &resolved
			} else {
				c.Spec.Vhosts.Sandbox = nil
			}
		}
		*cfg = c
	case api.WebSubAPI:
		if c.Spec.Vhosts == nil {
			main := routerCfg.VHosts.Main.Default
			c.Spec.Vhosts = &struct {
				Main    string  `json:"main" yaml:"main"`
				Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main: main,
			}
			if sandboxDefault := routerCfg.VHosts.Sandbox.Default; sandboxDefault != "" {
				c.Spec.Vhosts.Sandbox = &sandboxDefault
			}
			*cfg = c
			return nil
		}
		if c.Spec.Vhosts.Main == constants.VHostGatewayDefault {
			c.Spec.Vhosts.Main = routerCfg.VHosts.Main.Default
		}
		if c.Spec.Vhosts.Sandbox != nil && *c.Spec.Vhosts.Sandbox == constants.VHostGatewayDefault {
			resolved := routerCfg.VHosts.Sandbox.Default
			if resolved != "" {
				c.Spec.Vhosts.Sandbox = &resolved
			} else {
				c.Spec.Vhosts.Sandbox = nil
			}
		}
		*cfg = c
	}
	return nil
}

// annotationValue safely reads a single annotation key from a pointer-to-map.
func annotationValue(annotations *map[string]string, key string) string {
	if annotations == nil {
		return ""
	}
	return (*annotations)[key]
}
