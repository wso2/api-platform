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

// APIPortal is the minimal projection of api_portals this feature reads.
// api_portals is a stub table (see schema.*.sql) owned by another team —
// auth_type/auth_configuration/metadata are registration details this
// feature never touches and so are not modelled here.
type APIPortal struct {
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
