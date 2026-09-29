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

@api-key-auth
Feature: API key authentication policy
  As an API developer
  I want the api-key-auth policy to authenticate every request with a valid, API-scoped key
  So that unauthenticated or mis-authenticated traffic never reaches the backend

  # "first gateway" (rather than "the gateway services are running") keeps this Background valid
  # in the multigateway block, where an un-ordinaled platform-gateway lookup is rejected because
  # that block runs two full gateway replicas.
  Background:
    Given the first gateway is running
    And I authenticate using basic auth as "admin"
    And I resolve the "capture" service URL at "" and store it as "captureUpstream"

  # Group 01 - Core accept/reject
  @aka-g01 @aka-01
  Scenario: A request with a valid API key is authenticated and forwarded to the backend
    Given I generate a unique value from "aka-s01" and store it as "akaApi"
    And I generate a unique API context from "/aka-s01" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s01/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s01/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s01/probe" until status 200
    When I send a "GET" request to "${CTX:akaCtx}/s01/probe"
    Then the response status code should be 200
    And the JSON response field "path" should be "/s01/probe"
    And the response should not contain echoed header "API-Key"
    When I send a "GET" request to the "capture" service at "/test/captured?path=/s01/probe"
    Then the response status should be 200
    And the response body should contain "/s01/probe"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s01/probe" until status 404

  @aka-g01 @aka-02
  Scenario: A request without an API key is rejected and never reaches the backend
    Given I generate a unique value from "aka-s02" and store it as "akaApi"
    And I generate a unique API context from "/aka-s02" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s02/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s02/probe" until status 401
    When I send a "GET" request to "${CTX:akaCtx}/s02/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I send a "GET" request to the "capture" service at "/test/captured?path=/s02/probe"
    Then the response status should be 204
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s02/probe" until status 200
    When I send a "GET" request to the "capture" service at "/test/captured?path=/s02/probe"
    Then the response status should be 200
    And the response body should contain "/s02/probe"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s02/probe" until status 404

  @aka-g01 @aka-03
  Scenario: A request with an invalid or random API key is rejected and never reaches the backend
    Given I generate a unique value from "aka-s03" and store it as "akaApi"
    And I generate a unique API context from "/aka-s03" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s03/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s03/probe" until status 401
    When I set header "API-Key" to "apip_0000000000000000000000000000000000000000000000000000000000000000"
    And I send a "GET" request to "${CTX:akaCtx}/s03/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I set header "API-Key" to "not-a-real-key-${CTX:akaApi}"
    And I send a "GET" request to "${CTX:akaCtx}/s03/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I send a "GET" request to the "capture" service at "/test/captured?path=/s03/probe"
    Then the response status should be 204
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s03/probe" until status 200
    When I send a "GET" request to the "capture" service at "/test/captured?path=/s03/probe"
    Then the response status should be 200
    And the response body should contain "/s03/probe"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s03/probe" until status 404

  @aka-g01 @aka-04
  Scenario: A request with an empty API key value is rejected
    Given I generate a unique value from "aka-s04" and store it as "akaApi"
    And I generate a unique API context from "/aka-s04" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s04/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s04/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    When I set header "API-Key" to ""
    And I send a "GET" request to "${CTX:akaCtx}/s04/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s04/probe" until status 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s04/probe" until status 404

  # A raw request is required here: Go's http.Client trims leading/trailing OWS from a header
  # value before it ever reaches the wire, so "I set header" would silently deliver an empty
  # value and this scenario would degenerate into a duplicate of S04.
  @aka-g01 @aka-05
  Scenario: A request with a whitespace-only API key value is rejected
    Given I generate a unique value from "aka-s05" and store it as "akaApi"
    And I generate a unique API context from "/aka-s05" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s05/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s05/probe" until status 401
    When I send a raw "GET" request to "${CTX:akaCtx}/s05/probe" with headers:
      | API-Key | \x20\x20\x20 |
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I send a raw "GET" request to "${CTX:akaCtx}/s05/probe" with headers:
      | API-Key | \t\t |
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s05/probe" until status 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s05/probe" until status 404
