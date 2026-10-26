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
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wso2/api-platform/common/eventhub"
	httpmiddleware "github.com/wso2/api-platform/httpkit/middleware"

	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/gwversion"
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

	EventTypeAgentDeployed   = "agent.deployed"
	EventTypeAgentUndeployed = "agent.undeployed"
	EventTypeAgentDeleted    = "agent.deleted"

	EventTypeWebSubAPIDeployed   = "websub.deployed"
	EventTypeWebSubAPIUndeployed = "websub.undeployed"
	EventTypeWebSubAPIDeleted    = "websub.deleted"

	EventTypeWebBrokerAPIDeployed   = "webbroker.deployed"
	EventTypeWebBrokerAPIUndeployed = "webbroker.undeployed"
	EventTypeWebBrokerAPIDeleted    = "webbroker.deleted"

	EventTypeGraphQLAPIDeployed   = "graphqlapi.deployed"
	EventTypeGraphQLAPIUndeployed = "graphqlapi.undeployed"
	EventTypeGraphQLAPIDeleted    = "graphqlapi.deleted"

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
func (s *GatewayEventsService) BroadcastDeploymentEvent(ctx context.Context, gatewayID string, deployment *model.DeploymentEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeAPIDeployed, deployment)
}

// BroadcastUndeploymentEvent sends an API undeployment event to target gateway.
func (s *GatewayEventsService) BroadcastUndeploymentEvent(ctx context.Context, gatewayID string, undeployment *model.APIUndeploymentEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeAPIUndeployed, undeployment)
}

// BroadcastAPIDeletionEvent sends an API deletion event to target gateway.
func (s *GatewayEventsService) BroadcastAPIDeletionEvent(ctx context.Context, gatewayID string, deletion *model.APIDeletionEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeAPIDeleted, deletion)
}

// BroadcastLLMProviderDeploymentEvent sends an LLM provider deployment event to target gateway.
func (s *GatewayEventsService) BroadcastLLMProviderDeploymentEvent(ctx context.Context, gatewayID string, deployment *model.LLMProviderDeploymentEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeLLMProviderDeployed, deployment)
}

// BroadcastLLMProviderUndeploymentEvent sends an LLM provider undeployment event to target gateway.
func (s *GatewayEventsService) BroadcastLLMProviderUndeploymentEvent(ctx context.Context, gatewayID string, undeployment *model.LLMProviderUndeploymentEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeLLMProviderUndeployed, undeployment)
}

// BroadcastLLMProxyDeploymentEvent sends an LLM proxy deployment event to target gateway.
func (s *GatewayEventsService) BroadcastLLMProxyDeploymentEvent(ctx context.Context, gatewayID string, deployment *model.LLMProxyDeploymentEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeLLMProxyDeployed, deployment)
}

// BroadcastLLMProxyUndeploymentEvent sends an LLM proxy undeployment event to target gateway.
func (s *GatewayEventsService) BroadcastLLMProxyUndeploymentEvent(ctx context.Context, gatewayID string, undeployment *model.LLMProxyUndeploymentEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeLLMProxyUndeployed, undeployment)
}

// BroadcastMCPProxyDeploymentEvent sends an MCP proxy deployment event to target gateway.
func (s *GatewayEventsService) BroadcastMCPProxyDeploymentEvent(ctx context.Context, gatewayID string, deployment *model.MCPProxyDeploymentEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeMCPProxyDeployed, deployment)
}

// BroadcastMCPProxyUndeploymentEvent sends an MCP proxy undeployment event to target gateway.
func (s *GatewayEventsService) BroadcastMCPProxyUndeploymentEvent(ctx context.Context, gatewayID string, undeployment *model.MCPProxyUndeploymentEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeMCPProxyUndeployed, undeployment)
}

// BroadcastMCPProxyDeletionEvent sends an MCP proxy deletion event to target gateway.
func (s *GatewayEventsService) BroadcastMCPProxyDeletionEvent(ctx context.Context, gatewayID string, deletion *model.MCPProxyDeletionEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeMCPProxyDeleted, deletion)
}

// BroadcastAgentDeploymentEvent sends an Agent proxy deployment event to target gateway.
// Restoring a deployment sends the same event.
func (s *GatewayEventsService) BroadcastAgentDeploymentEvent(ctx context.Context, gatewayID string, deployment *model.AgentDeploymentEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeAgentDeployed, deployment)
}

