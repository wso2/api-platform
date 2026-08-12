/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

	"github.com/wso2/api-platform/platform-api/internal/constants"
)

// APIPortal represents an API Portal registered within an organization.
// The Configuration blob carries auth-type-specific fields:
//   - auth_type=local  : may hold key material references (local JWT minting).
//   - auth_type=oauth2 : holds STS token URL, client credentials, optional audience.
type APIPortal struct {
	ID             string                 `json:"id" db:"uuid"`
	OrganizationID string                 `json:"organizationId" db:"organization_uuid"`
	Handle         string                 `json:"handle" db:"handle"`
	Name           string                 `json:"name" db:"display_name"`
	Description    string                 `json:"description,omitempty" db:"description"`
	URL            string                 `json:"url,omitempty" db:"url"`
	WorkflowStatus string                 `json:"workflowStatus" db:"workflow_status"`
	AuthType       string                 `json:"authType" db:"auth_type"`
	Configuration  map[string]interface{} `json:"configuration,omitempty" db:"configuration"`
	DataVersion    string                 `json:"-" db:"data_version"`
	CreatedBy      string                 `json:"createdBy,omitempty" db:"created_by"`
	UpdatedBy      string                 `json:"updatedBy,omitempty" db:"updated_by"`
	CreatedAt      time.Time              `json:"createdAt" db:"created_at"`
	UpdatedAt      time.Time              `json:"updatedAt" db:"updated_at"`
}

// TableName returns the table name for the APIPortal model.
func (APIPortal) TableName() string {
	return "api_portals"
}

// IsPending returns true if the portal is still being provisioned or activated.
func (p *APIPortal) IsPending() bool {
	return p.WorkflowStatus == constants.APIPortalWorkflowStatusPending
}

// IsActive returns true if the portal is reachable and functional.
func (p *APIPortal) IsActive() bool {
	return p.WorkflowStatus == constants.APIPortalWorkflowStatusActive
}

// IsFailed returns true if provisioning or a subsequent health check has failed.
func (p *APIPortal) IsFailed() bool {
	return p.WorkflowStatus == constants.APIPortalWorkflowStatusFailed
}
