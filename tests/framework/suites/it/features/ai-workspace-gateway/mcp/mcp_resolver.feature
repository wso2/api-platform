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


@mcp-resolver
Feature: The MCP operation resolver
  As an operator
  I want the gateway to read an MCP request body once, before any policy runs
  So that a request is governed by the operation the upstream will actually execute

  # No policy is attached in any scenario here. That is the point: these describe what the
  # gateway itself does with an MCP body, separately from what a policy then decides about it.
  #
  # Each scenario has to say which side answered, because the upstream speaks JSON-RPC and the
  # gateway does not: a gateway refusal is a plain {"error","error_id"} document with an
  # x-error-id header, while anything carrying "jsonrpc" came from the MCP server.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # The ceiling exists because a body-resolved route is parsed before any authentication policy
  # has run, so an unbounded body would be work done for an unauthenticated caller.
  Scenario: A request body over the resolver's ceiling is refused by the gateway
    Given I generate a unique resource name from "mcp-oversized" and store it as "mcpName"
    And I generate a unique value from "mcp-oversized" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-oversized" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-oversized" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}            |
      | displayName       | ${CTX:mcpDisplayName}     |
      | version           | ${CTX:mcpVersion}         |
      | context           | ${CTX:mcpContext}         |
      | specVersion       | 2025-06-18                |
      | spec.upstream.url | http://testbench:3009${CTX:gatewayMCPUpstreamPath} |
    Then the response should be successful
    And I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp" until successful

    When I use the MCP Client to send a "tools/call" request of 65 KiB to "${CTX:mcpContext}/mcp"
    Then the response status code should be 413
    # The gateway's own refusal shape, which the upstream could not have produced.
    And the response header "x-error-id" should exist
    And the JSON response field "error" should be "Payload Too Large"
    And the response body should not contain "jsonrpc"

    When I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  # Without this the scenario above would prove only that some large request failed, not that a
  # ceiling decided it.
  Scenario: A request body under the resolver's ceiling reaches the upstream
    Given I generate a unique resource name from "mcp-large" and store it as "mcpName"
    And I generate a unique value from "mcp-large" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-large" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-large" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}            |
      | displayName       | ${CTX:mcpDisplayName}     |
      | version           | ${CTX:mcpVersion}         |
      | context           | ${CTX:mcpContext}         |
      | specVersion       | 2025-06-18                |
      | spec.upstream.url | http://testbench:3009${CTX:gatewayMCPUpstreamPath} |
    Then the response should be successful
    And I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp" until successful

    When I use the MCP Client to send a "tools/call" request of 60 KiB to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the JSON response should have field "result"

    When I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  # The resolver reports that it could not read this body unambiguously; it does not refuse it.
  # Refusing is a policy's decision, and with no policy attached there is nobody to make it - so
  # the request runs. mcp_policies_modern.feature asserts the same body being rejected once
  # mcp-spec-validation is attached, and the pair is what shows the split is deliberate.
  Scenario: An ambiguous request body is reported, not refused, when no policy governs it
    Given I generate a unique resource name from "mcp-ambiguous" and store it as "mcpName"
    And I generate a unique value from "mcp-ambiguous" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-ambiguous" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-ambiguous" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}            |
      | displayName       | ${CTX:mcpDisplayName}     |
      | version           | ${CTX:mcpVersion}         |
      | context           | ${CTX:mcpContext}         |
      | specVersion       | 2025-06-18                |
      | spec.upstream.url | http://testbench:3009${CTX:gatewayMCPUpstreamPath} |
    Then the response should be successful
    And I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp" until successful

    # Two spellings of the same member: encoding/json folds them, so the gateway and a backend
    # that matches exactly can read different operations out of these same bytes.
    When I use the MCP Client to send this request to "${CTX:mcpContext}/mcp":
      """
      {"jsonrpc":"2.0","id":1,"method":"tools/list","Method":"tools/call","params":{}}
      """
    Then the response should be successful
    # A JSON-RPC envelope carrying the tool list: the upstream answered, so nothing stopped it.
    And the JSON response should have field "result.tools"

    When I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful
