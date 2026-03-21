Feature: Log Message Policy Integration Tests
  Test the log-message policy for logging request and response payloads and headers

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ========================================
  # Basic Logging Scenarios
  # ========================================

  Scenario: Log request payload and headers
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-log-message-request
      spec:
        displayName: Log-Message-Request-Test
        version: v1.0.0
        context: /log-req/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /test
            policies:
              - name: log-message
                version: v0
                params:
                  request:
                    payload: true
                    headers: true
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/log-req/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/log-req/v1.0.0/test" with body:
      """
      {"message": "hello world"}
      """
    Then the response status code should be 200
    And the response body should contain "hello world"

  Scenario: Log response payload and headers
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-log-message-response
      spec:
        displayName: Log-Message-Response-Test
        version: v1.0.0
        context: /log-resp/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /info
            policies:
              - name: log-message
                version: v0
                params:
                  response:
                    payload: true
                    headers: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/log-resp/v1.0.0/info" to be ready
    When I send a GET request to "http://localhost:8080/log-resp/v1.0.0/info"
    Then the response status code should be 200
    And the response should be valid JSON

  Scenario: Log both request and response
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-log-message-both
      spec:
        displayName: Log-Message-Both-Test
        version: v1.0.0
        context: /log-both/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /test
            policies:
              - name: log-message
                version: v0
                params:
                  request:
                    payload: true
                    headers: true
                  response:
                    payload: true
                    headers: true
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/log-both/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/log-both/v1.0.0/test" with body:
      """
      {"test": "data", "count": 42}
      """
    Then the response status code should be 200
    And the response body should contain "test"
    And the response body should contain "data"

  # ========================================
  # Headers Only Scenarios
  # ========================================

  Scenario: Log only request headers without payload
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-log-message-req-headers-only
      spec:
        displayName: Log-Message-Request-Headers-Only-Test
        version: v1.0.0
        context: /log-req-headers/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /info
            policies:
              - name: log-message
                version: v0
                params:
                  request:
                    headers: true
                    payload: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/log-req-headers/v1.0.0/info" to be ready
    When I send a GET request to "http://localhost:8080/log-req-headers/v1.0.0/info"
    Then the response status code should be 200

  Scenario: Log only response headers without payload
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-log-message-resp-headers-only
      spec:
        displayName: Log-Message-Response-Headers-Only-Test
        version: v1.0.0
        context: /log-resp-headers/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /info
            policies:
              - name: log-message
                version: v0
                params:
                  response:
                    headers: true
                    payload: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/log-resp-headers/v1.0.0/info" to be ready
    When I send a GET request to "http://localhost:8080/log-resp-headers/v1.0.0/info"
    Then the response status code should be 200

  # ========================================
  # Payload Only Scenarios
  # ========================================

  Scenario: Log only request payload without headers
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-log-message-req-payload-only
      spec:
        displayName: Log-Message-Request-Payload-Only-Test
        version: v1.0.0
        context: /log-req-payload/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /test
            policies:
              - name: log-message
                version: v0
                params:
                  request:
                    payload: true
                    headers: false
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/log-req-payload/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/log-req-payload/v1.0.0/test" with body:
      """
      {"payload": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "payload"

  Scenario: Log only response payload without headers
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-log-message-resp-payload-only
      spec:
        displayName: Log-Message-Response-Payload-Only-Test
        version: v1.0.0
        context: /log-resp-payload/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /info
            policies:
              - name: log-message
                version: v0
                params:
                  response:
                    payload: true
                    headers: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/log-resp-payload/v1.0.0/info" to be ready
    When I send a GET request to "http://localhost:8080/log-resp-payload/v1.0.0/info"
    Then the response status code should be 200

  # ========================================
  # Exclude Headers Scenarios
  # ========================================

  Scenario: Log request headers with exclusions
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-log-message-req-exclude
      spec:
        displayName: Log-Message-Request-Exclude-Test
        version: v1.0.0
        context: /log-req-exclude/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /info
            policies:
              - name: log-message
                version: v0
                params:
                  request:
                    headers: true
                    excludeHeaders:
                      - Authorization
                      - Cookie
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/log-req-exclude/v1.0.0/info" to be ready
    When I send a GET request to "http://localhost:8080/log-req-exclude/v1.0.0/info"
    Then the response status code should be 200

  Scenario: Log response headers with exclusions
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-log-message-resp-exclude
      spec:
        displayName: Log-Message-Response-Exclude-Test
        version: v1.0.0
        context: /log-resp-exclude/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /info
            policies:
              - name: log-message
                version: v0
                params:
                  response:
                    headers: true
                    excludeHeaders:
                      - Set-Cookie
                      - X-Internal-Id
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/log-resp-exclude/v1.0.0/info" to be ready
    When I send a GET request to "http://localhost:8080/log-resp-exclude/v1.0.0/info"
    Then the response status code should be 200

  # ========================================
  # Combined with Other Policies
  # ========================================

  Scenario: Log message policy combined with set-headers
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-log-message-combined
      spec:
        displayName: Log-Message-Combined-Test
        version: v1.0.0
        context: /log-combined/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: GET
            path: /test
            policies:
              - name: set-headers
                version: v0
                params:
                  request:
                    headers:
                      - name: X-Custom-Header
                        value: CustomValue
              - name: log-message
                version: v0
                params:
                  request:
                    headers: true
                    payload: false
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/log-combined/v1.0.0/test" to be ready
    When I send a GET request to "http://localhost:8080/log-combined/v1.0.0/test"
    Then the response status code should be 200
    And the response should contain echoed header "x-custom-header" with value "CustomValue"

  # ========================================
  # Multiple HTTP Methods
  # ========================================

  Scenario: Log message with different HTTP methods
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-log-message-methods
      spec:
        displayName: Log-Message-Methods-Test
        version: v1.0.0
        context: /log-methods/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /info
            policies:
              - name: log-message
                version: v0
                params:
                  request:
                    headers: true
          - method: POST
            path: /echo
            policies:
              - name: log-message
                version: v0
                params:
                  request:
                    payload: true
                    headers: true
          - method: PUT
            path: /echo
            policies:
              - name: log-message
                version: v0
                params:
                  request:
                    payload: true
                  response:
                    payload: true
          - method: DELETE
            path: /echo
            policies:
              - name: log-message
                version: v0
                params:
                  response:
                    headers: true
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/log-methods/v1.0.0/info" to be ready
    # Test GET
    When I send a GET request to "http://localhost:8080/log-methods/v1.0.0/info"
    Then the response status code should be 200
    # Test POST
    When I send a POST request to "http://localhost:8080/log-methods/v1.0.0/echo" with body:
      """
      {"method": "POST"}
      """
    Then the response status code should be 200
    # Test PUT
    When I send a PUT request to "http://localhost:8080/log-methods/v1.0.0/echo" with body:
      """
      {"method": "PUT"}
      """
    Then the response status code should be 200
    # Test DELETE
    When I send a DELETE request to "http://localhost:8080/log-methods/v1.0.0/echo"
    Then the response status code should be 200

  # ========================================
  # Large Payload Scenarios
  # ========================================

  Scenario: Log message with large request payload
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-log-message-large-payload
      spec:
        displayName: Log-Message-Large-Payload-Test
        version: v1.0.0
        context: /log-large/$version
        upstream:
          main:
            url: http://sample-backend:9080/echo
        operations:
          - method: POST
            path: /test
            policies:
              - name: log-message
                version: v0
                params:
                  request:
                    payload: true
                  response:
                    payload: true
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/log-large/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/log-large/v1.0.0/test" with body:
      """
      {
        "data": "This is a larger payload to test logging capabilities",
        "items": [
          {"id": 1, "name": "Item One", "description": "First item in the list"},
          {"id": 2, "name": "Item Two", "description": "Second item in the list"},
          {"id": 3, "name": "Item Three", "description": "Third item in the list"}
        ],
        "metadata": {
          "source": "integration-test",
          "timestamp": "2025-01-01T00:00:00Z",
          "version": "1.0.0"
        }
      }
      """
    Then the response status code should be 200
    And the response body should contain "Item One"
    And the response body should contain "Item Three"
