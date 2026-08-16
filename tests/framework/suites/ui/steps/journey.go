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

package steps

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// cyid targets the app's own test hooks. The wrapping element carries the attribute; the
// real input/textarea lives inside it, hence the descendant selectors in the fill helpers.
func cyid(page playwright.Page, id string) playwright.Locator {
	return page.Locator(`[data-cyid="` + id + `"]`)
}

func fillCyidInput(page playwright.Page, id, value string) error {
	if err := cyid(page, id).Locator("input:visible").Fill(value); err != nil {
		return fmt.Errorf("filling %s: %w", id, err)
	}
	return nil
}

func fillCyidTextarea(page playwright.Page, id, value string) error {
	if err := cyid(page, id).Locator("textarea:visible").Fill(value); err != nil {
		return fmt.Errorf("filling %s: %w", id, err)
	}
	return nil
}

// mockLLMURL is the upstream every provider in this suite points at: the testbench's
// openai-dialect service, resolved on the block's network.
func (u *UI) mockLLMURL() (string, error) {
	inst, err := u.topo.Component("testbench")
	if err != nil {
		return "", err
	}
	base, err := inst.InternalURL("openai")
	if err != nil {
		return "", err
	}
	// The mock serves its canned OpenAI-dialect responses under /openai/v1; provider
	// resources (/chat/completions, ...) are appended to this base by the gateway.
	return base + "/openai/v1", nil
}

// --- projects ---

func (u *UI) createProject(ctx context.Context, name string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := cyid(page, "nav-projects").Click(); err != nil {
		return fmt.Errorf("opening Projects: %w", err)
	}
	// MUI renders these as RouterLink anchors, so by ROLE they are links, not buttons —
	// match by shape + text the way the product's own suite does.
	if err := page.Locator("button, a").Filter(playwright.LocatorFilterOptions{
		HasText: regexp.MustCompile(`Create Project|Add New Project`),
	}).First().Click(); err != nil {
		return fmt.Errorf("starting project creation: %w", err)
	}
	if err := fillCyidInput(page, "project-name-input", name); err != nil {
		return err
	}
	if err := fillCyidTextarea(page, "project-description-input",
		"UI suite project for the provider and proxy journey."); err != nil {
		return err
	}
	if err := cyid(page, "create-project-button").Click(); err != nil {
		return fmt.Errorf("submitting the project: %w", err)
	}
	return nil
}

func (u *UI) seesAmongProjects(ctx context.Context, name string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Locator(page.GetByText(name).First()).ToBeVisible()
}

// --- provider ---

func (u *UI) startAddingProviderFromTemplate(ctx context.Context, templateName string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := cyid(page, "nav-service-provider").Click(); err != nil {
		return fmt.Errorf("opening Service Provider: %w", err)
	}
	if err := cyid(page, "add-new-provider-button").Click(); err != nil {
		return fmt.Errorf("starting provider creation: %w", err)
	}
	if err := page.Locator(`[data-cyid^="provider-template-"]`).Filter(
		playwright.LocatorFilterOptions{HasText: templateName}).First().Click(); err != nil {
		return fmt.Errorf("picking the %q template: %w", templateName, err)
	}

	// Some templates interpose a version-selection screen; the form is next either way.
	form := cyid(page, "provider-name-input")
	versionContinue := cyid(page, "template-version-continue-button")
	if err := form.Or(versionContinue).First().WaitFor(); err != nil {
		return fmt.Errorf("neither the provider form nor the version screen appeared: %w", err)
	}
	if visible, _ := versionContinue.IsVisible(); visible {
		if err := page.Locator(`[data-cyid^="template-version-option-"]`).First().Click(); err != nil {
			return fmt.Errorf("picking a template version: %w", err)
		}
		if err := versionContinue.Click(); err != nil {
			return fmt.Errorf("continuing past the version screen: %w", err)
		}
	}
	return nil
}

func (u *UI) createProvider(ctx context.Context, name string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := fillCyidInput(page, "provider-name-input", name); err != nil {
		return err
	}
	if err := fillCyidTextarea(page, "provider-description-input",
		"UI suite provider from the mock-backed template."); err != nil {
		return err
	}
	upstream, err := u.mockLLMURL()
	if err != nil {
		return err
	}
	if err := fillCyidInput(page, "provider-upstream-url-input", upstream); err != nil {
		return err
	}
	if err := fillCyidInput(page, "provider-api-key-input", "sk-ui-suite-provider-key"); err != nil {
		return err
	}
	if err := cyid(page, "add-provider-button").Click(); err != nil {
		return fmt.Errorf("submitting the provider: %w", err)
	}
	return nil
}

