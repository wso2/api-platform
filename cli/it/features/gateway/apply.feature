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

Feature: Gateway Apply Command
    As a CLI user
    I want to apply API and MCP configurations to the gateway
    So that I can deploy APIs and MCPs

    Background:
        Given the CLI is available
        And the gateway is running

    # =========================================
    # Apply API Tests
    # =========================================

    @APPLY-001
    Scenario: Apply valid API yaml
        Given I have a gateway named "apply-test-gw" configured
        And I set the current gateway to "apply-test-gw"
        When I run ap with arguments "gateway apply -f resources/gateway/sample-api.yaml"
        Then the command should succeed

    @APPLY-002
    Scenario: Apply invalid yaml format
        Given I have a gateway named "apply-test-gw2" configured
        And I set the current gateway to "apply-test-gw2"
        When I run ap with arguments "gateway apply -f resources/gateway/invalid.yaml"
        Then the command should fail

    @APPLY-003
    Scenario: Apply missing file
        Given I have a gateway named "apply-test-gw3" configured
        And I set the current gateway to "apply-test-gw3"
        When I run ap with arguments "gateway apply -f non-existent-file.yaml"
        Then the command should fail
        And the output should contain "no such file"

    @APPLY-004
    Scenario: Apply valid MCP yaml
        Given the MCP server is running
        And I have a gateway named "apply-mcp-gw" configured
        And I set the current gateway to "apply-mcp-gw"
        When I run ap with arguments "gateway apply -f resources/gateway/sample-mcp-config.yaml"
        Then the command should succeed
