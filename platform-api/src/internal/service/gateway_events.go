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

	// Gateway event type constants
	EventTypeAPIDeployed   = "api.deployed"
	EventTypeAPIUndeployed = "api.undeployed"
	EventTypeAPIDeleted    = "api.deleted"

	EventTypeLLMProviderDeployed   = "llmprovider.deployed"
	EventTypeLLMProviderUndeployed = "llmprovider.undeployed"
	EventTypeLLMProviderDeleted    = "llmprovider.deleted"

	EventTypeLLMProxyDeployed   = "llmproxy.deployed"
	EventTypeLLMProxyUndeployed = "llmproxy.undeployed"
	EventTypeLLMProxyDeleted    = "llmproxy.deleted"

	EventTypeMCPProxyDeployed   = "mcpproxy.deployed"
	EventTypeMCPProxyUndeployed = "mcpproxy.undeployed"
	EventTypeMCPProxyDeleted    = "mcpproxy.deleted"

	EventTypeAPIKeyCreated = "apikey.created"
	EventTypeAPIKeyRevoked = "apikey.revoked"
	EventTypeAPIKeyUpdated = "apikey.updated"

	EventTypeSubscriptionCreated = "subscription.created"
	EventTypeSubscriptionUpdated = "subscription.updated"
	EventTypeSubscriptionDeleted = "subscription.deleted"

	EventTypeSubscriptionPlanCreated = "subscriptionPlan.created"
	EventTypeSubscriptionPlanUpdated = "subscriptionPlan.updated"
	EventTypeSubscriptionPlanDeleted = "subscriptionPlan.deleted"
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

// BroadcastDeploymentEvent sends an API deployment event to target gateway.
func (s *GatewayEventsService) BroadcastDeploymentEvent(gatewayID string, deployment *model.DeploymentEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeAPIDeployed, deployment)
}

// BroadcastUndeploymentEvent sends an API undeployment event to target gateway.
func (s *GatewayEventsService) BroadcastUndeploymentEvent(gatewayID string, undeployment *model.APIUndeploymentEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeAPIUndeployed, undeployment)
}

// BroadcastAPIDeletionEvent sends an API deletion event to target gateway.
func (s *GatewayEventsService) BroadcastAPIDeletionEvent(gatewayID string, deletion *model.APIDeletionEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeAPIDeleted, deletion)
}

// BroadcastLLMProviderDeploymentEvent sends an LLM provider deployment event to target gateway.
func (s *GatewayEventsService) BroadcastLLMProviderDeploymentEvent(gatewayID string, deployment *model.LLMProviderDeploymentEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeLLMProviderDeployed, deployment)
}

// BroadcastLLMProviderUndeploymentEvent sends an LLM provider undeployment event to target gateway.
func (s *GatewayEventsService) BroadcastLLMProviderUndeploymentEvent(gatewayID string, undeployment *model.LLMProviderUndeploymentEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeLLMProviderUndeployed, undeployment)
}

// BroadcastLLMProxyDeploymentEvent sends an LLM proxy deployment event to target gateway.
func (s *GatewayEventsService) BroadcastLLMProxyDeploymentEvent(gatewayID string, deployment *model.LLMProxyDeploymentEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeLLMProxyDeployed, deployment)
}

// BroadcastLLMProxyUndeploymentEvent sends an LLM proxy undeployment event to target gateway.
func (s *GatewayEventsService) BroadcastLLMProxyUndeploymentEvent(gatewayID string, undeployment *model.LLMProxyUndeploymentEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeLLMProxyUndeployed, undeployment)
}

// BroadcastMCPProxyDeploymentEvent sends an MCP proxy deployment event to target gateway.
func (s *GatewayEventsService) BroadcastMCPProxyDeploymentEvent(gatewayID string, deployment *model.MCPProxyDeploymentEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeMCPProxyDeployed, deployment)
}

// BroadcastMCPProxyUndeploymentEvent sends an MCP proxy undeployment event to target gateway.
func (s *GatewayEventsService) BroadcastMCPProxyUndeploymentEvent(gatewayID string, undeployment *model.MCPProxyUndeploymentEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeMCPProxyUndeployed, undeployment)
}

// BroadcastMCPProxyDeletionEvent sends an MCP proxy deletion event to target gateway.
func (s *GatewayEventsService) BroadcastMCPProxyDeletionEvent(gatewayID string, deletion *model.MCPProxyDeletionEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeMCPProxyDeleted, deletion)
}

