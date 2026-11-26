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

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

const (
	keyAPIPortalNavigationStatus = "uiAPIPortalNavigationStatus"

	// mobileViewportWidth is below the portal's 860px drawer breakpoint (side-bar.css).
	mobileViewportWidth  = 390
	mobileViewportHeight = 844
)

// registerShellSteps wires the steps of the portal shell: home, the role-based menu, error pages
// and the mobile layout.
func (u *Steps) registerShellSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the API Portal home shows the hero and the APIs, MCP Servers and API Workflows entry points$`, u.homeShowsEntryPoints)
	sc.Step(`^the API Portal shell shows the (anonymous|developer|administrator) menu$`, u.shellShowsMenu)
	sc.Step(`^the user opens the API Portal page "([^"]*)"$`, u.openPortalPage)
	sc.Step(`^the user opens the page of an API that does not exist$`, u.openMissingAPIPage)
	sc.Step(`^the user opens the API Portal settings$`, u.openPortalSettings)
	sc.Step(`^the administrator opens the API Portal settings from the sidebar$`, u.openSettingsFromSidebar)
	sc.Step(`^the API Portal shows the Page not found page with a link back to the portal$`, u.pageNotFoundIsShown)
	sc.Step(`^the API Portal shows the Access denied page$`, u.accessDeniedIsShown)
	sc.Step(`^the API Portal settings page is shown$`, u.settingsPageIsShown)
	sc.Step(`^the API Portal home, an API page and the settings fit a mobile-sized window with a working sidebar toggle$`, u.mobileLayoutWorks)
	sc.Step(`^the user shows and hides the password on the API Portal login form$`, u.togglePasswordVisibility)
	sc.Step(`^the (administrator|developer) signs in to the API Portal through the local form$`, u.signInThroughLocalFormAs)
}

// homeShowsEntryPoints asserts the default view home: the hero with its APIs and MCP Servers
// calls to action, and the API Workflows entry in the sidebar.
func (u *Steps) homeShowsEntryPoints(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Page(page).ToHaveURL(regexp.MustCompile(`/api-portal/default/views/default/?$`)); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(".hero")).ToBeVisible(); err != nil {
		return err
	}
	for _, target := range []string{"/apis", "/mcps"} {
		cta := page.Locator(fmt.Sprintf(`.hero-ctas a[href$="%s"]`, target))
		if err := u.expect.Locator(cta).ToBeVisible(); err != nil {
			return fmt.Errorf("the home hero has no %s entry point: %w", target, err)
		}
	}
	return u.expect.Locator(page.Locator(`#sidebar #api-workflows`)).ToBeAttached()
}

// shellShowsMenu asserts the header and sidebar a role gets: anonymous users see Log In and no
// Settings, a developer the profile menu without Settings, and an administrator Settings too.
func (u *Steps) shellShowsMenu(ctx context.Context, role string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	for _, selector := range []string{"#home", "#apis", "#mcps", "#api-workflows", "#applications", "#subscriptions", "#keys"} {
		if err := u.expect.Locator(page.Locator("#sidebar " + selector)).ToBeAttached(); err != nil {
			return fmt.Errorf("the %s sidebar is missing %s: %w", role, selector, err)
		}
	}
	settings := page.Locator("#sidebar #admin-settings")
	switch role {
	case "anonymous":
		if err := u.expect.Locator(page.Locator("header .login-btn")).ToBeVisible(); err != nil {
			return err
		}
		if err := u.expect.Locator(page.Locator("header .profile-link")).ToHaveCount(0); err != nil {
			return err
		}
		return u.expect.Locator(settings).ToHaveCount(0)
	case "developer":
		if err := u.expect.Locator(page.Locator("header .profile-link")).ToBeVisible(); err != nil {
			return err
		}
		return u.expect.Locator(settings).ToHaveCount(0)
	default:
		if err := u.expect.Locator(page.Locator("header .profile-link")).ToBeVisible(); err != nil {
			return err
		}
		return u.expect.Locator(settings).ToBeAttached()
	}
}

