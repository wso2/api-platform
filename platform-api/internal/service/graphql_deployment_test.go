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
	"context"
	"strings"
	"testing"
	"time"

	commonconstants "github.com/wso2/api-platform/common/constants"
	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// mockGraphQLArtifactRepo resolves any UUID/org pair to a GraphQLApi-kind
// artifact row — the only lookup BuildService.resolve does before handing off
// to graphqlAPIDefinition.Current, which performs the real, kind-specific
// lookup via the GraphQL repo itself. Every other method panics if invoked,
// via the embedded (nil) interface — mirrors gqlKeyArtifactRepo's approach in
// graphql_apikey_test.go.
type mockGraphQLArtifactRepo struct {
	repository.ArtifactRepository
}

func (m *mockGraphQLArtifactRepo) GetByUUID(uuid, orgUUID string) (*model.Artifact, error) {
	return &model.Artifact{UUID: uuid, OrganizationUUID: orgUUID, Type: constants.GraphQLApi}, nil
}

// newGraphQLDeploymentTestService wires a GraphQLAPIDeploymentService for
// tests, reusing the shared mockDeploymentRepo (deployment_test.go) and
// mockGatewayRepository (gateway_properties_test.go) test doubles.
// gatewayEventsService is left nil, which is a supported no-op path (mirrors
// LLMProviderDeploymentService's "if s.gatewayEventsService != nil" guard),
// so tests don't need to stand up an EventHub.
func newGraphQLDeploymentTestService(repo *mockGraphQLAPIRepo, deploymentRepo *mockDeploymentRepo, gatewayRepo *mockGatewayRepository) *GraphQLAPIDeploymentService {
	// Default project handle used by tests that don't care about the specific
	// value — GetProjectByUUIDAndOrgID.OrganizationID left empty matches any
	// orgID (see mockProjectRepo, llm_test.go), which is fine since none of
	// these tests assert on the project handle carried into the deployment.
	projectRepo := &mockProjectRepo{project: &model.Project{Handle: "default-project"}}
	return NewGraphQLAPIDeploymentService(
		repo,
		deploymentRepo,
		gatewayRepo,
		&mockOrganizationRepo{},
		nil,
		&mockGraphQLArtifactRepo{},
		nil,
		NewArtifactDefinitions(NewGraphQLAPIDefinition(repo, projectRepo)),
		newTestIdentityService(),
		&config.Server{Deployments: config.Deployments{MaxPerAPIGateway: 20}},
		newTestLogger(),
		nil,
	)
}

func graphQLDeploymentTestGateway() *model.Gateway {
	return &model.Gateway{ID: "gw-uuid-1", OrganizationID: "org-1", Handle: "prod-gateway", Name: "Prod Gateway"}
}

// TestGraphQLDeployAPI_Current_Success exercises DeployGraphQLAPI's "current"
// base path end-to-end: resolves the gateway/API, generates the deployment
// YAML, persists the deployment record, and returns a DEPLOYING response.
func TestGraphQLDeployAPI_Current_Success(t *testing.T) {
	ctx := "/countries"
	stored := &model.GraphQLAPI{
		ID:             "gql-uuid-1",
		Handle:         "countries-graphql-api",
		OrganizationID: "org-1",
		Name:           "Countries GraphQL API",
		Version:        "v1.0",
		Configuration: model.GraphQLAPIConfig{
			SDL:     validCountriesGraphQLSDL,
			Context: &ctx,
			Upstream: model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{URL: "https://countries.example.com/graphql"},
			},
		},
	}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) { return stored, nil },
		getByUUIDFunc:   func(uuid, orgUUID string) (*model.GraphQLAPI, error) { return stored, nil },
	}
	gateway := graphQLDeploymentTestGateway()
	gatewayRepo := &mockGatewayRepository{getByNameResult: gateway, getByUUIDResult: gateway}
	deploymentRepo := &mockDeploymentRepo{setCurrentUpdatedAt: time.Now()}

	svc := newGraphQLDeploymentTestService(repo, deploymentRepo, gatewayRepo)

	req := &api.DeployRequest{Name: "prod-deployment", Base: "current", GatewayId: "prod-gateway"}
	resp, err := svc.DeployGraphQLAPI(context.Background(), "countries-graphql-api", req, "org-1", "creator-uuid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected a deployment response, got nil")
	}
	if resp.Name != "prod-deployment" {
		t.Errorf("expected deployment name %q, got %q", "prod-deployment", resp.Name)
	}
	if string(resp.Status) != string(model.DeploymentStatusDeploying) {
		t.Errorf("expected initial status DEPLOYING, got %s", resp.Status)
	}
	if resp.GatewayId != "prod-gateway" {
		t.Errorf("expected gatewayId %q (handle, not UUID), got %q", "prod-gateway", resp.GatewayId)
	}
	if !deploymentRepo.setCurrentCalled {
		t.Error("expected deployment status to be set")
	}
	created := deploymentRepo.createdDeployment
	if resp.BuildId == nil || created == nil || created.BuildID == nil || *resp.BuildId != *created.BuildID {
		t.Errorf("expected the response to name the build the deployment runs, got %v", resp.BuildId)
	}
}

