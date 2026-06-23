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

//go:build integration

package integration

import (
	"fmt"
	"testing"

	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
)

// TestLifecycle_OrganizationPagination drives the real repository layer through
// create + paginated list, exercising DB.PaginationClause across the active
// engine. On SQL Server this is the code path that previously failed with
// "Incorrect syntax near 'LIMIT'".
func TestLifecycle_OrganizationPagination(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	orgRepo := repository.NewOrganizationRepo(it.db)

	baseline, err := orgRepo.ListOrganizations(1_000_000, 0)
	if err != nil {
		t.Fatalf("[%s] baseline list failed: %v", it.driver, err)
	}

	const n = 5
	for i := range n {
		org := &model.Organization{ID: id(), Handle: "lc-" + id()[:8], Name: fmt.Sprintf("org %d", i), Region: "us"}
		if err := orgRepo.CreateOrganization(org); err != nil {
			t.Fatalf("[%s] create org failed: %v", it.driver, err)
		}
	}
	total := len(baseline) + n

	// Page through in steps of 2 and confirm full, non-overlapping coverage.
	seen := map[string]bool{}
	for offset := 0; offset < total; offset += 2 {
		page, err := orgRepo.ListOrganizations(2, offset)
		if err != nil {
			t.Fatalf("[%s] ListOrganizations(2,%d) failed: %v", it.driver, offset, err)
		}
		want := 2
		if rem := total - offset; rem < want {
			want = rem
		}
		if len(page) != want {
			t.Fatalf("[%s] page at offset %d: want %d rows, got %d", it.driver, offset, want, len(page))
		}
		for _, o := range page {
			if seen[o.ID] {
				t.Fatalf("[%s] pagination overlap at offset %d: id %s seen twice", it.driver, offset, o.ID)
			}
			seen[o.ID] = true
		}
	}
	if len(seen) != total {
		t.Fatalf("[%s] paging covered %d rows, want %d", it.driver, len(seen), total)
	}
}

// TestLifecycle_SubscriptionPlanExistsAndList exercises FetchFirstClause (the
// SELECT 1 ... FETCH NEXT 1 existence check) and a filtered paginated list
// through the real repository, across the active engine.
func TestLifecycle_SubscriptionPlanExistsAndList(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	orgRepo := repository.NewOrganizationRepo(it.db)
	planRepo := repository.NewSubscriptionPlanRepo(it.db)

	org := &model.Organization{ID: id(), Handle: "pl-" + id()[:8], Name: "plan org", Region: "us"}
	if err := orgRepo.CreateOrganization(org); err != nil {
		t.Fatalf("[%s] create org failed: %v", it.driver, err)
	}

	exists, err := planRepo.ExistsByNameAndOrg("nope-"+id(), org.ID)
	if err != nil {
		t.Fatalf("[%s] ExistsByNameAndOrg failed: %v", it.driver, err)
	}
	if exists {
		t.Fatalf("[%s] ExistsByNameAndOrg: want false for missing plan", it.driver)
	}

	count := 5
	for i := range 3 {
		// Fully populated: the list repository scans billing_plan / throttle
		// columns into plain (non-nullable) fields (a pre-existing detail).
		plan := &model.SubscriptionPlan{
			UUID: id(), PlanName: fmt.Sprintf("plan-%d-%s", i, id()[:6]),
			BillingPlan: "free", StopOnQuotaReach: true,
			ThrottleLimitCount: &count, ThrottleLimitUnit: "min",
			OrganizationUUID: org.ID, Status: model.SubscriptionPlanStatus("ACTIVE"),
		}
		if err := planRepo.Create(plan); err != nil {
			t.Fatalf("[%s] create plan failed: %v", it.driver, err)
		}
	}
	plans, err := planRepo.ListByOrganization(org.ID, 2, 0)
	if err != nil {
		t.Fatalf("[%s] ListByOrganization failed: %v", it.driver, err)
	}
	if len(plans) != 2 {
		t.Fatalf("[%s] ListByOrganization(2,0): want 2, got %d", it.driver, len(plans))
	}
}

