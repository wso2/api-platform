/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package it

import (
	"encoding/json"

	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
)

type JsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// RegisterMCPSteps registers all MCP related step definitions
func RegisterMCPSteps(ctx *godog.ScenarioContext, state *TestState, httpSteps *steps.HTTPSteps) {
	ctx.Step(`^I deploy this MCP configuration:$`, func(body *godog.DocString) error {
		httpSteps.SetHeader("Content-Type", "application/yaml")
		return httpSteps.SendPOSTToService("gateway-controller", "/mcp-proxies", body)
	})

	ctx.Step(`^I list all MCP proxies$`, func() error {
		return httpSteps.SendGETToService("gateway-controller", "/mcp-proxies")
	})

	ctx.Step(`^I update the MCP proxy "([^"]*)" with:$`, func(name string, body *godog.DocString) error {
		httpSteps.SetHeader("Content-Type", "application/yaml")
		return httpSteps.SendPUTToService("gateway-controller", "/mcp-proxies/"+name, body)
	})

	ctx.Step(`^I delete the MCP proxy "([^"]*)"$`, func(name string) error {
		return httpSteps.SendDELETEToService("gateway-controller", "/mcp-proxies/"+name)
	})

	ctx.Step(`^I use the MCP Client to send an initialize request to "([^"]*)"$`, func(url string) error {
		httpSteps.SetHeader("Content-Type", "application/json")
		payload := generateMcpPayload("initialize")
		return httpSteps.SendMcpRequest(url, &godog.DocString{
			Content: payload,
		})
	})

	ctx.Step(`^I use the MCP Client to send a tools/call request to "([^"]*)"$`, func(url string) error {
		httpSteps.SetHeader("Content-Type", "application/json")
		payload := generateMcpPayload("tools/call")
		return httpSteps.SendMcpRequest(url, &godog.DocString{
			Content: payload,
		})
	})
}

func generateMcpPayload(method string) string {
	var initRequest JsonRPCRequest
	switch method {
	case "initialize":
		initRequest = JsonRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  method,
			Params: map[string]interface{}{
				"protocolVersion": "2025-06-18",
				"capabilities":    map[string]interface{}{"roots": map[string]bool{"listChanged": true}},
				"clientInfo":      map[string]string{"name": "gateway-it-client", "version": "1.0.0"},
			},
		}
	case "tools/call":
		initRequest = JsonRPCRequest{
			JSONRPC: "2.0",
			ID:      2,
			Method:  method,
			Params: map[string]any{
				"name": "add",
				"arguments": map[string]any{
					"a": 40,
					"b": 60,
				},
			},
		}
	default:
		return ""
	}

	payloadBytes, _ := json.Marshal(initRequest)
	return string(payloadBytes)
}
