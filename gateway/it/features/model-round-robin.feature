@model-round-robin
Feature: Model Round-Robin Load Balancing Policy
  Test the model-round-robin policy which distributes AI model requests evenly
  across multiple configured models in a cyclic round-robin pattern, ensuring
  equal request allocation and automatic suspension on failures.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ====================================================================
  # BASIC ROUND-ROBIN DISTRIBUTION - PAYLOAD LOCATION
  # ====================================================================

  Scenario: Basic round-robin with two models
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: rrr-two-models
      spec:
        displayName: RRR Two Models
        version: v1.0.0
        context: /rrr-two-models/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-round-robin
                version: v1
                params:
                  models:
                    - model: model-a
                    - model: model-b
                  requestModel:
                    location: payload
                    identifier: "$.model"
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/rrr-two-models/v1.0.0/health" to be ready
    # First request goes to model-a
    When I send a POST request to "http://localhost:8080/rrr-two-models/v1.0.0/chat" with body:
      """
      {"model": "original-model", "prompt": "Hello"}
      """
    Then the response status code should be 200
    And the response body should contain "model-a"
    # Second request goes to model-b
    When I send a POST request to "http://localhost:8080/rrr-two-models/v1.0.0/chat" with body:
      """
      {"model": "original-model", "prompt": "Hello"}
      """
    Then the response status code should be 200
    And the response body should contain "model-b"
    # Third request cycles back to model-a
    When I send a POST request to "http://localhost:8080/rrr-two-models/v1.0.0/chat" with body:
      """
      {"model": "original-model", "prompt": "Hello"}
      """
    Then the response status code should be 200
    And the response body should contain "model-a"
    # Fourth request goes to model-b
    When I send a POST request to "http://localhost:8080/rrr-two-models/v1.0.0/chat" with body:
      """
      {"model": "original-model", "prompt": "Hello"}
      """
    Then the response status code should be 200
    And the response body should contain "model-b"
    Given I authenticate using basic auth as "admin"
    When I delete the API "rrr-two-models"
    Then the response should be successful

  Scenario: Round-robin with three models
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: rrr-three-models
      spec:
        displayName: RRR Three Models
        version: v1.0.0
        context: /rrr-three-models/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-round-robin
                version: v1
                params:
                  models:
                    - model: model-alpha
                    - model: model-beta
                    - model: model-gamma
                  requestModel:
                    location: payload
                    identifier: "$.model"
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/rrr-three-models/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/rrr-three-models/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "model-alpha"
    When I send a POST request to "http://localhost:8080/rrr-three-models/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "model-beta"
    When I send a POST request to "http://localhost:8080/rrr-three-models/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "model-gamma"
    # Cycle back to first model
    When I send a POST request to "http://localhost:8080/rrr-three-models/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "model-alpha"
    Given I authenticate using basic auth as "admin"
    When I delete the API "rrr-three-models"
    Then the response should be successful

  Scenario: Round-robin with four models
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: rrr-four-models
      spec:
        displayName: RRR Four Models
        version: v1.0.0
        context: /rrr-four-models/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-round-robin
                version: v1
                params:
                  models:
                    - model: gpt-3.5-turbo
                    - model: gpt-4
                    - model: claude-3-sonnet
                    - model: gemini-pro
                  requestModel:
                    location: payload
                    identifier: "$.model"
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/rrr-four-models/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/rrr-four-models/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "gpt-3.5-turbo"
    When I send a POST request to "http://localhost:8080/rrr-four-models/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "gpt-4"
    When I send a POST request to "http://localhost:8080/rrr-four-models/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "claude-3-sonnet"
    When I send a POST request to "http://localhost:8080/rrr-four-models/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "gemini-pro"
    Given I authenticate using basic auth as "admin"
    When I delete the API "rrr-four-models"
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
        name: rrr-header-loc
      spec:
        displayName: RRR Header Location
        version: v1.0.0
        context: /rrr-header-loc/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-round-robin
                version: v1
                params:
                  models:
                    - model: header-model-1
                    - model: header-model-2
                  requestModel:
                    location: header
                    identifier: X-AI-Model
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/rrr-header-loc/v1.0.0/health" to be ready
    When I set header "X-AI-Model" to "original-model"
    And I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/rrr-header-loc/v1.0.0/chat" with body:
      """
      {"prompt": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "header-model-1"
    When I set header "X-AI-Model" to "original-model"
    And I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/rrr-header-loc/v1.0.0/chat" with body:
      """
      {"prompt": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "header-model-2"
    # Cycle wraps back to header-model-1
    When I set header "X-AI-Model" to "original-model"
    And I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/rrr-header-loc/v1.0.0/chat" with body:
      """
      {"prompt": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "header-model-1"
    Given I authenticate using basic auth as "admin"
    When I delete the API "rrr-header-loc"
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
        name: rrr-query-loc
      spec:
        displayName: RRR Query Param Location
        version: v1.0.0
        context: /rrr-query-loc/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /chat
            policies:
              - name: model-round-robin
                version: v1
                params:
                  models:
                    - model: query-model-1
                    - model: query-model-2
                    - model: query-model-3
                  requestModel:
                    location: queryParam
                    identifier: model
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/rrr-query-loc/v1.0.0/health" to be ready
    When I send a GET request to "http://localhost:8080/rrr-query-loc/v1.0.0/chat?model=original&prompt=hello"
    Then the response status code should be 200
    And the response body should contain "model=query-model-1"
    When I send a GET request to "http://localhost:8080/rrr-query-loc/v1.0.0/chat?model=original&prompt=hello"
    Then the response status code should be 200
    And the response body should contain "model=query-model-2"
    When I send a GET request to "http://localhost:8080/rrr-query-loc/v1.0.0/chat?model=original&prompt=hello"
    Then the response status code should be 200
    And the response body should contain "model=query-model-3"
    When I send a GET request to "http://localhost:8080/rrr-query-loc/v1.0.0/chat?model=original&prompt=hello"
    Then the response status code should be 200
    And the response body should contain "model=query-model-1"
    Given I authenticate using basic auth as "admin"
    When I delete the API "rrr-query-loc"
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
        name: rrr-path-loc
      spec:
        displayName: RRR Path Param Location
        version: v1.0.0
        context: /rrr-path-loc/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /models/*
            policies:
              - name: model-round-robin
                version: v1
                params:
                  models:
                    - model: path-model-x
                    - model: path-model-y
                  requestModel:
                    location: pathParam
                    identifier: /models/([^/]+)/chat
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/rrr-path-loc/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/rrr-path-loc/v1.0.0/models/original-model/chat" with body:
      """
      {"prompt": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "/models/path-model-x/chat"
    When I send a POST request to "http://localhost:8080/rrr-path-loc/v1.0.0/models/original-model/chat" with body:
      """
      {"prompt": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "/models/path-model-y/chat"
    # Cycle wraps back to path-model-x
    When I send a POST request to "http://localhost:8080/rrr-path-loc/v1.0.0/models/original-model/chat" with body:
      """
      {"prompt": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "/models/path-model-x/chat"
    Given I authenticate using basic auth as "admin"
    When I delete the API "rrr-path-loc"
    Then the response should be successful

  # ====================================================================
  # NESTED JSON PATH
  # ====================================================================

  Scenario: Model selection with nested JSONPath
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: rrr-nested-jsonpath
      spec:
        displayName: RRR Nested JSONPath
        version: v1.0.0
        context: /rrr-nested-jsonpath/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-round-robin
                version: v1
                params:
                  models:
                    - model: nested-model-1
                    - model: nested-model-2
                  requestModel:
                    location: payload
                    identifier: $.settings.ai.model
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/rrr-nested-jsonpath/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/rrr-nested-jsonpath/v1.0.0/chat" with body:
      """
      {"settings": {"ai": {"model": "original"}}, "prompt": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "nested-model-1"
    When I send a POST request to "http://localhost:8080/rrr-nested-jsonpath/v1.0.0/chat" with body:
      """
      {"settings": {"ai": {"model": "original"}}, "prompt": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "nested-model-2"
    # Cycle wraps back to nested-model-1
    When I send a POST request to "http://localhost:8080/rrr-nested-jsonpath/v1.0.0/chat" with body:
      """
      {"settings": {"ai": {"model": "original"}}, "prompt": "test"}
      """
    Then the response status code should be 200
    And the response body should contain "nested-model-1"
    Given I authenticate using basic auth as "admin"
    When I delete the API "rrr-nested-jsonpath"
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
        name: rrr-suspend-5xx
      spec:
        displayName: RRR Suspend 5xx
        version: v1.0.0
        context: /rrr-suspend-5xx/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-round-robin
                version: v1
                params:
                  models:
                    - model: first-model
                    - model: second-model
                    - model: third-model
                  suspendDuration: 3
                  requestModel:
                    location: payload
                    identifier: "$.model"
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/rrr-suspend-5xx/v1.0.0/health" to be ready
    # First request goes to first-model, returns 500 from backend
    When I send a POST request to "http://localhost:8080/rrr-suspend-5xx/v1.0.0/chat?statusCode=500" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 500
    # Next request should skip first-model and use second-model
    When I send a POST request to "http://localhost:8080/rrr-suspend-5xx/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "second-model"
    # Next request should use third-model
    When I send a POST request to "http://localhost:8080/rrr-suspend-5xx/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "third-model"
    # Next request should use second-model (skip suspended first-model)
    When I send a POST request to "http://localhost:8080/rrr-suspend-5xx/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "second-model"
    Given I authenticate using basic auth as "admin"
    When I delete the API "rrr-suspend-5xx"
    Then the response should be successful

  Scenario: Suspend model on 429 rate limit error
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: rrr-suspend-429
      spec:
        displayName: RRR Suspend 429
        version: v1.0.0
        context: /rrr-suspend-429/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-round-robin
                version: v1
                params:
                  models:
                    - model: model-1
                    - model: model-2
                  suspendDuration: 3
                  requestModel:
                    location: payload
                    identifier: "$.model"
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/rrr-suspend-429/v1.0.0/health" to be ready
    # First request returns 429 from backend
    When I send a POST request to "http://localhost:8080/rrr-suspend-429/v1.0.0/chat?statusCode=429" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 429
    # Next request should use model-2 (model-1 is suspended)
    When I send a POST request to "http://localhost:8080/rrr-suspend-429/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "model-2"
    # Another request should still use model-2
    When I send a POST request to "http://localhost:8080/rrr-suspend-429/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "model-2"
    Given I authenticate using basic auth as "admin"
    When I delete the API "rrr-suspend-429"
    Then the response should be successful

  Scenario: All models suspended returns 503
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: rrr-all-suspended
      spec:
        displayName: RRR All Suspended
        version: v1.0.0
        context: /rrr-all-suspended/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-round-robin
                version: v1
                params:
                  models:
                    - model: model-x
                    - model: model-y
                  suspendDuration: 5
                  requestModel:
                    location: payload
                    identifier: "$.model"
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/rrr-all-suspended/v1.0.0/health" to be ready
    # Trigger 500 error for model-x
    When I send a POST request to "http://localhost:8080/rrr-all-suspended/v1.0.0/chat?statusCode=500" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 500
    # Trigger 500 error for model-y
    When I send a POST request to "http://localhost:8080/rrr-all-suspended/v1.0.0/chat?statusCode=500" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 500
    # Now all models are suspended, should return 503
    When I send a POST request to "http://localhost:8080/rrr-all-suspended/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 503
    And the response body should contain "All models are currently unavailable"
    Given I authenticate using basic auth as "admin"
    When I delete the API "rrr-all-suspended"
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
        name: rrr-no-suspend
      spec:
        displayName: RRR No Suspend
        version: v1.0.0
        context: /rrr-no-suspend/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-round-robin
                version: v1
                params:
                  models:
                    - model: model-one
                    - model: model-two
                  suspendDuration: 0
                  requestModel:
                    location: payload
                    identifier: "$.model"
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/rrr-no-suspend/v1.0.0/health" to be ready
    # First request fails with 500
    When I send a POST request to "http://localhost:8080/rrr-no-suspend/v1.0.0/chat?statusCode=500" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 500
    # Next request continues rotation to model-two (no suspension)
    When I send a POST request to "http://localhost:8080/rrr-no-suspend/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "model-two"
    # Next request rotates back to model-one (not suspended)
    When I send a POST request to "http://localhost:8080/rrr-no-suspend/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should contain "model-one"
    Given I authenticate using basic auth as "admin"
    When I delete the API "rrr-no-suspend"
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
        name: rrr-empty-body
      spec:
        displayName: RRR Empty Body
        version: v1.0.0
        context: /rrr-empty-body/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-round-robin
                version: v1
                params:
                  models:
                    - model: selected-model
                  requestModel:
                    location: payload
                    identifier: "$.model"
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/rrr-empty-body/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/rrr-empty-body/v1.0.0/chat" with body:
      """
      """
    Then the response status code should be 400
    And the response body should contain "Request body is empty"
    Given I authenticate using basic auth as "admin"
    When I delete the API "rrr-empty-body"
    Then the response should be successful

  Scenario: Handle invalid JSON in request body
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: rrr-invalid-json
      spec:
        displayName: RRR Invalid JSON
        version: v1.0.0
        context: /rrr-invalid-json/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-round-robin
                version: v1
                params:
                  models:
                    - model: selected-model
                  requestModel:
                    location: payload
                    identifier: "$.model"
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/rrr-invalid-json/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/rrr-invalid-json/v1.0.0/chat" with body:
      """
      not valid json [
      """
    Then the response status code should be 400
    And the response body should contain "Invalid JSON in request body"
    Given I authenticate using basic auth as "admin"
    When I delete the API "rrr-invalid-json"
    Then the response should be successful

  Scenario: Handle invalid JSONPath
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: rrr-inv-jsonpath
      spec:
        displayName: RRR Invalid JSONPath
        version: v1.0.0
        context: /rrr-inv-jsonpath/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-round-robin
                version: v1
                params:
                  models:
                    - model: selected-model
                  requestModel:
                    location: payload
                    identifier: $.does.not.exist
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/rrr-inv-jsonpath/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/rrr-inv-jsonpath/v1.0.0/chat" with body:
      """
      {"model": "test"}
      """
    Then the response status code should be 400
    And the response body should contain "Invalid or missing model"
    Given I authenticate using basic auth as "admin"
    When I delete the API "rrr-inv-jsonpath"
    Then the response should be successful

  Scenario: Handle missing model field in payload
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: rrr-missing-model
      spec:
        displayName: RRR Missing Model
        version: v1.0.0
        context: /rrr-missing-model/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-round-robin
                version: v1
                params:
                  models:
                    - model: selected-model
                  requestModel:
                    location: payload
                    identifier: "$.model"
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/rrr-missing-model/v1.0.0/health" to be ready
    When I send a POST request to "http://localhost:8080/rrr-missing-model/v1.0.0/chat" with body:
      """
      {"prompt": "test without model field"}
      """
    Then the response status code should be 200
    And the response body should contain "selected-model"
    Given I authenticate using basic auth as "admin"
    When I delete the API "rrr-missing-model"
    Then the response should be successful

  Scenario: High availability with automatic failover
    When I deploy an API with the following configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: rrr-ha-failover
      spec:
        displayName: RRR HA Failover
        version: v1.0.0
        context: /rrr-ha-failover/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: POST
            path: /chat
            policies:
              - name: model-round-robin
                version: v1
                params:
                  models:
                    - model: primary-instance
                    - model: backup-instance-1
                    - model: backup-instance-2
                  suspendDuration: 10
                  requestModel:
                    location: payload
                    identifier: "$.model"
          - method: GET
            path: /health
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/rrr-ha-failover/v1.0.0/health" to be ready
    # Primary instance fails
    When I send a POST request to "http://localhost:8080/rrr-ha-failover/v1.0.0/chat?statusCode=500" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 500
    # System automatically fails over to backup instances
    When I send a POST request to "http://localhost:8080/rrr-ha-failover/v1.0.0/chat" with body:
      """
      {"model": "any"}
      """
    Then the response status code should be 200
    And the response body should match pattern "(backup-instance-1|backup-instance-2)"
    Given I authenticate using basic auth as "admin"
    When I delete the API "rrr-ha-failover"
    Then the response should be successful
