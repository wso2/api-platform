/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
 *
 */

package model

import "time"

// WebSubAPI represents a WebSub API entity in the platform
type WebSubAPI struct {
	UUID             string                 `json:"uuid" db:"-"`
	Handle           string                 `json:"id" db:"-"`
	OrganizationUUID string                 `json:"organizationId" db:"-"`
	ProjectUUID      string                 `json:"projectId" db:"-"`
	Name             string                 `json:"name" db:"-"`
	Description      string                 `json:"description,omitempty" db:"-"`
	CreatedBy        string                 `json:"createdBy,omitempty" db:"created_by"`
	UpdatedBy        string                 `json:"updatedBy,omitempty" db:"updated_by"`
	Version          string                 `json:"version" db:"-"`
	LifeCycleStatus  string                 `json:"lifeCycleStatus" db:"-"`
	CreatedAt        time.Time              `json:"createdAt" db:"-"`
	UpdatedAt        time.Time              `json:"updatedAt" db:"-"`
	Configuration    WebSubAPIConfiguration `json:"configuration" db:"-"`
}

// WebSubAPIConfiguration holds the WebSub API configuration stored as JSON in the DB
type WebSubAPIConfiguration struct {
	Name              string                    `json:"name,omitempty"`
	Version           string                    `json:"version,omitempty"`
	Context           *string                   `json:"context,omitempty"`
	Transport         []string                  `json:"transport,omitempty"`
	Channels          map[string]WebSubChannel  `json:"channels,omitempty"`
	Upstream          UpstreamConfig            `json:"upstream,omitempty"`
	AllChannels       *WebSubAllChannelPolicies `json:"allChannels,omitempty"`
	SubscriptionPlans []string                  `json:"subscriptionPlans,omitempty"`
}

// WebSubEventPolicies holds policies for a single event type.
type WebSubEventPolicies struct {
	Policies []Policy `json:"policies,omitempty"`
}

// WebSubAllChannelPolicies holds policies applied to all channels, organized by event type.
type WebSubAllChannelPolicies struct {
	OnSubscription    *WebSubEventPolicies `json:"on_subscription,omitempty"`
	OnUnsubscription  *WebSubEventPolicies `json:"on_unsubscription,omitempty"`
	OnMessageReceived *WebSubEventPolicies `json:"on_message_received,omitempty"`
	OnMessageDelivery *WebSubEventPolicies `json:"on_message_delivery,omitempty"`
}

// WebSubChannelPolicies holds policies applied to a specific channel, organized by event type.
type WebSubChannelPolicies = WebSubAllChannelPolicies

// WebSubChannel represents a single channel with optional per-channel policy overrides.
type WebSubChannel struct {
	OnSubscription    *WebSubEventPolicies `json:"on_subscription,omitempty"`
	OnUnsubscription  *WebSubEventPolicies `json:"on_unsubscription,omitempty"`
	OnMessageReceived *WebSubEventPolicies `json:"on_message_received,omitempty"`
	OnMessageDelivery *WebSubEventPolicies `json:"on_message_delivery,omitempty"`
}

// WebSubReceiver represents the receiver section of a WebSub API configuration.
type WebSubReceiver struct {
	Policies []Policy `json:"policies,omitempty"`
}

// WebSubHub represents the hub section of a WebSub API configuration.
type WebSubHub struct {
	Policies []Policy           `json:"policies,omitempty"`
	Channels []WebSubHubChannel `json:"channels,omitempty"`
}

// WebSubHubChannel represents a channel entry under the hub section.
type WebSubHubChannel struct {
	Name     string   `json:"name"`
	Policies []Policy `json:"policies,omitempty"`
}

// WebSubDelivery represents the delivery section of a WebSub API configuration.
type WebSubDelivery struct {
	Policies []Policy `json:"policies,omitempty"`
}
