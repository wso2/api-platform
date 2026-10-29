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

@model-failover @routing-layered
Feature: Load spreading and failover on the same operation
  A publisher spreads traffic across several models with a routing policy, then
  adds failover so a request whose target fails still completes instead of
  erroring. The two compose: the routing policy decides where a request goes,
  and failover decides where it goes next.

  The information flows ONE WAY. A routing policy publishes its ordered
  preference list; failover consumes it. Nothing flows back — failover's health
  is private per-instance state and is published to nobody. A routing policy
  therefore keeps nominating a target failover has suspended, and that is
  correct: the target is skipped where the chain is resolved, and a skip costs
  no attempt.

  "x-envoy-attempt-count" is the observable proof of how many upstream attempts
  a request took. One means the leading target answered; two means it failed and
  the next one in the chain did.

  A dead target is produced by pointing a provider's upstream at a host that
  does not resolve. That is a transport failure — deterministic, and independent
  of the request — unlike a forced status code, which would apply identically to
  every attempt and so could never demonstrate a rescue.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ====================================================================
  # SCENARIO 1 + 2: failover walks the routing policy's published order,
  # and the routing policy is consulted once per client request
  # ====================================================================

  Scenario: Failover continues through the routing policy's published order
    # Two live providers and one dead one, so the rotation visits a failing
    # target on a known turn and a healthy one on the others.
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: rl-dead-provider
      spec:
        displayName: RL Dead Provider
        version: v1.0
        template: openai
        context: /rl-dead-ctx
        upstream:
          url: http://rl-nonexistent-host:9080/dead-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: rl-alive-provider
      spec:
        displayName: RL Alive Provider
        version: v1.0
        template: openai
        context: /rl-alive-ctx
        upstream:
          url: http://sample-backend:9080/alive-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201

    # The rotation alternates between a target on the DEAD primary and one on the
    # live provider. model-failover is attached with no fallbacks of its own, so
    # the only depth available is the ranking the routing policy published —
    # which is precisely what this scenario is here to prove is consumed.
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: rl-order-proxy
      spec:
        displayName: RL Order Proxy
        version: v1.0
        context: /rl-order-proxy
        provider:
          id: rl-dead-provider
        additionalProviders:
          - id: rl-alive-provider
            as: alive-provider
        operationPolicies:
          - name: model-round-robin
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  requestModel:
                    location: payload
                    identifier: $.model
                  suspendDuration: 0
                  models:
                    - model: gpt-4o
                    - model: claude-sonnet-4-5-20250929
                      provider: alive-provider
          - name: model-failover
            version: v0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  suspendDuration: 0
                  targets:
                    - model: gpt-4o
      """
    Then the response status should be 201
    And I wait for 5 seconds

    # Turn 1 — the rotation leads with gpt-4o on the dead primary. The published
    # ranking is [gpt-4o, claude@alive], so failover continues to the second
    # member and the client gets a 200 it never retried for.
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/rl-order-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/alive-upstream"
    And the response header "x-envoy-attempt-count" should be "2"

    # Turn 2 — the cursor has advanced, so the rotation now leads with the live
    # target and nothing fails. One attempt, and it proves the ranking really
    # rotates rather than being a fixed list that happened to match.
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/rl-order-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/alive-upstream"
    And the response header "x-envoy-attempt-count" should be "1"

    # Turn 3 — back to the dead target, rescued again. The alternation is the
    # evidence that the routing policy advanced exactly once per CLIENT request
    # and not once per attempt (acceptance scenario 2): had it been re-invoked
    # for the retry in turn 1, the cursor would be off by one by now and this
    # turn would have taken a single attempt.
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/rl-order-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/alive-upstream"
    And the response header "x-envoy-attempt-count" should be "2"

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/rl-order-proxy"
    Then the response should be successful
    When I delete the LLM provider "rl-dead-provider"
    Then the response status code should be 200
    When I delete the LLM provider "rl-alive-provider"
    Then the response status code should be 200

  # ====================================================================
  # SCENARIO 3: a suspended target stops being attempted, even though the
  # routing policy goes on nominating it
  # ====================================================================

  Scenario: A suspended target is skipped without costing an attempt
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: rl-susp-dead-provider
      spec:
        displayName: RL Suspend Dead Provider
        version: v1.0
        template: openai
        context: /rl-susp-dead-ctx
        upstream:
          url: http://rl-suspend-nonexistent:9080/dead-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: rl-susp-alive-provider
      spec:
        displayName: RL Suspend Alive Provider
        version: v1.0
        template: openai
        context: /rl-susp-alive-ctx
        upstream:
          url: http://sample-backend:9080/alive-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201

    # One failure suspends, for long enough that the window cannot expire
    # mid-scenario. The routing policy's own suspendDuration stays 0 so that
    # what is observed here is failover's verdict and not its own.
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: rl-suspend-proxy
      spec:
        displayName: RL Suspend Proxy
        version: v1.0
        context: /rl-suspend-proxy
        provider:
          id: rl-susp-dead-provider
        additionalProviders:
          - id: rl-susp-alive-provider
            as: alive-provider
        operationPolicies:
          - name: model-round-robin
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  requestModel:
                    location: payload
                    identifier: $.model
                  suspendDuration: 0
                  models:
                    - model: gpt-4o
                    - model: claude-sonnet-4-5-20250929
                      provider: alive-provider
          - name: model-failover
            version: v0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  suspendDuration: 300
                  consecutiveFailures: 1
                  targets:
                    - model: gpt-4o
      """
    Then the response status should be 201
    And I wait for 5 seconds

    # First request: the rotation leads with the dead target, it fails, the
    # request is rescued — and that one failure suspends it.
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/rl-suspend-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"

    # Second request: the rotation leads with the LIVE target. One attempt.
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/rl-suspend-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "1"

    # Third request: the rotation comes back round and nominates the dead target
    # AGAIN — the routing policy never learned it was suspended, and is not
    # supposed to. Failover skips it when resolving the chain, so the request
    # leads with the live target instead and takes ONE attempt, not two.
    #
    # This single assertion is what separates the two designs. A count of 2
    # would mean the suspension was ignored and the dead target was attempted;
    # a count of 1 means it was skipped, and that a skip is not an attempt.
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/rl-suspend-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/alive-upstream"
    And the response header "x-envoy-attempt-count" should be "1"

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/rl-suspend-proxy"
    Then the response should be successful
    When I delete the LLM provider "rl-susp-dead-provider"
    Then the response status code should be 200
    When I delete the LLM provider "rl-susp-alive-provider"
    Then the response status code should be 200

  # ====================================================================
  # SCENARIO 6: a routing policy that has NOT adopted the contract still
  # composes, through the one-element-list path
  # ====================================================================

  Scenario: A non-adopting routing policy composes via its single selection
    # model-weighted-round-robin publishes only the single-selection keys — it
    # has not adopted the preference-list contract and is not required to
    # (FR-023). Failover must read that one selection as a one-element ranking
    # and continue into its own configured chain from there.
    #
    # This is the compatibility guarantee the whole contract rests on: no
    # routing policy has to change for failover to work.
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: rl-na-dead-provider
      spec:
        displayName: RL Non-Adopting Dead Provider
        version: v1.0
        template: openai
        context: /rl-na-dead-ctx
        upstream:
          url: http://rl-nonadopting-nonexistent:9080/dead-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: rl-na-alive-provider
      spec:
        displayName: RL Non-Adopting Alive Provider
        version: v1.0
        template: openai
        context: /rl-na-alive-ctx
        upstream:
          url: http://sample-backend:9080/alive-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201

    # Every weight is on the dead target, so the single selection is always
    # gpt-4o and the rescue can only have come from failover's OWN configured
    # fallback — not from anything the routing policy published.
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: rl-nonadopting-proxy
      spec:
        displayName: RL Non-Adopting Proxy
        version: v1.0
        context: /rl-nonadopting-proxy
        provider:
          id: rl-na-dead-provider
        additionalProviders:
          - id: rl-na-alive-provider
            as: alive-provider
        operationPolicies:
          - name: model-weighted-round-robin
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  requestModel:
                    location: payload
                    identifier: $.model
                  suspendDuration: 0
                  models:
                    - model: gpt-4o
                      weight: 1
          - name: model-failover
            version: v0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  suspendDuration: 0
                  targets:
                    - model: gpt-4o
                      fallbacks:
                        - model: claude-sonnet-4-5-20250929
                          provider: alive-provider
      """
    Then the response status should be 201
    And I wait for 5 seconds

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/rl-nonadopting-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/alive-upstream"
    And the response header "x-envoy-attempt-count" should be "2"

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/rl-nonadopting-proxy"
    Then the response should be successful
    When I delete the LLM provider "rl-na-dead-provider"
    Then the response status code should be 200
    When I delete the LLM provider "rl-na-alive-provider"
    Then the response status code should be 200

  # ====================================================================
  # SCENARIO 7 / FR-026c / SC-012: a routing policy deployed WITHOUT
  # failover is untouched, including its own suspendDuration
  # ====================================================================

  Scenario: A routing policy without failover keeps its own suspension
    # Adding the preference-list publication to model-round-robin must not
    # change a single thing for a deployment that does not use failover. Its own
    # suspendDuration still governs, and nothing supersedes it — failover is not
    # attached here, and even when it is, neither policy overrides the other.
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: rl-solo-dead-provider
      spec:
        displayName: RL Solo Dead Provider
        version: v1.0
        template: openai
        context: /rl-solo-dead-ctx
        upstream:
          url: http://rl-solo-nonexistent:9080/dead-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: rl-solo-alive-provider
      spec:
        displayName: RL Solo Alive Provider
        version: v1.0
        template: openai
        context: /rl-solo-alive-ctx
        upstream:
          url: http://sample-backend:9080/alive-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201

    # NO model-failover on this operation. One failure suspends the dead target
    # under the routing policy's OWN setting, which is the behaviour that
    # existed before this feature and must still exist after it.
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: rl-solo-proxy
      spec:
        displayName: RL Solo Proxy
        version: v1.0
        context: /rl-solo-proxy
        provider:
          id: rl-solo-dead-provider
        additionalProviders:
          - id: rl-solo-alive-provider
            as: alive-provider
        operationPolicies:
          - name: model-round-robin
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  requestModel:
                    location: payload
                    identifier: $.model
                  suspendDuration: 300
                  models:
                    - model: gpt-4o
                    - model: claude-sonnet-4-5-20250929
                      provider: alive-provider
      """
    Then the response status should be 201
    And I wait for 5 seconds

    # First request goes to the dead target and FAILS. With no failover
    # attached there is no rescue — exactly as before this feature existed.
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/rl-solo-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 503
    And the response header "x-envoy-attempt-count" should be "1"

    # That failure suspended the dead target under the ROUTING POLICY's own
    # suspendDuration. Both following requests therefore land on the live
    # target: the suspended one is no longer selected, even on the turn the
    # rotation would otherwise have given it.
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/rl-solo-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/alive-upstream"

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/rl-solo-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/alive-upstream"

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/rl-solo-proxy"
    Then the response should be successful
    When I delete the LLM provider "rl-solo-dead-provider"
    Then the response status code should be 200
    When I delete the LLM provider "rl-solo-alive-provider"
    Then the response status code should be 200
