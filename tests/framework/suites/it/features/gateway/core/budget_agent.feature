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

@budget-agent @BUDGET
Feature: Budgets on an agent's LLM proxy
  As a FinOps administrator
  I want budgets to know the project, agent and API key behind each LLM call
  So that I can cap what a project, an agent or one of its keys spends

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # The proxy carries the project-id and agent-id annotations the control plane sets on an
  # agent's proxy. Each chat-cached call costs $0.00065; $0.01 blocking at 10% refuses from
  # $0.001. Counted once, the second call is still admitted (spent $0.00065); counted at both
  # the proxy and the provider route, it would be refused (spent $0.0013).
  @BUDGET-026
  Scenario: Project, agent and API key budgets charge an agent's proxy calls once and leave direct provider calls alone
    Given I generate a unique resource name from "bgt-agent-provider" and store it as "providerName"
    And I generate a unique API version from "bgt-agent-provider" and store it as "providerVersion"
    And I generate a unique API context from "/bgt-agent-provider" and store it as "providerContext"
    And I generate a unique resource name from "bgt-agent-proxy" and store it as "proxyName"
    And I generate a unique API version from "bgt-agent-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/bgt-agent-proxy" and store it as "proxyContext"
    And I generate a unique resource name from "bgt-agent-key" and store it as "keyName"
    And I generate a unique resource name from "bgt-project" and store it as "projectBudget"
    And I generate a unique resource name from "bgt-agent" and store it as "agentBudget"
    And I generate a unique resource name from "bgt-key" and store it as "keyBudget"
    And I generate a unique resource name from "bgt-other-agent" and store it as "otherAgentBudget"
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
      | apiVersion           | ${CTX:gatewaySpecVersion} |
      | name                 | ${CTX:proxyName}          |
      | displayName          | ${CTX:proxyName}          |
      | version              | ${CTX:proxyVersion}       |
      | context              | ${CTX:proxyContext}       |
      | provider.id          | ${CTX:providerName}       |
      | metadata.annotations | {"gateway.api-platform.wso2.com/project-id":"api-platform","gateway.api-platform.wso2.com/agent-id":"support-agent"} |
      | spec.policies        | [{"name":"api-key-auth","version":"v1","paths":[{"path":"/*","methods":["*"],"params":{"key":"API-Key","in":"header"}}]}] |
    Then the response status code should be 201
    When I send a "POST" request to the "gateway-controller" service at "/llm-proxies/${CTX:proxyName}/api-keys" with body:
      """
      {"name": "${CTX:keyName}"}
      """
    Then the response status code should be 201
    And I store the JSON response field "apiKey.apiKey" as "agentKey"

    When I create Budget from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:projectBudget}      |
      | amount              | "0.01"                    |
      | period              | day                       |
      | action              | block                     |
      | resource.allocation | shared                    |
      | spec.resource.id    | ${CTX:providerName}       |
      | spec.limit.blockAt  | 10                        |
      | spec.consumers      | [{"type":"project","allocation":"shared","id":"api-platform"}] |
    Then the response status code should be 201
    When I create Budget from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:agentBudget}        |
      | amount              | "0.01"                    |
      | period              | day                       |
      | action              | block                     |
      | resource.allocation | shared                    |
      | spec.resource.id    | ${CTX:providerName}       |
      | spec.limit.blockAt  | 10                        |
      | spec.consumers      | [{"type":"agent","allocation":"shared","project":"api-platform","id":"support-agent"}] |
    Then the response status code should be 201
    When I create Budget from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:keyBudget}          |
      | amount              | "0.01"                    |
      | period              | day                       |
      | action              | block                     |
      | resource.allocation | shared                    |
      | spec.resource.id    | ${CTX:providerName}       |
      | spec.limit.blockAt  | 10                        |
      | spec.consumers      | [{"type":"apiKey","allocation":"shared","id":"${CTX:keyName}"}] |
    Then the response status code should be 201
    When I create Budget from "resources/templates/budget.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion} |
      | name                | ${CTX:otherAgentBudget}   |
      | amount              | "0.01"                    |
      | period              | day                       |
      | action              | block                     |
      | resource.allocation | shared                    |
      | spec.resource.id    | ${CTX:providerName}       |
      | spec.limit.blockAt  | 1                         |
      | spec.consumers      | [{"type":"agent","allocation":"shared","project":"api-platform","id":"research-agent"}] |
    Then the response status code should be 201
    When I send a "GET" request to the "policy-engine" service at "/config_dump" until lazy resource "${CTX:otherAgentBudget}" of type "Budget" is present
    Then the response status code should be 200

    Given I set header "Content-Type" to "application/json"
    And I set header "API-Key" to "${CTX:agentKey}"
    # A request without a model is not priced, so it shows the route is live without spending.
    And I send a "POST" request to "${CTX:proxyContext}/unknown-llm/v1/no-model-field" until status 200 with body:
      """
      {"messages":[{"role":"user","content":"Hello"}]}
      """
    When I send a "POST" request to "${CTX:proxyContext}/openai/v1/chat-cached" with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    When I send a "POST" request to "${CTX:proxyContext}/openai/v1/chat-cached" with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    When I send a "POST" request to "${CTX:proxyContext}/openai/v1/chat-cached" until status 429 with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the JSON response field "code" should be "BUDGET_EXCEEDED"
    And the JSON response array "budgets" should contain an item with "budget" equal to "${CTX:projectBudget}"
    And the JSON response array "budgets" should contain an item with "budget" equal to "${CTX:agentBudget}"
    And the JSON response array "budgets" should contain an item with "budget" equal to "${CTX:keyBudget}"
    And the JSON response array "budgets" should not contain an item with "budget" equal to "${CTX:otherAgentBudget}"

    # A direct call to the provider names no project, agent or key, so none of them charge it.
    When I clear all headers
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:providerContext}/openai/v1/chat-cached" with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
