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

@per-op-upstream-basic
Feature: Per-Operation Upstream Basic Routing
  As an API developer
  I want per-operation upstream refs to override API-level upstreams
  So that different operations can route to different backends

  Background:
    Given the gateway services are running

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
