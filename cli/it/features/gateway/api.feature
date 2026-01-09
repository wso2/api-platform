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

Feature: Gateway API Management Commands
    As a CLI user
    I want to manage APIs on the gateway
    So that I can list, view, and delete deployed APIs

    Background:
        Given the CLI is available
        And the gateway is running
        And I have a gateway named "api-test-gw" configured
        And I set the current gateway to "api-test-gw"

    # =========================================
    # API List Tests
    # =========================================

    @GW-API-001
    Scenario: List APIs
        Given I apply the resource file "resources/gateway/sample-api.yaml"
        When I run ap with arguments "gateway api list"
        Then the command should succeed
        # Clean up the sample API so subsequent "when empty" test is valid
        When I run ap with arguments "gateway api delete --id petstore-api-v1.0"
        Then the command should succeed

    @GW-API-002
    Scenario: List APIs when empty
        When I run ap with arguments "gateway api list"
        Then the command should succeed
        And the output should not contain "petstore"

    # =========================================
    # API Get Tests
    # =========================================

    @GW-API-003
    Scenario: Get existing API
        Given I apply the resource file "resources/gateway/sample-api.yaml"
        When I run ap with arguments "gateway api get --id petstore-api-v1.0"
        Then the command should succeed
        And the output should contain "petstore"

    @GW-API-004
    Scenario: Get non-existent API
        When I run ap with arguments "gateway api get --id non-existent-api"
        Then the command should fail
        And the output should contain "not found"

    # =========================================
    # API Delete Tests
    # =========================================

    @GW-API-005
    Scenario: Delete existing API
        Given I apply the resource file "resources/gateway/sample-api.yaml"
        When I run ap with arguments "gateway api delete --id petstore-api-v1.0"
        Then the command should succeed

    @GW-API-006
    Scenario: Delete non-existent API
        When I run ap with arguments "gateway api delete --id non-existent-api"
        Then the command should fail
        And the output should contain "not found"
