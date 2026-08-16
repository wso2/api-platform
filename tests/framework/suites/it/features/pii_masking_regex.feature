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

  # NOTE: the six masking-mode (redactPII: false) scenarios from the legacy feature are NOT
  # migrated here. They proved masking by reading GET /captured-request off the old
  # sample-backend to see the bytes the UPSTREAM actually received. The testbench backend
  # deliberately does not port that stateful endpoint, and in masking mode the gateway restores
  # the PII on the way back, so its own response cannot show the masked value. Migrating them
  # without a capture endpoint would mean dropping exactly the "sensitive value must not reach
  # upstream" assertions that are the whole point — so they are parked pending that endpoint.
  # The redaction-mode scenarios below assert on the gateway's own response and are complete.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ============================================================================
  # REDACTION MODE
  # ============================================================================

  Scenario: Redact email addresses permanently
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: pii-redact-email-api
      spec:
        displayName: PII Redact Email API
        version: v1.0
        context: /pii-redact-email/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /echo
            policies:
              - name: pii-masking-regex
                version: v1
                params:
                  customPIIEntities:
                    - piiEntity: "EMAIL"
                      piiRegex: "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
                  jsonPath: ""
                  redactPII: true
      """
    Then the response status code should be 201
    And I send a "GET" request to "/pii-redact-email/v1.0/health" until status 200

    # Send request with email - should be redacted with *****
    When I send a "POST" request to "/pii-redact-email/v1.0/echo" with body:
      """
      Email me at admin@company.com
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the response body should not contain "admin@company.com"
    And the JSON response field "body" should be "Email me at *****"

    # Cleanup
    When I delete the API "pii-redact-email-api"
    Then the response status code should be 200

  Scenario: Redact SSN permanently
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: pii-redact-ssn-api
      spec:
        displayName: PII Redact SSN API
        version: v1.0
        context: /pii-redact-ssn/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /echo
            policies:
              - name: pii-masking-regex
                version: v1
                params:
                  customPIIEntities:
                    - piiEntity: "SSN"
                      piiRegex: "\\b\\d{3}-\\d{2}-\\d{4}\\b"
                  jsonPath: ""
                  redactPII: true
      """
    Then the response status code should be 201
    And I send a "GET" request to "/pii-redact-ssn/v1.0/health" until status 200

    # Send request with SSN - should be redacted
    When I send a "POST" request to "/pii-redact-ssn/v1.0/echo" with body:
      """
      My SSN is 987-65-4321
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the response body should not contain "987-65-4321"
    And the JSON response field "body" should be "My SSN is *****"

    # Cleanup
    When I delete the API "pii-redact-ssn-api"
    Then the response status code should be 200

  Scenario: Redact multiple PII types
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: pii-redact-multi-api
      spec:
        displayName: PII Redact Multi API
        version: v1.0
        context: /pii-redact-multi/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /echo
            policies:
              - name: pii-masking-regex
                version: v1
                params:
                  customPIIEntities:
                    - piiEntity: "EMAIL"
                      piiRegex: "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
                    - piiEntity: "CREDIT_CARD"
                      piiRegex: "\\b\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}\\b"
                  jsonPath: ""
                  redactPII: true
      """
    Then the response status code should be 201
    And I send a "GET" request to "/pii-redact-multi/v1.0/health" until status 200

    # Send request with email and credit card
    When I send a "POST" request to "/pii-redact-multi/v1.0/echo" with body:
      """
      Send receipt to john@test.com. Card: 1234-5678-9012-3456
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the response body should not contain "john@test.com"
    And the response body should not contain "1234-5678-9012-3456"
    And the JSON response field "body" should be "Send receipt to *****. Card: *****"

    # Cleanup
    When I delete the API "pii-redact-multi-api"
    Then the response status code should be 200

  # ============================================================================
  # EDGE CASES
  # ============================================================================

  Scenario: Handle content without PII
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: pii-masking-no-pii-api
      spec:
        displayName: PII Masking No PII API
        version: v1.0
        context: /pii-masking-no-pii/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /echo
            policies:
              - name: pii-masking-regex
                version: v1
                params:
                  customPIIEntities:
                    - piiEntity: "EMAIL"
                      piiRegex: "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
                  jsonPath: ""
                  redactPII: false
      """
    Then the response status code should be 201
    And I send a "GET" request to "/pii-masking-no-pii/v1.0/health" until status 200

    # Send request without PII - should pass through unchanged
    When I send a "POST" request to "/pii-masking-no-pii/v1.0/echo" with body:
      """
      This is a clean message with no PII
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "body" should be "This is a clean message with no PII"

    # Cleanup
    When I delete the API "pii-masking-no-pii-api"
    Then the response status code should be 200

  Scenario: Handle empty request body
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: pii-masking-empty-api
      spec:
        displayName: PII Masking Empty API
        version: v1.0
        context: /pii-masking-empty/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /echo
            policies:
              - name: pii-masking-regex
                version: v1
                params:
                  customPIIEntities:
                    - piiEntity: "EMAIL"
                      piiRegex: "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
                  jsonPath: ""
                  redactPII: false
      """
    Then the response status code should be 201
    And I send a "GET" request to "/pii-masking-empty/v1.0/health" until status 200

    # Send empty body - should pass through
    When I send a "POST" request to "/pii-masking-empty/v1.0/echo" with body:
      """
      """
    Then the response status code should be 200
    And the response should be valid JSON

    # Cleanup
    When I delete the API "pii-masking-empty-api"
    Then the response status code should be 200

  Scenario: Handle invalid JSONPath
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: pii-masking-invalid-path-api
      spec:
        displayName: PII Masking Invalid Path API
        version: v1.0
        context: /pii-masking-invalid-path/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /echo
            policies:
              - name: pii-masking-regex
                version: v1
                params:
                  customPIIEntities:
                    - piiEntity: "EMAIL"
                      piiRegex: "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
                  jsonPath: "$.nonexistent.field"
                  redactPII: false
      """
    Then the response status code should be 201
    And I send a "GET" request to "/pii-masking-invalid-path/v1.0/health" until status 200

    # Send JSON without the expected field - should return error
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/pii-masking-invalid-path/v1.0/echo" with body:
      """
      {
        "message": "test@example.com"
      }
      """
    Then the response status code should be 500
    And the response should be valid JSON

    # Cleanup
    When I delete the API "pii-masking-invalid-path-api"
    Then the response status code should be 200

  # ============================================================================
  # REAL-WORLD SCENARIOS
  # ============================================================================

  Scenario: Mask credit card numbers
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: pii-masking-credit-card-api
      spec:
        displayName: PII Masking Credit Card API
        version: v1.0
        context: /pii-masking-credit-card/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /echo
            policies:
              - name: pii-masking-regex
                version: v1
                params:
                  customPIIEntities:
                    - piiEntity: "CREDIT_CARD"
                      piiRegex: "\\b\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}\\b"
                  jsonPath: ""
                  redactPII: true
      """
    Then the response status code should be 201
    And I send a "GET" request to "/pii-masking-credit-card/v1.0/health" until status 200

    # Send request with credit card number
    When I send a "POST" request to "/pii-masking-credit-card/v1.0/echo" with body:
      """
      Payment with card 4532-1234-5678-9012
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the response body should not contain "4532-1234-5678-9012"
    And the JSON response field "body" should be "Payment with card *****"

    # Cleanup
    When I delete the API "pii-masking-credit-card-api"
    Then the response status code should be 200

  Scenario: Comprehensive PII protection
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: pii-masking-comprehensive-api
      spec:
        displayName: PII Masking Comprehensive API
        version: v1.0
        context: /pii-masking-comprehensive/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /submit
            policies:
              - name: pii-masking-regex
                version: v1
                params:
                  customPIIEntities:
                    - piiEntity: "EMAIL"
                      piiRegex: "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
                    - piiEntity: "PHONE"
                      piiRegex: "\\b\\d{3}-\\d{3}-\\d{4}\\b"
                    - piiEntity: "SSN"
                      piiRegex: "\\b\\d{3}-\\d{2}-\\d{4}\\b"
                    - piiEntity: "CREDIT_CARD"
                      piiRegex: "\\b\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}\\b"
                  jsonPath: ""
                  redactPII: true
      """
    Then the response status code should be 201
    And I send a "GET" request to "/pii-masking-comprehensive/v1.0/health" until status 200

    # Send request with all PII types
    When I send a "POST" request to "/pii-masking-comprehensive/v1.0/submit" with body:
      """
      User: john@example.com, Phone: 555-123-4567, SSN: 123-45-6789, Card: 4532 1234 5678 9012
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the response body should not contain "john@example.com"
    And the response body should not contain "555-123-4567"
    And the response body should not contain "123-45-6789"
    And the response body should not contain "4532 1234 5678 9012"
    And the JSON response field "body" should be "User: *****, Phone: *****, SSN: *****, Card: *****"

    # Cleanup
    When I delete the API "pii-masking-comprehensive-api"
    Then the response status code should be 200
