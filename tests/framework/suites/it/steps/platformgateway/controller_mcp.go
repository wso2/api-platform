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

package platformgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// controllerMCPProtocolVersion is the MCP revision the gateway controller's endpoints implement.
// It is sessionless: every request carries the protocol version and client identity itself, in
// both the request headers and params._meta, instead of relying on an initialize handshake.
const controllerMCPProtocolVersion = "2026-07-28"

// controllerMCPPath is the MCP endpoint path beneath a controller service's API base path.
const controllerMCPPath = "/mcp"

const (
	toolDeployAPI   = "wso2_apip_gw_deploy_api"
	toolUndeployAPI = "wso2_apip_gw_undeploy_api"
)

func (g *Gateway) registerControllerMCPSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I send an MCP "([^"]*)" request to the "([^"]*)" service$`, g.sendControllerMCPMethod)
	sc.Step(`^I call the MCP tool "([^"]*)" on the "([^"]*)" service with arguments:$`,
		g.callControllerMCPTool)
	sc.Step(`^I deploy API through MCP from "([^"]*)" with values:$`, g.deployAPIThroughMCP)
	sc.Step(`^I update API "([^"]*)" through MCP from "([^"]*)" with values:$`, g.updateAPIThroughMCP)
	sc.Step(`^I undeploy API "([^"]*)" through MCP$`, g.undeployAPIThroughMCP)
}

// sendControllerMCPMethod sends one parameterless JSON-RPC request, such as tools/list.
func (g *Gateway) sendControllerMCPMethod(ctx context.Context, method, service string) error {
	_, err := g.controllerMCPRequest(ctx, service, method, map[string]any{})
	return err
}

// callControllerMCPTool calls one tool with the JSON object arguments given in the docstring.
func (g *Gateway) callControllerMCPTool(
	ctx context.Context, tool, service string, arguments *godog.DocString,
) error {
	expanded, err := stepscommon.Expand(ctx, arguments.Content)
	if err != nil {
		return err
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(expanded), &args); err != nil {
		return fmt.Errorf("MCP tool arguments must be a JSON object: %w", err)
	}
	_, err = g.controllerMCPToolCall(ctx, service, tool, args)
	return err
}

// deployAPIThroughMCP renders the canonical API template and creates the API through the
// management MCP endpoint's deploy tool.
//
// Like createAPI, a rejected deploy is published for the scenario to assert rather than failed
// here. An accepted one is registered for cleanup before anything else can fail, then awaited.
// A tool error arrives as HTTP 200 with result.isError, so success is judged on the JSON-RPC
// result, not the status code.
func (g *Gateway) deployAPIThroughMCP(ctx context.Context, templateName string, table *godog.Table) error {
	definition, err := g.renderMCPTemplate(ctx, templateName, table)
	if err != nil {
		return err
	}
	resp, err := g.controllerMCPToolCall(ctx, "gateway-controller", toolDeployAPI,
		map[string]any{"kind": "RestApi", "yaml": definition})
	if err != nil {
		return err
	}
	if !mcpToolSucceeded(resp) {
		return nil
	}

	name := apiNameFrom(definition)
	if name == "" {
		return nil
	}
	expectInDump(ctx, name, true)
	if err := tcontext.Set(ctx, keyLastAPIName, name); err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindAPI, ID: name, Actor: "admin", Description: "deployed through MCP by " + scenarioLabel(ctx),
	}); err != nil {
		deleteURL, urlErr := g.managementURL("/rest-apis/" + name)
		if urlErr == nil {
			if deleteErr := g.compensateDelete(ctx, deleteURL, g.scenarioHeaders(ctx)); deleteErr != nil {
				return fmt.Errorf("registering API %q for cleanup: %w; compensation failed: %v",
					name, err, deleteErr)
			}
		}
		return fmt.Errorf("registering API %q for cleanup: %w", name, err)
	}
	return g.awaitDeployed(ctx, name)
}

// updateAPIThroughMCP replaces an existing API through the deploy tool's update form.
func (g *Gateway) updateAPIThroughMCP(
	ctx context.Context, apiName, templateName string, table *godog.Table,
) error {
	name, err := stepscommon.Expand(ctx, apiName)
	if err != nil {
		return err
	}
	definition, err := g.renderMCPTemplate(ctx, templateName, table)
	if err != nil {
		return err
	}
	_, err = g.controllerMCPToolCall(ctx, "gateway-controller", toolDeployAPI,
		map[string]any{"kind": "RestApi", "yaml": definition, "id": name})
	return err
}

