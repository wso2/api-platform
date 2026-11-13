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

@budget-redis @BUDGET
Feature: Budget spend kept in Redis
  As a gateway operator
  I want budget spend kept in the shared store
  So that a gateway restart does not reset what was spent

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # Each chat-cached call costs $0.00065. $0.01 blocking at 10% refuses from $0.001: after
  # two calls. The no-model request is covered but charged $0, so it is refused only by
  # spend already recorded: after the restart it is refused only if the spend survived.
  @BUDGET-023
  Scenario: Spend survives a gateway restart when the counters are in Redis
    Given I generate a unique resource name from "bgt-redis-provider" and store it as "providerName"
    And I generate a unique API version from "bgt-redis" and store it as "providerVersion"
    And I generate a unique API context from "/bgt-redis" and store it as "providerContext"
    And I generate a unique resource name from "bgt-redis" and store it as "budgetName"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion} |
      | name               | ${CTX:providerName}       |
      | displayName        | ${CTX:providerName}       |
      | version            | ${CTX:providerVersion}    |
      | template           | openai                    |
      | spec.context       | ${CTX:providerContext}    |
      | spec.upstream.url  | http://testbench:3008     |
      | accessControl.mode | allow_all                 |
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
    Then the response status code should be 201
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:budgetName}" of type "Budget" is present
    Then the response status code should be 200

    Given I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:providerContext}/unknown-llm/v1/no-model-field" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-cached" with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-cached" until status 429 with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the JSON response field "budgets[0].budget" should be "${CTX:budgetName}"

    When I restart the "gateway-controller" service
    And I restart the "gateway-runtime" service
    And I wait for the gateway controller health endpoint
    And I set header "Content-Type" to "application/json"
    # Charged $0, so only the spend recorded before the restart can refuse it.
    And I send a "POST" request to "${CTX:providerContext}/unknown-llm/v1/no-model-field" until status 429 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    Then the JSON response field "budgets[0].budget" should be "${CTX:budgetName}"
