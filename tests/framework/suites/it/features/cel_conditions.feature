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

@cel @policy-conditions
Feature: CEL Execution Conditions
  As an API developer
  I want to use CEL expressions to conditionally execute policies
  So that I can apply policies based on request/response attributes

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ============================================================
  # Request Method Conditions
  # ============================================================

  Scenario: Policy executes only on POST requests using method condition
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: cel-method-condition-api-v1.0.0
      spec:
        displayName: CEL-Method-Condition-API
        version: v1.0.0
        context: /cel-method-test/$version
        upstream:
          main:
            url: http://testbench:3000/echo
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /resource
            policies:
              - name: set-headers
                version: v1
                executionCondition: 'request.Method == "POST"'
                params:
                  request:
                    headers:
                      - name: X-Cel-Executed
                        value: "true"
          - method: POST
            path: /resource
            policies:
              - name: set-headers
                version: v1
                executionCondition: 'request.Method == "POST"'
                params:
                  request:
                    headers:
                      - name: X-Cel-Executed
                        value: "true"
      """
    # GET request - condition is false, policy should NOT execute
    When I send a "GET" request to "/cel-method-test/v1.0.0/resource" until status 200
    Then the response status code should be 200
    And the response should not contain echoed header "x-cel-executed"
    # POST request - condition is true, policy SHOULD execute
    When I send a "POST" request to "/cel-method-test/v1.0.0/resource" with body:
      """
      {"test": "data"}
      """
    Then the response status code should be 200
    And the response should contain echoed header "x-cel-executed" with value "true"
    And I delete the API "cel-method-condition-api-v1.0.0"

  Scenario: Policy executes on multiple methods using IN operator
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: cel-multi-method-api-v1.0.0
      spec:
        displayName: CEL-Multi-Method-API
        version: v1.0.0
        context: /cel-multi-method/$version
        upstream:
          main:
            url: http://testbench:3000/echo
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /data
            policies:
              - name: set-headers
                version: v1
                executionCondition: 'request.Method in ["POST", "PUT", "DELETE"]'
                params:
                  request:
                    headers:
                      - name: X-Write-Operation
                        value: "true"
          - method: POST
            path: /data
            policies:
              - name: set-headers
                version: v1
                executionCondition: 'request.Method in ["POST", "PUT", "DELETE"]'
                params:
                  request:
                    headers:
                      - name: X-Write-Operation
                        value: "true"
          - method: PUT
            path: /data
            policies:
              - name: set-headers
                version: v1
                executionCondition: 'request.Method in ["POST", "PUT", "DELETE"]'
                params:
                  request:
                    headers:
                      - name: X-Write-Operation
                        value: "true"
      """
    # GET request - not in list, policy should NOT execute
    When I send a "GET" request to "/cel-multi-method/v1.0.0/data" until status 200
    Then the response status code should be 200
    And the response should not contain echoed header "x-write-operation"
    # POST request - in list, policy SHOULD execute
    When I send a "POST" request to "/cel-multi-method/v1.0.0/data" with body:
      """
      {"action": "create"}
      """
    Then the response status code should be 200
    And the response should contain echoed header "x-write-operation" with value "true"
    # PUT request - in list, policy SHOULD execute
    When I send a "PUT" request to "/cel-multi-method/v1.0.0/data" with body:
      """
      {"action": "update"}
      """
    Then the response status code should be 200
    And the response should contain echoed header "x-write-operation" with value "true"
    And I delete the API "cel-multi-method-api-v1.0.0"

  # ============================================================
  # Request Header Conditions
  # ============================================================

  Scenario: Policy executes only when specific header is present
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: cel-header-presence-api-v1.0.0
      spec:
        displayName: CEL-Header-Presence-API
        version: v1.0.0
        context: /cel-header-presence/$version
        upstream:
          main:
            url: http://testbench:3000/echo
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /check
            policies:
              - name: set-headers
                version: v1
                executionCondition: '"x-special-token" in request.Headers'
                params:
                  request:
                    headers:
                      - name: X-Token-Validated
                        value: "true"
      """
    # Request without header - condition is false, policy should NOT execute
    When I send a "GET" request to "/cel-header-presence/v1.0.0/check" until status 200
    Then the response status code should be 200
    And the response should not contain echoed header "x-token-validated"
    # Request with header - condition is true, policy SHOULD execute
    When I set header "X-Special-Token" to "my-secret-token"
    And I send a "GET" request to "/cel-header-presence/v1.0.0/check"
    Then the response status code should be 200
    And the response should contain echoed header "x-token-validated" with value "true"
    And I delete the API "cel-header-presence-api-v1.0.0"

  Scenario: Policy executes based on header value
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: cel-header-value-api-v1.0.0
      spec:
        displayName: CEL-Header-Value-API
        version: v1.0.0
        context: /cel-header-value/$version
        upstream:
          main:
            url: http://testbench:3000/echo
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /premium
            policies:
              - name: set-headers
                version: v1
                executionCondition: '"x-tier" in request.Headers && request.Headers["x-tier"][0] == "premium"'
                params:
                  request:
                    headers:
                      - name: X-Premium-Access
                        value: "granted"
      """
    And I send a "GET" request to "/cel-header-value/v1.0.0/health" until status 200
    # Request with wrong tier value - condition is false
    When I set header "X-Tier" to "basic"
    And I send a "GET" request to "/cel-header-value/v1.0.0/premium"
    Then the response status code should be 200
    And the response should not contain echoed header "x-premium-access"
    # Request with premium tier - condition is true
    When I set header "X-Tier" to "premium"
    And I send a "GET" request to "/cel-header-value/v1.0.0/premium"
    Then the response status code should be 200
    And the response should contain echoed header "x-premium-access" with value "granted"
    And I delete the API "cel-header-value-api-v1.0.0"

  # ============================================================
  # Request Path Conditions
  # ============================================================

  Scenario: Policy executes based on path prefix
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: cel-path-prefix-api-v1.0.0
      spec:
        displayName: CEL-Path-Prefix-API
        version: v1.0.0
        context: /cel-path-prefix/$version
        upstream:
          main:
            url: http://testbench:3000/echo
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /public/info
            policies:
              - name: set-headers
                version: v1
                executionCondition: 'request.Path.startsWith("/cel-path-prefix/v1.0.0/admin")'
                params:
                  request:
                    headers:
                      - name: X-Admin-Request
                        value: "true"
          - method: GET
            path: /admin/settings
            policies:
              - name: set-headers
                version: v1
                executionCondition: 'request.Path.startsWith("/cel-path-prefix/v1.0.0/admin")'
                params:
                  request:
                    headers:
                      - name: X-Admin-Request
                        value: "true"
      """
    # Public path - condition is false, policy should NOT execute
    When I send a "GET" request to "/cel-path-prefix/v1.0.0/public/info" until status 200
    Then the response status code should be 200
    And the response should not contain echoed header "x-admin-request"
    # Admin path - condition is true, policy SHOULD execute
    When I send a "GET" request to "/cel-path-prefix/v1.0.0/admin/settings"
    Then the response status code should be 200
    And the response should contain echoed header "x-admin-request" with value "true"
    And I delete the API "cel-path-prefix-api-v1.0.0"

  # ============================================================
  # Combined Conditions
  # ============================================================

  Scenario: Policy executes with combined method and header conditions
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: cel-combined-condition-api-v1.0.0
      spec:
        displayName: CEL-Combined-Condition-API
        version: v1.0.0
        context: /cel-combined/$version
        upstream:
          main:
            url: http://testbench:3000/echo
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /secure
            policies:
              - name: set-headers
                version: v1
                executionCondition: 'request.Method == "POST" && "x-auth-token" in request.Headers'
                params:
                  request:
                    headers:
                      - name: X-Secure-Write
                        value: "authorized"
          - method: POST
            path: /secure
            policies:
              - name: set-headers
                version: v1
                executionCondition: 'request.Method == "POST" && "x-auth-token" in request.Headers'
                params:
                  request:
                    headers:
                      - name: X-Secure-Write
                        value: "authorized"
      """
    # GET request - method condition false
    When I send a "GET" request to "/cel-combined/v1.0.0/secure" until status 200
    Then the response status code should be 200
    And the response should not contain echoed header "x-secure-write"
    # POST without auth header - header condition false
    When I send a "POST" request to "/cel-combined/v1.0.0/secure" with body:
      """
      {"data": "test"}
      """
    Then the response status code should be 200
    And the response should not contain echoed header "x-secure-write"
    # POST with auth header - both conditions true
    When I set header "X-Auth-Token" to "valid-token"
    And I send a "POST" request to "/cel-combined/v1.0.0/secure" with body:
      """
      {"data": "test"}
      """
    Then the response status code should be 200
    And the response should contain echoed header "x-secure-write" with value "authorized"
    And I reset the request
    And I delete the API "cel-combined-condition-api-v1.0.0"
