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
Feature: APIP cloud managed API Portal and consumer sign-in
  As an APIP cloud user
  I want to create a managed API Portal that my consumers can sign up and sign in to
  So that portal creation, its refusals, consumer sign-in and portal deletion are verified

  # The refusals run first, while the runner holds no portal: the organisation's plan may allow a
  # single portal, and the plan is checked before the request is.
  # The creation scenario then builds one portal for the runner, because a build takes minutes.
  # Every later scenario uses that portal and the last one deletes it. When the platform fails the
  # build, the creation scenario reports it and the later scenarios build one replacement. A
  # consumer is a user of the organisation's identity provider that the cloud API cannot remove,
  # so the runner signs up one consumer and the later sign-in scenarios reuse it.

  @ID_APIP_PORTAL_001
  Scenario Outline: A portal request that breaks the naming rules is refused and leaves nothing behind
    Given I obtain an APIP cloud console token
    When I request a managed API Portal that has "<defect>"
    Then the cloud API response status code should be 400
    And the refused managed API Portal should not be listed

    Examples:
      | defect                             |
      | an uppercase handle                |
      | a handle with an underscore        |
      | a handle shorter than 3 characters |
      | a handle longer than 40 characters |
      | no name                            |

  @ID_APIP_PORTAL_002
  Scenario: A portal whose consumers would sign in on an environment the organisation does not have is refused
    Given I obtain an APIP cloud console token
    When I request a managed API Portal whose login environment does not exist
    Then the cloud API response status code should be 400
    And the refused managed API Portal should not be listed

  @ID_APIP_PORTAL_003
  Scenario: A portal requested with a name and a handle is built and becomes active
    Given I obtain an APIP cloud console token
    When I request a synthetic managed API Portal for the runner
    Then the synthetic managed API Portal should be accepted for provisioning
    And the synthetic managed API Portal should become active
    And the cloud API response should report:
      | loginEnvironment | development |
      | selfSignup       | true        |
    And the synthetic managed API Portal should report an HTTPS URL
    And the managed API Portal list should include the synthetic managed API Portal

  # Skipped on an organisation whose plan allows a single portal: the second create is refused for
  # the plan, with 402, before the handle is checked.
  @ID_APIP_PORTAL_004
  Scenario: A portal whose handle is already in use is refused
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I request a managed API Portal under the handle of the synthetic managed API Portal
    Then the cloud API response status code should be 409
    And the synthetic managed API Portal should still be active

  @ID_APIP_PORTAL_005
  Scenario: Reading a portal that does not exist returns not found
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I read the synthetic managed API Portal
    Then the cloud API response status code should be 200
    When I read a managed API Portal that does not exist
    Then the cloud API response status code should be 404

  @ID_APIP_PORTAL_006
  Scenario: A visitor who opens the portal's sign-in is sent to its identity provider
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I open the sign-in page of the synthetic managed API Portal as a new visitor
    Then the visitor should be sent to the portal's identity provider to sign in

  @ID_APIP_PORTAL_007
  Scenario: A consumer signs up from the portal's sign-in page and is signed in
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I sign up on the synthetic managed API Portal as a new consumer
    Then the consumer should be signed in to the synthetic managed API Portal
    And a visitor without the consumer's session should be sent to sign in

  @ID_APIP_PORTAL_008
  Scenario: A consumer who signed up signs in again with a username and password
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I sign in to the synthetic managed API Portal as the consumer
    Then the consumer should be signed in to the synthetic managed API Portal

  @ID_APIP_PORTAL_009
  Scenario: A wrong password does not sign the consumer in
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I try to sign in to the synthetic managed API Portal as the consumer with a wrong password
    Then the identity provider should refuse the sign-in
    And the consumer should not be signed in to the synthetic managed API Portal
    When I sign in to the synthetic managed API Portal as the consumer
    Then the consumer should be signed in to the synthetic managed API Portal

  @ID_APIP_PORTAL_010
  Scenario: Signing out ends the consumer's session
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    And I sign in to the synthetic managed API Portal as the consumer
    And the consumer should be signed in to the synthetic managed API Portal
    When I sign out of the synthetic managed API Portal
    Then the consumer should not be signed in to the synthetic managed API Portal

  @ID_APIP_PORTAL_011
  Scenario: A deleted portal is torn down and leaves the list
    Given I obtain an APIP cloud console token
    And the runner has an active synthetic managed API Portal
    When I delete the synthetic managed API Portal
    Then the managed API Portal deletion should be accepted
    And the synthetic managed API Portal should eventually be gone