// openPortalPage opens a path of the default view in the browser and records the response
// status for the error-page assertions.
func (u *Steps) openPortalPage(ctx context.Context, path string) error {
	base, err := u.apiPortalURL()
	if err != nil {
		return err
	}
	return u.navigate(ctx, base+"/api-portal/default/views/default"+path)
}

// openMissingAPIPage opens the page of an API handle no fixture creates.
func (u *Steps) openMissingAPIPage(ctx context.Context) error {
	missing, err := unique.Unique(ctx, "missing-api")
	if err != nil {
		return err
	}
	return u.openPortalPage(ctx, "/api/"+missing)
}

// openPortalSettings opens the organization settings page in the browser.
func (u *Steps) openPortalSettings(ctx context.Context) error {
	base, err := u.apiPortalURL()
	if err != nil {
		return err
	}
	return u.navigate(ctx, base+"/api-portal/default/settings")
}

// openSettingsFromSidebar clicks the Settings entry that only an administrator's sidebar has, and
// stores the status of the page it opens.
func (u *Steps) openSettingsFromSidebar(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	link := page.Locator("#sidebar #admin-settings")
	if err := u.expect.Locator(link).ToBeVisible(); err != nil {
		return fmt.Errorf("the sidebar has no Settings entry: %w", err)
	}
	tcontext.Remove(ctx, keyAPIPortalNavigationStatus)
	response, err := page.ExpectNavigation(func() error { return link.Click() },
		playwright.PageExpectNavigationOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded})
	if err != nil {
		return fmt.Errorf("opening the settings from the sidebar: %w", err)
	}
	if response == nil {
		return fmt.Errorf("the Settings entry opened no document")
	}
	if err := u.expect.Page(page).ToHaveURL(regexp.MustCompile(`/api-portal/default/settings`)); err != nil {
		return err
	}
	return tcontext.Set(ctx, keyAPIPortalNavigationStatus, response.Status())
}

// navigate opens target and stores the document's response status.
func (u *Steps) navigate(ctx context.Context, target string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	tcontext.Remove(ctx, keyAPIPortalNavigationStatus)
	response, err := page.Goto(target, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded})
	if err != nil {
		return fmt.Errorf("opening %s: %w", target, err)
	}
	if response == nil {
		return fmt.Errorf("opening %s returned no document response", target)
	}
	return tcontext.Set(ctx, keyAPIPortalNavigationStatus, response.Status())
}

// navigationStatus returns the status of the last document navigate opened.
func navigationStatus(ctx context.Context) (int, error) {
	value, ok := tcontext.Get(ctx, keyAPIPortalNavigationStatus)
	if !ok {
		return 0, fmt.Errorf("no API Portal page has been opened")
	}
	status, ok := value.(int)
	if !ok {
		return 0, fmt.Errorf("API Portal navigation status has unexpected type %T", value)
	}
	return status, nil
}

// pageNotFoundIsShown asserts the portal's 404 page and follows its link back to the home page.
func (u *Steps) pageNotFoundIsShown(ctx context.Context) error {
	if err := u.errorPageIsShown(ctx, http.StatusNotFound, "Page not found"); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator(".ep-actions a.ep-btn--primary").Click(); err != nil {
		return fmt.Errorf("following the Page not found link home: %w", err)
	}
	return u.expect.Locator(page.Locator(".hero")).ToBeVisible()
}

// accessDeniedIsShown asserts the portal's 403 page and its way back.
func (u *Steps) accessDeniedIsShown(ctx context.Context) error {
	if err := u.errorPageIsShown(ctx, http.StatusForbidden, "Access denied"); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(".cfg-nav")).ToHaveCount(0); err != nil {
		return fmt.Errorf("the Access denied page rendered settings controls: %w", err)
	}
	return u.expect.Locator(page.Locator(".ep-actions .ep-btn--primary")).ToContainText("Go back")
}

