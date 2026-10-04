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

	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

const (
	keyPage           = "uiPage"
	keyBrowserContext = "uiBrowserContext"
)

func (u *Steps) workspaceURL() (string, error) {
	inst, err := u.topo.Component("ai-workspace")
	if err != nil {
		return "", err
	}
	base, err := inst.InternalURL("https")
	if err != nil {
		return "", err
	}
	return base + "/ai-workspace", nil
}

func (u *Steps) openWorkspace(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	url, err := u.workspaceURL()
	if err != nil {
		return err
	}
	if _, err := page.Goto(url); err != nil {
		return fmt.Errorf("opening %s: %w", url, err)
	}
	return nil
}

func (u *Steps) withoutRuntimeConfiguration(ctx context.Context) error {
	v, ok := tcontext.Get(ctx, keyBrowserContext)
	if !ok {
		return fmt.Errorf("ui: no browser context in scope")
	}
	bctx, ok := v.(playwright.BrowserContext)
	if !ok {
		return fmt.Errorf("ui: browser context has unexpected type %T", v)
	}
	return bctx.Route("**/runtime-config.js", func(route playwright.Route) {
		_ = route.Fulfill(playwright.RouteFulfillOptions{
			Body:        "globalThis.__RUNTIME_CONFIG__ = {};",
			ContentType: playwright.String("application/javascript"),
		})
	})
}

func (u *Steps) runtimeConfigurationFallbackIsActive(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	value, err := page.Evaluate(`globalThis.__RUNTIME_CONFIG__ || null`)
	if err != nil {
		return fmt.Errorf("reading runtime configuration: %w", err)
	}
	if value == nil {
		return fmt.Errorf("runtime configuration fallback was not loaded")
	}
	return nil
}

func (u *Steps) seesSignInForm(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(`input[placeholder="username"]`)).ToBeVisible(); err != nil {
		return fmt.Errorf("the username field never became visible: %w", err)
	}
	return u.expect.Locator(page.Locator(`input[type="password"]`)).ToBeVisible()
}
