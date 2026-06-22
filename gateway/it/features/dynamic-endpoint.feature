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

@dynamic-endpoint
Feature: Dynamic Endpoint policy
  As an API developer
  I want the dynamic-endpoint policy to route an operation to a named upstream definition
  So that specific operations can target alternate upstreams without changing the primary API structure

  Background:
    Given the gateway services are running

  # The policy sets the SDK UpstreamName field, diverting the request from the default
  # upstream.main to the named upstream definition. Operations without the policy keep
  # using upstream.main.
  Scenario: Operation routed to a named upstream definition while others use the default upstream
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha2
      kind: RestApi
      metadata:
        name: dynamic-endpoint-api-v1.0
      spec:
        displayName: Dynamic-Endpoint-API
        version: v1.0
        context: /dynamic-endpoint/$version
        upstreamDefinitions:
          - name: alt-upstream
            basePath: /alternate
            upstreams:
              - url: http://sample-backend:9080
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /whoami
            policies:
              - name: dynamic-endpoint
                version: v1
                params:
                  targetUpstream: alt-upstream
          - method: GET
            path: /ping
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/dynamic-endpoint/v1.0/ping" to be ready

    # Operation with the policy: diverted to alt-upstream. The backend echoes the
    # path it received — the alt-upstream base path /alternate confirms the routing.
    When I clear all headers
    And I send a GET request to "http://localhost:8080/dynamic-endpoint/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/alternate/whoami"

    # Operation without the policy: served by the default upstream.main (base path /).
    When I clear all headers
    And I send a GET request to "http://localhost:8080/dynamic-endpoint/v1.0/ping"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/ping"

    Given I authenticate using basic auth as "admin"
    When I delete the API "dynamic-endpoint-api-v1.0"
    Then the response should be successful

  # Each upstream definition carries a distinct base path. The policy must route each
  # operation to its targeted upstream AND the upstream's base path must be prepended
  # to the forwarded request path.
  Scenario: Different operations route to upstreams with different base paths
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha2
      kind: RestApi
      metadata:
        name: dynamic-endpoint-routes-v1.0
      spec:
        displayName: Dynamic-Endpoint-Routes-API
        version: v1.0
        context: /dynamic-endpoint-routes/$version
        upstreamDefinitions:
          - name: foo-upstream
            basePath: /foo
            upstreams:
              - url: http://sample-backend:9080
          - name: bar-upstream
            basePath: /bar
            upstreams:
              - url: http://sample-backend:9080
          - name: root-upstream
            upstreams:
              - url: http://sample-backend:9080
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /items
            policies:
              - name: dynamic-endpoint
                version: v1
                params:
                  targetUpstream: foo-upstream
          - method: GET
            path: /records
            policies:
              - name: dynamic-endpoint
                version: v1
                params:
                  targetUpstream: bar-upstream
          - method: GET
            path: /extras
            policies:
              - name: dynamic-endpoint
                version: v1
                params:
                  targetUpstream: root-upstream
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/dynamic-endpoint-routes/v1.0/items" to be ready

    # Routed to foo-upstream: base path /foo prepended.
    When I clear all headers
    And I send a GET request to "http://localhost:8080/dynamic-endpoint-routes/v1.0/items"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/foo/items"

    # Routed to bar-upstream: base path /bar prepended.
    When I clear all headers
    And I send a GET request to "http://localhost:8080/dynamic-endpoint-routes/v1.0/records"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/bar/records"

    # Routed to root-upstream: empty base path, so only the operation path reaches the backend.
    When I clear all headers
    And I send a GET request to "http://localhost:8080/dynamic-endpoint-routes/v1.0/extras"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/extras"

    Given I authenticate using basic auth as "admin"
    When I delete the API "dynamic-endpoint-routes-v1.0"
    Then the response should be successful

  # targetUpstream is a required parameter in the policy definition.
  Scenario: Deploy fails when targetUpstream is omitted
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha2
      kind: RestApi
      metadata:
        name: dynamic-endpoint-missing-param-v1.0
      spec:
        displayName: Dynamic-Endpoint-Missing-Param-API
        version: v1.0
        context: /dynamic-endpoint-missing/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /whoami
            policies:
              - name: dynamic-endpoint
                version: v1
                params: {}
      """
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "targetUpstream"

  # Regression: a dynamic-endpoint policy on the SANDBOX vhost must route to the target
  # upstream (base /alternate), not prepend the sandbox base path (/sandbox/alternate/whoami).
  Scenario: Dynamic-endpoint policy applies on the sandbox vhost
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha2
      kind: RestApi
      metadata:
        name: dynamic-endpoint-sandbox-v1.0
      spec:
        displayName: Dynamic-Endpoint-Sandbox-API
        version: v1.0
        context: /dynamic-endpoint-sandbox/$version
        vhosts:
          main: dyn-sb-main.local
          sandbox: dyn-sb-sandbox.local
        upstreamDefinitions:
          - name: alt-upstream
            basePath: /alternate
            upstreams:
              - url: http://sample-backend:9080
        upstream:
          main:
            url: http://sample-backend:9080
          sandbox:
            url: http://sample-backend:9080/sandbox
        operations:
          - method: GET
            path: /whoami
            policies:
              - name: dynamic-endpoint
                version: v1
                params:
                  targetUpstream: alt-upstream
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/dynamic-endpoint-sandbox/v1.0/whoami" to be ready with host "dyn-sb-main.local"

    # Main vhost: policy diverts to alt-upstream (base path /alternate).
    When I clear all headers
    And I set request host to "dyn-sb-main.local"
    And I send a GET request to "http://localhost:8080/dynamic-endpoint-sandbox/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/alternate/whoami"

    # Sandbox vhost: the same policy must divert to alt-upstream (base /alternate), not the static sandbox upstream.
    When I clear all headers
    And I set request host to "dyn-sb-sandbox.local"
    And I send a GET request to "http://localhost:8080/dynamic-endpoint-sandbox/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/alternate/whoami"

    Given I authenticate using basic auth as "admin"
    When I delete the API "dynamic-endpoint-sandbox-v1.0"
    Then the response should be successful

  # An API-level dynamic-endpoint (under spec.policies) applies to every operation on both
  # vhosts. On the sandbox vhost it must still divert to the target upstream (base /alternate),
  # not prepend the sandbox base path.
  Scenario: API-level dynamic-endpoint policy applies on the sandbox vhost
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha2
      kind: RestApi
      metadata:
        name: dynamic-endpoint-api-sandbox-v1.0
      spec:
        displayName: Dynamic-Endpoint-API-Sandbox-API
        version: v1.0
        context: /dynamic-endpoint-api-sandbox/$version
        vhosts:
          main: dyn-api-sb-main.local
          sandbox: dyn-api-sb-sandbox.local
        upstreamDefinitions:
          - name: alt-upstream
            basePath: /alternate
            upstreams:
              - url: http://sample-backend:9080
        upstream:
          main:
            url: http://sample-backend:9080
          sandbox:
            url: http://sample-backend:9080/sandbox
        policies:
          - name: dynamic-endpoint
            version: v1
            params:
              targetUpstream: alt-upstream
        operations:
          - method: GET
            path: /whoami
          - method: GET
            path: /ping
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/dynamic-endpoint-api-sandbox/v1.0/whoami" to be ready with host "dyn-api-sb-main.local"

    # Main vhost: the API-level policy diverts every operation to alt-upstream (base /alternate).
    When I clear all headers
    And I set request host to "dyn-api-sb-main.local"
    And I send a GET request to "http://localhost:8080/dynamic-endpoint-api-sandbox/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/alternate/whoami"

    # Sandbox vhost: the same API-level policy must divert to alt-upstream (base /alternate), not the static sandbox upstream.
    When I clear all headers
    And I set request host to "dyn-api-sb-sandbox.local"
    And I send a GET request to "http://localhost:8080/dynamic-endpoint-api-sandbox/v1.0/ping"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/alternate/ping"

    Given I authenticate using basic auth as "admin"
    When I delete the API "dynamic-endpoint-api-sandbox-v1.0"
    Then the response should be successful

  # Adding cluster_header routing to sandbox routes must not break operations WITHOUT a
  # dynamic-endpoint policy: those fall back to the sandbox upstream (base /sandbox).
  Scenario: Sandbox operation without the policy falls back to the sandbox upstream
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha2
      kind: RestApi
      metadata:
        name: dynamic-endpoint-sandbox-mixed-v1.0
      spec:
        displayName: Dynamic-Endpoint-Sandbox-Mixed-API
        version: v1.0
        context: /dynamic-endpoint-sandbox-mixed/$version
        vhosts:
          main: dyn-mixed-main.local
          sandbox: dyn-mixed-sandbox.local
        upstreamDefinitions:
          - name: alt-upstream
            basePath: /alternate
            upstreams:
              - url: http://sample-backend:9080
        upstream:
          main:
            url: http://sample-backend:9080
          sandbox:
            url: http://sample-backend:9080/sandbox
        operations:
          - method: GET
            path: /whoami
            policies:
              - name: dynamic-endpoint
                version: v1
                params:
                  targetUpstream: alt-upstream
          - method: GET
            path: /ping
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/dynamic-endpoint-sandbox-mixed/v1.0/ping" to be ready with host "dyn-mixed-sandbox.local"

    # Sandbox vhost, operation WITH the policy: diverted to alt-upstream (base /alternate).
    When I clear all headers
    And I set request host to "dyn-mixed-sandbox.local"
    And I send a GET request to "http://localhost:8080/dynamic-endpoint-sandbox-mixed/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/alternate/whoami"

    # Sandbox vhost, operation WITHOUT the policy: falls back to the sandbox upstream (base /sandbox).
    When I clear all headers
    And I set request host to "dyn-mixed-sandbox.local"
    And I send a GET request to "http://localhost:8080/dynamic-endpoint-sandbox-mixed/v1.0/ping"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/sandbox/ping"

    Given I authenticate using basic auth as "admin"
    When I delete the API "dynamic-endpoint-sandbox-mixed-v1.0"
    Then the response should be successful

  # A path-changing policy (request-rewrite) and dynamic-endpoint on the SAME operation must
  # compose: the rewrite sets mutations.Path -> metadata["path"], and the dynamic-endpoint
  # handler must NOT clobber it. The Lua filter prepends the target upstream base (/alternate)
  # exactly once, so the rewrite AND the routing both apply -> /alternate/rewritten.
  Scenario: Dynamic-endpoint combined with a path-rewrite policy on the same operation
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha2
      kind: RestApi
      metadata:
        name: dynamic-endpoint-rewrite-v1.0
      spec:
        displayName: Dynamic-Endpoint-Rewrite-API
        version: v1.0
        context: /dynamic-endpoint-rewrite/$version
        upstreamDefinitions:
          - name: alt-upstream
            basePath: /alternate
            upstreams:
              - url: http://sample-backend:9080
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /whoami
            policies:
              - name: request-rewrite
                version: v1
                params:
                  pathRewrite:
                    type: ReplaceFullPath
                    replaceFullPath: /rewritten
              - name: dynamic-endpoint
                version: v1
                params:
                  targetUpstream: alt-upstream
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/dynamic-endpoint-rewrite/v1.0/whoami" to be ready

    # Both the rewrite (/whoami -> /rewritten) and the dynamic-endpoint base (/alternate) apply,
    # prefixed exactly once.
    When I clear all headers
    And I send a GET request to "http://localhost:8080/dynamic-endpoint-rewrite/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/alternate/rewritten"

    Given I authenticate using basic auth as "admin"
    When I delete the API "dynamic-endpoint-rewrite-v1.0"
    Then the response should be successful

  # The same request-rewrite + dynamic-endpoint combination on the SANDBOX vhost: the sandbox
  # base (/sandbox) must NOT leak in. The request diverts to the target upstream (/alternate)
  # with the rewritten path, prefixed exactly once -> /alternate/rewritten.
  Scenario: Dynamic-endpoint and a path-rewrite policy combined on the sandbox vhost
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha2
      kind: RestApi
      metadata:
        name: dynamic-endpoint-rewrite-sandbox-v1.0
      spec:
        displayName: Dynamic-Endpoint-Rewrite-Sandbox-API
        version: v1.0
        context: /dynamic-endpoint-rewrite-sandbox/$version
        vhosts:
          main: dyn-rw-sb-main.local
          sandbox: dyn-rw-sb-sandbox.local
        upstreamDefinitions:
          - name: alt-upstream
            basePath: /alternate
            upstreams:
              - url: http://sample-backend:9080
        upstream:
          main:
            url: http://sample-backend:9080
          sandbox:
            url: http://sample-backend:9080/sandbox
        operations:
          - method: GET
            path: /whoami
            policies:
              - name: request-rewrite
                version: v1
                params:
                  pathRewrite:
                    type: ReplaceFullPath
                    replaceFullPath: /rewritten
              - name: dynamic-endpoint
                version: v1
                params:
                  targetUpstream: alt-upstream
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/dynamic-endpoint-rewrite-sandbox/v1.0/whoami" to be ready with host "dyn-rw-sb-sandbox.local"

    # Sandbox vhost: rewrite under the target upstream base (/alternate), prefixed once.
    # The sandbox base (/sandbox) must not appear.
    When I clear all headers
    And I set request host to "dyn-rw-sb-sandbox.local"
    And I send a GET request to "http://localhost:8080/dynamic-endpoint-rewrite-sandbox/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/alternate/rewritten"

    Given I authenticate using basic auth as "admin"
    When I delete the API "dynamic-endpoint-rewrite-sandbox-v1.0"
    Then the response should be successful
