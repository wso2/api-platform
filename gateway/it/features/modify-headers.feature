Feature: Modify Headers Policy Integration Tests
  Test the modify-headers policy for comprehensive header manipulation in request and response flows

  Background:
    Given the gateway is running

  # ========================================
  # Request Header Modifications
  # ========================================

#   Scenario: Set a single request header
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-set-request
#       basePath: /modify-headers-set-req
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: X-Custom-Header
#                     value: CustomValue
#       """
#     And I wait for the health endpoint to be ready
#     And I send a GET request to "/modify-headers-set-req/1.0.0/test"
#     Then the response status code should be 200
#     And the response should contain echoed header "x-custom-header" with value "CustomValue"

#   Scenario: Append values to a request header
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-append-request
#       basePath: /modify-headers-append-req
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 requestHeaders:
#                   - action: APPEND
#                     name: X-Forwarded-For
#                     value: gateway-proxy-1
#                   - action: APPEND
#                     name: X-Forwarded-For
#                     value: gateway-proxy-2
#       """
#     And I wait for the health endpoint to be ready
#     And I send a GET request to "/modify-headers-append-req/1.0.0/test"
#     Then the response status code should be 200
#     And the response should contain echoed header "x-forwarded-for" with both values "gateway-proxy-1" and "gateway-proxy-2"

#   Scenario: Delete a request header
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-delete-request
#       basePath: /modify-headers-delete-req
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 requestHeaders:
#                   - action: DELETE
#                     name: User-Agent
#       """
#     And I wait for the health endpoint to be ready
#     And I send a GET request to "/modify-headers-delete-req/1.0.0/test" with header "User-Agent: TestClient/1.0"
#     Then the response status code should be 200
#     And the response should not contain echoed header "user-agent"

#   Scenario: Multiple request header operations (SET, APPEND, DELETE)
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-multiple-request
#       basePath: /modify-headers-multiple-req
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: X-Request-ID
#                     value: req-12345
#                   - action: APPEND
#                     name: X-Tracking
#                     value: tracker-1
#                   - action: APPEND
#                     name: X-Tracking
#                     value: tracker-2
#                   - action: DELETE
#                     name: X-Internal-Token
#       """
#     And I wait for the health endpoint to be ready
#     And I send a GET request to "/modify-headers-multiple-req/1.0.0/test" with header "X-Internal-Token: secret-token"
#     Then the response status code should be 200
#     And the response should contain echoed header "x-request-id" with value "req-12345"
#     And the response should contain echoed header "x-tracking" with both values "tracker-1" and "tracker-2"
#     And the response should not contain echoed header "x-internal-token"

#   Scenario: SET replaces existing request header value
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-replace-request
#       basePath: /modify-headers-replace-req
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: Authorization
#                     value: Bearer new-token
#       """
#     And I wait for the health endpoint to be ready
#     And I send a GET request to "/modify-headers-replace-req/1.0.0/test" with header "Authorization: Bearer old-token"
#     Then the response status code should be 200
#     And the response should contain echoed header "authorization" with value "Bearer new-token"

#   Scenario: Header names are case-insensitive in request modifications
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-case-insensitive-req
#       basePath: /modify-headers-case-req
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: X-Custom-HEADER
#                     value: test-value
#       """
#     And I wait for the health endpoint to be ready
#     And I send a GET request to "/modify-headers-case-req/1.0.0/test"
#     Then the response status code should be 200
#     And the response should contain echoed header "x-custom-header" with value "test-value"

  # ========================================
  # Response Header Modifications
  # ========================================

#   Scenario: Set a single response header
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-set-response
#       basePath: /modify-headers-set-resp
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 responseHeaders:
#                   - action: SET
#                     name: X-Gateway-Response
#                     value: processed
#       """
#     And I wait for the health endpoint to be ready
#     And I send a GET request to "/modify-headers-set-resp/1.0.0/test"
#     Then the response status code should be 200
#     And the response should have header "X-Gateway-Response" with value "processed"

#   Scenario: Append values to a response header
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-append-response
#       basePath: /modify-headers-append-resp
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 responseHeaders:
#                   - action: APPEND
#                     name: X-Gateway-Chain
#                     value: gateway-1
#                   - action: APPEND
#                     name: X-Gateway-Chain
#                     value: gateway-2
#       """
#     And I wait for the health endpoint to be ready
#     And I send a GET request to "/modify-headers-append-resp/1.0.0/test"
#     Then the response status code should be 200
#     And the response should have header "X-Gateway-Chain" with values "gateway-1" and "gateway-2"

