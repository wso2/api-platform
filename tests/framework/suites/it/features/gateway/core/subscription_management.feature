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

@subscription-management
Feature: Subscription management
  As an external control plane managing subscriptions through the gateway management API
  I want to address subscriptions by my own reference ID and update their plan and token in place
  So that I never have to delete and recreate a subscription to change it

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A caller-supplied externalRefId becomes the subscription ID, and a generated ID is used otherwise
    Given I generate a unique value from "sub-mgmt-ref-plan" and store it as "planName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:planName}","throttleLimitCount":100,"throttleLimitUnit":"Min"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "planId"
    And I register the "subscription-plan" "${CTX:planId}" for cleanup

    Given I generate a unique value from "sub-mgmt-ref" and store it as "apiName"
    And I generate a unique API context from "/sub-mgmt-ref" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}  |
      | name                   | ${CTX:apiName}             |
      | spec.displayName       | ${CTX:apiName}             |
      | spec.version           | v1.0                       |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.subscriptionPlans | ["${CTX:planName}"]        |
      | spec.upstream.main.url | http://testbench:3002      |
      | spec.operations        | [{"method":"GET","path":"/health","policies":[{"name":"subscription-validation","version":"v1","params":{"subscriptionKeyHeader":"Subscription-Key"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 403

    Given I generate a unique value from "sub-mgmt-ref" and store it as "subRef"
    And I generate a unique value from "sub-mgmt-ref-token" and store it as "subToken"
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"externalRefId":"${CTX:subRef}","apiId":"${CTX:apiName}","subscriptionPlanId":"${CTX:planId}","subscriptionToken":"${CTX:subToken}"}
      """
    Then the response status should be 201
    And I register the "subscription" "${CTX:subRef}" for cleanup
    And the JSON response field "id" should be "${CTX:subRef}"
    And the JSON response field "externalRefId" should be "${CTX:subRef}"
    And the JSON response field "subscriptionToken" should be "${CTX:subToken}"

    When I send a "GET" request to the "gateway-controller" service at "/subscriptions/${CTX:subRef}"
    Then the response status should be 200
    And the JSON response field "id" should be "${CTX:subRef}"
    And the JSON response field "externalRefId" should be "${CTX:subRef}"
    And the JSON response field "subscriptionToken" should not exist

    When I set header "Subscription-Key" to "${CTX:subToken}"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200

    # Without externalRefId the gateway generates the ID and echoes it as externalRefId.
    Given I generate a unique value from "sub-mgmt-generated-token" and store it as "generatedToken"
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"apiId":"${CTX:apiName}","subscriptionToken":"${CTX:generatedToken}"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "generatedSubId"
    And I register the "subscription" "${CTX:generatedSubId}" for cleanup
    And the JSON response field "externalRefId" should be "${CTX:generatedSubId}"
    When I send a "GET" request to the "gateway-controller" service at "/subscriptions/${CTX:generatedSubId}"
    Then the response status should be 200
    And the JSON response field "externalRefId" should be "${CTX:generatedSubId}"

  Scenario: Subscriptions without an applicationId are told apart and managed by their externalRefId
    Given I generate a unique value from "sub-mgmt-multi" and store it as "apiName"
    And I generate a unique API context from "/sub-mgmt-multi" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}  |
      | name                   | ${CTX:apiName}             |
      | spec.displayName       | ${CTX:apiName}             |
      | spec.version           | v1.0                       |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002      |
      | spec.operations        | [{"method":"GET","path":"/health","policies":[{"name":"subscription-validation","version":"v1","params":{"subscriptionKeyHeader":"Subscription-Key"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 403

    Given I generate a unique value from "sub-mgmt-multi-a" and store it as "subRefA"
    And I generate a unique value from "sub-mgmt-multi-a-token" and store it as "tokenA"
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"externalRefId":"${CTX:subRefA}","apiId":"${CTX:apiName}","subscriptionToken":"${CTX:tokenA}"}
      """
    Then the response status should be 201
    And I register the "subscription" "${CTX:subRefA}" for cleanup
    Given I generate a unique value from "sub-mgmt-multi-b" and store it as "subRefB"
    And I generate a unique value from "sub-mgmt-multi-b-token" and store it as "tokenB"
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"externalRefId":"${CTX:subRefB}","apiId":"${CTX:apiName}","subscriptionToken":"${CTX:tokenB}"}
      """
    Then the response status should be 201
    And I register the "subscription" "${CTX:subRefB}" for cleanup

    When I send a "GET" request to the "gateway-controller" service at "/subscriptions?apiId=${CTX:apiName}"
    Then the response status should be 200
    And the JSON response field "count" should be 2
    And the JSON response array "subscriptions" should contain an item with "id" equal to "${CTX:subRefA}"
    And the JSON response array "subscriptions" should contain an item with "id" equal to "${CTX:subRefB}"

    # Revoke one of them by its reference; the other keeps working.
    When I send a "PUT" request to the "gateway-controller" service at "/subscriptions/${CTX:subRefA}" with body:
      """
      {"status":"REVOKED"}
      """
    Then the response status should be 200
    And the JSON response field "status" should be "REVOKED"
    When I send a "GET" request to the "gateway-controller" service at "/subscriptions/${CTX:subRefB}"
    Then the response status should be 200
    And the JSON response field "status" should be "ACTIVE"
    When I set header "Subscription-Key" to "${CTX:tokenA}"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 403
    When I set header "Subscription-Key" to "${CTX:tokenB}"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200

  Scenario: Rotating the token in place keeps the subscription and switches which token is accepted
    Given I generate a unique value from "sub-mgmt-rotate" and store it as "apiName"
    And I generate a unique API context from "/sub-mgmt-rotate" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}  |
      | name                   | ${CTX:apiName}             |
      | spec.displayName       | ${CTX:apiName}             |
      | spec.version           | v1.0                       |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002      |
      | spec.operations        | [{"method":"GET","path":"/health","policies":[{"name":"subscription-validation","version":"v1","params":{"subscriptionKeyHeader":"Subscription-Key"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 403

    Given I generate a unique value from "sub-mgmt-rotate" and store it as "subRef"
    And I generate a unique value from "sub-mgmt-rotate-old" and store it as "oldToken"
    And I generate a unique value from "sub-mgmt-rotate-new" and store it as "newToken"
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"externalRefId":"${CTX:subRef}","apiId":"${CTX:apiName}","subscriptionToken":"${CTX:oldToken}"}
      """
    Then the response status should be 201
    And I register the "subscription" "${CTX:subRef}" for cleanup
    When I set header "Subscription-Key" to "${CTX:oldToken}"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200

    When I send a "PUT" request to the "gateway-controller" service at "/subscriptions/${CTX:subRef}" with body:
      """
      {"subscriptionToken":"${CTX:newToken}"}
      """
    Then the response status should be 200
    And the JSON response field "id" should be "${CTX:subRef}"
    And the JSON response field "subscriptionToken" should not exist
    And the response body should not contain "${CTX:newToken}"

    When I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 403
    And I set header "Subscription-Key" to "${CTX:newToken}"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200

    # The subscription never went away and kept its ID.
    When I send a "GET" request to the "gateway-controller" service at "/subscriptions/${CTX:subRef}"
    Then the response status should be 200
    And the JSON response field "status" should be "ACTIVE"

  Scenario: Changing or removing the plan in place changes rate limiting for the same token
    Given I generate a unique value from "sub-mgmt-small" and store it as "smallPlanName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:smallPlanName}","throttleLimitCount":3,"throttleLimitUnit":"Min"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "smallPlanId"
    And I register the "subscription-plan" "${CTX:smallPlanId}" for cleanup
    Given I generate a unique value from "sub-mgmt-large" and store it as "largePlanName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:largePlanName}","throttleLimitCount":100,"throttleLimitUnit":"Min"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "largePlanId"
    And I register the "subscription-plan" "${CTX:largePlanId}" for cleanup

    Given I generate a unique value from "sub-mgmt-plan" and store it as "apiName"
    And I generate a unique API context from "/sub-mgmt-plan" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                      |
      | name                   | ${CTX:apiName}                                 |
      | spec.displayName       | ${CTX:apiName}                                 |
      | spec.version           | v1.0                                           |
      | spec.context           | ${CTX:apiContext}/$version                     |
      | spec.subscriptionPlans | ["${CTX:smallPlanName}","${CTX:largePlanName}"] |
      | spec.upstream.main.url | http://testbench:3002                          |
      | spec.operations        | [{"method":"GET","path":"/health","policies":[{"name":"subscription-validation","version":"v1","params":{"subscriptionKeyHeader":"Subscription-Key"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 403

    Given I generate a unique value from "sub-mgmt-plan" and store it as "subRef"
    And I generate a unique value from "sub-mgmt-plan-token" and store it as "subToken"
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"externalRefId":"${CTX:subRef}","apiId":"${CTX:apiName}","subscriptionPlanId":"${CTX:smallPlanId}","subscriptionToken":"${CTX:subToken}"}
      """
    Then the response status should be 201
    And I register the "subscription" "${CTX:subRef}" for cleanup

    # Exhaust the small plan: 3 allowed, the 4th is throttled.
    When I set header "Subscription-Key" to "${CTX:subToken}"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200
    And I send 3 "GET" requests to "${CTX:apiContext}/v1.0/health"
    Then the response status code should be 429

    # Move to the large plan in place. The request carries no token.
    When I send a "PUT" request to the "gateway-controller" service at "/subscriptions/${CTX:subRef}" with body:
      """
      {"subscriptionPlanId":"${CTX:largePlanId}"}
      """
    Then the response status should be 200
    And the JSON response field "id" should be "${CTX:subRef}"
    And the JSON response field "subscriptionPlanId" should be "${CTX:largePlanId}"
    When I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200

    # Status changes still work alongside the new fields.
    When I send a "PUT" request to the "gateway-controller" service at "/subscriptions/${CTX:subRef}" with body:
      """
      {"status":"INACTIVE"}
      """
    Then the response status should be 200
    And the JSON response field "subscriptionPlanId" should be "${CTX:largePlanId}"
    When I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 403
    When I send a "PUT" request to the "gateway-controller" service at "/subscriptions/${CTX:subRef}" with body:
      """
      {"status":"ACTIVE","subscriptionPlanId":"${CTX:smallPlanId}"}
      """
    Then the response status should be 200
    When I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 429

    # Remove the plan with an explicit null: the throttled subscription is no longer rate limited.
    When I send a "PUT" request to the "gateway-controller" service at "/subscriptions/${CTX:subRef}" with body:
      """
      {"subscriptionPlanId":null}
      """
    Then the response status should be 200
    And the JSON response field "subscriptionPlanId" should not exist
    When I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200
    And I send 5 "GET" requests to "${CTX:apiContext}/v1.0/health"
    Then the response status code should be 200

  Scenario: Invalid requests are rejected and conflicts never reveal which value collided
    Given I generate a unique value from "sub-mgmt-enabled" and store it as "enabledPlanName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:enabledPlanName}","throttleLimitCount":100,"throttleLimitUnit":"Min"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "enabledPlanId"
    And I register the "subscription-plan" "${CTX:enabledPlanId}" for cleanup
    Given I generate a unique value from "sub-mgmt-not-enabled" and store it as "notEnabledPlanName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:notEnabledPlanName}","throttleLimitCount":100,"throttleLimitUnit":"Min"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "notEnabledPlanId"
    And I register the "subscription-plan" "${CTX:notEnabledPlanId}" for cleanup
    Given I generate a unique value from "sub-mgmt-inactive" and store it as "inactivePlanName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:inactivePlanName}","throttleLimitCount":100,"throttleLimitUnit":"Min","status":"INACTIVE"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "inactivePlanId"
    And I register the "subscription-plan" "${CTX:inactivePlanId}" for cleanup

    Given I generate a unique value from "sub-mgmt-errors" and store it as "apiName"
    And I generate a unique API context from "/sub-mgmt-errors" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                             |
      | name                   | ${CTX:apiName}                                        |
      | spec.displayName       | ${CTX:apiName}                                        |
      | spec.version           | v1.0                                                  |
      | spec.context           | ${CTX:apiContext}/$version                            |
      | spec.subscriptionPlans | ["${CTX:enabledPlanName}","${CTX:inactivePlanName}"] |
      | spec.upstream.main.url | http://testbench:3002                                 |
      | spec.operations        | [{"method":"GET","path":"/health","policies":[{"name":"subscription-validation","version":"v1","params":{"subscriptionKeyHeader":"Subscription-Key"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 403

    Given I generate a unique value from "sub-mgmt-err-a" and store it as "subRefA"
    And I generate a unique value from "sub-mgmt-err-a-token" and store it as "tokenA"
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"externalRefId":"${CTX:subRefA}","apiId":"${CTX:apiName}","subscriptionPlanId":"${CTX:enabledPlanId}","subscriptionToken":"${CTX:tokenA}"}
      """
    Then the response status should be 201
    And I register the "subscription" "${CTX:subRefA}" for cleanup
    Given I generate a unique value from "sub-mgmt-err-b" and store it as "subRefB"
    And I generate a unique value from "sub-mgmt-err-b-token" and store it as "tokenB"
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"externalRefId":"${CTX:subRefB}","apiId":"${CTX:apiName}","subscriptionToken":"${CTX:tokenB}"}
      """
    Then the response status should be 201
    And I register the "subscription" "${CTX:subRefB}" for cleanup
    Given I generate a unique value from "sub-mgmt-err-c" and store it as "subRefC"
    And I generate a unique value from "sub-mgmt-err-c-token" and store it as "tokenC"

    # Invalid externalRefId values are rejected, not trimmed.
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"externalRefId":" padded-ref ","apiId":"${CTX:apiName}","subscriptionToken":"${CTX:tokenC}"}
      """
    Then the response status should be 400
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"externalRefId":"a/b","apiId":"${CTX:apiName}","subscriptionToken":"${CTX:tokenC}"}
      """
    Then the response status should be 400
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"externalRefId":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","apiId":"${CTX:apiName}","subscriptionToken":"${CTX:tokenC}"}
      """
    Then the response status should be 400

    # A reused externalRefId and a reused token get the same generic conflict.
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"externalRefId":"${CTX:subRefA}","apiId":"${CTX:apiName}","subscriptionToken":"${CTX:tokenC}"}
      """
    Then the response status should be 409
    And the JSON response field "message" should be "The subscription conflicts with an existing subscription"
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"externalRefId":"${CTX:subRefC}","apiId":"${CTX:apiName}","subscriptionToken":"${CTX:tokenA}"}
      """
    Then the response status should be 409
    And the JSON response field "message" should be "The subscription conflicts with an existing subscription"
    And the response body should not contain "token"

    # Plan problems on create
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"apiId":"${CTX:apiName}","subscriptionPlanId":"${CTX:notEnabledPlanId}","subscriptionToken":"${CTX:tokenC}"}
      """
    Then the response status should be 400
    And the response body should contain "not enabled for this API"

    # Plan problems on update: unknown, not enabled for the API, inactive
    When I send a "PUT" request to the "gateway-controller" service at "/subscriptions/${CTX:subRefA}" with body:
      """
      {"subscriptionPlanId":"does-not-exist"}
      """
    Then the response status should be 400
    When I send a "PUT" request to the "gateway-controller" service at "/subscriptions/${CTX:subRefA}" with body:
      """
      {"subscriptionPlanId":"${CTX:notEnabledPlanId}"}
      """
    Then the response status should be 400
    And the response body should contain "not enabled for this API"
    When I send a "PUT" request to the "gateway-controller" service at "/subscriptions/${CTX:subRefA}" with body:
      """
      {"subscriptionPlanId":"${CTX:inactivePlanId}"}
      """
    Then the response status should be 400
    And the response body should contain "not active"

    # Other invalid updates
    When I send a "PUT" request to the "gateway-controller" service at "/subscriptions/${CTX:subRefA}" with body:
      """
      {"status":"PAUSED"}
      """
    Then the response status should be 400
    When I send a "PUT" request to the "gateway-controller" service at "/subscriptions/${CTX:subRefA}" with body:
      """
      {"subscriptionToken":"   "}
      """
    Then the response status should be 400

    # Rotating to a token another subscription on the API already uses is a generic conflict.
    When I send a "PUT" request to the "gateway-controller" service at "/subscriptions/${CTX:subRefB}" with body:
      """
      {"subscriptionToken":"${CTX:tokenA}"}
      """
    Then the response status should be 409
    And the JSON response field "message" should be "The subscription conflicts with an existing subscription"
    And the response body should not contain "token"

    # A rejected update changes nothing, even fields that were valid in the same request.
    When I send a "PUT" request to the "gateway-controller" service at "/subscriptions/${CTX:subRefA}" with body:
      """
      {"status":"REVOKED","subscriptionPlanId":"does-not-exist"}
      """
    Then the response status should be 400
    When I send a "GET" request to the "gateway-controller" service at "/subscriptions/${CTX:subRefA}"
    Then the response status should be 200
    And the JSON response field "status" should be "ACTIVE"
    And the JSON response field "subscriptionPlanId" should be "${CTX:enabledPlanId}"
    When I set header "Subscription-Key" to "${CTX:tokenA}"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200
    When I set header "Subscription-Key" to "${CTX:tokenB}"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200

  Scenario: Deleting by externalRefId removes the subscription and frees the reference
    Given I generate a unique value from "sub-mgmt-delete" and store it as "apiName"
    And I generate a unique API context from "/sub-mgmt-delete" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}  |
      | name                   | ${CTX:apiName}             |
      | spec.displayName       | ${CTX:apiName}             |
      | spec.version           | v1.0                       |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002      |
      | spec.operations        | [{"method":"GET","path":"/health","policies":[{"name":"subscription-validation","version":"v1","params":{"subscriptionKeyHeader":"Subscription-Key"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 403

    Given I generate a unique value from "sub-mgmt-delete" and store it as "subRef"
    And I generate a unique value from "sub-mgmt-delete-token" and store it as "subToken"
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"externalRefId":"${CTX:subRef}","apiId":"${CTX:apiName}","subscriptionToken":"${CTX:subToken}"}
      """
    Then the response status should be 201
    And I register the "subscription" "${CTX:subRef}" for cleanup
    When I set header "Subscription-Key" to "${CTX:subToken}"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200

    When I send a "DELETE" request to the "gateway-controller" service at "/subscriptions/${CTX:subRef}"
    Then the response status should be 204
    When I send a "GET" request to the "gateway-controller" service at "/subscriptions/${CTX:subRef}"
    Then the response status should be 404
    When I send a "PUT" request to the "gateway-controller" service at "/subscriptions/${CTX:subRef}" with body:
      """
      {"status":"ACTIVE"}
      """
    Then the response status should be 404
    When I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 403

    # The reference can be reused once the subscription is gone. The cleanup registration made
    # for the first subscription still covers this one, since it has the same ID.
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"externalRefId":"${CTX:subRef}","apiId":"${CTX:apiName}","subscriptionToken":"${CTX:subToken}"}
      """
    Then the response status should be 201
