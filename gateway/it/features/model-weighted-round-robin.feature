Feature: Model Weighted Round-Robin Load Balancing Policy
  Test the model-weighted-round-robin policy which distributes AI model requests
  based on configurable weight values, implementing weighted round-robin selection
  and automatic suspension on failures.

  Background:
    Given API is deployed with following configuration
      """
      name: test-api
      version: 1.0.0
      basePath: /test
      type: REST
      endpointConfig:
        production:
          endpoint: http://sample-backend:9080
      """

  # ====================================================================
  # BASIC WEIGHTED DISTRIBUTION - PAYLOAD LOCATION
  # ====================================================================

#   Scenario: Basic weighted distribution with payload location
#     Given API is deployed with following configuration
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /chat
#           policies:
#             - name: model-weighted-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: gpt-3.5-turbo
#                     weight: 3
#                   - model: gpt-4
#                     weight: 1
#       """
#     And wait for health endpoint to be ready
    # First 3 requests should go to gpt-3.5-turbo (weight 3)
#     When client sends "POST" request to "/test/chat" with body
#       """
#       {"model": "original-model", "prompt": "Hello"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "gpt-3.5-turbo"
#     When client sends "POST" request to "/test/chat" with body
#       """
#       {"model": "original-model", "prompt": "Hello"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "gpt-3.5-turbo"
#     When client sends "POST" request to "/test/chat" with body
#       """
#       {"model": "original-model", "prompt": "Hello"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "gpt-3.5-turbo"
    # 4th request should go to gpt-4 (weight 1)
#     When client sends "POST" request to "/test/chat" with body
#       """
#       {"model": "original-model", "prompt": "Hello"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "gpt-4"
    # 5th request cycles back to gpt-3.5-turbo
#     When client sends "POST" request to "/test/chat" with body
#       """
#       {"model": "original-model", "prompt": "Hello"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "gpt-3.5-turbo"

#   Scenario: Equal weight distribution
#     Given API is deployed with following configuration
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /chat
#           policies:
#             - name: model-weighted-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: model-a
#                     weight: 1
#                   - model: model-b
#                     weight: 1
#       """
#     And wait for health endpoint to be ready
#     When client sends "POST" request to "/test/chat" with body
#       """
#       {"model": "original", "data": "test"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "model-a"
#     When client sends "POST" request to "/test/chat" with body
#       """
#       {"model": "original", "data": "test"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "model-b"

#   Scenario: Three models with different weights
#     Given API is deployed with following configuration
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /chat
#           policies:
#             - name: model-weighted-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: fast-model
#                     weight: 5
#                   - model: balanced-model
#                     weight: 3
#                   - model: premium-model
#                     weight: 2
#       """
#     And wait for health endpoint to be ready
    # Total weight = 10, so sequence is: [fast x5, balanced x3, premium x2]
#     When client sends "POST" request to "/test/chat" with body
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "fast-model"

  # ====================================================================
  # MODEL LOCATION: HEADER
  # ====================================================================

#   Scenario: Model selection with header location
#     Given API is deployed with following configuration
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /chat
#           policies:
#             - name: model-weighted-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: model-a
#                     weight: 1
#                   - model: model-b
#                     weight: 1
#                 requestModel:
#                   location: header
#                   identifier: X-Model
#       """
#     And wait for health endpoint to be ready
#     When client sends "POST" request to "/test/chat" with headers
#       """
#       X-Model: original-model
#       Content-Type: application/json
#       """
#     And body
#       """
#       {"prompt": "test"}
#       """
#     Then the response status code should be 200
#     And the response header "X-Model" should be "model-a"

  # ====================================================================
  # MODEL LOCATION: QUERY PARAMETER
  # ====================================================================

#   Scenario: Model selection with query parameter location
#     Given API is deployed with following configuration
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: GET
#           path: /chat
#           policies:
#             - name: model-weighted-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: model-alpha
#                     weight: 2
#                   - model: model-beta
#                     weight: 1
#                 requestModel:
#                   location: queryParam
#                   identifier: model
#       """
#     And wait for health endpoint to be ready
#     When client sends "GET" request to "/test/chat?model=original-model&prompt=hello"
#     Then the response status code should be 200
#     And the response body should contain "model=model-alpha"
#     When client sends "GET" request to "/test/chat?model=original-model&prompt=hello"
#     Then the response status code should be 200
#     And the response body should contain "model=model-alpha"
#     When client sends "GET" request to "/test/chat?model=original-model&prompt=hello"
#     Then the response status code should be 200
#     And the response body should contain "model=model-beta"

  # ====================================================================
  # MODEL LOCATION: PATH PARAMETER
  # ====================================================================

