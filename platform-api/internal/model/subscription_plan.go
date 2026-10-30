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

// MaxSubscriptionPlanHandleChars is the width of subscription_plans.handle. The API Portal keeps
// the handles of the plans it creates to the same width, so every plan made there can be copied
// here.
const MaxSubscriptionPlanHandleChars = 40

// SubscriptionPlanUpdate holds fields for partial updates.
// All pointer fields are only applied when non-nil (patch semantics).
type SubscriptionPlanUpdate struct {
	Handle             *string
	Name               *string
	StopOnQuotaReach   *bool
	ThrottleLimitCount *int
	ThrottleLimitUnit  *string
	// ClearLimit removes the plan's throttling limit entirely. Set when the caller
	// submits an explicit empty "limits" array. Ignored if ThrottleLimitCount is set.
	ClearLimit bool
	ExpiryTime *time.Time
	// ClearExpiryTime removes the plan's expiry. Set when the caller submits an
	// explicit null "expiryTime"; an omitted field leaves the expiry unchanged.
	// Ignored if ExpiryTime is set.
	ClearExpiryTime bool
	Status          *SubscriptionPlanStatus
}

// SubscriptionPlan represents an organization-scoped subscription plan
//
// NOTE: throttling limits are stored in the subscription_plan_limits table, which
// is modelled to hold MULTIPLE limits per plan. For now the platform-api only
// surfaces a SINGLE REQUEST_COUNT limit, mapped onto the StopOnQuotaReach /
// ThrottleLimitCount / ThrottleLimitUnit fields below (these no longer have
// columns on subscription_plans). This must be improved to expose all limits.
type SubscriptionPlan struct {
	UUID               string                 `json:"id" db:"uuid"`
	Handle             string                 `json:"handle" db:"handle"`
	Name               string                 `json:"name" db:"name"`
	StopOnQuotaReach   bool                   `json:"stopOnQuotaReach"`
	ThrottleLimitCount *int                   `json:"throttleLimitCount,omitempty"`
	ThrottleLimitUnit  string                 `json:"throttleLimitUnit,omitempty"`
	ExpiryTime         *time.Time             `json:"expiryTime,omitempty" db:"expiry_time"`
	OrganizationUUID   string                 `json:"organizationId" db:"organization_uuid"`
	Status             SubscriptionPlanStatus `json:"status" db:"status"`
	CreatedBy          string                 `json:"createdBy,omitempty" db:"created_by"`
	UpdatedBy          string                 `json:"updatedBy,omitempty" db:"updated_by"`
	CreatedAt          time.Time              `json:"createdAt" db:"created_at"`
	UpdatedAt          time.Time              `json:"updatedAt" db:"updated_at"`
}

// SubscriptionPlanUsage counts what offers or uses a subscription plan here:
//   - RESTAPIs: the REST APIs whose configuration lists the plan;
//   - Drafts: the APIs whose publication draft for an API Portal offers it;
//   - Publications: the APIs whose live publication on an API Portal offers it;
//   - Subscriptions: the subscriptions on the plan.
//
// Each API is counted once per kind, however many portals it is on. Only counts are kept, not
// names, since a plan may be offered by more APIs than a message can list.
type SubscriptionPlanUsage struct {
	RESTAPIs      int
	Drafts        int
	Publications  int
	Subscriptions int
}

// InUse reports whether anything offers or uses the plan.
func (u *SubscriptionPlanUsage) InUse() bool {
	return u.RESTAPIs > 0 || u.Drafts > 0 || u.Publications > 0 || u.Subscriptions > 0
}
