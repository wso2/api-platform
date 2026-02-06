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

  # ============================================================
  # Request Method Conditions
  # ============================================================

#   Scenario: Policy executes only on POST requests using method condition
#     When I deploy an API with the following configuration:
#       """
#       name: cel-method-condition-api
#       basePath: /cel-method-test
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /resource
#           policies:
#             - name: modify-headers
#               version: v0
#               executionCondition: 'request.Method == "POST"'
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: X-Cel-Executed
#                     value: "true"
#         - method: POST
#           path: /resource
#           policies:
#             - name: modify-headers
#               version: v0
#               executionCondition: 'request.Method == "POST"'
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: X-Cel-Executed
#                     value: "true"
#       """
#     And I wait for the endpoint "http://localhost:8080/cel-method-test/v0/health" to be ready
    # GET request - condition is false, policy should NOT execute
#     When I send a GET request to "/cel-method-test/1.0.0/resource"
#     Then the response status code should be 200
#     And the response should not contain echoed header "x-cel-executed"
    # POST request - condition is true, policy SHOULD execute
#     When I send a POST request to "/cel-method-test/1.0.0/resource" with body:
#       """
#       {"test": "data"}
#       """
#     Then the response status code should be 200
#     And the response should contain echoed header "x-cel-executed" with value "true"

#   Scenario: Policy executes on multiple methods using IN operator
#     When I deploy an API with the following configuration:
#       """
#       name: cel-multi-method-api
#       basePath: /cel-multi-method
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /data
#           policies:
#             - name: modify-headers
#               version: v0
#               executionCondition: 'request.Method in ["POST", "PUT", "DELETE"]'
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: X-Write-Operation
#                     value: "true"
#         - method: POST
#           path: /data
#           policies:
#             - name: modify-headers
#               version: v0
#               executionCondition: 'request.Method in ["POST", "PUT", "DELETE"]'
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: X-Write-Operation
#                     value: "true"
#         - method: PUT
#           path: /data
#           policies:
#             - name: modify-headers
#               version: v0
#               executionCondition: 'request.Method in ["POST", "PUT", "DELETE"]'
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: X-Write-Operation
#                     value: "true"
#       """
#     And I wait for the endpoint "http://localhost:8080/cel-multi-method/v0/health" to be ready
    # GET request - not in list, policy should NOT execute
#     When I send a GET request to "/cel-multi-method/1.0.0/data"
#     Then the response status code should be 200
#     And the response should not contain echoed header "x-write-operation"
    # POST request - in list, policy SHOULD execute
#     When I send a POST request to "/cel-multi-method/1.0.0/data" with body:
#       """
#       {"action": "create"}
#       """
#     Then the response status code should be 200
#     And the response should contain echoed header "x-write-operation" with value "true"
    # PUT request - in list, policy SHOULD execute
#     When I send a PUT request to "/cel-multi-method/1.0.0/data" with body:
#       """
#       {"action": "update"}
#       """
#     Then the response status code should be 200
#     And the response should contain echoed header "x-write-operation" with value "true"

  # ============================================================
  # Request Header Conditions
  # ============================================================

#   Scenario: Policy executes only when specific header is present
#     When I deploy an API with the following configuration:
#       """
#       name: cel-header-presence-api
#       basePath: /cel-header-presence
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /check
#           policies:
#             - name: modify-headers
#               version: v0
#               executionCondition: 'has(request.Headers["x-special-token"])'
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: X-Token-Validated
#                     value: "true"
#       """
#     And I wait for the endpoint "http://localhost:8080/cel-header-presence/v0/health" to be ready
    # Request without header - condition is false, policy should NOT execute
#     When I send a GET request to "/cel-header-presence/1.0.0/check"
#     Then the response status code should be 200
#     And the response should not contain echoed header "x-token-validated"
    # Request with header - condition is true, policy SHOULD execute
#     When I send a GET request to "/cel-header-presence/1.0.0/check" with header "X-Special-Token" value "my-secret-token"
#     Then the response status code should be 200
#     And the response should contain echoed header "x-token-validated" with value "true"

