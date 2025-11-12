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

// OrganizationCreateRequest is the payload used to create an organization in DevPortal.
type OrganizationCreateRequest struct {
	OrgID                  string                 `json:"orgId,omitempty"`
	OrgName                string                 `json:"orgName"`
	OrgHandle              string                 `json:"orgHandle"`
	OrganizationIdentifier string                 `json:"organizationIdentifier"`
	BusinessOwner          string                 `json:"businessOwner,omitempty"`
	BusinessOwnerContact   string                 `json:"businessOwnerContact,omitempty"`
	BusinessOwnerEmail     string                 `json:"businessOwnerEmail,omitempty"`
	RoleClaimName          string                 `json:"roleClaimName,omitempty"`
	GroupsClaimName        string                 `json:"groupsClaimName,omitempty"`
	OrganizationClaimName  string                 `json:"organizationClaimName,omitempty"`
	AdminRole              string                 `json:"adminRole,omitempty"`
	SubscriberRole         string                 `json:"subscriberRole,omitempty"`
	SuperAdminRole         string                 `json:"superAdminRole,omitempty"`
	OrgConfig              map[string]interface{} `json:"orgConfig,omitempty"`
}

// OrganizationResponse is the representation returned by DevPortal for an organization.
type OrganizationResponse struct {
	OrgID                  string                 `json:"orgId"`
	OrgName                string                 `json:"orgName"`
	OrgHandle              string                 `json:"orgHandle"`
	OrganizationIdentifier string                 `json:"organizationIdentifier"`
	BusinessOwner          string                 `json:"businessOwner,omitempty"`
	BusinessOwnerContact   string                 `json:"businessOwnerContact,omitempty"`
	BusinessOwnerEmail     string                 `json:"businessOwnerEmail,omitempty"`
	RoleClaimName          string                 `json:"roleClaimName,omitempty"`
	GroupsClaimName        string                 `json:"groupsClaimName,omitempty"`
	OrganizationClaimName  string                 `json:"organizationClaimName,omitempty"`
	AdminRole              string                 `json:"adminRole,omitempty"`
	SubscriberRole         string                 `json:"subscriberRole,omitempty"`
	SuperAdminRole         string                 `json:"superAdminRole,omitempty"`
	OrgConfiguration       map[string]interface{} `json:"orgConfiguration,omitempty"`
}

// OrganizationUpdateRequest contains fields allowed for updates.
type OrganizationUpdateRequest struct {
	OrgName       *string                `json:"orgName,omitempty"`
	BusinessOwner *string                `json:"businessOwner,omitempty"`
	OrgConfig     map[string]interface{} `json:"orgConfig,omitempty"`
}

// OrganizationListResponse is the JSON array returned by the DevPortal for a list
type OrganizationListResponse []OrganizationResponse
