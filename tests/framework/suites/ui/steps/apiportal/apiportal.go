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

package apiportal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

// Steps contains API Portal-specific UI bindings.
type Steps struct {
	topo       *frameworkruntime.Topology
	expect     playwright.PlaywrightAssertions
	pageGetter func(context.Context) (playwright.Page, error)
	client     *httpx.Client
	noRedirect *httpx.Client
}

// New constructs API Portal bindings using the suite's isolated browser page.
func New(topo *frameworkruntime.Topology, expect playwright.PlaywrightAssertions,
	pageGetter func(context.Context) (playwright.Page, error)) *Steps {
	return &Steps{
		topo: topo, expect: expect, pageGetter: pageGetter,
		client:     httpx.NewClient(httpx.Options{MaxRetries: 1, FollowRedirects: true}),
		noRedirect: httpx.NewClient(httpx.Options{MaxRetries: 1}),
	}
}

func (u *Steps) page(ctx context.Context) (playwright.Page, error) {
	return u.pageGetter(ctx)
}

// Register wires API Portal-specific steps into the scenario context.
func (u *Steps) Register(sc *godog.ScenarioContext) {
	sc.Step(`^the user opens the API Portal$`, u.openAPIPortal)
	sc.Step(`^the API Portal page is visible$`, u.apiPortalPageIsVisible)
	sc.Step(`^the API Portal page does not contain "([^"]*)"$`, u.apiPortalPageDoesNotContain)
	sc.Step(`^the API Portal hero section is visible$`, u.apiPortalHeroIsVisible)
	sc.Step(`^the API Portal API detail page is visible$`, u.apiPortalAPIDetailIsVisible)
	sc.Step(`^the API Portal MCP listing is visible with a server$`, u.apiPortalMCPListingIsVisible)
	sc.Step(`^the API Portal login form is visible$`, u.apiPortalLoginFormIsVisible)
	sc.Step(`^the user signs in to the API Portal through the local form$`, u.signInThroughLocalForm)
	sc.Step(`^the user signs out of the API Portal$`, u.logoutAPIPortal)
	sc.Step(`^the user submits the API Portal login with an incorrect password$`, u.submitAPIPortalLoginWithIncorrectPassword)
	sc.Step(`^the API Portal local login error is visible$`, u.apiPortalLocalLoginErrorIsVisible)
	sc.Step(`^the API Portal has a settings label fixture$`, u.seedSettingsLabelFixture)
	sc.Step(`^the API Portal has a settings view fixture$`, u.seedSettingsViewFixture)
	sc.Step(`^the administrator creates a key manager and a developer application$`, u.createKeyManagerAndDeveloperApplication)
	sc.Step(`^the developer can choose that key manager when creating an OAuth2 key$`, u.developerCanChooseKeyManagerForOAuth2Key)
	sc.Step(`^the user creates a view from a display name$`, u.createSettingsView)
	sc.Step(`^the user renames the settings view$`, u.renameSettingsView)
	sc.Step(`^the default view has an enabled delete control$`, u.defaultViewHasEnabledDeleteControl)
	sc.Step(`^the user creates a label from a display name$`, u.createSettingsLabel)
	sc.Step(`^the user creates an API workflow without a description$`, u.createAPIWorkflowWithoutDescription)
	sc.Step(`^the API Portal Applications page is visible$`, u.apiPortalApplicationsIsVisible)
	sc.Step(`^the API Portal API Keys page is visible$`, u.apiPortalAPIKeysIsVisible)
	sc.Step(`^the API Portal sidebar is collapsed by default and lists navigation items$`, u.sidebarIsCollapsedByDefault)
	sc.Step(`^the user pins the API Portal sidebar open and it persists across APIs navigation and reload$`, u.sidebarPersistsAcrossNavigation)
	sc.Step(`^the user collapses the API Portal sidebar$`, u.collapseSidebar)
	sc.Step(`^the API Portal sidebar is force-collapsed$`, u.sidebarIsForceCollapsed)
	sc.Step(`^the user requests API Portal path "([^"]*)" without following redirects$`, u.requestAPIPortalPathWithoutFollowingRedirects)
	sc.Step(`^the user requests API Portal path "([^"]*)"$`, u.requestAPIPortalPath)
	sc.Step(`^the API Portal response status is (\d+)$`, u.apiPortalResponseStatus)
	sc.Step(`^the API Portal response status is one of (\d+), (\d+), or (\d+)$`, u.apiPortalResponseStatusOneOf)
	sc.Step(`^the API Portal redirect contains "([^"]*)"$`, u.apiPortalRedirectContains)
	sc.Step(`^the API Portal response content type contains "([^"]*)"$`, u.apiPortalResponseContentTypeContains)
	sc.Step(`^the API Portal response body contains "([^"]*)"$`, u.apiPortalResponseBodyContains)
	sc.Step(`^the API Portal response JSON status is "([^"]*)"$`, u.apiPortalResponseJSONStatus)
	sc.Step(`^the API Portal response body does not contain "([^"]*)"$`, u.apiPortalResponseBodyDoesNotContain)
	sc.Step(`^the user is signed in to the API Portal$`, u.isSignedInToAPIPortal)
	sc.Step(`^the API Portal has portal-access fixtures$`, u.seedPortalAccessFixtures)
	sc.Step(`^the API Portal has a REST API details fixture$`, u.seedRESTAPIDetailsFixture)
	sc.Step(`^the API Portal has API listing fixtures$`, u.seedAPIListingFixtures)
	sc.Step(`^the API listing shows REST and GraphQL APIs but not MCP$`, u.apiListingShowsTypes)
	sc.Step(`^the API listing search returns only the GraphQL API$`, u.apiListingSearchReturnsGraphQL)
	sc.Step(`^the API listing search excludes the MCP server$`, u.apiListingSearchExcludesMCP)
	sc.Step(`^the API Portal has MCP listing fixtures$`, u.seedMCPListingFixtures)
	sc.Step(`^the MCP listing shows both servers but not the REST API$`, u.mcpListingShowsTypes)
	sc.Step(`^the MCP listing search returns only the first server$`, u.mcpListingSearchReturnsFirst)
	sc.Step(`^the MCP listing search excludes the REST API$`, u.mcpListingSearchExcludesREST)
	sc.Step(`^the API Portal has an application fixture$`, u.seedApplicationFixture)
	sc.Step(`^the user creates edits and deletes the application$`, u.applicationCRUD)
	sc.Step(`^the application detail shows the key association sections$`, u.applicationHasKeyAssociationSections)
	sc.Step(`^the user opens the seeded REST API overview and sees its details$`, u.openRESTAPIOverview)
	sc.Step(`^the user opens the seeded REST API specification and sees its OpenAPI document$`, u.openRESTAPISpecification)
	sc.Step(`^the user opens the seeded REST API documentation and sees the additional document$`, u.openRESTAPIDocumentation)
	sc.Step(`^the seeded REST API specification exposes the Try It console$`, u.restAPISpecificationExposesTryIt)
	sc.Step(`^the user browses the API listing and opens the seeded API$`, u.browseAPIListingAndOpenSeededAPI)
	sc.Step(`^the user browses the MCP servers listing$`, u.browseMCPListing)
	sc.Step(`^the user opens the API Workflows page$`, u.openAPIWorkflows)
	sc.Step(`^the user opens the Applications page while signed out$`, u.openApplicationsSignedOut)
	sc.Step(`^the user signs in and returns to Applications$`, u.signInAndReturnToApplications)
	sc.Step(`^the user opens the seeded API's API Keys page while signed out$`, u.openSeededAPIKeysSignedOut)
	sc.Step(`^the user signs in and returns to the seeded API's API Keys page$`, u.signInAndReturnToSeededAPIKeys)
}

const (
	keyAPIPortalResponse           = "uiAPIPortalResponse"
	keyAPIPortalAPI                = "uiAPIPortalAPI"
	keyAPIPortalMCP                = "uiAPIPortalMCP"
	keyAPIPortalDetailAPI          = "uiAPIPortalDetailAPI"
	keyAPIPortalDetailAPIName      = "uiAPIPortalDetailAPIName"
	keyAPIPortalListingREST        = "uiAPIPortalListingREST"
	keyAPIPortalListingGraphQL     = "uiAPIPortalListingGraphQL"
	keyAPIPortalListingMCP         = "uiAPIPortalListingMCP"
	keyAPIPortalMCPListingFirst    = "uiAPIPortalMCPListingFirst"
	keyAPIPortalMCPListingSecond   = "uiAPIPortalMCPListingSecond"
	keyAPIPortalMCPListingREST     = "uiAPIPortalMCPListingREST"
	keyAPIPortalApplication        = "uiAPIPortalApplication"
	keyAPIPortalApplicationName    = "uiAPIPortalApplicationName"
	keyAPIPortalKeyManager         = "uiAPIPortalKeyManager"
	keyAPIPortalKeyManagerName     = "uiAPIPortalKeyManagerName"
	keyAPIPortalKeyManagerA        = "uiAPIPortalKeyManagerA"
	keyAPIPortalKeyManagerB        = "uiAPIPortalKeyManagerB"
	keyAPIPortalKeyManagerAName    = "uiAPIPortalKeyManagerAName"
	keyAPIPortalKeyManagerBName    = "uiAPIPortalKeyManagerBName"
	keyAPIPortalKeyManagerASecret  = "uiAPIPortalKeyManagerASecret"
	keyAPIPortalKeyManagerBSecret  = "uiAPIPortalKeyManagerBSecret"
	keyAPIPortalKeyManagerAMapping = "uiAPIPortalKeyManagerAMapping"
	keyAPIPortalKeyManagerBMapping = "uiAPIPortalKeyManagerBMapping"
	keyAPIPortalSettingsLabel      = "uiAPIPortalSettingsLabel"
	keyAPIPortalSettingsLabelName  = "uiAPIPortalSettingsLabelName"
	keyAPIPortalSettingsView       = "uiAPIPortalSettingsView"
	keyAPIPortalWorkflow           = "uiAPIPortalWorkflow"
)

// KindAPIPortalAPI identifies an API owned by the API Portal test plane.
var KindAPIPortalAPI = cleanup.Kind{Name: "api-portal-api", Order: 50}

// KindAPIPortalMCP identifies an MCP server owned by the API Portal test plane.
var KindAPIPortalMCP = cleanup.Kind{Name: "api-portal-mcp-server", Order: 52}

// KindAPIPortalApplication identifies an application owned by the API Portal test plane.
var KindAPIPortalApplication = cleanup.Kind{Name: "api-portal-application", Order: 20}

// KindAPIPortalApplicationKey identifies an application key mapping owned by the API Portal.
var KindAPIPortalApplicationKey = cleanup.Kind{Name: "api-portal-application-key", Order: 10}

// KindAPIPortalKeyManager identifies a key manager owned by the API Portal test plane.
var KindAPIPortalKeyManager = cleanup.Kind{Name: "api-portal-key-manager", Order: 30}

// KindAPIPortalView identifies a view owned by the API Portal test plane.
var KindAPIPortalView = cleanup.Kind{Name: "api-portal-view", Order: 40}

