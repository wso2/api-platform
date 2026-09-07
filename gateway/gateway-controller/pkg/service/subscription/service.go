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

// Package subscription holds the business rules for subscriptions and
// subscription plans, so every entry point — the management REST API, the MCP
// endpoint, or any future caller — validates a request the same way.
package subscription

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// Result holds the result of a single-subscription operation.
type Result struct {
	Subscription *models.Subscription
}

// ListResult holds the result of a List operation.
type ListResult struct {
	Subscriptions []*models.Subscription
}

// PlanResult holds the result of a single-plan operation.
type PlanResult struct {
	Plan *models.SubscriptionPlan
}

// PlanListResult holds the result of a ListPlans operation.
type PlanListResult struct {
	Plans []*models.SubscriptionPlan
}

// SubscriptionService applies request-shaped business rules — identifier
// resolution, plan eligibility, enum parsing, model construction — on top of the
// persistence and replica-sync publishing that SubscriptionResourceService owns.
//
// Every mutation goes through s.resources so the persist-then-publish ordering
// lives in exactly one place; s.db is used for reads only.
type SubscriptionService struct {
	db        storage.Storage
	resources *utils.SubscriptionResourceService
}

// NewSubscriptionService creates a new SubscriptionService.
func NewSubscriptionService(db storage.Storage, resources *utils.SubscriptionResourceService) *SubscriptionService {
	if db == nil {
		panic("SubscriptionService requires non-nil storage")
	}
	if resources == nil {
		panic("SubscriptionService requires SubscriptionResourceService")
	}

	return &SubscriptionService{
		db:        db,
		resources: resources,
	}
}

// Subscriptions

// ListFilter narrows a subscription listing. APIID is an already-resolved
// deployment ID; the empty string means every API on this gateway.
type ListFilter struct {
	APIID         string
	ApplicationID *string
	Status        *string
}

// CreateParams holds parameters for the Create operation.
type CreateParams struct {
	Request       api.SubscriptionCreateRequest
	CorrelationID string
	Logger        *slog.Logger
}

// Create validates and stores a new subscription.
//
// Request.ApiId accepts either a deployment ID or a handle (metadata.name); it
// is resolved here rather than by the caller, so an MCP tool and the REST
// handler accept exactly the same identifiers.
func (s *SubscriptionService) Create(params CreateParams) (*Result, error) {
	log := params.Logger
	if log == nil {
		log = slog.Default()
	}
	req := params.Request

	if strings.TrimSpace(req.ApiId) == "" {
		return nil, &ValidationError{Message: "apiId is required"}
	}
	if strings.TrimSpace(req.SubscriptionToken) == "" {
		return nil, &ValidationError{Message: "subscriptionToken is required"}
	}

	apiID, err := s.ResolveAPIID(req.ApiId)
	if err != nil {
		return nil, err
	}

	if req.SubscriptionPlanId != nil && *req.SubscriptionPlanId != "" {
		if err := s.validatePlanForAPI(*req.SubscriptionPlanId, apiID, log); err != nil {
			return nil, err
		}
	}

	status := models.SubscriptionStatusActive
	if req.Status != nil {
		parsed, err := parseSubscriptionStatus(string(*req.Status))
		if err != nil {
			return nil, err
		}
		status = parsed
	}

	var appID *string
	if req.ApplicationId != nil && *req.ApplicationId != "" {
		appID = req.ApplicationId
	}

	sub := &models.Subscription{
		ID:                    uuid.New().String(),
		APIID:                 apiID,
		ApplicationID:         appID,
		SubscriptionPlanID:    req.SubscriptionPlanId,
		BillingCustomerID:     req.BillingCustomerId,
		BillingSubscriptionID: req.BillingSubscriptionId,
		Status:                status,
		SubscriptionToken:     strings.TrimSpace(req.SubscriptionToken),
	}

	if err := s.resources.SaveSubscription(sub, params.CorrelationID, log); err != nil {
		return nil, &OpError{Op: OpCreate, Cause: err}
	}

	return &Result{Subscription: sub}, nil
}

// UpdateParams holds parameters for the Update operation.
type UpdateParams struct {
	ID            string
	Request       api.SubscriptionUpdateRequest
	CorrelationID string
	Logger        *slog.Logger
}

