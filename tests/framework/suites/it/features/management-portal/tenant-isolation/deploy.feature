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

@TENANT
Feature: An organization cannot deploy to another organization's gateway
  As an organization administrator
  I want a deployment aimed at another organization's gateway to be refused as not found
  So that nothing I publish can ever run on a gateway outside my organization

  Background:
    Given the first gateway is running
    And the second gateway is running
    And I generate a unique value from "tenant-deploy-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  @TENANT-004
  Scenario: Deploying to another organization's connected gateway is refused as not found and nothing reaches it
    Given I store the second gateway id as "orgBGateway"
    And I send a "GET" request to the control plane at "/gateways/${CTX:orgBGateway}" as "org B admin" until the JSON field "isActive" is "true"
    And I generate a unique resource name from "tenant-deploy-api" and store it as "apiHandle"
    And I generate a unique API context from "/tenant-deploy-api" and store it as "apiContext"
    And I create a REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}"
    And I send a "GET" request to the second gateway "${CTX:apiContext}/health" until status 404
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:apiHandle}/deployments" with body:
      """
      {"name": "${CTX:apiHandle}-to-org-b", "base": "current", "gatewayId": "${CTX:orgBGateway}"}
      """
    Then the response status code should be 404
    And the JSON response field "code" should be "GATEWAY_NOT_FOUND"
    And the JSON response field "message" should be "The specified gateway could not be found."
    When I deploy the "RestApi" "${CTX:apiHandle}" to the first gateway via the control plane
    Then I send a "GET" request to the first gateway "${CTX:apiContext}/health" until status 200
    And I send a "GET" request to the second gateway "${CTX:apiContext}/health" and it never returns status 200
