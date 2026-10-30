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

@platform-api-build-deployment
Feature: A gateway serves the build it was given, and a served build is protected
  As an API publisher
  I want a deployment from a build to serve that snapshot even after the API has changed, and
  the build to stay while a gateway serves it
  So that I can roll a gateway back to a known version of an API

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique resource name from "bld-dep-proj" and store it as "project"
    And I generate a unique resource name from "bld-dep-api" and store it as "api"
    And I generate a unique API context from "/bld-dep" and store it as "apiContext"
    And I create a project "${CTX:project}" on the control plane
    And I store the registered gateway id as "gatewayId"
    And I create a REST API "${CTX:api}" via the control plane in project "${CTX:project}" with context "${CTX:apiContext}"

  Scenario: Deploying a build serves that snapshot, and deploying current serves the edit
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:api}/builds"
    Then the response status code should be 201
    And I store the JSON response field "buildId" as "firstBuild"
    When I send a "PUT" request to the control plane at "/rest-apis/${CTX:api}" with body:
      """
      {"displayName": "${CTX:api}", "context": "${CTX:apiContext}", "version": "v1", "projectId": "${CTX:project}", "upstream": {"main": {"url": "http://testbench:3000"}}, "operations": [{"request": {"method": "GET", "path": "/status"}}]}
      """
    Then the response status code should be 200
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:api}/deployments" with body:
      """
      {"name": "${UNIQUE:bld-first}", "base": "build", "buildId": "${CTX:firstBuild}", "gatewayId": "${CTX:gatewayId}"}
      """
    Then the response status code should be 201
    And the JSON response field "buildId" should be "${CTX:firstBuild}"
    And I store the JSON response field "deploymentId" as "firstDeployment"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/deployments/${CTX:firstDeployment}" until the JSON field "status" is "DEPLOYED"
    Then I send a "GET" request to "${CTX:apiContext}/health" until status 200
    When I send a "GET" request to "${CTX:apiContext}/status"
    Then the response status code should be 404
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:api}/deployments" with body:
      """
      {"name": "${UNIQUE:bld-current}", "base": "current", "gatewayId": "${CTX:gatewayId}"}
      """
    Then the response status code should be 201
    And the JSON response field "buildId" should not equal "${CTX:firstBuild}"
    And I store the JSON response field "buildId" as "currentBuild"
    And I store the JSON response field "deploymentId" as "currentDeployment"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/deployments/${CTX:currentDeployment}" until the JSON field "status" is "DEPLOYED"
    Then I send a "GET" request to "${CTX:apiContext}/status" until status 200
    And I send a "GET" request to "${CTX:apiContext}/health" until status 404
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/builds"
    Then the JSON response array "list" should contain an item with "buildId" equal to "${CTX:currentBuild}"
    And the JSON response array "list" should contain an item with "buildId" equal to "${CTX:firstBuild}"

  Scenario: A build a gateway serves cannot be deleted until it is undeployed
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:api}/builds"
    Then the response status code should be 201
    And I store the JSON response field "buildId" as "build"
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:api}/deployments" with body:
      """
      {"name": "${UNIQUE:bld-served}", "base": "build", "buildId": "${CTX:build}", "gatewayId": "${CTX:gatewayId}"}
      """
    Then the response status code should be 201
    And I store the JSON response field "deploymentId" as "deployment"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/deployments/${CTX:deployment}" until the JSON field "status" is "DEPLOYED"
    And I send a "DELETE" request to the control plane at "/rest-apis/${CTX:api}/builds/${CTX:build}"
    Then the response status code should be 409
    And the JSON response field "code" should be "BUILD_IN_USE"
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:api}/deployments/${CTX:deployment}/undeploy?gatewayId=${CTX:gatewayId}"
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/deployments/${CTX:deployment}" until the JSON field "status" is "UNDEPLOYED"
    And I send a "DELETE" request to the control plane at "/rest-apis/${CTX:api}/builds/${CTX:build}"
    Then the response status code should be 204
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/deployments/${CTX:deployment}"
    Then the response status code should be 200
    And the JSON response field "buildId" should not exist
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:api}/deployments/${CTX:deployment}/restore?gatewayId=${CTX:gatewayId}"
    Then the response status code should be 200
    And I send a "GET" request to "${CTX:apiContext}/health" until status 200

  # The control plane never returns the credential. What a gateway holds depends on its release:
  # one that cannot resolve a secret reference itself is sent the resolved value.
  Scenario: An LLM provider deployment carries its own upstream credential without exposing it
    Given I generate a unique resource name from "bld-llm-key" and store it as "providerSecret"
    And I generate a unique resource name from "bld-gw-key" and store it as "gatewaySecret"
    And I generate a unique resource name from "bld-llm" and store it as "provider"
    And I generate a unique API context from "/bld-llm" and store it as "providerContext"
    And I create a secret "${CTX:providerSecret}" via the control plane
    And I create a secret "${CTX:gatewaySecret}" with value "per-gateway-credential" via the control plane
    And I create an LLM provider "${CTX:provider}" via the control plane with context "${CTX:providerContext}" referencing template "openai" and secret "${CTX:providerSecret}"
    When I send a "POST" request to the control plane at "/llm-providers/${CTX:provider}/builds"
    Then the response status code should be 201
    And I store the JSON response field "buildId" as "build"
    When I send a "POST" request to the control plane at "/llm-providers/${CTX:provider}/deployments" with body:
      """
      {"name": "${UNIQUE:bld-llm-literal}", "base": "build", "buildId": "${CTX:build}", "gatewayId": "${CTX:gatewayId}", "metadata": {"upstreamAuthValue": "a-literal-credential", "upstreamAuthHeader": "X-Upstream-Key"}}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "LLM_PROVIDER_DEPLOYMENT_VALIDATION_FAILED"
    When I send a "POST" request to the control plane at "/llm-providers/${CTX:provider}/deployments" with body:
      """
      {"name": "${UNIQUE:bld-llm-badheader}", "base": "build", "buildId": "${CTX:build}", "gatewayId": "${CTX:gatewayId}", "metadata": {"upstreamAuthValue": "{{ secret \"${CTX:gatewaySecret}\" }}", "upstreamAuthHeader": "X Upstream Key"}}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "LLM_PROVIDER_DEPLOYMENT_VALIDATION_FAILED"
    When I send a "POST" request to the control plane at "/llm-providers/${CTX:provider}/deployments" with body:
      """
      {"name": "${UNIQUE:bld-llm-dep}", "base": "build", "buildId": "${CTX:build}", "gatewayId": "${CTX:gatewayId}", "metadata": {"upstreamAuthValue": "{{ secret \"${CTX:gatewaySecret}\" }}", "upstreamAuthHeader": "X-Upstream-Key"}}
      """
    Then the response status code should be 201
    And the JSON response field "buildId" should be "${CTX:build}"
    And the JSON response field "metadata.upstreamAuthHeader" should be "X-Upstream-Key"
    And the JSON response field "metadata.upstreamAuthValue" should not exist
    And I store the JSON response field "deploymentId" as "deployment"
    When I send a "GET" request to the control plane at "/llm-providers/${CTX:provider}/deployments/${CTX:deployment}" until the JSON field "status" is "DEPLOYED"
    Then the JSON response field "metadata.upstreamAuthValue" should not exist
    And the JSON response field "buildId" should be "${CTX:build}"
    When I send a "GET" request to the "gateway-controller" service at "/llm-providers/${CTX:provider}" until status 200
    Then the response body should contain "X-Upstream-Key"
