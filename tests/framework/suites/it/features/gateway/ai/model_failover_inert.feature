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

@model-failover-inert
Feature: Failover attached to a kind with no model concept
  An Mcp has no model concept at all, but it is transformed by the same transformer as
  every other kind, so the failover configuration genuinely gets built here rather than
  skipped by kind. Every member names a provider, and an Mcp has no upstream definitions
  for one to resolve against, so every member is dropped and the whole configuration
  collapses to nothing - no priority group, no retry policy, nothing on the virtual host.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I reset the analytics collector

  Scenario: Attached to a kind with no model concept, the policy deploys and does nothing
    # Naming a provider on the PRIMARY target is load-bearing. A bare model with no provider
    # resolves to the main upstream, which would leave a one-member chain - accepted and
    # harmless, but not the inert case this scenario is about.
    Given I generate a unique resource name from "inert-mcp" and store it as "mcpName"
    And I generate a unique value from "inert-mcp" and store it as "mcpDisplayName"
    And I generate a unique API version from "inert-mcp" and store it as "mcpVersion"
    And I generate a unique API context from "/inert-mcp" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name              | ${CTX:mcpName}                                                                                                                                                         |
      | displayName       | ${CTX:mcpDisplayName}                                                                                                                                                  |
      | version           | ${CTX:mcpVersion}                                                                                                                                                      |
      | context           | ${CTX:mcpContext}                                                                                                                                                      |
      | specVersion       | 2025-06-18                                                                                                                                                             |
      | spec.upstream.url | http://testbench:3009${CTX:gatewayMCPUpstreamPath}                                                                                                                     |
      | spec.policies     | [{"name":"model-failover","version":"v0","params":{"targets":[{"model":"gpt-4o","provider":"openai-primary","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"anthropic-secondary"}]}]}}] |
    # Accepted, not rejected: an unresolvable target must never block a deployment.
    Then the response should be successful
    And the response should be valid JSON

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    # Behaves exactly as the same proxy does with NO policy attached. This is the identical
    # call and identical expected answer as the policy-free MCP scenario in mcp_deploy -
    # that equivalence IS the requirement.
    When I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    When I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."

    # No failover configuration was generated. A count rather than an absence: the
    # attempt-count header is virtual-host scoped, so requiring it to be missing would
    # assert what else is deployed rather than that this policy stayed inert. Paired with
    # the positive assertions above, so it cannot pass vacuously on a request that never
    # reached the gateway.
    And the response should report one upstream attempt

    # And the request is recorded in analytics exactly as an unpolicied one is: an ordinary
    # event, nothing added and nothing missing.
    #
    # Deliberately NOT asserted: "the event carries no failover keys". A probe established
    # that failover telemetry does not reach the collector even when failover demonstrably
    # happens, so that assertion cannot currently fail and would be pure false confidence.
    # It belongs here once the analytics gap is closed.
    When I wait for an analytics event for path "${CTX:mcpContext}/mcp" with response status 200
    Then the analytics collector should have received at least 1 event

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful
