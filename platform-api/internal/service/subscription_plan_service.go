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
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
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
	audit         *AuditService
	slogger       *slog.Logger
	// meterRepo and pricingRepo check and store the pricing a plan is created with. While they are
	// nil, a plan with pricing is refused.
	meterRepo   repository.MeterRepository
	pricingRepo repository.SubscriptionPlanPricingRepository
}

// SetPricingRepositories lets a plan be created with its pricing.
func (s *SubscriptionPlanService) SetPricingRepositories(meterRepo repository.MeterRepository, pricingRepo repository.SubscriptionPlanPricingRepository) {
	s.meterRepo = meterRepo
	s.pricingRepo = pricingRepo
}

// NewSubscriptionPlanService creates a new subscription plan service
func NewSubscriptionPlanService(
	planRepo repository.SubscriptionPlanRepository,
	gatewayRepo repository.GatewayRepository,
	orgRepo repository.OrganizationRepository,
	gatewayEvents *GatewayEventsService,
	slogger *slog.Logger,
	audit *AuditService,
) *SubscriptionPlanService {
	if slogger == nil {
		slogger = slog.Default()
	}
	return &SubscriptionPlanService{
		planRepo:      planRepo,
		gatewayRepo:   gatewayRepo,
		orgRepo:       orgRepo,
		gatewayEvents: gatewayEvents,
		audit:         audit,
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

// CreatePlan creates a new subscription plan. pricingBody is the pricing request
// (SubscriptionPlanPricingRequest) of a paid plan, or nil for a free plan. The pricing is checked
// with the API Portal's rules and stored with the plan, in one transaction. This is the only place
// pricing is set: it cannot change once the plan exists.
func (s *SubscriptionPlanService) CreatePlan(ctx context.Context, orgUUID, actor string, plan *model.SubscriptionPlan, pricingBody []byte) (*model.SubscriptionPlan, error) {
	if plan.Handle == "" {
		return nil, apperror.ValidationFailed.New("handle is required")
	}
	if plan.Name == "" {
		return nil, apperror.ValidationFailed.New("name is required")
	}

	var pricing *model.SubscriptionPlanPricing
	if pricingBody != nil {
		if s.meterRepo == nil || s.pricingRepo == nil {
			return nil, apperror.Internal.New().WithLogMessage("plan pricing storage is not configured")
		}
		var err error
		if pricing, err = parsePlanPricing(pricingBody); err != nil {
			return nil, err
		}
		if err := checkPricedMeters(s.meterRepo, orgUUID, pricing); err != nil {
			return nil, err
		}
		pricing.CreatedAt = time.Now().UTC()
	}
	return s.createPlan(ctx, orgUUID, actor, plan, pricing)
}

// createPlan stores a new plan with its checked pricing (nil for a free plan) in one transaction and
// sends it to the gateways.
func (s *SubscriptionPlanService) createPlan(ctx context.Context, orgUUID, actor string, plan *model.SubscriptionPlan, pricing *model.SubscriptionPlanPricing) (*model.SubscriptionPlan, error) {
	exists, err := s.planRepo.ExistsByHandleAndOrg(plan.Handle, orgUUID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperror.SubscriptionPlanExists.New()
	}

	plan.OrganizationUUID = orgUUID
	plan.CreatedBy = actor
	plan.UpdatedBy = actor
	if plan.Status == "" {
		plan.Status = model.SubscriptionPlanStatusActive
	}
	if plan.ThrottleLimitUnit != "" && !constants.ValidThrottleLimitUnits[plan.ThrottleLimitUnit] {
		return nil, apperror.ValidationFailed.New("Invalid throttle limit unit")
	}

	if err := s.planRepo.CreateWithPricing(plan, pricing); err != nil {
		if IsAuditableFailure(err) {
			s.audit.Record(ctx, &AuditEvent{
				Action: model.AuditActionCreate, ResourceType: model.AuditResourceTypeSubscriptionPlan,
				Outcome: model.AuditOutcomeFailed, OrganizationUUID: orgUUID, PerformedBy: actor,
				ResourceUUID: plan.UUID, ResourceName: plan.Handle,
			})
		}
		return nil, err
	}
	s.audit.Record(ctx, &AuditEvent{
		OrganizationUUID: orgUUID, Action: model.AuditActionCreate,
		ResourceType: model.AuditResourceTypeSubscriptionPlan, Outcome: model.AuditOutcomeSucceeded,
		PerformedBy: actor, ResourceUUID: plan.UUID, ResourceName: plan.Handle,
		EventData: subscriptionPlanAuditFields(plan),
	})

	s.broadcastPlanEvent(ctx, orgUUID, "created", &model.SubscriptionPlanCreatedEvent{
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
			return nil, apperror.SubscriptionPlanNotFound.New()
		}
		return nil, err
	}
	if plan == nil {
		return nil, apperror.SubscriptionPlanNotFound.New()
	}
	return plan, nil
}

// GetUsage returns what offers or uses the plan: the REST APIs, publication drafts and publications
// that offer it, and how many subscriptions use it. A plan in use keeps its name and limit and
// cannot be deleted.
func (s *SubscriptionPlanService) GetUsage(plan *model.SubscriptionPlan) (*model.SubscriptionPlanUsage, error) {
	return s.planRepo.GetUsage(plan.UUID, plan.Handle, plan.OrganizationUUID)
}

// GetPlanNameMap returns a map of plan UUID to plan name for bulk lookup (avoids N+1 queries).
func (s *SubscriptionPlanService) GetPlanNameMap(planIDs []string, orgUUID string) (map[string]string, error) {
	return s.planRepo.GetByIDs(planIDs, orgUUID)
}

// ListPlans returns subscription plans for an organization with pagination, an
// optional name or handle search and an optional status ("" for any)
func (s *SubscriptionPlanService) ListPlans(orgUUID string, opts repository.ListOptions, status string) ([]*model.SubscriptionPlan, error) {
	return s.planRepo.ListByOrganization(orgUUID, opts, status)
}

// CountPlans returns the number of subscription plans in an organization that match
// the optional search and status ("" for any).
func (s *SubscriptionPlanService) CountPlans(orgUUID, search, status string) (int, error) {
	return s.planRepo.CountByOrganization(orgUUID, search, status)
}

// UpdatePlan updates a subscription plan through the REST API. A change made here is not sent to
// the API Portals that have the plan, and they show its name and request limit. So the name and
// limit can change only while nothing offers or uses the plan; the next publish that offers it then
// brings each portal's copy up to date. Pricing never changes.
func (s *SubscriptionPlanService) UpdatePlan(ctx context.Context, handle, orgUUID, actor string, update *model.SubscriptionPlanUpdate) (*model.SubscriptionPlan, error) {
	existing, err := s.planRepo.GetByHandleAndOrg(handle, orgUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.SubscriptionPlanNotFound.New()
		}
		return nil, err
	}
	if existing == nil {
		return nil, apperror.SubscriptionPlanNotFound.New()
	}
	if changesPortalValues(existing, update) {
		usage, err := s.planRepo.GetUsage(existing.UUID, existing.Handle, orgUUID)
		if err != nil {
			return nil, err
		}
		if usage.InUse() {
			return nil, apperror.SubscriptionPlanInUse.New(planEditInUseMessage(existing.Name))
		}
	}
	return s.applyPlanUpdate(ctx, existing, orgUUID, actor, update)
}

// changesPortalValues reports whether update changes what an API Portal shows of the plan: its
// name or its request limit. It compares the values, not which fields were sent, since a client
// sends every field on each save. Names are compared without surrounding spaces, since a client
// may trim the name it sends.
func changesPortalValues(existing *model.SubscriptionPlan, update *model.SubscriptionPlanUpdate) bool {
	if update.Name != nil && strings.TrimSpace(*update.Name) != strings.TrimSpace(existing.Name) {
		return true
	}
	if update.ThrottleLimitCount != nil {
		if existing.ThrottleLimitCount == nil || *existing.ThrottleLimitCount != *update.ThrottleLimitCount {
			return true
		}
	} else if update.ClearLimit && existing.ThrottleLimitCount != nil {
		return true
	}
	return update.ThrottleLimitUnit != nil && existing.ThrottleLimitCount != nil &&
		*update.ThrottleLimitUnit != existing.ThrottleLimitUnit
}

// applyPlanUpdate applies update to existing, saves it and tells the gateways.
func (s *SubscriptionPlanService) applyPlanUpdate(ctx context.Context, existing *model.SubscriptionPlan, orgUUID, actor string, update *model.SubscriptionPlanUpdate) (*model.SubscriptionPlan, error) {
	before := subscriptionPlanAuditFields(existing)
	if update.Name != nil {
		if *update.Name == "" {
			return nil, apperror.ValidationFailed.New("name is required")
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
			return nil, apperror.ValidationFailed.New("Invalid throttle limit unit")
		}
		existing.ThrottleLimitUnit = *update.ThrottleLimitUnit
	}
	if update.ExpiryTime != nil {
		existing.ExpiryTime = update.ExpiryTime
	} else if update.ClearExpiryTime {
		existing.ExpiryTime = nil
	}
	if update.Status != nil {
		switch *update.Status {
		case model.SubscriptionPlanStatusActive, model.SubscriptionPlanStatusInactive:
			existing.Status = *update.Status
		default:
			return nil, apperror.ValidationFailed.New(fmt.Sprintf("invalid status: %s", *update.Status))
		}
	}

	existing.UpdatedBy = actor
	if err := s.planRepo.Update(existing); err != nil {
		if IsAuditableFailure(err) {
			s.audit.Record(ctx, &AuditEvent{
				Action: model.AuditActionUpdate, ResourceType: model.AuditResourceTypeSubscriptionPlan,
				Outcome: model.AuditOutcomeFailed, OrganizationUUID: orgUUID, PerformedBy: actor,
				ResourceUUID: existing.UUID, ResourceName: existing.Handle,
			})
		}
		return nil, err
	}
	s.audit.Record(ctx, &AuditEvent{
		OrganizationUUID: orgUUID, Action: model.AuditActionUpdate,
		ResourceType: model.AuditResourceTypeSubscriptionPlan, Outcome: model.AuditOutcomeSucceeded,
		PerformedBy: actor, ResourceUUID: existing.UUID, ResourceName: existing.Handle,
		EventData: AuditDiff(before, subscriptionPlanAuditFields(existing)),
	})

	s.broadcastPlanEvent(ctx, orgUUID, "updated", &model.SubscriptionPlanUpdatedEvent{
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

// DeletePlan deletes a subscription plan through the REST API, only while nothing here offers or
// uses it. API Portals that have the plan keep it: a delete made here is not sent to them.
func (s *SubscriptionPlanService) DeletePlan(ctx context.Context, handle, orgUUID, actor string) error {
	existing, err := s.GetPlan(handle, orgUUID)
	if err != nil {
		return err
	}
	usage, err := s.planRepo.GetUsage(existing.UUID, existing.Handle, orgUUID)
	if err != nil {
		return err
	}
	if usage.InUse() {
		return apperror.SubscriptionPlanInUse.New(planInUseMessage(existing.Name))
	}
	return s.deletePlanWithReferences(ctx, existing, orgUUID, actor)
}

// deletePlanWithReferences deletes a plan and its references, in one transaction:
//   - the plan is removed from the REST APIs and artifact_subscription_plans rows that offer it;
//   - the publication drafts and live publications that offer it are kept, showing it as deleted;
//   - its subscriptions are kept, without a plan;
//   - the plan is deleted, with its limits and pricing.
//
// The gateways are then told the plan is deleted. DeletePlan calls it only once nothing uses the
// plan, so there it deletes just the plan. DeletePlanFromWebhook calls it for a plan the API Portal
// deleted, which may still be offered or used here.
func (s *SubscriptionPlanService) deletePlanWithReferences(ctx context.Context, plan *model.SubscriptionPlan, orgUUID, actor string) error {
	deleted, err := s.planRepo.DeleteWithReferences(plan.UUID, plan.Handle, orgUUID)
	if err != nil {
		if IsAuditableFailure(err) {
			s.audit.Record(ctx, &AuditEvent{
				Action: model.AuditActionDelete, ResourceType: model.AuditResourceTypeSubscriptionPlan,
				Outcome: model.AuditOutcomeFailed, OrganizationUUID: orgUUID, PerformedBy: actor,
				ResourceUUID: plan.UUID, ResourceName: plan.Handle,
			})
		}
		return err
	}
	if !deleted {
		// A concurrent delete removed it first.
		return nil
	}
	s.audit.Record(ctx, &AuditEvent{
		OrganizationUUID: orgUUID, Action: model.AuditActionDelete,
		ResourceType: model.AuditResourceTypeSubscriptionPlan, Outcome: model.AuditOutcomeSucceeded,
		PerformedBy: actor, ResourceUUID: plan.UUID, ResourceName: plan.Handle,
		EventData: map[string]any{"handle": plan.Handle},
	})

	s.broadcastPlanEvent(ctx, orgUUID, "deleted", &model.SubscriptionPlanDeletedEvent{
		PlanId:   plan.UUID,
		Handle:   plan.Handle,
		PlanName: plan.Name,
	})

	return nil
}

// planInUseMessage refuses deleting a plan in use, for example "The plan Gold is in use and can't
// be deleted."
func planInUseMessage(name string) string {
	return fmt.Sprintf("The plan %s is in use and can't be deleted.", name)
}

// planEditInUseMessage refuses a change to the name or limit of a plan in use, for example "The
// plan Gold is in use. To change its name or limit, create a new plan."
func planEditInUseMessage(name string) string {
	return fmt.Sprintf("The plan %s is in use. To change its name or limit, create a new plan.", name)
}

// UpsertPlanFromWebhook reconciles a plan created or changed in the API Portal. Both events carry
// the plan's pricing (nil for a free plan), which the portal never changes.
//   - When the organization does not have the plan, it is created with its pricing, in one
//     transaction. It is dated occurredAt, when it was made in the API Portal, not when the event
//     was processed, so that the portal's later deletion of it is not taken for the deletion of an
//     older plan with the same handle.
//   - Otherwise its pricing is checked against the stored one (reconcilePricing), and the name and
//     request limit are applied when they differ, so an echo of a change made here changes nothing.
//     The portal has no stop-on-quota or status setting, so those are left as they are.
//
// Each meter the prices count with must already be here. Created and updated plans are sent to
// the gateways as for REST changes. Webhook events carry no user, so the actor is empty, as for
// applications.
func (s *SubscriptionPlanService) UpsertPlanFromWebhook(ctx context.Context, orgUUID, handle, name string, limitCount *int, limitUnit string, pricing *model.SubscriptionPlanPricing, occurredAt time.Time) (*model.SubscriptionPlan, error) {
	if pricing != nil {
		if s.meterRepo == nil || s.pricingRepo == nil {
			return nil, apperror.Internal.New().WithLogMessage("plan pricing storage is not configured")
		}
		if err := checkPricedMeters(s.meterRepo, orgUUID, pricing); err != nil {
			return nil, err
		}
		pricing.CreatedAt = occurredAt.UTC()
	}
	existing, err := s.planRepo.GetByHandleAndOrg(handle, orgUUID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if existing == nil {
		plan := &model.SubscriptionPlan{
			Handle:             handle,
			Name:               name,
			StopOnQuotaReach:   true,
			ThrottleLimitCount: limitCount,
			ThrottleLimitUnit:  limitUnit,
			CreatedAt:          occurredAt.UTC(),
		}
		created, err := s.createPlan(ctx, orgUUID, "", plan, pricing)
		if err == nil || !apperror.SubscriptionPlanExists.Is(err) {
			return created, err
		}
		// A concurrent delivery created it first: reconcile against that copy instead.
		if existing, err = s.planRepo.GetByHandleAndOrg(handle, orgUUID); err != nil {
			return nil, err
		}
	}
	if err := s.reconcilePricing(existing, orgUUID, pricing); err != nil {
		return nil, err
	}

	update := &model.SubscriptionPlanUpdate{}
	changed := false
	if existing.Name != name {
		update.Name = &name
		changed = true
	}
	switch {
	case limitCount == nil:
		if existing.ThrottleLimitCount != nil {
			update.ClearLimit = true
			changed = true
		}
	case existing.ThrottleLimitCount == nil || *existing.ThrottleLimitCount != *limitCount || existing.ThrottleLimitUnit != limitUnit:
		update.ThrottleLimitCount = limitCount
		update.ThrottleLimitUnit = &limitUnit
		changed = true
	}
	if !changed {
		return existing, nil
	}
	// The change comes from the portal itself, so UpdatePlan's rule that a plan in use keeps its
	// name and limit does not apply.
	return s.applyPlanUpdate(ctx, existing, orgUUID, "", update)
}

// reconcilePricing accepts an event for a plan that is already here only when it carries the
// pricing the plan has: both free, or the same pricing. Every event of an API Portal plan carries
// the pricing the plan was created with, and the plan here was made from the first of them. So
// other pricing (a free plan here included) can only mean another plan with the same handle, made
// in the Management Portal or in another API Portal. That event is refused with a conflict, and
// nothing changes.
func (s *SubscriptionPlanService) reconcilePricing(plan *model.SubscriptionPlan, orgUUID string, pricing *model.SubscriptionPlanPricing) error {
	var stored *model.SubscriptionPlanPricing
	if s.pricingRepo != nil {
		var err error
		if stored, err = s.pricingRepo.GetByPlan(plan.UUID, orgUUID); err != nil {
			return err
		}
	}
	if stored == nil && pricing == nil {
		return nil
	}
	if stored != nil && pricing != nil && pricingKey(portalPricing(stored)) == pricingKey(portalPricing(pricing)) {
		return nil
	}
	return apperror.SubscriptionPlanPricingLocked.New().
		WithLogMessage(fmt.Sprintf("plan event for %q carries other pricing than the plan has", plan.Handle))
}

// DeletePlanFromWebhook follows a plan deletion the API Portal reported (deletePlanWithReferences).
//   - A plan that is already gone counts as deleted, so a repeated delivery succeeds.
//   - A plan created after occurredAt is a newer plan with the same handle (made again through the
//     REST API); the deletion was of the older one, so the newer plan is kept.
//
// A plan copied from a portal is dated when it was made in the portal (UpsertPlanFromWebhook), so
// the portal's deletion of it is always later than that date.
func (s *SubscriptionPlanService) DeletePlanFromWebhook(ctx context.Context, orgUUID, handle string, occurredAt time.Time) error {
	existing, err := s.planRepo.GetByHandleAndOrg(handle, orgUUID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && existing == nil) {
		return nil
	}
	if err != nil {
		return err
	}
	if existing.CreatedAt.After(occurredAt) {
		s.slogger.Info("Ignoring the deletion of an older subscription plan with the same handle", "orgId", orgUUID, "plan", handle)
		return nil
	}
	return s.deletePlanWithReferences(ctx, existing, orgUUID, "")
}

// broadcastPlanEvent sends a subscriptionPlan.* event to all gateways in the organization.
func (s *SubscriptionPlanService) broadcastPlanEvent(ctx context.Context, orgUUID, action string, payload interface{}) {
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
			broadcastErr = s.gatewayEvents.BroadcastSubscriptionPlanCreatedEvent(ctx, gw.ID, payload.(*model.SubscriptionPlanCreatedEvent))
		case "updated":
			broadcastErr = s.gatewayEvents.BroadcastSubscriptionPlanUpdatedEvent(ctx, gw.ID, payload.(*model.SubscriptionPlanUpdatedEvent))
		case "deleted":
			broadcastErr = s.gatewayEvents.BroadcastSubscriptionPlanDeletedEvent(ctx, gw.ID, payload.(*model.SubscriptionPlanDeletedEvent))
		}
		if broadcastErr != nil {
			s.slogger.Warn("Failed to broadcast subscriptionPlan event",
				"gatewayId", gw.ID, "action", action, "error", broadcastErr)
		}
	}
}

// subscriptionPlanAuditFields lists the subscription plan fields recorded in audit events.
func subscriptionPlanAuditFields(p *model.SubscriptionPlan) map[string]any {
	fields := map[string]any{
		"name":               p.Name,
		"status":             p.Status,
		"stopOnQuotaReach":   p.StopOnQuotaReach,
		"throttleLimitUnit":  p.ThrottleLimitUnit,
		"throttleLimitCount": nil,
	}
	if p.ThrottleLimitCount != nil {
		fields["throttleLimitCount"] = *p.ThrottleLimitCount
	}
	return fields
}
