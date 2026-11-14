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


@SUBPLAN @platform-api-plan-conditions-enforcement
Feature: A plan defined by its conditions is enforced on the gateway
  As an API platform operator
  I want a plan whose request limit is a request-count condition to throttle its subscribers
  So that plans made with conditions work on every gateway, including those that predate conditions

  # Scenarios SUBPLAN (scenarios/on-prem/SUBPLAN.yaml). The control plane sends a plan's conditions
  # and, for gateways that predate them, the request-count condition's one-unit window as the
  # plan's throttle limit, which subscription-validation v1 enforces.
  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique resource name from "cond-enf-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  @SUBPLAN-006 @secured
  Scenario: A plan's request-count condition throttles a subscription on the gateway
    Given I generate a unique resource name from "cond-enf-plan" and store it as "planHandle"
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id": "${CTX:planHandle}", "displayName": "${CTX:planHandle}", "status": "ACTIVE", "conditions": [
        {"type": "request-count", "params": {"limits": [{"count": 1000, "timeAmount": 2, "timeUnit": "HOUR"}, {"count": 3, "timeUnit": "MINUTE"}]}},
        {"id": "region-limit", "type": "custom", "params": {"regions": ["eu"]}}]}
      """
    Then the response status code should be 201
    And the control plane subscription_plan "${CTX:planHandle}" is registered for cleanup
    And I generate a unique resource name from "cond-enf-api" and store it as "apiHandle"
    And I generate a unique API context from "/cond-enf" and store it as "apiContext"
    When I create a secured REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" offering plan "${CTX:planHandle}"
    And I deploy the "RestApi" "${CTX:apiHandle}" to the gateway via the control plane
    Then I send a "GET" request to "${CTX:apiContext}/" until status 401

    When I generate a unique resource name from "cond-enf-app" and store it as "appHandle"
    And I create an application "${CTX:appHandle}" via the control plane in project "${CTX:projectHandle}"
    And I create a subscription for REST API "${CTX:apiHandle}" application "${CTX:appHandle}" plan "${CTX:planHandle}" via the control plane and store the subscription token as "subscriptionToken"
    And I generate a unique value from "cond-enf-key" and store it as "apiKeyValue"
    And I issue an API key "${CTX:apiKeyValue}" for REST API "${CTX:apiHandle}" via the control plane

    # The first one-unit window (per minute, not per 2 hours) is the limit: the 4th request in a minute is throttled
    When I set header "API-Key" to "${CTX:apiKeyValue}"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    And I send a "GET" request to "${CTX:apiContext}/" until status 200
    When I send 2 "GET" requests to "${CTX:apiContext}/"
    And I send a "GET" request to "${CTX:apiContext}/" until status 429
