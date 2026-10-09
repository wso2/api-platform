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

// Unit coverage for the Agent proxy service's create, list, delete and Agent
// Card fetch paths, plus the small helpers they share. The HTTP-level contract
// is covered end to end in internal/handler/agent_proxy_*_integration_test.go.

package service

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// apsTRepo is an in-memory Agent proxy repository keyed by handle, with error
// injection per operation. Operations the tests do not reach panic through the
// embedded nil interface.
type apsTRepo struct {
	repository.AgentProxyRepository
	byHandle map[string]*model.AgentProxy

	createErr error
	getErr    error
	listErr   error
	countErr  error
	updateErr error
	deleteErr error
	existsErr error

	created     *model.AgentProxy
	listOpts    repository.AgentProxyListOptions
	deleteCalls int
}

func newAPSTRepo(proxies ...*model.AgentProxy) *apsTRepo {
	r := &apsTRepo{byHandle: map[string]*model.AgentProxy{}}
	for _, p := range proxies {
		r.byHandle[p.Handle] = p
	}
	return r
}

func (r *apsTRepo) Create(p *model.AgentProxy) error {
	if r.createErr != nil {
		return r.createErr
	}
	p.UUID = "uuid-" + p.Handle
	r.created = p
	r.byHandle[p.Handle] = p
	return nil
}

func (r *apsTRepo) GetByHandle(handle, orgUUID string) (*model.AgentProxy, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	p, ok := r.byHandle[handle]
	if !ok || p.OrganizationUUID != orgUUID {
		return nil, nil
	}
	return p, nil
}

