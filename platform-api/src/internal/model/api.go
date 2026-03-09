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

package model

import (
	"time"
)

// API represents an API entity in the platform
type API struct {
	ID              string           `json:"id" db:"uuid"`
	Handle          string           `json:"handle" db:"handle"`
	Name            string           `json:"name" db:"name"`
	Kind            string           `json:"kind" db:"kind"`
	Description     string           `json:"description,omitempty" db:"description"`
	Version         string           `json:"version" db:"version"`
	CreatedBy       string           `json:"createdBy,omitempty" db:"created_by"`
	ProjectID       string           `json:"projectId" db:"project_uuid"`           // FK to Project.ID
	OrganizationID  string           `json:"organizationId" db:"organization_uuid"` // FK to Organization.ID
	CreatedAt       time.Time        `json:"createdAt,omitempty" db:"created_at"`
	UpdatedAt       time.Time        `json:"updatedAt,omitempty" db:"updated_at"`
	LifeCycleStatus string          `json:"lifeCycleStatus,omitempty" db:"lifecycle_status"`
	Transport       []string        `json:"transport,omitempty" db:"transport"`
	Channels        []Channel       `json:"channels,omitempty"`
	Configuration   RestAPIConfig    `json:"configuration" db:"-"`
}

type VhostsConfig struct {
	Main    string  `json:"main"`
	Sandbox *string `json:"sandbox,omitempty"`
}

type RestAPIConfig struct {
	Name       string         `json:"name,omitempty"`
	Version    string         `json:"version,omitempty"`
	Context    *string        `json:"context,omitempty"`
	Upstream   UpstreamConfig `json:"upstream,omitempty"`
	Policies   []Policy       `json:"policies,omitempty"`
	Operations []Operation    `json:"operations,omitempty"`
}

// TableName returns the table name for the API model
func (API) TableName() string {
	return "apis"
}

// APIMetadata contains minimal API information for handle-to-UUID resolution
type APIMetadata struct {
	ID             string `json:"id" db:"uuid"`
	Handle         string `json:"handle" db:"handle"`
	Name           string `json:"name" db:"name"`
	Version        string `json:"version" db:"version"`
	Kind           string `json:"kind" db:"kind"`
	OrganizationID string `json:"organizationId" db:"organization_uuid"`
}

// Operation represents an API operation
type Operation struct {
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Request     *OperationRequest `json:"request,omitempty"`
}

// Channel represents an API channel
type Channel struct {
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	Request     *ChannelRequest `json:"request,omitempty"`
}

// OperationRequest represents operation request details
type OperationRequest struct {
	Method   string   `json:"method,omitempty"`
	Path     string   `json:"path,omitempty"`
	Policies []Policy `json:"policies,omitempty"`
}

// ChannelRequest represents channel request details
type ChannelRequest struct {
	Method   string   `json:"method,omitempty"`
	Name     string   `json:"name,omitempty"`
	Policies []Policy `json:"policies,omitempty"`
}

// Policy represents a request or response policy
type Policy struct {
	ExecutionCondition *string                 `json:"executionCondition,omitempty"`
	Name               string                  `json:"name"`
	Params             *map[string]interface{} `json:"params,omitempty"`
	Version            string                  `json:"version"`
}

// APIAssociation represents the association between an API and a resource (gateway or dev portal)
type APIAssociation struct {
	ID              int64     `json:"id" db:"id"`
	ArtifactID      string    `json:"artifactId" db:"artifact_uuid"`
	OrganizationID  string    `json:"organizationId" db:"organization_uuid"`
	ResourceID      string    `json:"resourceId" db:"resource_uuid"`
	AssociationType string    `json:"associationType" db:"association_type"`
	CreatedAt       time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt       time.Time `json:"updatedAt" db:"updated_at"`
}

// TableName returns the table name for the APIAssociation model
func (APIAssociation) TableName() string {
	return "association_mappings"
}
