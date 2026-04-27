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

@subscription-analytics
Feature: Subscription Analytics - Monetized Subscription Billing Metadata
  As a platform administrator
  I want billing fields from a monetized subscription to appear in analytics events
  So that usage can be tracked and correlated with billing records

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Monetized subscription billing fields appear in analytics events
    # Step 1: Create a subscription plan
    When I create a subscription plan "MonetizedPlan" with 100 requests per minute
    Then the response status should be 201
    And I store the last response field "id" as "planId"

    # Step 2: Deploy API with subscription-validation policy
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: subscription-analytics-test-api
      spec:
        displayName: Subscription Analytics Test API
        version: v1.0
        context: /subscription-analytics-test/$version
        subscriptionPlans:
          - MonetizedPlan
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: subscription-validation
                version: v1
                params:
                  subscriptionKeyHeader: "Subscription-Key"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/subscription-analytics-test/v1.0/health" to return 403

    # Step 3: Reset analytics before injecting subscription so only events from this test are captured
    And I reset the analytics collector

    # Step 4: Inject monetized subscription with billing fields
    When I create a monetized subscription for API "subscription-analytics-test-api" with plan, token "sub-token-billing-test", billing customer "cust-123" and billing subscription "billing-sub-456"
    Then the response status should be 201

    # Step 5: Invoke API with subscription token
    When I send a GET request to "http://localhost:8080/subscription-analytics-test/v1.0/health" with header "Subscription-Key" value "sub-token-billing-test"
    Then the response status code should be 200

    # Step 6: Wait for analytics to be published and assert billing metadata
    And I wait 5 seconds for analytics to be published
    And the analytics collector should have received at least 1 event
    And the latest analytics event should have request URI "/subscription-analytics-test/v1.0/health"
    And the latest analytics event should have metadata field "billingCustomerId" with value "cust-123"
    And the latest analytics event should have metadata field "billingSubscriptionId" with value "billing-sub-456"
    And the latest analytics event should have metadata field "subscriptionStatus" with value "ACTIVE"
    And the latest analytics event should have metadata field "subscriptionPlanName" with value "MonetizedPlan"

    # Cleanup
    When I delete the API "subscription-analytics-test-api"
    Then the response should be successful
    When I delete the subscription plan with stored id "planId"
    Then the response should be successful
