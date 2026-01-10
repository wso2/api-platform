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

@ratelimit
Feature: Rate Limiting
  As an API developer
  I want to limit the rate of requests to my API
  So that I can protect my upstream services from excessive traffic

  Background:
    Given the gateway services are running

  Scenario: Enforce rate limit on API resource
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: ratelimit-test-api
      spec:
        displayName: RateLimit Test API
        version: v1.0
        context: /ratelimit/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /limited
            policies:
              - name: ratelimit
                version: v0.1.0
                params:
                  cost: 1
                  limits:
                    - limit: 10
                      duration: "1m"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/ratelimit/v1.0/limited" to be ready

    # Send 8 requests - all should succeed (readiness check used ~1, we have limit 10)
    When I send 8 GET requests to "http://localhost:8080/ratelimit/v1.0/limited"
    Then the response status code should be 200

    # Send 2 more requests to exhaust the quota (total ~11 requests including readiness)
    When I send 2 GET requests to "http://localhost:8080/ratelimit/v1.0/limited"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"
    And the response header "Retry-After" should exist

  Scenario: Rate limit headers are returned
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: ratelimit-headers-api
      spec:
        displayName: RateLimit Headers API
        version: v1.0
        context: /ratelimit-headers/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /check
            policies:
              - name: ratelimit
                version: v0.1.0
                params:
                  cost: 1
                  limits:
                    - limit: 100
                      duration: "1m"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/ratelimit-headers/v1.0/check" to be ready

    When I send a GET request to "http://localhost:8080/ratelimit-headers/v1.0/check"
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should be "100"
    And the response header "X-RateLimit-Remaining" should exist
    And the response header "X-RateLimit-Reset" should exist

  Scenario: Custom rate limit error response
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: ratelimit-custom-api
      spec:
        displayName: RateLimit Custom API
        version: v1.0
        context: /ratelimit-custom/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /custom
            policies:
              - name: ratelimit
                version: v0.1.0
                params:
                  cost: 1
                  limits:
                    - limit: 5
                      duration: "1m"
                  onRateLimitExceeded:
                    statusCode: 429
                    body: '{"error": "Too Many Requests", "code": 429001}'
                    bodyFormat: json
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/ratelimit-custom/v1.0/custom" to be ready

    # Send requests to exceed the limit (limit=5, readiness check uses ~1)
    When I send 5 GET requests to "http://localhost:8080/ratelimit-custom/v1.0/custom"
    Then the response status code should be 429
    And the response should be valid JSON
    And the JSON response field "error" should be "Too Many Requests"
    And the JSON response field "code" should be 429001

  Scenario: Weighted rate limiting with cost parameter
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: ratelimit-weighted-api
      spec:
        displayName: RateLimit Weighted API
        version: v1.0
        context: /ratelimit-weighted/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /expensive
            policies:
              - name: ratelimit
                version: v0.1.0
                params:
                  cost: 5
                  limits:
                    - limit: 20
                      duration: "1m"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/ratelimit-weighted/v1.0/expensive" to be ready

    # Each request costs 5 tokens. With limit 20 and readiness check (~1 request = 5 tokens),
    # we have ~15 tokens left. Send 3 requests (15 tokens) to exhaust quota.
    When I send 3 GET requests to "http://localhost:8080/ratelimit-weighted/v1.0/expensive"
    Then the response status code should be 200

    # Next request should be rate limited
    When I send a GET request to "http://localhost:8080/ratelimit-weighted/v1.0/expensive"
    Then the response status code should be 429

  Scenario: Cost extraction from response body using echo backend
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: ratelimit-cost-extraction-api
      spec:
        displayName: RateLimit Cost Extraction API
        version: v1.0
        context: /ratelimit-cost-extraction/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /anything
          - method: POST
            path: /anything
            policies:
              - name: ratelimit
                version: v0.1.0
                params:
                  limits:
                    - limit: 100
                      duration: "1m"
                  costExtraction:
                    enabled: true
                    sources:
                      - type: response_body
                        jsonPath: "$.json.custom_cost"
                    default: 1
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/ratelimit-cost-extraction/v1.0/anything" to be ready

    # Send a POST request with custom_cost=50 in the body
    # The echo backend will echo back the JSON, and cost extraction will read $.json.custom_cost
    When I send a POST request to "http://localhost:8080/ratelimit-cost-extraction/v1.0/anything" with body:
      """
      {"custom_cost": 50}
      """
    Then the response status code should be 200
    # After first request: 100 - 50 = 50 remaining
    And the response header "X-RateLimit-Remaining" should be "50"

    # Send another request with custom_cost=50
    When I send a POST request to "http://localhost:8080/ratelimit-cost-extraction/v1.0/anything" with body:
      """
      {"custom_cost": 50}
      """
    Then the response status code should be 200
    # After second request: 50 - 50 = 0 remaining
    And the response header "X-RateLimit-Remaining" should be "0"

    # Third request should be rate limited since quota is exhausted
    When I send a POST request to "http://localhost:8080/ratelimit-cost-extraction/v1.0/anything" with body:
      """
      {"custom_cost": 10}
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

  Scenario: API-level rate limiting with apiname key extraction
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: ratelimit-api-level-api
      spec:
        displayName: RateLimit API Level Test
        version: v1.0
        context: /ratelimit-api-level/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /route0
          - method: GET
            path: /route1
            policies:
              - name: ratelimit
                version: v0.1.0
                params:
                  cost: 1
                  limits:
                    - limit: 10
                      duration: "1m"
                  keyExtraction:
                    - type: apiname
          - method: GET
            path: /route2
            policies:
              - name: ratelimit
                version: v0.1.0
                params:
                  cost: 1
                  limits:
                    - limit: 10
                      duration: "1m"
                  keyExtraction:
                    - type: apiname
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/ratelimit-api-level/v1.0/route0" to be ready

    # Send 5 requests to route1 - uses API-level bucket
    When I send 5 GET requests to "http://localhost:8080/ratelimit-api-level/v1.0/route1"
    Then the response status code should be 200

    # Send 4 requests to route2 - shares the same API-level bucket
    # Total: 5 + 4 = 9 requests (still within limit 10)
    When I send 4 GET requests to "http://localhost:8080/ratelimit-api-level/v1.0/route2"
    Then the response status code should be 200

    # The 10th request (either route) should work
    When I send a GET request to "http://localhost:8080/ratelimit-api-level/v1.0/route1"
    Then the response status code should be 200

    # The 11th request should be rate limited - quota exhausted across both routes
    When I send a GET request to "http://localhost:8080/ratelimit-api-level/v1.0/route2"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    When I send a GET request to "http://localhost:8080/ratelimit-api-level/v1.0/route1"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

  Scenario: Updating API does not reset rate limit state
    Given I authenticate using basic auth as "admin"
    # Deploy initial API with rate limit
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: ratelimit-update-test-api
      spec:
        displayName: RateLimit Update Test API
        version: v1.0
        context: /ratelimit-update-test/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /test
          - method: GET
            path: /resource
            policies:
              - name: ratelimit
                version: v0.1.0
                params:
                  cost: 1
                  limits:
                    - limit: 5
                      duration: "1m"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/ratelimit-update-test/v1.0/test" to be ready

    # Exhaust the rate limit quota (5 requests)
    When I send 5 GET requests to "http://localhost:8080/ratelimit-update-test/v1.0/resource"
    Then the response status code should be 200

    # Verify quota is exhausted
    When I send a GET request to "http://localhost:8080/ratelimit-update-test/v1.0/resource"
    Then the response status code should be 429

    # Update the API by adding a new unrelated route (PUT /handle)
    When I update the API "ratelimit-update-test-api" with this configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: ratelimit-update-test-api
      spec:
        displayName: RateLimit Update Test API
        version: v1.0
        context: /ratelimit-update-test/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /test
          - method: GET
            path: /resource
            policies:
              - name: ratelimit
                version: v0.1.0
                params:
                  cost: 1
                  limits:
                    - limit: 5
                      duration: "1m"
          - method: PUT
            path: /handle
      """
    Then the response should be successful
    And I wait for 2 seconds

    # Verify rate limit state was preserved - should still be 429
    When I send a GET request to "http://localhost:8080/ratelimit-update-test/v1.0/resource"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"