// BroadcastLLMProviderDeletionEvent sends an LLM provider deletion event to target gateway.
func (s *GatewayEventsService) BroadcastLLMProviderDeletionEvent(gatewayID string, deletion *model.LLMProviderDeletionEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeLLMProviderDeleted, deletion)
}

// BroadcastLLMProxyDeletionEvent sends an LLM proxy deletion event to target gateway.
func (s *GatewayEventsService) BroadcastLLMProxyDeletionEvent(gatewayID string, deletion *model.LLMProxyDeletionEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeLLMProxyDeleted, deletion)
}

// BroadcastAPIKeyCreatedEvent sends an API key created event to target gateway.
func (s *GatewayEventsService) BroadcastAPIKeyCreatedEvent(gatewayID, userId string, event *model.APIKeyCreatedEvent) error {
	const maxAttempts = 2
	var lastError error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := s.broadcastEventWithUserID(gatewayID, userId, EventTypeAPIKeyCreated, event); err == nil {
			return nil
		} else {
			lastError = err
			s.slogger.Warn("API key created event delivery failed", "gatewayID", gatewayID, "error", err)
		}
	}
	s.slogger.Error("API key created event delivery failed", "gatewayID", gatewayID, "error", lastError)
	return fmt.Errorf("failed to deliver API key created event: %w", lastError)
}

// BroadcastAPIKeyRevokedEvent sends an API key revoked event to target gateway.
func (s *GatewayEventsService) BroadcastAPIKeyRevokedEvent(gatewayID, userId string, event *model.APIKeyRevokedEvent) error {
	const maxAttempts = 2
	var lastError error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := s.broadcastEventWithUserID(gatewayID, userId, EventTypeAPIKeyRevoked, event); err == nil {
			return nil
		} else {
			lastError = err
			s.slogger.Warn("API key revoked event delivery failed", "gatewayID", gatewayID, "error", err)
		}
	}
	s.slogger.Error("API key revoked event delivery failed", "gatewayID", gatewayID, "error", lastError)
	return fmt.Errorf("failed to deliver API key revoked event: %w", lastError)
}

// BroadcastAPIKeyUpdatedEvent sends an API key updated event to target gateway.
func (s *GatewayEventsService) BroadcastAPIKeyUpdatedEvent(gatewayID, userId string, event *model.APIKeyUpdatedEvent) error {
	const maxAttempts = 2
	var lastError error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := s.broadcastEventWithUserID(gatewayID, userId, EventTypeAPIKeyUpdated, event); err == nil {
			return nil
		} else {
			lastError = err
			s.slogger.Warn("API key updated event delivery failed", "gatewayID", gatewayID, "error", err)
		}
	}
	s.slogger.Error("API key updated event delivery failed", "gatewayID", gatewayID, "error", lastError)
	return fmt.Errorf("failed to deliver API key updated event: %w", lastError)
}

// BroadcastApplicationUpdatedEvent sends an application updated event to target gateway.
func (s *GatewayEventsService) BroadcastApplicationUpdatedEvent(gatewayID, userId string, event *model.ApplicationUpdatedEvent) error {
	const maxAttempts = 2

	var lastError error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		err := s.broadcastApplicationUpdated(gatewayID, userId, event)
		if err == nil {
			return nil
		}

		lastError = err
		s.slogger.Warn("Application updated event delivery failed", "gatewayID", gatewayID, "error", err)
	}

	s.slogger.Error("Application updated event delivery failed", "gatewayID", gatewayID, "error", lastError)
	return fmt.Errorf("failed to deliver application updated event: %w", lastError)
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
			s.slogger.Debug("API key updated event sent",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "keyName", event.KeyName)
			conn.DeliveryStats.IncrementTotalSent()
			s.manager.IncrementTotalEventsSent()
		}
	}

	// Log broadcast summary
	s.slogger.Debug("Broadcast summary", "gatewayID", gatewayID, "correlationId", correlationID, "type", "apikey.updated", "total", len(connections), "success", successCount, "failed", failureCount)

	// Return error if all deliveries failed
	if successCount == 0 {
		return fmt.Errorf("failed to deliver event to any connection: %w", lastError)
	}

	return nil
}

// BroadcastSubscriptionCreatedEvent sends a subscription.created event to the target gateway.
func (s *GatewayEventsService) BroadcastSubscriptionCreatedEvent(gatewayID string, event *model.SubscriptionCreatedEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeSubscriptionCreated, event)
}

// BroadcastSubscriptionUpdatedEvent sends a subscription.updated event to the target gateway.
func (s *GatewayEventsService) BroadcastSubscriptionUpdatedEvent(gatewayID string, event *model.SubscriptionUpdatedEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeSubscriptionUpdated, event)
}

