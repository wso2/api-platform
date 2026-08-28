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
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// --- test doubles -----------------------------------------------------

// mockGraphQLAPIRepo is a configurable in-memory-ish fake satisfying
// repository.GraphQLAPIRepository, mirroring the mocking style used across
// this repo's service-layer tests (see internal/service/api_test.go).
type mockGraphQLAPIRepo struct {
	existsResult bool
	existsErr    error

	created   *model.GraphQLAPI
	createErr error

	getByHandleFunc func(handle, orgUUID string) (*model.GraphQLAPI, error)

	updated   *model.GraphQLAPI
	updateErr error

	deleted   bool
	deleteErr error

	listResult []*model.GraphQLAPI
	listErr    error

	countResult           int
	countErr              error
	countByProjectResult  int
	countByProjectErr     error
	countByProjectCapture struct{ orgUUID, projectUUID string }

	gatewayDetails       []*model.APIGatewayWithDetails
	getGatewaysFunc      func(apiUUID, orgUUID string) ([]*model.APIGatewayWithDetails, error)
	associations         []*model.APIAssociation
	createdAssociations  []*model.APIAssociation
	createAssociationErr error
	updatedAssociation   bool

	ensureGatewayAssociationFunc func(apiUUID, gatewayUUID, orgUUID, createdBy, deployMetadata string, metadataProvided bool) (string, error)
}

func (m *mockGraphQLAPIRepo) Create(a *model.GraphQLAPI) error {
	if m.createErr != nil {
		return m.createErr
	}
	a.ID = "generated-uuid"
	m.created = a
	return nil
}

func (m *mockGraphQLAPIRepo) GetByHandle(handle, orgUUID string) (*model.GraphQLAPI, error) {
	if m.getByHandleFunc != nil {
		return m.getByHandleFunc(handle, orgUUID)
	}
	return m.created, nil
}

func (m *mockGraphQLAPIRepo) GetByUUID(uuid, orgUUID string) (*model.GraphQLAPI, error) {
	return nil, nil
}

func (m *mockGraphQLAPIRepo) List(orgUUID, projectUUID string, limit, offset int) ([]*model.GraphQLAPI, error) {
	return m.listResult, m.listErr
}

func (m *mockGraphQLAPIRepo) Count(orgUUID string) (int, error) { return m.countResult, m.countErr }

func (m *mockGraphQLAPIRepo) CountByProject(orgUUID, projectUUID string) (int, error) {
	m.countByProjectCapture.orgUUID = orgUUID
	m.countByProjectCapture.projectUUID = projectUUID
	return m.countByProjectResult, m.countByProjectErr
}

func (m *mockGraphQLAPIRepo) Update(a *model.GraphQLAPI) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.updated = a
	return nil
}

func (m *mockGraphQLAPIRepo) Delete(handle, orgUUID string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.deleted = true
	return nil
}

func (m *mockGraphQLAPIRepo) Exists(handle, orgUUID string) (bool, error) {
	return m.existsResult, m.existsErr
}

func (m *mockGraphQLAPIRepo) GetAPIGatewaysWithDetails(apiUUID, orgUUID string) ([]*model.APIGatewayWithDetails, error) {
	if m.getGatewaysFunc != nil {
		return m.getGatewaysFunc(apiUUID, orgUUID)
	}
	return m.gatewayDetails, nil
}

func (m *mockGraphQLAPIRepo) CreateAPIAssociation(association *model.APIAssociation) error {
	if m.createAssociationErr != nil {
		return m.createAssociationErr
	}
	m.createdAssociations = append(m.createdAssociations, association)
	return nil
}

func (m *mockGraphQLAPIRepo) GetAPIAssociations(apiUUID, associationType, orgUUID string) ([]*model.APIAssociation, error) {
	return m.associations, nil
}

func (m *mockGraphQLAPIRepo) UpdateAPIAssociation(apiUUID, resourceId, associationType, orgUUID, updatedBy string) error {
	m.updatedAssociation = true
	return nil
}

func (m *mockGraphQLAPIRepo) EnsureGatewayAssociation(apiUUID, gatewayUUID, orgUUID, createdBy, deployMetadata string, metadataProvided bool) (string, error) {
	if m.ensureGatewayAssociationFunc != nil {
		return m.ensureGatewayAssociationFunc(apiUUID, gatewayUUID, orgUUID, createdBy, deployMetadata, metadataProvided)
	}
	return deployMetadata, nil
}

var _ repository.GraphQLAPIRepository = (*mockGraphQLAPIRepo)(nil)

// mockGraphQLProjectRepo embeds the interface so only the methods a test
// needs are implemented; everything else panics if accidentally called.
type mockGraphQLProjectRepo struct {
	repository.ProjectRepository
	project *model.Project
}

func (m *mockGraphQLProjectRepo) GetProjectByUUID(projectId string) (*model.Project, error) {
	return m.project, nil
}

func (m *mockGraphQLProjectRepo) GetProjectByHandleAndOrgID(handle, orgID string) (*model.Project, error) {
	return m.project, nil
}

// newGraphQLTestService wires a GraphQLAPIService for tests, reusing the
// package's shared noopAuditRepo (llm_test.go) and newTestIdentityService
// (identity_test_helpers_test.go) test doubles. Gateway/org repos are wired
// with empty defaults — use newGraphQLTestServiceWithGateways for tests that
// exercise AddGatewaysToAPI/GetAPIGateways.
func newGraphQLTestService(repo *mockGraphQLAPIRepo, project *model.Project) *GraphQLAPIService {
	return newGraphQLTestServiceWithGateways(repo, project, &mockGatewayRepository{}, &mockOrganizationRepo{})
}

// newGraphQLTestServiceWithGateways is newGraphQLTestService with caller-supplied
// gateway/org repo mocks, for tests exercising the gateway-association methods.
func newGraphQLTestServiceWithGateways(repo *mockGraphQLAPIRepo, project *model.Project, gatewayRepo repository.GatewayRepository, orgRepo repository.OrganizationRepository) *GraphQLAPIService {
	return NewGraphQLAPIService(
		repo,
		&mockGraphQLProjectRepo{project: project},
		&noopAuditRepo{},
		nil, // deploymentRepo — not needed unless exercising Delete's origin-deletable guard
		gatewayRepo,
		orgRepo,
		nil, // gatewayEventsService — not needed unless exercising deletion-event broadcast
		newTestIdentityService(),
		slog.Default(),
	)
}

func graphQLCatalogCode(t *testing.T, err error) string {
	t.Helper()
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected an *apperror.Error, got %T: %v", err, err)
	}
	return appErr.Code
}

func graphQLStrPtr(s string) *string { return &s }

const validCountriesGraphQLSDL = `type Query {
  countries: [String]
}`

// --- tests --------------------------------------------------------------

func TestGraphQLCreate_WithSDL_Success(t *testing.T) {
	repo := &mockGraphQLAPIRepo{}
	project := &model.Project{ID: "project-uuid", OrganizationID: "org-1"}
	svc := newGraphQLTestService(repo, project)

	req := &api.CreateGraphQLAPIRequest{
		DisplayName: "Countries GraphQL API",
		Context:     "/countries",
		Version:     "v1.0",
		ProjectId:   "project-uuid",
		Sdl:         graphQLStrPtr(validCountriesGraphQLSDL),
		Upstream: api.Upstream{
			Main: api.UpstreamDefinition{Url: graphQLStrPtr("https://countries.example.com/graphql")},
		},
	}

	resp, err := svc.Create("org-1", "creator-uuid", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected a response, got nil")
	}
	if repo.created == nil {
		t.Fatal("expected repo.Create to be called")
	}
	if repo.created.Configuration.IntrospectionMode != "SDL" {
		t.Errorf("expected introspectionMode SDL, got %q", repo.created.Configuration.IntrospectionMode)
	}
	if repo.created.Configuration.SDL != validCountriesGraphQLSDL {
		t.Errorf("expected stored SDL to match the supplied SDL verbatim")
	}
	if repo.created.OrganizationID != "org-1" {
		t.Errorf("expected organization to come from the authenticated context, got %q", repo.created.OrganizationID)
	}
}

