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

import (
	"time"
)

// Deployment represents an immutable artifact deployment
// Status and UpdatedAt are populated from deployment_status table via JOIN
// If Status is nil, the deployment is ARCHIVED (not currently active or undeployed)
type Deployment struct {
	DeploymentID     string         `json:"deploymentId" db:"deployment_id"`
	Name             string         `json:"name" db:"name"`
	ArtifactID       string         `json:"artifactId" db:"artifact_uuid"`
	OrganizationID   string         `json:"organizationId" db:"organization_uuid"`
	GatewayID        string         `json:"gatewayId" db:"gateway_uuid"`
	BaseDeploymentID *string        `json:"baseDeploymentId,omitempty" db:"base_deployment_id"`
	Content          []byte         `json:"-" db:"content"`
	Metadata         map[string]any `json:"metadata,omitempty" db:"metadata"`
	CreatedAt        time.Time      `json:"createdAt" db:"created_at"`

	// Lifecycle state fields (from deployment_status table via JOIN)
	// nil values indicate ARCHIVED state (no record in status table)
	Status       *DeploymentStatus `json:"status,omitempty" db:"status"`
	UpdatedAt    *time.Time        `json:"updatedAt,omitempty" db:"status_updated_at"`
	StatusReason *string           `json:"statusReason,omitempty" db:"status_reason"`
}

// TableName returns the table name for the Deployment model
func (Deployment) TableName() string {
	return "deployments"
}

// DeploymentContent holds the artifact content for a single deployment,
// used internally when constructing batch archive responses.
type DeploymentContent struct {
	DeploymentID string
	ArtifactID   string
	Kind         string
	Content      []byte
}

// DeploymentStatus represents the status of a deployment
// Note: ARCHIVED is a derived state (not stored in database)
type DeploymentStatus string

const (
	DeploymentStatusDeployed    DeploymentStatus = "DEPLOYED"
	DeploymentStatusUndeployed  DeploymentStatus = "UNDEPLOYED"
	DeploymentStatusDeploying   DeploymentStatus = "DEPLOYING"
	DeploymentStatusUndeploying DeploymentStatus = "UNDEPLOYING"
	DeploymentStatusFailed      DeploymentStatus = "FAILED"
	DeploymentStatusArchived    DeploymentStatus = "ARCHIVED" // Derived state: exists in history but not in status table
)

// DeploymentInfo is a lightweight representation of a deployment
// Contains only the essential fields needed for listing deployments
type DeploymentInfo struct {
	DeploymentID string           `json:"deploymentId" db:"deployment_id"`
	ArtifactID   string           `json:"artifactId" db:"artifact_uuid"`
	Handle       string           `json:"handle" db:"handle"` // Artifact handle (apiId)
	Kind         string           `json:"kind" db:"kind"`     // Artifact kind: RestAPI, LLMProvider, LLMProxy, MCPProxy
	Status       DeploymentStatus `json:"status" db:"status"`
	PerformedAt  time.Time        `json:"performedAt" db:"performed_at"` // When the deploy/undeploy action was initiated
}

// DeploymentMetadata represents the metadata section of the API deployment YAML
type DeploymentMetadata struct {
	Name   string            `yaml:"name" binding:"required"`
	Labels map[string]string `yaml:"labels,omitempty"`
}

// MCPProxyDeploymentYAML represents the structure of the YAML used for deploying an MCP proxy
type MCPProxyDeploymentYAML struct {
	ApiVersion string                 `yaml:"apiVersion" binding:"required"`
	Kind       string                 `yaml:"kind" binding:"required"`
	Metadata   DeploymentMetadata     `yaml:"metadata" binding:"required"`
	Spec       MCPProxyDeploymentSpec `yaml:"spec" binding:"required"`
}

// MCPProxyDeploymentSpec represents the spec section of the MCP proxy deployment YAML
type MCPProxyDeploymentSpec struct {
	DisplayName string           `yaml:"displayName" binding:"required"`
	Version     string           `yaml:"version" binding:"required"`
	Context     string           `yaml:"context" binding:"required"`
	Vhost       *string          `yaml:"vhost" binding:"required"`
	Upstream    MCPProxyUpstream `yaml:"upstream" binding:"required"`
	SpecVersion string           `yaml:"specVersion" binding:"required"`
	Policies    []Policy         `yaml:"policies,omitempty"`
}

// Adding this type to support the model in the gateway side
type MCPProxyUpstream struct {
	URL  string        `yaml:"url" binding:"required"`
	Auth *UpstreamAuth `json:"auth,omitempty"`
}

// WebSubAPIDeploymentYAML represents the structure of the YAML used for deploying a WebSub API
type WebSubAPIDeploymentYAML struct {
	ApiVersion string                  `yaml:"apiVersion"`
	Kind       string                  `yaml:"kind"`
	Metadata   DeploymentMetadata      `yaml:"metadata"`
	Spec       WebSubAPIDeploymentSpec `yaml:"spec"`
}

// WebSubAPIDeploymentSpec represents the spec section of the WebSub API deployment YAML
type WebSubAPIDeploymentSpec struct {
	DisplayName string                     `yaml:"displayName"`
	Version     string                     `yaml:"version"`
	Context     string                     `yaml:"context"`
	Vhosts      *WebSubAPIDeploymentVhosts `yaml:"vhosts,omitempty"`
	Hub         WebSubHub                  `json:"hub" yaml:"hub"`
	Receiver    *WebSubReceiver            `json:"receiver,omitempty" yaml:"receiver,omitempty"`
	Delivery    *WebSubDelivery            `json:"delivery,omitempty" yaml:"delivery,omitempty"`
}

// WebSubAPIDeploymentVhosts represents vhost configuration in the WebSub API deployment YAML
type WebSubAPIDeploymentVhosts struct {
	Main    string  `yaml:"main"`
	Sandbox *string `yaml:"sandbox,omitempty"`
}

// WebSubAPIDeploymentChannel represents a channel in the WebSub API deployment YAML
type WebSubAPIDeploymentChannel struct {
	Name   string `yaml:"name"`
	Method string `yaml:"method"`
}

// WebSubDelivery Delivery configuration for the WebSub API - handles outbound event delivery to subscribers.
type WebSubDelivery struct {
	// Policies List of policies applied when delivering events to subscriber callback URLs (e.g., hmac-signature-validation)
	Policies *[]Policy `json:"policies,omitempty" yaml:"policies,omitempty"`
}

// WebSubHub Hub configuration for the WebSub API - handles subscriber management and event fan-out.
type WebSubHub struct {
	// Channels List of topic channels available for subscription
	Channels []HubChannel `json:"channels" yaml:"channels"`

	// Policies List of policies applied at the hub level (e.g., api-key-auth for subscribers)
	Policies *[]Policy `json:"policies,omitempty" yaml:"policies,omitempty"`
}

// HubChannel A subscribable topic channel within the WebSub hub.
type HubChannel struct {
	// Name Channel name or topic identifier relative to API context.
	Name string `json:"name" yaml:"name"`
	// Method The method by which the channel is identified (e.g., "SUB" for subscription-based, "PUB" for publish-only)
	Method string `json:"method" yaml:"method"`
	// Policies List of policies applied only to this channel (e.g., rbac)
	Policies *[]Policy `json:"policies,omitempty" yaml:"policies,omitempty"`
}

// WebSubReceiver Receiver configuration for the WebSub API - handles inbound event publishing from publishers.
type WebSubReceiver struct {
	// Policies List of policies applied to inbound webhook requests (e.g., hmac-signature-validation)
	Policies *[]Policy `json:"policies,omitempty" yaml:"policies,omitempty"`
}
