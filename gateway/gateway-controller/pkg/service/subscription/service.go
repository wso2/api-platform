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
	"regexp"
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

// MaxExternalRefIDLength matches the width of the subscriptions.uuid column.
const MaxExternalRefIDLength = 64

// externalRefIDPattern restricts caller-supplied subscription IDs to characters that are safe in a URL path segment.
var externalRefIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

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
	// An empty externalRefId is treated as absent, like the other optional identifiers on this request.
	externalRefID := ""
	if req.ExternalRefId != nil && *req.ExternalRefId != "" {
		externalRefID = *req.ExternalRefId
		if len(externalRefID) > MaxExternalRefIDLength || !externalRefIDPattern.MatchString(externalRefID) {
			return nil, &ValidationError{
				Message: fmt.Sprintf("externalRefId must be 1-%d characters of letters, digits, '.', '_' or '-', "+
					"starting with a letter or digit", MaxExternalRefIDLength),
			}
		}
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

	subID := uuid.New().String()
	if externalRefID != "" {
		subID = externalRefID
	}
	sub := &models.Subscription{
		ID:                    subID,
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
	// Existing is the stored subscription, as Get returned it. The caller reads
	// it first (the REST handler before it reads the request body), so a
	// missing subscription is reported before anything else. Update changes it
	// in place.
	Existing *models.Subscription
	Request  api.SubscriptionUpdateRequest
	// Fields holds the request's top-level keys. The generated pointer fields
	// cannot tell an omitted subscriptionPlanId from an explicit null, and the
	// two mean different things.
	Fields        map[string]interface{}
	CorrelationID string
	Logger        *slog.Logger
}

// Update applies a partial update to an existing subscription: its status, its
// plan and its token. Omitted fields are left unchanged.
func (s *SubscriptionService) Update(params UpdateParams) (*Result, error) {
	log := params.Logger
	if log == nil {
		log = slog.Default()
	}
	sub := params.Existing
	req := params.Request
	_, planIDPresent := params.Fields["subscriptionPlanId"]

	// Validate every field before changing sub, so a rejected request leaves it untouched.
	var newStatus *models.SubscriptionStatus
	if req.Status != nil {
		st, err := parseSubscriptionStatus(string(*req.Status))
		if err != nil {
			return nil, err
		}
		newStatus = &st
	}

	// An omitted subscriptionPlanId leaves the plan unchanged; an explicit null or empty string removes it.
	var newPlanID *string
	if planIDPresent && req.SubscriptionPlanId != nil && strings.TrimSpace(*req.SubscriptionPlanId) != "" {
		planID := strings.TrimSpace(*req.SubscriptionPlanId)
		newPlanID = &planID
		// Re-validate only on an actual change, so an unchanged plan that was later deactivated
		// doesn't block an unrelated status or token update.
		if sub.SubscriptionPlanID == nil || *sub.SubscriptionPlanID != planID {
			if err := s.validatePlanForAPI(planID, sub.APIID, log); err != nil {
				return nil, err
			}
		}
	}

	newToken := ""
	if req.SubscriptionToken != nil {
		newToken = strings.TrimSpace(*req.SubscriptionToken)
		if newToken == "" {
			return nil, &ValidationError{Message: "subscriptionToken must not be empty"}
		}
	}

	if newStatus != nil {
		sub.Status = *newStatus
	}
	if planIDPresent {
		sub.SubscriptionPlanID = newPlanID
	}
	if newToken != "" {
		// UpdateSubscription re-hashes a non-empty plain token; otherwise it keeps the stored hash.
		sub.SubscriptionToken = newToken
	}

	if err := s.resources.UpdateSubscription(sub, params.CorrelationID, log); err != nil {
		if storage.IsNotFoundError(err) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, &OpError{Op: OpUpdate, Cause: err}
	}

	return &Result{Subscription: sub}, nil
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
			return "", &OpError{Op: OpResolveByID, Cause: err}
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
		return "", &OpError{Op: OpResolveByHandle, Cause: err}
	}
	if cfg == nil {
		return "", fmt.Errorf("%w: %s", ErrAPINotFound, identifier)
	}

	return cfg.UUID, nil
}