func TestGraphQLCreate_WithIntrospection_Success(t *testing.T) {
	introspectionJSON := `{
		"data": {
			"__schema": {
				"queryType": {"name": "Query"},
				"mutationType": null,
				"subscriptionType": null,
				"types": [
					{
						"kind": "OBJECT",
						"name": "Query",
						"description": "",
						"fields": [
							{
								"name": "hello",
								"description": "",
								"args": [],
								"type": {"kind": "SCALAR", "name": "String", "ofType": null}
							}
						]
					}
				]
			}
		}
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(introspectionJSON))
	}))
	defer server.Close()

	repo := &mockGraphQLAPIRepo{}
	project := &model.Project{ID: "project-uuid", OrganizationID: "org-1"}
	svc := newGraphQLTestService(repo, project)

	req := &api.CreateGraphQLAPIRequest{
		DisplayName: "Introspected API",
		Context:     "/introspected",
		Version:     "v1.0",
		ProjectId:   "project-uuid",
		Upstream: api.Upstream{
			Main: api.UpstreamDefinition{Url: graphQLStrPtr(server.URL)},
		},
	}

	resp, err := svc.Create("org-1", "creator-uuid", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected a response, got nil")
	}
	if repo.created.Configuration.IntrospectionMode != "ENDPOINT" {
		t.Errorf("expected introspectionMode ENDPOINT, got %q", repo.created.Configuration.IntrospectionMode)
	}
	if !strings.Contains(repo.created.Configuration.SDL, "type Query") {
		t.Errorf("expected derived SDL to contain a Query type, got: %s", repo.created.Configuration.SDL)
	}
	if !strings.Contains(repo.created.Configuration.SDL, "hello") {
		t.Errorf("expected derived SDL to contain the introspected field, got: %s", repo.created.Configuration.SDL)
	}
}

// TestGraphQLCreate_IntrospectionFailure_UnprocessableEntity covers
// "introspection endpoint unreachable/malformed" — the counterpart to
// TestGraphQLCreate_MalformedSDL_UnprocessableEntity's "SDL fails to parse."
// fetchAndConvertGraphQLSchema's upstream client intentionally allows
// private/in-cluster addresses (it's the tenant's own configured backend,
// same policy as MCP) — unlike sdlUrl's public-only fetcher, so a local
// httptest.Server genuinely exercises this path rather than tripping an SSRF
// block first.
func TestGraphQLCreate_IntrospectionFailure_UnprocessableEntity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("not json at all"))
	}))
	defer server.Close()

	repo := &mockGraphQLAPIRepo{}
	project := &model.Project{ID: "project-uuid", OrganizationID: "org-1"}
	svc := newGraphQLTestService(repo, project)

	req := &api.CreateGraphQLAPIRequest{
		DisplayName: "Unreachable Introspection API",
		Context:     "/unreachable",
		Version:     "v1.0",
		ProjectId:   "project-uuid",
		Upstream:    api.Upstream{Main: api.UpstreamDefinition{Url: graphQLStrPtr(server.URL)}},
	}

	_, err := svc.Create("org-1", "creator-uuid", req)
	if err == nil {
		t.Fatal("expected an error for a failed introspection")
	}
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected an *apperror.Error, got %T: %v", err, err)
	}
	if appErr.Code != apperror.CodeGraphQLAPISchemaResolveFailed {
		t.Errorf("expected %s, got %s", apperror.CodeGraphQLAPISchemaResolveFailed, appErr.Code)
	}
	if appErr.HTTPStatus != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", appErr.HTTPStatus)
	}
	if strings.Contains(appErr.Message, "not json at all") || strings.Contains(appErr.Message, server.URL) {
		t.Errorf("client message leaks introspection internals: %q", appErr.Message)
	}
	if repo.created != nil {
		t.Error("expected no repository write when introspection fails")
	}
}

// TestGraphQLCreate_SchemaResolveFailure_IdenticalShapeRegardlessOfCause pins
// the CSV's "422 introspection failure and 422 SDL parse failure return the
// identical generic response shape" scenario directly: both failure causes
// route through the exact same apperror.GraphQLAPISchemaResolveFailed catalog
// entry, so the client-visible {code, httpStatus, message} triple must be
// byte-for-byte identical no matter which cause produced it — verified here
// rather than left to code inspection alone.
func TestGraphQLCreate_SchemaResolveFailure_IdenticalShapeRegardlessOfCause(t *testing.T) {
	introspectionServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer introspectionServer.Close()

	malformedSDLReq := &api.CreateGraphQLAPIRequest{
		DisplayName: "Broken API", Context: "/broken", Version: "v1.0", ProjectId: "project-uuid",
		Sdl:      graphQLStrPtr("this is not { valid SDL at all"),
		Upstream: api.Upstream{Main: api.UpstreamDefinition{Url: graphQLStrPtr("https://example.com/graphql")}},
	}
	introspectionFailureReq := &api.CreateGraphQLAPIRequest{
		DisplayName: "Unreachable API", Context: "/unreachable", Version: "v1.0", ProjectId: "project-uuid",
		Upstream: api.Upstream{Main: api.UpstreamDefinition{Url: graphQLStrPtr(introspectionServer.URL)}},
	}

	project := &model.Project{ID: "project-uuid", OrganizationID: "org-1"}
	_, sdlErr := newGraphQLTestService(&mockGraphQLAPIRepo{}, project).Create("org-1", "creator-uuid", malformedSDLReq)
	_, introspectErr := newGraphQLTestService(&mockGraphQLAPIRepo{}, project).Create("org-1", "creator-uuid", introspectionFailureReq)

	var sdlAppErr, introspectAppErr *apperror.Error
	if !errors.As(sdlErr, &sdlAppErr) || !errors.As(introspectErr, &introspectAppErr) {
		t.Fatalf("expected both errors to be *apperror.Error, got %T and %T", sdlErr, introspectErr)
	}
	if sdlAppErr.Code != introspectAppErr.Code {
		t.Errorf("expected identical error codes, got %q vs %q", sdlAppErr.Code, introspectAppErr.Code)
	}
	if sdlAppErr.HTTPStatus != introspectAppErr.HTTPStatus {
		t.Errorf("expected identical HTTP status, got %d vs %d", sdlAppErr.HTTPStatus, introspectAppErr.HTTPStatus)
	}
	if sdlAppErr.Message != introspectAppErr.Message {
		t.Errorf("expected identical generic message regardless of cause, got %q vs %q", sdlAppErr.Message, introspectAppErr.Message)
	}
}

func TestGraphQLCreate_DuplicateHandle_Conflict(t *testing.T) {
	repo := &mockGraphQLAPIRepo{existsResult: true}
	project := &model.Project{ID: "project-uuid", OrganizationID: "org-1"}
	svc := newGraphQLTestService(repo, project)

	req := &api.CreateGraphQLAPIRequest{
		Id:          graphQLStrPtr("countries-graphql-api"),
		DisplayName: "Countries GraphQL API",
		Context:     "/countries",
		Version:     "v1.0",
		ProjectId:   "project-uuid",
		Sdl:         graphQLStrPtr(validCountriesGraphQLSDL),
		Upstream:    api.Upstream{Main: api.UpstreamDefinition{Url: graphQLStrPtr("https://example.com/graphql")}},
	}

	_, err := svc.Create("org-1", "creator-uuid", req)
	if err == nil {
		t.Fatal("expected an error for a duplicate handle")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeGraphQLAPIExists {
		t.Errorf("expected %s, got %s", apperror.CodeGraphQLAPIExists, code)
	}
}

func TestGraphQLGet_CrossOrg_NotFound(t *testing.T) {
	stored := &model.GraphQLAPI{
		ID:             "some-uuid",
		Handle:         "countries-graphql-api",
		OrganizationID: "org-1",
		Configuration:  model.GraphQLAPIConfig{SDL: validCountriesGraphQLSDL},
	}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) {
			// Simulate the real repository's WHERE handle = ? AND organization_uuid = ?
			// clause: a lookup under a different org never matches the row.
			if orgUUID != stored.OrganizationID {
				return nil, nil
			}
			return stored, nil
		},
	}
	svc := newGraphQLTestService(repo, nil)

	// Same-org lookup succeeds and returns the full object, including sdl.
	resp, err := svc.Get("org-1", "countries-graphql-api")
	if err != nil {
		t.Fatalf("unexpected error for same-org lookup: %v", err)
	}
	if resp.Sdl == nil || *resp.Sdl != validCountriesGraphQLSDL {
		t.Errorf("expected Get to return the full object including sdl, got Sdl=%v", resp.Sdl)
	}

	// Cross-org lookup must be indistinguishable from "does not exist" (404,
	// never 403) per error-handling.md's existence-hiding convention.
	_, err = svc.Get("org-2", "countries-graphql-api")
	if err == nil {
		t.Fatal("expected an error for a cross-org lookup")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeGraphQLAPINotFound {
		t.Errorf("expected %s, got %s", apperror.CodeGraphQLAPINotFound, code)
	}
}

// TestGraphQLGet_NotFound covers a handle that simply doesn't exist (as
// opposed to TestGraphQLGet_CrossOrg_NotFound's wrong-org case) — both must
// produce the identical 404, never leaking which reason applied.
func TestGraphQLGet_NotFound(t *testing.T) {
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) {
			return nil, nil
		},
	}
	svc := newGraphQLTestService(repo, nil)

	_, err := svc.Get("org-1", "does-not-exist")
	if err == nil {
		t.Fatal("expected an error for a nonexistent handle")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeGraphQLAPINotFound {
		t.Errorf("expected %s, got %s", apperror.CodeGraphQLAPINotFound, code)
	}
}

// TestGraphQLGetDetail_OmitsSDL guards GetDetail's whole reason for existing:
// GET /graphql-apis/{graphqlApiId} must return everything Get does except
// sdl, which moved to GetSDL/GET .../sdl.
func TestGraphQLGetDetail_OmitsSDL(t *testing.T) {
	stored := &model.GraphQLAPI{
		ID:             "some-uuid",
		Handle:         "countries-graphql-api",
		Name:           "Countries GraphQL API",
		OrganizationID: "org-1",
		Configuration:  model.GraphQLAPIConfig{SDL: validCountriesGraphQLSDL},
	}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) {
			return stored, nil
		},
	}
	svc := newGraphQLTestService(repo, nil)

	resp, err := svc.GetDetail("org-1", "countries-graphql-api")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected a response, got nil")
	}
	if resp.DisplayName != stored.Name {
		t.Errorf("expected displayName %q, got %q", stored.Name, resp.DisplayName)
	}
	// GraphQLAPIDetail has no Sdl field at all — the compiler enforces the
	// omission; this test guards that GetDetail otherwise returns the same
	// metadata Get does.
}

func TestGraphQLGetDetail_NotFound(t *testing.T) {
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) {
			return nil, nil
		},
	}
	svc := newGraphQLTestService(repo, nil)

	_, err := svc.GetDetail("org-1", "does-not-exist")
	if err == nil {
		t.Fatal("expected an error for a nonexistent handle")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeGraphQLAPINotFound {
		t.Errorf("expected %s, got %s", apperror.CodeGraphQLAPINotFound, code)
	}
}

// TestGraphQLGetSDL_ReturnsSDL guards GetSDL — the counterpart endpoint that
// now serves what GetDetail omits.
func TestGraphQLGetSDL_ReturnsSDL(t *testing.T) {
	stored := &model.GraphQLAPI{
		ID:             "some-uuid",
		Handle:         "countries-graphql-api",
		OrganizationID: "org-1",
		Configuration:  model.GraphQLAPIConfig{SDL: validCountriesGraphQLSDL},
	}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) {
			if orgUUID != stored.OrganizationID {
				return nil, nil
			}
			return stored, nil
		},
	}
	svc := newGraphQLTestService(repo, nil)

	sdl, err := svc.GetSDL("org-1", "countries-graphql-api")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sdl != validCountriesGraphQLSDL {
		t.Errorf("expected the stored SDL, got %q", sdl)
	}

	// Cross-org lookup must 404 exactly like Get/GetDetail.
	if _, err := svc.GetSDL("org-2", "countries-graphql-api"); err == nil {
		t.Fatal("expected an error for a cross-org lookup")
	} else if code := graphQLCatalogCode(t, err); code != apperror.CodeGraphQLAPINotFound {
		t.Errorf("expected %s, got %s", apperror.CodeGraphQLAPINotFound, code)
	}
}

func TestGraphQLGetSDL_NotFound(t *testing.T) {
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) {
			return nil, nil
		},
	}
	svc := newGraphQLTestService(repo, nil)

	_, err := svc.GetSDL("org-1", "does-not-exist")
	if err == nil {
		t.Fatal("expected an error for a nonexistent handle")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeGraphQLAPINotFound {
		t.Errorf("expected %s, got %s", apperror.CodeGraphQLAPINotFound, code)
	}
}

// TestGraphQLList_NoProjectFilter_ReturnsAllAndResolvesHandles guards the
// no-project-filter path (Count, not CountByProject) and the per-item
// project-UUID -> handle resolution (mirrors REST's modelToRESTAPIUnresolved,
// see List's doc comment).
func TestGraphQLList_NoProjectFilter_ReturnsAllAndResolvesHandles(t *testing.T) {
	stored := []*model.GraphQLAPI{
		{
			ID: "uuid-1", Handle: "countries-graphql-api", Name: "Countries", Version: "v1.0",
			OrganizationID: "org-1", ProjectID: "project-uuid", CreatedBy: "creator-uuid",
			Configuration: model.GraphQLAPIConfig{SDL: validCountriesGraphQLSDL},
		},
		{
			ID: "uuid-2", Handle: "weather-graphql-api", Name: "Weather", Version: "v1.0",
			OrganizationID: "org-1", ProjectID: "project-uuid", CreatedBy: "creator-uuid",
			Configuration: model.GraphQLAPIConfig{SDL: validCountriesGraphQLSDL},
		},
	}
	repo := &mockGraphQLAPIRepo{listResult: stored, countResult: 2}
	project := &model.Project{ID: "project-uuid", Handle: "default-project", OrganizationID: "org-1"}
	svc := newGraphQLTestService(repo, project)

	resp, err := svc.List("org-1", "", 100, 0)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if resp.Count != 2 || resp.Pagination.Total != 2 {
		t.Fatalf("expected count/total 2, got count=%d total=%d", resp.Count, resp.Pagination.Total)
	}
	if len(resp.List) != 2 {
		t.Fatalf("expected 2 list items, got %d", len(resp.List))
	}
	for _, item := range resp.List {
		if item.ProjectId != "default-project" {
			t.Errorf("expected ProjectId resolved to handle %q, got %q", "default-project", item.ProjectId)
		}
	}
}

// TestGraphQLList_ProjectFilter_ResolvesHandleToUUIDBeforeFiltering guards the
// bug found during the live smoke test: a caller-supplied projectId is a
// handle, not the internal UUID rows are keyed on, and must be resolved via
// GetProjectByHandleAndOrgID before being used to filter/count.
func TestGraphQLList_ProjectFilter_ResolvesHandleToUUIDBeforeFiltering(t *testing.T) {
	repo := &mockGraphQLAPIRepo{listResult: nil, countByProjectResult: 0}
	project := &model.Project{ID: "project-uuid", Handle: "default-project", OrganizationID: "org-1"}
	svc := newGraphQLTestService(repo, project)

	if _, err := svc.List("org-1", "default-project", 100, 0); err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if repo.countByProjectCapture.projectUUID != "project-uuid" {
		t.Errorf("expected repo.CountByProject to be called with the resolved UUID %q, got %q", "project-uuid", repo.countByProjectCapture.projectUUID)
	}
}

// TestGraphQLList_UnknownProjectHandle_NotFound guards against silently
// falling back to an unfiltered (org-wide) list when the caller-supplied
// project handle doesn't resolve to any project in this org.
func TestGraphQLList_UnknownProjectHandle_NotFound(t *testing.T) {
	repo := &mockGraphQLAPIRepo{}
	svc := newGraphQLTestService(repo, nil) // mockGraphQLProjectRepo.project == nil => "not found"

	_, err := svc.List("org-1", "does-not-exist", 100, 0)
	if err == nil {
		t.Fatal("expected an error for an unknown project handle")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeProjectRefNotFound {
		t.Errorf("expected %s, got %s", apperror.CodeProjectRefNotFound, code)
	}
}

func TestGraphQLCreate_MalformedSDL_UnprocessableEntity(t *testing.T) {
	repo := &mockGraphQLAPIRepo{}
	project := &model.Project{ID: "project-uuid", OrganizationID: "org-1"}
	svc := newGraphQLTestService(repo, project)

	req := &api.CreateGraphQLAPIRequest{
		DisplayName: "Broken API",
		Context:     "/broken",
		Version:     "v1.0",
		ProjectId:   "project-uuid",
		Sdl:         graphQLStrPtr("this is not { valid SDL at all"),
		Upstream:    api.Upstream{Main: api.UpstreamDefinition{Url: graphQLStrPtr("https://example.com/graphql")}},
	}

	_, err := svc.Create("org-1", "creator-uuid", req)
	if err == nil {
		t.Fatal("expected an error for malformed SDL")
	}
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected an *apperror.Error, got %T: %v", err, err)
	}
	if appErr.Code != apperror.CodeGraphQLAPISchemaResolveFailed {
		t.Errorf("expected %s, got %s", apperror.CodeGraphQLAPISchemaResolveFailed, appErr.Code)
	}
	if appErr.HTTPStatus != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", appErr.HTTPStatus)
	}
	// Sterile response: the client message must never echo raw parser internals.
	if strings.Contains(strings.ToLower(appErr.Message), "expected") || strings.Contains(appErr.Message, "{") {
		t.Errorf("client message leaks parser internals: %q", appErr.Message)
	}
	if repo.created != nil {
		t.Error("expected no repository write for a schema that failed validation")
	}
}

// TestGraphQLCreate_SDLWithNoQueryRoot_UnprocessableEntity covers the
// schema.Query == nil branch in validateGraphQLSDL — syntactically valid SDL
// that nonetheless never defines a Query root type. Distinct from the
// malformed-syntax case above, which never reaches that check.
func TestGraphQLCreate_SDLWithNoQueryRoot_UnprocessableEntity(t *testing.T) {
	repo := &mockGraphQLAPIRepo{}
	project := &model.Project{ID: "project-uuid", OrganizationID: "org-1"}
	svc := newGraphQLTestService(repo, project)

	req := &api.CreateGraphQLAPIRequest{
		DisplayName: "No Query Root API",
		Context:     "/no-query-root",
		Version:     "v1.0",
		ProjectId:   "project-uuid",
		Sdl:         graphQLStrPtr("type Mutation { addCountry(name: String!): String }"),
		Upstream:    api.Upstream{Main: api.UpstreamDefinition{Url: graphQLStrPtr("https://example.com/graphql")}},
	}

	_, err := svc.Create("org-1", "creator-uuid", req)
	if err == nil {
		t.Fatal("expected an error for SDL with no Query root type")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeGraphQLAPISchemaResolveFailed {
		t.Errorf("expected %s, got %s", apperror.CodeGraphQLAPISchemaResolveFailed, code)
	}
	if repo.created != nil {
		t.Error("expected no repository write for a schema with no Query root type")
	}
}

// TestGraphQLCreate_SDLTakesPrecedenceOverIntrospection guards resolveSchema's
// ordering: when both sdl and upstream.main.url are supplied, sdl must win and
// introspection must never be attempted — asserted here by failing the test if
// the introspection endpoint receives any request at all.
func TestGraphQLCreate_SDLTakesPrecedenceOverIntrospection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("introspection endpoint must not be called when sdl is supplied")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	repo := &mockGraphQLAPIRepo{}
	project := &model.Project{ID: "project-uuid", OrganizationID: "org-1"}
	svc := newGraphQLTestService(repo, project)

	req := &api.CreateGraphQLAPIRequest{
		DisplayName: "SDL Precedence API",
		Context:     "/sdl-precedence",
		Version:     "v1.0",
		ProjectId:   "project-uuid",
		Sdl:         graphQLStrPtr(validCountriesGraphQLSDL),
		Upstream:    api.Upstream{Main: api.UpstreamDefinition{Url: graphQLStrPtr(server.URL)}},
	}

	if _, err := svc.Create("org-1", "creator-uuid", req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.created.Configuration.IntrospectionMode != "SDL" {
		t.Errorf("expected introspectionMode SDL when sdl is supplied alongside upstream.main.url, got %q", repo.created.Configuration.IntrospectionMode)
	}
	if repo.created.Configuration.SDL != validCountriesGraphQLSDL {
		t.Errorf("expected the supplied sdl to be used verbatim, got %q", repo.created.Configuration.SDL)
	}
}

// TestGraphQLCreate_SDLAndSDLUrlMutuallyExclusive guards resolveSchema's
// precedence check for the third onboarding input (sdlUrl) — sdl and sdlUrl
// must never both be honored silently.
func TestGraphQLCreate_SDLAndSDLUrlMutuallyExclusive(t *testing.T) {
	repo := &mockGraphQLAPIRepo{}
	project := &model.Project{ID: "project-uuid", OrganizationID: "org-1"}
	svc := newGraphQLTestService(repo, project)

	req := &api.CreateGraphQLAPIRequest{
		DisplayName: "Both SDL Sources API",
		Context:     "/both-sdl-sources",
		Version:     "v1.0",
		ProjectId:   "project-uuid",
		Sdl:         graphQLStrPtr(validCountriesGraphQLSDL),
		SdlUrl:      graphQLStrPtr("https://example.com/schema.graphql"),
	}

	_, err := svc.Create("org-1", "creator-uuid", req)
	if err == nil {
		t.Fatal("expected an error when both sdl and sdlUrl are supplied")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeCommonValidationFailed {
		t.Errorf("expected %s, got %s", apperror.CodeCommonValidationFailed, code)
	}
	if repo.created != nil {
		t.Error("expected no repository write when sdl and sdlUrl are both supplied")
	}
}

// TestGraphQLCreate_SDLUrlFetchFailure_SchemaResolveFailed covers the
// sdlUrl decision logic itself: a URL the SSRF guard refuses (loopback,
// standing in for "unreachable/disallowed") surfaces as the sterile
// GraphQLAPISchemaResolveFailed error, not a raw network error. The
// successful-fetch path is covered by utils.TestFetchOpenAPISpecFromURL_*,
// mirroring TestResolveTemplateOpenAPISpec's convention for the identical
// LLM-provider-template case — ipIsAllowed can't be overridden from this
// package, so a real successful fetch isn't exercisable here.
func TestGraphQLCreate_SDLUrlFetchFailure_SchemaResolveFailed(t *testing.T) {
	repo := &mockGraphQLAPIRepo{}
	project := &model.Project{ID: "project-uuid", OrganizationID: "org-1"}
	svc := newGraphQLTestService(repo, project)

	req := &api.CreateGraphQLAPIRequest{
		DisplayName: "SDL URL Blocked API",
		Context:     "/sdl-url-blocked",
		Version:     "v1.0",
		ProjectId:   "project-uuid",
		SdlUrl:      graphQLStrPtr("http://127.0.0.1:9/schema.graphql"),
	}

	_, err := svc.Create("org-1", "creator-uuid", req)
	if err == nil {
		t.Fatal("expected an error for a blocked sdlUrl")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeGraphQLAPISchemaResolveFailed {
		t.Errorf("expected %s, got %s", apperror.CodeGraphQLAPISchemaResolveFailed, code)
	}
	if repo.created != nil {
		t.Error("expected no repository write when sdlUrl fetch fails")
	}
}

// TestGraphQLCreate_SDLUrlFetchFailure_DoesNotFallBackToIntrospection locks in
// a real design decision in resolveSchema: a failed sdlUrl fetch fails the
// request outright — it does NOT silently fall back to introspecting
// upstream.main.url, even when that upstream is present and reachable. The
// introspection endpoint must never be called in this case.
func TestGraphQLCreate_SDLUrlFetchFailure_DoesNotFallBackToIntrospection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("introspection endpoint must not be called when sdlUrl was supplied and failed")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	repo := &mockGraphQLAPIRepo{}
	project := &model.Project{ID: "project-uuid", OrganizationID: "org-1"}
	svc := newGraphQLTestService(repo, project)

	req := &api.CreateGraphQLAPIRequest{
		DisplayName: "SDL URL Blocked With Upstream API",
		Context:     "/sdl-url-blocked-with-upstream",
		Version:     "v1.0",
		ProjectId:   "project-uuid",
		SdlUrl:      graphQLStrPtr("http://127.0.0.1:9/schema.graphql"),
		Upstream:    api.Upstream{Main: api.UpstreamDefinition{Url: graphQLStrPtr(server.URL)}},
	}

	_, err := svc.Create("org-1", "creator-uuid", req)
	if err == nil {
		t.Fatal("expected an error for a blocked sdlUrl, even with a reachable upstream present")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeGraphQLAPISchemaResolveFailed {
		t.Errorf("expected %s, got %s", apperror.CodeGraphQLAPISchemaResolveFailed, code)
	}
	if repo.created != nil {
		t.Error("expected no repository write when sdlUrl fetch fails")
	}
}

func TestGraphQLCreate_MissingSDLAndUpstream_ValidationFailed(t *testing.T) {
	repo := &mockGraphQLAPIRepo{}
	project := &model.Project{ID: "project-uuid", OrganizationID: "org-1"}
	svc := newGraphQLTestService(repo, project)

	req := &api.CreateGraphQLAPIRequest{
		DisplayName: "No Schema Source API",
		Context:     "/no-schema",
		Version:     "v1.0",
		ProjectId:   "project-uuid",
		// Neither Sdl nor Upstream.Main.Url supplied.
	}

	_, err := svc.Create("org-1", "creator-uuid", req)
	if err == nil {
		t.Fatal("expected an error when neither sdl nor upstream.main.url is supplied")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeCommonValidationFailed {
		t.Errorf("expected %s, got %s", apperror.CodeCommonValidationFailed, code)
	}
}

// TestGraphQLCreate_MissingContext_ValidationFailed covers the
// displayName/version/context required-fields check with context specifically
// omitted, matching the test-scenarios sheet's "context omitted" case.
func TestGraphQLCreate_MissingContext_ValidationFailed(t *testing.T) {
	repo := &mockGraphQLAPIRepo{}
	project := &model.Project{ID: "project-uuid", OrganizationID: "org-1"}
	svc := newGraphQLTestService(repo, project)

	req := &api.CreateGraphQLAPIRequest{
		DisplayName: "Countries GraphQL API",
		Version:     "v1.0",
		ProjectId:   "project-uuid",
		Sdl:         graphQLStrPtr(validCountriesGraphQLSDL),
		// Context omitted.
	}

	_, err := svc.Create("org-1", "creator-uuid", req)
	if err == nil {
		t.Fatal("expected an error when context is omitted")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeCommonValidationFailed {
		t.Errorf("expected %s, got %s", apperror.CodeCommonValidationFailed, code)
	}
	if repo.created != nil {
		t.Error("expected no repository write when a required field is missing")
	}
}

func TestGraphQLCreate_ProjectRefNotFound_CrossOrgProject(t *testing.T) {
	repo := &mockGraphQLAPIRepo{}
	// Project belongs to a different organization than the caller.
	project := &model.Project{ID: "project-uuid", OrganizationID: "other-org"}
	svc := newGraphQLTestService(repo, project)

	req := &api.CreateGraphQLAPIRequest{
		DisplayName: "Countries GraphQL API",
		Context:     "/countries",
		Version:     "v1.0",
		ProjectId:   "project-uuid",
		Sdl:         graphQLStrPtr(validCountriesGraphQLSDL),
		Upstream:    api.Upstream{Main: api.UpstreamDefinition{Url: graphQLStrPtr("https://example.com/graphql")}},
	}

	_, err := svc.Create("org-1", "creator-uuid", req)
	if err == nil {
		t.Fatal("expected an error for a project belonging to a different organization")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeProjectRefNotFound {
		t.Errorf("expected %s, got %s", apperror.CodeProjectRefNotFound, code)
	}
}

// TestGraphQLUpdate_Success covers the happy path Update never had a test for
// (only the DP-originated-blocked case existed) — a CP-originated artifact's
// displayName/version/sdl are replaced and persisted, and the response
// reflects the new values.
func TestGraphQLUpdate_Success(t *testing.T) {
	stored := &model.GraphQLAPI{
		ID:             "some-uuid",
		Handle:         "countries-graphql-api",
		OrganizationID: "org-1",
		ProjectID:      "project-uuid",
		Origin:         "control_plane",
		// Started life via introspection — Update below supplies sdl directly,
		// which must flip introspectionMode back to SDL.
		Configuration: model.GraphQLAPIConfig{SDL: validCountriesGraphQLSDL, IntrospectionMode: "ENDPOINT"},
	}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) {
			return stored, nil
		},
	}
	svc := newGraphQLTestService(repo, nil)

	updatedSDL := `type Query {
  countries: [String]
  country(code: ID!): String
}`
	req := &api.GraphQLAPI{
		DisplayName: "Countries GraphQL API v2",
		Context:     "/countries",
		Version:     "v1.1",
		Sdl:         graphQLStrPtr(updatedSDL),
		Upstream:    api.Upstream{Main: api.UpstreamDefinition{Url: graphQLStrPtr("https://example.com/graphql")}},
	}

	resp, err := svc.Update("org-1", "countries-graphql-api", "updater-uuid", req)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if repo.updated == nil {
		t.Fatal("expected the repository Update to be called")
	}
	if repo.updated.Name != "Countries GraphQL API v2" || repo.updated.Version != "v1.1" {
		t.Errorf("repo.Update was not given the new displayName/version: %+v", repo.updated)
	}
	if repo.updated.Configuration.SDL != updatedSDL {
		t.Errorf("repo.Update was not given the new sdl: %q", repo.updated.Configuration.SDL)
	}
	if repo.updated.Configuration.IntrospectionMode != "SDL" {
		t.Errorf("expected introspectionMode to flip to SDL when sdl is supplied directly, got %q", repo.updated.Configuration.IntrospectionMode)
	}
	if resp.IntrospectionMode == nil || *resp.IntrospectionMode != api.GraphQLIntrospectionMode("SDL") {
		t.Errorf("expected the response introspectionMode to be SDL, got %v", resp.IntrospectionMode)
	}
	if repo.updated.UpdatedBy != "updater-uuid" {
		t.Errorf("expected UpdatedBy to be set to the caller, got %q", repo.updated.UpdatedBy)
	}
	if resp.DisplayName != "Countries GraphQL API v2" || resp.Version != "v1.1" {
		t.Errorf("Update response did not reflect the new values: %+v", resp)
	}
}

// TestGraphQLUpdate_IDMismatch_400 pins Update's body-vs-path handle guard
// (graphql_api.go: "if req.Id != nil && *req.Id != "" && *req.Id != handle"),
// which had no test at all despite being a real, already-shipped check —
// the same convention REST API update uses.
func TestGraphQLUpdate_IDMismatch_400(t *testing.T) {
	stored := &model.GraphQLAPI{
		ID:             "some-uuid",
		Handle:         "countries-graphql-api",
		OrganizationID: "org-1",
		Configuration:  model.GraphQLAPIConfig{SDL: validCountriesGraphQLSDL},
	}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) {
			return stored, nil
		},
	}
	svc := newGraphQLTestService(repo, nil)

	req := &api.GraphQLAPI{
		Id:          graphQLStrPtr("a-different-handle"),
		DisplayName: "Countries GraphQL API",
		Context:     "/countries",
		Version:     "v1.0",
		Sdl:         graphQLStrPtr(validCountriesGraphQLSDL),
		Upstream:    api.Upstream{Main: api.UpstreamDefinition{Url: graphQLStrPtr("https://example.com/graphql")}},
	}

	_, err := svc.Update("org-1", "countries-graphql-api", "updater-uuid", req)
	if err == nil {
		t.Fatal("expected an error when the body id does not match the path handle")
	}
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected an *apperror.Error, got %T: %v", err, err)
	}
	if appErr.HTTPStatus != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", appErr.HTTPStatus)
	}
	if repo.updated != nil {
		t.Error("expected no repository write when the id mismatches the path handle")
	}
}

// TestGraphQLUpdate_ReIntrospect_RefreshesSchema pins Update's re-introspection
// path: omitting both sdl and sdlUrl while upstream.main.url is set makes
// resolveSchema re-derive the schema via introspection, exactly like Create's
// introspection flow — Update has no separate "re-introspect" code path, it
// reuses resolveSchema unmodified, but this behavior had no test of its own.
func TestGraphQLUpdate_ReIntrospect_RefreshesSchema(t *testing.T) {
	introspectionJSON := `{
		"data": {
			"__schema": {
				"queryType": {"name": "Query"},
				"mutationType": null,
				"subscriptionType": null,
				"types": [
					{
						"kind": "OBJECT",
						"name": "Query",
						"description": "",
						"fields": [
							{
								"name": "updatedField",
								"description": "",
								"args": [],
								"type": {"kind": "SCALAR", "name": "String", "ofType": null}
							}
						]
					}
				]
			}
		}
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(introspectionJSON))
	}))
	defer server.Close()

	stored := &model.GraphQLAPI{
		ID:             "some-uuid",
		Handle:         "countries-graphql-api",
		OrganizationID: "org-1",
		Configuration:  model.GraphQLAPIConfig{SDL: validCountriesGraphQLSDL, IntrospectionMode: "SDL"},
	}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) {
			return stored, nil
		},
	}
	svc := newGraphQLTestService(repo, nil)

	req := &api.GraphQLAPI{
		DisplayName: "Countries GraphQL API",
		Context:     "/countries",
		Version:     "v1.0",
		Upstream:    api.Upstream{Main: api.UpstreamDefinition{Url: graphQLStrPtr(server.URL)}},
	}

	if _, err := svc.Update("org-1", "countries-graphql-api", "updater-uuid", req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.updated == nil {
		t.Fatal("expected the repository Update to be called")
	}
	if repo.updated.Configuration.IntrospectionMode != "ENDPOINT" {
		t.Errorf("expected introspectionMode to flip to ENDPOINT, got %q", repo.updated.Configuration.IntrospectionMode)
	}
	if !strings.Contains(repo.updated.Configuration.SDL, "updatedField") {
		t.Errorf("expected the re-introspected SDL to reflect the backend's current schema, got: %s", repo.updated.Configuration.SDL)
	}
}

