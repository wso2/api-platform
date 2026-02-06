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

@aws-bedrock-guardrail
Feature: AWS Bedrock Guardrail Policy
  As an API developer
  I want to validate request and response content using AWS Bedrock Guardrails
  So that I can prevent harmful content and protect PII

  Background:
    Given the gateway services are running

  # Category 1: Basic Request Validation

  Scenario: Request with safe content passes through
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: bedrock-safe-request-api
      spec:
        displayName: Bedrock Guardrail - Safe Request
        version: v1.0
        context: /bedrock-safe-request/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: aws-bedrock-guardrail
                version: v0
                params:
                  request:
                    jsonPath: ""
                    redactPII: false
                    passthroughOnError: false
                    showAssessment: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/bedrock-safe-request/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/bedrock-safe-request/v1.0/validate" with body:
      """
      {"message": "Hello, this is safe content"}
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "bedrock-safe-request-api"
    Then the response should be successful

  Scenario: Request with violating content is blocked
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: bedrock-block-request-api
      spec:
        displayName: Bedrock Guardrail - Block Request
        version: v1.0
        context: /bedrock-block-request/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: aws-bedrock-guardrail
                version: v0
                params:
                  request:
                    jsonPath: ""
                    showAssessment: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/bedrock-block-request/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/bedrock-block-request/v1.0/validate" with body:
      """
      {"message": "This content contains violence and illegal activities"}
      """
    Then the response status code should be 422
    And the response body should contain "AWS_BEDROCK_GUARDRAIL"
    And the response body should contain "GUARDRAIL_INTERVENED"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "bedrock-block-request-api"
    Then the response should be successful

  Scenario: Request violation with detailed assessment
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: bedrock-assessment-api
      spec:
        displayName: Bedrock Guardrail - Assessment
        version: v1.0
        context: /bedrock-assessment/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: aws-bedrock-guardrail
                version: v0
                params:
                  request:
                    showAssessment: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/bedrock-assessment/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/bedrock-assessment/v1.0/validate" with body:
      """
      {"message": "This contains hate speech"}
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the response body should contain "assessments"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "bedrock-assessment-api"
    Then the response should be successful

  # Category 2: Response Validation

  Scenario: Response with safe content passes through
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: bedrock-safe-response-api
      spec:
        displayName: Bedrock Guardrail - Safe Response
        version: v1.0
        context: /bedrock-safe-response/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /data
            policies:
              - name: aws-bedrock-guardrail
                version: v0
                params:
                  response:
                    jsonPath: ""
                    showAssessment: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/bedrock-safe-response/v1.0/health" to be ready

    When I send a GET request to "http://localhost:8080/bedrock-safe-response/v1.0/data"
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "bedrock-safe-response-api"
    Then the response should be successful

  # Category 3: JSONPath Extraction

  Scenario: Request JSONPath extraction validates specific field
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: bedrock-jsonpath-api
      spec:
        displayName: Bedrock Guardrail - JSONPath
        version: v1.0
        context: /bedrock-jsonpath/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: aws-bedrock-guardrail
                version: v0
                params:
                  request:
                    jsonPath: "$.message"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/bedrock-jsonpath/v1.0/health" to be ready

    # Safe message field - should pass
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/bedrock-jsonpath/v1.0/validate" with body:
      """
      {
        "message": "Safe content",
        "metadata": "This contains violence but should be ignored"
      }
      """
    Then the response status code should be 200

    # Violating message field - should fail
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/bedrock-jsonpath/v1.0/validate" with body:
      """
      {
        "message": "This message contains violence",
        "metadata": "Safe metadata"
      }
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "bedrock-jsonpath-api"
    Then the response should be successful

  # Category 4: PII Masking with Restoration

  Scenario: PII is masked in request and restored in response
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: bedrock-pii-masking-api
      spec:
        displayName: Bedrock Guardrail - PII Masking
        version: v1.0
        context: /bedrock-pii-masking/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /get
          - method: POST
            path: /anything
            policies:
              - name: aws-bedrock-guardrail
                version: v0
                params:
                  request:
                    jsonPath: ""
                    redactPII: false
                  response:
                    jsonPath: ""
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/bedrock-pii-masking/v1.0/get" to be ready

    # Send request with PII - should be masked in processing but restored in response
    When I send a POST request to "http://localhost:8080/bedrock-pii-masking/v1.0/anything" with body:
      """
      {"message": "Contact me at mask-test@example.com"}
      """
    Then the response status code should be 200
    # The echo backend returns the request body, so we can verify PII restoration
    And the response body should contain "mask-test@example.com"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "bedrock-pii-masking-api"
    Then the response should be successful

  # Category 5: PII Redaction (Permanent)

  Scenario: PII is permanently redacted with redactPII true
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: bedrock-pii-redaction-api
      spec:
        displayName: Bedrock Guardrail - PII Redaction
        version: v1.0
        context: /bedrock-pii-redaction/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /get
          - method: POST
            path: /anything
            policies:
              - name: aws-bedrock-guardrail
                version: v0
                params:
                  request:
                    redactPII: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/bedrock-pii-redaction/v1.0/get" to be ready

    When I send a POST request to "http://localhost:8080/bedrock-pii-redaction/v1.0/anything" with body:
      """
      {"message": "My SSN is test@example.com"}
      """
    Then the response status code should be 200
    # PII should be permanently redacted, not restored
    And the response body should contain "*****"
    And the response body should not contain "test@example.com"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "bedrock-pii-redaction-api"
    Then the response should be successful

  # Category 6: Combined Request and Response Validation

  Scenario: Both request and response phases work independently
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: bedrock-both-phases-api
      spec:
        displayName: Bedrock Guardrail - Both Phases
        version: v1.0
        context: /bedrock-both-phases/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: aws-bedrock-guardrail
                version: v0
                params:
                  request:
                    jsonPath: ""
                  response:
                    jsonPath: ""
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/bedrock-both-phases/v1.0/health" to be ready

    # Send safe request content
    When I send a POST request to "http://localhost:8080/bedrock-both-phases/v1.0/validate" with body:
      """
      {"message": "Safe request content"}
      """
    # Assuming backend returns safe content
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "bedrock-both-phases-api"
    Then the response should be successful

  # Category 7: Edge Cases

  Scenario: Empty request body is handled correctly
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: bedrock-empty-body-api
      spec:
        displayName: Bedrock Guardrail - Empty Body
        version: v1.0
        context: /bedrock-empty-body/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: aws-bedrock-guardrail
                version: v0
                params:
                  request:
                    jsonPath: ""
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/bedrock-empty-body/v1.0/health" to be ready

    # Empty body - policy should handle gracefully
    When I send a POST request to "http://localhost:8080/bedrock-empty-body/v1.0/validate" with body:
      """
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "bedrock-empty-body-api"
    Then the response should be successful

  Scenario: Passthrough on error mode allows requests despite guardrail failures
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: bedrock-passthrough-api
      spec:
        displayName: Bedrock Guardrail - Passthrough on Error
        version: v1.0
        context: /bedrock-passthrough/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: aws-bedrock-guardrail
                version: v0
                params:
                  request:
                    jsonPath: ""
                    passthroughOnError: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/bedrock-passthrough/v1.0/health" to be ready

    # Send content that triggers error keyword in mock
    When I send a POST request to "http://localhost:8080/bedrock-passthrough/v1.0/validate" with body:
      """
      {"message": "This will simulate error in mock service"}
      """
    # Should pass through despite error due to passthroughOnError=true
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "bedrock-passthrough-api"
    Then the response should be successful

  # Category 8: Nested JSONPath Scenarios

  Scenario: Validate using nested JSONPath extraction
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: bedrock-nested-jsonpath-api
      spec:
        displayName: Bedrock Guardrail - Nested JSONPath
        version: v1.0
        context: /bedrock-nested-jsonpath/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: aws-bedrock-guardrail
                version: v0
                params:
                  request:
                    jsonPath: "$.data.content"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/bedrock-nested-jsonpath/v1.0/health" to be ready

    # Nested JSON with safe content field - should pass
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/bedrock-nested-jsonpath/v1.0/chat" with body:
      """
      {
        "data": {
          "content": "Safe nested message",
          "timestamp": "2025-01-01"
        },
        "metadata": "This outer field contains violence but should be ignored"
      }
      """
    Then the response status code should be 200

    # Nested JSON with violating content field - should fail
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/bedrock-nested-jsonpath/v1.0/chat" with body:
      """
      {
        "data": {
          "content": "This nested content contains violence",
          "timestamp": "2025-01-01"
        }
      }
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "bedrock-nested-jsonpath-api"
    Then the response should be successful

  Scenario: Handle invalid JSONPath gracefully
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: bedrock-invalid-path-api
      spec:
        displayName: Bedrock Guardrail - Invalid Path
        version: v1.0
        context: /bedrock-invalid-path/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: aws-bedrock-guardrail
                version: v0
                params:
                  request:
                    jsonPath: "$.nonexistent.field"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/bedrock-invalid-path/v1.0/health" to be ready

    # JSON without the expected path - should return error
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/bedrock-invalid-path/v1.0/validate" with body:
      """
      {
        "message": "This field exists but not the one we are looking for"
      }
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the response body should contain "AWS_BEDROCK_GUARDRAIL"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "bedrock-invalid-path-api"
    Then the response should be successful

  # Category 9: Error Response Structure Validation

  Scenario: Verify complete error response structure
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: bedrock-error-structure-api
      spec:
        displayName: Bedrock Guardrail - Error Structure
        version: v1.0
        context: /bedrock-error-structure/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: aws-bedrock-guardrail
                version: v0
                params:
                  request:
                    showAssessment: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/bedrock-error-structure/v1.0/health" to be ready

    # Trigger error and verify full response structure
    When I send a POST request to "http://localhost:8080/bedrock-error-structure/v1.0/validate" with body:
      """
      {"message": "This contains hate speech"}
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the JSON response field "type" should be "AWS_BEDROCK_GUARDRAIL"
    And the response body should contain "assessments"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "bedrock-error-structure-api"
    Then the response should be successful