// KindAPIPortalLabel identifies a label owned by the API Portal test plane.
var KindAPIPortalLabel = cleanup.Kind{Name: "api-portal-label", Order: 35}

// KindAPIPortalWorkflow identifies a workflow owned by the API Portal test plane.
var KindAPIPortalWorkflow = cleanup.Kind{Name: "api-portal-workflow", Order: 45}

func (u *Steps) isSignedInToAPIPortal(ctx context.Context) error {
	if err := u.openAPIPortal(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if count, err := page.Locator(".profile-link").Count(); err == nil && count > 0 {
		return nil
	}
	if err := page.Locator(".login-btn").Click(); err != nil {
		return fmt.Errorf("opening API Portal login: %w", err)
	}
	if err := page.Locator("#username").Fill(u.topo.Admin.Username); err != nil {
		return fmt.Errorf("filling API Portal username: %w", err)
	}
	if err := page.Locator("#password").Fill(u.topo.Admin.Password); err != nil {
		return fmt.Errorf("filling API Portal password: %w", err)
	}
	if err := page.Locator(".ln-signin-btn").Click(); err != nil {
		return fmt.Errorf("submitting API Portal login: %w", err)
	}
	return u.expect.Locator(page.Locator(".profile-link")).ToBeVisible()
}

func (u *Steps) signInThroughLocalForm(ctx context.Context) error {
	if err := u.openAPIPortal(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator(".login-btn").Click(); err != nil {
		return fmt.Errorf("opening API Portal login: %w", err)
	}
	if err := u.submitAPIPortalLogin(ctx); err != nil {
		return fmt.Errorf("submitting API Portal login: %w", err)
	}
	if err := u.expect.Page(page).ToHaveURL(regexp.MustCompile(`/views/default`)); err != nil {
		return err
	}
	return u.expect.Locator(page.Locator(".profile-link")).ToContainText(u.topo.Admin.Username)
}

func (u *Steps) submitAPIPortalLoginWithIncorrectPassword(ctx context.Context) error {
	if err := u.openAPIPortal(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator(".login-btn").Click(); err != nil {
		return fmt.Errorf("opening API Portal login: %w", err)
	}
	if err := page.Locator("#username").Fill(u.topo.Admin.Username); err != nil {
		return err
	}
	if err := page.Locator("#password").Fill("wrong-password"); err != nil {
		return err
	}
	return page.Locator(".ln-signin-btn").Click()
}

func (u *Steps) apiPortalLocalLoginErrorIsVisible(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(".ln-error-banner")).ToBeVisible(); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(".ln-error-banner")).ToContainText("Invalid username or password"); err != nil {
		return err
	}
	return u.expect.Page(page).ToHaveURL(regexp.MustCompile(`/login`))
}

func (u *Steps) openSettings(ctx context.Context) (playwright.Page, error) {
	page, err := u.page(ctx)
	if err != nil {
		return nil, err
	}
	base, err := u.apiPortalURL()
	if err != nil {
		return nil, err
	}
	if _, err := page.Goto(base + "/api-portal/default/settings"); err != nil {
		return nil, fmt.Errorf("opening API Portal settings: %w", err)
	}
	return page, nil
}

func (u *Steps) addSettingsLabel(ctx context.Context, page playwright.Page, prefix string) (string, string, error) {
	value, err := unique.Unique(ctx, prefix)
	if err != nil {
		return "", "", err
	}
	if err := page.Locator(`.cfg-nav-item[data-panel="cfg-labels"]`).Click(); err != nil {
		return "", "", fmt.Errorf("opening labels settings: %w", err)
	}
	if err := page.Locator("#cfg-add-label-btn").Click(); err != nil {
		return "", "", fmt.Errorf("opening add-label form: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#cfg-label-modal")).ToBeVisible(); err != nil {
		return "", "", err
	}
	display := "IT Settings Label " + value
	if err := page.Locator("#lbl-display").Fill(display); err != nil {
		return "", "", err
	}
	handle, err := page.Locator("#lbl-name").InputValue()
	if err != nil {
		return "", "", fmt.Errorf("reading generated label handle: %w", err)
	}
	if err := page.Locator("#cfg-label-modal-save").Click(); err != nil {
		return "", "", fmt.Errorf("saving settings label: %w", err)
	}
	row := page.Locator("#cfg-label-row-" + handle)
	if err := u.expect.Locator(row).ToBeVisible(); err != nil {
		return "", "", err
	}
	if err := u.expect.Locator(row).ToContainText(handle); err != nil {
		return "", "", err
	}
	if err := u.expect.Locator(row).ToContainText(display); err != nil {
		return "", "", err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalLabel, ID: handle, Actor: u.topo.Admin.Username}); err != nil {
		_ = u.deletePortalArtifact(ctx, "labels", handle)
		return "", "", fmt.Errorf("registering settings label for cleanup: %w", err)
	}
	return handle, display, nil
}

func (u *Steps) seedSettingsLabelFixture(ctx context.Context) error {
	page, err := u.openSettings(ctx)
	if err != nil {
		return err
	}
	handle, display, err := u.addSettingsLabel(ctx, page, "apiportal-settings-label")
	if err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyAPIPortalSettingsLabel, handle); err != nil {
		return err
	}
	return tcontext.Set(ctx, keyAPIPortalSettingsLabelName, display)
}

func (u *Steps) addSettingsView(ctx context.Context, page playwright.Page, label, prefix string) (string, error) {
	value, err := unique.Unique(ctx, prefix)
	if err != nil {
		return "", err
	}
	if err := page.Locator(`.cfg-nav-item[data-panel="cfg-views"]`).Click(); err != nil {
		return "", fmt.Errorf("opening views settings: %w", err)
	}
	if err := page.Locator("#cfg-add-view-btn").Click(); err != nil {
		return "", fmt.Errorf("opening add-view form: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#cfg-view-modal")).ToBeVisible(); err != nil {
		return "", err
	}
	display := "IT Settings View " + value
	if err := page.Locator("#view-display").Fill(display); err != nil {
		return "", err
	}
	handle, err := page.Locator("#view-handle").InputValue()
	if err != nil {
		return "", fmt.Errorf("reading generated view handle: %w", err)
	}
	if err := page.Locator(`#view-labels .cfg-label-toggle[data-value="` + label + `"]`).Click(); err != nil {
		return "", fmt.Errorf("selecting view label: %w", err)
	}
	if err := page.Locator("#cfg-view-modal-save").Click(); err != nil {
		return "", fmt.Errorf("saving settings view: %w", err)
	}
	row := page.Locator("#cfg-view-row-" + handle)
	if err := u.expect.Locator(row).ToBeVisible(); err != nil {
		return "", err
	}
	if err := u.expect.Locator(row).ToContainText(handle); err != nil {
		return "", err
	}
	if err := u.expect.Locator(row).ToContainText(display); err != nil {
		return "", err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalView, ID: handle, Actor: u.topo.Admin.Username}); err != nil {
		_ = u.deletePortalArtifact(ctx, "views", handle)
		return "", fmt.Errorf("registering settings view for cleanup: %w", err)
	}
	return handle, nil
}

func (u *Steps) seedSettingsViewFixture(ctx context.Context) error {
	page, err := u.openSettings(ctx)
	if err != nil {
		return err
	}
	label, labelName, err := u.addSettingsLabel(ctx, page, "apiportal-settings-view-label")
	if err != nil {
		return err
	}
	view, err := u.addSettingsView(ctx, page, label, "apiportal-settings-view")
	if err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyAPIPortalSettingsLabelName, labelName); err != nil {
		return err
	}
	return tcontext.Set(ctx, keyAPIPortalSettingsView, view)
}

func (u *Steps) createSettingsView(ctx context.Context) error {
	page, err := u.openSettings(ctx)
	if err != nil {
		return err
	}
	labelValue, ok := tcontext.Get(ctx, keyAPIPortalSettingsLabel)
	if !ok {
		return fmt.Errorf("settings label fixture is unavailable")
	}
	label, ok := labelValue.(string)
	if !ok || label == "" {
		return fmt.Errorf("settings label fixture has unexpected type %T", labelValue)
	}
	_, err = u.addSettingsView(ctx, page, label, "apiportal-settings-created-view")
	return err
}

func (u *Steps) renameSettingsView(ctx context.Context) error {
	page, err := u.openSettings(ctx)
	if err != nil {
		return err
	}
	viewValue, ok := tcontext.Get(ctx, keyAPIPortalSettingsView)
	if !ok {
		return fmt.Errorf("settings view fixture is unavailable")
	}
	view, ok := viewValue.(string)
	if !ok || view == "" {
		return fmt.Errorf("settings view fixture has unexpected type %T", viewValue)
	}
	if err := page.Locator(`.cfg-nav-item[data-panel="cfg-views"]`).Click(); err != nil {
		return fmt.Errorf("opening views settings: %w", err)
	}
	if err := page.Locator("#cfg-view-row-" + view + " .cfg-view-edit-btn").First().Click(); err != nil {
		return fmt.Errorf("opening settings view editor: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#cfg-view-modal")).ToBeVisible(); err != nil {
		return err
	}
	newValue, err := unique.Unique(ctx, "apiportal-settings-renamed-view")
	if err != nil {
		return err
	}
	newHandle := "renamed-view-" + strings.ReplaceAll(newValue, "_", "-")
	if err := page.Locator("#view-handle").Fill(newHandle); err != nil {
		return err
	}
	if err := page.Locator("#cfg-view-modal-save").Click(); err != nil {
		return fmt.Errorf("saving renamed settings view: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#cfg-view-row-" + newHandle)).ToBeVisible(); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#cfg-view-row-" + view)).ToHaveCount(0); err != nil {
		return err
	}
	labelValue, ok := tcontext.Get(ctx, keyAPIPortalSettingsLabelName)
	if !ok {
		return fmt.Errorf("settings label fixture display name is unavailable")
	}
	labelName, ok := labelValue.(string)
	if !ok || labelName == "" {
		return fmt.Errorf("settings label fixture display name has unexpected type %T", labelValue)
	}
	renamedRow := page.Locator("#cfg-view-row-" + newHandle)
	if err := u.expect.Locator(renamedRow).ToContainText(labelName); err != nil {
		return err
	}
	if reg, regErr := cleanup.Of(ctx); regErr == nil {
		reg.Deregister(KindAPIPortalView, view)
	}
	if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalView, ID: newHandle, Actor: u.topo.Admin.Username}); err != nil {
		_ = u.deletePortalArtifact(ctx, "views", newHandle)
		return fmt.Errorf("registering renamed settings view for cleanup: %w", err)
	}
	base, err := u.apiPortalURL()
	if err != nil {
		return err
	}
	if _, err := page.Goto(base + "/api-portal/default/views/" + url.PathEscape(newHandle)); err != nil {
		return fmt.Errorf("opening renamed settings view: %w", err)
	}
	return u.expect.Page(page).ToHaveURL(regexp.MustCompile(`/views/` + regexp.QuoteMeta(newHandle)))
}

