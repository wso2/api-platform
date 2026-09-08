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
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

@startup-db-bootstrap
Feature: Startup database bootstrap
  As a gateway operator
  I want the gateway-controller to restore persisted configs from the database on startup
  So that control-plane sync failures do not prevent already stored configs from becoming active

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Restarted gateway-controller restores persisted resources from the database
    Given I generate a unique resource name from "startup-db-mcp-v1" and store it as "mcpName"
    And I generate a unique API context from "/startup-db-mcp" and store it as "mcpContext"
    Given I generate a unique resource name from "startup-db-llm-provider" and store it as "resourceName1_1"
    Given I generate a unique API context from "/startup-db-llm" and store it as "resourceContext1_1"
    Given I generate a unique resource name from "startup-db-llm-proxy" and store it as "resourceName1_2"
    Given I generate a unique API context from "/startup-db-proxy" and store it as "resourceContext1_2"
    Given I generate a unique resource name from "startup-db-rest-api" and store it as "resourceName1_3"
    Given I generate a unique API context from "/startup-db-rest" and store it as "resourceContext1_3"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | gateway.api-platform.wso2.com/v1 |
      | name               | ${CTX:resourceName1_1}           |
      | displayName        | Startup DB LLM Provider           |
      | version            | v1.0                              |
      | template           | openai                            |
      | spec.context        | ${CTX:resourceContext1_1}         |
      | spec.upstream.url  | http://testbench:3008            |
      | spec.upstream.auth | {"type":"api-key","header":"Authorization","value":"Bearer sk-startup-db-test"} |
      | accessControl.mode | allow_all                         |
    Then the response status code should be 201

    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:resourceName1_2}            |
      | displayName | Startup DB LLM Proxy             |
      | version     | v1.0                              |
      | context     | ${CTX:resourceContext1_2}         |
      | provider.id | ${CTX:resourceName1_1}            |
    Then the response status should be 201

    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion   | gateway.api-platform.wso2.com/v1 |
      | name         | ${CTX:mcpName}                   |
      | displayName  | Startup DB MCP                   |
      | version      | v1.0                              |
      | context      | ${CTX:mcpContext}                 |
      | specVersion  | 2025-06-18                        |
      | spec.upstream.url | http://testbench:3009/mcp        |
    Then the response should be successful
    And the response should be valid JSON
    And the resource creation response should indicate successful deployment

    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1 |
      | name                  | ${CTX:resourceName1_3}           |
      | spec.displayName      | Startup DB Rest API              |
      | spec.version          | v1.0                              |
      | spec.context          | ${CTX:resourceContext1_3}/$version |
      | spec.upstream.main.url | http://testbench:3000/api/v2     |
      | spec.operations       | [{"method":"GET","path":"/{country_code}/{city}"}] |
    Then the response should be successful
    And the response should be valid JSON
    And the resource creation response should indicate successful deployment

    When I clear all headers
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:resourceContext1_1}/chat/completions" until status 200 with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "warmup"}]
      }
      """
    And I send a "POST" request to "${CTX:resourceContext1_2}/chat/completions" until status 200 with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "proxy warmup"}]
      }
      """
    And I send a "GET" request to "${CTX:resourceContext1_3}/v1.0/us/seattle" until status 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:resourceContext1_1}/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "before restart"}]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "object" should be "chat.completion"

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:resourceContext1_2}/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "proxy before restart"}]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "object" should be "chat.completion"

    When I clear all headers
    And I send a "GET" request to "${CTX:resourceContext1_3}/v1.0/us/seattle"
    Then the response status code should be 200
    And the response body should contain "/api/v2/us/seattle"

    When I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    When I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."

    When I restart the "gateway-controller" service
    And I wait for the gateway controller health endpoint
    And I clear all headers
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:resourceContext1_1}/chat/completions" until status 200 with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "after restart warmup"}]
      }
      """
    And I send a "POST" request to "${CTX:resourceContext1_2}/chat/completions" until status 200 with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "proxy after restart warmup"}]
      }
      """
    And I send a "GET" request to "${CTX:resourceContext1_3}/v1.0/us/seattle" until status 200

    Given I authenticate using basic auth as "admin"
    When I send a "GET" request to the "gateway-controller-admin" service at "/config_dump"
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should contain "${CTX:resourceName1_1}"
    And the response body should contain "${CTX:resourceName1_2}"
    And the response body should contain "${CTX:mcpName}"
    And the response body should contain "${CTX:resourceName1_3}"
    And the response body should contain "startup-db-llm"
    And the response body should contain "startup-db-proxy"
    And the response body should contain "startup-db-mcp"
    And the response body should contain "startup-db-rest"

    When I clear all headers
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:resourceContext1_1}/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "after restart"}]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "object" should be "chat.completion"

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:resourceContext1_2}/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "proxy after restart"}]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "object" should be "chat.completion"

    When I clear all headers
    And I send a "GET" request to "${CTX:resourceContext1_3}/v1.0/us/seattle"
    Then the response status code should be 200
    And the response body should contain "/api/v2/us/seattle"

    When I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    When I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."

    Given I authenticate using basic auth as "admin"
    When I send a "DELETE" request to the "gateway-controller" service at "/llm-proxies/${CTX:resourceName1_2}"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

    Given I authenticate using basic auth as "admin"
    When I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

    Given I authenticate using basic auth as "admin"
    When I delete the API "${CTX:resourceName1_3}"
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "${CTX:resourceName1_1}"
    Then the response status code should be 200