// TestGraphQLUpdate_ReIntrospectFails_NoPartialWrite pins the "no partial
// write" guarantee: resolveSchema runs — and can fail — before Update
// mutates the in-memory existing record or calls repo.Update, so a failed
// re-introspection must leave the stored config completely untouched.
func TestGraphQLUpdate_ReIntrospectFails_NoPartialWrite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	server.Close() // closed immediately — guarantees connection failure, not just a non-200

	stored := &model.GraphQLAPI{
		ID:             "some-uuid",
		Handle:         "countries-graphql-api",
		OrganizationID: "org-1",
		Configuration:  model.GraphQLAPIConfig{SDL: validCountriesGraphQLSDL, IntrospectionMode: "SDL"},
	}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) {
			return stored, nil
		},
	}
	svc := newGraphQLTestService(repo, nil)

	req := &api.GraphQLAPI{
		DisplayName: "Countries GraphQL API",
		Context:     "/countries",
		Version:     "v1.0",
		Upstream:    api.Upstream{Main: api.UpstreamDefinition{Url: graphQLStrPtr(server.URL)}},
	}

	_, err := svc.Update("org-1", "countries-graphql-api", "updater-uuid", req)
	if err == nil {
		t.Fatal("expected an error when re-introspection fails")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeGraphQLAPISchemaResolveFailed {
		t.Errorf("expected %s, got %s", apperror.CodeGraphQLAPISchemaResolveFailed, code)
	}
	if repo.updated != nil {
		t.Error("expected no repository write when re-introspection fails")
	}
	if stored.Configuration.SDL != validCountriesGraphQLSDL {
		t.Errorf("expected the in-memory existing record to be left unchanged, got sdl: %q", stored.Configuration.SDL)
	}
}

