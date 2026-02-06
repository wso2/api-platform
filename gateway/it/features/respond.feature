Feature: Respond Policy Integration Tests
  Test the respond policy for returning immediate responses without calling the backend

  Background:
    Given the gateway services are running

  # ========================================
  # Basic Response Scenarios
  # ========================================

#   Scenario: Return simple 200 OK response with plain text body
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-simple-200
#       basePath: /respond-simple-200
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 200
#                 body: "OK"
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-simple-200/v0/health" to be ready
#     And I send a GET request to "/respond-simple-200/1.0.0/test"
#     Then the response status code should be 200
#     And the response body should contain "OK"

#   Scenario: Return 201 Created with JSON body and headers
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-201-json
#       basePath: /respond-201-json
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: POST
#           path: /test
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 201
#                 body: '{"id": 123, "name": "Created Resource", "status": "success"}'
#                 headers:
#                   - name: Content-Type
#                     value: application/json
#                   - name: Location
#                     value: /api/resource/123
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-201-json/v0/health" to be ready
#     And I send a POST request to "/respond-201-json/1.0.0/test" with body:
#       """
#       {"test": "data"}
#       """
#     Then the response status code should be 201
#     And the response body should contain "Created Resource"
#     And the response header "Content-Type" should be "application/json"
#     And the response header "Location" should be "/api/resource/123"

#   Scenario: Return 204 No Content with empty body
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-204-empty
#       basePath: /respond-204-empty
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: DELETE
#           path: /test
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 204
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-204-empty/v0/health" to be ready
#     And I send a DELETE request to "/respond-204-empty/1.0.0/test"
#     Then the response status code should be 204
#     And the response body should be empty

#   Scenario: Default status code is 200 when not specified
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-default-200
#       basePath: /respond-default-200
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 body: "Default response"
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-default-200/v0/health" to be ready
#     And I send a GET request to "/respond-default-200/1.0.0/test"
#     Then the response status code should be 200
#     And the response body should contain "Default response"

  # ========================================
  # Error Responses (4xx)
  # ========================================

#   Scenario: Return 400 Bad Request error
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-400-error
#       basePath: /respond-400-error
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: POST
#           path: /test
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 400
#                 body: '{"error": "Bad Request", "message": "Invalid input data"}'
#                 headers:
#                   - name: Content-Type
#                     value: application/json
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-400-error/v0/health" to be ready
#     And I send a POST request to "/respond-400-error/1.0.0/test" with body:
#       """
#       {"invalid": "data"}
#       """
#     Then the response status code should be 400
#     And the response body should contain "Bad Request"
#     And the response body should contain "Invalid input data"

#   Scenario: Return 401 Unauthorized error
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-401-error
#       basePath: /respond-401-error
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 401
#                 body: '{"error": "Unauthorized", "message": "Authentication required"}'
#                 headers:
#                   - name: Content-Type
#                     value: application/json
#                   - name: WWW-Authenticate
#                     value: Bearer realm="api"
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-401-error/v0/health" to be ready
#     And I send a GET request to "/respond-401-error/1.0.0/test"
#     Then the response status code should be 401
#     And the response body should contain "Unauthorized"
#     And the response header "WWW-Authenticate" should contain "Bearer"

#   Scenario: Return 403 Forbidden error
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-403-error
#       basePath: /respond-403-error
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 403
#                 body: '{"error": "Forbidden", "message": "Access denied to this resource"}'
#                 headers:
#                   - name: Content-Type
#                     value: application/json
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-403-error/v0/health" to be ready
#     And I send a GET request to "/respond-403-error/1.0.0/test"
#     Then the response status code should be 403
#     And the response body should contain "Forbidden"
#     And the response body should contain "Access denied"

#   Scenario: Return 404 Not Found error
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-404-error
#       basePath: /respond-404-error
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 404
#                 body: '{"error": "Not Found", "message": "The requested resource does not exist"}'
#                 headers:
#                   - name: Content-Type
#                     value: application/json
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-404-error/v0/health" to be ready
#     And I send a GET request to "/respond-404-error/1.0.0/test"
#     Then the response status code should be 404
#     And the response body should contain "Not Found"

