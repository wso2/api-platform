//go:build integration

/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied. See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

package integration

import (
	"io"
	"log/slog"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

// seedVersion creates one REST API version through the real APIRepo (the
// artifacts + rest_apis two-table write) and returns the stored model.
func seedVersion(t *testing.T, it *itDB, repo repository.APIRepository,
	handle, displayName, version, projectID, orgID string, context *string) *model.API {
	t.Helper()
	apiModel := &model.API{
		Handle:          handle,
		Name:            displayName,
		Version:         version,
		ProjectID:       projectID,
		OrganizationID:  orgID,
		LifeCycleStatus: "CREATED",
		Configuration: model.RestAPIConfig{
			Name:      displayName,
			Version:   version,
			Transport: []string{"https"},
			Context:   context,
		},
	}
	if err := repo.CreateAPI(apiModel); err != nil {
		t.Fatalf("[%s] CreateAPI(%s) failed: %v", it.driver, handle, err)
	}
	return apiModel
}

// newVersionsService builds an APIService carrying only the two repositories
// GetAPIVersions reaches: the API repo for the lookup and the project repo for
// resolving each version's project UUID to its handle. Every other dependency
// is nil because no other collaborator is on this path.
func newVersionsService(it *itDB) *service.APIService {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return service.NewAPIService(
		repository.NewAPIRepo(it.db),
		repository.NewProjectRepo(it.db),
		nil, nil, nil, nil, nil, nil, nil,
		logger, nil, nil,
	)
}

// TestVersions_GetAPIVersionsByDisplayName drives the real APIRepo through the
// display-name version lookup on the active engine: the org + display-name
// predicates, the ORDER BY, and the empty-result case.
func TestVersions_GetAPIVersionsByDisplayName(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	orgID, projID := seedOrgProject(t, it, "versions")
	apiRepo := repository.NewAPIRepo(it.db)

	const displayName = "reading-list-api"
	for _, v := range []string{"v1.0", "v2.0", "v3.0"} {
		seedVersion(t, it, apiRepo, "reading-list-"+v+"-"+id()[:6], displayName, v, projID, orgID, nil)
	}
	// A second API in the same project that must never be picked up.
	seedVersion(t, it, apiRepo, "unrelated-"+id()[:6], "unrelated-api", "v1.0", projID, orgID, nil)

	t.Run("returns every version of the display name, in version order", func(t *testing.T) {
		versions, err := apiRepo.GetAPIVersionsByDisplayName(orgID, displayName)
		if err != nil {
			t.Fatalf("[%s] GetAPIVersionsByDisplayName failed: %v", it.driver, err)
		}
		if len(versions) != 3 {
			t.Fatalf("[%s] got %d versions, want 3", it.driver, len(versions))
		}
		want := []string{"v1.0", "v2.0", "v3.0"}
		for i, v := range versions {
			if v.Version != want[i] {
				t.Errorf("[%s] version[%d] = %q, want %q", it.driver, i, v.Version, want[i])
			}
			if v.Name != displayName {
				t.Errorf("[%s] version[%d] display name = %q, want %q", it.driver, i, v.Name, displayName)
			}
			if v.ProjectID != projID {
				t.Errorf("[%s] version[%d] project = %q, want %q", it.driver, i, v.ProjectID, projID)
			}
		}
	})

	t.Run("an unknown display name is an empty result, not an error", func(t *testing.T) {
		versions, err := apiRepo.GetAPIVersionsByDisplayName(orgID, "no-such-api-"+id()[:8])
		if err != nil {
			t.Fatalf("[%s] GetAPIVersionsByDisplayName failed: %v", it.driver, err)
		}
		if len(versions) != 0 {
			t.Fatalf("[%s] got %d versions, want 0", it.driver, len(versions))
		}
	})
}

// TestVersions_CrossOrgIsolation guards GO-AUTH-005 against the real foreign
// keys: the same display name in a second organization must not leak into the
// first organization's lookup.
func TestVersions_CrossOrgIsolation(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	orgA, projA := seedOrgProject(t, it, "versions-iso-a")
	orgB, projB := seedOrgProject(t, it, "versions-iso-b")
	apiRepo := repository.NewAPIRepo(it.db)

	const displayName = "shared-name"
	handleA := "shared-a-" + id()[:6]
	seedVersion(t, it, apiRepo, handleA, displayName, "v1.0", projA, orgA, nil)
	seedVersion(t, it, apiRepo, "shared-b1-"+id()[:6], displayName, "v1.0", projB, orgB, nil)
	seedVersion(t, it, apiRepo, "shared-b2-"+id()[:6], displayName, "v2.0", projB, orgB, nil)

	versions, err := apiRepo.GetAPIVersionsByDisplayName(orgA, displayName)
	if err != nil {
		t.Fatalf("[%s] GetAPIVersionsByDisplayName failed: %v", it.driver, err)
	}
	if len(versions) != 1 {
		t.Fatalf("[%s] got %d versions for org A, want 1 — rows from org B leaked", it.driver, len(versions))
	}
	if versions[0].OrganizationID != orgA {
		t.Fatalf("[%s] returned row belongs to org %q, want %q", it.driver, versions[0].OrganizationID, orgA)
	}
	if versions[0].Handle != handleA {
		t.Fatalf("[%s] returned handle %q, want %q", it.driver, versions[0].Handle, handleA)
	}
}

// TestVersions_AgreesWithUniquenessCheck is the invariant the endpoint's stated
// purpose rests on: callers use it to decide whether a version is free before
// creating one, so its answer must match what validateCreateAPIRequest would
// decide via CheckAPIExistsByNameAndVersionInOrganization. The two are separate
// SQL statements comparing the same column, so they are only guaranteed to
// agree when both are run against the same engine — which is what this does.
//
// This is also why the suite asserts nothing about case sensitivity: display
// name comparison follows the database collation (SQL Server's default is
// case-insensitive, SQLite's and Postgres' are not). Agreement between the two
// queries holds on every engine; a fixed case rule does not.
func TestVersions_AgreesWithUniquenessCheck(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	orgID, projID := seedOrgProject(t, it, "versions-agree")
	apiRepo := repository.NewAPIRepo(it.db)

	const displayName = "agreement-api"
	seedVersion(t, it, apiRepo, "agreement-"+id()[:6], displayName, "v1.0", projID, orgID, nil)

	versions, err := apiRepo.GetAPIVersionsByDisplayName(orgID, displayName)
	if err != nil {
		t.Fatalf("[%s] GetAPIVersionsByDisplayName failed: %v", it.driver, err)
	}
	taken := map[string]bool{}
	for _, v := range versions {
		taken[v.Version] = true
	}

	for _, version := range []string{"v1.0", "v2.0"} {
		exists, err := apiRepo.CheckAPIExistsByNameAndVersionInOrganization(displayName, version, orgID, "")
		if err != nil {
			t.Fatalf("[%s] CheckAPIExistsByNameAndVersionInOrganization failed: %v", it.driver, err)
		}
		if taken[version] != exists {
			t.Errorf("[%s] version %q: versions lookup says taken=%v, uniqueness check says exists=%v — "+
				"a create would disagree with what the endpoint reported",
				it.driver, version, taken[version], exists)
		}
	}
}

// TestVersions_ServiceResponseShape covers the service layer against real rows:
// the response envelope, and the project UUID → project handle resolution that
// only a real projects row can exercise.
func TestVersions_ServiceResponseShape(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	orgID, projID := seedOrgProject(t, it, "versions-svc")
	apiRepo := repository.NewAPIRepo(it.db)
	svc := newVersionsService(it)

	project, err := repository.NewProjectRepo(it.db).GetProjectByUUID(projID)
	if err != nil {
		t.Fatalf("[%s] GetProjectByUUID failed: %v", it.driver, err)
	}
	if project == nil {
		t.Fatalf("[%s] seeded project %q not found", it.driver, projID)
	}

	const displayName = "service-shape-api"
	context := "/" + project.Handle + "/service-shape-api/v1.0"
	seedVersion(t, it, apiRepo, "shape-v1-"+id()[:6], displayName, "v1.0", projID, orgID, &context)
	seedVersion(t, it, apiRepo, "shape-v2-"+id()[:6], displayName, "v2.0", projID, orgID, nil)

	t.Run("an existing display name reports the versions already taken", func(t *testing.T) {
		resp, err := svc.GetAPIVersions(orgID, displayName)
		if err != nil {
			t.Fatalf("[%s] GetAPIVersions failed: %v", it.driver, err)
		}
		if !resp.VersionsAvailable || resp.Count != 2 || len(resp.List) != 2 {
			t.Fatalf("[%s] got versionsAvailable=%v count=%d len=%d, want true/2/2",
				it.driver, resp.VersionsAvailable, resp.Count, len(resp.List))
		}
		if resp.DisplayName != displayName {
			t.Errorf("[%s] displayName = %q, want %q", it.driver, resp.DisplayName, displayName)
		}
		for i, v := range resp.List {
			// The project handle is what the other /rest-apis endpoints return;
			// the raw UUID reaching the client means the lookup fell through.
			if v.ProjectId != project.Handle {
				t.Errorf("[%s] list[%d].projectId = %q, want the project handle %q",
					it.driver, i, v.ProjectId, project.Handle)
			}
			if v.LifeCycleStatus == nil || *v.LifeCycleStatus != "CREATED" {
				t.Errorf("[%s] list[%d].lifeCycleStatus = %v, want CREATED", it.driver, i, v.LifeCycleStatus)
			}
			if v.CreatedAt == nil {
				t.Errorf("[%s] list[%d].createdAt is nil; want the stored timestamp", it.driver, i)
			}
		}
		if resp.List[0].Context == nil || *resp.List[0].Context != context {
			t.Errorf("[%s] list[0].context = %v, want %q", it.driver, resp.List[0].Context, context)
		}
		if resp.List[1].Context != nil {
			t.Errorf("[%s] list[1].context = %v; want it omitted when the configuration carries none",
				it.driver, resp.List[1].Context)
		}
	})

	t.Run("an unused display name reports versionsAvailable false, not an error", func(t *testing.T) {
		resp, err := svc.GetAPIVersions(orgID, "no-such-api-"+id()[:8])
		if err != nil {
			t.Fatalf("[%s] GetAPIVersions failed: %v", it.driver, err)
		}
		if resp.VersionsAvailable || resp.Count != 0 {
			t.Fatalf("[%s] got versionsAvailable=%v count=%d, want false/0",
				it.driver, resp.VersionsAvailable, resp.Count)
		}
		if resp.List == nil {
			t.Errorf("[%s] list is nil; it must serialize as [] rather than null", it.driver)
		}
	})
}
