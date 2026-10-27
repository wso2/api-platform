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

@platform-api-paid-subscription-access
Feature: A paid subscription's billing state decides its access at the gateway
  As the operator of a monetized API
  I want the gateway to stop serving a subscription that stopped paying, and to serve it again once it is paid
  So that unpaid use is never served, while a late payment does not lock a customer out for good

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "paid-access-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  Scenario: A subscription that stops paying is refused at the gateway, and is served again once paid
    Given I generate a unique resource name from "paid-access-plan" and store it as "planHandle"
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id":"${CTX:planHandle}","displayName":"Paid ${CTX:planHandle}","status":"ACTIVE","limits":[{"limitType":"REQUEST_COUNT","timeUnit":"HOUR","limitCount":10000}],"pricing":{"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"29"}}
      """
    Then the response status code should be 201
    Given I generate a unique resource name from "paid-access-api" and store it as "apiHandle"
    And I generate a unique API context from "/paid-access" and store it as "apiContext"
    When I create a secured REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" offering plan "${CTX:planHandle}"
    And I deploy the "RestApi" "${CTX:apiHandle}" to the gateway via the control plane
    Then I send a "GET" request to "${CTX:apiContext}/" until status 401
    When I generate a unique resource name from "paid-access-app" and store it as "appHandle"
    And I create an application "${CTX:appHandle}" via the control plane in project "${CTX:projectHandle}"
    And I create a subscription for REST API "${CTX:apiHandle}" application "${CTX:appHandle}" plan "${CTX:planHandle}" via the control plane and store the subscription token as "subscriptionToken"
    And I generate a unique value from "paid-access-key" and store it as "apiKeyValue"
    And I issue an API key "${CTX:apiKeyValue}" for REST API "${CTX:apiHandle}" via the control plane
    And I generate a unique resource name from "cus" and store it as "customerId"
    When I set header "API-Key" to "${CTX:apiKeyValue}"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    Then I send a "GET" request to "${CTX:apiContext}/" until status 200

    # The API Portal reports the checkout paid: the subscription carries its billing.
    When I clear all headers
    And the API Portal sends the "subscription.billing_updated" event to the control plane with data:
      """
      {"subscriber_id":"e2e-subscriber","api":{"ref_id":"${CTX:apiHandle}","type":"RestApi"},"billing":{"status":"ACTIVE","customer_id":"${CTX:customerId}","subscription_id":"sub-${CTX:customerId}"}}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/subscriptions?artifactId=${CTX:apiHandle}&subscriberId=e2e-subscriber"
    Then the response status code should be 200
    And the JSON response field "list[0].billing.status" should be "ACTIVE"
    And the JSON response field "list[0].billing.customerId" should be "${CTX:customerId}"
    And the JSON response field "list[0].billing.subscriptionId" should be "sub-${CTX:customerId}"

    # A renewal fails: past due is still served while the payment is retried.
    When the API Portal sends the "subscription.billing_updated" event to the control plane with data:
      """
      {"subscriber_id":"e2e-subscriber","api":{"ref_id":"${CTX:apiHandle}","type":"RestApi"},"billing":{"status":"PAST_DUE","customer_id":"${CTX:customerId}","subscription_id":"sub-${CTX:customerId}"}}
      """
    Then the response status code should be 200
    When I set header "API-Key" to "${CTX:apiKeyValue}"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    And I send a "GET" request to "${CTX:apiContext}/"
    Then the response status code should be 200

    # The retries fail: unpaid is refused at the gateway.
    When I clear all headers
    And the API Portal sends the "subscription.billing_updated" event to the control plane with data:
      """
      {"subscriber_id":"e2e-subscriber","api":{"ref_id":"${CTX:apiHandle}","type":"RestApi"},"billing":{"status":"UNPAID","customer_id":"${CTX:customerId}","subscription_id":"sub-${CTX:customerId}"}}
      """
    Then the response status code should be 200
    When I set header "API-Key" to "${CTX:apiKeyValue}"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    Then I send a "GET" request to "${CTX:apiContext}/" until status 403

    # A report older than the unpaid state arrives late and changes nothing.
    When I clear all headers
    And the API Portal sends the "subscription.billing_updated" event dated "2020-01-01T00:00:00Z" to the control plane with data:
      """
      {"subscriber_id":"e2e-subscriber","api":{"ref_id":"${CTX:apiHandle}","type":"RestApi"},"billing":{"status":"ACTIVE","customer_id":"${CTX:customerId}","subscription_id":"sub-${CTX:customerId}"}}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/subscriptions?artifactId=${CTX:apiHandle}&subscriberId=e2e-subscriber"
    Then the JSON response field "list[0].billing.status" should be "UNPAID"

    # The customer pays: served again.
    When the API Portal sends the "subscription.billing_updated" event to the control plane with data:
      """
      {"subscriber_id":"e2e-subscriber","api":{"ref_id":"${CTX:apiHandle}","type":"RestApi"},"billing":{"status":"ACTIVE","customer_id":"${CTX:customerId}","subscription_id":"sub-${CTX:customerId}"}}
      """
    Then the response status code should be 200
    When I set header "API-Key" to "${CTX:apiKeyValue}"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    Then I send a "GET" request to "${CTX:apiContext}/" until status 200

    # The paid subscription is cancelled: refused again.
    When I clear all headers
    And the API Portal sends the "subscription.billing_updated" event to the control plane with data:
      """
      {"subscriber_id":"e2e-subscriber","api":{"ref_id":"${CTX:apiHandle}","type":"RestApi"},"billing":{"status":"CANCELLED","customer_id":"${CTX:customerId}","subscription_id":"sub-${CTX:customerId}"}}
      """
    Then the response status code should be 200
    When I set header "API-Key" to "${CTX:apiKeyValue}"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    Then I send a "GET" request to "${CTX:apiContext}/" until status 403

  Scenario: A billing report for a subscription the control plane doesn't have is refused
    Given I generate a unique resource name from "no-such-api" and store it as "apiHandle"
    When the API Portal sends the "subscription.billing_updated" event to the control plane with data:
      """
      {"subscriber_id":"e2e-subscriber","api":{"ref_id":"${CTX:apiHandle}","type":"RestApi"},"billing":{"status":"ACTIVE","customer_id":"cus-x","subscription_id":"sub-x"}}
      """
    Then the response status code should be 404

  # Gateway 1.2.0 and older don't take a subscription's billing identifiers from the control plane.
  @gateway-billing-identifiers
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

  # Known issue: the gateway pulls an API's subscriptions only on its first connection, and never
  # from an on-prem control plane (gateway-controller pkg/controlplane/client.go
  # syncSubscriptionsForExistingAPIs). A subscription made before its API is deployed - or an API
  # deployed to another gateway later - never reaches that gateway, so a paying subscriber is refused.
  @known-issue
  Scenario: A paid subscription made before its API is deployed is served once the API is deployed
    Given I generate a unique resource name from "paid-pull-plan" and store it as "planHandle"
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id":"${CTX:planHandle}","displayName":"Paid ${CTX:planHandle}","status":"ACTIVE","limits":[{"limitType":"REQUEST_COUNT","timeUnit":"HOUR","limitCount":10000}],"pricing":{"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"29"}}
      """
    Then the response status code should be 201
    Given I generate a unique resource name from "paid-pull-api" and store it as "apiHandle"
    And I generate a unique API context from "/paid-pull" and store it as "apiContext"
    And I generate a unique resource name from "cus" and store it as "customerId"
    When I create a secured REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" offering plan "${CTX:planHandle}"
    And I generate a unique resource name from "paid-pull-app" and store it as "appHandle"
    And I create an application "${CTX:appHandle}" via the control plane in project "${CTX:projectHandle}"
    And I create a subscription for REST API "${CTX:apiHandle}" application "${CTX:appHandle}" plan "${CTX:planHandle}" via the control plane and store the subscription token as "subscriptionToken"
    And the API Portal sends the "subscription.billing_updated" event to the control plane with data:
      """
      {"subscriber_id":"e2e-subscriber","api":{"ref_id":"${CTX:apiHandle}","type":"RestApi"},"billing":{"status":"ACTIVE","customer_id":"${CTX:customerId}","subscription_id":"sub-${CTX:customerId}"}}
      """
    Then the response status code should be 200
    When I deploy the "RestApi" "${CTX:apiHandle}" to the gateway via the control plane
    And I send a "GET" request to "${CTX:apiContext}/" until status 401
    And I generate a unique value from "paid-pull-key" and store it as "apiKeyValue"
    And I issue an API key "${CTX:apiKeyValue}" for REST API "${CTX:apiHandle}" via the control plane
    And I set header "API-Key" to "${CTX:apiKeyValue}"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    Then I send a "GET" request to "${CTX:apiContext}/" until status 200
