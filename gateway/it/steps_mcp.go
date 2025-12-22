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
	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
)

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
}
