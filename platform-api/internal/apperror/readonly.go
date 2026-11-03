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

// TEMP-READ-ONLY-MODE: this whole file is part of the temporary organization-scoped
// read-only mode used while Bijira migrates from Platform API v1 to v2. Delete it
// when the mode is removed — see README.md "Read-only (maintenance) mode — temporary"
// for the removal checklist.

package apperror

import "net/http"

// CodeCommonOrganizationReadOnly is returned (with HTTP 503) for a write request
// on an organization that is read-only under the platform's temporary read-only
// (maintenance) mode — see config.ReadOnly.
const CodeCommonOrganizationReadOnly = "ORGANIZATION_READ_ONLY"

// OrganizationReadOnly is deliberately a 503, not a 403 or 409: the condition is
// temporary and operator-driven (config.ReadOnly), a client should retry once
// maintenance ends, and the gateway-controller treats 401/403/404/409/422 from
// the control plane as permanent failures and exits, but retries 503. It is
// registered through def() like every catalog entry, so catalog_test.go holds
// it to the same integrity checks.
var OrganizationReadOnly = def(CodeCommonOrganizationReadOnly, http.StatusServiceUnavailable,
	"The organization is in read-only mode: write operations are temporarily disabled for maintenance.")
