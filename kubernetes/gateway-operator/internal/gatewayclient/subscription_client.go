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
	"net/url"
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
	Id            string  `json:"id"`
	ApiId         string  `json:"apiId"`
	ApplicationId *string `json:"applicationId,omitempty"`
	GatewayId     string  `json:"gatewayId"`
}

type subscriptionListResponse struct {
	Subscriptions []SubscriptionResponse `json:"subscriptions"`
	Count         *int                   `json:"count,omitempty"`
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

// FindSubscriptionIDByAPIAndApplication lists subscriptions for the given apiID
// (and optional applicationID) and returns a single matching subscription ID
// when it can be unambiguously identified.
func FindSubscriptionIDByAPIAndApplication(ctx context.Context, gatewayEndpoint, apiID string, applicationID *string, auth AuthHeaderFunc) (string, error) {
	if apiID == "" {
		return "", fmt.Errorf("api id is required")
	}

	q := url.Values{}
	q.Set("apiId", apiID)
	if applicationID != nil && *applicationID != "" {
		q.Set("applicationId", *applicationID)
	}

	endpoint := gatewayEndpoint + subscriptionsPath + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", &RetryableError{Err: fmt.Errorf("create HTTP request: %w", err)}
	}
	if auth != nil {
		if err := auth(ctx, req); err != nil {
			return "", err
		}
	}

	client := &http.Client{Timeout: defaultProbeTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", &RetryableError{Err: fmt.Errorf("list subscriptions: %w", err)}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	switch {
	case resp.StatusCode == http.StatusOK:
		// continue
	case IsRetryableStatusCode(resp.StatusCode):
		return "", &RetryableError{
			Err:        fmt.Errorf("list subscriptions returned status %d: %s", resp.StatusCode, string(body)),
			StatusCode: resp.StatusCode,
		}
	default:
		return "", &NonRetryableError{
			Err:        fmt.Errorf("list subscriptions returned status %d: %s", resp.StatusCode, string(body)),
			StatusCode: resp.StatusCode,
		}
	}

	items := make([]SubscriptionResponse, 0)
	var wrapped subscriptionListResponse
	if err := json.Unmarshal(body, &wrapped); err == nil && wrapped.Subscriptions != nil {
		items = wrapped.Subscriptions
	} else {
		var plain []SubscriptionResponse
		if err := json.Unmarshal(body, &plain); err != nil {
			return "", fmt.Errorf("decode subscription list response: %w", err)
		}
		items = plain
	}

	if len(items) == 0 {
		return "", nil
	}
	if len(items) == 1 {
		return items[0].Id, nil
	}

	if applicationID != nil && *applicationID != "" {
		matches := make([]SubscriptionResponse, 0, len(items))
		for i := range items {
			if items[i].ApplicationId != nil && *items[i].ApplicationId == *applicationID {
				matches = append(matches, items[i])
			}
		}
		if len(matches) == 1 {
			return matches[0].Id, nil
		}
		return "", nil
	}

	nonAppMatches := make([]SubscriptionResponse, 0, len(items))
	for i := range items {
		if items[i].ApplicationId == nil || *items[i].ApplicationId == "" {
			nonAppMatches = append(nonAppMatches, items[i])
		}
	}
	if len(nonAppMatches) == 1 {
		return nonAppMatches[0].Id, nil
	}

	return "", nil
}
