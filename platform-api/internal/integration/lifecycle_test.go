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

	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
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

	exists, err := planRepo.ExistsByHandleAndOrg("nope-"+id(), org.ID)
	if err != nil {
		t.Fatalf("[%s] ExistsByHandleAndOrg failed: %v", it.driver, err)
	}
	if exists {
		t.Fatalf("[%s] ExistsByHandleAndOrg: want false for missing plan", it.driver)
	}

	count := 5
	for i := range 3 {
		// Fully populated: the throttle fields are persisted as a single
		// REQUEST_COUNT row in subscription_plan_limits and hydrated back via
		// the LEFT JOIN in the list/get queries.
		slug := fmt.Sprintf("plan-%d-%s", i, id()[:6])
		plan := &model.SubscriptionPlan{
			UUID: id(), Handle: slug, Name: fmt.Sprintf("Plan %d", i),
			StopOnQuotaReach: true,
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
	// The throttle triple (count/unit + stop_on_quota_reach) is stored as a single
	// row in subscription_plan_limits; confirm it round-trips through both the list
	// and single-get paths.
	for _, p := range plans {
		if p.ThrottleLimitCount == nil || *p.ThrottleLimitCount != count {
			t.Fatalf("[%s] list hydrate: ThrottleLimitCount = %v, want %d", it.driver, p.ThrottleLimitCount, count)
		}
		if p.ThrottleLimitUnit != "min" || !p.StopOnQuotaReach {
			t.Fatalf("[%s] list hydrate: unit=%q stop=%v, want unit=min stop=true", it.driver, p.ThrottleLimitUnit, p.StopOnQuotaReach)
		}
	}
	got, err := planRepo.GetByID(plans[0].UUID, org.ID)
	if err != nil {
		t.Fatalf("[%s] GetByID failed: %v", it.driver, err)
	}
	if got.ThrottleLimitCount == nil || *got.ThrottleLimitCount != count || got.ThrottleLimitUnit != "min" {
		t.Fatalf("[%s] GetByID hydrate: count=%v unit=%q, want %d/min", it.driver, got.ThrottleLimitCount, got.ThrottleLimitUnit, count)
	}

	// Update clearing the throttle should delete the limit row; reads then
	// report no throttle with the default stop_on_quota_reach.
	got.ThrottleLimitCount = nil
	got.ThrottleLimitUnit = ""
	got.StopOnQuotaReach = true
	if err := planRepo.Update(got); err != nil {
		t.Fatalf("[%s] Update (clear throttle) failed: %v", it.driver, err)
	}
	cleared, err := planRepo.GetByID(got.UUID, org.ID)
	if err != nil {
		t.Fatalf("[%s] GetByID after clear failed: %v", it.driver, err)
	}
	if cleared.ThrottleLimitCount != nil || cleared.ThrottleLimitUnit != "" {
		t.Fatalf("[%s] after clear: want no throttle, got count=%v unit=%q", it.driver, cleared.ThrottleLimitCount, cleared.ThrottleLimitUnit)
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
		pid := id()
		pName := fmt.Sprintf("proj-%d-%s", i, pid[:6])
		p := &model.Project{ID: pid, Handle: pName, Name: pName, OrganizationID: org.ID, Description: "p"}
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

// TestLifecycle_ApplicationByIDOrHandle exercises GetApplicationByIDOrHandle,
// whose `ORDER BY CASE … FetchFirstClause(1)` query was part of the LIMIT-1 fix
// (a single-row lookup that resolves by UUID or handle). Verified on every engine.
func TestLifecycle_ApplicationByIDOrHandle(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	orgRepo := repository.NewOrganizationRepo(it.db)
	projectRepo := repository.NewProjectRepo(it.db)
	appRepo := repository.NewApplicationRepo(it.db, repository.NewArtifactTableRegistry())

	org := &model.Organization{ID: id(), Handle: "ap-" + id()[:8], Name: "app org", Region: "us"}
	if err := orgRepo.CreateOrganization(org); err != nil {
		t.Fatalf("[%s] create org failed: %v", it.driver, err)
	}
	projID := id()
	projName := "p-" + projID[:6]
	proj := &model.Project{ID: projID, Handle: projName, Name: projName, OrganizationID: org.ID}
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