func (u *Steps) defaultViewHasEnabledDeleteControl(ctx context.Context) error {
	page, err := u.openSettings(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator(`.cfg-nav-item[data-panel="cfg-views"]`).Click(); err != nil {
		return fmt.Errorf("opening views settings: %w", err)
	}
	button := page.Locator(`#cfg-view-row-default .cfg-view-delete-btn`)
	if err := u.expect.Locator(button).ToBeVisible(); err != nil {
		return err
	}
	return u.expect.Locator(button).ToBeEnabled()
}

func (u *Steps) createSettingsLabel(ctx context.Context) error {
	page, err := u.openSettings(ctx)
	if err != nil {
		return err
	}
	_, _, err = u.addSettingsLabel(ctx, page, "apiportal-settings-created-label")
	return err
}

func (u *Steps) createAPIWorkflowWithoutDescription(ctx context.Context) error {
	page, err := u.openSettings(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator(`.cfg-nav-item[data-panel="cfg-workflows"]`).Click(); err != nil {
		return fmt.Errorf("opening workflows settings: %w", err)
	}
	if err := page.Locator("#createApiWorkflowBtn").Click(); err != nil {
		return fmt.Errorf("opening workflow form: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#afStep1")).ToBeVisible(); err != nil {
		return err
	}
	value, err := unique.Unique(ctx, "apiportal-settings-workflow")
	if err != nil {
		return err
	}
	name := "IT Workflow " + value
	if err := page.Locator("#apiWorkflowName").Fill(name); err != nil {
		return err
	}
	handle, err := page.Locator("#apiWorkflowHandle").InputValue()
	if err != nil {
		return fmt.Errorf("reading generated workflow handle: %w", err)
	}
	description, err := page.Locator("#apiWorkflowDescription").InputValue()
	if err != nil {
		return fmt.Errorf("reading workflow description: %w", err)
	}
	if description != "" {
		return fmt.Errorf("workflow description was not empty")
	}
	if err := page.Locator("#afContinueBtn").Click(); err != nil {
		return fmt.Errorf("opening workflow content step: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#afStep2")).ToBeVisible(); err != nil {
		return err
	}
	if err := page.Locator("#arazoFileInput").SetInputFiles([]playwright.InputFile{{
		Name: "workflow.md", MimeType: "text/markdown",
		Buffer: []byte("# IT Workflow\n\nCreated by an integration test.\n"),
	}}); err != nil {
		return fmt.Errorf("uploading workflow content: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#uploadFileName")).ToContainText("workflow.md"); err != nil {
		return err
	}
	if err := page.Locator("#afContinueBtn").Click(); err != nil {
		return fmt.Errorf("opening workflow prompt step: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#afStep3")).ToBeVisible(); err != nil {
		return err
	}
	prompt, err := page.Locator("#agentPromptField").InputValue()
	if err != nil {
		return fmt.Errorf("reading workflow agent prompt: %w", err)
	}
	if strings.TrimSpace(prompt) == "" {
		if err := page.Locator("#agentPromptField").Fill("Use this workflow in an integration test."); err != nil {
			return err
		}
	}
	if err := u.expect.Locator(page.Locator("#afReady1")).ToContainClass("is-ok"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#afReady1")).Not().ToContainClass("is-warn"); err != nil {
		return err
	}
	if err := page.Locator("#saveApiWorkflowBtn").Click(); err != nil {
		return fmt.Errorf("saving API workflow: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#apiWorkflowList")).ToBeVisible(); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#apiWorkflowList")).ToContainText(name); err != nil {
		return err
	}
	edit := page.Locator(`.api-workflow-edit-btn[data-api-workflow-id="` + handle + `"]`)
	if err := u.expect.Locator(edit).ToBeAttached(); err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: KindAPIPortalWorkflow, ID: "default/" + handle, Actor: u.topo.Admin.Username,
	}); err != nil {
		_ = u.deleteAPIWorkflow(ctx, "default", handle)
		return fmt.Errorf("registering API workflow for cleanup: %w", err)
	}
	if err := edit.Locator("xpath=ancestor::td").Locator(".cfg-menu-trigger").Click(); err != nil {
		return fmt.Errorf("opening workflow actions: %w", err)
	}
	if err := edit.Click(); err != nil {
		return fmt.Errorf("opening workflow editor: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#apiWorkflowForm")).ToBeVisible(); err != nil {
		return err
	}
	if got, err := page.Locator("#apiWorkflowName").InputValue(); err != nil {
		return err
	} else if got != name {
		return fmt.Errorf("workflow name did not persist")
	}
	got, err := page.Locator("#apiWorkflowDescription").InputValue()
	if err != nil {
		return err
	}
	if got != "" {
		return fmt.Errorf("workflow description did not remain empty")
	}
	return tcontext.Set(ctx, keyAPIPortalWorkflow, handle)
}

func (u *Steps) deleteAPIWorkflow(ctx context.Context, view, id string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	result, err := page.Evaluate(`async ({view, id}) => {
        const token = document.cookie.split('; ').find(v => v.startsWith('XSRF-TOKEN='));
        const response = await fetch('/api-portal/api/v0.9/views/' + encodeURIComponent(view) + '/api-workflows/' + encodeURIComponent(id), {
            method: 'DELETE',
            headers: { organization: 'default', 'X-CSRF-Token': token ? decodeURIComponent(token.split('=').slice(1).join('=')) : '' },
        });
        return { status: response.status };
    }`, map[string]any{"view": view, "id": id})
	if err != nil {
		return err
	}
	var response struct {
		Status int `json:"status"`
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return err
	}
	if response.Status != http.StatusNotFound && (response.Status < 200 || response.Status >= 300) {
		return fmt.Errorf("API Portal workflow deletion returned HTTP status %d", response.Status)
	}
	return nil
}

func (u *Steps) createKeyManagerAndDeveloperApplication(ctx context.Context) error {
	page, err := u.openSettings(ctx)
	if err != nil {
		return err
	}
	value, err := unique.Unique(ctx, "apiportal-settings-key-manager")
	if err != nil {
		return err
	}
	name := "IT Key Manager " + value
	if err := page.Locator(`.cfg-nav-item[data-panel="cfg-keymanagers"]`).Click(); err != nil {
		return fmt.Errorf("opening key-manager settings: %w", err)
	}
	if err := page.Locator("#cfg-add-km-btn").Click(); err != nil {
		return fmt.Errorf("opening add-key-manager form: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#cfg-km-modal")).ToBeVisible(); err != nil {
		return err
	}
	if err := page.Locator("#km-display").Fill(name); err != nil {
		return err
	}
	// "They already exist": this key manager only proxies token requests for
	// applications registered at the identity server, so it needs no registration
	// endpoint or provisioning credential. The form opens on "The portal creates
	// them", which now refuses a blank registration block rather than silently
	// saving an importing key manager — so a token-proxy-only key manager has to
	// say so explicitly.
	if err := page.Locator("#km-mode-import").Check(); err != nil {
		return fmt.Errorf("choosing the import key-creation mode: %w", err)
	}
	if err := page.Locator("#km-token-endpoint").Fill("https://idp.example.invalid/oauth2/token"); err != nil {
		return err
	}
	if err := page.Locator("#cfg-km-modal-save").Click(); err != nil {
		return fmt.Errorf("saving key manager: %w", err)
	}
	row := page.Locator(".cfg-km-edit-btn").Filter(playwright.LocatorFilterOptions{HasText: regexp.MustCompile(regexp.QuoteMeta(name))})
	if err := u.expect.Locator(row).ToBeVisible(); err != nil {
		return fmt.Errorf("waiting for created key manager: %w", err)
	}
	id, err := row.First().GetAttribute("data-id")
	if err != nil {
		return fmt.Errorf("reading created key manager: %w", err)
	}
	if id == "" {
		return fmt.Errorf("created key manager has no server-generated identifier")
	}
	if err := u.assertKeyManagerListed(page, name, id); err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalKeyManager, ID: id, Actor: u.topo.Admin.Username}); err != nil {
		_ = u.deletePortalArtifact(ctx, "key-managers", id)
		return fmt.Errorf("registering API Portal key manager for cleanup: %w", err)
	}
	if err := tcontext.Set(ctx, keyAPIPortalKeyManagerName, name); err != nil {
		_ = u.deletePortalArtifact(ctx, "key-managers", id)
		return err
	}
	if err := u.loginAsDeveloper(ctx); err != nil {
		return err
	}
	return u.seedApplicationFixture(ctx)
}

func (u *Steps) loginAsDeveloper(ctx context.Context) error {
	usernameValue, err := tcontext.Resolve(ctx, frameworkruntime.KeyDeveloperUser)
	if err != nil {
		return err
	}
	passwordValue, err := tcontext.Resolve(ctx, frameworkruntime.KeyDeveloperPass)
	if err != nil {
		return err
	}
	username, ok := usernameValue.(string)
	if !ok || username == "" {
		return fmt.Errorf("developer username has unexpected type %T", usernameValue)
	}
	password, ok := passwordValue.(string)
	if !ok || password == "" {
		return fmt.Errorf("developer password has unexpected type %T", passwordValue)
	}
	if err := u.logoutAPIPortal(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator("#username").Fill(username); err != nil {
		return err
	}
	if err := page.Locator("#password").Fill(password); err != nil {
		return err
	}
	if err := page.Locator(".ln-signin-btn").Click(); err != nil {
		return fmt.Errorf("signing in as developer: %w", err)
	}
	return u.expect.Locator(page.Locator(".profile-link")).ToBeVisible()
}

func (u *Steps) assertKeyManagerListed(page playwright.Page, expectedName, expectedID string) error {
	listed, err := u.keyManagerListed(page, expectedName, expectedID)
	if err != nil {
		return err
	}
	if listed {
		return nil
	}
	return fmt.Errorf("key-manager list did not contain the expected created key manager")
}

func (u *Steps) keyManagerListed(page playwright.Page, expectedName, expectedID string) (bool, error) {
	result, err := page.Evaluate(`async () => {
        const response = await fetch('/api-portal/api/v0.9/key-managers');
        return { status: response.status, body: await response.text() };
    }`, nil)
	if err != nil {
		return false, fmt.Errorf("requesting developer key-manager list: %w", err)
	}
	var response struct {
		Status int    `json:"status"`
		Body   string `json:"body"`
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return false, fmt.Errorf("reading developer key-manager list response: %w", err)
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return false, fmt.Errorf("decoding developer key-manager list response: %w", err)
	}
	if response.Status != http.StatusOK {
		return false, fmt.Errorf("developer key-manager list returned HTTP status %d", response.Status)
	}
	var payload struct {
		List []struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
		} `json:"list"`
	}
	if err := json.Unmarshal([]byte(response.Body), &payload); err != nil {
		return false, fmt.Errorf("decoding developer key-manager list: %w", err)
	}
	for _, item := range payload.List {
		if item.DisplayName == expectedName && item.ID == expectedID {
			return true, nil
		}
	}
	return false, nil
}