#   Scenario: Delete a response header
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-delete-response
#       basePath: /modify-headers-delete-resp
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 responseHeaders:
#                   - action: DELETE
#                     name: X-Echo-Response
#       """
#     And I wait for the health endpoint to be ready
#     And I send a GET request to "/modify-headers-delete-resp/1.0.0/test"
#     Then the response status code should be 200
#     And the response should not have header "X-Echo-Response"

#   Scenario: Multiple response header operations (SET, APPEND, DELETE)
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-multiple-response
#       basePath: /modify-headers-multiple-resp
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 responseHeaders:
#                   - action: SET
#                     name: X-Response-ID
#                     value: resp-67890
#                   - action: APPEND
#                     name: X-Process-Chain
#                     value: step-1
#                   - action: APPEND
#                     name: X-Process-Chain
#                     value: step-2
#                   - action: DELETE
#                     name: X-Internal-Debug
#       """
#     And I wait for the health endpoint to be ready
#     And I send a GET request to "/modify-headers-multiple-resp/1.0.0/test"
#     Then the response status code should be 200
#     And the response should have header "X-Response-ID" with value "resp-67890"
#     And the response should have header "X-Process-Chain" with values "step-1" and "step-2"
#     And the response should not have header "X-Internal-Debug"

  # ========================================
  # Combined Request and Response Modifications
  # ========================================

#   Scenario: Modify both request and response headers
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-both
#       basePath: /modify-headers-both
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: POST
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: X-Request-Modified
#                     value: "true"
#                   - action: DELETE
#                     name: X-Client-Secret
#                 responseHeaders:
#                   - action: SET
#                     name: X-Response-Modified
#                     value: "true"
#                   - action: DELETE
#                     name: X-Backend-Internal
#       """
#     And I wait for the health endpoint to be ready
#     And I send a POST request to "/modify-headers-both/1.0.0/test" with header "X-Client-Secret: secret123" and body:
#       """
#       {"test": "data"}
#       """
#     Then the response status code should be 200
#     And the response should contain echoed header "x-request-modified" with value "true"
#     And the response should not contain echoed header "x-client-secret"
#     And the response should have header "X-Response-Modified" with value "true"

  # ========================================
  # Security Headers Use Case
  # ========================================

#   Scenario: Add security headers to response
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-security
#       basePath: /modify-headers-security
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 responseHeaders:
#                   - action: SET
#                     name: X-Frame-Options
#                     value: DENY
#                   - action: SET
#                     name: X-Content-Type-Options
#                     value: nosniff
#                   - action: SET
#                     name: Strict-Transport-Security
#                     value: max-age=31536000
#                   - action: SET
#                     name: X-XSS-Protection
#                     value: 1; mode=block
#                   - action: DELETE
#                     name: Server
#                   - action: DELETE
#                     name: X-Powered-By
#       """
#     And I wait for the health endpoint to be ready
#     And I send a GET request to "/modify-headers-security/1.0.0/test"
#     Then the response status code should be 200
#     And the response should have header "X-Frame-Options" with value "DENY"
#     And the response should have header "X-Content-Type-Options" with value "nosniff"
#     And the response should have header "Strict-Transport-Security" with value "max-age=31536000"
#     And the response should have header "X-XSS-Protection" with value "1; mode=block"

  # ========================================
  # CORS Headers Use Case
  # ========================================

#   Scenario: Configure CORS headers on response
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-cors
#       basePath: /modify-headers-cors
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 responseHeaders:
#                   - action: SET
#                     name: Access-Control-Allow-Origin
#                     value: https://example.com
#                   - action: SET
#                     name: Access-Control-Allow-Methods
#                     value: GET, POST, PUT, DELETE
#                   - action: APPEND
#                     name: Access-Control-Allow-Headers
#                     value: Authorization
#                   - action: APPEND
#                     name: Access-Control-Allow-Headers
#                     value: Content-Type
#                   - action: SET
#                     name: Access-Control-Max-Age
#                     value: "3600"
#       """
#     And I wait for the health endpoint to be ready
#     And I send a GET request to "/modify-headers-cors/1.0.0/test"
#     Then the response status code should be 200
#     And the response should have header "Access-Control-Allow-Origin" with value "https://example.com"
#     And the response should have header "Access-Control-Allow-Methods" with value "GET, POST, PUT, DELETE"
#     And the response should have header "Access-Control-Allow-Headers" with values "Authorization" and "Content-Type"
#     And the response should have header "Access-Control-Max-Age" with value "3600"

  # ========================================
  # Rate Limit Headers Use Case
  # ========================================

