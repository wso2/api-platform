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
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/suites/ui/steps/uivalue"
)

func (s *Steps) registerCreateSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the API overview shows (name|version|context|description|endpoint) "([^"]*)"$`, s.overviewShows)
	sc.Step(`^the API overview lists exactly these resources:$`, s.overviewListsResources)
	sc.Step(`^the "([^"]*)" field in the Management Portal holds "([^"]*)"$`, s.fieldHolds)
	sc.Step(`^the "([^"]*)" field in the Management Portal shows the error "([^"]*)"$`, s.fieldShowsError)
	sc.Step(`^project "([^"]*)" has exactly these REST APIs on the control plane: "([^"]*)"$`, s.projectHasRESTAPIs)
}

// overviewShows reads one value where the API overview puts it: the name heading, the
// version, context and description in the header card, or the link on the Endpoints card.
func (s *Steps) overviewShows(ctx context.Context, field, value string) error {
	resolved, err := uivalue.Expand(ctx, value)
	if err != nil {
		return err
	}
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	exact := playwright.Bool(true)
	// The header card is the one holding the "Edit API details" link.
	header := page.Locator(".MuiCard-root").Filter(playwright.LocatorFilterOptions{
		Has: page.GetByRole("link", playwright.PageGetByRoleOptions{Name: "Edit API details", Exact: exact}),
	})
	var target playwright.Locator
	switch field {
	case "name":
		target = header.GetByRole("heading", playwright.LocatorGetByRoleOptions{Name: resolved, Exact: exact})
	case "version", "description":
		target = header.GetByText(resolved, playwright.LocatorGetByTextOptions{Exact: exact})
	case "context":
		context := header.Locator("p:text-is('Context:') + p")
		if err := s.expect.Locator(context).ToHaveText(resolved); err != nil {
			return fmt.Errorf("the API overview never showed context %q: %w", resolved, err)
		}
		return nil
	case "endpoint":
		target = s.card(page, "Endpoints").GetByRole("link", playwright.LocatorGetByRoleOptions{Name: resolved, Exact: exact})
	}
	if err := s.expect.Locator(target).ToBeVisible(); err != nil {
		return fmt.Errorf("the API overview never showed %s %q: %w", field, resolved, err)
	}
	return nil
}

// overviewListsResources compares the operations the overview's Resources panel renders, as
// "METHOD path", with the table's rows, ignoring order.
func (s *Steps) overviewListsResources(ctx context.Context, table *godog.Table) error {
	want, err := resourceRows(table)
	if err != nil {
		return err
	}
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	operations := page.Locator("#operations [data-path]")
	if err := s.expect.Locator(operations.First()).ToBeVisible(); err != nil {
		return fmt.Errorf("the API overview never listed its resources: %w", err)
	}
	if err := s.expect.Locator(operations).ToHaveCount(len(want)); err != nil {
		return fmt.Errorf("the API overview lists a different number of resources than %v: %w", want, err)
	}
	listed, err := operations.EvaluateAll(`spans => spans.map(span =>
		span.closest('button').querySelector('span').textContent.trim() + ' ' + span.dataset.path)`)
	if err != nil {
		return fmt.Errorf("reading the API overview's resources: %w", err)
	}
	var got []string
	for _, item := range listed.([]any) {
		method, path, _ := strings.Cut(fmt.Sprint(item), " ")
		got = append(got, strings.ToUpper(method)+" "+path)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		return fmt.Errorf("the API overview lists %v, want %v", got, want)
	}
	return nil
}

// fieldHolds expects the value of the field labelled label.
func (s *Steps) fieldHolds(ctx context.Context, label, value string) error {
	resolved, err := uivalue.Expand(ctx, value)
	if err != nil {
		return err
	}
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	if err := s.expect.Locator(page.GetByLabel(fieldLabel(label))).ToHaveValue(resolved); err != nil {
		return fmt.Errorf("the %q field never held %q: %w", label, resolved, err)
	}
	return nil
}

// fieldShowsError expects the message in the helper text the field names in aria-describedby,
// so a message elsewhere on the page does not count.
func (s *Steps) fieldShowsError(ctx context.Context, label, message string) error {
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	field := page.GetByLabel(fieldLabel(label))
	describedBy, err := field.GetAttribute("aria-describedby")
	if err != nil || describedBy == "" {
		return fmt.Errorf("the %q field names no helper text: %v", label, err)
	}
	if err := s.expect.Locator(page.Locator("[id=\"" + describedBy + "\"]")).ToHaveText(message); err != nil {
		return fmt.Errorf("the %q field never showed %q: %w", label, message, err)
	}
	return nil
}

// projectHasRESTAPIs lists the project's REST APIs on the control plane and expects exactly the
// comma-separated ids ("" for none), which shows nothing else was created.
func (s *Steps) projectHasRESTAPIs(ctx context.Context, project, ids string) error {
	resolvedProject, err := uivalue.Expand(ctx, project)
	if err != nil {
		return err
	}
	resolvedIDs, err := uivalue.Expand(ctx, ids)
	if err != nil {
		return err
	}
	var want []string
	for _, id := range strings.Split(resolvedIDs, ",") {
		if id = strings.TrimSpace(id); id != "" {
			want = append(want, id)
		}
	}
	slices.Sort(want)
	bearer, err := s.bearer(ctx, "admin")
	if err != nil {
		return err
	}
	base, err := s.topo.URL("platform-api", "https")
	if err != nil {
		return err
	}
	resp, err := s.client.Do(ctx, httpx.Request{
		Method:  http.MethodGet,
		URL:     base + "/api/v0.9/rest-apis?limit=100&projectId=" + url.QueryEscape(resolvedProject),
		Headers: map[string]string{"Authorization": "Bearer " + bearer},
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("listing project %q's REST APIs: %w", resolvedProject, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("listing project %q's REST APIs: %s", resolvedProject, resp.Describe())
	}
	var listed struct {
		List []struct {
			ID string `json:"id"`
		} `json:"list"`
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(resp.Body, &listed); err != nil {
		return fmt.Errorf("listing project %q's REST APIs: %w", resolvedProject, err)
	}
	if listed.Pagination.Total > len(listed.List) {
		return fmt.Errorf("project %q has %d REST APIs, more than one page lists", resolvedProject, listed.Pagination.Total)
	}
	got := []string{}
	for _, api := range listed.List {
		got = append(got, api.ID)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) && len(got)+len(want) > 0 {
		return fmt.Errorf("project %q has REST APIs %v, want %v", resolvedProject, got, want)
	}
	return nil
}

// resourceRows reads a two-column | METHOD | path | table as sorted "METHOD path" entries.
func resourceRows(table *godog.Table) ([]string, error) {
	var rows []string
	for i, row := range table.Rows {
		if len(row.Cells) != 2 {
			return nil, fmt.Errorf("resource row %d has %d cells, want | METHOD | path |", i+1, len(row.Cells))
		}
		rows = append(rows, strings.ToUpper(row.Cells[0].Value)+" "+row.Cells[1].Value)
	}
	slices.Sort(rows)
	return rows, nil
}
