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

Feature: DevPortal Organization Commands
    As a CLI user
    I want to manage developer portal organizations
    So that I can administer organizations from the command line

    Background:
        Given the CLI is available
        And the devportal is running
        And I have a devportal named "dp-org" configured

    # =========================================
    # Organization List / Get (against the seeded ACME org)
    # =========================================

    @DP-ORG-001
    Scenario: List organizations
        When I run ap with arguments "devportal org list"
        Then the command should succeed
        And the output should contain "ACME"

    @DP-ORG-002
    Scenario: Get the seeded organization by id
        When I run ap with arguments "devportal org get --org 1ba42a09-45c0-40f8-a1bf-e4aa7cde1575"
        Then the command should succeed
        And the output should contain "ACME"

    @DP-ORG-003
    Scenario: Get organization without required org flag fails
        When I run ap with arguments "devportal org get"
        Then the command should fail
        And the stderr should contain "required flag"

    # =========================================
    # Organization Create / Delete
    #
    # @wip — create/delete mutate state and depend on the exact organization CR
    # schema accepted by the server. Enable once the CR fixture is verified.
    # =========================================

    @DP-ORG-004 @wip
    Scenario: Create an organization
        When I run ap with arguments "devportal org add -f resources/devportal/organization.yaml"
        Then the command should succeed
        And the output should contain "Organization created"

    @DP-ORG-005 @wip
    Scenario: Delete an organization
        When I run ap with arguments "devportal org delete --org it-test-org"
        Then the command should succeed
