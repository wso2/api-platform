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

// APIPortalWorkflowStatusActive is the only api_portals.workflow_status value
// a fully-registered portal carries. "pending"/"failed" rows exist in the
// table (registration in progress or failed) but are excluded from the
// GET /api-publications rollup (Slice 3) — everything else in this feature
// only needs the portal to exist, per REST_Design.md.
const APIPortalWorkflowStatusActive = "active"

// PublicationAPIPortal is this feature's own deliberately minimal projection
// of api_portals — not a temporary stand-in, but a permanent data-minimization
// boundary: registration/auth details (auth_type, auth_configuration,
// metadata) are never modelled here, so they can never leak into a read this
// feature serves (e.g. the GET /api-publications rollup, Slice 3), regardless
// of how much the real api_portals row eventually carries. Named after this
// feature (the same convention as Publication/PublicationContent/
// PublicationSummary in publication.go), not "APIPortal", so it also can't
// collide with that team's own model once their real implementation lands in
// this same package (see Implementation_Plan.md's "When the real dependencies
// land" — api_portals is currently a stub table there, but this struct's
// narrowness is independent of that and survives it).
type PublicationAPIPortal struct {
	UUID             string
	OrganizationUUID string
	Handle           string
	DisplayName      string
	Description      string
	URL              string
	WorkflowStatus   string

	// CreatedAt is the portal's own registration time — used only as the
	// GET /api-publications rollup's "createdAt" sort key (Slice 3), not
	// otherwise read by this feature.
	CreatedAt time.Time
}
