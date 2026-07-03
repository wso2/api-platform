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
	"fmt"

	"github.com/cucumber/godog"

	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
)

// registerPaginationSteps wires the pagination and filtered-listing scenarios.
func registerPaginationSteps(ctx *godog.ScenarioContext, w *world) {
	// Organization pagination.
	ctx.Step(`^(\d+) additional organizations exist$`, w.additionalOrgsExist)
	ctx.Step(`^I page through organizations (\d+) at a time$`, w.pageOrganizations)
	ctx.Step(`^every organization is seen exactly once$`, w.everyOrgSeenOnce)

	// Subscription plan.
	ctx.Step(`^an organization exists$`, w.anOrganizationExists)
	ctx.Step(`^checking existence of a missing subscription plan returns false$`, w.missingPlanNotExists)
	ctx.Step(`^I create (\d+) subscription plans with a throttle limit of (\d+) per "([^"]*)"$`, w.createSubscriptionPlans)
	ctx.Step(`^listing subscription plans (\d+) at a time returns (\d+)$`, w.listSubscriptionPlans)
	ctx.Step(`^each listed plan round-trips its throttle limit$`, w.listedPlansHaveThrottle)
	ctx.Step(`^reading the first listed plan back round-trips its throttle limit$`, w.firstPlanGetHasThrottle)
	ctx.Step(`^I clear the throttle limit on the first listed plan$`, w.clearFirstPlanThrottle)
	ctx.Step(`^reading it back shows no throttle limit$`, w.firstPlanHasNoThrottle)

	// Project pagination.
	ctx.Step(`^(\d+) projects exist$`, w.projectsExist)
	ctx.Step(`^I page through projects (\d+) at a time$`, w.pageProjects)
	ctx.Step(`^every project is seen exactly once$`, w.everyProjectSeenOnce)

	// Subscription listing.
	ctx.Step(`^listing subscriptions with no filter returns (\d+)$`, w.listSubscriptionsNoFilter)
	ctx.Step(`^listing subscriptions filtered by status "([^"]*)" returns (\d+)$`, w.listSubscriptionsByStatus)

	// Application lookup.
	ctx.Step(`^an organization, project and application exist$`, w.orgProjectApplicationExist)
	ctx.Step(`^the application is found by its UUID$`, w.applicationFoundByUUID)
	ctx.Step(`^the application is found by its handle$`, w.applicationFoundByHandle)
	ctx.Step(`^a missing application identifier returns nothing$`, w.missingApplicationReturnsNil)

	ctx.Step(`^an organization and project exist$`, w.anOrgAndProjectExist)
}

// --- Organization pagination ------------------------------------------------

func (w *world) additionalOrgsExist(n int) error {
	orgRepo := repository.NewOrganizationRepo(w.it.db)
	baseline, err := orgRepo.ListOrganizations(1_000_000, 0)
	if err != nil {
		return fmt.Errorf("[%s] baseline list failed: %w", w.it.driver, err)
	}
	for i := range n {
		org := &model.Organization{ID: id(), Handle: "lc-" + id()[:8], Name: fmt.Sprintf("org %d", i), Region: "us"}
		if err := orgRepo.CreateOrganization(org); err != nil {
			return fmt.Errorf("[%s] create org failed: %w", w.it.driver, err)
		}
	}
	w.orgTotal = len(baseline) + n
	return nil
}

func (w *world) pageOrganizations(pageSize int) error {
	orgRepo := repository.NewOrganizationRepo(w.it.db)
	seen := map[string]bool{}
	for offset := 0; offset < w.orgTotal; offset += pageSize {
		page, err := orgRepo.ListOrganizations(pageSize, offset)
		if err != nil {
			return fmt.Errorf("[%s] ListOrganizations(%d,%d) failed: %w", w.it.driver, pageSize, offset, err)
		}
		want := pageSize
		if rem := w.orgTotal - offset; rem < want {
			want = rem
		}
		if len(page) != want {
			return fmt.Errorf("[%s] page at offset %d: want %d rows, got %d", w.it.driver, offset, want, len(page))
		}
		for _, o := range page {
			if seen[o.ID] {
				return fmt.Errorf("[%s] pagination overlap at offset %d: id %s seen twice", w.it.driver, offset, o.ID)
			}
			seen[o.ID] = true
		}
	}
	w.seenCount = len(seen)
	return nil
}

