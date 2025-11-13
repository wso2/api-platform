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

import (
	"platform-api/src/internal/model"
	"strings"
	"time"
)

// CreateDevPortalRequest represents the request to create a new DevPortal
type CreateDevPortalRequest struct {
	Name          string `json:"name" binding:"required" validate:"min=1,max=100"`
	APIUrl        string `json:"apiUrl" binding:"required" validate:"url"`
	Hostname      string `json:"hostname" binding:"required" validate:"hostname"`
	APIKey        string `json:"apiKey" binding:"required"`
	HeaderKeyName string `json:"headerKeyName,omitempty"`
	Visibility    string `json:"visibility,omitempty" validate:"omitempty,oneof=public private"`
	Description   string `json:"description,omitempty" validate:"max=500"`
	Identifier    string `json:"identifier" binding:"required" validate:"min=1,max=100"`
}

// UpdateDevPortalRequest represents the request to update a DevPortal
type UpdateDevPortalRequest struct {
	Name          *string `json:"name,omitempty" validate:"omitempty,min=1,max=100"`
	APIUrl        *string `json:"apiUrl,omitempty" validate:"omitempty,url"`
	Hostname      *string `json:"hostname,omitempty" validate:"omitempty,hostname"`
	APIKey        *string `json:"apiKey,omitempty"`
	HeaderKeyName *string `json:"headerKeyName,omitempty"`
	Visibility    *string `json:"visibility,omitempty" validate:"omitempty,oneof=public private"`
	Description   *string `json:"description,omitempty" validate:"omitempty,max=500"`
}

// DevPortalResponse represents the response when returning DevPortal information
type DevPortalResponse struct {
	UUID             string    `json:"uuid"`
	OrganizationUUID string    `json:"organizationUuid"`
	Name             string    `json:"name"`
	Identifier       string    `json:"identifier"`
	UIUrl            string    `json:"uiUrl"`
	APIUrl           string    `json:"apiUrl"`
	Hostname         string    `json:"hostname"`
	IsActive         bool      `json:"isActive"`
	IsEnabled        bool      `json:"isEnabled"`
	HeaderKeyName    string    `json:"headerKeyName,omitempty"`
	IsDefault        bool      `json:"isDefault"`
	Visibility       string    `json:"visibility"`
	Description      string    `json:"description"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// DevPortalListResponse represents a paginated list of DevPortals
type DevPortalListResponse struct {
	Count      int                  `json:"count"`
	List       []*DevPortalResponse `json:"list"`
	Pagination Pagination           `json:"pagination"`
}

// Platform-specific DTOs for API publishing

// Owners describes API owners information
type Owners struct {
	TechnicalOwner      string `json:"technicalOwner,omitempty"`
	TechnicalOwnerEmail string `json:"technicalOwnerEmail,omitempty"`
	BusinessOwner       string `json:"businessOwner,omitempty"`
	BusinessOwnerEmail  string `json:"businessOwnerEmail,omitempty"`
}

// PublishAPIInfo contains user-overridable API metadata for publishing
type PublishAPIInfo struct {
	APIName        string   `json:"apiName,omitempty"`
	APIDescription string   `json:"apiDescription,omitempty"`
	APIType        string   `json:"apiType,omitempty"`
	Visibility     string   `json:"visibility,omitempty"`
	VisibleGroups  []string `json:"visibleGroups,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	Owners         Owners   `json:"owners,omitempty"`
	Labels         []string `json:"labels,omitempty"`
}

