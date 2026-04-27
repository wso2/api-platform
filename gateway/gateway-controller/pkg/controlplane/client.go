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

package controlplane

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine/funcs"

	"github.com/gorilla/websocket"
	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

// State represents the connection state
type State int

const (
	// Disconnected state - no connection
	Disconnected State = iota
	// Connecting state - attempting to establish connection
	Connecting
	// Connected state - active connection
	Connected
	// Reconnecting state - attempting to reconnect after failure
	Reconnecting
)

// String returns the string representation of the state
func (s State) String() string {
	switch s {
	case Disconnected:
		return "disconnected"
	case Connecting:
		return "connecting"
	case Connected:
		return "connected"
	case Reconnecting:
		return "reconnecting"
	default:
		return "unknown"
	}
}

// ConnectionState holds the current state and metadata for the WebSocket connection
type ConnectionState struct {
	Current        State           // Current connection state
	Conn           *websocket.Conn // Active WebSocket connection (nil if not connected)
	LastConnected  time.Time       // Timestamp of last successful connection
	LastHeartbeat  int64           // Unix timestamp of last pong received (atomic)
	RetryCount     int             // Consecutive retry attempts
	NextRetryDelay time.Duration   // Backoff delay for next retry
	GatewayID      string          // Gateway UUID from connection.ack
	ConnectionID   string          // Connection UUID from connection.ack
	mu             sync.RWMutex    // Protects state transitions
}

// ControlPlaneClient interface defines the methods needed from the control plane client
type ControlPlaneClient interface {
	IsConnected() bool
	PushAPIDeployment(apiID string, apiConfig *models.StoredConfig, deploymentID string) error
	SyncArtifactsToOnPremAPIM(apimConfig *utils.APIMConfig) error
	IsOnPrem() bool
	GetAPIMConfig() *utils.APIMConfig
}

// Client manages the WebSocket connection to the control plane
type Client struct {
	config                      config.ControlPlaneConfig
	logger                      *slog.Logger
	state                       *ConnectionState
	ctx                         context.Context
	cancel                      context.CancelFunc
	stopChan                    chan struct{}
	wg                          sync.WaitGroup
	writeMu                     sync.Mutex // serializes writes to the WebSocket connection
	store                       *storage.ConfigStore
	db                          storage.Storage
	snapshotManager             *xds.SnapshotManager
	parser                      *config.Parser
	validator                   config.Validator
	deploymentService           *utils.APIDeploymentService
	apiUtilsService             *utils.APIUtilsService
	apiKeyService               *utils.APIKeyService
	llmDeploymentService        *utils.LLMDeploymentService
	mcpDeploymentService        *utils.MCPDeploymentService
	apiKeyXDSManager            utils.XDSManager
	apiKeyStore                 *storage.APIKeyStore
	routerConfig                *config.RouterConfig
	policyManager               *policyxds.PolicyManager
	systemConfig                *config.Config
	policyDefinitions           map[string]models.PolicyDefinition
	subscriptionSnapshotUpdater utils.SubscriptionSnapshotUpdater
	subscriptionResourceService *utils.SubscriptionResourceService
	eventHub                    eventhub.EventHub
	gatewayID                   string
	gatewayPath                 string      // cached gateway path from well-known discovery
	syncOnce                    sync.Once   // ensures deployment sync runs only on first connect
	isFirstConnect              atomic.Bool // true on first connect, flipped to false after
}

// NewClient creates a new control plane client
func NewClient(
	cfg config.ControlPlaneConfig,
	logger *slog.Logger,
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
	validator config.Validator,
	routerConfig *config.RouterConfig,
	apiKeyXDSManager utils.XDSManager,
	apiKeyStore *storage.APIKeyStore,
	apiKeyConfig *config.APIKeyConfig,
	policyManager *policyxds.PolicyManager,
	systemConfig *config.Config,
	policyDefinitions map[string]models.PolicyDefinition,
	lazyResourceManager *lazyresourcexds.LazyResourceStateManager,
	templateDefinitions map[string]*api.LLMProviderTemplate,
	subSnapshotManager utils.SubscriptionSnapshotUpdater,
	eventHubInstance eventhub.EventHub,
	secretResolver funcs.SecretResolver,
) *Client {
	if db == nil {
		panic("control plane client requires non-nil storage")
	}
	if eventHubInstance == nil {
		panic("control plane client requires non-nil EventHub")
	}
	if systemConfig == nil {
		panic("control plane client requires non-nil system config")
	}
	gatewayID := strings.TrimSpace(systemConfig.Controller.Server.GatewayID)
	if gatewayID == "" {
		panic("control plane client requires non-empty gateway ID")
	}

	ctx, cancel := context.WithCancel(context.Background())

	deploymentService := utils.NewAPIDeploymentService(store, db, snapshotManager, validator, routerConfig, eventHubInstance, gatewayID, secretResolver)
	apiKeyService := utils.NewAPIKeyService(store, db, apiKeyXDSManager, apiKeyConfig, eventHubInstance, gatewayID)
	subscriptionResourceService := utils.NewSubscriptionResourceService(db, subSnapshotManager, eventHubInstance, gatewayID)

	client := &Client{
		config:                      cfg,
		logger:                      logger,
		store:                       store,
		db:                          db,
		snapshotManager:             snapshotManager,
		parser:                      config.NewParser(),
		validator:                   validator,
		deploymentService:           deploymentService,
		apiKeyService:               apiKeyService,
		apiKeyXDSManager:            apiKeyXDSManager,
		apiKeyStore:                 apiKeyStore,
		routerConfig:                routerConfig,
		policyManager:               policyManager,
		systemConfig:                systemConfig,
		policyDefinitions:           policyDefinitions,
		subscriptionSnapshotUpdater: subSnapshotManager,
		subscriptionResourceService: subscriptionResourceService,
		eventHub:                    eventHubInstance,
		gatewayID:                   gatewayID,
		state: &ConnectionState{
			Current:        Disconnected,
			Conn:           nil,
			LastConnected:  time.Time{},
			LastHeartbeat:  0,
			RetryCount:     0,
			NextRetryDelay: cfg.ReconnectInitial,
		},
		ctx:      ctx,
		cancel:   cancel,
		stopChan: make(chan struct{}),
	}

	client.isFirstConnect.Store(true)

	policyVersionResolver := utils.NewLoadedPolicyVersionResolver(policyDefinitions)
	policyValidator := config.NewPolicyValidator(policyDefinitions)
	client.llmDeploymentService = utils.NewLLMDeploymentService(
		store,
		db,
		snapshotManager,
		lazyResourceManager,
		templateDefinitions,
		client.deploymentService,
		routerConfig,
		policyVersionResolver,
		policyValidator,
	)
	client.mcpDeploymentService = utils.NewMCPDeploymentService(
		store,
		db,
		snapshotManager,
		policyManager,
		policyValidator,
		eventHubInstance,
		gatewayID,
	)

	// Initialize API utils service with the proper base URL using the method
	client.apiUtilsService = utils.NewAPIUtilsService(utils.PlatformAPIConfig{
		BaseURL:            client.getRestAPIBaseURL(),
		Token:              "",
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		Timeout:            30 * time.Second,
	}, logger)

	// Set OAuth2 credentials for on-prem APIM (for API import operations)
	// Construct TokenURL from the controlplane host
	if cfg.Host != "" {
		client.apiUtilsService.TokenURL = fmt.Sprintf("https://%s/oauth2/token", cfg.Host)
	}
	client.apiUtilsService.ClientID = ""
	client.apiUtilsService.ClientSecret = ""
	client.apiUtilsService.Username = ""
	client.apiUtilsService.Password = ""

	return client
}

func (c *Client) getSubscriptionResourceService() *utils.SubscriptionResourceService {
	if c.subscriptionResourceService != nil {
		return c.subscriptionResourceService
	}

	c.subscriptionResourceService = utils.NewSubscriptionResourceService(c.db, c.subscriptionSnapshotUpdater, c.eventHub, c.gatewayID)

	return c.subscriptionResourceService
}

// GetAPIMConfig returns the APIM configuration for on-prem APIM publisher API related operations
// Uses the control plane TLS settings as defaults
func (c *Client) GetAPIMConfig() *utils.APIMConfig {
	return &utils.APIMConfig{
		InsecureSkipVerify: c.config.InsecureSkipVerify,
		Timeout:            30 * time.Second,
		Host:               c.config.Host,
		GatewayName:        c.config.GatewayName,
		ClientID:           c.config.ApimOAuth2ClientID,
		ClientSecret:       c.config.ApimOAuth2ClientSecret,
		Username:           c.config.ApimOAuth2Username,
		Password:           c.config.ApimOAuth2Password,
		TokenURL:           fmt.Sprintf("https://%s/oauth2/token", c.config.Host),
	}
}

// Start initiates the connection to the control plane
func (c *Client) Start() error {
	// Check if token is configured
	if c.config.Token == "" {
		c.logger.Info("Control plane token not configured, skipping connection")
		return nil
	}

	c.logger.Info("Starting control plane client",
		slog.String("host", c.config.Host),
		slog.String("websocket_url", c.getWebSocketConnectURL()),
	)

	// Start connection in background
	c.wg.Add(1)
	go c.connectionLoop()

	return nil
}

// Stop gracefully stops the control plane client
func (c *Client) Stop() {
	c.logger.Info("Stopping control plane client")

	// Signal shutdown
	close(c.stopChan)
	c.cancel()

	// Close active connection if exists: nil out conn under state.mu first so no
	// new sendMessage call can obtain it, then acquire writeMu to drain any
	// in-flight write before sending the close frame.
	c.state.mu.Lock()
	conn := c.state.Conn
	c.state.Conn = nil
	c.state.mu.Unlock()

	if conn != nil {
		closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Gateway shutting down")
		c.writeMu.Lock()
		_ = conn.WriteMessage(websocket.CloseMessage, closeMsg)
		c.writeMu.Unlock()
		_ = conn.Close()
	}

	// Wait for goroutines to finish
	c.wg.Wait()

	c.logger.Info("Control plane client stopped")
}

// Connect establishes a WebSocket connection to the control plane
func (c *Client) Connect() error {
	c.setState(Connecting)

	wsURL := c.resolveWebSocketConnectURL()
	c.logger.Info("Connecting to control plane",
		slog.String("url", wsURL),
		slog.Int("retry_count", c.state.RetryCount),
	)

	// Create WebSocket dialer with timeout
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{ // #nosec G402 -- Explicit operator-controlled opt-out for dev/test environments.
			InsecureSkipVerify: c.config.InsecureSkipVerify,
		},
	}

	// Log TLS configuration
	if c.config.InsecureSkipVerify {
		c.logger.Warn("TLS certificate verification disabled (insecure_skip_verify=true)")
	}

	// Add api-key header for authentication
	headers := http.Header{}
	headers.Add("api-key", c.config.Token)

	// Dial WebSocket: URL from well-known discovery with fallback to default path.
	conn, resp, err := dialer.Dial(wsURL, headers)
	if err != nil {
		if resp != nil {
			c.logger.Error("WebSocket connection failed",
				slog.Any("error", err),
				slog.Int("status_code", resp.StatusCode),
			)

			// Handle authentication failures
			if resp.StatusCode == http.StatusUnauthorized {
				c.logger.Error("Authentication failed - invalid or revoked token",
					slog.String("troubleshooting", "Check GATEWAY_REGISTRATION_TOKEN environment variable"),
				)
				return fmt.Errorf("authentication failed: %w", err)
			}
		} else {
			c.logger.Error("WebSocket connection failed",
				slog.Any("error", err),
			)
		}
		return err
	}

	// Store connection
	c.state.mu.Lock()
	c.state.Conn = conn
	c.state.LastConnected = time.Now()
	atomic.StoreInt64(&c.state.LastHeartbeat, time.Now().Unix())
	c.state.mu.Unlock()

	// Setup ping handler for heartbeat
	// When server sends PING, gorilla/websocket automatically sends PONG
	// and triggers this handler so we can update the heartbeat timestamp
	conn.SetPingHandler(func(appData string) error {
		atomic.StoreInt64(&c.state.LastHeartbeat, time.Now().Unix())
		// Return the default pong handler to send PONG response
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
	})

	// Wait for connection.ack message
	if err := c.waitForConnectionAck(conn); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			c.logger.Warn("Failed to close connection after missing connection.ack", slog.Any("error", closeErr))
		}
		c.state.mu.Lock()
		c.state.Conn = nil
		c.state.mu.Unlock()
		return fmt.Errorf("failed to receive connection.ack: %w", err)
	}

	// Transition to connected state
	c.setState(Connected)
	c.state.RetryCount = 0 // Reset retry count on successful connection

	c.logger.Info("Control plane connection established",
		slog.String("gateway_id", c.state.GatewayID),
		slog.String("connection_id", c.state.ConnectionID),
	)

	// Capture a stable gateway ID for background sync work to avoid
	// reading mutable state from goroutines.
	gatewayID := c.state.GatewayID

	// On first connect (startup): run deployment sync first, then subscription
	// sync sequentially — subscriptions reference APIs via FK, so they must
	// wait for deployments to land. On reconnects: only subscription sync runs
	// (deployment reconnect sync is deferred; see proposal doc).
	c.syncOnce.Do(func() {
		c.wg.Add(1)
		go func(gwID string) {
			defer c.wg.Done()
			c.syncDeployments(gwID)
			// Bottom-up sync: push gateway-created APIs to on-prem control plane
			if c.IsOnPrem() {
				if err := c.SyncArtifactsToOnPremAPIM(c.GetAPIMConfig()); err != nil {
					c.logger.Error("Failed to sync artifacts to on-prem APIM", slog.Any("error", err))
				}
			}
			c.syncSubscriptionPlans(gwID)
			c.syncSubscriptionsForExistingAPIs(gwID)
			// Sync API keys for LlmProvider, LlmProxy, and RestApi artifacts.
			c.syncAPIKeysForExistingArtifacts(gwID)
		}(gatewayID)
	})

	// On reconnects (syncOnce already fired): sync subscriptions independently.
	// Uses atomic flag to skip on first connect since syncOnce goroutine handles it.
	if !c.isFirstConnect.CompareAndSwap(true, false) {
		c.wg.Add(1)
		go func(gwID string) {
			defer c.wg.Done()
			// Bottom-up sync on reconnect
			if c.IsOnPrem() {
				if err := c.SyncArtifactsToOnPremAPIM(c.GetAPIMConfig()); err != nil {
					c.logger.Error("Failed to sync artifacts to on-prem APIM", slog.Any("error", err))
				}
			}
			c.syncSubscriptionPlans(gwID)
			c.syncSubscriptionsForExistingAPIs(gwID)
			// Sync API keys for LlmProvider, LlmProxy, and RestApi artifacts.
			c.syncAPIKeysForExistingArtifacts(gwID)
		}(gatewayID)
	}

	// Push gateway manifest to the control plane on connect
	c.wg.Add(1)
	go func(gwID string) {
		defer c.wg.Done()
		c.pushGatewayManifestOnConnect(gwID)
	}(gatewayID)

	// Start heartbeat monitor
	c.wg.Add(1)
	go c.heartbeatMonitor()

	return nil
}

// gatewayWellKnownResponse matches APIM well-known JSON: {"gatewayPath":"internal/data/v1"}.
// Extra fields from the server are ignored.
type gatewayWellKnownResponse struct {
	GatewayPath string `json:"gatewayPath"`
}

// resolveWebSocketConnectURL resolves the registration URL using the control plane well-known endpoint.
// Falls back to the default configured URL when discovery fails.
// The discovered gateway path is cached for reuse within the file.
func (c *Client) resolveWebSocketConnectURL() string {
	// Use cached gateway path if available (read under lock)
	c.state.mu.RLock()
	cachedPath := c.gatewayPath
	c.state.mu.RUnlock()

	if cachedPath != "" {
		resolvedURL := fmt.Sprintf("wss://%s%s/ws/gateways/connect", c.config.Host, cachedPath)
		c.logger.Debug("Using cached gateway path for WebSocket connect URL",
			slog.String("gateway_path", cachedPath),
			slog.String("resolved_url", resolvedURL),
		)
		return resolvedURL
	}

	gatewayPath, err := c.discoverGatewayPath()
	if err != nil {
		c.logger.Debug("Failed to resolve gateway path from well-known endpoint, falling back to configured URL",
			slog.Any("error", err),
		)
		return c.getWebSocketConnectURL()
	}

	// Cache the discovered gateway path for future use (write under lock)
	c.state.mu.Lock()
	c.gatewayPath = gatewayPath
	c.state.mu.Unlock()

	// Update apiUtilsService base URL to use the discovered gateway path
	c.apiUtilsService.SetBaseURL(c.getRestAPIBaseURL())

	resolvedURL := fmt.Sprintf("wss://%s%s/ws/gateways/connect", c.config.Host, gatewayPath)
	c.logger.Debug("Resolved WebSocket connect URL from well-known endpoint",
		slog.String("gateway_path", gatewayPath),
		slog.String("resolved_url", resolvedURL),
	)

	return resolvedURL
}

// GetGatewayPath returns the cached gateway path discovered from the well-known endpoint.
// Returns an empty string if the path has not been discovered yet.
func (c *Client) GetGatewayPath() string {
	c.state.mu.RLock()
	defer c.state.mu.RUnlock()
	return c.gatewayPath
}

