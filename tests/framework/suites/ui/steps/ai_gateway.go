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
)

var (
	gatewayWhitespaceRun     = regexp.MustCompile(`\s+`)
	gatewayNonAlphanumeric   = regexp.MustCompile(`[^a-z0-9-]`)
	gatewayHyphenRun         = regexp.MustCompile(`-+`)
	gatewayOverviewURLPrefix = regexp.MustCompile(`/gateways/view/([^/]+)$`)
)

// toGatewayID derives a gateway's id from its display name, mirroring the frontend's own
// generateGatewayName: lowercased and trimmed, whitespace runs collapsed to a single
// hyphen, every other non-alphanumeric character removed outright (not replaced), repeated
// hyphens collapsed, and the result capped at 64 characters.
func toGatewayID(name string) string {
	id := strings.ToLower(strings.TrimSpace(name))
	id = gatewayWhitespaceRun.ReplaceAllString(id, "-")
	id = gatewayNonAlphanumeric.ReplaceAllString(id, "")
	id = strings.Trim(id, "-")
	id = gatewayHyphenRun.ReplaceAllString(id, "-")
	if len(id) > 64 {
		id = id[:64]
	}
	return strings.TrimRight(id, "-")
}

// opensAIGateways navigates to the organization's AI Gateways list.
func (u *UI) opensAIGateways(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByText("AI Gateways").First().Click(); err != nil {
		return fmt.Errorf("opening AI Gateways: %w", err)
	}
	return nil
}

// createsAIGateway registers a new AI gateway with the given name and endpoint URL.
func (u *UI) createsAIGateway(ctx context.Context, name, url string) error {
	if err := u.opensAIGateways(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator("button, a").Filter(playwright.LocatorFilterOptions{
		HasText: regexp.MustCompile(`Add AI Gateway`),
	}).First().Click(); err != nil {
		return fmt.Errorf("starting gateway creation: %w", err)
	}
	if err := page.Locator(`input[placeholder="Enter gateway name"]`).Fill(name); err != nil {
		return fmt.Errorf("typing the gateway name: %w", err)
	}
	if err := page.Locator(`textarea[placeholder="Enter description"]`).Fill(
		"UI suite AI gateway lifecycle test."); err != nil {
		return fmt.Errorf("typing the gateway description: %w", err)
	}
	if err := page.Locator(`input[placeholder="Enter gateway URL"]`).Fill(url); err != nil {
		return fmt.Errorf("typing the gateway URL: %w", err)
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: "Add Gateway"}).Click(); err != nil {
		return fmt.Errorf("submitting the gateway: %w", err)
	}
	// The id is a client-computed slug the backend stores verbatim, so cleanup can register
	// it without waiting on the create response. A submission the backend goes on to reject
	// registers a gateway that never existed; the deleter treats that as already-gone
	// rather than an error.
	return cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindGateway, ID: toGatewayID(name), Actor: u.topo.Admin.Username,
	})
}

// onAIGatewayOverview asserts the browser reached a real gateway's own overview page.
func (u *UI) onAIGatewayOverview(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Page(page).ToHaveURL(gatewayOverviewURLPrefix)
}

// deletesAIGateway opens the delete confirmation for the named gateway's row on the AI
// Gateways list and confirms it.
func (u *UI) deletesAIGateway(ctx context.Context, name string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator(fmt.Sprintf(`button[aria-label="Delete %s"]`, name)).Click(); err != nil {
		return fmt.Errorf("opening the delete dialog for %q: %w", name, err)
	}
	dialog := page.GetByRole("dialog")
	if err := u.expect.Locator(dialog.GetByText("Delete AI Gateway")).ToBeVisible(); err != nil {
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
	reg.Deregister(cleanup.KindGateway, toGatewayID(name))
	return nil
}
