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

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// keySignedInState holds the runner's saved storage state (cookies + localStorage) after
// its first UI login. Runner-scoped: scenarios in a runner are sequential, so one login
// serves them all, and parallel runners each authenticate once on their own.
const keySignedInState = "uiSignedInState"

// signInAsAdministrator drives the real sign-in form with the test overlay's admin
// credentials. This is the step for the scenarios ABOUT logging in; everything else uses
// "the user is signed in", which replays the saved state instead of the form.
func (u *UI) signInAsAdministrator(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}

	if err := page.Locator(`input[placeholder="username"]`).Fill(u.topo.Admin.Username); err != nil {
		return fmt.Errorf("typing the username: %w", err)
	}
	if err := page.Locator(`input[type="password"]`).Fill(u.topo.Admin.Password); err != nil {
		return fmt.Errorf("typing the password: %w", err)
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Sign In"}).Click(); err != nil {
		return fmt.Errorf("clicking Sign In: %w", err)
	}
	return nil
}

// landsOnOrganizationHome asserts what a signed-in user SEES: the organization URL and the
// home content — the same two facts the product's own suite treats as "logged in".
func (u *UI) landsOnOrganizationHome(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Page(page).ToHaveURL(regexp.MustCompile(`/ai-workspace/organizations/[^/]+`)); err != nil {
		return fmt.Errorf("the URL never reached the organization home: %w", err)
	}
	return u.expect.Locator(page.GetByText("Quick Start").First()).ToBeVisible()
}

// isSignedIn boots the scenario into an authenticated session. The runner's FIRST use pays
// for one real UI login and saves the browser state; every later use starts a fresh context
// FROM that state — no login UI, no shared cookies jar, still a real session.
func (u *UI) isSignedIn(ctx context.Context) (context.Context, error) {
	local, ok := tcontext.LocalOf(ctx)
	if !ok {
		return ctx, fmt.Errorf("ui: no runner scope in context")
	}

	if v, found := tcontext.Get(ctx, keySignedInState); found {
		state, _ := v.(*playwright.OptionalStorageState)
		return u.reopenWithState(ctx, state)
	}

	// First scenario in this runner: the real thing, once.
	if err := u.openWorkspace(ctx); err != nil {
		return ctx, err
	}
	if err := u.signInAsAdministrator(ctx); err != nil {
		return ctx, err
	}
	if err := u.landsOnOrganizationHome(ctx); err != nil {
		return ctx, err
	}

	v, ok := tcontext.Get(ctx, keyBrowserContext)
	if !ok {
		return ctx, fmt.Errorf("ui: no browser context in scope")
	}
	bctx := v.(playwright.BrowserContext)
	state, err := bctx.StorageState()
	if err != nil {
		return ctx, fmt.Errorf("ui: saving the signed-in storage state: %w", err)
	}
	local.Set(keySignedInState, storageStateToOptional(state))
	return ctx, nil
}

// reopenWithState swaps the scenario's fresh context for one born signed-in.
func (u *UI) reopenWithState(ctx context.Context, state *playwright.OptionalStorageState) (context.Context, error) {
	u.closeScenarioPage(ctx)

	b, err := u.browserFor()
	if err != nil {
		return ctx, err
	}
	bctx, err := b.NewContext(playwright.BrowserNewContextOptions{
		IgnoreHttpsErrors: playwright.Bool(true),
		StorageState:      state,
	})
	if err != nil {
		return ctx, fmt.Errorf("ui: creating the signed-in context: %w", err)
	}
	page, err := newAppPage(bctx)
	if err != nil {
		_ = bctx.Close()
		return ctx, err
	}
	if err := tcontext.Set(ctx, keyBrowserContext, bctx); err != nil {
		return ctx, err
	}
	if err := tcontext.Set(ctx, keyPage, page); err != nil {
		return ctx, err
	}
	// A signed-in user starts somewhere; the organization home is that somewhere.
	if err := u.openWorkspace(ctx); err != nil {
		return ctx, err
	}
	return ctx, u.landsOnOrganizationHome(ctx)
}

// storageStateToOptional converts the saved state into the shape NewContext accepts.
func storageStateToOptional(s *playwright.StorageState) *playwright.OptionalStorageState {
	if s == nil {
		return nil
	}
	out := &playwright.OptionalStorageState{Origins: s.Origins}
	for _, c := range s.Cookies {
		c := c
		out.Cookies = append(out.Cookies, playwright.OptionalCookie{
			Name: c.Name, Value: c.Value, Domain: &c.Domain, Path: &c.Path,
			Expires: &c.Expires, HttpOnly: &c.HttpOnly, Secure: &c.Secure,
			SameSite: c.SameSite,
		})
	}
	return out
}
