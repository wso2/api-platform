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

package model

import "time"

// SubscriptionPlanStatus represents the lifecycle state of a subscription plan
type SubscriptionPlanStatus string

const (
	SubscriptionPlanStatusActive   SubscriptionPlanStatus = "ACTIVE"
	SubscriptionPlanStatusInactive SubscriptionPlanStatus = "INACTIVE"
)

// SubscriptionPlanUpdate holds fields for partial updates.
// All pointer fields are only applied when non-nil (patch semantics).
type SubscriptionPlanUpdate struct {
	PlanName           *string
	BillingPlan        *string
	StopOnQuotaReach   *bool
	ThrottleLimitCount *int
	ThrottleLimitUnit  *string
	ExpiryTime         *time.Time
	Status             *SubscriptionPlanStatus
}

// SubscriptionPlan represents an organization-scoped subscription plan
type SubscriptionPlan struct {
	UUID               string                 `json:"id" db:"uuid"`
	PlanName           string                 `json:"planName" db:"plan_name"`
	BillingPlan        string                 `json:"billingPlan,omitempty" db:"billing_plan"`
	StopOnQuotaReach   bool                   `json:"stopOnQuotaReach" db:"stop_on_quota_reach"`
	ThrottleLimitCount *int                   `json:"throttleLimitCount,omitempty" db:"throttle_limit_count"`
	ThrottleLimitUnit  string                 `json:"throttleLimitUnit,omitempty" db:"throttle_limit_unit"`
	ExpiryTime         *time.Time             `json:"expiryTime,omitempty" db:"expiry_time"`
	OrganizationUUID   string                 `json:"organizationId" db:"organization_uuid"`
	Status             SubscriptionPlanStatus `json:"status" db:"status"`
	CreatedAt          time.Time              `json:"createdAt" db:"created_at"`
	UpdatedAt          time.Time              `json:"updatedAt" db:"updated_at"`
}
