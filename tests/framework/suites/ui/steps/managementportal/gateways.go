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
	"regexp"
	"time"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/catalog/platformapi"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/suites/ui/steps/uivalue"
)

// tokenLine is the registration token in the env file the gateway page tells the user to write.
var tokenLine = regexp.MustCompile(`APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN=(\S+)`)

func (s *Steps) registerGatewaySteps(sc *godog.ScenarioContext) {
	sc.Step(`^a gateway is started as "([^"]*)" with the registration token the Management Portal shows$`, s.startGatewayWithShownToken)
	sc.Step(`^the gateway "([^"]*)" shows as "([^"]*)" in the Management Portal's gateway list$`, s.gatewayListStatus)
	sc.Step(`^the gateway "([^"]*)" provisioned in the Management Portal is deleted after the scenario$`, s.deleteConsoleGatewayAfterScenario)
}

// startGatewayWithShownToken starts another gateway of the block's kind, named with suffix, with
// the token the gateway page shows. The page's control-plane host is the console's configured
// public address; inside the block the gateway reaches the control plane by its service name.
func (s *Steps) startGatewayWithShownToken(ctx context.Context, suffix string) error {
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	command := page.Locator("pre, code").Filter(playwright.LocatorFilterOptions{HasText: tokenLine})
	if err := s.expect.Locator(command.First()).ToBeVisible(); err != nil {
		return fmt.Errorf("the gateway page never showed a registration token: %w", err)
	}
	text, err := command.First().TextContent()
	if err != nil {
		return fmt.Errorf("reading the registration token: %w", err)
	}
	match := tokenLine.FindStringSubmatch(text)
	if match == nil {
		return fmt.Errorf("the gateway page's command holds no registration token: %q", text)
	}
	if _, err := s.topo.LaunchComposeInstance(ctx, "platform-gateway", suffix, platformapi.GatewayControlPlaneEnv(match[1])); err != nil {
		return err
	}
	return nil
}

// gatewayListStatus reloads the gateway list until the gateway's card shows the status. A
// gateway reports in a few seconds after it starts.
func (s *Steps) gatewayListStatus(ctx context.Context, name, status string) error {
	resolved, err := uivalue.Expand(ctx, name)
	if err != nil {
		return err
	}
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	err = retry.Await(ctx, retry.Options{Timeout: 90 * time.Second, Interval: 3 * time.Second},
		func(ctx context.Context) (bool, error) {
			if err := s.openPage(ctx, "/organizations/default/gateways"); err != nil {
				return false, err
			}
			card := page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Open " + resolved, Exact: playwright.Bool(true)})
			chip := card.GetByText(status, playwright.LocatorGetByTextOptions{Exact: playwright.Bool(true)})
			if err := s.expect.Locator(chip).ToBeVisible(playwright.LocatorAssertionsToBeVisibleOptions{Timeout: playwright.Float(3000)}); err != nil {
				return false, nil
			}
			return true, nil
		}, func(shown bool) bool { return shown }, fmt.Sprintf("the gateway list never showed %q as %q", resolved, status))
	return err
}

// deleteConsoleGatewayAfterScenario registers a gateway the console provisions, which the
// framework never sees. A 404 at teardown counts as deleted.
func (s *Steps) deleteConsoleGatewayAfterScenario(ctx context.Context, name string) error {
	resolved, err := uivalue.Expand(ctx, name)
	if err != nil {
		return err
	}
	return cleanup.Register(ctx, cleanup.Resource{Kind: cleanup.KindGateway, ID: resolved, Actor: s.topo.Admin.Username})
}