// undeployAPIThroughMCP removes an API through the undeploy tool. A successful undeploy is
// deregistered so teardown does not attempt it again.
func (g *Gateway) undeployAPIThroughMCP(ctx context.Context, apiName string) error {
	name, err := stepscommon.Expand(ctx, apiName)
	if err != nil {
		return err
	}
	resp, err := g.controllerMCPToolCall(ctx, "gateway-controller", toolUndeployAPI,
		map[string]any{"kind": "RestApi", "id": name, "confirm": true})
	if err != nil {
		return err
	}
	if mcpToolSucceeded(resp) {
		if reg, err := cleanup.Of(ctx); err == nil {
			reg.Deregister(cleanup.KindAPI, name)
		}
		expectInDump(ctx, name, false)
	}
	return nil
}

// renderMCPTemplate renders a canonical API template for an MCP manifest argument.
func (g *Gateway) renderMCPTemplate(ctx context.Context, templateName string, table *godog.Table) (string, error) {
	path, err := g.templatePath(templateName)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read resource template %q: %w", templateName, err)
	}
	definition, err := g.renderResourceTemplate(ctx, "API", templateName, content, table)
	if err != nil {
		return "", fmt.Errorf("resource template %q: %w", templateName, err)
	}
	return definition, nil
}

// controllerMCPToolCall sends a tools/call request for one tool.
func (g *Gateway) controllerMCPToolCall(
	ctx context.Context, service, tool string, arguments map[string]any,
) (*httpx.Response, error) {
	return g.controllerMCPRequest(ctx, service, "tools/call", map[string]any{
		"name": tool, "arguments": arguments,
	})
}

// controllerMCPRequest sends one JSON-RPC request to a controller service's MCP endpoint through
// the funnel and publishes the response with an SSE body reduced to its JSON-RPC message.
func (g *Gateway) controllerMCPRequest(
	ctx context.Context, service, method string, params map[string]any,
) (*httpx.Response, error) {
	url, err := g.serviceURL(ctx, service, controllerMCPPath)
	if err != nil {
		return nil, err
	}
	body, err := controllerMCPRequestBody(method, params)
	if err != nil {
		return nil, err
	}
	tool, _ := params["name"].(string)
	resp, err := g.funnel.Send(ctx, httpx.Request{
		Method:  http.MethodPost,
		URL:     url,
		Headers: controllerMCPHeaders(g.scenarioHeaders(ctx), method, tool),
		Body:    body,
	})
	if err != nil {
		return nil, err
	}
	decodeMCPStream(resp)
	return resp, g.funnel.Publish(ctx, resp)
}

// controllerMCPRequestBody builds a JSON-RPC request carrying the protocol envelope in
// params._meta. The caller's params are copied, never modified.
func controllerMCPRequestBody(method string, params map[string]any) ([]byte, error) {
	if strings.TrimSpace(method) == "" {
		return nil, fmt.Errorf("an MCP request needs a JSON-RPC method")
	}
	withMeta := make(map[string]any, len(params)+1)
	for k, v := range params {
		withMeta[k] = v
	}
	withMeta["_meta"] = map[string]any{
		"io.modelcontextprotocol/protocolVersion":    controllerMCPProtocolVersion,
		"io.modelcontextprotocol/clientInfo":         map[string]any{"name": "framework-client", "version": "1.0.0"},
		"io.modelcontextprotocol/clientCapabilities": map[string]any{},
	}
	return json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": withMeta})
}

// controllerMCPHeaders returns the scenario headers plus the transport headers the protocol
// requires. Mcp-Name is required for tools/call and absent otherwise.
func controllerMCPHeaders(scenario map[string]string, method, tool string) map[string]string {
	headers := make(map[string]string, len(scenario)+5)
	for k, v := range scenario {
		headers[k] = v
	}
	headers["Content-Type"] = "application/json"
	headers["Accept"] = "application/json, text/event-stream"
	headers["MCP-Protocol-Version"] = controllerMCPProtocolVersion
	headers["Mcp-Method"] = method
	if method == "tools/call" && tool != "" {
		headers["Mcp-Name"] = tool
	}
	return headers
}

// decodeMCPStream replaces an SSE body with the JSON-RPC message it carries. Only an SSE
// response is touched, so a JSON error body is never reinterpreted; status and headers are kept
// exactly as the server sent them.
func decodeMCPStream(resp *httpx.Response) {
	if resp == nil || !strings.HasPrefix(strings.ToLower(resp.Headers.Get("Content-Type")), "text/event-stream") {
		return
	}
	resp.Body = mcpJSONPayload(resp.Body)
}

// mcpToolSucceeded reports whether a decoded response is a successful tool result: HTTP 2xx, a
// JSON-RPC result rather than an error, and no isError flag.
func mcpToolSucceeded(resp *httpx.Response) bool {
	if !resp.Succeeded() {
		return false
	}
	var message struct {
		Result *struct {
			IsError bool `json:"isError"`
		} `json:"result"`
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(resp.Body, &message); err != nil {
		return false
	}
	return message.Result != nil && !message.Result.IsError && len(message.Error) == 0
}