// Update applies a partial update to an existing subscription.
//
// api.SubscriptionUpdateRequest carries exactly one field, Status, so this is a
// status change and nothing else: the API, application, plan and billing
// identifiers of an existing subscription cannot be re-pointed.
func (s *SubscriptionService) Update(params UpdateParams) (*Result, error) {
	log := params.Logger
	if log == nil {
		log = slog.Default()
	}

	existing, err := s.loadSubscription(params.ID)
	if err != nil {
		return nil, err
	}

	if params.Request.Status != nil {
		parsed, err := parseSubscriptionStatus(string(*params.Request.Status))
		if err != nil {
			return nil, err
		}
		existing.Status = parsed
	}

	if err := s.resources.UpdateSubscription(existing, params.CorrelationID, log); err != nil {
		if storage.IsNotFoundError(err) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, &OpError{Op: OpUpdate, Cause: err}
	}

	return &Result{Subscription: existing}, nil
}

// Get retrieves a subscription by ID.
func (s *SubscriptionService) Get(id string) (*Result, error) {
	sub, err := s.loadSubscription(id)
	if err != nil {
		return nil, err
	}

	return &Result{Subscription: sub}, nil
}

// List returns the subscriptions matching filter.
func (s *SubscriptionService) List(filter ListFilter) (*ListResult, error) {
	// The gatewayID argument is ignored by the storage layer, which always scopes
	// to its own gateway; the empty string is what every caller passes.
	subs, err := s.db.ListSubscriptionsByAPI(filter.APIID, "", filter.ApplicationID, filter.Status)
	if err != nil {
		return nil, &OpError{Op: OpList, Cause: err}
	}

	return &ListResult{Subscriptions: subs}, nil
}

// Delete removes a subscription, reporting ErrSubscriptionNotFound when it does
// not exist. The existence check is explicit rather than inferred from the
// delete result, matching what the REST handler did.
func (s *SubscriptionService) Delete(id, correlationID string, log *slog.Logger) error {
	if log == nil {
		log = slog.Default()
	}

	if _, err := s.loadSubscription(id); err != nil {
		return err
	}

	if err := s.resources.DeleteSubscription(id, correlationID, log); err != nil {
		if storage.IsNotFoundError(err) {
			return ErrSubscriptionNotFound
		}
		return &OpError{Op: OpDelete, Cause: err}
	}

	return nil
}

// ResolveAPIID resolves an API identifier (deployment ID or metadata.name
// handle) to the internal deployment ID used for subscription persistence. It
// tries a direct ID lookup first, then falls back to handle resolution.
func (s *SubscriptionService) ResolveAPIID(identifier string) (string, error) {
	cfgByID, err := s.db.GetConfig(identifier)
	if err != nil {
		if !storage.IsNotFoundError(err) {
			return "", &OpError{Op: OpResolve, Cause: err}
		}
	} else if cfgByID != nil {
		if cfgByID.Kind != string(api.RestAPIKindRestApi) {
			return "", &NotRestAPIError{Identifier: identifier, Kind: cfgByID.Kind}
		}
		return cfgByID.UUID, nil
	}

	cfg, err := s.db.GetConfigByKindAndHandle(models.KindRestApi, identifier)
	if err != nil {
		if storage.IsNotFoundError(err) {
			return "", fmt.Errorf("%w: %s", ErrAPINotFound, identifier)
		}
		return "", &OpError{Op: OpResolve, Cause: err}
	}
	if cfg == nil {
		return "", fmt.Errorf("%w: %s", ErrAPINotFound, identifier)
	}

	return cfg.UUID, nil
}

// validatePlanForAPI checks that the plan exists, is ACTIVE, and — when the API
// declares an explicit plan list — that it is one of the plans that API offers.
// An API with no declared list accepts any active plan.
func (s *SubscriptionService) validatePlanForAPI(planID, apiID string, log *slog.Logger) error {
	plan, err := s.db.GetSubscriptionPlanByID(planID, "")
	if err != nil || plan == nil {
		log.Warn("Subscription plan not found for subscription creation",
			slog.String("subscription_plan_id", planID),
			slog.String("api_id", apiID))
		return &ValidationError{Message: "Subscription plan not found or not enabled"}
	}
	if plan.Status != models.SubscriptionPlanStatusActive {
		return &ValidationError{Message: "Subscription plan is not active"}
	}

	cfg, err := s.db.GetConfig(apiID)
	if err != nil || cfg == nil {
		log.Error("Failed to load API configuration for subscription plan validation",
			slog.String("api_id", apiID), slog.Any("error", err))
		return &OpError{Op: OpValidate, Cause: err}
	}
	if cfg.Kind != string(api.RestAPIKindRestApi) {
		return nil
	}

	restAPI, ok := cfg.Configuration.(api.RestAPI)
	if !ok {
		return nil
	}
	if restAPI.Spec.SubscriptionPlans == nil || len(*restAPI.Spec.SubscriptionPlans) == 0 {
		return nil
	}
	for _, name := range *restAPI.Spec.SubscriptionPlans {
		if strings.EqualFold(name, plan.PlanName) {
			return nil
		}
	}

	return &ValidationError{
		Message: fmt.Sprintf("Subscription plan %q is not enabled for this API", plan.PlanName),
	}
}