func (u *UI) onProviderOverview(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	// "…/service-provider/<id>" where <id> is a real id, not the transient "new" route.
	// RE2 has no lookahead, so the two facts are asserted separately: the shape first
	// (retrying until the redirect lands), then that the id is not the literal "new".
	if err := u.expect.Page(page).ToHaveURL(regexp.MustCompile(`/service-provider/[^/]+$`)); err != nil {
		return err
	}
	if err := u.expect.Page(page).Not().ToHaveURL(regexp.MustCompile(`/service-provider/new$`)); err != nil {
		return fmt.Errorf("still on the transient create route: %w", err)
	}
	return nil
}

// --- proxy ---

func (u *UI) createProxyInProject(ctx context.Context, proxyName, projectName string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	// The overview renders this CTA in several layout branches and only one carries the
	// cyid; match by text on any button the way the product's own suite does.
	if err := page.Locator("button, a").Filter(playwright.LocatorFilterOptions{
		HasText: regexp.MustCompile(`Create App LLM Proxy`),
	}).First().Click(); err != nil {
		return fmt.Errorf("starting proxy creation: %w", err)
	}
	if err := cyid(page, "proxy-project-select").Click(); err != nil {
		return fmt.Errorf("opening the project selector: %w", err)
	}
	if err := page.GetByRole("option",
		playwright.PageGetByRoleOptions{Name: projectName}).Click(); err != nil {
		return fmt.Errorf("picking project %q: %w", projectName, err)
	}
	if err := cyid(page, "proxy-project-continue-button").Click(); err != nil {
		return fmt.Errorf("continuing to the proxy form: %w", err)
	}
	if err := fillCyidInput(page, "proxy-name-input", proxyName); err != nil {
		return err
	}
	if err := fillCyidTextarea(page, "proxy-description-input",
		"UI suite proxy over the mock-backed provider."); err != nil {
		return err
	}
	// The value the proxy injects on its loopback hop into the provider's own context —
	// it must be a provider key the platform minted, or the loopback is rejected with 401.
	providerKey, err := u.latestAPIKey(ctx)
	if err != nil {
		return err
	}
	if err := fillCyidInput(page, "proxy-api-key-input", providerKey); err != nil {
		return err
	}
	if err := cyid(page, "create-proxy-button").Click(); err != nil {
		return fmt.Errorf("submitting the proxy: %w", err)
	}
	return nil
}

func (u *UI) onProxyOverview(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Page(page).ToHaveURL(regexp.MustCompile(`/proxies/[^/]+$`))
}

func (u *UI) deletesTheProxy(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator(`button[aria-label="Delete proxy"]`).Click(); err != nil {
		return fmt.Errorf("opening the delete dialog: %w", err)
	}
	dialog := page.GetByRole("dialog")
	if err := u.expect.Locator(dialog.GetByText("Delete App LLM Proxy")).ToBeVisible(); err != nil {
		return fmt.Errorf("the delete confirmation never appeared: %w", err)
	}
	if err := dialog.GetByRole("button",
		playwright.LocatorGetByRoleOptions{Name: "Delete"}).Click(); err != nil {
		return fmt.Errorf("confirming the delete: %w", err)
	}
	return nil
}

func (u *UI) backOnProxyList(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Page(page).ToHaveURL(regexp.MustCompile(`/proxies/?$`))
}

// --- generic visibility ---

func (u *UI) seesOnPage(ctx context.Context, text string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Locator(page.GetByText(text).First()).ToBeVisible()
}

func (u *UI) noLongerSees(ctx context.Context, text string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Locator(page.GetByText(text)).ToHaveCount(0)
}

// --- gateway deployment, keys, invocation ---

// Scenario-scoped keys for the credential and invocation result the journey carries
// between steps ("that key", "the completion").
const (
	keyLatestAPIKey = "uiLatestAPIKey"
	keyInvocation   = "uiInvocation"
)

// invocation is what the invoke step observed, for the assertion step.
type invocation struct {
	status int
	body   string
}

// invokeAttempts bounds the user-perspective wait for the deployment and the freshly
// generated key to reach the gateway: the invocation retries until the gateway serves it.
const invokeAttempts = 15

func (u *UI) deploysToGateway(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator("button, a").Filter(playwright.LocatorFilterOptions{
		HasText: regexp.MustCompile(`Deploy to Gateway`),
	}).First().Click(); err != nil {
		return fmt.Errorf("opening the deploy screen: %w", err)
	}
	if err := page.Locator("button").Filter(playwright.LocatorFilterOptions{
		HasText: regexp.MustCompile(`^Deploy$`),
	}).First().Click(); err != nil {
		return fmt.Errorf("clicking Deploy on the gateway card: %w", err)
	}
	return nil
}

