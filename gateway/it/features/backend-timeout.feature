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

@backend-timeout @resilience
Feature: Backend route timeouts via the resilience block
  As an API developer
  I want to configure the route timeout via a resilience block at the API and operation level
  So that requests to slow backends are terminated by the gateway within the configured time

  # These scenarios exercise resilience.timeout (Envoy RouteAction.Timeout). The slow
  # backend is httpbin (echo-backend) /delay/{n}, which sleeps n seconds before responding;
  # when the route timeout is shorter than the delay, the gateway returns 504.
  # resilience.idleTimeout maps to RouteAction.IdleTimeout and is covered by unit tests
  # (it cannot be exercised deterministically over HTTP here).

  Background:
    Given the gateway services are running

  # API-level resilience.timeout (2s) is shorter than the backend delay (5s), so the
  # gateway must time the route out with 504 at ~2s instead of waiting for the backend.
  Scenario: API-level resilience timeout terminates a slow backend
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: backend-timeout-api-v1.0
      spec:
        displayName: Backend-Timeout-API
        version: v1.0
        context: /backend-timeout-api/$version
        upstream:
          main:
            url: http://echo-backend:80
        resilience:
          timeout: 2s
        operations:
          - method: GET
            path: /get
          - method: GET
            path: /delay/5
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/backend-timeout-api/v1.0/get" to be ready
    And I record the current time as "request_start"
    When I send a GET request to "http://localhost:8080/backend-timeout-api/v1.0/delay/5"
    Then the response status code should be 504
    And the request should have taken at least "2" seconds since "request_start"
    Given I authenticate using basic auth as "admin"
    When I delete the API "backend-timeout-api-v1.0"
    Then the response should be successful

  # Operation-level resilience overrides the API level: the API-level timeout (10s) is
  # longer than the backend delay (5s) and would let the request succeed, but the
  # operation-level timeout (2s) wins, so the gateway returns 504 at ~2s.
  Scenario: Operation-level resilience timeout overrides the API level
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: backend-timeout-override-v1.0
      spec:
        displayName: Backend-Timeout-Override
        version: v1.0
        context: /backend-timeout-override/$version
        upstream:
          main:
            url: http://echo-backend:80
        resilience:
          timeout: 10s
        operations:
          - method: GET
            path: /get
          - method: GET
            path: /delay/5
            resilience:
              timeout: 2s
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/backend-timeout-override/v1.0/get" to be ready
    And I record the current time as "request_start"
    When I send a GET request to "http://localhost:8080/backend-timeout-override/v1.0/delay/5"
    Then the response status code should be 504
    And the request should have taken at least "2" seconds since "request_start"
    Given I authenticate using basic auth as "admin"
    When I delete the API "backend-timeout-override-v1.0"
    Then the response should be successful

  # Without a resilience block the global route timeout default (60s) applies, so a
  # backend that responds within it succeeds normally.
  Scenario: No resilience block falls back to the global default and succeeds
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: backend-timeout-default-v1.0
      spec:
        displayName: Backend-Timeout-Default
        version: v1.0
        context: /backend-timeout-default/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /get
          - method: GET
            path: /delay/2
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/backend-timeout-default/v1.0/get" to be ready
    When I send a GET request to "http://localhost:8080/backend-timeout-default/v1.0/delay/2"
    Then the response status code should be 200
    Given I authenticate using basic auth as "admin"
    When I delete the API "backend-timeout-default-v1.0"
    Then the response should be successful
