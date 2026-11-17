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
	"fmt"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/suites/ui/steps/uivalue"
)

// KindCustomPolicy is every version of a custom policy, by name, synced into the organization
// from the gateway Policies tab. It is deleted after the APIs, which may use it.
var KindCustomPolicy = cleanup.Kind{Name: "management-portal-custom-policy", Order: cleanup.KindAPI.Order + 10}

func (s *Steps) registerGatewayPolicySteps(sc *godog.ScenarioContext) {
	sc.Step(`^the user opens the "([^"]*)" tab in the Management Portal$`, s.openTab)
	sc.Step(`^the user searches for "([^"]*)" in the "([^"]*)" search field in the Management Portal$`, s.search)
	sc.Step(`^the user clicks "([^"]*)" on gateway policy "([^"]*)" version "([^"]*)" in the Management Portal$`,
		s.clickOnGatewayPolicy)
	sc.Step(`^gateway policy "([^"]*)" version "([^"]*)" in the Management Portal shows "([^"]*)"$`, s.gatewayPolicyShows)
	sc.Step(`^the custom policy "([^"]*)" synced in the Management Portal is deleted after the scenario$`,
		s.deleteSyncedCustomPolicyAfterScenario)
}

func (s *Steps) openTab(ctx context.Context, name string) error {
	resolved, err := uivalue.Expand(ctx, name)
	if err != nil {
		return err
	}
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByRole("tab", playwright.PageGetByRoleOptions{Name: resolved, Exact: playwright.Bool(true)}).Click(); err != nil {
		return fmt.Errorf("opening the %q tab: %w", resolved, err)
	}
	return nil
}

// search fills a search field, which carries only a placeholder, not a label.
func (s *Steps) search(ctx context.Context, value, placeholder string) error {
	resolved, err := uivalue.Expand(ctx, value)
	if err != nil {
		return err
	}
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	field := page.GetByPlaceholder(placeholder, playwright.PageGetByPlaceholderOptions{Exact: playwright.Bool(true)})
	if err := field.Fill(resolved); err != nil {
		return fmt.Errorf("searching for %q in the %q field: %w", resolved, placeholder, err)
	}
	return nil
}

// gatewayPolicyRow is the gateway Policies table row of one policy version. The version is
// matched exactly, so a row that names it in its sync status ("Replaces v1.0.0") is not chosen.
func (s *Steps) gatewayPolicyRow(ctx context.Context, name, version string) (playwright.Locator, error) {
	resolvedName, err := uivalue.Expand(ctx, name)
	if err != nil {
		return nil, err
	}
	resolvedVersion, err := uivalue.Expand(ctx, version)
	if err != nil {
		return nil, err
	}
	page, err := s.pageGetter(ctx)
	if err != nil {
		return nil, err
	}
	exact := playwright.PageGetByTextOptions{Exact: playwright.Bool(true)}
	return page.GetByRole("row").
		Filter(playwright.LocatorFilterOptions{Has: page.GetByText(resolvedName, exact)}).
		Filter(playwright.LocatorFilterOptions{Has: page.GetByText(resolvedVersion, exact)}), nil
}

func (s *Steps) clickOnGatewayPolicy(ctx context.Context, button, name, version string) error {
	resolved, err := uivalue.Expand(ctx, button)
	if err != nil {
		return err
	}
	row, err := s.gatewayPolicyRow(ctx, name, version)
	if err != nil {
		return err
	}
	if err := row.GetByRole("button", playwright.LocatorGetByRoleOptions{Name: resolved, Exact: playwright.Bool(true)}).Click(); err != nil {
		return fmt.Errorf("clicking %q on gateway policy %q %s: %w", resolved, name, version, err)
	}
	return nil
}

func (s *Steps) gatewayPolicyShows(ctx context.Context, name, version, text string) error {
	resolved, err := uivalue.Expand(ctx, text)
	if err != nil {
		return err
	}
	row, err := s.gatewayPolicyRow(ctx, name, version)
	if err != nil {
		return err
	}
	shown := row.GetByText(resolved, playwright.LocatorGetByTextOptions{Exact: playwright.Bool(true)})
	if err := s.expect.Locator(shown).ToBeVisible(); err != nil {
		return fmt.Errorf("gateway policy %q %s never showed %q: %w", name, version, resolved, err)
	}
	return nil
}

// deleteSyncedCustomPolicyAfterScenario registers, before the click that syncs it, every version
// of the custom policy name the organization will hold. A policy that was never synced is not a
// cleanup failure.
func (s *Steps) deleteSyncedCustomPolicyAfterScenario(ctx context.Context, name string) error {
	resolved, err := uivalue.Expand(ctx, name)
	if err != nil {
		return err
	}
	return cleanup.Register(ctx, cleanup.Resource{Kind: KindCustomPolicy, ID: resolved, Actor: s.topo.Admin.Username})
}