#   Scenario: Return 429 Too Many Requests
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-429-ratelimit
#       basePath: /respond-429-ratelimit
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 429
#                 body: '{"error": "Too many requests", "message": "Rate limit exceeded"}'
#                 headers:
#                   - name: Content-Type
#                     value: application/json
#                   - name: Retry-After
#                     value: "60"
#                   - name: X-RateLimit-Limit
#                     value: "100"
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-429-ratelimit/v0/health" to be ready
#     And I send a GET request to "/respond-429-ratelimit/1.0.0/test"
#     Then the response status code should be 429
#     And the response body should contain "Rate limit exceeded"
#     And the response header "Retry-After" should be "60"
#     And the response header "X-RateLimit-Limit" should be "100"

  # ========================================
  # Server Error Responses (5xx)
  # ========================================

#   Scenario: Return 500 Internal Server Error
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-500-error
#       basePath: /respond-500-error
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 500
#                 body: '{"error": "Internal Server Error", "message": "An unexpected error occurred"}'
#                 headers:
#                   - name: Content-Type
#                     value: application/json
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-500-error/v0/health" to be ready
#     And I send a GET request to "/respond-500-error/1.0.0/test"
#     Then the response status code should be 500
#     And the response body should contain "Internal Server Error"

#   Scenario: Return 503 Service Unavailable (maintenance mode)
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-503-maintenance
#       basePath: /respond-503-maintenance
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 503
#                 body: '{"error": "Service Unavailable", "message": "System under maintenance. Please try again later."}'
#                 headers:
#                   - name: Content-Type
#                     value: application/json
#                   - name: Retry-After
#                     value: "3600"
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-503-maintenance/v0/health" to be ready
#     And I send a GET request to "/respond-503-maintenance/1.0.0/test"
#     Then the response status code should be 503
#     And the response body should contain "under maintenance"
#     And the response header "Retry-After" should be "3600"

  # ========================================
  # Redirect Responses (3xx)
  # ========================================

#   Scenario: Return 301 Moved Permanently redirect
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-301-redirect
#       basePath: /respond-301-redirect
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 301
#                 headers:
#                   - name: Location
#                     value: https://example.com/new-location
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-301-redirect/v0/health" to be ready
#     And I send a GET request to "/respond-301-redirect/1.0.0/test"
#     Then the response status code should be 301
#     And the response header "Location" should be "https://example.com/new-location"

#   Scenario: Return 302 Found temporary redirect
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-302-redirect
#       basePath: /respond-302-redirect
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 302
#                 headers:
#                   - name: Location
#                     value: /temporary-location
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-302-redirect/v0/health" to be ready
#     And I send a GET request to "/respond-302-redirect/1.0.0/test"
#     Then the response status code should be 302
#     And the response header "Location" should be "/temporary-location"

  # ========================================
  # Different Content Types
  # ========================================

#   Scenario: Return XML response
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-xml
#       basePath: /respond-xml
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 200
#                 body: '<?xml version="1.0"?><response><status>success</status><message>XML response</message></response>'
#                 headers:
#                   - name: Content-Type
#                     value: application/xml
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-xml/v0/health" to be ready
#     And I send a GET request to "/respond-xml/1.0.0/test"
#     Then the response status code should be 200
#     And the response body should contain "<status>success</status>"
#     And the response header "Content-Type" should be "application/xml"

#   Scenario: Return HTML response
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-html
#       basePath: /respond-html
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 200
#                 body: '<html><head><title>API Documentation</title></head><body><h1>Welcome</h1><p>This is a static HTML response.</p></body></html>'
#                 headers:
#                   - name: Content-Type
#                     value: text/html
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-html/v0/health" to be ready
#     And I send a GET request to "/respond-html/1.0.0/test"
#     Then the response status code should be 200
#     And the response body should contain "<h1>Welcome</h1>"
#     And the response header "Content-Type" should be "text/html"

#   Scenario: Return plain text response
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-text
#       basePath: /respond-text
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 200
#                 body: "This is a plain text response with multiple lines.\nLine 2\nLine 3"
#                 headers:
#                   - name: Content-Type
#                     value: text/plain
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-text/v0/health" to be ready
#     And I send a GET request to "/respond-text/1.0.0/test"
#     Then the response status code should be 200
#     And the response body should contain "plain text response"
#     And the response header "Content-Type" should be "text/plain"

  # ========================================
  # Real-World Use Cases
  # ========================================

