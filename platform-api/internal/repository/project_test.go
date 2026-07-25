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

func seedTestOrg(t *testing.T, db *database.DB, orgUUID string) {
	t.Helper()
	query := `
		INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := db.Exec(db.Rebind(query), orgUUID, "acme", "Acme", "us", "idp-"+orgUUID, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("Failed to seed organization: %v", err)
	}
}

func TestProjectRepo_CreateAndRead_IsActive(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-1"
	seedTestOrg(t, db, orgUUID)
	repo := NewProjectRepo(db)

	cases := []struct {
		name     string
		uuid     string
		handle   string
		isActive bool
	}{
		{"active project", "proj-active", "active-proj", true},
		{"inactive project", "proj-inactive", "inactive-proj", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &model.Project{
				ID:             tc.uuid,
				Handle:         tc.handle,
				Name:           tc.handle,
				OrganizationID: orgUUID,
				IsActive:       tc.isActive,
			}
			if err := repo.CreateProject(p); err != nil {
				t.Fatalf("CreateProject: %v", err)
			}

			byHandle, err := repo.GetProjectByHandleAndOrgID(tc.handle, orgUUID)
			if err != nil || byHandle == nil {
				t.Fatalf("GetProjectByHandleAndOrgID: %v (nil=%v)", err, byHandle == nil)
			}
			if byHandle.IsActive != tc.isActive {
				t.Errorf("byHandle.IsActive = %v, want %v", byHandle.IsActive, tc.isActive)
			}

			byUUID, err := repo.GetProjectByUUID(tc.uuid)
			if err != nil || byUUID == nil {
				t.Fatalf("GetProjectByUUID: %v (nil=%v)", err, byUUID == nil)
			}
			if byUUID.IsActive != tc.isActive {
				t.Errorf("byUUID.IsActive = %v, want %v", byUUID.IsActive, tc.isActive)
			}
		})
	}

	// The list paths must surface the flag too.
	all, err := repo.GetProjectsByOrganizationID(orgUUID)
	if err != nil {
		t.Fatalf("GetProjectsByOrganizationID: %v", err)
	}
	got := map[string]bool{}
	for _, p := range all {
		got[p.Handle] = p.IsActive
	}
	if got["active-proj"] != true || got["inactive-proj"] != false {
		t.Errorf("GetProjectsByOrganizationID is_active mismatch: %v", got)
	}

	listed, err := repo.ListProjects(orgUUID, ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	gotListed := map[string]bool{}
	for _, p := range listed {
		gotListed[p.Handle] = p.IsActive
	}
	if gotListed["active-proj"] != true || gotListed["inactive-proj"] != false {
		t.Errorf("ListProjects is_active mismatch: %v", gotListed)
	}
}

func TestProjectRepo_SetProjectActive(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID = "org-1"
	seedTestOrg(t, db, orgUUID)
	repo := NewProjectRepo(db)

	p := &model.Project{
		ID:             "proj-1",
		Handle:         "proj",
		Name:           "proj",
		OrganizationID: orgUUID,
		IsActive:       false,
	}
	if err := repo.CreateProject(p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if err := repo.SetProjectActive("proj-1", true); err != nil {
		t.Fatalf("SetProjectActive(true): %v", err)
	}
	got, err := repo.GetProjectByUUID("proj-1")
	if err != nil || got == nil {
		t.Fatalf("GetProjectByUUID: %v (nil=%v)", err, got == nil)
	}
	if !got.IsActive {
		t.Errorf("after SetProjectActive(true), IsActive = false, want true")
	}

	if err := repo.SetProjectActive("proj-1", false); err != nil {
		t.Fatalf("SetProjectActive(false): %v", err)
	}
	got, err = repo.GetProjectByUUID("proj-1")
	if err != nil || got == nil {
		t.Fatalf("GetProjectByUUID: %v (nil=%v)", err, got == nil)
	}
	if got.IsActive {
		t.Errorf("after SetProjectActive(false), IsActive = true, want false")
	}
}
