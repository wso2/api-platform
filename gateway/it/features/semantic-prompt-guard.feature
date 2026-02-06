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

@semantic-prompt-guard
Feature: Semantic Prompt Guard Policy
  As an API developer
  I want to block or allow prompts based on semantic similarity
  So that I can protect my LLM from harmful or off-topic requests

  Background:
    Given the gateway services are running

  # ==========================================================================
  # Category 1: Denied Phrases Only (Blocklist Mode)
  # ==========================================================================

  Scenario: Request blocked - prompt matches denied phrase
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-guard-deny-block-api
      spec:
        displayName: Prompt Guard - Deny Block Test
        version: v1.0
        context: /prompt-guard-deny-block/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-prompt-guard
                version: v0
                params:
                  jsonPath: "$.prompt"
                  deniedPhrases:
                    - "hack the system"
                    - "bypass security"
                  denySimilarityThreshold: 0.9
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-guard-deny-block/v1.0/health" to be ready

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-guard-deny-block/v1.0/chat" with body:
      """
      {"prompt": "hack the system"}
      """
    Then the response status code should be 422
    And the response body should contain "SEMANTIC_PROMPT_GUARD"
    And the response body should contain "GUARDRAIL_INTERVENED"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-guard-deny-block-api"
    Then the response should be successful

  Scenario: Request allowed - prompt doesn't match denied phrases
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-guard-deny-allow-api
      spec:
        displayName: Prompt Guard - Deny Allow Test
        version: v1.0
        context: /prompt-guard-deny-allow/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-prompt-guard
                version: v0
                params:
                  jsonPath: "$.prompt"
                  deniedPhrases:
                    - "hack the system"
                  denySimilarityThreshold: 0.9
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-guard-deny-allow/v1.0/health" to be ready

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-guard-deny-allow/v1.0/chat" with body:
      """
      {"prompt": "what is the weather today"}
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-guard-deny-allow-api"
    Then the response should be successful

  # ==========================================================================
  # Category 2: Allowed Phrases Only (Allowlist Mode)
  # ==========================================================================

  Scenario: Request allowed - prompt matches allowed phrase
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-guard-allow-match-api
      spec:
        displayName: Prompt Guard - Allow Match Test
        version: v1.0
        context: /prompt-guard-allow-match/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-prompt-guard
                version: v0
                params:
                  jsonPath: "$.prompt"
                  allowedPhrases:
                    - "customer support"
                    - "product inquiry"
                  allowSimilarityThreshold: 0.9
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-guard-allow-match/v1.0/health" to be ready

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-guard-allow-match/v1.0/chat" with body:
      """
      {"prompt": "customer support"}
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-guard-allow-match-api"
    Then the response should be successful

  Scenario: Request blocked - prompt doesn't match allowed phrases
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-guard-allow-block-api
      spec:
        displayName: Prompt Guard - Allow Block Test
        version: v1.0
        context: /prompt-guard-allow-block/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-prompt-guard
                version: v0
                params:
                  jsonPath: "$.prompt"
                  allowedPhrases:
                    - "customer support"
                  allowSimilarityThreshold: 0.9
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-guard-allow-block/v1.0/health" to be ready

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-guard-allow-block/v1.0/chat" with body:
      """
      {"prompt": "hack the system"}
      """
    Then the response status code should be 422
    And the response body should contain "SEMANTIC_PROMPT_GUARD"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-guard-allow-block-api"
    Then the response should be successful

  # ==========================================================================
  # Category 3: Both Allowed and Denied Phrases
  # ==========================================================================

  Scenario: Denied takes priority - blocked even if could match allowed
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-guard-both-deny-api
      spec:
        displayName: Prompt Guard - Both Deny Priority Test
        version: v1.0
        context: /prompt-guard-both-deny/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-prompt-guard
                version: v0
                params:
                  jsonPath: "$.prompt"
                  allowedPhrases:
                    - "general questions"
                  deniedPhrases:
                    - "hack"
                  allowSimilarityThreshold: 0.5
                  denySimilarityThreshold: 0.9
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-guard-both-deny/v1.0/health" to be ready

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-guard-both-deny/v1.0/chat" with body:
      """
      {"prompt": "hack"}
      """
    Then the response status code should be 422
    And the response body should contain "SEMANTIC_PROMPT_GUARD"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-guard-both-deny-api"
    Then the response should be successful

  Scenario: Request allowed - matches allowed and doesn't match denied
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-guard-both-allow-api
      spec:
        displayName: Prompt Guard - Both Allow Test
        version: v1.0
        context: /prompt-guard-both-allow/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-prompt-guard
                version: v0
                params:
                  jsonPath: "$.prompt"
                  allowedPhrases:
                    - "customer support question"
                  deniedPhrases:
                    - "hack the system"
                  allowSimilarityThreshold: 0.9
                  denySimilarityThreshold: 0.9
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-guard-both-allow/v1.0/health" to be ready

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-guard-both-allow/v1.0/chat" with body:
      """
      {"prompt": "customer support question"}
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-guard-both-allow-api"
    Then the response should be successful

  # ==========================================================================
  # Category 4: Threshold Behavior
  # ==========================================================================

  Scenario: High allow threshold requires strict matching
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-guard-high-threshold-api
      spec:
        displayName: Prompt Guard - High Threshold Test
        version: v1.0
        context: /prompt-guard-high-threshold/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-prompt-guard
                version: v0
                params:
                  jsonPath: "$.prompt"
                  allowedPhrases:
                    - "hello world"
                  allowSimilarityThreshold: 0.99
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-guard-high-threshold/v1.0/health" to be ready

    # Exact match should pass
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-guard-high-threshold/v1.0/chat" with body:
      """
      {"prompt": "hello world"}
      """
    Then the response status code should be 200

    # Different prompt should fail with high threshold
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-guard-high-threshold/v1.0/chat" with body:
      """
      {"prompt": "hi there world"}
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-guard-high-threshold-api"
    Then the response should be successful

  Scenario: Low deny threshold provides broad blocking
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-guard-low-threshold-api
      spec:
        displayName: Prompt Guard - Low Threshold Test
        version: v1.0
        context: /prompt-guard-low-threshold/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-prompt-guard
                version: v0
                params:
                  jsonPath: "$.prompt"
                  deniedPhrases:
                    - "malicious attack"
                  denySimilarityThreshold: 0.5
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-guard-low-threshold/v1.0/health" to be ready

    # "malicious attack 1" is similar to "malicious attack" and should be blocked with low threshold
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-guard-low-threshold/v1.0/chat" with body:
      """
      {"prompt": "malicious attack 1"}
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-guard-low-threshold-api"
    Then the response should be successful

  Scenario: High deny threshold requires near-exact match
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-guard-high-deny-threshold-api
      spec:
        displayName: Prompt Guard - High Deny Threshold Test
        version: v1.0
        context: /prompt-guard-high-deny-threshold/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-prompt-guard
                version: v0
                params:
                  jsonPath: "$.prompt"
                  deniedPhrases:
                    - "malicious attack"
                  denySimilarityThreshold: 0.99
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-guard-high-deny-threshold/v1.0/health" to be ready

    # "malicious attack 1" is similar but not exact - should NOT be blocked with 0.99 threshold
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-guard-high-deny-threshold/v1.0/chat" with body:
      """
      {"prompt": "malicious attack 1"}
      """
    Then the response status code should be 200

    # Exact match should still be blocked
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-guard-high-deny-threshold/v1.0/chat" with body:
      """
      {"prompt": "malicious attack"}
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-guard-high-deny-threshold-api"
    Then the response should be successful

  # ==========================================================================
  # Category 5: JSONPath Extraction
  # ==========================================================================

  Scenario: JSONPath extracts specific field for validation
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-guard-jsonpath-api
      spec:
        displayName: Prompt Guard - JSONPath Test
        version: v1.0
        context: /prompt-guard-jsonpath/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-prompt-guard
                version: v0
                params:
                  jsonPath: "$.messages[0].content"
                  deniedPhrases:
                    - "hack"
                  denySimilarityThreshold: 0.9
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-guard-jsonpath/v1.0/health" to be ready

    # "hack" in system field should be ignored - only messages[0].content is validated
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-guard-jsonpath/v1.0/chat" with body:
      """
      {"messages": [{"role": "user", "content": "normal request"}], "system": "hack the server"}
      """
    Then the response status code should be 200

    # "hack" in messages[0].content should be blocked
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-guard-jsonpath/v1.0/chat" with body:
      """
      {"messages": [{"role": "user", "content": "hack"}], "system": "safe content"}
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-guard-jsonpath-api"
    Then the response should be successful

  Scenario: Invalid JSONPath returns error
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-guard-jsonpath-invalid-api
      spec:
        displayName: Prompt Guard - Invalid JSONPath Test
        version: v1.0
        context: /prompt-guard-jsonpath-invalid/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-prompt-guard
                version: v0
                params:
                  jsonPath: "$.nonexistent.field"
                  deniedPhrases:
                    - "test"
                  denySimilarityThreshold: 0.9
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-guard-jsonpath-invalid/v1.0/health" to be ready

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-guard-jsonpath-invalid/v1.0/chat" with body:
      """
      {"message": "hello world"}
      """
    Then the response status code should be 422
    And the response body should contain "SEMANTIC_PROMPT_GUARD"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-guard-jsonpath-invalid-api"
    Then the response should be successful

  # ==========================================================================
  # Category 6: showAssessment Parameter
  # ==========================================================================

  Scenario: showAssessment true includes detailed similarity info
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-guard-assessment-true-api
      spec:
        displayName: Prompt Guard - Assessment True Test
        version: v1.0
        context: /prompt-guard-assessment-true/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-prompt-guard
                version: v0
                params:
                  jsonPath: "$.prompt"
                  showAssessment: true
                  deniedPhrases:
                    - "hack the system"
                  denySimilarityThreshold: 0.9
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-guard-assessment-true/v1.0/health" to be ready

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-guard-assessment-true/v1.0/chat" with body:
      """
      {"prompt": "hack the system"}
      """
    Then the response status code should be 422
    And the response body should contain "assessments"
    And the response body should contain "similarity"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-guard-assessment-true-api"
    Then the response should be successful

  Scenario: showAssessment false returns minimal info
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-guard-assessment-false-api
      spec:
        displayName: Prompt Guard - Assessment False Test
        version: v1.0
        context: /prompt-guard-assessment-false/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-prompt-guard
                version: v0
                params:
                  jsonPath: "$.prompt"
                  showAssessment: false
                  deniedPhrases:
                    - "hack the system"
                  denySimilarityThreshold: 0.9
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-guard-assessment-false/v1.0/health" to be ready

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-guard-assessment-false/v1.0/chat" with body:
      """
      {"prompt": "hack the system"}
      """
    Then the response status code should be 422
    And the response body should contain "GUARDRAIL_INTERVENED"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-guard-assessment-false-api"
    Then the response should be successful

  # ==========================================================================
  # Category 7: Edge Cases
  # ==========================================================================

  Scenario: Empty request body returns error
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-guard-empty-body-api
      spec:
        displayName: Prompt Guard - Empty Body Test
        version: v1.0
        context: /prompt-guard-empty-body/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-prompt-guard
                version: v0
                params:
                  deniedPhrases:
                    - "test"
                  denySimilarityThreshold: 0.9
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-guard-empty-body/v1.0/health" to be ready

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-guard-empty-body/v1.0/chat" with body:
      """
      """
    Then the response status code should be 422
    And the response body should contain "SEMANTIC_PROMPT_GUARD"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-guard-empty-body-api"
    Then the response should be successful

  Scenario: Embedding generation failure returns error
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-guard-embed-fail-api
      spec:
        displayName: Prompt Guard - Embedding Failure Test
        version: v1.0
        context: /prompt-guard-embed-fail/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-prompt-guard
                version: v0
                params:
                  jsonPath: "$.prompt"
                  deniedPhrases:
                    - "test phrase"
                  denySimilarityThreshold: 0.9
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-guard-embed-fail/v1.0/health" to be ready

    # The mock embedding provider returns 500 when input contains "error" and "simulate"
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-guard-embed-fail/v1.0/chat" with body:
      """
      {"prompt": "simulate error in embedding"}
      """
    Then the response status code should be 422
    And the response body should contain "SEMANTIC_PROMPT_GUARD"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-guard-embed-fail-api"
    Then the response should be successful

  Scenario: Case insensitivity in semantic matching
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: prompt-guard-case-insensitive-api
      spec:
        displayName: Prompt Guard - Case Insensitive Test
        version: v1.0
        context: /prompt-guard-case-insensitive/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: semantic-prompt-guard
                version: v0
                params:
                  jsonPath: "$.prompt"
                  deniedPhrases:
                    - "HACK THE SYSTEM"
                  denySimilarityThreshold: 0.9
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/prompt-guard-case-insensitive/v1.0/health" to be ready

    # Lowercase version should still be blocked (embeddings are normalized)
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/prompt-guard-case-insensitive/v1.0/chat" with body:
      """
      {"prompt": "hack the system"}
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "prompt-guard-case-insensitive-api"
    Then the response should be successful
