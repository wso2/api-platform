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

@ap-cli-gateway
Feature: ap CLI gateway configuration
  As a CLI user
  I want to manage the gateways the ap CLI knows about
  So that I can connect to and interact with API Platform gateways

  Every scenario starts from an empty CLI configuration of its own, so the gateways it adds are
  visible to it alone.

  Background:
    Given the "ap" CLI is available

  @legacy:GW-MANAGE-001
  Scenario: Add a gateway with valid parameters
    When I run ap with arguments "gateway add --display-name test-gateway --server http://localhost:9090"
    Then the CLI command should succeed
    And the CLI output should contain "added"
    And the CLI output should contain "test-gateway"

  @legacy:GW-MANAGE-002
  Scenario: Adding a gateway without a display name is rejected
    When I run ap with arguments "gateway add --server http://localhost:9090"
    Then the CLI command should fail
    And the CLI error output should contain "required flag"

  # The CLI does not validate the URL format when a gateway is added.
  @legacy:GW-MANAGE-003
  Scenario: Add a gateway with an invalid server URL
    When I run ap with arguments "gateway add --display-name invalid-url-gw --server not-a-valid-url"
    Then the CLI command should succeed
    And the CLI output should contain "added"

  # Adding a gateway under a name that already exists replaces it.
  @legacy:GW-MANAGE-004
  Scenario: Add a gateway with a duplicate name
    Given the CLI uses the platform gateway as "duplicate-gw"
    When I run ap with arguments "gateway add --display-name duplicate-gw --server http://localhost:9090"
    Then the CLI command should succeed

  @legacy:GW-MANAGE-005
  Scenario: List gateways
    Given the CLI uses the platform gateway as "list-test-gw"
    When I run ap with arguments "gateway list"
    Then the CLI command should succeed
    And the CLI output should contain "list-test-gw"

  @legacy:GW-MANAGE-006
  Scenario: List gateways when none are configured
    When I run ap with arguments "gateway list"
    Then the CLI command should succeed

  @legacy:GW-MANAGE-007
  Scenario: Remove an existing gateway
    Given the CLI uses the platform gateway as "remove-test-gw"
    When I run ap with arguments "gateway remove --display-name remove-test-gw"
    Then the CLI command should succeed
    When I run ap with arguments "gateway list"
    Then the CLI output should not contain "remove-test-gw"

  @legacy:GW-MANAGE-008
  Scenario: Removing a gateway that does not exist is rejected
    When I run ap with arguments "gateway remove --display-name non-existent-gw"
    Then the CLI command should fail
    And the CLI output should contain "not found"

  @legacy:GW-MANAGE-009
  Scenario: Use an existing gateway
    Given the CLI uses the platform gateway as "use-test-gw"
    When I run ap with arguments "gateway use --display-name use-test-gw"
    Then the CLI command should succeed

  @legacy:GW-MANAGE-010
  Scenario: Using a gateway that does not exist is rejected
    When I run ap with arguments "gateway use --display-name non-existent-gw"
    Then the CLI command should fail
    And the CLI output should contain "not found"

  @legacy:GW-MANAGE-011
  Scenario: Show the current gateway
    Given the CLI uses the platform gateway as "current-test-gw"
    When I run ap with arguments "gateway current"
    Then the CLI command should succeed
    And the CLI output should contain "current-test-gw"

  @legacy:GW-MANAGE-012
  Scenario: Showing the current gateway when none is set is rejected
    When I run ap with arguments "gateway current"
    Then the CLI command should fail
    And the CLI output should contain "no active gateway"

  @legacy:GW-MANAGE-013
  Scenario: Check the health of the gateway
    Given the CLI uses the platform gateway as "health-test-gw"
    When I run ap with arguments "gateway health"
    Then the CLI command should succeed
    And the CLI output should contain "healthy"

  @legacy:GW-MANAGE-014
  Scenario: Checking the health of an unreachable gateway fails
    Given the CLI uses a gateway at "http://127.0.0.1:1" as "unreachable-gw"
    When I run ap with arguments "gateway health"
    Then the CLI command should fail
