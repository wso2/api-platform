# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------
#
# Migrated from gateway/it/features/token-based-ratelimit.feature.
#
# Every scenario rate-limits on the ACTUAL token usage a request carries, extracted by an
# LlmProviderTemplate from either the reflected request body, the query args, a request header,
# or the provider's real response, and then asserts on X-Ratelimit-Remaining / a 429. The token
# counts each assertion depends on are exact, so the upstream shape and the readiness settle both
# have to be right or the counts land in the wrong place.
#
# Two upstreams stand in for the legacy mocks, both now in the testbench catalog:
#
#   - testbench:3002 (the `echo` service, testbench/services/echo/echo.go) replaces
#     echo-backend-multi-arch:8080. It reflects the request as go-httpbin's /anything did: the
#     parsed body under `json`, the query under `args`, plus url/data — which is exactly what the
#     $.json.usage.* and $.args.* token identifiers read. It also serves /gzip, returning that
#     same reflection GENUINELY gzip-compressed (Content-Encoding: gzip), so the gzipped-response
#     scenario tests that the policy inflates before counting rather than reading plaintext.
#   - testbench:3008 (the `openai` service, testbench/services/openai/openai.go) replaces
#     mock-openapi:4010 for the provider-wide/consumer scenarios. It serves per-endpoint provider
#     bodies ported from the legacy prism spec: /anthropic/v1/messages and
#     /anthropic/v1/messages-web-search each return 50 input + 25 output = 75 tokens, and
#     /mistral/v1/chat/completions returns 100 + 50 with total_tokens 150 — the exact numbers the
#     X-Ratelimit-Remaining assertions (0, 925, 850) require.
#
# Mechanical differences from the legacy suite, applied throughout:
#   - Upstream http://echo-backend-multi-arch:8080/anything -> http://testbench:3002/anything, and
#     the bare http://echo-backend-multi-arch:8080 (the gzip scenario, whose request-rewrite
#     replaces the full path with /gzip) -> http://testbench:3002. Upstream
#     http://mock-openapi:4010 -> http://testbench:3008.
#   - Absolute data-plane URLs (http://localhost:8080/...) reduced to relative paths.
#   - Readiness after a fresh provider CREATE is the legacy suite's FUNCTIONAL probe, kept:
#     `I send a "GET" request to "<the path the scenario invokes>" until status 200`. It GETs that
#     exact path until it answers 200, through the retry funnel's propagation ceiling.
#
#     It costs no quota, which is what the exact-count assertions need: every provider here
#     declares `accessControl: deny_all` with `exceptions: [{path: /chat/completions,
#     methods: [POST, GET]}]`, while the token-based-ratelimit policy is declared `methods:
#     [POST]`. A GET therefore traverses the same route and the same policy chain without being
#     charged. The upstream answers it because both the echo service and the openai mock route on
#     PATH only and never on method.
#
#     It must be the functional probe rather than a structural one, and this is the whole reason:
#     a `deny_all` route answers 404 until the accessControl EXCEPTION reaches the policy engine,
#     which is ext_proc config delivered separately from Envoy's route table. So a 200 here proves
#     BOTH — the RDS push a CREATE races AND the policy chain. Waiting on Envoy's admin
#     config_dump for route PRESENCE proves only the first: it passed while the exception was
#     still in flight, and every request then 404'd. That cost 22 failures the first time this
#     block ran with 10 concurrent runners, having passed clean at 1. `I wait for policy snapshot
#     sync` is not a substitute either — it compares the engine's chain version globally and
#     returns vacuously when that version does not move.
#   - The one authenticate step lives in the Background; the per-scenario/pre-cleanup duplicates
#     are gone. Per-scenario cleanup (already present in the legacy file) is preserved.
#
# ONE SHAPE ADAPTATION, in the gzip scenario only. The legacy template read query args as
# $.args.total_tokens[0] / $.args.model[0], because go-httpbin returned each arg as a JSON ARRAY.
# The testbench echo service returns args as scalar strings ({"total_tokens": "1"}), matching the
# common case the other identifiers already assume, so the [0] index is dropped to
# $.args.total_tokens / $.args.model. This adapts the identifier to the new mock's shape exactly
# as the host swap does; it changes no count and no assertion — total_tokens=1 per request against
# a limit of 2 still yields remaining 1, then 0, then 429.
#

