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

@pii-masking-regex
Feature: PII Masking Regex
  As an API developer
  I want to mask or redact PII in requests and responses
  So that I can protect sensitive user data and comply with privacy regulations

  Background:
    Given the gateway services are running

  # ============================================================================
  # MASKING MODE - REQUEST AND RESPONSE
  # ============================================================================

  Scenario: Mask email addresses in request and restore in response
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pii-masking-email-api
      spec:
        displayName: PII Masking Email API
        version: v1.0
        context: /pii-masking-email/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /echo
            policies:
              - name: pii-masking-regex
                version: v0
                params:
                  piiEntities:
                    - piiEntity: "EMAIL"
                      piiRegex: "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
                  redactPII: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pii-masking-email/v1.0/health" to be ready

    # Send request with email
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/pii-masking-email/v1.0/echo" with body:
      """
      Contact me at john.doe@example.com for more info
      """
    Then the response status code should be 200
    And the response should be valid JSON

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "pii-masking-email-api"
    Then the response should be successful

  Scenario: Mask phone numbers in request
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pii-masking-phone-api
      spec:
        displayName: PII Masking Phone API
        version: v1.0
        context: /pii-masking-phone/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /echo
            policies:
              - name: pii-masking-regex
                version: v0
                params:
                  piiEntities:
                    - piiEntity: "PHONE"
                      piiRegex: "\\b\\d{3}-\\d{3}-\\d{4}\\b"
                  redactPII: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pii-masking-phone/v1.0/health" to be ready

    # Send request with phone number
    When I send a POST request to "http://localhost:8080/pii-masking-phone/v1.0/echo" with body:
      """
      Call me at 555-123-4567
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "pii-masking-phone-api"
    Then the response should be successful

  Scenario: Mask multiple PII entities in single request
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pii-masking-multi-api
      spec:
        displayName: PII Masking Multi API
        version: v1.0
        context: /pii-masking-multi/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /echo
            policies:
              - name: pii-masking-regex
                version: v0
                params:
                  piiEntities:
                    - piiEntity: "EMAIL"
                      piiRegex: "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
                    - piiEntity: "PHONE"
                      piiRegex: "\\b\\d{3}-\\d{3}-\\d{4}\\b"
                    - piiEntity: "SSN"
                      piiRegex: "\\b\\d{3}-\\d{2}-\\d{4}\\b"
                  redactPII: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pii-masking-multi/v1.0/health" to be ready

    # Send request with multiple PII types
    When I send a POST request to "http://localhost:8080/pii-masking-multi/v1.0/echo" with body:
      """
      Contact: john@example.com, Phone: 555-123-4567, SSN: 123-45-6789
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "pii-masking-multi-api"
    Then the response should be successful

  # ============================================================================
  # REDACTION MODE
  # ============================================================================

  Scenario: Redact email addresses permanently
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pii-redact-email-api
      spec:
        displayName: PII Redact Email API
        version: v1.0
        context: /pii-redact-email/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /echo
            policies:
              - name: pii-masking-regex
                version: v0
                params:
                  piiEntities:
                    - piiEntity: "EMAIL"
                      piiRegex: "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
                  redactPII: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pii-redact-email/v1.0/health" to be ready

    # Send request with email - should be redacted with *****
    When I send a POST request to "http://localhost:8080/pii-redact-email/v1.0/echo" with body:
      """
      Email me at admin@company.com
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "pii-redact-email-api"
    Then the response should be successful

  Scenario: Redact SSN permanently
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pii-redact-ssn-api
      spec:
        displayName: PII Redact SSN API
        version: v1.0
        context: /pii-redact-ssn/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /echo
            policies:
              - name: pii-masking-regex
                version: v0
                params:
                  piiEntities:
                    - piiEntity: "SSN"
                      piiRegex: "\\b\\d{3}-\\d{2}-\\d{4}\\b"
                  redactPII: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pii-redact-ssn/v1.0/health" to be ready

    # Send request with SSN - should be redacted
    When I send a POST request to "http://localhost:8080/pii-redact-ssn/v1.0/echo" with body:
      """
      My SSN is 987-65-4321
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "pii-redact-ssn-api"
    Then the response should be successful

  Scenario: Redact multiple PII types
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pii-redact-multi-api
      spec:
        displayName: PII Redact Multi API
        version: v1.0
        context: /pii-redact-multi/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /echo
            policies:
              - name: pii-masking-regex
                version: v0
                params:
                  piiEntities:
                    - piiEntity: "EMAIL"
                      piiRegex: "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
                    - piiEntity: "CREDIT_CARD"
                      piiRegex: "\\b\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}\\b"
                  redactPII: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pii-redact-multi/v1.0/health" to be ready

    # Send request with email and credit card
    When I send a POST request to "http://localhost:8080/pii-redact-multi/v1.0/echo" with body:
      """
      Send receipt to john@test.com. Card: 1234-5678-9012-3456
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "pii-redact-multi-api"
    Then the response should be successful

  # ============================================================================
  # JSONPATH EXTRACTION SCENARIOS
  # ============================================================================

  Scenario: Mask PII in specific JSON field
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pii-masking-jsonpath-api
      spec:
        displayName: PII Masking JSONPath API
        version: v1.0
        context: /pii-masking-jsonpath/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /echo
            policies:
              - name: pii-masking-regex
                version: v0
                params:
                  piiEntities:
                    - piiEntity: "EMAIL"
                      piiRegex: "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
                  jsonPath: "$.message"
                  redactPII: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pii-masking-jsonpath/v1.0/health" to be ready

    # Send JSON with email in specific field
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/pii-masking-jsonpath/v1.0/echo" with body:
      """
      {
        "message": "Contact admin@example.com",
        "metadata": "This also has email@test.com but should not be masked"
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "pii-masking-jsonpath-api"
    Then the response should be successful

  Scenario: Mask PII in nested JSON field
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pii-masking-nested-api
      spec:
        displayName: PII Masking Nested API
        version: v1.0
        context: /pii-masking-nested/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /echo
            policies:
              - name: pii-masking-regex
                version: v0
                params:
                  piiEntities:
                    - piiEntity: "PHONE"
                      piiRegex: "\\b\\d{3}-\\d{3}-\\d{4}\\b"
                  jsonPath: "$.user.contact"
                  redactPII: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pii-masking-nested/v1.0/health" to be ready

    # Send nested JSON
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/pii-masking-nested/v1.0/echo" with body:
      """
      {
        "user": {
          "name": "John",
          "contact": "Call me at 555-999-8888"
        }
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "pii-masking-nested-api"
    Then the response should be successful

  # ============================================================================
  # EDGE CASES
  # ============================================================================

  Scenario: Handle content without PII
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pii-masking-no-pii-api
      spec:
        displayName: PII Masking No PII API
        version: v1.0
        context: /pii-masking-no-pii/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /echo
            policies:
              - name: pii-masking-regex
                version: v0
                params:
                  piiEntities:
                    - piiEntity: "EMAIL"
                      piiRegex: "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
                  redactPII: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pii-masking-no-pii/v1.0/health" to be ready

    # Send request without PII - should pass through unchanged
    When I send a POST request to "http://localhost:8080/pii-masking-no-pii/v1.0/echo" with body:
      """
      This is a clean message with no PII
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "pii-masking-no-pii-api"
    Then the response should be successful

  Scenario: Handle empty request body
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pii-masking-empty-api
      spec:
        displayName: PII Masking Empty API
        version: v1.0
        context: /pii-masking-empty/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /echo
            policies:
              - name: pii-masking-regex
                version: v0
                params:
                  piiEntities:
                    - piiEntity: "EMAIL"
                      piiRegex: "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
                  redactPII: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pii-masking-empty/v1.0/health" to be ready

    # Send empty body - should pass through
    When I send a POST request to "http://localhost:8080/pii-masking-empty/v1.0/echo" with body:
      """
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "pii-masking-empty-api"
    Then the response should be successful

  Scenario: Handle invalid JSONPath
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pii-masking-invalid-path-api
      spec:
        displayName: PII Masking Invalid Path API
        version: v1.0
        context: /pii-masking-invalid-path/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /echo
            policies:
              - name: pii-masking-regex
                version: v0
                params:
                  piiEntities:
                    - piiEntity: "EMAIL"
                      piiRegex: "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
                  jsonPath: "$.nonexistent.field"
                  redactPII: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pii-masking-invalid-path/v1.0/health" to be ready

    # Send JSON without the expected field - should return error
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/pii-masking-invalid-path/v1.0/echo" with body:
      """
      {
        "message": "test@example.com"
      }
      """
    Then the response status code should be 500
    And the response should be valid JSON

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "pii-masking-invalid-path-api"
    Then the response should be successful

  # ============================================================================
  # REAL-WORLD SCENARIOS
  # ============================================================================

  Scenario: Mask multiple emails in single message
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pii-masking-multi-emails-api
      spec:
        displayName: PII Masking Multi Emails API
        version: v1.0
        context: /pii-masking-multi-emails/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /echo
            policies:
              - name: pii-masking-regex
                version: v0
                params:
                  piiEntities:
                    - piiEntity: "EMAIL"
                      piiRegex: "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
                  redactPII: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pii-masking-multi-emails/v1.0/health" to be ready

    # Send message with multiple email addresses
    When I send a POST request to "http://localhost:8080/pii-masking-multi-emails/v1.0/echo" with body:
      """
      CC: john@example.com, jane@test.org, admin@company.net
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "pii-masking-multi-emails-api"
    Then the response should be successful

  Scenario: Mask credit card numbers
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pii-masking-credit-card-api
      spec:
        displayName: PII Masking Credit Card API
        version: v1.0
        context: /pii-masking-credit-card/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /echo
            policies:
              - name: pii-masking-regex
                version: v0
                params:
                  piiEntities:
                    - piiEntity: "CREDIT_CARD"
                      piiRegex: "\\b\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}\\b"
                  redactPII: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pii-masking-credit-card/v1.0/health" to be ready

    # Send request with credit card number
    When I send a POST request to "http://localhost:8080/pii-masking-credit-card/v1.0/echo" with body:
      """
      Payment with card 4532-1234-5678-9012
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "pii-masking-credit-card-api"
    Then the response should be successful

  Scenario: Comprehensive PII protection
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: pii-masking-comprehensive-api
      spec:
        displayName: PII Masking Comprehensive API
        version: v1.0
        context: /pii-masking-comprehensive/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /submit
            policies:
              - name: pii-masking-regex
                version: v0
                params:
                  piiEntities:
                    - piiEntity: "EMAIL"
                      piiRegex: "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
                    - piiEntity: "PHONE"
                      piiRegex: "\\b\\d{3}-\\d{3}-\\d{4}\\b"
                    - piiEntity: "SSN"
                      piiRegex: "\\b\\d{3}-\\d{2}-\\d{4}\\b"
                    - piiEntity: "CREDIT_CARD"
                      piiRegex: "\\b\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}\\b"
                  redactPII: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/pii-masking-comprehensive/v1.0/health" to be ready

    # Send request with all PII types
    When I send a POST request to "http://localhost:8080/pii-masking-comprehensive/v1.0/submit" with body:
      """
      User: john@example.com, Phone: 555-123-4567, SSN: 123-45-6789, Card: 4532 1234 5678 9012
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "pii-masking-comprehensive-api"
    Then the response should be successful
