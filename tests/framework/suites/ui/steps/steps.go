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

// Package steps holds the UI suite's step definitions: godog features driving a real
// browser (on the block's network) against the AI Workspace.
package steps

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
)

// assertTimeoutMs bounds every web-first assertion. Generous over a fresh SPA load,
// still far below any scenario timeout.
const assertTimeoutMs = 15000

// UI holds what the UI steps need.
type UI struct {
	topo   *frameworkruntime.Topology
	expect playwright.PlaywrightAssertions
}

// New builds the step set for one block's topology.
func New(topo *frameworkruntime.Topology) *UI {
	return &UI{topo: topo, expect: playwright.NewPlaywrightAssertions(assertTimeoutMs)}
}

// Register wires the browser lifecycle and every step this suite provides.
func (u *UI) Register(sc *godog.ScenarioContext) {
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		return u.openScenarioPage(ctx)
	})
	sc.After(func(ctx context.Context, scn *godog.Scenario, scenarioErr error) (context.Context, error) {
		if scenarioErr != nil {
			name := artifactBaseName(scn.Name)
			u.saveFailureArtifacts(ctx, name)
			u.stopTracing(ctx, name, true)
		} else {
			u.stopTracing(ctx, "", false)
		}
		u.closeScenarioPage(ctx)
		// The step's own failure is already recorded; returning it again would re-report
		// it as a hook failure.
		return ctx, nil
	})

	sc.Step(`^the user opens the workspace$`, u.openWorkspace)
	sc.Step(`^the user sees the sign-in form$`, u.seesSignInForm)
	sc.Step(`^the user signs in as the administrator$`, u.signInAsAdministrator)
	sc.Step(`^the user lands on the organization home$`, u.landsOnOrganizationHome)
	sc.Step(`^the user is signed in$`, u.isSignedIn)

	sc.Step(`^the user creates a project named "([^"]*)"$`, u.createProject)
	sc.Step(`^the user sees "([^"]*)" among the projects$`, u.seesAmongProjects)
	sc.Step(`^the user starts adding a provider from the "([^"]*)" template$`, u.startAddingProviderFromTemplate)
	sc.Step(`^the user creates the provider "([^"]*)" pointed at the mock LLM$`, u.createProvider)
	sc.Step(`^the user is on the provider's overview page$`, u.onProviderOverview)
	sc.Step(`^the user deploys it to the gateway$`, u.deploysToGateway)
	sc.Step(`^the user sees the deployment is active$`, u.seesDeploymentActive)
	sc.Step(`^the user returns to the provider overview$`, u.returnsToProviderOverview)
	sc.Step(`^the user returns to the proxy overview$`, u.returnsToProxyOverview)
	sc.Step(`^the user generates an API key named "([^"]*)"$`, u.generatesAPIKey)
	sc.Step(`^the user creates an app LLM proxy "([^"]*)" in project "([^"]*)" using that key$`, u.createProxyInProject)
	sc.Step(`^the user is on the proxy's overview page$`, u.onProxyOverview)
	sc.Step(`^the user invokes the proxy's chat completions endpoint with that key$`, u.invokesChatCompletions)
	sc.Step(`^the completion answers "([^"]*)"$`, u.completionAnswers)
	sc.Step(`^the user deletes the proxy$`, u.deletesTheProxy)
	sc.Step(`^the user is back on the proxy list$`, u.backOnProxyList)
	sc.Step(`^the user sees "([^"]*)" on the page$`, u.seesOnPage)
	sc.Step(`^the user no longer sees "([^"]*)"$`, u.noLongerSees)
}

// workspaceURL is the address a user's browser opens: the alias form, resolved on the
// block's network where the browser runs. The /ai-workspace suffix is the SPA's base path.
func (u *UI) workspaceURL() (string, error) {
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

func (u *UI) openWorkspace(ctx context.Context) error {
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

func (u *UI) seesSignInForm(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(`input[placeholder="username"]`)).ToBeVisible(); err != nil {
		return fmt.Errorf("the username field never became visible: %w", err)
	}
	return u.expect.Locator(page.Locator(`input[type="password"]`)).ToBeVisible()
}