// isOnPrem returns true when the control plane is an on-prem deployment.
// IsOnPrem returns true if the gateway is connected to an on-prem control plane.
func (c *Client) IsOnPrem() bool {
	return c.GetGatewayPath() != ""
}

// Deprecated: use IsOnPrem() instead
func (c *Client) isOnPrem() bool {
	return c.IsOnPrem()
}

// discoverGatewayPath fetches the gateway websocket base path from the control plane well-known endpoint.
func (c *Client) discoverGatewayPath() (string, error) {
	wellKnownURL := fmt.Sprintf("https://%s/internal/gateway/.well-known", c.config.Host)

	httpClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{ // #nosec G402 -- Explicit operator-controlled opt-out for dev/test environments.
				InsecureSkipVerify: c.config.InsecureSkipVerify,
			},
		},
	}

	req, err := http.NewRequestWithContext(c.ctx, http.MethodGet, wellKnownURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create well-known request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call well-known endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("well-known endpoint returned non-200 status: %d", resp.StatusCode)
	}

	var wellKnownResp gatewayWellKnownResponse
	if err := json.NewDecoder(resp.Body).Decode(&wellKnownResp); err != nil {
		return "", fmt.Errorf("failed to decode well-known response: %w", err)
	}

	gatewayPath := normalizeGatewayPath(wellKnownResp.GatewayPath)
	if gatewayPath == "" {
		return "", fmt.Errorf("well-known response missing gatewayPath")
	}

	return gatewayPath, nil
}

func normalizeGatewayPath(path string) string {
	trimmed := strings.Trim(strings.TrimSpace(path), "/")
	if trimmed == "" {
		return ""
	}
	return "/" + trimmed
}

// sendMessage writes a message to the WebSocket connection with proper serialization
func (c *Client) sendMessage(message []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	c.state.mu.RLock()
	conn := c.state.Conn
	c.state.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("no active WebSocket connection")
	}

	return conn.WriteMessage(websocket.TextMessage, message)
}

// sendDeploymentAck sends a deployment acknowledgement message back to the control plane
func (c *Client) sendDeploymentAck(deploymentID, artifactID, resourceType, action, status string, performedAt time.Time, errorCode string) {
	// Skip ack if control plane is not configured — no connection will be active.
	if c.config.Host == "" {
		return
	}
	ack := DeploymentAckMessage{
		Type: "deployment.ack",
		Payload: DeploymentAckPayload{
			DeploymentID: deploymentID,
			ArtifactID:   artifactID,
			ResourceType: resourceType,
			Action:       action,
			Status:       status,
			PerformedAt:  performedAt,
			ErrorCode:    errorCode,
		},
	}

	ackJSON, err := json.Marshal(ack)
	if err != nil {
		c.logger.Error("Failed to marshal deployment ack",
			slog.String("deployment_id", deploymentID),
			slog.Any("error", err))
		return
	}

	if err := c.sendMessage(ackJSON); err != nil {
		c.logger.Error("Failed to send deployment ack",
			slog.String("deployment_id", deploymentID),
			slog.String("artifact_id", artifactID),
			slog.Any("error", err))
		return
	}

	c.logger.Info("Deployment ack sent",
		slog.String("deployment_id", deploymentID),
		slog.String("artifact_id", artifactID),
		slog.String("resource_type", resourceType),
		slog.String("action", action),
		slog.String("status", status))
}

// waitForConnectionAck waits for the connection.ack message from the server
func (c *Client) waitForConnectionAck(conn *websocket.Conn) error {
	// Set read deadline for ack message
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("failed to set read deadline for connection ack: %w", err)
	}
	defer func() {
		if err := conn.SetReadDeadline(time.Time{}); err != nil {
			c.logger.Warn("Failed to clear read deadline after connection ack", slog.Any("error", err))
		}
	}()

	_, message, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read connection.ack: %w", err)
	}

	var ack ConnectionAckMessage
	if err := json.Unmarshal(message, &ack); err != nil {
		return fmt.Errorf("failed to parse connection.ack: %w", err)
	}

	if ack.Type != "connection.ack" {
		return fmt.Errorf("expected connection.ack message, got: %s", ack.Type)
	}

	// Store gateway and connection IDs
	c.state.mu.Lock()
	c.state.GatewayID = ack.GatewayID
	c.state.ConnectionID = ack.ConnectionID
	c.state.mu.Unlock()

	c.logger.Info("Received connection acknowledgment",
		slog.String("gateway_id", ack.GatewayID),
		slog.String("connection_id", ack.ConnectionID),
		slog.String("timestamp", ack.Timestamp),
	)

	return nil
}

// refreshSubscriptionSnapshot triggers an xDS snapshot rebuild so the policy
// engine picks up the latest subscription state from the DB.
func (c *Client) refreshSubscriptionSnapshot() {
	if c.subscriptionSnapshotUpdater == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.subscriptionSnapshotUpdater.UpdateSnapshot(ctx); err != nil {
		c.logger.Warn("Failed to refresh subscription xDS snapshot after event", slog.Any("error", err))
	}
}

// syncSubscriptionPlans fetches all subscription plans from the control plane
// and persists them locally. This must run before subscription sync since
// subscriptions reference plans via foreign key.
func (c *Client) syncSubscriptionPlans(gatewayID string) {

	// Skip for on-prem control planes
	if c.isOnPrem() {
		c.logger.Debug("Skipping subscription plan bulk sync: on-prem control plane detected",
			slog.String("gateway_id", gatewayID),
		)
		return
	}

	if c.apiUtilsService == nil {
		return
	}

	resourceService := c.getSubscriptionResourceService()

	c.logger.Info("Starting bulk sync of subscription plans")

	plans, err := c.apiUtilsService.FetchSubscriptionPlans()
	if err != nil {
		c.logger.Warn("Failed to bulk-sync subscription plans", slog.Any("error", err))
		return
	}

	saved := 0
	fetchedIDs := make(map[string]struct{}, len(plans))
	for _, plan := range plans {
		select {
		case <-c.ctx.Done():
			c.logger.Info("Subscription plan sync aborted: client shutting down")
			return
		default:
		}

		plan.GatewayID = ""
		correlationID := plan.Etag
		if correlationID == "" {
			correlationID = utils.GenerateDeterministicUUIDv7(plan.ID, plan.UpdatedAt)
		}
		if err := resourceService.UpsertSubscriptionPlan(&plan, "CREATE", correlationID, c.logger); err != nil {
			c.logger.Warn("Failed to upsert subscription plan during bulk sync",
				slog.String("planId", plan.ID), slog.Any("error", err))
		} else {
			saved++
		}
		fetchedIDs[plan.ID] = struct{}{}
	}

	// Reconcile orphaned plans: delete plans that exist locally but were not in the remote set
	localPlans, err := c.db.ListSubscriptionPlans("")
	if err != nil {
		c.logger.Warn("Failed to list local subscription plans for orphan reconciliation", slog.Any("error", err))
	} else {
		for _, lp := range localPlans {
			if _, exists := fetchedIDs[lp.ID]; !exists {
				correlationID := utils.GenerateDeterministicUUIDv7(lp.ID, lp.UpdatedAt)
				if err := resourceService.DeleteSubscriptionPlan(lp.ID, correlationID, c.logger); err != nil {
					c.logger.Warn("Failed to delete orphaned subscription plan during bulk sync",
						slog.String("planId", lp.ID), slog.Any("error", err))
				}
			}
		}
	}

	c.logger.Info("Subscription plan bulk sync complete",
		slog.Int("total", len(plans)), slog.Int("saved", saved))
}

// syncSubscriptionsForExistingAPIs performs a one-time bulk sync of subscriptions
// for all REST APIs persisted in the local DB after the WebSocket connection is
// established. Enumeration uses the DB (not the in-memory config store) so bulk
// sync still runs when EventHub is enabled: deployment sync writes rows to the DB
// before the EventListener updates the store.
// The gatewayID parameter is a stable snapshot of the gateway identifier at the
// time the sync was scheduled, ensuring we don't cross-contaminate state across
// reconnects.
func (c *Client) syncSubscriptionsForExistingAPIs(gatewayID string) {

	// Skip for on-prem control planes
	if c.isOnPrem() {
		c.logger.Debug("Skipping subscription bulk sync: on-prem control plane detected",
			slog.String("gateway_id", gatewayID),
		)
		return
	}

	if c.apiUtilsService == nil || c.db == nil {
		return
	}

	resourceService := c.getSubscriptionResourceService()

	restCfgs, err := c.db.GetAllConfigsByKind(models.KindRestApi)
	if err != nil {
		c.logger.Warn("Failed to list REST APIs from DB for subscription bulk sync",
			slog.String("gateway_id", gatewayID),
			slog.Any("error", err),
		)
		return
	}

	for _, cfg := range restCfgs {
		// Abort promptly if the client is shutting down.
		select {
		case <-c.ctx.Done():
			c.logger.Info("Stopping subscription bulk sync due to client context cancellation")
			return
		case <-c.stopChan:
			c.logger.Info("Stopping subscription bulk sync due to stop signal")
			return
		default:
		}

		if cfg == nil {
			continue
		}

		apiID := cfg.UUID

		subs, err := c.apiUtilsService.FetchSubscriptionsForAPI(apiID)
		if err != nil {
			c.logger.Warn("Failed to bulk-sync subscriptions for API",
				slog.String("api_id", apiID),
				slog.Any("error", err),
			)
			continue
		}

		fetchedSubIDs := make(map[string]struct{}, len(subs))
		for i := range subs {
			select {
			case <-c.ctx.Done():
				c.logger.Info("Stopping subscription bulk sync during subscription processing due to client context cancellation")
				return
			case <-c.stopChan:
				c.logger.Info("Stopping subscription bulk sync during subscription processing due to stop signal")
				return
			default:
			}

			sub := subs[i] // copy
			if sub.ID == "" {
				continue
			}

			sub.APIID = apiID
			sub.GatewayID = ""

			correlationID := sub.Etag
			if correlationID == "" {
				correlationID = utils.GenerateDeterministicUUIDv7(sub.ID, sub.UpdatedAt)
			}
			if err := resourceService.UpsertSubscription(&sub, "CREATE", correlationID, c.logger); err != nil {
				c.logger.Warn("Failed to upsert subscription during bulk sync",
					slog.String("subscription_id", sub.ID),
					slog.String("api_id", apiID),
					slog.Any("error", err),
				)
			}
			fetchedSubIDs[sub.ID] = struct{}{}
		}

		// Reconcile orphaned subscriptions: delete subs that exist locally but were not in the remote set
		localSubs, err := c.db.ListSubscriptionsByAPI(apiID, "", nil, nil)
		if err != nil {
			c.logger.Warn("Failed to list local subscriptions for orphan reconciliation",
				slog.String("api_id", apiID), slog.Any("error", err))
		} else {
			for _, ls := range localSubs {
				if _, exists := fetchedSubIDs[ls.ID]; !exists {
					correlationID := utils.GenerateDeterministicUUIDv7(ls.ID, ls.UpdatedAt)
					if err := resourceService.DeleteSubscription(ls.ID, correlationID, c.logger); err != nil {
						c.logger.Warn("Failed to delete orphaned subscription during bulk sync",
							slog.String("subscription_id", ls.ID),
							slog.String("api_id", apiID), slog.Any("error", err))
					}
				}
			}
		}
	}
}

// syncAPIKeysForExistingArtifacts performs a one-time bulk sync of API keys for all
// currently known RestApi, WebSubApi, LlmProvider, and LlmProxy artifacts after the WebSocket connection
// is established. Upserts fetched keys into the DB, reconciles deletions per artifact,
// then reloads the in-memory store and refreshes the xDS snapshot once.
func (c *Client) syncAPIKeysForExistingArtifacts(gatewayID string) {
	// Skip for on-prem control planes
	if c.isOnPrem() {
		c.logger.Debug("Skipping API Key bulk sync: on-prem control plane detected",
			slog.String("gateway_id", gatewayID),
		)
		return
	}

	if c.apiUtilsService == nil || c.store == nil || c.apiKeyStore == nil {
		return
	}

	c.logger.Info("Starting API key sync at gateway startup")

	issuer := ""
	if c.systemConfig != nil {
		issuer = c.systemConfig.APIKey.Issuer
	}

	// Read from DB rather than the in-memory store. On a fresh gateway the
	// in-memory store may still be empty because the EventHub event from
	// syncDeployments hasn't been polled yet by the EventListener.
	configs, dbErr := c.db.GetAllConfigs()
	if dbErr != nil {
		c.logger.Error("Failed to load configs for API key sync", slog.Any("error", dbErr))
		return
	}
	artifactUUIDsByKind := make(map[string][]string)
	for _, cfg := range configs {
		if cfg == nil {
			continue
		}
		if cfg.Kind != models.KindLlmProvider && cfg.Kind != models.KindLlmProxy &&
			cfg.Kind != models.KindRestApi && cfg.Kind != models.KindWebSubApi {
			continue
		}
		artifactUUIDsByKind[cfg.Kind] = append(artifactUUIDsByKind[cfg.Kind], cfg.UUID)
	}

	for _, kind := range []string{models.KindRestApi, models.KindWebSubApi, models.KindLlmProvider, models.KindLlmProxy} {
		select {
		case <-c.ctx.Done():
			c.logger.Info("Stopping API key bulk sync due to client context cancellation")
			return
		case <-c.stopChan:
			c.logger.Info("Stopping API key bulk sync due to stop signal")
			return
		default:
		}

		keys, err := c.apiUtilsService.FetchAPIKeysByKind(kind, issuer)
		if err != nil {
			c.logger.Warn("Failed to bulk-sync API keys for kind",
				slog.String("kind", kind),
				slog.Any("error", err),
			)
			continue
		}

		fetchedUUIDs := make([]string, 0, len(keys))
		for i := range keys {
			select {
			case <-c.ctx.Done():
				c.logger.Info("Stopping API key bulk sync during key processing due to client context cancellation")
				return
			case <-c.stopChan:
				c.logger.Info("Stopping API key bulk sync during key processing due to stop signal")
				return
			default:
			}

			key := keys[i]
			if key.UUID == "" {
				continue
			}

			if err := c.db.UpsertAPIKey(&key); err != nil {
				c.logger.Warn("Failed to upsert API key during bulk sync",
					slog.String("key_uuid", key.UUID),
					slog.String("artifact_uuid", key.ArtifactUUID),
					slog.Any("error", err))
			} else {
				etag := key.ETag
				if etag == "" {
					etag = utils.APIKeyETag(key.ArtifactUUID, key.Name, key.UpdatedAt)
				}
				c.apiKeyService.PublishAPIKeyEvent("CREATE", key.ArtifactUUID, key.UUID, etag, c.logger)
			}
			fetchedUUIDs = append(fetchedUUIDs, key.UUID)
		}

		artifactUUIDs := artifactUUIDsByKind[kind]
		staleKeys, err := c.db.ListAPIKeysForArtifactsNotIn(artifactUUIDs, fetchedUUIDs)
		if err != nil {
			c.logger.Warn("Failed to list stale API keys before reconciliation",
				slog.String("kind", kind), slog.Any("error", err))
		}
		if len(staleKeys) > 0 {
			staleUUIDs := make([]string, len(staleKeys))
			for i, k := range staleKeys {
				staleUUIDs[i] = k.UUID
			}
			if err := c.db.DeleteAPIKeysByUUIDs(staleUUIDs); err != nil {
				c.logger.Warn("Failed to reconcile deleted API keys during bulk sync",
					slog.String("kind", kind), slog.Any("error", err))
			} else {
				for _, k := range staleKeys {
					c.apiKeyService.PublishAPIKeyEvent("DELETE", k.ArtifactUUID, k.UUID,
						utils.APIKeyETag(k.ArtifactUUID, k.Name, k.UpdatedAt), c.logger)
				}
			}
		}
	}
}

// Close closes the WebSocket connection
func (c *Client) Close() error {
	// Nil out conn under state.mu first so no new sendMessage call can obtain
	// it, then acquire writeMu to drain any in-flight write before sending the
	// close frame.
	c.state.mu.Lock()
	conn := c.state.Conn
	c.state.Conn = nil
	c.setStateNoLock(Disconnected)
	c.state.mu.Unlock()

	if conn != nil {
		closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Client closing connection")
		c.writeMu.Lock()
		writeErr := conn.WriteMessage(websocket.CloseMessage, closeMsg)
		c.writeMu.Unlock()
		closeErr := conn.Close()
		if writeErr != nil && closeErr != nil {
			return fmt.Errorf("failed to write close frame: %w; additionally failed to close connection: %v", writeErr, closeErr)
		}
		if writeErr != nil {
			return fmt.Errorf("failed to write close frame: %w", writeErr)
		}
		if closeErr != nil {
			return closeErr
		}
		return nil
	}

	return nil
}