// loadSubscription reads one subscription, normalising both a not-found error
// and a nil row to ErrSubscriptionNotFound. The storage layer can return either.
func (s *SubscriptionService) loadSubscription(id string) (*models.Subscription, error) {
	sub, err := s.db.GetSubscriptionByID(id, "")
	if err != nil {
		if storage.IsNotFoundError(err) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, &OpError{Op: OpLoad, Cause: err}
	}
	if sub == nil {
		return nil, ErrSubscriptionNotFound
	}

	return sub, nil
}

// parseSubscriptionStatus resolves a caller-supplied status to its model
// constant, rejecting anything outside the enum.
func parseSubscriptionStatus(raw string) (models.SubscriptionStatus, error) {
	status := models.SubscriptionStatus(raw)
	switch status {
	case models.SubscriptionStatusActive,
		models.SubscriptionStatusInactive,
		models.SubscriptionStatusRevoked:
		return status, nil
	default:
		return "", &ValidationError{Message: fmt.Sprintf("invalid status: %s", raw)}
	}
}

// Subscription plans

// CreatePlanParams holds parameters for the CreatePlan operation.
type CreatePlanParams struct {
	Request       api.SubscriptionPlanCreateRequest
	CorrelationID string
	Logger        *slog.Logger
}

// CreatePlan validates and stores a new subscription plan.
func (s *SubscriptionService) CreatePlan(params CreatePlanParams) (*PlanResult, error) {
	log := params.Logger
	if log == nil {
		log = slog.Default()
	}
	req := params.Request

	planName := strings.TrimSpace(req.PlanName)
	if planName == "" {
		return nil, &ValidationError{Message: "planName is required"}
	}

	var unitStr *string
	if req.ThrottleLimitUnit != nil {
		unit := string(*req.ThrottleLimitUnit)
		unitStr = &unit
	}
	if err := validateThrottleLimits(req.ThrottleLimitCount, unitStr); err != nil {
		return nil, &ValidationError{Message: err.Error()}
	}

	status := models.SubscriptionPlanStatusActive
	if req.Status != nil {
		parsed, err := parseSubscriptionPlanStatus(string(*req.Status))
		if err != nil {
			return nil, err
		}
		status = parsed
	}

	plan := &models.SubscriptionPlan{
		ID:               uuid.New().String(),
		PlanName:         planName,
		StopOnQuotaReach: true,
		Status:           status,
	}
	if req.BillingPlan != nil {
		plan.BillingPlan = req.BillingPlan
	}
	if req.StopOnQuotaReach != nil {
		plan.StopOnQuotaReach = *req.StopOnQuotaReach
	}
	if req.ThrottleLimitCount != nil && req.ThrottleLimitUnit != nil {
		unit := string(*req.ThrottleLimitUnit)
		plan.ThrottleLimitCount = req.ThrottleLimitCount
		plan.ThrottleLimitUnit = &unit
	}
	if req.ExpiryTime != nil {
		plan.ExpiryTime = req.ExpiryTime
	}

	if err := s.resources.SaveSubscriptionPlan(plan, params.CorrelationID, log); err != nil {
		return nil, &OpError{Op: OpCreate, Cause: err}
	}

	return &PlanResult{Plan: plan}, nil
}

// UpdatePlanParams holds parameters for the UpdatePlan operation.
type UpdatePlanParams struct {
	ID            string
	Request       api.SubscriptionPlanUpdateRequest
	CorrelationID string
	Logger        *slog.Logger
}

