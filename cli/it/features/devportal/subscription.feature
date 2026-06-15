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

Feature: DevPortal Subscription Commands
    As a CLI user
    I want to manage developer portal subscriptions
    So that I can administer API subscriptions from the command line

    Background:
        Given the CLI is available
        And the devportal is running
        And I have a devportal named "dp-sub" configured

    # =========================================
    # Subscription List (against the seeded ACME org)
    # =========================================

    @DP-SUB-001
    Scenario: List subscriptions in an organization
        When I run ap with arguments "devportal subscription get --org 1ba42a09-45c0-40f8-a1bf-e4aa7cde1575"
        Then the command should succeed
        And the output should contain "Platform subscriptions"

    @DP-SUB-002
    Scenario: List subscriptions without required org flag fails
        When I run ap with arguments "devportal subscription get"
        Then the command should fail
        And the stderr should contain "required flag"

    @DP-SUB-003
    Scenario: Create subscription without required flags fails
        When I run ap with arguments "devportal subscription create --org 1ba42a09-45c0-40f8-a1bf-e4aa7cde1575"
        Then the command should fail
        And the stderr should contain "required flag"

    # =========================================
    # Subscription Create / Edit / Delete
    #
    # @wip — require an existing API with token-based subscriptions enabled,
    # which in turn needs the API publish flow. Enable once that fixture exists.
    # =========================================

    @DP-SUB-004 @wip
    Scenario: Create a subscription
        When I run ap with arguments "devportal subscription create --org 1ba42a09-45c0-40f8-a1bf-e4aa7cde1575 --api-id it-test-api"
        Then the command should succeed

    @DP-SUB-005 @wip
    Scenario: Delete a subscription
        When I run ap with arguments "devportal subscription delete --org 1ba42a09-45c0-40f8-a1bf-e4aa7cde1575 --sub-id it-test-sub"
        Then the command should succeed