func (r *apsTRepo) List(orgUUID string, opts repository.AgentProxyListOptions) ([]*model.AgentProxy, error) {
	r.listOpts = opts
	if r.listErr != nil {
		return nil, r.listErr
	}
	var out []*model.AgentProxy
	for _, p := range r.byHandle {
		if p.OrganizationUUID == orgUUID && (opts.Protocol == "" || p.Protocol == opts.Protocol) &&
			(opts.ProjectUUID == "" || p.ProjectUUID == opts.ProjectUUID) {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r *apsTRepo) Count(orgUUID string, opts repository.AgentProxyListOptions) (int, error) {
	if r.countErr != nil {
		return 0, r.countErr
	}
	list, _ := r.List(orgUUID, opts)
	return len(list), nil
}

func (r *apsTRepo) Update(p *model.AgentProxy) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.byHandle[p.Handle] = p
	return nil
}

func (r *apsTRepo) Delete(handle, orgUUID string) error {
	r.deleteCalls++
	if r.deleteErr != nil {
		return r.deleteErr
	}
	delete(r.byHandle, handle)
	return nil
}

func (r *apsTRepo) Exists(handle, orgUUID string) (bool, error) {
	if r.existsErr != nil {
		return false, r.existsErr
	}
	p, ok := r.byHandle[handle]
	return ok && p.OrganizationUUID == orgUUID, nil
}

// apsTProjectRepo resolves projects with error injection on both lookups.
type apsTProjectRepo struct {
	repository.ProjectRepository
	project      *model.Project
	byHandleErr  error
	byUUIDErr    error
	byUUIDCalls  int
	nilOnUUIDGet bool
}

func (m *apsTProjectRepo) GetProjectByHandleAndOrgID(handle, orgID string) (*model.Project, error) {
	return m.project, m.byHandleErr
}

func (m *apsTProjectRepo) GetProjectByUUIDAndOrgID(projectID, orgID string) (*model.Project, error) {
	m.byUUIDCalls++
	if m.byUUIDErr != nil {
		return nil, m.byUUIDErr
	}
	if m.nilOnUUIDGet {
		return nil, nil
	}
	return m.project, nil
}

// apsTGatewayRepo serves an organization's gateways for the deletion fan-out.
type apsTGatewayRepo struct {
	repository.GatewayRepository
	gateways []*model.Gateway
	err      error
}

func (m *apsTGatewayRepo) GetByOrganizationID(orgID string) ([]*model.Gateway, error) {
	return m.gateways, m.err
}

// apsTDeploymentRepo answers the origin-deletability guard only.
type apsTDeploymentRepo struct {
	repository.DeploymentRepository
	active bool
	err    error
}

func (m *apsTDeploymentRepo) HasActiveDeployment(artifactUUID, orgID string) (bool, error) {
	return m.active, m.err
}

// apsTEventHub records the gateways an event was published to and fails for
// one chosen gateway.
type apsTEventHub struct {
	dpNoopEventHub
	failFor   string
	published []string
}

func (h *apsTEventHub) PublishEvent(gatewayID string, _ eventhub.Event) error {
	if gatewayID == h.failFor {
		return errors.New("publish failed")
	}
	h.published = append(h.published, gatewayID)
	return nil
}

func apsTProject() *model.Project {
	return &model.Project{ID: agentTestProject, Handle: "default-project", OrganizationID: agentTestOrg}
}

func newAPSTService(repo repository.AgentProxyRepository) *AgentProxyService {
	return newAgentProxyTestService(repo)
}

func apsTSecretService(handles map[string]string) (*SecretService, *mockSecretRepo) {
	secretRepo := newMockRepo()
	for handle, value := range handles {
		secretRepo.secrets[handle] = &model.Secret{
			Handle: handle, Status: model.SecretStatusActive, Ciphertext: []byte("enc:" + value),
		}
	}
	return NewSecretService(secretRepo, &mockVault{}, newTestIdentityService()), secretRepo
}

func apsTPtr[T any](v T) *T { return &v }

// ---- Create ----------------------------------------------------------------

func TestAgentProxyCreate_HappyPathWithSuppliedHandle(t *testing.T) {
	repo := newAPSTRepo()
	svc := newAPSTService(repo)

	req := validAgentProxyRequest()
	req.Id = apsTPtr("weather-agent")
	req.Description = apsTPtr("forecasts")

	resp, err := svc.Create(agentTestOrg, "alice", req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if resp == nil || resp.Id == nil || *resp.Id != "weather-agent" {
		t.Fatalf("unexpected response id: %+v", resp)
	}
	if resp.ProjectId != "default-project" {
		t.Fatalf("projectId = %q, want the handle", resp.ProjectId)
	}
	if repo.created == nil || repo.created.Origin != constants.OriginCP || repo.created.ProjectUUID != agentTestProject ||
		repo.created.OrganizationUUID != agentTestOrg || repo.created.Description != "forecasts" {
		t.Fatalf("stored row is wrong: %+v", repo.created)
	}
}

func TestAgentProxyCreate_DerivesHandleFromDisplayName(t *testing.T) {
	repo := newAPSTRepo()
	svc := newAPSTService(repo)

	resp, err := svc.Create(agentTestOrg, "alice", validAgentProxyRequest())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if resp.Id == nil || *resp.Id == "" {
		t.Fatal("no handle was derived")
	}
	if repo.created.Handle != *resp.Id {
		t.Fatalf("stored handle %q differs from response %q", repo.created.Handle, *resp.Id)
	}
}

// A display name that derives to a reserved handle must move on to another
// candidate rather than claiming the reserved segment.
func TestAgentProxyCreate_DerivedHandleSkipsReservedName(t *testing.T) {
	repo := newAPSTRepo()
	svc := newAPSTService(repo)

	req := validAgentProxyRequest()
	req.DisplayName = "Fetch Agent Card"

	resp, err := svc.Create(agentTestOrg, "alice", req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if *resp.Id == "fetch-agent-card" {
		t.Fatal("a generated handle claimed the reserved name")
	}
}

func TestAgentProxyCreate_Rejections(t *testing.T) {
	existing := storedAgentProxy(constants.OriginCP)

	cases := []struct {
		name   string
		mutate func(*AgentProxyService, *apsTRepo, *api.A2AAgentProxy)
		check  func(error) bool
	}{
		{
			name:   "nil body",
			mutate: nil,
			check:  apperror.ValidationFailed.Is,
		},
		{
			name: "duplicate handle",
			mutate: func(_ *AgentProxyService, _ *apsTRepo, req *api.A2AAgentProxy) {
				req.Id = apsTPtr(existing.Handle)
			},
			check: apperror.AgentProxyExists.Is,
		},
		{
			name: "reserved handle",
			mutate: func(_ *AgentProxyService, _ *apsTRepo, req *api.A2AAgentProxy) {
				req.Id = apsTPtr("fetch-agent-card")
			},
			check: apperror.ValidationFailed.Is,
		},
		{
			name: "exists check fails",
			mutate: func(_ *AgentProxyService, r *apsTRepo, req *api.A2AAgentProxy) {
				req.Id = apsTPtr("new-agent")
				r.existsErr = errors.New("db down")
			},
			check: func(err error) bool { return err != nil && !apperror.AgentProxyExists.Is(err) },
		},
		{
			name: "missing project id",
			mutate: func(_ *AgentProxyService, _ *apsTRepo, req *api.A2AAgentProxy) {
				req.ProjectId = "   "
			},
			check: apperror.ValidationFailed.Is,
		},
		{
			name: "project of another organization",
			mutate: func(s *AgentProxyService, _ *apsTRepo, _ *api.A2AAgentProxy) {
				p := apsTProject()
				p.OrganizationID = "other-org"
				s.projectRepo = &apsTProjectRepo{project: p}
			},
			check: apperror.ProjectNotFound.Is,
		},
		{
			name: "unknown project",
			mutate: func(s *AgentProxyService, _ *apsTRepo, _ *api.A2AAgentProxy) {
				s.projectRepo = &apsTProjectRepo{}
			},
			check: apperror.ProjectNotFound.Is,
		},
		{
			name: "project lookup fails",
			mutate: func(s *AgentProxyService, _ *apsTRepo, _ *api.A2AAgentProxy) {
				s.projectRepo = &apsTProjectRepo{byHandleErr: errors.New("db down")}
			},
			check: func(err error) bool { return err != nil && !apperror.ProjectNotFound.Is(err) },
		},
		{
			name: "project repository unavailable",
			mutate: func(s *AgentProxyService, _ *apsTRepo, _ *api.A2AAgentProxy) {
				s.projectRepo = nil
			},
			check: func(err error) bool { return err != nil },
		},
		{
			name: "credential-bearing auth with no value",
			mutate: func(_ *AgentProxyService, _ *apsTRepo, req *api.A2AAgentProxy) {
				bearer := api.Bearer
				req.Upstream.Main.Auth = &api.UpstreamAuth{Type: &bearer, Header: apsTPtr("Authorization")}
			},
			check: apperror.ValidationFailed.Is,
		},
		{
			name: "unresolvable secret reference",
			mutate: func(s *AgentProxyService, _ *apsTRepo, req *api.A2AAgentProxy) {
				ss, _ := apsTSecretService(nil)
				s.WithSecretService(ss)
				bearer := api.Bearer
				req.Upstream.Main.Auth = &api.UpstreamAuth{
					Type: &bearer, Header: apsTPtr("Authorization"), Value: apsTPtr(`{{ secret "missing-handle" }}`),
				}
			},
			check: func(err error) bool {
				return apperror.ValidationFailed.Is(err) && !strings.Contains(err.Error(), "missing-handle")
			},
		},
		{
			name: "unknown associated gateway",
			mutate: func(s *AgentProxyService, _ *apsTRepo, req *api.A2AAgentProxy) {
				s.gatewayRepo = &buildTestGatewayRepo{}
				req.AssociatedGateways = &[]api.AssociatedGateway{{Id: "nope"}}
			},
			check: apperror.GatewayNotFound.Is,
		},
		{
			name: "repository create races to a unique conflict",
			mutate: func(_ *AgentProxyService, r *apsTRepo, _ *api.A2AAgentProxy) {
				r.createErr = errors.New("UNIQUE constraint failed: agent_proxies.handle")
			},
			check: apperror.AgentProxyExists.Is,
		},
		{
			name: "repository create reports a project/org mismatch",
			mutate: func(_ *AgentProxyService, r *apsTRepo, _ *api.A2AAgentProxy) {
				r.createErr = repository.ErrAgentProxyProjectOrgMismatch
			},
			check: apperror.ProjectNotFound.Is,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newAPSTRepo(storedAgentProxy(constants.OriginCP))
			svc := newAPSTService(repo)
			var req *api.A2AAgentProxy
			if tc.mutate != nil {
				req = validAgentProxyRequest()
				tc.mutate(svc, repo, req)
			}
			_, err := svc.Create(agentTestOrg, "alice", req)
			if !tc.check(err) {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.mutate != nil && repo.created != nil {
				t.Fatal("a rejected create reached the repository")
			}
		})
	}
}

func TestAgentProxyCreate_ResolvedSecretReferenceIsAccepted(t *testing.T) {
	repo := newAPSTRepo()
	svc := newAPSTService(repo)
	ss, _ := apsTSecretService(map[string]string{"agent-token": "s3cret"})
	svc.WithSecretService(ss)

	req := validAgentProxyRequest()
	bearer := api.Bearer
	req.Upstream.Main.Auth = &api.UpstreamAuth{
		Type: &bearer, Header: apsTPtr("Authorization"), Value: apsTPtr(`{{ secret "agent-token" }}`),
	}
	if _, err := svc.Create(agentTestOrg, "alice", req); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got := repo.created.Configuration.Upstream.Main.Auth.Value; got != `{{ secret "agent-token" }}` {
		t.Fatalf("stored credential = %q, want the placeholder", got)
	}
}

// ---- List ------------------------------------------------------------------

func TestAgentProxyList_ReturnsItemsWithProjectHandles(t *testing.T) {
	a := storedAgentProxy(constants.OriginCP)
	b := storedAgentProxy(constants.OriginCP)
	b.UUID, b.Handle = "agent-uuid-2", "other-agent"
	foreign := storedAgentProxy(constants.OriginCP)
	foreign.UUID, foreign.Handle, foreign.OrganizationUUID = "agent-uuid-3", "foreign", "other-org"

	repo := newAPSTRepo(a, b, foreign)
	svc := newAPSTService(repo)
	projectRepo := &apsTProjectRepo{project: apsTProject()}
	svc.projectRepo = projectRepo

	resp, err := svc.List(agentTestOrg, nil, nil, 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if resp.Count != 2 || len(resp.List) != 2 || resp.Pagination.Total != 2 {
		t.Fatalf("count=%d len=%d total=%d, want 2/2/2", resp.Count, len(resp.List), resp.Pagination.Total)
	}
	if resp.Pagination.Limit != 10 || resp.Pagination.Offset != 0 {
		t.Fatalf("pagination echoed wrong: %+v", resp.Pagination)
	}
	for _, item := range resp.List {
		if item.ProjectId != "default-project" {
			t.Fatalf("item projectId = %q, want the handle", item.ProjectId)
		}
	}
	// Both items share a project: the per-page memo resolves it once.
	if projectRepo.byUUIDCalls != 1 {
		t.Fatalf("project resolved %d times, want 1", projectRepo.byUUIDCalls)
	}
}

func TestAgentProxyList_ProtocolFilter(t *testing.T) {
	repo := newAPSTRepo(storedAgentProxy(constants.OriginCP))
	svc := newAPSTService(repo)

	resp, err := svc.List(agentTestOrg, apsTPtr(string(model.AgentProxyProtocolA2A)), nil, 5, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if repo.listOpts.Protocol != model.AgentProxyProtocolA2A || resp.Count != 1 {
		t.Fatalf("filter not applied: opts=%+v count=%d", repo.listOpts, resp.Count)
	}

	for _, bad := range []string{"", "grpc", "A2A "} {
		if _, err := svc.List(agentTestOrg, apsTPtr(bad), nil, 5, 0); !apperror.ValidationFailed.Is(err) {
			t.Fatalf("filter %q: expected a validation failure, got %v", bad, err)
		}
	}
}

func TestAgentProxyList_ProjectFilter(t *testing.T) {
	inProject := storedAgentProxy(constants.OriginCP)
	elsewhere := storedAgentProxy(constants.OriginCP)
	elsewhere.UUID, elsewhere.Handle, elsewhere.ProjectUUID = "agent-uuid-2", "other-agent", "other-project-uuid"

	repo := newAPSTRepo(inProject, elsewhere)
	svc := newAPSTService(repo)
	svc.projectRepo = &apsTProjectRepo{project: apsTProject()}

	resp, err := svc.List(agentTestOrg, nil, apsTPtr("default-project"), 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if repo.listOpts.ProjectUUID != agentTestProject {
		t.Fatalf("project filter not passed to the repository: opts=%+v", repo.listOpts)
	}
	if resp.Count != 1 || resp.Pagination.Total != 1 || resp.List[0].Id != inProject.Handle {
		t.Fatalf("filtered list = %+v, want only %s", resp, inProject.Handle)
	}

	// Omitting the filter lists every project in the organization.
	if resp, err := svc.List(agentTestOrg, nil, nil, 10, 0); err != nil || resp.Pagination.Total != 2 || repo.listOpts.ProjectUUID != "" {
		t.Fatalf("unfiltered list: resp=%+v opts=%+v err=%v", resp, repo.listOpts, err)
	}

	for _, blank := range []string{"", "  "} {
		if _, err := svc.List(agentTestOrg, nil, apsTPtr(blank), 10, 0); !apperror.ValidationFailed.Is(err) {
			t.Fatalf("projectId %q: expected a validation failure, got %v", blank, err)
		}
	}

	svc.projectRepo = &apsTProjectRepo{}
	if _, err := svc.List(agentTestOrg, nil, apsTPtr("unknown-project"), 10, 0); !apperror.ProjectNotFound.Is(err) {
		t.Fatalf("unknown project: expected ProjectNotFound, got %v", err)
	}
	foreign := apsTProject()
	foreign.OrganizationID = "other-org"
	svc.projectRepo = &apsTProjectRepo{project: foreign}
	if _, err := svc.List(agentTestOrg, nil, apsTPtr("default-project"), 10, 0); !apperror.ProjectNotFound.Is(err) {
		t.Fatalf("cross-org project: expected ProjectNotFound, got %v", err)
	}
}

func TestAgentProxyList_Failures(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*AgentProxyService, *apsTRepo)
		check  func(error) bool
	}{
		{"list fails", func(_ *AgentProxyService, r *apsTRepo) { r.listErr = errors.New("db") }, func(e error) bool { return e != nil }},
		{"count fails", func(_ *AgentProxyService, r *apsTRepo) { r.countErr = errors.New("db") }, func(e error) bool { return e != nil }},
		{"project missing", func(s *AgentProxyService, _ *apsTRepo) {
			s.projectRepo = &apsTProjectRepo{project: apsTProject(), nilOnUUIDGet: true}
		}, apperror.ProjectNotFound.Is},
		{"project lookup fails", func(s *AgentProxyService, _ *apsTRepo) {
			s.projectRepo = &apsTProjectRepo{byUUIDErr: errors.New("db")}
		}, func(e error) bool { return e != nil && !apperror.ProjectNotFound.Is(e) }},
		{"project repository unavailable", func(s *AgentProxyService, _ *apsTRepo) { s.projectRepo = nil }, func(e error) bool { return e != nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newAPSTRepo(storedAgentProxy(constants.OriginCP))
			svc := newAPSTService(repo)
			tc.mutate(svc, repo)
			if _, err := svc.List(agentTestOrg, nil, nil, 10, 0); !tc.check(err) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// A stored row with no project UUID renders an empty projectId rather than
// failing the whole page.
func TestAgentProxyList_EmptyProjectUUIDIsNotLookedUp(t *testing.T) {
	p := storedAgentProxy(constants.OriginCP)
	p.ProjectUUID = ""
	svc := newAPSTService(newAPSTRepo(p))
	projectRepo := &apsTProjectRepo{project: apsTProject()}
	svc.projectRepo = projectRepo

	resp, err := svc.List(agentTestOrg, nil, nil, 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if resp.List[0].ProjectId != "" || projectRepo.byUUIDCalls != 0 {
		t.Fatalf("projectId=%q lookups=%d", resp.List[0].ProjectId, projectRepo.byUUIDCalls)
	}
}

// ---- Update (branches not covered in agent_proxy_test.go) ------------------

func TestAgentProxyUpdate_Rejections(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*AgentProxyService, *apsTRepo, *api.A2AAgentProxy)
		check  func(error) bool
	}{
		{"unknown handle", func(_ *AgentProxyService, r *apsTRepo, _ *api.A2AAgentProxy) {
			delete(r.byHandle, "weather-agent")
		}, apperror.AgentProxyNotFound.Is},
		{"lookup fails", func(_ *AgentProxyService, r *apsTRepo, _ *api.A2AAgentProxy) {
			r.getErr = errors.New("db")
		}, func(e error) bool { return e != nil && !apperror.AgentProxyNotFound.Is(e) }},
		{"project moved", func(s *AgentProxyService, _ *apsTRepo, _ *api.A2AAgentProxy) {
			p := apsTProject()
			p.ID = "another-project-uuid"
			s.projectRepo = &apsTProjectRepo{project: p}
		}, apperror.ValidationFailed.Is},
		{"project of another organization", func(s *AgentProxyService, _ *apsTRepo, _ *api.A2AAgentProxy) {
			p := apsTProject()
			p.OrganizationID = "other-org"
			s.projectRepo = &apsTProjectRepo{project: p}
		}, apperror.ProjectNotFound.Is},
		{"changed auth type drops the stored credential", func(_ *AgentProxyService, _ *apsTRepo, req *api.A2AAgentProxy) {
			apiKey := api.ApiKey
			req.Upstream.Main.Auth = &api.UpstreamAuth{Type: &apiKey, Header: apsTPtr("X-API-Key")}
		}, apperror.ValidationFailed.Is},
		{"unresolvable secret reference", func(s *AgentProxyService, _ *apsTRepo, req *api.A2AAgentProxy) {
			ss, _ := apsTSecretService(nil)
			s.WithSecretService(ss)
			bearer := api.Bearer
			req.Upstream.Main.Auth = &api.UpstreamAuth{
				Type: &bearer, Header: apsTPtr("Authorization"), Value: apsTPtr(`{{ secret "nope" }}`),
			}
		}, apperror.ValidationFailed.Is},
		{"unknown associated gateway", func(s *AgentProxyService, _ *apsTRepo, req *api.A2AAgentProxy) {
			s.gatewayRepo = &buildTestGatewayRepo{}
			req.AssociatedGateways = &[]api.AssociatedGateway{{Id: "nope"}}
		}, apperror.GatewayNotFound.Is},
		{"row vanished during update", func(_ *AgentProxyService, r *apsTRepo, _ *api.A2AAgentProxy) {
			r.updateErr = sql.ErrNoRows
		}, apperror.AgentProxyNotFound.Is},
		{"repository refuses a protocol change", func(_ *AgentProxyService, r *apsTRepo, _ *api.A2AAgentProxy) {
			r.updateErr = repository.ErrAgentProxyProtocolImmutable
		}, apperror.ValidationFailed.Is},
		{"repository fails", func(_ *AgentProxyService, r *apsTRepo, _ *api.A2AAgentProxy) {
			r.updateErr = errors.New("db")
		}, func(e error) bool { return e != nil && strings.Contains(e.Error(), "failed to update agent proxy") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stored := storedAgentProxy(constants.OriginCP)
			stored.Configuration.Upstream.Main.Auth = &model.UpstreamAuth{Type: "bearer", Header: "Authorization", Value: "stored"}
			repo := newAPSTRepo(stored)
			svc := newAPSTService(repo)
			req := validAgentProxyRequest()
			tc.mutate(svc, repo, req)
			if _, err := svc.Update(agentTestOrg, "weather-agent", "alice", req); !tc.check(err) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestAgentProxyUpdate_NilBody(t *testing.T) {
	svc := newAPSTService(newAPSTRepo())
	if _, err := svc.Update(agentTestOrg, "weather-agent", "alice", nil); !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected a validation failure, got %v", err)
	}
}

// Rotating the upstream credential to a new secret retires the old one, and
// the stale cached card is dropped.
func TestAgentProxyUpdate_RotatesSecretAndInvalidatesCard(t *testing.T) {
	stored := storedAgentProxy(constants.OriginCP)
	stored.Configuration.Upstream.Main.Auth = &model.UpstreamAuth{
		Type: "bearer", Header: "Authorization", Value: `{{ secret "old-token" }}`,
	}
	repo := newAPSTRepo(stored)
	svc := newAPSTService(repo)
	ss, secretRepo := apsTSecretService(map[string]string{"old-token": "a", "new-token": "b"})
	svc.WithSecretService(ss)
	svc.cardCache = newAgentCardCache(defaultTestCardCacheConfig())

	key := agentCardCacheKey{orgUUID: agentTestOrg, proxyUUID: stored.UUID}
	svc.cardCache.storeCard(key, svc.cardCache.begin(key), []byte(`{"name":"stale"}`))

	req := validAgentProxyRequest()
	bearer := api.Bearer
	req.Upstream.Main.Auth = &api.UpstreamAuth{
		Type: &bearer, Header: apsTPtr("Authorization"), Value: apsTPtr(`{{ secret "new-token" }}`),
	}
	if _, err := svc.Update(agentTestOrg, stored.Handle, "alice", req); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if secretRepo.secrets["old-token"].Status != model.SecretStatusDeprecated {
		t.Fatal("rotated-out secret was not retired")
	}
	if secretRepo.secrets["new-token"].Status != model.SecretStatusActive {
		t.Fatal("the new secret was touched")
	}
	if _, _, ok := svc.cardCache.get(key); ok {
		t.Fatal("cached card survived an update")
	}
}

// ---- Delete ----------------------------------------------------------------

func TestAgentProxyDelete_NotifiesEveryGatewayAndDropsCache(t *testing.T) {
	stored := storedAgentProxy(constants.OriginCP)
	repo := newAPSTRepo(stored)
	svc := newAPSTService(repo)
	hub := &apsTEventHub{failFor: "gw-2"}
	svc.gatewayEventsService = NewGatewayEventsService(hub, newTestIdentityService(), newTestLogger())
	svc.gatewayRepo = &apsTGatewayRepo{gateways: []*model.Gateway{{ID: "gw-1"}, {ID: "gw-2"}, {ID: "gw-3"}}}
	svc.cardCache = newAgentCardCache(defaultTestCardCacheConfig())
	key := agentCardCacheKey{orgUUID: agentTestOrg, proxyUUID: stored.UUID}
	svc.cardCache.storeCard(key, svc.cardCache.begin(key), []byte(`{"name":"x"}`))

	if err := svc.Delete(agentTestOrg, stored.Handle, "alice"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if repo.deleteCalls != 1 {
		t.Fatalf("repository delete called %d times", repo.deleteCalls)
	}
	// A failed broadcast to one gateway does not stop the others.
	if strings.Join(hub.published, ",") != "gw-1,gw-3" {
		t.Fatalf("published to %v, want gw-1 and gw-3", hub.published)
	}
	if _, _, ok := svc.cardCache.get(key); ok {
		t.Fatal("cached card survived deletion")
	}
}

// A failure to enumerate gateways is logged, not fatal: the row is still
// deleted and simply nobody is notified.
func TestAgentProxyDelete_GatewayListingFailureStillDeletes(t *testing.T) {
	repo := newAPSTRepo(storedAgentProxy(constants.OriginCP))
	svc := newAPSTService(repo)
	hub := &apsTEventHub{}
	svc.gatewayEventsService = NewGatewayEventsService(hub, newTestIdentityService(), newTestLogger())
	svc.gatewayRepo = &apsTGatewayRepo{err: errors.New("db")}

	if err := svc.Delete(agentTestOrg, "weather-agent", "alice"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if repo.deleteCalls != 1 || len(hub.published) != 0 {
		t.Fatalf("deleteCalls=%d published=%v", repo.deleteCalls, hub.published)
	}
}

func TestAgentProxyDelete_Rejections(t *testing.T) {
	cases := []struct {
		name       string
		origin     string
		mutate     func(*AgentProxyService, *apsTRepo)
		check      func(error) bool
		wantDelete bool
	}{
		{"unknown handle", constants.OriginCP, func(_ *AgentProxyService, r *apsTRepo) {
			delete(r.byHandle, "weather-agent")
		}, apperror.AgentProxyNotFound.Is, false},
		{"gateway-originated and still deployed", constants.OriginDP, func(s *AgentProxyService, _ *apsTRepo) {
			s.deploymentRepo = &apsTDeploymentRepo{active: true}
		}, apperror.ArtifactDeployed.Is, false},
		{"gateway-originated and deployment check fails", constants.OriginDP, func(s *AgentProxyService, _ *apsTRepo) {
			s.deploymentRepo = &apsTDeploymentRepo{err: errors.New("db")}
		}, func(e error) bool { return e != nil }, false},
		{"row vanished during delete", constants.OriginCP, func(_ *AgentProxyService, r *apsTRepo) {
			r.deleteErr = sql.ErrNoRows
		}, apperror.AgentProxyNotFound.Is, true},
		{"repository delete fails", constants.OriginCP, func(_ *AgentProxyService, r *apsTRepo) {
			r.deleteErr = errors.New("db")
		}, func(e error) bool { return e != nil && strings.Contains(e.Error(), "failed to delete agent proxy") }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newAPSTRepo(storedAgentProxy(tc.origin))
			svc := newAPSTService(repo)
			tc.mutate(svc, repo)
			if err := svc.Delete(agentTestOrg, "weather-agent", "alice"); !tc.check(err) {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := repo.deleteCalls > 0; got != tc.wantDelete {
				t.Fatalf("repository delete reached = %v, want %v", got, tc.wantDelete)
			}
		})
	}
}

func TestAgentProxyDelete_UndeployedGatewayOriginatedProxyIsDeletable(t *testing.T) {
	repo := newAPSTRepo(storedAgentProxy(constants.OriginDP))
	svc := newAPSTService(repo)
	svc.deploymentRepo = &apsTDeploymentRepo{active: false}

	if err := svc.Delete(agentTestOrg, "weather-agent", "alice"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if repo.deleteCalls != 1 {
		t.Fatal("an undeployed DP-originated Agent proxy was not deleted")
	}
}

// ---- FetchAgentCard --------------------------------------------------------

// apsTCardServer serves body with status on every path and records the auth
// header value it saw.
func apsTCardServer(t *testing.T, status int, body string) (*httptest.Server, *atomic.Value, *atomic.Int32) {
	t.Helper()
	var seen atomic.Value
	seen.Store("")
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		seen.Store(r.Header.Get("X-Agent-Key"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &seen, &hits
}

func TestAgentProxyFetchAgentCard_NilRequest(t *testing.T) {
	svc := newAPSTService(newAPSTRepo())
	if _, err := svc.FetchAgentCard(agentTestOrg, nil, false); !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected a validation failure, got %v", err)
	}
}

func TestAgentProxyFetchAgentCard_DirectURL(t *testing.T) {
	const card = `{"name":"weather"}`
	srv, seen, _ := apsTCardServer(t, http.StatusOK, card)
	svc := newAPSTService(newAPSTRepo())

	apiKey := api.ApiKey
	res, err := svc.FetchAgentCard(agentTestOrg, &dto.AgentCardFetchRequest{
		URL:  srv.URL,
		Auth: &api.UpstreamAuth{Type: &apiKey, Header: apsTPtr("X-Agent-Key"), Value: apsTPtr("supplied")},
	}, false)
	if err != nil {
		t.Fatalf("FetchAgentCard: %v", err)
	}
	if string(res.Card) != card || res.Cacheable {
		t.Fatalf("result = %+v", res)
	}
	if seen.Load() != "supplied" {
		t.Fatalf("upstream saw credential %q", seen.Load())
	}
}

func TestAgentProxyFetchAgentCard_DirectURLFailures(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		maxSize int64
		wantMsg string
	}{
		{"unauthorized", http.StatusUnauthorized, `{}`, 0, "rejected the credentials"},
		{"forbidden", http.StatusForbidden, `{}`, 0, "rejected the credentials"},
		{"server error", http.StatusInternalServerError, `{}`, 0, "could not reach"},
		{"not found", http.StatusNotFound, `{}`, 0, "could not reach"},
		{"not a JSON object", http.StatusOK, `[1,2]`, 0, "did not return a usable"},
		{"oversized card", http.StatusOK, `{"name":"` + strings.Repeat("x", 128) + `"}`, 64, "did not return a usable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, _, _ := apsTCardServer(t, tc.status, tc.body)
			svc := newAPSTService(newAPSTRepo())
			svc.cfg = &config.Server{AgentCardMaxFetchBytes: tc.maxSize}

			res, err := svc.FetchAgentCard(agentTestOrg, &dto.AgentCardFetchRequest{URL: srv.URL}, false)
			if !apperror.AgentProxyUpstreamUnreachable.Is(err) {
				t.Fatalf("expected AgentProxyUpstreamUnreachable, got %v", err)
			}
			if res == nil || res.Card != nil {
				t.Fatalf("a failed fetch carried a card: %+v", res)
			}
			var appErr *apperror.Error
			if !errors.As(err, &appErr) || !strings.Contains(appErr.Message, tc.wantMsg) {
				t.Fatalf("message = %v, want it to contain %q", err, tc.wantMsg)
			}
			if strings.Contains(appErr.Message, srv.URL) {
				t.Fatal("the client message leaked the upstream URL")
			}
		})
	}
}

func TestAgentProxyFetchAgentCard_DirectURLInvalid(t *testing.T) {
	svc := newAPSTService(newAPSTRepo())
	_, err := svc.FetchAgentCard(agentTestOrg, &dto.AgentCardFetchRequest{URL: "not a url"}, false)
	if !apperror.ValidationFailed.Is(err) {
		t.Fatalf("expected a validation failure, got %v", err)
	}
}

func TestAgentProxyFetchAgentCard_DirectURLUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // nothing listens there any more

	svc := newAPSTService(newAPSTRepo())
	_, err := svc.FetchAgentCard(agentTestOrg, &dto.AgentCardFetchRequest{URL: url}, false)
	if !apperror.AgentProxyUpstreamUnreachable.Is(err) || !errors.Is(err, utils.ErrAgentCardUnreachable) {
		t.Fatalf("expected an unreachable failure, got %v", err)
	}
}

func apsTStoredCardProxy(url string, auth *model.UpstreamAuth) *model.AgentProxy {
	p := storedAgentProxy(constants.OriginCP)
	p.Configuration.Upstream.Main = &model.UpstreamEndpoint{URL: url, Auth: auth}
	return p
}

func TestAgentProxyFetchAgentCard_StoredUsesCacheAndNoCacheRefreshes(t *testing.T) {
	const card = `{"name":"weather"}`
	srv, _, hits := apsTCardServer(t, http.StatusOK, card)
	svc := newAPSTService(newAPSTRepo(apsTStoredCardProxy(srv.URL, nil)))
	svc.cardCache = newAgentCardCache(defaultTestCardCacheConfig())
	req := &dto.AgentCardFetchRequest{AgentProxyID: "weather-agent"}

	first, err := svc.FetchAgentCard(agentTestOrg, req, false)
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if string(first.Card) != card || !first.Cacheable || first.MaxAge != defaultTestCardCacheConfig().PositiveTTL {
		t.Fatalf("first result = %+v", first)
	}

	second, err := svc.FetchAgentCard(agentTestOrg, req, false)
	if err != nil || string(second.Card) != card {
		t.Fatalf("cached fetch: %+v, %v", second, err)
	}
	if hits.Load() != 1 {
		t.Fatalf("upstream contacted %d times, want 1 (second served from cache)", hits.Load())
	}

	if _, err := svc.FetchAgentCard(agentTestOrg, req, true); err != nil {
		t.Fatalf("no-cache fetch: %v", err)
	}
	if hits.Load() != 2 {
		t.Fatalf("no-cache did not contact the upstream: hits=%d", hits.Load())
	}

	svc.InvalidateAgentCard(agentTestOrg, "agent-uuid-1")
	if _, err := svc.FetchAgentCard(agentTestOrg, req, false); err != nil {
		t.Fatalf("post-invalidation fetch: %v", err)
	}
	if hits.Load() != 3 {
		t.Fatalf("InvalidateAgentCard did not drop the entry: hits=%d", hits.Load())
	}
}

func TestAgentProxyFetchAgentCard_StoredFailureIsCached(t *testing.T) {
	srv, _, hits := apsTCardServer(t, http.StatusBadGateway, `{}`)
	svc := newAPSTService(newAPSTRepo(apsTStoredCardProxy(srv.URL, nil)))
	svc.cardCache = newAgentCardCache(defaultTestCardCacheConfig())
	req := &dto.AgentCardFetchRequest{AgentProxyID: "weather-agent"}

	res, err := svc.FetchAgentCard(agentTestOrg, req, false)
	if !apperror.AgentProxyUpstreamUnreachable.Is(err) || res == nil || !res.Cacheable {
		t.Fatalf("first fetch = %+v, %v", res, err)
	}
	res, err = svc.FetchAgentCard(agentTestOrg, req, false)
	if !apperror.AgentProxyUpstreamUnreachable.Is(err) || res == nil || res.Card != nil {
		t.Fatalf("cached failure = %+v, %v", res, err)
	}
	if hits.Load() != 1 {
		t.Fatalf("a cached failure re-contacted the upstream: hits=%d", hits.Load())
	}
}

func TestAgentProxyFetchAgentCard_StoredResolvesCredential(t *testing.T) {
	cases := []struct {
		name string
		auth *model.UpstreamAuth
		want string
	}{
		{"secret placeholder", &model.UpstreamAuth{Type: "api-key", Header: "X-Agent-Key", Value: `{{ secret "agent-key" }}`}, "resolved-secret"},
		{"plaintext value", &model.UpstreamAuth{Type: "api-key", Header: "X-Agent-Key", Value: "inline"}, "inline"},
		{"type none", &model.UpstreamAuth{Type: "none", Header: "X-Agent-Key", Value: "ignored"}, ""},
		{"no header", &model.UpstreamAuth{Type: "api-key", Value: "ignored"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, seen, _ := apsTCardServer(t, http.StatusOK, `{"name":"x"}`)
			svc := newAPSTService(newAPSTRepo(apsTStoredCardProxy(srv.URL, tc.auth)))
			ss, _ := apsTSecretService(map[string]string{"agent-key": "resolved-secret"})
			svc.WithSecretService(ss)

			if _, err := svc.FetchAgentCard(agentTestOrg, &dto.AgentCardFetchRequest{AgentProxyID: "weather-agent"}, true); err != nil {
				t.Fatalf("FetchAgentCard: %v", err)
			}
			if seen.Load() != tc.want {
				t.Fatalf("upstream saw %q, want %q", seen.Load(), tc.want)
			}
		})
	}
}

func TestAgentProxyFetchAgentCard_StoredRejections(t *testing.T) {
	secretAuth := &model.UpstreamAuth{Type: "api-key", Header: "X-Agent-Key", Value: `{{ secret "agent-key" }}`}
	cases := []struct {
		name   string
		proxy  func() *model.AgentProxy
		mutate func(*AgentProxyService)
		handle string
		check  func(error) bool
	}{
		{"unknown handle", func() *model.AgentProxy { return storedAgentProxy(constants.OriginCP) }, nil,
			"no-such-agent", apperror.AgentProxyNotFound.Is},
		{"blank handle", func() *model.AgentProxy { return storedAgentProxy(constants.OriginCP) }, nil,
			" ", apperror.ValidationFailed.Is},
		{"non-A2A protocol", func() *model.AgentProxy {
			p := storedAgentProxy(constants.OriginCP)
			p.Protocol = "mcp"
			return p
		}, nil, "weather-agent", apperror.ValidationFailed.Is},
		{"upstream by reference", func() *model.AgentProxy {
			p := storedAgentProxy(constants.OriginCP)
			p.Configuration.Upstream.Main = &model.UpstreamEndpoint{Ref: "shared-upstream"}
			return p
		}, nil, "weather-agent", apperror.ValidationFailed.Is},
		{"no main upstream", func() *model.AgentProxy {
			p := storedAgentProxy(constants.OriginCP)
			p.Configuration.Upstream.Main = nil
			return p
		}, nil, "weather-agent", apperror.ValidationFailed.Is},
		{"secret service unavailable", func() *model.AgentProxy {
			return apsTStoredCardProxy("http://127.0.0.1:1", secretAuth)
		}, nil, "weather-agent", apperror.Internal.Is},
		{"secret does not resolve", func() *model.AgentProxy {
			return apsTStoredCardProxy("http://127.0.0.1:1", secretAuth)
		}, func(s *AgentProxyService) {
			ss, _ := apsTSecretService(nil)
			s.WithSecretService(ss)
		}, "weather-agent", func(e error) bool {
			return apperror.Internal.Is(e) && !strings.Contains(e.Error(), "agent-key")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newAPSTService(newAPSTRepo(tc.proxy()))
			if tc.mutate != nil {
				tc.mutate(svc)
			}
			_, err := svc.FetchAgentCard(agentTestOrg, &dto.AgentCardFetchRequest{AgentProxyID: tc.handle}, false)
			if !tc.check(err) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// A foreign organization's handle is a 404 even when the owning organization
// already has a cached card for it.
func TestAgentProxyFetchAgentCard_StoredIsOrgScopedBeforeCache(t *testing.T) {
	srv, _, _ := apsTCardServer(t, http.StatusOK, `{"name":"x"}`)
	svc := newAPSTService(newAPSTRepo(apsTStoredCardProxy(srv.URL, nil)))
	svc.cardCache = newAgentCardCache(defaultTestCardCacheConfig())
	req := &dto.AgentCardFetchRequest{AgentProxyID: "weather-agent"}

	if _, err := svc.FetchAgentCard(agentTestOrg, req, false); err != nil {
		t.Fatalf("owner fetch: %v", err)
	}
	if _, err := svc.FetchAgentCard("other-org", req, false); !apperror.AgentProxyNotFound.Is(err) {
		t.Fatalf("foreign org fetch: expected AgentProxyNotFound, got %v", err)
	}
}

// ---- helpers ---------------------------------------------------------------

func TestSuppliedAgentCardAuthHeader(t *testing.T) {
	none, apiKey := api.None, api.ApiKey
	cases := []struct {
		name      string
		auth      *api.UpstreamAuth
		hdr, valu string
	}{
		{"nil", nil, "", ""},
		{"no type", &api.UpstreamAuth{Header: apsTPtr("H"), Value: apsTPtr("v")}, "", ""},
		{"none", &api.UpstreamAuth{Type: &none, Header: apsTPtr("H"), Value: apsTPtr("v")}, "", ""},
		{"missing header", &api.UpstreamAuth{Type: &apiKey, Value: apsTPtr("v")}, "", ""},
		{"missing value", &api.UpstreamAuth{Type: &apiKey, Header: apsTPtr("H")}, "", ""},
		{"complete", &api.UpstreamAuth{Type: &apiKey, Header: apsTPtr("H"), Value: apsTPtr("v")}, "H", "v"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, v := suppliedAgentCardAuthHeader(tc.auth)
			if h != tc.hdr || v != tc.valu {
				t.Fatalf("got (%q, %q), want (%q, %q)", h, v, tc.hdr, tc.valu)
			}
		})
	}
}

func TestAgentCardFetchFailure_MapsReasonClasses(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{fmt.Errorf("%w: 401", utils.ErrAgentCardUnauthorized), "rejected the credentials"},
		{fmt.Errorf("%w: html", utils.ErrAgentCardUnusable), "did not return a usable"},
		{fmt.Errorf("%w: refused", utils.ErrAgentCardUnreachable), "could not reach"},
		{errors.New("anything else"), "could not reach"},
	}
	for _, tc := range cases {
		got := agentCardFetchFailure(tc.err)
		if !apperror.AgentProxyUpstreamUnreachable.Is(got) || !strings.Contains(got.Message, tc.want) {
			t.Fatalf("%v -> %v, want message containing %q", tc.err, got, tc.want)
		}
		if !errors.Is(got, tc.err) {
			t.Fatalf("the cause was not wrapped for %v", tc.err)
		}
	}
}

func TestAgentCardMaxFetchBytes(t *testing.T) {
	svc := newAPSTService(newAPSTRepo())
	svc.cfg = nil
	if got := svc.agentCardMaxFetchBytes(); got != 0 {
		t.Fatalf("nil config = %d, want 0", got)
	}
	svc.cfg = &config.Server{AgentCardMaxFetchBytes: 4096}
	if got := svc.agentCardMaxFetchBytes(); got != 4096 {
		t.Fatalf("configured = %d, want 4096", got)
	}
}

func TestMapRepositoryError(t *testing.T) {
	svc := newAPSTService(newAPSTRepo())
	cases := []struct {
		name  string
		err   error
		check func(error) bool
	}{
		{"no rows", sql.ErrNoRows, apperror.AgentProxyNotFound.Is},
		{"protocol immutable", repository.ErrAgentProxyProtocolImmutable, apperror.ValidationFailed.Is},
		{"project org mismatch", repository.ErrAgentProxyProjectOrgMismatch, apperror.ProjectNotFound.Is},
		{"unique constraint", errors.New("UNIQUE constraint failed: x"), apperror.AgentProxyExists.Is},
		{"other", errors.New("boom"), func(e error) bool {
			var appErr *apperror.Error
			return !errors.As(e, &appErr) && strings.HasPrefix(e.Error(), "ctx: ")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := svc.mapRepositoryError(tc.err, "ctx")
			if !tc.check(got) || !errors.Is(got, tc.err) {
				t.Fatalf("mapped %v to %v", tc.err, got)
			}
		})
	}
}

func TestValidateEffectiveUpstreamAuth(t *testing.T) {
	complete := &model.UpstreamAuth{Type: "bearer", Value: "tok"}
	cases := []struct {
		name    string
		cfg     *model.UpstreamConfig
		wantErr string
	}{
		{"nil config", nil, ""},
		{"no endpoints", &model.UpstreamConfig{}, ""},
		{"no auth", &model.UpstreamConfig{Main: &model.UpstreamEndpoint{URL: "http://a"}}, ""},
		{"none needs no value", &model.UpstreamConfig{Main: &model.UpstreamEndpoint{Auth: &model.UpstreamAuth{Type: "none"}}}, ""},
		{"complete main and sandbox", &model.UpstreamConfig{
			Main: &model.UpstreamEndpoint{Auth: complete}, Sandbox: &model.UpstreamEndpoint{Auth: complete},
		}, ""},
		{"main missing value", &model.UpstreamConfig{Main: &model.UpstreamEndpoint{Auth: &model.UpstreamAuth{Type: "bearer", Value: "  "}}}, "main"},
		{"sandbox missing value", &model.UpstreamConfig{
			Main:    &model.UpstreamEndpoint{Auth: complete},
			Sandbox: &model.UpstreamEndpoint{Auth: &model.UpstreamAuth{Type: "api-key"}},
		}, "sandbox"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateEffectiveUpstreamAuth(tc.cfg)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !apperror.ValidationFailed.Is(err) || !strings.Contains(err.Error(), "upstream "+tc.wantErr+" auth") {
				t.Fatalf("got %v, want a validation failure naming %q", err, tc.wantErr)
			}
		})
	}
}

func TestEnsureAgentProxyHandleNotReserved(t *testing.T) {
	if err := ensureAgentProxyHandleNotReserved("fetch-agent-card"); !apperror.ValidationFailed.Is(err) {
		t.Fatalf("reserved handle: got %v", err)
	}
	if err := ensureAgentProxyHandleNotReserved("weather-agent"); err != nil {
		t.Fatalf("ordinary handle: got %v", err)
	}
}

func TestParseAgentProxyProtocolFilter(t *testing.T) {
	if p, err := parseAgentProxyProtocolFilter(nil); err != nil || p != "" {
		t.Fatalf("nil filter = (%q, %v)", p, err)
	}
	if p, err := parseAgentProxyProtocolFilter(apsTPtr("a2a")); err != nil || p != model.AgentProxyProtocolA2A {
		t.Fatalf("a2a filter = (%q, %v)", p, err)
	}
	for _, bad := range []string{"", "mcp", "A2A"} {
		if _, err := parseAgentProxyProtocolFilter(apsTPtr(bad)); !apperror.ValidationFailed.Is(err) {
			t.Fatalf("filter %q: got %v", bad, err)
		}
	}
}

// A non-validation failure from the secret store surfaces as a 500 whose
// message carries no secret handle.
func TestValidateSecretRefs_StoreFailureIsInternal(t *testing.T) {
	svc := newAPSTService(newAPSTRepo())
	ss, secretRepo := apsTSecretService(nil)
	secretRepo.existsFn = func(string, string) (bool, error) { return false, errors.New("db down") }
	svc.WithSecretService(ss)

	cfg := model.AgentProxyConfiguration{Upstream: model.UpstreamConfig{Main: &model.UpstreamEndpoint{
		Auth: &model.UpstreamAuth{Type: "bearer", Value: `{{ secret "private-handle" }}`},
	}}}
	err := svc.validateSecretRefs(agentTestOrg, cfg)
	if !apperror.Internal.Is(err) {
		t.Fatalf("expected Internal, got %v", err)
	}
	var appErr *apperror.Error
	if errors.As(err, &appErr) && strings.Contains(appErr.Message, "private-handle") {
		t.Fatal("the client message named the secret handle")
	}

	// No placeholders: nothing to validate.
	if err := svc.validateSecretRefs(agentTestOrg, model.AgentProxyConfiguration{}); err != nil {
		t.Fatalf("no placeholders: %v", err)
	}
}

func TestWithSecretServiceReturnsReceiver(t *testing.T) {
	svc := newAPSTService(newAPSTRepo())
	ss, _ := apsTSecretService(nil)
	if got := svc.WithSecretService(ss); got != svc || svc.secretService != ss {
		t.Fatal("WithSecretService did not inject into and return the receiver")
	}
}
