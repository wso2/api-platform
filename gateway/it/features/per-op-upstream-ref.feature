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

@per-op-upstream-ref
Feature: Per-Operation Upstream Ref
  As an API developer
  I want per-operation upstream refs to resolve through upstreamDefinitions
  So that different operations can route to different backends

  Background:
    Given the gateway services are running

  Scenario: Per-operation main refs route to different backend services on different ports
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: per-op-ref-api-v1.0
      spec:
        displayName: Per-Op-Ref-API
        version: v1.0
        context: /per-op/$version
        vhosts:
          main: per-op-main.local
        upstreamDefinitions:
          - name: users-svc
            upstreams:
              - url: http://sample-backend:9080
            basePath: /user-svc
          - name: orders-svc
            upstreams:
              - url: http://echo-backend:80
            basePath: /anything
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /users
            upstream:
              main:
                ref: users-svc
          - method: GET
            path: /orders
            upstream:
              main:
                ref: orders-svc
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/per-op/v1.0/users" to be ready with host "per-op-main.local"
    And I wait for the endpoint "http://localhost:8080/per-op/v1.0/orders" to be ready with host "per-op-main.local"

    When I clear all headers
    And I set request host to "per-op-main.local"
    And I send a GET request to "http://localhost:8080/per-op/v1.0/users"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/user-svc/users"

    When I clear all headers
    And I set request host to "per-op-main.local"
    And I send a GET request to "http://localhost:8080/per-op/v1.0/orders"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "url" should be "http://echo-backend/anything/orders"

    Given I authenticate using basic auth as "admin"
    When I delete the API "per-op-ref-api-v1.0"
    Then the response should be successful

  Scenario: Mixed operations - one with per-op ref, one falling back to API-level
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: per-op-mixed-api-v1.0
      spec:
        displayName: Per-Op-Mixed-API
        version: v1.0
        context: /per-op-mixed/$version
        vhosts:
          main: per-op-mixed-main.local
        upstreamDefinitions:
          - name: users-svc
            upstreams:
              - url: http://sample-backend:9080
            basePath: /user-svc
        upstream:
          main:
            url: http://sample-backend:9080/api-main
        operations:
          - method: GET
            path: /users
            upstream:
              main:
                ref: users-svc
          - method: GET
            path: /orders
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/per-op-mixed/v1.0/users" to be ready with host "per-op-mixed-main.local"

    When I clear all headers
    And I set request host to "per-op-mixed-main.local"
    And I send a GET request to "http://localhost:8080/per-op-mixed/v1.0/users"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/user-svc/users"

    When I clear all headers
    And I set request host to "per-op-mixed-main.local"
    And I send a GET request to "http://localhost:8080/per-op-mixed/v1.0/orders"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/api-main/orders"

    Given I authenticate using basic auth as "admin"
    When I delete the API "per-op-mixed-api-v1.0"
    Then the response should be successful

  Scenario: Operation-level dynamic-endpoint policy overrides the per-op ref
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: per-op-prec-op-api-v1.0
      spec:
        displayName: Per-Op-Prec-Op-API
        version: v1.0
        context: /per-op-prec-op/$version
        vhosts:
          main: per-op-prec-op-main.local
        upstreamDefinitions:
          - name: ref-svc
            upstreams:
              - url: http://sample-backend:9080
            basePath: /ref-svc
          - name: op-policy-svc
            upstreams:
              - url: http://sample-backend:9080
            basePath: /op-policy-svc
        upstream:
          main:
            url: http://sample-backend:9080/api-main
        operations:
          - method: GET
            path: /override
            upstream:
              main:
                ref: ref-svc
            policies:
              - name: dynamic-endpoint
                version: v1
                params:
                  targetUpstream: op-policy-svc
          - method: GET
            path: /fallback
            upstream:
              main:
                ref: ref-svc
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/per-op-prec-op/v1.0/fallback" to be ready with host "per-op-prec-op-main.local"

    # Operation-level dynamic-endpoint policy wins over the per-op ref.
    When I clear all headers
    And I set request host to "per-op-prec-op-main.local"
    And I send a GET request to "http://localhost:8080/per-op-prec-op/v1.0/override"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/op-policy-svc/override"

    # No policy on this op: the per-op ref is the default.
    When I clear all headers
    And I set request host to "per-op-prec-op-main.local"
    And I send a GET request to "http://localhost:8080/per-op-prec-op/v1.0/fallback"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/ref-svc/fallback"

    Given I authenticate using basic auth as "admin"
    When I delete the API "per-op-prec-op-api-v1.0"
    Then the response should be successful

  Scenario: API-level dynamic-endpoint policy overrides the per-op ref
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: per-op-prec-api-api-v1.0
      spec:
        displayName: Per-Op-Prec-Api-API
        version: v1.0
        context: /per-op-prec-api/$version
        vhosts:
          main: per-op-prec-api-main.local
        upstreamDefinitions:
          - name: ref-svc
            upstreams:
              - url: http://sample-backend:9080
            basePath: /ref-svc
          - name: global-policy-svc
            upstreams:
              - url: http://sample-backend:9080
            basePath: /global-policy-svc
        upstream:
          main:
            url: http://sample-backend:9080/api-main
        policies:
          - name: dynamic-endpoint
            version: v1
            params:
              targetUpstream: global-policy-svc
        operations:
          - method: GET
            path: /items
            upstream:
              main:
                ref: ref-svc
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/per-op-prec-api/v1.0/items" to be ready with host "per-op-prec-api-main.local"

    # API-level dynamic-endpoint policy wins over the per-op ref (dynamic beats static upstream).
    When I clear all headers
    And I set request host to "per-op-prec-api-main.local"
    And I send a GET request to "http://localhost:8080/per-op-prec-api/v1.0/items"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/global-policy-svc/items"

    Given I authenticate using basic auth as "admin"
    When I delete the API "per-op-prec-api-api-v1.0"
    Then the response should be successful

  Scenario: Request-rewrite policy composes with a per-op ref
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: per-op-ref-rewrite-api-v1.0
      spec:
        displayName: Per-Op-Ref-Rewrite-API
        version: v1.0
        context: /per-op-ref-rewrite/$version
        vhosts:
          main: per-op-ref-rewrite-main.local
        upstreamDefinitions:
          - name: ref-svc
            upstreams:
              - url: http://sample-backend:9080
            basePath: /ref-svc
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /whoami
            upstream:
              main:
                ref: ref-svc
            policies:
              - name: request-rewrite
                version: v1
                params:
                  pathRewrite:
                    type: ReplaceFullPath
                    replaceFullPath: /rewritten
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/per-op-ref-rewrite/v1.0/whoami" to be ready with host "per-op-ref-rewrite-main.local"

    When I clear all headers
    And I set request host to "per-op-ref-rewrite-main.local"
    And I send a GET request to "http://localhost:8080/per-op-ref-rewrite/v1.0/whoami"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/ref-svc/rewritten"

    Given I authenticate using basic auth as "admin"
    When I delete the API "per-op-ref-rewrite-api-v1.0"
    Then the response should be successful
