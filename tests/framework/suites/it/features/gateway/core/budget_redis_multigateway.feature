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

@budget-redis-multigateway @BUDGET
Feature: Budget spend shared by gateways through Redis
  As a gateway operator running more than one gateway
  I want every gateway sharing the counter store to count one budget's spend together
  So that a budget used up through one gateway refuses on the others too

  Background:
    Given the first gateway is running
    And the second gateway is running
    And I authenticate using basic auth as "admin"

  # Each gateway has its own database and only Redis in common. The same provider and budget
  # are created on both. Each chat-cached call costs $0.00065; $0.01 blocking at 10% refuses
  # from $0.001. Only the first gateway is sent priced traffic. The no-model request on the
  # second gateway is charged $0, so only the first gateway's spend can refuse it there.
  @BUDGET-024
  Scenario: Spend through one gateway refuses the same budget on another gateway sharing Redis
    Given I generate a unique resource name from "bgt-shared-provider" and store it as "providerName"
    And I generate a unique API version from "bgt-shared" and store it as "providerVersion"
    And I generate a unique API context from "/bgt-shared" and store it as "providerContext"
    And I generate a unique resource name from "bgt-shared" and store it as "budgetName"
    When I create LLM provider on the first gateway from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion} |
      | name               | ${CTX:providerName}       |
      | displayName        | ${CTX:providerName}       |
      | version            | ${CTX:providerVersion}    |
      | template           | openai                    |
      | spec.context       | ${CTX:providerContext}    |
      | spec.upstream.url  | http://testbench:3008     |
      | accessControl.mode | allow_all                 |
    Then the response status code should be 201
    When I create LLM provider on the second gateway from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion} |
      | name               | ${CTX:providerName}       |
      | displayName        | ${CTX:providerName}       |
      | version            | ${CTX:providerVersion}    |
      | template           | openai                    |
      | spec.context       | ${CTX:providerContext}    |
      | spec.upstream.url  | http://testbench:3008     |
      | accessControl.mode | allow_all                 |
    Then the response status code should be 201
    When I create Budget on the first gateway from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "0.01"                    |
      | period              | month                     |
      | action              | block                     |
      | resource.allocation | shared                    |
      | spec.resource.id    | ${CTX:providerName}       |
      | spec.limit.blockAt  | 10                        |
    Then the response status code should be 201
    When I create Budget on the second gateway from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "0.01"                    |
      | period              | month                     |
      | action              | block                     |
      | resource.allocation | shared                    |
      | spec.resource.id    | ${CTX:providerName}       |
      | spec.limit.blockAt  | 10                        |
    Then the response status code should be 201

    Given I set header "Content-Type" to "application/json"
    # Both gateways serve the route before anything is spent.
    And I send a "POST" request to the second gateway "${CTX:providerContext}/unknown-llm/v1/no-model-field" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    And I send a "POST" request to the first gateway "${CTX:providerContext}/unknown-llm/v1/no-model-field" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    When I send a "POST" request to the first gateway "${CTX:providerContext}/openai/v1/chat-cached" until status 429 with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the JSON response field "budgets[0].budget" should be "${CTX:budgetName}"
    # Charged $0 and never sent priced traffic: only the first gateway's spend refuses it.
    When I send a "POST" request to the second gateway "${CTX:providerContext}/unknown-llm/v1/no-model-field" until status 429 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    Then the JSON response field "budgets[0].budget" should be "${CTX:budgetName}"
