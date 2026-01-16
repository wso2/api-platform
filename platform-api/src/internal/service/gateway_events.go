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

package service

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	ws "platform-api/src/internal/websocket"

	"github.com/google/uuid"
)

const (
	// Maximum event payload size (1MB)
	MaxEventPayloadSize = 1024 * 1024
)

// GatewayEventsService handles broadcasting events to connected gateways
type GatewayEventsService struct {
	manager *ws.Manager
}

// NewGatewayEventsService creates a new gateway events service
func NewGatewayEventsService(manager *ws.Manager) *GatewayEventsService {
	return &GatewayEventsService{
		manager: manager,
	}
}

// BroadcastDeploymentEvent sends an API deployment event to target gateway
// This method handles:
// - Looking up gateway connections by gateway ID
// - Serializing event to JSON
// - Broadcasting to all connections for the gateway (clustering support)
// - Event ordering guarantee per gateway (sequential delivery)
// - Payload size validation
// - Delivery statistics tracking
// - Failure logging
func (s *GatewayEventsService) BroadcastDeploymentEvent(gatewayID string, deployment *model.APIDeploymentEvent) error {
	// Create correlation ID for tracing
	correlationID := uuid.New().String()

	// Serialize payload
	payloadJSON, err := json.Marshal(deployment)
	if err != nil {
		log.Printf("[ERROR] Failed to serialize deployment event: gatewayID=%s error=%v", gatewayID, err)
		return fmt.Errorf("failed to serialize deployment event: %w", err)
	}

	// Validate payload size
	if len(payloadJSON) > MaxEventPayloadSize {
		err := fmt.Errorf("event payload exceeds maximum size: %d bytes (limit: %d bytes)", len(payloadJSON), MaxEventPayloadSize)
		log.Printf("[ERROR] Payload size validation failed: gatewayID=%s size=%d error=%v", gatewayID, len(payloadJSON), err)
		return err
	}

	// Create gateway event DTO
	eventDTO := dto.GatewayEventDTO{
		Type:          "api.deployed",
		Payload:       deployment,
		Timestamp:     time.Now().Format(time.RFC3339),
		CorrelationID: correlationID,
	}

	// Serialize complete event
	eventJSON, err := json.Marshal(eventDTO)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal event DTO: gatewayID=%s correlationId=%s error=%v", gatewayID, correlationID, err)
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Get all connections for this gateway
	connections := s.manager.GetConnections(gatewayID)
	if len(connections) == 0 {
		log.Printf("[WARN] No active connections for gateway: gatewayID=%s correlationId=%s", gatewayID, correlationID)
		return fmt.Errorf("no active connections for gateway: %s", gatewayID)
	}

	// Broadcast to all connections (clustering support)
	// Events are delivered sequentially per connection to maintain ordering
	successCount := 0
	failureCount := 0
	var lastError error

	for _, conn := range connections {
		// Send event (Connection.Send is thread-safe)
		err := conn.Send(eventJSON)
		if err != nil {
			failureCount++
			lastError = err
			log.Printf("[ERROR] Failed to send deployment event: gatewayID=%s connectionID=%s correlationId=%s error=%v",
				gatewayID, conn.ConnectionID, correlationID, err)

			// Update delivery statistics for this connection
			conn.DeliveryStats.IncrementFailed(fmt.Sprintf("send error: %v", err))
		} else {
			successCount++
			log.Printf("[INFO] Deployment event sent: gatewayID=%s connectionID=%s correlationId=%s type=%s",
				gatewayID, conn.ConnectionID, correlationID, eventDTO.Type)

			// Update delivery statistics for this connection
			conn.DeliveryStats.IncrementTotalSent()
		}
	}

	// Log broadcast summary
	log.Printf("[INFO] Broadcast summary: gatewayID=%s correlationId=%s total=%d success=%d failed=%d",
		gatewayID, correlationID, len(connections), successCount, failureCount)

	// Return error if all deliveries failed
	if successCount == 0 {
		return fmt.Errorf("failed to deliver event to any connection: %w", lastError)
	}

	// Partial success is still considered success (some instances received the event)
	return nil
}

// BroadcastUndeploymentEvent sends an API undeployment event to target gateway
func (s *GatewayEventsService) BroadcastUndeploymentEvent(gatewayID string, undeployment *model.APIUndeploymentEvent) error {
	// Create correlation ID for tracing
	correlationID := uuid.New().String()

	// Serialize payload
	payloadJSON, err := json.Marshal(undeployment)
	if err != nil {
		log.Printf("[ERROR] Failed to serialize undeployment event: gatewayID=%s error=%v", gatewayID, err)
		return fmt.Errorf("failed to serialize undeployment event: %w", err)
	}

	// Validate payload size
	if len(payloadJSON) > MaxEventPayloadSize {
		err := fmt.Errorf("event payload exceeds maximum size: %d bytes (limit: %d bytes)", len(payloadJSON), MaxEventPayloadSize)
		log.Printf("[ERROR] Payload size validation failed: gatewayID=%s size=%d error=%v", gatewayID, len(payloadJSON), err)
		return err
	}

	// Create gateway event DTO with undeployment type
	eventDTO := dto.GatewayEventDTO{
		Type:          "api.undeployed",
		Payload:       undeployment,
		Timestamp:     time.Now().Format(time.RFC3339),
		CorrelationID: correlationID,
	}

	// Serialize complete event
	eventJSON, err := json.Marshal(eventDTO)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal undeployment event DTO: gatewayID=%s correlationId=%s error=%v", gatewayID, correlationID, err)
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Get all connections for this gateway
	connections := s.manager.GetConnections(gatewayID)
	if len(connections) == 0 {
		log.Printf("[WARN] No active connections for gateway: gatewayID=%s correlationId=%s", gatewayID, correlationID)
		return fmt.Errorf("no active connections for gateway: %s", gatewayID)
	}

	// Broadcast to all connections
	successCount := 0
	failureCount := 0
	var lastError error

	for _, conn := range connections {
		err := conn.Send(eventJSON)
		if err != nil {
			failureCount++
			lastError = err
			log.Printf("[ERROR] Failed to send undeployment event: gatewayID=%s connectionID=%s correlationId=%s error=%v",
				gatewayID, conn.ConnectionID, correlationID, err)
			conn.DeliveryStats.IncrementFailed(fmt.Sprintf("send error: %v", err))
		} else {
			successCount++
			log.Printf("[INFO] Undeployment event sent: gatewayID=%s connectionID=%s correlationId=%s type=%s",
				gatewayID, conn.ConnectionID, correlationID, eventDTO.Type)
			conn.DeliveryStats.IncrementTotalSent()
		}
	}

	// Log broadcast summary
	log.Printf("[INFO] Undeployment broadcast summary: gatewayID=%s correlationId=%s total=%d success=%d failed=%d",
		gatewayID, correlationID, len(connections), successCount, failureCount)

	if successCount == 0 {
		return fmt.Errorf("failed to deliver undeployment event to any connection: %w", lastError)
	}

	return nil
}
