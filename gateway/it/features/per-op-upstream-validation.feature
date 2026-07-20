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

@per-op-upstream-validation
Feature: Per-Operation Upstream Validation
  As an API developer
  I want malformed per-operation upstream configurations to be rejected
  So that invalid APIs cannot be deployed

  Background:
    Given the gateway services are running

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
