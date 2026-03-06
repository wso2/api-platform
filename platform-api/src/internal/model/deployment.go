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
	Status    *DeploymentStatus `json:"status,omitempty" db:"status"`
	UpdatedAt *time.Time        `json:"updatedAt,omitempty" db:"status_updated_at"`
}

// TableName returns the table name for the Deployment model
func (Deployment) TableName() string {
	return "deployments"
}

// DeploymentStatus represents the status of a deployment
// Note: ARCHIVED is a derived state (not stored in database)
type DeploymentStatus string

const (
	DeploymentStatusDeployed   DeploymentStatus = "DEPLOYED"
	DeploymentStatusUndeployed DeploymentStatus = "UNDEPLOYED"
	DeploymentStatusArchived   DeploymentStatus = "ARCHIVED" // Derived state: exists in history but not in status table
)

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
	DisplayName string         `yaml:"displayName" binding:"required"`
	Version     string         `yaml:"version" binding:"required"`
	Context     string         `yaml:"context" binding:"required"`
	Vhost       string         `yaml:"vhost" binding:"required"`
	Upstream    UpstreamConfig `yaml:"upstream" binding:"required"`
	SpecVersion string         `yaml:"specVersion" binding:"required"`
	Policies    []Policy       `yaml:"policies,omitempty"`
}
