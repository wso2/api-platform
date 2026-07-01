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
	artifactRepo     repository.ArtifactRepository
	subscriptionRepo repository.SubscriptionRepository
	planRepo         repository.SubscriptionPlanRepository
	orgRepo          repository.OrganizationRepository
	gatewayEvents    *GatewayEventsService
	auditRepo        repository.AuditRepository
	slogger          *slog.Logger
}

// NewSubscriptionService creates a new subscription service
func NewSubscriptionService(
	apiRepo repository.APIRepository,
	artifactRepo repository.ArtifactRepository,
	subscriptionRepo repository.SubscriptionRepository,
	planRepo repository.SubscriptionPlanRepository,
	orgRepo repository.OrganizationRepository,
	gatewayEvents *GatewayEventsService,
	auditRepo repository.AuditRepository,
	slogger *slog.Logger,
) *SubscriptionService {
	if slogger == nil {
		slogger = slog.Default()
	}
	return &SubscriptionService{
		apiRepo:          apiRepo,
		artifactRepo:     artifactRepo,
		subscriptionRepo: subscriptionRepo,
		planRepo:         planRepo,
		orgRepo:          orgRepo,
		gatewayEvents:    gatewayEvents,
		auditRepo:        auditRepo,
		slogger:          slogger,
	}
}

// ResolveOrgHandle returns the organization handle for display (organizationId in
// responses should be the handle, not the internal UUID).
func (s *SubscriptionService) ResolveOrgHandle(orgUUID string) string {
	if orgUUID == "" {
		return ""
	}
	org, err := s.orgRepo.GetOrganizationByUUID(orgUUID)
	if err != nil || org == nil {
		return orgUUID // fallback to UUID if lookup fails
	}
	return org.Handle
}

