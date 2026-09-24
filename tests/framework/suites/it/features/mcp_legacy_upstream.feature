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


@mcp-legacy-upstream
Feature: MCP proxies over a handshake-era upstream
  As an operator
  I want a proxy in front of an older MCP server to keep working exactly as it does today
  So that adopting the 2026-07-28 support does not depend on upstreams adopting it too

  # Every scenario here points at testbench:3013 rather than :3009. The two differ by era, not
  # by tools: 3013 runs the handshake, mints a session, and does not implement server/discover,
  # which is what an MCP server written before 2026-07-28 does. 3009 is stateless and modern and
  # therefore cannot show any of this.
  #
  # The /${CTX:testbenchPartition} segment is not decoration: sessions are state, and the shared
  # testbench hosts a stateful service only when its state is isolated per block. Drop the
  # segment and the request is refused before it reaches the server.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # A stateful server rejects a request that does not carry the session it opened, so the second
  # call below succeeds only if the gateway relayed both the header the server returned and the
  # one the client then sent. Nothing else in the suite covers this, because the shared MCP
  # upstream is stateless and never opens a session at all.
  Scenario: A session opened through the gateway is usable on the next request
    Given I generate a unique resource name from "mcp-legacy-session" and store it as "mcpName"
    And I generate a unique value from "mcp-legacy-session" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-legacy-session" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-legacy-session" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}            |
      | displayName       | ${CTX:mcpDisplayName}     |
      | version           | ${CTX:mcpVersion}         |
      | context           | ${CTX:mcpContext}         |
      | specVersion       | 2025-06-18                |
      | spec.upstream.url | http://testbench:3013/${CTX:testbenchPartition}${CTX:gatewayMCPUpstreamPath} |
    Then the response should be successful
    And I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp" until successful

    When I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the response header "Mcp-Session-Id" should exist
    And I store the response header "Mcp-Session-Id" as "mcpSession"

    When I set header "Mcp-Session-Id" to "${CTX:mcpSession}"
    And I use the MCP Client to send a notifications/initialized notification to "${CTX:mcpContext}/mcp"
    And I use the MCP Client to send a tools/list request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the JSON response should have field "result.tools"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  # The gateway forwards between eras rather than translating between them. A modern client
  # reaching an older server is refused by that server, and the refusal is what the client sees -
  # no session is minted on its behalf, and no handshake is inserted. Pinning it here means a
  # compatibility shim cannot be added later without a test saying so.
  Scenario: A 2026-07-28 request is refused by the upstream rather than translated
    Given I generate a unique resource name from "mcp-legacy-modern" and store it as "mcpName"
    And I generate a unique value from "mcp-legacy-modern" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-legacy-modern" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-legacy-modern" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}            |
      | displayName       | ${CTX:mcpDisplayName}     |
      | version           | ${CTX:mcpVersion}         |
      | context           | ${CTX:mcpContext}         |
      | specVersion       | 2026-07-28                |
      | spec.upstream.url | http://testbench:3013/${CTX:testbenchPartition}${CTX:gatewayMCPUpstreamPath} |
    Then the response should be successful
    And I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp" until successful

    When I use the MCP Client to send a "tools/list" request to "${CTX:mcpContext}/mcp" declaring MCP version "2026-07-28"
    Then the response should be a client error
    # The upstream's own words. A gateway that had translated the request would have produced
    # either a success or a refusal of its own, and neither mentions being stateless.
    And the response body should contain "stateless"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful
