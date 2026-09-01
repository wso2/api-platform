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

# The URLs this guardrail validates are the ones found IN THE PAYLOAD, not the API upstream:
# the policy extracts each URL from the body and performs an HTTP HEAD (or DNS-only) reachability
# check against it. So a "reachable" payload URL must point at something that actually answers,
# and the original's http://sample-backend:9080/... is rewritten to http://testbench:3000/...
# here for the same reason the upstream is — sample-backend does not exist in this environment,
# testbench does and reflects every path with a 200 (HEAD included). The negative cases keep
# their *.invalid hosts unchanged: that TLD is reserved (RFC 6761) and never resolves, so the
# "unreachable URL is blocked" assertions hold with no outbound internet access at all.

@url-guardrail
Feature: URL Guardrail
  As an API developer
  I want to validate URLs in requests and responses
  So that I can prevent invalid or unreachable URLs from being processed

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ============================================================================
  # HTTP REACHABILITY VALIDATION SCENARIOS (Default Mode)
  # ============================================================================

  Scenario: Block request with unreachable URL using HTTP HEAD check
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: url-http-check-api
      spec:
        displayName: URL HTTP Check API
        version: v1.0
        context: /url-http-check/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: url-guardrail
                version: v1
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    timeout: 5000
      """
    Then the response status code should be 201
    And I send a "GET" request to "/url-http-check/v1.0/health" until status 200

    # Valid reachable URL - should pass (testbench is reachable)
    When I send a "POST" request to "/url-http-check/v1.0/validate" with body:
      """
      Check this URL: http://testbench:3000/api/v1/health
      """
    Then the response status code should be 200

    # Invalid/unreachable URL - should fail
    When I send a "POST" request to "/url-http-check/v1.0/validate" with body:
      """
      Check this URL: http://nonexistent-host-12345.invalid/test
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the JSON response field "type" should be "URL_GUARDRAIL"

    # Cleanup
    When I delete the API "url-http-check-api"
    Then the response status code should be 200

  Scenario: Allow request without any URLs
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: url-no-urls-api
      spec:
        displayName: URL No URLs API
        version: v1.0
        context: /url-no-urls/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: url-guardrail
                version: v1
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    timeout: 5000
      """
    Then the response status code should be 201
    And I send a "GET" request to "/url-no-urls/v1.0/health" until status 200

    # Request without URLs - should pass
    When I send a "POST" request to "/url-no-urls/v1.0/validate" with body:
      """
      This is a message with no URLs at all, just plain text.
      """
    Then the response status code should be 200

    # Cleanup
    When I delete the API "url-no-urls-api"
    Then the response status code should be 200

  Scenario: Validate multiple URLs in single request
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: url-multiple-api
      spec:
        displayName: URL Multiple API
        version: v1.0
        context: /url-multiple/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: url-guardrail
                version: v1
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    timeout: 5000
      """
    Then the response status code should be 201
    And I send a "GET" request to "/url-multiple/v1.0/health" until status 200

    # Multiple valid URLs - should pass
    When I send a "POST" request to "/url-multiple/v1.0/validate" with body:
      """
      Check these URLs: http://testbench:3000/health and http://testbench:3000/api/v1/health
      """
    Then the response status code should be 200

    # Multiple URLs with one invalid - should fail
    When I send a "POST" request to "/url-multiple/v1.0/validate" with body:
      """
      Check http://testbench:3000/health and http://invalid-domain-xyz.invalid/test
      """
    Then the response status code should be 422

    # Cleanup
    When I delete the API "url-multiple-api"
    Then the response status code should be 200

  # ============================================================================
  # DNS-ONLY VALIDATION SCENARIOS
  # ============================================================================

  Scenario: Validate URL using DNS-only check
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: url-dns-only-api
      spec:
        displayName: URL DNS Only API
        version: v1.0
        context: /url-dns-only/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: url-guardrail
                version: v1
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    onlyDNS: true
                    timeout: 3000
      """
    Then the response status code should be 201
    And I send a "GET" request to "/url-dns-only/v1.0/health" until status 200

    # URL with resolvable domain - should pass
    When I send a "POST" request to "/url-dns-only/v1.0/validate" with body:
      """
      Check this URL: http://testbench:3000/test
      """
    Then the response status code should be 200

    # URL with non-resolvable domain - should fail
    When I send a "POST" request to "/url-dns-only/v1.0/validate" with body:
      """
      Check this URL: http://nonexistent-domain-xyz12345.invalid/test
      """
    Then the response status code should be 422

    # Cleanup
    When I delete the API "url-dns-only-api"
    Then the response status code should be 200

  # ============================================================================
  # JSONPATH EXTRACTION SCENARIOS
  # ============================================================================

  Scenario: Validate URL using JSONPath extraction
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: url-jsonpath-api
      spec:
        displayName: URL JSONPath API
        version: v1.0
        context: /url-jsonpath/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: url-guardrail
                version: v1
                params:
                  request:
                    enabled: true
                    jsonPath: "$.url"
                    timeout: 5000
      """
    Then the response status code should be 201
    And I send a "GET" request to "/url-jsonpath/v1.0/health" until status 200

    # JSON with valid URL in specified field - should pass
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/url-jsonpath/v1.0/validate" with body:
      """
      {
        "url": "http://testbench:3000/health",
        "other": "http://invalid-domain-xyz.invalid/test"
      }
      """
    Then the response status code should be 200

    # JSON with invalid URL in specified field - should fail
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/url-jsonpath/v1.0/validate" with body:
      """
      {
        "url": "http://nonexistent-host-abc123.invalid/test",
        "other": "http://testbench:3000/health"
      }
      """
    Then the response status code should be 422

    # Cleanup
    When I delete the API "url-jsonpath-api"
    Then the response status code should be 200

  Scenario: Validate URL using nested JSONPath
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: url-nested-jsonpath-api
      spec:
        displayName: URL Nested JSONPath API
        version: v1.0
        context: /url-nested-jsonpath/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: url-guardrail
                version: v1
                params:
                  request:
                    enabled: true
                    jsonPath: "$.data.link"
                    timeout: 5000
      """
    Then the response status code should be 201
    And I send a "GET" request to "/url-nested-jsonpath/v1.0/health" until status 200

    # Nested JSON with valid URL - should pass
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/url-nested-jsonpath/v1.0/validate" with body:
      """
      {
        "data": {
          "link": "http://testbench:3000/api/v1/health",
          "timestamp": "2025-01-01"
        },
        "badUrl": "http://invalid.invalid/test"
      }
      """
    Then the response status code should be 200

    # Nested JSON with invalid URL - should fail
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/url-nested-jsonpath/v1.0/validate" with body:
      """
      {
        "data": {
          "link": "http://nonexistent-domain-xyz.invalid/test",
          "timestamp": "2025-01-01"
        }
      }
      """
    Then the response status code should be 422

    # Cleanup
    When I delete the API "url-nested-jsonpath-api"
    Then the response status code should be 200

  Scenario: Handle invalid JSONPath gracefully
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: url-invalid-jsonpath-api
      spec:
        displayName: URL Invalid JSONPath API
        version: v1.0
        context: /url-invalid-jsonpath/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: url-guardrail
                version: v1
                params:
                  request:
                    enabled: true
                    jsonPath: "$.nonexistent.field"
                    timeout: 5000
      """
    Then the response status code should be 201
    And I send a "GET" request to "/url-invalid-jsonpath/v1.0/health" until status 200

    # JSON without the expected path - should return error
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/url-invalid-jsonpath/v1.0/validate" with body:
      """
      {
        "url": "http://testbench:3000/health"
      }
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the JSON response field "type" should be "URL_GUARDRAIL"

    # Cleanup
    When I delete the API "url-invalid-jsonpath-api"
    Then the response status code should be 200

  # ============================================================================
  # TIMEOUT SCENARIOS
  # ============================================================================

  Scenario: Custom timeout configuration
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: url-timeout-api
      spec:
        displayName: URL Timeout API
        version: v1.0
        context: /url-timeout/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: url-guardrail
                version: v1
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    timeout: 1000
      """
    Then the response status code should be 201
    And I send a "GET" request to "/url-timeout/v1.0/health" until status 200

    # Valid URL with short timeout - should still pass for reachable URLs
    When I send a "POST" request to "/url-timeout/v1.0/validate" with body:
      """
      Check this URL: http://testbench:3000/health
      """
    Then the response status code should be 200

    # Cleanup
    When I delete the API "url-timeout-api"
    Then the response status code should be 200

  # ============================================================================
  # SHOW ASSESSMENT SCENARIOS
  # ============================================================================

  Scenario: Show detailed assessment with invalid URLs
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: url-assessment-api
      spec:
        displayName: URL Assessment API
        version: v1.0
        context: /url-assessment/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: url-guardrail
                version: v1
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    showAssessment: true
                    timeout: 5000
      """
    Then the response status code should be 201
    And I send a "GET" request to "/url-assessment/v1.0/health" until status 200

    # Request that fails - should include assessment details with invalid URLs
    When I send a "POST" request to "/url-assessment/v1.0/validate" with body:
      """
      Check this URL: http://invalid-domain-xyz123.invalid/test
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the JSON response should have field "message.assessments"
    And the JSON response should have field "message.assessments.invalidUrls"

    # Cleanup
    When I delete the API "url-assessment-api"
    Then the response status code should be 200

  # ============================================================================
  # EDGE CASES
  # ============================================================================

  Scenario: Empty request body is handled correctly
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: url-empty-api
      spec:
        displayName: URL Empty API
        version: v1.0
        context: /url-empty/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: url-guardrail
                version: v1
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    timeout: 5000
      """
    Then the response status code should be 201
    And I send a "GET" request to "/url-empty/v1.0/health" until status 200

    # Empty body has no URLs - should pass
    When I send a "POST" request to "/url-empty/v1.0/validate" with body:
      """
      """
    Then the response status code should be 200

    # Cleanup
    When I delete the API "url-empty-api"
    Then the response status code should be 200

  Scenario: Handle malformed URLs gracefully
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: url-malformed-api
      spec:
        displayName: URL Malformed API
        version: v1.0
        context: /url-malformed/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: url-guardrail
                version: v1
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    timeout: 5000
      """
    Then the response status code should be 201
    And I send a "GET" request to "/url-malformed/v1.0/health" until status 200

    # Malformed URL should be caught by regex or treated as invalid
    When I send a "POST" request to "/url-malformed/v1.0/validate" with body:
      """
      This has text that looks like a URL: htp://wrong-protocol.com
      """
    Then the response status code should be 200

    # Cleanup
    When I delete the API "url-malformed-api"
    Then the response status code should be 200

  # ============================================================================
  # RESPONSE VALIDATION SCENARIOS
  # ============================================================================

  Scenario: Combined request and response validation
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: url-combined-api
      spec:
        displayName: URL Combined API
        version: v1.0
        context: /url-combined/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: url-guardrail
                version: v1
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    timeout: 5000
                  response:
                    jsonPath: ""
                    timeout: 5000
      """
    Then the response status code should be 201
    And I send a "GET" request to "/url-combined/v1.0/health" until status 200

    # Request with valid URL - should pass request validation
    When I send a "POST" request to "/url-combined/v1.0/validate" with body:
      """
      Check this URL: http://testbench:3000/health
      """
    Then the response status code should be 200

    # Request with invalid URL - should fail at request phase
    When I send a "POST" request to "/url-combined/v1.0/validate" with body:
      """
      Check this URL: http://invalid-domain-xyz.invalid/test
      """
    Then the response status code should be 422

    # Cleanup
    When I delete the API "url-combined-api"
    Then the response status code should be 200

  # ============================================================================
  # SPECIAL CONTENT SCENARIOS
  # ============================================================================

  Scenario: Handle URLs with special characters
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: url-special-chars-api
      spec:
        displayName: URL Special Chars API
        version: v1.0
        context: /url-special-chars/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: url-guardrail
                version: v1
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    timeout: 5000
      """
    Then the response status code should be 201
    And I send a "GET" request to "/url-special-chars/v1.0/health" until status 200

    # URL with query parameters and paths
    When I send a "POST" request to "/url-special-chars/v1.0/validate" with body:
      """
      Check this URL: http://testbench:3000/api/v1/test?param=value&other=123
      """
    Then the response status code should be 200

    # Cleanup
    When I delete the API "url-special-chars-api"
    Then the response status code should be 200

  Scenario: Handle plain text content type
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: url-plaintext-api
      spec:
        displayName: URL Plain Text API
        version: v1.0
        context: /url-plaintext/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: url-guardrail
                version: v1
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    timeout: 5000
      """
    Then the response status code should be 201
    And I send a "GET" request to "/url-plaintext/v1.0/health" until status 200

    # Plain text with valid URL
    When I set header "Content-Type" to "text/plain"
    And I send a "POST" request to "/url-plaintext/v1.0/validate" with body:
      """
      Please check http://testbench:3000/health for status
      """
    Then the response status code should be 200

    # Plain text with invalid URL
    When I set header "Content-Type" to "text/plain"
    And I send a "POST" request to "/url-plaintext/v1.0/validate" with body:
      """
      Please check http://nonexistent-host-abc.invalid/health
      """
    Then the response status code should be 422

    # Cleanup
    When I delete the API "url-plaintext-api"
    Then the response status code should be 200

  # ============================================================================
  # ERROR RESPONSE VALIDATION
  # ============================================================================

  Scenario: Verify complete error response structure
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: url-error-structure-api
      spec:
        displayName: URL Error Structure API
        version: v1.0
        context: /url-error-structure/$version
        upstream:
          main:
            url: http://testbench:3000/api/v1
        operations:
          - method: GET
            path: /health
          - method: POST
            path: /validate
            policies:
              - name: url-guardrail
                version: v1
                params:
                  request:
                    enabled: true
                    jsonPath: ""
                    showAssessment: true
                    timeout: 5000
      """
    Then the response status code should be 201
    And I send a "GET" request to "/url-error-structure/v1.0/health" until status 200

    # Trigger error and verify full response structure
    When I send a "POST" request to "/url-error-structure/v1.0/validate" with body:
      """
      Check this URL: http://invalid-domain-test123.invalid/test
      """
    Then the response status code should be 422
    And the response should be valid JSON
    And the JSON response field "type" should be "URL_GUARDRAIL"
    And the JSON response field "message.action" should be "GUARDRAIL_INTERVENED"
    And the JSON response field "message.interveningGuardrail" should be "url-guardrail"
    And the JSON response field "message.direction" should be "REQUEST"
    And the JSON response should have field "message.assessments"

    # Cleanup
    When I delete the API "url-error-structure-api"
    Then the response status code should be 200
