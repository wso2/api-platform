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
	DeploymentID     string         `json:"deploymentId" db:"deployment_uuid"`
	Name             string         `json:"name" db:"name"`
	ArtifactID       string         `json:"artifactId" db:"artifact_uuid"`
	OrganizationID   string         `json:"organizationId" db:"organization_uuid"`
	GatewayID        string         `json:"gatewayId" db:"gateway_uuid"`
	BaseDeploymentID *string        `json:"baseDeploymentId,omitempty" db:"base_deployment_uuid"`
	Content          []byte         `json:"-" db:"content"`
	Metadata         map[string]any `json:"metadata,omitempty" db:"metadata"`
	CreatedBy        string         `json:"createdBy,omitempty" db:"created_by"`
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
	Type         string
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
	DeploymentID string           `json:"deploymentId" db:"deployment_uuid"`
	ArtifactID   string           `json:"artifactId" db:"artifact_uuid"`
	Handle       string           `json:"handle" db:"handle"` // Artifact handle (apiId)
	Type         string           `json:"type" db:"type"`     // Artifact type: RestAPI, LLMProvider, LLMProxy, MCPProxy
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
	DisplayName     string                          `yaml:"displayName"`
	Version         string                          `yaml:"version"`
	Context         string                          `yaml:"context"`
	Vhosts          *WebSubAPIDeploymentVhosts      `yaml:"vhosts,omitempty"`
	AllChannels     *WebSubDeployAllChannelPolicies `yaml:"allChannels,omitempty"`
	Receiver        *WebSubDeployReceiver           `yaml:"receiver,omitempty"`
	Hub             *WebSubDeployHub                `yaml:"hub,omitempty"`
	Delivery        *WebSubDeployDelivery           `yaml:"delivery,omitempty"`
	Channels        map[string]WebSubDeployChannel  `yaml:"channels,omitempty"`
	DeploymentState string                          `yaml:"deploymentState,omitempty"`
}

// WebSubAPIDeploymentVhosts represents vhost configuration in the WebSub API deployment YAML
type WebSubAPIDeploymentVhosts struct {
	Main    string  `yaml:"main"`
	Sandbox *string `yaml:"sandbox,omitempty"`
}

// WebSubDeployEventPolicies wraps a list of policies for a single event type,
// matching the gateway controller's WebSubEventPolicies schema.
type WebSubDeployEventPolicies struct {
	Policies *[]Policy `yaml:"policies,omitempty"`
}

// WebSubDeployAllChannelPolicies represents policies for all channels in the deployment YAML, organized by event type.
type WebSubDeployAllChannelPolicies struct {
	OnSubscription    *WebSubDeployEventPolicies `yaml:"on_subscription,omitempty"`
	OnUnsubscription  *WebSubDeployEventPolicies `yaml:"on_unsubscription,omitempty"`
	OnMessageReceived *WebSubDeployEventPolicies `yaml:"on_message_received,omitempty"`
	OnMessageDelivery *WebSubDeployEventPolicies `yaml:"on_message_delivery,omitempty"`
}

// WebSubDeployChannel represents a single channel entry in the deployment YAML.
// Event policies are at the top level to match the gateway-controller's WebSubChannel schema.
type WebSubDeployChannel struct {
	OnSubscription    *WebSubDeployEventPolicies `yaml:"on_subscription,omitempty"`
	OnUnsubscription  *WebSubDeployEventPolicies `yaml:"on_unsubscription,omitempty"`
	OnMessageReceived *WebSubDeployEventPolicies `yaml:"on_message_received,omitempty"`
	OnMessageDelivery *WebSubDeployEventPolicies `yaml:"on_message_delivery,omitempty"`
}

// WebSubDeployReceiver represents the receiver section in the deployment YAML.
type WebSubDeployReceiver struct {
	Policies []Policy `yaml:"policies"`
}

// WebSubDeployHub represents the hub section in the deployment YAML.
type WebSubDeployHub struct {
	Policies []Policy                 `yaml:"policies"`
	Channels []WebSubDeployHubChannel `yaml:"channels,omitempty"`
}

