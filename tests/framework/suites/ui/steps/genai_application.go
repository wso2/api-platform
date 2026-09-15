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
	applicationNonAlphanumeric = regexp.MustCompile(`[^a-z0-9\s-]`)
	applicationWhitespaceRun   = regexp.MustCompile(`\s+`)
	applicationHyphenRun       = regexp.MustCompile(`-+`)
	applicationOverviewURL     = regexp.MustCompile(`/applications/([^/]+)$`)
)

// toApplicationID derives a GenAI application's id from its display name, mirroring the
// frontend's own buildApplicationHandle: lowercased and trimmed, every character outside
// [a-z0-9\s-] removed outright, whitespace runs collapsed to a single hyphen, repeated
// hyphens collapsed, and edge hyphens trimmed.
func toApplicationID(name string) string {
	id := strings.ToLower(strings.TrimSpace(name))
	id = applicationNonAlphanumeric.ReplaceAllString(id, "")
	id = applicationWhitespaceRun.ReplaceAllString(id, "-")
	id = applicationHyphenRun.ReplaceAllString(id, "-")
	return strings.Trim(id, "-")
}

// opensGenAIApplications switches the current project's view to its GenAI Applications list.
func (u *UI) opensGenAIApplications(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByText("GenAI Applications").First().Click(); err != nil {
		return fmt.Errorf("opening GenAI Applications: %w", err)
	}
	return nil
}

// createsGenAIApplication creates a new GenAI application with the given name from the
// current project's GenAI Applications list.
func (u *UI) createsGenAIApplication(ctx context.Context, name string) error {
	if err := u.opensGenAIApplications(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator("button, a").Filter(playwright.LocatorFilterOptions{
		HasText: regexp.MustCompile(`Add New Application|Create Application`),
	}).First().Click(); err != nil {
		return fmt.Errorf("starting application creation: %w", err)
	}
	if err := page.Locator(`input[placeholder="Documentation Assistant"]`).Fill(name); err != nil {
		return fmt.Errorf("typing the application name: %w", err)
	}
	if err := page.Locator(`textarea[placeholder="Short description of the application."]`).Fill(
		"UI suite GenAI application lifecycle test."); err != nil {
		return fmt.Errorf("typing the application description: %w", err)
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: "Create", Exact: playwright.Bool(true)}).Click(); err != nil {
		return fmt.Errorf("submitting the application: %w", err)
	}
	// The id is a client-computed slug the backend stores verbatim, so cleanup can register
	// it without waiting on the create response. A submission the backend goes on to reject
	// registers an application that never existed; the deleter treats that as already-gone
	// rather than an error.
	return cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindApplication, ID: toApplicationID(name), Actor: u.topo.Admin.Username,
	})
}

// onGenAIApplicationOverview asserts the browser reached a real application's own overview
// page, not the transient create route.
func (u *UI) onGenAIApplicationOverview(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Page(page).ToHaveURL(applicationOverviewURL); err != nil {
		return err
	}
	return u.expect.Page(page).Not().ToHaveURL(regexp.MustCompile(`/applications/create$`))
}

// deletesGenAIApplication opens the delete confirmation for the named application's card on
// the GenAI Applications list and confirms it.
func (u *UI) deletesGenAIApplication(ctx context.Context, name string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	card := page.Locator(".MuiCard-root").Filter(playwright.LocatorFilterOptions{HasText: name})
	if err := card.Locator(fmt.Sprintf(`button[aria-label="Delete %s"]`, name)).Click(); err != nil {
		return fmt.Errorf("opening the delete dialog for %q: %w", name, err)
	}
	dialog := page.GetByRole("dialog")
	if err := u.expect.Locator(dialog.GetByText("Delete application")).ToBeVisible(); err != nil {
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
	reg.Deregister(cleanup.KindApplication, toApplicationID(name))
	return nil
}
