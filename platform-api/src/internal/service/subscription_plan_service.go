/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
)

// defaultBillingPlan is the billing plan value sent to gateways. The platform-api no
// longer stores a per-plan billing plan, but the gateway event contract still carries
// the field, so a fixed "free" value is emitted to keep that contract unchanged.
const defaultBillingPlan = "free"

// SubscriptionPlanService handles subscription plan business logic
type SubscriptionPlanService struct {
	planRepo      repository.SubscriptionPlanRepository
	gatewayRepo   repository.GatewayRepository
	orgRepo       repository.OrganizationRepository
	gatewayEvents *GatewayEventsService
	auditRepo     repository.AuditRepository
	slogger       *slog.Logger
}

// NewSubscriptionPlanService creates a new subscription plan service
func NewSubscriptionPlanService(
	planRepo repository.SubscriptionPlanRepository,
	gatewayRepo repository.GatewayRepository,
	orgRepo repository.OrganizationRepository,
	gatewayEvents *GatewayEventsService,
	auditRepo repository.AuditRepository,
	slogger *slog.Logger,
) *SubscriptionPlanService {
	if slogger == nil {
		slogger = slog.Default()
	}
	return &SubscriptionPlanService{
		planRepo:      planRepo,
		gatewayRepo:   gatewayRepo,
		orgRepo:       orgRepo,
		gatewayEvents: gatewayEvents,
		auditRepo:     auditRepo,
		slogger:       slogger,
	}
}

// ResolveOrgHandle returns the organization handle for display (organizationId in
// responses should be the handle, not the internal UUID).
func (s *SubscriptionPlanService) ResolveOrgHandle(orgUUID string) string {
	if orgUUID == "" {
		return ""
	}
	org, err := s.orgRepo.GetOrganizationByUUID(orgUUID)
	if err != nil || org == nil {
		return orgUUID // fallback to UUID if lookup fails
	}
	return org.Handle
}

// CreatePlan creates a new subscription plan
func (s *SubscriptionPlanService) CreatePlan(orgUUID, actor string, plan *model.SubscriptionPlan) (*model.SubscriptionPlan, error) {
	if plan.Handle == "" {
		return nil, fmt.Errorf("handle is required")
	}
	if plan.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	exists, err := s.planRepo.ExistsByHandleAndOrg(plan.Handle, orgUUID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, constants.ErrSubscriptionPlanAlreadyExists
	}

	plan.OrganizationUUID = orgUUID
	plan.CreatedBy = actor
	plan.UpdatedBy = actor
	if plan.Status == "" {
		plan.Status = model.SubscriptionPlanStatusActive
	}
	if plan.ThrottleLimitUnit != "" && !constants.ValidThrottleLimitUnits[plan.ThrottleLimitUnit] {
		return nil, constants.ErrInvalidThrottleLimitUnit
	}

	if err := s.planRepo.Create(plan); err != nil {
		return nil, err
	}
	if s.auditRepo != nil {
		_ = s.auditRepo.Record("CREATE", plan.UUID, "subscription_plan", orgUUID, actor)
	}

	s.broadcastPlanEvent(orgUUID, "created", &model.SubscriptionPlanCreatedEvent{
		PlanId:             plan.UUID,
		Handle:             plan.Handle,
		PlanName:           plan.Name,
		BillingPlan:        defaultBillingPlan,
		StopOnQuotaReach:   plan.StopOnQuotaReach,
		ThrottleLimitCount: plan.ThrottleLimitCount,
		ThrottleLimitUnit:  plan.ThrottleLimitUnit,
		ExpiryTime:         plan.ExpiryTime,
		Status:             string(plan.Status),
	})

	return plan, nil
}

// GetPlan returns a subscription plan by handle
func (s *SubscriptionPlanService) GetPlan(handle, orgUUID string) (*model.SubscriptionPlan, error) {
	plan, err := s.planRepo.GetByHandleAndOrg(handle, orgUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrSubscriptionPlanNotFound
		}
		return nil, err
	}
	if plan == nil {
		return nil, constants.ErrSubscriptionPlanNotFound
	}
	return plan, nil
}

// GetPlanNameMap returns a map of plan UUID to plan name for bulk lookup (avoids N+1 queries).
func (s *SubscriptionPlanService) GetPlanNameMap(planIDs []string, orgUUID string) (map[string]string, error) {
	return s.planRepo.GetByIDs(planIDs, orgUUID)
}

// ListPlans returns subscription plans for an organization with pagination
func (s *SubscriptionPlanService) ListPlans(orgUUID string, limit, offset int) ([]*model.SubscriptionPlan, error) {
	return s.planRepo.ListByOrganization(orgUUID, limit, offset)
}

