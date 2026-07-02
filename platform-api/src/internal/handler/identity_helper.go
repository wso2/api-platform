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

package handler

import (
	"log/slog"
	"net/http"

	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/wso2/go-httpkit/httputil"
)

// resolveActor resolves the internal platform UUID for the actor behind r via
// identity, for use in audit columns (created_by/updated_by/revoked_by/
// performed_by). Identity is always resolved from the token claim (sub,
// falling back to the configured claim / user_id) — a missing identity mints
// an anonymous UUID rather than failing (see IdentityService.InternalUserID),
// so this only fails (500) on a genuine DB error.
func resolveActor(w http.ResponseWriter, r *http.Request, identity *service.IdentityService, slogger *slog.Logger, action string) (actor string, ok bool) {
	actor, err := identity.InternalUserID(r)
	if err != nil {
		slogger.Error("Failed to resolve user identity", "action", action, "error", err)
		httputil.WriteJSON(w, http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to resolve user identity"))
		return "", false
	}
	return actor, true
}
