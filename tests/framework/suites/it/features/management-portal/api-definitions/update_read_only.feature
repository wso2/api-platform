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

@APIDEF
Feature: A gateway-originated API keeps its operations when its definition is replaced
  As an API publisher
  I want a definition uploaded for an API that a gateway owns to change only the stored document
  So that the gateway stays the source of what that API serves

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique resource name from "apidef-proj" and store it as "project"
    And I generate a unique resource name from "apidef-api" and store it as "api"
    And I generate a unique API context from "/apidef" and store it as "apiContext"
    And I create a project "${CTX:project}" on the control plane

  @APIDEF-006
  Scenario: A definition uploaded for a gateway-originated API is stored, but its operations, its gateway and its deployments do not change
    Given I store the registered gateway id as "gatewayId"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                                     |
      | name                   | ${CTX:api}                                                    |
      | spec.displayName       | ${CTX:api}                                                    |
      | spec.version           | v1.0                                                          |
      | spec.context           | ${CTX:apiContext}                                             |
      | spec.upstream.main.url | http://testbench:3000                                         |
      | spec.operations        | [{"method":"GET","path":"/store/inventory"}]                  |
      | metadata.annotations   | {"gateway.api-platform.wso2.com/project-id":"${CTX:project}"} |
    Then the response should be successful
    And the control plane should receive the "RestApi" artifact "${CTX:api}"
    And the control plane copy of the "RestApi" artifact "${CTX:api}" should be marked as gateway-originated
    And I send a "GET" request to "${CTX:apiContext}/store/inventory" until status 200
    When I send a multipart "PUT" request to the control plane at "/rest-apis/${CTX:api}/openapi" with fields:
      | file | @spec:petstore-v1.yaml |
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/openapi"
    Then the JSON response field "content" should be spec fixture "petstore-v1.yaml"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}"
    Then the JSON response array field "operations" should have 1 item
    And the JSON response array "operations" should not contain an item with "request.path" equal to "/pets/{petId}"
    When I send a "POST" request to the control plane at "/rest-apis/${CTX:api}/deployments" with body:
      """
      {"name": "${UNIQUE:apidef-readonly}", "base": "current", "gatewayId": "${CTX:gatewayId}"}
      """
    Then the response status code should be 403
    And the JSON response field "code" should be "ARTIFACT_READ_ONLY"
    When I send a "GET" request to "${CTX:apiContext}/pets/42"
    Then the response status code should be 404
    When I send a "GET" request to "${CTX:apiContext}/store/inventory"
    Then the response status code should be 200
