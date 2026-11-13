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

package utils

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// UndeployEnvRefusedAtStartup marks a stored artifact whose spec reads a refused
// environment variable as undeployed, so the router translator skips its route,
// and hands it to onEnvRefused to persist the change. The artifact is never added
// to the runtime store, so it has no policy chain either. An artifact that is
// already undeployed needs no further change.
func UndeployEnvRefusedAtStartup(cfg *models.StoredConfig, onEnvRefused func(*models.StoredConfig) error, log *slog.Logger) {
	if cfg.DesiredState == models.StateUndeployed {
		return
	}
	if log != nil {
		log.Warn("Undeploying artifact: its spec reads an environment variable outside policy_engine.env_access.allow_list",
			slog.String("id", cfg.UUID),
			slog.String("kind", cfg.Kind),
			slog.String("handle", cfg.Handle))
	}
	cfg.DesiredState = models.StateUndeployed
	if onEnvRefused == nil {
		return
	}
	if err := onEnvRefused(cfg); err != nil && log != nil {
		log.Error("Failed to persist undeploy of artifact that reads a refused environment variable; it stays undeployed for this run only",
			slog.String("id", cfg.UUID),
			slog.String("kind", cfg.Kind),
			slog.String("handle", cfg.Handle),
			slog.Any("error", err))
	}
}

// ConfigUpdater persists an updated artifact.
type ConfigUpdater interface {
	UpdateConfig(cfg *models.StoredConfig) error
}

// EventPublisher publishes a replica-sync event.
type EventPublisher interface {
	PublishEvent(gatewayID string, event eventhub.Event) error
}

// NewEnvRefusedUndeployer returns the startup callback that persists the undeploy
// of an artifact whose spec reads a refused environment variable. It stores
// desired_state=undeployed (marking a gateway-originated artifact pending so the
// bottom-up sync also undeploys it in on-prem APIM) and publishes an UPDATE event
// so running replicas drop the artifact too. publisher may be nil.
func NewEnvRefusedUndeployer(db ConfigUpdater, publisher EventPublisher, gatewayID string, log *slog.Logger) func(*models.StoredConfig) error {
	return func(cfg *models.StoredConfig) error {
		cfg.DesiredState = models.StateUndeployed
		cfg.CPSyncStatus = cpSyncStatusForOrigin(cfg.Origin)
		cfg.UpdatedAt = time.Now()
		if err := db.UpdateConfig(cfg); err != nil {
			return fmt.Errorf("failed to persist undeploy of %s %q: %w", cfg.Kind, cfg.Handle, err)
		}

		eventType, ok := replicaSyncEventType(cfg.Kind)
		if publisher == nil || !ok {
			return nil
		}
		event := eventhub.Event{
			GatewayID:           gatewayID,
			OriginatedTimestamp: time.Now(),
			EventType:           eventType,
			Action:              "UPDATE",
			EntityID:            cfg.UUID,
			EventID:             uuid.New().String(),
			EventData:           eventhub.EmptyEventData,
		}
		if err := publisher.PublishEvent(gatewayID, event); err != nil && log != nil {
			log.Warn("Failed to publish undeploy event for artifact that reads a refused environment variable",
				slog.String("id", cfg.UUID),
				slog.String("kind", cfg.Kind),
				slog.Any("error", err))
		}
		return nil
	}
}

// replicaSyncEventType maps an artifact kind to the event type replicas listen on.
func replicaSyncEventType(kind string) (eventhub.EventType, bool) {
	switch kind {
	case models.KindRestApi, models.KindGraphQLApi:
		return eventhub.EventTypeAPI, true
	case models.KindLlmProvider:
		return eventhub.EventTypeLLMProvider, true
	case models.KindLlmProxy:
		return eventhub.EventTypeLLMProxy, true
	case models.KindMcp:
		return eventhub.EventTypeMCPProxy, true
	case models.KindAgent:
		return eventhub.EventTypeAgent, true
	default:
		return "", false
	}
}
