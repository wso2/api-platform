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

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
)

// mapArtifactGuardError maps read-only / deletion-guard errors raised when a
// mutating operation targets a data-plane-originated (origin=DP) artifact to
// the corresponding *apperror.Error, for the MapErrors middleware to log and
// serialize. It returns nil when err is not a guard error, so callers can
// `if guardErr := mapArtifactGuardError(err); guardErr != nil { return guardErr }`.
//
//   - ErrArtifactReadOnly        -> 403 Forbidden (update/deploy of a DP artifact)
//   - ErrArtifactRuntimeImmutable -> 403 Forbidden (edit that would change a DP artifact's runtime config)
//   - ErrArtifactDeployed        -> 409 Conflict  (delete of a still-deployed DP artifact)
func mapArtifactGuardError(err error) error {
	switch {
	case errors.Is(err, constants.ErrArtifactReadOnly):
		return apperror.ArtifactReadOnly.Wrap(err, "Artifact is read-only: it originated from a data-plane gateway")
	case errors.Is(err, constants.ErrArtifactRuntimeImmutable):
		return apperror.ArtifactRuntimeImmutable.Wrap(err, "Runtime configuration of this artifact cannot be changed")
	case errors.Is(err, constants.ErrArtifactDeployed):
		return apperror.ArtifactDeployed.Wrap(err, "Artifact is still deployed on a gateway and cannot be deleted")
	default:
		return nil
	}
}