// WebSubDeployHubChannel represents a channel entry under the hub section in the deployment YAML.
type WebSubDeployHubChannel struct {
	Name     string   `yaml:"name"`
	Policies []Policy `yaml:"policies"`
}

// WebSubDeployDelivery represents the delivery section in the deployment YAML.
type WebSubDeployDelivery struct {
	Policies []Policy `yaml:"policies"`
}

// WebBrokerAPIDeploymentYAML represents the structure of the YAML used for deploying a WebBroker API
type WebBrokerAPIDeploymentYAML struct {
	ApiVersion string                     `yaml:"apiVersion"`
	Kind       string                     `yaml:"kind"`
	Metadata   DeploymentMetadata         `yaml:"metadata"`
	Spec       WebBrokerAPIDeploymentSpec `yaml:"spec"`
}

// WebBrokerAPIDeploymentSpec represents the spec section of the WebBroker API deployment YAML
type WebBrokerAPIDeploymentSpec struct {
	DisplayName     string                             `yaml:"displayName"`
	Version         string                             `yaml:"version"`
	Context         string                             `yaml:"context"`
	Vhosts          *WebBrokerAPIDeploymentVhosts      `yaml:"vhosts,omitempty"`
	AllChannels     *WebBrokerDeployAllChannelPolicies `yaml:"allChannels,omitempty"`
	Receiver        *WebBrokerDeployReceiver           `yaml:"receiver,omitempty"`
	Broker          *WebBrokerDeployBroker             `yaml:"broker,omitempty"`
	Channels        map[string]WebBrokerDeployChannel  `yaml:"channels,omitempty"`
	DeploymentState string                             `yaml:"deploymentState,omitempty"`
}

// WebBrokerAPIDeploymentVhosts represents vhost configuration in the WebBroker API deployment YAML
type WebBrokerAPIDeploymentVhosts struct {
	Main    string  `yaml:"main"`
	Sandbox *string `yaml:"sandbox,omitempty"`
}

// WebBrokerDeployEventPolicies wraps a list of policies for a single event type,
// matching the gateway controller's WebBrokerEventPolicies schema.
type WebBrokerDeployEventPolicies struct {
	Policies *[]Policy `yaml:"policies,omitempty"`
}

// WebBrokerDeployAllChannelPolicies represents policies for all channels in the deployment YAML, organized by event type.
type WebBrokerDeployAllChannelPolicies struct {
	OnConnectionInit *WebBrokerDeployEventPolicies `yaml:"on_connection_init,omitempty"`
	OnProduce        *WebBrokerDeployEventPolicies `yaml:"on_produce,omitempty"`
	OnConsume        *WebBrokerDeployEventPolicies `yaml:"on_consume,omitempty"`
}

// WebBrokerDeployChannel represents a single channel entry in the deployment YAML.
type WebBrokerDeployChannel struct {
	ProduceTo        *WebBrokerDeployTopic         `yaml:"produceTo,omitempty"`
	ConsumeFrom      *WebBrokerDeployTopic         `yaml:"consumeFrom,omitempty"`
	OnConnectionInit *WebBrokerDeployEventPolicies `yaml:"on_connection_init,omitempty"`
	OnProduce        *WebBrokerDeployEventPolicies `yaml:"on_produce,omitempty"`
	OnConsume        *WebBrokerDeployEventPolicies `yaml:"on_consume,omitempty"`
}

// WebBrokerDeployTopic represents a topic configuration in the deployment YAML
type WebBrokerDeployTopic struct {
	Topic string `yaml:"topic"`
}

// WebBrokerDeployReceiver represents the receiver section in the deployment YAML.
type WebBrokerDeployReceiver struct {
	Name       string                 `yaml:"name"`
	Type       string                 `yaml:"type"`
	Properties map[string]interface{} `yaml:"properties,omitempty"`
	Policies   []Policy               `yaml:"policies,omitempty"`
}

// WebBrokerDeployBroker represents the broker section in the deployment YAML.
type WebBrokerDeployBroker struct {
	Name       string                 `yaml:"name"`
	Type       string                 `yaml:"type"`
	Properties map[string]interface{} `yaml:"properties,omitempty"`
	Policies   []Policy               `yaml:"policies,omitempty"`
}
