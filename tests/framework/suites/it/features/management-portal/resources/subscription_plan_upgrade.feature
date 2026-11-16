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


@SUBPLAN @platform-api-subscription-plan-upgrade
Feature: Plans stored before plan conditions keep their limits after an upgrade
  As an API platform operator upgrading the control plane
  I want plans saved with a request limit by an earlier release to keep that limit
  So that subscribers are throttled as before, and editing a plan moves it to conditions

  # Scenarios SUBPLAN (scenarios/on-prem/SUBPLAN.yaml). Each scenario stores a plan in the database as
  # a release before plan conditions did (a row in subscription_plan_limits), then restarts the
  # control plane, as an upgrade does, so it finds that table. The block runs alone, on database
  # servers, because the scenarios write to the database and restart the control plane.
  Background:
    Given I authenticate using basic auth as "admin"
    And I generate a unique resource name from "upg-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  @SUBPLAN-007 @type:smoke
  Scenario: A plan's earlier request limit is read as its request-count condition after an upgrade
    Given I generate a unique resource name from "upg-gold" and store it as "gold"
    And I generate a unique resource name from "upg-silver" and store it as "silver"
    And the control plane database holds a subscription plan "${CTX:gold}" of project "${CTX:projectHandle}"'s organization as a release before plan conditions stored it, with a request limit of 1000 per "HOUR" that rejects requests over it
    And the control plane database holds a subscription plan "${CTX:silver}" of project "${CTX:projectHandle}"'s organization as a release before plan conditions stored it, with a request limit of 100 per "MINUTE" that allows requests over it
    And the control plane subscription_plan "${CTX:gold}" is registered for cleanup
    And the control plane subscription_plan "${CTX:silver}" is registered for cleanup
    When I restart the "platform-api" service
    And I send a "GET" request to the control plane at "/subscription-plans/${CTX:gold}" until the JSON field "id" is "${CTX:gold}"
    Then the response status code should be 200
    And the JSON response array field "conditions" should have 1 items
    And the JSON response field "conditions[0].id" should be "requests"
    And the JSON response field "conditions[0].type" should be "request-count"
    And the JSON response field "conditions[0].params.limits[0].count" should be 1000
    And the JSON response field "conditions[0].params.limits[0].timeAmount" should be 1
    And the JSON response field "conditions[0].params.limits[0].timeUnit" should be "HOUR"
    And the JSON response field "conditions[0].params.onExceed" should be "reject"
    And the JSON response field "limits[0].limitCount" should be 1000
    # A limit that let requests through past it reads as onExceed allow
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:silver}"
    Then the response status code should be 200
    And the JSON response field "conditions[0].params.limits[0].count" should be 100
    And the JSON response field "conditions[0].params.limits[0].timeUnit" should be "MINUTE"
    And the JSON response field "conditions[0].params.onExceed" should be "allow"
    # Reading changes nothing in the database
    And the control plane database should hold 1 earlier limit row for subscription plan "${CTX:gold}"
    And the control plane database should hold 0 condition rows for subscription plan "${CTX:gold}"

  @SUBPLAN-008
  Scenario: Editing a plan stored before plan conditions keeps its limit and moves it to conditions
    Given I generate a unique resource name from "upg-edit" and store it as "plan"
    And the control plane database holds a subscription plan "${CTX:plan}" of project "${CTX:projectHandle}"'s organization as a release before plan conditions stored it, with a request limit of 1000 per "HOUR" that rejects requests over it
    And the control plane subscription_plan "${CTX:plan}" is registered for cleanup
    When I restart the "platform-api" service
    And I send a "GET" request to the control plane at "/subscription-plans/${CTX:plan}" until the JSON field "id" is "${CTX:plan}"
    Then the response status code should be 200
    # A rename keeps the limit; the plan is now stored as a condition, and its earlier row is gone
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:plan}" with body:
      """
      {"id": "${CTX:plan}", "displayName": "Gold ${CTX:plan}", "status": "ACTIVE"}
      """
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Gold ${CTX:plan}"
    And the JSON response array field "conditions" should have 1 items
    And the JSON response field "conditions[0].params.limits[0].count" should be 1000
    And the JSON response field "conditions[0].params.limits[0].timeUnit" should be "HOUR"
    And the JSON response field "conditions[0].params.onExceed" should be "reject"
    And the control plane database should hold 0 earlier limit rows for subscription plan "${CTX:plan}"
    And the control plane database should hold 1 condition row for subscription plan "${CTX:plan}"
    # A client that predates conditions still replaces the request limit with "limits"
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:plan}" with body:
      """
      {"id": "${CTX:plan}", "displayName": "Gold ${CTX:plan}", "status": "ACTIVE", "limits": [{"limitType": "REQUEST_COUNT", "timeUnit": "MINUTE", "limitCount": 20}]}
      """
    Then the response status code should be 200
    And the JSON response array field "conditions" should have 1 items
    And the JSON response field "conditions[0].params.limits[0].count" should be 20
    And the JSON response field "conditions[0].params.limits[0].timeUnit" should be "MINUTE"
    # Conditions can now be added beside the request limit
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:plan}" with body:
      """
      {"id": "${CTX:plan}", "displayName": "Gold ${CTX:plan}", "status": "ACTIVE", "conditions": [
        {"type": "request-count", "params": {"limits": [{"count": 20, "timeUnit": "MINUTE"}]}},
        {"type": "token-count", "params": {"limits": [{"count": 50000, "timeUnit": "DAY"}]}}]}
      """
    Then the response status code should be 200
    And the JSON response array field "conditions" should have 2 items
    And the JSON response field "conditions[1].id" should be "tokens"
    And the control plane database should hold 2 condition rows for subscription plan "${CTX:plan}"
    # After a restart the plan is read from its conditions, not from the earlier table
    When I restart the "platform-api" service
    And I send a "GET" request to the control plane at "/subscription-plans/${CTX:plan}" until the JSON field "id" is "${CTX:plan}"
    Then the response status code should be 200
    And the JSON response array field "conditions" should have 2 items
    And the JSON response field "conditions[0].params.limits[0].count" should be 20
    # An empty "limits" removes the request limit and keeps the token condition
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:plan}" with body:
      """
      {"id": "${CTX:plan}", "displayName": "Gold ${CTX:plan}", "status": "ACTIVE", "limits": []}
      """
    Then the response status code should be 200
    And the JSON response array field "conditions" should have 1 items
    And the JSON response field "conditions[0].id" should be "tokens"
    And the JSON response array field "limits" should have 0 items

  @SUBPLAN-009 @secured
  Scenario: A plan stored before plan conditions throttles its subscribers on the gateway after an upgrade
    Given the gateway services are running
    And I generate a unique resource name from "upg-enf-plan" and store it as "planHandle"
    And the control plane database holds a subscription plan "${CTX:planHandle}" of project "${CTX:projectHandle}"'s organization as a release before plan conditions stored it, with a request limit of 3 per "MINUTE" that rejects requests over it
    And the control plane subscription_plan "${CTX:planHandle}" is registered for cleanup
    When I restart the "platform-api" service
    And I send a "GET" request to the control plane at "/subscription-plans/${CTX:planHandle}" until the JSON field "id" is "${CTX:planHandle}"
    Then the response status code should be 200
    And I generate a unique resource name from "upg-enf-api" and store it as "apiHandle"
    And I generate a unique API context from "/upg-enf" and store it as "apiContext"
    When I create a secured REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" offering plan "${CTX:planHandle}"
    And I deploy the "RestApi" "${CTX:apiHandle}" to the gateway via the control plane
    Then I send a "GET" request to "${CTX:apiContext}/" until status 401

    When I generate a unique resource name from "upg-enf-app" and store it as "appHandle"
    And I create an application "${CTX:appHandle}" via the control plane in project "${CTX:projectHandle}"
    And I create a subscription for REST API "${CTX:apiHandle}" application "${CTX:appHandle}" plan "${CTX:planHandle}" via the control plane and store the subscription token as "subscriptionToken"
    And I generate a unique value from "upg-enf-key" and store it as "apiKeyValue"
    And I issue an API key "${CTX:apiKeyValue}" for REST API "${CTX:apiHandle}" via the control plane

    # The earlier limit, 3 a minute, reaches the gateway: the 4th request in a minute is throttled
    When I set header "API-Key" to "${CTX:apiKeyValue}"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    And I send a "GET" request to "${CTX:apiContext}/" until status 200
    When I send 2 "GET" requests to "${CTX:apiContext}/"
    Then the response status code should be 200
    When I send a "GET" request to "${CTX:apiContext}/"
    Then the response status code should be 429
