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
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

const (
	buildTestOrgUUID     = "00000000-0000-0000-0000-0000000000aa"
	buildTestAPIUUID     = "11111111-1111-1111-1111-1111111111aa"
	buildTestGatewayUUID = "22222222-2222-2222-2222-2222222222aa"
	buildTestBuildID     = "2026-01-31-2"
	buildTestBuildUUID   = "55555555-5555-5555-5555-5555555555aa"
)

// buildTestAPIRepo serves one API and accepts gateway associations.
type buildTestAPIRepo struct {
	repository.APIRepository
	apiModel *model.API
}

func (m *buildTestAPIRepo) GetAPIByUUID(uuid, orgUUID string) (*model.API, error) {
	return m.apiModel, nil
}

func (m *buildTestAPIRepo) GetAPIAssociations(apiUUID, associationType, orgUUID string) ([]*model.APIAssociation, error) {
	return nil, nil
}

func (m *buildTestAPIRepo) CreateAPIAssociation(association *model.APIAssociation) error {
	return nil
}

// buildTestDeploymentRepo records builds and deployments it is asked to create.
type buildTestDeploymentRepo struct {
	repository.DeploymentRepository

	build          *model.Build
	createdBuild   *model.Build
	createdWithCap int
	builds         []*model.Build
	getBuildCalls  int

	// baseDeployment is what a deployment id resolves to, for the tests that prove
	// naming one is no longer a way to deploy.
	baseDeployment      *model.Deployment
	getWithContentCalls int
	created             *model.Deployment
}

func (m *buildTestDeploymentRepo) CreateBuildWithLimitEnforcement(build *model.Build, hardLimit int) error {
	if build.BuildID == "" {
		build.BuildID = buildTestBuildID
	}
	m.createdBuild = build
	m.createdWithCap = hardLimit
	return nil
}

func (m *buildTestDeploymentRepo) GetBuild(buildID, artifactUUID, orgUUID string) (*model.Build, error) {
	m.getBuildCalls++
	if m.build != nil && m.build.BuildID == buildID {
		return m.build, nil
	}
	return nil, nil
}

func (m *buildTestDeploymentRepo) GetBuilds(artifactUUID, orgUUID string, limit int) ([]*model.Build, error) {
	return m.builds, nil
}

func (m *buildTestDeploymentRepo) GetWithContent(deploymentID, artifactUUID, orgUUID string) (*model.Deployment, error) {
	m.getWithContentCalls++
	return m.baseDeployment, nil
}

func (m *buildTestDeploymentRepo) CreateWithBuild(deployment *model.Deployment, build *model.Build,
	buildHardLimit, hardLimit int) error {
	if build.UUID == "" {
		build.UUID = buildTestBuildUUID
	}
	if build.BuildID == "" {
		build.BuildID = buildTestBuildID
	}
	m.createdBuild = build
	m.createdWithCap = buildHardLimit
	// As the real write does: the deployment's reference comes from the build it
	// has just stored.
	deployment.BuildUUID = &build.UUID
	deployment.BuildID = &build.BuildID
	return m.CreateWithLimitEnforcement(deployment, hardLimit)
}

func (m *buildTestDeploymentRepo) CreateWithLimitEnforcement(deployment *model.Deployment, hardLimit int) error {
	m.created = deployment
	return nil
}

func (m *buildTestDeploymentRepo) SetCurrentWithDetails(artifactUUID, orgUUID, gatewayID, deploymentID string,
	status model.DeploymentStatus, statusDesired string, performedAt *time.Time, statusReason string) (time.Time, error) {
	return time.Time{}, nil
}

// buildTestGatewayRepo serves one gateway by handle and by uuid.
type buildTestGatewayRepo struct {
	repository.GatewayRepository
	gateway *model.Gateway
}

func (m *buildTestGatewayRepo) GetByHandleAndOrgID(handle, orgUUID string) (*model.Gateway, error) {
	return m.gateway, nil
}

func (m *buildTestGatewayRepo) GetByUUID(gatewayID string) (*model.Gateway, error) {
	return m.gateway, nil
}