func (u *Steps) developerCanChooseKeyManagerForOAuth2Key(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	nameValue, ok := tcontext.Get(ctx, keyAPIPortalKeyManagerName)
	if !ok {
		return fmt.Errorf("API Portal key manager fixture name is unavailable")
	}
	name, ok := nameValue.(string)
	if !ok || name == "" {
		return fmt.Errorf("API Portal key manager fixture name has unexpected type %T", nameValue)
	}
	// The key manager is offered on the OAuth2 Keys page, not on the developer's
	// application — a key is created there and associated with an application
	// afterwards, so this is where "available to this developer" is observable.
	if err := u.openAPIPortalPath(ctx, "/oauth2-keys"); err != nil {
		return err
	}
	// The listing renders either the toolbar button or, with no keys yet, the one
	// in the empty state. Which of the two is present depends on whether this
	// developer has keys already, so accept either rather than assuming.
	addKey := page.Locator("#ok-add-btn, #ok-add-btn-empty").First()
	if err := addKey.Click(); err != nil {
		return fmt.Errorf("opening the add-OAuth2-key form: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#ok-add-modal")).ToBeVisible(); err != nil {
		return err
	}
	return u.expect.Locator(page.Locator("#ok-km-select")).ToContainText(name)
}

func (u *Steps) seedPortalAccessFixtures(ctx context.Context) error {
	apiID, err := unique.Unique(ctx, "portal-access-api")
	if err != nil {
		return err
	}
	mcpID, err := unique.Unique(ctx, "portal-access-mcp")
	if err != nil {
		return err
	}
	apiMetadata := map[string]any{
		"id": apiID, "name": "IT Portal Access API " + apiID, "version": "v1.0", "type": "REST",
		"status": "PUBLISHED", "labels": []string{"default"},
		"endPoints": map[string]string{
			"productionURL": "https://backend.example.invalid/" + apiID,
			"sandboxURL":    "https://sandbox.example.invalid/" + apiID,
		},
	}
	apiDefinition := map[string]any{
		"openapi": "3.0.3", "info": map[string]string{"title": "IT Portal Access API", "version": "1.0.0"},
		"components": map[string]any{"securitySchemes": map[string]any{
			"ApiKeyAuth": map[string]string{"type": "apiKey", "in": "header", "name": "apikey"},
		}},
		"security": []any{map[string]any{"ApiKeyAuth": []any{}}},
		"paths":    map[string]any{"/ping": map[string]any{"get": map[string]any{"responses": map[string]any{"200": map[string]string{"description": "ok"}}}}},
	}
	if err := u.createPortalArtifact(ctx, "/api-portal/api/v0.9/apis", apiMetadata, apiDefinition); err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: KindAPIPortalAPI, ID: apiID, Actor: u.topo.Admin.Username,
	}); err != nil {
		_ = u.deletePortalArtifact(ctx, "apis", apiID)
		return fmt.Errorf("registering API Portal API %q for cleanup: %w", apiID, err)
	}
	if err := tcontext.Set(ctx, keyAPIPortalAPI, apiID); err != nil {
		_ = u.deletePortalArtifact(ctx, "apis", apiID)
		return err
	}
	mcpMetadata := map[string]any{
		"id": mcpID, "name": "IT Portal Access MCP " + mcpID, "version": "v1.0", "type": "MCP",
		"status": "PUBLISHED", "labels": []string{"default"},
		"endPoints": map[string]string{
			"productionURL": "https://mcp.example.invalid/" + mcpID,
			"sandboxURL":    "https://mcp.example.invalid/" + mcpID,
		},
	}
	mcpDefinition := []any{map[string]any{
		"type": "TOOL", "name": "ping", "description": "Health-check tool.",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
	}}
	if err := u.createPortalArtifact(ctx, "/api-portal/api/v0.9/mcp-servers", mcpMetadata, mcpDefinition); err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: KindAPIPortalMCP, ID: mcpID, Actor: u.topo.Admin.Username,
	}); err != nil {
		_ = u.deletePortalArtifact(ctx, "mcp-servers", mcpID)
		return fmt.Errorf("registering API Portal MCP server %q for cleanup: %w", mcpID, err)
	}
	return tcontext.Set(ctx, keyAPIPortalMCP, mcpID)
}

func (u *Steps) seedRESTAPIDetailsFixture(ctx context.Context) error {
	apiID, err := unique.Unique(ctx, "rest-api-details")
	if err != nil {
		return err
	}
	apiName := "IT REST Detail API " + apiID
	metadata := map[string]any{
		"id": apiID, "name": apiName, "version": "v1.0", "type": "REST",
		"status": "PUBLISHED", "labels": []string{"default"},
		"endPoints": map[string]string{
			"productionURL": "https://backend.example.invalid/" + apiID,
			"sandboxURL":    "https://sandbox.example.invalid/" + apiID,
		},
		"subscriptionPlans": []map[string]string{{"id": "Bronze"}},
	}
	definition := map[string]any{
		"openapi": "3.0.3",
		"info":    map[string]string{"title": apiName, "version": "1.0.0"},
		"servers": []map[string]string{{"url": "https://backend.example.invalid"}},
		"components": map[string]any{"securitySchemes": map[string]any{
			"OAuth2": map[string]any{
				"type": "oauth2", "flows": map[string]any{
					"clientCredentials": map[string]any{
						"tokenUrl": "https://idp.example.invalid/token",
						"scopes":   map[string]string{"read:items": "Read items", "write:items": "Write items"},
					},
				},
			},
		}},
		"security": []map[string][]string{{"OAuth2": {"read:items"}}},
		"paths": map[string]any{
			"/items": map[string]any{
				"get":  map[string]any{"summary": "List items", "responses": map[string]any{"200": map[string]string{"description": "ok"}}},
				"post": map[string]any{"summary": "Create an item", "responses": map[string]any{"201": map[string]string{"description": "created"}}},
			},
			"/items/{id}": map[string]any{
				"get": map[string]any{"summary": "Get an item", "responses": map[string]any{"200": map[string]string{"description": "ok"}}},
			},
		},
	}
	if err := u.createPortalArtifact(ctx, "/api-portal/api/v0.9/apis", metadata, definition,
		map[string]any{"name": "getting-started.md", "content": "# Getting Started\n\nGuide for the IT REST Detail API.\n"}); err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: KindAPIPortalAPI, ID: apiID, Actor: u.topo.Admin.Username,
	}); err != nil {
		_ = u.deletePortalArtifact(ctx, "apis", apiID)
		return fmt.Errorf("registering REST API details fixture for cleanup: %w", err)
	}
	if err := tcontext.Set(ctx, keyAPIPortalDetailAPI, apiID); err != nil {
		_ = u.deletePortalArtifact(ctx, "apis", apiID)
		return err
	}
	return tcontext.Set(ctx, keyAPIPortalDetailAPIName, apiName)
}