// errorPageIsShown asserts the last navigation's status and the error page's title.
func (u *Steps) errorPageIsShown(ctx context.Context, wantStatus int, title string) error {
	status, err := navigationStatus(ctx)
	if err != nil {
		return err
	}
	if status != wantStatus {
		return fmt.Errorf("API Portal page status = %d, want %d", status, wantStatus)
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Locator(page.Locator("h1.ep-title")).ToHaveText(title)
}

// settingsPageIsShown asserts the settings page opened for its owner, the administrator.
func (u *Steps) settingsPageIsShown(ctx context.Context) error {
	status, err := navigationStatus(ctx)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("API Portal settings status = %d, want 200", status)
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("h1.ep-title")).ToHaveCount(0); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("h1.cfg-heading")).ToHaveText("Settings"); err != nil {
		return err
	}
	return u.expect.Locator(page.Locator(`.cfg-nav [data-panel="cfg-organizations"]`)).ToBeVisible()
}

// mobileLayoutWorks narrows the window below the drawer breakpoint, checks that home, an API
// page and the settings fit without horizontal scrolling, and that the sidebar toggle opens and
// closes the drawer. The original window size is restored afterwards.
func (u *Steps) mobileLayoutWorks(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	original := page.ViewportSize()
	if err := page.SetViewportSize(mobileViewportWidth, mobileViewportHeight); err != nil {
		return fmt.Errorf("narrowing the window: %w", err)
	}
	defer func() {
		if original != nil {
			_ = page.SetViewportSize(original.Width, original.Height)
		}
	}()
	apiID, err := contextString(ctx, keyAPIPortalAPI)
	if err != nil {
		return err
	}
	if err := u.openPortalPage(ctx, "/"); err != nil {
		return err
	}
	if err := u.drawerToggles(page); err != nil {
		return err
	}
	if err := u.fitsWindow(page, "home"); err != nil {
		return err
	}
	if err := u.openPortalPage(ctx, "/api/"+apiID); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(".aov-hero-name")).ToBeVisible(); err != nil {
		return err
	}
	if err := u.fitsWindow(page, "the API page"); err != nil {
		return err
	}
	if err := u.openPortalSettings(ctx); err != nil {
		return err
	}
	if err := u.settingsPageIsShown(ctx); err != nil {
		return err
	}
	return u.fitsWindow(page, "the settings")
}

// drawerToggles opens the mobile drawer with the header toggle and closes it from the backdrop.
func (u *Steps) drawerToggles(page playwright.Page) error {
	toggle := page.Locator("#sidebarToggle")
	if err := u.expect.Locator(toggle).ToBeVisible(); err != nil {
		return fmt.Errorf("the mobile sidebar toggle is not shown: %w", err)
	}
	if err := toggle.Click(); err != nil {
		return fmt.Errorf("opening the mobile sidebar: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#sidebar")).ToContainClass("mobile-open"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#sidebar #apis")).ToBeVisible(); err != nil {
		return err
	}
	// The open drawer covers the left of the backdrop, so the click lands on its right edge.
	if err := page.Locator("#sidebarBackdrop").Click(playwright.LocatorClickOptions{
		Position: &playwright.Position{X: mobileViewportWidth - 10, Y: mobileViewportHeight / 2},
	}); err != nil {
		return fmt.Errorf("closing the mobile sidebar: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#sidebar")).Not().ToContainClass("mobile-open"); err != nil {
		return err
	}
	return u.expect.Locator(toggle).ToHaveAttribute("aria-expanded", "false")
}

