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
Feature: Replacing an API's OpenAPI definition updates its operations and keeps what still applies
  As an API publisher
  I want to upload a new definition for an API I already have
  So that new operations are added and removed ones go away, without losing the policies I set

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique resource name from "apidef-proj" and store it as "project"
    And I generate a unique resource name from "apidef-api" and store it as "api"
    And I generate a unique API context from "/apidef" and store it as "apiContext"
    And I create a project "${CTX:project}" on the control plane

  @APIDEF-004
  Scenario: A new definition adds and removes operations, an operation it keeps keeps its policy, and a redeploy serves the result
    Given I send a multipart "POST" request to the control plane at "/rest-apis/import-openapi" with fields:
      | id          | ${CTX:api}                                 |
      | displayName | ${CTX:api}                                 |
      | version     | v1.0                                       |
      | context     | ${CTX:apiContext}                          |
      | projectId   | ${CTX:project}                             |
      | upstream    | {"main": {"url": "http://testbench:3000"}} |
      | file        | @spec:petstore-v1.yaml                     |
    And the response status code should be 201
    And I send a "PUT" request to the control plane at "/rest-apis/${CTX:api}" with body:
      """
      {"displayName": "${CTX:api}", "context": "${CTX:apiContext}", "version": "v1.0", "projectId": "${CTX:project}", "upstream": {"main": {"url": "http://testbench:3000"}}, "operations": [{"name": "listPets", "request": {"method": "GET", "path": "/pets", "policies": [{"name": "set-headers", "version": "v1", "params": {"request": {"headers": [{"name": "X-Apidef-Policy", "value": "kept"}]}}}]}}, {"name": "createPet", "request": {"method": "POST", "path": "/pets"}}, {"name": "getPet", "request": {"method": "GET", "path": "/pets/{petId}"}}, {"name": "getInventory", "request": {"method": "GET", "path": "/store/inventory"}}]}
      """
    And the response status code should be 200
    And I deploy the "RestApi" "${CTX:api}" to the gateway via the control plane and store the deployment id as "firstDeployment"
    And I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/deployments/${CTX:firstDeployment}" until the JSON field "status" is "DEPLOYED"
    And I send a "GET" request to "${CTX:apiContext}/pets" until status 200
    And the JSON response field "headers.X-Apidef-Policy[0]" should be "kept"
    When I send a multipart "PUT" request to the control plane at "/rest-apis/${CTX:api}/openapi" with fields:
      | file | @spec:petstore-v2.yaml |
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}"
    Then the JSON response array field "operations" should have 4 items
    And the JSON response array "operations" item with "name" equal to "listPetPhotos" should have "request.path" equal to "/pets/{petId}/photos"
    And the JSON response array "operations" should not contain an item with "name" equal to "getInventory"
    And the JSON response array "operations" item with "name" equal to "listPets" should have "request.policies[0].name" equal to "set-headers"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/openapi"
    Then the JSON response field "content" should be spec fixture "petstore-v2.yaml"
    When I deploy the "RestApi" "${CTX:api}" to the gateway via the control plane and store the deployment id as "secondDeployment"
    And I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/deployments/${CTX:secondDeployment}" until the JSON field "status" is "DEPLOYED"
    Then I send a "GET" request to "${CTX:apiContext}/pets/42/photos" until status 200
    And the JSON response field "path" should be "/pets/42/photos"
    And I send a "GET" request to "${CTX:apiContext}/store/inventory" until status 404
    When I send a "GET" request to "${CTX:apiContext}/pets"
    Then the response status code should be 200
    And the JSON response field "headers.X-Apidef-Policy[0]" should be "kept"

  @APIDEF-005
  Scenario: A definition that renames an operation's path gives the renamed operation no policy, and it works without it
    Given I send a multipart "POST" request to the control plane at "/rest-apis/import-openapi" with fields:
      | id          | ${CTX:api}                                 |
      | displayName | ${CTX:api}                                 |
      | version     | v1.0                                       |
      | context     | ${CTX:apiContext}                          |
      | projectId   | ${CTX:project}                             |
      | upstream    | {"main": {"url": "http://testbench:3000"}} |
      | file        | @spec:petstore-v1.yaml                     |
    And the response status code should be 201
    And I send a "PUT" request to the control plane at "/rest-apis/${CTX:api}" with body:
      """
      {"displayName": "${CTX:api}", "context": "${CTX:apiContext}", "version": "v1.0", "projectId": "${CTX:project}", "upstream": {"main": {"url": "http://testbench:3000"}}, "operations": [{"name": "listPets", "request": {"method": "GET", "path": "/pets", "policies": [{"name": "set-headers", "version": "v1", "params": {"request": {"headers": [{"name": "X-Apidef-Policy", "value": "kept"}]}}}]}}, {"name": "createPet", "request": {"method": "POST", "path": "/pets"}}, {"name": "getPet", "request": {"method": "GET", "path": "/pets/{petId}"}}, {"name": "getInventory", "request": {"method": "GET", "path": "/store/inventory"}}]}
      """
    And the response status code should be 200
    And I deploy the "RestApi" "${CTX:api}" to the gateway via the control plane and store the deployment id as "firstDeployment"
    And I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/deployments/${CTX:firstDeployment}" until the JSON field "status" is "DEPLOYED"
    And I send a "GET" request to "${CTX:apiContext}/pets" until status 200
    And the JSON response field "headers.X-Apidef-Policy[0]" should be "kept"
    When I send a multipart "PUT" request to the control plane at "/rest-apis/${CTX:api}/openapi" with fields:
      | file | @spec:petstore-renamed.yaml |
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}"
    Then the JSON response array "operations" item with "name" equal to "listAnimals" should have "request.path" equal to "/animals"
    And the JSON response array "operations" item with "name" equal to "listAnimals" should not have field "request.policies[0]"
    And the JSON response array "operations" should not contain an item with "name" equal to "listPets"
    When I deploy the "RestApi" "${CTX:api}" to the gateway via the control plane and store the deployment id as "secondDeployment"
    And I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/deployments/${CTX:secondDeployment}" until the JSON field "status" is "DEPLOYED"
    Then I send a "GET" request to "${CTX:apiContext}/animals" until status 200
    And the JSON response field "path" should be "/animals"
    And the JSON response field "headers.X-Apidef-Policy" should not exist
