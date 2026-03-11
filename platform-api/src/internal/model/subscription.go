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

package model

import "time"

// SubscriptionStatus represents the lifecycle state of a subscription
type SubscriptionStatus string

const (
	SubscriptionStatusActive   SubscriptionStatus = "ACTIVE"
	SubscriptionStatusInactive SubscriptionStatus = "INACTIVE"
	SubscriptionStatusRevoked  SubscriptionStatus = "REVOKED"
)

// Subscription represents a subscription to a REST API
type Subscription struct {
	UUID               string             `json:"id" db:"uuid"`
	APIUUID            string             `json:"apiId" db:"api_uuid"`
	ApplicationID      *string            `json:"applicationId,omitempty" db:"application_id"`
	SubscriptionToken  string             `json:"subscriptionToken" db:"subscription_token"` // Decrypted for API response
	SubscriptionPlanID *string            `json:"subscriptionPlanId,omitempty" db:"subscription_plan_uuid"`
	OrganizationUUID   string             `json:"organizationId" db:"organization_uuid"`
	Status             SubscriptionStatus `json:"status" db:"status"`
	CreatedAt          time.Time          `json:"createdAt" db:"created_at"`
	UpdatedAt          time.Time          `json:"updatedAt" db:"updated_at"`
}
