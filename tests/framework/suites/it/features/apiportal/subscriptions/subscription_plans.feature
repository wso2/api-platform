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

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml) at the API level. Plan
  # create, update, delete and access are covered by this feature's tests. Every refused create or
  # delete is followed by a read that shows what is stored.

  @APIP-023 @type:smoke
  Scenario: An administrator manages plans and their limits
    # An administrator manages a free plan with limits from creation to deletion
    Given I generate a unique resource name from "portal-plan" and store it as "planId"
    And I generate a unique resource name from "portal-calls" and store it as "meterId"
    And a unique API Portal resource is created at "/meters" as "admin" with values and stored as "meterId":
      | id           | ${CTX:meterId}                                                                                  |
      | displayName  | API calls ${CTX:meterId}                                                                        |
      | type         | REQUEST_COUNT                                                                                   |
      | filterFields | [{"field":"Status","eventKey":"response.status"},{"field":"Method","eventKey":"request.verb"}] |
    Given a unique API Portal resource is created at "/subscription-plans" as "admin" with values and stored as "planId":
      | id          | ${CTX:planId}                                                                                                                      |
      | displayName | Team ${CTX:planId}                                                                                                                 |
      | refId       | team-plan-${CTX:planId}                                                                                                            |
      | limits      | [{"limitType":"REQUEST_COUNT","limitCount":1000,"timeUnit":"HOUR"},{"limitType":"BANDWIDTH","limitCount":-1,"timeUnit":"MONTH"}] |
    Then the response status code should be 201
    And the JSON response field "paid" should be false
    And the JSON response array field "limits" should have 2 items
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "refId" should be "team-plan-${CTX:planId}"
    And the JSON response array "limits" item with "limitType" equal to "BANDWIDTH" should have "limitCount" equal to "-1"
    And the JSON response array "limits" item with "limitType" equal to "REQUEST_COUNT" should have "timeUnit" equal to "HOUR"

    When I send an authenticated API Portal "PUT" request to "/subscription-plans" as "admin" with JSON values:
      | id          | ${CTX:planId}                                                          |
      | displayName | Team plus ${CTX:planId}                                                |
      | limits      | [{"limitType":"TOTAL_TOKEN_COUNT","limitCount":50000,"timeUnit":"DAY"}] |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the JSON response field "displayName" should be "Team plus ${CTX:planId}"
    And the JSON response array field "limits" should have 1 item
    And the JSON response field "limits[0].limitType" should be "TOTAL_TOKEN_COUNT"

    When I send an authenticated API Portal "DELETE" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 404

    # A plan created without an id gets a generated one
    Given I generate a unique resource name from "portal-plan" and store it as "planId"
    And I generate a unique resource name from "portal-calls" and store it as "meterId"
    And a unique API Portal resource is created at "/meters" as "admin" with values and stored as "meterId":
      | id           | ${CTX:meterId}                                                                                  |
      | displayName  | API calls ${CTX:meterId}                                                                        |
      | type         | REQUEST_COUNT                                                                                   |
      | filterFields | [{"field":"Status","eventKey":"response.status"},{"field":"Method","eventKey":"request.verb"}] |
    Given a unique API Portal resource is created at "/subscription-plans" as "admin" with values and stored as "generatedPlanId":
      | displayName | Generated ${CTX:planId}                                                  |
      | limits      | [{"limitType":"EVENT_COUNT","limitCount":500,"timeUnit":"MINUTE"}]      |
    Then the response status code should be 201
    And the JSON response field "id" should match pattern "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:generatedPlanId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Generated ${CTX:planId}"

    # A plan limit of -1, meaning unlimited is accepted and stored as given
    Given I generate a unique resource name from "portal-limit-plan" and store it as "planId"
    Given a unique API Portal resource is created at "/subscription-plans" as "admin" with values and stored as "planId":
      | id          | ${CTX:planId}       |
      | displayName | Limit ${CTX:planId} |
      | limits      | [{"limitType":"BANDWIDTH","limitCount":-1,"timeUnit":"DAY"}]            |
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 200
    And the JSON response array field "limits" should have 1 item
    And the JSON response array "limits" item with "limitType" equal to "BANDWIDTH" should have "limitCount" equal to "-1"
    And the JSON response array "limits" item with "limitType" equal to "BANDWIDTH" should have "timeUnit" equal to "DAY"

    # A plan limit of no time window is accepted and stored as given
    Given I generate a unique resource name from "portal-limit-plan" and store it as "planId"
    Given a unique API Portal resource is created at "/subscription-plans" as "admin" with values and stored as "planId":
      | id          | ${CTX:planId}       |
      | displayName | Limit ${CTX:planId} |
      | limits      | [{"limitType":"EVENT_COUNT","limitCount":250}]            |
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 200
    And the JSON response array field "limits" should have 1 item
    And the JSON response array "limits" item with "limitType" equal to "EVENT_COUNT" should have "limitCount" equal to "250"
    And the JSON response array "limits" item with "limitType" equal to "EVENT_COUNT" should have "timeUnit" equal to "null"

    # A plan limit of 100 per minute is accepted and stored as given
    Given I generate a unique resource name from "portal-limit-plan" and store it as "planId"
    Given a unique API Portal resource is created at "/subscription-plans" as "admin" with values and stored as "planId":
      | id          | ${CTX:planId}       |
      | displayName | Limit ${CTX:planId} |
      | limits      | [{"limitType":"REQUEST_COUNT","limitCount":100,"timeUnit":"MINUTE"}]            |
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 200
    And the JSON response array field "limits" should have 1 item
    And the JSON response array "limits" item with "limitType" equal to "REQUEST_COUNT" should have "limitCount" equal to "100"
    And the JSON response array "limits" item with "limitType" equal to "REQUEST_COUNT" should have "timeUnit" equal to "MINUTE"

    # A plan limit of tokens per hour is accepted and stored as given
    Given I generate a unique resource name from "portal-limit-plan" and store it as "planId"
    Given a unique API Portal resource is created at "/subscription-plans" as "admin" with values and stored as "planId":
      | id          | ${CTX:planId}       |
      | displayName | Limit ${CTX:planId} |
      | limits      | [{"limitType":"TOTAL_TOKEN_COUNT","limitCount":9000,"timeUnit":"HOUR"}]            |
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 200
    And the JSON response array field "limits" should have 1 item
    And the JSON response array "limits" item with "limitType" equal to "TOTAL_TOKEN_COUNT" should have "limitCount" equal to "9000"
    And the JSON response array "limits" item with "limitType" equal to "TOTAL_TOKEN_COUNT" should have "timeUnit" equal to "HOUR"

    # A plan limit of requests per day is accepted and stored as given
    Given I generate a unique resource name from "portal-limit-plan" and store it as "planId"
    Given a unique API Portal resource is created at "/subscription-plans" as "admin" with values and stored as "planId":
      | id          | ${CTX:planId}       |
      | displayName | Limit ${CTX:planId} |
      | limits      | [{"limitType":"REQUEST_COUNT","limitCount":5000,"timeUnit":"DAY"}]            |
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 200
    And the JSON response array field "limits" should have 1 item
    And the JSON response array "limits" item with "limitType" equal to "REQUEST_COUNT" should have "limitCount" equal to "5000"
    And the JSON response array "limits" item with "limitType" equal to "REQUEST_COUNT" should have "timeUnit" equal to "DAY"

    # A plan limit of bandwidth per month is accepted and stored as given
    Given I generate a unique resource name from "portal-limit-plan" and store it as "planId"
    Given a unique API Portal resource is created at "/subscription-plans" as "admin" with values and stored as "planId":
      | id          | ${CTX:planId}       |
      | displayName | Limit ${CTX:planId} |
      | limits      | [{"limitType":"BANDWIDTH","limitCount":2048,"timeUnit":"MONTH"}]            |
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 200
    And the JSON response array field "limits" should have 1 item
    And the JSON response array "limits" item with "limitType" equal to "BANDWIDTH" should have "limitCount" equal to "2048"
    And the JSON response array "limits" item with "limitType" equal to "BANDWIDTH" should have "timeUnit" equal to "MONTH"

  @APIP-023 @type:negative @type:security
  Scenario: Invalid plans are refused, plans are read-only for other roles, and a plan in use cannot be deleted
    # A plan whose limit count is zero is refused with the reason and nothing is stored
    Given I generate a unique resource name from "portal-plan" and store it as "planId"
    And I generate a unique resource name from "portal-calls" and store it as "meterId"
    And a unique API Portal resource is created at "/meters" as "admin" with values and stored as "meterId":
      | id           | ${CTX:meterId}                                                                                  |
      | displayName  | API calls ${CTX:meterId}                                                                        |
      | type         | REQUEST_COUNT                                                                                   |
      | filterFields | [{"field":"Status","eventKey":"response.status"},{"field":"Method","eventKey":"request.verb"}] |
    When I send an authenticated API Portal "POST" request to "/subscription-plans" as "admin" with JSON values:
      | id          | ${CTX:planId}         |
      | displayName | Refused ${CTX:planId} |
      | limits      | [{"limitType":"REQUEST_COUNT","limitCount":0,"timeUnit":"MINUTE"}]              |
    Then the response status code should be 400
    And the JSON response field "errors[0].message" should be "limitCount must be -1 (unlimited) or a positive number for each limit"
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 404

    # A plan whose time amount is zero is refused with the reason and nothing is stored
    Given I generate a unique resource name from "portal-plan" and store it as "planId"
    And I generate a unique resource name from "portal-calls" and store it as "meterId"
    And a unique API Portal resource is created at "/meters" as "admin" with values and stored as "meterId":
      | id           | ${CTX:meterId}                                                                                  |
      | displayName  | API calls ${CTX:meterId}                                                                        |
      | type         | REQUEST_COUNT                                                                                   |
      | filterFields | [{"field":"Status","eventKey":"response.status"},{"field":"Method","eventKey":"request.verb"}] |
    When I send an authenticated API Portal "POST" request to "/subscription-plans" as "admin" with JSON values:
      | id          | ${CTX:planId}         |
      | displayName | Refused ${CTX:planId} |
      | limits      | [{"limitType":"REQUEST_COUNT","limitCount":10,"timeUnit":"MINUTE","timeAmount":0}]              |
    Then the response status code should be 400
    And the JSON response field "errors[0].message" should be "timeAmount must be a positive number when provided"
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 404

    # A plan limit with an unknown limit type is refused and nothing is stored
    Given I generate a unique resource name from "portal-limit-plan" and store it as "planId"
    When I send an authenticated API Portal "POST" request to "/subscription-plans" as "admin" with JSON values:
      | id          | ${CTX:planId}         |
      | displayName | Refused ${CTX:planId} |
      | limits      | [{"limitType":"CPU_TIME","limitCount":10,"timeUnit":"MINUTE"}]              |
    Then the response status code should be 400
    And the response body should contain "limitType"
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 404

    # A plan limit with an unknown time unit is refused and nothing is stored
    Given I generate a unique resource name from "portal-limit-plan" and store it as "planId"
    When I send an authenticated API Portal "POST" request to "/subscription-plans" as "admin" with JSON values:
      | id          | ${CTX:planId}         |
      | displayName | Refused ${CTX:planId} |
      | limits      | [{"limitType":"REQUEST_COUNT","limitCount":10,"timeUnit":"WEEK"}]              |
    Then the response status code should be 400
    And the response body should contain "timeUnit"
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 404

    # A plan limit with a count of -2 is refused and nothing is stored
    Given I generate a unique resource name from "portal-limit-plan" and store it as "planId"
    When I send an authenticated API Portal "POST" request to "/subscription-plans" as "admin" with JSON values:
      | id          | ${CTX:planId}         |
      | displayName | Refused ${CTX:planId} |
      | limits      | [{"limitType":"REQUEST_COUNT","limitCount":-2,"timeUnit":"MINUTE"}]              |
    Then the response status code should be 400
    And the response body should contain "limitCount"
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 404

    # A publisher can read plans but cannot create, change or delete them
    Given I generate a unique resource name from "portal-plan" and store it as "planId"
    And I generate a unique resource name from "portal-calls" and store it as "meterId"
    And a unique API Portal resource is created at "/meters" as "admin" with values and stored as "meterId":
      | id           | ${CTX:meterId}                                                                                  |
      | displayName  | API calls ${CTX:meterId}                                                                        |
      | type         | REQUEST_COUNT                                                                                   |
      | filterFields | [{"field":"Status","eventKey":"response.status"},{"field":"Method","eventKey":"request.verb"}] |
    Given a unique API Portal resource is created at "/subscription-plans" as "admin" with values and stored as "planId":
      | id          | ${CTX:planId}                                                       |
      | displayName | Shared ${CTX:planId}                                                |
      | limits      | [{"limitType":"REQUEST_COUNT","limitCount":100,"timeUnit":"DAY"}]  |
    And I generate a unique resource name from "portal-denied-plan" and store it as "deniedPlanId"
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "POST" request to "/subscription-plans" as "publisher" with JSON values:
      | id          | ${CTX:deniedPlanId} |
      | displayName | Denied plan         |
    Then the response status code should be 403
    When I send an authenticated API Portal "PUT" request to "/subscription-plans" as "publisher" with JSON values:
      | id          | ${CTX:planId} |
      | displayName | Renamed plan  |
    Then the response status code should be 403
    When I send an authenticated API Portal "DELETE" request to "/subscription-plans/${CTX:planId}" as "publisher"
    Then the response status code should be 403
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Shared ${CTX:planId}"
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:deniedPlanId}" as "admin"
    Then the response status code should be 404

    # A developer can read plans but cannot create, change or delete them
    Given I generate a unique resource name from "portal-plan" and store it as "planId"
    And I generate a unique resource name from "portal-calls" and store it as "meterId"
    And a unique API Portal resource is created at "/meters" as "admin" with values and stored as "meterId":
      | id           | ${CTX:meterId}                                                                                  |
      | displayName  | API calls ${CTX:meterId}                                                                        |
      | type         | REQUEST_COUNT                                                                                   |
      | filterFields | [{"field":"Status","eventKey":"response.status"},{"field":"Method","eventKey":"request.verb"}] |
    Given a unique API Portal resource is created at "/subscription-plans" as "admin" with values and stored as "planId":
      | id          | ${CTX:planId}                                                       |
      | displayName | Shared ${CTX:planId}                                                |
      | limits      | [{"limitType":"REQUEST_COUNT","limitCount":100,"timeUnit":"DAY"}]  |
    And I generate a unique resource name from "portal-denied-plan" and store it as "deniedPlanId"
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "developer"
    Then the response status code should be 200
    When I send an authenticated API Portal "POST" request to "/subscription-plans" as "developer" with JSON values:
      | id          | ${CTX:deniedPlanId} |
      | displayName | Denied plan         |
    Then the response status code should be 403
    When I send an authenticated API Portal "PUT" request to "/subscription-plans" as "developer" with JSON values:
      | id          | ${CTX:planId} |
      | displayName | Renamed plan  |
    Then the response status code should be 403
    When I send an authenticated API Portal "DELETE" request to "/subscription-plans/${CTX:planId}" as "developer"
    Then the response status code should be 403
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Shared ${CTX:planId}"
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:deniedPlanId}" as "admin"
    Then the response status code should be 404

    # A free plan with a subscription can't be deleted or taken off its API, and the subscription keeps working
    Given I generate a unique resource name from "portal-limit-plan" and store it as "planId"
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
