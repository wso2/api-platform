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
	"time"
)

// PublicationStatus represents the status of an API publication to a DevPortal
type PublicationStatus string

const (
	PublishedStatus    PublicationStatus = "published"
	UnpublishedStatus  PublicationStatus = "unpublished"
	FailedStatus       PublicationStatus = "failed"
	PublishingStatus   PublicationStatus = "publishing"
	UnpublishingStatus PublicationStatus = "unpublishing"
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
	SandboxGatewayUUID    string `json:"sandboxGatewayUuid" db:"sandbox_gateway_uuid"`
	ProductionGatewayUUID string `json:"productionGatewayUuid" db:"production_gateway_uuid"`

	// Timestamps
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt time.Time `json:"updatedAt" db:"updated_at"`
}

// PublicationStateMachine manages valid state transitions for API publications
type PublicationStateMachine struct {
	currentStatus PublicationStatus
}

// NewPublicationStateMachine creates a new state machine with the given status
func NewPublicationStateMachine(status PublicationStatus) *PublicationStateMachine {
	return &PublicationStateMachine{currentStatus: status}
}

// CanTransitionTo checks if a transition to the new status is valid
func (sm *PublicationStateMachine) CanTransitionTo(newStatus PublicationStatus) bool {
	switch sm.currentStatus {
	case PublishingStatus:
		return newStatus == PublishedStatus || newStatus == FailedStatus
	case PublishedStatus:
		return newStatus == UnpublishingStatus
	case UnpublishingStatus:
		return newStatus == UnpublishedStatus || newStatus == FailedStatus
	case UnpublishedStatus:
		return newStatus == PublishingStatus
	case FailedStatus:
		return newStatus == PublishingStatus // Allow retry from failed state
	default:
		return false
	}
}

// TransitionTo changes the status if the transition is valid
func (sm *PublicationStateMachine) TransitionTo(newStatus PublicationStatus) error {
	if !sm.CanTransitionTo(newStatus) {
		return fmt.Errorf("invalid status transition from %s to %s", sm.currentStatus, newStatus)
	}
	sm.currentStatus = newStatus
	return nil
}

// GetCurrentStatus returns the current status
func (sm *PublicationStateMachine) GetCurrentStatus() PublicationStatus {
	return sm.currentStatus
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// TableName returns the table name for the APIPublication model
func (APIPublication) TableName() string {
	return "api_publications"
}

// IsPublished returns true if the API is currently published to the DevPortal
func (ap *APIPublication) IsPublished() bool {
	return ap.Status == PublishedStatus
}

// IsActive returns true if the publication is in an active state (published or publishing)
func (ap *APIPublication) IsActive() bool {
	return ap.Status == PublishedStatus || ap.Status == PublishingStatus
}

// IsPending returns true if the publication is in a pending state
func (ap *APIPublication) IsPending() bool {
	return ap.Status == PublishingStatus || ap.Status == UnpublishingStatus
}

// SetPublished marks the publication as published
func (ap *APIPublication) SetPublished(devPortalRefID string) error {
	sm := NewPublicationStateMachine(ap.Status)
	if err := sm.TransitionTo(PublishedStatus); err != nil {
		return err
	}
	ap.Status = PublishedStatus
	ap.DevPortalRefID = &devPortalRefID
	ap.UpdatedAt = time.Now()
	return nil
}

// SetUnpublished marks the publication as unpublished
func (ap *APIPublication) SetUnpublished() error {
	sm := NewPublicationStateMachine(ap.Status)
	if err := sm.TransitionTo(UnpublishedStatus); err != nil {
		return err
	}
	ap.Status = UnpublishedStatus
	ap.DevPortalRefID = nil
	ap.UpdatedAt = time.Now()
	return nil
}

// SetFailed marks the publication as failed
func (ap *APIPublication) SetFailed() error {
	sm := NewPublicationStateMachine(ap.Status)
	if err := sm.TransitionTo(FailedStatus); err != nil {
		return err
	}
	ap.Status = FailedStatus
	ap.UpdatedAt = time.Now()
	return nil
}

// SetPublishing marks the publication as in progress
func (ap *APIPublication) SetPublishing() error {
	sm := NewPublicationStateMachine(ap.Status)
	if err := sm.TransitionTo(PublishingStatus); err != nil {
		return err
	}
	ap.Status = PublishingStatus
	ap.UpdatedAt = time.Now()
	return nil
}

// SetUnpublishing marks the unpublication as in progress
func (ap *APIPublication) SetUnpublishing() error {
	sm := NewPublicationStateMachine(ap.Status)
	if err := sm.TransitionTo(UnpublishingStatus); err != nil {
		return err
	}
	ap.Status = UnpublishingStatus
	ap.UpdatedAt = time.Now()
	return nil
}

// ValidateStatus validates the publication status
func (ps PublicationStatus) Validate() error {
	switch ps {
	case PublishedStatus, UnpublishedStatus, FailedStatus, PublishingStatus, UnpublishingStatus:
		return nil
	default:
		return &ValidationError{
			Field:   "status",
			Message: "invalid publication status",
		}
	}
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

	if ap.SandboxGatewayUUID == "" {
		return &ValidationError{
			Field:   "sandboxGatewayUuid",
			Message: "Sandbox Gateway UUID is required",
		}
	}

	if ap.ProductionGatewayUUID == "" {
		return &ValidationError{
			Field:   "productionGatewayUuid",
			Message: "Production Gateway UUID is required",
		}
	}

	if err := ap.Status.Validate(); err != nil {
		return err
	}

	return nil
}