#   Scenario: API mocking - return mocked user data
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-mock-user
#       basePath: /respond-mock-user
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /users/:id
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 200
#                 body: '{"id": 123, "name": "John Doe", "email": "john@example.com", "role": "admin"}'
#                 headers:
#                   - name: Content-Type
#                     value: application/json
#                   - name: X-Mock-Response
#                     value: "true"
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-mock-user/v0/health" to be ready
#     And I send a GET request to "/respond-mock-user/1.0.0/users/123"
#     Then the response status code should be 200
#     And the response body should contain "John Doe"
#     And the response body should contain "john@example.com"
#     And the response header "X-Mock-Response" should be "true"

#   Scenario: Deprecated API endpoint notice
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-deprecated
#       basePath: /respond-deprecated
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /v1/users
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 410
#                 body: '{"error": "Gone", "message": "This endpoint is deprecated. Please use /v2/users instead.", "migration_guide": "https://docs.example.com/migration"}'
#                 headers:
#                   - name: Content-Type
#                     value: application/json
#                   - name: X-API-Deprecated
#                     value: "true"
#                   - name: X-API-Sunset-Date
#                     value: "2026-12-31"
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-deprecated/v0/health" to be ready
#     And I send a GET request to "/respond-deprecated/1.0.0/v1/users"
#     Then the response status code should be 410
#     And the response body should contain "deprecated"
#     And the response body should contain "/v2/users"
#     And the response header "X-API-Deprecated" should be "true"

#   Scenario: Health check stub
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-health
#       basePath: /respond-health
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /health
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 200
#                 body: '{"status": "healthy", "version": "1.0.0", "uptime": 3600}'
#                 headers:
#                   - name: Content-Type
#                     value: application/json
#                   - name: Cache-Control
#                     value: no-cache
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-health/v0/health" to be ready
#     And I send a GET request to "/respond-health/1.0.0/health"
#     Then the response status code should be 200
#     And the response body should contain "healthy"
#     And the response header "Cache-Control" should be "no-cache"

#   Scenario: CORS preflight OPTIONS response
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-cors-preflight
#       basePath: /respond-cors
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: OPTIONS
#           path: /test
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 204
#                 headers:
#                   - name: Access-Control-Allow-Origin
#                     value: "*"
#                   - name: Access-Control-Allow-Methods
#                     value: GET, POST, PUT, DELETE, OPTIONS
#                   - name: Access-Control-Allow-Headers
#                     value: Content-Type, Authorization
#                   - name: Access-Control-Max-Age
#                     value: "86400"
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-cors/v0/health" to be ready
#     And I send an OPTIONS request to "/respond-cors/1.0.0/test"
#     Then the response status code should be 204
#     And the response header "Access-Control-Allow-Origin" should be "*"
#     And the response header "Access-Control-Allow-Methods" should be "GET, POST, PUT, DELETE, OPTIONS"
#     And the response header "Access-Control-Max-Age" should be "86400"

  # ========================================
  # Edge Cases
  # ========================================

#   Scenario: Response with no body and no headers
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-minimal
#       basePath: /respond-minimal
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 200
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-minimal/v0/health" to be ready
#     And I send a GET request to "/respond-minimal/1.0.0/test"
#     Then the response status code should be 200

#   Scenario: Response with empty body string
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-empty-body
#       basePath: /respond-empty-body
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 200
#                 body: ""
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-empty-body/v0/health" to be ready
#     And I send a GET request to "/respond-empty-body/1.0.0/test"
#     Then the response status code should be 200
#     And the response body should be empty

#   Scenario: Response with multiple custom headers
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-multiple-headers
#       basePath: /respond-multi-headers
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 200
#                 body: '{"message": "success"}'
#                 headers:
#                   - name: Content-Type
#                     value: application/json
#                   - name: X-Custom-Header-1
#                     value: value1
#                   - name: X-Custom-Header-2
#                     value: value2
#                   - name: X-Custom-Header-3
#                     value: value3
#                   - name: Cache-Control
#                     value: max-age=3600
#                   - name: X-Request-ID
#                     value: req-abc-123
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-multi-headers/v0/health" to be ready
#     And I send a GET request to "/respond-multi-headers/1.0.0/test"
#     Then the response status code should be 200
#     And the response header "X-Custom-Header-1" should be "value1"
#     And the response header "X-Custom-Header-2" should be "value2"
#     And the response header "X-Custom-Header-3" should be "value3"
#     And the response header "Cache-Control" should be "max-age=3600"
#     And the response header "X-Request-ID" should be "req-abc-123"

