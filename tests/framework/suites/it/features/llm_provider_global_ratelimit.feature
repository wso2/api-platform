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
# Migrated from gateway/it/features/llm-provider-global-ratelimit.feature.
#
# Mechanical differences from the legacy suite:
#   - Upstream http://echo-backend-multi-arch:8080/anything becomes http://testbench:3000/anything.
#     Every assertion here is a status code (200/429) or the gateway's own "Rate limit exceeded"
#     429 body — none reads the reflected request body back ($.json.*/$.args.*/url/data) — so the
#     plain reflector on :3000 (which 200s on any path) is the correct upstream, per the
#     echo-backend->:3000 rule.
#   - Absolute data-plane URLs (http://localhost:8080/...) reduced to relative paths.
#   - The one authenticate step lives in the Background; the per-scenario duplicates are gone.
#   - Every scenario now cleans up the providers/proxies/templates it created (the legacy file
#     left them behind; the LLM create/deploy steps do not auto-register teardown).
#
# COUNTING: none of these are exact-count scenarios — each exhausts a bucket by sending roughly
# twice its limit and asserts the final request is 429 (or that a sibling resource is/ isn't
# limited). Readiness runs a single GET against /chat/completions to prove the data plane is
# serving before the count starts; that one request cannot flip any 200/429 assertion given the
# 2x margin, and it never touches the separate /embeddings bucket that the isolation scenarios
# assert must still be 200.

