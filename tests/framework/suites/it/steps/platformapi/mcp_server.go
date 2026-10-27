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

package platformapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// mcpPath is the Platform API's MCP endpoint, under apiBase.
const mcpPath = "/mcp"

// mcpAccept is what an MCP client sends, as the Streamable HTTP transport asks.
const mcpAccept = "application/json, text/event-stream"

// mcpCollections maps the apiType a tool takes to its control-plane collection.
var mcpCollections = map[string]string{
	"rest-api":     "rest-apis",
	"llm-provider": "llm-providers",
	"llm-proxy":    "llm-proxies",
	"mcp-proxy":    "mcp-proxies",
}

// mcpReply is the part of a JSON-RPC answer the steps read.
type mcpReply struct {
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
	Result *struct {
		IsError           bool `json:"isError"`
		StructuredContent struct {
			ID          string `json:"id"`
			Deployments []struct {
				ID string `json:"id"`
			} `json:"deployments"`
		} `json:"structuredContent"`
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	} `json:"result"`
}

// mcpToolArguments is the part of a tool call's arguments the cleanup tracking reads.
type mcpToolArguments struct {
	APIType   string `json:"apiType"`
	APIID     string `json:"apiId"`
	ProjectID string `json:"projectId"`
	GatewayID string `json:"gatewayId"`
}

// RegisterMCPServer binds the steps that call the Platform API's MCP tools as a signed-in user.
//
// Each call is one stateless request through the control-plane funnel, so the answer is
// published for the shared assertions. A resource a tool creates is registered for cleanup from
// the tool's result, because an MCP call answers 200 with no Location header.
func RegisterMCPServer(sc *godog.ScenarioContext, s *Steps) {
	sc.Step(`^I call the MCP tool "([^"]*)"(?: as "([^"]*)")?(?: with header "([^"]*)" set to "([^"]*)")?$`,
		func(ctx context.Context, tool, who, header, value string) error {
			return s.callMCPToolStep(ctx, tool, who, header, value, nil)
		})
	sc.Step(`^I call the MCP tool "([^"]*)"(?: as "([^"]*)")? with arguments:$`,
		func(ctx context.Context, tool, who string, arguments *godog.DocString) error {
			return s.callMCPToolStep(ctx, tool, who, "", "", arguments)
		})
	sc.Step(`^I call the MCP tool "([^"]*)"(?: as "([^"]*)")? until the JSON field "([^"]*)" is "([^"]*)" with arguments:$`,
		s.pollMCPToolField)
	sc.Step(`^I list the MCP tools(?: as "([^"]*)")?$`, s.listMCPToolsStep)
	sc.Step(`^I list the MCP tools with the token "([^"]*)"$`, s.listMCPToolsWithToken)
	sc.Step(`^the MCP tool call should succeed$`, s.mcpToolCallSucceeded)
	sc.Step(`^the MCP tool call should fail with "([^"]*)"$`, s.mcpToolCallFailed)
	sc.Step(`^the MCP tools should be "([^"]*)"$`, s.mcpToolsAre)
}

func (s *Steps) callMCPToolStep(
	ctx context.Context, tool, who, header, value string, arguments *godog.DocString,
) error {
	var headers map[string]string
	if strings.TrimSpace(header) != "" {
		headers = map[string]string{header: value}
	}
	content := ""
	if arguments != nil {
		content = arguments.Content
	}
	_, err := s.callMCPTool(ctx, tool, who, headers, content)
	return err
}

// pollMCPToolField repeats a tool call until a field of its answer holds the wanted value,
// publishing the matching answer. Deployment status is acknowledged by the gateway after the
// deploy call returns.
func (s *Steps) pollMCPToolField(
	ctx context.Context, tool, who, field, want string, arguments *godog.DocString,
) error {
	expected, err := stepscommon.Expand(ctx, want)
	if err != nil {
		return err
	}
	last := "no response"
	err = retry.Await(ctx, retry.Options{Interval: time.Second},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, callErr := s.callMCPTool(ctx, tool, who, nil, arguments.Content)
			if callErr != nil {
				return nil, retry.Transient(callErr)
			}
			last = resp.Describe()
			return resp, nil
		},
		func(resp *httpx.Response) bool { return jsonFieldEquals(resp, field, expected) },
		fmt.Sprintf("waiting for MCP tool %s to report %s %q", tool, field, expected))
	if err != nil {
		return fmt.Errorf("%w (last %s)", err, last)
	}
	return nil
}

func (s *Steps) listMCPToolsStep(ctx context.Context, who string) error {
	_, err := s.sendMCP(ctx, who, nil, nil, []byte(mcpToolsListBody))
	return err
}

func (s *Steps) listMCPToolsWithToken(ctx context.Context, token string) error {
	_, err := s.sendMCP(ctx, "", nil, &token, []byte(mcpToolsListBody))
	return err
}

