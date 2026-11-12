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

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
)

// mcpListingResource is the resource the MCP listing fixtures declare next to their tool.
const mcpListingResource = "greetings"

// registerMCPSteps wires the MCP server journey steps that go beyond the listing.
func (u *Steps) registerMCPSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the first MCP listing server page shows its URL, tools, resources and client configuration, and no prompts$`, u.firstMCPPageSections)
	sc.Step(`^the first MCP listing server documentation opens the MCP Playground$`, u.firstMCPPlayground)
	sc.Step(`^the wizard MCP server page shows its tools, resources and prompts$`, u.wizardMCPPageSections)
	sc.Step(`^the MCP catalogue (marks|no longer marks) the wizard MCP server as deprecated$`, u.mcpCatalogueMarksDeprecated)
	sc.Step(`^the user deletes the wizard MCP server from its row menu$`, u.deleteWizardMCP)
	sc.Step(`^the MCP catalogue no longer lists the wizard MCP server$`, u.mcpCatalogueOmitsWizardMCP)
}

// openMCPPage opens an MCP server's page and waits for its name.
func (u *Steps) openMCPPage(ctx context.Context, id string) (playwright.Page, error) {
	if err := u.openPortalPage(ctx, "/mcp/"+id); err != nil {
		return nil, err
	}
	page, err := u.page(ctx)
	if err != nil {
		return nil, err
	}
	if err := u.expect.Locator(page.Locator(".aov-hero-name")).ToBeVisible(); err != nil {
		return nil, err
	}
	return page, nil
}

// firstMCPPageSections asserts the first listing server's page: its URL with a copy control, the
// Tools section with the tool's input schema, the Resources section, no Prompts section, and the
// client configuration with its copy control and the server URL.
func (u *Steps) firstMCPPageSections(ctx context.Context) error {
	id, err := u.firstMCPListingID(ctx)
	if err != nil {
		return err
	}
	page, err := u.openMCPPage(ctx, id)
	if err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#token_mcp_prod_url")).ToContainText("backend.example.invalid/" + id); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(`#token_mcp_prod_url + .copy-btn`)).ToBeVisible(); err != nil {
		return err
	}
	if err := u.sectionTitles(page, "MCP Server URL", "Tools", "Resources", "MCP Server Configuration"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(".aov-section-title").Filter(playwright.LocatorFilterOptions{HasText: regexp.MustCompile("^Prompts$")})).ToHaveCount(0); err != nil {
		return fmt.Errorf("a server without prompts shows a Prompts section: %w", err)
	}
	tool := page.Locator(".mc-accordion").Filter(playwright.LocatorFilterOptions{HasText: "hello"})
	if err := u.expect.Locator(tool.Locator(".mc-json-block")).ToContainText(`"type": "object"`); err != nil {
		return fmt.Errorf("the tool's input schema is not shown: %w", err)
	}
	resource := page.Locator(".mc-accordion").Filter(playwright.LocatorFilterOptions{HasText: mcpListingResource})
	if err := u.expect.Locator(resource.Locator(".mc-accordion-meta-val").First()).ToHaveText("greetings://all"); err != nil {
		return err
	}
	config := page.Locator("#mcp-cfg-" + id)
	if err := u.expect.Locator(config).ToContainText(`"url": "https://backend.example.invalid/` + id + `"`); err != nil {
		return err
	}
	return u.expect.Locator(page.Locator(".mc-config-copy")).ToBeVisible()
}

// firstMCPPlayground opens the first listing server's documentation and asserts the MCP
// Playground entry and the playground the portal renders for it.
func (u *Steps) firstMCPPlayground(ctx context.Context) error {
	id, err := u.firstMCPListingID(ctx)
	if err != nil {
		return err
	}
	if err := u.openPortalPage(ctx, "/mcp/"+id+"/docs/specification"); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(".adoc-nav .adoc-nav-item--active")).ToContainText("MCP Playground"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(`script[src*="mcp-playground"]`)).ToHaveCount(1); err != nil {
		return err
	}
	return u.expect.Locator(page.Locator("#root")).ToBeAttached()
}

// wizardMCPPageSections opens the wizard MCP server's page and asserts the Tools, Resources and
// Prompts sections its schema declares.
func (u *Steps) wizardMCPPageSections(ctx context.Context) error {
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	page, err := u.openMCPPage(ctx, id)
	if err != nil {
		return err
	}
	if err := u.sectionTitles(page, "Tools", "Resources", "Prompts"); err != nil {
		return err
	}
	for _, name := range []string{"ping", "status", "summarize"} {
		if err := u.expect.Locator(page.Locator(".mc-accordion-head-label").Filter(playwright.LocatorFilterOptions{HasText: name})).ToHaveCount(1); err != nil {
			return fmt.Errorf("the wizard MCP server page does not list %q: %w", name, err)
		}
	}
	return nil
}

// mcpCatalogueMarksDeprecated checks the Deprecated badge on the wizard MCP server's card.
func (u *Steps) mcpCatalogueMarksDeprecated(ctx context.Context, marks string) error {
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	if err := u.waitForPortalCards(ctx, "/mcps", id); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expectCount(page.Locator("#apiCard-"+id+" .dp-badge--deprecated"), marks == "marks")
}

// deleteWizardMCP deletes the wizard MCP server from its settings row menu.
func (u *Steps) deleteWizardMCP(ctx context.Context) error {
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	page, err := u.openWizardPanel(ctx, wizardPanelMCPs)
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
	if err := row.Locator(".cfg-delete-trigger").Click(); err != nil {
		return fmt.Errorf("choosing Delete: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#cfg-del-api-name-txt")).ToHaveText("IT Wizard " + id); err != nil {
		return err
	}
	if err := page.Locator("#cfg-del-api-confirm").Click(); err != nil {
		return fmt.Errorf("confirming Delete: %w", err)
	}
	if err := u.expect.Locator(wizardRow(page, id)).ToHaveCount(0); err != nil {
		return err
	}
	status, _, err := u.portalCall(page, http.MethodGet, "/mcp-servers/"+id, nil)
	if err != nil {
		return err
	}
	if status != http.StatusNotFound {
		return fmt.Errorf("GET /mcp-servers/%s = %d after the delete, want 404", id, status)
	}
	registry, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	registry.Deregister(KindAPIPortalMCP, id)
	return nil
}

// mcpCatalogueOmitsWizardMCP checks that the deleted server has no card while the listing still
// shows the fixture servers.
func (u *Steps) mcpCatalogueOmitsWizardMCP(ctx context.Context) error {
	id, err := u.wizardItem(ctx)
	if err != nil {
		return err
	}
	first, err := u.firstMCPListingID(ctx)
	if err != nil {
		return err
	}
	if err := u.waitForPortalCards(ctx, "/mcps", first); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Locator(page.Locator("#apiCard-" + id)).ToHaveCount(0)
}
