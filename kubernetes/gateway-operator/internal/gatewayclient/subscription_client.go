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
	"strings"
)

// SubscriptionCreatePayload mirrors the management-API
// SubscriptionCreateRequest body.
type SubscriptionCreatePayload struct {
	ExternalRefId         *string `json:"externalRefId,omitempty"`
	ApiId                 string  `json:"apiId"`
	SubscriptionToken     string  `json:"subscriptionToken"`
	ApplicationId         *string `json:"applicationId,omitempty"`
	SubscriptionPlanId    *string `json:"subscriptionPlanId,omitempty"`
	BillingCustomerId     *string `json:"billingCustomerId,omitempty"`
	BillingSubscriptionId *string `json:"billingSubscriptionId,omitempty"`
	Status                *string `json:"status,omitempty"`
}

// SubscriptionUpdatePayload mirrors SubscriptionUpdateRequest, a partial update
// where omitted fields are left unchanged on the gateway.
type SubscriptionUpdatePayload struct {
	Status *string `json:"status,omitempty"`
	// SubscriptionPlanId is always sent so the CR stays the source of truth:
	// nil marshals as an explicit null, which removes the plan on the gateway.
	SubscriptionPlanId *string `json:"subscriptionPlanId"`
	// SubscriptionToken rotates the token in place when set.
	SubscriptionToken *string `json:"subscriptionToken,omitempty"`
}

// SubscriptionResponse captures the gateway-issued fields returned from
// POST/PUT /subscriptions.
type SubscriptionResponse struct {
	Id            string  `json:"id"`
	ExternalRefId *string `json:"externalRefId,omitempty"`
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
	return putSubscription(ctx, gatewayEndpoint, subscriptionID, payload, auth)
}

// UpdateSubscriptionStatus PUTs only the status, leaving the plan and token unchanged on the gateway.
func UpdateSubscriptionStatus(ctx context.Context, gatewayEndpoint, subscriptionID string, status *string, auth AuthHeaderFunc) (*SubscriptionResponse, error) {
	payload := struct {
		Status *string `json:"status,omitempty"`
	}{Status: status}
	return putSubscription(ctx, gatewayEndpoint, subscriptionID, payload, auth)
}

func putSubscription(ctx context.Context, gatewayEndpoint, subscriptionID string, payload any, auth AuthHeaderFunc) (*SubscriptionResponse, error) {
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

// ListSubscriptions GETs /subscriptions?apiId=... (and applicationId when set). The gateway resolves
// apiId from either the API handle or its internal id and returns only that API's subscriptions.
func ListSubscriptions(ctx context.Context, gatewayEndpoint, apiID string, applicationID *string, auth AuthHeaderFunc) ([]SubscriptionResponse, error) {
	if apiID == "" {
		return nil, fmt.Errorf("api id is required")
	}

	q := url.Values{}
	q.Set("apiId", apiID)
	if applicationID != nil && *applicationID != "" {
		q.Set("applicationId", *applicationID)
	}

	endpoint := gatewayEndpoint + subscriptionsPath + "?" + q.Encode()
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
		return nil, &RetryableError{Err: fmt.Errorf("list subscriptions: %w", err)}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	switch {
	case resp.StatusCode == http.StatusOK:
		// continue
	case IsRetryableStatusCode(resp.StatusCode):
		return nil, &RetryableError{
			Err:        fmt.Errorf("list subscriptions returned status %d: %s", resp.StatusCode, string(body)),
			StatusCode: resp.StatusCode,
		}
	default:
		return nil, &NonRetryableError{
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
			return nil, fmt.Errorf("decode subscription list response: %w", err)
		}
		items = plain
	}

	return items, nil
}

// FindSubscriptionIDByAPIAndApplication lists subscriptions for the given apiID
// (and optional applicationID) and returns the id of the one subscription whose
// application matches applicationID. It returns "" when no subscription or more
// than one matches, so a caller never adopts another application's subscription.
func FindSubscriptionIDByAPIAndApplication(ctx context.Context, gatewayEndpoint, apiID string, applicationID *string, auth AuthHeaderFunc) (string, error) {
	items, err := ListSubscriptions(ctx, gatewayEndpoint, apiID, applicationID, auth)
	if err != nil {
		return "", err
	}

	matchID := ""
	matches := 0
	for i := range items {
		if SameApplication(items[i].ApplicationId, applicationID) {
			matches++
			matchID = items[i].Id
		}
	}
	if matches != 1 {
		return "", nil
	}
	return matchID, nil
}

// SameApplication reports whether two subscription application ids refer to the
// same application. An unset, empty or whitespace-only id means no application.
func SameApplication(a, b *string) bool {
	return normalizedApplicationID(a) == normalizedApplicationID(b)
}

func normalizedApplicationID(id *string) string {
	if id == nil {
		return ""
	}
	return strings.TrimSpace(*id)
}
