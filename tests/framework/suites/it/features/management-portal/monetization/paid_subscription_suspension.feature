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

@platform-api-paid-subscription-suspension
Feature: Suspending a paid subscription keeps its billing
  As the operator of a monetized API
  I want an administrator to suspend and resume a paid subscription
  So that a suspended subscription is refused at the gateway, and keeps its billing once resumed

  # Runs on gateways newer than 1.2.0 only: older gateways don't take a subscription's billing
  # identifiers from the control plane.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "paid-access-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  Scenario: An administrator can suspend a paid subscription, and resuming it keeps its billing
    Given I generate a unique resource name from "paid-suspend-plan" and store it as "planHandle"
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id":"${CTX:planHandle}","displayName":"Paid ${CTX:planHandle}","status":"ACTIVE","limits":[{"limitType":"REQUEST_COUNT","timeUnit":"HOUR","limitCount":10000}],"pricing":{"pricingType":"FLAT_RATE","currency":"EUR","billingPeriod":"YEAR","flatAmount":"290"}}
      """
    Then the response status code should be 201
    Given I generate a unique resource name from "paid-suspend-api" and store it as "apiHandle"
    And I generate a unique API context from "/paid-suspend" and store it as "apiContext"
    And I generate a unique resource name from "cus" and store it as "customerId"
    When I create a secured REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" offering plan "${CTX:planHandle}"
    And I deploy the "RestApi" "${CTX:apiHandle}" to the gateway via the control plane
    And I send a "GET" request to "${CTX:apiContext}/" until status 401
    And I generate a unique resource name from "paid-suspend-app" and store it as "appHandle"
    And I create an application "${CTX:appHandle}" via the control plane in project "${CTX:projectHandle}"
    And I create a subscription for REST API "${CTX:apiHandle}" application "${CTX:appHandle}" plan "${CTX:planHandle}" via the control plane and store the subscription token as "subscriptionToken"
    And I generate a unique value from "paid-suspend-key" and store it as "apiKeyValue"
    And I issue an API key "${CTX:apiKeyValue}" for REST API "${CTX:apiHandle}" via the control plane
    When I clear all headers
    And the API Portal sends the "subscription.billing_updated" event to the control plane with data:
      """
      {"subscriber_id":"e2e-subscriber","api":{"ref_id":"${CTX:apiHandle}","type":"RestApi"},"billing":{"status":"PAST_DUE","customer_id":"${CTX:customerId}","subscription_id":"sub-${CTX:customerId}"}}
      """
    Then the response status code should be 200

    # Past due is served, and the gateway holds the subscription's billing identifiers.
    When I set header "API-Key" to "${CTX:apiKeyValue}"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    Then I send a "GET" request to "${CTX:apiContext}/" until status 200
    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I send a "GET" request to the "gateway-controller" service at "/subscriptions?apiId=${CTX:apiHandle}"
    Then the response status should be 200
    And the JSON response array "subscriptions" should contain an item with "billingCustomerId" equal to "${CTX:customerId}"

    # An administrator suspends the paid subscription: refused, then served again, billing kept.
    When I clear all headers
    And I send a "GET" request to the control plane at "/subscriptions?artifactId=${CTX:apiHandle}&subscriberId=e2e-subscriber"
    And I store the JSON response field "list[0].id" as "subscriptionId"
    And I send a "PUT" request to the control plane at "/subscriptions/${CTX:subscriptionId}?subscriberId=e2e-subscriber" with body:
      """
      {"status":"INACTIVE"}
      """
    Then the response status code should be 200
    When I set header "API-Key" to "${CTX:apiKeyValue}"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    Then I send a "GET" request to "${CTX:apiContext}/" until status 403
    When I clear all headers
    And I send a "PUT" request to the control plane at "/subscriptions/${CTX:subscriptionId}?subscriberId=e2e-subscriber" with body:
      """
      {"status":"ACTIVE"}
      """
    Then the response status code should be 200
    And the JSON response field "billing.customerId" should be "${CTX:customerId}"
    When I set header "API-Key" to "${CTX:apiKeyValue}"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    Then I send a "GET" request to "${CTX:apiContext}/" until status 200
