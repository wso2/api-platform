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
	"fmt"

	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
)

// RegisterLLMSteps registers all LLM provider template step definitions
func RegisterLLMSteps(ctx *godog.ScenarioContext, state *TestState, httpSteps *steps.HTTPSteps) {
	ctx.Step(`^I create this LLM provider template:$`, func(body *godog.DocString) error {
		httpSteps.SetHeader("Content-Type", "application/yaml")
		return httpSteps.SendPOSTToService("gateway-controller", "/llm-provider-templates", body)
	})

	ctx.Step(`^I retrieve the LLM provider template "([^"]*)"$`, func(templateID string) error {
		return httpSteps.SendGETToService("gateway-controller", "/llm-provider-templates/"+templateID)
	})

	ctx.Step(`^I update the LLM provider template "([^"]*)" with:$`, func(templateID string, body *godog.DocString) error {
		httpSteps.SetHeader("Content-Type", "application/yaml")
		return httpSteps.SendPUTToService("gateway-controller", "/llm-provider-templates/"+templateID, body)
	})

	ctx.Step(`^I delete the LLM provider template "([^"]*)"$`, func(templateID string) error {
		return httpSteps.SendDELETEToService("gateway-controller", "/llm-provider-templates/"+templateID)
	})

	ctx.Step(`^I list all LLM provider templates$`, func() error {
		return httpSteps.SendGETToService("gateway-controller", "/llm-provider-templates")
	})

	ctx.Step(`^I list LLM provider templates with filter "([^"]*)" as "([^"]*)"$`, func(filterKey, filterValue string) error {
		return httpSteps.SendGETToService("gateway-controller", "/llm-provider-templates?"+filterKey+"="+filterValue)
	})

	ctx.Step(`^the response should contain oob-templates$`, func() error {
		// This step verifies that out-of-box templates are present in the list response
		// The actual assertion is done by checking the response body
		body := httpSteps.LastBody()
		if len(body) == 0 {
			return nil
		}
		// The actual validation of OOB templates should be done using JSON assertions
		// in the feature file itself, so this step just ensures we got a response
		var response struct {
			Count     int `json:"count"`
			Templates []struct {
				ID string `json:"id"`
			} `json:"templates"`
		}

		if err := json.Unmarshal([]byte(body), &response); err != nil {
			return fmt.Errorf("failed to parse response JSON: %w", err)
		}

		// 1️⃣ Validate count
		expectedCount := 7
		if response.Count != expectedCount {
			return fmt.Errorf(
				"expected template count to be %d, but got %d",
				expectedCount,
				response.Count,
			)
		}

		// 2️⃣ Collect actual template IDs
		actualIDs := make(map[string]bool)
		for _, t := range response.Templates {
			actualIDs[t.ID] = true
		}

		// 3️⃣ Expected OOB template IDs
		expectedIDs := []string{
			"azureai-foundry",
			"anthropic",
			"openai",
			"gemini",
			"azure-openai",
			"mistralai",
			"awsbedrock",
		}

		// 4️⃣ Validate all expected IDs are present
		for _, expectedID := range expectedIDs {
			if !actualIDs[expectedID] {
				return fmt.Errorf(
					"expected oob-template with id '%s' was not found in response",
					expectedID,
				)
			}
		}

		return nil
	})
}
