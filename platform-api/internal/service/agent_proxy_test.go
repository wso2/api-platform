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

// Service-layer rules the HTTP path cannot reach on its own: a protocol change
// (no second protocol is registered, so the request decoder rejects one before
// the service ever sees it), the read-only guard on a gateway-originated Agent
// proxy, and the project-deletion guard.

package service

import (
	"log/slog"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

const (
	agentTestOrg     = "org-agent-unit"
	agentTestProject = "550e8400-e29b-41d4-a716-446655440000"
)

// mockAgentProxyRepository is a minimal stand-in for the Agent proxy
// repository. Only the reads the tests below reach are implemented; anything
// else panics through the embedded nil interface rather than silently
// succeeding.
type mockAgentProxyRepository struct {
	repository.AgentProxyRepository
	stored          *model.AgentProxy
	countByProject  int
	deleteCallCount int
}

func (m *mockAgentProxyRepository) GetByHandle(handle, orgUUID string) (*model.AgentProxy, error) {
	return m.stored, nil
}

func (m *mockAgentProxyRepository) CountByProject(orgUUID, projectUUID string) (int, error) {
	return m.countByProject, nil
}

func (m *mockAgentProxyRepository) Delete(handle, orgUUID string) error {
	m.deleteCallCount++
	return nil
}

func newAgentProxyTestService(repo repository.AgentProxyRepository) *AgentProxyService {
	projectRepo := &mockProjectRepo{project: &model.Project{
		ID: agentTestProject, Handle: "default-project", OrganizationID: agentTestOrg,
	}}
	return NewAgentProxyService(repo, projectRepo, nil, nil, nil,
		slog.Default(), &noopAuditRepo{}, &config.Server{}, newTestIdentityService())
}

// storedAgentProxy is an A2A Agent proxy as the repository would return it.
func storedAgentProxy(origin string) *model.AgentProxy {
	return &model.AgentProxy{
		UUID:             "agent-uuid-1",
		Handle:           "weather-agent",
		OrganizationUUID: agentTestOrg,
		ProjectUUID:      agentTestProject,
		Name:             "Weather Agent",
		Version:          "v1.0",
		Protocol:         model.AgentProxyProtocolA2A,
		Origin:           origin,
		Configuration: model.AgentProxyConfiguration{
			Upstream: model.UpstreamConfig{Main: &model.UpstreamEndpoint{URL: "http://weather-agent:9000"}},
			A2A: &model.A2AProtocolConfig{
				ProtocolVersion: "1.0",
				OperationConfigs: model.A2AOperationConfigs{
					Transports: []model.A2ATransport{{ProtocolBinding: "JSONRPC"}},
				},
			},
		},
	}
}

// validAgentProxyRequest is an otherwise-valid replacement body, so each test
// below isolates the single rule it is about.
func validAgentProxyRequest() *api.A2AAgentProxy {
	url := "http://weather-agent:9000"
	return &api.A2AAgentProxy{
		DisplayName: "Weather Agent",
		Version:     "v1.0",
		ProjectId:   "default-project",
		Protocol:    api.A2AAgentProxyProtocolA2a,
		Upstream:    api.Upstream{Main: api.UpstreamDefinition{Url: &url}},
		A2a: api.A2AProtocolConfig{
			ProtocolVersion: "1.0",
			OperationConfigs: api.A2AOperationConfigs{
				Transports: []api.A2ATransport{{ProtocolBinding: "JSONRPC"}},
			},
		},
	}
}

// TestAgentProxyServiceUpdateRejectsProtocolChange pins the comparison to the
// persisted column. Nothing in the request or in the stored configuration
// document is consulted for it — the document carries no discriminator at all.
func TestAgentProxyServiceUpdateRejectsProtocolChange(t *testing.T) {
	svc := newAgentProxyTestService(&mockAgentProxyRepository{stored: storedAgentProxy(constants.OriginCP)})

	req := validAgentProxyRequest()
	req.Protocol = "mcp"

	_, err := svc.Update(agentTestOrg, "weather-agent", "alice", req)
	if !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected a validation failure, got: %v", err)
	}
}

func TestAgentProxyServiceRejectsUnsupportedProtocolOnCreate(t *testing.T) {
	svc := newAgentProxyTestService(&mockAgentProxyRepository{})

	req := validAgentProxyRequest()
	req.Protocol = "grpc"

	if _, err := svc.Create(agentTestOrg, "alice", req); !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected a validation failure, got: %v", err)
	}
}