func newBuildTestService(apiRepo *buildTestAPIRepo, depRepo *buildTestDeploymentRepo) *DeploymentService {
	return &DeploymentService{
		apiRepo:        apiRepo,
		deploymentRepo: depRepo,
		gatewayRepo: &buildTestGatewayRepo{gateway: &model.Gateway{
			ID:      buildTestGatewayUUID,
			Handle:  "test-gateway",
			Version: "1.0.0",
		}},
		apiUtil: &utils.APIUtil{},
		cfg:     &testConfig,
		slogger: slog.Default(),
	}
}

func buildTestAPI() *model.API {
	return &model.API{
		ID:          buildTestAPIUUID,
		Handle:      "orders-api",
		Kind:        constants.RestApi,
		DataVersion: "1.0",
	}
}

// A build is a snapshot of the definition as it stands now, stored at the
// platform's own data version — it is not translated, because the gateway it will
// be deployed to is not known yet.
func TestCreateBuild_StoresASnapshotAtThePlatformDataVersion(t *testing.T) {
	depRepo := &buildTestDeploymentRepo{}
	service := newBuildTestService(&buildTestAPIRepo{apiModel: buildTestAPI()}, depRepo)

	build, err := service.CreateBuild(buildTestAPIUUID, buildTestOrgUUID, "tester", nil)
	if err != nil {
		t.Fatalf("CreateBuild: %v", err)
	}
	if depRepo.createdBuild == nil {
		t.Fatal("no build was stored")
	}
	if len(depRepo.createdBuild.Content) == 0 {
		t.Error("the stored build has no rendered content")
	}
	if depRepo.createdBuild.DataVersion != "1.0" {
		t.Errorf("data version = %q, want the API's own 1.0", depRepo.createdBuild.DataVersion)
	}
	if depRepo.createdBuild.ArtifactID != buildTestAPIUUID ||
		depRepo.createdBuild.OrganizationID != buildTestOrgUUID {
		t.Error("the build is not scoped to the API and organization")
	}
	if depRepo.createdBuild.CreatedBy != "tester" {
		t.Errorf("createdBy = %q", depRepo.createdBuild.CreatedBy)
	}
	if build.BuildId == "" {
		t.Error("no build id was returned")
	}
	// The configured cap reaches the store, which is what prunes the API's older
	// unused builds as this one is added.
	if depRepo.createdWithCap != testConfig.Deployments.MaxBuildsPerAPI {
		t.Errorf("stored with cap %d, want the configured %d",
			depRepo.createdWithCap, testConfig.Deployments.MaxBuildsPerAPI)
	}
}

// The metadata bag travels with the build and is handed back untouched, which is
// what lets a caller record where a build came from — a commit, for an API kept in
// a repository — and read it off the build later.
func TestCreateBuild_RecordsTheGivenMetadata(t *testing.T) {
	depRepo := &buildTestDeploymentRepo{}
	service := newBuildTestService(&buildTestAPIRepo{apiModel: buildTestAPI()}, depRepo)

	build, err := service.CreateBuild(buildTestAPIUUID, buildTestOrgUUID, "tester",
		map[string]interface{}{"commitId": "9f1c2ab"})
	if err != nil {
		t.Fatalf("CreateBuild: %v", err)
	}
	if depRepo.createdBuild.Metadata["commitId"] != "9f1c2ab" {
		t.Errorf("stored metadata = %v, want the commit recorded",
			depRepo.createdBuild.Metadata)
	}
	if build.Metadata == nil || (*build.Metadata)["commitId"] != "9f1c2ab" {
		t.Errorf("returned metadata = %v, want the commit reported back", build.Metadata)
	}
}

func TestCreateBuild_APINotFound(t *testing.T) {
	service := newBuildTestService(&buildTestAPIRepo{apiModel: nil}, &buildTestDeploymentRepo{})

	if _, err := service.CreateBuild(buildTestAPIUUID, buildTestOrgUUID, "tester", nil); err == nil {
		t.Fatal("expected an error for an API that does not exist")
	}
}

func TestGetBuild_NotFound(t *testing.T) {
	service := newBuildTestService(&buildTestAPIRepo{apiModel: buildTestAPI()}, &buildTestDeploymentRepo{})

	_, err := service.GetBuild(buildTestAPIUUID, buildTestBuildID, buildTestOrgUUID)
	if err == nil || !apperror.BuildNotFound.Is(err) {
		t.Fatalf("expected BuildNotFound, got %v", err)
	}
}

