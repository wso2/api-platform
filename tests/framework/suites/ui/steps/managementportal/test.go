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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/suites/ui/steps/uivalue"
)

func (s *Steps) registerTestSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the user sends "([A-Z]+) ([^"]*)" from the Test page's Console view$`, s.sendFromConsole)
	sc.Step(`^the Test page shows the cURL command and request URL "([^"]*)" for "([A-Z]+) ([^"]*)"$`, s.consoleShowsRequest)
	sc.Step(`^the Test page's response to "([A-Z]+) ([^"]*)" is (\d+) with the backend echoing "([A-Z]+) ([^"]*)" sent (through the proxy|from the browser)$`,
		s.consoleResponseEchoes)
	sc.Step(`^the Test page's response to "([A-Z]+) ([^"]*)" shows a CORS or network error$`, s.consoleShowsFetchError)
	sc.Step(`^the user adds "([^"]*)" with value "([^"]*)" in the cURL builder$`, s.addCurlRow)
	sc.Step(`^the cURL command in the Management Portal contains "([^"]*)"$`, s.curlContains)
	sc.Step(`^running the cURL command in a terminal answers (\d+) with the backend echoing "([A-Z]+) ([^"]*)" and header "([^"]*)" set to "([^"]*)"$`,
		s.runCurl)
}

// echo is the part of the testbench backend's reflected request a test compares.
type echo struct {
	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Headers map[string][]string `json:"headers"`
}

// operation is the Console view's (Swagger UI) block for one operation.
func (s *Steps) operation(ctx context.Context, method, path string) (playwright.Locator, error) {
	page, err := s.pageGetter(ctx)
	if err != nil {
		return nil, err
	}
	return page.Locator(".opblock").
		Filter(playwright.LocatorFilterOptions{Has: page.Locator(".opblock-summary-method:text-is('" + method + "')")}).
		Filter(playwright.LocatorFilterOptions{Has: page.Locator("[data-path=\"" + path + "\"]")}), nil
}

// sendFromConsole opens the operation and presses Execute. A response from an earlier send is
// cleared first, so the checks that follow read this send's response.
func (s *Steps) sendFromConsole(ctx context.Context, method, path string) error {
	op, err := s.operation(ctx, method, path)
	if err != nil {
		return err
	}
	if open, _ := op.GetAttribute("class"); !strings.Contains(open, "is-open") {
		if err := op.Locator(".opblock-summary").First().Click(); err != nil {
			return fmt.Errorf("opening %s %s: %w", method, path, err)
		}
	}
	clear := op.GetByRole("button", playwright.LocatorGetByRoleOptions{Name: "Clear", Exact: playwright.Bool(true)})
	if visible, _ := clear.IsVisible(); visible {
		if err := clear.Click(); err != nil {
			return fmt.Errorf("clearing %s %s's last response: %w", method, path, err)
		}
	}
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	local, ok := tcontext.LocalOf(ctx)
	if !ok || local == nil {
		return fmt.Errorf("sending %s %s needs the runner context", method, path)
	}
	// The browser's own request tells who sends the call: the console's relay, or the browser
	// straight to the gateway.
	execute := op.GetByRole("button", playwright.LocatorGetByRoleOptions{Name: "Execute", Exact: playwright.Bool(true)})
	request, err := page.ExpectRequest(func(r playwright.Request) bool {
		return strings.Contains(r.URL(), relayPath) || strings.HasSuffix(strings.SplitN(r.URL(), "?", 2)[0], path)
	}, func() error { return execute.Click() })
	if err != nil {
		return fmt.Errorf("executing %s %s: %w", method, path, err)
	}
	sender := "from the browser"
	if strings.Contains(request.URL(), relayPath) {
		sender = "through the proxy"
	}
	local.Set(senderKey(method, path), sender)
	return nil
}

// relayPath is the console's endpoint that sends a Test page call on the browser's behalf.
const relayPath = "/gateway-relay/invoke"

func senderKey(method, path string) string { return "console-sender:" + method + " " + path }

func (s *Steps) consoleShowsRequest(ctx context.Context, url, method, path string) error {
	resolved, err := uivalue.Expand(ctx, url)
	if err != nil {
		return err
	}
	op, err := s.operation(ctx, method, path)
	if err != nil {
		return err
	}
	if err := s.expect.Locator(op.Locator(".curl-command")).ToContainText("curl -X '" + method + "'"); err != nil {
		return fmt.Errorf("%s %s shows no cURL command: %w", method, path, err)
	}
	if err := s.expect.Locator(op.Locator(".curl-command")).ToContainText("'" + resolved + "'"); err != nil {
		return fmt.Errorf("%s %s's cURL command does not call %s: %w", method, path, resolved, err)
	}
	if err := s.expect.Locator(op.Locator(".request-url pre")).ToHaveText(resolved); err != nil {
		return fmt.Errorf("%s %s shows another request URL than %s: %w", method, path, resolved, err)
	}
	return nil
}

