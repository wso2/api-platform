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
	"encoding/base64"
	"fmt"

	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
)

// RegisterAPISteps registers all API deployment step definitions
func RegisterAPISteps(ctx *godog.ScenarioContext, state *TestState, httpSteps *steps.HTTPSteps) {
	ctx.Step(`^I authenticate using basic auth as "([^"]*)"$`, func(userKey string) error {
		user, ok := state.Config.Users[userKey]
		if !ok {
			return fmt.Errorf("unknown user: %s", userKey)
		}
		credentials := base64.StdEncoding.EncodeToString([]byte(user.Username + ":" + user.Password))
		httpSteps.SetHeader("Authorization", "Basic "+credentials)
		return nil
	})

	ctx.Step(`^I deploy this API configuration:$`, func(body *godog.DocString) error {
		httpSteps.SetHeader("Content-Type", "application/yaml")
		return httpSteps.SendPOSTToService("gateway-controller", "/apis", body)
	})
}
