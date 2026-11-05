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
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/catalog/platformapi"
	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
	"github.com/wso2/api-platform/tests/framework/suites/ui/steps/uivalue"
)

// sessionCookie is the BFF's session cookie. It holds the control-plane token itself, so a
// token the control plane issued for the second organization is that organization's session.
const sessionCookie = "_api_control_plane_session"

// secondOrganizationAdmin names the administrator of the control plane's second organization
// in features.
const secondOrganizationAdmin = "org B admin"

// Cleanup kinds for resources the second organization owns, which only it can delete.
var (
	KindSecondOrganizationRESTAPI = cleanup.Kind{Name: "management-portal-second-org-rest-api", Order: cleanup.KindAPI.Order}
	KindSecondOrganizationGateway = cleanup.Kind{Name: "management-portal-second-org-gateway", Order: cleanup.KindGateway.Order}
)

// Steps holds the Management Portal UI bindings.
type Steps struct {
	topo       *frameworkruntime.Topology
	expect     playwright.PlaywrightAssertions
	pageGetter func(context.Context) (playwright.Page, error)
	client     *httpx.Client
}

// New constructs the Management Portal bindings over the suite's scenario page.
func New(topo *frameworkruntime.Topology, expect playwright.PlaywrightAssertions,
	pageGetter func(context.Context) (playwright.Page, error)) *Steps {
	return &Steps{
		topo: topo, expect: expect, pageGetter: pageGetter,
		client: httpx.NewClient(httpx.Options{MaxRetries: 1, TLSClientConfig: controlPlaneTLS()}),
	}
}

// Register wires the Management Portal steps into the scenario context.
func (s *Steps) Register(sc *godog.ScenarioContext) {
	sc.Step(`^I generate a unique handle from "([^"]*)" and store it as "([^"]*)"$`, generateHandle)
	sc.Step(`^a project "([^"]*)" exists on the control plane$`, s.projectExists)
	sc.Step(`^a REST API "([^"]*)" exists in project "([^"]*)" on the control plane(?: for "([^"]*)")?$`, s.restAPIExists)
	sc.Step(`^a gateway "([^"]*)" is registered on the control plane(?: for "([^"]*)")?$`, s.gatewayIsRegistered)
	sc.Step(`^the user signs in to the Management Portal as the administrator$`, s.signInAsAdministrator)
	sc.Step(`^the user signs in to the Management Portal as "([^"]*)"$`, s.signInAs)
	sc.Step(`^the user opens the Management Portal page "([^"]*)"$`, s.openPage)
	sc.Step(`^the Management Portal shows "([^"]*)"$`, s.shows)
	sc.Step(`^the Management Portal does not show "([^"]*)"$`, s.doesNotShow)
	sc.Step(`^the Management Portal is not on a page under "([^"]*)"$`, s.notUnder)
}

// generateHandle stores a unique name the control plane accepts as a handle: lowercase
// letters, digits and hyphens.
func generateHandle(ctx context.Context, base, key string) error {
	local, ok := tcontext.LocalOf(ctx)
	if !ok || local == nil {
		return fmt.Errorf("cannot store generated handle %q without runner context", key)
	}
	value, err := unique.Unique(ctx, base)
	if err != nil {
		return err
	}
	local.Set(key, strings.ToLower(strings.ReplaceAll(value, "_", "-")))
	return nil
}

func controlPlaneTLS() *tls.Config {
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	roots.AppendCertsFromPEM(shared.ControlPlaneCrypto()["certs/cert.pem"])
	return &tls.Config{RootCAs: roots, ServerName: "platform-api"}
}

func (s *Steps) portalURL(path string) (string, error) {
	inst, err := s.topo.Component("api-control-plane")
	if err != nil {
		return "", err
	}
	base, err := inst.InternalURL("http")
	if err != nil {
		return "", err
	}
	return base + path, nil
}

// bearer returns a control-plane token for the administrator, or for the second
// organization's administrator, registering that organization on first use.
func (s *Steps) bearer(ctx context.Context, who string) (string, error) {
	if who != "" && who != "admin" && who != secondOrganizationAdmin {
		return "", fmt.Errorf("unknown control-plane actor %q: supported actors are admin and %s", who, secondOrganizationAdmin)
	}
	base, err := s.topo.URL("platform-api", "https")
	if err != nil {
		return "", err
	}
	if who == secondOrganizationAdmin {
		return platformapi.SecondOrganizationLogin(ctx, base)
	}
	return platformapi.ControlPlaneLogin(ctx, base, s.topo.Admin.Username, s.topo.Admin.Password)
}

