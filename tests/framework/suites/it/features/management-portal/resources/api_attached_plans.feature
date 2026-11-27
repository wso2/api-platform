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


@SUBPLAN @platform-api-attached-plans
Feature: The subscription plans attached to an API are kept with their details and lock the plans they use
  As an API publisher
  I want the plans I attach to an API to be listed with their names and whether they changed since
  So that I know what subscribers are offered, and an attached plan cannot change under them

  # Scenarios SUBPLAN (scenarios/on-prem/SUBPLAN.yaml).
  Background:
    Given I generate a unique resource name from "att-proj" and store it as "project"
    And I generate a unique resource name from "att-gold" and store it as "gold"
    And I generate a unique resource name from "att-silver" and store it as "silver"
    And I generate a unique resource name from "att-api" and store it as "apiName"
    And I generate a unique API context from "/att-api" and store it as "apiContext"
    And I create a project "${CTX:project}" on the control plane
    And I create a subscription plan "${CTX:gold}" allowing 100 requests per minute via the control plane
    And I create a subscription plan "${CTX:silver}" allowing 10 requests per minute via the control plane

  @SUBPLAN-004 @type:smoke
  Scenario: Plans attached by handle or by reference are returned with their names and status, and count as in use
    When I send a "POST" request to the control plane at "/rest-apis" with body:
      """
      {"displayName": "${CTX:apiName}", "context": "${CTX:apiContext}", "version": "v1", "projectId": "${CTX:project}", "upstream": {"main": {"url": "http://backend.example.com"}}, "subscriptionPlans": ["${CTX:gold}", {"id": "${CTX:silver}", "displayName": "ignored", "sourceStatus": "DELETED"}]}
      """
    Then the response status code should be 201
    And I store the JSON response field "id" as "api"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}"
    Then the response status code should be 200
    And the JSON response array field "subscriptionPlans" should have 2 items
    # displayName and sourceStatus are read-only: what was sent is ignored
    And the JSON response array "subscriptionPlans" item with "id" equal to "${CTX:gold}" should have "sourceStatus" equal to "UNCHANGED"
    And the JSON response array "subscriptionPlans" item with "id" equal to "${CTX:silver}" should have "displayName" equal to "${CTX:silver}"
    And the JSON response array "subscriptionPlans" item with "id" equal to "${CTX:silver}" should have "sourceStatus" equal to "UNCHANGED"
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:gold}"
    Then the JSON response field "usage.inUse" should be true
    And the JSON response field "usage.restApis" should be 1

    # A plan in use keeps its name and conditions, but its description can change
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:gold}" with body:
      """
      {"id": "${CTX:gold}", "displayName": "Renamed ${CTX:gold}", "status": "ACTIVE"}
      """
    Then the response status code should be 409
    And the JSON response field "code" should be "SUBSCRIPTION_PLAN_IN_USE"
    And the JSON response field "message" should be "The plan ${CTX:gold} is in use. To change its name or conditions, create a new plan."
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:gold}" with body:
      """
      {"id": "${CTX:gold}", "displayName": "${CTX:gold}", "status": "ACTIVE", "conditions": [{"type": "request-count", "params": {"limits": [{"count": 5, "timeUnit": "MINUTE"}]}}]}
      """
    Then the response status code should be 409
    And the JSON response field "code" should be "SUBSCRIPTION_PLAN_IN_USE"
    When I send a "DELETE" request to the control plane at "/subscription-plans/${CTX:gold}"
    Then the response status code should be 409
    And the JSON response field "message" should be "The plan ${CTX:gold} is in use and can't be deleted."
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:gold}" with body:
      """
      {"id": "${CTX:gold}", "displayName": "${CTX:gold}", "status": "ACTIVE", "description": "Now with a description"}
      """
    Then the response status code should be 200

    # The plan changed after it was attached: UPDATED until the API is saved again
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}"
    Then the JSON response array "subscriptionPlans" item with "id" equal to "${CTX:gold}" should have "sourceStatus" equal to "UPDATED"
    And the JSON response array "subscriptionPlans" item with "id" equal to "${CTX:silver}" should have "sourceStatus" equal to "UNCHANGED"
    When I send a "PUT" request to the control plane at "/rest-apis/${CTX:api}" with body:
      """
      {"displayName": "${CTX:apiName}", "context": "${CTX:apiContext}", "version": "v1", "projectId": "${CTX:project}", "upstream": {"main": {"url": "http://backend.example.com"}}, "subscriptionPlans": [{"id": "${CTX:gold}"}]}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}"
    Then the JSON response array field "subscriptionPlans" should have 1 item
    And the JSON response field "subscriptionPlans[0].sourceStatus" should be "UNCHANGED"

    # A detached plan is free again: it can be renamed
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:silver}"
    Then the JSON response field "usage.inUse" should be false
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:silver}" with body:
      """
      {"id": "${CTX:silver}", "displayName": "Renamed ${CTX:silver}", "status": "ACTIVE"}
      """
    Then the response status code should be 200

    # Removing the API frees the last plan
    When I send a "DELETE" request to the control plane at "/rest-apis/${CTX:api}"
    Then the response status code should be 204
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:gold}"
    Then the JSON response field "usage.inUse" should be false

  @SUBPLAN-005 @type:negative
  Scenario: Attaching a plan that is inactive, unknown or blank is refused and the API keeps its plans
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:silver}" with body:
      """
      {"id": "${CTX:silver}", "displayName": "${CTX:silver}", "status": "INACTIVE"}
      """
    Then the response status code should be 200
    When I send a "POST" request to the control plane at "/rest-apis" with body:
      """
      {"displayName": "${CTX:apiName}", "context": "${CTX:apiContext}", "version": "v1", "projectId": "${CTX:project}", "upstream": {"main": {"url": "http://backend.example.com"}}, "subscriptionPlans": [{"id": "${CTX:silver}"}]}
      """
    Then the response status code should be 400
    And the response body should contain "was not found or is not active."
    When I send a "POST" request to the control plane at "/rest-apis" with body:
      """
      {"displayName": "${CTX:apiName}", "context": "${CTX:apiContext}", "version": "v1", "projectId": "${CTX:project}", "upstream": {"main": {"url": "http://backend.example.com"}}, "subscriptionPlans": ["${CTX:gold}"]}
      """
    Then the response status code should be 201
    And I store the JSON response field "id" as "api"
    When I send a "PUT" request to the control plane at "/rest-apis/${CTX:api}" with body:
      """
      {"displayName": "${CTX:apiName}", "context": "${CTX:apiContext}", "version": "v1", "projectId": "${CTX:project}", "upstream": {"main": {"url": "http://backend.example.com"}}, "subscriptionPlans": [{"id": "${CTX:gold}"}, {"id": "no-such-plan-${CTX:gold}"}]}
      """
    Then the response status code should be 400
    And the response body should contain "was not found or is not active."
    When I send a "PUT" request to the control plane at "/rest-apis/${CTX:api}" with body:
      """
      {"displayName": "${CTX:apiName}", "context": "${CTX:apiContext}", "version": "v1", "projectId": "${CTX:project}", "upstream": {"main": {"url": "http://backend.example.com"}}, "subscriptionPlans": ["  "]}
      """
    Then the response status code should be 400
    And the JSON response field "message" should be "subscriptionPlans must name at least one plan; leave it out to offer every active plan"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:api}"
    Then the JSON response array field "subscriptionPlans" should have 1 item
    And the JSON response field "subscriptionPlans[0].id" should be "${CTX:gold}"
    When I send a "DELETE" request to the control plane at "/rest-apis/${CTX:api}"
    Then the response status code should be 204
