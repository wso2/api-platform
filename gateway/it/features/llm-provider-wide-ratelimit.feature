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

@llm-provider-wide-ratelimit
Feature: Provider-wide rate limiting for LLM providers
  As an API developer
  I want a single rate-limit bucket shared across every resource of an LLM provider
  So that I can enforce one provider-wide quota regardless of which resource is called

  Background:
    Given the gateway services are running

  # Scenario 1 — globalPolicies: one shared bucket across ALL resources (the headline).
  # Exhausting /chat/completions also limits /embeddings because they share a single api-level bucket.
  # RED before the controller change (field unknown or silently ignored → no limit enforced).
  # GREEN after Phases 2–3.
  Scenario: globalPolicies shares one rate-limit bucket across all resources
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha2
      kind: LlmProviderTemplate
      metadata:
        name: global-rl-template
      spec:
        displayName: Global RateLimit Template
        promptTokens:
          location: payload
          identifier: $.json.usage.prompt_tokens
        completionTokens:
          location: payload
          identifier: $.json.usage.completion_tokens
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
      """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha2
      kind: LlmProvider
      metadata:
        name: global-rl-provider
      spec:
        displayName: Global RateLimit Provider
        version: v1.0
        context: /global-rl
        template: global-rl-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [GET]
            - path: /embeddings
              methods: [GET]
        globalPolicies:
          - name: basic-ratelimit
            version: v1
            params:
              limits:
                - requests: 10
                  duration: "1h"
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/global-rl/chat/completions" to be ready

    # Exhaust the shared api-level bucket via /chat/completions
    When I send 20 GET requests to "http://localhost:8080/global-rl/chat/completions"
    Then the response status code should be 429

    # KEY ASSERTION: a SEPARATE resource must ALSO be limited — it shares the same bucket
    When I send a GET request to "http://localhost:8080/global-rl/embeddings"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

  # Scenario 2 — operationPolicies: independent bucket per resource.
  # Exhausting /chat/completions does NOT affect /embeddings (its own bucket).
  # RED before the controller change (field unknown or silently ignored).
  # GREEN after Phases 2–3.
  Scenario: operationPolicies keeps independent rate-limit buckets per resource
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha2
      kind: LlmProviderTemplate
      metadata:
        name: op-rl-template
      spec:
        displayName: Operation RateLimit Template
        promptTokens:
          location: payload
          identifier: $.json.usage.prompt_tokens
        completionTokens:
          location: payload
          identifier: $.json.usage.completion_tokens
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
      """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha2
      kind: LlmProvider
      metadata:
        name: op-rl-provider
      spec:
        displayName: Operation RateLimit Provider
        version: v1.0
        context: /op-rl
        template: op-rl-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [GET]
            - path: /embeddings
              methods: [GET]
        operationPolicies:
          - name: basic-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [GET]
                params:
                  limits:
                    - requests: 10
                      duration: "1h"
              - path: /embeddings
                methods: [GET]
                params:
                  limits:
                    - requests: 10
                      duration: "1h"
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/op-rl/chat/completions" to be ready

    When I send 20 GET requests to "http://localhost:8080/op-rl/chat/completions"
    Then the response status code should be 429

    # Separate resource has its OWN bucket — still served
    When I send a GET request to "http://localhost:8080/op-rl/embeddings"
    Then the response status code should be 200

  # Scenario 3 — combined: globalPolicies caps total traffic; operationPolicies caps /chat/completions independently.
  # /chat/completions is blocked by its tighter per-resource bucket; /embeddings is isolated from that
  # exhaustion but is eventually capped by the shared global bucket.
  # GREEN after Phases 2–3 when both fields are wired up.
  Scenario: globalPolicies and operationPolicies coexist — per-resource cap blocks one resource without affecting another
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha2
      kind: LlmProviderTemplate
      metadata:
        name: combined-rl-template
      spec:
        displayName: Combined RateLimit Template
        promptTokens:
          location: payload
          identifier: $.json.usage.prompt_tokens
        completionTokens:
          location: payload
          identifier: $.json.usage.completion_tokens
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
      """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha2
      kind: LlmProvider
      metadata:
        name: combined-rl-provider
      spec:
        displayName: Combined RateLimit Provider
        version: v1.0
        context: /combined-rl
        template: combined-rl-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [GET]
            - path: /embeddings
              methods: [GET]
        globalPolicies:
          - name: basic-ratelimit
            version: v1
            params:
              limits:
                - requests: 20
                  duration: "1h"
        operationPolicies:
          - name: basic-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [GET]
                params:
                  limits:
                    - requests: 5
                      duration: "1h"
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/combined-rl/chat/completions" to be ready

    # Per-resource op bucket for /chat/completions is exhausted (limit: 5)
    When I send 10 GET requests to "http://localhost:8080/combined-rl/chat/completions"
    Then the response status code should be 429

    # KEY ASSERTION: /embeddings has no op policy — its independent bucket is untouched
    When I send a GET request to "http://localhost:8080/combined-rl/embeddings"
    Then the response status code should be 200

    # Exhaust the shared global bucket via /embeddings (global limit: 20; at least 5 already consumed)
    When I send 25 GET requests to "http://localhost:8080/combined-rl/embeddings"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

  # Scenario 4 — deprecated policies field still works (regression guard).
  # Identical to the operationPolicies scenario but using the legacy policies: field.
  # GREEN before AND after the controller change.
  Scenario: deprecated policies field still enforces independent per-resource rate limits
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha2
      kind: LlmProviderTemplate
      metadata:
        name: legacy-rl-template
      spec:
        displayName: Legacy RateLimit Template
        promptTokens:
          location: payload
          identifier: $.json.usage.prompt_tokens
        completionTokens:
          location: payload
          identifier: $.json.usage.completion_tokens
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
      """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha2
      kind: LlmProvider
      metadata:
        name: legacy-rl-provider
      spec:
        displayName: Legacy RateLimit Provider
        version: v1.0
        context: /legacy-rl
        template: legacy-rl-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [GET]
            - path: /embeddings
              methods: [GET]
        policies:
          - name: basic-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [GET]
                params:
                  limits:
                    - requests: 10
                      duration: "1h"
              - path: /embeddings
                methods: [GET]
                params:
                  limits:
                    - requests: 10
                      duration: "1h"
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/legacy-rl/chat/completions" to be ready

    When I send 20 GET requests to "http://localhost:8080/legacy-rl/chat/completions"
    Then the response status code should be 429

    # Separate resource has its OWN bucket — still served
    When I send a GET request to "http://localhost:8080/legacy-rl/embeddings"
    Then the response status code should be 200
