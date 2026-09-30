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

package aiworkspace

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

const keyMCPCapturePath = "uiMCPCapturePath"

// mcpCaptureUpstream returns an MCP proxy upstream URL on the testbench capture service and
// records the path that service will store the gateway's request under. The path is unique to
// the proxy, so a scenario reads back only the requests its own proxy forwarded.
//
// The URL ends in /mcp because the block's gateway forwards an MCP proxy's /mcp route to the
// upstream URL as configured, rather than appending the request path to it.
func (u *Steps) mcpCaptureUpstream(ctx context.Context, proxyID string) (string, error) {
	base, err := u.captureBaseURL()
	if err != nil {
		return "", err
	}
	capturePath := "/mcp-upstream-auth/" + proxyID + "/mcp"
	if err := tcontext.Set(ctx, keyMCPCapturePath, capturePath); err != nil {
		return "", err
	}
	return base + capturePath, nil
}

// captureBaseURL is the testbench capture service's address on the block's network, including
// the block partition the service keys its records by.
func (u *Steps) captureBaseURL() (string, error) {
	inst, err := u.topo.Component("testbench")
	if err != nil {
		return "", err
	}
	base, err := inst.InternalURL("capture")
	if err != nil {
		return "", err
	}
	if u.topo.Block == nil {
		return "", fmt.Errorf("the capture service requires a block partition, but the topology has no block")
	}
	return base + "/" + u.topo.Block.PartitionKey(), nil
}

// seedsLegacyMCPProxy stores an MCP proxy through platform-api with the upstream auth type
// "header" that older AI Workspace releases wrote, in the scenario's latest project. The
// current UI never writes that type, so the proxy is created through the API.
func (u *Steps) seedsLegacyMCPProxy(ctx context.Context, name, authHeader, secretHandle string) error {
	name, err := expandUIValue(ctx, name)
	if err != nil {
		return err
	}
	if secretHandle, err = secretHandleFor(ctx, secretHandle); err != nil {
		return err
	}
	projectID, err := u.latestProjectID(ctx)
	if err != nil {
		return err
	}
	id := toProviderID(name)
	upstreamURL, err := u.mcpCaptureUpstream(ctx, id)
	if err != nil {
		return err
	}
	page, base, token, err := u.platformAPI(ctx)
	if err != nil {
		return err
	}
	resp, err := page.Context().Request().Post(base+"/api/v0.9/mcp-proxies",
		playwright.APIRequestContextPostOptions{
			Headers: map[string]string{"Authorization": "Bearer " + token},
			Data: map[string]any{
				"id":             id,
				"displayName":    name,
				"version":        "v1.0",
				"projectId":      projectID,
				"context":        "/" + id,
				"mcpSpecVersion": "2025-06-18",
				"upstream": map[string]any{"main": map[string]any{
					"url": upstreamURL,
					"auth": map[string]any{
						"type":   "header",
						"header": authHeader,
						"value":  fmt.Sprintf("{{ secret %q }}", secretHandle),
					},
				}},
			},
			IgnoreHttpsErrors: playwright.Bool(true),
		})
	if err != nil {
		return fmt.Errorf("creating the legacy MCP proxy %q: %w", name, err)
	}
	if status := resp.Status(); status != http.StatusCreated && status != http.StatusOK {
		body, _ := resp.Text()
		return fmt.Errorf("creating the legacy MCP proxy %q returned %d: %s", name, status, body)
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindMCPServer, ID: id, Actor: u.topo.Admin.Username,
	}); err != nil {
		if _, delErr := page.Context().Request().Delete(base+"/api/v0.9/mcp-proxies/"+id,
			playwright.APIRequestContextDeleteOptions{
				Headers:           map[string]string{"Authorization": "Bearer " + token},
				IgnoreHttpsErrors: playwright.Bool(true),
			}); delErr != nil {
			return fmt.Errorf("registering MCP proxy %q for cleanup: %w; compensating delete failed: %v", id, err, delErr)
		}
		return fmt.Errorf("registering MCP proxy %q for cleanup: %w", id, err)
	}
	return tcontext.Set(ctx, keyLatestMCPProxyID, id)
}

// submitsMCPProxyAtCaptureWithCredential creates an MCP proxy through the UI form, pointed at
// the testbench capture service, with an explicit auth header and value.
func (u *Steps) submitsMCPProxyAtCaptureWithCredential(ctx context.Context, name, authHeader, authValue string) error {
	name, err := expandUIValue(ctx, name)
	if err != nil {
		return err
	}
	upstreamURL, err := u.mcpCaptureUpstream(ctx, toProviderID(name))
	if err != nil {
		return err
	}
	return u.submitsMCPProxyWithCredential(ctx, name, upstreamURL, authHeader, authValue)
}