// TestLifecycle_ProjectPagination exercises ProjectRepo.ListProjects pagination
// (the project.go LIMIT ? OFFSET ? query) through the real repository.
func TestLifecycle_ProjectPagination(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	orgRepo := repository.NewOrganizationRepo(it.db)
	projectRepo := repository.NewProjectRepo(it.db)

	org := &model.Organization{ID: id(), Handle: "pr-" + id()[:8], Name: "proj org", Region: "us"}
	if err := orgRepo.CreateOrganization(org); err != nil {
		t.Fatalf("[%s] create org failed: %v", it.driver, err)
	}

	const n = 5
	for i := range n {
		p := &model.Project{ID: id(), Name: fmt.Sprintf("proj-%d-%s", i, id()[:6]), OrganizationID: org.ID, Description: "p"}
		if err := projectRepo.CreateProject(p); err != nil {
			t.Fatalf("[%s] create project failed: %v", it.driver, err)
		}
	}

	seen := map[string]bool{}
	for offset := 0; offset < n; offset += 2 {
		page, err := projectRepo.ListProjects(org.ID, 2, offset)
		if err != nil {
			t.Fatalf("[%s] ListProjects(2,%d) failed: %v", it.driver, offset, err)
		}
		want := 2
		if rem := n - offset; rem < want {
			want = rem
		}
		if len(page) != want {
			t.Fatalf("[%s] ListProjects offset %d: want %d, got %d", it.driver, offset, want, len(page))
		}
		for _, p := range page {
			if seen[p.ID] {
				t.Fatalf("[%s] project pagination overlap at offset %d: %s", it.driver, offset, p.ID)
			}
			seen[p.ID] = true
		}
	}
	if len(seen) != n {
		t.Fatalf("[%s] project paging covered %d, want %d", it.driver, len(seen), n)
	}
}

// TestLifecycle_SubscriptionListByFilters exercises SubscriptionRepo.ListByFilters
// (the subscription_repository.go LIMIT ? OFFSET ? query) including the status
// filter, reusing the seeded org graph (which creates one ACTIVE subscription).
func TestLifecycle_SubscriptionListByFilters(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	g := seedOrgGraph(t, it)
	subRepo := repository.NewSubscriptionRepo(it.db)

	all, err := subRepo.ListByFilters(g.org, nil, nil, nil, nil, 10, 0)
	if err != nil {
		t.Fatalf("[%s] ListByFilters (no filter) failed: %v", it.driver, err)
	}
	if len(all) != 1 {
		t.Fatalf("[%s] ListByFilters: want 1 subscription, got %d", it.driver, len(all))
	}

	active := "ACTIVE"
	got, err := subRepo.ListByFilters(g.org, nil, nil, nil, &active, 10, 0)
	if err != nil {
		t.Fatalf("[%s] ListByFilters (status=ACTIVE) failed: %v", it.driver, err)
	}
	if len(got) != 1 {
		t.Fatalf("[%s] ListByFilters(status=ACTIVE): want 1, got %d", it.driver, len(got))
	}

	revoked := "REVOKED"
	none, err := subRepo.ListByFilters(g.org, nil, nil, nil, &revoked, 10, 0)
	if err != nil {
		t.Fatalf("[%s] ListByFilters (status=REVOKED) failed: %v", it.driver, err)
	}
	if len(none) != 0 {
		t.Fatalf("[%s] ListByFilters(status=REVOKED): want 0, got %d", it.driver, len(none))
	}
}

