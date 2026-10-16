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

@subscription-management-api
Feature: Subscription management REST API
  As an external control plane managing subscriptions through the gateway management API
  I want to address subscriptions by my own reference ID and update their plan and token in place
  So that I never have to delete and recreate a subscription to change it

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A caller-supplied externalRefId becomes the subscription ID, and a generated ID is used otherwise
    When I create a subscription plan "SubMgmtRefPlan" with 100 requests per minute
    Then the response status should be 201
    And I store the last response field "id" as "refPlanId"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: sub-mgmt-ref-api
      spec:
        displayName: Subscription Management Ref API
        version: v1.0
        context: /sub-mgmt-ref/$version
        subscriptionPlans:
          - SubMgmtRefPlan
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: subscription-validation
                version: v1
                params:
                  subscriptionKeyHeader: "Subscription-Key"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/sub-mgmt-ref/v1.0/health" to return 403

    When I create a subscription via the management API with body:
      """
      {"externalRefId": "it-ref-sub-1", "apiId": "sub-mgmt-ref-api", "subscriptionPlanId": "${refPlanId}", "subscriptionToken": "it-ref-token-1-0123456789abcdef0123"}
      """
    Then the response status should be 201
    And the JSON response field "id" should be "it-ref-sub-1"
    And the JSON response field "externalRefId" should be "it-ref-sub-1"
    And the JSON response field "subscriptionToken" should be "it-ref-token-1-0123456789abcdef0123"

    When I send a GET request to the management API at "/subscriptions/it-ref-sub-1"
    Then the response status should be 200
    And the JSON response field "id" should be "it-ref-sub-1"
    And the JSON response field "externalRefId" should be "it-ref-sub-1"
    And the JSON response field "subscriptionToken" should not exist

    When I send a GET request to "http://localhost:8080/sub-mgmt-ref/v1.0/health" with header "Subscription-Key" value "it-ref-token-1-0123456789abcdef0123"
    Then the response status code should be 200

    # Without externalRefId the gateway generates the ID and echoes it as externalRefId.
    When I create a subscription via the management API with body:
      """
      {"apiId": "sub-mgmt-ref-api", "subscriptionToken": "it-ref-token-2-0123456789abcdef0123"}
      """
    Then the response status should be 201
    And I store the last response field "id" as "generatedSubId"
    And the response field "externalRefId" should equal the stored value "generatedSubId"
    When I send a GET request to the management API at "/subscriptions/${generatedSubId}"
    Then the response status should be 200
    And the response field "externalRefId" should equal the stored value "generatedSubId"

    # Cleanup
    When I send a DELETE request to the management API at "/subscriptions/it-ref-sub-1"
    Then the response status should be 204
    When I send a DELETE request to the management API at "/subscriptions/${generatedSubId}"
    Then the response status should be 204
    When I delete the API "sub-mgmt-ref-api"
    Then the response should be successful
    When I delete the subscription plan with stored id "refPlanId"
    Then the response status should be 204


  Scenario: Subscriptions without an applicationId are told apart and managed by their externalRefId
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: sub-mgmt-multi-api
      spec:
        displayName: Subscription Management Multi API
        version: v1.0
        context: /sub-mgmt-multi/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: subscription-validation
                version: v1
                params:
                  subscriptionKeyHeader: "Subscription-Key"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/sub-mgmt-multi/v1.0/health" to return 403

    When I create a subscription via the management API with body:
      """
      {"externalRefId": "it-multi-sub-a", "apiId": "sub-mgmt-multi-api", "subscriptionToken": "it-multi-token-a-0123456789abcdef012"}
      """
    Then the response status should be 201
    When I create a subscription via the management API with body:
      """
      {"externalRefId": "it-multi-sub-b", "apiId": "sub-mgmt-multi-api", "subscriptionToken": "it-multi-token-b-0123456789abcdef012"}
      """
    Then the response status should be 201

    When I send a GET request to the management API at "/subscriptions?apiId=sub-mgmt-multi-api"
    Then the response status should be 200
    And the JSON response field "count" should be 2
    And the response body should contain "it-multi-sub-a"
    And the response body should contain "it-multi-sub-b"

    # Revoke one of them by its reference; the other keeps working.
    When I update the subscription "it-multi-sub-a" via the management API with body:
      """
      {"status": "REVOKED"}
      """
    Then the response status should be 200
    And the JSON response field "status" should be "REVOKED"
    When I send a GET request to the management API at "/subscriptions/it-multi-sub-b"
    Then the response status should be 200
    And the JSON response field "status" should be "ACTIVE"
    When I send a GET request to "http://localhost:8080/sub-mgmt-multi/v1.0/health" with header "Subscription-Key" value "it-multi-token-a-0123456789abcdef012"
    Then the response status code should be 403
    When I send a GET request to "http://localhost:8080/sub-mgmt-multi/v1.0/health" with header "Subscription-Key" value "it-multi-token-b-0123456789abcdef012"
    Then the response status code should be 200

    # Cleanup
    When I send a DELETE request to the management API at "/subscriptions/it-multi-sub-a"
    Then the response status should be 204
    When I send a DELETE request to the management API at "/subscriptions/it-multi-sub-b"
    Then the response status should be 204
    When I delete the API "sub-mgmt-multi-api"
    Then the response should be successful


  Scenario: Rotating the token in place keeps the subscription and switches which token is accepted
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: sub-mgmt-rotate-api
      spec:
        displayName: Subscription Management Rotate API
        version: v1.0
        context: /sub-mgmt-rotate/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: subscription-validation
                version: v1
                params:
                  subscriptionKeyHeader: "Subscription-Key"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/sub-mgmt-rotate/v1.0/health" to return 403

    When I create a subscription via the management API with body:
      """
      {"externalRefId": "it-rotate-sub", "apiId": "sub-mgmt-rotate-api", "subscriptionToken": "it-rotate-token-old-0123456789abcdef"}
      """
    Then the response status should be 201
    When I send a GET request to "http://localhost:8080/sub-mgmt-rotate/v1.0/health" with header "Subscription-Key" value "it-rotate-token-old-0123456789abcdef"
    Then the response status code should be 200

    When I update the subscription "it-rotate-sub" via the management API with body:
      """
      {"subscriptionToken": "it-rotate-token-new-0123456789abcdef"}
      """
    Then the response status should be 200
    And the JSON response field "id" should be "it-rotate-sub"
    And the JSON response field "subscriptionToken" should not exist
    And the response body should not contain "it-rotate-token-new-0123456789abcdef"

    When I send a GET request to "http://localhost:8080/sub-mgmt-rotate/v1.0/health" with header "Subscription-Key" value "it-rotate-token-old-0123456789abcdef"
    Then the response status code should be 403
    When I send a GET request to "http://localhost:8080/sub-mgmt-rotate/v1.0/health" with header "Subscription-Key" value "it-rotate-token-new-0123456789abcdef"
    Then the response status code should be 200

    # The subscription never went away and kept its ID.
    When I send a GET request to the management API at "/subscriptions/it-rotate-sub"
    Then the response status should be 200
    And the JSON response field "status" should be "ACTIVE"

    # Cleanup
    When I send a DELETE request to the management API at "/subscriptions/it-rotate-sub"
    Then the response status should be 204
    When I delete the API "sub-mgmt-rotate-api"
    Then the response should be successful


  Scenario: Changing or removing the plan in place changes rate limiting for the same token
    When I create a subscription plan "SubMgmtSmall" with 3 requests per minute
    Then the response status should be 201
    And I store the last response field "id" as "smallPlanId"
    When I create a subscription plan "SubMgmtLarge" with 100 requests per minute
    Then the response status should be 201
    And I store the last response field "id" as "largePlanId"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: sub-mgmt-plan-api
      spec:
        displayName: Subscription Management Plan API
        version: v1.0
        context: /sub-mgmt-plan/$version
        subscriptionPlans:
          - SubMgmtSmall
          - SubMgmtLarge
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: subscription-validation
                version: v1
                params:
                  subscriptionKeyHeader: "Subscription-Key"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/sub-mgmt-plan/v1.0/health" to return 403

    When I create a subscription via the management API with body:
      """
      {"externalRefId": "it-plan-sub", "apiId": "sub-mgmt-plan-api", "subscriptionPlanId": "${smallPlanId}", "subscriptionToken": "it-plan-token-0123456789abcdef012345"}
      """
    Then the response status should be 201

    # Exhaust the small plan: 3 allowed, the 4th is throttled.
    When I send a GET request to "http://localhost:8080/sub-mgmt-plan/v1.0/health" with header "Subscription-Key" value "it-plan-token-0123456789abcdef012345"
    Then the response status code should be 200
    When I send 3 GET requests to "http://localhost:8080/sub-mgmt-plan/v1.0/health" with header "Subscription-Key" value "it-plan-token-0123456789abcdef012345"
    Then the response status code should be 429

    # Move to the large plan in place. The request carries no token.
    When I update the subscription "it-plan-sub" via the management API with body:
      """
      {"subscriptionPlanId": "${largePlanId}"}
      """
    Then the response status should be 200
    And the JSON response field "id" should be "it-plan-sub"
    And the response field "subscriptionPlanId" should equal the stored value "largePlanId"
    When I send a GET request to "http://localhost:8080/sub-mgmt-plan/v1.0/health" with header "Subscription-Key" value "it-plan-token-0123456789abcdef012345"
    Then the response status code should be 200

    # Status changes still work alongside the new fields.
    When I update the subscription "it-plan-sub" via the management API with body:
      """
      {"status": "INACTIVE"}
      """
    Then the response status should be 200
    And the response field "subscriptionPlanId" should equal the stored value "largePlanId"
    When I send a GET request to "http://localhost:8080/sub-mgmt-plan/v1.0/health" with header "Subscription-Key" value "it-plan-token-0123456789abcdef012345"
    Then the response status code should be 403
    When I update the subscription "it-plan-sub" via the management API with body:
      """
      {"status": "ACTIVE"}
      """
    Then the response status should be 200

    # Remove the plan with an explicit null: no plan means no rate limit.
    When I update the subscription "it-plan-sub" via the management API with body:
      """
      {"subscriptionPlanId": null}
      """
    Then the response status should be 200
    And the JSON response field "subscriptionPlanId" should not exist
    When I send 5 GET requests to "http://localhost:8080/sub-mgmt-plan/v1.0/health" with header "Subscription-Key" value "it-plan-token-0123456789abcdef012345"
    Then the response status code should be 200

    # Cleanup
    When I send a DELETE request to the management API at "/subscriptions/it-plan-sub"
    Then the response status should be 204
    When I delete the API "sub-mgmt-plan-api"
    Then the response should be successful
    When I delete the subscription plan with stored id "smallPlanId"
    Then the response status should be 204
    When I delete the subscription plan with stored id "largePlanId"
    Then the response status should be 204


  Scenario: Invalid requests are rejected and conflicts never reveal which value collided
    When I create a subscription plan "SubMgmtEnabled" with 100 requests per minute
    Then the response status should be 201
    And I store the last response field "id" as "enabledPlanId"
    When I create a subscription plan "SubMgmtNotEnabled" with 100 requests per minute
    Then the response status should be 201
    And I store the last response field "id" as "notEnabledPlanId"
    When I create a subscription plan "SubMgmtInactive" with 100 requests per minute
    Then the response status should be 201
    And I store the last response field "id" as "inactivePlanId"
    When I update the subscription plan with stored id "inactivePlanId" with body:
      """
      {"status": "INACTIVE"}
      """
    Then the response status should be 200
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: sub-mgmt-errors-api
      spec:
        displayName: Subscription Management Errors API
        version: v1.0
        context: /sub-mgmt-errors/$version
        subscriptionPlans:
          - SubMgmtEnabled
          - SubMgmtInactive
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: subscription-validation
                version: v1
                params:
                  subscriptionKeyHeader: "Subscription-Key"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/sub-mgmt-errors/v1.0/health" to return 403

    When I create a subscription via the management API with body:
      """
      {"externalRefId": "it-err-sub-a", "apiId": "sub-mgmt-errors-api", "subscriptionPlanId": "${enabledPlanId}", "subscriptionToken": "it-err-token-a-0123456789abcdef01234"}
      """
    Then the response status should be 201
    When I create a subscription via the management API with body:
      """
      {"externalRefId": "it-err-sub-b", "apiId": "sub-mgmt-errors-api", "subscriptionToken": "it-err-token-b-0123456789abcdef01234"}
      """
    Then the response status should be 201

    # Invalid externalRefId values are rejected, not trimmed.
    When I create a subscription via the management API with body:
      """
      {"externalRefId": " padded-ref ", "apiId": "sub-mgmt-errors-api", "subscriptionToken": "it-err-token-c-0123456789abcdef01234"}
      """
    Then the response status should be 400
    When I create a subscription via the management API with body:
      """
      {"externalRefId": "a/b", "apiId": "sub-mgmt-errors-api", "subscriptionToken": "it-err-token-c-0123456789abcdef01234"}
      """
    Then the response status should be 400
    When I create a subscription via the management API with body:
      """
      {"externalRefId": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "apiId": "sub-mgmt-errors-api", "subscriptionToken": "it-err-token-c-0123456789abcdef01234"}
      """
    Then the response status should be 400

    # A reused externalRefId and a reused token get the same generic conflict.
    When I create a subscription via the management API with body:
      """
      {"externalRefId": "it-err-sub-a", "apiId": "sub-mgmt-errors-api", "subscriptionToken": "it-err-token-c-0123456789abcdef01234"}
      """
    Then the response status should be 409
    And the JSON response field "message" should be "The subscription conflicts with an existing subscription"
    When I create a subscription via the management API with body:
      """
      {"externalRefId": "it-err-sub-c", "apiId": "sub-mgmt-errors-api", "subscriptionToken": "it-err-token-a-0123456789abcdef01234"}
      """
    Then the response status should be 409
    And the JSON response field "message" should be "The subscription conflicts with an existing subscription"
    And the response body should not contain "token"

    # Plan problems on create
    When I create a subscription via the management API with body:
      """
      {"apiId": "sub-mgmt-errors-api", "subscriptionPlanId": "${notEnabledPlanId}", "subscriptionToken": "it-err-token-d-0123456789abcdef01234"}
      """
    Then the response status should be 400
    And the response body should contain "not enabled for this API"

    # Plan problems on update: unknown, not enabled for the API, inactive
    When I update the subscription "it-err-sub-a" via the management API with body:
      """
      {"subscriptionPlanId": "does-not-exist"}
      """
    Then the response status should be 400
    When I update the subscription "it-err-sub-a" via the management API with body:
      """
      {"subscriptionPlanId": "${notEnabledPlanId}"}
      """
    Then the response status should be 400
    And the response body should contain "not enabled for this API"
    When I update the subscription "it-err-sub-a" via the management API with body:
      """
      {"subscriptionPlanId": "${inactivePlanId}"}
      """
    Then the response status should be 400
    And the response body should contain "not active"

    # Other invalid updates
    When I update the subscription "it-err-sub-a" via the management API with body:
      """
      {"status": "PAUSED"}
      """
    Then the response status should be 400
    When I update the subscription "it-err-sub-a" via the management API with body:
      """
      {"subscriptionToken": "   "}
      """
    Then the response status should be 400

    # Rotating to a token another subscription on the API already uses is a generic conflict.
    When I update the subscription "it-err-sub-b" via the management API with body:
      """
      {"subscriptionToken": "it-err-token-a-0123456789abcdef01234"}
      """
    Then the response status should be 409
    And the JSON response field "message" should be "The subscription conflicts with an existing subscription"
    And the response body should not contain "token"

    # A rejected update changes nothing, even fields that were valid in the same request.
    When I update the subscription "it-err-sub-a" via the management API with body:
      """
      {"status": "REVOKED", "subscriptionPlanId": "does-not-exist"}
      """
    Then the response status should be 400
    When I send a GET request to the management API at "/subscriptions/it-err-sub-a"
    Then the response status should be 200
    And the JSON response field "status" should be "ACTIVE"
    And the response field "subscriptionPlanId" should equal the stored value "enabledPlanId"
    When I send a GET request to "http://localhost:8080/sub-mgmt-errors/v1.0/health" with header "Subscription-Key" value "it-err-token-a-0123456789abcdef01234"
    Then the response status code should be 200
    When I send a GET request to "http://localhost:8080/sub-mgmt-errors/v1.0/health" with header "Subscription-Key" value "it-err-token-b-0123456789abcdef01234"
    Then the response status code should be 200

    # Cleanup
    When I send a DELETE request to the management API at "/subscriptions/it-err-sub-a"
    Then the response status should be 204
    When I send a DELETE request to the management API at "/subscriptions/it-err-sub-b"
    Then the response status should be 204
    When I delete the API "sub-mgmt-errors-api"
    Then the response should be successful
    When I delete the subscription plan with stored id "enabledPlanId"
    Then the response status should be 204
    When I delete the subscription plan with stored id "notEnabledPlanId"
    Then the response status should be 204
    When I delete the subscription plan with stored id "inactivePlanId"
    Then the response status should be 204


  Scenario: Deleting by externalRefId removes the subscription and frees the reference
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: sub-mgmt-delete-api
      spec:
        displayName: Subscription Management Delete API
        version: v1.0
        context: /sub-mgmt-delete/$version
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: subscription-validation
                version: v1
                params:
                  subscriptionKeyHeader: "Subscription-Key"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/sub-mgmt-delete/v1.0/health" to return 403

    When I create a subscription via the management API with body:
      """
      {"externalRefId": "it-delete-sub", "apiId": "sub-mgmt-delete-api", "subscriptionToken": "it-delete-token-0123456789abcdef0123"}
      """
    Then the response status should be 201
    When I send a GET request to "http://localhost:8080/sub-mgmt-delete/v1.0/health" with header "Subscription-Key" value "it-delete-token-0123456789abcdef0123"
    Then the response status code should be 200

    When I send a DELETE request to the management API at "/subscriptions/it-delete-sub"
    Then the response status should be 204
    When I send a GET request to the management API at "/subscriptions/it-delete-sub"
    Then the response status should be 404
    When I update the subscription "it-delete-sub" via the management API with body:
      """
      {"status": "ACTIVE"}
      """
    Then the response status should be 404
    When I send a GET request to "http://localhost:8080/sub-mgmt-delete/v1.0/health" with header "Subscription-Key" value "it-delete-token-0123456789abcdef0123"
    Then the response status code should be 403

    # The reference can be reused once the subscription is gone.
    When I create a subscription via the management API with body:
      """
      {"externalRefId": "it-delete-sub", "apiId": "sub-mgmt-delete-api", "subscriptionToken": "it-delete-token-0123456789abcdef0123"}
      """
    Then the response status should be 201

    # Cleanup
    When I send a DELETE request to the management API at "/subscriptions/it-delete-sub"
    Then the response status should be 204
    When I delete the API "sub-mgmt-delete-api"
    Then the response should be successful