// TestGraphQLDeployAPI_LTSGateway_Refused verifies that a deploy to an LTS
// gateway release, none of which has the GraphQLApi artifact kind
// (gwversion.MinGraphQLKindGatewayVersion), is refused with
// DEPLOYMENT_KIND_UNSUPPORTED_BY_GATEWAY before any deployment row is written
// or event sent, instead of shipping an artifact the gateway cannot run.
func TestGraphQLDeployAPI_LTSGateway_Refused(t *testing.T) {
	ctx := "/countries"
	stored := &model.GraphQLAPI{
		ID:             "gql-uuid-1",
		Handle:         "countries-graphql-api",
		OrganizationID: "org-1",
		Name:           "Countries GraphQL API",
		Version:        "v1.0",
		Configuration: model.GraphQLAPIConfig{
			SDL:     validCountriesGraphQLSDL,
			Context: &ctx,
			Upstream: model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{URL: "https://countries.example.com/graphql"},
			},
		},
	}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) { return stored, nil },
		getByUUIDFunc:   func(uuid, orgUUID string) (*model.GraphQLAPI, error) { return stored, nil },
	}
	for _, version := range []string{"1.0.0", "1.1.0", "1.2.0"} {
		t.Run(version, func(t *testing.T) {
			ltsGateway := &model.Gateway{ID: "gw-uuid-1", OrganizationID: "org-1", Handle: "prod-gateway", Name: "Prod Gateway", Version: version}
			gatewayRepo := &mockGatewayRepository{getByNameResult: ltsGateway, getByUUIDResult: ltsGateway}
			deploymentRepo := &mockDeploymentRepo{setCurrentUpdatedAt: time.Now()}

			svc := newGraphQLDeploymentTestService(repo, deploymentRepo, gatewayRepo)

			req := &api.DeployRequest{Name: "prod-deployment", Base: "current", GatewayId: "prod-gateway"}
			_, err := svc.DeployGraphQLAPI(context.Background(), "countries-graphql-api", req, "org-1", "creator-uuid")
			if err == nil {
				t.Fatalf("expected deploying to gateway %s to be refused", version)
			}
			if code := graphQLCatalogCode(t, err); code != apperror.CodeDeploymentKindUnsupportedByGateway {
				t.Errorf("expected %s, got %s", apperror.CodeDeploymentKindUnsupportedByGateway, code)
			}
			if deploymentRepo.createdDeployment != nil || deploymentRepo.setCurrentCalled {
				t.Error("expected nothing to be written for a refused deploy")
			}
		})
	}
}

