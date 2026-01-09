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
                    - limit: 5
                      duration: "1m"
      """
    Then the response should be successful
    And I wait for 2 seconds
    
    # Send 5 allowed requests
    When I send 5 GET requests to "http://localhost:8080/ratelimit/v1.0/limited"
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "0"

    # Send 6th request (should fail)
    When I send a GET request to "http://localhost:8080/ratelimit/v1.0/limited"
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"
    And the response header "Retry-After" should exist

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
                  limits:
                    - limit: 1
                      duration: "1m"
                  onRateLimitExceeded:
                    statusCode: 429
                    body: '{"error": "Too Many Requests", "code": 429001}'
                    bodyFormat: json
      """
    Then the response should be successful
    And I wait for 2 seconds
    
    # Send 1 allowed request
    When I send a GET request to "http://localhost:8080/ratelimit-custom/v1.0/custom"
    Then the response status code should be 200

    # Send request exceeding limit
    When I send a GET request to "http://localhost:8080/ratelimit-custom/v1.0/custom"
    Then the response status code should be 429
    And the response should be valid JSON
    And the JSON response field "error" should be "Too Many Requests"
    And the JSON response field "code" should be 429001
