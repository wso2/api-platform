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

package aiworkspace

import (
	"context"
	"fmt"
	"regexp"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// keyProviderIDs maps a generated provider display name to the id the platform gave it, for
// steps that find a provider's card by name.
const keyProviderIDs = "uiProviderIDsByName"

// maskedCredential is how the Connection tab renders a stored credential.
const maskedCredential = "******"

func rememberProviderID(ctx context.Context, name, id string) error {
	ids, _ := providerIDs(ctx)
	if ids == nil {
		ids = map[string]string{}
	}
	ids[name] = id
	return tcontext.Set(ctx, keyProviderIDs, ids)
}

func providerIDs(ctx context.Context) (map[string]string, bool) {
	v, ok := tcontext.Get(ctx, keyProviderIDs)
	ids, _ := v.(map[string]string)
	return ids, ok && ids != nil
}

func providerIDFor(ctx context.Context, name string) (string, error) {
	ids, _ := providerIDs(ctx)
	id, ok := ids[name]
	if !ok {
		return "", fmt.Errorf("provider %q was not created in this scenario", name)
	}
	return id, nil
}

// providerExists creates a provider through platform-api, the way another administrator or
// an automation would, so list behaviour can be checked without driving the form each time.
func (u *Steps) providerExists(ctx context.Context, name string) error {
	name, err := expandUIValue(ctx, name)
	if err != nil {
		return err
	}
	upstream, err := u.mockLLMURL()
	if err != nil {
		return err
	}
	page, base, token, err := u.platformAPI(ctx)
	if err != nil {
		return err
	}
	resp, err := page.Context().Request().Post(base+"/api/v0.9/llm-providers",
		playwright.APIRequestContextPostOptions{
			Headers: map[string]string{"Authorization": "Bearer " + token},
			Data: map[string]any{
				"displayName":   name,
				"version":       "v1.0",
				"template":      "azure-openai",
				"upstream":      map[string]any{"main": map[string]any{"url": upstream}},
				"accessControl": map[string]any{"mode": "allow_all"},
			},
			IgnoreHttpsErrors: playwright.Bool(true),
		})
	if err != nil {
		return fmt.Errorf("creating provider %q: %w", name, err)
	}
	if resp.Status() != 201 {
		body, _ := resp.Text()
		return fmt.Errorf("creating provider %q returned %d: %s", name, resp.Status(), body)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := resp.JSON(&created); err != nil || created.ID == "" {
		return fmt.Errorf("the provider %q create response carried no id: %v", name, err)
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindLLMProvider, ID: created.ID, Actor: u.topo.Admin.Username,
	}); err != nil {
		if delErr := u.deleteProviderID(ctx, created.ID); delErr != nil {
			return fmt.Errorf("registering provider %q cleanup: %w; best-effort deletion also failed: %v", name, err, delErr)
		}
		return fmt.Errorf("registering provider %q cleanup: %w; provider was deleted", name, err)
	}
	return rememberProviderID(ctx, name, created.ID)
}

// moreProvidersExist creates count further providers with generated names, newer than any
// the scenario made before.
func (u *Steps) moreProvidersExist(ctx context.Context, count int) error {
	for i := 1; i <= count; i++ {
		if err := u.providerExists(ctx, fmt.Sprintf("${UNIQUE:filler-provider-%d}", i)); err != nil {
			return err
		}
	}
	return nil
}

// searchesProviderList opens the provider list freshly and types query into its search box.
func (u *Steps) searchesProviderList(ctx context.Context, query string) error {
	query, err := expandUIValue(ctx, query)
	if err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := cyid(page, "nav-service-provider").Click(); err != nil {
		return fmt.Errorf("opening Service Provider: %w", err)
	}
	if _, err := page.Reload(); err != nil {
		return fmt.Errorf("reloading the provider list: %w", err)
	}
	return fillCyidInput(page, "provider-search-input", query)
}

func providerCards(page playwright.Page) playwright.Locator {
	return page.Locator(`[data-cyid^="provider-card-"]`)
}

// providerListShowsOnly expects exactly one card in the searched list, the named provider's.
func (u *Steps) providerListShowsOnly(ctx context.Context, name string) error {
	name, err := expandUIValue(ctx, name)
	if err != nil {
		return err
	}
	id, err := providerIDFor(ctx, name)
	if err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Locator(cyid(page, "provider-card-"+id)).ToBeVisible(); err != nil {
		return fmt.Errorf("provider %q is not shown in the searched list: %w", name, err)
	}
	if err := u.expect.Locator(providerCards(page)).ToHaveCount(1); err != nil {
		return fmt.Errorf("the searched list shows providers other than %q: %w", name, err)
	}
	return nil
}

func (u *Steps) providerListShowsNone(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Locator(providerCards(page)).ToHaveCount(0)
}

// providerListShows expects each named provider's card in the list as it stands.
func (u *Steps) providerListShows(ctx context.Context, table *godog.Table) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	for _, row := range table.Rows {
		name, err := expandUIValue(ctx, row.Cells[0].Value)
		if err != nil {
			return err
		}
		id, err := providerIDFor(ctx, name)
		if err != nil {
			return err
		}
		if err := u.expect.Locator(cyid(page, "provider-card-"+id)).ToBeVisible(); err != nil {
			return fmt.Errorf("provider %q is not shown in the list: %w", name, err)
		}
	}
	return nil
}

func (u *Steps) clearsProviderSearch(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return fillCyidInput(page, "provider-search-input", "")
}

