/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"
	ws "platform-api/src/internal/websocket"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WebSocketHandler handles WebSocket connection upgrades and lifecycle
type WebSocketHandler struct {
	manager           *ws.Manager
	gatewayService    *service.GatewayService
	deploymentService *service.DeploymentService
	upgrader          websocket.Upgrader
	slogger           *slog.Logger

	// Rate limiting: track connection attempts per IP
	rateLimitMu    sync.RWMutex
	rateLimitMap   map[string][]time.Time // IP -> timestamps of connection attempts
	rateLimitCount int                    // Attempts allowed per minute
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(manager *ws.Manager, gatewayService *service.GatewayService, deploymentService *service.DeploymentService, rateLimitCount int, slogger *slog.Logger) *WebSocketHandler {
	return &WebSocketHandler{
		manager:           manager,
		gatewayService:    gatewayService,
		deploymentService: deploymentService,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// TODO: Implement proper origin checking in production
				return true
			},
			HandshakeTimeout: 10 * time.Second,
		},
		slogger:        slogger,
		rateLimitMap:   make(map[string][]time.Time),
		rateLimitCount: rateLimitCount,
	}
}

// Connect handles WebSocket upgrade requests at /api/internal/v1/ws/gateways/connect
// This is the entry point for gateway connections.
func (h *WebSocketHandler) Connect(c *gin.Context) {
	// Extract client IP for rate limiting
	clientIP := c.ClientIP()

	// Check rate limit
	if !h.checkRateLimit(clientIP) {
		h.slogger.Warn("Rate limit exceeded for IP", "ip", clientIP)
		h.manager.IncrementFailedConnections()
		c.JSON(http.StatusTooManyRequests, utils.NewErrorResponse(429, "Too Many Requests",
			"Connection rate limit exceeded. Please try again later."))
		return
	}

	// Extract and validate API key from header
	apiKey := c.GetHeader("api-key")
	if apiKey == "" {
		h.slogger.Warn("WebSocket connection attempt without API key", "ip", clientIP)
		h.manager.IncrementFailedConnections()
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"API key is required. Provide 'api-key' header."))
		return
	}

	// Authenticate gateway using API key
	gateway, err := h.gatewayService.VerifyToken(apiKey)
	if err != nil {
		h.slogger.Warn("WebSocket authentication failed", "ip", clientIP, "error", err)
		h.manager.IncrementFailedConnections()
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Invalid or expired API key"))
		return
	}

	// Check organization connection limit before upgrading to WebSocket
	if !h.manager.CanAcceptOrgConnection(gateway.OrganizationID) {
		stats := h.manager.GetOrgConnectionStats(gateway.OrganizationID)
		h.slogger.Warn("Organization connection limit exceeded", "orgID", gateway.OrganizationID,
			"count", stats.CurrentCount, "max", stats.MaxAllowed)
		h.manager.IncrementFailedConnections()
		c.JSON(http.StatusTooManyRequests, utils.NewErrorResponse(429, "Too Many Requests",
			"Organization connection limit reached. Maximum allowed connections: "+
				fmt.Sprintf("%d", stats.MaxAllowed)))
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.slogger.Error("WebSocket upgrade failed", "gatewayID", gateway.ID, "error", err)
		// Upgrade error is already sent by upgrader
		return
	}

	// Create WebSocket transport
	transport := ws.NewWebSocketTransport(conn)

	// Register connection with manager
	connection, err := h.manager.Register(gateway.ID, transport, apiKey, gateway.OrganizationID)
	if err != nil {
		h.slogger.Error("Connection registration failed", "gatewayID", gateway.ID, "orgID", gateway.OrganizationID, "error", err)
		h.manager.IncrementFailedConnections()

		// Check if this is an org connection limit error
		if orgLimitErr, ok := err.(*ws.OrgConnectionLimitError); ok {
			errorMsg := map[string]interface{}{
				"type":         "error",
				"code":         "ORG_CONNECTION_LIMIT_EXCEEDED",
				"message":      "Organization connection limit reached",
				"currentCount": orgLimitErr.CurrentCount,
				"maxAllowed":   orgLimitErr.MaxAllowed,
			}
			if jsonErr, _ := json.Marshal(errorMsg); jsonErr != nil {
				conn.WriteMessage(websocket.TextMessage, jsonErr)
			}
			h.slogger.Warn("Organization connection limit exceeded", "orgID", orgLimitErr.OrganizationID,
				"count", orgLimitErr.CurrentCount, "max", orgLimitErr.MaxAllowed)
		} else {
			// Generic error
			errorMsg := map[string]string{
				"type":    "error",
				"message": err.Error(),
			}
			if jsonErr, _ := json.Marshal(errorMsg); jsonErr != nil {
				conn.WriteMessage(websocket.TextMessage, jsonErr)
			}
		}
		conn.Close()
		return
	}

	// Send connection acknowledgment
	ack := dto.ConnectionAckDTO{
		Type:         "connection.ack",
		GatewayID:    gateway.ID,
		ConnectionID: connection.ConnectionID,
		Timestamp:    time.Now().Format(time.RFC3339),
	}

	ackJSON, err := json.Marshal(ack)
	if err != nil {
		h.slogger.Error("Failed to marshal connection ACK", "gatewayID", gateway.ID, "error", err)
	} else {
		if err := connection.Send(ackJSON); err != nil {
			h.slogger.Error("Failed to send connection ACK", "gatewayID", gateway.ID, "connectionID", connection.ConnectionID, "error", err)
		}
	}

	h.slogger.Info("WebSocket connection established", "gatewayID", gateway.ID, "connectionID", connection.ConnectionID)

	// Update gateway active status to true when connection is established
	if err := h.gatewayService.UpdateGatewayActiveStatus(gateway.ID, true); err != nil {
		h.slogger.Error("Failed to update gateway active status to true", "gatewayID", gateway.ID, "error", err)
	}

	// Start reading messages (blocks until connection closes)
	// This keeps the handler goroutine alive to maintain the connection
	h.readLoop(connection)

	// Connection closed - cleanup
	h.slogger.Info("WebSocket connection closed", "gatewayID", gateway.ID, "connectionID", connection.ConnectionID)
	h.manager.Unregister(gateway.ID, connection.ConnectionID)

	// Only set inactive if no remaining connections for this gateway
	if len(h.manager.GetConnections(gateway.ID)) == 0 {
		if err := h.gatewayService.UpdateGatewayActiveStatus(gateway.ID, false); err != nil {
			h.slogger.Error("Failed to update gateway active status to false", "gatewayID", gateway.ID, "error", err)
		}
	}
}

