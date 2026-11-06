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
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

const (
	artifactsTestOrg     = "org-artifacts-unit"
	artifactsTestProject = "6f1c2d3e-4a5b-4c6d-8e7f-9a0b1c2d3e4f"
)

// countCall is the organization and project a count was asked for.
type countCall struct{ org, project string }

type countingRESTRepo struct {
	repository.APIRepository
	n     int
	calls []countCall
}

func (r *countingRESTRepo) CountAPIsByOrganizationUUID(orgUUID, projectUUID, _ string) (int, error) {
	r.calls = append(r.calls, countCall{orgUUID, projectUUID})
	return r.n, nil
}

type countingGraphQLRepo struct {
	repository.GraphQLAPIRepository
	n     int
	err   error
	calls []countCall
}

func (r *countingGraphQLRepo) CountByProject(orgUUID, projectUUID, _ string) (int, error) {
	r.calls = append(r.calls, countCall{orgUUID, projectUUID})
	return r.n, r.err
}

type countingLLMProxyRepo struct {
	repository.LLMProxyRepository
	n     int
	calls []countCall
}

func (r *countingLLMProxyRepo) CountByProject(orgUUID, projectUUID string) (int, error) {
	r.calls = append(r.calls, countCall{orgUUID, projectUUID})
	return r.n, nil
}

type countingMCPProxyRepo struct {
	repository.MCPProxyRepository
	n     int
	calls []countCall
}

func (r *countingMCPProxyRepo) CountByProject(orgUUID, projectUUID string) (int, error) {
	r.calls = append(r.calls, countCall{orgUUID, projectUUID})
	return r.n, nil
}

type countingAgentProxyRepo struct {
	repository.AgentProxyRepository
	n     int
	calls []countCall
}

func (r *countingAgentProxyRepo) CountByProject(orgUUID, projectUUID string) (int, error) {
	r.calls = append(r.calls, countCall{orgUUID, projectUUID})
	return r.n, nil
}

// artifactRepos holds one counting repository per artifact kind.
type artifactRepos struct {
	rest    *countingRESTRepo
	graphql *countingGraphQLRepo
	llm     *countingLLMProxyRepo
	mcp     *countingMCPProxyRepo
	agent   *countingAgentProxyRepo
}

func newArtifactRepos() *artifactRepos {
	return &artifactRepos{
		rest:    &countingRESTRepo{},
		graphql: &countingGraphQLRepo{},
		llm:     &countingLLMProxyRepo{},
		mcp:     &countingMCPProxyRepo{},
		agent:   &countingAgentProxyRepo{},
	}
}

// service returns a ProjectService whose organization holds one project, "payments".
func (r *artifactRepos) service() *ProjectService {
	projects := &mockProjectDeletionRepo{projects: []*model.Project{
		{ID: artifactsTestProject, Handle: "payments", OrganizationID: artifactsTestOrg},
	}}
	return NewProjectService(projects, nil, r.rest, r.graphql, r.llm, r.mcp, r.agent, nil, nil, nil, nil)
}

func TestCountArtifactsByProject_AddsUpEveryKind(t *testing.T) {
	repos := newArtifactRepos()
	repos.rest.n, repos.graphql.n, repos.llm.n, repos.mcp.n, repos.agent.n = 2, 1, 1, 3, 1

	total, err := repos.service().CountArtifactsByProject("payments", artifactsTestOrg)
	require.NoError(t, err)
	assert.Equal(t, 8, total)

	want := []countCall{{artifactsTestOrg, artifactsTestProject}}
	assert.Equal(t, want, repos.rest.calls, "REST APIs")
	assert.Equal(t, want, repos.graphql.calls, "GraphQL APIs")
	assert.Equal(t, want, repos.llm.calls, "LLM proxies")
	assert.Equal(t, want, repos.mcp.calls, "MCP proxies")
	assert.Equal(t, want, repos.agent.calls, "Agent proxies")
}

func TestCountArtifactsByProject_EmptyProjectIsZero(t *testing.T) {
	total, err := newArtifactRepos().service().CountArtifactsByProject("payments", artifactsTestOrg)
	require.NoError(t, err)
	assert.Zero(t, total)
}

func TestCountArtifactsByProject_UnknownProjectIsNotFound(t *testing.T) {
	repos := newArtifactRepos()

	_, err := repos.service().CountArtifactsByProject("orders", artifactsTestOrg)
	assert.True(t, apperror.ProjectNotFound.Is(err), "got %v", err)
	assert.Empty(t, repos.rest.calls)
}

func TestCountArtifactsByProject_OtherOrganizationIsNotFound(t *testing.T) {
	_, err := newArtifactRepos().service().CountArtifactsByProject("payments", "org-other")
	assert.True(t, apperror.ProjectNotFound.Is(err), "got %v", err)
}

// A failed count must not read as a smaller total, or a project could look empty.
func TestCountArtifactsByProject_FailedCountIsAnError(t *testing.T) {
	repos := newArtifactRepos()
	repos.rest.n = 2
	repos.graphql.err = errors.New("connection reset")

	total, err := repos.service().CountArtifactsByProject("payments", artifactsTestOrg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GraphQL APIs")
	assert.Zero(t, total)
}

// GraphQL APIs and LLM proxies cascade with their project, so deleting a project is refused
// while either remains, as it is for REST APIs.
func TestDeleteProject_RefusedWhileGraphQLAPIsOrLLMProxiesRemain(t *testing.T) {
	for name, tc := range map[string]struct {
		graphql, llm int
		want         string
	}{
		"GraphQL API": {graphql: 1, want: "GraphQL APIs"},
		"LLM proxy":   {llm: 1, want: "LLM proxies"},
		"neither":     {},
	} {
		t.Run(name, func(t *testing.T) {
			projects := &mockProjectDeletionRepo{projects: []*model.Project{
				{ID: artifactsTestProject, Handle: "payments", OrganizationID: artifactsTestOrg},
				{ID: "project-2", Handle: "orders", OrganizationID: artifactsTestOrg},
			}}
			svc := NewProjectService(projects, nil, &emptyAPIRepo{}, &countingGraphQLRepo{n: tc.graphql},
				&countingLLMProxyRepo{n: tc.llm}, &mockAgentProjectMCPRepo{}, &countingAgentProxyRepo{},
				&emptyApplicationRepo{}, &noopAuditRepo{}, newTestIdentityService(), slog.Default())

			err := svc.DeleteProject("payments", artifactsTestOrg, "alice")
			if tc.want == "" {
				require.NoError(t, err)
				return
			}
			assert.True(t, apperror.ValidationFailed.Is(err), "got %v", err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
