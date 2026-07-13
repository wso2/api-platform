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

package handlers

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"log/slog"
	"net/http"

	"github.com/wso2/api-platform/common/eventhub"
	commonmodels "github.com/wso2/api-platform/common/models"
	"github.com/wso2/api-platform/common/redact"
	adminapi "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/admin"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/handlers/handlerkit"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/apikeyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/controlplane"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/secrets"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/restapi"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	"github.com/wso2/go-httpkit/httputil"
)

// APIServer implements the generated ServerInterface
type APIServer struct {
	*RestAPIHandler // embedded — promotes CreateRestAPI, ListRestAPIs, GetRestAPIById, UpdateRestAPI, DeleteRestAPI

	restAPIService              *restapi.RestAPIService
	store                       *storage.ConfigStore
	db                          storage.Storage
	snapshotManager             *xds.SnapshotManager
	policyManager               *policyxds.PolicyManager
	policyDefinitions           map[string]models.PolicyDefinition // key name|version
	policyDefMu                 sync.RWMutex
	parser                      *config.Parser
	validator                   config.Validator
	logger                      *slog.Logger
	deploymentService           *utils.APIDeploymentService
	mcpDeploymentService        *utils.MCPDeploymentService
	llmDeploymentService        *utils.LLMDeploymentService
	secretService               *secrets.SecretService
	apiKeyService               *utils.APIKeyService
	apiKeyXDSManager            *apikeyxds.APIKeyStateManager
	controlPlaneClient          controlplane.ControlPlaneClient
	routerConfig                *config.RouterConfig
	httpClient                  *http.Client
	systemConfig                *config.Config
	eventHub                    eventhub.EventHub
	gatewayID                   string
	subscriptionSnapshotUpdater utils.SubscriptionSnapshotUpdater
	subscriptionResourceService *utils.SubscriptionResourceService
}

// NewAPIServer creates a new API server with dependencies
func NewAPIServer(
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
	policyManager *policyxds.PolicyManager,
	lazyResourceManager *lazyresourcexds.LazyResourceStateManager,
	logger *slog.Logger,
	controlPlaneClient controlplane.ControlPlaneClient,
	policyDefinitions map[string]models.PolicyDefinition,
	templateDefinitions map[string]*api.LLMProviderTemplate,
	validator config.Validator,
	apiKeyXDSManager *apikeyxds.APIKeyStateManager,
	systemConfig *config.Config,
	eventHub eventhub.EventHub,
	subscriptionSnapshotUpdater utils.SubscriptionSnapshotUpdater,
	secretService *secrets.SecretService,
	restAPIService *restapi.RestAPIService,
) *APIServer {
	if db == nil {
		panic("APIServer requires non-nil storage")
	}
	if eventHub == nil {
		panic("APIServer requires non-nil EventHub")
	}
	if systemConfig == nil {
		panic("APIServer requires non-nil system config")
	}
	gatewayID := strings.TrimSpace(systemConfig.Controller.Server.GatewayID)
	if gatewayID == "" {
		panic("APIServer requires non-empty gateway ID")
	}

	deploymentService := utils.NewAPIDeploymentService(store, db, snapshotManager, validator, &systemConfig.Router, eventHub, gatewayID, secretService)
	apiKeyService := utils.NewAPIKeyService(store, db, apiKeyXDSManager, &systemConfig.APIKey, eventHub, gatewayID)
	subscriptionResourceService := utils.NewSubscriptionResourceService(db, subscriptionSnapshotUpdater, eventHub, gatewayID)

	policyVersionResolver := utils.NewLoadedPolicyVersionResolver(policyDefinitions)
	policyValidator := config.NewPolicyValidator(policyDefinitions)
	parser := config.NewParser()
	httpClient := &http.Client{Timeout: 10 * time.Second}
	routerConfig := &systemConfig.Router
	mcpDeploymentService := utils.NewMCPDeploymentService(store, db, snapshotManager, policyManager, policyValidator, eventHub, gatewayID, secretService)

	server := &APIServer{
		store:                store,
		db:                   db,
		snapshotManager:      snapshotManager,
		policyManager:        policyManager,
		policyDefinitions:    policyDefinitions,
		parser:               parser,
		validator:            validator,
		logger:               logger,
		deploymentService:    deploymentService,
		mcpDeploymentService: mcpDeploymentService,
		llmDeploymentService: utils.NewLLMDeploymentService(store, db, snapshotManager, lazyResourceManager, templateDefinitions,
			deploymentService, routerConfig, policyVersionResolver, policyValidator),
		secretService:               secretService,
		apiKeyService:               apiKeyService,
		apiKeyXDSManager:            apiKeyXDSManager,
		controlPlaneClient:          controlPlaneClient,
		routerConfig:                routerConfig,
		httpClient:                  httpClient,
		systemConfig:                systemConfig,
		eventHub:                    eventHub,
		gatewayID:                   gatewayID,
		subscriptionSnapshotUpdater: subscriptionSnapshotUpdater,
		subscriptionResourceService: subscriptionResourceService,
	}
	// Wire the DP->CP push into the LLM/MCP deployment services so create flows push to the
	// control plane from the service layer (mirroring the REST API service), instead of the
	// handler doing it. This keeps the push behavior identical whether an artifact is created
	// via these handlers or directly through the service layer (e.g. the immutable loader).
	pushEnabled := systemConfig.Controller.ControlPlane.DeploymentSyncEnabled
	server.mcpDeploymentService.SetControlPlanePusher(controlPlaneClient, pushEnabled)
	server.llmDeploymentService.SetControlPlanePusher(controlPlaneClient, pushEnabled)

	server.restAPIService = restAPIService
	server.RestAPIHandler = NewRestAPIHandler(restAPIService, logger)
	// Wire the shared control-plane (DP->CP) push hooks so REST APIs use the same push
	// path (APIServer.waitForDeploymentAndPush / pushArtifactUndeploy) as all other kinds.
	server.RestAPIHandler.pushArtifactUndeploy = server.pushArtifactUndeploy

	// Register status update callback
	snapshotManager.SetStatusCallback(server.handleStatusUpdate)

	return server
}

