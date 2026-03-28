@model-weighted-round-robin
Feature: Model Weighted Round-Robin Load Balancing Policy
  Test the model-weighted-round-robin policy which distributes AI model requests
  based on configurable weight values, implementing weighted round-robin selection
  and automatic suspension on failures.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ====================================================================
  # BASIC WEIGHTED DISTRIBUTION - PAYLOAD LOCATION
  # ====================================================================

  Scenario: Basic weighted distribution with payload location
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: wrr-basic
      spec:
        displayName: WRR Basic
        version: v1.0.0
        context: /wrr-basic/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-weighted-round-robin
                version: v1
                params:
                  models:
                    - model: gpt-3.5-turbo
                      weight: 3
                    - model: gpt-4
                      weight: 1
                  requestModel:
                    location: payload
                    identifier: "$.model"
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/wrr-basic/v1.0.0/health" to be ready
    # First 3 requests should go to gpt-3.5-turbo (weight 3)
    When I send a POST request to "http://localhost:8080/wrr-basic/v1.0.0/chat" with body:
      """
      {"model": "original-model", "prompt": "Hello"}
      """
    Then the response status code should be 200
    And the response body should contain "gpt-3.5-turbo"
    When I send a POST request to "http://localhost:8080/wrr-basic/v1.0.0/chat" with body:
      """
      {"model": "original-model", "prompt": "Hello"}
      """
    Then the response status code should be 200
    And the response body should contain "gpt-3.5-turbo"
    When I send a POST request to "http://localhost:8080/wrr-basic/v1.0.0/chat" with body:
      """
      {"model": "original-model", "prompt": "Hello"}
      """
    Then the response status code should be 200
    And the response body should contain "gpt-3.5-turbo"
    # 4th request should go to gpt-4 (weight 1)
    When I send a POST request to "http://localhost:8080/wrr-basic/v1.0.0/chat" with body:
      """
      {"model": "original-model", "prompt": "Hello"}
      """
    Then the response status code should be 200
    And the response body should contain "gpt-4"
    # 5th request cycles back to gpt-3.5-turbo
    When I send a POST request to "http://localhost:8080/wrr-basic/v1.0.0/chat" with body:
      """
      {"model": "original-model", "prompt": "Hello"}
      """
    Then the response status code should be 200
    And the response body should contain "gpt-3.5-turbo"
    Given I authenticate using basic auth as "admin"
    When I delete the API "wrr-basic"
    Then the response should be successful

  Scenario: Equal weight distribution
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: wrr-equal
      spec:
        displayName: WRR Equal
        version: v1.0.0
        context: /wrr-equal/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-weighted-round-robin
                version: v1
                params:
                  models:
                    - model: model-a
                      weight: 1
                    - model: model-b
                      weight: 1
                  requestModel:
                    location: payload
                    identifier: "$.model"
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/wrr-equal/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/wrr-equal/v1.0.0/chat" with body:
      """
      {"model": "original", "data": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "model-a"
    When I send a POST request to "http://localhost:8080/wrr-equal/v1.0.0/chat" with body:
      """
      {"model": "original", "data": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "model-b"
    # Cycle wraps back to model-a
    When I send a POST request to "http://localhost:8080/wrr-equal/v1.0.0/chat" with body:
      """
      {"model": "original", "data": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "model-a"
    Given I authenticate using basic auth as "admin"
    When I delete the API "wrr-equal"
    Then the response should be successful

  Scenario: Three models with different weights
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: wrr-three
      spec:
        displayName: WRR Three
        version: v1.0.0
        context: /wrr-three/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-weighted-round-robin
                version: v1
                params:
                  models:
                    - model: fast-model
                      weight: 5
                    - model: balanced-model
                      weight: 3
                    - model: premium-model
                      weight: 2
                  requestModel:
                    location: payload
                    identifier: "$.model"
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/wrr-three/v1.0.0/health" to be ready
    # Total weight = 10, so sequence is: [fast x5, balanced x3, premium x2]
    When I send a POST request to "http://localhost:8080/wrr-three/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "fast-model"
    When I send a POST request to "http://localhost:8080/wrr-three/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "fast-model"
    When I send a POST request to "http://localhost:8080/wrr-three/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "fast-model"
    When I send a POST request to "http://localhost:8080/wrr-three/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "fast-model"
    When I send a POST request to "http://localhost:8080/wrr-three/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "fast-model"
    # Weight boundary: 6th request transitions to balanced-model
    When I send a POST request to "http://localhost:8080/wrr-three/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "balanced-model"
    When I send a POST request to "http://localhost:8080/wrr-three/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "balanced-model"
    When I send a POST request to "http://localhost:8080/wrr-three/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "balanced-model"
    # Weight boundary: 9th request transitions to premium-model
    When I send a POST request to "http://localhost:8080/wrr-three/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "premium-model"
    Given I authenticate using basic auth as "admin"
    When I delete the API "wrr-three"
    Then the response should be successful

  # ====================================================================
  # MODEL LOCATION: HEADER
  # ====================================================================

  Scenario: Model selection with header location
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: wrr-header
      spec:
        displayName: WRR Header
        version: v1.0.0
        context: /wrr-header/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-weighted-round-robin
                version: v1
                params:
                  models:
                    - model: model-a
                      weight: 1
                    - model: model-b
                      weight: 1
                  requestModel:
                    location: header
                    identifier: X-Model
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/wrr-header/v1.0.0/health" to be ready
    When I set header "X-Model" to "original-model"
    And I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/wrr-header/v1.0.0/chat" with body:
      """
      {"prompt": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "model-a"
    When I set header "X-Model" to "original-model"
    And I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/wrr-header/v1.0.0/chat" with body:
      """
      {"prompt": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "model-b"
    Given I authenticate using basic auth as "admin"
    When I delete the API "wrr-header"
    Then the response should be successful

  # ====================================================================
  # MODEL LOCATION: QUERY PARAMETER
  # ====================================================================

  Scenario: Model selection with query parameter location
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: wrr-query
      spec:
        displayName: WRR Query
        version: v1.0.0
        context: /wrr-query/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /chat
            policies:
              - name: model-weighted-round-robin
                version: v1
                params:
                  models:
                    - model: model-alpha
                      weight: 2
                    - model: model-beta
                      weight: 1
                  requestModel:
                    location: queryParam
                    identifier: model
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/wrr-query/v1.0.0/health" to be ready
    # Sequence: [model-alpha, model-alpha, model-beta]
    When I send a GET request to "http://localhost:8080/wrr-query/v1.0.0/chat?model=original-model&prompt=hello"
    Then the response status code should be 200
    And the response body should contain "model=model-alpha"
    When I send a GET request to "http://localhost:8080/wrr-query/v1.0.0/chat?model=original-model&prompt=hello"
    Then the response status code should be 200
    And the response body should contain "model=model-alpha"
    When I send a GET request to "http://localhost:8080/wrr-query/v1.0.0/chat?model=original-model&prompt=hello"
    Then the response status code should be 200
    And the response body should contain "model=model-beta"
    Given I authenticate using basic auth as "admin"
    When I delete the API "wrr-query"
    Then the response should be successful

  # ====================================================================
  # MODEL LOCATION: PATH PARAMETER
  # ====================================================================

  Scenario: Model selection with path parameter location
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: wrr-path
      spec:
        displayName: WRR Path
        version: v1.0.0
        context: /wrr-path/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /models/*
            policies:
              - name: model-weighted-round-robin
                version: v1
                params:
                  models:
                    - model: new-model-1
                      weight: 1
                    - model: new-model-2
                      weight: 1
                  requestModel:
                    location: pathParam
                    identifier: /models/([^/]+)/chat
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/wrr-path/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/wrr-path/v1.0.0/models/old-model/chat" with body:
      """
      {"prompt": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "/models/new-model-1/chat"
    When I send a POST request to "http://localhost:8080/wrr-path/v1.0.0/models/old-model/chat" with body:
      """
      {"prompt": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "/models/new-model-2/chat"
    Given I authenticate using basic auth as "admin"
    When I delete the API "wrr-path"
    Then the response should be successful

  # ====================================================================
  # MODEL SUSPENSION ON ERRORS
  # ====================================================================

  Scenario: Suspend model on 5xx error with recovery
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: wrr-susp-5xx
      spec:
        displayName: WRR Suspend 5xx
        version: v1.0.0
        context: /wrr-susp-5xx/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-weighted-round-robin
                version: v1
                params:
                  models:
                    - model: failing-model
                      weight: 1
                    - model: working-model
                      weight: 1
                  suspendDuration: 3
                  requestModel:
                    location: payload
                    identifier: "$.model"
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/wrr-susp-5xx/v1.0.0/health" to be ready
    # First request goes to failing-model, returns 500 from backend
    When I send a POST request to "http://localhost:8080/wrr-susp-5xx/v1.0.0/chat?statusCode=500" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 500
    # Next request should skip suspended failing-model and use working-model
    When I send a POST request to "http://localhost:8080/wrr-susp-5xx/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "working-model"
    # Another request should still use working-model (failing-model is suspended)
    When I send a POST request to "http://localhost:8080/wrr-susp-5xx/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "working-model"
    Given I authenticate using basic auth as "admin"
    When I delete the API "wrr-susp-5xx"
    Then the response should be successful

  Scenario: Suspend model on 429 rate limit error
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: wrr-susp-429
      spec:
        displayName: WRR Suspend 429
        version: v1.0.0
        context: /wrr-susp-429/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-weighted-round-robin
                version: v1
                params:
                  models:
                    - model: rate-limited-model
                      weight: 1
                    - model: available-model
                      weight: 1
                  suspendDuration: 3
                  requestModel:
                    location: payload
                    identifier: "$.model"
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/wrr-susp-429/v1.0.0/health" to be ready
    # First request returns 429 from backend
    When I send a POST request to "http://localhost:8080/wrr-susp-429/v1.0.0/chat?statusCode=429" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 429
    # Next request should use available-model (rate-limited-model is suspended)
    When I send a POST request to "http://localhost:8080/wrr-susp-429/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "available-model"
    # Another request should still use available-model (rate-limited-model remains suspended)
    When I send a POST request to "http://localhost:8080/wrr-susp-429/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "available-model"
    Given I authenticate using basic auth as "admin"
    When I delete the API "wrr-susp-429"
    Then the response should be successful

  Scenario: All models suspended returns 503
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: wrr-all-susp
      spec:
        displayName: WRR All Suspended
        version: v1.0.0
        context: /wrr-all-susp/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-weighted-round-robin
                version: v1
                params:
                  models:
                    - model: model-1
                      weight: 1
                    - model: model-2
                      weight: 1
                  suspendDuration: 5
                  requestModel:
                    location: payload
                    identifier: "$.model"
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/wrr-all-susp/v1.0.0/health" to be ready
    # Trigger 500 error for model-1
    When I send a POST request to "http://localhost:8080/wrr-all-susp/v1.0.0/chat?statusCode=500" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 500
    # Trigger 500 error for model-2
    When I send a POST request to "http://localhost:8080/wrr-all-susp/v1.0.0/chat?statusCode=500" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 500
    # Now all models are suspended, should return 503
    When I send a POST request to "http://localhost:8080/wrr-all-susp/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 503
    And the response body should contain "All models are currently unavailable"
    Given I authenticate using basic auth as "admin"
    When I delete the API "wrr-all-susp"
    Then the response should be successful

  # ====================================================================
  # SUSPENSION DISABLED (suspendDuration = 0)
  # ====================================================================

  Scenario: No suspension when suspendDuration is 0
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: wrr-no-susp
      spec:
        displayName: WRR No Suspend
        version: v1.0.0
        context: /wrr-no-susp/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-weighted-round-robin
                version: v1
                params:
                  models:
                    - model: model-a
                      weight: 1
                    - model: model-b
                      weight: 1
                  suspendDuration: 0
                  requestModel:
                    location: payload
                    identifier: "$.model"
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/wrr-no-susp/v1.0.0/health" to be ready
    # First request fails with 500
    When I send a POST request to "http://localhost:8080/wrr-no-susp/v1.0.0/chat?statusCode=500" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 500
    # Next request still rotates to model-b (no suspension)
    When I send a POST request to "http://localhost:8080/wrr-no-susp/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "model-b"
    # Next request rotates back to model-a (not suspended)
    When I send a POST request to "http://localhost:8080/wrr-no-susp/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "model-a"
    Given I authenticate using basic auth as "admin"
    When I delete the API "wrr-no-susp"
    Then the response should be successful

  # ====================================================================
  # EDGE CASES
  # ====================================================================

  Scenario: Handle empty request body
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: wrr-empty
      spec:
        displayName: WRR Empty Body
        version: v1.0.0
        context: /wrr-empty/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-weighted-round-robin
                version: v1
                params:
                  models:
                    - model: selected-model
                      weight: 1
                  requestModel:
                    location: payload
                    identifier: "$.model"
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/wrr-empty/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/wrr-empty/v1.0.0/chat" with body:
      """
      """
    Then the response status code should be 400
    And the response body should contain "Request body is empty"
    Given I authenticate using basic auth as "admin"
    When I delete the API "wrr-empty"
    Then the response should be successful

  Scenario: Handle invalid JSON in request body
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: wrr-inv-json
      spec:
        displayName: WRR Invalid JSON
        version: v1.0.0
        context: /wrr-inv-json/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-weighted-round-robin
                version: v1
                params:
                  models:
                    - model: selected-model
                      weight: 1
                  requestModel:
                    location: payload
                    identifier: "$.model"
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/wrr-inv-json/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/wrr-inv-json/v1.0.0/chat" with body:
      """
      invalid json {
      """
    Then the response status code should be 400
    And the response body should contain "Invalid JSON in request body"
    Given I authenticate using basic auth as "admin"
    When I delete the API "wrr-inv-json"
    Then the response should be successful

  Scenario: Handle invalid JSONPath
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: wrr-inv-path
      spec:
        displayName: WRR Invalid JSONPath
        version: v1.0.0
        context: /wrr-inv-path/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-weighted-round-robin
                version: v1
                params:
                  models:
                    - model: selected-model
                      weight: 1
                  requestModel:
                    location: payload
                    identifier: $.nonexistent.field
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/wrr-inv-path/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/wrr-inv-path/v1.0.0/chat" with body:
      """
      {"model": "test"}
      """
    Then the response status code should be 400
    And the response body should contain "Invalid or missing model"
    Given I authenticate using basic auth as "admin"
    When I delete the API "wrr-inv-path"
    Then the response should be successful

  Scenario: Handle missing model field in payload
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: wrr-missing
      spec:
        displayName: WRR Missing Model
        version: v1.0.0
        context: /wrr-missing/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-weighted-round-robin
                version: v1
                params:
                  models:
                    - model: selected-model
                      weight: 1
                  requestModel:
                    location: payload
                    identifier: "$.model"
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/wrr-missing/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/wrr-missing/v1.0.0/chat" with body:
      """
      {"prompt": "test without model field"}
      """
    Then the response status code should be 200
    And the response body should contain "selected-model"
    Given I authenticate using basic auth as "admin"
    When I delete the API "wrr-missing"
    Then the response should be successful

  # ====================================================================
  # REAL-WORLD SCENARIOS
  # ====================================================================

  Scenario: Fallback to secondary models on primary failure
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: wrr-fallback
      spec:
        displayName: WRR Fallback
        version: v1.0.0
        context: /wrr-fallback/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-weighted-round-robin
                version: v1
                params:
                  models:
                    - model: primary-model
                      weight: 4
                    - model: secondary-model-1
                      weight: 3
                    - model: secondary-model-2
                      weight: 3
                  suspendDuration: 10
                  requestModel:
                    location: payload
                    identifier: "$.model"
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/wrr-fallback/v1.0.0/health" to be ready
    # Primary model fails
    When I send a POST request to "http://localhost:8080/wrr-fallback/v1.0.0/chat?statusCode=500" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 500
    # Subsequent requests use secondary models
    When I send a POST request to "http://localhost:8080/wrr-fallback/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should match pattern "(secondary-model-1|secondary-model-2)"
    Given I authenticate using basic auth as "admin"
    When I delete the API "wrr-fallback"
    Then the response should be successful

  Scenario: Canary deployment with small weight for new model
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: wrr-canary
      spec:
        displayName: WRR Canary
        version: v1.0.0
        context: /wrr-canary/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-weighted-round-robin
                version: v1
                params:
                  models:
                    - model: stable-model-v1
                      weight: 9
                    - model: canary-model-v2
                      weight: 1
                  requestModel:
                    location: payload
                    identifier: "$.model"
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/wrr-canary/v1.0.0/health" to be ready
    # Requests 1-9 go to stable-model-v1, request 10 goes to canary-model-v2
    When I send a POST request to "http://localhost:8080/wrr-canary/v1.0.0/chat" with body:
      """
      {"model": "any", "prompt": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "stable-model-v1"
    When I send a POST request to "http://localhost:8080/wrr-canary/v1.0.0/chat" with body:
      """
      {"model": "any", "prompt": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "stable-model-v1"
    When I send a POST request to "http://localhost:8080/wrr-canary/v1.0.0/chat" with body:
      """
      {"model": "any", "prompt": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "stable-model-v1"
    When I send a POST request to "http://localhost:8080/wrr-canary/v1.0.0/chat" with body:
      """
      {"model": "any", "prompt": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "stable-model-v1"
    When I send a POST request to "http://localhost:8080/wrr-canary/v1.0.0/chat" with body:
      """
      {"model": "any", "prompt": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "stable-model-v1"
    When I send a POST request to "http://localhost:8080/wrr-canary/v1.0.0/chat" with body:
      """
      {"model": "any", "prompt": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "stable-model-v1"
    When I send a POST request to "http://localhost:8080/wrr-canary/v1.0.0/chat" with body:
      """
      {"model": "any", "prompt": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "stable-model-v1"
    When I send a POST request to "http://localhost:8080/wrr-canary/v1.0.0/chat" with body:
      """
      {"model": "any", "prompt": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "stable-model-v1"
    When I send a POST request to "http://localhost:8080/wrr-canary/v1.0.0/chat" with body:
      """
      {"model": "any", "prompt": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "stable-model-v1"
    # Weight boundary: 10th request transitions to canary-model-v2
    When I send a POST request to "http://localhost:8080/wrr-canary/v1.0.0/chat" with body:
      """
      {"model": "any", "prompt": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "canary-model-v2"
    Given I authenticate using basic auth as "admin"
    When I delete the API "wrr-canary"
    Then the response should be successful