// callMCPTool sends one tools/call and registers or deregisters what the tool created or
// deleted. The arguments are a JSON object that may hold context placeholders.
func (s *Steps) callMCPTool(
	ctx context.Context, tool, who string, headers map[string]string, arguments string,
) (*httpx.Response, error) {
	expanded, err := stepscommon.Expand(ctx, arguments)
	if err != nil {
		return nil, err
	}
	body, err := mcpToolCallBody(tool, []byte(expanded))
	if err != nil {
		return nil, err
	}
	resp, err := s.sendMCP(ctx, who, headers, nil, body)
	if err != nil {
		return nil, err
	}
	send := func(ctx context.Context, who string, body []byte) (*httpx.Response, error) {
		return s.sendMCP(ctx, who, nil, nil, body)
	}
	if err := s.trackMCPToolResult(ctx, who, tool, []byte(expanded), resp, send); err != nil {
		return resp, err
	}
	return resp, nil
}

func (s *Steps) sendMCP(
	ctx context.Context, who string, headers map[string]string, bearer *string, body []byte,
) (*httpx.Response, error) {
	allHeaders := map[string]string{"Accept": mcpAccept}
	maps.Copy(allHeaders, headers)
	return s.sendControlPlane(ctx, controlPlaneCall{
		method: http.MethodPost, path: mcpPath, who: who, headers: allHeaders, bearer: bearer, body: body,
	})
}

const mcpToolsListBody = `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`

// mcpToolCallBody is the JSON-RPC request for one tool call. No arguments means an empty object.
func mcpToolCallBody(tool string, arguments []byte) ([]byte, error) {
	if len(bytes.TrimSpace(arguments)) == 0 {
		arguments = []byte("{}")
	}
	var object map[string]any
	if err := json.Unmarshal(arguments, &object); err != nil || object == nil {
		return nil, fmt.Errorf("the arguments of MCP tool %q are not a JSON object: %s", tool, arguments)
	}
	return json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": json.RawMessage(arguments)},
	})
}

// decodeMCPReply reads a JSON-RPC answer. It is an error when the answer is not HTTP 200 JSON.
func decodeMCPReply(resp *httpx.Response) (mcpReply, error) {
	var reply mcpReply
	if resp == nil || resp.StatusCode != http.StatusOK {
		return reply, fmt.Errorf("expected HTTP 200 from the MCP endpoint: %s", resp.Describe())
	}
	if err := json.Unmarshal(resp.Body, &reply); err != nil {
		return reply, fmt.Errorf("the MCP answer is not JSON: %w (%s)", err, resp.Describe())
	}
	return reply, nil
}

// mcpToolSucceeded checks the answer is a tool result that is not an error.
func mcpToolSucceeded(resp *httpx.Response) error {
	reply, err := decodeMCPReply(resp)
	if err != nil {
		return err
	}
	if reply.Error != nil {
		return fmt.Errorf("the MCP call failed with a JSON-RPC error %q: %s", reply.Error.Message, resp.Describe())
	}
	if reply.Result == nil {
		return fmt.Errorf("the MCP answer has no result: %s", resp.Describe())
	}
	if reply.Result.IsError {
		return fmt.Errorf("the MCP tool answered with an error: %s", resp.Describe())
	}
	return nil
}

// mcpToolFailedWith checks the answer is a tool error with exactly this sentence.
func mcpToolFailedWith(resp *httpx.Response, sentence string) error {
	reply, err := decodeMCPReply(resp)
	if err != nil {
		return err
	}
	if reply.Error != nil {
		return fmt.Errorf("expected a tool error, got the JSON-RPC error %q: %s", reply.Error.Message, resp.Describe())
	}
	if reply.Result == nil || !reply.Result.IsError {
		return fmt.Errorf("expected a tool error, but the call succeeded: %s", resp.Describe())
	}
	if len(reply.Result.Content) == 0 || reply.Result.Content[0].Text != sentence {
		got := ""
		if len(reply.Result.Content) > 0 {
			got = reply.Result.Content[0].Text
		}
		return fmt.Errorf("the tool error: expected %q, got %q", sentence, got)
	}
	return nil
}

// mcpToolNames returns the names in a tools/list answer.
func mcpToolNames(resp *httpx.Response) ([]string, error) {
	reply, err := decodeMCPReply(resp)
	if err != nil {
		return nil, err
	}
	if reply.Error != nil {
		return nil, fmt.Errorf("tools/list failed with the JSON-RPC error %q: %s", reply.Error.Message, resp.Describe())
	}
	if reply.Result == nil {
		return nil, fmt.Errorf("the tools/list answer has no result: %s", resp.Describe())
	}
	names := make([]string, 0, len(reply.Result.Tools))
	for _, tool := range reply.Result.Tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	return names, nil
}

// sameToolNames checks the listed names are exactly the wanted comma-separated set.
func sameToolNames(got []string, want string) error {
	var wanted []string
	for _, name := range strings.Split(want, ",") {
		if name = strings.TrimSpace(name); name != "" {
			wanted = append(wanted, name)
		}
	}
	slices.Sort(wanted)
	if slices.Equal(got, wanted) {
		return nil
	}
	var missing, unexpected []string
	for _, name := range wanted {
		if !slices.Contains(got, name) {
			missing = append(missing, name)
		}
	}
	for _, name := range got {
		if !slices.Contains(wanted, name) {
			unexpected = append(unexpected, name)
		}
	}
	return fmt.Errorf("the MCP tools differ: missing [%s], unexpected [%s], listed [%s]",
		strings.Join(missing, ", "), strings.Join(unexpected, ", "), strings.Join(got, ", "))
}