func (s *APIServer) getSubscriptionResourceService() *utils.SubscriptionResourceService {
	if s.subscriptionResourceService != nil {
		return s.subscriptionResourceService
	}

	s.subscriptionResourceService = utils.NewSubscriptionResourceService(s.db, s.subscriptionSnapshotUpdater, s.eventHub, s.gatewayID)

	return s.subscriptionResourceService
}

// handleStatusUpdate is called by SnapshotManager after xDS deployment
func (s *APIServer) handleStatusUpdate(configID string, success bool, correlationID string) {
	// Create a logger with correlation ID if provided
	log := s.logger
	if correlationID != "" {
		log = s.logger.With(slog.String("correlation_id", correlationID))
	}

	cfg, err := s.store.Get(configID)
	if err != nil {
		log.Warn("Config not found for status update", slog.String("id", configID))
		return
	}

	if success {
		log.Info("Configuration deployed successfully",
			slog.String("id", configID),
			slog.String("kind", cfg.Kind),
			slog.String("handle", cfg.Handle))
	} else {
		log.Error("Configuration deployment failed",
			slog.String("id", configID),
			slog.String("kind", cfg.Kind),
			slog.String("handle", cfg.Handle))
	}
}

// GetXDSSyncStatus implements the GET /xds_sync_status endpoint.
func (s *APIServer) GetXDSSyncStatus(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, s.GetXDSSyncStatusResponse())
}

// GetXDSSyncStatusResponse builds the xDS sync status response payload.
func (s *APIServer) GetXDSSyncStatusResponse() adminapi.XDSSyncStatusResponse {
	timestamp := time.Now()
	component := "gateway-controller"
	policyChainVersion := s.getPolicyChainVersionString()

	return adminapi.XDSSyncStatusResponse{
		Component:          &component,
		Timestamp:          &timestamp,
		PolicyChainVersion: &policyChainVersion,
	}
}

