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
	"net/http"
	"slices"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

const (
	portalPlansPageSize = 20
	// portalPlansMaxPages bounds the listing loop; running into it is an error,
	// not a short list, so plans that exist are never mistaken for missing ones.
	portalPlansMaxPages = 250
	// portalPlansMaxResponseBytes bounds one page of the plan listing.
	portalPlansMaxResponseBytes = 1 << 20 // 1 MiB
)

// portalPlanList is the part of the portal's plan listing this package reads.
type portalPlanList struct {
	List []struct {
		ID string `json:"id"`
	} `json:"list"`
	Pagination struct {
		Total int `json:"total"`
	} `json:"pagination"`
}

// portalPlan is one entry of the portal's bulk plan create.
type portalPlan struct {
	ID          string            `json:"id"`
	RefID       string            `json:"refId"`
	DisplayName string            `json:"displayName"`
	Limits      []portalPlanLimit `json:"limits,omitempty"`
}

type portalPlanLimit struct {
	LimitType  string `json:"limitType"`
	LimitCount int    `json:"limitCount"`
	TimeUnit   string `json:"timeUnit,omitempty"`
	TimeAmount int    `json:"timeAmount"`
}

// CreateMissingPlans implements PortalPublisher: it creates, in one bulk
// request, the plans whose handle the portal does not list.
func (p *HTTPPortalPublisher) CreateMissingPlans(ctx context.Context, portal *model.APIPortal, plans []*model.SubscriptionPlan) error {
	if len(plans) == 0 {
		return nil
	}
	missing, err := p.findMissingPlans(ctx, portal, plans)
	if err != nil {
		return err
	}
	if len(missing) == 0 {
		return nil
	}
	return p.createPlans(ctx, portal, missing)
}

// findMissingPlans pages through the portal's plans and returns those of plans
// it does not list, in input order. It stops once every plan has been seen or
// the listing ends.
func (p *HTTPPortalPublisher) findMissingPlans(ctx context.Context, portal *model.APIPortal, plans []*model.SubscriptionPlan) ([]*model.SubscriptionPlan, error) {
	missing := slices.Clone(plans)
	for page := 0; page < portalPlansMaxPages; page++ {
		offset := page * portalPlansPageSize
		list, err := p.listPlansPage(ctx, portal, offset)
		if err != nil {
			return nil, err
		}
		listed := make(map[string]struct{}, len(list.List))
		for _, item := range list.List {
			listed[item.ID] = struct{}{}
		}
		missing = slices.DeleteFunc(missing, func(plan *model.SubscriptionPlan) bool {
			_, ok := listed[plan.Handle]
			return ok
		})
		if len(missing) == 0 || len(list.List) == 0 || offset+len(list.List) >= list.Pagination.Total {
			return missing, nil
		}
	}
	return nil, fmt.Errorf("portal plan listing exceeds %d pages", portalPlansMaxPages)
}

func (p *HTTPPortalPublisher) listPlansPage(ctx context.Context, portal *model.APIPortal, offset int) (*portalPlanList, error) {
	authHeader, err := p.authHeader(ctx, portal)
	if err != nil {
		return nil, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, p.client.TotalTimeout())
	defer cancel()

	target := fmt.Sprintf("%s?limit=%d&offset=%d", plansURL(portal), portalPlansPageSize, offset)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, target, nil)
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
func (p *HTTPPortalPublisher) createPlans(ctx context.Context, portal *model.APIPortal, plans []*model.SubscriptionPlan) error {
	payload := make([]portalPlan, 0, len(plans))
	for _, plan := range plans {
		payload = append(payload, toPortalPlan(plan))
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to encode portal subscription plans: %w", err)
	}

	authHeader, err := p.authHeader(ctx, portal)
	if err != nil {
		return err
	}

	reqCtx, cancel := context.WithTimeout(ctx, p.client.TotalTimeout())
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, plansURL(portal), bytes.NewReader(body))
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

// toPortalPlan maps a plan to the portal's shape. A plan without a throttle
// limit is sent with no limits, which the portal treats as unlimited.
func toPortalPlan(plan *model.SubscriptionPlan) portalPlan {
	out := portalPlan{ID: plan.Handle, RefID: plan.UUID, DisplayName: plan.Name}
	if plan.ThrottleLimitCount != nil {
		out.Limits = []portalPlanLimit{{
			LimitType:  constants.LimitTypeRequestCount,
			LimitCount: *plan.ThrottleLimitCount,
			TimeUnit:   plan.ThrottleLimitUnit,
			TimeAmount: 1,
		}}
	}
	return out
}
