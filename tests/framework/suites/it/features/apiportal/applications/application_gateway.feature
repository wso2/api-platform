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


@APIPORTAL @apiportal-application-gateway
Feature: API keys keep working at the gateway after their application is deleted
  As a developer who deletes an application from the API Portal
  I want the API keys it held to keep authorizing calls at the gateway
  So that tidying my applications never breaks a client that is still calling the API

  # Scenario APIPORTAL (scenarios/APIPORTAL.yaml) APIP-DEVPORTAL-WEBHOOK-002, at the API level.
  # Workbook row APIP-014 step 5, gateway side; the portal and webhook side is APIP-WEBHOOKS-024.
  # The portal's credentials reach the gateway through its signed webhook to platform-api, as in
  # webhooks/devportal_webhook.feature, whose fixtures create the application and the key as the
  # administrator. The portal dispatches events in the order they occur, so a second key revoked
  # after the deletion, and refused at the gateway, shows the deletion's event has been processed.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And a webhook subscriber targeting the control plane is registered in the API Portal
    And I generate a unique value from "devportal-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane

  @APIP-DEVPORTAL-WEBHOOK-002
  Scenario: An API key linked to a deleted application keeps authorizing calls at the gateway
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