func (s *APIServer) SearchDeployments(w http.ResponseWriter, r *http.Request, kind string) {
	filterKeys := []string{"displayName", "version", "context", "status"}
	filters := make(map[string]string)
	for _, k := range filterKeys {
		if v := r.URL.Query().Get(k); v != "" {
			filters[k] = v
		}
	}

	if s.store == nil {
		httputil.WriteJSON(w, http.StatusOK, map[string]any{
			"status": "success",
			"count":  0,
		})
		return
	}

	configs := s.store.GetAllByKind(kind)
	if kind == string(api.MCPProxyConfigurationKindMcp) && s.mcpDeploymentService != nil {
		var err error
		configs, err = s.mcpDeploymentService.ListMCPProxies()
		if err != nil {
			s.logger.Error("Failed to list MCP proxies", slog.Any("error", err))
			httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Failed to list MCP proxies",
			})
			return
		}
	}

	// Filter configs and build the k8s-style response items once, independent of kind.
	items := make([]any, 0, len(configs))
	for _, cfg := range configs {
		if v, ok := filters["displayName"]; ok && cfg.DisplayName != v {
			continue
		}
		if v, ok := filters["version"]; ok && cfg.Version != v {
			continue
		}
		if v, ok := filters["context"]; ok {
			cfgContext, err := cfg.GetContext()
			if err != nil {
				s.logger.Warn("Failed to get context for config",
					slog.String("id", cfg.UUID),
					slog.String("displayName", cfg.DisplayName),
					slog.Any("error", err))
				continue
			}
			if cfgContext != v {
				continue
			}
		}
		if v, ok := filters["status"]; ok && string(cfg.DesiredState) != v {
			continue
		}

		if kind == string(api.MCPProxyConfigurationKindMcp) {
			mcp, err := rematerializeMCPProxyConfig(s.logger, cfg.UUID, cfg.DisplayName, cfg.SourceConfiguration)
			if err != nil {
				httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
					Status:  "error",
					Message: "Failed to get stored MCP configuration",
				})
				return
			}
			items = append(items, buildResourceResponseFromStored(mcp, cfg))
			continue
		}

		items = append(items, buildResourceResponseFromStored(cfg.SourceConfiguration, cfg))
	}

	// Each kind has its own envelope key to preserve the existing URL contract.
	envelopeKey := "apis"
	switch kind {
	case string(api.MCPProxyConfigurationKindMcp):
		envelopeKey = "mcpProxies"
	case "WebSubApi":
		envelopeKey = "websubApis"
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"status":    "success",
		"count":     len(items),
		envelopeKey: items,
	})
}

// GetAPIByNameVersion implements ServerInterface.GetAPIByNameVersion
// (GET /apis/{name}/{version})
func (s *APIServer) GetAPIByNameVersion(w http.ResponseWriter, r *http.Request, name string, version string) {
	// Get correlation-aware logger from context
	log := middleware.GetLogger(r, s.logger)

	cfg, err := s.store.GetByKindNameAndVersion(models.KindRestApi, name, version)
	if err != nil || cfg == nil {
		log.Warn("API configuration not found",
			slog.String("name", name),
			slog.String("version", version))
		httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("RestAPI with name '%s' and version '%s' not found", name, version),
		})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, buildResourceResponseFromStored(cfg.SourceConfiguration, cfg))
}

// pushArtifactUndeploy notifies the control plane that a gateway-originated artifact
// has been deleted from this gateway. The control plane keeps the artifact but marks
// it undeployed (it is not removed and can be re-deployed later). It is a no-op for
// control-plane-originated artifacts or when push is disabled / disconnected.
// deploymentPusher builds the handlerkit.DeploymentPusher for this server's
// current dependencies. pushArtifactUndeploy/waitForDeploymentAndPush delegate
// to it so the shared push logic lives in one place (handlerkit), reusable by
// any binary that imports gateway-controller as a library.
func (s *APIServer) deploymentPusher() *handlerkit.DeploymentPusher {
	return &handlerkit.DeploymentPusher{
		Store:              s.store,
		ControlPlaneClient: s.controlPlaneClient,
		SystemConfig:       s.systemConfig,
	}
}

func (s *APIServer) pushArtifactUndeploy(cfg *models.StoredConfig, log *slog.Logger) {
	s.deploymentPusher().PushArtifactUndeploy(cfg, log)
}

// waitForDeploymentAndPush waits for API deployment to complete and pushes it to the control plane
// This is only called for APIs created directly via gateway endpoint (not from platform API).
//
// minDeployedAt is the DeployedAt of the deployment this push was triggered for.
func (s *APIServer) waitForDeploymentAndPush(configID string, correlationID string, minDeployedAt *time.Time, log *slog.Logger) {
	s.deploymentPusher().WaitForDeploymentAndPush(configID, correlationID, minDeployedAt, log)
}

