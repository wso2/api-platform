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
	buildTestBuildID     = "44444444-4444-4444-4444-4444444444aa"
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

	build        *model.Build
	createdBuild *model.Build
	builds       []*model.Build

	// baseDeployment is what a base id resolves to as a DEPLOYMENT; nil means the
	// id is not a deployment, which is what sends the lookup on to builds.
	baseDeployment *model.Deployment
	created        *model.Deployment
}

func (m *buildTestDeploymentRepo) CreateBuild(build *model.Build) error {
	if build.BuildID == "" {
		build.BuildID = buildTestBuildID
	}
	m.createdBuild = build
	return nil
}

func (m *buildTestDeploymentRepo) GetBuild(buildID, artifactUUID, orgUUID string) (*model.Build, error) {
	if m.build != nil && m.build.BuildID == buildID {
		return m.build, nil
	}
	return nil, nil
}

func (m *buildTestDeploymentRepo) GetBuilds(artifactUUID, orgUUID string, limit int) ([]*model.Build, error) {
	return m.builds, nil
}

func (m *buildTestDeploymentRepo) GetWithContent(deploymentID, artifactUUID, orgUUID string) (*model.Deployment, error) {
	return m.baseDeployment, nil
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

	build, err := service.CreateBuild(buildTestAPIUUID, buildTestOrgUUID, "tester")
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
	if build.BuildId.String() == "" {
		t.Error("no build id was returned")
	}
}

func TestCreateBuild_APINotFound(t *testing.T) {
	service := newBuildTestService(&buildTestAPIRepo{apiModel: nil}, &buildTestDeploymentRepo{})

	if _, err := service.CreateBuild(buildTestAPIUUID, buildTestOrgUUID, "tester"); err == nil {
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
			BuildID:     buildTestBuildID,
			ArtifactID:  buildTestAPIUUID,
			Content:     []byte(snapshot),
			DataVersion: "1.0",
		},
	}
	service := newBuildTestService(&buildTestAPIRepo{apiModel: buildTestAPI()}, depRepo)

	deployment, err := service.DeployAPI(buildTestAPIUUID, &api.DeployRequest{
		Name:      "orders-dev",
		Base:      buildTestBuildID,
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
	// A deployment made from a build names it, so what is running can be traced
	// back to the snapshot it came from.
	if depRepo.created.Metadata[constants.MetadataKeyBuildID] != buildTestBuildID {
		t.Errorf("buildId metadata = %v, want %q",
			depRepo.created.Metadata[constants.MetadataKeyBuildID], buildTestBuildID)
	}
	// A build is not a deployment, so it is not recorded as the base deployment.
	if depRepo.created.BaseDeploymentID != nil {
		t.Errorf("baseDeploymentId = %v, want nil for a build base", *depRepo.created.BaseDeploymentID)
	}
	if deployment == nil {
		t.Fatal("no deployment was returned")
	}
}

// A base that is neither a deployment nor a build must be rejected rather than
// silently falling back to rendering the current definition.
func TestDeployAPI_UnknownBaseIsRejected(t *testing.T) {
	service := newBuildTestService(&buildTestAPIRepo{apiModel: buildTestAPI()}, &buildTestDeploymentRepo{})

	_, err := service.DeployAPI(buildTestAPIUUID, &api.DeployRequest{
		Name:      "orders-dev",
		Base:      "99999999-9999-9999-9999-999999999999",
		GatewayId: "test-gateway",
	}, buildTestOrgUUID, "tester")
	if err == nil || !apperror.DeploymentBaseNotFound.Is(err) {
		t.Fatalf("expected DeploymentBaseNotFound, got %v", err)
	}
}

// A promotion runs the base deployment's artifact, so it runs the base's build:
// the id has to survive the promotion or the trace stops at the first one.
func TestDeployAPI_PromotionCarriesTheBuildIDForward(t *testing.T) {
	const snapshot = "apiVersion: gateway.wso2.com/v1\nkind: RestApi\nmetadata:\n  name: orders-api\nspec:\n  context: /orders\n"
	depRepo := &buildTestDeploymentRepo{
		baseDeployment: &model.Deployment{
			DeploymentID: "33333333-3333-3333-3333-3333333333aa",
			ArtifactID:   buildTestAPIUUID,
			GatewayID:    buildTestGatewayUUID,
			Content:      []byte(snapshot),
			Metadata:     map[string]any{constants.MetadataKeyBuildID: buildTestBuildID},
		},
	}
	service := newBuildTestService(&buildTestAPIRepo{apiModel: buildTestAPI()}, depRepo)

	_, err := service.DeployAPI(buildTestAPIUUID, &api.DeployRequest{
		Name:      "orders-prod",
		Base:      "33333333-3333-3333-3333-3333333333aa",
		GatewayId: "test-gateway",
	}, buildTestOrgUUID, "tester")
	if err != nil {
		t.Fatalf("DeployAPI: %v", err)
	}
	if depRepo.created.Metadata[constants.MetadataKeyBuildID] != buildTestBuildID {
		t.Errorf("promoted deployment lost the build id: %v",
			depRepo.created.Metadata[constants.MetadataKeyBuildID])
	}
	if depRepo.created.BaseDeploymentID == nil {
		t.Error("a promotion should record the deployment it came from")
	}
}
