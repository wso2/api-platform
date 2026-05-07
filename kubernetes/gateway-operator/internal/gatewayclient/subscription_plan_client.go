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

package gatewayclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SubscriptionPlanCreatePayload mirrors the management-API
// SubscriptionPlanCreateRequest. Pointer fields are omitted when nil so the
// gateway-controller fills in defaults.
type SubscriptionPlanCreatePayload struct {
	PlanName           string     `json:"planName"`
	BillingPlan        *string    `json:"billingPlan,omitempty"`
	ExpiryTime         *time.Time `json:"expiryTime,omitempty"`
	Status             *string    `json:"status,omitempty"`
	StopOnQuotaReach   *bool      `json:"stopOnQuotaReach,omitempty"`
	ThrottleLimitCount *int64     `json:"throttleLimitCount,omitempty"`
	ThrottleLimitUnit  *string    `json:"throttleLimitUnit,omitempty"`
}

// SubscriptionPlanUpdatePayload mirrors SubscriptionPlanUpdateRequest.
type SubscriptionPlanUpdatePayload struct {
	PlanName           *string    `json:"planName,omitempty"`
	BillingPlan        *string    `json:"billingPlan,omitempty"`
	ExpiryTime         *time.Time `json:"expiryTime,omitempty"`
	Status             *string    `json:"status,omitempty"`
	StopOnQuotaReach   *bool      `json:"stopOnQuotaReach,omitempty"`
	ThrottleLimitCount *int64     `json:"throttleLimitCount,omitempty"`
	ThrottleLimitUnit  *string    `json:"throttleLimitUnit,omitempty"`
}

// SubscriptionPlanResponse captures the gateway-issued fields returned from
// POST/PUT /subscription-plans.
type SubscriptionPlanResponse struct {
	Id        string `json:"id"`
	PlanName  string `json:"planName"`
	GatewayId string `json:"gatewayId"`
}

func decodeSubscriptionPlanResponseBody(respBody []byte) (SubscriptionPlanResponse, error) {
	var aux struct {
		Id        string `json:"id"`
		PlanId    string `json:"planId"`
		PlanName  string `json:"planName"`
		GatewayId string `json:"gatewayId"`
	}
	if err := json.Unmarshal(respBody, &aux); err != nil {
		return SubscriptionPlanResponse{}, err
	}
	id := strings.TrimSpace(aux.Id)
	if id == "" {
		id = strings.TrimSpace(aux.PlanId)
	}
	return SubscriptionPlanResponse{
		Id:        id,
		PlanName:  aux.PlanName,
		GatewayId: aux.GatewayId,
	}, nil
}

// ListSubscriptionPlans GETs /subscription-plans.
func ListSubscriptionPlans(ctx context.Context, gatewayEndpoint string, auth AuthHeaderFunc) ([]SubscriptionPlanResponse, error) {
	endpoint := gatewayEndpoint + subscriptionPlansPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &RetryableError{Err: fmt.Errorf("create HTTP request: %w", err)}
	}
	if auth != nil {
		if err := auth(ctx, req); err != nil {
			return nil, err
		}
	}
	client := &http.Client{Timeout: defaultProbeTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &RetryableError{Err: fmt.Errorf("list subscription plans: %w", err)}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK {
		var wrap struct {
			SubscriptionPlans []json.RawMessage `json:"subscriptionPlans"`
		}
		if err := json.Unmarshal(body, &wrap); err != nil {
			return nil, &NonRetryableError{Err: fmt.Errorf("decode subscription plan list: %w", err), StatusCode: resp.StatusCode}
		}
		out := make([]SubscriptionPlanResponse, 0, len(wrap.SubscriptionPlans))
		for _, raw := range wrap.SubscriptionPlans {
			p, err := decodeSubscriptionPlanResponseBody(raw)
			if err != nil {
				return nil, &NonRetryableError{Err: fmt.Errorf("decode subscription plan item: %w", err), StatusCode: resp.StatusCode}
			}
			out = append(out, p)
		}
		return out, nil
	}
	errMsg := fmt.Errorf("list subscription plans returned status %d: %s", resp.StatusCode, string(body))
	if IsRetryableStatusCode(resp.StatusCode) {
		return nil, &RetryableError{Err: errMsg, StatusCode: resp.StatusCode}
	}
	return nil, &NonRetryableError{Err: errMsg, StatusCode: resp.StatusCode}
}

// FindSubscriptionPlanIDByPlanName returns the gateway plan UUID for an exact
// spec.planName match, or ("", nil) if none is found.
func FindSubscriptionPlanIDByPlanName(ctx context.Context, gatewayEndpoint, planName string, auth AuthHeaderFunc) (string, error) {
	want := strings.TrimSpace(planName)
	if want == "" {
		return "", nil
	}
	list, err := ListSubscriptionPlans(ctx, gatewayEndpoint, auth)
	if err != nil {
		return "", err
	}
	for _, p := range list {
		if strings.TrimSpace(p.PlanName) == want && p.Id != "" {
			return p.Id, nil
		}
	}
	return "", nil
}

// CreateSubscriptionPlan POSTs a SubscriptionPlanCreateRequest payload and
// returns the parsed response (in particular the gateway-issued id).
func CreateSubscriptionPlan(ctx context.Context, gatewayEndpoint string, payload SubscriptionPlanCreatePayload, auth AuthHeaderFunc) (*SubscriptionPlanResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal subscription plan: %w", err)
	}
	respBody, err := CreateResource(ctx, gatewayEndpoint, subscriptionPlansPath, body, PayloadContentTypeJSON, auth)
	if err != nil {
		return nil, err
	}
	out, err := decodeSubscriptionPlanResponseBody(respBody)
	if err != nil {
		return nil, fmt.Errorf("decode subscription plan response: %w", err)
	}
	return &out, nil
}

// UpdateSubscriptionPlan PUTs a SubscriptionPlanUpdateRequest payload to
// /subscription-plans/{planId}.
func UpdateSubscriptionPlan(ctx context.Context, gatewayEndpoint, planID string, payload SubscriptionPlanUpdatePayload, auth AuthHeaderFunc) (*SubscriptionPlanResponse, error) {
	if planID == "" {
		return nil, fmt.Errorf("subscription plan id is required")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal subscription plan update: %w", err)
	}
	respBody, err := UpdateResource(ctx, gatewayEndpoint, subscriptionPlansPath, planID, body, PayloadContentTypeJSON, auth)
	if err != nil {
		return nil, err
	}
	out, err := decodeSubscriptionPlanResponseBody(respBody)
	if err != nil {
		return nil, fmt.Errorf("decode subscription plan response: %w", err)
	}
	return &out, nil
}

// DeleteSubscriptionPlan DELETEs /subscription-plans/{planId}.
func DeleteSubscriptionPlan(ctx context.Context, gatewayEndpoint, planID string, auth AuthHeaderFunc) error {
	if planID == "" {
		return fmt.Errorf("subscription plan id is required")
	}
	return DeleteResource(ctx, gatewayEndpoint, subscriptionPlansPath, planID, auth)
}
