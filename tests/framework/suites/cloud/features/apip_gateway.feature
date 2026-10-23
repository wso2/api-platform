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
Feature: APIP cloud managed gateway lifecycle
  As an APIP cloud user
  I want to list, create, read, update and delete managed gateways
  So that the gateways an environment offers are the ones that were registered for it

  @TC_APIP_GW_001 @ID_APIP_GATEWAY_001
  Scenario: The organisation's managed gateways are listed with their status
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    When I list the managed APIP gateways
    Then the cloud API response status code should be 200
    And the listed managed gateways should each report an identity, an environment and a status
    And the listed managed gateways should include the active gateway

  @TC_APIP_GW_002 @ID_APIP_GATEWAY_002
  Scenario: An existing environment can be selected to host a managed gateway
    Given I obtain an APIP cloud console token
    When I select an existing APIP environment for a managed gateway
    Then the selected APIP environment should be listed by the platform

  @TC_APIP_GW_003 @TC_APIP_GW_004 @TC_APIP_GW_005 @TC_APIP_GW_006 @ID_APIP_GATEWAY_003
  Scenario: A managed gateway can be retrieved and deleted
    Given I obtain an APIP cloud console token
    And I select an existing APIP environment for a managed gateway
    When I create a synthetic managed APIP gateway
    Then the managed APIP gateway should be retrievable through the managed-gateway endpoint
    When I delete the synthetic managed APIP gateway
    Then the managed APIP gateway should eventually be absent

  # A second request for the same name is answered 201 with a second gateway whose id carries a
  # random suffix, instead of 409 (B27).
  @known-issue @TC_APIP_GW_007 @ID_APIP_GATEWAY_004
  Scenario: A gateway name already used in the environment is refused
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I choose a synthetic managed APIP gateway name
    When I request a synthetic managed APIP gateway with:
      | environment | ${CTX:cloudGatewayEnvironment} |
    Then the cloud API response status code should be 201
    When I request a synthetic managed APIP gateway with:
      | environment | ${CTX:cloudGatewayEnvironment} |
    Then the cloud API response status code should be 409
    And the cloud API error code should be "conflict"
    And the environment should hold exactly one managed gateway with the chosen name

  @TC_APIP_GW_008 @known-issue @ID_APIP_GATEWAY_005
  Scenario: The same gateway name can be used in two environments
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create a synthetic non-production APIP environment
    And I choose a synthetic managed APIP gateway name
    When I request a synthetic managed APIP gateway with:
      | environment | ${CTX:cloudGatewayEnvironment} |
    Then the cloud API response status code should be 201
    When I request a synthetic managed APIP gateway with:
      | environment | ${CTX:cloudEnvironmentName} |
    Then the cloud API response status code should be 201
    And every managed gateway the scenario created should be retrievable

  @TC_APIP_GW_009 @ID_APIP_GATEWAY_006
  Scenario: A managed gateway cannot be created in an environment that does not exist
    Given I obtain an APIP cloud console token
    And I choose a synthetic managed APIP gateway name
    When I request a synthetic managed APIP gateway with:
      | environment | syn-absent-environment |
    Then the cloud API response status code should be 400
    And the cloud API error code should be "invalid_request"
    And the cloud API response body should contain "syn-absent-environment"

  # The platform refuses the version, but the integrator answers its caller 502 for every failed
  # gateway create, so the console sees 500 instead of 400 (B26).
  @known-issue @TC_APIP_GW_010 @ID_APIP_GATEWAY_007
  Scenario Outline: A version that is neither major.minor nor CalVer is refused
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I choose a synthetic managed APIP gateway name
    When I request a synthetic managed APIP gateway with:
      | environment | ${CTX:cloudGatewayEnvironment} |
      | version     | <version>                      |
    Then the cloud API response status code should be 400
    And the cloud API error code should be "invalid_request"

    Examples:
      | version     |
      | latest      |
      | sha-3f2a9c1 |
      | v1.2        |
      | 1.2.3       |
      | 1           |
      | 2026-05-13  |

  @TC_APIP_GW_012 @known-issue @ID_APIP_GATEWAY_008
  Scenario: The default gateway of an environment cannot be deleted while others exist
    Given I obtain an APIP cloud console token
    And I create a synthetic non-production APIP environment
    And I create 2 synthetic managed APIP gateways in the synthetic APIP environment
    When I delete the default gateway of the synthetic APIP environment
    Then the cloud API response status code should be 409
    And the cloud API error code should be "gateway_is_default"
    And the cloud API response body should contain "${CTX:cloudManagedGatewayID}"
    And the refused default gateway should still be its environment's default

  @TC_APIP_GW_013 @ID_APIP_GATEWAY_009
  Scenario: Updating a gateway's metadata is reflected when it is read
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    When I create a synthetic managed APIP gateway in the active gateway's environment
    And I update the synthetic managed APIP gateway with:
      | displayName | Renamed synthetic gateway                |
      | description | Updated by the APIP cloud integration test |
      | isCritical  | true                                     |
      | properties  | {"owner": "cloud-it"}                    |
    Then the cloud API response status code should be 200
    And the managed APIP gateway should report:
      | displayName      | Renamed synthetic gateway                |
      | description      | Updated by the APIP cloud integration test |
      | isCritical       | true                                     |
      | properties.owner | cloud-it                                 |
    And the update should have left the managed APIP gateway's identity unchanged

  @TC_APIP_GW_014 @ID_APIP_GATEWAY_010
  Scenario: A created gateway is bound to its environment and listed for it
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    When I create a synthetic managed APIP gateway in the active gateway's environment
    Then the managed APIP gateway should be bound to the environment it was created in

  @TC_APIP_GW_019 @ID_APIP_GATEWAY_011
  Scenario: Reading a managed gateway that does not exist returns not found
    Given I obtain an APIP cloud console token
    When I send an authenticated "GET" request to the APIP cloud path "/managed-gateways/no-such-gateway"
    Then the cloud API response status code should be 404
    And the cloud API error code should be "not_found"

  @TC_APIP_GW_020 @ID_APIP_GATEWAY_012
  Scenario: Gateway types are kept apart in the listing
    Given I obtain an APIP cloud console token
    When I list the managed APIP gateways of type "regular"
    Then the cloud API response status code should be 200
    And every listed managed gateway should be of type "regular"
    When I list the managed APIP gateways of type "ai"
    Then the cloud API response status code should be 200
    And every listed managed gateway should be of type "ai"
    When I list the managed APIP gateways of type "event"
    Then the cloud API response status code should be 200
    And every listed managed gateway should be of type "event"
    And the managed gateway types should partition the unfiltered listing
