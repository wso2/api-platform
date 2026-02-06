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

@word-count-guardrail
Feature: Word Count Guardrail
  As an API developer
  I want to limit the word count in requests and responses
  So that I can prevent overly long or short content from being processed

  Background:
    Given the gateway services are running

  # ============================================================================
  # REQUEST VALIDATION SCENARIOS
  # ============================================================================

  Scenario: Block request when word count exceeds maximum
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: word-count-max-test-api
      spec:
        displayName: Word Count Max Test API
        version: v1.0
        context: /word-count-max/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: word-count-guardrail
                version: v0
                params:
                  request:
                    min: 1
                    max: 10
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/word-count-max/v1.0/health" to be ready

    # Request with 5 words - should pass (within limit)
    When I send a POST request to "http://localhost:8080/word-count-max/v1.0/validate" with body:
      """
      This has exactly five words
      """
    Then the response status code should be 200

    # Request with 15 words - should be blocked (exceeds max of 10)
    When I send a POST request to "http://localhost:8080/word-count-max/v1.0/validate" with body:
      """
      This is a much longer message that contains way more than ten words and should fail validation
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "word-count-max-test-api"
    Then the response should be successful

  Scenario: Block request when word count is below minimum
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: word-count-min-test-api
      spec:
        displayName: Word Count Min Test API
        version: v1.0
        context: /word-count-min/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: word-count-guardrail
                version: v0
                params:
                  request:
                    min: 5
                    max: 100
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/word-count-min/v1.0/health" to be ready

    # Request with 2 words - should be blocked (below min of 5)
    When I send a POST request to "http://localhost:8080/word-count-min/v1.0/validate" with body:
      """
      Too short
      """
    Then the response status code should be 422

    # Request with 7 words - should pass (above min)
    When I send a POST request to "http://localhost:8080/word-count-min/v1.0/validate" with body:
      """
      This message has exactly seven words total
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "word-count-min-test-api"
    Then the response should be successful

  # ============================================================================
  # JSONPATH EXTRACTION SCENARIOS
  # ============================================================================

  Scenario: Validate word count using JSONPath extraction
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: word-count-jsonpath-api
      spec:
        displayName: Word Count JSONPath API
        version: v1.0
        context: /word-count-jsonpath/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: word-count-guardrail
                version: v0
                params:
                  request:
                    min: 1
                    max: 20
                    jsonPath: "$.message"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/word-count-jsonpath/v1.0/health" to be ready

    # JSON with message field under 20 words - should pass
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/word-count-jsonpath/v1.0/chat" with body:
      """
      {
        "message": "Hello this is a short message",
        "metadata": "This field has many many many words but should be ignored by the guardrail"
      }
      """
    Then the response status code should be 200

    # JSON with message field over 20 words - should fail
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/word-count-jsonpath/v1.0/chat" with body:
      """
      {
        "message": "This is a very long message that contains way more than twenty words and should definitely fail the word count validation because it exceeds the maximum limit",
        "metadata": "short"
      }
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "word-count-jsonpath-api"
    Then the response should be successful

  # ============================================================================
  # INVERTED LOGIC SCENARIOS
  # ============================================================================

  Scenario: Inverted logic blocks content within the range
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: word-count-invert-api
      spec:
        displayName: Word Count Invert API
        version: v1.0
        context: /word-count-invert/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: word-count-guardrail
                version: v0
                params:
                  request:
                    min: 5
                    max: 10
                    invert: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/word-count-invert/v1.0/health" to be ready

    # Request with 7 words (within 5-10 range) - should FAIL because invert=true
    When I send a POST request to "http://localhost:8080/word-count-invert/v1.0/validate" with body:
      """
      This has exactly seven words here
      """
    Then the response status code should be 422

    # Request with 3 words (outside 5-10 range) - should PASS because invert=true
    When I send a POST request to "http://localhost:8080/word-count-invert/v1.0/validate" with body:
      """
      Only three words
      """
    Then the response status code should be 200

    # Request with 15 words (outside 5-10 range) - should PASS because invert=true
    When I send a POST request to "http://localhost:8080/word-count-invert/v1.0/validate" with body:
      """
      This is a much longer message that has way more than ten words so it passes
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "word-count-invert-api"
    Then the response should be successful

  # ============================================================================
  # SHOW ASSESSMENT SCENARIOS
  # ============================================================================

  Scenario: Show detailed assessment in error response
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: word-count-assessment-api
      spec:
        displayName: Word Count Assessment API
        version: v1.0
        context: /word-count-assessment/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: word-count-guardrail
                version: v0
                params:
                  request:
                    min: 1
                    max: 5
                    showAssessment: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/word-count-assessment/v1.0/health" to be ready

    # Request that fails - should include assessment details
    When I send a POST request to "http://localhost:8080/word-count-assessment/v1.0/validate" with body:
      """
      This message has way more than five words and should fail
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the response body should contain "word"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "word-count-assessment-api"
    Then the response should be successful

  # ============================================================================
  # EDGE CASES
  # ============================================================================

  Scenario: Empty request body is handled correctly
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: word-count-empty-api
      spec:
        displayName: Word Count Empty API
        version: v1.0
        context: /word-count-empty/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: word-count-guardrail
                version: v0
                params:
                  request:
                    min: 1
                    max: 100
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/word-count-empty/v1.0/health" to be ready

    # Empty body has 0 words - should fail (below min of 1)
    When I send a POST request to "http://localhost:8080/word-count-empty/v1.0/validate" with body:
      """
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "word-count-empty-api"
    Then the response should be successful

  Scenario: Exact boundary values are accepted
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: word-count-boundary-api
      spec:
        displayName: Word Count Boundary API
        version: v1.0
        context: /word-count-boundary/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: word-count-guardrail
                version: v0
                params:
                  request:
                    min: 5
                    max: 10
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/word-count-boundary/v1.0/health" to be ready

    # Exactly 5 words (min boundary) - should pass
    When I send a POST request to "http://localhost:8080/word-count-boundary/v1.0/validate" with body:
      """
      One two three four five
      """
    Then the response status code should be 200

    # Exactly 10 words (max boundary) - should pass
    When I send a POST request to "http://localhost:8080/word-count-boundary/v1.0/validate" with body:
      """
      One two three four five six seven eight nine ten
      """
    Then the response status code should be 200

    # 4 words (below min) - should fail
    When I send a POST request to "http://localhost:8080/word-count-boundary/v1.0/validate" with body:
      """
      One two three four
      """
    Then the response status code should be 422

    # 11 words (above max) - should fail
    When I send a POST request to "http://localhost:8080/word-count-boundary/v1.0/validate" with body:
      """
      One two three four five six seven eight nine ten eleven
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "word-count-boundary-api"
    Then the response should be successful

  # ============================================================================
  # RESPONSE VALIDATION SCENARIOS
  # ============================================================================

  Scenario: Combined request and response validation
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: word-count-combined-api
      spec:
        displayName: Word Count Combined API
        version: v1.0
        context: /word-count-combined/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: word-count-guardrail
                version: v0
                params:
                  request:
                    min: 1
                    max: 10
                  response:
                    min: 1
                    max: 100
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/word-count-combined/v1.0/health" to be ready

    # Request within limit - should pass request validation
    When I send a POST request to "http://localhost:8080/word-count-combined/v1.0/validate" with body:
      """
      Five words in this request
      """
    Then the response status code should be 200

    # Request exceeds limit - should fail at request phase
    When I send a POST request to "http://localhost:8080/word-count-combined/v1.0/validate" with body:
      """
      This is a much longer message that contains way more than ten words and should fail
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "word-count-combined-api"
    Then the response should be successful

  # ============================================================================
  # ADVANCED JSONPATH SCENARIOS
  # ============================================================================

  Scenario: Validate word count using nested JSONPath
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: word-count-nested-jsonpath-api
      spec:
        displayName: Word Count Nested JSONPath API
        version: v1.0
        context: /word-count-nested/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: word-count-guardrail
                version: v0
                params:
                  request:
                    min: 1
                    max: 10
                    jsonPath: "$.data.content"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/word-count-nested/v1.0/health" to be ready

    # Nested JSON with content field under 10 words - should pass
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/word-count-nested/v1.0/chat" with body:
      """
      {
        "data": {
          "content": "Short nested message here",
          "timestamp": "2025-01-01"
        },
        "metadata": "This outer field has many words but should be ignored completely"
      }
      """
    Then the response status code should be 200

    # Nested JSON with content field over 10 words - should fail
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/word-count-nested/v1.0/chat" with body:
      """
      {
        "data": {
          "content": "This nested content field has way more than ten words and should fail validation",
          "timestamp": "2025-01-01"
        }
      }
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "word-count-nested-jsonpath-api"
    Then the response should be successful

  Scenario: Handle invalid JSONPath gracefully
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: word-count-invalid-path-api
      spec:
        displayName: Word Count Invalid Path API
        version: v1.0
        context: /word-count-invalid-path/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: word-count-guardrail
                version: v0
                params:
                  request:
                    min: 1
                    max: 10
                    jsonPath: "$.nonexistent.field"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/word-count-invalid-path/v1.0/health" to be ready

    # JSON without the expected path - should return error
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/word-count-invalid-path/v1.0/validate" with body:
      """
      {
        "message": "This field exists but not the one we are looking for"
      }
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the response body should contain "WORD_COUNT_GUARDRAIL"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "word-count-invalid-path-api"
    Then the response should be successful

  # ============================================================================
  # SPECIAL CONTENT SCENARIOS
  # ============================================================================

  Scenario: Handle punctuation and special characters correctly
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: word-count-punctuation-api
      spec:
        displayName: Word Count Punctuation API
        version: v1.0
        context: /word-count-punctuation/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: word-count-guardrail
                version: v0
                params:
                  request:
                    min: 1
                    max: 5
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/word-count-punctuation/v1.0/health" to be ready

    # Punctuation should not affect word count - "Hello... world!!!" = 2 words
    When I send a POST request to "http://localhost:8080/word-count-punctuation/v1.0/validate" with body:
      """
      Hello... world!!! How are you?
      """
    Then the response status code should be 200

    # Hyphenated words count as one - "well-known" = 1 word
    When I send a POST request to "http://localhost:8080/word-count-punctuation/v1.0/validate" with body:
      """
      This is a well-known fact
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "word-count-punctuation-api"
    Then the response should be successful

  Scenario: Handle plain text content type
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: word-count-plaintext-api
      spec:
        displayName: Word Count Plain Text API
        version: v1.0
        context: /word-count-plaintext/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: word-count-guardrail
                version: v0
                params:
                  request:
                    min: 1
                    max: 10
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/word-count-plaintext/v1.0/health" to be ready

    # Plain text content - should count words correctly
    When I set header "Content-Type" to "text/plain"
    And I send a POST request to "http://localhost:8080/word-count-plaintext/v1.0/validate" with body:
      """
      This is plain text with seven words
      """
    Then the response status code should be 200

    # Plain text exceeding limit
    When I set header "Content-Type" to "text/plain"
    And I send a POST request to "http://localhost:8080/word-count-plaintext/v1.0/validate" with body:
      """
      This plain text message has way more than the allowed ten words limit
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "word-count-plaintext-api"
    Then the response should be successful

  # ============================================================================
  # ERROR RESPONSE VALIDATION
  # ============================================================================

  Scenario: Verify complete error response structure
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: word-count-error-structure-api
      spec:
        displayName: Word Count Error Structure API
        version: v1.0
        context: /word-count-error-structure/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: word-count-guardrail
                version: v0
                params:
                  request:
                    min: 1
                    max: 5
                    showAssessment: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/word-count-error-structure/v1.0/health" to be ready

    # Trigger error and verify full response structure
    When I send a POST request to "http://localhost:8080/word-count-error-structure/v1.0/validate" with body:
      """
      This message has more than five words and will fail
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the JSON response field "type" should be "WORD_COUNT_GUARDRAIL"
    And the JSON response field "message.action" should be "GUARDRAIL_INTERVENED"
    And the JSON response field "message.interveningGuardrail" should be "word-count-guardrail"
    And the JSON response field "message.direction" should be "REQUEST"
    And the response body should contain "assessments"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "word-count-error-structure-api"
    Then the response should be successful