// APIInfo contains basic API metadata for platform API publishing
type APIInfo struct {
	APIID          string   `json:"apiId,omitempty"`
	ReferenceID    string   `json:"referenceId,omitempty"`
	APIName        string   `json:"apiName"`
	APIHandle      string   `json:"apiHandle,omitempty"`
	APIDescription string   `json:"apiDescription,omitempty"`
	APIVersion     string   `json:"apiVersion,omitempty"`
	APIType        string   `json:"apiType,omitempty"`
	APIStatus      string   `json:"apiStatus,omitempty"`
	Provider       string   `json:"provider,omitempty"`
	Visibility     string   `json:"visibility,omitempty"`
	VisibleGroups  []string `json:"visibleGroups,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	Owners         Owners   `json:"owners,omitempty"`
	Labels         []string `json:"labels,omitempty"`
}

// EndPoints describes production/sandbox endpoints
type EndPoints struct {
	ProductionURL string `json:"productionURL,omitempty"`
	SandboxURL    string `json:"sandboxURL,omitempty"`
}

// APIMetadataRequest is the platform API metadata request with string-based subscription policies
type APIMetadataRequest struct {
	APIInfo              APIInfo   `json:"apiInfo"`
	EndPoints            EndPoints `json:"endPoints,omitempty"`
	SubscriptionPolicies []string  `json:"subscriptionPolicies,omitempty"`
}

// PublishToDevPortalRequest represents the request to publish an API to a specific DevPortal
type PublishToDevPortalRequest struct {
	DevPortalUUID        string          `json:"devPortalUuid" binding:"required"`
	APIInfo              *PublishAPIInfo `json:"apiInfo,omitempty"` // Made optional for defaults
	EndPoints            EndPoints       `json:"endPoints" binding:"required"`
	SubscriptionPolicies []string        `json:"subscriptionPolicies,omitempty"`
}

// PublishToDevPortalResponse represents the response after publishing an API to a DevPortal
type PublishToDevPortalResponse struct {
	Message        string    `json:"message"`
	APIID          string    `json:"apiId"`
	DevPortalUUID  string    `json:"devPortalUuid"`
	DevPortalName  string    `json:"devPortalName"`
	ApiPortalRefID string    `json:"apiPortalRefId"`
	PublishedAt    time.Time `json:"publishedAt"`
}

// UnpublishFromDevPortalRequest represents the request to unpublish an API from a DevPortal
type UnpublishFromDevPortalRequest struct {
	DevPortalUUID string `json:"devPortalUuid" binding:"required"`
}

// UnpublishFromDevPortalResponse represents the response after unpublishing an API from a DevPortal
type UnpublishFromDevPortalResponse struct {
	Message       string    `json:"message"`
	APIID         string    `json:"apiId"`
	DevPortalUUID string    `json:"devPortalUuid"`
	DevPortalName string    `json:"devPortalName"`
	UnpublishedAt time.Time `json:"unpublishedAt"`
}

// ActivateDevPortalResponse represents the response after activating a DevPortal
type ActivateDevPortalResponse struct {
	Message       string    `json:"message"`
	DevPortalUUID string    `json:"devPortalUuid"`
	DevPortalName string    `json:"devPortalName"`
	ActivatedAt   time.Time `json:"activatedAt"`
}

// DeactivateDevPortalResponse represents the response after deactivating a DevPortal
type DeactivateDevPortalResponse struct {
	Message       string    `json:"message"`
	DevPortalUUID string    `json:"devPortalUuid"`
	DevPortalName string    `json:"devPortalName"`
	DeactivatedAt time.Time `json:"deactivatedAt"`
}

// APIPublicationDetails represents publication-specific information
// This mirrors the deployment details structure used for gateways
type APIPublicationDetails struct {
	Status             string    `json:"status"` // published, failed, publishing
	APIVersion         string    `json:"apiVersion,omitempty"`
	DevPortalRefID     string    `json:"devPortalRefId,omitempty"`
	SandboxEndpoint    string    `json:"sandboxEndpoint"`
	ProductionEndpoint string    `json:"productionEndpoint"`
	PublishedAt        time.Time `json:"publishedAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// APIDevPortalResponse represents a DevPortal with API association and publication details
// This extends DevPortalResponse with additional association and publication fields
type APIDevPortalResponse struct {
	DevPortalResponse                        // Embedded DevPortal details
	AssociatedAt      time.Time              `json:"associatedAt"`
	IsPublished       bool                   `json:"isPublished"`
	Publication       *APIPublicationDetails `json:"publication,omitempty"` // Only present when isPublished is true
}

// APIDevPortalListResponse represents a paginated list of DevPortals with API association and publication details
type APIDevPortalListResponse struct {
	Count      int                    `json:"count"`      // Number of items in current response
	List       []APIDevPortalResponse `json:"list"`       // Array of DevPortal objects with publication details
	Pagination Pagination             `json:"pagination"` // Pagination metadata
}

// ToModel converts CreateDevPortalRequest to DevPortal model
func (req *CreateDevPortalRequest) ToModel(orgUUID string) *model.DevPortal {
	visibility := req.Visibility
	if visibility == "" {
		visibility = "private" // Default to private if not specified
	}

	return &model.DevPortal{
		OrganizationUUID: orgUUID,
		Name:             strings.TrimSpace(req.Name),
		APIUrl:           strings.TrimSpace(req.APIUrl),
		Hostname:         strings.TrimSpace(req.Hostname),
		APIKey:           strings.TrimSpace(req.APIKey),
		IsActive:         false, // New DevPortals start inactive; must be activated explicitly
		IsEnabled:        false, // New DevPortals are disabled by default
		HeaderKeyName:    strings.TrimSpace(req.HeaderKeyName),
		IsDefault:        false, // New DevPortals are never default, must use set-default endpoint
		Visibility:       visibility,
		Description:      strings.TrimSpace(req.Description),
		Identifier:       strings.TrimSpace(req.Identifier),
	}
}

// ToResponse converts DevPortal model to DevPortalResponse (without API key for security)
func ToDevPortalResponse(devportal *model.DevPortal) *DevPortalResponse {
	return &DevPortalResponse{
		UUID:             devportal.UUID,
		OrganizationUUID: devportal.OrganizationUUID,
		Name:             devportal.Name,
		Identifier:       devportal.Identifier,
		UIUrl:            devportal.GetUIUrl(), // Computed field
		APIUrl:           devportal.APIUrl,
		Hostname:         devportal.Hostname,
		IsActive:         devportal.IsActive,
		IsEnabled:        devportal.IsEnabled,
		HeaderKeyName:    devportal.HeaderKeyName,
		IsDefault:        devportal.IsDefault,
		Visibility:       devportal.Visibility,
		Description:      devportal.Description,
		CreatedAt:        devportal.CreatedAt,
		UpdatedAt:        devportal.UpdatedAt,
	}
}

// ToDevPortalListResponse converts a slice of DevPortal models to DevPortalListResponse
func ToDevPortalListResponse(devportals []*model.DevPortal, pagination Pagination) *DevPortalListResponse {
	responses := make([]*DevPortalResponse, len(devportals))
	for i, devportal := range devportals {
		responses[i] = ToDevPortalResponse(devportal)
	}

	return &DevPortalListResponse{
		Count:      len(responses),
		List:       responses,
		Pagination: pagination,
	}
}
