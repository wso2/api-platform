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

Feature: DevPortal Configuration Commands
    As a CLI user
    I want to manage developer portal configurations
    So that I can connect to and interact with API Platform developer portals

    Background:
        Given the CLI is available

    # =========================================
    # DevPortal Add Tests
    # =========================================

    @DP-MANAGE-001
    Scenario: Add devportal with api-key auth
        When I run ap with arguments "devportal add --display-name dp-add-test --server http://localhost:3000 --auth api-key --api-key devportal-it-test-key --no-interactive"
        Then the command should succeed
        And the output should contain "added"
        And the output should contain "dp-add-test"

    @DP-MANAGE-002
    Scenario: Add devportal without server flag fails
        When I run ap with arguments "devportal add --display-name dp-no-server --auth api-key --api-key devportal-it-test-key --no-interactive"
        Then the command should fail
        And the output should contain "server"

    @DP-MANAGE-003
    Scenario: Add devportal with invalid auth type fails
        When I run ap with arguments "devportal add --display-name dp-bad-auth --server http://localhost:3000 --auth invalid --no-interactive"
        Then the command should fail
        And the output should contain "invalid auth type"

    # =========================================
    # DevPortal List Tests
    # =========================================

    @DP-MANAGE-004
    Scenario: List devportals
        Given I have a devportal named "dp-list-test" configured
        When I run ap with arguments "devportal list"
        Then the command should succeed
        And the output should contain "dp-list-test"

    @DP-MANAGE-005
    Scenario: List devportals when none configured
        Given I reset the CLI configuration
        When I run ap with arguments "devportal list"
        Then the command should succeed
        And the output should contain "No devportal configured"

    # =========================================
    # DevPortal Use / Current Tests
    # =========================================

    @DP-MANAGE-006
    Scenario: Use existing devportal
        Given I have a devportal named "dp-use-test" configured
        When I run ap with arguments "devportal use --display-name dp-use-test"
        Then the command should succeed
        And the output should contain "DevPortal set to dp-use-test"

    @DP-MANAGE-007
    Scenario: Use non-existent devportal fails
        When I run ap with arguments "devportal use --display-name dp-missing"
        Then the command should fail
        And the output should contain "not found"

    @DP-MANAGE-008
    Scenario: Show current devportal
        Given I have a devportal named "dp-current-test" configured
        And I set the current devportal to "dp-current-test"
        When I run ap with arguments "devportal current"
        Then the command should succeed
        And the output should contain "dp-current-test"

    @DP-MANAGE-009
    Scenario: Show current devportal when none set
        Given I reset the CLI configuration
        When I run ap with arguments "devportal current"
        Then the command should fail
        And the output should contain "no active devportal"

    # =========================================
    # DevPortal Remove Tests
    # =========================================

    @DP-MANAGE-010
    Scenario: Remove existing devportal
        Given I have a devportal named "dp-remove-test" configured
        When I run ap with arguments "devportal remove --display-name dp-remove-test"
        Then the command should succeed
        And the output should contain "removed"

    @DP-MANAGE-011
    Scenario: Remove non-existent devportal fails
        When I run ap with arguments "devportal remove --display-name dp-missing"
        Then the command should fail
        And the output should contain "not found"

    # =========================================
    # DevPortal Health Tests
    # =========================================

    @DP-MANAGE-012
    Scenario: Check devportal health
        Given the devportal is running
        And I have a devportal named "dp-health-test" configured
        When I run ap with arguments "devportal health"
        Then the command should succeed
        And the output should contain "DevPortal Status: ok"

    @DP-MANAGE-013
    Scenario: Check health of unreachable devportal
        Given I run ap with arguments "devportal add --display-name dp-unreachable --server http://localhost:19998 --auth api-key --api-key x --no-interactive"
        And I run ap with arguments "devportal use --display-name dp-unreachable"
        When I run ap with arguments "devportal health"
        Then the command should fail