#   Scenario: Large JSON response body
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-large-json
#       basePath: /respond-large-json
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 200
#                 body: '{"users": [{"id": 1, "name": "User 1"}, {"id": 2, "name": "User 2"}, {"id": 3, "name": "User 3"}, {"id": 4, "name": "User 4"}, {"id": 5, "name": "User 5"}], "total": 5, "page": 1, "pageSize": 10, "metadata": {"timestamp": "2026-01-28T10:00:00Z", "version": "v1"}}'
#                 headers:
#                   - name: Content-Type
#                     value: application/json
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-large-json/v0/health" to be ready
#     And I send a GET request to "/respond-large-json/1.0.0/test"
#     Then the response status code should be 200
#     And the response body should contain "User 1"
#     And the response body should contain "User 5"
#     And the response body should contain "total"

#   Scenario: Response with special characters in body
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-special-chars
#       basePath: /respond-special-chars
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /test
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 200
#                 body: '{"message": "Special chars: <>&\"''\n\t", "emoji": "🎉✅❌", "unicode": "Hello 世界"}'
#                 headers:
#                   - name: Content-Type
#                     value: application/json; charset=utf-8
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-special-chars/v0/health" to be ready
#     And I send a GET request to "/respond-special-chars/1.0.0/test"
#     Then the response status code should be 200
#     And the response body should contain "Special chars"
#     And the response body should contain "🎉"

  # ========================================
  # Status Code Ranges
  # ========================================

#   Scenario: Return 100-level informational status code
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-100-continue
#       basePath: /respond-100-continue
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: POST
#           path: /test
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 102
#                 body: '{"status": "Processing"}'
#                 headers:
#                   - name: Content-Type
#                     value: application/json
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-100-continue/v0/health" to be ready
#     And I send a POST request to "/respond-100-continue/1.0.0/test" with body:
#       """
#       {"test": "data"}
#       """
#     Then the response status code should be 102

#   Scenario: Return custom error code 418 I'm a teapot
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-418-teapot
#       basePath: /respond-418-teapot
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: POST
#           path: /brew-coffee
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 418
#                 body: '{"error": "I am a teapot", "message": "This server refuses to brew coffee"}'
#                 headers:
#                   - name: Content-Type
#                     value: application/json
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-418-teapot/v0/health" to be ready
#     And I send a POST request to "/respond-418-teapot/1.0.0/brew-coffee" with body:
#       """
#       {"beverage": "coffee"}
#       """
#     Then the response status code should be 418
#     And the response body should contain "teapot"

  # ========================================
  # Content Negotiation
  # ========================================

#   Scenario: Return different response based on path but same policy
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-multi-path
#       basePath: /respond-multi-path
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /json
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 200
#                 body: '{"format": "json"}'
#                 headers:
#                   - name: Content-Type
#                     value: application/json
#         - method: GET
#           path: /xml
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 200
#                 body: '<?xml version="1.0"?><response><format>xml</format></response>'
#                 headers:
#                   - name: Content-Type
#                     value: application/xml
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-multi-path/v0/health" to be ready
#     And I send a GET request to "/respond-multi-path/1.0.0/json"
#     Then the response status code should be 200
#     And the response body should contain "json"
#     And I send a GET request to "/respond-multi-path/1.0.0/xml"
#     Then the response status code should be 200
#     And the response body should contain "<format>xml</format>"

  # ========================================
  # Caching Headers
  # ========================================

#   Scenario: Response with cache control headers
#     When I deploy an API with the following configuration:
#       """
#       name: test-respond-cache-control
#       basePath: /respond-cache
#       version: 1.0.0
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080/echo
#       operations:
#         - method: GET
#           path: /static
#           policies:
#             - name: respond
#               version: v0
#               params:
#                 statusCode: 200
#                 body: '{"data": "static content"}'
#                 headers:
#                   - name: Content-Type
#                     value: application/json
#                   - name: Cache-Control
#                     value: public, max-age=86400
#                   - name: ETag
#                     value: "abc123"
#                   - name: Expires
#                     value: "Tue, 28 Jan 2026 12:00:00 GMT"
#       """
#     And I wait for the endpoint "http://localhost:8080/respond-cache/v0/health" to be ready
#     And I send a GET request to "/respond-cache/1.0.0/static"
#     Then the response status code should be 200
#     And the response header "Cache-Control" should be "public, max-age=86400"
#     And the response header "ETag" should contain "abc123"
#     And the response header "Expires" should be "Tue, 28 Jan 2026 12:00:00 GMT"
