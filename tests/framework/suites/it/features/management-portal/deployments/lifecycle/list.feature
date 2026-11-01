# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

@DEPLC
Feature: An artifact's deployments list filters by gateway and by status
  As an API publisher running an artifact on more than one gateway
  I want to list its deployments for one gateway or in one state
  So that I can see where it runs and what I can restore

  Background:
    Given the first gateway is running
    And the second gateway is running
    And I generate a unique resource name from "deplc-list-proj" and store it as "project"
    And I generate a unique resource name from "deplc-list" and store it as "artifact"
    And I generate a unique API context from "/deplc-list" and store it as "artifactContext"
    And I create a project "${CTX:project}" on the control plane
    And I store the first gateway id as "firstGateway"
    And I store the second gateway id as "secondGateway"

  @DEPLC-007
  Scenario Outline: Listing deployments on two gateways, unfiltered and filtered by gateway and by status
    Given I create a deployable "<kind>" "${CTX:artifact}" via the control plane in project "${CTX:project}" with context "${CTX:artifactContext}"
    And I deploy the "<kind>" "${CTX:artifact}" to the first gateway via the control plane and store the deployment id as "deployed"
    And I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:deployed}" until the JSON field "status" is "DEPLOYED"
    And I deploy the "<kind>" "${CTX:artifact}" to the second gateway via the control plane and store the deployment id as "undeployed"
    And I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:undeployed}" until the JSON field "status" is "DEPLOYED"
    And I send a "POST" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:undeployed}/undeploy?gatewayId=${CTX:secondGateway}"
    And the response status code should be 200
    And I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments/${CTX:undeployed}" until the JSON field "status" is "UNDEPLOYED"
    When I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments"
    Then the response status code should be 200
    And the JSON response array field "list" should have 2 items
    And the JSON response array "list" item with "deploymentId" equal to "${CTX:deployed}" should have "gatewayId" equal to "${CTX:firstGateway}"
    And the JSON response array "list" item with "deploymentId" equal to "${CTX:deployed}" should have "status" equal to "DEPLOYED"
    And the JSON response array "list" item with "deploymentId" equal to "${CTX:undeployed}" should have "gatewayId" equal to "${CTX:secondGateway}"
    And the JSON response array "list" item with "deploymentId" equal to "${CTX:undeployed}" should have "status" equal to "UNDEPLOYED"
    When I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments?gatewayId=${CTX:firstGateway}"
    Then the response status code should be 200
    And the JSON response array field "list" should have 1 item
    And the JSON response array "list" should contain an item with "deploymentId" equal to "${CTX:deployed}"
    When I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments?gatewayId=${CTX:secondGateway}"
    Then the response status code should be 200
    And the JSON response array field "list" should have 1 item
    And the JSON response array "list" should contain an item with "deploymentId" equal to "${CTX:undeployed}"
    When I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments?status=DEPLOYED"
    Then the response status code should be 200
    And the JSON response array field "list" should have 1 item
    And the JSON response array "list" should contain an item with "deploymentId" equal to "${CTX:deployed}"
    When I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments?status=UNDEPLOYED"
    Then the response status code should be 200
    And the JSON response array field "list" should have 1 item
    And the JSON response array "list" should contain an item with "deploymentId" equal to "${CTX:undeployed}"
    When I send a "GET" request to the control plane at "/<collection>/${CTX:artifact}/deployments?status=RUNNING"
    Then the response status code should be 400
    And the JSON response field "code" should be "DEPLOYMENT_INVALID_STATUS"

    Examples:
      | kind        | collection    |
      | LlmProvider | llm-providers |
      | LlmProxy    | llm-proxies   |
      | Mcp         | mcp-proxies   |

    @graphql
    Examples:
      | kind       | collection   |
      | GraphQLApi | graphql-apis |
