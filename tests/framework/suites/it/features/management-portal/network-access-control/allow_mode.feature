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

@NETAC
Feature: In allow mode the control plane reaches only the hosts the operator listed
  As a platform operator
  I want to list the hosts a tenant- or user-supplied URL may point at
  So that every other host is refused before the control plane sends anything to it

  # The block's platform-api runs network access control in allow mode with one entry,
  # 'testbench' (overlays/platform-api-access-control-allow.toml). An entry is a regular
  # expression matched case-insensitively against the whole hostname, so a host that merely
  # starts with it is not listed.
  Background:
    Given I authenticate using basic auth as "admin"
    And I generate a unique resource name from "netac-allow-proj" and store it as "project"
    And I create a project "${CTX:project}" on the control plane

  @NETAC-PA-006 @type:negative
  Scenario: Only a listed host can be imported from, matched against the whole hostname in any case
    Given I generate a unique resource name from "netac-allow-api" and store it as "api"
    And I generate a unique resource name from "netac-allow-upper" and store it as "upperApi"
    And I generate a unique resource name from "netac-allow-refused" and store it as "refusedApi"
    And I generate a unique API context from "/netac-allow" and store it as "apiContext"
    And I generate a unique API context from "/netac-allow-upper" and store it as "upperContext"
    And I generate a unique API context from "/netac-allow-refused" and store it as "refusedContext"
    When I send a multipart "POST" request to the control plane at "/rest-apis/import-openapi" with fields:
      | id          | ${CTX:refusedApi}                                       |
      | displayName | ${CTX:refusedApi}                                       |
      | version     | v1.0                                                    |
      | context     | ${CTX:refusedContext}                                   |
      | projectId   | ${CTX:project}                                          |
      | upstream    | {"main": {"url": "http://testbench:3000"}}              |
      | url         | http://specs.unlisted.example:3024/specs/petstore-v1.yaml |
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"
    And the response body should not contain "unlisted"
    When I send a multipart "POST" request to the control plane at "/rest-apis/import-openapi" with fields:
      | id          | ${CTX:refusedApi}                                           |
      | displayName | ${CTX:refusedApi}                                           |
      | version     | v1.0                                                        |
      | context     | ${CTX:refusedContext}                                       |
      | projectId   | ${CTX:project}                                              |
      | upstream    | {"main": {"url": "http://testbench:3000"}}                  |
      | url         | http://testbench.attacker.example:3024/specs/petstore-v1.yaml |
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:refusedApi}"
    Then the response status code should be 404
    When I send a multipart "POST" request to the control plane at "/rest-apis/import-openapi" with fields:
      | id          | ${CTX:api}                                       |
      | displayName | ${CTX:api}                                       |
      | version     | v1.0                                             |
      | context     | ${CTX:apiContext}                                |
      | projectId   | ${CTX:project}                                   |
      | upstream    | {"main": {"url": "http://testbench:3000"}}       |
      | url         | http://testbench:3024/specs/petstore-v1.yaml     |
    Then the response status code should be 201
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}"
    Then the response status code should be 200
    When I send a multipart "POST" request to the control plane at "/rest-apis/import-openapi" with fields:
      | id          | ${CTX:upperApi}                                  |
      | displayName | ${CTX:upperApi}                                  |
      | version     | v1.0                                             |
      | context     | ${CTX:upperContext}                              |
      | projectId   | ${CTX:project}                                   |
      | upstream    | {"main": {"url": "http://testbench:3000"}}       |
      | url         | http://TESTBENCH:3024/specs/petstore-v1.yaml     |
    Then the response status code should be 201

  @NETAC-PA-007 @type:negative
  Scenario: MCP server info is fetched only from a listed host
    When I send a "POST" request to the control plane at "/mcp-proxies/fetch-server-info" with body:
      """
      {"url": "http://mcp.unlisted.example:3009/mcp"}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "DESTINATION_NOT_ALLOWED"
    And the response body should not contain "unlisted"
    When I send a "POST" request to the control plane at "/mcp-proxies/fetch-server-info" with body:
      """
      {"url": "http://testbench:3009/mcp"}
      """
    Then the response status code should be 200
    And the JSON response should have field "serverInfo"
