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
	ctx.Step(`^I deploy this API configuration:$`, func(body *godog.DocString) error {
		httpSteps.SetHeader("Content-Type", "application/yaml")
		err := httpSteps.SendPOSTToService("gateway-controller", "/apis", body)
		if err != nil {
			return err
		}
		time.Sleep(policyPropagationDelay)
		return nil
	})

	ctx.Step(`^I delete the API "([^"]*)"$`, func(name string) error {
		err := httpSteps.SendDELETEToService("gateway-controller", "/apis/"+name)
		if err != nil {
			return err
		}
		time.Sleep(policyPropagationDelay)
		return nil
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
