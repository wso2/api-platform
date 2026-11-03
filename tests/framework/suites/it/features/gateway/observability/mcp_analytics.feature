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

@mcp-analytics
Feature: Analytics published for MCP proxies
  As an operator
  I want each MCP request recorded with the operation it invoked
  So that usage is attributed to a tool rather than to one shared route

  # Every JSON-RPC method reaches the same route, so the request URI cannot say which operation
  # ran: without these properties an MCP event is indistinguishable from any other on the proxy.
  # The analytics policy takes them from the operation resolver, which reads the body once for
  # the whole chain, and parses the body itself where no resolver ran. Both paths publish the
  # same properties, so this asserts the published result rather than the path that produced it.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I reset the analytics collector

  Scenario: An MCP event names the JSON-RPC method and the tool it invoked
    Given I generate a unique resource name from "mcp-analytics" and store it as "mcpName"
    And I generate a unique value from "mcp-analytics" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-analytics" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-analytics" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}            |
      | displayName       | ${CTX:mcpDisplayName}     |
      | version           | ${CTX:mcpVersion}         |
      | context           | ${CTX:mcpContext}         |
      | specVersion       | 2025-06-18                |
      | spec.upstream.url | http://testbench:3009${CTX:gatewayMCPUpstreamPath} |
    Then the response should be successful

    # The route answers a moment after the deployment call returns, so the first request is a
    # warm-up that retries rather than an assertion.
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

    # tools/call and the initialize that preceded it share a route, so the properties are what
    # tell the two events apart.
    Then the analytics collector should have received at least 1 event
    And the latest analytics event for path "${CTX:mcpContext}/mcp" should have metadata field "mcpAnalytics.jsonRpcMethod" with value "tools/call"
    And the latest analytics event for path "${CTX:mcpContext}/mcp" should have metadata field "mcpAnalytics.capabilityName" with value "add"
    And the latest analytics event for path "${CTX:mcpContext}/mcp" should have metadata field "mcpAnalytics.capability" with value "TOOL"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  # The modern era mirrors the method and the capability name into headers, and the analytics
  # publisher reads them from there rather than from a parsed body. Same assertions as the
  # scenario above, reached by a different route through the publisher - which is the whole
  # point of running both.
  Scenario: A 2026-07-28 event names the operation from the mirrored headers
    Given I generate a unique resource name from "mcp-analytics-modern" and store it as "mcpName"
    And I generate a unique value from "mcp-analytics-modern" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-analytics-modern" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-analytics-modern" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}            |
      | displayName       | ${CTX:mcpDisplayName}     |
      | version           | ${CTX:mcpVersion}         |
      | context           | ${CTX:mcpContext}         |
      | specVersion       | 2026-07-28                |
      | spec.upstream.url | http://testbench:3009${CTX:gatewayMCPUpstreamPath} |
    Then the response should be successful

    # 2026-07-28 has no handshake, so the warm-up is the call itself, retried until the route is
    # live rather than preceded by an initialize that this era does not define.
    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I set header "MCP-Protocol-Version" to "2026-07-28"
    And I set header "Mcp-Method" to "tools/list"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"warmup","version":"1.0.0"},"io.modelcontextprotocol/clientCapabilities":{}}}}
      """
    And I clear all headers
    And I authenticate using basic auth as "admin"

    When I use the MCP Client to send a "tools/call" request for "add" to "${CTX:mcpContext}/mcp" declaring MCP version "2026-07-28"
    Then the response should be successful

    Then the analytics collector should have received at least 1 event
    And the latest analytics event for path "${CTX:mcpContext}/mcp" should have metadata field "mcpAnalytics.jsonRpcMethod" with value "tools/call"
    And the latest analytics event for path "${CTX:mcpContext}/mcp" should have metadata field "mcpAnalytics.capabilityName" with value "add"
    And the latest analytics event for path "${CTX:mcpContext}/mcp" should have metadata field "mcpAnalytics.capability" with value "TOOL"
    # Only a modern request states its version in the request itself; a legacy one negotiates it.
    And the latest analytics event for path "${CTX:mcpContext}/mcp" should have metadata field "mcpAnalytics.clientInfo.requestedProtocolVersion" with value "2026-07-28"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  # The testbench registers no resources, so the upstream answers this with an error. That is
  # what makes it worth asserting: the request-side properties describe what was asked for, and
  # they have to survive the answer being a failure.
  Scenario: A resources/read event names the resource even when the upstream cannot serve it
    Given I generate a unique resource name from "mcp-analytics-resource" and store it as "mcpName"
    And I generate a unique value from "mcp-analytics-resource" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-analytics-resource" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-analytics-resource" and store it as "mcpContext"
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

    When I use the MCP Client to send this request to "${CTX:mcpContext}/mcp":
      """
      {"jsonrpc":"2.0","id":8,"method":"resources/read","params":{"uri":"file:///absent.txt"}}
      """

    Then the analytics collector should have received at least 1 event
    And the latest analytics event for path "${CTX:mcpContext}/mcp" should have metadata field "mcpAnalytics.jsonRpcMethod" with value "resources/read"
    And the latest analytics event for path "${CTX:mcpContext}/mcp" should have metadata field "mcpAnalytics.capability" with value "RESOURCE"
    And the latest analytics event for path "${CTX:mcpContext}/mcp" should have metadata field "mcpAnalytics.resourceUri" with value "file:///absent.txt"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  # server/discover is the RPC 2026-07-28 introduced, and it is the one call whose answer
  # describes the server rather than a capability. Measured against the upstream directly, its
  # result carries every revision that server serves and the resultType the revision requires -
  # so this asserts the whole path: a modern RPC routed, answered, and read on the way back.
  Scenario: A server/discover event records what the upstream said about itself
    Given I generate a unique resource name from "mcp-analytics-discover" and store it as "mcpName"
    And I generate a unique value from "mcp-analytics-discover" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-analytics-discover" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-analytics-discover" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}            |
      | displayName       | ${CTX:mcpDisplayName}     |
      | version           | ${CTX:mcpVersion}         |
      | context           | ${CTX:mcpContext}         |
      | specVersion       | 2026-07-28                |
      | spec.upstream.url | http://testbench:3009${CTX:gatewayMCPUpstreamPath} |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I set header "MCP-Protocol-Version" to "2026-07-28"
    And I set header "Mcp-Method" to "server/discover"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"warmup","version":"1.0.0"},"io.modelcontextprotocol/clientCapabilities":{}}}}
      """
    And I clear all headers
    And I authenticate using basic auth as "admin"

    When I use the MCP Client to send a "server/discover" request to "${CTX:mcpContext}/mcp" declaring MCP version "2026-07-28"
    Then the response should be successful

    Then the analytics collector should have received at least 1 event
    And the latest analytics event for path "${CTX:mcpContext}/mcp" should have metadata field "mcpAnalytics.jsonRpcMethod" with value "server/discover"
    # The newest revision the server named. Reported only by server/discover: a handshake
    # negotiates one version and can never say what else the server would have served.
    And the latest analytics event for path "${CTX:mcpContext}/mcp" should have metadata field "mcpAnalytics.serverInfo.supportedVersions[0]" with value "2026-07-28"
    # 2026-07-28 requires resultType on every result, and this is the only place the suite sees
    # a real modern response rather than a request.
    And the latest analytics event for path "${CTX:mcpContext}/mcp" should have metadata field "mcpAnalytics.resultType" with value "complete"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful
