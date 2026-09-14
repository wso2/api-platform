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
	"log/slog"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

const (
	kindTestOrgUUID = "bbbbbbbb-0000-0000-0000-000000000001"
	kindTestUUID    = "bbbbbbbb-0000-0000-0000-000000000002"
)

// kindTestArtifactRepo reports one artifact of whatever kind the test sets.
type kindTestArtifactRepo struct {
	repository.ArtifactRepository
	kind    string
	missing bool
}

func (m *kindTestArtifactRepo) GetByUUID(uuid, orgUUID string) (*model.Artifact, error) {
	if m.missing {
		return nil, nil
	}
	return &model.Artifact{UUID: uuid, Type: m.kind, OrganizationUUID: orgUUID, Handle: "thing"}, nil
}

// fakeDefinition stands in for one kind's renderer and records being used.
type fakeDefinition struct {
	kind    string
	yaml    string
	origin  string
	called  int
	failErr error
}

func (d *fakeDefinition) Kind() string { return d.kind }

func (d *fakeDefinition) Current(artifact *model.Artifact) (*ArtifactSnapshot, error) {
	d.called++
	if d.failErr != nil {
		return nil, d.failErr
	}
	return &ArtifactSnapshot{Definition: d.yaml, DataVersion: "1.0", Origin: d.origin}, nil
}

func (d *fakeDefinition) Decode(content []byte) (any, error) { return string(content), nil }

func newKindTestBuildService(t *testing.T, kind string, depRepo repository.DeploymentRepository,
	definitions ...ArtifactDefinition) *BuildService {
	t.Helper()
	return NewBuildService(
		&kindTestArtifactRepo{kind: kind},
		depRepo,
		NewArtifactDefinitions(definitions...),
		&testConfig,
		slog.Default(),
	)
}

// The point of the shared store: which renderer runs is decided by the kind on the
// artifact row, so an MCP proxy is snapshotted by MCP's renderer and a REST API by
// REST's, through one code path.
func TestBuildService_RendersWithTheDefinitionForTheArtifactsKind(t *testing.T) {
	mcp := &fakeDefinition{kind: "Mcp", yaml: "kind: McpProxy"}
	rest := &fakeDefinition{kind: "RestApi", yaml: "kind: RestApi"}
	depRepo := &buildTestDeploymentRepo{}
	service := newKindTestBuildService(t, "Mcp", depRepo, mcp, rest)

	if _, err := service.Create(kindTestUUID, kindTestOrgUUID, "tester", "", nil); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if mcp.called != 1 {
		t.Errorf("the MCP renderer ran %d times, want 1", mcp.called)
	}
	if rest.called != 0 {
		t.Errorf("the REST renderer ran for an MCP artifact")
	}
	if got := string(depRepo.createdBuild.Content); !strings.Contains(got, "McpProxy") {
		t.Errorf("stored content = %q, want what the MCP renderer produced", got)
	}
}

// A kind with no renderer registered is a wiring mistake, not something a caller
// did, so it must not surface as a not-found or a bad request.
func TestBuildService_UnregisteredKindIsInternal(t *testing.T) {
	service := newKindTestBuildService(t, "Mcp", &buildTestDeploymentRepo{},
		&fakeDefinition{kind: "RestApi"})

	_, err := service.Create(kindTestUUID, kindTestOrgUUID, "tester", "", nil)
	if err == nil {
		t.Fatal("expected an error for a kind with no definition")
	}
	if apperror.ArtifactNotFound.Is(err) {
		t.Error("an unregistered kind was reported as a missing artifact")
	}
}

// A missing artifact is a not-found, whatever the kind would have been.
func TestBuildService_MissingArtifactIsNotFound(t *testing.T) {
	service := NewBuildService(
		&kindTestArtifactRepo{kind: "Mcp", missing: true},
		&buildTestDeploymentRepo{},
		NewArtifactDefinitions(&fakeDefinition{kind: "Mcp"}),
		&testConfig,
		slog.Default(),
	)

	if _, err := service.Create(kindTestUUID, kindTestOrgUUID, "tester", "", nil); !apperror.ArtifactNotFound.Is(err) {
		t.Fatalf("error = %v, want ArtifactNotFound", err)
	}
}

// A DP-originated artifact is read-only in the control plane, so there is nothing
// to snapshot — and that guard has to hold for every kind, not just REST.
func TestBuildService_RefusesADataPlaneArtifactForAnyKind(t *testing.T) {
	depRepo := &buildTestDeploymentRepo{}
	service := newKindTestBuildService(t, "Mcp", depRepo,
		&fakeDefinition{kind: "Mcp", yaml: "kind: McpProxy", origin: constants.OriginDP})

	if _, err := service.Create(kindTestUUID, kindTestOrgUUID, "tester", "", nil); err == nil {
		t.Fatal("expected a data-plane artifact to be refused")
	}
	if depRepo.createdBuild != nil {
		t.Error("a build was stored for a data-plane artifact")
	}
}
