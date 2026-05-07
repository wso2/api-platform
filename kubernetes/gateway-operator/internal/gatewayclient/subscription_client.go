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
)

// SubscriptionCreatePayload mirrors the management-API
// SubscriptionCreateRequest body.
type SubscriptionCreatePayload struct {
	ApiId                 string  `json:"apiId"`
	SubscriptionToken     string  `json:"subscriptionToken"`
	ApplicationId         *string `json:"applicationId,omitempty"`
	SubscriptionPlanId    *string `json:"subscriptionPlanId,omitempty"`
	BillingCustomerId     *string `json:"billingCustomerId,omitempty"`
	BillingSubscriptionId *string `json:"billingSubscriptionId,omitempty"`
	Status                *string `json:"status,omitempty"`
}

// SubscriptionUpdatePayload mirrors SubscriptionUpdateRequest.
type SubscriptionUpdatePayload struct {
	Status *string `json:"status,omitempty"`
}

// SubscriptionResponse captures the gateway-issued fields returned from
// POST/PUT /subscriptions.
type SubscriptionResponse struct {
	Id        string `json:"id"`
	ApiId     string `json:"apiId"`
	GatewayId string `json:"gatewayId"`
}

// CreateSubscription POSTs the payload and returns the parsed response.
func CreateSubscription(ctx context.Context, gatewayEndpoint string, payload SubscriptionCreatePayload, auth AuthHeaderFunc) (*SubscriptionResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal subscription: %w", err)
	}
	respBody, err := CreateResource(ctx, gatewayEndpoint, subscriptionsPath, body, PayloadContentTypeJSON, auth)
	if err != nil {
		return nil, err
	}
	var out SubscriptionResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("decode subscription response: %w", err)
	}
	return &out, nil
}

// UpdateSubscription PUTs a SubscriptionUpdateRequest payload to
// /subscriptions/{subscriptionId}.
func UpdateSubscription(ctx context.Context, gatewayEndpoint, subscriptionID string, payload SubscriptionUpdatePayload, auth AuthHeaderFunc) (*SubscriptionResponse, error) {
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription id is required")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal subscription update: %w", err)
	}
	respBody, err := UpdateResource(ctx, gatewayEndpoint, subscriptionsPath, subscriptionID, body, PayloadContentTypeJSON, auth)
	if err != nil {
		return nil, err
	}
	var out SubscriptionResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("decode subscription response: %w", err)
	}
	return &out, nil
}

// DeleteSubscription DELETEs /subscriptions/{subscriptionId}.
func DeleteSubscription(ctx context.Context, gatewayEndpoint, subscriptionID string, auth AuthHeaderFunc) error {
	if subscriptionID == "" {
		return fmt.Errorf("subscription id is required")
	}
	return DeleteResource(ctx, gatewayEndpoint, subscriptionsPath, subscriptionID, auth)
}