// TestGraphQLUpdate_MalformedSDL_UnprocessableEntity is Update's counterpart
// to TestGraphQLCreate_MalformedSDL_UnprocessableEntity — resolveSchema's SDL
// parse validation is shared by both entry points, but only Create had a test
// pinning it; a broken update must be rejected without touching storage.
func TestGraphQLUpdate_MalformedSDL_UnprocessableEntity(t *testing.T) {
	stored := &model.GraphQLAPI{
		ID:             "some-uuid",
		Handle:         "countries-graphql-api",
		OrganizationID: "org-1",
		Configuration:  model.GraphQLAPIConfig{SDL: validCountriesGraphQLSDL},
	}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) {
			return stored, nil
		},
	}
	svc := newGraphQLTestService(repo, nil)

	req := &api.GraphQLAPI{
		DisplayName: "Countries GraphQL API",
		Context:     "/countries",
		Version:     "v1.0",
		Sdl:         graphQLStrPtr("this is not { valid SDL at all"),
		Upstream:    api.Upstream{Main: api.UpstreamDefinition{Url: graphQLStrPtr("https://example.com/graphql")}},
	}

	_, err := svc.Update("org-1", "countries-graphql-api", "updater-uuid", req)
	if err == nil {
		t.Fatal("expected an error for malformed SDL")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeGraphQLAPISchemaResolveFailed {
		t.Errorf("expected %s, got %s", apperror.CodeGraphQLAPISchemaResolveFailed, code)
	}
	if repo.updated != nil {
		t.Error("expected no repository write for malformed SDL")
	}
}

