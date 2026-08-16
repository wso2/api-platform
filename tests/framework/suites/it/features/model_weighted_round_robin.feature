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
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: wrr-basic
      spec:
        displayName: WRR Basic
        version: v1.0.0
        context: /wrr-basic/$version
        upstream:
          main:
            url: http://testbench:3000
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
    Then the response status code should be 201
    And I send a "GET" request to "/wrr-basic/v1.0.0/health" until status 200
    # First 3 requests should go to gpt-3.5-turbo (weight 3)
    When I send a "POST" request to "/wrr-basic/v1.0.0/chat" with body:
      """
      {"model": "original-model", "prompt": "Hello"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"gpt-3.5-turbo","prompt":"Hello"}
      """
    When I send a "POST" request to "/wrr-basic/v1.0.0/chat" with body:
      """
      {"model": "original-model", "prompt": "Hello"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"gpt-3.5-turbo","prompt":"Hello"}
      """
    When I send a "POST" request to "/wrr-basic/v1.0.0/chat" with body:
      """
      {"model": "original-model", "prompt": "Hello"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"gpt-3.5-turbo","prompt":"Hello"}
      """
    # 4th request should go to gpt-4 (weight 1)
    When I send a "POST" request to "/wrr-basic/v1.0.0/chat" with body:
      """
      {"model": "original-model", "prompt": "Hello"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"gpt-4","prompt":"Hello"}
      """
    # 5th request cycles back to gpt-3.5-turbo
    When I send a "POST" request to "/wrr-basic/v1.0.0/chat" with body:
      """
      {"model": "original-model", "prompt": "Hello"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"gpt-3.5-turbo","prompt":"Hello"}
      """
    When I delete the API "wrr-basic"
    Then the response status code should be 200

  Scenario: Equal weight distribution
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: wrr-equal
      spec:
        displayName: WRR Equal
        version: v1.0.0
        context: /wrr-equal/$version
        upstream:
          main:
            url: http://testbench:3000
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
    Then the response status code should be 201
    And I send a "GET" request to "/wrr-equal/v1.0.0/health" until status 200
    When I send a "POST" request to "/wrr-equal/v1.0.0/chat" with body:
      """
      {"model": "original", "data": "test"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"data":"test","model":"model-a"}
      """
    When I send a "POST" request to "/wrr-equal/v1.0.0/chat" with body:
      """
      {"model": "original", "data": "test"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"data":"test","model":"model-b"}
      """
    # Cycle wraps back to model-a
    When I send a "POST" request to "/wrr-equal/v1.0.0/chat" with body:
      """
      {"model": "original", "data": "test"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"data":"test","model":"model-a"}
      """
    When I delete the API "wrr-equal"
    Then the response status code should be 200

  Scenario: Three models with different weights
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: wrr-three
      spec:
        displayName: WRR Three
        version: v1.0.0
        context: /wrr-three/$version
        upstream:
          main:
            url: http://testbench:3000
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
    Then the response status code should be 201
    And I send a "GET" request to "/wrr-three/v1.0.0/health" until status 200
    # Total weight = 10, so sequence is: [fast x5, balanced x3, premium x2]
    When I send a "POST" request to "/wrr-three/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"fast-model"}
      """
    When I send a "POST" request to "/wrr-three/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"fast-model"}
      """
    When I send a "POST" request to "/wrr-three/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"fast-model"}
      """
    When I send a "POST" request to "/wrr-three/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"fast-model"}
      """
    When I send a "POST" request to "/wrr-three/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"fast-model"}
      """
    # Weight boundary: 6th request transitions to balanced-model
    When I send a "POST" request to "/wrr-three/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"balanced-model"}
      """
    When I send a "POST" request to "/wrr-three/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"balanced-model"}
      """
    When I send a "POST" request to "/wrr-three/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"balanced-model"}
      """
    # Weight boundary: 9th request transitions to premium-model
    When I send a "POST" request to "/wrr-three/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"premium-model"}
      """
    When I delete the API "wrr-three"
    Then the response status code should be 200

  # ====================================================================
  # MODEL LOCATION: HEADER
  # ====================================================================

  Scenario: Model selection with header location
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: wrr-header
      spec:
        displayName: WRR Header
        version: v1.0.0
        context: /wrr-header/$version
        upstream:
          main:
            url: http://testbench:3000
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
    Then the response status code should be 201
    And I send a "GET" request to "/wrr-header/v1.0.0/health" until status 200
    When I set header "X-Model" to "original-model"
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/wrr-header/v1.0.0/chat" with body:
      """
      {"prompt": "test"}
      """
    Then the response status code should be 200
    And the JSON response field "headers.X-Model[0]" should be "model-a"
    When I set header "X-Model" to "original-model"
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/wrr-header/v1.0.0/chat" with body:
      """
      {"prompt": "test"}
      """
    Then the response status code should be 200
    And the JSON response field "headers.X-Model[0]" should be "model-b"
    When I delete the API "wrr-header"
    Then the response status code should be 200

  # ====================================================================
  # MODEL LOCATION: QUERY PARAMETER
  # ====================================================================

  Scenario: Model selection with query parameter location
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: wrr-query
      spec:
        displayName: WRR Query
        version: v1.0.0
        context: /wrr-query/$version
        upstream:
          main:
            url: http://testbench:3000
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
    Then the response status code should be 201
    # Sequence: [model-alpha, model-alpha, model-beta]
    When I send a "GET" request to "/wrr-query/v1.0.0/chat?model=original-model&prompt=hello" until status 200
    Then the response status code should be 200
    And the JSON response field "query" should be "model=model-alpha&prompt=hello"
    When I send a "GET" request to "/wrr-query/v1.0.0/chat?model=original-model&prompt=hello"
    Then the response status code should be 200
    And the JSON response field "query" should be "model=model-alpha&prompt=hello"
    When I send a "GET" request to "/wrr-query/v1.0.0/chat?model=original-model&prompt=hello"
    Then the response status code should be 200
    And the JSON response field "query" should be "model=model-beta&prompt=hello"
    When I delete the API "wrr-query"
    Then the response status code should be 200

  # ====================================================================
  # MODEL LOCATION: PATH PARAMETER
  # ====================================================================

  Scenario: Model selection with path parameter location
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: wrr-path
      spec:
        displayName: WRR Path
        version: v1.0.0
        context: /wrr-path/$version
        upstream:
          main:
            url: http://testbench:3000
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
    Then the response status code should be 201
    And I send a "GET" request to "/wrr-path/v1.0.0/health" until status 200
    When I send a "POST" request to "/wrr-path/v1.0.0/models/old-model/chat" with body:
      """
      {"prompt": "test"}
      """
    Then the response status code should be 200
    And the JSON response field "path" should be "/models/new-model-1/chat"
    When I send a "POST" request to "/wrr-path/v1.0.0/models/old-model/chat" with body:
      """
      {"prompt": "test"}
      """
    Then the response status code should be 200
    And the JSON response field "path" should be "/models/new-model-2/chat"
    When I delete the API "wrr-path"
    Then the response status code should be 200

  # ====================================================================
  # MODEL SUSPENSION ON ERRORS
  # ====================================================================

  Scenario: Suspend model on 5xx error with recovery
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: wrr-susp-5xx
      spec:
        displayName: WRR Suspend 5xx
        version: v1.0.0
        context: /wrr-susp-5xx/$version
        upstream:
          main:
            url: http://testbench:3000
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
    Then the response status code should be 201
    And I send a "GET" request to "/wrr-susp-5xx/v1.0.0/health" until status 200
    # First request goes to failing-model, returns 500 from backend
    When I send a "POST" request to "/wrr-susp-5xx/v1.0.0/chat?statusCode=500" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 500
    # Next request should skip suspended failing-model and use working-model
    When I send a "POST" request to "/wrr-susp-5xx/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"working-model"}
      """
    # Another request should still use working-model (failing-model is suspended)
    When I send a "POST" request to "/wrr-susp-5xx/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"working-model"}
      """
    When I delete the API "wrr-susp-5xx"
    Then the response status code should be 200

  Scenario: Suspend model on 429 rate limit error
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: wrr-susp-429
      spec:
        displayName: WRR Suspend 429
        version: v1.0.0
        context: /wrr-susp-429/$version
        upstream:
          main:
            url: http://testbench:3000
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
    Then the response status code should be 201
    And I send a "GET" request to "/wrr-susp-429/v1.0.0/health" until status 200
    # First request returns 429 from backend
    When I send a "POST" request to "/wrr-susp-429/v1.0.0/chat?statusCode=429" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 429
    # Next request should use available-model (rate-limited-model is suspended)
    When I send a "POST" request to "/wrr-susp-429/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"available-model"}
      """
    # Another request should still use available-model (rate-limited-model remains suspended)
    When I send a "POST" request to "/wrr-susp-429/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"available-model"}
      """
    When I delete the API "wrr-susp-429"
    Then the response status code should be 200

  Scenario: All models suspended returns 503
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: wrr-all-susp
      spec:
        displayName: WRR All Suspended
        version: v1.0.0
        context: /wrr-all-susp/$version
        upstream:
          main:
            url: http://testbench:3000
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
    Then the response status code should be 201
    And I send a "GET" request to "/wrr-all-susp/v1.0.0/health" until status 200
    # Trigger 500 error for model-1
    When I send a "POST" request to "/wrr-all-susp/v1.0.0/chat?statusCode=500" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 500
    # Trigger 500 error for model-2
    When I send a "POST" request to "/wrr-all-susp/v1.0.0/chat?statusCode=500" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 500
    # Now all models are suspended, should return 503
    When I send a "POST" request to "/wrr-all-susp/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 503
    And the JSON response field "error" should be "All models are currently unavailable"
    When I delete the API "wrr-all-susp"
    Then the response status code should be 200

  # ====================================================================
  # SUSPENSION DISABLED (suspendDuration = 0)
  # ====================================================================

  Scenario: No suspension when suspendDuration is 0
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: wrr-no-susp
      spec:
        displayName: WRR No Suspend
        version: v1.0.0
        context: /wrr-no-susp/$version
        upstream:
          main:
            url: http://testbench:3000
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
    Then the response status code should be 201
    And I send a "GET" request to "/wrr-no-susp/v1.0.0/health" until status 200
    # First request fails with 500
    When I send a "POST" request to "/wrr-no-susp/v1.0.0/chat?statusCode=500" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 500
    # Next request still rotates to model-b (no suspension)
    When I send a "POST" request to "/wrr-no-susp/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"model-b"}
      """
    # Next request rotates back to model-a (not suspended)
    When I send a "POST" request to "/wrr-no-susp/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"model-a"}
      """
    When I delete the API "wrr-no-susp"
    Then the response status code should be 200

  # ====================================================================
  # EDGE CASES
  # ====================================================================

  Scenario: Handle empty request body
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: wrr-empty
      spec:
        displayName: WRR Empty Body
        version: v1.0.0
        context: /wrr-empty/$version
        upstream:
          main:
            url: http://testbench:3000
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
    Then the response status code should be 201
    And I send a "GET" request to "/wrr-empty/v1.0.0/health" until status 200
    When I send a "POST" request to "/wrr-empty/v1.0.0/chat" with body:
      """
      """
    Then the response status code should be 400
    And the JSON response field "error" should be "Request body is empty."
    When I delete the API "wrr-empty"
    Then the response status code should be 200

  Scenario: Handle invalid JSON in request body
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: wrr-inv-json
      spec:
        displayName: WRR Invalid JSON
        version: v1.0.0
        context: /wrr-inv-json/$version
        upstream:
          main:
            url: http://testbench:3000
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
    Then the response status code should be 201
    And I send a "GET" request to "/wrr-inv-json/v1.0.0/health" until status 200
    When I send a "POST" request to "/wrr-inv-json/v1.0.0/chat" with body:
      """
      invalid json {
      """
    Then the response status code should be 400
    And the JSON response field "error" should be "Invalid JSON in request body: invalid character 'i' looking for beginning of value"
    When I delete the API "wrr-inv-json"
    Then the response status code should be 200

  Scenario: Handle invalid JSONPath
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: wrr-inv-path
      spec:
        displayName: WRR Invalid JSONPath
        version: v1.0.0
        context: /wrr-inv-path/$version
        upstream:
          main:
            url: http://testbench:3000
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
    Then the response status code should be 201
    And I send a "GET" request to "/wrr-inv-path/v1.0.0/health" until status 200
    When I send a "POST" request to "/wrr-inv-path/v1.0.0/chat" with body:
      """
      {"model": "test"}
      """
    Then the response status code should be 400
    And the JSON response field "error" should be "Invalid or missing model at '$.nonexistent.field': key not found: nonexistent"
    When I delete the API "wrr-inv-path"
    Then the response status code should be 200

  Scenario: Handle missing model field in payload
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: wrr-missing
      spec:
        displayName: WRR Missing Model
        version: v1.0.0
        context: /wrr-missing/$version
        upstream:
          main:
            url: http://testbench:3000
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
    Then the response status code should be 201
    And I send a "GET" request to "/wrr-missing/v1.0.0/health" until status 200
    When I send a "POST" request to "/wrr-missing/v1.0.0/chat" with body:
      """
      {"prompt": "test without model field"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"selected-model","prompt":"test without model field"}
      """
    When I delete the API "wrr-missing"
    Then the response status code should be 200

  # ====================================================================
  # REAL-WORLD SCENARIOS
  # ====================================================================

  Scenario: Fallback to secondary models on primary failure
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: wrr-fallback
      spec:
        displayName: WRR Fallback
        version: v1.0.0
        context: /wrr-fallback/$version
        upstream:
          main:
            url: http://testbench:3000
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
    Then the response status code should be 201
    And I send a "GET" request to "/wrr-fallback/v1.0.0/health" until status 200
    # Primary model fails
    When I send a "POST" request to "/wrr-fallback/v1.0.0/chat?statusCode=500" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 500
    # Subsequent requests use secondary models
    When I send a "POST" request to "/wrr-fallback/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    # Either backup may serve — the selection is non-deterministic — but the suspended
    # model must not, which is what suspendDuration claims.
    And the response body should match pattern "(secondary-model-1|secondary-model-2)"
    And the response body should not contain "primary-model"
    When I delete the API "wrr-fallback"
    Then the response status code should be 200

  Scenario: Canary deployment with small weight for new model
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: wrr-canary
      spec:
        displayName: WRR Canary
        version: v1.0.0
        context: /wrr-canary/$version
        upstream:
          main:
            url: http://testbench:3000
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
    Then the response status code should be 201
    And I send a "GET" request to "/wrr-canary/v1.0.0/health" until status 200
    # Requests 1-9 go to stable-model-v1, request 10 goes to canary-model-v2
    When I send a "POST" request to "/wrr-canary/v1.0.0/chat" with body:
      """
      {"model": "any", "prompt": "test"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"stable-model-v1","prompt":"test"}
      """
    When I send a "POST" request to "/wrr-canary/v1.0.0/chat" with body:
      """
      {"model": "any", "prompt": "test"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"stable-model-v1","prompt":"test"}
      """
    When I send a "POST" request to "/wrr-canary/v1.0.0/chat" with body:
      """
      {"model": "any", "prompt": "test"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"stable-model-v1","prompt":"test"}
      """
    When I send a "POST" request to "/wrr-canary/v1.0.0/chat" with body:
      """
      {"model": "any", "prompt": "test"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"stable-model-v1","prompt":"test"}
      """
    When I send a "POST" request to "/wrr-canary/v1.0.0/chat" with body:
      """
      {"model": "any", "prompt": "test"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"stable-model-v1","prompt":"test"}
      """
    When I send a "POST" request to "/wrr-canary/v1.0.0/chat" with body:
      """
      {"model": "any", "prompt": "test"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"stable-model-v1","prompt":"test"}
      """
    When I send a "POST" request to "/wrr-canary/v1.0.0/chat" with body:
      """
      {"model": "any", "prompt": "test"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"stable-model-v1","prompt":"test"}
      """
    When I send a "POST" request to "/wrr-canary/v1.0.0/chat" with body:
      """
      {"model": "any", "prompt": "test"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"stable-model-v1","prompt":"test"}
      """
    When I send a "POST" request to "/wrr-canary/v1.0.0/chat" with body:
      """
      {"model": "any", "prompt": "test"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"stable-model-v1","prompt":"test"}
      """
    # Weight boundary: 10th request transitions to canary-model-v2
    When I send a "POST" request to "/wrr-canary/v1.0.0/chat" with body:
      """
      {"model": "any", "prompt": "test"}
      """
    Then the response status code should be 200
    And the JSON response field "body" should be:
      """
      {"model":"canary-model-v2","prompt":"test"}
      """
    When I delete the API "wrr-canary"
    Then the response status code should be 200
