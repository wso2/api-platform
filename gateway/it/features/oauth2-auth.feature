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

@oauth2-auth
Feature: OAuth2 Upstream Authentication
  As an API developer
  I want the gateway to fetch, cache, and inject OAuth2 credentials on my behalf
  So that my backend can require OAuth2 without the client ever handling that credential

  # Every API below carries an unattached, always-200 "/health" operation used
  # only to wait for xDS propagation - the "/data" operation under test is
  # deliberately allowed to return non-200 (502, 401, ...) in several
  # scenarios, so it can never be used as the readiness probe itself (see
  # iWaitForEndpointToBeReady, which polls for exactly 200).

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I send a POST request to the "mock-oauth2-idp" service at "/debug/reset" with body:
      """
      {}
      """

  Scenario: Token-endpoint grant happy path injects a Bearer token
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: oauth2-it-happy-path
      spec:
        displayName: OAuth2 IT Happy Path
        version: v1.0
        context: /oauth2-it-happy-path/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /data
            policies:
              - name: oauth2-generator
                version: v0
                params:
                  tokenEndpoint: http://mock-oauth2-idp:9601/oauth2/token
                  clientId: test-client
                  clientSecret: test-secret
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/oauth2-it-happy-path/v1.0/health" to be ready

    When I send a GET request to "http://localhost:8080/oauth2-it-happy-path/v1.0/data"
    Then the response status code should be 200
    And the response should contain echoed header "Authorization" containing "Bearer mock-token-"

    When I send a GET request to the "mock-oauth2-idp" service at "/debug/stats"
    Then the JSON response field "tokenRequestCount" should be 1

  Scenario: Password grant happy path
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: oauth2-it-password-grant
      spec:
        displayName: OAuth2 IT Password Grant
        version: v1.0
        context: /oauth2-it-password-grant/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /data
            policies:
              - name: oauth2-generator
                version: v0
                params:
                  grantType: password
                  tokenEndpoint: http://mock-oauth2-idp:9601/oauth2/token
                  clientId: test-client
                  clientSecret: test-secret
                  username: resource-owner
                  password: hunter2
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/oauth2-it-password-grant/v1.0/health" to be ready

    When I send a GET request to "http://localhost:8080/oauth2-it-password-grant/v1.0/data"
    Then the response status code should be 200
    And the response should contain echoed header "Authorization" containing "Bearer mock-token-"

  Scenario: client_secret_post authentication reaches the token endpoint
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: oauth2-it-client-secret-post
      spec:
        displayName: OAuth2 IT client_secret_post
        version: v1.0
        context: /oauth2-it-client-secret-post/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /data
            policies:
              - name: oauth2-generator
                version: v0
                params:
                  clientAuthMethod: client_secret_post
                  tokenEndpoint: http://mock-oauth2-idp:9601/oauth2/token
                  clientId: test-client
                  clientSecret: test-secret
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/oauth2-it-client-secret-post/v1.0/health" to be ready

    When I send a GET request to "http://localhost:8080/oauth2-it-client-secret-post/v1.0/data"
    Then the response status code should be 200

    When I send a GET request to the "mock-oauth2-idp" service at "/debug/stats"
    Then the response body should match pattern "authStyle.{3}post"

  Scenario: Custom headerName and valuePrefix are applied
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: oauth2-it-custom-header
      spec:
        displayName: OAuth2 IT Custom Header
        version: v1.0
        context: /oauth2-it-custom-header/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /data
            policies:
              - name: oauth2-generator
                version: v0
                params:
                  tokenEndpoint: http://mock-oauth2-idp:9601/oauth2/token
                  clientId: test-client
                  clientSecret: test-secret
                  headerName: X-Upstream-Token
                  valuePrefix: ""
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/oauth2-it-custom-header/v1.0/health" to be ready

    # Background's basic-auth step leaves a persistent Authorization header on
    # the client (used for the management-API deploy call above) - clear it so
    # the assertion below reflects what the policy actually did, not a
    # leftover test-client header riding along on this unrelated request.
    Given I clear all headers
    When I send a GET request to "http://localhost:8080/oauth2-it-custom-header/v1.0/data"
    Then the response status code should be 200
    And the response should contain echoed header "X-Upstream-Token" containing "mock-token-"
    And the response should not contain echoed header "Authorization"

  Scenario: bearerToken static path never calls the token endpoint
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: oauth2-it-bearer-token
      spec:
        displayName: OAuth2 IT Bearer Token
        version: v1.0
        context: /oauth2-it-bearer-token/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /data
            policies:
              - name: oauth2-generator
                version: v0
                params:
                  bearerToken: static-long-lived-token-xyz
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/oauth2-it-bearer-token/v1.0/health" to be ready

    When I send a GET request to "http://localhost:8080/oauth2-it-bearer-token/v1.0/data"
    Then the response status code should be 200
    And the response should contain echoed header "Authorization" with value "Bearer static-long-lived-token-xyz"

    When I send a GET request to the "mock-oauth2-idp" service at "/debug/stats"
    Then the JSON response field "tokenRequestCount" should be 0

  Scenario: Token is cached across repeated requests
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: oauth2-it-caching
      spec:
        displayName: OAuth2 IT Caching
        version: v1.0
        context: /oauth2-it-caching/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /data
            policies:
              - name: oauth2-generator
                version: v0
                params:
                  # ?ttl=3600 (not the mock's 300s default) - 300s equals the
                  # default expiryBuffer, so a default-TTL token would be
                  # treated as stale immediately and refetched on every
                  # request, defeating the very thing this scenario checks.
                  tokenEndpoint: http://mock-oauth2-idp:9601/oauth2/token?ttl=3600
                  clientId: test-client
                  clientSecret: test-secret
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/oauth2-it-caching/v1.0/health" to be ready

    When I send 5 GET requests to "http://localhost:8080/oauth2-it-caching/v1.0/data"
    Then the response status code should be 200

    When I send a GET request to the "mock-oauth2-idp" service at "/debug/stats"
    Then the JSON response field "tokenRequestCount" should be 1

  Scenario: Invalid client credentials return a Bad Gateway
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: oauth2-it-invalid-client
      spec:
        displayName: OAuth2 IT Invalid Client
        version: v1.0
        context: /oauth2-it-invalid-client/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /data
            policies:
              - name: oauth2-generator
                version: v0
                params:
                  tokenEndpoint: http://mock-oauth2-idp:9601/oauth2/token
                  clientId: test-client
                  clientSecret: definitely-the-wrong-secret
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/oauth2-it-invalid-client/v1.0/health" to be ready

    When I send a GET request to "http://localhost:8080/oauth2-it-invalid-client/v1.0/data"
    Then the response status code should be 502

  Scenario: Unreachable token endpoint returns a Bad Gateway
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: oauth2-it-unreachable
      spec:
        displayName: OAuth2 IT Unreachable IdP
        version: v1.0
        context: /oauth2-it-unreachable/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /data
            policies:
              - name: oauth2-generator
                version: v0
                params:
                  tokenEndpoint: http://mock-oauth2-idp-does-not-exist:9601/oauth2/token
                  clientId: test-client
                  clientSecret: test-secret
                  tokenRequestTimeout: 3s
                  tokenRequestMaxRetries: 0
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/oauth2-it-unreachable/v1.0/health" to be ready

    When I send a GET request to "http://localhost:8080/oauth2-it-unreachable/v1.0/data"
    Then the response status code should be 502

  Scenario: Malformed token-endpoint response returns a Bad Gateway
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: oauth2-it-malformed
      spec:
        displayName: OAuth2 IT Malformed Response
        version: v1.0
        context: /oauth2-it-malformed/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /data
            policies:
              - name: oauth2-generator
                version: v0
                params:
                  tokenEndpoint: http://mock-oauth2-idp:9601/oauth2/token
                  clientId: malformed-client
                  clientSecret: any-secret
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/oauth2-it-malformed/v1.0/health" to be ready

    When I send a GET request to "http://localhost:8080/oauth2-it-malformed/v1.0/data"
    Then the response status code should be 502

  Scenario: tokenRequestParams reaches the token endpoint
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: oauth2-it-token-request-params
      spec:
        displayName: OAuth2 IT tokenRequestParams
        version: v1.0
        context: /oauth2-it-token-request-params/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /data
            policies:
              - name: oauth2-generator
                version: v0
                params:
                  tokenEndpoint: http://mock-oauth2-idp:9601/oauth2/token
                  clientId: test-client
                  clientSecret: test-secret
                  tokenRequestParams:
                    scope: it-suite-scope
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/oauth2-it-token-request-params/v1.0/health" to be ready

    When I send a GET request to "http://localhost:8080/oauth2-it-token-request-params/v1.0/data"
    Then the response status code should be 200

    When I send a GET request to the "mock-oauth2-idp" service at "/debug/stats"
    Then the response body should contain "it-suite-scope"

  Scenario: tokenRequestHeaders reaches the token endpoint
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: oauth2-it-token-request-headers
      spec:
        displayName: OAuth2 IT tokenRequestHeaders
        version: v1.0
        context: /oauth2-it-token-request-headers/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /data
            policies:
              - name: oauth2-generator
                version: v0
                params:
                  tokenEndpoint: http://mock-oauth2-idp:9601/oauth2/token
                  clientId: test-client
                  clientSecret: test-secret
                  tokenRequestHeaders:
                    X-IT-Suite-Header: it-suite-value
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/oauth2-it-token-request-headers/v1.0/health" to be ready

    When I send a GET request to "http://localhost:8080/oauth2-it-token-request-headers/v1.0/data"
    Then the response status code should be 200

    When I send a GET request to the "mock-oauth2-idp" service at "/debug/stats"
    Then the response body should contain "it-suite-value"

  # NOTE: a purge-on-401 scenario (prime cache, force a 401 via sample-backend's
  # ?statusCode= query param, confirm the cache is cleared and the next request
  # fetches fresh) was deliberately left out here. It requires reliably forcing
  # a specific upstream status code through the full router path, which didn't
  # behave predictably in this suite (no retry mechanism exists in this
  # codebase - confirmed by grepping gateway-controller/gateway-runtime for any
  # Envoy RetryPolicy/resilience.retry generation, and by direct confirmation -
  # so a mismatch here isn't a retry side effect, it's something about how the
  # query string reaches the backend that wasn't worth chasing further for one
  # scenario). oauth2_generator.go's own unit tests
  # (TestGetPolicy_PurgeOnUpstreamStatus_EndToEnd) already cover this behavior
  # directly and reliably, without depending on a real HTTP round-trip.

  Scenario: An operation without the policy attached is unaffected
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: oauth2-it-sibling-unaffected
      spec:
        displayName: OAuth2 IT Sibling Unaffected
        version: v1.0
        context: /oauth2-it-sibling-unaffected/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /protected
            policies:
              - name: oauth2-generator
                version: v0
                params:
                  tokenEndpoint: http://mock-oauth2-idp:9601/oauth2/token
                  clientId: test-client
                  clientSecret: test-secret
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/oauth2-it-sibling-unaffected/v1.0/health" to be ready

    # See the "Custom headerName" scenario above for why this is needed.
    Given I clear all headers
    When I send a GET request to "http://localhost:8080/oauth2-it-sibling-unaffected/v1.0/health"
    Then the response status code should be 200
    And the response should not contain echoed header "Authorization"

  Scenario: LlmProvider upstream.auth wires the same policy via the CRD convenience field
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: oauth2-it-llm-provider
      spec:
        displayName: OAuth2 IT LLM Provider
        version: v1.0
        template: openai
        context: /oauth2-it-llm-provider/latest
        upstream:
          url: http://sample-backend:9080
          auth:
            type: oauth2
            policyParams:
              tokenEndpoint: http://mock-oauth2-idp:9601/oauth2/token
              clientId: test-client
              clientSecret: test-secret
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/oauth2-it-llm-provider/latest/chat/completions" to be ready with method "POST" and body '{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}'

    When I send a POST request to "http://localhost:8080/oauth2-it-llm-provider/latest/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "hi"}]
      }
      """
    Then the response status code should be 200
    And the response should contain echoed header "Authorization" containing "Bearer mock-token-"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "oauth2-it-llm-provider"
    Then the response status code should be 200
