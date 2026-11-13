# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

# Runs in the mcp-agent-policies-analytics block, whose platform-gateway carries the
# mcp-jwt-auth.toml overlay registering the "mock-jwks" issuer that jwt-auth validates against.
@budget-jwt @BUDGET
Feature: Budgets on an LLM provider secured with jwt-auth
  As a FinOps administrator
  I want a budget to read the verified token of each request
  So that each user's spend is capped on their own

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # Each chat-cached call costs $0.00065; $0.01 blocking at 10% refuses from $0.001: after two
  # calls by one user. The budget gives every token subject its own allowance.
  @BUDGET-025
  Scenario: A per-user budget on a jwt-auth route charges each token subject and runs after authentication
    Given I generate a unique resource name from "bgt-jwt-provider" and store it as "providerName"
    And I generate a unique API version from "bgt-jwt" and store it as "providerVersion"
    And I generate a unique API context from "/bgt-jwt" and store it as "providerContext"
    And I generate a unique resource name from "bgt-jwt" and store it as "budgetName"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion} |
      | name               | ${CTX:providerName}       |
      | displayName        | ${CTX:providerName}       |
      | version            | ${CTX:providerVersion}    |
      | template           | openai                    |
      | spec.context       | ${CTX:providerContext}    |
      | spec.upstream.url  | http://testbench:3008     |
      | accessControl.mode | allow_all                 |
      | spec.policies      | [{"name":"jwt-auth","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"issuers":["mock-jwks"]}}]}] |
    Then the response status code should be 201
    When I create Budget from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "0.01"                    |
      | period              | month                     |
      | action              | block                     |
      | resource.allocation | shared                    |
      | spec.resource.id    | ${CTX:providerName}       |
      | spec.limit.blockAt  | 10                        |
      | spec.consumers      | [{"type":"attribute","allocation":"separate","expression":"request.Auth.Claims[\"sub\"]"}] |
    Then the response status code should be 201
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:budgetName}" of type "Budget" is present
    Then the response status code should be 200

    Given I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-cached" until status 401 with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and claims "sub=alice@wso2.com" and store it as "aliceToken"
    And I set header "Authorization" to "Bearer ${CTX:aliceToken}"
    And I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-cached" until status 429 with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the JSON response field "code" should be "BUDGET_EXCEEDED"
    And the JSON response field "budgets[0].budget" should be "${CTX:budgetName}"

    # Another user has an allowance of their own.
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and claims "sub=bob@wso2.com" and store it as "bobToken"
    And I set header "Authorization" to "Bearer ${CTX:bobToken}"
    And I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-cached" with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200

    # Authentication runs first: without a token the request is refused by jwt-auth, not the budget.
    When I clear all headers
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-cached" with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 401
