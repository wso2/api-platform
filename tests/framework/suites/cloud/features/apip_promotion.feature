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

  # The scenarios promote through the project that CLOUD_IT_PIPELINE_PROJECT names, which must
  # already be bound to a deployment pipeline over environments that hold an active gateway each.
  # Without it, or without a promotion chain long enough in its pipeline, a scenario is skipped.

  @TC_APIP_PROM_001 @ID_APIP_PROMOTION_002
  Scenario: An API promoted along a pipeline is served by the gateway of the target environment
    Given I obtain an APIP cloud console token
    And I find the pipeline-bound APIP project
    And I find a promotion chain of 2 environments with an active gateway each in the project's deployment pipeline
    When I create a synthetic REST API in the pipeline-bound APIP project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudPipelineEnvironment1} |
      | gatewayId   | ${CTX:cloudPipelineGateway1}     |
    Then the cloud API response status code should be 201
    And the deployment should have rendered a build of the API
    And the synthetic REST API deployment should become DEPLOYED
    When I request a deployment of the synthetic REST API with:
      | environment     | ${CTX:cloudPipelineEnvironment2} |
      | fromEnvironment | ${CTX:cloudPipelineEnvironment1} |
      | gatewayId       | ${CTX:cloudPipelineGateway2}     |
    Then the cloud API response status code should be 201
    And the deployment response should list the gateway of pipeline environment 2
    And the deployed build should be the rendered build
    And the synthetic REST API should be DEPLOYED on every gateway the deployment request named
    And I send a "GET" request to "${CTX:cloudPipelineEndpoint2}/${CTX:cloudAPIID}/v1/posts/1" until status 200
    And the synthetic REST API data-plane response should contain post 1

  @TC_APIP_PROM_002 @ID_APIP_PROMOTION_003
  Scenario: Deploying directly to an environment a promotion path leads to is refused
    Given I obtain an APIP cloud console token
    And I find the pipeline-bound APIP project
    And I find a promotion chain of 2 environments with an active gateway each in the project's deployment pipeline
    When I create a synthetic REST API in the pipeline-bound APIP project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudPipelineEnvironment2} |
    Then the cloud API response status code should be 400
    And the cloud API response body should contain "is a promotion target"
    And the cloud API response body should contain "${CTX:cloudPipelineEnvironment2}"
    And the deployment stage of pipeline environment 2 should show no gateway as deployed

  @TC_APIP_PROM_003 @ID_APIP_PROMOTION_004
  Scenario: A promotion that names no build carries the build the source environment is running
    Given I obtain an APIP cloud console token
    And I find the pipeline-bound APIP project
    And I find a promotion chain of 2 environments with an active gateway each in the project's deployment pipeline
    When I create a synthetic REST API in the pipeline-bound APIP project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudPipelineEnvironment1} |
      | gatewayId   | ${CTX:cloudPipelineGateway1}     |
    Then the cloud API response status code should be 201
    And the deployment should have rendered a build of the API
    And the synthetic REST API deployment should become DEPLOYED
    And I update the synthetic REST API with:
      | operations | [{"name":"getPostTwo","request":{"method":"GET","path":"/posts/2"}}] |
    When I request a deployment of the synthetic REST API with:
      | environment     | ${CTX:cloudPipelineEnvironment2} |
      | fromEnvironment | ${CTX:cloudPipelineEnvironment1} |
      | gatewayId       | ${CTX:cloudPipelineGateway2}     |
    Then the cloud API response status code should be 201
    And the deployment response should list the gateway of pipeline environment 2
    And the deployed build should be the rendered build
    And the synthetic REST API should hold 1 builds
    And the deployment stage of pipeline environment 2 should show its gateway running the rendered build
    And the deployment stage of the active gateway's environment should show the active gateway as DEPLOYED

  @TC_APIP_PROM_004 @ID_APIP_PROMOTION_005
  Scenario: Promoting a build that is not deployed in the source environment is refused
    Given I obtain an APIP cloud console token
    And I find the pipeline-bound APIP project
    And I find a promotion chain of 2 environments with an active gateway each in the project's deployment pipeline
    When I create a synthetic REST API in the pipeline-bound APIP project
    And I request a deployment of the synthetic REST API with:
      | environment     | ${CTX:cloudPipelineEnvironment2} |
      | fromEnvironment | ${CTX:cloudPipelineEnvironment1} |
      | gatewayId       | ${CTX:cloudPipelineGateway2}     |
    Then the cloud API response status code should be 400
    And the cloud API response body should contain "Nothing is deployed in environment"
    When I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudPipelineEnvironment1} |
      | gatewayId   | ${CTX:cloudPipelineGateway1}     |
    Then the cloud API response status code should be 201
    And the deployment should have rendered a build of the API
    And the synthetic REST API deployment should become DEPLOYED
    When I request a deployment of the synthetic REST API with:
      | environment     | ${CTX:cloudPipelineEnvironment2} |
      | fromEnvironment | ${CTX:cloudPipelineEnvironment1} |
      | gatewayId       | ${CTX:cloudPipelineGateway2}     |
      | buildId         | ${UNIQUE:missing-build}          |
    Then the cloud API response status code should be 400
    And the cloud API response body should contain "is not deployed in environment"
    And the cloud API response body should contain "${CTX:cloudDeployedBuildID}"
    And the deployment stage of pipeline environment 2 should show no gateway as deployed

  @TC_APIP_PROM_005 @ID_APIP_PROMOTION_006
  Scenario: Promotion follows the environment chain of the pipeline in order
    Given I obtain an APIP cloud console token
    And I find the pipeline-bound APIP project
    And I find a promotion chain of 3 environments with an active gateway each in the project's deployment pipeline
    When I create a synthetic REST API in the pipeline-bound APIP project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudPipelineEnvironment1} |
      | gatewayId   | ${CTX:cloudPipelineGateway1}     |
    Then the cloud API response status code should be 201
    And the deployment should have rendered a build of the API
    And the synthetic REST API deployment should become DEPLOYED
    When I request a deployment of the synthetic REST API with:
      | environment     | ${CTX:cloudPipelineEnvironment3} |
      | fromEnvironment | ${CTX:cloudPipelineEnvironment1} |
      | gatewayId       | ${CTX:cloudPipelineGateway3}     |
    Then the cloud API response status code should be 400
    And the cloud API response body should contain "has no promotion path from"
    And the deployment stage of pipeline environment 3 should show no gateway as deployed
    When I request a deployment of the synthetic REST API with:
      | environment     | ${CTX:cloudPipelineEnvironment2} |
      | fromEnvironment | ${CTX:cloudPipelineEnvironment1} |
      | gatewayId       | ${CTX:cloudPipelineGateway2}     |
    Then the cloud API response status code should be 201
    And the deployment response should list the gateway of pipeline environment 2
    And the deployed build should be the rendered build
    And the synthetic REST API should be DEPLOYED on every gateway the deployment request named
    When I request a deployment of the synthetic REST API with:
      | environment     | ${CTX:cloudPipelineEnvironment3} |
      | fromEnvironment | ${CTX:cloudPipelineEnvironment2} |
      | gatewayId       | ${CTX:cloudPipelineGateway3}     |
    Then the cloud API response status code should be 201
    And the deployment response should list the gateway of pipeline environment 3
    And the deployed build should be the rendered build
    And the synthetic REST API should be DEPLOYED on every gateway the deployment request named
    And the deployment stage of pipeline environment 3 should show its gateway running the rendered build

  @TC_APIP_PROM_008 @ID_APIP_PROMOTION_007
  Scenario: A promoted deployment can be undeployed in the target environment alone
    Given I obtain an APIP cloud console token
    And I find the pipeline-bound APIP project
    And I find a promotion chain of 2 environments with an active gateway each in the project's deployment pipeline
    When I create a synthetic REST API in the pipeline-bound APIP project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudPipelineEnvironment1} |
      | gatewayId   | ${CTX:cloudPipelineGateway1}     |
    Then the cloud API response status code should be 201
    And the deployment should have rendered a build of the API
    And the synthetic REST API deployment should become DEPLOYED
    When I request a deployment of the synthetic REST API with:
      | environment     | ${CTX:cloudPipelineEnvironment2} |
      | fromEnvironment | ${CTX:cloudPipelineEnvironment1} |
      | gatewayId       | ${CTX:cloudPipelineGateway2}     |
    Then the cloud API response status code should be 201
    And the deployment response should list the gateway of pipeline environment 2
    And the deployed build should be the rendered build
    And the synthetic REST API should be DEPLOYED on every gateway the deployment request named
    When I undeploy the promoted deployment from pipeline environment 2
    Then the deployment stage of pipeline environment 2 should show its gateway as no longer deployed
    And the deployment stage of the active gateway's environment should show the active gateway as DEPLOYED
    And I send a "GET" request to "${CTX:cloudPipelineEndpoint2}/${CTX:cloudAPIID}/v1/posts/1" until status 404
    And I send a "GET" request to "${CTX:cloudPipelineEndpoint1}/${CTX:cloudAPIID}/v1/posts/1" until status 200
