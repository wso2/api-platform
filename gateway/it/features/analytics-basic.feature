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
    And I wait for the endpoint "http://localhost:8080/analytics-dedup-proxy/chat/completions" to be ready with method "POST" and body '{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "Hello"}]}'

    And I wait 5 seconds for analytics to be published
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
    And the latest analytics event should have AI context field "provider" with value "openai"
    And the latest analytics event should have AI context field "model_type" with value "chat"

  # Moesif prices an LLM call from the event's top-level ai_context block. It is published
  # beside the aiMetadata and aiTokenUsage metadata keys, which stay as they are. The mock
  # upstream returns a fixed model and usage, so every published value is asserted exactly.
  Scenario: LLM provider invocation publishes the ai_context block
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: analytics-ai-context-provider
      spec:
        displayName: Analytics AI Context Provider
        version: v1.0
        template: openai
        context: /analytics-ai-context
        upstream:
          url: http://mock-openapi:4010/openai/v1
          auth:
            type: api-key
            header: Authorization
            value: Bearer sk-test-key
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/analytics-ai-context/chat/completions" to be ready with method "POST" and body '{"model": "gpt-4.1", "messages": [{"role": "user", "content": "Hello"}]}'

    And I wait 5 seconds for analytics to be published
    Given I reset the analytics collector
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/analytics-ai-context/chat/completions" with body:
      """
      {"model": "gpt-4.1", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And I wait 5 seconds for analytics to be published
    And the analytics collector should have received 1 event
    And the latest analytics event should have AI context field "provider" with value "openai"
    And the latest analytics event should have AI context field "model" with value "gpt-4.1-2025-04-14"
    And the latest analytics event should have AI context field "model_type" with value "chat"
    And the latest analytics event should have AI context field "usage.prompt_tokens" with value "19"
    And the latest analytics event should have AI context field "usage.completion_tokens" with value "10"
    And the latest analytics event should have AI context field "usage.total_tokens" with value "29"
    # Moesif computes cost itself; a cost_metrics object on the event would switch that off.
    And the latest analytics event should not have AI context field "cost_metrics"

    # An embeddings response reports no completion count: it is left out, never sent as 0.
    Given I reset the analytics collector
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/analytics-ai-context/embeddings" with body:
      """
      {"model": "text-embedding-ada-002", "input": "Hello"}
      """
    Then the response status code should be 200
    And I wait 5 seconds for analytics to be published
    And the analytics collector should have received 1 event
    And the latest analytics event should have AI context field "model" with value "text-embedding-ada-002"
    And the latest analytics event should have AI context field "model_type" with value "embeddings"
    And the latest analytics event should have AI context field "usage.prompt_tokens" with value "8"
    And the latest analytics event should have AI context field "usage.total_tokens" with value "8"
    And the latest analytics event should not have AI context field "usage.completion_tokens"
