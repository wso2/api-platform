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


@mcp-spec-versions-cp
Feature: MCP spec versions from the control plane to the gateway
  As an API platform operator
  I want the MCP revisions I declare in the control plane to arrive at the gateway
  So that a proxy in front of a dual-era server is described the same way on both sides

  # NOT WIRED TO A RUNNER YET, deliberately. Every scenario here needs control-plane support
  # that is not on this branch's base: platform-api has no mcpSpecVersions field, and its
  # discovery probe reports no versions at all, so a proxy declaring a list never reaches the
  # gateway and a fetch returns nothing. Measured, not assumed - the runner was added, run, and
  # removed after all three scenarios failed for exactly those reasons.
  #
  # Wiring it is one runner block in it-suite.yaml once the platform-api change lands, and the
  # last scenario additionally needs server/discover support, which is why it carries its own
  # tag.
  #
  # The two halves of the platform hold this value in different shapes, and the hand-off between
  # them is the only place it can be lost. It has been lost before: a gateway that did not know
  # the field discarded it at unmarshal, the deployment succeeded, and both sides reported
  # success while the proxy ran on a default. Nothing in either half's own tests could see that,
  # which is why it is asserted here rather than there.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "mcp-cp-versions-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  Scenario: Spec versions declared in the control plane reach the gateway
    Given I generate a unique resource name from "mcp-cp-versions" and store it as "mcpId"
    And I generate a unique API context from "/mcp-cp-versions" and store it as "mcpContext"
    When I create an MCP proxy "${CTX:mcpId}" via the control plane with context "${CTX:mcpContext}" and spec versions "2025-06-18,2026-07-28"
    And I deploy the "Mcp" "${CTX:mcpId}" to the gateway via the control plane
    Then I send a "GET" request to the "gateway-controller" service at "/mcp-proxies/${CTX:mcpId}" until status 200

    # Read from the gateway's own copy, not from the response the control plane gave us: the
    # question is what survived the hand-off.
    And the stored Mcp configuration for "${CTX:mcpId}" should contain:
      """
      2025-06-18
      """
    And the stored Mcp configuration for "${CTX:mcpId}" should contain:
      """
      2026-07-28
      """

  # upstreamMcpSpecVersions records what a server said about itself when it was discovered. It
  # restricts nothing, so it has no business on a gateway - and the deployment spec is an
  # allow-list that omits it. An allow-list is exactly the kind of guarantee that changes
  # silently when someone adds a field, so it is worth one line.
  Scenario: What the upstream reported at discovery stays in the control plane
    Given I generate a unique resource name from "mcp-cp-upstream" and store it as "mcpId"
    And I generate a unique API context from "/mcp-cp-upstream" and store it as "mcpContext"
    When I create an MCP proxy "${CTX:mcpId}" via the control plane with context "${CTX:mcpContext}" and spec versions "2025-06-18"
    And I deploy the "Mcp" "${CTX:mcpId}" to the gateway via the control plane
    Then I send a "GET" request to the "gateway-controller" service at "/mcp-proxies/${CTX:mcpId}" until status 200
    # Paired with a presence assertion, because an absence alone is also satisfied by reading
    # the wrong row, or by reading the right one before the deployment wrote it.
    And the stored Mcp configuration for "${CTX:mcpId}" should contain:
      """
      2025-06-18
      """
    And the stored Mcp configuration for "${CTX:mcpId}" should not contain:
      """
      upstreamMcpSpecVersions
      """
    And the stored Mcp configuration for "${CTX:mcpId}" should not contain:
      """
      upstreamSpecVersions
      """

  # Discovery against a server that does not implement server/discover has to fall back to the
  # initialize handshake. The control plane's own tests prove that branch against stubs written
  # alongside it; this proves it against a real server that genuinely lacks the method, which is
  # the case that has produced surprises before.
  Scenario: Discovery falls back to the handshake against a server without server/discover
    When I fetch MCP server info via the control plane for "http://testbench:3013/${CTX:testbenchPartition}/mcp"
    Then the fetched MCP server info should report spec version "2025-06-18"

  # A modern server answers server/discover with every revision it serves, which no handshake
  # can report. Needs the control-plane discovery change, so it is held back by its own tag
  # until that merges - see the runner in it-suite.yaml.
  @pending-platform-api-discovery
  Scenario: Discovery reports every revision a modern server serves
    When I fetch MCP server info via the control plane for "http://testbench:3009/mcp"
    Then the fetched MCP server info should report spec version "2026-07-28"
