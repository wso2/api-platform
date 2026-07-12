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
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
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
		return "", apperror.ArtifactNotFound.New()
	}
	metadata, err := s.artifactRepo.GetAPIMetadataByHandleAndKind(apiId, kind, orgUUID)
	if err != nil {
		return "", fmt.Errorf("failed to resolve artifact by handle and kind: %w", err)
	}
	if metadata == nil {
		return "", apperror.ArtifactNotFound.New()
	}
	return metadata.ID, nil
}

// resolveAPIUUID resolves apiId (handle or UUID) to rest_apis.uuid for the organization
func (s *SubscriptionService) resolveAPIUUID(apiId, orgUUID string) (string, error) {
	if apiId == "" {
		return "", apperror.ArtifactNotFound.New()
	}
	apiModel, err := s.apiRepo.GetAPIByUUID(apiId, orgUUID)
	if err != nil {
		if !apperror.RESTAPINotFound.Is(err) {
			return "", fmt.Errorf("failed to resolve API by UUID: %w", err)
		}
	} else if apiModel != nil {
		return apiModel.ID, nil
	}

	metadata, err := s.artifactRepo.GetAPIMetadataByHandle(apiId, orgUUID)
	if err != nil {
		if apperror.RESTAPINotFound.Is(err) {
			return "", apperror.ArtifactNotFound.New()
		}
		return "", fmt.Errorf("failed to resolve API by handle: %w", err)
	}
	if metadata == nil {
		return "", apperror.ArtifactNotFound.New()
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
// subscriberID identifies who the subscription is for (client-supplied, not a token
// identity); subscriptionToken, when non-empty, is the token imported from the
// Developer Portal (empty means generate one); actor is the authenticated caller's
// internal UUID, used for created_by/updated_by/audit.
func (s *SubscriptionService) CreateSubscription(apiId, kind, orgUUID string, subscriberID string, applicationId *string, subscriptionPlanId *string, subscriptionToken string, status string, actor string) (*model.Subscription, error) {
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
		return nil, apperror.ValidationFailed.New("subscriberId is required")
	}
	exists, err := s.subscriptionRepo.ExistsByAPIAndSubscriber(apiUUID, subscriberID, orgUUID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperror.SubscriptionExists.New()
	}

	// subscriptionPlanId carries the Developer Portal subscription plan handle. Resolve it to the
	// plan's UUID, which is what the subscriptions.subscription_plan_uuid foreign key references.
	if subscriptionPlanId != nil && *subscriptionPlanId != "" {
		plan, err := s.planRepo.GetByHandleAndOrg(*subscriptionPlanId, orgUUID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, apperror.SubscriptionPlanNotFound.New()
			}
			return nil, err
		}
		if plan == nil {
			return nil, apperror.SubscriptionPlanNotFound.New()
		}
		subscriptionPlanId = &plan.UUID
	}

	// A non-empty subscriptionToken is the token imported from the Developer Portal (the
	// value shown to the user); the repository persists it as-is. When empty (interactive
	// creation), the repository generates a fresh token.
	sub := &model.Subscription{
		ArtifactUUID:       apiUUID,
		SubscriberID:       subscriberID,
		ApplicationID:      applicationId,
		SubscriptionPlanID: subscriptionPlanId,
		SubscriptionToken:  subscriptionToken,
		OrganizationUUID:   orgUUID,
		Status:             st,
		CreatedBy:          actor,
		UpdatedBy:          actor,
	}
	if err := s.subscriptionRepo.Create(sub); err != nil {
		return nil, err
	}
	_ = s.auditRepo.Record("CREATE", sub.UUID, "subscription", sub.OrganizationUUID, actor)

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
		if apperror.SubscriptionNotFound.Is(err) {
			return nil, apperror.SubscriptionNotFound.New()
		}
		return nil, err
	}
	if sub == nil {
		return nil, apperror.SubscriptionNotFound.New()
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

// FindByArtifactKindAndSubscriber locates a single subscription by (API, subscriber) within the org,
// resolving the API by handle + kind so the lookup is scoped to the artifact table backing that kind
// (a handle shared across kinds resolves unambiguously). Returns (nil, nil) when none exists and
// ErrAPINotFound when the artifact itself cannot be resolved.
func (s *SubscriptionService) FindByArtifactKindAndSubscriber(orgUUID, apiHandle, kind, subscriberID string) (*model.Subscription, error) {
	apiUUID, err := s.resolveArtifactUUIDByKind(apiHandle, kind, orgUUID)
	if err != nil {
		return nil, err
	}
	sub := subscriberID
	list, err := s.subscriptionRepo.ListByFilters(orgUUID, &apiUUID, &sub, nil, nil, 1, 0)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return list[0], nil
}

// UpdateSubscription updates a subscription (e.g. status). subscriberID must match the
// stored subscriber_id; actor is the authenticated caller's internal UUID, used for
// updated_by/audit.
func (s *SubscriptionService) UpdateSubscription(subscriptionId, orgUUID, subscriberID, status, actor string) (*model.Subscription, error) {
	sub, err := s.subscriptionRepo.GetByID(subscriptionId, orgUUID)
	if err != nil {
		if apperror.SubscriptionNotFound.Is(err) {
			return nil, apperror.SubscriptionNotFound.New()
		}
		return nil, err
	}
	if sub == nil {
		return nil, apperror.SubscriptionNotFound.New()
	}
	if sub.SubscriberID != subscriberID {
		return nil, apperror.SubscriptionForbidden.New()
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
	sub.UpdatedBy = actor
	if err := s.subscriptionRepo.Update(sub); err != nil {
		return nil, err
	}
	_ = s.auditRepo.Record("UPDATE", sub.UUID, "subscription", sub.OrganizationUUID, actor)

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

// ChangePlan switches the subscription plan of an existing subscription and broadcasts the change
// to the gateways where the API is deployed. subscriberID must match the stored subscriber_id.
func (s *SubscriptionService) ChangePlan(subscriptionId, orgUUID, subscriberID, planHandle string) (*model.Subscription, error) {
	sub, err := s.subscriptionRepo.GetByID(subscriptionId, orgUUID)
	if err != nil {
		if apperror.SubscriptionNotFound.Is(err) {
			return nil, apperror.SubscriptionNotFound.New()
		}
		return nil, err
	}
	if sub == nil {
		return nil, apperror.SubscriptionNotFound.New()
	}
	if sub.SubscriberID != subscriberID {
		return nil, apperror.SubscriptionForbidden.New()
	}

	// planHandle carries the Developer Portal subscription plan handle. Resolve it to the plan's
	// UUID, which is what the subscriptions.subscription_plan_uuid foreign key references.
	planRecord, err := s.planRepo.GetByHandleAndOrg(planHandle, orgUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.SubscriptionPlanNotFound.New()
		}
		return nil, err
	}
	if planRecord == nil {
		return nil, apperror.SubscriptionPlanNotFound.New()
	}
	plan := planRecord.UUID
	sub.SubscriptionPlanID = &plan
	if err := s.subscriptionRepo.Update(sub); err != nil {
		return nil, err
	}

	if s.gatewayEvents != nil {
		gateways, err := s.apiRepo.GetAPIGatewaysWithDetails(sub.ArtifactUUID, orgUUID)
		if err != nil {
			s.slogger.Warn("Failed to load gateways for subscription plan change broadcast",
				"apiId", sub.ArtifactUUID, "subscriptionId", sub.UUID, "error", err)
		} else {
			event := &model.SubscriptionUpdatedEvent{
				ApiId:              sub.ArtifactUUID,
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
					s.slogger.Warn("Failed to broadcast subscription plan change event",
						"gatewayId", gw.ID, "subscriptionId", sub.UUID, "error", err)
				}
			}
		}
	}

	return sub, nil
}

// RegenerateToken rotates the subscription's token to the value provided by the Developer Portal
// (delivered encrypted in the subscription.token_regenerated event). The old token is invalidated;
// the new token is persisted (hashed + encrypted) and broadcast to the gateways where the API is
// deployed. subscriberID must match the stored subscriber_id.
func (s *SubscriptionService) RegenerateToken(subscriptionId, orgUUID, subscriberID, newToken string) (*model.Subscription, error) {
	if newToken == "" {
		return nil, apperror.ValidationFailed.New("subscription token is required")
	}
	sub, err := s.subscriptionRepo.GetByID(subscriptionId, orgUUID)
	if err != nil {
		if apperror.SubscriptionNotFound.Is(err) {
			return nil, apperror.SubscriptionNotFound.New()
		}
		return nil, err
	}
	if sub == nil {
		return nil, apperror.SubscriptionNotFound.New()
	}
	if sub.SubscriberID != subscriberID {
		return nil, apperror.SubscriptionForbidden.New()
	}

	if err := s.subscriptionRepo.UpdateToken(subscriptionId, orgUUID, newToken); err != nil {
		return nil, err
	}
	sub.SubscriptionToken = newToken
	_ = s.auditRepo.Record("UPDATE", sub.UUID, "subscription", sub.OrganizationUUID, subscriberID)

	if s.gatewayEvents != nil {
		gateways, err := s.apiRepo.GetAPIGatewaysWithDetails(sub.ArtifactUUID, orgUUID)
		if err != nil {
			s.slogger.Warn("Failed to load gateways for subscription token regeneration broadcast",
				"apiUUID", sub.ArtifactUUID, "subscriptionId", sub.UUID, "error", err)
		} else {
			event := &model.SubscriptionUpdatedEvent{
				ApiId:              sub.ArtifactUUID,
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
					s.slogger.Warn("Failed to broadcast subscription token regeneration event",
						"gatewayId", gw.ID, "subscriptionId", sub.UUID, "error", err)
				}
			}
		}
	}

	return sub, nil
}

// DeleteSubscription removes a subscription. subscriberID must match the stored
// subscriber_id; actor is the authenticated caller's internal UUID, used for audit.
func (s *SubscriptionService) DeleteSubscription(subscriptionId, orgUUID, subscriberID, actor string) error {
	sub, err := s.subscriptionRepo.GetByID(subscriptionId, orgUUID)
	if err != nil {
		if apperror.SubscriptionNotFound.Is(err) {
			return apperror.SubscriptionNotFound.New()
		}
		return err
	}
	if sub == nil {
		return apperror.SubscriptionNotFound.New()
	}
	if sub.SubscriberID != subscriberID {
		return apperror.SubscriptionForbidden.New()
	}

	if err := s.subscriptionRepo.Delete(subscriptionId, orgUUID); err != nil {
		return err
	}
	_ = s.auditRepo.Record("DELETE", subscriptionId, "subscription", orgUUID, actor)

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
