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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

const (
	// portalPlansPageSize is below the portal's own maximum page size.
	portalPlansPageSize = 50
	// portalPlansMaxPages bounds the listing loop; running into it is an error,
	// not a short list, so plans that exist are never mistaken for missing ones.
	portalPlansMaxPages = 250
	// portalPlansMaxResponseBytes bounds one page of the plan listing.
	portalPlansMaxResponseBytes = 1 << 20 // 1 MiB
)

// portalPlanList is the part of the portal's plan listing this package reads.
type portalPlanList struct {
	List       []portalListedPlan `json:"list"`
	Pagination struct {
		Total int `json:"total"`
	} `json:"pagination"`
}

// portalListedPlan is one plan of the portal's listing: what a publish compares with the plan
// it offers. Pricing is absent for a free plan.
type portalListedPlan struct {
	ID          string              `json:"id"`
	DisplayName string              `json:"displayName"`
	Limits      []portalPlanLimit   `json:"limits"`
	Pricing     *planPricingRequest `json:"pricing"`
}

// portalPlan is one entry of the portal's bulk plan create.
type portalPlan struct {
	ID          string            `json:"id"`
	RefID       string            `json:"refId"`
	DisplayName string            `json:"displayName"`
	Limits      []portalPlanLimit `json:"limits,omitempty"`
	// Pricing is set for a paid plan: the portal saves the plan and its pricing together, or
	// neither.
	Pricing *planPricingRequest `json:"pricing,omitempty"`
}

// portalPlanUpdate is one entry of the portal's bulk plan update. It carries no pricing, which
// the portal refuses for a plan that exists, and always carries limits, since the portal keeps a
// plan's limits when the field is absent.
type portalPlanUpdate struct {
	ID          string            `json:"id"`
	DisplayName string            `json:"displayName"`
	Limits      []portalPlanLimit `json:"limits"`
}

type portalPlanLimit struct {
	LimitType  string `json:"limitType"`
	LimitCount int    `json:"limitCount"`
	TimeUnit   string `json:"timeUnit,omitempty"`
	TimeAmount int    `json:"timeAmount"`
}

// SyncPlans implements PortalPublisher, in this order:
//  1. lists the portal's plans, and refuses when a copy is priced differently;
//  2. checks the portal's copies of the meters the missing plans charge for;
//  3. creates the missing meters, one by one;
//  4. creates the missing plans in one bulk request, each paid one with its pricing;
//  5. updates the copies whose name or limit differs, in another bulk request.
//
// When a write fails, what was created before it is removed again (undoAfter).
func (p *HTTPPortalPublisher) SyncPlans(ctx context.Context, portal *model.APIPortal, sync PlanSync) (PlanSyncResult, error) {
	var created PlanSyncResult
	if len(sync.Plans) == 0 {
		return created, nil
	}
	listed, err := p.findPortalPlans(ctx, portal, sync.Plans)
	if err != nil {
		return created, err
	}
	var missing, changed []*model.SubscriptionPlan
	for _, plan := range sync.Plans {
		portalCopy, ok := listed[plan.Handle]
		switch {
		case !ok:
			missing = append(missing, plan)
		case !samePricing(sync.Pricing[plan.UUID], portalCopy.Pricing):
			return created, &PortalPlanPricingConflictError{Handle: plan.Handle}
		case strings.TrimSpace(portalCopy.DisplayName) != strings.TrimSpace(plan.Name) ||
			!sameLimits(portalLimits(plan), portalCopy.Limits):
			changed = append(changed, plan)
		}
	}
	meters, err := p.missingMeters(ctx, portal, pricedMeters(missing, sync))
	if err != nil {
		return created, err
	}
	for _, meter := range meters {
		if err := p.createMeter(ctx, portal, meter); err != nil {
			return PlanSyncResult{}, p.undoAfter(ctx, portal, created, err)
		}
		created.Meters = append(created.Meters, meter.Handle)
	}
	if len(missing) > 0 {
		if err := p.createPlans(ctx, portal, missing, sync.Pricing); err != nil {
			return PlanSyncResult{}, p.undoAfter(ctx, portal, created, err)
		}
		for _, plan := range missing {
			created.Plans = append(created.Plans, plan.Handle)
		}
	}
	if len(changed) > 0 {
		if err := p.updatePlans(ctx, portal, changed, sync.Pricing); err != nil {
			return PlanSyncResult{}, p.undoAfter(ctx, portal, created, err)
		}
	}
	return created, nil
}

