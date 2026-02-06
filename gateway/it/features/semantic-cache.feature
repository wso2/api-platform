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

@semantic-cache
Feature: Semantic Cache Policy
  As an API developer
  I want to cache LLM responses based on semantic similarity
  So that I can reduce latency and costs for similar requests

  Background:
    Given the gateway services are running

  # Category 1: Basic Cache Behavior

  Scenario: Cache miss - first request goes to upstream
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: semantic-cache-miss-api
      spec:
        displayName: Semantic Cache - Miss Test
        version: v1.0
        context: /semantic-cache-miss/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-cache
                version: v0
                params:
                  similarityThreshold: 0.9
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/semantic-cache-miss/v1.0/health" to be ready

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/semantic-cache-miss/v1.0/chat" with body:
      """
      {"prompt": "What is the capital of France?"}
      """
    Then the response status code should be 200
    And the response header "X-Cache-Status" should not exist

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "semantic-cache-miss-api"
    Then the response should be successful

  Scenario: Cache hit - identical request returns cached response
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: semantic-cache-hit-api
      spec:
        displayName: Semantic Cache - Hit Test
        version: v1.0
        context: /semantic-cache-hit/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-cache
                version: v0
                params:
                  similarityThreshold: 0.9
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/semantic-cache-hit/v1.0/health" to be ready

    # First request - cache miss
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/semantic-cache-hit/v1.0/chat" with body:
      """
      {"prompt": "What is the capital of Germany?"}
      """
    Then the response status code should be 200

    # Second identical request - should be cache hit
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/semantic-cache-hit/v1.0/chat" with body:
      """
      {"prompt": "What is the capital of Germany?"}
      """
    Then the response status code should be 200
    And the response header "X-Cache-Status" should be "HIT"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "semantic-cache-hit-api"
    Then the response should be successful

  Scenario: Semantically similar request returns cached response
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: semantic-cache-similar-api
      spec:
        displayName: Semantic Cache - Similar Test
        version: v1.0
        context: /semantic-cache-similar/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-cache
                version: v0
                params:
                  similarityThreshold: 0.8
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/semantic-cache-similar/v1.0/health" to be ready

    # First request
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/semantic-cache-similar/v1.0/chat" with body:
      """
      {"prompt": "capital of italy"}
      """
    Then the response status code should be 200

    # Similar request (case change only - should match due to normalization)
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/semantic-cache-similar/v1.0/chat" with body:
      """
      {"prompt": "Capital of Italy"}
      """
    Then the response status code should be 200
    And the response header "X-Cache-Status" should be "HIT"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "semantic-cache-similar-api"
    Then the response should be successful

  # Category 2: Threshold Behavior

  Scenario: High threshold requires near-exact matches
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: semantic-cache-high-threshold-api
      spec:
        displayName: Semantic Cache - High Threshold
        version: v1.0
        context: /semantic-cache-high-threshold/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-cache
                version: v0
                params:
                  similarityThreshold: 0.99
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/semantic-cache-high-threshold/v1.0/health" to be ready

    # First request
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/semantic-cache-high-threshold/v1.0/chat" with body:
      """
      {"prompt": "Hello world"}
      """
    Then the response status code should be 200

    # Different request - should NOT hit cache with high threshold
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/semantic-cache-high-threshold/v1.0/chat" with body:
      """
      {"prompt": "Hello everyone"}
      """
    Then the response status code should be 200
    And the response header "X-Cache-Status" should not exist

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "semantic-cache-high-threshold-api"
    Then the response should be successful

  Scenario: Low threshold matches broader range
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: semantic-cache-low-threshold-api
      spec:
        displayName: Semantic Cache - Low Threshold
        version: v1.0
        context: /semantic-cache-low-threshold/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-cache
                version: v0
                params:
                  similarityThreshold: 0.5
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/semantic-cache-low-threshold/v1.0/health" to be ready

    # First request
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/semantic-cache-low-threshold/v1.0/chat" with body:
      """
      {"prompt": "greetings friend"}
      """
    Then the response status code should be 200

    # Exact same request with low threshold - should hit cache
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/semantic-cache-low-threshold/v1.0/chat" with body:
      """
      {"prompt": "greetings friend"}
      """
    Then the response status code should be 200
    And the response header "X-Cache-Status" should be "HIT"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "semantic-cache-low-threshold-api"
    Then the response should be successful

  # Category 3: JSONPath Extraction

  Scenario: JSONPath extracts specific field for embedding
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: semantic-cache-jsonpath-api
      spec:
        displayName: Semantic Cache - JSONPath
        version: v1.0
        context: /semantic-cache-jsonpath/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-cache
                version: v0
                params:
                  similarityThreshold: 0.9
                  jsonPath: "$.messages[0].content"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/semantic-cache-jsonpath/v1.0/health" to be ready

    # First request
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/semantic-cache-jsonpath/v1.0/chat" with body:
      """
      {
        "messages": [{"role": "user", "content": "Hello AI assistant"}],
        "metadata": "request-1"
      }
      """
    Then the response status code should be 200

    # Second request with same content field but different metadata - should hit cache
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/semantic-cache-jsonpath/v1.0/chat" with body:
      """
      {
        "messages": [{"role": "user", "content": "Hello AI assistant"}],
        "metadata": "request-2-different"
      }
      """
    Then the response status code should be 200
    And the response header "X-Cache-Status" should be "HIT"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "semantic-cache-jsonpath-api"
    Then the response should be successful

  Scenario: Invalid JSONPath returns error
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: semantic-cache-invalid-jsonpath-api
      spec:
        displayName: Semantic Cache - Invalid JSONPath
        version: v1.0
        context: /semantic-cache-invalid-jsonpath/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-cache
                version: v0
                params:
                  similarityThreshold: 0.9
                  jsonPath: "$.nonexistent.field"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/semantic-cache-invalid-jsonpath/v1.0/health" to be ready

    # Request with JSON that doesn't match the JSONPath
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/semantic-cache-invalid-jsonpath/v1.0/chat" with body:
      """
      {"message": "This field exists but not the expected path"}
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the response body should contain "SEMANTIC_CACHE"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "semantic-cache-invalid-jsonpath-api"
    Then the response should be successful

  # Category 4: Edge Cases

  Scenario: Empty request body is handled gracefully
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: semantic-cache-empty-body-api
      spec:
        displayName: Semantic Cache - Empty Body
        version: v1.0
        context: /semantic-cache-empty-body/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-cache
                version: v0
                params:
                  similarityThreshold: 0.9
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/semantic-cache-empty-body/v1.0/health" to be ready

    # Empty body - should pass through to upstream without error
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/semantic-cache-empty-body/v1.0/chat" with body:
      """
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "semantic-cache-empty-body-api"
    Then the response should be successful

  Scenario: Non-200 responses are not cached
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: semantic-cache-non-200-api
      spec:
        displayName: Semantic Cache - Non-200 Response
        version: v1.0
        context: /semantic-cache-non-200/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /get
          - method: POST
            path: /status/500
            policies:
              - name: semantic-cache
                version: v0
                params:
                  similarityThreshold: 0.9
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/semantic-cache-non-200/v1.0/get" to be ready

    # First request to status/500 endpoint - should get 500
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/semantic-cache-non-200/v1.0/status/500" with body:
      """
      {"prompt": "unique-non-200-test-prompt-67890"}
      """
    Then the response status code should be 500

    # Second identical request should NOT return cache hit (non-200 responses are not cached)
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/semantic-cache-non-200/v1.0/status/500" with body:
      """
      {"prompt": "unique-non-200-test-prompt-67890"}
      """
    Then the response status code should be 500
    And the response header "X-Cache-Status" should not exist

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "semantic-cache-non-200-api"
    Then the response should be successful

  Scenario: Embedding generation failure allows request to proceed
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: semantic-cache-embed-error-api
      spec:
        displayName: Semantic Cache - Embedding Error
        version: v1.0
        context: /semantic-cache-embed-error/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-cache
                version: v0
                params:
                  similarityThreshold: 0.9
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/semantic-cache-embed-error/v1.0/health" to be ready

    # Request with keyword that triggers error in mock embedding provider
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/semantic-cache-embed-error/v1.0/chat" with body:
      """
      {"prompt": "error simulate embedding failure"}
      """
    # Should still get response from upstream despite embedding error
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "semantic-cache-embed-error-api"
    Then the response should be successful
