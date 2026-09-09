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

	"github.com/wso2/api-platform/tests/framework/core/coverage"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// assertTimeoutMs bounds every web-first assertion. Generous over a fresh SPA load,
// still far below any scenario timeout.
const assertTimeoutMs = 15000

const keyBrowserCoverageReset = "uiBrowserCoverageReset"

// UI holds what the UI steps need.
type UI struct {
	topo         *frameworkruntime.Topology
	expect       playwright.PlaywrightAssertions
	coverageSink *coverage.Sink
}

// New builds the step set for one block's topology.
func New(topo *frameworkruntime.Topology, coverageSink *coverage.Sink) *UI {
	return &UI{
		topo:         topo,
		expect:       playwright.NewPlaywrightAssertions(assertTimeoutMs),
		coverageSink: coverageSink,
	}
}

// Register wires the browser lifecycle and every step this suite provides.
func (u *UI) Register(sc *godog.ScenarioContext) {
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		return u.openScenarioPage(ctx)
	})
	sc.StepContext().After(func(ctx context.Context, _ *godog.Step, _ godog.StepResultStatus, _ error) (context.Context, error) {
		if u.coverageSink == nil || tcontext.Contains(ctx, keyBrowserCoverageReset) {
			return ctx, nil
		}
		if err := u.resetBrowserCoverage(ctx); err != nil {
			return ctx, err
		}
		if err := tcontext.Set(ctx, keyBrowserCoverageReset, true); err != nil {
			return ctx, err
		}
		return ctx, nil
	})
	sc.After(func(ctx context.Context, scn *godog.Scenario, scenarioErr error) (context.Context, error) {
		if scenarioErr != nil {
			name := artifactBaseName(scn.Name)
			u.saveFailureArtifacts(ctx, name)
			u.stopTracing(ctx, name, true)
		} else {
			u.stopTracing(ctx, "", false)
		}
		coverageErr := u.collectBrowserCoverage(ctx, scn.Name)
		u.closeScenarioPage(ctx)
		if coverageErr != nil {
			return ctx, coverageErr
		}
		// The step's own failure is already recorded; returning it again would re-report
		// it as a hook failure.
		return ctx, nil
	})

	sc.Step(`^the user opens the workspace$`, u.openWorkspace)
	sc.Step(`^the browser has no runtime configuration$`, u.withoutRuntimeConfiguration)
	sc.Step(`^the runtime configuration fallback is active$`, u.runtimeConfigurationFallbackIsActive)
	sc.Step(`^the user sees the sign-in form$`, u.seesSignInForm)
	sc.Step(`^the user signs in as the administrator$`, u.signInAsAdministrator)
	sc.Step(`^the user lands on the organization home$`, u.landsOnOrganizationHome)
	sc.Step(`^the user is signed in$`, u.isSignedIn)

	sc.Step(`^the user creates a project named "([^"]*)"$`, u.createProject)
	sc.Step(`^the user sees "([^"]*)" among the projects$`, u.seesAmongProjects)
	sc.Step(`^the user starts adding a provider from the "([^"]*)" template$`, u.startAddingProviderFromTemplate)
	sc.Step(`^the user creates the provider "([^"]*)" pointed at the mock LLM$`, u.createProvider)
	sc.Step(`^the user creates the provider "([^"]*)" using the template's built-in endpoint$`, u.createProviderFromBuiltInEndpoint)
	sc.Step(`^the user is on the provider's overview page$`, u.onProviderOverview)
	sc.Step(`^the user deploys it to the gateway$`, u.deploysToGateway)
	sc.Step(`^the user sees the deployment is active$`, u.seesDeploymentActive)
	sc.Step(`^the user returns to the provider overview$`, u.returnsToProviderOverview)
	sc.Step(`^the user returns to the proxy overview$`, u.returnsToProxyOverview)
	sc.Step(`^the user generates an API key named "([^"]*)"$`, u.generatesAPIKey)
	sc.Step(`^the user creates an app LLM proxy "([^"]*)" in project "([^"]*)" using that key$`, u.createProxyInProject)
	sc.Step(`^the user creates an app LLM proxy "([^"]*)" in project "([^"]*)" using the API key "([^"]*)"$`, u.createProxyInProjectUsingAPIKey)
	sc.Step(`^the user is on the proxy's overview page$`, u.onProxyOverview)
	sc.Step(`^the user invokes the proxy's chat completions endpoint with that key$`, u.invokesChatCompletions)
	sc.Step(`^the completion answers "([^"]*)"$`, u.completionAnswers)
	sc.Step(`^the user deletes the proxy$`, u.deletesTheProxy)
	sc.Step(`^the user is back on the proxy list$`, u.backOnProxyList)
	sc.Step(`^the user sees "([^"]*)" on the page$`, u.seesOnPage)
	sc.Step(`^the user no longer sees "([^"]*)"$`, u.noLongerSees)

	sc.Step(`^a secret "([^"]*)" already holds the value "([^"]*)"$`, u.aSecretAlreadyHoldsTheValue)
	sc.Step(`^fetching the secret "([^"]*)" directly returns no plaintext value$`, u.fetchingTheSecretDirectlyReturnsNoPlaintextValue)
	sc.Step(`^creating a secret always fails$`, u.secretCreationAlwaysFails)
	sc.Step(`^the user creates the provider "([^"]*)" using the template's built-in endpoint with the credential "([^"]*)"$`, u.submitsProviderWithCredential)
	sc.Step(`^the user creates the provider "([^"]*)" using the template's built-in endpoint with the credential placeholder "([^"]*)"$`, u.submitsProviderWithCredentialPlaceholder)
	sc.Step(`^the user opens the provider's Connection tab$`, u.opensProviderConnectionTab)
	sc.Step(`^the user opens the provider "([^"]*)" from the provider list$`, u.opensProviderFromList)
	sc.Step(`^the user changes the provider's credential to "([^"]*)"$`, u.changesProviderCredential)
	sc.Step(`^the user changes the provider's credential to the placeholder referencing "([^"]*)"$`, u.changesProviderCredentialToPlaceholder)
	sc.Step(`^a secret was created for that credential$`, u.aSecretWasCreatedForThatCredential)
	sc.Step(`^no secret was created for that credential$`, u.noSecretWasCreatedForThatCredential)
	sc.Step(`^the provider was created with a placeholder referencing that secret, not the credential "([^"]*)"$`, u.theProviderWasCreatedWithAPlaceholder)
	sc.Step(`^the provider was created with the placeholder referencing "([^"]*)"$`, u.theProviderWasCreatedWithThePlaceholderReferencing)
	sc.Step(`^the provider was updated with a placeholder referencing that secret, not the credential "([^"]*)"$`, u.theProviderWasUpdatedWithAPlaceholder)
	sc.Step(`^the provider was updated with the placeholder referencing "([^"]*)"$`, u.theProviderWasUpdatedWithThePlaceholderReferencing)
	sc.Step(`^no provider was created$`, u.theProviderWasNotCreated)
	sc.Step(`^the provider was not updated$`, u.theProviderWasNotUpdated)
	sc.Step(`^the provider was updated$`, u.theProviderWasUpdated)
	sc.Step(`^the page never shows the credential "([^"]*)"$`, u.pageNeverShows)
	sc.Step(`^the user sees an error notification$`, u.seesAnErrorNotification)

	sc.Step(`^the user creates an app LLM proxy "([^"]*)" in project "([^"]*)" using the API key placeholder referencing "([^"]*)"$`, u.createProxyInProjectUsingAPIKeyPlaceholder)
	sc.Step(`^the user opens the proxy's Provider tab$`, u.opensProxyProviderTab)
	sc.Step(`^the user changes the proxy's credential to "([^"]*)"$`, u.changesProxyCredential)
	sc.Step(`^the user changes the proxy's credential to the placeholder referencing "([^"]*)"$`, u.changesProxyCredentialToPlaceholder)
	sc.Step(`^the proxy was created with a placeholder referencing that secret, not the credential "([^"]*)"$`, u.theProxyWasCreatedWithAPlaceholder)
	sc.Step(`^the proxy was created with the placeholder referencing "([^"]*)"$`, u.theProxyWasCreatedWithThePlaceholderReferencing)
	sc.Step(`^the proxy was updated with a placeholder referencing that secret, not the credential "([^"]*)"$`, u.theProxyWasUpdatedWithAPlaceholder)
	sc.Step(`^the proxy was updated with the placeholder referencing "([^"]*)"$`, u.theProxyWasUpdatedWithThePlaceholderReferencing)
	sc.Step(`^no proxy was created$`, u.theProxyWasNotCreated)
	sc.Step(`^the proxy was not updated$`, u.theProxyWasNotUpdated)
	sc.Step(`^the proxy was updated$`, u.theProxyWasUpdated)
	sc.Step(`^the current secret is remembered as the original$`, u.theCurrentSecretIsRememberedAsTheOriginal)
	sc.Step(`^the original secret is now deprecated$`, u.theOriginalSecretIsNowDeprecated)

	sc.Step(`^the user opens the project "([^"]*)"$`, u.opensProject)
	sc.Step(`^the user opens MCP Proxies$`, u.opensMCPProxies)
	sc.Step(`^the user creates an MCP proxy "([^"]*)" using the sample URL$`, u.createsMCPProxyUsingSampleURL)
	sc.Step(`^the user is on the MCP proxy's overview page$`, u.onMCPProxyOverview)
	sc.Step(`^the user deletes the MCP proxy "([^"]*)"$`, u.deletesMCPProxy)
	sc.Step(`^the user returns to the organization level$`, u.returnsToOrganizationLevel)
	sc.Step(`^the user opens the projects list$`, u.opensProjectsList)
	sc.Step(`^the user deletes the project "([^"]*)"$`, u.deletesProject)

	sc.Step(`^the user creates the MCP proxy "([^"]*)" at "([^"]*)" with the auth header "([^"]*)" set to the placeholder referencing "([^"]*)"$`, u.submitsMCPProxyWithCredentialPlaceholder)
	sc.Step(`^the user creates the MCP proxy "([^"]*)" at "([^"]*)" with the auth header "([^"]*)" set to "([^"]*)"$`, u.submitsMCPProxyWithCredential)
	sc.Step(`^the user creates the MCP proxy "([^"]*)" at "([^"]*)" with no credential$`, u.submitsMCPProxyWithoutCredential)
	sc.Step(`^the user validates the MCP proxy endpoint "([^"]*)" with the auth header "([^"]*)" set to "([^"]*)"$`, u.validatesMCPProxyEndpoint)
	sc.Step(`^the MCP proxy was created with a placeholder referencing that secret, not the credential "([^"]*)"$`, u.theMCPProxyWasCreatedWithAPlaceholder)
	sc.Step(`^no MCP proxy was created$`, u.noMCPProxyWasCreated)
	sc.Step(`^the MCP proxy was created without an auth block$`, u.theMCPProxyWasCreatedWithoutAnAuthBlock)
	sc.Step(`^the secret handle is a random UUID$`, u.theSecretHandleIsARandomUUID)

	sc.Step(`^the user opens the MCP proxy's Policies tab$`, u.opensMCPProxyPoliciesTab)
	sc.Step(`^the user adds a CORS policy and saves$`, u.addsACORSPolicyAndSaves)
	sc.Step(`^the MCP proxy update kept the auth header "([^"]*)" and type "([^"]*)"$`, u.theMCPProxyUpdateKeptTheAuthHeaderAndType)
	sc.Step(`^the MCP proxy update had no auth block$`, u.theMCPProxyUpdateHadNoAuthBlock)
	sc.Step(`^the MCP proxy update body does not include "([^"]*)"$`, u.theMCPProxyUpdateBodyDoesNotInclude)

	sc.Step(`^the user creates the AI gateway "([^"]*)" at "([^"]*)"$`, u.createsAIGateway)
	sc.Step(`^the user is on the AI gateway's overview page$`, u.onAIGatewayOverview)
	sc.Step(`^the user opens AI Gateways$`, u.opensAIGateways)
	sc.Step(`^the user deletes the AI gateway "([^"]*)"$`, u.deletesAIGateway)

	sc.Step(`^the user opens the MCP proxy's Backend Connection tab$`, u.opensMCPProxyBackendConnectionTab)
	sc.Step(`^the backend connection auth value field shows the masked sentinel$`, u.theBackendConnectionAuthValueFieldShowsTheMaskedSentinel)
	sc.Step(`^the user edits the backend connection URL to "([^"]*)"$`, u.editsBackendConnectionURL)
	sc.Step(`^the user edits the backend connection auth header to "([^"]*)"$`, u.editsBackendConnectionAuthHeader)
	sc.Step(`^the user edits the backend connection auth value to "([^"]*)"$`, u.editsBackendConnectionAuthValue)
	sc.Step(`^the user refetches the server info$`, u.clicksRefetchServerInfo)
	sc.Step(`^the user saves the backend connection$`, u.savesBackendConnection)
	sc.Step(`^the refetch request used only the stored proxy$`, u.theRefetchRequestUsedOnlyTheStoredProxy)
	sc.Step(`^the refetch request sent the live credential for "([^"]*)" with header "([^"]*)" and value "([^"]*)"$`, u.theRefetchRequestSentTheLiveCredential)
	sc.Step(`^the refetch request used the edited URL "([^"]*)" alongside the stored proxy$`, u.theRefetchRequestUsedTheEditedURLAndStoredProxy)
	sc.Step(`^the MCP proxy update carries the URL "([^"]*)"$`, u.theMCPProxyUpdateCarriesTheURL)
	sc.Step(`^the MCP proxy was updated with a placeholder referencing that secret, not the credential "([^"]*)"$`, u.theMCPProxyWasUpdatedWithAPlaceholder)

	sc.Step(`^the user opens GenAI Applications$`, u.opensGenAIApplications)
	sc.Step(`^the user creates the GenAI application "([^"]*)"$`, u.createsGenAIApplication)
	sc.Step(`^the user is on the GenAI application's overview page$`, u.onGenAIApplicationOverview)
	sc.Step(`^the user deletes the GenAI application "([^"]*)"$`, u.deletesGenAIApplication)

	sc.Step(`^the user creates the LLM provider template "([^"]*)" at "([^"]*)"$`, u.createsLLMProviderTemplate)
	sc.Step(`^the user opens the LLM provider template "([^"]*)"$`, u.opensLLMProviderTemplate)
	sc.Step(`^the user creates version "([^"]*)" of the template from version "([^"]*)" at "([^"]*)"$`,
		func(ctx context.Context, toVersion, fromVersion, url string) error {
			return u.createsLLMProviderTemplateVersion(ctx, fromVersion, toVersion, url)
		})
	sc.Step(`^the user sees a "([^"]*)" version button$`, u.seesVersionButton)
	sc.Step(`^the user creates the provider "([^"]*)" from the "([^"]*)" template's "([^"]*)" version$`,
		u.createsProviderFromTemplateVersion)
	sc.Step(`^the user attempts to delete the current template version$`, u.attemptsToDeleteCurrentTemplateVersion)
	sc.Step(`^the user deletes the provider "([^"]*)" directly$`, u.deletesProviderDirectly)
	sc.Step(`^the user deletes the template's "([^"]*)" version$`, u.deletesTemplateVersion)
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

func (u *UI) withoutRuntimeConfiguration(ctx context.Context) error {
	v, ok := tcontext.Get(ctx, keyBrowserContext)
	if !ok {
		return fmt.Errorf("ui: no browser context in scope")
	}
	bctx, ok := v.(playwright.BrowserContext)
	if !ok {
		return fmt.Errorf("ui: browser context has unexpected type %T", v)
	}
	return bctx.Route("**/runtime-config.js", func(route playwright.Route) {
		_ = route.Fulfill(playwright.RouteFulfillOptions{
			Body:        "globalThis.__RUNTIME_CONFIG__ = {};",
			ContentType: playwright.String("application/javascript"),
		})
	})
}

func (u *UI) runtimeConfigurationFallbackIsActive(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	value, err := page.Evaluate(`globalThis.__RUNTIME_CONFIG__ || null`)
	if err != nil {
		return fmt.Errorf("reading runtime configuration: %w", err)
	}
	if value == nil {
		return fmt.Errorf("runtime configuration fallback was not loaded")
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