func (s *Steps) mcpToolCallSucceeded(ctx context.Context) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	return mcpToolSucceeded(resp)
}

func (s *Steps) mcpToolCallFailed(ctx context.Context, sentence string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	expected, err := stepscommon.Expand(ctx, sentence)
	if err != nil {
		return err
	}
	return mcpToolFailedWith(resp, expected)
}

func (s *Steps) mcpToolsAre(ctx context.Context, names string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	got, err := mcpToolNames(resp)
	if err != nil {
		return err
	}
	return sameToolNames(got, names)
}

// mcpSender sends one JSON-RPC body to the MCP endpoint as the named user.
type mcpSender func(ctx context.Context, who string, body []byte) (*httpx.Response, error)

// trackMCPToolResult registers what a successful tool call created, and drops the records of
// what it deleted, so cleanup follows the tool's own answer. A refused or failed call changes
// nothing. A create or deploy that cannot be tracked is undone through send, as the same user.
func (s *Steps) trackMCPToolResult(
	ctx context.Context, who, tool string, arguments []byte, resp *httpx.Response, send mcpSender,
) error {
	if resp == nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	var reply mcpReply
	if json.Unmarshal(resp.Body, &reply) != nil || reply.Error != nil || reply.Result == nil || reply.Result.IsError {
		return nil
	}
	var args mcpToolArguments
	if err := json.Unmarshal(arguments, &args); err != nil {
		return fmt.Errorf("reading the arguments of MCP tool %q: %w", tool, err)
	}
	switch tool {
	case "create_project":
		return s.trackMCPCreated(ctx, tool, "projects", reply.Result.StructuredContent.ID)
	case "delete_project":
		return deregisterDeletedPlatformResource(ctx, apiBase+"/projects/"+url.PathEscape(args.ProjectID))
	case "create_api", "deploy_api", "delete_api":
		collection, ok := mcpCollections[args.APIType]
		if !ok {
			err := fmt.Errorf("MCP tool %q: no cleanup is defined for apiType %q", tool, args.APIType)
			if compensateErr := compensateMCPTool(ctx, send, who, tool, args, reply); compensateErr != nil {
				return fmt.Errorf("%w; compensating call failed: %v", err, compensateErr)
			}
			return err
		}
		return s.trackMCPTypedResult(ctx, tool, collection, args, reply)
	}
	return nil
}

// compensateMCPTool undoes a create_api or deploy_api that cannot be tracked for cleanup, by
// calling delete_api or undeploy_api. Any other tool needs no undoing.
func compensateMCPTool(
	ctx context.Context, send mcpSender, who, tool string, args mcpToolArguments, reply mcpReply,
) error {
	var undo string
	arguments := map[string]any{"apiType": args.APIType, "confirm": true}
	switch tool {
	case "create_api":
		undo = "delete_api"
		arguments["apiId"] = reply.Result.StructuredContent.ID
	case "deploy_api":
		undo = "undeploy_api"
		arguments["apiId"] = args.APIID
		arguments["gatewayId"] = args.GatewayID
	default:
		return nil
	}
	raw, err := json.Marshal(arguments)
	if err != nil {
		return err
	}
	body, err := mcpToolCallBody(undo, raw)
	if err != nil {
		return err
	}
	resp, err := send(ctx, who, body)
	if err != nil {
		return fmt.Errorf("%s: %w", undo, err)
	}
	if err := mcpToolSucceeded(resp); err != nil {
		return fmt.Errorf("%s: %w", undo, err)
	}
	return nil
}

func (s *Steps) trackMCPTypedResult(
	ctx context.Context, tool, collection string, args mcpToolArguments, reply mcpReply,
) error {
	switch tool {
	case "create_api":
		return s.trackMCPCreated(ctx, tool, collection, reply.Result.StructuredContent.ID)
	case "delete_api":
		return deregisterDeletedPlatformResource(ctx, apiBase+"/"+collection+"/"+url.PathEscape(args.APIID))
	}
	deployments := reply.Result.StructuredContent.Deployments
	if len(deployments) == 0 || deployments[0].ID == "" {
		return fmt.Errorf("MCP tool %q answered with no deployment id, so it cannot be cleaned up", tool)
	}
	location := apiBase + "/" + collection + "/" + args.APIID + "/deployments/" + deployments[0].ID
	body, err := json.Marshal(map[string]string{"gatewayId": args.GatewayID})
	if err != nil {
		return err
	}
	return s.trackCreatedPlatformResource(ctx, location, body)
}

func (s *Steps) trackMCPCreated(ctx context.Context, tool, collection, id string) error {
	if id == "" {
		return fmt.Errorf("MCP tool %q answered with no id, so what it created cannot be cleaned up", tool)
	}
	return s.trackCreatedPlatformResource(ctx, apiBase+"/"+collection+"/"+id, nil)
}