// heartbeatMonitor checks for heartbeat timeouts
func (c *Client) heartbeatMonitor() {
	defer c.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			lastHeartbeat := atomic.LoadInt64(&c.state.LastHeartbeat)
			timeSinceLastHeartbeat := time.Since(time.Unix(lastHeartbeat, 0))

			// Check if heartbeat timeout exceeded (35s = 30s server timeout + 5s grace)
			if timeSinceLastHeartbeat > 35*time.Second {
				c.logger.Warn("Heartbeat timeout detected",
					slog.Duration("time_since_last_heartbeat", timeSinceLastHeartbeat),
				)

				// Trigger reconnection
				c.state.mu.Lock()
				if c.state.Conn != nil {
					if err := c.state.Conn.Close(); err != nil {
						c.logger.Warn("Failed to close connection after heartbeat timeout", slog.Any("error", err))
					}
					c.state.Conn = nil
				}
				c.state.mu.Unlock()

				c.setState(Reconnecting)
				return
			}

		case <-c.stopChan:
			return
		case <-c.ctx.Done():
			return
		}
	}
}

// connectionLoop manages the connection lifecycle with reconnection
func (c *Client) connectionLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.stopChan:
			return
		case <-c.ctx.Done():
			return
		default:
		}

		// Attempt connection
		err := c.Connect()
		if err != nil {
			c.logger.Warn("Connection failed, will retry",
				slog.Any("error", err),
				slog.Duration("retry_delay", c.state.NextRetryDelay),
				slog.Int("retry_count", c.state.RetryCount),
			)

			c.setState(Reconnecting)
			c.state.RetryCount++

			// Calculate next retry delay with exponential backoff
			c.calculateNextRetryDelay()

			// Wait before retry
			select {
			case <-time.After(c.state.NextRetryDelay):
				continue
			case <-c.stopChan:
				return
			case <-c.ctx.Done():
				return
			}
		}

		// Connection successful, wait for disconnection
		c.waitForDisconnection()

		// Check if we should reconnect
		if c.isShuttingDown() {
			return
		}

		c.setState(Reconnecting)
	}
}

// waitForDisconnection waits for the connection to be closed and processes incoming messages
func (c *Client) waitForDisconnection() {
	c.state.mu.RLock()
	conn := c.state.Conn
	c.state.mu.RUnlock()

	if conn == nil {
		return
	}

	// Read loop to detect disconnection and process messages
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			c.logger.Warn("Connection lost",
				slog.Any("error", err),
			)

			c.state.mu.Lock()
			if c.state.Conn != nil {
				if closeErr := c.state.Conn.Close(); closeErr != nil {
					c.logger.Warn("Failed to close lost connection", slog.Any("error", closeErr))
				}
				c.state.Conn = nil
			}
			c.state.mu.Unlock()

			break
		}

		// Process received message
		c.handleMessage(messageType, message)
	}
}

// redactSensitiveEventPayload returns a redaction message for event types that contain sensitive data.
// Used for logging so raw subscription tokens and API keys are never written to logs.
func redactSensitiveEventPayload(eventType string) string {
	switch eventType {
	case "subscription.created", "subscription.updated", "subscription.deleted":
		return "[REDACTED - contains sensitive subscription token]"
	case "apikey.created", "apikey.updated", "apikey.revoked":
		return "[REDACTED - contains sensitive API key data]"
	default:
		return "[REDACTED]"
	}
}

// handleMessage processes incoming WebSocket messages
func (c *Client) handleMessage(messageType int, message []byte) {
	// Log the message type
	c.logger.Debug("Received WebSocket message",
		slog.Int("message_type", messageType),
		slog.Int("message_length", len(message)),
	)

	// Only process text messages (JSON events)
	if messageType != websocket.TextMessage {
		c.logger.Debug("Ignoring non-text message",
			slog.Int("message_type", messageType),
		)
		return
	}

	// Parse as generic event to extract type
	var event map[string]interface{}
	if err := json.Unmarshal(message, &event); err != nil {
		c.logger.Error("Failed to parse WebSocket message",
			slog.Any("error", err),
			slog.String("message", string(message)),
		)
		return
	}

	// Extract event type
	eventType, ok := event["type"].(string)
	if !ok {
		c.logger.Warn("Message missing 'type' field",
			slog.String("message", string(message)),
		)
		return
	}

	// Log the event to console (skip payload for events with sensitive data)
	isSensitiveEvent := eventType == "apikey.created" || eventType == "apikey.updated" || eventType == "apikey.revoked" ||
		eventType == "subscription.created" || eventType == "subscription.updated" || eventType == "subscription.deleted"
	if isSensitiveEvent {
		redactMsg := redactSensitiveEventPayload(eventType)
		c.logger.Info("Received WebSocket event",
			slog.String("type", eventType),
			slog.String("payload", redactMsg),
		)
	} else {
		c.logger.Info("Received WebSocket event",
			slog.String("type", eventType),
			slog.String("payload", string(message)),
		)
	}

	// Handle specific event types
	switch eventType {
	case "connection.ack":
		// Already handled during connection establishment
		c.logger.Debug("Received connection.ack (already processed)")
	case "api.deployed":
		c.handleAPIDeployedEvent(event)
	case "api.undeployed":
		c.handleAPIUndeployedEvent(event)
	case "api.deleted":
		c.handleAPIDeletedEvent(event)
	case "llmprovider.deployed":
		c.handleLLMProviderDeployedEvent(event)
	case "llmprovider.undeployed":
		c.handleLLMProviderUndeployedEvent(event)
	case "llmproxy.deployed":
		c.handleLLMProxyDeployedEvent(event)
	case "llmproxy.undeployed":
		c.handleLLMProxyUndeployedEvent(event)
	case "llmprovider.deleted":
		c.handleLLMProviderDeletedEvent(event)
	case "llmproxy.deleted":
		c.handleLLMProxyDeletedEvent(event)
	case "apikey.created":
		c.handleAPIKeyCreatedEvent(event)
	case "apikey.updated":
		c.handleAPIKeyUpdatedEvent(event)
	case "apikey.revoked":
		c.handleAPIKeyRevokedEvent(event)
	case "subscription.created":
		c.handleSubscriptionCreatedEvent(event)
	case "subscription.updated":
		c.handleSubscriptionUpdatedEvent(event)
	case "subscription.deleted":
		c.handleSubscriptionDeletedEvent(event)
	case "subscriptionPlan.created":
		c.handleSubscriptionPlanCreatedEvent(event)
	case "subscriptionPlan.updated":
		c.handleSubscriptionPlanUpdatedEvent(event)
	case "subscriptionPlan.deleted":
		c.handleSubscriptionPlanDeletedEvent(event)
	case "mcpproxy.deployed":
		c.handleMCPProxyDeploymentEvent(event)
	case "mcpproxy.undeployed":
		c.handleMCPProxyUndeploymentEvent(event)
	case "mcpproxy.deleted":
		c.handleMCPProxyDeletedEvent(event)
	case "websub.deployed":
		c.handleWebSubAPIDeployedEvent(event)
	case "websub.undeployed":
		c.handleWebSubAPIUndeployedEvent(event)
	case "websub.deleted":
		c.handleWebSubAPIDeletedEvent(event)
	case "application.updated":
		c.handleApplicationUpdatedEvent(event)
	default:
		c.logger.Info("Received unknown event type (will be processed when handlers are implemented)",
			slog.String("type", eventType),
		)
	}
}

// fetchAndDeployAPI fetches API definition and deploys it
func (c *Client) fetchAndDeployAPI(apiID, deploymentID string, deployedAt *time.Time, correlationID string) (*utils.APIDeploymentResult, error) {
	// Fetch API definition from control plane
	zipData, err := c.apiUtilsService.FetchAPIDefinition(apiID)
	if err != nil {
		c.logger.Error("Failed to fetch API definition",
			slog.String("api_id", apiID),
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("failed to fetch API definition: %w", err)
	}

	// Extract YAML directly from zip file in memory (no need to save to disk)
	yamlData, err := c.apiUtilsService.ExtractYAMLFromZip(zipData)
	if err != nil {
		c.logger.Error("Failed to extract YAML from zip",
			slog.String("api_id", apiID),
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("failed to extract YAML from zip: %w", err)
	}

	// log the yaml for debugging (only compute hash if debug logging is enabled)
	if c.logger.Enabled(context.Background(), slog.LevelDebug) {
		yamlHash := sha256.Sum256(yamlData)
		c.logger.Debug("Extracted YAML data",
			slog.String("api_id", apiID),
			slog.Int("yaml_bytes", len(yamlData)),
			slog.String("yaml_hash", fmt.Sprintf("%x", yamlHash)),
		)
	}

	// Create API configuration from YAML using the deployment service
	result, err := c.apiUtilsService.CreateAPIFromYAML(yamlData, apiID, deploymentID, deployedAt, correlationID, c.deploymentService)
	if err != nil {
		c.logger.Error("Failed to create API from YAML",
			slog.String("api_id", apiID),
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("failed to create API from YAML: %w", err)
	}

	return result, nil
}

// updatePolicyForDeployment updates policy engine for API deployment
func (c *Client) updatePolicyForDeployment(apiID, correlationID string, result *utils.APIDeploymentResult) error {
	if c.policyManager == nil {
		c.logger.Error("Failed to update policy engine snapshot: policy manager is not available",
			slog.String("api_id", apiID),
			slog.String("correlation_id", correlationID),
		)
		return fmt.Errorf("policy manager is not available")
	}

	if result == nil {
		return nil
	}

	if err := c.policyManager.UpsertAPIConfig(result.StoredConfig); err != nil {
		c.logger.Error("Failed to upsert runtime config for deployment",
			slog.Any("error", err),
			slog.String("api_id", apiID),
			slog.String("correlation_id", correlationID))
		return fmt.Errorf("failed to upsert runtime config: %w", err)
	}
	c.logger.Info("Successfully updated policy engine snapshot",
		slog.String("api_id", apiID),
		slog.String("correlation_id", correlationID))

	return nil
}

// handleAPIDeployedEvent handles API deployment events
func (c *Client) handleAPIDeployedEvent(event map[string]interface{}) {
	c.logger.Info("API Deployment Event",
		slog.Any("payload", event["payload"]),
		slog.Any("timestamp", event["timestamp"]),
		slog.Any("correlationId", event["correlationId"]),
	)

	// Parse the event into structured format
	eventBytes, err := json.Marshal(event)
	if err != nil {
		c.logger.Error("Failed to marshal event for parsing",
			slog.Any("error", err),
		)
		return
	}

	var deployedEvent APIDeployedEvent
	if err := json.Unmarshal(eventBytes, &deployedEvent); err != nil {
		c.logger.Error("Failed to parse API deployment event",
			slog.Any("error", err),
		)
		return
	}

	// Extract API ID
	apiID := deployedEvent.Payload.APIID
	if apiID == "" {
		c.logger.Error("API ID is empty in deployment event")
		return
	}

	c.logger.Info("Processing API deployment",
		slog.String("api_id", apiID),
		slog.String("deployment_id", deployedEvent.Payload.DeploymentID),
		slog.String("correlation_id", deployedEvent.CorrelationID),
	)

	// Fetch API definition and deploy
	// (deploymentService handles DB + event publishing when eventHub is set)
	performedAt := deployedEvent.Payload.PerformedAt.Truncate(time.Millisecond)
	if performedAt.IsZero() {
		performedAt = time.Now().Truncate(time.Millisecond)
	}
	result, err := c.fetchAndDeployAPI(apiID, deployedEvent.Payload.DeploymentID, &performedAt, deployedEvent.CorrelationID)
	if err != nil {
		c.sendDeploymentAck(deployedEvent.Payload.DeploymentID, apiID, "api", "deploy", "failed",
			deployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	if result.IsStale {
		// Stale event — DB was not modified. Do not send ack; in HA mode the
		// controller that actually processed the event will ack. If all controllers
		// see stale, platform-API will timeout and handle accordingly.
		c.logger.Debug("Skipped stale API deploy event (newer version exists in DB)",
			slog.String("api_id", apiID),
			slog.String("deployment_id", deployedEvent.Payload.DeploymentID),
		)
		return
	}

	c.sendDeploymentAck(deployedEvent.Payload.DeploymentID, apiID, "api", "deploy", "success",
		deployedEvent.Payload.PerformedAt, "")

	c.logger.Info("Successfully processed API deployment event",
		slog.String("api_id", apiID),
		slog.String("correlation_id", deployedEvent.CorrelationID),
	)
}

// handleAPIUndeployedEvent handles API undeployment events
func (c *Client) handleAPIUndeployedEvent(event map[string]interface{}) {
	c.logger.Info("API Undeployment Event",
		slog.Any("payload", event["payload"]),
		slog.Any("timestamp", event["timestamp"]),
		slog.Any("correlationId", event["correlationId"]),
	)

	// Parse the event into structured format
	eventBytes, err := json.Marshal(event)
	if err != nil {
		c.logger.Error("Failed to marshal event for parsing",
			slog.Any("error", err),
		)
		return
	}

	var undeployedEvent APIUndeployedEvent
	if err := json.Unmarshal(eventBytes, &undeployedEvent); err != nil {
		c.logger.Error("Failed to parse API undeployment event",
			slog.Any("error", err),
		)
		return
	}

	// Extract API ID
	apiID := undeployedEvent.Payload.APIID
	if apiID == "" {
		c.logger.Error("API ID is empty in undeployment event")
		return
	}

	c.logger.Info("Processing API undeployment",
		slog.String("api_id", apiID),
		slog.String("correlation_id", undeployedEvent.CorrelationID),
	)

	// Check if API exists on this gateway
	apiConfig, err := c.findAPIConfig(apiID)
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.logger.Warn("API configuration not found for undeployment",
				slog.String("api_id", apiID),
			)
			// Still send success ack - the API is already undeployed
			c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, apiID, "api", "undeploy", "success",
				undeployedEvent.Payload.PerformedAt, "")
			return
		}
		// Real storage error - log and abort
		c.logger.Error("Failed to fetch API configuration for undeployment",
			slog.String("api_id", apiID),
			slog.String("correlation_id", undeployedEvent.CorrelationID),
			slog.Any("error", err),
		)
		c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, apiID, "api", "undeploy", "failed",
			undeployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	// Only process undeploy if the event's DeploymentID matches the current one.
	// This prevents stale undeploy events from affecting a newer deployment.
	if apiConfig.DeploymentID != "" && undeployedEvent.Payload.DeploymentID != "" &&
		apiConfig.DeploymentID != undeployedEvent.Payload.DeploymentID {
		c.logger.Warn("Ignoring stale API undeploy event: deployment ID mismatch",
			slog.String("api_id", apiID),
			slog.String("event_deployment_id", undeployedEvent.Payload.DeploymentID),
			slog.String("current_deployment_id", apiConfig.DeploymentID),
		)
		c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, apiID, "api", "undeploy", "failed",
			undeployedEvent.Payload.PerformedAt, "DEPLOYMENT_ID_MISMATCH")
		return
	}

	// Set status to undeployed (preserve config, keys, and policies)
	// Use CP event timestamp for consistent sync ordering; fall back to local time if not provided
	apiUndeployPerformedAt := undeployedEvent.Payload.PerformedAt.Truncate(time.Millisecond)
	if apiUndeployPerformedAt.IsZero() {
		apiUndeployPerformedAt = time.Now().Truncate(time.Millisecond)
	}
	apiConfig.DesiredState = models.StateUndeployed
	apiConfig.DeploymentID = undeployedEvent.Payload.DeploymentID
	apiConfig.DeployedAt = &apiUndeployPerformedAt
	apiConfig.UpdatedAt = time.Now()

	// Timestamp-guarded upsert: only writes if deployed_at is newer than what's in DB.
	// This prevents stale undeploy events from overwriting newer state.
	affected, err := c.db.UpsertConfig(apiConfig)
	if err != nil {
		c.logger.Error("Failed to upsert config for undeployment",
			slog.String("api_id", apiID),
			slog.Any("error", err),
		)
		c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, apiID, "api", "undeploy", "failed",
			undeployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}
	if !affected {
		// Stale event — DB was not modified. Do not send ack; in HA mode the
		// controller that actually processed the event will ack. If all controllers
		// see stale, platform-API will timeout and handle accordingly.
		c.logger.Debug("Skipped stale API undeploy event (newer version exists in DB)",
			slog.String("api_id", apiID),
			slog.String("deployment_id", undeployedEvent.Payload.DeploymentID),
		)
		return
	}

	evt := eventhub.Event{
		EventType: eventhub.EventTypeAPI,
		Action:    "UPDATE",
		EntityID:  apiID,
		EventID:   undeployedEvent.CorrelationID,
	}
	if err := c.eventHub.PublishEvent(c.gatewayID, evt); err != nil {
		c.logger.Error("Failed to publish API undeployment event", slog.Any("error", err))
	}

	c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, apiID, "api", "undeploy", "success",
		undeployedEvent.Payload.PerformedAt, "")

	c.logger.Info("Successfully processed API undeployment event",
		slog.String("api_id", apiID),
		slog.String("correlation_id", undeployedEvent.CorrelationID),
	)
}

// findAPIConfig checks if an API exists in the database.
func (c *Client) findAPIConfig(apiID string) (*models.StoredConfig, error) {
	config, err := c.db.GetConfig(apiID)
	if err == nil {
		return config, nil
	}
	if storage.IsNotFoundError(err) {
		return nil, storage.ErrNotFound
	}
	return nil, fmt.Errorf("database error while fetching config: %w", err)
}

// removePolicyConfiguration removes runtime config from the policy engine.
// cfg must be non-nil; if it is nil (e.g. orphaned resource), the call is a no-op.
func (c *Client) removePolicyConfiguration(cfg *models.StoredConfig, correlationID string, isOrphaned bool) {
	if c.policyManager == nil || cfg == nil {
		return
	}

	if err := c.policyManager.DeleteAPIConfig(cfg.Kind, cfg.Handle); err != nil {
		c.logger.Warn("Failed to remove runtime policy configuration",
			slog.Any("error", err),
			slog.String("api_id", cfg.UUID),
			slog.String("correlation_id", correlationID),
		)
		return
	}

	if isOrphaned {
		c.logger.Debug("Checked and cleaned up orphaned policy configuration",
			slog.String("api_id", cfg.UUID),
			slog.String("correlation_id", correlationID),
		)
	} else {
		c.logger.Info("Successfully removed runtime policy configuration",
			slog.String("api_id", cfg.UUID),
			slog.String("correlation_id", correlationID),
		)
	}
}

// updateXDSSnapshotAsync updates xDS snapshot in the background
// isOrphaned: true for orphaned resource cleanup (logs at WARN level)
// isUndeployment: true for undeployment, false for deletion (only relevant when !isOrphaned)
func (c *Client) updateXDSSnapshotAsync(apiID, correlationID string, isOrphaned, isUndeployment bool) {
	if c.snapshotManager == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := c.snapshotManager.UpdateSnapshot(ctx, correlationID); err != nil {
			// Log level depends on operation context
			if isOrphaned {
				c.logger.Warn("Failed to update xDS snapshot for orphaned resource cleanup",
					slog.String("api_id", apiID),
					slog.Any("error", err),
				)
			} else {
				operation := "deletion"
				if isUndeployment {
					operation = "undeployment"
				}
				c.logger.Error("Failed to update xDS snapshot after API "+operation,
					slog.String("api_id", apiID),
					slog.String("operation", operation),
					slog.Any("error", err),
				)
			}
		} else if !isOrphaned {
			operation := "deletion"
			if isUndeployment {
				operation = "undeployment"
			}
			c.logger.Info("Successfully updated xDS snapshot after API "+operation,
				slog.String("api_id", apiID),
				slog.String("operation", operation),
			)
		}
	}()
}