// publishWebSubEvent publishes an event for WebSub API lifecycle changes.
func (s *APIServer) publishWebSubEvent(eventType eventhub.EventType, action, entityID, correlationID string, logger *slog.Logger) {
	(&handlerkit.EventPublisher{EventHub: s.eventHub, GatewayID: s.gatewayID}).PublishEvent(eventType, action, entityID, correlationID, logger)
}

// GetConfigDump implements the GET /config_dump endpoint
func (s *APIServer) GetConfigDump(w http.ResponseWriter, r *http.Request) {
	log := middleware.GetLogger(r, s.logger)
	log.Info("Retrieving configuration dump")

	response, err := s.BuildConfigDumpResponse(log)
	if err != nil {
		log.Error("Failed to retrieve configuration dump", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	jsonBytes, err := json.Marshal(*response)
	if err != nil {
		log.Error("Failed to marshal configuration dump", slog.Any("error", err))
		httputil.WriteJSON(w, http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	sensitiveValues := s.store.GetAllSensitiveValues()
	redacted := redact.Redact(string(jsonBytes), sensitiveValues)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(redacted)) //nolint:errcheck
	log.Info("Configuration dump retrieved successfully",
		slog.Int("apis", len(*response.Apis)),
		slog.Int("policies", len(*response.Policies)),
		slog.Int("certificates", len(*response.Certificates)))
}

// BuildConfigDumpResponse builds the complete configuration dump response payload.
func (s *APIServer) BuildConfigDumpResponse(log *slog.Logger) (*adminapi.ConfigDumpResponse, error) {
	log.Info("Retrieving configuration dump")

	// Get all APIs
	allConfigs := s.store.GetAll()

	// Build API list with metadata using the generated types
	apisSlice := make([]adminapi.ConfigDumpAPIItem, 0, len(allConfigs))

	for _, cfg := range allConfigs {
		// Use handle (metadata.name) as the id in the dump
		configHandle := cfg.Handle
		if configHandle == "" {
			log.Warn("Config missing handle, skipping in dump", slog.String("id", cfg.UUID))
			continue
		}

		// Convert desired state to the admin API status type
		var status adminapi.ConfigDumpAPIMetadataStatus
		switch cfg.DesiredState {
		case models.StateDeployed:
			status = adminapi.Deployed
		case models.StateUndeployed:
			status = adminapi.Undeployed
		default:
			status = adminapi.Deployed
		}

		configuration, err := toGenericMap(cfg.Configuration)
		if err != nil {
			return nil, fmt.Errorf("failed to convert API configuration: %w", err)
		}

		item := adminapi.ConfigDumpAPIItem{
			Configuration: &configuration,
			Id:            convertHandleToUUID(configHandle),
			Metadata: &adminapi.ConfigDumpAPIMetadata{
				CreatedAt:  &cfg.CreatedAt,
				UpdatedAt:  &cfg.UpdatedAt,
				DeployedAt: cfg.DeployedAt,
				Status:     &status,
			},
		}
		apisSlice = append(apisSlice, item)
	}

	// Get all policies (excluding system policies)
	s.policyDefMu.RLock()
	policies := make([]map[string]interface{}, 0, len(s.policyDefinitions))
	for _, policy := range s.policyDefinitions {
		policyMap, err := toGenericMap(policy)
		if err != nil {
			s.policyDefMu.RUnlock()
			return nil, fmt.Errorf("failed to convert policy definition: %w", err)
		}
		policies = append(policies, policyMap)
	}
	s.policyDefMu.RUnlock()

	// Sort policies for consistent output
	sort.Slice(policies, func(i, j int) bool {
		nameI, _ := policies[i]["name"].(string)
		nameJ, _ := policies[j]["name"].(string)
		if nameI == nameJ {
			versionI, _ := policies[i]["version"].(string)
			versionJ, _ := policies[j]["version"].(string)
			return versionI < versionJ
		}
		return nameI < nameJ
	})

	// Get all certificates
	certificates := make([]adminapi.CertificateResponse, 0)
	totalBytes := 0

	certs, err := s.db.ListCertificates()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve certificates: %w", err)
	}

	for _, cert := range certs {
		totalBytes += len(cert.Certificate)

		certStatus := "success"
		certificates = append(certificates, adminapi.CertificateResponse{
			Id:       &cert.UUID,
			Name:     &cert.Name,
			Subject:  &cert.Subject,
			Issuer:   &cert.Issuer,
			NotAfter: &cert.NotAfter,
			Count:    &cert.CertCount,
			Status:   &certStatus,
		})
	}

	// Calculate statistics
	totalApis := len(apisSlice)
	totalPolicies := len(policies)
	totalCertificates := len(certificates)

	timestamp := time.Now()
	status := "success"
	policyChainVersion := s.getPolicyChainVersionString()

	// Build response
	response := &adminapi.ConfigDumpResponse{
		Status:       &status,
		Timestamp:    &timestamp,
		Apis:         &apisSlice,
		Policies:     &policies,
		Certificates: &certificates,
		Statistics: &struct {
			TotalApis             *int `json:"totalApis,omitempty" yaml:"totalApis,omitempty"`
			TotalCertificateBytes *int `json:"totalCertificateBytes,omitempty" yaml:"totalCertificateBytes,omitempty"`
			TotalCertificates     *int `json:"totalCertificates,omitempty" yaml:"totalCertificates,omitempty"`
			TotalPolicies         *int `json:"totalPolicies,omitempty" yaml:"totalPolicies,omitempty"`
		}{
			TotalApis:             &totalApis,
			TotalPolicies:         &totalPolicies,
			TotalCertificates:     &totalCertificates,
			TotalCertificateBytes: &totalBytes,
		},
		XdsSync: &adminapi.ConfigDumpXDSSync{
			PolicyChainVersion: &policyChainVersion,
		},
	}

	return response, nil
}

func (s *APIServer) getPolicyChainVersionString() string {
	if s.policyManager == nil {
		return "0"
	}
	return strconv.FormatInt(s.policyManager.GetResourceVersion(), 10)
}

func ptr[T any](v T) *T { return &v }

// extractAuthenticatedUser extracts and validates the authenticated user from the request context.
// Returns the AuthContext object and handles error responses automatically.
func (s *APIServer) extractAuthenticatedUser(w http.ResponseWriter, r *http.Request, operationName string, correlationID string) (*commonmodels.AuthContext, bool) {
	return handlerkit.ExtractAuthenticatedUser(w, r, s.logger, operationName, correlationID)
}

// bindRequestBody binds the request body based on Content-Type header.
// Supports both JSON and YAML content types.
// Handles Content-Type headers case-insensitively and strips parameters (e.g., charset).
func (s *APIServer) bindRequestBody(r *http.Request, request interface{}) error {
	return handlerkit.BindRequestBody(r, request)
}

// getLLMProviderTemplate extracts the template name from sourceConfig and retrieves the template.
// Returns the template configuration if found, nil otherwise.
func (s *APIServer) getLLMProviderTemplate(sourceConfig any) (*api.LLMProviderTemplate, error) {
	if sourceConfig == nil {
		return nil, fmt.Errorf("sourceConfig is nil")
	}

	// Try to extract the template name from sourceConfig
	// and get the template from the store
	templateName, err := utils.GetValueFromSourceConfig(sourceConfig, "spec.template")
	if err != nil {
		return nil, fmt.Errorf("failed to extract template name: %w", err)
	}
	templateNameStr, ok := templateName.(string)
	if !ok {
		return nil, fmt.Errorf("template name is not a string: %v", templateName)
	}
	if templateNameStr == "" {
		return nil, fmt.Errorf("template name is empty")
	}

	storedTemplate, err := s.store.GetTemplateByHandle(templateNameStr)
	if err != nil {
		return nil, fmt.Errorf("failed to get template '%s' from store: %w", templateNameStr, err)
	}

	return &storedTemplate.Configuration, nil
}

// populatePropsForSystemPolicies populates the props for system policies
// based on the source configuration
// Note: Template handle is now passed via route metadata instead of props
func (s *APIServer) populatePropsForSystemPolicies(srcConfig any, props map[string]any) {
	if srcConfig == nil {
		return
	}
	// Template handle is now extracted and added to route metadata in translator.go
	// No need to pass template via props anymore
}

func toGenericMap(value interface{}) (map[string]interface{}, error) {
	bytes, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(bytes, &result); err != nil {
		return nil, err
	}
	return result, nil
}
