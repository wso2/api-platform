/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.  You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package managementportal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/suites/ui/steps/uivalue"
)

// KindGraphQLAPI is a GraphQL API the administrator owns.
var KindGraphQLAPI = cleanup.Kind{Name: "management-portal-graphql-api", Order: cleanup.KindAPI.Order}

// deploymentStartedToast is the console's confirmation of a new deployment.
var deploymentStartedToast = regexp.MustCompile(`^Deployment "(.+)" started\.$`)

func (s *Steps) registerDeploySteps(sc *godog.ScenarioContext) {
	sc.Step(`^a GraphQL API "([^"]*)" exists in project "([^"]*)" on the control plane$`, s.graphQLAPIExists)
	sc.Step(`^the user clicks "([^"]*)" on gateway "([^"]*)" in the Management Portal$`, s.clickOnGateway)
	sc.Step(`^the user clicks "([^"]*)" in the Management Portal$`, s.click)
	sc.Step(`^the user selects deployment "([^"]*)" in the Management Portal$`, s.selectDeployment)
	sc.Step(`^the Management Portal confirms a deployment started and stores its name as "([^"]*)"$`, s.deploymentStarted)
	sc.Step(`^the Management Portal confirms (stopping|redeploying|deleting) deployment "([^"]*)"$`, s.confirms)
	sc.Step(`^the user closes the drawer in the Management Portal$`, s.closeDrawer)
	sc.Step(`^the deployment status on gateway "([^"]*)" becomes "([^"]*)"$`, s.deploymentStatusBecomes)
	sc.Step(`^the Management Portal's "([^"]*)" button is (enabled|disabled)$`, s.buttonState)
	sc.Step(`^the Management Portal offers (a|no) "([^"]*)" button$`, s.offersButton)
	sc.Step(`^the gateway answers (\d+) to a "([^"]*)" request to "([^"]*)"$`, s.gatewayAnswers)
	sc.Step(`^the gateway answers (\d+) to a "([^"]*)" request to "([^"]*)" with body:$`, s.gatewayAnswersWithBody)
}

