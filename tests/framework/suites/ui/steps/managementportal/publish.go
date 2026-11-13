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

package managementportal

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"slices"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/suites/ui/steps/uivalue"
)

// KindAPIPortal is an API Portal registered with the control plane. It is removed after the
// APIs, which may still be published to it.
var KindAPIPortal = cleanup.Kind{Name: "management-portal-api-portal", Order: cleanup.KindAPI.Order + 5}

func (s *Steps) registerPublishSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the block's API Portal is registered on the control plane as "([^"]*)"$`, s.apiPortalRegistered)
	sc.Step(`^the user clicks "([^"]*)" on portal "([^"]*)" in the Management Portal$`, s.clickOnPortal)
	sc.Step(`^the publication status on portal "([^"]*)" becomes "([^"]*)"$`, s.publicationStatusBecomes)
	sc.Step(`^the user follows "View in Portal" on portal "([^"]*)"$`, s.viewInPortal)
	sc.Step(`^the user follows the "([^"]*)" link in the API Portal$`, s.followPortalLink)
	sc.Step(`^the API Portal's API page lists exactly these resources:$`, s.portalListsResources)
	sc.Step(`^the API Portal's documentation lists "([^"]*)" under "([^"]*)"$`, s.portalListsDocument)
}

// apiPortalRegistered registers the block's API Portal under a handle, with the shared key the
// portal accepts.
func (s *Steps) apiPortalRegistered(ctx context.Context, handle string) error {
	resolved, err := uivalue.Expand(ctx, handle)
	if err != nil {
		return err
	}
	portal, err := s.topo.Component("api-portal")
	if err != nil {
		return err
	}
	url, err := portal.InternalURL("https")
	if err != nil {
		return err
	}
	return s.create(ctx, "admin", "/api-portals", map[string]any{
		"name": resolved, "handle": resolved, "url": url, "sharedKey": shared.APIPortalSharedKey(),
	}, KindAPIPortal)
}

// portalCard is the Publish page's card for one API Portal.
func (s *Steps) portalCard(ctx context.Context, portal string) (playwright.Locator, error) {
	resolved, err := uivalue.Expand(ctx, portal)
	if err != nil {
		return nil, err
	}
	page, err := s.pageGetter(ctx)
	if err != nil {
		return nil, err
	}
	return s.card(page, resolved), nil
}

func (s *Steps) clickOnPortal(ctx context.Context, button, portal string) error {
	card, err := s.portalCard(ctx, portal)
	if err != nil {
		return err
	}
	if err := card.GetByRole("button", playwright.LocatorGetByRoleOptions{Name: button, Exact: playwright.Bool(true)}).Click(); err != nil {
		return fmt.Errorf("clicking %q on portal %q: %w", button, portal, err)
	}
	return nil
}

// publicationStatusBecomes waits for the card's status chip to read status.
func (s *Steps) publicationStatusBecomes(ctx context.Context, portal, status string) error {
	card, err := s.portalCard(ctx, portal)
	if err != nil {
		return err
	}
	chip := card.GetByText(status, playwright.LocatorGetByTextOptions{Exact: playwright.Bool(true)})
	if err := s.expect.Locator(chip).ToBeVisible(playwright.LocatorAssertionsToBeVisibleOptions{Timeout: playwright.Float(60000)}); err != nil {
		return fmt.Errorf("the publication on portal %q never became %q: %w", portal, status, err)
	}
	return nil
}

// viewInPortal opens the address the card's "View in Portal" link points at in this tab; the
// link itself opens a new one.
func (s *Steps) viewInPortal(ctx context.Context, portal string) error {
	card, err := s.portalCard(ctx, portal)
	if err != nil {
		return err
	}
	link := card.GetByRole("link", playwright.LocatorGetByRoleOptions{Name: "View in Portal", Exact: playwright.Bool(true)})
	if err := s.expect.Locator(link).ToBeEnabled(); err != nil {
		return fmt.Errorf("portal %q offers no View in Portal link: %w", portal, err)
	}
	href, err := link.GetAttribute("href")
	if err != nil || href == "" {
		return fmt.Errorf("portal %q's View in Portal link has no address: %v", portal, err)
	}
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	if _, err := page.Goto(href); err != nil {
		return fmt.Errorf("opening %s: %w", href, err)
	}
	return nil
}

// followPortalLink opens the address of an API Portal link, which the portal writes relative.
func (s *Steps) followPortalLink(ctx context.Context, name string) error {
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	// The portal repeats some links in its collapsed side menu, so the visible one is followed.
	link := page.Locator("a:visible").Filter(playwright.LocatorFilterOptions{
		HasText: regexp.MustCompile(`^\s*` + regexp.QuoteMeta(name) + `\s*$`),
	}).First()
	href, err := link.GetAttribute("href")
	if err != nil || href == "" {
		return fmt.Errorf("the API Portal shows no %q link: %v", name, err)
	}
	base, err := url.Parse(page.URL())
	if err != nil {
		return err
	}
	target, err := base.Parse(href)
	if err != nil {
		return err
	}
	if _, err := page.Goto(target.String()); err != nil {
		return fmt.Errorf("opening %s: %w", target, err)
	}
	return nil
}

// portalListsResources compares the API page's resource rows, as "METHOD path", with the table.
func (s *Steps) portalListsResources(ctx context.Context, table *godog.Table) error {
	want, err := resourceRows(table)
	if err != nil {
		return err
	}
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	rows := page.Locator(".aov-resource-row")
	if err := s.expect.Locator(rows).ToHaveCount(len(want)); err != nil {
		return fmt.Errorf("the API Portal lists a different number of resources than %v: %w", want, err)
	}
	listed, err := rows.EvaluateAll(`rows => rows.map(row =>
		row.querySelector('.aov-method-badge').textContent.trim() + ' ' + row.querySelector('.aov-resource-path').textContent.trim())`)
	if err != nil {
		return fmt.Errorf("reading the API Portal's resources: %w", err)
	}
	var got []string
	for _, item := range listed.([]any) {
		got = append(got, fmt.Sprint(item))
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		return fmt.Errorf("the API Portal lists %v, want %v", got, want)
	}
	return nil
}

// portalListsDocument expects an entry in the documentation page's group titled group.
func (s *Steps) portalListsDocument(ctx context.Context, name, group string) error {
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	exact := playwright.Bool(true)
	section := page.Locator(".adoc-nav-group").Filter(playwright.LocatorFilterOptions{
		Has: page.Locator(".adoc-nav-group-title").GetByText(group, playwright.LocatorGetByTextOptions{Exact: exact}),
	})
	entry := section.Locator(".adoc-nav-item").GetByText(name, playwright.LocatorGetByTextOptions{Exact: exact})
	if err := s.expect.Locator(entry).ToBeVisible(); err != nil {
		return fmt.Errorf("the API Portal's documentation never listed %q under %q: %w", name, group, err)
	}
	return nil
}
