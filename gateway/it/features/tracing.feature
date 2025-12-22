Feature: Distributed Tracing
  As a developer
  I want to ensure that API requests are traced
  So that I can observe the system behavior

  @config-tracing
  Scenario: API invocation generates a trace
    Given the Gateway is running with tracing enabled
    And I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: tracing-api-v1.0
      spec:
        displayName: Tracing-API
        version: v1.0
        context: /tracing/v1.0
        upstream:
          main:
            url: http://sample-backend:5000/api/v2
        operations:
          - method: GET
            path: /{country_code}/{city}
          - method: GET
            path: /alerts/active
          - method: POST
            path: /alerts/active
      """
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And I wait for 2 seconds
    When I send a GET request to "http://localhost:8080/tracing/v1.0/us/seattle"
    Then the response should be successful
    And the response should be valid JSON
    And the response body should contain "/api/v2/us/seattle"
    And I should see a trace for "Tracing-API" in the OpenTelemetry collector logs
