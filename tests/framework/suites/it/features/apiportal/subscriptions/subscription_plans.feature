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


@APIPORTAL @apiportal-subscription-plan
Feature: The limits of a subscription plan, and a plan that subscriptions still use
  As an API Portal administrator
  I want a plan to hold exactly the limits I give it, and to stay while anything uses it
  So that subscribers are never left on a plan that no longer exists

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml), IDs APIP-CORE-187..189, at the API level. Plan
  # create, update, delete and access are covered by APIP-CORE-049..057. Every refused create or
  # delete is followed by a read that shows what is stored.

  Background:
    Given I generate a unique resource name from "portal-limit-plan" and store it as "planId"

  # Workbook row APIP-023 step 2.
  @APIP-CORE-187
  Scenario Outline: A plan limit of <case> is accepted and stored as given
    Given a unique API Portal resource is created at "/subscription-plans" as "admin" with values and stored as "planId":
      | id          | ${CTX:planId}       |
      | displayName | Limit ${CTX:planId} |
      | limits      | <limits>            |
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 200
    And the JSON response array field "limits" should have 1 item
    And the JSON response array "limits" item with "limitType" equal to "<type>" should have "limitCount" equal to "<count>"
    And the JSON response array "limits" item with "limitType" equal to "<type>" should have "timeUnit" equal to "<timeUnit>"

    Examples:
      | case                  | limits                                                                   | type              | count | timeUnit |
      | -1, meaning unlimited | [{"limitType":"BANDWIDTH","limitCount":-1,"timeUnit":"DAY"}]            | BANDWIDTH         | -1    | DAY      |
      | no time window        | [{"limitType":"EVENT_COUNT","limitCount":250}]                          | EVENT_COUNT       | 250   | null     |
      | 100 per minute        | [{"limitType":"REQUEST_COUNT","limitCount":100,"timeUnit":"MINUTE"}]    | REQUEST_COUNT     | 100   | MINUTE   |
      | tokens per hour       | [{"limitType":"TOTAL_TOKEN_COUNT","limitCount":9000,"timeUnit":"HOUR"}] | TOTAL_TOKEN_COUNT | 9000  | HOUR     |
      | requests per day      | [{"limitType":"REQUEST_COUNT","limitCount":5000,"timeUnit":"DAY"}]      | REQUEST_COUNT     | 5000  | DAY      |
      | bandwidth per month   | [{"limitType":"BANDWIDTH","limitCount":2048,"timeUnit":"MONTH"}]        | BANDWIDTH         | 2048  | MONTH    |

  # Workbook row APIP-023 step 2.
  @APIP-CORE-188 @type:negative
  Scenario Outline: A plan limit with <problem> is refused and nothing is stored
    When I send an authenticated API Portal "POST" request to "/subscription-plans" as "admin" with JSON values:
      | id          | ${CTX:planId}         |
      | displayName | Refused ${CTX:planId} |
      | limits      | <limits>              |
    Then the response status code should be 400
    And the response body should contain "<field>"
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 404

    Examples:
      | problem               | limits                                                               | field      |
      | an unknown limit type | [{"limitType":"CPU_TIME","limitCount":10,"timeUnit":"MINUTE"}]      | limitType  |
      | an unknown time unit  | [{"limitType":"REQUEST_COUNT","limitCount":10,"timeUnit":"WEEK"}]   | timeUnit   |
      | a count of -2         | [{"limitType":"REQUEST_COUNT","limitCount":-2,"timeUnit":"MINUTE"}] | limitCount |

  # Workbook row APIP-023 step 4.
  @APIP-CORE-189 @type:negative
  Scenario: A free plan with a subscription can't be deleted or taken off its API, and the subscription keeps working
    Given a unique API Portal resource is created at "/subscription-plans" as "admin" with values and stored as "planId":
      | id          | ${CTX:planId}                                                       |
      | displayName | Free ${CTX:planId}                                                  |
      | limits      | [{"limitType":"REQUEST_COUNT","limitCount":100,"timeUnit":"MINUTE"}] |
    And a REST API is created in the API Portal and stored as "apiId"
    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with definition "none" with values:
      | id                | ${CTX:apiId}                            |
      | name              | ${CTX:apiId}                            |
      | subscriptionPlans | [{"id":"Gold"},{"id":"${CTX:planId}"}] |
    Then the response status code should be 200
    Given a developer subscription for API "${CTX:apiId}" using plan "${CTX:planId}" is created and stored as "subscriptionId"

    When I send an authenticated API Portal "DELETE" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 409
    And the JSON response field "code" should be "PLAN_IN_USE"
    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with definition "none" with values:
      | id                | ${CTX:apiId}    |
      | name              | ${CTX:apiId}    |
      | subscriptionPlans | [{"id":"Gold"}] |
    Then the response status code should be 409
    And the response body should contain "has subscriptions to this API"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the JSON response array "subscriptionPlans" should contain an item with "id" equal to "${CTX:planId}"
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the response status code should be 200
    And the JSON response field "status" should be "ACTIVE"
    And the JSON response field "subscriptionPlanName" should be "Free ${CTX:planId}"

    When I send an authenticated API Portal "DELETE" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the response status code should be 200
    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with definition "none" with values:
      | id                | ${CTX:apiId}    |
      | name              | ${CTX:apiId}    |
      | subscriptionPlans | [{"id":"Gold"}] |
    Then the response status code should be 200
    When I send an authenticated API Portal "DELETE" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 404
