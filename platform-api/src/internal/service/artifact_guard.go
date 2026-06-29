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

package service

import (
	"fmt"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/repository"
)

// ensureArtifactMutableByUUID looks the artifact up by UUID and returns
// ErrArtifactReadOnly when it is data-plane-originated (origin "gateway_api"). DP artifacts
// are read-only in the control plane, so CP-initiated deployment-lifecycle changes
// (deploy/undeploy/restore/delete) must be rejected. A nil repository or an unknown
// artifact is treated as mutable (no-op), which keeps hand-built unit-test services
// that omit the repo working.
func ensureArtifactMutableByUUID(repo repository.ArtifactRepository, artifactUUID, orgID string) error {
	if repo == nil {
		return nil
	}
	artifact, err := repo.GetByUUID(artifactUUID, orgID)
	if err != nil {
		return fmt.Errorf("failed to look up artifact origin: %w", err)
	}
	if artifact == nil {
		return nil
	}
	return ensureOriginMutable(artifact.Origin)
}

// ensureOriginMutable returns ErrArtifactReadOnly when the artifact originated from
// a data-plane gateway (origin "gateway_api"). gateway_api artifacts are read-only in the control
// plane: their metadata cannot be updated and they cannot be (re)deployed from the
// CP. Documentation and OpenAPI/spec updates are handled by separate endpoints and
// do not call this guard.
func ensureOriginMutable(origin string) error {
	if origin == constants.OriginDP {
		return constants.ErrArtifactReadOnly
	}
	return nil
}

// ensureOriginDeletable enforces the deletion rule for DP-originated artifacts: they
// may be deleted from the control plane only once they are undeployed on every
// gateway they were deployed to. CP-originated artifacts are unaffected by this guard.
func ensureOriginDeletable(deploymentRepo repository.DeploymentRepository, origin, artifactUUID, orgID string) error {
	if origin != constants.OriginDP {
		return nil
	}
	if deploymentRepo == nil {
		return fmt.Errorf("deployment repository is not configured")
	}
	active, err := deploymentRepo.HasActiveDeployment(artifactUUID, orgID)
	if err != nil {
		return err
	}
	if active {
		return constants.ErrArtifactDeployed
	}
	return nil
}