func (w *world) everyOrgSeenOnce() error {
	if w.seenCount != w.orgTotal {
		return fmt.Errorf("[%s] paging covered %d rows, want %d", w.it.driver, w.seenCount, w.orgTotal)
	}
	return nil
}

// --- Subscription plan ------------------------------------------------------

func (w *world) anOrganizationExists() error {
	orgRepo := repository.NewOrganizationRepo(w.it.db)
	org := &model.Organization{ID: id(), Handle: "pl-" + id()[:8], Name: "plan org", Region: "us"}
	if err := orgRepo.CreateOrganization(org); err != nil {
		return fmt.Errorf("[%s] create org failed: %w", w.it.driver, err)
	}
	w.orgID = org.ID
	return nil
}

func (w *world) missingPlanNotExists() error {
	planRepo := repository.NewSubscriptionPlanRepo(w.it.db)
	exists, err := planRepo.ExistsByHandleAndOrg("nope-"+id(), w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] ExistsByHandleAndOrg failed: %w", w.it.driver, err)
	}
	if exists {
		return fmt.Errorf("[%s] ExistsByHandleAndOrg: want false for missing plan", w.it.driver)
	}
	return nil
}

func (w *world) createSubscriptionPlans(n, limit int, unit string) error {
	planRepo := repository.NewSubscriptionPlanRepo(w.it.db)
	w.throttle = limit
	w.throttleUnit = unit
	for i := range n {
		count := limit
		slug := fmt.Sprintf("plan-%d-%s", i, id()[:6])
		plan := &model.SubscriptionPlan{
			UUID: id(), Handle: slug, Name: fmt.Sprintf("Plan %d", i),
			StopOnQuotaReach:   true,
			ThrottleLimitCount: &count, ThrottleLimitUnit: unit,
			OrganizationUUID: w.orgID, Status: model.SubscriptionPlanStatus("ACTIVE"),
		}
		if err := planRepo.Create(plan); err != nil {
			return fmt.Errorf("[%s] create plan failed: %w", w.it.driver, err)
		}
	}
	return nil
}

func (w *world) listSubscriptionPlans(pageSize, want int) error {
	planRepo := repository.NewSubscriptionPlanRepo(w.it.db)
	plans, err := planRepo.ListByOrganization(w.orgID, pageSize, 0)
	if err != nil {
		return fmt.Errorf("[%s] ListByOrganization failed: %w", w.it.driver, err)
	}
	if len(plans) != want {
		return fmt.Errorf("[%s] ListByOrganization(%d,0): want %d, got %d", w.it.driver, pageSize, want, len(plans))
	}
	w.listedPlanID = plans[0].UUID
	return nil
}

func (w *world) listedPlansHaveThrottle() error {
	planRepo := repository.NewSubscriptionPlanRepo(w.it.db)
	plans, err := planRepo.ListByOrganization(w.orgID, 2, 0)
	if err != nil {
		return fmt.Errorf("[%s] ListByOrganization failed: %w", w.it.driver, err)
	}
	for _, p := range plans {
		if p.ThrottleLimitCount == nil || *p.ThrottleLimitCount != w.throttle {
			return fmt.Errorf("[%s] list hydrate: ThrottleLimitCount = %v, want %d", w.it.driver, p.ThrottleLimitCount, w.throttle)
		}
		if p.ThrottleLimitUnit != w.throttleUnit || !p.StopOnQuotaReach {
			return fmt.Errorf("[%s] list hydrate: unit=%q stop=%v, want unit=%s stop=true", w.it.driver, p.ThrottleLimitUnit, p.StopOnQuotaReach, w.throttleUnit)
		}
	}
	return nil
}

