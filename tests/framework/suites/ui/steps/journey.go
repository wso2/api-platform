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

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
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

const keyLatestProjectID = "uiLatestProjectID"

// opensProjectsList navigates to the organization's project list.
func (u *UI) opensProjectsList(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := cyid(page, "nav-projects").Click(); err != nil {
		return fmt.Errorf("opening Projects: %w", err)
	}
	return nil
}

func (u *UI) createProject(ctx context.Context, name string) error {
	if err := u.opensProjectsList(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
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
	// The project's id is backend-generated, not a client-computed slug, so cleanup needs
	// the create response rather than a name-derived guess.
	resp, err := page.ExpectResponse(func(r playwright.Response) bool {
		return strings.Contains(r.URL(), "/projects") && r.Request().Method() == "POST"
	}, func() error {
		return cyid(page, "create-project-button").Click()
	})
	if err != nil {
		return fmt.Errorf("submitting the project: %w", err)
	}
	return u.registerCreatedProject(ctx, resp)
}

// registerCreatedProject records the project a create response describes for cleanup, so
// it is removed even when the scenario never deletes it explicitly.
func (u *UI) registerCreatedProject(ctx context.Context, resp playwright.Response) error {
	if resp.Status() < 200 || resp.Status() >= 300 {
		return nil
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := resp.JSON(&body); err != nil {
		return fmt.Errorf("reading the created project's id: %w", err)
	}
	if body.ID == "" {
		return fmt.Errorf("the project create response carried no id")
	}
	if err := tcontext.Set(ctx, keyLatestProjectID, body.ID); err != nil {
		return err
	}
	return cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindProject, ID: body.ID, Actor: u.topo.Admin.Username,
	})
}

// latestProjectID is the id the most recent create-project response returned.
func (u *UI) latestProjectID(ctx context.Context) (string, error) {
	v, ok := tcontext.Get(ctx, keyLatestProjectID)
	if !ok {
		return "", fmt.Errorf("no project id in scope — no create-project step ran")
	}
	id, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("project id in scope has unexpected type %T", v)
	}
	return id, nil
}

func (u *UI) seesAmongProjects(ctx context.Context, name string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Locator(page.GetByText(name).First()).ToBeVisible()
}

// --- provider ---

// opensProviderFromTemplateCard opens the "Add New Provider" screen and picks the named
// template's card, returning the page positioned on either the provider form or, for a
// template with more than one version, the version-selection screen.
func (u *UI) opensProviderFromTemplateCard(ctx context.Context, templateName string) (playwright.Locator, error) {
	page, err := u.page(ctx)
	if err != nil {
		return nil, err
	}
	if err := cyid(page, "nav-service-provider").Click(); err != nil {
		return nil, fmt.Errorf("opening Service Provider: %w", err)
	}
	if err := cyid(page, "add-new-provider-button").Click(); err != nil {
		return nil, fmt.Errorf("starting provider creation: %w", err)
	}
	// The template picker collapses to its first 8 (alphabetical) entries behind a "See
	// more" toggle once more templates exist than that, which any organization-created
	// template can fall past. Expanding it is a no-op when every template already fits.
	seeMore := page.GetByText("See more", playwright.PageGetByTextOptions{Exact: playwright.Bool(true)})
	_ = seeMore.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(5000)})
	// Matching the display name's own text node, not the card's aggregate text: a template
	// with no bundled logo renders a fallback initials avatar (e.g. "EC") alongside the
	// name, and that initials text is a sibling within the same card container, so a
	// HasText filter against the whole card would need to account for it. An exact match
	// also avoids picking the wrong card when one name is a substring of another's ("OpenAI"
	// inside "Azure OpenAI").
	if err := page.GetByText(templateName, playwright.PageGetByTextOptions{
		Exact: playwright.Bool(true)}).First().Click(); err != nil {
		return nil, fmt.Errorf("picking the %q template: %w", templateName, err)
	}

	// Some templates interpose a version-selection screen; the form is next either way.
	form := cyid(page, "provider-name-input")
	versionContinue := cyid(page, "template-version-continue-button")
	if err := form.Or(versionContinue).First().WaitFor(); err != nil {
		return nil, fmt.Errorf("neither the provider form nor the version screen appeared: %w", err)
	}
	return versionContinue, nil
}

