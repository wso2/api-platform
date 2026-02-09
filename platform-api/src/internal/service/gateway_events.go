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
		log.Printf("[ERROR] Failed to marshal event DTO: gatewayID=%s correlationId=%s error=%v", gatewayID, correlationID, err)
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

// BroadcastAPIKeyCreatedEvent sends an API key created event to target gateway.
// This method handles:
// - Looking up gateway connections by gateway ID
// - Serializing event to JSON
// - Broadcasting to all connections for the gateway (clustering support)
// - Up to 2 attempts per call (no backoff; caller should handle broader retry logic if needed)
// - Payload size validation
// - Delivery statistics tracking
func (s *GatewayEventsService) BroadcastAPIKeyCreatedEvent(gatewayID, userId string, event *model.APIKeyCreatedEvent) error {
	const maxAttempts = 2

	var lastError error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		err := s.broadcastAPIKeyCreated(gatewayID, userId, event)
		if err == nil {
			return nil
		}

		lastError = err
		log.Printf("[WARN] API key created event delivery failed: gatewayID=%s error=%v",
			gatewayID, err)
	}

	log.Printf("[ERROR] API key created event delivery failed: gatewayID=%s error=%v",
		gatewayID, lastError)
	return fmt.Errorf("failed to deliver API key created event: %w", lastError)
}

// BroadcastAPIKeyRevokedEvent sends an API key revoked event to target gateway.
// This method handles:
// - Looking up gateway connections by gateway ID
// - Serializing event to JSON
// - Broadcasting to all connections for the gateway (clustering support)
// - Up to 2 attempts per call (no backoff; caller should handle broader retry logic if needed)
// - Payload size validation
// - Delivery statistics tracking
func (s *GatewayEventsService) BroadcastAPIKeyRevokedEvent(gatewayID, userId string, event *model.APIKeyRevokedEvent) error {
	const maxAttempts = 2

	var lastError error

	// Up to 2 attempts (no backoff)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		err := s.broadcastAPIKeyRevoked(gatewayID, userId, event)
		if err == nil {
			return nil
		}

		lastError = err
		log.Printf("[WARN] API key revoked event delivery failed: gatewayID=%s error=%v",
			gatewayID, err)
	}

	log.Printf("[ERROR] API key revoked event delivery failed: gatewayID=%s error=%v",
		gatewayID, lastError)
	return fmt.Errorf("failed to deliver API key revoked event: %w", lastError)
}

// broadcastAPIKeyCreated is the internal implementation for broadcasting API key created events
func (s *GatewayEventsService) broadcastAPIKeyCreated(gatewayID, userId string, event *model.APIKeyCreatedEvent) error {
	// Create correlation ID for tracing
	correlationID := uuid.New().String()

	// Serialize payload
	payloadJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to serialize API key created event: %w", err)
	}

	// Validate payload size
	if len(payloadJSON) > MaxEventPayloadSize {
		err := fmt.Errorf("event payload exceeds maximum size: %d bytes (limit: %d bytes)", len(payloadJSON), MaxEventPayloadSize)
		return err
	}

	// Create gateway event DTO
	eventDTO := dto.GatewayEventDTO{
		Type:          "apikey.created",
		Payload:       event,
		Timestamp:     time.Now().Format(time.RFC3339),
		CorrelationID: correlationID,
		UserId:        userId,
	}

	// Serialize complete event
	eventJSON, err := json.Marshal(eventDTO)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Get all connections for this gateway
	connections := s.manager.GetConnections(gatewayID)
	if len(connections) == 0 {
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
			log.Printf("[ERROR] Failed to send API key created event: gatewayID=%s connectionID=%s correlationId=%s error=%v",
				gatewayID, conn.ConnectionID, correlationID, err)
			conn.DeliveryStats.IncrementFailed(fmt.Sprintf("send error: %v", err))
		} else {
			successCount++
			log.Printf("[INFO] API key created event sent: gatewayID=%s connectionID=%s correlationId=%s keyName=%s",
				gatewayID, conn.ConnectionID, correlationID, event.Name)
			conn.DeliveryStats.IncrementTotalSent()
		}
	}

	// Log broadcast summary
	log.Printf("[INFO] Broadcast summary: gatewayID=%s correlationId=%s type=apikey.created total=%d success=%d failed=%d",
		gatewayID, correlationID, len(connections), successCount, failureCount)

	// Return error if all deliveries failed
	if successCount == 0 {
		return fmt.Errorf("failed to deliver event to any connection: %w", lastError)
	}

	return nil
}

