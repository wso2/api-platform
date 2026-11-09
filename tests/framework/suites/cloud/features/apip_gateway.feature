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
  I want to list managed gateways and have a gateway request that breaks the rules refused
  So that the gateways an environment offers are the ones that were registered for it, and a refused request registers nothing

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

  @TC_APIP_GW_009 @ID_APIP_GATEWAY_003
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
  @known-issue @TC_APIP_GW_010 @ID_APIP_GATEWAY_004
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

  @TC_APIP_GW_019 @ID_APIP_GATEWAY_005
  Scenario: Reading a managed gateway that does not exist returns not found
    Given I obtain an APIP cloud console token
    When I send an authenticated "GET" request to the APIP cloud path "/managed-gateways/no-such-gateway"
    Then the cloud API response status code should be 404
    And the cloud API error code should be "not_found"
