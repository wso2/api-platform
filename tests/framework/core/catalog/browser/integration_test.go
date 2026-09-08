//go:build integration

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

package browser

import (
	"context"
	"fmt"
	"testing"
	"time"

	playwright "github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/runtime"
)

// TestBrowserComponentServesPlaywright proves the whole browser stack before any UI feature
// exists: the pinned image boots under the launcher, run-server answers on the ws endpoint,
// and playwright-go — through its local driver — connects, opens a page and reads a title.
func TestBrowserComponentServesPlaywright(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Driver only — the browsers live in the container; installing them here too would
	// defeat the point of the in-network browser.
	require.NoError(t, playwright.Install(&playwright.RunOptions{SkipInstallBrowsers: true}),
		"installing the playwright driver")

	nw, err := runtime.NewNetwork(ctx, "browser-probe")
	require.NoError(t, err)
	t.Cleanup(func() { _ = nw.Remove(context.Background()) })

	c, err := runtime.Launch(ctx, Browser(), runtime.Options{Network: nw, Replicas: 1})
	require.NoError(t, err, "launching the browser component")
	t.Cleanup(func() { _ = c.Stop(context.Background()) })

	port, err := c.Instance.MappedPort("ws")
	require.NoError(t, err)
	ws := fmt.Sprintf("ws://%s:%d%s", c.Instance.Host(), port, PlaywrightWSPath)

	pw, err := playwright.Run()
	require.NoError(t, err)
	t.Cleanup(func() { _ = pw.Stop() })

	browser, err := pw.Chromium.Connect(ws)
	require.NoError(t, err, "connecting to the remote browser at %s", ws)
	t.Cleanup(func() { _ = browser.Close() })
	require.True(t, browser.IsConnected())

	page, err := browser.NewPage()
	require.NoError(t, err)
	_, err = page.Goto("data:text/html,<title>probe</title><h1>browser component works</h1>")
	require.NoError(t, err)

	title, err := page.Title()
	require.NoError(t, err)
	require.Equal(t, "probe", title)
}