// TestGraphQLUpdate_SDLWithNoQueryRoot_UnprocessableEntity is Update's
// counterpart to TestGraphQLCreate_SDLWithNoQueryRoot_UnprocessableEntity.
func TestGraphQLUpdate_SDLWithNoQueryRoot_UnprocessableEntity(t *testing.T) {
	stored := &model.GraphQLAPI{
		ID:             "some-uuid",
		Handle:         "countries-graphql-api",
		OrganizationID: "org-1",
		Configuration:  model.GraphQLAPIConfig{SDL: validCountriesGraphQLSDL},
	}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) {
			return stored, nil
		},
	}
	svc := newGraphQLTestService(repo, nil)

	req := &api.GraphQLAPI{
		DisplayName: "Countries GraphQL API",
		Context:     "/countries",
		Version:     "v1.0",
		Sdl:         graphQLStrPtr("type Mutation { addCountry(name: String!): String }"),
		Upstream:    api.Upstream{Main: api.UpstreamDefinition{Url: graphQLStrPtr("https://example.com/graphql")}},
	}

	_, err := svc.Update("org-1", "countries-graphql-api", "updater-uuid", req)
	if err == nil {
		t.Fatal("expected an error for SDL with no Query root type")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeGraphQLAPISchemaResolveFailed {
		t.Errorf("expected %s, got %s", apperror.CodeGraphQLAPISchemaResolveFailed, code)
	}
	if repo.updated != nil {
		t.Error("expected no repository write for a schema with no Query root type")
	}
}