// The point of preparing: deploying a build sends THAT snapshot, not a fresh
// rendering of whatever the API's definition has become since.
func TestDeployAPI_FromABuild_SendsTheStoredSnapshot(t *testing.T) {
	const snapshot = "apiVersion: gateway.wso2.com/v1\nkind: RestApi\nmetadata:\n  name: orders-api\nspec:\n  context: /orders\n"
	depRepo := &buildTestDeploymentRepo{
		build: &model.Build{
			UUID:        buildTestBuildUUID,
			BuildID:     buildTestBuildID,
			ArtifactID:  buildTestAPIUUID,
			Content:     []byte(snapshot),
			DataVersion: "1.0",
		},
	}
	service := newBuildTestService(&buildTestAPIRepo{apiModel: buildTestAPI()}, depRepo)

	deployment, err := service.DeployAPI(buildTestAPIUUID, &api.DeployRequest{
		Name:      "orders-dev",
		Base:      "build",
		BuildId:   ptr(buildTestBuildID),
		GatewayId: "test-gateway",
	}, buildTestOrgUUID, "tester")
	if err != nil {
		t.Fatalf("DeployAPI: %v", err)
	}
	if depRepo.created == nil {
		t.Fatal("no deployment was created")
	}
	if !strings.Contains(string(depRepo.created.Content), "/orders") {
		t.Errorf("the deployment does not carry the build's artifact: %s", depRepo.created.Content)
	}
	// A deployment made from a build references the build row, which is the only
	// record of where it came from — and what makes "which deployments came from
	// this build" answerable, and pruning able to tell what is still in use.
	if depRepo.created.BuildUUID == nil || *depRepo.created.BuildUUID != buildTestBuildUUID {
		t.Errorf("buildUuid = %v, want %q", depRepo.created.BuildUUID, buildTestBuildUUID)
	}
	if depRepo.created.BuildID == nil || *depRepo.created.BuildID != buildTestBuildID {
		t.Errorf("buildId = %v, want %q", depRepo.created.BuildID, buildTestBuildID)
	}
	// The readable id lives with the build, never copied into the deployment's
	// metadata, so the two can never drift apart.
	if _, ok := depRepo.created.Metadata["buildId"]; ok {
		t.Error("the build id was copied into deployment metadata")
	}
	// A build is not a deployment, so it is not recorded as the base deployment.
	if depRepo.created.BaseDeploymentID != nil {
		t.Errorf("baseDeploymentId = %v, want nil for a build base", *depRepo.created.BaseDeploymentID)
	}
	if deployment == nil {
		t.Fatal("no deployment was returned")
	}
}

// Deploying from the definition is not deploying WITHOUT a build: the render is
// stored as one and the deployment runs that, so what a gateway is serving is
// always traceable to a snapshot, and the next environment has something to
// promote. The build is written with the deployment, not before it.
func TestDeployAPI_FromTheDefinitionStoresTheBuildItRuns(t *testing.T) {
	depRepo := &buildTestDeploymentRepo{}
	service := newBuildTestService(&buildTestAPIRepo{apiModel: buildTestAPI()}, depRepo)

	deployment, err := service.DeployAPI(buildTestAPIUUID, &api.DeployRequest{
		Name:      "orders-dev",
		Base:      "current",
		GatewayId: "test-gateway",
	}, buildTestOrgUUID, "tester")
	if err != nil {
		t.Fatalf("DeployAPI: %v", err)
	}
	if deployment == nil || depRepo.created == nil {
		t.Fatal("no deployment was created")
	}
	if len(depRepo.created.Content) == 0 {
		t.Error("the deployment carries no artifact")
	}
	if depRepo.createdBuild == nil {
		t.Fatal("no build was stored for a deployment rendered from the definition")
	}
	if depRepo.created.BuildUUID == nil || *depRepo.created.BuildUUID != depRepo.createdBuild.UUID {
		t.Errorf("buildUuid = %v, want the stored build %q",
			depRepo.created.BuildUUID, depRepo.createdBuild.UUID)
	}
	if deployment.BuildId == nil || *deployment.BuildId != depRepo.createdBuild.BuildID {
		t.Errorf("response buildId = %v, want %q", deployment.BuildId, depRepo.createdBuild.BuildID)
	}
	// The build takes its place in the API's budget exactly as a prepared one does.
	if depRepo.createdWithCap != testConfig.Deployments.MaxBuildsPerAPI {
		t.Errorf("build limit = %d, want the configured %d",
			depRepo.createdWithCap, testConfig.Deployments.MaxBuildsPerAPI)
	}
	// Nothing to look up: the build is the one this deploy just rendered.
	if depRepo.getBuildCalls != 0 {
		t.Errorf("builds were read %d time(s); deploying from the definition must not need them",
			depRepo.getBuildCalls)
	}
}

