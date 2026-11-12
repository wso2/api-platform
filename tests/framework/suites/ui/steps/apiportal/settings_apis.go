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
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

const (
	keyBulkDeleteAPIs = "uiAPIPortalBulkDeleteAPIs"
	keyLLMView        = "uiAPIPortalLLMView"

	// wizardTag and wizardPlan are what the wizard journey attaches to the API it creates.
	wizardTag  = "wizard-journey"
	wizardPlan = "Bronze"
)

// registerSettingsAPISteps wires the steps of the journeys in which an administrator manages APIs
// and their content in the settings.
func (u *Steps) registerSettingsAPISteps(sc *godog.ScenarioContext) {
	sc.Step(`^the wizard refuses to leave the details step while its required fields are empty$`, u.wizardRequiresDetails)
	sc.Step(`^the wizard API shows in the catalogue with its type, tag, plan and documents$`, u.wizardAPIInCatalogue)
	sc.Step(`^the user (deprecates|publishes) the wizard (API|MCP server) from its row menu$`, u.changeWizardAPIStatus)
	sc.Step(`^the catalogue (marks|no longer marks) the wizard API as deprecated$`, u.catalogueMarksDeprecated)
	sc.Step(`^the API Portal has two APIs without subscriptions and one a developer subscribed to$`, u.seedBulkDeleteAPIs)
	sc.Step(`^the user deletes those three APIs together from the settings table$`, u.bulkDeleteAPIs)
	sc.Step(`^the two APIs without subscriptions are deleted and the subscribed API is kept$`, u.bulkDeleteResult)
	sc.Step(`^the wizard API page shows the imported landing page and icon, and keeps its plans$`, u.wizardAPIPageShowsContent)
	sc.Step(`^the API Portal has a view of its own for LLM instructions$`, u.seedLLMView)
	sc.Step(`^the user sets the LLM instructions of that view and previews them$`, u.setLLMInstructions)
	sc.Step(`^the llms\.txt of that view uses the saved instructions$`, u.llmsTxtUsesInstructions)
	sc.Step(`^the user turns AI discovery off for that view$`, u.turnAIDiscoveryOff)
	sc.Step(`^that view serves no llms\.txt and no longer offers AI Ready or Try with AI$`, u.aiDiscoveryIsOff)
}

// wizardPickLabelPlanAndTag attaches the default label, the Bronze plan and a tag on the Details
// step, so the API shows in the default view with a plan to subscribe to.
func (u *Steps) wizardPickLabelPlanAndTag(page playwright.Page) error {
	label := page.Locator(`#wz-labels .cfg-label-toggle[data-value="default"]`)
	pressed, err := label.GetAttribute("aria-pressed")
	if err != nil {
		return fmt.Errorf("reading the default label: %w", err)
	}
	if pressed != "true" {
		if err := label.Click(); err != nil {
			return fmt.Errorf("selecting the default label: %w", err)
		}
	}
	if err := u.expect.Locator(label).ToHaveAttribute("aria-pressed", "true"); err != nil {
		return err
	}
	if err := page.Locator("#wz-policy-search").Fill(wizardPlan); err != nil {
		return fmt.Errorf("searching the plans: %w", err)
	}
	option := page.Locator(`#wz-policy-dropdown .cfg-chip-dd-item[data-name="` + wizardPlan + `"]`)
	if err := option.DispatchEvent("mousedown", nil); err != nil {
		return fmt.Errorf("choosing the %s plan: %w", wizardPlan, err)
	}
	if err := u.expect.Locator(page.Locator("#wz-policy-chips .cfg-chip")).ToHaveText([]string{wizardPlan}); err != nil {
		return err
	}
	if err := page.Locator("#wz-tags-input").Fill(wizardTag); err != nil {
		return fmt.Errorf("typing the tag: %w", err)
	}
	if err := page.Locator("#wz-tags-input").Press("Enter"); err != nil {
		return fmt.Errorf("adding the tag: %w", err)
	}
	return u.expect.Locator(page.Locator("#wz-tags-chips .cfg-chip")).ToContainText([]string{wizardTag})
}

