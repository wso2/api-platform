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

@budget-ha @BUDGET
Feature: Budgets on a gateway with two controllers
  As a gateway operator running the gateway-controller highly available
  I want a budget written through one controller to be enforced by runtimes fed by the other
  So that budgets work the same with one controller or several

  # The block runs two controllers on one PostgreSQL database. The management API is served by
  # the first; the runtime is fed only by the second, so every budget change below reaches
  # enforcement through the event path between them. Spend is counted in Redis.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  @BUDGET-027
  Scenario: A budget written through one controller reaches the runtime fed by the other and follows its changes
    Given I generate a unique resource name from "bgt-ha-xds" and store it as "budgetName"
    When I create Budget from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "500.00"                  |
      | period              | month                     |
      | action              | block                     |
      | spec.limit.blockAt  | 95                        |
    Then the response status code should be 201
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:budgetName}" of type "Budget" is present
    Then the JSON response array "lazy_resources.resources_by_type.Budget" item with "id" equal to "${CTX:budgetName}" should have "resource.limit.amount" equal to "500.00"

    When I update Budget "${CTX:budgetName}" from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "650.00"                  |
      | period              | month                     |
      | action              | block                     |
      | spec.limit.blockAt  | 95                        |
    Then the response status code should be 200
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:budgetName}" of type "Budget" has "limit.amount" equal to "650.00"
    Then the response status code should be 200

    When I update Budget "${CTX:budgetName}" from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "650.00"                  |
      | period              | month                     |
      | action              | block                     |
      | spec.status         | inactive                  |
      | spec.limit.blockAt  | 95                        |
    Then the response status code should be 200
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:budgetName}" of type "Budget" is absent
    Then the response status code should be 200

    When I update Budget "${CTX:budgetName}" from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "650.00"                  |
      | period              | month                     |
      | action              | block                     |
      | spec.limit.blockAt  | 95                        |
    Then the response status code should be 200
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:budgetName}" of type "Budget" is present
    Then the response status code should be 200

    When I delete the Budget "${CTX:budgetName}"
    Then the response status code should be 200
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:budgetName}" of type "Budget" is absent
    Then the response status code should be 200

  # Each chat-cached call costs $0.00065; a $0.01 budget blocking at 1% is used up by one call.
  # Raising the amount through the first controller must lift the block on the runtime fed by
  # the second, which reads the spend back from Redis.
  @BUDGET-028
  Scenario: A budget used up through one controller's runtime is lifted by an edit through the other
    Given I generate a unique resource name from "bgt-ha-provider" and store it as "providerName"
    And I generate a unique API version from "bgt-ha" and store it as "providerVersion"
    And I generate a unique API context from "/bgt-ha" and store it as "providerContext"
    And I generate a unique resource name from "bgt-ha-block" and store it as "budgetName"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}  |
      | name               | ${CTX:providerName}        |
      | displayName        | ${CTX:providerName}        |
      | version            | ${CTX:providerVersion}     |
      | template           | openai                     |
      | spec.context       | ${CTX:providerContext}     |
      | spec.upstream.url  | http://testbench:3008      |
      | accessControl.mode | allow_all                  |
    Then the response status code should be 201
    When I create Budget from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "0.01"                    |
      | period              | day                       |
      | action              | block                     |
      | spec.filters        | [{"type":"llmProvider","id":"${CTX:providerName}"}] |
      | spec.limit.blockAt  | 1                         |
    Then the response status code should be 201
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:budgetName}" of type "Budget" is present
    Then the response status code should be 200

    Given I set header "Content-Type" to "application/json"
    # A request without a model is not priced, so it shows the route is live without spending.
    And I send a "POST" request to "${CTX:providerContext}/unknown-llm/v1/no-model-field" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-cached" until status 429 with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the JSON response field "code" should be "BUDGET_EXCEEDED"
    And the JSON response field "budgets[0].budget" should be "${CTX:budgetName}"

    When I update Budget "${CTX:budgetName}" from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "100.00"                  |
      | period              | day                       |
      | action              | block                     |
      | spec.filters        | [{"type":"llmProvider","id":"${CTX:providerName}"}] |
      | spec.limit.blockAt  | 100                       |
    Then the response status code should be 200
    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-cached" until status 200 with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
