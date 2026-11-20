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


@APIPORTAL @apiportal-subscription-plan-upgrade
Feature: Plans stored before plan conditions keep their limits after an API Portal upgrade
  As an API Portal administrator upgrading the portal
  I want the limits an earlier release stored for a plan to read as its conditions
  So that subscribers see and keep the same plan, and saving the plan moves it to conditions

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml). Each scenario stores a plan's limits as a
  # release before plan conditions did (rows in subscription_plan_limits, no condition rows), then
  # restarts the portal, as an upgrade does, so it finds that table. EVENT_COUNT and BANDWIDTH rows
  # have no condition: they are not shown and are left as they are. The block runs alone, on
  # database servers, because the scenarios write to the database and restart the portal.

  @APIP-058 @type:smoke
  Scenario: A plan's earlier limits read as its conditions after an upgrade, and a rename keeps them where they are
    Given I generate a unique resource name from "portal-upg-plan" and store it as "planId"
    And a unique API Portal resource is created at "/subscription-plans" as "admin" with values and stored as "planId":
      | id          | ${CTX:planId}      |
      | displayName | Gold ${CTX:planId} |
      | conditions  | []                 |
    Then the response status code should be 201
    And the API Portal "api-portal" database holds subscription plan "${CTX:planId}" as a release before plan conditions stored it, with limits:
      | limit_type        | time_unit | time_amount | limit_count |
      | REQUEST_COUNT     | DAY       | 1           | 5000        |
      | REQUEST_COUNT     | MINUTE    | 1           | 100         |
      | TOTAL_TOKEN_COUNT | HOUR      | 1           | 20000       |
      | EVENT_COUNT       | MINUTE    | 1           | 50          |
      | BANDWIDTH         | MONTH     | 1           | 1024        |
    When I stop the API Portal
    And I start the API Portal and wait until it is ready
    And I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 200
    # Request limits, shortest window first, and total token limits become conditions
    And the JSON response array field "conditions" should have 2 items
    And the JSON response field "conditions[0].id" should be "requests"
    And the JSON response array field "conditions[0].params.limits" should have 2 items
    And the JSON response field "conditions[0].params.limits[0].count" should be 100
    And the JSON response field "conditions[0].params.limits[0].timeUnit" should be "MINUTE"
    And the JSON response field "conditions[0].params.limits[1].count" should be 5000
    And the JSON response field "conditions[0].params.limits[1].timeUnit" should be "DAY"
    And the JSON response field "conditions[1].id" should be "tokens"
    And the JSON response field "conditions[1].params.limits[0].count" should be 20000
    And the JSON response field "conditions[1].params.tokenType" should be "total"
    # "limits" still shows the request limits to clients that predate conditions
    And the JSON response array field "limits" should have 2 items

    # A rename sends neither conditions nor limits: the plan stays as it was stored
    When I send an authenticated API Portal "PUT" request to "/subscription-plans" as "admin" with JSON values:
      | id          | ${CTX:planId}           |
      | displayName | Gold plus ${CTX:planId} |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the JSON response field "displayName" should be "Gold plus ${CTX:planId}"
    And the JSON response array field "conditions" should have 2 items
    And the API Portal "api-portal" database should hold 0 condition rows for subscription plan "${CTX:planId}"
    And the API Portal "api-portal" database should hold these earlier limit rows for subscription plan "${CTX:planId}":
      | limit_type        | time_unit | time_amount | limit_count |
      | REQUEST_COUNT     | DAY       | 1           | 5000        |
      | REQUEST_COUNT     | MINUTE    | 1           | 100         |
      | TOTAL_TOKEN_COUNT | HOUR      | 1           | 20000       |
      | EVENT_COUNT       | MINUTE    | 1           | 50          |
      | BANDWIDTH         | MONTH     | 1           | 1024        |

  @APIP-059
  Scenario: Saving the limits of a plan stored before plan conditions moves it to conditions and keeps its other rows
    Given I generate a unique resource name from "portal-upg-save" and store it as "planId"
    And a unique API Portal resource is created at "/subscription-plans" as "admin" with values and stored as "planId":
      | id          | ${CTX:planId}        |
      | displayName | Silver ${CTX:planId} |
      | conditions  | []                   |
    Then the response status code should be 201
    And the API Portal "api-portal" database holds subscription plan "${CTX:planId}" as a release before plan conditions stored it, with limits:
      | limit_type        | time_unit | time_amount | limit_count |
      | REQUEST_COUNT     | MINUTE    | 1           | 100         |
      | TOTAL_TOKEN_COUNT | HOUR      | 1           | 20000       |
      | EVENT_COUNT       | MINUTE    | 1           | 50          |
      | BANDWIDTH         | MONTH     | 1           | 1024        |
    When I stop the API Portal
    And I start the API Portal and wait until it is ready
    # A client that predates conditions saves its request limits: the plan moves to conditions
    When I send an authenticated API Portal "PUT" request to "/subscription-plans" as "admin" with JSON values:
      | id          | ${CTX:planId}                                                       |
      | displayName | Silver ${CTX:planId}                                                |
      | limits      | [{"limitType":"REQUEST_COUNT","limitCount":40,"timeUnit":"HOUR"}]   |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 200
    And the JSON response array field "conditions" should have 2 items
    And the JSON response field "conditions[0].id" should be "requests"
    And the JSON response array field "conditions[0].params.limits" should have 1 item
    And the JSON response field "conditions[0].params.limits[0].count" should be 40
    And the JSON response field "conditions[0].params.limits[0].timeUnit" should be "HOUR"
    # The token limit is kept as a condition
    And the JSON response field "conditions[1].id" should be "tokens"
    And the JSON response field "conditions[1].params.limits[0].count" should be 20000
    And the API Portal "api-portal" database should hold 2 condition rows for subscription plan "${CTX:planId}"
    # Rows with no condition stay; the rows conditions now hold are gone
    And the API Portal "api-portal" database should hold these earlier limit rows for subscription plan "${CTX:planId}":
      | limit_type  | time_unit | time_amount | limit_count |
      | EVENT_COUNT | MINUTE    | 1           | 50          |
      | BANDWIDTH   | MONTH     | 1           | 1024        |
    # After another restart the plan is read from its conditions
    When I stop the API Portal
    And I start the API Portal and wait until it is ready
    And I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the JSON response array field "conditions" should have 2 items
    And the JSON response field "conditions[0].params.limits[0].count" should be 40