@token-based-ratelimit
Feature: Token-Based Rate Limiting
  As an API developer
  I want to rate limit LLM APIs based on token usage
  So that I can control costs and prevent abuse based on actual resource consumption

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Enforce token-based rate limit on LLM API
    # Create LLM provider template with token extraction paths
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: test-openai-template
      spec:
        displayName: Test OpenAI Template
        promptTokens:
          location: payload
          identifier: $.json.usage.prompt_tokens
        completionTokens:
          location: payload
          identifier: $.json.usage.completion_tokens
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
        requestModel:
          location: payload
          identifier: $.json.model
        responseModel:
          location: payload
          identifier: $.json.model
      """
    Then the response status code should be 201
    And the JSON response field "status.id" should be "test-openai-template"

    # Create LLM provider with token-based-ratelimit policy attached
    # Note: Upstream, accessControl, and policies are required fields
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: test-openai-provider
      spec:
        displayName: Test OpenAI Provider
        version: v1.0
        context: /token-ratelimit
        template: test-openai-template
        upstream:
          url: http://testbench:3002/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: token-based-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  promptTokenLimits:
                    - count: 10
                      duration: "1m"
                  totalTokenLimits:
                    - count: 20
                      duration: "1m"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And the JSON response field "status.state" should be "deployed"
    And I send a "GET" request to "/token-ratelimit/chat/completions" until status 200

    # Send requests and verify rate limiting based on token consumption
    # The echo backend wraps the request body in a 'json' field in the response
    # Must use application/json content-type for the echo backend to parse the body
    Given I set header "Content-Type" to "application/json"

    # First request: consume 5 prompt tokens
    When I send a "POST" request to "/token-ratelimit/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Hello"}],
        "usage": {
          "prompt_tokens": 5,
          "completion_tokens": 3,
          "total_tokens": 8
        }
      }
      """
    Then the response status code should be 200

    # Second request: consume 5 more prompt tokens (total 10/10)
    When I send a "POST" request to "/token-ratelimit/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "World"}],
        "usage": {
          "prompt_tokens": 5,
          "completion_tokens": 3,
          "total_tokens": 8
        }
      }
      """
    Then the response status code should be 200

    # Third request: should be rate limited (prompt token quota exhausted)
    When I send a "POST" request to "/token-ratelimit/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Test"}],
        "usage": {
          "prompt_tokens": 1,
          "completion_tokens": 1,
          "total_tokens": 2
        }
      }
      """
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

    # Cleanup
    When I delete the LLM provider "test-openai-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "test-openai-template"
    Then the response status code should be 200

  Scenario: Token-based rate limit with multiple quotas
    # Create LLM provider template
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: multi-quota-template
      spec:
        displayName: Multi Quota Template
        promptTokens:
          location: payload
          identifier: $.json.usage.prompt_tokens
        completionTokens:
          location: payload
          identifier: $.json.usage.completion_tokens
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
        requestModel:
          location: payload
          identifier: $.json.model
        responseModel:
          location: payload
          identifier: $.json.model
      """
    Then the response status code should be 201

    # Create LLM provider with multiple token quota policies
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: multi-quota-provider
      spec:
        displayName: Multi Quota Provider
        version: v1.0
        context: /multi-quota
        template: multi-quota-template
        upstream:
          url: http://testbench:3002/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: token-based-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  promptTokenLimits:
                    - count: 5
                      duration: "1m"
                  completionTokenLimits:
                    - count: 10
                      duration: "1m"
                  totalTokenLimits:
                    - count: 15
                      duration: "1m"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I send a "GET" request to "/multi-quota/chat/completions" until status 200

    # Must use application/json content-type for the echo backend to parse the body
    Given I set header "Content-Type" to "application/json"

    # Request 1: consume 5 prompt tokens (exhausts prompt quota)
    When I send a "POST" request to "/multi-quota/chat/completions" with body:
      """
      {
        "usage": {
          "prompt_tokens": 5,
          "completion_tokens": 5,
          "total_tokens": 10
        }
      }
      """
    Then the response status code should be 200

    # Request 2: should fail due to prompt token limit (5/5 exhausted)
    # even though completion (5/10) and total (10/15) quotas have room
    When I send a "POST" request to "/multi-quota/chat/completions" with body:
      """
      {
        "usage": {
          "prompt_tokens": 1,
          "completion_tokens": 1,
          "total_tokens": 2
        }
      }
      """
    Then the response status code should be 429

    # Cleanup
    When I delete the LLM provider "multi-quota-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "multi-quota-template"
    Then the response status code should be 200

  Scenario: Token-based rate limiting extracts tokens from gzipped backend responses
    # See the header note: the echo service returns query args as scalar strings, so the legacy
    # $.args.total_tokens[0] / $.args.model[0] (go-httpbin array form) become $.args.total_tokens /
    # $.args.model here. Counts and assertions are unchanged.
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: gzip-response-template
      spec:
        displayName: Gzip Response Template
        totalTokens:
          location: payload
          identifier: $.args.total_tokens
        requestModel:
          location: payload
          identifier: $.args.model
        responseModel:
          location: payload
          identifier: $.args.model
      """
    Then the response status code should be 201

    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: gzip-response-provider
      spec:
        displayName: Gzip Response Provider
        version: v1.0
        context: /gzip-response
        template: gzip-response-template
        upstream:
          url: http://testbench:3002
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: request-rewrite
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST, GET]
                params:
                  pathRewrite:
                    type: ReplaceFullPath
                    replaceFullPath: "/gzip"
          - name: token-based-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  totalTokenLimits:
                    - count: 2
                      duration: "1m"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I send a "GET" request to "/gzip-response/chat/completions" until status 200

    Given I set header "Content-Type" to "application/json"
    And I set header "Accept-Encoding" to "gzip"

    # First request: consume 1 token from gzipped response body
    When I send a "POST" request to "/gzip-response/chat/completions?model=gpt-4&total_tokens=1" with body:
      """
      {}
      """
    Then the response status code should be 200
    And the response header "Content-Encoding" should contain "gzip"
    And the response header "X-Ratelimit-Remaining" should be "1"

    # Second request: consume final token
    When I send a "POST" request to "/gzip-response/chat/completions?model=gpt-4&total_tokens=1" with body:
      """
      {}
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "0"

    # Third request should now be blocked
    When I send a "POST" request to "/gzip-response/chat/completions?model=gpt-4&total_tokens=1" with body:
      """
      {}
      """
    Then the response status code should be 429

    And I reset the request
    When I delete the LLM provider "gzip-response-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "gzip-response-template"
    Then the response status code should be 200

  Scenario: Token-based rate limit returns proper headers
    # Create LLM provider template
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: headers-test-template
      spec:
        displayName: Headers Test Template
        promptTokens:
          location: payload
          identifier: $.json.usage.prompt_tokens
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
        requestModel:
          location: payload
          identifier: $.json.model
        responseModel:
          location: payload
          identifier: $.json.model
      """
    Then the response status code should be 201

    # Create LLM provider with rate limit policy
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: headers-test-provider
      spec:
        displayName: Headers Test Provider
        version: v1.0
        context: /headers-ratelimit
        template: headers-test-template
        upstream:
          url: http://testbench:3002/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: token-based-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  totalTokenLimits:
                    - count: 100
                      duration: "1h"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I send a "GET" request to "/headers-ratelimit/chat/completions" until status 200

    # Must use application/json content-type for the echo backend to parse the body
    Given I set header "Content-Type" to "application/json"

    # Send request and check rate limit headers
    When I send a "POST" request to "/headers-ratelimit/chat/completions" with body:
      """
      {
        "usage": {
          "prompt_tokens": 10,
          "completion_tokens": 5,
          "total_tokens": 15
        }
      }
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should exist
    And the response header "X-RateLimit-Remaining" should exist
    And the response header "X-RateLimit-Reset" should exist

    # Cleanup
    When I delete the LLM provider "headers-test-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "headers-test-template"
    Then the response status code should be 200

  Scenario: Per-provider rate limiting isolation
    # Create two LLM provider templates
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: provider-a-template
      spec:
        displayName: Provider A Template
        promptTokens:
          location: payload
          identifier: $.json.usage.prompt_tokens
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
        requestModel:
          location: payload
          identifier: $.json.model
        responseModel:
          location: payload
          identifier: $.json.model
      """
    Then the response status code should be 201

    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: provider-b-template
      spec:
        displayName: Provider B Template
        promptTokens:
          location: payload
          identifier: $.json.usage.prompt_tokens
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
        requestModel:
          location: payload
          identifier: $.json.model
        responseModel:
          location: payload
          identifier: $.json.model
      """
    Then the response status code should be 201

    # Create two LLM providers with rate limit policies
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: provider-a
      spec:
        displayName: Provider A
        version: v1.0
        context: /provider-a
        template: provider-a-template
        upstream:
          url: http://testbench:3002/anything
          auth:
            type: api-key
            header: Authorization
            value: key-a
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: token-based-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  totalTokenLimits:
                    - count: 5
                      duration: "1h"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201

    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: provider-b
      spec:
        displayName: Provider B
        version: v1.0
        context: /provider-b
        template: provider-b-template
        upstream:
          url: http://testbench:3002/anything
          auth:
            type: api-key
            header: Authorization
            value: key-b
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: token-based-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  totalTokenLimits:
                    - count: 5
                      duration: "1h"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I send a "GET" request to "/provider-a/chat/completions" until status 200
    And I send a "GET" request to "/provider-b/chat/completions" until status 200

    # Must use application/json content-type for the echo backend to parse the body
    Given I set header "Content-Type" to "application/json"

    # Exhaust Provider A's quota (limit is 5 per hour)
    # Use multiple requests since cost is extracted from response
    When I send a "POST" request to "/provider-a/chat/completions" with body:
      """
      {
        "usage": {
          "prompt_tokens": 2,
          "completion_tokens": 1,
          "total_tokens": 3
        }
      }
      """
    Then the response status code should be 200

    # Second request: 2 more tokens (total: 5, remaining: 0)
    When I send a "POST" request to "/provider-a/chat/completions" with body:
      """
      {
        "usage": {
          "prompt_tokens": 1,
          "completion_tokens": 1,
          "total_tokens": 2
        }
      }
      """
    Then the response status code should be 200

    # Provider A should be rate limited (quota exhausted)
    When I send a "POST" request to "/provider-a/chat/completions" with body:
      """
      {
        "usage": {
          "prompt_tokens": 1,
          "completion_tokens": 0,
          "total_tokens": 1
        }
      }
      """
    Then the response status code should be 429

    # Provider B should still work (independent quota)
    When I send a "POST" request to "/provider-b/chat/completions" with body:
      """
      {
        "usage": {
          "prompt_tokens": 3,
          "completion_tokens": 3,
          "total_tokens": 6
        }
      }
      """
    Then the response status code should be 200

    # Cleanup
    When I delete the LLM provider "provider-a"
    Then the response status code should be 200
    When I delete the LLM provider "provider-b"
    Then the response status code should be 200
    When I delete the LLM provider template "provider-a-template"
    Then the response status code should be 200
    When I delete the LLM provider template "provider-b-template"
    Then the response status code should be 200

  Scenario: Multiple quotas are enforced independently with correct quota identified
    # Create template with all three token types
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: multi-quota-detailed-template
      spec:
        displayName: Multi Quota Detailed Template
        promptTokens:
          location: payload
          identifier: $.json.usage.prompt_tokens
        completionTokens:
          location: payload
          identifier: $.json.usage.completion_tokens
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
        requestModel:
          location: payload
          identifier: $.json.model
        responseModel:
          location: payload
          identifier: $.json.model
      """
    Then the response status code should be 201

    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: multi-quota-detailed-provider
      spec:
        displayName: Multi Quota Detailed Provider
        version: v1.0
        context: /multi-quota-detailed
        template: multi-quota-detailed-template
        upstream:
          url: http://testbench:3002/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: token-based-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  promptTokenLimits:
                    - count: 10
                      duration: "1m"
                  completionTokenLimits:
                    - count: 20
                      duration: "1m"
                  totalTokenLimits:
                    - count: 25
                      duration: "1m"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I send a "GET" request to "/multi-quota-detailed/chat/completions" until status 200

    # Must use application/json content-type for the echo backend to parse the body
    Given I set header "Content-Type" to "application/json"

    # First request: 8 prompt + 15 completion = 23 total
    # All within limits: prompt(8/10), completion(15/20), total(23/25)
    When I send a "POST" request to "/multi-quota-detailed/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "usage": {
          "prompt_tokens": 8,
          "completion_tokens": 15,
          "total_tokens": 23
        }
      }
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "2"

    # Second request: 3 prompt + 5 completion = 8 total
    # With response-phase cost extraction, request is allowed (cost not known yet)
    # completion_tokens becomes 15+5=20 (exhausted)
    # prompt_tokens becomes 8+3=11 (exceeded after consumption)
    # total_tokens becomes 23+8=31 (exceeded after consumption)
    When I send a "POST" request to "/multi-quota-detailed/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "usage": {
          "prompt_tokens": 3,
          "completion_tokens": 5,
          "total_tokens": 8
        }
      }
      """
    Then the response status code should be 200
    # completion_tokens is now exhausted (20/20), which is the most restrictive
    And the response header "X-Ratelimit-Remaining" should be "0"

    # Third request: should be blocked because completion_tokens quota is exhausted
    When I send a "POST" request to "/multi-quota-detailed/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "usage": {
          "prompt_tokens": 1,
          "completion_tokens": 1,
          "total_tokens": 2
        }
      }
      """
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."
    # Verify correct quota is identified in header (if supported)
    And the response header "X-Ratelimit-Quota" should exist

    # Cleanup
    When I delete the LLM provider "multi-quota-detailed-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "multi-quota-detailed-template"
    Then the response status code should be 200

  Scenario: Rate limit window resets after time window expires
    # Create provider with short 10-second window for testing
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: window-test-template
      spec:
        displayName: Window Test Template
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
        requestModel:
          location: payload
          identifier: $.json.model
        responseModel:
          location: payload
          identifier: $.json.model
      """
    Then the response status code should be 201

    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: window-test-provider
      spec:
        displayName: Window Test Provider
        version: v1.0
        context: /window-test
        template: window-test-template
        upstream:
          url: http://testbench:3002/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: token-based-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  totalTokenLimits:
                    - count: 5
                      duration: "10s"  # Short window for testing
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I send a "GET" request to "/window-test/chat/completions" until status 200

    Given I set header "Content-Type" to "application/json"

    # Exhaust the quota (5 tokens in 10 seconds)
    When I send a "POST" request to "/window-test/chat/completions" with body:
      """
      {
        "usage": {"total_tokens": 5}
      }
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "0"

    # Immediate next request should be rate limited
    When I send a "POST" request to "/window-test/chat/completions" with body:
      """
      {
        "usage": {"total_tokens": 1}
      }
      """
    Then the response status code should be 429

    # Poll until the window rolls, replacing legacy's fixed 11-second sleep. While exhausted the
    # 429s are free (refused pre-flight, no upstream call, so no usage to charge). The request
    # that finally succeeds IS the "after reset it works" assertion — a separate POST after this
    # would spend a second token and leave remaining at 3, not 4. Same body as the requests
    # above, so it charges the same 1 token.
    When I send a "POST" request to "/window-test/chat/completions" until status 200 with body:
      """
      {
        "usage": {"total_tokens": 1}
      }
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "4"

    # Cleanup
    When I delete the LLM provider "window-test-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "window-test-template"
    Then the response status code should be 200

  Scenario: Zero token usage should not consume quota
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: zero-token-template
      spec:
        displayName: Zero Token Test Template
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
        requestModel:
          location: payload
          identifier: $.json.model
        responseModel:
          location: payload
          identifier: $.json.model
      """
    Then the response status code should be 201

    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: zero-token-provider
      spec:
        displayName: Zero Token Test Provider
        version: v1.0
        context: /zero-token-test
        template: zero-token-template
        upstream:
          url: http://testbench:3002/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: token-based-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  totalTokenLimits:
                    - count: 10
                      duration: "1h"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I send a "GET" request to "/zero-token-test/chat/completions" until status 200

    Given I set header "Content-Type" to "application/json"

    # First request: 5 tokens
    When I send a "POST" request to "/zero-token-test/chat/completions" with body:
      """
      {
        "usage": {"total_tokens": 5}
      }
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "5"

    # Second request: 0 tokens should not consume quota
    When I send a "POST" request to "/zero-token-test/chat/completions" with body:
      """
      {
        "usage": {"total_tokens": 0}
      }
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "5"

    # Third request: 5 tokens (should still be within limit)
    When I send a "POST" request to "/zero-token-test/chat/completions" with body:
      """
      {
        "usage": {"total_tokens": 5}
      }
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "0"

    # Fourth request: should be rate limited
    When I send a "POST" request to "/zero-token-test/chat/completions" with body:
      """
      {
        "usage": {"total_tokens": 1}
      }
      """
    Then the response status code should be 429

    # Cleanup
    When I delete the LLM provider "zero-token-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "zero-token-template"
    Then the response status code should be 200

  Scenario: Cost extraction from request headers blocks immediately
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: header-cost-template
      spec:
        displayName: Header Cost Test Template
        totalTokens:
          location: header
          identifier: X-Token-Cost
        requestModel:
          location: payload
          identifier: $.json.model
        responseModel:
          location: payload
          identifier: $.json.model
      """
    Then the response status code should be 201

    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: header-cost-provider
      spec:
        displayName: Header Cost Test Provider
        version: v1.0
        context: /header-cost-test
        template: header-cost-template
        upstream:
          url: http://testbench:3002/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: token-based-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  totalTokenLimits:
                    - count: 10
                      duration: "1h"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I send a "GET" request to "/header-cost-test/chat/completions" until status 200

    Given I set header "Content-Type" to "application/json"
    And I set header "X-Token-Cost" to "6"

    # First request: 6 tokens via header (request-phase extraction)
    When I send a "POST" request to "/header-cost-test/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "4"

    # Second request: 6 tokens would exceed limit (10), should block immediately
    When I send a "POST" request to "/header-cost-test/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

    # Cleanup
    When I delete the LLM provider "header-cost-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "header-cost-template"
    Then the response status code should be 200

  Scenario: Template change triggers cache invalidation
    # Create initial template
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: change-test-template
      spec:
        displayName: Change Test Template
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
        requestModel:
          location: payload
          identifier: $.json.model
        responseModel:
          location: payload
          identifier: $.json.model
      """
    Then the response status code should be 201

    # Create provider with limit of 5
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: change-test-provider
      spec:
        displayName: Change Test Provider
        version: v1.0
        context: /change-test
        template: change-test-template
        upstream:
          url: http://testbench:3002/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: token-based-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  totalTokenLimits:
                    - count: 5
                      duration: "1h"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I send a "GET" request to "/change-test/chat/completions" until status 200

    Given I set header "Content-Type" to "application/json"

    # Use up the quota (5/5)
    When I send a "POST" request to "/change-test/chat/completions" with body:
      """
      {"usage": {"total_tokens": 5}}
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "0"

    # Next request should be blocked
    When I send a "POST" request to "/change-test/chat/completions" with body:
      """
      {"usage": {"total_tokens": 1}}
      """
    Then the response status code should be 429

    # Delete and recreate provider with higher limit
    When I delete the LLM provider "change-test-provider"
    Then the response status code should be 200

    # Recreate provider with limit of 10 (should get fresh quota). The route-readiness after the
    # recreate stands in for the legacy `wait 2 seconds` settle — there is no sleep step, and
    # policy snapshot sync is not a route-readiness primitive. Because the recreated route matches
    # the same path as the deleted one, this cannot distinguish the new route from a not-yet-removed
    # old one; the fresh-quota assertion below is what proves the recreate took effect.
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: change-test-provider
      spec:
        displayName: Change Test Provider
        version: v1.0
        context: /change-test
        template: change-test-template
        upstream:
          url: http://testbench:3002/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: token-based-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  totalTokenLimits:
                    - count: 10
                      duration: "1h"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    Given I set header "Content-Type" to "application/json"

    # POLLED, not sampled once. The provider was deleted and recreated at the SAME path, so a
    # route-presence probe cannot tell the new one from the old — both are POST on
    # /change-test/chat/completions. What distinguishes them is behaviour: the old provider's
    # quota was exhausted above and answers 429, the new one has a limit of 10 and answers 200.
    # Polling that is the only honest way to wait for the replacement to take effect.
    #
    # Safe here, and only here: this scenario asserts a FRESH allowance rather than an exact
    # count, so a few probes cannot exhaust a limit that was just reset. In an exact-count
    # scenario this step would spend the very budget under test.
    When I send a "POST" request to "/change-test/chat/completions" until status 200 with body:
      """
      {"usage": {"total_tokens": 5}}
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "5"

    # Cleanup
    When I delete the LLM provider "change-test-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "change-test-template"
    Then the response status code should be 200

  Scenario: Different providers with same template have isolated quotas
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: shared-template
      spec:
        displayName: Shared Template
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
        requestModel:
          location: payload
          identifier: $.json.model
        responseModel:
          location: payload
          identifier: $.json.model
      """
    Then the response status code should be 201

    # Create first provider
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: provider-alpha
      spec:
        displayName: Provider Alpha
        version: v1.0
        context: /provider-alpha
        template: shared-template
        upstream:
          url: http://testbench:3002/anything
          auth:
            type: api-key
            header: Authorization
            value: key-alpha
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: token-based-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  totalTokenLimits:
                    - count: 5
                      duration: "1h"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201

    # Create second provider with same template
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: provider-beta
      spec:
        displayName: Provider Beta
        version: v1.0
        context: /provider-beta
        template: shared-template
        upstream:
          url: http://testbench:3002/anything
          auth:
            type: api-key
            header: Authorization
            value: key-beta
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: token-based-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  totalTokenLimits:
                    - count: 5
                      duration: "1h"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I send a "GET" request to "/provider-alpha/chat/completions" until status 200
    And I send a "GET" request to "/provider-beta/chat/completions" until status 200

    Given I set header "Content-Type" to "application/json"

    # Exhaust provider-alpha's quota
    When I send a "POST" request to "/provider-alpha/chat/completions" with body:
      """
      {"usage": {"total_tokens": 5}}
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "0"

    # provider-alpha should now be blocked
    When I send a "POST" request to "/provider-alpha/chat/completions" with body:
      """
      {"usage": {"total_tokens": 1}}
      """
    Then the response status code should be 429

    # provider-beta should still have full quota
    When I send a "POST" request to "/provider-beta/chat/completions" with body:
      """
      {"usage": {"total_tokens": 3}}
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "2"

    # Cleanup
    When I delete the LLM provider "provider-alpha"
    Then the response status code should be 200
    When I delete the LLM provider "provider-beta"
    Then the response status code should be 200
    When I delete the LLM provider template "shared-template"
    Then the response status code should be 200

  Scenario: Empty prompt/completion limits with total-only limit still enforces rate limiting
    # Create template with all token extraction paths
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: empty-limits-template
      spec:
        displayName: Empty Limits Template
        promptTokens:
          location: payload
          identifier: $.json.usage.prompt_tokens
        completionTokens:
          location: payload
          identifier: $.json.usage.completion_tokens
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
        requestModel:
          location: payload
          identifier: $.json.model
        responseModel:
          location: payload
          identifier: $.json.model
      """
    Then the response status code should be 201

    # Create provider with explicit empty prompt/completion limit arrays and only total limit configured
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: empty-limits-provider
      spec:
        displayName: Empty Limits Provider
        version: v1.0
        context: /empty-limits
        template: empty-limits-template
        upstream:
          url: http://testbench:3002/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: token-based-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  promptTokenLimits: []
                  completionTokenLimits: []
                  totalTokenLimits:
                    - count: 5
                      duration: "1m"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I send a "GET" request to "/empty-limits/chat/completions" until status 200

    # Must use application/json content-type for the echo backend to parse the body
    Given I set header "Content-Type" to "application/json"

    # First request consumes the entire total token quota
    When I send a "POST" request to "/empty-limits/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "usage": {
          "prompt_tokens": 0,
          "completion_tokens": 5,
          "total_tokens": 5
        }
      }
      """
    Then the response status code should be 200

    # Next request should be blocked by total token quota
    When I send a "POST" request to "/empty-limits/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "usage": {
          "prompt_tokens": 0,
          "completion_tokens": 1,
          "total_tokens": 1
        }
      }
      """
    Then the response status code should be 429

    # Cleanup
    When I delete the LLM provider "empty-limits-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "empty-limits-template"
    Then the response status code should be 200

  Scenario: Empty completion/total limits with prompt-only limit still enforces rate limiting
    # Create template with all token extraction paths
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: prompt-only-empty-limits-template
      spec:
        displayName: Prompt Only Empty Limits Template
        promptTokens:
          location: payload
          identifier: $.json.usage.prompt_tokens
        completionTokens:
          location: payload
          identifier: $.json.usage.completion_tokens
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
        requestModel:
          location: payload
          identifier: $.json.model
        responseModel:
          location: payload
          identifier: $.json.model
      """
    Then the response status code should be 201

    # Create provider with only prompt limits configured
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: prompt-only-empty-limits-provider
      spec:
        displayName: Prompt Only Empty Limits Provider
        version: v1.0
        context: /prompt-only-empty-limits
        template: prompt-only-empty-limits-template
        upstream:
          url: http://testbench:3002/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: token-based-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  promptTokenLimits:
                    - count: 5
                      duration: "1m"
                  completionTokenLimits: []
                  totalTokenLimits: []
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I send a "GET" request to "/prompt-only-empty-limits/chat/completions" until status 200

    Given I set header "Content-Type" to "application/json"

    # First request consumes the entire prompt token quota
    When I send a "POST" request to "/prompt-only-empty-limits/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "usage": {
          "prompt_tokens": 5,
          "completion_tokens": 0,
          "total_tokens": 5
        }
      }
      """
    Then the response status code should be 200

    # Next request should be blocked by prompt token quota
    When I send a "POST" request to "/prompt-only-empty-limits/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "usage": {
          "prompt_tokens": 1,
          "completion_tokens": 0,
          "total_tokens": 1
        }
      }
      """
    Then the response status code should be 429

    # Cleanup
    When I delete the LLM provider "prompt-only-empty-limits-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "prompt-only-empty-limits-template"
    Then the response status code should be 200

  Scenario: Empty prompt/total limits with completion-only limit still enforces rate limiting
    # Create template with all token extraction paths
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: completion-only-empty-limits-template
      spec:
        displayName: Completion Only Empty Limits Template
        promptTokens:
          location: payload
          identifier: $.json.usage.prompt_tokens
        completionTokens:
          location: payload
          identifier: $.json.usage.completion_tokens
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
        requestModel:
          location: payload
          identifier: $.json.model
        responseModel:
          location: payload
          identifier: $.json.model
      """
    Then the response status code should be 201

    # Create provider with only completion limits configured
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: completion-only-empty-limits-provider
      spec:
        displayName: Completion Only Empty Limits Provider
        version: v1.0
        context: /completion-only-empty-limits
        template: completion-only-empty-limits-template
        upstream:
          url: http://testbench:3002/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: token-based-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  promptTokenLimits: []
                  completionTokenLimits:
                    - count: 5
                      duration: "1m"
                  totalTokenLimits: []
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I send a "GET" request to "/completion-only-empty-limits/chat/completions" until status 200

    Given I set header "Content-Type" to "application/json"

    # First request consumes the entire completion token quota
    When I send a "POST" request to "/completion-only-empty-limits/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "usage": {
          "prompt_tokens": 0,
          "completion_tokens": 5,
          "total_tokens": 5
        }
      }
      """
    Then the response status code should be 200

    # Next request should be blocked by completion token quota
    When I send a "POST" request to "/completion-only-empty-limits/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "usage": {
          "prompt_tokens": 0,
          "completion_tokens": 1,
          "total_tokens": 1
        }
      }
      """
    Then the response status code should be 429

    # Cleanup
    When I delete the LLM provider "completion-only-empty-limits-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "completion-only-empty-limits-template"
    Then the response status code should be 200

  Scenario: Provider-wide total token quota charges actual token usage and is shared across resources
    # Anthropic's Messages API response has no total-token field, so the built-in "anthropic"
    # template defines only promptTokens/completionTokens. The total_tokens quota must fall
    # back to summing them (50 input + 25 output = 75) instead of charging a flat 1 per request.
    # Attached provider-wide (globalPolicies), the quota must also be shared across every
    # resource of the provider, not siloed per path.
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: prov-wide-anthropic-provider
      spec:
        displayName: Provider Wide Anthropic Provider
        version: v1.0
        context: /prov-wide-anthropic
        template: anthropic
        upstream:
          url: http://testbench:3008
          auth:
            type: api-key
            header: Authorization
            value: test-key
        accessControl:
          mode: allow_all
        globalPolicies:
          - name: token-based-ratelimit
            version: v1
            params:
              totalTokenLimits:
                - count: 75
                  duration: "1h"
      """
    Then the response status code should be 201
    # Charged nothing: /__readiness reports zero token usage in every template dialect, so the
    # 75-token budget below stays intact. One resource is enough — the provider's routes are
    # programmed as a unit, so a 404 on another of its paths later is a real defect to surface,
    # not a race to wait out.
    And I send a "POST" request to "/prov-wide-anthropic/__readiness" until status 200

    Given I set header "Content-Type" to "application/json"

    # Consumes 75 tokens (50 input + 25 output) of the 75-token quota — remaining hits 0.
    # Before the fix, this would charge 1 and leave remaining at 74.
    When I send a "POST" request to "/prov-wide-anthropic/anthropic/v1/messages" with body:
      """
      {"model": "claude-3-5-haiku-20241022", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100}
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "0"

    # Quota is already exhausted — blocked at the pre-flight check before reaching upstream.
    When I send a "POST" request to "/prov-wide-anthropic/anthropic/v1/messages" with body:
      """
      {"model": "claude-3-5-haiku-20241022", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100}
      """
    Then the response status code should be 429

    # KEY ASSERTION: a DIFFERENT resource on the same provider is also blocked — proves the
    # quota is one shared apiname-keyed bucket, not an independent per-route bucket.
    When I send a "POST" request to "/prov-wide-anthropic/anthropic/v1/messages-web-search" with body:
      """
      {"model": "claude-3-5-haiku-20241022", "messages": [{"role": "user", "content": "search"}], "max_tokens": 100}
      """
    Then the response status code should be 429

    # Cleanup
    When I delete the LLM provider "prov-wide-anthropic-provider"
    Then the response status code should be 200

  Scenario: Total token quota does not double-count when the template already defines totalTokens
    # Regression guard: the "mistralai" built-in template defines promptTokens, completionTokens,
    # AND totalTokens. The total_tokens quota must be charged from totalTokens alone (150) —
    # never the sum of all three (300) — so templates that already worked correctly are not
    # broken by the fallback added for templates like Anthropic that have no totalTokens.
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: prov-wide-mistral-provider
      spec:
        displayName: Provider Wide Mistral Provider
        version: v1.0
        context: /prov-wide-mistral
        template: mistralai
        upstream:
          url: http://testbench:3008
          auth:
            type: api-key
            header: Authorization
            value: test-key
        accessControl:
          mode: allow_all
        globalPolicies:
          - name: token-based-ratelimit
            version: v1
            params:
              totalTokenLimits:
                - count: 1000
                  duration: "1h"
      """
    Then the response status code should be 201
    # Readiness gate: a real request, so it proves the route AND the ext_proc policy are live —
    # sent to /__readiness, which reports zero token usage in every template dialect, so it is
    # charged 0 and the 850 assertion below is unaffected.
    #
    # Replaces a wait that read Envoy's config_dump. Routes reach Envoy over RDS while policy
    # chains reach the engine on a separate path, so the dump showed the route as present while
    # the next request still answered 404 {"error":"Not Found"}. That wait PASSED and the request
    # failed; a longer timeout cannot help a condition that is already true.
    And I send a "POST" request to "/prov-wide-mistral/__readiness" until status 200

    Given I set header "Content-Type" to "application/json"

    # 100 prompt + 50 completion = 150 total_tokens. If double-counted (150+100+50=300),
    # remaining would be 700 instead of 850.
    When I send a "POST" request to "/prov-wide-mistral/mistral/v1/chat/completions" with body:
      """
      {"model": "mistral-small-latest", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "850"

    # Cleanup
    When I delete the LLM provider "prov-wide-mistral-provider"
    Then the response status code should be 200

  Scenario: Consumer-level total token quota also charges actual token usage, not a flat 1 per request
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: consumer-anthropic-provider
      spec:
        displayName: Consumer Anthropic Provider
        version: v1.0
        context: /consumer-anthropic
        template: anthropic
        upstream:
          url: http://testbench:3008
          auth:
            type: api-key
            header: Authorization
            value: test-key
        accessControl:
          mode: allow_all
        policies:
          - name: token-based-ratelimit
            version: v1
            paths:
              - path: /*
                methods: ['*']
                params:
                  totalTokenLimits:
                    - count: 1000
                      duration: "1h"
                  algorithm: fixed-window
                  backend: memory
                  consumerBased: true
      """
    Then the response status code should be 201
    And I send a "GET" request to "/consumer-anthropic/anthropic/v1/messages" until status 200

    Given I set header "Content-Type" to "application/json"

    # 50 input + 25 output = 75 tokens consumed of the 1000-token quota — remaining is 925.
    # Before the fix, this would charge 1 and leave remaining at 999.
    When I send a "POST" request to "/consumer-anthropic/anthropic/v1/messages" with body:
      """
      {"model": "claude-3-5-haiku-20241022", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100}
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "925"

    # Cleanup
    When I delete the LLM provider "consumer-anthropic-provider"
    Then the response status code should be 200
