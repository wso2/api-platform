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
 * software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package it

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
)

// RegisterSubscriptionSteps registers step definitions for subscription validation tests.
func RegisterSubscriptionSteps(ctx *godog.ScenarioContext, state *TestState, httpSteps *steps.HTTPSteps) {
	ctx.Step(`^I create a subscription plan "([^"]*)" with (\d+) requests per minute$`, func(planName string, limit int) error {
		body := fmt.Sprintf(`{"planName":"%s","throttleLimitCount":%d,"throttleLimitUnit":"Min"}`, planName, limit)
		httpSteps.SetHeader("Content-Type", "application/json")
		err := httpSteps.SendPOSTToService("gateway-controller", "/subscription-plans", &godog.DocString{Content: body})
		if err != nil {
			return err
		}
		time.Sleep(policyPropagationDelay)
		return nil
	})

	ctx.Step(`^I store the last response field "([^"]*)" as "([^"]*)"$`, func(fieldName, contextKey string) error {
		body := httpSteps.LastBody()
		if body == nil {
			return fmt.Errorf("no response body to parse")
		}
		var m map[string]interface{}
		if err := json.Unmarshal(body, &m); err != nil {
			return err
		}
		v, ok := m[fieldName]
		if !ok {
			return fmt.Errorf("field %q not found in response", fieldName)
		}
		var s string
		switch val := v.(type) {
		case string:
			s = val
		case float64:
			s = strconv.FormatInt(int64(val), 10)
		default:
			s = fmt.Sprintf("%v", v)
		}
		state.SetContextValue(contextKey, s)
		return nil
	})

	ctx.Step(`^I create a subscription for API "([^"]*)" with plan and token "([^"]*)"$`, func(apiHandle, token string) error {
		planID, ok := state.GetContextString("planId")
		if !ok {
			return fmt.Errorf("planId not found in context; create a subscription plan first and store its id")
		}
		// Mock platform-api event: inject subscription.created (platform-api normally sends this via WebSocket)
		body := fmt.Sprintf(`{"apiHandle":"%s","subscriptionToken":"%s","subscriptionPlanId":"%s"}`, apiHandle, token, planID)
		httpSteps.SetHeader("Content-Type", "application/json")
		err := httpSteps.SendPOSTToService("mock-platform-api", "/inject-subscription", &godog.DocString{Content: body})
		if err != nil {
			return err
		}
		time.Sleep(policyPropagationDelay)
		return nil
	})

	ctx.Step(`^I create a monetized subscription for API "([^"]*)" with plan, token "([^"]*)", billing customer "([^"]*)" and billing subscription "([^"]*)"$`,
		func(apiHandle, token, billingCustomerID, billingSubscriptionID string) error {
			planID, ok := state.GetContextString("planId")
			if !ok {
				return fmt.Errorf("planId not found in context; create a subscription plan first and store its id")
			}
			body := fmt.Sprintf(`{"apiHandle":"%s","subscriptionToken":"%s","subscriptionPlanId":"%s","billingCustomerId":"%s","billingSubscriptionId":"%s"}`,
				apiHandle, token, planID, billingCustomerID, billingSubscriptionID)
			httpSteps.SetHeader("Content-Type", "application/json")
			err := httpSteps.SendPOSTToService("mock-platform-api", "/inject-subscription", &godog.DocString{Content: body})
			if err != nil {
				return err
			}
			time.Sleep(policyPropagationDelay)
			return nil
		})

	ctx.Step(`^I delete the subscription plan with stored id "([^"]*)"$`, func(contextKey string) error {
		planID, ok := state.GetContextString(contextKey)
		if !ok {
			return fmt.Errorf("%q not found in context", contextKey)
		}
		return httpSteps.SendDELETEToService("gateway-controller", "/subscription-plans/"+planID)
	})
}
