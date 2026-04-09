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

@consumer-request-based-ratelimit
Feature: Consumer Request-Based Rate Limiting
  As an API developer
  I want request count limits to be enforced independently per GenAI application
  So that one application exhausting its request quota does not block other applications

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Each consumer gets an independent request counter
    # Each app gets 2 requests/hour independently.
    # App A sends 2 requests (limit reached) and gets blocked on the 3rd.
    # App B is unaffected — its counter is still at 0.
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: crbrl-template
      spec:
        displayName: CRBRL Template
      """
    Then the response status code should be 201

    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: crbrl-provider
      spec:
        displayName: CRBRL Provider
        version: v1.0
        context: /crbrl
        template: crbrl-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: test-key
        accessControl:
          mode: allow_all
        policies:
          - name: api-key-auth
            version: v1
            paths:
              - path: /*
                methods: ['*']
                params:
                  key: x-api-key
                  in: header
          - name: advanced-ratelimit
            version: v1
            paths:
              - path: /*
                methods: ['*']
                params:
                  quotas:
                    - name: consumer-request-limit
                      limits:
                        - limit: 2
                          duration: "1h"
                      keyExtraction:
                        - type: routename
                        - type: metadata
                          key: x-wso2-application-id
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    # Create API key for App A
    When I send a POST request to the "gateway-controller" service at "/llm-providers/crbrl-provider/api-keys" with body:
      """
      {
        "name": "crbrl-app-a",
        "apiKey": "crbrl-app-a-key-000000000000000000000000"
      }
      """
    Then the response status code should be 201

    # Create API key for App B
    When I send a POST request to the "gateway-controller" service at "/llm-providers/crbrl-provider/api-keys" with body:
      """
      {
        "name": "crbrl-app-b",
        "apiKey": "crbrl-app-b-key-000000000000000000000000"
      }
      """
    Then the response status code should be 201
    And I wait for 2 seconds

    Given I set header "Content-Type" to "application/json"

    # App A: request 1 — allowed (counter: 1/2)
    When I send a POST request to "http://localhost:8080/crbrl/chat/completions" with header "x-api-key" value "crbrl-app-a-key-000000000000000000000000" with body:
      """
      {"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # App A: request 2 — allowed (counter: 2/2, limit reached)
    When I send a POST request to "http://localhost:8080/crbrl/chat/completions" with header "x-api-key" value "crbrl-app-a-key-000000000000000000000000" with body:
      """
      {"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # App A: request 3 — blocked, request quota exhausted
    When I send a POST request to "http://localhost:8080/crbrl/chat/completions" with header "x-api-key" value "crbrl-app-a-key-000000000000000000000000" with body:
      """
      {"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    # App B: request 1 — should succeed, App B has its own independent counter
    When I send a POST request to "http://localhost:8080/crbrl/chat/completions" with header "x-api-key" value "crbrl-app-b-key-000000000000000000000000" with body:
      """
      {"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "crbrl-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "crbrl-template"
    Then the response status code should be 200

  Scenario: Backend request limit blocks all consumers when shared quota is exhausted
    # Backend limit: 3 requests/hour shared across all apps.
    # Consumer limit: 3 requests/hour per app independently.
    # App A sends 3 requests — exhausts the shared backend counter.
    # App B's next request is blocked by the backend limit even though
    # App B's own consumer counter is still at 0.
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: crbrl-both-template
      spec:
        displayName: CRBRL Both Template
      """
    Then the response status code should be 201

    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: crbrl-both-provider
      spec:
        displayName: CRBRL Both Provider
        version: v1.0
        context: /crbrl-both
        template: crbrl-both-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: test-key
        accessControl:
          mode: allow_all
        policies:
          - name: api-key-auth
            version: v1
            paths:
              - path: /*
                methods: ['*']
                params:
                  key: x-api-key
                  in: header
          - name: advanced-ratelimit
            version: v1
            paths:
              - path: /*
                methods: ['*']
                params:
                  quotas:
                    - name: backend-request-limit
                      limits:
                        - limit: 3
                          duration: "1h"
                      keyExtraction:
                        - type: routename
          - name: advanced-ratelimit
            version: v1
            paths:
              - path: /*
                methods: ['*']
                params:
                  quotas:
                    - name: consumer-request-limit
                      limits:
                        - limit: 3
                          duration: "1h"
                      keyExtraction:
                        - type: routename
                        - type: metadata
                          key: x-wso2-application-id
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    # Create API key for App A
    When I send a POST request to the "gateway-controller" service at "/llm-providers/crbrl-both-provider/api-keys" with body:
      """
      {
        "name": "crbrl-both-app-a",
        "apiKey": "crbrl-both-app-a-key-00000000000000000000000"
      }
      """
    Then the response status code should be 201

    # Create API key for App B
    When I send a POST request to the "gateway-controller" service at "/llm-providers/crbrl-both-provider/api-keys" with body:
      """
      {
        "name": "crbrl-both-app-b",
        "apiKey": "crbrl-both-app-b-key-00000000000000000000000"
      }
      """
    Then the response status code should be 201
    And I wait for 2 seconds

    Given I set header "Content-Type" to "application/json"

    # App A: requests 1-3 — exhausts the shared backend counter (3/3)
    When I send a POST request to "http://localhost:8080/crbrl-both/chat/completions" with header "x-api-key" value "crbrl-both-app-a-key-00000000000000000000000" with body:
      """
      {"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/crbrl-both/chat/completions" with header "x-api-key" value "crbrl-both-app-a-key-00000000000000000000000" with body:
      """
      {"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/crbrl-both/chat/completions" with header "x-api-key" value "crbrl-both-app-a-key-00000000000000000000000" with body:
      """
      {"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # App B: blocked by the shared backend counter even though its own consumer counter is at 0
    When I send a POST request to "http://localhost:8080/crbrl-both/chat/completions" with header "x-api-key" value "crbrl-both-app-b-key-00000000000000000000000" with body:
      """
      {"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "crbrl-both-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "crbrl-both-template"
    Then the response status code should be 200

  Scenario: Requests without an app ID share a single "default" counter
    # When no api-key-auth is in the chain, x-wso2-application-id is never written to
    # metadata. The fallback key "default" is used instead of a "_missing_metadata_*_"
    # placeholder, so all unauthenticated requests count against the same "default" bucket.
    # Limit: 2 requests/hour. After 2 requests the "default" counter is exhausted and
    # all further requests (still with no app ID) are blocked.
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: crbrl-fallback-template
      spec:
        displayName: CRBRL Fallback Template
      """
    Then the response status code should be 201

    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: crbrl-fallback-provider
      spec:
        displayName: CRBRL Fallback Provider
        version: v1.0
        context: /crbrl-fallback
        template: crbrl-fallback-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: test-key
        accessControl:
          mode: allow_all
        policies:
          - name: advanced-ratelimit
            version: v1
            paths:
              - path: /*
                methods: ['*']
                params:
                  quotas:
                    - name: consumer-request-limit
                      limits:
                        - limit: 2
                          duration: "1h"
                      keyExtraction:
                        - type: routename
                        - type: metadata
                          key: x-wso2-application-id
                          fallback: default
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    # Request 1 — no app ID in metadata, key = "crbrl-fallback:default" — allowed (1/2)
    When I send a POST request to "http://localhost:8080/crbrl-fallback/chat/completions" with body:
      """
      {"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # Request 2 — no app ID in metadata, same "default" counter — allowed (2/2)
    When I send a POST request to "http://localhost:8080/crbrl-fallback/chat/completions" with body:
      """
      {"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # Request 3 — "default" counter exhausted — blocked
    When I send a POST request to "http://localhost:8080/crbrl-fallback/chat/completions" with body:
      """
      {"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "crbrl-fallback-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "crbrl-fallback-template"
    Then the response status code should be 200
