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
	"net/http"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

// resolveActorErr resolves the internal platform UUID for the actor behind r
// via identity, for use in audit columns (created_by/updated_by/revoked_by/
// performed_by). Identity is always resolved from the token claim (sub,
// falling back to the configured claim / user_id) — a missing identity mints
// an anonymous UUID rather than failing (see IdentityService.InternalUserID),
// so this only fails (500) on a genuine DB error. Returns an *apperror.Error
// for the mapper to log and serialize.
func resolveActorErr(r *http.Request, identity *service.IdentityService, action string) (string, error) {
	actor, err := identity.InternalUserID(r)
	if err != nil {
		return "", apperror.Internal.Wrap(err).
			WithLogMessage("failed to resolve user identity for action: " + action)
	}
	return actor, nil
}
