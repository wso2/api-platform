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
	"regexp"

	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
)

// unreachableAgentURL is a well-formed address with no agent behind it. The form
// accepts the proxy regardless: the Agent Card fetch is a convenience that seeds
// the transports, and failing it warns rather than blocking creation.
const unreachableAgentURL = "http://agent-proxy-ui-suite.invalid:9000"

// opensAgentProxies switches the current project's view to its Agent Proxies list.
func (u *Steps) opensAgentProxies(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByText("Agent Proxies").First().Click(); err != nil {
		return fmt.Errorf("opening Agent Proxies: %w", err)
	}
	return nil
}

// startsCreatingAgentProxy opens the agent proxy create form from the current project's
// Agent Proxies list.
func (u *Steps) startsCreatingAgentProxy(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator("button, a").Filter(playwright.LocatorFilterOptions{
		HasText: regexp.MustCompile(`Create Agent Proxy`),
	}).First().Click(); err != nil {
		return fmt.Errorf("starting agent proxy creation: %w", err)
	}
	return nil
}

// createsAgentProxyForUnreachableAgent creates an agent proxy whose upstream cannot be
// reached: the form probes the URL for an Agent Card, reports that it could not retrieve
// one, and offers Next anyway. Next only appears once the probe resolves either way.
func (u *Steps) createsAgentProxyForUnreachableAgent(ctx context.Context, name string) error {
	var err error
	name, err = expandUIValue(ctx, name)
	if err != nil {
		return err
	}
	if err := u.startsCreatingAgentProxy(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByPlaceholder("Enter the URL of your A2A agent").Fill(
		unreachableAgentURL); err != nil {
		return fmt.Errorf("typing the agent URL: %w", err)
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: "Fetch Agent Info"}).Click(); err != nil {
		return fmt.Errorf("probing the agent URL: %w", err)
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Next"}).Click(
		playwright.LocatorClickOptions{Timeout: playwright.Float(60000)},
	); err != nil {
		return fmt.Errorf("advancing past the Agent Card probe: %w", err)
	}
	if err := page.Locator(`input[placeholder="Trip Planning Agent"]`).Fill(name); err != nil {
		return fmt.Errorf("typing the agent proxy name: %w", err)
	}
	if err := page.Locator(
		`textarea[placeholder="Plans multi-city trips end to end"]`).Fill(
		"UI suite agent proxy created against an unreachable agent."); err != nil {
		return fmt.Errorf("typing the agent proxy description: %w", err)
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: "Create", Exact: playwright.Bool(true)}).Click(); err != nil {
		return fmt.Errorf("submitting the agent proxy: %w", err)
	}
	// The id is the client-computed slug of the name, which the backend stores verbatim,
	// so cleanup can register it without waiting on the create response. A submission the
	// backend goes on to reject registers a proxy that never existed; the deleter treats
	// that as an already-gone resource rather than an error.
	return cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindAgentProxy, ID: toProviderID(name), Actor: u.topo.Admin.Username,
	})
}

// agentProxyOverviewURL matches "…/agent-proxy/<id>", capturing the id, on a real agent
// proxy's overview page — never the transient "/agent-proxy/create" route.
var agentProxyOverviewURL = regexp.MustCompile(`/agent-proxy/([^/]+)$`)

// onAgentProxyOverview asserts the browser reached a real agent proxy's own overview page,
// not the transient create route. RE2 has no lookahead, so the two facts are asserted
// separately: the shape first (retrying until the redirect lands), then that the id is not
// the literal "create".
func (u *Steps) onAgentProxyOverview(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Page(page).ToHaveURL(agentProxyOverviewURL); err != nil {
		return err
	}
	if err := u.expect.Page(page).Not().ToHaveURL(
		regexp.MustCompile(`/agent-proxy/create$`)); err != nil {
		return fmt.Errorf("still on the transient create route: %w", err)
	}
	return nil
}

// deletesAgentProxy opens the delete confirmation for the named proxy's row on the Agent
// Proxies list and confirms it.
func (u *Steps) deletesAgentProxy(ctx context.Context, name string) error {
	var err error
	name, err = expandUIValue(ctx, name)
	if err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator(
		fmt.Sprintf(`button[aria-label="Delete %s"]`, name)).Click(); err != nil {
		return fmt.Errorf("opening the delete dialog for %q: %w", name, err)
	}
	dialog := page.GetByRole("dialog")
	if err := u.expect.Locator(dialog.GetByText("Delete agent proxy")).ToBeVisible(); err != nil {
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
	reg.Deregister(cleanup.KindAgentProxy, toProviderID(name))
	return nil
}
