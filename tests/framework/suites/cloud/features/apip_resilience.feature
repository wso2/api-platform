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
Feature: APIP cloud gateway resilience
  As an APIP cloud user
  I want a gateway to keep serving when it is changed or a dependency fails
  So that scaling and failures do not change what clients are answered

  # Known issue: the gateway takes longer to activate than the wait allows (B25).
  @TC_APIP_RES_011 @known-issue @known-issue-1 @ID_APIP_RESILIENCE_001
  Scenario: A gateway scaled to more runtime replicas keeps routing consistently across them
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    And I create 1 synthetic managed APIP gateway in the active gateway's environment
    And the synthetic managed APIP gateways should become active
    And the synthetic managed APIP gateway's configuration should become readable
    When I save the gateway configuration with:
      | gateway.gatewayRuntime.deployment.replicaCount | 2 |
    Then the cloud API response status code should be 200
    And the synthetic managed APIP gateway's configuration phase should become "healthy"
    When I create a synthetic REST API in the default project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudManagedGatewayID}   |
    Then the cloud API response status code should be 201
    And the synthetic REST API should be DEPLOYED on every gateway the deployment request named
    When I read the endpoint of the synthetic managed APIP gateway
    Then I send a "GET" request to "${CTX:cloudManagedGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 200
    And requests to "${CTX:cloudManagedGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" should be served by at least 2 different gateway runtime pods