// resolveArtifactUUIDByKind resolves an artifact handle to its UUID within the table
// backing the given kind (e.g. RestApi, LlmProvider). Used when creating a subscription,
// where the caller specifies the kind so the handle is resolved against exactly one table.
func (s *SubscriptionService) resolveArtifactUUIDByKind(apiId, kind, orgUUID string) (string, error) {
	if apiId == "" || kind == "" {
		return "", constants.ErrAPINotFound
	}
	metadata, err := s.artifactRepo.GetAPIMetadataByHandleAndKind(apiId, kind, orgUUID)
	if err != nil {
		return "", fmt.Errorf("failed to resolve artifact by handle and kind: %w", err)
	}
	if metadata == nil {
		return "", constants.ErrAPINotFound
	}
	return metadata.ID, nil
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

	metadata, err := s.artifactRepo.GetAPIMetadataByHandle(apiId, orgUUID)
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

// ResolveArtifactHandleAndKind returns the artifact handle and kind for display. apiId in
// responses should be the handle (not the internal UUID); kind identifies the artifact type.
// Resolves across all artifact kinds, so it works for REST APIs, LLM providers/proxies, etc.
func (s *SubscriptionService) ResolveArtifactHandleAndKind(artifactUUID, orgUUID string) (handle, kind string) {
	if artifactUUID == "" {
		return "", ""
	}
	art, err := s.artifactRepo.GetByUUID(artifactUUID, orgUUID)
	if err != nil || art == nil {
		return artifactUUID, "" // fallback to UUID if lookup fails
	}
	return art.Handle, art.Type
}

// GetArtifactMetadataMap returns a map of artifact UUID to metadata (handle, kind) for bulk
// lookup across all artifact kinds (avoids N+1 queries).
func (s *SubscriptionService) GetArtifactMetadataMap(uuids []string, orgUUID string) (map[string]*model.APIMetadata, error) {
	return s.artifactRepo.GetMetadataByUUIDs(uuids, orgUUID)
}

// CreateSubscription creates a new subscription for an artifact of the given kind.
// apiId is the artifact handle; kind selects the artifact table it is resolved against.
func (s *SubscriptionService) CreateSubscription(apiId, kind, orgUUID string, subscriberID string, applicationId *string, subscriptionPlanId *string, status string) (*model.Subscription, error) {
	apiUUID, err := s.resolveArtifactUUIDByKind(apiId, kind, orgUUID)
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

	if subscriberID == "" {
		return nil, fmt.Errorf("subscriberId is required")
	}
	exists, err := s.subscriptionRepo.ExistsByAPIAndSubscriber(apiUUID, subscriberID, orgUUID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, constants.ErrSubscriptionAlreadyExists
	}

	if subscriptionPlanId != nil && *subscriptionPlanId != "" {
		plan, err := s.planRepo.GetByHandleAndOrg(*subscriptionPlanId, orgUUID)
		if err != nil {
			return nil, err
		}
		if plan == nil {
			return nil, constants.ErrSubscriptionPlanNotFound
		}
	}

	sub := &model.Subscription{
		ArtifactUUID:       apiUUID,
		SubscriberID:       subscriberID,
		ApplicationID:      applicationId,
		SubscriptionPlanID: subscriptionPlanId,
		OrganizationUUID:   orgUUID,
		Status:             st,
		CreatedBy:          subscriberID,
		UpdatedBy:          subscriberID,
	}
	if err := s.subscriptionRepo.Create(sub); err != nil {
		return nil, err
	}
	_ = s.auditRepo.Record("CREATE", sub.UUID, "subscription", sub.OrganizationUUID, subscriberID)

	if s.gatewayEvents != nil {
		gateways, err := s.apiRepo.GetAPIGatewaysWithDetails(apiUUID, orgUUID)
		if err != nil {
			s.slogger.Warn("Failed to load gateways for subscription.created broadcast", "apiUUID", apiUUID, "error", err)
		} else {
			var planID, appID string
			if sub.SubscriptionPlanID != nil {
				planID = *sub.SubscriptionPlanID
			}
			if sub.ApplicationID != nil {
				appID = *sub.ApplicationID
			}
			event := &model.SubscriptionCreatedEvent{
				ApiId:              sub.ArtifactUUID,
				SubscriptionId:     sub.UUID,
				ApplicationId:      appID,
				SubscriptionToken:  sub.SubscriptionToken,
				SubscriptionPlanId: planID,
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

// ListSubscriptionsByFilters returns subscriptions filtered by API, subscriber and/or application with pagination.
// It returns the list, total count matching filters, and any error.
func (s *SubscriptionService) ListSubscriptionsByFilters(orgUUID string, apiId *string, subscriberID *string, applicationID *string, status *string, limit, offset int) ([]*model.Subscription, int, error) {
	var apiUUID *string
	if apiId != nil && *apiId != "" {
		resolved, err := s.resolveAPIUUID(*apiId, orgUUID)
		if err != nil {
			return nil, 0, err
		}
		apiUUID = &resolved
	}
	list, err := s.subscriptionRepo.ListByFilters(orgUUID, apiUUID, subscriberID, applicationID, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.subscriptionRepo.CountByFilters(orgUUID, apiUUID, subscriberID, applicationID, status)
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// UpdateSubscription updates a subscription (e.g. status). subscriberID must match the stored subscriber_id.
func (s *SubscriptionService) UpdateSubscription(subscriptionId, orgUUID, subscriberID, status string) (*model.Subscription, error) {
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
	if sub.SubscriberID != subscriberID {
		return nil, constants.ErrSubscriptionSubscriberMismatch
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
	sub.UpdatedBy = subscriberID
	if err := s.subscriptionRepo.Update(sub); err != nil {
		return nil, err
	}
	_ = s.auditRepo.Record("UPDATE", sub.UUID, "subscription", sub.OrganizationUUID, subscriberID)

	if s.gatewayEvents != nil {
		gateways, err := s.apiRepo.GetAPIGatewaysWithDetails(sub.ArtifactUUID, orgUUID)
		if err != nil {
			s.slogger.Warn("Failed to load gateways for subscription.updated broadcast", "apiUUID", sub.ArtifactUUID, "error", err)
		} else {
			var planID, appID string
			if sub.SubscriptionPlanID != nil {
				planID = *sub.SubscriptionPlanID
			}
			if sub.ApplicationID != nil {
				appID = *sub.ApplicationID
			}
			event := &model.SubscriptionUpdatedEvent{
				ApiId:              sub.ArtifactUUID,
				SubscriptionId:     sub.UUID,
				ApplicationId:      appID,
				SubscriptionToken:  sub.SubscriptionToken,
				SubscriptionPlanId: planID,
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

// DeleteSubscription removes a subscription. subscriberID must match the stored subscriber_id.
func (s *SubscriptionService) DeleteSubscription(subscriptionId, orgUUID, subscriberID string) error {
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
	if sub.SubscriberID != subscriberID {
		return constants.ErrSubscriptionSubscriberMismatch
	}

	if err := s.subscriptionRepo.Delete(subscriptionId, orgUUID); err != nil {
		return err
	}
	_ = s.auditRepo.Record("DELETE", subscriptionId, "subscription", orgUUID, subscriberID)

	if s.gatewayEvents != nil {
		gateways, err := s.apiRepo.GetAPIGatewaysWithDetails(sub.ArtifactUUID, orgUUID)
		if err != nil {
			s.slogger.Warn("Failed to load gateways for subscription.deleted event",
				"apiId", sub.ArtifactUUID, "subscriptionId", sub.UUID, "error", err)
			return nil
		}
		event := &model.SubscriptionDeletedEvent{
			ApiId:             sub.ArtifactUUID,
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
