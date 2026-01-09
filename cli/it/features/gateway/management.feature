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

Feature: Gateway Management Commands
    As a CLI user
    I want to manage gateway configurations
    So that I can connect to and interact with API Platform gateways

    Background:
        Given the CLI is available

    # =========================================
    # Gateway Add Tests
    # =========================================

    @GW-MANAGE-001
    Scenario: Add gateway with valid parameters
        Given the gateway is running
        When I run ap with arguments "gateway add --display-name test-gateway --server http://localhost:9090"
        Then the command should succeed
        And the output should contain "added"
        And the output should contain "test-gateway"

    @GW-MANAGE-002
    Scenario: Add gateway without name flag
        When I run ap with arguments "gateway add --server http://localhost:9090"
        Then the command should fail
        And the stderr should contain "required flag"

    @GW-MANAGE-003
    Scenario: Add gateway with invalid server URL
        # The CLI accepts invalid URLs - it doesn't validate URL format on add
        When I run ap with arguments "gateway add --display-name invalid-url-gw --server not-a-valid-url"
        Then the command should succeed
        And the output should contain "added"

    @GW-MANAGE-004
    Scenario: Add gateway with duplicate name
        Given the gateway is running
        And I have a gateway named "duplicate-gw" configured
        When I run ap with arguments "gateway add --display-name duplicate-gw --server http://localhost:9090"
        Then the command should succeed
        # CLI overwrites existing gateways with same name

    # =========================================
    # Gateway List Tests
    # =========================================

    @GW-MANAGE-005
    Scenario: List gateways
        Given the gateway is running
        And I have a gateway named "list-test-gw" configured
        When I run ap with arguments "gateway list"
        Then the command should succeed
        And the output should contain "list-test-gw"

    @GW-MANAGE-006
    Scenario: List gateways when empty
        Given I reset the CLI configuration
        When I run ap with arguments "gateway list"
        Then the command should succeed
        # May show empty table or "No gateways" message

    # =========================================
    # Gateway Remove Tests
    # =========================================

    @GW-MANAGE-007
    Scenario: Remove existing gateway
        Given the gateway is running
        And I have a gateway named "remove-test-gw" configured
        When I run ap with arguments "gateway remove --display-name remove-test-gw"
        Then the command should succeed

    @GW-MANAGE-008
    Scenario: Remove non-existent gateway
        When I run ap with arguments "gateway remove --display-name non-existent-gw"
        Then the command should fail
        And the output should contain "not found"

    # =========================================
    # Gateway Use Tests
    # =========================================

    @GW-MANAGE-009
    Scenario: Use existing gateway
        Given the gateway is running
        And I have a gateway named "use-test-gw" configured
        When I run ap with arguments "gateway use --display-name use-test-gw"
        Then the command should succeed

    @GW-MANAGE-010
    Scenario: Use non-existent gateway
        When I run ap with arguments "gateway use --display-name non-existent-gw"
        Then the command should fail
        And the output should contain "not found"

    # =========================================
    # Gateway Current Tests
    # =========================================

    @GW-MANAGE-011
    Scenario: Show current gateway
        Given the gateway is running
        And I have a gateway named "current-test-gw" configured
        And I set the current gateway to "current-test-gw"
        When I run ap with arguments "gateway current"
        Then the command should succeed
        And the output should contain "current-test-gw"

    @GW-MANAGE-012
    Scenario: Show current gateway when none set
        Given I reset the CLI configuration
        When I run ap with arguments "gateway current"
        Then the command should fail
        And the output should contain "no active gateway"

    # =========================================
    # Gateway Health Tests
    # =========================================

    @GW-MANAGE-013
    Scenario: Check gateway health
        Given the gateway is running
        And I have a gateway named "health-test-gw" configured
        And I set the current gateway to "health-test-gw"
        When I run ap with arguments "gateway health"
        Then the command should succeed
        And the output should contain "healthy"

    @GW-MANAGE-014
    Scenario: Check health of unreachable gateway
        Given I have a gateway named "unreachable-gw" with server "http://localhost:19999"
        And I set the current gateway to "unreachable-gw"
        When I run ap with arguments "gateway health"
        Then the command should fail
