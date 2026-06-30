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

package dto

import "time"

// GatewayDeploymentInfo represents a single deployment for a gateway
// Used to compare local gateway state with platform-api state
type GatewayDeploymentInfo struct {
	ArtifactID   string    `json:"artifactId"`   // Artifact identifier (handle) - REST API, LLM Provider, or LLM Proxy
	DeploymentID string    `json:"deploymentId"` // Unique deployment artifact ID
	Kind         string    `json:"kind"`         // Artifact type: RestAPI, LLMProvider, LLMProxy
	State        string    `json:"state"`        // Deployment state: DEPLOYED or UNDEPLOYED
	DeployedAt   time.Time `json:"deployedAt"`   // Timestamp when the deployment action was performed
	Etag         string    `json:"etag"`         // Deterministic UUIDv7 derived from deploymentId + deployedAt
}

// GatewayDeploymentsResponse represents the response for listing gateway deployments
type GatewayDeploymentsResponse struct {
	Deployments []GatewayDeploymentInfo `json:"deployments"`
}

// DeploymentsBatchFetchRequest represents the request body for batch fetching deployments
type DeploymentsBatchFetchRequest struct {
	DeploymentIDs []string `json:"deploymentIds" binding:"required,min=1"`
}

// ArtifactsExistRequest represents the request body for checking artifact existence
type ArtifactsExistRequest struct {
	ArtifactIDs []string `json:"artifactIds" binding:"required,min=1"`
}

// ArtifactExistenceInfo represents the existence status of a single artifact
type ArtifactExistenceInfo struct {
	ArtifactID string `json:"artifactId"`
	Exists     bool   `json:"exists"`
}

// ArtifactsExistResponse represents the response for checking artifact existence
type ArtifactsExistResponse struct {
	Artifacts []ArtifactExistenceInfo `json:"artifacts"`
}

// GatewaySubscriptionPlanInfo represents a subscription plan in internal gateway responses.
//
// StopOnQuotaReach is exposed as a boolean to match the gateway-controller's model;
// the platform-api stores it as a SMALLINT (0/1) and converts at this boundary.
type GatewaySubscriptionPlanInfo struct {
	ID                 string     `json:"id"`
	Handle             string     `json:"handle"`
	PlanName           string     `json:"planName"`
	BillingPlan        string     `json:"billingPlan,omitempty"`
	StopOnQuotaReach   bool       `json:"stopOnQuotaReach"`
	ThrottleLimitCount *int       `json:"throttleLimitCount,omitempty"`
	ThrottleLimitUnit  string     `json:"throttleLimitUnit,omitempty"`
	ExpiryTime         *time.Time `json:"expiryTime,omitempty"`
	Status             string     `json:"status"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
	Etag               string     `json:"etag"` // Deterministic UUIDv7 derived from id + updatedAt
}

// GatewayHmacSecretInfo represents a single HMAC secret returned to the gateway-controller.
// The Secret field contains the plaintext value — this is only exposed on the internal endpoint.
type GatewayHmacSecretInfo struct {
	Name   string `json:"name"`
	Secret string `json:"secret"`
}

// GatewayHmacSecretsResponse is the response for GET /api/internal/v1/websub-apis/:apiId/secrets.
type GatewayHmacSecretsResponse struct {
	ArtifactID string                  `json:"artifactId"`
	Secrets    []GatewayHmacSecretInfo `json:"secrets"`
}

// GatewaySubscriptionInfo represents a subscription in internal gateway responses.
type GatewaySubscriptionInfo struct {
	ID                 string    `json:"id"`
	APIID              string    `json:"apiId"`
	ApplicationID      *string   `json:"applicationId,omitempty"`
	SubscriptionToken  string    `json:"subscriptionToken"`
	SubscriptionPlanID *string   `json:"subscriptionPlanId,omitempty"`
	Status             string    `json:"status"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
	Etag               string    `json:"etag"` // Deterministic UUIDv7 derived from id + updatedAt
}
