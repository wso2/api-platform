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

  # ==================== GENERATE API KEY ====================
  
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

  # ==================== LIST API KEYS ====================
  
  Scenario: List API keys for non-existent API returns 404
    When I send a GET request to the "gateway-controller" service at "/apis/non-existent-api-id/api-keys"
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: List API keys with invalid API ID format returns 404
    When I send a GET request to the "gateway-controller" service at "/apis/invalid@api#id/api-keys"
    Then the response status should be 404
    And the response should be valid JSON

  # ==================== REVOKE API KEY ====================
  
  Scenario: Revoke API key with invalid formats returns 404
    When I send a DELETE request to the "gateway-controller" service at "/apis/invalid@api/api-keys/invalid@key"
    Then the response status should be 404
    And the response should be valid JSON

  # ==================== REGENERATE API KEY ====================
  
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
