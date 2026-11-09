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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

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
	auditRepo     repository.AuditRepository
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

// CreatePlan creates a new subscription plan. pricingBody is the pricing request
// (SubscriptionPlanPricingRequest) of a paid plan, or nil for a free plan. The pricing is checked
// with the API Portal's rules and stored with the plan, in one transaction. This is the only place
// pricing is set: it cannot change once the plan exists.
func (s *SubscriptionPlanService) CreatePlan(orgUUID, actor string, plan *model.SubscriptionPlan, pricingBody []byte) (*model.SubscriptionPlan, error) {
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
	return s.createPlan(orgUUID, actor, plan, pricing)
}

// createPlan stores a new plan with its checked pricing (nil for a free plan) in one transaction and
// sends it to the gateways.
func (s *SubscriptionPlanService) createPlan(orgUUID, actor string, plan *model.SubscriptionPlan, pricing *model.SubscriptionPlanPricing) (*model.SubscriptionPlan, error) {
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
	if err := checkPlanDescription(plan.Description); err != nil {
		return nil, err
	}
	conditions, err := normalizeConditions(plan.Conditions)
	if err != nil {
		return nil, err
	}
	plan.Conditions = conditions
	plan.SetLegacyLimit()

	if err := s.planRepo.CreateWithPricing(plan, pricing); err != nil {
		return nil, planStorageError(err)
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
		ThrottleLimitUnit:  gatewayThrottleUnit(plan.ThrottleLimitUnit),
		ExpiryTime:         plan.ExpiryTime,
		Status:             string(plan.Status),
		Conditions:         plan.Conditions,
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
// that offer it, and how many subscriptions use it. A plan in use keeps its name and conditions and
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

// UpdatePlan updates a plan through the REST API. Changes aren't sent to API Portals, and subscribers
// are already on the plan, so the name and conditions can change only while nothing offers or uses it.
// The description, expiry and status can always change. Pricing never changes.
func (s *SubscriptionPlanService) UpdatePlan(handle, orgUUID, actor string, update *model.SubscriptionPlanUpdate) (*model.SubscriptionPlan, error) {
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
	conditions, err := updatedConditions(existing, update)
	if err != nil {
		return nil, err
	}
	if changesPortalValues(existing, update, conditions) {
		usage, err := s.planRepo.GetUsage(existing.UUID, existing.Handle, orgUUID)
		if err != nil {
			return nil, err
		}
		if usage.InUse() {
			return nil, apperror.SubscriptionPlanInUse.New(planEditInUseMessage(existing.Name))
		}
	}
	return s.applyPlanUpdate(existing, orgUUID, actor, update, conditions)
}

// changesPortalValues reports whether the update changes the name or conditions, which a plan in use
// keeps. Values are compared, not which fields were sent; names ignore surrounding spaces.
func changesPortalValues(existing *model.SubscriptionPlan, update *model.SubscriptionPlanUpdate, conditions []model.SubscriptionPlanCondition) bool {
	if update.Name != nil && strings.TrimSpace(*update.Name) != strings.TrimSpace(existing.Name) {
		return true
	}
	return !sameConditions(existing.Conditions, conditions)
}

// updatedConditions returns the plan's conditions after the update, normalized. An old plan read from
// subscription_plan_limits equals the same limit sent as conditions, so saving it unchanged isn't a change.
func updatedConditions(existing *model.SubscriptionPlan, update *model.SubscriptionPlanUpdate) ([]model.SubscriptionPlanCondition, error) {
	current, err := normalizeConditions(existing.Conditions)
	if err != nil {
		// Keep a stored condition that no longer validates as it is.
		current = existing.Conditions
	}
	existing.Conditions = current
	switch {
	case update.Conditions != nil:
		return normalizeConditions(*update.Conditions)
	case update.RequestLimit != nil:
		return normalizeConditions(withRequestLimit(current, update.RequestLimit))
	case update.ClearLimit:
		return normalizeConditions(withRequestLimit(current, nil))
	default:
		return current, nil
	}
}

// applyPlanUpdate saves the update with the given conditions and tells the gateways.
func (s *SubscriptionPlanService) applyPlanUpdate(existing *model.SubscriptionPlan, orgUUID, actor string, update *model.SubscriptionPlanUpdate, conditions []model.SubscriptionPlanCondition) (*model.SubscriptionPlan, error) {
	if update.Name != nil {
		name := strings.TrimSpace(*update.Name)
		if name == "" {
			return nil, apperror.ValidationFailed.New("name is required")
		}
		existing.Name = name
	}
	if update.Description != nil {
		existing.Description = strings.TrimSpace(*update.Description)
		if err := checkPlanDescription(existing.Description); err != nil {
			return nil, err
		}
	}
	existing.Conditions = conditions
	existing.SetLegacyLimit()
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
		return nil, planStorageError(err)
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
		ThrottleLimitUnit:  gatewayThrottleUnit(existing.ThrottleLimitUnit),
		ExpiryTime:         existing.ExpiryTime,
		Status:             string(existing.Status),
		Conditions:         existing.Conditions,
	})

	return existing, nil
}