// TestGraphQLDeployAPI_STSGateway_Allowed verifies a date-named (STS) gateway
// release, such as the cloud's, is treated as a current build and passes the
// GraphQLApi kind gate.
func TestGraphQLDeployAPI_STSGateway_Allowed(t *testing.T) {
	ctx := "/countries"
	stored := &model.GraphQLAPI{
		ID:             "gql-uuid-1",
		Handle:         "countries-graphql-api",
		OrganizationID: "org-1",
		Name:           "Countries GraphQL API",
		Version:        "v1.0",
		Configuration: model.GraphQLAPIConfig{
			SDL:     validCountriesGraphQLSDL,
			Context: &ctx,
			Upstream: model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{URL: "https://countries.example.com/graphql"},
			},
		},
	}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) { return stored, nil },
		getByUUIDFunc:   func(uuid, orgUUID string) (*model.GraphQLAPI, error) { return stored, nil },
	}
	stsGateway := &model.Gateway{ID: "gw-uuid-1", OrganizationID: "org-1", Handle: "prod-gateway", Name: "Prod Gateway", Version: "2026.09.24"}
	gatewayRepo := &mockGatewayRepository{getByNameResult: stsGateway, getByUUIDResult: stsGateway}
	deploymentRepo := &mockDeploymentRepo{setCurrentUpdatedAt: time.Now()}

	svc := newGraphQLDeploymentTestService(repo, deploymentRepo, gatewayRepo)

	req := &api.DeployRequest{Name: "prod-deployment", Base: "current", GatewayId: "prod-gateway"}
	if _, err := svc.DeployGraphQLAPI(context.Background(), "countries-graphql-api", req, "org-1", "creator-uuid"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deploymentRepo.createdDeployment == nil {
		t.Fatal("expected a deployment to be created")
	}
	if !strings.Contains(string(deploymentRepo.createdDeployment.Content), constants.GatewayApiVersion) {
		t.Errorf("expected deployment content to use %q for a current gateway", constants.GatewayApiVersion)
	}
}

// TestGraphQLDeployAPI_APINotFound verifies deploying a nonexistent GraphQL
// API returns GRAPHQL_API_NOT_FOUND rather than a generic error.
func TestGraphQLDeployAPI_APINotFound(t *testing.T) {
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) { return nil, nil },
	}
	gateway := graphQLDeploymentTestGateway()
	gatewayRepo := &mockGatewayRepository{getByNameResult: gateway, getByUUIDResult: gateway}
	svc := newGraphQLDeploymentTestService(repo, &mockDeploymentRepo{}, gatewayRepo)

	req := &api.DeployRequest{Name: "prod-deployment", Base: "current", GatewayId: "prod-gateway"}
	_, err := svc.DeployGraphQLAPI(context.Background(), "does-not-exist", req, "org-1", "creator-uuid")
	if err == nil {
		t.Fatal("expected an error deploying a nonexistent GraphQL API")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeGraphQLAPINotFound {
		t.Errorf("expected %s, got %s", apperror.CodeGraphQLAPINotFound, code)
	}
}

// TestGenerateGraphQLAPIDeploymentYAML_CarriesUpstreamAuth pins the fix the
// design doc explicitly calls for: REST's BuildAPIDeploymentYAML has a known,
// still-unfixed bug where dto.UpstreamTarget has no Auth field at all, so
// upstream.main.auth is silently dropped before the YAML ever reaches the
// gateway. GraphQLUpstreamTarget was built with an Auth field from day one to
// avoid copying that gap — this test is the regression guard proving the
// generator actually carries it through, not just that the field exists on
// the struct.
func TestGenerateGraphQLAPIDeploymentYAML_CarriesUpstreamAuth(t *testing.T) {
	ctx := "/countries"
	apiModel := &model.GraphQLAPI{
		ID:      "gql-uuid-1",
		Handle:  "countries-graphql-api",
		Name:    "Countries GraphQL API",
		Version: "v1.0",
		Configuration: model.GraphQLAPIConfig{
			Context: &ctx,
			Upstream: model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{
					URL: "https://countries.example.com/graphql",
					Auth: &model.UpstreamAuth{
						Type:   "apiKey",
						Header: "X-API-Key",
						Value:  "super-secret-value",
					},
				},
			},
		},
	}

	yamlData, err := generateGraphQLAPIDeploymentYAML(apiModel, "default-project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if yamlData.Spec.Upstream == nil || yamlData.Spec.Upstream.Main == nil {
		t.Fatal("expected spec.upstream.main to be populated")
	}
	auth := yamlData.Spec.Upstream.Main.Auth
	if auth == nil {
		t.Fatal("expected upstream.main.auth to be carried through into the deployment YAML, got nil")
	}
	if auth.Type != "apiKey" || auth.Header != "X-API-Key" || auth.Value != "super-secret-value" {
		t.Errorf("upstream.main.auth was not carried through unmodified: %+v", auth)
	}
}

