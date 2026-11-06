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

@apip-cloud @apip-self-hosted-gateway
Feature: APIP cloud self-hosted gateway lifecycle
  As an APIP cloud user
  I want to register a gateway I run myself and manage its registration tokens
  So that the control-plane side of self-hosted gateway onboarding is verified

  Scenario: A self-hosted gateway is registered awaiting connection, issued a token, and deleted
    Given I obtain an APIP cloud console token
    And I select an existing APIP environment for a self-hosted gateway
    When I create a synthetic self-hosted APIP gateway
    Then the self-hosted APIP gateway should be retrievable and awaiting connection
    When I generate a registration token for the self-hosted APIP gateway
    Then the self-hosted APIP gateway should have exactly 1 active registration token
    When I delete the synthetic self-hosted APIP gateway
    Then the self-hosted APIP gateway should eventually be absent

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
