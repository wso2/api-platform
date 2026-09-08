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

package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// stubsMCPServerValidationSuccess fulfils every fetch-server-info probe for the rest of the
// scenario with a fixed, successful validation result, so a scenario never depends on a
// real MCP server being reachable at whatever endpoint it types in.
func (u *UI) stubsMCPServerValidationSuccess(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return page.Route("**/fetch-server-info*", func(route playwright.Route) {
		_ = route.Fulfill(playwright.RouteFulfillOptions{
			Status:      playwright.Int(200),
			ContentType: playwright.String("application/json"),
			Body: `{"serverInfo":{"name":"Stub MCP Server","version":"1.0.0"},` +
				`"tools":[],"resources":[],"prompts":[]}`,
		})
	})
}

// fillsMCPProxyEndpointAndAuth types the endpoint URL, and — when both are non-empty —
// expands the Advanced Configurations panel and types the auth header name and value.
func fillsMCPProxyEndpointAndAuth(page playwright.Page, endpointURL, authHeader, authValue string) error {
	if err := page.Locator(`input[placeholder="Enter URL of Your MCP Proxy"]`).Fill(endpointURL); err != nil {
		return fmt.Errorf("typing the endpoint URL: %w", err)
	}
	if authHeader == "" || authValue == "" {
		return nil
	}
	if err := page.GetByText("Advanced Configurations").Click(); err != nil {
		return fmt.Errorf("expanding Advanced Configurations: %w", err)
	}
	if err := page.Locator(`input[placeholder="Header"]`).Fill(authHeader); err != nil {
		return fmt.Errorf("typing the auth header: %w", err)
	}
	if err := page.Locator(`input[placeholder="Value"]`).Fill(authValue); err != nil {
		return fmt.Errorf("typing the auth value: %w", err)
	}
	return nil
}

// submitsMCPProxyWithCredential fills and submits the MCP proxy create form with an
// explicit auth header and value, recording the /secrets and /mcp-proxies calls it makes.
func (u *UI) submitsMCPProxyWithCredential(ctx context.Context, name, endpointURL, authHeader, authValue string) error {
	if err := u.watchSecretAndProviderCalls(ctx); err != nil {
		return err
	}
	if err := u.stubsMCPServerValidationSuccess(ctx); err != nil {
		return err
	}
	if err := u.startsCreatingMCPProxy(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := fillsMCPProxyEndpointAndAuth(page, endpointURL, authHeader, authValue); err != nil {
		return err
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: "Fetch Server Info"}).Click(); err != nil {
		return fmt.Errorf("fetching server info: %w", err)
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Next"}).Click(
		playwright.LocatorClickOptions{Timeout: playwright.Float(60000)},
	); err != nil {
		return fmt.Errorf("advancing past validation: %w", err)
	}
	if err := page.Locator(`input[placeholder="WSO2 MCP Proxy"]`).Fill(name); err != nil {
		return fmt.Errorf("typing the proxy name: %w", err)
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: "Create", Exact: playwright.Bool(true)}).Click(); err != nil {
		return fmt.Errorf("submitting the MCP proxy: %w", err)
	}
	id := toProviderID(name)
	if err := tcontext.Set(ctx, keyLatestMCPProxyID, id); err != nil {
		return err
	}
	// Registered optimistically: a submission the backend rejects registers a proxy that
	// never existed, and the deleter treats that as already-gone rather than an error.
	return cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindMCPServer, ID: id, Actor: u.topo.Admin.Username,
	})
}

const keyLatestMCPProxyID = "uiLatestMCPProxyID"

// latestMCPProxyID is the id of the most recently created MCP proxy in this scenario.
func (u *UI) latestMCPProxyID(ctx context.Context) (string, error) {
	v, ok := tcontext.Get(ctx, keyLatestMCPProxyID)
	if !ok {
		return "", fmt.Errorf("no MCP proxy id in scope — no create step ran")
	}
	id, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("MCP proxy id in scope has unexpected type %T", v)
	}
	return id, nil
}

// submitsMCPProxyWithCredentialPlaceholder is submitsMCPProxyWithCredential, with the auth
// value built as a placeholder referencing an existing secret handle.
func (u *UI) submitsMCPProxyWithCredentialPlaceholder(ctx context.Context, name, endpointURL, authHeader, handle string) error {
	return u.submitsMCPProxyWithCredential(ctx, name, endpointURL, authHeader, secretPlaceholder(handle))
}

// submitsMCPProxyWithoutCredential is submitsMCPProxyWithCredential, leaving the auth
// fields untouched.
func (u *UI) submitsMCPProxyWithoutCredential(ctx context.Context, name, endpointURL string) error {
	return u.submitsMCPProxyWithCredential(ctx, name, endpointURL, "", "")
}

