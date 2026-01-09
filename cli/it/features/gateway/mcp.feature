# --------------------------------------------------------------------
# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

Feature: Gateway MCP Management Commands
    As a CLI user
    I want to manage MCPs on the gateway
    So that I can generate, list, view, and delete MCP configurations

    Background:
        Given the CLI is available

    # =========================================
    # MCP Generate Tests
    # =========================================

    @MCP-001
    Scenario: Generate MCP config from valid server
        Given the MCP server is running
        When I run ap with arguments "gateway mcp generate --server http://localhost:3001/mcp --output /tmp/mcp-test-output"
        Then the command should succeed

    @MCP-002
    Scenario: Generate MCP config with invalid server URL
        When I run ap with arguments "gateway mcp generate --server not-a-valid-url --output /tmp/mcp-test-output"
        Then the command should fail
        And the output should contain "unsupported protocol"

    @MCP-003
    Scenario: Generate MCP config from unreachable server
        When I run ap with arguments "gateway mcp generate --server http://localhost:19999/mcp --output /tmp/mcp-test-output"
        Then the command should fail
        And the output should contain "connection refused"

    # =========================================
    # MCP List Tests
    # =========================================

    @MCP-004
    Scenario: List MCPs
        Given the gateway is running
        And I have a gateway named "mcp-list-gw" configured
        And I set the current gateway to "mcp-list-gw"
        And I apply the resource file "resources/gateway/sample-mcp-config.yaml"
        When I run ap with arguments "gateway mcp list"
        Then the command should succeed

    @MCP-005
    Scenario: List MCPs when empty
        Given the gateway is running
        And I have a gateway named "mcp-empty-gw" configured
        And I set the current gateway to "mcp-empty-gw"
        When I run ap with arguments "gateway mcp list"
        Then the command should succeed

    # =========================================
    # MCP Get Tests
    # =========================================

    @MCP-006
    Scenario: Get existing MCP
        Given the gateway is running
        And I have a gateway named "mcp-get-gw" configured
        And I set the current gateway to "mcp-get-gw"
        And I apply the resource file "resources/gateway/sample-mcp-config.yaml"
        When I run ap with arguments "gateway mcp get --id test-mcp-v1.0"
        Then the command should succeed
        And the output should contain "test-mcp"

    @MCP-007
    Scenario: Get non-existent MCP
        Given the gateway is running
        And I have a gateway named "mcp-get-fail-gw" configured
        And I set the current gateway to "mcp-get-fail-gw"
        When I run ap with arguments "gateway mcp get --id non-existent-mcp"
        Then the command should fail
        And the output should contain "not found"

    # =========================================
    # MCP Delete Tests
    # =========================================

    @MCP-008
    Scenario: Delete existing MCP
        Given the gateway is running
        And I have a gateway named "mcp-delete-gw" configured
        And I set the current gateway to "mcp-delete-gw"
        And I apply the resource file "resources/gateway/sample-mcp-config.yaml"
        When I run ap with arguments "gateway mcp delete --id test-mcp-v1.0"
        Then the command should succeed

    @MCP-009
    Scenario: Delete non-existent MCP
        Given the gateway is running
        And I have a gateway named "mcp-delete-fail-gw" configured
        And I set the current gateway to "mcp-delete-fail-gw"
        When I run ap with arguments "gateway mcp delete --id non-existent-mcp"
        Then the command should fail
        And the output should contain "not found"
