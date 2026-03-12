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

// SubscriptionPlanService handles subscription plan business logic
type SubscriptionPlanService struct {
	planRepo      repository.SubscriptionPlanRepository
	gatewayRepo   repository.GatewayRepository
	gatewayEvents *GatewayEventsService
	slogger       *slog.Logger
}

// NewSubscriptionPlanService creates a new subscription plan service
func NewSubscriptionPlanService(
	planRepo repository.SubscriptionPlanRepository,
	gatewayRepo repository.GatewayRepository,
	gatewayEvents *GatewayEventsService,
	slogger *slog.Logger,
) *SubscriptionPlanService {
	if slogger == nil {
		slogger = slog.Default()
	}
	return &SubscriptionPlanService{
		planRepo:      planRepo,
		gatewayRepo:   gatewayRepo,
		gatewayEvents: gatewayEvents,
		slogger:       slogger,
	}
}

// CreatePlan creates a new subscription plan
func (s *SubscriptionPlanService) CreatePlan(orgUUID string, plan *model.SubscriptionPlan) (*model.SubscriptionPlan, error) {
	if plan.PlanName == "" {
		return nil, fmt.Errorf("planName is required")
	}

	exists, err := s.planRepo.ExistsByNameAndOrg(plan.PlanName, orgUUID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, constants.ErrSubscriptionPlanAlreadyExists
	}

	plan.OrganizationUUID = orgUUID
	if plan.Status == "" {
		plan.Status = model.SubscriptionPlanStatusActive
	}

	if err := s.planRepo.Create(plan); err != nil {
		return nil, err
	}

	s.broadcastPlanEvent(orgUUID, "created", &model.SubscriptionPlanCreatedEvent{
		PlanId:             plan.UUID,
		PlanName:           plan.PlanName,
		BillingPlan:        plan.BillingPlan,
		StopOnQuotaReach:   plan.StopOnQuotaReach,
		ThrottleLimitCount: plan.ThrottleLimitCount,
		ThrottleLimitUnit:  plan.ThrottleLimitUnit,
		ExpiryTime:         plan.ExpiryTime,
		Status:             string(plan.Status),
	})

	return plan, nil
}

// GetPlan returns a subscription plan by ID
func (s *SubscriptionPlanService) GetPlan(planID, orgUUID string) (*model.SubscriptionPlan, error) {
	plan, err := s.planRepo.GetByID(planID, orgUUID)
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

// ListPlans returns subscription plans for an organization with pagination
func (s *SubscriptionPlanService) ListPlans(orgUUID string, limit, offset int) ([]*model.SubscriptionPlan, error) {
	return s.planRepo.ListByOrganization(orgUUID, limit, offset)
}

// UpdatePlan updates a subscription plan
func (s *SubscriptionPlanService) UpdatePlan(planID, orgUUID string, update *model.SubscriptionPlanUpdate) (*model.SubscriptionPlan, error) {
	existing, err := s.planRepo.GetByID(planID, orgUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrSubscriptionPlanNotFound
		}
		return nil, err
	}
	if existing == nil {
		return nil, constants.ErrSubscriptionPlanNotFound
	}

	if update.PlanName != nil && *update.PlanName != existing.PlanName {
		nameExists, err := s.planRepo.ExistsByNameAndOrg(*update.PlanName, orgUUID)
		if err != nil {
			return nil, err
		}
		if nameExists {
			return nil, constants.ErrSubscriptionPlanAlreadyExists
		}
		existing.PlanName = *update.PlanName
	}
	if update.BillingPlan != nil {
		existing.BillingPlan = *update.BillingPlan
	}
	if update.StopOnQuotaReach != nil {
		existing.StopOnQuotaReach = *update.StopOnQuotaReach
	}
	if update.ThrottleLimitCount != nil {
		existing.ThrottleLimitCount = update.ThrottleLimitCount
	}
	if update.ThrottleLimitUnit != nil {
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

	if err := s.planRepo.Update(existing); err != nil {
		return nil, err
	}

	s.broadcastPlanEvent(orgUUID, "updated", &model.SubscriptionPlanUpdatedEvent{
		PlanId:             existing.UUID,
		PlanName:           existing.PlanName,
		BillingPlan:        existing.BillingPlan,
		StopOnQuotaReach:   existing.StopOnQuotaReach,
		ThrottleLimitCount: existing.ThrottleLimitCount,
		ThrottleLimitUnit:  existing.ThrottleLimitUnit,
		ExpiryTime:         existing.ExpiryTime,
		Status:             string(existing.Status),
	})

	return existing, nil
}

// DeletePlan removes a subscription plan
func (s *SubscriptionPlanService) DeletePlan(planID, orgUUID string) error {
	existing, err := s.planRepo.GetByID(planID, orgUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return constants.ErrSubscriptionPlanNotFound
		}
		return err
	}
	if existing == nil {
		return constants.ErrSubscriptionPlanNotFound
	}

	if err := s.planRepo.Delete(planID, orgUUID); err != nil {
		return err
	}

	s.broadcastPlanEvent(orgUUID, "deleted", &model.SubscriptionPlanDeletedEvent{
		PlanId:   existing.UUID,
		PlanName: existing.PlanName,
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
