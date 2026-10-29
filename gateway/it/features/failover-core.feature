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

@model-failover
Feature: Model Failover Core Behaviour
  When an AI model target fails, the gateway re-attempts the SAME client request
  against the next target in the chain, so the consuming application receives a
  successful response instead of an upstream failure and never writes retry logic
  of its own.

  The re-attempt happens inside the proxy: the downstream filter chain runs once,
  and only the upstream filters run again per attempt. "x-envoy-attempt-count" on
  the response is the observable proof of how many upstream attempts the request
  actually took — one means no failover occurred, two means the first target
  failed and the second answered.

  A dead target is produced by pointing a provider's upstream at a host that does
  not resolve. That is a transport failure, which is deterministic and independent
  of the request — unlike a forced status code, which would apply identically to
  every attempt and could never demonstrate a rescue.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ====================================================================
  # A FAILING TARGET IS RESCUED BY THE NEXT ONE IN THE CHAIN
  # ====================================================================

  Scenario: A request to a failing target succeeds on the next target
    # --- Primary provider: upstream host does not resolve, so every attempt fails ---
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: fo-dead-provider
      spec:
        displayName: FO Dead Provider
        version: v1.0
        template: openai
        context: /fo-dead-ctx
        upstream:
          url: http://fo-nonexistent-host:9080/dead-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    # --- Fallback provider: healthy, echoes its own upstream path back ---
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: fo-alive-provider
      spec:
        displayName: FO Alive Provider
        version: v1.0
        template: openai
        context: /fo-alive-ctx
        upstream:
          url: http://sample-backend:9080/alive-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: fo-rescue-proxy
      spec:
        displayName: FO Rescue Proxy
        version: v1.0
        context: /fo-rescue-proxy
        provider:
          id: fo-dead-provider
        additionalProviders:
          - id: fo-alive-provider
            as: alive-provider
        policies:
          - name: model-failover
            version: v0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  targets:
                    - model: gpt-4o
                      fallbacks:
                        - model: claude-sonnet-4-5-20250929
                          provider: alive-provider
      """
    Then the response status should be 201
    And I wait for 3 seconds

    # The client sees a 200 it never had to retry for, served by the SECOND target.
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/fo-rescue-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/alive-upstream"
    And the response header "x-envoy-attempt-count" should be "2"

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/fo-rescue-proxy"
    Then the response should be successful
    When I delete the LLM provider "fo-dead-provider"
    Then the response status code should be 200
    When I delete the LLM provider "fo-alive-provider"
    Then the response status code should be 200

  # ====================================================================
  # A HEALTHY FIRST TARGET IS NEVER RE-ATTEMPTED
  # ====================================================================

  Scenario: A successful first attempt makes no further attempts
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: fo-first-provider
      spec:
        displayName: FO First Provider
        version: v1.0
        template: openai
        context: /fo-first-ctx
        upstream:
          url: http://sample-backend:9080/first-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: fo-second-provider
      spec:
        displayName: FO Second Provider
        version: v1.0
        template: openai
        context: /fo-second-ctx
        upstream:
          url: http://sample-backend:9080/second-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: fo-healthy-proxy
      spec:
        displayName: FO Healthy Proxy
        version: v1.0
        context: /fo-healthy-proxy
        provider:
          id: fo-first-provider
        additionalProviders:
          - id: fo-second-provider
            as: second-provider
        policies:
          - name: model-failover
            version: v0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  targets:
                    - model: gpt-4o
                      fallbacks:
                        - model: claude-sonnet-4-5-20250929
                          provider: second-provider
      """
    Then the response status should be 201
    And I wait for 3 seconds

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/fo-healthy-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/first-upstream"
    And the response body should not contain "/second-upstream"
    And the response header "x-envoy-attempt-count" should be "1"

    # A status outside the failure conditions is the client's own problem: it must be
    # returned as-is, on the first attempt, and never re-sent to another provider --
    # re-attempting it would bill a second inference call to mask a client bug.
    #
    # Not asserted here yet. The IT sample backend (ghcr.io/wso2/api-platform/sample-service)
    # is a pure echo: it reports "?statusCode=400" back in its "query" field and still
    # answers 200, so there is currently no way to make an upstream return a chosen
    # status. Other features in this suite assume otherwise. Covering this needs a
    # backend that can be told to fail; until then the unit tests in
    # gc:policies/model-failover/classify.go carry the rule.

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/fo-healthy-proxy"
    Then the response should be successful
    When I delete the LLM provider "fo-first-provider"
    Then the response status code should be 200
    When I delete the LLM provider "fo-second-provider"
    Then the response status code should be 200

  # ====================================================================
  # AN EXHAUSTED CHAIN RETURNS THE UPSTREAM'S OWN FAILURE
  # ====================================================================

  Scenario: Every target failing returns the last failure, not a gateway-invented one
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: fo-dead-one-provider
      spec:
        displayName: FO Dead One Provider
        version: v1.0
        template: openai
        context: /fo-dead-one-ctx
        upstream:
          url: http://fo-nonexistent-one:9080/dead-one
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: fo-dead-two-provider
      spec:
        displayName: FO Dead Two Provider
        version: v1.0
        template: openai
        context: /fo-dead-two-ctx
        upstream:
          url: http://fo-nonexistent-two:9080/dead-two
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: fo-exhausted-proxy
      spec:
        displayName: FO Exhausted Proxy
        version: v1.0
        context: /fo-exhausted-proxy
        provider:
          id: fo-dead-one-provider
        additionalProviders:
          - id: fo-dead-two-provider
            as: dead-two-provider
        policies:
          - name: model-failover
            version: v0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  targets:
                    - model: gpt-4o
                      fallbacks:
                        - model: claude-sonnet-4-5-20250929
                          provider: dead-two-provider
      """
    Then the response status should be 201
    And I wait for 3 seconds

    # Both targets were attempted. The status asserted here is the one Envoy
    # produces today when the last attempt fails.
    #
    # FR-014 calls for a single sterile failover-exhausted error instead, carrying
    # no upstream body, credential, or internal address. That is task T039 and is
    # NOT implemented yet -- when it lands it changes what the client sees here,
    # so this scenario must be updated in the same change rather than left
    # asserting the pass-through status.
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/fo-exhausted-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 503
    And the response header "x-envoy-attempt-count" should be "2"

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/fo-exhausted-proxy"
    Then the response should be successful
    When I delete the LLM provider "fo-dead-one-provider"
    Then the response status code should be 200
    When I delete the LLM provider "fo-dead-two-provider"
    Then the response status code should be 200

  # ====================================================================
  # A POLICY THAT RESOLVES TO NOTHING IS A SILENT NO-OP
  # ====================================================================

  Scenario: Targets naming providers that are not deployed here change nothing
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: fo-inert-provider
      spec:
        displayName: FO Inert Provider
        version: v1.0
        template: openai
        context: /fo-inert-ctx
        upstream:
          url: http://sample-backend:9080/inert-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    # Every target names a provider this environment does not have. The deployment
    # must still be accepted and the operation must behave exactly as if the policy
    # were not attached -- failover must never be the policy that blocks a GitOps
    # rollout over an environment mismatch.
    # The attempt-count flag is per virtual host, so the preceding scenario's failover
    # route must be fully withdrawn via xDS before this one deploys. Three seconds
    # proved too short in practice and this scenario failed with a stale flag.
    And I wait for 8 seconds
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: fo-inert-proxy
      spec:
        displayName: FO Inert Proxy
        version: v1.0
        context: /fo-inert-proxy
        provider:
          id: fo-inert-provider
        policies:
          - name: model-failover
            version: v0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  targets:
                    - model: gpt-4o
                      provider: provider-that-does-not-exist
                      fallbacks:
                        - model: claude-sonnet-4-5-20250929
                          provider: another-missing-provider
      """
    Then the response status should be 201
    And I wait for 3 seconds

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/fo-inert-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/inert-upstream"
    # No route on this virtual host fails over, so the attempt-count header is not
    # generated at all -- an inert policy is indistinguishable from an absent one.
    #
    # The flag is per VIRTUAL HOST, not per route: it is on as soon as ANY route on
    # the vhost fails over. The earlier scenarios therefore delete their proxies in
    # their own cleanup steps, and this scenario must stay last in the file. If it
    # ever fails with the header present, suspect a failover-bearing API that was
    # left deployed rather than a defect in this policy.
    And the response header "x-envoy-attempt-count" should not exist

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/fo-inert-proxy"
    Then the response should be successful
    When I delete the LLM provider "fo-inert-provider"
    Then the response status code should be 200