func (u *UI) seesDeploymentActive(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	// The same row reads "Deployment Status Failed" on a broken deployment, so the
	// assertion pins the label's own row, not a stray "Active" elsewhere on the page.
	row := page.GetByText("Deployment Status").First().Locator("xpath=..")
	return u.expect.Locator(row.GetByText("Active")).ToBeVisible()
}

func (u *UI) returnsToProviderOverview(ctx context.Context) error {
	return u.clicksBack(ctx, "Back to Service Provider")
}

func (u *UI) returnsToProxyOverview(ctx context.Context) error {
	return u.clicksBack(ctx, "Back to App LLM Proxy")
}

func (u *UI) clicksBack(ctx context.Context, label string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByText(label).First().Click(); err != nil {
		return fmt.Errorf("clicking %q: %w", label, err)
	}
	return nil
}

// generatesAPIKey drives the Generate API Key dialog and keeps the one-time key it
// displays; later steps read it back as "that key".
func (u *UI) generatesAPIKey(ctx context.Context, keyName string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: "Generate API Key"}).Click(); err != nil {
		return fmt.Errorf("opening the key dialog: %w", err)
	}
	dialog := page.GetByRole("dialog")
	if err := dialog.Locator(`input[placeholder="Ex: Production Key"]`).Fill(keyName); err != nil {
		return fmt.Errorf("naming the key: %w", err)
	}
	if err := dialog.GetByRole("button", playwright.LocatorGetByRoleOptions{
		Name: "Generate", Exact: playwright.Bool(true)}).Click(); err != nil {
		return fmt.Errorf("generating the key: %w", err)
	}
	if err := u.expect.Locator(dialog.GetByText("API Key Generated Successfully")).ToBeVisible(); err != nil {
		return fmt.Errorf("the generated-key dialog never appeared: %w", err)
	}
	dialogText, err := dialog.InnerText()
	if err != nil {
		return err
	}
	key := regexp.MustCompile(`[a-f0-9]{48,}`).FindString(dialogText)
	if key == "" {
		return fmt.Errorf("no key in the dialog text")
	}
	if err := dialog.GetByRole("button", playwright.LocatorGetByRoleOptions{
		Name: "Done"}).Click(); err != nil {
		return fmt.Errorf("closing the key dialog: %w", err)
	}
	return tcontext.Set(ctx, keyLatestAPIKey, key)
}

// latestAPIKey is the one-time key the most recent generate step displayed.
func (u *UI) latestAPIKey(ctx context.Context) (string, error) {
	v, ok := tcontext.Get(ctx, keyLatestAPIKey)
	if !ok {
		return "", fmt.Errorf("no API key in scope — no generate step ran")
	}
	key, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("API key in scope has unexpected type %T", v)
	}
	return key, nil
}

// invokesChatCompletions calls the endpoint exactly as the overview presents it: the
// rendered invoke URL plus the X-API-Key header the key dialog named. The request is
// fired from the browser's own network position via its API request context.
func (u *UI) invokesChatCompletions(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	key, err := u.latestAPIKey(ctx)
	if err != nil {
		return err
	}
	invokeURL, err := page.Locator(`input[value^="http"]`).First().InputValue()
	if err != nil {
		return fmt.Errorf("reading the invoke URL: %w", err)
	}
	var inv invocation
	for attempt := 1; attempt <= invokeAttempts; attempt++ {
		resp, err := page.Context().Request().Post(invokeURL+"/chat/completions",
			playwright.APIRequestContextPostOptions{
				Data: map[string]any{
					"model":    "gpt-4o",
					"messages": []map[string]string{{"role": "user", "content": "Hello"}},
				},
				Headers: map[string]string{"X-API-Key": key},
			})
		if err != nil {
			return fmt.Errorf("invoking %s: %w", invokeURL, err)
		}
		body, _ := resp.Body()
		inv = invocation{status: resp.Status(), body: string(body)}
		if inv.status == 200 {
			break
		}
		page.WaitForTimeout(2000)
	}
	return tcontext.Set(ctx, keyInvocation, inv)
}

// completionAnswers asserts the model's answer made it back through the whole chain —
// the user-visible proof the proxy is live on the real gateway.
func (u *UI) completionAnswers(ctx context.Context, text string) error {
	v, ok := tcontext.Get(ctx, keyInvocation)
	if !ok {
		return fmt.Errorf("no invocation in scope — the invoke step did not run")
	}
	inv, ok := v.(invocation)
	if !ok {
		return fmt.Errorf("invocation in scope has unexpected type %T", v)
	}
	if inv.status != 200 {
		return fmt.Errorf("the invocation returned %d: %s", inv.status, inv.body)
	}
	if !strings.Contains(inv.body, text) {
		return fmt.Errorf("the completion %q is not in the response: %s", text, inv.body)
	}
	return nil
}
