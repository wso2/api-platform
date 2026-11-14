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



@APIPORTAL @apiportal-subscription-plan-conditions
Feature: The conditions of a subscription plan
  As an API Portal administrator
  I want a plan to hold the conditions I give it, each with its limits or values
  So that the gateway enforces the plan subscribers chose

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml) at the API level. "limits" in a response mirrors
  # only the request-count condition; the conditions are the plan's rules.

  @APIP-056 @type:smoke
  Scenario: An administrator saves a plan's conditions, changes them and makes the plan unlimited
    # Every kind of condition is stored with its limits, defaults and values
    Given I generate a unique resource name from "portal-cond-plan" and store it as "planId"
    And a unique API Portal resource is created at "/subscription-plans" as "admin" with values and stored as "planId":
      | id          | ${CTX:planId}                                                                                                                                                                                                                                                                                                                                                                                                                       |
      | displayName | Platinum ${CTX:planId}                                                                                                                                                                                                                                                                                                                                                                                                              |
      | conditions  | [{"type":"request-count","params":{"limits":[{"count":100,"timeUnit":"MINUTE"},{"count":5000,"timeUnit":"DAY"}]}},{"type":"token-count","params":{"limits":[{"count":10000,"timeUnit":"HOUR"}],"onExceed":"allow"}},{"type":"tool-call-count","params":{"limits":[{"count":50,"timeAmount":5,"timeUnit":"MINUTE"}]}},{"id":"region-limit","type":"custom","params":{"regions":"eu,us","policy":{"name":"region-guard","version":"v1"}}}] |
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 200
    And the JSON response array field "conditions" should have 4 items
    And the JSON response field "conditions[0].id" should be "requests"
    And the JSON response array field "conditions[0].params.limits" should have 2 items
    And the JSON response field "conditions[0].params.onExceed" should be "reject"
    And the JSON response field "conditions[1].id" should be "tokens"
    And the JSON response field "conditions[1].params.onExceed" should be "allow"
    And the JSON response field "conditions[1].params.tokenType" should be "total"
    And the JSON response field "conditions[2].id" should be "tool-calls"
    And the JSON response field "conditions[2].params.limits[0].timeAmount" should be 5
    And the JSON response field "conditions[3].id" should be "region-limit"
    And the JSON response field "conditions[3].params.regions" should be "eu,us"
    And the JSON response field "conditions[3].params.policy.name" should be "region-guard"
    And the JSON response field "conditions[3].params.policy.version" should be "v1"
    # "limits" mirrors the request-count condition's two windows only
    And the JSON response array field "limits" should have 2 items

    # An update without conditions or limits keeps the conditions
    When I send an authenticated API Portal "PUT" request to "/subscription-plans" as "admin" with JSON values:
      | id          | ${CTX:planId}               |
      | displayName | Platinum plus ${CTX:planId} |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the JSON response field "displayName" should be "Platinum plus ${CTX:planId}"
    And the JSON response array field "conditions" should have 4 items

    # Conditions sent replace them all: a plan with only a custom condition has no request limits
    When I send an authenticated API Portal "PUT" request to "/subscription-plans" as "admin" with JSON values:
      | id          | ${CTX:planId}                                                                  |
      | displayName | Platinum plus ${CTX:planId}                                                    |
      | conditions  | [{"id":"region-limit","type":"custom","params":{"policy":{"name":"region-guard","version":"v2"}}}] |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the JSON response array field "conditions" should have 1 item
    And the JSON response field "conditions[0].params.policy.version" should be "v2"
    And the JSON response array field "limits" should have 0 items

    # No conditions: the plan is unlimited and only needs a subscription
    When I send an authenticated API Portal "PUT" request to "/subscription-plans" as "admin" with JSON values:
      | id          | ${CTX:planId}               |
      | displayName | Platinum plus ${CTX:planId} |
      | conditions  | []                          |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the JSON response array field "conditions" should have 0 items
    And the JSON response array field "limits" should have 0 items

  @APIP-057 @type:negative
  Scenario Outline: A plan with an invalid condition is refused with the reason and nothing is stored
    Given I generate a unique resource name from "portal-cond-plan" and store it as "planId"
    When I send an authenticated API Portal "POST" request to "/subscription-plans" as "admin" with JSON values:
      | id          | ${CTX:planId}         |
      | displayName | Refused ${CTX:planId} |
      | conditions  | <conditions>          |
    # The request's schema refuses an unknown or missing type and a condition without params;
    # the plan's own checks refuse the rest
    Then the response status code should be 400
    And the JSON response field "errors[0].message" should be "<message>"
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 404

    Examples:
      | case                     | conditions                                                                                                                                                  | message                                                                                                                                 |
      | event-count was removed  | [{"type":"event-count","params":{"limits":[{"count":10,"timeUnit":"MINUTE"}]}}]                                                                             | type of condition 1 must be one of request-count, tool-call-count, token-count, custom                                                    |
      | no type                  | [{"params":{"limits":[{"count":10,"timeUnit":"MINUTE"}]}}]                                                                                                  | type of condition 1 is required                                                                                                         |
      | count of zero            | [{"type":"request-count","params":{"limits":[{"count":0,"timeUnit":"MINUTE"}]}}]                                                                            | conditions[0].params.limits[0]: count must be at least 1                                                                                |
      | fractional count         | [{"type":"request-count","params":{"limits":[{"count":1.5,"timeUnit":"MINUTE"}]}}]                                                                          | conditions[0].params.limits[0]: count must be a whole number                                                                            |
      | unknown window           | [{"type":"request-count","params":{"limits":[{"count":10,"timeUnit":"WEEK"}]}}]                                                                             | conditions[0].params.limits[0]: timeUnit must be one of MINUTE, HOUR, DAY or MONTH                                                      |
      | no limits                | [{"type":"tool-call-count","params":{"limits":[]}}]                                                                                                         | conditions[0]: params.limits needs at least one limit                                                                                   |
      | windows of equal length  | [{"type":"request-count","params":{"limits":[{"count":10,"timeAmount":60,"timeUnit":"MINUTE"},{"count":20,"timeUnit":"HOUR"}]}}]                            | conditions[0].params.limits[1]: the condition already has a limit over 1 HOUR; each limit needs a window of its own length              |
      | unknown onExceed         | [{"type":"request-count","params":{"limits":[{"count":10,"timeUnit":"MINUTE"}],"onExceed":"throttle"}}]                                                     | conditions[0]: params.onExceed must be reject or allow                                                                                  |
      | token type on requests   | [{"type":"request-count","params":{"limits":[{"count":10,"timeUnit":"MINUTE"}],"tokenType":"prompt"}}]                                                      | conditions[0]: params.tokenType is for token-count conditions only                                                                      |
      | unknown token type       | [{"type":"token-count","params":{"limits":[{"count":10,"timeUnit":"MINUTE"}],"tokenType":"cached"}}]                                                        | conditions[0]: params.tokenType must be total, prompt or completion                                                                     |
      | wrong built-in id        | [{"id":"calls","type":"request-count","params":{"limits":[{"count":10,"timeUnit":"MINUTE"}]}}]                                                              | conditions[0]: the id of a request-count condition is requests                                                                          |
      | two request-count        | [{"type":"request-count","params":{"limits":[{"count":10,"timeUnit":"MINUTE"}]}},{"type":"request-count","params":{"limits":[{"count":20,"timeUnit":"DAY"}]}}] | conditions[1]: a plan can have only one request-count condition                                                                     |
      | custom with a bad id     | [{"id":"Region_Limit","type":"custom","params":{}}]                                                                                                         | conditions[0]: a custom condition needs an id of lowercase letters, digits and hyphens, starting with a letter (at most 63 characters) |
      | custom with built-in id  | [{"id":"tokens","type":"custom","params":{}}]                                                                                                               | conditions[0]: tokens is the id of a built-in condition                                                                                 |
      | custom without params    | [{"id":"region-limit","type":"custom"}]                                                                                                                     | params of condition 1 is required                                                       |
      | custom repeated          | [{"id":"region-limit","type":"custom","params":{}},{"id":"region-limit","type":"custom","params":{}}]                                                       | conditions[1]: the plan already has a condition region-limit                                                                            |