// BroadcastSubscriptionDeletedEvent sends a subscription.deleted event to the target gateway.
func (s *GatewayEventsService) BroadcastSubscriptionDeletedEvent(gatewayID string, event *model.SubscriptionDeletedEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeSubscriptionDeleted, event)
}

// broadcastEvent is the generic internal helper for broadcasting any gateway event.
func (s *GatewayEventsService) broadcastEvent(gatewayID, eventType string, payload interface{}) error {
	return s.broadcastEventWithUserID(gatewayID, "", eventType, payload)
}

// broadcastEventWithUserID is the generic internal helper for broadcasting gateway events with an optional userId.
func (s *GatewayEventsService) broadcastEventWithUserID(gatewayID, userId, eventType string, payload interface{}) error {
	correlationID := uuid.New().String()

	// Guard against nil or typed-nil payloads to avoid broadcasting malformed events.
	if payload == nil {
		return fmt.Errorf("%s payload is nil", eventType)
	}
	// Detect typed nils (e.g. nil slices, maps, pointers) which would serialize to JSON null and may not be expected by gateways.
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
		UserId:        userId,
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
	failureCount := 0
	var lastError error
	for _, conn := range connections {
		if err := conn.Send(eventJSON); err != nil {
			failureCount++
			lastError = err
			s.slogger.Error("Failed to send event",
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

	s.slogger.Debug("Broadcast summary", "gatewayID", gatewayID, "correlationId", correlationID, "type", eventType, "total", len(connections), "success", successCount, "failed", failureCount)

	if successCount == 0 {
		return fmt.Errorf("failed to deliver %s event to any connection: %w", eventType, lastError)
	}

	return nil
}

// BroadcastSubscriptionPlanCreatedEvent sends a subscriptionPlan.created event to the target gateway.
func (s *GatewayEventsService) BroadcastSubscriptionPlanCreatedEvent(gatewayID string, event *model.SubscriptionPlanCreatedEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeSubscriptionPlanCreated, event)
}

// BroadcastSubscriptionPlanUpdatedEvent sends a subscriptionPlan.updated event to the target gateway.
func (s *GatewayEventsService) BroadcastSubscriptionPlanUpdatedEvent(gatewayID string, event *model.SubscriptionPlanUpdatedEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeSubscriptionPlanUpdated, event)
}

// BroadcastSubscriptionPlanDeletedEvent sends a subscriptionPlan.deleted event to the target gateway.
func (s *GatewayEventsService) BroadcastSubscriptionPlanDeletedEvent(gatewayID string, event *model.SubscriptionPlanDeletedEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeSubscriptionPlanDeleted, event)
}

func (s *GatewayEventsService) broadcastApplicationUpdated(gatewayID, userId string, event *model.ApplicationUpdatedEvent) error {
	correlationID := uuid.New().String()

	payloadJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to serialize application updated event: %w", err)
	}

	if len(payloadJSON) > MaxEventPayloadSize {
		err := fmt.Errorf("event payload exceeds maximum size: %d bytes (limit: %d bytes)", len(payloadJSON), MaxEventPayloadSize)
		return err
	}

	eventDTO := dto.GatewayEventDTO{
		Type:          "application.updated",
		Payload:       event,
		Timestamp:     time.Now().Format(time.RFC3339),
		CorrelationID: correlationID,
		UserId:        userId,
	}

	eventJSON, err := json.Marshal(eventDTO)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	connections := s.manager.GetConnections(gatewayID)
	if len(connections) == 0 {
		return fmt.Errorf("no active connections for gateway: %s", gatewayID)
	}

	successCount := 0
	failureCount := 0
	var lastError error

	for _, conn := range connections {
		err := conn.Send(eventJSON)
		if err != nil {
			failureCount++
			lastError = err
			s.slogger.Error("Failed to send application updated event",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "error", err)
			conn.DeliveryStats.IncrementFailed(fmt.Sprintf("send error: %v", err))
		} else {
			successCount++
			s.slogger.Info("Application updated event sent",
				"gatewayID", gatewayID, "connectionID", conn.ConnectionID, "correlationId", correlationID, "applicationId", event.ApplicationId)
			conn.DeliveryStats.IncrementTotalSent()
			s.manager.IncrementTotalEventsSent()
		}
	}

	s.slogger.Info("Broadcast summary", "gatewayID", gatewayID, "correlationId", correlationID, "type", "application.updated", "total", len(connections), "success", successCount, "failed", failureCount)

	if successCount == 0 {
		return fmt.Errorf("failed to deliver event to any connection: %w", lastError)
	}

	return nil
}
