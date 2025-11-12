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

// Owners describes API owners information
type Owners struct {
	TechnicalOwner      string `json:"technicalOwner,omitempty"`
	TechnicalOwnerEmail string `json:"technicalOwnerEmail,omitempty"`
	BusinessOwner       string `json:"businessOwner,omitempty"`
	BusinessOwnerEmail  string `json:"businessOwnerEmail,omitempty"`
}

// APIInfo contains basic API metadata
type APIInfo struct {
	APIID          string        `json:"apiId,omitempty"`
	ReferenceID    string        `json:"referenceID,omitempty"`
	APIStatus      string        `json:"apiStatus,omitempty"`
	Provider       string        `json:"provider,omitempty"`
	APIName        string        `json:"apiName"`
	APIHandle      string        `json:"apiHandle"`
	APIDescription string        `json:"apiDescription,omitempty"`
	APIVersion     string        `json:"apiVersion,omitempty"`
	APIType        APIType       `json:"apiType,omitempty"`
	Visibility     APIVisibility `json:"visibility,omitempty"`
	VisibleGroups  []string      `json:"visibleGroups,omitempty"`
	Tags           []string      `json:"tags,omitempty"`
	Owners         Owners        `json:"owners,omitempty"`
	Labels         []string      `json:"labels,omitempty"`
}

// EndPoints describes production/sandbox endpoints
type EndPoints struct {
	ProductionURL string `json:"productionURL,omitempty"`
	SandboxURL    string `json:"sandboxURL,omitempty"`
}

// APIMetadataRequest is the JSON payload placed in multipart field `apiMetadata`
type APIMetadataRequest struct {
	APIInfo              APIInfo              `json:"apiInfo"`
	EndPoints            EndPoints            `json:"endPoints,omitempty"`
	SubscriptionPolicies []string 				`json:"subscriptionPolicies,omitempty"`
}

// APIResponse represents an API returned by the DevPortal
type APIResponse struct {
	ID        string    `json:"id,omitempty"`
	APIInfo   APIInfo   `json:"apiInfo,omitempty"`
	EndPoints EndPoints `json:"endPoints,omitempty"`
}

// APIListResponse is a list of APIs
type APIListResponse struct {
	Items []APIResponse `json:"items"`
}