// wizardRequiresDetails opens the create wizard and checks its Details validation: Next stays
// disabled until name, version and handle are filled; then a malformed handle, a missing type, an
// invalid production URL and an invalid owner email each report an error and keep the wizard on
// Details. The wizard is left without saving, so nothing is stored.
func (u *Steps) wizardRequiresDetails(ctx context.Context) error {
	page, err := u.openWizardPanel(ctx, wizardPanelAPIs)
	if err != nil {
		return err
	}
	next := page.Locator(wizardNext)
	return wizardAll(
		wizardClick(page, "#cfg-add-api-btn"),
		u.wizardHasValue(page, "#wz-name", ""),
		func() error { return u.expect.Locator(next).ToBeDisabled() },
		wizardFill(page, "#wz-name", "Required fields"),
		func() error { return u.expect.Locator(next).ToBeDisabled() },
		wizardFill(page, "#wz-version", "v1.0"),
		u.wizardHasValue(page, "#wz-handle", "required-fields-v1.0"),
		func() error { return u.expect.Locator(next).ToBeEnabled() },
		wizardFill(page, "#wz-handle", "Not A Handle!"),
		wizardFill(page, "#wz-prod", "not-a-url"),
		wizardFill(page, "#wz-biz-email", "not-an-email"),
		wizardClick(page, wizardNext),
		u.wizardHasText(page, "#wz-handle-error", "Must be lowercase letters, numbers, hyphens, and dots only."),
		u.wizardHasText(page, "#wz-type-error", "API type is required."),
		u.wizardHasText(page, "#wz-prod-error", "Enter a valid http:// or https:// URL."),
		u.wizardVisible(page, "#wz-biz-email-error"),
		u.wizardHidden(page, "#wz-name-error"),
		u.wizardVisible(page, "#cfg-wstep-0"),
		u.wizardHidden(page, "#cfg-wstep-1"),
		wizardClick(page, "#cfg-wizard-back-link"),
		u.wizardHidden(page, "#cfg-apis-wizard"),
		u.wizardVisible(page, "#cfg-apis-list"),
		func() error { return u.expect.Locator(wizardRow(page, "Not A Handle!")).ToHaveCount(0) },
	)
}

// wizardAPIInCatalogue opens the default view and checks the wizard API's card and its
// documentation: the REST badge, the tag, one plan, and both documents in the navigation.
func (u *Steps) wizardAPIInCatalogue(ctx context.Context) error {
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	if err := u.waitForPortalCards(ctx, "/apis", id); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	card := page.Locator("#apiCard-" + id)
	for selector, text := range map[string]string{
		".api-card-title":                      "IT Wizard " + id,
		".api-card-meta-row > .dp-badge--rest": "REST",
		".dp-tag":                              wizardTag,
		".api-plans-link":                      "1 plan",
	} {
		if err := u.expect.Locator(card.Locator(selector)).ToContainText(text); err != nil {
			return fmt.Errorf("the wizard API card %s: %w", selector, err)
		}
	}
	if err := u.openPortalPage(ctx, "/api/"+id+"/docs/specification"); err != nil {
		return err
	}
	nav := page.Locator(".adoc-nav")
	if err := u.expect.Locator(nav).ToContainText("API Definition"); err != nil {
		return err
	}
	return u.expect.Locator(nav.Locator(`a.doc-link[href*="/docs/Other/"]`)).ToHaveText([]string{"A", "B"})
}

// changeWizardAPIStatus deprecates or publishes the wizard API or MCP server from its settings row
// menu and waits for the row to show the new status.
func (u *Steps) changeWizardAPIStatus(ctx context.Context, action, kind string) error {
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	panel := wizardPanelAPIs
	if kind == "MCP server" {
		panel = wizardPanelMCPs
	}
	page, err := u.openWizardPanel(ctx, panel)
	if err != nil {
		return err
	}
	row := wizardRow(page, id)
	if err := u.revealOnPage(page, row); err != nil {
		return err
	}
	if err := row.Locator(".cfg-menu-trigger").Click(); err != nil {
		return fmt.Errorf("opening the row menu: %w", err)
	}
	want, toast := "Deprecated", " deprecated."
	if action == "deprecates" {
		if err := row.Locator(".cfg-unpublish-trigger").Click(); err != nil {
			return fmt.Errorf("choosing Deprecate: %w", err)
		}
		if err := u.expect.Locator(page.Locator("#cfg-dep-api-name-txt")).ToHaveText("IT Wizard " + id); err != nil {
			return err
		}
		if err := page.Locator("#cfg-dep-api-confirm").Click(); err != nil {
			return fmt.Errorf("confirming Deprecate: %w", err)
		}
	} else {
		want, toast = "Published", " published."
		if err := row.Locator(".cfg-publish-trigger").Click(); err != nil {
			return fmt.Errorf("choosing Publish: %w", err)
		}
	}
	if err := u.expectToast(page, "IT Wizard "+id+toast); err != nil {
		return err
	}
	return u.expect.Locator(wizardRow(page, id).Locator(".cfg-status-badge")).ToHaveText(want)
}