// providerOverviewShows checks the overview header against a table of field and expected
// text. Fields: name, template, version, description, creator.
func (u *Steps) providerOverviewShows(ctx context.Context, table *godog.Table) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	for _, row := range table.Rows {
		field := row.Cells[0].Value
		want, err := expandUIValue(ctx, row.Cells[1].Value)
		if err != nil {
			return err
		}
		var target playwright.Locator
		switch field {
		case "name":
			target = page.GetByRole("heading", playwright.PageGetByRoleOptions{Name: providerListName(want), Exact: playwright.Bool(true)})
		case "template", "version", "description":
			target = page.GetByText(want, playwright.PageGetByTextOptions{Exact: playwright.Bool(true)}).First()
		case "creator":
			target = page.GetByText("Created by: "+want, playwright.PageGetByTextOptions{Exact: playwright.Bool(true)})
		default:
			return fmt.Errorf("unknown provider overview field %q", field)
		}
		if err := u.expect.Locator(target).ToBeVisible(); err != nil {
			return fmt.Errorf("the overview does not show %s %q: %w", field, want, err)
		}
	}
	return nil
}

// providerOverviewShowsIDContext checks the overview's context line against the context the
// form derives from the provider's generated id.
func (u *Steps) providerOverviewShowsIDContext(ctx context.Context) error {
	id, err := latestProviderID(ctx)
	if err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Locator(page.GetByText("Context: /"+id, playwright.PageGetByTextOptions{Exact: playwright.Bool(true)})).ToBeVisible()
}

// connectionTabShowsMockEndpointAndMaskedCredential checks the Connection tab shows the
// endpoint the provider was created with and its credential only in masked form.
func (u *Steps) connectionTabShowsMockEndpointAndMaskedCredential(ctx context.Context) error {
	endpoint, err := u.mockLLMURL()
	if err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	field := page.Locator("label").Filter(playwright.LocatorFilterOptions{HasText: regexp.MustCompile(`^Provider Endpoint`)}).
		Locator("xpath=..").Locator("input")
	if err := u.expect.Locator(field).ToHaveValue(endpoint); err != nil {
		return fmt.Errorf("the Connection tab does not show the endpoint %q: %w", endpoint, err)
	}
	if err := u.expect.Locator(credentialField(page)).ToHaveValue(maskedCredential); err != nil {
		return fmt.Errorf("the Connection tab does not show the credential masked: %w", err)
	}
	return nil
}

// providerOfferedForProxyInProject opens the project's create-proxy form and expects the
// provider among those its provider selector offers.
func (u *Steps) providerOfferedForProxyInProject(ctx context.Context, providerName, projectName string) error {
	providerName, err := expandUIValue(ctx, providerName)
	if err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	// Projects are opened from the organization's project list, so leave any project first.
	if inProject, _ := page.Locator(`button[aria-label="Go to organization level"]`).IsVisible(); inProject {
		if err := u.returnsToOrganizationLevel(ctx); err != nil {
			return err
		}
	}
	if err := u.opensProjectsList(ctx); err != nil {
		return err
	}
	if err := u.opensProject(ctx, projectName); err != nil {
		return err
	}
	if err := cyid(page, "nav-proxies").Click(); err != nil {
		return fmt.Errorf("opening App LLM Proxies: %w", err)
	}
	if err := page.Locator("button, a").Filter(playwright.LocatorFilterOptions{
		HasText: regexp.MustCompile(`Create App LLM Proxy`),
	}).First().Click(); err != nil {
		return fmt.Errorf("starting proxy creation in project %q: %w", projectName, err)
	}
	if err := cyid(page, "proxy-provider-select").Click(); err != nil {
		return fmt.Errorf("opening the provider selector: %w", err)
	}
	option := page.GetByRole("option", playwright.PageGetByRoleOptions{Name: providerName, Exact: playwright.Bool(true)})
	if err := u.expect.Locator(option).ToBeVisible(); err != nil {
		return fmt.Errorf("provider %q is not offered for a proxy in project %q: %w", providerName, projectName, err)
	}
	return page.Keyboard().Press("Escape")
}

// connectionTabShowsEndpointAndHeader checks the Connection tab's endpoint and the header the
// credential is sent in, the two fields a provider template sets.
func (u *Steps) connectionTabShowsEndpointAndHeader(ctx context.Context, endpoint, header string) error {
	if endpoint == "the mock LLM" {
		mock, err := u.mockLLMURL()
		if err != nil {
			return err
		}
		endpoint = mock
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	field := func(label string) playwright.Locator {
		return page.Locator("label").Filter(playwright.LocatorFilterOptions{HasText: regexp.MustCompile(`^` + label)}).
			Locator("xpath=..").Locator("input")
	}
	if err := u.expect.Locator(field("Provider Endpoint")).ToHaveValue(endpoint); err != nil {
		return fmt.Errorf("the Connection tab does not show the endpoint %q: %w", endpoint, err)
	}
	if err := u.expect.Locator(field("Authentication Header")).ToHaveValue(header); err != nil {
		return fmt.Errorf("the Connection tab does not show the authentication header %q: %w", header, err)
	}
	return nil
}

// addProviderFormAsksForEndpoint checks whether the form for the chosen template offers an
// endpoint field: a template with a built-in endpoint does not.
func (u *Steps) addProviderFormAsksForEndpoint(ctx context.Context, asks string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Locator(cyid(page, "provider-name-input")).ToBeVisible(); err != nil {
		return fmt.Errorf("the add-provider form is not shown: %w", err)
	}
	field := cyid(page, "provider-upstream-url-input")
	if asks == "asks" {
		return u.expect.Locator(field).ToBeVisible()
	}
	return u.expect.Locator(field).ToHaveCount(0)
}
