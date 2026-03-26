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

// DeploymentNotification represents the request body for gateway API deployment registration
type DeploymentNotification struct {
	ID                string           `json:"id" binding:"required"`
	Configuration     APIConfiguration `json:"configuration" binding:"required"`
	Status            string           `json:"status" binding:"required"`
	CreatedAt         time.Time        `json:"createdAt" binding:"required"`
	UpdatedAt         time.Time        `json:"updatedAt" binding:"required"`
	DeployedAt        *time.Time       `json:"deployedAt,omitempty"`
	ProjectIdentifier string           `json:"projectIdentifier" binding:"required"`
}

// APIConfiguration represents the API configuration
type APIConfiguration struct {
	Version string        `json:"version" yaml:"version" binding:"required"`
	Kind    string        `json:"kind" yaml:"kind" binding:"required"`
	Spec    APIConfigData `json:"spec" yaml:"spec" binding:"required"`
}

// APIConfigData represents the detailed API configuration
type APIConfigData struct {
	Name        string           `json:"name" yaml:"name" binding:"required"`
	Version     string           `json:"version" yaml:"version" binding:"required"`
	Context     string           `json:"context" yaml:"context" binding:"required"`
	ProjectName string           `json:"projectName,omitempty" yaml:"projectName,omitempty"`
	Upstreams   []Upstream       `json:"upstreams" yaml:"upstream" binding:"required"`
	Operations  []BasicOperation `json:"operations" yaml:"operations" binding:"required"`
}

// Upstream represents backend service configuration
type Upstream struct {
	URL string `json:"url" binding:"required"`
}

// BasicOperation represents API basic operation configuration
type BasicOperation struct {
	Method string `json:"method" binding:"required"`
	Path   string `json:"path" binding:"required"`
}

// GatewayDeploymentResponse represents the response for successful API deployment registration
type GatewayDeploymentResponse struct {
	APIId        string `json:"apiId"`
	DeploymentId int64  `json:"deploymentId"`
	Message      string `json:"message"`
	Created      bool   `json:"created"`
}

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

// GatewaySubscriptionPlanInfo represents a subscription plan in internal gateway responses.
type GatewaySubscriptionPlanInfo struct {
	ID                 string                 `json:"id"`
	PlanName           string                 `json:"planName"`
	BillingPlan        string                 `json:"billingPlan,omitempty"`
	StopOnQuotaReach   bool                   `json:"stopOnQuotaReach"`
	ThrottleLimitCount *int                   `json:"throttleLimitCount,omitempty"`
	ThrottleLimitUnit  string                 `json:"throttleLimitUnit,omitempty"`
	ExpiryTime         *time.Time             `json:"expiryTime,omitempty"`
	Status             string                 `json:"status"`
	CreatedAt          time.Time              `json:"createdAt"`
	UpdatedAt          time.Time              `json:"updatedAt"`
	Etag               string                 `json:"etag"` // Deterministic UUIDv7 derived from id + updatedAt
}

// GatewaySubscriptionInfo represents a subscription in internal gateway responses.
type GatewaySubscriptionInfo struct {
	ID                string    `json:"id"`
	APIID             string    `json:"apiId"`
	ApplicationID     *string   `json:"applicationId,omitempty"`
	SubscriptionToken string    `json:"subscriptionToken"`
	SubscriptionPlanID *string  `json:"subscriptionPlanId,omitempty"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
	Etag              string    `json:"etag"` // Deterministic UUIDv7 derived from id + updatedAt
}