// BroadcastAgentUndeploymentEvent sends an Agent proxy undeployment event to target gateway.
func (s *GatewayEventsService) BroadcastAgentUndeploymentEvent(ctx context.Context, gatewayID string, undeployment *model.AgentUndeploymentEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeAgentUndeployed, undeployment)
}

// BroadcastAgentDeletionEvent sends an Agent proxy deletion event to target gateway.
func (s *GatewayEventsService) BroadcastAgentDeletionEvent(ctx context.Context, gatewayID string, deletion *model.AgentDeletionEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeAgentDeleted, deletion)
}

// BroadcastWebSubAPIDeploymentEvent sends a WebSub API deployment event to target gateway.
func (s *GatewayEventsService) BroadcastWebSubAPIDeploymentEvent(ctx context.Context, gatewayID string, deployment *model.WebSubAPIDeploymentEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeWebSubAPIDeployed, deployment)
}

// BroadcastWebSubAPIUndeploymentEvent sends a WebSub API undeployment event to target gateway.
func (s *GatewayEventsService) BroadcastWebSubAPIUndeploymentEvent(ctx context.Context, gatewayID string, undeployment *model.WebSubAPIUndeploymentEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeWebSubAPIUndeployed, undeployment)
}

// BroadcastWebSubAPIDeletionEvent sends a WebSub API deletion event to target gateway.
func (s *GatewayEventsService) BroadcastWebSubAPIDeletionEvent(ctx context.Context, gatewayID string, deletion *model.WebSubAPIDeletionEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeWebSubAPIDeleted, deletion)
}

// BroadcastWebBrokerAPIDeploymentEvent sends a WebBroker API deployment event to target gateway.
func (s *GatewayEventsService) BroadcastWebBrokerAPIDeploymentEvent(ctx context.Context, gatewayID string, deployment *model.WebBrokerAPIDeploymentEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeWebBrokerAPIDeployed, deployment)
}

// BroadcastWebBrokerAPIUndeploymentEvent sends a WebBroker API undeployment event to target gateway.
func (s *GatewayEventsService) BroadcastWebBrokerAPIUndeploymentEvent(ctx context.Context, gatewayID string, undeployment *model.WebBrokerAPIUndeploymentEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeWebBrokerAPIUndeployed, undeployment)
}

// BroadcastWebBrokerAPIDeletionEvent sends a WebBroker API deletion event to target gateway.
func (s *GatewayEventsService) BroadcastWebBrokerAPIDeletionEvent(ctx context.Context, gatewayID string, deletion *model.WebBrokerAPIDeletionEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeWebBrokerAPIDeleted, deletion)
}

// BroadcastGraphQLAPIDeploymentEvent sends a GraphQL API deployment event to target gateway.
func (s *GatewayEventsService) BroadcastGraphQLAPIDeploymentEvent(ctx context.Context, gatewayID string, deployment *model.GraphQLAPIDeploymentEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeGraphQLAPIDeployed, deployment)
}

// BroadcastGraphQLAPIUndeploymentEvent sends a GraphQL API undeployment event to target gateway.
func (s *GatewayEventsService) BroadcastGraphQLAPIUndeploymentEvent(ctx context.Context, gatewayID string, undeployment *model.GraphQLAPIUndeploymentEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeGraphQLAPIUndeployed, undeployment)
}

// BroadcastGraphQLAPIDeletionEvent sends a GraphQL API deletion event to target gateway.
func (s *GatewayEventsService) BroadcastGraphQLAPIDeletionEvent(ctx context.Context, gatewayID string, deletion *model.GraphQLAPIDeletionEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeGraphQLAPIDeleted, deletion)
}

// BroadcastLLMProviderDeletionEvent sends an LLM provider deletion event to target gateway.
func (s *GatewayEventsService) BroadcastLLMProviderDeletionEvent(ctx context.Context, gatewayID string, deletion *model.LLMProviderDeletionEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeLLMProviderDeleted, deletion)
}

// BroadcastLLMProxyDeletionEvent sends an LLM proxy deletion event to target gateway.
func (s *GatewayEventsService) BroadcastLLMProxyDeletionEvent(ctx context.Context, gatewayID string, deletion *model.LLMProxyDeletionEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeLLMProxyDeleted, deletion)
}