#   Scenario: Add rate limit information to response headers
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-ratelimit
#       basePath: /modify-headers-ratelimit
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 responseHeaders:
#                   - action: SET
#                     name: X-RateLimit-Limit
#                     value: "1000"
#                   - action: SET
#                     name: X-RateLimit-Remaining
#                     value: "999"
#                   - action: SET
#                     name: X-RateLimit-Reset
#                     value: "1640995200"
#       """
#     And I wait for the health endpoint to be ready
#     And I send a GET request to "/modify-headers-ratelimit/1.0.0/test"
#     Then the response status code should be 200
#     And the response should have header "X-RateLimit-Limit" with value "1000"
#     And the response should have header "X-RateLimit-Remaining" with value "999"
#     And the response should have header "X-RateLimit-Reset" with value "1640995200"

  # ========================================
  # Edge Cases
  # ========================================

#   Scenario: SET with empty value creates header with empty string
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-empty-value
#       basePath: /modify-headers-empty
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: X-Empty-Header
#                     value: ""
#       """
#     And I wait for the health endpoint to be ready
#     And I send a GET request to "/modify-headers-empty/1.0.0/test"
#     Then the response status code should be 200
#     And the response should contain echoed header "x-empty-header" with value ""

#   Scenario: DELETE non-existent header does not cause error
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-delete-nonexistent
#       basePath: /modify-headers-delete-none
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 requestHeaders:
#                   - action: DELETE
#                     name: X-Does-Not-Exist
#       """
#     And I wait for the health endpoint to be ready
#     And I send a GET request to "/modify-headers-delete-none/1.0.0/test"
#     Then the response status code should be 200

#   Scenario: Header value with special characters
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-special-chars
#       basePath: /modify-headers-special
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: X-Special-Value
#                     value: "value with spaces, commas; semicolons: colons = equals"
#       """
#     And I wait for the health endpoint to be ready
#     And I send a GET request to "/modify-headers-special/1.0.0/test"
#     Then the response status code should be 200
#     And the response should contain echoed header "x-special-value" with value "value with spaces, commas; semicolons: colons = equals"

#   Scenario: Very long header value
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-long-value
#       basePath: /modify-headers-long
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: X-Long-Value
#                     value: "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Sed ut perspiciatis unde omnis iste natus error sit voluptatem accusantium doloremque laudantium, totam rem aperiam, eaque ipsa quae ab illo inventore veritatis et quasi architecto beatae vitae dicta sunt explicabo."
#       """
#     And I wait for the health endpoint to be ready
#     And I send a GET request to "/modify-headers-long/1.0.0/test"
#     Then the response status code should be 200
#     And the response should contain echoed header "x-long-value" containing "Lorem ipsum dolor sit amet"

#   Scenario: Only request headers specified (no response headers)
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-only-request
#       basePath: /modify-headers-only-req
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: X-Only-Request
#                     value: test-value
#       """
#     And I wait for the health endpoint to be ready
#     And I send a GET request to "/modify-headers-only-req/1.0.0/test"
#     Then the response status code should be 200
#     And the response should contain echoed header "x-only-request" with value "test-value"

#   Scenario: Only response headers specified (no request headers)
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-only-response
#       basePath: /modify-headers-only-resp
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 responseHeaders:
#                   - action: SET
#                     name: X-Only-Response
#                     value: test-value
#       """
#     And I wait for the health endpoint to be ready
#     And I send a GET request to "/modify-headers-only-resp/1.0.0/test"
#     Then the response status code should be 200
#     And the response should have header "X-Only-Response" with value "test-value"

  # ========================================
  # Custom Tracking Headers Use Case
  # ========================================

