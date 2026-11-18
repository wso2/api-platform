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

# Runs a real gateway on this machine, so it needs Docker and runs in the apip-cloud-self-hosted
# block, never in apip-cloud. The block's gateway loads the integration-test policy definitions
# under core/catalog/platformgateway/resources/custom-policies and reports all but it-builtin as
# custom. Those names are fixed and a synced policy outlives the gateway, so a scenario that syncs
# first removes any record of its policy an earlier run left.
@apip-cloud @apip-cloud-self-hosted @SELF-HOSTED-GATEWAY
Feature: APIP cloud custom policies of a self-hosted gateway
  As an APIP cloud user
  I want the custom policies my self-hosted gateway provides to reach the cloud
  So that I can sync them into my organisation and use them on my APIs

  Background:
    Given I obtain an APIP cloud console token
    And I select an existing APIP environment for a self-hosted gateway
    And I create a synthetic self-hosted APIP gateway
    And I start a local APIP gateway connected to the cloud control plane with a new registration token
    And the self-hosted APIP gateway should become active
    And the self-hosted APIP gateway's manifest should eventually list the policy "it-custom-basic"

  @ID_APIP_SELF_HOSTED_GATEWAY_004
  Scenario: A connected self-hosted gateway's manifest reports its custom policies with their definitions
    Then the self-hosted APIP gateway's manifest should report "it-custom-basic" version "v1.0.0" as a custom policy
    And the self-hosted APIP gateway's manifest should report "it-builtin" version "v1.0.0" as a built-in policy

  @ID_APIP_SELF_HOSTED_GATEWAY_005
  Scenario: A custom policy synced from a self-hosted gateway is added to the organisation and can be deleted
    Given I remove any custom policy "it-custom-basic" an earlier run left in the organisation
    When I sync the custom policy "it-custom-basic" version "v1.0.0" from the self-hosted APIP gateway
    Then the cloud API response status code should be 200
    And the cloud API response should report:
      | name                                                    | it-custom-basic |
      | version                                                 | v1.0.0          |
      | displayName                                             | IT Custom Basic |
      | policyDefinition.parameters.properties.headerValue.type | string          |
    And the synced custom policy should be listed in the organisation
    When I delete the synced custom policy
    Then the synced custom policy should eventually be absent
    And the organisation should hold no custom policy "it-custom-basic"

  @ID_APIP_SELF_HOSTED_GATEWAY_006 @type:negative
  Scenario: Syncing a policy the self-hosted gateway does not report as custom is refused
    When I sync the custom policy "it-builtin" version "v1.0.0" from the self-hosted APIP gateway
    Then the cloud API response status code should be 409
    And the cloud API error code should be "POLICY_INVALID_STATE"
    And the organisation should hold no custom policy "it-builtin"