#   Scenario: Policy executes based on header value
#     When I deploy an API with the following configuration:
#       """
#       name: cel-header-value-api
#       basePath: /cel-header-value
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /premium
#           policies:
#             - name: modify-headers
#               version: v0
#               executionCondition: 'has(request.Headers["x-tier"]) && request.Headers["x-tier"][0] == "premium"'
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: X-Premium-Access
#                     value: "granted"
#       """
#     And I wait for the endpoint "http://localhost:8080/cel-header-value/v0/health" to be ready
    # Request with wrong tier value - condition is false
#     When I send a GET request to "/cel-header-value/1.0.0/premium" with header "X-Tier" value "basic"
#     Then the response status code should be 200
#     And the response should not contain echoed header "x-premium-access"
    # Request with premium tier - condition is true
#     When I send a GET request to "/cel-header-value/1.0.0/premium" with header "X-Tier" value "premium"
#     Then the response status code should be 200
#     And the response should contain echoed header "x-premium-access" with value "granted"

  # ============================================================
  # Request Path Conditions
  # ============================================================

#   Scenario: Policy executes based on path prefix
#     When I deploy an API with the following configuration:
#       """
#       name: cel-path-prefix-api
#       basePath: /cel-path-prefix
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /public/info
#           policies:
#             - name: modify-headers
#               version: v0
#               executionCondition: 'request.Path.startsWith("/cel-path-prefix/1.0.0/admin")'
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: X-Admin-Request
#                     value: "true"
#         - method: GET
#           path: /admin/settings
#           policies:
#             - name: modify-headers
#               version: v0
#               executionCondition: 'request.Path.startsWith("/cel-path-prefix/1.0.0/admin")'
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: X-Admin-Request
#                     value: "true"
#       """
#     And I wait for the endpoint "http://localhost:8080/cel-path-prefix/v0/health" to be ready
    # Public path - condition is false, policy should NOT execute
#     When I send a GET request to "/cel-path-prefix/1.0.0/public/info"
#     Then the response status code should be 200
#     And the response should not contain echoed header "x-admin-request"
    # Admin path - condition is true, policy SHOULD execute
#     When I send a GET request to "/cel-path-prefix/1.0.0/admin/settings"
#     Then the response status code should be 200
#     And the response should contain echoed header "x-admin-request" with value "true"

  # ============================================================
  # Combined Conditions
  # ============================================================

#   Scenario: Policy executes with combined method and header conditions
#     When I deploy an API with the following configuration:
#       """
#       name: cel-combined-condition-api
#       basePath: /cel-combined
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /secure
#           policies:
#             - name: modify-headers
#               version: v0
#               executionCondition: 'request.Method == "POST" && has(request.Headers["x-auth-token"])'
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: X-Secure-Write
#                     value: "authorized"
#         - method: POST
#           path: /secure
#           policies:
#             - name: modify-headers
#               version: v0
#               executionCondition: 'request.Method == "POST" && has(request.Headers["x-auth-token"])'
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: X-Secure-Write
#                     value: "authorized"
#       """
#     And I wait for the endpoint "http://localhost:8080/cel-combined/v0/health" to be ready
    # GET request - method condition false
#     When I send a GET request to "/cel-combined/1.0.0/secure"
#     Then the response status code should be 200
#     And the response should not contain echoed header "x-secure-write"
    # POST without auth header - header condition false
#     When I send a POST request to "/cel-combined/1.0.0/secure" with body:
#       """
#       {"data": "test"}
#       """
#     Then the response status code should be 200
#     And the response should not contain echoed header "x-secure-write"
    # POST with auth header - both conditions true
#     When I set header "X-Auth-Token" to "valid-token"
#     And I send a POST request to "/cel-combined/1.0.0/secure" with body:
#       """
#       {"data": "test"}
#       """
#     Then the response status code should be 200
#     And the response should contain echoed header "x-secure-write" with value "authorized"
#     And I clear all headers
