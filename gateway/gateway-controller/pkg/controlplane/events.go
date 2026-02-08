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

package controlplane

// ConnectionAckMessage represents the acknowledgment message sent by the control plane
// upon successful WebSocket connection establishment
type ConnectionAckMessage struct {
	Type         string `json:"type"`         // Message type (always "connection.ack")
	GatewayID    string `json:"gatewayId"`    // Gateway UUID
	ConnectionID string `json:"connectionId"` // Unique connection UUID
	Timestamp    string `json:"timestamp"`    // RFC3339 timestamp
}

// APIDeployedEventPayload represents the payload of an API deployment event
type APIDeployedEventPayload struct {
	APIID       string `json:"apiId"`
	Environment string `json:"environment"`
	RevisionID  string `json:"revisionId"`
	VHost       string `json:"vhost"`
}

// APIDeployedEvent represents the complete API deployment event
type APIDeployedEvent struct {
	Type          string                  `json:"type"`
	Payload       APIDeployedEventPayload `json:"payload"`
	Timestamp     string                  `json:"timestamp"`
	CorrelationID string                  `json:"correlationId"`
}

// APIKeyCreatedEventPayload represents the payload of an API key created event.
type APIKeyCreatedEventPayload struct {
	ApiId         string  `json:"apiId"`
	ApiKey        string  `json:"apiKey"`         // Plain text API key (will be hashed by gateway)
	Name          string  `json:"name,omitempty"` //  URL-safe identifier (3-63 chars, lowercase alphanumeric with hyphens)
	ExternalRefId *string `json:"externalRefId,omitempty"`
	Operations    string  `json:"operations"`
	ExpiresAt     *string `json:"expiresAt,omitempty"` // ISO 8601 format
	ExpiresIn     *struct {
		Duration int    `json:"duration,omitempty"`
		Unit     string `json:"unit,omitempty"`
	} `json:"expiresIn,omitempty"`
	DisplayName *string `json:"displayName,omitempty"`
}

// APIKeyCreatedEvent represents the complete API key created event
type APIKeyCreatedEvent struct {
	Type          string                    `json:"type"`
	Payload       APIKeyCreatedEventPayload `json:"payload"`
	Timestamp     string                    `json:"timestamp"`
	CorrelationID string                    `json:"correlationId"`
	UserId        string                    `json:"userId"`
}

type APIKeyUpdatedEventPayload struct {
	ApiId         string  `json:"apiId"`
	KeyName       string  `json:"keyName"`
	ApiKey        string  `json:"apiKey"` // Plain text API key (will be hashed by gateway)
	ExternalRefId string  `json:"externalRefId"`
	Operations    string  `json:"operations"`
	ExpiresAt     *string `json:"expiresAt,omitempty"` // ISO 8601 format
	ExpiresIn     *struct {
		Duration int    `json:"duration,omitempty"`
		Unit     string `json:"unit,omitempty"`
	} `json:"expiresIn,omitempty"`
	DisplayName string `json:"displayName"`
}

// APIKeyUpdatedEvent represents the complete API key updated event
type APIKeyUpdatedEvent struct {
	Type          string                    `json:"type"`
	Payload       APIKeyUpdatedEventPayload `json:"payload"`
	Timestamp     string                    `json:"timestamp"`
	CorrelationID string                    `json:"correlationId"`
	UserId        string                    `json:"userId"`
}

// APIKeyRevokedEventPayload represents the payload of an API key revoked event
type APIKeyRevokedEventPayload struct {
	ApiId   string `json:"apiId"`
	KeyName string `json:"keyName"`
}

// APIKeyRevokedEvent represents the complete API key revoked event
type APIKeyRevokedEvent struct {
	Type          string                    `json:"type"`
	Payload       APIKeyRevokedEventPayload `json:"payload"`
	Timestamp     string                    `json:"timestamp"`
	CorrelationID string                    `json:"correlationId"`
	UserId        string                    `json:"userId"`
}
