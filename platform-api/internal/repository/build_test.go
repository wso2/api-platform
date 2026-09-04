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
	"fmt"
	"strings"
	"testing"
	"time"

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

// The snapshot and its property bag come back exactly as stored: a build is what a
// deploy sends, so anything lost here would silently change what runs.
func TestGetBuild_ReturnsTheSnapshotAndItsProperties(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	stored := buildOn(time.Date(2026, 1, 31, 9, 0, 0, 0, time.UTC))
	stored.Properties = map[string]any{"commitId": "9f1c2ab"}
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
	if read.Properties["commitId"] != "9f1c2ab" {
		t.Errorf("properties = %v, want the commit that was recorded", read.Properties)
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
	metadata := fmt.Sprintf(`{"buildId":%q}`, build.BuildID)
	_, err := db.Exec(`
		INSERT INTO deployments (uuid, display_name, artifact_uuid, organization_uuid, gateway_uuid, build_uuid, content, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		deploymentID, deploymentID, buildRepoAPIUUID, buildRepoOrgUUID, gatewayUUID,
		build.UUID, []byte("content"), metadata, time.Now().UTC())
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

// Reaching the limit prunes a batch of the API's oldest builds, so preparing
// repeatedly cannot grow the table without bound.
func TestCreateBuild_PrunesTheOldestBuildsAtTheLimit(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	createTestAPI(t, db, buildRepoAPIUUID, buildRepoOrgUUID)
	repo := NewDeploymentRepo(db, NewArtifactTableRegistry())

	// The eleventh build is the one that finds the limit already reached: a batch of
	// five older builds goes, and the new one is added to what remains.
	prepareBuilds(t, repo, 11, 10)

	kept := storedBuildIDs(t, repo)
	want := []string{
		"2026-01-31-6", "2026-01-31-7", "2026-01-31-8", "2026-01-31-9", "2026-01-31-10",
		"2026-01-31-11",
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
	// Five of the unused builds went instead, oldest first.
	for _, pruned := range []string{"2026-01-31-3", "2026-01-31-4", "2026-01-31-5", "2026-01-31-6", "2026-01-31-7"} {
		if kept[pruned] {
			t.Errorf("unused build %s should have been pruned, kept %v", pruned, kept)
		}
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

// With every old build in use there is nothing safe to remove, so the API keeps
// more than the limit rather than the prepare failing or a running build going.
func TestCreateBuild_KeepsEverythingWhenNothingIsFreeToGo(t *testing.T) {
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
	if err := repo.CreateBuildWithLimitEnforcement(fourth, 3); err != nil {
		t.Fatalf("CreateBuildWithLimitEnforcement: %v", err)
	}
	if kept := storedBuildIDs(t, repo); len(kept) != 4 {
		t.Errorf("kept %v, want all four builds retained", kept)
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
func TestCreateBuild_PruningClearsTheReferenceAndLeavesTheReadableID(t *testing.T) {
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
	var metadata []byte
	err := db.QueryRow(`SELECT build_uuid, metadata FROM deployments WHERE uuid = ?`, "dep-old").
		Scan(&buildUUID, &metadata)
	if err != nil {
		t.Fatalf("read deployment: %v", err)
	}
	if buildUUID.Valid {
		t.Errorf("build_uuid = %q, want NULL once the build is pruned", buildUUID.String)
	}
	if !strings.Contains(string(metadata), builds[0].BuildID) {
		t.Errorf("metadata = %s, want the readable build id kept", metadata)
	}
}
