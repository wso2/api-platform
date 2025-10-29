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

// OrganizationCreateRequest represents the request body for creating an organization in developer portal
//
// This DTO is used when platform-api synchronizes organizations to the developer portal
// during organization creation workflow.
type OrganizationCreateRequest struct {
	ID          string `json:"id"`          // UUID from platform-api (must match)
	Name        string `json:"name"`        // Organization name
	DisplayName string `json:"displayName"` // Human-readable display name
	Description string `json:"description"` // Organization description
}

// OrganizationCreateResponse represents the response from developer portal after organization creation
//
// This DTO contains the confirmed organization details from the developer portal,
// including the creation timestamp.
type OrganizationCreateResponse struct {
	ID          string `json:"id"`          // Created organization UUID (should match request ID)
	Name        string `json:"name"`        // Organization name
	DisplayName string `json:"displayName"` // Display name
	CreatedAt   string `json:"createdAt"`   // Timestamp of creation (ISO 8601)
}
