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
Feature: API key authentication policy under gateway and auth-component restarts
  As an API developer
  I want the api-key-auth policy to keep authenticating correctly across gateway-runtime and
  gateway-controller restarts, and to fail closed rather than open when the policy engine itself
  is unavailable or hung
  So that a control-plane or data-plane disruption never becomes an authentication bypass

  Background:
    Given the first gateway is running
    And I authenticate using basic auth as "admin"
    And I resolve the "capture" service URL at "" and store it as "captureUpstream"

  # Group 08 - Process restart resiliency
  @aka-g08 @aka-29
  Scenario: An existing key keeps working after a gateway-runtime restart, with no bypass during it
    Given I generate a unique value from "aka-s29" and store it as "akaApi"
    And I generate a unique API context from "/aka-s29" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s29/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s29/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s29/probe" until status 200
    When I start background "GET" traffic to "${CTX:akaCtx}/s29/probe" with header "API-Key" set to "invalid-${CTX:akaApi}" as "badProbe"
    And I restart the "gateway-runtime" service
    And I send a "GET" request to "${CTX:akaCtx}/s29/probe" until status 200
    When I send a "GET" request to "${CTX:akaCtx}/s29/probe"
    Then the response status code should be 200
    When I set header "API-Key" to "invalid-${CTX:akaApi}"
    And I send a "GET" request to "${CTX:akaCtx}/s29/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I stop background traffic "badProbe"
    Then background traffic "badProbe" should have recorded at least 10 attempts
    And background traffic "badProbe" should never have received status 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s29/probe" until status 404

  @aka-g08 @aka-30
  Scenario: A gateway-controller restart keeps the runtime enforcing while it is down, and restores config after
    Given I generate a unique value from "aka-s30" and store it as "akaApi"
    And I generate a unique API context from "/aka-s30" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s30/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s30/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "k1"
    And I set header "API-Key" to "${CTX:k1}"
    And I send a "GET" request to "${CTX:akaCtx}/s30/probe" until status 200
    When I start background "GET" traffic to "${CTX:akaCtx}/s30/probe" with header "API-Key" set to "invalid-${CTX:akaApi}" as "badProbe"
    And I stop the gateway service "gateway-controller"
    When I send a "GET" request to "${CTX:akaCtx}/s30/probe"
    Then the response status code should be 200
    When I set header "API-Key" to "invalid-${CTX:akaApi}"
    And I send a "GET" request to "${CTX:akaCtx}/s30/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I start the gateway service "gateway-controller"
    And I wait for the gateway controller health endpoint
    When I reset the request
    And I send a "DELETE" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys/key-one"
    Then the response status should be 200
    When I set header "API-Key" to "${CTX:k1}"
    And I send a "GET" request to "${CTX:akaCtx}/s30/probe" until status 401
    When I send a "GET" request to "${CTX:akaCtx}/s30/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-two"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "k2"
    And I set header "API-Key" to "${CTX:k2}"
    And I send a "GET" request to "${CTX:akaCtx}/s30/probe" until status 200
    When I send a "GET" request to "${CTX:akaCtx}/s30/probe"
    Then the response status code should be 200
    When I stop background traffic "badProbe"
    Then background traffic "badProbe" should have recorded at least 10 attempts
    And background traffic "badProbe" should never have received status 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s30/probe" until status 404

  # Group 15 - Auth-component fail-safe
  @aka-g15 @aka-50
  Scenario: The gateway fails closed when the policy engine becomes unavailable
    Given I generate a unique value from "aka-s50" and store it as "akaApi"
    And I generate a unique API context from "/aka-s50" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s50/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s50/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s50/probe" until status 200
    When I start background "GET" traffic to "${CTX:akaCtx}/s50/probe" with header "API-Key" set to "invalid-${CTX:akaApi}" as "badProbe"
    And I send signal "KILL" to the "policy-engine" process in the "gateway-runtime" service
    And I check the health of all gateway services until service "router" is unhealthy
    Then the health check should report service "policy-engine" as unhealthy
    And the health check should report service "router" as unhealthy
    When I restart the "gateway-runtime" service
    And I send a "GET" request to "${CTX:akaCtx}/s50/probe" until status 200
    When I stop background traffic "badProbe"
    Then background traffic "badProbe" should have recorded at least 5 attempts
    And background traffic "badProbe" should never have received status 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s50/probe" until status 404

  @aka-g15 @aka-51
  Scenario: A hung policy engine fails the request securely instead of letting it through
    Given I generate a unique value from "aka-s51" and store it as "akaApi"
    And I generate a unique API context from "/aka-s51" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s51/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]},{"method":"GET","path":"/s51/fresh","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s51/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "akaKey"
    And I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s51/probe" until status 200
    When I send signal "STOP" to the "policy-engine" process in the "gateway-runtime" service
    And I reset the request
    And I send a "GET" request to "${CTX:akaCtx}/s51/fresh"
    Then the response should be a server error
    And the gateway should have responded within "10" seconds
    When I set header "API-Key" to "${CTX:akaKey}"
    And I send a "GET" request to "${CTX:akaCtx}/s51/probe"
    Then the response should be a server error
    When I send a "GET" request to the "capture" service at "/test/captured?path=/s51/fresh" until status 204
    When I send signal "CONT" to the "policy-engine" process in the "gateway-runtime" service
    And I send a "GET" request to "${CTX:akaCtx}/s51/probe" until status 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s51/probe" until status 404

  @aka-g15 @aka-52
  Scenario: Once the policy engine is restored, valid keys work again and invalid keys stay rejected
    Given I generate a unique value from "aka-s52" and store it as "akaApi"
    And I generate a unique API context from "/aka-s52" and store it as "akaCtx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:akaApi}             |
      | spec.displayName       | ${CTX:akaApi}             |
      | spec.version           | v1.0                      |
      | spec.context           | ${CTX:akaCtx}             |
      | spec.upstream.main.url | ${CTX:captureUpstream}    |
      | spec.operations        | [{"method":"GET","path":"/s52/probe","policies":[{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}]}] |
    Then the resource creation response should indicate successful deployment
    And I send a "GET" request to "${CTX:akaCtx}/s52/probe" until status 401
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-one"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "k1"
    And I set header "API-Key" to "${CTX:k1}"
    And I send a "GET" request to "${CTX:akaCtx}/s52/probe" until status 200
    When I send signal "STOP" to the "policy-engine" process in the "gateway-runtime" service
    And I send a "GET" request to "${CTX:akaCtx}/s52/probe"
    Then the response should be a server error
    When I send signal "CONT" to the "policy-engine" process in the "gateway-runtime" service
    And I send a "GET" request to "${CTX:akaCtx}/s52/probe" until status 200
    When I send a "GET" request to "${CTX:akaCtx}/s52/probe"
    Then the response status code should be 200
    When I set header "API-Key" to "invalid-${CTX:akaApi}"
    And I send a "GET" request to "${CTX:akaCtx}/s52/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I reset the request
    And I send a "GET" request to "${CTX:akaCtx}/s52/probe"
    Then the response status code should be 401
    And the JSON response field "error" should be "Unauthorized"
    And the JSON response field "message" should be "Valid API key required"
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:akaApi}/api-keys" with body:
      """
      {"name":"key-two"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "k2"
    And I set header "API-Key" to "${CTX:k2}"
    And I send a "GET" request to "${CTX:akaCtx}/s52/probe" until status 200
    When I send a "GET" request to "${CTX:akaCtx}/s52/probe"
    Then the response status code should be 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:akaApi}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:akaCtx}/s52/probe" until status 404
