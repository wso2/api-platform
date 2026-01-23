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

package it

import (
	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
)

// RegisterSecretSteps registers all secret management step definitions
func RegisterSecretSteps(ctx *godog.ScenarioContext, state *TestState, httpSteps *steps.HTTPSteps) {
	// Create secret steps
	ctx.Step(`^I create this secret:$`, func(body *godog.DocString) error {
		httpSteps.SetHeader("Content-Type", "application/yaml")
		return httpSteps.SendPOSTToService("gateway-controller", "/secrets", body)
	})

	// Create secret with oversized value (for validation testing)
	ctx.Step(`^I create a secret with oversized value$`, func() error {
		// Generate a value larger than 10KB (10,240 bytes)
		oversizedValue := make([]byte, 10241)
		for i := range oversizedValue {
			oversizedValue[i] = 'x'
		}

		body := &godog.DocString{
			Content: `apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Secret
metadata:
  name: oversized-secret
spec:
  displayName: Oversized Secret
  description: Secret with value exceeding 10KB limit
  type: default
  value: ` + string(oversizedValue),
		}
		httpSteps.SetHeader("Content-Type", "application/yaml")
		return httpSteps.SendPOSTToService("gateway-controller", "/secrets", body)
	})

	// Create secret with value of given size (bytes)
	ctx.Step(`^I create a secret with value size (\d+)$`, func(size int) error {
		// Generate value of requested size
		value := make([]byte, size)
		for i := range value {
			value[i] = 'x'
		}

		body := &godog.DocString{
			Content: `apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Secret
metadata:
  name: size-test-secret
spec:
  displayName: Size Test Secret
  description: Secret with configurable value size
  type: default
  value: ` + string(value),
		}

		httpSteps.SetHeader("Content-Type", "application/yaml")
		return httpSteps.SendPOSTToService("gateway-controller", "/secrets", body)
	})

	// Retrieve secret step
	ctx.Step(`^I retrieve the secret "([^"]*)"$`, func(secretID string) error {
		return httpSteps.SendGETToService("gateway-controller", "/secrets/"+secretID)
	})

	// Update secret steps
	ctx.Step(`^I update the secret "([^"]*)" with:$`, func(secretID string, body *godog.DocString) error {
		httpSteps.SetHeader("Content-Type", "application/yaml")
		return httpSteps.SendPUTToService("gateway-controller", "/secrets/"+secretID, body)
	})

	// Delete secret step
	ctx.Step(`^I delete the secret "([^"]*)"$`, func(secretID string) error {
		return httpSteps.SendDELETEToService("gateway-controller", "/secrets/"+secretID)
	})

	// List secrets step
	ctx.Step(`^I list all secrets$`, func() error {
		return httpSteps.SendGETToService("gateway-controller", "/secrets")
	})
}
