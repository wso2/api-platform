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

Feature: LLM Proxy Management Operations
  As an API administrator
  I want to manage LLM proxies via REST API handlers
  So that I can create, read, update, delete, and list LLM proxies

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ==================== LIST LLM PROXIES ====================
  
  Scenario: List all LLM proxies when none exist
    When I send a GET request to the "gateway-controller" service at "/llm-proxies"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: List LLM proxies with pagination parameters
    When I send a GET request to the "gateway-controller" service at "/llm-proxies?limit=10&offset=0"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: List LLM proxies with different limit values
    When I send a GET request to the "gateway-controller" service at "/llm-proxies?limit=5"
    Then the response should be successful
    And the response should be valid JSON

  Scenario: List LLM proxies with offset only
    When I send a GET request to the "gateway-controller" service at "/llm-proxies?offset=10"
    Then the response should be successful
    And the response should be valid JSON

  # ==================== GET LLM PROXY BY ID ====================
  
  Scenario: Get LLM proxy by non-existent ID returns 404
    When I send a GET request to the "gateway-controller" service at "/llm-proxies/non-existent-proxy-id-12345"
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Get LLM proxy with invalid ID format returns 404
    When I send a GET request to the "gateway-controller" service at "/llm-proxies/invalid@proxy#id$format"
    Then the response status should be 404
    And the response should be valid JSON

  # ==================== DELETE LLM PROXY ====================
  
  Scenario: Delete non-existent LLM proxy returns 404
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/non-existent-proxy-delete-123"
    Then the response status should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Delete LLM proxy with invalid ID format returns 404
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/invalid-delete@id"
    Then the response status should be 404
    And the response should be valid JSON

  # ==================== UPDATE LLM PROXY ====================

  Scenario: Update non-existent LLM proxy returns 400
    When I send a PUT request to the "gateway-controller" service at "/llm-proxies/non-existent-proxy-update" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "LlmProxy",
        "metadata": {
          "name": "non-existent-proxy-update"
        },
        "spec": {
          "displayName": "Test",
          "version": "v1.0",
          "context": "/test"
        }
      }
      """
    Then the response status should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  # ==================== CREATE LLM PROXY - VALIDATION ====================

  Scenario: Create LLM proxy with missing required fields returns error
    When I send a POST request to the "gateway-controller" service at "/llm-proxies" with body:
      """
      {
        "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
        "kind": "LlmProxy",
        "metadata": {
          "name": "invalid-proxy"
        },
        "spec": {
          "displayName": "Invalid Proxy"
        }
      }
      """
    Then the response should be a client error
    And the response should be valid JSON

  # ==================== COMPLETE LLM PROXY LIFECYCLE ====================

  Scenario: Complete LLM proxy lifecycle - create, get, update, and delete
    # First, create the LLM provider that the proxy will reference
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: lifecycle-test-provider
      spec:
        displayName: Lifecycle Test Provider
        version: v1.0
        template: openai
        upstream:
          url: https://mock-openapi-https:9443/openai/v1
          auth:
            type: api-key
            header: Authorization
            value: Bearer sk-test-key
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    # Create LLM proxy referencing the provider
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProxy
      metadata:
        name: lifecycle-llm-proxy
      spec:
        displayName: Lifecycle LLM Proxy
        version: v1.0
        provider: lifecycle-test-provider
      """
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    # Get the LLM proxy
    When I send a GET request to the "gateway-controller" service at "/llm-proxies/lifecycle-llm-proxy"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "Lifecycle LLM Proxy"
    # Update the LLM proxy
    When I update the LLM proxy "lifecycle-llm-proxy" with:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProxy
      metadata:
        name: lifecycle-llm-proxy
      spec:
        displayName: Updated Lifecycle LLM Proxy
        version: v1.1
        provider: lifecycle-test-provider
      """
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    # Verify update
    When I send a GET request to the "gateway-controller" service at "/llm-proxies/lifecycle-llm-proxy"
    Then the response should be successful
    And the response body should contain "Updated Lifecycle LLM Proxy"
    # Delete the LLM proxy
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/lifecycle-llm-proxy"
    Then the response should be successful
    And the JSON response field "status" should be "success"
    # Verify deletion
    When I send a GET request to the "gateway-controller" service at "/llm-proxies/lifecycle-llm-proxy"
    Then the response status should be 404
    # Cleanup: delete the provider
    When I delete the LLM provider "lifecycle-test-provider"
    Then the response status code should be 200

  Scenario: List LLM proxies after creating one
    # First, create the LLM provider
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: list-test-provider
      spec:
        displayName: List Test Provider
        version: v1.0
        template: openai
        upstream:
          url: https://mock-openapi-https:9443/openai/v1
          auth:
            type: api-key
            header: Authorization
            value: Bearer sk-test-key
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    # Create LLM proxy
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProxy
      metadata:
        name: list-test-llm-proxy
      spec:
        displayName: List Test LLM Proxy
        version: v1.0
        provider: list-test-provider
      """
    Then the response status should be 201
    # List LLM proxies
    When I send a GET request to the "gateway-controller" service at "/llm-proxies"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "list-test-llm-proxy"
    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/list-test-llm-proxy"
    Then the response should be successful
    When I delete the LLM provider "list-test-provider"
    Then the response status code should be 200

