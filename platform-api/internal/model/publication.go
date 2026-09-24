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

import "time"

// PublicationContentType is the discriminator on api_publication_contents.type.
type PublicationContentType string

const (
	PublicationContentTypeDefinition PublicationContentType = "API_DEFINITION"
	PublicationContentTypeImage      PublicationContentType = "IMAGE"
	PublicationContentTypeMarketing  PublicationContentType = "MARKETING"
)

// Status of a live publication row.
const (
	PublicationStatusPublished  = "PUBLISHED"
	PublicationStatusDeprecated = "DEPRECATED"
)

// Publication is one row of api_publications — a draft (IsDraft true) or a live
// listing (IsDraft false), one (artifact, api portal) pairing per row. Content
// (definition/landing page/thumbnail) lives in PublicationContent rows;
// SubscriptionPlanIds/DocIds are handles resolved from the mapping tables, never
// the internal UUIDs those tables actually store.
type Publication struct {
	UUID                string   `json:"id" db:"uuid"`
	OrganizationUUID    string   `json:"-" db:"organization_uuid"`
	ArtifactUUID        string   `json:"-" db:"artifact_uuid"`
	APIPortalUUID       string   `json:"-" db:"api_portal_uuid"`
	IsDraft             bool     `json:"-" db:"is_draft"`
	Status              string   `json:"status,omitempty" db:"status"`
	DisplayName         string   `json:"displayName" db:"display_name"`
	Version             string   `json:"version" db:"version"`
	Description         string   `json:"description,omitempty" db:"description"`
	Tags                []string `json:"tags,omitempty"`
	Labels              []string `json:"labels,omitempty"`
	AgentVisibility     string   `json:"agentVisibility,omitempty" db:"agent_visibility"`
	ProductionURL       string   `json:"productionUrl,omitempty" db:"production_url"`
	SandboxURL          string   `json:"sandboxUrl,omitempty" db:"sandbox_url"`
	BusinessOwner       string   `json:"businessOwner,omitempty" db:"business_owner"`
	BusinessOwnerEmail  string   `json:"businessOwnerEmail,omitempty" db:"business_owner_email"`
	TechnicalOwner      string   `json:"technicalOwner,omitempty" db:"technical_owner"`
	TechnicalOwnerEmail string   `json:"technicalOwnerEmail,omitempty" db:"technical_owner_email"`

	// SubscriptionPlanIds and DocIds are handles (never the internal
	// subscription_plan_uuid/doc_uuid the mapping tables store), resolved by the
	// repository layer via SubscriptionPlanRepository/DocumentRepository.
	SubscriptionPlanIds []string `json:"subscriptionPlanIds"`
	DocIds              []string `json:"docIds"`

	HasThumbnail   bool `json:"hasThumbnail"`
	HasLandingPage bool `json:"hasLandingPage"`

	// APIPortalHandle/APIPortalName are populated only for a live publication
	// read (PublicationService.GetPublication) — the portal's own handle and
	// display name for the response's apiPortalId/apiPortalName fields. Left
	// empty for a draft, which is always read in the context of one portal
	// already named in the request path.
	APIPortalHandle string `json:"-"`
	APIPortalName   string `json:"-"`

	DataVersion string    `json:"-" db:"data_version"`
	CreatedBy   string    `json:"createdBy,omitempty" db:"created_by"`
	CreatedAt   time.Time `json:"createdAt" db:"created_at"`
	UpdatedBy   string    `json:"updatedBy,omitempty" db:"updated_by"`
	UpdatedAt   time.Time `json:"updatedAt" db:"updated_at"`
}

// PublicationStatusRow is one api_publications row for one artifact, reduced
// to just what the GET /api-publications rollup needs to annotate a portal:
// which tier the row belongs to, the live tier's status, and its own
// updated_at. A row is returned regardless of whether it's the draft or the
// live listing — the caller (PublicationService.ListPublicationSummary)
// splits by IsDraft.
type PublicationStatusRow struct {
	APIPortalUUID string
	IsDraft       bool
	Status        string // only meaningful when IsDraft is false
	UpdatedAt     time.Time
}

// PublicationSummary is one row of the GET /api-publications rollup — one
// active API Portal annotated with this API's publication status against
// it. Status is NOT_PUBLISHED/PUBLISHED/DEPRECATED: NOT_PUBLISHED is this
// view's own label for "no live row exists," covering both "never
// published" and "unpublished since" without distinguishing them —
// api_publications itself never stores that value.
type PublicationSummary struct {
	APIPortalHandle      string
	APIPortalName        string
	APIPortalDescription string
	APIPortalURL         string
	Status               string
	DraftUpdatedAt       *time.Time
	PublicationUpdatedAt *time.Time

	// APIPortalCreatedAt is the portal's own registration time, carried
	// through purely as the "createdAt" sort key — not part of the response.
	APIPortalCreatedAt time.Time
}

// PublicationContent is one row of api_publication_contents: the definition,
// landing page, or thumbnail attached to a draft or live Publication row.
type PublicationContent struct {
	UUID             string                 `db:"uuid"`
	OrganizationUUID string                 `db:"organization_uuid"`
	PublicationUUID  string                 `db:"publication_uuid"`
	Type             PublicationContentType `db:"type"`
	FileName         string                 `db:"file_name"`
	ContentType      string                 `db:"content_type"`
	Content          []byte                 `db:"content"`
	DataVersion      string                 `db:"data_version"`
	CreatedBy        string                 `db:"created_by"`
	CreatedAt        time.Time              `db:"created_at"`
	UpdatedBy        string                 `db:"updated_by"`
	UpdatedAt        time.Time              `db:"updated_at"`
}