// readLoop reads messages from the WebSocket connection and routes them to handlers.
func (h *WebSocketHandler) readLoop(conn *ws.Connection) {
	defer func() {
		if r := recover(); r != nil {
			h.slogger.Error("Panic in WebSocket read loop", "gatewayID", conn.GatewayID, "connectionID", conn.ConnectionID, "panic", r)
		}
	}()

	for {
		if conn.IsClosed() {
			return
		}

		wsTransport, ok := conn.Transport.(*ws.WebSocketTransport)
		if !ok {
			h.slogger.Error("Invalid transport type for connection", "gatewayID", conn.GatewayID, "connectionID", conn.ConnectionID)
			return
		}

		_, message, err := wsTransport.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				h.slogger.Error("WebSocket read error", "gatewayID", conn.GatewayID, "connectionID", conn.ConnectionID, "error", err)
			}
			return
		}

		// Parse message type
		var msg struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(message, &msg); err != nil {
			h.slogger.Warn("Failed to parse incoming WebSocket message",
				"gatewayID", conn.GatewayID, "connectionID", conn.ConnectionID, "error", err)
			continue
		}

		switch msg.Type {
		case "deployment.ack":
			h.handleDeploymentAck(conn, msg.Payload)
		default:
			h.slogger.Debug("Received unknown message type from gateway",
				"gatewayID", conn.GatewayID, "type", msg.Type)
		}
	}
}

// handleDeploymentAck processes a deployment acknowledgement message from the gateway
func (h *WebSocketHandler) handleDeploymentAck(conn *ws.Connection, payload json.RawMessage) {
	var ack model.DeploymentAckPayload
	if err := json.Unmarshal(payload, &ack); err != nil {
		h.slogger.Error("Failed to parse deployment.ack payload",
			"gatewayID", conn.GatewayID, "connectionID", conn.ConnectionID, "error", err)
		return
	}

	h.slogger.Info("Received deployment.ack",
		"gatewayID", conn.GatewayID, "artifactID", ack.ArtifactID,
		"deploymentID", ack.DeploymentID, "action", ack.Action,
		"status", ack.Status, "performedAt", ack.PerformedAt)

	if h.deploymentService == nil {
		h.slogger.Error("DeploymentService not available for ack handling",
			"gatewayID", conn.GatewayID)
		return
	}

	if err := h.deploymentService.HandleDeploymentAck(conn.GatewayID, conn.OrganizationID, &ack); err != nil {
		h.slogger.Error("Failed to handle deployment ack",
			"gatewayID", conn.GatewayID, "error", err)
	}
}

// checkRateLimit verifies if the client IP is within rate limits.
// Returns true if connection is allowed, false if rate limit exceeded.
//
// Rate limit: rateLimitCount connections per minute per IP
func (h *WebSocketHandler) checkRateLimit(clientIP string) bool {
	h.rateLimitMu.Lock()
	defer h.rateLimitMu.Unlock()

	now := time.Now()
	oneMinuteAgo := now.Add(-1 * time.Minute)

	// Get recent connection attempts for this IP
	attempts, exists := h.rateLimitMap[clientIP]
	if !exists {
		attempts = []time.Time{}
	}

	// Filter out attempts older than 1 minute
	var recentAttempts []time.Time
	for _, t := range attempts {
		if t.After(oneMinuteAgo) {
			recentAttempts = append(recentAttempts, t)
		}
	}

	// Check if rate limit exceeded
	if len(recentAttempts) >= h.rateLimitCount {
		return false // Rate limit exceeded
	}

	// Add current attempt
	recentAttempts = append(recentAttempts, now)
	h.rateLimitMap[clientIP] = recentAttempts

	return true // Connection allowed
}

// RegisterRoutes registers WebSocket routes with the router
func (h *WebSocketHandler) RegisterRoutes(r *gin.Engine) {
	wsGroup := r.Group("/api/internal/v1/ws/gateways")
	{
		wsGroup.GET("/connect", h.Connect)
	}
}
