Feature: Analytics - Basic Event Capture
  As a platform administrator
  I want analytics events to be captured and published
  So that I can monitor API usage and performance

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I reset the analytics collector

  Scenario: REST API request generates analytics event
    Given I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: test-analytics-api
      spec:
        displayName: Test Analytics API
        version: v1
        context: /analytics-test/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /info
      """
    When I send a GET request to "http://localhost:8080/analytics-test/v1/info"
    Then the response status code should be 200
    And I wait 5 seconds for analytics to be published
    And the analytics collector should have received at least 1 event
    And the latest analytics event should have request URI "/analytics-test/v1/info"
    And the latest analytics event should have request method "GET"
    And the latest analytics event should have response status 200

  Scenario: Analytics event contains API metadata
    Given I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: metadata-test-api
      spec:
        displayName: Metadata Test API
        version: v2
        context: /metadata-test/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /data
      """
    When I send a POST request to "http://localhost:8080/metadata-test/v2/data" with body:
      """
      {"test": "data"}
      """
    Then the response status code should be 200
    And I wait 5 seconds for analytics to be published
    And the analytics collector should have received at least 1 event
    And the latest analytics event should have metadata field "apiContext" with value "/metadata-test/v2"
    And the latest analytics event should have metadata field "apiName" with value "Metadata Test API"
    And the latest analytics event should have metadata field "apiVersion" with value "v2"

  Scenario: Multiple requests generate multiple analytics events
    Given I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: multi-request-api
      spec:
        displayName: Multi Request API
        version: v1
        context: /multi-test/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /ping
      """
    When I send a GET request to "http://localhost:8080/multi-test/v1/ping"
    And I send a GET request to "http://localhost:8080/multi-test/v1/ping"
    And I send a GET request to "http://localhost:8080/multi-test/v1/ping"
    Then the response status code should be 200
    And I wait 5 seconds for analytics to be published
    And the analytics collector should have received at least 3 events
