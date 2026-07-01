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
	"bytes"
	"fmt"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/repository"

	"gopkg.in/yaml.v3"
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

// compareRuntimeArtifacts fingerprints two already-generated runtime artifacts and
// defers to ensureRuntimeArtifactUnchanged. It is the common tail shared by the
// per-kind guards once each has produced the stored and proposed artifact structs.
// Control-plane artifacts are always mutable, so it returns immediately for any
// non-DP origin and only fingerprints when the origin is DP.
func compareRuntimeArtifacts(origin string, existing, proposed any) error {
	if origin != constants.OriginDP {
		return nil
	}
	existingFP, err := runtimeArtifactFingerprint(existing)
	if err != nil {
		return err
	}
	proposedFP, err := runtimeArtifactFingerprint(proposed)
	if err != nil {
		return err
	}
	return ensureRuntimeArtifactUnchanged(origin, existingFP, proposedFP)
}

// runtimeArtifactFingerprint serializes a generated gateway runtime artifact (any of
// the per-kind deployment-YAML structs) into a canonical, comparable byte form. Two
// artifacts with equal fingerprints deploy to the gateway identically. Callers build
// the fingerprint for the currently-stored model and the would-be-updated model with
// the SAME generator (the one used at deploy time) so defaulting/normalization is
// symmetric and only a genuine runtime change surfaces as a difference.
func runtimeArtifactFingerprint(artifact any) ([]byte, error) {
	b, err := yaml.Marshal(artifact)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize runtime artifact for read-only check: %w", err)
	}
	return b, nil
}

// ensureRuntimeArtifactUnchanged permits an update to a DP-originated (origin
// "gateway_api") artifact only when it does not change the gateway runtime artifact.
// Control-plane artifacts are always mutable, so it is a no-op for them. For DP
// artifacts, the two fingerprints are the runtime artifact the stored model produces
// (existing) and the one the updated model would produce (proposed); when they differ
// the update is rejected with ErrArtifactRuntimeImmutable. This lets metadata-only
// edits (description, lifecycle status, display data, docs) through while keeping the
// gateway-owned runtime configuration read-only in the control plane.
func ensureRuntimeArtifactUnchanged(origin string, existing, proposed []byte) error {
	if origin != constants.OriginDP {
		return nil
	}
	if bytes.Equal(existing, proposed) {
		return nil
	}
	return constants.ErrArtifactRuntimeImmutable
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
