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

@mcp-subscriptions
Feature: MCP proxy subscriptions
  As an external control plane managing subscriptions through the gateway management API
  I want MCP proxies to take subscriptions the way REST APIs do
  So that subscription-validation protects and rate limits MCP traffic by plan

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A subscription key opens an MCP proxy, its plan rate limits it, and revoke, resume, plan change, token rotation and delete apply in place
    Given I generate a unique value from "mcp-sub-small" and store it as "smallPlanName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:smallPlanName}","throttleLimitCount":3,"throttleLimitUnit":"Min"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "smallPlanId"
    And I register the "subscription-plan" "${CTX:smallPlanId}" for cleanup
    Given I generate a unique value from "mcp-sub-large" and store it as "largePlanName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:largePlanName}","throttleLimitCount":100,"throttleLimitUnit":"Min"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "largePlanId"
    And I register the "subscription-plan" "${CTX:largePlanId}" for cleanup

    Given I generate a unique resource name from "mcp-sub" and store it as "mcpName"
    And I generate a unique API version from "mcp-sub" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-sub" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                                                              |
      | name                   | ${CTX:mcpName}                                                                         |
      | displayName            | ${CTX:mcpName}                                                                         |
      | version                | ${CTX:mcpVersion}                                                                      |
      | context                | ${CTX:mcpContext}                                                                      |
      | specVersion            | 2025-06-18                                                                             |
      | spec.upstream.url      | http://testbench:3009${CTX:gatewayMCPUpstreamPath}                                     |
      | spec.subscriptionPlans | ["${CTX:smallPlanName}","${CTX:largePlanName}"]                                        |
      | spec.policies          | [{"name":"subscription-validation","version":"v1","params":{"subscriptionKeyHeader":"Subscription-Key"}}] |
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/mcp-proxies/${CTX:mcpName}"
    Then the response status code should be 200
    And the response body should contain "${CTX:smallPlanName}"

    When I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 403 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"it","version":"1.0.0"}}}
      """

    Given I generate a unique value from "mcp-sub" and store it as "subRef"
    And I generate a unique value from "mcp-sub-token" and store it as "subToken"
    And I generate a unique value from "mcp-sub-token-new" and store it as "newToken"
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"externalRefId":"${CTX:subRef}","apiId":"${CTX:mcpName}","subscriptionPlanId":"${CTX:smallPlanId}","subscriptionToken":"${CTX:subToken}"}
      """
    Then the response status should be 201
    And I register the "subscription" "${CTX:subRef}" for cleanup
    When I send a "GET" request to the "gateway-controller" service at "/subscriptions?apiId=${CTX:mcpName}"
    Then the response status should be 200
    And the JSON response field "count" should be 1

    When I set header "Subscription-Key" to "${CTX:subToken}"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"it","version":"1.0.0"}}}
      """
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 429 with body:
      """
      {"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"it","version":"1.0.0"}}}
      """

    When I send a "PUT" request to the "gateway-controller" service at "/subscriptions/${CTX:subRef}" with body:
      """
      {"subscriptionPlanId":"${CTX:largePlanId}"}
      """
    Then the response status should be 200
    And the JSON response field "subscriptionPlanId" should be "${CTX:largePlanId}"
    When I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":3,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"it","version":"1.0.0"}}}
      """

    When I send a "PUT" request to the "gateway-controller" service at "/subscriptions/${CTX:subRef}" with body:
      """
      {"status":"INACTIVE"}
      """
    Then the response status should be 200
    When I send a "POST" request to "${CTX:mcpContext}/mcp" until status 403 with body:
      """
      {"jsonrpc":"2.0","id":4,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"it","version":"1.0.0"}}}
      """
    When I send a "PUT" request to the "gateway-controller" service at "/subscriptions/${CTX:subRef}" with body:
      """
      {"status":"ACTIVE"}
      """
    Then the response status should be 200
    When I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":5,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"it","version":"1.0.0"}}}
      """

    When I send a "PUT" request to the "gateway-controller" service at "/subscriptions/${CTX:subRef}" with body:
      """
      {"subscriptionToken":"${CTX:newToken}"}
      """
    Then the response status should be 200
    When I send a "POST" request to "${CTX:mcpContext}/mcp" until status 403 with body:
      """
      {"jsonrpc":"2.0","id":6,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"it","version":"1.0.0"}}}
      """
    When I set header "Subscription-Key" to "${CTX:newToken}"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":7,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"it","version":"1.0.0"}}}
      """

    When I send a "DELETE" request to the "gateway-controller" service at "/subscriptions/${CTX:subRef}"
    Then the response status should be 204
    And I deregister the "subscription" "${CTX:subRef}" from cleanup
    When I send a "POST" request to "${CTX:mcpContext}/mcp" until status 403 with body:
      """
      {"jsonrpc":"2.0","id":8,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"it","version":"1.0.0"}}}
      """

  Scenario: An MCP proxy accepts only the plans it offers, and deleting it removes its subscriptions
    Given I generate a unique value from "mcp-sub-offered" and store it as "offeredPlanName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:offeredPlanName}","throttleLimitCount":100,"throttleLimitUnit":"Min"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "offeredPlanId"
    And I register the "subscription-plan" "${CTX:offeredPlanId}" for cleanup
    Given I generate a unique value from "mcp-sub-other" and store it as "otherPlanName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:otherPlanName}","throttleLimitCount":100,"throttleLimitUnit":"Min"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "otherPlanId"
    And I register the "subscription-plan" "${CTX:otherPlanId}" for cleanup

    Given I generate a unique resource name from "mcp-sub-plans" and store it as "mcpName"
    And I generate a unique API version from "mcp-sub-plans" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-sub-plans" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                          |
      | name                   | ${CTX:mcpName}                                     |
      | displayName            | ${CTX:mcpName}                                     |
      | version                | ${CTX:mcpVersion}                                  |
      | context                | ${CTX:mcpContext}                                  |
      | specVersion            | 2025-06-18                                         |
      | spec.upstream.url      | http://testbench:3009${CTX:gatewayMCPUpstreamPath} |
      | spec.subscriptionPlans | ["${CTX:offeredPlanName}"]                         |
    Then the response should be successful

    Given I generate a unique value from "mcp-sub-plans-token" and store it as "otherToken"
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"apiId":"${CTX:mcpName}","subscriptionPlanId":"${CTX:otherPlanId}","subscriptionToken":"${CTX:otherToken}"}
      """
    Then the response status should be 400
    And the response body should contain "is not enabled for this API"

    Given I generate a unique value from "mcp-sub-plans" and store it as "subRef"
    And I generate a unique value from "mcp-sub-plans-token-ok" and store it as "subToken"
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"externalRefId":"${CTX:subRef}","apiId":"${CTX:mcpName}","subscriptionPlanId":"${CTX:offeredPlanId}","subscriptionToken":"${CTX:subToken}"}
      """
    Then the response status should be 201
    And I register the "subscription" "${CTX:subRef}" for cleanup

    When I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/subscriptions/${CTX:subRef}" until status 404
    And I deregister the "subscription" "${CTX:subRef}" from cleanup
