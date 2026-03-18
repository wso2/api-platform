/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package eventlistener

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

// APIKeyXDSManager defines API key xDS operations used by the listener.
type APIKeyXDSManager interface {
	StoreAPIKey(apiId, apiName, apiVersion string, apiKey *models.APIKey, correlationID string) error
	RevokeAPIKey(apiId, apiName, apiVersion, apiKeyName, correlationID string) error
	RemoveAPIKeysByAPI(apiId, apiName, apiVersion, correlationID string) error
}

// EventListener listens for events from EventHub and processes them
// to keep the local replica synchronized with other replicas.
type EventListener struct {
	eventHub          eventhub.EventHub
	store             *storage.ConfigStore
	db                storage.Storage
	snapshotManager   *xds.SnapshotManager
	apiKeyXDSManager  APIKeyXDSManager
	policyManager     *policyxds.PolicyManager
	routerConfig      *config.RouterConfig
	logger            *slog.Logger
	systemConfig      *config.Config
	policyDefinitions map[string]api.PolicyDefinition

	eventCh <-chan eventhub.Event
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewEventListener creates a new EventListener
func NewEventListener(
	eventHub eventhub.EventHub,
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
	apiKeyXDSManager APIKeyXDSManager,
	policyManager *policyxds.PolicyManager,
	routerConfig *config.RouterConfig,
	logger *slog.Logger,
	systemConfig *config.Config,
	policyDefinitions map[string]api.PolicyDefinition,
) *EventListener {
	ctx, cancel := context.WithCancel(context.Background())
	return &EventListener{
		eventHub:          eventHub,
		store:             store,
		db:                db,
		snapshotManager:   snapshotManager,
		apiKeyXDSManager:  apiKeyXDSManager,
		policyManager:     policyManager,
		routerConfig:      routerConfig,
		logger:            logger,
		systemConfig:      systemConfig,
		policyDefinitions: policyDefinitions,
		ctx:               ctx,
		cancel:            cancel,
	}
}

// Start begins listening for events
func (l *EventListener) Start() error {
	if l.systemConfig == nil {
		return fmt.Errorf("event listener requires system configuration")
	}

	gatewayID := strings.TrimSpace(l.systemConfig.Controller.Server.GatewayID)
	if gatewayID == "" {
		return fmt.Errorf("event listener requires controller.server.gateway_id")
	}

	ch, err := l.eventHub.Subscribe(gatewayID)
	if err != nil {
		return err
	}
	l.eventCh = ch

	// Start processing goroutine
	go l.processEvents()

	l.logger.Info("Event listener started", slog.String("gateway_id", gatewayID))
	return nil
}

// Stop gracefully stops the event listener
func (l *EventListener) Stop() {
	l.cancel()
	if l.eventHub != nil && l.eventCh != nil && l.systemConfig != nil {
		gatewayID := strings.TrimSpace(l.systemConfig.Controller.Server.GatewayID)
		if gatewayID != "" {
			if err := l.eventHub.Unsubscribe(gatewayID, l.eventCh); err != nil {
				l.logger.Warn("Failed to unsubscribe event listener",
					slog.String("gateway_id", gatewayID),
					slog.Any("error", err))
			}
		}
	}
	l.logger.Info("Event listener stopped")
}

// processEvents handles incoming events from the EventHub subscription
func (l *EventListener) processEvents() {
	for {
		select {
		case <-l.ctx.Done():
			return
		case event, ok := <-l.eventCh:
			if !ok {
				l.logger.Info("Event channel closed, stopping event processing")
				return
			}
			l.processEventSafely(event)
		}
	}
}

// processEventSafely processes a single event and recovers from panics so
// the listener loop can continue processing subsequent events.
func (l *EventListener) processEventSafely(event eventhub.Event) {
	defer func() {
		if p := recover(); p != nil {
			l.logger.Error("Recovered from panic while processing event",
				slog.String("event_type", string(event.EventType)),
				slog.String("action", event.Action),
				slog.String("entity_id", event.EntityID),
				slog.String("event_id", event.EventID),
				slog.Any("panic", p),
				slog.String("stack_trace", string(debug.Stack())))
		}
	}()

	l.handleEvent(event)
}

// handleEvent dispatches events to the appropriate handler by event type
func (l *EventListener) handleEvent(event eventhub.Event) {
	l.logger.Info("Processing replica sync event",
		slog.String("event_type", string(event.EventType)),
		slog.String("action", event.Action),
		slog.String("entity_id", event.EntityID),
		slog.String("event_id", event.EventID))

	switch event.EventType {
	case eventhub.EventTypeAPI:
		l.processAPIEvent(event)
	case eventhub.EventTypeAPIKey:
		l.processAPIKeyEvent(event)
	case eventhub.EventTypeCertificate:
		l.logger.Info("Certificate event received (processing not yet implemented)",
			slog.String("entity_id", event.EntityID))
	case eventhub.EventTypeLLMTemplate:
		l.logger.Info("LLM template event received (processing not yet implemented)",
			slog.String("entity_id", event.EntityID))
	default:
		l.logger.Warn("Unknown event type received",
			slog.String("event_type", string(event.EventType)),
			slog.String("entity_id", event.EntityID))
	}
}