// The build is the definition as it stood, not one deployment's customization of
// it: a deployment's overrides belong to that deployment, so the next environment
// promoting this build is not silently given this gateway's endpoint.
func TestDeployAPI_OverridesDoNotReachTheBuild(t *testing.T) {
	apiModel := buildTestAPI()
	apiModel.Configuration.Upstream.Main = &model.UpstreamEndpoint{URL: "http://orders.internal:8080"}
	depRepo := &buildTestDeploymentRepo{}
	service := newBuildTestService(&buildTestAPIRepo{apiModel: apiModel}, depRepo)

	_, err := service.DeployAPI(buildTestAPIUUID, &api.DeployRequest{
		Name:      "orders-dev",
		Base:      "current",
		GatewayId: "test-gateway",
		Metadata:  &map[string]interface{}{"endpointUrl": "https://orders-dev.example.com"},
	}, buildTestOrgUUID, "tester")
	if err != nil {
		t.Fatalf("DeployAPI: %v", err)
	}
	if depRepo.createdBuild == nil || depRepo.created == nil {
		t.Fatal("the deploy stored no build")
	}
	if !strings.Contains(string(depRepo.created.Content), "orders-dev.example.com") {
		t.Errorf("the deployment did not take the override: %s", depRepo.created.Content)
	}
	if strings.Contains(string(depRepo.createdBuild.Content), "orders-dev.example.com") {
		t.Errorf("the override reached the build: %s", depRepo.createdBuild.Content)
	}
	if !strings.Contains(string(depRepo.createdBuild.Content), "orders.internal:8080") {
		t.Errorf("the build is not the definition as it stood: %s", depRepo.createdBuild.Content)
	}
}

// The explicit field says what the value is, instead of leaving the server to
// guess whether an id names a deployment or a build.
func TestDeployAPI_BuildIdNamesTheBuildDirectly(t *testing.T) {
	const snapshot = "apiVersion: gateway.wso2.com/v1\nkind: RestApi\nmetadata:\n  name: orders-api\nspec:\n  context: /orders\n"
	depRepo := &buildTestDeploymentRepo{
		build: &model.Build{
			UUID:        buildTestBuildUUID,
			BuildID:     buildTestBuildID,
			ArtifactID:  buildTestAPIUUID,
			Content:     []byte(snapshot),
			DataVersion: "1.0",
		},
	}
	service := newBuildTestService(&buildTestAPIRepo{apiModel: buildTestAPI()}, depRepo)

	_, err := service.DeployAPI(buildTestAPIUUID, &api.DeployRequest{
		Name:      "orders-prod",
		Base:      "build",
		BuildId:   ptr(buildTestBuildID),
		GatewayId: "test-gateway",
	}, buildTestOrgUUID, "tester")
	if err != nil {
		t.Fatalf("DeployAPI: %v", err)
	}
	// Named the build, so it ships that snapshot rather than rendering the
	// definition again.
	if !strings.Contains(string(depRepo.created.Content), "/orders") {
		t.Errorf("the deployment does not carry the build's artifact: %s", depRepo.created.Content)
	}
	if depRepo.created.BuildUUID == nil || *depRepo.created.BuildUUID != buildTestBuildUUID {
		t.Errorf("buildUuid = %v, want %q", depRepo.created.BuildUUID, buildTestBuildUUID)
	}
}

