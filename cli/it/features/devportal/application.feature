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

Feature: DevPortal Application Commands
    As a CLI user
    I want to manage developer portal applications
    So that I can administer applications from the command line

    # NOTE: All application happy-path scenarios are tagged @wip and therefore
    # SKIPPED. The CLI now targets the org-scoped application path
    # (/o/{orgId}/devportal/v1/applications), but the server spec still exposes the
    # application endpoints without the org prefix, so these commands currently
    # fail against the running devportal. Remove the @wip tag once the server side
    # is aligned (see docs/devportal-openapi-spec-v1.yaml).

    Background:
        Given the CLI is available
        And the devportal is running
        And I have a devportal named "dp-app" configured

    # =========================================
    # Application flag validation (active even while the API is @wip)
    # =========================================

    @DP-APP-001
    Scenario: Create application without required flags fails
        When I run ap with arguments "devportal application create --org 1ba42a09-45c0-40f8-a1bf-e4aa7cde1575"
        Then the command should fail
        And the stderr should contain "required flag"

    # =========================================
    # Application Create / Get / Update / Delete
    # =========================================

    @DP-APP-002 @wip
    Scenario: Create an application
        When I run ap with arguments "devportal application create --org 1ba42a09-45c0-40f8-a1bf-e4aa7cde1575 --name it-test-app --type WEB"
        Then the command should succeed
        And the output should contain "Application created"

    @DP-APP-003 @wip
    Scenario: List applications
        When I run ap with arguments "devportal application get --org 1ba42a09-45c0-40f8-a1bf-e4aa7cde1575"
        Then the command should succeed

    @DP-APP-004 @wip
    Scenario: Delete an application
        When I run ap with arguments "devportal application delete --org 1ba42a09-45c0-40f8-a1bf-e4aa7cde1575 --app-id it-test-app"
        Then the command should succeed