// findPortalPlans pages through the portal's plans and returns its copies of plans, by handle. It
// stops once every plan has been seen or the listing ends.
func (p *HTTPPortalPublisher) findPortalPlans(ctx context.Context, portal *model.APIPortal, plans []*model.SubscriptionPlan) (map[string]*portalListedPlan, error) {
	ctx, cancel := context.WithTimeout(ctx, p.plansListingTimeout)
	defer cancel()

	wanted := make(map[string]struct{}, len(plans))
	for _, plan := range plans {
		wanted[plan.Handle] = struct{}{}
	}
	found := make(map[string]*portalListedPlan, len(plans))
	for page := 0; page < portalPlansMaxPages; page++ {
		offset := page * portalPlansPageSize
		list, err := p.listPlansPage(ctx, portal, offset)
		if err != nil {
			return nil, err
		}
		for i := range list.List {
			item := &list.List[i]
			if _, ok := wanted[item.ID]; ok {
				found[item.ID] = item
			}
		}
		if len(found) == len(wanted) || len(list.List) == 0 || offset+len(list.List) >= list.Pagination.Total {
			return found, nil
		}
	}
	return nil, fmt.Errorf("portal plan listing exceeds %d pages", portalPlansMaxPages)
}

func (p *HTTPPortalPublisher) listPlansPage(ctx context.Context, portal *model.APIPortal, offset int) (*portalPlanList, error) {
	authHeader, err := p.authHeader(ctx, portal)
	if err != nil {
		return nil, err
	}

	target := fmt.Sprintf("%s?limit=%d&offset=%d", plansURL(portal), portalPlansPageSize, offset)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build portal plan listing request: %w", err)
	}
	req.Header.Set("Authorization", authHeader)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("portal plan listing failed: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		var list portalPlanList
		if err := json.NewDecoder(io.LimitReader(resp.Body, portalPlansMaxResponseBytes)).Decode(&list); err != nil {
			return nil, fmt.Errorf("portal plan listing returned an unreadable body: %w", err)
		}
		return &list, nil
	case isPortalAuthFailure(resp.StatusCode):
		return nil, portalAuthError(resp.StatusCode)
	case isPortalRejection(resp.StatusCode):
		return nil, portalRejection(resp, "the API Portal rejected the subscription plan listing")
	default:
		return nil, fmt.Errorf("portal plan listing failed: unexpected status %d", resp.StatusCode)
	}
}

// createPlans sends one bulk create. The portal applies it all or nothing.
func (p *HTTPPortalPublisher) createPlans(ctx context.Context, portal *model.APIPortal, plans []*model.SubscriptionPlan, pricing map[string]*model.SubscriptionPlanPricing) error {
	payload := make([]portalPlan, 0, len(plans))
	for _, plan := range plans {
		payload = append(payload, toPortalPlan(plan, pricing[plan.UUID]))
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to encode portal subscription plans: %w", err)
	}

	authHeader, err := p.authHeader(ctx, portal)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, plansURL(portal), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to build portal plan create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("portal plan create failed: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusCreated:
		return nil
	case isPortalAuthFailure(resp.StatusCode):
		return portalAuthError(resp.StatusCode)
	case isPortalRejection(resp.StatusCode):
		return portalRejection(resp, "the API Portal rejected the subscription plans")
	default:
		return fmt.Errorf("portal plan create failed: unexpected status %d", resp.StatusCode)
	}
}

// updatePlans sends one bulk update of the plans' names and limits, which the portal applies all
// or nothing. If a copy was deleted in the portal since the listing, the portal's update creates it
// again (201), without pricing. That is fine for a free plan, but a paid plan would become a free
// copy, so it is reported as a pricing conflict.
func (p *HTTPPortalPublisher) updatePlans(ctx context.Context, portal *model.APIPortal, plans []*model.SubscriptionPlan, pricing map[string]*model.SubscriptionPlanPricing) error {
	payload := make([]portalPlanUpdate, 0, len(plans))
	for _, plan := range plans {
		payload = append(payload, portalPlanUpdate{ID: plan.Handle, DisplayName: plan.Name, Limits: portalLimits(plan)})
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to encode portal subscription plan updates: %w", err)
	}

	authHeader, err := p.authHeader(ctx, portal)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, plansURL(portal), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to build portal plan update request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("portal plan update failed: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		return nil
	case resp.StatusCode == http.StatusCreated:
		for _, plan := range plans {
			if pricing[plan.UUID] != nil {
				return &PortalPlanPricingConflictError{Handle: plan.Handle}
			}
		}
		return nil
	case isPortalAuthFailure(resp.StatusCode):
		return portalAuthError(resp.StatusCode)
	case isPortalRejection(resp.StatusCode):
		return portalRejection(resp, "the API Portal rejected the subscription plan updates")
	default:
		return fmt.Errorf("portal plan update failed: unexpected status %d", resp.StatusCode)
	}
}

