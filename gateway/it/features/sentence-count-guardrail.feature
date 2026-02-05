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

@sentence-count-guardrail
Feature: Sentence Count Guardrail
  As an API developer
  I want to limit the sentence count in requests and responses
  So that I can prevent overly long or short content from being processed

  Background:
    Given the gateway services are running

  # ============================================================================
  # REQUEST VALIDATION SCENARIOS
  # ============================================================================

  Scenario: Block request when sentence count exceeds maximum
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: sentence-count-max-test-api
      spec:
        displayName: Sentence Count Max Test API
        version: v1.0
        context: /sentence-count-max/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: sentence-count-guardrail
                version: v0
                params:
                  request:
                    min: 1
                    max: 3
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/sentence-count-max/v1.0/health" to be ready

    # Request with 2 sentences - should pass (within limit)
    When I send a POST request to "http://localhost:8080/sentence-count-max/v1.0/validate" with body:
      """
      Hello world. This is fine.
      """
    Then the response status code should be 200

    # Request with 5 sentences - should be blocked (exceeds max of 3)
    When I send a POST request to "http://localhost:8080/sentence-count-max/v1.0/validate" with body:
      """
      One. Two. Three. Four. Five.
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "sentence-count-max-test-api"
    Then the response should be successful

  Scenario: Block request when sentence count is below minimum
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: sentence-count-min-test-api
      spec:
        displayName: Sentence Count Min Test API
        version: v1.0
        context: /sentence-count-min/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: sentence-count-guardrail
                version: v0
                params:
                  request:
                    min: 3
                    max: 100
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/sentence-count-min/v1.0/health" to be ready

    # Request with 1 sentence - should be blocked (below min of 3)
    When I send a POST request to "http://localhost:8080/sentence-count-min/v1.0/validate" with body:
      """
      Only one sentence here.
      """
    Then the response status code should be 422

    # Request with 4 sentences - should pass (above min)
    When I send a POST request to "http://localhost:8080/sentence-count-min/v1.0/validate" with body:
      """
      First sentence. Second sentence. Third sentence. Fourth sentence.
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "sentence-count-min-test-api"
    Then the response should be successful

  # ============================================================================
  # JSONPATH EXTRACTION SCENARIOS
  # ============================================================================

  Scenario: Validate sentence count using JSONPath extraction
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: sentence-count-jsonpath-api
      spec:
        displayName: Sentence Count JSONPath API
        version: v1.0
        context: /sentence-count-jsonpath/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: sentence-count-guardrail
                version: v0
                params:
                  request:
                    min: 1
                    max: 5
                    jsonPath: "$.message"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/sentence-count-jsonpath/v1.0/health" to be ready

    # JSON with message field under 5 sentences - should pass
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/sentence-count-jsonpath/v1.0/chat" with body:
      """
      {
        "message": "Hello there. How are you?",
        "metadata": "This field has many sentences. One here. Two here. Three here. Four here. But should be ignored."
      }
      """
    Then the response status code should be 200

    # JSON with message field over 5 sentences - should fail
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/sentence-count-jsonpath/v1.0/chat" with body:
      """
      {
        "message": "First. Second. Third. Fourth. Fifth. Sixth. This exceeds the limit!",
        "metadata": "short"
      }
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "sentence-count-jsonpath-api"
    Then the response should be successful

  Scenario: Validate sentence count using nested JSONPath
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: sentence-count-nested-jsonpath-api
      spec:
        displayName: Sentence Count Nested JSONPath API
        version: v1.0
        context: /sentence-count-nested/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: sentence-count-guardrail
                version: v0
                params:
                  request:
                    min: 1
                    max: 3
                    jsonPath: "$.data.content"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/sentence-count-nested/v1.0/health" to be ready

    # Nested JSON with content field under 3 sentences - should pass
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/sentence-count-nested/v1.0/chat" with body:
      """
      {
        "data": {
          "content": "Short message. Just two sentences.",
          "timestamp": "2025-01-01"
        },
        "metadata": "This outer field has many sentences. But should be ignored. Completely ignored."
      }
      """
    Then the response status code should be 200

    # Nested JSON with content field over 3 sentences - should fail
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/sentence-count-nested/v1.0/chat" with body:
      """
      {
        "data": {
          "content": "First sentence. Second sentence. Third sentence. Fourth sentence. This exceeds!",
          "timestamp": "2025-01-01"
        }
      }
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "sentence-count-nested-jsonpath-api"
    Then the response should be successful

  Scenario: Handle invalid JSONPath gracefully
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: sentence-count-invalid-path-api
      spec:
        displayName: Sentence Count Invalid Path API
        version: v1.0
        context: /sentence-count-invalid-path/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: sentence-count-guardrail
                version: v0
                params:
                  request:
                    min: 1
                    max: 10
                    jsonPath: "$.nonexistent.field"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/sentence-count-invalid-path/v1.0/health" to be ready

    # JSON without the expected path - should return error
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/sentence-count-invalid-path/v1.0/validate" with body:
      """
      {
        "message": "This field exists. But not the one we are looking for."
      }
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the response body should contain "SENTENCE_COUNT_GUARDRAIL"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "sentence-count-invalid-path-api"
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
        name: sentence-count-invert-api
      spec:
        displayName: Sentence Count Invert API
        version: v1.0
        context: /sentence-count-invert/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: sentence-count-guardrail
                version: v0
                params:
                  request:
                    min: 2
                    max: 4
                    invert: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/sentence-count-invert/v1.0/health" to be ready

    # Request with 3 sentences (within 2-4 range) - should FAIL because invert=true
    When I send a POST request to "http://localhost:8080/sentence-count-invert/v1.0/validate" with body:
      """
      One sentence. Two sentences. Three sentences.
      """
    Then the response status code should be 422

    # Request with 1 sentence (outside 2-4 range) - should PASS because invert=true
    When I send a POST request to "http://localhost:8080/sentence-count-invert/v1.0/validate" with body:
      """
      Only one sentence here.
      """
    Then the response status code should be 200

    # Request with 6 sentences (outside 2-4 range) - should PASS because invert=true
    When I send a POST request to "http://localhost:8080/sentence-count-invert/v1.0/validate" with body:
      """
      One. Two. Three. Four. Five. Six sentences here!
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "sentence-count-invert-api"
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
        name: sentence-count-assessment-api
      spec:
        displayName: Sentence Count Assessment API
        version: v1.0
        context: /sentence-count-assessment/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: sentence-count-guardrail
                version: v0
                params:
                  request:
                    min: 1
                    max: 2
                    showAssessment: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/sentence-count-assessment/v1.0/health" to be ready

    # Request that fails - should include assessment details
    When I send a POST request to "http://localhost:8080/sentence-count-assessment/v1.0/validate" with body:
      """
      First. Second. Third. Fourth. This exceeds the max of two sentences!
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the response body should contain "sentence"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "sentence-count-assessment-api"
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
        name: sentence-count-empty-api
      spec:
        displayName: Sentence Count Empty API
        version: v1.0
        context: /sentence-count-empty/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: sentence-count-guardrail
                version: v0
                params:
                  request:
                    min: 1
                    max: 100
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/sentence-count-empty/v1.0/health" to be ready

    # Empty body has 0 sentences - should fail (below min of 1)
    When I send a POST request to "http://localhost:8080/sentence-count-empty/v1.0/validate" with body:
      """
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "sentence-count-empty-api"
    Then the response should be successful

  Scenario: Exact boundary values are accepted
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: sentence-count-boundary-api
      spec:
        displayName: Sentence Count Boundary API
        version: v1.0
        context: /sentence-count-boundary/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: sentence-count-guardrail
                version: v0
                params:
                  request:
                    min: 2
                    max: 4
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/sentence-count-boundary/v1.0/health" to be ready

    # Exactly 2 sentences (min boundary) - should pass
    When I send a POST request to "http://localhost:8080/sentence-count-boundary/v1.0/validate" with body:
      """
      First sentence. Second sentence.
      """
    Then the response status code should be 200

    # Exactly 4 sentences (max boundary) - should pass
    When I send a POST request to "http://localhost:8080/sentence-count-boundary/v1.0/validate" with body:
      """
      One. Two. Three. Four.
      """
    Then the response status code should be 200

    # 1 sentence (below min) - should fail
    When I send a POST request to "http://localhost:8080/sentence-count-boundary/v1.0/validate" with body:
      """
      Only one sentence.
      """
    Then the response status code should be 422

    # 5 sentences (above max) - should fail
    When I send a POST request to "http://localhost:8080/sentence-count-boundary/v1.0/validate" with body:
      """
      One. Two. Three. Four. Five.
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "sentence-count-boundary-api"
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
        name: sentence-count-combined-api
      spec:
        displayName: Sentence Count Combined API
        version: v1.0
        context: /sentence-count-combined/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: sentence-count-guardrail
                version: v0
                params:
                  request:
                    min: 1
                    max: 5
                  response:
                    min: 1
                    max: 100
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/sentence-count-combined/v1.0/health" to be ready

    # Request within limit - should pass request validation
    When I send a POST request to "http://localhost:8080/sentence-count-combined/v1.0/validate" with body:
      """
      Two sentences here. This should pass.
      """
    Then the response status code should be 200

    # Request exceeds limit - should fail at request phase
    When I send a POST request to "http://localhost:8080/sentence-count-combined/v1.0/validate" with body:
      """
      One. Two. Three. Four. Five. Six sentences total!
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "sentence-count-combined-api"
    Then the response should be successful

  # ============================================================================
  # SPECIAL CONTENT SCENARIOS
  # ============================================================================

  Scenario: Handle multiple punctuation marks correctly
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: sentence-count-punctuation-api
      spec:
        displayName: Sentence Count Punctuation API
        version: v1.0
        context: /sentence-count-punctuation/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: sentence-count-guardrail
                version: v0
                params:
                  request:
                    min: 1
                    max: 3
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/sentence-count-punctuation/v1.0/health" to be ready

    # Multiple exclamation marks - "Hello!!! World???" should count based on [.!?] splitting
    When I send a POST request to "http://localhost:8080/sentence-count-punctuation/v1.0/validate" with body:
      """
      Hello!!! World???
      """
    Then the response status code should be 200

    # Mixed punctuation - should be counted correctly
    When I send a POST request to "http://localhost:8080/sentence-count-punctuation/v1.0/validate" with body:
      """
      What! Really? Yes.
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "sentence-count-punctuation-api"
    Then the response should be successful

  Scenario: Handle plain text content type
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: sentence-count-plaintext-api
      spec:
        displayName: Sentence Count Plain Text API
        version: v1.0
        context: /sentence-count-plaintext/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: sentence-count-guardrail
                version: v0
                params:
                  request:
                    min: 1
                    max: 5
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/sentence-count-plaintext/v1.0/health" to be ready

    # Plain text content - should count sentences correctly
    When I set header "Content-Type" to "text/plain"
    And I send a POST request to "http://localhost:8080/sentence-count-plaintext/v1.0/validate" with body:
      """
      This is plain text. It has three sentences. All should be counted.
      """
    Then the response status code should be 200

    # Plain text exceeding limit
    When I set header "Content-Type" to "text/plain"
    And I send a POST request to "http://localhost:8080/sentence-count-plaintext/v1.0/validate" with body:
      """
      One. Two. Three. Four. Five. Six. This exceeds the limit!
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "sentence-count-plaintext-api"
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
        name: sentence-count-error-structure-api
      spec:
        displayName: Sentence Count Error Structure API
        version: v1.0
        context: /sentence-count-error-structure/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: sentence-count-guardrail
                version: v0
                params:
                  request:
                    min: 1
                    max: 2
                    showAssessment: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/sentence-count-error-structure/v1.0/health" to be ready

    # Trigger error and verify full response structure
    When I send a POST request to "http://localhost:8080/sentence-count-error-structure/v1.0/validate" with body:
      """
      First sentence. Second sentence. Third sentence. This will fail!
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the JSON response field "type" should be "SENTENCE_COUNT_GUARDRAIL"
    And the JSON response field "message.action" should be "GUARDRAIL_INTERVENED"
    And the JSON response field "message.interveningGuardrail" should be "sentence-count-guardrail"
    And the JSON response field "message.direction" should be "REQUEST"
    And the response body should contain "assessments"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "sentence-count-error-structure-api"
    Then the response should be successful
