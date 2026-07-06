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
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wso2/api-platform/common/eventhub"

	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

const (
	// Maximum event payload size (1MB)
	MaxEventPayloadSize = 1024 * 1024

	// eventTypePlatformGateway is the EventHub entity_type used for all platform-api gateway events.
	// The full event type string is stored inside the EventData JSON payload.
	eventTypePlatformGateway eventhub.EventType = "PLATFORM_GATEWAY_EVENT"

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

	EventTypeWebSubAPIDeployed   = "websub.deployed"
	EventTypeWebSubAPIUndeployed = "websub.undeployed"
	EventTypeWebSubAPIDeleted    = "websub.deleted"

	EventTypeWebBrokerAPIDeployed   = "webbroker.deployed"
	EventTypeWebBrokerAPIUndeployed = "webbroker.undeployed"
	EventTypeWebBrokerAPIDeleted    = "webbroker.deleted"

	EventTypeAPIKeyCreated = "apikey.created"
	EventTypeAPIKeyRevoked = "apikey.revoked"
	EventTypeAPIKeyUpdated = "apikey.updated"

	EventTypeWebSubAPIHmacSecretCreated = "websub.hmacsecret.created"
	EventTypeWebSubAPIHmacSecretUpdated = "websub.hmacsecret.updated"
	EventTypeWebSubAPIHmacSecretDeleted = "websub.hmacsecret.deleted"

	EventTypeSubscriptionCreated = "subscription.created"
	EventTypeSubscriptionUpdated = "subscription.updated"
	EventTypeSubscriptionDeleted = "subscription.deleted"

	EventTypeSubscriptionPlanCreated = "subscriptionPlan.created"
	EventTypeSubscriptionPlanUpdated = "subscriptionPlan.updated"
	EventTypeSubscriptionPlanDeleted = "subscriptionPlan.deleted"
)

// GatewayEventsService handles broadcasting events to connected gateways via EventHub.
// Publishing to the EventHub persists the event to the shared DB so any platform-api
// replica can pick it up and deliver it to the gateway WebSocket it holds.
type GatewayEventsService struct {
	hub      eventhub.EventHub
	identity *IdentityService
	slogger  *slog.Logger
}

// NewGatewayEventsService creates a new gateway events service backed by the EventHub.
func NewGatewayEventsService(hub eventhub.EventHub, identity *IdentityService, slogger *slog.Logger) *GatewayEventsService {
	return &GatewayEventsService{
		hub:      hub,
		identity: identity,
		slogger:  slogger,
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

// BroadcastWebSubAPIDeploymentEvent sends a WebSub API deployment event to target gateway.
func (s *GatewayEventsService) BroadcastWebSubAPIDeploymentEvent(gatewayID string, deployment *model.WebSubAPIDeploymentEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeWebSubAPIDeployed, deployment)
}

// BroadcastWebSubAPIUndeploymentEvent sends a WebSub API undeployment event to target gateway.
func (s *GatewayEventsService) BroadcastWebSubAPIUndeploymentEvent(gatewayID string, undeployment *model.WebSubAPIUndeploymentEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeWebSubAPIUndeployed, undeployment)
}

// BroadcastWebSubAPIDeletionEvent sends a WebSub API deletion event to target gateway.
func (s *GatewayEventsService) BroadcastWebSubAPIDeletionEvent(gatewayID string, deletion *model.WebSubAPIDeletionEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeWebSubAPIDeleted, deletion)
}

// BroadcastWebBrokerAPIDeploymentEvent sends a WebBroker API deployment event to target gateway.
func (s *GatewayEventsService) BroadcastWebBrokerAPIDeploymentEvent(gatewayID string, deployment *model.WebBrokerAPIDeploymentEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeWebBrokerAPIDeployed, deployment)
}

// BroadcastWebBrokerAPIUndeploymentEvent sends a WebBroker API undeployment event to target gateway.
func (s *GatewayEventsService) BroadcastWebBrokerAPIUndeploymentEvent(gatewayID string, undeployment *model.WebBrokerAPIUndeploymentEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeWebBrokerAPIUndeployed, undeployment)
}

// BroadcastWebBrokerAPIDeletionEvent sends a WebBroker API deletion event to target gateway.
func (s *GatewayEventsService) BroadcastWebBrokerAPIDeletionEvent(gatewayID string, deletion *model.WebBrokerAPIDeletionEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeWebBrokerAPIDeleted, deletion)
}

// BroadcastLLMProviderDeletionEvent sends an LLM provider deletion event to target gateway.
func (s *GatewayEventsService) BroadcastLLMProviderDeletionEvent(gatewayID string, deletion *model.LLMProviderDeletionEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeLLMProviderDeleted, deletion)
}

// BroadcastLLMProxyDeletionEvent sends an LLM proxy deletion event to target gateway.
func (s *GatewayEventsService) BroadcastLLMProxyDeletionEvent(gatewayID string, deletion *model.LLMProxyDeletionEvent) error {
	return s.broadcastEvent(gatewayID, EventTypeLLMProxyDeleted, deletion)
}

// BroadcastWebSubAPIHmacSecretEvent sends a WebSub API HMAC secret lifecycle event to target gateway.
// action should be "CREATED", "UPDATED", or "DELETED".
func (s *GatewayEventsService) BroadcastWebSubAPIHmacSecretEvent(gatewayID, action string, event *model.WebSubAPIHmacSecretEvent) error {
	var eventType string
	switch action {
	case "CREATED":
		eventType = EventTypeWebSubAPIHmacSecretCreated
	case "UPDATED":
		eventType = EventTypeWebSubAPIHmacSecretUpdated
	case "DELETED":
		eventType = EventTypeWebSubAPIHmacSecretDeleted
	default:
		eventType = EventTypeWebSubAPIHmacSecretUpdated
	}
	return s.broadcastEvent(gatewayID, eventType, event)
}

