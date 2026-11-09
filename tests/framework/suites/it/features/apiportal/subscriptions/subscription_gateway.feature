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


@APIPORTAL @apiportal-subscription-gateway
Feature: The gateway enforces the subscriptions and API keys held in the API Portal
  As an API consumer calling an API through the gateway
  I want the gateway to honour my subscription, its plan's limit and my API key exactly as the portal holds them
  So that a call succeeds only with credentials the portal still considers valid

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml). The control
  # plane creates and deploys a secured API offering a plan, publishes it to the portal, and the
  # portal's webhook carries every subscription and key change to the gateway. The subscription and
  # key are created by the administrator; the developer is the other user.

  @APIP-024 @type:security
  Scenario: The gateway accepts a subscription only with the right credentials, and follows its suspension and removal
    # The gateway takes the subscription token only from Subscription-Key, and only together with the API key
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And a webhook subscriber targeting the control plane is registered in the API Portal
    And I generate a unique value from "subscription-gateway-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane
    And I generate a unique resource name from "subscription-gateway-plan" and store it as "planHandle"
    And I generate a unique resource name from "subscription-gateway-api" and store it as "apiHandle"
    And I generate a unique API context from "/subscription-gateway" and store it as "apiContext"
    Given I create a subscription plan "${CTX:planHandle}" allowing 10000 requests per hour via the control plane
    When I create a secured REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" offering plan "${CTX:planHandle}"
    And I deploy the "RestApi" "${CTX:apiHandle}" to the gateway via the control plane
    Then I send a "GET" request to "${CTX:apiContext}/" until status 401
    When the subscription plan "${CTX:planHandle}" is synced to the API Portal
    And the "${CTX:apiHandle}" API is published to the API Portal offering plan "${CTX:planHandle}"
    And an application subscribed to API "${CTX:apiHandle}" plan "${CTX:planHandle}" is created in the API Portal, the token is stored as "subscriptionToken" and the subscription is stored as "subscriptionId"
    And an API key for API "${CTX:apiHandle}" is generated in the API Portal, the value is stored as "apiKeyValue" and the handle is stored as "apiKeyHandle"
    And I set header "API-Key" to "${CTX:apiKeyValue}"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    Then I send a "GET" request to "${CTX:apiContext}/" until status 200

    When I clear all headers
    And I set header "API-Key" to "${CTX:apiKeyValue}"
    And I set header "X-Subscription-Token" to "${CTX:subscriptionToken}"
    And I send a "GET" request to "${CTX:apiContext}/"
    Then the response status code should be 403
    When I clear all headers
    And I set header "API-Key" to "${CTX:apiKeyValue}"
    And I send a "GET" request to "${CTX:apiContext}/"
    Then the response status code should be 403
    When I clear all headers
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    And I send a "GET" request to "${CTX:apiContext}/"
    Then the response status code should be 401
    When I clear all headers
    And I send a "GET" request to "${CTX:apiContext}/"
    Then the response status code should be 401

    When I set header "API-Key" to "${CTX:apiKeyValue}"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    And I send a "GET" request to "${CTX:apiContext}/"
    Then the response status code should be 200

    # A suspended or ended subscription is refused at the gateway, and subscribing again gives a new token

    When I send an authenticated API Portal "PUT" request to "/subscriptions/${CTX:subscriptionId}" as "admin" with JSON values:
      | status | EXPIRED |
    Then the response status code should be 400
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}" as "admin"
    Then the JSON response field "status" should be "ACTIVE"
    When I send a "GET" request to "${CTX:apiContext}/"
    Then the response status code should be 200

    When the subscription "${CTX:subscriptionId}" status is set to "INACTIVE" in the API Portal
    Then I send a "GET" request to "${CTX:apiContext}/" until status 403
    And the response body should contain "forbidden"
    When the subscription "${CTX:subscriptionId}" status is set to "ACTIVE" in the API Portal
    Then I send a "GET" request to "${CTX:apiContext}/" until status 200

    When the subscription "${CTX:subscriptionId}" is removed in the API Portal
    Then I send a "GET" request to "${CTX:apiContext}/" until status 403

    When I send an authenticated API Portal "POST" request to "/subscriptions" as "admin" with JSON values:
      | artifactId         | dp-${CTX:apiHandle} |
      | subscriptionPlanId | ${CTX:planHandle}   |
    Then the response status code should be 201
    And I store the JSON response field "subscriptionId" as "newSubscriptionId"
    And I register API Portal resource "${CTX:newSubscriptionId}" at "/subscriptions" for cleanup
    And the JSON response field "subscriptionId" should not equal "${CTX:subscriptionId}"
    And the JSON response field "subscriptionToken" should not equal "${CTX:subscriptionToken}"
    And I store the JSON response field "subscriptionToken" as "newSubscriptionToken"
    When I set header "Subscription-Key" to "${CTX:newSubscriptionToken}"
    Then I send a "GET" request to "${CTX:apiContext}/" until status 200
    When I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    And I send a "GET" request to "${CTX:apiContext}/"
    Then the response status code should be 403

  @APIP-027 @type:security
  Scenario: API keys keep working at the gateway against other users and after their application is deleted
    # Another user's attempts to rotate or revoke an API key leave it working at the gateway
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And a webhook subscriber targeting the control plane is registered in the API Portal
    And I generate a unique value from "subscription-gateway-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane
    And I generate a unique resource name from "subscription-gateway-plan" and store it as "planHandle"
    And I generate a unique resource name from "subscription-gateway-api" and store it as "apiHandle"
    And I generate a unique API context from "/subscription-gateway" and store it as "apiContext"
    Given I create a subscription plan "${CTX:planHandle}" allowing 10000 requests per hour via the control plane
    When I create a secured REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" offering plan "${CTX:planHandle}"
    And I deploy the "RestApi" "${CTX:apiHandle}" to the gateway via the control plane
    Then I send a "GET" request to "${CTX:apiContext}/" until status 401
    When the subscription plan "${CTX:planHandle}" is synced to the API Portal
    And the "${CTX:apiHandle}" API is published to the API Portal offering plan "${CTX:planHandle}"
    And an application subscribed to API "${CTX:apiHandle}" plan "${CTX:planHandle}" is created in the API Portal, the token is stored as "subscriptionToken" and the subscription is stored as "subscriptionId"
    And an API key for API "${CTX:apiHandle}" is generated in the API Portal, the value is stored as "apiKeyValue" and the handle is stored as "apiKeyHandle"
    And I set header "API-Key" to "${CTX:apiKeyValue}"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    Then I send a "GET" request to "${CTX:apiContext}/" until status 200

    When I send an authenticated API Portal "POST" request to "/apis/dp-${CTX:apiHandle}/api-keys/regenerate" as "developer" with JSON values:
      | keyId | ${CTX:apiKeyHandle} |
    Then the response status code should be 404
    And the JSON response field "key" should not exist
    When I send an authenticated API Portal "POST" request to "/apis/dp-${CTX:apiHandle}/api-keys/revoke" as "developer" with JSON values:
      | keyId | ${CTX:apiKeyHandle} |
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/apis/dp-${CTX:apiHandle}/api-keys" as "admin"
    Then the JSON response array "list" item with "id" equal to "${CTX:apiKeyHandle}" should have "status" equal to "ACTIVE"
    When I send a "GET" request to "${CTX:apiContext}/"
    Then the response status code should be 200

    # An API key linked to a deleted application keeps authorizing calls at the gateway
    Given I generate a unique resource name from "devportal-plan" and store it as "planHandle"
    And I create a subscription plan "${CTX:planHandle}" allowing 10000 requests per hour via the control plane
    And I generate a unique resource name from "devportal-api" and store it as "apiHandle"
    And I generate a unique API context from "/devportal" and store it as "apiContext"
    When I create a secured REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" offering plan "${CTX:planHandle}"
    And I deploy the "RestApi" "${CTX:apiHandle}" to the gateway via the control plane
    Then I send a "GET" request to "${CTX:apiContext}/" until status 401
    When the subscription plan "${CTX:planHandle}" is synced to the API Portal
    And the "${CTX:apiHandle}" API is published to the API Portal offering plan "${CTX:planHandle}"
    And an application subscribed to API "${CTX:apiHandle}" plan "${CTX:planHandle}" is created in the API Portal, the token is stored as "subscriptionToken" and the subscription is stored as "subscriptionId"
    And an API key for API "${CTX:apiHandle}" is generated in the API Portal, the value is stored as "apiKeyValue" and the handle is stored as "apiKeyHandle"
    And an API key for API "${CTX:apiHandle}" is generated in the API Portal, the value is stored as "barrierKeyValue" and the handle is stored as "barrierKeyHandle"
    When I send an authenticated API Portal "POST" request to "/apis/dp-${CTX:apiHandle}/api-keys/associate" as "admin" with JSON values:
      | keyId | ${CTX:apiKeyHandle}      |
      | appId | ${CTX:apiHandle}-app     |
    Then the response status code should be 200
    And the JSON response field "application.id" should be "${CTX:apiHandle}-app"
    When I set header "API-Key" to "${CTX:apiKeyValue}"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    Then I send a "GET" request to "${CTX:apiContext}/" until status 200
    When I set header "API-Key" to "${CTX:barrierKeyValue}"
    Then I send a "GET" request to "${CTX:apiContext}/" until status 200

    When I send an authenticated API Portal "DELETE" request to "/applications/${CTX:apiHandle}-app" as "admin"
    Then the response status code should be 200
    When the API key "${CTX:barrierKeyHandle}" for API "${CTX:apiHandle}" is revoked in the API Portal
    And I set header "API-Key" to "${CTX:barrierKeyValue}"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    Then I send a "GET" request to "${CTX:apiContext}/" until status 401
    When I send an authenticated API Portal "GET" request to "/apis/dp-${CTX:apiHandle}/api-keys" as "admin"
    Then the JSON response array "list" item with "id" equal to "${CTX:apiKeyHandle}" should have "status" equal to "ACTIVE"
    And the JSON response array "list" item with "id" equal to "${CTX:apiKeyHandle}" should have "appId" equal to "null"
    When I set header "API-Key" to "${CTX:apiKeyValue}"
    And I set header "Subscription-Key" to "${CTX:subscriptionToken}"
    And I send a "GET" request to "${CTX:apiContext}/"
    Then the response status code should be 200

    When I clear all headers
    And I send a "GET" request to "${CTX:apiContext}/"
    Then the response status code should be 401
