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
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// newGraphQLDeploymentTestService wires a GraphQLAPIDeploymentService for
// tests, reusing the shared mockDeploymentRepo (deployment_test.go) and
// mockGatewayRepository (gateway_properties_test.go) test doubles.
// gatewayEventsService is left nil, which is a supported no-op path (mirrors
// LLMProviderDeploymentService's "if s.gatewayEventsService != nil" guard),
// so tests don't need to stand up an EventHub.
func newGraphQLDeploymentTestService(repo *mockGraphQLAPIRepo, deploymentRepo *mockDeploymentRepo, gatewayRepo *mockGatewayRepository) *GraphQLAPIDeploymentService {
	return NewGraphQLAPIDeploymentService(
		repo,
		deploymentRepo,
		gatewayRepo,
		&mockOrganizationRepo{},
		nil,
		nil,
		&config.Server{Deployments: config.Deployments{MaxPerAPIGateway: 20}},
		newTestLogger(),
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
	}
	gateway := graphQLDeploymentTestGateway()
	gatewayRepo := &mockGatewayRepository{getByNameResult: gateway, getByUUIDResult: gateway}
	deploymentRepo := &mockDeploymentRepo{setCurrentUpdatedAt: time.Now()}

	svc := newGraphQLDeploymentTestService(repo, deploymentRepo, gatewayRepo)

	req := &api.DeployRequest{Name: "prod-deployment", Base: "current", GatewayId: "prod-gateway"}
	resp, err := svc.DeployGraphQLAPI("countries-graphql-api", req, "org-1", "creator-uuid")
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
}

// TestGraphQLDeployAPI_LegacyGateway_DownConvertsApiVersion pins the fix for
// the gap found auditing deployments/gateways/api-keys wiring for GraphQL:
// DeployGraphQLAPI previously stamped constants.GatewayApiVersion
// unconditionally and never called gatewaytranslator.Translate, so a
// gateway older than gatewaytranslator.MinGatewayV1Version ("1.2.0") would
// silently receive a v1 artifact it can't parse — unlike RestApi, MCP, and
// LLM Provider/Proxy, which all down-convert via Translate before deploying.
func TestGraphQLDeployAPI_LegacyGateway_DownConvertsApiVersion(t *testing.T) {
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
	}
	// Below gatewaytranslator.MinGatewayV1Version ("1.2.0") — must down-convert.
	legacyGateway := &model.Gateway{ID: "gw-uuid-1", OrganizationID: "org-1", Handle: "prod-gateway", Name: "Prod Gateway", Version: "1.1.0"}
	gatewayRepo := &mockGatewayRepository{getByNameResult: legacyGateway, getByUUIDResult: legacyGateway}
	deploymentRepo := &mockDeploymentRepo{setCurrentUpdatedAt: time.Now()}

	svc := newGraphQLDeploymentTestService(repo, deploymentRepo, gatewayRepo)

	req := &api.DeployRequest{Name: "prod-deployment", Base: "current", GatewayId: "prod-gateway"}
	if _, err := svc.DeployGraphQLAPI("countries-graphql-api", req, "org-1", "creator-uuid"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deploymentRepo.createdDeployment == nil {
		t.Fatal("expected a deployment to be created")
	}
	content := string(deploymentRepo.createdDeployment.Content)
	if !strings.Contains(content, constants.GatewayApiVersionV1Alpha1) {
		t.Errorf("expected deployment content to use %q for a legacy gateway, got:\n%s", constants.GatewayApiVersionV1Alpha1, content)
	}
	if strings.Contains(content, constants.GatewayApiVersion+"\n") {
		t.Errorf("expected deployment content NOT to use latest %q for a legacy gateway, got:\n%s", constants.GatewayApiVersion, content)
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
	_, err := svc.DeployGraphQLAPI("does-not-exist", req, "org-1", "creator-uuid")
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

	yamlData, err := generateGraphQLAPIDeploymentYAML(apiModel)
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

	yamlData, err := generateGraphQLAPIDeploymentYAML(apiModel)
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
	}
	gateway := graphQLDeploymentTestGateway()
	gatewayRepo := &mockGatewayRepository{getByNameResult: gateway, getByUUIDResult: gateway}
	deploymentRepo := &mockDeploymentRepo{setCurrentUpdatedAt: time.Now()}

	svc := newGraphQLDeploymentTestService(repo, deploymentRepo, gatewayRepo)

	req := &api.DeployRequest{Name: "prod-deployment", Base: "current", GatewayId: "prod-gateway"}
	if _, err := svc.DeployGraphQLAPI("countries-graphql-api", req, "org-1", "creator-uuid"); err != nil {
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
			Status:       &deployed,
		},
		setCurrentUpdatedAt: time.Now(),
	}

	svc := newGraphQLDeploymentTestService(repo, deploymentRepo, gatewayRepo)

	resp, err := svc.UndeployGraphQLAPIDeployment("countries-graphql-api", "dep-1", "prod-gateway", "org-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp.Status) != string(model.DeploymentStatusUndeploying) {
		t.Errorf("expected initial status UNDEPLOYING, got %s", resp.Status)
	}
	if deploymentRepo.setCurrentStatus != model.DeploymentStatusUndeploying {
		t.Errorf("expected repo to be asked to set status UNDEPLOYING, got %s", deploymentRepo.setCurrentStatus)
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

	_, err := svc.UndeployGraphQLAPIDeployment("countries-graphql-api", "dep-1", "staging-gateway", "org-1")
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
			Content:      []byte("apiVersion: v1"),
		},
		currentDeploymentID: "dep-0",
		currentStatus:       model.DeploymentStatusUndeployed,
		setCurrentUpdatedAt: time.Now(),
	}

	svc := newGraphQLDeploymentTestService(repo, deploymentRepo, gatewayRepo)

	resp, err := svc.RestoreGraphQLAPIDeployment("countries-graphql-api", "dep-1", "prod-gateway", "org-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp.Status) != string(model.DeploymentStatusDeploying) {
		t.Errorf("expected initial status DEPLOYING, got %s", resp.Status)
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

	_, err := svc.RestoreGraphQLAPIDeployment("countries-graphql-api", "dep-1", "prod-gateway", "org-1")
	if err == nil {
		t.Fatal("expected an error restoring an already-deployed deployment")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeDeploymentRestoreConflict {
		t.Errorf("expected %s, got %s", apperror.CodeDeploymentRestoreConflict, code)
	}
}