// broadcastAPIKeyRevoked is the internal implementation for broadcasting API key revoked events
func (s *GatewayEventsService) broadcastAPIKeyRevoked(gatewayID, userId string, event *model.APIKeyRevokedEvent) error {
	// Create correlation ID for tracing
	correlationID := uuid.New().String()

	// Serialize payload
	payloadJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to serialize API key revoked event: %w", err)
	}

	// Validate payload size
	if len(payloadJSON) > MaxEventPayloadSize {
		err := fmt.Errorf("event payload exceeds maximum size: %d bytes (limit: %d bytes)", len(payloadJSON), MaxEventPayloadSize)
		return err
	}

	// Create gateway event DTO
	eventDTO := dto.GatewayEventDTO{
		Type:          "apikey.revoked",
		Payload:       event,
		Timestamp:     time.Now().Format(time.RFC3339),
		CorrelationID: correlationID,
		UserId:        userId,
	}

	// Serialize complete event
	eventJSON, err := json.Marshal(eventDTO)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Get all connections for this gateway
	connections := s.manager.GetConnections(gatewayID)
	if len(connections) == 0 {
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
			log.Printf("[ERROR] Failed to send API key revoked event: gatewayID=%s connectionID=%s correlationId=%s error=%v",
				gatewayID, conn.ConnectionID, correlationID, err)
			conn.DeliveryStats.IncrementFailed(fmt.Sprintf("send error: %v", err))
		} else {
			successCount++
			log.Printf("[INFO] API key revoked event sent: gatewayID=%s connectionID=%s correlationId=%s keyName=%s",
				gatewayID, conn.ConnectionID, correlationID, event.KeyName)
			conn.DeliveryStats.IncrementTotalSent()
		}
	}

	// Log broadcast summary
	log.Printf("[INFO] Broadcast summary: gatewayID=%s correlationId=%s type=apikey.revoked total=%d success=%d failed=%d",
		gatewayID, correlationID, len(connections), successCount, failureCount)

	// Return error if all deliveries failed
	if successCount == 0 {
		return fmt.Errorf("failed to deliver event to any connection: %w", lastError)
	}

	return nil
}

// BroadcastAPIKeyUpdatedEvent sends an API key updated event to target gateway.
// This method handles:
// - Looking up gateway connections by gateway ID
// - Serializing event to JSON
// - Broadcasting to all connections for the gateway (clustering support)
// - Up to 2 attempts per call (no backoff; caller should handle broader retry logic if needed)
// - Payload size validation
// - Delivery statistics tracking
func (s *GatewayEventsService) BroadcastAPIKeyUpdatedEvent(gatewayID, userId string, event *model.APIKeyUpdatedEvent) error {
	const maxAttempts = 2

	var lastError error

	// Up to 2 attempts (no backoff)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		err := s.broadcastAPIKeyUpdated(gatewayID, userId, event)
		if err == nil {
			return nil
		}

		lastError = err
		log.Printf("[WARN] API key updated event delivery failed: gatewayID=%s error=%v",
			gatewayID, err)
	}

	log.Printf("[ERROR] API key updated event delivery failed: gatewayID=%s error=%v",
		gatewayID, lastError)
	return fmt.Errorf("failed to deliver API key update event: %w", lastError)
}

// broadcastAPIKeyUpdated is the internal implementation for broadcasting API key updated events
func (s *GatewayEventsService) broadcastAPIKeyUpdated(gatewayID, userId string, event *model.APIKeyUpdatedEvent) error {
	// Create correlation ID for tracing
	correlationID := uuid.New().String()

	// Serialize payload
	payloadJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to serialize API key updated event: %w", err)
	}

	// Validate payload size
	if len(payloadJSON) > MaxEventPayloadSize {
		err := fmt.Errorf("event payload exceeds maximum size: %d bytes (limit: %d bytes)", len(payloadJSON), MaxEventPayloadSize)
		return err
	}

	// Create gateway event DTO
	eventDTO := dto.GatewayEventDTO{
		Type:          "apikey.updated",
		Payload:       event,
		Timestamp:     time.Now().Format(time.RFC3339),
		CorrelationID: correlationID,
		UserId:        userId,
	}

	// Serialize complete event
	eventJSON, err := json.Marshal(eventDTO)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Get all connections for this gateway
	connections := s.manager.GetConnections(gatewayID)
	if len(connections) == 0 {
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
			log.Printf("[ERROR] Failed to send API key updated event: gatewayID=%s connectionID=%s correlationId=%s error=%v",
				gatewayID, conn.ConnectionID, correlationID, err)
			conn.DeliveryStats.IncrementFailed(fmt.Sprintf("send error: %v", err))
		} else {
			successCount++
			log.Printf("[INFO] API key updated event sent: gatewayID=%s connectionID=%s correlationId=%s keyName=%s",
				gatewayID, conn.ConnectionID, correlationID, event.KeyName)
			conn.DeliveryStats.IncrementTotalSent()
		}
	}

	// Log broadcast summary
	log.Printf("[INFO] Broadcast summary: gatewayID=%s correlationId=%s type=apikey.updated total=%d success=%d failed=%d",
		gatewayID, correlationID, len(connections), successCount, failureCount)

	// Return error if all deliveries failed
	if successCount == 0 {
		return fmt.Errorf("failed to deliver event to any connection: %w", lastError)
	}

	return nil
}
