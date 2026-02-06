Feature: Model Round-Robin Load Balancing Policy
  Test the model-round-robin policy which distributes AI model requests evenly
  across multiple configured models in a cyclic round-robin pattern, ensuring
  equal request allocation and automatic suspension on failures.

  Background:
    Given I deploy an API with the following configuration:
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
  # BASIC ROUND-ROBIN DISTRIBUTION - PAYLOAD LOCATION
  # ====================================================================

#   Scenario: Basic round-robin with two models
#     Given I deploy an API with the following configuration:
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
#             - name: model-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: model-a
#                   - model: model-b
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
    # First request goes to model-a
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "original-model", "prompt": "Hello"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "model-a"
    # Second request goes to model-b
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "original-model", "prompt": "Hello"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "model-b"
    # Third request cycles back to model-a
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "original-model", "prompt": "Hello"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "model-a"
    # Fourth request goes to model-b
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "original-model", "prompt": "Hello"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "model-b"

#   Scenario: Round-robin with three models
#     Given I deploy an API with the following configuration:
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
#             - name: model-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: model-alpha
#                   - model: model-beta
#                   - model: model-gamma
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "model-alpha"
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "model-beta"
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "model-gamma"
    # Cycle back to first model
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "model-alpha"

#   Scenario: Round-robin with four models
#     Given I deploy an API with the following configuration:
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
#             - name: model-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: gpt-3.5-turbo
#                   - model: gpt-4
#                   - model: claude-3-sonnet
#                   - model: gemini-pro
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "gpt-3.5-turbo"
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "gpt-4"
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "claude-3-sonnet"
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "gemini-pro"

  # ====================================================================
  # MODEL LOCATION: HEADER
  # ====================================================================

#   Scenario: Model selection with header location
#     Given I deploy an API with the following configuration:
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
#             - name: model-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: header-model-1
#                   - model: header-model-2
#                 requestModel:
#                   location: header
#                   identifier: X-AI-Model
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I set the following headers:
#       """
#       X-AI-Model: original-model
#       Content-Type: application/json
#       """
#     And I send a POST request to "/test/chat" with body:
#       """
#       {"prompt": "test"}
#       """
#     Then the response status code should be 200
#     And the response header "X-AI-Model" should be "header-model-1"
#     When I set the following headers:
#       """
#       X-AI-Model: original-model
#       Content-Type: application/json
#       """
#     And I send a POST request to "/test/chat" with body:
#       """
#       {"prompt": "test"}
#       """
#     Then the response status code should be 200
#     And the response header "X-AI-Model" should be "header-model-2"

  # ====================================================================
  # MODEL LOCATION: QUERY PARAMETER
  # ====================================================================

#   Scenario: Model selection with query parameter location
#     Given I deploy an API with the following configuration:
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
#             - name: model-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: query-model-1
#                   - model: query-model-2
#                   - model: query-model-3
#                 requestModel:
#                   location: queryParam
#                   identifier: model
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a GET request to "/test/chat?model=original&prompt=hello"
#     Then the response status code should be 200
#     And the response body should contain "model=query-model-1"
#     When I send a GET request to "/test/chat?model=original&prompt=hello"
#     Then the response status code should be 200
#     And the response body should contain "model=query-model-2"
#     When I send a GET request to "/test/chat?model=original&prompt=hello"
#     Then the response status code should be 200
#     And the response body should contain "model=query-model-3"
#     When I send a GET request to "/test/chat?model=original&prompt=hello"
#     Then the response status code should be 200
#     And the response body should contain "model=query-model-1"

  # ====================================================================
  # MODEL LOCATION: PATH PARAMETER
  # ====================================================================

#   Scenario: Model selection with path parameter location
#     Given I deploy an API with the following configuration:
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
#             - name: model-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: path-model-x
#                   - model: path-model-y
#                 requestModel:
#                   location: pathParam
#                   identifier: /models/([^/]+)/chat
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/models/original-model/chat" with body:
#       """
#       {"prompt": "test"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "/models/path-model-x/chat"
#     When I send a POST request to "/test/models/original-model/chat" with body:
#       """
#       {"prompt": "test"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "/models/path-model-y/chat"

  # ====================================================================
  # NESTED JSON PATH
  # ====================================================================

#   Scenario: Model selection with nested JSONPath
#     Given I deploy an API with the following configuration:
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
#             - name: model-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: nested-model-1
#                   - model: nested-model-2
#                 requestModel:
#                   location: payload
#                   identifier: $.settings.ai.model
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"settings": {"ai": {"model": "original"}}, "prompt": "test"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "nested-model-1"
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"settings": {"ai": {"model": "original"}}, "prompt": "test"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "nested-model-2"

  # ====================================================================
  # MODEL SUSPENSION ON ERRORS
  # ====================================================================

#   Scenario: Suspend model on 5xx error with recovery
#     Given I deploy an API with the following configuration:
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
#             - name: model-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: first-model
#                   - model: second-model
#                   - model: third-model
#                 suspendDuration: 3
#         - method: POST
#           path: /error500
#           policies:
#             - name: model-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: first-model
#                   - model: second-model
#                   - model: third-model
#                 suspendDuration: 3
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
    # First request goes to first-model, returns 500 from backend
