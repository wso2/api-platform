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
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	"go.uber.org/zap"
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

// Client manages the WebSocket connection to the control plane
type Client struct {
	config            config.ControlPlaneConfig
	logger            *zap.Logger
	state             *ConnectionState
	ctx               context.Context
	cancel            context.CancelFunc
	stopChan          chan struct{}
	wg                sync.WaitGroup
	store             *storage.ConfigStore
	db                storage.Storage
	snapshotManager   *xds.SnapshotManager
	parser            *config.Parser
	validator         *config.Validator
	deploymentService *utils.APIDeploymentService
	apiUtilsService   *utils.APIUtilsService
}

// NewClient creates a new control plane client
func NewClient(
	cfg config.ControlPlaneConfig,
	logger *zap.Logger,
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		config:            cfg,
		logger:            logger,
		store:             store,
		db:                db,
		snapshotManager:   snapshotManager,
		parser:            config.NewParser(),
		validator:         config.NewValidator(),
		deploymentService: utils.NewAPIDeploymentService(store, db, snapshotManager),
		apiUtilsService: utils.NewAPIUtilsService(utils.APIFetchConfig{
			BaseURL:            fmt.Sprintf("https://%s/api/internal/v1", cfg.Host),
			Token:              cfg.Token,
			InsecureSkipVerify: cfg.InsecureSkipVerify,
			Timeout:            30 * time.Second,
		}, logger),
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
}

// Start initiates the connection to the control plane
func (c *Client) Start() error {
	// Check if token is configured
	if c.config.Token == "" {
		c.logger.Info("Control plane token not configured, skipping connection")
		return nil
	}

	c.logger.Info("Starting control plane client",
		zap.String("host", c.config.Host),
		zap.String("websocket_url", c.getWebSocketURL()),
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
		zap.String("url", c.getWebSocketURL()),
		zap.Int("retry_count", c.state.RetryCount),
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
				zap.Error(err),
				zap.Int("status_code", resp.StatusCode),
			)

			// Handle authentication failures
			if resp.StatusCode == http.StatusUnauthorized {
				c.logger.Error("Authentication failed - invalid or revoked token",
					zap.String("troubleshooting", "Check GATEWAY_REGISTRATION_TOKEN environment variable"),
				)
				return fmt.Errorf("authentication failed: %w", err)
			}
		} else {
			c.logger.Error("WebSocket connection failed",
				zap.Error(err),
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
		zap.String("gateway_id", c.state.GatewayID),
		zap.String("connection_id", c.state.ConnectionID),
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
		zap.String("gateway_id", ack.GatewayID),
		zap.String("connection_id", ack.ConnectionID),
		zap.String("timestamp", ack.Timestamp),
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
		c.setState(Disconnected)

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
					zap.Duration("time_since_last_heartbeat", timeSinceLastHeartbeat),
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
				zap.Error(err),
				zap.Duration("retry_delay", c.state.NextRetryDelay),
				zap.Int("retry_count", c.state.RetryCount),
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
				zap.Error(err),
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
		zap.Int("message_type", messageType),
		zap.Int("message_length", len(message)),
	)

	// Only process text messages (JSON events)
	if messageType != websocket.TextMessage {
		c.logger.Debug("Ignoring non-text message",
			zap.Int("message_type", messageType),
		)
		return
	}

	// Parse as generic event to extract type
	var event map[string]interface{}
	if err := json.Unmarshal(message, &event); err != nil {
		c.logger.Error("Failed to parse WebSocket message",
			zap.Error(err),
			zap.String("message", string(message)),
		)
		return
	}

	// Extract event type
	eventType, ok := event["type"].(string)
	if !ok {
		c.logger.Warn("Message missing 'type' field",
			zap.String("message", string(message)),
		)
		return
	}

	// Log the event to console
	c.logger.Info("Received WebSocket event",
		zap.String("type", eventType),
		zap.String("payload", string(message)),
	)

	// Handle specific event types
	switch eventType {
	case "connection.ack":
		// Already handled during connection establishment
		c.logger.Debug("Received connection.ack (already processed)")
	case "api.deployed":
		c.handleAPIDeployedEvent(event)
	case "api.undeployed":
		c.handleAPIUndeployedEvent(event)
	default:
		c.logger.Info("Received unknown event type (will be processed when handlers are implemented)",
			zap.String("type", eventType),
		)
	}
}

// handleAPIDeployedEvent handles API deployment events
func (c *Client) handleAPIDeployedEvent(event map[string]interface{}) {
	c.logger.Info("API Deployment Event",
		zap.Any("payload", event["payload"]),
		zap.Any("timestamp", event["timestamp"]),
		zap.Any("correlationId", event["correlationId"]),
	)

	// Parse the event into structured format
	eventBytes, err := json.Marshal(event)
	if err != nil {
		c.logger.Error("Failed to marshal event for parsing",
			zap.Error(err),
		)
		return
	}

	var deployedEvent APIDeployedEvent
	if err := json.Unmarshal(eventBytes, &deployedEvent); err != nil {
		c.logger.Error("Failed to parse API deployment event",
			zap.Error(err),
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
		zap.String("api_id", apiID),
		zap.String("environment", deployedEvent.Payload.Environment),
		zap.String("revision_id", deployedEvent.Payload.RevisionID),
		zap.String("vhost", deployedEvent.Payload.VHost),
		zap.String("correlation_id", deployedEvent.CorrelationID),
	)

	// Fetch API definition from control plane
	zipData, err := c.apiUtilsService.FetchAPIDefinition(apiID)
	if err != nil {
		c.logger.Error("Failed to fetch API definition",
			zap.String("api_id", apiID),
			zap.Error(err),
		)
		return
	}

	// Extract YAML directly from zip file in memory (no need to save to disk)
	yamlData, err := c.apiUtilsService.ExtractYAMLFromZip(zipData)
	if err != nil {
		c.logger.Error("Failed to extract YAML from zip",
			zap.String("api_id", apiID),
			zap.Error(err),
		)
		return
	}

	// log the yaml for debugging
	c.logger.Debug("Extracted YAML data",
		zap.String("api_id", apiID),
		zap.String("yaml_data", string(yamlData)),
	)

	// Create API configuration from YAML using the deployment service
	if err := c.apiUtilsService.CreateAPIFromYAML(yamlData, apiID, deployedEvent.CorrelationID, c.deploymentService); err != nil {
		c.logger.Error("Failed to create API from YAML",
			zap.String("api_id", apiID),
			zap.Error(err),
		)
		return
	}

	c.logger.Info("Successfully processed API deployment event",
		zap.String("api_id", apiID),
		zap.String("correlation_id", deployedEvent.CorrelationID),
	)
}

// handleAPIUndeployedEvent handles API undeployment events
func (c *Client) handleAPIUndeployedEvent(event map[string]interface{}) {
	c.logger.Info("API Undeployment Event",
		zap.Any("payload", event["payload"]),
		zap.Any("timestamp", event["timestamp"]),
		zap.Any("correlationId", event["correlationId"]),
	)
	// TODO: Implement actual API undeployment logic in Phase 6
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
	oldState := c.state.Current
	c.state.Current = newState
	c.state.mu.Unlock()

	if oldState != newState {
		c.logger.Info("Connection state changed",
			zap.String("from", oldState.String()),
			zap.String("to", newState.String()),
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
	return c.GetState() == Connected
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