// catalogueMarksDeprecated checks the Deprecated badge on the wizard API's catalogue card.
func (u *Steps) catalogueMarksDeprecated(ctx context.Context, marks string) error {
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	if err := u.openAPIListing(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	card := page.Locator("#apiCard-" + id)
	if err := u.revealOnPage(page, card); err != nil {
		return err
	}
	return u.expectCount(card.Locator(".dp-badge--deprecated"), marks == "marks")
}

// seedBulkDeleteAPIs creates three APIs with the Bronze plan and subscribes the developer to the
// third. The administrator is signed in again afterwards.
func (u *Steps) seedBulkDeleteAPIs(ctx context.Context) error {
	ids := make([]string, 0, 3)
	for i := range 3 {
		id, err := unique.Unique(ctx, fmt.Sprintf("bulk-delete-%d", i+1))
		if err != nil {
			return err
		}
		api := catalogueAPI{ID: id, Name: "IT Bulk Delete " + id, Type: "REST", Badge: "REST", Plans: []string{"Bronze"}}
		if err := u.createCatalogueAPI(ctx, api); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := tcontext.Set(ctx, keyBulkDeleteAPIs, strings.Join(ids, ",")); err != nil {
		return err
	}
	if err := u.waitForPortalCards(ctx, "/apis", ids...); err != nil {
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
	if _, err := u.createSubscription(ctx, page, ids[2], "Bronze"); err != nil {
		return err
	}
	return u.loginAs(ctx, u.topo.Admin.Username, u.topo.Admin.Password)
}

// bulkDeleteIDs returns the three APIs seedBulkDeleteAPIs created; the third is subscribed.
func bulkDeleteIDs(ctx context.Context) ([]string, error) {
	raw, err := contextString(ctx, keyBulkDeleteAPIs)
	if err != nil {
		return nil, err
	}
	return strings.Split(raw, ","), nil
}

// bulkDeleteAPIs selects the three APIs in the settings table and deletes them together.
func (u *Steps) bulkDeleteAPIs(ctx context.Context) error {
	ids, err := bulkDeleteIDs(ctx)
	if err != nil {
		return err
	}
	page, err := u.openWizardPanel(ctx, wizardPanelAPIs)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := u.revealOnPage(page, wizardRow(page, id)); err != nil {
			return err
		}
		if err := wizardRow(page, id).Locator(".cfg-row-check").Check(); err != nil {
			return fmt.Errorf("selecting %s: %w", id, err)
		}
	}
	if err := u.expect.Locator(page.Locator("#cfg-bulk-bar-count")).ToHaveText("3 APIs selected"); err != nil {
		return err
	}
	if err := page.Locator("#cfg-bulk-delete-btn").Click(); err != nil {
		return fmt.Errorf("deleting the selected APIs: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#cfg-del-api-name-txt")).ToHaveText("3 selected APIs"); err != nil {
		return err
	}
	if err := page.Locator("#cfg-del-api-confirm").Click(); err != nil {
		return fmt.Errorf("confirming the delete: %w", err)
	}
	return u.expectToast(page, "1 record(s) could not be deleted.")
}

// bulkDeleteResult checks that the two APIs without subscriptions are gone from the table and the
// store, while the subscribed API stays in both.
func (u *Steps) bulkDeleteResult(ctx context.Context) error {
	ids, err := bulkDeleteIDs(ctx)
	if err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	registry, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	for i, id := range ids {
		kept := i == 2
		if err := u.expectCount(wizardRow(page, id), kept); err != nil {
			return fmt.Errorf("the settings row of %s: %w", id, err)
		}
		status, _, err := u.portalCall(page, http.MethodGet, "/apis/"+id, nil)
		if err != nil {
			return err
		}
		want := http.StatusNotFound
		if kept {
			want = http.StatusOK
		}
		if status != want {
			return fmt.Errorf("GET /apis/%s = %d after the bulk delete, want %d", id, status, want)
		}
		if !kept {
			registry.Deregister(KindAPIPortalAPI, id)
		}
	}
	return nil
}

// wizardAPIPageShowsContent opens the wizard API's page after a content package was imported and
// checks the landing page body and icon, with the plans panel kept below them.
func (u *Steps) wizardAPIPageShowsContent(ctx context.Context) error {
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
	if err := u.expect.Locator(page.Locator("body")).ToContainText("Landing page"); err != nil {
		return err
	}
	icon := page.Locator(".aov-hero-thumb img.aov-api-img")
	if err := u.expect.Locator(icon).ToHaveAttribute("src", regexp.MustCompile(regexp.QuoteMeta(wizardIconPNG))); err != nil {
		return err
	}
	if err := u.imageLoaded(ctx, icon); err != nil {
		return err
	}
	return u.expect.Locator(page.Locator("#subscriptionPlans .aov-plan-card")).ToHaveCount(1)
}

// seedLLMView creates a view with the default label, so LLM instruction changes stay inside the
// journey, and waits until it lists APIs.
func (u *Steps) seedLLMView(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	view, err := unique.Unique(ctx, "llm-view")
	if err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalView, ID: view, Actor: u.topo.Admin.Username}); err != nil {
		return fmt.Errorf("registering view %q for cleanup: %w", view, err)
	}
	status, body, err := u.portalCall(page, http.MethodPost, "/views", map[string]any{
		"id": view, "displayName": "IT LLM View " + view, "labels": []string{"default"},
	})
	if err != nil {
		return err
	}
	if status != http.StatusCreated {
		return fmt.Errorf("creating view %q: status %d %s", view, status, body)
	}
	return tcontext.Set(ctx, keyLLMView, view)
}