// TestGenerateGraphQLAPIDeploymentYAML_CarriesSandboxAuth is the sandbox
// counterpart to TestGenerateGraphQLAPIDeploymentYAML_CarriesUpstreamAuth,
// asserting the generator itself (not just the full deploy flow) populates
// spec.upstream.sandbox from apiModel.Configuration.Upstream.Sandbox.
func TestGenerateGraphQLAPIDeploymentYAML_CarriesSandboxAuth(t *testing.T) {
	ctx := "/countries"
	apiModel := &model.GraphQLAPI{
		ID:      "gql-uuid-1",
		Handle:  "countries-graphql-api",
		Name:    "Countries GraphQL API",
		Version: "v1.0",
		Configuration: model.GraphQLAPIConfig{
			Context: &ctx,
			Upstream: model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{URL: "https://countries.example.com/graphql"},
				Sandbox: &model.UpstreamEndpoint{
					URL: "https://sandbox.countries.example.com/graphql",
					Auth: &model.UpstreamAuth{
						Type:   "bearer",
						Header: "Authorization",
						Value:  "sandbox-secret-value",
					},
				},
			},
		},
	}

	yamlData, err := generateGraphQLAPIDeploymentYAML(apiModel, "default-project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if yamlData.Spec.Upstream == nil || yamlData.Spec.Upstream.Sandbox == nil {
		t.Fatal("expected spec.upstream.sandbox to be populated")
	}
	sandbox := yamlData.Spec.Upstream.Sandbox
	if sandbox.URL != "https://sandbox.countries.example.com/graphql" {
		t.Errorf("sandbox.url = %q, want the configured sandbox URL", sandbox.URL)
	}
	if sandbox.Auth == nil {
		t.Fatal("expected upstream.sandbox.auth to be carried through into the deployment YAML, got nil")
	}
	if sandbox.Auth.Type != "bearer" || sandbox.Auth.Header != "Authorization" || sandbox.Auth.Value != "sandbox-secret-value" {
		t.Errorf("upstream.sandbox.auth was not carried through unmodified: %+v", sandbox.Auth)
	}
}

// TestGenerateGraphQLAPIDeploymentYAML_ProjectAnnotationIsHandleNotUUID pins
// the fix for a DP→CP round-trip bug: the project-id annotation this
// generator writes is read back on the artifact-import path
// (ResolveImportProject / artifact_import.go) via
// GetProjectByHandleAndOrgID, which has always expected a project *handle*.
// apiModel.ProjectID is the project's UUID (db:"project_uuid"), not its
// handle — passing it straight into the annotation made every DP→CP import
// of a bottom-up-pushed GraphQL artifact fail with PROJECT_NOT_FOUND. The
// caller now resolves the handle separately and passes it in explicitly; this
// test guards that the annotation actually holds that resolved handle, not
// apiModel.ProjectID.
func TestGenerateGraphQLAPIDeploymentYAML_ProjectAnnotationIsHandleNotUUID(t *testing.T) {
	ctx := "/countries"
	apiModel := &model.GraphQLAPI{
		ID:        "gql-uuid-1",
		Handle:    "countries-graphql-api",
		Name:      "Countries GraphQL API",
		Version:   "v1.0",
		ProjectID: "01a0ec74-a474-7d2d-98f7-b46b936969c3", // a UUID — must never end up in the annotation
		Configuration: model.GraphQLAPIConfig{
			Context: &ctx,
			Upstream: model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{URL: "https://countries.example.com/graphql"},
			},
		},
	}

	const projectHandle = "countries-project"
	yamlData, err := generateGraphQLAPIDeploymentYAML(apiModel, projectHandle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotAnnotation := yamlData.Metadata.Annotations[commonconstants.AnnotationProjectID]
	if gotAnnotation != projectHandle {
		t.Errorf("project-id annotation = %q, want the resolved handle %q", gotAnnotation, projectHandle)
	}
	if gotAnnotation == apiModel.ProjectID {
		t.Errorf("project-id annotation carried the raw UUID (%q) instead of the resolved handle", apiModel.ProjectID)
	}

	gotLabel := yamlData.Metadata.Labels[commonconstants.DeprecatedLabelProjectID]
	if gotLabel != projectHandle {
		t.Errorf("deprecated project-id label = %q, want the resolved handle %q", gotLabel, projectHandle)
	}
}

