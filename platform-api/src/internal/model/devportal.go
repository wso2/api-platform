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
	IsEnabled		 bool      `json:"isEnabled" db:"is_enabled"`
	APIKey           string    `json:"apiKey" db:"api_key"`
	HeaderKeyName    string    `json:"headerKeyName" db:"header_key_name"`
	IsDefault        bool      `json:"isDefault" db:"is_default"`
	Visibility       string    `json:"visibility" db:"visibility"`
	Description      string    `json:"description" db:"description"`
	CreatedAt        time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt        time.Time `json:"updatedAt" db:"updated_at"`
}

// GetAuthHeader returns the authentication header name and value
// Always returns header authentication since we use header mode exclusively
func (d *DevPortal) GetAuthHeader() (string, string) {
	return d.HeaderKeyName, d.APIKey
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

// CanBeActivated checks if the DevPortal can be activated
// Business rule: DevPortal must have valid configuration and not already be active
func (d *DevPortal) CanBeActivated() error {
	if d.IsActive {
		return fmt.Errorf("DevPortal %s is already active", d.Name)
	}

	if err := d.Validate(); err != nil {
		return fmt.Errorf("DevPortal %s cannot be activated due to validation errors: %w", d.Name, err)
	}

	return nil
}

// CanBeDeactivated checks if the DevPortal can be deactivated
// Business rule: Default DevPortals cannot be deactivated
func (d *DevPortal) CanBeDeactivated() error {
	if d.IsDefault {
		return constants.ErrDevPortalCannotDeactivateDefault
	}

	if !d.IsActive {
		return fmt.Errorf("DevPortal %s is not active", d.Name)
	}

	return nil
}

// RequiresSync checks if the DevPortal requires synchronization
// Business rule: Active DevPortals should be synchronized
func (d *DevPortal) RequiresSync() bool {
	return d.IsActive
}

// IsReadyForPublishing checks if the DevPortal is ready for API publishing
// Business rule: DevPortal must be active and properly configured
func (d *DevPortal) IsReadyForPublishing() error {
	if !d.IsActive {
		return fmt.Errorf("DevPortal %s is not active", d.Name)
	}

	if err := d.Validate(); err != nil {
		return fmt.Errorf("DevPortal %s is not properly configured for publishing: %w", d.Name, err)
	}

	return nil
}
