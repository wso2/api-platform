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

package models

import "time"

// SubscriptionPlanStatus represents the status of a subscription plan
type SubscriptionPlanStatus string

const (
	SubscriptionPlanStatusActive   SubscriptionPlanStatus = "ACTIVE"
	SubscriptionPlanStatusInactive SubscriptionPlanStatus = "INACTIVE"
)

// SubscriptionPlan represents an organization-scoped subscription plan
type SubscriptionPlan struct {
	ID                 string                 `json:"id"`
	GatewayID          string                 `json:"gatewayId" db:"gateway_id"`
	PlanName           string                 `json:"planName" db:"plan_name"`
	BillingPlan        *string                `json:"billingPlan,omitempty" db:"billing_plan"`
	StopOnQuotaReach   bool                   `json:"stopOnQuotaReach" db:"stop_on_quota_reach"`
	ThrottleLimitCount *int                   `json:"throttleLimitCount,omitempty" db:"throttle_limit_count"`
	ThrottleLimitUnit  *string                `json:"throttleLimitUnit,omitempty" db:"throttle_limit_unit"`
	ExpiryTime         *time.Time             `json:"expiryTime,omitempty" db:"expiry_time"`
	Status             SubscriptionPlanStatus `json:"status" db:"status"`
	CreatedAt          time.Time              `json:"createdAt" db:"created_at"`
	UpdatedAt          time.Time              `json:"updatedAt" db:"updated_at"`
}
