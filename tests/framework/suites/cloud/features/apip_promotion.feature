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

  # Known issue: the gateway takes longer to activate than the wait allows (B25).
  @TC_APIP_PROM_001 @known-issue @known-issue-1 @ID_APIP_PROMOTION_001
  Scenario: An API promoted along a pipeline is served by the gateway of the target environment
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create 1 synthetic non-production APIP environment
    And I create a synthetic managed APIP gateway in each synthetic APIP environment
    And the synthetic managed APIP gateways should become active
    And I create a synthetic deployment pipeline with the promotion paths:
      | source                         | targets                           |
      | ${CTX:cloudGatewayEnvironment} | ${CTX:cloudSyntheticEnvironment1} |
    And I create a synthetic APIP project bound to the synthetic deployment pipeline
    When I create a synthetic REST API in the synthetic APIP project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudGatewayID}          |
    Then the cloud API response status code should be 201
    And the deployment should have rendered a build of the API
    And the synthetic REST API deployment should become DEPLOYED
    When I request a deployment of the synthetic REST API with:
      | environment     | ${CTX:cloudSyntheticEnvironment1} |
      | fromEnvironment | ${CTX:cloudGatewayEnvironment}    |
      | gatewayId       | ${CTX:cloudSyntheticGateway1}     |
    Then the cloud API response status code should be 201
    And the deployment response should list the gateway of synthetic APIP environment 1
    And the deployed build should be the rendered build
    And the synthetic REST API should be DEPLOYED on every gateway the deployment request named
    When I read the endpoint of the gateway of synthetic APIP environment 1
    Then I send a "GET" request to "${CTX:cloudSyntheticEndpoint1}/${CTX:cloudAPIID}/v1/posts/1" until status 200
    And the synthetic REST API data-plane response should contain post 1

  @TC_APIP_PROM_002 @ID_APIP_PROMOTION_002
  Scenario: Deploying directly to an environment a promotion path leads to is refused
    Given I obtain an APIP cloud console token
    And I find the environment of the default APIP gateway
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

  @TC_APIP_PROM_003 @ID_APIP_PROMOTION_003
  Scenario: A promotion that names no build carries the build the source environment is running
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create 1 synthetic non-production APIP environment
    And I create a synthetic managed APIP gateway in each synthetic APIP environment
    And I create a synthetic deployment pipeline with the promotion paths:
      | source                         | targets                           |
      | ${CTX:cloudGatewayEnvironment} | ${CTX:cloudSyntheticEnvironment1} |
    And I create a synthetic APIP project bound to the synthetic deployment pipeline
    When I create a synthetic REST API in the synthetic APIP project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudGatewayID}          |
    Then the cloud API response status code should be 201
    And the deployment should have rendered a build of the API
    And the synthetic REST API deployment should become DEPLOYED
    And I update the synthetic REST API with:
      | operations | [{"name":"getPostTwo","request":{"method":"GET","path":"/posts/2"}}] |
    When I request a deployment of the synthetic REST API with:
      | environment     | ${CTX:cloudSyntheticEnvironment1} |
      | fromEnvironment | ${CTX:cloudGatewayEnvironment}    |
      | gatewayId       | ${CTX:cloudSyntheticGateway1}     |
    Then the cloud API response status code should be 201
    And the deployment response should list the gateway of synthetic APIP environment 1
    And the deployed build should be the rendered build
    And the synthetic REST API should hold 1 builds
    And the deployment stage of synthetic APIP environment 1 should show its gateway running the rendered build
    And the deployment stage of the active gateway's environment should show the active gateway as DEPLOYED

  @TC_APIP_PROM_004 @ID_APIP_PROMOTION_004
  Scenario: Promoting a build that is not deployed in the source environment is refused
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create 1 synthetic non-production APIP environment
    And I create a synthetic managed APIP gateway in each synthetic APIP environment
    And I create a synthetic deployment pipeline with the promotion paths:
      | source                         | targets                           |
      | ${CTX:cloudGatewayEnvironment} | ${CTX:cloudSyntheticEnvironment1} |
    And I create a synthetic APIP project bound to the synthetic deployment pipeline
    When I create a synthetic REST API in the synthetic APIP project
    And I request a deployment of the synthetic REST API with:
      | environment     | ${CTX:cloudSyntheticEnvironment1} |
      | fromEnvironment | ${CTX:cloudGatewayEnvironment}    |
      | gatewayId       | ${CTX:cloudSyntheticGateway1}     |
    Then the cloud API response status code should be 400
    And the cloud API response body should contain "Nothing is deployed in environment"
    When I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudGatewayID}          |
    Then the cloud API response status code should be 201
    And the deployment should have rendered a build of the API
    And the synthetic REST API deployment should become DEPLOYED
    When I request a deployment of the synthetic REST API with:
      | environment     | ${CTX:cloudSyntheticEnvironment1} |
      | fromEnvironment | ${CTX:cloudGatewayEnvironment}    |
      | gatewayId       | ${CTX:cloudSyntheticGateway1}     |
      | buildId         | ${UNIQUE:missing-build}           |
    Then the cloud API response status code should be 400
    And the cloud API response body should contain "is not deployed in environment"
    And the cloud API response body should contain "${CTX:cloudDeployedBuildID}"
    And the deployment stage of synthetic APIP environment 1 should show no gateway as deployed

  # Known issue: the gateway takes longer to activate than the wait allows (B25).
  @TC_APIP_PROM_005 @known-issue @known-issue-1 @ID_APIP_PROMOTION_005
  Scenario: Promotion follows the environment chain of the pipeline in order
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create 2 synthetic non-production APIP environments
    And I create a synthetic managed APIP gateway in each synthetic APIP environment
    And the synthetic managed APIP gateways should become active
    And I create a synthetic deployment pipeline with the promotion paths:
      | source                            | targets                           |
      | ${CTX:cloudGatewayEnvironment}    | ${CTX:cloudSyntheticEnvironment1} |
      | ${CTX:cloudSyntheticEnvironment1} | ${CTX:cloudSyntheticEnvironment2} |
    And I create a synthetic APIP project bound to the synthetic deployment pipeline
    When I create a synthetic REST API in the synthetic APIP project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudGatewayID}          |
    Then the cloud API response status code should be 201
    And the deployment should have rendered a build of the API
    And the synthetic REST API deployment should become DEPLOYED
    When I request a deployment of the synthetic REST API with:
      | environment     | ${CTX:cloudSyntheticEnvironment2} |
      | fromEnvironment | ${CTX:cloudGatewayEnvironment}    |
      | gatewayId       | ${CTX:cloudSyntheticGateway2}     |
    Then the cloud API response status code should be 400
    And the cloud API response body should contain "has no promotion path from"
    And the deployment stage of synthetic APIP environment 2 should show no gateway as deployed
    When I request a deployment of the synthetic REST API with:
      | environment     | ${CTX:cloudSyntheticEnvironment1} |
      | fromEnvironment | ${CTX:cloudGatewayEnvironment}    |
      | gatewayId       | ${CTX:cloudSyntheticGateway1}     |
    Then the cloud API response status code should be 201
    And the deployment response should list the gateway of synthetic APIP environment 1
    And the deployed build should be the rendered build
    And the synthetic REST API should be DEPLOYED on every gateway the deployment request named
    When I request a deployment of the synthetic REST API with:
      | environment     | ${CTX:cloudSyntheticEnvironment2} |
      | fromEnvironment | ${CTX:cloudSyntheticEnvironment1} |
      | gatewayId       | ${CTX:cloudSyntheticGateway2}     |
    Then the cloud API response status code should be 201
    And the deployment response should list the gateway of synthetic APIP environment 2
    And the deployed build should be the rendered build
    And the synthetic REST API should be DEPLOYED on every gateway the deployment request named
    And the deployment stage of synthetic APIP environment 2 should show its gateway running the rendered build

  @TC_APIP_PROM_006 @ID_APIP_PROMOTION_006
  Scenario: An organisation-level artifact cannot be promoted and the refusal names the environment to deploy to
    Given I obtain an APIP cloud console token
    And I find the environment of the default APIP gateway
    When I request a deployment of an LLM provider that does not exist with:
      | environment     | ${CTX:cloudGatewayEnvironment} |
      | fromEnvironment | ${UNIQUE:source-environment}   |
    Then the cloud API response status code should be 400
    And the cloud API response body should contain "no deployment pipeline"
    And the cloud API response body should contain "${CTX:cloudGatewayEnvironment}"

  @TC_APIP_PROM_007 @ID_APIP_PROMOTION_007
  Scenario: Promoting into an environment that has no gateway is refused
    Given I obtain an APIP cloud console token
    And I find the environment of the default APIP gateway
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

  # Known issue: the gateway takes longer to activate than the wait allows (B25).
  @TC_APIP_PROM_008 @known-issue @known-issue-1 @ID_APIP_PROMOTION_008
  Scenario: A promoted deployment can be undeployed in the target environment alone
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create 1 synthetic non-production APIP environment
    And I create a synthetic managed APIP gateway in each synthetic APIP environment
    And the synthetic managed APIP gateways should become active
    And I create a synthetic deployment pipeline with the promotion paths:
      | source                         | targets                           |
      | ${CTX:cloudGatewayEnvironment} | ${CTX:cloudSyntheticEnvironment1} |
    And I create a synthetic APIP project bound to the synthetic deployment pipeline
    When I create a synthetic REST API in the synthetic APIP project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudGatewayID}          |
    Then the cloud API response status code should be 201
    And the deployment should have rendered a build of the API
    And the synthetic REST API deployment should become DEPLOYED
    When I request a deployment of the synthetic REST API with:
      | environment     | ${CTX:cloudSyntheticEnvironment1} |
      | fromEnvironment | ${CTX:cloudGatewayEnvironment}    |
      | gatewayId       | ${CTX:cloudSyntheticGateway1}     |
    Then the cloud API response status code should be 201
    And the deployment response should list the gateway of synthetic APIP environment 1
    And the deployed build should be the rendered build
    And the synthetic REST API should be DEPLOYED on every gateway the deployment request named
    When I undeploy the promoted deployment from synthetic APIP environment 1
    Then the deployment stage of synthetic APIP environment 1 should show its gateway as no longer deployed
    And the deployment stage of the active gateway's environment should show the active gateway as DEPLOYED
    When I read the endpoint of the gateway of synthetic APIP environment 1
    Then I send a "GET" request to "${CTX:cloudSyntheticEndpoint1}/${CTX:cloudAPIID}/v1/posts/1" until status 404
    And I send a "GET" request to "${CTX:cloudGatewayEndpoint}/${CTX:cloudAPIID}/v1/posts/1" until status 200

  @TC_APIP_PROM_009 @ID_APIP_PROMOTION_009
  Scenario: Which environments can be deployed to directly follows the pipeline as it is now
    Given I obtain an APIP cloud console token
    And I find an active APIP gateway
    And I create 1 synthetic non-production APIP environment
    And I create a synthetic managed APIP gateway in each synthetic APIP environment
    And I create a synthetic deployment pipeline with the promotion paths:
      | source                         | targets                           |
      | ${CTX:cloudGatewayEnvironment} | ${CTX:cloudSyntheticEnvironment1} |
    And I create a synthetic APIP project bound to the synthetic deployment pipeline
    When I create a synthetic REST API in the synthetic APIP project
    And I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudSyntheticEnvironment1} |
      | gatewayId   | ${CTX:cloudSyntheticGateway1}     |
    Then the cloud API response status code should be 400
    And the cloud API response body should contain "is a promotion target"
    When I replace the promotion paths of the synthetic deployment pipeline with:
      | source                            | targets                        |
      | ${CTX:cloudSyntheticEnvironment1} | ${CTX:cloudGatewayEnvironment} |
    Then the cloud API response status code should be 200
    When I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudSyntheticEnvironment1} |
      | gatewayId   | ${CTX:cloudSyntheticGateway1}     |
    Then the cloud API response status code should be 201
    And the deployment response should list the gateway of synthetic APIP environment 1
    When I request a deployment of the synthetic REST API with:
      | environment | ${CTX:cloudGatewayEnvironment} |
      | gatewayId   | ${CTX:cloudGatewayID}          |
    Then the cloud API response status code should be 400
    And the cloud API response body should contain "is a promotion target"
    And the cloud API response body should contain "${CTX:cloudGatewayEnvironment}"
