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

// APIPublishRequest represents the complete API metadata for publishing to dev poratl
//
// This DTO is used when platform-api publishes APIs to the dev poratl. It matches
// the multipart form-data structure where this entire struct is serialized as JSON
// in the "apiMetadata" field, and the OpenAPI definition is sent separately as a file.
type APIPublishRequest struct {
	APIInfo              APIInfo              `json:"apiInfo"`                        // Required: Core API information
	SubscriptionPolicies []SubscriptionPolicy `json:"subscriptionPolicies,omitempty"` // Optional: Subscription policies for this API
	EndPoints            EndPoints            `json:"endPoints"`                      // Required: API endpoint URLs
}

// APIInfo contains core API metadata
//
// Required fields: ReferenceID, APIName, APIHandle, APIVersion, APIType
type APIInfo struct {
	APIID          string   `json:"apiId,omitempty"`         // Optional: API UUID (auto-generated if not provided)
	ReferenceID    string   `json:"referenceID"`             // Required: Unique reference ID from platform-api (UUID)
	APIName        string   `json:"apiName"`                 // Required: API name
	APIHandle      string   `json:"apiHandle"`               // Required: URL-friendly identifier (format: {apiName}-{version})
	Provider       string   `json:"provider,omitempty"`      // Optional: Provider name (default: "WSO2")
	APICategory    string   `json:"apiCategory,omitempty"`   // Optional: API category (e.g., "Travel", "Finance")
	APIDescription string   `json:"apiDescription"`          // Required: API description
	Visibility     string   `json:"visibility,omitempty"`    // Optional: "PUBLIC", "PRIVATE", or "RESTRICTED" (default: "PUBLIC")
	VisibleGroups  []string `json:"visibleGroups,omitempty"` // Optional: Array of group names (required if visibility is "RESTRICTED")
	Owners         *Owners  `json:"owners,omitempty"`        // Optional: Owner information
	APIVersion     string   `json:"apiVersion"`              // Required: API version (e.g., "3.0.2")
	APIType        string   `json:"apiType"`                 // Required: API type (use "REST" for RESTful APIs)
	APIStatus      string   `json:"apiStatus,omitempty"`     // Optional: API status (e.g., "PUBLISHED", "CREATED", "DEPRECATED")
	Labels         []string `json:"labels"`                  // Required: Array of label names
	Tags           []string `json:"tags,omitempty"`          // Optional: Array of tag strings
}

// Owners contains API owner information
type Owners struct {
	TechnicalOwner      string `json:"technicalOwner,omitempty"`      // Technical owner name
	TechnicalOwnerEmail string `json:"technicalOwnerEmail,omitempty"` // Technical owner email
	BusinessOwner       string `json:"businessOwner,omitempty"`       // Business owner name
	BusinessOwnerEmail  string `json:"businessOwnerEmail,omitempty"`  // Business owner email
}

// SubscriptionPolicy represents a subscription policy reference
//
// The policy must already exist in the organization before being referenced here.
type SubscriptionPolicy struct {
	PolicyName string `json:"policyName"` // Policy name (e.g., "unlimited", "gold", "platinum")
}

// EndPoints contains API endpoint URLs
//
// Both ProductionURL and SandboxURL are required fields.
type EndPoints struct {
	ProductionURL string `json:"productionURL"` // Required: Production endpoint URL
	SandboxURL    string `json:"sandboxURL"`    // Required: Sandbox endpoint URL
}

// APIPublishResponse represents the response from dev portal after API publishing
type APIPublishResponse struct {
	APIID       string `json:"apiID"`       // Created/updated API UUID in ap portal
	APIHandle   string `json:"apiHandle"`   // URL-friendly identifier
	ReferenceID string `json:"referenceID"` // Reference ID from platform-api
	Message     string `json:"message"`     // Success message
}
