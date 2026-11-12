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

@budget-management @BUDGET
Feature: Budget management
  As a gateway administrator
  I want to manage budgets through the management API
  So that I can cap what consumers spend on LLM providers

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  @BUDGET-001 @BUDGET-002 @BUDGET-003 @BUDGET-004 @BUDGET-005
  Scenario: A budget on one provider is created with defaults, read, listed, updated and deleted
    Given I generate a unique resource name from "bgt-provider" and store it as "providerName"
    And I generate a unique API version from "bgt-provider" and store it as "providerVersion"
    And I generate a unique API context from "/bgt-provider" and store it as "providerContext"
    And I generate a unique resource name from "bgt-lifecycle" and store it as "budgetName"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}       |
      | name               | ${CTX:providerName}             |
      | displayName        | ${CTX:providerName}             |
      | version            | ${CTX:providerVersion}          |
      | template           | openai                          |
      | spec.context       | ${CTX:providerContext}          |
      | spec.upstream.url  | http://testbench:3008/openai/v1 |
      | accessControl.mode | allow_all                       |
    Then the response status code should be 201

    When I create Budget from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "500.00"                  |
      | period              | month                     |
      | action              | block                     |
      | resource.allocation | shared                    |
      | spec.resource.id    | ${CTX:providerName}       |
      | spec.consumers      | [{"type":"attribute","allocation":"shared","expression":"'research' in request.Auth.Claims[\"groups\"]"}] |
    Then the response status code should be 201
    And the JSON response field "status.id" should be "${CTX:budgetName}"
    And the JSON response field "status.state" should be "deployed"
    And the JSON response field "spec.status" should be "active"
    And the JSON response field "spec.limit.currency" should be "USD"
    And the JSON response field "spec.limit.blockAt" should be 100

    When I send a "GET" request to the "gateway-controller" service at "/budgets/${CTX:budgetName}"
    Then the response status code should be 200
    And the JSON response field "spec.limit.amount" should be "500.00"
    And the JSON response field "spec.resource.id" should be "${CTX:providerName}"
    And the JSON response field "spec.consumers[0].allocation" should be "shared"

    When I send a "GET" request to the "gateway-controller" service at "/budgets"
    Then the response status code should be 200
    And the JSON response array "budgets" should contain an item with "metadata.name" equal to "${CTX:budgetName}"

    When I update Budget "${CTX:budgetName}" from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "750.00"                  |
      | period              | month                     |
      | action              | allow                     |
      | resource.allocation | shared                    |
      | spec.resource.id    | ${CTX:providerName}       |
      | spec.status         | inactive                  |
      | spec.limit.blockAt  | 90                        |
      | spec.consumers      | [{"type":"attribute","allocation":"shared","expression":"'research' in request.Auth.Claims[\"groups\"]"}] |
      | spec.exemptions     | [{"type":"attribute","expression":"request.Auth.Claims[\"sub\"] == \"admin\""}] |
    Then the response status code should be 200

    When I send a "GET" request to the "gateway-controller" service at "/budgets/${CTX:budgetName}"
    Then the response status code should be 200
    And the JSON response field "spec.limit.amount" should be "750.00"
    And the JSON response field "spec.limit.action" should be "allow"
    And the JSON response field "spec.limit.blockAt" should be 90
    And the JSON response field "spec.status" should be "inactive"
    And the JSON response array field "spec.exemptions" should have 1 item

    When I delete the Budget "${CTX:budgetName}"
    Then the response status code should be 200
    And the JSON response field "message" should be "Budget deleted successfully"

    When I send a "GET" request to the "gateway-controller" service at "/budgets/${CTX:budgetName}"
    Then the response status code should be 404

  @BUDGET-006 @type:negative
  Scenario: Changing a budget's period is refused and the budget keeps its period
    Given I generate a unique resource name from "bgt-fixed" and store it as "budgetName"
    When I create Budget from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "100.00"                  |
      | period              | week                      |
      | action              | block                     |
      | resource.allocation | separate                  |
    Then the response status code should be 201

    When I update Budget "${CTX:budgetName}" from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "100.00"                  |
      | period              | day                       |
      | action              | block                     |
      | resource.allocation | separate                  |
    Then the response status code should be 400
    And the JSON response field "errors[0].field" should be "spec.limit.period"

    When I send a "GET" request to the "gateway-controller" service at "/budgets/${CTX:budgetName}"
    Then the JSON response field "spec.limit.period" should be "week"

  @BUDGET-007 @type:negative
  Scenario: An invalid budget is rejected with every failing field and nothing is stored
    Given I generate a unique resource name from "bgt-invalid" and store it as "budgetName"
    When I create Budget from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "-5"                      |
      | period              | year                      |
      | action              | block                     |
      | resource.allocation | shared                    |
      | spec.limit.blockAt  | 101                       |
    Then the response status code should be 400
    And the response body should contain "spec.limit.amount"
    And the response body should contain "spec.limit.period"
    And the response body should contain "spec.limit.blockAt"

    When I send a "GET" request to the "gateway-controller" service at "/budgets/${CTX:budgetName}"
    Then the response status code should be 404

  @BUDGET-008 @type:negative
  Scenario: A budget whose name is taken is refused
    Given I generate a unique resource name from "bgt-duplicate" and store it as "budgetName"
    When I create Budget from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "10"                      |
      | period              | day                       |
      | action              | block                     |
      | resource.allocation | shared                    |
    Then the response status code should be 201

    When I create Budget from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "20"                      |
      | period              | day                       |
      | action              | block                     |
      | resource.allocation | shared                    |
    Then the response status code should be 409

  @BUDGET-009 @type:negative
  Scenario: A budget naming a provider the gateway does not have is rejected
    Given I generate a unique resource name from "bgt-noprovider" and store it as "budgetName"
    When I create Budget from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "10"                      |
      | period              | day                       |
      | action              | block                     |
      | resource.allocation | shared                    |
      | spec.resource.id    | no-such-provider          |
    Then the response status code should be 400
    And the JSON response field "errors[0].field" should be "spec.resource.id"

  @BUDGET-010 @BUDGET-011 @BUDGET-012 @BUDGET-013
  Scenario: An active budget reaches the policy engine, follows its edits and is withdrawn when deactivated or deleted
    Given I generate a unique resource name from "bgt-xds" and store it as "budgetName"
    When I create Budget from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "500.00"                  |
      | period              | month                     |
      | action              | block                     |
      | resource.allocation | shared                    |
      | spec.limit.blockAt  | 95                        |
      | spec.consumers      | [{"type":"attribute","allocation":"separate","expression":"request.Auth.Claims[\"sub\"] != \"\""}] |
    Then the response status code should be 201
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:budgetName}" of type "Budget" is present
    Then the JSON response array "lazy_resources.resources_by_type.Budget" item with "id" equal to "${CTX:budgetName}" should have "resource.limit.amount" equal to "500.00"
    And the JSON response array "lazy_resources.resources_by_type.Budget" item with "id" equal to "${CTX:budgetName}" should have "resource.limit.blockAt" equal to "95"
    And the JSON response array "lazy_resources.resources_by_type.Budget" item with "id" equal to "${CTX:budgetName}" should have "resource.resource.allocation" equal to "shared"
    And the JSON response array "lazy_resources.resources_by_type.Budget" item with "id" equal to "${CTX:budgetName}" should have "resource.consumers[0].allocation" equal to "separate"

    When I update Budget "${CTX:budgetName}" from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "650.00"                  |
      | period              | month                     |
      | action              | block                     |
      | resource.allocation | shared                    |
      | spec.limit.blockAt  | 95                        |
      | spec.consumers      | [{"type":"attribute","allocation":"separate","expression":"request.Auth.Claims[\"sub\"] != \"\""}] |
    Then the response status code should be 200
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:budgetName}" of type "Budget" has "limit.amount" equal to "650.00"
    Then the response status code should be 200

    When I update Budget "${CTX:budgetName}" from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "650.00"                  |
      | period              | month                     |
      | action              | block                     |
      | resource.allocation | shared                    |
      | spec.status         | inactive                  |
      | spec.limit.blockAt  | 95                        |
      | spec.consumers      | [{"type":"attribute","allocation":"separate","expression":"request.Auth.Claims[\"sub\"] != \"\""}] |
    Then the response status code should be 200
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:budgetName}" of type "Budget" is absent
    Then the response status code should be 200

    When I update Budget "${CTX:budgetName}" from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "650.00"                  |
      | period              | month                     |
      | action              | block                     |
      | resource.allocation | shared                    |
      | spec.limit.blockAt  | 95                        |
      | spec.consumers      | [{"type":"attribute","allocation":"separate","expression":"request.Auth.Claims[\"sub\"] != \"\""}] |
    Then the response status code should be 200
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:budgetName}" of type "Budget" is present
    Then the response status code should be 200

    When I delete the Budget "${CTX:budgetName}"
    Then the response status code should be 200
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:budgetName}" of type "Budget" is absent
    Then the response status code should be 200

  # Each chat-cached call costs $0.00065; a $0.01 budget blocking at 1% is used up by one call.
  @BUDGET-014
  Scenario: A Block budget refuses requests once its block point is spent
    Given I generate a unique resource name from "bgt-block-provider" and store it as "providerName"
    And I generate a unique API version from "bgt-block" and store it as "providerVersion"
    And I generate a unique API context from "/bgt-block" and store it as "providerContext"
    And I generate a unique resource name from "bgt-block" and store it as "budgetName"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}  |
      | name               | ${CTX:providerName}        |
      | displayName        | ${CTX:providerName}        |
      | version            | ${CTX:providerVersion}     |
      | template           | openai                     |
      | spec.context       | ${CTX:providerContext}     |
      | spec.upstream.url  | http://testbench:3008      |
      | accessControl.mode | allow_all                  |
      | spec.policies      | [{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201
    When I create Budget from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "0.01"                    |
      | period              | day                       |
      | action              | block                     |
      | resource.allocation | shared                    |
      | spec.resource.id    | ${CTX:providerName}       |
      | spec.limit.blockAt  | 1                         |
    Then the response status code should be 201
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:budgetName}" of type "Budget" is present
    Then the response status code should be 200
    # A request without a model is not priced, so it shows the route is live without spending.
    And I send a "POST" request to "${CTX:providerContext}/unknown-llm/v1/no-model-field" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-cached" with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200

    # The charge lands when the response ends, so the next request may still be admitted.
    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-cached" until status 429 with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the JSON response field "code" should be "BUDGET_EXCEEDED"
    And the JSON response field "budgets[0].budget" should be "${CTX:budgetName}"
    And the JSON response field "budgets[0].limit" should be "0.01"
    And the JSON response field "budgets[0].period" should be "day"
    And the response header "Retry-After" should exist

  @BUDGET-015
  Scenario: An Allow budget never refuses
    Given I generate a unique resource name from "bgt-allow-provider" and store it as "providerName"
    And I generate a unique API version from "bgt-allow" and store it as "providerVersion"
    And I generate a unique API context from "/bgt-allow" and store it as "providerContext"
    And I generate a unique resource name from "bgt-allow" and store it as "budgetName"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}  |
      | name               | ${CTX:providerName}        |
      | displayName        | ${CTX:providerName}        |
      | version            | ${CTX:providerVersion}     |
      | template           | openai                     |
      | spec.context       | ${CTX:providerContext}     |
      | spec.upstream.url  | http://testbench:3008      |
      | accessControl.mode | allow_all                  |
      | spec.policies      | [{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201
    When I create Budget from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "0.01"                    |
      | period              | day                       |
      | action              | allow                     |
      | resource.allocation | shared                    |
      | spec.resource.id    | ${CTX:providerName}       |
      | spec.limit.blockAt  | 1                         |
    Then the response status code should be 201
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:budgetName}" of type "Budget" is present
    Then the response status code should be 200
    # A request without a model is not priced, so it shows the route is live without spending.
    And I send a "POST" request to "${CTX:providerContext}/unknown-llm/v1/no-model-field" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-cached" with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-cached" with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-cached" with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200

  @BUDGET-016
  Scenario: An inactive budget neither refuses nor reaches the policy engine
    Given I generate a unique resource name from "bgt-inactive-provider" and store it as "providerName"
    And I generate a unique API version from "bgt-inactive" and store it as "providerVersion"
    And I generate a unique API context from "/bgt-inactive" and store it as "providerContext"
    And I generate a unique resource name from "bgt-inactive" and store it as "budgetName"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}  |
      | name               | ${CTX:providerName}        |
      | displayName        | ${CTX:providerName}        |
      | version            | ${CTX:providerVersion}     |
      | template           | openai                     |
      | spec.context       | ${CTX:providerContext}     |
      | spec.upstream.url  | http://testbench:3008      |
      | accessControl.mode | allow_all                  |
      | spec.policies      | [{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201
    When I create Budget from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "0.01"                    |
      | period              | day                       |
      | action              | block                     |
      | resource.allocation | shared                    |
      | spec.resource.id    | ${CTX:providerName}       |
      | spec.limit.blockAt  | 1                         |
      | spec.status         | inactive                  |
    Then the response status code should be 201
    # A request without a model is not priced, so it shows the route is live without spending.
    And I send a "POST" request to "${CTX:providerContext}/unknown-llm/v1/no-model-field" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-cached" with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-cached" with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-cached" with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    When I send a "GET" request to the "policy-engine" service at "/config_dump"
    Then the lazy resources should not contain resource "${CTX:budgetName}" of type "Budget"

  @BUDGET-017 @BUDGET-018
  Scenario: Separate allowances are used up one by one and exempt traffic is never refused
    Given I generate a unique resource name from "bgt-separate-provider" and store it as "providerName"
    And I generate a unique API version from "bgt-separate" and store it as "providerVersion"
    And I generate a unique API context from "/bgt-separate" and store it as "providerContext"
    And I generate a unique resource name from "bgt-separate" and store it as "budgetName"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}  |
      | name               | ${CTX:providerName}        |
      | displayName        | ${CTX:providerName}        |
      | version            | ${CTX:providerVersion}     |
      | template           | openai                     |
      | spec.context       | ${CTX:providerContext}     |
      | spec.upstream.url  | http://testbench:3008      |
      | accessControl.mode | allow_all                  |
      | spec.policies      | [{"name":"llm-cost","version":"v1","paths":[{"path":"/*","methods":["*"]}]}] |
    Then the response status code should be 201
    When I create Budget from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "0.01"                    |
      | period              | day                       |
      | action              | block                     |
      | resource.allocation | shared                    |
      | spec.resource.id    | ${CTX:providerName}       |
      | spec.limit.blockAt  | 1                         |
      | spec.consumers      | [{"type":"attribute","allocation":"separate","expression":"request.Headers[\"x-team\"][0]"}] |
      | spec.exemptions     | [{"type":"attribute","expression":"request.Headers[\"x-budget-exempt\"][0] == \"yes\""}] |
    Then the response status code should be 201
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:budgetName}" of type "Budget" is present
    Then the response status code should be 200
    Given I set header "x-team" to "team-a"
    # A request without a model is not priced, so it shows the route is live without spending.
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

    # Another team has its own allowance.
    Given I set header "x-team" to "team-b"
    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-cached" with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200

    # Exempt traffic is left out of the budget.
    Given I set header "x-team" to "team-a"
    And I set header "x-budget-exempt" to "yes"
    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-cached" with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
