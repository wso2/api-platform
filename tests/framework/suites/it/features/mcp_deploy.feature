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

@mcp-deploy
Feature: MCP proxy CRUD and connectivity
  As an API developer
  I want to deploy an MCP proxy configuration and connect to it
  So that I can verify the gateway routes MCP requests correctly

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Deploy an MCP proxy and perform a tools/call
    Given I generate a unique resource name from "mcp-deploy" and store it as "mcpName"
    And I generate a unique value from "mcp-deploy" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-deploy" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-deploy" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    When I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    When I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response should have field "result"
    And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."

    When I update MCP proxy "${CTX:mcpName}" from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"

    When I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: An MCP proxy handles a notification and a tools/list request
    Given I generate a unique resource name from "mcp-notify" and store it as "mcpName"
    And I generate a unique value from "mcp-notify" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-notify" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-notify" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    When I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    When I use the MCP Client to send a notifications/initialized notification to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    When I use the MCP Client to send a tools/list request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response should have field "result.tools"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  Scenario: An MCP proxy rejects a tools/call request with invalid params
    Given I generate a unique resource name from "mcp-invalid-tools" and store it as "mcpName"
    And I generate a unique value from "mcp-invalid-tools" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-invalid-tools" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-invalid-tools" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    When I use the MCP Client to send a tools/call request with invalid params to "${CTX:mcpContext}/mcp"
    Then the response status code should be 200
    And the response body should contain "error"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  # ==================== MCP PROXY ERROR CASES ====================

  Scenario: List MCP proxies when none exist
    When I list all MCP proxies
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: List MCP proxies with pagination parameters
    When I send a "GET" request to the "gateway-controller" service at "/mcp-proxies?limit=10&offset=0"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: Get a non-existent MCP proxy returns 404
    When I send a "GET" request to the "gateway-controller" service at "/mcp-proxies/non-existent-mcp-id"
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Get an MCP proxy with an invalid ID format returns 404
    When I send a "GET" request to the "gateway-controller" service at "/mcp-proxies/invalid@mcp#id"
    Then the response status code should be 404
    And the response should be valid JSON

  Scenario: Delete a non-existent MCP proxy returns 404
    When I delete the MCP proxy "non-existent-mcp-delete"
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Update a non-existent MCP proxy returns 404
    Given I generate a unique resource name from "mcp-nonexistent-update" and store it as "mcpName"
    When I update MCP proxy "${CTX:mcpName}" from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | Nonexistent MCP Update              |
      | version           | v1.0                                |
      | context           | /nonexistent-mcp-update             |
      | specVersion       | 2025-06-18                          |
      | spec.upstream.url | http://testbench:3009/mcp           |
    Then the response status code should be 404
    And the response should be valid JSON

  Scenario: Deploy an MCP proxy with labels and verify they are stored
    Given I generate a unique resource name from "mcp-labeled" and store it as "mcpName"
    And I generate a unique value from "mcp-labeled" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-labeled" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-labeled" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion              | gateway.api-platform.wso2.com/v1 |
      | name                    | ${CTX:mcpName}                    |
      | displayName             | ${CTX:mcpDisplayName}             |
      | version                 | ${CTX:mcpVersion}                 |
      | context                 | ${CTX:mcpContext}                 |
      | specVersion             | 2025-06-18                         |
      | spec.upstream.url       | http://testbench:3009/mcp          |
      | metadata.labels         | {"environment":"production","team":"mcp-team","service":"mcp-proxy"} |
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status.state" should be "deployed"

    When I get the MCP proxy "${CTX:mcpName}"
    Then the response should be successful
    And the JSON response field "metadata.labels.environment" should be "production"
    And the JSON response field "metadata.labels.team" should be "mcp-team"
    And the JSON response field "metadata.labels.service" should be "mcp-proxy"

    When I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  Scenario: Deploy an MCP proxy with invalid labels (spaces in keys) fails
    Given I generate a unique resource name from "mcp-invalid-labels" and store it as "mcpName"
    And I generate a unique API version from "mcp-invalid-labels" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-invalid-labels" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | Invalid Labels MCP                  |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | metadata.labels   | {"Invalid Key":"value"}             |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "configuration validation failed"

  # ==================== MCP PROXY ADDITIONAL ERROR CASES ====================

  Scenario: Deploy an MCP proxy with an invalid JSON body returns an error
    When I send a "POST" request to the "gateway-controller" service at "/mcp-proxies" with body:
      """
      { this is not valid json content
      """
    Then the response should be a client error
    And the response should be valid JSON

  # The mcp.yaml template's own placeholders (displayName/version/context/specVersion) are
  # always required by the renderer itself, so a "missing required field" case can only be
  # expressed for a field the template supplies through an omittable dotted-path overlay -
  # upstream is the one such field, covered below. A field the template hardcodes cannot be
  # omitted through this mechanism, so no separate "missing required fields" scenario exists.

  Scenario: Deploy an MCP proxy with an invalid spec version returns 400
    Given I generate a unique resource name from "mcp-invalid-spec-version" and store it as "mcpName"
    And I generate a unique API context from "/mcp-invalid-spec-version" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:mcpName}                    |
      | displayName | Invalid Spec Version MCP            |
      | version     | v1.0                                |
      | context     | ${CTX:mcpContext}                  |
      | specVersion | 2025-03-18                          |
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Deploy an MCP proxy without an upstream returns 400
    Given I generate a unique resource name from "mcp-missing-upstream" and store it as "mcpName"
    And I generate a unique API context from "/mcp-missing-upstream" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:mcpName}                    |
      | displayName | Missing Upstream MCP                |
      | version     | v1.0                                |
      | context     | ${CTX:mcpContext}                  |
      | specVersion | 2025-06-18                          |
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Deploying a duplicate MCP proxy returns a conflict
    Given I generate a unique resource name from "mcp-duplicate" and store it as "mcpName"
    And I generate a unique value from "mcp-duplicate" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-duplicate" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-duplicate" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
    Then the response should be successful

    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
    Then the response status code should be 409
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

    When I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  Scenario: Update an MCP proxy with an invalid JSON body returns an error
    When I send a "PUT" request to the "gateway-controller" service at "/mcp-proxies/some-mcp" with body:
      """
      { invalid json body
      """
    Then the response should be a client error
    And the response should be valid JSON

  # ==================== MCP PROXY FILTER TESTS ====================

  Scenario: List MCP proxies filtered by displayName
    Given I generate a unique resource name from "mcp-filter" and store it as "mcpName"
    And I generate a unique value from "UniqueMCPFilterTest" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-filter" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-filter" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
    Then the response should be successful

    When I send a "GET" request to the "gateway-controller" service at "/mcp-proxies?displayName=${CTX:mcpDisplayName}"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "${CTX:mcpDisplayName}"

    When I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  Scenario: List MCP proxies filtered by version
    Given I generate a unique resource name from "mcp-version-filter" and store it as "mcpName"
    And I generate a unique value from "mcp-version-filter" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-version-filter" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-version-filter" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
    Then the response should be successful

    When I send a "GET" request to the "gateway-controller" service at "/mcp-proxies?version=${CTX:mcpVersion}"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

    When I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful
