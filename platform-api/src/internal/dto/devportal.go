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
	IsActive      *bool   `json:"isActive,omitempty"`
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

// PublishToDevPortalRequest represents the request to publish an API to a specific DevPortal
type PublishToDevPortalRequest struct {
	DevPortalUUID string `json:"devPortalUuid" binding:"required"`
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

// ToModel converts CreateDevPortalRequest to DevPortal model
func (req *CreateDevPortalRequest) ToModel(orgUUID string) *model.DevPortal {
	visibility := req.Visibility
	if visibility == "" {
		visibility = "private" // Default to private if not specified
	}

	return &model.DevPortal{
		OrganizationUUID: orgUUID,
		Name:             req.Name,
		APIUrl:           req.APIUrl,
		Hostname:         req.Hostname,
		IsActive:         false, // New DevPortals start inactive; must be activated explicitly
		APIKey:           req.APIKey,
		HeaderKeyName:    req.HeaderKeyName,
		IsDefault:        false, // New DevPortals are never default, must use set-default endpoint
		Visibility:       visibility,
		Description:      req.Description,
		Identifier:       req.Identifier,
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
