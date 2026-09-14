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
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

@api-keys
Feature: API key management
  As an API administrator
  I want to manage API keys for APIs
  So that I can control access through API key authentication

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Complete API key lifecycle - generate, list, regenerate, and revoke
    Given I generate a unique value from "test-key" and store it as "apiKeyName"
    Given I generate a unique value from "apikey-lifecycle-api" and store it as "apiKeyName1_1"
    Given I generate a unique API context from "/apikey-lifecycle" and store it as "apiKeyContext1_1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiKeyName1_1}              |
      | spec.displayName       | APIKey-Lifecycle-API              |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiKeyContext1_1}            |
      | spec.upstream.main.url | http://testbench:3000              |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
    Then the response should be successful
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:apiKeyName1_1}/api-keys" with body:
      """
      {
        "name": "${CTX:apiKeyName}"
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response should have field "apiKey"
    And the JSON response should have field "apiKey.name"
    And the JSON response should have field "apiKey.apiKey"
    And I wait for policy snapshot sync
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:apiKeyName1_1}/api-keys"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "${CTX:apiKeyName}"
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:apiKeyName1_1}/api-keys/${CTX:apiKeyName}/regenerate" with body:
      """
      {}
      """
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response should have field "apiKey.apiKey"
    When I send a "DELETE" request to the "gateway-controller" service at "/rest-apis/${CTX:apiKeyName1_1}/api-keys/${CTX:apiKeyName}"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:apiKeyName1_1}/api-keys"
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should not contain "${CTX:apiKeyName}"
    When I delete the API "${CTX:apiKeyName1_1}"
    Then the response should be successful

  Scenario: Generate multiple API keys for same API
    Given I generate a unique value from "key-one" and store it as "firstKeyName"
    And I generate a unique value from "key-two" and store it as "secondKeyName"
    Given I generate a unique value from "multi-key-api" and store it as "apiKeyName2_1"
    Given I generate a unique API context from "/multi-key" and store it as "apiKeyContext2_1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiKeyName2_1}              |
      | spec.displayName       | Multi-Key-API                     |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiKeyContext2_1}            |
      | spec.upstream.main.url | http://testbench:3000              |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
    Then the response should be successful
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:apiKeyName2_1}/api-keys" with body:
      """
      {
        "name": "${CTX:firstKeyName}"
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:apiKeyName2_1}/api-keys" with body:
      """
      {
        "name": "${CTX:secondKeyName}"
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    And I wait for policy snapshot sync
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:apiKeyName2_1}/api-keys"
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should contain "${CTX:firstKeyName}"
    And the response body should contain "${CTX:secondKeyName}"
    When I delete the API "${CTX:apiKeyName2_1}"
    Then the response should be successful

  Scenario: List API keys for API with no keys returns empty list
    Given I generate a unique value from "no-keys-api" and store it as "apiKeyName3_1"
    Given I generate a unique API context from "/no-keys" and store it as "apiKeyContext3_1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiKeyName3_1}              |
      | spec.displayName       | No-Keys-API                       |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiKeyContext3_1}            |
      | spec.upstream.main.url | http://testbench:3000              |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:apiKeyName3_1}/api-keys"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    When I delete the API "${CTX:apiKeyName3_1}"
    Then the response should be successful

  Scenario: Generate API key for non-existent API returns 404
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/non-existent-api-id/api-keys" with body:
      """
      {
        "name": "test-key"
      }
      """
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Generate API key without name auto-generates name
    Given I generate a unique value from "key-validation-api" and store it as "apiKeyName5_1"
    Given I generate a unique API context from "/key-validation" and store it as "apiKeyContext5_1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiKeyName5_1}              |
      | spec.displayName       | Key-Validation-API                |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiKeyContext5_1}            |
      | spec.upstream.main.url | http://testbench:3000              |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
    Then the response should be successful
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:apiKeyName5_1}/api-keys" with body:
      """
      {}
      """
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response should have field "apiKey"
    When I delete the API "${CTX:apiKeyName5_1}"
    Then the response should be successful

  Scenario: List API keys for non-existent API returns 404
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/non-existent-api-id/api-keys"
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: List API keys with invalid API ID format returns 404
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/invalid@api#id/api-keys"
    Then the response status should be 404
    And the response should be valid JSON

  Scenario: Revoke API key with invalid formats returns 404
    When I send a "DELETE" request to the "gateway-controller" service at "/rest-apis/invalid@api/api-keys/invalid@key"
    Then the response status should be 404
    And the response should be valid JSON

  Scenario: Revoke non-existent API key returns success (idempotent)
    Given I generate a unique value from "revoke-error-api" and store it as "apiKeyName9_1"
    Given I generate a unique API context from "/revoke-error" and store it as "apiKeyContext9_1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiKeyName9_1}              |
      | spec.displayName       | Revoke-Error-API                  |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiKeyContext9_1}            |
      | spec.upstream.main.url | http://testbench:3000              |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
    Then the response should be successful
    When I send a "DELETE" request to the "gateway-controller" service at "/rest-apis/${CTX:apiKeyName9_1}/api-keys/non-existent-key"
    Then the response status should be 200
    And the response should be valid JSON
    When I delete the API "${CTX:apiKeyName9_1}"
    Then the response should be successful

  Scenario: Regenerate API key for non-existent API returns 404
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/non-existent-api-id/api-keys/test-key/regenerate" with body:
      """
      {}
      """
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Regenerate non-existent API key returns 404
    Given I generate a unique value from "test-regenerate-api" and store it as "apiKeyName11_1"
    Given I generate a unique API context from "/test-regen" and store it as "apiKeyContext11_1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiKeyName11_1}             |
      | spec.displayName       | Test-Regenerate-Api               |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiKeyContext11_1}           |
      | spec.upstream.main.url | http://testbench:3000              |
      | spec.operations        | [{"method":"GET","path":"/test"}] |
    Then the response should be successful
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:apiKeyName11_1}/api-keys/non-existent-key/regenerate" with body:
      """
      {}
      """
    Then the response status should be 404
    When I delete the API "${CTX:apiKeyName11_1}"
    Then the response should be successful

  Scenario: Regenerate API key with invalid ID formats returns 404
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/invalid@api/api-keys/invalid@key/regenerate" with body:
      """
      {}
      """
    Then the response status should be 404
    And the response should be valid JSON

  Scenario: Generate API key with invalid JSON body returns error
    Given I generate a unique value from "invalid-json-key-api" and store it as "apiKeyName13_1"
    Given I generate a unique API context from "/invalid-json-key" and store it as "apiKeyContext13_1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiKeyName13_1}             |
      | spec.displayName       | Invalid-JSON-Key-API              |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiKeyContext13_1}           |
      | spec.upstream.main.url | http://testbench:3000              |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
    Then the response should be successful
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:apiKeyName13_1}/api-keys" with body:
      """
      { this is not valid json
      """
    Then the response should be a client error
    And the response should be valid JSON
    When I delete the API "${CTX:apiKeyName13_1}"
    Then the response should be successful

  Scenario: API key with special characters in name
    Given I generate a unique value from "special-char-key-api" and store it as "apiKeyName14_1"
    Given I generate a unique API context from "/special-char-key" and store it as "apiKeyContext14_1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiKeyName14_1}             |
      | spec.displayName       | Special-Char-Key-API              |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiKeyContext14_1}           |
      | spec.upstream.main.url | http://testbench:3000              |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
    Then the response should be successful
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:apiKeyName14_1}/api-keys" with body:
      """
      {
        "name": "my-api-key_v1"
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    When I delete the API "${CTX:apiKeyName14_1}"
    Then the response should be successful

  Scenario: List API keys with pagination parameters
    Given I generate a unique value from "paginated-keys-api" and store it as "apiKeyName15_1"
    Given I generate a unique API context from "/paginated-keys" and store it as "apiKeyContext15_1"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiKeyName15_1}             |
      | spec.displayName       | Paginated-Keys-API                |
      | spec.version           | v1.0                               |
      | spec.context           | ${CTX:apiKeyContext15_1}           |
      | spec.upstream.main.url | http://testbench:3000              |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:apiKeyName15_1}/api-keys?limit=10&offset=0"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    When I delete the API "${CTX:apiKeyName15_1}"
    Then the response should be successful