#   Scenario: Add custom tracking and correlation headers
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-tracking
#       basePath: /modify-headers-tracking
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: POST
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: X-Correlation-ID
#                     value: corr-abc123
#                   - action: SET
#                     name: X-Gateway-Version
#                     value: v2.0.0
#                   - action: APPEND
#                     name: X-Route-Path
#                     value: gateway
#                   - action: APPEND
#                     name: X-Route-Path
#                     value: policy-engine
#                 responseHeaders:
#                   - action: SET
#                     name: X-Response-Time
#                     value: 45ms
#                   - action: SET
#                     name: X-Gateway-ID
#                     value: gateway-instance-1
#       """
#     And I wait for the health endpoint to be ready
#     And I send a POST request to "/modify-headers-tracking/1.0.0/test" with body:
#       """
#       {"transaction": "payment"}
#       """
#     Then the response status code should be 200
#     And the response should contain echoed header "x-correlation-id" with value "corr-abc123"
#     And the response should contain echoed header "x-gateway-version" with value "v2.0.0"
#     And the response should contain echoed header "x-route-path" with both values "gateway" and "policy-engine"
#     And the response should have header "X-Response-Time" with value "45ms"
#     And the response should have header "X-Gateway-ID" with value "gateway-instance-1"

  # ========================================
  # Header Name with Underscores and Hyphens
  # ========================================

#   Scenario: Header names with underscores and hyphens
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-names
#       basePath: /modify-headers-names
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: X-Custom_Header-123
#                     value: test-value-1
#                   - action: SET
#                     name: X_Another-Custom_Header
#                     value: test-value-2
#       """
#     And I wait for the health endpoint to be ready
#     And I send a GET request to "/modify-headers-names/1.0.0/test"
#     Then the response status code should be 200
#     And the response should contain echoed header "x-custom_header-123" with value "test-value-1"
#     And the response should contain echoed header "x_another-custom_header" with value "test-value-2"

  # ========================================
  # Multiple SET operations on same header (last wins)
  # ========================================

#   Scenario: Multiple SET operations on same header - last one wins
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-multiple-set
#       basePath: /modify-headers-multi-set
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: X-Test-Header
#                     value: first-value
#                   - action: SET
#                     name: X-Test-Header
#                     value: second-value
#                   - action: SET
#                     name: X-Test-Header
#                     value: final-value
#       """
#     And I wait for the health endpoint to be ready
#     And I send a GET request to "/modify-headers-multi-set/1.0.0/test"
#     Then the response status code should be 200
#     And the response should contain echoed header "x-test-header" with value "final-value"

  # ========================================
  # Content-Type Preservation
  # ========================================

#   Scenario: Modify headers while preserving content-type
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-content-type
#       basePath: /modify-headers-content
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: POST
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: X-Custom-Request
#                     value: modified
#                 responseHeaders:
#                   - action: SET
#                     name: X-Custom-Response
#                     value: processed
#       """
#     And I wait for the health endpoint to be ready
#     And I send a POST request to "/modify-headers-content/1.0.0/test" with header "Content-Type: application/json" and body:
#       """
#       {"message": "test"}
#       """
#     Then the response status code should be 200
#     And the response should have header "Content-Type" containing "application/json"
#     And the response should have header "X-Custom-Response" with value "processed"

  # ========================================
  # Authorization Header Modification
  # ========================================

#   Scenario: Replace authorization header for backend
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-auth
#       basePath: /modify-headers-auth
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: Authorization
#                     value: Bearer backend-service-token-xyz
#       """
#     And I wait for the health endpoint to be ready
#     And I send a GET request to "/modify-headers-auth/1.0.0/test" with header "Authorization: Bearer client-token-abc"
#     Then the response status code should be 200
#     And the response should contain echoed header "authorization" with value "Bearer backend-service-token-xyz"

  # ========================================
  # API Versioning Headers
  # ========================================

#   Scenario: Add API version headers to request and response
#     When I deploy an API with the following configuration:
#       """
#       name: test-modify-headers-versioning
#       basePath: /modify-headers-version
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: modify-headers
#               version: v0
#               params:
#                 requestHeaders:
#                   - action: SET
#                     name: X-API-Version
#                     value: v1.0.0
#                   - action: SET
#                     name: X-Backend-Version
#                     value: v2.5.0
#                 responseHeaders:
#                   - action: SET
#                     name: X-Gateway-Version
#                     value: v1.2.3
#                   - action: SET
#                     name: X-API-Deprecated
#                     value: "false"
#       """
#     And I wait for the health endpoint to be ready
#     And I send a GET request to "/modify-headers-version/1.0.0/test"
#     Then the response status code should be 200
#     And the response should contain echoed header "x-api-version" with value "v1.0.0"
#     And the response should contain echoed header "x-backend-version" with value "v2.5.0"
#     And the response should have header "X-Gateway-Version" with value "v1.2.3"
#     And the response should have header "X-API-Deprecated" with value "false"
