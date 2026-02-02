# --------------------------------------------------------------------
# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

Feature: API Key Management Operations
  As an API administrator
  I want to manage API keys for APIs
  So that I can control access through API key authentication

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ==================== API KEY LIFECYCLE - SUCCESS PATH ====================

  Scenario: Complete API key lifecycle - generate, list, regenerate, and revoke
    # First create an API
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: apikey-lifecycle-api
      spec:
        displayName: APIKey-Lifecycle-API
        version: v1.0
        context: /apikey-lifecycle
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /data
      """
    Then the response should be successful
    
    # Generate API key
    When I send a POST request to the "gateway-controller" service at "/apis/apikey-lifecycle-api/api-keys" with body:
      """
      {
        "name": "test-key-1"
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response should have field "api_key"
    And the JSON response should have field "api_key.name"
    And the JSON response should have field "api_key.api_key"
    
    # List API keys - should have 1 key
    When I send a GET request to the "gateway-controller" service at "/apis/apikey-lifecycle-api/api-keys"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "test-key-1"
    
    # Regenerate API key
    When I send a POST request to the "gateway-controller" service at "/apis/apikey-lifecycle-api/api-keys/test-key-1/regenerate" with body:
      """
      {}
      """
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response should have field "api_key.api_key"
    
    # Revoke API key
    When I send a DELETE request to the "gateway-controller" service at "/apis/apikey-lifecycle-api/api-keys/test-key-1"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    
    # Verify key is revoked - list should be empty
    When I send a GET request to the "gateway-controller" service at "/apis/apikey-lifecycle-api/api-keys"
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should not contain "test-key-1"
    
    # Cleanup
    When I delete the API "apikey-lifecycle-api"
    Then the response should be successful

  Scenario: Generate multiple API keys for same API
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: multi-key-api
      spec:
        displayName: Multi-Key-API
        version: v1.0
        context: /multi-key
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /data
      """
    Then the response should be successful
    
    # Generate first key
    When I send a POST request to the "gateway-controller" service at "/apis/multi-key-api/api-keys" with body:
      """
      {
        "name": "key-one"
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    
    # Generate second key
    When I send a POST request to the "gateway-controller" service at "/apis/multi-key-api/api-keys" with body:
      """
      {
        "name": "key-two"
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    
    # List should show both keys
    When I send a GET request to the "gateway-controller" service at "/apis/multi-key-api/api-keys"
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should contain "key-one"
    And the response body should contain "key-two"
    
    # Cleanup
    When I delete the API "multi-key-api"
    Then the response should be successful

  Scenario: List API keys for API with no keys returns empty list
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: no-keys-api
      spec:
        displayName: No-Keys-API
        version: v1.0
        context: /no-keys
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /data
      """
    Then the response should be successful
    When I send a GET request to the "gateway-controller" service at "/apis/no-keys-api/api-keys"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    # Cleanup
    When I delete the API "no-keys-api"
    Then the response should be successful

  # ==================== GENERATE API KEY - ERROR CASES ====================
  
  Scenario: Generate API key for non-existent API returns 404
    When I send a POST request to the "gateway-controller" service at "/apis/non-existent-api-id/api-keys" with body:
      """
      {
        "name": "test-key"
      }
      """
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Generate API key without name auto-generates name
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: key-validation-api
      spec:
        displayName: Key-Validation-API
        version: v1.0
        context: /key-validation
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /data
      """
    Then the response should be successful
    When I send a POST request to the "gateway-controller" service at "/apis/key-validation-api/api-keys" with body:
      """
      {}
      """
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response should have field "api_key"
    # Cleanup
    When I delete the API "key-validation-api"
    Then the response should be successful

  # ==================== LIST API KEYS - ERROR CASES ====================
  
  Scenario: List API keys for non-existent API returns 404
    When I send a GET request to the "gateway-controller" service at "/apis/non-existent-api-id/api-keys"
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: List API keys with invalid API ID format returns 404
    When I send a GET request to the "gateway-controller" service at "/apis/invalid@api#id/api-keys"
    Then the response status should be 404
    And the response should be valid JSON

  # ==================== REVOKE API KEY - ERROR CASES ====================
  
  Scenario: Revoke API key with invalid formats returns 404
    When I send a DELETE request to the "gateway-controller" service at "/apis/invalid@api/api-keys/invalid@key"
    Then the response status should be 404
    And the response should be valid JSON

  Scenario: Revoke non-existent API key returns success (idempotent)
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: revoke-error-api
      spec:
        displayName: Revoke-Error-API
        version: v1.0
        context: /revoke-error
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /data
      """
    Then the response should be successful
    # Revoking non-existent key is idempotent - returns success
    When I send a DELETE request to the "gateway-controller" service at "/apis/revoke-error-api/api-keys/non-existent-key"
    Then the response status should be 200
    And the response should be valid JSON
    # Cleanup
    When I delete the API "revoke-error-api"
    Then the response should be successful

  # ==================== REGENERATE API KEY - ERROR CASES ====================
  
  Scenario: Regenerate API key for non-existent API returns 404
    When I send a POST request to the "gateway-controller" service at "/apis/non-existent-api-id/api-keys/test-key/regenerate" with body:
      """
      {}
      """
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Regenerate non-existent API key returns 404
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-regenerate-api
      spec:
        displayName: Test-Regenerate-Api
        version: v1.0
        context: /test-regen
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
      """
    Then the response should be successful
    When I send a POST request to the "gateway-controller" service at "/apis/test-regenerate-api/api-keys/non-existent-key/regenerate" with body:
      """
      {}
      """
    Then the response status should be 404
    # Cleanup
    When I delete the API "test-regenerate-api"
    Then the response should be successful

  Scenario: Regenerate API key with invalid ID formats returns 404
    When I send a POST request to the "gateway-controller" service at "/apis/invalid@api/api-keys/invalid@key/regenerate" with body:
      """
      {}
      """
    Then the response status should be 404
    And the response should be valid JSON

  # ==================== GENERATE API KEY - ADDITIONAL ERROR CASES ====================

  Scenario: Generate API key with invalid JSON body returns error
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: invalid-json-key-api
      spec:
        displayName: Invalid-JSON-Key-API
        version: v1.0
        context: /invalid-json-key
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /data
      """
    Then the response should be successful
    When I send a POST request to the "gateway-controller" service at "/apis/invalid-json-key-api/api-keys" with body:
      """
      { this is not valid json
      """
    Then the response should be a client error
    And the response should be valid JSON
    # Cleanup
    When I delete the API "invalid-json-key-api"
    Then the response should be successful

  Scenario: API key with special characters in name
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: special-char-key-api
      spec:
        displayName: Special-Char-Key-API
        version: v1.0
        context: /special-char-key
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /data
      """
    Then the response should be successful
    # Generate key with hyphens and underscores (should be allowed)
    When I send a POST request to the "gateway-controller" service at "/apis/special-char-key-api/api-keys" with body:
      """
      {
        "name": "my-api-key_v1"
      }
      """
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    # Cleanup
    When I delete the API "special-char-key-api"
    Then the response should be successful

  # ==================== LIST API KEYS WITH PAGINATION ====================

  Scenario: List API keys with pagination parameters
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: paginated-keys-api
      spec:
        displayName: Paginated-Keys-API
        version: v1.0
        context: /paginated-keys
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /data
      """
    Then the response should be successful
    When I send a GET request to the "gateway-controller" service at "/apis/paginated-keys-api/api-keys?limit=10&offset=0"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    # Cleanup
    When I delete the API "paginated-keys-api"
    Then the response should be successful
