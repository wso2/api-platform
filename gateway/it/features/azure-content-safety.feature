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

@azure-content-safety
Feature: Azure Content Safety Content Moderation Policy
  As an API developer
  I want to validate request and response content using Azure Content Safety API
  So that I can prevent harmful content from being processed

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
        name: azure-safe-request-api
      spec:
        displayName: Azure Content Safety - Safe Request
        version: v1.0
        context: /azure-safe-request/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: azure-content-safety-content-moderation
                version: v0
                params:
                  request:
                    jsonPath: ""
                    hateCategory: 4
                    violenceCategory: 4
                    sexualCategory: 4
                    selfHarmCategory: 4
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/azure-safe-request/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/azure-safe-request/v1.0/validate" with body:
      """
      {"message": "Hello, this is safe content"}
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "azure-safe-request-api"
    Then the response should be successful

  Scenario: Request with hate speech is blocked
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: azure-hate-block-api
      spec:
        displayName: Azure Content Safety - Hate Speech Block
        version: v1.0
        context: /azure-hate-block/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: azure-content-safety-content-moderation
                version: v0
                params:
                  request:
                    hateCategory: 4
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/azure-hate-block/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/azure-hate-block/v1.0/validate" with body:
      """
      {"message": "This content contains hate speech"}
      """
    Then the response status code should be 422
    And the response body should contain "AZURE_CONTENT_SAFETY_CONTENT_MODERATION"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "azure-hate-block-api"
    Then the response should be successful

  Scenario: Request with violence is blocked
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: azure-violence-block-api
      spec:
        displayName: Azure Content Safety - Violence Block
        version: v1.0
        context: /azure-violence-block/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: azure-content-safety-content-moderation
                version: v0
                params:
                  request:
                    violenceCategory: 4
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/azure-violence-block/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/azure-violence-block/v1.0/validate" with body:
      """
      {"message": "This content contains violence"}
      """
    Then the response status code should be 422
    And the response body should contain "AZURE_CONTENT_SAFETY_CONTENT_MODERATION"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "azure-violence-block-api"
    Then the response should be successful

  Scenario: Request with detailed assessment enabled
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: azure-assessment-api
      spec:
        displayName: Azure Content Safety - Detailed Assessment
        version: v1.0
        context: /azure-assessment/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: azure-content-safety-content-moderation
                version: v0
                params:
                  request:
                    showAssessment: true
                    hateCategory: 4
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/azure-assessment/v1.0/health" to be ready

    When I send a POST request to "http://localhost:8080/azure-assessment/v1.0/validate" with body:
      """
      {"message": "This contains hate speech"}
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the response body should contain "assessments"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "azure-assessment-api"
    Then the response should be successful

  # Category 2: Response Validation
  # Note: Response validation scenarios are skipped because they require
  # the upstream to return violating content, which is difficult to test
  # reliably with the available mock backends.

  # Category 3: Category Threshold Configuration

  Scenario: Disabled category allows violating content
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: azure-disabled-category-api
      spec:
        displayName: Azure Content Safety - Disabled Category
        version: v1.0
        context: /azure-disabled-category/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: azure-content-safety-content-moderation
                version: v0
                params:
                  request:
                    hateCategory: -1
                    violenceCategory: 4
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/azure-disabled-category/v1.0/health" to be ready

    # Hate content should pass through because hateCategory is disabled (-1)
    When I send a POST request to "http://localhost:8080/azure-disabled-category/v1.0/validate" with body:
      """
      {"message": "This contains hate speech but should pass"}
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "azure-disabled-category-api"
    Then the response should be successful

  Scenario: Multiple categories enabled with different thresholds
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: azure-multi-category-api
      spec:
        displayName: Azure Content Safety - Multiple Categories
        version: v1.0
        context: /azure-multi-category/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: azure-content-safety-content-moderation
                version: v0
                params:
                  request:
                    hateCategory: 4
                    violenceCategory: 4
                    sexualCategory: 4
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/azure-multi-category/v1.0/health" to be ready

    # Test hate category
    When I send a POST request to "http://localhost:8080/azure-multi-category/v1.0/validate" with body:
      """
      {"message": "This contains hate"}
      """
    Then the response status code should be 422

    # Test violence category
    When I send a POST request to "http://localhost:8080/azure-multi-category/v1.0/validate" with body:
      """
      {"message": "This contains violence"}
      """
    Then the response status code should be 422

    # Test sexual category
    When I send a POST request to "http://localhost:8080/azure-multi-category/v1.0/validate" with body:
      """
      {"message": "This contains sexual content"}
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "azure-multi-category-api"
    Then the response should be successful

  # Category 4: JSONPath Extraction

  Scenario: JSONPath extraction validates specific field
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: azure-jsonpath-api
      spec:
        displayName: Azure Content Safety - JSONPath
        version: v1.0
        context: /azure-jsonpath/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: azure-content-safety-content-moderation
                version: v0
                params:
                  request:
                    jsonPath: "$.message"
                    violenceCategory: 4
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/azure-jsonpath/v1.0/health" to be ready

    # Safe message field - should pass
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/azure-jsonpath/v1.0/validate" with body:
      """
      {
        "message": "Safe content",
        "metadata": "This contains violence but should be ignored"
      }
      """
    Then the response status code should be 200

    # Violating message field - should fail
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/azure-jsonpath/v1.0/validate" with body:
      """
      {
        "message": "This message contains violence",
        "metadata": "Safe metadata"
      }
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "azure-jsonpath-api"
    Then the response should be successful

  Scenario: Nested JSONPath extraction
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: azure-nested-jsonpath-api
      spec:
        displayName: Azure Content Safety - Nested JSONPath
        version: v1.0
        context: /azure-nested-jsonpath/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /chat
            policies:
              - name: azure-content-safety-content-moderation
                version: v0
                params:
                  request:
                    jsonPath: "$.data.content"
                    hateCategory: 4
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/azure-nested-jsonpath/v1.0/health" to be ready

    # Nested safe content - should pass
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/azure-nested-jsonpath/v1.0/chat" with body:
      """
      {
        "data": {
          "content": "Safe nested message",
          "timestamp": "2025-01-01"
        },
        "metadata": "This outer field contains hate but should be ignored"
      }
      """
    Then the response status code should be 200

    # Nested violating content - should fail
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/azure-nested-jsonpath/v1.0/chat" with body:
      """
      {
        "data": {
          "content": "This nested content contains hate",
          "timestamp": "2025-01-01"
        }
      }
      """
    Then the response status code should be 422

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "azure-nested-jsonpath-api"
    Then the response should be successful

  # Category 5: Combined Request and Response Validation

  Scenario: Both request and response phases work independently
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: azure-both-phases-api
      spec:
        displayName: Azure Content Safety - Both Phases
        version: v1.0
        context: /azure-both-phases/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: azure-content-safety-content-moderation
                version: v0
                params:
                  request:
                    violenceCategory: 4
                  response:
                    hateCategory: 4
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/azure-both-phases/v1.0/health" to be ready

    # Safe request - should pass both phases
    When I send a POST request to "http://localhost:8080/azure-both-phases/v1.0/validate" with body:
      """
      {"message": "Safe request content"}
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "azure-both-phases-api"
    Then the response should be successful

  # Category 6: Edge Cases

  Scenario: Empty request body is handled gracefully
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: azure-empty-body-api
      spec:
        displayName: Azure Content Safety - Empty Body
        version: v1.0
        context: /azure-empty-body/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: azure-content-safety-content-moderation
                version: v0
                params:
                  request:
                    violenceCategory: 4
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/azure-empty-body/v1.0/health" to be ready

    # Empty body - should pass through
    When I send a POST request to "http://localhost:8080/azure-empty-body/v1.0/validate" with body:
      """
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "azure-empty-body-api"
    Then the response should be successful

  Scenario: Passthrough on error allows requests despite API failures
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: azure-passthrough-api
      spec:
        displayName: Azure Content Safety - Passthrough on Error
        version: v1.0
        context: /azure-passthrough/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: azure-content-safety-content-moderation
                version: v0
                params:
                  request:
                    passthroughOnError: true
                    violenceCategory: 4
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/azure-passthrough/v1.0/health" to be ready

    # Send content with error trigger - should pass through due to passthroughOnError
    When I send a POST request to "http://localhost:8080/azure-passthrough/v1.0/validate" with body:
      """
      {"message": "This will simulate error in mock service"}
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "azure-passthrough-api"
    Then the response should be successful

  # Category 7: Error Response Structure Validation

  Scenario: Verify complete error response structure
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: azure-error-structure-api
      spec:
        displayName: Azure Content Safety - Error Structure
        version: v1.0
        context: /azure-error-structure/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: azure-content-safety-content-moderation
                version: v0
                params:
                  request:
                    showAssessment: true
                    hateCategory: 4
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/azure-error-structure/v1.0/health" to be ready

    # Trigger error and verify full response structure
    When I send a POST request to "http://localhost:8080/azure-error-structure/v1.0/validate" with body:
      """
      {"message": "This contains hate speech"}
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the JSON response field "type" should be "AZURE_CONTENT_SAFETY_CONTENT_MODERATION"
    And the response body should contain "assessments"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "azure-error-structure-api"
    Then the response should be successful
