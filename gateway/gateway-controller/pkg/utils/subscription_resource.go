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
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// SubscriptionSnapshotUpdater captures the subscription xDS refresh surface used by replica-synced writers.
type SubscriptionSnapshotUpdater interface {
	UpdateSnapshot(ctx context.Context) error
}

// SubscriptionResourceService persists subscription resources and coordinates replica-sync publishing.
type SubscriptionResourceService struct {
	db              storage.Storage
	snapshotUpdater SubscriptionSnapshotUpdater
	eventHub        eventhub.EventHub
	gatewayID       string
}

// NewSubscriptionResourceService creates a new service for subscription-related resources.
func NewSubscriptionResourceService(db storage.Storage, snapshotUpdater SubscriptionSnapshotUpdater) *SubscriptionResourceService {
	if db == nil {
		panic("SubscriptionResourceService requires non-nil storage")
	}

	return &SubscriptionResourceService{
		db:              db,
		snapshotUpdater: snapshotUpdater,
	}
}

// SetEventHub configures EventHub publishing for replica-synced subscription resources.
func (s *SubscriptionResourceService) SetEventHub(eventHub eventhub.EventHub, gatewayID string) {
	s.eventHub = eventHub
	s.gatewayID = gatewayID
}

// SaveSubscription stores a new subscription and publishes a replica-sync event after the DB write succeeds.
func (s *SubscriptionResourceService) SaveSubscription(sub *models.Subscription, correlationID string, logger *slog.Logger) error {
	return s.persistAndSync(eventhub.EventTypeSubscription, "CREATE", sub.ID, true, correlationID, logger, func() error {
		return s.requireDB().SaveSubscription(sub)
	})
}

// UpsertSubscription stores or updates a subscription and publishes the supplied action.
func (s *SubscriptionResourceService) UpsertSubscription(sub *models.Subscription, action, correlationID string, logger *slog.Logger) error {
	return s.persistAndSync(eventhub.EventTypeSubscription, action, sub.ID, true, correlationID, logger, func() error {
		db := s.requireDB()
		switch action {
		case "UPDATE":
			return db.UpdateSubscription(sub)
		default:
			if err := db.SaveSubscription(sub); err != nil {
				if !storage.IsConflictError(err) {
					return err
				}

				existing, getErr := db.GetSubscriptionByID(sub.ID, "")
				if getErr != nil {
					return getErr
				}

				copySubscriptionFields(existing, sub)
				return db.UpdateSubscription(existing)
			}
			return nil
		}
	})
}

// UpdateSubscription updates an existing subscription and publishes a replica-sync event after the DB write succeeds.
func (s *SubscriptionResourceService) UpdateSubscription(sub *models.Subscription, correlationID string, logger *slog.Logger) error {
	return s.persistAndSync(eventhub.EventTypeSubscription, "UPDATE", sub.ID, true, correlationID, logger, func() error {
		return s.requireDB().UpdateSubscription(sub)
	})
}

// DeleteSubscription removes a subscription and publishes a replica-sync event after the DB write succeeds.
func (s *SubscriptionResourceService) DeleteSubscription(id, correlationID string, logger *slog.Logger) error {
	return s.persistAndSync(eventhub.EventTypeSubscription, "DELETE", id, true, correlationID, logger, func() error {
		return s.requireDB().DeleteSubscription(id, "")
	})
}

// SaveSubscriptionPlan stores a new subscription plan and publishes a replica-sync event after the DB write succeeds.
func (s *SubscriptionResourceService) SaveSubscriptionPlan(plan *models.SubscriptionPlan, correlationID string, logger *slog.Logger) error {
	return s.persistAndSync(eventhub.EventTypeSubscriptionPlan, "CREATE", plan.ID, true, correlationID, logger, func() error {
		return s.requireDB().SaveSubscriptionPlan(plan)
	})
}

// UpsertSubscriptionPlan stores or updates a subscription plan and publishes the supplied action.
func (s *SubscriptionResourceService) UpsertSubscriptionPlan(plan *models.SubscriptionPlan, action, correlationID string, logger *slog.Logger) error {
	return s.persistAndSync(eventhub.EventTypeSubscriptionPlan, action, plan.ID, true, correlationID, logger, func() error {
		db := s.requireDB()
		switch action {
		case "UPDATE":
			return db.UpdateSubscriptionPlan(plan)
		default:
			if err := db.SaveSubscriptionPlan(plan); err != nil {
				if !storage.IsConflictError(err) {
					return err
				}
				return db.UpdateSubscriptionPlan(plan)
			}
			return nil
		}
	})
}