// fitsWindow fails when the document is wider than the window.
func (u *Steps) fitsWindow(page playwright.Page, what string) error {
	overflow, err := page.Evaluate(`() => document.documentElement.scrollWidth - document.documentElement.clientWidth`)
	if err != nil {
		return fmt.Errorf("measuring %s: %w", what, err)
	}
	width, err := numberOf(overflow)
	if err != nil {
		return err
	}
	if width > 0 {
		return fmt.Errorf("%s scrolls horizontally by %vpx on a mobile-sized window", what, width)
	}
	return nil
}

// numberOf converts a number returned by page.Evaluate.
func numberOf(value any) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("evaluated value has unexpected type %T", value)
	}
}

// togglePasswordVisibility opens the login form, types a password and checks that the toggle
// shows and hides it.
func (u *Steps) togglePasswordVisibility(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	password := page.Locator("#password")
	if err := u.expect.Locator(password).ToBeVisible(); err != nil {
		return fmt.Errorf("the login form is not shown: %w", err)
	}
	if err := password.Fill("visible-check"); err != nil {
		return err
	}
	if err := u.expect.Locator(password).ToHaveAttribute("type", "password"); err != nil {
		return err
	}
	for _, want := range []string{"text", "password"} {
		if err := page.Locator("#ln-pw-toggle").Click(); err != nil {
			return fmt.Errorf("toggling the password visibility: %w", err)
		}
		if err := u.expect.Locator(password).ToHaveAttribute("type", want); err != nil {
			return err
		}
	}
	return password.Fill("")
}

// signInThroughLocalFormAs submits the local login form shown on the page as the role, and
// asserts the default view with the user signed in.
func (u *Steps) signInThroughLocalFormAs(ctx context.Context, role string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	username, password, err := u.credentials(ctx, role)
	if err != nil {
		return err
	}
	if err := page.Locator("#username").Fill(username); err != nil {
		return err
	}
	if err := page.Locator("#password").Fill(password); err != nil {
		return err
	}
	if err := page.Locator(".ln-signin-btn").Click(); err != nil {
		return fmt.Errorf("signing in as %s: %w", role, err)
	}
	if err := u.expect.Page(page).ToHaveURL(regexp.MustCompile(`/views/default`)); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#local-login-form")).ToHaveCount(0); err != nil {
		return err
	}
	return u.expect.Locator(page.Locator(".profile-link")).ToBeVisible()
}

// credentials returns the username and password of the role.
func (u *Steps) credentials(ctx context.Context, role string) (string, string, error) {
	if role != "developer" {
		return u.topo.Admin.Username, u.topo.Admin.Password, nil
	}
	return u.developerCredentials(ctx)
}

// contextString reads a non-empty string stored in the scenario context.
func contextString(ctx context.Context, key string) (string, error) {
	value, ok := tcontext.Get(ctx, key)
	if !ok {
		return "", fmt.Errorf("scenario value %q is unavailable", key)
	}
	text, ok := value.(string)
	if !ok || text == "" {
		return "", fmt.Errorf("scenario value %q has unexpected type %T", key, value)
	}
	return text, nil
}

// revealOnPage pages through the visible paginator until target is shown. The portal pages the
// settings APIs and MCP servers tables and the catalogue grids in the browser, so an item another
// runner pushed past the first page exists in the document but is hidden.
func (u *Steps) revealOnPage(page playwright.Page, target playwright.Locator) error {
	if err := u.expect.Locator(target).ToHaveCount(1); err != nil {
		return err
	}
	next := page.Locator(`.dp-pagination:visible button[aria-label="Next page"]:not([disabled])`)
	for range 100 {
		visible, err := target.IsVisible()
		if err != nil {
			return err
		}
		if visible {
			return nil
		}
		count, err := next.Count()
		if err != nil {
			return err
		}
		if count == 0 {
			return u.expect.Locator(target).ToBeVisible()
		}
		if err := next.First().Click(); err != nil {
			return fmt.Errorf("turning the page: %w", err)
		}
	}
	return u.expect.Locator(target).ToBeVisible()
}
