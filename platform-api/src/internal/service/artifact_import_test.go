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
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"platform-api/src/config"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/database"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"

	_ "github.com/mattn/go-sqlite3"
	commonconstants "github.com/wso2/api-platform/common/constants"
)

// projectAnnotations builds the k8s-style metadata annotations that convey the project.
func projectAnnotations(project string) map[string]string {
	return map[string]string{commonconstants.AnnotationProjectID: project}
}

const (
	importTestOrgID     = "org-import-001"
	importTestProjectID = "project-import-001"
	importTestGatewayID = "gw-import-001"
)

// Fixed deployment timestamps used to express last-in-wins intent deterministically.
// A re-push that carries a DeployedAt strictly newer than the current watermark wins
// (its metadata is written); a re-push carrying an older/equal/nil DeployedAt is stale
// and leaves the working copy untouched.
var (
	baseDeployedAt      = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newerDeployedAt     = baseDeployedAt.Add(time.Hour)
	evenNewerDeployedAt = baseDeployedAt.Add(2 * time.Hour)
	olderDeployedAt     = baseDeployedAt.Add(-time.Hour)
)

// withDeployedAt returns a copy of req with its DeployedAt set to t.
func withDeployedAt(req dto.ImportGatewayArtifactRequest, t time.Time) dto.ImportGatewayArtifactRequest {
	req.DeployedAt = &t
	return req
}

// importTestDeps bundles the import service with the repos and db needed for assertions.
type importTestDeps struct {
	svc          *ArtifactImportService
	db           *database.DB
	artifactRepo repository.ArtifactRepository
	apiRepo      repository.APIRepository
	templateRepo repository.LLMProviderTemplateRepository
	deployment   repository.DeploymentRepository
}