// UpdatePlan updates a subscription plan
func (s *SubscriptionPlanService) UpdatePlan(handle, orgUUID, actor string, update *model.SubscriptionPlanUpdate) (*model.SubscriptionPlan, error) {
	existing, err := s.planRepo.GetByHandleAndOrg(handle, orgUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrSubscriptionPlanNotFound
		}
		return nil, err
	}
	if existing == nil {
		return nil, constants.ErrSubscriptionPlanNotFound
	}

	// The handle (id) is immutable; reject any attempt to change it to a different value.
	if update.Handle != nil && *update.Handle != "" && *update.Handle != existing.Handle {
		return nil, constants.ErrHandleImmutable
	}
	if update.Name != nil {
		if *update.Name == "" {
			return nil, fmt.Errorf("name is required")
		}
		existing.Name = *update.Name
	}
	if update.StopOnQuotaReach != nil {
		existing.StopOnQuotaReach = *update.StopOnQuotaReach
	}
	if update.ThrottleLimitCount != nil {
		existing.ThrottleLimitCount = update.ThrottleLimitCount
	} else if update.ClearLimit {
		existing.ThrottleLimitCount = nil
		existing.ThrottleLimitUnit = ""
		existing.StopOnQuotaReach = true
	}
	if update.ThrottleLimitUnit != nil {
		if !constants.ValidThrottleLimitUnits[*update.ThrottleLimitUnit] {
			return nil, constants.ErrInvalidThrottleLimitUnit
		}
		existing.ThrottleLimitUnit = *update.ThrottleLimitUnit
	}
	if update.ExpiryTime != nil {
		existing.ExpiryTime = update.ExpiryTime
	}
	if update.Status != nil {
		switch *update.Status {
		case model.SubscriptionPlanStatusActive, model.SubscriptionPlanStatusInactive:
			existing.Status = *update.Status
		default:
			return nil, fmt.Errorf("invalid status: %s", *update.Status)
		}
	}

	existing.UpdatedBy = actor
	if err := s.planRepo.Update(existing); err != nil {
		return nil, err
	}
	if s.auditRepo != nil {
		_ = s.auditRepo.Record("UPDATE", existing.UUID, "subscription_plan", orgUUID, actor)
	}

	s.broadcastPlanEvent(orgUUID, "updated", &model.SubscriptionPlanUpdatedEvent{
		PlanId:             existing.UUID,
		Handle:             existing.Handle,
		PlanName:           existing.Name,
		BillingPlan:        defaultBillingPlan,
		StopOnQuotaReach:   existing.StopOnQuotaReach,
		ThrottleLimitCount: existing.ThrottleLimitCount,
		ThrottleLimitUnit:  existing.ThrottleLimitUnit,
		ExpiryTime:         existing.ExpiryTime,
		Status:             string(existing.Status),
	})

	return existing, nil
}

// DeletePlan removes a subscription plan
func (s *SubscriptionPlanService) DeletePlan(handle, orgUUID, actor string) error {
	existing, err := s.planRepo.GetByHandleAndOrg(handle, orgUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return constants.ErrSubscriptionPlanNotFound
		}
		return err
	}
	if existing == nil {
		return constants.ErrSubscriptionPlanNotFound
	}

	if err := s.planRepo.Delete(existing.UUID, orgUUID); err != nil {
		return err
	}
	if s.auditRepo != nil {
		_ = s.auditRepo.Record("DELETE", existing.UUID, "subscription_plan", orgUUID, actor)
	}

	s.broadcastPlanEvent(orgUUID, "deleted", &model.SubscriptionPlanDeletedEvent{
		PlanId:   existing.UUID,
		Handle:   existing.Handle,
		PlanName: existing.Name,
	})

	return nil
}

// broadcastPlanEvent sends a subscriptionPlan.* event to all gateways in the organization.
func (s *SubscriptionPlanService) broadcastPlanEvent(orgUUID, action string, payload interface{}) {
	if s.gatewayEvents == nil || s.gatewayRepo == nil {
		return
	}
	gateways, err := s.gatewayRepo.GetByOrganizationID(orgUUID)
	if err != nil {
		s.slogger.Warn("Failed to load gateways for subscriptionPlan broadcast",
			"orgId", orgUUID, "action", action, "error", err)
		return
	}
	for _, gw := range gateways {
		if gw == nil || gw.ID == "" {
			continue
		}
		var broadcastErr error
		switch action {
		case "created":
			broadcastErr = s.gatewayEvents.BroadcastSubscriptionPlanCreatedEvent(gw.ID, payload.(*model.SubscriptionPlanCreatedEvent))
		case "updated":
			broadcastErr = s.gatewayEvents.BroadcastSubscriptionPlanUpdatedEvent(gw.ID, payload.(*model.SubscriptionPlanUpdatedEvent))
		case "deleted":
			broadcastErr = s.gatewayEvents.BroadcastSubscriptionPlanDeletedEvent(gw.ID, payload.(*model.SubscriptionPlanDeletedEvent))
		}
		if broadcastErr != nil {
			s.slogger.Warn("Failed to broadcast subscriptionPlan event",
				"gatewayId", gw.ID, "action", action, "error", broadcastErr)
		}
	}
}
