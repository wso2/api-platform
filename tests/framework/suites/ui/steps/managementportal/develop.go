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
	"encoding/base64"
	"fmt"
	"regexp"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/suites/ui/steps/uivalue"
)

func (s *Steps) registerDevelopSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the user opens resource "([A-Z]+) ([^"]*)" on the Policies page$`, s.openResource)
	sc.Step(`^the Management Portal shows global policy "([^"]*)"$`, s.showsGlobalPolicy)
	sc.Step(`^the user selects "([^"]*)" as the "([^"]*)" in the Management Portal$`, s.selectOption)
	sc.Step(`^the API overview's "([^"]*)" card shows "([^"]*)"$`, s.overviewCardShows)
	sc.Step(`^the documents page lists "([^"]*)" under "([^"]*)"$`, s.documentListedUnder)
	sc.Step(`^the policy catalog's "([^"]*)" entry in the Management Portal can (be|not be) attached$`, s.catalogEntryAttachable)
	sc.Step(`^the gateway answers (\d+) to a "([^"]*)" request to "([^"]*)" as "([^"]*)" with password "([^"]*)"$`,
		s.gatewayAnswersAs)
}

// openResource expands a resource on the Policies page, which makes it the one a policy picked
// from the catalog attaches to.
func (s *Steps) openResource(ctx context.Context, method, path string) error {
	_, err := s.resourceBody(ctx, method, path)
	return err
}

// showsGlobalPolicy expects the policy in the Global policies card, where API-level policies sit.
func (s *Steps) showsGlobalPolicy(ctx context.Context, policy string) error {
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	card := s.card(page, "Global policies")
	if err := s.expect.Locator(card.GetByText(policyLabel(policy))).ToBeVisible(); err != nil {
		return fmt.Errorf("the Global policies card never showed %q: %w", policy, err)
	}
	return nil
}

// selectOption picks an option from the drop-down labelled label.
func (s *Steps) selectOption(ctx context.Context, option, label string) error {
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByRole("combobox", playwright.PageGetByRoleOptions{Name: fieldLabel(label)}).Click(); err != nil {
		return fmt.Errorf("opening the %q drop-down: %w", label, err)
	}
	if err := page.GetByRole("option", playwright.PageGetByRoleOptions{Name: option, Exact: playwright.Bool(true)}).Click(); err != nil {
		return fmt.Errorf("choosing %q as the %q: %w", option, label, err)
	}
	return nil
}

// overviewCardShows expects text inside the overview card with the given heading.
func (s *Steps) overviewCardShows(ctx context.Context, heading, text string) error {
	resolved, err := uivalue.Expand(ctx, text)
	if err != nil {
		return err
	}
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	if err := s.expect.Locator(s.card(page, heading).GetByText(resolved).First()).ToBeVisible(); err != nil {
		return fmt.Errorf("the %q card never showed %q: %w", heading, resolved, err)
	}
	return nil
}

// documentListedUnder expects the document in the region its type's group toggle controls. The
// toggle is named "<type> (<count>)".
func (s *Steps) documentListedUnder(ctx context.Context, name, documentType string) error {
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	group := page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: regexp.MustCompile(`^` + regexp.QuoteMeta(documentType) + ` \(\d+\)$`),
	})
	if err := s.expect.Locator(group).ToBeVisible(); err != nil {
		return fmt.Errorf("the documents page never showed a %q group: %w", documentType, err)
	}
	region, err := group.GetAttribute("aria-controls")
	if err != nil || region == "" {
		return fmt.Errorf("the %q group names no region: %v", documentType, err)
	}
	listed := page.Locator("[id=\""+region+"\"]").GetByText(name, playwright.LocatorGetByTextOptions{Exact: playwright.Bool(true)})
	if err := s.expect.Locator(listed).ToBeVisible(); err != nil {
		return fmt.Errorf("the %q group never listed %q: %w", documentType, name, err)
	}
	return nil
}

// catalogEntryAttachable reads how the Policies page offers a catalog entry: an attachable one is
// draggable, a read-only page marks it aria-disabled and not draggable.
func (s *Steps) catalogEntryAttachable(ctx context.Context, policy, can string) error {
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	entry := page.Locator(".MuiCard-root").Filter(playwright.LocatorFilterOptions{
		Has: page.GetByText(policy, playwright.PageGetByTextOptions{Exact: playwright.Bool(true)}),
	}).Last()
	attachable := can == "be"
	if err := s.expect.Locator(entry).ToHaveAttribute("draggable", fmt.Sprint(attachable)); err != nil {
		return fmt.Errorf("the catalog's %q entry can %s attached: %w", policy, can, err)
	}
	disabled := s.expect.Locator(entry)
	if attachable {
		disabled = disabled.Not()
	}
	if err := disabled.ToHaveAttribute("aria-disabled", "true"); err != nil {
		return fmt.Errorf("the catalog's %q entry can %s attached: %w", policy, can, err)
	}
	return nil
}

// card is the card whose heading reads heading.
func (s *Steps) card(page playwright.Page, heading string) playwright.Locator {
	return page.Locator(".MuiCard-root").Filter(playwright.LocatorFilterOptions{
		Has: page.GetByRole("heading", playwright.PageGetByRoleOptions{Name: heading, Exact: playwright.Bool(true)}),
	})
}

// gatewayAnswersAs sends the request with Basic credentials until the gateway answers status.
func (s *Steps) gatewayAnswersAs(ctx context.Context, status int, method, path, username, password string) error {
	resolved, err := uivalue.Expand(ctx, path)
	if err != nil {
		return err
	}
	base, err := s.topo.URL("platform-gateway", "http")
	if err != nil {
		return err
	}
	request := httpx.Request{Method: method, URL: base + resolved, Headers: map[string]string{
		"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password)),
	}}
	return retry.Await(ctx, retry.Options{}, func(ctx context.Context) (*httpx.Response, error) {
		return s.client.Do(ctx, request, 0, 0)
	}, func(resp *httpx.Response) bool { return resp != nil && resp.StatusCode == status },
		fmt.Sprintf("the gateway never answered %d to %s %s as %q", status, method, resolved, username))
}
