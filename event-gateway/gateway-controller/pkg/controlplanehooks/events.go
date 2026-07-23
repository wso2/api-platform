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

// Package controlplanehooks implements gateway-controller (core)'s
// controlplane.ControlPlaneEventGatewayHooks interface, moved out of core's
// pkg/controlplane/client.go and pkg/controlplane/events.go.
package controlplanehooks

import "time"

// WebSubAPIDeployedEventPayload represents the payload of a WebSub API deployment event.
type WebSubAPIDeployedEventPayload struct {
	APIID        string    `json:"apiId"`
	DeploymentID string    `json:"deploymentId"`
	PerformedAt  time.Time `json:"performedAt"`
}

// WebSubAPIDeployedEvent represents the complete WebSub API deployment event.
type WebSubAPIDeployedEvent struct {
	Type          string                        `json:"type"`
	Payload       WebSubAPIDeployedEventPayload `json:"payload"`
	Timestamp     string                        `json:"timestamp"`
	CorrelationID string                        `json:"correlationId"`
}

// WebSubAPIUndeployedEventPayload represents the payload of a WebSub API undeployment event.
type WebSubAPIUndeployedEventPayload struct {
	APIID        string    `json:"apiId"`
	DeploymentID string    `json:"deploymentId"`
	PerformedAt  time.Time `json:"performedAt"`
}

// WebSubAPIUndeployedEvent represents the complete WebSub API undeployment event.
type WebSubAPIUndeployedEvent struct {
	Type          string                           `json:"type"`
	Payload       WebSubAPIUndeployedEventPayload `json:"payload"`
	Timestamp     string                           `json:"timestamp"`
	CorrelationID string                           `json:"correlationId"`
}

// WebSubAPIDeletedEventPayload represents the payload of a WebSub API deletion event.
type WebSubAPIDeletedEventPayload struct {
	APIID string `json:"apiId"`
}

// WebSubAPIDeletedEvent represents the complete WebSub API deletion event.
type WebSubAPIDeletedEvent struct {
	Type          string                       `json:"type"`
	Payload       WebSubAPIDeletedEventPayload `json:"payload"`
	Timestamp     string                       `json:"timestamp"`
	CorrelationID string                       `json:"correlationId"`
}

// WebBrokerAPIDeployedEventPayload represents the payload of a WebBroker API deployment event.
type WebBrokerAPIDeployedEventPayload struct {
	APIID        string    `json:"apiId"`
	DeploymentID string    `json:"deploymentId"`
	PerformedAt  time.Time `json:"performedAt"`
}

// WebBrokerAPIDeployedEvent represents the complete WebBroker API deployment event.
type WebBrokerAPIDeployedEvent struct {
	Type          string                           `json:"type"`
	Payload       WebBrokerAPIDeployedEventPayload `json:"payload"`
	Timestamp     string                           `json:"timestamp"`
	CorrelationID string                           `json:"correlationId"`
}

// WebBrokerAPIUndeployedEventPayload represents the payload of a WebBroker API undeployment event.
type WebBrokerAPIUndeployedEventPayload struct {
	APIID        string    `json:"apiId"`
	DeploymentID string    `json:"deploymentId"`
	PerformedAt  time.Time `json:"performedAt"`
}

// WebBrokerAPIUndeployedEvent represents the complete WebBroker API undeployment event.
type WebBrokerAPIUndeployedEvent struct {
	Type          string                             `json:"type"`
	Payload       WebBrokerAPIUndeployedEventPayload `json:"payload"`
	Timestamp     string                             `json:"timestamp"`
	CorrelationID string                             `json:"correlationId"`
}

// WebBrokerAPIDeletedEventPayload represents the payload of a WebBroker API deletion event.
type WebBrokerAPIDeletedEventPayload struct {
	APIID string `json:"apiId"`
}

// WebBrokerAPIDeletedEvent represents the complete WebBroker API deletion event.
type WebBrokerAPIDeletedEvent struct {
	Type          string                          `json:"type"`
	Payload       WebBrokerAPIDeletedEventPayload `json:"payload"`
	Timestamp     string                          `json:"timestamp"`
	CorrelationID string                          `json:"correlationId"`
}

// platformHmacSecretEventPayload is the payload for websub.hmacsecret.* events.
type platformHmacSecretEventPayload struct {
	ArtifactUUID string `json:"artifactUuid"`
	SecretName   string `json:"secretName"`
}

// platformHmacSecretInfo is the per-secret DTO returned by the internal HMAC endpoint.
type platformHmacSecretInfo struct {
	Name   string `json:"name"`
	Secret string `json:"secret"`
}

// platformHmacSecretsResponse is the response body from GET /websub-apis/:id/secrets.
type platformHmacSecretsResponse struct {
	ArtifactID string                   `json:"artifactId"`
	Secrets    []platformHmacSecretInfo `json:"secrets"`
}

// hmacSecretInfo is the internal view of a platform-managed HMAC secret.
type hmacSecretInfo struct {
	Name      string
	Plaintext string
}