func (u *Steps) seedAPIListingFixtures(ctx context.Context) error {
	restID, err := unique.Unique(ctx, "api-listing-rest")
	if err != nil {
		return err
	}
	graphqlID, err := unique.Unique(ctx, "api-listing-graphql")
	if err != nil {
		return err
	}
	mcpID, err := unique.Unique(ctx, "api-listing-mcp")
	if err != nil {
		return err
	}

	endpoint := func(id string) map[string]string {
		return map[string]string{
			"productionURL": "https://backend.example.invalid/" + id,
			"sandboxURL":    "https://sandbox.example.invalid/" + id,
		}
	}
	seedAPI := func(id, name, apiType string, definition any) error {
		metadata := map[string]any{
			"id": id, "name": name, "version": "v1.0", "type": apiType,
			"status": "PUBLISHED", "labels": []string{"default"}, "endPoints": endpoint(id),
		}
		if err := u.createPortalArtifact(ctx, "/api-portal/api/v0.9/apis", metadata, definition); err != nil {
			return err
		}
		if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalAPI, ID: id, Actor: u.topo.Admin.Username}); err != nil {
			_ = u.deletePortalArtifact(ctx, "apis", id)
			return fmt.Errorf("registering API listing API %q for cleanup: %w", id, err)
		}
		return nil
	}

	restName := "IT Listing REST API " + restID
	restDefinition := map[string]any{
		"openapi": "3.0.3", "info": map[string]string{"title": restName, "version": "1.0.0"},
		"paths": map[string]any{"/ping": map[string]any{
			"get": map[string]any{"responses": map[string]any{"200": map[string]string{"description": "ok"}}},
		}},
	}
	if err := seedAPI(restID, restName, "REST", restDefinition); err != nil {
		return err
	}

	graphqlName := "IT Listing GraphQL API " + graphqlID
	if err := seedAPI(graphqlID, graphqlName, "GRAPHQL", "type Query { hello: String }\n"); err != nil {
		return err
	}

	mcpName := "IT Listing MCP Server " + mcpID
	mcpMetadata := map[string]any{
		"id": mcpID, "name": mcpName, "version": "v1.0", "type": "MCP",
		"status": "PUBLISHED", "labels": []string{"default"}, "endPoints": endpoint(mcpID),
	}
	mcpDefinition := []map[string]any{{
		"type": "TOOL", "name": "hello", "description": "A greeting tool",
		"inputSchema": map[string]any{"type": "object"},
	}}
	if err := u.createPortalArtifact(ctx, "/api-portal/api/v0.9/mcp-servers", mcpMetadata, mcpDefinition); err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalMCP, ID: mcpID, Actor: u.topo.Admin.Username}); err != nil {
		_ = u.deletePortalArtifact(ctx, "mcp-servers", mcpID)
		return fmt.Errorf("registering API listing MCP server %q for cleanup: %w", mcpID, err)
	}
	if err := u.waitForPortalCards(ctx, "/apis", restID, graphqlID); err != nil {
		return err
	}
	if err := u.waitForPortalCards(ctx, "/mcps", mcpID); err != nil {
		return err
	}
	for key, value := range map[string]string{
		keyAPIPortalListingREST: restName, keyAPIPortalListingGraphQL: graphqlName, keyAPIPortalListingMCP: mcpName,
	} {
		if err := tcontext.Set(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

func (u *Steps) apiListingNames(ctx context.Context) (string, string, string, error) {
	values := make([]string, 0, 3)
	for _, key := range []string{keyAPIPortalListingREST, keyAPIPortalListingGraphQL, keyAPIPortalListingMCP} {
		value, ok := tcontext.Get(ctx, key)
		if !ok {
			return "", "", "", fmt.Errorf("API listing fixture %q is unavailable", key)
		}
		name, ok := value.(string)
		if !ok || name == "" {
			return "", "", "", fmt.Errorf("API listing fixture %q has unexpected type %T", key, value)
		}
		values = append(values, name)
	}
	return values[0], values[1], values[2], nil
}

func (u *Steps) apiListingShowsTypes(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	restName, graphqlName, mcpName, err := u.apiListingNames(ctx)
	if err != nil {
		return err
	}
	if err := u.openAPIPortalPath(ctx, "/apis"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(".apilist-results-heading")).ToContainText("APIs"); err != nil {
		return err
	}
	texts, err := page.Locator(".api-card").AllTextContents()
	if err != nil {
		return fmt.Errorf("reading API listing cards: %w", err)
	}
	for _, expected := range []string{restName, graphqlName} {
		if !slices.ContainsFunc(texts, func(text string) bool { return strings.Contains(text, expected) }) {
			return fmt.Errorf("API listing did not show %q", expected)
		}
	}
	if slices.ContainsFunc(texts, func(text string) bool { return strings.Contains(text, mcpName) }) {
		return fmt.Errorf("API listing unexpectedly showed MCP server %q", mcpName)
	}
	if count, err := page.Locator(".dp-badge--rest").Count(); err != nil || count < 1 {
		return fmt.Errorf("API listing did not show a REST badge")
	}
	if count, err := page.Locator(".dp-badge--graphql").Count(); err != nil || count < 1 {
		return fmt.Errorf("API listing did not show a GraphQL badge")
	}
	return nil
}

func (u *Steps) apiListingSearchReturnsGraphQL(ctx context.Context) error {
	restName, graphqlName, _, err := u.apiListingNames(ctx)
	if err != nil {
		return err
	}
	return u.searchAPIListing(ctx, graphqlName, graphqlName, restName)
}

func (u *Steps) apiListingSearchExcludesMCP(ctx context.Context) error {
	_, _, mcpName, err := u.apiListingNames(ctx)
	if err != nil {
		return err
	}
	return u.searchAPIListing(ctx, mcpName, "", mcpName)
}

func (u *Steps) seedMCPListingFixtures(ctx context.Context) error {
	firstID, err := unique.Unique(ctx, "mcp-listing-alpha")
	if err != nil {
		return err
	}
	secondID, err := unique.Unique(ctx, "mcp-listing-beta")
	if err != nil {
		return err
	}
	restID, err := unique.Unique(ctx, "mcp-listing-rest")
	if err != nil {
		return err
	}
	endpoint := func(id string) map[string]string {
		return map[string]string{
			"productionURL": "https://backend.example.invalid/" + id,
			"sandboxURL":    "https://sandbox.example.invalid/" + id,
		}
	}
	seedMCP := func(id, name string) error {
		metadata := map[string]any{
			"id": id, "name": name, "version": "v1.0", "type": "MCP",
			"status": "PUBLISHED", "labels": []string{"default"}, "endPoints": endpoint(id),
		}
		definition := []map[string]any{{
			"type": "TOOL", "name": "hello", "description": "A greeting tool",
			"inputSchema": map[string]any{"type": "object"},
		}}
		if err := u.createPortalArtifact(ctx, "/api-portal/api/v0.9/mcp-servers", metadata, definition); err != nil {
			return err
		}
		if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalMCP, ID: id, Actor: u.topo.Admin.Username}); err != nil {
			_ = u.deletePortalArtifact(ctx, "mcp-servers", id)
			return fmt.Errorf("registering MCP listing server %q for cleanup: %w", id, err)
		}
		return nil
	}
	firstName := "IT Listing MCP Alpha " + firstID
	if err := seedMCP(firstID, firstName); err != nil {
		return err
	}
	secondName := "IT Listing MCP Beta " + secondID
	if err := seedMCP(secondID, secondName); err != nil {
		return err
	}
	restName := "IT Listing MCP-page REST API " + restID
	restDefinition := map[string]any{
		"openapi": "3.0.3", "info": map[string]string{"title": restName, "version": "1.0.0"},
		"paths": map[string]any{"/ping": map[string]any{
			"get": map[string]any{"responses": map[string]any{"200": map[string]string{"description": "ok"}}},
		}},
	}
	metadata := map[string]any{
		"id": restID, "name": restName, "version": "v1.0", "type": "REST",
		"status": "PUBLISHED", "labels": []string{"default"}, "endPoints": endpoint(restID),
	}
	if err := u.createPortalArtifact(ctx, "/api-portal/api/v0.9/apis", metadata, restDefinition); err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalAPI, ID: restID, Actor: u.topo.Admin.Username}); err != nil {
		_ = u.deletePortalArtifact(ctx, "apis", restID)
		return fmt.Errorf("registering MCP listing REST API %q for cleanup: %w", restID, err)
	}
	if err := u.waitForPortalCards(ctx, "/mcps", firstID, secondID); err != nil {
		return err
	}
	if err := u.waitForPortalCards(ctx, "/apis", restID); err != nil {
		return err
	}
	for key, value := range map[string]string{
		keyAPIPortalMCPListingFirst: firstName, keyAPIPortalMCPListingSecond: secondName, keyAPIPortalMCPListingREST: restName,
	} {
		if err := tcontext.Set(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

func (u *Steps) mcpListingNames(ctx context.Context) (string, string, string, error) {
	values := make([]string, 0, 3)
	for _, key := range []string{keyAPIPortalMCPListingFirst, keyAPIPortalMCPListingSecond, keyAPIPortalMCPListingREST} {
		value, ok := tcontext.Get(ctx, key)
		if !ok {
			return "", "", "", fmt.Errorf("MCP listing fixture %q is unavailable", key)
		}
		name, ok := value.(string)
		if !ok || name == "" {
			return "", "", "", fmt.Errorf("MCP listing fixture %q has unexpected type %T", key, value)
		}
		values = append(values, name)
	}
	return values[0], values[1], values[2], nil
}

func (u *Steps) mcpListingShowsTypes(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	firstName, secondName, restName, err := u.mcpListingNames(ctx)
	if err != nil {
		return err
	}
	if err := u.openAPIPortalPath(ctx, "/mcps"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(".apilist-results-heading")).ToContainText("MCP Servers"); err != nil {
		return err
	}
	texts, err := page.Locator(".api-card").AllTextContents()
	if err != nil {
		return fmt.Errorf("reading MCP listing cards: %w", err)
	}
	for _, expected := range []string{firstName, secondName} {
		if !slices.ContainsFunc(texts, func(text string) bool { return strings.Contains(text, expected) }) {
			return fmt.Errorf("MCP listing did not show %q", expected)
		}
	}
	if slices.ContainsFunc(texts, func(text string) bool { return strings.Contains(text, restName) }) {
		return fmt.Errorf("MCP listing unexpectedly showed REST API %q", restName)
	}
	if count, err := page.Locator(".dp-badge--mcp").Count(); err != nil || count < 2 {
		return fmt.Errorf("MCP listing did not show MCP badges for both servers")
	}
	return nil
}

func (u *Steps) mcpListingSearchReturnsFirst(ctx context.Context) error {
	firstName, secondName, _, err := u.mcpListingNames(ctx)
	if err != nil {
		return err
	}
	return u.searchPortalListing(ctx, "/mcps", firstName, firstName, secondName)
}

func (u *Steps) mcpListingSearchExcludesREST(ctx context.Context) error {
	_, _, restName, err := u.mcpListingNames(ctx)
	if err != nil {
		return err
	}
	return u.searchPortalListing(ctx, "/mcps", restName, "", restName)
}

func (u *Steps) seedApplicationFixture(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	nameID, err := unique.Unique(ctx, "api-portal-application")
	if err != nil {
		return err
	}
	name := "IT Detail App " + nameID
	if err := u.openAPIPortalPath(ctx, "/applications"); err != nil {
		return err
	}
	if err := page.Locator("#apps-create-btn, #apps-create-btn-empty").First().Click(); err != nil {
		return fmt.Errorf("opening application creation: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#app-create-modal")).ToBeVisible(); err != nil {
		return err
	}
	if err := page.Locator("#app-name-input").Fill(name); err != nil {
		return fmt.Errorf("entering application name: %w", err)
	}
	if err := page.Locator("#app-desc-input").Fill("Created by the API Portal UI suite."); err != nil {
		return fmt.Errorf("entering application description: %w", err)
	}
	if err := page.Locator("#app-create-confirm").Click(); err != nil {
		return fmt.Errorf("creating application: %w", err)
	}
	card := page.Locator(".app-card").Filter(playwright.LocatorFilterOptions{
		HasText: regexp.MustCompile(regexp.QuoteMeta(name)),
	})
	if err := u.expect.Locator(card).ToBeVisible(); err != nil {
		return fmt.Errorf("waiting for created application: %w", err)
	}
	id, err := card.GetAttribute("data-id")
	if err != nil {
		return fmt.Errorf("reading created application: %w", err)
	}
	if id == "" {
		return fmt.Errorf("created application has no identifier")
	}
	applicationID := id
	if err := cleanup.Register(ctx, cleanup.Resource{Kind: KindAPIPortalApplication, ID: applicationID, Actor: u.topo.Admin.Username}); err != nil {
		_ = u.deletePortalArtifact(ctx, "applications", applicationID)
		return fmt.Errorf("registering API Portal application for cleanup: %w", err)
	}
	if err := tcontext.Set(ctx, keyAPIPortalApplication, applicationID); err != nil {
		_ = u.deletePortalArtifact(ctx, "applications", applicationID)
		return err
	}
	return tcontext.Set(ctx, keyAPIPortalApplicationName, name)
}

func (u *Steps) applicationDetails(ctx context.Context) (playwright.Page, string, string, error) {
	page, err := u.page(ctx)
	if err != nil {
		return nil, "", "", err
	}
	idValue, ok := tcontext.Get(ctx, keyAPIPortalApplication)
	if !ok {
		return nil, "", "", fmt.Errorf("API Portal application fixture is unavailable")
	}
	id, ok := idValue.(string)
	if !ok || id == "" {
		return nil, "", "", fmt.Errorf("API Portal application fixture has unexpected identifier type %T", idValue)
	}
	nameValue, ok := tcontext.Get(ctx, keyAPIPortalApplicationName)
	if !ok {
		return nil, "", "", fmt.Errorf("API Portal application fixture name is unavailable")
	}
	name, ok := nameValue.(string)
	if !ok || name == "" {
		return nil, "", "", fmt.Errorf("API Portal application fixture has unexpected name type %T", nameValue)
	}
	if err := u.openAPIPortalPath(ctx, "/applications/"+url.PathEscape(id)); err != nil {
		return nil, "", "", err
	}
	return page, id, name, nil
}

func (u *Steps) applicationCRUD(ctx context.Context) error {
	page, applicationID, name, err := u.applicationDetails(ctx)
	if err != nil {
		return err
	}
	renamed := name + " Renamed"
	if err := u.expect.Locator(page.Locator("#applicationName")).ToContainText(name); err != nil {
		return err
	}
	if err := page.Locator("#editNameBtn").Click(); err != nil {
		return fmt.Errorf("editing application name: %w", err)
	}
	if err := page.Locator("#applicationName").Fill(renamed); err != nil {
		return fmt.Errorf("entering renamed application: %w", err)
	}
	if err := page.Locator("#saveNameBtn").Click(); err != nil {
		return fmt.Errorf("saving application name: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#applicationName")).ToContainText(renamed); err != nil {
		return err
	}
	if err := page.Locator("#editDescriptionBtn").Click(); err != nil {
		return fmt.Errorf("editing application description: %w", err)
	}
	if err := page.Locator("#applicationDescription").Fill("Updated by the API Portal UI suite."); err != nil {
		return fmt.Errorf("entering application description: %w", err)
	}
	if _, err := page.ExpectNavigation(func() error {
		return page.Locator("#saveDescriptionBtn").Click()
	}, playwright.PageExpectNavigationOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return fmt.Errorf("saving application description: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#applicationDescription")).ToContainText("Updated by the API Portal UI suite."); err != nil {
		return err
	}
	if err := u.openAPIPortalPath(ctx, "/applications"); err != nil {
		return err
	}
	card := page.Locator(".app-card").Filter(playwright.LocatorFilterOptions{HasText: regexp.MustCompile(regexp.QuoteMeta(renamed))})
	if err := card.Locator(".app-delete-btn").Click(); err != nil {
		return fmt.Errorf("opening application deletion: %w", err)
	}
	if err := page.Locator("#app-delete-modal").WaitFor(playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateVisible}); err != nil {
		return fmt.Errorf("waiting for application deletion: %w", err)
	}
	// Confirming the deletion reloads the listing. Await that navigation so the
	// absence assertion below runs against the reloaded document rather than the
	// empty one the browser holds mid-navigation.
	if _, err := page.ExpectNavigation(func() error {
		return page.Locator("#app-delete-confirm").Click()
	}, playwright.PageExpectNavigationOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return fmt.Errorf("deleting application: %w", err)
	}
	if err := u.expect.Locator(page.Locator(".app-card").Filter(playwright.LocatorFilterOptions{
		HasText: regexp.MustCompile(regexp.QuoteMeta(renamed)),
	})).ToHaveCount(0); err != nil {
		return fmt.Errorf("confirming application deletion: %w", err)
	}
	reg, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	reg.Deregister(KindAPIPortalApplication, applicationID)
	return nil
}

// applicationHasKeyAssociationSections asserts what the application detail page
// does now: it associates keys that were created elsewhere.
//
// It used to assert a "Manage Keys" card per key manager, and an "unavailable"
// message when none was configured. Both are gone — key generation moved to its
// own page and a key manager is no longer bound to an application — so there is
// no longer a key-manager-dependent state on this page to assert.
func (u *Steps) applicationHasKeyAssociationSections(ctx context.Context) error {
	page, _, name, err := u.applicationDetails(ctx)
	if err != nil {
		return err
	}
	// Establish that this IS the detail page before asserting anything inside it.
	// Without this the step cannot tell "the key sections are missing" from "the
	// document is not the application detail page at all" — a blank or error
	// response fails on .ak-title with an empty snapshot, which reads as the
	// former and is the latter.
	if err := u.expect.Locator(page.Locator("#applicationName")).ToContainText(name); err != nil {
		return fmt.Errorf("application detail page did not render for %q (url %s): %w", name, page.URL(), err)
	}
	/*
	 * Both sections render their heading with the same .ak-title class, so that
	 * class on its own matches two elements. A Playwright attachment assertion
	 * resolves its locator strictly: two matches never resolve, and the failure
	 * surfaces as "expected to be attached" with a nil received value -- which
	 * reads as "the heading is missing" when the page in fact has two of them.
	 *
	 * So address each section by the text of its own heading, and each control
	 * by its own id. That also makes the assertion say what the scenario means:
	 * both association sections are present, not merely that something with this
	 * class is.
	 */
	for _, section := range []struct{ heading, control string }{
		{"API keys", "#btn-open-associate-key"},
		{"OAuth2 keys", "#btn-open-associate-oauth2-key"},
	} {
		heading := page.Locator(".ak-title").Filter(playwright.LocatorFilterOptions{
			HasText: regexp.MustCompile(regexp.QuoteMeta(section.heading)),
		})
		if err := u.expect.Locator(heading).ToBeAttached(); err != nil {
			return fmt.Errorf("%q section heading is missing on %s [%s]: %w",
				section.heading, page.URL(), describeKeySectionDOM(page), err)
		}
		if err := u.expect.Locator(page.Locator(section.control)).ToBeAttached(); err != nil {
			return fmt.Errorf("%q association control (%s) is missing on %s: %w",
				section.heading, section.control, page.URL(), err)
		}
	}
	return nil
}

// describeKeySectionDOM reports what the document actually holds when the key
// association assertion fails.
//
// The failure it exists for is contradictory on its face: browser coverage shows
// application-api-keys.js finding #application-api-keys-config on every load of
// this page, and .ak-title sits nine lines below that element in the same
// partial — so both are present or neither is. This prints the counts side by
// side, plus whether the page is still open, which separates "the section is
// genuinely missing" from "the assertion ran against a page that was going away".
func describeKeySectionDOM(page playwright.Page) string {
	if page.IsClosed() {
		return "page closed"
	}
	counts := map[string]any{}
	for _, sel := range []string{".ak-title", "#application-api-keys-config", "#applicationName", "section"} {
		if n, err := page.Locator(sel).Count(); err == nil {
			counts[sel] = n
		} else {
			counts[sel] = "count error: " + err.Error()
		}
	}
	body := -1
	if html, err := page.Content(); err == nil {
		body = len(html)
	}
	return fmt.Sprintf("open, html=%dB, .ak-title=%v, config-div=%v, #applicationName=%v, section=%v",
		body, counts[".ak-title"], counts["#application-api-keys-config"],
		counts["#applicationName"], counts["section"])
}

func (u *Steps) searchAPIListing(ctx context.Context, query, mustContain, mustNotContain string) error {
	return u.searchPortalListing(ctx, "/apis", query, mustContain, mustNotContain)
}

// waitForPortalCards waits for the portal's read-only listing to expose each
// scenario-owned artifact after its management API accepts creation.
func (u *Steps) waitForPortalCards(ctx context.Context, path string, ids ...string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.openAPIPortalPath(ctx, path); err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{}, func(ctx context.Context) (bool, error) {
		for _, id := range ids {
			count, err := page.Locator("#apiCard-" + id).Count()
			if err != nil {
				return false, retry.Transient(err)
			}
			if count == 0 {
				return false, nil
			}
		}
		return true, nil
	}, func(ready bool) bool { return ready }, fmt.Sprintf("waiting for API Portal listing %q to expose seeded artifacts", path))
}

