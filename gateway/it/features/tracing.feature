Feature: Distributed Tracing
  As a developer
  I want to ensure that API requests are traced
  So that I can observe the system behavior

  @config-tracing
  Scenario: API invocation generates a trace
    Given the Gateway is running with tracing enabled
    And I have a valid API Key for the "Sales API"
    When I send a GET request to "http://localhost:9090/api/v1/sales/orders"
    Then the response status code should be 200
    And I should see a trace for "Sales API" in the OpenTelemetry collector logs