func (w *world) firstPlanGetHasThrottle() error {
	planRepo := repository.NewSubscriptionPlanRepo(w.it.db)
	got, err := planRepo.GetByID(w.listedPlanID, w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] GetByID failed: %w", w.it.driver, err)
	}
	if got.ThrottleLimitCount == nil || *got.ThrottleLimitCount != w.throttle || got.ThrottleLimitUnit != w.throttleUnit {
		return fmt.Errorf("[%s] GetByID hydrate: count=%v unit=%q, want %d/%s", w.it.driver, got.ThrottleLimitCount, got.ThrottleLimitUnit, w.throttle, w.throttleUnit)
	}
	return nil
}

func (w *world) clearFirstPlanThrottle() error {
	planRepo := repository.NewSubscriptionPlanRepo(w.it.db)
	got, err := planRepo.GetByID(w.listedPlanID, w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] GetByID failed: %w", w.it.driver, err)
	}
	got.ThrottleLimitCount = nil
	got.ThrottleLimitUnit = ""
	got.StopOnQuotaReach = true
	if err := planRepo.Update(got); err != nil {
		return fmt.Errorf("[%s] Update (clear throttle) failed: %w", w.it.driver, err)
	}
	return nil
}

func (w *world) firstPlanHasNoThrottle() error {
	planRepo := repository.NewSubscriptionPlanRepo(w.it.db)
	cleared, err := planRepo.GetByID(w.listedPlanID, w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] GetByID after clear failed: %w", w.it.driver, err)
	}
	if cleared.ThrottleLimitCount != nil || cleared.ThrottleLimitUnit != "" {
		return fmt.Errorf("[%s] after clear: want no throttle, got count=%v unit=%q", w.it.driver, cleared.ThrottleLimitCount, cleared.ThrottleLimitUnit)
	}
	return nil
}

// --- Project pagination -----------------------------------------------------

func (w *world) projectsExist(n int) error {
	projectRepo := repository.NewProjectRepo(w.it.db)
	for i := range n {
		pid := id()
		pName := fmt.Sprintf("proj-%d-%s", i, pid[:6])
		p := &model.Project{ID: pid, Handle: pName, Name: pName, OrganizationID: w.orgID, Description: "p"}
		if err := projectRepo.CreateProject(p); err != nil {
			return fmt.Errorf("[%s] create project failed: %w", w.it.driver, err)
		}
	}
	w.projN = n
	return nil
}

func (w *world) pageProjects(pageSize int) error {
	projectRepo := repository.NewProjectRepo(w.it.db)
	seen := map[string]bool{}
	for offset := 0; offset < w.projN; offset += pageSize {
		page, err := projectRepo.ListProjects(w.orgID, pageSize, offset)
		if err != nil {
			return fmt.Errorf("[%s] ListProjects(%d,%d) failed: %w", w.it.driver, pageSize, offset, err)
		}
		want := pageSize
		if rem := w.projN - offset; rem < want {
			want = rem
		}
		if len(page) != want {
			return fmt.Errorf("[%s] ListProjects offset %d: want %d, got %d", w.it.driver, offset, want, len(page))
		}
		for _, p := range page {
			if seen[p.ID] {
				return fmt.Errorf("[%s] project pagination overlap at offset %d: %s", w.it.driver, offset, p.ID)
			}
			seen[p.ID] = true
		}
	}
	w.seenCount = len(seen)
	return nil
}

func (w *world) everyProjectSeenOnce() error {
	if w.seenCount != w.projN {
		return fmt.Errorf("[%s] project paging covered %d, want %d", w.it.driver, w.seenCount, w.projN)
	}
	return nil
}

// --- Subscription listing by filter -----------------------------------------

