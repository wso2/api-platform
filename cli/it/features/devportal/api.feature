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

Feature: DevPortal REST API Commands
    As a CLI user
    I want to manage developer portal APIs
    So that I can publish and inspect APIs from the command line

    Background:
        Given the CLI is available
        And the devportal is running
        And I have a devportal named "dp-api" configured

    # =========================================
    # REST API List / Get (against the seeded ACME org)
    # =========================================

    @DP-API-001
    Scenario: List APIs in an organization
        When I run ap with arguments "devportal rest-api list --org 1ba42a09-45c0-40f8-a1bf-e4aa7cde1575"
        Then the command should succeed
        And the output should contain "No APIs found in the organization"

    @DP-API-002
    Scenario: List APIs without required org flag fails
        When I run ap with arguments "devportal rest-api list"
        Then the command should fail
        And the stderr should contain "required flag"

    @DP-API-003
    Scenario: Get API without required flags fails
        When I run ap with arguments "devportal rest-api get --org 1ba42a09-45c0-40f8-a1bf-e4aa7cde1575"
        Then the command should fail
        And the stderr should contain "required flag"

    # =========================================
    # REST API Publish / Get / Delete
    #
    # @wip — these mutate state and require a valid devportal API artifact ZIP
    # fixture. Enable once the artifact (resources/devportal/api-artifact.zip) is
    # produced (e.g. via `ap devportal build`).
    # =========================================

    @DP-API-004 @wip
    Scenario: Publish an API
        When I run ap with arguments "devportal rest-api publish -f resources/devportal/api-artifact.zip --org 1ba42a09-45c0-40f8-a1bf-e4aa7cde1575"
        Then the command should succeed

    @DP-API-005 @wip
    Scenario: Get a published API
        When I run ap with arguments "devportal rest-api get --org 1ba42a09-45c0-40f8-a1bf-e4aa7cde1575 --api-id it-test-api"
        Then the command should succeed

    @DP-API-006 @wip
    Scenario: Delete a published API
        When I run ap with arguments "devportal rest-api delete --org 1ba42a09-45c0-40f8-a1bf-e4aa7cde1575 --api-id it-test-api"
        Then the command should succeed
