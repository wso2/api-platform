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
	"time"

	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
)

// policyPropagationDelay is the time to wait after mutating operations
// to allow the Policy Engine to receive and apply configuration changes.
const policyPropagationDelay = 2 * time.Second

// RegisterAPISteps registers all API deployment step definitions
func RegisterAPISteps(ctx *godog.ScenarioContext, state *TestState, httpSteps *steps.HTTPSteps) {
	// Single deploy function used by multiple step patterns
	deployAPI := func(body *godog.DocString) error {
		httpSteps.SetHeader("Content-Type", "application/yaml")
		err := httpSteps.SendPOSTToService("gateway-controller", "/apis", body)
		if err != nil {
			return err
		}
		time.Sleep(policyPropagationDelay)
		return nil
	}

	// Single delete function used by multiple step patterns
	deleteAPI := func(name string) error {
		err := httpSteps.SendDELETEToService("gateway-controller", "/apis/"+name)
		if err != nil {
			return err
		}
		time.Sleep(policyPropagationDelay)
		return nil
	}

	// Register multiple step patterns for deploy
	ctx.Step(`^I deploy this API configuration:$`, deployAPI)
	ctx.Step(`^I deploy an API with the following configuration:$`, deployAPI)
	ctx.Step(`^I deploy a test API with the following configuration:$`, deployAPI)

	// Register multiple step patterns for delete
	ctx.Step(`^I delete the API "([^"]*)"$`, deleteAPI)
	// Note: Version parameter is semantically meaningful in tests but not used by the API endpoint.
	// The API deletes by name only - version is embedded in the API YAML, not in the DELETE path.
	ctx.Step(`^I delete the API "([^"]*)" version "([^"]*)"$`, func(name, version string) error {
		return deleteAPI(name)
	})

	ctx.Step(`^I update the API "([^"]*)" with this configuration:$`, func(apiName string, body *godog.DocString) error {
		httpSteps.SetHeader("Content-Type", "application/yaml")
		err := httpSteps.SendPUTToService("gateway-controller", "/apis/"+apiName, body)
		if err != nil {
			return err
		}
		time.Sleep(policyPropagationDelay)
		return nil
	})

	ctx.Step(`^I get the API "([^"]*)"$`, func(name string) error {
		return httpSteps.SendGETToService("gateway-controller", "/apis/"+name)
	})
}
