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
Feature: APIP cloud promotion along a deployment pipeline
  As an APIP cloud user
  I want an API to reach a later environment only by promotion, carrying the build the earlier one runs
  So that no environment runs a build that skipped the one before it

  @TC_APIP_PROM_002 @ID_APIP_PROMOTION_001
  Scenario: Deploying directly to an environment a promotion path leads to is refused
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create 1 synthetic non-production APIP environment
    And I create a synthetic deployment pipeline with the promotion paths:
      | source                         | targets                           |
      | ${CTX:cloudGatewayEnvironment} | ${CTX:cloudSyntheticEnvironment1} |
    And I create a synthetic APIP project bound to the synthetic deployment pipeline
    When I create a synthetic REST API in the synthetic APIP project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudSyntheticEnvironment1} |
    Then the cloud API response status code should be 400
    And the cloud API response body should contain "is a promotion target"
    And the cloud API response body should contain "${CTX:cloudSyntheticEnvironment1}"
    And the deployment stage of synthetic APIP environment 1 should show no gateway as deployed

  @TC_APIP_PROM_007 @ID_APIP_PROMOTION_002
  Scenario: Promoting into an environment that has no gateway is refused
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create 1 synthetic non-production APIP environment
    And I create a synthetic deployment pipeline with the promotion paths:
      | source                         | targets                           |
      | ${CTX:cloudGatewayEnvironment} | ${CTX:cloudSyntheticEnvironment1} |
    And I create a synthetic APIP project bound to the synthetic deployment pipeline
    When I create a synthetic REST API in the synthetic APIP project
    And I request a deployment of the synthetic REST API with:
      | environment     | ${CTX:cloudSyntheticEnvironment1} |
      | fromEnvironment | ${CTX:cloudGatewayEnvironment}    |
    Then the cloud API response status code should be 400
    And the cloud API response body should contain "has no gateway to deploy to"
    And the cloud API response body should contain "${CTX:cloudSyntheticEnvironment1}"
    And the deployment stage of synthetic APIP environment 1 should show no gateway as deployed