func (u *Steps) searchPortalListing(ctx context.Context, path, query, mustContain, mustNotContain string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.openAPIPortalPath(ctx, path); err != nil {
		return err
	}
	if err := page.Locator("#query").Fill(query); err != nil {
		return fmt.Errorf("entering API listing search: %w", err)
	}
	if err := page.Locator("#query").Press("Enter"); err != nil {
		return fmt.Errorf("submitting API listing search: %w", err)
	}
	if err := u.expect.Page(page).ToHaveURL(regexp.MustCompile(`[?&]query=`)); err != nil {
		return err
	}
	if err := retry.Await(ctx, retry.Options{}, func(context.Context) (bool, error) {
		texts, err := page.Locator(".api-card").AllTextContents()
		if err != nil {
			return false, retry.Transient(err)
		}
		return portalListingMatches(texts, mustContain, mustNotContain), nil
	}, func(ready bool) bool { return ready }, fmt.Sprintf("waiting for filtered API Portal listing %q", query)); err != nil {
		return err
	}
	texts, err := page.Locator(".api-card").AllTextContents()
	if err != nil {
		return fmt.Errorf("reading filtered API listing cards: %w", err)
	}
	if mustContain != "" && !slices.ContainsFunc(texts, func(text string) bool { return strings.Contains(text, mustContain) }) {
		return fmt.Errorf("filtered API listing did not show %q", mustContain)
	}
	if mustNotContain != "" && slices.ContainsFunc(texts, func(text string) bool { return strings.Contains(text, mustNotContain) }) {
		return fmt.Errorf("filtered API listing unexpectedly showed %q", mustNotContain)
	}
	return nil
}

func portalListingMatches(texts []string, mustContain, mustNotContain string) bool {
	hasRequired := mustContain == "" || slices.ContainsFunc(texts, func(text string) bool {
		return strings.Contains(text, mustContain)
	})
	hasExcluded := mustNotContain != "" && slices.ContainsFunc(texts, func(text string) bool {
		return strings.Contains(text, mustNotContain)
	})
	return hasRequired && !hasExcluded
}

func (u *Steps) seededRESTAPIID(ctx context.Context) (string, error) {
	v, ok := tcontext.Get(ctx, keyAPIPortalDetailAPI)
	if !ok {
		return "", fmt.Errorf("seeded REST API details fixture is unavailable")
	}
	id, ok := v.(string)
	if !ok || id == "" {
		return "", fmt.Errorf("seeded REST API details fixture has unexpected type %T", v)
	}
	return id, nil
}

func (u *Steps) openRESTAPIDetailsPage(ctx context.Context, suffix string) (playwright.Page, error) {
	page, err := u.page(ctx)
	if err != nil {
		return nil, err
	}
	id, err := u.seededRESTAPIID(ctx)
	if err != nil {
		return nil, err
	}
	if err := u.openAPIPortalPath(ctx, "/api/"+id+suffix); err != nil {
		return nil, err
	}
	return page, nil
}

func (u *Steps) openRESTAPIOverview(ctx context.Context) error {
	page, err := u.openRESTAPIDetailsPage(ctx, "")
	if err != nil {
		return err
	}
	nameValue, ok := tcontext.Get(ctx, keyAPIPortalDetailAPIName)
	if !ok {
		return fmt.Errorf("seeded REST API details fixture name is unavailable")
	}
	apiName, ok := nameValue.(string)
	if !ok || apiName == "" {
		return fmt.Errorf("seeded REST API details fixture has unexpected name type %T", nameValue)
	}
	for selector, text := range map[string]string{
		".aov-hero-name":      apiName,
		"#token_api_prod_url": "backend.example.invalid",
		"#token_api_dev_url":  "sandbox.example.invalid",
		".aov-plan-card":      "Bronze",
	} {
		if err := u.expect.Locator(page.Locator(selector)).ToContainText(text); err != nil {
			return err
		}
	}
	for _, text := range []string{"Endpoints", "Resources", "Scopes"} {
		if err := u.expect.Locator(page.Locator(".aov-section-title").Filter(playwright.LocatorFilterOptions{HasText: regexp.MustCompile("^" + text + "$")})).ToBeAttached(); err != nil {
			return err
		}
	}
	if err := u.expect.Locator(page.Locator(".aov-plans-title")).ToContainText("Subscription plans"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(".dp-badge--version")).ToContainText("v1.0"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(".dp-badge--rest")).ToBeVisible(); err != nil {
		return err
	}
	if count, err := page.Locator(".aov-resource-row").Count(); err != nil || count < 2 {
		if err != nil {
			return fmt.Errorf("counting REST API resources: %w", err)
		}
		return fmt.Errorf("REST API details showed %d resources, want at least 2", count)
	}
	texts, err := page.Locator(".aov-resource-row").AllTextContents()
	if err != nil {
		return fmt.Errorf("reading REST API resources: %w", err)
	}
	if !slices.ContainsFunc(texts, func(text string) bool { return strings.Contains(text, "/items") }) {
		return fmt.Errorf("REST API details did not show /items resource")
	}
	if !slices.ContainsFunc(texts, func(text string) bool { return strings.Contains(text, "GET") }) {
		return fmt.Errorf("REST API details did not show GET resource method")
	}
	if !slices.ContainsFunc(texts, func(text string) bool { return strings.Contains(text, "POST") }) {
		return fmt.Errorf("REST API details did not show POST resource method")
	}
	return nil
}