// startAddingProviderFromTemplate opens the provider form from the named template's card,
// picking its first offered version when the template has more than one.
func (u *UI) startAddingProviderFromTemplate(ctx context.Context, templateName string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	versionContinue, err := u.opensProviderFromTemplateCard(ctx, templateName)
	if err != nil {
		return err
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

// startAddingProviderFromTemplateVersion opens the provider form from the named template's
// card, picking a specific offered version.
func (u *UI) startAddingProviderFromTemplateVersion(ctx context.Context, templateName, version string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	versionContinue, err := u.opensProviderFromTemplateCard(ctx, templateName)
	if err != nil {
		return err
	}
	if visible, _ := versionContinue.IsVisible(); visible {
		if err := cyid(page, "template-version-option-"+version).Click(); err != nil {
			return fmt.Errorf("picking template version %q: %w", version, err)
		}
		if err := versionContinue.Click(); err != nil {
			return fmt.Errorf("continuing past the version screen: %w", err)
		}
	}
	return nil
}

func (u *UI) createProvider(ctx context.Context, name string) error {
	upstream, err := u.mockLLMURL()
	if err != nil {
		return err
	}
	return u.submitProviderForm(ctx, name, &upstream)
}

// createProviderFromBuiltInEndpoint is for templates that already bake in their own
// upstream endpoint (e.g. OpenAI's built-in points at the real api.openai.com) — the form
// never renders an upstream URL field for them at all, unlike createProvider's template.
func (u *UI) createProviderFromBuiltInEndpoint(ctx context.Context, name string) error {
	return u.submitProviderForm(ctx, name, nil)
}

// nonAlphanumericRun matches the separators a provider name is split on when deriving its
// id, mirroring the frontend's own slug regex (ProviderTemplateFormFields.tsx's
// toProviderId).
var nonAlphanumericRun = regexp.MustCompile(`[^a-z0-9]+`)

// toProviderID derives a provider's id from its display name: lowercased, trimmed, with
// every run of non-alphanumeric characters collapsed to a single hyphen.
func toProviderID(name string) string {
	return strings.Trim(nonAlphanumericRun.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-"), "-")
}

// providerAutoContext predicts the context value the form fills in from the provider's
// name before the user ever touches it.
func providerAutoContext(name string) string {
	id := toProviderID(name)
	if id == "" {
		return "/"
	}
	return "/" + id
}

// submitProviderForm fills the fields every provider template's form has and submits it.
// upstreamURL is filled only when non-nil; a nil value matches a template whose form has
// no such field to begin with (ProviderTemplateFormFields.tsx renders it conditionally).
func (u *UI) submitProviderForm(ctx context.Context, name string, upstreamURL *string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := fillCyidInput(page, "provider-name-input", name); err != nil {
		return err
	}
	// The context field auto-fills from the name as soon as it's typed — asserted here,
	// before the form is submitted and this page is gone, exactly where the legacy suite
	// checked it too.
	if err := u.expect.Locator(cyid(page, "provider-context-input").Locator("input:visible")).
		ToHaveValue(providerAutoContext(name)); err != nil {
		return fmt.Errorf("the context field never auto-filled from the provider name: %w", err)
	}
	if err := fillCyidTextarea(page, "provider-description-input",
		"UI suite provider from the mock-backed template."); err != nil {
		return err
	}
	if upstreamURL != nil {
		if err := fillCyidInput(page, "provider-upstream-url-input", *upstreamURL); err != nil {
			return err
		}
	}
	if err := fillCyidInput(page, "provider-api-key-input", "sk-ui-suite-provider-key"); err != nil {
		return err
	}
	if err := cyid(page, "add-provider-button").Click(); err != nil {
		return fmt.Errorf("submitting the provider: %w", err)
	}
	// The id is a client-computed slug the backend stores verbatim, so cleanup can register
	// it without waiting on the create response. A submission the backend goes on to reject
	// registers a provider that never existed; the deleter treats that as an already-gone
	// resource rather than an error.
	return cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindLLMProvider, ID: toProviderID(name), Actor: u.topo.Admin.Username,
	})
}

func (u *UI) onProviderOverview(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	// "…/service-provider/<id>" where <id> is a real id, not the transient "create" route.
	// RE2 has no lookahead, so the two facts are asserted separately: the shape first
	// (retrying until the redirect lands), then that the id is not the literal "create".
	if err := u.expect.Page(page).ToHaveURL(regexp.MustCompile(`/service-provider/[^/]+$`)); err != nil {
		return err
	}
	if err := u.expect.Page(page).Not().ToHaveURL(regexp.MustCompile(`/service-provider/create$`)); err != nil {
		return fmt.Errorf("still on the transient create route: %w", err)
	}
	return nil
}

// --- proxy ---

func (u *UI) createProxyInProject(ctx context.Context, proxyName, projectName string) error {
	// The value the proxy injects on its loopback hop into the provider's own context —
	// it must be a provider key the platform minted, or the loopback is rejected with 401.
	providerKey, err := u.latestAPIKey(ctx)
	if err != nil {
		return err
	}
	return u.submitProxyForm(ctx, proxyName, projectName, providerKey)
}

// createProxyInProjectUsingAPIKey is for journeys that never deploy the proxy, so no live
// loopback call is ever made to authenticate — a fixed credential is enough, the same way
// the provider's own upstream credential is never a real one either in this suite. It also
// records the /secrets and /llm-proxies requests the submission makes, for scenarios that
// assert on how the credential was stored.
func (u *UI) createProxyInProjectUsingAPIKey(ctx context.Context, proxyName, projectName, apiKey string) error {
	if err := u.watchSecretAndProviderCalls(ctx); err != nil {
		return err
	}
	return u.submitProxyForm(ctx, proxyName, projectName, apiKey)
}

// createProxyInProjectUsingAPIKeyPlaceholder is createProxyInProjectUsingAPIKey, with the
// credential built as a placeholder referencing an existing secret handle.
func (u *UI) createProxyInProjectUsingAPIKeyPlaceholder(ctx context.Context, proxyName, projectName, handle string) error {
	return u.createProxyInProjectUsingAPIKey(ctx, proxyName, projectName, secretPlaceholder(handle))
}

func (u *UI) submitProxyForm(ctx context.Context, proxyName, projectName, apiKey string) error {
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
	if err := fillCyidInput(page, "proxy-api-key-input", apiKey); err != nil {
		return err
	}
	if err := cyid(page, "create-proxy-button").Click(); err != nil {
		return fmt.Errorf("submitting the proxy: %w", err)
	}
	// Proxies and providers share the same client-side slug algorithm (toProviderID), so a
	// submission the backend rejects registers a proxy that never existed; the deleter
	// treats that as already-gone rather than an error.
	return cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindLLMProxy, ID: toProviderID(proxyName), Actor: u.topo.Admin.Username,
	})
}

