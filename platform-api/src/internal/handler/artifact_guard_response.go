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
	"errors"
	"net/http"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/utils"

	"github.com/wso2/go-httpkit/httputil"
)

// respondArtifactGuardError writes the appropriate HTTP response for read-only /
// deletion-guard errors raised when a mutating operation targets a
// data-plane-originated (origin=DP) artifact. It returns true when it handled the
// error (and wrote a response), so callers can simply `return`.
//
//   - ErrArtifactReadOnly        -> 403 Forbidden (update/deploy of a DP artifact)
//   - ErrArtifactRuntimeImmutable -> 403 Forbidden (edit that would change a DP artifact's runtime config)
//   - ErrArtifactDeployed        -> 409 Conflict  (delete of a still-deployed DP artifact)
func respondArtifactGuardError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, constants.ErrArtifactReadOnly):
		httputil.WriteJSON(w, http.StatusForbidden, utils.NewErrorResponse(403, "Forbidden", err.Error()))
		return true
	case errors.Is(err, constants.ErrArtifactRuntimeImmutable):
		httputil.WriteJSON(w, http.StatusForbidden, utils.NewErrorResponse(403, "Forbidden", err.Error()))
		return true
	case errors.Is(err, constants.ErrArtifactDeployed):
		httputil.WriteJSON(w, http.StatusConflict, utils.NewErrorResponse(409, "Conflict", err.Error()))
		return true
	default:
		return false
	}
}
