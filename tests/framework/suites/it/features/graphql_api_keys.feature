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

# Mirrors api_keys.feature (RestApi) scenario-for-scenario against the
# /graphql-apis/{id}/api-keys endpoints. The API key CRUD logic itself is
# shared, kind-agnostic service code (utils.APIKeyService), so this exists
# primarily to guard the gateway-controller wiring specific to the GraphQL
# path: the OpenAPI spec paths, the ServerInterface methods, and the
# relativeRoles auth-route map entries in cmd/controller/main.go.
@graphql-api-keys
Feature: GraphQL API key management
  As an API administrator
  I want to manage API keys for GraphQL APIs
  So that I can control access through API key authentication

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Complete API key lifecycle - generate, list, regenerate, and revoke
    Given I generate a unique value from "test-key" and store it as "apiKeyName"
    Given I generate a unique value from "graphql-apikey-lifecycle-api" and store it as "graphqlApiKeyName1_1"
    Given I generate a unique API context from "/graphql-apikey-lifecycle" and store it as "graphqlApiKeyContext1_1"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}          |
      | name                   | ${CTX:graphqlApiKeyName1_1}        |
      | spec.displayName       | GraphQL-APIKey-Lifecycle-API       |
      | spec.version           | v1.0                                |
      | spec.context           | ${CTX:graphqlApiKeyContext1_1}     |
      | spec.upstream.main.url | http://testbench:3000/graphql      |
    Then the response should be successful
    When I send a "POST" request to the "gateway-controller" service at "/graphql-apis/${CTX:graphqlApiKeyName1_1}/api-keys" with body:
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
    When I send a "GET" request to the "gateway-controller" service at "/graphql-apis/${CTX:graphqlApiKeyName1_1}/api-keys"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "${CTX:apiKeyName}"
    When I send a "POST" request to the "gateway-controller" service at "/graphql-apis/${CTX:graphqlApiKeyName1_1}/api-keys/${CTX:apiKeyName}/regenerate" with body:
      """
      {}
      """
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response should have field "apiKey.apiKey"
    When I send a "DELETE" request to the "gateway-controller" service at "/graphql-apis/${CTX:graphqlApiKeyName1_1}/api-keys/${CTX:apiKeyName}"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    When I send a "GET" request to the "gateway-controller" service at "/graphql-apis/${CTX:graphqlApiKeyName1_1}/api-keys"
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should not contain "${CTX:apiKeyName}"
    When I delete the GraphQL API "${CTX:graphqlApiKeyName1_1}"
    Then the response should be successful

  Scenario: Generate multiple API keys for same GraphQL API
    Given I generate a unique value from "key-one" and store it as "firstKeyName"
    And I generate a unique value from "key-two" and store it as "secondKeyName"
    Given I generate a unique value from "graphql-multi-key-api" and store it as "graphqlApiKeyName2_1"
    Given I generate a unique API context from "/graphql-multi-key" and store it as "graphqlApiKeyContext2_1"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}      |
      | name                   | ${CTX:graphqlApiKeyName2_1}    |
      | spec.displayName       | GraphQL-Multi-Key-API          |
      | spec.version           | v1.0                            |
      | spec.context           | ${CTX:graphqlApiKeyContext2_1} |
      | spec.upstream.main.url | http://testbench:3000/graphql  |
    Then the response should be successful
    When I send a "POST" request to the "gateway-controller" service at "/graphql-apis/${CTX:graphqlApiKeyName2_1}/api-keys" with body:
      """
      {
        "name": "${CTX:firstKeyName}"
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    When I send a "POST" request to the "gateway-controller" service at "/graphql-apis/${CTX:graphqlApiKeyName2_1}/api-keys" with body:
      """
      {
        "name": "${CTX:secondKeyName}"
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    When I send a "GET" request to the "gateway-controller" service at "/graphql-apis/${CTX:graphqlApiKeyName2_1}/api-keys"
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should contain "${CTX:firstKeyName}"
    And the response body should contain "${CTX:secondKeyName}"
    When I delete the GraphQL API "${CTX:graphqlApiKeyName2_1}"
    Then the response should be successful

  Scenario: List API keys for GraphQL API with no keys returns empty list
    Given I generate a unique value from "graphql-no-keys-api" and store it as "graphqlApiKeyName3_1"
    Given I generate a unique API context from "/graphql-no-keys" and store it as "graphqlApiKeyContext3_1"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}      |
      | name                   | ${CTX:graphqlApiKeyName3_1}    |
      | spec.displayName       | GraphQL-No-Keys-API            |
      | spec.version           | v1.0                            |
      | spec.context           | ${CTX:graphqlApiKeyContext3_1} |
      | spec.upstream.main.url | http://testbench:3000/graphql  |
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/graphql-apis/${CTX:graphqlApiKeyName3_1}/api-keys"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    When I delete the GraphQL API "${CTX:graphqlApiKeyName3_1}"
    Then the response should be successful

  Scenario: Generate API key for non-existent GraphQL API returns 404
    When I send a "POST" request to the "gateway-controller" service at "/graphql-apis/non-existent-api-id/api-keys" with body:
      """
      {
        "name": "test-key"
      }
      """
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Generate API key without name auto-generates name
    Given I generate a unique value from "graphql-key-validation-api" and store it as "graphqlApiKeyName5_1"
    Given I generate a unique API context from "/graphql-key-validation" and store it as "graphqlApiKeyContext5_1"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}      |
      | name                   | ${CTX:graphqlApiKeyName5_1}    |
      | spec.displayName       | GraphQL-Key-Validation-API     |
      | spec.version           | v1.0                            |
      | spec.context           | ${CTX:graphqlApiKeyContext5_1} |
      | spec.upstream.main.url | http://testbench:3000/graphql  |
    Then the response should be successful
    When I send a "POST" request to the "gateway-controller" service at "/graphql-apis/${CTX:graphqlApiKeyName5_1}/api-keys" with body:
      """
      {}
      """
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response should have field "apiKey"
    When I delete the GraphQL API "${CTX:graphqlApiKeyName5_1}"
    Then the response should be successful

  Scenario: List API keys for non-existent GraphQL API returns 404
    When I send a "GET" request to the "gateway-controller" service at "/graphql-apis/non-existent-api-id/api-keys"
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: List API keys with invalid GraphQL API ID format returns 404
    When I send a "GET" request to the "gateway-controller" service at "/graphql-apis/invalid@api!id/api-keys"
    Then the response status should be 404
    And the response should be valid JSON

  Scenario: Revoke API key with invalid formats returns 404
    When I send a "DELETE" request to the "gateway-controller" service at "/graphql-apis/invalid@api/api-keys/invalid@key"
    Then the response status should be 404
    And the response should be valid JSON

  Scenario: Revoke non-existent API key returns success (idempotent)
    Given I generate a unique value from "graphql-revoke-error-api" and store it as "graphqlApiKeyName9_1"
    Given I generate a unique API context from "/graphql-revoke-error" and store it as "graphqlApiKeyContext9_1"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}      |
      | name                   | ${CTX:graphqlApiKeyName9_1}    |
      | spec.displayName       | GraphQL-Revoke-Error-API       |
      | spec.version           | v1.0                            |
      | spec.context           | ${CTX:graphqlApiKeyContext9_1} |
      | spec.upstream.main.url | http://testbench:3000/graphql  |
    Then the response should be successful
    When I send a "DELETE" request to the "gateway-controller" service at "/graphql-apis/${CTX:graphqlApiKeyName9_1}/api-keys/non-existent-key"
    Then the response status should be 200
    And the response should be valid JSON
    When I delete the GraphQL API "${CTX:graphqlApiKeyName9_1}"
    Then the response should be successful

  Scenario: Regenerate API key for non-existent GraphQL API returns 404
    When I send a "POST" request to the "gateway-controller" service at "/graphql-apis/non-existent-api-id/api-keys/test-key/regenerate" with body:
      """
      {}
      """
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Regenerate non-existent API key returns 404
    Given I generate a unique value from "graphql-test-regenerate-api" and store it as "graphqlApiKeyName11_1"
    Given I generate a unique API context from "/graphql-test-regen" and store it as "graphqlApiKeyContext11_1"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}       |
      | name                   | ${CTX:graphqlApiKeyName11_1}    |
      | spec.displayName       | GraphQL-Test-Regenerate-Api     |
      | spec.version           | v1.0                             |
      | spec.context           | ${CTX:graphqlApiKeyContext11_1} |
      | spec.upstream.main.url | http://testbench:3000/graphql   |
    Then the response should be successful
    When I send a "POST" request to the "gateway-controller" service at "/graphql-apis/${CTX:graphqlApiKeyName11_1}/api-keys/non-existent-key/regenerate" with body:
      """
      {}
      """
    Then the response status should be 404
    When I delete the GraphQL API "${CTX:graphqlApiKeyName11_1}"
    Then the response should be successful

  Scenario: Regenerate API key with invalid ID formats returns 404
    When I send a "POST" request to the "gateway-controller" service at "/graphql-apis/invalid@api/api-keys/invalid@key/regenerate" with body:
      """
      {}
      """
    Then the response status should be 404
    And the response should be valid JSON

  Scenario: Generate API key with invalid JSON body returns error
    Given I generate a unique value from "graphql-invalid-json-key-api" and store it as "graphqlApiKeyName13_1"
    Given I generate a unique API context from "/graphql-invalid-json-key" and store it as "graphqlApiKeyContext13_1"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}       |
      | name                   | ${CTX:graphqlApiKeyName13_1}    |
      | spec.displayName       | GraphQL-Invalid-JSON-Key-API    |
      | spec.version           | v1.0                             |
      | spec.context           | ${CTX:graphqlApiKeyContext13_1} |
      | spec.upstream.main.url | http://testbench:3000/graphql   |
    Then the response should be successful
    When I send a "POST" request to the "gateway-controller" service at "/graphql-apis/${CTX:graphqlApiKeyName13_1}/api-keys" with body:
      """
      { this is not valid json
      """
    Then the response should be a client error
    And the response should be valid JSON
    When I delete the GraphQL API "${CTX:graphqlApiKeyName13_1}"
    Then the response should be successful

  Scenario: API key with special characters in name
    Given I generate a unique value from "graphql-special-char-key-api" and store it as "graphqlApiKeyName14_1"
    Given I generate a unique API context from "/graphql-special-char-key" and store it as "graphqlApiKeyContext14_1"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}       |
      | name                   | ${CTX:graphqlApiKeyName14_1}    |
      | spec.displayName       | GraphQL-Special-Char-Key-API    |
      | spec.version           | v1.0                             |
      | spec.context           | ${CTX:graphqlApiKeyContext14_1} |
      | spec.upstream.main.url | http://testbench:3000/graphql   |
    Then the response should be successful
    When I send a "POST" request to the "gateway-controller" service at "/graphql-apis/${CTX:graphqlApiKeyName14_1}/api-keys" with body:
      """
      {
        "name": "my-api-key_v1"
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    When I delete the GraphQL API "${CTX:graphqlApiKeyName14_1}"
    Then the response should be successful

  Scenario: List API keys with pagination parameters
    Given I generate a unique value from "graphql-paginated-keys-api" and store it as "graphqlApiKeyName15_1"
    Given I generate a unique API context from "/graphql-paginated-keys" and store it as "graphqlApiKeyContext15_1"
    When I create GraphQL API from "resources/templates/graphql-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}       |
      | name                   | ${CTX:graphqlApiKeyName15_1}    |
      | spec.displayName       | GraphQL-Paginated-Keys-API      |
      | spec.version           | v1.0                             |
      | spec.context           | ${CTX:graphqlApiKeyContext15_1} |
      | spec.upstream.main.url | http://testbench:3000/graphql   |
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/graphql-apis/${CTX:graphqlApiKeyName15_1}/api-keys?limit=10&offset=0"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    When I delete the GraphQL API "${CTX:graphqlApiKeyName15_1}"
    Then the response should be successful
