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

package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

const (
	buildRepoAPIUUID = "aaaaaaaa-0000-0000-0000-00000000000b"
	buildRepoOrgUUID = "aaaaaaaa-0000-0000-0000-00000000000c"
)

// buildOn returns a build of the test API prepared at the given instant.
func buildOn(day time.Time) *model.Build {
	return &model.Build{
		ArtifactID:     buildRepoAPIUUID,
		OrganizationID: buildRepoOrgUUID,
		Content:        []byte("apiVersion: gateway.wso2.com/v1\nkind: RestApi\n"),
		DataVersion:    "1.0",
		CreatedBy:      "tester",
		CreatedAt:      day,
	}
}

// A build id is meant to be readable and said out loud: the day it was prepared
// and that day's index for the API. The index restarts with each date.
func TestCreateBuild_IDIsTheDateAndThatDaysIndex(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	first := time.Date(2026, 1, 31, 9, 0, 0, 0, time.UTC)
	for _, want := range []string{"2026-01-31-1", "2026-01-31-2", "2026-01-31-3"} {
		build := buildOn(first)
		if err := repo.CreateBuildWithLimitEnforcement(build, 0); err != nil {
			t.Fatalf("CreateBuild: %v", err)
		}
		if build.BuildID != want {
			t.Fatalf("build id = %q, want %q", build.BuildID, want)
		}
	}

	nextDay := buildOn(time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC))
	if err := repo.CreateBuildWithLimitEnforcement(nextDay, 0); err != nil {
		t.Fatalf("CreateBuild: %v", err)
	}
	if nextDay.BuildID != "2026-02-01-1" {
		t.Errorf("build id = %q, want the index to restart on a new date", nextDay.BuildID)
	}
}

