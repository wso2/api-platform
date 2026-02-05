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
	"log"
	"net/http"
	"sync"
	"time"

	"platform-api/src/internal/dto"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"
	ws "platform-api/src/internal/websocket"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WebSocketHandler handles WebSocket connection upgrades and lifecycle
type WebSocketHandler struct {
	manager        *ws.Manager
	gatewayService *service.GatewayService
	upgrader       websocket.Upgrader

	// Rate limiting: track connection attempts per IP
	rateLimitMu    sync.RWMutex
	rateLimitMap   map[string][]time.Time // IP -> timestamps of connection attempts
	rateLimitCount int                    // Attempts allowed per minute
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(manager *ws.Manager, gatewayService *service.GatewayService, rateLimitCount int) *WebSocketHandler {
	return &WebSocketHandler{
		manager:        manager,
		gatewayService: gatewayService,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// TODO: Implement proper origin checking in production
				return true
			},
			HandshakeTimeout: 10 * time.Second,
		},
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
		log.Printf("[WARN] Rate limit exceeded for IP: %s", clientIP)
		c.JSON(http.StatusTooManyRequests, utils.NewErrorResponse(429, "Too Many Requests",
			"Connection rate limit exceeded. Please try again later."))
		return
	}

	// Extract and validate API key from header
	apiKey := c.GetHeader("api-key")
	if apiKey == "" {
		log.Printf("[WARN] WebSocket connection attempt without API key from IP: %s", clientIP)
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"API key is required. Provide 'api-key' header."))
		return
	}

	// Authenticate gateway using API key
	gateway, err := h.gatewayService.VerifyToken(apiKey)
	if err != nil {
		log.Printf("[WARN] WebSocket authentication failed: ip=%s error=%v", clientIP, err)
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Invalid or expired API key"))
		return
	}

	// Check organization connection limit before upgrading to WebSocket
	if !h.manager.CanAcceptOrgConnection(gateway.OrganizationID) {
		stats := h.manager.GetOrgConnectionStats(gateway.OrganizationID)
		log.Printf("[WARN] Organization connection limit exceeded: orgID=%s count=%d max=%d",
			gateway.OrganizationID, stats.CurrentCount, stats.MaxAllowed)
		c.JSON(http.StatusTooManyRequests, utils.NewErrorResponse(429, "Too Many Requests",
			"Organization connection limit reached. Maximum allowed connections: "+
				fmt.Sprintf("%d", stats.MaxAllowed)))
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[ERROR] WebSocket upgrade failed: gatewayID=%s error=%v", gateway.ID, err)
		// Upgrade error is already sent by upgrader
		return
	}

	// Create WebSocket transport
	transport := ws.NewWebSocketTransport(conn)

	// Register connection with manager
	connection, err := h.manager.Register(gateway.ID, transport, apiKey, gateway.OrganizationID)
	if err != nil {
		log.Printf("[ERROR] Connection registration failed: gatewayID=%s orgID=%s error=%v",
			gateway.ID, gateway.OrganizationID, err)

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
			log.Printf("[WARN] Organization connection limit exceeded: orgID=%s count=%d max=%d",
				orgLimitErr.OrganizationID, orgLimitErr.CurrentCount, orgLimitErr.MaxAllowed)
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
		log.Printf("[ERROR] Failed to marshal connection ACK: gatewayID=%s error=%v", gateway.ID, err)
	} else {
		if err := connection.Send(ackJSON); err != nil {
			log.Printf("[ERROR] Failed to send connection ACK: gatewayID=%s connectionID=%s error=%v",
				gateway.ID, connection.ConnectionID, err)
		}
	}

	log.Printf("[INFO] WebSocket connection established: gatewayID=%s connectionID=%s",
		gateway.ID, connection.ConnectionID)

	// Update gateway active status to true when connection is established
	if err := h.gatewayService.UpdateGatewayActiveStatus(gateway.ID, true); err != nil {
		log.Printf("[ERROR] Failed to update gateway active status to true: gatewayID=%s error=%v", gateway.ID, err)
	}

	// Start reading messages (blocks until connection closes)
	// This keeps the handler goroutine alive to maintain the connection
	h.readLoop(connection)

	// Connection closed - cleanup
	log.Printf("[INFO] WebSocket connection closed: gatewayID=%s connectionID=%s",
		gateway.ID, connection.ConnectionID)
	h.manager.Unregister(gateway.ID, connection.ConnectionID)

	// Update gateway active status to false when connection is disconnected
	if err := h.gatewayService.UpdateGatewayActiveStatus(gateway.ID, false); err != nil {
		log.Printf("[ERROR] Failed to update gateway active status to false: gatewayID=%s error=%v", gateway.ID, err)
	}
}

// readLoop reads messages from the WebSocket connection.
// This is primarily for handling control frames (ping/pong) and detecting disconnections.
// Gateways are not expected to send application messages to the platform.
func (h *WebSocketHandler) readLoop(conn *ws.Connection) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ERROR] Panic in WebSocket read loop: gatewayID=%s connectionID=%s panic=%v",
				conn.GatewayID, conn.ConnectionID, r)
		}
	}()

	// Read messages until connection closes
	// The gorilla/websocket library handles ping/pong automatically via SetPongHandler
	for {
		// Check if connection is closed
		if conn.IsClosed() {
			return
		}

		// Read next message (blocks until message or error)
		// We don't expect gateways to send messages, but we need to read
		// to detect disconnections and handle control frames
		wsTransport, ok := conn.Transport.(*ws.WebSocketTransport)
		if !ok {
			log.Printf("[ERROR] Invalid transport type for connection: gatewayID=%s connectionID=%s",
				conn.GatewayID, conn.ConnectionID)
			return
		}

		_, _, err := wsTransport.ReadMessage()
		if err != nil {
			// Connection closed or error occurred
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("[ERROR] WebSocket read error: gatewayID=%s connectionID=%s error=%v",
					conn.GatewayID, conn.ConnectionID, err)
			}
			return
		}

		// If gateway sends messages, we can handle them here in future iterations
		// For now, we just ignore any messages from the gateway
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
