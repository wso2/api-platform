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

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/suites/ui/steps/uivalue"
	"github.com/wso2/api-platform/tests/framework/testbench/services/specs"
)

func (s *Steps) registerAPIDefinitionSteps(sc *godog.ScenarioContext) {
	sc.Step(`^a REST API "([^"]*)" is imported from spec fixture "([^"]*)" in project "([^"]*)" on the control plane$`,
		s.importedAPIExists)
	sc.Step(`^I store the https URL of spec fixture "([^"]*)" as "([^"]*)"$`, storeSpecFixtureURL)
	sc.Step(`^the user chooses "([^"]*)" in the Management Portal$`, s.choose)
	sc.Step(`^the user enters "([^"]*)" as the "([^"]*)" in the Management Portal$`, s.enter)
	sc.Step(`^the user submits the "([^"]*)" field in the Management Portal$`, s.submitField)
	sc.Step(`^the user confirms with "([^"]*)" in the Management Portal$`, s.confirmDialog)
	sc.Step(`^the user uploads spec fixture "([^"]*)" in the Management Portal$`, s.uploadFixture)
	sc.Step(`^the user uploads a generated spec of (\d+) bytes in the Management Portal$`, s.uploadGenerated)
	sc.Step(`^operation "([A-Z]+) ([^"]*)" of REST API "([^"]*)" has policy "([^"]*)" on the control plane with params:$`,
		s.operationHasPolicy)
	sc.Step(`^the Management Portal shows policy "([^"]*)" on resource "([A-Z]+) ([^"]*)"$`, s.resourceShowsPolicy)
	sc.Step(`^the Management Portal shows no policy on resource "([A-Z]+) ([^"]*)"$`, s.resourceShowsNoPolicy)
}