// The id is unique per API, not globally, so two APIs prepared on the same day
// both start at index 1 — which is what keeps the id short enough to be readable.
func TestCreateBuild_IndexIsPerAPI(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	const otherAPIUUID = "aaaaaaaa-0000-0000-0000-00000000000d"
	insertBuildTestArtifact(t, db, otherAPIUUID, buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	day := time.Date(2026, 1, 31, 9, 0, 0, 0, time.UTC)
	mine := buildOn(day)
	if err := repo.CreateBuildWithLimitEnforcement(mine, 0); err != nil {
		t.Fatalf("CreateBuild: %v", err)
	}
	theirs := buildOn(day)
	theirs.ArtifactID = otherAPIUUID
	if err := repo.CreateBuildWithLimitEnforcement(theirs, 0); err != nil {
		t.Fatalf("CreateBuild: %v", err)
	}
	if mine.BuildID != "2026-01-31-1" || theirs.BuildID != "2026-01-31-1" {
		t.Errorf("ids = %q and %q, want each API to start at index 1",
			mine.BuildID, theirs.BuildID)
	}
}

// The snapshot and its metadata come back exactly as stored: a build is what a
// deploy sends, so anything lost here would silently change what runs.
func TestGetBuild_ReturnsTheSnapshotAndItsMetadata(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	stored := buildOn(time.Date(2026, 1, 31, 9, 0, 0, 0, time.UTC))
	stored.Metadata = map[string]any{"commitId": "9f1c2ab"}
	if err := repo.CreateBuildWithLimitEnforcement(stored, 0); err != nil {
		t.Fatalf("CreateBuild: %v", err)
	}

	read, err := repo.GetBuild(stored.BuildID, buildRepoAPIUUID, buildRepoOrgUUID)
	if err != nil {
		t.Fatalf("GetBuild: %v", err)
	}
	if read == nil {
		t.Fatal("the build was not found")
	}
	if string(read.Content) != string(stored.Content) {
		t.Error("the stored snapshot did not come back unchanged")
	}
	if read.Metadata["commitId"] != "9f1c2ab" {
		t.Errorf("metadata = %v, want the commit that was recorded", read.Metadata)
	}
	if read.DataVersion != "1.0" || read.CreatedBy != "tester" {
		t.Errorf("build = %+v, want its data version and author preserved", read)
	}
}

// A build id belongs to one API. Resolving it under another API must miss, because
// that scoping is what stops one API's build being deployed as another's.
func TestGetBuild_IsScopedToItsAPI(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	const otherAPIUUID = "aaaaaaaa-0000-0000-0000-00000000000d"
	insertBuildTestArtifact(t, db, otherAPIUUID, buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	stored := buildOn(time.Date(2026, 1, 31, 9, 0, 0, 0, time.UTC))
	if err := repo.CreateBuildWithLimitEnforcement(stored, 0); err != nil {
		t.Fatalf("CreateBuild: %v", err)
	}

	read, err := repo.GetBuild(stored.BuildID, otherAPIUUID, buildRepoOrgUUID)
	if err != nil {
		t.Fatalf("GetBuild: %v", err)
	}
	if read != nil {
		t.Error("a build resolved under an API it does not belong to")
	}
}

// A listing is for choosing what to deploy, so it is newest first and carries no
// artifacts.
func TestGetBuilds_NewestFirstWithoutContent(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	day := time.Date(2026, 1, 31, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		if err := repo.CreateBuildWithLimitEnforcement(buildOn(day.Add(time.Duration(i)*time.Hour)), 0); err != nil {
			t.Fatalf("CreateBuild: %v", err)
		}
	}

	builds, err := repo.GetBuilds(buildRepoAPIUUID, buildRepoOrgUUID, 0)
	if err != nil {
		t.Fatalf("GetBuilds: %v", err)
	}
	if len(builds) != 3 {
		t.Fatalf("got %d builds, want 3", len(builds))
	}
	if builds[0].BuildID != "2026-01-31-3" {
		t.Errorf("first build = %q, want the newest", builds[0].BuildID)
	}
	if len(builds[0].Content) != 0 {
		t.Error("a listing should not carry the rendered artifact")
	}
}

// insertBuildTestArtifact adds a second artifact under the test organization, so per-API
// scoping can be asserted without a full API row.
func insertBuildTestArtifact(t *testing.T, db *database.DB, artifactUUID, orgUUID string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES (?, ?, ?)`,
		artifactUUID, "RestApi", orgUUID)
	if err != nil {
		t.Fatalf("Failed to create artifact: %v", err)
	}
}

// deployFromBuild makes one gateway's CURRENT deployment come from a build, which
// is what makes that build in use: a status row is what marks a deployment as the
// one a gateway is serving, and build_uuid is what says where it came from.
func deployFromBuild(t *testing.T, db *database.DB, gatewayUUID, deploymentID string, build *model.Build) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO deployments (uuid, display_name, artifact_uuid, organization_uuid, gateway_uuid, build_uuid, content, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		deploymentID, deploymentID, buildRepoAPIUUID, buildRepoOrgUUID, gatewayUUID,
		build.UUID, []byte("content"), time.Now().UTC())
	if err != nil {
		t.Fatalf("Failed to insert deployment: %v", err)
	}
	_, err = db.Exec(`
		REPLACE INTO deployment_status (artifact_uuid, organization_uuid, gateway_uuid, deployment_uuid, status, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		buildRepoAPIUUID, buildRepoOrgUUID, gatewayUUID, deploymentID, "DEPLOYED", time.Now().UTC())
	if err != nil {
		t.Fatalf("Failed to set deployment status: %v", err)
	}
}

// storedBuildIDs lists what the API has kept, oldest first.
func storedBuildIDs(t *testing.T, repo DeploymentRepository) []string {
	t.Helper()
	builds, err := repo.GetBuilds(buildRepoAPIUUID, buildRepoOrgUUID, 0)
	if err != nil {
		t.Fatalf("GetBuilds: %v", err)
	}
	out := make([]string, 0, len(builds))
	for i := len(builds) - 1; i >= 0; i-- {
		out = append(out, builds[i].BuildID)
	}
	return out
}

// prepareBuilds adds n builds an hour apart, oldest first.
func prepareBuilds(t *testing.T, repo DeploymentRepository, n, hardLimit int) []*model.Build {
	t.Helper()
	day := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	builds := make([]*model.Build, 0, n)
	for i := 0; i < n; i++ {
		build := buildOn(day.Add(time.Duration(i) * time.Hour))
		if err := repo.CreateBuildWithLimitEnforcement(build, hardLimit); err != nil {
			t.Fatalf("CreateBuildWithLimitEnforcement: %v", err)
		}
		builds = append(builds, build)
	}
	return builds
}

// A deploy from the API's definition renders the build and stores it with the
// deployment that runs it, on one transaction. Nothing can prune a build the
// deployment naming it does not yet exist to protect, and the deployment cannot end
// up naming a build that was never recorded.
func TestCreateDeployment_StoresTheBuildItRunsAlongsideIt(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	createTestGateway(t, db, "gw-1", buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	build := buildOn(time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC))
	build.BuildID = ""
	deployed := model.DeploymentStatusDeployed
	deployment := &model.Deployment{
		DeploymentID:   "dep-1",
		Name:           "dep-1",
		ArtifactID:     buildRepoAPIUUID,
		GatewayID:      "gw-1",
		OrganizationID: buildRepoOrgUUID,
		Content:        []byte("content"),
		Status:         &deployed,
	}
	if err := repo.CreateWithBuild(deployment, build, 0, 100); err != nil {
		t.Fatalf("CreateWithBuild: %v", err)
	}

	// The build was given an id of its own and the deployment names it.
	if build.BuildID != "2026-01-31-1" {
		t.Errorf("buildId = %q, want %q", build.BuildID, "2026-01-31-1")
	}
	if deployment.BuildUUID == nil || *deployment.BuildUUID != build.UUID {
		t.Errorf("deployment buildUuid = %v, want %q", deployment.BuildUUID, build.UUID)
	}
	stored, err := repo.GetBuild(build.BuildID, buildRepoAPIUUID, buildRepoOrgUUID)
	if err != nil || stored == nil {
		t.Fatalf("the build was not stored: %v", err)
	}
	dep, err := repo.GetWithContent("dep-1", buildRepoAPIUUID, buildRepoOrgUUID)
	if err != nil {
		t.Fatalf("GetWithContent: %v", err)
	}
	if dep.BuildID == nil || *dep.BuildID != build.BuildID {
		t.Errorf("buildId = %v, want %q", dep.BuildID, build.BuildID)
	}
}

// The other half of committing them together: a deploy that cannot be recorded
// leaves no build behind either. A build nothing deployed would otherwise sit in the
// API's budget and be offered as something to deploy.
func TestCreateDeployment_AFailedDeployStoresNoBuild(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	build := buildOn(time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC))
	build.BuildID = ""
	deployed := model.DeploymentStatusDeployed
	// The gateway does not exist, so recording the deployment fails.
	err := repo.CreateWithBuild(&model.Deployment{
		DeploymentID:   "dep-1",
		Name:           "dep-1",
		ArtifactID:     buildRepoAPIUUID,
		GatewayID:      "gw-that-was-never-created",
		OrganizationID: buildRepoOrgUUID,
		Content:        []byte("content"),
		Status:         &deployed,
	}, build, 0, 100)
	if err == nil {
		t.Fatal("CreateWithBuild succeeded against a gateway that does not exist")
	}

	if kept := storedBuildIDs(t, repo); len(kept) != 0 {
		t.Errorf("builds = %v, want none stored by a deploy that failed", kept)
	}
}

// A deploy of a build prepared earlier resolves it before the transaction that
// records the deployment opens, so a prepare running alongside can prune it in
// between. The build is read again inside that transaction, so the deploy is refused
// rather than committed with an origin it has lost — the next stage promotes what
// this one is running, and a deployment that cannot name its build ends the pipeline.
func TestCreateDeployment_RefusesABuildPrunedMidDeploy(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	createTestGateway(t, db, "gw-1", buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	build := buildOn(time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC))
	if err := repo.CreateBuildWithLimitEnforcement(build, 0); err != nil {
		t.Fatalf("CreateBuildWithLimitEnforcement: %v", err)
	}
	// The prepare that raced this deploy, pruning the build it had already resolved.
	if _, err := db.Exec(`DELETE FROM builds WHERE uuid = ?`, build.UUID); err != nil {
		t.Fatalf("prune the build: %v", err)
	}

	deployed := model.DeploymentStatusDeployed
	err := repo.CreateWithLimitEnforcement(&model.Deployment{
		DeploymentID:   "dep-1",
		Name:           "dep-1",
		ArtifactID:     buildRepoAPIUUID,
		GatewayID:      "gw-1",
		OrganizationID: buildRepoOrgUUID,
		Content:        []byte("content"),
		Status:         &deployed,
		BuildUUID:      &build.UUID,
	}, 100)
	if err == nil {
		t.Fatal("expected a deploy of a pruned build to be refused")
	}
	if !apperror.BuildNotFound.Is(err) {
		t.Errorf("error = %v, want BuildNotFound", err)
	}
	if dep, err := repo.GetWithContent("dep-1", buildRepoAPIUUID, buildRepoOrgUUID); err == nil && dep != nil {
		t.Error("the deployment was recorded anyway")
	}
}

// A deploy that fails for a reason of its own must not be reported as a lost
// build: only the build being gone means that. Here the build is intact and the
// gateway does not exist, so the foreign-key failure belongs to the gateway and is
// surfaced as itself.
func TestCreateDeployment_KeepsAnUnrelatedFailureAsItself(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	build := buildOn(time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC))
	if err := repo.CreateBuildWithLimitEnforcement(build, 0); err != nil {
		t.Fatalf("CreateBuildWithLimitEnforcement: %v", err)
	}

	deployed := model.DeploymentStatusDeployed
	err := repo.CreateWithLimitEnforcement(&model.Deployment{
		DeploymentID:   "dep-1",
		Name:           "dep-1",
		ArtifactID:     buildRepoAPIUUID,
		GatewayID:      "gw-that-was-never-created",
		OrganizationID: buildRepoOrgUUID,
		Content:        []byte("content"),
		Status:         &deployed,
		BuildUUID:      &build.UUID,
	}, 100)
	if err == nil {
		t.Fatal("CreateWithLimitEnforcement succeeded against a gateway that does not exist")
	}
	if apperror.BuildNotFound.Is(err) {
		t.Errorf("err = %v, want the underlying failure rather than a lost build", err)
	}
}

