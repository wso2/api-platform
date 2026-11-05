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

    # Both targets were attempted and the chain is spent, so FR-014's single
    # sterile error is what the client receives -- gateway-invented, deliberately,
    # because the alternative is forwarding a body this gateway never wrote.
    # Implemented by T039; this scenario previously asserted the pass-through
    # status and carried a note to update it here when that landed.
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/fo-exhausted-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 503
    And the response body should contain "failover_exhausted"

    # x-envoy-attempt-count is deliberately NOT asserted here, and its absence is
    # the point rather than an oversight: the exhaustion response REPLACES the
    # upstream's, headers included, so Envoy's own header does not survive it. How
    # many attempts were made is still recorded -- in the telemetry record and the
    # gateway log -- it simply stops being something the client is told.

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/fo-exhausted-proxy"
    Then the response should be successful
    When I delete the LLM provider "fo-dead-one-provider"
    Then the response status code should be 200
    When I delete the LLM provider "fo-dead-two-provider"
    Then the response status code should be 200

  # ====================================================================
  # A SURPLUS ATTEMPT BUDGET PRODUCES NO SURPLUS ATTEMPTS
  # ====================================================================
  #
  # The attempt budget and the published chain are sized from DIFFERENT things.
  # num_retries comes from every target the operation can resolve, counted at
  # deploy time; the chain published per request covers only the target the
  # request actually resolved to, plus its own configured fallbacks. A proxy
  # carrying two independent chains therefore has a budget larger than the chain
  # serving any single request, and what happens to the difference is the
  # property that decided the whole mechanism.
  #
  # Of the two proxy mechanisms that could carry a per-request ordering, one
  # spends a surplus attempt on a target the policy had EXCLUDED, with correct
  # retry arithmetic the only thing preventing it. We took the other and accepted
  # an Envoy upgrade to get it, because suspension exists precisely to keep
  # traffic off a target failover ruled out. Nothing in the suite asserted that
  # until this scenario.
  #
  # Measured here (2026-11-05, against a budget of four and a published chain of
  # two): the surplus attempts are NOT MADE. Each published group is dialled
  # exactly once and the request then ends. That is stronger than the behaviour
  # this scenario was first written to expect -- a spike had shown the surplus
  # repeating the last group, so the scenario originally asserted a repeat and
  # failed, correctly, against a gateway that makes no surplus attempt at all.
  # The chain, not the budget, bounds the attempt sequence.
  #
  # The spare chain's provider is HEALTHY and deliberately so: it is the trap. If
  # an attempt ever reached outside the published groups it would land on a target
  # that answers 200, and this scenario would see a success where it demands an
  # exhaustion. A dead spare would make the test pass for the wrong reason.

  Scenario: A chain shorter than the attempt budget ends at its last target, reaching no other
    # --- Chain 1, both members dead: this is the chain the request will use ---
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: fo-ovr-dead-primary
      spec:
        displayName: FO Overrun Dead Primary
        version: v1.0
        template: openai
        context: /fo-ovr-dead-primary-ctx
        upstream:
          url: http://fo-overrun-nonexistent-a:9080/overrun-dead-a
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: fo-ovr-dead-fallback
      spec:
        displayName: FO Overrun Dead Fallback
        version: v1.0
        template: openai
        context: /fo-ovr-dead-fallback-ctx
        upstream:
          url: http://fo-overrun-nonexistent-b:9080/overrun-dead-b
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    # --- Chain 2, healthy: never selected by this request, only widens the budget ---
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: fo-ovr-spare
      spec:
        displayName: FO Overrun Spare
        version: v1.0
        template: openai
        context: /fo-ovr-spare-ctx
        upstream:
          url: http://sample-backend:9080/overrun-spare-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    # Four resolvable (model, provider) members across two chains, so the attempt
    # budget is four. A request for gpt-4o publishes a chain of TWO -- its own
    # target and the one fallback written for it. A chain written for one target
    # is never borrowed for another, so the spare chain contributes attempts but
    # no groups, which is exactly the surplus this scenario is about.
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: fo-overrun-proxy
      spec:
        displayName: FO Overrun Proxy
        version: v1.0
        context: /fo-overrun-proxy
        provider:
          id: fo-ovr-dead-primary
        additionalProviders:
          - id: fo-ovr-dead-fallback
            as: dead-fallback
          - id: fo-ovr-spare
            as: spare-provider
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
                          provider: dead-fallback
                    - model: gpt-4o-mini
                      provider: spare-provider
                      fallbacks:
                        - model: claude-haiku-4-5-20251001
                          provider: spare-provider
      """
    Then the response status should be 201
    And I wait for 3 seconds

    # Noted AFTER deployment settles, so configuration logging naming the spare
    # upstream cannot satisfy -- or falsify -- an assertion about where this one
    # request was dialled.
    When I note the current gateway runtime log position
    And I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/fo-overrun-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    # Exhaustion, not a 200. A surplus attempt that escaped the published groups
    # would have found the healthy spare and answered successfully, so the status
    # alone carries most of this scenario's weight.
    Then the response status code should be 503
    And the response body should contain "failover_exhausted"
    # The direct form of the same claim: the spare's upstream was never dialled.
    And the gateway runtime log since then should not contain "/overrun-spare-upstream"
    # The attempt sequence, pinned exactly: each published group dialled ONCE,
    # and nothing more, against a budget that allowed four attempts.
    #
    # The search string is the router's own access-log line rather than the bare
    # upstream path, and that is load-bearing. A single attempt writes the path
    # TWICE -- once in the policy engine's structured log entry, once in the
    # router's access line -- so counting "/overrun-dead-a" reports 2 for one
    # attempt. An earlier version of this scenario asserted "at least 2 times"
    # against that string and passed while observing nothing at all. Matching on
    # "POST <context> HTTP/1.1" appears once per upstream attempt and nowhere
    # else, verified directly against the container log.
    And the gateway runtime log since then should contain "POST /fo-ovr-dead-primary-ctx/chat/completions HTTP/1.1" exactly 1 times
    And the gateway runtime log since then should contain "POST /fo-ovr-dead-fallback-ctx/chat/completions HTTP/1.1" exactly 1 times

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/fo-overrun-proxy"
    Then the response should be successful
    When I delete the LLM provider "fo-ovr-dead-primary"
    Then the response status code should be 200
    When I delete the LLM provider "fo-ovr-dead-fallback"
    Then the response status code should be 200
    When I delete the LLM provider "fo-ovr-spare"
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
    # One attempt: the policy resolved nothing, so nothing failed over.
    #
    # Asserted as a COUNT rather than as the header's absence, which is not a
    # property this operation controls. include_attempt_count_in_response is set
    # per VIRTUAL HOST, so the header appears on every response in the vhost as
    # soon as ANY route in it fails over -- demanding it be missing asserts which
    # other APIs happen to be deployed, not anything about this one. It passed
    # while this file ran alone and failed the moment the failover features ran
    # together, with no defect in either. The vhost scoping itself is a known
    # limitation against FR-030d/SC-012a, recorded in poc-results.md.
    And the response should report one upstream attempt

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/fo-inert-proxy"
    Then the response should be successful
    When I delete the LLM provider "fo-inert-provider"
    Then the response status code should be 200