// validatesMCPProxyEndpoint fills the endpoint URL and auth fields and clicks Fetch Server
// Info, stopping once the probe resolves rather than advancing to Next — for checking what
// happens during validation alone, without creating a proxy.
func (u *UI) validatesMCPProxyEndpoint(ctx context.Context, endpointURL, authHeader, authValue string) error {
	if err := u.watchSecretAndProviderCalls(ctx); err != nil {
		return err
	}
	if err := u.stubsMCPServerValidationSuccess(ctx); err != nil {
		return err
	}
	if err := u.startsCreatingMCPProxy(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := fillsMCPProxyEndpointAndAuth(page, endpointURL, authHeader, authValue); err != nil {
		return err
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: "Fetch Server Info"}).Click(); err != nil {
		return fmt.Errorf("fetching server info: %w", err)
	}
	// The Next button only renders once the probe resolves; waiting for it confirms the
	// validation round trip completed without advancing past it.
	return u.expect.Locator(page.GetByRole("button",
		playwright.PageGetByRoleOptions{Name: "Next"})).ToBeVisible()
}

// --- assertions on recorded calls ---

// mcpProxyAuthValue decodes the credential value an MCP proxy create/update request body
// carries. The path matches a provider's own upstream.main.auth.value shape exactly.
func mcpProxyAuthValue(call recordedCall) (string, error) {
	return providerAuthValue(call)
}

// mcpProxyCallCarriesAPlaceholder asserts the most recent request of the given method
// carries a secret placeholder and never the plaintext credential.
func (u *UI) mcpProxyCallCarriesAPlaceholder(ctx context.Context, method, plaintext string) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	call, err := t.awaitMCPProxyCall(ctx, method)
	if err != nil {
		return fmt.Errorf("no %s request to /mcp-proxies was recorded: %w", method, err)
	}
	authValue, err := mcpProxyAuthValue(call)
	if err != nil {
		return err
	}
	if !strings.Contains(authValue, `{{ secret "`) {
		return fmt.Errorf("the credential carries no secret placeholder: %s", authValue)
	}
	if strings.Contains(authValue, plaintext) {
		return fmt.Errorf("the credential still carries the plaintext value")
	}
	return nil
}

func (u *UI) theMCPProxyWasCreatedWithAPlaceholder(ctx context.Context, plaintext string) error {
	return u.mcpProxyCallCarriesAPlaceholder(ctx, "POST", plaintext)
}

func (u *UI) theMCPProxyWasUpdatedWithAPlaceholder(ctx context.Context, plaintext string) error {
	return u.mcpProxyCallCarriesAPlaceholder(ctx, "PUT", plaintext)
}

// theMCPProxyUpdateCarriesTheURL asserts a policy-save or connection-save PUT carries the
// given upstream endpoint URL.
func (u *UI) theMCPProxyUpdateCarriesTheURL(ctx context.Context, url string) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	call, err := t.awaitMCPProxyCall(ctx, "PUT")
	if err != nil {
		return fmt.Errorf("no PUT request to /mcp-proxies was recorded: %w", err)
	}
	var body struct {
		Upstream struct {
			Main struct {
				URL string `json:"url"`
			} `json:"main"`
		} `json:"upstream"`
	}
	if err := json.Unmarshal([]byte(call.body), &body); err != nil {
		return fmt.Errorf("parsing the request body: %w", err)
	}
	if body.Upstream.Main.URL != url {
		return fmt.Errorf("upstream url = %q, want %q", body.Upstream.Main.URL, url)
	}
	return nil
}

func (u *UI) noMCPProxyWasCreated(ctx context.Context) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	if n := t.mcpProxyCount("POST"); n != 0 {
		return fmt.Errorf("expected no MCP proxy creation request, got %d", n)
	}
	return nil
}

// mcpProxyCallHasNoAuthBlock asserts the most recent request of the given method carries
// no upstream.main.auth field at all, matching the form's own conditional payload
// construction.
func (u *UI) mcpProxyCallHasNoAuthBlock(ctx context.Context, method string) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	call, err := t.awaitMCPProxyCall(ctx, method)
	if err != nil {
		return fmt.Errorf("no %s request to /mcp-proxies was recorded: %w", method, err)
	}
	var body struct {
		Upstream struct {
			Main struct {
				Auth *struct {
					Value string `json:"value"`
				} `json:"auth"`
			} `json:"main"`
		} `json:"upstream"`
	}
	if err := json.Unmarshal([]byte(call.body), &body); err != nil {
		return fmt.Errorf("parsing the request body: %w", err)
	}
	if body.Upstream.Main.Auth != nil {
		return fmt.Errorf("expected no auth block, got one referencing %q", body.Upstream.Main.Auth.Value)
	}
	return nil
}

// theMCPProxyWasCreatedWithoutAnAuthBlock asserts the create request's upstream.main config
// carries no auth field at all, matching the form's own conditional payload construction.
func (u *UI) theMCPProxyWasCreatedWithoutAnAuthBlock(ctx context.Context) error {
	return u.mcpProxyCallHasNoAuthBlock(ctx, "POST")
}