func (u *Steps) openRESTAPISpecification(ctx context.Context) error {
	page, err := u.openRESTAPIDetailsPage(ctx, "")
	if err != nil {
		return err
	}
	if err := page.Locator(`a.dp-btn[href$="/docs/specification"]`).Click(); err != nil {
		return fmt.Errorf("opening REST API specification: %w", err)
	}
	if err := u.expect.Locator(page.Locator(".page-title")).ToContainText("Documentation"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(".adoc-file-badge--spec")).ToContainText("openapi"); err != nil {
		return err
	}
	elements := page.Locator("elements-api")
	if err := u.expect.Locator(elements).ToBeAttached(); err != nil {
		return err
	}
	for _, expected := range []string{"/items", "read:items", "write:items"} {
		value, err := elements.GetAttribute("apiDescriptionDocument")
		if err != nil {
			return fmt.Errorf("reading REST API specification: %w", err)
		}
		if !strings.Contains(value, expected) {
			return fmt.Errorf("REST API specification does not contain %q", expected)
		}
	}
	return nil
}

func (u *Steps) openRESTAPIDocumentation(ctx context.Context) error {
	page, err := u.openRESTAPIDetailsPage(ctx, "/docs/specification")
	if err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(".adoc-nav")).ToBeVisible(); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(".adoc-nav")).ToContainText("API Definition"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(".adoc-nav .adoc-nav-group-title").Filter(playwright.LocatorFilterOptions{HasText: regexp.MustCompile("^Other$")})).ToBeAttached(); err != nil {
		return err
	}
	doc := page.Locator(`.adoc-nav a.doc-link[href*="/docs/Other/"]`)
	if err := u.expect.Locator(doc).ToContainText("getting-started"); err != nil {
		return err
	}
	docHref, err := doc.GetAttribute("href")
	if err != nil {
		return fmt.Errorf("reading REST API additional document link: %w", err)
	}
	if strings.TrimSpace(docHref) == "" {
		return fmt.Errorf("REST API additional document link has no URL")
	}
	base, err := u.apiPortalURL()
	if err != nil {
		return err
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return fmt.Errorf("parsing API Portal URL: %w", err)
	}
	documentURL, err := url.Parse(docHref)
	if err != nil {
		return fmt.Errorf("parsing REST API additional document URL: %w", err)
	}
	docURL := baseURL.ResolveReference(documentURL).String()
	if err := retry.Await(ctx, retry.Options{}, func(context.Context) (bool, error) {
		response, err := page.Context().Request().Get(docURL)
		if err != nil {
			return false, err
		}
		body, bodyErr := response.Body()
		if bodyErr != nil {
			return false, bodyErr
		}
		return response.Status() >= 200 && response.Status() < 300 && strings.Contains(string(body), "Getting Started"), nil
	}, func(available bool) bool { return available }, "waiting for REST API additional document availability"); err != nil {
		return err
	}
	if err := doc.Click(); err != nil {
		return fmt.Errorf("opening REST API additional document: %w", err)
	}
	content := page.Locator(".api-markdown-content")
	if err := u.expect.Locator(content).ToContainText("Getting Started"); err != nil {
		return fmt.Errorf("%w%s", err, u.describeServedDocument(ctx, docURL))
	}
	return nil
}

func (u *Steps) describeServedDocument(ctx context.Context, docURL string) string {
	page, err := u.page(ctx)
	if err != nil {
		return ""
	}
	response, err := page.Context().Request().Get(docURL)
	if err != nil {
		return fmt.Sprintf("\nre-requesting %s for diagnosis failed: %v", docURL, err)
	}
	body, err := response.Body()
	if err != nil {
		return fmt.Sprintf("\nreading the re-requested %s failed: %v", docURL, err)
	}
	return fmt.Sprintf("\nserver returned HTTP %d for %s on re-request; body %s the document content and %s the file badge",
		response.Status(), docURL,
		containsWord(string(body), "Getting Started"),
		containsWord(string(body), "adoc-file-badge"))
}

func containsWord(body, needle string) string {
	if strings.Contains(body, needle) {
		return "contains"
	}
	return "omits"
}

func (u *Steps) restAPISpecificationExposesTryIt(ctx context.Context) error {
	page, err := u.openRESTAPIDetailsPage(ctx, "/docs/specification")
	if err != nil {
		return err
	}
	elements := page.Locator("elements-api")
	if err := u.expect.Locator(elements).ToBeAttached(); err != nil {
		return err
	}
	proxy, err := elements.GetAttribute("tryItCorsProxy")
	if err != nil {
		return fmt.Errorf("reading REST API Try It configuration: %w", err)
	}
	if proxy != "" {
		return fmt.Errorf("REST API Try It console unexpectedly configures a CORS proxy")
	}
	return nil
}

func (u *Steps) createPortalArtifact(ctx context.Context, path string, metadata, definition any, docs ...map[string]any) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("encoding API Portal fixture metadata: %w", err)
	}
	var definitionJSON []byte
	if definitionText, ok := definition.(string); ok {
		definitionJSON = []byte(definitionText)
	} else {
		definitionJSON, err = json.Marshal(definition)
		if err != nil {
			return fmt.Errorf("encoding API Portal fixture definition: %w", err)
		}
	}
	docsJSON, err := json.Marshal(docs)
	if err != nil {
		return fmt.Errorf("encoding API Portal fixture documents: %w", err)
	}
	result, err := page.Evaluate(`async ({path, metadataJSON, definitionJSON, docs}) => {
        const token = document.cookie.split('; ').find(v => v.startsWith('XSRF-TOKEN='));
        const form = new FormData();
        form.append('metadata', metadataJSON);
        form.append('definition', new Blob([definitionJSON], { type: 'application/json' }), 'definition.json');
        for (const doc of (JSON.parse(docs || '[]') || [])) {
            form.append('docs', new Blob([doc.content], { type: 'text/markdown' }), doc.name);
        }
        const response = await fetch(path, {
            method: 'POST',
            headers: { organization: 'default', 'X-CSRF-Token': token ? decodeURIComponent(token.split('=').slice(1).join('=')) : '' },
            body: form,
        });
        return { status: response.status, body: await response.text() };
    }`, map[string]any{
		"path":           path,
		"metadataJSON":   string(metadataJSON),
		"definitionJSON": string(definitionJSON),
		"docs":           string(docsJSON),
	})
	if err != nil {
		return fmt.Errorf("creating API Portal fixture: %w", err)
	}
	var response struct {
		Status int    `json:"status"`
		Body   string `json:"body"`
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("reading API Portal fixture response: %w", err)
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return fmt.Errorf("decoding API Portal fixture response: %w", err)
	}
	if response.Status < 200 || response.Status >= 300 {
		return fmt.Errorf("API Portal fixture creation returned HTTP status %d", response.Status)
	}
	return nil
}

func (u *Steps) deletePortalArtifact(ctx context.Context, collection, id string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	result, err := page.Evaluate(`async ({collection, id}) => {
            const token = document.cookie.split('; ').find(v => v.startsWith('XSRF-TOKEN='));
            const response = await fetch('/api-portal/api/v0.9/' + collection + '/' + encodeURIComponent(id), {
                method: 'DELETE',
                headers: { organization: 'default', 'X-CSRF-Token': token ? decodeURIComponent(token.split('=').slice(1).join('=')) : '' },
            });
			return { status: response.status };
		}`, map[string]any{"collection": collection, "id": id})
	if err != nil {
		return err
	}
	var response struct {
		Status int `json:"status"`
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return err
	}
	if response.Status != 404 && (response.Status < 200 || response.Status >= 300) {
		return fmt.Errorf("API Portal deletion returned HTTP status %d", response.Status)
	}
	return nil
}

func (u *Steps) browseAPIListingAndOpenSeededAPI(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	apiValue, ok := tcontext.Get(ctx, keyAPIPortalAPI)
	if !ok {
		return fmt.Errorf("API Portal portal-access API fixture is unavailable")
	}
	apiID, ok := apiValue.(string)
	if !ok || apiID == "" {
		return fmt.Errorf("API Portal portal-access API fixture has unexpected identifier type %T", apiValue)
	}
	if err := u.openAPIPortalPath(ctx, "/apis"); err != nil {
		return err
	}
	card := page.Locator("#apiCard-" + apiID)
	if err := u.expect.Locator(card).ToBeVisible(); err != nil {
		return fmt.Errorf("waiting for the seeded API listing card: %w", err)
	}
	return card.Locator(".api-card-top").Click()
}

func (u *Steps) browseMCPListing(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.openAPIPortalPath(ctx, "/mcps"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(".apilist-results-heading")).ToContainText("MCP Servers"); err != nil {
		return err
	}
	if count, err := page.Locator(".api-card").Count(); err != nil {
		return fmt.Errorf("counting MCP cards: %w", err)
	} else if count < 1 {
		return fmt.Errorf("the MCP listing did not show the seeded server")
	}
	return nil
}

func (u *Steps) openAPIWorkflows(ctx context.Context) error {
	return u.openAPIPortalPath(ctx, "/api-workflows")
}

