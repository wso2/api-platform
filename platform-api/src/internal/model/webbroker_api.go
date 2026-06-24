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

// WebBrokerAPI represents a WebBroker API entity in the platform
type WebBrokerAPI struct {
	UUID             string                    `json:"uuid" db:"-"`
	Handle           string                    `json:"id" db:"-"`
	OrganizationUUID string                    `json:"organizationId" db:"-"`
	ProjectUUID      string                    `json:"projectId" db:"-"`
	Name             string                    `json:"name" db:"-"`
	Description      string                    `json:"description,omitempty" db:"-"`
	CreatedBy        string                    `json:"createdBy,omitempty" db:"created_by"`
	UpdatedBy        string                    `json:"updatedBy,omitempty" db:"updated_by"`
	Version          string                    `json:"version" db:"-"`
	LifeCycleStatus  string                    `json:"lifeCycleStatus" db:"-"`
	CreatedAt        time.Time                 `json:"createdAt" db:"-"`
	UpdatedAt        time.Time                 `json:"updatedAt" db:"-"`
	Configuration    WebBrokerAPIConfiguration `json:"configuration" db:"-"`
}

// WebBrokerAPIConfiguration holds the WebBroker API configuration stored as JSON in the DB
type WebBrokerAPIConfiguration struct {
	Name              string                       `json:"name,omitempty"`
	Version           string                       `json:"version,omitempty"`
	Context           *string                      `json:"context,omitempty"`
	Transport         []string                     `json:"transport,omitempty"`
	Receiver          WebBrokerReceiver            `json:"receiver,omitempty"`
	Broker            WebBrokerBroker              `json:"broker,omitempty"`
	Channels          map[string]WebBrokerChannel  `json:"channels,omitempty"`
	AllChannels       *WebBrokerAllChannelPolicies `json:"allChannels,omitempty"`
	SubscriptionPlans []string                     `json:"subscriptionPlans,omitempty"`
}

// WebBrokerReceiver represents the receiver configuration
type WebBrokerReceiver struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"` // e.g., "websocket"
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// WebBrokerBroker represents the broker configuration
type WebBrokerBroker struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"` // e.g., "kafka"
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// WebBrokerEventPolicies holds policies for a single event type
type WebBrokerEventPolicies struct {
	Policies []Policy `json:"policies,omitempty"`
}

// WebBrokerAllChannelPolicies holds policies applied to all channels, organized by event type
type WebBrokerAllChannelPolicies struct {
	OnConnectionInit *WebBrokerEventPolicies `json:"on_connection_init,omitempty"`
	OnProduce        *WebBrokerEventPolicies `json:"on_produce,omitempty"`
	OnConsume        *WebBrokerEventPolicies `json:"on_consume,omitempty"`
}

// WebBrokerChannelPolicies holds policies applied to a specific channel, organized by event type
type WebBrokerChannelPolicies = WebBrokerAllChannelPolicies

// WebBrokerChannel represents a single channel with optional per-channel policy overrides
type WebBrokerChannel struct {
	ProduceTo        *WebBrokerTopic         `json:"produceTo,omitempty"`
	ConsumeFrom      *WebBrokerTopic         `json:"consumeFrom,omitempty"`
	OnConnectionInit *WebBrokerEventPolicies `json:"on_connection_init,omitempty"`
	OnProduce        *WebBrokerEventPolicies `json:"on_produce,omitempty"`
	OnConsume        *WebBrokerEventPolicies `json:"on_consume,omitempty"`
}

// WebBrokerTopic represents a topic configuration
type WebBrokerTopic struct {
	Topic string `json:"topic"`
}
