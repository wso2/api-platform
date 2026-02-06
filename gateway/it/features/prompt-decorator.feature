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

@prompt-decorator
Feature: Prompt Decorator
  As an API developer
  I want to modify LLM prompts by adding custom instructions
  So that I can enforce consistent behavior across all LLM requests

  Background:
    Given the gateway services are running

  # ============================================================================
  # CHAT COMPLETION MODE - ARRAY DECORATION
  # ============================================================================

  Scenario: Prepend system message to chat completion messages
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-decorator-prepend-api
      spec:
        displayName: Prompt Decorator Prepend API
        version: v1.0
        context: /prompt-decorator-prepend/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-decorator
                version: v0
                params:
                  promptDecoratorConfig: '{"decoration": [{"role": "system", "content": "You are a helpful assistant."}]}'
                  jsonPath: "$.messages"
                  append: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-decorator-prepend/v1.0/health" to be ready

    # Send chat completion request
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-decorator-prepend/v1.0/chat" with body:
      """
      {
        "messages": [
          {"role": "user", "content": "Hello"}
        ]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-decorator-prepend-api"
    Then the response should be successful

  Scenario: Append system message to chat completion messages
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-decorator-append-api
      spec:
        displayName: Prompt Decorator Append API
        version: v1.0
        context: /prompt-decorator-append/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-decorator
                version: v0
                params:
                  promptDecoratorConfig: '{"decoration": [{"role": "system", "content": "Always be concise."}]}'
                  jsonPath: "$.messages"
                  append: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-decorator-append/v1.0/health" to be ready

    # Send chat completion request
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-decorator-append/v1.0/chat" with body:
      """
      {
        "messages": [
          {"role": "system", "content": "You are a translator."},
          {"role": "user", "content": "Translate to French: Hello"}
        ]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-decorator-append-api"
    Then the response should be successful

  Scenario: Add multiple decoration messages to chat
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-decorator-multi-api
      spec:
        displayName: Prompt Decorator Multi API
        version: v1.0
        context: /prompt-decorator-multi/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-decorator
                version: v0
                params:
                  promptDecoratorConfig: '{"decoration": [{"role": "system", "content": "You are an expert."}, {"role": "system", "content": "Always verify facts."}]}'
                  jsonPath: "$.messages"
                  append: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-decorator-multi/v1.0/health" to be ready

    # Send chat completion request
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-decorator-multi/v1.0/chat" with body:
      """
      {
        "messages": [
          {"role": "user", "content": "What is AI?"}
        ]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-decorator-multi-api"
    Then the response should be successful

  # ============================================================================
  # TEXT COMPLETION MODE - STRING DECORATION
  # ============================================================================

  Scenario: Prepend instruction to text prompt string
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-decorator-text-prepend-api
      spec:
        displayName: Prompt Decorator Text Prepend API
        version: v1.0
        context: /prompt-decorator-text-prepend/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /complete
            policies:
              - name: prompt-decorator
                version: v0
                params:
                  promptDecoratorConfig: '{"decoration": "Summarize the following:"}'
                  jsonPath: "$.prompt"
                  append: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-decorator-text-prepend/v1.0/health" to be ready

    # Send text completion request
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-decorator-text-prepend/v1.0/complete" with body:
      """
      {
        "prompt": "AI is artificial intelligence."
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-decorator-text-prepend-api"
    Then the response should be successful

  Scenario: Append instruction to text prompt string
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-decorator-text-append-api
      spec:
        displayName: Prompt Decorator Text Append API
        version: v1.0
        context: /prompt-decorator-text-append/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /complete
            policies:
              - name: prompt-decorator
                version: v0
                params:
                  promptDecoratorConfig: '{"decoration": "Please be brief."}'
                  jsonPath: "$.prompt"
                  append: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-decorator-text-append/v1.0/health" to be ready

    # Send text completion request
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-decorator-text-append/v1.0/complete" with body:
      """
      {
        "prompt": "Explain quantum computing."
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-decorator-text-append-api"
    Then the response should be successful

  # ============================================================================
  # LAST MESSAGE CONTENT DECORATION
  # ============================================================================

  Scenario: Decorate last message content with array decoration
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-decorator-last-msg-api
      spec:
        displayName: Prompt Decorator Last Message API
        version: v1.0
        context: /prompt-decorator-last-msg/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-decorator
                version: v0
                params:
                  promptDecoratorConfig: '{"decoration": [{"role": "system", "content": "Format your answer in JSON."}]}'
                  jsonPath: "$.messages[-1].content"
                  append: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-decorator-last-msg/v1.0/health" to be ready

    # Send chat completion request
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-decorator-last-msg/v1.0/chat" with body:
      """
      {
        "messages": [
          {"role": "system", "content": "You are helpful."},
          {"role": "user", "content": "List 3 colors"}
        ]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-decorator-last-msg-api"
    Then the response should be successful

  Scenario: Decorate last message content with string decoration
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-decorator-last-string-api
      spec:
        displayName: Prompt Decorator Last String API
        version: v1.0
        context: /prompt-decorator-last-string/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-decorator
                version: v0
                params:
                  promptDecoratorConfig: '{"decoration": "Be creative!"}'
                  jsonPath: "$.messages[-1].content"
                  append: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-decorator-last-string/v1.0/health" to be ready

    # Send chat completion request
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-decorator-last-string/v1.0/chat" with body:
      """
      {
        "messages": [
          {"role": "user", "content": "Write a poem"}
        ]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-decorator-last-string-api"
    Then the response should be successful

  # ============================================================================
  # NESTED JSONPATH SCENARIOS
  # ============================================================================

  Scenario: Decorate nested prompt field
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-decorator-nested-api
      spec:
        displayName: Prompt Decorator Nested API
        version: v1.0
        context: /prompt-decorator-nested/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /complete
            policies:
              - name: prompt-decorator
                version: v0
                params:
                  promptDecoratorConfig: '{"decoration": "Answer concisely:"}'
                  jsonPath: "$.request.prompt"
                  append: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-decorator-nested/v1.0/health" to be ready

    # Send nested structure request
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-decorator-nested/v1.0/complete" with body:
      """
      {
        "request": {
          "prompt": "What is machine learning?",
          "temperature": 0.7
        }
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-decorator-nested-api"
    Then the response should be successful

  # ============================================================================
  # EDGE CASES
  # ============================================================================

  Scenario: Handle empty request body
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-decorator-empty-api
      spec:
        displayName: Prompt Decorator Empty API
        version: v1.0
        context: /prompt-decorator-empty/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-decorator
                version: v0
                params:
                  promptDecoratorConfig: '{"decoration": [{"role": "system", "content": "Test"}]}'
                  jsonPath: "$.messages"
                  append: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-decorator-empty/v1.0/health" to be ready

    # Send empty body - should return error
    When I send a POST request to "http://localhost:8080/prompt-decorator-empty/v1.0/chat" with body:
      """
      """
    Then the response status code should be 500
    And the response should be valid JSON
    And the response body should contain "PROMPT_DECORATOR_ERROR"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-decorator-empty-api"
    Then the response should be successful

  Scenario: Handle invalid JSONPath
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-decorator-invalid-path-api
      spec:
        displayName: Prompt Decorator Invalid Path API
        version: v1.0
        context: /prompt-decorator-invalid-path/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-decorator
                version: v0
                params:
                  promptDecoratorConfig: '{"decoration": [{"role": "system", "content": "Test"}]}'
                  jsonPath: "$.nonexistent.field"
                  append: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-decorator-invalid-path/v1.0/health" to be ready

    # Send request without the expected field
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-decorator-invalid-path/v1.0/chat" with body:
      """
      {
        "messages": [
          {"role": "user", "content": "Hello"}
        ]
      }
      """
    Then the response status code should be 500
    And the response should be valid JSON
    And the response body should contain "PROMPT_DECORATOR_ERROR"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-decorator-invalid-path-api"
    Then the response should be successful

  Scenario: Handle invalid JSON payload
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-decorator-invalid-json-api
      spec:
        displayName: Prompt Decorator Invalid JSON API
        version: v1.0
        context: /prompt-decorator-invalid-json/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-decorator
                version: v0
                params:
                  promptDecoratorConfig: '{"decoration": [{"role": "system", "content": "Test"}]}'
                  jsonPath: "$.messages"
                  append: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-decorator-invalid-json/v1.0/health" to be ready

    # Send invalid JSON
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-decorator-invalid-json/v1.0/chat" with body:
      """
      {invalid json}
      """
    Then the response status code should be 500
    And the response should be valid JSON
    And the response body should contain "PROMPT_DECORATOR_ERROR"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-decorator-invalid-json-api"
    Then the response should be successful

  # ============================================================================
  # REAL-WORLD SCENARIOS
  # ============================================================================

  Scenario: Add safety instructions to all requests
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-decorator-safety-api
      spec:
        displayName: Prompt Decorator Safety API
        version: v1.0
        context: /prompt-decorator-safety/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-decorator
                version: v0
                params:
                  promptDecoratorConfig: '{"decoration": [{"role": "system", "content": "Never provide harmful, illegal, or unethical advice. Always prioritize user safety."}]}'
                  jsonPath: "$.messages"
                  append: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-decorator-safety/v1.0/health" to be ready

    # Send chat request
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-decorator-safety/v1.0/chat" with body:
      """
      {
        "messages": [
          {"role": "user", "content": "How do I bake a cake?"}
        ]
      }
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-decorator-safety-api"
    Then the response should be successful

  Scenario: Enforce JSON output format
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-decorator-json-format-api
      spec:
        displayName: Prompt Decorator JSON Format API
        version: v1.0
        context: /prompt-decorator-json-format/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-decorator
                version: v0
                params:
                  promptDecoratorConfig: '{"decoration": [{"role": "system", "content": "Always respond in valid JSON format."}]}'
                  jsonPath: "$.messages"
                  append: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-decorator-json-format/v1.0/health" to be ready

    # Send chat request
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-decorator-json-format/v1.0/chat" with body:
      """
      {
        "messages": [
          {"role": "user", "content": "List 3 programming languages"}
        ]
      }
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-decorator-json-format-api"
    Then the response should be successful

  Scenario: Add context to existing conversation
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-decorator-context-api
      spec:
        displayName: Prompt Decorator Context API
        version: v1.0
        context: /prompt-decorator-context/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: prompt-decorator
                version: v0
                params:
                  promptDecoratorConfig: '{"decoration": [{"role": "system", "content": "The user is a software developer with 5 years of experience."}]}'
                  jsonPath: "$.messages"
                  append: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-decorator-context/v1.0/health" to be ready

    # Send chat request with existing conversation
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-decorator-context/v1.0/chat" with body:
      """
      {
        "messages": [
          {"role": "system", "content": "You are a coding tutor."},
          {"role": "user", "content": "Explain async/await"},
          {"role": "assistant", "content": "Async/await is a pattern..."},
          {"role": "user", "content": "Can you give an example?"}
        ]
      }
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-decorator-context-api"
    Then the response should be successful