// TestGraphQLUpdate_SDLAndSDLUrlMutuallyExclusive is Update's counterpart to
// TestGraphQLCreate_SDLAndSDLUrlMutuallyExclusive — the same resolveSchema
// validation is shared by both entry points.
func TestGraphQLUpdate_SDLAndSDLUrlMutuallyExclusive(t *testing.T) {
	stored := &model.GraphQLAPI{
		ID:             "some-uuid",
		Handle:         "countries-graphql-api",
		OrganizationID: "org-1",
		Origin:         "control_plane",
		Configuration:  model.GraphQLAPIConfig{SDL: validCountriesGraphQLSDL},
	}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) {
			return stored, nil
		},
	}
	svc := newGraphQLTestService(repo, nil)

	req := &api.GraphQLAPI{
		DisplayName: "Countries GraphQL API",
		Context:     "/countries",
		Version:     "v1.0",
		Sdl:         graphQLStrPtr(validCountriesGraphQLSDL),
		SdlUrl:      graphQLStrPtr("https://example.com/schema.graphql"),
	}

	_, err := svc.Update("org-1", "countries-graphql-api", "updater-uuid", req)
	if err == nil {
		t.Fatal("expected an error when both sdl and sdlUrl are supplied")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeCommonValidationFailed {
		t.Errorf("expected %s, got %s", apperror.CodeCommonValidationFailed, code)
	}
	if repo.updated != nil {
		t.Error("expected no repository write when sdl and sdlUrl are both supplied")
	}
}

