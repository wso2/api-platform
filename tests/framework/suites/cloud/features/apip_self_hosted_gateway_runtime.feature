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
# block, never in apip-cloud.
@apip-cloud @apip-cloud-self-hosted @SELF-HOSTED-GATEWAY
Feature: APIP cloud self-hosted gateway connected to the cloud control plane
  As an APIP cloud user
  I want a gateway I run myself to connect to the cloud with its registration token
  So that the APIs I deploy to it in the console are served by my gateway

  @ID_APIP_SELF_HOSTED_GATEWAY_003
  Scenario: A local gateway connects with its registration token and serves the API deployed to it
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    And I select the active APIP gateway's environment for a self-hosted gateway
    When I create a synthetic self-hosted APIP gateway
    Then the self-hosted APIP gateway should be retrievable and awaiting connection
    When I start a local APIP gateway connected to the cloud control plane with a new registration token
    Then the self-hosted APIP gateway should become active
    When I use the local self-hosted APIP gateway as the active gateway
    And I create a synthetic REST API in the default project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudGatewayID}          |
    Then the cloud API response status code should be 201
    And the deployment response should list the active gateway
    And the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 200
    When I undeploy the synthetic REST API
    Then the synthetic REST API deployment should become UNDEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 404
    When I stop the local self-hosted APIP gateway
    Then the self-hosted APIP gateway should become inactive
