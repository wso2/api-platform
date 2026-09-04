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
		if err := repo.CreateBuild(build); err != nil {
			t.Fatalf("CreateBuild: %v", err)
		}
		if build.BuildID != want {
			t.Fatalf("build id = %q, want %q", build.BuildID, want)
		}
	}

	nextDay := buildOn(time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC))
	if err := repo.CreateBuild(nextDay); err != nil {
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
	if err := repo.CreateBuild(mine); err != nil {
		t.Fatalf("CreateBuild: %v", err)
	}
	theirs := buildOn(day)
	theirs.ArtifactID = otherAPIUUID
	if err := repo.CreateBuild(theirs); err != nil {
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
	if err := repo.CreateBuild(stored); err != nil {
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
	if err := repo.CreateBuild(stored); err != nil {
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
		if err := repo.CreateBuild(buildOn(day.Add(time.Duration(i) * time.Hour))); err != nil {
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