// base and buildId have to agree: "build" without one leaves nothing to resolve.
func TestDeployAPI_BuildBaseRequiresABuildId(t *testing.T) {
	service := newBuildTestService(&buildTestAPIRepo{apiModel: buildTestAPI()}, &buildTestDeploymentRepo{})

	_, err := service.DeployAPI(buildTestAPIUUID, &api.DeployRequest{
		Name:      "orders-prod",
		Base:      "build",
		GatewayId: "test-gateway",
	}, buildTestOrgUUID, "tester")
	if err == nil {
		t.Fatal("expected base 'build' without a buildId to be rejected")
	}
}

// And the other way: a buildId sent with base "current" would be silently ignored
// — the deploy renders its own build — so the request is refused rather than
// quietly deploying something else.
func TestDeployAPI_BuildIdIsRejectedWithBaseCurrent(t *testing.T) {
	depRepo := &buildTestDeploymentRepo{}
	service := newBuildTestService(&buildTestAPIRepo{apiModel: buildTestAPI()}, depRepo)

	_, err := service.DeployAPI(buildTestAPIUUID, &api.DeployRequest{
		Name:      "orders-prod",
		Base:      "current",
		BuildId:   ptr(buildTestBuildID),
		GatewayId: "test-gateway",
	}, buildTestOrgUUID, "tester")
	if err == nil {
		t.Fatal("expected a buildId alongside base 'current' to be rejected")
	}
	if depRepo.created != nil {
		t.Error("a deployment was created from a request that should not have been accepted")
	}
}

// base is what says where the artifact comes from, so it is always required.
func TestDeployAPI_BaseIsRequired(t *testing.T) {
	service := newBuildTestService(&buildTestAPIRepo{apiModel: buildTestAPI()}, &buildTestDeploymentRepo{})

	_, err := service.DeployAPI(buildTestAPIUUID, &api.DeployRequest{
		Name:      "orders-prod",
		GatewayId: "test-gateway",
	}, buildTestOrgUUID, "tester")
	if err == nil {
		t.Fatal("expected a request without a base to be rejected")
	}
}

// Naming a build that does not exist is a build error, not a base error: the
// caller said what it was passing, so the answer can say so too.
func TestDeployAPI_UnknownBuildIdIsRejected(t *testing.T) {
	service := newBuildTestService(&buildTestAPIRepo{apiModel: buildTestAPI()}, &buildTestDeploymentRepo{})

	_, err := service.DeployAPI(buildTestAPIUUID, &api.DeployRequest{
		Name:      "orders-prod",
		Base:      "build",
		BuildId:   ptr("2099-01-01-9"),
		GatewayId: "test-gateway",
	}, buildTestOrgUUID, "tester")
	if err == nil || !apperror.BuildNotFound.Is(err) {
		t.Fatalf("expected BuildNotFound, got %v", err)
	}
}

// base names one of two sources and nothing else. An id here used to mean "promote
// that deployment"; it is now refused, so a caller that has not been updated is
// told rather than quietly given a rendering of the current definition.
func TestDeployAPI_ADeploymentIdIsNotABase(t *testing.T) {
	depRepo := &buildTestDeploymentRepo{
		// A real deployment, to show that even a resolvable id is not a base.
		baseDeployment: &model.Deployment{
			DeploymentID: "33333333-3333-3333-3333-3333333333aa",
			ArtifactID:   buildTestAPIUUID,
			GatewayID:    buildTestGatewayUUID,
			Content:      []byte("apiVersion: gateway.wso2.com/v1\nkind: RestApi\n"),
		},
	}
	service := newBuildTestService(&buildTestAPIRepo{apiModel: buildTestAPI()}, depRepo)

	_, err := service.DeployAPI(buildTestAPIUUID, &api.DeployRequest{
		Name:      "orders-prod",
		Base:      "33333333-3333-3333-3333-3333333333aa",
		GatewayId: "test-gateway",
	}, buildTestOrgUUID, "tester")
	if err == nil || !apperror.RESTAPIDeploymentValidationFailed.Is(err) {
		t.Fatalf("expected a validation failure, got %v", err)
	}
	if depRepo.created != nil {
		t.Error("a deployment was created from a base that is no longer accepted")
	}
	if depRepo.getWithContentCalls != 0 {
		t.Errorf("deployments were read %d time(s); a deploymentId base is refused outright",
			depRepo.getWithContentCalls)
	}
}
