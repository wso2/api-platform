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

  # This test skipped until fix the issue of stale data while project creation. 
  @known-issue 
  @TC_APIP_DEP_006 @ID_APIP_DEPLOY_011
  Scenario: Deploying to a gateway that belongs to another environment is refused and leaves the API undeployed
    Given I obtain an APIP cloud console token
    And I find the pipeline-bound APIP project
    And I find a promotion chain of 2 environments with an active gateway each in the project's deployment pipeline
    When I create a synthetic REST API in the pipeline-bound APIP project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudPipelineEnvironment1} |
      | gatewayId   | ${CTX:cloudPipelineGateway2}     |
    Then the cloud API response status code should be 400
    And the cloud API response body should contain "is not in environment"
    And the deployment stage of pipeline environment 2 should show no gateway as deployed

  @ID_APIP_DEPLOY_012
  Scenario: Deploying a build serves that build and not a later edit, and the edit is served once it is built and deployed
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic REST API in the default project
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 200
    When I update the synthetic REST API with:
      | operations | [{"name":"getPostOne","request":{"method":"GET","path":"/posts/1"}},{"name":"getPostTwo","request":{"method":"GET","path":"/posts/2"}}] |
    Then the synthetic REST API should have an operation named "getPostTwo"
    And a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/2" should keep returning 404 for 20 seconds
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 200
    When I prepare a build of the synthetic REST API
    And I deploy build 2 of the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/2" until status 200
    And the synthetic REST API data-plane response should contain post 2

  @ID_APIP_DEPLOY_013
  Scenario: Deploying an earlier build again serves it again
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic REST API in the default project
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    When I update the synthetic REST API with:
      | operations | [{"name":"getPostOne","request":{"method":"GET","path":"/posts/1"}},{"name":"getPostTwo","request":{"method":"GET","path":"/posts/2"}}] |
    And I redeploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/2" until status 200
    When I deploy build 1 of the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/2" until status 404
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 200
    And the synthetic REST API data-plane response should contain post 1

  @ID_APIP_DEPLOY_014
  Scenario: A build a gateway is serving cannot be deleted, and once it is undeployed it can be
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic REST API in the default project
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    When I delete build 1 of the synthetic REST API
    Then the cloud API response status code should be 409
    And the cloud API error code should be "BUILD_IN_USE"
    And the synthetic REST API builds should include build 1
    When I undeploy the synthetic REST API
    Then the synthetic REST API deployment should become UNDEPLOYED
    When I delete build 1 of the synthetic REST API
    Then the cloud API response status code should be 204
    And the synthetic REST API builds should not include build 1

  # 5 is the platform's default number of builds an API holds.
  @ID_APIP_DEPLOY_015
  Scenario: Preparing builds past the limit removes the oldest free build, keeps the one a gateway serves, and never reuses an identifier
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic REST API in the default project
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    When I prepare 5 builds of the synthetic REST API
    Then the synthetic REST API should hold 5 builds
    And the synthetic REST API builds should include build 1
    And the synthetic REST API builds should not include build 2
    And the synthetic REST API builds should include build 3
    And the synthetic REST API builds should list build 6 first
    And the builds of the synthetic REST API should all have different identifiers

  @ID_APIP_DEPLOY_016 @type:negative
  Scenario: Undeploying from an environment the deployment is not in, or a second time, is refused and changes nothing
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic REST API in the default project
    And I deploy the synthetic REST API to the active gateway
    Then the synthetic REST API deployment should become DEPLOYED
    When I try to undeploy the synthetic REST API from the environment "${UNIQUE:missing-env}"
    Then the cloud API response status code should be 400
    And the cloud API response body should contain "is not in environment"
    And the synthetic REST API deployment should become DEPLOYED
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 200
    When I undeploy the synthetic REST API
    Then the synthetic REST API deployment should become UNDEPLOYED
    When I try to undeploy the synthetic REST API
    Then the cloud API response status code should be 409
    And the cloud API error code should be "DEPLOYMENT_NOT_ACTIVE"
    And the synthetic REST API deployment should become UNDEPLOYED

  @ID_APIP_DEPLOY_017 @type:negative
  Scenario: A build is deleted only through its own API and kind, and through another it is not found and stays listed
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    When I create a synthetic REST API in the default project
    And I prepare a build of the synthetic REST API
    And I create a new version "2.0.0" of the synthetic REST API with:
      | operations | [{"name":"getPostTwo","request":{"method":"GET","path":"/posts/2"}}] |
    When I delete build 1 of the synthetic REST API through the new API version
    Then the cloud API response status code should be 404
    When I delete build 1 of the synthetic REST API as a GraphQL API
    Then the cloud API response status code should be 404
    And the synthetic REST API builds should include build 1
    When I delete build 1 of the synthetic REST API
    Then the cloud API response status code should be 204
    And the synthetic REST API builds should not include build 1

  @ID_APIP_DEPLOY_018 @type:negative
  Scenario: A build or deployment request that is not JSON, or a deployment that names no environment, is refused and creates nothing
    Given I obtain an APIP cloud console token
    And I find the default APIP project
    And I find an active APIP gateway
    When I create a synthetic REST API in the default project
    And I send a build request for the synthetic REST API whose body is not JSON
    Then the cloud API response status code should be 400
    And the cloud API error code should be "invalid_request"
    When I send a deployment request for the synthetic REST API whose body is not JSON
    Then the cloud API response status code should be 400
    And the cloud API error code should be "invalid_request"
    When I request a deployment of the synthetic REST API with:
      | gatewayId | ${CTX:cloudGatewayID} |
    Then the cloud API response status code should be 400
    And the cloud API response body should contain "Field 'environment' is required."
    And the synthetic REST API should hold 0 builds
    And the deployment stage of the active gateway's environment should show the active gateway as NOT_DEPLOYED
    When I prepare a build of the synthetic REST API
    Then the synthetic REST API should hold 1 builds
    When I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudGatewayID}          |
    Then the cloud API response status code should be 201
    And the synthetic REST API deployment should become DEPLOYED
