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
Feature: A REST API is created from an OpenAPI definition, by file or by URL
  As an API publisher
  I want to create an API from the OpenAPI definition I already have
  So that its operations come from the definition instead of being entered again

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique resource name from "apidef-proj" and store it as "project"
    And I generate a unique resource name from "apidef-api" and store it as "api"
    And I generate a unique API context from "/apidef" and store it as "apiContext"
    And I create a project "${CTX:project}" on the control plane

  @APIDEF-001
  Scenario: An API imported from an OpenAPI file has the given details, the definition's operations and the stored definition, and serves them
    When I send a multipart "POST" request to the control plane at "/rest-apis/import-openapi" with fields:
      | id          | ${CTX:api}                                 |
      | displayName | ${CTX:api}                                 |
      | version     | v1.0                                       |
      | context     | ${CTX:apiContext}                          |
      | description | Pets imported from a file                  |
      | projectId   | ${CTX:project}                             |
      | upstream    | {"main": {"url": "http://testbench:3000"}} |
      | file        | @spec:petstore-v1.yaml                     |
    Then the response status code should be 201
    And the JSON response field "id" should be "${CTX:api}"
    And the JSON response field "displayName" should be "${CTX:api}"
    And the JSON response field "version" should be "v1.0"
    And the JSON response field "context" should be "${CTX:apiContext}"
    And the JSON response field "description" should be "Pets imported from a file"
    And the JSON response field "projectId" should be "${CTX:project}"
    And the JSON response field "upstream.main.url" should be "http://testbench:3000"
    And the JSON response field "lifeCycleStatus" should be "CREATED"
    And the JSON response array field "operations" should have 4 items
    And the JSON response array "operations" item with "name" equal to "listPets" should have "request.path" equal to "/pets"
    And the JSON response array "operations" item with "name" equal to "createPet" should have "request.method" equal to "POST"
    And the JSON response array "operations" item with "name" equal to "getPet" should have "request.path" equal to "/pets/{petId}"
    And the JSON response array "operations" item with "name" equal to "getInventory" should have "request.path" equal to "/store/inventory"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}"
    Then the response status code should be 200
    And the JSON response field "projectId" should be "${CTX:project}"
    And the JSON response field "lifeCycleStatus" should be "CREATED"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/openapi"
    Then the response status code should be 200
    And the JSON response field "content" should be spec fixture "petstore-v1.yaml"
    When I deploy the "RestApi" "${CTX:api}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    Then I send a "GET" request to "${CTX:apiContext}/pets/42" until status 200
    And the JSON response field "path" should be "/pets/42"

  @APIDEF-002
  Scenario: An API imported from an OpenAPI URL over HTTPS has the given details, the definition's operations and the stored definition, and serves them
    Given I store the https URL of spec fixture "petstore-v1.yaml" as "specUrl"
    When I send a multipart "POST" request to the control plane at "/rest-apis/import-openapi" with fields:
      | id          | ${CTX:api}                                 |
      | displayName | ${CTX:api}                                 |
      | version     | v1.0                                       |
      | context     | ${CTX:apiContext}                          |
      | description | Pets imported from a URL                   |
      | projectId   | ${CTX:project}                             |
      | upstream    | {"main": {"url": "http://testbench:3000"}} |
      | url         | ${CTX:specUrl}                             |
    Then the response status code should be 201
    And the JSON response field "id" should be "${CTX:api}"
    And the JSON response field "displayName" should be "${CTX:api}"
    And the JSON response field "version" should be "v1.0"
    And the JSON response field "context" should be "${CTX:apiContext}"
    And the JSON response field "description" should be "Pets imported from a URL"
    And the JSON response field "projectId" should be "${CTX:project}"
    And the JSON response field "upstream.main.url" should be "http://testbench:3000"
    And the JSON response field "lifeCycleStatus" should be "CREATED"
    And the JSON response array field "operations" should have 4 items
    And the JSON response array "operations" item with "name" equal to "listPets" should have "request.path" equal to "/pets"
    And the JSON response array "operations" item with "name" equal to "createPet" should have "request.method" equal to "POST"
    And the JSON response array "operations" item with "name" equal to "getPet" should have "request.path" equal to "/pets/{petId}"
    And the JSON response array "operations" item with "name" equal to "getInventory" should have "request.path" equal to "/store/inventory"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/openapi"
    Then the response status code should be 200
    And the JSON response field "content" should be spec fixture "petstore-v1.yaml"
    When I deploy the "RestApi" "${CTX:api}" to the gateway via the control plane and store the deployment id as "deploymentId"
    And I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/deployments/${CTX:deploymentId}" until the JSON field "status" is "DEPLOYED"
    Then I send a "GET" request to "${CTX:apiContext}/pets/42" until status 200
    And the JSON response field "path" should be "/pets/42"

  @APIDEF-003
  Scenario: A second version imported under the same display name keeps each version's own operations and definition
    Given I generate a unique resource name from "apidef-api-v2" and store it as "api2"
    And I generate a unique API context from "/apidef-v2" and store it as "apiContext2"
    When I send a multipart "POST" request to the control plane at "/rest-apis/import-openapi" with fields:
      | id          | ${CTX:api}                                 |
      | displayName | ${CTX:api}                                 |
      | version     | v1.0                                       |
      | context     | ${CTX:apiContext}                          |
      | description | Version one                                |
      | projectId   | ${CTX:project}                             |
      | upstream    | {"main": {"url": "http://testbench:3000"}} |
      | file        | @spec:petstore-v1.yaml                     |
    Then the response status code should be 201
    When I send a multipart "POST" request to the control plane at "/rest-apis/import-openapi" with fields:
      | id          | ${CTX:api2}                                |
      | displayName | ${CTX:api}                                 |
      | version     | v2.0                                       |
      | context     | ${CTX:apiContext2}                         |
      | description | Version two                                |
      | projectId   | ${CTX:project}                             |
      | upstream    | {"main": {"url": "http://testbench:3000"}} |
      | file        | @spec:inventory.yaml                       |
    Then the response status code should be 201
    When I send a "GET" request to the control plane at "/rest-apis?projectId=${CTX:project}"
    Then the response status code should be 200
    And the JSON response array "list" item with "id" equal to "${CTX:api}" should have "displayName" equal to "${CTX:api}"
    And the JSON response array "list" item with "id" equal to "${CTX:api}" should have "version" equal to "v1.0"
    And the JSON response array "list" item with "id" equal to "${CTX:api2}" should have "displayName" equal to "${CTX:api}"
    And the JSON response array "list" item with "id" equal to "${CTX:api2}" should have "version" equal to "v2.0"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}"
    Then the response status code should be 200
    And the JSON response field "version" should be "v1.0"
    And the JSON response field "description" should be "Version one"
    And the JSON response array field "operations" should have 4 items
    And the JSON response array "operations" item with "name" equal to "listPets" should have "request.path" equal to "/pets"
    And the JSON response array "operations" item with "name" equal to "createPet" should have "request.method" equal to "POST"
    And the JSON response array "operations" item with "name" equal to "getPet" should have "request.path" equal to "/pets/{petId}"
    And the JSON response array "operations" item with "name" equal to "getInventory" should have "request.path" equal to "/store/inventory"
    And the JSON response array "operations" should not contain an item with "name" equal to "listItems"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api2}"
    Then the response status code should be 200
    And the JSON response array field "operations" should have 2 items
    And the JSON response array "operations" item with "name" equal to "listItems" should have "request.path" equal to "/items"
    And the JSON response array "operations" should not contain an item with "name" equal to "listPets"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/openapi"
    Then the JSON response field "content" should be spec fixture "petstore-v1.yaml"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api2}/openapi"
    Then the JSON response field "content" should be spec fixture "inventory.yaml"

  @APIDEF-007
  Scenario: Deleting an imported API removes its definition, and the same id imports again with a new definition
    When I send a multipart "POST" request to the control plane at "/rest-apis/import-openapi" with fields:
      | id          | ${CTX:api}                                 |
      | displayName | ${CTX:api}                                 |
      | version     | v1.0                                       |
      | context     | ${CTX:apiContext}                          |
      | description | Before the delete                          |
      | projectId   | ${CTX:project}                             |
      | upstream    | {"main": {"url": "http://testbench:3000"}} |
      | file        | @spec:petstore-v1.yaml                     |
    Then the response status code should be 201
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/openapi"
    Then the response status code should be 200
    When I send a "DELETE" request to the control plane at "/rest-apis/${CTX:api}"
    Then the response status code should be 204
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/openapi"
    Then the response status code should be 404
    When I send a multipart "POST" request to the control plane at "/rest-apis/import-openapi" with fields:
      | id          | ${CTX:api}                                 |
      | displayName | ${CTX:api}                                 |
      | version     | v1.0                                       |
      | context     | ${CTX:apiContext}                          |
      | description | After the delete                           |
      | projectId   | ${CTX:project}                             |
      | upstream    | {"main": {"url": "http://testbench:3000"}} |
      | file        | @spec:inventory.yaml                       |
    Then the response status code should be 201
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/openapi"
    Then the response status code should be 200
    And the JSON response field "content" should be spec fixture "inventory.yaml"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}"
    Then the JSON response array field "operations" should have 2 items
    And the JSON response field "description" should be "After the delete"

  @APIDEF-008 @type:negative
  Scenario: Importing with both a file and a URL, or with neither, is refused and creates nothing
    Given I store the https URL of spec fixture "petstore-v1.yaml" as "specUrl"
    When I send a multipart "POST" request to the control plane at "/rest-apis/import-openapi" with fields:
      | id          | ${CTX:api}                                 |
      | displayName | ${CTX:api}                                 |
      | version     | v1.0                                       |
      | context     | ${CTX:apiContext}                          |
      | description | Both                                       |
      | projectId   | ${CTX:project}                             |
      | upstream    | {"main": {"url": "http://testbench:3000"}} |
      | file        | @spec:petstore-v1.yaml                     |
      | url         | ${CTX:specUrl}                             |
    Then the response status code should be 400
    And the JSON response field "message" should contain "either"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}"
    Then the response status code should be 404
    When I send a multipart "POST" request to the control plane at "/rest-apis/import-openapi" with fields:
      | id          | ${CTX:api}                                 |
      | displayName | ${CTX:api}                                 |
      | version     | v1.0                                       |
      | context     | ${CTX:apiContext}                          |
      | description | Neither                                    |
      | projectId   | ${CTX:project}                             |
      | upstream    | {"main": {"url": "http://testbench:3000"}} |
    Then the response status code should be 400
    And the JSON response field "message" should contain "either"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}"
    Then the response status code should be 404

  @APIDEF-009 @type:negative
  Scenario Outline: Importing a file that is not a usable OpenAPI 3.x definition is refused with its reason and creates nothing
    When I send a multipart "POST" request to the control plane at "/rest-apis/import-openapi" with fields:
      | id          | ${CTX:api}                                 |
      | displayName | ${CTX:api}                                 |
      | version     | v1.0                                       |
      | context     | ${CTX:apiContext}                          |
      | description | Unusable                                   |
      | projectId   | ${CTX:project}                             |
      | upstream    | {"main": {"url": "http://testbench:3000"}} |
      | file        | @spec:<fixture>                            |
    Then the response status code should be 400
    And the JSON response field "errors" should contain "<reason>"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}"
    Then the response status code should be 404

    Examples:
      | fixture          | reason                     |
      | empty-paths.yaml | at least one operation     |
      | invalid.yaml     | get.responses              |
      | swagger2.yaml    | Swagger 2.x is not allowed |
