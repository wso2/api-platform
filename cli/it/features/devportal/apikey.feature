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

Feature: DevPortal API Key Commands
    As a CLI user
    I want to manage developer portal API keys
    So that I can issue and revoke API keys from the command line

    Background:
        Given the CLI is available
        And the devportal is running
        And I have a devportal named "dp-apikey" configured

    # =========================================
    # API Key flag validation
    # =========================================

    @DP-APIKEY-001
    Scenario: List API keys without required flags fails
        When I run ap with arguments "devportal api-key get --org 1ba42a09-45c0-40f8-a1bf-e4aa7cde1575"
        Then the command should fail
        And the stderr should contain "required flag"

    # =========================================
    # API Key Generate / Get / Regenerate / Revoke
    #
    # @wip — require an existing API to bind the key to. Enable once the API
    # publish fixture exists.
    # =========================================

    @DP-APIKEY-002 @wip
    Scenario: Generate an API key
        When I run ap with arguments "devportal api-key generate --org 1ba42a09-45c0-40f8-a1bf-e4aa7cde1575 --api-id it-test-api --name it-test-key --no-interactive"
        Then the command should succeed

    @DP-APIKEY-003 @wip
    Scenario: List API keys for an API
        When I run ap with arguments "devportal api-key get --org 1ba42a09-45c0-40f8-a1bf-e4aa7cde1575 --api-id it-test-api"
        Then the command should succeed
