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

package handler

import (
	"net/http"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"

	"github.com/wso2/api-platform/httpkit/httputil"
)

// readOnlyMode is the one thing the handlers need from config.ReadOnly. It is
// declared here, not in the handlers' own files, so those files need no new
// import for their single marked field; config.ReadOnly satisfies it with a
// nil-safe method, so a never-wired (nil) field means "mode off".
type readOnlyMode interface {
	IsReadOnlyOrg(orgUUID string) bool
}

// SetReadOnly wires the read-only (maintenance) mode configuration into the
// handler. The gateway-token routes bypass the user-JWT auth chain, so the global
// middleware.ReadOnlyGuard cannot see their organization; the handlers that
// write consult this instead, once the gateway token has identified it.
func (h *GatewayInternalAPIHandler) SetReadOnly(ro *config.ReadOnly) {
	h.readOnly = ro
}

// rejectIfReadOnly writes a 503 and reports true when the authenticated
// gateway's organization is read-only. It runs right after authenticateRequest
// in every gateway-token handler that writes. The response uses the Gateway
// Internal API's own error shape. 503 is deliberate: the gateway-controller
// treats 401/403/404/409/422 as permanent failures and exits, but retries 503.
func (h *GatewayInternalAPIHandler) rejectIfReadOnly(w http.ResponseWriter, r *http.Request, orgID, gatewayID string) bool {
	if h.readOnly == nil || !h.readOnly.IsReadOnlyOrg(orgID) {
		return false
	}
	h.slogger.Warn("Rejected gateway write in read-only mode",
		"orgID", orgID, "gatewayID", gatewayID, "route", r.Pattern)
	appErr := apperror.OrganizationReadOnly.New()
	httputil.WriteJSON(w, appErr.HTTPStatus, internalErrorFromApp(appErr))
	return true
}