// BroadcastAPIKeyCreatedEvent sends an API key created event to target gateway.
func (s *GatewayEventsService) BroadcastAPIKeyCreatedEvent(gatewayID, userId string, event *model.APIKeyCreatedEvent) error {
	return s.broadcastEventWithUserID(gatewayID, userId, EventTypeAPIKeyCreated, event)
}

// BroadcastAPIKeyRevokedEvent sends an API key revoked event to target gateway.
func (s *GatewayEventsService) BroadcastAPIKeyRevokedEvent(gatewayID, userId string, event *model.APIKeyRevokedEvent) error {
	return s.broadcastEventWithUserID(gatewayID, userId, EventTypeAPIKeyRevoked, event)
}

// BroadcastAPIKeyUpdatedEvent sends an API key updated event to target gateway.
func (s *GatewayEventsService) BroadcastAPIKeyUpdatedEvent(gatewayID, userId string, event *model.APIKeyUpdatedEvent) error {
	return s.broadcastEventWithUserID(gatewayID, userId, EventTypeAPIKeyUpdated, event)
}

// BroadcastApplicationUpdatedEvent sends an application updated event to target gateway.
func (s *GatewayEventsService) BroadcastApplicationUpdatedEvent(gatewayID, userId string, event *model.ApplicationUpdatedEvent) error {
	return s.broadcastEventWithUserID(gatewayID, userId, "application.updated", event)
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

// broadcastEvent is the generic helper for broadcasting gateway events without a userId.
func (s *GatewayEventsService) broadcastEvent(gatewayID, eventType string, payload interface{}) error {
	return s.broadcastEventWithUserID(gatewayID, "", eventType, payload)
}

// broadcastEventWithUserID serializes the payload into a GatewayEventDTO and publishes it
// to the EventHub. The EventHub persists it to the shared DB; the EventDispatcher on
// whichever replica holds the gateway WebSocket will pick it up and deliver it.
//
// userId is expected to be the internal UUID stored in the triggering audit column (or
// empty when the event has no associated actor, e.g. system-triggered deployment events).
// This is the single sink for every actor-bearing gateway event, so it is also the single
// place the internal UUID is unwrapped to the raw external identity (constants.DeletedUser
// if the UUID has no mapping) before the event leaves the process — the internal UUID must
// never reach the data plane. An empty userId is left empty (no actor to resolve), not
// turned into constants.DeletedUser.
func (s *GatewayEventsService) broadcastEventWithUserID(gatewayID, userId, eventType string, payload interface{}) error {
	correlationID := uuid.New().String()

	if payload == nil {
		return fmt.Errorf("%s payload is nil", eventType)
	}
	val := reflect.ValueOf(payload)
	switch val.Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Interface:
		if val.IsNil() {
			return fmt.Errorf("%s payload is nil", eventType)
		}
	}

	resolvedUserId := userId
	if userId != "" && s.identity != nil {
		if resolved, err := s.identity.SubForUUID(userId); err != nil {
			s.slogger.Error("Failed to resolve actor identity for gateway event", "eventType", eventType, "error", err)
		} else {
			resolvedUserId = resolved
		}
	}

	eventDTO := dto.GatewayEventDTO{
		Type:          eventType,
		Payload:       payload,
		Timestamp:     time.Now().Format(time.RFC3339),
		CorrelationID: correlationID,
		UserId:        resolvedUserId,
	}

	eventJSON, err := json.Marshal(eventDTO)
	if err != nil {
		return fmt.Errorf("failed to marshal %s event: %w", eventType, err)
	}
	if len(eventJSON) > MaxEventPayloadSize {
		return fmt.Errorf("%s payload exceeds maximum size: %d (limit: %d)", eventType, len(eventJSON), MaxEventPayloadSize)
	}

	hubEvent := eventhub.Event{
		GatewayID:           gatewayID,
		OriginatedTimestamp: time.Now(),
		EventType:           eventTypePlatformGateway,
		Action:              actionForEventType(eventType),
		EntityID:            gatewayID,
		EventID:             correlationID,
		EventData:           string(eventJSON),
	}

	if err := s.hub.PublishEvent(gatewayID, hubEvent); err != nil {
		s.slogger.Error("Failed to publish gateway event",
			"gatewayID", gatewayID,
			"eventType", eventType,
			"correlationID", correlationID,
			"error", err,
		)
		return fmt.Errorf("failed to publish %s event: %w", eventType, err)
	}

	s.slogger.Debug("Published gateway event", "gatewayID", gatewayID, "type", eventType, "correlationID", correlationID)
	return nil
}

// actionForEventType maps a gateway event type string to the EventHub action field.
// The action column has a CHECK constraint of CREATE/UPDATE/DELETE.
func actionForEventType(eventType string) string {
	switch {
	case strings.HasSuffix(eventType, ".created"), strings.HasSuffix(eventType, ".deployed"):
		return "CREATE"
	case strings.HasSuffix(eventType, ".deleted"), strings.HasSuffix(eventType, ".undeployed"):
		return "DELETE"
	default:
		return "UPDATE"
	}
}