#     When I send a POST request to "/test/error500" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 500
    # Next request should skip first-model and use second-model
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "second-model"
    # Next request should use third-model
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "third-model"
    # Next request should use second-model (skip suspended first-model)
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "second-model"

#   Scenario: Suspend model on 429 rate limit error
#     Given I deploy an API with the following configuration:
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
#             - name: model-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: model-1
#                   - model: model-2
#                 suspendDuration: 3
#         - method: POST
#           path: /error429
#           policies:
#             - name: model-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: model-1
#                   - model: model-2
#                 suspendDuration: 3
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
    # First request returns 429 from backend
#     When I send a POST request to "/test/error429" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 429
    # Next request should use model-2 (model-1 is suspended)
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "model-2"
    # Another request should still use model-2
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "model-2"

#   Scenario: All models suspended returns 503
#     Given I deploy an API with the following configuration:
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
#             - name: model-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: model-x
#                   - model: model-y
#                 suspendDuration: 5
#         - method: POST
#           path: /error500
#           policies:
#             - name: model-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: model-x
#                   - model: model-y
#                 suspendDuration: 5
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
    # Trigger 500 error for model-x
#     When I send a POST request to "/test/error500" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 500
    # Trigger 500 error for model-y
#     When I send a POST request to "/test/error500" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 500
    # Now all models are suspended, should return 503
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 503
#     And the response body should contain "All models are currently unavailable"

  # ====================================================================
  # SUSPENSION DISABLED (suspendDuration = 0)
  # ====================================================================

#   Scenario: No suspension when suspendDuration is 0
#     Given I deploy an API with the following configuration:
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
#             - name: model-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: model-one
#                   - model: model-two
#                 suspendDuration: 0
#         - method: POST
#           path: /error500
#           policies:
#             - name: model-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: model-one
#                   - model: model-two
#                 suspendDuration: 0
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
    # First request fails with 500
#     When I send a POST request to "/test/error500" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 500
    # Next request continues rotation to model-two (no suspension)
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "model-two"
    # Next request rotates back to model-one (not suspended)
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "model-one"

  # ====================================================================
  # EDGE CASES
  # ====================================================================

#   Scenario: Handle empty request body
#     Given I deploy an API with the following configuration:
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
#             - name: model-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: selected-model
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/chat" with body:
#       """
#       """
#     Then the response status code should be 400
#     And the response body should contain "Request body is empty"

#   Scenario: Handle invalid JSON in request body
#     Given I deploy an API with the following configuration:
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
#             - name: model-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: selected-model
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/chat" with body:
#       """
#       not valid json [
#       """
#     Then the response status code should be 400
#     And the response body should contain "Invalid JSON in request body"

#   Scenario: Handle invalid JSONPath
#     Given I deploy an API with the following configuration:
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
#             - name: model-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: selected-model
#                 requestModel:
#                   location: payload
#                   identifier: $.does.not.exist
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "test"}
#       """
#     Then the response status code should be 400
#     And the response body should contain "Invalid or missing model"

#   Scenario: Handle missing model field in payload
#     Given I deploy an API with the following configuration:
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
#             - name: model-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: selected-model
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"prompt": "test without model field"}
#       """
#     Then the response status code should be 400
#     And the response body should contain "Invalid or missing model"

#   Scenario: Single model always returns same model
#     Given I deploy an API with the following configuration:
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
#             - name: model-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: only-model
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "only-model"
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "only-model"
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should contain "only-model"

  # ====================================================================
  # REAL-WORLD SCENARIOS
  # ====================================================================

#   Scenario: Equal load balancing across multiple AI providers
#     Given I deploy an API with the following configuration:
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
#             - name: model-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: openai-gpt-4
#                   - model: anthropic-claude-3
#                   - model: google-gemini-pro
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
    # Each model should get equal share of requests
#     When I send a POST request to "/test/completions" with body:
#       """
#       {"model": "any", "messages": [{"role": "user", "content": "Hello"}]}
#       """
#     Then the response status code should be 200
#     And the response body should contain "openai-gpt-4"

#   Scenario: High availability with automatic failover
#     Given I deploy an API with the following configuration:
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
#             - name: model-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: primary-instance
#                   - model: backup-instance-1
#                   - model: backup-instance-2
#                 suspendDuration: 10
#         - method: POST
#           path: /error500
#           policies:
#             - name: model-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: primary-instance
#                   - model: backup-instance-1
#                   - model: backup-instance-2
#                 suspendDuration: 10
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
    # Primary instance fails
#     When I send a POST request to "/test/error500" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 500
    # System automatically fails over to backup instances
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "any"}
#       """
#     Then the response status code should be 200
#     And the response body should match pattern "(backup-instance-1|backup-instance-2)"

#   Scenario: Cost optimization across identical model deployments
#     Given I deploy an API with the following configuration:
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
#             - name: model-round-robin
#               version: v0
#               params:
#                 models:
#                   - model: gpt-4-region-us
#                   - model: gpt-4-region-eu
#                   - model: gpt-4-region-asia
#       """
#     And I wait for the endpoint "http://localhost:8080/test/v0/health" to be ready
    # Distribute load evenly across regions to optimize costs
#     When I send a POST request to "/test/chat" with body:
#       """
#       {"model": "any", "prompt": "test"}
#       """
#     Then the response status code should be 200
