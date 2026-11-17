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

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

const (
	deletionTestOrg     = "org-deletion-unit"
	deletionTestProject = "6f1c2d3e-4a5b-4c6d-8e7f-9a0b1c2d3e4f"
)

type countingRESTRepo struct {
	repository.APIRepository
	n int
}

func (r *countingRESTRepo) CountAPIsByOrganizationUUID(_, _, _ string) (int, error) { return r.n, nil }

type countingGraphQLRepo struct {
	repository.GraphQLAPIRepository
	n   int
	err error
}

func (r *countingGraphQLRepo) CountByProject(_, _, _ string) (int, error) { return r.n, r.err }

type countingLLMProxyRepo struct {
	repository.LLMProxyRepository
	n int
}

func (r *countingLLMProxyRepo) CountByProject(_, _ string) (int, error) { return r.n, nil }

type countingMCPProxyRepo struct {
	repository.MCPProxyRepository
	n int
}

func (r *countingMCPProxyRepo) CountByProject(_, _ string) (int, error) { return r.n, nil }

type countingAgentProxyRepo struct {
	repository.AgentProxyRepository
	n int
}

func (r *countingAgentProxyRepo) CountByProject(_, _ string) (int, error) { return r.n, nil }

type countingApplicationRepo struct {
	repository.ApplicationRepository
	n int
}

func (r *countingApplicationRepo) CountApplicationsByProjectID(_, _, _ string) (int, error) {
	return r.n, nil
}

type refusingGuard struct{ err error }

func (g refusingGuard) CheckProjectDeletion(_, _ string) error { return g.err }

// deletionRepos holds one counting repository per kind a project deletion checks.
type deletionRepos struct {
	rest    *countingRESTRepo
	graphql *countingGraphQLRepo
	llm     *countingLLMProxyRepo
	mcp     *countingMCPProxyRepo
	agent   *countingAgentProxyRepo
	apps    *countingApplicationRepo
}

func newDeletionRepos() *deletionRepos {
	return &deletionRepos{
		rest:    &countingRESTRepo{},
		graphql: &countingGraphQLRepo{},
		llm:     &countingLLMProxyRepo{},
		mcp:     &countingMCPProxyRepo{},
		agent:   &countingAgentProxyRepo{},
		apps:    &countingApplicationRepo{},
	}
}

// service returns a ProjectService for an organization holding "payments" and, unless
// onlyProject is set, a second project.
func (r *deletionRepos) service(cfg *config.Server, onlyProject bool) *ProjectService {
	projects := []*model.Project{{ID: deletionTestProject, Handle: "payments", OrganizationID: deletionTestOrg}}
	if !onlyProject {
		projects = append(projects, &model.Project{ID: "project-2", Handle: "orders", OrganizationID: deletionTestOrg})
	}
	return NewProjectService(&mockProjectDeletionRepo{projects: projects}, nil, r.rest, r.graphql, r.llm, r.mcp,
		r.agent, r.apps, &noopAuditRepo{}, newTestIdentityService(), cfg, slog.Default())
}

func TestCheckProjectDeletion_EmptyProjectCanBeDeleted(t *testing.T) {
	svc := newDeletionRepos().service(nil, false)

	require.NoError(t, svc.CheckProjectDeletion("payments", deletionTestOrg))
	require.NoError(t, svc.DeleteProject("payments", deletionTestOrg, "alice"))
}

// The check refuses exactly what the delete refuses, with the same reason.
func TestCheckProjectDeletion_RefusesWhatDeleteRefuses(t *testing.T) {
	for name, tc := range map[string]struct {
		set  func(*deletionRepos)
		want string
	}{
		"REST API":    {func(r *deletionRepos) { r.rest.n = 1 }, "Project has associated APIs"},
		"GraphQL API": {func(r *deletionRepos) { r.graphql.n = 1 }, "Project has associated GraphQL APIs"},
		"LLM proxy":   {func(r *deletionRepos) { r.llm.n = 1 }, "Project has associated LLM proxies"},
		"MCP proxy":   {func(r *deletionRepos) { r.mcp.n = 1 }, "Project has associated MCP proxies"},
		"Agent proxy": {func(r *deletionRepos) { r.agent.n = 1 }, "Project has associated Agent proxies"},
		"application": {func(r *deletionRepos) { r.apps.n = 1 }, "Project has associated applications"},
	} {
		t.Run(name, func(t *testing.T) {
			repos := newDeletionRepos()
			tc.set(repos)
			svc := repos.service(nil, false)

			checkErr := svc.CheckProjectDeletion("payments", deletionTestOrg)
			assert.True(t, apperror.ValidationFailed.Is(checkErr), "got %v", checkErr)
			assert.Contains(t, checkErr.Error(), tc.want)
			assert.Equal(t, checkErr.Error(), svc.DeleteProject("payments", deletionTestOrg, "alice").Error())
		})
	}
}

// A plugin's deletion guard refuses the check as it refuses the delete.
func TestCheckProjectDeletion_RunsTheDeletionGuards(t *testing.T) {
	svc := newDeletionRepos().service(nil, false)
	refusal := apperror.ValidationFailed.New("Project has associated WebSub APIs")
	svc.RegisterDeletionGuard(refusingGuard{err: refusal})

	assert.ErrorIs(t, svc.CheckProjectDeletion("payments", deletionTestOrg), refusal)
}

// An organization keeps its last project only while new organizations get a default one.
func TestCheckProjectDeletion_LastProject(t *testing.T) {
	err := newDeletionRepos().service(nil, true).CheckProjectDeletion("payments", deletionTestOrg)
	assert.True(t, apperror.ValidationFailed.Is(err), "got %v", err)
	assert.Contains(t, err.Error(), "at least one project")

	noDefault := &config.Server{Organizations: config.Organizations{CreateDefaultProject: false}}
	require.NoError(t, newDeletionRepos().service(noDefault, true).CheckProjectDeletion("payments", deletionTestOrg))
}

func TestCheckProjectDeletion_UnknownProjectIsNotFound(t *testing.T) {
	svc := newDeletionRepos().service(nil, false)

	assert.True(t, apperror.ProjectNotFound.Is(svc.CheckProjectDeletion("unknown", deletionTestOrg)))
	assert.True(t, apperror.ProjectNotFound.Is(svc.CheckProjectDeletion("payments", "org-other")))
}

// A check that cannot complete is an error, never a pass.
func TestCheckProjectDeletion_FailedCheckIsAnError(t *testing.T) {
	repos := newDeletionRepos()
	repos.graphql.err = errors.New("connection reset")

	err := repos.service(nil, false).CheckProjectDeletion("payments", deletionTestOrg)
	require.Error(t, err)
	assert.False(t, apperror.ValidationFailed.Is(err))
}