// llmText returns the portal name and description the journey writes for its view.
func llmText(view string) (string, string) {
	return "IT Portal " + view, "Instructions for agents of " + view + "."
}

// openLLMPanel opens the LLM Instructions panel and selects the journey's view in its view
// selector.
func (u *Steps) openLLMPanel(ctx context.Context, view string) (playwright.Page, error) {
	page, err := u.openSettingsPanel(ctx, "cfg-llm")
	if err != nil {
		return nil, err
	}
	panel := page.Locator("#cfg-llm")
	if err := panel.Locator(".cfg-view-combo-trigger").Click(); err != nil {
		return nil, fmt.Errorf("opening the view selector: %w", err)
	}
	if err := panel.Locator(".cfg-view-combo-input").Fill(view); err != nil {
		return nil, fmt.Errorf("filtering the views: %w", err)
	}
	if err := panel.Locator(`.cfg-view-combo-option[data-value="` + view + `"]`).Click(); err != nil {
		return nil, fmt.Errorf("choosing the view: %w", err)
	}
	// Choosing a view reloads the settings page for it, which takes longer than an assertion's
	// default wait.
	if err := page.WaitForURL(regexp.MustCompile(`[?&]view=`+regexp.QuoteMeta(view)+`(#|&|$)`), playwright.PageWaitForURLOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return nil, fmt.Errorf("waiting for the settings of view %q: %w", view, err)
	}
	if err := u.expect.Locator(page.Locator("#cfg-llm .cfg-view-combo-current")).ToHaveText("IT LLM View " + view); err != nil {
		return nil, err
	}
	return page, nil
}

// setLLMInstructions types the portal name and description, waits for the preview to use them,
// and saves.
func (u *Steps) setLLMInstructions(ctx context.Context) error {
	view, err := contextString(ctx, keyLLMView)
	if err != nil {
		return err
	}
	page, err := u.openLLMPanel(ctx, view)
	if err != nil {
		return err
	}
	name, description := llmText(view)
	if err := u.expect.Locator(page.Locator("#aiEnabledToggle")).ToBeChecked(); err != nil {
		return err
	}
	if err := page.Locator("#llmsPortalName").Fill(name); err != nil {
		return err
	}
	if err := page.Locator("#llmsPortalDescription").Fill(description); err != nil {
		return err
	}
	preview := page.Locator("#llmsPreviewContent")
	if err := u.expect.Locator(preview).ToContainText(name); err != nil {
		return fmt.Errorf("the preview does not use the portal name: %w", err)
	}
	if err := u.expect.Locator(preview).ToContainText(description); err != nil {
		return fmt.Errorf("the preview does not use the description: %w", err)
	}
	if err := page.Locator("#saveLlmsConfigBtn").Click(); err != nil {
		return fmt.Errorf("saving the LLM instructions: %w", err)
	}
	return u.expectToast(page, "LLM Instructions saved successfully")
}