// A reference that cannot be resolved at all is refused, not quietly cleared:
// reaching this means an invariant broke upstream, which is worth failing over
// rather than hiding behind a deployment with no origin.
func TestCreateDeployment_RefusesAnUnresolvableBuildReference(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	createTestGateway(t, db, "gw-1", buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	deployed := model.DeploymentStatusDeployed
	missing := "99999999-9999-9999-9999-999999999999"
	err := repo.CreateWithLimitEnforcement(&model.Deployment{
		DeploymentID:   "dep-1",
		Name:           "dep-1",
		ArtifactID:     buildRepoAPIUUID,
		GatewayID:      "gw-1",
		OrganizationID: buildRepoOrgUUID,
		Content:        []byte("content"),
		Status:         &deployed,
		BuildUUID:      &missing,
	}, 100)
	if err == nil {
		t.Fatal("expected a reference that cannot be resolved to be refused")
	}
	if !apperror.BuildNotFound.Is(err) {
		t.Errorf("error = %v, want BuildNotFound", err)
	}
}

// The foreign key alone would accept any build. A deployment carrying another
// API's build would report that build's id as its own origin, so the reference is
// checked against the deployment's own API and organization, not just for existence.
func TestCreateDeployment_RefusesABuildFromAnotherAPI(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	createTestGateway(t, db, "gw-1", buildRepoOrgUUID)
	otherAPIUUID := "aaaaaaaa-0000-0000-0000-00000000000e"
	insertBuildTestArtifact(t, db, otherAPIUUID, buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	// A real build, but prepared for a different API.
	foreign := buildOn(time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC))
	foreign.ArtifactID = otherAPIUUID
	if err := repo.CreateBuildWithLimitEnforcement(foreign, 0); err != nil {
		t.Fatalf("CreateBuildWithLimitEnforcement: %v", err)
	}

	deployed := model.DeploymentStatusDeployed
	err := repo.CreateWithLimitEnforcement(&model.Deployment{
		DeploymentID:   "dep-1",
		Name:           "dep-1",
		ArtifactID:     buildRepoAPIUUID,
		GatewayID:      "gw-1",
		OrganizationID: buildRepoOrgUUID,
		Content:        []byte("content"),
		Status:         &deployed,
		BuildUUID:      &foreign.UUID,
	}, 100)
	if err == nil {
		t.Fatal("expected a build belonging to another API to be refused")
	}
	if !apperror.BuildNotFound.Is(err) {
		t.Errorf("error = %v, want BuildNotFound", err)
	}
}

// Pruning and adding are one transaction, so an attempt that cannot finish takes
// nothing with it. Without that, a failed prepare would still have spent five of
// the API's builds — and worse, could delete a build a concurrent deploy had just
// resolved and was about to reference.
func TestCreateBuild_AFailedAttemptPrunesNothing(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	builds := prepareBuilds(t, repo, 10, 0)
	before := storedBuildIDs(t, repo)

	// At the limit, so this prepare prunes first — and then fails, because the id
	// it was given belongs to the newest build, which pruning does not reach.
	doomed := buildOn(time.Date(2026, 1, 31, 10, 0, 0, 0, time.UTC))
	doomed.BuildID = builds[len(builds)-1].BuildID
	if err := repo.CreateBuildWithLimitEnforcement(doomed, 10); err == nil {
		t.Fatal("expected the duplicate build id to be rejected")
	}

	after := storedBuildIDs(t, repo)
	if !reflect.DeepEqual(before, after) {
		t.Errorf("builds after the failed attempt = %v, want them untouched: %v", after, before)
	}
}

// Reaching the limit removes the API's oldest free build — exactly the one slot the
// new build needs — so the table stays at the limit rather than sawing down to well
// under it every time a prepare finds it full.
func TestCreateBuild_PrunesTheOldestBuildAtTheLimit(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	// The eleventh build is the one that finds the limit already reached: the single
	// oldest build goes, and the new one takes its place.
	prepareBuilds(t, repo, 11, 10)

	kept := storedBuildIDs(t, repo)
	want := []string{
		"2026-01-31-2", "2026-01-31-3", "2026-01-31-4", "2026-01-31-5", "2026-01-31-6",
		"2026-01-31-7", "2026-01-31-8", "2026-01-31-9", "2026-01-31-10", "2026-01-31-11",
	}
	if len(kept) != len(want) {
		t.Fatalf("kept %v, want %v", kept, want)
	}
	for i := range want {
		if kept[i] != want[i] {
			t.Fatalf("kept %v, want %v", kept, want)
		}
	}
}

// The rule that matters: a build a gateway is currently deployed from survives,
// however old it is, and a newer unused build goes instead. Age only orders the
// builds that are free to go.
func TestCreateBuild_KeepsBuildsAGatewayIsDeployedFrom(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	createTestGateway(t, db, "gw-1", buildRepoOrgUUID)
	createTestGateway(t, db, "gw-2", buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	builds := prepareBuilds(t, repo, 10, 0)
	// The two oldest builds are what the gateways are serving.
	deployFromBuild(t, db, "gw-1", "dep-1", builds[0])
	deployFromBuild(t, db, "gw-2", "dep-2", builds[1])

	eleventh := buildOn(time.Date(2026, 1, 31, 10, 0, 0, 0, time.UTC))
	if err := repo.CreateBuildWithLimitEnforcement(eleventh, 10); err != nil {
		t.Fatalf("CreateBuildWithLimitEnforcement: %v", err)
	}

	kept := map[string]bool{}
	for _, buildID := range storedBuildIDs(t, repo) {
		kept[buildID] = true
	}
	for _, inUse := range []string{"2026-01-31-1", "2026-01-31-2"} {
		if !kept[inUse] {
			t.Errorf("build %s is deployed on a gateway but was pruned", inUse)
		}
	}
	// The oldest build that is free to go went instead — the third, since the two
	// older ones are being served.
	if kept["2026-01-31-3"] {
		t.Errorf("unused build 2026-01-31-3 should have been pruned, kept %v", kept)
	}
	if !kept["2026-01-31-9"] || !kept["2026-01-31-10"] || !kept[eleventh.BuildID] {
		t.Errorf("the newest builds should have been kept, got %v", kept)
	}
}

// An archived deployment is not a reason to keep a build: it carries its own
// rendered content, so restoring it never needs the build back.
func TestCreateBuild_AnArchivedDeploymentDoesNotHoldABuild(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	createTestGateway(t, db, "gw-1", buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	builds := prepareBuilds(t, repo, 10, 0)
	// Deployed from the oldest build, then superseded: the status row moves to the
	// newer deployment, leaving the first one archived.
	deployFromBuild(t, db, "gw-1", "dep-old", builds[0])
	deployFromBuild(t, db, "gw-1", "dep-new", builds[9])

	eleventh := buildOn(time.Date(2026, 1, 31, 10, 0, 0, 0, time.UTC))
	if err := repo.CreateBuildWithLimitEnforcement(eleventh, 10); err != nil {
		t.Fatalf("CreateBuildWithLimitEnforcement: %v", err)
	}

	kept := map[string]bool{}
	for _, buildID := range storedBuildIDs(t, repo) {
		kept[buildID] = true
	}
	if kept["2026-01-31-1"] {
		t.Error("a build held only by an archived deployment should have been pruned")
	}
	if !kept["2026-01-31-10"] {
		t.Error("the build the gateway is now serving was pruned")
	}
}

// With every build in use there is nothing safe to remove, so the prepare is
// REFUSED. Neither alternative is acceptable: deleting a build a gateway is serving
// takes away what a promotion out of that environment carries, and quietly storing
// one more puts the API over the limit it is entitled to. Which deployment to give
// up is the caller's decision, so they are told.
func TestCreateBuild_RefusesWhenNothingIsFreeToGo(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	builds := prepareBuilds(t, repo, 3, 0)
	for i, build := range builds {
		gatewayID := fmt.Sprintf("gw-%d", i+1)
		createTestGateway(t, db, gatewayID, buildRepoOrgUUID)
		deployFromBuild(t, db, gatewayID, fmt.Sprintf("dep-%d", i+1), build)
	}

	fourth := buildOn(time.Date(2026, 1, 31, 3, 0, 0, 0, time.UTC))
	err := repo.CreateBuildWithLimitEnforcement(fourth, 3)
	if !errors.Is(err, ErrBuildLimitReached) {
		t.Fatalf("error = %v, want ErrBuildLimitReached", err)
	}
	// And the refusal took nothing with it: the three in-use builds are all still
	// there, and the one that was refused was not stored.
	if kept := storedBuildIDs(t, repo); len(kept) != 3 {
		t.Errorf("kept %v, want the three in-use builds and nothing more", kept)
	}
}

// A limit lowered since the last prepare leaves the API over it by more than one.
// Pruning removes as many free builds as the new limit demands, so the API
// converges on the first prepare instead of drifting down one build at a time.
func TestCreateBuild_ConvergesOnALoweredLimit(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	prepareBuilds(t, repo, 10, 0)

	// Ten stored, and now a limit of three: the seven oldest go, leaving room for
	// the new build to make three.
	eleventh := buildOn(time.Date(2026, 1, 31, 10, 0, 0, 0, time.UTC))
	if err := repo.CreateBuildWithLimitEnforcement(eleventh, 3); err != nil {
		t.Fatalf("CreateBuildWithLimitEnforcement: %v", err)
	}

	kept := storedBuildIDs(t, repo)
	want := []string{"2026-01-31-9", "2026-01-31-10", "2026-01-31-11"}
	if !reflect.DeepEqual(kept, want) {
		t.Errorf("kept %v, want %v", kept, want)
	}
}

// Deleting a build is what makes room when the limit refuses another prepare, so
// the two have to fit together: a build no deployment holds goes, and preparing
// then succeeds where it had just been refused.
func TestDeleteBuild_FreesRoomForAnotherPrepare(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	builds := prepareBuilds(t, repo, 3, 0)
	// Two of the three are being served, so only the middle one is free.
	createTestGateway(t, db, "gw-1", buildRepoOrgUUID)
	createTestGateway(t, db, "gw-2", buildRepoOrgUUID)
	deployFromBuild(t, db, "gw-1", "dep-1", builds[0])
	deployFromBuild(t, db, "gw-2", "dep-2", builds[2])

	if err := repo.DeleteBuild(builds[1].BuildID, buildRepoAPIUUID, buildRepoOrgUUID); err != nil {
		t.Fatalf("DeleteBuild: %v", err)
	}
	kept := storedBuildIDs(t, repo)
	if len(kept) != 2 || kept[0] != builds[0].BuildID || kept[1] != builds[2].BuildID {
		t.Fatalf("kept %v, want the two builds that are deployed", kept)
	}

	fourth := buildOn(time.Date(2026, 1, 31, 3, 0, 0, 0, time.UTC))
	if err := repo.CreateBuildWithLimitEnforcement(fourth, 3); err != nil {
		t.Fatalf("preparing after the delete freed a slot: %v", err)
	}
}

// A build a gateway is serving is not deletable. Removing it would leave that
// deployment with no snapshot to promote onward, and the definition as it stood
// cannot be rendered again — so the caller has to undeploy first, and is told so
// rather than having the build taken out from under a running gateway.
func TestDeleteBuild_RefusesABuildADeploymentHolds(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	createTestGateway(t, db, "gw-1", buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	builds := prepareBuilds(t, repo, 2, 0)
	deployFromBuild(t, db, "gw-1", "dep-1", builds[0])

	err := repo.DeleteBuild(builds[0].BuildID, buildRepoAPIUUID, buildRepoOrgUUID)
	if !errors.Is(err, ErrBuildInUse) {
		t.Fatalf("error = %v, want ErrBuildInUse", err)
	}
	if kept := storedBuildIDs(t, repo); len(kept) != 2 {
		t.Errorf("kept %v, want both builds still stored", kept)
	}
}

// An archived deployment does not hold a build: it carries its own rendered
// content, so it never needs the build back. Deleting the build only clears the
// reference the archived deployment no longer needs.
func TestDeleteBuild_AnArchivedDeploymentDoesNotHoldIt(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	createTestGateway(t, db, "gw-1", buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	builds := prepareBuilds(t, repo, 2, 0)
	// The first deployment is superseded by the second, so its status row moves on
	// and it is left archived.
	deployFromBuild(t, db, "gw-1", "dep-old", builds[0])
	deployFromBuild(t, db, "gw-1", "dep-new", builds[1])

	if err := repo.DeleteBuild(builds[0].BuildID, buildRepoAPIUUID, buildRepoOrgUUID); err != nil {
		t.Fatalf("DeleteBuild: %v", err)
	}
	kept := storedBuildIDs(t, repo)
	if len(kept) != 1 || kept[0] != builds[1].BuildID {
		t.Errorf("kept %v, want only the build the gateway is serving", kept)
	}
}

// A build id that is not one of this API's is a not-found, not a silent success —
// and, since build ids are unique only per API, not another API's build either.
func TestDeleteBuild_UnknownBuildIsNotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	const otherAPIUUID = "aaaaaaaa-0000-0000-0000-00000000000e"
	insertBuildTestArtifact(t, db, otherAPIUUID, buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	builds := prepareBuilds(t, repo, 1, 0)

	if err := repo.DeleteBuild("2026-01-31-99", buildRepoAPIUUID, buildRepoOrgUUID); !errors.Is(err, ErrBuildNotFound) {
		t.Errorf("error = %v, want ErrBuildNotFound", err)
	}
	// The id exists, but under a different API.
	if err := repo.DeleteBuild(builds[0].BuildID, otherAPIUUID, buildRepoOrgUUID); !errors.Is(err, ErrBuildNotFound) {
		t.Errorf("error for another API's build = %v, want ErrBuildNotFound", err)
	}
	if kept := storedBuildIDs(t, repo); len(kept) != 1 {
		t.Errorf("kept %v, want the build untouched", kept)
	}
}

// The description is what tells one snapshot from another when choosing which to
// deploy or which to delete, so it has to survive the round trip — on the single
// read and in the listing, which are separate queries.
func TestCreateBuild_DescriptionIsStoredAndReadBack(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	build := buildOn(time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC))
	build.Description = "Adds the /reports endpoint"
	if err := repo.CreateBuildWithLimitEnforcement(build, 0); err != nil {
		t.Fatalf("CreateBuildWithLimitEnforcement: %v", err)
	}
	// A build prepared without one reads back empty rather than failing to scan.
	plain := buildOn(time.Date(2026, 1, 31, 1, 0, 0, 0, time.UTC))
	if err := repo.CreateBuildWithLimitEnforcement(plain, 0); err != nil {
		t.Fatalf("CreateBuildWithLimitEnforcement: %v", err)
	}

	got, err := repo.GetBuild(build.BuildID, buildRepoAPIUUID, buildRepoOrgUUID)
	if err != nil {
		t.Fatalf("GetBuild: %v", err)
	}
	if got.Description != "Adds the /reports endpoint" {
		t.Errorf("description = %q, want the note it was prepared with", got.Description)
	}

	listed, err := repo.GetBuilds(buildRepoAPIUUID, buildRepoOrgUUID, 0)
	if err != nil {
		t.Fatalf("GetBuilds: %v", err)
	}
	descriptions := map[string]string{}
	for _, b := range listed {
		descriptions[b.BuildID] = b.Description
	}
	if descriptions[build.BuildID] != "Adds the /reports endpoint" {
		t.Errorf("listed description = %q, want the note", descriptions[build.BuildID])
	}
	if descriptions[plain.BuildID] != "" {
		t.Errorf("a build prepared without a description listed %q", descriptions[plain.BuildID])
	}
}

// The budget is per API: one API reaching its limit must not prune another's
// builds, which is why the count and the cleanup are both scoped to the artifact.
func TestCreateBuild_PruningIsScopedToOneAPI(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	const otherAPIUUID = "aaaaaaaa-0000-0000-0000-00000000000d"
	insertBuildTestArtifact(t, db, otherAPIUUID, buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	day := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 2; i++ {
		other := buildOn(day.Add(time.Duration(i) * time.Hour))
		other.ArtifactID = otherAPIUUID
		if err := repo.CreateBuildWithLimitEnforcement(other, 2); err != nil {
			t.Fatalf("CreateBuildWithLimitEnforcement: %v", err)
		}
	}
	prepareBuilds(t, repo, 3, 2)

	otherBuilds, err := repo.GetBuilds(otherAPIUUID, buildRepoOrgUUID, 0)
	if err != nil {
		t.Fatalf("GetBuilds: %v", err)
	}
	if len(otherBuilds) != 2 {
		t.Errorf("the other API kept %d builds, want its own 2 untouched", len(otherBuilds))
	}
}

// Pruning a build clears the references to it rather than leaving them dangling,
// and the deployment keeps the readable build id in its metadata — so the origin
// stays legible after the snapshot itself is gone.
func TestCreateBuild_PruningClearsTheReferenceOnArchivedDeployments(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	createTestGateway(t, db, "gw-1", buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	builds := prepareBuilds(t, repo, 10, 0)
	// Deployed from the oldest build, then superseded, so that build is free to go
	// while a deployment still points at it.
	deployFromBuild(t, db, "gw-1", "dep-old", builds[0])
	deployFromBuild(t, db, "gw-1", "dep-new", builds[9])

	eleventh := buildOn(time.Date(2026, 1, 31, 10, 0, 0, 0, time.UTC))
	if err := repo.CreateBuildWithLimitEnforcement(eleventh, 10); err != nil {
		t.Fatalf("CreateBuildWithLimitEnforcement: %v", err)
	}

	var buildUUID sql.NullString
	if err := db.QueryRow(`SELECT build_uuid FROM deployments WHERE uuid = ?`, "dep-old").
		Scan(&buildUUID); err != nil {
		t.Fatalf("read deployment: %v", err)
	}
	if buildUUID.Valid {
		t.Errorf("build_uuid = %q, want NULL once the build is pruned", buildUUID.String)
	}
	// The deployment still runs, but it now reports no build — which is the honest
	// answer, because the snapshot it came from is gone and cannot be promoted.
	dep, err := repo.GetWithContent("dep-old", buildRepoAPIUUID, buildRepoOrgUUID)
	if err != nil {
		t.Fatalf("GetWithContent: %v", err)
	}
	if dep.BuildID != nil {
		t.Errorf("buildId = %q, want none once the build is pruned", *dep.BuildID)
	}

	// The build the other gateway is still serving keeps both.
	current, err := repo.GetWithContent("dep-new", buildRepoAPIUUID, buildRepoOrgUUID)
	if err != nil {
		t.Fatalf("GetWithContent: %v", err)
	}
	if current.BuildID == nil || *current.BuildID != builds[9].BuildID {
		t.Errorf("buildId = %v, want %q for the build still in use", current.BuildID, builds[9].BuildID)
	}
}