// cleanupOrphanedResources attempts to clean up stale resources when API config doesn't exist
func (c *Client) cleanupOrphanedResources(apiID, correlationID string) {
	c.logger.Info("API configuration not found on this gateway, checking for stale resources",
		slog.String("api_id", apiID),
		slog.String("correlation_id", correlationID),
	)

	// Check and clean up stale API keys from database
	if err := c.db.RemoveAPIKeysAPI(apiID); err != nil {
		c.logger.Warn("Failed to remove stale API keys from database",
			slog.String("api_id", apiID),
			slog.Any("error", err),
		)
	} else {
		c.logger.Debug("Cleaned up any stale API keys from database",
			slog.String("api_id", apiID),
		)
	}

	if err := c.db.DeleteSubscriptionsForAPINotIn(apiID, nil); err != nil {
		c.logger.Warn("Failed to remove stale subscriptions from database",
			slog.String("api_id", apiID),
			slog.Any("error", err),
		)
	} else {
		c.logger.Debug("Cleaned up any stale subscriptions from database",
			slog.String("api_id", apiID),
		)
	}

	evt := eventhub.Event{
		EventType: eventhub.EventTypeAPI,
		Action:    "DELETE",
		EntityID:  apiID,
		EventID:   correlationID,
	}
	if err := c.eventHub.PublishEvent(c.gatewayID, evt); err != nil {
		c.logger.Error("Failed to publish orphan cleanup event", slog.Any("error", err))
	}

	c.logger.Info("Successfully processed stale resource cleanup",
		slog.String("api_id", apiID),
		slog.String("correlation_id", correlationID),
	)
}

// performFullAPIDeletion performs complete deletion of API and all related resources
func (c *Client) performFullAPIDeletion(apiID string, apiConfig *models.StoredConfig, correlationID string) {
	c.logger.Info("API configuration found, performing full deletion",
		slog.String("api_id", apiID),
	)

	// 1. Delete API configuration from database first (for atomicity)
	if err := c.db.DeleteConfig(apiID); err != nil {
		c.logger.Error("Failed to delete API configuration from database",
			slog.String("api_id", apiID),
			slog.Any("error", err),
		)
		// Continue with cleanup even if database deletion fails
	} else {
		c.logger.Info("Successfully deleted API configuration from database",
			slog.String("api_id", apiID),
		)
	}

	// 2. Delete all API keys from database
	if err := c.db.RemoveAPIKeysAPI(apiID); err != nil {
		c.logger.Warn("Failed to delete API keys from database",
			slog.String("api_id", apiID),
			slog.Any("error", err),
		)
	} else {
		c.logger.Info("Successfully deleted API keys from database",
			slog.String("api_id", apiID),
		)
	}

	// 3. Delete all subscriptions from database (no FK cascade)
	if err := c.db.DeleteSubscriptionsForAPINotIn(apiID, nil); err != nil {
		c.logger.Warn("Failed to delete subscriptions from database",
			slog.String("api_id", apiID),
			slog.Any("error", err),
		)
	} else {
		c.logger.Info("Successfully deleted subscriptions from database",
			slog.String("api_id", apiID),
		)
	}

	// Refresh subscription xDS so policy engine drops tokens immediately.
	c.refreshSubscriptionSnapshot()

	evt := eventhub.Event{
		EventType: eventhub.EventTypeAPI,
		Action:    "DELETE",
		EntityID:  apiID,
		EventID:   correlationID,
	}
	if err := c.eventHub.PublishEvent(c.gatewayID, evt); err != nil {
		c.logger.Error("Failed to publish API deletion event", slog.Any("error", err))
	}

	c.logger.Info("Successfully processed API deletion event",
		slog.String("api_id", apiID),
		slog.String("correlation_id", correlationID),
	)
}

// handleAPIDeletedEvent handles API deletion events
func (c *Client) handleAPIDeletedEvent(event map[string]interface{}) {
	c.logger.Info("API Deletion Event",
		slog.Any("payload", event["payload"]),
		slog.Any("timestamp", event["timestamp"]),
		slog.Any("correlationId", event["correlationId"]),
	)

	// Parse the event into structured format
	eventBytes, err := json.Marshal(event)
	if err != nil {
		c.logger.Error("Failed to marshal event for parsing",
			slog.Any("error", err),
		)
		return
	}

	var deletedEvent APIDeletedEvent
	if err := json.Unmarshal(eventBytes, &deletedEvent); err != nil {
		c.logger.Error("Failed to parse API deletion event",
			slog.Any("error", err),
		)
		return
	}

	// Extract API ID
	apiID := deletedEvent.Payload.APIID
	if apiID == "" {
		c.logger.Error("API ID is empty in deletion event")
		return
	}

	c.logger.Info("Processing API deletion",
		slog.String("api_id", apiID),
		slog.String("correlation_id", deletedEvent.CorrelationID),
	)

	// Check if API exists on this gateway
	apiConfig, err := c.findAPIConfig(apiID)
	if err != nil {
		if storage.IsNotFoundError(err) {
			// Config not found - proceed with orphan cleanup
			c.cleanupOrphanedResources(apiID, deletedEvent.CorrelationID)
			return
		}
		// Real storage error (DB failure, etc.) - log and abort
		// Do NOT proceed with orphan cleanup as the config might actually exist
		c.logger.Error("Failed to fetch API configuration for deletion, aborting",
			slog.String("api_id", apiID),
			slog.String("correlation_id", deletedEvent.CorrelationID),
			slog.Any("error", err),
		)
		return
	}

	// Config found - perform full deletion
	c.performFullAPIDeletion(apiID, apiConfig, deletedEvent.CorrelationID)
}