// graphQLAPIExists creates a GraphQL API at the context "/<name>". The control plane takes a
// GraphQL API as multipart metadata with an inline schema.
func (s *Steps) graphQLAPIExists(ctx context.Context, name, project string) error {
	resolved, err := uivalue.Expand(ctx, name)
	if err != nil {
		return err
	}
	resolvedProject, err := uivalue.Expand(ctx, project)
	if err != nil {
		return err
	}
	metadata, err := json.Marshal(map[string]any{
		"id":           resolved,
		"displayName":  resolved,
		"context":      "/" + resolved,
		"version":      "v1",
		"projectId":    resolvedProject,
		"upstream":     map[string]any{"main": map[string]any{"url": "http://testbench:3017/graphql"}},
		"schemaSource": "inline",
		"sdl":          "type Query { hello: String }",
	})
	if err != nil {
		return err
	}
	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	if err := form.WriteField("metadata", string(metadata)); err != nil {
		return err
	}
	if err := form.Close(); err != nil {
		return err
	}
	bearer, err := s.bearer(ctx, "admin")
	if err != nil {
		return err
	}
	base, err := s.topo.URL("platform-api", "https")
	if err != nil {
		return err
	}
	resp, err := s.client.Do(ctx, httpx.Request{
		Method: http.MethodPost, URL: base + "/api/v0.9/graphql-apis", Body: body.Bytes(),
		Headers: map[string]string{"Authorization": "Bearer " + bearer, "Content-Type": form.FormDataContentType()},
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("creating GraphQL API %q: %w", resolved, err)
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("creating GraphQL API %q: %s", resolved, resp.Describe())
	}
	return cleanup.Register(ctx, cleanup.Resource{Kind: KindGraphQLAPI, ID: resolved, Actor: s.topo.Admin.Username})
}

// gatewayCard is the deploy page's card for one gateway.
func (s *Steps) gatewayCard(ctx context.Context, gateway string) (playwright.Locator, error) {
	resolved, err := uivalue.Expand(ctx, gateway)
	if err != nil {
		return nil, err
	}
	page, err := s.pageGetter(ctx)
	if err != nil {
		return nil, err
	}
	return page.Locator(".MuiAccordion-root").Filter(playwright.LocatorFilterOptions{HasText: resolved}), nil
}

func (s *Steps) clickOnGateway(ctx context.Context, button, gateway string) error {
	resolved, err := uivalue.Expand(ctx, button)
	if err != nil {
		return err
	}
	card, err := s.gatewayCard(ctx, gateway)
	if err != nil {
		return err
	}
	if err := card.GetByRole("button", playwright.LocatorGetByRoleOptions{Name: resolved, Exact: playwright.Bool(true)}).Click(); err != nil {
		return fmt.Errorf("clicking %q on gateway %q: %w", resolved, gateway, err)
	}
	return nil
}

// click presses a button anywhere on the page. An open drawer hides the page behind it from
// role queries, so a button of the drawer is the one found.
func (s *Steps) click(ctx context.Context, button string) error {
	locator, err := s.button(ctx, button)
	if err != nil {
		return err
	}
	if err := locator.Click(); err != nil {
		return fmt.Errorf("clicking %q: %w", button, err)
	}
	return nil
}

func (s *Steps) button(ctx context.Context, name string) (playwright.Locator, error) {
	resolved, err := uivalue.Expand(ctx, name)
	if err != nil {
		return nil, err
	}
	page, err := s.pageGetter(ctx)
	if err != nil {
		return nil, err
	}
	return page.GetByRole("button", playwright.PageGetByRoleOptions{Name: resolved, Exact: playwright.Bool(true)}), nil
}

// selectDeployment picks a deployment in the restore drawer. A radio's name is its whole row,
// so the match is anchored on the deployment name that starts it.
func (s *Steps) selectDeployment(ctx context.Context, name string) error {
	resolved, err := uivalue.Expand(ctx, name)
	if err != nil {
		return err
	}
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	row := regexp.MustCompile("^" + regexp.QuoteMeta(resolved) + `(\s|$)`)
	if err := page.GetByRole("radio", playwright.PageGetByRoleOptions{Name: row}).Check(); err != nil {
		return fmt.Errorf("selecting deployment %q: %w", resolved, err)
	}
	return nil
}

func (s *Steps) toast(ctx context.Context) (playwright.Locator, error) {
	page, err := s.pageGetter(ctx)
	if err != nil {
		return nil, err
	}
	return page.GetByRole("alert"), nil
}

func (s *Steps) deploymentStarted(ctx context.Context, key string) error {
	toast, err := s.toast(ctx)
	if err != nil {
		return err
	}
	if err := s.expect.Locator(toast).ToHaveText(deploymentStartedToast); err != nil {
		return fmt.Errorf("the Management Portal never confirmed a deployment: %w", err)
	}
	text, err := toast.TextContent()
	if err != nil {
		return err
	}
	match := deploymentStartedToast.FindStringSubmatch(strings.TrimSpace(text))
	if match == nil {
		return fmt.Errorf("the deployment confirmation %q names no deployment", text)
	}
	return tcontext.Set(ctx, key, match[1])
}

// confirmations are the console's toasts for each deployment action.
var confirmations = map[string]string{
	"stopping":    `Stopping "%s".`,
	"redeploying": `Redeploying "%s".`,
	"deleting":    `Deleted "%s".`,
}

func (s *Steps) confirms(ctx context.Context, action, name string) error {
	resolved, err := uivalue.Expand(ctx, name)
	if err != nil {
		return err
	}
	message := fmt.Sprintf(confirmations[action], resolved)
	toast, err := s.toast(ctx)
	if err != nil {
		return err
	}
	if err := s.expect.Locator(toast).ToHaveText(message); err != nil {
		return fmt.Errorf("the Management Portal never confirmed %q: %w", message, err)
	}
	return nil
}

// closeDrawer dismisses an open drawer the way a keyboard user does; the history drawer has
// no close button.
func (s *Steps) closeDrawer(ctx context.Context) error {
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	if err := page.Keyboard().Press("Escape"); err != nil {
		return fmt.Errorf("closing the drawer: %w", err)
	}
	return s.expect.Locator(page.Locator(".MuiDrawer-root")).ToHaveCount(0)
}

// statusSettleTimeout outlasts the console's deployment poll, which backs off to one fetch
// every 30 seconds after a few actions on the page, plus the gateway's acknowledgement.
const statusSettleTimeout = 60_000

// deploymentStatusBecomes reads the status bar of an expanded gateway card: Active for a
// live deployment, Suspended for an undeployed one.
func (s *Steps) deploymentStatusBecomes(ctx context.Context, gateway, status string) error {
	card, err := s.gatewayCard(ctx, gateway)
	if err != nil {
		return err
	}
	label := card.Locator(`xpath=.//*[normalize-space(text())="Deployment Status"]/following-sibling::*[1]`)
	if err := s.expect.Locator(label).ToHaveText(status, playwright.LocatorAssertionsToHaveTextOptions{
		Timeout: playwright.Float(statusSettleTimeout),
	}); err != nil {
		return fmt.Errorf("the deployment on gateway %q never became %q: %w", gateway, status, err)
	}
	return nil
}

func (s *Steps) buttonState(ctx context.Context, name, state string) error {
	locator, err := s.button(ctx, name)
	if err != nil {
		return err
	}
	if state == "enabled" {
		err = s.expect.Locator(locator).ToBeEnabled()
	} else {
		err = s.expect.Locator(locator).ToBeDisabled()
	}
	if err != nil {
		return fmt.Errorf("the %q button is not %s: %w", name, state, err)
	}
	return nil
}

// offersButton checks a button is there or absent. Features check absence after the page
// shows what it loaded, so absence is not mistaken for a page still loading.
func (s *Steps) offersButton(ctx context.Context, which, name string) error {
	locator, err := s.button(ctx, name)
	if err != nil {
		return err
	}
	if which == "a" {
		err = s.expect.Locator(locator).ToBeVisible()
	} else {
		err = s.expect.Locator(locator).ToHaveCount(0)
	}
	if err != nil {
		return fmt.Errorf("expected %s %q button: %w", which, name, err)
	}
	return nil
}

func (s *Steps) gatewayAnswers(ctx context.Context, status int, method, path string) error {
	return s.awaitGateway(ctx, status, method, path, "")
}

func (s *Steps) gatewayAnswersWithBody(ctx context.Context, status int, method, path string, body *godog.DocString) error {
	return s.awaitGateway(ctx, status, method, path, body.Content)
}

// awaitGateway sends the request to the gateway until it answers status. The gateway applies
// a deploy or an undeploy shortly after the control plane accepts it.
func (s *Steps) awaitGateway(ctx context.Context, status int, method, path, body string) error {
	resolved, err := uivalue.Expand(ctx, path)
	if err != nil {
		return err
	}
	base, err := s.topo.URL("platform-gateway", "http")
	if err != nil {
		return err
	}
	request := httpx.Request{Method: method, URL: base + resolved}
	if body != "" {
		request.ContentType = "application/json"
		request.Body = []byte(body)
	}
	last, err := retry.Until(ctx, retry.Options{}, func(ctx context.Context) (*httpx.Response, error) {
		return s.client.Do(ctx, request, 0, 0)
	}, func(resp *httpx.Response) bool { return resp.StatusCode == status })
	if err != nil {
		described := "no response"
		if last != nil {
			described = last.Describe()
		}
		return fmt.Errorf("the gateway never answered %d to %s %s; last: %s: %w", status, method, resolved, described, err)
	}
	return nil
}
