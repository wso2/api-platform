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
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: dynamic-endpoint-api-v1.0
      spec:
        displayName: Dynamic-Endpoint-API
        version: v1.0
        context: /dynamic-endpoint/$version
        upstreamDefinitions:
          - name: alt-upstream
            upstreams:
              - url: http://sample-backend:9080/alternate
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
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: dynamic-endpoint-routes-v1.0
      spec:
        displayName: Dynamic-Endpoint-Routes-API
        version: v1.0
        context: /dynamic-endpoint-routes/$version
        upstreamDefinitions:
          - name: foo-upstream
            upstreams:
              - url: http://sample-backend:9080/foo
          - name: bar-upstream
            upstreams:
              - url: http://sample-backend:9080/bar
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
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
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
