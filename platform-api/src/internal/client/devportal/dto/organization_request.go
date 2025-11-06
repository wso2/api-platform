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

// OrganizationCreateRequest represents the request body for creating an organization in dev poratl
//
// This DTO is used when platform-api synchronizes organizations to the dev poratl
// during organization creation workflow. Fields match the actual dev poratl API spec.
type OrganizationCreateRequest struct {
	OrgID                  string `json:"orgId"`                           // Organization UUID (optional, auto-generated if not provided)
	OrgName                string `json:"orgName"`                         // Organization name (required, unique)
	OrgHandle              string `json:"orgHandle"`                       // URL-friendly handle (required, unique)
	OrganizationIdentifier string `json:"organizationIdentifier"`          // Organization identifier (required, unique)
	BusinessOwner          string `json:"businessOwner,omitempty"`         // Business owner name (optional)
	BusinessOwnerContact   string `json:"businessOwnerContact,omitempty"`  // Contact number (optional)
	BusinessOwnerEmail     string `json:"businessOwnerEmail,omitempty"`    // Email address (optional)
	RoleClaimName          string `json:"roleClaimName,omitempty"`         // JWT claim for roles (default: "roles")
	GroupsClaimName        string `json:"groupsClaimName,omitempty"`       // JWT claim for groups (default: "groups")
	OrganizationClaimName  string `json:"organizationClaimName,omitempty"` // JWT claim for organization (default: "organizationID")
	AdminRole              string `json:"adminRole,omitempty"`             // Admin role name (default: "admin")
	SubscriberRole         string `json:"subscriberRole,omitempty"`        // Subscriber role (default: "Internal/subscriber")
	SuperAdminRole         string `json:"superAdminRole,omitempty"`        // Super admin role (default: "superAdmin")
}

// OrganizationCreateResponse represents the response from dev poratl after organization creation
//
// This DTO contains the confirmed organization details from the dev poratl.
type OrganizationCreateResponse struct {
	OrgID                  string                 `json:"orgId"`                  // Created organization UUID
	OrgName                string                 `json:"orgName"`                // Organization name
	OrgHandle              string                 `json:"orgHandle"`              // URL-friendly handle
	OrganizationIdentifier string                 `json:"organizationIdentifier"` // Organization identifier
	BusinessOwner          string                 `json:"businessOwner"`          // Business owner name
	BusinessOwnerContact   string                 `json:"businessOwnerContact"`   // Contact number
	BusinessOwnerEmail     string                 `json:"businessOwnerEmail"`     // Email address
	RoleClaimName          string                 `json:"roleClaimName"`          // JWT claim for roles
	GroupsClaimName        string                 `json:"groupsClaimName"`        // JWT claim for groups
	OrganizationClaimName  string                 `json:"organizationClaimName"`  // JWT claim for organization
	AdminRole              string                 `json:"adminRole"`              // Admin role name
	SubscriberRole         string                 `json:"subscriberRole"`         // Subscriber role
	SuperAdminRole         string                 `json:"superAdminRole"`         // Super admin role
	OrgConfiguration       map[string]interface{} `json:"orgConfiguration"`       // Organization configuration
}
