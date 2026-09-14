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

	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
)

// opensProject navigates from the organization's project list into the named project's
// own scoped workspace.
func (u *UI) opensProject(ctx context.Context, name string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByText(name).First().Click(); err != nil {
		return fmt.Errorf("opening the project %q: %w", name, err)
	}
	return nil
}

// opensMCPProxies switches the current project's view to its MCP Proxies list.
func (u *UI) opensMCPProxies(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByText("MCP Proxies").First().Click(); err != nil {
		return fmt.Errorf("opening MCP Proxies: %w", err)
	}
	return nil
}

// startsCreatingMCPProxy opens the MCP proxy create form from the current project's MCP
// Proxies list.
func (u *UI) startsCreatingMCPProxy(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator("button, a").Filter(playwright.LocatorFilterOptions{
		HasText: regexp.MustCompile(`Create MCP Proxy`),
	}).First().Click(); err != nil {
		return fmt.Errorf("starting MCP proxy creation: %w", err)
	}
	return nil
}

// createsMCPProxyUsingSampleURL creates an MCP proxy from the form's built-in sample
// endpoint: starts the create flow, probes the sample URL, then names and submits the
// proxy. The probe is a real backend call validating the sample endpoint, and the Next
// button only appears once it resolves.
func (u *UI) createsMCPProxyUsingSampleURL(ctx context.Context, name string) error {
	if err := u.startsCreatingMCPProxy(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: "Try with Sample URL"}).Click(); err != nil {
		return fmt.Errorf("selecting the sample URL: %w", err)
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Next"}).Click(
		playwright.LocatorClickOptions{Timeout: playwright.Float(60000)},
	); err != nil {
		return fmt.Errorf("advancing past the sample URL probe: %w", err)
	}
	if err := page.Locator(`input[placeholder="WSO2 MCP Proxy"]`).Fill(name); err != nil {
		return fmt.Errorf("typing the proxy name: %w", err)
	}
	if err := page.Locator(`textarea[placeholder="Primary MCP Proxy"]`).Fill(
		"UI suite MCP proxy created from the sample endpoint."); err != nil {
		return fmt.Errorf("typing the proxy description: %w", err)
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: "Create", Exact: playwright.Bool(true)}).Click(); err != nil {
		return fmt.Errorf("submitting the MCP proxy: %w", err)
	}
	// The id is a client-computed slug the backend stores verbatim, so cleanup can register
	// it without waiting on the create response. A submission the backend goes on to reject
	// registers a proxy that never existed; the deleter treats that as an already-gone
	// resource rather than an error.
	return cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindMCPServer, ID: toProviderID(name), Actor: u.topo.Admin.Username,
	})
}

// mcpProxyOverviewURL matches "…/mcp-proxy/<id>", capturing the id, on a real MCP proxy's
// overview page — never the transient "/mcp-proxy/create" route.
var mcpProxyOverviewURL = regexp.MustCompile(`/mcp-proxy/([^/]+)$`)

// onMCPProxyOverview asserts the browser reached a real MCP proxy's own overview page, not
// the transient create route. RE2 has no lookahead, so the two facts are asserted
// separately: the shape first (retrying until the redirect lands), then that the id is not
// the literal "create".
func (u *UI) onMCPProxyOverview(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Page(page).ToHaveURL(mcpProxyOverviewURL); err != nil {
		return err
	}
	if err := u.expect.Page(page).Not().ToHaveURL(regexp.MustCompile(`/mcp-proxy/create$`)); err != nil {
		return fmt.Errorf("still on the transient create route: %w", err)
	}
	return nil
}

// deletesMCPProxy opens the delete confirmation for the named proxy's row on the MCP
// Proxies list and confirms it.
func (u *UI) deletesMCPProxy(ctx context.Context, name string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator(fmt.Sprintf(`button[aria-label="Delete %s"]`, name)).Click(); err != nil {
		return fmt.Errorf("opening the delete dialog for %q: %w", name, err)
	}
	dialog := page.GetByRole("dialog")
	if err := u.expect.Locator(dialog.GetByText("Delete external server")).ToBeVisible(); err != nil {
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
	reg.Deregister(cleanup.KindMCPServer, toProviderID(name))
	return nil
}

// returnsToOrganizationLevel leaves the current project's scoped workspace.
func (u *UI) returnsToOrganizationLevel(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator(`button[aria-label="Go to organization level"]`).Click(); err != nil {
		return fmt.Errorf("returning to the organization level: %w", err)
	}
	return nil
}

// deletesProject opens the delete confirmation for the named project's card on the
// project list and confirms it.
func (u *UI) deletesProject(ctx context.Context, name string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	card := page.Locator(".MuiCard-root").Filter(playwright.LocatorFilterOptions{HasText: name})
	if err := card.Locator(`button[aria-label="Delete project"]`).Click(); err != nil {
		return fmt.Errorf("opening the delete dialog for project %q: %w", name, err)
	}
	dialog := page.GetByRole("dialog")
	if err := u.expect.Locator(dialog.GetByText("Delete Project")).ToBeVisible(); err != nil {
		return fmt.Errorf("the delete confirmation never appeared: %w", err)
	}
	if err := dialog.GetByRole("button",
		playwright.LocatorGetByRoleOptions{Name: "Delete"}).Click(); err != nil {
		return fmt.Errorf("confirming the delete: %w", err)
	}
	id, err := u.latestProjectID(ctx)
	if err != nil {
		return err
	}
	reg, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	reg.Deregister(cleanup.KindProject, id)
	return nil
}
