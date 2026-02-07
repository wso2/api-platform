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
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"

	"github.com/gorilla/websocket"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
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
	NotifyAPIDeployment(apiID string, apiConfig *models.StoredConfig, revisionID string) error
}

// Client manages the WebSocket connection to the control plane
type Client struct {
	config            config.ControlPlaneConfig
	logger            *slog.Logger
	state             *ConnectionState
	ctx               context.Context
	cancel            context.CancelFunc
	stopChan          chan struct{}
	wg                sync.WaitGroup
	store             *storage.ConfigStore
	db                storage.Storage
	snapshotManager   *xds.SnapshotManager
	parser            *config.Parser
	validator         config.Validator
	deploymentService *utils.APIDeploymentService
	apiUtilsService   *utils.APIUtilsService
	apiKeyService     *utils.APIKeyService
	routerConfig      *config.RouterConfig
	policyManager     *policyxds.PolicyManager
	systemConfig      *config.Config
	policyDefinitions map[string]api.PolicyDefinition
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
	apiKeyConfig *config.APIKeyConfig,
	policyManager *policyxds.PolicyManager,
	systemConfig *config.Config,
	policyDefinitions map[string]api.PolicyDefinition,
) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	client := &Client{
		config:            cfg,
		logger:            logger,
		store:             store,
		db:                db,
		snapshotManager:   snapshotManager,
		parser:            config.NewParser(),
		validator:         validator,
		deploymentService: utils.NewAPIDeploymentService(store, db, snapshotManager, validator, routerConfig),
		apiKeyService:     utils.NewAPIKeyService(store, db, apiKeyXDSManager, apiKeyConfig),
		routerConfig:      routerConfig,
		policyManager:     policyManager,
		systemConfig:      systemConfig,
		policyDefinitions: policyDefinitions,
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

	// Initialize API utils service with the proper base URL using the method
	client.apiUtilsService = utils.NewAPIUtilsService(utils.PlatformAPIConfig{
		BaseURL:            client.getRestAPIBaseURL(),
		Token:              cfg.Token,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		Timeout:            30 * time.Second,
	}, logger)

	return client
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
		slog.String("websocket_url", c.getWebSocketURL()),
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

	// Close active connection if exists
	c.state.mu.Lock()
	if c.state.Conn != nil {
		// Send close frame with normal closure code
		closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Gateway shutting down")
		_ = c.state.Conn.WriteMessage(websocket.CloseMessage, closeMsg)
		_ = c.state.Conn.Close()
		c.state.Conn = nil
	}
	c.state.mu.Unlock()

	// Wait for goroutines to finish
	c.wg.Wait()

	c.logger.Info("Control plane client stopped")
}

// Connect establishes a WebSocket connection to the control plane
func (c *Client) Connect() error {
	c.setState(Connecting)

	c.logger.Info("Connecting to control plane",
		slog.String("url", c.getWebSocketURL()),
		slog.Int("retry_count", c.state.RetryCount),
	)

	// Create WebSocket dialer with timeout
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: c.config.InsecureSkipVerify,
		},
	}

	// Log TLS configuration
	if c.config.InsecureSkipVerify {
		c.logger.Debug("TLS certificate verification disabled (insecure_skip_verify=true)")
	}

	// Add api-key header for authentication
	headers := http.Header{}
	headers.Add("api-key", c.config.Token)

	// Dial WebSocket
	wsURL := c.getWebSocketURL() + "/gateways/connect"
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
		conn.Close()
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

	// Start heartbeat monitor
	c.wg.Add(1)
	go c.heartbeatMonitor()

	return nil
}

