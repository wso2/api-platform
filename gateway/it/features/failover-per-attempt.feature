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
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# --------------------------------------------------------------------

@model-failover @per-attempt
Feature: Per-attempt policies run for the provider an attempt actually reaches
  Policies attached to a provider run once per upstream attempt, and only on
  attempts routed to that provider — including a failover retry that lands there.

  This is the half that operation policies cannot do. The gateway runs the
  operation chain once per client request and never re-enters it on a retry, so
  anything that must differ between attempts — the provider's credential, its
  request shaping — has nowhere else to live.

  Each provider stamps a different header, and the sample backend echoes what it
  received. So the echoed header names which provider's chain ran for the attempt
  that actually answered. If chain selection were per request rather than per
  attempt, the rescued request would carry the FIRST provider's header.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: The rescuing attempt runs the rescuing provider's own policies
    # Primary: upstream host does not resolve, so every attempt to it fails.
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: pa-dead-provider
      spec:
        displayName: PA Dead Provider
        version: v1.0
        template: openai
        context: /pa-dead-ctx
        upstream:
          url: http://pa-nonexistent-host:9080/dead-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: pa-alive-provider
      spec:
        displayName: PA Alive Provider
        version: v1.0
        template: openai
        context: /pa-alive-ctx
        upstream:
          url: http://sample-backend:9080/alive-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    # Each provider carries its OWN per-attempt policy, stamping a distinct header.
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: pa-proxy
      spec:
        displayName: PA Proxy
        version: v1.0
        context: /pa-proxy
        provider:
          id: pa-dead-provider
        additionalProviders:
          - id: pa-alive-provider
            as: alive-provider
            policies:
              - name: set-headers
                version: v1
                params:
                  request:
                    headers:
                      - name: X-Attempt-Provider
                        value: alive
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

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/pa-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    # The decisive assertion: the second attempt ran the SECOND provider's chain.
    # A per-request chain would have stamped nothing here, since the primary
    # provider carries no policies at all.
    And the response body should contain "X-Attempt-Provider"
    And the response body should contain "alive"
    And the response body should contain "/alive-upstream"

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/pa-proxy"
    Then the response should be successful
    When I delete the LLM provider "pa-dead-provider"
    Then the response status code should be 200
    When I delete the LLM provider "pa-alive-provider"
    Then the response status code should be 200
