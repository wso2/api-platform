Feature: Respond Policy Integration Tests
  Test the respond policy for returning immediate responses without calling the backend

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ========================================
  # Basic Response Scenarios
  # ========================================

  Scenario: Return simple 200 OK response with plain text body
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-simple-200
      spec:
        displayName: Respond-Simple-200-Test
        version: v1.0.0
        context: /respond-simple-200/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: "OK"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-simple-200/v1.0.0/test" to be ready
    When I send a GET request to "http://localhost:8080/respond-simple-200/v1.0.0/test"
    Then the response status code should be 200
    And the response body should contain "OK"

  Scenario: Return 201 Created with JSON body and headers
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-201-json
      spec:
        displayName: Respond-201-Created-Test
        version: v1.0.0
        context: /respond-201-json/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: "OK"
          - method: POST
            path: /test
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 201
                  body: '{"id": 123, "name": "Created Resource", "status": "success"}'
                  headers:
                    - name: Content-Type
                      value: application/json
                    - name: Location
                      value: /api/resource/123
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-201-json/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/respond-201-json/v1.0.0/test" with body:
      """
      {"test": "data"}
      """
    Then the response status code should be 201
    And the response body should contain "Created Resource"
    And the response header "Content-Type" should be "application/json"
    And the response header "Location" should be "/api/resource/123"

  Scenario: Return 204 No Content with empty body
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-204-empty
      spec:
        displayName: Respond-204-NoContent-Test
        version: v1.0.0
        context: /respond-204-empty/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: "OK"
          - method: DELETE
            path: /test
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 204
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-204-empty/v1.0.0/health" to be ready
    When I send a DELETE request to "http://localhost:8080/respond-204-empty/v1.0.0/test"
    Then the response status code should be 204
    And the response body should be empty

  Scenario: Default status code is 200 when not specified
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-default-200
      spec:
        displayName: Respond-Default-200-Test
        version: v1.0.0
        context: /respond-default-200/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
            policies:
              - name: respond
                version: v1
                params:
                  body: "Default response"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-default-200/v1.0.0/test" to be ready
    When I send a GET request to "http://localhost:8080/respond-default-200/v1.0.0/test"
    Then the response status code should be 200
    And the response body should contain "Default response"

  # ========================================
  # Error Responses (4xx)
  # ========================================

  Scenario: Return 400 Bad Request error
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-400-error
      spec:
        displayName: Respond-400-BadRequest-Test
        version: v1.0.0
        context: /respond-400-error/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: "OK"
          - method: POST
            path: /test
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 400
                  body: '{"error": "Bad Request", "message": "Invalid input data"}'
                  headers:
                    - name: Content-Type
                      value: application/json
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-400-error/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/respond-400-error/v1.0.0/test" with body:
      """
      {"invalid": "data"}
      """
    Then the response status code should be 400
    And the response body should contain "Bad Request"
    And the response body should contain "Invalid input data"

  Scenario: Return 401 Unauthorized error
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-401-error
      spec:
        displayName: Respond-401-Unauthorized-Test
        version: v1.0.0
        context: /respond-401-error/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: "OK"
          - method: GET
            path: /test
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 401
                  body: '{"error": "Unauthorized", "message": "Authentication required"}'
                  headers:
                    - name: Content-Type
                      value: application/json
                    - name: WWW-Authenticate
                      value: Bearer realm="api"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-401-error/v1.0.0/health" to be ready
    When I send a GET request to "http://localhost:8080/respond-401-error/v1.0.0/test"
    Then the response status code should be 401
    And the response body should contain "Unauthorized"
    And the response header "WWW-Authenticate" should contain "Bearer"

  Scenario: Return 403 Forbidden error
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-403-error
      spec:
        displayName: Respond-403-Forbidden-Test
        version: v1.0.0
        context: /respond-403-error/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: "OK"
          - method: GET
            path: /test
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 403
                  body: '{"error": "Forbidden", "message": "Access denied to this resource"}'
                  headers:
                    - name: Content-Type
                      value: application/json
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-403-error/v1.0.0/health" to be ready
    When I send a GET request to "http://localhost:8080/respond-403-error/v1.0.0/test"
    Then the response status code should be 403
    And the response body should contain "Forbidden"
    And the response body should contain "Access denied"

  Scenario: Return 404 Not Found error
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-404-error
      spec:
        displayName: Respond-404-NotFound-Test
        version: v1.0.0
        context: /respond-404-error/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: "OK"
          - method: GET
            path: /test
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 404
                  body: '{"error": "Not Found", "message": "The requested resource does not exist"}'
                  headers:
                    - name: Content-Type
                      value: application/json
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-404-error/v1.0.0/health" to be ready
    When I send a GET request to "http://localhost:8080/respond-404-error/v1.0.0/test"
    Then the response status code should be 404
    And the response body should contain "Not Found"

  Scenario: Return 429 Too Many Requests
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-429-ratelimit
      spec:
        displayName: Respond-429-RateLimit-Test
        version: v1.0.0
        context: /respond-429-ratelimit/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: "OK"
          - method: GET
            path: /test
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 429
                  body: '{"error": "Too many requests", "message": "Rate limit exceeded"}'
                  headers:
                    - name: Content-Type
                      value: application/json
                    - name: Retry-After
                      value: "60"
                    - name: X-RateLimit-Limit
                      value: "100"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-429-ratelimit/v1.0.0/health" to be ready
    When I send a GET request to "http://localhost:8080/respond-429-ratelimit/v1.0.0/test"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"
    And the response header "Retry-After" should be "60"
    And the response header "X-RateLimit-Limit" should be "100"

  # ========================================
  # Server Error Responses (5xx)
  # ========================================

  Scenario: Return 500 Internal Server Error
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-500-error
      spec:
        displayName: Respond-500-InternalError-Test
        version: v1.0.0
        context: /respond-500-error/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: "OK"
          - method: GET
            path: /test
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 500
                  body: '{"error": "Internal Server Error", "message": "An unexpected error occurred"}'
                  headers:
                    - name: Content-Type
                      value: application/json
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-500-error/v1.0.0/health" to be ready
    When I send a GET request to "http://localhost:8080/respond-500-error/v1.0.0/test"
    Then the response status code should be 500
    And the response body should contain "Internal Server Error"

  Scenario: Return 503 Service Unavailable (maintenance mode)
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-503-maintenance
      spec:
        displayName: Respond-503-Maintenance-Test
        version: v1.0.0
        context: /respond-503-maintenance/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: "OK"
          - method: GET
            path: /test
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 503
                  body: '{"error": "Service Unavailable", "message": "System under maintenance. Please try again later."}'
                  headers:
                    - name: Content-Type
                      value: application/json
                    - name: Retry-After
                      value: "3600"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-503-maintenance/v1.0.0/health" to be ready
    When I send a GET request to "http://localhost:8080/respond-503-maintenance/v1.0.0/test"
    Then the response status code should be 503
    And the response body should contain "under maintenance"
    And the response header "Retry-After" should be "3600"

  # ========================================
  # Redirect Responses (3xx)
  # ========================================

  Scenario: Return 301 Moved Permanently redirect
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-301-redirect
      spec:
        displayName: Respond-301-Redirect-Test
        version: v1.0.0
        context: /respond-301-redirect/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: "OK"
          - method: GET
            path: /test
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 301
                  headers:
                    - name: Location
                      value: https://example.com/new-location
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-301-redirect/v1.0.0/health" to be ready
    When I send a GET request to "http://localhost:8080/respond-301-redirect/v1.0.0/test"
    Then the response status code should be 301
    And the response header "Location" should be "https://example.com/new-location"

  Scenario: Return 302 Found temporary redirect
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-302-redirect
      spec:
        displayName: Respond-302-Redirect-Test
        version: v1.0.0
        context: /respond-302-redirect/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: "OK"
          - method: GET
            path: /test
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 302
                  headers:
                    - name: Location
                      value: /temporary-location
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-302-redirect/v1.0.0/health" to be ready
    When I send a GET request to "http://localhost:8080/respond-302-redirect/v1.0.0/test"
    Then the response status code should be 302
    And the response header "Location" should be "/temporary-location"

  # ========================================
  # Different Content Types
  # ========================================

  Scenario: Return XML response
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-xml
      spec:
        displayName: Respond-XML-Test
        version: v1.0.0
        context: /respond-xml/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: '<?xml version="1.0"?><response><status>success</status><message>XML response</message></response>'
                  headers:
                    - name: Content-Type
                      value: application/xml
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-xml/v1.0.0/test" to be ready
    When I send a GET request to "http://localhost:8080/respond-xml/v1.0.0/test"
    Then the response status code should be 200
    And the response body should contain "<status>success</status>"
    And the response header "Content-Type" should be "application/xml"

  Scenario: Return HTML response
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-html
      spec:
        displayName: Respond-HTML-Test
        version: v1.0.0
        context: /respond-html/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: '<html><head><title>API Documentation</title></head><body><h1>Welcome</h1><p>This is a static HTML response.</p></body></html>'
                  headers:
                    - name: Content-Type
                      value: text/html
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-html/v1.0.0/test" to be ready
    When I send a GET request to "http://localhost:8080/respond-html/v1.0.0/test"
    Then the response status code should be 200
    And the response body should contain "<h1>Welcome</h1>"
    And the response header "Content-Type" should be "text/html"

  Scenario: Return plain text response
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-text
      spec:
        displayName: Respond-Text-Test
        version: v1.0.0
        context: /respond-text/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: "This is a plain text response with multiple lines.\nLine 2\nLine 3"
                  headers:
                    - name: Content-Type
                      value: text/plain
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-text/v1.0.0/test" to be ready
    When I send a GET request to "http://localhost:8080/respond-text/v1.0.0/test"
    Then the response status code should be 200
    And the response body should contain "plain text response"
    And the response header "Content-Type" should be "text/plain"

  # ========================================
  # Real-World Use Cases
  # ========================================

  Scenario: API mocking - return mocked user data
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-mock-user
      spec:
        displayName: Respond-Mock-User-Test
        version: v1.0.0
        context: /respond-mock-user/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: "OK"
          - method: GET
            path: /users/{id}
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: '{"id": 123, "name": "John Doe", "email": "john@example.com", "role": "admin"}'
                  headers:
                    - name: Content-Type
                      value: application/json
                    - name: X-Mock-Response
                      value: "true"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-mock-user/v1.0.0/health" to be ready
    When I send a GET request to "http://localhost:8080/respond-mock-user/v1.0.0/users/123"
    Then the response status code should be 200
    And the response body should contain "John Doe"
    And the response body should contain "john@example.com"
    And the response header "X-Mock-Response" should be "true"

  Scenario: Deprecated API endpoint notice
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-deprecated
      spec:
        displayName: Respond-Deprecated-Test
        version: v1.0.0
        context: /respond-deprecated/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: "OK"
          - method: GET
            path: /v1/users
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 410
                  body: '{"error": "Gone", "message": "This endpoint is deprecated. Please use /v2/users instead.", "migration_guide": "https://docs.example.com/migration"}'
                  headers:
                    - name: Content-Type
                      value: application/json
                    - name: X-API-Deprecated
                      value: "true"
                    - name: X-API-Sunset-Date
                      value: "2026-12-31"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-deprecated/v1.0.0/health" to be ready
    When I send a GET request to "http://localhost:8080/respond-deprecated/v1.0.0/v1/users"
    Then the response status code should be 410
    And the response body should contain "deprecated"
    And the response body should contain "/v2/users"
    And the response header "X-API-Deprecated" should be "true"

  Scenario: Health check stub
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-health
      spec:
        displayName: Respond-Health-Test
        version: v1.0.0
        context: /respond-health/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: '{"status": "healthy", "version": "1.0.0", "uptime": 3600}'
                  headers:
                    - name: Content-Type
                      value: application/json
                    - name: Cache-Control
                      value: no-cache
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-health/v1.0.0/health" to be ready
    When I send a GET request to "http://localhost:8080/respond-health/v1.0.0/health"
    Then the response status code should be 200
    And the response body should contain "healthy"
    And the response header "Cache-Control" should be "no-cache"

  Scenario: CORS preflight OPTIONS response
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-cors-preflight
      spec:
        displayName: Respond-CORS-Test
        version: v1.0.0
        context: /respond-cors/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: "OK"
          - method: OPTIONS
            path: /test
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 204
                  headers:
                    - name: Access-Control-Allow-Origin
                      value: "*"
                    - name: Access-Control-Allow-Methods
                      value: GET, POST, PUT, DELETE, OPTIONS
                    - name: Access-Control-Allow-Headers
                      value: Content-Type, Authorization
                    - name: Access-Control-Max-Age
                      value: "86400"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-cors/v1.0.0/health" to be ready
    When I send an OPTIONS request to "http://localhost:8080/respond-cors/v1.0.0/test"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should be "*"
    And the response header "Access-Control-Allow-Methods" should be "GET, POST, PUT, DELETE, OPTIONS"
    And the response header "Access-Control-Max-Age" should be "86400"

  # ========================================
  # Edge Cases
  # ========================================

  Scenario: Response with no body and no headers
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-minimal
      spec:
        displayName: Respond-Minimal-Test
        version: v1.0.0
        context: /respond-minimal/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-minimal/v1.0.0/test" to be ready
    When I send a GET request to "http://localhost:8080/respond-minimal/v1.0.0/test"
    Then the response status code should be 200

  Scenario: Response with empty body string
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-empty-body
      spec:
        displayName: Respond-Empty-Body-Test
        version: v1.0.0
        context: /respond-empty-body/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: ""
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-empty-body/v1.0.0/test" to be ready
    When I send a GET request to "http://localhost:8080/respond-empty-body/v1.0.0/test"
    Then the response status code should be 200
    And the response body should be empty

  Scenario: Response with multiple custom headers
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-multiple-headers
      spec:
        displayName: Respond-Multiple-Headers-Test
        version: v1.0.0
        context: /respond-multi-headers/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: '{"message": "success"}'
                  headers:
                    - name: Content-Type
                      value: application/json
                    - name: X-Custom-Header-1
                      value: value1
                    - name: X-Custom-Header-2
                      value: value2
                    - name: X-Custom-Header-3
                      value: value3
                    - name: Cache-Control
                      value: max-age=3600
                    - name: X-Request-ID
                      value: req-abc-123
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-multi-headers/v1.0.0/test" to be ready
    When I send a GET request to "http://localhost:8080/respond-multi-headers/v1.0.0/test"
    Then the response status code should be 200
    And the response header "X-Custom-Header-1" should be "value1"
    And the response header "X-Custom-Header-2" should be "value2"
    And the response header "X-Custom-Header-3" should be "value3"
    And the response header "Cache-Control" should be "max-age=3600"
    And the response header "X-Request-ID" should be "req-abc-123"

  Scenario: Large JSON response body
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-large-json
      spec:
        displayName: Respond-Large-JSON-Test
        version: v1.0.0
        context: /respond-large-json/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: '{"users": [{"id": 1, "name": "User 1"}, {"id": 2, "name": "User 2"}, {"id": 3, "name": "User 3"}, {"id": 4, "name": "User 4"}, {"id": 5, "name": "User 5"}], "total": 5, "page": 1, "pageSize": 10, "metadata": {"timestamp": "2026-01-28T10:00:00Z", "version": "v1"}}'
                  headers:
                    - name: Content-Type
                      value: application/json
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-large-json/v1.0.0/test" to be ready
    When I send a GET request to "http://localhost:8080/respond-large-json/v1.0.0/test"
    Then the response status code should be 200
    And the response body should contain "User 1"
    And the response body should contain "User 5"
    And the response body should contain "total"

  Scenario: Response with special characters in body
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-special-chars
      spec:
        displayName: Respond-Special-Chars-Test
        version: v1.0.0
        context: /respond-special-chars/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: '{"message": "Special chars: <>&\"''\n\t", "emoji": "🎉✅❌", "unicode": "Hello 世界"}'
                  headers:
                    - name: Content-Type
                      value: application/json; charset=utf-8
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-special-chars/v1.0.0/test" to be ready
    When I send a GET request to "http://localhost:8080/respond-special-chars/v1.0.0/test"
    Then the response status code should be 200
    And the response body should contain "Special chars"
    And the response body should contain "🎉"

  # ========================================
  # Status Code Ranges
  # ========================================

  Scenario: Return 206 Partial Content status code
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-206-partial
      spec:
        displayName: Respond-206-Partial-Test
        version: v1.0.0
        context: /respond-206-partial/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: "OK"
          - method: POST
            path: /test
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 206
                  body: '{"status": "Partial Content", "range": "bytes 0-1023/2048"}'
                  headers:
                    - name: Content-Type
                      value: application/json
                    - name: Content-Range
                      value: "bytes 0-1023/2048"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-206-partial/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/respond-206-partial/v1.0.0/test" with body:
      """
      {"test": "data"}
      """
    Then the response status code should be 206
    And the response body should contain "Partial Content"
    And the response header "Content-Range" should be "bytes 0-1023/2048"

  Scenario: Return custom error code 418 I'm a teapot
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-418-teapot
      spec:
        displayName: Respond-418-Teapot-Test
        version: v1.0.0
        context: /respond-418-teapot/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: "OK"
          - method: POST
            path: /brew-coffee
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 418
                  body: '{"error": "I am a teapot", "message": "This server refuses to brew coffee"}'
                  headers:
                    - name: Content-Type
                      value: application/json
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-418-teapot/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/respond-418-teapot/v1.0.0/brew-coffee" with body:
      """
      {"beverage": "coffee"}
      """
    Then the response status code should be 418
    And the response body should contain "teapot"

  # ========================================
  # Content Negotiation
  # ========================================

  Scenario: Return different response based on path but same policy
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-multi-path
      spec:
        displayName: Respond-Multi-Path-Test
        version: v1.0.0
        context: /respond-multi-path/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /json
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: '{"format": "json"}'
                  headers:
                    - name: Content-Type
                      value: application/json
          - method: GET
            path: /xml
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: '<?xml version="1.0"?><response><format>xml</format></response>'
                  headers:
                    - name: Content-Type
                      value: application/xml
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-multi-path/v1.0.0/json" to be ready
    When I send a GET request to "http://localhost:8080/respond-multi-path/v1.0.0/json"
    Then the response status code should be 200
    And the response body should contain "json"
    When I send a GET request to "http://localhost:8080/respond-multi-path/v1.0.0/xml"
    Then the response status code should be 200
    And the response body should contain "<format>xml</format>"

  # ========================================
  # Caching Headers
  # ========================================

  Scenario: Response with cache control headers
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-respond-cache-control
      spec:
        displayName: Respond-Cache-Control-Test
        version: v1.0.0
        context: /respond-cache/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /static
            policies:
              - name: respond
                version: v1
                params:
                  statusCode: 200
                  body: '{"data": "static content"}'
                  headers:
                    - name: Content-Type
                      value: application/json
                    - name: Cache-Control
                      value: public, max-age=86400
                    - name: ETag
                      value: "abc123"
                    - name: Expires
                      value: "Tue, 28 Jan 2026 12:00:00 GMT"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/respond-cache/v1.0.0/static" to be ready
    When I send a GET request to "http://localhost:8080/respond-cache/v1.0.0/static"
    Then the response status code should be 200
    And the response header "Cache-Control" should be "public, max-age=86400"
    And the response header "ETag" should contain "abc123"
    And the response header "Expires" should be "Tue, 28 Jan 2026 12:00:00 GMT"
