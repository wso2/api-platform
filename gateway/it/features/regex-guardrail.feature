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

@regex-guardrail
Feature: Regex Guardrail
  As an API developer
  I want to validate content against regular expression patterns
  So that I can enforce content rules and prevent unwanted data

  Background:
    Given the gateway services are running

  # ============================================================================
  # BASIC REGEX MATCHING SCENARIOS
  # ============================================================================

  Scenario: Pass request when content matches regex pattern
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: regex-match-pass-api
      spec:
        displayName: Regex Match Pass API
        version: v1.0
        context: /regex-match-pass/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: regex-guardrail
                version: v0
                params:
                  request:
                    regex: "^[A-Z]{3}-[0-9]{4}$"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/regex-match-pass/v1.0/health" to be ready

    # Content matches pattern - should pass
    When I send a POST request to "http://localhost:8080/regex-match-pass/v1.0/validate" with body:
      """
      ABC-1234
      """
    Then the response status code should be 200

    # Content doesn't match pattern - should fail
    When I send a POST request to "http://localhost:8080/regex-match-pass/v1.0/validate" with body:
      """
      invalid-format
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the response body should contain "REGEX_GUARDRAIL"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "regex-match-pass-api"
    Then the response should be successful

  Scenario: Block profanity using regex pattern
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: regex-profanity-api
      spec:
        displayName: Regex Profanity API
        version: v1.0
        context: /regex-profanity/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: regex-guardrail
                version: v0
                params:
                  request:
                    regex: "(badword|profanity|offensive)"
                    invert: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/regex-profanity/v1.0/health" to be ready

    # Clean content - should pass
    When I send a POST request to "http://localhost:8080/regex-profanity/v1.0/validate" with body:
      """
      This is a clean message with no issues.
      """
    Then the response status code should be 200

    # Content with banned word - should fail
    When I send a POST request to "http://localhost:8080/regex-profanity/v1.0/validate" with body:
      """
      This message contains a badword and should fail.
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "regex-profanity-api"
    Then the response should be successful

  Scenario: Validate email format with regex
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: regex-email-api
      spec:
        displayName: Regex Email API
        version: v1.0
        context: /regex-email/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: regex-guardrail
                version: v0
                params:
                  request:
                    regex: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/regex-email/v1.0/health" to be ready

    # Valid email format - should pass
    When I send a POST request to "http://localhost:8080/regex-email/v1.0/validate" with body:
      """
      user@example.com
      """
    Then the response status code should be 200

    # Invalid email format - should fail
    When I send a POST request to "http://localhost:8080/regex-email/v1.0/validate" with body:
      """
      not-an-email
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "regex-email-api"
    Then the response should be successful

  # ============================================================================
  # INVERTED LOGIC SCENARIOS
  # ============================================================================

  Scenario: Inverted logic passes when regex does not match
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: regex-invert-api
      spec:
        displayName: Regex Invert API
        version: v1.0
        context: /regex-invert/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: regex-guardrail
                version: v0
                params:
                  request:
                    regex: "\\b\\d{3}-\\d{2}-\\d{4}\\b"
                    invert: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/regex-invert/v1.0/health" to be ready

    # Content without SSN pattern - should pass (invert=true)
    When I send a POST request to "http://localhost:8080/regex-invert/v1.0/validate" with body:
      """
      This is safe content without sensitive data.
      """
    Then the response status code should be 200

    # Content with SSN pattern - should fail (invert=true blocks matches)
    When I send a POST request to "http://localhost:8080/regex-invert/v1.0/validate" with body:
      """
      My SSN is 123-45-6789 please keep it safe.
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "regex-invert-api"
    Then the response should be successful

  Scenario: Inverted logic for credit card number detection
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: regex-creditcard-api
      spec:
        displayName: Regex Credit Card API
        version: v1.0
        context: /regex-creditcard/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: regex-guardrail
                version: v0
                params:
                  request:
                    regex: "\\b\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}\\b"
                    invert: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/regex-creditcard/v1.0/health" to be ready

    # Safe content - should pass
    When I send a POST request to "http://localhost:8080/regex-creditcard/v1.0/validate" with body:
      """
      Please process my order for 100 USD.
      """
    Then the response status code should be 200

    # Content with credit card pattern - should fail
    When I send a POST request to "http://localhost:8080/regex-creditcard/v1.0/validate" with body:
      """
      Card number: 4532-1234-5678-9012
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "regex-creditcard-api"
    Then the response should be successful

  # ============================================================================
  # JSONPATH EXTRACTION SCENARIOS
  # ============================================================================

  Scenario: Validate regex using JSONPath extraction
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: regex-jsonpath-api
      spec:
        displayName: Regex JSONPath API
        version: v1.0
        context: /regex-jsonpath/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: regex-guardrail
                version: v0
                params:
                  request:
                    jsonPath: "$.code"
                    regex: "^[A-Z]{2}[0-9]{3}$"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/regex-jsonpath/v1.0/health" to be ready

    # JSON with valid code format - should pass
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/regex-jsonpath/v1.0/validate" with body:
      """
      {
        "code": "AB123",
        "message": "This field can be anything INVALID999"
      }
      """
    Then the response status code should be 200

    # JSON with invalid code format - should fail
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/regex-jsonpath/v1.0/validate" with body:
      """
      {
        "code": "invalid",
        "message": "AB123"
      }
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "regex-jsonpath-api"
    Then the response should be successful

  Scenario: Validate regex using nested JSONPath
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: regex-nested-jsonpath-api
      spec:
        displayName: Regex Nested JSONPath API
        version: v1.0
        context: /regex-nested-jsonpath/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: regex-guardrail
                version: v0
                params:
                  request:
                    jsonPath: "$.user.username"
                    regex: "^[a-z0-9_]{3,16}$"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/regex-nested-jsonpath/v1.0/health" to be ready

    # Nested JSON with valid username - should pass
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/regex-nested-jsonpath/v1.0/validate" with body:
      """
      {
        "user": {
          "username": "valid_user123",
          "email": "invalid@format"
        }
      }
      """
    Then the response status code should be 200

    # Nested JSON with invalid username - should fail
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/regex-nested-jsonpath/v1.0/validate" with body:
      """
      {
        "user": {
          "username": "Invalid-Username!",
          "email": "valid@example.com"
        }
      }
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "regex-nested-jsonpath-api"
    Then the response should be successful

  Scenario: Handle invalid JSONPath gracefully
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: regex-invalid-jsonpath-api
      spec:
        displayName: Regex Invalid JSONPath API
        version: v1.0
        context: /regex-invalid-jsonpath/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: regex-guardrail
                version: v0
                params:
                  request:
                    jsonPath: "$.nonexistent.field"
                    regex: ".*"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/regex-invalid-jsonpath/v1.0/health" to be ready

    # JSON without the expected path - should return error
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/regex-invalid-jsonpath/v1.0/validate" with body:
      """
      {
        "message": "This field exists but not the one we want"
      }
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the response body should contain "REGEX_GUARDRAIL"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "regex-invalid-jsonpath-api"
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
        name: regex-assessment-api
      spec:
        displayName: Regex Assessment API
        version: v1.0
        context: /regex-assessment/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: regex-guardrail
                version: v0
                params:
                  request:
                    regex: "^[0-9]{5}$"
                    showAssessment: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/regex-assessment/v1.0/health" to be ready

    # Request that fails - should include assessment details
    When I send a POST request to "http://localhost:8080/regex-assessment/v1.0/validate" with body:
      """
      12345ABC
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the response body should contain "assessments"
    And the response body should contain "regular expression"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "regex-assessment-api"
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
        name: regex-empty-api
      spec:
        displayName: Regex Empty API
        version: v1.0
        context: /regex-empty/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: regex-guardrail
                version: v0
                params:
                  request:
                    regex: ".*"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/regex-empty/v1.0/health" to be ready

    # Empty body - should pass (empty bodies are allowed)
    When I send a POST request to "http://localhost:8080/regex-empty/v1.0/validate" with body:
      """
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "regex-empty-api"
    Then the response should be successful

  Scenario: Complex regex pattern with multiple alternatives
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: regex-complex-api
      spec:
        displayName: Regex Complex API
        version: v1.0
        context: /regex-complex/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: regex-guardrail
                version: v0
                params:
                  request:
                    regex: "^(approved|pending|rejected)$"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/regex-complex/v1.0/health" to be ready

    # Allowed status - should pass
    When I send a POST request to "http://localhost:8080/regex-complex/v1.0/validate" with body:
      """
      approved
      """
    Then the response status code should be 200

    # Another allowed status - should pass
    When I send a POST request to "http://localhost:8080/regex-complex/v1.0/validate" with body:
      """
      pending
      """
    Then the response status code should be 200

    # Disallowed status - should fail
    When I send a POST request to "http://localhost:8080/regex-complex/v1.0/validate" with body:
      """
      unknown
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "regex-complex-api"
    Then the response should be successful

  Scenario: Case-sensitive regex matching
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: regex-case-sensitive-api
      spec:
        displayName: Regex Case Sensitive API
        version: v1.0
        context: /regex-case-sensitive/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: regex-guardrail
                version: v0
                params:
                  request:
                    regex: "^UPPERCASE$"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/regex-case-sensitive/v1.0/health" to be ready

    # Correct case - should pass
    When I send a POST request to "http://localhost:8080/regex-case-sensitive/v1.0/validate" with body:
      """
      UPPERCASE
      """
    Then the response status code should be 200

    # Wrong case - should fail
    When I send a POST request to "http://localhost:8080/regex-case-sensitive/v1.0/validate" with body:
      """
      uppercase
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "regex-case-sensitive-api"
    Then the response should be successful

  Scenario: Case-insensitive regex matching
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: regex-case-insensitive-api
      spec:
        displayName: Regex Case Insensitive API
        version: v1.0
        context: /regex-case-insensitive/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: regex-guardrail
                version: v0
                params:
                  request:
                    regex: "(?i)^hello$"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/regex-case-insensitive/v1.0/health" to be ready

    # Lowercase - should pass
    When I send a POST request to "http://localhost:8080/regex-case-insensitive/v1.0/validate" with body:
      """
      hello
      """
    Then the response status code should be 200

    # Uppercase - should pass (case insensitive)
    When I send a POST request to "http://localhost:8080/regex-case-insensitive/v1.0/validate" with body:
      """
      HELLO
      """
    Then the response status code should be 200

    # Mixed case - should pass (case insensitive)
    When I send a POST request to "http://localhost:8080/regex-case-insensitive/v1.0/validate" with body:
      """
      HeLLo
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "regex-case-insensitive-api"
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
        name: regex-combined-api
      spec:
        displayName: Regex Combined API
        version: v1.0
        context: /regex-combined/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: regex-guardrail
                version: v0
                params:
                  request:
                    regex: "^[A-Za-z0-9 ]+$"
                  response:
                    regex: ".*"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/regex-combined/v1.0/health" to be ready

    # Request with valid alphanumeric content - should pass
    When I send a POST request to "http://localhost:8080/regex-combined/v1.0/validate" with body:
      """
      Valid Content 123
      """
    Then the response status code should be 200

    # Request with special characters - should fail at request phase
    When I send a POST request to "http://localhost:8080/regex-combined/v1.0/validate" with body:
      """
      Invalid@Content#
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "regex-combined-api"
    Then the response should be successful

  # ============================================================================
  # SPECIAL CONTENT SCENARIOS
  # ============================================================================


  Scenario: Validate phone number format
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: regex-phone-api
      spec:
        displayName: Regex Phone API
        version: v1.0
        context: /regex-phone/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: regex-guardrail
                version: v0
                params:
                  request:
                    regex: "^\\+?[1-9]\\d{1,14}$"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/regex-phone/v1.0/health" to be ready

    # Valid international phone format - should pass
    When I send a POST request to "http://localhost:8080/regex-phone/v1.0/validate" with body:
      """
      +14155552671
      """
    Then the response status code should be 200

    # Invalid phone format - should fail
    When I send a POST request to "http://localhost:8080/regex-phone/v1.0/validate" with body:
      """
      not-a-phone
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "regex-phone-api"
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
        name: regex-error-structure-api
      spec:
        displayName: Regex Error Structure API
        version: v1.0
        context: /regex-error-structure/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: regex-guardrail
                version: v0
                params:
                  request:
                    regex: "^[0-9]+$"
                    showAssessment: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/regex-error-structure/v1.0/health" to be ready

    # Trigger error and verify full response structure
    When I send a POST request to "http://localhost:8080/regex-error-structure/v1.0/validate" with body:
      """
      abc123
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the JSON response field "type" should be "REGEX_GUARDRAIL"
    And the JSON response field "message.action" should be "GUARDRAIL_INTERVENED"
    And the JSON response field "message.interveningGuardrail" should be "regex-guardrail"
    And the JSON response field "message.direction" should be "REQUEST"
    And the response body should contain "assessments"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "regex-error-structure-api"
    Then the response should be successful

  # ============================================================================
  # RESPONSE VALIDATION WITH SPECIFIC REGEXES
  # ============================================================================

  Scenario: Validate response body with JSON status field
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: regex-response-json-api
      spec:
        displayName: Regex Response JSON API
        version: v1.0
        context: /regex-response-json/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /echo
            policies:
              - name: regex-guardrail
                version: v0
                params:
                  response:
                    jsonPath: "$.method"
                    regex: "^POST$"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/regex-response-json/v1.0/health" to be ready

    # Request that generates valid response - should pass response validation
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/regex-response-json/v1.0/echo" with body:
      """
      {"test": "data"}
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "regex-response-json-api"
    Then the response should be successful

  Scenario: Block response with forbidden content pattern
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: regex-response-block-api
      spec:
        displayName: Regex Response Block API
        version: v1.0
        context: /regex-response-block/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /echo
            policies:
              - name: regex-guardrail
                version: v0
                params:
                  response:
                    regex: "(internal|confidential|secret)"
                    invert: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/regex-response-block/v1.0/health" to be ready

    # Response without forbidden words should pass
    When I send a GET request to "http://localhost:8080/regex-response-block/v1.0/echo"
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "regex-response-block-api"
    Then the response should be successful

  Scenario: Response validation with format check
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: regex-response-format-api
      spec:
        displayName: Regex Response Format API
        version: v1.0
        context: /regex-response-format/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /echo
            policies:
              - name: regex-guardrail
                version: v0
                params:
                  response:
                    jsonPath: "$.host"
                    regex: "^[a-zA-Z0-9.-]+:[0-9]+$"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/regex-response-format/v1.0/health" to be ready

    # Request that generates response with valid host format
    When I send a POST request to "http://localhost:8080/regex-response-format/v1.0/echo" with body:
      """
      test
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "regex-response-format-api"
    Then the response should be successful

  # ============================================================================
  # UNICODE AND INTERNATIONALIZATION SCENARIOS
  # ============================================================================

  Scenario: Validate Unicode characters in content
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: regex-unicode-api
      spec:
        displayName: Regex Unicode API
        version: v1.0
        context: /regex-unicode/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: regex-guardrail
                version: v0
                params:
                  request:
                    regex: "^[\\p{L}\\p{N}\\s]+$"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/regex-unicode/v1.0/health" to be ready

    # Content with Unicode letters and numbers - should pass
    When I send a POST request to "http://localhost:8080/regex-unicode/v1.0/validate" with body:
      """
      Hello 世界 123
      """
    Then the response status code should be 200

    # Content with special symbols - should fail
    When I send a POST request to "http://localhost:8080/regex-unicode/v1.0/validate" with body:
      """
      Hello@World!
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "regex-unicode-api"
    Then the response should be successful

  Scenario: Validate international characters in different scripts
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: regex-international-api
      spec:
        displayName: Regex International API
        version: v1.0
        context: /regex-international/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: regex-guardrail
                version: v0
                params:
                  request:
                    regex: ".*"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/regex-international/v1.0/health" to be ready

    # Chinese characters - should pass
    When I send a POST request to "http://localhost:8080/regex-international/v1.0/validate" with body:
      """
      你好世界
      """
    Then the response status code should be 200

    # Arabic characters - should pass
    When I send a POST request to "http://localhost:8080/regex-international/v1.0/validate" with body:
      """
      مرحبا بالعالم
      """
    Then the response status code should be 200

    # Cyrillic characters - should pass
    When I send a POST request to "http://localhost:8080/regex-international/v1.0/validate" with body:
      """
      Привет мир
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "regex-international-api"
    Then the response should be successful

  Scenario: Validate emoji and special Unicode symbols
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: regex-emoji-api
      spec:
        displayName: Regex Emoji API
        version: v1.0
        context: /regex-emoji/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: regex-guardrail
                version: v0
                params:
                  request:
                    regex: "^[a-zA-Z0-9\\s]+$"
                    invert: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/regex-emoji/v1.0/health" to be ready

    # Content with emoji (not alphanumeric) - should pass with invert=true
    When I send a POST request to "http://localhost:8080/regex-emoji/v1.0/validate" with body:
      """
      Hello 😀 🌍
      """
    Then the response status code should be 200

    # Pure alphanumeric content - should fail with invert=true
    When I send a POST request to "http://localhost:8080/regex-emoji/v1.0/validate" with body:
      """
      Hello World 123
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "regex-emoji-api"
    Then the response should be successful

  Scenario: Validate UTF-8 encoded content with diacritics
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: regex-diacritics-api
      spec:
        displayName: Regex Diacritics API
        version: v1.0
        context: /regex-diacritics/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: regex-guardrail
                version: v0
                params:
                  request:
                    regex: "^[\\p{L}\\s]+$"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/regex-diacritics/v1.0/health" to be ready

    # Content with diacritics - should pass
    When I send a POST request to "http://localhost:8080/regex-diacritics/v1.0/validate" with body:
      """
      café résumé naïve
      """
    Then the response status code should be 200

    # Content with numbers - should fail
    When I send a POST request to "http://localhost:8080/regex-diacritics/v1.0/validate" with body:
      """
      café 123
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "regex-diacritics-api"
    Then the response should be successful
