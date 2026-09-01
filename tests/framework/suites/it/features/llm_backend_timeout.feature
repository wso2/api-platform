# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License. You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------
#
# Migrated from gateway/it/features/llm-backend-timeout.feature.
#
# Mechanical differences from the legacy suite:
#   - Upstream http://echo-backend:80 becomes http://testbench:3000. The slow backend is the
#     testbench reflector's GET /delay/{n}, which sleeps n seconds then returns 200 — the same
#     contract the timeout idiom (backend_timeout.feature) already relies on. A route timeout
#     shorter than the delay makes the gateway return 504. (An unreachable address such as
#     192.0.2.1 is deliberately NOT used: that is a CONNECT timeout / 503, a different code path
#     and status from the route/response timeout / 504 these scenarios assert.)
#   - Absolute data-plane URLs (http://localhost:8080/...) reduced to relative paths and
#     readiness waits.
#   - The one authenticate step lives in the Background; the per-scenario/pre-delete duplicates
#     are gone. Each scenario deletes its own provider/proxy.
#
# FINDING 2 (docs/FINDINGS.md): the gateway gives up ~1-2% early on both connect and route
# timeouts (measured 1.97s against a configured 2000ms). The timeout steps carry a 5%
# proportional tolerance, which absorbs it. The configured bounds are kept exactly as the legacy
# feature wrote them; @finding-2-early-timeout marks the scenarios that sit near that boundary so
# they can be found if the gap widens.

@llm-backend-timeout
Feature: Backend route timeouts for LLM providers and proxies via the resilience block
  As an API developer
  I want to configure the route timeout via an API-level resilience block on LLM providers and proxies
  So that requests to slow LLM backends are terminated by the gateway within the configured time

  # LLM kinds support resilience at the API level only

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # allow_all creates a catch-all route for all traffic; the API-level resilience.timeout (2s) is
  # shorter than the backend delay (5s), so the gateway times the route out with 504 at ~2s.
  @finding-2-early-timeout
  Scenario: LLM provider API-level resilience timeout terminates a slow backend
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: llm-timeout-provider
      spec:
        displayName: LLM Timeout Provider
        version: v1.0
        template: openai
        context: /llm-timeout
        upstream:
          url: http://testbench:3000
        accessControl:
          mode: allow_all
        resilience:
          timeout: 2s
      """
    Then the response status code should be 201
    And I send a "GET" request to "/llm-timeout/get" until status 200
    When I send a "GET" request to "/llm-timeout/delay/5"
    Then the gateway should have timed out after "2" seconds with status 504
    When I delete the LLM provider "llm-timeout-provider"
    Then the response status code should be 200

  # deny_all creates routes only for the allow-listed exception paths; the resilience block must be
  # attached to those (forwarding) routes too. The allow-listed /delay/5 route times out at ~2s.
  @finding-2-early-timeout
  Scenario: LLM provider deny_all attaches the timeout to allow-listed routes
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: llm-timeout-deny-provider
      spec:
        displayName: LLM Timeout Deny Provider
        version: v1.0
        template: openai
        context: /llm-timeout-deny
        upstream:
          url: http://testbench:3000
        accessControl:
          mode: deny_all
          exceptions:
            - path: /get
              methods: [GET]
            - path: /delay/5
              methods: [GET]
        resilience:
          timeout: 2s
      """
    Then the response status code should be 201
    And I send a "GET" request to "/llm-timeout-deny/get" until status 200
    When I send a "GET" request to "/llm-timeout-deny/delay/5"
    Then the gateway should have timed out after "2" seconds with status 504
    When I delete the LLM provider "llm-timeout-deny-provider"
    Then the response status code should be 200

  # Without a resilience block the global route timeout default (60s) applies, so a backend that
  # responds within it succeeds normally.
  Scenario: LLM provider without a resilience block falls back to the global default and succeeds
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: llm-timeout-default-provider
      spec:
        displayName: LLM Timeout Default Provider
        version: v1.0
        template: openai
        context: /llm-timeout-default
        upstream:
          url: http://testbench:3000
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I send a "GET" request to "/llm-timeout-default/delay/2" until status 200
    Then the response status code should be 200
    When I delete the LLM provider "llm-timeout-default-provider"
    Then the response status code should be 200

  # A proxy is always allow-all (catch-all). The proxy's own API-level resilience.timeout (2s)
  # bounds the whole proxied call (client -> proxy route -> loopback -> provider route -> backend).
  # The backing provider is left on the global default, so the 504 is attributable to the proxy.
  @finding-2-early-timeout
  Scenario: LLM proxy API-level resilience timeout terminates a slow backend
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: llm-timeout-proxy-provider
      spec:
        displayName: LLM Timeout Proxy Provider
        version: v1.0
        template: openai
        context: /llm-timeout-backing
        upstream:
          url: http://testbench:3000
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create LLM proxy with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: llm-timeout-proxy
      spec:
        displayName: LLM Timeout Proxy
        version: v1.0
        context: /llm-timeout-proxy
        provider:
          id: llm-timeout-proxy-provider
        resilience:
          timeout: 2s
      """
    Then the response status code should be 201
    And I send a "GET" request to "/llm-timeout-proxy/get" until status 200
    When I send a "GET" request to "/llm-timeout-proxy/delay/5"
    Then the gateway should have timed out after "2" seconds with status 504
    When I send a "DELETE" request to the "gateway-controller" service at "/llm-proxies/llm-timeout-proxy"
    Then the response status code should be 200
    When I delete the LLM provider "llm-timeout-proxy-provider"
    Then the response status code should be 200

  # Both the proxy and its backing provider set their own resilience.timeout. The request traverses
  # both routes (proxy 6s -> loopback -> provider 2s -> backend /delay/10), so the shorter inner
  # provider timeout (2s) must fire first and bound the whole call. The upper-bound assertion proves
  # the 2s provider timeout won, not the 6s proxy timeout.
  @finding-2-early-timeout
  Scenario: LLM proxy and provider each set resilience and the shorter (provider) timeout wins
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: llm-timeout-both-provider
      spec:
        displayName: LLM Timeout Both Provider
        version: v1.0
        template: openai
        context: /llm-timeout-both-backing
        upstream:
          url: http://testbench:3000
        accessControl:
          mode: allow_all
        resilience:
          timeout: 2s
      """
    Then the response status code should be 201
    When I create LLM proxy with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: llm-timeout-both-proxy
      spec:
        displayName: LLM Timeout Both Proxy
        version: v1.0
        context: /llm-timeout-both
        provider:
          id: llm-timeout-both-provider
        resilience:
          timeout: 6s
      """
    Then the response status code should be 201
    And I send a "GET" request to "/llm-timeout-both/get" until status 200
    When I send a "GET" request to "/llm-timeout-both/delay/10"
    Then the gateway should have timed out after "2" seconds with status 504
    And the gateway should have responded within "4" seconds
    When I send a "DELETE" request to the "gateway-controller" service at "/llm-proxies/llm-timeout-both-proxy"
    Then the response status code should be 200
    When I delete the LLM provider "llm-timeout-both-provider"
    Then the response status code should be 200
