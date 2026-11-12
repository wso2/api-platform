/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package apiportal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

// The limits the free plan journey sets, before and after its edit.
const (
	freePlanMinuteLimit       = "500"
	freePlanMinuteLimitEdited = "600"
	freePlanDayLimit          = "20000"
)

// registerPlanSteps wires the free subscription plan journey steps.
func (u *Steps) registerPlanSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the administrator adds a free plan with a per-minute and a per-day limit in the settings$`, u.addFreePlan)
	sc.Step(`^the administrator raises that plan's per-minute limit$`, u.editFreePlanLimit)
	sc.Step(`^the administrator offers that plan on an API in the settings$`, u.offerPlanOnAPI)
	sc.Step(`^the API page shows that plan's card with both of its limits$`, u.planCardOnAPIPage)
	sc.Step(`^that plan cannot be deleted while a developer subscribes to it$`, u.planDeleteRefused)
}

// addFreePlan adds a plan with two request limits, refusing a limit count that is not positive.
func (u *Steps) addFreePlan(ctx context.Context) error {
	page, err := u.openSettingsPanel(ctx, "cfg-plans")
	if err != nil {
		return err
	}
	name, err := u.openNewPlan(ctx, page, "ui-free")
	if err != nil {
		return err
	}
	if err := fillAndCheck(page, "#pol-desc", "A free plan with two limits"); err != nil {
		return err
	}
	limits := page.Locator("#pol-limits-list .cfg-limit-row")
	for range 2 {
		if err := page.Locator("#pol-add-limit-btn").Click(); err != nil {
			return fmt.Errorf("adding a limit: %w", err)
		}
	}
	if err := u.expect.Locator(limits).ToHaveCount(2); err != nil {
		return err
	}
	for i, limit := range []struct{ count, unit string }{{"0", "MINUTE"}, {freePlanDayLimit, "DAY"}} {
		row := limits.Nth(i)
		if _, err := row.Locator(".cfg-limit-type").SelectOption(playwright.SelectOptionValues{Values: &[]string{"REQUEST_COUNT"}}); err != nil {
			return err
		}
		if err := row.Locator(".cfg-limit-count").Fill(limit.count); err != nil {
			return err
		}
		if err := row.Locator(".cfg-limit-amount").Fill("1"); err != nil {
			return err
		}
		if _, err := row.Locator(".cfg-limit-unit").SelectOption(playwright.SelectOptionValues{Values: &[]string{limit.unit}}); err != nil {
			return err
		}
	}
	if err := u.saveExpecting(page, "#cfg-plan-modal-save", "Limit count must be -1 (unlimited) or a positive number."); err != nil {
		return err
	}
	if err := limits.Nth(0).Locator(".cfg-limit-count").Fill(freePlanMinuteLimit); err != nil {
		return err
	}
	if err := u.saveAndReload(page, page.Locator("#cfg-plan-modal-save"), "Plan created."); err != nil {
		return err
	}
	if err := u.registerPlan(ctx, page, name); err != nil {
		return err
	}
	return u.expect.Locator(planRow(page, name)).ToBeVisible()
}

// editFreePlanLimit raises the plan's per-minute limit and checks the stored plan.
func (u *Steps) editFreePlanLimit(ctx context.Context) error {
	name, err := resolveText(ctx, keyBillingPlanName)
	if err != nil {
		return err
	}
	planID, err := resolveText(ctx, keyBillingPlan)
	if err != nil {
		return err
	}
	page, err := u.openSettingsPanel(ctx, "cfg-plans")
	if err != nil {
		return err
	}
	if err := planRow(page, name).Locator(".cfg-plan-edit-btn").Click(); err != nil {
		return fmt.Errorf("opening the plan editor: %w", err)
	}
	limits := page.Locator("#pol-limits-list .cfg-limit-row")
	if err := u.expect.Locator(limits).ToHaveCount(2); err != nil {
		return err
	}
	minute, err := limitRowByUnit(limits, "MINUTE")
	if err != nil {
		return err
	}
	if err := u.expect.Locator(minute.Locator(".cfg-limit-count")).ToHaveValue(freePlanMinuteLimit); err != nil {
		return err
	}
	if err := minute.Locator(".cfg-limit-count").Fill(freePlanMinuteLimitEdited); err != nil {
		return err
	}
	if err := u.saveAndReload(page, page.Locator("#cfg-plan-modal-save"), "Plan updated."); err != nil {
		return err
	}
	status, body, err := u.portalCall(page, http.MethodGet, "/subscription-plans/"+planID, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("reading plan %q: status %d", planID, status)
	}
	var plan struct {
		Limits []struct {
			LimitCount int    `json:"limitCount"`
			TimeUnit   string `json:"timeUnit"`
		} `json:"limits"`
	}
	if err := decodeJSONBody(body, &plan); err != nil {
		return err
	}
	stored := map[string]int{}
	for _, limit := range plan.Limits {
		stored[limit.TimeUnit] = limit.LimitCount
	}
	if fmt.Sprint(stored["MINUTE"]) != freePlanMinuteLimitEdited || fmt.Sprint(stored["DAY"]) != freePlanDayLimit {
		return fmt.Errorf("plan %q is stored with limits %v", planID, stored)
	}
	return nil
}

