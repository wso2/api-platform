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

@ap-cli-gateway
Feature: ap CLI MCP proxy commands
  As a CLI user
  I want to manage MCP proxies on the gateway
  So that I can generate, list, view, and delete MCP configurations

  Background:
    Given the "ap" CLI is available

  @legacy:GW-MCP-001
  Scenario: Generate an MCP configuration from a reachable server
    Given I resolve the "mcp" service host URL at "/mcp" and store it as "mcpServerURL"
    When I run ap with arguments "gateway mcp generate --server ${CTX:mcpServerURL} --output mcp-generated"
    Then the CLI command should succeed

  @legacy:GW-MCP-002
  Scenario: Generating an MCP configuration from an invalid server URL is rejected
    When I run ap with arguments "gateway mcp generate --server not-a-valid-url --output mcp-generated"
    Then the CLI command should fail
    And the CLI output should contain "unsupported protocol"

  @legacy:GW-MCP-003
  Scenario: Generating an MCP configuration from an unreachable server fails
    When I run ap with arguments "gateway mcp generate --server http://127.0.0.1:1/mcp --output mcp-generated"
    Then the CLI command should fail
    And the CLI output should contain "connection refused"

  @legacy:GW-MCP-004
  Scenario: List MCP proxies
    Given the CLI uses the platform gateway as "mcp-gw"
    And I generate a unique resource name from "cli-mcp" and store it as "mcpName"
    And I generate a unique API version from "cli-mcp" and store it as "mcpVersion"
    And I generate a unique API context from "/cli-mcp" and store it as "mcpContext"
    And I apply MCP proxy from "resources/templates/mcp.yaml" through the CLI with values:
      | apiVersion        | ${CTX:gatewaySpecVersion}                         |
      | name              | ${CTX:mcpName}                                    |
      | displayName       | ${CTX:mcpName}                                    |
      | version           | ${CTX:mcpVersion}                                 |
      | context           | ${CTX:mcpContext}                                 |
      | specVersion       | 2025-06-18                                        |
      | spec.upstream.url | http://testbench:3009${CTX:gatewayMCPUpstreamPath} |
    And the CLI command should succeed
    When I run ap with arguments "gateway mcp list"
    Then the CLI command should succeed
    And the CLI output should contain "${CTX:mcpName}"

  # Other runners deploy MCP proxies to the same gateway, so an empty list cannot be asserted.
  @legacy:GW-MCP-005
  Scenario: List MCP proxies without deploying one
    Given the CLI uses the platform gateway as "mcp-gw"
    When I run ap with arguments "gateway mcp list"
    Then the CLI command should succeed

  @legacy:GW-MCP-006
  Scenario: Get an existing MCP proxy
    Given the CLI uses the platform gateway as "mcp-gw"
    And I generate a unique resource name from "cli-mcp" and store it as "mcpName"
    And I generate a unique API version from "cli-mcp" and store it as "mcpVersion"
    And I generate a unique API context from "/cli-mcp" and store it as "mcpContext"
    And I apply MCP proxy from "resources/templates/mcp.yaml" through the CLI with values:
      | apiVersion        | ${CTX:gatewaySpecVersion}                         |
      | name              | ${CTX:mcpName}                                    |
      | displayName       | ${CTX:mcpName}                                    |
      | version           | ${CTX:mcpVersion}                                 |
      | context           | ${CTX:mcpContext}                                 |
      | specVersion       | 2025-06-18                                        |
      | spec.upstream.url | http://testbench:3009${CTX:gatewayMCPUpstreamPath} |
    And the CLI command should succeed
    When I run ap with arguments "gateway mcp get --id ${CTX:mcpName}"
    Then the CLI command should succeed
    And the CLI output should contain "${CTX:mcpName}"

  @legacy:GW-MCP-007
  Scenario: Getting an MCP proxy that does not exist is rejected
    Given the CLI uses the platform gateway as "mcp-gw"
    And I generate a unique resource name from "cli-missing-mcp" and store it as "missingMcp"
    When I run ap with arguments "gateway mcp get --id ${CTX:missingMcp}"
    Then the CLI command should fail
    And the CLI output should contain "not found"

  @legacy:GW-MCP-008
  Scenario: Delete an existing MCP proxy
    Given the CLI uses the platform gateway as "mcp-gw"
    And I generate a unique resource name from "cli-mcp" and store it as "mcpName"
    And I generate a unique API version from "cli-mcp" and store it as "mcpVersion"
    And I generate a unique API context from "/cli-mcp" and store it as "mcpContext"
    And I apply MCP proxy from "resources/templates/mcp.yaml" through the CLI with values:
      | apiVersion        | ${CTX:gatewaySpecVersion}                         |
      | name              | ${CTX:mcpName}                                    |
      | displayName       | ${CTX:mcpName}                                    |
      | version           | ${CTX:mcpVersion}                                 |
      | context           | ${CTX:mcpContext}                                 |
      | specVersion       | 2025-06-18                                        |
      | spec.upstream.url | http://testbench:3009${CTX:gatewayMCPUpstreamPath} |
    And the CLI command should succeed
    When I run ap with arguments "gateway mcp delete --id ${CTX:mcpName}"
    Then the CLI command should succeed

  @legacy:GW-MCP-009
  Scenario: Deleting an MCP proxy that does not exist is rejected
    Given the CLI uses the platform gateway as "mcp-gw"
    And I generate a unique resource name from "cli-missing-mcp" and store it as "missingMcp"
    When I run ap with arguments "gateway mcp delete --id ${CTX:missingMcp}"
    Then the CLI command should fail
    And the CLI output should contain "not found"
