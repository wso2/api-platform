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
Feature: An OpenAPI definition import is bounded in size and never fetches from a forbidden address
  As a platform operator
  I want definitions over the configured size and URLs aimed at internal addresses refused
  So that an import can neither exhaust the control plane nor reach what it must not

  # The block's platform-api allows 65536-byte definitions (overlays/platform-api-openapi-import.toml).
  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique resource name from "apidef-proj" and store it as "project"
    And I generate a unique resource name from "apidef-api" and store it as "api"
    And I generate a unique API context from "/apidef" and store it as "apiContext"
    And I create a project "${CTX:project}" on the control plane

  @APIDEF-012 @type:boundary
  Scenario: A definition just over the configured size is refused as too large, as a file and from a URL, and creates nothing
    Given I store the https URL of spec fixture "large?bytes=65537" as "largeUrl"
    When I send a multipart "POST" request to the control plane at "/rest-apis/import-openapi" with fields:
      | id          | ${CTX:api}                                 |
      | displayName | ${CTX:api}                                 |
      | version     | v1.0                                       |
      | context     | ${CTX:apiContext}                          |
      | projectId   | ${CTX:project}                             |
      | upstream    | {"main": {"url": "http://testbench:3000"}} |
      | file        | @generated-spec:65537                      |
    Then the response status code should be 413
    And the JSON response field "code" should be "PAYLOAD_TOO_LARGE"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}"
    Then the response status code should be 404
    When I send a multipart "POST" request to the control plane at "/rest-apis/import-openapi" with fields:
      | id          | ${CTX:api}                                 |
      | displayName | ${CTX:api}                                 |
      | version     | v1.0                                       |
      | context     | ${CTX:apiContext}                          |
      | projectId   | ${CTX:project}                             |
      | upstream    | {"main": {"url": "http://testbench:3000"}} |
      | url         | ${CTX:largeUrl}                            |
    Then the response status code should be 413
    And the JSON response field "code" should be "PAYLOAD_TOO_LARGE"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}"
    Then the response status code should be 404
    When I send a multipart "POST" request to the control plane at "/rest-apis/import-openapi" with fields:
      | id          | ${CTX:api}                                 |
      | displayName | ${CTX:api}                                 |
      | version     | v1.0                                       |
      | context     | ${CTX:apiContext}                          |
      | projectId   | ${CTX:project}                             |
      | upstream    | {"main": {"url": "http://testbench:3000"}} |
      | file        | @generated-spec:65536                      |
    Then the response status code should be 201

  @APIDEF-013 @type:negative
  Scenario: Importing from the cloud metadata address or a file URL is refused with a generic error and creates nothing
    When I send a multipart "POST" request to the control plane at "/rest-apis/import-openapi" with fields:
      | id          | ${CTX:api}                                 |
      | displayName | ${CTX:api}                                 |
      | version     | v1.0                                       |
      | context     | ${CTX:apiContext}                          |
      | projectId   | ${CTX:project}                             |
      | upstream    | {"main": {"url": "http://testbench:3000"}} |
      | url         | http://169.254.169.254/latest/meta-data/   |
    Then the response status code should be 400
    And the JSON response field "message" should be "failed to fetch OpenAPI spec from the provided URL"
    And the response body should not contain "169.254"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}"
    Then the response status code should be 404
    When I send a multipart "POST" request to the control plane at "/rest-apis/import-openapi" with fields:
      | id          | ${CTX:api}                                 |
      | displayName | ${CTX:api}                                 |
      | version     | v1.0                                       |
      | context     | ${CTX:apiContext}                          |
      | projectId   | ${CTX:project}                             |
      | upstream    | {"main": {"url": "http://testbench:3000"}} |
      | url         | file:///etc/passwd                         |
    Then the response status code should be 400
    And the JSON response field "message" should be "failed to fetch OpenAPI spec from the provided URL"
    And the response body should not contain "/etc/passwd"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}"
    Then the response status code should be 404