// TestLifecycle_DevPortalDefault exercises the devportal default-flag queries
// (GetDefaultByOrganizationUUID + SetAsDefault). These used the `is_default =
// TRUE`/`FALSE` boolean literals that are invalid on SQL Server; the fix binds a
// Go bool instead, which this verifies works on every engine.
func TestLifecycle_DevPortalDefault(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	orgRepo := repository.NewOrganizationRepo(it.db)
	dpRepo := repository.NewDevPortalRepository(it.db)

	org := &model.Organization{ID: id(), Handle: "dp-" + id()[:8], Name: "dp org", Region: "us"}
	if err := orgRepo.CreateOrganization(org); err != nil {
		t.Fatalf("[%s] create org failed: %v", it.driver, err)
	}

	dp1 := newDevPortal(org.ID, "dp1", true)
	dp2 := newDevPortal(org.ID, "dp2", false)
	if err := dpRepo.Create(dp1); err != nil {
		t.Fatalf("[%s] create dp1 failed: %v", it.driver, err)
	}
	if err := dpRepo.Create(dp2); err != nil {
		t.Fatalf("[%s] create dp2 failed: %v", it.driver, err)
	}

	def, err := dpRepo.GetDefaultByOrganizationUUID(org.ID)
	if err != nil {
		t.Fatalf("[%s] GetDefaultByOrganizationUUID failed: %v", it.driver, err)
	}
	if def == nil || def.UUID != dp1.UUID {
		t.Fatalf("[%s] default devportal: want dp1, got %+v", it.driver, def)
	}

	// Switch the default to dp2 (unsets dp1, sets dp2 — the boolean UPDATE path).
	if err := dpRepo.SetAsDefault(dp2.UUID, org.ID); err != nil {
		t.Fatalf("[%s] SetAsDefault(dp2) failed: %v", it.driver, err)
	}
	def, err = dpRepo.GetDefaultByOrganizationUUID(org.ID)
	if err != nil {
		t.Fatalf("[%s] GetDefaultByOrganizationUUID (after switch) failed: %v", it.driver, err)
	}
	if def == nil || def.UUID != dp2.UUID {
		t.Fatalf("[%s] default devportal after switch: want dp2, got %+v", it.driver, def)
	}
}

// TestLifecycle_ApplicationByIDOrHandle exercises GetApplicationByIDOrHandle,
// whose `ORDER BY CASE … FetchFirstClause(1)` query was part of the LIMIT-1 fix
// (a single-row lookup that resolves by UUID or handle). Verified on every engine.
func TestLifecycle_ApplicationByIDOrHandle(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	orgRepo := repository.NewOrganizationRepo(it.db)
	projectRepo := repository.NewProjectRepo(it.db)
	appRepo := repository.NewApplicationRepo(it.db)

	org := &model.Organization{ID: id(), Handle: "ap-" + id()[:8], Name: "app org", Region: "us"}
	if err := orgRepo.CreateOrganization(org); err != nil {
		t.Fatalf("[%s] create org failed: %v", it.driver, err)
	}
	proj := &model.Project{ID: id(), Name: "p-" + id()[:6], OrganizationID: org.ID}
	if err := projectRepo.CreateProject(proj); err != nil {
		t.Fatalf("[%s] create project failed: %v", it.driver, err)
	}
	app := &model.Application{
		UUID: id(), Handle: "app-" + id()[:8], ProjectUUID: proj.ID,
		OrganizationUUID: org.ID, Name: "app", Type: "standard",
	}
	if err := appRepo.CreateApplication(app); err != nil {
		t.Fatalf("[%s] create application failed: %v", it.driver, err)
	}

	byUUID, err := appRepo.GetApplicationByIDOrHandle(app.UUID, org.ID)
	if err != nil {
		t.Fatalf("[%s] GetApplicationByIDOrHandle(uuid) failed: %v", it.driver, err)
	}
	if byUUID == nil || byUUID.UUID != app.UUID {
		t.Fatalf("[%s] lookup by uuid: want %s, got %+v", it.driver, app.UUID, byUUID)
	}

	byHandle, err := appRepo.GetApplicationByIDOrHandle(app.Handle, org.ID)
	if err != nil {
		t.Fatalf("[%s] GetApplicationByIDOrHandle(handle) failed: %v", it.driver, err)
	}
	if byHandle == nil || byHandle.UUID != app.UUID {
		t.Fatalf("[%s] lookup by handle: want %s, got %+v", it.driver, app.UUID, byHandle)
	}

	missing, err := appRepo.GetApplicationByIDOrHandle("does-not-exist-"+id(), org.ID)
	if err != nil {
		t.Fatalf("[%s] GetApplicationByIDOrHandle(missing) failed: %v", it.driver, err)
	}
	if missing != nil {
		t.Fatalf("[%s] lookup of missing app: want nil, got %+v", it.driver, missing)
	}
}

func newDevPortal(orgID, name string, isDefault bool) *model.DevPortal {
	u := id()
	return &model.DevPortal{
		UUID: u, OrganizationUUID: orgID, Name: name,
		Identifier: name + "-" + u[:8],
		APIUrl:     "http://" + name + "-" + u[:8],
		Hostname:   name + "-" + u[:8] + ".local",
		APIKey:     "k", HeaderKeyName: "x-wso2-api-key",
		IsDefault: isDefault, Visibility: "private",
	}
}