// TestGraphQLUpdate_SDLUrlFetchFailure_SchemaResolveFailed is Update's
// counterpart to TestGraphQLCreate_SDLUrlFetchFailure_SchemaResolveFailed.
func TestGraphQLUpdate_SDLUrlFetchFailure_SchemaResolveFailed(t *testing.T) {
	stored := &model.GraphQLAPI{
		ID:             "some-uuid",
		Handle:         "countries-graphql-api",
		OrganizationID: "org-1",
		Origin:         "control_plane",
		Configuration:  model.GraphQLAPIConfig{SDL: validCountriesGraphQLSDL},
	}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) {
			return stored, nil
		},
	}
	svc := newGraphQLTestService(repo, nil)

	req := &api.GraphQLAPI{
		DisplayName: "Countries GraphQL API",
		Context:     "/countries",
		Version:     "v1.0",
		SdlUrl:      graphQLStrPtr("http://127.0.0.1:9/schema.graphql"),
	}

	_, err := svc.Update("org-1", "countries-graphql-api", "updater-uuid", req)
	if err == nil {
		t.Fatal("expected an error for a blocked sdlUrl")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeGraphQLAPISchemaResolveFailed {
		t.Errorf("expected %s, got %s", apperror.CodeGraphQLAPISchemaResolveFailed, code)
	}
	if repo.updated != nil {
		t.Error("expected no repository write when sdlUrl fetch fails")
	}
}

func TestGraphQLUpdate_DPOriginated_Blocked(t *testing.T) {
	stored := &model.GraphQLAPI{
		ID:             "some-uuid",
		Handle:         "countries-graphql-api",
		OrganizationID: "org-1",
		Origin:         "gateway_api",
		Configuration:  model.GraphQLAPIConfig{SDL: validCountriesGraphQLSDL},
	}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) {
			return stored, nil
		},
	}
	svc := newGraphQLTestService(repo, nil)

	req := &api.GraphQLAPI{
		DisplayName: "Countries GraphQL API",
		Context:     "/countries",
		Version:     "v1.0",
		Sdl:         graphQLStrPtr(validCountriesGraphQLSDL),
		Upstream:    api.Upstream{Main: api.UpstreamDefinition{Url: graphQLStrPtr("https://example.com/graphql")}},
	}

	_, err := svc.Update("org-1", "countries-graphql-api", "updater-uuid", req)
	if err == nil {
		t.Fatal("expected an error updating a DP-originated (gateway_api) GraphQL API")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeArtifactReadOnly {
		t.Errorf("expected %s, got %s", apperror.CodeArtifactReadOnly, code)
	}
	if repo.updated != nil {
		t.Error("expected no repository write for a DP-originated artifact update")
	}
}

