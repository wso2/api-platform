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
@sandbox-routing
Feature: Sandbox Routing
  As an API developer
  I want main and sandbox upstreams to route by host
  So that sandbox traffic can be validated independently

  Background:
    Given the gateway services are running

  Scenario: Route requests to different upstreams using main and sandbox vhosts
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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
            url: http://testbench:3000
          sandbox:
            url: http://testbench:3000/sandbox
        operations:
          - method: GET
            path: /whoami
      """
    Then the response status code should be 201
    And I set request host to "main.local"
    And I send a "GET" request to "/env/v1.0/whoami" until status 200
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I reset the request
    And I set request host to "sandbox.local"
    And I send a "GET" request to "/env/v1.0/whoami"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-api-v1.0"
    Then the response status code should be 200

  Scenario: Route requests with main via ref and sandbox via url
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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
              - url: http://testbench:3000
        upstream:
          main:
            ref: main-upstream
          sandbox:
            url: http://testbench:3000/sandbox
        operations:
          - method: GET
            path: /whoami
      """
    Then the response status code should be 201
    And I set request host to "main-ref.local"
    And I send a "GET" request to "/env-main-ref/v1.0/whoami" until status 200
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I reset the request
    And I set request host to "sandbox-ref.local"
    And I send a "GET" request to "/env-main-ref/v1.0/whoami"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-main-ref-v1.0"
    Then the response status code should be 200

  Scenario: Route requests with main via url and sandbox via ref
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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
            basePath: /sandbox
            upstreams:
              - url: http://testbench:3000
        upstream:
          main:
            url: http://testbench:3000
          sandbox:
            ref: sandbox-upstream
        operations:
          - method: GET
            path: /whoami
      """
    Then the response status code should be 201
    And I set request host to "main-sandbox-ref.local"
    And I send a "GET" request to "/env-sandbox-ref/v1.0/whoami" until status 200
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I reset the request
    And I set request host to "sandbox-sandbox-ref.local"
    And I send a "GET" request to "/env-sandbox-ref/v1.0/whoami"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-sandbox-ref-v1.0"
    Then the response status code should be 200

  Scenario: Deploy API with missing main ref definition should fail
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "errors[0].message" should be "Referenced upstream definition 'non-existent-upstream' not found: no upstreamDefinitions provided"

  Scenario: Deploy API with invalid URL in upstreamDefinitions should fail
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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
              - url: ftp://testbench:3000
        upstream:
          main:
            ref: invalid-upstream
        operations:
          - method: GET
            path: /whoami
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "errors[0].message" should be "URL must use http or https scheme"

  # Expected functionality test:
  # Policies should apply to sandbox routes even when sandbox upstream is configured via `ref`.
  # This scenario is intended to expose parity gaps between `sandbox.url` and `sandbox.ref`.
  Scenario: Policy effects should apply for sandbox ref routes
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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
            basePath: /sandbox
            upstreams:
              - url: http://testbench:3000
        upstream:
          main:
            url: http://testbench:3000
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
    Then the response status code should be 201
    And I set request host to "main-sandbox-policy.local"
    And I send a "GET" request to "/env-sandbox-ref-policy/v1.0/whoami" until status 200
    And the response header "X-Sandbox-Ref-Policy" should be "applied"

    When I reset the request
    And I set request host to "sandbox-sandbox-policy.local"
    And I send a "GET" request to "/env-sandbox-ref-policy/v1.0/whoami"
    Then the response status code should be 200
    And the response header "X-Sandbox-Ref-Policy" should be "applied"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-sandbox-ref-policy-v1.0"
    Then the response status code should be 200

  Scenario: Route requests with both main and sandbox via ref
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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
              - url: http://testbench:3000
          - name: sandbox-upstream
            basePath: /sandbox
            upstreams:
              - url: http://testbench:3000
        upstream:
          main:
            ref: main-upstream
          sandbox:
            ref: sandbox-upstream
        operations:
          - method: GET
            path: /whoami
      """
    Then the response status code should be 201
    And I set request host to "main-both-ref.local"
    And I send a "GET" request to "/env-both-ref/v1.0/whoami" until status 200
    And the JSON response field "path" should be "/whoami"

    When I reset the request
    And I set request host to "sandbox-both-ref.local"
    And I send a "GET" request to "/env-both-ref/v1.0/whoami"
    Then the response status code should be 200
    And the JSON response field "path" should be "/sandbox/whoami"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-both-ref-v1.0"
    Then the response status code should be 200

  Scenario: Sandbox ref with hostRewrite manual should preserve incoming host
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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
            basePath: /anything
            upstreams:
              - url: http://testbench:3002
        upstream:
          main:
            url: http://testbench:3000
          sandbox:
            ref: sandbox-upstream
            hostRewrite: manual
        operations:
          - method: GET
            path: /whoami
      """
    Then the response status code should be 201
    And I set request host to "main-sandbox-manual.local"
    And I send a "GET" request to "/env-sandbox-manual/v1.0/whoami" until status 200

    When I reset the request
    And I set request host to "sandbox-sandbox-manual.local"
    And I send a "GET" request to "/env-sandbox-manual/v1.0/whoami"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "headers.Host" should contain "sandbox-sandbox-manual.local"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-sandbox-manual-host-v1.0"
    Then the response status code should be 200

  Scenario: Sandbox ref should honor upstreamDefinitions connect timeout
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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
            url: http://testbench:3000
          sandbox:
            ref: sandbox-timeout-upstream
        operations:
          - method: GET
            path: /whoami
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync
    And I set request host to "main-sandbox-timeout.local"
    And I send a "GET" request to "/env-sandbox-timeout/v1.0/whoami" until status 200
    When I reset the request
    And I set request host to "sandbox-sandbox-timeout.local"
    And I send a "GET" request to "/env-sandbox-timeout/v1.0/whoami"
    Then the gateway should have timed out after "6" seconds with status 503

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-sandbox-timeout-ref-v1.0"
    Then the response status code should be 200

  Scenario: Sandbox ref should work with HTTP upstream
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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
            basePath: /openai/v1
            upstreams:
              - url: http://testbench:3008
        upstream:
          main:
            url: http://testbench:3000
          sandbox:
            ref: sandbox-http-upstream
        operations:
          - method: POST
            path: /chat/completions
      """
    Then the response status code should be 201

    # Readiness on the MAIN vhost: both vhosts' routes for one API are programmed together, so a
    # serving main route proves the sandbox one is live. The sibling sandbox-timeout scenario
    # gates the same way and depends on it — it needs 503, not 404, from the sandbox host.
    When I set request host to "main-sandbox-https.local"
    And I send a "POST" request to "/env-sandbox-https/v1.0/chat/completions" until status 200

    When I reset the request
    And I set header "Content-Type" to "application/json"
    And I set header "Authorization" to "Bearer sk-test-key"
    And I set request host to "sandbox-sandbox-https.local"
    And I send a "POST" request to "/env-sandbox-https/v1.0/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [
          {"role": "user", "content": "Hello, how are you?"}
        ]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "object" should be "chat.completion"
    And the JSON response should have field "choices"
    And the JSON response field "object" should be "chat.completion"

    # The scenario set a bearer token for the data-plane call, and a sticky Authorization
    # header outranks basic auth on the management API — so the cleanup must clear it first
    # or the DELETE is rejected 401 for a reason unrelated to what this scenario tests.
    When I reset the request
    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-sandbox-https-ref-v1.0"
    Then the response status code should be 200

  # upstreamDefinitions URLs must be host[:port] only; the base path belongs in basePath.
  # A path embedded in the URL (here on load-balanced URLs) is rejected with a 400.
  Scenario: Deploy fails when an upstreamDefinitions URL contains a path
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: env-routing-sandbox-multi-url-ref-v1.0
      spec:
        displayName: Env-Routing-Sandbox-Multi-URL-Ref-API
        version: v1.0
        context: /env-sandbox-multi/$version
        upstreamDefinitions:
          - name: sandbox-multi-upstream
            upstreams:
              - url: http://testbench:3000/first
              - url: http://testbench:3000/sandbox
        upstream:
          main:
            ref: sandbox-multi-upstream
        operations:
          - method: GET
            path: /whoami
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "errors[0].message" should be "URL must not include a path; set the base path in upstreamDefinitions[].basePath instead"

  Scenario: Path params should route correctly with sandbox ref
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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
            basePath: /sandbox
            upstreams:
              - url: http://testbench:3000
        upstream:
          main:
            url: http://testbench:3000
          sandbox:
            ref: sandbox-upstream
        operations:
          - method: GET
            path: /{country}/{city}
      """
    Then the response status code should be 201
    And I set request host to "main-sandbox-params.local"
    And I send a "GET" request to "/env-sandbox-params/v1.0/us/seattle" until status 200
    And the JSON response field "path" should be "/us/seattle"

    When I reset the request
    And I set request host to "sandbox-sandbox-params.local"
    And I send a "GET" request to "/env-sandbox-params/v1.0/us/seattle"
    Then the response status code should be 200
    And the JSON response field "path" should be "/sandbox/us/seattle"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-sandbox-params-ref-v1.0"
    Then the response status code should be 200

  Scenario: Wildcard route should work with sandbox ref
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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
            basePath: /sandbox
            upstreams:
              - url: http://testbench:3000
        upstream:
          main:
            url: http://testbench:3000
          sandbox:
            ref: sandbox-upstream
        operations:
          - method: GET
            path: /assets/*
      """
    Then the response status code should be 201
    And I set request host to "main-sandbox-wild.local"
    And I send a "GET" request to "/env-sandbox-wild/v1.0/assets/a/b/c" until status 200

    When I reset the request
    And I set request host to "sandbox-sandbox-wild.local"
    And I send a "GET" request to "/env-sandbox-wild/v1.0/assets/a/b/c"
    Then the response status code should be 200
    And the JSON response field "path" should be "/sandbox/assets/a/b/c"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-sandbox-wildcard-ref-v1.0"
    Then the response status code should be 200

  Scenario: Deploy should fail when sandbox ref is missing while main is valid
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: env-routing-missing-sandbox-ref-v1.0
      spec:
        displayName: Env-Routing-Missing-Sandbox-Ref-API
        version: v1.0
        context: /env-missing-sandbox-ref/$version
        upstream:
          main:
            url: http://testbench:3000
          sandbox:
            ref: missing-sandbox-upstream
        operations:
          - method: GET
            path: /whoami
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "errors[0].message" should be "Referenced upstream definition 'missing-sandbox-upstream' not found: no upstreamDefinitions provided"

  Scenario: Deploy should fail for duplicate upstream definition names used by refs
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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
              - url: http://testbench:3000
          - name: duplicate-upstream
            upstreams:
              - url: http://testbench:3000/sandbox
        upstream:
          main:
            ref: duplicate-upstream
        operations:
          - method: GET
            path: /whoami
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "errors[0].message" should be "Duplicate upstream definition name 'duplicate-upstream'"

  # Expected functionality test:
  # Policies should apply to sandbox routes when main and sandbox are both configured via `ref`.
  Scenario: Policy effects should apply for main ref and sandbox ref routes
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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
              - url: http://testbench:3000
          - name: sandbox-upstream
            basePath: /sandbox
            upstreams:
              - url: http://testbench:3000
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
    Then the response status code should be 201
    And I set request host to "main-both-ref-policy.local"
    And I send a "GET" request to "/env-both-ref-policy/v1.0/whoami" until status 200
    And the response header "X-Both-Ref-Policy" should be "applied"

    When I reset the request
    And I set request host to "sandbox-both-ref-policy.local"
    And I send a "GET" request to "/env-both-ref-policy/v1.0/whoami"
    Then the response status code should be 200
    And the response header "X-Both-Ref-Policy" should be "applied"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-both-ref-policy-v1.0"
    Then the response status code should be 200

  # Expected functionality test:
  # Policies should apply to sandbox routes when main uses `ref` and sandbox uses `url`.
  Scenario: Policy effects should apply for main ref and sandbox url routes
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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
              - url: http://testbench:3000
        upstream:
          main:
            ref: main-upstream
          sandbox:
            url: http://testbench:3000/sandbox
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
    Then the response status code should be 201
    And I set request host to "main-main-ref-policy.local"
    And I send a "GET" request to "/env-main-ref-policy/v1.0/whoami" until status 200
    And the response header "X-Main-Ref-Policy" should be "applied"

    When I reset the request
    And I set request host to "sandbox-main-ref-policy.local"
    And I send a "GET" request to "/env-main-ref-policy/v1.0/whoami"
    Then the response status code should be 200
    And the response header "X-Main-Ref-Policy" should be "applied"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-main-ref-policy-v1.0"
    Then the response status code should be 200

  # An API deployed with main-only must NOT serve traffic on the sandbox vhost.
  # Prevents a regression where the router falls through to the main upstream
  # for unmatched sandbox-vhost requests.
  Scenario: Requests to sandbox vhost are rejected when API has no sandbox upstream
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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
            url: http://testbench:3000
        operations:
          - method: GET
            path: /whoami
      """
    Then the response status code should be 201
    And I set request host to "no-sandbox-main.local"
    And I send a "GET" request to "/env-no-sandbox/v1.0/whoami" until status 200
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I reset the request
    And I set request host to "no-sandbox-sandbox.local"
    And I send a "GET" request to "/env-no-sandbox/v1.0/whoami"
    Then the response status code should be 404

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-no-sandbox-v1.0"
    Then the response status code should be 200

  # Validates URL scheme check on the direct upstream.sandbox.url field.
  # This exercises a different code path from the upstreamDefinitions URL validation
  # and produces a distinct error message ("Upstream URL must use http or https scheme").
  Scenario: Deploy API with invalid URL scheme in direct sandbox url should fail
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: env-routing-invalid-sandbox-url-v1.0
      spec:
        displayName: Env-Routing-Invalid-Sandbox-URL-API
        version: v1.0
        context: /env-invalid-sandbox-url/$version
        upstream:
          main:
            url: http://testbench:3000
          sandbox:
            url: ftp://testbench:3000
        operations:
          - method: GET
            path: /whoami
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response field "errors[0].message" should be "Upstream URL must use http or https scheme"

  # Completes the 2×2 policy matrix (main×sandbox) × (url×ref).
  # The other three combinations are covered by existing scenarios.
  Scenario: Policy effects should apply for main url and sandbox url routes
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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
            url: http://testbench:3000
          sandbox:
            url: http://testbench:3000/sandbox
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
    Then the response status code should be 201
    And I set request host to "main-both-url-policy.local"
    And I send a "GET" request to "/env-both-url-policy/v1.0/whoami" until status 200
    And the response header "X-Both-URL-Policy" should be "applied"

    When I reset the request
    And I set request host to "sandbox-both-url-policy.local"
    And I send a "GET" request to "/env-both-url-policy/v1.0/whoami"
    Then the response status code should be 200
    And the response header "X-Both-URL-Policy" should be "applied"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-both-url-policy-v1.0"
    Then the response status code should be 200

  # Simulates the common operational flow of first deploying to production only,
  # then later enabling sandbox by re-deploying with a sandbox upstream added.
  Scenario: Sandbox routing becomes available after redeploying API with sandbox upstream added
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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
            url: http://testbench:3000
        operations:
          - method: GET
            path: /whoami
      """
    Then the response status code should be 201
    And I set request host to "redeploy-main.local"
    And I send a "GET" request to "/env-redeploy-sandbox/v1.0/whoami" until status 200
    And the response should be valid JSON
    And the JSON response field "path" should be "/whoami"

    When I reset the request
    And I set request host to "redeploy-sandbox.local"
    And I send a "GET" request to "/env-redeploy-sandbox/v1.0/whoami"
    Then the response status code should be 404

    Given I authenticate using basic auth as "admin"
    When I update the API "env-routing-redeploy-sandbox-v1.0" with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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
            url: http://testbench:3000
          sandbox:
            url: http://testbench:3000/sandbox
        operations:
          - method: GET
            path: /whoami
      """
    Then the response status code should be 200
    And I set request host to "redeploy-sandbox.local"
    And I send a "GET" request to "/env-redeploy-sandbox/v1.0/whoami" until status 200
    And the response should be valid JSON
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-redeploy-sandbox-v1.0"
    Then the response status code should be 200

  # When vhosts.main is customised but vhosts.sandbox is omitted, the sandbox upstream
  # is still reachable via the global default sandbox vhost (sandbox-*).
  Scenario: Sandbox routes use global default vhost when vhosts.sandbox is omitted
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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
            url: http://testbench:3000
          sandbox:
            url: http://testbench:3000/sandbox
        operations:
          - method: GET
            path: /whoami
      """
    Then the response status code should be 201
    And I set request host to "sb-vhost-fallback-main.local"
    And I send a "GET" request to "/env-sb-vhost-fallback/v1.0/whoami" until status 200
    And the JSON response field "path" should be "/whoami"

    # sandbox-sb-vhost-fallback.local matches sandbox-* (global default) — sandbox upstream reachable
    And I set request host to "sandbox-sb-vhost-fallback.local"
    And I send a "GET" request to "/env-sb-vhost-fallback/v1.0/whoami" until status 200
    And the JSON response field "environment" should be "sandbox"
    And the JSON response field "path" should be "/sandbox/whoami"

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-sandbox-vhost-fallback-v1.0"
    Then the response status code should be 200

  # Stronger isolation check than the existing "no-sandbox" scenario:
  # uses a host that actually matches the global sandbox-* pattern, confirming
  # that a main-only API registers no routes under the real sandbox virtual host.
  Scenario: Requests to global default sandbox vhost are rejected when API has no sandbox upstream
    Given I authenticate using basic auth as "admin"
    When I create API with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
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
            url: http://testbench:3000
        operations:
          - method: GET
            path: /whoami
      """
    Then the response status code should be 201
    And I set request host to "no-sb-global-main.local"
    And I send a "GET" request to "/env-no-sb-global/v1.0/whoami" until status 200
    And the JSON response field "path" should be "/whoami"

    # sandbox-no-sb-global.local matches sandbox-* (global default sandbox vhost).
    # Because this API has no sandbox upstream, no routes exist there → 404.
    When I reset the request
    And I set request host to "sandbox-no-sb-global.local"
    And I send a "GET" request to "/env-no-sb-global/v1.0/whoami"
    Then the response status code should be 404

    Given I authenticate using basic auth as "admin"
    When I delete the API "env-routing-no-sandbox-global-vhost-v1.0"
    Then the response status code should be 200