#   Scenario: Model selection with path parameter location
#     Given API is deployed with following configuration
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /models/*/chat
#           policies:
#             - name: model-weighted-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: new-model-1
#                     weight: 1
#                   - model: new-model-2
#                     weight: 1
#                 requestModel:
#                   location: pathParam
#                   identifier: /models/([^/]+)/chat
#       """
#     And wait for health endpoint to be ready
#     When client sends "POST" request to "/test/models/old-model/chat" with body
#       """
#       {"prompt": "test"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "/models/new-model-1/chat"
#     When client sends "POST" request to "/test/models/old-model/chat" with body
#       """
#       {"prompt": "test"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "/models/new-model-2/chat"

  # ====================================================================
  # MODEL SUSPENSION ON ERRORS
  # ====================================================================

#   Scenario: Suspend model on 5xx error with recovery
#     Given API is deployed with following configuration
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /chat
#           policies:
#             - name: model-weighted-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: failing-model
#                     weight: 1
#                   - model: working-model
#                     weight: 1
#                 suspendDuration: 3
#         - method: POST
#           path: /error500
#           policies:
#             - name: model-weighted-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: failing-model
#                     weight: 1
#                   - model: working-model
#                     weight: 1
#                 suspendDuration: 3
#       """
#     And wait for health endpoint to be ready
    # First request goes to failing-model, returns 500 from backend
#     When client sends "POST" request to "/test/error500" with body
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 500
    # Next request should skip suspended failing-model and use working-model
#     When client sends "POST" request to "/test/chat" with body
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "working-model"
    # Another request should still use working-model (failing-model is suspended)
#     When client sends "POST" request to "/test/chat" with body
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "working-model"

#   Scenario: Suspend model on 429 rate limit error
#     Given API is deployed with following configuration
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /chat
#           policies:
#             - name: model-weighted-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: rate-limited-model
#                     weight: 1
#                   - model: available-model
#                     weight: 1
#                 suspendDuration: 3
#         - method: POST
#           path: /error429
#           policies:
#             - name: model-weighted-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: rate-limited-model
#                     weight: 1
#                   - model: available-model
#                     weight: 1
#                 suspendDuration: 3
#       """
#     And wait for health endpoint to be ready
    # First request returns 429 from backend
#     When client sends "POST" request to "/test/error429" with body
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 429
    # Next request should use available-model (rate-limited-model is suspended)
#     When client sends "POST" request to "/test/chat" with body
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "available-model"

#   Scenario: All models suspended returns 503
#     Given API is deployed with following configuration
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /chat
#           policies:
#             - name: model-weighted-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: model-1
#                     weight: 1
#                   - model: model-2
#                     weight: 1
#                 suspendDuration: 5
#         - method: POST
#           path: /error500
#           policies:
#             - name: model-weighted-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: model-1
#                     weight: 1
#                   - model: model-2
#                     weight: 1
#                 suspendDuration: 5
#       """
#     And wait for health endpoint to be ready
    # Trigger 500 error for model-1
#     When client sends "POST" request to "/test/error500" with body
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 500
    # Trigger 500 error for model-2
#     When client sends "POST" request to "/test/error500" with body
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 500
    # Now all models are suspended, should return 503
#     When client sends "POST" request to "/test/chat" with body
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 503
#     And the response body should contain "All models are currently unavailable"

  # ====================================================================
  # SUSPENSION DISABLED (suspendDuration = 0)
  # ====================================================================

#   Scenario: No suspension when suspendDuration is 0
#     Given API is deployed with following configuration
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /chat
#           policies:
#             - name: model-weighted-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: model-a
#                     weight: 1
#                   - model: model-b
#                     weight: 1
#                 suspendDuration: 0
#         - method: POST
#           path: /error500
#           policies:
#             - name: model-weighted-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: model-a
#                     weight: 1
#                   - model: model-b
#                     weight: 1
#                 suspendDuration: 0
#       """
#     And wait for health endpoint to be ready
    # First request fails with 500
#     When client sends "POST" request to "/test/error500" with body
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 500
    # Next request still rotates to model-b (no suspension)
#     When client sends "POST" request to "/test/chat" with body
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "model-b"
    # Next request rotates back to model-a (not suspended)
#     When client sends "POST" request to "/test/chat" with body
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "model-a"

  # ====================================================================
  # EDGE CASES
  # ====================================================================

#   Scenario: Handle empty request body
#     Given API is deployed with following configuration
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /chat
#           policies:
#             - name: model-weighted-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: selected-model
#                     weight: 1
#       """
#     And wait for health endpoint to be ready
#     When client sends "POST" request to "/test/chat" with body
#       """
#       """
#     Then the response status code should be 400
#     And the response body should contain "Request body is empty"

#   Scenario: Handle invalid JSON in request body
#     Given API is deployed with following configuration
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /chat
#           policies:
#             - name: model-weighted-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: selected-model
#                     weight: 1
#       """
#     And wait for health endpoint to be ready
#     When client sends "POST" request to "/test/chat" with body
#       """
#       invalid json {
#       """
#     Then the response status code should be 400
#     And the response body should contain "Invalid JSON in request body"

#   Scenario: Handle invalid JSONPath
#     Given API is deployed with following configuration
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /chat
#           policies:
#             - name: model-weighted-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: selected-model
#                     weight: 1
#                 requestModel:
#                   location: payload
#                   identifier: $.nonexistent.field
#       """
#     And wait for health endpoint to be ready
#     When client sends "POST" request to "/test/chat" with body
#       """
#       {"model": "test"}
#       """
#     Then the response status code should be 400
#     And the response body should contain "Invalid or missing model"

#   Scenario: Handle missing model field in payload
#     Given API is deployed with following configuration
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /chat
#           policies:
#             - name: model-weighted-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: selected-model
#                     weight: 1
#       """
#     And wait for health endpoint to be ready
#     When client sends "POST" request to "/test/chat" with body
#       """
#       {"prompt": "test without model field"}
#       """
#     Then the response status code should be 400
#     And the response body should contain "Invalid or missing model"

#   Scenario: Single model with high weight
#     Given API is deployed with following configuration
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /chat
#           policies:
#             - name: model-weighted-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: only-model
#                     weight: 100
#       """
#     And wait for health endpoint to be ready
#     When client sends "POST" request to "/test/chat" with body
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "only-model"
    # All subsequent requests also use the same model
#     When client sends "POST" request to "/test/chat" with body
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "only-model"

  # ====================================================================
  # REAL-WORLD SCENARIOS
  # ====================================================================

#   Scenario: Load balance between cheap and expensive models
#     Given API is deployed with following configuration
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /completions
#           policies:
#             - name: model-weighted-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: gpt-3.5-turbo
#                     weight: 7
#                   - model: gpt-4
#                     weight: 3
#       """
#     And wait for health endpoint to be ready
    # Expect 70% gpt-3.5-turbo, 30% gpt-4
#     When client sends "POST" request to "/test/completions" with body
#       """
#       {"model": "any", "messages": [{"role": "user", "content": "Hello"}]}
#       """
#     Then the response status code should be 200

#   Scenario: Fallback to secondary models on primary failure
#     Given API is deployed with following configuration
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /chat
#           policies:
#             - name: model-weighted-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: primary-model
#                     weight: 4
#                   - model: secondary-model-1
#                     weight: 3
#                   - model: secondary-model-2
#                     weight: 3
#                 suspendDuration: 10
#         - method: POST
#           path: /error500
#           policies:
#             - name: model-weighted-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: primary-model
#                     weight: 4
#                   - model: secondary-model-1
#                     weight: 3
#                   - model: secondary-model-2
#                     weight: 3
#                 suspendDuration: 10
#       """
#     And wait for health endpoint to be ready
    # Primary model fails
#     When client sends "POST" request to "/test/error500" with body
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 500
    # Subsequent requests use secondary models
#     When client sends "POST" request to "/test/chat" with body
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should match pattern "(secondary-model-1|secondary-model-2)"

#   Scenario: Canary deployment with small weight for new model
#     Given API is deployed with following configuration
#       """
#       name: test-api
#       version: 1.0.0
#       basePath: /test
#       type: REST
#       endpointConfig:
#         production:
#           endpoint: http://sample-backend:9080
#       operations:
#         - method: POST
#           path: /chat
#           policies:
#             - name: model-weighted-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: stable-model-v1
#                     weight: 9
#                   - model: canary-model-v2
#                     weight: 1
#       """
#     And wait for health endpoint to be ready
    # 10% of requests go to canary, 90% to stable
#     When client sends "POST" request to "/test/chat" with body
#       """
#       {"model": "any", "prompt": "test"}
#       """
#     Then the response status code should be 200
