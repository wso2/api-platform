/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package handler

import (
	"errors"
	"net/http"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"

	"github.com/wso2/go-httpkit/httputil"
)

// respondArtifactGuardError writes the appropriate HTTP response for read-only /
// deletion-guard errors raised when a mutating operation targets a
// data-plane-originated (origin=DP) artifact. It returns true when it handled the
// error (and wrote a response), so callers can simply `return`.
//
//   - ErrArtifactReadOnly  -> 403 Forbidden (update/deploy of a DP artifact)
//   - ErrArtifactDeployed  -> 409 Conflict  (delete of a still-deployed DP artifact)
func respondArtifactGuardError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, constants.ErrArtifactReadOnly):
		httputil.WriteJSON(w, http.StatusForbidden, apperror.NewErrorResponse(403, "Forbidden", err.Error()))
		return true
	case errors.Is(err, constants.ErrArtifactDeployed):
		httputil.WriteJSON(w, http.StatusConflict, apperror.NewErrorResponse(409, "Conflict", err.Error()))
		return true
	default:
		return false
	}
}
