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
	"fmt"
	"net/url"
	"time"

	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
)

// RegisterGraphQLSteps registers all GraphQL API deployment step definitions.
// Mirrors RegisterAPISteps (RestApi) / RegisterMCPSteps (Mcp) — GraphQLApi is a
// core kind on the gateway-controller with the same generic
// create/list/get/update/delete surface at /graphql-apis, just with no
// per-operation routes: a GraphQL API always resolves to exactly one POST
// route, unlike REST's operations[] list.
func RegisterGraphQLSteps(ctx *godog.ScenarioContext, state *TestState, httpSteps *steps.HTTPSteps, jwtSteps *JWTSteps) {
	deployGraphQLAPI := func(body *godog.DocString) error {
		httpSteps.SetHeader("Content-Type", "application/yaml")
		err := httpSteps.SendPOSTToService("gateway-controller", "/graphql-apis", body)
		if err != nil {
			return err
		}
		time.Sleep(policyPropagationDelay)
		return nil
	}

	deleteGraphQLAPI := func(name string) error {
		err := httpSteps.SendDELETEToService("gateway-controller", "/graphql-apis/"+url.PathEscape(name))
		if err != nil {
			return err
		}
		time.Sleep(policyPropagationDelay)
		return nil
	}

	ctx.Step(`^I deploy this GraphQL configuration:$`, deployGraphQLAPI)

	ctx.Step(`^I list all GraphQL APIs$`, func() error {
		return httpSteps.SendGETToService("gateway-controller", "/graphql-apis")
	})

	ctx.Step(`^I get the GraphQL API "([^"]*)"$`, func(name string) error {
		return httpSteps.SendGETToService("gateway-controller", "/graphql-apis/"+url.PathEscape(name))
	})

	ctx.Step(`^I update the GraphQL API "([^"]*)" with:$`, func(name string, body *godog.DocString) error {
		httpSteps.SetHeader("Content-Type", "application/yaml")
		err := httpSteps.SendPUTToService("gateway-controller", "/graphql-apis/"+url.PathEscape(name), body)
		if err != nil {
			return err
		}
		time.Sleep(policyPropagationDelay)
		return nil
	})

	ctx.Step(`^I delete the GraphQL API "([^"]*)"$`, deleteGraphQLAPI)

	// Invoking the deployed single-route GraphQL endpoint with a bearer token —
	// the generic "I send a POST request... with the JWT token" step (steps_jwt.go)
	// has no body variant, and a GraphQL query is always a POST with a JSON body.
	ctx.Step(`^I send a POST request to "([^"]*)" with the JWT token and body:$`, func(url string, body *godog.DocString) error {
		if jwtSteps == nil || jwtSteps.currentToken == "" {
			return fmt.Errorf("no JWT token available - call 'I get a JWT token from the mock JWKS server' first")
		}
		httpSteps.SetHeader("Content-Type", "application/json")
		httpSteps.SetHeader("Authorization", "Bearer "+jwtSteps.currentToken)
		return httpSteps.ISendPOSTRequestWithBody(url, body)
	})
}