// TestGraphQLAPIDefinition_Current_AnnotatesProjectHandle guards the same fix on
// the path deployments actually take: builds and "current"-base deploys render
// through graphqlAPIDefinition.Current, which must resolve the API's project
// UUID to its handle before generating the YAML.
func TestGraphQLAPIDefinition_Current_AnnotatesProjectHandle(t *testing.T) {
	ctx := "/countries"
	apiModel := &model.GraphQLAPI{
		ID:        "gql-uuid-1",
		Handle:    "countries-graphql-api",
		Name:      "Countries GraphQL API",
		Version:   "v1.0",
		ProjectID: "01a0ec74-a474-7d2d-98f7-b46b936969c3",
		Configuration: model.GraphQLAPIConfig{
			Context: &ctx,
			Upstream: model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{URL: "https://countries.example.com/graphql"},
			},
		},
	}
	repo := &mockGraphQLAPIRepo{
		getByUUIDFunc: func(uuid, orgUUID string) (*model.GraphQLAPI, error) { return apiModel, nil },
	}
	projectRepo := &mockProjectRepo{project: &model.Project{Handle: "countries-project"}}

	snapshot, err := NewGraphQLAPIDefinition(repo, projectRepo).Current(
		&model.Artifact{UUID: apiModel.ID, OrganizationUUID: "org-1", Type: constants.GraphQLApi},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	definition, ok := snapshot.Definition.(*dto.GraphQLAPIDeploymentYAML)
	if !ok {
		t.Fatalf("definition is %T, want *dto.GraphQLAPIDeploymentYAML", snapshot.Definition)
	}
	if got := definition.Metadata.Annotations[commonconstants.AnnotationProjectID]; got != "countries-project" {
		t.Errorf("project-id annotation = %q, want the project handle %q", got, "countries-project")
	}
}

// TestGenerateGraphQLAPIDeploymentYAML_CarriesSandboxUpstream pins the fix for
// the gap where GraphQLUpstream had only a Main field: upstream.sandbox is a
// genuinely supported concept everywhere else (the gateway OpenAPI spec's
// GraphQLAPIConfigData.Upstream.Sandbox, GraphQLAPITransformer's sandbox
// route, and the CP's own read-response round-trip in
// TestGraphQLUpstreamAuth_RedactedAcrossAllResponseShapes), but the
// deployment YAML generator silently dropped it before it ever reached the
// gateway. This exercises the full DeployGraphQLAPI path (not just the
// generator) so it also proves gatewaytranslator.Translate still succeeds
// with a sandbox upstream present.
func TestGenerateGraphQLAPIDeploymentYAML_CarriesSandboxUpstream(t *testing.T) {
	ctx := "/countries"
	stored := &model.GraphQLAPI{
		ID:             "gql-uuid-1",
		Handle:         "countries-graphql-api",
		OrganizationID: "org-1",
		Name:           "Countries GraphQL API",
		Version:        "v1.0",
		Configuration: model.GraphQLAPIConfig{
			SDL:     validCountriesGraphQLSDL,
			Context: &ctx,
			Upstream: model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{URL: "https://countries.example.com/graphql"},
				Sandbox: &model.UpstreamEndpoint{
					URL: "https://sandbox.countries.example.com/graphql",
					Auth: &model.UpstreamAuth{
						Type:   "bearer",
						Header: "Authorization",
						Value:  "sandbox-secret",
					},
				},
			},
		},
	}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) { return stored, nil },
		getByUUIDFunc:   func(uuid, orgUUID string) (*model.GraphQLAPI, error) { return stored, nil },
	}
	gateway := graphQLDeploymentTestGateway()
	gatewayRepo := &mockGatewayRepository{getByNameResult: gateway, getByUUIDResult: gateway}
	deploymentRepo := &mockDeploymentRepo{setCurrentUpdatedAt: time.Now()}

	svc := newGraphQLDeploymentTestService(repo, deploymentRepo, gatewayRepo)

	req := &api.DeployRequest{Name: "prod-deployment", Base: "current", GatewayId: "prod-gateway"}
	if _, err := svc.DeployGraphQLAPI(context.Background(), "countries-graphql-api", req, "org-1", "creator-uuid"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deploymentRepo.createdDeployment == nil {
		t.Fatal("expected a deployment to be created")
	}
	content := string(deploymentRepo.createdDeployment.Content)
	if !strings.Contains(content, "sandbox.countries.example.com") {
		t.Errorf("expected deployment content to contain spec.upstream.sandbox.url, got:\n%s", content)
	}
}

