/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package dto

// SubscriptionPolicyCreateRequest represents the request body for creating a subscription policy
//
// This DTO is used when creating default "unlimited" subscription policies for new organizations
// in the developer portal. Per spec, the unlimited policy has 1000000 requests per minute.
type SubscriptionPolicyCreateRequest struct {
	PolicyName   string `json:"policyName"`   // Policy identifier (e.g., "unlimited")
	DisplayName  string `json:"displayName"`  // Human-readable name (e.g., "Unlimited Tier")
	BillingPlan  string `json:"billingPlan"`  // Billing plan type (e.g., "FREE")
	Description  string `json:"description"`  // Policy description
	Type         string `json:"type"`         // Policy type (e.g., "requestCount")
	TimeUnit     int    `json:"timeUnit"`     // Time unit value (e.g., 60 for 1 minute)
	UnitTime     string `json:"unitTime"`     // Time unit string (e.g., "min")
	RequestCount int    `json:"requestCount"` // Maximum requests allowed (e.g., 1000000)
}

// SubscriptionPolicyCreateResponse represents the response from developer portal after policy creation
//
// This DTO contains the confirmed subscription policy details from the developer portal.
type SubscriptionPolicyCreateResponse struct {
	ID          string `json:"id"`          // Created policy UUID
	PolicyName  string `json:"policyName"`  // Policy identifier
	DisplayName string `json:"displayName"` // Display name
	CreatedAt   string `json:"createdAt"`   // Timestamp of creation (ISO 8601)
}