@llm-provider-global-ratelimit
Feature: Provider-wide rate limiting for LLM providers
  As an API developer
  I want a single rate-limit bucket shared across every resource of an LLM provider
  So that I can enforce one provider-wide quota regardless of which resource is called

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # Scenario 1 — globalPolicies: one shared bucket across ALL resources (the headline).
  # Exhausting /chat/completions also limits /embeddings because they share a single api-level bucket.
  Scenario: globalPolicies shares one rate-limit bucket across all resources
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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

    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: global-rl-provider
      spec:
        displayName: Global RateLimit Provider
        version: v1.0
        context: /global-rl
        template: global-rl-template
        upstream:
          url: http://testbench:3000/anything
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
    And I send a "GET" request to "/global-rl/chat/completions" until status 200

    # Exhaust the shared api-level bucket via /chat/completions
    When I send 20 "GET" requests to "/global-rl/chat/completions"
    Then the response status code should be 429

    # KEY ASSERTION: a SEPARATE resource must ALSO be limited — it shares the same bucket
    When I send a "GET" request to "/global-rl/embeddings"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

    # Cleanup
    When I delete the LLM provider "global-rl-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "global-rl-template"
    Then the response status code should be 200

  # Scenario 2 — operationPolicies: independent bucket per resource.
  # Exhausting /chat/completions does NOT affect /embeddings (its own bucket).
  Scenario: operationPolicies keeps independent rate-limit buckets per resource
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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

    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: op-rl-provider
      spec:
        displayName: Operation RateLimit Provider
        version: v1.0
        context: /op-rl
        template: op-rl-template
        upstream:
          url: http://testbench:3000/anything
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
    And I send a "GET" request to "/op-rl/chat/completions" until status 200

    When I send 20 "GET" requests to "/op-rl/chat/completions"
    Then the response status code should be 429

    # Separate resource has its OWN bucket — still served
    When I send a "GET" request to "/op-rl/embeddings"
    Then the response status code should be 200

    # Cleanup
    When I delete the LLM provider "op-rl-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "op-rl-template"
    Then the response status code should be 200

  # Scenario 3 — combined: globalPolicies caps total traffic; operationPolicies caps /chat/completions independently.
  # /chat/completions is blocked by its tighter per-resource bucket; /embeddings is isolated from that
  # exhaustion but is eventually capped by the shared global bucket.
  Scenario: globalPolicies and operationPolicies coexist — per-resource cap blocks one resource without affecting another
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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

    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: combined-rl-provider
      spec:
        displayName: Combined RateLimit Provider
        version: v1.0
        context: /combined-rl
        template: combined-rl-template
        upstream:
          url: http://testbench:3000/anything
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
    And I send a "GET" request to "/combined-rl/chat/completions" until status 200

    # Per-resource op bucket for /chat/completions is exhausted (limit: 5)
    When I send 10 "GET" requests to "/combined-rl/chat/completions"
    Then the response status code should be 429

    # KEY ASSERTION: /embeddings has no op policy — its independent bucket is untouched
    When I send a "GET" request to "/combined-rl/embeddings"
    Then the response status code should be 200

    # Exhaust the shared global bucket via /embeddings (global limit: 20; at least 5 already consumed)
    When I send 25 "GET" requests to "/combined-rl/embeddings"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

    # Cleanup
    When I delete the LLM provider "combined-rl-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "combined-rl-template"
    Then the response status code should be 200

  # Scenario 4 — deprecated policies field still works (regression guard).
  # Identical to the operationPolicies scenario but using the legacy policies: field.
  Scenario: deprecated policies field still enforces independent per-resource rate limits
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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

    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: legacy-rl-provider
      spec:
        displayName: Legacy RateLimit Provider
        version: v1.0
        context: /legacy-rl
        template: legacy-rl-template
        upstream:
          url: http://testbench:3000/anything
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
    And I send a "GET" request to "/legacy-rl/chat/completions" until status 200

    When I send 20 "GET" requests to "/legacy-rl/chat/completions"
    Then the response status code should be 429

    # Separate resource has its OWN bucket — still served
    When I send a "GET" request to "/legacy-rl/embeddings"
    Then the response status code should be 200

    # Cleanup
    When I delete the LLM provider "legacy-rl-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "legacy-rl-template"
    Then the response status code should be 200

  # ---------------------------------------------------------------------------
  # Scenarios 5–6: advanced-ratelimit on LLM providers
  # ---------------------------------------------------------------------------

  # Scenario 5 — advanced-ratelimit in globalPolicies with keyExtraction=apiname.
  # All operations share ONE counter. Exhausting /chat/completions also limits
  # /embeddings because the counter key is the API name, not the route.
  Scenario: advanced-ratelimit globalPolicies with keyExtraction=apiname shares one bucket across all provider resources
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: adv-gl-prov-tpl
      spec:
        displayName: Adv Global Provider Template
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

    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: adv-gl-provider
      spec:
        displayName: Adv Global RateLimit Provider
        version: v1.0
        context: /adv-gl
        template: adv-gl-prov-tpl
        upstream:
          url: http://testbench:3000/anything
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
          - name: advanced-ratelimit
            version: v1
            params:
              quotas:
                - name: request-limit
                  limits:
                    - limit: 10
                      duration: "1h"
              keyExtraction:
                - type: apiname
      """
    Then the response status code should be 201
    And I send a "GET" request to "/adv-gl/chat/completions" until status 200

    # Exhaust the shared api-level bucket via /chat/completions
    When I send 20 "GET" requests to "/adv-gl/chat/completions"
    Then the response status code should be 429

    # KEY ASSERTION: /embeddings shares the same apiname-keyed bucket — must also be limited
    When I send a "GET" request to "/adv-gl/embeddings"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

    # Cleanup
    When I delete the LLM provider "adv-gl-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "adv-gl-prov-tpl"
    Then the response status code should be 200

  # Scenario 6 — advanced-ratelimit in operationPolicies without keyExtraction.
  # Default key is routename, so each operation gets its own isolated counter.
  # Exhausting /chat/completions leaves /embeddings unaffected.
  Scenario: advanced-ratelimit operationPolicies without keyExtraction keeps independent buckets per provider resource
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: adv-op-prov-tpl
      spec:
        displayName: Adv Op Provider Template
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

    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: adv-op-provider
      spec:
        displayName: Adv Op RateLimit Provider
        version: v1.0
        context: /adv-op
        template: adv-op-prov-tpl
        upstream:
          url: http://testbench:3000/anything
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
          - name: advanced-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [GET]
                params:
                  quotas:
                    - name: request-limit
                      limits:
                        - limit: 10
                          duration: "1h"
              - path: /embeddings
                methods: [GET]
                params:
                  quotas:
                    - name: request-limit
                      limits:
                        - limit: 10
                          duration: "1h"
      """
    Then the response status code should be 201
    And I send a "GET" request to "/adv-op/chat/completions" until status 200

    When I send 20 "GET" requests to "/adv-op/chat/completions"
    Then the response status code should be 429

    # /embeddings has its own routename-keyed bucket — completely unaffected
    When I send a "GET" request to "/adv-op/embeddings"
    Then the response status code should be 200

    # Cleanup
    When I delete the LLM provider "adv-op-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "adv-op-prov-tpl"
    Then the response status code should be 200

  # ---------------------------------------------------------------------------
  # Scenarios 7–8: advanced-ratelimit on LLM proxies
  # ---------------------------------------------------------------------------

  # Scenario 7 — advanced-ratelimit in proxy globalPolicies with keyExtraction=apiname.
  # The proxy exposes multiple operations; the shared apiname counter spans all of them.
  Scenario: advanced-ratelimit globalPolicies with keyExtraction=apiname shares one bucket across all proxy resources
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: adv-gl-proxy-tpl
      spec:
        displayName: Adv Global Proxy Template
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

    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: adv-gl-proxy-backend
      spec:
        displayName: Adv Global Proxy Backend
        version: v1.0
        context: /adv-gl-pb
        template: adv-gl-proxy-tpl
        upstream:
          url: http://testbench:3000/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201

    When I create LLM proxy with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: adv-gl-proxy
      spec:
        displayName: Adv Global RateLimit Proxy
        version: v1.0
        context: /adv-gl-px
        provider:
          id: adv-gl-proxy-backend
        globalPolicies:
          - name: advanced-ratelimit
            version: v1
            params:
              quotas:
                - name: request-limit
                  limits:
                    - limit: 10
                      duration: "1h"
              keyExtraction:
                - type: apiname
      """
    Then the response status code should be 201
    And I send a "GET" request to "/adv-gl-px/chat/completions" until status 200

    # Exhaust the shared api-level proxy bucket via /chat/completions
    When I send 20 "GET" requests to "/adv-gl-px/chat/completions"
    Then the response status code should be 429

    # KEY ASSERTION: /embeddings shares the same apiname-keyed bucket — must also be limited
    When I send a "GET" request to "/adv-gl-px/embeddings"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

    # Cleanup
    When I send a "DELETE" request to the "gateway-controller" service at "/llm-proxies/adv-gl-proxy"
    Then the response status code should be 200
    When I delete the LLM provider "adv-gl-proxy-backend"
    Then the response status code should be 200
    When I delete the LLM provider template "adv-gl-proxy-tpl"
    Then the response status code should be 200

  # Scenario 8 — advanced-ratelimit in proxy operationPolicies without keyExtraction.
  # Default routename key gives each operation its own isolated counter.
  # Exhausting /chat/completions leaves /embeddings unaffected.
  Scenario: advanced-ratelimit operationPolicies without keyExtraction keeps independent buckets per proxy resource
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: adv-op-proxy-tpl
      spec:
        displayName: Adv Op Proxy Template
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

    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: adv-op-proxy-backend
      spec:
        displayName: Adv Op Proxy Backend
        version: v1.0
        context: /adv-op-pb
        template: adv-op-proxy-tpl
        upstream:
          url: http://testbench:3000/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201

    When I create LLM proxy with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: adv-op-proxy
      spec:
        displayName: Adv Op RateLimit Proxy
        version: v1.0
        context: /adv-op-px
        provider:
          id: adv-op-proxy-backend
        operationPolicies:
          - name: advanced-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [GET]
                params:
                  quotas:
                    - name: request-limit
                      limits:
                        - limit: 10
                          duration: "1h"
      """
    Then the response status code should be 201
    And I send a "GET" request to "/adv-op-px/chat/completions" until status 200

    When I send 20 "GET" requests to "/adv-op-px/chat/completions"
    Then the response status code should be 429

    # /embeddings has its own routename-keyed bucket — completely unaffected
    When I send a "GET" request to "/adv-op-px/embeddings"
    Then the response status code should be 200

    # Cleanup
    When I send a "DELETE" request to the "gateway-controller" service at "/llm-proxies/adv-op-proxy"
    Then the response status code should be 200
    When I delete the LLM provider "adv-op-proxy-backend"
    Then the response status code should be 200
    When I delete the LLM provider template "adv-op-proxy-tpl"
    Then the response status code should be 200

  # ---------------------------------------------------------------------------
  # Scenarios 9–10: mixed advanced-ratelimit (global) + basic-ratelimit (operation)
  # This is the realistic production case: a provider-wide cap enforced by
  # advanced-ratelimit (apiname key) combined with a tighter per-path cap
  # enforced by basic-ratelimit. The operation policy fires first; but because
  # global policies run before operation policies in the chain, the shared global
  # counter is still incremented even for requests that the operation policy
  # ultimately rejects — so /embeddings eventually hits the global cap even
  # though it has no operation policy of its own.
  # ---------------------------------------------------------------------------

  # Scenario 9 — Provider: advanced-ratelimit global (5/hr, apiname) +
  # basic-ratelimit operation (3/hr for /chat/completions).
  Scenario: mixed advanced-ratelimit global and basic-ratelimit operation on provider — global bucket exhausted by rejected operation traffic
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: mix-prov-tpl
      spec:
        displayName: Mix Provider Template
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

    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mix-provider
      spec:
        displayName: Mix RateLimit Provider
        version: v1.0
        context: /mix-prov
        template: mix-prov-tpl
        upstream:
          url: http://testbench:3000/anything
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
          - name: advanced-ratelimit
            version: v1
            params:
              quotas:
                - name: request-limit
                  limits:
                    - limit: 5
                      duration: "1h"
              keyExtraction:
                - type: apiname
        operationPolicies:
          - name: basic-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [GET]
                params:
                  limits:
                    - requests: 3
                      duration: "1h"
      """
    Then the response status code should be 201
    And I send a "GET" request to "/mix-prov/chat/completions" until status 200

    # Operation policy (3/hr) fires before global (5/hr) for /chat/completions.
    # Global still increments on every attempt — including those the operation policy rejects.
    When I send 10 "GET" requests to "/mix-prov/chat/completions"
    Then the response status code should be 429

    # KEY ASSERTION: /embeddings has no operation policy but shares the global
    # apiname bucket. The global was exhausted by /chat/completions traffic,
    # so /embeddings is also blocked.
    When I send a "GET" request to "/mix-prov/embeddings"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

    # Cleanup
    When I delete the LLM provider "mix-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "mix-prov-tpl"
    Then the response status code should be 200

  # Scenario 10 — Proxy: advanced-ratelimit global (5/hr, apiname) +
  # basic-ratelimit operation (3/hr for /chat/completions).
  Scenario: mixed advanced-ratelimit global and basic-ratelimit operation on proxy — global bucket exhausted by rejected operation traffic
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: mix-proxy-tpl
      spec:
        displayName: Mix Proxy Template
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

    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mix-proxy-backend
      spec:
        displayName: Mix Proxy Backend
        version: v1.0
        context: /mix-pb
        template: mix-proxy-tpl
        upstream:
          url: http://testbench:3000/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201

    When I create LLM proxy with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: mix-proxy
      spec:
        displayName: Mix RateLimit Proxy
        version: v1.0
        context: /mix-px
        provider:
          id: mix-proxy-backend
        globalPolicies:
          - name: advanced-ratelimit
            version: v1
            params:
              quotas:
                - name: request-limit
                  limits:
                    - limit: 5
                      duration: "1h"
              keyExtraction:
                - type: apiname
        operationPolicies:
          - name: basic-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [GET]
                params:
                  limits:
                    - requests: 3
                      duration: "1h"
      """
    Then the response status code should be 201
    And I send a "GET" request to "/mix-px/chat/completions" until status 200

    # Operation policy (3/hr) fires before global (5/hr) for /chat/completions.
    # Global still increments on every attempt — including those the operation policy rejects.
    When I send 10 "GET" requests to "/mix-px/chat/completions"
    Then the response status code should be 429

    # KEY ASSERTION: /embeddings has no operation policy but shares the global
    # apiname bucket. The global was exhausted by /chat/completions traffic,
    # so /embeddings is also blocked.
    When I send a "GET" request to "/mix-px/embeddings"
    Then the response status code should be 429
    And the JSON response field "message" should be "Rate limit exceeded. Please try again later."

    # Cleanup
    When I send a "DELETE" request to the "gateway-controller" service at "/llm-proxies/mix-proxy"
    Then the response status code should be 200
    When I delete the LLM provider "mix-proxy-backend"
    Then the response status code should be 200
    When I delete the LLM provider template "mix-proxy-tpl"
    Then the response status code should be 200