// TestGraphQLUndeployDeployment_Success verifies an active deployment
// transitions to UNDEPLOYING when the bound gateway matches the request.
func TestGraphQLUndeployDeployment_Success(t *testing.T) {
	buildID := "2026-10-21-1"
	stored := &model.GraphQLAPI{ID: "gql-uuid-1", Handle: "countries-graphql-api", OrganizationID: "org-1"}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) { return stored, nil },
	}
	gateway := graphQLDeploymentTestGateway()
	gatewayRepo := &mockGatewayRepository{getByNameResult: gateway, getByUUIDResult: gateway}
	deployed := model.DeploymentStatusDeployed
	deploymentRepo := &mockDeploymentRepo{
		deploymentWithState: &model.Deployment{
			DeploymentID: "dep-1",
			Name:         "prod-deployment",
			ArtifactID:   stored.ID,
			GatewayID:    gateway.ID,
			BuildID:      &buildID,
			Status:       &deployed,
		},
		setCurrentUpdatedAt: time.Now(),
	}

	svc := newGraphQLDeploymentTestService(repo, deploymentRepo, gatewayRepo)

	resp, err := svc.UndeployGraphQLAPIDeployment(context.Background(), "countries-graphql-api", "dep-1", "prod-gateway", "org-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp.Status) != string(model.DeploymentStatusUndeploying) {
		t.Errorf("expected initial status UNDEPLOYING, got %s", resp.Status)
	}
	if deploymentRepo.setCurrentStatus != model.DeploymentStatusUndeploying {
		t.Errorf("expected repo to be asked to set status UNDEPLOYING, got %s", deploymentRepo.setCurrentStatus)
	}
	if resp.BuildId == nil || *resp.BuildId != buildID {
		t.Errorf("expected buildId %q, got %v", buildID, resp.BuildId)
	}
}

// TestGraphQLUndeployDeployment_GatewayMismatch_Rejected verifies a gatewayId
// that doesn't match the deployment's bound gateway is rejected — this
// prevents an unintended undeploy against the wrong gateway.
func TestGraphQLUndeployDeployment_GatewayMismatch_Rejected(t *testing.T) {
	stored := &model.GraphQLAPI{ID: "gql-uuid-1", Handle: "countries-graphql-api", OrganizationID: "org-1"}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) { return stored, nil },
	}
	boundGateway := graphQLDeploymentTestGateway()
	otherGateway := &model.Gateway{ID: "gw-uuid-2", OrganizationID: "org-1", Handle: "staging-gateway"}
	gatewayRepo := &mockGatewayRepository{getByNameResult: otherGateway, getByUUIDResult: boundGateway}
	deployed := model.DeploymentStatusDeployed
	deploymentRepo := &mockDeploymentRepo{
		deploymentWithState: &model.Deployment{
			DeploymentID: "dep-1",
			ArtifactID:   stored.ID,
			GatewayID:    boundGateway.ID,
			Status:       &deployed,
		},
	}

	svc := newGraphQLDeploymentTestService(repo, deploymentRepo, gatewayRepo)

	_, err := svc.UndeployGraphQLAPIDeployment(context.Background(), "countries-graphql-api", "dep-1", "staging-gateway", "org-1", "")
	if err == nil {
		t.Fatal("expected an error for a gateway mismatch")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeDeploymentGatewayMismatch {
		t.Errorf("expected %s, got %s", apperror.CodeDeploymentGatewayMismatch, code)
	}
	if deploymentRepo.setCurrentCalled {
		t.Error("expected no status change for a rejected gateway mismatch")
	}
}