// llmsTxtPath is the public llms.txt of a view.
func llmsTxtPath(view string) string {
	return "/api-portal/default/views/" + view + "/llms.txt"
}

// llmsTxtUsesInstructions reads the view's public llms.txt without a session.
func (u *Steps) llmsTxtUsesInstructions(ctx context.Context) error {
	view, err := contextString(ctx, keyLLMView)
	if err != nil {
		return err
	}
	name, description := llmText(view)
	return retry.Await(ctx, retry.Options{}, func(ctx context.Context) (bool, error) {
		status, body, err := u.publicGet(ctx, llmsTxtPath(view))
		if err != nil {
			return false, retry.Transient(err)
		}
		return status == http.StatusOK && strings.Contains(body, name) && strings.Contains(body, description), nil
	}, func(ok bool) bool { return ok }, "waiting for llms.txt to use the saved instructions")
}

// publicGet reads a portal path anonymously, without the browser's session.
func (u *Steps) publicGet(ctx context.Context, path string) (int, string, error) {
	base, err := u.topo.URL("api-portal", "https")
	if err != nil {
		return 0, "", err
	}
	resp, err := u.client.Do(ctx, httpx.Request{Method: http.MethodGet, URL: base + path}, 0, 0)
	if err != nil {
		return 0, "", fmt.Errorf("GET %s: %w", path, err)
	}
	return resp.StatusCode, string(resp.Body), nil
}

// turnAIDiscoveryOff switches AI discovery off for the journey's view and saves.
func (u *Steps) turnAIDiscoveryOff(ctx context.Context) error {
	view, err := contextString(ctx, keyLLMView)
	if err != nil {
		return err
	}
	page, err := u.openLLMPanel(ctx, view)
	if err != nil {
		return err
	}
	name, _ := llmText(view)
	if err := u.expect.Locator(page.Locator("#llmsPortalName")).ToHaveValue(name); err != nil {
		return fmt.Errorf("the saved portal name was not reloaded: %w", err)
	}
	if err := page.Locator("#aiEnabledToggle").Uncheck(); err != nil {
		return fmt.Errorf("turning AI discovery off: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#llmsPreviewContent")).ToContainText("llms.txt is disabled"); err != nil {
		return err
	}
	if err := page.Locator("#saveLlmsConfigBtn").Click(); err != nil {
		return fmt.Errorf("saving the LLM instructions: %w", err)
	}
	return u.expectToast(page, "LLM Instructions saved successfully")
}

// aiDiscoveryIsOff checks the view with AI discovery off: llms.txt is 404 and the catalogue offers
// neither AI Ready nor Try with AI, while the default view still offers both.
func (u *Steps) aiDiscoveryIsOff(ctx context.Context) error {
	view, err := contextString(ctx, keyLLMView)
	if err != nil {
		return err
	}
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	if err := retry.Await(ctx, retry.Options{}, func(ctx context.Context) (bool, error) {
		status, _, err := u.publicGet(ctx, llmsTxtPath(view))
		if err != nil {
			return false, retry.Transient(err)
		}
		return status == http.StatusNotFound, nil
	}, func(ok bool) bool { return ok }, "waiting for llms.txt to be turned off"); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	base, err := u.apiPortalURL()
	if err != nil {
		return err
	}
	for _, v := range []struct {
		view string
		on   bool
	}{{view, false}, {"default", true}} {
		if err := u.navigate(ctx, base+"/api-portal/default/views/"+v.view+"/apis"); err != nil {
			return err
		}
		card := page.Locator("#apiCard-" + id)
		if err := u.revealOnPage(page, card); err != nil {
			return fmt.Errorf("the %s view does not list the wizard API: %w", v.view, err)
		}
		if err := u.expectCount(card.Locator(".dp-badge--ai"), v.on); err != nil {
			return fmt.Errorf("the %s view's AI Ready badge: %w", v.view, err)
		}
		if err := u.navigate(ctx, base+"/api-portal/default/views/"+v.view+"/api/"+id); err != nil {
			return err
		}
		if err := u.expectCount(page.Locator(".aov-hero-actions .dp-btn--ai"), v.on); err != nil {
			return fmt.Errorf("the %s view's Try with AI button: %w", v.view, err)
		}
	}
	return nil
}
