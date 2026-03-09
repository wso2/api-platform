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
	"log/slog"
	"reflect"
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
	slogger *slog.Logger
}

// NewGatewayEventsService creates a new gateway events service
func NewGatewayEventsService(manager *ws.Manager, slogger *slog.Logger) *GatewayEventsService {
	return &GatewayEventsService{
		manager: manager,
		slogger: slogger,
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
func (s *GatewayEventsService) BroadcastDeploymentEvent(gatewayID string, deployment *model.DeploymentEvent) error {
	// Create correlation ID for tracing
	correlationID := uuid.New().String()

	// Serialize payload
	payloadJSON, err := json.Marshal(deployment)
	if err != nil {
		s.slogger.Error("Failed to serialize deployment event", "gatewayID", gatewayID, "error", err)
		return fmt.Errorf("failed to serialize deployment event: %w", err)
	}

	// Validate payload size
	if len(payloadJSON) > MaxEventPayloadSize {
		err := fmt.Errorf("event payload exceeds maximum size: %d bytes (limit: %d bytes)", len(payloadJSON), MaxEventPayloadSize)
		s.slogger.Error("Payload size validation failed", "gatewayID", gatewayID, "size", len(payloadJSON), "error", err)
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
		s.slogger.Error("Failed to marshal event DTO", "gatewayID", gatewayID, "correlationId", correlationID, "error", err)
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Get all connections for this gateway
	connections := s.manager.GetConnections(gatewayID)
	if len(connections) == 0 {
		s.slogger.Warn("No active connections for gateway", "gatewayID", gatewayID, "correlationId", correlationID)
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
			s.slogger.Error("Failed to send deployment event",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "error", err)

			// Update delivery statistics for this connection
			conn.DeliveryStats.IncrementFailed(fmt.Sprintf("send error: %v", err))
		} else {
			successCount++
			s.slogger.Info("Deployment event sent",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "type", eventDTO.Type)

			// Update delivery statistics for this connection
			conn.DeliveryStats.IncrementTotalSent()
			s.manager.IncrementTotalEventsSent()
		}
	}

	// Log broadcast summary
	s.slogger.Info("Broadcast summary", "gatewayID", gatewayID, "correlationId", correlationID, "total", len(connections), "success", successCount, "failed", failureCount)

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
		s.slogger.Error("Failed to serialize undeployment event", "gatewayID", gatewayID, "error", err)
		return fmt.Errorf("failed to serialize undeployment event: %w", err)
	}

	// Validate payload size
	if len(payloadJSON) > MaxEventPayloadSize {
		err := fmt.Errorf("event payload exceeds maximum size: %d bytes (limit: %d bytes)", len(payloadJSON), MaxEventPayloadSize)
		s.slogger.Error("Payload size validation failed", "gatewayID", gatewayID, "size", len(payloadJSON), "error", err)
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
		s.slogger.Error("Failed to marshal event DTO", "gatewayID", gatewayID, "correlationId", correlationID, "error", err)
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Get all connections for this gateway
	connections := s.manager.GetConnections(gatewayID)
	if len(connections) == 0 {
		s.slogger.Warn("No active connections for gateway", "gatewayID", gatewayID, "correlationId", correlationID)
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
			s.slogger.Error("Failed to send undeployment event",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "error", err)
			conn.DeliveryStats.IncrementFailed(fmt.Sprintf("send error: %v", err))
		} else {
			successCount++
			s.slogger.Info("Undeployment event sent",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "type", eventDTO.Type)
			conn.DeliveryStats.IncrementTotalSent()
			s.manager.IncrementTotalEventsSent()
		}
	}

	// Log broadcast summary
	s.slogger.Info("Undeployment broadcast summary", "gatewayID", gatewayID, "correlationId", correlationID, "total", len(connections), "success", successCount, "failed", failureCount)

	if successCount == 0 {
		return fmt.Errorf("failed to deliver undeployment event to any connection: %w", lastError)
	}

	return nil
}

// BroadcastAPIDeletionEvent sends an API deletion event to target gateway
func (s *GatewayEventsService) BroadcastAPIDeletionEvent(gatewayID string, deletion *model.APIDeletionEvent) error {
	// Create correlation ID for tracing
	correlationID := uuid.New().String()

	// Serialize payload
	payloadJSON, err := json.Marshal(deletion)
	if err != nil {
		s.slogger.Error("Failed to serialize API deletion event", "gatewayID", gatewayID, "error", err)
		return fmt.Errorf("failed to serialize API deletion event: %w", err)
	}

	// Validate payload size
	if len(payloadJSON) > MaxEventPayloadSize {
		err := fmt.Errorf("event payload exceeds maximum size: %d bytes (limit: %d bytes)", len(payloadJSON), MaxEventPayloadSize)
		s.slogger.Error("Payload size validation failed", "gatewayID", gatewayID, "size", len(payloadJSON), "error", err)
		return err
	}

	// Create gateway event DTO with deletion type
	eventDTO := dto.GatewayEventDTO{
		Type:          "api.deleted",
		Payload:       deletion,
		Timestamp:     time.Now().Format(time.RFC3339),
		CorrelationID: correlationID,
	}

	// Serialize complete event
	eventJSON, err := json.Marshal(eventDTO)
	if err != nil {
		s.slogger.Error("Failed to marshal event DTO", "gatewayID", gatewayID, "correlationId", correlationID, "error", err)
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Get all connections for this gateway
	connections := s.manager.GetConnections(gatewayID)
	if len(connections) == 0 {
		s.slogger.Warn("No active connections for gateway", "gatewayID", gatewayID, "correlationId", correlationID)
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
			s.slogger.Error("Failed to send API deletion event",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "error", err)
			conn.DeliveryStats.IncrementFailed(fmt.Sprintf("send error: %v", err))
		} else {
			successCount++
			s.slogger.Info("API deletion event sent",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "type", eventDTO.Type)
			conn.DeliveryStats.IncrementTotalSent()
			s.manager.IncrementTotalEventsSent()
		}
	}

	// Log broadcast summary
	s.slogger.Info("API deletion broadcast summary", "gatewayID", gatewayID, "correlationId", correlationID, "total", len(connections), "success", successCount, "failed", failureCount)

	if successCount == 0 {
		return fmt.Errorf("failed to deliver API deletion event to any connection: %w", lastError)
	}

	return nil
}

// BroadcastLLMProviderDeploymentEvent sends an LLM provider deployment event to target gateway
func (s *GatewayEventsService) BroadcastLLMProviderDeploymentEvent(gatewayID string, deployment *model.LLMProviderDeploymentEvent) error {
	// Create correlation ID for tracing
	correlationID := uuid.New().String()

	// Serialize payload
	payloadJSON, err := json.Marshal(deployment)
	if err != nil {
		s.slogger.Error("Failed to serialize LLM provider deployment event", "gatewayID", gatewayID, "error", err)
		return fmt.Errorf("failed to serialize LLM provider deployment event: %w", err)
	}

	// Validate payload size
	if len(payloadJSON) > MaxEventPayloadSize {
		err := fmt.Errorf("event payload exceeds maximum size: %d bytes (limit: %d bytes)", len(payloadJSON), MaxEventPayloadSize)
		s.slogger.Error("Payload size validation failed", "gatewayID", gatewayID, "size", len(payloadJSON), "error", err)
		return err
	}

	// Create gateway event DTO
	eventDTO := dto.GatewayEventDTO{
		Type:          "llmprovider.deployed",
		Payload:       deployment,
		Timestamp:     time.Now().Format(time.RFC3339),
		CorrelationID: correlationID,
	}

	// Serialize complete event
	eventJSON, err := json.Marshal(eventDTO)
	if err != nil {
		s.slogger.Error("Failed to marshal event DTO", "gatewayID", gatewayID, "correlationId", correlationID, "error", err)
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Get all connections for this gateway
	connections := s.manager.GetConnections(gatewayID)
	if len(connections) == 0 {
		s.slogger.Warn("No active connections for gateway", "gatewayID", gatewayID, "correlationId", correlationID)
		return fmt.Errorf("no active connections for gateway: %s", gatewayID)
	}

	// Broadcast to all connections
	successCount := 0
	failureCount := 0
	var lastError error

	for _, conn := range connections {
		// Send event (Connection.Send is thread-safe)
		err := conn.Send(eventJSON)
		if err != nil {
			failureCount++
			lastError = err
			s.slogger.Error("Failed to send LLM provider deployment event",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "error", err)
			conn.DeliveryStats.IncrementFailed(fmt.Sprintf("send error: %v", err))
		} else {
			successCount++
			s.slogger.Info("LLM provider deployment event sent",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "type", eventDTO.Type)
			conn.DeliveryStats.IncrementTotalSent()
			s.manager.IncrementTotalEventsSent()
		}
	}

	// Log broadcast summary
	s.slogger.Info("LLM provider deployment broadcast summary", "gatewayID", gatewayID, "correlationId", correlationID, "total", len(connections), "success", successCount, "failed", failureCount)

	if successCount == 0 {
		return fmt.Errorf("failed to deliver LLM provider deployment event to any connection: %w", lastError)
	}

	return nil
}

// BroadcastLLMProviderUndeploymentEvent sends an LLM provider undeployment event to target gateway
func (s *GatewayEventsService) BroadcastLLMProviderUndeploymentEvent(gatewayID string, undeployment *model.LLMProviderUndeploymentEvent) error {
	// Create correlation ID for tracing
	correlationID := uuid.New().String()

	// Serialize payload
	payloadJSON, err := json.Marshal(undeployment)
	if err != nil {
		s.slogger.Error("Failed to serialize LLM provider undeployment event", "gatewayID", gatewayID, "error", err)
		return fmt.Errorf("failed to serialize LLM provider undeployment event: %w", err)
	}

	// Validate payload size
	if len(payloadJSON) > MaxEventPayloadSize {
		err := fmt.Errorf("event payload exceeds maximum size: %d bytes (limit: %d bytes)", len(payloadJSON), MaxEventPayloadSize)
		s.slogger.Error("Payload size validation failed", "gatewayID", gatewayID, "size", len(payloadJSON), "error", err)
		return err
	}

	// Create gateway event DTO with undeployment type
	eventDTO := dto.GatewayEventDTO{
		Type:          "llmprovider.undeployed",
		Payload:       undeployment,
		Timestamp:     time.Now().Format(time.RFC3339),
		CorrelationID: correlationID,
	}

	// Serialize complete event
	eventJSON, err := json.Marshal(eventDTO)
	if err != nil {
		s.slogger.Error("Failed to marshal event DTO", "gatewayID", gatewayID, "correlationId", correlationID, "error", err)
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Get all connections for this gateway
	connections := s.manager.GetConnections(gatewayID)
	if len(connections) == 0 {
		s.slogger.Warn("No active connections for gateway", "gatewayID", gatewayID, "correlationId", correlationID)
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
			s.slogger.Error("Failed to send LLM provider undeployment event",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "error", err)
			conn.DeliveryStats.IncrementFailed(fmt.Sprintf("send error: %v", err))
		} else {
			successCount++
			s.slogger.Info("LLM provider undeployment event sent",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "type", eventDTO.Type)
			conn.DeliveryStats.IncrementTotalSent()
			s.manager.IncrementTotalEventsSent()
		}
	}

	// Log broadcast summary
	s.slogger.Info("LLM provider undeployment broadcast summary", "gatewayID", gatewayID, "correlationId", correlationID, "total", len(connections), "success", successCount, "failed", failureCount)

	if successCount == 0 {
		return fmt.Errorf("failed to deliver LLM provider undeployment event to any connection: %w", lastError)
	}

	return nil
}

// BroadcastLLMProxyDeploymentEvent sends an LLM proxy deployment event to target gateway
func (s *GatewayEventsService) BroadcastLLMProxyDeploymentEvent(gatewayID string, deployment *model.LLMProxyDeploymentEvent) error {
	// Create correlation ID for tracing
	correlationID := uuid.New().String()

	// Serialize payload
	payloadJSON, err := json.Marshal(deployment)
	if err != nil {
		s.slogger.Error("Failed to serialize LLM proxy deployment event", "gatewayID", gatewayID, "error", err)
		return fmt.Errorf("failed to serialize LLM proxy deployment event: %w", err)
	}

	// Validate payload size
	if len(payloadJSON) > MaxEventPayloadSize {
		err := fmt.Errorf("event payload exceeds maximum size: %d bytes (limit: %d bytes)", len(payloadJSON), MaxEventPayloadSize)
		s.slogger.Error("Payload size validation failed", "gatewayID", gatewayID, "size", len(payloadJSON), "error", err)
		return err
	}

	// Create gateway event DTO
	eventDTO := dto.GatewayEventDTO{
		Type:          "llmproxy.deployed",
		Payload:       deployment,
		Timestamp:     time.Now().Format(time.RFC3339),
		CorrelationID: correlationID,
	}

	// Serialize complete event
	eventJSON, err := json.Marshal(eventDTO)
	if err != nil {
		s.slogger.Error("Failed to marshal event DTO", "gatewayID", gatewayID, "correlationId", correlationID, "error", err)
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Get all connections for this gateway
	connections := s.manager.GetConnections(gatewayID)
	if len(connections) == 0 {
		s.slogger.Warn("No active connections for gateway", "gatewayID", gatewayID, "correlationId", correlationID)
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
			s.slogger.Error("Failed to send LLM proxy deployment event",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "error", err)
			conn.DeliveryStats.IncrementFailed(fmt.Sprintf("send error: %v", err))
		} else {
			successCount++
			s.slogger.Info("LLM proxy deployment event sent",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "type", eventDTO.Type)
			conn.DeliveryStats.IncrementTotalSent()
			s.manager.IncrementTotalEventsSent()
		}
	}

	// Log broadcast summary
	s.slogger.Info("LLM proxy deployment broadcast summary", "gatewayID", gatewayID, "correlationId", correlationID, "total", len(connections), "success", successCount, "failed", failureCount)

	if successCount == 0 {
		return fmt.Errorf("failed to deliver LLM proxy deployment event to any connection: %w", lastError)
	}

	return nil
}

// BroadcastLLMProxyUndeploymentEvent sends an LLM proxy undeployment event to target gateway
func (s *GatewayEventsService) BroadcastLLMProxyUndeploymentEvent(gatewayID string, undeployment *model.LLMProxyUndeploymentEvent) error {
	// Create correlation ID for tracing
	correlationID := uuid.New().String()

	// Serialize payload
	payloadJSON, err := json.Marshal(undeployment)
	if err != nil {
		s.slogger.Error("Failed to serialize LLM proxy undeployment event", "gatewayID", gatewayID, "error", err)
		return fmt.Errorf("failed to serialize LLM proxy undeployment event: %w", err)
	}

	// Validate payload size
	if len(payloadJSON) > MaxEventPayloadSize {
		err := fmt.Errorf("event payload exceeds maximum size: %d bytes (limit: %d bytes)", len(payloadJSON), MaxEventPayloadSize)
		s.slogger.Error("Payload size validation failed", "gatewayID", gatewayID, "size", len(payloadJSON), "error", err)
		return err
	}

	// Create gateway event DTO with undeployment type
	eventDTO := dto.GatewayEventDTO{
		Type:          "llmproxy.undeployed",
		Payload:       undeployment,
		Timestamp:     time.Now().Format(time.RFC3339),
		CorrelationID: correlationID,
	}

	// Serialize complete event
	eventJSON, err := json.Marshal(eventDTO)
	if err != nil {
		s.slogger.Error("Failed to marshal event DTO", "gatewayID", gatewayID, "correlationId", correlationID, "error", err)
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Get all connections for this gateway
	connections := s.manager.GetConnections(gatewayID)
	if len(connections) == 0 {
		s.slogger.Warn("No active connections for gateway", "gatewayID", gatewayID, "correlationId", correlationID)
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
			s.slogger.Error("Failed to send LLM proxy undeployment event",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "error", err)
			conn.DeliveryStats.IncrementFailed(fmt.Sprintf("send error: %v", err))
		} else {
			successCount++
			s.slogger.Info("LLM proxy undeployment event sent",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "type", eventDTO.Type)
			conn.DeliveryStats.IncrementTotalSent()
			s.manager.IncrementTotalEventsSent()
		}
	}

	// Log broadcast summary
	s.slogger.Info("LLM proxy undeployment broadcast summary", "gatewayID", gatewayID, "correlationId", correlationID, "total", len(connections), "success", successCount, "failed", failureCount)

	if successCount == 0 {
		return fmt.Errorf("failed to deliver LLM proxy undeployment event to any connection: %w", lastError)
	}

	return nil
}

// BroadcastMCPProxyDeploymentEvent sends an MCP proxy deployment event to target gateway
func (s *GatewayEventsService) BroadcastMCPProxyDeploymentEvent(gatewayID string, deployment *model.MCPProxyDeploymentEvent) error {
	// Create correlation ID for tracing
	correlationID := uuid.New().String()

	// Serialize payload
	payloadJSON, err := json.Marshal(deployment)
	if err != nil {
		s.slogger.Error("Failed to serialize MCP proxy deployment event", "gatewayID", gatewayID, "error", err)
		return fmt.Errorf("failed to serialize MCP proxy deployment event: %w", err)
	}

	// Validate payload size
	if len(payloadJSON) > MaxEventPayloadSize {
		err := fmt.Errorf("event payload exceeds maximum size: %d bytes (limit: %d bytes)", len(payloadJSON), MaxEventPayloadSize)
		s.slogger.Error("Payload size validation failed", "gatewayID", gatewayID, "size", len(payloadJSON), "error", err)
		return err
	}

	// Create gateway event DTO
	eventDTO := dto.GatewayEventDTO{
		Type:          "mcpproxy.deployed",
		Payload:       deployment,
		Timestamp:     time.Now().Format(time.RFC3339),
		CorrelationID: correlationID,
	}

	// Serialize complete event
	eventJSON, err := json.Marshal(eventDTO)
	if err != nil {
		s.slogger.Error("Failed to marshal event DTO", "gatewayID", gatewayID, "correlationId", correlationID, "error", err)
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Get all connections for this gateway
	connections := s.manager.GetConnections(gatewayID)
	if len(connections) == 0 {
		s.slogger.Warn("No active connections for gateway", "gatewayID", gatewayID, "correlationId", correlationID)
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
			s.slogger.Error("Failed to send MCP proxy deployment event",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "error", err)
			conn.DeliveryStats.IncrementFailed(fmt.Sprintf("send error: %v", err))
		} else {
			successCount++
			s.slogger.Info("MCP proxy deployment event sent",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "type", eventDTO.Type)
			conn.DeliveryStats.IncrementTotalSent()
			s.manager.IncrementTotalEventsSent()
		}
	}

	// Log broadcast summary
	s.slogger.Info("MCP proxy deployment broadcast summary", "gatewayID", gatewayID, "correlationId", correlationID, "total", len(connections), "success", successCount, "failed", failureCount)

	if successCount == 0 {
		return fmt.Errorf("failed to deliver MCP proxy deployment event to any connection: %w", lastError)
	}

	return nil
}

// BroadcastMCPProxyUndeploymentEvent sends an MCP proxy undeployment event to target gateway
func (s *GatewayEventsService) BroadcastMCPProxyUndeploymentEvent(gatewayID string, undeployment *model.MCPProxyUndeploymentEvent) error {
	// Create correlation ID for tracing
	correlationID := uuid.New().String()

	// Serialize payload
	payloadJSON, err := json.Marshal(undeployment)
	if err != nil {
		s.slogger.Error("Failed to serialize MCP proxy undeployment event", "gatewayID", gatewayID, "error", err)
		return fmt.Errorf("failed to serialize MCP proxy undeployment event: %w", err)
	}

	// Validate payload size
	if len(payloadJSON) > MaxEventPayloadSize {
		err := fmt.Errorf("event payload exceeds maximum size: %d bytes (limit: %d bytes)", len(payloadJSON), MaxEventPayloadSize)
		s.slogger.Error("Payload size validation failed", "gatewayID", gatewayID, "size", len(payloadJSON), "error", err)
		return err
	}

	// Create gateway event DTO with undeployment type
	eventDTO := dto.GatewayEventDTO{
		Type:          "mcpproxy.undeployed",
		Payload:       undeployment,
		Timestamp:     time.Now().Format(time.RFC3339),
		CorrelationID: correlationID,
	}

	// Serialize complete event
	eventJSON, err := json.Marshal(eventDTO)
	if err != nil {
		s.slogger.Error("Failed to marshal event DTO", "gatewayID", gatewayID, "correlationId", correlationID, "error", err)
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Get all connections for this gateway
	connections := s.manager.GetConnections(gatewayID)
	if len(connections) == 0 {
		s.slogger.Warn("No active connections for gateway", "gatewayID", gatewayID, "correlationId", correlationID)
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
			s.slogger.Error("Failed to send MCP proxy undeployment event",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "error", err)
			conn.DeliveryStats.IncrementFailed(fmt.Sprintf("send error: %v", err))
		} else {
			successCount++
			s.slogger.Info("MCP proxy undeployment event sent",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "type", eventDTO.Type)
			conn.DeliveryStats.IncrementTotalSent()
			s.manager.IncrementTotalEventsSent()
		}
	}

	// Log broadcast summary
	s.slogger.Info("MCP proxy undeployment broadcast summary", "gatewayID", gatewayID, "correlationId", correlationID, "total", len(connections), "success", successCount, "failed", failureCount)

	if successCount == 0 {
		return fmt.Errorf("failed to deliver MCP proxy undeployment event to any connection: %w", lastError)
	}

	return nil
}

// BroadcastMCPProxyDeletionEvent sends an MCP proxy deletion event to target gateway
func (s *GatewayEventsService) BroadcastMCPProxyDeletionEvent(gatewayID string, deletion *model.MCPProxyDeletionEvent) error {
	// Create correlation ID for tracing
	correlationID := uuid.New().String()

	// Serialize payload
	payloadJSON, err := json.Marshal(deletion)
	if err != nil {
		s.slogger.Error("Failed to serialize MCP proxy deletion event", "gatewayID", gatewayID, "error", err)
		return fmt.Errorf("failed to serialize MCP proxy deletion event: %w", err)
	}

	// Validate payload size
	if len(payloadJSON) > MaxEventPayloadSize {
		err := fmt.Errorf("event payload exceeds maximum size: %d bytes (limit: %d bytes)", len(payloadJSON), MaxEventPayloadSize)
		s.slogger.Error("Payload size validation failed", "gatewayID", gatewayID, "size", len(payloadJSON), "error", err)
		return err
	}

	// Create gateway event DTO with deletion type
	eventDTO := dto.GatewayEventDTO{
		Type:          "mcpproxy.deleted",
		Payload:       deletion,
		Timestamp:     time.Now().Format(time.RFC3339),
		CorrelationID: correlationID,
	}

	// Serialize complete event
	eventJSON, err := json.Marshal(eventDTO)
	if err != nil {
		s.slogger.Error("Failed to marshal event DTO", "gatewayID", gatewayID, "correlationId", correlationID, "error", err)
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Get all connections for this gateway
	connections := s.manager.GetConnections(gatewayID)
	if len(connections) == 0 {
		s.slogger.Warn("No active connections for gateway", "gatewayID", gatewayID, "correlationId", correlationID)
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
			s.slogger.Error("Failed to send MCP proxy deletion event",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "error", err)
			conn.DeliveryStats.IncrementFailed(fmt.Sprintf("send error: %v", err))
		} else {
			successCount++
			s.slogger.Info("MCP proxy deletion event sent",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "type", eventDTO.Type)
			conn.DeliveryStats.IncrementTotalSent()
			s.manager.IncrementTotalEventsSent()
		}
	}

	// Log broadcast summary
	s.slogger.Info("MCP proxy deletion broadcast summary", "gatewayID", gatewayID, "correlationId", correlationID, "total", len(connections), "success", successCount, "failed", failureCount)

	if successCount == 0 {
		return fmt.Errorf("failed to deliver MCP proxy deletion event to any connection: %w", lastError)
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
		s.slogger.Warn("API key created event delivery failed", "gatewayID", gatewayID, "error", err)
	}

	s.slogger.Error("API key created event delivery failed", "gatewayID", gatewayID, "error", lastError)
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
		s.slogger.Warn("API key revoked event delivery failed", "gatewayID", gatewayID, "error", err)
	}

	s.slogger.Error("API key revoked event delivery failed", "gatewayID", gatewayID, "error", lastError)
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
			s.slogger.Error("Failed to send API key created event",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "error", err)
			conn.DeliveryStats.IncrementFailed(fmt.Sprintf("send error: %v", err))
		} else {
			successCount++
			s.slogger.Info("API key created event sent",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "keyName", event.Name)
			conn.DeliveryStats.IncrementTotalSent()
			s.manager.IncrementTotalEventsSent()
		}
	}

	// Log broadcast summary
	s.slogger.Info("Broadcast summary", "gatewayID", gatewayID, "correlationId", correlationID, "type", "apikey.created", "total", len(connections), "success", successCount, "failed", failureCount)

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
			s.slogger.Error("Failed to send API key revoked event",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "error", err)
			conn.DeliveryStats.IncrementFailed(fmt.Sprintf("send error: %v", err))
		} else {
			successCount++
			s.slogger.Info("API key revoked event sent",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "keyName", event.KeyName)
			conn.DeliveryStats.IncrementTotalSent()
			s.manager.IncrementTotalEventsSent()
		}
	}

	// Log broadcast summary
	s.slogger.Info("Broadcast summary", "gatewayID", gatewayID, "correlationId", correlationID, "type", "apikey.revoked", "total", len(connections), "success", successCount, "failed", failureCount)

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
		s.slogger.Warn("API key updated event delivery failed", "gatewayID", gatewayID, "error", err)
	}

	s.slogger.Error("API key updated event delivery failed", "gatewayID", gatewayID, "error", lastError)
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
			s.slogger.Error("Failed to send API key updated event",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "error", err)
			conn.DeliveryStats.IncrementFailed(fmt.Sprintf("send error: %v", err))
		} else {
			successCount++
			s.slogger.Info("API key updated event sent",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "keyName", event.KeyName)
			conn.DeliveryStats.IncrementTotalSent()
			s.manager.IncrementTotalEventsSent()
		}
	}

	// Log broadcast summary
	s.slogger.Info("Broadcast summary", "gatewayID", gatewayID, "correlationId", correlationID, "type", "apikey.updated", "total", len(connections), "success", successCount, "failed", failureCount)

	// Return error if all deliveries failed
	if successCount == 0 {
		return fmt.Errorf("failed to deliver event to any connection: %w", lastError)
	}

	return nil
}