// limitRowByUnit finds the plan form's limit row with a time unit; the form does not keep the
// order the limits were added in.
func limitRowByUnit(limits playwright.Locator, unit string) (playwright.Locator, error) {
	count, err := limits.Count()
	if err != nil {
		return nil, err
	}
	for i := range count {
		row := limits.Nth(i)
		value, err := row.Locator(".cfg-limit-unit").InputValue()
		if err != nil {
			return nil, err
		}
		if value == unit {
			return row, nil
		}
	}
	return nil, fmt.Errorf("the plan form has no limit per %s", unit)
}

// offerPlanOnAPI creates an API, then adds the plan to it in the API's wizard.
func (u *Steps) offerPlanOnAPI(ctx context.Context) error {
	planID, err := resolveText(ctx, keyBillingPlan)
	if err != nil {
		return err
	}
	generated, err := unique.Unique(ctx, "plan-api")
	if err != nil {
		return err
	}
	// The wizard validates the handle on edit too, so the fixture uses a handle it accepts.
	id := wizardHandle(generated)
	api := catalogueAPI{ID: id, Name: "IT Plan API " + id, Type: "REST", Badge: "REST"}
	if err := u.createCatalogueAPI(ctx, api); err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyAPIPortalWizardItem, id); err != nil {
		return err
	}
	page, err := u.openWizardEditIn(ctx, wizardPanelAPIs, "Edit API")
	if err != nil {
		return err
	}
	if err := page.Locator("#wz-policy-search").Fill("Plan"); err != nil {
		return err
	}
	if err := page.Locator(`#wz-policy-dropdown .cfg-chip-dd-item[data-name="`+planID+`"]`).DispatchEvent("mousedown", nil); err != nil {
		return fmt.Errorf("choosing the plan: %w", err)
	}
	if err := u.wizardEditToAssets(page); err != nil {
		return err
	}
	if err := page.Locator(wizardNext).Click(); err != nil {
		return fmt.Errorf("saving the API: %w", err)
	}
	if err := u.wizardSaved(page); err != nil {
		return err
	}
	return u.waitForPortalCards(ctx, "/apis", id)
}

// planCardOnAPIPage checks the plan's card on the API page and its two limits.
func (u *Steps) planCardOnAPIPage(ctx context.Context) error {
	planID, err := resolveText(ctx, keyBillingPlan)
	if err != nil {
		return err
	}
	name, err := resolveText(ctx, keyBillingPlanName)
	if err != nil {
		return err
	}
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	if err := u.openPortalPage(ctx, "/api/"+id); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	card := page.Locator(`#subscriptionPlans .aov-plan-card[data-policy-name="` + planID + `"]`)
	if err := u.expect.Locator(card.Locator(".aov-plan-name")).ToHaveText(name); err != nil {
		return err
	}
	for _, count := range []string{freePlanMinuteLimitEdited, freePlanDayLimit} {
		limit := card.Locator(".aov-plan-count").Filter(playwright.LocatorFilterOptions{HasText: regexpExact(count)})
		if err := u.expect.Locator(limit).ToHaveCount(1); err != nil {
			return fmt.Errorf("the plan card does not show the limit %s: %w", count, err)
		}
	}
	return u.expect.Locator(card.Locator(".aov-plan-count")).ToHaveCount(2)
}

// planDeleteRefused subscribes the developer to the plan, then checks as the administrator that
// the delete dialog explains the plan is in use and offers no delete, and that the plan is kept.
func (u *Steps) planDeleteRefused(ctx context.Context) error {
	planID, err := resolveText(ctx, keyBillingPlan)
	if err != nil {
		return err
	}
	name, err := resolveText(ctx, keyBillingPlanName)
	if err != nil {
		return err
	}
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	if err := u.loginAsDeveloper(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.openAPIPortal(ctx); err != nil {
		return err
	}
	if _, err := u.createSubscription(ctx, page, id, planID); err != nil {
		return err
	}
	if err := u.loginAs(ctx, u.topo.Admin.Username, u.topo.Admin.Password); err != nil {
		return err
	}
	page, err = u.openSettingsPanel(ctx, "cfg-plans")
	if err != nil {
		return err
	}
	row := planRow(page, name)
	if err := row.Locator(".cfg-plan-delete-btn").Click(); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#cfg-del-plan-blocked")).ToHaveText(name + " is in use and can't be deleted."); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#cfg-del-plan-ask")).ToBeHidden(); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#cfg-del-plan-confirm")).ToBeDisabled(); err != nil {
		return err
	}
	if err := page.Locator("#cfg-del-plan-cancel").Click(); err != nil {
		return err
	}
	status, _, err := u.portalCall(page, http.MethodGet, "/subscription-plans/"+planID, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("GET /subscription-plans/%s = %d after the refused delete, want 200", planID, status)
	}
	return nil
}

// decodeJSONBody decodes a JSON response body read through portalCall.
func decodeJSONBody(body string, out any) error {
	if err := json.Unmarshal([]byte(body), out); err != nil {
		return fmt.Errorf("decoding the response: %w", err)
	}
	return nil
}

// regexpExact matches text that is exactly value.
func regexpExact(value string) *regexp.Regexp {
	return regexp.MustCompile("^" + regexp.QuoteMeta(value) + "$")
}
