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

@policy-path-specificity @llm
Feature: LLM policy path and method specificity
  As an API developer
  I want a policy attached to overlapping paths and methods on an LLM provider or proxy
  to apply only the most specific match (path first, then method) to each request
  So that overlapping path/method policies do not stack on the same route
  # Note: this applies to LlmProvider and LlmProxy (both go through the LLM transformer);
  # RestApi operations use explicit per-operation policies and are unaffected.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ------------------------------------------------------------------
  # Reproduces a path-specificity bug using an LLM provider with the
  # advanced-ratelimit policy attached to TWO overlapping paths:
  #
  #   - POST /chat/completions -> 4 requests / hour   (specific)
  #   - /* (all methods)       -> 1 request  / hour   (wildcard catch-all)
  #
  # EXPECTED: a POST to /chat/completions is governed ONLY by the specific
  #           4/hour quota; the 1/hour wildcard quota governs every OTHER path.
  #
  # ACTUAL (bug): the controller's LLM transformer fans the wildcard policy out
  #           onto the /chat/completions operation too, so that route's policy
  #           chain ends up with BOTH advanced-ratelimit instances (visible in
  #           the policy-engine config dump). The 1/hour wildcard quota is then
  #           also enforced on /chat/completions and blocks the 2nd request.
  #
  # This scenario asserts the EXPECTED behaviour, so it FAILS on the current
  # (buggy) gateway at request #2 — which is how it validates the issue — and
  # PASSES once the most-specific-path-wins fix is in place.
  # ------------------------------------------------------------------
  Scenario: Overlapping specific and wildcard advanced-ratelimit paths apply only the most specific match
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: advrl-path-template
      spec:
        displayName: AdvRL Path Specificity Template
      """
    Then the response status code should be 201

    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: advrl-path-provider
      spec:
        displayName: AdvRL Path Specificity Provider
        version: v1.0
        context: /advrl-path
        template: advrl-path-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: allow_all
        policies:
          - name: advanced-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  quotas:
                    - name: chat-quota
                      limits:
                        - limit: 4
                          duration: "1h"
              - path: /*
                methods:
                  - '*'
                params:
                  quotas:
                    - name: wildcard-quota
                      limits:
                        - limit: 1
                          duration: "1h"
      """
    Then the response status code should be 201
    And I wait for 2 seconds
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    # ----- /chat/completions must be governed ONLY by the specific 4/hour quota -----

    # Request 1 - allowed (chat-quota 1/4)
    When I send a POST request to "http://localhost:8080/advrl-path/chat/completions" with body:
      """
      {"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # Request 2 - EXPECTED allowed (chat-quota 2/4).
    # BUG: current gateway returns 429 here because the /* wildcard quota (1/hour)
    # is wrongly applied to /chat/completions and is already exhausted after request 1.
    When I send a POST request to "http://localhost:8080/advrl-path/chat/completions" with body:
      """
      {"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # Request 3 - allowed (chat-quota 3/4)
    When I send a POST request to "http://localhost:8080/advrl-path/chat/completions" with body:
      """
      {"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # Request 4 - allowed (chat-quota 4/4)
    When I send a POST request to "http://localhost:8080/advrl-path/chat/completions" with body:
      """
      {"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # Request 5 - blocked: the specific 4/hour chat-quota is now exhausted
    When I send a POST request to "http://localhost:8080/advrl-path/chat/completions" with body:
      """
      {"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    # ----- every OTHER path is governed by the 1/hour wildcard quota (intended) -----

    # Request 1 to /embeddings - allowed (wildcard-quota 1/1)
    When I send a POST request to "http://localhost:8080/advrl-path/embeddings" with body:
      """
      {"model": "text-embedding-3-small", "input": "Hello"}
      """
    Then the response status code should be 200

    # Request 2 to /embeddings - blocked by the 1/hour wildcard quota (intended)
    When I send a POST request to "http://localhost:8080/advrl-path/embeddings" with body:
      """
      {"model": "text-embedding-3-small", "input": "Hello"}
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "advrl-path-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "advrl-path-template"
    Then the response status code should be 200

  # ------------------------------------------------------------------
  # Generalises the fix beyond the root /*: the SAME policy attached to three
  # overlapping paths of decreasing specificity must apply only the most specific
  # match to each request -
  #   - /chat/completions -> 4 / hour   (exact, most specific)
  #   - /chat/*           -> 2 / hour   (nested wildcard)
  #   - /*                -> 1 / hour   (root catch-all)
  # Distinct quota names keep the per-path buckets isolated, and the X-RateLimit-Limit
  # header proves which quota governs each path. A naive "/* only" fix would still
  # wrongly stack /chat/* onto /chat/completions; this asserts that does not happen.
  # ------------------------------------------------------------------
  Scenario: Overlapping nested paths each apply only their most specific advanced-ratelimit
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: nestedrl-template
      spec:
        displayName: Nested RL Template
      """
    Then the response status code should be 201

    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: nestedrl-provider
      spec:
        displayName: Nested RL Provider
        version: v1.0
        context: /nestedrl
        template: nestedrl-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: allow_all
        policies:
          - name: advanced-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods:
                  - '*'
                params:
                  quotas:
                    - name: chat-exact-quota
                      limits:
                        - limit: 4
                          duration: "1h"
              - path: /chat/*
                methods:
                  - '*'
                params:
                  quotas:
                    - name: chat-wild-quota
                      limits:
                        - limit: 2
                          duration: "1h"
              - path: /*
                methods:
                  - '*'
                params:
                  quotas:
                    - name: root-wild-quota
                      limits:
                        - limit: 1
                          duration: "1h"
      """
    Then the response status code should be 201
    And I wait for 2 seconds
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    # /chat/completions -> governed ONLY by the exact-path quota (4/hour)
    When I send a POST request to "http://localhost:8080/nestedrl/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should be "4"
    When I send a POST request to "http://localhost:8080/nestedrl/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/nestedrl/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/nestedrl/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    # 5th request exceeds the exact-path limit of 4
    When I send a POST request to "http://localhost:8080/nestedrl/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 429

    # /chat/<other> -> governed ONLY by the nested wildcard /chat/* quota (2/hour)
    When I send a POST request to "http://localhost:8080/nestedrl/chat/embeddings" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should be "2"
    When I send a POST request to "http://localhost:8080/nestedrl/chat/embeddings" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    # 3rd request exceeds the /chat/* limit of 2
    When I send a POST request to "http://localhost:8080/nestedrl/chat/embeddings" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 429

    # any other path -> governed ONLY by the root /* quota (1/hour)
    When I send a POST request to "http://localhost:8080/nestedrl/models" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should be "1"
    # 2nd request exceeds the /* limit of 1
    When I send a POST request to "http://localhost:8080/nestedrl/models" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "nestedrl-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "nestedrl-template"
    Then the response status code should be 200

  # ------------------------------------------------------------------
  # Same path with different methods + order independence. The wildcard paths are
  # declared FIRST (before the specific /chat/completions entries) to prove the
  # outcome does not depend on declaration order:
  #   - POST /chat/completions -> 4  / hour
  #   - GET  /chat/completions -> 10 / hour
  #   - /chat/* (any method)   -> 2  / hour
  #   - /*      (any method)   -> 1  / hour
  # ------------------------------------------------------------------
  Scenario: Per-method limits on the same path win over wildcards regardless of declaration order
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: methodrl-template
      spec:
        displayName: Method RL Template
      """
    Then the response status code should be 201

    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: methodrl-provider
      spec:
        displayName: Method RL Provider
        version: v1.0
        context: /methodrl
        template: methodrl-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: allow_all
        policies:
          - name: advanced-ratelimit
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  quotas:
                    - name: root-quota
                      limits:
                        - limit: 1
                          duration: "1h"
              - path: /chat/*
                methods:
                  - '*'
                params:
                  quotas:
                    - name: chatwild-quota
                      limits:
                        - limit: 2
                          duration: "1h"
              - path: /chat/completions
                methods:
                  - 'GET'
                params:
                  quotas:
                    - name: cc-get-quota
                      limits:
                        - limit: 10
                          duration: "1h"
              - path: /chat/completions
                methods:
                  - 'POST'
                params:
                  quotas:
                    - name: cc-post-quota
                      limits:
                        - limit: 4
                          duration: "1h"
      """
    Then the response status code should be 201
    And I wait for 2 seconds
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    # POST /chat/completions -> the POST-specific quota (4/hour)
    When I send a POST request to "http://localhost:8080/methodrl/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should be "4"
    And the response header "X-RateLimit-Remaining" should be "3"
    When I send a POST request to "http://localhost:8080/methodrl/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/methodrl/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/methodrl/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    # 5th POST exceeds the POST limit of 4
    When I send a POST request to "http://localhost:8080/methodrl/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 429

    # GET /chat/completions -> the GET-specific quota (10/hour), a SEPARATE bucket from POST.
    # These succeed even though the POST quota is exhausted, proving method-level isolation
    # and that GET is governed by 10 (not 4, 2, or 1).
    When I send a GET request to "http://localhost:8080/methodrl/chat/completions"
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should be "10"
    And the response header "X-RateLimit-Remaining" should be "9"
    When I send a GET request to "http://localhost:8080/methodrl/chat/completions"
    Then the response status code should be 200
    When I send a GET request to "http://localhost:8080/methodrl/chat/completions"
    Then the response status code should be 200
    When I send a GET request to "http://localhost:8080/methodrl/chat/completions"
    Then the response status code should be 200
    When I send a GET request to "http://localhost:8080/methodrl/chat/completions"
    Then the response status code should be 200

    # /chat/<other> -> the /chat/* quota (2/hour)
    When I send a POST request to "http://localhost:8080/methodrl/chat/embeddings" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should be "2"
    When I send a POST request to "http://localhost:8080/methodrl/chat/embeddings" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    # 3rd exceeds the /chat/* limit of 2
    When I send a POST request to "http://localhost:8080/methodrl/chat/embeddings" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 429

    # any other path -> the root /* quota (1/hour)
    When I send a POST request to "http://localhost:8080/methodrl/models" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should be "1"
    # 2nd exceeds the /* limit of 1
    When I send a POST request to "http://localhost:8080/methodrl/models" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "methodrl-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "methodrl-template"
    Then the response status code should be 200

  # ------------------------------------------------------------------
  # Method specificity on the SAME path: a concrete method beats the '*' wildcard method,
  # mirroring how a specific path beats a wildcard path.
  #   - /chat/completions '*'  -> 4  / hour  (all methods)
  #   - /chat/completions GET  -> 10 / hour  (GET wins over '*' for GET requests)
  #   - /chat/*           '*'  -> 2  / hour
  #   - /*                '*'  -> 1  / hour
  # So GET /chat/completions = 10, every other method on /chat/completions = 4.
  # ------------------------------------------------------------------
  Scenario: A concrete method beats the wildcard method on the same path
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: methodspec-template
      spec:
        displayName: Method Spec Template
      """
    Then the response status code should be 201

    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: methodspec-provider
      spec:
        displayName: Method Spec Provider
        version: v1.0
        context: /methodspec
        template: methodspec-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: allow_all
        policies:
          - name: advanced-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods:
                  - '*'
                params:
                  quotas:
                    - name: cc-all-quota
                      limits:
                        - limit: 4
                          duration: "1h"
              - path: /chat/completions
                methods:
                  - 'GET'
                params:
                  quotas:
                    - name: cc-get-quota
                      limits:
                        - limit: 10
                          duration: "1h"
              - path: /*
                methods:
                  - '*'
                params:
                  quotas:
                    - name: root-quota
                      limits:
                        - limit: 1
                          duration: "1h"
              - path: /chat/*
                methods:
                  - '*'
                params:
                  quotas:
                    - name: chatwild-quota
                      limits:
                        - limit: 2
                          duration: "1h"
      """
    Then the response status code should be 201
    And I wait for 2 seconds
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    # GET /chat/completions -> the GET-specific quota (10/hour), winning over the '*' entry (4)
    When I send a GET request to "http://localhost:8080/methodspec/chat/completions"
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should be "10"
    And the response header "X-RateLimit-Remaining" should be "9"
    When I send a GET request to "http://localhost:8080/methodspec/chat/completions"
    Then the response status code should be 200
    When I send a GET request to "http://localhost:8080/methodspec/chat/completions"
    Then the response status code should be 200
    When I send a GET request to "http://localhost:8080/methodspec/chat/completions"
    Then the response status code should be 200
    # 5th GET still succeeds (limit is 10, not 4) - proves GET is NOT governed by the '*' entry
    When I send a GET request to "http://localhost:8080/methodspec/chat/completions"
    Then the response status code should be 200

    # POST /chat/completions -> the '*' entry (4/hour); GET's usage did not touch this bucket
    When I send a POST request to "http://localhost:8080/methodspec/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should be "4"
    And the response header "X-RateLimit-Remaining" should be "3"
    When I send a POST request to "http://localhost:8080/methodspec/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/methodspec/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/methodspec/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    # 5th POST exceeds the '*' entry limit of 4
    When I send a POST request to "http://localhost:8080/methodspec/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 429

    # /chat/<other> -> the /chat/* quota (2/hour)
    When I send a POST request to "http://localhost:8080/methodspec/chat/embeddings" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should be "2"
    When I send a POST request to "http://localhost:8080/methodspec/chat/embeddings" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/methodspec/chat/embeddings" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 429

    # any other path -> the root /* quota (1/hour)
    When I send a POST request to "http://localhost:8080/methodspec/models" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should be "1"
    When I send a POST request to "http://localhost:8080/methodspec/models" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "methodspec-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "methodspec-template"
    Then the response status code should be 200

  # ------------------------------------------------------------------
  # Narrower method set wins on the same path: [POST] is more specific than [GET, POST].
  #   - /chat/completions [GET, POST] -> 3   / hour
  #   - /chat/completions [POST]      -> 100 / hour
  # So GET = 3 (only the [GET, POST] entry covers it), POST = 100 (the narrower [POST] entry).
  # ------------------------------------------------------------------
  Scenario: A narrower method set wins over a broader one on the same path
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: narrowrl-template
      spec:
        displayName: Narrow RL Template
      """
    Then the response status code should be 201

    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: narrowrl-provider
      spec:
        displayName: Narrow RL Provider
        version: v1.0
        context: /narrowrl
        template: narrowrl-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: allow_all
        policies:
          - name: advanced-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods: [GET, POST]
                params:
                  quotas:
                    - name: cc-readwrite-quota
                      limits:
                        - limit: 3
                          duration: "1h"
              - path: /chat/completions
                methods: [POST]
                params:
                  quotas:
                    - name: cc-write-quota
                      limits:
                        - limit: 100
                          duration: "1h"
      """
    Then the response status code should be 201
    And I wait for 2 seconds
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    # GET is covered only by the [GET, POST] entry -> 3/hour
    When I send a GET request to "http://localhost:8080/narrowrl/chat/completions"
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should be "3"
    And the response header "X-RateLimit-Remaining" should be "2"
    When I send a GET request to "http://localhost:8080/narrowrl/chat/completions"
    Then the response status code should be 200
    When I send a GET request to "http://localhost:8080/narrowrl/chat/completions"
    Then the response status code should be 200
    # 4th GET exceeds the [GET, POST] limit of 3
    When I send a GET request to "http://localhost:8080/narrowrl/chat/completions"
    Then the response status code should be 429

    # POST is covered by the narrower [POST] entry -> 100/hour (NOT capped at 3)
    When I send a POST request to "http://localhost:8080/narrowrl/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should be "100"
    And the response header "X-RateLimit-Remaining" should be "99"
    When I send a POST request to "http://localhost:8080/narrowrl/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/narrowrl/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/narrowrl/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    # 5th POST still succeeds (limit is 100, not 3) - proves POST uses the narrower [POST] entry
    When I send a POST request to "http://localhost:8080/narrowrl/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "narrowrl-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "narrowrl-template"
    Then the response status code should be 200

  # ------------------------------------------------------------------
  # The same most-specific-wins logic applies to an LlmProxy's own policies.
  #   proxy policies: /chat/completions -> 4 / hour, /* -> 1 / hour
  # So /chat/completions = 4 (not also limited by the /* entry), other paths = 1.
  # ------------------------------------------------------------------
  Scenario: Most specific wins for an LLM proxy's policies
    # Backing provider (no policies, just forwards to the echo backend)
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: proxyspec-template
      spec:
        displayName: Proxy Spec Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: proxyspec-provider
      spec:
        displayName: Proxy Spec Provider
        version: v1.0
        context: /proxyspec-provider
        template: proxyspec-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201

    # Proxy carrying the overlapping-path advanced-ratelimit policy
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProxy
      metadata:
        name: proxyspec-proxy
      spec:
        displayName: Proxy Spec Proxy
        version: v1.0
        context: /proxyspec
        provider:
          id: proxyspec-provider
        policies:
          - name: advanced-ratelimit
            version: v1
            paths:
              - path: /chat/completions
                methods:
                  - '*'
                params:
                  quotas:
                    - name: proxy-cc-quota
                      limits:
                        - limit: 4
                          duration: "1h"
              - path: /*
                methods:
                  - '*'
                params:
                  quotas:
                    - name: proxy-root-quota
                      limits:
                        - limit: 1
                          duration: "1h"
      """
    Then the response status code should be 201
    And I wait for 2 seconds
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    # POST /chat/completions on the proxy -> the specific quota (4/hour)
    When I send a POST request to "http://localhost:8080/proxyspec/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should be "4"
    When I send a POST request to "http://localhost:8080/proxyspec/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/proxyspec/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/proxyspec/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    # 5th exceeds the specific limit of 4
    When I send a POST request to "http://localhost:8080/proxyspec/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 429

    # any other path on the proxy -> the /* quota (1/hour)
    When I send a POST request to "http://localhost:8080/proxyspec/embeddings" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should be "1"
    When I send a POST request to "http://localhost:8080/proxyspec/embeddings" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/proxyspec-proxy"
    Then the response should be successful
    When I delete the LLM provider "proxyspec-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "proxyspec-template"
    Then the response status code should be 200

  # ------------------------------------------------------------------
  # Two separate policy blocks of the SAME name both apply. Each block is resolved
  # most-specific-within-itself, and every block layers onto the route. set-headers is used
  # (two blocks setting DIFFERENT response headers) so "both applied" is directly assertable:
  #   Block 1 (set-headers, X-Tier):  /chat/completions [*]->chat-all, [GET]->chat-get,
  #                                   /chat/* [*]->chat-wild, /* [*]->root
  #   Block 2 (set-headers, X-Global): /* [*]->global-applied
  # Every response carries the block-1 X-Tier for its most specific path/method AND the
  # block-2 X-Global, proving both same-name blocks apply.
  # ------------------------------------------------------------------
  Scenario: Two policy blocks of the same name both apply, each most-specific within itself
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: twoblocks-template
      spec:
        displayName: Two Blocks Template
      """
    Then the response status code should be 201

    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: twoblocks-provider
      spec:
        displayName: Two Blocks Provider
        version: v1.0
        context: /twoblocks
        template: twoblocks-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: allow_all
        policies:
          - name: set-headers
            version: v1
            paths:
              - path: /chat/completions
                methods:
                  - '*'
                params:
                  response:
                    headers:
                      - name: X-Tier
                        value: chat-all
              - path: /chat/completions
                methods:
                  - 'GET'
                params:
                  response:
                    headers:
                      - name: X-Tier
                        value: chat-get
              - path: /chat/*
                methods:
                  - '*'
                params:
                  response:
                    headers:
                      - name: X-Tier
                        value: chat-wild
              - path: /*
                methods:
                  - '*'
                params:
                  response:
                    headers:
                      - name: X-Tier
                        value: root
          - name: set-headers
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  response:
                    headers:
                      - name: X-Global
                        value: global-applied
      """
    Then the response status code should be 201
    And I wait for 2 seconds
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    # POST /chat/completions -> block 1 most-specific is the [*] entry (chat-all); block 2 also applies
    When I send a POST request to "http://localhost:8080/twoblocks/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    And the response header "X-Tier" should be "chat-all"
    And the response header "X-Global" should be "global-applied"

    # GET /chat/completions -> block 1 most-specific is the [GET] entry (chat-get); block 2 also applies
    When I send a GET request to "http://localhost:8080/twoblocks/chat/completions"
    Then the response status code should be 200
    And the response header "X-Tier" should be "chat-get"
    And the response header "X-Global" should be "global-applied"

    # /chat/<other> -> block 1 /chat/* (chat-wild); block 2 also applies
    When I send a POST request to "http://localhost:8080/twoblocks/chat/embeddings" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    And the response header "X-Tier" should be "chat-wild"
    And the response header "X-Global" should be "global-applied"

    # any other path -> block 1 /* (root); block 2 also applies
    When I send a POST request to "http://localhost:8080/twoblocks/models" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    And the response header "X-Tier" should be "root"
    And the response header "X-Global" should be "global-applied"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "twoblocks-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "twoblocks-template"
    Then the response status code should be 200
