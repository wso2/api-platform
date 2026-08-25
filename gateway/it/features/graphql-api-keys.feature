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

# Mirrors features/api-keys.feature (RestApi) scenario-for-scenario against the
# /graphql-apis/{id}/api-keys endpoints added to close the gap identified in
# docs/specs/graphql-api-support.md - the API key CRUD logic itself is shared,
# kind-agnostic service code (utils.APIKeyService), so this exists primarily to
# guard the gateway-controller wiring specific to the GraphQL path: the OpenAPI
# spec paths, the ServerInterface methods, and the relativeRoles auth-route map
# entries in cmd/controller/main.go (a route missing from that map is denied as
# a 404 before ever reaching the handler - the exact bug this suite would have
# caught).
Feature: GraphQL API Key Management Operations
  As an API administrator
  I want to manage API keys for GraphQL APIs
  So that I can control access through API key authentication

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ==================== API KEY LIFECYCLE - SUCCESS PATH ====================

  Scenario: Complete API key lifecycle - generate, list, regenerate, and revoke
    When I deploy this GraphQL configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: GraphQLApi
      metadata:
        name: graphql-apikey-lifecycle-api
      spec:
        displayName: GraphQL APIKey Lifecycle API
        version: v1.0
        context: /graphql-apikey-lifecycle
        upstream:
          main:
            url: http://sample-backend:9080/graphql
      """
    Then the response should be successful

    # Generate API key
    When I send a POST request to the "gateway-controller" service at "/graphql-apis/graphql-apikey-lifecycle-api/api-keys" with body:
      """
      {
        "name": "test-key-1"
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response should have field "apiKey"
    And the JSON response should have field "apiKey.name"
    And the JSON response should have field "apiKey.apiKey"
    And I wait for 2 seconds

    # List API keys - should have 1 key
    When I send a GET request to the "gateway-controller" service at "/graphql-apis/graphql-apikey-lifecycle-api/api-keys"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "test-key-1"

    # Regenerate API key
    When I send a POST request to the "gateway-controller" service at "/graphql-apis/graphql-apikey-lifecycle-api/api-keys/test-key-1/regenerate" with body:
      """
      {}
      """
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response should have field "apiKey.apiKey"

    # Revoke API key
    When I send a DELETE request to the "gateway-controller" service at "/graphql-apis/graphql-apikey-lifecycle-api/api-keys/test-key-1"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

    # Verify key is revoked - list should be empty
    When I send a GET request to the "gateway-controller" service at "/graphql-apis/graphql-apikey-lifecycle-api/api-keys"
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should not contain "test-key-1"

    # Cleanup
    When I delete the GraphQL API "graphql-apikey-lifecycle-api"
    Then the response should be successful

  Scenario: Generate multiple API keys for same GraphQL API
    When I deploy this GraphQL configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: GraphQLApi
      metadata:
        name: graphql-multi-key-api
      spec:
        displayName: GraphQL Multi Key API
        version: v1.0
        context: /graphql-multi-key
        upstream:
          main:
            url: http://sample-backend:9080/graphql
      """
    Then the response should be successful

    # Generate first key
    When I send a POST request to the "gateway-controller" service at "/graphql-apis/graphql-multi-key-api/api-keys" with body:
      """
      {
        "name": "key-one"
      }
      """
    Then the response status should be 201
    And the response should be valid JSON

    # Generate second key
    When I send a POST request to the "gateway-controller" service at "/graphql-apis/graphql-multi-key-api/api-keys" with body:
      """
      {
        "name": "key-two"
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    And I wait for 2 seconds

    # List should show both keys
    When I send a GET request to the "gateway-controller" service at "/graphql-apis/graphql-multi-key-api/api-keys"
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should contain "key-one"
    And the response body should contain "key-two"

    # Cleanup
    When I delete the GraphQL API "graphql-multi-key-api"
    Then the response should be successful

  Scenario: List API keys for GraphQL API with no keys returns empty list
    When I deploy this GraphQL configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: GraphQLApi
      metadata:
        name: graphql-no-keys-api
      spec:
        displayName: GraphQL No Keys API
        version: v1.0
        context: /graphql-no-keys
        upstream:
          main:
            url: http://sample-backend:9080/graphql
      """
    Then the response should be successful
    When I send a GET request to the "gateway-controller" service at "/graphql-apis/graphql-no-keys-api/api-keys"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    # Cleanup
    When I delete the GraphQL API "graphql-no-keys-api"
    Then the response should be successful

  # ==================== GENERATE API KEY - ERROR CASES ====================

  Scenario: Generate API key for non-existent GraphQL API returns 404
    When I send a POST request to the "gateway-controller" service at "/graphql-apis/non-existent-api-id/api-keys" with body:
      """
      {
        "name": "test-key"
      }
      """
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Generate API key without name auto-generates name
    When I deploy this GraphQL configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: GraphQLApi
      metadata:
        name: graphql-key-validation-api
      spec:
        displayName: GraphQL Key Validation API
        version: v1.0
        context: /graphql-key-validation
        upstream:
          main:
            url: http://sample-backend:9080/graphql
      """
    Then the response should be successful
    When I send a POST request to the "gateway-controller" service at "/graphql-apis/graphql-key-validation-api/api-keys" with body:
      """
      {}
      """
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response should have field "apiKey"
    # Cleanup
    When I delete the GraphQL API "graphql-key-validation-api"
    Then the response should be successful

  # ==================== LIST API KEYS - ERROR CASES ====================

  Scenario: List API keys for non-existent GraphQL API returns 404
    When I send a GET request to the "gateway-controller" service at "/graphql-apis/non-existent-api-id/api-keys"
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: List API keys with invalid GraphQL API ID format returns 404
    When I send a GET request to the "gateway-controller" service at "/graphql-apis/invalid@api#id/api-keys"
    Then the response status should be 404
    And the response should be valid JSON

  # ==================== REVOKE API KEY - ERROR CASES ====================

  Scenario: Revoke API key with invalid formats returns 404
    When I send a DELETE request to the "gateway-controller" service at "/graphql-apis/invalid@api/api-keys/invalid@key"
    Then the response status should be 404
    And the response should be valid JSON

  Scenario: Revoke non-existent API key returns success (idempotent)
    When I deploy this GraphQL configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: GraphQLApi
      metadata:
        name: graphql-revoke-error-api
      spec:
        displayName: GraphQL Revoke Error API
        version: v1.0
        context: /graphql-revoke-error
        upstream:
          main:
            url: http://sample-backend:9080/graphql
      """
    Then the response should be successful
    # Revoking non-existent key is idempotent - returns success
    When I send a DELETE request to the "gateway-controller" service at "/graphql-apis/graphql-revoke-error-api/api-keys/non-existent-key"
    Then the response status should be 200
    And the response should be valid JSON
    # Cleanup
    When I delete the GraphQL API "graphql-revoke-error-api"
    Then the response should be successful

  # ==================== REGENERATE API KEY - ERROR CASES ====================

  Scenario: Regenerate API key for non-existent GraphQL API returns 404
    When I send a POST request to the "gateway-controller" service at "/graphql-apis/non-existent-api-id/api-keys/test-key/regenerate" with body:
      """
      {}
      """
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Regenerate non-existent API key returns 404
    When I deploy this GraphQL configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: GraphQLApi
      metadata:
        name: graphql-test-regenerate-api
      spec:
        displayName: GraphQL Test Regenerate API
        version: v1.0
        context: /graphql-test-regen
        upstream:
          main:
            url: http://sample-backend:9080/graphql
      """
    Then the response should be successful
    When I send a POST request to the "gateway-controller" service at "/graphql-apis/graphql-test-regenerate-api/api-keys/non-existent-key/regenerate" with body:
      """
      {}
      """
    Then the response status should be 404
    # Cleanup
    When I delete the GraphQL API "graphql-test-regenerate-api"
    Then the response should be successful

  Scenario: Regenerate API key with invalid ID formats returns 404
    When I send a POST request to the "gateway-controller" service at "/graphql-apis/invalid@api/api-keys/invalid@key/regenerate" with body:
      """
      {}
      """
    Then the response status should be 404
    And the response should be valid JSON

  # ==================== GENERATE API KEY - ADDITIONAL ERROR CASES ====================

  Scenario: Generate API key with invalid JSON body returns error
    When I deploy this GraphQL configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: GraphQLApi
      metadata:
        name: graphql-invalid-json-key-api
      spec:
        displayName: GraphQL Invalid JSON Key API
        version: v1.0
        context: /graphql-invalid-json-key
        upstream:
          main:
            url: http://sample-backend:9080/graphql
      """
    Then the response should be successful
    When I send a POST request to the "gateway-controller" service at "/graphql-apis/graphql-invalid-json-key-api/api-keys" with body:
      """
      { this is not valid json
      """
    Then the response should be a client error
    And the response should be valid JSON
    # Cleanup
    When I delete the GraphQL API "graphql-invalid-json-key-api"
    Then the response should be successful

  Scenario: API key with special characters in name
    When I deploy this GraphQL configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: GraphQLApi
      metadata:
        name: graphql-special-char-key-api
      spec:
        displayName: GraphQL Special Char Key API
        version: v1.0
        context: /graphql-special-char-key
        upstream:
          main:
            url: http://sample-backend:9080/graphql
      """
    Then the response should be successful
    # Generate key with hyphens and underscores (should be allowed)
    When I send a POST request to the "gateway-controller" service at "/graphql-apis/graphql-special-char-key-api/api-keys" with body:
      """
      {
        "name": "my-api-key_v1"
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    # Cleanup
    When I delete the GraphQL API "graphql-special-char-key-api"
    Then the response should be successful

  # ==================== LIST API KEYS WITH PAGINATION ====================

  Scenario: List API keys with pagination parameters
    When I deploy this GraphQL configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: GraphQLApi
      metadata:
        name: graphql-paginated-keys-api
      spec:
        displayName: GraphQL Paginated Keys API
        version: v1.0
        context: /graphql-paginated-keys
        upstream:
          main:
            url: http://sample-backend:9080/graphql
      """
    Then the response should be successful
    When I send a GET request to the "gateway-controller" service at "/graphql-apis/graphql-paginated-keys-api/api-keys?limit=10&offset=0"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    # Cleanup
    When I delete the GraphQL API "graphql-paginated-keys-api"
    Then the response should be successful
