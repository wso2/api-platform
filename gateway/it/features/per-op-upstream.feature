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

@per-op-upstream
Feature: Per-Operation Upstream
  As an API developer
  I want per-operation upstream refs to override the API-level upstream, with API-level
  URL edits staying cluster-stable
  So that different operations can route to different backends without disruptive redeploys

  Background:
    Given the gateway services are running

  # ===== from per-op-upstream-basic.feature =====
  Scenario: API-level main fallback when operation has no per-op upstream
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: per-op-basic-fm-api-v1.0
      spec:
        displayName: Per-Op-Basic-FM-API
        version: v1.0
        context: /per-op-basic-fm/$version
        vhosts:
          main: per-op-basic-fm-main.local
          sandbox: per-op-basic-fm-sandbox.local
        upstream:
          main:
            url: http://sample-backend:9080/api-main
          sandbox:
            url: http://sample-backend:9080/api-sandbox
        operations:
          - method: GET
            path: /users
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/per-op-basic-fm/v1.0/users" to be ready with host "per-op-basic-fm-main.local"

    When I clear all headers
    And I set request host to "per-op-basic-fm-main.local"
    And I send a GET request to "http://localhost:8080/per-op-basic-fm/v1.0/users"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/api-main/users"

    Given I authenticate using basic auth as "admin"
    When I delete the API "per-op-basic-fm-api-v1.0"
    Then the response should be successful

  Scenario: API-level sandbox fallback when operation has no per-op upstream
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: per-op-basic-fs-api-v1.0
      spec:
        displayName: Per-Op-Basic-FS-API
        version: v1.0
        context: /per-op-basic-fs/$version
        vhosts:
          main: per-op-basic-fs-main.local
          sandbox: per-op-basic-fs-sandbox.local
        upstream:
          main:
            url: http://sample-backend:9080/api-main
          sandbox:
            url: http://sample-backend:9080/api-sandbox
        operations:
          - method: GET
            path: /users
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/per-op-basic-fs/v1.0/users" to be ready with host "per-op-basic-fs-sandbox.local"

    When I clear all headers
    And I set request host to "per-op-basic-fs-sandbox.local"
    And I send a GET request to "http://localhost:8080/per-op-basic-fs/v1.0/users"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/api-sandbox/users"

    Given I authenticate using basic auth as "admin"
    When I delete the API "per-op-basic-fs-api-v1.0"
    Then the response should be successful

  Scenario: Per-operation main ref overrides API-level main
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: per-op-basic-om-api-v1.0
      spec:
        displayName: Per-Op-Basic-OM-API
        version: v1.0
        context: /per-op-basic-om/$version
        vhosts:
          main: per-op-basic-om-main.local
        upstreamDefinitions:
          - name: op-main-svc
            upstreams:
              - url: http://sample-backend:9080
            basePath: /op-main
        upstream:
          main:
            url: http://sample-backend:9080/api-main
        operations:
          - method: GET
            path: /users
            upstream:
              main:
                ref: op-main-svc
          - method: GET
            path: /orders
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/per-op-basic-om/v1.0/users" to be ready with host "per-op-basic-om-main.local"

    When I clear all headers
    And I set request host to "per-op-basic-om-main.local"
    And I send a GET request to "http://localhost:8080/per-op-basic-om/v1.0/users"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/op-main/users"

    When I clear all headers
    And I set request host to "per-op-basic-om-main.local"
    And I send a GET request to "http://localhost:8080/per-op-basic-om/v1.0/orders"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/api-main/orders"

    Given I authenticate using basic auth as "admin"
    When I delete the API "per-op-basic-om-api-v1.0"
    Then the response should be successful

  Scenario: Per-operation sandbox-only override routes sandbox traffic to the operation upstream
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: per-op-basic-os-api-v1.0
      spec:
        displayName: Per-Op-Basic-OS-API
        version: v1.0
        context: /per-op-basic-os/$version
        vhosts:
          main: per-op-basic-os-main.local
          sandbox: per-op-basic-os-sandbox.local
        upstreamDefinitions:
          - name: op-sandbox-svc
            upstreams:
              - url: http://sample-backend:9080
            basePath: /op-sandbox
        upstream:
          main:
            url: http://sample-backend:9080/api-main
        operations:
          - method: GET
            path: /users
            upstream:
              sandbox:
                ref: op-sandbox-svc
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/per-op-basic-os/v1.0/users" to be ready with host "per-op-basic-os-sandbox.local"

    When I clear all headers
    And I set request host to "per-op-basic-os-sandbox.local"
    And I send a GET request to "http://localhost:8080/per-op-basic-os/v1.0/users"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/op-sandbox/users"

    Given I authenticate using basic auth as "admin"
    When I delete the API "per-op-basic-os-api-v1.0"
    Then the response should be successful

  Scenario: Sandbox falls back when operation only has per-op main
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: per-op-basic-sf-api-v1.0
      spec:
        displayName: Per-Op-Basic-SF-API
        version: v1.0
        context: /per-op-basic-sf/$version
        vhosts:
          main: per-op-basic-sf-main.local
          sandbox: per-op-basic-sf-sandbox.local
        upstreamDefinitions:
          - name: op-main-svc
            upstreams:
              - url: http://sample-backend:9080
            basePath: /op-main
        upstream:
          main:
            url: http://sample-backend:9080/api-main
          sandbox:
            url: http://sample-backend:9080/api-sandbox
        operations:
          - method: GET
            path: /users
            upstream:
              main:
                ref: op-main-svc
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/per-op-basic-sf/v1.0/users" to be ready with host "per-op-basic-sf-sandbox.local"

    When I clear all headers
    And I set request host to "per-op-basic-sf-sandbox.local"
    And I send a GET request to "http://localhost:8080/per-op-basic-sf/v1.0/users"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/api-sandbox/users"

    Given I authenticate using basic auth as "admin"
    When I delete the API "per-op-basic-sf-api-v1.0"
    Then the response should be successful

  Scenario: Main falls back when operation only has per-op sandbox
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: per-op-basic-mf-api-v1.0
      spec:
        displayName: Per-Op-Basic-MF-API
        version: v1.0
        context: /per-op-basic-mf/$version
        vhosts:
          main: per-op-basic-mf-main.local
          sandbox: per-op-basic-mf-sandbox.local
        upstreamDefinitions:
          - name: op-sandbox-svc
            upstreams:
              - url: http://sample-backend:9080
            basePath: /op-sandbox
        upstream:
          main:
            url: http://sample-backend:9080/api-main
          sandbox:
            url: http://sample-backend:9080/api-sandbox
        operations:
          - method: GET
            path: /users
            upstream:
              sandbox:
                ref: op-sandbox-svc
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/per-op-basic-mf/v1.0/users" to be ready with host "per-op-basic-mf-main.local"

    When I clear all headers
    And I set request host to "per-op-basic-mf-main.local"
    And I send a GET request to "http://localhost:8080/per-op-basic-mf/v1.0/users"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/api-main/users"

    Given I authenticate using basic auth as "admin"
    When I delete the API "per-op-basic-mf-api-v1.0"
    Then the response should be successful

  Scenario: Operation with both per-op main and sandbox overrides
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: per-op-basic-both-api-v1.0
      spec:
        displayName: Per-Op-Basic-Both-API
        version: v1.0
        context: /per-op-basic-both/$version
        vhosts:
          main: per-op-basic-both-main.local
          sandbox: per-op-basic-both-sandbox.local
        upstreamDefinitions:
          - name: op-main-svc
            upstreams:
              - url: http://sample-backend:9080
            basePath: /op-main
          - name: op-sandbox-svc
            upstreams:
              - url: http://sample-backend:9080
            basePath: /op-sandbox
        upstream:
          main:
            url: http://sample-backend:9080/api-main
          sandbox:
            url: http://sample-backend:9080/api-sandbox
        operations:
          - method: GET
            path: /users
            upstream:
              main:
                ref: op-main-svc
              sandbox:
                ref: op-sandbox-svc
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/per-op-basic-both/v1.0/users" to be ready with host "per-op-basic-both-main.local"

    When I clear all headers
    And I set request host to "per-op-basic-both-main.local"
    And I send a GET request to "http://localhost:8080/per-op-basic-both/v1.0/users"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/op-main/users"

    When I clear all headers
    And I set request host to "per-op-basic-both-sandbox.local"
    And I send a GET request to "http://localhost:8080/per-op-basic-both/v1.0/users"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/op-sandbox/users"

    Given I authenticate using basic auth as "admin"
    When I delete the API "per-op-basic-both-api-v1.0"
    Then the response should be successful

  Scenario: Per-operation upstream definition basePath update routes to the new path
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: per-op-basic-eds-api-v1.0
      spec:
        displayName: Per-Op-Basic-EDS-API
        version: v1.0
        context: /per-op-basic-eds/$version
        vhosts:
          main: per-op-basic-eds-main.local
        upstreamDefinitions:
          - name: op-versioned-svc
            upstreams:
              - url: http://sample-backend:9080
            basePath: /version-a
        upstream:
          main:
            url: http://sample-backend:9080/api-main
        operations:
          - method: GET
            path: /endpoint
            upstream:
              main:
                ref: op-versioned-svc
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/per-op-basic-eds/v1.0/endpoint" to be ready with host "per-op-basic-eds-main.local"

    When I clear all headers
    And I set request host to "per-op-basic-eds-main.local"
    And I send a GET request to "http://localhost:8080/per-op-basic-eds/v1.0/endpoint"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/version-a/endpoint"

    Given I authenticate using basic auth as "admin"
    When I update the API "per-op-basic-eds-api-v1.0" with this configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: per-op-basic-eds-api-v1.0
      spec:
        displayName: Per-Op-Basic-EDS-API
        version: v1.0
        context: /per-op-basic-eds/$version
        vhosts:
          main: per-op-basic-eds-main.local
        upstreamDefinitions:
          - name: op-versioned-svc
            upstreams:
              - url: http://sample-backend:9080
            basePath: /version-b
        upstream:
          main:
            url: http://sample-backend:9080/api-main
        operations:
          - method: GET
            path: /endpoint
            upstream:
              main:
                ref: op-versioned-svc
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/per-op-basic-eds/v1.0/endpoint" to be ready with host "per-op-basic-eds-main.local"

    When I clear all headers
    And I set request host to "per-op-basic-eds-main.local"
    And I send a GET request to "http://localhost:8080/per-op-basic-eds/v1.0/endpoint"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/version-b/endpoint"

    Given I authenticate using basic auth as "admin"
    When I delete the API "per-op-basic-eds-api-v1.0"
    Then the response should be successful

  Scenario: Per-op sandbox inherits API-level main hostRewrite when no API-level sandbox
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: per-op-basic-sbhr-api-v1.0
      spec:
        displayName: Per-Op-Basic-SBHR-API
        version: v1.0
        context: /per-op-basic-sbhr/$version
        vhosts:
          main: per-op-basic-sbhr-main.local
          sandbox: per-op-basic-sbhr-sandbox.local
        upstreamDefinitions:
          - name: op-sandbox-svc
            upstreams:
              - url: http://echo-backend:80
            basePath: /anything
        upstream:
          main:
            url: http://echo-backend:80/anything
            hostRewrite: manual
        operations:
          - method: GET
            path: /test
            upstream:
              sandbox:
                ref: op-sandbox-svc
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/per-op-basic-sbhr/v1.0/test" to be ready with host "per-op-basic-sbhr-sandbox.local"

    When I clear all headers
    And I set request host to "per-op-basic-sbhr-sandbox.local"
    And I send a GET request to "http://localhost:8080/per-op-basic-sbhr/v1.0/test"
    Then the response status code should be 200
    And the JSON response field "headers.Host" should be "per-op-basic-sbhr-sandbox.local"

    Given I authenticate using basic auth as "admin"
    When I delete the API "per-op-basic-sbhr-api-v1.0"
    Then the response should be successful

  # ===== from per-op-upstream-ref.feature =====
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

  # ===== from per-op-upstream-validation.feature =====
  Scenario: Empty per-op upstream wrapper is rejected
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: per-op-val-empty-api-v1.0
      spec:
        displayName: Per-Op-Val-Empty-API
        version: v1.0
        context: /per-op-val-empty/$version
        vhosts:
          main: per-op-val-empty-main.local
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /users
            upstream: {}
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "At least one of 'main' or 'sandbox' must be set"

  Scenario: Per-op ref to non-existent upstream definition is rejected
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: per-op-val-missing-ref-api-v1.0
      spec:
        displayName: Per-Op-Val-Missing-Ref-API
        version: v1.0
        context: /per-op-val-missing-ref/$version
        vhosts:
          main: per-op-val-missing-ref-main.local
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /users
            upstream:
              main:
                ref: does-not-exist
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Referenced upstream definition 'does-not-exist' not found"

  Scenario: Empty per-op leaf is rejected
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: per-op-val-empty-leaf-api-v1.0
      spec:
        displayName: Per-Op-Val-Empty-Leaf-API
        version: v1.0
        context: /per-op-val-empty-leaf/$version
        vhosts:
          main: per-op-val-empty-leaf-main.local
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /users
            upstream:
              main: {}
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Upstream ref is required"

  Scenario: Empty per-op sandbox leaf with no API-level sandbox is rejected
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: per-op-val-empty-sandbox-api-v1.0
      spec:
        displayName: Per-Op-Val-Empty-Sandbox-API
        version: v1.0
        context: /per-op-val-empty-sandbox/$version
        vhosts:
          main: per-op-val-empty-sandbox-main.local
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /users
            upstream:
              sandbox: {}
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Upstream ref is required"

  Scenario: Per-op ref with invalid characters is rejected
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: per-op-val-bad-ref-api-v1.0
      spec:
        displayName: Per-Op-Val-Bad-Ref-API
        version: v1.0
        context: /per-op-val-bad-ref/$version
        vhosts:
          main: per-op-val-bad-ref-main.local
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /users
            upstream:
              main:
                ref: "bad/ref!"
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "must match pattern"

  # The management validator deliberately accepts a zero connect timeout; only the
  # duration format is enforced here. Runtime timeout semantics are the translator's
  # concern, so deployment must succeed.
  Scenario: Zero connect timeout in an upstreamDefinition is accepted
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: per-op-val-zero-timeout-api-v1.0
      spec:
        displayName: Per-Op-Val-Zero-Timeout-API
        version: v1.0
        context: /per-op-val-zero-timeout/$version
        vhosts:
          main: per-op-val-zero-timeout-main.local
        upstreamDefinitions:
          - name: slow-svc
            timeout:
              connect: 0s
            upstreams:
              - url: http://sample-backend:9080
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /users
            upstream:
              main:
                ref: slow-svc
      """
    Then the response status code should be 201
    And the response should be valid JSON

  # ===== from api-level-url-stable.feature =====
  Scenario: API-level main upstream URL update (host and path change) routes to new backend (URL-stable cluster naming)
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: api-level-url-stable-main-api-v1.0
      spec:
        displayName: API-Level-URL-Stable-Main-API
        version: v1.0
        context: /api-level-url-stable-main/$version
        vhosts:
          main: api-level-url-stable-main.local
        upstream:
          main:
            url: http://sample-backend:9080/version-a
        operations:
          - method: GET
            path: /endpoint
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/api-level-url-stable-main/v1.0/endpoint" to be ready with host "api-level-url-stable-main.local"

    When I clear all headers
    And I set request host to "api-level-url-stable-main.local"
    And I send a GET request to "http://localhost:8080/api-level-url-stable-main/v1.0/endpoint"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/version-a/endpoint"

    # Envoy admin: the API-level cluster must use the identity-derived name
    # (main_<hash>) and there must be no URL-derived (cluster_<scheme>_<host>)
    # cluster. The URL-derived form is what the pre-change naming produced, so
    # this assertion fails on the old naming scheme. The exact name set is
    # captured so the post-update step can prove the NAME survived the update.
    When I clear all headers
    And I send a GET request to "http://localhost:9901/clusters"
    Then the response should be successful
    And the response body should contain "main_"
    And the response body should not contain "cluster_http_"
    And the response body should not contain "cluster_https_"
    And I capture the Envoy cluster names prefixed "main_"

    Given I authenticate using basic auth as "admin"
    When I update the API "api-level-url-stable-main-api-v1.0" with this configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: api-level-url-stable-main-api-v1.0
      spec:
        displayName: API-Level-URL-Stable-Main-API
        version: v1.0
        context: /api-level-url-stable-main/$version
        vhosts:
          main: api-level-url-stable-main.local
        upstream:
          main:
            # The host changes too (container alias of the same backend), proving
            # the cluster survives a HOST edit, not only a path edit. The old
            # URL-derived naming kept its name across path edits but renamed the
            # cluster on any host or scheme change.
            url: http://it-sample-backend:9080/version-b
        operations:
          - method: GET
            path: /endpoint
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/api-level-url-stable-main/v1.0/endpoint" to be ready with host "api-level-url-stable-main.local"

    When I clear all headers
    And I set request host to "api-level-url-stable-main.local"
    And I send a GET request to "http://localhost:8080/api-level-url-stable-main/v1.0/endpoint"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/version-b/endpoint"

    # After the HOST change the exact cluster-name set must be UNCHANGED:
    # this proves the same main_<hash> cluster survived the host edit (a
    # rename to a different main_<hash> would fail the unchanged step). The
    # old naming would have minted a new cluster_http_it-sample-backend_9080
    # cluster here and dropped the previous one.
    When I clear all headers
    And I send a GET request to "http://localhost:9901/clusters"
    Then the response should be successful
    And the response body should contain "main_"
    And the response body should not contain "cluster_http_"
    And the response body should not contain "cluster_https_"
    And the Envoy cluster names prefixed "main_" should be unchanged

    Given I authenticate using basic auth as "admin"
    When I delete the API "api-level-url-stable-main-api-v1.0"
    Then the response should be successful

  Scenario: API-level sandbox upstream URL update (host and path change) routes to new backend
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: api-level-url-stable-sandbox-api-v1.0
      spec:
        displayName: API-Level-URL-Stable-Sandbox-API
        version: v1.0
        context: /api-level-url-stable-sandbox/$version
        vhosts:
          main: api-level-url-stable-sandbox-main.local
          sandbox: api-level-url-stable-sandbox-sb.local
        upstream:
          main:
            url: http://sample-backend:9080/api-main
          sandbox:
            url: http://sample-backend:9080/sandbox-a
        operations:
          - method: GET
            path: /endpoint
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/api-level-url-stable-sandbox/v1.0/endpoint" to be ready with host "api-level-url-stable-sandbox-sb.local"

    When I clear all headers
    And I set request host to "api-level-url-stable-sandbox-sb.local"
    And I send a GET request to "http://localhost:8080/api-level-url-stable-sandbox/v1.0/endpoint"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/sandbox-a/endpoint"

    # Capture the sandbox cluster-name set so the post-update step can prove
    # the sandbox_<hash> name survived the URL update.
    When I clear all headers
    And I send a GET request to "http://localhost:9901/clusters"
    Then the response should be successful
    And the response body should contain "sandbox_"
    And the response body should not contain "cluster_http_"
    And I capture the Envoy cluster names prefixed "sandbox_"

    Given I authenticate using basic auth as "admin"
    When I update the API "api-level-url-stable-sandbox-api-v1.0" with this configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: api-level-url-stable-sandbox-api-v1.0
      spec:
        displayName: API-Level-URL-Stable-Sandbox-API
        version: v1.0
        context: /api-level-url-stable-sandbox/$version
        vhosts:
          main: api-level-url-stable-sandbox-main.local
          sandbox: api-level-url-stable-sandbox-sb.local
        upstream:
          main:
            url: http://sample-backend:9080/api-main
          sandbox:
            # The sandbox host changes too (container alias of the same
            # backend), so this update exercises a host edit on the sandbox
            # cluster, not only a path edit.
            url: http://it-sample-backend:9080/sandbox-b
        operations:
          - method: GET
            path: /endpoint
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/api-level-url-stable-sandbox/v1.0/endpoint" to be ready with host "api-level-url-stable-sandbox-sb.local"

    When I clear all headers
    And I set request host to "api-level-url-stable-sandbox-sb.local"
    And I send a GET request to "http://localhost:8080/api-level-url-stable-sandbox/v1.0/endpoint"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/sandbox-b/endpoint"

    # Envoy admin: the sandbox cluster must use the identity-derived name
    # (sandbox_<hash>); no URL-derived cluster may exist, and the exact name
    # set must be unchanged across the host edit (identity proof). Fails on
    # the old URL-derived naming scheme.
    When I clear all headers
    And I send a GET request to "http://localhost:9901/clusters"
    Then the response should be successful
    And the response body should contain "sandbox_"
    And the response body should not contain "cluster_http_"
    And the response body should not contain "cluster_https_"
    And the Envoy cluster names prefixed "sandbox_" should be unchanged

    Given I authenticate using basic auth as "admin"
    When I delete the API "api-level-url-stable-sandbox-api-v1.0"
    Then the response should be successful

  Scenario: API-level upstream ref resolves to the referenced upstreamDefinitions entry
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: api-level-url-stable-default-api-v1.0
      spec:
        displayName: API-Level-URL-Stable-Default-API
        version: v1.0
        context: /api-level-url-stable-default/$version
        vhosts:
          main: api-level-url-stable-default.local
        upstreamDefinitions:
          - name: backend-default
            basePath: /api-main
            upstreams:
              - url: http://sample-backend:9080
        upstream:
          main:
            ref: backend-default
        operations:
          - method: GET
            path: /endpoint
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/api-level-url-stable-default/v1.0/endpoint" to be ready with host "api-level-url-stable-default.local"

    When I clear all headers
    And I set request host to "api-level-url-stable-default.local"
    And I send a GET request to "http://localhost:8080/api-level-url-stable-default/v1.0/endpoint"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/api-main/endpoint"

    Given I authenticate using basic auth as "admin"
    When I delete the API "api-level-url-stable-default-api-v1.0"
    Then the response should be successful

  Scenario: API-level main and sandbox on the same backend host get separate identity-named clusters
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: api-level-url-stable-collision-api-v1.0
      spec:
        displayName: API-Level-URL-Stable-Collision-API
        version: v1.0
        context: /api-level-url-stable-collision/$version
        vhosts:
          main: api-level-url-stable-collision-main.local
          sandbox: api-level-url-stable-collision-sb.local
        upstream:
          main:
            url: http://sample-backend:9080/collision-main
          sandbox:
            url: http://sample-backend:9080/collision-sandbox
        operations:
          - method: GET
            path: /endpoint
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/api-level-url-stable-collision/v1.0/endpoint" to be ready with host "api-level-url-stable-collision-main.local"

    # Main and sandbox share the same backend host:port but must route to their
    # own base paths. The old URL-derived naming keyed the cluster on host and
    # scheme only, so main and sandbox collapsed into one shared cluster here;
    # identity naming gives each its own.
    When I clear all headers
    And I set request host to "api-level-url-stable-collision-main.local"
    And I send a GET request to "http://localhost:8080/api-level-url-stable-collision/v1.0/endpoint"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/collision-main/endpoint"

    When I clear all headers
    And I set request host to "api-level-url-stable-collision-sb.local"
    And I send a GET request to "http://localhost:8080/api-level-url-stable-collision/v1.0/endpoint"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/collision-sandbox/endpoint"

    # Envoy admin: an identity-named main_<hash> and a sandbox_<hash> cluster
    # must both exist (they do not collide), and no URL-derived cluster may
    # exist. Under the old naming both upstreams shared one cluster_<scheme>_<host>
    # cluster, so this assertion fails on the previous scheme.
    When I clear all headers
    And I send a GET request to "http://localhost:9901/clusters"
    Then the response should be successful
    And the response body should contain "main_"
    And the response body should contain "sandbox_"
    And the response body should not contain "cluster_http_"
    And the response body should not contain "cluster_https_"

    Given I authenticate using basic auth as "admin"
    When I delete the API "api-level-url-stable-collision-api-v1.0"
    Then the response should be successful

  Scenario: Two APIs sharing the same backend host route independently through their own identity-named clusters
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: api-level-url-stable-shared-a-v1.0
      spec:
        displayName: API-Level-URL-Stable-Shared-A
        version: v1.0
        context: /api-level-url-stable-shared-a/$version
        vhosts:
          main: api-level-url-stable-shared-a.local
        upstream:
          main:
            url: http://sample-backend:9080/shared-a
        operations:
          - method: GET
            path: /endpoint
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/api-level-url-stable-shared-a/v1.0/endpoint" to be ready with host "api-level-url-stable-shared-a.local"

    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: api-level-url-stable-shared-b-v1.0
      spec:
        displayName: API-Level-URL-Stable-Shared-B
        version: v1.0
        context: /api-level-url-stable-shared-b/$version
        vhosts:
          main: api-level-url-stable-shared-b.local
        upstream:
          main:
            url: http://sample-backend:9080/shared-b
        operations:
          - method: GET
            path: /endpoint
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/api-level-url-stable-shared-b/v1.0/endpoint" to be ready with host "api-level-url-stable-shared-b.local"

    # Two distinct APIs point at the same backend host:port. The old URL-derived
    # naming made them share one cluster_<scheme>_<host> cluster; identity naming
    # keys each cluster on its API ID, so the two APIs route independently to their
    # own base paths under identity-named clusters and no URL-derived cluster exists.
    When I clear all headers
    And I set request host to "api-level-url-stable-shared-a.local"
    And I send a GET request to "http://localhost:8080/api-level-url-stable-shared-a/v1.0/endpoint"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/shared-a/endpoint"

    When I clear all headers
    And I set request host to "api-level-url-stable-shared-b.local"
    And I send a GET request to "http://localhost:8080/api-level-url-stable-shared-b/v1.0/endpoint"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/shared-b/endpoint"

    When I clear all headers
    And I send a GET request to "http://localhost:9901/clusters"
    Then the response should be successful
    And the response body should contain "main_"
    And the response body should not contain "cluster_http_"
    And the response body should not contain "cluster_https_"

    # Delete API-B and confirm API-A still routes, proving the two APIs own
    # independent clusters (deleting one does not disturb the other).
    Given I authenticate using basic auth as "admin"
    When I delete the API "api-level-url-stable-shared-b-v1.0"
    Then the response should be successful

    When I clear all headers
    And I set request host to "api-level-url-stable-shared-a.local"
    And I send a GET request to "http://localhost:8080/api-level-url-stable-shared-a/v1.0/endpoint"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/shared-a/endpoint"

    Given I authenticate using basic auth as "admin"
    When I delete the API "api-level-url-stable-shared-a-v1.0"
    Then the response should be successful

  Scenario: API-level main upstream scheme and port change keeps the same identity-named cluster
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: api-level-url-stable-scheme-api-v1.0
      spec:
        displayName: API-Level-URL-Stable-Scheme-API
        version: v1.0
        context: /api-level-url-stable-scheme/$version
        vhosts:
          main: api-level-url-stable-scheme.local
        upstream:
          main:
            url: http://sample-backend:9080/version-a
        operations:
          - method: GET
            path: /endpoint
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/api-level-url-stable-scheme/v1.0/endpoint" to be ready with host "api-level-url-stable-scheme.local"

    # Capture the identity-derived cluster name while the upstream is plain http.
    When I clear all headers
    And I send a GET request to "http://localhost:9901/clusters"
    Then the response should be successful
    And the response body should contain "main_"
    And the response body should not contain "cluster_http_"
    And the response body should not contain "cluster_https_"
    And I capture the Envoy cluster names prefixed "main_"

    # Change the upstream scheme (http -> https) AND port (9080 -> 9443) in one
    # edit. The old URL-derived naming embedded scheme and port in the cluster
    # name (cluster_<scheme>_<host>_<port>), so this edit would have minted a new
    # cluster_https_ cluster and dropped the previous one. Identity-based naming
    # must keep the SAME main_<hash> and never produce a cluster_https_. TLS
    # routing itself is not asserted (there is no TLS echo backend), so there is no
    # endpoint-readiness wait here; the cluster-set check below observes a settle
    # window instead to cover xDS propagation. The cluster
    # name is stable independent of upstream reachability.
    Given I authenticate using basic auth as "admin"
    When I update the API "api-level-url-stable-scheme-api-v1.0" with this configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: api-level-url-stable-scheme-api-v1.0
      spec:
        displayName: API-Level-URL-Stable-Scheme-API
        version: v1.0
        context: /api-level-url-stable-scheme/$version
        vhosts:
          main: api-level-url-stable-scheme.local
        upstream:
          main:
            url: https://sample-backend:9443/version-b
        operations:
          - method: GET
            path: /endpoint
      """
    Then the response should be successful

    # The main_<hash> name set must be UNCHANGED after the scheme/port edit, and
    # no URL-derived cluster_https_ may appear.
    When I clear all headers
    And I send a GET request to "http://localhost:9901/clusters"
    Then the response should be successful
    And the response body should contain "main_"
    And the response body should not contain "cluster_http_"
    And the response body should not contain "cluster_https_"
    And the Envoy cluster names prefixed "main_" should be unchanged

    Given I authenticate using basic auth as "admin"
    When I delete the API "api-level-url-stable-scheme-api-v1.0"
    Then the response should be successful

  # ===== per-operation upstream on match-form operations (Gateway-API-style method + path.value + headers) =====
  Scenario: Per-operation main ref on a match-form operation routes to the ref'd backend
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: per-op-match-basic-api-v1.0
      spec:
        displayName: Per-Op-Match-Basic-API
        version: v1.0
        context: /per-op-match-basic/$version
        vhosts:
          main: per-op-match-basic-main.local
        upstreamDefinitions:
          - name: match-main-svc
            upstreams:
              - url: http://sample-backend:9080
            basePath: /match-main
        upstream:
          main:
            url: http://sample-backend:9080/api-main
        operations:
          - match:
              method: GET
              path:
                value: /users
            upstream:
              main:
                ref: match-main-svc
          - match:
              method: GET
              path:
                value: /orders
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/per-op-match-basic/v1.0/users" to be ready with host "per-op-match-basic-main.local"

    When I clear all headers
    And I set request host to "per-op-match-basic-main.local"
    And I send a GET request to "http://localhost:8080/per-op-match-basic/v1.0/users"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/match-main/users"

    When I clear all headers
    And I set request host to "per-op-match-basic-main.local"
    And I send a GET request to "http://localhost:8080/per-op-match-basic/v1.0/orders"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/api-main/orders"

    Given I authenticate using basic auth as "admin"
    When I delete the API "per-op-match-basic-api-v1.0"
    Then the response should be successful

  Scenario: Header matcher on a match-form operation selects the per-operation ref
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: per-op-match-hdr-api-v1.0
      spec:
        displayName: Per-Op-Match-Hdr-API
        version: v1.0
        context: /per-op-match-hdr/$version
        vhosts:
          main: per-op-match-hdr-main.local
        upstreamDefinitions:
          - name: match-canary-svc
            upstreams:
              - url: http://sample-backend:9080
            basePath: /canary
        upstream:
          main:
            url: http://sample-backend:9080/api-main
        operations:
          - match:
              method: GET
              path:
                value: /items
              headers:
                - name: x-variant
                  value: canary
            upstream:
              main:
                ref: match-canary-svc
          - match:
              method: GET
              path:
                value: /items
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/per-op-match-hdr/v1.0/items" to be ready with host "per-op-match-hdr-main.local"

    When I clear all headers
    And I set request host to "per-op-match-hdr-main.local"
    And I set header "x-variant" to "canary"
    And I send a GET request to "http://localhost:8080/per-op-match-hdr/v1.0/items"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/canary/items"

    When I clear all headers
    And I set request host to "per-op-match-hdr-main.local"
    And I send a GET request to "http://localhost:8080/per-op-match-hdr/v1.0/items"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/api-main/items"

    Given I authenticate using basic auth as "admin"
    When I delete the API "per-op-match-hdr-api-v1.0"
    Then the response should be successful

  Scenario: Per-operation sandbox ref on a match-form operation routes sandbox traffic
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: per-op-match-sb-api-v1.0
      spec:
        displayName: Per-Op-Match-SB-API
        version: v1.0
        context: /per-op-match-sb/$version
        vhosts:
          main: per-op-match-sb-main.local
          sandbox: per-op-match-sb-sandbox.local
        upstreamDefinitions:
          - name: match-sandbox-svc
            upstreams:
              - url: http://sample-backend:9080
            basePath: /match-sandbox
        upstream:
          main:
            url: http://sample-backend:9080/api-main
        operations:
          - match:
              method: GET
              path:
                value: /users
            upstream:
              sandbox:
                ref: match-sandbox-svc
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/per-op-match-sb/v1.0/users" to be ready with host "per-op-match-sb-sandbox.local"

    When I clear all headers
    And I set request host to "per-op-match-sb-sandbox.local"
    And I send a GET request to "http://localhost:8080/per-op-match-sb/v1.0/users"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "path" should be "/match-sandbox/users"

    Given I authenticate using basic auth as "admin"
    When I delete the API "per-op-match-sb-api-v1.0"
    Then the response should be successful