// TestAgentProxyServiceUpdateRejectsReadOnlyArtifact covers the origin guard: a
// gateway-originated Agent proxy is owned by its data plane and is not editable
// here. The 403 comes from the stored origin, never from a caller-supplied
// readOnly value.
func TestAgentProxyServiceUpdateRejectsReadOnlyArtifact(t *testing.T) {
	svc := newAgentProxyTestService(&mockAgentProxyRepository{stored: storedAgentProxy(constants.OriginDP)})

	readOnly := false
	req := validAgentProxyRequest()
	req.ReadOnly = &readOnly // ignored: the request does not get to declare this

	_, err := svc.Update(agentTestOrg, "weather-agent", "alice", req)
	if !apperror.ArtifactReadOnly.Is(err) {
		t.Fatalf("expected ArtifactReadOnly, got: %v", err)
	}
}

// TestAgentProxyServiceGetReturnsProjectHandle guards the response contract:
// projectId is the project handle, never the stored project UUID, which a
// client has no way to resolve back.
func TestAgentProxyServiceGetReturnsProjectHandle(t *testing.T) {
	svc := newAgentProxyTestService(&mockAgentProxyRepository{stored: storedAgentProxy(constants.OriginCP)})

	resp, err := svc.Get(agentTestOrg, "weather-agent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if resp.ProjectId != "default-project" {
		t.Fatalf("projectId = %q, want the project handle", resp.ProjectId)
	}
	if resp.Protocol != api.A2AAgentProxyProtocol(model.AgentProxyProtocolA2A) {
		t.Fatalf("protocol = %q; it must come from the column", resp.Protocol)
	}
}

func TestAgentProxyServiceGetMissingIsNotFound(t *testing.T) {
	svc := newAgentProxyTestService(&mockAgentProxyRepository{})

	if _, err := svc.Get(agentTestOrg, "no-such-agent"); !apperror.AgentProxyNotFound.Is(err) {
		t.Fatalf("expected AgentProxyNotFound, got: %v", err)
	}
}

// TestProjectDeletionIsRefusedWhileAgentProxiesRemain guards the project
// deletion path. agent_proxies.project_uuid cascades in the database, so
// without this check deleting a project would silently take its Agent proxies
// with it.
func TestProjectDeletionIsRefusedWhileAgentProxiesRemain(t *testing.T) {
	agentRepo := &mockAgentProxyRepository{countByProject: 1}
	projectRepo := &mockProjectDeletionRepo{
		projects: []*model.Project{
			{ID: agentTestProject, Handle: "default-project", OrganizationID: agentTestOrg},
			{ID: "project-2", Handle: "second-project", OrganizationID: agentTestOrg},
		},
	}
	svc := NewProjectService(projectRepo, nil, &emptyAPIRepo{}, &mockAgentProjectMCPRepo{},
		agentRepo, &emptyApplicationRepo{}, &noopAuditRepo{}, newTestIdentityService(), slog.Default())

	err := svc.DeleteProject("default-project", agentTestOrg, "alice")
	if !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected a validation failure while Agent proxies remain, got: %v", err)
	}

	// With none left the guard no longer applies, so the same call proceeds.
	agentRepo.countByProject = 0
	if err := svc.DeleteProject("default-project", agentTestOrg, "alice"); err != nil {
		t.Fatalf("delete with no Agent proxies remaining: %v", err)
	}
}

// --- minimal repositories for the project-deletion guard --------------------

type mockProjectDeletionRepo struct {
	repository.ProjectRepository
	projects []*model.Project
}

func (m *mockProjectDeletionRepo) GetProjectByHandleAndOrgID(handle, orgID string) (*model.Project, error) {
	for _, p := range m.projects {
		if p.Handle == handle && p.OrganizationID == orgID {
			return p, nil
		}
	}
	return nil, nil
}

func (m *mockProjectDeletionRepo) GetProjectsByOrganizationID(orgID string) ([]*model.Project, error) {
	return m.projects, nil
}

func (m *mockProjectDeletionRepo) DeleteProject(projectID string) error { return nil }

type emptyAPIRepo struct{ repository.APIRepository }

func (emptyAPIRepo) GetAPIsByProjectUUID(projectUUID, orgID string) ([]*model.API, error) {
	return nil, nil
}

type mockAgentProjectMCPRepo struct{ repository.MCPProxyRepository }

func (mockAgentProjectMCPRepo) CountByProject(orgUUID, projectUUID string) (int, error) {
	return 0, nil
}

type emptyApplicationRepo struct {
	repository.ApplicationRepository
}

func (emptyApplicationRepo) CountApplicationsByProjectID(projectID, orgID, search string) (int, error) {
	return 0, nil
}
