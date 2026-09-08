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

	playwright "github.com/mxschmitt/playwright-go"
)

// opensMCPProxyBackendConnectionTab switches the MCP proxy overview to its Backend
// Connection tab and waits for the endpoint field to render, confirming the proxy's stored
// connection details have loaded.
func (u *UI) opensMCPProxyBackendConnectionTab(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByRole("tab", playwright.PageGetByRoleOptions{
		Name: "Backend Connection"}).Click(); err != nil {
		return fmt.Errorf("opening the Backend Connection tab: %w", err)
	}
	return u.expect.Locator(page.Locator(`[data-testid="backend-connection-endpoint-url"]`)).ToBeVisible()
}

// theBackendConnectionAuthValueFieldShowsTheMaskedSentinel asserts the auth value field
// displays the masked placeholder rather than a real credential — the value is write-only
// server-side, so this is the only thing the field can ever legitimately show on load.
func (u *UI) theBackendConnectionAuthValueFieldShowsTheMaskedSentinel(ctx context.Context) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	return u.expect.Locator(page.Locator(`[data-testid="backend-connection-auth-value"]`)).ToHaveValue("******")
}

func (u *UI) editsBackendConnectionURL(ctx context.Context, newURL string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator(`[data-testid="backend-connection-endpoint-url"]`).Fill(newURL); err != nil {
		return fmt.Errorf("editing the endpoint URL: %w", err)
	}
	return nil
}

func (u *UI) editsBackendConnectionAuthHeader(ctx context.Context, newHeader string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator(`[data-testid="backend-connection-auth-header"]`).Fill(newHeader); err != nil {
		return fmt.Errorf("editing the auth header: %w", err)
	}
	return nil
}

// editsBackendConnectionAuthValue focuses the masked value field before filling it, mirroring
// the product's own flow for clearing the sentinel to type a live credential.
func (u *UI) editsBackendConnectionAuthValue(ctx context.Context, newValue string) error {
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	field := page.Locator(`[data-testid="backend-connection-auth-value"]`)
	if err := field.Click(); err != nil {
		return fmt.Errorf("focusing the auth value field: %w", err)
	}
	if err := field.Fill(newValue); err != nil {
		return fmt.Errorf("editing the auth value: %w", err)
	}
	return nil
}

// clicksRefetchServerInfo triggers the Backend Connection tab's own validation probe,
// recording the /mcp-proxies/fetch-server-info request it makes.
func (u *UI) clicksRefetchServerInfo(ctx context.Context) error {
	if err := u.watchSecretAndProviderCalls(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.Locator(`[data-testid="backend-connection-refetch"]`).Click(); err != nil {
		return fmt.Errorf("clicking Refetch Server Info: %w", err)
	}
	return nil
}

// savesBackendConnection clicks the Backend Connection tab's Save button, recording the
// /secrets and /mcp-proxies calls the save makes.
func (u *UI) savesBackendConnection(ctx context.Context) error {
	if err := u.watchSecretAndProviderCalls(ctx); err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: "Save", Exact: playwright.Bool(true)}).Click(); err != nil {
		return fmt.Errorf("saving the backend connection: %w", err)
	}
	return nil
}

// fetchServerInfoRequest decodes a POST /mcp-proxies/fetch-server-info request body into
// its three mutually exclusive shapes: proxyId alone, url [+ proxyId], or url + auth.
type fetchServerInfoRequest struct {
	ProxyID string `json:"proxyId"`
	URL     string `json:"url"`
	Auth    *struct {
		Type   string `json:"type"`
		Header string `json:"header"`
		Value  string `json:"value"`
	} `json:"auth"`
}

func decodeFetchServerInfoRequest(call recordedCall) (fetchServerInfoRequest, error) {
	var req fetchServerInfoRequest
	if err := json.Unmarshal([]byte(call.body), &req); err != nil {
		return req, fmt.Errorf("parsing the request body: %w", err)
	}
	return req, nil
}

// theRefetchRequestUsedOnlyTheStoredProxy asserts the most recent fetch-server-info request
// carries only the proxy's own id, letting the backend resolve the stored url and
// credential itself.
func (u *UI) theRefetchRequestUsedOnlyTheStoredProxy(ctx context.Context) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	call, err := t.awaitFetchServerInfoCall(ctx)
	if err != nil {
		return fmt.Errorf("no fetch-server-info request was recorded: %w", err)
	}
	req, err := decodeFetchServerInfoRequest(call)
	if err != nil {
		return err
	}
	id, err := u.latestMCPProxyID(ctx)
	if err != nil {
		return err
	}
	if req.ProxyID != id {
		return fmt.Errorf("proxyId = %q, want %q", req.ProxyID, id)
	}
	if req.URL != "" {
		return fmt.Errorf("expected url to be omitted, got %q", req.URL)
	}
	if req.Auth != nil {
		return fmt.Errorf("expected no auth override, got one")
	}
	return nil
}

// theRefetchRequestSentTheLiveCredential asserts the most recent fetch-server-info request
// validates the live, unsaved url and credential directly, omitting proxyId.
func (u *UI) theRefetchRequestSentTheLiveCredential(ctx context.Context, url, header, value string) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	call, err := t.awaitFetchServerInfoCall(ctx)
	if err != nil {
		return fmt.Errorf("no fetch-server-info request was recorded: %w", err)
	}
	req, err := decodeFetchServerInfoRequest(call)
	if err != nil {
		return err
	}
	if req.ProxyID != "" {
		return fmt.Errorf("expected proxyId to be omitted, got %q", req.ProxyID)
	}
	if req.URL != url {
		return fmt.Errorf("url = %q, want %q", req.URL, url)
	}
	if req.Auth == nil {
		return fmt.Errorf("expected an auth override, got none")
	}
	if req.Auth.Type != "header" {
		return fmt.Errorf("auth type = %q, want %q", req.Auth.Type, "header")
	}
	if req.Auth.Header != header {
		return fmt.Errorf("auth header = %q, want %q", req.Auth.Header, header)
	}
	if req.Auth.Value != value {
		return fmt.Errorf("auth value did not match the expected credential")
	}
	return nil
}

// theRefetchRequestUsedTheEditedURLAndStoredProxy asserts the most recent fetch-server-info
// request validates an unsaved endpoint edit while still letting the backend supply the
// stored credential via proxyId.
func (u *UI) theRefetchRequestUsedTheEditedURLAndStoredProxy(ctx context.Context, url string) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	call, err := t.awaitFetchServerInfoCall(ctx)
	if err != nil {
		return fmt.Errorf("no fetch-server-info request was recorded: %w", err)
	}
	req, err := decodeFetchServerInfoRequest(call)
	if err != nil {
		return err
	}
	id, err := u.latestMCPProxyID(ctx)
	if err != nil {
		return err
	}
	if req.URL != url {
		return fmt.Errorf("url = %q, want %q", req.URL, url)
	}
	if req.ProxyID != id {
		return fmt.Errorf("proxyId = %q, want %q", req.ProxyID, id)
	}
	if req.Auth != nil {
		return fmt.Errorf("expected no auth override, got one")
	}
	return nil
}