// handleLLMProxyDeployedEvent handles LLM proxy deployment events
func (c *Client) handleLLMProxyDeployedEvent(event map[string]interface{}) {
	c.logger.Info("LLM Proxy Deployment Event",
		slog.Any("payload", event["payload"]),
		slog.Any("timestamp", event["timestamp"]),
		slog.Any("correlationId", event["correlationId"]),
	)

	// Parse the event into structured format
	eventBytes, err := json.Marshal(event)
	if err != nil {
		c.logger.Error("Failed to marshal LLM proxy deployment event for parsing",
			slog.Any("error", err),
		)
		return
	}

	var deployedEvent LLMProxyDeployedEvent
	if err := json.Unmarshal(eventBytes, &deployedEvent); err != nil {
		c.logger.Error("Failed to parse LLM proxy deployment event",
			slog.Any("error", err),
		)
		return
	}

	proxyID := deployedEvent.Payload.ProxyID
	if proxyID == "" {
		c.logger.Error("Proxy ID is empty in LLM proxy deployment event")
		return
	}

	c.logger.Info("Processing LLM proxy deployment",
		slog.String("proxy_id", proxyID),
		slog.String("deployment_id", deployedEvent.Payload.DeploymentID),
		slog.String("correlation_id", deployedEvent.CorrelationID),
	)

	// Fetch LLM proxy definition from control plane
	zipData, err := c.apiUtilsService.FetchLLMProxyDefinition(proxyID)
	if err != nil {
		c.logger.Error("Failed to fetch LLM proxy definition",
			slog.String("proxy_id", proxyID),
			slog.Any("error", err),
		)
		c.sendDeploymentAck(deployedEvent.Payload.DeploymentID, proxyID, "llmproxy", "deploy", "failed",
			deployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	// Extract YAML from ZIP
	yamlData, err := c.apiUtilsService.ExtractYAMLFromZip(zipData)
	if err != nil {
		c.logger.Error("Failed to extract YAML from LLM proxy ZIP",
			slog.String("proxy_id", proxyID),
			slog.Any("error", err),
		)
		c.sendDeploymentAck(deployedEvent.Payload.DeploymentID, proxyID, "llmproxy", "deploy", "failed",
			deployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	if c.llmDeploymentService == nil {
		c.logger.Error("LLM deployment service not available",
			slog.String("proxy_id", proxyID),
			slog.String("correlation_id", deployedEvent.CorrelationID),
		)
		c.sendDeploymentAck(deployedEvent.Payload.DeploymentID, proxyID, "llmproxy", "deploy", "failed",
			deployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	// Create LLM proxy configuration from YAML using the deployment service
	llmProxyPerformedAt := deployedEvent.Payload.PerformedAt.Truncate(time.Millisecond)
	if llmProxyPerformedAt.IsZero() {
		llmProxyPerformedAt = time.Now().Truncate(time.Millisecond)
	}
	result, err := c.apiUtilsService.CreateLLMProxyFromYAML(yamlData, proxyID, deployedEvent.Payload.DeploymentID, &llmProxyPerformedAt, deployedEvent.CorrelationID, c.llmDeploymentService)
	if err != nil {
		c.logger.Error("Failed to create LLM proxy from YAML",
			slog.String("proxy_id", proxyID),
			slog.Any("error", err),
		)
		c.sendDeploymentAck(deployedEvent.Payload.DeploymentID, proxyID, "llmproxy", "deploy", "failed",
			deployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	if result.IsStale {
		c.logger.Debug("Skipped stale LLM proxy deploy event (newer version exists in DB)",
			slog.String("proxy_id", proxyID),
			slog.String("deployment_id", deployedEvent.Payload.DeploymentID),
		)
		return
	}

	c.sendDeploymentAck(deployedEvent.Payload.DeploymentID, proxyID, "llmproxy", "deploy", "success",
		deployedEvent.Payload.PerformedAt, "")

	c.logger.Info("Successfully processed LLM proxy deployment event",
		slog.String("proxy_id", proxyID),
		slog.String("correlation_id", deployedEvent.CorrelationID),
	)
}

// handleLLMProviderDeployedEvent handles LLM provider deployment events
func (c *Client) handleLLMProviderDeployedEvent(event map[string]interface{}) {
	c.logger.Info("LLM Provider Deployment Event",
		slog.Any("payload", event["payload"]),
		slog.Any("timestamp", event["timestamp"]),
		slog.Any("correlationId", event["correlationId"]),
	)

	// Parse the event into structured format
	eventBytes, err := json.Marshal(event)
	if err != nil {
		c.logger.Error("Failed to marshal LLM provider deployment event for parsing",
			slog.Any("error", err),
		)
		return
	}

	var deployedEvent LLMProviderDeployedEvent
	if err := json.Unmarshal(eventBytes, &deployedEvent); err != nil {
		c.logger.Error("Failed to parse LLM provider deployment event",
			slog.Any("error", err),
		)
		return
	}

	providerID := deployedEvent.Payload.ProviderID
	if providerID == "" {
		c.logger.Error("Provider ID is empty in LLM provider deployment event")
		return
	}

	c.logger.Info("Processing LLM provider deployment",
		slog.String("provider_id", providerID),
		slog.String("deployment_id", deployedEvent.Payload.DeploymentID),
		slog.String("correlation_id", deployedEvent.CorrelationID),
	)

	// Fetch LLM provider definition from control plane
	zipData, err := c.apiUtilsService.FetchLLMProviderDefinition(providerID)
	if err != nil {
		c.logger.Error("Failed to fetch LLM provider definition",
			slog.String("provider_id", providerID),
			slog.Any("error", err),
		)
		c.sendDeploymentAck(deployedEvent.Payload.DeploymentID, providerID, "llmprovider", "deploy", "failed",
			deployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	// Extract YAML from ZIP
	yamlData, err := c.apiUtilsService.ExtractYAMLFromZip(zipData)
	if err != nil {
		c.logger.Error("Failed to extract YAML from LLM provider ZIP",
			slog.String("provider_id", providerID),
			slog.Any("error", err),
		)
		c.sendDeploymentAck(deployedEvent.Payload.DeploymentID, providerID, "llmprovider", "deploy", "failed",
			deployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	if c.llmDeploymentService == nil {
		c.logger.Error("LLM deployment service not available",
			slog.String("provider_id", providerID),
			slog.String("correlation_id", deployedEvent.CorrelationID),
		)
		c.sendDeploymentAck(deployedEvent.Payload.DeploymentID, providerID, "llmprovider", "deploy", "failed",
			deployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	// Create LLM provider configuration from YAML using the deployment service
	llmProviderPerformedAt := deployedEvent.Payload.PerformedAt.Truncate(time.Millisecond)
	if llmProviderPerformedAt.IsZero() {
		llmProviderPerformedAt = time.Now().Truncate(time.Millisecond)
	}
	result, err := c.apiUtilsService.CreateLLMProviderFromYAML(yamlData, providerID, deployedEvent.Payload.DeploymentID, &llmProviderPerformedAt, deployedEvent.CorrelationID, c.llmDeploymentService)
	if err != nil {
		c.logger.Error("Failed to create LLM provider from YAML",
			slog.String("provider_id", providerID),
			slog.Any("error", err),
		)
		c.sendDeploymentAck(deployedEvent.Payload.DeploymentID, providerID, "llmprovider", "deploy", "failed",
			deployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	if result.IsStale {
		c.logger.Debug("Skipped stale LLM provider deploy event (newer version exists in DB)",
			slog.String("provider_id", providerID),
			slog.String("deployment_id", deployedEvent.Payload.DeploymentID),
		)
		return
	}

	c.sendDeploymentAck(deployedEvent.Payload.DeploymentID, providerID, "llmprovider", "deploy", "success",
		deployedEvent.Payload.PerformedAt, "")

	c.logger.Info("Successfully processed LLM provider deployment event",
		slog.String("provider_id", providerID),
		slog.String("correlation_id", deployedEvent.CorrelationID),
	)
}

// handleLLMProviderUndeployedEvent handles LLM provider undeployment events.
// This performs a soft undeploy (set desired_state = undeployed) rather than a hard delete,
// preserving the config, keys, and policies for potential redeployment.
func (c *Client) handleLLMProviderUndeployedEvent(event map[string]interface{}) {
	c.logger.Info("LLM Provider Undeployment Event",
		slog.Any("payload", event["payload"]),
		slog.Any("timestamp", event["timestamp"]),
		slog.Any("correlationId", event["correlationId"]),
	)

	// Parse the event into structured format
	eventBytes, err := json.Marshal(event)
	if err != nil {
		c.logger.Error("Failed to marshal LLM provider undeployment event for parsing",
			slog.Any("error", err),
		)
		return
	}

	var undeployedEvent LLMProviderUndeployedEvent
	if err := json.Unmarshal(eventBytes, &undeployedEvent); err != nil {
		c.logger.Error("Failed to parse LLM provider undeployment event",
			slog.Any("error", err),
		)
		return
	}

	providerID := undeployedEvent.Payload.ProviderID
	if providerID == "" {
		c.logger.Error("Provider ID is empty in LLM provider undeployment event")
		return
	}

	c.logger.Info("Processing LLM provider undeployment",
		slog.String("provider_id", providerID),
		slog.String("deployment_id", undeployedEvent.Payload.DeploymentID),
		slog.String("correlation_id", undeployedEvent.CorrelationID),
	)

	// Look up existing config
	providerConfig, err := c.findAPIConfig(providerID)
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.logger.Warn("LLM provider configuration not found for undeployment",
				slog.String("provider_id", providerID),
			)
			// Still send success ack - the provider is already gone
			c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, providerID, "llmprovider", "undeploy", "success",
				undeployedEvent.Payload.PerformedAt, "")
			return
		}
		c.logger.Error("Failed to fetch LLM provider configuration for undeployment",
			slog.String("provider_id", providerID),
			slog.Any("error", err),
		)
		c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, providerID, "llmprovider", "undeploy", "failed",
			undeployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	// Only process undeploy if the event's DeploymentID matches the current one.
	if providerConfig.DeploymentID != "" && undeployedEvent.Payload.DeploymentID != "" &&
		providerConfig.DeploymentID != undeployedEvent.Payload.DeploymentID {
		c.logger.Warn("Ignoring stale LLM provider undeploy event: deployment ID mismatch",
			slog.String("provider_id", providerID),
			slog.String("event_deployment_id", undeployedEvent.Payload.DeploymentID),
			slog.String("current_deployment_id", providerConfig.DeploymentID),
		)
		c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, providerID, "llmprovider", "undeploy", "failed",
			undeployedEvent.Payload.PerformedAt, "DEPLOYMENT_ID_MISMATCH")
		return
	}

	// Soft undeploy: set desired_state to undeployed (preserve config, keys, and policies)
	performedAt := undeployedEvent.Payload.PerformedAt.Truncate(time.Millisecond)
	if performedAt.IsZero() {
		performedAt = time.Now().Truncate(time.Millisecond)
	}
	providerConfig.DesiredState = models.StateUndeployed
	providerConfig.DeploymentID = undeployedEvent.Payload.DeploymentID
	providerConfig.DeployedAt = &performedAt
	providerConfig.UpdatedAt = time.Now()

	// Timestamp-guarded upsert: only writes if deployed_at is newer than what's in DB.
	affected, err := c.db.UpsertConfig(providerConfig)
	if err != nil {
		c.logger.Error("Failed to upsert config for LLM provider undeployment",
			slog.String("provider_id", providerID),
			slog.Any("error", err),
		)
		c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, providerID, "llmprovider", "undeploy", "failed",
			undeployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}
	if !affected {
		c.logger.Debug("Skipped stale LLM provider undeploy event (newer version exists in DB)",
			slog.String("provider_id", providerID),
			slog.String("deployment_id", undeployedEvent.Payload.DeploymentID),
		)
		return
	}

	evt := eventhub.Event{
		EventType: eventhub.EventTypeLLMProvider,
		Action:    "UPDATE",
		EntityID:  providerID,
		EventID:   undeployedEvent.CorrelationID,
	}
	if err := c.eventHub.PublishEvent(c.gatewayID, evt); err != nil {
		c.logger.Error("Failed to publish LLM provider undeployment event", slog.Any("error", err))
	}

	c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, providerID, "llmprovider", "undeploy", "success",
		undeployedEvent.Payload.PerformedAt, "")

	c.logger.Info("Successfully processed LLM provider undeployment event",
		slog.String("provider_id", providerID),
		slog.String("correlation_id", undeployedEvent.CorrelationID),
	)
}

// handleLLMProxyUndeployedEvent handles LLM proxy undeployment events.
// This performs a soft undeploy (set desired_state = undeployed) rather than a hard delete,
// preserving the config, keys, and policies for potential redeployment.
func (c *Client) handleLLMProxyUndeployedEvent(event map[string]interface{}) {
	c.logger.Info("LLM Proxy Undeployment Event",
		slog.Any("payload", event["payload"]),
		slog.Any("timestamp", event["timestamp"]),
		slog.Any("correlationId", event["correlationId"]),
	)

	// Parse the event into structured format
	eventBytes, err := json.Marshal(event)
	if err != nil {
		c.logger.Error("Failed to marshal LLM proxy undeployment event for parsing",
			slog.Any("error", err),
		)
		return
	}

	var undeployedEvent LLMProxyUndeployedEvent
	if err := json.Unmarshal(eventBytes, &undeployedEvent); err != nil {
		c.logger.Error("Failed to parse LLM proxy undeployment event",
			slog.Any("error", err),
		)
		return
	}

	proxyID := undeployedEvent.Payload.ProxyID
	if proxyID == "" {
		c.logger.Error("Proxy ID is empty in LLM proxy undeployment event")
		return
	}

	c.logger.Info("Processing LLM proxy undeployment",
		slog.String("proxy_id", proxyID),
		slog.String("deployment_id", undeployedEvent.Payload.DeploymentID),
		slog.String("correlation_id", undeployedEvent.CorrelationID),
	)

	// Look up existing config
	proxyConfig, err := c.findAPIConfig(proxyID)
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.logger.Warn("LLM proxy configuration not found for undeployment",
				slog.String("proxy_id", proxyID),
			)
			// Still send success ack - the proxy is already gone
			c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, proxyID, "llmproxy", "undeploy", "success",
				undeployedEvent.Payload.PerformedAt, "")
			return
		}
		c.logger.Error("Failed to fetch LLM proxy configuration for undeployment",
			slog.String("proxy_id", proxyID),
			slog.Any("error", err),
		)
		c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, proxyID, "llmproxy", "undeploy", "failed",
			undeployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	// Only process undeploy if the event's DeploymentID matches the current one.
	if proxyConfig.DeploymentID != "" && undeployedEvent.Payload.DeploymentID != "" &&
		proxyConfig.DeploymentID != undeployedEvent.Payload.DeploymentID {
		c.logger.Warn("Ignoring stale LLM proxy undeploy event: deployment ID mismatch",
			slog.String("proxy_id", proxyID),
			slog.String("event_deployment_id", undeployedEvent.Payload.DeploymentID),
			slog.String("current_deployment_id", proxyConfig.DeploymentID),
		)
		c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, proxyID, "llmproxy", "undeploy", "failed",
			undeployedEvent.Payload.PerformedAt, "DEPLOYMENT_ID_MISMATCH")
		return
	}

	// Soft undeploy: set desired_state to undeployed (preserve config, keys, and policies)
	performedAt := undeployedEvent.Payload.PerformedAt.Truncate(time.Millisecond)
	if performedAt.IsZero() {
		performedAt = time.Now().Truncate(time.Millisecond)
	}
	proxyConfig.DesiredState = models.StateUndeployed
	proxyConfig.DeploymentID = undeployedEvent.Payload.DeploymentID
	proxyConfig.DeployedAt = &performedAt
	proxyConfig.UpdatedAt = time.Now()

	// Timestamp-guarded upsert: only writes if deployed_at is newer than what's in DB.
	affected, err := c.db.UpsertConfig(proxyConfig)
	if err != nil {
		c.logger.Error("Failed to upsert config for LLM proxy undeployment",
			slog.String("proxy_id", proxyID),
			slog.Any("error", err),
		)
		c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, proxyID, "llmproxy", "undeploy", "failed",
			undeployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}
	if !affected {
		c.logger.Debug("Skipped stale LLM proxy undeploy event (newer version exists in DB)",
			slog.String("proxy_id", proxyID),
			slog.String("deployment_id", undeployedEvent.Payload.DeploymentID),
		)
		return
	}

	evt := eventhub.Event{
		EventType: eventhub.EventTypeLLMProxy,
		Action:    "UPDATE",
		EntityID:  proxyID,
		EventID:   undeployedEvent.CorrelationID,
	}
	if err := c.eventHub.PublishEvent(c.gatewayID, evt); err != nil {
		c.logger.Error("Failed to publish LLM proxy undeployment event", slog.Any("error", err))
	}

	c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, proxyID, "llmproxy", "undeploy", "success",
		undeployedEvent.Payload.PerformedAt, "")

	c.logger.Info("Successfully processed LLM proxy undeployment event",
		slog.String("proxy_id", proxyID),
		slog.String("correlation_id", undeployedEvent.CorrelationID),
	)
}

// handleLLMProviderDeletedEvent handles LLM provider deletion events.
// This performs a hard delete, permanently removing the provider and all related resources.
func (c *Client) handleLLMProviderDeletedEvent(event map[string]interface{}) {
	c.logger.Info("LLM Provider Deletion Event",
		slog.Any("payload", event["payload"]),
		slog.Any("timestamp", event["timestamp"]),
		slog.Any("correlationId", event["correlationId"]),
	)

	eventBytes, err := json.Marshal(event)
	if err != nil {
		c.logger.Error("Failed to marshal LLM provider deletion event for parsing",
			slog.Any("error", err),
		)
		return
	}

	var deletedEvent LLMProviderDeletedEvent
	if err := json.Unmarshal(eventBytes, &deletedEvent); err != nil {
		c.logger.Error("Failed to parse LLM provider deletion event",
			slog.Any("error", err),
		)
		return
	}

	providerID := deletedEvent.Payload.ProviderID
	if providerID == "" {
		c.logger.Error("Provider ID is empty in LLM provider deletion event")
		return
	}

	if c.llmDeploymentService == nil {
		c.logger.Error("LLM deployment service not available",
			slog.String("provider_id", providerID),
			slog.String("correlation_id", deletedEvent.CorrelationID),
		)
		return
	}

	// Check if provider exists on this gateway
	providerConfig, err := c.findAPIConfig(providerID)
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.logger.Info("LLM provider configuration not found, cleaning up orphaned API keys",
				slog.String("provider_id", providerID),
				slog.String("correlation_id", deletedEvent.CorrelationID),
			)
			// Clean up any orphaned API keys that may exist for this provider
			if dbErr := c.db.RemoveAPIKeysAPI(providerID); dbErr != nil {
				c.logger.Warn("Failed to remove orphaned API keys for LLM provider",
					slog.String("provider_id", providerID),
					slog.Any("error", dbErr),
				)
			}
			// Publish DELETE event to EventHub for HA replica sync
			evt := eventhub.Event{
				EventType: eventhub.EventTypeLLMProvider,
				Action:    "DELETE",
				EntityID:  providerID,
				EventID:   deletedEvent.CorrelationID,
			}
			if pubErr := c.eventHub.PublishEvent(c.gatewayID, evt); pubErr != nil {
				c.logger.Error("Failed to publish orphan cleanup event for LLM provider", slog.Any("error", pubErr))
			}
			return
		}
		c.logger.Error("Failed to fetch LLM provider configuration for deletion, aborting",
			slog.String("provider_id", providerID),
			slog.String("correlation_id", deletedEvent.CorrelationID),
			slog.Any("error", err),
		)
		return
	}

	// Delete via LLM deployment service (handles DB cleanup, store cleanup,
	// eventHub publish, template mapping removal, and xDS update)
	_, err = c.llmDeploymentService.DeleteLLMProvider(providerConfig.Handle, deletedEvent.CorrelationID, c.logger)
	if err != nil {
		c.logger.Error("Failed to delete LLM provider configuration",
			slog.String("provider_id", providerID),
			slog.Any("error", err),
		)
		return
	}

	c.logger.Info("Successfully processed LLM provider deletion event",
		slog.String("provider_id", providerID),
		slog.String("correlation_id", deletedEvent.CorrelationID),
	)
}

// handleLLMProxyDeletedEvent handles LLM proxy deletion events.
// This performs a hard delete, permanently removing the proxy and all related resources.
func (c *Client) handleLLMProxyDeletedEvent(event map[string]interface{}) {
	c.logger.Info("LLM Proxy Deletion Event",
		slog.Any("payload", event["payload"]),
		slog.Any("timestamp", event["timestamp"]),
		slog.Any("correlationId", event["correlationId"]),
	)

	eventBytes, err := json.Marshal(event)
	if err != nil {
		c.logger.Error("Failed to marshal LLM proxy deletion event for parsing",
			slog.Any("error", err),
		)
		return
	}

	var deletedEvent LLMProxyDeletedEvent
	if err := json.Unmarshal(eventBytes, &deletedEvent); err != nil {
		c.logger.Error("Failed to parse LLM proxy deletion event",
			slog.Any("error", err),
		)
		return
	}

	proxyID := deletedEvent.Payload.ProxyID
	if proxyID == "" {
		c.logger.Error("Proxy ID is empty in LLM proxy deletion event")
		return
	}

	if c.llmDeploymentService == nil {
		c.logger.Error("LLM deployment service not available",
			slog.String("proxy_id", proxyID),
			slog.String("correlation_id", deletedEvent.CorrelationID),
		)
		return
	}

	// Check if proxy exists on this gateway
	proxyConfig, err := c.findAPIConfig(proxyID)
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.logger.Info("LLM proxy configuration not found, cleaning up orphaned API keys",
				slog.String("proxy_id", proxyID),
				slog.String("correlation_id", deletedEvent.CorrelationID),
			)
			// Clean up any orphaned API keys that may exist for this proxy
			if dbErr := c.db.RemoveAPIKeysAPI(proxyID); dbErr != nil {
				c.logger.Warn("Failed to remove orphaned API keys for LLM proxy",
					slog.String("proxy_id", proxyID),
					slog.Any("error", dbErr),
				)
			}
			// Publish DELETE event to EventHub for HA replica sync
			evt := eventhub.Event{
				EventType: eventhub.EventTypeLLMProxy,
				Action:    "DELETE",
				EntityID:  proxyID,
				EventID:   deletedEvent.CorrelationID,
			}
			if pubErr := c.eventHub.PublishEvent(c.gatewayID, evt); pubErr != nil {
				c.logger.Error("Failed to publish orphan cleanup event for LLM proxy", slog.Any("error", pubErr))
			}
			return
		}
		c.logger.Error("Failed to fetch LLM proxy configuration for deletion, aborting",
			slog.String("proxy_id", proxyID),
			slog.String("correlation_id", deletedEvent.CorrelationID),
			slog.Any("error", err),
		)
		return
	}

	// Delete via LLM deployment service (handles DB cleanup, store cleanup,
	// eventHub publish, and xDS update)
	_, err = c.llmDeploymentService.DeleteLLMProxy(proxyConfig.Handle, deletedEvent.CorrelationID, c.logger)
	if err != nil {
		c.logger.Error("Failed to delete LLM proxy configuration",
			slog.String("proxy_id", proxyID),
			slog.Any("error", err),
		)
		return
	}

	c.logger.Info("Successfully processed LLM proxy deletion event",
		slog.String("proxy_id", proxyID),
		slog.String("correlation_id", deletedEvent.CorrelationID),
	)
}

func (c *Client) handleWebSubAPIDeployedEvent(event map[string]any) {
	c.logger.Debug("WebSub API Deployment Event",
		slog.Any("payload", event["payload"]),
		slog.Any("timestamp", event["timestamp"]),
		slog.Any("correlationId", event["correlationId"]),
	)

	eventBytes, err := json.Marshal(event)
	if err != nil {
		c.logger.Error("Failed to marshal WebSub API deployment event for parsing",
			slog.Any("error", err),
		)
		return
	}

	var deployedEvent WebSubAPIDeployedEvent
	if err := json.Unmarshal(eventBytes, &deployedEvent); err != nil {
		c.logger.Error("Failed to parse WebSub API deployment event",
			slog.Any("error", err),
		)
		return
	}

	apiID := deployedEvent.Payload.APIID
	if apiID == "" {
		c.logger.Error("API ID is empty in WebSub API deployment event")
		return
	}

	c.logger.Info("Processing WebSub API deployment",
		slog.String("api_id", apiID),
		slog.String("deployment_id", deployedEvent.Payload.DeploymentID),
		slog.String("correlation_id", deployedEvent.CorrelationID),
	)

	// Fetch WebSub API definition from control plane
	zipData, err := c.apiUtilsService.FetchWebSubAPIDefinition(apiID)
	if err != nil {
		c.logger.Error("Failed to fetch WebSub API definition",
			slog.String("api_id", apiID),
			slog.Any("error", err),
		)
		c.sendDeploymentAck(deployedEvent.Payload.DeploymentID, apiID, "websub", "deploy", "failed",
			deployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	yamlData, err := c.apiUtilsService.ExtractYAMLFromZip(zipData)
	if err != nil {
		c.logger.Error("Failed to extract YAML from WebSub API ZIP",
			slog.String("api_id", apiID),
			slog.Any("error", err),
		)
		c.sendDeploymentAck(deployedEvent.Payload.DeploymentID, apiID, "websub", "deploy", "failed",
			deployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	performedAt := deployedEvent.Payload.PerformedAt.Truncate(time.Millisecond)
	if performedAt.IsZero() {
		performedAt = time.Now().Truncate(time.Millisecond)
	}
	result, err := c.apiUtilsService.CreateAPIFromYAML(yamlData, apiID, deployedEvent.Payload.DeploymentID, &performedAt, deployedEvent.CorrelationID, c.deploymentService)
	if err != nil {
		c.logger.Error("Failed to create WebSub API from YAML",
			slog.String("api_id", apiID),
			slog.Any("error", err),
		)
		c.sendDeploymentAck(deployedEvent.Payload.DeploymentID, apiID, "websub", "deploy", "failed",
			deployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	if result.IsStale {
		c.logger.Debug("Skipped stale WebSub API deploy event (newer version exists in DB)",
			slog.String("api_id", apiID),
			slog.String("deployment_id", deployedEvent.Payload.DeploymentID),
		)
		return
	}

	c.sendDeploymentAck(deployedEvent.Payload.DeploymentID, apiID, "websub", "deploy", "success",
		deployedEvent.Payload.PerformedAt, "")

	c.logger.Info("Successfully processed WebSub API deployment event",
		slog.String("api_id", apiID),
		slog.String("correlation_id", deployedEvent.CorrelationID),
	)
}

func (c *Client) handleWebSubAPIUndeployedEvent(event map[string]any) {
	c.logger.Debug("WebSub API Undeployment Event",
		slog.Any("payload", event["payload"]),
		slog.Any("timestamp", event["timestamp"]),
		slog.Any("correlationId", event["correlationId"]),
	)

	eventBytes, err := json.Marshal(event)
	if err != nil {
		c.logger.Error("Failed to marshal WebSub API undeployment event for parsing",
			slog.Any("error", err),
		)
		return
	}

	var undeployedEvent WebSubAPIUndeployedEvent
	if err := json.Unmarshal(eventBytes, &undeployedEvent); err != nil {
		c.logger.Error("Failed to parse WebSub API undeployment event",
			slog.Any("error", err),
		)
		return
	}

	apiID := undeployedEvent.Payload.APIID
	if apiID == "" {
		c.logger.Error("API ID is empty in WebSub API undeployment event")
		return
	}

	apiConfig, err := c.findAPIConfig(apiID)
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.logger.Warn("WebSub API configuration not found for undeployment",
				slog.String("api_id", apiID),
			)
			c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, apiID, "websub", "undeploy", "success",
				undeployedEvent.Payload.PerformedAt, "")
			return
		}
		c.logger.Error("Failed to fetch WebSub API configuration for undeployment",
			slog.String("api_id", apiID),
			slog.String("correlation_id", undeployedEvent.CorrelationID),
			slog.Any("error", err),
		)
		c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, apiID, "websub", "undeploy", "failed",
			undeployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	if apiConfig.DeploymentID != "" && undeployedEvent.Payload.DeploymentID != "" &&
		apiConfig.DeploymentID != undeployedEvent.Payload.DeploymentID {
		c.logger.Warn("Ignoring stale WebSub API undeploy event: deployment ID mismatch",
			slog.String("api_id", apiID),
			slog.String("event_deployment_id", undeployedEvent.Payload.DeploymentID),
			slog.String("current_deployment_id", apiConfig.DeploymentID),
		)
		c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, apiID, "websub", "undeploy", "failed",
			undeployedEvent.Payload.PerformedAt, "DEPLOYMENT_ID_MISMATCH")
		return
	}

	performedAt := undeployedEvent.Payload.PerformedAt.Truncate(time.Millisecond)
	if performedAt.IsZero() {
		performedAt = time.Now().Truncate(time.Millisecond)
	}
	apiConfig.DesiredState = models.StateUndeployed
	apiConfig.DeploymentID = undeployedEvent.Payload.DeploymentID
	apiConfig.DeployedAt = &performedAt
	apiConfig.UpdatedAt = time.Now()

	affected, err := c.db.UpsertConfig(apiConfig)
	if err != nil {
		c.logger.Error("Failed to upsert config for WebSub API undeployment",
			slog.String("api_id", apiID),
			slog.Any("error", err),
		)
		c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, apiID, "websub", "undeploy", "failed",
			undeployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}
	if !affected {
		c.logger.Debug("Skipped stale WebSub API undeploy event (newer version exists in DB)",
			slog.String("api_id", apiID),
			slog.String("deployment_id", undeployedEvent.Payload.DeploymentID),
		)
		return
	}

	evt := eventhub.Event{
		EventType: eventhub.EventTypeAPI,
		Action:    "UPDATE",
		EntityID:  apiID,
		EventID:   undeployedEvent.CorrelationID,
	}
	if err := c.eventHub.PublishEvent(c.gatewayID, evt); err != nil {
		c.logger.Error("Failed to publish WebSub API undeployment event", slog.Any("error", err))
	}

	c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, apiID, "websub", "undeploy", "success",
		undeployedEvent.Payload.PerformedAt, "")

	c.logger.Info("Successfully processed WebSub API undeployment event",
		slog.String("api_id", apiID),
		slog.String("correlation_id", undeployedEvent.CorrelationID),
	)
}

func (c *Client) handleWebSubAPIDeletedEvent(event map[string]any) {
	c.logger.Debug("WebSub API Deleted Event",
		slog.Any("payload", event["payload"]),
		slog.Any("timestamp", event["timestamp"]),
		slog.Any("correlationId", event["correlationId"]),
	)

	eventBytes, err := json.Marshal(event)
	if err != nil {
		c.logger.Error("Failed to marshal WebSub API deleted event for parsing",
			slog.Any("error", err),
		)
		return
	}

	var deletedEvent WebSubAPIDeletedEvent
	if err := json.Unmarshal(eventBytes, &deletedEvent); err != nil {
		c.logger.Error("Failed to parse WebSub API deleted event",
			slog.Any("error", err),
		)
		return
	}

	apiID := deletedEvent.Payload.APIID
	if apiID == "" {
		c.logger.Error("API ID is empty in WebSub API deleted event")
		return
	}

	apiConfig, err := c.findAPIConfig(apiID)
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.logger.Warn("WebSub API configuration not found for deletion",
				slog.String("api_id", apiID),
			)
			return
		}
		c.logger.Error("Failed to fetch WebSub API configuration for deletion",
			slog.String("api_id", apiID),
			slog.String("correlation_id", deletedEvent.CorrelationID),
			slog.Any("error", err),
		)
		return
	}

	c.performFullAPIDeletion(apiID, apiConfig, deletedEvent.CorrelationID)
}

func (c *Client) handleMCPProxyDeploymentEvent(event map[string]any) {
	c.logger.Debug("MCP Proxy Deployment Event",
		slog.Any("payload", event["payload"]),
		slog.Any("timestamp", event["timestamp"]),
		slog.Any("correlationId", event["correlationId"]),
	)

	// Parse the event into structured format
	eventBytes, err := json.Marshal(event)
	if err != nil {
		c.logger.Error("Failed to marshal MCP proxy deployment event for parsing",
			slog.Any("error", err),
		)
		return
	}

	var deployedEvent MCPProxyDeployedEvent
	if err := json.Unmarshal(eventBytes, &deployedEvent); err != nil {
		c.logger.Error("Failed to parse MCP proxy deployment event",
			slog.Any("error", err),
		)
		return
	}

	proxyID := deployedEvent.Payload.ProxyID
	if proxyID == "" {
		c.logger.Error("Proxy ID is empty in MCP proxy deployment event")
		return
	}

	c.logger.Debug("Processing MCP proxy deployment",
		slog.String("proxy_id", proxyID),
		slog.String("deployment_id", deployedEvent.Payload.DeploymentID),
		slog.String("correlation_id", deployedEvent.CorrelationID),
	)

	// Fetch MCP proxy definition from control plane
	zipData, err := c.apiUtilsService.FetchMCPProxyDefinition(proxyID)
	if err != nil {
		c.logger.Error("Failed to fetch MCP proxy definition",
			slog.String("proxy_id", proxyID),
			slog.Any("error", err),
		)
		c.sendDeploymentAck(deployedEvent.Payload.DeploymentID, proxyID, "mcpproxy", "deploy", "failed",
			deployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	// Extract YAML from ZIP
	yamlData, err := c.apiUtilsService.ExtractYAMLFromZip(zipData)
	if err != nil {
		c.logger.Error("Failed to extract YAML from MCP proxy ZIP",
			slog.String("proxy_id", proxyID),
			slog.Any("error", err),
		)
		c.sendDeploymentAck(deployedEvent.Payload.DeploymentID, proxyID, "mcpproxy", "deploy", "failed",
			deployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	if c.mcpDeploymentService == nil {
		c.logger.Error("MCP deployment service not available",
			slog.String("proxy_id", proxyID),
			slog.String("correlation_id", deployedEvent.CorrelationID),
		)
		c.sendDeploymentAck(deployedEvent.Payload.DeploymentID, proxyID, "mcpproxy", "deploy", "failed",
			deployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	// Create MCP proxy configuration from YAML using the deployment service
	mcpPerformedAt := deployedEvent.Payload.PerformedAt.Truncate(time.Millisecond)
	if mcpPerformedAt.IsZero() {
		mcpPerformedAt = time.Now().Truncate(time.Millisecond)
	}
	result, err := c.apiUtilsService.CreateMCPProxyFromYAML(yamlData, proxyID, deployedEvent.Payload.DeploymentID, &mcpPerformedAt, deployedEvent.CorrelationID, c.mcpDeploymentService)
	if err != nil {
		c.logger.Error("Failed to create MCP proxy from YAML",
			slog.String("proxy_id", proxyID),
			slog.Any("error", err),
		)
		c.sendDeploymentAck(deployedEvent.Payload.DeploymentID, proxyID, "mcpproxy", "deploy", "failed",
			deployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	if result.IsStale {
		// Stale event — DB was not modified. Do not send ack; in HA mode the
		// controller that actually processed the event will ack. If all controllers
		// see stale, platform-API will timeout and handle accordingly.
		c.logger.Debug("Skipped stale MCP proxy deploy event (newer version exists in DB)",
			slog.String("proxy_id", proxyID),
			slog.String("deployment_id", deployedEvent.Payload.DeploymentID),
		)
		return
	}

	c.sendDeploymentAck(deployedEvent.Payload.DeploymentID, proxyID, "mcpproxy", "deploy", "success",
		deployedEvent.Payload.PerformedAt, "")

	c.logger.Info("Successfully processed MCP proxy deployment event",
		slog.String("proxy_id", proxyID),
		slog.String("correlation_id", deployedEvent.CorrelationID),
	)
}

func (c *Client) handleMCPProxyUndeploymentEvent(event map[string]any) {
	c.logger.Debug("MCP Proxy Undeployment Event",
		slog.Any("payload", event["payload"]),
		slog.Any("timestamp", event["timestamp"]),
		slog.Any("correlationId", event["correlationId"]),
	)

	// Parse the event into structured format
	eventBytes, err := json.Marshal(event)
	if err != nil {
		c.logger.Error("Failed to marshal MCP proxy undeployment event for parsing",
			slog.Any("error", err),
		)
		return
	}

	var undeployedEvent MCPProxyUndeployedEvent
	if err := json.Unmarshal(eventBytes, &undeployedEvent); err != nil {
		c.logger.Error("Failed to parse MCP proxy undeployment event",
			slog.Any("error", err),
		)
		return
	}

	proxyID := undeployedEvent.Payload.ProxyID
	if proxyID == "" {
		c.logger.Error("Proxy ID is empty in MCP proxy undeployment event")
		return
	}

	if c.mcpDeploymentService == nil {
		c.logger.Error("MCP deployment service not available",
			slog.String("proxy_id", proxyID),
			slog.String("correlation_id", undeployedEvent.CorrelationID),
		)
		c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, proxyID, "mcpproxy", "undeploy", "failed",
			undeployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}

	_, err = c.mcpDeploymentService.UndeployMCPProxy(
		proxyID,
		undeployedEvent.Payload.DeploymentID,
		&undeployedEvent.Payload.PerformedAt,
		undeployedEvent.CorrelationID,
		c.logger,
	)
	if err != nil {
		if storage.IsNotFoundError(err) || strings.Contains(strings.ToLower(err.Error()), "not found") {
			c.logger.Warn("MCP proxy configuration not found for undeployment",
				slog.String("proxy_id", proxyID),
			)
			c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, proxyID, "mcpproxy", "undeploy", "success",
				undeployedEvent.Payload.PerformedAt, "")
			return
		}
		if errors.Is(err, utils.ErrMCPDeploymentIDMismatch) {
			c.logger.Warn("Ignoring stale MCP proxy undeploy event: deployment ID mismatch",
				slog.String("proxy_id", proxyID),
				slog.String("event_deployment_id", undeployedEvent.Payload.DeploymentID),
			)
			c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, proxyID, "mcpproxy", "undeploy", "failed",
				undeployedEvent.Payload.PerformedAt, "DEPLOYMENT_ID_MISMATCH")
			return
		}
		if errors.Is(err, utils.ErrMCPUndeployStale) {
			c.logger.Debug("Skipped stale MCP proxy undeploy event (newer version exists in DB)",
				slog.String("proxy_id", proxyID),
				slog.String("deployment_id", undeployedEvent.Payload.DeploymentID),
			)
			return
		}
		c.logger.Error("Failed to undeploy MCP proxy configuration",
			slog.String("proxy_id", proxyID),
			slog.String("correlation_id", undeployedEvent.CorrelationID),
			slog.Any("error", err),
		)
		c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, proxyID, "mcpproxy", "undeploy", "failed",
			undeployedEvent.Payload.PerformedAt, "GATEWAY_PROCESSING_ERROR")
		return
	}
	c.sendDeploymentAck(undeployedEvent.Payload.DeploymentID, proxyID, "mcpproxy", "undeploy", "success",
		undeployedEvent.Payload.PerformedAt, "")
	c.logger.Info("Successfully processed MCP proxy undeployment event",
		slog.String("proxy_id", proxyID),
		slog.String("correlation_id", undeployedEvent.CorrelationID),
	)
}

func (c *Client) handleMCPProxyDeletedEvent(event map[string]any) {
	c.logger.Debug("MCP Proxy Deleted Event",
		slog.Any("payload", event["payload"]),
		slog.Any("timestamp", event["timestamp"]),
		slog.Any("correlationId", event["correlationId"]),
	)

	// Parse the event into structured format
	eventBytes, err := json.Marshal(event)
	if err != nil {
		c.logger.Error("Failed to marshal MCP proxy deleted event for parsing",
			slog.Any("error", err),
		)
		return
	}

	var deletedEvent MCPProxyDeletedEvent
	if err := json.Unmarshal(eventBytes, &deletedEvent); err != nil {
		c.logger.Error("Failed to parse MCP proxy deleted event",
			slog.Any("error", err),
		)
		return
	}

	proxyID := deletedEvent.Payload.ProxyID
	if proxyID == "" {
		c.logger.Error("Proxy ID is empty in MCP proxy deleted event")
		return
	}

	// Check if MCP proxy exists on this gateway
	mcpConfig, err := c.findAPIConfig(proxyID)
	if err != nil {
		if storage.IsNotFoundError(err) {
			c.logger.Warn("MCP proxy configuration not found for deletion",
				slog.String("proxy_id", proxyID),
			)
			// Not an error - the MCP proxy might already be undeployed or deleted
			return
		}
		// Real storage error - log and abort
		c.logger.Error("Failed to fetch MCP proxy configuration for deletion",
			slog.String("proxy_id", proxyID),
			slog.String("correlation_id", deletedEvent.CorrelationID),
			slog.Any("error", err),
		)
		return
	}

	if c.mcpDeploymentService == nil {
		c.logger.Error("MCP deployment service not available",
			slog.String("proxy_id", proxyID),
			slog.String("correlation_id", deletedEvent.CorrelationID),
		)
		return
	}

	_, err = c.mcpDeploymentService.DeleteMCPProxy(mcpConfig.Handle, deletedEvent.CorrelationID, c.logger)
	if err != nil {
		c.logger.Error("Failed to delete MCP proxy configuration",
			slog.String("proxy_id", proxyID),
			slog.Any("error", err),
		)
		return
	}

	c.logger.Debug("Successfully processed MCP proxy deleted event",
		slog.String("proxy_id", proxyID),
		slog.String("correlation_id", deletedEvent.CorrelationID),
	)
}

// handleAPIKeyCreatedEvent handles API key created events from platform-api
func (c *Client) handleAPIKeyCreatedEvent(event map[string]interface{}) {
	baseLogger := c.logger
	if baseLogger == nil {
		baseLogger = slog.Default()
	}
	baseLogger.Info("API Key Created Event received",
		slog.Any("correlation_id", event["correlationId"]),
		slog.Any("timestamp", event["timestamp"]),
	)

	eventBytes, err := json.Marshal(event)
	if err != nil {
		baseLogger.Error("Failed to marshal API key created event for parsing",
			slog.Any("correlation_id", event["correlationId"]),
			slog.Any("error", err),
		)
		return
	}

	var keyCreatedEvent APIKeyCreatedEvent
	if err := json.Unmarshal(eventBytes, &keyCreatedEvent); err != nil {
		baseLogger.Error("Failed to parse API key created event",
			slog.Any("correlation_id", event["correlationId"]),
			slog.Any("error", err),
		)
		return
	}

	// Defensive nil/empty checks on required fields before logging or proceeding
	if keyCreatedEvent.Payload.ApiId == "" {
		baseLogger.Error("API key created event missing required api_id",
			slog.Any("correlation_id", event["correlationId"]),
		)
		return
	}
	if keyCreatedEvent.Payload.ApiKeyHashes == "" {
		baseLogger.Error("API key created event missing required api_key_hashes",
			slog.Any("correlation_id", event["correlationId"]),
		)
		return
	}
	// Validate Name - required field for external API key events
	// Since no response is sent back through WebSocket, the caller must know the identifier
	if keyCreatedEvent.Payload.Name != "" {
		// Validate the name format
		if err := utils.ValidateAPIKeyName(keyCreatedEvent.Payload.Name); err != nil {
			baseLogger.Error("API key created event has invalid name",
				slog.Any("correlation_id", event["correlationId"]),
				slog.Any("error", err),
			)
			return
		}
	}

	logger := baseLogger.With(
		slog.String("correlation_id", keyCreatedEvent.CorrelationID),
		slog.String("user_id", keyCreatedEvent.UserId),
		slog.String("api_id", keyCreatedEvent.Payload.ApiId),
	)

	payload := keyCreatedEvent.Payload

	var expiresAt *time.Time
	var duration *int
	now := time.Now()

	var keyUUID *string
	if payload.UUID != "" {
		keyUUID = &payload.UUID
	}

	// Parse authoritative timestamps from the platform API
	var createdAt, updatedAt *time.Time
	if payload.CreatedAt != "" {
		t, err := time.Parse(time.RFC3339, payload.CreatedAt)
		if err != nil {
			logger.Warn("Invalid created_at format in API key event, using local time",
				slog.String("created_at", payload.CreatedAt),
				slog.Any("error", err),
			)
		} else {
			createdAt = &t
		}
	}
	if payload.UpdatedAt != "" {
		t, err := time.Parse(time.RFC3339, payload.UpdatedAt)
		if err != nil {
			logger.Warn("Invalid updated_at format in API key event, using local time",
				slog.String("updated_at", payload.UpdatedAt),
				slog.Any("error", err),
			)
		} else {
			updatedAt = &t
		}
	}

	apiKeyCreationRequest := api.APIKeyCreationRequest{
		MaskedApiKey:  &payload.MaskedApiKey,
		Name:          &payload.Name,
		ExternalRefId: payload.ExternalRefId,
		Issuer:        payload.Issuer,
	}
	if payload.ExpiresAt != nil {
		// payload.ExpiresAt is likely a *string (RFC3339). Attempt to parse it to time.Time
		parsedExpiresAt, err := time.Parse(time.RFC3339, *payload.ExpiresAt)
		if err != nil {
			logger.Error("Invalid expires_at format for API key, expected RFC3339",
				slog.Any("expires_at", *payload.ExpiresAt),
				slog.Any("error", err),
			)
			return
		}
		if parsedExpiresAt.Before(now) {
			logger.Error("API key expiration time must be in the future",
				slog.String("expires_at", parsedExpiresAt.Format(time.RFC3339)),
				slog.String("now", now.Format(time.RFC3339)))
			return
		}
		// If expires_at is explicitly provided, use it
		expiresAt = &parsedExpiresAt
		apiKeyCreationRequest.ExpiresAt = expiresAt
	} else if payload.ExpiresIn != nil {
		duration = &payload.ExpiresIn.Duration
		timeDuration := time.Duration(*duration)
		switch payload.ExpiresIn.Unit {
		case string(api.APIKeyCreationRequestExpiresInUnitSeconds):
			timeDuration *= time.Second
		case string(api.APIKeyCreationRequestExpiresInUnitMinutes):
			timeDuration *= time.Minute
		case string(api.APIKeyCreationRequestExpiresInUnitHours):
			timeDuration *= time.Hour
		case string(api.APIKeyCreationRequestExpiresInUnitDays):
			timeDuration *= 24 * time.Hour
		case string(api.APIKeyCreationRequestExpiresInUnitWeeks):
			timeDuration *= 7 * 24 * time.Hour
		case string(api.APIKeyCreationRequestExpiresInUnitMonths):
			timeDuration *= 30 * 24 * time.Hour // Approximate month as 30 days
		default:
			logger.Error("Unsupported expiration unit", slog.Any("expires_in.unit", payload.ExpiresIn.Unit))
			return
		}
		expiry := now.Add(timeDuration)
		expiresAt = &expiry
		apiKeyCreationRequest.ExpiresAt = expiresAt
	}

	result, err := c.apiKeyService.CreateExternalAPIKeyFromEvent(
		payload.ApiId,
		keyCreatedEvent.UserId,
		&apiKeyCreationRequest,
		keyUUID,
		&payload.ApiKeyHashes,
		keyCreatedEvent.CorrelationID,
		logger,
		createdAt,
		updatedAt,
	)
	if err != nil {
		logger.Error("Failed to create external API key", slog.Any("error", err))
		return
	}

	logger.Info("Successfully processed API key created event",
		slog.String("api_key_name", result.Response.ApiKey.Name),
	)
}

// handleAPIKeyRevokedEvent handles API key revoked events from platform-api
func (c *Client) handleAPIKeyRevokedEvent(event map[string]interface{}) {
	baseLogger := c.logger
	if baseLogger == nil {
		baseLogger = slog.Default()
	}
	baseLogger.Info("API Key Revoked Event received",
		slog.Any("correlation_id", event["correlationId"]),
		slog.Any("timestamp", event["timestamp"]),
	)

	eventBytes, err := json.Marshal(event)
	if err != nil {
		baseLogger.Error("Failed to marshal API key revoked event for parsing",
			slog.Any("correlation_id", event["correlationId"]),
			slog.Any("error", err),
		)
		return
	}

	var evt APIKeyRevokedEvent
	if err := json.Unmarshal(eventBytes, &evt); err != nil {
		baseLogger.Error("Failed to parse API key revoked event",
			slog.Any("correlation_id", event["correlationId"]),
			slog.Any("error", err),
		)
		return
	}

	// Defensive nil/empty checks on required fields before logging or proceeding
	if evt.Payload.ApiId == "" {
		baseLogger.Error("API key revoked event missing required api_id",
			slog.Any("correlation_id", event["correlationId"]),
		)
		return
	}
	if evt.Payload.KeyName == "" {
		baseLogger.Error("API key revoked event missing required key_name",
			slog.Any("correlation_id", event["correlationId"]),
		)
		return
	}

	logger := baseLogger.With(
		slog.String("correlation_id", evt.CorrelationID),
		slog.String("user_id", evt.UserId),
		slog.String("api_id", evt.Payload.ApiId),
		slog.String("api_key_name", evt.Payload.KeyName),
	)

	payload := evt.Payload

	err = c.apiKeyService.RevokeExternalAPIKeyFromEvent(
		payload.ApiId,
		payload.KeyName,
		evt.UserId,
		evt.CorrelationID,
		logger,
	)
	if err != nil {
		logger.Error("Failed to revoke external API key", slog.Any("error", err))
		return
	}

	logger.Info("Successfully processed API key revoked event")
}

// handleAPIKeyUpdatedEvent handles API key updated events from platform-api.
func (c *Client) handleAPIKeyUpdatedEvent(event map[string]interface{}) {
	baseLogger := c.logger
	if baseLogger == nil {
		baseLogger = slog.Default()
	}
	baseLogger.Info("API Key Updated Event received",
		slog.Any("correlation_id", event["correlationId"]),
		slog.Any("timestamp", event["timestamp"]),
	)
	eventBytes, err := json.Marshal(event)
	if err != nil {
		baseLogger.Error("Failed to marshal event for parsing",
			slog.Any("correlation_id", event["correlationId"]),
			slog.Any("error", err),
		)
		return
	}

	var evt APIKeyUpdatedEvent
	if err := json.Unmarshal(eventBytes, &evt); err != nil {
		baseLogger.Error("Failed to parse API key updated event",
			slog.Any("correlation_id", event["correlationId"]),
			slog.Any("error", err),
		)
		return
	}

	payload := evt.Payload

	// Defensive nil/empty checks on required fields
	if payload.ApiId == "" {
		baseLogger.Error("API key updated event missing required api_id",
			slog.Any("correlation_id", event["correlationId"]),
		)
		return
	}
	if payload.KeyName == "" {
		baseLogger.Error("API key updated event missing required key_name",
			slog.Any("correlation_id", event["correlationId"]),
		)
		return
	}
	if payload.ApiKeyHashes == "" {
		baseLogger.Error("API key updated event missing required api_key_hashes",
			slog.Any("correlation_id", event["correlationId"]),
			slog.String("api_id", payload.ApiId),
			slog.String("key_name", payload.KeyName),
		)
		return
	}
	logger := baseLogger.With(
		slog.String("correlation_id", evt.CorrelationID),
		slog.String("user_id", evt.UserId),
		slog.String("api_id", payload.ApiId),
		slog.String("key_name", payload.KeyName),
	)

	var expiresAt *time.Time
	var duration *int
	now := time.Now()

	// Parse authoritative updated_at timestamp from the platform API
	var updatedAt *time.Time
	if payload.UpdatedAt != "" {
		t, err := time.Parse(time.RFC3339, payload.UpdatedAt)
		if err != nil {
			logger.Warn("Invalid updated_at format in API key updated event, using local time",
				slog.String("updated_at", payload.UpdatedAt),
				slog.Any("error", err),
			)
		} else {
			updatedAt = &t
		}
	}

	apiKeyCreationRequest := api.APIKeyCreationRequest{
		MaskedApiKey:  &payload.MaskedApiKey,
		ExternalRefId: payload.ExternalRefId,
		Name:          &payload.KeyName,
		Issuer:        payload.Issuer,
	}
	if payload.ExpiresAt != nil {
		// payload.ExpiresAt is likely a *string (RFC3339). Attempt to parse it to time.Time
		parsedExpiresAt, err := time.Parse(time.RFC3339, *payload.ExpiresAt)
		if err != nil {
			logger.Error("Invalid expires_at format for API key, expected RFC3339",
				slog.Any("expires_at", *payload.ExpiresAt),
				slog.Any("error", err),
			)
			return
		}
		if parsedExpiresAt.Before(now) {
			logger.Error("API key expiration time must be in the future",
				slog.String("expires_at", parsedExpiresAt.Format(time.RFC3339)),
				slog.String("now", now.Format(time.RFC3339)))
			return
		}
		// If expires_at is explicitly provided, use it
		expiresAt = &parsedExpiresAt
		apiKeyCreationRequest.ExpiresAt = expiresAt
	} else if payload.ExpiresIn != nil {
		duration = &payload.ExpiresIn.Duration
		timeDuration := time.Duration(*duration)
		switch payload.ExpiresIn.Unit {
		case string(api.APIKeyCreationRequestExpiresInUnitSeconds):
			timeDuration *= time.Second
		case string(api.APIKeyCreationRequestExpiresInUnitMinutes):
			timeDuration *= time.Minute
		case string(api.APIKeyCreationRequestExpiresInUnitHours):
			timeDuration *= time.Hour
		case string(api.APIKeyCreationRequestExpiresInUnitDays):
			timeDuration *= 24 * time.Hour
		case string(api.APIKeyCreationRequestExpiresInUnitWeeks):
			timeDuration *= 7 * 24 * time.Hour
		case string(api.APIKeyCreationRequestExpiresInUnitMonths):
			timeDuration *= 30 * 24 * time.Hour // Approximate month as 30 days
		default:
			logger.Error("Unsupported expiration unit", slog.Any("expires_in.unit", payload.ExpiresIn.Unit))
			return
		}
		expiry := now.Add(timeDuration)
		expiresAt = &expiry
		apiKeyCreationRequest.ExpiresAt = expiresAt
	}

	err = c.apiKeyService.UpdateExternalAPIKeyFromEvent(
		payload.ApiId,
		payload.KeyName,
		&apiKeyCreationRequest,
		&payload.ApiKeyHashes,
		evt.UserId,
		evt.CorrelationID,
		logger,
		updatedAt,
	)
	if err != nil {
		logger.Error("Failed to update external API key", slog.Any("error", err))
		return
	}
	logger.Info("Successfully processed API key updated event")
}

// handleApplicationUpdatedEvent handles application mapping update events from platform-api.
func (c *Client) handleApplicationUpdatedEvent(event map[string]interface{}) {
	baseLogger := c.logger
	if baseLogger == nil {
		baseLogger = slog.Default()
	}

	resourceService := c.getSubscriptionResourceService()

	eventBytes, err := json.Marshal(event)
	if err != nil {
		baseLogger.Error("Failed to marshal application updated event for parsing",
			slog.Any("correlation_id", event["correlationId"]),
			slog.Any("error", err),
		)
		return
	}

	var evt ApplicationUpdatedEvent
	if err := json.Unmarshal(eventBytes, &evt); err != nil {
		baseLogger.Error("Failed to parse application updated event",
			slog.Any("correlation_id", event["correlationId"]),
			slog.Any("error", err),
		)
		return
	}

	if evt.Payload.ApplicationId == "" {
		baseLogger.Error("Application updated event missing required application_id",
			slog.Any("correlation_id", event["correlationId"]),
		)
		return
	}

	if evt.Payload.ApplicationUuid == "" {
		baseLogger.Error("Application updated event missing required application_uuid",
			slog.Any("correlation_id", event["correlationId"]),
		)
		return
	}

	if evt.Payload.ApplicationName == "" {
		baseLogger.Error("Application updated event missing required application_name",
			slog.Any("correlation_id", event["correlationId"]),
		)
		return
	}

	if evt.Payload.ApplicationType == "" {
		baseLogger.Error("Application updated event missing required application_type",
			slog.Any("correlation_id", event["correlationId"]),
		)
		return
	}

	logger := baseLogger.With(
		slog.String("correlation_id", evt.CorrelationID),
		slog.String("application_id", evt.Payload.ApplicationId),
		slog.String("application_uuid", evt.Payload.ApplicationUuid),
		slog.String("application_name", evt.Payload.ApplicationName),
		slog.String("application_type", evt.Payload.ApplicationType),
	)

	affectedAPIKeyUUIDs := make(map[string]struct{})
	apiKeysByUUID := make(map[string]*models.APIKey)
	if c.apiKeyXDSManager != nil {
		apiKeys, err := c.db.GetAllAPIKeys()
		if err != nil {
			logger.Error("Failed to load API keys for xDS refresh after application mapping update", slog.Any("error", err))
		} else {
			for _, apiKey := range apiKeys {
				if apiKey == nil || apiKey.UUID == "" {
					continue
				}

				apiKeysByUUID[apiKey.UUID] = apiKey
				if apiKey.ApplicationID == evt.Payload.ApplicationUuid {
					affectedAPIKeyUUIDs[apiKey.UUID] = struct{}{}
				}
			}
		}
	}

	resolvedMappings := make([]*models.ApplicationAPIKeyMapping, 0, len(evt.Payload.Mappings))

	for _, mapping := range evt.Payload.Mappings {
		if mapping.ApiKeyUuid == "" {
			logger.Warn("Skipping invalid application mapping entry in event",
				slog.String("api_key_uuid", mapping.ApiKeyUuid),
			)
			continue
		}

		apiKey, err := c.db.GetAPIKeyByUUID(mapping.ApiKeyUuid)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				logger.Warn("Skipping unresolved API key for application mapping",
					slog.String("api_key_uuid", mapping.ApiKeyUuid),
				)
				continue
			}

			logger.Error("Failed to resolve API key for application mapping",
				slog.String("api_key_uuid", mapping.ApiKeyUuid),
				slog.Any("error", err),
			)
			return
		}

		resolvedMappings = append(resolvedMappings, &models.ApplicationAPIKeyMapping{
			ApplicationUUID: evt.Payload.ApplicationUuid,
			APIKeyID:        apiKey.UUID,
		})
		affectedAPIKeyUUIDs[apiKey.UUID] = struct{}{}
	}

	application := &models.StoredApplication{
		ApplicationID:   evt.Payload.ApplicationId,
		ApplicationUUID: evt.Payload.ApplicationUuid,
		ApplicationName: evt.Payload.ApplicationName,
		ApplicationType: evt.Payload.ApplicationType,
	}

	if err := resourceService.ReplaceApplicationAPIKeyMappings(application, resolvedMappings, evt.CorrelationID, logger); err != nil {
		logger.Error("Failed to persist application key mappings", slog.Any("error", err))
		return
	}

	if c.apiKeyXDSManager != nil {
		cfgByArtifactUUID := make(map[string]*models.StoredConfig)
		missingCfgArtifactUUIDs := make(map[string]error)

		for apiKeyUUID := range affectedAPIKeyUUIDs {
			apiKey := apiKeysByUUID[apiKeyUUID]
			if apiKey == nil {
				continue
			}

			cfg := cfgByArtifactUUID[apiKey.ArtifactUUID]
			if cfg == nil {
				if cfgErr, missing := missingCfgArtifactUUIDs[apiKey.ArtifactUUID]; missing {
					logger.Debug("Skipping API key xDS refresh due to missing API config",
						slog.String("api_key_uuid", apiKey.UUID),
						slog.String("artifact_uuid", apiKey.ArtifactUUID),
						slog.Any("error", cfgErr),
					)
					continue
				}

				cfgLoaded, cfgErr := c.db.GetConfig(apiKey.ArtifactUUID)
				if cfgErr != nil {
					missingCfgArtifactUUIDs[apiKey.ArtifactUUID] = cfgErr
					logger.Debug("Skipping API key xDS refresh due to missing API config",
						slog.String("api_key_uuid", apiKey.UUID),
						slog.String("artifact_uuid", apiKey.ArtifactUUID),
						slog.Any("error", cfgErr),
					)
					continue
				}

				cfg = cfgLoaded
				cfgByArtifactUUID[apiKey.ArtifactUUID] = cfgLoaded
			}

			if err := c.apiKeyXDSManager.StoreAPIKey(apiKey.ArtifactUUID, cfg.DisplayName, cfg.Version, apiKey, evt.CorrelationID); err != nil {
				logger.Error("Failed to refresh API key xDS state after application mapping update",
					slog.String("api_key_uuid", apiKey.UUID),
					slog.String("artifact_uuid", apiKey.ArtifactUUID),
					slog.Any("error", err),
				)
			}
		}
	}

	logger.Info("Successfully processed application updated event", slog.Int("mapping_count", len(resolvedMappings)))
}

// calculateNextRetryDelay calculates the next retry delay with exponential backoff and jitter
func (c *Client) calculateNextRetryDelay() {
	// Exponential backoff with bounded doubling to avoid overflow before capping.
	baseDelay := c.config.ReconnectInitial
	retries := c.state.RetryCount
	if retries < 0 {
		retries = 0
	}
	for i := 0; i < retries; i++ {
		if baseDelay >= c.config.ReconnectMax {
			baseDelay = c.config.ReconnectMax
			break
		}
		if baseDelay > c.config.ReconnectMax/2 {
			baseDelay = c.config.ReconnectMax
			break
		}
		baseDelay *= 2
	}

	// Cap at maximum
	if baseDelay > c.config.ReconnectMax {
		baseDelay = c.config.ReconnectMax
	}

	// Add jitter (±25%)
	jitter := time.Duration(float64(baseDelay) * 0.25 * (2*float64(time.Now().UnixNano()%100)/100 - 1))
	c.state.NextRetryDelay = baseDelay + jitter

	// Ensure it doesn't go negative or exceed max
	if c.state.NextRetryDelay < 0 {
		c.state.NextRetryDelay = c.config.ReconnectInitial
	}
	if c.state.NextRetryDelay > c.config.ReconnectMax {
		c.state.NextRetryDelay = c.config.ReconnectMax
	}
}

// handleSubscriptionCreatedEvent processes subscription.created events from platform-api.
func (c *Client) handleSubscriptionCreatedEvent(event map[string]interface{}) {
	baseLogger := c.logger
	if baseLogger == nil {
		baseLogger = slog.Default()
	}

	resourceService := c.getSubscriptionResourceService()

	var createdEvent SubscriptionCreatedEvent
	if err := utils.MapToStruct(event, &createdEvent); err != nil {
		baseLogger.Error("Failed to parse subscription.created event", slog.Any("error", err))
		return
	}
	payload := createdEvent.Payload
	if payload.APIID == "" || payload.SubscriptionID == "" || payload.SubscriptionToken == "" {
		baseLogger.Error("subscription.created event missing required fields",
			slog.String("apiId", payload.APIID),
			slog.String("subscriptionId", payload.SubscriptionID),
			slog.Bool("hasToken", payload.SubscriptionToken != ""))
		return
	}
	logger := baseLogger.With(
		slog.String("correlation_id", createdEvent.CorrelationID),
		slog.String("subscription_id", payload.SubscriptionID),
		slog.String("api_id", payload.APIID),
	)

	status := models.SubscriptionStatus(payload.Status)
	sub := &models.Subscription{
		ID:                payload.SubscriptionID,
		APIID:             payload.APIID,
		SubscriptionToken: payload.SubscriptionToken,
		Status:            status,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	if payload.ApplicationID != "" {
		sub.ApplicationID = &payload.ApplicationID
	}
	if payload.SubscriptionPlanId != "" {
		sub.SubscriptionPlanID = &payload.SubscriptionPlanId
	}

	if payload.BillingCustomerID != "" {
		sub.BillingCustomerID = &payload.BillingCustomerID
	}

	if payload.BillingSubscriptionID != "" {
		sub.BillingSubscriptionID = &payload.BillingSubscriptionID
	}

	if err := resourceService.UpsertSubscription(sub, "CREATE", createdEvent.CorrelationID, logger); err != nil {
		logger.Error("Failed to persist subscription from subscription.created event",
			slog.Any("error", err))
		return
	}
}

// handleSubscriptionUpdatedEvent processes subscription.updated events from platform-api.
func (c *Client) handleSubscriptionUpdatedEvent(event map[string]interface{}) {
	baseLogger := c.logger
	if baseLogger == nil {
		baseLogger = slog.Default()
	}

	resourceService := c.getSubscriptionResourceService()

	var updatedEvent SubscriptionUpdatedEvent
	if err := utils.MapToStruct(event, &updatedEvent); err != nil {
		baseLogger.Error("Failed to parse subscription.updated event", slog.Any("error", err))
		return
	}
	payload := updatedEvent.Payload
	if payload.SubscriptionID == "" {
		baseLogger.Error("subscription.updated event missing subscriptionId")
		return
	}
	logger := baseLogger.With(
		slog.String("correlation_id", updatedEvent.CorrelationID),
		slog.String("subscription_id", payload.SubscriptionID),
	)

	existing, err := c.db.GetSubscriptionByID(payload.SubscriptionID, "")
	if err != nil {
		if storage.IsNotFoundError(err) {
			logger.Debug("Subscription not found for subscription.updated event; skipping",
				slog.String("id", payload.SubscriptionID))
			return
		}
		logger.Error("Failed to fetch subscription for update", slog.Any("error", err))
		return
	}
	if existing == nil {
		logger.Debug("Subscription nil for subscription.updated event; skipping",
			slog.String("id", payload.SubscriptionID))
		return
	}

	// Copy all mutable fields from payload into existing before update.
	if payload.APIID != "" {
		existing.APIID = payload.APIID
	}
	if payload.ApplicationID != "" {
		existing.ApplicationID = &payload.ApplicationID
	} else {
		existing.ApplicationID = nil
	}
	if payload.SubscriptionToken != "" {
		existing.SubscriptionToken = payload.SubscriptionToken
	}
	if payload.SubscriptionPlanId != "" {
		existing.SubscriptionPlanID = &payload.SubscriptionPlanId
	} else {
		existing.SubscriptionPlanID = nil
	}
	existing.Status = models.SubscriptionStatus(payload.Status)
	if err := resourceService.UpdateSubscription(existing, updatedEvent.CorrelationID, logger); err != nil {
		logger.Error("Failed to update subscription from subscription.updated event",
			slog.Any("error", err))
		return
	}
}

// handleSubscriptionDeletedEvent processes subscription.deleted events from platform-api.
func (c *Client) handleSubscriptionDeletedEvent(event map[string]interface{}) {
	baseLogger := c.logger
	if baseLogger == nil {
		baseLogger = slog.Default()
	}

	resourceService := c.getSubscriptionResourceService()

	var deletedEvent SubscriptionDeletedEvent
	if err := utils.MapToStruct(event, &deletedEvent); err != nil {
		baseLogger.Error("Failed to parse subscription.deleted event", slog.Any("error", err))
		return
	}
	payload := deletedEvent.Payload
	if payload.SubscriptionID == "" {
		baseLogger.Error("subscription.deleted event missing subscriptionId")
		return
	}
	logger := baseLogger.With(
		slog.String("correlation_id", deletedEvent.CorrelationID),
		slog.String("subscription_id", payload.SubscriptionID),
	)

	if err := resourceService.DeleteSubscription(payload.SubscriptionID, deletedEvent.CorrelationID, logger); err != nil {
		logger.Warn("Failed to delete subscription from subscription.deleted event",
			slog.Any("error", err))
		return
	}
}

// handleSubscriptionPlanCreatedEvent processes subscriptionPlan.created events.
func (c *Client) handleSubscriptionPlanCreatedEvent(event map[string]interface{}) {
	baseLogger := c.logger
	resourceService := c.getSubscriptionResourceService()

	var created SubscriptionPlanCreatedEvent
	if err := utils.MapToStruct(event, &created); err != nil {
		baseLogger.Error("Failed to parse subscriptionPlan.created event", slog.Any("error", err))
		return
	}
	payload := created.Payload
	if payload.PlanId == "" || payload.PlanName == "" {
		baseLogger.Error("subscriptionPlan.created event missing required fields", slog.Any("payload", payload))
		return
	}
	logger := baseLogger.With(
		slog.String("correlation_id", created.CorrelationID),
		slog.String("plan_id", payload.PlanId),
	)

	var billingPlan, throttleLimitUnit *string
	if payload.BillingPlan != "" {
		billingPlan = &payload.BillingPlan
	}
	if payload.ThrottleLimitUnit != "" {
		throttleLimitUnit = &payload.ThrottleLimitUnit
	}
	plan := &models.SubscriptionPlan{
		ID:                 payload.PlanId,
		PlanName:           payload.PlanName,
		BillingPlan:        billingPlan,
		StopOnQuotaReach:   payload.StopOnQuotaReach,
		ThrottleLimitCount: payload.ThrottleLimitCount,
		ThrottleLimitUnit:  throttleLimitUnit,
		ExpiryTime:         payload.ExpiryTime,
		Status:             models.SubscriptionPlanStatus(payload.Status),
	}

	if err := resourceService.UpsertSubscriptionPlan(plan, "CREATE", created.CorrelationID, logger); err != nil {
		logger.Error("Failed to persist subscription plan from subscriptionPlan.created event",
			slog.Any("error", err))
		return
	}
}

// handleSubscriptionPlanUpdatedEvent processes subscriptionPlan.updated events.
func (c *Client) handleSubscriptionPlanUpdatedEvent(event map[string]interface{}) {
	baseLogger := c.logger
	resourceService := c.getSubscriptionResourceService()

	var updated SubscriptionPlanUpdatedEvent
	if err := utils.MapToStruct(event, &updated); err != nil {
		baseLogger.Error("Failed to parse subscriptionPlan.updated event", slog.Any("error", err))
		return
	}
	payload := updated.Payload
	if payload.PlanId == "" {
		baseLogger.Error("subscriptionPlan.updated event missing planId")
		return
	}
	logger := baseLogger.With(
		slog.String("correlation_id", updated.CorrelationID),
		slog.String("plan_id", payload.PlanId),
	)

	existing, err := c.db.GetSubscriptionPlanByID(payload.PlanId, "")
	if err != nil {
		if storage.IsNotFoundError(err) {
			logger.Debug("Subscription plan not found for update; skipping",
				slog.String("planId", payload.PlanId))
			return
		}
		logger.Error("Failed to fetch subscription plan for update", slog.Any("error", err))
		return
	}

	if payload.PlanName != "" {
		existing.PlanName = payload.PlanName
	}
	// BillingPlan and ThrottleLimitUnit: empty string in payload clears (nil); non-empty sets
	var billingPlan, throttleLimitUnit *string
	if payload.BillingPlan != "" {
		bp := payload.BillingPlan
		billingPlan = &bp
	}
	existing.BillingPlan = billingPlan
	existing.StopOnQuotaReach = payload.StopOnQuotaReach
	existing.ThrottleLimitCount = payload.ThrottleLimitCount
	if payload.ThrottleLimitUnit != "" {
		tlu := payload.ThrottleLimitUnit
		throttleLimitUnit = &tlu
	}
	existing.ThrottleLimitUnit = throttleLimitUnit
	existing.ExpiryTime = payload.ExpiryTime
	if payload.Status != "" {
		existing.Status = models.SubscriptionPlanStatus(payload.Status)
	}

	if err := resourceService.UpdateSubscriptionPlan(existing, updated.CorrelationID, logger); err != nil {
		logger.Error("Failed to update subscription plan from subscriptionPlan.updated event",
			slog.Any("error", err))
		return
	}
}

// handleSubscriptionPlanDeletedEvent processes subscriptionPlan.deleted events.
func (c *Client) handleSubscriptionPlanDeletedEvent(event map[string]interface{}) {
	baseLogger := c.logger
	resourceService := c.getSubscriptionResourceService()

	var deleted SubscriptionPlanDeletedEvent
	if err := utils.MapToStruct(event, &deleted); err != nil {
		baseLogger.Error("Failed to parse subscriptionPlan.deleted event", slog.Any("error", err))
		return
	}
	payload := deleted.Payload
	if payload.PlanId == "" {
		baseLogger.Error("subscriptionPlan.deleted event missing planId")
		return
	}
	logger := baseLogger.With(
		slog.String("correlation_id", deleted.CorrelationID),
		slog.String("plan_id", payload.PlanId),
	)

	if err := resourceService.DeleteSubscriptionPlan(payload.PlanId, deleted.CorrelationID, logger); err != nil {
		logger.Warn("Failed to delete subscription plan from subscriptionPlan.deleted event",
			slog.Any("error", err))
		return
	}
}

// setState updates the connection state
func (c *Client) setState(newState State) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	c.setStateNoLock(newState)
}

// setStateNoLock updates the connection state without acquiring the lock
// This should only be called when the caller already holds c.state.mu.Lock()
func (c *Client) setStateNoLock(newState State) {
	oldState := c.state.Current
	c.state.Current = newState

	if oldState != newState {
		c.logger.Info("Connection state changed",
			slog.String("from", oldState.String()),
			slog.String("to", newState.String()),
		)
	}
}

// isShuttingDown checks if the client is shutting down
func (c *Client) isShuttingDown() bool {
	select {
	case <-c.stopChan:
		return true
	case <-c.ctx.Done():
		return true
	default:
		return false
	}
}

// GetState returns the current connection state (thread-safe)
func (c *Client) GetState() State {
	c.state.mu.RLock()
	defer c.state.mu.RUnlock()
	return c.state.Current
}

// IsConnected returns true if the client is currently connected
func (c *Client) IsConnected() bool {
	c.state.mu.RLock()
	defer c.state.mu.RUnlock()
	return c.state.Current == Connected && c.state.Conn != nil
}

// PushAPIDeployment pushes API deployment details to the control plane
func (c *Client) PushAPIDeployment(apiID string, apiConfig *models.StoredConfig, deploymentID string) error {
	// Check if connected to control plane
	if !c.IsConnected() {
		c.logger.Debug("Not connected to control plane, skipping API deployment push",
			slog.String("api_id", apiID))
		return nil
	}

	return c.apiUtilsService.PushAPIDeployment(apiID, apiConfig, deploymentID)
}

// getWebSocketURL constructs the base WebSocket URL from configuration (cloud default; on-prem may override via well-known).
func (c *Client) getWebSocketURL() string {
	return fmt.Sprintf("wss://%s/api/internal/v1/ws", c.config.Host)
}

// getWebSocketConnectURL returns the full WebSocket URL for gateway connect (fallback when well-known is unavailable).
func (c *Client) getWebSocketConnectURL() string {
	return c.getWebSocketURL() + "/gateways/connect"
}

// getRestAPIBaseURL constructs the base REST API URL from configuration.
// Uses the discovered gateway path if available, otherwise falls back to default.
func (c *Client) getRestAPIBaseURL() string {
	c.state.mu.RLock()
	path := c.gatewayPath
	c.state.mu.RUnlock()

	if path != "" {
		return fmt.Sprintf("https://%s%s", c.config.Host, path)
	}
	return fmt.Sprintf("https://%s/api/internal/v1", c.config.Host)
}

// pushGatewayManifest POSTs the gateway's installed policy manifest to the control plane.
func (c *Client) pushGatewayManifest(gatewayID string, policies []models.PolicyDefinition) error {
	url := c.getRestAPIBaseURL() + "/gateways/" + gatewayID + "/manifest"

	body := struct {
		Policies []models.PolicyDefinition `json:"policies"`
	}{Policies: policies}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest payload: %w", err)
	}

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{ // #nosec G402 -- Explicit operator-controlled opt-out for dev/test environments.
				InsecureSkipVerify: c.config.InsecureSkipVerify,
			},
		},
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create manifest request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", c.config.Token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send gateway manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gateway manifest push failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

// pushGatewayManifestOnConnect collects all loaded policy definitions and POSTs
// them to the control plane immediately after the connection is established.
func (c *Client) pushGatewayManifestOnConnect(gatewayID string) {
	// Skip for on-prem control planes
	if c.isOnPrem() {
		c.logger.Debug("Skipping gateway manifest push: on-prem control plane detected",
			slog.String("gateway_id", gatewayID),
		)
		return
	}

	c.logger.Info("Pushing gateway manifest on connect",
		slog.String("gateway_id", gatewayID),
	)

	policies := make([]models.PolicyDefinition, 0, len(c.policyDefinitions))
	for _, def := range c.policyDefinitions {
		if strings.HasPrefix(def.Name, "wso2_apip_sys_") {
			// Skip internal system policies
			continue
		}
		policies = append(policies, def)
	}

	const maxRetries = 3
	var pushErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		pushErr = c.pushGatewayManifest(gatewayID, policies)
		if pushErr == nil {
			break
		}
		c.logger.Warn("Failed to push gateway manifest, retrying",
			slog.String("gateway_id", gatewayID),
			slog.Int("attempt", attempt),
			slog.Int("max_retries", maxRetries),
			slog.Any("error", pushErr),
		)
		if attempt < maxRetries {
			select {
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			case <-c.ctx.Done():
				c.logger.Warn("Context cancelled, aborting gateway manifest push retries",
					slog.String("gateway_id", gatewayID))
				return
			}
		}
	}
	if pushErr != nil {
		c.logger.Error("Failed to push gateway manifest after all retries",
			slog.String("gateway_id", gatewayID),
			slog.Any("error", pushErr),
		)
		return
	}

	c.logger.Info("Successfully pushed gateway manifest to control plane",
		slog.String("gateway_id", gatewayID),
		slog.Int("policy_count", len(policies)),
	)
}