// BroadcastWebSubAPIHmacSecretEvent sends a WebSub API HMAC secret lifecycle event to target gateway.
// action should be "CREATED", "UPDATED", or "DELETED".
func (s *GatewayEventsService) BroadcastWebSubAPIHmacSecretEvent(ctx context.Context, gatewayID, action string, event *model.WebSubAPIHmacSecretEvent) error {
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
	return s.broadcastEvent(ctx, gatewayID, eventType, event)
}

// BroadcastAPIKeyCreatedEvent sends an API key created event to target gateway.
func (s *GatewayEventsService) BroadcastAPIKeyCreatedEvent(ctx context.Context, gatewayID, userId string, event *model.APIKeyCreatedEvent) error {
	return s.broadcastEventWithUserID(ctx, gatewayID, userId, EventTypeAPIKeyCreated, event)
}

// BroadcastAPIKeyRevokedEvent sends an API key revoked event to target gateway.
func (s *GatewayEventsService) BroadcastAPIKeyRevokedEvent(ctx context.Context, gatewayID, userId string, event *model.APIKeyRevokedEvent) error {
	return s.broadcastEventWithUserID(ctx, gatewayID, userId, EventTypeAPIKeyRevoked, event)
}

// BroadcastAPIKeyUpdatedEvent sends an API key updated event to target gateway.
func (s *GatewayEventsService) BroadcastAPIKeyUpdatedEvent(ctx context.Context, gatewayID, userId string, event *model.APIKeyUpdatedEvent) error {
	return s.broadcastEventWithUserID(ctx, gatewayID, userId, EventTypeAPIKeyUpdated, event)
}

// BroadcastAPIKeyRevokedEventFor sends an API key revoked event to gateway gw,
// stamped with the identity that gateway's release accepts (see KeyEventActor).
// keyCreatedBy is the revoked key's creator; userId is the caller platform-api
// has already authorized.
func (s *GatewayEventsService) BroadcastAPIKeyRevokedEventFor(ctx context.Context, gw *model.APIGatewayWithDetails, keyCreatedBy, userId string, event *model.APIKeyRevokedEvent) error {
	actor, err := s.KeyEventActor(gw.Version, keyCreatedBy, userId)
	if err != nil {
		return fmt.Errorf("%s event for gateway %s: %w", EventTypeAPIKeyRevoked, gw.ID, err)
	}
	return s.broadcastEventWithUserID(ctx, gw.ID, actor, EventTypeAPIKeyRevoked, event)
}

// BroadcastAPIKeyUpdatedEventFor is BroadcastAPIKeyRevokedEventFor for an
// updated event.
func (s *GatewayEventsService) BroadcastAPIKeyUpdatedEventFor(ctx context.Context, gw *model.APIGatewayWithDetails, keyCreatedBy, userId string, event *model.APIKeyUpdatedEvent) error {
	actor, err := s.KeyEventActor(gw.Version, keyCreatedBy, userId)
	if err != nil {
		return fmt.Errorf("%s event for gateway %s: %w", EventTypeAPIKeyUpdated, gw.ID, err)
	}
	return s.broadcastEventWithUserID(ctx, gw.ID, actor, EventTypeAPIKeyUpdated, event)
}

// KeyEventActor returns the user id to stamp on an API key revoke/update event
// for a gateway that reported gatewayVersion, leaving the decision to
// gatewaytranslator.KeyEventActor, which knows what each gateway release
// expects. Only gateways that re-check ownership need the key creator's IdP
// subject; if it cannot be resolved for one of them the error is returned and
// nothing is published, because the fallback (the creator's UUID) resolves to
// the migration actor for a key Platform API v1 created without a creator, an
// identity the gateway rejects while the caller is told the event was sent.
func (s *GatewayEventsService) KeyEventActor(gatewayVersion, keyCreatedBy, userId string) (string, error) {
	if !gwversion.RechecksKeyEventOwnership(gatewayVersion) {
		return gatewaytranslator.KeyEventActor(gatewayVersion, keyCreatedBy, "", userId), nil
	}
	creatorSub := ""
	if keyCreatedBy != "" && s.identity != nil {
		sub, err := s.identity.SubForUUID(keyCreatedBy)
		if err != nil {
			return "", fmt.Errorf("resolve the API key creator for a gateway that re-checks key ownership: %w", err)
		}
		creatorSub = sub
	}
	return gatewaytranslator.KeyEventActor(gatewayVersion, keyCreatedBy, creatorSub, userId), nil
}