func (w *world) listSubscriptionsNoFilter(want int) error {
	subRepo := repository.NewSubscriptionRepo(w.it.db)
	all, err := subRepo.ListByFilters(w.g.org, nil, nil, nil, nil, 10, 0)
	if err != nil {
		return fmt.Errorf("[%s] ListByFilters (no filter) failed: %w", w.it.driver, err)
	}
	if len(all) != want {
		return fmt.Errorf("[%s] ListByFilters: want %d subscriptions, got %d", w.it.driver, want, len(all))
	}
	return nil
}

func (w *world) listSubscriptionsByStatus(status string, want int) error {
	subRepo := repository.NewSubscriptionRepo(w.it.db)
	s := status
	got, err := subRepo.ListByFilters(w.g.org, nil, nil, nil, &s, 10, 0)
	if err != nil {
		return fmt.Errorf("[%s] ListByFilters (status=%s) failed: %w", w.it.driver, status, err)
	}
	if len(got) != want {
		return fmt.Errorf("[%s] ListByFilters(status=%s): want %d, got %d", w.it.driver, status, want, len(got))
	}
	return nil
}

// --- Application lookup ------------------------------------------------------

func (w *world) orgProjectApplicationExist() error {
	orgRepo := repository.NewOrganizationRepo(w.it.db)
	projectRepo := repository.NewProjectRepo(w.it.db)
	appRepo := repository.NewApplicationRepo(w.it.db, repository.NewArtifactTableRegistry())

	org := &model.Organization{ID: id(), Handle: "ap-" + id()[:8], Name: "app org", Region: "us"}
	if err := orgRepo.CreateOrganization(org); err != nil {
		return fmt.Errorf("[%s] create org failed: %w", w.it.driver, err)
	}
	projID := id()
	projName := "p-" + projID[:6]
	proj := &model.Project{ID: projID, Handle: projName, Name: projName, OrganizationID: org.ID}
	if err := projectRepo.CreateProject(proj); err != nil {
		return fmt.Errorf("[%s] create project failed: %w", w.it.driver, err)
	}
	app := &model.Application{
		UUID: id(), Handle: "app-" + id()[:8], ProjectUUID: proj.ID,
		OrganizationUUID: org.ID, Name: "app", Type: "standard",
	}
	if err := appRepo.CreateApplication(app); err != nil {
		return fmt.Errorf("[%s] create application failed: %w", w.it.driver, err)
	}
	w.orgID = org.ID
	w.appUUID = app.UUID
	w.appHandle = app.Handle
	return nil
}

func (w *world) applicationFoundByUUID() error {
	appRepo := repository.NewApplicationRepo(w.it.db, repository.NewArtifactTableRegistry())
	byUUID, err := appRepo.GetApplicationByIDOrHandle(w.appUUID, w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] GetApplicationByIDOrHandle(uuid) failed: %w", w.it.driver, err)
	}
	if byUUID == nil || byUUID.UUID != w.appUUID {
		return fmt.Errorf("[%s] lookup by uuid: want %s, got %+v", w.it.driver, w.appUUID, byUUID)
	}
	return nil
}

func (w *world) applicationFoundByHandle() error {
	appRepo := repository.NewApplicationRepo(w.it.db, repository.NewArtifactTableRegistry())
	byHandle, err := appRepo.GetApplicationByIDOrHandle(w.appHandle, w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] GetApplicationByIDOrHandle(handle) failed: %w", w.it.driver, err)
	}
	if byHandle == nil || byHandle.UUID != w.appUUID {
		return fmt.Errorf("[%s] lookup by handle: want %s, got %+v", w.it.driver, w.appUUID, byHandle)
	}
	return nil
}

func (w *world) missingApplicationReturnsNil() error {
	appRepo := repository.NewApplicationRepo(w.it.db, repository.NewArtifactTableRegistry())
	missing, err := appRepo.GetApplicationByIDOrHandle("does-not-exist-"+id(), w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] GetApplicationByIDOrHandle(missing) failed: %w", w.it.driver, err)
	}
	if missing != nil {
		return fmt.Errorf("[%s] lookup of missing app: want nil, got %+v", w.it.driver, missing)
	}
	return nil
}