// create posts a control-plane resource as who and registers it for cleanup, removing it
// again when registration fails.
func (s *Steps) create(ctx context.Context, who, collection string, body map[string]any, kind cleanup.Kind) error {
	bearer, err := s.bearer(ctx, who)
	if err != nil {
		return err
	}
	base, err := s.topo.URL("platform-api", "https")
	if err != nil {
		return err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	headers := map[string]string{"Authorization": "Bearer " + bearer}
	resp, err := s.client.Do(ctx, httpx.Request{
		Method: http.MethodPost, URL: base + "/api/v0.9" + collection, Headers: headers,
		ContentType: "application/json", Body: payload,
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("creating %s: %w", collection, err)
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("creating %s: %s", collection, resp.Describe())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp.Body, &created); err != nil || created.ID == "" {
		return fmt.Errorf("creating %s returned no id: %s", collection, resp.Describe())
	}
	actor := who
	if actor == "" || actor == "admin" {
		actor = s.topo.Admin.Username
	}
	if err := cleanup.Register(ctx, cleanup.Resource{Kind: kind, ID: created.ID, Actor: actor}); err != nil {
		_, _ = s.client.Do(ctx, httpx.Request{
			Method: http.MethodDelete, URL: base + "/api/v0.9" + collection + "/" + created.ID, Headers: headers,
		}, 0, 0)
		return err
	}
	return nil
}

func (s *Steps) projectExists(ctx context.Context, handle string) error {
	resolved, err := uivalue.Expand(ctx, handle)
	if err != nil {
		return err
	}
	return s.create(ctx, "admin", "/projects",
		map[string]any{"id": resolved, "displayName": resolved}, cleanup.KindProject)
}

func (s *Steps) restAPIExists(ctx context.Context, name, project, who string) error {
	resolved, err := uivalue.Expand(ctx, name)
	if err != nil {
		return err
	}
	resolvedProject, err := uivalue.Expand(ctx, project)
	if err != nil {
		return err
	}
	kind := cleanup.KindAPI
	if who == secondOrganizationAdmin {
		kind = KindSecondOrganizationRESTAPI
	}
	return s.create(ctx, who, "/rest-apis", map[string]any{
		"displayName": resolved,
		"context":     "/" + resolved,
		"version":     "v1",
		"projectId":   resolvedProject,
		"upstream":    map[string]any{"main": map[string]any{"url": "http://testbench:3000"}},
	}, kind)
}

func (s *Steps) gatewayIsRegistered(ctx context.Context, name, who string) error {
	resolved, err := uivalue.Expand(ctx, name)
	if err != nil {
		return err
	}
	kind := cleanup.KindGateway
	if who == secondOrganizationAdmin {
		kind = KindSecondOrganizationGateway
	}
	return s.create(ctx, who, "/gateways", map[string]any{
		"id":                resolved,
		"displayName":       resolved,
		"endpoints":         []string{"http://" + resolved + ".example.com"},
		"functionalityType": "regular",
	}, kind)
}

// signInAsAdministrator drives the real sign-in form, starting from a signed-out browser.
func (s *Steps) signInAsAdministrator(ctx context.Context) error {
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	if err := page.Context().ClearCookies(); err != nil {
		return fmt.Errorf("signing out: %w", err)
	}
	if err := s.openPage(ctx, "/login"); err != nil {
		return err
	}
	if err := page.Locator("#login-username").Fill(s.topo.Admin.Username); err != nil {
		return fmt.Errorf("typing the username: %w", err)
	}
	if err := page.Locator("#login-password").Fill(s.topo.Admin.Password); err != nil {
		return fmt.Errorf("typing the password: %w", err)
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Sign in"}).Click(); err != nil {
		return fmt.Errorf("clicking Sign in: %w", err)
	}
	if err := s.expect.Page(page).ToHaveURL(regexp.MustCompile(`/organizations(/|$)`)); err != nil {
		return fmt.Errorf("signing in never reached the organizations: %w", err)
	}
	return nil
}

// signInAs gives the browser the second organization's session. The file-based login knows a
// single organization, so the session holds a token the control plane issued for the second
// one; the control plane verifies it on every request the console makes.
func (s *Steps) signInAs(ctx context.Context, who string) error {
	if who != secondOrganizationAdmin {
		return fmt.Errorf("unknown Management Portal user %q: use the administrator or %s", who, secondOrganizationAdmin)
	}
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	token, err := s.bearer(ctx, who)
	if err != nil {
		return err
	}
	portal, err := s.portalURL("")
	if err != nil {
		return err
	}
	if err := page.Context().ClearCookies(); err != nil {
		return fmt.Errorf("signing out: %w", err)
	}
	if err := page.Context().AddCookies([]playwright.OptionalCookie{{
		Name: sessionCookie, Value: token, URL: playwright.String(portal),
		HttpOnly: playwright.Bool(true), SameSite: playwright.SameSiteAttributeLax,
	}}); err != nil {
		return fmt.Errorf("signing in as %s: %w", who, err)
	}
	return nil
}

func (s *Steps) openPage(ctx context.Context, path string) error {
	resolved, err := uivalue.Expand(ctx, path)
	if err != nil {
		return err
	}
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	url, err := s.portalURL(resolved)
	if err != nil {
		return err
	}
	if _, err := page.Goto(url); err != nil {
		return fmt.Errorf("opening %s: %w", url, err)
	}
	return nil
}

func (s *Steps) shows(ctx context.Context, text string) error {
	resolved, err := uivalue.Expand(ctx, text)
	if err != nil {
		return err
	}
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	if err := s.expect.Locator(page.GetByText(resolved).First()).ToBeVisible(); err != nil {
		return fmt.Errorf("the Management Portal never showed %q: %w", resolved, err)
	}
	return nil
}

// doesNotShow asserts the text is absent. Features check it after the page has shown what
// it loaded, so absence is not mistaken for a page still loading.
func (s *Steps) doesNotShow(ctx context.Context, text string) error {
	resolved, err := uivalue.Expand(ctx, text)
	if err != nil {
		return err
	}
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	if err := s.expect.Locator(page.GetByText(resolved)).ToHaveCount(0); err != nil {
		return fmt.Errorf("the Management Portal shows %q: %w", resolved, err)
	}
	return nil
}

func (s *Steps) notUnder(ctx context.Context, path string) error {
	resolved, err := uivalue.Expand(ctx, path)
	if err != nil {
		return err
	}
	page, err := s.pageGetter(ctx)
	if err != nil {
		return err
	}
	prefix, err := s.portalURL(strings.TrimSuffix(resolved, "/"))
	if err != nil {
		return err
	}
	under := regexp.MustCompile("^" + regexp.QuoteMeta(prefix) + "(/|$)")
	if err := s.expect.Page(page).Not().ToHaveURL(under); err != nil {
		return fmt.Errorf("the Management Portal stayed under %s: %w", resolved, err)
	}
	return nil
}
