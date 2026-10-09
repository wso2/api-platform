/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package aiworkspace

import (
	"context"
	"fmt"

	playwright "github.com/mxschmitt/playwright-go"
)

// agentProxyConnectionField locates a Backend Connection tab input by its label, which the
// form renders beside the field rather than bound to it.
func agentProxyConnectionField(page playwright.Page, label string) playwright.Locator {
	return page.Locator("label").Filter(playwright.LocatorFilterOptions{HasText: label}).
		Locator("xpath=..").Locator("input")
}

// policyMapperSection scopes to one policy mapper by its heading, so a page holding several
// of them resolves the right controls.
func policyMapperSection(page playwright.Page, title string) playwright.Locator {
	return page.GetByText(title, playwright.PageGetByTextOptions{
		Exact: playwright.Bool(true)}).Locator("xpath=../../..")
}

// opensAgentProxyTab switches the agent proxy overview to one of its tabs.
func (u *Steps) opensAgentProxyTab(ctx context.Context, tab string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByRole("tab", playwright.PageGetByRoleOptions{
		Name: tab}).Click(); err != nil {
		return fmt.Errorf("opening the %q tab: %w", tab, err)
	}
	return nil
}

// opensPolicyDrawerFor opens the policy picker belonging to the named mapper.
func (u *Steps) opensPolicyDrawerFor(ctx context.Context, title string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := policyMapperSection(page, title).GetByRole("button",
		playwright.LocatorGetByRoleOptions{Name: "Add Policy"}).Click(); err != nil {
		return fmt.Errorf("opening the policy drawer for %q: %w", title, err)
	}
	return nil
}

// policyDrawerIsOpen asserts the policy picker rendered its search field.
func (u *Steps) policyDrawerIsOpen(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Locator(page.GetByPlaceholder("Search policies")).ToBeVisible()
}

// searchesPolicies filters the policy picker to the given term.
func (u *Steps) searchesPolicies(ctx context.Context, term string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByPlaceholder("Search policies").Fill(term); err != nil {
		return fmt.Errorf("searching the policies for %q: %w", term, err)
	}
	return nil
}

// closesPolicyDrawer dismisses the policy picker.
func (u *Steps) closesPolicyDrawer(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: "Close policy drawer"}).Click(); err != nil {
		return fmt.Errorf("closing the policy drawer: %w", err)
	}
	return u.expect.Locator(page.GetByPlaceholder("Search policies")).Not().ToBeVisible()
}

// setsAgentProxyAuthType picks an authentication mode on the Backend Connection tab.
func (u *Steps) setsAgentProxyAuthType(ctx context.Context, authType string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator("label").Filter(playwright.LocatorFilterOptions{
		HasText: "Authentication"}).First().Locator("xpath=..").
		Locator("[role='combobox']").Click(); err != nil {
		return fmt.Errorf("opening the authentication options: %w", err)
	}
	if err := page.GetByRole("option", playwright.PageGetByRoleOptions{
		Name: authType, Exact: playwright.Bool(true)}).Click(); err != nil {
		return fmt.Errorf("selecting the authentication type %q: %w", authType, err)
	}
	return nil
}

// setsAgentProxyCredential fills the header name and credential the selected authentication
// mode requires.
func (u *Steps) setsAgentProxyCredential(ctx context.Context, header, value string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := agentProxyConnectionField(page, "Authentication Header").Fill(header); err != nil {
		return fmt.Errorf("typing the authentication header: %w", err)
	}
	field := agentProxyConnectionField(page, "Credentials")
	if err := field.Click(); err != nil {
		return fmt.Errorf("focusing the credential field: %w", err)
	}
	if err := field.Fill(value); err != nil {
		return fmt.Errorf("typing the credential: %w", err)
	}
	return nil
}

// savesAgentProxyChanges commits the overview's pending edits. One Save button serves
// every tab, so the tab a change was made on does not matter here.
func (u *Steps) savesAgentProxyChanges(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: "Save", Exact: playwright.Bool(true)}).Click(); err != nil {
		return fmt.Errorf("saving the agent proxy: %w", err)
	}
	return nil
}

// opensAgentProxyEditPage follows the overview's edit action.
func (u *Steps) opensAgentProxyEditPage(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByRole("link", playwright.PageGetByRoleOptions{
		Name: "Edit Agent Proxy"}).Click(); err != nil {
		return fmt.Errorf("opening the agent proxy edit page: %w", err)
	}
	return nil
}

// agentCardSection scopes to one of the Agent Card tab's two cards by its heading, so a
// control is read from the intended card rather than whichever renders first.
func agentCardSection(page playwright.Page, heading string) playwright.Locator {
	return page.GetByText(heading, playwright.PageGetByTextOptions{
		Exact: playwright.Bool(true)}).Locator("xpath=../../..")
}

// setsAgentCardMode picks how one of the cards is served.
func (u *Steps) setsAgentCardMode(ctx context.Context, heading, mode string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := agentCardSection(page, heading).GetByRole("radio",
		playwright.LocatorGetByRoleOptions{Name: mode}).Check(); err != nil {
		return fmt.Errorf("setting %q to %q: %w", heading, mode, err)
	}
	return nil
}

// turnsOffCardRewrite clears the URL rewriting switch on one of the cards.
func (u *Steps) turnsOffCardRewrite(ctx context.Context, heading string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := agentCardSection(page, heading).GetByRole("switch",
		playwright.LocatorGetByRoleOptions{}).First().Uncheck(); err != nil {
		return fmt.Errorf("turning off URL rewriting for %q: %w", heading, err)
	}
	return nil
}

// cardIsServed asserts a card's mode is the one selected.
func (u *Steps) cardIsServed(ctx context.Context, heading, mode string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Locator(agentCardSection(page, heading).GetByRole("radio",
		playwright.LocatorGetByRoleOptions{Name: mode})).ToBeChecked()
}