// checkPlanDescription refuses a description longer than the column holds.
func checkPlanDescription(description string) error {
	if utf8.RuneCountInString(description) > model.MaxSubscriptionPlanDescriptionChars {
		return apperror.ValidationFailed.New(fmt.Sprintf("description must be at most %d characters", model.MaxSubscriptionPlanDescriptionChars))
	}
	return nil
}

// truncateRunes cuts s to at most n characters.
func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// gatewayThrottleUnits maps platform-api time units to the names gateways use for a plan's throttle
// unit (Min, Hour, Day, Month), which subscription-validation v1 reads.
var gatewayThrottleUnits = map[string]string{
	constants.ThrottleLimitUnitMinute: "Min",
	constants.ThrottleLimitUnitHour:   "Hour",
	constants.ThrottleLimitUnitDay:    "Day",
	constants.ThrottleLimitUnitMonth:  "Month",
}

// gatewayThrottleUnit converts a time unit for gateways. Unknown values are passed through.
func gatewayThrottleUnit(unit string) string {
	if gw, ok := gatewayThrottleUnits[strings.ToUpper(unit)]; ok {
		return gw
	}
	return unit
}

// planStorageError reports a database not yet upgraded for conditions with a clear message.
func planStorageError(err error) error {
	switch {
	case errors.Is(err, repository.ErrPlanConditionsTableMissing):
		return apperror.SubscriptionPlanConditionsUnsupported.Wrap(err,
			"Only request limits can be saved until the database is upgraded for subscription plan conditions.").WithLogMessage(err.Error())
	case errors.Is(err, repository.ErrPlanDescriptionColumnMissing):
		return apperror.SubscriptionPlanConditionsUnsupported.Wrap(err,
			"A plan description can be saved only after the database is upgraded for subscription plan conditions.").WithLogMessage(err.Error())
	}
	return err
}

// DeletePlan deletes a subscription plan through the REST API, only while nothing here offers or
// uses it. API Portals that have the plan keep it: a delete made here is not sent to them.
func (s *SubscriptionPlanService) DeletePlan(handle, orgUUID, actor string) error {
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
	return s.deletePlanWithReferences(existing, orgUUID, actor)
}

