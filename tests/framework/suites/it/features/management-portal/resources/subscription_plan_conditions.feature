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


@SUBPLAN @platform-api-subscription-plan-conditions
Feature: A subscription plan's conditions are kept, replaced and checked by the control plane
  As an API platform administrator
  I want a plan to hold the conditions I give it, each with its limits or values
  So that the policies attached to an API enforce what subscribers chose

  # Scenarios SUBPLAN (scenarios/on-prem/SUBPLAN.yaml). A plan with no conditions is unlimited.
  # "limits" is the deprecated view of the request-count condition only.

  @SUBPLAN-001 @type:smoke
  Scenario: Every kind of condition is stored with its defaults, replaced, and removed to make the plan unlimited
    Given I generate a unique resource name from "cond-plan" and store it as "plan"
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id": "${CTX:plan}", "displayName": "Platinum ${CTX:plan}", "status": "ACTIVE", "description": "Requests, tokens, tool calls and regions", "conditions": [
        {"type": "request-count", "params": {"limits": [{"count": 100, "timeUnit": "MINUTE"}, {"count": 5000, "timeUnit": "DAY"}]}},
        {"type": "token-count", "params": {"limits": [{"count": 10000, "timeUnit": "HOUR"}], "onExceed": "allow", "tokenType": "prompt"}},
        {"id": "tool-calls", "type": "tool-call-count", "params": {"limits": [{"count": 50, "timeAmount": 5, "timeUnit": "MINUTE"}]}},
        {"id": "region-limit", "type": "custom", "params": {"regions": ["eu", "us"], "policy": {"name": "region-guard", "version": "v1"}}}]}
      """
    Then the response status code should be 201
    And the control plane subscription_plan "${CTX:plan}" is registered for cleanup
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:plan}"
    Then the response status code should be 200
    And the JSON response field "description" should be "Requests, tokens, tool calls and regions"
    And the JSON response array field "conditions" should have 4 items
    # A built-in condition gets the id its type fixes, and its defaults
    And the JSON response field "conditions[0].id" should be "requests"
    And the JSON response array field "conditions[0].params.limits" should have 2 items
    And the JSON response field "conditions[0].params.limits[0].timeAmount" should be 1
    And the JSON response field "conditions[0].params.onExceed" should be "reject"
    And the JSON response field "conditions[1].id" should be "tokens"
    And the JSON response field "conditions[1].params.onExceed" should be "allow"
    And the JSON response field "conditions[1].params.tokenType" should be "prompt"
    And the JSON response field "conditions[2].params.limits[0].timeAmount" should be 5
    # A custom condition keeps its values, including the policy that enforces it
    And the JSON response field "conditions[3].id" should be "region-limit"
    And the JSON response field "conditions[3].params.regions[1]" should be "us"
    And the JSON response field "conditions[3].params.policy.name" should be "region-guard"
    And the JSON response field "conditions[3].params.policy.version" should be "v1"
    # "limits" shows only the request-count condition's windows
    And the JSON response array field "limits" should have 2 items
    And the JSON response field "usage.inUse" should be false

    # An update that leaves conditions out keeps them
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:plan}" with body:
      """
      {"id": "${CTX:plan}", "displayName": "Platinum plus ${CTX:plan}", "status": "ACTIVE"}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:plan}"
    Then the JSON response field "displayName" should be "Platinum plus ${CTX:plan}"
    And the JSON response array field "conditions" should have 4 items

    # Conditions sent replace them all
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:plan}" with body:
      """
      {"id": "${CTX:plan}", "displayName": "Platinum plus ${CTX:plan}", "status": "ACTIVE", "conditions": [{"id": "region-limit", "type": "custom", "params": {"policy": {"name": "region-guard", "version": "v2"}}}]}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:plan}"
    Then the JSON response array field "conditions" should have 1 item
    And the JSON response field "conditions[0].params.policy.version" should be "v2"
    And the JSON response array field "limits" should have 0 items

    # [] removes them all: the plan is unlimited
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:plan}" with body:
      """
      {"id": "${CTX:plan}", "displayName": "Platinum plus ${CTX:plan}", "status": "ACTIVE", "conditions": []}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:plan}"
    Then the JSON response array field "conditions" should have 0 items
    And the JSON response array field "limits" should have 0 items

  @SUBPLAN-002
  Scenario: Request limits from clients that predate conditions set only the request-count condition
    Given I generate a unique resource name from "legacy-plan" and store it as "plan"
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id": "${CTX:plan}", "displayName": "Legacy ${CTX:plan}", "status": "ACTIVE", "limits": [{"limitType": "REQUEST_COUNT", "timeUnit": "HOUR", "limitCount": 1000}]}
      """
    Then the response status code should be 201
    And the control plane subscription_plan "${CTX:plan}" is registered for cleanup
    And the JSON response array field "conditions" should have 1 item
    And the JSON response field "conditions[0].type" should be "request-count"
    And the JSON response field "conditions[0].params.limits[0].count" should be 1000
    And the JSON response field "conditions[0].params.limits[0].timeUnit" should be "HOUR"
    And the JSON response field "limits[0].limitCount" should be 1000

    # A custom condition is added beside it
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:plan}" with body:
      """
      {"id": "${CTX:plan}", "displayName": "Legacy ${CTX:plan}", "status": "ACTIVE", "conditions": [{"type": "request-count", "params": {"limits": [{"count": 1000, "timeUnit": "HOUR"}]}}, {"id": "region-limit", "type": "custom", "params": {}}]}
      """
    Then the response status code should be 200

    # Limits sent alone replace the request-count condition and keep the others
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:plan}" with body:
      """
      {"id": "${CTX:plan}", "displayName": "Legacy ${CTX:plan}", "status": "ACTIVE", "limits": [{"limitType": "REQUEST_COUNT", "timeUnit": "MINUTE", "limitCount": 20}]}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:plan}"
    Then the JSON response array field "conditions" should have 2 items
    And the JSON response array "conditions" item with "id" equal to "requests" should have "params.limits[0].count" equal to "20"
    And the JSON response array "conditions" item with "id" equal to "region-limit" should have "type" equal to "custom"

    # Empty limits remove only the request-count condition
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:plan}" with body:
      """
      {"id": "${CTX:plan}", "displayName": "Legacy ${CTX:plan}", "status": "ACTIVE", "limits": []}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:plan}"
    Then the JSON response array field "conditions" should have 1 item
    And the JSON response field "conditions[0].id" should be "region-limit"
    And the JSON response array field "limits" should have 0 items

    # Conditions and limits together are refused, and so are limits other than requests
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:plan}" with body:
      """
      {"id": "${CTX:plan}", "displayName": "Legacy ${CTX:plan}", "status": "ACTIVE", "conditions": [], "limits": []}
      """
    Then the response status code should be 400
    And the JSON response field "message" should be "send conditions or limits, not both"
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:plan}" with body:
      """
      {"id": "${CTX:plan}", "displayName": "Legacy ${CTX:plan}", "status": "ACTIVE", "limits": [{"limitType": "EVENT_COUNT", "timeUnit": "MINUTE", "limitCount": 20}]}
      """
    Then the response status code should be 400
    And the JSON response field "message" should be "limitType: only REQUEST_COUNT is currently supported"
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:plan}"
    Then the JSON response array field "conditions" should have 1 item

  @SUBPLAN-003 @type:negative
  Scenario Outline: A plan with an invalid condition is refused with the reason and nothing is stored
    Given I generate a unique resource name from "bad-plan" and store it as "plan"
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id": "${CTX:plan}", "displayName": "Refused ${CTX:plan}", "status": "ACTIVE", "conditions": <conditions>}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"
    And the JSON response field "message" should be "<message>"
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:plan}"
    Then the response status code should be 404

    Examples:
      | case                    | conditions                                                                                                                                                  | message                                                                                                                                 |
      | event-count was removed | [{"type":"event-count","params":{"limits":[{"count":10,"timeUnit":"MINUTE"}]}}]                                                                             | conditions[0]: type must be one of request-count, tool-call-count, token-count or custom                                               |
      | no type                 | [{"params":{"limits":[{"count":10,"timeUnit":"MINUTE"}]}}]                                                                                                  | conditions[0]: type is required                                                                                                         |
      | no params               | [{"type":"request-count"}]                                                                                                                                  | conditions[0]: params with at least one limit is required                                                                               |
      | count of zero           | [{"type":"request-count","params":{"limits":[{"count":0,"timeUnit":"MINUTE"}]}}]                                                                            | conditions[0].params.limits[0]: count must be at least 1                                                                                |
      | negative window         | [{"type":"request-count","params":{"limits":[{"count":10,"timeAmount":-1,"timeUnit":"MINUTE"}]}}]                                                           | conditions[0].params.limits[0]: timeAmount must be at least 1                                                                           |
      | unknown window          | [{"type":"request-count","params":{"limits":[{"count":10,"timeUnit":"WEEK"}]}}]                                                                             | conditions[0].params.limits[0]: timeUnit must be one of MINUTE, HOUR, DAY or MONTH                                                      |
      | no limits               | [{"type":"tool-call-count","params":{"limits":[]}}]                                                                                                         | conditions[0]: params.limits needs at least one limit                                                                                   |
      | windows of equal length | [{"type":"request-count","params":{"limits":[{"count":10,"timeAmount":60,"timeUnit":"MINUTE"},{"count":20,"timeUnit":"HOUR"}]}}]                            | conditions[0].params.limits[1]: the condition already has a limit over 1 HOUR; each limit needs a window of its own length               |
      | unknown onExceed        | [{"type":"request-count","params":{"limits":[{"count":10,"timeUnit":"MINUTE"}],"onExceed":"throttle"}}]                                                     | conditions[0]: params.onExceed must be reject or allow                                                                                  |
      | token type on requests  | [{"type":"request-count","params":{"limits":[{"count":10,"timeUnit":"MINUTE"}],"tokenType":"prompt"}}]                                                      | conditions[0]: params.tokenType is for token-count conditions only                                                                      |
      | unknown token type      | [{"type":"token-count","params":{"limits":[{"count":10,"timeUnit":"MINUTE"}],"tokenType":"cached"}}]                                                        | conditions[0]: params.tokenType must be total, prompt or completion                                                                     |
      | wrong built-in id       | [{"id":"calls","type":"request-count","params":{"limits":[{"count":10,"timeUnit":"MINUTE"}]}}]                                                              | conditions[0]: the id of a request-count condition is requests                                                                          |
      | two request-count       | [{"type":"request-count","params":{"limits":[{"count":10,"timeUnit":"MINUTE"}]}},{"type":"request-count","params":{"limits":[{"count":20,"timeUnit":"DAY"}]}}] | conditions[1]: a plan can have only one request-count condition                                                                     |
      | custom with a bad id    | [{"id":"Region_Limit","type":"custom","params":{}}]                                                                                                         | conditions[0]: a custom condition needs an id of lowercase letters, digits and hyphens, starting with a letter (at most 63 characters) |
      | custom with built-in id | [{"id":"tokens","type":"custom","params":{}}]                                                                                                               | conditions[0]: tokens is the id of a built-in condition                                                                                 |
      | custom without params   | [{"id":"region-limit","type":"custom"}]                                                                                                                     | conditions[0]: params is required; send {} for a custom condition with no values                                                       |
      | custom params not a map | [{"id":"region-limit","type":"custom","params":["eu"]}]                                                                                                     | conditions[0]: params must be a JSON object of key-value pairs                                                                          |
      | custom repeated         | [{"id":"region-limit","type":"custom","params":{}},{"id":"region-limit","type":"custom","params":{}}]                                                       | conditions[1]: the plan already has a condition region-limit                                                                            |
      | conditions not a list   | {"type":"request-count"}                                                                                                                                    | conditions must be an array of {id, type, params}                                                                                       |
