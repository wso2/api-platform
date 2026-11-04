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

@DSYNC
Feature: A gateway catches up on deployments made while its controller was offline
  As an API publisher
  I want deploys and undeploys made while a gateway is disconnected to take effect when it returns
  So that a gateway outage never leaves it serving the wrong APIs or reporting the wrong status

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique resource name from "dsync-proj" and store it as "project"
    And I generate a unique resource name from "dsync-api" and store it as "api"
    And I generate a unique API context from "/dsync" and store it as "apiContext"
    And I create a project "${CTX:project}" on the control plane
    And I store the registered gateway id as "gatewayId"
    And I create a REST API "${CTX:api}" via the control plane in project "${CTX:project}" with context "${CTX:apiContext}"

  # The gateway applies the change on reconnect, but its startup sync sends no acknowledgement and
  # platform-api keeps a timed-out deployment FAILED: https://github.com/wso2/api-platform/issues/4016
  @DSYNC-001 @known-issue
  Scenario: A deploy made while the controller is stopped times out, then is served and deployed once it reconnects
    When I stop the gateway service "gateway-controller"
    And I send a "POST" request to the control plane at "/rest-apis/${CTX:api}/deployments" with body:
      """
      {"name": "${UNIQUE:dsync-offline}", "base": "current", "gatewayId": "${CTX:gatewayId}"}
      """
    Then the response status code should be 201
    And the JSON response field "status" should be "DEPLOYING"
    And I store the JSON response field "deploymentId" as "deploymentId"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/deployments/${CTX:deploymentId}" until the JSON field "status" is "FAILED"
    Then the JSON response field "statusReason" should be "DEPLOYMENT_TIMEOUT"
    And I send a "GET" request to "${CTX:apiContext}/health" until status 404
    When I start the gateway service "gateway-controller"
    And I wait for the gateway controller health endpoint
    And I send a "GET" request to the control plane at "/gateways/${CTX:gatewayId}" until the JSON field "isActive" is "true"
    Then I send a "GET" request to "${CTX:apiContext}/health" until status 200
    And I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"

  # The gateway applies the change on reconnect, but its startup sync sends no acknowledgement and
  # platform-api keeps a timed-out deployment FAILED: https://github.com/wso2/api-platform/issues/4016
  @DSYNC-002 @known-issue
  Scenario: An undeploy made while the controller is stopped takes the API off the gateway once it reconnects
    Given I send a "POST" request to the control plane at "/rest-apis/${CTX:api}/deployments" with body:
      """
      {"name": "${UNIQUE:dsync-live}", "base": "current", "gatewayId": "${CTX:gatewayId}"}
      """
    And the response status code should be 201
    And I store the JSON response field "deploymentId" as "deploymentId"
    And I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    And I send a "GET" request to "${CTX:apiContext}/health" until status 200
    When I stop the gateway service "gateway-controller"
    And I send a "POST" request to the control plane at "/rest-apis/${CTX:api}/deployments/${CTX:deploymentId}/undeploy?gatewayId=${CTX:gatewayId}"
    Then the response status code should be 200
    And the JSON response field "status" should be "UNDEPLOYING"
    When I start the gateway service "gateway-controller"
    And I wait for the gateway controller health endpoint
    And I send a "GET" request to the control plane at "/gateways/${CTX:gatewayId}" until the JSON field "isActive" is "true"
    Then I send a "GET" request to "${CTX:apiContext}/health" until status 404
    And I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/deployments/${CTX:deploymentId}" until the JSON field "status" is "UNDEPLOYED"

  # The gateway applies the change on reconnect, but its startup sync sends no acknowledgement and
  # platform-api keeps a timed-out deployment FAILED: https://github.com/wso2/api-platform/issues/4016
  @DSYNC-003 @known-issue
  Scenario: A new build deployed while the controller is stopped is served and deployed once it reconnects
    Given I send a "POST" request to the control plane at "/rest-apis/${CTX:api}/builds"
    And the response status code should be 201
    And I store the JSON response field "buildId" as "buildA"
    And I send a "PUT" request to the control plane at "/rest-apis/${CTX:api}" with body:
      """
      {"displayName": "${CTX:api}", "context": "${CTX:apiContext}", "version": "v1", "projectId": "${CTX:project}", "upstream": {"main": {"url": "http://testbench:3000"}}, "operations": [{"request": {"method": "GET", "path": "/status"}}]}
      """
    And the response status code should be 200
    And I send a "POST" request to the control plane at "/rest-apis/${CTX:api}/builds"
    And the response status code should be 201
    And I store the JSON response field "buildId" as "buildB"
    And I send a "POST" request to the control plane at "/rest-apis/${CTX:api}/deployments" with body:
      """
      {"name": "${UNIQUE:dsync-build-a}", "base": "build", "buildId": "${CTX:buildA}", "gatewayId": "${CTX:gatewayId}"}
      """
    And the response status code should be 201
    And I store the JSON response field "deploymentId" as "deploymentA"
    And I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/deployments/${CTX:deploymentA}" until the JSON field "status" is "DEPLOYED"
    And I send a "GET" request to "${CTX:apiContext}/health" until status 200
    When I stop the gateway service "gateway-controller"
    And I send a "POST" request to the control plane at "/rest-apis/${CTX:api}/deployments" with body:
      """
      {"name": "${UNIQUE:dsync-build-b}", "base": "build", "buildId": "${CTX:buildB}", "gatewayId": "${CTX:gatewayId}"}
      """
    Then the response status code should be 201
    And I store the JSON response field "deploymentId" as "deploymentB"
    When I start the gateway service "gateway-controller"
    And I wait for the gateway controller health endpoint
    And I send a "GET" request to the control plane at "/gateways/${CTX:gatewayId}" until the JSON field "isActive" is "true"
    Then I send a "GET" request to "${CTX:apiContext}/status" until status 200
    And I send a "GET" request to "${CTX:apiContext}/health" until status 404
    And I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/deployments/${CTX:deploymentB}" until the JSON field "status" is "DEPLOYED"
    And the JSON response field "buildId" should be "${CTX:buildB}"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/deployments?status=DEPLOYED"
    Then the JSON response array field "list" should have 1 item
    And the JSON response array "list" should contain an item with "deploymentId" equal to "${CTX:deploymentB}"

  # What the gateway serves after an outage, without the status platform-api reports (#4016), so
  # CI keeps covering the part of DSYNC-001 to 003 the product meets while those are excluded.
  Scenario: A gateway that was offline serves exactly what platform-api changed once it reconnects
    Given I generate a unique resource name from "dsync-new" and store it as "newApi"
    And I generate a unique API context from "/dsync-new" and store it as "newContext"
    And I generate a unique resource name from "dsync-built" and store it as "builtApi"
    And I generate a unique API context from "/dsync-built" and store it as "builtContext"
    And I create a REST API "${CTX:newApi}" via the control plane in project "${CTX:project}" with context "${CTX:newContext}"
    And I create a REST API "${CTX:builtApi}" via the control plane in project "${CTX:project}" with context "${CTX:builtContext}"
    And I deploy the "RestApi" "${CTX:api}" to the gateway via the control plane and store the deployment id as "liveDeployment"
    And I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/deployments/${CTX:liveDeployment}" until the JSON field "status" is "DEPLOYED"
    And I send a "GET" request to "${CTX:apiContext}/health" until status 200
    And I send a "POST" request to the control plane at "/rest-apis/${CTX:builtApi}/builds"
    And the response status code should be 201
    And I store the JSON response field "buildId" as "buildA"
    And I send a "PUT" request to the control plane at "/rest-apis/${CTX:builtApi}" with body:
      """
      {"displayName": "${CTX:builtApi}", "context": "${CTX:builtContext}", "version": "v1", "projectId": "${CTX:project}", "upstream": {"main": {"url": "http://testbench:3000"}}, "operations": [{"request": {"method": "GET", "path": "/status"}}]}
      """
    And the response status code should be 200
    And I send a "POST" request to the control plane at "/rest-apis/${CTX:builtApi}/builds"
    And the response status code should be 201
    And I store the JSON response field "buildId" as "buildB"
    And I send a "POST" request to the control plane at "/rest-apis/${CTX:builtApi}/deployments" with body:
      """
      {"name": "${UNIQUE:dsync-served-a}", "base": "build", "buildId": "${CTX:buildA}", "gatewayId": "${CTX:gatewayId}"}
      """
    And the response status code should be 201
    And I store the JSON response field "deploymentId" as "builtDeployment"
    And I send a "GET" request to the control plane at "/rest-apis/${CTX:builtApi}/deployments/${CTX:builtDeployment}" until the JSON field "status" is "DEPLOYED"
    And I send a "GET" request to "${CTX:builtContext}/health" until status 200
    And I send a "GET" request to "${CTX:newContext}/health" until status 404
    When I stop the gateway service "gateway-controller"
    And I send a "POST" request to the control plane at "/rest-apis/${CTX:newApi}/deployments" with body:
      """
      {"name": "${UNIQUE:dsync-served-new}", "base": "current", "gatewayId": "${CTX:gatewayId}"}
      """
    And the response status code should be 201
    And I send a "POST" request to the control plane at "/rest-apis/${CTX:api}/deployments/${CTX:liveDeployment}/undeploy?gatewayId=${CTX:gatewayId}"
    And the response status code should be 200
    And I send a "POST" request to the control plane at "/rest-apis/${CTX:builtApi}/deployments" with body:
      """
      {"name": "${UNIQUE:dsync-served-b}", "base": "build", "buildId": "${CTX:buildB}", "gatewayId": "${CTX:gatewayId}"}
      """
    And the response status code should be 201
    And I start the gateway service "gateway-controller"
    And I wait for the gateway controller health endpoint
    And I send a "GET" request to the control plane at "/gateways/${CTX:gatewayId}" until the JSON field "isActive" is "true"
    Then I send a "GET" request to "${CTX:newContext}/health" until status 200
    And I send a "GET" request to "${CTX:apiContext}/health" until status 404
    And I send a "GET" request to "${CTX:builtContext}/status" until status 200
    And I send a "GET" request to "${CTX:builtContext}/health" until status 404

  @DSYNC-004
  Scenario: Restarting the controller with nothing changed keeps every API served, deployed and listed once
    Given I generate a unique resource name from "dsync-api-2" and store it as "api2"
    And I generate a unique API context from "/dsync-2" and store it as "apiContext2"
    And I generate a unique resource name from "dsync-api-3" and store it as "api3"
    And I generate a unique API context from "/dsync-3" and store it as "apiContext3"
    And I create a REST API "${CTX:api2}" via the control plane in project "${CTX:project}" with context "${CTX:apiContext2}"
    And I create a REST API "${CTX:api3}" via the control plane in project "${CTX:project}" with context "${CTX:apiContext3}"
    And I deploy the "RestApi" "${CTX:api}" to the gateway via the control plane and store the deployment id as "deployment1"
    And I deploy the "RestApi" "${CTX:api2}" to the gateway via the control plane and store the deployment id as "deployment2"
    And I deploy the "RestApi" "${CTX:api3}" to the gateway via the control plane and store the deployment id as "deployment3"
    And I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/deployments/${CTX:deployment1}" until the JSON field "status" is "DEPLOYED"
    And I send a "GET" request to the control plane at "/rest-apis/${CTX:api2}/deployments/${CTX:deployment2}" until the JSON field "status" is "DEPLOYED"
    And I send a "GET" request to the control plane at "/rest-apis/${CTX:api3}/deployments/${CTX:deployment3}" until the JSON field "status" is "DEPLOYED"
    And I send a "GET" request to "${CTX:apiContext}/health" until status 200
    And I send a "GET" request to "${CTX:apiContext2}/health" until status 200
    And I send a "GET" request to "${CTX:apiContext3}/health" until status 200
    When I restart the "gateway-controller" service
    And I wait for the gateway controller health endpoint
    And I send a "GET" request to the control plane at "/gateways/${CTX:gatewayId}" until the JSON field "isActive" is "true"
    Then I send a "GET" request to "${CTX:apiContext}/health" until status 200
    And I send a "GET" request to "${CTX:apiContext2}/health" until status 200
    And I send a "GET" request to "${CTX:apiContext3}/health" until status 200
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/deployments"
    Then the JSON response array field "list" should have 1 item
    And the JSON response array "list" item with "deploymentId" equal to "${CTX:deployment1}" should have "status" equal to "DEPLOYED"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api2}/deployments"
    Then the JSON response array field "list" should have 1 item
    And the JSON response array "list" item with "deploymentId" equal to "${CTX:deployment2}" should have "status" equal to "DEPLOYED"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api3}/deployments"
    Then the JSON response array field "list" should have 1 item
    And the JSON response array "list" item with "deploymentId" equal to "${CTX:deployment3}" should have "status" equal to "DEPLOYED"
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis?context=${CTX:apiContext}"
    Then the JSON response field "count" should be 1
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis?context=${CTX:apiContext2}"
    Then the JSON response field "count" should be 1
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis?context=${CTX:apiContext3}"
    Then the JSON response field "count" should be 1