// UpdatePlan applies a partial update to an existing subscription plan.
func (s *SubscriptionService) UpdatePlan(params UpdatePlanParams) (*PlanResult, error) {
	log := params.Logger
	if log == nil {
		log = slog.Default()
	}

	existing, err := s.loadPlan(params.ID)
	if err != nil {
		return nil, err
	}
	req := params.Request

	// Throttle limits are validated before any field is applied, matching the
	// order the REST handler used: a bad limit rejects the whole request rather
	// than half-applying it.
	var unitStr *string
	if req.ThrottleLimitUnit != nil {
		unit := string(*req.ThrottleLimitUnit)
		unitStr = &unit
	}
	if err := validateThrottleLimits(req.ThrottleLimitCount, unitStr); err != nil {
		return nil, &ValidationError{Message: err.Error()}
	}

	if req.PlanName != nil {
		trimmed := strings.TrimSpace(*req.PlanName)
		if trimmed == "" {
			return nil, &ValidationError{Message: "planName cannot be empty"}
		}
		existing.PlanName = trimmed
	}
	if req.BillingPlan != nil {
		existing.BillingPlan = req.BillingPlan
	}
	if req.StopOnQuotaReach != nil {
		existing.StopOnQuotaReach = *req.StopOnQuotaReach
	}
	if req.ThrottleLimitCount != nil && req.ThrottleLimitUnit != nil {
		unit := string(*req.ThrottleLimitUnit)
		existing.ThrottleLimitCount = req.ThrottleLimitCount
		existing.ThrottleLimitUnit = &unit
	}
	if req.ExpiryTime != nil {
		existing.ExpiryTime = req.ExpiryTime
	}
	if req.Status != nil {
		parsed, err := parseSubscriptionPlanStatus(string(*req.Status))
		if err != nil {
			return nil, err
		}
		existing.Status = parsed
	}

	if err := s.resources.UpdateSubscriptionPlan(existing, params.CorrelationID, log); err != nil {
		return nil, &OpError{Op: OpUpdate, Cause: err}
	}

	return &PlanResult{Plan: existing}, nil
}

// GetPlan retrieves a subscription plan by ID.
func (s *SubscriptionService) GetPlan(id string) (*PlanResult, error) {
	plan, err := s.loadPlan(id)
	if err != nil {
		return nil, err
	}

	return &PlanResult{Plan: plan}, nil
}

// ListPlans returns every subscription plan on this gateway.
func (s *SubscriptionService) ListPlans() (*PlanListResult, error) {
	plans, err := s.db.ListSubscriptionPlans("")
	if err != nil {
		return nil, &OpError{Op: OpList, Cause: err}
	}

	return &PlanListResult{Plans: plans}, nil
}

// DeletePlan removes a subscription plan.
func (s *SubscriptionService) DeletePlan(id, correlationID string, log *slog.Logger) error {
	if log == nil {
		log = slog.Default()
	}

	if err := s.resources.DeleteSubscriptionPlan(id, correlationID, log); err != nil {
		if storage.IsNotFoundError(err) {
			return ErrPlanNotFound
		}
		return &OpError{Op: OpDelete, Cause: err}
	}

	return nil
}

// loadPlan reads one plan, normalising both a not-found error and a nil row to
// ErrPlanNotFound. The storage layer can return either.
func (s *SubscriptionService) loadPlan(id string) (*models.SubscriptionPlan, error) {
	// The gatewayID argument is ignored by the storage layer, which always scopes
	// to its own gateway; the empty string is what every caller passes.
	plan, err := s.db.GetSubscriptionPlanByID(id, "")
	if err != nil {
		if storage.IsNotFoundError(err) {
			return nil, ErrPlanNotFound
		}
		return nil, &OpError{Op: OpLoad, Cause: err}
	}
	if plan == nil {
		return nil, ErrPlanNotFound
	}

	return plan, nil
}

// parseSubscriptionPlanStatus resolves a caller-supplied status to its model
// constant, rejecting anything outside the enum.
func parseSubscriptionPlanStatus(raw string) (models.SubscriptionPlanStatus, error) {
	status := models.SubscriptionPlanStatus(raw)
	switch status {
	case models.SubscriptionPlanStatusActive, models.SubscriptionPlanStatusInactive:
		return status, nil
	default:
		return "", &ValidationError{Message: fmt.Sprintf("invalid status: %s", raw)}
	}
}

// validateThrottleLimits ensures throttleLimitCount and throttleLimitUnit are provided together,
// count is positive, and unit is one of Day, Hour, Min, Month.
func validateThrottleLimits(count *int, unit *string) error {
	countProvided := count != nil
	unitProvided := unit != nil && *unit != ""
	if countProvided != unitProvided {
		return fmt.Errorf("throttleLimitCount and throttleLimitUnit must be provided together")
	}
	if !countProvided {
		return nil
	}
	if *count <= 0 {
		return fmt.Errorf("throttleLimitCount must be positive")
	}
	switch *unit {
	case "Day", "Hour", "Min", "Month":
		return nil
	default:
		return fmt.Errorf("throttleLimitUnit must be one of: Day, Hour, Min, Month")
	}
}
