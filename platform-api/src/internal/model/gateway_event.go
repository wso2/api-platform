/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package model

import (
	"encoding/json"
	"time"
)

// GatewayEvent represents a notification to be sent to a gateway instance.
// Events are sent over the WebSocket connection to notify gateways of
// platform-side changes that require gateway action (e.g., API deployment).
type GatewayEvent struct {
	// Type identifies the event category (e.g., "api.deployed", "api.undeployed")
	Type string `json:"type"`

	// Payload contains event-specific data as raw JSON.
	// The structure depends on the event type:
	//   - "api.deployed": APIDeploymentEvent
	//   - "api.undeployed": APIUndeploymentEvent
	//   - "gateway.config.updated": GatewayConfigEvent
	Payload json.RawMessage `json:"payload"`

	// GatewayID identifies the target gateway for this event (UUID)
	GatewayID string `json:"gatewayId"`

	// Timestamp records when the event was created on the platform
	Timestamp time.Time `json:"timestamp"`

	// CorrelationID provides a unique identifier for request tracing across systems
	CorrelationID string `json:"correlationId"`
}

// APIDeploymentEvent contains payload data for "api.deployed" event type.
// This event is sent when an API revision is successfully deployed to a gateway.
type APIDeploymentEvent struct {
	// APIUUID identifies the deployed API
	APIUUID string `json:"apiUuid"`

	// RevisionID identifies the specific API revision deployed
	RevisionID string `json:"revisionId"`

	// Vhost specifies the virtual host where the API is deployed
	Vhost string `json:"vhost"`

	// Environment specifies the deployment environment (e.g., "production", "sandbox")
	Environment string `json:"environment"`
}

// APIUndeploymentEvent contains payload data for "api.undeployed" event type.
// This event is sent when an API is undeployed from a gateway.
type APIUndeploymentEvent struct {
	// APIUUID identifies the undeployed API
	APIUUID string `json:"apiUuid"`

	// Vhost specifies the virtual host from which the API is undeployed
	Vhost string `json:"vhost"`

	// Environment specifies the deployment environment
	Environment string `json:"environment"`
}

// GatewayConfigEvent contains payload data for "gateway.config.updated" event type.
// This event is sent when gateway configuration needs to be refreshed.
type GatewayConfigEvent struct {
	// ConfigType identifies the configuration category (e.g., "rate-limit", "cors")
	ConfigType string `json:"configType"`

	// Action specifies the configuration change action ("update", "delete", "refresh")
	Action string `json:"action"`
}
