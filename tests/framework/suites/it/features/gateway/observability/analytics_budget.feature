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

@analytics-budget @BUDGET
Feature: Budget properties on analytics events
  As a FinOps analyst
  I want each LLM request's analytics event to name the budgets it counted against
  So that request-level reports can be filtered by budget

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I reset the analytics collector

  @BUDGET-021
  Scenario: A request a budget covers names the budget and its cost status; others carry no budget
    Given I generate a unique resource name from "bgt-an-provider" and store it as "providerName"
    And I generate a unique API version from "bgt-an" and store it as "providerVersion"
    And I generate a unique API context from "/bgt-an" and store it as "providerContext"
    And I generate a unique resource name from "bgt-an-other" and store it as "otherProviderName"
    And I generate a unique API version from "bgt-an-other" and store it as "otherProviderVersion"
    And I generate a unique API context from "/bgt-an-other" and store it as "otherProviderContext"
    And I generate a unique resource name from "bgt-an" and store it as "budgetName"
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
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}    |
      | name               | ${CTX:otherProviderName}     |
      | displayName        | ${CTX:otherProviderName}     |
      | version            | ${CTX:otherProviderVersion}  |
      | template           | openai                       |
      | spec.context       | ${CTX:otherProviderContext}  |
      | spec.upstream.url  | http://testbench:3008        |
      | accessControl.mode | allow_all                    |
    Then the response status code should be 201
    When I create Budget from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "100"                     |
      | period              | month                     |
      | action              | allow                     |
      | resource.allocation | shared                    |
      | spec.resource.id    | ${CTX:providerName}       |
    Then the response status code should be 201
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:budgetName}" of type "Budget" is present
    Then the response status code should be 200

    Given I set header "Content-Type" to "application/json"
    # A request without a model shows each route is live without spending.
    And I send a "POST" request to "${CTX:providerContext}/unknown-llm/v1/no-model-field" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    And I send a "POST" request to "${CTX:otherProviderContext}/unknown-llm/v1/no-model-field" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    When I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-cached" until its analytics event has metadata field "budget.costStatus" with value "priced" with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the latest analytics event for path "${CTX:providerContext}/openai/v1/chat-cached" should have metadata field "budget.names[0]" with value "${CTX:budgetName}"

    # The other provider has no budget: its events carry no budget property.
    When I send a "POST" request to "${CTX:otherProviderContext}/openai/v1/chat-cached" until its analytics event has metadata field "apiType" with value "LlmProvider" with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the latest analytics event for path "${CTX:otherProviderContext}/openai/v1/chat-cached" should not have metadata field "budget"

  # An LLM proxy call is published once, as the proxy's event; its provider hop, where the
  # budget is counted, is suppressed and hands the budget property over.
  @BUDGET-022
  Scenario: A call through an LLM proxy publishes the budget on the proxy's event
    Given I generate a unique resource name from "bgt-anp-provider" and store it as "providerName"
    And I generate a unique API version from "bgt-anp" and store it as "providerVersion"
    And I generate a unique API context from "/bgt-anp" and store it as "providerContext"
    And I generate a unique resource name from "bgt-anp-proxy" and store it as "proxyName"
    And I generate a unique API version from "bgt-anp-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/bgt-anp-proxy" and store it as "proxyContext"
    And I generate a unique resource name from "bgt-anp" and store it as "budgetName"
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
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion  | ${CTX:gatewaySpecVersion} |
      | name        | ${CTX:proxyName}          |
      | displayName | ${CTX:proxyName}          |
      | version     | ${CTX:proxyVersion}       |
      | context     | ${CTX:proxyContext}       |
      | provider.id | ${CTX:providerName}       |
    Then the response status code should be 201
    When I create Budget from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:budgetName}         |
      | amount              | "100"                     |
      | period              | month                     |
      | action              | allow                     |
      | resource.allocation | shared                    |
      | spec.resource.id    | ${CTX:providerName}       |
    Then the response status code should be 201
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:budgetName}" of type "Budget" is present
    Then the response status code should be 200

    Given I set header "Content-Type" to "application/json"
    # A request without a model shows the route is live without spending.
    And I send a "POST" request to "${CTX:proxyContext}/unknown-llm/v1/no-model-field" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    When I send a "POST" request to "${CTX:proxyContext}/openai/v1/chat-cached" until its analytics event has metadata field "budget.costStatus" with value "priced" with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the latest analytics event for path "${CTX:proxyContext}/openai/v1/chat-cached" should have metadata field "budget.names[0]" with value "${CTX:budgetName}"
    And the latest analytics event for path "${CTX:proxyContext}/openai/v1/chat-cached" should have metadata field "apiType" with value "LlmProxy"