func TestGraphQLDelete_NotFound(t *testing.T) {
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) {
			return nil, nil
		},
	}
	svc := newGraphQLTestService(repo, nil)

	err := svc.Delete("org-1", "does-not-exist", "deleter-uuid")
	if err == nil {
		t.Fatal("expected an error deleting a nonexistent GraphQL API")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeGraphQLAPINotFound {
		t.Errorf("expected %s, got %s", apperror.CodeGraphQLAPINotFound, code)
	}
	if repo.deleted {
		t.Error("expected the repository Delete to never be called for a 404")
	}
}

// stubOrgGatewaysRepo returns a fixed gateway list from GetByOrganizationID,
// for tests exercising deletion's fan-out broadcast (which reads every
// gateway in the org, not just associated ones — see GraphQLAPIService.Delete's
// comment on why: deployment_status rows may already be gone).
type stubOrgGatewaysRepo struct {
	repository.GatewayRepository
	gateways []*model.Gateway
}

func (r *stubOrgGatewaysRepo) GetByOrganizationID(orgID string) ([]*model.Gateway, error) {
	return r.gateways, nil
}

// decodeGraphQLDeletionEvent extracts the ApiId from a captured
// "graphqlapi.deleted" event, mirroring decodeKeyName's envelope-unwrap
// pattern (deployment_apikey_backfill_test.go).
func decodeGraphQLDeletionEvent(t *testing.T, e eventhub.Event) string {
	t.Helper()
	var envelope dto.GatewayEventDTO
	if err := json.Unmarshal([]byte(e.EventData), &envelope); err != nil {
		t.Fatalf("failed to decode event envelope: %v", err)
	}
	if envelope.Type != EventTypeGraphQLAPIDeleted {
		t.Fatalf("unexpected event type %q, want %q", envelope.Type, EventTypeGraphQLAPIDeleted)
	}
	payloadBytes, err := json.Marshal(envelope.Payload)
	if err != nil {
		t.Fatalf("failed to re-marshal payload: %v", err)
	}
	var deletion model.GraphQLAPIDeletionEvent
	if err := json.Unmarshal(payloadBytes, &deletion); err != nil {
		t.Fatalf("failed to decode deletion payload: %v", err)
	}
	return deletion.ApiId
}

// TestGraphQLDelete_BroadcastsDeletionEventToAllOrgGateways pins the fix for
// the gap found auditing deployments/gateways/api-keys wiring for GraphQL:
// GraphQLAPIService.Delete previously deleted the row and audited it but
// never notified any gateway, leaving a stale artifact behind — unlike
// APIService.DeleteAPI (api.go) and MCPProxyService.Delete (mcp.go), which
// both fan out a deletion event to every gateway in the org.
func TestGraphQLDelete_BroadcastsDeletionEventToAllOrgGateways(t *testing.T) {
	stored := &model.GraphQLAPI{
		ID:             "graphql-uuid",
		Handle:         "countries-graphql-api",
		OrganizationID: "org-1",
		Origin:         "control_plane",
	}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) {
			return stored, nil
		},
	}
	gatewayRepo := &stubOrgGatewaysRepo{gateways: []*model.Gateway{{ID: "gw-1"}, {ID: "gw-2"}}}
	hub := &capturingEventHub{}
	events := NewGatewayEventsService(hub, newTestIdentityService(), newTestLogger())

	svc := NewGraphQLAPIService(repo, &mockGraphQLProjectRepo{}, &noopAuditRepo{}, nil,
		gatewayRepo, &mockOrganizationRepo{}, events, newTestIdentityService(), slog.Default())

	if err := svc.Delete("org-1", "countries-graphql-api", "deleter-uuid"); err != nil {
		t.Fatalf("Delete() = %v, want success", err)
	}
	if !repo.deleted {
		t.Fatal("expected the repository Delete to be called")
	}
	if len(hub.published) != 2 {
		t.Fatalf("expected 2 broadcasts (one per org gateway), got %d", len(hub.published))
	}
	for _, e := range hub.published {
		if apiID := decodeGraphQLDeletionEvent(t, e); apiID != "graphql-uuid" {
			t.Errorf("expected deletion event apiId %q, got %q", "graphql-uuid", apiID)
		}
	}
}

// TestGraphQLDelete_DPOriginated_BlockedWhileDeployed pins the other half of
// the same fix: Delete now uses ensureOriginDeletable (same guard
// APIService.DeleteAPI/MCPProxyService.Delete use), not the stricter
// ensureOriginMutable — a DP-originated GraphQL API can be deleted from the
// control plane once undeployed everywhere, not never.
func TestGraphQLDelete_DPOriginated_BlockedWhileDeployed(t *testing.T) {
	stored := &model.GraphQLAPI{
		ID:             "graphql-uuid",
		Handle:         "countries-graphql-api",
		OrganizationID: "org-1",
		Origin:         "gateway_api",
	}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) {
			return stored, nil
		},
	}
	deploymentRepo := &stubActiveDeploymentRepo{active: true}

	svc := NewGraphQLAPIService(repo, &mockGraphQLProjectRepo{}, &noopAuditRepo{}, deploymentRepo,
		&mockGatewayRepository{}, &mockOrganizationRepo{}, nil, newTestIdentityService(), slog.Default())

	err := svc.Delete("org-1", "countries-graphql-api", "deleter-uuid")
	if err == nil {
		t.Fatal("expected an error deleting a DP-originated GraphQL API that is still deployed")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeArtifactDeployed {
		t.Errorf("expected %s, got %s", apperror.CodeArtifactDeployed, code)
	}
	if repo.deleted {
		t.Error("expected the repository Delete to never be called while still deployed")
	}
}

// TestGraphQLDelete_DPOriginated_SucceedsOnceUndeployed is the other half of
// ensureOriginDeletable's contract, alongside
// TestGraphQLDelete_DPOriginated_BlockedWhileDeployed: a DP-originated
// artifact CAN be deleted from the control plane once it's undeployed on
// every gateway — the guard blocks deletion only while actively deployed, not
// unconditionally like the ensureOriginMutable guard Update still uses.
func TestGraphQLDelete_DPOriginated_SucceedsOnceUndeployed(t *testing.T) {
	stored := &model.GraphQLAPI{
		ID:             "graphql-uuid",
		Handle:         "countries-graphql-api",
		OrganizationID: "org-1",
		Origin:         "gateway_api",
	}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) {
			return stored, nil
		},
	}
	deploymentRepo := &stubActiveDeploymentRepo{active: false}

	svc := NewGraphQLAPIService(repo, &mockGraphQLProjectRepo{}, &noopAuditRepo{}, deploymentRepo,
		&stubOrgGatewaysRepo{}, &mockOrganizationRepo{}, nil, newTestIdentityService(), slog.Default())

	if err := svc.Delete("org-1", "countries-graphql-api", "deleter-uuid"); err != nil {
		t.Fatalf("Delete() = %v, want success for a DP-originated artifact with no active deployment", err)
	}
	if !repo.deleted {
		t.Error("expected the repository Delete to be called once undeployed")
	}
}

// stubActiveDeploymentRepo reports a fixed HasActiveDeployment result, for
// exercising ensureOriginDeletable without a real DeploymentRepository.
type stubActiveDeploymentRepo struct {
	repository.DeploymentRepository
	active bool
}

func (r *stubActiveDeploymentRepo) HasActiveDeployment(artifactUUID, orgID string) (bool, error) {
	return r.active, nil
}
