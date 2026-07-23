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
	"strings"
	"sync"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/service"
	ws "github.com/wso2/api-platform/platform-api/internal/websocket"

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
func (h *WebSocketHandler) Connect(w http.ResponseWriter, r *http.Request) error {
	// Extract client IP for rate limiting
	clientIP := r.RemoteAddr
	if i := strings.LastIndex(clientIP, ":"); i != -1 {
		clientIP = clientIP[:i]
	}

	// Check rate limit
	if !h.checkRateLimit(clientIP) {
		h.manager.IncrementFailedConnections()
		return apperror.TooManyRequests.New("Connection rate limit exceeded. Please try again later.").
			WithLogMessage(fmt.Sprintf("rate limit exceeded for IP %s", clientIP))
	}

	// Extract and validate API key from header. Per the unified auth-failure rule, a missing key
	// and an invalid key both surface the identical generic response; the specific reason is
	// internal-only via WithLogMessage.
	apiKey := r.Header.Get("api-key")
	if apiKey == "" {
		h.manager.IncrementFailedConnections()
		return apperror.Unauthorized.New().
			WithLogMessage(fmt.Sprintf("WebSocket connection attempt without API key from IP %s", clientIP))
	}

	// Authenticate gateway using API key
	gateway, err := h.gatewayService.VerifyToken(apiKey)
	if err != nil {
		h.manager.IncrementFailedConnections()
		return apperror.Unauthorized.Wrap(err).
			WithLogMessage(fmt.Sprintf("WebSocket authentication failed from IP %s", clientIP))
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.slogger.Error("WebSocket upgrade failed", "gatewayID", gateway.ID, "error", err)
		// Upgrade error response is already written to w by the upgrader itself.
		return nil
	}

	// Create WebSocket transport
	transport := ws.NewWebSocketTransport(conn)

	// Register connection with manager
	connection, err := h.manager.Register(gateway.ID, transport, apiKey, gateway.OrganizationID)
	if err != nil {
		h.slogger.Error("Connection registration failed", "gatewayID", gateway.ID, "orgID", gateway.OrganizationID, "error", err)
		h.manager.IncrementFailedConnections()

		errorMsg := map[string]string{
			"type":    "error",
			"message": err.Error(),
		}
		if jsonErr, _ := json.Marshal(errorMsg); jsonErr != nil {
			conn.WriteMessage(websocket.TextMessage, jsonErr)
		}
		conn.Close()
		return nil
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
	return nil
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

// RegisterRoutes registers WebSocket routes with the mux.
func (h *WebSocketHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/internal/v1/ws/gateways/connect", middleware.MapErrors(h.slogger, h.Connect))
}
