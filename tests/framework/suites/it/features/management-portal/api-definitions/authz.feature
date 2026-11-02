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
Feature: Importing, validating and replacing an OpenAPI definition need their own scopes
  As a platform operator
  I want only callers who may create or update APIs to import, validate or replace a definition
  So that a reader can see a definition but never change one

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique resource name from "apidef-proj" and store it as "project"
    And I generate a unique resource name from "apidef-api" and store it as "api"
    And I generate a unique API context from "/apidef" and store it as "apiContext"
    And I create a project "${CTX:project}" on the control plane
    Given I send a multipart "POST" request to the control plane at "/rest-apis/import-openapi" with fields:
      | id          | ${CTX:api}                                 |
      | displayName | ${CTX:api}                                 |
      | version     | v1.0                                       |
      | context     | ${CTX:apiContext}                          |
      | projectId   | ${CTX:project}                             |
      | upstream    | {"main": {"url": "http://testbench:3000"}} |
      | file        | @spec:petstore-v1.yaml                     |
    And the response status code should be 201

  @APIDEF-014 @type:security
  Scenario Outline: A caller without a create or update scope cannot validate, import or replace a definition, and a reader can still read one
    When I send a multipart "POST" request to the control plane at "/rest-apis/validate-openapi" as "<actor>" with fields:
      | file | @spec:petstore-v1.yaml |
    Then the response status code should be 403
    And the JSON response field "code" should be "FORBIDDEN"
    Given I generate a unique resource name from "apidef-denied" and store it as "denied"
    When I send a multipart "POST" request to the control plane at "/rest-apis/import-openapi" as "<actor>" with fields:
      | id          | ${CTX:denied}                              |
      | displayName | ${CTX:denied}                              |
      | version     | v1.0                                       |
      | context     | /${CTX:denied}                             |
      | projectId   | ${CTX:project}                             |
      | upstream    | {"main": {"url": "http://testbench:3000"}} |
      | file        | @spec:petstore-v1.yaml                     |
    Then the response status code should be 403
    And the JSON response field "code" should be "FORBIDDEN"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:denied}"
    Then the response status code should be 404
    When I send a multipart "PUT" request to the control plane at "/rest-apis/${CTX:api}/openapi" as "<actor>" with fields:
      | file | @spec:petstore-v2.yaml |
    Then the response status code should be 403
    And the JSON response field "code" should be "FORBIDDEN"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/openapi" as "developer"
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}/openapi"
    Then the JSON response field "content" should be spec fixture "petstore-v1.yaml"
    # Positive control: a publisher may do all three.
    When I send a multipart "POST" request to the control plane at "/rest-apis/validate-openapi" as "publisher" with fields:
      | file | @spec:petstore-v1.yaml |
    Then the response status code should be 200
    When I send a multipart "PUT" request to the control plane at "/rest-apis/${CTX:api}/openapi" as "publisher" with fields:
      | file | @spec:petstore-v2.yaml |
    Then the response status code should be 200

    Examples:
      | grant                 | actor                  |
      | only ap:rest_api:read | developer              |
      | no scope              | a caller with no scope |
