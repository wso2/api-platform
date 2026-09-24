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

@mcp-policies
Feature: MCP proxy behavior under attached policies
  As an API developer
  I want to attach policies to an MCP proxy
  So that I can verify the proxy enforces them correctly

  # What is left here belongs to no single MCP policy: attaching one that does not exist, and a
  # policy that is not MCP-specific. Each MCP policy has its own file - mcp_auth.feature,
  # mcp_authz.feature, mcp_ratelimit.feature, mcp_acl_list.feature, mcp_rewrite.feature and
  # mcp_spec_validation.feature - each holding that policy's behaviour in both protocol eras.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Deploying an MCP proxy with a non-existing policy fails
    Given I generate a unique resource name from "mcp-nonexistent-policy" and store it as "mcpName"
    And I generate a unique API context from "/mcp-nonexistent-policy" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}                    |
      | displayName       | MCP Non-Existing Policy Test        |
      | version           | v1.0                                |
      | context           | ${CTX:mcpContext}                  |
      | specVersion       | 2025-06-18                          |
      | spec.upstream.url | http://testbench:3009/mcp           |
      | spec.policies     | [{"name":"non-existing-policy","version":"v1","params":{}}] |
    Then the response status code should be 400
    And the response should be valid JSON

  Scenario: An MCP proxy with cors handles preflight and disallowed-origin requests
    Given I generate a unique resource name from "mcp-cors" and store it as "mcpName"
    And I generate a unique value from "mcp-cors" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-cors" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-cors" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"cors","version":"v1","params":{"allowedOrigins":["http://example.com"],"allowedMethods":["GET","POST"],"allowedHeaders":["Content-Type"],"exposedHeaders":["X-Custom-Header"]}}] |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    # Preflight request from allowed origin
    When I clear all headers
    And I set header "Origin" to "http://example.com"
    And I set header "Access-Control-Request-Method" to "POST"
    And I set header "Access-Control-Request-Headers" to "Content-Type"
    And I send a "OPTIONS" request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should be "http://example.com"
    And the response header "Access-Control-Allow-Methods" should contain "POST"
    And the response header "Access-Control-Allow-Headers" should contain "Content-Type"

    # Preflight request from disallowed origin should not return CORS headers
    When I set header "Origin" to "http://evil.com"
    And I set header "Access-Control-Request-Method" to "POST"
    And I set header "Access-Control-Request-Headers" to "Content-Type"
    And I send a "OPTIONS" request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should not exist

    # Preflight request from an origin with a disallowed suffix should not return CORS headers
    When I set header "Origin" to "http://example.com.evil.com"
    And I set header "Access-Control-Request-Method" to "POST"
    And I set header "Access-Control-Request-Headers" to "Content-Type"
    And I send a "OPTIONS" request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should not exist

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful
