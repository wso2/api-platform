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
