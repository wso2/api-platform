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

@backend-timeout @timeouts
Feature: Upstream and downstream timeouts
  As an API developer
  I want gateway timeout settings to be enforced
  So that unreachable upstreams and incomplete downstream requests fail predictably

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: RestApi backend timeout uses the upstream definition
    Given I generate a unique value from "timeout-rest-api" and store it as "apiName1"
    And I generate a unique API context from "/timeout-rest-api" and store it as "apiContext1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                    | gateway.api-platform.wso2.com/v1                                                                      |
      | name                          | ${CTX:apiName1}                                                                                     |
      | spec.displayName              | Timeout API                                                                                            |
      | spec.version                  | v1.0                                                                                                   |
      | spec.context                  | ${CTX:apiContext1}/$version                                                                           |
      | spec.upstreamDefinitions      | [{"name":"timeout-upstream","timeout":{"connect":"8000ms"},"upstreams":[{"url":"http://192.0.2.1:8080"}]}] |
      | spec.upstream.main.ref        | timeout-upstream                                                                                      |
      | spec.operations               | [{"method":"GET","path":"/"}]                                                                  |
    Then the response should be successful
    When I send a "GET" request to "${CTX:apiContext1}/v1.0/" until it times out after "8" seconds with status 503
    When I delete the API "${CTX:apiName1}"
    Then the response should be successful

  Scenario: RestApi backend timeout uses the gateway global default
    Given I generate a unique value from "timeout-global-api" and store it as "apiName2"
    And I generate a unique API context from "/timeout-global-api" and store it as "apiContext2"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                    | gateway.api-platform.wso2.com/v1                                                                     |
      | name                          | ${CTX:apiName2}                                                                                     |
      | spec.displayName              | Global Timeout API                                                                                   |
      | spec.version                  | v1.0                                                                                                  |
      | spec.context                  | ${CTX:apiContext2}/$version                                                                          |
      | spec.upstreamDefinitions      | [{"name":"global-timeout-upstream","upstreams":[{"url":"http://192.0.2.1:8080"}]}]          |
      | spec.upstream.main.ref        | global-timeout-upstream                                                                              |
      | spec.operations               | [{"method":"GET","path":"/"}]                                                                 |
    Then the response should be successful
    When I send a "GET" request to "${CTX:apiContext2}/v1.0/" until it times out after "5" seconds with status 503
    When I delete the API "${CTX:apiName2}"
    Then the response should be successful

  Scenario: HTTP connection manager rejects incomplete request headers
    Given I generate a unique value from "headers-timeout-api" and store it as "apiName3"
    And I generate a unique API context from "/headers-timeout-api" and store it as "apiContext3"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                    | gateway.api-platform.wso2.com/v1                                                        |
      | name                          | ${CTX:apiName3}                                                                        |
      | spec.displayName              | Headers Timeout API                                                                      |
      | spec.version                  | v1.0                                                                                     |
      | spec.context                  | ${CTX:apiContext3}/$version                                                             |
      | spec.upstreamDefinitions      | [{"name":"headers-timeout-upstream","upstreams":[{"url":"http://testbench:3000"}]}] |
      | spec.upstream.main.ref        | headers-timeout-upstream                                                               |
      | spec.operations               | [{"method":"GET","path":"/"}]                                                     |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext3}/v1.0/" until status 200
    When I send an incomplete HTTP request to "${CTX:apiContext3}/v1.0/"
    Then the response status code should be 408
    And the gateway should have timed out after "4" seconds with status 408
    When I delete the API "${CTX:apiName3}"
    Then the response should be successful

  Scenario: LLM provider backend timeout uses its upstream definition
    Given I generate a unique resource name from "llm-connect-timeout-provider" and store it as "providerName4"
    And I generate a unique API context from "/llm-connect-timeout" and store it as "providerContext4"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion                    | gateway.api-platform.wso2.com/v1                                      |
      | name                          | ${CTX:providerName4}                                                  |
      | displayName                   | LLM Connect Timeout Provider                                           |
      | version                       | v1.0                                                                   |
      | template                      | openai                                                                 |
      | context                       | ${CTX:providerContext4}                                                |
      | spec.upstreamDefinitions      | [{"name":"llm-timeout-upstream","timeout":{"connect":"8000ms"},"upstreams":[{"url":"http://192.0.2.1:8080"}]}] |
      | spec.upstream.ref             | llm-timeout-upstream                                                   |
      | accessControl.mode            | allow_all                                                               |
    Then the response status code should be 201
    When I send a "GET" request to "${CTX:providerContext4}/get" until it times out after "8" seconds with status 503
    When I delete the LLM provider "${CTX:providerName4}"
    Then the response should be successful

  Scenario: MCP backend timeout uses its upstream definition
    Given I generate a unique resource name from "mcp-connect-timeout" and store it as "mcpName5"
    And I generate a unique API context from "/mcp-connect-timeout" and store it as "mcpContext5"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion                 | gateway.api-platform.wso2.com/v1                                                                                         |
      | name                       | ${CTX:mcpName5}                                                                                                          |
      | displayName                | MCP Connect Timeout                                                                                                     |
      | version                    | v1.0                                                                                                                     |
      | context                    | ${CTX:mcpContext5}                                                                                                       |
      | specVersion                | 2025-06-18                                                                                                               |
      | spec.upstreamDefinitions   | [{"name":"mcp-timeout-upstream","timeout":{"connect":"8000ms"},"upstreams":[{"url":"http://192.0.2.1:3001"}]}] |
      | spec.upstream.ref          | mcp-timeout-upstream                                                                                                     |
    Then the response should be successful
    When I send a "GET" request to "${CTX:mcpContext5}/mcp" until it times out after "8" seconds with status 503
    When I delete the MCP proxy "${CTX:mcpName5}"
    Then the response should be successful