// BroadcastSubscriptionCreatedEvent sends a subscription.created event to the target gateway.
func (s *GatewayEventsService) BroadcastSubscriptionCreatedEvent(gatewayID string, event *model.SubscriptionCreatedEvent) error {
	return s.broadcastSubscriptionEvent(gatewayID, "subscription.created", event)
}

// BroadcastSubscriptionUpdatedEvent sends a subscription.updated event to the target gateway.
func (s *GatewayEventsService) BroadcastSubscriptionUpdatedEvent(gatewayID string, event *model.SubscriptionUpdatedEvent) error {
	return s.broadcastSubscriptionEvent(gatewayID, "subscription.updated", event)
}

// BroadcastSubscriptionDeletedEvent sends a subscription.deleted event to the target gateway.
func (s *GatewayEventsService) BroadcastSubscriptionDeletedEvent(gatewayID string, event *model.SubscriptionDeletedEvent) error {
	return s.broadcastSubscriptionEvent(gatewayID, "subscription.deleted", event)
}

func (s *GatewayEventsService) broadcastSubscriptionEvent(gatewayID, eventType string, payload interface{}) error {
	correlationID := uuid.New().String()

	// Guard against nil or typed-nil payloads to avoid broadcasting malformed events.
	if payload == nil {
		return fmt.Errorf("%s payload is nil", eventType)
	}
	// Detect typed nils (e.g., (*SubscriptionCreatedEvent)(nil)).
	val := reflect.ValueOf(payload)
	switch val.Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Interface:
		if val.IsNil() {
			return fmt.Errorf("%s payload is nil", eventType)
		}
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to serialize %s event: %w", eventType, err)
	}
	if len(payloadJSON) > MaxEventPayloadSize {
		return fmt.Errorf("%s payload exceeds maximum size: %d (limit: %d)", eventType, len(payloadJSON), MaxEventPayloadSize)
	}

	eventDTO := dto.GatewayEventDTO{
		Type:          eventType,
		Payload:       payload,
		Timestamp:     time.Now().Format(time.RFC3339),
		CorrelationID: correlationID,
	}

	eventJSON, err := json.Marshal(eventDTO)
	if err != nil {
		return fmt.Errorf("failed to marshal %s event: %w", eventType, err)
	}

	connections := s.manager.GetConnections(gatewayID)
	if len(connections) == 0 {
		return fmt.Errorf("no active connections for gateway: %s", gatewayID)
	}

	successCount := 0
	var lastError error
	for _, conn := range connections {
		if err := conn.Send(eventJSON); err != nil {
			lastError = err
			s.slogger.Error("Failed to send subscription event",
				"gatewayID", gatewayID,
				"connectionID", conn.ConnectionID,
				"correlationId", correlationID,
				"type", eventType,
				"error", err,
			)
			conn.DeliveryStats.IncrementFailed(fmt.Sprintf("send error: %v", err))
		} else {
			successCount++
			conn.DeliveryStats.IncrementTotalSent()
			s.manager.IncrementTotalEventsSent()
		}
	}

	if successCount == 0 {
		return fmt.Errorf("failed to deliver %s event to any connection: %w", eventType, lastError)
	}

	return nil
}

// BroadcastSubscriptionPlanCreatedEvent sends a subscriptionPlan.created event to the target gateway.
func (s *GatewayEventsService) BroadcastSubscriptionPlanCreatedEvent(gatewayID string, event *model.SubscriptionPlanCreatedEvent) error {
	return s.broadcastSubscriptionEvent(gatewayID, "subscriptionPlan.created", event)
}

// BroadcastSubscriptionPlanUpdatedEvent sends a subscriptionPlan.updated event to the target gateway.
func (s *GatewayEventsService) BroadcastSubscriptionPlanUpdatedEvent(gatewayID string, event *model.SubscriptionPlanUpdatedEvent) error {
	return s.broadcastSubscriptionEvent(gatewayID, "subscriptionPlan.updated", event)
}

// BroadcastSubscriptionPlanDeletedEvent sends a subscriptionPlan.deleted event to the target gateway.
func (s *GatewayEventsService) BroadcastSubscriptionPlanDeletedEvent(gatewayID string, event *model.SubscriptionPlanDeletedEvent) error {
	return s.broadcastSubscriptionEvent(gatewayID, "subscriptionPlan.deleted", event)
}
