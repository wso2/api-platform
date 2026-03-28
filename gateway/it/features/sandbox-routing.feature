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

@sandbox-routing
Feature: Sandbox Routing
  As an API developer
  I want main and sandbox upstreams to route by host
  So that sandbox traffic can be validated independently

  Background:
    Given the gateway services are running

  Scenario: Route requests to different upstreams using main and sandbox vhosts
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-api-v1.0
      spec:
        displayName: Env-Routing-API
        version: v1.0
        context: /env/$version
        vhosts:
          main: main.local
          sandbox: sandbox.local
        upstream:
          main:
            url: http://sample-backend:9080
          sandbox:
            url: http://sample-backend:9080/sandbox
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/env/v1.0/whoami" to be ready with host "main.local"

    When I clear all headers
    And I set request host to "main.local"
    And I send a GET request to "http://localhost:8080/env/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "sandbox.local"
    And I send a GET request to "http://localhost:8080/env/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-api-v1.0"
    Then the response should be successful

  Scenario: Route requests with main via ref and sandbox via url
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-main-ref-v1.0
      spec:
        displayName: Env-Routing-Main-Ref-API
        version: v1.0
        context: /env-main-ref/$version
        vhosts:
          main: main-ref.local
          sandbox: sandbox-ref.local
        upstreamDefinitions:
          - name: main-upstream
            upstreams:
              - url: http://sample-backend:9080
        upstream:
          main:
            ref: main-upstream
          sandbox:
            url: http://sample-backend:9080/sandbox
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/env-main-ref/v1.0/whoami" to be ready with host "main-ref.local"

    When I clear all headers
    And I set request host to "main-ref.local"
    And I send a GET request to "http://localhost:8080/env-main-ref/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "sandbox-ref.local"
    And I send a GET request to "http://localhost:8080/env-main-ref/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-main-ref-v1.0"
    Then the response should be successful

  Scenario: Route requests with main via url and sandbox via ref
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-sandbox-ref-v1.0
      spec:
        displayName: Env-Routing-Sandbox-Ref-API
        version: v1.0
        context: /env-sandbox-ref/$version
        vhosts:
          main: main-sandbox-ref.local
          sandbox: sandbox-sandbox-ref.local
        upstreamDefinitions:
          - name: sandbox-upstream
            upstreams:
              - url: http://sample-backend:9080/sandbox
        upstream:
          main:
            url: http://sample-backend:9080
          sandbox:
            ref: sandbox-upstream
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/env-sandbox-ref/v1.0/whoami" to be ready with host "main-sandbox-ref.local"

    When I clear all headers
    And I set request host to "main-sandbox-ref.local"
    And I send a GET request to "http://localhost:8080/env-sandbox-ref/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "sandbox-sandbox-ref.local"
    And I send a GET request to "http://localhost:8080/env-sandbox-ref/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-sandbox-ref-v1.0"
    Then the response should be successful

  Scenario: Deploy API with missing main ref definition should fail
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-missing-ref-v1.0
      spec:
        displayName: Env-Routing-Missing-Ref-API
        version: v1.0
        context: /env-missing-ref/$version
        upstream:
          main:
            ref: non-existent-upstream
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Referenced upstream definition 'non-existent-upstream' not found"

  Scenario: Deploy API with invalid URL in upstreamDefinitions should fail
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-invalid-upstream-def-v1.0
      spec:
        displayName: Env-Routing-Invalid-Definition-API
        version: v1.0
        context: /env-invalid-def/$version
        upstreamDefinitions:
          - name: invalid-upstream
            upstreams:
              - url: ftp://sample-backend:9080
        upstream:
          main:
            ref: invalid-upstream
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "URL must use http or https scheme"

  # Expected functionality test:
  # Policies should apply to sandbox routes even when sandbox upstream is configured via `ref`.
  # This scenario is intended to expose parity gaps between `sandbox.url` and `sandbox.ref`.
  Scenario: Policy effects should apply for sandbox ref routes
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-sandbox-ref-policy-v1.0
      spec:
        displayName: Env-Routing-Sandbox-Ref-Policy-API
        version: v1.0
        context: /env-sandbox-ref-policy/$version
        vhosts:
          main: main-sandbox-policy.local
          sandbox: sandbox-sandbox-policy.local
        upstreamDefinitions:
          - name: sandbox-upstream
            upstreams:
              - url: http://sample-backend:9080/sandbox
        upstream:
          main:
            url: http://sample-backend:9080
          sandbox:
            ref: sandbox-upstream
        operations:
          - method: GET
            path: /whoami
            policies:
              - name: set-headers
                version: v1
                params:
                  response:
                    headers:
                      - name: X-Sandbox-Ref-Policy
                        value: applied
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/env-sandbox-ref-policy/v1.0/whoami" to be ready with host "main-sandbox-policy.local"

    When I clear all headers
    And I set request host to "main-sandbox-policy.local"
    And I send a GET request to "http://localhost:8080/env-sandbox-ref-policy/v1.0/whoami"
    Then the response should be successful
    And the response should have header "X-Sandbox-Ref-Policy" with value "applied"

    When I clear all headers
    And I set request host to "sandbox-sandbox-policy.local"
    And I send a GET request to "http://localhost:8080/env-sandbox-ref-policy/v1.0/whoami"
    Then the response should be successful
    And the response should have header "X-Sandbox-Ref-Policy" with value "applied"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-sandbox-ref-policy-v1.0"
    Then the response should be successful

  Scenario: Route requests with both main and sandbox via ref
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-both-ref-v1.0
      spec:
        displayName: Env-Routing-Both-Ref-API
        version: v1.0
        context: /env-both-ref/$version
        vhosts:
          main: main-both-ref.local
          sandbox: sandbox-both-ref.local
        upstreamDefinitions:
          - name: main-upstream
            upstreams:
              - url: http://sample-backend:9080
          - name: sandbox-upstream
            upstreams:
              - url: http://sample-backend:9080/sandbox
        upstream:
          main:
            ref: main-upstream
          sandbox:
            ref: sandbox-upstream
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/env-both-ref/v1.0/whoami" to be ready with host "main-both-ref.local"

    When I clear all headers
    And I set request host to "main-both-ref.local"
    And I send a GET request to "http://localhost:8080/env-both-ref/v1.0/whoami"
    Then the response should be successful
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "sandbox-both-ref.local"
    And I send a GET request to "http://localhost:8080/env-both-ref/v1.0/whoami"
    Then the response should be successful
    And the JSON response field "path" should be "/sandbox/whoami"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-both-ref-v1.0"
    Then the response should be successful

  Scenario: Sandbox ref with hostRewrite manual should preserve incoming host
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-sandbox-manual-host-v1.0
      spec:
        displayName: Env-Routing-Sandbox-Manual-Host-API
        version: v1.0
        context: /env-sandbox-manual/$version
        vhosts:
          main: main-sandbox-manual.local
          sandbox: sandbox-sandbox-manual.local
        upstreamDefinitions:
          - name: sandbox-upstream
            upstreams:
              - url: http://echo-backend:80/anything
        upstream:
          main:
            url: http://sample-backend:9080
          sandbox:
            ref: sandbox-upstream
            hostRewrite: manual
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/env-sandbox-manual/v1.0/whoami" to be ready with host "main-sandbox-manual.local"

    When I clear all headers
    And I set request host to "sandbox-sandbox-manual.local"
    And I send a GET request to "http://localhost:8080/env-sandbox-manual/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "headers.Host" should contain "sandbox-sandbox-manual.local"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-sandbox-manual-host-v1.0"
    Then the response should be successful

  Scenario: Sandbox ref should honor upstreamDefinitions connect timeout
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-sandbox-timeout-ref-v1.0
      spec:
        displayName: Env-Routing-Sandbox-Timeout-Ref-API
        version: v1.0
        context: /env-sandbox-timeout/$version
        vhosts:
          main: main-sandbox-timeout.local
          sandbox: sandbox-sandbox-timeout.local
        upstreamDefinitions:
          - name: sandbox-timeout-upstream
            timeout:
              connect: 6000ms
            upstreams:
              - url: http://192.0.2.1:80
        upstream:
          main:
            url: http://sample-backend:9080
          sandbox:
            ref: sandbox-timeout-upstream
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be successful
    And I record the current time as "request_start"
    When I clear all headers
    And I set request host to "sandbox-sandbox-timeout.local"
    And I send a GET request to "http://localhost:8080/env-sandbox-timeout/v1.0/whoami"
    Then the response status code should be 503
    And the request should have taken at least "6" seconds since "request_start"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-sandbox-timeout-ref-v1.0"
    Then the response should be successful

  Scenario: Sandbox ref should work with HTTP upstream
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-sandbox-https-ref-v1.0
      spec:
        displayName: Env-Routing-Sandbox-HTTPS-Ref-API
        version: v1.0
        context: /env-sandbox-https/$version
        vhosts:
          main: main-sandbox-https.local
          sandbox: sandbox-sandbox-https.local
        upstreamDefinitions:
          - name: sandbox-http-upstream
            upstreams:
              - url: http://mock-openapi:4010/openai/v1
        upstream:
          main:
            url: http://sample-backend:9080
          sandbox:
            ref: sandbox-http-upstream
        operations:
          - method: POST
            path: /chat/completions
      """
    Then the response should be successful
    And I wait for 3 seconds

    When I clear all headers
    And I set header "Content-Type" to "application/json"
    And I set header "Authorization" to "Bearer sk-test-key"
    And I set request host to "sandbox-sandbox-https.local"
    And I send a POST request to "http://localhost:8080/env-sandbox-https/v1.0/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [
          {"role": "user", "content": "Hello, how are you?"}
        ]
      }
      """
    Then the response should be successful
    And the response should be valid JSON
    And the response body should contain "chat.completion"
    And the response body should contain "choices"
    And the JSON response field "object" should be "chat.completion"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-sandbox-https-ref-v1.0"
    Then the response should be successful

  Scenario: Sandbox ref with multiple urls should use the first url
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-sandbox-multi-url-ref-v1.0
      spec:
        displayName: Env-Routing-Sandbox-Multi-URL-Ref-API
        version: v1.0
        context: /env-sandbox-multi/$version
        vhosts:
          main: main-sandbox-multi.local
          sandbox: sandbox-sandbox-multi.local
        upstreamDefinitions:
          - name: sandbox-multi-upstream
            upstreams:
              - url: http://sample-backend:9080/first
              - url: http://sample-backend:9080/sandbox
        upstream:
          main:
            url: http://sample-backend:9080
          sandbox:
            ref: sandbox-multi-upstream
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/env-sandbox-multi/v1.0/whoami" to be ready with host "sandbox-sandbox-multi.local"

    When I clear all headers
    And I set request host to "sandbox-sandbox-multi.local"
    And I send a GET request to "http://localhost:8080/env-sandbox-multi/v1.0/whoami"
    Then the response should be successful
    And the JSON response field "path" should be "/first/whoami"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-sandbox-multi-url-ref-v1.0"
    Then the response should be successful

  Scenario: Path params should route correctly with sandbox ref
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-sandbox-params-ref-v1.0
      spec:
        displayName: Env-Routing-Sandbox-Params-Ref-API
        version: v1.0
        context: /env-sandbox-params/$version
        vhosts:
          main: main-sandbox-params.local
          sandbox: sandbox-sandbox-params.local
        upstreamDefinitions:
          - name: sandbox-upstream
            upstreams:
              - url: http://sample-backend:9080/sandbox
        upstream:
          main:
            url: http://sample-backend:9080
          sandbox:
            ref: sandbox-upstream
        operations:
          - method: GET
            path: /{country}/{city}
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/env-sandbox-params/v1.0/us/seattle" to be ready with host "main-sandbox-params.local"

    When I clear all headers
    And I set request host to "main-sandbox-params.local"
    And I send a GET request to "http://localhost:8080/env-sandbox-params/v1.0/us/seattle"
    Then the response should be successful
    And the JSON response field "path" should be "/us/seattle"

    When I clear all headers
    And I set request host to "sandbox-sandbox-params.local"
    And I send a GET request to "http://localhost:8080/env-sandbox-params/v1.0/us/seattle"
    Then the response should be successful
    And the JSON response field "path" should be "/sandbox/us/seattle"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-sandbox-params-ref-v1.0"
    Then the response should be successful

  Scenario: Wildcard route should work with sandbox ref
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-sandbox-wildcard-ref-v1.0
      spec:
        displayName: Env-Routing-Sandbox-Wildcard-Ref-API
        version: v1.0
        context: /env-sandbox-wild/$version
        vhosts:
          main: main-sandbox-wild.local
          sandbox: sandbox-sandbox-wild.local
        upstreamDefinitions:
          - name: sandbox-upstream
            upstreams:
              - url: http://sample-backend:9080/sandbox
        upstream:
          main:
            url: http://sample-backend:9080
          sandbox:
            ref: sandbox-upstream
        operations:
          - method: GET
            path: /assets/*
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/env-sandbox-wild/v1.0/assets/a/b/c" to be ready with host "main-sandbox-wild.local"

    When I clear all headers
    And I set request host to "sandbox-sandbox-wild.local"
    And I send a GET request to "http://localhost:8080/env-sandbox-wild/v1.0/assets/a/b/c"
    Then the response should be successful
    And the response body should contain "/sandbox/"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-sandbox-wildcard-ref-v1.0"
    Then the response should be successful

  Scenario: Deploy should fail when sandbox ref is missing while main is valid
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-missing-sandbox-ref-v1.0
      spec:
        displayName: Env-Routing-Missing-Sandbox-Ref-API
        version: v1.0
        context: /env-missing-sandbox-ref/$version
        upstream:
          main:
            url: http://sample-backend:9080
          sandbox:
            ref: missing-sandbox-upstream
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Referenced upstream definition 'missing-sandbox-upstream' not found"

  Scenario: Deploy should fail for duplicate upstream definition names used by refs
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-duplicate-upstream-def-v1.0
      spec:
        displayName: Env-Routing-Duplicate-Upstream-Def-API
        version: v1.0
        context: /env-dup-upstream-def/$version
        upstreamDefinitions:
          - name: duplicate-upstream
            upstreams:
              - url: http://sample-backend:9080
          - name: duplicate-upstream
            upstreams:
              - url: http://sample-backend:9080/sandbox
        upstream:
          main:
            ref: duplicate-upstream
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Duplicate upstream definition name 'duplicate-upstream'"

  # Expected functionality test:
  # Policies should apply to sandbox routes when main and sandbox are both configured via `ref`.
  Scenario: Policy effects should apply for main ref and sandbox ref routes
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-both-ref-policy-v1.0
      spec:
        displayName: Env-Routing-Both-Ref-Policy-API
        version: v1.0
        context: /env-both-ref-policy/$version
        vhosts:
          main: main-both-ref-policy.local
          sandbox: sandbox-both-ref-policy.local
        upstreamDefinitions:
          - name: main-upstream
            upstreams:
              - url: http://sample-backend:9080
          - name: sandbox-upstream
            upstreams:
              - url: http://sample-backend:9080/sandbox
        upstream:
          main:
            ref: main-upstream
          sandbox:
            ref: sandbox-upstream
        operations:
          - method: GET
            path: /whoami
            policies:
              - name: set-headers
                version: v1
                params:
                  response:
                    headers:
                      - name: X-Both-Ref-Policy
                        value: applied
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/env-both-ref-policy/v1.0/whoami" to be ready with host "main-both-ref-policy.local"

    When I clear all headers
    And I set request host to "main-both-ref-policy.local"
    And I send a GET request to "http://localhost:8080/env-both-ref-policy/v1.0/whoami"
    Then the response should be successful
    And the response should have header "X-Both-Ref-Policy" with value "applied"

    When I clear all headers
    And I set request host to "sandbox-both-ref-policy.local"
    And I send a GET request to "http://localhost:8080/env-both-ref-policy/v1.0/whoami"
    Then the response should be successful
    And the response should have header "X-Both-Ref-Policy" with value "applied"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-both-ref-policy-v1.0"
    Then the response should be successful

  # Expected functionality test:
  # Policies should apply to sandbox routes when main uses `ref` and sandbox uses `url`.
  Scenario: Policy effects should apply for main ref and sandbox url routes
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-main-ref-policy-v1.0
      spec:
        displayName: Env-Routing-Main-Ref-Policy-API
        version: v1.0
        context: /env-main-ref-policy/$version
        vhosts:
          main: main-main-ref-policy.local
          sandbox: sandbox-main-ref-policy.local
        upstreamDefinitions:
          - name: main-upstream
            upstreams:
              - url: http://sample-backend:9080
        upstream:
          main:
            ref: main-upstream
          sandbox:
            url: http://sample-backend:9080/sandbox
        operations:
          - method: GET
            path: /whoami
            policies:
              - name: set-headers
                version: v1
                params:
                  response:
                    headers:
                      - name: X-Main-Ref-Policy
                        value: applied
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/env-main-ref-policy/v1.0/whoami" to be ready with host "main-main-ref-policy.local"

    When I clear all headers
    And I set request host to "main-main-ref-policy.local"
    And I send a GET request to "http://localhost:8080/env-main-ref-policy/v1.0/whoami"
    Then the response should be successful
    And the response should have header "X-Main-Ref-Policy" with value "applied"

    When I clear all headers
    And I set request host to "sandbox-main-ref-policy.local"
    And I send a GET request to "http://localhost:8080/env-main-ref-policy/v1.0/whoami"
    Then the response should be successful
    And the response should have header "X-Main-Ref-Policy" with value "applied"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-main-ref-policy-v1.0"
    Then the response should be successful

  # An API deployed with main-only must NOT serve traffic on the sandbox vhost.
  # Prevents a regression where the router falls through to the main upstream
  # for unmatched sandbox-vhost requests.
  Scenario: Requests to sandbox vhost are rejected when API has no sandbox upstream
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-no-sandbox-v1.0
      spec:
        displayName: Env-Routing-No-Sandbox-API
        version: v1.0
        context: /env-no-sandbox/$version
        vhosts:
          main: no-sandbox-main.local
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/env-no-sandbox/v1.0/whoami" to be ready with host "no-sandbox-main.local"

    When I clear all headers
    And I set request host to "no-sandbox-main.local"
    And I send a GET request to "http://localhost:8080/env-no-sandbox/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "no-sandbox-sandbox.local"
    And I send a GET request to "http://localhost:8080/env-no-sandbox/v1.0/whoami"
    Then the response status code should be 404

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-no-sandbox-v1.0"
    Then the response should be successful

  # Validates URL scheme check on the direct upstream.sandbox.url field.
  # This exercises a different code path from the upstreamDefinitions URL validation
  # and produces a distinct error message ("Upstream URL must use http or https scheme").
  Scenario: Deploy API with invalid URL scheme in direct sandbox url should fail
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-invalid-sandbox-url-v1.0
      spec:
        displayName: Env-Routing-Invalid-Sandbox-URL-API
        version: v1.0
        context: /env-invalid-sandbox-url/$version
        upstream:
          main:
            url: http://sample-backend:9080
          sandbox:
            url: ftp://sample-backend:9080
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Upstream URL must use http or https scheme"

  # Completes the 2×2 policy matrix (main×sandbox) × (url×ref).
  # The other three combinations are covered by existing scenarios.
  Scenario: Policy effects should apply for main url and sandbox url routes
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-both-url-policy-v1.0
      spec:
        displayName: Env-Routing-Both-URL-Policy-API
        version: v1.0
        context: /env-both-url-policy/$version
        vhosts:
          main: main-both-url-policy.local
          sandbox: sandbox-both-url-policy.local
        upstream:
          main:
            url: http://sample-backend:9080
          sandbox:
            url: http://sample-backend:9080/sandbox
        operations:
          - method: GET
            path: /whoami
            policies:
              - name: set-headers
                version: v1
                params:
                  response:
                    headers:
                      - name: X-Both-URL-Policy
                        value: applied
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/env-both-url-policy/v1.0/whoami" to be ready with host "main-both-url-policy.local"

    When I clear all headers
    And I set request host to "main-both-url-policy.local"
    And I send a GET request to "http://localhost:8080/env-both-url-policy/v1.0/whoami"
    Then the response should be successful
    And the response should have header "X-Both-URL-Policy" with value "applied"

    When I clear all headers
    And I set request host to "sandbox-both-url-policy.local"
    And I send a GET request to "http://localhost:8080/env-both-url-policy/v1.0/whoami"
    Then the response should be successful
    And the response should have header "X-Both-URL-Policy" with value "applied"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-both-url-policy-v1.0"
    Then the response should be successful

  # Simulates the common operational flow of first deploying to production only,
  # then later enabling sandbox by re-deploying with a sandbox upstream added.
  Scenario: Sandbox routing becomes available after redeploying API with sandbox upstream added
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-redeploy-sandbox-v1.0
      spec:
        displayName: Env-Routing-Redeploy-Sandbox-API
        version: v1.0
        context: /env-redeploy-sandbox/$version
        vhosts:
          main: redeploy-main.local
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/env-redeploy-sandbox/v1.0/whoami" to be ready with host "redeploy-main.local"

    When I clear all headers
    And I set request host to "redeploy-main.local"
    And I send a GET request to "http://localhost:8080/env-redeploy-sandbox/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I clear all headers
    And I set request host to "redeploy-sandbox.local"
    And I send a GET request to "http://localhost:8080/env-redeploy-sandbox/v1.0/whoami"
    Then the response status code should be 404

    Given I authenticate using basic auth as "admin"
    When I update the API "env-routing-redeploy-sandbox-v1.0" with this configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-redeploy-sandbox-v1.0
      spec:
        displayName: Env-Routing-Redeploy-Sandbox-API
        version: v1.0
        context: /env-redeploy-sandbox/$version
        vhosts:
          main: redeploy-main.local
          sandbox: redeploy-sandbox.local
        upstream:
          main:
            url: http://sample-backend:9080
          sandbox:
            url: http://sample-backend:9080/sandbox
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/env-redeploy-sandbox/v1.0/whoami" to be ready with host "redeploy-sandbox.local"

    When I clear all headers
    And I set request host to "redeploy-sandbox.local"
    And I send a GET request to "http://localhost:8080/env-redeploy-sandbox/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-redeploy-sandbox-v1.0"
    Then the response should be successful

  # When vhosts.main is customised but vhosts.sandbox is omitted, the sandbox upstream
  # is still reachable via the global default sandbox vhost (sandbox-*).
  Scenario: Sandbox routes use global default vhost when vhosts.sandbox is omitted
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-sandbox-vhost-fallback-v1.0
      spec:
        displayName: Env-Routing-Sandbox-Vhost-Fallback-API
        version: v1.0
        context: /env-sb-vhost-fallback/$version
        vhosts:
          main: sb-vhost-fallback-main.local
        upstream:
          main:
            url: http://sample-backend:9080
          sandbox:
            url: http://sample-backend:9080/sandbox
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/env-sb-vhost-fallback/v1.0/whoami" to be ready with host "sb-vhost-fallback-main.local"

    When I clear all headers
    And I set request host to "sb-vhost-fallback-main.local"
    And I send a GET request to "http://localhost:8080/env-sb-vhost-fallback/v1.0/whoami"
    Then the response should be successful
    And the JSON response field "path" should be "/whoami"

    # sandbox-sb-vhost-fallback.local matches sandbox-* (global default) — sandbox upstream reachable
    And I wait for the endpoint "http://localhost:8080/env-sb-vhost-fallback/v1.0/whoami" to be ready with host "sandbox-sb-vhost-fallback.local"
    When I clear all headers
    And I set request host to "sandbox-sb-vhost-fallback.local"
    And I send a GET request to "http://localhost:8080/env-sb-vhost-fallback/v1.0/whoami"
    Then the response should be successful
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-sandbox-vhost-fallback-v1.0"
    Then the response should be successful

  # Stronger isolation check than the existing "no-sandbox" scenario:
  # uses a host that actually matches the global sandbox-* pattern, confirming
  # that a main-only API registers no routes under the real sandbox virtual host.
  Scenario: Requests to global default sandbox vhost are rejected when API has no sandbox upstream
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: env-routing-no-sandbox-global-vhost-v1.0
      spec:
        displayName: Env-Routing-No-Sandbox-Global-Vhost-API
        version: v1.0
        context: /env-no-sb-global/$version
        vhosts:
          main: no-sb-global-main.local
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /whoami
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/env-no-sb-global/v1.0/whoami" to be ready with host "no-sb-global-main.local"

    When I clear all headers
    And I set request host to "no-sb-global-main.local"
    And I send a GET request to "http://localhost:8080/env-no-sb-global/v1.0/whoami"
    Then the response should be successful
    And the JSON response field "path" should be "/whoami"

    # sandbox-no-sb-global.local matches sandbox-* (global default sandbox vhost).
    # Because this API has no sandbox upstream, no routes exist there → 404.
    When I clear all headers
    And I set request host to "sandbox-no-sb-global.local"
    And I send a GET request to "http://localhost:8080/env-no-sb-global/v1.0/whoami"
    Then the response status code should be 404

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-no-sandbox-global-vhost-v1.0"
    Then the response should be successful