// TestGraphQLRestoreDeployment_Success verifies restoring an UNDEPLOYED
// deployment transitions it back to DEPLOYING.
func TestGraphQLRestoreDeployment_Success(t *testing.T) {
	buildID := "2026-10-21-1"
	stored := &model.GraphQLAPI{ID: "gql-uuid-1", Handle: "countries-graphql-api", OrganizationID: "org-1"}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) { return stored, nil },
	}
	gateway := graphQLDeploymentTestGateway()
	gatewayRepo := &mockGatewayRepository{getByNameResult: gateway, getByUUIDResult: gateway}
	deploymentRepo := &mockDeploymentRepo{
		deploymentWithContent: &model.Deployment{
			DeploymentID: "dep-1",
			Name:         "prod-deployment",
			ArtifactID:   stored.ID,
			GatewayID:    gateway.ID,
			BuildID:      &buildID,
			Content:      []byte("apiVersion: v1"),
		},
		currentDeploymentID: "dep-0",
		currentStatus:       model.DeploymentStatusUndeployed,
		setCurrentUpdatedAt: time.Now(),
	}

	svc := newGraphQLDeploymentTestService(repo, deploymentRepo, gatewayRepo)

	resp, err := svc.RestoreGraphQLAPIDeployment(context.Background(), "countries-graphql-api", "dep-1", "prod-gateway", "org-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp.Status) != string(model.DeploymentStatusDeploying) {
		t.Errorf("expected initial status DEPLOYING, got %s", resp.Status)
	}
	if resp.BuildId == nil || *resp.BuildId != buildID {
		t.Errorf("expected buildId %q, got %v", buildID, resp.BuildId)
	}
}

// TestGraphQLRestoreDeployment_LTSGateway_Refused verifies a restore is gated on
// the GraphQLApi kind the same way a deploy is: an LTS gateway release is
// refused before the deployment is made current again.
func TestGraphQLRestoreDeployment_LTSGateway_Refused(t *testing.T) {
	stored := &model.GraphQLAPI{ID: "gql-uuid-1", Handle: "countries-graphql-api", OrganizationID: "org-1"}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) { return stored, nil },
	}
	gateway := graphQLDeploymentTestGateway()
	gateway.Version = "1.2.0"
	gatewayRepo := &mockGatewayRepository{getByNameResult: gateway, getByUUIDResult: gateway}
	deploymentRepo := &mockDeploymentRepo{
		deploymentWithContent: &model.Deployment{
			DeploymentID: "dep-1",
			Name:         "prod-deployment",
			ArtifactID:   stored.ID,
			GatewayID:    gateway.ID,
			Content:      []byte("apiVersion: v1"),
		},
		currentDeploymentID: "dep-0",
		currentStatus:       model.DeploymentStatusUndeployed,
		setCurrentUpdatedAt: time.Now(),
	}

	svc := newGraphQLDeploymentTestService(repo, deploymentRepo, gatewayRepo)

	_, err := svc.RestoreGraphQLAPIDeployment(context.Background(), "countries-graphql-api", "dep-1", "prod-gateway", "org-1", "")
	if err == nil {
		t.Fatal("expected restoring onto an LTS gateway to be refused")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeDeploymentKindUnsupportedByGateway {
		t.Errorf("expected %s, got %s", apperror.CodeDeploymentKindUnsupportedByGateway, code)
	}
	if deploymentRepo.setCurrentCalled {
		t.Error("expected the deployment not to be made current for a refused restore")
	}
}

// TestGraphQLRestoreDeployment_AlreadyDeployed_Conflict verifies restoring the
// deployment that is already the gateway's current, deployed one is rejected.
func TestGraphQLRestoreDeployment_AlreadyDeployed_Conflict(t *testing.T) {
	stored := &model.GraphQLAPI{ID: "gql-uuid-1", Handle: "countries-graphql-api", OrganizationID: "org-1"}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) { return stored, nil },
	}
	gateway := graphQLDeploymentTestGateway()
	gatewayRepo := &mockGatewayRepository{getByNameResult: gateway, getByUUIDResult: gateway}
	deploymentRepo := &mockDeploymentRepo{
		deploymentWithContent: &model.Deployment{
			DeploymentID: "dep-1",
			ArtifactID:   stored.ID,
			GatewayID:    gateway.ID,
			Content:      []byte("apiVersion: v1"),
		},
		currentDeploymentID: "dep-1",
		currentStatus:       model.DeploymentStatusDeployed,
	}

	svc := newGraphQLDeploymentTestService(repo, deploymentRepo, gatewayRepo)

	_, err := svc.RestoreGraphQLAPIDeployment(context.Background(), "countries-graphql-api", "dep-1", "prod-gateway", "org-1", "")
	if err == nil {
		t.Fatal("expected an error restoring an already-deployed deployment")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeDeploymentRestoreConflict {
		t.Errorf("expected %s, got %s", apperror.CodeDeploymentRestoreConflict, code)
	}
}