// toPortalPlan maps a plan to the portal's shape. A plan without a throttle
// limit is sent with no limits, which the portal treats as unlimited. The
// reference is the handle: the portal names a plan by its reference in the
// subscription and subscription_plan events it sends back, and those are
// resolved here by handle. pricing is nil for a free plan.
func toPortalPlan(plan *model.SubscriptionPlan, pricing *model.SubscriptionPlanPricing) portalPlan {
	out := portalPlan{ID: plan.Handle, RefID: plan.Handle, DisplayName: plan.Name}
	if pricing != nil {
		out.Pricing = portalPricing(pricing)
	}
	if limits := portalLimits(plan); len(limits) > 0 {
		out.Limits = limits
	}
	return out
}

// portalLimits is a plan's request limit in the portal's shape, empty when the plan has none.
func portalLimits(plan *model.SubscriptionPlan) []portalPlanLimit {
	if plan.ThrottleLimitCount == nil {
		return []portalPlanLimit{}
	}
	return []portalPlanLimit{{
		LimitType:  constants.LimitTypeRequestCount,
		LimitCount: *plan.ThrottleLimitCount,
		TimeUnit:   plan.ThrottleLimitUnit,
		TimeAmount: 1,
	}}
}

// sameLimits reports whether a portal copy's limits are want, ignoring order and the case of
// the codes.
func sameLimits(want, got []portalPlanLimit) bool {
	if len(want) != len(got) {
		return false
	}
	key := func(l portalPlanLimit) string {
		return fmt.Sprintf("%s|%d|%s|%d", strings.ToUpper(l.LimitType), l.LimitCount, strings.ToUpper(l.TimeUnit), l.TimeAmount)
	}
	keys := make(map[string]int, len(want))
	for _, l := range want {
		keys[key(l)]++
	}
	for _, l := range got {
		k := key(l)
		if keys[k] == 0 {
			return false
		}
		keys[k]--
	}
	return true
}

// samePricing reports whether a portal copy's pricing is the plan's own. Both are reduced to the
// same form: codes and property names upper-cased, amounts and a comparison's number compared as
// numbers, filter values and prices in a fixed order, and an absent list equal to an empty one.
func samePricing(plan *model.SubscriptionPlanPricing, portalCopy *planPricingRequest) bool {
	if plan == nil || portalCopy == nil {
		return plan == nil && portalCopy == nil
	}
	return pricingKey(portalPricing(plan)) == pricingKey(portalCopy)
}

func pricingKey(p *planPricingRequest) string {
	prices := make([]string, 0, len(p.Prices))
	for _, price := range p.Prices {
		filters := make([]string, 0, len(price.Filters))
		for _, f := range price.Filters {
			operator := strings.ToUpper(strings.TrimSpace(f.Operator))
			values := make([]string, 0, len(f.Values))
			for _, v := range f.Values {
				values = append(values, filterValueKey(operator, v))
			}
			slices.Sort(values)
			filters = append(filters, fmt.Sprintf("%s|%s|%q",
				strings.ToUpper(strings.TrimSpace(f.Field)), operator, slices.Compact(values)))
		}
		slices.Sort(filters)
		tiers := make([]string, 0, len(price.Tiers))
		for _, t := range price.Tiers {
			tiers = append(tiers, fmt.Sprintf("%s|%s|%s", optionalInt(t.UpTo), amountKey(t.UnitAmount), amountKey(t.FlatAmount)))
		}
		pricingModel := strings.ToUpper(price.PricingModel)
		quantity := price.UnitQuantity
		if quantity == nil && pricingModel == model.PricingModelPerUnit {
			// The portal stores a per-unit price without a quantity as per 1 unit.
			one := int64(1)
			quantity = &one
		}
		prices = append(prices, fmt.Sprintf("%s|%s|%s|%s|%q|%q", price.MeterID, pricingModel,
			amountKey(price.UnitAmount), optionalInt(quantity), filters, tiers))
	}
	slices.Sort(prices)
	return fmt.Sprintf("%s|%s|%s|%s|%q", strings.ToUpper(p.PricingType), strings.ToUpper(p.Currency),
		strings.ToUpper(p.BillingPeriod), amountKey(p.FlatAmount), prices)
}

// filterValueKey is a filter value as the API Portal stores it: trimmed, and a comparison's number
// compared as a number. The portal also drops repeated values.
func filterValueKey(operator, value string) string {
	value = strings.TrimSpace(value)
	switch operator {
	case model.FilterOperatorGreaterThan, model.FilterOperatorGreaterThanOrEqual, model.FilterOperatorLessThan,
		model.FilterOperatorLessThanOrEqual:
		return amountKey(&value)
	}
	return value
}

// amountKey is an amount in a form that does not depend on how it was written ("20", "20.00").
func amountKey(amount *string) string {
	if amount == nil {
		return ""
	}
	if r, ok := new(big.Rat).SetString(strings.TrimSpace(*amount)); ok {
		return r.RatString()
	}
	return *amount
}

func optionalInt(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}
