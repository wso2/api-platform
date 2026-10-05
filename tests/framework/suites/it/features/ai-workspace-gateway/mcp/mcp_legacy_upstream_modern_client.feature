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


@mcp-legacy-upstream-modern-client
Feature: A modern MCP client against a handshake-era upstream
  As an operator
  I want a 2026-07-28 client reaching an older MCP server to see that server's own refusal
  So that the gateway is understood to forward between eras rather than translate between them

  # Separate from mcp_legacy_upstream.feature because this scenario deploys a 2026-07-28 proxy,
  # which only a Gateway newer than 1.2.0 accepts; its runner carries that version gate while the
  # sibling feature's scenarios run against every supported release.
  #
  # The upstream is testbench:3014 rather than :3009, and the two differ by era, not by tools:
  # 3014 runs the handshake, mints a session, and does not implement server/discover, which is
  # what an MCP server written before 2026-07-28 does. The /${CTX:testbenchPartition} segment is
  # not decoration: sessions are state, and the shared testbench hosts a stateful service only
  # when its state is isolated per block.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

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
      | spec.upstream.url | http://testbench:3014/${CTX:testbenchPartition}${CTX:gatewayMCPUpstreamPath} |
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
