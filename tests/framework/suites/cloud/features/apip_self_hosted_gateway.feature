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
Feature: APIP cloud self-hosted gateway lifecycle
  As an APIP cloud user
  I want to register a gateway I run myself and manage its registration tokens
  So that the control-plane side of self-hosted gateway onboarding is verified

  @ID_APIP_SELF_HOSTED_GATEWAY_001
  Scenario: A self-hosted gateway is registered awaiting connection, issued a token, and deleted
    Given I obtain an APIP cloud console token
    And I select an existing APIP environment for a self-hosted gateway
    When I create a synthetic self-hosted APIP gateway
    Then the self-hosted APIP gateway should be retrievable and awaiting connection
    When I generate a registration token for the self-hosted APIP gateway
    Then the self-hosted APIP gateway should have exactly 1 active registration token
    When I delete the synthetic self-hosted APIP gateway
    Then the self-hosted APIP gateway should eventually be absent

  @ID_APIP_SELF_HOSTED_GATEWAY_002
  Scenario: Reconfiguring a self-hosted gateway's registration token revokes the previous one
    Given I obtain an APIP cloud console token
    And I select an existing APIP environment for a self-hosted gateway
    When I create a synthetic self-hosted APIP gateway
    Then the self-hosted APIP gateway should be retrievable and awaiting connection
    When I generate a registration token for the self-hosted APIP gateway
    And I reconfigure the registration token of the self-hosted APIP gateway
    Then the self-hosted APIP gateway should have exactly 1 active registration token
    And the revoked registration tokens should no longer be active
    When I delete the synthetic self-hosted APIP gateway
    Then the self-hosted APIP gateway should eventually be absent

  # The cloud counterpart of on-premises "A registered gateway round-trips its endpoints and
  # properties and starts inactive", through the cloud's /self-hosted-gateways. The cloud names
  # the gateway <environment>-<name> and adds properties.environment.
  @ID_APIP_SELF_HOSTED_GATEWAY_004
  Scenario: A self-hosted gateway keeps the endpoints and properties it is registered with, starts inactive, and is listed
    Given I obtain an APIP cloud console token
    And I select an existing APIP environment for a self-hosted gateway
    When I create a synthetic self-hosted APIP gateway with:
      | endpoints  | ["http://a.example.com", "http://b.example.com"] |
      | properties | {"region": "eu", "tier": "gold"}                 |
    Then the self-hosted APIP gateway should be retrievable and inactive, with the endpoints and properties it was created with
    And the self-hosted APIP gateway should be listed
    When I delete the synthetic self-hosted APIP gateway
    Then the self-hosted APIP gateway should eventually be absent

  # The creator half of on-premises "A registered gateway round-trips its endpoints and properties
  # and starts inactive": the cloud records the console user, where on-premises records "admin".
  # It reads back as "deleted_user" (see the scenario list entry).
  @ID_APIP_SELF_HOSTED_GATEWAY_006 @known-issue
  Scenario: A self-hosted gateway names the console user as its creator
    Given I obtain an APIP cloud console token
    And I select an existing APIP environment for a self-hosted gateway
    When I create a synthetic self-hosted APIP gateway
    Then the self-hosted APIP gateway should name the console user as its creator
    When I delete the synthetic self-hosted APIP gateway
    Then the self-hosted APIP gateway should eventually be absent

  # The cloud counterpart of on-premises "Gateway tokens are rotated and revoked independently".
  # The cloud answers a revoke with 204, where platform-api's own /gateways route answers 200.
  @ID_APIP_SELF_HOSTED_GATEWAY_005
  Scenario: A self-hosted gateway's registration tokens are revoked one at a time
    Given I obtain an APIP cloud console token
    And I select an existing APIP environment for a self-hosted gateway
    When I create a synthetic self-hosted APIP gateway
    And I generate a registration token for the self-hosted APIP gateway
    And I generate another registration token for the self-hosted APIP gateway
    Then the self-hosted APIP gateway should have exactly 2 active registration tokens
    When I revoke the first registration token of the self-hosted APIP gateway
    Then the self-hosted APIP gateway should have exactly 1 active registration token
    And the revoked registration tokens should no longer be active
    And the last registration token of the self-hosted APIP gateway should still be active
    When I revoke the first registration token of the self-hosted APIP gateway again
    Then the self-hosted APIP gateway should have exactly 1 active registration token
    And the revoked registration tokens should no longer be active
    And the last registration token of the self-hosted APIP gateway should still be active
    When I delete the synthetic self-hosted APIP gateway
    Then the self-hosted APIP gateway should eventually be absent