func (u *UI) onProxyOverview(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	// "…/proxies/<id>" where <id> is a real id, not the transient "create" route. RE2 has
	// no lookahead, so the two facts are asserted separately: the shape first (retrying
	// until the redirect lands), then that the id is not the literal "create".
	if err := u.expect.Page(page).ToHaveURL(regexp.MustCompile(`/proxies/[^/]+$`)); err != nil {
		return err
	}
	if err := u.expect.Page(page).Not().ToHaveURL(regexp.MustCompile(`/proxies/create$`)); err != nil {
		return fmt.Errorf("still on the transient create route: %w", err)
	}
	return nil
}

// proxyOverviewURL matches "…/proxies/<id>", capturing the id, on a real proxy's overview
// page — never the transient "/proxies/create" route, which onProxyOverview already keeps
// callers off of before they reach a step needing the id.
var proxyOverviewURL = regexp.MustCompile(`/proxies/([^/]+)$`)

func (u *UI) deletesTheProxy(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	match := proxyOverviewURL.FindStringSubmatch(page.URL())
	if match == nil {
		return fmt.Errorf("deleting the proxy: not on a proxy overview page (%s)", page.URL())
	}
	id := match[1]
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
	reg, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	reg.Deregister(cleanup.KindLLMProxy, id)
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

// providerOverviewURL matches "…/service-provider/<id>", capturing the id, mirroring
// proxyOverviewURL for the provider side of the same owner-from-URL lookup.
var providerOverviewURL = regexp.MustCompile(`/service-provider/([^/]+)$`)

// apiKeyOwnerFromURL identifies which collection ("llm-providers" or "llm-proxies") and
// owning resource id an API key belongs to, from whichever overview page it was generated
// on — the two owners have separate, otherwise identically-shaped API key endpoints.
func apiKeyOwnerFromURL(pageURL string) (collection, ownerID string, err error) {
	if match := providerOverviewURL.FindStringSubmatch(pageURL); match != nil {
		return "llm-providers", match[1], nil
	}
	if match := proxyOverviewURL.FindStringSubmatch(pageURL); match != nil {
		return "llm-proxies", match[1], nil
	}
	return "", "", fmt.Errorf("generating an API key: not on a provider or proxy overview page (%s)", pageURL)
}

// generatesAPIKey drives the Generate API Key dialog and keeps the one-time key it
// displays; later steps read it back as "that key".
func (u *UI) generatesAPIKey(ctx context.Context, keyName string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	collection, ownerID, err := apiKeyOwnerFromURL(page.URL())
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
	if err := tcontext.Set(ctx, keyLatestAPIKey, key); err != nil {
		return err
	}
	// The composite id encodes what the deleter needs to reach the right nested endpoint:
	// the owner's own collection and id, plus the key's own client-computed handle.
	return cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindAPIKey, ID: collection + "/" + ownerID + "/" + toProviderID(keyName),
		Actor: u.topo.Admin.Username,
	})
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
	inv, err := retry.Until(ctx, retry.Options{}, func(context.Context) (invocation, error) {
		resp, err := page.Context().Request().Post(invokeURL+"/chat/completions",
			playwright.APIRequestContextPostOptions{
				Data: map[string]any{
					"model":    "gpt-4o",
					"messages": []map[string]string{{"role": "user", "content": "Hello"}},
				},
				Headers: map[string]string{"X-API-Key": key},
			})
		if err != nil {
			return invocation{}, err
		}
		body, _ := resp.Body()
		return invocation{status: resp.Status(), body: string(body)}, nil
	}, func(inv invocation) bool { return inv.status == 200 })
	if err != nil {
		return fmt.Errorf("invoking %s: %w", invokeURL, err)
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
