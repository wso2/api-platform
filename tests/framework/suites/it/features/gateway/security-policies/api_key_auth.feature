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

  # Group 10 - Ambiguous / duplicate credentials
  @aka-g10 @aka-34
  Scenario: Duplicate identical API key headers are handled consistently
    Given I generate a unique value from "aka-s34" and store it as "akaApi"
    And I generate a unique API context from "/aka-s34" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s34/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s34/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    When I send a raw "GET" request to "${CTX:akaCtx}/s34/probe" with headers:
      | API-Key | ${CTX:akaKey} |
      | API-Key | ${CTX:akaKey} |
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I send a raw "GET" request to "${CTX:akaCtx}/s34/probe" with headers:
      | API-Key | ${CTX:akaKey} |
      | API-Key | ${CTX:akaKey} |
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I send a raw "GET" request to "${CTX:akaCtx}/s34/probe" with headers:
      | API-Key | ${CTX:akaKey} |
      | API-Key | ${CTX:akaKey} |
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I send a raw "GET" request to "${CTX:akaCtx}/s34/probe" with headers:
      | API-Key | bad |
      | API-Key | bad |
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s34/probe" until status 404

  @aka-g10 @aka-35
  Scenario: A duplicate header with one invalid value is not rescued by the other
    Given I generate a unique value from "aka-s35" and store it as "akaApi"
    And I generate a unique API context from "/aka-s35" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s35/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s35/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    When I send a raw "GET" request to "${CTX:akaCtx}/s35/probe" with headers:
      | API-Key | bad           |
      | API-Key | ${CTX:akaKey} |
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I send a raw "GET" request to "${CTX:akaCtx}/s35/probe" with headers:
      | API-Key | ${CTX:akaKey} |
      | API-Key | bad           |
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s35/probe" until status 404

  @aka-g10 @aka-36
  Scenario: Comma-separated API key values in a single header are rejected
    Given I generate a unique value from "aka-s36" and store it as "akaApi"
    And I generate a unique API context from "/aka-s36" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s36/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s36/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s36/probe" until status 200
    When I set header "API-Key" to "${CTX:akaKey},${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s36/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I set header "API-Key" to "bad,${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s36/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I set header "API-Key" to "${CTX:akaKey}, bad"
    And I send a "GET" request to "${CTX:akaCtx}/s36/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s36/probe"
    Then the response status code should be 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s36/probe" until status 404

  @aka-g10 @aka-37
  Scenario: Unrelated authentication headers are preserved and do not override API key auth
    Given I generate a unique value from "aka-s37" and store it as "akaApi"
    And I generate a unique API context from "/aka-s37" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s37/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s37/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s37/probe" until status 200
    When I set header "Authorization" to "Bearer garbage"
    And I set header "X-API-Key" to "other"
    And I set header "Cookie" to "session=abc"
    And I send a "GET" request to "${CTX:akaCtx}/s37/probe"
    Then the response status code should be 200
    And the response should contain echoed header "Authorization" with value "Bearer garbage"
    And the response should not contain echoed header "API-Key"
    When I reset the request
    And I set header "API-Key" to "bad"
    And I send a "GET" request to "${CTX:akaCtx}/s37/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s37/probe" until status 404

  # Group 11 - Policy chaining
  @aka-g11 @aka-38
  Scenario: Valid authentication lets rate limiting execute normally afterward
    Given I generate a unique value from "aka-s38" and store it as "akaApi"
    And I generate a unique API context from "/aka-s38" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s38/limited","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}},{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":3,"duration":"1h"}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s38/limited" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s38/limited" until status 200
    When I send a "GET" request to "${CTX:akaCtx}/s38/limited"
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should be "3"
    And the response header "X-RateLimit-Remaining" should exist
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s38/limited" until status 404

  @aka-g11 @aka-39
  Scenario: An invalid key stops at authentication, so later policies and the backend never run
    Given I generate a unique value from "aka-s39" and store it as "akaApi"
    And I generate a unique API context from "/aka-s39" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s39/limited","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}},{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":2,"duration":"1h"}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s39/limited" until status 401
    When I set header "API-Key" to "bad"
    And I send 5 "GET" requests to "${CTX:akaCtx}/s39/limited"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I send a "GET" request to the "capture" service at "/test/captured?path=/s39/limited" until status 204
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s39/limited" until status 200
    When I send a "GET" request to "${CTX:akaCtx}/s39/limited"
    Then the response status code should be 200
    When I send a "GET" request to "${CTX:akaCtx}/s39/limited"
    Then the response status code should be 429
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s39/limited" until status 404

  @aka-g11 @aka-40
  Scenario: A downstream rate-limit failure is reported as a rate limit, not an authentication failure
    Given I generate a unique value from "aka-s40" and store it as "akaApi"
    And I generate a unique API context from "/aka-s40" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s40/limited","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}},{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":1,"duration":"1h"}]}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s40/limited" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s40/limited" until status 200
    When I send a "GET" request to "${CTX:akaCtx}/s40/limited"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"
    And the response body should not contain "Valid API key required"
    When I set header "API-Key" to "invalid-${CTX:akaApi}"
    And I send a "GET" request to "${CTX:akaCtx}/s40/limited"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s40/limited" until status 404

  # Group 12 - Auth context and isolation
  @aka-g12 @aka-41
  Scenario: The authentication context is available to downstream policies
    Given I generate a unique value from "aka-s41" and store it as "akaApi"
    And I generate a unique API context from "/aka-s41" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s41/ctx","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}},{"name":"set-headers","version":"v1","executionCondition":"\"x-wso2-application-id\" in request.Metadata","params":{"request":{"headers":[{"name":"X-Aka-Authenticated","value":"true"}]}}}]},{"method":"GET","path":"/s41/control","policies":[{"name":"set-headers","version":"v1","executionCondition":"\"x-wso2-application-id\" in request.Metadata","params":{"request":{"headers":[{"name":"X-Aka-Authenticated","value":"true"}]}}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s41/ctx" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s41/ctx" until status 200
    When I send a "GET" request to "${CTX:akaCtx}/s41/ctx"
    Then the response status code should be 200
    And the response should contain echoed header "X-Aka-Authenticated" with value "true"
    And the response should not contain echoed header "API-Key"
    When I send a "GET" request to "${CTX:akaCtx}/s41/control"
    Then the response status code should be 200
    And the response should not contain echoed header "X-Aka-Authenticated"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s41/ctx" until status 404

  @aka-g12 @aka-42
  Scenario: Concurrent requests keep authentication isolated per request
    Given I generate a unique value from "aka-s42" and store it as "akaApi"
    And I generate a unique API context from "/aka-s42" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s42/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s42/probe" until status 401
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
    And I send a "GET" request to "${CTX:akaCtx}/s42/probe" until status 200
    When I set header "API-Key" to "${CTX:k2}"
    And I send a "GET" request to "${CTX:akaCtx}/s42/probe" until status 200
    When I set header "API-Key" to "${CTX:k3}"
    And I send a "GET" request to "${CTX:akaCtx}/s42/probe" until status 200
    When I reset the request
    And I send 100 concurrent "GET" requests to "${CTX:akaCtx}/s42/probe" cycling header "API-Key" through:
      | ${CTX:k1}         | 200 |
      | bad-${CTX:akaApi} | 401 |
      | ${CTX:k2}         | 200 |
      | <absent>          | 401 |
      | ${CTX:k3}         | 200 |
    Then every concurrent response should have its expected status
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s42/probe" until status 404

  @aka-g12 @aka-43
  Scenario: Spoofed identity headers do not bypass authentication
    Given I generate a unique value from "aka-s43" and store it as "akaApi"
    And I generate a unique API context from "/aka-s43" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s43/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s43/probe" until status 401
    When I set header "X-WSO2-Application-Id" to "attacker"
    And I set header "X-WSO2-Application-Name" to "attacker"
    And I set header "X-Authenticated" to "true"
    And I set header "X-Auth-Type" to "apikey"
    And I set header "X-Consumer-Username" to "admin"
    And I set header "X-Forwarded-User" to "admin"
    And I send a "GET" request to "${CTX:akaCtx}/s43/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I send a "GET" request to the "capture" service at "/test/captured?path=/s43/probe" until status 204
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s43/probe" until status 200
    When I send a "GET" request to the "capture" service at "/test/captured?path=/s43/probe" until status 200
    Then the response body should contain "/s43/probe"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s43/probe" until status 404

  # Group 13 - Malicious, oversized and volume input
  # Each example's operation path is discriminated by <label>, because a literal shared path
  # would let one outline example's capture record be read by another: "capture" records the
  # last request per path within the whole block partition, not per scenario.
  @aka-g13 @aka-44
  Scenario Outline: A malformed API key value "<label>" is rejected without leaking it back
    Given I generate a unique value from "aka-s44" and store it as "akaApi"
    And I generate a unique API context from "/aka-s44" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s44/<label>/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s44/<label>/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    When I send a raw "GET" request to "${CTX:akaCtx}/s44/<label>/probe" with headers:
      | API-Key | <value> |
    Then the response should be a client error
    And the response body should not contain "<value>"
    When I send a "GET" request to the "capture" service at "/test/captured?path=/s44/<label>/probe" until status 204
    When I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s44/<label>/probe" until status 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s44/<label>/probe" until status 404

    Examples:
      | label         | value        |
      | sql-injection | ' OR '1'='1  |
      | wildcard      | *            |
      | prefix-only   | apip_        |
      | unicode       | ключ-🔑-κλειδί |

  # This outline drops the leak-check assertion for values that can't survive substitution into
  # its quoted step argument: an escape sequence (\r\n) decodes to real bytes on the wire but
  # stays literal text in that comparison, and a literal '"' (json-fragment) breaks the quoted
  # argument outright. A raw CTL/NUL byte isn't exercised at all - the gateway's HTTP/1.1 codec
  # closes the connection before writing any response, so there's no status to assert against.
  @aka-g13 @aka-44
  Scenario Outline: A malformed API key value "<label>" is rejected (leak assertion omitted)
    Given I generate a unique value from "aka-s44" and store it as "akaApi"
    And I generate a unique API context from "/aka-s44" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s44/<label>/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s44/<label>/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    When I send a raw "GET" request to "${CTX:akaCtx}/s44/<label>/probe" with headers:
      | API-Key | <value> |
    Then the response should be a client error
    When I send a "GET" request to the "capture" service at "/test/captured?path=/s44/<label>/probe" until status 204
    When I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s44/<label>/probe" until status 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s44/<label>/probe" until status 404

    Examples:
      | label          | value                       |
      | json-fragment  | {"apiKey":"*","valid":true} |
      | crlf-injection | abc\r\nX-Injected: yes      |

  @aka-g13 @aka-45
  Scenario: An oversized API key is rejected per limits, and the gateway stays stable
    Given I generate a unique value from "aka-s45" and store it as "akaApi"
    And I generate a unique API context from "/aka-s45" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s45/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s45/probe" until status 401
    When I generate a 4096-character value from "k" and store it as "bigKey"
    And I set header "API-Key" to "${CTX:bigKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s45/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I generate a 131072-character value from "k" and store it as "hugeKey"
    And I set header "API-Key" to "${CTX:hugeKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s45/probe" expecting rejection
    When I check the health of all gateway services
    Then all services should report healthy status
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s45/probe" until status 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s45/probe" until status 404

  @aka-g13 @aka-46
  Scenario: High-volume invalid traffic is consistently rejected
    Given I generate a unique value from "aka-s46" and store it as "akaApi"
    And I generate a unique API context from "/aka-s46" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s46/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s46/probe" until status 401
    When I send 300 concurrent "GET" requests to "${CTX:akaCtx}/s46/probe" cycling header "API-Key" through:
      | bad-a-${CTX:akaApi} | 401 |
      | bad-b-${CTX:akaApi} | 401 |
      | <absent>            | 401 |
    Then every concurrent response should have its expected status
    When I check the health of all gateway services
    Then all services should report healthy status
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s46/probe" until status 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s46/probe" until status 404

  @aka-g13 @aka-47
  Scenario: Mixed valid and invalid concurrent traffic only lets the valid requests reach the backend
    Given I generate a unique value from "aka-s47" and store it as "akaApi"
    And I generate a unique API context from "/aka-s47" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s47/only-invalid","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s47/mixed","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s47/only-invalid" until status 401
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
    When I send 60 concurrent "GET" requests to "${CTX:akaCtx}/s47/only-invalid" cycling header "API-Key" through:
      | bad-${CTX:akaApi} | 401 |
      | <absent>          | 401 |
    Then every concurrent response should have its expected status
    When I send a "GET" request to the "capture" service at "/test/captured?path=/s47/only-invalid" until status 204
    When I set header "API-Key" to "${CTX:k1}"
    And I send a "GET" request to "${CTX:akaCtx}/s47/mixed" until status 200
    When I set header "API-Key" to "${CTX:k2}"
    And I send a "GET" request to "${CTX:akaCtx}/s47/mixed" until status 200
    When I reset the request
    And I send 120 concurrent "GET" requests to "${CTX:akaCtx}/s47/mixed" cycling header "API-Key" through:
      | ${CTX:k1}         | 200 |
      | bad-${CTX:akaApi} | 401 |
      | ${CTX:k2}         | 200 |
      | <absent>          | 401 |
    Then every concurrent response should have its expected status
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s47/only-invalid" until status 404

  # Group 14 - Response and log hygiene
  @aka-g14 @aka-48
  Scenario: Error responses do not leak key material or the rejection reason
    Given I generate a unique value from "aka-s48" and store it as "akaApi"
    And I generate a unique API context from "/aka-s48" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s48/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s48/probe" until status 401
    When I set header "API-Key" to "${CTX:akaApi}-wrong-key-0123456789abcdef-xyz"
    And I send a "GET" request to "${CTX:akaCtx}/s48/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    And the response header "Content-Type" should contain "application/json"
    And the response body should not contain "${CTX:akaApi}-wrong-key-0123456789abcdef-xyz"
    And the response body should not contain "apip_"
    And the response body should not contain "revoked"
    And the response body should not contain "expired"
    And the response header "API-Key" should not exist
    When I reset the request
    And I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"short-key","expiresIn":{"unit":"seconds","duration":5}}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "shortKey"
    And I set header "API-Key" to "${CTX:shortKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s48/probe" until status 200
    And I send a "GET" request to "${CTX:akaCtx}/s48/probe" until status 401
    When I send a "GET" request to "${CTX:akaCtx}/s48/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    And the response header "Content-Type" should contain "application/json"
    And the response body should not contain "${CTX:shortKey}"
    And the response body should not contain "apip_"
    And the response body should not contain "revoked"
    And the response body should not contain "expired"
    And the response header "API-Key" should not exist
    When I reset the request
    And I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"revoke-key"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "revokeKey"
    And I set header "API-Key" to "${CTX:revokeKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s48/probe" until status 200
    When I reset the request
    And I send a "DELETE" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys/revoke-key"
    Then the response status should be 200
    When I set header "API-Key" to "${CTX:revokeKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s48/probe" until status 401
    When I send a "GET" request to "${CTX:akaCtx}/s48/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    And the response header "Content-Type" should contain "application/json"
    And the response body should not contain "${CTX:revokeKey}"
    And the response body should not contain "apip_"
    And the response body should not contain "revoked"
    And the response body should not contain "expired"
    And the response header "API-Key" should not exist
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s48/probe" until status 404

  @aka-g14 @aka-49
  Scenario: Secrets are not logged in plaintext
    Given I generate a unique value from "aka-s49" and store it as "akaApi"
    And I generate a unique API context from "/aka-s49" and store it as "akaCtx"
    And I generate a unique resource name from "aka-s49" and store it as "secretSeed"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s49/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s49/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"external-key","apiKey":"${CTX:secretSeed}-log-hygiene-marker-0123456789abcdef"}
      """
    Then the response status should be 201
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"generated-key"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "genKey"
    And I set header "API-Key" to "${CTX:secretSeed}-log-hygiene-marker-0123456789abcdef"
    And I send a "GET" request to "${CTX:akaCtx}/s49/probe" until status 200
    When I set header "API-Key" to "${CTX:genKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s49/probe"
    Then the response status code should be 200
    When I set header "API-Key" to "${CTX:secretSeed}-wrong-marker-0123456789abcdef"
    And I send a "GET" request to "${CTX:akaCtx}/s49/probe"
    Then the response status code should be 401
    Then the "gateway-runtime" service logs should contain "${CTX:akaApi}"
    And the "gateway-runtime" service logs should not contain "log-hygiene-marker-0123456789abcdef"
    And the "gateway-runtime" service logs should not contain "wrong-marker-0123456789abcdef"
    And the "gateway-runtime" service logs should not contain "${CTX:genKey}"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s49/probe" until status 404

  # Group 16 - Backend availability
  @aka-g16 @aka-53
  Scenario: A valid key passes authentication even when the backend is down
    Given I generate a unique value from "aka-s53" and store it as "akaApi"
    And I generate a unique API context from "/aka-s53" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | http://testbench:3999     |
      | spec.operations        | [{"method":"GET","path":"/s53/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s53/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s53/probe" until status 503
    When I send a "GET" request to "${CTX:akaCtx}/s53/probe"
    Then the response should be a server error
    And the response body should not contain "Valid API key required"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s53/probe" until status 404

  @aka-g16 @aka-54
  Scenario: An invalid key fails authentication before the backend is ever reached
    Given I generate a unique value from "aka-s54" and store it as "akaApi"
    And I generate a unique API context from "/aka-s54" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | http://testbench:3999     |
      | spec.operations        | [{"method":"GET","path":"/s54/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s54/probe" until status 401
    When I set header "API-Key" to "invalid-${CTX:akaApi}"
    And I send a "GET" request to "${CTX:akaCtx}/s54/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I reset the request
    And I send a "GET" request to "${CTX:akaCtx}/s54/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s54/probe" until status 404

  # Group 17 - Deploy-time config validation
  @aka-g17 @aka-55
  Scenario Outline: Missing mandatory parameters are rejected at deploy time
    Given I generate a unique value from "aka-s55" and store it as "akaApi"
    And I generate a unique API context from "/aka-s55" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s55/probe","policies":[<policy>]}] |
    Then the response status code should be 400
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"
    And the response body should contain "is required"
    When I get the API "${CTX:akaApi}"
    Then the response status should be 404

    Examples:
      | policy                                                            |
      | {"name":"api-key-auth","version":"v1"}                            |
      | {"name":"api-key-auth","version":"v1","params":{"key":"API-Key"}} |
      | {"name":"api-key-auth","version":"v1","params":{"in":"header"}}   |

  @aka-g17 @aka-56
  Scenario Outline: An invalid "<label>" parameter is rejected at deploy time
    Given I generate a unique value from "aka-s56" and store it as "akaApi"
    And I generate a unique API context from "/aka-s56" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s56/<label>/probe","policies":[{"name":"api-key-auth","version":"v1","params":<params>}]}] |
    Then the response status code should be 400
    And the JSON response field "status" should be "error"
    And the JSON response field "message" should be "Configuration validation failed"
    And the response body should contain "<expected>"
    When I get the API "${CTX:akaApi}"
    Then the response status should be 404

    Examples:
      | label         | params                                         | expected            |
      | key-number    | {"key":123,"in":"header"}                      | Invalid type        |
      | in-query      | {"key":"API-Key","in":"query"}                 | must be one of      |
      | in-cookie     | {"key":"API-Key","in":"cookie"}                | must be one of      |
      | in-number     | {"key":"API-Key","in":5}                       | Invalid type        |
      | unknown-param | {"key":"API-Key","in":"header","header":"X"}   | Additional property |
      # | key-empty     | {"key":"","in":"header"}                       | key                 |

  @aka-g17 @aka-57
  Scenario Outline: An unsupported policy version "<version>" is rejected, not silently unauthenticated
    Given I generate a unique value from "aka-s57" and store it as "akaApi"
    And I generate a unique API context from "/aka-s57" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s57/probe","policies":[{"name":"api-key-auth","version":"<version>","params":{"key":"API-Key","in":"header"}}]}] |
    Then the response status code should be 400
    And the JSON response field "status" should be "error"
    And the response body should contain "<expected>"
    When I get the API "${CTX:akaApi}"
    Then the response status should be 404

    Examples:
      | version | expected                               |
      | v2      | not found in loaded policy definitions |
      | v999    | not found in loaded policy definitions |
      | v1.2.1  | must be major-only                     |

  # Group 18 - Config-update convergence
  @aka-g18 @aka-58
  Scenario: Rapid API updates under traffic converge without a transient authentication bypass
    Given I generate a unique value from "aka-s58" and store it as "akaApi"
    And I generate a unique API context from "/aka-s58" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s58/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s58/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "k1"
    And I set header "API-Key" to "${CTX:k1}"
    And I send a "GET" request to "${CTX:akaCtx}/s58/probe" until status 200
    When I start background "GET" traffic to "${CTX:akaCtx}/s58/probe" with header "API-Key" set to "bad-${CTX:akaApi}" as "badProbe"
    And I start background "GET" traffic to "${CTX:akaCtx}/s58/probe" with header "X-Unrelated" set to "1" as "noKey"
    And I update API "${CTX:akaApi}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}-u1          |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s58/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s58/extra-1","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the API update response should indicate successful deployment
    When I update API "${CTX:akaApi}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}-u2          |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s58/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s58/extra-1","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s58/extra-2","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the API update response should indicate successful deployment
    When I update API "${CTX:akaApi}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}-u3          |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s58/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s58/extra-1","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s58/extra-2","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s58/extra-3","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the API update response should indicate successful deployment
    When I update API "${CTX:akaApi}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}-u4          |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s58/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s58/extra-1","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s58/extra-2","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s58/extra-3","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s58/extra-4","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the API update response should indicate successful deployment
    When I update API "${CTX:akaApi}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}-u5          |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s58/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s58/extra-1","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s58/extra-2","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s58/extra-3","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s58/extra-4","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s58/extra-5","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the API update response should indicate successful deployment
    When I update API "${CTX:akaApi}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}-u6          |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s58/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s58/extra-1","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s58/extra-2","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s58/extra-3","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s58/extra-4","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s58/extra-5","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s58/final","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the API update response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s58/final" until status 200
    When I reset the request
    And I send a "GET" request to "${CTX:akaCtx}/s58/final"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I stop background traffic "badProbe"
    And I stop background traffic "noKey"
    Then background traffic "badProbe" should have recorded at least 10 attempts
    And background traffic "badProbe" should never have received status 200
    And background traffic "noKey" should have recorded at least 10 attempts
    And background traffic "noKey" should never have received status 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s58/probe" until status 404

  # Group 19 - Protocol consistency
  @aka-g19 @aka-59
  Scenario: HTTP/1.1 and HTTP/2 enforce authentication identically
    Given I generate a unique value from "aka-s59" and store it as "akaApi"
    And I generate a unique API context from "/aka-s59" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s59/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s59/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "k1"
    And I set header "API-Key" to "${CTX:k1}"
    And I send a "GET" request to "${CTX:akaCtx}/s59/probe" until status 200
    When I send a "GET" request to "${CTX:akaCtx}/s59/probe"
    Then the response status code should be 200
    When I set header "API-Key" to "invalid-${CTX:akaApi}"
    And I send a "GET" request to "${CTX:akaCtx}/s59/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I reset the request
    And I send a "GET" request to "${CTX:akaCtx}/s59/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I set header "API-Key" to "${CTX:k1}"
    And I send a "GET" request to "${CTX:akaCtx}/s59/probe" over HTTP/2
    Then the response status code should be 200
    When I set header "API-Key" to "invalid-${CTX:akaApi}"
    And I send a "GET" request to "${CTX:akaCtx}/s59/probe" over HTTP/2
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I reset the request
    And I send a "GET" request to "${CTX:akaCtx}/s59/probe" over HTTP/2
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s59/probe" until status 404

  # Group 20 - Large payload
  @aka-g20 @aka-60
  Scenario: A large request body with an invalid key is rejected before it reaches the backend
    Given I generate a unique value from "aka-s60" and store it as "akaApi"
    And I generate a unique API context from "/aka-s60" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"POST","path":"/s60/upload","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "POST" request to "${CTX:akaCtx}/s60/upload" until status 401 with body:
      """
      {"ok":true}
      """
    When I generate a 1048576-character value from "x" and store it as "bigBody"
    And I set header "API-Key" to "invalid-${CTX:akaApi}"
    And I send a "POST" request to "${CTX:akaCtx}/s60/upload" with body:
      """
      {"blob":"${CTX:bigBody}"}
      """
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I send a "GET" request to the "capture" service at "/test/captured?path=/s60/upload" until status 204
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "POST" request to "${CTX:akaCtx}/s60/upload" until status 200 with body:
      """
      {"ok":true}
      """
    When I send a "GET" request to the "capture" service at "/test/captured?path=/s60/upload" until status 200
    Then the response body should contain "/s60/upload"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "POST" request to "${CTX:akaCtx}/s60/upload" until status 404 with body:
      """
      {}
      """

  # Group 21 - CORS interplay
  # Policy order matters: cors first lets the preflight short-circuit before AKA runs; with
  # AKA first, the preflight itself would be rejected with 401.
  @aka-g21 @aka-61
  Scenario: A CORS preflight succeeds without a key, but the actual call still needs one
    Given I generate a unique value from "aka-s61" and store it as "akaApi"
    And I generate a unique API context from "/aka-s61" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.policies          | [{"name":"cors","version":"v1","params":{"allowedOrigins":["http://example.com"],"allowedMethods":["GET"],"allowedHeaders":["API-Key","Content-Type"]}},{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}] |
      | spec.operations        | [{"method":"GET","path":"/s61/data"},{"method":"OPTIONS","path":"/s61/data"}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s61/data" until status 401
    When I set header "Origin" to "http://example.com"
    And I set header "Access-Control-Request-Method" to "GET"
    And I set header "Access-Control-Request-Headers" to "API-Key"
    And I send a "OPTIONS" request to "${CTX:akaCtx}/s61/data"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should be "http://example.com"
    And the response header "Access-Control-Allow-Headers" should contain "API-Key"
    When I reset the request
    And I set header "Origin" to "http://example.com"
    And I send a "GET" request to "${CTX:akaCtx}/s61/data"
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
    And I send a "GET" request to "${CTX:akaCtx}/s61/data" until status 200
    When I send a "GET" request to "${CTX:akaCtx}/s61/data"
    Then the response status code should be 200
    And the response header "Access-Control-Allow-Origin" should be "http://example.com"
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s61/data" until status 404

  # Group 22 - API lifecycle churn
  @aka-g22 @aka-62
  Scenario: Deleting and recreating an API on the same context does not carry old keys forward
    Given I generate a unique value from "aka-s62" and store it as "akaApi"
    And I generate a unique API context from "/aka-s62" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s62/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s62/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "oldKey"
    And I set header "API-Key" to "${CTX:oldKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s62/probe" until status 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s62/probe" until status 404
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s62/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s62/probe" until status 401
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys"
    Then the response body should not contain "key-one"
    When I send 20 concurrent "GET" requests to "${CTX:akaCtx}/s62/probe" cycling header "API-Key" through:
      | ${CTX:oldKey} | 401 |
    Then every concurrent response should have its expected status
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And the JSON response field "apiKey.apiKey" should not equal "${CTX:oldKey}"
    And I store the JSON response field "apiKey.apiKey" as "newKey"
    And I set header "API-Key" to "${CTX:newKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s62/probe" until status 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s62/probe" until status 404