// consoleResponseEchoes reads the live response: its status, and the body the testbench backend
// reflects, which names the method and path the gateway forwarded. The reflected User-Agent tells
// who sent it: the relay is platform-api's Go client, a direct call is the browser.
func (s *Steps) consoleResponseEchoes(ctx context.Context, method, path string, status int, echoMethod, echoPath, sender string) error {
	op, err := s.operation(ctx, method, path)
	if err != nil {
		return err
	}
	live := op.Locator(".live-responses-table tbody .response")
	if err := s.expect.Locator(live.Locator(".response-col_status")).ToHaveText(fmt.Sprint(status)); err != nil {
		return fmt.Errorf("%s %s never answered %d in the Console view: %w", method, path, status, err)
	}
	body, err := live.Locator(".response-col_description pre.microlight").First().TextContent()
	if err != nil {
		return fmt.Errorf("reading %s %s's response body: %w", method, path, err)
	}
	var got echo
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		return fmt.Errorf("%s %s's response body is not the backend's echo: %q", method, path, body)
	}
	if got.Method != echoMethod || got.Path != echoPath {
		return fmt.Errorf("%s %s reached the backend as %s %s, want %s %s", method, path, got.Method, got.Path, echoMethod, echoPath)
	}
	if recorded, _ := tcontext.Get(ctx, senderKey(method, path)); recorded != sender {
		return fmt.Errorf("%s %s was sent %v, not %s", method, path, recorded, sender)
	}
	agent := strings.Join(got.Headers["User-Agent"], " ")
	if fromBrowser := strings.HasPrefix(agent, "Mozilla/"); fromBrowser != (sender == "from the browser") {
		return fmt.Errorf("%s %s was not sent %s: the backend saw User-Agent %q", method, path, sender, agent)
	}
	return nil
}

// consoleShowsFetchError expects the browser's failed fetch, which Swagger UI reports with CORS
// among the possible reasons.
func (s *Steps) consoleShowsFetchError(ctx context.Context, method, path string) error {
	op, err := s.operation(ctx, method, path)
	if err != nil {
		return err
	}
	live := op.Locator(".live-responses-table tbody .response")
	for _, text := range []string{"Failed to fetch.", "CORS"} {
		if err := s.expect.Locator(live).ToContainText(text); err != nil {
			return fmt.Errorf("%s %s never showed %q: %w", method, path, text, err)
		}
	}
	return nil
}

// addCurlRow fills the empty last row of the open table, which then adds another empty row.
func (s *Steps) addCurlRow(ctx context.Context, key, value string) error {
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	keys := page.GetByLabel("Key", playwright.PageGetByLabelOptions{Exact: playwright.Bool(true)})
	row, err := keys.Count()
	if err != nil || row == 0 {
		return fmt.Errorf("the cURL builder shows no row to fill: %v", err)
	}
	if err := keys.Nth(row - 1).Fill(key); err != nil {
		return fmt.Errorf("entering %q as a key: %w", key, err)
	}
	values := page.GetByLabel("Value", playwright.PageGetByLabelOptions{Exact: playwright.Bool(true)})
	if err := values.Nth(row - 1).Fill(value); err != nil {
		return fmt.Errorf("entering %q as %q's value: %w", value, key, err)
	}
	return nil
}

// command is the cURL command as the panel shows it, one argument per line.
func (s *Steps) command(ctx context.Context) (playwright.Locator, error) {
	page, err := s.pageGetter(ctx)
	if err != nil {
		return nil, err
	}
	return page.Locator("div:has(> div > div > h6:text-is('cURL command')) pre code"), nil
}

func (s *Steps) curlContains(ctx context.Context, text string) error {
	resolved, err := uivalue.Expand(ctx, text)
	if err != nil {
		return err
	}
	code, err := s.command(ctx)
	if err != nil {
		return err
	}
	if err := s.expect.Locator(code).ToContainText(resolved); err != nil {
		return fmt.Errorf("the cURL command never contained %q: %w", resolved, err)
	}
	return nil
}

// runCurl runs the panel's command with curl in the gateway-runtime container, where the
// gateway's address resolves as it does for the user. Only the status is added to the output.
func (s *Steps) runCurl(ctx context.Context, status int, echoMethod, echoPath, header, value string) error {
	code, err := s.command(ctx)
	if err != nil {
		return err
	}
	command, err := code.TextContent()
	if err != nil {
		return fmt.Errorf("reading the cURL command: %w", err)
	}
	stack, service, err := s.topo.ServiceControl("gateway-runtime")
	if err != nil {
		return err
	}
	out, err := stack.Exec(ctx, service, []string{"sh", "-c", command + ` -sS -w '\n%{http_code}'`})
	if err != nil {
		return fmt.Errorf("running %q: %w", command, err)
	}
	out = strings.TrimSpace(out)
	cut := strings.LastIndex(out, "\n")
	if cut < 0 || strings.TrimSpace(out[cut+1:]) != fmt.Sprint(status) {
		return fmt.Errorf("running %q answered %q, want status %d", command, out, status)
	}
	var got echo
	if err := json.Unmarshal([]byte(out[:cut]), &got); err != nil {
		return fmt.Errorf("running %q answered no backend echo: %q", command, out[:cut])
	}
	if got.Method != echoMethod || got.Path != echoPath {
		return fmt.Errorf("running %q reached the backend as %s %s, want %s %s", command, got.Method, got.Path, echoMethod, echoPath)
	}
	if values := got.Headers[header]; len(values) != 1 || values[0] != value {
		return fmt.Errorf("running %q sent header %s %v, want %q", command, header, values, value)
	}
	return nil
}
