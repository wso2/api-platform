/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package service

import (
	"errors"
	"fmt"
	"log/slog"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
)

// SubscriptionService handles application-level subscription business logic
type SubscriptionService struct {
	apiRepo          repository.APIRepository
	subscriptionRepo repository.SubscriptionRepository
	gatewayEvents    *GatewayEventsService
	slogger          *slog.Logger
}

// NewSubscriptionService creates a new subscription service
func NewSubscriptionService(
	apiRepo repository.APIRepository,
	subscriptionRepo repository.SubscriptionRepository,
	gatewayEvents *GatewayEventsService,
	slogger *slog.Logger,
) *SubscriptionService {
	if slogger == nil {
		slogger = slog.Default()
	}
	return &SubscriptionService{
		apiRepo:          apiRepo,
		subscriptionRepo: subscriptionRepo,
		gatewayEvents:    gatewayEvents,
		slogger:          slogger,
	}
}

// resolveAPIUUID resolves apiId (handle or UUID) to rest_apis.uuid for the organization
func (s *SubscriptionService) resolveAPIUUID(apiId, orgUUID string) (string, error) {
	if apiId == "" {
		return "", constants.ErrAPINotFound
	}
	apiModel, err := s.apiRepo.GetAPIByUUID(apiId, orgUUID)
	if err != nil {
		if !errors.Is(err, constants.ErrAPINotFound) {
			return "", fmt.Errorf("failed to resolve API by UUID: %w", err)
		}
	} else if apiModel != nil {
		return apiModel.ID, nil
	}

	metadata, err := s.apiRepo.GetAPIMetadataByHandle(apiId, orgUUID)
	if err != nil {
		if errors.Is(err, constants.ErrAPINotFound) {
			return "", constants.ErrAPINotFound
		}
		return "", fmt.Errorf("failed to resolve API by handle: %w", err)
	}
	if metadata == nil {
		return "", constants.ErrAPINotFound
	}
	return metadata.ID, nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ResolveAPIHandle returns the API handle for display (apiId in responses should be the handle, not UUID).
func (s *SubscriptionService) ResolveAPIHandle(apiUUID, orgUUID string) string {
	if apiUUID == "" {
		return ""
	}
	api, err := s.apiRepo.GetAPIByUUID(apiUUID, orgUUID)
	if err != nil || api == nil {
		return apiUUID // fallback to UUID if lookup fails
	}
	return api.Handle
}

// GetAPIHandleMap returns a map of API UUID to handle for bulk lookup (avoids N+1 queries).
func (s *SubscriptionService) GetAPIHandleMap(uuids []string, orgUUID string) (map[string]string, error) {
	return s.apiRepo.GetAPIsByUUIDs(uuids, orgUUID)
}

// CreateSubscription creates a new subscription for an API
func (s *SubscriptionService) CreateSubscription(apiId, orgUUID string, applicationId *string, subscriptionPlanId *string, status string) (*model.Subscription, error) {
	apiUUID, err := s.resolveAPIUUID(apiId, orgUUID)
	if err != nil {
		return nil, err
	}
	st := model.SubscriptionStatusActive
	if status != "" {
		st = model.SubscriptionStatus(status)
		switch st {
		case model.SubscriptionStatusActive, model.SubscriptionStatusInactive, model.SubscriptionStatusRevoked:
		default:
			return nil, fmt.Errorf("invalid status: %s", status)
		}
	}

	appID := derefString(applicationId)
	exists, err := s.subscriptionRepo.ExistsByAPIAndApplication(apiUUID, appID, orgUUID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, constants.ErrSubscriptionAlreadyExists
	}

	sub := &model.Subscription{
		APIUUID:            apiUUID,
		ApplicationID:      applicationId,
		SubscriptionPlanID: subscriptionPlanId,
		OrganizationUUID:   orgUUID,
		Status:             st,
	}
	if err := s.subscriptionRepo.Create(sub); err != nil {
		return nil, err
	}

	if s.gatewayEvents != nil {
		gateways, err := s.apiRepo.GetAPIGatewaysWithDetails(apiUUID, orgUUID)
		if err != nil {
			s.slogger.Warn("Failed to load gateways for subscription.created broadcast",
				"apiId", apiUUID, "subscriptionId", sub.UUID, "error", err)
		} else {
			event := &model.SubscriptionCreatedEvent{
				ApiId:              apiUUID,
				SubscriptionId:     sub.UUID,
				ApplicationId:      derefString(sub.ApplicationID),
				SubscriptionToken:  sub.SubscriptionToken,
				SubscriptionPlanId: derefString(sub.SubscriptionPlanID),
				Status:             string(sub.Status),
			}
			for _, gw := range gateways {
				if gw == nil || gw.ID == "" {
					continue
				}
				if err := s.gatewayEvents.BroadcastSubscriptionCreatedEvent(gw.ID, event); err != nil {
					s.slogger.Warn("Failed to broadcast subscription.created event",
						"gatewayId", gw.ID, "subscriptionId", sub.UUID, "error", err)
				}
			}
		}
	}

	return sub, nil
}

// GetSubscription returns a subscription by ID (and org)
func (s *SubscriptionService) GetSubscription(subscriptionId, orgUUID string) (*model.Subscription, error) {
	sub, err := s.subscriptionRepo.GetByID(subscriptionId, orgUUID)
	if err != nil {
		if errors.Is(err, constants.ErrSubscriptionNotFound) {
			return nil, constants.ErrSubscriptionNotFound
		}
		return nil, err
	}
	if sub == nil {
		return nil, constants.ErrSubscriptionNotFound
	}
	return sub, nil
}

// ListSubscriptionsByFilters returns subscriptions filtered by API and/or application with pagination.
// It returns the list, total count matching filters, and any error.
func (s *SubscriptionService) ListSubscriptionsByFilters(orgUUID string, apiId *string, applicationID *string, status *string, limit, offset int) ([]*model.Subscription, int, error) {
	var apiUUID *string
	if apiId != nil && *apiId != "" {
		resolved, err := s.resolveAPIUUID(*apiId, orgUUID)
		if err != nil {
			return nil, 0, err
		}
		apiUUID = &resolved
	}
	list, err := s.subscriptionRepo.ListByFilters(orgUUID, apiUUID, applicationID, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.subscriptionRepo.CountByFilters(orgUUID, apiUUID, applicationID, status)
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// UpdateSubscription updates a subscription (e.g. status)
func (s *SubscriptionService) UpdateSubscription(subscriptionId, orgUUID, status string) (*model.Subscription, error) {
	sub, err := s.subscriptionRepo.GetByID(subscriptionId, orgUUID)
	if err != nil {
		if errors.Is(err, constants.ErrSubscriptionNotFound) {
			return nil, constants.ErrSubscriptionNotFound
		}
		return nil, err
	}
	if sub == nil {
		return nil, constants.ErrSubscriptionNotFound
	}
	if status != "" {
		st := model.SubscriptionStatus(status)
		switch st {
		case model.SubscriptionStatusActive, model.SubscriptionStatusInactive, model.SubscriptionStatusRevoked:
			sub.Status = st
		default:
			return nil, fmt.Errorf("invalid status: %s", status)
		}
	}
	if err := s.subscriptionRepo.Update(sub); err != nil {
		return nil, err
	}

	if s.gatewayEvents != nil {
		gateways, err := s.apiRepo.GetAPIGatewaysWithDetails(sub.APIUUID, orgUUID)
		if err != nil {
			s.slogger.Warn("Failed to load gateways for subscription.updated broadcast",
				"apiId", sub.APIUUID, "subscriptionId", sub.UUID, "error", err)
		} else {
			event := &model.SubscriptionUpdatedEvent{
				ApiId:              sub.APIUUID,
				SubscriptionId:     sub.UUID,
				ApplicationId:      derefString(sub.ApplicationID),
				SubscriptionToken:  sub.SubscriptionToken,
				SubscriptionPlanId: derefString(sub.SubscriptionPlanID),
				Status:             string(sub.Status),
			}
			for _, gw := range gateways {
				if gw == nil || gw.ID == "" {
					continue
				}
				if err := s.gatewayEvents.BroadcastSubscriptionUpdatedEvent(gw.ID, event); err != nil {
					s.slogger.Warn("Failed to broadcast subscription.updated event",
						"gatewayId", gw.ID, "subscriptionId", sub.UUID, "error", err)
				}
			}
		}
	}

	return sub, nil
}

// DeleteSubscription removes a subscription
func (s *SubscriptionService) DeleteSubscription(subscriptionId, orgUUID string) error {
	sub, err := s.subscriptionRepo.GetByID(subscriptionId, orgUUID)
	if err != nil {
		if errors.Is(err, constants.ErrSubscriptionNotFound) {
			return constants.ErrSubscriptionNotFound
		}
		return err
	}
	if sub == nil {
		return constants.ErrSubscriptionNotFound
	}

	if err := s.subscriptionRepo.Delete(subscriptionId, orgUUID); err != nil {
		return err
	}

	if s.gatewayEvents != nil {
		gateways, err := s.apiRepo.GetAPIGatewaysWithDetails(sub.APIUUID, orgUUID)
		if err != nil {
			s.slogger.Warn("Failed to load gateways for subscription.deleted event",
				"apiId", sub.APIUUID, "subscriptionId", sub.UUID, "error", err)
			return nil
		}
		event := &model.SubscriptionDeletedEvent{
			ApiId:             sub.APIUUID,
			SubscriptionId:    sub.UUID,
			ApplicationId:     derefString(sub.ApplicationID),
			SubscriptionToken: sub.SubscriptionToken,
		}
		for _, gw := range gateways {
			if gw == nil || gw.ID == "" {
				continue
			}
			if err := s.gatewayEvents.BroadcastSubscriptionDeletedEvent(gw.ID, event); err != nil {
				s.slogger.Warn("Failed to broadcast subscription.deleted event",
					"gatewayId", gw.ID, "subscriptionId", sub.UUID, "error", err)
			}
		}
	}
	return nil
}