func setupImportTest(t *testing.T) *importTestDeps {
	t.Helper()

	tmpDir := t.TempDir()
	sqlDB, err := sql.Open("sqlite3", filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable fks: %v", err)
	}
	db := &database.DB{DB: sqlDB}
	t.Cleanup(func() { db.Close() })

	schema, err := os.ReadFile(filepath.Join("..", "database", "schema.sqlite.sql"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if _, err := db.Exec(string(schema)); err != nil {
		t.Fatalf("apply schema: %v", err)
	}

	// Seed org, project, gateway.
	if _, err := db.Exec(`INSERT INTO organizations (uuid, handle, name, region, created_at, updated_at)
		VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))`,
		importTestOrgID, "h-"+importTestOrgID, "Import Org", "default"); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO projects (uuid, handle, name, organization_uuid, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		importTestProjectID, "default", "default", importTestOrgID, ""); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO gateways (uuid, organization_uuid, handle, name, description, vhost, properties, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		importTestGatewayID, importTestOrgID, "gw1", "Gateway 1", "", "gw.example.com", "{}"); err != nil {
		t.Fatalf("seed gateway: %v", err)
	}

	apiRepo := repository.NewAPIRepo(db)
	providerRepo := repository.NewLLMProviderRepo(db)
	templateRepo := repository.NewLLMProviderTemplateRepo(db)
	proxyRepo := repository.NewLLMProxyRepo(db)
	mcpProxyRepo := repository.NewMCPProxyRepo(db)
	artifactRepo := repository.NewArtifactRepo(db)
	deploymentRepo := repository.NewDeploymentRepo(db)
	gatewayRepo := repository.NewGatewayRepo(db)
	projectRepo := repository.NewProjectRepo(db)

	cfg := &config.Server{}
	cfg.Deployments.MaxPerAPIGateway = 10

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewArtifactImportService(apiRepo, providerRepo, templateRepo, proxyRepo, mcpProxyRepo,
		artifactRepo, deploymentRepo, gatewayRepo, projectRepo, cfg, logger, fakeMCPServerInfoFetcher{})

	return &importTestDeps{
		svc:          svc,
		db:           db,
		artifactRepo: artifactRepo,
		apiRepo:      apiRepo,
		templateRepo: templateRepo,
		deployment:   deploymentRepo,
	}
}

func restImportRequest(id, name, displayName string) dto.ImportGatewayArtifactRequest {
	return dto.ImportGatewayArtifactRequest{
		DPID:   id,
		Status: "deployed",
		Configuration: dto.ArtifactImportConfig{
			APIVersion: "api-platform.wso2.com/v1",
			Kind:       constants.RestApi,
			Metadata:   dto.ArtifactImportMetadata{Name: name, Annotations: projectAnnotations("default")},
			Spec: map[string]interface{}{
				"displayName": displayName,
				"version":     "v1.0",
				"context":     "/weather",
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func TestArtifactImport_CreateRestAPI(t *testing.T) {
	d := setupImportTest(t)

	const id = "11111111-1111-1111-1111-111111111111"
	resp, err := d.svc.Import(importTestOrgID, importTestGatewayID, restImportRequest(id, "weather-api", "Weather API"))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	// The control plane mints its own UUID; it must NOT reuse the data-plane UUID.
	if resp.ID == "" || resp.ID == id {
		t.Errorf("response ID = %q, want a freshly generated CP UUID (not the DP UUID %q)", resp.ID, id)
	}
	cpID := resp.ID
	if resp.Origin != constants.OriginDP {
		t.Errorf("response Origin = %q, want DP", resp.Origin)
	}
	if resp.Status != "deployed" {
		t.Errorf("response Status = %q, want deployed", resp.Status)
	}

	// Artifact row should exist with origin DP under the CP-generated UUID.
	art, err := d.artifactRepo.GetByUUID(cpID, importTestOrgID)
	if err != nil || art == nil {
		t.Fatalf("GetByUUID returned (%v, %v)", art, err)
	}
	if art.Origin != constants.OriginDP {
		t.Errorf("artifact origin = %q, want DP", art.Origin)
	}
	if art.Type != constants.RestApi {
		t.Errorf("artifact kind = %q, want RestApi", art.Type)
	}

	// Deployment status should be DEPLOYED on the gateway.
	depID, status, _, err := d.deployment.GetStatus(cpID, importTestOrgID, importTestGatewayID)
	if err != nil {
		t.Fatalf("GetStatus error = %v", err)
	}
	if depID == "" || status != model.DeploymentStatusDeployed {
		t.Errorf("deployment status = (%q,%q), want non-empty DEPLOYED", depID, status)
	}
}

func TestArtifactImport_UnsupportedKind(t *testing.T) {
	d := setupImportTest(t)
	req := restImportRequest("22222222-2222-2222-2222-222222222222", "x", "X")
	req.Configuration.Kind = "Nonexistent"

	_, err := d.svc.Import(importTestOrgID, importTestGatewayID, req)
	if !errors.Is(err, constants.ErrArtifactInvalidKind) {
		t.Fatalf("Import() error = %v, want ErrArtifactInvalidKind", err)
	}
}

func TestArtifactImport_MissingProjectForProjectScopedKind(t *testing.T) {
	d := setupImportTest(t)
	req := restImportRequest("33333333-3333-3333-3333-333333333333", "x", "X")
	req.Configuration.Metadata.Annotations = nil // REST requires a project

	_, err := d.svc.Import(importTestOrgID, importTestGatewayID, req)
	if !errors.Is(err, constants.ErrInvalidInput) {
		t.Fatalf("Import() error = %v, want ErrInvalidInput", err)
	}
}

func TestArtifactImport_NonexistentProject(t *testing.T) {
	d := setupImportTest(t)
	const id = "44444444-4444-4444-4444-444444444444"
	req := restImportRequest(id, "x", "X")
	req.Configuration.Metadata.Annotations = projectAnnotations("no-such-project")

	// Project provided but not present in the org -> the import fails with ErrProjectNotFound
	// and no artifact is created.
	if _, err := d.svc.Import(importTestOrgID, importTestGatewayID, req); !errors.Is(err, constants.ErrProjectNotFound) {
		t.Fatalf("Import() error = %v, want ErrProjectNotFound", err)
	}
	art, err := d.artifactRepo.GetByHandle("x", importTestOrgID)
	if err != nil {
		t.Fatalf("GetByHandle: %v", err)
	}
	if art != nil {
		t.Errorf("artifact was created despite the project not existing in the org")
	}
}

func TestArtifactImport_CPArtifactMetadataNotOverwritten(t *testing.T) {
	d := setupImportTest(t)

	const DPId = "55555555-5555-5555-5555-555555555556"
	// Pre-create a CP-originated REST API. The control plane mints its own UUID on
	// create, so capture the assigned UUID instead of assuming one.
	cpAPI := &model.API{
		Handle:          "cp-api",
		Name:            "Original CP Name",
		Version:         "v1.0",
		Kind:            constants.RestApi,
		ProjectID:       importTestProjectID,
		OrganizationID:  importTestOrgID,
		Origin:          constants.OriginCP,
		LifeCycleStatus: "CREATED",
		Configuration:   model.RestAPIConfig{Name: "Original CP Name", Version: "v1.0", Transport: []string{"https"}},
	}
	if err := d.apiRepo.CreateAPI(cpAPI); err != nil {
		t.Fatalf("seed CP api: %v", err)
	}
	cpID := cpAPI.ID // UUID assigned by the control plane on create

	// The gateway creates the same handle (with a different DP UUID and minor
	// differences) and pushes it to the control plane. Because the artifact's origin is
	// CP, its metadata must be preserved even though this gateway syncs metadata; the
	// push should only add a new deployment entry for the existing CP artifact.
	req := restImportRequest(DPId, "cp-api", "Hacked Name")
	resp, err := d.svc.Import(importTestOrgID, importTestGatewayID, req)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	// The push must re-attach to the existing CP artifact (matched by handle) rather
	// than mint a new artifact or adopt the data-plane UUID.
	if resp.ID != cpID {
		t.Errorf("import response ID = %q, want existing CP UUID %q", resp.ID, cpID)
	}

	art, _ := d.artifactRepo.GetByUUID(cpID, importTestOrgID)
	if art == nil {
		t.Fatal("artifact missing after import")
	}
	if art.Name != "Original CP Name" {
		t.Errorf("CP artifact name overwritten to %q; metadata must not change for origin CP", art.Name)
	}
	if art.Origin != constants.OriginCP {
		t.Errorf("origin changed to %q; must stay CP", art.Origin)
	}

	// A new deployment entry must have been added for the CP artifact on this gateway.
	depID, status, _, err := d.deployment.GetStatus(cpID, importTestOrgID, importTestGatewayID)
	if err != nil {
		t.Fatalf("GetStatus error = %v", err)
	}
	if depID == "" || status != model.DeploymentStatusDeployed {
		t.Errorf("deployment status = (%q, %q), want a new DEPLOYED entry for the CP artifact", depID, status)
	}
}

func TestArtifactImport_DPArtifactMetadataUpdatedOnNewerPush(t *testing.T) {
	d := setupImportTest(t)

	const id = "66666666-6666-6666-6666-666666666666"
	// First import creates the DP artifact and sets the metadata watermark.
	resp, err := d.svc.Import(importTestOrgID, importTestGatewayID,
		withDeployedAt(restImportRequest(id, "dp-api", "First Name"), baseDeployedAt))
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	cpID := resp.ID
	// Second import (same handle) carries a strictly newer DeployedAt, so it wins and
	// updates the working-copy display name (last-in-wins).
	if _, err := d.svc.Import(importTestOrgID, importTestGatewayID,
		withDeployedAt(restImportRequest(id, "dp-api", "Second Name"), newerDeployedAt)); err != nil {
		t.Fatalf("second import: %v", err)
	}

	art, _ := d.artifactRepo.GetByUUID(cpID, importTestOrgID)
	if art == nil || art.Name != "Second Name" {
		t.Errorf("DP artifact name = %v, want updated to 'Second Name' for a newer-DeployedAt push", art)
	}
}

func TestArtifactImport_DPArtifactMetadataNotUpdatedOnStalePush(t *testing.T) {
	d := setupImportTest(t)

	const id = "77777777-7777-7777-7777-777777777777"
	resp, err := d.svc.Import(importTestOrgID, importTestGatewayID,
		withDeployedAt(restImportRequest(id, "dp-api", "First Name"), baseDeployedAt))
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	cpID := resp.ID
	// The re-push carries an OLDER DeployedAt than the watermark, so it is stale: the
	// working copy/metadata must be left untouched.
	if _, err := d.svc.Import(importTestOrgID, importTestGatewayID,
		withDeployedAt(restImportRequest(id, "dp-api", "Second Name"), olderDeployedAt)); err != nil {
		t.Fatalf("second import: %v", err)
	}

	art, _ := d.artifactRepo.GetByUUID(cpID, importTestOrgID)
	if art == nil || art.Name != "First Name" {
		t.Errorf("DP artifact name = %v, want unchanged 'First Name' for a stale (older-DeployedAt) push", art)
	}
}

func TestArtifactImport_UndeployedStatus(t *testing.T) {
	d := setupImportTest(t)

	const id = "88888888-8888-8888-8888-888888888888"
	resp, err := d.svc.Import(importTestOrgID, importTestGatewayID, restImportRequest(id, "u-api", "U API"))
	if err != nil {
		t.Fatalf("create import: %v", err)
	}
	cpID := resp.ID
	// Push an undeployed status.
	req := restImportRequest(id, "u-api", "U API")
	req.Status = "undeployed"
	if _, err := d.svc.Import(importTestOrgID, importTestGatewayID, req); err != nil {
		t.Fatalf("undeploy import: %v", err)
	}

	_, status, _, err := d.deployment.GetStatus(cpID, importTestOrgID, importTestGatewayID)
	if err != nil {
		t.Fatalf("GetStatus error = %v", err)
	}
	if status != model.DeploymentStatusUndeployed {
		t.Errorf("status = %q, want UNDEPLOYED", status)
	}
	// Artifact must still exist.
	if art, _ := d.artifactRepo.GetByUUID(cpID, importTestOrgID); art == nil {
		t.Error("artifact was removed; it must remain in an undeployed state")
	}
}

func TestArtifactImport_LLMProviderTemplate_OrgLevelNoDeployment(t *testing.T) {
	d := setupImportTest(t)

	const id = "99999999-9999-9999-9999-999999999999"
	req := dto.ImportGatewayArtifactRequest{
		DPID:   id,
		Status: "deployed",
		Configuration: dto.ArtifactImportConfig{
			APIVersion: "api-platform.wso2.com/v1",
			Kind:       constants.LLMProviderTemplate,
			// No project — org-level kind.
			Metadata: dto.ArtifactImportMetadata{Name: "openai-template"},
			Spec:     map[string]interface{}{"displayName": "OpenAI Template"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	resp, err := d.svc.Import(importTestOrgID, importTestGatewayID, req)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if resp.Origin != constants.OriginDP {
		t.Errorf("response origin = %q, want DP", resp.Origin)
	}
	cpID := resp.ID

	tmpl, err := d.templateRepo.GetByUUID(cpID, importTestOrgID)
	if err != nil || tmpl == nil {
		t.Fatalf("template GetByUUID = (%v, %v)", tmpl, err)
	}
	if tmpl.Origin != constants.OriginDP {
		t.Errorf("template origin = %q, want DP", tmpl.Origin)
	}
	// Templates have no per-gateway deployment status.
	depID, _, _, err := d.deployment.GetStatus(cpID, importTestOrgID, importTestGatewayID)
	if err != nil {
		t.Fatalf("GetStatus error = %v", err)
	}
	if depID != "" {
		t.Errorf("template should not have a deployment status row, got deploymentID %q", depID)
	}
}

func TestArtifactImport_Enforcement_ReadOnlyAndDeletion(t *testing.T) {
	d := setupImportTest(t)

	const id = "b0000000-0000-0000-0000-000000000001"
	resp, err := d.svc.Import(importTestOrgID, importTestGatewayID, restImportRequest(id, "guarded-api", "Guarded API"))
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	cpID := resp.ID

	// The REST read path must surface origin=DP so the service guards can act on it.
	api, err := d.apiRepo.GetAPIByUUID(cpID, importTestOrgID)
	if err != nil || api == nil {
		t.Fatalf("GetAPIByUUID = (%v, %v)", api, err)
	}
	if api.Origin != constants.OriginDP {
		t.Fatalf("api.Origin = %q, want DP", api.Origin)
	}

	// Update of a DP artifact is blocked.
	if err := ensureOriginMutable(api.Origin); !errors.Is(err, constants.ErrArtifactReadOnly) {
		t.Errorf("ensureOriginMutable = %v, want ErrArtifactReadOnly", err)
	}

	// While deployed, deletion is blocked.
	active, err := d.deployment.HasActiveDeployment(cpID, importTestOrgID)
	if err != nil {
		t.Fatalf("HasActiveDeployment: %v", err)
	}
	if !active {
		t.Fatal("HasActiveDeployment = false, want true for a deployed artifact")
	}
	if err := ensureOriginDeletable(d.deployment, api.Origin, cpID, importTestOrgID); !errors.Is(err, constants.ErrArtifactDeployed) {
		t.Errorf("ensureOriginDeletable (deployed) = %v, want ErrArtifactDeployed", err)
	}

	// Once undeployed on the gateway, deletion is permitted.
	undeploy := restImportRequest(id, "guarded-api", "Guarded API")
	undeploy.Status = "undeployed"
	if _, err := d.svc.Import(importTestOrgID, importTestGatewayID, undeploy); err != nil {
		t.Fatalf("undeploy import: %v", err)
	}
	active, err = d.deployment.HasActiveDeployment(cpID, importTestOrgID)
	if err != nil {
		t.Fatalf("HasActiveDeployment after undeploy: %v", err)
	}
	if active {
		t.Fatal("HasActiveDeployment = true after undeploy, want false")
	}
	if err := ensureOriginDeletable(d.deployment, api.Origin, cpID, importTestOrgID); err != nil {
		t.Errorf("ensureOriginDeletable (undeployed) = %v, want nil", err)
	}
}

func TestArtifactGuard_CPArtifactUnaffected(t *testing.T) {
	// CP-origin artifacts are never read-only and not subject to the DP deletion guard.
	if err := ensureOriginMutable(constants.OriginCP); err != nil {
		t.Errorf("ensureOriginMutable(CP) = %v, want nil", err)
	}
	d := setupImportTest(t)
	if err := ensureOriginDeletable(d.deployment, constants.OriginCP, "any", importTestOrgID); err != nil {
		t.Errorf("ensureOriginDeletable(CP) = %v, want nil", err)
	}
}

func TestArtifactImport_AllSupportedKindsRegistered(t *testing.T) {
	d := setupImportTest(t)
	for _, kind := range []string{
		constants.RestApi,
		constants.LLMProvider,
		constants.LLMProviderTemplate,
		constants.LLMProxy,
		constants.MCPProxy,
	} {
		importer, ok := d.svc.importers[kind]
		if !ok {
			t.Errorf("no importer registered for kind %q", kind)
			continue
		}
		if importer.Kind() != kind {
			t.Errorf("importer for %q reports Kind() = %q", kind, importer.Kind())
		}
	}
}

func TestArtifactImport_ProxyMissingProvider(t *testing.T) {
	d := setupImportTest(t)

	req := dto.ImportGatewayArtifactRequest{
		DPID:   "c0000000-0000-0000-0000-0000000000ff",
		Status: "deployed",
		Configuration: dto.ArtifactImportConfig{
			APIVersion: "api-platform.wso2.com/v1",
			Kind:       constants.LLMProxy,
			Metadata:   dto.ArtifactImportMetadata{Name: "orphan-proxy", Annotations: projectAnnotations("default")},
			Spec:       map[string]interface{}{"provider": map[string]interface{}{"id": "does-not-exist-handle"}},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err := d.svc.Import(importTestOrgID, importTestGatewayID, req)
	if !errors.Is(err, constants.ErrInvalidInput) {
		t.Fatalf("Import() error = %v, want ErrInvalidInput for a missing provider reference", err)
	}
}

func TestArtifactImport_UndeployThenRedeploy(t *testing.T) {
	d := setupImportTest(t)

	const id = "ab000000-0000-0000-0000-000000000001"

	// 1. Initial deploy (sets the metadata watermark).
	resp, err := d.svc.Import(importTestOrgID, importTestGatewayID,
		withDeployedAt(restImportRequest(id, "rd-api", "Original Name"), baseDeployedAt))
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	cpID := resp.ID
	dep1, st1, _, err := d.deployment.GetStatus(cpID, importTestOrgID, importTestGatewayID)
	if err != nil || dep1 == "" || st1 != model.DeploymentStatusDeployed {
		t.Fatalf("after deploy: dep=%q status=%q err=%v", dep1, st1, err)
	}

	// 2. Gateway delete -> undeploy push: artifact stays, status flips to UNDEPLOYED,
	//    no new deployment, metadata untouched. The data-plane UUID is regenerated on
	//    recreate, so the undeploy is matched by handle, not UUID.
	undeploy := restImportRequest("ab000000-0000-0000-0000-0000000000ff", "rd-api", "Hacked On Undeploy")
	undeploy.Status = "undeployed"
	undeployResp, err := d.svc.Import(importTestOrgID, importTestGatewayID, undeploy)
	if err != nil {
		t.Fatalf("undeploy: %v", err)
	}
	// Undeploy must resolve to the same CP artifact (by handle) and echo its CP UUID.
	if undeployResp.ID != cpID {
		t.Errorf("undeploy resolved CP ID = %q, want %q (matched by handle)", undeployResp.ID, cpID)
	}
	depU, stU, _, _ := d.deployment.GetStatus(cpID, importTestOrgID, importTestGatewayID)
	if stU != model.DeploymentStatusUndeployed {
		t.Errorf("after undeploy status = %q, want UNDEPLOYED", stU)
	}
	if depU != dep1 {
		t.Errorf("undeploy must not create a new deployment (dep1=%q depU=%q)", dep1, depU)
	}
	art, _ := d.artifactRepo.GetByUUID(cpID, importTestOrgID)
	if art == nil {
		t.Fatal("artifact was deleted on undeploy; it must remain")
	}
	if art.Name != "Original Name" {
		t.Errorf("undeploy changed metadata name to %q; metadata must be untouched", art.Name)
	}

	// 3. Re-create on the gateway with a NEW data-plane UUID and re-push: the CP finds
	//    the undeployed artifact by handle, reuses its CP UUID, creates a NEW deployment
	//    (status DEPLOYED), and updates metadata because this push carries a newer DeployedAt.
	redeploy := withDeployedAt(restImportRequest("ab000000-0000-0000-0000-00000000aaaa", "rd-api", "Updated Name"), newerDeployedAt)
	redeployResp, err := d.svc.Import(importTestOrgID, importTestGatewayID, redeploy)
	if err != nil {
		t.Fatalf("redeploy: %v", err)
	}
	if redeployResp.ID != cpID {
		t.Errorf("redeploy CP ID = %q, want reuse of %q (matched by handle)", redeployResp.ID, cpID)
	}
	dep2, st2, _, _ := d.deployment.GetStatus(cpID, importTestOrgID, importTestGatewayID)
	if st2 != model.DeploymentStatusDeployed {
		t.Errorf("after redeploy status = %q, want DEPLOYED", st2)
	}
	if dep2 == "" || dep2 == dep1 {
		t.Errorf("redeploy must create a new deployment (dep1=%q dep2=%q)", dep1, dep2)
	}
	art2, _ := d.artifactRepo.GetByUUID(cpID, importTestOrgID)
	if art2.Name != "Updated Name" {
		t.Errorf("redeploy with a newer DeployedAt did not update metadata name: got %q", art2.Name)
	}
}

func TestArtifactImport_RedeployNoMetadataUpdateOnStalePush(t *testing.T) {
	d := setupImportTest(t)

	const id = "ab000000-0000-0000-0000-000000000002"
	resp, err := d.svc.Import(importTestOrgID, importTestGatewayID,
		withDeployedAt(restImportRequest(id, "rd2-api", "Original Name"), baseDeployedAt))
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	cpID := resp.ID
	// Re-deploy with a different display name but an OLDER DeployedAt (stale); metadata
	// must NOT change.
	if _, err := d.svc.Import(importTestOrgID, importTestGatewayID,
		withDeployedAt(restImportRequest(id, "rd2-api", "Updated Name"), olderDeployedAt)); err != nil {
		t.Fatalf("redeploy: %v", err)
	}
	art, _ := d.artifactRepo.GetByUUID(cpID, importTestOrgID)
	if art == nil || art.Name != "Original Name" {
		t.Errorf("metadata changed despite a stale (older-DeployedAt) push: got %v", art)
	}
}

func TestArtifactImport_GatewayNotFound(t *testing.T) {
	d := setupImportTest(t)
	_, err := d.svc.Import(importTestOrgID, "nonexistent-gateway", restImportRequest("a0000000-0000-0000-0000-000000000000", "x", "X"))
	if !errors.Is(err, constants.ErrGatewayNotFound) {
		t.Fatalf("Import() error = %v, want ErrGatewayNotFound", err)
	}
}
