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
    When I send a "GET" request to the "capture" service at "/test/captured?path=/s01/probe" until status 200
    Then the response body should contain "/s01/probe"
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
    When I send a "GET" request to the "capture" service at "/test/captured?path=/s02/probe" until status 204
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s02/probe" until status 200
    When I send a "GET" request to the "capture" service at "/test/captured?path=/s02/probe" until status 200
    Then the response body should contain "/s02/probe"
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
    When I send a "GET" request to the "capture" service at "/test/captured?path=/s03/probe" until status 204
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s03/probe" until status 200
    When I send a "GET" request to the "capture" service at "/test/captured?path=/s03/probe" until status 200
    Then the response body should contain "/s03/probe"
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

  # Group 02 - Header / location config
  # Go's HTTP client canonicalizes a header name before it reaches the wire, so a raw request is
  # required to prove the policy itself matches the configured header case-insensitively.
  @aka-g02 @aka-06
  Scenario: The configured API key header name is matched case-insensitively
    Given I generate a unique value from "aka-s06" and store it as "akaApi"
    And I generate a unique API context from "/aka-s06" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s06/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"X-API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s06/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "X-API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s06/probe" until status 200
    When I send a raw "GET" request to "${CTX:akaCtx}/s06/probe" with headers:
      | x-api-key | ${CTX:akaKey} |
    Then the response status code should be 200
    When I send a raw "GET" request to "${CTX:akaCtx}/s06/probe" with headers:
      | X-API-KEY | ${CTX:akaKey} |
    Then the response status code should be 200
    When I send a raw "GET" request to "${CTX:akaCtx}/s06/probe" with headers:
      | x-ApI-kEy | ${CTX:akaKey} |
    Then the response status code should be 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s06/probe" until status 404

  @aka-g02 @aka-07
  Scenario: The API key value is matched case-sensitively
    Given I generate a unique value from "aka-s07" and store it as "akaApi"
    And I generate a unique API context from "/aka-s07" and store it as "akaCtx"
    And I generate a unique resource name from "aka-s07" and store it as "caseSeed"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s07/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s07/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"case-key","apiKey":"${CTX:caseSeed}-MixedCaseKey-ABCdefGHIjklMNOpqrSTUvwx"}
      """
    Then the response status should be 201
    And I set header "API-Key" to "${CTX:caseSeed}-MixedCaseKey-ABCdefGHIjklMNOpqrSTUvwx"
    And I send a "GET" request to "${CTX:akaCtx}/s07/probe" until status 200
    When I set header "API-Key" to "${CTX:caseSeed}-mixedcasekey-abcdefghijklmnopqrstuvwx"
    And I send a "GET" request to "${CTX:akaCtx}/s07/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I set header "API-Key" to "${CTX:caseSeed}-MIXEDCASEKEY-ABCDEFGHIJKLMNOPQRSTUVWX"
    And I send a "GET" request to "${CTX:akaCtx}/s07/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s07/probe" until status 404

  @aka-g02 @aka-08
  Scenario: A key presented in an unconfigured location is rejected
    Given I generate a unique value from "aka-s08" and store it as "akaApi"
    And I generate a unique API context from "/aka-s08" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s08/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s08/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s08/probe" until status 200
    When I reset the request
    And I send a "GET" request to "${CTX:akaCtx}/s08/probe?API-Key=${CTX:akaKey}"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I set header "Cookie" to "API-Key=${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s08/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I reset the request
    And I set header "Authorization" to "Bearer ${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s08/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I reset the request
    And I set header "Authorization" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s08/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I reset the request
    And I set header "X-API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s08/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s08/probe"
    Then the response status code should be 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s08/probe" until status 404

  @aka-g02 @aka-09
  Scenario: A custom configured header name authenticates the request
    Given I generate a unique value from "aka-s09" and store it as "akaApi"
    And I generate a unique API context from "/aka-s09" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s09/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"X-Custom-Auth","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s09/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "X-Custom-Auth" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s09/probe" until status 200
    When I send a "GET" request to "${CTX:akaCtx}/s09/probe"
    Then the response status code should be 200
    And the response should not contain echoed header "X-Custom-Auth"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s09/probe" until status 404

  @aka-g02 @aka-10
  Scenario: A key sent under the default header names is rejected when a custom header is configured
    Given I generate a unique value from "aka-s10" and store it as "akaApi"
    And I generate a unique API context from "/aka-s10" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s10/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"X-Custom-Auth","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s10/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "X-Custom-Auth" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s10/probe" until status 200
    When I reset the request
    And I set header "X-API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s10/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I reset the request
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s10/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I reset the request
    And I set header "X-Custom-Auth" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s10/probe"
    Then the response status code should be 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s10/probe" until status 404

  # Group 03 - Enforcement scope
  @aka-g03 @aka-11
  Scenario: An API-level policy protects every operation
    Given I generate a unique value from "aka-s11" and store it as "akaApi"
    And I generate a unique API context from "/aka-s11" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.policies          | [{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}] |
      | spec.operations        | [{"method":"GET","path":"/s11/a"},{"method":"POST","path":"/s11/b"},{"method":"PUT","path":"/s11/c/{id}"},{"method":"DELETE","path":"/s11/c/{id}"},{"method":"PATCH","path":"/s11/c/{id}"}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s11/a" until status 401
    When I send a "GET" request to "${CTX:akaCtx}/s11/a"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I send a "POST" request to "${CTX:akaCtx}/s11/b"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I send a "PUT" request to "${CTX:akaCtx}/s11/c/1"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I send a "DELETE" request to "${CTX:akaCtx}/s11/c/1"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I send a "PATCH" request to "${CTX:akaCtx}/s11/c/1"
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
    And I send a "GET" request to "${CTX:akaCtx}/s11/a" until status 200
    When I send a "POST" request to "${CTX:akaCtx}/s11/b"
    Then the response status code should be 200
    When I send a "PUT" request to "${CTX:akaCtx}/s11/c/1"
    Then the response status code should be 200
    When I send a "DELETE" request to "${CTX:akaCtx}/s11/c/1"
    Then the response status code should be 200
    When I send a "PATCH" request to "${CTX:akaCtx}/s11/c/1"
    Then the response status code should be 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s11/a" until status 404

  @aka-g03 @aka-12
  Scenario: An operation-level policy protects only that operation
    Given I generate a unique value from "aka-s12" and store it as "akaApi"
    And I generate a unique API context from "/aka-s12" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s12/secure","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s12/open"}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s12/secure" until status 401
    When I send a "GET" request to "${CTX:akaCtx}/s12/open"
    Then the response status code should be 200
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s12/secure" until status 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s12/secure" until status 404

  @aka-g03 @aka-13
  Scenario: Enforcement follows the configured HTTP method
    Given I generate a unique value from "aka-s13" and store it as "akaApi"
    And I generate a unique API context from "/aka-s13" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s13/res","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"POST","path":"/s13/res"},{"method":"DELETE","path":"/s13/res","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s13/res" until status 401
    When I send a "DELETE" request to "${CTX:akaCtx}/s13/res"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I send a "POST" request to "${CTX:akaCtx}/s13/res"
    Then the response status code should be 200
    When I send a "PUT" request to "${CTX:akaCtx}/s13/res"
    Then the response status code should be 404
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s13/res" until status 200
    When I send a "DELETE" request to "${CTX:akaCtx}/s13/res"
    Then the response status code should be 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s13/res" until status 404

  @aka-g03 @aka-14
  Scenario: Path, query, and trailing-slash variations cannot bypass authentication
    Given I generate a unique value from "aka-s14" and store it as "akaApi"
    And I generate a unique API context from "/aka-s14" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s14/items/{id}","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s14/items/42" until status 401
    When I send a "GET" request to "${CTX:akaCtx}/s14/items/42"
    Then the response should be a client error
    When I send a "GET" request to "${CTX:akaCtx}/s14/items/42?foo=bar"
    Then the response should be a client error
    When I send a "GET" request to "${CTX:akaCtx}/s14/items/42?API-Key=x"
    Then the response should be a client error
    When I send a "GET" request to "${CTX:akaCtx}/s14/items/42/"
    Then the response should be a client error
    When I send a "GET" request to "${CTX:akaCtx}/s14//items/42"
    Then the response should be a client error
    When I send a "GET" request to "${CTX:akaCtx}/s14/items/./42"
    Then the response should be a client error
    When I send a "GET" request to "${CTX:akaCtx}/s14/items/%34%32"
    Then the response should be a client error
    When I send a "GET" request to "${CTX:akaCtx}/s14/ITEMS/42"
    Then the response should be a client error
    When I send a "GET" request to "${CTX:akaCtx}/s14/items/42;jsessionid=1"
    Then the response should be a client error
    When I send a "GET" request to the "capture" service at "/test/captured?path=/s14/items/42" until status 204
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s14/items/42" until status 200
    When I send a "GET" request to "${CTX:akaCtx}/s14/items/42?foo=bar"
    Then the response status code should be 200
    And the JSON response field "query" should be "foo=bar"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s14/items/42" until status 404

  # Group 04 - Key-API binding
  @aka-g04 @aka-15
  Scenario: A key issued for one API authenticates requests to that same API
    Given I generate a unique value from "aka-s15" and store it as "akaApi"
    And I generate a unique API context from "/aka-s15" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s15/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s15/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s15/probe" until status 200
    When I send a "GET" request to "${CTX:akaCtx}/s15/probe"
    Then the response status code should be 200
    And the JSON response field "path" should be "/s15/probe"
    And the response should not contain echoed header "API-Key"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s15/probe" until status 404

  @aka-g04 @aka-16
  Scenario: A key issued for one API is rejected by another API
    Given I generate a unique value from "aka-s16-a" and store it as "akaApiA"
    And I generate a unique API context from "/aka-s16-a" and store it as "akaCtxA"
    And I generate a unique value from "aka-s16-b" and store it as "akaApiB"
    And I generate a unique API context from "/aka-s16-b" and store it as "akaCtxB"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApiA}            |
      | spec.displayName       | ${CTX:akaApiA}            |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtxA}            |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s16/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApiB}            |
      | spec.displayName       | ${CTX:akaApiB}            |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtxB}            |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s16/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtxA}/s16/probe" until status 401
    And I send a "GET" request to "${CTX:akaCtxB}/s16/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApiA}/api-keys" with body:
      """
      {"name":"key-a"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "keyA"
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApiB}/api-keys" with body:
      """
      {"name":"key-b"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "keyB"
    And I set header "API-Key" to "${CTX:keyA}"
    And I send a "GET" request to "${CTX:akaCtxA}/s16/probe" until status 200
    When I set header "API-Key" to "${CTX:keyB}"
    And I send a "GET" request to "${CTX:akaCtxB}/s16/probe" until status 200
    When I set header "API-Key" to "${CTX:keyA}"
    And I send a "GET" request to "${CTX:akaCtxB}/s16/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I set header "API-Key" to "${CTX:keyB}"
    And I send a "GET" request to "${CTX:akaCtxA}/s16/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I send a "GET" request to "${CTX:akaCtxB}/s16/probe"
    Then the response status code should be 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApiA}"
    Then the response should be successful
    And I delete the API "${CTX:akaApiB}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtxA}/s16/probe" until status 404
    And I send a "GET" request to "${CTX:akaCtxB}/s16/probe" until status 404

  @aka-g04 @aka-17
  Scenario: Multiple active keys on one API all work, and revoking one leaves the others valid
    Given I generate a unique value from "aka-s17" and store it as "akaApi"
    And I generate a unique API context from "/aka-s17" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s17/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s17/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "k1"
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-two"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "k2"
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-three"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "k3"
    And I set header "API-Key" to "${CTX:k1}"
    And I send a "GET" request to "${CTX:akaCtx}/s17/probe" until status 200
    When I set header "API-Key" to "${CTX:k2}"
    And I send a "GET" request to "${CTX:akaCtx}/s17/probe" until status 200
    When I set header "API-Key" to "${CTX:k3}"
    And I send a "GET" request to "${CTX:akaCtx}/s17/probe" until status 200
    When I reset the request
    And I send a "DELETE" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys/key-two"
    Then the response status should be 200
    When I set header "API-Key" to "${CTX:k2}"
    And I send a "GET" request to "${CTX:akaCtx}/s17/probe" until status 401
    When I send a "GET" request to "${CTX:akaCtx}/s17/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I set header "API-Key" to "${CTX:k1}"
    And I send a "GET" request to "${CTX:akaCtx}/s17/probe"
    Then the response status code should be 200
    When I set header "API-Key" to "${CTX:k3}"
    And I send a "GET" request to "${CTX:akaCtx}/s17/probe"
    Then the response status code should be 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s17/probe" until status 404

  # Group 05 - Lifecycle to runtime effect
  @aka-g05 @aka-18
  Scenario: A revoked key is rejected once the revocation has propagated
    Given I generate a unique value from "aka-s18" and store it as "akaApi"
    And I generate a unique API context from "/aka-s18" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s18/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s18/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "k1"
    And I set header "API-Key" to "${CTX:k1}"
    And I send a "GET" request to "${CTX:akaCtx}/s18/probe" until status 200
    When I reset the request
    And I send a "DELETE" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys/key-one"
    Then the response status should be 200
    And the JSON response field "status" should be "success"
    When I set header "API-Key" to "${CTX:k1}"
    And I send a "GET" request to "${CTX:akaCtx}/s18/probe" until status 401
    When I send a "GET" request to "${CTX:akaCtx}/s18/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s18/probe" until status 404

  @aka-g05 @aka-19
  Scenario: Regenerating or rotating a key invalidates the old value
    Given I generate a unique value from "aka-s19" and store it as "akaApi"
    And I generate a unique API context from "/aka-s19" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s19/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s19/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "oldKey"
    And I set header "API-Key" to "${CTX:oldKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s19/probe" until status 200
    When I reset the request
    And I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys/key-one/regenerate" with body:
      """
      {}
      """
    Then the response status should be 200
    And the JSON response field "apiKey.apiKey" should not equal "${CTX:oldKey}"
    And I store the JSON response field "apiKey.apiKey" as "newKey"
    And I set header "API-Key" to "${CTX:newKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s19/probe" until status 200
    When I send a "GET" request to "${CTX:akaCtx}/s19/probe"
    Then the response status code should be 200
    When I set header "API-Key" to "${CTX:oldKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s19/probe" until status 401
    When I send a "GET" request to "${CTX:akaCtx}/s19/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I reset the request
    And I generate a unique resource name from "aka-s19" and store it as "externalSeed"
    And I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"external-key","apiKey":"${CTX:externalSeed}-original-custom-value-0123456789abcdef"}
      """
    Then the response status should be 201
    And I set header "API-Key" to "${CTX:externalSeed}-original-custom-value-0123456789abcdef"
    And I send a "GET" request to "${CTX:akaCtx}/s19/probe" until status 200
    When I reset the request
    And I send a "PUT" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys/external-key" with body:
      """
      {"apiKey":"${CTX:externalSeed}-rotated-custom-value-0123456789abcdef"}
      """
    Then the response status should be 200
    And I set header "API-Key" to "${CTX:externalSeed}-rotated-custom-value-0123456789abcdef"
    And I send a "GET" request to "${CTX:akaCtx}/s19/probe" until status 200
    When I set header "API-Key" to "${CTX:externalSeed}-original-custom-value-0123456789abcdef"
    And I send a "GET" request to "${CTX:akaCtx}/s19/probe" until status 401
    When I send a "GET" request to "${CTX:akaCtx}/s19/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s19/probe" until status 404

  @aka-g05 @aka-20
  Scenario: A deleted key stays rejected even after its name is reused
    Given I generate a unique value from "aka-s20" and store it as "akaApi"
    And I generate a unique API context from "/aka-s20" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s20/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s20/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "firstKey"
    And I set header "API-Key" to "${CTX:firstKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s20/probe" until status 200
    When I reset the request
    And I send a "DELETE" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys/key-one"
    Then the response status should be 200
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys"
    Then the response body should not contain "key-one"
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And the JSON response field "apiKey.apiKey" should not equal "${CTX:firstKey}"
    And I store the JSON response field "apiKey.apiKey" as "secondKey"
    And I set header "API-Key" to "${CTX:secondKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s20/probe" until status 200
    When I set header "API-Key" to "${CTX:firstKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s20/probe" until status 401
    When I send a "GET" request to "${CTX:akaCtx}/s20/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s20/probe" until status 404

  # Group 06 - Expiry
  @aka-g06 @aka-21
  Scenario: A key used before its expiry works
    Given I generate a unique value from "aka-s21" and store it as "akaApi"
    And I generate a unique API context from "/aka-s21" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s21/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s21/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"hour-key","expiresIn":{"unit":"hours","duration":1}}
      """
    Then the response status should be 201
    And the JSON response field "apiKey.expiresAt" should not equal "null"
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s21/probe" until status 200
    When I send a "GET" request to "${CTX:akaCtx}/s21/probe"
    Then the response status code should be 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s21/probe" until status 404

  @aka-g06 @aka-22
  Scenario: A key is rejected once it has expired
    Given I generate a unique value from "aka-s22" and store it as "akaApi"
    And I generate a unique API context from "/aka-s22" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s22/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s22/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"short-key","expiresIn":{"unit":"seconds","duration":20}}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s22/probe" until status 200
    When I send a "GET" request to "${CTX:akaCtx}/s22/probe"
    Then the response status code should be 200
    And I send a "GET" request to "${CTX:akaCtx}/s22/probe" until status 401
    When I send a "GET" request to "${CTX:akaCtx}/s22/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s22/probe" until status 404

  @aka-g06 @aka-23
  Scenario: A key without an expiry stays valid until it is revoked
    Given I generate a unique value from "aka-s23" and store it as "akaApi"
    And I generate a unique API context from "/aka-s23" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s23/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s23/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"forever-key"}
      """
    Then the response status should be 201
    And the JSON response field "apiKey.expiresAt" should be "null"
    And I store the JSON response field "apiKey.apiKey" as "foreverKey"
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"short-key","expiresIn":{"unit":"seconds","duration":20}}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "shortKey"
    And I set header "API-Key" to "${CTX:shortKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s23/probe" until status 200
    And I send a "GET" request to "${CTX:akaCtx}/s23/probe" until status 401
    When I set header "API-Key" to "${CTX:foreverKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s23/probe"
    Then the response status code should be 200
    When I reset the request
    And I send a "DELETE" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys/forever-key"
    Then the response status should be 200
    When I set header "API-Key" to "${CTX:foreverKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s23/probe" until status 401
    When I send a "GET" request to "${CTX:akaCtx}/s23/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s23/probe" until status 404

  # Group 07 - Live config propagation
  @aka-g07 @aka-24
  Scenario: A key created after the API is deployed becomes usable without a restart
    Given I generate a unique value from "aka-s24" and store it as "akaApi"
    And I generate a unique API context from "/aka-s24" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s24/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s24/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"late-key"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "k1"
    And I set header "API-Key" to "${CTX:k1}"
    And I send a "GET" request to "${CTX:akaCtx}/s24/probe" until status 200
    When I send a "GET" request to "${CTX:akaCtx}/s24/probe"
    Then the response status code should be 200
    When I reset the request
    And I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"later-key"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "k2"
    And I set header "API-Key" to "${CTX:k2}"
    And I send a "GET" request to "${CTX:akaCtx}/s24/probe" until status 200
    When I send a "GET" request to "${CTX:akaCtx}/s24/probe"
    Then the response status code should be 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s24/probe" until status 404

  @aka-g07 @aka-25
  Scenario: Updating an API without touching its auth policy keeps existing keys working
    Given I generate a unique value from "aka-s25" and store it as "akaApi"
    And I generate a unique API context from "/aka-s25" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s25/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s25/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "k1"
    And I set header "API-Key" to "${CTX:k1}"
    And I send a "GET" request to "${CTX:akaCtx}/s25/probe" until status 200
    When I update API "${CTX:akaApi}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}-v2          |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s25/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s25/extra","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the API update response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s25/extra" until status 200
    When I send a "GET" request to "${CTX:akaCtx}/s25/probe"
    Then the response status code should be 200
    When I reset the request
    And I send a "GET" request to "${CTX:akaCtx}/s25/extra"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s25/probe" until status 404

  @aka-g07 @aka-26
  Scenario: Adding the policy to a deployed unsecured API starts enforcing it
    Given I generate a unique value from "aka-s26" and store it as "akaApi"
    And I generate a unique API context from "/aka-s26" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s26/probe"}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s26/probe" until status 200
    When I update API "${CTX:akaApi}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s26/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the API update response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s26/probe" until status 401
    When I send a "GET" request to "${CTX:akaCtx}/s26/probe"
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
    And I send a "GET" request to "${CTX:akaCtx}/s26/probe" until status 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s26/probe" until status 404

  @aka-g07 @aka-27
  Scenario: Removing the policy from a deployed API stops enforcing it
    Given I generate a unique value from "aka-s27" and store it as "akaApi"
    And I generate a unique API context from "/aka-s27" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s27/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s27/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "k1"
    And I set header "API-Key" to "${CTX:k1}"
    And I send a "GET" request to "${CTX:akaCtx}/s27/probe" until status 200
    When I send a "GET" request to "${CTX:akaCtx}/s27/probe"
    Then the response status code should be 200
    And the response should not contain echoed header "API-Key"
    When I update API "${CTX:akaApi}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s27/probe"}] |
    Then the API update response should indicate successful deployment
    And I reset the request
    And I send a "GET" request to "${CTX:akaCtx}/s27/probe" until status 200
    When I send a "GET" request to "${CTX:akaCtx}/s27/probe"
    Then the response status code should be 200
    When I set header "API-Key" to "${CTX:k1}"
    And I send a "GET" request to "${CTX:akaCtx}/s27/probe"
    Then the response status code should be 200
    And the response should contain echoed header "API-Key" with value "${CTX:k1}"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s27/probe" until status 404

  @aka-g07 @aka-28
  Scenario: Changing the configured header name rejects the old header and accepts the new one
    Given I generate a unique value from "aka-s28" and store it as "akaApi"
    And I generate a unique API context from "/aka-s28" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s28/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"X-Old-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s28/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "k1"
    And I set header "X-Old-Key" to "${CTX:k1}"
    And I send a "GET" request to "${CTX:akaCtx}/s28/probe" until status 200
    When I update API "${CTX:akaApi}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s28/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"X-New-Key","in":"header"}}]}] |
    Then the API update response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s28/probe" until status 401
    When I send a "GET" request to "${CTX:akaCtx}/s28/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I reset the request
    And I set header "X-New-Key" to "${CTX:k1}"
    And I send a "GET" request to "${CTX:akaCtx}/s28/probe" until status 200
    When I send a "GET" request to "${CTX:akaCtx}/s28/probe"
    Then the response status code should be 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s28/probe" until status 404