// operationHasPolicy attaches version v1 of a policy, with the given params, to one operation of
// a REST API: it reads the API and writes it back with the policy on that operation, the way a
// publisher's update does.
func (s *Steps) operationHasPolicy(ctx context.Context, method, path, name, policy string, params *godog.DocString) error {
	resolved, err := uivalue.Expand(ctx, name)
	if err != nil {
		return err
	}
	var values map[string]any
	if err := json.Unmarshal([]byte(params.Content), &values); err != nil {
		return fmt.Errorf("policy %q params are not a JSON object: %w", policy, err)
	}
	bearer, err := s.bearer(ctx, "admin")
	if err != nil {
		return err
	}
	base, err := s.topo.URL("platform-api", "https")
	if err != nil {
		return err
	}
	url := base + "/api/v0.9/rest-apis/" + resolved
	headers := map[string]string{"Authorization": "Bearer " + bearer}
	resp, err := s.client.Do(ctx, httpx.Request{Method: http.MethodGet, URL: url, Headers: headers}, 0, 0)
	if err != nil {
		return fmt.Errorf("reading REST API %q: %w", resolved, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("reading REST API %q: %s", resolved, resp.Describe())
	}
	var api map[string]any
	if err := json.Unmarshal(resp.Body, &api); err != nil {
		return fmt.Errorf("reading REST API %q: %w", resolved, err)
	}
	if err := attachOperationPolicy(api, method, path, map[string]any{"name": policy, "version": "v1", "params": values}); err != nil {
		return fmt.Errorf("REST API %q: %w", resolved, err)
	}
	payload, err := json.Marshal(api)
	if err != nil {
		return err
	}
	resp, err = s.client.Do(ctx, httpx.Request{
		Method: http.MethodPut, URL: url, Headers: headers, ContentType: "application/json", Body: payload,
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("updating REST API %q: %w", resolved, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("updating REST API %q: %s", resolved, resp.Describe())
	}
	return nil
}

// attachOperationPolicy appends policy to the operation of api with the given method and path.
func attachOperationPolicy(api map[string]any, method, path string, policy map[string]any) error {
	operations, _ := api["operations"].([]any)
	for _, raw := range operations {
		operation, _ := raw.(map[string]any)
		request, _ := operation["request"].(map[string]any)
		if request == nil || request["method"] != method || request["path"] != path {
			continue
		}
		policies, _ := request["policies"].([]any)
		request["policies"] = append(policies, policy)
		return nil
	}
	return fmt.Errorf("has no operation %s %s", method, path)
}

// resourceShowsPolicy expands a resource on the Policies page and expects the policy in it.
func (s *Steps) resourceShowsPolicy(ctx context.Context, policy, method, path string) error {
	body, err := s.resourceBody(ctx, method, path)
	if err != nil {
		return err
	}
	if err := s.expect.Locator(body.GetByText(policy, playwright.LocatorGetByTextOptions{Exact: playwright.Bool(true)})).
		ToBeVisible(); err != nil {
		return fmt.Errorf("resource %s %s shows no policy %q: %w", method, path, policy, err)
	}
	return nil
}

// resourceShowsNoPolicy expands a resource on the Policies page and expects no attached policy.
// The drop hint shows whether or not policies are attached, so it only marks the region as
// loaded; each attached policy has its own "Remove policy" button.
func (s *Steps) resourceShowsNoPolicy(ctx context.Context, method, path string) error {
	body, err := s.resourceBody(ctx, method, path)
	if err != nil {
		return err
	}
	if err := s.expect.Locator(body.GetByText("Drag and drop policies here, or use Add Policy.")).ToBeVisible(); err != nil {
		return fmt.Errorf("resource %s %s never showed its policies: %w", method, path, err)
	}
	removes := body.GetByRole("button", playwright.LocatorGetByRoleOptions{Name: "Remove policy", Exact: playwright.Bool(true)})
	if err := s.expect.Locator(removes).ToHaveCount(0); err != nil {
		return fmt.Errorf("resource %s %s has attached policies: %w", method, path, err)
	}
	return nil
}

// resourceBody expands the resource's row on the Policies page and returns the region its
// toggle controls.
func (s *Steps) resourceBody(ctx context.Context, method, path string) (playwright.Locator, error) {
	page, err := s.pageGetter(ctx)
	if err != nil {
		return nil, err
	}
	toggle := page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: "Show details for " + method + " " + path, Exact: playwright.Bool(true),
	})
	// The label turns to "Hide details" once expanded, so the region is read first.
	region, err := toggle.GetAttribute("aria-controls")
	if err != nil || region == "" {
		return nil, fmt.Errorf("resource %s %s names no details region: %v", method, path, err)
	}
	if err := toggle.Click(); err != nil {
		return nil, fmt.Errorf("expanding resource %s %s: %w", method, path, err)
	}
	return page.Locator("[id=\"" + region + "\"]"), nil
}

// importedAPIExists creates a REST API from a fixture through the control plane's import, at the
// context "/<name>" with the testbench backend as its upstream.
func (s *Steps) importedAPIExists(ctx context.Context, name, fixture, project string) error {
	resolved, err := uivalue.Expand(ctx, name)
	if err != nil {
		return err
	}
	resolvedProject, err := uivalue.Expand(ctx, project)
	if err != nil {
		return err
	}
	definition, err := specs.Fixture(fixture)
	if err != nil {
		return err
	}
	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	for _, field := range [][2]string{
		{"id", resolved}, {"displayName", resolved}, {"version", "v1.0"}, {"context", "/" + resolved},
		{"projectId", resolvedProject}, {"upstream", `{"main": {"url": "http://testbench:3000"}}`},
	} {
		if err := form.WriteField(field[0], field[1]); err != nil {
			return err
		}
	}
	part, err := form.CreateFormFile("file", fixture)
	if err != nil {
		return err
	}
	if _, err := part.Write(definition); err != nil {
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
		Method: http.MethodPost, URL: base + "/api/v0.9/rest-apis/import-openapi", Body: body.Bytes(),
		Headers: map[string]string{"Authorization": "Bearer " + bearer, "Content-Type": form.FormDataContentType()},
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("importing REST API %q: %w", resolved, err)
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("importing REST API %q: %s", resolved, resp.Describe())
	}
	return cleanup.Register(ctx, cleanup.Resource{Kind: cleanup.KindAPI, ID: resolved, Actor: s.topo.Admin.Username})
}

// storeSpecFixtureURL stores where platform-api fetches a specs fixture from over HTTPS. The
// browser never fetches it: the console asks the control plane to.
func storeSpecFixtureURL(ctx context.Context, name, key string) error {
	local, ok := tcontext.LocalOf(ctx)
	if !ok || local == nil {
		return fmt.Errorf("cannot store spec fixture URL %q without runner context", key)
	}
	local.Set(key, fmt.Sprintf("https://testbench:%d/specs/%s", specs.TLSPort, name))
	return nil
}

// choose clicks an option the page shows as text, such as an API type card or an approach.
func (s *Steps) choose(ctx context.Context, text string) error {
	resolved, err := uivalue.Expand(ctx, text)
	if err != nil {
		return err
	}
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByText(resolved, playwright.PageGetByTextOptions{Exact: playwright.Bool(true)}).First().Click(); err != nil {
		return fmt.Errorf("choosing %q: %w", resolved, err)
	}
	return nil
}

// enter replaces the value of the field labelled label.
func (s *Steps) enter(ctx context.Context, value, label string) error {
	resolved, err := uivalue.Expand(ctx, value)
	if err != nil {
		return err
	}
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByLabel(fieldLabel(label)).Fill(resolved); err != nil {
		return fmt.Errorf("entering %q as the %q: %w", resolved, label, err)
	}
	return nil
}

// submitField presses Enter in a field, which submits the form it belongs to: the console
// reads a contract URL only when its form is submitted.
func (s *Steps) submitField(ctx context.Context, label string) error {
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByLabel(fieldLabel(label)).Press("Enter"); err != nil {
		return fmt.Errorf("submitting the %q field: %w", label, err)
	}
	return nil
}

// confirmDialog presses a button of the open dialog, where the page behind it may show a
// button with the same name.
func (s *Steps) confirmDialog(ctx context.Context, button string) error {
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	dialog := page.GetByRole("dialog")
	if err := dialog.GetByRole("button", playwright.LocatorGetByRoleOptions{Name: button, Exact: playwright.Bool(true)}).Click(); err != nil {
		return fmt.Errorf("confirming with %q: %w", button, err)
	}
	if err := s.expect.Locator(dialog).ToHaveCount(0); err != nil {
		return fmt.Errorf("the dialog stayed open after %q: %w", button, err)
	}
	return nil
}

// fieldLabel matches a field's accessible name, which for a required field ends in an asterisk.
func fieldLabel(label string) *regexp.Regexp {
	return regexp.MustCompile(`^\s*` + regexp.QuoteMeta(label) + `\s*\*?\s*$`)
}

func (s *Steps) uploadFixture(ctx context.Context, name string) error {
	definition, err := specs.Fixture(name)
	if err != nil {
		return err
	}
	return s.upload(ctx, name, definition)
}

func (s *Steps) uploadGenerated(ctx context.Context, size int) error {
	definition, err := specs.Generate(size)
	if err != nil {
		return err
	}
	return s.upload(ctx, "generated.yaml", definition)
}

// upload gives the page's file input a file, as choosing it in the file picker does. The last
// input is the one an open dialog renders over the page.
func (s *Steps) upload(ctx context.Context, name string, content []byte) error {
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	input := page.Locator("input[type=file]").Last()
	if err := input.SetInputFiles([]playwright.InputFile{{Name: name, MimeType: "application/yaml", Buffer: content}}); err != nil {
		return fmt.Errorf("uploading %q: %w", name, err)
	}
	return nil
}