// opensMCPProxy opens the named proxy from the current project's MCP Proxies list.
func (u *Steps) opensMCPProxy(ctx context.Context, name string) error {
	name, err := expandUIValue(ctx, name)
	if err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	if err := page.GetByText(name, playwright.PageGetByTextOptions{Exact: playwright.Bool(true)}).First().Click(); err != nil {
		return fmt.Errorf("opening the MCP proxy %q: %w", name, err)
	}
	return nil
}

// invokesMCPProxyThroughGateway sends an MCP initialize request to the scenario's latest MCP
// proxy on the block's gateway, polling until the deployment serves it.
func (u *Steps) invokesMCPProxyThroughGateway(ctx context.Context) error {
	id, err := u.latestMCPProxyID(ctx)
	if err != nil {
		return err
	}
	page, base, token, err := u.platformAPI(ctx)
	if err != nil {
		return err
	}
	resp, err := page.Context().Request().Get(base+"/api/v0.9/mcp-proxies/"+id,
		playwright.APIRequestContextGetOptions{
			Headers:           map[string]string{"Authorization": "Bearer " + token},
			IgnoreHttpsErrors: playwright.Bool(true),
		})
	if err != nil {
		return fmt.Errorf("reading MCP proxy %q: %w", id, err)
	}
	var proxy struct {
		Context string `json:"context"`
	}
	if err := resp.JSON(&proxy); err != nil {
		return fmt.Errorf("reading MCP proxy %q: %w", id, err)
	}
	if proxy.Context == "" {
		return fmt.Errorf("MCP proxy %q has no context", id)
	}
	inst, err := u.topo.Component("platform-gateway")
	if err != nil {
		return err
	}
	gatewayURL, err := inst.InternalURL("http")
	if err != nil {
		return err
	}
	invokeURL := gatewayURL + strings.TrimSuffix(proxy.Context, "/") + "/mcp"
	inv, err := retry.Until(ctx, retry.Options{}, func(context.Context) (invocation, error) {
		resp, err := page.Context().Request().Post(invokeURL, playwright.APIRequestContextPostOptions{
			Headers: map[string]string{"Accept": "application/json, text/event-stream"},
			Data: map[string]any{
				"jsonrpc": "2.0", "id": 1, "method": "initialize",
				"params": map[string]any{
					"protocolVersion": "2025-06-18",
					"capabilities":    map[string]any{},
					"clientInfo":      map[string]any{"name": "ui-suite", "version": "1.0.0"},
				},
			},
		})
		if err != nil {
			return invocation{}, err
		}
		body, _ := resp.Body()
		return invocation{status: resp.Status(), body: string(body)}, nil
	}, func(inv invocation) bool { return inv.status == http.StatusOK })
	if err != nil {
		return fmt.Errorf("invoking %s: %w", invokeURL, err)
	}
	return tcontext.Set(ctx, keyInvocation, inv)
}

// mcpUpstreamReceivedHeader asserts the capture service recorded the scenario's MCP request
// with the given header and value — proof the gateway injected the proxy's upstream credential.
func (u *Steps) mcpUpstreamReceivedHeader(ctx context.Context, header, value string) error {
	v, ok := tcontext.Get(ctx, keyMCPCapturePath)
	if !ok {
		return fmt.Errorf("no capture path in scope — no MCP proxy was pointed at the capture service")
	}
	capturePath, ok := v.(string)
	if !ok {
		return fmt.Errorf("capture path in scope has unexpected type %T", v)
	}
	base, err := u.captureBaseURL()
	if err != nil {
		return err
	}
	page, err := u.page(ctx)
	if err != nil {
		return err
	}
	resp, err := page.Context().Request().Get(base + "/test/captured?path=" + url.QueryEscape(capturePath))
	if err != nil {
		return fmt.Errorf("reading the captured upstream request: %w", err)
	}
	if resp.Status() != http.StatusOK {
		return fmt.Errorf("the upstream received no request at %s (capture returned %d)", capturePath, resp.Status())
	}
	var captured struct {
		Headers map[string][]string `json:"headers"`
	}
	if err := resp.JSON(&captured); err != nil {
		return fmt.Errorf("reading the captured upstream request: %w", err)
	}
	got := http.Header(captured.Headers).Values(header)
	for _, g := range got {
		if g == value {
			return nil
		}
	}
	if len(got) == 0 {
		return fmt.Errorf("the upstream request carried no %q header", header)
	}
	return fmt.Errorf("the upstream request's %q header did not carry the expected credential", header)
}