// UpdateSubscriptionPlan updates an existing subscription plan and publishes a replica-sync event after the DB write succeeds.
func (s *SubscriptionResourceService) UpdateSubscriptionPlan(plan *models.SubscriptionPlan, correlationID string, logger *slog.Logger) error {
	return s.persistAndSync(eventhub.EventTypeSubscriptionPlan, "UPDATE", plan.ID, true, correlationID, logger, func() error {
		return s.requireDB().UpdateSubscriptionPlan(plan)
	})
}

// DeleteSubscriptionPlan removes a subscription plan and publishes a replica-sync event after the DB write succeeds.
func (s *SubscriptionResourceService) DeleteSubscriptionPlan(id, correlationID string, logger *slog.Logger) error {
	return s.persistAndSync(eventhub.EventTypeSubscriptionPlan, "DELETE", id, true, correlationID, logger, func() error {
		return s.requireDB().DeleteSubscriptionPlan(id, "")
	})
}

// ReplaceApplicationAPIKeyMappings persists canonical application metadata and publishes a replica-sync event.
func (s *SubscriptionResourceService) ReplaceApplicationAPIKeyMappings(application *models.StoredApplication, mappings []*models.ApplicationAPIKeyMapping, correlationID string, logger *slog.Logger) error {
	return s.persistAndSync(eventhub.EventTypeApplication, "UPDATE", application.ApplicationUUID, false, correlationID, logger, func() error {
		return s.requireDB().ReplaceApplicationAPIKeyMappings(application, mappings)
	})
}

func (s *SubscriptionResourceService) requireDB() storage.Storage {
	return s.db
}

func (s *SubscriptionResourceService) requireReplicaSyncDependencies() error {
	if s.eventHub == nil {
		return fmt.Errorf("SubscriptionResourceService requires EventHub")
	}
	if strings.TrimSpace(s.gatewayID) == "" {
		return fmt.Errorf("SubscriptionResourceService requires gateway ID")
	}
	return nil
}

func (s *SubscriptionResourceService) persistAndSync(
	eventType eventhub.EventType,
	action string,
	entityID string,
	refreshSubscriptionSnapshot bool,
	correlationID string,
	logger *slog.Logger,
	persist func() error,
) error {
	if logger == nil {
		logger = slog.Default()
	}
	if err := s.requireReplicaSyncDependencies(); err != nil {
		return err
	}

	if err := persist(); err != nil {
		return err
	}

	s.publishEvent(eventType, action, entityID, correlationID, logger)
	return nil
}

func (s *SubscriptionResourceService) publishEvent(eventType eventhub.EventType, action, entityID, correlationID string, logger *slog.Logger) {
	event := eventhub.Event{
		GatewayID:           s.gatewayID,
		OriginatedTimestamp: time.Now(),
		EventType:           eventType,
		Action:              action,
		EntityID:            entityID,
		EventID:             correlationID,
		EventData:           eventhub.EmptyEventData,
	}
	if err := s.eventHub.PublishEvent(s.gatewayID, event); err != nil {
		logger.Error("Failed to publish subscription resource event",
			slog.String("event_type", string(eventType)),
			slog.String("action", action),
			slog.String("entity_id", entityID),
			slog.Any("error", err))
	}
}

func (s *SubscriptionResourceService) refreshSubscriptionSnapshot(entityID, correlationID string, logger *slog.Logger) error {
	if s.snapshotUpdater == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.snapshotUpdater.UpdateSnapshot(ctx); err != nil {
		logger.Error("Failed to refresh subscription snapshot after local write",
			slog.String("entity_id", entityID),
			slog.String("correlation_id", correlationID),
			slog.Any("error", err))
		return err
	}

	return nil
}

func copySubscriptionFields(dst, src *models.Subscription) {
	dst.APIID = src.APIID
	dst.ApplicationID = src.ApplicationID
	dst.SubscriptionToken = src.SubscriptionToken
	dst.SubscriptionTokenHash = src.SubscriptionTokenHash
	dst.SubscriptionPlanID = src.SubscriptionPlanID
	dst.Status = src.Status
	dst.UpdatedAt = src.UpdatedAt
}
