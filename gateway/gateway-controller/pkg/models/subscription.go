/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package models

import "time"

// SubscriptionStatus represents the status of an application-level subscription
type SubscriptionStatus string

const (
	SubscriptionStatusActive   SubscriptionStatus = "ACTIVE"
	SubscriptionStatusInactive SubscriptionStatus = "INACTIVE"
	SubscriptionStatusRevoked  SubscriptionStatus = "REVOKED"
)

// Subscription represents a subscription to an API (deployment)
type Subscription struct {
	ID                    string             `json:"id"`
	APIID                 string             `json:"apiId" db:"api_id"`
	ApplicationID         *string            `json:"applicationId,omitempty" db:"application_id"`
	SubscriptionToken     string             `json:"subscriptionToken"`                              // Transient; only set when creating from request, never from DB (gateway stores only hash)
	SubscriptionTokenHash string             `json:"-" db:"subscription_token_hash"`                 // For xDS validation; not exposed in API
	SubscriptionPlanID    *string            `json:"subscriptionPlanId,omitempty" db:"subscription_plan_id"`
	BillingCustomerID     *string            `json:"billingCustomerId,omitempty" db:"billing_customer_id"`
	BillingSubscriptionID *string            `json:"billingSubscriptionId,omitempty" db:"billing_subscription_id"`
	GatewayID             string             `json:"gatewayId" db:"gateway_id"`
	Status                SubscriptionStatus `json:"status" db:"status"`
	CreatedAt             time.Time          `json:"createdAt" db:"created_at"`
	UpdatedAt             time.Time          `json:"updatedAt" db:"updated_at"`
	Etag                  string             `json:"etag,omitempty"` // Transient; from CP sync response, not stored in DB
}
