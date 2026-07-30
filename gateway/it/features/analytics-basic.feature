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
      apiVersion: gateway.api-platform.wso2.com/v1
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
    And I wait for the endpoint "http://localhost:8080/analytics-test/v1/info" to be ready
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
      apiVersion: gateway.api-platform.wso2.com/v1
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
    And I wait for the endpoint "http://localhost:8080/metadata-test/v2/data" to be ready with method "POST" and body '{"test": "data"}'
    When I send a POST request to "http://localhost:8080/metadata-test/v2/data" with body:
      """
      {"test": "data"}
      """
    Then the response status code should be 200
    And I wait 5 seconds for analytics to be published
    And the analytics collector should have received at least 1 event
    And the latest analytics event should have request URI "/metadata-test/v2/data"
    And the latest analytics event should have metadata field "apiContext" with value "/metadata-test/v2"
    And the latest analytics event should have metadata field "apiName" with value "Metadata Test API"
    And the latest analytics event should have metadata field "apiVersion" with value "v2"

  Scenario: Multiple requests generate multiple analytics events
    Given I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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
    And I wait for the endpoint "http://localhost:8080/multi-test/v1/ping" to be ready
    When I send a GET request to "http://localhost:8080/multi-test/v1/ping"
    And I send a GET request to "http://localhost:8080/multi-test/v1/ping"
    And I send a GET request to "http://localhost:8080/multi-test/v1/ping"
    Then the response status code should be 200
    And I wait 5 seconds for analytics to be published
    And the analytics collector should have received at least 3 events

  # An LLM proxy forwards to its provider over the gateway's internal loopback, so one client
  # call traverses the listener twice and the access-log service delivers two entries. Only the
  # proxy's own event may be published: it carries the user identity, and the loopback provider
  # hop is its anonymous duplicate. Asserts an EXACT count — "at least 1" would pass while
  # double-counting, which is the bug this guards.
  Scenario: LLM proxy invocation generates exactly one analytics event
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: analytics-dedup-provider
      spec:
        displayName: Analytics Dedup Provider
        version: v1.0
        template: openai
        context: /analytics-dedup-provider-ctx
        upstream:
          url: http://sample-backend:9080/analytics-dedup-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: analytics-dedup-proxy
      spec:
        displayName: Analytics Dedup Proxy
        version: v1.0
        context: /analytics-dedup-proxy
        provider:
          id: analytics-dedup-provider
      """
    Then the response status should be 201
    And I wait for 3 seconds

    # Reset after deployment so the count covers only the single invocation below. Readiness
    # probes and deployment traffic would otherwise be counted too.
    Given I reset the analytics collector
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/analytics-dedup-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And I wait 5 seconds for analytics to be published
    And the analytics collector should have received 1 event
    # The surviving event must be the proxy's own, identified two ways: its request URI is the
    # proxy context (not the provider's loopback context) and its api kind is LlmProxy.
    And the latest analytics event should have request URI "/analytics-dedup-proxy/chat/completions"
    And the latest analytics event should have metadata field "apiType" with value "LlmProxy"
