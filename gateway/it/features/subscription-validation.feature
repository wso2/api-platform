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

@subscription-validation
Feature: Subscription Validation
  As an API consumer
  I want to access APIs only with a valid subscription
  So that access is controlled and rate limits are enforced per subscription

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Subscription validation and rate limit enforcement
    # Step 1: Create a business plan with 3 requests per minute
    When I create a subscription plan "Business" with 3 requests per minute
    Then the response status should be 201
    And I store the last response field "id" as "planId"

    # Step 2: Deploy API with subscription plan and subscription-validation policy
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: subscription-validation-test-api
      spec:
        displayName: Subscription Validation Test API
        version: v1.0
        context: /subscription-validation-test/$version
        subscriptionPlans:
          - Business
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: subscription-validation
                version: v0
                params:
                  subscriptionKeyHeader: "Subscription-Key"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/subscription-validation-test/v1.0/health" to return 403

    # Step 3: Invoke API without subscription header - expect 403
    When I send a GET request to "http://localhost:8080/subscription-validation-test/v1.0/health"
    Then the response status code should be 403

    # Step 4: Inject subscription via mock platform-api (mimics subscription.created WebSocket event from platform-api)
    When I create a subscription for API "subscription-validation-test-api" with plan and token "mock-subscription-token-for-it-test"
    Then the response status should be 201

    # Step 5: Invoke API with subscription token - expect 200
    When I send a GET request to "http://localhost:8080/subscription-validation-test/v1.0/health" with header "Subscription-Key" value "mock-subscription-token-for-it-test"
    Then the response status code should be 200

    # Step 6: Invoke 3 more times to exhaust rate limit (3 req/min) - 4th request total gets 429
    When I send 3 GET requests to "http://localhost:8080/subscription-validation-test/v1.0/health" with header "Subscription-Key" value "mock-subscription-token-for-it-test"
    Then the response status code should be 429

    # Cleanup
    When I delete the API "subscription-validation-test-api"
    Then the response should be successful
