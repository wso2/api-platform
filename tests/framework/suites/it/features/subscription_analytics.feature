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

@subscription-analytics
Feature: Subscription analytics monetized billing metadata
  As a platform administrator
  I want billing fields from a monetized subscription to appear in analytics events
  So that usage can be tracked and correlated with billing records
  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Monetized subscription billing fields appear in analytics events
    Given I generate a unique value from "subscription-analytics-plan" and store it as "planName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:planName}","throttleLimitCount":100,"throttleLimitUnit":"Min"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "planId"

    Given I generate a unique value from "subscription-analytics" and store it as "apiName"
    And I generate a unique API context from "/subscription-analytics" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion              | gateway.api-platform.wso2.com/v1 |
      | name                    | ${CTX:apiName}                    |
      | spec.displayName        | ${CTX:apiName}                    |
      | spec.version            | v1.0                                |
      | spec.context            | ${CTX:apiContext}/$version          |
      | spec.subscriptionPlans  | ["${CTX:planName}"]                 |
      | spec.upstream.main.url  | http://testbench:3002                |
      | spec.operations         | [{"method":"GET","path":"/health","policies":[{"name":"subscription-validation","version":"v1","params":{"subscriptionKeyHeader":"Subscription-Key"}}]}] |
    Then the response should be successful

    # A fresh route protected by subscription-validation rejects an unrecognized key with 403 -
    # this doubles as the route-readiness fold, exactly as the plain-200 idiom does elsewhere.
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 403

    And I reset the analytics collector

    Given I generate a unique value from "subscription-analytics-token" and store it as "subToken"
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"apiId":"${CTX:apiName}","subscriptionToken":"${CTX:subToken}","subscriptionPlanId":"${CTX:planId}","billingCustomerId":"cust-123","billingSubscriptionId":"billing-sub-456"}
      """
    Then the response status should be 201

    When I set header "Subscription-Key" to "${CTX:subToken}"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200

    And the analytics collector should have received at least 1 event
    And the latest analytics event for path "${CTX:apiContext}/v1.0/health" should have metadata field "billingCustomerId" with value "cust-123"
    And the latest analytics event for path "${CTX:apiContext}/v1.0/health" should have metadata field "billingSubscriptionId" with value "billing-sub-456"
    And the latest analytics event for path "${CTX:apiContext}/v1.0/health" should have metadata field "subscriptionStatus" with value "ACTIVE"
    And the latest analytics event for path "${CTX:apiContext}/v1.0/health" should have metadata field "subscriptionPlanName" with value "${CTX:planName}"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    When I send a "DELETE" request to the "gateway-controller" service at "/subscription-plans/${CTX:planId}"
    Then the response should be successful
