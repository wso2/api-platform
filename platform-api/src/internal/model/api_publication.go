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
	"time"
)

// PublicationStatus represents the status of an API publication to a DevPortal
type PublicationStatus string

const (
	PublishedStatus  PublicationStatus = "published"
	FailedStatus     PublicationStatus = "failed"
	PublishingStatus PublicationStatus = "publishing"
)

// APIPublication represents the current publication status of an API to a DevPortal
// Note: This table uses a composite primary key (api_uuid, devportal_uuid, organization_uuid)
type APIPublication struct {
	APIUUID          string `json:"apiUuid" db:"api_uuid"`
	DevPortalUUID    string `json:"devPortalUuid" db:"devportal_uuid"`
	OrganizationUUID string `json:"organizationUuid" db:"organization_uuid"`
	// Publication status and basic metadata
	Status         PublicationStatus `json:"status" db:"status"`
	APIVersion     *string           `json:"apiVersion,omitempty" db:"api_version"`
	DevPortalRefID *string           `json:"devPortalRefId,omitempty" db:"devportal_ref_id"`

	// Gateway endpoints for sandbox and production
	SandboxEndpointURL    string `json:"sandboxEndpointUrl" db:"sandbox_endpoint_url"`
	ProductionEndpointURL string `json:"productionEndpointUrl" db:"production_endpoint_url"`

	// Timestamps
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt time.Time `json:"updatedAt" db:"updated_at"`
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// Validate validates the APIPublication model
func (ap *APIPublication) Validate() error {
	if ap.APIUUID == "" {
		return &ValidationError{
			Field:   "apiUuid",
			Message: "API UUID is required",
		}
	}

	if ap.DevPortalUUID == "" {
		return &ValidationError{
			Field:   "devPortalUuid",
			Message: "DevPortal UUID is required",
		}
	}

	if ap.OrganizationUUID == "" {
		return &ValidationError{
			Field:   "organizationUuid",
			Message: "Organization UUID is required",
		}
	}

	if ap.SandboxEndpointURL == "" {
		return &ValidationError{
			Field:   "sandboxEndpointUrl",
			Message: "Sandbox endpoint URL is required",
		}
	}

	if ap.ProductionEndpointURL == "" {
		return &ValidationError{
			Field:   "productionEndpointUrl",
			Message: "Production endpoint URL is required",
		}
	}

	return nil
}
