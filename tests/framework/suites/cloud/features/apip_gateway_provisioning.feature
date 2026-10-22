# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except in compliance
# with the License. You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# --------------------------------------------------------------------

@apip-cloud
Feature: APIP cloud managed gateway provisioning
  As an APIP cloud user
  I want a managed gateway to be built in the background and report how far it got
  So that a gateway that is being built, connected or failed can be told apart from the platform's own answers

  # Known issue: the gateway takes longer to activate than the wait allows (B25).
  @TC_APIP_GW_011 @TC_APIP_GW_017 @TC_APIP_PROV_001 @TC_APIP_PROV_003 @known-issue @known-issue-1 @ID_APIP_GATEWAY_PROVISIONING_001
  Scenario: A gateway registered with no optional settings connects and reports its version
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I choose a synthetic managed APIP gateway name
    When I request a synthetic managed APIP gateway with:
      | environment | ${CTX:cloudGatewayEnvironment} |
    Then the cloud API response status code should be 201
    And the managed APIP gateway should not report its controller as connected yet
    And the managed APIP gateway should report no version
    And the synthetic managed APIP gateways should become active
    And the managed APIP gateway should report its controller as connected
    And the managed APIP gateway should report a version
    And the managed APIP gateway endpoint should answer requests

  @TC_APIP_GW_015 @TC_APIP_PROV_011 @ID_APIP_GATEWAY_PROVISIONING_002
  Scenario: A gateway's provisioning progress is visible on read until it is built
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    When I create a synthetic managed APIP gateway in the active gateway's environment
    Then the managed APIP gateway should report a recorded provisioning state on its first read
    And the managed APIP gateway status should progress from provisioning to built without failing

  @TC_APIP_GW_016 @ID_APIP_GATEWAY_PROVISIONING_003
  Scenario: A new gateway carries a recorded provisioning state from its first read
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    When I create a synthetic managed APIP gateway in the active gateway's environment
    Then the managed APIP gateway should report a recorded provisioning state on its first read

  # A repeated request creates a second gateway under a suffixed id instead of being refused or
  # answered with the first (B27).
  @known-issue @TC_APIP_PROV_002 @ID_APIP_GATEWAY_PROVISIONING_004
  Scenario: Repeating a creation request neither duplicates the gateway nor leaves it half built
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I choose a synthetic managed APIP gateway name
    When I request a synthetic managed APIP gateway with:
      | environment | ${CTX:cloudGatewayEnvironment} |
    Then the cloud API response status code should be 201
    When I request a synthetic managed APIP gateway with:
      | environment | ${CTX:cloudGatewayEnvironment} |
    Then the environment should hold exactly one managed gateway with the chosen name
    And the managed APIP gateway status should progress from provisioning to built without failing

  @TC_APIP_PROV_004 @ID_APIP_GATEWAY_PROVISIONING_005
  Scenario: Deleting a gateway removes it and frees its name for reuse
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I choose a synthetic managed APIP gateway name
    When I request a synthetic managed APIP gateway with:
      | environment | ${CTX:cloudGatewayEnvironment} |
    Then the cloud API response status code should be 201
    When I delete the synthetic managed APIP gateway
    Then the managed APIP gateway should eventually be absent
    When I request a synthetic managed APIP gateway with:
      | environment | ${CTX:cloudGatewayEnvironment} |
    Then the cloud API response status code should be 201
    And the managed APIP gateway should be registered under the id its name gives it
    And the environment should hold exactly one managed gateway with the chosen name

  # Known issue: the gateway takes longer to activate than the wait allows (B25).
  @TC_APIP_PROV_005 @known-issue @known-issue-1 @ID_APIP_GATEWAY_PROVISIONING_006
  Scenario: A gateway created in a new environment is provisioned and routable
    Given I obtain an APIP cloud console token
    And I create a synthetic non-production APIP environment
    When I create 1 synthetic managed APIP gateway in the synthetic APIP environment
    Then the synthetic managed APIP gateways should become active
    And the managed APIP gateway should report its controller as connected
    And the managed APIP gateway endpoint should answer requests

  @TC_APIP_PROV_006 @ID_APIP_GATEWAY_PROVISIONING_007
  Scenario: Two gateways of an organisation are deployed to and deleted independently
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    And I create 2 synthetic managed APIP gateways in the active gateway's environment
    And I create a synthetic REST API in the default project
    When I deploy a build of the synthetic REST API to synthetic gateway 1
    Then the synthetic REST API should be deployed on synthetic gateway 1
    And the synthetic REST API should not be deployed on synthetic gateway 2
    When I delete synthetic gateway 2
    Then synthetic gateway 1 should still be retrievable
    And the synthetic REST API should be deployed on synthetic gateway 1

  # Known issue: the gateway takes longer to activate than the wait allows (B25).
  @TC_APIP_PROV_007 @known-issue @known-issue-1 @ID_APIP_GATEWAY_PROVISIONING_008
  Scenario: Gateways whose handles reach the length limit still receive a routable service
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I choose a synthetic managed APIP gateway name
    When I create a synthetic managed APIP gateway whose handle is 2 characters short of the environment's limit
    And I create a synthetic managed APIP gateway whose handle is 1 character short of the environment's limit
    And I create a synthetic managed APIP gateway whose handle is 0 characters short of the environment's limit
    Then the synthetic managed APIP gateways should become active
    And every managed gateway the scenario created should be retrievable
    And the managed APIP gateway endpoint should answer requests