// theMCPProxyUpdateHadNoAuthBlock asserts a policy-save PUT carries no auth field at all,
// matching how a proxy created without a credential renders no auth block to preserve.
func (u *UI) theMCPProxyUpdateHadNoAuthBlock(ctx context.Context) error {
	return u.mcpProxyCallHasNoAuthBlock(ctx, "PUT")
}

// secretHandleUUID matches the shape crypto.randomUUID() produces.
var secretHandleUUID = regexp.MustCompile(
	`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// theSecretHandleIsARandomUUID asserts the most recently created secret's handle is a
// random UUID rather than a slug derived from any name typed during the scenario.
func (u *UI) theSecretHandleIsARandomUUID(ctx context.Context) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	handle, ok := t.lastSecretHandle()
	if !ok {
		return fmt.Errorf("no secret handle recorded in this scenario")
	}
	if !secretHandleUUID.MatchString(handle) {
		return fmt.Errorf("secret handle %q is not a random UUID", handle)
	}
	return nil
}

// --- policy-save update flow ---

// opensMCPProxyPoliciesTab switches the MCP proxy overview to its Policies tab.
func (u *UI) opensMCPProxyPoliciesTab(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByRole("tab", playwright.PageGetByRoleOptions{Name: "Policies"}).Click(); err != nil {
		return fmt.Errorf("opening the Policies tab: %w", err)
	}
	return nil
}

// addsACORSPolicyAndSaves attaches the CORS policy and saves, recording the /secrets and
// /mcp-proxies calls the save makes. Save stays disabled until the policy list actually
// changes, so attaching one policy is what enables it.
func (u *UI) addsACORSPolicyAndSaves(ctx context.Context) error {
	if err := u.watchSecretAndProviderCalls(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Add Policies"}).Click(); err != nil {
		return fmt.Errorf("opening the policy picker: %w", err)
	}
	if err := page.GetByText("CORS").First().Click(); err != nil {
		return fmt.Errorf("selecting the CORS policy: %w", err)
	}
	if err := page.Locator(`[data-testid="policy-param-submit"]`).Click(); err != nil {
		return fmt.Errorf("submitting the policy's parameters: %w", err)
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: "Save", Exact: playwright.Bool(true)}).Click(); err != nil {
		return fmt.Errorf("saving the policy list: %w", err)
	}
	return nil
}

// theMCPProxyUpdateKeptTheAuthHeaderAndType asserts a policy-save PUT preserves the auth
// block's header and type while carrying no value — auth.value is write-only and never
// present in the client's locally cached proxy, so a save that re-sends that object
// legitimately omits it without losing the stored credential server-side.
func (u *UI) theMCPProxyUpdateKeptTheAuthHeaderAndType(ctx context.Context, header, authType string) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	call, err := t.awaitMCPProxyCall(ctx, "PUT")
	if err != nil {
		return fmt.Errorf("no PUT request to /mcp-proxies was recorded: %w", err)
	}
	var body struct {
		Upstream struct {
			Main struct {
				Auth *struct {
					Type   string `json:"type"`
					Header string `json:"header"`
					Value  string `json:"value"`
				} `json:"auth"`
			} `json:"main"`
		} `json:"upstream"`
	}
	if err := json.Unmarshal([]byte(call.body), &body); err != nil {
		return fmt.Errorf("parsing the request body: %w", err)
	}
	if body.Upstream.Main.Auth == nil {
		return fmt.Errorf("expected an auth block, got none")
	}
	if body.Upstream.Main.Auth.Header != header {
		return fmt.Errorf("auth header = %q, want %q", body.Upstream.Main.Auth.Header, header)
	}
	if body.Upstream.Main.Auth.Type != authType {
		return fmt.Errorf("auth type = %q, want %q", body.Upstream.Main.Auth.Type, authType)
	}
	if body.Upstream.Main.Auth.Value != "" {
		return fmt.Errorf("expected the auth value to be omitted, got %q", body.Upstream.Main.Auth.Value)
	}
	return nil
}

// theMCPProxyUpdateBodyDoesNotInclude asserts the most recent PUT request body, re-encoded
// to a canonical form, does not contain the given substring — re-encoding first avoids a
// false negative from the raw body's own JSON-escaped quoting.
func (u *UI) theMCPProxyUpdateBodyDoesNotInclude(ctx context.Context, substring string) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	call, err := t.awaitMCPProxyCall(ctx, "PUT")
	if err != nil {
		return fmt.Errorf("no PUT request to /mcp-proxies was recorded: %w", err)
	}
	var parsed any
	if err := json.Unmarshal([]byte(call.body), &parsed); err != nil {
		return fmt.Errorf("parsing the request body: %w", err)
	}
	canonical, err := json.Marshal(parsed)
	if err != nil {
		return fmt.Errorf("re-encoding the request body: %w", err)
	}
	if strings.Contains(string(canonical), substring) {
		return fmt.Errorf("the PUT body unexpectedly includes %q", substring)
	}
	return nil
}