// validatePlanForAPI checks that the plan exists, is ACTIVE and is enabled for the API.
func (s *SubscriptionService) validatePlanForAPI(planID, apiID string, log *slog.Logger) error {
	plan, err := s.db.GetSubscriptionPlanByID(planID, "")
	if err != nil || plan == nil {
		log.Warn("Subscription plan not found for subscription",
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
	if cfg.Kind == string(api.RestAPIKindRestApi) {
		if restAPI, ok := cfg.Configuration.(api.RestAPI); ok {
			if restAPI.Spec.SubscriptionPlans != nil && len(*restAPI.Spec.SubscriptionPlans) > 0 {
				enabled := false
				for _, name := range *restAPI.Spec.SubscriptionPlans {
					if strings.EqualFold(name, plan.PlanName) {
						enabled = true
						break
					}
				}
				if !enabled {
					return &ValidationError{
						Message: fmt.Sprintf("Subscription plan %q is not enabled for this API", plan.PlanName),
					}
				}
			}
		}
	}
	return nil
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
	// Existing is the stored plan, as GetPlan returned it. The caller reads it
	// first (the REST handler before it reads the request body).
	Existing *models.SubscriptionPlan
	Request  api.SubscriptionPlanUpdateRequest
	// Fields holds the request's top-level keys, so an explicit null can be
	// told apart from an omitted field.
	Fields        map[string]interface{}
	CorrelationID string
	Logger        *slog.Logger
}

// UpdatePlan applies a partial update to an existing subscription plan.
// Omitted fields are left unchanged; an explicit null removes billingPlan,
// expiryTime or, sent for both throttle fields, the throttle limit.
func (s *SubscriptionService) UpdatePlan(params UpdatePlanParams) (*PlanResult, error) {
	log := params.Logger
	if log == nil {
		log = slog.Default()
	}
	req := params.Request
	fields := params.Fields

	// Apply the changes to a copy, so a rejected request leaves the stored plan untouched.
	updated := *params.Existing

	if req.PlanName != nil {
		trimmed := strings.TrimSpace(*req.PlanName)
		if trimmed == "" {
			return nil, &ValidationError{Message: "planName cannot be empty"}
		}
		updated.PlanName = trimmed
	}

	// Omitted fields stay unchanged; an explicit null (or empty string) removes billingPlan.
	if _, sent := fields["billingPlan"]; sent {
		if req.BillingPlan == nil || strings.TrimSpace(*req.BillingPlan) == "" {
			updated.BillingPlan = nil
		} else {
			billingPlan := *req.BillingPlan
			updated.BillingPlan = &billingPlan
		}
	}

	if req.StopOnQuotaReach != nil {
		updated.StopOnQuotaReach = *req.StopOnQuotaReach
	}

	// The throttle limit is a pair: both fields are set together, or both sent as null to make the
	// plan unlimited.
	_, countSent := fields["throttleLimitCount"]
	_, unitSent := fields["throttleLimitUnit"]
	if countSent || unitSent {
		countCleared := countSent && req.ThrottleLimitCount == nil
		unitCleared := unitSent && (req.ThrottleLimitUnit == nil || *req.ThrottleLimitUnit == "")
		switch {
		case countCleared && unitCleared:
			updated.ThrottleLimitCount = nil
			updated.ThrottleLimitUnit = nil
		case countCleared || unitCleared:
			return nil, &ValidationError{Message: "throttleLimitCount and throttleLimitUnit must be set or cleared together"}
		default:
			var unitStr *string
			if req.ThrottleLimitUnit != nil {
				unit := string(*req.ThrottleLimitUnit)
				unitStr = &unit
			}
			if err := validateThrottleLimits(req.ThrottleLimitCount, unitStr); err != nil {
				return nil, &ValidationError{Message: err.Error()}
			}
			count := *req.ThrottleLimitCount
			updated.ThrottleLimitCount = &count
			updated.ThrottleLimitUnit = unitStr
		}
	}

	// Omitted expiryTime stays unchanged; an explicit null removes it.
	if _, sent := fields["expiryTime"]; sent {
		updated.ExpiryTime = req.ExpiryTime
	}

	if req.Status != nil {
		parsed, err := parseSubscriptionPlanStatus(string(*req.Status))
		if err != nil {
			return nil, err
		}
		updated.Status = parsed
	}

	if err := s.resources.UpdateSubscriptionPlan(&updated, params.CorrelationID, log); err != nil {
		if storage.IsNotFoundError(err) {
			return nil, ErrPlanNotFound
		}
		return nil, &OpError{Op: OpUpdate, Cause: err}
	}

	return &PlanResult{Plan: &updated}, nil
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
