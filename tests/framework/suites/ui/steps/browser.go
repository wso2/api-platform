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
	"strings"
	"sync"

	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/catalog/browser"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// Scenario-scoped keys. The page is scenario state for the same reason request headers are
// in the gateway suite: runners are parallel, and anything shared mutable belongs in a scope.
const (
	keyPage           = "uiPage"
	keyBrowserContext = "uiBrowserContext"
)

// driver starts the local playwright driver once per process. The driver is a multiplexer:
// every block's browser connection goes through this one Node subprocess.
var driver = sync.OnceValues(func() (*playwright.Playwright, error) {
	if err := playwright.Install(&playwright.RunOptions{SkipInstallBrowsers: true}); err != nil {
		return nil, fmt.Errorf("ui: installing the playwright driver: %w", err)
	}
	if err := browser.PlaywrightVersionMatchesModule(); err != nil {
		return nil, err
	}
	return playwright.Run()
})

// browsers holds one remote-browser connection per BLOCK — each block runs its own browser
// container, and scenarios isolate through contexts, not browser instances. Connections live
// until the process exits; the containers they point at die at block teardown, which closes
// them from the far side.
var browsers = struct {
	sync.Mutex
	byWS map[string]playwright.Browser
}{byWS: map[string]playwright.Browser{}}

// browserFor connects to this block's browser container, once.
func (u *UI) browserFor() (playwright.Browser, error) {
	base, err := u.topo.URL("browser", "ws")
	if err != nil {
		return nil, err
	}
	ws := "ws" + strings.TrimPrefix(base, "http") + browser.PlaywrightWSPath

	browsers.Lock()
	defer browsers.Unlock()
	if b, ok := browsers.byWS[ws]; ok && b.IsConnected() {
		return b, nil
	}
	pw, err := driver()
	if err != nil {
		return nil, err
	}
	b, err := pw.Chromium.Connect(ws)
	if err != nil {
		return nil, fmt.Errorf("ui: connecting to the block's browser at %s: %w", ws, err)
	}
	browsers.byWS[ws] = b
	return b, nil
}

// openScenarioPage gives the scenario its own isolated context and page.
//
// The portal serves a self-signed certificate; the browser accepts it the way a user
// clicking through the warning does.
func (u *UI) openScenarioPage(ctx context.Context) (context.Context, error) {
	b, err := u.browserFor()
	if err != nil {
		return ctx, err
	}
	bctx, err := b.NewContext(playwright.BrowserNewContextOptions{
		IgnoreHttpsErrors: playwright.Bool(true),
	})
	if err != nil {
		return ctx, fmt.Errorf("ui: creating the scenario's browser context: %w", err)
	}
	// Retain-on-failure tracing: every scenario records, the After hook keeps the trace
	// only for failures. Best-effort — a broken recorder must not fail the scenario.
	_ = bctx.Tracing().Start(playwright.TracingStartOptions{
		Screenshots: playwright.Bool(true),
		Snapshots:   playwright.Bool(true),
	})
	page, err := newAppPage(bctx)
	if err != nil {
		_ = bctx.Close()
		return ctx, fmt.Errorf("ui: opening the scenario's page: %w", err)
	}
	if err := tcontext.Set(ctx, keyBrowserContext, bctx); err != nil {
		return ctx, err
	}
	if err := tcontext.Set(ctx, keyPage, page); err != nil {
		return ctx, err
	}
	return ctx, nil
}

// newAppPage opens a page pre-seeded the way the product's own suite seeds every visit:
// the quickstart intro is marked seen BEFORE app code runs, or its overlay intercepts the
// first click of every scenario that navigates.
func newAppPage(bctx playwright.BrowserContext) (playwright.Page, error) {
	if err := bctx.AddInitScript(playwright.Script{
		Content: playwright.String(`try { localStorage.setItem('qs_intro_seen_v1', '1'); } catch (e) {}`),
	}); err != nil {
		return nil, fmt.Errorf("ui: seeding the intro-seen flag: %w", err)
	}
	return bctx.NewPage()
}

// closeScenarioPage discards the scenario's context — cookies, storage, pages, all of it.
func (u *UI) closeScenarioPage(ctx context.Context) {
	if v, ok := tcontext.Get(ctx, keyBrowserContext); ok {
		if bctx, ok := v.(playwright.BrowserContext); ok {
			_ = bctx.Close()
		}
	}
}

// page is the scenario's page, for steps.
func (u *UI) page(ctx context.Context) (playwright.Page, error) {
	v, ok := tcontext.Get(ctx, keyPage)
	if !ok {
		return nil, fmt.Errorf("ui: no page in scope — the scenario hook did not run")
	}
	p, ok := v.(playwright.Page)
	if !ok {
		return nil, fmt.Errorf("ui: page in scope has unexpected type %T", v)
	}
	return p, nil
}