// waitForConnectionAck waits for the connection.ack message from the server
func (c *Client) waitForConnectionAck(conn *websocket.Conn) error {
	// Set read deadline for ack message
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetReadDeadline(time.Time{}) // Clear deadline

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

// Close closes the WebSocket connection
func (c *Client) Close() error {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()

	if c.state.Conn != nil {
		// Send close frame with normal closure code
		closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Client closing connection")
		_ = c.state.Conn.WriteMessage(websocket.CloseMessage, closeMsg)

		err := c.state.Conn.Close()
		c.state.Conn = nil
		c.setStateNoLock(Disconnected)

		return err
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
					c.state.Conn.Close()
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
				c.state.Conn.Close()
				c.state.Conn = nil
			}
			c.state.mu.Unlock()

			break
		}

		// Process received message
		c.handleMessage(messageType, message)
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

	// Log the event to console (skip payload for API key events with sensitive data)
	isSensitiveEvent := eventType == "apikey.created" || eventType == "apikey.updated" || eventType == "apikey.revoked"
	if isSensitiveEvent {
		c.logger.Info("Received WebSocket event",
			slog.String("type", eventType),
			slog.String("payload", "[REDACTED - contains sensitive API key data]"),
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
	case "apikey.created":
		c.handleAPIKeyCreatedEvent(event)
	case "apikey.updated":
		c.handleAPIKeyUpdatedEvent(event)
	case "apikey.revoked":
		c.handleAPIKeyRevokedEvent(event)
	default:
		c.logger.Info("Received unknown event type (will be processed when handlers are implemented)",
			slog.String("type", eventType),
		)
	}
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
		slog.String("environment", deployedEvent.Payload.Environment),
		slog.String("revision_id", deployedEvent.Payload.RevisionID),
		slog.String("vhost", deployedEvent.Payload.VHost),
		slog.String("correlation_id", deployedEvent.CorrelationID),
	)

	// Fetch API definition from control plane
	zipData, err := c.apiUtilsService.FetchAPIDefinition(apiID)
	if err != nil {
		c.logger.Error("Failed to fetch API definition",
			slog.String("api_id", apiID),
			slog.Any("error", err),
		)
		return
	}

	// Extract YAML directly from zip file in memory (no need to save to disk)
	yamlData, err := c.apiUtilsService.ExtractYAMLFromZip(zipData)
	if err != nil {
		c.logger.Error("Failed to extract YAML from zip",
			slog.String("api_id", apiID),
			slog.Any("error", err),
		)
		return
	}

	// log the yaml for debugging
	c.logger.Debug("Extracted YAML data",
		slog.String("api_id", apiID),
		slog.String("yaml_data", string(yamlData)),
	)

	// Create API configuration from YAML using the deployment service
	result, err := c.apiUtilsService.CreateAPIFromYAML(yamlData, apiID, deployedEvent.CorrelationID, c.deploymentService)
	if err != nil {
		c.logger.Error("Failed to create API from YAML",
			slog.String("api_id", apiID),
			slog.Any("error", err),
		)
		return
	}

	if c.policyManager == nil {
		c.logger.Error("Failed to update policy engine snapshot: policy manager is not available",
			slog.String("api_id", apiID),
			slog.String("correlation_id", deployedEvent.CorrelationID),
		)
		return
	}

	// Update policy engine xDS snapshot
	if c.policyManager != nil && result != nil {
		var storedPolicy *models.StoredPolicyConfig

		// Guard against nil systemConfig before deriving policies
		if c.systemConfig != nil {
			storedPolicy = utils.DerivePolicyFromAPIConfig(result.StoredConfig, c.routerConfig, c.systemConfig, c.policyDefinitions)
		} else {
			c.logger.Warn("Cannot derive policies: systemConfig is nil",
				slog.String("api_id", apiID),
				slog.String("correlation_id", deployedEvent.CorrelationID))
		}

		if storedPolicy != nil {
			if err := c.policyManager.AddPolicy(storedPolicy); err != nil {
				c.logger.Error("Failed to update policy engine snapshot",
					slog.Any("error", err),
					slog.String("api_id", apiID),
					slog.String("correlation_id", deployedEvent.CorrelationID))
			} else {
				c.logger.Info("Successfully updated policy engine snapshot",
					slog.String("api_id", apiID),
					slog.String("policy_id", storedPolicy.ID))
			}
		} else if result.IsUpdate {
			// No policies but this is an update, so remove any existing policies
			if err := c.policyManager.RemovePolicy(result.StoredConfig.ID + "-policies"); err != nil {
				c.logger.Error("Failed to remove policy from policy engine",
					slog.Any("error", err),
					slog.String("api_id", apiID),
					slog.String("correlation_id", deployedEvent.CorrelationID))
			}
		}
	}

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
	// TODO: Implement actual API undeployment logic in Phase 6
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
	if keyCreatedEvent.Payload.ApiKey == "" {
		baseLogger.Error("API key created event missing required api_key",
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

	// Validate DisplayName - optional field (pointer may be nil)
	if keyCreatedEvent.Payload.DisplayName != nil && strings.TrimSpace(*keyCreatedEvent.Payload.DisplayName) != "" {
		// Validate the display name format
		if err := utils.ValidateDisplayName(*keyCreatedEvent.Payload.DisplayName); err != nil {
			baseLogger.Error("API key created event has invalid display_name",
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


	apiKeyCreationRequest := api.APIKeyCreationRequest{
		ApiKey:        &payload.ApiKey,
		DisplayName:   payload.DisplayName,
		Name:          &payload.Name,
		ExternalRefId: payload.ExternalRefId,
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
		keyCreatedEvent.CorrelationID,
		logger,
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
	if payload.ApiKey == "" {
		baseLogger.Error("API key updated event missing required api_key",
			slog.Any("correlation_id", event["correlationId"]),
			slog.String("api_id", payload.ApiId),
			slog.String("key_name", payload.KeyName),
		)
		return
	}
	if payload.DisplayName == "" {
		baseLogger.Error("API key updated event missing required display_name",
			slog.Any("correlation_id", event["correlationId"]),
			slog.String("api_id", payload.ApiId),
			slog.String("key_name", payload.KeyName),
		)
		return
	}

	// Validate the display name format
	if err := utils.ValidateDisplayName(payload.DisplayName); err != nil {
		baseLogger.Error("API key updated event has invalid display_name",
			slog.Any("correlation_id", event["correlationId"]),
			slog.String("api_id", payload.ApiId),
			slog.String("key_name", payload.KeyName),
			slog.Any("error", err),
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

	apiKeyCreationRequest := api.APIKeyCreationRequest{
		ApiKey:        &payload.ApiKey,
		DisplayName:   &payload.DisplayName,
		ExternalRefId: &payload.ExternalRefId,
		Name:          &payload.KeyName,
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
		evt.UserId,
		evt.CorrelationID,
		logger,
	)
	if err != nil {
		logger.Error("Failed to update external API key", slog.Any("error", err))
		return
	}
	logger.Info("Successfully processed API key updated event")
}

// calculateNextRetryDelay calculates the next retry delay with exponential backoff and jitter
func (c *Client) calculateNextRetryDelay() {
	// Exponential backoff: initial * 2^retries
	baseDelay := c.config.ReconnectInitial * time.Duration(1<<uint(c.state.RetryCount))

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

// NotifyAPIDeployment sends a REST API call to platform-api when an API is deployed successfully
func (c *Client) NotifyAPIDeployment(apiID string, apiConfig *models.StoredConfig, revisionID string) error {
	// Check if connected to control plane
	if !c.IsConnected() {
		c.logger.Debug("Not connected to control plane, skipping API deployment notification",
			slog.String("api_id", apiID))
		return nil
	}

	// Use the api utils service to send the deployment notification
	return c.apiUtilsService.NotifyAPIDeployment(apiID, apiConfig, revisionID)
}

// getWebSocketURL constructs the base WebSocket URL from configuration
func (c *Client) getWebSocketURL() string {
	return fmt.Sprintf("wss://%s/api/internal/v1/ws",
		c.config.Host,
	)
}

// getRestAPIBaseURL constructs the base REST API URL from configuration
func (c *Client) getRestAPIBaseURL() string {
	return fmt.Sprintf("https://%s/api/internal/v1",
		c.config.Host,
	)
}