// deletePlanWithReferences deletes a plan and its references, in one transaction:
//   - the plan is removed from the REST APIs and artifact_subscription_plans rows that offer it;
//   - the publication drafts and live publications that offer it are kept, showing it as deleted;
//   - its subscriptions are kept, without a plan;
//   - the plan is deleted, with its conditions and pricing.
//
// The gateways are then told the plan is deleted. DeletePlan calls it only once nothing uses the
// plan, so there it deletes just the plan. DeletePlanFromWebhook calls it for a plan the API Portal
// deleted, which may still be offered or used here.
func (s *SubscriptionPlanService) deletePlanWithReferences(plan *model.SubscriptionPlan, orgUUID, actor string) error {
	deleted, err := s.planRepo.DeleteWithReferences(plan.UUID, plan.Handle, orgUUID)
	if err != nil {
		return err
	}
	if !deleted {
		// A concurrent delete removed it first.
		return nil
	}
	if s.auditRepo != nil {
		_ = s.auditRepo.Record("DELETE", plan.UUID, "subscription_plan", orgUUID, actor)
	}

	s.broadcastPlanEvent(orgUUID, "deleted", &model.SubscriptionPlanDeletedEvent{
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

// planEditInUseMessage refuses a change to the name or conditions of a plan in use, for example "The
// plan Gold is in use. To change its name or conditions, create a new plan."
func planEditInUseMessage(name string) string {
	return fmt.Sprintf("The plan %s is in use. To change its name or conditions, create a new plan.", name)
}

// UpsertPlanFromWebhook applies a plan created or changed in the API Portal.
//   - A new plan is created with its pricing, dated when the portal made it, so a later deletion
//     event isn't mistaken for the deletion of an older plan with the same handle.
//   - An existing plan must keep its pricing (reconcilePricing); its name, description and conditions
//     are updated when they differ, so an echo of a change made here changes nothing.
//
// Portals that predate conditions send only request limits (requestWindows); those change only the
// request-count condition (portalConditions). Meters used by the prices must already exist here.
func (s *SubscriptionPlanService) UpsertPlanFromWebhook(orgUUID, handle, name, description string, conditions *[]model.SubscriptionPlanCondition, requestWindows []model.ConditionWindow, pricing *model.SubscriptionPlanPricing, occurredAt time.Time) (*model.SubscriptionPlan, error) {
	// Stored trimmed, so compare trimmed: a name with edge spaces would otherwise update on every event.
	name = strings.TrimSpace(name)
	// A portal on SQLite may hold a longer description than the column; keep the plan in sync.
	description = truncateRunes(strings.TrimSpace(description), model.MaxSubscriptionPlanDescriptionChars)
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
		planConditions, err := portalConditions(nil, conditions, requestWindows)
		if err != nil {
			return nil, err
		}
		plan := &model.SubscriptionPlan{
			Handle:              handle,
			Name:                name,
			Description:         description,
			DescriptionOptional: true,
			Conditions:          planConditions,
			CreatedAt:           occurredAt.UTC(),
		}
		created, err := s.createPlan(orgUUID, "", plan, pricing)
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

	current, err := updatedConditions(existing, &model.SubscriptionPlanUpdate{})
	if err != nil {
		return nil, err
	}
	planConditions, err := portalConditions(current, conditions, requestWindows)
	if err != nil {
		return nil, err
	}
	update := &model.SubscriptionPlanUpdate{Conditions: &planConditions}
	changed := false
	if existing.Name != name {
		update.Name = &name
		changed = true
	}
	if existing.Description != description {
		update.Description = &description
		changed = true
	}
	if !sameConditions(current, planConditions) {
		changed = true
	}
	if !changed {
		return existing, nil
	}
	// The change comes from the portal itself, so UpdatePlan's rule that a plan in use keeps its
	// name and conditions does not apply.
	existing.DescriptionOptional = true
	return s.applyPlanUpdate(existing, orgUUID, "", update, planConditions)
}

// portalConditions returns the conditions a portal event gives the plan: the event's conditions, or,
// from an older portal, the current ones with only the request-count condition replaced by its limits
// (keeping onExceed, which that portal can't set).
func portalConditions(current []model.SubscriptionPlanCondition, conditions *[]model.SubscriptionPlanCondition, requestWindows []model.ConditionWindow) ([]model.SubscriptionPlanCondition, error) {
	if conditions != nil {
		return normalizeConditions(*conditions)
	}
	if len(requestWindows) == 0 {
		return normalizeConditions(withRequestLimit(current, nil))
	}
	stopOnQuotaReach := true
	if c := model.FindCondition(current, model.ConditionTypeRequestCount); c != nil {
		var params model.ConditionLimits
		if err := json.Unmarshal(c.Params, &params); err == nil {
			stopOnQuotaReach = params.OnExceed != model.ConditionOnExceedAllow
		}
	}
	requestLimit := model.NewRequestCountCondition(requestWindows, stopOnQuotaReach)
	return normalizeConditions(withRequestLimit(current, &requestLimit))
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
func (s *SubscriptionPlanService) DeletePlanFromWebhook(orgUUID, handle string, occurredAt time.Time) error {
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
	return s.deletePlanWithReferences(existing, orgUUID, "")
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