// BroadcastApplicationUpdatedEvent sends an application updated event to target gateway.
func (s *GatewayEventsService) BroadcastApplicationUpdatedEvent(ctx context.Context, gatewayID, userId string, event *model.ApplicationUpdatedEvent) error {
	return s.broadcastEventWithUserID(ctx, gatewayID, userId, "application.updated", event)
}

// BroadcastSubscriptionCreatedEvent sends a subscription.created event to the target gateway.
func (s *GatewayEventsService) BroadcastSubscriptionCreatedEvent(ctx context.Context, gatewayID string, event *model.SubscriptionCreatedEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeSubscriptionCreated, event)
}

// BroadcastSubscriptionUpdatedEvent sends a subscription.updated event to the target gateway.
func (s *GatewayEventsService) BroadcastSubscriptionUpdatedEvent(ctx context.Context, gatewayID string, event *model.SubscriptionUpdatedEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeSubscriptionUpdated, event)
}

// BroadcastSubscriptionDeletedEvent sends a subscription.deleted event to the target gateway.
func (s *GatewayEventsService) BroadcastSubscriptionDeletedEvent(ctx context.Context, gatewayID string, event *model.SubscriptionDeletedEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeSubscriptionDeleted, event)
}

// BroadcastSubscriptionPlanCreatedEvent sends a subscriptionPlan.created event to the target gateway.
func (s *GatewayEventsService) BroadcastSubscriptionPlanCreatedEvent(ctx context.Context, gatewayID string, event *model.SubscriptionPlanCreatedEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeSubscriptionPlanCreated, event)
}

// BroadcastSubscriptionPlanUpdatedEvent sends a subscriptionPlan.updated event to the target gateway.
func (s *GatewayEventsService) BroadcastSubscriptionPlanUpdatedEvent(ctx context.Context, gatewayID string, event *model.SubscriptionPlanUpdatedEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeSubscriptionPlanUpdated, event)
}

// BroadcastSubscriptionPlanDeletedEvent sends a subscriptionPlan.deleted event to the target gateway.
func (s *GatewayEventsService) BroadcastSubscriptionPlanDeletedEvent(ctx context.Context, gatewayID string, event *model.SubscriptionPlanDeletedEvent) error {
	return s.broadcastEvent(ctx, gatewayID, EventTypeSubscriptionPlanDeleted, event)
}

// broadcastEvent is the generic helper for broadcasting gateway events without a userId.
func (s *GatewayEventsService) broadcastEvent(ctx context.Context, gatewayID, eventType string, payload interface{}) error {
	return s.broadcastEventWithUserID(ctx, gatewayID, "", eventType, payload)
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
//
// Every event gets its own ID, sent as both the EventHub event ID and the payload's
// correlationId: the event store and gateways treat it as unique per event, and one
// request can send several events to the same gateway (a deploy and its API key
// backfill). The request's correlation ID is logged next to it, which links an audit
// log row to the gateway logs for the event.
func (s *GatewayEventsService) broadcastEventWithUserID(ctx context.Context, gatewayID, userId, eventType string, payload interface{}) error {
	eventID := uuid.New().String()
	requestCorrelationID := httpmiddleware.GetCorrelationIDFromContext(ctx)

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
		CorrelationID: eventID,
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
		EventID:             eventID,
		EventData:           string(eventJSON),
	}

	if err := s.hub.PublishEvent(gatewayID, hubEvent); err != nil {
		s.slogger.Error("Failed to publish gateway event",
			"gatewayID", gatewayID,
			"eventType", eventType,
			"eventID", eventID,
			"requestCorrelationID", requestCorrelationID,
			"error", err,
		)
		return fmt.Errorf("failed to publish %s event: %w", eventType, err)
	}

	s.slogger.Debug("Published gateway event", "gatewayID", gatewayID, "type", eventType,
		"eventID", eventID, "requestCorrelationID", requestCorrelationID)
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
