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
	"fmt"
	"platform-api/src/internal/constants"
	"time"
)

// DevPortal represents a developer portal associated with an organization
type DevPortal struct {
	UUID             string    `json:"uuid" db:"uuid"`
	OrganizationUUID string    `json:"organizationUuid" db:"organization_uuid"`
	Name             string    `json:"name" db:"name"`
	Identifier       string    `json:"identifier" db:"identifier"`
	APIUrl           string    `json:"apiUrl" db:"api_url"`
	Hostname         string    `json:"hostname" db:"hostname"`
	IsActive         bool      `json:"isActive" db:"is_active"`
	IsEnabled        bool      `json:"isEnabled" db:"is_enabled"`
	APIKey           string    `json:"apiKey" db:"api_key"`
	HeaderKeyName    string    `json:"headerKeyName" db:"header_key_name"`
	IsDefault        bool      `json:"isDefault" db:"is_default"`
	Visibility       string    `json:"visibility" db:"visibility"`
	Description      string    `json:"description" db:"description"`
	CreatedAt        time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt        time.Time `json:"updatedAt" db:"updated_at"`
}

// GetUIUrl returns the computed UI URL based on API URL and identifier
func (d *DevPortal) GetUIUrl() string {
	return fmt.Sprintf("%s/%s/views/default/apis", d.APIUrl, d.Identifier)
}

// Validate performs basic validation of DevPortal fields
func (d *DevPortal) Validate() error {
	if d.Name == "" {
		return constants.ErrDevPortalNameRequired
	}
	if d.Identifier == "" {
		return constants.ErrDevPortalIdentifierRequired
	}
	if d.APIUrl == "" {
		return constants.ErrDevPortalAPIUrlRequired
	}
	if d.Hostname == "" {
		return constants.ErrDevPortalHostnameRequired
	}
	if d.APIKey == "" {
		return constants.ErrDevPortalAPIKeyRequired
	}
	if d.HeaderKeyName == "" {
		return constants.ErrDevPortalHeaderKeyNameRequired
	}
	if d.Visibility != "public" && d.Visibility != "private" {
		return constants.ErrDevPortalInvalidVisibility
	}
	return nil
}

// APIDevPortalWithDetails represents a DevPortal with its association and publication details for an API
type APIDevPortalWithDetails struct {
	// DevPortal information
	UUID             string    `json:"uuid" db:"uuid"`
	OrganizationUUID string    `json:"organizationUuid" db:"organization_uuid"`
	Name             string    `json:"name" db:"name"`
	Identifier       string    `json:"identifier" db:"identifier"`
	APIUrl           string    `json:"apiUrl" db:"api_url"`
	Hostname         string    `json:"hostname" db:"hostname"`
	IsActive         bool      `json:"isActive" db:"is_active"`
	IsEnabled        bool      `json:"isEnabled" db:"is_enabled"`
	IsDefault        bool      `json:"isDefault" db:"is_default"`
	Visibility       string    `json:"visibility" db:"visibility"`
	Description      string    `json:"description" db:"description"`
	CreatedAt        time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt        time.Time `json:"updatedAt" db:"updated_at"`

	// Association information (from api_associations table)
	AssociatedAt         time.Time `json:"associatedAt" db:"associated_at"`
	AssociationUpdatedAt time.Time `json:"associationUpdatedAt" db:"association_updated_at"`

	// Publication information (nullable if not published - from api_publications table)
	IsPublished           bool       `json:"isPublished" db:"is_published"`
	PublicationStatus     *string    `json:"publicationStatus,omitempty" db:"publication_status"`
	APIVersion            *string    `json:"apiVersion,omitempty" db:"api_version"`
	DevPortalRefID        *string    `json:"devPortalRefId,omitempty" db:"devportal_ref_id"`
	SandboxEndpointURL    *string    `json:"sandboxEndpointUrl,omitempty" db:"sandbox_endpoint_url"`
	ProductionEndpointURL *string    `json:"productionEndpointUrl,omitempty" db:"production_endpoint_url"`
	PublishedAt           *time.Time `json:"publishedAt,omitempty" db:"published_at"`
	PublicationUpdatedAt  *time.Time `json:"publicationUpdatedAt,omitempty" db:"publication_updated_at"`
}
