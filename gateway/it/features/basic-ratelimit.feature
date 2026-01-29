# --------------------------------------------------------------------
# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

@basic-ratelimit
Feature: Basic Rate Limiting
  As an API developer
  I want a simple rate limiting policy
  So that I can easily protect my APIs without complex configuration

  Background:
    Given the gateway services are running

  Scenario: Enforce basic rate limit on API resource
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: basic-ratelimit-test-api
      spec:
        displayName: Basic RateLimit Test API
        version: v1.0
        context: /basic-ratelimit/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /limited
            policies:
              - name: basic-ratelimit
                version: v0.1.2
                params:
                  limits:
                    - limit: 5
                      duration: "1h"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/basic-ratelimit/v1.0/limited" to be ready

    # Send 4 requests - all should succeed (readiness check used ~1)
    When I send 4 GET requests to "http://localhost:8080/basic-ratelimit/v1.0/limited"
    Then the response status code should be 200

    # Send 1 more request to exhaust the quota (total ~6 requests including readiness)
    When I send a GET request to "http://localhost:8080/basic-ratelimit/v1.0/limited"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

  Scenario: Rate limit headers are returned
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: basic-ratelimit-headers-api
      spec:
        displayName: Basic RateLimit Headers API
        version: v1.0
        context: /basic-ratelimit-headers/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /check
            policies:
              - name: basic-ratelimit
                version: v0.1.2
                params:
                  limits:
                    - limit: 100
                      duration: "1h"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/basic-ratelimit-headers/v1.0/check" to be ready

    When I send a GET request to "http://localhost:8080/basic-ratelimit-headers/v1.0/check"
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should be "100"
    And the response header "X-RateLimit-Remaining" should exist
    And the response header "X-RateLimit-Reset" should exist

  Scenario: Multiple limits enforce most restrictive limit
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: basic-ratelimit-multi-limits-api
      spec:
        displayName: Basic RateLimit Multi Limits API
        version: v1.0
        context: /basic-ratelimit-multi/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /resource
            policies:
              - name: basic-ratelimit
                version: v0.1.2
                params:
                  limits:
                    - limit: 10
                      duration: "1h"
                    - limit: 5
                      duration: "24h"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/basic-ratelimit-multi/v1.0/health" to be ready

    # 24h limit (5) is more restrictive than 1h limit (10)
    # Send 5 requests - should succeed (5/5 for 24h, 5/10 for 1h)
    When I send 5 GET requests to "http://localhost:8080/basic-ratelimit-multi/v1.0/resource"
    Then the response status code should be 200

    # 6th request should be blocked by 24h limit
    When I send a GET request to "http://localhost:8080/basic-ratelimit-multi/v1.0/resource"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

  Scenario: Per-route rate limiting with basic-ratelimit
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: basic-ratelimit-per-route-api
      spec:
        displayName: Basic RateLimit Per Route API
        version: v1.0
        context: /basic-ratelimit-per-route/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /route1
            policies:
              - name: basic-ratelimit
                version: v0.1.2
                params:
                  limits:
                    - limit: 3
                      duration: "1h"
          - method: GET
            path: /route2
            policies:
              - name: basic-ratelimit
                version: v0.1.2
                params:
                  limits:
                    - limit: 3
                      duration: "1h"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/basic-ratelimit-per-route/v1.0/health" to be ready

    # Each route has its own quota (basic-ratelimit uses routename as key)
    # Send 3 requests to route1 - should succeed (uses route1's quota)
    When I send 3 GET requests to "http://localhost:8080/basic-ratelimit-per-route/v1.0/route1"
    Then the response status code should be 200

    # route1's 4th request should be rate limited
    When I send a GET request to "http://localhost:8080/basic-ratelimit-per-route/v1.0/route1"
    Then the response status code should be 429

    # route2 has its own separate quota - should still work
    When I send 3 GET requests to "http://localhost:8080/basic-ratelimit-per-route/v1.0/route2"
    Then the response status code should be 200

    # route2's 4th request should also be rate limited
    When I send a GET request to "http://localhost:8080/basic-ratelimit-per-route/v1.0/route2"
    Then the response status code should be 429

  Scenario: 429 response includes Retry-After header
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: basic-ratelimit-retry-after-api
      spec:
        displayName: Basic RateLimit Retry After API
        version: v1.0
        context: /basic-ratelimit-retry/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /resource
            policies:
              - name: basic-ratelimit
                version: v0.1.2
                params:
                  limits:
                    - limit: 3
                      duration: "1h"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/basic-ratelimit-retry/v1.0/health" to be ready

    # Exhaust the rate limit (limit=3)
    When I send 3 GET requests to "http://localhost:8080/basic-ratelimit-retry/v1.0/resource"
    Then the response status code should be 200

    # Next request should be rate limited with Retry-After header
    When I send a GET request to "http://localhost:8080/basic-ratelimit-retry/v1.0/resource"
    Then the response status code should be 429
    And the response header "Retry-After" should exist

  Scenario: Rate limit scope based on policy attachment level
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: basic-ratelimit-scope-api
      spec:
        displayName: Basic RateLimit Scope API
        version: v1.0
        context: /basic-ratelimit-scope/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        policies:
          - name: basic-ratelimit
            version: v0.1.2
            params:
              limits:
                - limit: 5
                  duration: "1h"
        operations:
          - method: GET
            path: /health
            policies:
              - name: basic-ratelimit
                version: v0.1.2
                params:
                  limits:
                    - limit: 100
                      duration: "1h"
          - method: GET
            path: /resource-a
          - method: GET
            path: /resource-b
            policies:
              - name: basic-ratelimit
                version: v0.1.2
                params:
                  limits:
                    - limit: 3
                      duration: "1h"
          - method: GET
            path: /resource-c
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/basic-ratelimit-scope/v1.0/health" to be ready

    # Resource B has its own route-level policy (Limit: 3)
    # Send 3 requests to B -> Should succeed
    When I send 3 GET requests to "http://localhost:8080/basic-ratelimit-scope/v1.0/resource-b"
    Then the response status code should be 200

    # 4th request to B -> Should fail (Limit 3 exhausted)
    When I send a GET request to "http://localhost:8080/basic-ratelimit-scope/v1.0/resource-b"
    Then the response status code should be 429

    # Resource A and C fall back to API-level policy (Limit: 5, Shared)
    # Send 2 requests to A -> Should succeed
    When I send 2 GET requests to "http://localhost:8080/basic-ratelimit-scope/v1.0/resource-a"
    Then the response status code should be 200

    # Send 2 requests to C -> Should succeed (Total 4/5)
    When I send 2 GET requests to "http://localhost:8080/basic-ratelimit-scope/v1.0/resource-c"
    Then the response status code should be 200

    # Send 1 request to A -> Should succeed (Total 5/5)
    When I send a GET request to "http://localhost:8080/basic-ratelimit-scope/v1.0/resource-a"
    Then the response status code should be 200

    # Send 1 request to C -> Should fail (Total 6/5, Limit 5 exhausted)
    When I send a GET request to "http://localhost:8080/basic-ratelimit-scope/v1.0/resource-c"
    Then the response status code should be 429

    # Verify B is still rate limited (independent of A/C bucket)
    When I send a GET request to "http://localhost:8080/basic-ratelimit-scope/v1.0/resource-b"
    Then the response status code should be 429
