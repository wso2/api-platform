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
Feature: APIP cloud builds and deployments
  As an APIP cloud user
  I want to prepare builds of an API and deploy them through the project's pipeline
  So that what a gateway serves can always be traced to a build

  @TC_APIP_DEP_001 @ID_APIP_DEPLOY_001
  Scenario: A build can be prepared for an API and is listed
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    When I create a synthetic REST API in the default project
    And I prepare a build of the synthetic REST API with metadata:
      | commit | 0a1b2c3 |
    Then the prepared build should have a day-and-index identifier
    And the synthetic REST API builds should include the prepared build

  @TC_APIP_DEP_002 @ID_APIP_DEPLOY_002
  Scenario: The builds of an API are listed newest first
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic REST API in the default project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudGatewayID}          |
    Then the cloud API response status code should be 201
    And the deployment should have rendered a build of the API
    And the synthetic REST API deployment should become DEPLOYED
    When I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudGatewayID}          |
    Then the cloud API response status code should be 201
    And the deployment should have rendered a build of the API
    And the synthetic REST API builds should list the rendered builds newest first

  @TC_APIP_DEP_003 @ID_APIP_DEPLOY_003
  Scenario: A build that already exists can be deployed to a gateway in an environment
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic REST API in the default project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudGatewayID}          |
    Then the cloud API response status code should be 201
    And the deployment should have rendered a build of the API
    And the synthetic REST API deployment should become DEPLOYED
    When I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudGatewayID}          |
      | buildId     | ${CTX:cloudBuildID}            |
    Then the cloud API response status code should be 201
    And the deployment response should list the active gateway
    And the deployed build should be the rendered build
    And the synthetic REST API deployment should become DEPLOYED
    And the deployment stage of the active gateway's environment should show the active gateway as DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 200

  @TC_APIP_DEP_004 @ID_APIP_DEPLOY_004
  Scenario: The deployments of an API list one stage per environment of its pipeline
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic REST API in the default project
    Then the synthetic REST API deployments should have one stage per environment of its project's deployment pipeline
    And the deployment stage of the active gateway's environment should show the active gateway as NOT_DEPLOYED

  @TC_APIP_DEP_005 @ID_APIP_DEPLOY_005
  Scenario: An undeployed API is no longer served by the gateway
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic REST API in the default project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudGatewayID}          |
    Then the cloud API response status code should be 201
    And the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 200
    When I undeploy the synthetic REST API
    Then the synthetic REST API deployment should become UNDEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 404

  @TC_APIP_DEP_007 @ID_APIP_DEPLOY_006
  Scenario: Deploying to an environment that does not exist is rejected
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    When I create a synthetic REST API in the default project
    And I request a deployment of the synthetic REST API with:
      | environment | ${UNIQUE:missing-env} |
    Then the cloud API response status code should be 400
    And the cloud API response body should contain "not part of the project's deployment pipeline"

  @TC_APIP_DEP_008 @ID_APIP_DEPLOY_007
  # The OpenAPI documents a 4xx, and the code returns 500.
  Scenario: Deploying a build that does not exist is rejected
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic REST API in the default project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudGatewayID}          |
      | buildId     | ${UNIQUE:missing-build}        |
    Then the cloud API response status code should be a client error

  @TC_APIP_DEP_011 @ID_APIP_DEPLOY_008
  Scenario: A deployment the gateway refuses is reported as failed with a reason
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic REST API in the default project with:
      | policies | [{"name":"no-such-policy","version":"v1"}] |
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudGatewayID}          |
    Then the cloud API response status code should be 201
    And the deployment response should list the active gateway
    And the synthetic REST API deployment should become FAILED
    And the synthetic REST API deployment should report a status reason code
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 404

  @TC_APIP_DEP_012 @ID_APIP_DEPLOY_009
  Scenario: A per-deployment endpoint override reaches the gateway in the form the upstream expects
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    And an echo backend is available for deployment overrides
    When I create a synthetic REST API in the default project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment}                         |
      | gatewayId   | ${CTX:cloudGatewayID}                                  |
      | parameters  | {"productionEndpoint":"${CTX:cloudEchoBackendURL}"} |
    Then the cloud API response status code should be 201
    And the deployment response should list the active gateway
    And the deployment response should report the echo backend as the production endpoint
    And the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 200
    And the echo backend should have received a request for "/posts/1"

  @TC_APIP_DEP_015 @ID_APIP_DEPLOY_010
  Scenario: Deploying to a gateway that does not exist is refused and leaves the API undeployed
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic REST API in the default project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment} |
      | gatewayId   | ${UNIQUE:missing-gateway}      |
    Then the cloud API response status code should be 400
    And the cloud API response body should contain "is not in environment"
    And the deployment stage of the active gateway's environment should show the active gateway as NOT_DEPLOYED
