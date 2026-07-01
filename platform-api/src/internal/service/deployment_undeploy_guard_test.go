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
	"database/sql"
	"errors"
	"testing"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
)

// guardStubArtifactRepo is a minimal ArtifactRepository whose GetByUUID returns a
// fixed artifact, used to exercise the DP-origin undeploy guard.
type guardStubArtifactRepo struct{ art *model.Artifact }

func (g *guardStubArtifactRepo) Create(tx *sql.Tx, a *model.Artifact) error        { return nil }
func (g *guardStubArtifactRepo) Delete(tx *sql.Tx, uuid string) error              { return nil }
func (g *guardStubArtifactRepo) Update(tx *sql.Tx, a *model.Artifact) error        { return nil }
func (g *guardStubArtifactRepo) Exists(kind, handle, orgUUID string) (bool, error) { return false, nil }
func (g *guardStubArtifactRepo) GetByHandle(h, o string) (*model.Artifact, error)  { return g.art, nil }
func (g *guardStubArtifactRepo) GetByUUID(u, o string) (*model.Artifact, error)    { return g.art, nil }
func (g *guardStubArtifactRepo) CountByKindAndOrg(k, o string) (int, error)        { return 0, nil }
func (g *guardStubArtifactRepo) ExistsByUUIDs(u []string, o string) ([]string, error) {
	return nil, nil
}
func (g *guardStubArtifactRepo) GetAPIMetadataByHandle(h, o string) (*model.APIMetadata, error) {
	return nil, nil
}
func (g *guardStubArtifactRepo) GetAPIMetadataByHandleAndKind(h, k, o string) (*model.APIMetadata, error) {
	return nil, nil
}
func (g *guardStubArtifactRepo) GetMetadataByUUIDs(u []string, o string) (map[string]*model.APIMetadata, error) {
	return map[string]*model.APIMetadata{}, nil
}

// TestUndeployDeployment_BlockedForDPOrigin verifies the control plane refuses to
// undeploy a data-plane-originated (origin=DP) artifact: its deploy/undeploy lifecycle
// is owned by the gateway. The guard runs before any deployment lookup.
func TestUndeployDeployment_BlockedForDPOrigin(t *testing.T) {
	svc := &DeploymentService{
		artifactRepo: &guardStubArtifactRepo{
			art: &model.Artifact{UUID: "artifact-1", Origin: constants.OriginDP},
		},
	}

	_, err := svc.UndeployDeployment("artifact-1", "deployment-1", "gateway-1", "org-1", "tester")
	if !errors.Is(err, constants.ErrArtifactReadOnly) {
		t.Fatalf("UndeployDeployment(DP origin) error = %v, want ErrArtifactReadOnly", err)
	}
}

// TestRestoreDeployment_BlockedForDPOrigin verifies the control plane refuses to restore
// a deployment of a data-plane-originated artifact.
func TestRestoreDeployment_BlockedForDPOrigin(t *testing.T) {
	svc := &DeploymentService{
		artifactRepo: &guardStubArtifactRepo{
			art: &model.Artifact{UUID: "artifact-1", Origin: constants.OriginDP},
		},
	}

	_, err := svc.RestoreDeployment("artifact-1", "deployment-1", "gateway-1", "org-1", "tester")
	if !errors.Is(err, constants.ErrArtifactReadOnly) {
		t.Fatalf("RestoreDeployment(DP origin) error = %v, want ErrArtifactReadOnly", err)
	}
}
