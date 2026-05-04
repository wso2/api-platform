Feature: Interceptor Service Policy Integration Tests
  Validate interceptor-service policy for request/response mutation and direct responses

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Request interceptor mutates path, headers, and body before upstream call
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: interceptor-request-mutation-api
      spec:
        displayName: Interceptor Request Mutation API
        version: v1.0
        context: /interceptor-request/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: POST
            path: /mutate
            policies:
              - name: interceptor-service
                version: v1
                params:
                  endpoint: http://mock-interceptor-service:8080
                  request:
                    includeRequestHeaders: true
                    includeRequestBody: true
                    passthroughOnError: false
      """
    Then the response should be successful
    And I wait for policy snapshot sync
    And I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/interceptor-request/v1.0/mutate" with body:
      """
      {"client":"payload"}
      """
    Then the response status code should be 200
    And the JSON response field "url" should contain "/anything/intercepted"
    And the JSON response field "headers.X-Interceptor-Request" should be "true"
    And the JSON response field "data" should contain "mutated-by-interceptor"
    # Cleanup
    When I delete the API "interceptor-request-mutation-api"
    Then the response should be successful

  Scenario: Request interceptor short-circuits with direct response
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: interceptor-direct-respond-api
      spec:
        displayName: Interceptor Direct Respond API
        version: v1.0
        context: /interceptor-direct/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /block
            policies:
              - name: interceptor-service
                version: v1
                params:
                  endpoint: http://mock-interceptor-service:8080
                  request:
                    includeRequestHeaders: true
                    includeRequestBody: false
                    passthroughOnError: false
      """
    Then the response should be successful
    And I wait for policy snapshot sync
    When I send a GET request to "http://localhost:8080/interceptor-direct/v1.0/block"
    Then the response status code should be 403
    And the response header "X-Interceptor-Decision" should be "blocked"
    And the response body should contain "blocked by interceptor"
    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "interceptor-direct-respond-api"
    Then the response should be successful

  Scenario: Response interceptor rewrites status, headers, and body
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: interceptor-response-mutation-api
      spec:
        displayName: Interceptor Response Mutation API
        version: v1.0
        context: /interceptor-response/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /response-rewrite
            policies:
              - name: interceptor-service
                version: v1
                params:
                  endpoint: http://mock-interceptor-service:8080
                  request:
                    includeRequestHeaders: true
                    includeRequestBody: false
                    passthroughOnError: false
                  response:
                    includeRequestHeaders: true
                    includeRequestBody: false
                    includeResponseHeaders: true
                    includeResponseBody: true
                    passthroughOnError: false
      """
    Then the response should be successful
    And I wait for policy snapshot sync
    When I send a GET request to "http://localhost:8080/interceptor-response/v1.0/response-rewrite"
    Then the response status code should be 202
    And the response header "X-Interceptor-Response" should be "true"
    And the response header "X-Interceptor-Trace" should be "request-phase"
    And the response body should contain "response-overridden"
    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the API "interceptor-response-mutation-api"
    Then the response should be successful
