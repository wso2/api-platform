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

// APIDeploymentNotification represents the request body for gateway API deployment registration
type APIDeploymentNotification struct {
	ID                string           `json:"id" binding:"required"`
	Configuration     APIConfiguration `json:"configuration" binding:"required"`
	Status            string           `json:"status" binding:"required"`
	CreatedAt         time.Time        `json:"createdAt" binding:"required"`
	UpdatedAt         time.Time        `json:"updatedAt" binding:"required"`
	DeployedAt        *time.Time       `json:"deployedAt,omitempty"`
	DeployedVersion   *int             `json:"deployedVersion,omitempty"`
	ProjectIdentifier string           `json:"projectIdentifier" binding:"required"`
}

// APIConfiguration represents the API configuration data
type APIConfiguration struct {
	Version string        `json:"version" binding:"required"`
	Kind    string        `json:"kind" binding:"required"`
	Data    APIConfigData `json:"data" binding:"required"`
}

// APIConfigData represents the detailed API configuration
type APIConfigData struct {
	Name        string           `json:"name" binding:"required"`
	Version     string           `json:"version" binding:"required"`
	Context     string           `json:"context" binding:"required"`
	ProjectName string           `json:"projectName,omitempty"`
	Upstream    []Upstream       `json:"upstream" binding:"required"`
	Operations  []BasicOperation `json:"operations" binding:"required"`
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

// GatewayAPIDeploymentResponse represents the response for successful API deployment registration
type GatewayAPIDeploymentResponse struct {
	APIId        string `json:"apiId"`
	DeploymentId int64  `json:"deploymentId"`
	Message      string `json:"message"`
	Created      bool   `json:"created"`
}