func (u *Steps) openApplicationsSignedOut(ctx context.Context) error {
	if err := u.logoutAPIPortal(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.openAPIPortal(ctx); err != nil {
		return err
	}
	if err := page.Locator("#sidebar #applications").Click(); err != nil {
		return fmt.Errorf("opening Applications: %w", err)
	}
	return u.expect.Page(page).ToHaveURL(regexp.MustCompile(`/login`))
}

func (u *Steps) signInAndReturnToApplications(ctx context.Context) error {
	if err := u.submitAPIPortalLogin(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Locator(page.Locator(".page-title")).ToContainText("Applications")
}

func (u *Steps) openSeededAPIKeysSignedOut(ctx context.Context) error {
	if err := u.logoutAPIPortal(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	id, ok := tcontext.Get(ctx, keyAPIPortalAPI)
	if !ok {
		return fmt.Errorf("seeded API id is unavailable")
	}
	idString, ok := id.(string)
	if !ok || idString == "" {
		return fmt.Errorf("seeded API id has unexpected type %T", id)
	}
	if err := u.openAPIPortalPath(ctx, "/api/"+idString); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(".aov-hero-name")).ToBeVisible(); err != nil {
		return err
	}
	if err := page.Locator(`.aov-hero-actions a[href$="/api-keys"]`).Click(); err != nil {
		return fmt.Errorf("opening API keys: %w", err)
	}
	return u.expect.Page(page).ToHaveURL(regexp.MustCompile(`/login`))
}

func (u *Steps) signInAndReturnToSeededAPIKeys(ctx context.Context) error {
	if err := u.submitAPIPortalLogin(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Page(page).ToHaveURL(regexp.MustCompile(`/api/.+/api-keys`))
}

func (u *Steps) logoutAPIPortal(ctx context.Context) error {
	return u.openAPIPortalPath(ctx, "/logout")
}

func (u *Steps) submitAPIPortalLogin(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator("#username").Fill(u.topo.Admin.Username); err != nil {
		return err
	}
	if err := page.Locator("#password").Fill(u.topo.Admin.Password); err != nil {
		return err
	}
	return page.Locator(".ln-signin-btn").Click()
}

func (u *Steps) openAPIPortal(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	base, err := u.apiPortalURL()
	if err != nil {
		return err
	}
	portalURL := base + "/api-portal"
	if err := retry.Await(ctx, retry.Options{}, func(context.Context) (bool, error) {
		// The portal is an SPA. Waiting for every asset's load event makes the
		// navigation unnecessarily sensitive to unrelated concurrent requests.
		if _, err := page.Goto(portalURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		}); err != nil {
			return false, retry.Transient(fmt.Errorf("opening API Portal: %w", err))
		}
		visible, err := page.Locator("body").IsVisible()
		if err != nil {
			return false, retry.Transient(fmt.Errorf("checking API Portal shell: %w", err))
		}
		return visible, nil
	}, func(ready bool) bool { return ready }, "waiting for API Portal shell"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("body")).ToBeVisible(); err != nil {
		return fmt.Errorf("API Portal shell was not ready: %w", err)
	}
	return nil
}

func (u *Steps) openAPIPortalPath(ctx context.Context, path string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	base, err := u.apiPortalURL()
	if err != nil {
		return err
	}
	if _, err := page.Goto(base+"/api-portal/default/views/default"+path, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return fmt.Errorf("opening API Portal path %q: %w", path, err)
	}
	return nil
}

func (u *Steps) apiPortalPageIsVisible(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Locator(page.Locator("body")).ToBeVisible()
}

func (u *Steps) apiPortalPageDoesNotContain(ctx context.Context, unwanted string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Locator(page.Locator("body")).Not().ToContainText(unwanted)
}

func (u *Steps) apiPortalHeroIsVisible(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Locator(page.Locator(".hero")).ToBeVisible()
}

func (u *Steps) apiPortalAPIDetailIsVisible(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Page(page).ToHaveURL(regexp.MustCompile(`/api/[^/?]+$`)); err != nil {
		return err
	}
	return u.expect.Locator(page.Locator(".aov-hero-name")).ToBeVisible()
}

func (u *Steps) apiPortalMCPListingIsVisible(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(".apilist-results-heading")).ToContainText("MCP Servers"); err != nil {
		return err
	}
	count, err := page.Locator(".api-card").Count()
	if err != nil {
		return fmt.Errorf("counting MCP cards: %w", err)
	}
	if count < 1 {
		return fmt.Errorf("the MCP listing did not show the seeded server")
	}
	return nil
}

func (u *Steps) apiPortalLoginFormIsVisible(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Locator(page.Locator("#local-login-form")).ToBeVisible()
}

func (u *Steps) apiPortalApplicationsIsVisible(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator(".page-title")).ToContainText("Applications"); err != nil {
		return err
	}
	return u.expect.Locator(page.Locator("#local-login-form")).ToHaveCount(0)
}

func (u *Steps) apiPortalAPIKeysIsVisible(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := u.expect.Page(page).ToHaveURL(regexp.MustCompile(`/api/.+/api-keys`)); err != nil {
		return err
	}
	return u.expect.Locator(page.Locator("#local-login-form")).ToHaveCount(0)
}

func (u *Steps) sidebarIsCollapsedByDefault(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	sidebar := page.Locator("#sidebar")
	if err := u.expect.Locator(sidebar).ToBeVisible(); err != nil {
		return err
	}
	if err := u.expect.Locator(sidebar).Not().ToContainClass("expanded"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#collapseBtn .collapse-text")).ToContainText("Expand"); err != nil {
		return err
	}
	for _, selector := range []string{"#home", "#apis", "#mcps", "#api-workflows", "#applications"} {
		if err := u.expect.Locator(sidebar.Locator(selector)).ToBeAttached(); err != nil {
			return fmt.Errorf("API Portal sidebar is missing %s: %w", selector, err)
		}
	}
	width, err := sidebar.Evaluate("el => el.getBoundingClientRect().width", nil)
	if err != nil {
		return fmt.Errorf("reading collapsed API Portal sidebar width: %w", err)
	}
	var widthValue float64
	switch value := width.(type) {
	case float64:
		widthValue = value
	case int:
		widthValue = float64(value)
	default:
		return fmt.Errorf("API Portal sidebar width has unexpected type %T", width)
	}
	if widthValue >= 140 {
		return fmt.Errorf("API Portal sidebar width = %v, want less than 140", width)
	}
	return nil
}

func (u *Steps) sidebarPersistsAcrossNavigation(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator("#collapseBtn").Click(); err != nil {
		return fmt.Errorf("expanding API Portal sidebar: %w", err)
	}
	if err := u.expect.Locator(page.Locator("#sidebar")).ToContainClass("expanded"); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#collapseBtn .collapse-text")).ToContainText("Collapse"); err != nil {
		return err
	}
	if err := page.Locator("#sidebar #apis").Click(); err != nil {
		return fmt.Errorf("navigating to API listing: %w", err)
	}
	if err := u.expect.Page(page).ToHaveURL(regexp.MustCompile(`/apis`)); err != nil {
		return err
	}
	if err := u.expect.Locator(page.Locator("#sidebar")).ToContainClass("expanded"); err != nil {
		return err
	}
	if _, err := page.Reload(); err != nil {
		return fmt.Errorf("reloading API listing: %w", err)
	}
	return u.expect.Locator(page.Locator("#sidebar")).ToContainClass("expanded")
}

func (u *Steps) collapseSidebar(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator("#collapseBtn").Click(); err != nil {
		return fmt.Errorf("collapsing API Portal sidebar: %w", err)
	}
	return nil
}

func (u *Steps) sidebarIsForceCollapsed(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	sidebar := page.Locator("#sidebar")
	if err := u.expect.Locator(sidebar).Not().ToContainClass("expanded"); err != nil {
		return err
	}
	if err := u.expect.Locator(sidebar).ToContainClass("force-collapse"); err != nil {
		return err
	}
	return u.expect.Locator(page.Locator("#collapseBtn .collapse-text")).ToContainText("Expand")
}

func (u *Steps) apiPortalURL() (string, error) {
	inst, err := u.topo.Component("api-portal")
	if err != nil {
		return "", err
	}
	return inst.InternalURL("http")
}

func (u *Steps) requestAPIPortalPath(ctx context.Context, path string) error {
	return u.requestAPIPortal(ctx, path)
}

func (u *Steps) requestAPIPortalPathWithoutFollowingRedirects(ctx context.Context, path string) error {
	return u.requestAPIPortalWithClient(ctx, path, u.noRedirect)
}

func (u *Steps) requestAPIPortal(ctx context.Context, path string) error {
	return u.requestAPIPortalWithClient(ctx, path, u.client)
}

func (u *Steps) requestAPIPortalWithClient(ctx context.Context, path string, client *httpx.Client) error {
	base, err := u.topo.URL("api-portal", "http")
	if err != nil {
		return err
	}
	// Clear the prior response before issuing the request so a failed request
	// cannot be mistaken for a successful assertion against stale state.
	if err := tcontext.Set(ctx, keyAPIPortalResponse, (*httpx.Response)(nil)); err != nil {
		return err
	}
	resp, err := client.Do(ctx, httpx.Request{Method: http.MethodGet, URL: base + path}, 0, 0)
	if err != nil {
		return fmt.Errorf("requesting API Portal path %q: %w", path, err)
	}
	return tcontext.Set(ctx, keyAPIPortalResponse, resp)
}

func (u *Steps) apiPortalResponse(ctx context.Context) (*httpx.Response, error) {
	v, ok := tcontext.Get(ctx, keyAPIPortalResponse)
	if !ok {
		return nil, fmt.Errorf("no API Portal response is available")
	}
	resp, ok := v.(*httpx.Response)
	if !ok || resp == nil {
		return nil, fmt.Errorf("API Portal response has unexpected type %T", v)
	}
	return resp, nil
}

func (u *Steps) apiPortalResponseStatus(ctx context.Context, expected string) error {
	resp, err := u.apiPortalResponse(ctx)
	if err != nil {
		return err
	}
	want, err := strconv.Atoi(expected)
	if err != nil {
		return fmt.Errorf("invalid expected HTTP status %q: %w", expected, err)
	}
	if got := resp.StatusCode; got != want {
		return fmt.Errorf("API Portal response status = %d, want %d", got, want)
	}
	return nil
}

func (u *Steps) apiPortalResponseStatusOneOf(ctx context.Context, first, second, third string) error {
	resp, err := u.apiPortalResponse(ctx)
	if err != nil {
		return err
	}
	got := resp.StatusCode
	for _, candidate := range []string{first, second, third} {
		want, err := strconv.Atoi(candidate)
		if err != nil {
			return fmt.Errorf("invalid expected HTTP status %q: %w", candidate, err)
		}
		if got == want {
			return nil
		}
	}
	return fmt.Errorf("API Portal response status = %d, want one of %s, %s, or %s", got, first, second, third)
}

func (u *Steps) apiPortalRedirectContains(ctx context.Context, expected string) error {
	resp, err := u.apiPortalResponse(ctx)
	if err != nil {
		return err
	}
	location := resp.Headers.Get("Location")
	if !strings.Contains(location, expected) {
		return fmt.Errorf("API Portal redirect %q does not contain %q", location, expected)
	}
	return nil
}

func (u *Steps) apiPortalResponseContentTypeContains(ctx context.Context, expected string) error {
	resp, err := u.apiPortalResponse(ctx)
	if err != nil {
		return err
	}
	contentType := resp.Headers.Get("Content-Type")
	if !strings.Contains(contentType, expected) {
		return fmt.Errorf("API Portal content type %q does not contain %q", contentType, expected)
	}
	return nil
}

func (u *Steps) apiPortalResponseBodyContains(ctx context.Context, expected string) error {
	return u.apiPortalResponseBodyMatch(ctx, expected, true)
}

func (u *Steps) apiPortalResponseJSONStatus(ctx context.Context, expected string) error {
	resp, err := u.apiPortalResponse(ctx)
	if err != nil {
		return err
	}
	body := resp.Body
	var document struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return fmt.Errorf("decoding API Portal response body: %w", err)
	}
	if document.Status != expected {
		return fmt.Errorf("API Portal response JSON status = %q, want %q", document.Status, expected)
	}
	return nil
}

func (u *Steps) apiPortalResponseBodyDoesNotContain(ctx context.Context, expected string) error {
	return u.apiPortalResponseBodyMatch(ctx, expected, false)
}

func (u *Steps) apiPortalResponseBodyMatch(ctx context.Context, expected string, want bool) error {
	resp, err := u.apiPortalResponse(ctx)
	if err != nil {
		return err
	}
	body := resp.Body
	contains := strings.Contains(string(body), expected)
	if contains != want {
		return fmt.Errorf("API Portal response body containment for %q = %t, want %t", expected, contains, want)
	}
	return nil
}
