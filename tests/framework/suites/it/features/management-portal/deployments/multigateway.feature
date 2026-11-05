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

@multigateway @TENANT
Feature: An API deployed through platform-api reaches every gateway it is deployed to
  As an API platform operator running more than one gateway environment
  I want an API deployed to one gateway to leave every OTHER gateway unaffected, and an
  undeploy from one gateway to leave every other gateway still serving it
  So that operating multiple gateway environments behind one control plane is genuinely
  independent per gateway, not accidentally shared state

  Background:
    Given the first gateway is running
    And the second gateway is running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "multigw-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  Scenario: An API deployed to two gateways is served by both, and undeploy is isolated
    Given I generate a unique resource name from "multigw" and store it as "apiHandle"
    And I generate a unique API context from "/multigw" and store it as "apiContext"
    When I create a REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}"
    And I deploy the "RestApi" "${CTX:apiHandle}" to the first gateway via the control plane
    Then I send a "GET" request to the first gateway "${CTX:apiContext}/health" until status 200

    When I deploy the "RestApi" "${CTX:apiHandle}" to the second gateway via the control plane and store the deployment id as "deploymentId2"
    Then I send a "GET" request to the second gateway "${CTX:apiContext}/health" until status 200
    And I send a "GET" request to the first gateway "${CTX:apiContext}/health" until status 200

    When I undeploy the "RestApi" "${CTX:apiHandle}" deployment "${CTX:deploymentId2}" from the second gateway via the control plane
    Then I send a "GET" request to the second gateway "${CTX:apiContext}/health" until status 404
    And I send a "GET" request to the first gateway "${CTX:apiContext}/health" until status 200

  @TENANT-006
  Scenario: A gateway gets only what is deployed to it, so another gateway of the organization gets neither the API nor its secret
    Given I generate a unique resource name from "multigw-secret" and store it as "secretHandle"
    And I create a secret "${CTX:secretHandle}" via the control plane
    And I generate a unique resource name from "multigw-scoped" and store it as "apiHandle"
    And I generate a unique API context from "/multigw-scoped" and store it as "apiContext"
    And I create a REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" and a policy header secret "${CTX:secretHandle}"
    And I deploy the "RestApi" "${CTX:apiHandle}" to the first gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/rest-apis/${CTX:apiHandle}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    And I obtain an API key for the first gateway via the control plane
    And I resolve the gateway internal artifact id of REST API deployment "${CTX:deploymentId}" and store it as "apiUuid"
    When I send a "GET" request to the gateway internal API at "/apis/${CTX:apiUuid}"
    Then the response status code should be 200
    When I send a "GET" request to the gateway internal API at "/secrets"
    Then the response status code should be 200
    And the response body should contain "${CTX:secretHandle}"
    When I obtain an API key for the second gateway via the control plane
    And I send a "GET" request to the gateway internal API at "/apis/${CTX:apiUuid}"
    Then the response status code should be 404
    When I send a "GET" request to the gateway internal API at "/secrets"
    Then the response status code should be 200
    And the JSON response array field "list" should have 0 items
